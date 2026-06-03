package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/businessapikeys"
	"imagestudio/internal/config"
)

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

func TestAuthorizeBusinessImageFileSignature(t *testing.T) {
	cfg := &config.Config{}
	cfg.ExternalAPI.SigningSecret = "sek"
	s := &Server{cfg: cfg}
	name := "business-abc.png"
	exp := time.Now().UTC().Add(time.Hour).Unix()
	good := httptest.NewRequest("GET", "/v1/files/image/"+name+signImageFileQuery("sek", name, exp), nil)
	if st, ok := s.authorizeBusinessImageFile(good, name); !ok || st != http.StatusOK {
		t.Fatalf("valid sig rejected: st=%d ok=%v", st, ok)
	}
	staleExp := time.Now().UTC().Add(-time.Hour).Unix()
	stale := httptest.NewRequest("GET", "/v1/files/image/"+name+signImageFileQuery("sek", name, staleExp), nil)
	if _, ok := s.authorizeBusinessImageFile(stale, name); ok {
		t.Fatal("expired sig accepted")
	}
	cross := httptest.NewRequest("GET", "/v1/files/image/"+name+signImageFileQuery("sek", "business-other.png", exp), nil)
	if _, ok := s.authorizeBusinessImageFile(cross, name); ok {
		t.Fatal("cross-file sig accepted")
	}
}

func TestExternalAPIKeyMiddlewareGate(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	sOff := &Server{cfg: &config.Config{}}
	rr := httptest.NewRecorder()
	sOff.requireExternalAPIKey(next).ServeHTTP(rr, httptest.NewRequest("GET", "/v1/images/models", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled: want 503 got %d", rr.Code)
	}

	cfgOn := &config.Config{}
	cfgOn.ExternalAPI.Enabled = true
	sOn := &Server{cfg: cfgOn}
	rr2 := httptest.NewRecorder()
	sOn.requireExternalAPIKey(next).ServeHTTP(rr2, httptest.NewRequest("GET", "/v1/images/models", nil))
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401 got %d", rr2.Code)
	}
}

func TestV1CreateImageGenerationValidation(t *testing.T) {
	cfg := &config.Config{}
	cfg.ExternalAPI.Enabled = true
	cfg.ExternalAPI.MaxMetadataBytes = 10
	s := &Server{cfg: cfg}
	ctx := context.WithValue(context.Background(), ctxKeyAPIKey, businessapikeys.APIKey{ID: "k1", UserID: "u1", Status: "active"})

	r1 := httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{}`)).WithContext(ctx)
	w1 := httptest.NewRecorder()
	s.handleV1CreateImageGeneration(w1, r1)
	if w1.Code != http.StatusBadRequest {
		t.Fatalf("missing prompt: want 400 got %d", w1.Code)
	}

	r2 := httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{"prompt":"x","metadata":{"aaaaaaaaaa":"bbbbbbbbbb"}}`)).WithContext(ctx)
	w2 := httptest.NewRecorder()
	s.handleV1CreateImageGeneration(w2, r2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("oversized metadata: want 400 got %d", w2.Code)
	}
}

func TestMetadataOrNilAndParseTime(t *testing.T) {
	if metadataOrNil(nil) != nil {
		t.Fatal("nil bytes should map to nil")
	}
	if metadataOrNil([]byte("{}")) != nil {
		t.Fatal("empty object should map to nil")
	}
	if string(metadataOrNil([]byte(`{"a":1}`))) != `{"a":1}` {
		t.Fatal("non-empty metadata should pass through")
	}
	ts := "2026-06-04T12:00:00Z"
	want, _ := time.Parse(time.RFC3339, ts)
	if got := parseRFC3339Unix(ts); got != want.Unix() {
		t.Fatalf("parseRFC3339Unix=%d want %d", got, want.Unix())
	}
	if parseRFC3339Unix("garbage") != 0 {
		t.Fatal("garbage should be 0")
	}
}
