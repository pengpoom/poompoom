package businessapikeys

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

type Store struct {
	db     *sql.DB
	driver string
	secret string
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
	store := NewStoreWithDB(db, cfg.Database.Driver, strings.TrimSpace(cfg.ExternalAPI.SigningSecret))
	store.ownDB = true
	return store, nil
}

func NewStoreWithDB(db *sql.DB, driver, secret string) *Store {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		driver = "postgres"
	}
	return &Store{db: db, driver: driver, secret: secret}
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}

func (s *Store) Close() error {
	if s == nil || s.db == nil || !s.ownDB {
		return nil
	}
	return s.db.Close()
}

const apiKeyColumns = `id, user_id, name, key_prefix, key_last4, status,
	credit_limit, used_credits, rate_limit_per_minute, concurrency_limit,
	allowed_models, last_used_at, created_at, updated_at, revoked_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(sc rowScanner) (APIKey, error) {
	var (
		key           APIKey
		allowedModels string
	)
	if err := sc.Scan(
		&key.ID, &key.UserID, &key.Name, &key.KeyPrefix, &key.KeyLast4, &key.Status,
		&key.CreditLimit, &key.UsedCredits, &key.RateLimitPerMinute, &key.ConcurrencyLimit,
		&allowedModels, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt, &key.RevokedAt,
	); err != nil {
		return APIKey{}, err
	}
	key.AllowedModels = []string{}
	if strings.TrimSpace(allowedModels) != "" {
		_ = json.Unmarshal([]byte(allowedModels), &key.AllowedModels)
	}
	if key.AllowedModels == nil {
		key.AllowedModels = []string{}
	}
	return key, nil
}

type CreateInput struct {
	UserID             string
	Name               string
	Env                string
	CreditLimit        int64
	RateLimitPerMinute int
	ConcurrencyLimit   int
	AllowedModels      []string
}

func (s *Store) Create(ctx context.Context, in CreateInput) (APIKey, string, error) {
	gen, err := GenerateKey(in.Env)
	if err != nil {
		return APIKey{}, "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	models := in.AllowedModels
	if models == nil {
		models = []string{}
	}
	modelsJSON, err := json.Marshal(models)
	if err != nil {
		return APIKey{}, "", err
	}
	rate := in.RateLimitPerMinute
	if rate <= 0 {
		rate = 60
	}
	concurrency := in.ConcurrencyLimit
	if concurrency <= 0 {
		concurrency = 2
	}
	key := APIKey{
		ID:                 newAPIKeyID(),
		UserID:             strings.TrimSpace(in.UserID),
		Name:               strings.TrimSpace(in.Name),
		KeyPrefix:          gen.Prefix,
		KeyLast4:           gen.Last4,
		Status:             StatusActive,
		CreditLimit:        in.CreditLimit,
		UsedCredits:        0,
		RateLimitPerMinute: rate,
		ConcurrencyLimit:   concurrency,
		AllowedModels:      models,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO business_api_keys(
		id, user_id, name, key_prefix, key_last4, key_hash, status,
		credit_limit, used_credits, rate_limit_per_minute, concurrency_limit,
		allowed_models, metadata_json, last_used_at, created_at, updated_at, revoked_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		key.ID, key.UserID, key.Name, key.KeyPrefix, key.KeyLast4,
		KeyHash(s.secret, gen.Plaintext), key.Status,
		key.CreditLimit, key.UsedCredits, key.RateLimitPerMinute, key.ConcurrencyLimit,
		string(modelsJSON), "{}", "", key.CreatedAt, key.UpdatedAt, "",
	)
	if err != nil {
		return APIKey{}, "", err
	}
	return key, gen.Plaintext, nil
}

// Authenticate 用明文 key 反查激活状态的记录，并刷新 last_used_at。
// 因 key_hash 唯一且为确定性 HMAC，直接等值查询即匹配，无需应用层逐字符比较。
func (s *Store) Authenticate(ctx context.Context, plaintext string) (APIKey, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return APIKey{}, sql.ErrNoRows
	}
	hash := KeyHash(s.secret, plaintext)
	row := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT `+apiKeyColumns+` FROM business_api_keys WHERE key_hash = ? AND status = ?`),
		hash, StatusActive)
	key, err := scanAPIKey(row)
	if err != nil {
		return APIKey{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = s.db.ExecContext(ctx, s.rebind(
		`UPDATE business_api_keys SET last_used_at = ?, updated_at = ? WHERE id = ?`),
		now, now, key.ID)
	key.LastUsedAt = now
	return key, nil
}
