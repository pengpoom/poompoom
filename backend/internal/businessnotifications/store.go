package businessnotifications

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

const (
	LevelInfo    = "info"
	LevelWarning = "warning"
	LevelSuccess = "success"

	AudienceAll = "all"

	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"

	NotifyModeSilent = "silent"
	NotifyModePopup  = "popup"

	TargetModeAll     = "all"
	TargetModeBalance = "balance"

	userStatusActive = "active"
)

type Targeting struct {
	Mode    string         `json:"mode"`
	Balance *BalanceTarget `json:"balance,omitempty"`
}

type BalanceTarget struct {
	Operator string `json:"operator"`
	Value    int64  `json:"value"`
}

type Notification struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	Level         string    `json:"level"`
	Audience      string    `json:"audience"`
	Status        string    `json:"status"`
	CreatedBy     string    `json:"createdBy"`
	CreatedAt     string    `json:"createdAt"`
	UpdatedAt     string    `json:"updatedAt"`
	PublishedAt   string    `json:"publishedAt,omitempty"`
	ReadAt        string    `json:"readAt,omitempty"`
	ReadCount     int64     `json:"readCount,omitempty"`
	AudienceCount int64     `json:"audienceCount,omitempty"`
	NotifyMode    string    `json:"notifyMode"`
	StartsAt      string    `json:"startsAt,omitempty"`
	EndsAt        string    `json:"endsAt,omitempty"`
	Targeting     Targeting `json:"targeting"`
}

type MutationInput struct {
	Title      string
	Body       string
	Level      string
	Audience   string
	Publish    bool
	Status     string
	NotifyMode string
	StartsAt   string
	EndsAt     string
	Targeting  Targeting
	CreatedBy  string
}

