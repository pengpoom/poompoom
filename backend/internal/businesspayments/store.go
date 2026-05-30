package businesspayments

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/businesscredits"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

const (
	ProviderManual  = "manual"
	ProviderEasyPay = "easypay"

	MethodManual = "manual"
	MethodAlipay = "alipay"
	MethodWxpay  = "wxpay"

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
	SubscriptionStatusUpgraded  = "upgraded"

	BillingActionNew     = "new"
	BillingActionRenewal = "renewal"
	BillingActionUpgrade = "upgrade"

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
	ErrPackageInvalid               = errors.New("payment package is invalid")
	ErrPackageDisabled              = errors.New("payment package is disabled")
	ErrPackageInUse                 = errors.New("payment package is used by orders")
	ErrOrderInvalid                 = errors.New("payment order is invalid")
	ErrOrderNotPayable              = errors.New("payment order is not payable")
	ErrOrderNotRefundable           = errors.New("payment order is not refundable")
	ErrSubscriptionChange           = errors.New("subscription change is not allowed")
	ErrSubscriptionDurationMismatch = errors.New("subscription duration mismatch")
	ErrProviderInvalid              = errors.New("payment provider is invalid")
	ErrProviderUnavailable          = errors.New("payment provider is unavailable")
	ErrProviderInUse                = errors.New("payment provider has pending orders")
)

type Package struct {
	ID           string `json:"id"`
	PackageType  string `json:"packageType"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	AmountCents  int64  `json:"amountCents"`
	Credits      int64  `json:"credits"`
	DurationDays int    `json:"durationDays"`
	LevelTag     string `json:"levelTag"`
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
	LevelTag     string
	Currency     string
	Enabled      bool
	SortOrder    int
}

type Provider struct {
	ID               string            `json:"id"`
	ProviderKey      string            `json:"providerKey"`
	Name             string            `json:"name"`
	Enabled          bool              `json:"enabled"`
	SupportedMethods []string          `json:"supportedMethods"`
	Config           map[string]string `json:"config,omitempty"`
	SortOrder        int               `json:"sortOrder"`
	CreatedAt        string            `json:"createdAt"`
	UpdatedAt        string            `json:"updatedAt"`
}

type ProviderInput struct {
	ProviderKey      string
	Name             string
	Enabled          bool
	SupportedMethods []string
	Config           map[string]string
	SortOrder        int
}

type PaymentMethod struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	ProviderKey string `json:"providerKey"`
}

type Order struct {
	ID                  string `json:"id"`
	UserID              string `json:"userId"`
	UserEmail           string `json:"userEmail"`
	Username            string `json:"username"`
	PackageID           string `json:"packageId"`
	PackageType         string `json:"packageType"`
	AmountCents         int64  `json:"amountCents"`
	Credits             int64  `json:"credits"`
	DurationDays        int    `json:"durationDays"`
	Currency            string `json:"currency"`
	PaymentMethod       string `json:"paymentMethod"`
	ProviderKey         string `json:"providerKey"`
	ProviderInstanceID  string `json:"providerInstanceId"`
	OutTradeNo          string `json:"outTradeNo"`
	ProviderTradeNo     string `json:"providerTradeNo"`
	Status              string `json:"status"`
	PayURL              string `json:"payUrl"`
	QRCode              string `json:"qrCode"`
	ExpiresAt           string `json:"expiresAt,omitempty"`
	PaidAt              string `json:"paidAt,omitempty"`
	CompletedAt         string `json:"completedAt,omitempty"`
	FailedAt            string `json:"failedAt,omitempty"`
	RefundedAt          string `json:"refundedAt,omitempty"`
	CreditLedgerID      string `json:"creditLedgerId,omitempty"`
	BillingAction       string `json:"billingAction,omitempty"`
	UpgradeFromSubID    string `json:"upgradeFromSubscriptionId,omitempty"`
	UpgradeCreditCents  int64  `json:"upgradeCreditCents,omitempty"`
	OriginalAmountCents int64  `json:"originalAmountCents,omitempty"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
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
	ID                string `json:"id"`
	UserID            string `json:"userId"`
	UserEmail         string `json:"userEmail,omitempty"`
	Username          string `json:"username,omitempty"`
	OrderID           string `json:"orderId"`
	PackageID         string `json:"packageId"`
	PackageName       string `json:"packageName"`
	DurationDays      int    `json:"durationDays"`
	CreditsTotal      int64  `json:"creditsTotal"`
	CreditsUsed       int64  `json:"creditsUsed"`
	CreditsLeft       int64  `json:"creditsLeft"`
	Status            string `json:"status"`
	Active            bool   `json:"active"`
	StartsAt          string `json:"startsAt"`
	ExpiresAt         string `json:"expiresAt"`
	CoverageExpiresAt string `json:"coverageExpiresAt,omitempty"`
	CancelledAt       string `json:"cancelledAt,omitempty"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type SubscriptionTotals struct {
	Reserved int64
	Refunded int64
}

type subscriptionRun struct {
	StartsAt     time.Time
	ExpiresAt    time.Time
	DurationDays int
	AmountCents  int64
	CreditsTotal int64
	CreditsLeft  int64
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
	Kind   string
	Search string
}

type SubscriptionFilters struct {
	UserID       string
	Status       string
	Search       string
	ActiveWindow string
}

type CreateOrderInput struct {
	UserID        string
	Username      string
	UserEmail     string
	PackageID     string
	PaymentMethod string
	NotifyBaseURL string
	ReturnURL     string
	ClientIP      string
	UserAgent     string
}

type CompleteOrderInput struct {
	OrderID           string
	Operator          string
	ProviderTradeNo   string
	CommissionRateBPS int
}

type PaymentProvider interface {
	Key() string
	CreatePayment(context.Context, ProviderPaymentRequest) (ProviderPayment, error)
}

type ProviderPaymentRequest struct {
	Order     Order
	Method    string
	Subject   string
	NotifyURL string
	ReturnURL string
	ClientIP  string
	UserAgent string
}

type ProviderPayment struct {
	ProviderTradeNo string
	PayURL          string
	QRCode          string
}

type ManualProvider struct{}

func (ManualProvider) Key() string { return ProviderManual }

func (ManualProvider) CreatePayment(context.Context, ProviderPaymentRequest) (ProviderPayment, error) {
	return ProviderPayment{}, nil
}

type EasyPayProvider struct {
	config map[string]string
	client *http.Client
}

const (
	easyPaySuccessCode = 1
	easyPayPaidStatus  = 1
	easyPaySignType    = "MD5"
)

func NewEasyPayProvider(configMap map[string]string) (*EasyPayProvider, error) {
	configMap = normalizeProviderConfig(ProviderEasyPay, configMap)
	if err := validateProviderConfig(ProviderEasyPay, configMap); err != nil {
		return nil, err
	}
	return &EasyPayProvider{
		config: configMap,
		client: &http.Client{Timeout: 12 * time.Second},
	}, nil
}

func (p *EasyPayProvider) Key() string { return ProviderEasyPay }

func (p *EasyPayProvider) CreatePayment(ctx context.Context, req ProviderPaymentRequest) (ProviderPayment, error) {
	method := normalizePaymentMethod(req.Method)
	if method != MethodAlipay && method != MethodWxpay {
		return ProviderPayment{}, ErrProviderInvalid
	}
	amount := fmt.Sprintf("%.2f", float64(req.Order.AmountCents)/100)
	params := map[string]string{
		"pid":          p.config["pid"],
		"type":         method,
		"out_trade_no": req.Order.OutTradeNo,
		"notify_url":   req.NotifyURL,
		"return_url":   req.ReturnURL,
		"name":         req.Subject,
		"money":        amount,
		"clientip":     req.ClientIP,
	}
	if cid := p.channelID(method); cid != "" {
		params["cid"] = cid
	}
	params["sign"] = easyPaySign(params, p.config["pkey"])
	params["sign_type"] = easyPaySignType

	if strings.EqualFold(p.config["paymentMode"], "popup") {
		q := url.Values{}
		for key, value := range params {
			q.Set(key, value)
		}
		return ProviderPayment{PayURL: p.apiBase() + "/submit.php?" + q.Encode()}, nil
	}

	body, err := postEasyPayForm(ctx, p.client, p.apiBase()+"/mapi.php", params)
	if err != nil {
		return ProviderPayment{}, err
	}
	var payload struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		TradeNo string `json:"trade_no"`
		PayURL  string `json:"payurl"`
		PayURL2 string `json:"payurl2"`
		QRCode  string `json:"qrcode"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ProviderPayment{}, fmt.Errorf("解析易支付响应失败")
	}
	if payload.Code != easyPaySuccessCode {
		return ProviderPayment{}, errors.New(firstNonEmpty(payload.Msg, "易支付创建订单失败"))
	}
	return ProviderPayment{ProviderTradeNo: payload.TradeNo, PayURL: firstNonEmpty(payload.PayURL, payload.PayURL2), QRCode: payload.QRCode}, nil
}

