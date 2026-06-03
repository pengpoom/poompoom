package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/businessauth"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func apiAccessTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
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
	server := NewServerWithDatabase(cfg, nil, nil, db)
	return server, server.Handler()
}

func TestAdminToggleUserAPIAccess(t *testing.T) {
	server, handler := apiAccessTestServer(t)
	ctx := context.Background()

	suffix := time.Now().UTC().Format("20060102150405.000000")
	adminID := "apiaccess_admin_" + suffix
	userID := "apiaccess_target_" + suffix
	authStore, err := server.newBusinessAuthStore()
	if err != nil {
		t.Fatalf("newBusinessAuthStore: %v", err)
	}
	defer authStore.Close()
	for _, u := range []businessauth.BootstrapUser{
		{ID: adminID, Username: adminID, Email: adminID + "@example.com", Password: "p", Role: businessauth.RoleAdmin},
		{ID: userID, Username: userID, Email: userID + "@example.com", Password: "p", Role: businessauth.RoleAdmin},
	} {
		if err := authStore.EnsureBootstrapUser(ctx, u); err != nil {
			t.Fatalf("EnsureBootstrapUser(%s): %v", u.ID, err)
		}
	}
	adminToken := postgresAPICreateSession(t, ctx, server, loginAccount{
		Username: adminID, Email: adminID + "@example.com", Role: businessauth.RoleAdmin, UserID: adminID,
	})
	rec := postgresAPIServeJSON(t, handler, http.MethodPatch,
		"/api/business/users/"+userID+"/api-access", adminToken, map[string]any{"enabled": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d, body = %s", rec.Code, rec.Body.String())
	}
	enabled, err := authStore.IsUserAPIAccessEnabled(ctx, userID)
	if err != nil || !enabled {
		t.Fatalf("after toggle, enabled=%v err=%v, want true", enabled, err)
	}
}

func TestMeReportsAPIAccess(t *testing.T) {
	server, handler := apiAccessTestServer(t)
	ctx := context.Background()
	suffix := time.Now().UTC().Format("20060102150405.000000")
	userID := "apiaccess_me_" + suffix
	authStore, err := server.newBusinessAuthStore()
	if err != nil {
		t.Fatalf("newBusinessAuthStore: %v", err)
	}
	defer authStore.Close()
	if err := authStore.EnsureBootstrapUser(ctx, businessauth.BootstrapUser{
		ID: userID, Username: userID, Email: userID + "@example.com", Password: "p", Role: businessauth.RoleUser,
	}); err != nil {
		t.Fatalf("EnsureBootstrapUser: %v", err)
	}
	if _, err := authStore.SetUserAPIAccessEnabled(ctx, userID, true); err != nil {
		t.Fatalf("SetUserAPIAccessEnabled: %v", err)
	}
	token := postgresAPICreateSession(t, ctx, server, loginAccount{
		Username: userID, Email: userID + "@example.com", Role: businessauth.RoleUser, UserID: userID,
	})
	rec := postgresAPIServeJSON(t, handler, http.MethodGet, "/api/business/me", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		ApiAccessEnabled bool `json:"apiAccessEnabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if !payload.ApiAccessEnabled {
		t.Fatalf("me.apiAccessEnabled = false, want true")
	}
}
