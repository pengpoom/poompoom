package businesssettings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/businessproviders"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
	"imagestudio/internal/sqlitedb"
)

const settingsKey = "system"

type Settings struct {
	Site       SiteSettings       `json:"site"`
	User       UserSettings       `json:"user"`
	Email      EmailSettings      `json:"email"`
	Generation GenerationSettings `json:"generation"`
	Billing    BillingSettings    `json:"billing"`
	Runtime    RuntimeSettings    `json:"runtime"`
	Security   SecuritySettings   `json:"security"`
}

type SiteSettings struct {
	Name        string `json:"name"`
	Subtitle    string `json:"subtitle"`
	LogoURL     string `json:"logoUrl"`
	ContactInfo string `json:"contactInfo"`
}

type UserSettings struct {
	DefaultRole    string `json:"defaultRole"`
	DefaultCredits int64  `json:"defaultCredits"`
	Registration   bool   `json:"registration"`
}

type EmailSettings struct {
	SMTPHost string `json:"smtpHost"`
	SMTPPort int    `json:"smtpPort"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	FromName string `json:"fromName"`
}

type GenerationSettings struct {
	DefaultPlatform string `json:"defaultPlatform"`
	DefaultQuality  string `json:"defaultQuality"`
	DefaultSize     string `json:"defaultSize"`
	DefaultCount    int    `json:"defaultCount"`
	MaxCount        int    `json:"maxCount"`
}

type BillingSettings struct {
	GPTImageCost       int64 `json:"gptImageCost"`
	GeminiBananaCost   int64 `json:"geminiBananaCost"`
	RefundOnFailure    bool  `json:"refundOnFailure"`
	RefundPartialCount bool  `json:"refundPartialCount"`
}

type RuntimeSettings struct {
	MaxImageConcurrency      int `json:"maxImageConcurrency"`
	ImageQueueLimit          int `json:"imageQueueLimit"`
	ImageQueueTimeoutSeconds int `json:"imageQueueTimeoutSeconds"`
	MaxUserActiveJobs        int `json:"maxUserActiveJobs"`
	MaxProviderRunningJobs   int `json:"maxProviderRunningJobs"`
	MaxQueuedJobs            int `json:"maxQueuedJobs"`
}

type SecuritySettings struct {
	ImageFileAuthRequired bool `json:"imageFileAuthRequired"`
}

type RuntimeInfo struct {
	SQLitePath                string `json:"sqlitePath"`
	ImageDir                  string `json:"imageDir"`
	ImageFileAuthRequired     bool   `json:"imageFileAuthRequired"`
	LegacyConfigWritable      bool   `json:"legacyConfigWritable"`
	BusinessSettingsTableName string `json:"businessSettingsTableName"`
}

type Response struct {
	Settings Settings    `json:"settings"`
	Runtime  RuntimeInfo `json:"runtime"`
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

func (s *Store) Get(ctx context.Context) (Settings, error) {
	settings, _, err := s.GetWithFound(ctx)
	return settings, err
}

func (s *Store) GetWithFound(ctx context.Context) (Settings, bool, error) {
	settings := Defaults()
	var raw []byte
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT value_json FROM business_system_settings WHERE key = ?`),
		settingsKey,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return settings, false, nil
	}
	if err != nil {
		return Settings{}, false, err
	}
	if len(raw) == 0 {
		return settings, true, nil
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return Settings{}, false, err
	}
	return Normalize(settings), true, nil
}

