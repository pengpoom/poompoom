package businessmodels

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/businessproviders"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

const (
	AdapterOpenAIImages = "openai-images"
	AdapterGemini       = "gemini"
)

type Capabilities struct {
	Generate           bool     `json:"generate"`
	Edit               bool     `json:"edit"`
	ReferenceImage     bool     `json:"referenceImage"`
	Mask               bool     `json:"mask"`
	Sizes              []string `json:"sizes,omitempty"`
	Qualities          []string `json:"qualities,omitempty"`
	MaxImages          int      `json:"maxImages"`
	MaxReferenceImages int      `json:"maxReferenceImages"`
}

type Model struct {
	ID             string       `json:"id"`
	Vendor         string       `json:"vendor"`
	VendorLabel    string       `json:"vendorLabel"`
	DisplayName    string       `json:"displayName"`
	Adapter        string       `json:"adapter"`
	Platform       string       `json:"platform"`
	UpstreamModel  string       `json:"upstreamModel"`
	Enabled        bool         `json:"enabled"`
	Preview        bool         `json:"preview,omitempty"`
	CompareEnabled bool         `json:"compareEnabled"`
	Capabilities   Capabilities `json:"capabilities"`
	CreditCost     int64        `json:"creditCost"`
	IsDefault      bool         `json:"isDefault"`
	SortOrder      int          `json:"sortOrder"`
	Availability   Availability `json:"availability,omitempty"`
	CreatedAt      string       `json:"createdAt,omitempty"`
	UpdatedAt      string       `json:"updatedAt,omitempty"`
}

type Availability struct {
	Status                   string   `json:"status"`
	Available                bool     `json:"available"`
	Message                  string   `json:"message"`
	Issues                   []string `json:"issues,omitempty"`
	APIProviderAvailable     bool     `json:"apiProviderAvailable"`
	APIProviderName          string   `json:"apiProviderName,omitempty"`
	APIProviderDefaultModel  string   `json:"apiProviderDefaultModel,omitempty"`
	APIProviderModelMismatch bool     `json:"apiProviderModelMismatch"`
	PoolAvailable            bool     `json:"poolAvailable"`
	PoolName                 string   `json:"poolName,omitempty"`
	PoolMemberID             string   `json:"poolMemberId,omitempty"`
	PoolMemberName           string   `json:"poolMemberName,omitempty"`
	PoolMemberDefaultModel   string   `json:"poolMemberDefaultModel,omitempty"`
	PoolMemberModelMismatch  bool     `json:"poolMemberModelMismatch"`
}

type MutationInput struct {
	ID             string
	Vendor         string
	VendorLabel    string
	DisplayName    string
	Adapter        string
	Platform       string
	UpstreamModel  string
	Enabled        bool
	Preview        bool
	CompareEnabled bool
	Capabilities   Capabilities
	CreditCost     int64
	IsDefault      bool
	SortOrder      int
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

func (s *Store) List(ctx context.Context, includeDisabled bool) ([]Model, error) {
	if err := s.EnsureDefaults(ctx); err != nil {
		return nil, err
	}
	query := `SELECT id, vendor, vendor_label, display_name, adapter, platform, upstream_model,
	        enabled, preview, compare_enabled, generate_enabled, edit_enabled,
	        reference_image_enabled, mask_enabled, sizes, qualities, max_images,
	        max_reference_images, credit_cost, is_default, sort_order, created_at, updated_at
	 FROM business_image_models`
	if !includeDisabled {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY sort_order ASC, vendor_label ASC, display_name ASC`
	rows, err := s.db.QueryContext(ctx, s.rebind(query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Model, 0)
	for rows.Next() {
		item, err := scanModel(rows)
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

func (s *Store) Get(ctx context.Context, id string) (Model, bool, error) {
	id = NormalizeModelID(id)
	if id == "" {
		return Model{}, false, nil
	}
	if err := s.EnsureDefaults(ctx); err != nil {
		return Model{}, false, err
	}
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, vendor, vendor_label, display_name, adapter, platform, upstream_model,
		        enabled, preview, compare_enabled, generate_enabled, edit_enabled,
		        reference_image_enabled, mask_enabled, sizes, qualities, max_images,
		        max_reference_images, credit_cost, is_default, sort_order, created_at, updated_at
		 FROM business_image_models
		 WHERE id = ?`),
		id,
	)
	item, err := scanModel(row)
	if err == sql.ErrNoRows {
		return Model{}, false, nil
	}
	if err != nil {
		return Model{}, false, err
	}
	return item, true, nil
}

