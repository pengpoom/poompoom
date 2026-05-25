package businesspayments

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/businesscredits"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

const (
	ProviderManual = "manual"

	PackageTypeBalance      = "balance"
	PackageTypeSubscription = "subscription"
	PackageTypeMonthly      = "monthly"

	StatusPending   = "pending"
	StatusPaid      = "paid"
	StatusCompleted = "completed"
	StatusExpired   = "expired"
	StatusCancelled = "cancelled"
	StatusFailed    = "failed"
	StatusRefunded  = "refunded"

	SubscriptionStatusActive    = "active"
	SubscriptionStatusExpired   = "expired"
	SubscriptionStatusCancelled = "cancelled"

	CommissionStatusSettled  = "settled"
	CommissionStatusReversed = "reversed"

	ReasonPaymentRecharge        = "payment_recharge"
	ReasonSubscriptionReserve    = "subscription_credit_reserve"
	ReasonSubscriptionRefund     = "subscription_credit_refund"
	ReasonAffiliateOrderReward   = "affiliate_order_commission"
	ReasonAffiliateOrderReversal = "affiliate_order_commission_reversal"

	SourceTypePaymentOrder        = "payment_order"
	SourceTypeSubscription        = "subscription"
	SourceTypeAffiliateCommission = "affiliate_commission"
)

var (
	ErrPackageInvalid     = errors.New("payment package is invalid")
	ErrPackageDisabled    = errors.New("payment package is disabled")
	ErrPackageInUse       = errors.New("payment package is used by orders")
	ErrOrderInvalid       = errors.New("payment order is invalid")
	ErrOrderNotPayable    = errors.New("payment order is not payable")
	ErrOrderNotRefundable = errors.New("payment order is not refundable")
)

