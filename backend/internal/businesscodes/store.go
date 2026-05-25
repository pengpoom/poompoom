package businesscodes

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

const (
	TypeRedeem = "redeem"
	TypePromo  = "promo"
	TypeInvite = "invite"

	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusExpired  = "expired"

	ContextRegistration = "registration"
	ContextRecharge     = "recharge_center"

	ReasonRedeemCode        = "redeem_code"
	ReasonRegistrationPromo = "registration_promo"

	affiliateProfilePrefix = "AFF"
)

var (
	ErrCodeInvalid    = errors.New("code is invalid")
	ErrCodeInactive   = errors.New("code is inactive")
	ErrCodeExhausted  = errors.New("code is exhausted")
	ErrCodeUsed       = errors.New("code already used")
	ErrCodeWrongType  = errors.New("code type is not allowed")
	ErrCreditsInvalid = errors.New("credits must be non-negative")
)

type Code struct {
	ID          string `json:"id"`
	CodePreview string `json:"codePreview"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Credits     int64  `json:"credits"`
	MaxUses     int    `json:"maxUses"`
	UsedCount   int    `json:"usedCount"`
	Status      string `json:"status"`
	StartsAt    string `json:"startsAt,omitempty"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
	CreatedBy   string `json:"createdBy"`
	Note        string `json:"note"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type Usage struct {
	ID             string `json:"id"`
	CodeID         string `json:"codeId"`
	UserID         string `json:"userId"`
	Context        string `json:"context"`
	CreditsGranted int64  `json:"creditsGranted"`
	LedgerID       string `json:"ledgerId"`
	CreatedAt      string `json:"createdAt"`
}

type UsageWithUser struct {
	Usage
	UID        int64  `json:"uid"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	UserStatus string `json:"userStatus"`
}

type CreateInput struct {
	Code      string
	Type      string
	Title     string
	Credits   int64
	MaxUses   int
	Status    string
	StartsAt  string
	ExpiresAt string
	CreatedBy string
	Note      string
}

type UpdateInput struct {
	Title     string
	Credits   int64
	MaxUses   int
	Status    string
	StartsAt  string
	ExpiresAt string
	Note      string
}

type ListFilters struct {
	Type   string
	Status string
	Search string
}

type ConsumeResult struct {
	Code           Code   `json:"code"`
	Usage          Usage  `json:"usage"`
	CreditsGranted int64  `json:"creditsGranted"`
	LedgerID       string `json:"ledgerId,omitempty"`
}

