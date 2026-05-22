package businessproviders

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/sqlitedb"
)

const (
	PlatformGPTImage     = "gpt-image"
	PlatformGeminiBanana = "gemini-banana"
)

type Provider struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	BaseURL      string `json:"baseUrl"`
	APIKey       string `json:"apiKey"`
	DefaultModel string `json:"defaultModel"`
	Enabled      bool   `json:"enabled"`
	IsDefault    bool   `json:"isDefault"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type MutationInput struct {
	Name         string
	Platform     string
	BaseURL      string
	APIKey       string
	DefaultModel string
	Enabled      bool
	IsDefault    bool
}

type Store struct {
	db *sql.DB
}

func NewStore(cfg *config.Config) (*Store, error) {
	rawPath := strings.TrimSpace(cfg.Storage.SQLitePath)
	if rawPath == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	db, err := sqlitedb.Open(cfg.ResolvePath(rawPath))
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
		`CREATE TABLE IF NOT EXISTS business_api_providers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			platform TEXT NOT NULL,
			base_url TEXT NOT NULL,
			api_key TEXT NOT NULL,
			default_model TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			is_default INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_business_api_providers_platform
			ON business_api_providers(platform, enabled, is_default);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) List(ctx context.Context) ([]Provider, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, platform, base_url, api_key, default_model, enabled, is_default, created_at, updated_at
		 FROM business_api_providers
		 ORDER BY platform ASC, is_default DESC, enabled DESC, created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Provider, 0)
	for rows.Next() {
		item, err := scanProvider(rows)
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

func (s *Store) Create(ctx context.Context, input MutationInput) (Provider, error) {
	provider, err := normalizeInput(input)
	if err != nil {
		return Provider{}, err
	}
	provider.ID = newProviderID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	provider.CreatedAt = now
	provider.UpdatedAt = now
	if provider.IsDefault {
		if err := s.clearDefault(ctx, provider.Platform); err != nil {
			return Provider{}, err
		}
	} else if provider.Enabled {
		hasDefault, err := s.hasDefault(ctx, provider.Platform)
		if err != nil {
			return Provider{}, err
		}
		provider.IsDefault = !hasDefault
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO business_api_providers(id, name, platform, base_url, api_key, default_model, enabled, is_default, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		provider.ID,
		provider.Name,
		provider.Platform,
		provider.BaseURL,
		provider.APIKey,
		provider.DefaultModel,
		boolInt(provider.Enabled),
		boolInt(provider.IsDefault),
		provider.CreatedAt,
		provider.UpdatedAt,
	)
	return provider, err
}

func (s *Store) Update(ctx context.Context, id string, input MutationInput) (Provider, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Provider{}, false, nil
	}
	current, ok, err := s.Get(ctx, id)
	if err != nil || !ok {
		return Provider{}, ok, err
	}
	next, err := normalizeInput(input)
	if err != nil {
		return Provider{}, false, err
	}
	next.ID = current.ID
	next.CreatedAt = current.CreatedAt
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if next.IsDefault {
		if err := s.clearDefault(ctx, next.Platform); err != nil {
			return Provider{}, false, err
		}
	} else if current.IsDefault && next.Enabled {
		next.IsDefault = true
	}
	_, err = s.db.ExecContext(
		ctx,
		`UPDATE business_api_providers
		 SET name = ?, platform = ?, base_url = ?, api_key = ?, default_model = ?, enabled = ?, is_default = ?, updated_at = ?
		 WHERE id = ?`,
		next.Name,
		next.Platform,
		next.BaseURL,
		next.APIKey,
		next.DefaultModel,
		boolInt(next.Enabled),
		boolInt(next.IsDefault),
		next.UpdatedAt,
		next.ID,
	)
	if err != nil {
		return Provider{}, false, err
	}
	return next, true, nil
}

func (s *Store) SetDefault(ctx context.Context, id string) (Provider, bool, error) {
	item, ok, err := s.Get(ctx, id)
	if err != nil || !ok {
		return Provider{}, ok, err
	}
	if err := s.clearDefault(ctx, item.Platform); err != nil {
		return Provider{}, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		`UPDATE business_api_providers
		 SET enabled = 1, is_default = 1, updated_at = ?
		 WHERE id = ?`,
		now,
		item.ID,
	)
	if err != nil {
		return Provider{}, false, err
	}
	item.Enabled = true
	item.IsDefault = true
	item.UpdatedAt = now
	return item, true, nil
}

func (s *Store) Delete(ctx context.Context, id string) (bool, error) {
	id = cleanID(id)
	if id == "" {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM business_api_providers WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) Get(ctx context.Context, id string) (Provider, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Provider{}, false, nil
	}
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, platform, base_url, api_key, default_model, enabled, is_default, created_at, updated_at
		 FROM business_api_providers
		 WHERE id = ?`,
		id,
	)
	item, err := scanProvider(row)
	if err == sql.ErrNoRows {
		return Provider{}, false, nil
	}
	if err != nil {
		return Provider{}, false, err
	}
	return item, true, nil
}

func (s *Store) DefaultForPlatform(ctx context.Context, platform string) (Provider, bool, error) {
	platform = NormalizePlatform(platform)
	if platform == "" {
		return Provider{}, false, nil
	}
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, platform, base_url, api_key, default_model, enabled, is_default, created_at, updated_at
		 FROM business_api_providers
		 WHERE platform = ? AND enabled = 1
		 ORDER BY is_default DESC, updated_at DESC
		 LIMIT 1`,
		platform,
	)
	item, err := scanProvider(row)
	if err == sql.ErrNoRows {
		return Provider{}, false, nil
	}
	if err != nil {
		return Provider{}, false, err
	}
	return item, true, nil
}

