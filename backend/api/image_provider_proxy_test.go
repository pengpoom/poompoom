package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"imagestudio/internal/businessauth"
	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
	"imagestudio/internal/businessjobs"
	"imagestudio/internal/businesspayments"
	"imagestudio/internal/businessproviders"
	"imagestudio/internal/businesssettings"
	"imagestudio/internal/businesstracker"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func newBusinessImageTestConfig(t *testing.T) *config.Config {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	cfg := config.New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 4
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60
	resetBusinessImageProxyTestData(t, cfg)
	return cfg
}

func resetBusinessImageProxyTestData(t *testing.T, cfg *config.Config) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open postgres database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("migrate postgres database: %v", err)
	}
	_, err = db.ExecContext(ctx, `TRUNCATE
		business_image_assets,
		business_image_generations,
		business_image_conversations,
		business_image_jobs,
		business_image_tracker,
		business_notification_reads,
		business_notifications,
		business_credit_ledger,
		business_user_credits,
		business_provider_members,
		business_provider_groups,
		business_api_providers,
		business_system_settings,
		email_verification_codes,
		user_sessions,
		business_users
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("reset business image proxy test tables: %v", err)
	}
}

func TestExtractProviderImageGenerateMetadataPrefersJobID(t *testing.T) {
	metadata := extractProviderImageGenerateMetadata(map[string]any{
		"jobId": "job-current",
	})

	if metadata.JobID != "job-current" {
		t.Fatalf("JobID = %q, want job-current", metadata.JobID)
	}
}

func TestProviderImageGenerateProxiesOpenAICompatibleRequest(t *testing.T) {
	var gotAuth string
	var gotPayload map[string]any
	imageB64 := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("upstream path = %q, want /v1/images/generations", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode upstream payload: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","n":0,"conversationId":"conv-1","turnId":"turn-1","jobId":"job-1","title":"Cat session"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer provider-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotPayload["model"] != "gpt-image-test" {
		t.Fatalf("model = %#v, want gpt-image-test", gotPayload["model"])
	}
	if gotPayload["n"] != float64(1) {
		t.Fatalf("n = %#v, want 1", gotPayload["n"])
	}
	if gotPayload["response_format"] != "b64_json" {
		t.Fatalf("response_format = %#v, want b64_json", gotPayload["response_format"])
	}
	for _, key := range []string{"conversationId", "turnId", "jobId", "title"} {
		if _, ok := gotPayload[key]; ok {
			t.Fatalf("provider payload unexpectedly contains %q", key)
		}
	}

	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business image store: %v", err)
	}
	defer store.Close()
	conversation, ok, err := store.GetConversation(context.Background(), "conv-1")
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if !ok {
		t.Fatal("conversation was not recorded")
	}
	if conversation.UserID != businessimage.DevUserID {
		t.Fatalf("conversation user_id = %q, want %q", conversation.UserID, businessimage.DevUserID)
	}
	if conversation.Title != "Cat session" {
		t.Fatalf("conversation title = %q, want Cat session", conversation.Title)
	}
	generations, err := store.ListGenerations(context.Background(), "conv-1", 10)
	if err != nil {
		t.Fatalf("list generations: %v", err)
	}
	if len(generations) != 1 {
		t.Fatalf("generations len = %d, want 1", len(generations))
	}
	generation := generations[0]
	if generation.ID != "job-1" || generation.TurnID != "turn-1" {
		t.Fatalf("generation id/turn = %q/%q, want job-1/turn-1", generation.ID, generation.TurnID)
	}
	if generation.Prompt != "cat" || generation.Model != "gpt-image-test" || generation.Count != 1 {
		t.Fatalf("generation payload = prompt:%q model:%q count:%d", generation.Prompt, generation.Model, generation.Count)
	}
	if strings.Contains(string(generation.Response), "b64_json") {
		t.Fatalf("generation response should not keep b64_json: %s", generation.Response)
	}
	if !strings.Contains(string(generation.Response), `"/v1/files/image/business-`) {
		t.Fatalf("generation response missing persisted image url: %s", generation.Response)
	}
	var savedResponse struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(generation.Response, &savedResponse); err != nil {
		t.Fatalf("decode saved response: %v", err)
	}
	if len(savedResponse.Data) != 1 || savedResponse.Data[0].URL == "" {
		t.Fatalf("saved response data = %#v", savedResponse.Data)
	}
	fileName := strings.TrimPrefix(savedResponse.Data[0].URL, "/v1/files/image/")
	if _, err := os.Stat(filepath.Join(cfg.ResolvePath(cfg.Storage.ImageDir), fileName)); err != nil {
		t.Fatalf("persisted image file not found: %v", err)
	}
	owners, err := store.AssetOwnerUserIDs(context.Background(), fileName)
	if err != nil {
		t.Fatalf("list asset owners: %v", err)
	}
	if len(owners) != 1 || owners[0] != businessimage.DevUserID {
		t.Fatalf("asset owners = %#v, want [%q]", owners, businessimage.DevUserID)
	}
	assetFiles, err := store.AssetFileNamesForConversation(context.Background(), "conv-1", businessimage.DevUserID)
	if err != nil {
		t.Fatalf("list conversation assets: %v", err)
	}
	if len(assetFiles) != 1 || assetFiles[0] != fileName {
		t.Fatalf("asset files = %#v, want [%q]", assetFiles, fileName)
	}

	trackerStore, err := businesstracker.NewStore(cfg)
	if err != nil {
		t.Fatalf("open tracker store: %v", err)
	}
	defer trackerStore.Close()
	trackerSummary, err := trackerStore.Summary(context.Background(), 3600)
	if err != nil {
		t.Fatalf("tracker summary: %v", err)
	}
	if trackerSummary.Total != 1 || trackerSummary.Succeeded != 1 || trackerSummary.Failed != 0 {
		t.Fatalf("tracker summary = %#v", trackerSummary)
	}
	if trackerSummary.ActualCount != 1 || trackerSummary.StorageBytes <= 0 {
		t.Fatalf("tracker image stats = actual:%d storage:%d", trackerSummary.ActualCount, trackerSummary.StorageBytes)
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	job, ok, err := jobStore.Get(context.Background(), "job-1", businessimage.DevUserID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if !ok {
		t.Fatal("job was not recorded")
	}
	if job.Status != businessjobs.StatusSucceeded || job.Stage != "finished" {
		t.Fatalf("job status/stage = %q/%q", job.Status, job.Stage)
	}
	if job.ConversationID != "conv-1" || job.GenerationID != "job-1" || job.TurnID != "turn-1" {
		t.Fatalf("job ids = %#v", job)
	}
	if job.RequestedCount != 1 || job.ActualCount != 1 || job.StorageBytes <= 0 {
		t.Fatalf("job stats = requested:%d actual:%d storage:%d", job.RequestedCount, job.ActualCount, job.StorageBytes)
	}
}

func TestProviderImageGenerateKeepsSubscriptionFirstAfterPartialRefund(t *testing.T) {
	imageB64 := base64.StdEncoding.EncodeToString([]byte("partial-image"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	seedBusinessSubscription(t, cfg, businessimage.DevUserID, 2)
	server := NewServer(cfg, nil, nil)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","n":4,"conversationId":"conv-partial-subscription","turnId":"turn-partial-subscription","jobId":"job-partial-subscription"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	paymentStore, err := businesspayments.NewStore(cfg)
	if err != nil {
		t.Fatalf("open payment store: %v", err)
	}
	defer paymentStore.Close()
	subscriptionTotals, err := paymentStore.SubscriptionGenerationTotals(context.Background(), businessimage.DevUserID, "job-partial-subscription")
	if err != nil {
		t.Fatalf("subscription totals: %v", err)
	}
	if subscriptionTotals.Reserved != 2 || subscriptionTotals.Refunded != 1 {
		t.Fatalf("subscription totals = %#v, want reserved 2 refunded 1", subscriptionTotals)
	}
	subscription, err := paymentStore.GetCurrentSubscription(context.Background(), businessimage.DevUserID)
	if err != nil {
		t.Fatalf("current subscription: %v", err)
	}
	if subscription.CreditsLeft != 1 {
		t.Fatalf("subscription credits left = %d, want 1", subscription.CreditsLeft)
	}

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("open credit store: %v", err)
	}
	defer creditStore.Close()
	balanceTotals, err := creditStore.GenerationTotals(context.Background(), businessimage.DevUserID, "job-partial-subscription")
	if err != nil {
		t.Fatalf("balance totals: %v", err)
	}
	if balanceTotals.Reserved != 2 || balanceTotals.Refunded != 2 {
		t.Fatalf("balance totals = %#v, want reserved 2 refunded 2", balanceTotals)
	}
	summary, err := creditStore.Summary(context.Background(), businessimage.DevUserID)
	if err != nil {
		t.Fatalf("credit summary: %v", err)
	}
	if summary.Balance != 5 || summary.Spent != 0 {
		t.Fatalf("credit summary = %#v, want unchanged balance 5", summary)
	}
}

func TestProviderImageEditProxiesOpenAICompatibleMultipartRequest(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotPrompt string
	var gotModel string
	var gotImage []byte
	var gotMask []byte
	imageB64 := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 7, 8, 9})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		gotPrompt = r.FormValue("prompt")
		gotModel = r.FormValue("model")
		imageFile, _, err := r.FormFile("image")
		if err != nil {
			t.Fatalf("image file missing: %v", err)
		}
		gotImage, _ = io.ReadAll(imageFile)
		_ = imageFile.Close()
		maskFile, _, err := r.FormFile("mask")
		if err != nil {
			t.Fatalf("mask file missing: %v", err)
		}
		gotMask, _ = io.ReadAll(maskFile)
		_ = maskFile.Close()
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "edited cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	sourceImage := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("source-image"))
	sourceMask := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("source-mask"))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"mode":"edit","prompt":"make it blue","n":1,"conversationId":"conv-edit","turnId":"turn-edit","jobId":"job-edit","sourceImages":[{"id":"src","role":"image","name":"source.png","dataUrl":"`+sourceImage+`"},{"id":"mask","role":"mask","name":"mask.png","dataUrl":"`+sourceMask+`"}]}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotPath != "/v1/images/edits" {
		t.Fatalf("upstream path = %q, want /v1/images/edits", gotPath)
	}
	if gotAuth != "Bearer provider-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotPrompt != "make it blue" || gotModel != "gpt-image-test" {
		t.Fatalf("multipart fields prompt/model = %q/%q", gotPrompt, gotModel)
	}
	if string(gotImage) != "source-image" || string(gotMask) != "source-mask" {
		t.Fatalf("multipart image/mask = %q/%q", string(gotImage), string(gotMask))
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	job, ok, err := jobStore.Get(context.Background(), "job-edit", businessimage.DevUserID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if !ok {
		t.Fatal("job not found")
	}
	var savedPayload map[string]any
	if err := json.Unmarshal(job.PayloadJSON, &savedPayload); err != nil {
		t.Fatalf("decode job payload: %v", err)
	}
	savedSources, ok := savedPayload["sourceImages"].([]any)
	if !ok || len(savedSources) != 2 {
		t.Fatalf("saved sourceImages = %#v", savedPayload["sourceImages"])
	}
	for _, raw := range savedSources {
		source, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("saved source = %#v", raw)
		}
		if strings.TrimSpace(stringValue(source["dataUrl"])) != "" {
			t.Fatalf("source payload still contains dataUrl: %#v", source)
		}
		url := strings.TrimSpace(stringValue(source["url"]))
		if !strings.HasPrefix(url, "/v1/files/image/business-") {
			t.Fatalf("source url = %q", url)
		}
	}
}

