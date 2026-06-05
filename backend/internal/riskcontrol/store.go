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
	ProviderAliyun = "aliyun"

	FailModeClosed = "fail_closed"
	FailModeOpen   = "fail_open"

	ActionAllow = "allow"
	ActionBlock = "block"
	ActionError = "error"

	DecisionErrorUnavailable = "risk_control_unavailable"
	DecisionErrorConfig      = "risk_control_config_error"

	ScopeGlobal = "global"
	ScopePlan   = "plan"
	ScopeUser   = "user"
	ScopeAPIKey = "api_key"

	RiskLevelLow    = "low"
	RiskLevelMedium = "medium"
	RiskLevelHigh   = "high"
)

type Config struct {
	Enabled               bool               `json:"enabled"`
	Mode                  string             `json:"mode"`
	Provider              string             `json:"provider"`
	ProviderChain         []string           `json:"providerChain"`
	FailMode              string             `json:"failMode"`
	RiskLevel             string             `json:"riskLevel,omitempty"`
	BaseURL               string             `json:"baseUrl"`
	APIKey                string             `json:"apiKey,omitempty"`
	Model                 string             `json:"model"`
	OpenAIBaseURL         string             `json:"openaiBaseUrl"`
	OpenAIAPIKey          string             `json:"openaiApiKey,omitempty"`
	OpenAIModel           string             `json:"openaiModel"`
	AliyunAccessKeyID     string             `json:"aliyunAccessKeyId,omitempty"`
	AliyunAccessKeySecret string             `json:"aliyunAccessKeySecret,omitempty"`
	AliyunRegionID        string             `json:"aliyunRegionId"`
	AliyunEndpoint        string             `json:"aliyunEndpoint"`
	AliyunTextService     string             `json:"aliyunTextService"`
	AliyunBlockRiskLevel  string             `json:"aliyunBlockRiskLevel"`
	TimeoutMS             int                `json:"timeoutMs"`
	RecordNonHits         bool               `json:"recordNonHits"`
	BlockMessage          string             `json:"blockMessage"`
	Thresholds            map[string]float64 `json:"thresholds"`
}

type ConfigView struct {
	Enabled                         bool               `json:"enabled"`
	Mode                            string             `json:"mode"`
	Provider                        string             `json:"provider"`
	ProviderChain                   []string           `json:"providerChain"`
	FailMode                        string             `json:"failMode"`
	RiskLevel                       string             `json:"riskLevel"`
	BaseURL                         string             `json:"baseUrl"`
	Model                           string             `json:"model"`
	APIKeyConfigured                bool               `json:"apiKeyConfigured"`
	APIKeyMasked                    string             `json:"apiKeyMasked"`
	OpenAIBaseURL                   string             `json:"openaiBaseUrl"`
	OpenAIModel                     string             `json:"openaiModel"`
	OpenAIAPIKeyConfigured          bool               `json:"openaiApiKeyConfigured"`
	OpenAIAPIKeyMasked              string             `json:"openaiApiKeyMasked"`
	AliyunAccessKeyID               string             `json:"aliyunAccessKeyId"`
	AliyunAccessKeySecretMasked     string             `json:"aliyunAccessKeySecretMasked"`
	AliyunAccessKeySecretConfigured bool               `json:"aliyunAccessKeySecretConfigured"`
	AliyunRegionID                  string             `json:"aliyunRegionId"`
	AliyunEndpoint                  string             `json:"aliyunEndpoint"`
	AliyunTextService               string             `json:"aliyunTextService"`
	AliyunBlockRiskLevel            string             `json:"aliyunBlockRiskLevel"`
	TimeoutMS                       int                `json:"timeoutMs"`
	RecordNonHits                   bool               `json:"recordNonHits"`
	BlockMessage                    string             `json:"blockMessage"`
	Thresholds                      map[string]float64 `json:"thresholds"`
}

type Policy struct {
	ID           string             `json:"id"`
	Scope        string             `json:"scope"`
	TargetID     string             `json:"targetId"`
	Enabled      bool               `json:"enabled"`
	Mode         string             `json:"mode,omitempty"`
	RiskLevel    string             `json:"riskLevel,omitempty"`
	BlockMessage string             `json:"blockMessage,omitempty"`
	Thresholds   map[string]float64 `json:"thresholds,omitempty"`
	CreatedAt    string             `json:"createdAt"`
	UpdatedAt    string             `json:"updatedAt"`
}

