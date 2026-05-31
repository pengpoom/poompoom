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
	"imagestudio/internal/database"
)

const (
	PlatformGPTImage     = "gpt-image"
	PlatformGeminiBanana = "gemini-banana"
)

const (
	MemberStatusActive      = "active"
	MemberStatusLimited     = "limited"
	MemberStatusUnavailable = "unavailable"
)

const (
	GroupMatchFallback = "fallback"
	GroupMatchAny      = "any"
	GroupMatchAll      = "all"
)

const (
	SelectionStrategyTagged         = "tagged_pool"
	SelectionStrategyTaggedFallback = "tagged_pool_fallback"
	SelectionStrategyFallback       = "fallback_pool"
)

const (
	defaultMemberWeight           = 1
	defaultMemberCooldownSeconds  = 120
	defaultMemberFailureThreshold = 5
)

const memberSelectColumns = `id, group_id, name, platform, base_url, api_key, default_model,
	enabled, priority, weight, max_concurrent, cooldown_seconds, failure_threshold,
	consecutive_failures, status, cooldown_until, success_count, fail_count,
	last_used_at, last_error, last_error_at, created_at, updated_at`

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

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	MatchMode   string `json:"matchMode"`
	Enabled     bool   `json:"enabled"`
	IsDefault   bool   `json:"isDefault"`
	Priority    int    `json:"priority"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type GroupInput struct {
	Name        string
	Platform    string
	Description string
	Tags        string
	MatchMode   string
	Enabled     bool
	IsDefault   bool
	Priority    int
}

type Member struct {
	ID                  string `json:"id"`
	GroupID             string `json:"groupId"`
	Name                string `json:"name"`
	Platform            string `json:"platform"`
	BaseURL             string `json:"baseUrl"`
	APIKey              string `json:"apiKey"`
	DefaultModel        string `json:"defaultModel"`
	Enabled             bool   `json:"enabled"`
	Priority            int    `json:"priority"`
	Weight              int    `json:"weight"`
	MaxConcurrent       int    `json:"maxConcurrent"`
	CooldownSeconds     int    `json:"cooldownSeconds"`
	FailureThreshold    int    `json:"failureThreshold"`
	ConsecutiveFailures int64  `json:"consecutiveFailures"`
	Status              string `json:"status"`
	CooldownUntil       string `json:"cooldownUntil"`
	SuccessCount        int64  `json:"successCount"`
	FailCount           int64  `json:"failCount"`
	LastUsedAt          string `json:"lastUsedAt"`
	LastError           string `json:"lastError"`
	LastErrorAt         string `json:"lastErrorAt"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type MemberInput struct {
	GroupID          string
	Name             string
	Platform         string
	BaseURL          string
	APIKey           string
	DefaultModel     string
	Enabled          bool
	Priority         int
	Weight           int
	MaxConcurrent    int
	CooldownSeconds  int
	FailureThreshold int
	Status           string
}

type PoolSelection struct {
	Group    Group
	Member   Member
	Strategy string
}

type PoolSelectionPolicy struct {
	Platform string
	Tags     []string
}

type poolSelectionScope struct {
	GroupIDs []string
	Strategy string
}

type Pool struct {
	Group
	Members []Member `json:"members"`
}