func TestProviderImageEditSubmitRunsWorkerAndExposesPayload(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	var gotPath string
	imageB64 := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if _, _, err := r.FormFile("image"); err != nil {
			t.Fatalf("image file missing: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "edited cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	sourceImage := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("source-image"))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"mode":"edit","prompt":"make it blue","n":1,"conversationId":"conv-edit-submit","turnId":"turn-edit-submit","jobId":"job-edit-submit","sourceImages":[{"id":"src","role":"image","name":"source.png","dataUrl":"`+sourceImage+`"}]}`),
	)
	req.Header.Set("Authorization", "Bearer "+defaultUserAuthKey)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	var job businessjobs.Job
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		var ok bool
		job, ok, err = jobStore.Get(context.Background(), "job-edit-submit", businessimage.DevUserID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if ok && job.Status == businessjobs.StatusSucceeded {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.Status != businessjobs.StatusSucceeded {
		t.Fatalf("job status = %q, body = %s", job.Status, rec.Body.String())
	}
	if gotPath != "/v1/images/edits" {
		t.Fatalf("upstream path = %q, want /v1/images/edits", gotPath)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/business/jobs/job-edit-submit", nil)
	getReq.Header.Set("Authorization", "Bearer "+defaultUserAuthKey)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get job status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
	var payload struct {
		Item struct {
			Payload struct {
				Mode         string `json:"mode"`
				SourceImages []struct {
					Role    string `json:"role"`
					URL     string `json:"url"`
					DataURL string `json:"dataUrl"`
				} `json:"sourceImages"`
			} `json:"payload"`
		} `json:"item"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode job payload: %v", err)
	}
	if payload.Item.Payload.Mode != "edit" || len(payload.Item.Payload.SourceImages) != 1 {
		t.Fatalf("public payload = %#v", payload.Item.Payload)
	}
	if payload.Item.Payload.SourceImages[0].DataURL != "" || !strings.HasPrefix(payload.Item.Payload.SourceImages[0].URL, "/v1/files/image/business-") {
		t.Fatalf("public source image = %#v", payload.Item.Payload.SourceImages[0])
	}
}

func TestProviderImageGenerateSubmitReturnsQueuedJobAndRunsWorker(t *testing.T) {
	var gotAuth string
	imageB64 := base64.StdEncoding.EncodeToString([]byte("async-image"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-submit","turnId":"turn-submit","jobId":"job-submit"}`),
	)
	req.Header.Set("Authorization", "Bearer "+defaultUserAuthKey)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"jobId":"job-submit"`) || !strings.Contains(rec.Body.String(), `"status":"queued"`) {
		t.Fatalf("queued job response = %s", rec.Body.String())
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	var job businessjobs.Job
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		var ok bool
		job, ok, err = jobStore.Get(context.Background(), "job-submit", businessimage.DevUserID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if ok && job.Status == businessjobs.StatusSucceeded {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.Status != businessjobs.StatusSucceeded {
		t.Fatalf("job status = %q, body = %s", job.Status, rec.Body.String())
	}
	if len(job.PayloadJSON) == 0 {
		t.Fatal("job payload_json was not saved")
	}
	if gotAuth != "Bearer provider-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}

	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business image store: %v", err)
	}
	defer store.Close()
	generations, err := store.ListGenerations(context.Background(), "conv-submit", 10)
	if err != nil {
		t.Fatalf("list generations: %v", err)
	}
	if len(generations) != 1 || generations[0].Status != "succeeded" {
		t.Fatalf("generations = %#v", generations)
	}
	if !strings.Contains(string(generations[0].Response), `"/v1/files/image/business-`) {
		t.Fatalf("generation response missing persisted url: %s", generations[0].Response)
	}
}

