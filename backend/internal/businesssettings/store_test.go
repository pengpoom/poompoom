package businesssettings

import "testing"

func TestNormalizeTrimsApiBaseURL(t *testing.T) {
	in := Defaults()
	in.Site.ApiBaseURL = "  https://api.example.com/  "
	out := Normalize(in)
	if out.Site.ApiBaseURL != "https://api.example.com/" {
		t.Fatalf("ApiBaseURL = %q, want trimmed value", out.Site.ApiBaseURL)
	}
}

func TestResolveAPIBaseURLPrefersSetting(t *testing.T) {
	s := Defaults()
	s.Site.ApiBaseURL = "https://custom.example.com"
	if got := ResolveAPIBaseURL(s, "https://fallback.example.com"); got != "https://custom.example.com" {
		t.Fatalf("ResolveAPIBaseURL = %q, want the configured setting", got)
	}
}

func TestResolveAPIBaseURLFallsBackWhenEmpty(t *testing.T) {
	s := Defaults()
	s.Site.ApiBaseURL = "   "
	if got := ResolveAPIBaseURL(s, "  https://fallback.example.com  "); got != "https://fallback.example.com" {
		t.Fatalf("ResolveAPIBaseURL = %q, want trimmed fallback", got)
	}
}