type Package struct {
	ID           string `json:"id"`
	PackageType  string `json:"packageType"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	AmountCents  int64  `json:"amountCents"`
	Credits      int64  `json:"credits"`
	DurationDays int    `json:"durationDays"`
	Currency     string `json:"currency"`
	Enabled      bool   `json:"enabled"`
	SortOrder    int    `json:"sortOrder"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type PackageInput struct {
	PackageType  string
	Name         string
	Description  string
	AmountCents  int64
	Credits      int64
	DurationDays int
	Currency     string
	Enabled      bool
	SortOrder    int
}

type Provider struct {
	ID               string   `json:"id"`
	ProviderKey      string   `json:"providerKey"`
	Name             string   `json:"name"`
	Enabled          bool     `json:"enabled"`
	SupportedMethods []string `json:"supportedMethods"`
	SortOrder        int      `json:"sortOrder"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

type Order struct {
	ID                 string `json:"id"`
	UserID             string `json:"userId"`
	UserEmail          string `json:"userEmail"`
	Username           string `json:"username"`
	PackageID          string `json:"packageId"`
	PackageType        string `json:"packageType"`
	AmountCents        int64  `json:"amountCents"`
	Credits            int64  `json:"credits"`
	DurationDays       int    `json:"durationDays"`
	Currency           string `json:"currency"`
	ProviderKey        string `json:"providerKey"`
	ProviderInstanceID string `json:"providerInstanceId"`
	OutTradeNo         string `json:"outTradeNo"`
	ProviderTradeNo    string `json:"providerTradeNo"`
	Status             string `json:"status"`
	PayURL             string `json:"payUrl"`
	QRCode             string `json:"qrCode"`
	ExpiresAt          string `json:"expiresAt,omitempty"`
	PaidAt             string `json:"paidAt,omitempty"`
	CompletedAt        string `json:"completedAt,omitempty"`
	FailedAt           string `json:"failedAt,omitempty"`
	RefundedAt         string `json:"refundedAt,omitempty"`
	CreditLedgerID     string `json:"creditLedgerId,omitempty"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type OrderWithCommission struct {
	Order      Order      `json:"order"`
	Commission Commission `json:"commission,omitempty"`
}

type Commission struct {
	ID              string `json:"id"`
	OrderID         string `json:"orderId"`
	ReferrerUserID  string `json:"referrerUserId"`
	ReferredUserID  string `json:"referredUserId"`
	BaseAmountCents int64  `json:"baseAmountCents"`
	RateBPS         int    `json:"rateBps"`
	Credits         int64  `json:"credits"`
	LedgerID        string `json:"ledgerId,omitempty"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
	SettledAt       string `json:"settledAt,omitempty"`
	ReversedAt      string `json:"reversedAt,omitempty"`
}

type ReserveResult struct {
	SubscriptionID string `json:"subscriptionId,omitempty"`
	CreditsUsed    int64  `json:"creditsUsed"`
	CreditsLeft    int64  `json:"creditsLeft"`
	Reserved       int64  `json:"reserved"`
}

type Subscription struct {
	ID           string `json:"id"`
	UserID       string `json:"userId"`
	OrderID      string `json:"orderId"`
	PackageID    string `json:"packageId"`
	PackageName  string `json:"packageName"`
	DurationDays int    `json:"durationDays"`
	CreditsTotal int64  `json:"creditsTotal"`
	CreditsUsed  int64  `json:"creditsUsed"`
	CreditsLeft  int64  `json:"creditsLeft"`
	Status       string `json:"status"`
	Active       bool   `json:"active"`
	StartsAt     string `json:"startsAt"`
	ExpiresAt    string `json:"expiresAt"`
	CancelledAt  string `json:"cancelledAt,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type SubscriptionTotals struct {
	Reserved int64
	Refunded int64
}

type AuditLog struct {
	ID        string `json:"id"`
	OrderID   string `json:"orderId"`
	Action    string `json:"action"`
	Detail    any    `json:"detail,omitempty"`
	Operator  string `json:"operator"`
	CreatedAt string `json:"createdAt"`
}

type OrderFilters struct {
	UserID string
	Status string
}

type CreateOrderInput struct {
	UserID    string
	Username  string
	UserEmail string
	PackageID string
}

type CompleteOrderInput struct {
	OrderID           string
	Operator          string
	ProviderTradeNo   string
	CommissionRateBPS int
}

type PaymentProvider interface {
	Key() string
	CreatePayment(context.Context, Order) (ProviderPayment, error)
}

type ProviderPayment struct {
	PayURL string
	QRCode string
}

type ManualProvider struct{}

func (ManualProvider) Key() string { return ProviderManual }

func (ManualProvider) CreatePayment(context.Context, Order) (ProviderPayment, error) {
	return ProviderPayment{}, nil
}

type Store struct {
	db     *sql.DB
	driver string
	ownDB  bool
}

func NewStore(cfg *config.Config) (*Store, error) {
	if !database.IsPostgres(cfg.Database.Driver) {
		return nil, fmt.Errorf("unsupported database driver %q", strings.TrimSpace(cfg.Database.Driver))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := NewStoreWithDB(db, cfg.Database.Driver)
	store.ownDB = true
	return store, nil
}

func NewStoreWithDB(db *sql.DB, driver string) *Store {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		driver = "postgres"
	}
	return &Store{db: db, driver: driver}
}

func (s *Store) Close() error {
	if s == nil || s.db == nil || !s.ownDB {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	items := []Provider{{
		ID:               ProviderManual,
		ProviderKey:      ProviderManual,
		Name:             "人工确认",
		Enabled:          true,
		SupportedMethods: []string{"manual"},
		SortOrder:        0,
		CreatedAt:        "",
		UpdatedAt:        "",
	}}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id, provider_key, name, enabled, supported_methods, sort_order, created_at, updated_at
		FROM business_payment_providers
		ORDER BY sort_order ASC, created_at DESC`))
	if err != nil {
		return items, nil
	}
	defer rows.Close()
	for rows.Next() {
		var item Provider
		var enabled int
		var methodsRaw string
		if err := rows.Scan(&item.ID, &item.ProviderKey, &item.Name, &enabled, &methodsRaw, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.SupportedMethods = splitCSV(methodsRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListPackages(ctx context.Context, includeDisabled bool) ([]Package, error) {
	query := `SELECT id, package_type, name, description, amount_cents, credits, duration_days, currency, enabled, sort_order, created_at, updated_at
		FROM business_payment_packages`
	args := []any{}
	if !includeDisabled {
		query += ` WHERE enabled = ?`
		args = append(args, 1)
	}
	query += ` ORDER BY sort_order ASC, amount_cents ASC, created_at DESC`
	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Package{}
	for rows.Next() {
		item, err := scanPackage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreatePackage(ctx context.Context, input PackageInput) (Package, error) {
	item, err := normalizePackageInput(input)
	if err != nil {
		return Package{}, err
	}
	now := time.Now().UTC()
	item.ID = newID("pay_pkg")
	item.CreatedAt = now.Format(time.RFC3339Nano)
	item.UpdatedAt = item.CreatedAt
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_payment_packages(
			id, package_type, name, description, amount_cents, credits, duration_days, currency, enabled, sort_order, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		item.ID,
		item.PackageType,
		item.Name,
		item.Description,
		item.AmountCents,
		item.Credits,
		item.DurationDays,
		item.Currency,
		boolInt(item.Enabled),
		item.SortOrder,
		now,
		now,
	)
	if err != nil {
		return Package{}, err
	}
	return item, nil
}

func (s *Store) UpdatePackage(ctx context.Context, id string, input PackageInput) (Package, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Package{}, false, nil
	}
	item, err := normalizePackageInput(input)
	if err != nil {
		return Package{}, false, err
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_payment_packages
		SET package_type = ?, name = ?, description = ?, amount_cents = ?, credits = ?, duration_days = ?, currency = ?, enabled = ?, sort_order = ?, updated_at = ?
		WHERE id = ?`),
		item.PackageType,
		item.Name,
		item.Description,
		item.AmountCents,
		item.Credits,
		item.DurationDays,
		item.Currency,
		boolInt(item.Enabled),
		item.SortOrder,
		now,
		id,
	)
	if err != nil {
		return Package{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Package{}, false, err
	}
	if affected == 0 {
		return Package{}, false, nil
	}
	next, ok, err := s.GetPackage(ctx, id, true)
	return next, ok, err
}

func (s *Store) GetPackage(ctx context.Context, id string, includeDisabled bool) (Package, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Package{}, false, nil
	}
	query := `SELECT id, package_type, name, description, amount_cents, credits, duration_days, currency, enabled, sort_order, created_at, updated_at
		FROM business_payment_packages
		WHERE id = ?`
	args := []any{id}
	if !includeDisabled {
		query += ` AND enabled = ?`
		args = append(args, 1)
	}
	var item Package
	var enabled int
	err := s.db.QueryRowContext(ctx, s.rebind(query), args...).Scan(
		&item.ID,
		&item.PackageType,
		&item.Name,
		&item.Description,
		&item.AmountCents,
		&item.Credits,
		&item.DurationDays,
		&item.Currency,
		&enabled,
		&item.SortOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Package{}, false, nil
	}
	if err != nil {
		return Package{}, false, err
	}
	item.PackageType = normalizePackageType(item.PackageType)
	item.DurationDays = normalizeDurationDays(item.PackageType, item.DurationDays)
	item.Enabled = enabled != 0
	return item, true, nil
}

func (s *Store) DeletePackage(ctx context.Context, id string) (bool, error) {
	id = cleanID(id)
	if id == "" {
		return false, nil
	}
	var orderCount int
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM business_payment_orders WHERE package_id = ?`), id).Scan(&orderCount); err != nil {
		return false, err
	}
	if orderCount > 0 {
		return false, ErrPackageInUse
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_payment_packages WHERE id = ?`), id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) CreateOrder(ctx context.Context, input CreateOrderInput) (Order, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return Order{}, fmt.Errorf("user id is required")
	}
	pkg, ok, err := s.GetPackage(ctx, input.PackageID, false)
	if err != nil {
		return Order{}, err
	}
	if !ok {
		return Order{}, ErrPackageInvalid
	}
	if !pkg.Enabled {
		return Order{}, ErrPackageDisabled
	}
	now := time.Now().UTC()
	order := Order{
		ID:                 newID("pay_order"),
		UserID:             userID,
		UserEmail:          cleanText(input.UserEmail, 240),
		Username:           cleanText(input.Username, 120),
		PackageID:          pkg.ID,
		PackageType:        pkg.PackageType,
		AmountCents:        pkg.AmountCents,
		Credits:            pkg.Credits,
		DurationDays:       pkg.DurationDays,
		Currency:           pkg.Currency,
		ProviderKey:        ProviderManual,
		ProviderInstanceID: ProviderManual,
		OutTradeNo:         newOutTradeNo(),
		Status:             StatusPending,
		ExpiresAt:          now.Add(30 * time.Minute).Format(time.RFC3339Nano),
		CreatedAt:          now.Format(time.RFC3339Nano),
		UpdatedAt:          now.Format(time.RFC3339Nano),
	}
	packageSnapshot, _ := json.Marshal(pkg)
	providerSnapshot, _ := json.Marshal(map[string]any{
		"providerKey": ProviderManual,
		"name":        "人工确认",
	})
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO business_payment_orders(
			id, user_id, user_email, username, package_id, package_type, package_snapshot_json,
			amount_cents, credits, duration_days, currency, provider_key, provider_instance_id,
			provider_snapshot_json, out_trade_no, status, expires_at, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		order.ID,
		order.UserID,
		order.UserEmail,
		order.Username,
		order.PackageID,
		order.PackageType,
		packageSnapshot,
		order.AmountCents,
		order.Credits,
		order.DurationDays,
		order.Currency,
		order.ProviderKey,
		order.ProviderInstanceID,
		providerSnapshot,
		order.OutTradeNo,
		order.Status,
		parseStoredTime(order.ExpiresAt),
		now,
		now,
	)
	if err != nil {
		return Order{}, err
	}
	if err := s.writeAuditWithTx(ctx, tx, order.ID, "order_created", map[string]any{"packageId": pkg.ID}, userID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (s *Store) ListOrders(ctx context.Context, filters OrderFilters, limit int) ([]Order, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := []string{"1 = 1"}
	args := []any{}
	if userID := strings.TrimSpace(filters.UserID); userID != "" {
		where = append(where, "user_id = ?")
		args = append(args, userID)
	}
	if status := normalizeStatus(filters.Status); status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT
			id, user_id, user_email, username, package_id, package_type, amount_cents, credits, duration_days, currency,
			provider_key, provider_instance_id, out_trade_no, provider_trade_no, status,
			pay_url, qr_code, expires_at, paid_at, completed_at, failed_at, refunded_at,
			credit_ledger_id, created_at, updated_at
		FROM business_payment_orders
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, id DESC
		LIMIT ?`), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Order{}
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetOrder(ctx context.Context, id string, userID string) (Order, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Order{}, false, nil
	}
	query := `SELECT
			id, user_id, user_email, username, package_id, package_type, amount_cents, credits, duration_days, currency,
			provider_key, provider_instance_id, out_trade_no, provider_trade_no, status,
			pay_url, qr_code, expires_at, paid_at, completed_at, failed_at, refunded_at,
			credit_ledger_id, created_at, updated_at
		FROM business_payment_orders
		WHERE id = ?`
	args := []any{id}
	if strings.TrimSpace(userID) != "" {
		query += ` AND user_id = ?`
		args = append(args, strings.TrimSpace(userID))
	}
	var item Order
	err := s.db.QueryRowContext(ctx, s.rebind(query), args...).Scan(orderScanDest(&item)...)
	if err == sql.ErrNoRows {
		return Order{}, false, nil
	}
	if err != nil {
		return Order{}, false, err
	}
	normalizeOrder(&item)
	return item, true, nil
}

func (s *Store) CompleteOrder(ctx context.Context, input CompleteOrderInput) (OrderWithCommission, error) {
	orderID := cleanID(input.OrderID)
	if orderID == "" {
		return OrderWithCommission{}, ErrOrderInvalid
	}
	operator := cleanText(input.Operator, 120)
	if operator == "" {
		operator = "system"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OrderWithCommission{}, err
	}
	defer tx.Rollback()

	order, err := s.orderForUpdate(ctx, tx, orderID)
	if err != nil {
		return OrderWithCommission{}, err
	}
	if order.Status == StatusCompleted {
		commission, _ := s.getCommissionWithTx(ctx, tx, order.ID)
		return OrderWithCommission{Order: order, Commission: commission}, tx.Commit()
	}
	if order.Status != StatusPending && order.Status != StatusPaid {
		return OrderWithCommission{}, ErrOrderNotPayable
	}

	now := time.Now().UTC()
	creditStore := businesscredits.NewStoreWithDB(s.db, s.driver)
	ledgerID := ""
	if order.PackageType == PackageTypeBalance {
		ledger, err := creditStore.AddWithTxSource(ctx, tx, order.UserID, order.Credits, ReasonPaymentRecharge, SourceTypePaymentOrder, order.ID, false)
		if err != nil {
			return OrderWithCommission{}, err
		}
		ledgerID = ledger.ID
	}
	providerTradeNo := cleanText(input.ProviderTradeNo, 120)
	if providerTradeNo == "" {
		providerTradeNo = order.OutTradeNo
	}
	_, err = tx.ExecContext(ctx, s.rebind(`UPDATE business_payment_orders
		SET status = ?, provider_trade_no = ?, paid_at = COALESCE(paid_at, ?), completed_at = ?, credit_ledger_id = ?, updated_at = ?
		WHERE id = ?`),
		StatusCompleted,
		providerTradeNo,
		now,
		now,
		ledgerID,
		now,
		order.ID,
	)
	if err != nil {
		return OrderWithCommission{}, err
	}
	order.Status = StatusCompleted
	order.ProviderTradeNo = providerTradeNo
	order.PaidAt = now.Format(time.RFC3339Nano)
	order.CompletedAt = order.PaidAt
	order.CreditLedgerID = ledgerID
	order.UpdatedAt = order.PaidAt
	if order.PackageType == PackageTypeBalance {
		if err := s.writeAuditWithTx(ctx, tx, order.ID, "credit_granted", map[string]any{"ledgerId": ledgerID, "credits": order.Credits}, operator); err != nil {
			return OrderWithCommission{}, err
		}
	}
	if order.PackageType == PackageTypeSubscription {
		subscription, err := s.activateSubscriptionWithTx(ctx, tx, order, now)
		if err != nil {
			return OrderWithCommission{}, err
		}
		if err := s.writeAuditWithTx(ctx, tx, order.ID, "subscription_activated", map[string]any{
			"subscriptionId": subscription.ID,
			"startsAt":       subscription.StartsAt,
			"expiresAt":      subscription.ExpiresAt,
			"durationDays":   subscription.DurationDays,
			"creditsTotal":   subscription.CreditsTotal,
		}, operator); err != nil {
			return OrderWithCommission{}, err
		}
	}
	if order.PackageType != PackageTypeBalance && order.PackageType != PackageTypeSubscription {
		return OrderWithCommission{}, ErrPackageInvalid
	}
	commission, err := s.settleAffiliateCommissionWithTx(ctx, tx, creditStore, order, normalizeRateBPS(input.CommissionRateBPS), operator)
	if err != nil {
		return OrderWithCommission{}, err
	}
	if err := s.writeAuditWithTx(ctx, tx, order.ID, "order_completed", map[string]any{"providerTradeNo": providerTradeNo}, operator); err != nil {
		return OrderWithCommission{}, err
	}
	if err := tx.Commit(); err != nil {
		return OrderWithCommission{}, err
	}
	return OrderWithCommission{Order: order, Commission: commission}, nil
}

func (s *Store) CancelOrder(ctx context.Context, id string, operator string) (Order, bool, error) {
	return s.setTerminalStatus(ctx, id, operator, StatusCancelled, "order_cancelled")
}

func (s *Store) RefundOrder(ctx context.Context, id string, operator string) (OrderWithCommission, error) {
	orderID := cleanID(id)
	if orderID == "" {
		return OrderWithCommission{}, ErrOrderInvalid
	}
	operator = cleanText(operator, 120)
	if operator == "" {
		operator = "system"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OrderWithCommission{}, err
	}
	defer tx.Rollback()
	order, err := s.orderForUpdate(ctx, tx, orderID)
	if err != nil {
		return OrderWithCommission{}, err
	}
	if order.Status == StatusRefunded {
		commission, _ := s.getCommissionWithTx(ctx, tx, order.ID)
		return OrderWithCommission{Order: order, Commission: commission}, tx.Commit()
	}
	if order.Status != StatusCompleted {
		return OrderWithCommission{}, ErrOrderNotRefundable
	}
	creditStore := businesscredits.NewStoreWithDB(s.db, s.driver)
	ledgerID := ""
	if order.PackageType == PackageTypeBalance {
		ledger, err := creditStore.AddWithTxSource(ctx, tx, order.UserID, -order.Credits, "payment_refund", SourceTypePaymentOrder, order.ID, true)
		if err != nil {
			return OrderWithCommission{}, err
		}
		ledgerID = ledger.ID
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, s.rebind(`UPDATE business_payment_orders
		SET status = ?, refunded_at = ?, updated_at = ?
		WHERE id = ?`), StatusRefunded, now, now, order.ID)
	if err != nil {
		return OrderWithCommission{}, err
	}
	order.Status = StatusRefunded
	order.RefundedAt = now.Format(time.RFC3339Nano)
	order.UpdatedAt = order.RefundedAt
	commission, err := s.reverseAffiliateCommissionWithTx(ctx, tx, creditStore, order.ID, operator)
	if err != nil {
		return OrderWithCommission{}, err
	}
	if order.PackageType == PackageTypeBalance {
		if err := s.writeAuditWithTx(ctx, tx, order.ID, "refunded", map[string]any{"ledgerId": ledgerID, "credits": order.Credits}, operator); err != nil {
			return OrderWithCommission{}, err
		}
	}
	if order.PackageType == PackageTypeSubscription {
		if subscription, err := s.cancelSubscriptionWithTx(ctx, tx, order.ID); err != nil {
			return OrderWithCommission{}, err
		} else if subscription.ID != "" {
			if err := s.writeAuditWithTx(ctx, tx, order.ID, "subscription_cancelled", map[string]any{"subscriptionId": subscription.ID}, operator); err != nil {
				return OrderWithCommission{}, err
			}
		}
	}
	if order.PackageType != PackageTypeBalance && order.PackageType != PackageTypeSubscription {
		return OrderWithCommission{}, ErrPackageInvalid
	}
	if err := tx.Commit(); err != nil {
		return OrderWithCommission{}, err
	}
	return OrderWithCommission{Order: order, Commission: commission}, nil
}

func (s *Store) ListAuditLogs(ctx context.Context, orderID string, limit int) ([]AuditLog, error) {
	orderID = cleanID(orderID)
	if orderID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id, order_id, action, detail_json, operator, created_at
		FROM business_payment_audit_logs
		WHERE order_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?`), orderID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AuditLog{}
	for rows.Next() {
		var item AuditLog
		var raw []byte
		if err := rows.Scan(&item.ID, &item.OrderID, &item.Action, &raw, &item.Operator, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			var detail any
			if json.Unmarshal(raw, &detail) == nil {
				item.Detail = detail
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetCurrentSubscription(ctx context.Context, userID string) (Subscription, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Subscription{}, nil
	}
	if err := s.expireUserSubscriptions(ctx, userID, time.Now().UTC()); err != nil {
		return Subscription{}, err
	}
	now := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, s.rebind(`SELECT id, user_id, order_id, package_id, package_name,
			duration_days, credits_total, credits_used,
			status, starts_at, expires_at, cancelled_at, created_at, updated_at
		FROM business_user_subscriptions
		WHERE user_id = ? AND status = ? AND expires_at > ?
		ORDER BY expires_at DESC, created_at DESC
		LIMIT 1`), userID, SubscriptionStatusActive, now)
	item, err := scanSubscription(row)
	if err == nil {
		if err := s.populateActiveSubscriptionTotals(ctx, userID, now, &item); err != nil {
			return Subscription{}, err
		}
		return item, nil
	}
	if err == sql.ErrNoRows {
		row = s.db.QueryRowContext(ctx, s.rebind(`SELECT id, user_id, order_id, package_id, package_name,
				duration_days, credits_total, credits_used,
				status, starts_at, expires_at, cancelled_at, created_at, updated_at
			FROM business_user_subscriptions
			WHERE user_id = ?
			ORDER BY updated_at DESC, created_at DESC
			LIMIT 1`), userID)
		item, err = scanSubscription(row)
		if err == sql.ErrNoRows {
			return Subscription{}, nil
		}
	}
	if err != nil {
		return Subscription{}, err
	}
	return item, nil
}

func (s *Store) populateActiveSubscriptionTotals(ctx context.Context, userID string, now time.Time, item *Subscription) error {
	if item == nil || item.ID == "" {
		return nil
	}
	var startsAt sql.NullTime
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT
			COALESCE(SUM(credits_total), 0),
			COALESCE(SUM(credits_used), 0),
			MIN(starts_at)
		FROM business_user_subscriptions
		WHERE user_id = ? AND status = ? AND expires_at > ?`),
		userID,
		SubscriptionStatusActive,
		now,
	).Scan(&item.CreditsTotal, &item.CreditsUsed, &startsAt)
	if err != nil {
		return err
	}
	if startsAt.Valid {
		item.StartsAt = startsAt.Time.UTC().Format(time.RFC3339Nano)
	}
	item.CreditsLeft = item.CreditsTotal - item.CreditsUsed
	if item.CreditsLeft < 0 {
		item.CreditsLeft = 0
	}
	return nil
}

func (s *Store) ReserveSubscriptionCredits(ctx context.Context, userID string, amount int64, generationID string) (ReserveResult, error) {
	userID = strings.TrimSpace(userID)
	generationID = strings.TrimSpace(generationID)
	if userID == "" || amount <= 0 {
		return ReserveResult{}, nil
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReserveResult{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
		SET status = ?, updated_at = ?
		WHERE user_id = ? AND status = ? AND expires_at <= ?`),
		SubscriptionStatusExpired,
		now,
		userID,
		SubscriptionStatusActive,
		now,
	); err != nil {
		return ReserveResult{}, err
	}
	row := tx.QueryRowContext(ctx, s.rebind(`SELECT id, credits_total, credits_used
		FROM business_user_subscriptions
		WHERE user_id = ? AND status = ? AND expires_at > ? AND credits_total > credits_used
		ORDER BY expires_at ASC, created_at ASC
		LIMIT 1
		FOR UPDATE`), userID, SubscriptionStatusActive, now)
	var subscriptionID string
	var total int64
	var used int64
	if err := row.Scan(&subscriptionID, &total, &used); err == sql.ErrNoRows {
		return ReserveResult{}, nil
	} else if err != nil {
		return ReserveResult{}, err
	}
	available := total - used
	if available <= 0 {
		return ReserveResult{}, nil
	}
	reserved := amount
	if reserved > available {
		reserved = available
	}
	nextUsed := used + reserved
	if _, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
		SET credits_used = ?, updated_at = ?
		WHERE id = ?`), nextUsed, now, subscriptionID); err != nil {
		return ReserveResult{}, err
	}
	if generationID != "" {
		if _, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO business_credit_ledger(
			id, user_id, delta, balance_after, reason, generation_id, source_type, source_id, created_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`),
			newID("ledger"),
			userID,
			-reserved,
			total-nextUsed,
			ReasonSubscriptionReserve,
			generationID,
			SourceTypeSubscription,
			subscriptionID,
			now,
		); err != nil {
			return ReserveResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ReserveResult{}, err
	}
	return ReserveResult{SubscriptionID: subscriptionID, CreditsUsed: nextUsed, CreditsLeft: total - nextUsed, Reserved: reserved}, nil
}