type PolicyInput struct {
	Scope        string              `json:"scope"`
	TargetID     string              `json:"targetId"`
	Enabled      *bool               `json:"enabled"`
	Mode         *string             `json:"mode"`
	RiskLevel    *string             `json:"riskLevel"`
	BlockMessage *string             `json:"blockMessage"`
	Thresholds   *map[string]float64 `json:"thresholds"`
}

type PolicyFilter struct {
	Scope    string
	TargetID string
}

type PolicyResolutionInput struct {
	UserID   string
	PlanTag  string
	APIKeyID string
}

type PolicyResolution struct {
	Config        Config   `json:"config"`
	AppliedScopes []string `json:"appliedScopes"`
}

type UpdateConfigInput struct {
	Enabled                    *bool               `json:"enabled"`
	Mode                       *string             `json:"mode"`
	Provider                   *string             `json:"provider"`
	ProviderChain              *[]string           `json:"providerChain"`
	FailMode                   *string             `json:"failMode"`
	BaseURL                    *string             `json:"baseUrl"`
	APIKey                     *string             `json:"apiKey"`
	ClearAPIKey                bool                `json:"clearApiKey"`
	Model                      *string             `json:"model"`
	OpenAIBaseURL              *string             `json:"openaiBaseUrl"`
	OpenAIAPIKey               *string             `json:"openaiApiKey"`
	ClearOpenAIAPIKey          bool                `json:"clearOpenaiApiKey"`
	OpenAIModel                *string             `json:"openaiModel"`
	AliyunAccessKeyID          *string             `json:"aliyunAccessKeyId"`
	AliyunAccessKeySecret      *string             `json:"aliyunAccessKeySecret"`
	ClearAliyunAccessKeySecret bool                `json:"clearAliyunAccessKeySecret"`
	AliyunRegionID             *string             `json:"aliyunRegionId"`
	AliyunEndpoint             *string             `json:"aliyunEndpoint"`
	AliyunTextService          *string             `json:"aliyunTextService"`
	AliyunBlockRiskLevel       *string             `json:"aliyunBlockRiskLevel"`
	TimeoutMS                  *int                `json:"timeoutMs"`
	RecordNonHits              *bool               `json:"recordNonHits"`
	BlockMessage               *string             `json:"blockMessage"`
	Thresholds                 *map[string]float64 `json:"thresholds"`
}

type Log struct {
	ID                string             `json:"id"`
	UserID            string             `json:"userId"`
	JobID             string             `json:"jobId"`
	ConversationID    string             `json:"conversationId"`
	TurnID            string             `json:"turnId"`
	Platform          string             `json:"platform"`
	Model             string             `json:"model"`
	Mode              string             `json:"mode"`
	Provider          string             `json:"provider"`
	RiskLevel         string             `json:"riskLevel"`
	ProviderReason    string             `json:"providerReason"`
	ProviderLatencyMS int64              `json:"providerLatencyMs"`
	Action            string             `json:"action"`
	Flagged           bool               `json:"flagged"`
	HighestCategory   string             `json:"highestCategory"`
	HighestScore      float64            `json:"highestScore"`
	CategoryScores    map[string]float64 `json:"categoryScores"`
	InputExcerpt      string             `json:"inputExcerpt"`
	Error             string             `json:"error"`
	LatencyMS         int64              `json:"latencyMs"`
	CreatedAt         string             `json:"createdAt"`
}

