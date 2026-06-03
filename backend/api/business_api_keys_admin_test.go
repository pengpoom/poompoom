package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/businessapikeys"
	"imagestudio/internal/businessauth"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func TestAdminAPIKeyLifecycle(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cfg := config.New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.ExternalAPI.SigningSecret = "test-secret"

	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405.000000")
	adminID := "apikey_admin_" + suffix
	ownerID := "apikey_owner_" + suffix
	authStore := businessauth.NewStoreWithDB(db, cfg.Database.Driver)
	for _, u := range []businessauth.BootstrapUser{
		{ID: adminID, Username: adminID, Email: adminID + "@example.com", Password: "p", Role: businessauth.RoleAdmin},
		{ID: ownerID, Username: ownerID, Email: ownerID + "@example.com", Password: "p", Role: businessauth.RoleAdmin},
	} {
		if err := authStore.EnsureBootstrapUser(ctx, u); err != nil {
			t.Fatalf("EnsureBootstrapUser(%s) error: %v", u.ID, err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM business_api_keys WHERE user_id = $1", ownerID)
		_, _ = db.Exec("DELETE FROM business_users WHERE id IN ($1, $2)", adminID, ownerID)
	})

	server := NewServerWithDatabase(cfg, nil, nil, db)
	handler := server.Handler()
	adminToken := postgresAPICreateSession(t, ctx, server, loginAccount{
		Username: adminID, Email: adminID + "@example.com",
		Role: businessauth.RoleAdmin, UserID: adminID,
	})

	// create
	rec := postgresAPIServeJSON(t, handler, http.MethodPost, "/api/business/admin/api-keys", adminToken, map[string]any{
		"userId": ownerID, "name": "production", "env": "live",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Item   businessapikeys.APIKey `json:"item"`
		Secret string                 `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if !strings.HasPrefix(created.Secret, "poom_live_") {
		t.Fatalf("secret = %q, want poom_live_ prefix", created.Secret)
	}
	keyID := created.Item.ID

	// list
	rec = postgresAPIServeJSON(t, handler, http.MethodGet, "/api/business/admin/api-keys?userId="+ownerID, adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items []businessapikeys.APIKey `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("list returned %d, want 1", len(listed.Items))
	}

	// patch -> disabled
	rec = postgresAPIServeJSON(t, handler, http.MethodPatch, "/api/business/admin/api-keys/"+keyID, adminToken, map[string]any{
		"status": "disabled",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// revoke
	rec = postgresAPIServeJSON(t, handler, http.MethodPost, "/api/business/admin/api-keys/"+keyID+"/revoke", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// no token -> unauthorized
	rec = postgresAPIServeJSON(t, handler, http.MethodGet, "/api/business/admin/api-keys?userId="+ownerID, "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d, want 401", rec.Code)
	}
}
