package businessimage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"imagestudio/internal/businesscredits"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

const DevUserID = "dev_user"

type Conversation struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Generation struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	ConversationID string          `json:"conversation_id"`
	TurnID         string          `json:"turn_id"`
	Prompt         string          `json:"prompt"`
	Model          string          `json:"model"`
	Size           string          `json:"size,omitempty"`
	Quality        string          `json:"quality,omitempty"`
	Count          int             `json:"count"`
	Status         string          `json:"status"`
	Response       json.RawMessage `json:"response,omitempty"`
	Error          string          `json:"error,omitempty"`
	CreatedAt      string          `json:"created_at"`
	FinishedAt     string          `json:"finished_at,omitempty"`
}

type Asset struct {
	ID             string `json:"id"`
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id"`
	GenerationID   string `json:"generation_id"`
	FileName       string `json:"file_name"`
	FilePath       string `json:"file_path"`
	URL            string `json:"url"`
	MimeType       string `json:"mime_type"`
	SizeBytes      int64  `json:"size_bytes"`
	SHA256         string `json:"sha256"`
	CreatedAt      string `json:"created_at"`
}

type AssetDetail struct {
	Asset
	ConversationTitle string `json:"conversation_title,omitempty"`
	Prompt            string `json:"prompt,omitempty"`
	Model             string `json:"model,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
}

type UserUsage struct {
	UserID          string `json:"user_id"`
	GenerationCount int64  `json:"generation_count"`
	SuccessCount    int64  `json:"success_count"`
	FailedCount     int64  `json:"failed_count"`
	ImageCount      int64  `json:"image_count"`
	StorageBytes    int64  `json:"storage_bytes"`
	LastGeneratedAt string `json:"last_generated_at,omitempty"`
}

type ModelUsage struct {
	Model           string `json:"model"`
	GenerationCount int64  `json:"generation_count"`
	SuccessCount    int64  `json:"success_count"`
	FailedCount     int64  `json:"failed_count"`
	ImageCount      int64  `json:"image_count"`
	CreditDelta     int64  `json:"credit_delta"`
	CreditsUsed     int64  `json:"credits_used"`
	LastGeneratedAt string `json:"last_generated_at,omitempty"`
}

type UsageRecord struct {
	ID             string `json:"id"`
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id"`
	GenerationID   string `json:"generation_id"`
	TurnID         string `json:"turn_id"`
	Prompt         string `json:"prompt"`
	Model          string `json:"model"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	Count          int    `json:"count"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	CreatedAt      string `json:"created_at"`
	FinishedAt     string `json:"finished_at,omitempty"`
	DurationMs     int64  `json:"duration_ms"`
	CreditDelta    int64  `json:"credit_delta"`
	CreditsUsed    int64  `json:"credits_used"`
}

type UsageRecordFilter struct {
	UserID  string
	UserIDs []string
	Status  string
	Model   string
	From    string
	To      string
}

type UsageSummaryFilter struct {
	From string
	To   string
}

type StorageAsset struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id"`
	GenerationID   string `json:"generation_id"`
	FileName       string `json:"file_name"`
	FilePath       string `json:"file_path"`
	URL            string `json:"url"`
	SizeBytes      int64  `json:"size_bytes"`
}

type StorageReference struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id"`
	GenerationID   string `json:"generation_id"`
	FileName       string `json:"file_name"`
}

type BrokenAsset struct {
	FileName     string `json:"file_name"`
	UserID       string `json:"user_id"`
	GenerationID string `json:"generation_id"`
	Reason       string `json:"reason"`
}

type StorageReportData struct {
	Assets           []StorageAsset     `json:"assets"`
	References       []StorageReference `json:"references"`
	LegacyReferences []StorageReference `json:"legacy_references"`
	BrokenAssets     []BrokenAsset      `json:"broken_assets"`
}