type AdminListFilters struct {
	Status string
	Search string
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

func (s *Store) ListAdmin(ctx context.Context, limit int, filters ...AdminListFilters) ([]Notification, error) {
	filter := AdminListFilters{}
	if len(filters) > 0 {
		filter = filters[0]
	}
	limit = normalizeLimit(limit)
	query := `SELECT n.id, n.title, n.body, n.level, n.audience, n.status, n.created_by, n.created_at, n.updated_at, n.published_at, '',
			0, 0, n.notify_mode, n.starts_at, n.ends_at, n.targeting
		FROM business_notifications n
		WHERE 1 = 1`
	args := []any{}
	if status := normalizeStatus(filter.Status); status != "" {
		query += ` AND n.status = ?`
		args = append(args, status)
	}
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	if search != "" {
		searchRunes := []rune(search)
		if len(searchRunes) > 200 {
			search = string(searchRunes[:200])
		}
		pattern := "%" + search + "%"
		query += ` AND (LOWER(n.title) LIKE ? OR LOWER(n.body) LIKE ?)`
		args = append(args, pattern, pattern)
	}
	query += `
		ORDER BY n.created_at DESC
		LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanNotifications(rows)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if err := s.fillAdminStats(ctx, &items[index]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) GetAdmin(ctx context.Context, id string) (Notification, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Notification{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, s.rebind(`SELECT n.id, n.title, n.body, n.level, n.audience, n.status, n.created_by, n.created_at, n.updated_at, n.published_at, '',
			0, 0, n.notify_mode, n.starts_at, n.ends_at, n.targeting
		FROM business_notifications n
		WHERE n.id = ?`), id)
	item, err := scanNotification(row)
	if err == sql.ErrNoRows {
		return Notification{}, false, nil
	}
	if err != nil {
		return Notification{}, false, err
	}
	if err := s.fillAdminStats(ctx, &item); err != nil {
		return Notification{}, false, err
	}
	return item, true, nil
}

func (s *Store) ListForUser(ctx context.Context, userID string, limit int) ([]Notification, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return []Notification{}, nil
	}
	limit = normalizeLimit(limit)
	balance, err := s.userBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT n.id, n.title, n.body, n.level, n.audience, n.status, n.created_by, n.created_at, n.updated_at, n.published_at, COALESCE(r.read_at, ''), 0, 0, n.notify_mode, n.starts_at, n.ends_at, n.targeting
		FROM business_notifications n
		LEFT JOIN business_notification_reads r ON r.notification_id = n.id AND r.user_id = ?
		WHERE n.status = ? AND n.audience = ?
			AND COALESCE(NULLIF(TRIM(n.starts_at), '')::timestamptz <= ?::timestamptz, true)
			AND COALESCE(NULLIF(TRIM(n.ends_at), '')::timestamptz > ?::timestamptz, true)
		ORDER BY n.published_at DESC, n.created_at DESC`), userID, StatusPublished, AudienceAll, now, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	allItems, err := scanNotifications(rows)
	if err != nil {
		return nil, err
	}
	items := make([]Notification, 0, minInt(limit, len(allItems)))
	for _, item := range allItems {
		if !matchesTargeting(item.Targeting, balance) {
			continue
		}
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (s *Store) UnreadCount(ctx context.Context, userID string) (int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, nil
	}
	balance, err := s.userBalance(ctx, userID)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT n.id, n.title, n.body, n.level, n.audience, n.status, n.created_by, n.created_at, n.updated_at, n.published_at, '', 0, 0, n.notify_mode, n.starts_at, n.ends_at, n.targeting
		FROM business_notifications n
		LEFT JOIN business_notification_reads r ON r.notification_id = n.id AND r.user_id = ?
		WHERE n.status = ? AND n.audience = ? AND r.notification_id IS NULL
			AND COALESCE(NULLIF(TRIM(n.starts_at), '')::timestamptz <= ?::timestamptz, true)
			AND COALESCE(NULLIF(TRIM(n.ends_at), '')::timestamptz > ?::timestamptz, true)`), userID, StatusPublished, AudienceAll, now, now)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	items, err := scanNotifications(rows)
	if err != nil {
		return 0, err
	}
	var count int64
	for _, item := range items {
		if matchesTargeting(item.Targeting, balance) {
			count++
		}
	}
	return count, nil
}

func (s *Store) Create(ctx context.Context, input MutationInput) (Notification, error) {
	item, err := normalizeInput(input)
	if err != nil {
		return Notification{}, err
	}
	item.ID = newNotificationID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	item.CreatedAt = now
	item.UpdatedAt = now
	if item.Status == StatusPublished {
		item.PublishedAt = now
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_notifications(id, title, body, level, audience, status, created_by, created_at, updated_at, published_at, notify_mode, starts_at, ends_at, targeting)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		item.ID,
		item.Title,
		item.Body,
		item.Level,
		item.Audience,
		item.Status,
		item.CreatedBy,
		item.CreatedAt,
		item.UpdatedAt,
		item.PublishedAt,
		item.NotifyMode,
		item.StartsAt,
		item.EndsAt,
		encodeTargeting(item.Targeting),
	)
	if err != nil {
		return Notification{}, err
	}
	created, ok, err := s.GetAdmin(ctx, item.ID)
	if err != nil || !ok {
		return item, err
	}
	return created, nil
}

func (s *Store) Update(ctx context.Context, id string, input MutationInput) (Notification, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Notification{}, false, nil
	}
	item, err := normalizeInput(input)
	if err != nil {
		return Notification{}, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	publishedAt := ""
	if item.Status == StatusPublished {
		publishedAt = now
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_notifications
		SET title = ?, body = ?, level = ?, audience = ?, status = ?, notify_mode = ?, starts_at = ?, ends_at = ?, targeting = ?, updated_at = ?,
			published_at = CASE
				WHEN ? = ? AND TRIM(published_at) = '' THEN ?
				WHEN ? = ? THEN published_at
				ELSE ''
			END
		WHERE id = ?`),
		item.Title,
		item.Body,
		item.Level,
		item.Audience,
		item.Status,
		item.NotifyMode,
		item.StartsAt,
		item.EndsAt,
		encodeTargeting(item.Targeting),
		now,
		item.Status,
		StatusPublished,
		publishedAt,
		item.Status,
		StatusPublished,
		id,
	)
	if err != nil {
		return Notification{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Notification{}, false, err
	}
	if affected == 0 {
		return Notification{}, false, nil
	}
	updated, ok, err := s.GetAdmin(ctx, id)
	return updated, ok, err
}