func (s *Store) RefundSubscriptionCredits(ctx context.Context, userID string, amount int64, generationID string) (ReserveResult, error) {
	userID = strings.TrimSpace(userID)
	generationID = strings.TrimSpace(generationID)
	if userID == "" || amount <= 0 || generationID == "" {
		return ReserveResult{}, nil
	}
	totals, err := s.SubscriptionGenerationTotals(ctx, userID, generationID)
	if err != nil {
		return ReserveResult{}, err
	}
	refundable := totals.Reserved - totals.Refunded
	if refundable <= 0 {
		return ReserveResult{}, nil
	}
	if amount > refundable {
		amount = refundable
	}
	var subscriptionID string
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT source_id
		FROM business_credit_ledger
		WHERE user_id = ? AND generation_id = ? AND reason = ? AND source_type = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1`), userID, generationID, ReasonSubscriptionReserve, SourceTypeSubscription).Scan(&subscriptionID); err != nil {
		return ReserveResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReserveResult{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, s.rebind(`SELECT credits_total, credits_used
		FROM business_user_subscriptions
		WHERE id = ? AND user_id = ?
		FOR UPDATE`), subscriptionID, userID)
	var total int64
	var used int64
	if err := row.Scan(&total, &used); err != nil {
		return ReserveResult{}, err
	}
	refunded := amount
	if refunded > used {
		refunded = used
	}
	nextUsed := used - refunded
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
		SET credits_used = ?, updated_at = ?
		WHERE id = ?`), nextUsed, now, subscriptionID); err != nil {
		return ReserveResult{}, err
	}
	if _, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO business_credit_ledger(
		id, user_id, delta, balance_after, reason, generation_id, source_type, source_id, created_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		newID("ledger"),
		userID,
		refunded,
		total-nextUsed,
		ReasonSubscriptionRefund,
		generationID,
		SourceTypeSubscription,
		subscriptionID,
		now,
	); err != nil {
		return ReserveResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReserveResult{}, err
	}
	return ReserveResult{SubscriptionID: subscriptionID, CreditsUsed: nextUsed, CreditsLeft: total - nextUsed, Reserved: refunded}, nil
}

