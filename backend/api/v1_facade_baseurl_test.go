package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestOrigin(t *testing.T) {
	cases := []struct {
		name   string
		setup  func() *http.Request
		expect string
	}{
		{
			name: "plain host http",
			setup: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "http://1.2.3.4:7000/v1/images/generations", nil)
			},
			expect: "http://1.2.3.4:7000",
		},
		{
			name: "tls means https",
			setup: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "https://poomp.top/v1/images/generations", nil)
			},
			expect: "https://poomp.top",
		},
		{
			name: "x-forwarded-proto https",
			setup: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://poomp.top/x", nil)
				r.Header.Set("X-Forwarded-Proto", "https")
				return r
			},
			expect: "https://poomp.top",
		},
		{
			name: "x-forwarded-host wins over host",
			setup: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://internal:7000/x", nil)
				r.Header.Set("X-Forwarded-Host", "poomp.top")
				r.Header.Set("X-Forwarded-Proto", "https")
				return r
			},
			expect: "https://poomp.top",
		},
		{
			name: "empty host yields empty",
			setup: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://x/x", nil)
				r.Host = ""
				return r
			},
			expect: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requestOrigin(c.setup()); got != c.expect {
				t.Fatalf("requestOrigin = %q, want %q", got, c.expect)
			}
		})
	}
}

func TestResolveFacadeBaseURL(t *testing.T) {
	server, _ := apiAccessTestServer(t)
	server.cfg.ExternalAPI.BaseURL = "https://config.example.com"

	// 系统设置为空时，回退到请求 Host 推导出的 origin
	originReq := httptest.NewRequest(http.MethodGet, "http://1.2.3.4:7000/v1/images/generations", nil)
	ctxWithOrigin := withRequestOrigin(context.Background(), originReq)
	if got := server.resolveFacadeBaseURL(ctxWithOrigin); got != "http://1.2.3.4:7000" {
		t.Fatalf("resolveFacadeBaseURL(origin) = %q, want http://1.2.3.4:7000", got)
	}

	// 系统设置为空且无 origin 时，回退到 .env/config 的 base_url
	if got := server.resolveFacadeBaseURL(context.Background()); got != "https://config.example.com" {
		t.Fatalf("resolveFacadeBaseURL(config) = %q, want https://config.example.com", got)
	}
}