func TestRecoverQueuedBusinessImageJobsRunsPersistedJob(t *testing.T) {
	var gotAuth string
	imageB64 := base64.StdEncoding.EncodeToString([]byte("recovered-image"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)

	payload := map[string]any{
		"prompt":         "cat",
		"conversationId": "conv-recover",
		"turnId":         "turn-recover",
		"jobId":          "job-recover",
		"title":          "Recovered session",
	}
	job, err := server.createQueuedProviderImageJob(context.Background(), businessimage.DevUserID, payload, time.Now().Add(-time.Minute).UTC())
	if err != nil {
		t.Fatalf("create queued job: %v", err)
	}
	if job.Status != businessjobs.StatusQueued {
		t.Fatalf("created job status = %q, want queued", job.Status)
	}

	if recovered := server.recoverQueuedBusinessImageJobs(context.Background()); recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	var recoveredJob businessjobs.Job
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		var ok bool
		recoveredJob, ok, err = jobStore.Get(context.Background(), "job-recover", businessimage.DevUserID)
		if err != nil {
			t.Fatalf("get recovered job: %v", err)
		}
		if ok && recoveredJob.Status == businessjobs.StatusSucceeded {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if recoveredJob.Status != businessjobs.StatusSucceeded {
		t.Fatalf("recovered job status = %q", recoveredJob.Status)
	}
	if gotAuth != "Bearer provider-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}

	imageStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business image store: %v", err)
	}
	defer imageStore.Close()
	generations, err := imageStore.ListGenerations(context.Background(), "conv-recover", businessimage.DevUserID, 10)
	if err != nil {
		t.Fatalf("list recovered generations: %v", err)
	}
	if len(generations) != 1 || generations[0].Status != businessjobs.StatusSucceeded {
		t.Fatalf("recovered generations = %#v", generations)
	}
	if !strings.Contains(string(generations[0].Response), `"/v1/files/image/business-`) {
		t.Fatalf("generation response missing persisted url: %s", generations[0].Response)
	}
}

func TestRecoverQueuedBusinessImageJobsDoesNotDoubleClaim(t *testing.T) {
	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	server := NewServer(cfg, nil, nil)
	payload := map[string]any{
		"prompt":         "cat",
		"conversationId": "conv-claim",
		"turnId":         "turn-claim",
		"jobId":          "job-claim",
	}
	job, err := server.createQueuedProviderImageJob(context.Background(), businessimage.DevUserID, payload, time.Now().UTC())
	if err != nil {
		t.Fatalf("create queued job: %v", err)
	}

	store, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer store.Close()
	if _, ok, err := store.ClaimQueued(context.Background(), job.ID, job.UserID); err != nil || !ok {
		t.Fatalf("first claim ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.ClaimQueued(context.Background(), job.ID, job.UserID); err != nil {
		t.Fatalf("second claim returned error: %v", err)
	} else if ok {
		t.Fatal("second claim succeeded while first claim lease is still active")
	}
}

func TestRunQueuedBusinessImageJobClaimsByJobIDOnly(t *testing.T) {
	var gotAuth string
	imageB64 := base64.StdEncoding.EncodeToString([]byte("queued-id-image"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)

	payload := map[string]any{
		"prompt":         "cat",
		"conversationId": "conv-id-only",
		"turnId":         "turn-id-only",
		"jobId":          "job-id-only",
	}
	if _, err := server.createQueuedProviderImageJob(context.Background(), businessimage.DevUserID, payload, time.Now().UTC()); err != nil {
		t.Fatalf("create queued job: %v", err)
	}
	if recovered := server.runQueuedBusinessImageJob(context.Background(), "job-id-only"); recovered != 1 {
		t.Fatalf("runQueuedBusinessImageJob() = %d, want 1", recovered)
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	var job businessjobs.Job
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		var ok bool
		job, ok, err = jobStore.Get(context.Background(), "job-id-only", businessimage.DevUserID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if ok && job.Status == businessjobs.StatusSucceeded {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.Status != businessjobs.StatusSucceeded {
		t.Fatalf("job status = %q", job.Status)
	}
	if gotAuth != "Bearer provider-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestRenewBusinessImageJobLeaseUpdatesRunningJob(t *testing.T) {
	cfg := newBusinessImageTestConfig(t)
	server := NewServer(cfg, nil, nil)
	store, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer store.Close()

	pastLease := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	_, err = store.Save(context.Background(), businessjobs.Job{
		ID:             "job-heartbeat",
		UserID:         businessimage.DevUserID,
		ConversationID: "conv-heartbeat",
		GenerationID:   "gen-heartbeat",
		Status:         businessjobs.StatusRunning,
		Stage:          "running",
		RequestedCount: 1,
		LeaseUntil:     pastLease,
	})
	if err != nil {
		t.Fatalf("save running job: %v", err)
	}

	if !server.renewBusinessImageJobLease(context.Background(), "job-heartbeat", businessimage.DevUserID) {
		t.Fatal("renewBusinessImageJobLease() = false, want true")
	}
	job, ok, err := store.Get(context.Background(), "job-heartbeat", businessimage.DevUserID)
	if err != nil || !ok {
		t.Fatalf("get renewed job ok=%v err=%v", ok, err)
	}
	if job.LeaseUntil == "" || job.LeaseUntil == pastLease {
		t.Fatalf("LeaseUntil = %q, want renewed value", job.LeaseUntil)
	}
}

func TestCreateQueuedProviderImageJobEnforcesUserActiveCapacity(t *testing.T) {
	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	saveBusinessSystemSettings(t, cfg, func(settings *businesssettings.Settings) {
		settings.Runtime.MaxUserActiveJobs = 1
		settings.Runtime.MaxQueuedJobs = 100
		settings.Runtime.MaxProviderRunningJobs = 100
	})
	server := NewServer(cfg, nil, nil)
	ctx := context.Background()
	firstPayload := map[string]any{
		"prompt":         "cat",
		"conversationId": "conv-user-capacity-one",
		"turnId":         "turn-user-capacity-one",
		"jobId":          "job-user-capacity-one",
	}
	first, err := server.createQueuedProviderImageJob(ctx, businessimage.DevUserID, firstPayload, time.Now().UTC())
	if err != nil {
		t.Fatalf("first createQueuedProviderImageJob() returned error: %v", err)
	}

	secondPayload := map[string]any{
		"prompt":         "cat",
		"conversationId": "conv-user-capacity-two",
		"turnId":         "turn-user-capacity-two",
		"jobId":          "job-user-capacity-two",
	}
	if _, err := server.createQueuedProviderImageJob(ctx, businessimage.DevUserID, secondPayload, time.Now().UTC()); !providerSubmitErrorCodeIs(err, "image_user_job_limit") {
		t.Fatalf("second createQueuedProviderImageJob() error = %v, want image_user_job_limit", err)
	} else if !providerSubmitErrorMessageIs(err, "你已有任务正在排队或生成，请稍后再试。") {
		t.Fatalf("second createQueuedProviderImageJob() error = %v, want user capacity message", err)
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	first.Status = businessjobs.StatusSucceeded
	first.Stage = "done"
	first.ActualCount = 1
	if _, err := jobStore.Save(ctx, first); err != nil {
		t.Fatalf("release first job: %v", err)
	}
	if _, err := server.createQueuedProviderImageJob(ctx, businessimage.DevUserID, secondPayload, time.Now().UTC()); err != nil {
		t.Fatalf("createQueuedProviderImageJob() after release returned error: %v", err)
	}
}

func TestCreateQueuedProviderImageJobEnforcesGlobalQueuedCapacity(t *testing.T) {
	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	saveBusinessSystemSettings(t, cfg, func(settings *businesssettings.Settings) {
		settings.Runtime.MaxUserActiveJobs = 100
		settings.Runtime.MaxQueuedJobs = 1
		settings.Runtime.MaxProviderRunningJobs = 100
	})
	server := NewServer(cfg, nil, nil)
	ctx := context.Background()
	firstPayload := map[string]any{
		"prompt":         "cat",
		"conversationId": "conv-global-capacity-one",
		"turnId":         "turn-global-capacity-one",
		"jobId":          "job-global-capacity-one",
	}
	first, err := server.createQueuedProviderImageJob(ctx, "user-global-capacity-one", firstPayload, time.Now().UTC())
	if err != nil {
		t.Fatalf("first createQueuedProviderImageJob() returned error: %v", err)
	}

	secondPayload := map[string]any{
		"prompt":         "cat",
		"conversationId": "conv-global-capacity-two",
		"turnId":         "turn-global-capacity-two",
		"jobId":          "job-global-capacity-two",
	}
	if _, err := server.createQueuedProviderImageJob(ctx, "user-global-capacity-two", secondPayload, time.Now().UTC()); !providerSubmitErrorCodeIs(err, "image_queue_full") {
		t.Fatalf("second createQueuedProviderImageJob() error = %v, want image_queue_full", err)
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	first.Status = businessjobs.StatusRunning
	first.Stage = "running"
	if _, err := jobStore.Save(ctx, first); err != nil {
		t.Fatalf("move first job to running: %v", err)
	}
	if _, err := server.createQueuedProviderImageJob(ctx, "user-global-capacity-two", secondPayload, time.Now().UTC()); err != nil {
		t.Fatalf("createQueuedProviderImageJob() after queued release returned error: %v", err)
	}
}

func TestCreateQueuedProviderImageJobPersistsTaggedFallbackDispatchTrace(t *testing.T) {
	cfg := newBusinessImageTestConfig(t)
	store, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	fallbackGroup, err := store.CreateGroup(context.Background(), businessproviders.GroupInput{
		Name:      "fallback-pool",
		Platform:  businessproviders.PlatformGPTImage,
		Enabled:   true,
		IsDefault: true,
		Priority:  1,
		MatchMode: businessproviders.GroupMatchFallback,
	})
	if err != nil {
		t.Fatalf("create fallback group: %v", err)
	}
	taggedGroup, err := store.CreateGroup(context.Background(), businessproviders.GroupInput{
		Name:      "high-pool",
		Platform:  businessproviders.PlatformGPTImage,
		Tags:      "quality:high",
		MatchMode: businessproviders.GroupMatchAll,
		Enabled:   true,
		Priority:  1,
	})
	if err != nil {
		t.Fatalf("create tagged group: %v", err)
	}
	fallbackMember, err := store.CreateMember(context.Background(), businessproviders.MemberInput{
		GroupID:      fallbackGroup.ID,
		Name:         "fallback-member",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      "http://127.0.0.1:1",
		APIKey:       "fallback-key",
		DefaultModel: "gpt-image-fallback",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create fallback member: %v", err)
	}
	if _, err := store.CreateMember(context.Background(), businessproviders.MemberInput{
		GroupID:      taggedGroup.ID,
		Name:         "unavailable-member",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      "http://127.0.0.1:1",
		APIKey:       "tagged-key",
		DefaultModel: "gpt-image-tagged",
		Enabled:      true,
		Status:       businessproviders.MemberStatusUnavailable,
	}); err != nil {
		t.Fatalf("create unavailable tagged member: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close provider store: %v", err)
	}

	server := NewServer(cfg, nil, nil)
	job, err := server.createQueuedProviderImageJob(context.Background(), businessimage.DevUserID, map[string]any{
		"prompt":         "cat",
		"conversationId": "conv-tagged-fallback-trace",
		"turnId":         "turn-tagged-fallback-trace",
		"jobId":          "job-tagged-fallback-trace",
		"quality":        "high",
		"dispatchTags":   []string{"quality:high"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("createQueuedProviderImageJob() returned error: %v", err)
	}
	if job.ProviderID != fallbackMember.ID || job.ProviderName != fallbackMember.Name {
		t.Fatalf("job provider = %q/%q, want %q/%q", job.ProviderID, job.ProviderName, fallbackMember.ID, fallbackMember.Name)
	}

	var savedPayload map[string]any
	if err := json.Unmarshal(job.PayloadJSON, &savedPayload); err != nil {
		t.Fatalf("decode job payload: %v", err)
	}
	if savedPayload["providerSource"] != imageProviderSourcePool {
		t.Fatalf("providerSource = %#v, want %q", savedPayload["providerSource"], imageProviderSourcePool)
	}
	if savedPayload["providerGroupId"] != fallbackGroup.ID {
		t.Fatalf("providerGroupId = %#v, want %q", savedPayload["providerGroupId"], fallbackGroup.ID)
	}
	if savedPayload["dispatchStrategy"] != businessproviders.SelectionStrategyTaggedFallback {
		t.Fatalf("dispatchStrategy = %#v, want %q", savedPayload["dispatchStrategy"], businessproviders.SelectionStrategyTaggedFallback)
	}
	trace := providerDispatchTagStrings(savedPayload["dispatchTrace"])
	joinedTrace := strings.Join(trace, "\n")
	for _, want := range []string{"请求标签命中标签池", "标签池无可用成员", "fallback 池命中成员：fallback-member"} {
		if !strings.Contains(joinedTrace, want) {
			t.Fatalf("dispatchTrace = %#v, missing %q", trace, want)
		}
	}

	view := businessImageJobViewFromJob(job, map[string]string{fallbackGroup.ID: fallbackGroup.Name})
	if view.DispatchStrategy != businessproviders.SelectionStrategyTaggedFallback {
		t.Fatalf("view dispatchStrategy = %q", view.DispatchStrategy)
	}
	if !strings.Contains(strings.Join(view.DispatchTrace, "\n"), "标签池无可用成员") {
		t.Fatalf("view dispatchTrace = %#v", view.DispatchTrace)
	}
	if _, ok := view.Payload["dispatchTrace"]; ok {
		t.Fatalf("public payload should hide dispatchTrace: %#v", view.Payload)
	}
}

func TestProviderImageGenerateEnforcesProviderRunningCapacity(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("capacity-image"))},
			},
		})
	}))
	defer upstream.Close()

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	cfg.APIAccess.Platform = "gpt-image"
	cfg.APIAccess.BaseURL = upstream.URL
	cfg.APIAccess.APIKey = "provider-key"
	saveBusinessSystemSettings(t, cfg, func(settings *businesssettings.Settings) {
		settings.Runtime.MaxUserActiveJobs = 100
		settings.Runtime.MaxQueuedJobs = 100
		settings.Runtime.MaxProviderRunningJobs = 1
	})
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	provider := seedBusinessAPIProvider(t, cfg, businessproviders.MutationInput{
		Name:         "Capacity Test Provider",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      upstream.URL,
		APIKey:       "provider-key",
		DefaultModel: "gpt-image-test",
		Enabled:      true,
		IsDefault:    true,
	})
	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	blocker, err := jobStore.Save(context.Background(), businessjobs.Job{
		ID:             "job-provider-capacity-blocker",
		UserID:         "user-provider-capacity-blocker",
		ConversationID: "conv-provider-capacity-blocker",
		GenerationID:   "gen-provider-capacity-blocker",
		Platform:       businessproviders.PlatformGPTImage,
		ProviderID:     provider.ID,
		Status:         businessjobs.StatusRunning,
		Stage:          "running",
		RequestedCount: 1,
	})
	if err != nil {
		t.Fatalf("save blocking job: %v", err)
	}
	if err := jobStore.Close(); err != nil {
		t.Fatalf("close job store: %v", err)
	}

	server := NewServer(cfg, nil, nil)
	blocked := server.executeProviderImageGenerate(providerImageGenerateExecution{
		Context:   context.Background(),
		UserID:    businessimage.DevUserID,
		StartedAt: time.Now().UTC(),
		Payload: map[string]any{
			"prompt":         "cat",
			"conversationId": "conv-provider-capacity-blocked",
			"turnId":         "turn-provider-capacity-blocked",
			"jobId":          "job-provider-capacity-blocked",
		},
	})
	if blocked.StatusCode != http.StatusTooManyRequests || blocked.ErrorCode != "image_provider_running_limit" {
		t.Fatalf("blocked result = %#v, want provider running limit", blocked)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0 while provider capacity is full", upstreamCalls)
	}

	jobStore, err = businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store for release: %v", err)
	}
	blocker.Status = businessjobs.StatusSucceeded
	blocker.Stage = "done"
	blocker.ActualCount = 1
	if _, err := jobStore.Save(context.Background(), blocker); err != nil {
		t.Fatalf("release blocking job: %v", err)
	}
	if err := jobStore.Close(); err != nil {
		t.Fatalf("close job store after release: %v", err)
	}

	allowed := server.executeProviderImageGenerate(providerImageGenerateExecution{
		Context:   context.Background(),
		UserID:    businessimage.DevUserID,
		StartedAt: time.Now().UTC(),
		Payload: map[string]any{
			"prompt":         "cat",
			"conversationId": "conv-provider-capacity-allowed",
			"turnId":         "turn-provider-capacity-allowed",
			"jobId":          "job-provider-capacity-allowed",
		},
	})
	if allowed.StatusCode != http.StatusOK {
		t.Fatalf("allowed result status = %d, body = %s", allowed.StatusCode, string(allowed.Body))
	}
	if upstreamCalls != 1 {
		t.Fatalf("upstreamCalls = %d, want 1 after release", upstreamCalls)
	}
}

func TestProviderImageGeneratePrefersAPIAccessConfig(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotPayload map[string]any
	imageB64 := base64.StdEncoding.EncodeToString([]byte("image"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode upstream payload: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("IMAGE_API_KEY", "env-provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-env")

	cfg := newBusinessImageTestConfig(t)
	cfg.APIAccess.Platform = "gpt-image"
	cfg.APIAccess.BaseURL = upstream.URL
	cfg.APIAccess.APIKey = "api-access-key"
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-api-access","turnId":"turn-api-access","jobId":"job-api-access"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("upstream path = %q, want /v1/images/generations", gotPath)
	}
	if gotAuth != "Bearer api-access-key" {
		t.Fatalf("Authorization = %q, want API access key", gotAuth)
	}
	if gotPayload["model"] != "gpt-image-2" {
		t.Fatalf("model = %#v, want migrated provider default model", gotPayload["model"])
	}
}

func TestProviderImageGeneratePrefersProviderStoreOverLegacyAPIAccess(t *testing.T) {
	var gotAuth string
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("image"))},
			},
		})
	}))
	defer upstream.Close()

	cfg := newBusinessImageTestConfig(t)
	cfg.APIAccess.Platform = "gpt-image"
	cfg.APIAccess.BaseURL = "http://127.0.0.1:1"
	cfg.APIAccess.APIKey = "legacy-key"
	providerStore, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	_, err = providerStore.Create(context.Background(), businessproviders.MutationInput{
		Name:         "db-provider",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      upstream.URL,
		APIKey:       "db-key",
		DefaultModel: "gpt-image-2",
		Enabled:      true,
		IsDefault:    true,
	})
	_ = providerStore.Close()
	if err != nil {
		t.Fatalf("create db provider: %v", err)
	}
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)

	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-db-provider","turnId":"turn-db-provider","jobId":"job-db-provider"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("upstream path = %q, want /v1/images/generations", gotPath)
	}
	if gotAuth != "Bearer db-key" {
		t.Fatalf("Authorization = %q, want db provider key", gotAuth)
	}
}

