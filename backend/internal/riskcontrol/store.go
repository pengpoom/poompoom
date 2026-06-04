package riskcontrol

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
	configKey = "risk_control"

	ModeObserve  = "observe"
	ModePreBlock = "pre_block"

	ProviderOpenAI = "openai"

	ActionAllow = "allow"
	ActionBlock = "block"
	ActionError = "error"

	DecisionErrorUnavailable = "risk_control_unavailable"
	DecisionErrorConfig      = "risk_control_config_error"
)

type Config struct {
	Enabled       bool               `json:"enabled"`
	Mode          string             `json:"mode"`
	Provider      string             `json:"provider"`
	BaseURL       string             `json:"baseUrl"`
	APIKey        string             `json:"apiKey,omitempty"`
	Model         string             `json:"model"`
	TimeoutMS     int                `json:"timeoutMs"`
	RecordNonHits bool               `json:"recordNonHits"`
	BlockMessage  string             `json:"blockMessage"`
	Thresholds    map[string]float64 `json:"thresholds"`
}

type ConfigView struct {
	Enabled          bool               `json:"enabled"`
	Mode             string             `json:"mode"`
	Provider         string             `json:"provider"`
	BaseURL          string             `json:"baseUrl"`
	Model            string             `json:"model"`
	APIKeyConfigured bool               `json:"apiKeyConfigured"`
	APIKeyMasked     string             `json:"apiKeyMasked"`
	TimeoutMS        int                `json:"timeoutMs"`
	RecordNonHits    bool               `json:"recordNonHits"`
	BlockMessage     string             `json:"blockMessage"`
	Thresholds       map[string]float64 `json:"thresholds"`
}

type UpdateConfigInput struct {
	Enabled       *bool               `json:"enabled"`
	Mode          *string             `json:"mode"`
	Provider      *string             `json:"provider"`
	BaseURL       *string             `json:"baseUrl"`
	APIKey        *string             `json:"apiKey"`
	ClearAPIKey   bool                `json:"clearApiKey"`
	Model         *string             `json:"model"`
	TimeoutMS     *int                `json:"timeoutMs"`
	RecordNonHits *bool               `json:"recordNonHits"`
	BlockMessage  *string             `json:"blockMessage"`
	Thresholds    *map[string]float64 `json:"thresholds"`
}

type Log struct {
	ID              string             `json:"id"`
	UserID          string             `json:"userId"`
	JobID           string             `json:"jobId"`
	ConversationID  string             `json:"conversationId"`
	TurnID          string             `json:"turnId"`
	Platform        string             `json:"platform"`
	Model           string             `json:"model"`
	Mode            string             `json:"mode"`
	Action          string             `json:"action"`
	Flagged         bool               `json:"flagged"`
	HighestCategory string             `json:"highestCategory"`
	HighestScore    float64            `json:"highestScore"`
	CategoryScores  map[string]float64 `json:"categoryScores"`
	InputExcerpt    string             `json:"inputExcerpt"`
	Error           string             `json:"error"`
	LatencyMS       int64              `json:"latencyMs"`
	CreatedAt       string             `json:"createdAt"`
}

type LogInput struct {
	UserID          string
	JobID           string
	ConversationID  string
	TurnID          string
	Platform        string
	Model           string
	Mode            string
	Action          string
	Flagged         bool
	HighestCategory string
	HighestScore    float64
	CategoryScores  map[string]float64
	InputExcerpt    string
	Error           string
	LatencyMS       int64
}

type ListFilter struct {
	Result   string
	Platform string
	Search   string
	From     string
	To       string
	Limit    int
	Offset   int
}

type Status struct {
	Enabled          bool   `json:"enabled"`
	Mode             string `json:"mode"`
	Provider         string `json:"provider"`
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
	Last24hTotal     int64  `json:"last24hTotal"`
	Last24hFlagged   int64  `json:"last24hFlagged"`
	Last24hBlocked   int64  `json:"last24hBlocked"`
	Last24hErrors    int64  `json:"last24hErrors"`
}

type Store struct {
	db     *sql.DB
	driver string
	ownDB  bool
}