func (s *Store) SubscriptionGenerationTotals(ctx context.Context, userID string, generationID string) (SubscriptionTotals, error) {
	userID = strings.TrimSpace(userID)
	generationID = strings.TrimSpace(generationID)
	if userID == "" || generationID == "" {
		return SubscriptionTotals{}, nil
	}
	var totals SubscriptionTotals
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT
		    COALESCE(SUM(CASE WHEN reason = ? THEN -delta ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN reason = ? THEN delta ELSE 0 END), 0)
		   FROM business_credit_ledger
		  WHERE user_id = ? AND generation_id = ? AND source_type = ?`),
		ReasonSubscriptionReserve,
		ReasonSubscriptionRefund,
		userID,
		generationID,
		SourceTypeSubscription,
	).Scan(&totals.Reserved, &totals.Refunded)
	if err != nil {
		return SubscriptionTotals{}, err
	}
	if totals.Reserved < 0 {
		totals.Reserved = 0
	}
	if totals.Refunded < 0 {
		totals.Refunded = 0
	}
	return totals, nil
}

func (s *Store) setTerminalStatus(ctx context.Context, id string, operator string, status string, action string) (Order, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Order{}, false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, false, err
	}
	defer tx.Rollback()
	order, err := s.orderForUpdate(ctx, tx, id)
	if err == sql.ErrNoRows {
		return Order{}, false, nil
	}
	if err != nil {
		return Order{}, false, err
	}
	if order.Status != StatusPending && order.Status != StatusFailed && order.Status != StatusExpired {
		return Order{}, true, ErrOrderNotPayable
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, s.rebind(`UPDATE business_payment_orders
		SET status = ?, updated_at = ?
		WHERE id = ?`), status, now, id)
	if err != nil {
		return Order{}, false, err
	}
	order.Status = status
	order.UpdatedAt = now.Format(time.RFC3339Nano)
	if err := s.writeAuditWithTx(ctx, tx, order.ID, action, nil, operator); err != nil {
		return Order{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, false, err
	}
	return order, true, nil
}

func (s *Store) activateSubscriptionWithTx(ctx context.Context, tx *sql.Tx, order Order, now time.Time) (Subscription, error) {
	existing, err := s.currentActiveSubscriptionWithTx(ctx, tx, order.UserID, now)
	if err != nil {
		return Subscription{}, err
	}
	startsAt := now
	if existing.ExpiresAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, existing.ExpiresAt); err == nil && parsed.After(now) {
			startsAt = parsed.UTC()
		}
	}
	durationDays := normalizeDurationDays(PackageTypeSubscription, order.DurationDays)
	expiresAt := startsAt.AddDate(0, 0, durationDays)
	pkgName := order.PackageID
	if name := s.packageNameFromOrderSnapshot(ctx, tx, order.ID); name != "" {
		pkgName = name
	}
	subscription := Subscription{
		ID:           newID("sub"),
		UserID:       order.UserID,
		OrderID:      order.ID,
		PackageID:    order.PackageID,
		PackageName:  pkgName,
		DurationDays: durationDays,
		CreditsTotal: order.Credits,
		CreditsLeft:  order.Credits,
		Status:       SubscriptionStatusActive,
		Active:       true,
		StartsAt:     startsAt.Format(time.RFC3339Nano),
		ExpiresAt:    expiresAt.Format(time.RFC3339Nano),
		CreatedAt:    now.Format(time.RFC3339Nano),
		UpdatedAt:    now.Format(time.RFC3339Nano),
	}
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO business_user_subscriptions(
			id, user_id, order_id, package_id, package_name, duration_days, credits_total, credits_used,
			status, starts_at, expires_at, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		subscription.ID,
		subscription.UserID,
		subscription.OrderID,
		subscription.PackageID,
		subscription.PackageName,
		subscription.DurationDays,
		subscription.CreditsTotal,
		subscription.CreditsUsed,
		subscription.Status,
		startsAt,
		expiresAt,
		now,
		now,
	)
	if err != nil {
		return Subscription{}, err
	}
	return subscription, nil
}

func (s *Store) currentActiveSubscriptionWithTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) (Subscription, error) {
	_, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
		SET status = ?, updated_at = ?
		WHERE user_id = ? AND status = ? AND expires_at <= ?`),
		SubscriptionStatusExpired,
		now,
		userID,
		SubscriptionStatusActive,
		now,
	)
	if err != nil {
		return Subscription{}, err
	}
	row := tx.QueryRowContext(ctx, s.rebind(`SELECT id, user_id, order_id, package_id, package_name,
			duration_days, credits_total, credits_used,
			status, starts_at, expires_at, cancelled_at, created_at, updated_at
		FROM business_user_subscriptions
		WHERE user_id = ? AND status = ? AND expires_at > ?
		ORDER BY expires_at DESC, created_at DESC
		LIMIT 1
		FOR UPDATE`), userID, SubscriptionStatusActive, now)
	item, err := scanSubscription(row)
	if err == sql.ErrNoRows {
		return Subscription{}, nil
	}
	return item, err
}