func TestProviderImageGeneratePrefersProviderPoolOverLegacyProvider(t *testing.T) {
	var gotAuth string
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("pool-image"))},
			},
		})
	}))
	defer upstream.Close()

	cfg := newBusinessImageTestConfig(t)
	seedBusinessAPIProvider(t, cfg, businessproviders.MutationInput{
		Name:         "legacy-provider",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      "http://127.0.0.1:1",
		APIKey:       "legacy-key",
		DefaultModel: "gpt-image-legacy",
		Enabled:      true,
		IsDefault:    true,
	})
	group, member := seedBusinessProviderPoolMember(t, cfg, upstream.URL, "pool-key")
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)

	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-pool-provider","turnId":"turn-pool-provider","jobId":"job-pool-provider"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("upstream path = %q, want /v1/images/generations", gotPath)
	}
	if gotAuth != "Bearer pool-key" {
		t.Fatalf("Authorization = %q, want pool key", gotAuth)
	}
	store, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	defer store.Close()
	savedMember, ok, err := store.GetMember(context.Background(), member.ID)
	if err != nil || !ok {
		t.Fatalf("get pool member ok=%v err=%v", ok, err)
	}
	if savedMember.SuccessCount != 1 || savedMember.FailCount != 0 || savedMember.Status != businessproviders.MemberStatusActive {
		t.Fatalf("pool member health = %#v", savedMember)
	}
	if savedMember.GroupID != group.ID {
		t.Fatalf("pool member group = %q, want %q", savedMember.GroupID, group.ID)
	}
}

func TestProviderImageGenerateMarksProviderPoolFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": map[string]any{"message": "rate limit"},
		})
	}))
	defer upstream.Close()

	cfg := newBusinessImageTestConfig(t)
	_, member := seedBusinessProviderPoolMember(t, cfg, upstream.URL, "pool-key")
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)

	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-pool-failure","turnId":"turn-pool-failure","jobId":"job-pool-failure"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	store, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	defer store.Close()
	savedMember, ok, err := store.GetMember(context.Background(), member.ID)
	if err != nil || !ok {
		t.Fatalf("get pool member ok=%v err=%v", ok, err)
	}
	if savedMember.FailCount != 1 || savedMember.Status != businessproviders.MemberStatusLimited || savedMember.CooldownUntil == "" {
		t.Fatalf("pool member health = %#v", savedMember)
	}
}

func TestProviderImageGenerateFallsBackToAPIAccessAfterPoolFailure(t *testing.T) {
	poolCalls := 0
	poolUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		poolCalls++
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": map[string]any{"message": "pool upstream failed"},
		})
	}))
	defer poolUpstream.Close()

	var fallbackAuth string
	fallbackCalls := 0
	fallbackUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls++
		fallbackAuth = r.Header.Get("Authorization")
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("fallback-image"))},
			},
		})
	}))
	defer fallbackUpstream.Close()

	cfg := newBusinessImageTestConfig(t)
	cfg.APIAccess.Platform = businessproviders.PlatformGPTImage
	cfg.APIAccess.BaseURL = fallbackUpstream.URL
	cfg.APIAccess.APIKey = "fallback-key"
	_, member := seedBusinessProviderPoolMember(t, cfg, poolUpstream.URL, "pool-key")
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)

	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-pool-fallback","turnId":"turn-pool-fallback","jobId":"job-pool-fallback"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if poolCalls != 2 || fallbackCalls != 1 {
		t.Fatalf("calls pool=%d fallback=%d, want pool retry twice then fallback once", poolCalls, fallbackCalls)
	}
	if fallbackAuth != "Bearer fallback-key" {
		t.Fatalf("fallback Authorization = %q, want fallback key", fallbackAuth)
	}
	store, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	defer store.Close()
	savedMember, ok, err := store.GetMember(context.Background(), member.ID)
	if err != nil || !ok {
		t.Fatalf("get pool member ok=%v err=%v", ok, err)
	}
	if savedMember.FailCount != 1 || savedMember.Status != businessproviders.MemberStatusLimited {
		t.Fatalf("pool member health = %#v", savedMember)
	}
	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	job, ok, err := jobStore.Get(context.Background(), "job-pool-fallback", businessimage.DevUserID)
	if err != nil || !ok {
		t.Fatalf("get fallback job ok=%v err=%v", ok, err)
	}
	if job.Status != businessjobs.StatusSucceeded || job.ProviderID != "api_access" || job.ProviderName != "API 接入配置" {
		t.Fatalf("fallback job = %#v", job)
	}
	var savedPayload map[string]any
	if err := json.Unmarshal(job.PayloadJSON, &savedPayload); err != nil {
		t.Fatalf("decode job payload: %v", err)
	}
	if savedPayload["providerSource"] != imageProviderSourceAPIAccess || savedPayload["providerGroupId"] != nil {
		t.Fatalf("saved provider payload = %#v", savedPayload)
	}
}

func TestProviderImageGenerateReportsUnavailablePoolWithoutRequiredAPIAccess(t *testing.T) {
	cfg := newBusinessImageTestConfig(t)
	_, member := seedBusinessProviderPoolMember(t, cfg, "http://127.0.0.1:1", "pool-key")
	store, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	_, found, err := store.UpdateMember(context.Background(), member.ID, businessproviders.MemberInput{
		GroupID:       member.GroupID,
		Name:          member.Name,
		Platform:      member.Platform,
		BaseURL:       member.BaseURL,
		APIKey:        member.APIKey,
		DefaultModel:  member.DefaultModel,
		Enabled:       false,
		Priority:      member.Priority,
		Weight:        member.Weight,
		MaxConcurrent: member.MaxConcurrent,
		Status:        businessproviders.MemberStatusActive,
	})
	if err != nil || !found {
		t.Fatalf("disable pool member found=%v err=%v", found, err)
	}
	_ = store.Close()
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)

	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-pool-unavailable","turnId":"turn-pool-unavailable","jobId":"job-pool-unavailable"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "当前平台没有可用号池成员") {
		t.Fatalf("response should explain unavailable pool, got: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "IMAGE_BASE_URL") {
		t.Fatalf("response should not require API access when pool is configured: %s", rec.Body.String())
	}
}

func TestProviderImageGenerateFailsPoolWithoutFallbackConfig(t *testing.T) {
	poolCalls := 0
	poolUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		poolCalls++
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": map[string]any{"message": "pool upstream failed"},
		})
	}))
	defer poolUpstream.Close()

	cfg := newBusinessImageTestConfig(t)
	_, member := seedBusinessProviderPoolMember(t, cfg, poolUpstream.URL, "pool-key")
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)

	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-pool-no-fallback","turnId":"turn-pool-no-fallback","jobId":"job-pool-no-fallback"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	if poolCalls != 2 {
		t.Fatalf("poolCalls = %d, want retry twice", poolCalls)
	}
	if !strings.Contains(rec.Body.String(), "provider_pool_failed_no_fallback") {
		t.Fatalf("response missing provider_pool_failed_no_fallback: %s", rec.Body.String())
	}
	store, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	defer store.Close()
	savedMember, ok, err := store.GetMember(context.Background(), member.ID)
	if err != nil || !ok {
		t.Fatalf("get pool member ok=%v err=%v", ok, err)
	}
	if savedMember.FailCount != 1 || savedMember.Status != businessproviders.MemberStatusLimited {
		t.Fatalf("pool member health = %#v", savedMember)
	}
}

func TestProviderImageGenerateFailsEmptySuccessfulResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data":    []map[string]any{},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-empty-response","turnId":"turn-empty-response","jobId":"job-empty-response"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "provider_empty_response") {
		t.Fatalf("response missing provider_empty_response: %s", rec.Body.String())
	}
	imageStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business image store: %v", err)
	}
	defer imageStore.Close()
	generations, err := imageStore.ListGenerations(context.Background(), "conv-empty-response", businessimage.DevUserID, 10)
	if err != nil {
		t.Fatalf("list generations: %v", err)
	}
	if len(generations) != 1 || generations[0].Status != businessjobs.StatusFailed || generations[0].Error == "" {
		t.Fatalf("generations = %#v", generations)
	}
	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	job, ok, err := jobStore.Get(context.Background(), "job-empty-response", businessimage.DevUserID)
	if err != nil || !ok {
		t.Fatalf("get job ok=%v err=%v", ok, err)
	}
	if job.Status != businessjobs.StatusFailed || job.ErrorCode != "provider_empty_response" || job.ActualCount != 0 {
		t.Fatalf("job = %#v", job)
	}
}

func TestProviderImageGenerateProxiesGeminiBananaRequest(t *testing.T) {
	var gotAPIKey string
	var gotPath string
	var gotPayload map[string]any
	imageB64 := base64.StdEncoding.EncodeToString([]byte("gemini-image"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-goog-api-key")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode Gemini payload: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "revised cat"},
							{"inlineData": map[string]any{
								"mimeType": "image/png",
								"data":     imageB64,
							}},
						},
					},
				},
			},
		})
	}))
	defer upstream.Close()

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	providerStore, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	_, err = providerStore.Create(context.Background(), businessproviders.MutationInput{
		Name:         "banana",
		Platform:     businessproviders.PlatformGeminiBanana,
		BaseURL:      upstream.URL,
		APIKey:       "banana-key",
		DefaultModel: "gemini-2.5-flash-image",
		Enabled:      true,
		IsDefault:    true,
	})
	_ = providerStore.Close()
	if err != nil {
		t.Fatalf("create gemini provider: %v", err)
	}
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)

	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","model":"gpt-image-2","platform":"gemini-banana","size":"1248x1248","conversationId":"conv-banana","turnId":"turn-banana","jobId":"job-banana"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotAPIKey != "banana-key" {
		t.Fatalf("x-goog-api-key = %q, want banana-key", gotAPIKey)
	}
	if gotPath != "/v1beta/models/gemini-2.5-flash-image:generateContent" {
		t.Fatalf("path = %q", gotPath)
	}
	contents, ok := gotPayload["contents"].([]any)
	if !ok || len(contents) != 1 {
		t.Fatalf("contents = %#v", gotPayload["contents"])
	}
	generationConfig, ok := gotPayload["generationConfig"].(map[string]any)
	if !ok {
		t.Fatalf("generationConfig = %#v", gotPayload["generationConfig"])
	}
	modalities, ok := generationConfig["responseModalities"].([]any)
	if !ok || len(modalities) != 2 || modalities[0] != "TEXT" || modalities[1] != "IMAGE" {
		t.Fatalf("responseModalities = %#v", generationConfig["responseModalities"])
	}
	imageConfig, ok := generationConfig["imageConfig"].(map[string]any)
	if !ok || imageConfig["aspectRatio"] != "1:1" {
		t.Fatalf("imageConfig = %#v", generationConfig["imageConfig"])
	}
	if !strings.Contains(rec.Body.String(), "b64_json") {
		t.Fatalf("response missing b64_json: %s", rec.Body.String())
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	job, ok, err := jobStore.Get(context.Background(), "job-banana", businessimage.DevUserID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if !ok {
		t.Fatalf("job not found")
	}
	if job.Model != "gemini-2.5-flash-image" {
		t.Fatalf("job model = %q", job.Model)
	}

	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business image store: %v", err)
	}
	defer store.Close()
	generations, err := store.ListGenerations(context.Background(), "conv-banana", 10)
	if err != nil {
		t.Fatalf("list generations: %v", err)
	}
	if len(generations) != 1 {
		t.Fatalf("generations len = %d, want 1", len(generations))
	}
	generation := generations[0]
	if generation.Model != "gemini-2.5-flash-image" {
		t.Fatalf("generation model = %q", generation.Model)
	}
	if !strings.Contains(string(generation.Response), `"platform":"gemini-banana"`) {
		t.Fatalf("generation response missing platform: %s", generation.Response)
	}
	if strings.Contains(string(generation.Response), "b64_json") {
		t.Fatalf("generation response should persist image URL instead of b64_json: %s", generation.Response)
	}
}