func NewStore(cfg *config.Config) (*Store, error) {
	if !database.IsPostgres(cfg.Database.Driver) {
		return nil, newRiskControlConfigError(fmt.Sprintf("unsupported database driver %q", strings.TrimSpace(cfg.Database.Driver)))
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

func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		Mode:          ModeObserve,
		Provider:      ProviderOpenAI,
		BaseURL:       "https://api.openai.com",
		APIKey:        "",
		Model:         "omni-moderation-latest",
		TimeoutMS:     3000,
		RecordNonHits: false,
		BlockMessage:  "内容审计命中风险规则，请调整输入后重试",
		Thresholds: map[string]float64{
			"harassment":             0.98,
			"harassment/threatening": 0.90,
			"hate":                   0.65,
			"hate/threatening":       0.65,
			"illicit":                0.95,
			"illicit/violent":        0.95,
			"self-harm":              0.65,
			"self-harm/intent":       0.85,
			"self-harm/instructions": 0.65,
			"sexual":                 0.65,
			"sexual/minors":          0.65,
			"violence":               0.95,
			"violence/graphic":       0.95,
		},
	}
}

func NormalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode != ModePreBlock {
		cfg.Mode = ModeObserve
	}
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if cfg.Provider != ProviderOpenAI {
		cfg.Provider = ProviderOpenAI
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaults.BaseURL
	}
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	if cfg.TimeoutMS < 500 {
		cfg.TimeoutMS = defaults.TimeoutMS
	}
	if cfg.TimeoutMS > 30000 {
		cfg.TimeoutMS = 30000
	}
	cfg.BlockMessage = strings.TrimSpace(cfg.BlockMessage)
	if cfg.BlockMessage == "" {
		cfg.BlockMessage = defaults.BlockMessage
	}
	cfg.Thresholds = normalizeThresholds(cfg.Thresholds, defaults.Thresholds)
	return cfg
}

func (c Config) View() ConfigView {
	c = NormalizeConfig(c)
	return ConfigView{
		Enabled:          c.Enabled,
		Mode:             c.Mode,
		Provider:         c.Provider,
		BaseURL:          c.BaseURL,
		Model:            c.Model,
		APIKeyConfigured: c.APIKey != "",
		APIKeyMasked:     MaskSecret(c.APIKey),
		TimeoutMS:        c.TimeoutMS,
		RecordNonHits:    c.RecordNonHits,
		BlockMessage:     c.BlockMessage,
		Thresholds:       c.Thresholds,
	}
}

func (s *Store) GetConfig(ctx context.Context) (Config, error) {
	cfg := DefaultConfig()
	var raw []byte
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT value_json FROM business_system_settings WHERE key = ?`), configKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return Config{}, wrapRiskControlConfigError(err)
		}
	}
	return NormalizeConfig(cfg), nil
}

func (s *Store) UpdateConfig(ctx context.Context, input UpdateConfigInput) (Config, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return Config{}, err
	}
	if input.Enabled != nil {
		cfg.Enabled = *input.Enabled
	}
	if input.Mode != nil {
		cfg.Mode = *input.Mode
	}
	if input.Provider != nil {
		cfg.Provider = *input.Provider
	}
	if input.BaseURL != nil {
		cfg.BaseURL = *input.BaseURL
	}
	if input.ClearAPIKey {
		cfg.APIKey = ""
	} else if input.APIKey != nil {
		nextKey := strings.TrimSpace(*input.APIKey)
		if nextKey != "" {
			cfg.APIKey = nextKey
		}
	}
	if input.Model != nil {
		cfg.Model = *input.Model
	}
	if input.TimeoutMS != nil {
		cfg.TimeoutMS = *input.TimeoutMS
	}
	if input.RecordNonHits != nil {
		cfg.RecordNonHits = *input.RecordNonHits
	}
	if input.BlockMessage != nil {
		cfg.BlockMessage = *input.BlockMessage
	}
	if input.Thresholds != nil {
		cfg.Thresholds = *input.Thresholds
	}
	cfg = NormalizeConfig(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return Config{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_system_settings(key, value_json, created_at, updated_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`), configKey, raw, now, now)
	return cfg, err
}

