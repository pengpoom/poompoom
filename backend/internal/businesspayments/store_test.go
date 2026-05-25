package businesspayments

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"imagestudio/internal/businesscredits"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func TestManualOrderCompletionGrantsCreditsAndCommission(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	ctx := context.Background()
	cfg := databaseTestConfig(dsn)
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405")
	referrerID := "pay_referrer_" + suffix
	userID := "pay_user_" + suffix
	cleanupPaymentTestData(t, db, referrerID, userID)
	defer cleanupPaymentTestData(t, db, referrerID, userID)

	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO business_users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $7, $7), ($8, $9, $10, $4, $5, $6, $7, $7)`,
		referrerID,
		"pay_referrer_"+suffix,
		"pay_referrer_"+suffix+"@example.com",
		"hash",
		"user",
		"active",
		now,
		userID,
		"pay_user_"+suffix,
		"pay_user_"+suffix+"@example.com",
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO business_affiliate_referrals(id, referrer_user_id, referred_user_id, affiliate_code_preview, status, created_at)
		VALUES($1, $2, $3, $4, $5, $6)`, "pay_ref_"+suffix, referrerID, userID, "AFFTEST", "bound", now); err != nil {
		t.Fatalf("insert referral: %v", err)
	}

	store := NewStoreWithDB(db, cfg.Database.Driver)
	pkg, err := store.CreatePackage(ctx, PackageInput{
		Name:        "测试套餐",
		AmountCents: 990,
		Credits:     100,
		Currency:    "CNY",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	order, err := store.CreateOrder(ctx, CreateOrderInput{
		UserID:    userID,
		Username:  "pay_user_" + suffix,
		UserEmail: "pay_user_" + suffix + "@example.com",
		PackageID: pkg.ID,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	result, err := store.CompleteOrder(ctx, CompleteOrderInput{
		OrderID:           order.ID,
		Operator:          "test_admin",
		CommissionRateBPS: 1000,
	})
	if err != nil {
		t.Fatalf("complete order: %v", err)
	}
	if result.Order.Status != StatusCompleted || result.Order.CreditLedgerID == "" {
		t.Fatalf("completed order = %#v", result.Order)
	}
	if result.Commission.Credits != 10 || result.Commission.LedgerID == "" {
		t.Fatalf("commission = %#v, want 10 credits with ledger", result.Commission)
	}

	creditStore := businesscredits.NewStoreWithDB(db, cfg.Database.Driver)
	userCredit, err := creditStore.Summary(ctx, userID)
	if err != nil {
		t.Fatalf("user credit summary: %v", err)
	}
	if userCredit.Balance != 100 {
		t.Fatalf("user balance = %d, want 100", userCredit.Balance)
	}
	referrerCredit, err := creditStore.Summary(ctx, referrerID)
	if err != nil {
		t.Fatalf("referrer credit summary: %v", err)
	}
	if referrerCredit.Balance != 10 {
		t.Fatalf("referrer balance = %d, want 10", referrerCredit.Balance)
	}
}

func TestSubscriptionOrderCompletionExtendsSubscription(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	ctx := context.Background()
	cfg := databaseTestConfig(dsn)
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405") + "_sub"
	userID := "pay_user_" + suffix
	cleanupPaymentTestData(t, db, "", userID)
	defer cleanupPaymentTestData(t, db, "", userID)

	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO business_users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $7, $7)`,
		userID,
		"pay_user_"+suffix,
		"pay_user_"+suffix+"@example.com",
		"hash",
		"user",
		"active",
		now,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	store := NewStoreWithDB(db, cfg.Database.Driver)
	pkg, err := store.CreatePackage(ctx, PackageInput{
		PackageType:  PackageTypeSubscription,
		Name:         "测试套餐",
		AmountCents:  1990,
		Credits:      200,
		DurationDays: 15,
		Currency:     "CNY",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	firstOrder, err := store.CreateOrder(ctx, CreateOrderInput{
		UserID:    userID,
		Username:  "pay_user_" + suffix,
		UserEmail: "pay_user_" + suffix + "@example.com",
		PackageID: pkg.ID,
	})
	if err != nil {
		t.Fatalf("create first order: %v", err)
	}
	firstResult, err := store.CompleteOrder(ctx, CompleteOrderInput{OrderID: firstOrder.ID, Operator: "test_admin"})
	if err != nil {
		t.Fatalf("complete first order: %v", err)
	}
	if firstResult.Order.PackageType != PackageTypeSubscription {
		t.Fatalf("first order package type = %q", firstResult.Order.PackageType)
	}
	creditStore := businesscredits.NewStoreWithDB(db, cfg.Database.Driver)
	userCredit, err := creditStore.Summary(ctx, userID)
	if err != nil {
		t.Fatalf("user credit summary: %v", err)
	}
	if userCredit.Balance != 0 {
		t.Fatalf("subscription order added balance = %d, want 0", userCredit.Balance)
	}
	firstSub, err := store.GetCurrentSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("first subscription: %v", err)
	}
	if !firstSub.Active || firstSub.OrderID != firstOrder.ID || firstSub.CreditsTotal != 200 || firstSub.CreditsLeft != 200 || firstSub.DurationDays != 15 {
		t.Fatalf("first subscription = %#v", firstSub)
	}
	reserved, err := store.ReserveSubscriptionCredits(ctx, userID, 40, "sub-generation-"+suffix)
	if err != nil {
		t.Fatalf("reserve subscription credits: %v", err)
	}
	if reserved.Reserved != 40 || reserved.CreditsLeft != 160 {
		t.Fatalf("reserved subscription credits = %#v", reserved)
	}
	refunded, err := store.RefundSubscriptionCredits(ctx, userID, 10, "sub-generation-"+suffix)
	if err != nil {
		t.Fatalf("refund subscription credits: %v", err)
	}
	if refunded.Reserved != 10 || refunded.CreditsLeft != 170 {
		t.Fatalf("refunded subscription credits = %#v", refunded)
	}

	secondOrder, err := store.CreateOrder(ctx, CreateOrderInput{
		UserID:    userID,
		Username:  "pay_user_" + suffix,
		UserEmail: "pay_user_" + suffix + "@example.com",
		PackageID: pkg.ID,
	})
	if err != nil {
		t.Fatalf("create second order: %v", err)
	}
	if _, err := store.CompleteOrder(ctx, CompleteOrderInput{OrderID: secondOrder.ID, Operator: "test_admin"}); err != nil {
		t.Fatalf("complete second order: %v", err)
	}
	secondSub, err := store.GetCurrentSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("second subscription: %v", err)
	}
	firstExpires, err := time.Parse(time.RFC3339Nano, firstSub.ExpiresAt)
	if err != nil {
		t.Fatalf("parse first expires: %v", err)
	}
	secondExpires, err := time.Parse(time.RFC3339Nano, secondSub.ExpiresAt)
	if err != nil {
		t.Fatalf("parse second expires: %v", err)
	}
	if !secondExpires.After(firstExpires.AddDate(0, 0, 12)) {
		t.Fatalf("second expires = %s, want about subscription duration after %s", secondSub.ExpiresAt, firstSub.ExpiresAt)
	}
}

func TestSubscriptionRefundAndExpiry(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	ctx := context.Background()
	cfg := databaseTestConfig(dsn)
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405") + "_sub_cancel"
	userID := "pay_user_" + suffix
	cleanupPaymentTestData(t, db, "", userID)
	defer cleanupPaymentTestData(t, db, "", userID)

	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO business_users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $7, $7)`,
		userID,
		"pay_user_"+suffix,
		"pay_user_"+suffix+"@example.com",
		"hash",
		"user",
		"active",
		now,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	store := NewStoreWithDB(db, cfg.Database.Driver)
	pkg, err := store.CreatePackage(ctx, PackageInput{
		PackageType:  PackageTypeSubscription,
		Name:         "测试套餐",
		AmountCents:  1990,
		Credits:      200,
		DurationDays: 30,
		Currency:     "CNY",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	order, err := store.CreateOrder(ctx, CreateOrderInput{
		UserID:    userID,
		Username:  "pay_user_" + suffix,
		UserEmail: "pay_user_" + suffix + "@example.com",
		PackageID: pkg.ID,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := store.CompleteOrder(ctx, CompleteOrderInput{OrderID: order.ID, Operator: "test_admin"}); err != nil {
		t.Fatalf("complete order: %v", err)
	}
	if _, err := store.RefundOrder(ctx, order.ID, "test_admin"); err != nil {
		t.Fatalf("refund order: %v", err)
	}
	cancelledSub, err := store.GetCurrentSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("cancelled subscription: %v", err)
	}
	if cancelledSub.Active || cancelledSub.Status != SubscriptionStatusCancelled {
		t.Fatalf("cancelled subscription = %#v", cancelledSub)
	}

	expiringOrder, err := store.CreateOrder(ctx, CreateOrderInput{
		UserID:    userID,
		Username:  "pay_user_" + suffix,
		UserEmail: "pay_user_" + suffix + "@example.com",
		PackageID: pkg.ID,
	})
	if err != nil {
		t.Fatalf("create expiring order: %v", err)
	}
	if _, err := store.CompleteOrder(ctx, CompleteOrderInput{OrderID: expiringOrder.ID, Operator: "test_admin"}); err != nil {
		t.Fatalf("complete expiring order: %v", err)
	}
	activeSub, err := store.GetCurrentSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("active subscription: %v", err)
	}
	if !activeSub.Active {
		t.Fatalf("active subscription = %#v", activeSub)
	}
	if _, err := db.ExecContext(ctx, `UPDATE business_user_subscriptions SET expires_at = $1 WHERE id = $2`, time.Now().UTC().Add(-time.Hour), activeSub.ID); err != nil {
		t.Fatalf("force expire subscription: %v", err)
	}
	expiredSub, err := store.GetCurrentSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("expired subscription: %v", err)
	}
	if expiredSub.Active || expiredSub.Status != SubscriptionStatusExpired {
		t.Fatalf("expired subscription = %#v", expiredSub)
	}
}

func cleanupPaymentTestData(t *testing.T, db *sql.DB, referrerID string, userID string) {
	t.Helper()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM business_payment_audit_logs WHERE order_id IN (SELECT id FROM business_payment_orders WHERE user_id IN ($1, $2))`, referrerID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_affiliate_commissions WHERE order_id IN (SELECT id FROM business_payment_orders WHERE user_id IN ($1, $2))`, referrerID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_user_subscriptions WHERE user_id IN ($1, $2)`, referrerID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_payment_orders WHERE user_id IN ($1, $2)`, referrerID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_payment_packages WHERE name = $1`, "测试套餐")
	_, _ = db.ExecContext(ctx, `DELETE FROM business_affiliate_referrals WHERE referrer_user_id IN ($1, $2) OR referred_user_id IN ($1, $2)`, referrerID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_credit_ledger WHERE user_id IN ($1, $2)`, referrerID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_user_credits WHERE user_id IN ($1, $2)`, referrerID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_users WHERE id IN ($1, $2)`, referrerID, userID)
}

func databaseTestConfig(dsn string) *config.Config {
	cfg := config.New("")
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 4
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60
	return cfg
}