type MemberFailure struct {
	Status          string
	CooldownSeconds int
	ErrorMessage    string
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
		`CREATE TABLE IF NOT EXISTS business_provider_groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			platform TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '',
			match_mode TEXT NOT NULL DEFAULT 'fallback',
			enabled INTEGER NOT NULL DEFAULT 1,
			is_default INTEGER NOT NULL DEFAULT 0,
			priority INTEGER NOT NULL DEFAULT 50,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS business_provider_members (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL,
			name TEXT NOT NULL,
			platform TEXT NOT NULL,
			base_url TEXT NOT NULL,
			api_key TEXT NOT NULL,
			default_model TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 50,
			weight INTEGER NOT NULL DEFAULT 1,
			max_concurrent INTEGER NOT NULL DEFAULT 0,
			cooldown_seconds INTEGER NOT NULL DEFAULT 120,
			failure_threshold INTEGER NOT NULL DEFAULT 5,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			cooldown_until TEXT NOT NULL DEFAULT '',
			success_count INTEGER NOT NULL DEFAULT 0,
			fail_count INTEGER NOT NULL DEFAULT 0,
			last_used_at TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			last_error_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
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
		s.rebind(`SELECT id, name, platform, base_url, api_key, default_model, enabled, is_default, created_at, updated_at
		 FROM business_api_providers
		 ORDER BY platform ASC, is_default DESC, enabled DESC, created_at ASC`),
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
		s.rebind(`INSERT INTO business_api_providers(id, name, platform, base_url, api_key, default_model, enabled, is_default, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
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
		s.rebind(`UPDATE business_api_providers
		 SET name = ?, platform = ?, base_url = ?, api_key = ?, default_model = ?, enabled = ?, is_default = ?, updated_at = ?
		 WHERE id = ?`),
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
		s.rebind(`UPDATE business_api_providers
		 SET enabled = 1, is_default = 1, updated_at = ?
		 WHERE id = ?`),
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
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_api_providers WHERE id = ?`), id)
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
		s.rebind(`SELECT id, name, platform, base_url, api_key, default_model, enabled, is_default, created_at, updated_at
		 FROM business_api_providers
		 WHERE id = ?`),
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
		s.rebind(`SELECT id, name, platform, base_url, api_key, default_model, enabled, is_default, created_at, updated_at
		 FROM business_api_providers
		 WHERE platform = ? AND enabled = 1
		 ORDER BY is_default DESC, updated_at DESC
		 LIMIT 1`),
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

func (s *Store) CreateGroup(ctx context.Context, input GroupInput) (Group, error) {
	group, err := normalizeGroupInput(input)
	if err != nil {
		return Group{}, err
	}
	group.ID = newProviderGroupID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	group.CreatedAt = now
	group.UpdatedAt = now
	if group.IsDefault {
		if err := s.clearDefaultGroup(ctx, group.Platform); err != nil {
			return Group{}, err
		}
	} else if group.Enabled {
		hasDefault, err := s.hasDefaultGroup(ctx, group.Platform)
		if err != nil {
			return Group{}, err
		}
		group.IsDefault = !hasDefault
	}
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_provider_groups(id, name, platform, description, tags, match_mode, enabled, is_default, priority, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		group.ID,
		group.Name,
		group.Platform,
		group.Description,
		group.Tags,
		group.MatchMode,
		boolInt(group.Enabled),
		boolInt(group.IsDefault),
		group.Priority,
		group.CreatedAt,
		group.UpdatedAt,
	)
	return group, err
}

func (s *Store) ListPools(ctx context.Context) ([]Pool, error) {
	groups, err := s.listGroups(ctx)
	if err != nil {
		return nil, err
	}
	pools := make([]Pool, 0, len(groups))
	groupIndex := make(map[string]int, len(groups))
	for _, group := range groups {
		groupIndex[group.ID] = len(pools)
		pools = append(pools, Pool{Group: group, Members: []Member{}})
	}
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT `+memberSelectColumns+`
		 FROM business_provider_members
		 ORDER BY group_id ASC, enabled DESC, priority ASC, weight DESC,
		          CASE WHEN last_used_at = '' THEN 0 ELSE 1 END ASC,
		          last_used_at ASC, created_at ASC`),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		if index, ok := groupIndex[member.GroupID]; ok {
			pools[index].Members = append(pools[index].Members, member)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pools, nil
}

func (s *Store) UpdateGroup(ctx context.Context, id string, input GroupInput) (Group, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Group{}, false, nil
	}
	current, ok, err := s.GetGroup(ctx, id)
	if err != nil || !ok {
		return Group{}, ok, err
	}
	next, err := normalizeGroupInput(input)
	if err != nil {
		return Group{}, false, err
	}
	if next.Platform != current.Platform {
		hasMembers, err := s.groupHasMembers(ctx, current.ID)
		if err != nil {
			return Group{}, false, err
		}
		if hasMembers {
			return Group{}, false, fmt.Errorf("provider group platform cannot change while members exist")
		}
	}
	next.ID = current.ID
	next.CreatedAt = current.CreatedAt
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if next.IsDefault {
		if err := s.clearDefaultGroup(ctx, next.Platform); err != nil {
			return Group{}, false, err
		}
	} else if current.IsDefault && next.Enabled && next.Platform == current.Platform {
		next.IsDefault = true
	}
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_groups
		 SET name = ?, platform = ?, description = ?, tags = ?, match_mode = ?, enabled = ?, is_default = ?, priority = ?, updated_at = ?
		 WHERE id = ?`),
		next.Name,
		next.Platform,
		next.Description,
		next.Tags,
		next.MatchMode,
		boolInt(next.Enabled),
		boolInt(next.IsDefault),
		next.Priority,
		next.UpdatedAt,
		next.ID,
	)
	if err != nil {
		return Group{}, false, err
	}
	return next, true, nil
}

func (s *Store) SetDefaultGroup(ctx context.Context, id string) (Group, bool, error) {
	item, ok, err := s.GetGroup(ctx, id)
	if err != nil || !ok {
		return Group{}, ok, err
	}
	if err := s.clearDefaultGroup(ctx, item.Platform); err != nil {
		return Group{}, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_groups
		 SET enabled = 1, is_default = 1, updated_at = ?
		 WHERE id = ?`),
		now,
		item.ID,
	)
	if err != nil {
		return Group{}, false, err
	}
	item.Enabled = true
	item.IsDefault = true
	item.UpdatedAt = now
	return item, true, nil
}

