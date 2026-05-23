package businessjobs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
	"imagestudio/internal/sqlitedb"
)

const (
	StatusQueued          = "queued"
	StatusRunning         = "running"
	StatusCancelRequested = "cancel_requested"
	StatusCancelled       = "cancelled"
	StatusSucceeded       = "succeeded"
	StatusFailed          = "failed"
)

const defaultClaimLease = 30 * time.Second

const jobSelectColumns = `id, user_id, conversation_id, generation_id, turn_id, platform,
	provider_id, provider_name, model, prompt, size, quality,
	requested_count, actual_count, status, stage, upstream_sent, error_code,
	error_message, queue_wait_ms, upstream_duration_ms,
	persist_duration_ms, total_duration_ms, storage_bytes,
	credit_reserved, credit_refunded, payload_json, created_at, queued_at,
	started_at, finished_at, updated_at, claimed_by, claimed_at, lease_until,
	attempts, last_error, next_run_at`

type Job struct {
	ID                 string `json:"id"`
	UserID             string `json:"userId"`
	ConversationID     string `json:"conversationId,omitempty"`
	GenerationID       string `json:"generationId,omitempty"`
	TurnID             string `json:"turnId,omitempty"`
	Platform           string `json:"platform,omitempty"`
	ProviderID         string `json:"providerId,omitempty"`
	ProviderName       string `json:"providerName,omitempty"`
	Model              string `json:"model,omitempty"`
	Prompt             string `json:"prompt,omitempty"`
	Size               string `json:"size,omitempty"`
	Quality            string `json:"quality,omitempty"`
	RequestedCount     int    `json:"requestedCount"`
	ActualCount        int    `json:"actualCount"`
	Status             string `json:"status"`
	Stage              string `json:"stage,omitempty"`
	UpstreamSent       bool   `json:"upstreamSent"`
	UpstreamStatus     string `json:"upstreamStatus"`
	ClaimedBy          string `json:"claimedBy,omitempty"`
	ClaimedAt          string `json:"claimedAt,omitempty"`
	LeaseUntil         string `json:"leaseUntil,omitempty"`
	Attempts           int    `json:"attempts"`
	LastError          string `json:"lastError,omitempty"`
	NextRunAt          string `json:"nextRunAt,omitempty"`
	ErrorCode          string `json:"errorCode,omitempty"`
	ErrorMessage       string `json:"errorMessage,omitempty"`
	QueueWaitMS        int64  `json:"queueWaitMs"`
	UpstreamDurationMS int64  `json:"upstreamDurationMs"`
	PersistDurationMS  int64  `json:"persistDurationMs"`
	TotalDurationMS    int64  `json:"totalDurationMs"`
	StorageBytes       int64  `json:"storageBytes"`
	CreditReserved     int64  `json:"creditReserved"`
	CreditRefunded     int64  `json:"creditRefunded"`
	PayloadJSON        []byte `json:"-"`
	CreatedAt          string `json:"createdAt"`
	QueuedAt           string `json:"queuedAt,omitempty"`
	StartedAt          string `json:"startedAt,omitempty"`
	FinishedAt         string `json:"finishedAt,omitempty"`
	UpdatedAt          string `json:"updatedAt"`
}

type Store struct {
	db     *sql.DB
	driver string
	ownDB  bool
}

type ReconcileOptions struct {
	QueuedBefore          time.Time
	RunningBefore         time.Time
	CancelRequestedBefore time.Time
	Now                   time.Time
}

type ReconcileResult struct {
	QueuedFailed             int64 `json:"queuedFailed"`
	RunningFailed            int64 `json:"runningFailed"`
	CancelRequestedCancelled int64 `json:"cancelRequestedCancelled"`
}

type AdminListFilter struct {
	UserID   string
	Status   string
	Platform string
	From     string
	To       string
	Limit    int
	Offset   int
}

type ActiveCounts struct {
	Queued  int64 `json:"queued"`
	Running int64 `json:"running"`
}

func (c ActiveCounts) Total() int64 {
	return c.Queued + c.Running
}

type CapacityLimits struct {
	MaxUserActiveJobs      int
	MaxQueuedJobs          int
	MaxProviderRunningJobs int
}

const (
	CapacityUserActiveLimit     = "image_user_job_limit"
	CapacityQueueFull           = "image_queue_full"
	CapacityProviderRunningFull = "image_provider_running_limit"
)

type CapacityError struct {
	Code    string
	Limit   int
	Current int64
}

