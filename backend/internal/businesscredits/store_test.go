package businesscredits

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func TestRefundIsIdempotentByGeneration(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	ctx := context.Background()
	cfg := creditDatabaseTestConfig(dsn)
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405")
	userID := "credit_refund_" + suffix
	generationID := "credit_generation_" + suffix
	cleanupCreditTestData(t, db, userID)
	defer cleanupCreditTestData(t, db, userID)

	store := NewStoreWithDB(db, cfg.Database.Driver)
	if summary, _, err := store.SetBalance(ctx, userID, 10, ReasonAdminAdjustment); err != nil {
		t.Fatalf("set balance: %v", err)
	} else if summary.Balance != 10 {
		t.Fatalf("initial summary = %#v, want balance 10", summary)
	}
	if summary, _, err := store.Reserve(ctx, userID, 3, generationID); err != nil {
		t.Fatalf("reserve credits: %v", err)
	} else if summary.Balance != 7 {
		t.Fatalf("reserved summary = %#v, want balance 7", summary)
	}
	if summary, entry, err := store.Refund(ctx, userID, 5, generationID); err != nil {
		t.Fatalf("refund credits: %v", err)
	} else if summary.Balance != 10 || entry.Delta != 3 {
		t.Fatalf("refund summary=%#v entry=%#v, want balance 10 delta 3", summary, entry)
	}
	if summary, entry, err := store.Refund(ctx, userID, 5, generationID); err != nil {
		t.Fatalf("second refund credits: %v", err)
	} else if summary.Balance != 10 || entry.ID != "" {
		t.Fatalf("second refund summary=%#v entry=%#v, want no-op at balance 10", summary, entry)
	}
	totals, err := store.GenerationTotals(ctx, userID, generationID)
	if err != nil {
		t.Fatalf("generation totals: %v", err)
	}
	if totals.Reserved != 3 || totals.Refunded != 3 {
		t.Fatalf("generation totals = %#v, want reserved 3 refunded 3", totals)
	}
}

func cleanupCreditTestData(t *testing.T, db *sql.DB, userID string) {
	t.Helper()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM business_credit_ledger WHERE user_id = $1`, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_user_credits WHERE user_id = $1`, userID)
}

func creditDatabaseTestConfig(dsn string) *config.Config {
	cfg := config.New("")
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 4
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60
	return cfg
}
