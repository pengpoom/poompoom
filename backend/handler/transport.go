package handler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"imagestudio/internal/outboundproxy"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// newChromeTransport returns an http.RoundTripper that mimics Chrome's TLS fingerprint
// and properly supports HTTP/2.
func newChromeTransport(proxyURL ...string) http.RoundTripper {
	configuredProxyURL := firstProxyURL(proxyURL...)
	fallbackTransport, err := outboundproxy.NewHTTPTransport(configuredProxyURL)
	if err != nil {
		panic(err)
	}
	tunnelDialContext, err := outboundproxy.NewTunnelDialContext(configuredProxyURL)
	if err != nil {
		panic(err)
	}

	return &chromeTransport{
		fallback:   fallbackTransport,
		tunnelDial: tunnelDialContext,
	}
}

type chromeTransport struct {
	mu         sync.Mutex
	h2Conns    map[string]*http2.ClientConn
	fallback   http.RoundTripper
	tunnelDial func(context.Context, string, string) (net.Conn, error)
}

func (t *chromeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" {
		return t.fallback.RoundTrip(req)
	}

	addr := req.URL.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = addr + ":443"
	}
	host := req.URL.Hostname()

	t.mu.Lock()
	if t.h2Conns == nil {
		t.h2Conns = make(map[string]*http2.ClientConn)
	}
	cc, ok := t.h2Conns[addr]
	if ok {
		// Check if connection is still usable
		if cc.CanTakeNewRequest() {
			t.mu.Unlock()
			return cc.RoundTrip(req)
		}
		delete(t.h2Conns, addr)
	}
	t.mu.Unlock()

	// Establish new connection
	conn, err := t.dialTLS(req.Context(), addr, host)
	if err != nil {
		return nil, err
	}

	// Create HTTP/2 client connection
	tr := &http2.Transport{}
	newCC, err := tr.NewClientConn(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("http2 client conn: %w", err)
	}

	t.mu.Lock()
	t.h2Conns[addr] = newCC
	t.mu.Unlock()

	return newCC.RoundTrip(req)
}

func (t *chromeTransport) dialTLS(ctx context.Context, addr, host string) (net.Conn, error) {
	conn, err := t.tunnelDial(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	tlsConn := utls.UClient(conn, &utls.Config{
		ServerName: host,
		NextProtos: []string{"h2", "http/1.1"},
	}, utls.HelloChrome_Auto)

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	return tlsConn, nil
}

func firstProxyURL(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