func (s *Store) cancelSubscriptionWithTx(ctx context.Context, tx *sql.Tx, orderID string) (Subscription, error) {
	now := time.Now().UTC()
	_, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
		SET status = ?, cancelled_at = ?, updated_at = ?
		WHERE order_id = ? AND status <> ?`),
		SubscriptionStatusCancelled,
		now,
		now,
		orderID,
		SubscriptionStatusCancelled,
	)
	if err != nil {
		return Subscription{}, err
	}
	row := tx.QueryRowContext(ctx, s.rebind(`SELECT id, user_id, order_id, package_id, package_name,
			duration_days, credits_total, credits_used,
			status, starts_at, expires_at, cancelled_at, created_at, updated_at
		FROM business_user_subscriptions
		WHERE order_id = ?`), orderID)
	item, err := scanSubscription(row)
	if err == sql.ErrNoRows {
		return Subscription{}, nil
	}
	return item, err
}

func (s *Store) expireUserSubscriptions(ctx context.Context, userID string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
		SET status = ?, updated_at = ?
		WHERE user_id = ? AND status = ? AND expires_at <= ?`),
		SubscriptionStatusExpired,
		now,
		userID,
		SubscriptionStatusActive,
		now,
	)
	return err
}

func (s *Store) packageNameFromOrderSnapshot(ctx context.Context, tx *sql.Tx, orderID string) string {
	var raw []byte
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT package_snapshot_json FROM business_payment_orders WHERE id = ?`), orderID).Scan(&raw); err != nil {
		return ""
	}
	var snapshot struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &snapshot) != nil {
		return ""
	}
	return cleanText(snapshot.Name, 80)
}

func (s *Store) settleAffiliateCommissionWithTx(ctx context.Context, tx *sql.Tx, creditStore *businesscredits.Store, order Order, rateBPS int, operator string) (Commission, error) {
	if rateBPS <= 0 {
		return Commission{}, nil
	}
	var referrerUserID string
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT referrer_user_id
		FROM business_affiliate_referrals
		WHERE referred_user_id = ?`), order.UserID).Scan(&referrerUserID)
	if err == sql.ErrNoRows {
		return Commission{}, nil
	}
	if err != nil {
		return Commission{}, err
	}
	if strings.TrimSpace(referrerUserID) == "" || referrerUserID == order.UserID {
		return Commission{}, nil
	}
	credits := order.Credits * int64(rateBPS) / 10000
	if credits <= 0 {
		return Commission{}, nil
	}
	existing, err := s.getCommissionWithTx(ctx, tx, order.ID)
	if err != nil && err != sql.ErrNoRows {
		return Commission{}, err
	}
	if existing.ID != "" {
		return existing, nil
	}
	now := time.Now().UTC()
	commission := Commission{
		ID:              newID("aff_comm"),
		OrderID:         order.ID,
		ReferrerUserID:  referrerUserID,
		ReferredUserID:  order.UserID,
		BaseAmountCents: order.AmountCents,
		RateBPS:         rateBPS,
		Credits:         credits,
		Status:          CommissionStatusSettled,
		CreatedAt:       now.Format(time.RFC3339Nano),
		SettledAt:       now.Format(time.RFC3339Nano),
	}
	ledger, err := creditStore.AddWithTxSource(ctx, tx, referrerUserID, credits, ReasonAffiliateOrderReward, SourceTypeAffiliateCommission, commission.ID, false)
	if err != nil {
		return Commission{}, err
	}
	commission.LedgerID = ledger.ID
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO business_affiliate_commissions(
			id, order_id, referrer_user_id, referred_user_id, base_amount_cents, rate_bps,
			credits, ledger_id, status, created_at, settled_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		commission.ID,
		commission.OrderID,
		commission.ReferrerUserID,
		commission.ReferredUserID,
		commission.BaseAmountCents,
		commission.RateBPS,
		commission.Credits,
		commission.LedgerID,
		commission.Status,
		now,
		now,
	)
	if err != nil {
		return Commission{}, err
	}
	if err := s.writeAuditWithTx(ctx, tx, order.ID, "affiliate_commission_created", map[string]any{"commissionId": commission.ID, "credits": credits}, operator); err != nil {
		return Commission{}, err
	}
	return commission, nil
}

