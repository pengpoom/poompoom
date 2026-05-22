package businesscredits

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/sqlitedb"
)

const (
	ReasonAdminAdjustment        = "admin_adjustment"
	ReasonAdminRecharge          = "admin_recharge"
	ReasonAdminRefund            = "admin_refund"
	ReasonImageGenerationReserve = "image_generation_reserve"
	ReasonImageGenerationRefund  = "image_generation_refund"
)

var ErrInsufficientBalance = errors.New("insufficient credits")

type Summary struct {
	UserID    string `json:"user_id"`
	Balance   int64  `json:"balance"`
	Spent     int64  `json:"spent"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type LedgerEntry struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	Delta        int64  `json:"delta"`
	BalanceAfter int64  `json:"balance_after"`
	Reason       string `json:"reason"`
	GenerationID string `json:"generation_id,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type GenerationTotals struct {
	Reserved int64
	Refunded int64
}

type Store struct {
	db *sql.DB
}

func NewStore(cfg *config.Config) (*Store, error) {
	rawPath := strings.TrimSpace(cfg.Storage.SQLitePath)
	if rawPath == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	path := cfg.ResolvePath(rawPath)
	db, err := sqlitedb.Open(path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.init(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS business_user_credits (
			user_id TEXT PRIMARY KEY,
			balance INTEGER NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS business_credit_ledger (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			delta INTEGER NOT NULL,
			balance_after INTEGER NOT NULL,
			reason TEXT NOT NULL,
			generation_id TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_business_credit_ledger_user_created
			ON business_credit_ledger(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_credit_ledger_generation
			ON business_credit_ledger(generation_id);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Summary(ctx context.Context, userID string) (Summary, error) {
	userID = cleanUserID(userID)
	if userID == "" {
		return Summary{}, fmt.Errorf("user id is required")
	}
	summaries, err := s.Summaries(ctx, []string{userID})
	if err != nil {
		return Summary{}, err
	}
	summary := summaries[userID]
	summary.UserID = userID
	return summary, nil
}

func (s *Store) Summaries(ctx context.Context, userIDs []string) (map[string]Summary, error) {
	summaries := make(map[string]Summary, len(userIDs))
	for _, userID := range userIDs {
		cleaned := cleanUserID(userID)
		if cleaned == "" {
			continue
		}
		summaries[cleaned] = Summary{UserID: cleaned}
	}
	if len(summaries) == 0 {
		return summaries, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT user_id, balance, updated_at
		 FROM business_user_credits`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var summary Summary
		if err := rows.Scan(&summary.UserID, &summary.Balance, &summary.UpdatedAt); err != nil {
			return nil, err
		}
		if _, ok := summaries[summary.UserID]; !ok {
			continue
		}
		summaries[summary.UserID] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	spentRows, err := s.db.QueryContext(
		ctx,
		`SELECT user_id,
		        COALESCE(SUM(CASE
		          WHEN reason IN (?, ?) THEN -delta
		          ELSE 0
		        END), 0) AS spent
		 FROM business_credit_ledger
		 GROUP BY user_id`,
		ReasonImageGenerationReserve,
		ReasonImageGenerationRefund,
	)
	if err != nil {
		return nil, err
	}
	defer spentRows.Close()

	for spentRows.Next() {
		var userID string
		var spent int64
		if err := spentRows.Scan(&userID, &spent); err != nil {
			return nil, err
		}
		summary, ok := summaries[userID]
		if !ok {
			continue
		}
		if spent < 0 {
			spent = 0
		}
		summary.Spent = spent
		summaries[userID] = summary
	}
	if err := spentRows.Err(); err != nil {
		return nil, err
	}

	return summaries, nil
}

func (s *Store) SetBalance(ctx context.Context, userID string, balance int64, reason string) (Summary, LedgerEntry, error) {
	userID = cleanUserID(userID)
	reason = normalizeReason(reason, ReasonAdminAdjustment)
	if userID == "" {
		return Summary{}, LedgerEntry{}, fmt.Errorf("user id is required")
	}
	if balance < 0 {
		return Summary{}, LedgerEntry{}, fmt.Errorf("balance must be non-negative")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	defer tx.Rollback()

	current, err := balanceForUpdate(ctx, tx, userID)
	if err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	delta := balance - current
	entry, err := writeBalanceDelta(ctx, tx, userID, delta, reason, "", balance)
	if err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	summary, err := s.Summary(ctx, userID)
	if err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	return summary, entry, nil
}

func (s *Store) Reserve(ctx context.Context, userID string, amount int64, generationID string) (Summary, LedgerEntry, error) {
	return s.Add(ctx, userID, -amount, ReasonImageGenerationReserve, generationID, true)
}

func (s *Store) Refund(ctx context.Context, userID string, amount int64, generationID string) (Summary, LedgerEntry, error) {
	return s.Add(ctx, userID, amount, ReasonImageGenerationRefund, generationID, false)
}

func (s *Store) GenerationTotals(ctx context.Context, userID string, generationID string) (GenerationTotals, error) {
	userID = cleanUserID(userID)
	generationID = strings.TrimSpace(generationID)
	if userID == "" || generationID == "" {
		return GenerationTotals{}, nil
	}
	var totals GenerationTotals
	err := s.db.QueryRowContext(
		ctx,
		`SELECT
		    COALESCE(SUM(CASE WHEN reason = ? THEN -delta ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN reason = ? THEN delta ELSE 0 END), 0)
		   FROM business_credit_ledger
		  WHERE user_id = ? AND generation_id = ?`,
		ReasonImageGenerationReserve,
		ReasonImageGenerationRefund,
		userID,
		generationID,
	).Scan(&totals.Reserved, &totals.Refunded)
	if err != nil {
		return GenerationTotals{}, err
	}
	if totals.Reserved < 0 {
		totals.Reserved = 0
	}
	if totals.Refunded < 0 {
		totals.Refunded = 0
	}
	return totals, nil
}

func (s *Store) Add(ctx context.Context, userID string, delta int64, reason string, generationID string, requireSufficient bool) (Summary, LedgerEntry, error) {
	userID = cleanUserID(userID)
	reason = normalizeReason(reason, ReasonAdminAdjustment)
	generationID = strings.TrimSpace(generationID)
	if userID == "" {
		return Summary{}, LedgerEntry{}, fmt.Errorf("user id is required")
	}
	if delta == 0 {
		summary, err := s.Summary(ctx, userID)
		return summary, LedgerEntry{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	defer tx.Rollback()

	current, err := balanceForUpdate(ctx, tx, userID)
	if err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	next := current + delta
	if next < 0 && requireSufficient {
		return Summary{}, LedgerEntry{}, ErrInsufficientBalance
	}
	if next < 0 {
		return Summary{}, LedgerEntry{}, fmt.Errorf("balance cannot be negative")
	}

	entry, err := writeBalanceDelta(ctx, tx, userID, delta, reason, generationID, next)
	if err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	summary, err := s.Summary(ctx, userID)
	if err != nil {
		return Summary{}, LedgerEntry{}, err
	}
	return summary, entry, nil
}

func (s *Store) LedgerEntries(ctx context.Context, userID string, limit int) ([]LedgerEntry, error) {
	items, _, err := s.LedgerEntriesPage(ctx, userID, limit, 0)
	return items, err
}

func (s *Store) DeleteUserCreditData(ctx context.Context, userID string) error {
	userID = cleanUserID(userID)
	if userID == "" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM business_user_credits WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM business_credit_ledger WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LedgerEntriesPage(ctx context.Context, userID string, limit int, offset int) ([]LedgerEntry, int64, error) {
	userID = cleanUserID(userID)
	if userID == "" {
		return nil, 0, fmt.Errorf("user id is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM business_credit_ledger
		 WHERE user_id = ?`,
		userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, user_id, delta, balance_after, reason, generation_id, created_at
		 FROM business_credit_ledger
		 WHERE user_id = ?
		 ORDER BY created_at DESC, id DESC
		 LIMIT ? OFFSET ?`,
		userID,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []LedgerEntry{}
	for rows.Next() {
		var item LedgerEntry
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Delta,
			&item.BalanceAfter,
			&item.Reason,
			&item.GenerationID,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func balanceForUpdate(ctx context.Context, tx *sql.Tx, userID string) (int64, error) {
	var balance int64
	err := tx.QueryRowContext(
		ctx,
		`SELECT balance
		 FROM business_user_credits
		 WHERE user_id = ?`,
		userID,
	).Scan(&balance)
	if err == sql.ErrNoRows {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO business_user_credits(user_id, balance, updated_at)
			 VALUES(?, ?, ?)`,
			userID,
			0,
			now,
		); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func writeBalanceDelta(ctx context.Context, tx *sql.Tx, userID string, delta int64, reason string, generationID string, balanceAfter int64) (LedgerEntry, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	entry := LedgerEntry{
		ID:           newLedgerID(),
		UserID:       userID,
		Delta:        delta,
		BalanceAfter: balanceAfter,
		Reason:       reason,
		GenerationID: generationID,
		CreatedAt:    now,
	}
	_, err := tx.ExecContext(
		ctx,
		`UPDATE business_user_credits
		 SET balance = ?, updated_at = ?
		 WHERE user_id = ?`,
		balanceAfter,
		now,
		userID,
	)
	if err != nil {
		return LedgerEntry{}, err
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO business_credit_ledger(
			id, user_id, delta, balance_after, reason, generation_id, created_at
		) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.UserID,
		entry.Delta,
		entry.BalanceAfter,
		entry.Reason,
		entry.GenerationID,
		entry.CreatedAt,
	)
	if err != nil {
		return LedgerEntry{}, err
	}
	return entry, nil
}

func normalizeReason(value string, fallback string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return fallback
	}
	return normalized
}

func cleanUserID(value string) string {
	return strings.TrimSpace(value)
}

func newLedgerID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("ledger_%d", time.Now().UnixNano())
	}
	return "ledger_" + hex.EncodeToString(raw[:])
}
