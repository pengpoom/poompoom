package api

import "testing"

func TestToPublicStatus(t *testing.T) {
	cases := map[string]string{
		"queued":           "queued",
		"running":          "running",
		"cancel_requested": "running",
		"cancelled":        "cancelled",
		"succeeded":        "succeeded",
		"failed":           "failed",
		"weird":            "running",
	}
	for in, want := range cases {
		if got := toPublicStatus(in); got != want {
			t.Errorf("toPublicStatus(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSignedImageURL(t *testing.T) {
	secret := "test-secret"
	name := "business-abc.png"
	exp := int64(1780480000)
	sig := signImageFileToken(secret, name, exp)
	if !verifyImageFileToken(secret, name, exp, sig) {
		t.Fatal("valid signature rejected")
	}
	if verifyImageFileToken(secret, name, exp, sig+"x") {
		t.Fatal("tampered signature accepted")
	}
	if verifyImageFileToken(secret, "business-other.png", exp, sig) {
		t.Fatal("cross-file signature accepted")
	}
	q := signImageFileQuery(secret, name, exp)
	if q == "" || q[0] != '?' {
		t.Fatalf("query=%q", q)
	}
}