func (s *Store) reverseAffiliateCommissionWithTx(ctx context.Context, tx *sql.Tx, creditStore *businesscredits.Store, orderID string, operator string) (Commission, error) {
	commission, err := s.getCommissionWithTx(ctx, tx, orderID)
	if err == sql.ErrNoRows || commission.ID == "" || commission.Status == CommissionStatusReversed {
		return commission, nil
	}
	if err != nil {
		return Commission{}, err
	}
	ledger, err := creditStore.AddWithTxSource(ctx, tx, commission.ReferrerUserID, -commission.Credits, ReasonAffiliateOrderReversal, SourceTypeAffiliateCommission, commission.ID, true)
	if err != nil {
		return Commission{}, err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, s.rebind(`UPDATE business_affiliate_commissions
		SET status = ?, reversed_at = ?, ledger_id = ?
		WHERE id = ?`), CommissionStatusReversed, now, ledger.ID, commission.ID)
	if err != nil {
		return Commission{}, err
	}
	commission.Status = CommissionStatusReversed
	commission.ReversedAt = now.Format(time.RFC3339Nano)
	commission.LedgerID = ledger.ID
	if err := s.writeAuditWithTx(ctx, tx, orderID, "affiliate_commission_reversed", map[string]any{"commissionId": commission.ID, "ledgerId": ledger.ID}, operator); err != nil {
		return Commission{}, err
	}
	return commission, nil
}

