package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/businessauth"
	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
	"imagestudio/internal/businessjobs"
	"imagestudio/internal/businessproviders"
	"imagestudio/internal/businesstracker"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func TestPostgresBusinessImageAPIFlow(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	var upstreamAuth string
	var upstreamPayload map[string]any
	imageB64 := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("upstream path = %q, want /v1/images/generations", r.URL.Path)
		}
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamPayload); err != nil {
			t.Fatalf("decode upstream payload: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"b64_json": imageB64, "revised_prompt": "postgres smoke"},
			},
		})
	}))
	defer upstream.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cfg := config.New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 5
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60
	cfg.Storage.ImageDir = "data/business-images"

	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open postgres database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("migrate postgres database: %v", err)
	}

	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	ids := postgresAPITestIDs{
		AdminID:        "pg_api_" + suffix + "_admin",
		AdminUsername:  "pg_api_admin_" + suffix,
		AdminEmail:     "pg_api_admin_" + suffix + "@example.test",
		UserID:         "pg_api_" + suffix + "_user",
		UserUsername:   "pg_api_user_" + suffix,
		UserEmail:      "pg_api_user_" + suffix + "@example.test",
		ConversationID: "pg_api_" + suffix + "_conversation",
		TurnID:         "pg_api_" + suffix + "_turn",
		JobID:          "pg_api_" + suffix + "_job",
	}
	t.Cleanup(func() {
		cleanupPostgresAPITestData(t, db, ids)
	})

	authStore := businessauth.NewStoreWithDB(db, cfg.Database.Driver)
	const password = "postgres-api-pass"
	if err := authStore.EnsureBootstrapUsers(ctx, []businessauth.BootstrapUser{
		{
			ID:       ids.AdminID,
			Username: ids.AdminUsername,
			Email:    ids.AdminEmail,
			Password: password,
			Role:     businessauth.RoleAdmin,
		},
		{
			ID:       ids.UserID,
			Username: ids.UserUsername,
			Email:    ids.UserEmail,
			Password: password,
			Role:     businessauth.RoleUser,
		},
	}); err != nil {
		t.Fatalf("seed business users: %v", err)
	}

	providerDefaults, err := snapshotProviderDefaultFlags(ctx, db, businessproviders.PlatformGPTImage)
	if err != nil {
		t.Fatalf("snapshot provider defaults: %v", err)
	}
	providerID := ""
	t.Cleanup(func() {
		restorePostgresAPIProviderState(t, db, businessproviders.PlatformGPTImage, providerID, providerDefaults)
	})
	providerStore := businessproviders.NewStoreWithDB(db, cfg.Database.Driver)
	provider, err := providerStore.Create(ctx, businessproviders.MutationInput{
		Name:         "PostgreSQL API Test",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      upstream.URL,
		APIKey:       "provider-key",
		DefaultModel: "gpt-image-test",
		Enabled:      true,
		IsDefault:    true,
	})
	if err != nil {
		t.Fatalf("create postgres provider: %v", err)
	}
	providerID = provider.ID

	server := NewServerWithDatabase(cfg, nil, nil, db)
	handler := server.Handler()
	adminToken := postgresAPICreateSession(t, ctx, server, loginAccount{
		Username: ids.AdminUsername,
		Email:    ids.AdminEmail,
		Role:     businessauth.RoleAdmin,
		UserID:   ids.AdminID,
	})
	userToken := postgresAPICreateSession(t, ctx, server, loginAccount{
		Username: ids.UserUsername,
		Email:    ids.UserEmail,
		Role:     businessauth.RoleUser,
		UserID:   ids.UserID,
	})

	postgresAPISetCredit(t, handler, adminToken, ids.UserID, 100)
	creditBefore := postgresAPIGetCredit(t, handler, userToken)
	if creditBefore.Balance != 100 {
		t.Fatalf("credit before generation = %#v, want balance 100", creditBefore)
	}

	submitBody := map[string]any{
		"prompt":         "postgres api smoke",
		"n":              1,
		"platform":       businessproviders.PlatformGPTImage,
		"conversationId": ids.ConversationID,
		"turnId":         ids.TurnID,
		"jobId":          ids.JobID,
		"title":          "PostgreSQL API smoke",
	}
	rec := postgresAPIServeJSON(t, handler, http.MethodPost, "/api/image/generate", userToken, submitBody)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var submitPayload providerImageGenerateJobPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &submitPayload); err != nil {
		t.Fatalf("decode submit payload: %v", err)
	}
	if submitPayload.JobID != ids.JobID || submitPayload.Status != businessjobs.StatusQueued {
		t.Fatalf("submit payload = %#v", submitPayload)
	}

	job := postgresAPIWaitForJob(t, handler, userToken, ids.JobID)
	if job.Status != businessjobs.StatusSucceeded {
		t.Fatalf("job status = %q, error = %s/%s", job.Status, job.ErrorCode, job.ErrorMessage)
	}
	if job.ProviderID != provider.ID || job.Model != "gpt-image-test" {
		t.Fatalf("job provider/model = %q/%q, want %q/gpt-image-test", job.ProviderID, job.Model, provider.ID)
	}
	if upstreamAuth != "Bearer provider-key" {
		t.Fatalf("upstream Authorization = %q, want provider key", upstreamAuth)
	}
	if upstreamPayload["prompt"] != "postgres api smoke" || upstreamPayload["model"] != "gpt-image-test" {
		t.Fatalf("upstream payload = %#v", upstreamPayload)
	}
	for _, key := range []string{"conversationId", "turnId", "jobId", "title"} {
		if _, ok := upstreamPayload[key]; ok {
			t.Fatalf("upstream payload unexpectedly contains %q", key)
		}
	}

	conversation := postgresAPIGetConversation(t, handler, userToken, ids.ConversationID)
	if conversation.Conversation.ID != ids.ConversationID || len(conversation.Generations) != 1 {
		t.Fatalf("conversation payload = %#v", conversation)
	}
	generation := conversation.Generations[0]
	if generation.ID != ids.JobID || generation.Status != businessjobs.StatusSucceeded {
		t.Fatalf("generation = %#v", generation)
	}
	responseText := string(generation.Response)
	if strings.Contains(responseText, "b64_json") || !strings.Contains(responseText, "/v1/files/image/business-") {
		t.Fatalf("generation response = %s", responseText)
	}

	creditAfter := postgresAPIGetCredit(t, handler, userToken)
	expectedSpent := job.CreditReserved - job.CreditRefunded
	if expectedSpent < 0 {
		expectedSpent = 0
	}
	if creditAfter.Balance != 100-expectedSpent || creditAfter.Spent != expectedSpent {
		t.Fatalf("credit after generation = %#v, want balance %d spent %d", creditAfter, 100-expectedSpent, expectedSpent)
	}

	var trackerStatus string
	if err := db.QueryRowContext(
		ctx,
		`SELECT status FROM business_image_tracker WHERE user_id = $1 AND generation_id = $2 ORDER BY created_at DESC LIMIT 1`,
		ids.UserID,
		ids.JobID,
	).Scan(&trackerStatus); err != nil {
		t.Fatalf("read tracker status: %v", err)
	}
	if trackerStatus != businesstracker.StatusSucceeded {
		t.Fatalf("tracker status = %q, want %q", trackerStatus, businesstracker.StatusSucceeded)
	}

	for _, target := range []string{
		"/api/business/admin/dashboard",
		"/api/business/system-settings",
		"/api/business/api-providers",
		"/api/business/users",
		"/api/business/admin/jobs",
		"/api/business/admin/usage",
		"/api/business/storage/report",
		"/api/business/tracker/summary?windowSeconds=3600",
	} {
		postgresAPIAssertOK(t, handler, adminToken, target)
	}
}

