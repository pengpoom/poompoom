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
	pendingOrder, err := store.CreateOrder(ctx, CreateOrderInput{
		UserID:    userID,
		Username:  "pay_user_" + suffix,
		UserEmail: "pay_user_" + suffix + "@example.com",
		PackageID: pkg.ID,
	})
	if err != nil {
		t.Fatalf("create pending order: %v", err)
	}
	allOrders, total, err := store.ListOrders(ctx, OrderFilters{UserID: userID}, 1, 0)
	if err != nil {
		t.Fatalf("list paged orders: %v", err)
	}
	if total != 2 || len(allOrders) != 1 || allOrders[0].ID != pendingOrder.ID {
		t.Fatalf("paged orders total=%d items=%#v, want newest pending order with total 2", total, allOrders)
	}
	completedOrders, total, err := store.ListOrders(ctx, OrderFilters{
		UserID: userID,
		Status: StatusCompleted,
		Kind:   PackageTypeBalance,
		Search: userID,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list filtered orders: %v", err)
	}
	if total != 1 || len(completedOrders) != 1 || completedOrders[0].ID != result.Order.ID {
		t.Fatalf("filtered orders total=%d items=%#v, want completed balance order", total, completedOrders)
	}
	subscriptionOrders, total, err := store.ListOrders(ctx, OrderFilters{UserID: userID, Kind: PackageTypeSubscription}, 10, 0)
	if err != nil {
		t.Fatalf("list subscription orders: %v", err)
	}
	if total != 0 || len(subscriptionOrders) != 0 {
		t.Fatalf("subscription orders total=%d items=%#v, want empty", total, subscriptionOrders)
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
	generationID := "sub-generation-" + suffix
	reserved, err := store.ReserveSubscriptionCredits(ctx, userID, 40, generationID)
	if err != nil {
		t.Fatalf("reserve subscription credits: %v", err)
	}
	if reserved.Reserved != 40 || reserved.CreditsLeft != 160 {
		t.Fatalf("reserved subscription credits = %#v", reserved)
	}
	refunded, err := store.RefundSubscriptionCredits(ctx, userID, 10, generationID)
	if err != nil {
		t.Fatalf("refund subscription credits: %v", err)
	}
	if refunded.Reserved != 10 || refunded.CreditsLeft != 170 {
		t.Fatalf("refunded subscription credits = %#v", refunded)
	}
	refundedAgain, err := store.RefundSubscriptionCredits(ctx, userID, 40, generationID)
	if err != nil {
		t.Fatalf("second refund subscription credits: %v", err)
	}
	if refundedAgain.Reserved != 30 || refundedAgain.CreditsLeft != 200 {
		t.Fatalf("second refunded subscription credits = %#v", refundedAgain)
	}
	refundedThird, err := store.RefundSubscriptionCredits(ctx, userID, 40, generationID)
	if err != nil {
		t.Fatalf("third refund subscription credits: %v", err)
	}
	if refundedThird.Reserved != 0 {
		t.Fatalf("third refunded subscription credits = %#v, want no-op", refundedThird)
	}
	totals, err := store.SubscriptionGenerationTotals(ctx, userID, generationID)
	if err != nil {
		t.Fatalf("subscription generation totals: %v", err)
	}
	if totals.Reserved != 40 || totals.Refunded != 40 {
		t.Fatalf("subscription generation totals = %#v, want reserved 40 refunded 40", totals)
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
	var secondExpiresRaw time.Time
	if err := db.QueryRowContext(ctx, `SELECT expires_at FROM business_user_subscriptions WHERE order_id = $1`, secondOrder.ID).Scan(&secondExpiresRaw); err != nil {
		t.Fatalf("load second subscription expiry: %v", err)
	}
	secondExpires := secondExpiresRaw.UTC()
	if secondSub.OrderID != firstSub.OrderID {
		t.Fatalf("current subscription order = %s, want still current active order %s", secondSub.OrderID, firstSub.OrderID)
	}
	coverageExpires, err := time.Parse(time.RFC3339Nano, secondSub.CoverageExpiresAt)
	if err != nil {
		t.Fatalf("parse coverage expires: %v", err)
	}
	if !secondExpires.After(firstExpires.AddDate(0, 0, 12)) {
		t.Fatalf("second expires = %s, want about subscription duration after %s", secondExpires.Format(time.RFC3339Nano), firstSub.ExpiresAt)
	}
	if !coverageExpires.Equal(secondExpires) {
		t.Fatalf("coverage expires = %s, want %s", secondSub.CoverageExpiresAt, secondExpires.Format(time.RFC3339Nano))
	}
	if _, err := db.ExecContext(ctx, `UPDATE business_user_subscriptions
		SET credits_used = credits_total, updated_at = $1
		WHERE order_id = $2`, time.Now().UTC(), firstOrder.ID); err != nil {
		t.Fatalf("exhaust current subscription: %v", err)
	}
	futureReserve, err := store.ReserveSubscriptionCredits(ctx, userID, 1, "future-sub-generation-"+suffix)
	if err != nil {
		t.Fatalf("future reserve subscription credits: %v", err)
	}
	if futureReserve.Reserved != 0 {
		t.Fatalf("future reserve = %#v, want no credits from queued renewal", futureReserve)
	}
}

func TestSubscriptionUpgradeCoversAllRemainingSameDurationSubscriptions(t *testing.T) {
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

	suffix := time.Now().UTC().Format("20060102150405") + "_sub_upgrade"
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
	basePkg, err := store.CreatePackage(ctx, PackageInput{
		PackageType:  PackageTypeSubscription,
		Name:         "测试套餐",
		AmountCents:  3000,
		Credits:      300,
		DurationDays: 30,
		Currency:     "CNY",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create base package: %v", err)
	}
	upgradePkg, err := store.CreatePackage(ctx, PackageInput{
		PackageType:  PackageTypeSubscription,
		Name:         "测试套餐",
		AmountCents:  6000,
		Credits:      600,
		DurationDays: 30,
		Currency:     "CNY",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create upgrade package: %v", err)
	}
	differentDurationPkg, err := store.CreatePackage(ctx, PackageInput{
		PackageType:  PackageTypeSubscription,
		Name:         "测试套餐",
		AmountCents:  9000,
		Credits:      900,
		DurationDays: 60,
		Currency:     "CNY",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create different duration package: %v", err)
	}

	for i := 0; i < 2; i++ {
		order, err := store.CreateOrder(ctx, CreateOrderInput{
			UserID:    userID,
			Username:  "pay_user_" + suffix,
			UserEmail: "pay_user_" + suffix + "@example.com",
			PackageID: basePkg.ID,
		})
		if err != nil {
			t.Fatalf("create base order %d: %v", i+1, err)
		}
		if _, err := store.CompleteOrder(ctx, CompleteOrderInput{OrderID: order.ID, Operator: "test_admin"}); err != nil {
			t.Fatalf("complete base order %d: %v", i+1, err)
		}
	}
	currentSub, err := store.GetCurrentSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("current subscription: %v", err)
	}
	currentStarts, err := time.Parse(time.RFC3339Nano, currentSub.StartsAt)
	if err != nil {
		t.Fatalf("parse current starts: %v", err)
	}
	firstStarts := currentStarts.AddDate(0, 0, -15)
	firstExpires := currentStarts.AddDate(0, 0, 15)
	secondStarts := firstExpires
	secondExpires := secondStarts.AddDate(0, 0, 30)
	if _, err := db.ExecContext(ctx, `UPDATE business_user_subscriptions
		SET starts_at = $1, expires_at = $2, updated_at = $3
		WHERE order_id = $4`, firstStarts, firstExpires, now, currentSub.OrderID); err != nil {
		t.Fatalf("shape current subscription: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE business_user_subscriptions
		SET starts_at = $1, expires_at = $2, updated_at = $3
		WHERE user_id = $4 AND order_id <> $5`, secondStarts, secondExpires, now, userID, currentSub.OrderID); err != nil {
		t.Fatalf("shape subscriptions: %v", err)
	}

	if _, err := store.CreateOrder(ctx, CreateOrderInput{
		UserID:    userID,
		Username:  "pay_user_" + suffix,
		UserEmail: "pay_user_" + suffix + "@example.com",
		PackageID: differentDurationPkg.ID,
	}); err != ErrSubscriptionDurationMismatch {
		t.Fatalf("different duration upgrade error = %v, want %v", err, ErrSubscriptionDurationMismatch)
	}

	upgradeOrder, err := store.CreateOrder(ctx, CreateOrderInput{
		UserID:    userID,
		Username:  "pay_user_" + suffix,
		UserEmail: "pay_user_" + suffix + "@example.com",
		PackageID: upgradePkg.ID,
	})
	if err != nil {
		t.Fatalf("create upgrade order: %v", err)
	}
	if upgradeOrder.BillingAction != BillingActionUpgrade {
		t.Fatalf("billing action = %q, want upgrade", upgradeOrder.BillingAction)
	}
	if upgradeOrder.OriginalAmountCents < 8800 || upgradeOrder.OriginalAmountCents > 9200 {
		t.Fatalf("upgrade original amount = %d, want about 9000", upgradeOrder.OriginalAmountCents)
	}
	if upgradeOrder.UpgradeCreditCents < 4300 || upgradeOrder.UpgradeCreditCents > 4700 {
		t.Fatalf("upgrade credit = %d, want about 4500", upgradeOrder.UpgradeCreditCents)
	}
	if upgradeOrder.AmountCents < 4300 || upgradeOrder.AmountCents > 4700 {
		t.Fatalf("upgrade amount = %d, want about 4500", upgradeOrder.AmountCents)
	}
	result, err := store.CompleteOrder(ctx, CompleteOrderInput{OrderID: upgradeOrder.ID, Operator: "test_admin"})
	if err != nil {
		t.Fatalf("complete upgrade order: %v", err)
	}
	upgradedSub, err := store.GetCurrentSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("upgraded subscription: %v", err)
	}
	if upgradedSub.OrderID != result.Order.ID || upgradedSub.PackageID != upgradePkg.ID {
		t.Fatalf("upgraded subscription = %#v", upgradedSub)
	}
	upgradedExpires, err := time.Parse(time.RFC3339Nano, upgradedSub.ExpiresAt)
	if err != nil {
		t.Fatalf("parse upgraded expires: %v", err)
	}
	if upgradedExpires.Before(secondExpires.Add(-time.Hour)) || upgradedExpires.After(secondExpires.Add(time.Hour)) {
		t.Fatalf("upgraded expires = %s, want about %s", upgradedSub.ExpiresAt, secondExpires.Format(time.RFC3339Nano))
	}
	items, total, err := store.ListSubscriptions(ctx, SubscriptionFilters{Status: SubscriptionStatusUpgraded}, 10, 0)
	if err != nil {
		t.Fatalf("list upgraded subscriptions: %v", err)
	}
	upgradedRows := 0
	for _, item := range items {
		if item.UserID == userID {
			upgradedRows++
		}
	}
	if total < 2 || upgradedRows != 2 {
		t.Fatalf("upgraded rows = %d total = %d, want two user rows", upgradedRows, total)
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
