package businessauth

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func TestPostgresStoreIntegration(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	cfg := config.New(t.TempDir())
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 4
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405")
	userID := "pg_smoke_" + suffix
	email := fmt.Sprintf("%s@example.com", userID)
	username := userID
	t.Cleanup(func() {
		cleanupPostgresAuthTestRows(context.Background(), db, userID, email)
	})

	store := NewStoreWithDB(db, cfg.Database.Driver)
	user := BootstrapUser{
		ID:       userID,
		Username: username,
		Email:    email,
		Password: "test-pass-123",
		Role:     RoleAdmin,
	}
	if err := store.EnsureBootstrapUser(ctx, user); err != nil {
		t.Fatalf("EnsureBootstrapUser() returned error: %v", err)
	}
	authenticated, ok, err := store.Authenticate(ctx, email, user.Password)
	if err != nil || !ok {
		t.Fatalf("Authenticate() ok=%v err=%v", ok, err)
	}
	if authenticated.ID != userID || authenticated.Role != RoleAdmin {
		t.Fatalf("authenticated user = %#v", authenticated)
	}

	session, err := store.CreateSession(ctx, "session_"+userID, "token-"+userID, authenticated, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	loaded, ok, err := store.GetSessionByToken(ctx, "token-"+userID)
	if err != nil || !ok {
		t.Fatalf("GetSessionByToken() ok=%v err=%v", ok, err)
	}
	if loaded.ID != session.ID || loaded.User.ID != userID {
		t.Fatalf("loaded session = %#v", loaded)
	}
	if err := store.RevokeSessionByToken(ctx, "token-"+userID); err != nil {
		t.Fatalf("RevokeSessionByToken() returned error: %v", err)
	}
	if _, ok, err := store.GetSessionByToken(ctx, "token-"+userID); err != nil || ok {
		t.Fatalf("revoked session ok=%v err=%v", ok, err)
	}
}

func cleanupPostgresAuthTestRows(ctx context.Context, db *sql.DB, userID, email string) {
	_, _ = db.ExecContext(ctx, `DELETE FROM user_sessions WHERE user_id = $1`, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM email_verification_codes WHERE email = $1`, email)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_users WHERE id = $1`, userID)
}