func (p *EasyPayProvider) VerifyNotification(rawBody string) (EasyPayNotification, error) {
	values, err := url.ParseQuery(rawBody)
	if err != nil {
		return EasyPayNotification{}, err
	}
	params := make(map[string]string, len(values))
	for key := range values {
		params[key] = values.Get(key)
	}
	sign := strings.TrimSpace(params["sign"])
	if sign == "" || !hmac.Equal([]byte(easyPaySign(params, p.config["pkey"])), []byte(sign)) {
		return EasyPayNotification{}, fmt.Errorf("invalid signature")
	}
	amountCents := moneyToCents(params["money"])
	status := StatusFailed
	if strings.EqualFold(params["trade_status"], "TRADE_SUCCESS") {
		status = StatusPaid
	}
	return EasyPayNotification{
		OutTradeNo:      strings.TrimSpace(params["out_trade_no"]),
		ProviderTradeNo: strings.TrimSpace(params["trade_no"]),
		Status:          status,
		AmountCents:     amountCents,
		Raw:             rawBody,
	}, nil
}

type EasyPayNotification struct {
	OutTradeNo      string
	ProviderTradeNo string
	Status          string
	AmountCents     int64
	Raw             string
}

func (p *EasyPayProvider) apiBase() string {
	return normalizeEasyPayAPIBase(p.config["apiBase"])
}

func (p *EasyPayProvider) channelID(method string) string {
	if method == MethodAlipay {
		return firstNonEmpty(p.config["cidAlipay"], p.config["cid"])
	}
	return firstNonEmpty(p.config["cidWxpay"], p.config["cid"])
}

func postEasyPayForm(ctx context.Context, client *http.Client, endpoint string, params map[string]string) ([]byte, error) {
	form := url.Values{}
	for key, value := range params {
		form.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("易支付 HTTP %d", resp.StatusCode)
	}
	return body, nil
}

type Store struct {
	db           *sql.DB
	driver       string
	configSecret string
	ownDB        bool
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
	store := NewStoreWithDBWithSecret(db, cfg.Database.Driver, paymentConfigSecret(cfg))
	store.ownDB = true
	return store, nil
}

func NewStoreWithDB(db *sql.DB, driver string) *Store {
	return NewStoreWithDBWithSecret(db, driver, "")
}