func (s *Store) DeleteGroup(ctx context.Context, id string) (bool, error) {
	id = cleanID(id)
	if id == "" {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_provider_groups WHERE id = ?`), id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) CreateMember(ctx context.Context, input MemberInput) (Member, error) {
	member, err := normalizeMemberInput(input)
	if err != nil {
		return Member{}, err
	}
	group, ok, err := s.GetGroup(ctx, member.GroupID)
	if err != nil {
		return Member{}, err
	}
	if !ok {
		return Member{}, fmt.Errorf("provider group not found")
	}
	if member.Platform == "" {
		member.Platform = group.Platform
	}
	if member.Platform != group.Platform {
		return Member{}, fmt.Errorf("member platform must match provider group platform")
	}
	if member.Name == "" {
		member.Name = member.Platform
	}
	if member.DefaultModel == "" {
		member.DefaultModel = defaultModelForPlatform(member.Platform)
	}
	member.ID = newProviderMemberID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	member.CreatedAt = now
	member.UpdatedAt = now
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_provider_members(id, group_id, name, platform, base_url, api_key, default_model, enabled, priority, weight, max_concurrent, cooldown_seconds, failure_threshold, consecutive_failures, status, cooldown_until, success_count, fail_count, last_used_at, last_error, last_error_at, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		member.ID,
		member.GroupID,
		member.Name,
		member.Platform,
		member.BaseURL,
		member.APIKey,
		member.DefaultModel,
		boolInt(member.Enabled),
		member.Priority,
		member.Weight,
		member.MaxConcurrent,
		member.CooldownSeconds,
		member.FailureThreshold,
		member.ConsecutiveFailures,
		member.Status,
		member.CooldownUntil,
		member.SuccessCount,
		member.FailCount,
		member.LastUsedAt,
		member.LastError,
		member.LastErrorAt,
		member.CreatedAt,
		member.UpdatedAt,
	)
	return member, err
}

func (s *Store) UpdateMember(ctx context.Context, id string, input MemberInput) (Member, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Member{}, false, nil
	}
	current, ok, err := s.GetMember(ctx, id)
	if err != nil || !ok {
		return Member{}, ok, err
	}
	next, err := normalizeMemberInput(input)
	if err != nil {
		return Member{}, false, err
	}
	group, ok, err := s.GetGroup(ctx, next.GroupID)
	if err != nil {
		return Member{}, false, err
	}
	if !ok {
		return Member{}, false, fmt.Errorf("provider group not found")
	}
	if next.Platform == "" {
		next.Platform = group.Platform
	}
	if next.Platform != group.Platform {
		return Member{}, false, fmt.Errorf("member platform must match provider group platform")
	}
	if next.Name == "" {
		next.Name = next.Platform
	}
	if next.DefaultModel == "" {
		next.DefaultModel = defaultModelForPlatform(next.Platform)
	}
	next.ID = current.ID
	next.CooldownUntil = current.CooldownUntil
	next.SuccessCount = current.SuccessCount
	next.FailCount = current.FailCount
	next.ConsecutiveFailures = current.ConsecutiveFailures
	next.LastUsedAt = current.LastUsedAt
	next.LastError = current.LastError
	next.LastErrorAt = current.LastErrorAt
	next.CreatedAt = current.CreatedAt
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_members
		 SET group_id = ?, name = ?, platform = ?, base_url = ?, api_key = ?, default_model = ?, enabled = ?, priority = ?, weight = ?, max_concurrent = ?, cooldown_seconds = ?, failure_threshold = ?, status = ?, updated_at = ?
		 WHERE id = ?`),
		next.GroupID,
		next.Name,
		next.Platform,
		next.BaseURL,
		next.APIKey,
		next.DefaultModel,
		boolInt(next.Enabled),
		next.Priority,
		next.Weight,
		next.MaxConcurrent,
		next.CooldownSeconds,
		next.FailureThreshold,
		next.Status,
		next.UpdatedAt,
		next.ID,
	)
	if err != nil {
		return Member{}, false, err
	}
	return next, true, nil
}