type postgresAPITestIDs struct {
	AdminID        string
	AdminUsername  string
	AdminEmail     string
	UserID         string
	UserUsername   string
	UserEmail      string
	ConversationID string
	TurnID         string
	JobID          string
}

type providerDefaultFlagSnapshot struct {
	ID        string
	IsDefault int
}

func snapshotProviderDefaultFlags(ctx context.Context, db *sql.DB, platform string) ([]providerDefaultFlagSnapshot, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT id, is_default FROM business_api_providers WHERE platform = $1`,
		platform,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []providerDefaultFlagSnapshot{}
	for rows.Next() {
		var item providerDefaultFlagSnapshot
		if err := rows.Scan(&item.ID, &item.IsDefault); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func restorePostgresAPIProviderState(t *testing.T, db *sql.DB, platform string, providerID string, defaults []providerDefaultFlagSnapshot) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Logf("restore provider state begin failed: %v", err)
		return
	}
	defer tx.Rollback()
	if strings.TrimSpace(providerID) != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM business_api_providers WHERE id = $1`, providerID); err != nil {
			t.Logf("delete test provider failed: %v", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE business_api_providers SET is_default = 0 WHERE platform = $1`, platform); err != nil {
		t.Logf("clear provider defaults failed: %v", err)
		return
	}
	for _, item := range defaults {
		if _, err := tx.ExecContext(ctx, `UPDATE business_api_providers SET is_default = $1 WHERE id = $2`, item.IsDefault, item.ID); err != nil {
			t.Logf("restore provider default %s failed: %v", item.ID, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		t.Logf("restore provider state commit failed: %v", err)
	}
}

func cleanupPostgresAPITestData(t *testing.T, db *sql.DB, ids postgresAPITestIDs) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM business_image_tracker WHERE user_id = $1 OR conversation_id = $2 OR generation_id = $3`, []any{ids.UserID, ids.ConversationID, ids.JobID}},
		{`DELETE FROM business_image_assets WHERE user_id = $1 OR conversation_id = $2 OR generation_id = $3`, []any{ids.UserID, ids.ConversationID, ids.JobID}},
		{`DELETE FROM business_image_generations WHERE user_id = $1 OR conversation_id = $2 OR id = $3`, []any{ids.UserID, ids.ConversationID, ids.JobID}},
		{`DELETE FROM business_image_conversations WHERE user_id = $1 OR id = $2`, []any{ids.UserID, ids.ConversationID}},
		{`DELETE FROM business_image_jobs WHERE user_id = $1 OR conversation_id = $2 OR generation_id = $3 OR id = $3`, []any{ids.UserID, ids.ConversationID, ids.JobID}},
		{`DELETE FROM business_credit_ledger WHERE user_id IN ($1, $2)`, []any{ids.AdminID, ids.UserID}},
		{`DELETE FROM business_user_credits WHERE user_id IN ($1, $2)`, []any{ids.AdminID, ids.UserID}},
		{`DELETE FROM user_sessions WHERE user_id IN ($1, $2)`, []any{ids.AdminID, ids.UserID}},
		{`DELETE FROM email_verification_codes WHERE email IN ($1, $2)`, []any{ids.AdminEmail, ids.UserEmail}},
		{`DELETE FROM business_users WHERE id IN ($1, $2) OR username IN ($3, $4) OR email IN ($5, $6)`, []any{ids.AdminID, ids.UserID, ids.AdminUsername, ids.UserUsername, ids.AdminEmail, ids.UserEmail}},
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Logf("cleanup query failed: %v", err)
		}
	}
}