type LogInput struct {
	UserID            string
	JobID             string
	ConversationID    string
	TurnID            string
	Platform          string
	Model             string
	Mode              string
	Provider          string
	RiskLevel         string
	ProviderReason    string
	ProviderLatencyMS int64
	Action            string
	Flagged           bool
	HighestCategory   string
	HighestScore      float64
	CategoryScores    map[string]float64
	InputExcerpt      string
	Error             string
	LatencyMS         int64
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
	Enabled          bool     `json:"enabled"`
	Mode             string   `json:"mode"`
	Provider         string   `json:"provider"`
	ProviderChain    []string `json:"providerChain"`
	APIKeyConfigured bool     `json:"apiKeyConfigured"`
	Last24hTotal     int64    `json:"last24hTotal"`
	Last24hFlagged   int64    `json:"last24hFlagged"`
	Last24hBlocked   int64    `json:"last24hBlocked"`
	Last24hErrors    int64    `json:"last24hErrors"`
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
		Enabled:               false,
		Mode:                  ModeObserve,
		Provider:              ProviderOpenAI,
		ProviderChain:         []string{ProviderOpenAI},
		FailMode:              FailModeClosed,
		RiskLevel:             RiskLevelLow,
		BaseURL:               "https://api.openai.com",
		APIKey:                "",
		Model:                 "omni-moderation-latest",
		OpenAIBaseURL:         "https://api.openai.com",
		OpenAIAPIKey:          "",
		OpenAIModel:           "omni-moderation-latest",
		AliyunAccessKeyID:     "",
		AliyunAccessKeySecret: "",
		AliyunRegionID:        "ap-southeast-1",
		AliyunEndpoint:        "green-cip.ap-southeast-1.aliyuncs.com",
		AliyunTextService:     "ugc_moderation_byllm_cb",
		AliyunBlockRiskLevel:  RiskLevelLow,
		TimeoutMS:             3000,
		RecordNonHits:         false,
		BlockMessage:          "内容审计命中风险规则，请调整输入后重试",
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
	cfg.Provider = normalizeProvider(cfg.Provider)
	if cfg.Provider == "" {
		cfg.Provider = normalizeProvider(defaults.Provider)
	}
	cfg.ProviderChain = normalizeProviderChain(cfg.ProviderChain, cfg.Provider)
	if len(cfg.ProviderChain) == 0 {
		cfg.ProviderChain = append([]string(nil), defaults.ProviderChain...)
	}
	cfg.Provider = cfg.ProviderChain[0]
	cfg.FailMode = normalizeFailMode(cfg.FailMode)
	if cfg.FailMode == "" {
		cfg.FailMode = defaults.FailMode
	}
	cfg.RiskLevel = normalizeRiskLevel(cfg.RiskLevel)
	if cfg.RiskLevel == "" {
		cfg.RiskLevel = defaults.RiskLevel
	}

	cfg.BaseURL = normalizeBaseURL(cfg.BaseURL, defaults.BaseURL)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	cfg.OpenAIBaseURL = normalizeBaseURL(firstNonEmpty(cfg.OpenAIBaseURL, cfg.BaseURL), defaults.OpenAIBaseURL)
	cfg.OpenAIAPIKey = strings.TrimSpace(firstNonEmpty(cfg.OpenAIAPIKey, cfg.APIKey))
	cfg.OpenAIModel = strings.TrimSpace(firstNonEmpty(cfg.OpenAIModel, cfg.Model))
	if cfg.OpenAIModel == "" {
		cfg.OpenAIModel = defaults.OpenAIModel
	}
	cfg.BaseURL = cfg.OpenAIBaseURL
	cfg.APIKey = cfg.OpenAIAPIKey
	cfg.Model = cfg.OpenAIModel

	cfg.AliyunAccessKeyID = strings.TrimSpace(cfg.AliyunAccessKeyID)
	cfg.AliyunAccessKeySecret = strings.TrimSpace(cfg.AliyunAccessKeySecret)
	cfg.AliyunRegionID = strings.TrimSpace(cfg.AliyunRegionID)
	if cfg.AliyunRegionID == "" {
		cfg.AliyunRegionID = defaults.AliyunRegionID
	}
	cfg.AliyunEndpoint = strings.TrimSpace(cfg.AliyunEndpoint)
	if cfg.AliyunEndpoint == "" {
		cfg.AliyunEndpoint = defaults.AliyunEndpoint
	}
	cfg.AliyunTextService = strings.TrimSpace(cfg.AliyunTextService)
	if cfg.AliyunTextService == "" {
		cfg.AliyunTextService = defaults.AliyunTextService
	}
	cfg.AliyunBlockRiskLevel = normalizeRiskLevel(cfg.AliyunBlockRiskLevel)
	if cfg.AliyunBlockRiskLevel == "" {
		cfg.AliyunBlockRiskLevel = defaults.AliyunBlockRiskLevel
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
		Enabled:                         c.Enabled,
		Mode:                            c.Mode,
		Provider:                        c.Provider,
		ProviderChain:                   append([]string(nil), c.ProviderChain...),
		FailMode:                        c.FailMode,
		RiskLevel:                       c.RiskLevel,
		BaseURL:                         c.BaseURL,
		Model:                           c.Model,
		APIKeyConfigured:                c.APIKey != "",
		APIKeyMasked:                    MaskSecret(c.APIKey),
		OpenAIBaseURL:                   c.OpenAIBaseURL,
		OpenAIModel:                     c.OpenAIModel,
		OpenAIAPIKeyConfigured:          c.OpenAIAPIKey != "",
		OpenAIAPIKeyMasked:              MaskSecret(c.OpenAIAPIKey),
		AliyunAccessKeyID:               c.AliyunAccessKeyID,
		AliyunAccessKeySecretMasked:     MaskSecret(c.AliyunAccessKeySecret),
		AliyunAccessKeySecretConfigured: c.AliyunAccessKeySecret != "",
		AliyunRegionID:                  c.AliyunRegionID,
		AliyunEndpoint:                  c.AliyunEndpoint,
		AliyunTextService:               c.AliyunTextService,
		AliyunBlockRiskLevel:            c.AliyunBlockRiskLevel,
		TimeoutMS:                       c.TimeoutMS,
		RecordNonHits:                   c.RecordNonHits,
		BlockMessage:                    c.BlockMessage,
		Thresholds:                      c.Thresholds,
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
	if input.ProviderChain != nil {
		cfg.ProviderChain = *input.ProviderChain
	}
	if input.FailMode != nil {
		cfg.FailMode = *input.FailMode
	}
	if input.BaseURL != nil {
		cfg.BaseURL = *input.BaseURL
	}
	if input.ClearAPIKey {
		cfg.APIKey = ""
		cfg.OpenAIAPIKey = ""
	} else if input.APIKey != nil {
		nextKey := strings.TrimSpace(*input.APIKey)
		if nextKey != "" {
			cfg.APIKey = nextKey
			cfg.OpenAIAPIKey = nextKey
		}
	}
	if input.Model != nil {
		cfg.Model = *input.Model
	}
	if input.OpenAIBaseURL != nil {
		cfg.OpenAIBaseURL = *input.OpenAIBaseURL
	}
	if input.ClearOpenAIAPIKey {
		cfg.OpenAIAPIKey = ""
		cfg.APIKey = ""
	} else if input.OpenAIAPIKey != nil {
		nextKey := strings.TrimSpace(*input.OpenAIAPIKey)
		if nextKey != "" {
			cfg.OpenAIAPIKey = nextKey
			cfg.APIKey = nextKey
		}
	}
	if input.OpenAIModel != nil {
		cfg.OpenAIModel = *input.OpenAIModel
	}
	if input.AliyunAccessKeyID != nil {
		cfg.AliyunAccessKeyID = *input.AliyunAccessKeyID
	}
	if input.ClearAliyunAccessKeySecret {
		cfg.AliyunAccessKeySecret = ""
	} else if input.AliyunAccessKeySecret != nil {
		nextSecret := strings.TrimSpace(*input.AliyunAccessKeySecret)
		if nextSecret != "" {
			cfg.AliyunAccessKeySecret = nextSecret
		}
	}
	if input.AliyunRegionID != nil {
		cfg.AliyunRegionID = *input.AliyunRegionID
	}
	if input.AliyunEndpoint != nil {
		cfg.AliyunEndpoint = *input.AliyunEndpoint
	}
	if input.AliyunTextService != nil {
		cfg.AliyunTextService = *input.AliyunTextService
	}
	if input.AliyunBlockRiskLevel != nil {
		cfg.AliyunBlockRiskLevel = *input.AliyunBlockRiskLevel
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
		ID:                newLogID(),
		UserID:            strings.TrimSpace(input.UserID),
		JobID:             strings.TrimSpace(input.JobID),
		ConversationID:    strings.TrimSpace(input.ConversationID),
		TurnID:            strings.TrimSpace(input.TurnID),
		Platform:          strings.TrimSpace(input.Platform),
		Model:             strings.TrimSpace(input.Model),
		Mode:              strings.TrimSpace(input.Mode),
		Provider:          normalizeProvider(input.Provider),
		RiskLevel:         normalizeRiskLevel(input.RiskLevel),
		ProviderReason:    strings.TrimSpace(input.ProviderReason),
		ProviderLatencyMS: input.ProviderLatencyMS,
		Action:            strings.TrimSpace(input.Action),
		Flagged:           input.Flagged,
		HighestCategory:   strings.TrimSpace(input.HighestCategory),
		HighestScore:      input.HighestScore,
		CategoryScores:    copyScores(input.CategoryScores),
		InputExcerpt:      strings.TrimSpace(input.InputExcerpt),
		Error:             strings.TrimSpace(input.Error),
		LatencyMS:         input.LatencyMS,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339Nano),
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
	if item.ProviderLatencyMS < 0 {
		item.ProviderLatencyMS = 0
	}
	rawScores, err := json.Marshal(item.CategoryScores)
	if err != nil {
		return Log{}, err
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_risk_control_logs(
		id, user_id, job_id, conversation_id, turn_id, platform, model, mode,
		moderation_provider, risk_level, provider_reason, provider_latency_ms, action,
		flagged, highest_category, highest_score, category_scores_json, input_excerpt,
		error, latency_ms, created_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		item.ID,
		item.UserID,
		item.JobID,
		item.ConversationID,
		item.TurnID,
		item.Platform,
		item.Model,
		item.Mode,
		item.Provider,
		item.RiskLevel,
		item.ProviderReason,
		item.ProviderLatencyMS,
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
	query := `SELECT id, user_id, job_id, conversation_id, turn_id, platform, model, mode,
		moderation_provider, risk_level, provider_reason, provider_latency_ms, action,
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
		ProviderChain:    append([]string(nil), cfg.ProviderChain...),
		APIKeyConfigured: anyProviderConfigured(cfg),
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

func (s *Store) ListPolicies(ctx context.Context, filter PolicyFilter) ([]Policy, error) {
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if scope := normalizeScope(filter.Scope); scope != "" {
		clauses = append(clauses, "scope = ?")
		args = append(args, scope)
	}
	if targetID := strings.TrimSpace(filter.TargetID); targetID != "" {
		clauses = append(clauses, "target_id = ?")
		args = append(args, targetID)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id, scope, target_id, enabled, mode, risk_level, block_message,
		thresholds_json, created_at, updated_at FROM business_risk_control_policies`+where+`
		ORDER BY CASE scope
			WHEN 'global' THEN 1
			WHEN 'plan' THEN 2
			WHEN 'user' THEN 3
			WHEN 'api_key' THEN 4
			ELSE 9
		END, target_id ASC, updated_at DESC`), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Policy, 0)
	for rows.Next() {
		item, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertPolicy(ctx context.Context, input PolicyInput) (Policy, error) {
	scope := normalizeScope(input.Scope)
	targetID := normalizePolicyTargetID(scope, input.TargetID)
	if scope == "" {
		return Policy{}, fmt.Errorf("policy scope is required")
	}
	if targetID == "" && scope != ScopeGlobal {
		return Policy{}, fmt.Errorf("policy targetId is required")
	}
	current, ok, err := s.getPolicyByScopeTarget(ctx, scope, targetID)
	if err != nil {
		return Policy{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if !ok {
		current = Policy{
			ID:         newPolicyID(),
			Scope:      scope,
			TargetID:   targetID,
			Enabled:    true,
			Thresholds: map[string]float64{},
			CreatedAt:  now,
		}
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}
	if input.Mode != nil {
		current.Mode = normalizePolicyMode(*input.Mode)
	}
	if input.RiskLevel != nil {
		current.RiskLevel = normalizeRiskLevel(*input.RiskLevel)
	}
	if input.BlockMessage != nil {
		current.BlockMessage = strings.TrimSpace(*input.BlockMessage)
	}
	if input.Thresholds != nil {
		current.Thresholds = normalizePolicyThresholds(*input.Thresholds)
	}
	current.UpdatedAt = now
	rawThresholds, err := json.Marshal(current.Thresholds)
	if err != nil {
		return Policy{}, err
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_risk_control_policies(
		id, scope, target_id, enabled, mode, risk_level, block_message, thresholds_json, created_at, updated_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(scope, target_id) DO UPDATE SET
		enabled = excluded.enabled,
		mode = excluded.mode,
		risk_level = excluded.risk_level,
		block_message = excluded.block_message,
		thresholds_json = excluded.thresholds_json,
		updated_at = excluded.updated_at`),
		current.ID,
		current.Scope,
		current.TargetID,
		current.Enabled,
		current.Mode,
		current.RiskLevel,
		current.BlockMessage,
		rawThresholds,
		current.CreatedAt,
		current.UpdatedAt,
	)
	if err != nil {
		return Policy{}, err
	}
	item, ok, err := s.getPolicyByScopeTarget(ctx, scope, targetID)
	if err != nil {
		return Policy{}, err
	}
	if !ok {
		return Policy{}, sql.ErrNoRows
	}
	return item, nil
}

func (s *Store) DeletePolicy(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_risk_control_policies WHERE id = ?`), strings.TrimSpace(id))
	return err
}

func (s *Store) ResolvePolicy(ctx context.Context, input PolicyResolutionInput) (PolicyResolution, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return PolicyResolution{}, err
	}
	cfg = NormalizeConfig(cfg)
	applied := []string{ScopeGlobal}
	keys := []struct {
		scope    string
		targetID string
	}{
		{scope: ScopeGlobal, targetID: ""},
	}
	if plan := strings.TrimSpace(input.PlanTag); plan != "" {
		keys = append(keys, struct {
			scope    string
			targetID string
		}{scope: ScopePlan, targetID: strings.ToLower(plan)})
	}
	if userID := strings.TrimSpace(input.UserID); userID != "" {
		keys = append(keys, struct {
			scope    string
			targetID string
		}{scope: ScopeUser, targetID: userID})
	}
	if apiKeyID := strings.TrimSpace(input.APIKeyID); apiKeyID != "" {
		keys = append(keys, struct {
			scope    string
			targetID string
		}{scope: ScopeAPIKey, targetID: apiKeyID})
	}
	for _, key := range keys {
		policy, ok, err := s.getPolicyByScopeTarget(ctx, key.scope, key.targetID)
		if err != nil {
			return PolicyResolution{}, err
		}
		if !ok || !policy.Enabled {
			continue
		}
		cfg = applyPolicyToConfig(cfg, policy)
		applied = append(applied, policy.Scope+":"+policy.TargetID)
	}
	return PolicyResolution{Config: NormalizeConfig(cfg), AppliedScopes: applied}, nil
}

func (s *Store) getPolicyByScopeTarget(ctx context.Context, scope, targetID string) (Policy, bool, error) {
	scope = normalizeScope(scope)
	targetID = normalizePolicyTargetID(scope, targetID)
	row := s.db.QueryRowContext(ctx, s.rebind(`SELECT id, scope, target_id, enabled, mode, risk_level, block_message,
		thresholds_json, created_at, updated_at FROM business_risk_control_policies WHERE scope = ? AND target_id = ?`), scope, targetID)
	item, err := scanPolicy(row)
	if err == sql.ErrNoRows {
		return Policy{}, false, nil
	}
	if err != nil {
		return Policy{}, false, err
	}
	return item, true, nil
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
		&item.Provider,
		&item.RiskLevel,
		&item.ProviderReason,
		&item.ProviderLatencyMS,
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
	item.Provider = normalizeProvider(item.Provider)
	item.RiskLevel = normalizeRiskLevel(item.RiskLevel)
	return item, nil
}

func scanPolicy(scanner interface {
	Scan(dest ...any) error
}) (Policy, error) {
	var item Policy
	var rawThresholds []byte
	err := scanner.Scan(
		&item.ID,
		&item.Scope,
		&item.TargetID,
		&item.Enabled,
		&item.Mode,
		&item.RiskLevel,
		&item.BlockMessage,
		&rawThresholds,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Policy{}, err
	}
	item.Scope = normalizeScope(item.Scope)
	item.TargetID = normalizePolicyTargetID(item.Scope, item.TargetID)
	item.Mode = normalizePolicyMode(item.Mode)
	item.RiskLevel = normalizeRiskLevel(item.RiskLevel)
	item.BlockMessage = strings.TrimSpace(item.BlockMessage)
	item.Thresholds = map[string]float64{}
	if len(rawThresholds) > 0 {
		_ = json.Unmarshal(rawThresholds, &item.Thresholds)
	}
	item.Thresholds = normalizePolicyThresholds(item.Thresholds)
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

func normalizeProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProviderOpenAI:
		return ProviderOpenAI
	case ProviderAliyun:
		return ProviderAliyun
	default:
		return ""
	}
}

func normalizeProviderChain(values []string, fallback string) []string {
	out := make([]string, 0, len(values)+1)
	seen := map[string]bool{}
	for _, value := range values {
		provider := normalizeProvider(value)
		if provider == "" || seen[provider] {
			continue
		}
		seen[provider] = true
		out = append(out, provider)
	}
	if len(out) == 0 {
		provider := normalizeProvider(fallback)
		if provider != "" {
			out = append(out, provider)
		}
	}
	return out
}

func normalizeFailMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case FailModeOpen:
		return FailModeOpen
	case FailModeClosed:
		return FailModeClosed
	default:
		return ""
	}
}

func normalizeBaseURL(value string, fallback string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return strings.TrimRight(strings.TrimSpace(fallback), "/")
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func anyProviderConfigured(cfg Config) bool {
	cfg = NormalizeConfig(cfg)
	for _, provider := range cfg.ProviderChain {
		switch provider {
		case ProviderOpenAI:
			if cfg.OpenAIAPIKey != "" {
				return true
			}
		case ProviderAliyun:
			if cfg.AliyunAccessKeyID != "" && cfg.AliyunAccessKeySecret != "" {
				return true
			}
		}
	}
	return false
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

func normalizeScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ScopeGlobal:
		return ScopeGlobal
	case ScopePlan:
		return ScopePlan
	case ScopeUser:
		return ScopeUser
	case ScopeAPIKey:
		return ScopeAPIKey
	default:
		return ""
	}
}

func normalizePolicyTargetID(scope, targetID string) string {
	targetID = strings.TrimSpace(targetID)
	if scope == ScopeGlobal {
		return ""
	}
	if scope == ScopePlan {
		return strings.ToLower(targetID)
	}
	return targetID
}

func normalizePolicyMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case ModeObserve:
		return ModeObserve
	case ModePreBlock:
		return ModePreBlock
	default:
		return ""
	}
}

func normalizeRiskLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case RiskLevelLow:
		return RiskLevelLow
	case RiskLevelMedium:
		return RiskLevelMedium
	case RiskLevelHigh:
		return RiskLevelHigh
	default:
		return ""
	}
}

func normalizePolicyThresholds(values map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(values))
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

func applyPolicyToConfig(cfg Config, policy Policy) Config {
	if policy.Mode != "" {
		cfg.Mode = policy.Mode
	}
	if policy.BlockMessage != "" {
		cfg.BlockMessage = policy.BlockMessage
	}
	if policy.RiskLevel != "" {
		cfg.RiskLevel = policy.RiskLevel
		cfg.AliyunBlockRiskLevel = policy.RiskLevel
		cfg.Thresholds = mergeThresholds(cfg.Thresholds, thresholdsForRiskLevel(policy.RiskLevel))
	}
	if len(policy.Thresholds) > 0 {
		cfg.Thresholds = mergeThresholds(cfg.Thresholds, policy.Thresholds)
	}
	return NormalizeConfig(cfg)
}

func mergeThresholds(base, override map[string]float64) map[string]float64 {
	out := copyScores(base)
	for key, value := range normalizePolicyThresholds(override) {
		out[key] = value
	}
	return out
}

func thresholdsForRiskLevel(level string) map[string]float64 {
	switch normalizeRiskLevel(level) {
	case RiskLevelHigh:
		return map[string]float64{
			"harassment":             1,
			"harassment/threatening": 0.98,
			"hate":                   0.98,
			"hate/threatening":       0.98,
			"illicit":                1,
			"illicit/violent":        1,
			"self-harm":              0.98,
			"self-harm/intent":       0.98,
			"self-harm/instructions": 0.98,
			"sexual":                 0.98,
			"sexual/minors":          0.90,
			"violence":               1,
			"violence/graphic":       1,
		}
	case RiskLevelMedium:
		return map[string]float64{
			"harassment":             0.99,
			"harassment/threatening": 0.95,
			"hate":                   0.85,
			"hate/threatening":       0.85,
			"illicit":                0.98,
			"illicit/violent":        0.98,
			"self-harm":              0.85,
			"self-harm/intent":       0.92,
			"self-harm/instructions": 0.85,
			"sexual":                 0.85,
			"sexual/minors":          0.75,
			"violence":               0.98,
			"violence/graphic":       0.98,
		}
	case RiskLevelLow:
		return DefaultConfig().Thresholds
	default:
		return nil
	}
}

func newLogID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "risk_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("risk_%d", time.Now().UnixNano())
}

func newPolicyID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "riskpol_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("riskpol_%d", time.Now().UnixNano())
}