func NewStoreWithDBWithSecret(db *sql.DB, driver string, secret string) *Store {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		driver = "postgres"
	}
	return &Store{db: db, driver: driver, configSecret: strings.TrimSpace(secret)}
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
		SupportedMethods: []string{MethodManual},
		SortOrder:        0,
		CreatedAt:        "",
		UpdatedAt:        "",
	}}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id, provider_key, name, enabled, supported_methods, config_ciphertext, sort_order, created_at, updated_at
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
		var configCipher []byte
		if err := rows.Scan(&item.ID, &item.ProviderKey, &item.Name, &enabled, &methodsRaw, &configCipher, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.SupportedMethods = splitCSV(methodsRaw)
		item.Config = maskProviderConfig(item.ProviderKey, s.decryptProviderConfig(configCipher))
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateProvider(ctx context.Context, input ProviderInput) (Provider, error) {
	item, configMap, err := s.normalizeProviderInput(input)
	if err != nil {
		return Provider{}, err
	}
	configCipher, err := s.encryptProviderConfig(configMap)
	if err != nil {
		return Provider{}, err
	}
	now := time.Now().UTC()
	item.ID = newID("pay_provider")
	item.CreatedAt = now.Format(time.RFC3339Nano)
	item.UpdatedAt = item.CreatedAt
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_payment_providers(
			id, provider_key, name, enabled, supported_methods, config_ciphertext, sort_order, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		item.ID,
		item.ProviderKey,
		item.Name,
		boolInt(item.Enabled),
		strings.Join(item.SupportedMethods, ","),
		configCipher,
		item.SortOrder,
		now,
		now,
	)
	if err != nil {
		return Provider{}, err
	}
	item.Config = maskProviderConfig(item.ProviderKey, configMap)
	return item, nil
}

func (s *Store) UpdateProvider(ctx context.Context, id string, input ProviderInput) (Provider, bool, error) {
	id = cleanID(id)
	if id == "" || id == ProviderManual {
		return Provider{}, false, nil
	}
	current, currentConfig, ok, err := s.getProviderForRuntime(ctx, id, false)
	if err != nil || !ok {
		return Provider{}, ok, err
	}
	nextInput := input
	nextInput.ProviderKey = current.ProviderKey
	normalized, configMap, err := s.normalizeProviderInput(nextInput)
	if err != nil {
		return Provider{}, false, err
	}
	configMap = mergeProviderConfig(current.ProviderKey, currentConfig, configMap)
	configCipher, err := s.encryptProviderConfig(configMap)
	if err != nil {
		return Provider{}, false, err
	}
	if !normalized.Enabled && s.providerHasActiveOrders(ctx, id) {
		return Provider{}, false, ErrProviderInUse
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_payment_providers
		SET name = ?, enabled = ?, supported_methods = ?, config_ciphertext = ?, sort_order = ?, updated_at = ?
		WHERE id = ?`),
		normalized.Name,
		boolInt(normalized.Enabled),
		strings.Join(normalized.SupportedMethods, ","),
		configCipher,
		normalized.SortOrder,
		now,
		id,
	)
	if err != nil {
		return Provider{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Provider{}, false, err
	}
	if affected == 0 {
		return Provider{}, false, nil
	}
	updated := normalized
	updated.ID = id
	updated.CreatedAt = current.CreatedAt
	updated.UpdatedAt = now.Format(time.RFC3339Nano)
	updated.Config = maskProviderConfig(updated.ProviderKey, configMap)
	return updated, true, nil
}

func (s *Store) DeleteProvider(ctx context.Context, id string) (bool, error) {
	id = cleanID(id)
	if id == "" || id == ProviderManual {
		return false, nil
	}
	if s.providerHasActiveOrders(ctx, id) {
		return false, ErrProviderInUse
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_payment_providers WHERE id = ?`), id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) AvailablePaymentMethods(ctx context.Context) ([]PaymentMethod, error) {
	providers, err := s.enabledProviderRuntimes(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{MethodManual: true}
	items := []PaymentMethod{{
		Key:         MethodManual,
		Label:       paymentMethodLabel(MethodManual),
		ProviderKey: ProviderManual,
	}}
	for _, provider := range providers {
		for _, method := range provider.SupportedMethods {
			method = normalizePaymentMethod(method)
			if method == "" || seen[method] {
				continue
			}
			seen[method] = true
			items = append(items, PaymentMethod{
				Key:         method,
				Label:       paymentMethodLabel(method),
				ProviderKey: provider.ProviderKey,
			})
		}
	}
	return items, nil
}

func (s *Store) ListPackages(ctx context.Context, includeDisabled bool) ([]Package, error) {
	query := `SELECT id, package_type, name, description, amount_cents, credits, duration_days, level_tag, currency, enabled, sort_order, created_at, updated_at
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
			id, package_type, name, description, amount_cents, credits, duration_days, level_tag, currency, enabled, sort_order, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		item.ID,
		item.PackageType,
		item.Name,
		item.Description,
		item.AmountCents,
		item.Credits,
		item.DurationDays,
		item.LevelTag,
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
		SET package_type = ?, name = ?, description = ?, amount_cents = ?, credits = ?, duration_days = ?, level_tag = ?, currency = ?, enabled = ?, sort_order = ?, updated_at = ?
		WHERE id = ?`),
		item.PackageType,
		item.Name,
		item.Description,
		item.AmountCents,
		item.Credits,
		item.DurationDays,
		item.LevelTag,
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
	query := `SELECT id, package_type, name, description, amount_cents, credits, duration_days, level_tag, currency, enabled, sort_order, created_at, updated_at
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
		&item.LevelTag,
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
	item.LevelTag = normalizeLevelTag(item.LevelTag)
	item.Enabled = enabled != 0
	return item, true, nil
}

func (s *Store) UserPaidPackageLevelTags(ctx context.Context, userID string, packageType string) ([]string, error) {
	userID = strings.TrimSpace(userID)
	packageType = normalizePackageType(packageType)
	if userID == "" || packageType == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT DISTINCT p.level_tag
		FROM business_payment_orders o
		JOIN business_payment_packages p ON p.id = o.package_id
		WHERE o.user_id = ?
		  AND o.package_type = ?
		  AND o.status = ?
		  AND p.level_tag <> ''`), userID, packageType, StatusCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	seen := map[string]struct{}{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tag = normalizeLevelTag(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		items = append(items, tag)
	}
	return items, rows.Err()
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

type providerRuntime struct {
	Provider
	Config map[string]string
}

func (s *Store) normalizeProviderInput(input ProviderInput) (Provider, map[string]string, error) {
	providerKey := normalizeProviderKey(input.ProviderKey)
	if providerKey == "" {
		return Provider{}, nil, ErrProviderInvalid
	}
	name := cleanText(input.Name, 80)
	if name == "" {
		name = providerName(providerKey)
	}
	methods := normalizePaymentMethods(input.SupportedMethods, providerKey)
	if len(methods) == 0 {
		return Provider{}, nil, ErrProviderInvalid
	}
	configMap := normalizeProviderConfig(providerKey, input.Config)
	if input.Enabled {
		if err := validateProviderConfig(providerKey, configMap); err != nil {
			return Provider{}, nil, err
		}
	}
	return Provider{
		ProviderKey:      providerKey,
		Name:             name,
		Enabled:          input.Enabled,
		SupportedMethods: methods,
		SortOrder:        input.SortOrder,
	}, configMap, nil
}

func (s *Store) selectProvider(ctx context.Context, method string) (providerRuntime, map[string]string, error) {
	method = normalizePaymentMethod(method)
	if method == "" || method == MethodManual {
		return providerRuntime{
			Provider: Provider{
				ID:               ProviderManual,
				ProviderKey:      ProviderManual,
				Name:             "人工确认",
				Enabled:          true,
				SupportedMethods: []string{MethodManual},
			},
		}, nil, nil
	}
	providers, err := s.enabledProviderRuntimes(ctx)
	if err != nil {
		return providerRuntime{}, nil, err
	}
	for _, provider := range providers {
		if stringSliceContains(provider.SupportedMethods, method) {
			return provider, provider.Config, nil
		}
	}
	return providerRuntime{}, nil, ErrProviderUnavailable
}

func (s *Store) enabledProviderRuntimes(ctx context.Context) ([]providerRuntime, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id, provider_key, name, enabled, supported_methods, config_ciphertext, sort_order, created_at, updated_at
		FROM business_payment_providers
		WHERE enabled = ?
		ORDER BY sort_order ASC, created_at DESC`), 1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []providerRuntime{}
	for rows.Next() {
		provider, configMap, err := scanProviderRuntime(rows, s)
		if err != nil {
			return nil, err
		}
		items = append(items, providerRuntime{Provider: provider, Config: configMap})
	}
	return items, rows.Err()
}

func (s *Store) getProviderForRuntime(ctx context.Context, id string, enabledOnly bool) (Provider, map[string]string, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Provider{}, nil, false, nil
	}
	query := `SELECT id, provider_key, name, enabled, supported_methods, config_ciphertext, sort_order, created_at, updated_at
		FROM business_payment_providers
		WHERE id = ?`
	args := []any{id}
	if enabledOnly {
		query += ` AND enabled = ?`
		args = append(args, 1)
	}
	row := s.db.QueryRowContext(ctx, s.rebind(query), args...)
	provider, configMap, err := scanProviderRuntime(row, s)
	if err == sql.ErrNoRows {
		return Provider{}, nil, false, nil
	}
	if err != nil {
		return Provider{}, nil, false, err
	}
	return provider, configMap, true, nil
}

func scanProviderRuntime(row interface{ Scan(dest ...any) error }, store *Store) (Provider, map[string]string, error) {
	var item Provider
	var enabled int
	var methodsRaw string
	var configCipher []byte
	if err := row.Scan(&item.ID, &item.ProviderKey, &item.Name, &enabled, &methodsRaw, &configCipher, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Provider{}, nil, err
	}
	item.Enabled = enabled != 0
	item.ProviderKey = normalizeProviderKey(item.ProviderKey)
	item.SupportedMethods = normalizePaymentMethods(splitCSV(methodsRaw), item.ProviderKey)
	configMap := store.decryptProviderConfig(configCipher)
	item.Config = maskProviderConfig(item.ProviderKey, configMap)
	return item, configMap, nil
}

func (s *Store) providerHasActiveOrders(ctx context.Context, providerID string) bool {
	var count int
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM business_payment_orders
		WHERE provider_instance_id = ? AND status IN (?, ?)`), providerID, StatusPending, StatusPaid).Scan(&count)
	return err == nil && count > 0
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
	paymentMethod := normalizePaymentMethod(input.PaymentMethod)
	provider, providerConfig, err := s.selectProvider(ctx, paymentMethod)
	if err != nil {
		return Order{}, err
	}
	providerInstanceID := provider.ID
	if providerInstanceID == "" {
		providerInstanceID = ProviderManual
	}
	providerKey := provider.ProviderKey
	if providerKey == "" {
		providerKey = ProviderManual
	}
	if paymentMethod == "" {
		paymentMethod = firstPaymentMethod(provider.SupportedMethods)
	}
	order := Order{
		ID:                  newID("pay_order"),
		UserID:              userID,
		UserEmail:           cleanText(input.UserEmail, 240),
		Username:            cleanText(input.Username, 120),
		PackageID:           pkg.ID,
		PackageType:         pkg.PackageType,
		AmountCents:         pkg.AmountCents,
		Credits:             pkg.Credits,
		DurationDays:        pkg.DurationDays,
		Currency:            pkg.Currency,
		PaymentMethod:       paymentMethod,
		ProviderKey:         providerKey,
		ProviderInstanceID:  providerInstanceID,
		OutTradeNo:          newOutTradeNo(),
		Status:              StatusPending,
		ExpiresAt:           now.Add(30 * time.Minute).Format(time.RFC3339Nano),
		OriginalAmountCents: pkg.AmountCents,
		CreatedAt:           now.Format(time.RFC3339Nano),
		UpdatedAt:           now.Format(time.RFC3339Nano),
	}
	packageSnapshot, _ := json.Marshal(pkg)
	providerSnapshot, _ := json.Marshal(providerSnapshot(provider, providerConfig, paymentMethod))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	if order.PackageType == PackageTypeSubscription {
		action, fromSubID, creditCents, amountCents, originalAmountCents, credits, err := s.subscriptionOrderPricingWithTx(ctx, tx, pkg, userID, now)
		if err != nil {
			return Order{}, err
		}
		order.BillingAction = action
		order.UpgradeFromSubID = fromSubID
		order.UpgradeCreditCents = creditCents
		order.AmountCents = amountCents
		order.OriginalAmountCents = originalAmountCents
		order.Credits = credits
	}
	if order.BillingAction == "" {
		order.BillingAction = BillingActionNew
	}
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO business_payment_orders(
			id, user_id, user_email, username, package_id, package_type, package_snapshot_json,
			amount_cents, credits, duration_days, currency, payment_method, provider_key, provider_instance_id,
			provider_snapshot_json, out_trade_no, status, expires_at, credit_ledger_id,
			billing_action, upgrade_from_subscription_id, upgrade_credit_cents, original_amount_cents,
			created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
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
		order.PaymentMethod,
		order.ProviderKey,
		order.ProviderInstanceID,
		providerSnapshot,
		order.OutTradeNo,
		order.Status,
		parseStoredTime(order.ExpiresAt),
		order.CreditLedgerID,
		order.BillingAction,
		order.UpgradeFromSubID,
		order.UpgradeCreditCents,
		order.OriginalAmountCents,
		now,
		now,
	)
	if err != nil {
		return Order{}, err
	}
	if err := s.writeAuditWithTx(ctx, tx, order.ID, "order_created", map[string]any{
		"packageId":           pkg.ID,
		"billingAction":       order.BillingAction,
		"amountCents":         order.AmountCents,
		"originalAmountCents": order.OriginalAmountCents,
		"upgradeCreditCents":  order.UpgradeCreditCents,
		"paymentMethod":       order.PaymentMethod,
		"providerKey":         order.ProviderKey,
		"providerInstanceId":  order.ProviderInstanceID,
	}, userID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	if provider.ProviderKey != ProviderManual {
		payment, err := s.createProviderPayment(ctx, provider, providerConfig, order, input)
		if err != nil {
			_ = s.MarkOrderFailed(context.Background(), order.ID, err.Error())
			return Order{}, err
		}
		updated, err := s.UpdateOrderPayment(ctx, order.ID, payment)
		if err != nil {
			return Order{}, err
		}
		order = updated
	}
	normalizeOrder(&order)
	return order, nil
}

func (s *Store) ListOrders(ctx context.Context, filters OrderFilters, limit int, offset int) ([]Order, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
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
	switch normalizeOrderKind(filters.Kind) {
	case "balance":
		where = append(where, "package_type = ?")
		args = append(args, PackageTypeBalance)
	case "subscription":
		where = append(where, "package_type = ? AND billing_action NOT IN (?, ?)")
		args = append(args, PackageTypeSubscription, BillingActionRenewal, BillingActionUpgrade)
	case BillingActionRenewal:
		where = append(where, "billing_action = ?")
		args = append(args, BillingActionRenewal)
	case BillingActionUpgrade:
		where = append(where, "billing_action = ?")
		args = append(args, BillingActionUpgrade)
	}
	if search := strings.ToLower(strings.TrimSpace(filters.Search)); search != "" {
		where = append(where, `(LOWER(id) LIKE ? OR LOWER(out_trade_no) LIKE ? OR LOWER(COALESCE(provider_trade_no, '')) LIKE ? OR LOWER(COALESCE(user_email, '')) LIKE ? OR LOWER(COALESCE(username, '')) LIKE ? OR LOWER(user_id) LIKE ? OR LOWER(COALESCE(payment_method, '')) LIKE ?)`)
		like := "%" + search + "%"
		args = append(args, like, like, like, like, like, like, like)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*)
		FROM business_payment_orders
		WHERE `+whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT
			id, user_id, user_email, username, package_id, package_type, amount_cents, credits, duration_days, currency,
			payment_method, provider_key, provider_instance_id, out_trade_no, provider_trade_no, status,
			pay_url, qr_code, expires_at, paid_at, completed_at, failed_at, refunded_at,
			credit_ledger_id, billing_action, upgrade_from_subscription_id, upgrade_credit_cents, original_amount_cents,
			created_at, updated_at
		FROM business_payment_orders
		WHERE `+whereSQL+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?`), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Order{}
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) GetOrder(ctx context.Context, id string, userID string) (Order, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Order{}, false, nil
	}
	query := `SELECT
			id, user_id, user_email, username, package_id, package_type, amount_cents, credits, duration_days, currency,
			payment_method, provider_key, provider_instance_id, out_trade_no, provider_trade_no, status,
			pay_url, qr_code, expires_at, paid_at, completed_at, failed_at, refunded_at,
			credit_ledger_id, billing_action, upgrade_from_subscription_id, upgrade_credit_cents, original_amount_cents,
			created_at, updated_at
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

func (s *Store) subscriptionOrderPricingWithTx(ctx context.Context, tx *sql.Tx, pkg Package, userID string, now time.Time) (string, string, int64, int64, int64, int64, error) {
	current, err := s.currentUsableSubscriptionWithTx(ctx, tx, userID, now)
	if err != nil {
		return "", "", 0, 0, 0, 0, err
	}
	if current.ID == "" {
		return BillingActionNew, "", 0, pkg.AmountCents, pkg.AmountCents, pkg.Credits, nil
	}
	if current.PackageID == pkg.ID {
		return BillingActionRenewal, "", 0, pkg.AmountCents, pkg.AmountCents, pkg.Credits, nil
	}
	if normalizeDurationDays(PackageTypeSubscription, current.DurationDays) != normalizeDurationDays(PackageTypeSubscription, pkg.DurationDays) {
		return "", "", 0, 0, 0, 0, ErrSubscriptionDurationMismatch
	}
	oldAmountCents, err := s.subscriptionOriginalAmountWithTx(ctx, tx, current)
	if err != nil {
		return "", "", 0, 0, 0, 0, err
	}
	if oldAmountCents <= 0 {
		oldAmountCents = s.subscriptionCurrentPackageAmountWithTx(ctx, tx, current.PackageID)
	}
	oldPeriodAmountCents := subscriptionPeriodAmountCents(current, oldAmountCents)
	if oldPeriodAmountCents <= 0 {
		oldPeriodAmountCents = oldAmountCents
	}
	if pkg.AmountCents <= oldPeriodAmountCents {
		return "", "", 0, 0, 0, 0, ErrSubscriptionChange
	}
	activeRuns, err := s.activeSubscriptionRunsWithTx(ctx, tx, userID, current.PackageID, now)
	if err != nil {
		return "", "", 0, 0, 0, 0, err
	}
	targetDurationDays := normalizeDurationDays(PackageTypeSubscription, pkg.DurationDays)
	for _, run := range activeRuns {
		if normalizeDurationDays(PackageTypeSubscription, run.DurationDays) != targetDurationDays {
			return "", "", 0, 0, 0, 0, ErrSubscriptionDurationMismatch
		}
	}
	remainingAmountCents, remainingCredits := prorateSubscriptionUpgradeTarget(activeRuns, pkg.AmountCents, pkg.Credits, now)
	if remainingAmountCents <= 0 {
		remainingAmountCents = pkg.AmountCents
	}
	if remainingCredits <= 0 {
		remainingCredits = pkg.Credits
	}
	creditCents := subscriptionUpgradeCreditCents(activeRuns, oldAmountCents, now)
	if creditCents > remainingAmountCents {
		creditCents = remainingAmountCents
	}
	return BillingActionUpgrade, current.ID, creditCents, remainingAmountCents - creditCents, remainingAmountCents, remainingCredits, nil
}

func (s *Store) subscriptionOriginalAmountWithTx(ctx context.Context, tx *sql.Tx, subscription Subscription) (int64, error) {
	if subscription.OrderID == "" {
		return 0, nil
	}
	var amountCents int64
	var originalAmountCents int64
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT amount_cents, original_amount_cents
		FROM business_payment_orders
		WHERE id = ?`), subscription.OrderID).Scan(&amountCents, &originalAmountCents)
	if err == sql.ErrNoRows {
		return s.subscriptionCurrentPackageAmountWithTx(ctx, tx, subscription.PackageID), nil
	}
	if err != nil {
		return 0, err
	}
	if originalAmountCents > 0 {
		return originalAmountCents, nil
	}
	if amountCents > 0 {
		return amountCents, nil
	}
	return s.subscriptionCurrentPackageAmountWithTx(ctx, tx, subscription.PackageID), nil
}

func (s *Store) subscriptionCurrentPackageAmountWithTx(ctx context.Context, tx *sql.Tx, packageID string) int64 {
	if packageID == "" {
		return 0
	}
	var amountCents int64
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT amount_cents FROM business_payment_packages WHERE id = ?`), packageID).Scan(&amountCents); err != nil {
		return 0
	}
	return amountCents
}