func postgresAPICreateSession(t *testing.T, ctx context.Context, server *Server, account loginAccount) string {
	t.Helper()
	token, _, err := server.createAuthSession(ctx, account)
	if err != nil {
		t.Fatalf("create session for %s: %v", account.UserID, err)
	}
	if token == "" {
		t.Fatal("session token is empty")
	}
	return token
}

func postgresAPISetCredit(t *testing.T, handler http.Handler, token string, userID string, balance int64) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"balance": balance,
		"reason":  "postgres_api_test",
	})
	if err != nil {
		t.Fatalf("marshal credit request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/business/users/"+userID+"/credit", bytes.NewReader(raw))
	req.SetPathValue("id", userID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set credit status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func postgresAPIGetCredit(t *testing.T, handler http.Handler, token string) businesscredits.Summary {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/business/credit", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get credit status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var summary businesscredits.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode credit: %v", err)
	}
	return summary
}

func postgresAPIWaitForJob(t *testing.T, handler http.Handler, token string, jobID string) businessjobs.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last businessjobs.Job
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, "/api/business/jobs/"+jobID, nil)
		req.SetPathValue("id", jobID)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("get job status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var payload struct {
			Item businessImageJobView `json:"item"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode job: %v", err)
		}
		last = payload.Item.Job
		switch last.Status {
		case businessjobs.StatusSucceeded, businessjobs.StatusFailed, businessjobs.StatusCancelled:
			return last
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish, last = %#v", jobID, last)
	return businessjobs.Job{}
}

func postgresAPIGetConversation(t *testing.T, handler http.Handler, token string, conversationID string) businessimage.ConversationWithGenerations {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/business/image/conversations/"+conversationID, nil)
	req.SetPathValue("id", conversationID)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get conversation status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Item businessimage.ConversationWithGenerations `json:"item"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode conversation: %v", err)
	}
	return payload.Item
}

func postgresAPIAssertOK(t *testing.T, handler http.Handler, token string, target string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, body = %s", target, rec.Code, rec.Body.String())
	}
}

func postgresAPIServeJSON(t *testing.T, handler http.Handler, method string, target string, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
