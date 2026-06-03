package businessapikeys

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/businessauth"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func newAPIKeyTestStore(t *testing.T) (*Store, *sql.DB, string) {
	t.Helper()
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
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewStoreWithDB(db, cfg.Database.Driver, "test-secret")

	suffix := time.Now().UTC().Format("20060102150405.000000")
	userID := "apikey_owner_" + suffix
	authStore := businessauth.NewStoreWithDB(db, cfg.Database.Driver)
	if err := authStore.EnsureBootstrapUser(ctx, businessauth.BootstrapUser{
		ID:       userID,
		Username: userID,
		Email:    userID + "@example.com",
		Password: "test-pass-123",
		Role:     businessauth.RoleAdmin,
	}); err != nil {
		t.Fatalf("EnsureBootstrapUser() error: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM business_api_keys WHERE user_id = $1", userID)
		_, _ = db.Exec("DELETE FROM business_users WHERE id = $1", userID)
	})
	return store, db, userID
}

func TestCreateAndAuthenticate(t *testing.T) {
	store, _, userID := newAPIKeyTestStore(t)
	ctx := context.Background()

	key, plaintext, err := store.Create(ctx, CreateInput{UserID: userID, Name: "production", Env: "live"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if !strings.HasPrefix(plaintext, "poom_live_") {
		t.Fatalf("plaintext = %q, want poom_live_ prefix", plaintext)
	}
	if key.Status != StatusActive {
		t.Fatalf("status = %q, want active", key.Status)
	}

	found, err := store.Authenticate(ctx, plaintext)
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}
	if found.ID != key.ID || found.UserID != userID {
		t.Fatalf("authenticated key = %#v, want id=%s user=%s", found, key.ID, userID)
	}
}

func TestAuthenticateWrongKeyRejected(t *testing.T) {
	store, _, _ := newAPIKeyTestStore(t)
	if _, err := store.Authenticate(context.Background(), "poom_live_doesnotexist"); err != sql.ErrNoRows {
		t.Fatalf("Authenticate(wrong) error = %v, want sql.ErrNoRows", err)
	}
}
