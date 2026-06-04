package businessnotifications

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func TestPostgresNotificationVisibility(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	cfg := config.New(t.TempDir())
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 4
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	lowUserID := "notice_low_" + suffix
	highUserID := "notice_high_" + suffix
	threshold := time.Now().UnixNano()
	t.Cleanup(func() {
		cleanupNotificationTestRows(context.Background(), db, suffix, lowUserID, highUserID)
	})
	seedNotificationUser(t, ctx, db, lowUserID, "low-"+suffix, 5)
	seedNotificationUser(t, ctx, db, highUserID, "high-"+suffix, threshold+10)

	store := NewStoreWithDB(db, cfg.Database.Driver)
	now := time.Now().UTC()
	visible, err := store.Create(ctx, MutationInput{
		Title:      "visible " + suffix,
		Body:       "visible body",
		Status:     StatusPublished,
		NotifyMode: NotifyModePopup,
		StartsAt:   now.Add(-time.Minute).Format(time.RFC3339Nano),
		EndsAt:     now.Add(time.Hour).Format(time.RFC3339Nano),
		Targeting: Targeting{
			Mode: TargetModeBalance,
			Balance: &BalanceTarget{
				Operator: ">=",
				Value:    threshold,
			},
		},
	})
	if err != nil {
		t.Fatalf("Create visible notification returned error: %v", err)
	}
	if visible.Targeting.Mode != TargetModeBalance || visible.NotifyMode != NotifyModePopup {
		t.Fatalf("created notification = %#v", visible)
	}
	if _, err := store.Create(ctx, MutationInput{
		Title:    "future " + suffix,
		Body:     "future body",
		Status:   StatusPublished,
		StartsAt: now.Add(time.Hour).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("Create future notification returned error: %v", err)
	}
	if _, err := store.Create(ctx, MutationInput{
		Title:  "expired " + suffix,
		Body:   "expired body",
		Status: StatusPublished,
		EndsAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("Create expired notification returned error: %v", err)
	}

	lowItems, err := store.ListForUser(ctx, lowUserID, 20)
	if err != nil {
		t.Fatalf("ListForUser(low) returned error: %v", err)
	}
	if hasNotification(lowItems, visible.ID) {
		t.Fatalf("low user unexpectedly received targeted notification: %#v", lowItems)
	}
	highItems, err := store.ListForUser(ctx, highUserID, 20)
	if err != nil {
		t.Fatalf("ListForUser(high) returned error: %v", err)
	}
	targeted, ok := findNotification(highItems, visible.ID)
	if !ok || targeted.NotifyMode != NotifyModePopup {
		t.Fatalf("high user items = %#v, want targeted popup", highItems)
	}
	unread, err := store.UnreadCount(ctx, highUserID)
	if err != nil {
		t.Fatalf("UnreadCount(high) returned error: %v", err)
	}
	if unread != 1 {
		t.Fatalf("UnreadCount(high) = %d, want 1", unread)
	}

	adminItems, err := store.ListAdmin(ctx, 20, AdminListFilters{Search: suffix})
	if err != nil {
		t.Fatalf("ListAdmin() returned error: %v", err)
	}
	var found Notification
	for _, item := range adminItems {
		if item.ID == visible.ID {
			found = item
			break
		}
	}
	if found.ID == "" {
		t.Fatalf("visible notification not found in admin items: %#v", adminItems)
	}
	if found.AudienceCount != 1 {
		t.Fatalf("AudienceCount = %d, want 1", found.AudienceCount)
	}
	if err := store.MarkRead(ctx, highUserID, []string{visible.ID}); err != nil {
		t.Fatalf("MarkRead() returned error: %v", err)
	}
	unread, err = store.UnreadCount(ctx, highUserID)
	if err != nil {
		t.Fatalf("UnreadCount(high after read) returned error: %v", err)
	}
	if unread != 0 {
		t.Fatalf("UnreadCount(high after read) = %d, want 0", unread)
	}
}

func findNotification(items []Notification, id string) (Notification, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return Notification{}, false
}

func hasNotification(items []Notification, id string) bool {
	_, ok := findNotification(items, id)
	return ok
}

func seedNotificationUser(t *testing.T, ctx context.Context, db *sql.DB, userID string, username string, balance int64) {
	t.Helper()
	now := time.Now().UTC()
	nowText := now.Format(time.RFC3339Nano)
	_, err := db.ExecContext(ctx, `INSERT INTO business_users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES($1, $2, $3, $4, 'user', 'active', $5, $6)`,
		userID,
		username,
		username+"@example.test",
		"test",
		nowText,
		nowText,
	)
	if err != nil {
		t.Fatalf("insert test user %q: %v", userID, err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO business_user_credits(user_id, balance, updated_at)
		VALUES($1, $2, $3)`, userID, balance, nowText)
	if err != nil {
		t.Fatalf("insert test credit %q: %v", userID, err)
	}
}

func cleanupNotificationTestRows(ctx context.Context, db *sql.DB, suffix string, userIDs ...string) {
	_, _ = db.ExecContext(ctx, `DELETE FROM business_notifications WHERE title LIKE $1`, "%"+suffix)
	for _, userID := range userIDs {
		_, _ = db.ExecContext(ctx, `DELETE FROM business_notification_reads WHERE user_id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM business_user_credits WHERE user_id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM business_users WHERE id = $1`, userID)
	}
}