func (s *Store) DeleteMember(ctx context.Context, id string) (bool, error) {
	id = cleanID(id)
	if id == "" {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_provider_members WHERE id = ?`), id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) GetGroup(ctx context.Context, id string) (Group, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Group{}, false, nil
	}
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, name, platform, description, tags, match_mode, enabled, is_default, priority, created_at, updated_at
		 FROM business_provider_groups
		 WHERE id = ?`),
		id,
	)
	item, err := scanGroup(row)
	if err == sql.ErrNoRows {
		return Group{}, false, nil
	}
	if err != nil {
		return Group{}, false, err
	}
	return item, true, nil
}

func (s *Store) GetMember(ctx context.Context, id string) (Member, bool, error) {
	id = cleanID(id)
	if id == "" {
		return Member{}, false, nil
	}
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT `+memberSelectColumns+`
		 FROM business_provider_members
		 WHERE id = ?`),
		id,
	)
	item, err := scanMember(row)
	if err == sql.ErrNoRows {
		return Member{}, false, nil
	}
	if err != nil {
		return Member{}, false, err
	}
	return item, true, nil
}

func (s *Store) listGroups(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT id, name, platform, description, tags, match_mode, enabled, is_default, priority, created_at, updated_at
		 FROM business_provider_groups
		 ORDER BY platform ASC, is_default DESC, enabled DESC, match_mode ASC, priority ASC, created_at ASC`),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Group, 0)
	for rows.Next() {
		item, err := scanGroup(rows)
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

func (s *Store) groupHasMembers(ctx context.Context, groupID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT COUNT(*) FROM business_provider_members WHERE group_id = ?`),
		groupID,
	).Scan(&count)
	return count > 0, err
}

func (s *Store) SelectDefaultMemberForPlatform(ctx context.Context, platform string) (PoolSelection, bool, error) {
	return s.SelectMember(ctx, PoolSelectionPolicy{Platform: platform})
}

func (s *Store) SelectMember(ctx context.Context, policy PoolSelectionPolicy) (PoolSelection, bool, error) {
	platform := NormalizePlatform(policy.Platform)
	if platform == "" {
		return PoolSelection{}, false, nil
	}
	scopes, err := s.selectGroupScopesForPolicy(ctx, platform, policy.Tags)
	if err != nil || len(scopes) == 0 {
		return PoolSelection{}, false, err
	}
	for _, scope := range scopes {
		selection, ok, err := s.selectMemberFromGroupIDs(ctx, platform, scope.GroupIDs, scope.Strategy)
		if err != nil || ok {
			return selection, ok, err
		}
	}
	return PoolSelection{}, false, nil
}

func (s *Store) selectMemberFromGroupIDs(ctx context.Context, platform string, groupIDs []string, strategy string) (PoolSelection, bool, error) {
	if len(groupIDs) == 0 {
		return PoolSelection{}, false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	placeholders := make([]string, len(groupIDs))
	args := make([]any, 0, len(groupIDs)+4)
	for i, groupID := range groupIDs {
		placeholders[i] = "?"
		args = append(args, groupID)
	}
	args = append(args, platform, MemberStatusUnavailable, now, now)
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT
			g.id, g.name, g.platform, g.description, g.tags, g.match_mode, g.enabled, g.is_default, g.priority, g.created_at, g.updated_at,
			m.id, m.group_id, m.name, m.platform, m.base_url, m.api_key, m.default_model,
			m.enabled, m.priority, m.weight, m.max_concurrent, m.cooldown_seconds, m.failure_threshold,
			m.consecutive_failures, m.status, m.cooldown_until, m.success_count, m.fail_count,
			m.last_used_at, m.last_error, m.last_error_at, m.created_at, m.updated_at
		 FROM business_provider_groups g
		 JOIN business_provider_members m ON m.group_id = g.id
		 LEFT JOIN (
			SELECT provider_id, COUNT(*) AS running_count
			FROM business_image_jobs
			WHERE status = 'running'
			GROUP BY provider_id
		 ) r ON r.provider_id = m.id
		 WHERE g.id IN (`+strings.Join(placeholders, ",")+`)
		   AND g.platform = ? AND g.enabled = 1
		   AND m.platform = g.platform AND m.enabled = 1
		   AND m.status <> ?
		   AND (m.cooldown_until = '' OR m.cooldown_until <= ?)
		   AND (m.max_concurrent <= 0 OR COALESCE(r.running_count, 0) < m.max_concurrent)
		 ORDER BY g.is_default DESC, g.priority ASC, g.created_at ASC,
		          m.priority ASC,
		          COALESCE(r.running_count, 0) ASC,
		          CASE
		            WHEN m.weight <= 1 THEN 1.0
		            WHEN m.last_used_at = '' THEN 0.0
		            ELSE EXTRACT(EPOCH FROM (?::timestamptz - m.last_used_at::timestamptz)) / GREATEST(m.weight, 1)
		          END DESC,
		          CASE WHEN m.last_used_at = '' THEN 0 ELSE 1 END ASC,
		          m.last_used_at ASC,
		          m.created_at ASC
		 LIMIT 1`),
		args...,
	)
	selection, err := scanPoolSelection(row)
	if err == sql.ErrNoRows {
		return PoolSelection{}, false, nil
	}
	if err != nil {
		return PoolSelection{}, false, err
	}
	selection.Strategy = strategy
	return selection, true, nil
}