func (s *Store) getCommissionWithTx(ctx context.Context, tx *sql.Tx, orderID string) (Commission, error) {
	var item Commission
	var settledAt sql.NullTime
	var reversedAt sql.NullTime
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT id, order_id, referrer_user_id, referred_user_id,
			base_amount_cents, rate_bps, credits, ledger_id, status, created_at, settled_at, reversed_at
		FROM business_affiliate_commissions
		WHERE order_id = ?`), orderID).Scan(
		&item.ID,
		&item.OrderID,
		&item.ReferrerUserID,
		&item.ReferredUserID,
		&item.BaseAmountCents,
		&item.RateBPS,
		&item.Credits,
		&item.LedgerID,
		&item.Status,
		&item.CreatedAt,
		&settledAt,
		&reversedAt,
	)
	if err != nil {
		return Commission{}, err
	}
	item.SettledAt = formatNullTime(settledAt)
	item.ReversedAt = formatNullTime(reversedAt)
	return item, nil
}

func (s *Store) orderForUpdate(ctx context.Context, tx *sql.Tx, id string) (Order, error) {
	var item Order
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT
			id, user_id, user_email, username, package_id, package_type, amount_cents, credits, duration_days, currency,
			provider_key, provider_instance_id, out_trade_no, provider_trade_no, status,
			pay_url, qr_code, expires_at, paid_at, completed_at, failed_at, refunded_at,
			credit_ledger_id, created_at, updated_at
		FROM business_payment_orders
		WHERE id = ?
		FOR UPDATE`), id).Scan(orderScanDest(&item)...)
	if err == nil {
		normalizeOrder(&item)
	}
	return item, err
}