func (s *Store) FindByPlatformModel(ctx context.Context, platform string, upstreamModel string) (Model, bool, error) {
	platform = businessproviders.NormalizePlatform(platform)
	upstreamModel = strings.TrimSpace(upstreamModel)
	if upstreamModel == "" {
		return Model{}, false, nil
	}
	if err := s.EnsureDefaults(ctx); err != nil {
		return Model{}, false, err
	}
	if platform != "" {
		item, ok, err := s.findByPlatformModel(ctx, platform, upstreamModel)
		if err != nil || ok {
			return item, ok, err
		}
	}
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, vendor, vendor_label, display_name, adapter, platform, upstream_model,
		        enabled, preview, compare_enabled, generate_enabled, edit_enabled,
		        reference_image_enabled, mask_enabled, sizes, qualities, max_images,
		        max_reference_images, credit_cost, is_default, sort_order, created_at, updated_at
		 FROM business_image_models
		 WHERE LOWER(upstream_model) = LOWER(?)
		 ORDER BY enabled DESC, sort_order ASC, updated_at DESC
		 LIMIT 1`),
		upstreamModel,
	)
	item, err := scanModel(row)
	if err == sql.ErrNoRows {
		return Model{}, false, nil
	}
	if err != nil {
		return Model{}, false, err
	}
	return item, true, nil
}

func (s *Store) findByPlatformModel(ctx context.Context, platform string, upstreamModel string) (Model, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, vendor, vendor_label, display_name, adapter, platform, upstream_model,
		        enabled, preview, compare_enabled, generate_enabled, edit_enabled,
		        reference_image_enabled, mask_enabled, sizes, qualities, max_images,
		        max_reference_images, credit_cost, is_default, sort_order, created_at, updated_at
		 FROM business_image_models
		 WHERE platform = ? AND LOWER(upstream_model) = LOWER(?)
		 ORDER BY enabled DESC, sort_order ASC, updated_at DESC
		 LIMIT 1`),
		platform,
		upstreamModel,
	)
	item, err := scanModel(row)
	if err == sql.ErrNoRows {
		return Model{}, false, nil
	}
	if err != nil {
		return Model{}, false, err
	}
	return item, true, nil
}

func (s *Store) Default(ctx context.Context) (Model, bool, error) {
	if err := s.EnsureDefaults(ctx); err != nil {
		return Model{}, false, err
	}
	row := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, vendor, vendor_label, display_name, adapter, platform, upstream_model,
		        enabled, preview, compare_enabled, generate_enabled, edit_enabled,
		        reference_image_enabled, mask_enabled, sizes, qualities, max_images,
		        max_reference_images, credit_cost, is_default, sort_order, created_at, updated_at
		 FROM business_image_models
		 WHERE enabled = 1
		 ORDER BY is_default DESC, sort_order ASC, updated_at DESC
		 LIMIT 1`),
	)
	item, err := scanModel(row)
	if err == sql.ErrNoRows {
		return Model{}, false, nil
	}
	if err != nil {
		return Model{}, false, err
	}
	return item, true, nil
}

func (s *Store) Create(ctx context.Context, input MutationInput) (Model, error) {
	item, err := normalizeInput(input, true)
	if err != nil {
		return Model{}, err
	}
	if item.ID == "" {
		item.ID = newModelID(item.Vendor, item.UpstreamModel)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	item.CreatedAt = now
	item.UpdatedAt = now
	if item.IsDefault {
		if err := s.clearDefault(ctx); err != nil {
			return Model{}, err
		}
	} else if item.Enabled {
		hasDefault, err := s.hasDefault(ctx)
		if err != nil {
			return Model{}, err
		}
		item.IsDefault = !hasDefault
	}
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_image_models(
			id, vendor, vendor_label, display_name, adapter, platform, upstream_model,
			enabled, preview, compare_enabled, generate_enabled, edit_enabled,
			reference_image_enabled, mask_enabled, sizes, qualities, max_images,
			max_reference_images, credit_cost, is_default, sort_order, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		modelValues(item)...,
	)
	if err != nil {
		return Model{}, err
	}
	return item, nil
}

func (s *Store) Update(ctx context.Context, id string, input MutationInput) (Model, error) {
	id = NormalizeModelID(id)
	if id == "" {
		return Model{}, fmt.Errorf("id is required")
	}
	existing, ok, err := s.Get(ctx, id)
	if err != nil {
		return Model{}, err
	}
	if !ok {
		return Model{}, sql.ErrNoRows
	}
	input.ID = id
	item, err := normalizeInput(input, false)
	if err != nil {
		return Model{}, err
	}
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if item.IsDefault {
		if err := s.clearDefaultExcept(ctx, id); err != nil {
			return Model{}, err
		}
	} else if existing.IsDefault && item.Enabled {
		item.IsDefault = true
	}
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_image_models
		 SET vendor = ?, vendor_label = ?, display_name = ?, adapter = ?, platform = ?, upstream_model = ?,
		     enabled = ?, preview = ?, compare_enabled = ?, generate_enabled = ?, edit_enabled = ?,
		     reference_image_enabled = ?, mask_enabled = ?, sizes = ?, qualities = ?, max_images = ?,
		     max_reference_images = ?, credit_cost = ?, is_default = ?, sort_order = ?, updated_at = ?
		 WHERE id = ?`),
		item.Vendor,
		item.VendorLabel,
		item.DisplayName,
		item.Adapter,
		item.Platform,
		item.UpstreamModel,
		boolInt(item.Enabled),
		boolInt(item.Preview),
		boolInt(item.CompareEnabled),
		boolInt(item.Capabilities.Generate),
		boolInt(item.Capabilities.Edit),
		boolInt(item.Capabilities.ReferenceImage),
		boolInt(item.Capabilities.Mask),
		joinList(item.Capabilities.Sizes),
		joinList(item.Capabilities.Qualities),
		item.Capabilities.MaxImages,
		item.Capabilities.MaxReferenceImages,
		item.CreditCost,
		boolInt(item.IsDefault),
		item.SortOrder,
		item.UpdatedAt,
		id,
	)
	if err != nil {
		return Model{}, err
	}
	return item, nil
}