type LegacyAssetBackfillFile struct {
	FileName  string `json:"file_name"`
	FilePath  string `json:"file_path"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type LegacyAssetBackfillResult struct {
	Matched            int `json:"matched"`
	Backfilled         int `json:"backfilled"`
	SkippedMissingFile int `json:"skipped_missing_file"`
	SkippedAmbiguous   int `json:"skipped_ambiguous"`
}

type ConversationWithGenerations struct {
	Conversation Conversation `json:"conversation"`
	Generations  []Generation `json:"generations"`
}

type DeleteResult struct {
	Conversations int64 `json:"conversations"`
	Generations   int64 `json:"generations"`
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
		`CREATE TABLE IF NOT EXISTS business_image_conversations (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			title TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_conversations_user_updated
			ON business_image_conversations(user_id, updated_at DESC);`,
		`CREATE TABLE IF NOT EXISTS business_image_generations (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			model TEXT NOT NULL,
			size TEXT NOT NULL,
			quality TEXT NOT NULL,
			count INTEGER NOT NULL,
			status TEXT NOT NULL,
			response_json BLOB,
			error TEXT NOT NULL,
			created_at TEXT NOT NULL,
			finished_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_generations_conversation_created
			ON business_image_generations(conversation_id, created_at ASC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_generations_user_created
			ON business_image_generations(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_generations_filter
			ON business_image_generations(user_id, status, model, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_generations_created
			ON business_image_generations(created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS business_image_assets (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			generation_id TEXT NOT NULL,
			file_name TEXT NOT NULL,
			file_path TEXT NOT NULL,
			url TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			sha256 TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_business_image_assets_file_name
			ON business_image_assets(file_name);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_assets_user_conversation
			ON business_image_assets(user_id, conversation_id);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_assets_user_created
			ON business_image_assets(user_id, created_at DESC, file_name DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_business_image_assets_generation
			ON business_image_assets(generation_id);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UpsertConversation(ctx context.Context, conversation Conversation) (Conversation, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	conversation.ID = cleanID(conversation.ID)
	conversation.UserID = firstNonEmpty(conversation.UserID, DevUserID)
	conversation.Title = firstNonEmpty(conversation.Title, "新建生图会话")
	conversation.CreatedAt = firstNonEmpty(conversation.CreatedAt, now)
	conversation.UpdatedAt = firstNonEmpty(conversation.UpdatedAt, now)
	if conversation.ID == "" {
		return Conversation{}, fmt.Errorf("conversation id is required")
	}
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_image_conversations(id, user_id, title, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			updated_at = excluded.updated_at
		 WHERE business_image_conversations.user_id = excluded.user_id`),
		conversation.ID,
		conversation.UserID,
		conversation.Title,
		conversation.CreatedAt,
		conversation.UpdatedAt,
	)
	return conversation, err
}

func (s *Store) GetConversation(ctx context.Context, id string, userID ...string) (Conversation, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Conversation{}, false, fmt.Errorf("conversation id is required")
	}
	var conversation Conversation
	args := []any{id}
	whereUserID := firstNonEmpty(userID...)
	query := `SELECT id, user_id, title, created_at, updated_at
		 FROM business_image_conversations
		 WHERE id = ?`
	if whereUserID != "" {
		query += ` AND user_id = ?`
		args = append(args, whereUserID)
	}
	err := s.db.QueryRowContext(ctx, s.rebind(query), args...).Scan(
		&conversation.ID,
		&conversation.UserID,
		&conversation.Title,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Conversation{}, false, nil
	}
	if err != nil {
		return Conversation{}, false, err
	}
	return conversation, true, nil
}

func (s *Store) ListConversations(ctx context.Context, userID string, limit int) ([]Conversation, error) {
	userID = firstNonEmpty(userID, DevUserID)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT id, user_id, title, created_at, updated_at
		 FROM business_image_conversations
		 WHERE user_id = ?
		 ORDER BY updated_at DESC
		 LIMIT ?`),
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Conversation{}
	for rows.Next() {
		var conversation Conversation
		if err := rows.Scan(
			&conversation.ID,
			&conversation.UserID,
			&conversation.Title,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, conversation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetConversationWithGenerations(ctx context.Context, id string, userID string, limit int) (ConversationWithGenerations, bool, error) {
	conversation, ok, err := s.GetConversation(ctx, id, userID)
	if err != nil || !ok {
		return ConversationWithGenerations{}, ok, err
	}
	generations, err := s.ListGenerations(ctx, conversation.ID, userID, limit)
	if err != nil {
		return ConversationWithGenerations{}, false, err
	}
	return ConversationWithGenerations{
		Conversation: conversation,
		Generations:  generations,
	}, true, nil
}

func (s *Store) SaveGeneration(ctx context.Context, generation Generation) (Generation, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	generation.ID = cleanID(generation.ID)
	generation.UserID = firstNonEmpty(generation.UserID, DevUserID)
	generation.ConversationID = cleanID(generation.ConversationID)
	generation.TurnID = cleanID(generation.TurnID)
	generation.Status = firstNonEmpty(generation.Status, "succeeded")
	generation.CreatedAt = firstNonEmpty(generation.CreatedAt, now)
	generation.FinishedAt = firstNonEmpty(generation.FinishedAt, now)
	if generation.ID == "" {
		return Generation{}, fmt.Errorf("generation id is required")
	}
	if generation.ConversationID == "" {
		return Generation{}, fmt.Errorf("conversation id is required")
	}
	if generation.TurnID == "" {
		return Generation{}, fmt.Errorf("turn id is required")
	}
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_image_generations(
			id, user_id, conversation_id, turn_id, prompt, model, size, quality,
			count, status, response_json, error, created_at, finished_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			conversation_id = excluded.conversation_id,
			turn_id = excluded.turn_id,
			prompt = excluded.prompt,
			model = excluded.model,
			size = excluded.size,
			quality = excluded.quality,
			count = excluded.count,
			status = excluded.status,
			response_json = excluded.response_json,
			error = excluded.error,
			finished_at = excluded.finished_at
		 WHERE business_image_generations.user_id = excluded.user_id`),
		generation.ID,
		generation.UserID,
		generation.ConversationID,
		generation.TurnID,
		generation.Prompt,
		generation.Model,
		generation.Size,
		generation.Quality,
		generation.Count,
		generation.Status,
		[]byte(generation.Response),
		generation.Error,
		generation.CreatedAt,
		generation.FinishedAt,
	)
	return generation, err
}

func (s *Store) MarkGenerationFinished(ctx context.Context, userID string, generationID string, status string, errorMessage string) (int64, error) {
	userID = firstNonEmpty(userID, DevUserID)
	generationID = cleanID(generationID)
	status = strings.TrimSpace(status)
	errorMessage = strings.TrimSpace(errorMessage)
	if userID == "" || generationID == "" || status == "" {
		return 0, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_image_generations
		    SET status = ?,
		        error = ?,
		        finished_at = ?
		  WHERE status IN ('queued', 'running', 'cancel_requested')
		    AND user_id = ?
		    AND id = ?`),
		status,
		errorMessage,
		now,
		userID,
		generationID,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) SaveAsset(ctx context.Context, asset Asset) (Asset, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	asset.ID = cleanID(asset.ID)
	asset.UserID = firstNonEmpty(asset.UserID, DevUserID)
	asset.ConversationID = cleanID(asset.ConversationID)
	asset.GenerationID = cleanID(asset.GenerationID)
	asset.FileName = cleanImageFileName(asset.FileName)
	asset.FilePath = strings.TrimSpace(asset.FilePath)
	asset.URL = firstNonEmpty(asset.URL, "/v1/files/image/"+asset.FileName)
	asset.MimeType = firstNonEmpty(asset.MimeType, "image/png")
	asset.SHA256 = strings.TrimSpace(asset.SHA256)
	asset.CreatedAt = firstNonEmpty(asset.CreatedAt, now)
	if asset.ID == "" {
		asset.ID = asset.FileName
	}
	if asset.FileName == "" {
		return Asset{}, fmt.Errorf("asset file name is required")
	}
	if asset.ConversationID == "" {
		return Asset{}, fmt.Errorf("asset conversation id is required")
	}
	if asset.GenerationID == "" {
		return Asset{}, fmt.Errorf("asset generation id is required")
	}
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_image_assets(
			id, user_id, conversation_id, generation_id, file_name, file_path,
			url, mime_type, size_bytes, sha256, created_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_name) DO UPDATE SET
			user_id = excluded.user_id,
			conversation_id = excluded.conversation_id,
			generation_id = excluded.generation_id,
			file_path = excluded.file_path,
			url = excluded.url,
			mime_type = excluded.mime_type,
			size_bytes = excluded.size_bytes,
			sha256 = excluded.sha256`),
		asset.ID,
		asset.UserID,
		asset.ConversationID,
		asset.GenerationID,
		asset.FileName,
		asset.FilePath,
		asset.URL,
		asset.MimeType,
		asset.SizeBytes,
		asset.SHA256,
		asset.CreatedAt,
	)
	return asset, err
}

func (s *Store) ListGenerations(ctx context.Context, conversationID string, args ...any) ([]Generation, error) {
	conversationID = cleanID(conversationID)
	if conversationID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	userID, limit := parseListGenerationsArgs(args...)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id, user_id, conversation_id, turn_id, prompt, model, size, quality,
			count, status, response_json, error, created_at, finished_at
		 FROM business_image_generations
		 WHERE conversation_id = ?`
	queryArgs := []any{conversationID}
	if userID != "" {
		query += ` AND user_id = ?`
		queryArgs = append(queryArgs, userID)
	}
	query += `
		 ORDER BY created_at ASC
		 LIMIT ?`
	queryArgs = append(queryArgs, limit)
	rows, err := s.db.QueryContext(ctx, s.rebind(query), queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	generations := []Generation{}
	for rows.Next() {
		var generation Generation
		var response []byte
		if err := rows.Scan(
			&generation.ID,
			&generation.UserID,
			&generation.ConversationID,
			&generation.TurnID,
			&generation.Prompt,
			&generation.Model,
			&generation.Size,
			&generation.Quality,
			&generation.Count,
			&generation.Status,
			&response,
			&generation.Error,
			&generation.CreatedAt,
			&generation.FinishedAt,
		); err != nil {
			return nil, err
		}
		generation.Response = append(json.RawMessage(nil), response...)
		generations = append(generations, generation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return generations, nil
}

func (s *Store) ImageFileNamesForConversation(ctx context.Context, conversationID string, userID string) ([]string, error) {
	conversationID = cleanID(conversationID)
	userID = firstNonEmpty(userID, DevUserID)
	if conversationID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	assetFileNames, err := s.AssetFileNamesForConversation(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT response_json
		 FROM business_image_generations
		 WHERE conversation_id = ? AND user_id = ?`),
		conversationID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	responseFileNames, err := collectImageFileNames(rows)
	if err != nil {
		return nil, err
	}
	return mergeFileNameSlices(assetFileNames, responseFileNames), nil
}

func (s *Store) ImageFileNamesForUser(ctx context.Context, userID string) ([]string, error) {
	userID = firstNonEmpty(userID, DevUserID)
	assetFileNames, err := s.AssetFileNamesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT response_json
		 FROM business_image_generations
		 WHERE user_id = ?`),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	responseFileNames, err := collectImageFileNames(rows)
	if err != nil {
		return nil, err
	}
	return mergeFileNameSlices(assetFileNames, responseFileNames), nil
}

func (s *Store) ImageFileNameReferenced(ctx context.Context, fileName string) (bool, error) {
	fileName = cleanImageFileName(fileName)
	if fileName == "" {
		return false, nil
	}
	var assetCount int
	if err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT COUNT(*)
		 FROM business_image_assets a
		 JOIN business_image_generations g
		   ON g.id = a.generation_id AND g.user_id = a.user_id
		 WHERE a.file_name = ?`),
		fileName,
	).Scan(&assetCount); err != nil {
		return false, err
	}
	if assetCount > 0 {
		return true, nil
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT response_json
		 FROM business_image_generations
		 WHERE response_json IS NOT NULL`,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var response []byte
		if err := rows.Scan(&response); err != nil {
			return false, err
		}
		for _, candidate := range imageFileNamesFromResponse(response) {
			if candidate == fileName {
				return true, nil
			}
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (s *Store) ImageFileOwnerUserIDs(ctx context.Context, fileName string) ([]string, error) {
	fileName = cleanImageFileName(fileName)
	if fileName == "" {
		return nil, nil
	}
	owners, err := s.AssetOwnerUserIDs(ctx, fileName)
	if err != nil {
		return nil, err
	}
	if len(owners) > 0 {
		return owners, nil
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT user_id, response_json
		 FROM business_image_generations
		 WHERE response_json IS NOT NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	owners = []string{}
	for rows.Next() {
		var userID string
		var response []byte
		if err := rows.Scan(&userID, &response); err != nil {
			return nil, err
		}
		for _, candidate := range imageFileNamesFromResponse(response) {
			if candidate != fileName {
				continue
			}
			if _, ok := seen[userID]; ok {
				continue
			}
			seen[userID] = struct{}{}
			owners = append(owners, userID)
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return owners, nil
}

func (s *Store) AssetFileNamesForConversation(ctx context.Context, conversationID string, userID string) ([]string, error) {
	conversationID = cleanID(conversationID)
	userID = firstNonEmpty(userID, DevUserID)
	if conversationID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT file_name
		 FROM business_image_assets
		 WHERE conversation_id = ? AND user_id = ?`),
		conversationID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAssetFileNames(rows)
}

func (s *Store) AssetFileNamesForUser(ctx context.Context, userID string) ([]string, error) {
	userID = firstNonEmpty(userID, DevUserID)
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT file_name
		 FROM business_image_assets
		 WHERE user_id = ?`),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAssetFileNames(rows)
}

func (s *Store) AssetOwnerUserIDs(ctx context.Context, fileName string) ([]string, error) {
	fileName = cleanImageFileName(fileName)
	if fileName == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT DISTINCT user_id
		 FROM business_image_assets
		 WHERE file_name = ?`),
		fileName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owners := []string{}
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(userID) != "" {
			owners = append(owners, userID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return owners, nil
}

func (s *Store) DeleteAssetsByFileName(ctx context.Context, fileName string) error {
	fileName = cleanImageFileName(fileName)
	if fileName == "" {
		return nil
	}
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`DELETE FROM business_image_assets
		 WHERE file_name = ?`),
		fileName,
	)
	return err
}

func (s *Store) DeleteAssetsByUser(ctx context.Context, userID string) error {
	userID = firstNonEmpty(userID, DevUserID)
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`DELETE FROM business_image_assets
		 WHERE user_id = ?`),
		userID,
	)
	return err
}

func (s *Store) UserUsage(ctx context.Context) (map[string]UserUsage, error) {
	return s.UserUsageFiltered(ctx, UsageSummaryFilter{})
}

func (s *Store) UserUsageFiltered(ctx context.Context, filter UsageSummaryFilter) (map[string]UserUsage, error) {
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	usages := map[string]UserUsage{}

	generationFilters, generationArgs := usageTimeFilters("created_at", filter)
	generationWhereSQL := ""
	if len(generationFilters) > 0 {
		generationWhereSQL = " WHERE " + strings.Join(generationFilters, " AND ")
	}
	generationRows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT user_id,
		        COUNT(*) AS generation_count,
		        SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END) AS success_count,
		        SUM(CASE WHEN status != 'succeeded' THEN 1 ELSE 0 END) AS failed_count,
		        MAX(created_at) AS last_generated_at
		 FROM business_image_generations
		 `+generationWhereSQL+`
		 GROUP BY user_id`),
		generationArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer generationRows.Close()

	for generationRows.Next() {
		var usage UserUsage
		if err := generationRows.Scan(
			&usage.UserID,
			&usage.GenerationCount,
			&usage.SuccessCount,
			&usage.FailedCount,
			&usage.LastGeneratedAt,
		); err != nil {
			return nil, err
		}
		usages[usage.UserID] = usage
	}
	if err := generationRows.Err(); err != nil {
		return nil, err
	}

	assetFilters, assetArgs := usageTimeFilters("generation.created_at", filter)
	assetWhereSQL := ""
	if len(assetFilters) > 0 {
		assetWhereSQL = " WHERE " + strings.Join(assetFilters, " AND ")
	}
	assetRows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT asset.user_id,
		        COUNT(*) AS image_count,
		        COALESCE(SUM(asset.size_bytes), 0) AS storage_bytes
		 FROM business_image_assets AS asset
		 JOIN business_image_generations AS generation
		   ON generation.id = asset.generation_id
		  AND generation.user_id = asset.user_id
		 `+assetWhereSQL+`
		 GROUP BY asset.user_id`),
		assetArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer assetRows.Close()

	for assetRows.Next() {
		var userID string
		var imageCount int64
		var storageBytes int64
		if err := assetRows.Scan(&userID, &imageCount, &storageBytes); err != nil {
			return nil, err
		}
		usage := usages[userID]
		usage.UserID = userID
		usage.ImageCount = imageCount
		usage.StorageBytes = storageBytes
		usages[userID] = usage
	}
	if err := assetRows.Err(); err != nil {
		return nil, err
	}

	if filter.From == "" && filter.To == "" {
		if err := s.addLegacyImageCounts(ctx, usages); err != nil {
			return nil, err
		}
	}

	return usages, nil
}

func (s *Store) ConversationTotal(ctx context.Context) (int64, error) {
	return s.ConversationTotalFiltered(ctx, UsageSummaryFilter{})
}

func (s *Store) ConversationTotalFiltered(ctx context.Context, filter UsageSummaryFilter) (int64, error) {
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	if filter.From == "" && filter.To == "" {
		var count int64
		err := s.db.QueryRowContext(
			ctx,
			s.rebind(`SELECT COUNT(*)
		 FROM business_image_conversations`),
		).Scan(&count)
		return count, err
	}

	filters, args := usageTimeFilters("created_at", filter)
	whereSQL := ""
	if len(filters) > 0 {
		whereSQL = " WHERE " + strings.Join(filters, " AND ")
	}
	var count int64
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT COUNT(DISTINCT conversation_id)
		 FROM business_image_generations`+whereSQL),
		args...,
	).Scan(&count)
	return count, err
}

func (s *Store) ModelUsage(ctx context.Context, limit int) ([]ModelUsage, error) {
	return s.ModelUsageFiltered(ctx, UsageSummaryFilter{}, limit)
}

func (s *Store) ModelUsageFiltered(ctx context.Context, filter UsageSummaryFilter, limit int) ([]ModelUsage, error) {
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	filters, filterArgs := usageTimeFilters("generation.created_at", filter)
	whereSQL := ""
	if len(filters) > 0 {
		whereSQL = " WHERE " + strings.Join(filters, " AND ")
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT generation.model,
		        COUNT(*) AS generation_count,
		        SUM(CASE WHEN generation.status = 'succeeded' THEN 1 ELSE 0 END) AS success_count,
		        SUM(CASE WHEN generation.status != 'succeeded' THEN 1 ELSE 0 END) AS failed_count,
		        SUM(CASE WHEN generation.status = 'succeeded' THEN generation.count ELSE 0 END) AS image_count,
		        COALESCE(SUM(ledger.credit_delta), 0) AS credit_delta,
		        MAX(generation.created_at) AS last_generated_at
		 FROM business_image_generations AS generation
		 LEFT JOIN (
		   SELECT user_id, generation_id, SUM(delta) AS credit_delta
		   FROM business_credit_ledger
		   WHERE reason IN (?, ?)
		   GROUP BY user_id, generation_id
		 ) AS ledger
		   ON ledger.generation_id = generation.id
		  AND ledger.user_id = generation.user_id`+whereSQL+`
		 GROUP BY generation.model
		 ORDER BY generation_count DESC, last_generated_at DESC
		 LIMIT ?`),
		append([]any{
			businesscredits.ReasonImageGenerationReserve,
			businesscredits.ReasonImageGenerationRefund,
		}, append(filterArgs, limit)...)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ModelUsage{}
	for rows.Next() {
		var item ModelUsage
		if err := rows.Scan(
			&item.Model,
			&item.GenerationCount,
			&item.SuccessCount,
			&item.FailedCount,
			&item.ImageCount,
			&item.CreditDelta,
			&item.LastGeneratedAt,
		); err != nil {
			return nil, err
		}
		if item.CreditDelta < 0 {
			item.CreditsUsed = -item.CreditDelta
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) UsageRecords(ctx context.Context, userID string, limit int) ([]UsageRecord, error) {
	items, _, err := s.UsageRecordsPage(ctx, UsageRecordFilter{UserID: userID}, limit, 0)
	return items, err
}

func (s *Store) UsageRecordsPage(ctx context.Context, filter UsageRecordFilter, limit int, offset int) ([]UsageRecord, int64, error) {
	filter.UserID = strings.TrimSpace(filter.UserID)
	filter.UserIDs = compactStrings(filter.UserIDs)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Model = strings.TrimSpace(filter.Model)
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	filters := []string{}
	filterArgs := []any{}
	if filter.UserID != "" {
		filters = append(filters, "generation.user_id = ?")
		filterArgs = append(filterArgs, filter.UserID)
	} else if len(filter.UserIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(filter.UserIDs)), ",")
		filters = append(filters, "generation.user_id IN ("+placeholders+")")
		for _, userID := range filter.UserIDs {
			filterArgs = append(filterArgs, userID)
		}
	}
	if filter.Status != "" {
		filters = append(filters, "generation.status = ?")
		filterArgs = append(filterArgs, filter.Status)
	}
	if filter.Model != "" {
		filters = append(filters, "generation.model = ?")
		filterArgs = append(filterArgs, filter.Model)
	}
	if filter.From != "" {
		filters = append(filters, "generation.created_at >= ?")
		filterArgs = append(filterArgs, filter.From)
	}
	if filter.To != "" {
		filters = append(filters, "generation.created_at <= ?")
		filterArgs = append(filterArgs, filter.To)
	}
	whereSQL := ""
	if len(filters) > 0 {
		whereSQL = " WHERE " + strings.Join(filters, " AND ")
	}

	var total int64
	if err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT COUNT(*)
		 FROM business_image_generations AS generation`+whereSQL),
		filterArgs...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT generation.id, generation.user_id, generation.conversation_id, generation.turn_id,
		        generation.prompt, generation.model, generation.size, generation.quality,
		        generation.count, generation.status, generation.error, generation.created_at,
		        generation.finished_at,
		        COALESCE(SUM(ledger.delta), 0) AS credit_delta
		 FROM business_image_generations AS generation
		 LEFT JOIN business_credit_ledger AS ledger
		   ON ledger.generation_id = generation.id
		  AND ledger.user_id = generation.user_id
		  AND ledger.reason IN (?, ?)`
	args := []any{
		businesscredits.ReasonImageGenerationReserve,
		businesscredits.ReasonImageGenerationRefund,
	}
	args = append(args, filterArgs...)
	query += whereSQL
	query += ` GROUP BY generation.id
		 ORDER BY generation.created_at DESC
		 LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []UsageRecord{}
	for rows.Next() {
		var item UsageRecord
		if err := rows.Scan(
			&item.GenerationID,
			&item.UserID,
			&item.ConversationID,
			&item.TurnID,
			&item.Prompt,
			&item.Model,
			&item.Size,
			&item.Quality,
			&item.Count,
			&item.Status,
			&item.Error,
			&item.CreatedAt,
			&item.FinishedAt,
			&item.CreditDelta,
		); err != nil {
			return nil, 0, err
		}
		item.ID = item.GenerationID
		item.DurationMs = durationMs(item.CreatedAt, item.FinishedAt)
		if item.CreditDelta < 0 {
			item.CreditsUsed = -item.CreditDelta
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func usageTimeFilters(column string, filter UsageSummaryFilter) ([]string, []any) {
	column = strings.TrimSpace(column)
	if column == "" {
		return nil, nil
	}
	filters := []string{}
	args := []any{}
	if strings.TrimSpace(filter.From) != "" {
		filters = append(filters, column+" >= ?")
		args = append(args, strings.TrimSpace(filter.From))
	}
	if strings.TrimSpace(filter.To) != "" {
		filters = append(filters, column+" <= ?")
		args = append(args, strings.TrimSpace(filter.To))
	}
	return filters, args
}

func (s *Store) AssetsByUser(ctx context.Context, userID string, limit int) ([]Asset, error) {
	items, _, err := s.AssetsByUserPage(ctx, userID, limit, 0)
	return items, err
}

func (s *Store) AssetsByGeneration(ctx context.Context, generationID string, userID string) ([]Asset, error) {
	generationID = cleanID(generationID)
	if generationID == "" {
		return nil, fmt.Errorf("generation id is required")
	}
	where := "generation_id = ?"
	args := []any{generationID}
	if uid := strings.TrimSpace(userID); uid != "" {
		where += " AND user_id = ?"
		args = append(args, uid)
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT id, user_id, conversation_id, generation_id, file_name, file_path,
		        url, mime_type, size_bytes, sha256, created_at
		 FROM business_image_assets
		 WHERE `+where+`
		 ORDER BY created_at ASC, file_name ASC`), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Asset{}
	for rows.Next() {
		var item Asset
		if err := rows.Scan(&item.ID, &item.UserID, &item.ConversationID, &item.GenerationID,
			&item.FileName, &item.FilePath, &item.URL, &item.MimeType, &item.SizeBytes,
			&item.SHA256, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AssetsByUserPage(ctx context.Context, userID string, limit int, offset int) ([]Asset, int64, error) {
	userID = strings.TrimSpace(userID)
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
		s.rebind(`SELECT COUNT(*)
		 FROM business_image_assets
		 WHERE user_id = ?`),
		userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT id, user_id, conversation_id, generation_id, file_name, file_path,
		        url, mime_type, size_bytes, sha256, created_at
		 FROM business_image_assets
		 WHERE user_id = ?
		 ORDER BY created_at DESC, file_name DESC
		 LIMIT ? OFFSET ?`),
		userID,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Asset{}
	for rows.Next() {
		var item Asset
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.ConversationID,
			&item.GenerationID,
			&item.FileName,
			&item.FilePath,
			&item.URL,
			&item.MimeType,
			&item.SizeBytes,
			&item.SHA256,
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

func (s *Store) AssetDetailsByUserPage(ctx context.Context, userID string, limit int, offset int) ([]AssetDetail, int64, error) {
	userID = strings.TrimSpace(userID)
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
		s.rebind(`SELECT COUNT(*)
		 FROM business_image_assets
		 WHERE user_id = ?`),
		userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT asset.id, asset.user_id, asset.conversation_id, asset.generation_id,
		        asset.file_name, asset.file_path, asset.url, asset.mime_type, asset.size_bytes,
		        asset.sha256, asset.created_at,
		        COALESCE(conversation.title, '') AS conversation_title,
		        COALESCE(generation.prompt, '') AS prompt,
		        COALESCE(generation.model, '') AS model,
		        COALESCE(generation.size, '') AS size,
		        COALESCE(generation.quality, '') AS quality
		 FROM business_image_assets AS asset
		 LEFT JOIN business_image_conversations AS conversation
		   ON conversation.id = asset.conversation_id
		  AND conversation.user_id = asset.user_id
		 LEFT JOIN business_image_generations AS generation
		   ON generation.id = asset.generation_id
		  AND generation.user_id = asset.user_id
		 WHERE asset.user_id = ?
		 ORDER BY asset.created_at DESC, asset.file_name DESC
		 LIMIT ? OFFSET ?`),
		userID,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []AssetDetail{}
	for rows.Next() {
		var item AssetDetail
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.ConversationID,
			&item.GenerationID,
			&item.FileName,
			&item.FilePath,
			&item.URL,
			&item.MimeType,
			&item.SizeBytes,
			&item.SHA256,
			&item.CreatedAt,
			&item.ConversationTitle,
			&item.Prompt,
			&item.Model,
			&item.Size,
			&item.Quality,
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

func (s *Store) ConversationCount(ctx context.Context, userID string) (int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, fmt.Errorf("user id is required")
	}
	var count int64
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT COUNT(*)
		 FROM business_image_conversations
		 WHERE user_id = ?`),
		userID,
	).Scan(&count)
	return count, err
}

func (s *Store) StorageReportData(ctx context.Context) (StorageReportData, error) {
	assets, err := s.storageAssets(ctx)
	if err != nil {
		return StorageReportData{}, err
	}
	references, err := s.storageReferences(ctx, false)
	if err != nil {
		return StorageReportData{}, err
	}
	legacyReferences, err := s.storageReferences(ctx, true)
	if err != nil {
		return StorageReportData{}, err
	}
	brokenAssets, err := s.brokenAssets(ctx)
	if err != nil {
		return StorageReportData{}, err
	}
	return StorageReportData{
		Assets:           assets,
		References:       references,
		LegacyReferences: legacyReferences,
		BrokenAssets:     brokenAssets,
	}, nil
}

func (s *Store) BackfillLegacyAssets(ctx context.Context, files map[string]LegacyAssetBackfillFile) (LegacyAssetBackfillResult, error) {
	references, err := s.storageReferences(ctx, true)
	if err != nil {
		return LegacyAssetBackfillResult{}, err
	}
	referenceCounts := map[string]int{}
	for _, reference := range references {
		fileName := cleanImageFileName(reference.FileName)
		if fileName == "" {
			continue
		}
		referenceCounts[fileName]++
	}

	result := LegacyAssetBackfillResult{}
	for _, reference := range references {
		fileName := cleanImageFileName(reference.FileName)
		if fileName == "" {
			continue
		}
		result.Matched++
		if referenceCounts[fileName] > 1 {
			result.SkippedAmbiguous++
			continue
		}
		file, ok := files[fileName]
		if !ok {
			result.SkippedMissingFile++
			continue
		}
		if _, err := s.SaveAsset(ctx, Asset{
			ID:             fileName,
			UserID:         reference.UserID,
			ConversationID: reference.ConversationID,
			GenerationID:   reference.GenerationID,
			FileName:       fileName,
			FilePath:       file.FilePath,
			URL:            "/v1/files/image/" + fileName,
			MimeType:       file.MimeType,
			SizeBytes:      file.SizeBytes,
			SHA256:         file.SHA256,
		}); err != nil {
			return result, err
		}
		result.Backfilled++
	}
	return result, nil
}

func (s *Store) storageAssets(ctx context.Context) ([]StorageAsset, error) {
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT user_id, conversation_id, generation_id, file_name, file_path, url, size_bytes
		 FROM business_image_assets
		 ORDER BY created_at ASC, file_name ASC`),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []StorageAsset{}
	for rows.Next() {
		var item StorageAsset
		if err := rows.Scan(
			&item.UserID,
			&item.ConversationID,
			&item.GenerationID,
			&item.FileName,
			&item.FilePath,
			&item.URL,
			&item.SizeBytes,
		); err != nil {
			return nil, err
		}
		item.FileName = cleanImageFileName(item.FileName)
		if item.FileName == "" {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) storageReferences(ctx context.Context, legacyOnly bool) ([]StorageReference, error) {
	existingAssetFiles := map[string]struct{}{}
	if legacyOnly {
		var err error
		existingAssetFiles, err = s.assetFileNameSet(ctx)
		if err != nil {
			return nil, err
		}
	}
	query := `SELECT generation.id, generation.user_id, generation.conversation_id, generation.response_json
		 FROM business_image_generations AS generation
		 WHERE generation.response_json IS NOT NULL
		 ORDER BY generation.created_at ASC, generation.id ASC`
	rows, err := s.db.QueryContext(ctx, s.rebind(query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []StorageReference{}
	seen := map[string]struct{}{}
	for rows.Next() {
		var generationID string
		var userID string
		var conversationID string
		var response []byte
		if err := rows.Scan(&generationID, &userID, &conversationID, &response); err != nil {
			return nil, err
		}
		for _, fileName := range imageFileNamesFromResponse(response) {
			if _, ok := existingAssetFiles[fileName]; ok {
				continue
			}
			key := userID + "\x00" + generationID + "\x00" + fileName
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, StorageReference{
				UserID:         userID,
				ConversationID: conversationID,
				GenerationID:   generationID,
				FileName:       fileName,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) assetFileNameSet(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT file_name
		 FROM business_image_assets`),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := map[string]struct{}{}
	for rows.Next() {
		var fileName string
		if err := rows.Scan(&fileName); err != nil {
			return nil, err
		}
		if cleaned := cleanImageFileName(fileName); cleaned != "" {
			items[cleaned] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) brokenAssets(ctx context.Context) ([]BrokenAsset, error) {
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT asset.file_name, asset.user_id, asset.generation_id
		 FROM business_image_assets AS asset
		 LEFT JOIN business_image_generations AS generation
		   ON generation.id = asset.generation_id
		  AND generation.user_id = asset.user_id
		 WHERE generation.id IS NULL
		 ORDER BY asset.created_at ASC, asset.file_name ASC`),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []BrokenAsset{}
	for rows.Next() {
		var item BrokenAsset
		if err := rows.Scan(&item.FileName, &item.UserID, &item.GenerationID); err != nil {
			return nil, err
		}
		item.FileName = cleanImageFileName(item.FileName)
		if item.FileName == "" {
			continue
		}
		item.Reason = "generation_not_found"
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) DeleteConversation(ctx context.Context, id string, userID string) (DeleteResult, error) {
	id = cleanID(id)
	userID = firstNonEmpty(userID, DevUserID)
	if id == "" {
		return DeleteResult{}, fmt.Errorf("conversation id is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DeleteResult{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	generationResult, err := tx.ExecContext(
		ctx,
		s.rebind(`DELETE FROM business_image_generations
		 WHERE conversation_id = ? AND user_id = ?`),
		id,
		userID,
	)
	if err != nil {
		return DeleteResult{}, err
	}
	conversationResult, err := tx.ExecContext(
		ctx,
		s.rebind(`DELETE FROM business_image_conversations
		 WHERE id = ? AND user_id = ?`),
		id,
		userID,
	)
	if err != nil {
		return DeleteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return DeleteResult{}, err
	}
	tx = nil

	generations, _ := generationResult.RowsAffected()
	conversations, _ := conversationResult.RowsAffected()
	return DeleteResult{
		Conversations: conversations,
		Generations:   generations,
	}, nil
}

func (s *Store) RenameConversation(ctx context.Context, id string, userID string, title string) (Conversation, bool, error) {
	id = cleanID(id)
	userID = firstNonEmpty(userID, DevUserID)
	title = strings.TrimSpace(title)
	if id == "" {
		return Conversation{}, false, fmt.Errorf("conversation id is required")
	}
	if title == "" {
		return Conversation{}, false, fmt.Errorf("conversation title is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_image_conversations
		 SET title = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`),
		title,
		now,
		id,
		userID,
	)
	if err != nil {
		return Conversation{}, false, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return Conversation{}, false, nil
	}
	return s.GetConversation(ctx, id, userID)
}

func (s *Store) ClearConversations(ctx context.Context, userID string) (DeleteResult, error) {
	userID = firstNonEmpty(userID, DevUserID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DeleteResult{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	generationResult, err := tx.ExecContext(
		ctx,
		s.rebind(`DELETE FROM business_image_generations
		 WHERE user_id = ?`),
		userID,
	)
	if err != nil {
		return DeleteResult{}, err
	}
	conversationResult, err := tx.ExecContext(
		ctx,
		s.rebind(`DELETE FROM business_image_conversations
		 WHERE user_id = ?`),
		userID,
	)
	if err != nil {
		return DeleteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return DeleteResult{}, err
	}
	tx = nil

	generations, _ := generationResult.RowsAffected()
	conversations, _ := conversationResult.RowsAffected()
	return DeleteResult{
		Conversations: conversations,
		Generations:   generations,
	}, nil
}

func parseListGenerationsArgs(args ...any) (string, int) {
	userID := ""
	limit := 0
	for _, arg := range args {
		switch typed := arg.(type) {
		case string:
			userID = strings.TrimSpace(typed)
		case int:
			limit = typed
		}
	}
	return userID, limit
}

type responseRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func collectImageFileNames(rows responseRows) ([]string, error) {
	seen := map[string]struct{}{}
	for rows.Next() {
		var response []byte
		if err := rows.Scan(&response); err != nil {
			return nil, err
		}
		for _, fileName := range imageFileNamesFromResponse(response) {
			seen[fileName] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]string, 0, len(seen))
	for fileName := range seen {
		items = append(items, fileName)
	}
	return items, nil
}

func collectAssetFileNames(rows responseRows) ([]string, error) {
	seen := map[string]struct{}{}
	for rows.Next() {
		var fileName string
		if err := rows.Scan(&fileName); err != nil {
			return nil, err
		}
		if cleaned := cleanImageFileName(fileName); cleaned != "" {
			seen[cleaned] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]string, 0, len(seen))
	for fileName := range seen {
		items = append(items, fileName)
	}
	return items, nil
}

func mergeFileNameSlices(groups ...[]string) []string {
	seen := map[string]struct{}{}
	items := []string{}
	for _, group := range groups {
		for _, fileName := range group {
			cleaned := cleanImageFileName(fileName)
			if cleaned == "" {
				continue
			}
			if _, ok := seen[cleaned]; ok {
				continue
			}
			seen[cleaned] = struct{}{}
			items = append(items, cleaned)
		}
	}
	return items
}

func (s *Store) addLegacyImageCounts(ctx context.Context, usages map[string]UserUsage) error {
	assetFileNames, err := s.assetFileNameSet(ctx)
	if err != nil {
		return err
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT generation.user_id, generation.response_json
		 FROM business_image_generations AS generation
		 WHERE generation.response_json IS NOT NULL`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	legacyCounts := map[string]int64{}
	for rows.Next() {
		var userID string
		var response []byte
		if err := rows.Scan(&userID, &response); err != nil {
			return err
		}
		count := legacyImageItemCountFromResponse(response, assetFileNames)
		if count <= 0 {
			continue
		}
		legacyCounts[userID] += int64(count)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for userID, legacyCount := range legacyCounts {
		usage := usages[userID]
		usage.UserID = userID
		usage.ImageCount += legacyCount
		usages[userID] = usage
	}
	return nil
}

func legacyImageItemCountFromResponse(response []byte, assetFileNames map[string]struct{}) int {
	if len(response) == 0 {
		return 0
	}
	var payload struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response, &payload); err != nil {
		return 0
	}
	count := 0
	for _, item := range payload.Data {
		fileName := imageFileNameFromURL(item.URL)
		if fileName != "" {
			if _, ok := assetFileNames[fileName]; !ok {
				count++
			}
			continue
		}
		if strings.TrimSpace(item.B64JSON) != "" {
			count++
		}
	}
	return count
}

func imageItemCountFromResponse(response []byte) int {
	if len(response) == 0 {
		return 0
	}
	var payload struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response, &payload); err != nil {
		return 0
	}
	count := 0
	for _, item := range payload.Data {
		if strings.TrimSpace(item.URL) != "" || strings.TrimSpace(item.B64JSON) != "" {
			count++
		}
	}
	return count
}

func imageFileNamesFromResponse(response []byte) []string {
	if len(response) == 0 {
		return nil
	}
	var payload struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response, &payload); err != nil {
		return nil
	}
	items := make([]string, 0, len(payload.Data))
	seen := map[string]struct{}{}
	for _, item := range payload.Data {
		fileName := imageFileNameFromURL(item.URL)
		if fileName == "" {
			continue
		}
		if _, ok := seen[fileName]; ok {
			continue
		}
		seen[fileName] = struct{}{}
		items = append(items, fileName)
	}
	return items
}

func imageFileNameFromURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	const marker = "/v1/files/image/"
	index := strings.LastIndex(trimmed, marker)
	if index < 0 {
		return ""
	}
	return cleanImageFileName(trimmed[index+len(marker):])
}

func cleanImageFileName(name string) string {
	baseName := filepath.Base(strings.TrimSpace(name))
	if !strings.HasPrefix(baseName, "business-") {
		return ""
	}
	return baseName
}

func cleanID(id string) string {
	return strings.ReplaceAll(strings.TrimSpace(id), "/", "-")
}

func durationMs(startValue string, endValue string) int64 {
	start, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(startValue))
	if err != nil {
		return 0
	}
	end, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(endValue))
	if err != nil {
		return 0
	}
	duration := end.Sub(start).Milliseconds()
	if duration < 0 {
		return 0
	}
	return duration
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func (s *Store) isPostgres() bool {
	return database.IsPostgres(s.driver)
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}