func subscriptionUpgradeCreditCents(runs []subscriptionRun, fallbackAmountCents int64, now time.Time) int64 {
	total := int64(0)
	for _, run := range runs {
		if !run.ExpiresAt.After(now) || !run.ExpiresAt.After(run.StartsAt) {
			continue
		}
		effectiveStart := run.StartsAt
		if effectiveStart.Before(now) {
			effectiveStart = now
		}
		totalSeconds := int64(run.ExpiresAt.Sub(run.StartsAt).Seconds())
		remainingSeconds := int64(run.ExpiresAt.Sub(effectiveStart).Seconds())
		if totalSeconds <= 0 || remainingSeconds <= 0 {
			continue
		}
		runAmountCents := run.AmountCents
		if runAmountCents <= 0 {
			runAmountCents = fallbackAmountCents
		}
		if runAmountCents <= 0 {
			continue
		}
		timeValue := runAmountCents * remainingSeconds / totalSeconds
		creditValue := int64(0)
		if run.CreditsTotal > 0 {
			creditValue = runAmountCents * run.CreditsLeft / run.CreditsTotal
		}
		total += clampCents(minInt64(timeValue, creditValue), runAmountCents)
	}
	return total
}

func subscriptionPeriodAmountCents(subscription Subscription, amountCents int64) int64 {
	if amountCents <= 0 {
		return 0
	}
	startsAt, startErr := time.Parse(time.RFC3339Nano, subscription.StartsAt)
	expiresAt, expireErr := time.Parse(time.RFC3339Nano, subscription.ExpiresAt)
	durationSeconds := int64(normalizeDurationDays(PackageTypeSubscription, subscription.DurationDays)) * 24 * 60 * 60
	if startErr != nil || expireErr != nil || durationSeconds <= 0 || !expiresAt.After(startsAt) {
		return amountCents
	}
	totalSeconds := int64(expiresAt.Sub(startsAt).Seconds())
	if totalSeconds <= 0 || totalSeconds == durationSeconds {
		return amountCents
	}
	return amountCents * durationSeconds / totalSeconds
}