func (s *Store) selectGroupScopesForPolicy(ctx context.Context, platform string, rawTags []string) ([]poolSelectionScope, error) {
	pools, err := s.ListPools(ctx)
	if err != nil {
		return nil, err
	}
	requestTags := normalizeTags(rawTags)
	matched := make([]string, 0, len(pools))
	fallback := make([]string, 0, len(pools))
	for _, pool := range pools {
		if NormalizePlatform(pool.Platform) != platform || !pool.Enabled {
			continue
		}
		if containsTag(requestTags, "pool:"+pool.ID) {
			matched = append(matched, pool.ID)
			continue
		}
		if pool.MatchMode == GroupMatchFallback || strings.TrimSpace(pool.Tags) == "" {
			fallback = append(fallback, pool.ID)
			continue
		}
		if groupMatchesTags(pool.Group, requestTags) {
			matched = append(matched, pool.ID)
		}
	}
	scopes := make([]poolSelectionScope, 0, 2)
	if len(matched) > 0 {
		scopes = append(scopes, poolSelectionScope{
			GroupIDs: matched,
			Strategy: SelectionStrategyTagged,
		})
		if len(fallback) > 0 {
			scopes = append(scopes, poolSelectionScope{
				GroupIDs: fallback,
				Strategy: SelectionStrategyTaggedFallback,
			})
		}
		return scopes, nil
	}
	if len(fallback) > 0 {
		scopes = append(scopes, poolSelectionScope{
			GroupIDs: fallback,
			Strategy: SelectionStrategyFallback,
		})
	}
	return scopes, nil
}

func containsTag(tags []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func groupMatchesTags(group Group, requestTags []string) bool {
	groupTags := normalizeTagList(group.Tags)
	if len(groupTags) == 0 || len(requestTags) == 0 {
		return false
	}
	requestSet := make(map[string]struct{}, len(requestTags))
	for _, tag := range requestTags {
		requestSet[tag] = struct{}{}
	}
	if group.MatchMode == GroupMatchAll {
		for _, tag := range groupTags {
			if _, ok := requestSet[tag]; !ok {
				return false
			}
		}
		return true
	}
	for _, tag := range groupTags {
		if _, ok := requestSet[tag]; ok {
			return true
		}
	}
	return false
}

func normalizeTagList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return normalizeTags(strings.Split(value, ","))
}

