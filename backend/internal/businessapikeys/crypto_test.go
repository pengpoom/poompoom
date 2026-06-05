package businessapikeys

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "test-signing-secret"
	plaintext := "poom_live_abcdef0123456789"
	token, err := EncryptKey(secret, plaintext)
	if err != nil {
		t.Fatalf("EncryptKey() error: %v", err)
	}
	if token == "" || token == plaintext {
		t.Fatalf("token = %q, want non-empty ciphertext different from plaintext", token)
	}
	got, err := DecryptKey(secret, token)
	if err != nil {
		t.Fatalf("DecryptKey() error: %v", err)
	}
	if got != plaintext {
		t.Fatalf("DecryptKey() = %q, want %q", got, plaintext)
	}
}

func TestEncryptUsesRandomNonce(t *testing.T) {
	secret := "test-signing-secret"
	plaintext := "poom_live_same"
	a, err := EncryptKey(secret, plaintext)
	if err != nil {
		t.Fatalf("EncryptKey() a error: %v", err)
	}
	b, err := EncryptKey(secret, plaintext)
	if err != nil {
		t.Fatalf("EncryptKey() b error: %v", err)
	}
	if a == b {
		t.Fatalf("two encryptions of same plaintext produced identical ciphertext (nonce not random)")
	}
}

func TestDecryptWrongSecretFails(t *testing.T) {
	token, err := EncryptKey("secret-one", "poom_live_x")
	if err != nil {
		t.Fatalf("EncryptKey() error: %v", err)
	}
	if _, err := DecryptKey("secret-two", token); err == nil {
		t.Fatalf("DecryptKey(wrong secret) error = nil, want error")
	}
}

func TestDecryptCorruptedFails(t *testing.T) {
	if _, err := DecryptKey("secret", "not-valid-base64!!!"); err == nil {
		t.Fatalf("DecryptKey(corrupted) error = nil, want error")
	}
	if _, err := DecryptKey("secret", "c2hvcnQ="); err == nil {
		t.Fatalf("DecryptKey(too short) error = nil, want error")
	}
}