func TestProviderImageEditProxiesGeminiBananaInlineImageRequest(t *testing.T) {
	var gotPayload map[string]any
	imageB64 := base64.StdEncoding.EncodeToString([]byte("gemini-edit-image"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode Gemini payload: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "edited"},
							{"inlineData": map[string]any{
								"mimeType": "image/png",
								"data":     imageB64,
							}},
						},
					},
				},
			},
		})
	}))
	defer upstream.Close()

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	providerStore, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	_, err = providerStore.Create(context.Background(), businessproviders.MutationInput{
		Name:         "banana",
		Platform:     businessproviders.PlatformGeminiBanana,
		BaseURL:      upstream.URL,
		APIKey:       "banana-key",
		DefaultModel: "gemini-2.5-flash-image",
		Enabled:      true,
		IsDefault:    true,
	})
	_ = providerStore.Close()
	if err != nil {
		t.Fatalf("create gemini provider: %v", err)
	}
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)

	server := NewServer(cfg, nil, nil)
	sourceImage := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("source-image"))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"mode":"edit","prompt":"make it blue","platform":"gemini-banana","conversationId":"conv-gemini-edit","turnId":"turn-gemini-edit","jobId":"job-gemini-edit","sourceImages":[{"id":"src","role":"image","name":"source.png","dataUrl":"`+sourceImage+`"}]}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	contents, ok := gotPayload["contents"].([]any)
	if !ok || len(contents) != 1 {
		t.Fatalf("contents = %#v", gotPayload["contents"])
	}
	firstContent, ok := contents[0].(map[string]any)
	if !ok {
		t.Fatalf("content = %#v", contents[0])
	}
	parts, ok := firstContent["parts"].([]any)
	if !ok || len(parts) < 2 {
		t.Fatalf("parts = %#v", firstContent["parts"])
	}
	var foundInline bool
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		if inline, ok := part["inlineData"].(map[string]any); ok && inline["data"] == base64.StdEncoding.EncodeToString([]byte("source-image")) {
			foundInline = true
		}
	}
	if !foundInline {
		t.Fatalf("Gemini payload missing source inlineData: %#v", parts)
	}
}

func TestImageProviderProxyConfigNormalizesBaseURL(t *testing.T) {
	cfg := newBusinessImageTestConfig(t)
	cfg.APIAccess.Platform = "gpt-image"
	cfg.APIAccess.BaseURL = "https://example.test/v1/"
	cfg.APIAccess.APIKey = "api-access-key"
	server := NewServer(cfg, nil, nil)

	providerCfg, err := server.imageProviderProxyConfig()
	if err != nil {
		t.Fatalf("imageProviderProxyConfig() returned error: %v", err)
	}
	if providerCfg.BaseURL != "https://example.test/v1" {
		t.Fatalf("BaseURL = %q, want https://example.test/v1", providerCfg.BaseURL)
	}

	cfg.APIAccess.BaseURL = "https://example.test/"
	providerCfg, err = server.imageProviderProxyConfig()
	if err != nil {
		t.Fatalf("imageProviderProxyConfig() returned error: %v", err)
	}
	if providerCfg.BaseURL != "https://example.test/v1" {
		t.Fatalf("BaseURL = %q, want https://example.test/v1", providerCfg.BaseURL)
	}
}

func TestProviderImageGenerateRequiresProviderConfig(t *testing.T) {
	server := NewServer(config.New(t.TempDir()), nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/image/generate", strings.NewReader(`{"prompt":"cat"}`))
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestProviderImageGenerateRecordsFailedUpstreamResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": map[string]any{
				"message": "Upstream request failed",
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-failed","turnId":"turn-failed","jobId":"job-failed"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}

	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business image store: %v", err)
	}
	defer store.Close()
	conversation, ok, err := store.GetConversation(context.Background(), "conv-failed")
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if !ok {
		t.Fatal("failed conversation was not recorded")
	}
	if conversation.Title != "生成 · cat" {
		t.Fatalf("conversation title = %q, want 生成 · cat", conversation.Title)
	}
	generations, err := store.ListGenerations(context.Background(), "conv-failed", 10)
	if err != nil {
		t.Fatalf("list generations: %v", err)
	}
	if len(generations) != 1 {
		t.Fatalf("generations len = %d, want 1", len(generations))
	}
	generation := generations[0]
	if generation.Status != "failed" {
		t.Fatalf("generation status = %q, want failed", generation.Status)
	}
	if generation.Error != "Upstream request failed" {
		t.Fatalf("generation error = %q, want Upstream request failed", generation.Error)
	}
	if !strings.Contains(string(generation.Response), "Upstream request failed") {
		t.Fatalf("generation response missing upstream error: %s", generation.Response)
	}
	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("open credit store: %v", err)
	}
	defer creditStore.Close()
	summary, err := creditStore.Summary(context.Background(), businessimage.DevUserID)
	if err != nil {
		t.Fatalf("get credit summary: %v", err)
	}
	if summary.Balance != 5 || summary.Spent != 0 {
		t.Fatalf("credit summary after failed generation = %#v", summary)
	}

	trackerStore, err := businesstracker.NewStore(cfg)
	if err != nil {
		t.Fatalf("open tracker store: %v", err)
	}
	defer trackerStore.Close()
	trackerSummary, err := trackerStore.Summary(context.Background(), 3600)
	if err != nil {
		t.Fatalf("tracker summary: %v", err)
	}
	if trackerSummary.Total != 1 || trackerSummary.Succeeded != 0 || trackerSummary.Failed != 1 {
		t.Fatalf("tracker summary = %#v", trackerSummary)
	}
	if len(trackerSummary.RecentFailures) != 1 {
		t.Fatalf("tracker recent failures = %#v", trackerSummary.RecentFailures)
	}
	failure := trackerSummary.RecentFailures[0]
	if failure.Stage != "upstream" || failure.ErrorCode != "provider_error" || !strings.Contains(failure.ErrorMessage, "Upstream request failed") {
		t.Fatalf("tracker failure = %#v", failure)
	}
	if failure.CreditReserved <= 0 || failure.CreditRefunded != failure.CreditReserved {
		t.Fatalf("tracker credit fields = reserved:%d refunded:%d", failure.CreditReserved, failure.CreditRefunded)
	}
}

func TestProviderImageGenerateRetriesTransientUpstreamError(t *testing.T) {
	attempts := 0
	imageB64 := base64.StdEncoding.EncodeToString([]byte("retry-image"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error": map[string]any{"message": "temporary upstream failure"},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "cat"},
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","conversationId":"conv-retry","turnId":"turn-retry","jobId":"job-retry"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if attempts != 2 {
		t.Fatalf("upstream attempts = %d, want 2", attempts)
	}
	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer jobStore.Close()
	job, ok, err := jobStore.Get(context.Background(), "job-retry", businessimage.DevUserID)
	if err != nil || !ok {
		t.Fatalf("get retry job ok=%v err=%v", ok, err)
	}
	if job.Status != businessjobs.StatusSucceeded {
		t.Fatalf("retry job status = %q", job.Status)
	}
}

func TestProviderImageGenerateDoesNotOverwriteFinalizedStaleJob(t *testing.T) {
	imageB64 := base64.StdEncoding.EncodeToString([]byte("image"))
	upstreamEntered := make(chan struct{})
	releaseUpstream := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(upstreamEntered)
		<-releaseUpstream
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{{"b64_json": imageB64}},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)
	done := make(chan providerImageGenerateResult, 1)
	go func() {
		done <- server.executeProviderImageGenerate(providerImageGenerateExecution{
			Context:   context.Background(),
			UserID:    businessimage.DevUserID,
			StartedAt: time.Now().UTC(),
			Payload: map[string]any{
				"prompt":         "cat",
				"conversationId": "conv-finalized",
				"turnId":         "turn-finalized",
				"jobId":          "job-finalized",
			},
		})
	}()

	select {
	case <-upstreamEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream was not reached")
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	job, ok, err := jobStore.Get(context.Background(), "job-finalized", businessimage.DevUserID)
	if err != nil || !ok {
		t.Fatalf("get running job ok=%v err=%v", ok, err)
	}
	job.Status = businessjobs.StatusFailed
	job.Stage = "stale"
	job.ErrorCode = "stale_running"
	job.ErrorMessage = "任务超时，已自动标记失败"
	if _, err := jobStore.Save(context.Background(), job); err != nil {
		t.Fatalf("save finalized job: %v", err)
	}
	if err := jobStore.Close(); err != nil {
		t.Fatalf("close job store: %v", err)
	}
	imageStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open image store: %v", err)
	}
	if _, err := imageStore.MarkGenerationFinished(context.Background(), businessimage.DevUserID, "job-finalized", businessjobs.StatusFailed, "任务超时，已自动标记失败"); err != nil {
		t.Fatalf("mark generation failed: %v", err)
	}
	if err := imageStore.Close(); err != nil {
		t.Fatalf("close image store: %v", err)
	}
	close(releaseUpstream)

	select {
	case result := <-done:
		if result.StatusCode != http.StatusConflict {
			t.Fatalf("result status = %d, want %d", result.StatusCode, http.StatusConflict)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("generation did not finish")
	}

	imageStore, err = businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("reopen image store: %v", err)
	}
	generations, err := imageStore.ListGenerations(context.Background(), "conv-finalized", 10)
	if err != nil {
		t.Fatalf("list generations: %v", err)
	}
	if len(generations) != 1 || generations[0].Status != businessjobs.StatusFailed || len(generations[0].Response) != 0 {
		t.Fatalf("generation after late success = %#v", generations)
	}
	if err := imageStore.Close(); err != nil {
		t.Fatalf("close image store after check: %v", err)
	}
}

func TestProviderImageGenerateCancelBeforeReserveDoesNotChargeOrReachUpstream(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString([]byte("image"))}},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 5)
	server := NewServer(cfg, nil, nil)

	result := server.executeProviderImageGenerate(providerImageGenerateExecution{
		Context:   context.Background(),
		UserID:    businessimage.DevUserID,
		StartedAt: time.Now().UTC(),
		Payload: map[string]any{
			"prompt":         "cat",
			"conversationId": "conv-cancel-before-reserve",
			"turnId":         "turn-cancel-before-reserve",
			"jobId":          "job-cancel-before-reserve",
		},
		AfterRunningMarked: func() {
			store, err := businessjobs.NewStore(cfg)
			if err != nil {
				t.Fatalf("open job store in hook: %v", err)
			}
			defer store.Close()
			if _, _, err := store.RequestCancel(context.Background(), "job-cancel-before-reserve", businessimage.DevUserID); err != nil {
				t.Fatalf("request cancel in hook: %v", err)
			}
		},
	})

	if result.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body = %s", result.StatusCode, http.StatusConflict, string(result.Body))
	}
	if upstreamCalled {
		t.Fatal("upstream should not be called after cancellation before reserve")
	}

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("open credit store: %v", err)
	}
	summary, err := creditStore.Summary(context.Background(), businessimage.DevUserID)
	if err != nil {
		t.Fatalf("credit summary: %v", err)
	}
	totals, err := creditStore.GenerationTotals(context.Background(), businessimage.DevUserID, "job-cancel-before-reserve")
	if err != nil {
		t.Fatalf("generation totals: %v", err)
	}
	if err := creditStore.Close(); err != nil {
		t.Fatalf("close credit store: %v", err)
	}
	if summary.Balance != 5 || summary.Spent != 0 {
		t.Fatalf("credit summary = %#v, want unchanged balance 5 spent 0", summary)
	}
	if totals.Reserved != 0 || totals.Refunded != 0 {
		t.Fatalf("generation totals = %#v, want no ledger entries", totals)
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("open job store after cancel: %v", err)
	}
	job, ok, err := jobStore.Get(context.Background(), "job-cancel-before-reserve", businessimage.DevUserID)
	if err != nil || !ok {
		t.Fatalf("get job after cancel ok=%v err=%v", ok, err)
	}
	if err := jobStore.Close(); err != nil {
		t.Fatalf("close job store after cancel: %v", err)
	}
	if job.Status != businessjobs.StatusCancelled || job.CreditRefunded != 0 {
		t.Fatalf("job after cancel = %#v, want cancelled without credit ledger", job)
	}

	imageStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open image store: %v", err)
	}
	generations, err := imageStore.ListGenerations(context.Background(), "conv-cancel-before-reserve", businessimage.DevUserID, 10)
	if err != nil {
		t.Fatalf("list generations: %v", err)
	}
	if err := imageStore.Close(); err != nil {
		t.Fatalf("close image store: %v", err)
	}
	if len(generations) != 1 || generations[0].Status != businessjobs.StatusCancelled {
		t.Fatalf("generations after cancel = %#v, want one cancelled generation", generations)
	}
}