func (s *Store) EnsureConfigProvider(ctx context.Context, cfg *config.Config) error {
	platform := NormalizePlatform(cfg.APIAccess.Platform)
	baseURL := strings.TrimSpace(cfg.APIAccess.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIAccess.APIKey)
	if platform == "" || baseURL == "" || apiKey == "" {
		return nil
	}
	if _, ok, err := s.DefaultForPlatform(ctx, platform); err != nil {
		return err
	} else if ok {
		return nil
	}
	_, err := s.Create(ctx, MutationInput{
		Name:         "默认 API 接入",
		Platform:     platform,
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: "gpt-image-2",
		Enabled:      true,
		IsDefault:    true,
	})
	return err
}

func (s *Store) clearDefault(ctx context.Context, platform string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE business_api_providers SET is_default = 0 WHERE platform = ?`,
		platform,
	)
	return err
}

func (s *Store) hasDefault(ctx context.Context, platform string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM business_api_providers WHERE platform = ? AND enabled = 1 AND is_default = 1`,
		platform,
	).Scan(&count)
	return count > 0, err
}

type providerScanner interface {
	Scan(dest ...any) error
}

func scanProvider(row providerScanner) (Provider, error) {
	var item Provider
	var enabled int
	var isDefault int
	err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Platform,
		&item.BaseURL,
		&item.APIKey,
		&item.DefaultModel,
		&enabled,
		&isDefault,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Provider{}, err
	}
	item.Platform = NormalizePlatform(item.Platform)
	item.Enabled = enabled != 0
	item.IsDefault = isDefault != 0
	return item, nil
}

func normalizeInput(input MutationInput) (Provider, error) {
	item := Provider{
		Name:         strings.TrimSpace(input.Name),
		Platform:     NormalizePlatform(input.Platform),
		BaseURL:      strings.TrimSpace(input.BaseURL),
		APIKey:       strings.TrimSpace(input.APIKey),
		DefaultModel: strings.TrimSpace(input.DefaultModel),
		Enabled:      input.Enabled,
		IsDefault:    input.IsDefault,
	}
	if item.Name == "" {
		item.Name = item.Platform
	}
	if item.Platform == "" {
		return Provider{}, fmt.Errorf("platform must be gpt-image or gemini-banana")
	}
	if item.BaseURL == "" {
		return Provider{}, fmt.Errorf("baseUrl is required")
	}
	if item.APIKey == "" {
		return Provider{}, fmt.Errorf("apiKey is required")
	}
	if item.DefaultModel == "" {
		item.DefaultModel = defaultModelForPlatform(item.Platform)
	}
	return item, nil
}

func NormalizePlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", PlatformGPTImage:
		return PlatformGPTImage
	case PlatformGeminiBanana:
		return PlatformGeminiBanana
	default:
		return ""
	}
}

func defaultModelForPlatform(platform string) string {
	switch NormalizePlatform(platform) {
	case PlatformGeminiBanana:
		return "gemini-2.5-flash-image"
	default:
		return "gpt-image-2"
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func cleanID(value string) string {
	return strings.TrimSpace(value)
}

func newProviderID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("provider-%d", time.Now().UnixNano())
	}
	return "provider_" + hex.EncodeToString(bytes[:])
}
