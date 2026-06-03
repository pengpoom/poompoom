package businessauth

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func newTestConfig(t *testing.T) *config.Config {
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
	resetTestStore(t, cfg)
	return cfg
}

func resetTestStore(t *testing.T, cfg *config.Config) {
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
	if _, err := db.ExecContext(ctx, `TRUNCATE email_verification_codes, user_sessions, business_users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset auth test tables: %v", err)
	}
}

func TestEnsureBootstrapUserStoresPasswordHash(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	err = store.EnsureBootstrapUser(context.Background(), BootstrapUser{
		ID:       "dev_admin",
		Username: "Admin",
		Password: "secret-pass",
		Role:     RoleAdmin,
	})
	if err != nil {
		t.Fatalf("EnsureBootstrapUser() returned error: %v", err)
	}

	user, ok, err := store.GetUserByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername() returned error: %v", err)
	}
	if !ok {
		t.Fatal("bootstrap user was not created")
	}
	if user.PasswordHash == "secret-pass" || !strings.HasPrefix(user.PasswordHash, "$2") {
		t.Fatalf("password hash = %q, want bcrypt hash", user.PasswordHash)
	}
	if user.Email != "admin@local.invalid" {
		t.Fatalf("email = %q, want admin@local.invalid", user.Email)
	}
	if user.Role != RoleAdmin || user.Status != StatusActive {
		t.Fatalf("user role/status = %q/%q", user.Role, user.Status)
	}
}

func TestEnsureBootstrapUserRefreshesExistingID(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.EnsureBootstrapUser(ctx, BootstrapUser{
		ID:       "dev_admin",
		Username: "admin",
		Email:    "admin@example.com",
		Password: "first-pass",
		Role:     RoleAdmin,
	}); err != nil {
		t.Fatalf("EnsureBootstrapUser(first) returned error: %v", err)
	}

	if err := store.EnsureBootstrapUser(ctx, BootstrapUser{
		ID:       "dev_admin",
		Username: "owner",
		Email:    "owner@example.com",
		Password: "second-pass",
		Role:     RoleAdmin,
	}); err != nil {
		t.Fatalf("EnsureBootstrapUser(second) returned error: %v", err)
	}

	user, ok, err := store.GetUserByID(ctx, "dev_admin")
	if err != nil || !ok {
		t.Fatalf("GetUserByID() ok=%v err=%v", ok, err)
	}
	if user.Username != "owner" || user.Email != "owner@example.com" {
		t.Fatalf("user = %#v, want refreshed username/email", user)
	}

	var count int
	if err := store.db.QueryRowContext(ctx, store.rebind(`SELECT COUNT(*) FROM business_users WHERE id = ?`), "dev_admin").Scan(&count); err != nil {
		t.Fatalf("count users by id: %v", err)
	}
	if count != 1 {
		t.Fatalf("bootstrap user count = %d, want 1", count)
	}
}

func TestAuthenticateUsesDatabasePasswordHash(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	err = store.EnsureBootstrapUser(context.Background(), BootstrapUser{
		ID:       "dev_user",
		Username: "tester",
		Password: "tester-pass",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("EnsureBootstrapUser() returned error: %v", err)
	}

	user, ok, err := store.Authenticate(context.Background(), "tester", "tester-pass")
	if err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}
	if !ok {
		t.Fatal("Authenticate() ok = false, want true")
	}
	if user.ID != "dev_user" || user.Role != RoleUser {
		t.Fatalf("authenticated user = %#v", user)
	}

	if _, ok, err := store.Authenticate(context.Background(), "tester", "wrong-pass"); err != nil || ok {
		t.Fatalf("Authenticate(wrong) ok=%v err=%v, want false nil", ok, err)
	}
	if _, ok, err := store.Authenticate(context.Background(), "tester@local.invalid", "tester-pass"); err != nil || !ok {
		t.Fatalf("Authenticate(email) ok=%v err=%v, want true nil", ok, err)
	}
}

func TestSessionLifecyclePersistsTokenHash(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.EnsureBootstrapUser(ctx, BootstrapUser{
		ID:       "dev_user",
		Username: "tester",
		Password: "tester-pass",
		Role:     RoleUser,
	}); err != nil {
		t.Fatalf("EnsureBootstrapUser() returned error: %v", err)
	}
	user, ok, err := store.GetUserByUsername(ctx, "tester")
	if err != nil || !ok {
		t.Fatalf("GetUserByUsername() ok=%v err=%v", ok, err)
	}

	token := "sess_plain_token"
	if _, err := store.CreateSession(ctx, "session-1", token, user, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	var storedHash string
	if err := store.db.QueryRowContext(ctx, `SELECT token_hash FROM user_sessions WHERE id = ?`, "session-1").Scan(&storedHash); err != nil {
		t.Fatalf("query stored token hash: %v", err)
	}
	if storedHash == token || storedHash != TokenHash(token) {
		t.Fatalf("stored token hash = %q, want sha256 token hash", storedHash)
	}

	session, ok, err := store.GetSessionByToken(ctx, token)
	if err != nil {
		t.Fatalf("GetSessionByToken() returned error: %v", err)
	}
	if !ok || session.User.ID != user.ID || session.TokenHash != storedHash {
		t.Fatalf("session = %#v ok=%v", session, ok)
	}

	if err := store.RevokeSessionByToken(ctx, token); err != nil {
		t.Fatalf("RevokeSessionByToken() returned error: %v", err)
	}
	if _, ok, err := store.GetSessionByToken(ctx, token); err != nil || ok {
		t.Fatalf("GetSessionByToken(revoked) ok=%v err=%v, want false nil", ok, err)
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.EnsureBootstrapUser(ctx, BootstrapUser{
		ID:       "dev_user",
		Username: "tester",
		Password: "tester-pass",
		Role:     RoleUser,
	}); err != nil {
		t.Fatalf("EnsureBootstrapUser() returned error: %v", err)
	}
	user, ok, err := store.GetUserByUsername(ctx, "tester")
	if err != nil || !ok {
		t.Fatalf("GetUserByUsername() ok=%v err=%v", ok, err)
	}
	token := "sess_expired_token"
	if _, err := store.CreateSession(ctx, "session-expired", token, user, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	if _, ok, err := store.GetSessionByToken(ctx, token); err != nil || ok {
		t.Fatalf("GetSessionByToken(expired) ok=%v err=%v, want false nil", ok, err)
	}
}

func TestCreateUserListsAndRejectsDuplicates(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	user, err := store.CreateUser(ctx, "guest", "guest-pass", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser() returned error: %v", err)
	}
	if user.Username != "guest" || user.Email != "guest@local.invalid" || user.Role != RoleUser || user.Status != StatusActive {
		t.Fatalf("created user = %#v", user)
	}
	if user.UID != DefaultUIDStart {
		t.Fatalf("created user uid = %d, want %d", user.UID, DefaultUIDStart)
	}

	users, err := store.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() returned error: %v", err)
	}
	if len(users) != 1 || users[0].Username != "guest" || users[0].UID != DefaultUIDStart {
		t.Fatalf("users = %#v", users)
	}

	if _, err := store.CreateUser(ctx, "guest", "other-pass", RoleUser); err != ErrUserAlreadyExists {
		t.Fatalf("CreateUser(duplicate) err=%v, want ErrUserAlreadyExists", err)
	}

	emailUser, err := store.CreateUser(ctx, "guest@example.com", "guest-pass", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser(email) returned error: %v", err)
	}
	if emailUser.Username != "guest2" || emailUser.Email != "guest@example.com" {
		t.Fatalf("email user = %#v", emailUser)
	}
	if emailUser.UID != DefaultUIDStart+1 {
		t.Fatalf("email user uid = %d, want %d", emailUser.UID, DefaultUIDStart+1)
	}
}

func TestMigrateBusinessUsersUIDAssignsCreatedOrder(t *testing.T) {
	cfg := newTestConfig(t)
	ctx := context.Background()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open postgres database: %v", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `TRUNCATE email_verification_codes, user_sessions, business_users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset users table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO business_users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES
		('u2', 'second', 'second@example.com', 'hash', 'user', 'active', '2026-01-02T00:00:00Z', '2026-01-02T00:00:00Z'),
		('u1', 'first', 'first@example.com', 'hash', 'user', 'active', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert users without uid: %v", err)
	}

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()
	users, err := store.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers() returned error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("users = %#v, want 2 users", users)
	}
	if users[0].Username != "first" || users[0].UID != DefaultUIDStart {
		t.Fatalf("first migrated user = %#v", users[0])
	}
	if users[1].Username != "second" || users[1].UID != DefaultUIDStart+1 {
		t.Fatalf("second migrated user = %#v", users[1])
	}
}

