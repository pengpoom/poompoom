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

func TestRevokeThenAuthenticateRejected(t *testing.T) {
	store, _, userID := newAPIKeyTestStore(t)
	ctx := context.Background()
	key, plaintext, err := store.Create(ctx, CreateInput{UserID: userID, Name: "to-revoke", Env: "live"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	revoked, err := store.Revoke(ctx, key.ID)
	if err != nil {
		t.Fatalf("Revoke() error: %v", err)
	}
	if revoked.Status != StatusRevoked {
		t.Fatalf("status = %q, want revoked", revoked.Status)
	}
	if _, err := store.Authenticate(ctx, plaintext); err != sql.ErrNoRows {
		t.Fatalf("Authenticate(after revoke) error = %v, want sql.ErrNoRows", err)
	}
}

func TestUpdateDisabledRejectsAuthenticate(t *testing.T) {
	store, _, userID := newAPIKeyTestStore(t)
	ctx := context.Background()
	key, plaintext, _ := store.Create(ctx, CreateInput{UserID: userID, Name: "to-disable", Env: "live"})
	disabled := StatusDisabled
	if _, err := store.Update(ctx, key.ID, UpdateInput{Status: &disabled}); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if _, err := store.Authenticate(ctx, plaintext); err != sql.ErrNoRows {
		t.Fatalf("Authenticate(disabled) error = %v, want sql.ErrNoRows", err)
	}
}

func TestDeleteRemovesKey(t *testing.T) {
	store, _, userID := newAPIKeyTestStore(t)
	ctx := context.Background()
	key, _, err := store.Create(ctx, CreateInput{UserID: userID, Name: "to-delete", Env: "live"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := store.Delete(ctx, key.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if _, err := store.GetByID(ctx, key.ID); err != sql.ErrNoRows {
		t.Fatalf("GetByID(after delete) error = %v, want sql.ErrNoRows", err)
	}
	list, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListByUser len = %d, want 0", len(list))
	}
}

func TestListByUserAndUpdateLimits(t *testing.T) {
	store, _, userID := newAPIKeyTestStore(t)
	ctx := context.Background()
	if _, _, err := store.Create(ctx, CreateInput{UserID: userID, Name: "k1", Env: "live"}); err != nil {
		t.Fatalf("Create k1 error: %v", err)
	}
	second, _, err := store.Create(ctx, CreateInput{UserID: userID, Name: "k2", Env: "test"})
	if err != nil {
		t.Fatalf("Create k2 error: %v", err)
	}
	keys, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("ListByUser returned %d keys, want 2", len(keys))
	}
	newLimit := int64(5000)
	updated, err := store.Update(ctx, second.ID, UpdateInput{CreditLimit: &newLimit})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.CreditLimit != 5000 {
		t.Fatalf("creditLimit = %d, want 5000", updated.CreditLimit)
	}
	reloaded, err := store.GetByID(ctx, second.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if reloaded.CreditLimit != 5000 {
		t.Fatalf("reloaded creditLimit = %d, want 5000", reloaded.CreditLimit)
	}
}

func TestListAllAPIKeys(t *testing.T) {
	store, _, userID := newAPIKeyTestStore(t)
	ctx := context.Background()
	a, _, err := store.Create(ctx, CreateInput{UserID: userID, Name: "all-1", Env: "live"})
	if err != nil {
		t.Fatalf("Create all-1 error: %v", err)
	}
	b, _, err := store.Create(ctx, CreateInput{UserID: userID, Name: "all-2", Env: "test"})
	if err != nil {
		t.Fatalf("Create all-2 error: %v", err)
	}
	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	found := map[string]bool{}
	for _, k := range keys {
		found[k.ID] = true
	}
	if !found[a.ID] || !found[b.ID] {
		t.Fatalf("List() missing created keys: a=%v b=%v (total %d)", found[a.ID], found[b.ID], len(keys))
	}
}