func TestProviderImageGenerateUsesImageAdmissionGate(t *testing.T) {
	imageB64 := base64.StdEncoding.EncodeToString([]byte("image"))
	upstreamEntered := make(chan struct{}, 2)
	releaseUpstream := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamEntered <- struct{}{}
		<-releaseUpstream
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{{"b64_json": imageB64}},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Server.MaxImageConcurrency = 1
	cfg.Server.ImageQueueLimit = 2
	cfg.Server.ImageQueueTimeoutSeconds = 2
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 10)
	server := NewServer(cfg, nil, nil)

	runRequest := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/image/generate", strings.NewReader(body))
		rec := httptest.NewRecorder()
		server.handleProviderImageGenerate(rec, req)
		return rec
	}

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstDone <- runRequest(`{"prompt":"first","conversationId":"conv-admission","turnId":"turn-1","jobId":"job-1"}`)
	}()

	select {
	case <-upstreamEntered:
	case <-time.After(time.Second):
		t.Fatal("first request did not enter upstream")
	}
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		close(releaseUpstream)
		t.Fatalf("open business image store: %v", err)
	}
	generations, err := store.ListGenerations(context.Background(), "conv-admission", businessimage.DevUserID, 10)
	_ = store.Close()
	if err != nil {
		close(releaseUpstream)
		t.Fatalf("list generations while running: %v", err)
	}
	if len(generations) != 1 || generations[0].ID != "job-1" || generations[0].Status != "running" {
		close(releaseUpstream)
		t.Fatalf("running generation = %#v", generations)
	}

	secondDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		secondDone <- runRequest(`{"prompt":"second","conversationId":"conv-admission","turnId":"turn-2","jobId":"job-2"}`)
	}()

	select {
	case <-upstreamEntered:
		t.Fatal("second request entered upstream before admission slot was released")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseUpstream)

	select {
	case rec := <-firstDone:
		if rec.Code != http.StatusOK {
			t.Fatalf("first status = %d, body = %s", rec.Code, rec.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("first request did not finish")
	}
	store, err = businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business image store after first done: %v", err)
	}
	generations, err = store.ListGenerations(context.Background(), "conv-admission", businessimage.DevUserID, 10)
	_ = store.Close()
	if err != nil {
		t.Fatalf("list generations after first done: %v", err)
	}
	if len(generations) == 0 || generations[0].ID != "job-1" || generations[0].Status != "succeeded" {
		t.Fatalf("completed generation = %#v", generations)
	}
	select {
	case <-upstreamEntered:
	case <-time.After(time.Second):
		t.Fatal("second request did not enter upstream after slot release")
	}
	select {
	case rec := <-secondDone:
		if rec.Code != http.StatusOK {
			t.Fatalf("second status = %d, body = %s", rec.Code, rec.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("second request did not finish")
	}
}

func TestProviderImageGenerateReturnsQueueFull(t *testing.T) {
	imageB64 := base64.StdEncoding.EncodeToString([]byte("image"))
	upstreamEntered := make(chan struct{}, 2)
	releaseUpstream := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamEntered <- struct{}{}
		<-releaseUpstream
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{{"b64_json": imageB64}},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	cfg.Server.MaxImageConcurrency = 1
	cfg.Server.ImageQueueLimit = 1
	cfg.Server.ImageQueueTimeoutSeconds = 2
	seedBusinessCredit(t, cfg, businessimage.DevUserID, 10)
	server := NewServer(cfg, nil, nil)

	runRequest := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/image/generate", strings.NewReader(body))
		rec := httptest.NewRecorder()
		server.handleProviderImageGenerate(rec, req)
		return rec
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rec := runRequest(`{"prompt":"first","conversationId":"conv-admission-full","turnId":"turn-1","jobId":"job-1"}`)
		if rec.Code != http.StatusOK {
			t.Errorf("first status = %d, body = %s", rec.Code, rec.Body.String())
		}
	}()
	select {
	case <-upstreamEntered:
	case <-time.After(time.Second):
		t.Fatal("first request did not enter upstream")
	}

	queuedDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		queuedDone <- runRequest(`{"prompt":"queued","conversationId":"conv-admission-full","turnId":"turn-2","jobId":"job-2"}`)
	}()
	time.Sleep(100 * time.Millisecond)

	rejected := runRequest(`{"prompt":"rejected","conversationId":"conv-admission-full","turnId":"turn-3","jobId":"job-3"}`)
	if rejected.Code != http.StatusTooManyRequests {
		close(releaseUpstream)
		t.Fatalf("rejected status = %d, want %d, body = %s", rejected.Code, http.StatusTooManyRequests, rejected.Body.String())
	}
	if !strings.Contains(rejected.Body.String(), "image_queue_full") {
		close(releaseUpstream)
		t.Fatalf("queue full response missing code: %s", rejected.Body.String())
	}

	close(releaseUpstream)
	select {
	case rec := <-queuedDone:
		if rec.Code != http.StatusOK {
			t.Fatalf("queued status = %d, body = %s", rec.Code, rec.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("queued request did not finish")
	}
	wg.Wait()
}

func TestProviderImageGenerateRequiresBusinessCredits(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString([]byte("image"))}},
		})
	}))
	defer upstream.Close()

	t.Setenv("IMAGE_PROVIDER", "openai_compatible")
	t.Setenv("IMAGE_BASE_URL", upstream.URL)
	t.Setenv("IMAGE_API_KEY", "provider-key")
	t.Setenv("IMAGE_MODEL", "gpt-image-test")

	cfg := newBusinessImageTestConfig(t)
	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/image/generate",
		strings.NewReader(`{"prompt":"cat","n":1,"conversationId":"conv-credit","turnId":"turn-credit","jobId":"job-credit"}`),
	)
	rec := httptest.NewRecorder()

	server.handleProviderImageGenerate(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusPaymentRequired, rec.Body.String())
	}
	if upstreamCalled {
		t.Fatal("upstream should not be called when balance is insufficient")
	}
}

func TestBusinessImageConversationHandlersReadRecordedGenerations(t *testing.T) {
	cfg := newBusinessImageTestConfig(t)
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_, err = store.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "conv-1",
		UserID: businessimage.DevUserID,
		Title:  "Cat session",
	})
	if err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	_, err = store.SaveGeneration(context.Background(), businessimage.Generation{
		ID:             "job-1",
		UserID:         businessimage.DevUserID,
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		Prompt:         "cat",
		Model:          "gpt-image-test",
		Count:          1,
		Status:         "succeeded",
		Response:       json.RawMessage(`{"data":[{"b64_json":"aW1hZ2U="}]}`),
	})
	if err != nil {
		t.Fatalf("save generation: %v", err)
	}
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	listReq := httptest.NewRequest(http.MethodGet, "/api/business/image/conversations", nil)
	listRec := httptest.NewRecorder()
	server.handleListBusinessImageConversations(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"id":"conv-1"`) {
		t.Fatalf("list body missing conversation: %s", listRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/business/image/conversations/conv-1", nil)
	getReq.SetPathValue("id", "conv-1")
	getRec := httptest.NewRecorder()
	server.handleGetBusinessImageConversation(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
	body := getRec.Body.String()
	for _, expected := range []string{`"id":"conv-1"`, `"id":"job-1"`, `"prompt":"cat"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("get body missing %s: %s", expected, body)
		}
	}
}

func TestBusinessImageConversationHandlersUseLoggedInUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_, err = store.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "admin-conv",
		UserID: "dev_admin",
		Title:  "Admin session",
	})
	if err != nil {
		t.Fatalf("save admin conversation: %v", err)
	}
	_, err = store.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "user-conv",
		UserID: "dev_user",
		Title:  "User session",
	})
	if err != nil {
		t.Fatalf("save user conversation: %v", err)
	}
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	adminToken := loginTestToken(t, server, "owner", "owner-pass")
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	adminReq := httptest.NewRequest(http.MethodGet, "/api/business/image/conversations", nil)
	adminReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusOK {
		t.Fatalf("admin list status = %d, body = %s", adminRec.Code, adminRec.Body.String())
	}
	if !strings.Contains(adminRec.Body.String(), `"id":"admin-conv"`) || strings.Contains(adminRec.Body.String(), `"id":"user-conv"`) {
		t.Fatalf("admin list body not isolated: %s", adminRec.Body.String())
	}

	userReq := httptest.NewRequest(http.MethodGet, "/api/business/image/conversations", nil)
	userReq.Header.Set("Authorization", "Bearer "+userToken)
	userRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(userRec, userReq)
	if userRec.Code != http.StatusOK {
		t.Fatalf("user list status = %d, body = %s", userRec.Code, userRec.Body.String())
	}
	if !strings.Contains(userRec.Body.String(), `"id":"user-conv"`) || strings.Contains(userRec.Body.String(), `"id":"admin-conv"`) {
		t.Fatalf("user list body not isolated: %s", userRec.Body.String())
	}

	crossReq := httptest.NewRequest(http.MethodGet, "/api/business/image/conversations/admin-conv", nil)
	crossReq.SetPathValue("id", "admin-conv")
	crossReq.Header.Set("Authorization", "Bearer "+userToken)
	crossRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusNotFound {
		t.Fatalf("cross get status = %d, want %d, body = %s", crossRec.Code, http.StatusNotFound, crossRec.Body.String())
	}
}

func TestBusinessImageConversationRenameIsScopedToLoggedInUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_, err = store.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "admin-conv",
		UserID: "dev_admin",
		Title:  "Admin session",
	})
	if err != nil {
		t.Fatalf("save admin conversation: %v", err)
	}
	_, err = store.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "user-conv",
		UserID: "dev_user",
		Title:  "User session",
	})
	if err != nil {
		t.Fatalf("save user conversation: %v", err)
	}
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	renameReq := httptest.NewRequest(http.MethodPatch, "/api/business/image/conversations/user-conv", strings.NewReader(`{"title":"  旅行灵感  "}`))
	renameReq.SetPathValue("id", "user-conv")
	renameReq.Header.Set("Authorization", "Bearer "+userToken)
	renameRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(renameRec, renameReq)
	if renameRec.Code != http.StatusOK {
		t.Fatalf("rename status = %d, body = %s", renameRec.Code, renameRec.Body.String())
	}
	if !strings.Contains(renameRec.Body.String(), `"title":"旅行灵感"`) {
		t.Fatalf("rename body missing trimmed title: %s", renameRec.Body.String())
	}

	crossReq := httptest.NewRequest(http.MethodPatch, "/api/business/image/conversations/admin-conv", strings.NewReader(`{"title":"Wrong user"}`))
	crossReq.SetPathValue("id", "admin-conv")
	crossReq.Header.Set("Authorization", "Bearer "+userToken)
	crossRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusNotFound {
		t.Fatalf("cross rename status = %d, want %d, body = %s", crossRec.Code, http.StatusNotFound, crossRec.Body.String())
	}

	verifyStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open verify store: %v", err)
	}
	defer verifyStore.Close()
	userConversation, ok, err := verifyStore.GetConversation(context.Background(), "user-conv", "dev_user")
	if err != nil || !ok {
		t.Fatalf("user conversation missing after rename, ok=%v err=%v", ok, err)
	}
	if userConversation.Title != "旅行灵感" {
		t.Fatalf("user title = %q, want %q", userConversation.Title, "旅行灵感")
	}
	adminConversation, ok, err := verifyStore.GetConversation(context.Background(), "admin-conv", "dev_admin")
	if err != nil || !ok {
		t.Fatalf("admin conversation missing after cross rename, ok=%v err=%v", ok, err)
	}
	if adminConversation.Title != "Admin session" {
		t.Fatalf("admin title changed to %q", adminConversation.Title)
	}
}