func (s *Store) Delete(ctx context.Context, id string) (bool, error) {
	id = NormalizeModelID(id)
	if id == "" {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM business_image_models WHERE id = ?`), id)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	return affected > 0, nil
}

func (s *Store) SetDefault(ctx context.Context, id string) (Model, bool, error) {
	id = NormalizeModelID(id)
	if id == "" {
		return Model{}, false, nil
	}
	item, ok, err := s.Get(ctx, id)
	if err != nil || !ok {
		return item, ok, err
	}
	if !item.Enabled {
		return Model{}, false, fmt.Errorf("disabled model cannot be default")
	}
	if err := s.clearDefaultExcept(ctx, id); err != nil {
		return Model{}, false, err
	}
	item.IsDefault = true
	item.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_image_models SET is_default = 1, updated_at = ? WHERE id = ?`),
		item.UpdatedAt,
		id,
	)
	if err != nil {
		return Model{}, false, err
	}
	return item, true, nil
}

func (s *Store) EnsureDefaults(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM business_image_models`)).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, input := range DefaultModels() {
		if _, err := s.Create(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

func DefaultModels() []MutationInput {
	capabilities := DefaultCapabilities()
	return []MutationInput{
		{
			ID:             "openai/gpt-image-2",
			Vendor:         "openai",
			VendorLabel:    "OpenAI",
			DisplayName:    "GPT Image 2",
			Adapter:        AdapterOpenAIImages,
			Platform:       businessproviders.PlatformGPTImage,
			UpstreamModel:  "gpt-image-2",
			Enabled:        true,
			CompareEnabled: true,
			Capabilities:   capabilities,
			CreditCost:     10,
			IsDefault:      true,
			SortOrder:      10,
		},
		{
			ID:             "openai/gpt-image-1.5",
			Vendor:         "openai",
			VendorLabel:    "OpenAI",
			DisplayName:    "GPT Image 1.5",
			Adapter:        AdapterOpenAIImages,
			Platform:       businessproviders.PlatformGPTImage,
			UpstreamModel:  "gpt-image-1.5",
			Enabled:        true,
			CompareEnabled: true,
			Capabilities:   capabilities,
			CreditCost:     10,
			SortOrder:      20,
		},
		{
			ID:             "google/gemini-3.1-flash-image-preview",
			Vendor:         "google",
			VendorLabel:    "Google",
			DisplayName:    "Gemini 3.1 Flash Image Preview",
			Adapter:        AdapterGemini,
			Platform:       businessproviders.PlatformGeminiBanana,
			UpstreamModel:  "gemini-3.1-flash-image-preview",
			Enabled:        true,
			Preview:        true,
			CompareEnabled: true,
			Capabilities:   capabilities,
			CreditCost:     10,
			SortOrder:      30,
		},
		{
			ID:             "google/gemini-3-pro-image-preview",
			Vendor:         "google",
			VendorLabel:    "Google",
			DisplayName:    "Gemini 3 Pro Image Preview",
			Adapter:        AdapterGemini,
			Platform:       businessproviders.PlatformGeminiBanana,
			UpstreamModel:  "gemini-3-pro-image-preview",
			Enabled:        true,
			Preview:        true,
			CompareEnabled: true,
			Capabilities:   capabilities,
			CreditCost:     10,
			SortOrder:      40,
		},
		{
			ID:             "google/gemini-2.5-flash-image",
			Vendor:         "google",
			VendorLabel:    "Google",
			DisplayName:    "Gemini 2.5 Flash Image",
			Adapter:        AdapterGemini,
			Platform:       businessproviders.PlatformGeminiBanana,
			UpstreamModel:  "gemini-2.5-flash-image",
			Enabled:        true,
			CompareEnabled: true,
			Capabilities:   capabilities,
			CreditCost:     10,
			SortOrder:      50,
		},
	}
}

func DefaultCapabilities() Capabilities {
	return Capabilities{
		Generate:           true,
		Edit:               true,
		ReferenceImage:     true,
		Mask:               true,
		Sizes:              []string{"auto", "1:1", "2:3", "3:2", "3:4", "4:3", "9:16", "16:9", "21:9"},
		Qualities:          []string{"low", "medium", "high"},
		MaxImages:          1,
		MaxReferenceImages: 8,
	}
}

func CreditCostForModel(item Model, count int, fallbackUnitCost int64) int64 {
	if count < 1 {
		count = 1
	}
	unitCost := item.CreditCost
	if unitCost < 0 {
		unitCost = 0
	}
	if unitCost == 0 && fallbackUnitCost > 0 {
		unitCost = fallbackUnitCost
	}
	return unitCost * int64(count)
}

func NormalizeModelID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeInput(input MutationInput, allowGeneratedID bool) (Model, error) {
	item := Model{
		ID:             NormalizeModelID(input.ID),
		Vendor:         strings.ToLower(strings.TrimSpace(input.Vendor)),
		VendorLabel:    strings.TrimSpace(input.VendorLabel),
		DisplayName:    strings.TrimSpace(input.DisplayName),
		Adapter:        normalizeAdapter(input.Adapter, input.Platform),
		Platform:       businessproviders.NormalizePlatform(input.Platform),
		UpstreamModel:  strings.TrimSpace(input.UpstreamModel),
		Enabled:        input.Enabled,
		Preview:        input.Preview,
		CompareEnabled: input.CompareEnabled,
		Capabilities:   normalizeCapabilities(input.Capabilities),
		CreditCost:     input.CreditCost,
		IsDefault:      input.IsDefault,
		SortOrder:      input.SortOrder,
	}
	if item.Vendor == "" {
		return Model{}, fmt.Errorf("vendor is required")
	}
	if item.VendorLabel == "" {
		item.VendorLabel = titleLabel(item.Vendor)
	}
	if item.DisplayName == "" {
		item.DisplayName = item.UpstreamModel
	}
	if item.Platform == "" {
		return Model{}, fmt.Errorf("unsupported provider platform")
	}
	if item.UpstreamModel == "" {
		return Model{}, fmt.Errorf("upstreamModel is required")
	}
	if item.Adapter == "" {
		item.Adapter = normalizeAdapter("", item.Platform)
	}
	if item.ID == "" && !allowGeneratedID {
		return Model{}, fmt.Errorf("id is required")
	}
	if item.CreditCost < 0 {
		item.CreditCost = 0
	}
	if item.SortOrder < 0 {
		item.SortOrder = 0
	}
	return item, nil
}

func normalizeCapabilities(value Capabilities) Capabilities {
	if value.MaxImages <= 0 {
		value.MaxImages = 1
	}
	if value.MaxReferenceImages <= 0 {
		value.MaxReferenceImages = 8
	}
	value.Sizes = normalizeList(value.Sizes, DefaultCapabilities().Sizes)
	value.Qualities = normalizeList(value.Qualities, DefaultCapabilities().Qualities)
	return value
}

func normalizeList(values []string, fallback []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, value)
	}
	if len(items) == 0 {
		return append([]string(nil), fallback...)
	}
	return items
}

func normalizeAdapter(adapter string, platform string) string {
	switch strings.ToLower(strings.TrimSpace(adapter)) {
	case AdapterOpenAIImages:
		return AdapterOpenAIImages
	case AdapterGemini:
		return AdapterGemini
	}
	switch businessproviders.NormalizePlatform(platform) {
	case businessproviders.PlatformGeminiBanana:
		return AdapterGemini
	case businessproviders.PlatformGPTImage,
		businessproviders.PlatformDoubao,
		businessproviders.PlatformQwen,
		businessproviders.PlatformBaidu,
		businessproviders.PlatformZAI,
		businessproviders.PlatformTencent,
		businessproviders.PlatformKling,
		businessproviders.PlatformGrok:
		return AdapterOpenAIImages
	default:
		return ""
	}
}

func scanModel(scanner interface {
	Scan(dest ...any) error
}) (Model, error) {
	var item Model
	var enabled, preview, compareEnabled int
	var generateEnabled, editEnabled, referenceImageEnabled, maskEnabled int
	var sizes, qualities string
	var isDefault int
	err := scanner.Scan(
		&item.ID,
		&item.Vendor,
		&item.VendorLabel,
		&item.DisplayName,
		&item.Adapter,
		&item.Platform,
		&item.UpstreamModel,
		&enabled,
		&preview,
		&compareEnabled,
		&generateEnabled,
		&editEnabled,
		&referenceImageEnabled,
		&maskEnabled,
		&sizes,
		&qualities,
		&item.Capabilities.MaxImages,
		&item.Capabilities.MaxReferenceImages,
		&item.CreditCost,
		&isDefault,
		&item.SortOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Model{}, err
	}
	item.Enabled = enabled != 0
	item.Preview = preview != 0
	item.CompareEnabled = compareEnabled != 0
	item.Capabilities.Generate = generateEnabled != 0
	item.Capabilities.Edit = editEnabled != 0
	item.Capabilities.ReferenceImage = referenceImageEnabled != 0
	item.Capabilities.Mask = maskEnabled != 0
	item.Capabilities.Sizes = splitList(sizes)
	item.Capabilities.Qualities = splitList(qualities)
	item.IsDefault = isDefault != 0
	return item, nil
}

func modelValues(item Model) []any {
	return []any{
		item.ID,
		item.Vendor,
		item.VendorLabel,
		item.DisplayName,
		item.Adapter,
		item.Platform,
		item.UpstreamModel,
		boolInt(item.Enabled),
		boolInt(item.Preview),
		boolInt(item.CompareEnabled),
		boolInt(item.Capabilities.Generate),
		boolInt(item.Capabilities.Edit),
		boolInt(item.Capabilities.ReferenceImage),
		boolInt(item.Capabilities.Mask),
		joinList(item.Capabilities.Sizes),
		joinList(item.Capabilities.Qualities),
		item.Capabilities.MaxImages,
		item.Capabilities.MaxReferenceImages,
		item.CreditCost,
		boolInt(item.IsDefault),
		item.SortOrder,
		item.CreatedAt,
		item.UpdatedAt,
	}
}

func (s *Store) hasDefault(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM business_image_models WHERE is_default = 1`)).Scan(&count)
	return count > 0, err
}

func (s *Store) clearDefault(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_image_models SET is_default = 0 WHERE is_default = 1`))
	return err
}

func (s *Store) clearDefaultExcept(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE business_image_models SET is_default = 0 WHERE is_default = 1 AND id != ?`), id)
	return err
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func joinList(values []string) string {
	return strings.Join(normalizeList(values, nil), ",")
}

func titleLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func newModelID(vendor string, upstreamModel string) string {
	vendor = strings.ToLower(strings.TrimSpace(vendor))
	upstreamModel = strings.ToLower(strings.TrimSpace(upstreamModel))
	if vendor != "" && upstreamModel != "" {
		return vendor + "/" + upstreamModel
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("model-%d", time.Now().UnixNano())
	}
	return "model_" + hex.EncodeToString(bytes[:])
}

func (s *Store) isPostgres() bool {
	return database.IsPostgres(s.driver)
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}
