package businesstracker

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
	"imagestudio/internal/sqlitedb"
)

const (
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

type Record struct {
	ID                 string `json:"id"`
	UserID             string `json:"userId,omitempty"`
	ConversationID     string `json:"conversationId,omitempty"`
	GenerationID       string `json:"generationId,omitempty"`
	TurnID             string `json:"turnId,omitempty"`
	Platform           string `json:"platform,omitempty"`
	ProviderID         string `json:"providerId,omitempty"`
	ProviderName       string `json:"providerName,omitempty"`
	Model              string `json:"model,omitempty"`
	Status             string `json:"status"`
	Stage              string `json:"stage,omitempty"`
	ErrorCode          string `json:"errorCode,omitempty"`
	ErrorMessage       string `json:"errorMessage,omitempty"`
	RequestedCount     int    `json:"requestedCount"`
	ActualCount        int    `json:"actualCount"`
	QueueWaitMS        int64  `json:"queueWaitMs"`
	UpstreamDurationMS int64  `json:"upstreamDurationMs"`
	PersistDurationMS  int64  `json:"persistDurationMs"`
	TotalDurationMS    int64  `json:"totalDurationMs"`
	CreditReserved     int64  `json:"creditReserved"`
	CreditRefunded     int64  `json:"creditRefunded"`
	StorageBytes       int64  `json:"storageBytes"`
	CreatedAt          string `json:"createdAt"`
	AdmittedAt         string `json:"admittedAt,omitempty"`
	UpstreamStartedAt  string `json:"upstreamStartedAt,omitempty"`
	UpstreamFinishedAt string `json:"upstreamFinishedAt,omitempty"`
	FinishedAt         string `json:"finishedAt,omitempty"`
}

type PlatformSummary struct {
	Platform           string  `json:"platform"`
	Total              int64   `json:"total"`
	Succeeded          int64   `json:"succeeded"`
	Failed             int64   `json:"failed"`
	SuccessRate        float64 `json:"successRate"`
	AvgUpstreamMS      int64   `json:"avgUpstreamMs"`
	P95UpstreamMS      int64   `json:"p95UpstreamMs"`
	LastErrorCode      string  `json:"lastErrorCode,omitempty"`
	LastErrorMessage   string  `json:"lastErrorMessage,omitempty"`
	LastErrorCreatedAt string  `json:"lastErrorCreatedAt,omitempty"`
}

type Summary struct {
	WindowSeconds  int64             `json:"windowSeconds"`
	WindowStart    string            `json:"windowStart"`
	Total          int64             `json:"total"`
	Succeeded      int64             `json:"succeeded"`
	Failed         int64             `json:"failed"`
	SuccessRate    float64           `json:"successRate"`
	RequestedCount int64             `json:"requestedCount"`
	ActualCount    int64             `json:"actualCount"`
	AvgTotalMS     int64             `json:"avgTotalMs"`
	P95TotalMS     int64             `json:"p95TotalMs"`
	AvgQueueMS     int64             `json:"avgQueueMs"`
	AvgUpstreamMS  int64             `json:"avgUpstreamMs"`
	P95UpstreamMS  int64             `json:"p95UpstreamMs"`
	StorageBytes   int64             `json:"storageBytes"`
	Platforms      []PlatformSummary `json:"platforms"`
	RecentFailures []Record          `json:"recentFailures"`
}

type Store struct {
	db     *sql.DB
	driver string
	ownDB  bool
}

func NewStore(cfg *config.Config) (*Store, error) {
	if database.IsPostgres(cfg.Database.Driver) {
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
	rawPath := strings.TrimSpace(cfg.Storage.SQLitePath)
	if rawPath == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	db, err := sqlitedb.Open(cfg.ResolvePath(rawPath))
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, driver: "sqlite", ownDB: true}
	if err := store.init(); err != nil {
		_ = store.Close()
		return nil, err
	}
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
	if s == nil || s.db == nil {
		return nil
	}
	if !s.ownDB {
		return nil
	}
	return s.db.Close()
}

func (s *Store) DeleteUserRecords(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_image_tracker WHERE user_id = ?`), userID)
	return err
}

func (s *Store) init() error {
	if s.isPostgres() {
		return nil
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS business_image_tracker (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			generation_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			provider_name TEXT NOT NULL,
			model TEXT NOT NULL,
			status TEXT NOT NULL,
			stage TEXT NOT NULL,
			error_code TEXT NOT NULL,
			error_message TEXT NOT NULL,
			requested_count INTEGER NOT NULL,
			actual_count INTEGER NOT NULL,
			queue_wait_ms INTEGER NOT NULL,
			upstream_duration_ms INTEGER NOT NULL,
			persist_duration_ms INTEGER NOT NULL,
			total_duration_ms INTEGER NOT NULL,
			credit_reserved INTEGER NOT NULL,
			credit_refunded INTEGER NOT NULL,
			storage_bytes INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			admitted_at TEXT NOT NULL,
			upstream_started_at TEXT NOT NULL,
			upstream_finished_at TEXT NOT NULL,
			finished_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_tracker_created
			ON business_image_tracker(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_tracker_platform_created
			ON business_image_tracker(platform, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_tracker_status_created
			ON business_image_tracker(status, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_tracker_user_created
			ON business_image_tracker(user_id, created_at DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func NewRecordID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "track_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("track_%d", time.Now().UnixNano())
}

func (s *Store) Save(ctx context.Context, record Record) (Record, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.ID = clean(record.ID)
	if record.ID == "" {
		record.ID = NewRecordID()
	}
	record.UserID = strings.TrimSpace(record.UserID)
	record.ConversationID = strings.TrimSpace(record.ConversationID)
	record.GenerationID = strings.TrimSpace(record.GenerationID)
	record.TurnID = strings.TrimSpace(record.TurnID)
	record.Platform = strings.TrimSpace(record.Platform)
	record.ProviderID = strings.TrimSpace(record.ProviderID)
	record.ProviderName = strings.TrimSpace(record.ProviderName)
	record.Model = strings.TrimSpace(record.Model)
	record.Status = normalizeStatus(record.Status)
	record.Stage = strings.TrimSpace(record.Stage)
	record.ErrorCode = strings.TrimSpace(record.ErrorCode)
	record.ErrorMessage = strings.TrimSpace(record.ErrorMessage)
	record.CreatedAt = firstNonEmpty(record.CreatedAt, now)
	record.FinishedAt = firstNonEmpty(record.FinishedAt, now)
	if record.RequestedCount < 0 {
		record.RequestedCount = 0
	}
	if record.ActualCount < 0 {
		record.ActualCount = 0
	}

	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_image_tracker(
			id, user_id, conversation_id, generation_id, turn_id, platform,
			provider_id, provider_name, model, status, stage, error_code,
			error_message, requested_count, actual_count, queue_wait_ms,
			upstream_duration_ms, persist_duration_ms, total_duration_ms,
			credit_reserved, credit_refunded, storage_bytes, created_at,
			admitted_at, upstream_started_at, upstream_finished_at, finished_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			conversation_id = excluded.conversation_id,
			generation_id = excluded.generation_id,
			turn_id = excluded.turn_id,
			platform = excluded.platform,
			provider_id = excluded.provider_id,
			provider_name = excluded.provider_name,
			model = excluded.model,
			status = excluded.status,
			stage = excluded.stage,
			error_code = excluded.error_code,
			error_message = excluded.error_message,
			requested_count = excluded.requested_count,
			actual_count = excluded.actual_count,
			queue_wait_ms = excluded.queue_wait_ms,
			upstream_duration_ms = excluded.upstream_duration_ms,
			persist_duration_ms = excluded.persist_duration_ms,
			total_duration_ms = excluded.total_duration_ms,
			credit_reserved = excluded.credit_reserved,
			credit_refunded = excluded.credit_refunded,
			storage_bytes = excluded.storage_bytes,
			admitted_at = excluded.admitted_at,
			upstream_started_at = excluded.upstream_started_at,
			upstream_finished_at = excluded.upstream_finished_at,
			finished_at = excluded.finished_at`),
		record.ID,
		record.UserID,
		record.ConversationID,
		record.GenerationID,
		record.TurnID,
		record.Platform,
		record.ProviderID,
		record.ProviderName,
		record.Model,
		record.Status,
		record.Stage,
		record.ErrorCode,
		record.ErrorMessage,
		record.RequestedCount,
		record.ActualCount,
		record.QueueWaitMS,
		record.UpstreamDurationMS,
		record.PersistDurationMS,
		record.TotalDurationMS,
		record.CreditReserved,
		record.CreditRefunded,
		record.StorageBytes,
		record.CreatedAt,
		record.AdmittedAt,
		record.UpstreamStartedAt,
		record.UpstreamFinishedAt,
		record.FinishedAt,
	)
	return record, err
}

