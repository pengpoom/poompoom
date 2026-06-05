package config

import (
	"encoding/hex"
	"testing"
)

func TestEnsureExternalAPISigningSecretGeneratesWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg := New(dir)
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.ExternalAPI.Enabled {
		t.Fatalf("precondition: expected default Enabled=true")
	}
	if cfg.ExternalAPI.SigningSecret != "" {
		t.Fatalf("precondition: expected empty SigningSecret, got %q", cfg.ExternalAPI.SigningSecret)
	}

	generated, err := cfg.EnsureExternalAPISigningSecret()
	if err != nil {
		t.Fatalf("EnsureExternalAPISigningSecret() error: %v", err)
	}
	if !generated {
		t.Fatalf("generated = false, want true (secret was empty)")
	}
	secret := cfg.ExternalAPI.SigningSecret
	if len(secret) != 64 {
		t.Fatalf("secret length = %d, want 64 hex chars", len(secret))
	}
	if _, err := hex.DecodeString(secret); err != nil {
		t.Fatalf("secret is not valid hex: %v", err)
	}

	// 持久化：用同一目录重新加载，应读到同一密钥
	reloaded := New(dir)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("reload Load() error: %v", err)
	}
	if reloaded.ExternalAPI.SigningSecret != secret {
		t.Fatalf("persisted secret = %q, want %q", reloaded.ExternalAPI.SigningSecret, secret)
	}

	// 幂等：再次调用不应改变已存在的密钥
	again, err := cfg.EnsureExternalAPISigningSecret()
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if again {
		t.Fatalf("generated = true on second call, want false (idempotent)")
	}
	if cfg.ExternalAPI.SigningSecret != secret {
		t.Fatalf("secret changed on second call: got %q, want %q", cfg.ExternalAPI.SigningSecret, secret)
	}
}

func TestEnsureExternalAPISigningSecretKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	cfg := New(dir)
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	const existing = "preset-secret-value"
	if err := cfg.SaveOverride("external_api", "signing_secret", existing); err != nil {
		t.Fatalf("SaveOverride() error: %v", err)
	}

	generated, err := cfg.EnsureExternalAPISigningSecret()
	if err != nil {
		t.Fatalf("EnsureExternalAPISigningSecret() error: %v", err)
	}
	if generated {
		t.Fatalf("generated = true, want false (secret already set)")
	}
	if cfg.ExternalAPI.SigningSecret != existing {
		t.Fatalf("secret = %q, want %q", cfg.ExternalAPI.SigningSecret, existing)
	}
}

func TestEnsureExternalAPISigningSecretSkipsWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := New(dir)
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := cfg.SaveOverride("external_api", "enabled", false); err != nil {
		t.Fatalf("SaveOverride() error: %v", err)
	}

	generated, err := cfg.EnsureExternalAPISigningSecret()
	if err != nil {
		t.Fatalf("EnsureExternalAPISigningSecret() error: %v", err)
	}
	if generated {
		t.Fatalf("generated = true, want false (disabled)")
	}
	if cfg.ExternalAPI.SigningSecret != "" {
		t.Fatalf("secret = %q, want empty (disabled must not generate)", cfg.ExternalAPI.SigningSecret)
	}
}