func TestDisableAndResetUserInvalidatesSessions(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	user, err := store.CreateUser(ctx, "tester", "old-pass", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser() returned error: %v", err)
	}

	token := "sess_disable_me"
	if _, err := store.CreateSession(ctx, "session-1", token, user, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	updated, ok, err := store.SetUserStatus(ctx, user.ID, StatusDisabled)
	if err != nil {
		t.Fatalf("SetUserStatus() returned error: %v", err)
	}
	if !ok || updated.Status != StatusDisabled {
		t.Fatalf("updated user = %#v ok=%v", updated, ok)
	}
	if _, ok, err := store.GetSessionByToken(ctx, token); err != nil || ok {
		t.Fatalf("GetSessionByToken(disabled) ok=%v err=%v, want false nil", ok, err)
	}

	reenabled, ok, err := store.SetUserStatus(ctx, user.ID, StatusActive)
	if err != nil {
		t.Fatalf("SetUserStatus(active) returned error: %v", err)
	}
	if !ok || reenabled.Status != StatusActive {
		t.Fatalf("reenabled user = %#v ok=%v", reenabled, ok)
	}

	_, ok, err = store.ResetUserPassword(ctx, user.ID, "new-pass")
	if err != nil {
		t.Fatalf("ResetUserPassword() returned error: %v", err)
	}
	if !ok {
		t.Fatal("ResetUserPassword() ok=false, want true")
	}
	if _, ok, err := store.Authenticate(ctx, "tester", "old-pass"); err != nil || ok {
		t.Fatalf("Authenticate(old) ok=%v err=%v, want false nil", ok, err)
	}
	if _, ok, err := store.Authenticate(ctx, "tester", "new-pass"); err != nil || !ok {
		t.Fatalf("Authenticate(new) ok=%v err=%v, want true nil", ok, err)
	}
}

func TestDeleteUserRevokesSessionsAndProtectsLastAdmin(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	admin, err := store.CreateUser(ctx, "admin@example.com", "admin-pass", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser(admin) returned error: %v", err)
	}
	if _, ok, err := store.DeleteUser(ctx, admin.ID); err != ErrLastAdminUser || ok {
		t.Fatalf("DeleteUser(last admin) ok=%v err=%v, want ErrLastAdminUser", ok, err)
	}

	user, err := store.CreateUser(ctx, "tester@example.com", "tester-pass", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser(user) returned error: %v", err)
	}
	token := "sess_delete_me"
	if _, err := store.CreateSession(ctx, "session-delete", token, user, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	deleted, ok, err := store.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUser() returned error: %v", err)
	}
	if !ok || deleted.ID != user.ID || deleted.Status != StatusDeleted || deleted.DeletedAt == "" {
		t.Fatalf("deleted user = %#v ok=%v", deleted, ok)
	}
	if fetched, ok, err := store.GetUserByID(ctx, user.ID); err != nil || !ok || fetched.Status != StatusDeleted {
		t.Fatalf("GetUserByID(deleted) user=%#v ok=%v err=%v", fetched, ok, err)
	}
	users, err := store.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() returned error: %v", err)
	}
	for _, listed := range users {
		if listed.ID == user.ID {
			t.Fatalf("deleted user listed by default: %#v", listed)
		}
	}
	users, err = store.ListUsers(ctx, ListUsersOptions{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListUsers(include deleted) returned error: %v", err)
	}
	foundDeleted := false
	for _, listed := range users {
		if listed.ID == user.ID && listed.Status == StatusDeleted {
			foundDeleted = true
			break
		}
	}
	if !foundDeleted {
		t.Fatalf("deleted user missing from include-deleted list: %#v", users)
	}
	if _, ok, err := store.GetSessionByToken(ctx, token); err != nil || ok {
		t.Fatalf("GetSessionByToken(deleted) ok=%v err=%v, want false nil", ok, err)
	}
	if _, err := store.CreateUser(ctx, "tester@example.com", "tester-pass", RoleUser); err != ErrUserDeleted {
		t.Fatalf("CreateUser(deleted email) err=%v, want ErrUserDeleted", err)
	}
	restored, ok, err := store.RestoreUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("RestoreUser() returned error: %v", err)
	}
	if !ok || restored.Status != StatusActive || restored.DeletedAt != "" {
		t.Fatalf("restored user = %#v ok=%v", restored, ok)
	}
}

