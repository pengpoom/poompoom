package api

import (
	"context"
	"net/http"
	"strings"

	"imagestudio/internal/businesssettings"
)

const ctxKeyRequestOrigin ctxKey = "external_api_request_origin"

// requestOrigin 从请求推导对外可访问的 scheme://host：
// X-Forwarded-Host 优先于 Host，X-Forwarded-Proto/TLS 判定 https。
func requestOrigin(r *http.Request) string {
	host := strings.TrimSpace(r.Host)
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fwd != "" {
		host = fwd
	}
	if host == "" {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}
	return scheme + "://" + host
}

func withRequestOrigin(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, ctxKeyRequestOrigin, requestOrigin(r))
}

func requestOriginFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestOrigin).(string)
	return v
}

// resolveFacadeBaseURL 解析 facade 对外 URL 前缀，优先级：
// 系统设置 base_url（数据库）> 当前请求 Host 推导 > .env/config 的 base_url。
func (s *Server) resolveFacadeBaseURL(ctx context.Context) string {
	settings := s.businessSystemSettingsForContext(ctx)
	fallback := strings.TrimSpace(s.cfg.ExternalAPI.BaseURL)
	if origin := requestOriginFromContext(ctx); origin != "" {
		fallback = origin
	}
	return strings.TrimRight(strings.TrimSpace(businesssettings.ResolveAPIBaseURL(settings, fallback)), "/")
}