func (s *Store) MarkMemberAcquired(ctx context.Context, id string) error {
	id = cleanID(id)
	if id == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_members
		 SET status = ?, cooldown_until = '', last_used_at = ?, updated_at = ?
		 WHERE id = ?`),
		MemberStatusActive,
		now,
		now,
		id,
	)
	return err
}

func (s *Store) ReleaseMember(ctx context.Context, id string) error {
	id = cleanID(id)
	if id == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_members
		 SET status = ?, cooldown_until = '', updated_at = ?
		 WHERE id = ? AND status <> ?`),
		MemberStatusActive,
		now,
		id,
		MemberStatusUnavailable,
	)
	return err
}

func (s *Store) ReportMemberSuccess(ctx context.Context, id string) error {
	id = cleanID(id)
	if id == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_members
		 SET status = ?, cooldown_until = '', success_count = success_count + 1, consecutive_failures = 0, last_used_at = ?, last_error = '', last_error_at = '', updated_at = ?
		 WHERE id = ?`),
		MemberStatusActive,
		now,
		now,
		id,
	)
	return err
}

func (s *Store) ReportMemberFailure(ctx context.Context, id string, failure MemberFailure) error {
	id = cleanID(id)
	if id == "" {
		return nil
	}
	member, ok, err := s.GetMember(ctx, id)
	if err != nil || !ok {
		return err
	}
	status := NormalizeMemberStatus(failure.Status)
	if status == "" {
		status = MemberStatusLimited
	}
	nextConsecutiveFailures := member.ConsecutiveFailures + 1
	if member.FailureThreshold > 0 && nextConsecutiveFailures >= int64(member.FailureThreshold) {
		status = MemberStatusUnavailable
	}
	cooldownUntil := ""
	if status == MemberStatusLimited && failure.CooldownSeconds > 0 {
		cooldownSeconds := failure.CooldownSeconds
		cooldownUntil = time.Now().UTC().Add(time.Duration(cooldownSeconds) * time.Second).Format(time.RFC3339Nano)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_members
		 SET status = ?, cooldown_until = ?, fail_count = fail_count + 1, consecutive_failures = ?, last_error = ?, last_error_at = ?, updated_at = ?
		 WHERE id = ?`),
		status,
		cooldownUntil,
		nextConsecutiveFailures,
		strings.TrimSpace(failure.ErrorMessage),
		now,
		now,
		id,
	)
	return err
}

func (s *Store) RecoverMember(ctx context.Context, id string) (Member, bool, error) {
	member, ok, err := s.GetMember(ctx, id)
	if err != nil || !ok {
		return Member{}, ok, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_members
		 SET status = ?, enabled = 1, cooldown_until = '', consecutive_failures = 0, last_error = '', last_error_at = '', updated_at = ?
		 WHERE id = ?`),
		MemberStatusActive,
		now,
		member.ID,
	)
	if err != nil {
		return Member{}, false, err
	}
	member.Status = MemberStatusActive
	member.Enabled = true
	member.CooldownUntil = ""
	member.ConsecutiveFailures = 0
	member.LastError = ""
	member.LastErrorAt = ""
	member.UpdatedAt = now
	return member, true, nil
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

func (s *Store) clearDefaultGroup(ctx context.Context, platform string) error {
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_provider_groups SET is_default = 0 WHERE platform = ?`),
		platform,
	)
	return err
}

func (s *Store) hasDefaultGroup(ctx context.Context, platform string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT COUNT(*) FROM business_provider_groups WHERE platform = ? AND enabled = 1 AND is_default = 1`),
		platform,
	).Scan(&count)
	return count > 0, err
}

func (s *Store) clearDefault(ctx context.Context, platform string) error {
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_api_providers SET is_default = 0 WHERE platform = ?`),
		platform,
	)
	return err
}