func prorateSubscriptionUpgradeTarget(runs []subscriptionRun, newAmountCents int64, newCredits int64, now time.Time) (int64, int64) {
	totalAmount := int64(0)
	totalCredits := int64(0)
	for _, run := range runs {
		if !run.ExpiresAt.After(now) || !run.ExpiresAt.After(run.StartsAt) {
			continue
		}
		effectiveStart := run.StartsAt
		if effectiveStart.Before(now) {
			effectiveStart = now
		}
		totalSeconds := int64(run.ExpiresAt.Sub(run.StartsAt).Seconds())
		remainingSeconds := int64(run.ExpiresAt.Sub(effectiveStart).Seconds())
		if totalSeconds <= 0 || remainingSeconds <= 0 {
			continue
		}
		periodSeconds := int64(normalizeDurationDays(PackageTypeSubscription, run.DurationDays)) * 24 * 60 * 60
		if periodSeconds <= 0 {
			periodSeconds = totalSeconds
		}
		totalAmount += newAmountCents * remainingSeconds / periodSeconds
		totalCredits += newCredits * remainingSeconds / periodSeconds
	}
	return totalAmount, totalCredits
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
			"subscriptionId":     subscription.ID,
			"startsAt":           subscription.StartsAt,
			"expiresAt":          subscription.ExpiresAt,
			"durationDays":       subscription.DurationDays,
			"creditsTotal":       subscription.CreditsTotal,
			"billingAction":      order.BillingAction,
			"upgradeFromSubId":   order.UpgradeFromSubID,
			"upgradeCreditCents": order.UpgradeCreditCents,
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

func (s *Store) CompleteOrderFromProvider(ctx context.Context, outTradeNo string, providerTradeNo string, amountCents int64, operator string) (OrderWithCommission, error) {
	outTradeNo = strings.TrimSpace(outTradeNo)
	if outTradeNo == "" {
		return OrderWithCommission{}, ErrOrderInvalid
	}
	var orderID string
	var orderAmountCents int64
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT id, amount_cents FROM business_payment_orders WHERE out_trade_no = ?`), outTradeNo).Scan(&orderID, &orderAmountCents)
	if err == sql.ErrNoRows {
		return OrderWithCommission{}, ErrOrderInvalid
	}
	if err != nil {
		return OrderWithCommission{}, err
	}
	if amountCents > 0 && orderAmountCents > 0 && amountCents != orderAmountCents {
		return OrderWithCommission{}, fmt.Errorf("payment amount mismatch")
	}
	return s.CompleteOrder(ctx, CompleteOrderInput{
		OrderID:         orderID,
		Operator:        firstNonEmpty(operator, "payment_callback"),
		ProviderTradeNo: providerTradeNo,
	})
}

func (s *Store) UpdateOrderPayment(ctx context.Context, id string, payment ProviderPayment) (Order, error) {
	id = cleanID(id)
	if id == "" {
		return Order{}, ErrOrderInvalid
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_payment_orders
		SET provider_trade_no = COALESCE(NULLIF(?, ''), provider_trade_no),
			pay_url = ?, qr_code = ?, updated_at = ?
		WHERE id = ? AND status = ?`),
		cleanText(payment.ProviderTradeNo, 120),
		strings.TrimSpace(payment.PayURL),
		strings.TrimSpace(payment.QRCode),
		now,
		id,
		StatusPending,
	)
	if err != nil {
		return Order{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Order{}, err
	}
	if affected == 0 {
		return Order{}, ErrOrderInvalid
	}
	order, ok, err := s.GetOrder(ctx, id, "")
	if err != nil {
		return Order{}, err
	}
	if !ok {
		return Order{}, ErrOrderInvalid
	}
	return order, nil
}