func (s *Store) Save(ctx context.Context, settings Settings) (Settings, error) {
	next := Normalize(settings)
	raw, err := json.Marshal(next)
	if err != nil {
		return Settings{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_system_settings(key, value_json, created_at, updated_at)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		   value_json = excluded.value_json,
		   updated_at = excluded.updated_at`),
		settingsKey,
		raw,
		now,
		now,
	)
	if err != nil {
		return Settings{}, err
	}
	return next, nil
}

func Defaults() Settings {
	return Settings{
		Site: SiteSettings{
			Name:        "ImageStudio",
			Subtitle:    "图片生成工作台",
			LogoURL:     "",
			ContactInfo: "",
		},
		User: UserSettings{
			DefaultRole:    "user",
			DefaultCredits: 20,
			Registration:   false,
		},
		Email: EmailSettings{
			SMTPHost: "",
			SMTPPort: 587,
			Username: "",
			Password: "",
			From:     "",
			FromName: "ImageStudio",
		},
		Generation: GenerationSettings{
			DefaultPlatform: businessproviders.PlatformGPTImage,
			DefaultQuality:  "high",
			DefaultSize:     "1024x1024",
			DefaultCount:    1,
			MaxCount:        8,
		},
		Billing: BillingSettings{
			GPTImageCost:       1,
			GeminiBananaCost:   1,
			RefundOnFailure:    true,
			RefundPartialCount: true,
		},
		Runtime: RuntimeSettings{
			MaxImageConcurrency:      8,
			ImageQueueLimit:          32,
			ImageQueueTimeoutSeconds: 20,
			MaxUserActiveJobs:        8,
			MaxProviderRunningJobs:   4,
			MaxQueuedJobs:            1000,
		},
		Security: SecuritySettings{
			ImageFileAuthRequired: true,
		},
	}
}

func Normalize(settings Settings) Settings {
	defaults := Defaults()
	settings.Site.Name = strings.TrimSpace(settings.Site.Name)
	if settings.Site.Name == "" {
		settings.Site.Name = defaults.Site.Name
	}
	settings.Site.Subtitle = strings.TrimSpace(settings.Site.Subtitle)
	if settings.Site.Subtitle == "" {
		settings.Site.Subtitle = defaults.Site.Subtitle
	}
	settings.Site.LogoURL = strings.TrimSpace(settings.Site.LogoURL)
	settings.Site.ContactInfo = strings.TrimSpace(settings.Site.ContactInfo)

	settings.User.DefaultRole = normalizeRole(settings.User.DefaultRole)
	if settings.User.DefaultCredits < 0 {
		settings.User.DefaultCredits = 0
	}
	settings.Email.SMTPHost = strings.TrimSpace(settings.Email.SMTPHost)
	if settings.Email.SMTPPort <= 0 {
		settings.Email.SMTPPort = defaults.Email.SMTPPort
	}
	if settings.Email.SMTPPort > 65535 {
		settings.Email.SMTPPort = 65535
	}
	settings.Email.Username = strings.TrimSpace(settings.Email.Username)
	settings.Email.Password = strings.TrimSpace(settings.Email.Password)
	settings.Email.From = strings.TrimSpace(settings.Email.From)
	settings.Email.FromName = strings.TrimSpace(settings.Email.FromName)
	if settings.Email.FromName == "" {
		settings.Email.FromName = defaults.Email.FromName
	}

	settings.Generation.DefaultPlatform = businessproviders.NormalizePlatform(settings.Generation.DefaultPlatform)
	if settings.Generation.DefaultPlatform == "" {
		settings.Generation.DefaultPlatform = defaults.Generation.DefaultPlatform
	}
	settings.Generation.DefaultQuality = normalizeQuality(settings.Generation.DefaultQuality)
	settings.Generation.DefaultSize = normalizeSize(settings.Generation.DefaultSize)
	if settings.Generation.DefaultCount < 1 {
		settings.Generation.DefaultCount = defaults.Generation.DefaultCount
	}
	if settings.Generation.MaxCount < 1 {
		settings.Generation.MaxCount = defaults.Generation.MaxCount
	}
	if settings.Generation.MaxCount > 8 {
		settings.Generation.MaxCount = 8
	}
	if settings.Generation.DefaultCount > settings.Generation.MaxCount {
		settings.Generation.DefaultCount = settings.Generation.MaxCount
	}

	if settings.Billing.GPTImageCost < 0 {
		settings.Billing.GPTImageCost = 0
	}
	if settings.Billing.GeminiBananaCost < 0 {
		settings.Billing.GeminiBananaCost = 0
	}
	if settings.Runtime.MaxImageConcurrency < 1 {
		settings.Runtime.MaxImageConcurrency = defaults.Runtime.MaxImageConcurrency
	}
	if settings.Runtime.MaxImageConcurrency > 128 {
		settings.Runtime.MaxImageConcurrency = 128
	}
	if settings.Runtime.ImageQueueLimit < 0 {
		settings.Runtime.ImageQueueLimit = defaults.Runtime.ImageQueueLimit
	}
	if settings.Runtime.ImageQueueLimit > 10000 {
		settings.Runtime.ImageQueueLimit = 10000
	}
	if settings.Runtime.ImageQueueTimeoutSeconds < 1 {
		settings.Runtime.ImageQueueTimeoutSeconds = defaults.Runtime.ImageQueueTimeoutSeconds
	}
	if settings.Runtime.ImageQueueTimeoutSeconds > 3600 {
		settings.Runtime.ImageQueueTimeoutSeconds = 3600
	}
	if settings.Runtime.MaxUserActiveJobs < 0 {
		settings.Runtime.MaxUserActiveJobs = defaults.Runtime.MaxUserActiveJobs
	}
	if settings.Runtime.MaxUserActiveJobs > 10000 {
		settings.Runtime.MaxUserActiveJobs = 10000
	}
	if settings.Runtime.MaxProviderRunningJobs < 0 {
		settings.Runtime.MaxProviderRunningJobs = defaults.Runtime.MaxProviderRunningJobs
	}
	if settings.Runtime.MaxProviderRunningJobs > 10000 {
		settings.Runtime.MaxProviderRunningJobs = 10000
	}
	if settings.Runtime.MaxQueuedJobs < 0 {
		settings.Runtime.MaxQueuedJobs = defaults.Runtime.MaxQueuedJobs
	}
	if settings.Runtime.MaxQueuedJobs > 1000000 {
		settings.Runtime.MaxQueuedJobs = 1000000
	}
	return settings
}

func Runtime(cfg *config.Config) RuntimeInfo {
	return RuntimeInfo{
		SQLitePath:                cfg.ResolvePath(cfg.Storage.SQLitePath),
		ImageDir:                  cfg.ResolvePath(cfg.Storage.ImageDir),
		ImageFileAuthRequired:     true,
		LegacyConfigWritable:      false,
		BusinessSettingsTableName: "business_system_settings",
	}
}

func WithConfigRuntime(settings Settings, cfg *config.Config) Settings {
	settings = Normalize(settings)
	if cfg == nil {
		return settings
	}
	maxConcurrency, queueLimit, queueTimeout := cfg.ImageQueueConfig()
	settings.Runtime = RuntimeSettings{
		MaxImageConcurrency:      maxConcurrency,
		ImageQueueLimit:          queueLimit,
		ImageQueueTimeoutSeconds: int(queueTimeout / time.Second),
		MaxUserActiveJobs:        settings.Runtime.MaxUserActiveJobs,
		MaxProviderRunningJobs:   settings.Runtime.MaxProviderRunningJobs,
		MaxQueuedJobs:            settings.Runtime.MaxQueuedJobs,
	}
	return Normalize(settings)
}

func CreditCostForPlatform(settings Settings, platform string, count int) int64 {
	settings = Normalize(settings)
	if count < 1 {
		count = 1
	}
	var unitCost int64
	switch businessproviders.NormalizePlatform(platform) {
	case businessproviders.PlatformGeminiBanana:
		unitCost = settings.Billing.GeminiBananaCost
	default:
		unitCost = settings.Billing.GPTImageCost
	}
	return unitCost * int64(count)
}

func (s *Store) init() error {
	if s.isPostgres() {
		return nil
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS business_system_settings (
		key TEXT PRIMARY KEY,
		value_json BLOB NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`)
	return err
}

func (s *Store) isPostgres() bool {
	return database.IsPostgres(s.driver)
}

func (s *Store) rebind(query string) string {
	return database.Rebind(s.driver, query)
}

func normalizeRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "admin":
		return "admin"
	default:
		return "user"
	}
}

func normalizeQuality(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return Defaults().Generation.DefaultQuality
	}
}

func normalizeSize(value string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	switch normalized {
	case "1024x1024", "1536x1024", "1024x1536":
		return normalized
	default:
		return Defaults().Generation.DefaultSize
	}
}