func (s *Store) Summary(ctx context.Context, windowSeconds int64) (Summary, error) {
	if windowSeconds <= 0 {
		windowSeconds = 600
	}
	if windowSeconds > 86400 {
		windowSeconds = 86400
	}
	windowStart := time.Now().UTC().Add(-time.Duration(windowSeconds) * time.Second).Format(time.RFC3339Nano)
	summary := Summary{
		WindowSeconds:  windowSeconds,
		WindowStart:    windowStart,
		Platforms:      []PlatformSummary{},
		RecentFailures: []Record{},
	}

	var avgTotalMS float64
	var avgQueueMS float64
	var avgUpstreamMS float64
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN status != ? THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(requested_count), 0),
		        COALESCE(SUM(actual_count), 0),
		        COALESCE(AVG(NULLIF(total_duration_ms, 0)), 0),
		        COALESCE(AVG(queue_wait_ms), 0),
		        COALESCE(AVG(NULLIF(upstream_duration_ms, 0)), 0),
		        COALESCE(SUM(storage_bytes), 0)
		 FROM business_image_tracker
		 WHERE created_at >= ?`),
		StatusSucceeded,
		StatusSucceeded,
		windowStart,
	).Scan(
		&summary.Total,
		&summary.Succeeded,
		&summary.Failed,
		&summary.RequestedCount,
		&summary.ActualCount,
		&avgTotalMS,
		&avgQueueMS,
		&avgUpstreamMS,
		&summary.StorageBytes,
	)
	if err != nil {
		return Summary{}, err
	}
	summary.AvgTotalMS = int64(math.Round(avgTotalMS))
	summary.AvgQueueMS = int64(math.Round(avgQueueMS))
	summary.AvgUpstreamMS = int64(math.Round(avgUpstreamMS))
	summary.SuccessRate = successRate(summary.Succeeded, summary.Total)

	totalDurations, err := s.durationValues(ctx, "total_duration_ms", windowStart, "")
	if err != nil {
		return Summary{}, err
	}
	summary.P95TotalMS = percentile(totalDurations, 0.95)
	upstreamDurations, err := s.durationValues(ctx, "upstream_duration_ms", windowStart, "")
	if err != nil {
		return Summary{}, err
	}
	summary.P95UpstreamMS = percentile(upstreamDurations, 0.95)

	platforms, err := s.platformSummary(ctx, windowStart)
	if err != nil {
		return Summary{}, err
	}
	summary.Platforms = platforms

	failures, err := s.recentFailures(ctx, windowStart, 10)
	if err != nil {
		return Summary{}, err
	}
	summary.RecentFailures = failures
	return summary, nil
}

func (s *Store) platformSummary(ctx context.Context, windowStart string) ([]PlatformSummary, error) {
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT platform,
		        COUNT(*),
		        COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN status != ? THEN 1 ELSE 0 END), 0),
		        COALESCE(AVG(NULLIF(upstream_duration_ms, 0)), 0)
		 FROM business_image_tracker
		 WHERE created_at >= ?
		 GROUP BY platform
		 ORDER BY COUNT(*) DESC, platform ASC`),
		StatusSucceeded,
		StatusSucceeded,
		windowStart,
	)
	if err != nil {
		return nil, err
	}

	items := []PlatformSummary{}
	for rows.Next() {
		var item PlatformSummary
		var avgUpstreamMS float64
		if err := rows.Scan(
			&item.Platform,
			&item.Total,
			&item.Succeeded,
			&item.Failed,
			&avgUpstreamMS,
		); err != nil {
			return nil, err
		}
		item.AvgUpstreamMS = int64(math.Round(avgUpstreamMS))
		item.SuccessRate = successRate(item.Succeeded, item.Total)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	for index := range items {
		durations, err := s.durationValues(ctx, "upstream_duration_ms", windowStart, items[index].Platform)
		if err != nil {
			return nil, err
		}
		items[index].P95UpstreamMS = percentile(durations, 0.95)
		if err := s.fillLastPlatformError(ctx, &items[index], windowStart); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) fillLastPlatformError(ctx context.Context, item *PlatformSummary, windowStart string) error {
	if item == nil || strings.TrimSpace(item.Platform) == "" {
		return nil
	}
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT error_code, error_message, created_at
		 FROM business_image_tracker
		 WHERE created_at >= ? AND platform = ? AND status != ?
		 ORDER BY created_at DESC
		 LIMIT 1`),
		windowStart,
		item.Platform,
		StatusSucceeded,
	).Scan(&item.LastErrorCode, &item.LastErrorMessage, &item.LastErrorCreatedAt)
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}

func (s *Store) durationValues(ctx context.Context, column string, windowStart string, platform string) ([]int64, error) {
	if column != "total_duration_ms" && column != "upstream_duration_ms" {
		return nil, fmt.Errorf("unsupported duration column %q", column)
	}
	args := []any{windowStart}
	query := `SELECT ` + column + `
		FROM business_image_tracker
		WHERE created_at >= ? AND ` + column + ` > 0`
	if strings.TrimSpace(platform) != "" {
		query += ` AND platform = ?`
		args = append(args, platform)
	}
	query += ` ORDER BY ` + column + ` ASC`

	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []int64{}
	for rows.Next() {
		var value int64
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *Store) recentFailures(ctx context.Context, windowStart string, limit int) ([]Record, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT id, user_id, conversation_id, generation_id, turn_id, platform,
		        provider_id, provider_name, model, status, stage, error_code,
		        error_message, requested_count, actual_count, queue_wait_ms,
		        upstream_duration_ms, persist_duration_ms, total_duration_ms,
		        credit_reserved, credit_refunded, storage_bytes, created_at,
		        admitted_at, upstream_started_at, upstream_finished_at, finished_at
		 FROM business_image_tracker
		 WHERE created_at >= ? AND status != ?
		 ORDER BY created_at DESC
		 LIMIT ?`),
		windowStart,
		StatusSucceeded,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Record{}
	for rows.Next() {
		item, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func scanRecord(scanner interface {
	Scan(dest ...any) error
}) (Record, error) {
	var item Record
	err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.ConversationID,
		&item.GenerationID,
		&item.TurnID,
		&item.Platform,
		&item.ProviderID,
		&item.ProviderName,
		&item.Model,
		&item.Status,
		&item.Stage,
		&item.ErrorCode,
		&item.ErrorMessage,
		&item.RequestedCount,
		&item.ActualCount,
		&item.QueueWaitMS,
		&item.UpstreamDurationMS,
		&item.PersistDurationMS,
		&item.TotalDurationMS,
		&item.CreditReserved,
		&item.CreditRefunded,
		&item.StorageBytes,
		&item.CreatedAt,
		&item.AdmittedAt,
		&item.UpstreamStartedAt,
		&item.UpstreamFinishedAt,
		&item.FinishedAt,
	)
	return item, err
}

func (s *Store) isPostgres() bool {
	return database.IsPostgres(s.driver)
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}

func percentile(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	index := int(math.Ceil(float64(len(values))*p)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func successRate(succeeded int64, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round((float64(succeeded)/float64(total))*1000) / 10
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case StatusSucceeded:
		return StatusSucceeded
	default:
		return StatusFailed
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clean(value string) string {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.ReplaceAll(cleaned, "\x00", "")
	return cleaned
}
