package config

import "testing"

func TestExternalAPIConfigDefaults(t *testing.T) {
	cfg := New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !cfg.ExternalAPI.Enabled {
		t.Fatalf("ExternalAPI.Enabled = false, want true (must default ON)")
	}
	if cfg.ExternalAPI.SignedURLTTLSeconds != 3600 {
		t.Fatalf("ExternalAPI.SignedURLTTLSeconds = %d, want 3600", cfg.ExternalAPI.SignedURLTTLSeconds)
	}
	if cfg.ExternalAPI.DefaultRateLimitPerMinute != 60 {
		t.Fatalf("ExternalAPI.DefaultRateLimitPerMinute = %d, want 60", cfg.ExternalAPI.DefaultRateLimitPerMinute)
	}
}