func (s *Store) MarkOrderFailed(ctx context.Context, id string, reason string) error {
	id = cleanID(id)
	if id == "" {
		return ErrOrderInvalid
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	order, err := s.orderForUpdate(ctx, tx, id)
	if err != nil {
		return err
	}
	if order.Status != StatusPending {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_payment_orders
		SET status = ?, failed_at = ?, updated_at = ?
		WHERE id = ?`), StatusFailed, now, now, id); err != nil {
		return err
	}
	if err := s.writeAuditWithTx(ctx, tx, id, "payment_create_failed", map[string]any{"reason": cleanText(reason, 300)}, "system"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) HandleEasyPayNotification(ctx context.Context, rawBody string) (OrderWithCommission, error) {
	values, err := url.ParseQuery(strings.TrimSpace(rawBody))
	if err != nil {
		return OrderWithCommission{}, err
	}
	outTradeNo := strings.TrimSpace(values.Get("out_trade_no"))
	if outTradeNo == "" {
		return OrderWithCommission{}, ErrOrderInvalid
	}
	var providerInstanceID string
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT provider_instance_id
		FROM business_payment_orders
		WHERE out_trade_no = ?`), outTradeNo).Scan(&providerInstanceID)
	if err == sql.ErrNoRows {
		return OrderWithCommission{}, ErrOrderInvalid
	}
	if err != nil {
		return OrderWithCommission{}, err
	}
	provider, configMap, ok, err := s.getProviderForRuntime(ctx, providerInstanceID, false)
	if err != nil {
		return OrderWithCommission{}, err
	}
	if !ok || provider.ProviderKey != ProviderEasyPay {
		return OrderWithCommission{}, ErrProviderInvalid
	}
	runtime, err := NewEasyPayProvider(configMap)
	if err != nil {
		return OrderWithCommission{}, err
	}
	notice, err := runtime.VerifyNotification(rawBody)
	if err != nil {
		return OrderWithCommission{}, err
	}
	if notice.Status != StatusPaid {
		return OrderWithCommission{}, ErrOrderNotPayable
	}
	return s.CompleteOrderFromProvider(ctx, notice.OutTradeNo, notice.ProviderTradeNo, notice.AmountCents, "easypay_callback")
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
		WHERE user_id = ? AND status = ? AND starts_at <= ? AND expires_at > ?
		ORDER BY expires_at DESC, created_at DESC
		LIMIT 1`), userID, SubscriptionStatusActive, now, now)
	item, err := scanSubscription(row)
	if err == nil {
		if err := s.populateActiveSubscriptionTotals(ctx, userID, now, &item); err != nil {
			return Subscription{}, err
		}
		updateSubscriptionActive(&item, now)
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
	updateSubscriptionActive(&item, now)
	return item, nil
}

func (s *Store) ListSubscriptions(ctx context.Context, filters SubscriptionFilters, limit int, offset int) ([]Subscription, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	now := time.Now().UTC()
	if err := s.expireAllSubscriptions(ctx, now); err != nil {
		return nil, 0, err
	}
	where := []string{"1 = 1"}
	args := []any{}
	if userID := strings.TrimSpace(filters.UserID); userID != "" {
		where = append(where, "s.user_id = ?")
		args = append(args, userID)
	}
	if status := normalizeSubscriptionStatus(filters.Status); status != "" {
		where = append(where, "s.status = ?")
		args = append(args, status)
	}
	switch strings.ToLower(strings.TrimSpace(filters.ActiveWindow)) {
	case "current":
		where = append(where, "s.status = ? AND s.starts_at <= ? AND s.expires_at > ?")
		args = append(args, SubscriptionStatusActive, now, now)
	case "future":
		where = append(where, "s.status = ? AND s.starts_at > ? AND s.expires_at > ?")
		args = append(args, SubscriptionStatusActive, now, now)
	case "history":
		where = append(where, "NOT (s.status = ? AND s.expires_at > ?)")
		args = append(args, SubscriptionStatusActive, now)
	}
	if search := strings.ToLower(strings.TrimSpace(filters.Search)); search != "" {
		where = append(where, `(LOWER(s.package_name) LIKE ? OR LOWER(s.order_id) LIKE ? OR LOWER(COALESCE(u.email, '')) LIKE ? OR LOWER(COALESCE(u.username, '')) LIKE ?)`)
		like := "%" + search + "%"
		args = append(args, like, like, like, like)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*)
		FROM business_user_subscriptions s
		LEFT JOIN business_users u ON u.id = s.user_id
		WHERE `+whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT
			s.id, s.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
			s.order_id, s.package_id, s.package_name, s.duration_days, s.credits_total, s.credits_used,
			s.status, s.starts_at, s.expires_at, s.cancelled_at, s.created_at, s.updated_at
		FROM business_user_subscriptions s
		LEFT JOIN business_users u ON u.id = s.user_id
		WHERE `+whereSQL+`
		ORDER BY s.expires_at DESC, s.created_at DESC
		LIMIT ? OFFSET ?`), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Subscription{}
	for rows.Next() {
		item, err := scanSubscriptionWithUser(rows)
		if err != nil {
			return nil, 0, err
		}
		if item.Status == SubscriptionStatusActive && item.UserID != "" {
			if err := s.populateSubscriptionCoverage(ctx, item.UserID, now, &item); err != nil {
				return nil, 0, err
			}
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
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
		WHERE user_id = ? AND status = ? AND starts_at <= ? AND expires_at > ?`),
		userID,
		SubscriptionStatusActive,
		now,
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
	return s.populateSubscriptionCoverage(ctx, userID, now, item)
}

func (s *Store) populateSubscriptionCoverage(ctx context.Context, userID string, now time.Time, item *Subscription) error {
	if item == nil || strings.TrimSpace(userID) == "" {
		return nil
	}
	var expiresAt sql.NullTime
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT MAX(expires_at)
		FROM business_user_subscriptions
		WHERE user_id = ? AND status = ? AND expires_at > ?`),
		userID,
		SubscriptionStatusActive,
		now,
	).Scan(&expiresAt)
	if err != nil {
		return err
	}
	if expiresAt.Valid {
		item.CoverageExpiresAt = expiresAt.Time.UTC().Format(time.RFC3339Nano)
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
	if err := s.expireUserSubscriptionsWithTx(ctx, tx, userID, now); err != nil {
		return ReserveResult{}, err
	}
	row := tx.QueryRowContext(ctx, s.rebind(`SELECT id, credits_total, credits_used
		FROM business_user_subscriptions
		WHERE user_id = ? AND status = ? AND starts_at <= ? AND expires_at > ? AND credits_total > credits_used
		ORDER BY expires_at ASC, created_at ASC
		LIMIT 1
		FOR UPDATE`), userID, SubscriptionStatusActive, now, now)
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReserveResult{}, err
	}
	defer tx.Rollback()
	var subscriptionID string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT source_id
		FROM business_credit_ledger
		WHERE user_id = ? AND generation_id = ? AND reason = ? AND source_type = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1`), userID, generationID, ReasonSubscriptionReserve, SourceTypeSubscription).Scan(&subscriptionID); err != nil {
		return ReserveResult{}, err
	}
	row := tx.QueryRowContext(ctx, s.rebind(`SELECT credits_total, credits_used
		FROM business_user_subscriptions
		WHERE id = ? AND user_id = ?
		FOR UPDATE`), subscriptionID, userID)
	var total int64
	var used int64
	if err := row.Scan(&total, &used); err != nil {
		return ReserveResult{}, err
	}
	totals, err := s.subscriptionGenerationTotalsWithTx(ctx, tx, userID, generationID)
	if err != nil {
		return ReserveResult{}, err
	}
	refundable := totals.Reserved - totals.Refunded
	if refundable <= 0 {
		if err := tx.Commit(); err != nil {
			return ReserveResult{}, err
		}
		return ReserveResult{SubscriptionID: subscriptionID, CreditsUsed: used, CreditsLeft: total - used}, nil
	}
	if amount > refundable {
		amount = refundable
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
	return s.subscriptionGenerationTotalsQuery(ctx, s.db, userID, generationID)
}

