package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/businessapikeys"
	"imagestudio/internal/businessauth"
	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessproviders"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
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

func TestV1RoutesRegistered(t *testing.T) {
	h := (&Server{cfg: &config.Config{}}).Handler()
	for _, tc := range []struct {
		method string
		path   string
	}{
		{"POST", "/v1/images/generations"},
		{"GET", "/v1/images/jobs/abc"},
		{"DELETE", "/v1/images/jobs/abc"},
		{"GET", "/v1/images/models"},
		{"GET", "/v1/images/credits"},
	} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}")))
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s: want 503 (disabled) got %d", tc.method, tc.path, rr.Code)
		}
	}
}

func facadeServe(t *testing.T, handler http.Handler, method, target, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestV1FacadeEndToEnd(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	imageB64 := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 1,
			"data":    []map[string]any{{"b64_json": imageB64}},
		})
	}))
	defer upstream.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := config.New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 5
	cfg.Database.MaxIdleConns = 2
	cfg.Storage.ImageDir = t.TempDir()
	cfg.JobQueue.Backend = "local"
	cfg.ExternalAPI.Enabled = true
	cfg.ExternalAPI.SigningSecret = "facade-secret"
	cfg.ExternalAPI.BaseURL = ""
	cfg.ExternalAPI.SignedURLTTLSeconds = 3600

	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	userID := "facade_user_" + suffix
	authStore := businessauth.NewStoreWithDB(db, cfg.Database.Driver)
	if err := authStore.EnsureBootstrapUsers(ctx, []businessauth.BootstrapUser{
		{ID: userID, Username: "facade_" + suffix, Email: "facade_" + suffix + "@example.test", Password: "pass", Role: businessauth.RoleUser},
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	providerStore := businessproviders.NewStoreWithDB(db, cfg.Database.Driver)
	if _, err := providerStore.Create(ctx, businessproviders.MutationInput{
		Name:         "Facade Test Provider",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      upstream.URL,
		APIKey:       "provider-key",
		DefaultModel: "gpt-image-test",
		Enabled:      true,
		IsDefault:    true,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	creditStore := businesscredits.NewStoreWithDB(db, cfg.Database.Driver)
	if _, _, err := creditStore.SetBalance(ctx, userID, 100, "test"); err != nil {
		t.Fatalf("set credit: %v", err)
	}

	server := NewServerWithDatabase(cfg, nil, nil, db)
	handler := server.Handler()

	keyStore, err := server.newBusinessAPIKeyStore()
	if err != nil {
		t.Fatalf("key store: %v", err)
	}
	defer keyStore.Close()
	_, plaintext, err := keyStore.Create(ctx, businessapikeys.CreateInput{UserID: userID, Name: "facade key"})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	rec := facadeServe(t, handler, http.MethodPost, "/v1/images/generations", plaintext, `{"prompt":"facade smoke","metadata":{"scene":"demo"}}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit: want 202 got %d body=%s", rec.Code, rec.Body.String())
	}
	var created v1ImageJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode submit: %v", err)
	}
	if created.ID == "" || created.Status != "queued" {
		t.Fatalf("submit resp = %#v", created)
	}

	var done v1ImageJobResponse
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		pr := facadeServe(t, handler, http.MethodGet, "/v1/images/jobs/"+created.ID, plaintext, "")
		if pr.Code != http.StatusOK {
			t.Fatalf("poll: status %d body %s", pr.Code, pr.Body.String())
		}
		done = v1ImageJobResponse{}
		_ = json.Unmarshal(pr.Body.Bytes(), &done)
		if done.Status == "succeeded" || done.Status == "failed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if done.Status != "succeeded" {
		t.Fatalf("job not succeeded: %#v", done)
	}
	if len(done.Images) == 0 || done.Images[0].URL == "" {
		t.Fatalf("no image url: %#v", done.Images)
	}
	if string(done.Metadata) != `{"scene":"demo"}` {
		t.Fatalf("metadata not echoed: %q", string(done.Metadata))
	}

	ir := httptest.NewRecorder()
	handler.ServeHTTP(ir, httptest.NewRequest(http.MethodGet, done.Images[0].URL, nil))
	if ir.Code != http.StatusOK {
		t.Fatalf("signed image fetch: want 200 got %d body=%s", ir.Code, ir.Body.String())
	}

	_, plaintext2, err := keyStore.Create(ctx, businessapikeys.CreateInput{UserID: userID, Name: "other key"})
	if err != nil {
		t.Fatalf("create key2: %v", err)
	}
	cr := facadeServe(t, handler, http.MethodGet, "/v1/images/jobs/"+created.ID, plaintext2, "")
	if cr.Code != http.StatusNotFound {
		t.Fatalf("cross-key isolation: want 404 got %d", cr.Code)
	}
}