func (s *Store) writeAuditWithTx(ctx context.Context, tx *sql.Tx, orderID string, action string, detail any, operator string) error {
	raw, _ := json.Marshal(detail)
	if detail == nil {
		raw = nil
	}
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO business_payment_audit_logs(id, order_id, action, detail_json, operator, created_at)
		VALUES(?, ?, ?, ?, ?, ?)`),
		newID("pay_log"),
		orderID,
		cleanText(action, 80),
		raw,
		cleanText(operator, 120),
		time.Now().UTC(),
	)
	return err
}

func scanPackage(row interface{ Scan(dest ...any) error }) (Package, error) {
	var item Package
	var enabled int
	if err := row.Scan(
		&item.ID,
		&item.PackageType,
		&item.Name,
		&item.Description,
		&item.AmountCents,
		&item.Credits,
		&item.DurationDays,
		&item.Currency,
		&enabled,
		&item.SortOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return Package{}, err
	}
	item.PackageType = normalizePackageType(item.PackageType)
	item.DurationDays = normalizeDurationDays(item.PackageType, item.DurationDays)
	item.Enabled = enabled != 0
	return item, nil
}

func scanOrder(row interface{ Scan(dest ...any) error }) (Order, error) {
	var item Order
	if err := row.Scan(orderScanDest(&item)...); err != nil {
		return Order{}, err
	}
	normalizeOrder(&item)
	return item, nil
}

func scanSubscription(row interface{ Scan(dest ...any) error }) (Subscription, error) {
	var item Subscription
	var cancelledAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.OrderID,
		&item.PackageID,
		&item.PackageName,
		&item.DurationDays,
		&item.CreditsTotal,
		&item.CreditsUsed,
		&item.Status,
		nullTimeScanner{target: &item.StartsAt},
		nullTimeScanner{target: &item.ExpiresAt},
		&cancelledAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return Subscription{}, err
	}
	item.CancelledAt = formatNullTime(cancelledAt)
	item.Active = item.Status == SubscriptionStatusActive
	item.DurationDays = normalizeDurationDays(PackageTypeSubscription, item.DurationDays)
	item.CreditsLeft = item.CreditsTotal - item.CreditsUsed
	if item.CreditsLeft < 0 {
		item.CreditsLeft = 0
	}
	return item, nil
}

func orderScanDest(item *Order) []any {
	return []any{
		&item.ID,
		&item.UserID,
		&item.UserEmail,
		&item.Username,
		&item.PackageID,
		&item.PackageType,
		&item.AmountCents,
		&item.Credits,
		&item.DurationDays,
		&item.Currency,
		&item.ProviderKey,
		&item.ProviderInstanceID,
		&item.OutTradeNo,
		&item.ProviderTradeNo,
		&item.Status,
		&item.PayURL,
		&item.QRCode,
		nullTimeScanner{target: &item.ExpiresAt},
		nullTimeScanner{target: &item.PaidAt},
		nullTimeScanner{target: &item.CompletedAt},
		nullTimeScanner{target: &item.FailedAt},
		nullTimeScanner{target: &item.RefundedAt},
		&item.CreditLedgerID,
		&item.CreatedAt,
		&item.UpdatedAt,
	}
}

func normalizeOrder(item *Order) {
	if item == nil {
		return
	}
	item.PackageType = normalizePackageType(item.PackageType)
	item.DurationDays = normalizeDurationDays(item.PackageType, item.DurationDays)
}

type nullTimeScanner struct {
	target *string
}

func (s nullTimeScanner) Scan(value any) error {
	var t sql.NullTime
	if err := t.Scan(value); err != nil {
		return err
	}
	*s.target = formatNullTime(t)
	return nil
}

func normalizePackageInput(input PackageInput) (Package, error) {
	name := cleanText(input.Name, 80)
	if name == "" {
		name = "充值套餐"
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "CNY"
	}
	if input.AmountCents <= 0 || input.Credits <= 0 {
		return Package{}, fmt.Errorf("amount and credits must be positive")
	}
	packageType := normalizePackageType(input.PackageType)
	return Package{
		PackageType:  packageType,
		Name:         name,
		Description:  cleanText(input.Description, 240),
		AmountCents:  input.AmountCents,
		Credits:      input.Credits,
		DurationDays: normalizeDurationDays(packageType, input.DurationDays),
		Currency:     currency,
		Enabled:      input.Enabled,
		SortOrder:    input.SortOrder,
	}, nil
}

func normalizePackageType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PackageTypeSubscription, PackageTypeMonthly:
		return PackageTypeSubscription
	default:
		return PackageTypeBalance
	}
}

func normalizeDurationDays(packageType string, value int) int {
	if normalizePackageType(packageType) != PackageTypeSubscription {
		return 0
	}
	if value <= 0 {
		return 30
	}
	if value > 3650 {
		return 3650
	}
	return value
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case StatusPending, StatusPaid, StatusCompleted, StatusExpired, StatusCancelled, StatusFailed, StatusRefunded:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeRateBPS(value int) int {
	if value < 0 {
		return 0
	}
	if value > 10000 {
		return 10000
	}
	return value
}

func parseStoredTime(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return parsed.UTC()
}

func formatNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339Nano)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func cleanText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return value
}

func cleanID(id string) string {
	return strings.ReplaceAll(strings.TrimSpace(id), "/", "-")
}

func newID(prefix string) string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return prefix + "_" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(raw[:])
}

func newOutTradeNo() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "PAY" + time.Now().UTC().Format("20060102150405")
	}
	return "PAY" + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(raw[:]))
}

func hashKey(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}
