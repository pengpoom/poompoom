package api

import (
	"context"
	"database/sql"
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

func TestUserSelfServeAPIKeys(t *testing.T) {
	server, handler := apiAccessTestServer(t)
	ctx := context.Background()
	suffix := time.Now().UTC().Format("20060102150405.000000")
	userID := "apiaccess_self_" + suffix
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
	token := postgresAPICreateSession(t, ctx, server, loginAccount{
		Username: userID, Email: userID + "@example.com", Role: businessauth.RoleUser, UserID: userID,
	})

	// 未开通：建 key 应 403
	rec := postgresAPIServeJSON(t, handler, http.MethodPost, "/api/business/api-keys", token, map[string]any{"name": "k1"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create before enable = %d, want 403", rec.Code)
	}

	// 开通后：建 key 成功，返回明文
	if _, err := authStore.SetUserAPIAccessEnabled(ctx, userID, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	rec = postgresAPIServeJSON(t, handler, http.MethodPost, "/api/business/api-keys", token, map[string]any{"name": "k1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("create after enable = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Item   businessapikeys.APIKey `json:"item"`
		Secret string                 `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(created.Secret, "poom_live_") {
		t.Fatalf("secret = %q, want poom_live_ prefix", created.Secret)
	}
	t.Cleanup(func() {
		_, _ = serverDB(server).Exec("DELETE FROM business_api_keys WHERE user_id = $1", userID)
		_, _ = serverDB(server).Exec("DELETE FROM business_users WHERE id = $1", userID)
	})

	// 列表：含刚建的 key
	rec = postgresAPIServeJSON(t, handler, http.MethodGet, "/api/business/api-keys", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d", rec.Code)
	}
	var listed struct {
		Items []businessapikeys.APIKey `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listed)
	if len(listed.Items) != 1 {
		t.Fatalf("list len = %d, want 1", len(listed.Items))
	}

	// revoke 自己的 key 成功
	rec = postgresAPIServeJSON(t, handler, http.MethodPost, "/api/business/api-keys/"+created.Item.ID+"/revoke", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke own = %d, body = %s", rec.Code, rec.Body.String())
	}

	// revoke 别人的 key：404（归属校验）
	rec = postgresAPIServeJSON(t, handler, http.MethodPost, "/api/business/api-keys/apikey_not_mine/revoke", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("revoke other = %d, want 404", rec.Code)
	}
}

func serverDB(s *Server) *sql.DB { return s.db }

func TestAdminListUsersAPIAccess(t *testing.T) {
	server, handler := apiAccessTestServer(t)
	ctx := context.Background()
	suffix := time.Now().UTC().Format("20060102150405.000000")
	adminID := "apiaccess_lister_" + suffix
	onID := "apiaccess_on_" + suffix
	offID := "apiaccess_off_" + suffix
	authStore, err := server.newBusinessAuthStore()
	if err != nil {
		t.Fatalf("newBusinessAuthStore: %v", err)
	}
	defer authStore.Close()
	for _, u := range []businessauth.BootstrapUser{
		{ID: adminID, Username: adminID, Email: adminID + "@example.com", Password: "p", Role: businessauth.RoleAdmin},
		{ID: onID, Username: onID, Email: onID + "@example.com", Password: "p", Role: businessauth.RoleUser},
		{ID: offID, Username: offID, Email: offID + "@example.com", Password: "p", Role: businessauth.RoleUser},
	} {
		if err := authStore.EnsureBootstrapUser(ctx, u); err != nil {
			t.Fatalf("EnsureBootstrapUser(%s): %v", u.ID, err)
		}
	}
	t.Cleanup(func() {
		_, _ = serverDB(server).Exec("DELETE FROM business_users WHERE id IN ($1, $2, $3)", adminID, onID, offID)
	})
	if _, err := authStore.SetUserAPIAccessEnabled(ctx, onID, true); err != nil {
		t.Fatalf("SetUserAPIAccessEnabled: %v", err)
	}

	adminToken := postgresAPICreateSession(t, ctx, server, loginAccount{
		Username: adminID, Email: adminID + "@example.com", Role: businessauth.RoleAdmin, UserID: adminID,
	})
	rec := postgresAPIServeJSON(t, handler, http.MethodGet, "/api/business/admin/users-api-access", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		EnabledUserIDs []string `json:"enabledUserIds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	set := map[string]bool{}
	for _, id := range payload.EnabledUserIDs {
		set[id] = true
	}
	if !set[onID] {
		t.Fatalf("enabled list missing %s", onID)
	}
	if set[offID] {
		t.Fatalf("enabled list unexpectedly contains %s", offID)
	}

	// 无 admin session → 401
	rec = postgresAPIServeJSON(t, handler, http.MethodGet, "/api/business/admin/users-api-access", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d, want 401", rec.Code)
	}
}