func (s *Store) InsertLog(ctx context.Context, input LogInput) (Log, error) {
	item := Log{
		ID:              newLogID(),
		UserID:          strings.TrimSpace(input.UserID),
		JobID:           strings.TrimSpace(input.JobID),
		ConversationID:  strings.TrimSpace(input.ConversationID),
		TurnID:          strings.TrimSpace(input.TurnID),
		Platform:        strings.TrimSpace(input.Platform),
		Model:           strings.TrimSpace(input.Model),
		Mode:            strings.TrimSpace(input.Mode),
		Action:          strings.TrimSpace(input.Action),
		Flagged:         input.Flagged,
		HighestCategory: strings.TrimSpace(input.HighestCategory),
		HighestScore:    input.HighestScore,
		CategoryScores:  copyScores(input.CategoryScores),
		InputExcerpt:    strings.TrimSpace(input.InputExcerpt),
		Error:           strings.TrimSpace(input.Error),
		LatencyMS:       input.LatencyMS,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	if item.Action == "" {
		item.Action = ActionAllow
	}
	if item.Mode == "" {
		item.Mode = ModeObserve
	}
	if item.LatencyMS < 0 {
		item.LatencyMS = 0
	}
	rawScores, err := json.Marshal(item.CategoryScores)
	if err != nil {
		return Log{}, err
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_risk_control_logs(
		id, user_id, job_id, conversation_id, turn_id, platform, model, mode, action,
		flagged, highest_category, highest_score, category_scores_json, input_excerpt,
		error, latency_ms, created_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		item.ID,
		item.UserID,
		item.JobID,
		item.ConversationID,
		item.TurnID,
		item.Platform,
		item.Model,
		item.Mode,
		item.Action,
		item.Flagged,
		item.HighestCategory,
		item.HighestScore,
		rawScores,
		item.InputExcerpt,
		item.Error,
		item.LatencyMS,
		item.CreatedAt,
	)
	return item, err
}

func (s *Store) ListLogs(ctx context.Context, filter ListFilter) ([]Log, int64, error) {
	where, args := buildLogWhere(filter)
	totalQuery := `SELECT COUNT(*) FROM business_risk_control_logs` + where
	var total int64
	if err := s.db.QueryRowContext(ctx, s.rebind(totalQuery), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	query := `SELECT id, user_id, job_id, conversation_id, turn_id, platform, model, mode, action,
		flagged, highest_category, highest_score, category_scores_json, input_excerpt,
		error, latency_ms, created_at
		FROM business_risk_control_logs` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Log, 0, limit)
	for rows.Next() {
		item, err := scanLog(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) Status(ctx context.Context) (Status, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return Status{}, err
	}
	status := Status{
		Enabled:          cfg.Enabled,
		Mode:             cfg.Mode,
		Provider:         cfg.Provider,
		APIKeyConfigured: cfg.APIKey != "",
	}
	since := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339Nano)
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN flagged THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN action = ? THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN action = ? THEN 1 ELSE 0 END), 0)
		FROM business_risk_control_logs WHERE created_at >= ?`), ActionBlock, ActionError, since).
		Scan(&status.Last24hTotal, &status.Last24hFlagged, &status.Last24hBlocked, &status.Last24hErrors)
	return status, err
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}

func buildLogWhere(filter ListFilter) (string, []any) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	switch strings.ToLower(strings.TrimSpace(filter.Result)) {
	case "flagged":
		clauses = append(clauses, "flagged = TRUE")
	case "blocked":
		clauses = append(clauses, "action = ?")
		args = append(args, ActionBlock)
	case "error":
		clauses = append(clauses, "action = ?")
		args = append(args, ActionError)
	case "allowed":
		clauses = append(clauses, "action = ?")
		args = append(args, ActionAllow)
	}
	if platform := strings.TrimSpace(filter.Platform); platform != "" {
		clauses = append(clauses, "platform = ?")
		args = append(args, platform)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		clauses = append(clauses, "(LOWER(input_excerpt) LIKE ? OR LOWER(user_id) LIKE ? OR LOWER(model) LIKE ?)")
		args = append(args, like, like, like)
	}
	if from := strings.TrimSpace(filter.From); from != "" {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, from)
	}
	if to := strings.TrimSpace(filter.To); to != "" {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, to)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func scanLog(scanner interface {
	Scan(dest ...any) error
}) (Log, error) {
	var item Log
	var rawScores []byte
	err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.JobID,
		&item.ConversationID,
		&item.TurnID,
		&item.Platform,
		&item.Model,
		&item.Mode,
		&item.Action,
		&item.Flagged,
		&item.HighestCategory,
		&item.HighestScore,
		&rawScores,
		&item.InputExcerpt,
		&item.Error,
		&item.LatencyMS,
		&item.CreatedAt,
	)
	if err != nil {
		return Log{}, err
	}
	item.CategoryScores = map[string]float64{}
	if len(rawScores) > 0 {
		_ = json.Unmarshal(rawScores, &item.CategoryScores)
	}
	return item, nil
}

func normalizeThresholds(values map[string]float64, defaults map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(defaults)+len(values))
	for key, value := range defaults {
		out[key] = value
	}
	for key, value := range values {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}
		out[key] = value
	}
	return out
}

func copyScores(values map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func MaskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}

func newLogID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "risk_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("risk_%d", time.Now().UnixNano())
}