func TestUserAPIAccessToggle(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	cfg := config.New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}
	store := NewStoreWithDB(db, cfg.Database.Driver)

	userID := "apiaccess_user_" + time.Now().UTC().Format("20060102150405.000000")
	if err := store.EnsureBootstrapUser(ctx, BootstrapUser{
		ID: userID, Username: userID, Email: userID + "@example.com", Password: "p", Role: RoleAdmin,
	}); err != nil {
		t.Fatalf("EnsureBootstrapUser() error: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM business_users WHERE id = $1", userID) })

	// 默认应为 false
	enabled, err := store.IsUserAPIAccessEnabled(ctx, userID)
	if err != nil {
		t.Fatalf("IsUserAPIAccessEnabled() error: %v", err)
	}
	if enabled {
		t.Fatalf("default api_access_enabled = true, want false")
	}

	// 打开
	ok, err := store.SetUserAPIAccessEnabled(ctx, userID, true)
	if err != nil || !ok {
		t.Fatalf("SetUserAPIAccessEnabled(true) ok=%v err=%v", ok, err)
	}
	enabled, _ = store.IsUserAPIAccessEnabled(ctx, userID)
	if !enabled {
		t.Fatalf("after enable, api_access_enabled = false, want true")
	}

	// 不存在的用户
	ok, err = store.SetUserAPIAccessEnabled(ctx, "no_such_user", true)
	if err != nil {
		t.Fatalf("SetUserAPIAccessEnabled(missing) error: %v", err)
	}
	if ok {
		t.Fatalf("SetUserAPIAccessEnabled(missing) ok=true, want false")
	}
}
