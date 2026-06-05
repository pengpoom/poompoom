package businesssettings

import "strings"

// ResolveAPIBaseURL 返回对外 API 的公开访问地址：优先使用站点设置中的值，为空时
// 回退到给定默认值（通常来自 .env / config 的 EXTERNAL_API_BASE_URL）。
func ResolveAPIBaseURL(settings Settings, fallback string) string {
	if v := strings.TrimSpace(settings.Site.ApiBaseURL); v != "" {
		return v
	}
	return strings.TrimSpace(fallback)
}