func (e *CapacityError) Error() string {
	return strings.TrimSpace(e.Code)
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

func (s *Store) init() error {
	if s.isPostgres() {
		return nil
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS business_image_jobs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			generation_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			provider_name TEXT NOT NULL,
			model TEXT NOT NULL,
			prompt TEXT NOT NULL,
			size TEXT NOT NULL,
			quality TEXT NOT NULL,
			requested_count INTEGER NOT NULL,
			actual_count INTEGER NOT NULL,
			status TEXT NOT NULL,
			stage TEXT NOT NULL,
			upstream_sent INTEGER NOT NULL DEFAULT 0,
			error_code TEXT NOT NULL,
			error_message TEXT NOT NULL,
			queue_wait_ms INTEGER NOT NULL,
			upstream_duration_ms INTEGER NOT NULL,
			persist_duration_ms INTEGER NOT NULL,
			total_duration_ms INTEGER NOT NULL,
			storage_bytes INTEGER NOT NULL,
			credit_reserved INTEGER NOT NULL,
			credit_refunded INTEGER NOT NULL,
			payload_json BLOB,
			created_at TEXT NOT NULL,
			queued_at TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			claimed_by TEXT NOT NULL DEFAULT '',
			claimed_at TEXT NOT NULL DEFAULT '',
			lease_until TEXT NOT NULL DEFAULT '',
			attempts INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			next_run_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_jobs_user_updated
			ON business_image_jobs(user_id, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_jobs_conversation_created
			ON business_image_jobs(user_id, conversation_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_jobs_generation
			ON business_image_jobs(user_id, generation_id);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_jobs_status_updated
			ON business_image_jobs(status, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_jobs_runnable
			ON business_image_jobs(status, next_run_at, lease_until, created_at);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.ensureColumn("business_image_jobs", "payload_json", "BLOB"); err != nil {
		return err
	}
	if err := s.ensureColumn("business_image_jobs", "upstream_sent", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("business_image_jobs", "claimed_by", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("business_image_jobs", "claimed_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("business_image_jobs", "lease_until", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("business_image_jobs", "attempts", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("business_image_jobs", "last_error", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("business_image_jobs", "next_run_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(table string, column string, definition string) error {
	table = strings.TrimSpace(table)
	column = strings.TrimSpace(column)
	definition = strings.TrimSpace(definition)
	if table == "" || column == "" || definition == "" {
		return nil
	}
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func NewJobID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "job_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

func (s *Store) Save(ctx context.Context, job Job) (Job, error) {
	job, err := s.prepareSaveJob(job)
	if err != nil {
		return Job{}, err
	}
	err = s.execSave(ctx, s.db, job)
	return withDerivedFields(job), err
}

func (s *Store) SaveQueuedWithCapacity(ctx context.Context, job Job, limits CapacityLimits) (Job, error) {
	job, err := s.prepareSaveJob(job)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusQueued {
		err = s.execSave(ctx, s.db, job)
		return withDerivedFields(job), err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	if err := s.lockCapacity(ctx, tx); err != nil {
		return Job{}, err
	}
	if err := s.checkQueuedCapacity(ctx, tx, job, limits); err != nil {
		return Job{}, err
	}
	if err := s.execSave(ctx, tx, job); err != nil {
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return withDerivedFields(job), nil
}

func (s *Store) SaveRunningWithProviderCapacity(ctx context.Context, job Job, limits CapacityLimits) (Job, error) {
	job, err := s.prepareSaveJob(job)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusRunning || limits.MaxProviderRunningJobs <= 0 {
		err = s.execSave(ctx, s.db, job)
		return withDerivedFields(job), err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	if err := s.lockCapacity(ctx, tx); err != nil {
		return Job{}, err
	}
	if err := s.checkProviderRunningCapacity(ctx, tx, job, limits.MaxProviderRunningJobs); err != nil {
		return Job{}, err
	}
	if err := s.execSave(ctx, tx, job); err != nil {
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return withDerivedFields(job), nil
}

func (s *Store) prepareSaveJob(job Job) (Job, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	job.ID = clean(job.ID)
	if job.ID == "" {
		job.ID = NewJobID()
	}
	job.UserID = strings.TrimSpace(job.UserID)
	if job.UserID == "" {
		return Job{}, fmt.Errorf("job user id is required")
	}
	job.ConversationID = clean(job.ConversationID)
	job.GenerationID = clean(firstNonEmpty(job.GenerationID, job.ID))
	job.TurnID = clean(job.TurnID)
	job.Platform = strings.TrimSpace(job.Platform)
	job.ProviderID = strings.TrimSpace(job.ProviderID)
	job.ProviderName = strings.TrimSpace(job.ProviderName)
	job.Model = strings.TrimSpace(job.Model)
	job.Prompt = strings.TrimSpace(job.Prompt)
	job.Size = strings.TrimSpace(job.Size)
	job.Quality = strings.TrimSpace(job.Quality)
	job.Status = normalizeStatus(job.Status)
	job.Stage = strings.TrimSpace(job.Stage)
	job.ClaimedBy = strings.TrimSpace(job.ClaimedBy)
	job.ClaimedAt = strings.TrimSpace(job.ClaimedAt)
	job.LeaseUntil = strings.TrimSpace(job.LeaseUntil)
	job.LastError = strings.TrimSpace(job.LastError)
	job.NextRunAt = strings.TrimSpace(job.NextRunAt)
	job.UpstreamSent = HasUpstreamSent(job)
	job.UpstreamStatus = upstreamStatus(job.UpstreamSent)
	job.ErrorCode = strings.TrimSpace(job.ErrorCode)
	job.ErrorMessage = strings.TrimSpace(job.ErrorMessage)
	job.CreatedAt = firstNonEmpty(job.CreatedAt, now)
	job.UpdatedAt = now
	if job.Status == StatusQueued {
		job.QueuedAt = firstNonEmpty(job.QueuedAt, now)
	}
	if job.Status == StatusRunning {
		job.StartedAt = firstNonEmpty(job.StartedAt, now)
	}
	if isFinalStatus(job.Status) {
		job.FinishedAt = firstNonEmpty(job.FinishedAt, now)
	}
	if job.RequestedCount < 0 {
		job.RequestedCount = 0
	}
	if job.ActualCount < 0 {
		job.ActualCount = 0
	}
	if job.Attempts < 0 {
		job.Attempts = 0
	}
	if (job.Status != StatusQueued || job.Stage != "claimed") && job.Status != StatusRunning {
		job.LeaseUntil = ""
	}
	if job.Status != StatusQueued {
		job.NextRunAt = ""
	}
	if isFinalStatus(job.Status) {
		job.LeaseUntil = ""
		job.NextRunAt = ""
		if job.Status == StatusSucceeded {
			job.LastError = ""
		} else if job.LastError == "" {
			job.LastError = firstNonEmpty(job.ErrorMessage, job.ErrorCode)
		}
	}
	return job, nil
}

type jobExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type jobQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Store) lockCapacity(ctx context.Context, tx *sql.Tx) error {
	if !s.isPostgres() {
		return nil
	}
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('business_image_jobs_capacity'))`)
	return err
}

func (s *Store) checkQueuedCapacity(ctx context.Context, q jobQueryer, job Job, limits CapacityLimits) error {
	if limits.MaxUserActiveJobs > 0 {
		counts, err := s.countActiveWith(ctx, q, "user_id = ?", job.UserID)
		if err != nil {
			return err
		}
		if counts.Total() >= int64(limits.MaxUserActiveJobs) {
			return &CapacityError{Code: CapacityUserActiveLimit, Limit: limits.MaxUserActiveJobs, Current: counts.Total()}
		}
	}
	if limits.MaxQueuedJobs > 0 {
		counts, err := s.countActiveWith(ctx, q, "status = ?", StatusQueued)
		if err != nil {
			return err
		}
		if counts.Queued >= int64(limits.MaxQueuedJobs) {
			return &CapacityError{Code: CapacityQueueFull, Limit: limits.MaxQueuedJobs, Current: counts.Queued}
		}
	}
	return nil
}

func (s *Store) checkProviderRunningCapacity(ctx context.Context, q jobQueryer, job Job, limit int) error {
	if limit <= 0 {
		return nil
	}
	whereSQL := ""
	args := []any{}
	if strings.TrimSpace(job.ProviderID) != "" {
		whereSQL = "provider_id = ? AND status = ? AND id != ?"
		args = []any{strings.TrimSpace(job.ProviderID), StatusRunning, job.ID}
	} else if strings.TrimSpace(job.Platform) != "" {
		whereSQL = "platform = ? AND status = ? AND id != ?"
		args = []any{strings.TrimSpace(job.Platform), StatusRunning, job.ID}
	} else {
		return nil
	}
	counts, err := s.countActiveWith(ctx, q, whereSQL, args...)
	if err != nil {
		return err
	}
	if counts.Running >= int64(limit) {
		return &CapacityError{Code: CapacityProviderRunningFull, Limit: limit, Current: counts.Running}
	}
	return nil
}

func (s *Store) execSave(ctx context.Context, exec jobExecer, job Job) error {
	_, err := exec.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_image_jobs(
			id, user_id, conversation_id, generation_id, turn_id, platform,
			provider_id, provider_name, model, prompt, size, quality,
			requested_count, actual_count, status, stage, upstream_sent, error_code,
			error_message, queue_wait_ms, upstream_duration_ms,
			persist_duration_ms, total_duration_ms, storage_bytes,
			credit_reserved, credit_refunded, payload_json, created_at, queued_at,
			started_at, finished_at, updated_at, claimed_by, claimed_at, lease_until,
			attempts, last_error, next_run_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			conversation_id = excluded.conversation_id,
			generation_id = excluded.generation_id,
			turn_id = excluded.turn_id,
			platform = excluded.platform,
			provider_id = excluded.provider_id,
			provider_name = excluded.provider_name,
			model = excluded.model,
			prompt = excluded.prompt,
			size = excluded.size,
			quality = excluded.quality,
			requested_count = excluded.requested_count,
			actual_count = excluded.actual_count,
			status = excluded.status,
			stage = excluded.stage,
			upstream_sent = CASE
				WHEN business_image_jobs.upstream_sent = 1 THEN 1
				ELSE excluded.upstream_sent
			END,
			error_code = excluded.error_code,
			error_message = excluded.error_message,
			queue_wait_ms = excluded.queue_wait_ms,
			upstream_duration_ms = excluded.upstream_duration_ms,
			persist_duration_ms = excluded.persist_duration_ms,
			total_duration_ms = excluded.total_duration_ms,
			storage_bytes = excluded.storage_bytes,
			credit_reserved = excluded.credit_reserved,
			credit_refunded = excluded.credit_refunded,
			payload_json = CASE
				WHEN excluded.payload_json IS NOT NULL AND length(excluded.payload_json) > 0 THEN excluded.payload_json
				ELSE business_image_jobs.payload_json
			END,
			queued_at = excluded.queued_at,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			claimed_by = CASE
				WHEN excluded.claimed_by != '' THEN excluded.claimed_by
				ELSE business_image_jobs.claimed_by
			END,
			claimed_at = CASE
				WHEN excluded.claimed_at != '' THEN excluded.claimed_at
				ELSE business_image_jobs.claimed_at
			END,
			lease_until = excluded.lease_until,
			attempts = CASE
				WHEN excluded.attempts > business_image_jobs.attempts THEN excluded.attempts
				ELSE business_image_jobs.attempts
			END,
			last_error = excluded.last_error,
			next_run_at = excluded.next_run_at,
			updated_at = excluded.updated_at
		 WHERE business_image_jobs.user_id = excluded.user_id`),
		job.ID,
		job.UserID,
		job.ConversationID,
		job.GenerationID,
		job.TurnID,
		job.Platform,
		job.ProviderID,
		job.ProviderName,
		job.Model,
		job.Prompt,
		job.Size,
		job.Quality,
		job.RequestedCount,
		job.ActualCount,
		job.Status,
		job.Stage,
		boolToInt(job.UpstreamSent),
		job.ErrorCode,
		job.ErrorMessage,
		job.QueueWaitMS,
		job.UpstreamDurationMS,
		job.PersistDurationMS,
		job.TotalDurationMS,
		job.StorageBytes,
		job.CreditReserved,
		job.CreditRefunded,
		job.PayloadJSON,
		job.CreatedAt,
		job.QueuedAt,
		job.StartedAt,
		job.FinishedAt,
		job.UpdatedAt,
		job.ClaimedBy,
		job.ClaimedAt,
		job.LeaseUntil,
		job.Attempts,
		job.LastError,
		job.NextRunAt,
	)
	return err
}

func (s *Store) Get(ctx context.Context, id string, userID string) (Job, bool, error) {
	id = clean(id)
	userID = strings.TrimSpace(userID)
	if id == "" || userID == "" {
		return Job{}, false, nil
	}
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT `+jobSelectColumns+`
		   FROM business_image_jobs
		  WHERE id = ? AND user_id = ?`),
		id,
		userID,
	)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func (s *Store) GetByID(ctx context.Context, id string) (Job, bool, error) {
	id = clean(id)
	if id == "" {
		return Job{}, false, nil
	}
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT `+jobSelectColumns+`
		   FROM business_image_jobs
		  WHERE id = ?`),
		id,
	)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func (s *Store) List(ctx context.Context, userID string, conversationID string, limit int) ([]Job, error) {
	userID = strings.TrimSpace(userID)
	conversationID = clean(conversationID)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if conversationID != "" {
		rows, err = s.db.QueryContext(
			ctx,
			s.rebind(`SELECT `+jobSelectColumns+`
			   FROM business_image_jobs
			  WHERE user_id = ? AND conversation_id = ?
			  ORDER BY created_at DESC
			  LIMIT ?`),
			userID,
			conversationID,
			limit,
		)
	} else {
		rows, err = s.db.QueryContext(
			ctx,
			s.rebind(`SELECT `+jobSelectColumns+`
			   FROM business_image_jobs
			  WHERE user_id = ?
			  ORDER BY updated_at DESC
			  LIMIT ?`),
			userID,
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Job, 0)
	for rows.Next() {
		item, err := scanJob(rows)
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

func (s *Store) AdminList(ctx context.Context, filter AdminListFilter) ([]Job, int64, error) {
	filter.UserID = strings.TrimSpace(filter.UserID)
	filter.Status = normalizeOptionalStatus(filter.Status)
	filter.Platform = strings.TrimSpace(filter.Platform)
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	whereSQL, args := adminListWhere(filter)
	var total int64
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM business_image_jobs`+whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.Limit, filter.Offset)
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT `+jobSelectColumns+`
		   FROM business_image_jobs`+whereSQL+`
		  ORDER BY updated_at DESC, created_at DESC
		  LIMIT ? OFFSET ?`),
		queryArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]Job, 0)
	for rows.Next() {
		item, err := scanJob(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Store) CountUserActive(ctx context.Context, userID string) (ActiveCounts, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ActiveCounts{}, nil
	}
	return s.countActiveWith(ctx, s.db, "user_id = ?", userID)
}

func (s *Store) CountProviderRunning(ctx context.Context, providerID string, platform string) (int64, error) {
	providerID = strings.TrimSpace(providerID)
	platform = strings.TrimSpace(platform)
	if providerID != "" {
		counts, err := s.countActiveWith(ctx, s.db, "provider_id = ? AND status = ?", providerID, StatusRunning)
		return counts.Running, err
	}
	if platform != "" {
		counts, err := s.countActiveWith(ctx, s.db, "platform = ? AND status = ?", platform, StatusRunning)
		return counts.Running, err
	}
	return 0, nil
}

func (s *Store) CountQueued(ctx context.Context) (int64, error) {
	counts, err := s.countActiveWith(ctx, s.db, "status = ?", StatusQueued)
	return counts.Queued, err
}

func (s *Store) countActiveWith(ctx context.Context, q jobQueryer, extraWhere string, args ...any) (ActiveCounts, error) {
	extraWhere = strings.TrimSpace(extraWhere)
	clauses := []string{"status IN (?, ?)"}
	queryArgs := []any{StatusQueued, StatusRunning}
	if extraWhere != "" {
		clauses = append(clauses, extraWhere)
		queryArgs = append(queryArgs, args...)
	}
	query := `SELECT status, COUNT(*) FROM business_image_jobs WHERE ` + strings.Join(clauses, " AND ") + ` GROUP BY status`
	rows, err := q.QueryContext(ctx, s.rebind(query), queryArgs...)
	if err != nil {
		return ActiveCounts{}, err
	}
	defer rows.Close()
	var counts ActiveCounts
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return ActiveCounts{}, err
		}
		switch normalizeStatus(status) {
		case StatusQueued:
			counts.Queued += count
		case StatusRunning:
			counts.Running += count
		}
	}
	return counts, rows.Err()
}

func (s *Store) ListQueued(ctx context.Context, limit int) ([]Job, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	nowText := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT `+jobSelectColumns+`
		   FROM business_image_jobs
		  WHERE status = ?
		    AND (next_run_at = '' OR next_run_at <= ?)
		    AND (lease_until = '' OR lease_until <= ?)
		  ORDER BY created_at ASC
		  LIMIT ?`),
		StatusQueued,
		nowText,
		nowText,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Job, 0)
	for rows.Next() {
		item, err := scanJob(rows)
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

func (s *Store) ClaimQueued(ctx context.Context, id string, userID string) (Job, bool, error) {
	return s.ClaimQueuedBy(ctx, id, userID, "local")
}

func (s *Store) ClaimQueuedByJobID(ctx context.Context, id string, workerID string) (Job, bool, error) {
	id = clean(id)
	workerID = normalizeWorkerID(workerID)
	if id == "" {
		return Job{}, false, nil
	}
	if s.isPostgres() {
		return s.claimQueuedPostgresByJobID(ctx, id, workerID)
	}
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	leaseUntil := nowTime.Add(defaultClaimLease).Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE business_image_jobs
		    SET stage = ?,
		        claimed_by = ?,
		        claimed_at = ?,
		        lease_until = ?,
		        attempts = attempts + 1,
		        updated_at = ?
		  WHERE id = ?
		    AND status = ?
		    AND (next_run_at = '' OR next_run_at <= ?)
		    AND (lease_until = '' OR lease_until <= ?)`,
		"claimed",
		workerID,
		now,
		leaseUntil,
		now,
		id,
		StatusQueued,
		now,
		now,
	)
	if err != nil {
		return Job{}, false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Job{}, false, err
	}
	if affected == 0 {
		return Job{}, false, nil
	}
	return s.GetByID(ctx, id)
}

func (s *Store) ClaimQueuedBy(ctx context.Context, id string, userID string, workerID string) (Job, bool, error) {
	id = clean(id)
	userID = strings.TrimSpace(userID)
	workerID = normalizeWorkerID(workerID)
	if id == "" || userID == "" {
		return Job{}, false, nil
	}
	if s.isPostgres() {
		return s.claimQueuedPostgres(ctx, id, userID, workerID)
	}
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	leaseUntil := nowTime.Add(defaultClaimLease).Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE business_image_jobs
		    SET stage = ?,
		        claimed_by = ?,
		        claimed_at = ?,
		        lease_until = ?,
		        attempts = attempts + 1,
		        updated_at = ?
		  WHERE id = ?
		    AND user_id = ?
		    AND status = ?
		    AND (next_run_at = '' OR next_run_at <= ?)
		    AND (lease_until = '' OR lease_until <= ?)`,
		"claimed",
		workerID,
		now,
		leaseUntil,
		now,
		id,
		userID,
		StatusQueued,
		now,
		now,
	)
	if err != nil {
		return Job{}, false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Job{}, false, err
	}
	if affected == 0 {
		return Job{}, false, nil
	}
	return s.Get(ctx, id, userID)
}

func (s *Store) claimQueuedPostgresByJobID(ctx context.Context, id string, workerID string) (Job, bool, error) {
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	leaseUntil := nowTime.Add(defaultClaimLease).Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, false, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(
		ctx,
		`SELECT `+jobSelectColumns+`
		   FROM business_image_jobs
		  WHERE id = $1
		    AND status = $2
		    AND (next_run_at = '' OR next_run_at <= $3)
		    AND (lease_until = '' OR lease_until <= $4)
		  FOR UPDATE SKIP LOCKED`,
		id,
		StatusQueued,
		now,
		now,
	)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE business_image_jobs
		    SET stage = $1,
		        claimed_by = $2,
		        claimed_at = $3,
		        lease_until = $4,
		        attempts = attempts + 1,
		        updated_at = $5
		  WHERE id = $6`,
		"claimed",
		workerID,
		now,
		leaseUntil,
		now,
		id,
	); err != nil {
		return Job{}, false, err
	}
	job.Stage = "claimed"
	job.ClaimedBy = workerID
	job.ClaimedAt = now
	job.LeaseUntil = leaseUntil
	job.Attempts++
	job.UpdatedAt = now
	if err := tx.Commit(); err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func (s *Store) claimQueuedPostgres(ctx context.Context, id string, userID string, workerID string) (Job, bool, error) {
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	leaseUntil := nowTime.Add(defaultClaimLease).Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, false, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(
		ctx,
		`SELECT `+jobSelectColumns+`
		   FROM business_image_jobs
		  WHERE id = $1
		    AND user_id = $2
		    AND status = $3
		    AND (next_run_at = '' OR next_run_at <= $4)
		    AND (lease_until = '' OR lease_until <= $5)
		  FOR UPDATE SKIP LOCKED`,
		id,
		userID,
		StatusQueued,
		now,
		now,
	)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE business_image_jobs
		    SET stage = $1,
		        claimed_by = $2,
		        claimed_at = $3,
		        lease_until = $4,
		        attempts = attempts + 1,
		        updated_at = $5
		  WHERE id = $6 AND user_id = $7`,
		"claimed",
		workerID,
		now,
		leaseUntil,
		now,
		id,
		userID,
	); err != nil {
		return Job{}, false, err
	}
	job.Stage = "claimed"
	job.ClaimedBy = workerID
	job.ClaimedAt = now
	job.LeaseUntil = leaseUntil
	job.Attempts++
	job.UpdatedAt = now
	if err := tx.Commit(); err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func adminListWhere(filter AdminListFilter) (string, []any) {
	clauses := []string{}
	args := []any{}
	if filter.UserID != "" {
		clauses = append(clauses, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Platform != "" {
		clauses = append(clauses, "platform = ?")
		args = append(args, filter.Platform)
	}
	if filter.From != "" {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, filter.From)
	}
	if filter.To != "" {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, filter.To)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *Store) RequestCancel(ctx context.Context, id string, userID string) (Job, bool, error) {
	job, ok, err := s.Get(ctx, id, userID)
	if err != nil || !ok {
		return Job{}, ok, err
	}
	switch normalizeStatus(job.Status) {
	case StatusQueued:
		job.Status = StatusCancelled
		job.Stage = "cancelled"
		job.ErrorCode = "cancelled"
		job.ErrorMessage = "任务已取消"
	case StatusRunning, StatusCancelRequested:
		job.Status = StatusCancelRequested
		job.Stage = "cancel_requested"
		job.ErrorCode = "cancel_requested"
		job.ErrorMessage = "正在取消任务"
	case StatusSucceeded, StatusFailed, StatusCancelled:
		return job, true, nil
	default:
		job.Status = StatusCancelRequested
		job.Stage = "cancel_requested"
		job.ErrorCode = "cancel_requested"
		job.ErrorMessage = "正在取消任务"
	}
	saved, err := s.Save(ctx, job)
	if err != nil {
		return Job{}, false, err
	}
	return saved, true, nil
}

func (s *Store) ReconcileStale(ctx context.Context, options ReconcileOptions) (ReconcileResult, error) {
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	nowText := now.Format(time.RFC3339Nano)
	result := ReconcileResult{}

	if !options.QueuedBefore.IsZero() {
		affected, err := s.updateStaleJobs(
			ctx,
			StatusQueued,
			"failed",
			"stale_queued",
			"任务排队超时，已自动标记失败",
			options.QueuedBefore,
			nowText,
		)
		if err != nil {
			return result, err
		}
		result.QueuedFailed = affected
	}
	if !options.RunningBefore.IsZero() {
		affected, err := s.updateStaleJobs(
			ctx,
			StatusRunning,
			"failed",
			"stale_running",
			"任务运行超时，已自动标记失败",
			options.RunningBefore,
			nowText,
		)
		if err != nil {
			return result, err
		}
		result.RunningFailed = affected
	}
	if !options.CancelRequestedBefore.IsZero() {
		affected, err := s.updateStaleJobs(
			ctx,
			StatusCancelRequested,
			"cancelled",
			"stale_cancel_requested",
			"任务取消超时，已自动标记取消",
			options.CancelRequestedBefore,
			nowText,
		)
		if err != nil {
			return result, err
		}
		result.CancelRequestedCancelled = affected
	}

	return result, nil
}

func (s *Store) ListStale(ctx context.Context, options ReconcileOptions, limit int) ([]Job, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	items := make([]Job, 0)
	appendItems := func(status string, before time.Time, remaining int) error {
		if before.IsZero() || remaining <= 0 {
			return nil
		}
		beforeText := before.UTC().Format(time.RFC3339Nano)
		args := []any{normalizeStatus(status), beforeText}
		whereSQL := `status = ?
			    AND updated_at != ''
			    AND updated_at < ?`
		if normalizeStatus(status) == StatusRunning {
			whereSQL = `status = ?
			    AND (
			      (lease_until != '' AND lease_until < ?)
			      OR (lease_until = '' AND updated_at != '' AND updated_at < ?)
			    )`
			args = []any{normalizeStatus(status), beforeText, beforeText}
		}
		args = append(args, remaining)
		rows, err := s.db.QueryContext(
			ctx,
			s.rebind(`SELECT `+jobSelectColumns+`
			   FROM business_image_jobs
				  WHERE `+whereSQL+`
				  ORDER BY updated_at ASC
				  LIMIT ?`),
			args...,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanJob(rows)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	}
	if err := appendItems(StatusCancelRequested, options.CancelRequestedBefore, limit-len(items)); err != nil {
		return nil, err
	}
	if err := appendItems(StatusRunning, options.RunningBefore, limit-len(items)); err != nil {
		return nil, err
	}
	if err := appendItems(StatusQueued, options.QueuedBefore, limit-len(items)); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) updateStaleJobs(
	ctx context.Context,
	currentStatus string,
	finalStatus string,
	errorCode string,
	errorMessage string,
	before time.Time,
	nowText string,
) (int64, error) {
	currentStatus = normalizeStatus(currentStatus)
	finalStatus = normalizeStatus(finalStatus)
	if before.IsZero() || isFinalStatus(currentStatus) || !isFinalStatus(finalStatus) {
		return 0, nil
	}
	stage := finalStatus
	if finalStatus == StatusFailed {
		stage = "stale"
	}
	beforeText := before.UTC().Format(time.RFC3339Nano)
	if s.isPostgres() {
		return s.updateStaleJobsPostgres(ctx, currentStatus, finalStatus, errorCode, errorMessage, before, nowText)
	}
	whereSQL := `status = ?
		    AND updated_at != ''
		    AND updated_at < ?`
	whereArgs := []any{currentStatus, beforeText}
	if currentStatus == StatusRunning {
		whereSQL = `status = ?
		    AND (
		      (lease_until != '' AND lease_until < ?)
		      OR (lease_until = '' AND updated_at != '' AND updated_at < ?)
		    )`
		whereArgs = []any{currentStatus, beforeText, beforeText}
	}
	args := []any{
		finalStatus,
		stage,
		strings.TrimSpace(errorCode),
		strings.TrimSpace(errorMessage),
		strings.TrimSpace(firstNonEmpty(errorMessage, errorCode)),
		nowText,
		nowText,
		nowText,
	}
	args = append(args, whereArgs...)
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE business_image_jobs
		    SET status = ?,
		        stage = ?,
		        error_code = ?,
		        error_message = ?,
		        last_error = ?,
		        lease_until = '',
		        next_run_at = '',
		        finished_at = ?,
		        total_duration_ms = CASE
		          WHEN created_at != '' THEN CAST((julianday(?) - julianday(created_at)) * 86400000 AS INTEGER)
		          ELSE total_duration_ms
		        END,
		        updated_at = ?
		  WHERE `+whereSQL,
		args...,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) updateStaleJobsPostgres(
	ctx context.Context,
	currentStatus string,
	finalStatus string,
	errorCode string,
	errorMessage string,
	before time.Time,
	nowText string,
) (int64, error) {
	beforeText := before.UTC().Format(time.RFC3339Nano)
	stage := finalStatus
	if finalStatus == StatusFailed {
		stage = "stale"
	}
	whereSQL := `status = $9
		    AND updated_at != ''
		    AND updated_at < $10`
	if currentStatus == StatusRunning {
		whereSQL = `status = $9
		    AND (
		      (lease_until != '' AND lease_until < $10)
		      OR (lease_until = '' AND updated_at != '' AND updated_at < $10)
		    )`
	}
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE business_image_jobs
		    SET status = $1,
		        stage = $2,
		        error_code = $3,
		        error_message = $4,
		        last_error = $5,
		        lease_until = '',
		        next_run_at = '',
		        finished_at = $6,
		        total_duration_ms = CASE
		          WHEN created_at != '' THEN CAST(EXTRACT(EPOCH FROM (($7)::timestamptz - created_at::timestamptz)) * 1000 AS BIGINT)
		          ELSE total_duration_ms
		        END,
		        updated_at = $8
		  WHERE `+whereSQL,
		finalStatus,
		stage,
		strings.TrimSpace(errorCode),
		strings.TrimSpace(errorMessage),
		strings.TrimSpace(firstNonEmpty(errorMessage, errorCode)),
		nowText,
		nowText,
		nowText,
		currentStatus,
		beforeText,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var item Job
	var upstreamSent int
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.ConversationID,
		&item.GenerationID,
		&item.TurnID,
		&item.Platform,
		&item.ProviderID,
		&item.ProviderName,
		&item.Model,
		&item.Prompt,
		&item.Size,
		&item.Quality,
		&item.RequestedCount,
		&item.ActualCount,
		&item.Status,
		&item.Stage,
		&upstreamSent,
		&item.ErrorCode,
		&item.ErrorMessage,
		&item.QueueWaitMS,
		&item.UpstreamDurationMS,
		&item.PersistDurationMS,
		&item.TotalDurationMS,
		&item.StorageBytes,
		&item.CreditReserved,
		&item.CreditRefunded,
		&item.PayloadJSON,
		&item.CreatedAt,
		&item.QueuedAt,
		&item.StartedAt,
		&item.FinishedAt,
		&item.UpdatedAt,
		&item.ClaimedBy,
		&item.ClaimedAt,
		&item.LeaseUntil,
		&item.Attempts,
		&item.LastError,
		&item.NextRunAt,
	)
	if err != nil {
		return item, err
	}
	item.UpstreamSent = upstreamSent == 1
	return withDerivedFields(item), nil
}

func HasUpstreamSent(job Job) bool {
	if job.UpstreamSent {
		return true
	}
	if strings.TrimSpace(job.Stage) == "upstream" {
		return true
	}
	if normalizeStatus(job.Status) == StatusSucceeded {
		return true
	}
	return job.UpstreamDurationMS > 0
}

func withDerivedFields(job Job) Job {
	job.UpstreamSent = HasUpstreamSent(job)
	job.UpstreamStatus = upstreamStatus(job.UpstreamSent)
	return job
}

func upstreamStatus(sent bool) string {
	if sent {
		return "sent"
	}
	return "pending"
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func normalizeWorkerID(workerID string) string {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return "local"
	}
	return workerID
}

func (s *Store) isPostgres() bool {
	return database.IsPostgres(s.driver)
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case StatusRunning:
		return StatusRunning
	case StatusCancelRequested:
		return StatusCancelRequested
	case StatusCancelled, "canceled":
		return StatusCancelled
	case StatusSucceeded:
		return StatusSucceeded
	case StatusFailed:
		return StatusFailed
	default:
		return StatusQueued
	}
}

func normalizeOptionalStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case StatusQueued, StatusRunning, StatusCancelRequested, StatusCancelled, StatusSucceeded, StatusFailed:
		return status
	default:
		return ""
	}
}

func isFinalStatus(status string) bool {
	status = normalizeStatus(status)
	return status == StatusSucceeded || status == StatusFailed || status == StatusCancelled
}

func clean(value string) string {
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
