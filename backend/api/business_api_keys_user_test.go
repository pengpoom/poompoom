package api

import (
	"context"
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
