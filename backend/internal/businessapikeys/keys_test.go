package businessapikeys

import (
	"strings"
	"testing"
)

func TestGenerateKeyLiveFormat(t *testing.T) {
	gen, err := GenerateKey("live")
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	if !strings.HasPrefix(gen.Plaintext, "poom_live_") {
		t.Fatalf("plaintext = %q, want poom_live_ prefix", gen.Plaintext)
	}
	if gen.Prefix != "poom_live" {
		t.Fatalf("prefix = %q, want poom_live", gen.Prefix)
	}
	if gen.Last4 != gen.Plaintext[len(gen.Plaintext)-4:] {
		t.Fatalf("last4 = %q does not match plaintext tail", gen.Last4)
	}
}

func TestGenerateKeyUnknownEnvFallsBackToLive(t *testing.T) {
	gen, err := GenerateKey("weird")
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	if gen.Prefix != "poom_live" {
		t.Fatalf("prefix = %q, want poom_live fallback", gen.Prefix)
	}
}

func TestGenerateKeyUnique(t *testing.T) {
	a, _ := GenerateKey("live")
	b, _ := GenerateKey("live")
	if a.Plaintext == b.Plaintext {
		t.Fatalf("two generated keys are identical: %q", a.Plaintext)
	}
}

func TestKeyHashDeterministicAndSecretSensitive(t *testing.T) {
	h1 := KeyHash("secret-a", "poom_live_abc")
	h2 := KeyHash("secret-a", "poom_live_abc")
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %q vs %q", h1, h2)
	}
	if KeyHash("secret-b", "poom_live_abc") == h1 {
		t.Fatalf("hash should differ for different secret")
	}
	if h1 == "" {
		t.Fatalf("hash is empty")
	}
}