func (s *Store) Delete(ctx context.Context, id string) (bool, error) {
	id = cleanID(id)
	if id == "" {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_notifications WHERE id = ?`), id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) MarkRead(ctx context.Context, userID string, ids []string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	cleaned := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = cleanID(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	if len(cleaned) == 0 {
		return nil
	}
	readAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range cleaned {
		if _, err := s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_notification_reads(notification_id, user_id, read_at)
			VALUES(?, ?, ?)
			ON CONFLICT(notification_id, user_id) DO UPDATE SET read_at = EXCLUDED.read_at`), id, userID, readAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) fillAdminStats(ctx context.Context, item *Notification) error {
	audienceCount, err := s.countAudience(ctx, item.Targeting)
	if err != nil {
		return err
	}
	readCount, err := s.countReads(ctx, item.ID, item.Targeting)
	if err != nil {
		return err
	}
	item.AudienceCount = audienceCount
	item.ReadCount = readCount
	return nil
}

func (s *Store) countAudience(ctx context.Context, targeting Targeting) (int64, error) {
	query := `SELECT COUNT(*)
		FROM business_users u
		LEFT JOIN business_user_credits c ON c.user_id = u.id
		WHERE u.status = ? AND u.deleted_at IS NULL`
	args := []any{userStatusActive}
	clause, clauseArgs := targetingSQLClause(targeting, "c.balance")
	query += clause
	args = append(args, clauseArgs...)
	var count int64
	err := s.db.QueryRowContext(ctx, s.rebind(query), args...).Scan(&count)
	return count, err
}

func (s *Store) countReads(ctx context.Context, notificationID string, targeting Targeting) (int64, error) {
	query := `SELECT COUNT(DISTINCT r.user_id)
		FROM business_notification_reads r
		JOIN business_users u ON u.id = r.user_id
		LEFT JOIN business_user_credits c ON c.user_id = u.id
		WHERE r.notification_id = ? AND u.status = ? AND u.deleted_at IS NULL`
	args := []any{notificationID, userStatusActive}
	clause, clauseArgs := targetingSQLClause(targeting, "c.balance")
	query += clause
	args = append(args, clauseArgs...)
	var count int64
	err := s.db.QueryRowContext(ctx, s.rebind(query), args...).Scan(&count)
	return count, err
}

