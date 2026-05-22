package outboundproxy

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "empty", raw: "", wantErr: false},
		{name: "socks5h", raw: "socks5h://127.0.0.1:10808", wantErr: false},
		{name: "http", raw: "http://127.0.0.1:7890", wantErr: false},
		{name: "unsupported", raw: "ftp://127.0.0.1:21", wantErr: true},
		{name: "missing host", raw: "socks5h://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestNewHTTPTransportForSOCKSProxy(t *testing.T) {
	transport, err := NewHTTPTransport("socks5h://127.0.0.1:10808")
	if err != nil {
		t.Fatalf("NewHTTPTransport returned error: %v", err)
	}
	if transport.Proxy != nil {
		t.Fatal("expected socks transport to dial directly without http proxy func")
	}
	if transport.DialContext == nil {
		t.Fatal("expected socks transport to configure DialContext")
	}
}

func TestNewHTTPTransportForHTTPProxy(t *testing.T) {
	transport, err := NewHTTPTransport("http://127.0.0.1:7890")
	if err != nil {
		t.Fatalf("NewHTTPTransport returned error: %v", err)
	}
	if transport.Proxy == nil {
		t.Fatal("expected http proxy transport to configure Proxy func")
	}
}

func TestNewHTTPTransportWithoutProxyPreservesProxyFunc(t *testing.T) {
	transport, err := NewHTTPTransport("")
	if err != nil {
		t.Fatalf("NewHTTPTransport returned error: %v", err)
	}
	if transport.Proxy == nil {
		t.Fatal("expected default proxy func to be preserved")
	}
}

func TestBindConnToContextCancelsBlockedRead(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	resetDeadline := bindConnToContext(ctx, serverConn)
	defer resetDeadline()

	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		_, err := serverConn.Read(buf)
		errCh <- err
	}()

	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected canceled read to fail")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for canceled read")
	}
}

func TestDefaultProxyFuncCanResolveEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")

	transport, err := NewHTTPTransport("")
	if err != nil {
		t.Fatalf("NewHTTPTransport returned error: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy returned error: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("expected environment proxy, got %v", proxyURL)
	}
}