func (s *Store) subscriptionGenerationTotalsWithTx(ctx context.Context, tx *sql.Tx, userID string, generationID string) (SubscriptionTotals, error) {
	if tx == nil {
		return SubscriptionTotals{}, fmt.Errorf("transaction is required")
	}
	userID = strings.TrimSpace(userID)
	generationID = strings.TrimSpace(generationID)
	if userID == "" || generationID == "" {
		return SubscriptionTotals{}, nil
	}
	return s.subscriptionGenerationTotalsQuery(ctx, tx, userID, generationID)
}

type paymentQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) subscriptionGenerationTotalsQuery(ctx context.Context, queryer paymentQueryer, userID string, generationID string) (SubscriptionTotals, error) {
	var totals SubscriptionTotals
	err := queryer.QueryRowContext(ctx, s.rebind(`SELECT
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
	startsAt := now
	expiresAtOverride := time.Time{}
	if order.BillingAction == BillingActionUpgrade {
		existing, err := s.currentActiveSubscriptionWithTx(ctx, tx, order.UserID, now)
		if err != nil {
			return Subscription{}, err
		}
		if existing.ExpiresAt != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, existing.ExpiresAt); err == nil && parsed.After(now) {
				expiresAtOverride = parsed.UTC()
			}
		}
		if err := s.markActiveSubscriptionsUpgradedWithTx(ctx, tx, order.UserID, now); err != nil {
			return Subscription{}, err
		}
	} else {
		existing, err := s.currentActiveSubscriptionWithTx(ctx, tx, order.UserID, now)
		if err != nil {
			return Subscription{}, err
		}
		if existing.ExpiresAt != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, existing.ExpiresAt); err == nil && parsed.After(now) {
				startsAt = parsed.UTC()
			}
		}
	}
	durationDays := normalizeDurationDays(PackageTypeSubscription, order.DurationDays)
	expiresAt := startsAt.AddDate(0, 0, durationDays)
	if !expiresAtOverride.IsZero() && expiresAtOverride.After(startsAt) {
		expiresAt = expiresAtOverride
	}
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
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO business_user_subscriptions(
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
	if err := s.expireUserSubscriptionsWithTx(ctx, tx, userID, now); err != nil {
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

func (s *Store) currentUsableSubscriptionWithTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) (Subscription, error) {
	if err := s.expireUserSubscriptionsWithTx(ctx, tx, userID, now); err != nil {
		return Subscription{}, err
	}
	row := tx.QueryRowContext(ctx, s.rebind(`SELECT id, user_id, order_id, package_id, package_name,
			duration_days, credits_total, credits_used,
			status, starts_at, expires_at, cancelled_at, created_at, updated_at
		FROM business_user_subscriptions
		WHERE user_id = ? AND status = ? AND starts_at <= ? AND expires_at > ?
		ORDER BY starts_at DESC, created_at DESC
		LIMIT 1
		FOR UPDATE`), userID, SubscriptionStatusActive, now, now)
	item, err := scanSubscription(row)
	if err == sql.ErrNoRows {
		return Subscription{}, nil
	}
	return item, err
}

func (s *Store) activeSubscriptionRunsWithTx(ctx context.Context, tx *sql.Tx, userID string, packageID string, now time.Time) ([]subscriptionRun, error) {
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT s.starts_at, s.expires_at, s.duration_days, s.credits_total, s.credits_used,
				COALESCE(NULLIF(o.original_amount_cents, 0), o.amount_cents, 0)
			FROM business_user_subscriptions s
			LEFT JOIN business_payment_orders o ON o.id = s.order_id
			WHERE s.user_id = ? AND s.status = ? AND s.package_id = ? AND s.expires_at > ?
			ORDER BY s.starts_at ASC
			FOR UPDATE OF s`),
		userID,
		SubscriptionStatusActive,
		packageID,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []subscriptionRun{}
	for rows.Next() {
		var run subscriptionRun
		var used int64
		if err := rows.Scan(&run.StartsAt, &run.ExpiresAt, &run.DurationDays, &run.CreditsTotal, &used, &run.AmountCents); err != nil {
			return nil, err
		}
		run.CreditsLeft = run.CreditsTotal - used
		if run.CreditsLeft < 0 {
			run.CreditsLeft = 0
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) markActiveSubscriptionsUpgradedWithTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) error {
	_, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
		SET status = ?, cancelled_at = ?, updated_at = ?
		WHERE user_id = ? AND status = ? AND expires_at > ?`),
		SubscriptionStatusUpgraded,
		now,
		now,
		userID,
		SubscriptionStatusActive,
		now,
	)
	return err
}

func (s *Store) expireUserSubscriptionsWithTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) error {
	_, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
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

func (s *Store) expireAllSubscriptions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_user_subscriptions
		SET status = ?, updated_at = ?
		WHERE status = ? AND expires_at <= ?`),
		SubscriptionStatusExpired,
		now,
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
			payment_method, provider_key, provider_instance_id, out_trade_no, provider_trade_no, status,
			pay_url, qr_code, expires_at, paid_at, completed_at, failed_at, refunded_at,
			credit_ledger_id, billing_action, upgrade_from_subscription_id, upgrade_credit_cents, original_amount_cents,
			created_at, updated_at
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
		&item.LevelTag,
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
	item.LevelTag = normalizeLevelTag(item.LevelTag)
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
	updateSubscriptionActive(&item, time.Now().UTC())
	return item, nil
}

func scanSubscriptionWithUser(row interface{ Scan(dest ...any) error }) (Subscription, error) {
	var item Subscription
	var cancelledAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.UserEmail,
		&item.Username,
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
	updateSubscriptionActive(&item, time.Now().UTC())
	return item, nil
}

func updateSubscriptionActive(item *Subscription, now time.Time) {
	if item == nil || item.Status != SubscriptionStatusActive {
		if item != nil {
			item.Active = false
		}
		return
	}
	startsAt, startErr := time.Parse(time.RFC3339Nano, item.StartsAt)
	expiresAt, expireErr := time.Parse(time.RFC3339Nano, item.ExpiresAt)
	if startErr != nil || expireErr != nil {
		item.Active = true
		return
	}
	item.Active = !startsAt.After(now) && expiresAt.After(now)
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
		&item.PaymentMethod,
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
		&item.BillingAction,
		&item.UpgradeFromSubID,
		&item.UpgradeCreditCents,
		&item.OriginalAmountCents,
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
	item.BillingAction = normalizeBillingAction(item.BillingAction)
	item.PaymentMethod = normalizePaymentMethod(item.PaymentMethod)
	if item.PaymentMethod == "" {
		item.PaymentMethod = MethodManual
	}
	if item.OriginalAmountCents <= 0 {
		item.OriginalAmountCents = item.AmountCents
	}
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
		LevelTag:     normalizeLevelTag(input.LevelTag),
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

func normalizeSubscriptionStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SubscriptionStatusActive, SubscriptionStatusExpired, SubscriptionStatusCancelled, SubscriptionStatusUpgraded:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeBillingAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case BillingActionNew, BillingActionRenewal, BillingActionUpgrade:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeProviderKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProviderEasyPay:
		return ProviderEasyPay
	default:
		return ""
	}
}