func (s *Store) hasDefault(ctx context.Context, platform string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT COUNT(*) FROM business_api_providers WHERE platform = ? AND enabled = 1 AND is_default = 1`),
		platform,
	).Scan(&count)
	return count > 0, err
}

func (s *Store) isPostgres() bool {
	return database.IsPostgres(s.driver)
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
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

func scanGroup(row providerScanner) (Group, error) {
	var item Group
	var enabled int
	var isDefault int
	err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Platform,
		&item.Description,
		&item.Tags,
		&item.MatchMode,
		&enabled,
		&isDefault,
		&item.Priority,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Group{}, err
	}
	item.Platform = NormalizePlatform(item.Platform)
	item.Tags = normalizeTagListString(item.Tags)
	item.MatchMode = NormalizeGroupMatchMode(item.MatchMode)
	item.Enabled = enabled != 0
	item.IsDefault = isDefault != 0
	return item, nil
}

func scanMember(row providerScanner) (Member, error) {
	var item Member
	var enabled int
	err := row.Scan(
		&item.ID,
		&item.GroupID,
		&item.Name,
		&item.Platform,
		&item.BaseURL,
		&item.APIKey,
		&item.DefaultModel,
		&enabled,
		&item.Priority,
		&item.Weight,
		&item.MaxConcurrent,
		&item.CooldownSeconds,
		&item.FailureThreshold,
		&item.ConsecutiveFailures,
		&item.Status,
		&item.CooldownUntil,
		&item.SuccessCount,
		&item.FailCount,
		&item.LastUsedAt,
		&item.LastError,
		&item.LastErrorAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Member{}, err
	}
	item.Platform = NormalizePlatform(item.Platform)
	item.Enabled = enabled != 0
	item.Weight = normalizePositiveDefault(item.Weight, defaultMemberWeight)
	item.MaxConcurrent = normalizeNonNegative(item.MaxConcurrent)
	item.CooldownSeconds = normalizePositiveDefault(item.CooldownSeconds, defaultMemberCooldownSeconds)
	item.FailureThreshold = normalizePositiveDefault(item.FailureThreshold, defaultMemberFailureThreshold)
	if item.ConsecutiveFailures < 0 {
		item.ConsecutiveFailures = 0
	}
	item.Status = NormalizeMemberStatus(item.Status)
	if item.Status == "" {
		item.Status = MemberStatusActive
	}
	return item, nil
}

func scanPoolSelection(row providerScanner) (PoolSelection, error) {
	var selection PoolSelection
	var groupEnabled int
	var groupDefault int
	var memberEnabled int
	err := row.Scan(
		&selection.Group.ID,
		&selection.Group.Name,
		&selection.Group.Platform,
		&selection.Group.Description,
		&selection.Group.Tags,
		&selection.Group.MatchMode,
		&groupEnabled,
		&groupDefault,
		&selection.Group.Priority,
		&selection.Group.CreatedAt,
		&selection.Group.UpdatedAt,
		&selection.Member.ID,
		&selection.Member.GroupID,
		&selection.Member.Name,
		&selection.Member.Platform,
		&selection.Member.BaseURL,
		&selection.Member.APIKey,
		&selection.Member.DefaultModel,
		&memberEnabled,
		&selection.Member.Priority,
		&selection.Member.Weight,
		&selection.Member.MaxConcurrent,
		&selection.Member.CooldownSeconds,
		&selection.Member.FailureThreshold,
		&selection.Member.ConsecutiveFailures,
		&selection.Member.Status,
		&selection.Member.CooldownUntil,
		&selection.Member.SuccessCount,
		&selection.Member.FailCount,
		&selection.Member.LastUsedAt,
		&selection.Member.LastError,
		&selection.Member.LastErrorAt,
		&selection.Member.CreatedAt,
		&selection.Member.UpdatedAt,
	)
	if err != nil {
		return PoolSelection{}, err
	}
	selection.Group.Platform = NormalizePlatform(selection.Group.Platform)
	selection.Group.Tags = normalizeTagListString(selection.Group.Tags)
	selection.Group.MatchMode = NormalizeGroupMatchMode(selection.Group.MatchMode)
	selection.Group.Enabled = groupEnabled != 0
	selection.Group.IsDefault = groupDefault != 0
	selection.Member.Platform = NormalizePlatform(selection.Member.Platform)
	selection.Member.Enabled = memberEnabled != 0
	selection.Member.Weight = normalizePositiveDefault(selection.Member.Weight, defaultMemberWeight)
	selection.Member.MaxConcurrent = normalizeNonNegative(selection.Member.MaxConcurrent)
	selection.Member.CooldownSeconds = normalizePositiveDefault(selection.Member.CooldownSeconds, defaultMemberCooldownSeconds)
	selection.Member.FailureThreshold = normalizePositiveDefault(selection.Member.FailureThreshold, defaultMemberFailureThreshold)
	if selection.Member.ConsecutiveFailures < 0 {
		selection.Member.ConsecutiveFailures = 0
	}
	selection.Member.Status = NormalizeMemberStatus(selection.Member.Status)
	if selection.Member.Status == "" {
		selection.Member.Status = MemberStatusActive
	}
	return selection, nil
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

func normalizeGroupInput(input GroupInput) (Group, error) {
	item := Group{
		Name:        strings.TrimSpace(input.Name),
		Platform:    NormalizePlatform(input.Platform),
		Description: strings.TrimSpace(input.Description),
		Tags:        normalizeTagListString(input.Tags),
		MatchMode:   NormalizeGroupMatchMode(input.MatchMode),
		Enabled:     input.Enabled,
		IsDefault:   input.IsDefault,
		Priority:    input.Priority,
	}
	if item.Name == "" {
		item.Name = item.Platform
	}
	if item.Platform == "" {
		return Group{}, fmt.Errorf("platform must be gpt-image or gemini-banana")
	}
	if item.Priority < 0 {
		item.Priority = 0
	}
	return item, nil
}

func NormalizeGroupMatchMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case GroupMatchAny:
		return GroupMatchAny
	case GroupMatchAll:
		return GroupMatchAll
	default:
		return GroupMatchFallback
	}
}

func normalizeTagListString(value string) string {
	tags := normalizeTags(strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' ' || r == '，'
	}))
	return strings.Join(tags, ",")
}

func normalizeTags(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	tags := make([]string, 0, len(values))
	for _, raw := range values {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func normalizeMemberInput(input MemberInput) (Member, error) {
	platform := ""
	rawPlatform := strings.TrimSpace(input.Platform)
	if rawPlatform != "" {
		platform = NormalizePlatform(rawPlatform)
		if platform == "" {
			return Member{}, fmt.Errorf("platform must be gpt-image or gemini-banana")
		}
	}
	item := Member{
		GroupID:          cleanID(input.GroupID),
		Name:             strings.TrimSpace(input.Name),
		Platform:         platform,
		BaseURL:          strings.TrimSpace(input.BaseURL),
		APIKey:           strings.TrimSpace(input.APIKey),
		DefaultModel:     strings.TrimSpace(input.DefaultModel),
		Enabled:          input.Enabled,
		Priority:         input.Priority,
		Weight:           normalizePositiveDefault(input.Weight, defaultMemberWeight),
		MaxConcurrent:    normalizeNonNegative(input.MaxConcurrent),
		CooldownSeconds:  normalizePositiveDefault(input.CooldownSeconds, defaultMemberCooldownSeconds),
		FailureThreshold: normalizePositiveDefault(input.FailureThreshold, defaultMemberFailureThreshold),
		Status:           NormalizeMemberStatus(input.Status),
	}
	if item.GroupID == "" {
		return Member{}, fmt.Errorf("groupId is required")
	}
	if item.Name == "" {
		item.Name = item.Platform
	}
	if item.BaseURL == "" {
		return Member{}, fmt.Errorf("baseUrl is required")
	}
	if item.APIKey == "" {
		return Member{}, fmt.Errorf("apiKey is required")
	}
	if item.DefaultModel == "" && item.Platform != "" {
		item.DefaultModel = defaultModelForPlatform(item.Platform)
	}
	if item.Priority < 0 {
		item.Priority = 0
	}
	if item.Status == "" {
		item.Status = MemberStatusActive
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

func NormalizeMemberStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", MemberStatusActive:
		return MemberStatusActive
	case MemberStatusLimited:
		return MemberStatusLimited
	case MemberStatusUnavailable:
		return MemberStatusUnavailable
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

func normalizePositiveDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func normalizeNonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
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

func newProviderGroupID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("provider_group-%d", time.Now().UnixNano())
	}
	return "provider_group_" + hex.EncodeToString(bytes[:])
}

func newProviderMemberID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("provider_member-%d", time.Now().UnixNano())
	}
	return "provider_member_" + hex.EncodeToString(bytes[:])
}