func (s *Store) userBalance(ctx context.Context, userID string) (int64, error) {
	var balance int64
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COALESCE(balance, 0)
		FROM business_user_credits
		WHERE user_id = ?`), userID).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return balance, err
}

func scanNotifications(rows *sql.Rows) ([]Notification, error) {
	items := make([]Notification, 0)
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner) (Notification, error) {
	var item Notification
	var targeting string
	err := row.Scan(
		&item.ID,
		&item.Title,
		&item.Body,
		&item.Level,
		&item.Audience,
		&item.Status,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.PublishedAt,
		&item.ReadAt,
		&item.ReadCount,
		&item.AudienceCount,
		&item.NotifyMode,
		&item.StartsAt,
		&item.EndsAt,
		&targeting,
	)
	item.Targeting = decodeTargeting(targeting)
	return item, err
}

func normalizeInput(input MutationInput) (Notification, error) {
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if title == "" {
		return Notification{}, fmt.Errorf("title is required")
	}
	if body == "" {
		return Notification{}, fmt.Errorf("body is required")
	}
	if len([]rune(title)) > 80 {
		return Notification{}, fmt.Errorf("title is too long")
	}
	if len([]rune(body)) > 1200 {
		return Notification{}, fmt.Errorf("body is too long")
	}
	startsAt, err := normalizeOptionalTime(input.StartsAt)
	if err != nil {
		return Notification{}, fmt.Errorf("starts_at is invalid")
	}
	endsAt, err := normalizeOptionalTime(input.EndsAt)
	if err != nil {
		return Notification{}, fmt.Errorf("ends_at is invalid")
	}
	if startsAt != "" && endsAt != "" {
		startTime, _ := time.Parse(time.RFC3339Nano, startsAt)
		endTime, _ := time.Parse(time.RFC3339Nano, endsAt)
		if !startTime.Before(endTime) {
			return Notification{}, fmt.Errorf("starts_at must be before ends_at")
		}
	}
	level := normalizeLevel(input.Level)
	statusInput := strings.TrimSpace(input.Status)
	status := normalizeStatus(statusInput)
	if statusInput != "" && status == "" {
		return Notification{}, fmt.Errorf("status must be draft, published or archived")
	}
	if status == "" {
		status = StatusDraft
		if input.Publish {
			status = StatusPublished
		}
	}
	targeting, err := normalizeTargeting(input.Targeting)
	if err != nil {
		return Notification{}, err
	}
	return Notification{
		Title:      title,
		Body:       body,
		Level:      level,
		Audience:   AudienceAll,
		Status:     status,
		NotifyMode: normalizeNotifyMode(input.NotifyMode),
		StartsAt:   startsAt,
		EndsAt:     endsAt,
		Targeting:  targeting,
		CreatedBy:  strings.TrimSpace(input.CreatedBy),
	}, nil
}

func normalizeLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case LevelWarning:
		return LevelWarning
	case LevelSuccess:
		return LevelSuccess
	default:
		return LevelInfo
	}
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case StatusDraft:
		return StatusDraft
	case StatusPublished:
		return StatusPublished
	case StatusArchived:
		return StatusArchived
	default:
		return ""
	}
}

func normalizeNotifyMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case NotifyModePopup:
		return NotifyModePopup
	default:
		return NotifyModeSilent
	}
}

func normalizeTargeting(value Targeting) (Targeting, error) {
	mode := strings.ToLower(strings.TrimSpace(value.Mode))
	if mode == "" || mode == TargetModeAll {
		return Targeting{Mode: TargetModeAll}, nil
	}
	if mode != TargetModeBalance {
		return Targeting{}, fmt.Errorf("targeting mode must be all or balance")
	}
	if value.Balance == nil {
		return Targeting{}, fmt.Errorf("balance targeting is required")
	}
	operator := normalizeBalanceOperator(value.Balance.Operator)
	if operator == "" {
		return Targeting{}, fmt.Errorf("balance operator must be one of >, >=, <, <=, =")
	}
	if value.Balance.Value < 0 {
		return Targeting{}, fmt.Errorf("balance threshold must be non-negative")
	}
	return Targeting{
		Mode: TargetModeBalance,
		Balance: &BalanceTarget{
			Operator: operator,
			Value:    value.Balance.Value,
		},
	}, nil
}

func normalizeBalanceOperator(value string) string {
	switch strings.TrimSpace(value) {
	case ">", ">=", "<", "<=", "=":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func normalizeOptionalTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return "", err
		}
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func encodeTargeting(value Targeting) string {
	normalized, err := normalizeTargeting(value)
	if err != nil {
		normalized = Targeting{Mode: TargetModeAll}
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return `{"mode":"all"}`
	}
	return string(payload)
}

func decodeTargeting(value string) Targeting {
	value = strings.TrimSpace(value)
	if value == "" {
		return Targeting{Mode: TargetModeAll}
	}
	var targeting Targeting
	if err := json.Unmarshal([]byte(value), &targeting); err != nil {
		return Targeting{Mode: TargetModeAll}
	}
	normalized, err := normalizeTargeting(targeting)
	if err != nil {
		return Targeting{Mode: TargetModeAll}
	}
	return normalized
}

func matchesTargeting(targeting Targeting, balance int64) bool {
	normalized, err := normalizeTargeting(targeting)
	if err != nil || normalized.Mode == TargetModeAll {
		return true
	}
	if normalized.Balance == nil {
		return true
	}
	switch normalized.Balance.Operator {
	case ">":
		return balance > normalized.Balance.Value
	case ">=":
		return balance >= normalized.Balance.Value
	case "<":
		return balance < normalized.Balance.Value
	case "<=":
		return balance <= normalized.Balance.Value
	case "=":
		return balance == normalized.Balance.Value
	default:
		return true
	}
}

func targetingSQLClause(targeting Targeting, balanceExpr string) (string, []any) {
	normalized, err := normalizeTargeting(targeting)
	if err != nil || normalized.Mode != TargetModeBalance || normalized.Balance == nil {
		return "", nil
	}
	return fmt.Sprintf(" AND COALESCE(%s, 0) %s ?", balanceExpr, normalized.Balance.Operator), []any{normalized.Balance.Value}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func cleanID(value string) string {
	return strings.TrimSpace(value)
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func newNotificationID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return "notice_" + hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("notice_%d", time.Now().UnixNano())
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}