func TestBusinessImageConversationDeleteIsScopedToLoggedInUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	adminFile := seedBusinessImageFile(t, cfg, "business-admin.png")
	userFile := seedBusinessImageFile(t, cfg, "business-user.png")
	saveBusinessConversationWithGeneration(t, store, "admin-conv", "admin-task", "dev_admin", "business-admin.png")
	saveBusinessConversationWithGeneration(t, store, "user-conv", "user-task", "dev_user", "business-user.png")
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	crossReq := httptest.NewRequest(http.MethodDelete, "/api/business/image/conversations/admin-conv", nil)
	crossReq.SetPathValue("id", "admin-conv")
	crossReq.Header.Set("Authorization", "Bearer "+userToken)
	crossRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusNotFound {
		t.Fatalf("cross delete status = %d, want %d, body = %s", crossRec.Code, http.StatusNotFound, crossRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/business/image/conversations/user-conv", nil)
	deleteReq.SetPathValue("id", "user-conv")
	deleteReq.Header.Set("Authorization", "Bearer "+userToken)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	verifyStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open verify store: %v", err)
	}
	defer verifyStore.Close()
	if _, ok, err := verifyStore.GetConversation(context.Background(), "admin-conv", "dev_admin"); err != nil || !ok {
		t.Fatalf("admin conversation should remain, ok=%v err=%v", ok, err)
	}
	if _, ok, err := verifyStore.GetConversation(context.Background(), "user-conv", "dev_user"); err != nil || ok {
		t.Fatalf("user conversation should be deleted, ok=%v err=%v", ok, err)
	}
	generations, err := verifyStore.ListGenerations(context.Background(), "user-conv", "dev_user", 10)
	if err != nil {
		t.Fatalf("list deleted user generations: %v", err)
	}
	if len(generations) != 0 {
		t.Fatalf("deleted user generations len = %d, want 0", len(generations))
	}
	if _, err := os.Stat(adminFile); err != nil {
		t.Fatalf("admin file should remain: %v", err)
	}
	if _, err := os.Stat(userFile); !os.IsNotExist(err) {
		t.Fatalf("user file should be deleted, err=%v", err)
	}
}

func TestBusinessImageConversationClearIsScopedToLoggedInUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	adminFile := seedBusinessImageFile(t, cfg, "business-admin-clear.png")
	userFile1 := seedBusinessImageFile(t, cfg, "business-user-clear-1.png")
	userFile2 := seedBusinessImageFile(t, cfg, "business-user-clear-2.png")
	saveBusinessConversationWithGeneration(t, store, "admin-conv-1", "admin-job-1", "dev_admin", "business-admin-clear.png")
	saveBusinessConversationWithGeneration(t, store, "admin-conv-2", "admin-job-2", "dev_admin", "business-admin-clear.png")
	saveBusinessConversationWithGeneration(t, store, "user-conv-1", "user-job-1", "dev_user", "business-user-clear-1.png")
	saveBusinessConversationWithGeneration(t, store, "user-conv-2", "user-job-2", "dev_user", "business-user-clear-2.png")
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	clearReq := httptest.NewRequest(http.MethodDelete, "/api/business/image/conversations", nil)
	clearReq.Header.Set("Authorization", "Bearer "+userToken)
	clearRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(clearRec, clearReq)
	if clearRec.Code != http.StatusOK {
		t.Fatalf("clear status = %d, body = %s", clearRec.Code, clearRec.Body.String())
	}

	verifyStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open verify store: %v", err)
	}
	defer verifyStore.Close()
	adminItems, err := verifyStore.ListConversations(context.Background(), "dev_admin", 10)
	if err != nil {
		t.Fatalf("list admin conversations: %v", err)
	}
	if len(adminItems) != 2 {
		t.Fatalf("admin conversations len = %d, want 2", len(adminItems))
	}
	userItems, err := verifyStore.ListConversations(context.Background(), "dev_user", 10)
	if err != nil {
		t.Fatalf("list user conversations: %v", err)
	}
	if len(userItems) != 0 {
		t.Fatalf("user conversations len = %d, want 0", len(userItems))
	}
	if _, err := os.Stat(adminFile); err != nil {
		t.Fatalf("admin file should remain: %v", err)
	}
	for _, file := range []string{userFile1, userFile2} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatalf("user file should be deleted, file=%s err=%v", file, err)
		}
	}
}

func TestBusinessImageConversationDeleteKeepsSharedImageFile(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	sharedFile := seedBusinessImageFile(t, cfg, "business-shared.png")
	saveBusinessConversationWithGeneration(t, store, "conv-a", "job-a", "dev_user", "business-shared.png")
	saveBusinessConversationWithGeneration(t, store, "conv-b", "job-b", "dev_user", "business-shared.png")
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/business/image/conversations/conv-a", nil)
	deleteReq.SetPathValue("id", "conv-a")
	deleteReq.Header.Set("Authorization", "Bearer "+userToken)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	if _, err := os.Stat(sharedFile); err != nil {
		t.Fatalf("shared file should remain: %v", err)
	}
}

func TestBusinessImageConversationDeleteRemovesLegacyImageDirFile(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	legacyPath := seedBusinessImageFileInDir(t, cfg, legacyImageDir, "business-legacy-delete.png")
	saveBusinessConversationWithGeneration(t, store, "legacy-conv", "legacy-task", "dev_user", "business-legacy-delete.png")
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/business/image/conversations/legacy-conv", nil)
	deleteReq.SetPathValue("id", "legacy-conv")
	deleteReq.Header.Set("Authorization", "Bearer "+userToken)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy file should be deleted, err=%v", err)
	}
}

func TestBusinessImageConversationDeleteRemovesAssetRecord(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	fileName := "business-asset-delete.png"
	filePath := seedBusinessImageFile(t, cfg, fileName)
	saveBusinessConversationWithGeneration(t, store, "asset-conv", "asset-task", "dev_user", fileName)
	if _, err := store.SaveAsset(context.Background(), businessimage.Asset{
		ID:             fileName,
		UserID:         "dev_user",
		ConversationID: "asset-conv",
		GenerationID:   "asset-task",
		FileName:       fileName,
		FilePath:       filePath,
		URL:            "/v1/files/image/" + fileName,
		MimeType:       "image/png",
		SizeBytes:      5,
		SHA256:         "asset-sha",
	}); err != nil {
		t.Fatalf("save asset: %v", err)
	}
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/business/image/conversations/asset-conv", nil)
	deleteReq.SetPathValue("id", "asset-conv")
	deleteReq.Header.Set("Authorization", "Bearer "+userToken)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("asset file should be deleted, err=%v", err)
	}

	verifyStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open verify store: %v", err)
	}
	defer verifyStore.Close()
	owners, err := verifyStore.AssetOwnerUserIDs(context.Background(), fileName)
	if err != nil {
		t.Fatalf("list asset owners: %v", err)
	}
	if len(owners) != 0 {
		t.Fatalf("asset owners after delete = %#v, want empty", owners)
	}
}

func TestBusinessImageFileAccessRequiresOwnerSession(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	seedBusinessImageFile(t, cfg, "business-admin-image.png")
	seedBusinessImageFile(t, cfg, "business-user-image.png")
	saveBusinessConversationWithGeneration(t, store, "admin-conv", "admin-task", "dev_admin", "business-admin-image.png")
	saveBusinessConversationWithGeneration(t, store, "user-conv", "user-task", "dev_user", "business-user-image.png")
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	adminToken := loginTestToken(t, server, "owner", "owner-pass")
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	unauthorizedReq := httptest.NewRequest(http.MethodGet, "/v1/files/image/business-user-image.png", nil)
	unauthorizedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorizedRec, unauthorizedReq)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized image status = %d, want %d", unauthorizedRec.Code, http.StatusUnauthorized)
	}

	crossReq := httptest.NewRequest(http.MethodGet, "/v1/files/image/business-admin-image.png", nil)
	crossReq.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: userToken})
	crossRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("cross image status = %d, want %d, body = %s", crossRec.Code, http.StatusForbidden, crossRec.Body.String())
	}

	ownerReq := httptest.NewRequest(http.MethodGet, "/v1/files/image/business-user-image.png", nil)
	ownerReq.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: userToken})
	ownerRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(ownerRec, ownerReq)
	if ownerRec.Code != http.StatusOK {
		t.Fatalf("owner image status = %d, want %d, body = %s", ownerRec.Code, http.StatusOK, ownerRec.Body.String())
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/v1/files/image/business-user-image.png", nil)
	adminReq.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: adminToken})
	adminRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusOK {
		t.Fatalf("admin image status = %d, want %d, body = %s", adminRec.Code, http.StatusOK, adminRec.Body.String())
	}
}