func providerName(providerKey string) string {
	switch normalizeProviderKey(providerKey) {
	case ProviderEasyPay:
		return "EasyPay"
	default:
		return "支付渠道"
	}
}

func normalizePaymentMethod(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case MethodManual:
		return MethodManual
	case MethodAlipay, "ali_pay":
		return MethodAlipay
	case MethodWxpay, "wechat", "wechat_pay", "weixin":
		return MethodWxpay
	default:
		return ""
	}
}

func paymentMethodLabel(method string) string {
	switch normalizePaymentMethod(method) {
	case MethodManual:
		return "人工确认"
	case MethodAlipay:
		return "支付宝"
	case MethodWxpay:
		return "微信支付"
	default:
		return method
	}
}

func normalizePaymentMethods(values []string, providerKey string) []string {
	if providerKey == ProviderManual {
		return []string{MethodManual}
	}
	allowed := map[string]bool{
		MethodAlipay: true,
		MethodWxpay:  true,
	}
	seen := map[string]bool{}
	items := []string{}
	for _, value := range values {
		method := normalizePaymentMethod(value)
		if !allowed[method] || seen[method] {
			continue
		}
		seen[method] = true
		items = append(items, method)
	}
	if len(items) == 0 && normalizeProviderKey(providerKey) == ProviderEasyPay {
		items = []string{MethodAlipay, MethodWxpay}
	}
	return items
}

func normalizeProviderConfig(providerKey string, configMap map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range configMap {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		out[key] = value
	}
	if normalizeProviderKey(providerKey) == ProviderEasyPay {
		if out["paymentMode"] == "" {
			out["paymentMode"] = "qrcode"
		}
		out["apiBase"] = normalizeEasyPayAPIBase(out["apiBase"])
	}
	return out
}

func validateProviderConfig(providerKey string, configMap map[string]string) error {
	switch normalizeProviderKey(providerKey) {
	case ProviderEasyPay:
		if configMap["apiBase"] == "" || configMap["pid"] == "" || configMap["pkey"] == "" {
			return ErrProviderInvalid
		}
		if _, err := url.ParseRequestURI(configMap["apiBase"]); err != nil {
			return ErrProviderInvalid
		}
		return nil
	default:
		return ErrProviderInvalid
	}
}

func mergeProviderConfig(providerKey string, current map[string]string, next map[string]string) map[string]string {
	merged := map[string]string{}
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range next {
		if isProviderSecretKey(key) && (value == "" || isMaskedSecret(value)) {
			continue
		}
		merged[key] = value
	}
	return normalizeProviderConfig(providerKey, merged)
}

func maskProviderConfig(providerKey string, configMap map[string]string) map[string]string {
	if len(configMap) == 0 {
		return nil
	}
	masked := map[string]string{}
	for key, value := range configMap {
		if isProviderSecretKey(key) && value != "" {
			masked[key] = "********"
			continue
		}
		masked[key] = value
	}
	return normalizeProviderConfig(providerKey, masked)
}

func isProviderSecretKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "pkey", "secret", "key", "api_key":
		return true
	default:
		return false
	}
}

func isMaskedSecret(value string) bool {
	value = strings.TrimSpace(value)
	return value == "********" || value == "******" || value == "••••••••"
}

func normalizeEasyPayAPIBase(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		value = "https://" + value
	}
	return value
}

func firstPaymentMethod(values []string) string {
	for _, value := range values {
		if method := normalizePaymentMethod(value); method != "" {
			return method
		}
	}
	return MethodManual
}

func stringSliceContains(values []string, target string) bool {
	target = normalizePaymentMethod(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if normalizePaymentMethod(value) == target {
			return true
		}
	}
	return false
}

func normalizeOrderKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "balance", PackageTypeSubscription, BillingActionRenewal, BillingActionUpgrade:
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

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func clampCents(value int64, maxValue int64) int64 {
	if value < 0 {
		return 0
	}
	if value > maxValue {
		return maxValue
	}
	return value
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func easyPaySign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "sign" || key == "sign_type" || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	raw := strings.Join(parts, "&") + strings.TrimSpace(secret)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func moneyToCents(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int64(amount*100 + 0.5)
}

func providerSnapshot(provider providerRuntime, configMap map[string]string, method string) map[string]any {
	return map[string]any{
		"id":               provider.ID,
		"providerKey":      provider.ProviderKey,
		"name":             provider.Name,
		"paymentMethod":    normalizePaymentMethod(method),
		"supportedMethods": provider.SupportedMethods,
		"config":           maskProviderConfig(provider.ProviderKey, configMap),
	}
}

func (s *Store) createProviderPayment(ctx context.Context, provider providerRuntime, configMap map[string]string, order Order, input CreateOrderInput) (ProviderPayment, error) {
	switch provider.ProviderKey {
	case ProviderManual, "":
		return ProviderPayment{}, nil
	case ProviderEasyPay:
		runtime, err := NewEasyPayProvider(configMap)
		if err != nil {
			return ProviderPayment{}, err
		}
		baseURL := strings.TrimRight(strings.TrimSpace(input.NotifyBaseURL), "/")
		if baseURL == "" {
			return ProviderPayment{}, ErrProviderInvalid
		}
		return runtime.CreatePayment(ctx, ProviderPaymentRequest{
			Order:     order,
			Method:    order.PaymentMethod,
			Subject:   fmt.Sprintf("ImageStudio %s", order.OutTradeNo),
			NotifyURL: baseURL + "/api/business/payment/webhook/easypay",
			ReturnURL: firstNonEmpty(input.ReturnURL, baseURL+"/credits"),
			ClientIP:  input.ClientIP,
			UserAgent: input.UserAgent,
		})
	default:
		return ProviderPayment{}, ErrProviderInvalid
	}
}

func paymentConfigSecret(cfg *config.Config) string {
	if value := strings.TrimSpace(os.Getenv("PAYMENT_CONFIG_SECRET")); value != "" {
		return value
	}
	if cfg != nil {
		if value := strings.TrimSpace(cfg.App.AuthKey); value != "" {
			return value
		}
		if value := strings.TrimSpace(cfg.App.APIKey); value != "" {
			return value
		}
	}
	return "imagestudio-payment-config"
}

func (s *Store) encryptProviderConfig(configMap map[string]string) ([]byte, error) {
	if len(configMap) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}
	gcm, err := providerConfigGCM(s.configSecret)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nonce, nonce, raw, nil)
	return []byte("v1:" + base64.RawURLEncoding.EncodeToString(sealed)), nil
}

func (s *Store) decryptProviderConfig(raw []byte) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	text := strings.TrimSpace(string(raw))
	var payload []byte
	if strings.HasPrefix(text, "v1:") {
		data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(text, "v1:"))
		if err != nil {
			return nil
		}
		gcm, err := providerConfigGCM(s.configSecret)
		if err != nil || len(data) < gcm.NonceSize() {
			return nil
		}
		nonce := data[:gcm.NonceSize()]
		ciphertext := data[gcm.NonceSize():]
		payload, err = gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil
		}
	} else {
		payload = raw
	}
	out := map[string]string{}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil
	}
	return out
}

func providerConfigGCM(secret string) (cipher.AEAD, error) {
	sum := sha256.Sum256([]byte(firstNonEmpty(secret, "imagestudio-payment-config")))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
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

func normalizeLevelTag(value string) string {
	tag := strings.ToLower(strings.TrimSpace(value))
	if tag == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range tag {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == ':', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
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