type AffiliateProfile struct {
	UserID      string `json:"userId"`
	CodePreview string `json:"codePreview"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type AffiliateSummary struct {
	Enabled                   bool             `json:"enabled"`
	Profile                   AffiliateProfile `json:"profile,omitempty"`
	ReferralCount             int64            `json:"referralCount"`
	RegistrationRewardEnabled bool             `json:"registrationRewardEnabled"`
	RegistrationRewardCredits int64            `json:"registrationRewardCredits"`
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

func (s *Store) List(ctx context.Context, limit int, filter ListFilters) ([]Code, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT id, code_preview, type, title, credits, max_uses, used_count, status,
			starts_at, expires_at, created_by, note, created_at, updated_at
		FROM business_codes
		WHERE 1 = 1`
	args := []any{}
	if strings.EqualFold(strings.TrimSpace(filter.Type), "registration") {
		query += ` AND type IN (?, ?)`
		args = append(args, TypeInvite, TypePromo)
	} else if codeType := normalizeType(filter.Type); codeType != "" {
		query += ` AND type = ?`
		args = append(args, codeType)
	}
	if status := normalizeStatus(filter.Status); status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	if search != "" {
		runes := []rune(search)
		if len(runes) > 200 {
			search = string(runes[:200])
		}
		pattern := "%" + search + "%"
		query += ` AND (LOWER(title) LIKE ? OR LOWER(code_preview) LIKE ? OR LOWER(note) LIKE ?)`
		args = append(args, pattern, pattern, pattern)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Code{}
	for rows.Next() {
		item, err := scanCode(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Create(ctx context.Context, input CreateInput) (Code, string, error) {
	item, rawCode, err := normalizeCreateInput(input)
	if err != nil {
		return Code{}, "", err
	}
	now := time.Now().UTC()
	item.ID = newID("code")
	item.CodePreview = normalizeRawCode(rawCode)
	item.CreatedAt = now.Format(time.RFC3339Nano)
	item.UpdatedAt = item.CreatedAt
	startsAt, err := parseOptionalTime(input.StartsAt)
	if err != nil {
		return Code{}, "", fmt.Errorf("starts_at is invalid")
	}
	expiresAt, err := parseOptionalTime(input.ExpiresAt)
	if err != nil {
		return Code{}, "", fmt.Errorf("expires_at is invalid")
	}
	if !startsAt.IsZero() && !expiresAt.IsZero() && !expiresAt.After(startsAt) {
		return Code{}, "", fmt.Errorf("expires_at must be after starts_at")
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_codes(
			id, code_hash, code_preview, type, title, credits, max_uses, used_count, status,
			starts_at, expires_at, created_by, note, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		item.ID,
		hashCode(rawCode),
		item.CodePreview,
		item.Type,
		item.Title,
		item.Credits,
		item.MaxUses,
		0,
		item.Status,
		nullableTime(startsAt),
		nullableTime(expiresAt),
		item.CreatedBy,
		item.Note,
		now,
		now,
	)
	if err != nil {
		return Code{}, "", err
	}
	created, ok, err := s.Get(ctx, item.ID)
	if err != nil || !ok {
		return item, rawCode, err
	}
	return created, rawCode, nil
}

func (s *Store) Get(ctx context.Context, id string) (Code, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Code{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, s.rebind(`SELECT id, code_preview, type, title, credits, max_uses, used_count, status,
			starts_at, expires_at, created_by, note, created_at, updated_at
		FROM business_codes
		WHERE id = ?`), id)
	item, err := scanCode(row)
	if err == sql.ErrNoRows {
		return Code{}, false, nil
	}
	return item, err == nil, err
}

func (s *Store) Update(ctx context.Context, id string, input UpdateInput) (Code, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Code{}, false, nil
	}
	current, ok, err := s.Get(ctx, id)
	if err != nil || !ok {
		return Code{}, ok, err
	}
	title := cleanText(input.Title, 80)
	if title == "" {
		return Code{}, false, fmt.Errorf("title is required")
	}
	if input.Credits < 0 {
		return Code{}, false, ErrCreditsInvalid
	}
	if current.Type == TypeInvite && input.Credits > 0 {
		return Code{}, false, fmt.Errorf("invite code cannot grant credits")
	}
	if current.Type == TypeRedeem && input.Credits <= 0 {
		return Code{}, false, fmt.Errorf("redeem code credits must be positive")
	}
	if input.MaxUses < 1 {
		return Code{}, false, fmt.Errorf("max_uses must be positive")
	}
	status := normalizeStatus(input.Status)
	if status == "" {
		return Code{}, false, fmt.Errorf("status is invalid")
	}
	startsAt, err := parseOptionalTime(input.StartsAt)
	if err != nil {
		return Code{}, false, fmt.Errorf("starts_at is invalid")
	}
	expiresAt, err := parseOptionalTime(input.ExpiresAt)
	if err != nil {
		return Code{}, false, fmt.Errorf("expires_at is invalid")
	}
	if !startsAt.IsZero() && !expiresAt.IsZero() && !expiresAt.After(startsAt) {
		return Code{}, false, fmt.Errorf("expires_at must be after starts_at")
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_codes
		SET title = ?, credits = ?, max_uses = ?, status = ?, starts_at = ?, expires_at = ?, note = ?, updated_at = ?
		WHERE id = ?`),
		title,
		input.Credits,
		input.MaxUses,
		status,
		nullableTime(startsAt),
		nullableTime(expiresAt),
		cleanText(input.Note, 500),
		now,
		id,
	)
	if err != nil {
		return Code{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Code{}, false, err
	}
	if affected == 0 {
		return Code{}, false, nil
	}
	item, ok, err := s.Get(ctx, id)
	return item, ok, err
}

func (s *Store) Delete(ctx context.Context, id string) (bool, error) {
	id = cleanID(id)
	if id == "" {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_codes WHERE id = ?`), id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) UpdateStatusBatch(ctx context.Context, ids []string, status string) (int64, error) {
	status = normalizeStatus(status)
	if status == "" {
		return 0, fmt.Errorf("status is invalid")
	}
	cleaned := make([]string, 0, len(ids))
	for _, id := range ids {
		id = cleanID(id)
		if id != "" {
			cleaned = append(cleaned, id)
		}
	}
	if len(cleaned) == 0 {
		return 0, nil
	}
	if len(cleaned) > 200 {
		return 0, fmt.Errorf("too many codes")
	}

	placeholders := make([]string, 0, len(cleaned))
	args := make([]any, 0, len(cleaned)+2)
	args = append(args, status, time.Now().UTC())
	for _, id := range cleaned {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_codes
		SET status = ?, updated_at = ?
		WHERE id IN (`+strings.Join(placeholders, ", ")+`)`), args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (s *Store) ListUsages(ctx context.Context, codeID string, limit int) ([]UsageWithUser, error) {
	codeID = cleanID(codeID)
	if codeID == "" {
		return []UsageWithUser{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT
			u.id, u.code_id, u.user_id, u.context, u.credits_granted, u.ledger_id, u.created_at,
			COALESCE(bu.uid, 0), COALESCE(bu.username, ''), COALESCE(bu.email, ''), COALESCE(bu.status, '')
		FROM business_code_usages AS u
		LEFT JOIN business_users AS bu ON bu.id = u.user_id
		WHERE u.code_id = ?
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT ?`), codeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []UsageWithUser{}
	for rows.Next() {
		var item UsageWithUser
		var createdAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.CodeID,
			&item.UserID,
			&item.Context,
			&item.CreditsGranted,
			&item.LedgerID,
			&createdAt,
			&item.UID,
			&item.Username,
			&item.Email,
			&item.UserStatus,
		); err != nil {
			return nil, err
		}
		item.CreatedAt = formatNullTime(createdAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ValidateRegistrationCode(ctx context.Context, rawCode string) (Code, error) {
	rawCode = normalizeRawCode(rawCode)
	if rawCode == "" {
		return Code{}, ErrCodeInvalid
	}
	code, err := s.codeByHash(ctx, hashCode(rawCode))
	if err != nil {
		return Code{}, err
	}
	if code.Type != TypeInvite && code.Type != TypePromo {
		return Code{}, ErrCodeWrongType
	}
	if err := validateUsableCode(code, time.Now().UTC()); err != nil {
		return Code{}, err
	}
	return code, nil
}

func (s *Store) ConsumeRegistrationCode(ctx context.Context, rawCode string, userID string, writeCredit func(context.Context, *sql.Tx, string, int64, string) (string, error)) (ConsumeResult, error) {
	return s.consume(ctx, rawCode, userID, []string{TypeInvite, TypePromo}, ContextRegistration, func(code Code) bool {
		return code.Type == TypePromo && code.Credits > 0
	}, writeCredit)
}

func (s *Store) ConsumeRegistrationCodeWithTx(ctx context.Context, tx *sql.Tx, rawCode string, userID string, writeCredit func(context.Context, *sql.Tx, string, int64, string) (string, error)) (ConsumeResult, error) {
	return s.consumeWithTx(ctx, tx, rawCode, userID, []string{TypeInvite, TypePromo}, ContextRegistration, func(code Code) bool {
		return code.Type == TypePromo && code.Credits > 0
	}, writeCredit)
}

func (s *Store) ConsumeRedeemCode(ctx context.Context, rawCode string, userID string, writeCredit func(context.Context, *sql.Tx, string, int64, string) (string, error)) (ConsumeResult, error) {
	return s.consume(ctx, rawCode, userID, []string{TypeRedeem}, ContextRecharge, func(code Code) bool {
		return code.Credits > 0
	}, writeCredit)
}

func (s *Store) EnsureAffiliateProfile(ctx context.Context, userID string) (AffiliateProfile, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AffiliateProfile{}, fmt.Errorf("user id is required")
	}
	if profile, ok, err := s.GetAffiliateProfile(ctx, userID); err != nil || ok {
		return profile, err
	}
	rawCode, err := randomDisplayCode(affiliateProfilePrefix, 8)
	if err != nil {
		return AffiliateProfile{}, err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_affiliate_profiles(user_id, code_hash, code_preview, enabled, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO NOTHING`),
		userID,
		hashCode(rawCode),
		rawCode,
		1,
		now,
		now,
	)
	if err != nil {
		return AffiliateProfile{}, err
	}
	profile, _, err := s.GetAffiliateProfile(ctx, userID)
	return profile, err
}

func (s *Store) GetAffiliateProfile(ctx context.Context, userID string) (AffiliateProfile, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AffiliateProfile{}, false, nil
	}
	var profile AffiliateProfile
	var enabled int
	var createdAt sql.NullTime
	var updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT user_id, code_preview, enabled, created_at, updated_at
		FROM business_affiliate_profiles
		WHERE user_id = ?`), userID).Scan(&profile.UserID, &profile.CodePreview, &enabled, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return AffiliateProfile{}, false, nil
	}
	if err != nil {
		return AffiliateProfile{}, false, err
	}
	profile.Enabled = enabled != 0
	profile.CreatedAt = formatNullTime(createdAt)
	profile.UpdatedAt = formatNullTime(updatedAt)
	return profile, true, nil
}

func (s *Store) AffiliateSummary(ctx context.Context, userID string, enabled bool, registrationRewardEnabled bool, registrationRewardCredits int64) (AffiliateSummary, error) {
	if !enabled {
		return AffiliateSummary{Enabled: false}, nil
	}
	profile, err := s.EnsureAffiliateProfile(ctx, userID)
	if err != nil {
		return AffiliateSummary{}, err
	}
	var referralCount int64
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM business_affiliate_referrals WHERE referrer_user_id = ?`), userID).Scan(&referralCount); err != nil {
		return AffiliateSummary{}, err
	}
	return AffiliateSummary{
		Enabled:                   true,
		Profile:                   profile,
		ReferralCount:             referralCount,
		RegistrationRewardEnabled: registrationRewardEnabled,
		RegistrationRewardCredits: registrationRewardCredits,
	}, nil
}

type AffiliateBindResult struct {
	Bound          bool
	ReferrerUserID string
}

func (s *Store) BindAffiliateReferralWithTx(ctx context.Context, tx *sql.Tx, affiliateCode string, referredUserID string) (AffiliateBindResult, error) {
	affiliateCode = normalizeRawCode(affiliateCode)
	referredUserID = strings.TrimSpace(referredUserID)
	if affiliateCode == "" || referredUserID == "" {
		return AffiliateBindResult{}, nil
	}
	if tx == nil {
		return AffiliateBindResult{}, fmt.Errorf("transaction is required")
	}
	var referrerUserID string
	var codePreview string
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT user_id, code_preview
		FROM business_affiliate_profiles
		WHERE code_hash = ? AND enabled = 1`), hashCode(affiliateCode)).Scan(&referrerUserID, &codePreview)
	if err == sql.ErrNoRows {
		return AffiliateBindResult{}, nil
	}
	if err != nil {
		return AffiliateBindResult{}, err
	}
	if strings.TrimSpace(referrerUserID) == "" || referrerUserID == referredUserID {
		return AffiliateBindResult{}, nil
	}
	result, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO business_affiliate_referrals(
			id, referrer_user_id, referred_user_id, affiliate_code_preview, status, created_at
		) VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(referred_user_id) DO NOTHING`),
		newID("aff_ref"),
		referrerUserID,
		referredUserID,
		codePreview,
		"bound",
		time.Now().UTC(),
	)
	if err != nil {
		return AffiliateBindResult{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return AffiliateBindResult{}, err
	}
	if rowsAffected == 0 {
		return AffiliateBindResult{}, nil
	}
	return AffiliateBindResult{Bound: true, ReferrerUserID: referrerUserID}, nil
}

func (s *Store) consume(ctx context.Context, rawCode string, userID string, allowedTypes []string, usageContext string, shouldGrantCredits func(Code) bool, writeCredit func(context.Context, *sql.Tx, string, int64, string) (string, error)) (ConsumeResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConsumeResult{}, err
	}
	defer tx.Rollback()

	result, err := s.consumeWithTx(ctx, tx, rawCode, userID, allowedTypes, usageContext, shouldGrantCredits, writeCredit)
	if err != nil {
		return ConsumeResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ConsumeResult{}, err
	}
	return result, nil
}

func (s *Store) consumeWithTx(ctx context.Context, tx *sql.Tx, rawCode string, userID string, allowedTypes []string, usageContext string, shouldGrantCredits func(Code) bool, writeCredit func(context.Context, *sql.Tx, string, int64, string) (string, error)) (ConsumeResult, error) {
	rawCode = normalizeRawCode(rawCode)
	userID = strings.TrimSpace(userID)
	if rawCode == "" {
		return ConsumeResult{}, ErrCodeInvalid
	}
	if userID == "" {
		return ConsumeResult{}, fmt.Errorf("user id is required")
	}
	if tx == nil {
		return ConsumeResult{}, fmt.Errorf("transaction is required")
	}

	code, err := s.codeByHashForUpdate(ctx, tx, hashCode(rawCode))
	if err != nil {
		return ConsumeResult{}, err
	}
	if !containsType(allowedTypes, code.Type) {
		return ConsumeResult{}, ErrCodeWrongType
	}
	if err := validateUsableCode(code, time.Now().UTC()); err != nil {
		return ConsumeResult{}, err
	}
	var existingID string
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT id
		FROM business_code_usages
		WHERE code_id = ? AND user_id = ? AND context = ?
		LIMIT 1`), code.ID, userID, usageContext).Scan(&existingID)
	if err == nil {
		return ConsumeResult{}, ErrCodeUsed
	}
	if err != sql.ErrNoRows {
		return ConsumeResult{}, err
	}

	creditsGranted := int64(0)
	ledgerID := ""
	if shouldGrantCredits(code) {
		if writeCredit == nil {
			return ConsumeResult{}, fmt.Errorf("credit writer is required")
		}
		ledgerID, err = writeCredit(ctx, tx, userID, code.Credits, creditReasonForContext(usageContext))
		if err != nil {
			return ConsumeResult{}, err
		}
		creditsGranted = code.Credits
	}
	usage := Usage{
		ID:             newID("code_use"),
		CodeID:         code.ID,
		UserID:         userID,
		Context:        usageContext,
		CreditsGranted: creditsGranted,
		LedgerID:       ledgerID,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	if _, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO business_code_usages(id, code_id, user_id, context, credits_granted, ledger_id, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)`),
		usage.ID,
		usage.CodeID,
		usage.UserID,
		usage.Context,
		usage.CreditsGranted,
		usage.LedgerID,
		time.Now().UTC(),
	); err != nil {
		return ConsumeResult{}, err
	}
	if _, err := tx.ExecContext(ctx, s.rebind(`UPDATE business_codes
		SET used_count = used_count + 1, updated_at = ?
	WHERE id = ?`), time.Now().UTC(), code.ID); err != nil {
		return ConsumeResult{}, err
	}
	code.UsedCount++
	code.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return ConsumeResult{
		Code:           code,
		Usage:          usage,
		CreditsGranted: creditsGranted,
		LedgerID:       ledgerID,
	}, nil
}

func (s *Store) codeByHash(ctx context.Context, codeHash string) (Code, error) {
	row := s.db.QueryRowContext(ctx, s.rebind(`SELECT id, code_preview, type, title, credits, max_uses, used_count, status,
			starts_at, expires_at, created_by, note, created_at, updated_at
		FROM business_codes
		WHERE code_hash = ?`), codeHash)
	item, err := scanCode(row)
	if err == sql.ErrNoRows {
		return Code{}, ErrCodeInvalid
	}
	return item, err
}

func (s *Store) codeByHashForUpdate(ctx context.Context, tx *sql.Tx, codeHash string) (Code, error) {
	row := tx.QueryRowContext(ctx, s.rebind(`SELECT id, code_preview, type, title, credits, max_uses, used_count, status,
			starts_at, expires_at, created_by, note, created_at, updated_at
		FROM business_codes
		WHERE code_hash = ?
		FOR UPDATE`), codeHash)
	item, err := scanCode(row)
	if err == sql.ErrNoRows {
		return Code{}, ErrCodeInvalid
	}
	return item, err
}

func scanCode(row interface{ Scan(dest ...any) error }) (Code, error) {
	var item Code
	var startsAt sql.NullTime
	var expiresAt sql.NullTime
	var createdAt time.Time
	var updatedAt time.Time
	err := row.Scan(
		&item.ID,
		&item.CodePreview,
		&item.Type,
		&item.Title,
		&item.Credits,
		&item.MaxUses,
		&item.UsedCount,
		&item.Status,
		&startsAt,
		&expiresAt,
		&item.CreatedBy,
		&item.Note,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return item, err
	}
	item.StartsAt = formatNullTime(startsAt)
	item.ExpiresAt = formatNullTime(expiresAt)
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return item, err
}

func normalizeCreateInput(input CreateInput) (Code, string, error) {
	codeType := normalizeType(input.Type)
	if codeType == "" {
		return Code{}, "", fmt.Errorf("type is invalid")
	}
	rawCode := normalizeRawCode(input.Code)
	if rawCode == "" {
		generated, err := randomDisplayCode(defaultCodePrefix(codeType), 10)
		if err != nil {
			return Code{}, "", err
		}
		rawCode = generated
	}
	title := cleanText(input.Title, 80)
	if title == "" {
		title = defaultTitle(codeType)
	}
	if input.Credits < 0 {
		return Code{}, "", ErrCreditsInvalid
	}
	if codeType == TypeInvite && input.Credits > 0 {
		return Code{}, "", fmt.Errorf("invite code cannot grant credits")
	}
	if codeType == TypeRedeem && input.Credits <= 0 {
		return Code{}, "", fmt.Errorf("redeem code credits must be positive")
	}
	maxUses := input.MaxUses
	if maxUses < 1 {
		maxUses = 1
	}
	status := normalizeStatus(input.Status)
	if status == "" {
		status = StatusActive
	}
	return Code{
		Type:      codeType,
		Title:     title,
		Credits:   input.Credits,
		MaxUses:   maxUses,
		Status:    status,
		CreatedBy: strings.TrimSpace(input.CreatedBy),
		Note:      cleanText(input.Note, 500),
	}, rawCode, nil
}

func validateUsableCode(code Code, now time.Time) error {
	if code.ID == "" {
		return ErrCodeInvalid
	}
	if code.Status != StatusActive {
		return ErrCodeInactive
	}
	if code.MaxUses > 0 && code.UsedCount >= code.MaxUses {
		return ErrCodeExhausted
	}
	if startsAt, err := parseOptionalTime(code.StartsAt); err == nil && !startsAt.IsZero() && startsAt.After(now) {
		return ErrCodeInactive
	}
	if expiresAt, err := parseOptionalTime(code.ExpiresAt); err == nil && !expiresAt.IsZero() && !expiresAt.After(now) {
		return ErrCodeInactive
	}
	return nil
}

func normalizeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TypeRedeem:
		return TypeRedeem
	case TypePromo:
		return TypePromo
	case TypeInvite:
		return TypeInvite
	default:
		return ""
	}
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case StatusActive:
		return StatusActive
	case StatusDisabled:
		return StatusDisabled
	case StatusExpired:
		return StatusExpired
	default:
		return ""
	}
}

func normalizeRawCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
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

func hashCode(rawCode string) string {
	sum := sha256.Sum256([]byte(normalizeRawCode(rawCode)))
	return hex.EncodeToString(sum[:])
}

func randomDisplayCode(prefix string, size int) (string, error) {
	if size < 6 {
		size = 6
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := strings.ToUpper(base64.RawURLEncoding.EncodeToString(raw))
	token = strings.NewReplacer("_", "", "-", "").Replace(token)
	if len(token) > size {
		token = token[:size]
	}
	prefix = normalizeRawCode(prefix)
	if prefix == "" {
		return token, nil
	}
	return prefix + token, nil
}

func parseOptionalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed.UTC(), nil
	}
	parsed, err = time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, err
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func formatNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339Nano)
}

func defaultCodePrefix(codeType string) string {
	switch codeType {
	case TypeRedeem:
		return "RD"
	case TypePromo:
		return "PM"
	case TypeInvite:
		return "IN"
	default:
		return "CD"
	}
}

func defaultTitle(codeType string) string {
	switch codeType {
	case TypeRedeem:
		return "兑换码"
	case TypePromo:
		return "优惠码"
	case TypeInvite:
		return "邀请码"
	default:
		return "注册码"
	}
}

func containsType(types []string, value string) bool {
	for _, item := range types {
		if item == value {
			return true
		}
	}
	return false
}

func creditReasonForContext(value string) string {
	if value == ContextRegistration {
		return ReasonRegistrationPromo
	}
	return ReasonRedeemCode
}

func newID(prefix string) string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return prefix + "_" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(raw[:])
}

func cleanID(id string) string {
	return strings.ReplaceAll(strings.TrimSpace(id), "/", "-")
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}