func TestBusinessImageFileAccessUsesAssetTableOwner(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	fileName := "business-asset-only.png"
	filePath := seedBusinessImageFile(t, cfg, fileName)
	if _, err := store.SaveAsset(context.Background(), businessimage.Asset{
		ID:             fileName,
		UserID:         "dev_user",
		ConversationID: "asset-only-conv",
		GenerationID:   "asset-only-task",
		FileName:       fileName,
		FilePath:       filePath,
		URL:            "/v1/files/image/" + fileName,
		MimeType:       "image/png",
		SizeBytes:      5,
		SHA256:         "asset-only-sha",
	}); err != nil {
		t.Fatalf("save asset: %v", err)
	}
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	req := httptest.NewRequest(http.MethodGet, "/v1/files/image/"+fileName, nil)
	req.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: userToken})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("asset table image status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestBusinessStorageReportFindsInconsistencies(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	okPath := seedBusinessImageFile(t, cfg, "business-ok.png")
	orphanPath := seedBusinessImageFile(t, cfg, "business-orphan.png")
	legacyPath := seedBusinessImageFile(t, cfg, "business-legacy-report.png")
	saveBusinessConversationWithGeneration(t, store, "ok-conv", "ok-task", "dev_user", "business-ok.png")
	if _, err := store.SaveAsset(context.Background(), businessimage.Asset{
		ID:             "business-ok.png",
		UserID:         "dev_user",
		ConversationID: "ok-conv",
		GenerationID:   "ok-task",
		FileName:       "business-ok.png",
		FilePath:       okPath,
		URL:            "/v1/files/image/business-ok.png",
		MimeType:       "image/png",
		SizeBytes:      5,
		SHA256:         "ok-sha",
	}); err != nil {
		t.Fatalf("save ok asset: %v", err)
	}
	if _, err := store.SaveAsset(context.Background(), businessimage.Asset{
		ID:             "business-missing.png",
		UserID:         "dev_user",
		ConversationID: "missing-conv",
		GenerationID:   "missing-task",
		FileName:       "business-missing.png",
		FilePath:       filepath.Join(cfg.ResolvePath(cfg.Storage.ImageDir), "business-missing.png"),
		URL:            "/v1/files/image/business-missing.png",
		MimeType:       "image/png",
		SizeBytes:      123,
		SHA256:         "missing-sha",
	}); err != nil {
		t.Fatalf("save missing asset: %v", err)
	}
	saveBusinessConversationWithGeneration(t, store, "legacy-conv", "legacy-task", "dev_user", "business-legacy-report.png")
	if _, err := store.SaveAsset(context.Background(), businessimage.Asset{
		ID:             "business-broken.png",
		UserID:         "dev_user",
		ConversationID: "broken-conv",
		GenerationID:   "broken-task",
		FileName:       "business-broken.png",
		FilePath:       filepath.Join(cfg.ResolvePath(cfg.Storage.ImageDir), "business-broken.png"),
		URL:            "/v1/files/image/business-broken.png",
		MimeType:       "image/png",
		SizeBytes:      456,
		SHA256:         "broken-sha",
	}); err != nil {
		t.Fatalf("save broken asset: %v", err)
	}
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	adminToken := loginTestToken(t, server, "owner", "owner-pass")
	req := httptest.NewRequest(http.MethodGet, "/api/business/storage/report", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("report status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var report businessStorageReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Summary.AssetFiles != 3 ||
		report.Summary.DiskFiles != 3 ||
		report.Summary.MissingFiles != 2 ||
		report.Summary.OrphanFiles != 1 ||
		report.Summary.LegacyReferencedFiles != 1 ||
		report.Summary.BrokenAssets != 2 {
		t.Fatalf("report summary = %#v", report.Summary)
	}
	if !reportHasMissingFile(report, "business-missing.png") || !reportHasMissingFile(report, "business-broken.png") {
		t.Fatalf("missing files = %#v", report.MissingFiles)
	}
	if len(report.OrphanFiles) != 1 || report.OrphanFiles[0].FileName != "business-orphan.png" || report.OrphanFiles[0].Path != orphanPath {
		t.Fatalf("orphan files = %#v", report.OrphanFiles)
	}
	if len(report.LegacyReferencedFiles) != 1 ||
		report.LegacyReferencedFiles[0].FileName != "business-legacy-report.png" ||
		!report.LegacyReferencedFiles[0].OnDisk ||
		report.LegacyReferencedFiles[0].GenerationID != "legacy-task" ||
		legacyPath == "" {
		t.Fatalf("legacy references = %#v", report.LegacyReferencedFiles)
	}
	if !reportHasBrokenAsset(report, "business-missing.png") || !reportHasBrokenAsset(report, "business-broken.png") {
		t.Fatalf("broken assets = %#v", report.BrokenAssets)
	}
}

func TestBackfillBusinessStorageAssets(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	legacyPath := seedBusinessImageFile(t, cfg, "business-legacy-backfill.png")
	saveBusinessConversationWithGeneration(t, store, "legacy-conv", "legacy-task", "dev_user", "business-legacy-backfill.png")
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	adminToken := loginTestToken(t, server, "owner", "owner-pass")
	req := httptest.NewRequest(http.MethodPost, "/api/business/storage/backfill-assets", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("backfill status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var payload businessStorageBackfillResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode backfill response: %v", err)
	}
	if payload.Result.Matched != 1 || payload.Result.Backfilled != 1 || payload.Report.Summary.LegacyReferencedFiles != 0 {
		t.Fatalf("backfill payload = %#v", payload)
	}

	store, err = businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store.Close()
	owners, err := store.AssetOwnerUserIDs(context.Background(), "business-legacy-backfill.png")
	if err != nil {
		t.Fatalf("list asset owners: %v", err)
	}
	if len(owners) != 1 || owners[0] != "dev_user" {
		t.Fatalf("asset owners = %#v", owners)
	}
	assetFiles, err := store.AssetFileNamesForConversation(context.Background(), "legacy-conv", "dev_user")
	if err != nil {
		t.Fatalf("list asset files: %v", err)
	}
	if len(assetFiles) != 1 || assetFiles[0] != "business-legacy-backfill.png" || legacyPath == "" {
		t.Fatalf("asset files = %#v", assetFiles)
	}
}

func TestRequireUIAuthRefreshesSessionCookieForImageRequests(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newBusinessImageTestConfig(t)
	cfg.Storage.ImageDir = "data/business-images"
	store, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	seedBusinessImageFile(t, cfg, "business-user-cookie.png")
	saveBusinessConversationWithGeneration(t, store, "user-conv", "user-task", "dev_user", "business-user-cookie.png")
	_ = store.Close()

	server := NewServer(cfg, nil, nil)
	userToken := loginTestToken(t, server, "tester", "tester-pass")

	listReq := httptest.NewRequest(http.MethodGet, "/api/business/image/conversations", nil)
	listReq.Header.Set("Authorization", "Bearer "+userToken)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	cookie := findCookie(listRec.Result().Cookies(), authSessionCookieName)
	if cookie == nil || cookie.Value == "" {
		t.Fatalf("session cookie was not refreshed: %#v", listRec.Result().Cookies())
	}

	imageReq := httptest.NewRequest(http.MethodGet, "/v1/files/image/business-user-cookie.png", nil)
	imageReq.AddCookie(cookie)
	imageRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(imageRec, imageReq)
	if imageRec.Code != http.StatusOK {
		t.Fatalf("image status = %d, want %d, body = %s", imageRec.Code, http.StatusOK, imageRec.Body.String())
	}
}

func saveBusinessConversationWithGeneration(t *testing.T, store *businessimage.Store, conversationID, generationID, userID string, fileName ...string) {
	t.Helper()
	_, err := store.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     conversationID,
		UserID: userID,
		Title:  conversationID,
	})
	if err != nil {
		t.Fatalf("save conversation %s: %v", conversationID, err)
	}
	response := json.RawMessage(`{"data":[{"b64_json":"aW1hZ2U="}]}`)
	if len(fileName) > 0 && strings.TrimSpace(fileName[0]) != "" {
		response = json.RawMessage(`{"data":[{"url":"/v1/files/image/` + strings.TrimSpace(fileName[0]) + `"}]}`)
	}
	_, err = store.SaveGeneration(context.Background(), businessimage.Generation{
		ID:             generationID,
		UserID:         userID,
		ConversationID: conversationID,
		TurnID:         generationID + "-turn",
		Prompt:         "cat",
		Model:          "gpt-image-test",
		Count:          1,
		Status:         "succeeded",
		Response:       response,
	})
	if err != nil {
		t.Fatalf("save generation %s: %v", generationID, err)
	}
}

func seedBusinessCredit(t *testing.T, cfg *config.Config, userID string, balance int64) {
	t.Helper()
	store, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("open credit store: %v", err)
	}
	defer store.Close()
	if _, _, err := store.SetBalance(context.Background(), userID, balance, businesscredits.ReasonAdminAdjustment); err != nil {
		t.Fatalf("seed credit: %v", err)
	}
}

func seedBusinessSubscription(t *testing.T, cfg *config.Config, userID string, credits int64) {
	t.Helper()
	authStore, err := businessauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	if err := authStore.EnsureBootstrapUser(context.Background(), businessauth.BootstrapUser{
		ID:       userID,
		Username: userID,
		Email:    userID + "@example.test",
		Password: "test-password",
		Role:     businessauth.RoleUser,
	}); err != nil {
		t.Fatalf("ensure subscription user: %v", err)
	}
	if err := authStore.Close(); err != nil {
		t.Fatalf("close auth store: %v", err)
	}

	store, err := businesspayments.NewStore(cfg)
	if err != nil {
		t.Fatalf("open payment store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()
	pkg, err := store.CreatePackage(ctx, businesspayments.PackageInput{
		PackageType:  businesspayments.PackageTypeSubscription,
		Name:         "测试订阅",
		AmountCents:  990,
		Credits:      credits,
		DurationDays: 30,
		Currency:     "CNY",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create subscription package: %v", err)
	}
	order, err := store.CreateOrder(ctx, businesspayments.CreateOrderInput{
		UserID:    userID,
		Username:  userID,
		UserEmail: userID + "@example.test",
		PackageID: pkg.ID,
	})
	if err != nil {
		t.Fatalf("create subscription order: %v", err)
	}
	if _, err := store.CompleteOrder(ctx, businesspayments.CompleteOrderInput{OrderID: order.ID, Operator: "test_admin"}); err != nil {
		t.Fatalf("complete subscription order: %v", err)
	}
}

func saveBusinessSystemSettings(t *testing.T, cfg *config.Config, mutate func(*businesssettings.Settings)) businesssettings.Settings {
	t.Helper()
	store, err := businesssettings.NewStore(cfg)
	if err != nil {
		t.Fatalf("open settings store: %v", err)
	}
	defer store.Close()
	settings := businesssettings.WithConfigRuntime(businesssettings.Defaults(), cfg)
	if mutate != nil {
		mutate(&settings)
	}
	saved, err := store.Save(context.Background(), settings)
	if err != nil {
		t.Fatalf("save settings: %v", err)
	}
	return saved
}

func seedBusinessAPIProvider(t *testing.T, cfg *config.Config, input businessproviders.MutationInput) businessproviders.Provider {
	t.Helper()
	store, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	defer store.Close()
	provider, err := store.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	return provider
}

func seedBusinessProviderPoolMember(t *testing.T, cfg *config.Config, baseURL string, apiKey string) (businessproviders.Group, businessproviders.Member) {
	t.Helper()
	store, err := businessproviders.NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	defer store.Close()
	group, err := store.CreateGroup(context.Background(), businessproviders.GroupInput{
		Name:      "Pool Test Group",
		Platform:  businessproviders.PlatformGPTImage,
		Enabled:   true,
		IsDefault: true,
		Priority:  10,
	})
	if err != nil {
		t.Fatalf("seed provider group: %v", err)
	}
	member, err := store.CreateMember(context.Background(), businessproviders.MemberInput{
		GroupID:      group.ID,
		Name:         "Pool Test Member",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: "gpt-image-pool",
		Enabled:      true,
		Priority:     10,
	})
	if err != nil {
		t.Fatalf("seed provider member: %v", err)
	}
	return group, member
}

func providerSubmitErrorCodeIs(err error, code string) bool {
	submitErr, ok := err.(*providerImageGenerateSubmitError)
	return ok && submitErr.result.ErrorCode == code && submitErr.result.StatusCode == http.StatusTooManyRequests
}

func providerSubmitErrorMessageIs(err error, message string) bool {
	submitErr, ok := err.(*providerImageGenerateSubmitError)
	return ok && submitErr.result.ErrorMessage == message
}

func seedBusinessImageFile(t *testing.T, cfg *config.Config, fileName string) string {
	t.Helper()
	return seedBusinessImageFileInDir(t, cfg, cfg.Storage.ImageDir, fileName)
}

func seedBusinessImageFileInDir(t *testing.T, cfg *config.Config, dir, fileName string) string {
	t.Helper()
	path := filepath.Join(cfg.ResolvePath(dir), fileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir image dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("image"), 0o644); err != nil {
		t.Fatalf("write image file: %v", err)
	}
	return path
}

func reportHasMissingFile(report businessStorageReport, fileName string) bool {
	for _, item := range report.MissingFiles {
		if item.FileName == fileName {
			return true
		}
	}
	return false
}

func reportHasBrokenAsset(report businessStorageReport, fileName string) bool {
	for _, item := range report.BrokenAssets {
		if item.FileName == fileName {
			return true
		}
	}
	return false
}

func loginTestToken(t *testing.T, server *Server, username, password string) string {
	t.Helper()
	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/login",
		strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode login payload: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("login token is empty")
	}
	return payload.Token
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
