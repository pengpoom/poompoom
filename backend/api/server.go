package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"imagestudio/handler"
	"imagestudio/internal/accounts"
	"imagestudio/internal/buildinfo"
	"imagestudio/internal/businessauth"
	"imagestudio/internal/businesscodes"
	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
	"imagestudio/internal/businessjobs"
	"imagestudio/internal/businessnotifications"
	"imagestudio/internal/businesspayments"
	"imagestudio/internal/businessproviders"
	"imagestudio/internal/businesssettings"
	"imagestudio/internal/businesstracker"
	"imagestudio/internal/cliproxy"
	"imagestudio/internal/config"
	"imagestudio/internal/middleware"
	"imagestudio/internal/newapi"
	"imagestudio/internal/sub2api"
)

type Server struct {
	cfg                    *config.Config
	runtimeMu              sync.RWMutex
	db                     *sql.DB
	store                  *accounts.Store
	syncClient             *cliproxy.Client
	syncRunMu              sync.RWMutex
	syncRunCache           map[string]*sourceSyncRunResult
	versionMu              sync.Mutex
	versionCache           *versionCheckInfo
	versionCacheUntil      time.Time
	systemUpdateMu         sync.RWMutex
	systemUpdateSnapshot   *systemUpdateStatus
	maintenanceMu          sync.RWMutex
	maintenanceMode        bool
	maintenanceUpdatedAt   time.Time
	maintenanceUpdatedBy   string
	accountRefreshMu       sync.RWMutex
	accountRefreshRun      *accountRefreshRunResult
	staticDir              string
	reqLogs                *imageRequestLogStore
	imageAdmission         *imageAdmissionController
	loginLimiter           *loginRateLimiter
	bootstrapWarningOnce   sync.Once
	businessJobMu          sync.RWMutex
	activeBusinessJobs     map[string]activeBusinessImageJob
	businessJobReconcileMu sync.Mutex
	businessJobWorkerOnce  sync.Once
	businessJobDispatcher  businessJobDispatcher
	officialClientFactory  func(accessToken, proxyURL string, authData map[string]any, requestConfig handler.ImageRequestConfig) imageWorkflowClient
	responsesClientFactory func(accessToken, proxyURL string, authData map[string]any, requestConfig handler.ImageRequestConfig) imageWorkflowClient
	cpaClientFactory       func(baseURL, apiKey string, timeout time.Duration, routeStrategy string) cpaRouteAwareImageWorkflowClient
	newAPIClientFactory    func(cfg *config.Config) *newapi.Client
	sub2apiClientFactory   func(cfg *config.Config) *sub2api.Client
	sourceClientMu         sync.Mutex
	cachedNewAPIClient     *newapi.Client
	cachedNewAPIKey        string
	cachedSub2APIClient    *sub2api.Client
	cachedSub2APIKey       string
}

type requestError struct {
	code    string
	message string
}

type accountRefreshRunResult struct {
	OK         bool   `json:"ok"`
	Running    bool   `json:"running"`
	Error      string `json:"error,omitempty"`
	Total      int    `json:"total"`
	Processed  int    `json:"processed"`
	Refreshed  int    `json:"refreshed"`
	Failed     int    `json:"failed"`
	Current    string `json:"current,omitempty"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

const cpaFixedImageModel = "gpt-image-2"
const maxBulkAccountRefreshWorkers = 4
const authRoleAdmin = "admin"
const authRoleUser = "user"
const defaultAdminUsername = "admin"
const defaultUserUsername = "test"
const defaultAdminPassword = "admin123"
const defaultUserPassword = "test123"
const defaultUserAuthKey = "dev-user-key"
const authSessionCookieName = "image_studio_session"

type authSession struct {
	Username  string
	Email     string
	Role      string
	UserID    string
	AvatarURL string
	ExpiresAt time.Time
}

type loginAccount struct {
	Username  string
	Email     string
	Role      string
	UserID    string
	AvatarURL string
}

func (e *requestError) Error() string {
	return firstNonEmpty(e.message, e.code)
}

func NewServer(cfg *config.Config, store *accounts.Store, syncClient *cliproxy.Client) *Server {
	return NewServerWithDatabase(cfg, store, syncClient, nil)
}

func NewServerWithDatabase(cfg *config.Config, store *accounts.Store, syncClient *cliproxy.Client, db *sql.DB) *Server {
	server := &Server{
		cfg:                cfg,
		db:                 db,
		store:              store,
		syncClient:         syncClient,
		syncRunCache:       map[string]*sourceSyncRunResult{},
		staticDir:          cfg.ResolvePath(cfg.Server.StaticDir),
		reqLogs:            newImageRequestLogStore(),
		imageAdmission:     newImageAdmissionController(),
		loginLimiter:       newLoginRateLimiter(5, 15*time.Minute),
		activeBusinessJobs: map[string]activeBusinessImageJob{},
		officialClientFactory: func(accessToken, proxyURL string, authData map[string]any, requestConfig handler.ImageRequestConfig) imageWorkflowClient {
			return handler.NewChatGPTClientWithProxyAndConfig(
				accessToken,
				firstNonEmpty(stringValue(authData["cookies"]), stringValue(authData["cookie"])),
				proxyURL,
				requestConfig,
			)
		},
		responsesClientFactory: func(accessToken, proxyURL string, authData map[string]any, requestConfig handler.ImageRequestConfig) imageWorkflowClient {
			return handler.NewResponsesClientWithProxyAndConfig(accessToken, proxyURL, authData, requestConfig)
		},
		cpaClientFactory: func(baseURL, apiKey string, timeout time.Duration, routeStrategy string) cpaRouteAwareImageWorkflowClient {
			return newCPAImageClient(baseURL, apiKey, timeout, routeStrategy)
		},
		newAPIClientFactory: func(cfg *config.Config) *newapi.Client {
			timeout := time.Duration(max(10, cfg.NewAPI.RequestTimeout)) * time.Second
			return newapi.New(
				cfg.NewAPI.BaseURL,
				cfg.NewAPI.Username,
				cfg.NewAPI.Password,
				cfg.NewAPI.AccessToken,
				cfg.NewAPI.UserID,
				cfg.NewAPI.SessionCookie,
				timeout,
				cfg.SyncProxyURL(),
			)
		},
		sub2apiClientFactory: func(cfg *config.Config) *sub2api.Client {
			timeout := time.Duration(max(10, cfg.Sub2API.RequestTimeout)) * time.Second
			return sub2api.New(
				cfg.Sub2API.BaseURL,
				cfg.Sub2API.Email,
				cfg.Sub2API.Password,
				cfg.Sub2API.APIKey,
				cfg.Sub2API.GroupID,
				timeout,
				cfg.SyncProxyURL(),
			)
		},
	}
	return server
}

func (s *Server) newBusinessAuthStore() (*businessauth.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businessauth.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businessauth.NewStore(s.cfg)
}

func (s *Server) newBusinessCreditStore() (*businesscredits.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businesscredits.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businesscredits.NewStore(s.cfg)
}

func (s *Server) newBusinessCodeStore() (*businesscodes.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businesscodes.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businesscodes.NewStore(s.cfg)
}

func (s *Server) newBusinessPaymentStore() (*businesspayments.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businesspayments.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businesspayments.NewStore(s.cfg)
}

func (s *Server) newBusinessJobStore() (*businessjobs.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businessjobs.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businessjobs.NewStore(s.cfg)
}

func (s *Server) newBusinessImageStore() (*businessimage.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businessimage.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businessimage.NewStore(s.cfg)
}

func (s *Server) newBusinessSettingsStore() (*businesssettings.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businesssettings.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businesssettings.NewStore(s.cfg)
}

func (s *Server) newBusinessProviderStore() (*businessproviders.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businessproviders.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businessproviders.NewStore(s.cfg)
}

func (s *Server) newBusinessNotificationStore() (*businessnotifications.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businessnotifications.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businessnotifications.NewStore(s.cfg)
}

func (s *Server) newBusinessTrackerStore() (*businesstracker.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businesstracker.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businesstracker.NewStore(s.cfg)
}

func (s *Server) getStore() *accounts.Store {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return s.store
}

func (s *Server) getSyncClient() *cliproxy.Client {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return s.syncClient
}

func (s *Server) getStaticDir() string {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return s.staticDir
}

func (s *Server) swapRuntime(store *accounts.Store, syncClient *cliproxy.Client, staticDir string) *accounts.Store {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	previous := s.store
	s.store = store
	s.syncClient = syncClient
	s.staticDir = staticDir
	return previous
}

func (s *Server) buildSyncClientFromConfig() *cliproxy.Client {
	timeout := time.Duration(max(10, s.cfg.Sync.RequestTimeout)) * time.Second
	return cliproxy.New(s.cfg.Sync.Enabled, s.cfg.Sync.BaseURL, s.cfg.Sync.ManagementKey, s.cfg.Sync.ProviderType, timeout, s.cfg.SyncProxyURL())
}

func (s *Server) getNewAPIClient() *newapi.Client {
	key := newAPIClientCacheKey(s.cfg)
	s.sourceClientMu.Lock()
	defer s.sourceClientMu.Unlock()
	if s.cachedNewAPIClient != nil && s.cachedNewAPIKey == key {
		return s.cachedNewAPIClient
	}
	client := s.newAPIClientFactory(s.cfg)
	s.cachedNewAPIClient = client
	s.cachedNewAPIKey = key
	return client
}

func (s *Server) getSub2APIClient() *sub2api.Client {
	key := sub2APIClientCacheKey(s.cfg)
	s.sourceClientMu.Lock()
	defer s.sourceClientMu.Unlock()
	if s.cachedSub2APIClient != nil && s.cachedSub2APIKey == key {
		return s.cachedSub2APIClient
	}
	client := s.sub2apiClientFactory(s.cfg)
	s.cachedSub2APIClient = client
	s.cachedSub2APIKey = key
	return client
}

func (s *Server) getAccountRefreshRun() *accountRefreshRunResult {
	s.accountRefreshMu.RLock()
	defer s.accountRefreshMu.RUnlock()
	if s.accountRefreshRun == nil {
		return nil
	}
	copy := *s.accountRefreshRun
	return &copy
}

func (s *Server) setAccountRefreshRun(run *accountRefreshRunResult) {
	s.accountRefreshMu.Lock()
	defer s.accountRefreshMu.Unlock()
	if run == nil {
		s.accountRefreshRun = nil
		return
	}
	copy := *run
	s.accountRefreshRun = &copy
}

func (s *Server) finishAccountRefreshRun(run *accountRefreshRunResult) {
	if run == nil {
		return
	}
	run.Running = false
	run.Current = ""
	run.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	run.UpdatedAt = run.FinishedAt
	s.setAccountRefreshRun(run)
}

func newAPIClientCacheKey(cfg *config.Config) string {
	values := []string{
		cfg.NewAPI.BaseURL,
		cfg.NewAPI.Username,
		cfg.NewAPI.Password,
		cfg.NewAPI.AccessToken,
		strconv.Itoa(cfg.NewAPI.UserID),
		cfg.NewAPI.SessionCookie,
		strconv.Itoa(cfg.NewAPI.RequestTimeout),
		cfg.SyncProxyURL(),
	}
	return strings.Join(values, "\x00")
}

func sub2APIClientCacheKey(cfg *config.Config) string {
	values := []string{
		cfg.Sub2API.BaseURL,
		cfg.Sub2API.Email,
		cfg.Sub2API.Password,
		cfg.Sub2API.APIKey,
		cfg.Sub2API.GroupID,
		strconv.Itoa(cfg.Sub2API.RequestTimeout),
		cfg.SyncProxyURL(),
	}
	return strings.Join(values, "\x00")
}

func (s *Server) reloadRuntimeDependencies(previous configPayload) error {
	nextStaticDir := s.cfg.ResolvePath(s.cfg.Server.StaticDir)
	nextSyncClient := s.buildSyncClientFromConfig()
	currentStore := s.getStore()
	nextStore := currentStore

	if storageSettingsChanged(previous, s.buildConfigPayload()) {
		reloadedStore, err := accounts.NewStore(s.cfg)
		if err != nil {
			return err
		}
		snapshot, err := currentStore.Snapshot()
		if err != nil {
			_ = reloadedStore.Close()
			return err
		}
		if err := reloadedStore.ReplaceAllData(snapshot); err != nil {
			_ = reloadedStore.Close()
			return err
		}
		nextStore = reloadedStore
	}
	if err := s.migrateImageFilesIfNeeded(previous, s.buildConfigPayload()); err != nil {
		if nextStore != currentStore && nextStore != nil {
			_ = nextStore.Close()
		}
		return err
	}

	previousStore := s.swapRuntime(nextStore, nextSyncClient, nextStaticDir)
	if previousStore != nil && previousStore != nextStore {
		_ = previousStore.Close()
	}
	return nil
}

func (s *Server) migrateImageFilesIfNeeded(previous, next configPayload) error {
	oldDir := s.cfg.ResolvePath(previous.Storage.ImageDir)
	newDir := s.cfg.ResolvePath(next.Storage.ImageDir)
	if strings.EqualFold(filepath.Clean(oldDir), filepath.Clean(newDir)) {
		return nil
	}
	info, err := os.Stat(oldDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return err
	}
	normalizedNewDir := filepath.Clean(newDir)
	return filepath.Walk(oldDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if strings.HasPrefix(filepath.Clean(path)+string(os.PathSeparator), normalizedNewDir+string(os.PathSeparator)) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(oldDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(newDir, rel)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(targetPath); err == nil {
			return os.Remove(path)
		}
		if err := os.Rename(path, targetPath); err == nil {
			return nil
		}
		if err := copyFile(path, targetPath); err != nil {
			return err
		}
		return os.Remove(path)
	})
}

func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	return nil
}

func storageSettingsChanged(previous, next configPayload) bool {
	return previous.Storage.Backend != next.Storage.Backend ||
		previous.Storage.ConfigBackend != next.Storage.ConfigBackend ||
		previous.Storage.AuthDir != next.Storage.AuthDir ||
		previous.Storage.StateFile != next.Storage.StateFile ||
		previous.Storage.SyncStateDir != next.Storage.SyncStateDir ||
		previous.Storage.ImageDir != next.Storage.ImageDir ||
		previous.Storage.ImageStorage != next.Storage.ImageStorage ||
		previous.Storage.ImageConversationStorage != next.Storage.ImageConversationStorage ||
		previous.Storage.ImageDataStorage != next.Storage.ImageDataStorage ||
		previous.Storage.RedisAddr != next.Storage.RedisAddr ||
		previous.Storage.RedisPassword != next.Storage.RedisPassword ||
		previous.Storage.RedisDB != next.Storage.RedisDB ||
		previous.Storage.RedisPrefix != next.Storage.RedisPrefix ||
		previous.Sync.ProviderType != next.Sync.ProviderType
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("POST /auth/login", http.HandlerFunc(s.handleLogin))
	mux.Handle("POST /auth/logout", http.HandlerFunc(s.handleLogout))
	mux.Handle("GET /auth/register/options", http.HandlerFunc(s.handleRegistrationOptions))
	mux.Handle("POST /auth/register/code", http.HandlerFunc(s.handleSendRegistrationVerificationCode))
	mux.Handle("POST /auth/register", http.HandlerFunc(s.handleRegisterBusinessUser))
	mux.Handle("POST /auth/password-reset/code", http.HandlerFunc(s.handleSendPasswordResetVerificationCode))
	mux.Handle("POST /auth/password-reset", http.HandlerFunc(s.handleResetPasswordByEmailVerification))
	mux.Handle("GET /version", http.HandlerFunc(s.handleVersion))
	mux.Handle("GET /health", http.HandlerFunc(handleHealth))
	mux.Handle("GET /api/site", http.HandlerFunc(s.handleGetPublicSiteSettings))

	mux.Handle("GET /api/config", s.requireAdminAuth(http.HandlerFunc(s.handleGetConfig)))
	mux.Handle("GET /api/config/defaults", s.requireAdminAuth(http.HandlerFunc(s.handleGetDefaultConfig)))
	mux.Handle("PUT /api/config", s.requireAdminAuth(http.HandlerFunc(s.handleUpdateConfig)))
	mux.Handle("POST /api/proxy/test", s.requireAdminAuth(http.HandlerFunc(s.handleProxyTest)))
	mux.Handle("POST /api/integration/test", s.requireAdminAuth(http.HandlerFunc(s.handleIntegrationTest)))
	mux.Handle("POST /api/integration/newapi/token", s.requireAdminAuth(http.HandlerFunc(s.handleNewAPITokenDiscover)))
	mux.Handle("POST /api/integration/sub2api/groups", s.requireAdminAuth(http.HandlerFunc(s.handleSub2APIGroups)))
	mux.Handle("GET /api/startup/check", s.requireAdminAuth(http.HandlerFunc(s.handleStartupCheck)))
	mux.Handle("GET /api/runtime/status", s.requireAdminAuth(http.HandlerFunc(s.handleRuntimeStatus)))
	mux.Handle("GET /api/system/maintenance", s.requireAdminAuth(http.HandlerFunc(s.handleMaintenanceStatus)))
	mux.Handle("PUT /api/system/maintenance", s.requireAdminAuth(http.HandlerFunc(s.handleMaintenanceUpdate)))
	mux.Handle("GET /api/system/update/status", s.requireAdminAuth(http.HandlerFunc(s.handleSystemUpdateStatus)))
	mux.Handle("POST /api/system/update/start", s.requireAdminAuth(http.HandlerFunc(s.handleSystemUpdateStart)))
	mux.Handle("GET /api/diagnostics/export", s.requireAdminAuth(http.HandlerFunc(s.handleExportDiagnostics)))
	mux.Handle("POST /api/tools/admission-stress", s.requireAdminAuth(http.HandlerFunc(s.handleAdmissionStress)))
	mux.Handle("GET /api/sync/status", s.requireAdminAuth(http.HandlerFunc(s.handleSyncStatus)))
	mux.Handle("POST /api/sync/run", s.requireAdminAuth(http.HandlerFunc(s.handleRunSync)))
	mux.Handle("GET /api/business/admin/dashboard", s.requireAdminAuth(http.HandlerFunc(s.handleGetBusinessDashboard)))
	mux.Handle("GET /api/business/system-settings", s.requireAdminAuth(http.HandlerFunc(s.handleGetBusinessSystemSettings)))
	mux.Handle("PUT /api/business/system-settings", s.requireAdminAuth(http.HandlerFunc(s.handleUpdateBusinessSystemSettings)))
	mux.Handle("GET /api/business/users", s.requireAdminAuth(http.HandlerFunc(s.handleListBusinessUsers)))
	mux.Handle("POST /api/business/users", s.requireAdminAuth(http.HandlerFunc(s.handleCreateBusinessUser)))
	mux.Handle("GET /api/business/users/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleGetBusinessUserDetail)))
	mux.Handle("PATCH /api/business/users/{id}/status", s.requireAdminAuth(http.HandlerFunc(s.handleUpdateBusinessUserStatus)))
	mux.Handle("PATCH /api/business/users/{id}/billing-levels", s.requireAdminAuth(http.HandlerFunc(s.handleUpdateBusinessUserBillingLevels)))
	mux.Handle("PATCH /api/business/users/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleUpdateBusinessUser)))
	mux.Handle("DELETE /api/business/users/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleDeleteBusinessUser)))
	mux.Handle("POST /api/business/users/{id}/restore", s.requireAdminAuth(http.HandlerFunc(s.handleRestoreBusinessUser)))
	mux.Handle("DELETE /api/business/users/{id}/purge", s.requireAdminAuth(http.HandlerFunc(s.handlePurgeBusinessUser)))
	mux.Handle("DELETE /api/business/users/{id}/data", s.requireAdminAuth(http.HandlerFunc(s.handleClearBusinessUserData)))
	mux.Handle("PATCH /api/business/users/{id}/password", s.requireAdminAuth(http.HandlerFunc(s.handleResetBusinessUserPassword)))
	mux.Handle("PUT /api/business/users/{id}/credit", s.requireAdminAuth(http.HandlerFunc(s.handleSetBusinessUserCredit)))
	mux.Handle("GET /api/business/users/{id}/subscription", s.requireAdminAuth(http.HandlerFunc(s.handleAdminGetBusinessSubscription)))
	mux.Handle("GET /api/business/admin/usage", s.requireAdminAuth(http.HandlerFunc(s.handleListAllBusinessUsage)))
	mux.Handle("GET /api/business/admin/jobs", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListBusinessImageJobs)))
	mux.Handle("GET /api/business/admin/compare-batches/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminGetBusinessImageCompareBatch)))
	mux.Handle("GET /api/business/storage/report", s.requireAdminAuth(http.HandlerFunc(s.handleBusinessStorageReport)))
	mux.Handle("POST /api/business/storage/backfill-assets", s.requireAdminAuth(http.HandlerFunc(s.handleBackfillBusinessStorageAssets)))
	mux.Handle("GET /api/business/tracker/summary", s.requireAdminAuth(http.HandlerFunc(s.handleBusinessTrackerSummary)))
	mux.Handle("GET /api/business/api-providers", s.requireAdminAuth(http.HandlerFunc(s.handleListBusinessAPIProviders)))
	mux.Handle("POST /api/business/api-providers", s.requireAdminAuth(http.HandlerFunc(s.handleCreateBusinessAPIProvider)))
	mux.Handle("PUT /api/business/api-providers/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleUpdateBusinessAPIProvider)))
	mux.Handle("POST /api/business/api-providers/{id}/default", s.requireAdminAuth(http.HandlerFunc(s.handleSetDefaultBusinessAPIProvider)))
	mux.Handle("POST /api/business/api-providers/{id}/test", s.requireAdminAuth(http.HandlerFunc(s.handleTestBusinessAPIProvider)))
	mux.Handle("DELETE /api/business/api-providers/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleDeleteBusinessAPIProvider)))
	mux.Handle("GET /api/business/provider-pools", s.requireAdminAuth(http.HandlerFunc(s.handleListBusinessProviderPools)))
	mux.Handle("POST /api/business/provider-pools", s.requireAdminAuth(http.HandlerFunc(s.handleCreateBusinessProviderGroup)))
	mux.Handle("POST /api/business/provider-pools/preview", s.requireAdminAuth(http.HandlerFunc(s.handlePreviewBusinessProviderDispatch)))
	mux.Handle("PUT /api/business/provider-pools/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleUpdateBusinessProviderGroup)))
	mux.Handle("POST /api/business/provider-pools/{id}/default", s.requireAdminAuth(http.HandlerFunc(s.handleSetDefaultBusinessProviderGroup)))
	mux.Handle("POST /api/business/provider-pools/{id}/test", s.requireAdminAuth(http.HandlerFunc(s.handleTestBusinessProviderGroup)))
	mux.Handle("DELETE /api/business/provider-pools/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleDeleteBusinessProviderGroup)))
	mux.Handle("POST /api/business/provider-pool-members", s.requireAdminAuth(http.HandlerFunc(s.handleCreateBusinessProviderMember)))
	mux.Handle("PUT /api/business/provider-pool-members/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleUpdateBusinessProviderMember)))
	mux.Handle("POST /api/business/provider-pool-members/{id}/test", s.requireAdminAuth(http.HandlerFunc(s.handleTestBusinessProviderMember)))
	mux.Handle("POST /api/business/provider-pool-members/{id}/recover", s.requireAdminAuth(http.HandlerFunc(s.handleRecoverBusinessProviderMember)))
	mux.Handle("DELETE /api/business/provider-pool-members/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleDeleteBusinessProviderMember)))
	mux.Handle("GET /api/business/admin/image-models", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListImageModels)))
	mux.Handle("POST /api/business/admin/image-models", s.requireAdminAuth(http.HandlerFunc(s.handleAdminCreateImageModel)))
	mux.Handle("PUT /api/business/admin/image-models/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminUpdateImageModel)))
	mux.Handle("POST /api/business/admin/image-models/{id}/default", s.requireAdminAuth(http.HandlerFunc(s.handleAdminSetDefaultImageModel)))
	mux.Handle("POST /api/business/admin/image-models/{id}/test", s.requireAdminAuth(http.HandlerFunc(s.handleAdminTestImageModel)))
	mux.Handle("DELETE /api/business/admin/image-models/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminDeleteImageModel)))
	mux.Handle("GET /api/business/admin/notifications", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListBusinessNotifications)))
	mux.Handle("POST /api/business/admin/notifications", s.requireAdminAuth(http.HandlerFunc(s.handleAdminCreateBusinessNotification)))
	mux.Handle("PUT /api/business/admin/notifications/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminUpdateBusinessNotification)))
	mux.Handle("DELETE /api/business/admin/notifications/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminDeleteBusinessNotification)))
	mux.Handle("GET /api/business/admin/codes", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListBusinessCodes)))
	mux.Handle("POST /api/business/admin/codes", s.requireAdminAuth(http.HandlerFunc(s.handleAdminCreateBusinessCode)))
	mux.Handle("POST /api/business/admin/codes/batch-status", s.requireAdminAuth(http.HandlerFunc(s.handleAdminBatchUpdateBusinessCodeStatus)))
	mux.Handle("PUT /api/business/admin/codes/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminUpdateBusinessCode)))
	mux.Handle("GET /api/business/admin/codes/{id}/usages", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListBusinessCodeUsages)))
	mux.Handle("DELETE /api/business/admin/codes/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminDeleteBusinessCode)))
	mux.Handle("GET /api/business/admin/affiliate/referrals", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListBusinessAffiliateReferrals)))
	mux.Handle("GET /api/business/admin/payment/packages", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListPaymentPackages)))
	mux.Handle("POST /api/business/admin/payment/packages", s.requireAdminAuth(http.HandlerFunc(s.handleAdminCreatePaymentPackage)))
	mux.Handle("PUT /api/business/admin/payment/packages/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminUpdatePaymentPackage)))
	mux.Handle("DELETE /api/business/admin/payment/packages/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminDeletePaymentPackage)))
	mux.Handle("GET /api/business/admin/payment/providers", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListPaymentProviders)))
	mux.Handle("POST /api/business/admin/payment/providers", s.requireAdminAuth(http.HandlerFunc(s.handleAdminCreatePaymentProvider)))
	mux.Handle("PUT /api/business/admin/payment/providers/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminUpdatePaymentProvider)))
	mux.Handle("DELETE /api/business/admin/payment/providers/{id}", s.requireAdminAuth(http.HandlerFunc(s.handleAdminDeletePaymentProvider)))
	mux.Handle("GET /api/business/admin/payment/orders", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListPaymentOrders)))
	mux.Handle("GET /api/business/admin/payment/subscriptions", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListBusinessSubscriptions)))
	mux.Handle("POST /api/business/admin/payment/orders/{id}/complete", s.requireAdminAuth(http.HandlerFunc(s.handleAdminCompletePaymentOrder)))
	mux.Handle("POST /api/business/admin/payment/orders/{id}/cancel", s.requireAdminAuth(http.HandlerFunc(s.handleAdminCancelPaymentOrder)))
	mux.Handle("POST /api/business/admin/payment/orders/{id}/refund", s.requireAdminAuth(http.HandlerFunc(s.handleAdminRefundPaymentOrder)))
	mux.Handle("GET /api/business/admin/payment/orders/{id}/audit", s.requireAdminAuth(http.HandlerFunc(s.handleAdminListPaymentOrderAuditLogs)))
	mux.Handle("GET /api/business/me", s.requireUIAuth(http.HandlerFunc(s.handleGetBusinessMe)))
	mux.Handle("POST /api/business/me/avatar", s.requireUIAuth(http.HandlerFunc(s.handleUploadBusinessMeAvatar)))
	mux.Handle("PATCH /api/business/me/password", s.requireUIAuth(http.HandlerFunc(s.handleChangeBusinessMePassword)))
	mux.Handle("GET /api/business/avatars/{name}", s.requireUIAuth(http.HandlerFunc(s.handleBusinessAvatarFile)))
	mux.Handle("GET /api/business/credit", s.requireUIAuth(http.HandlerFunc(s.handleGetBusinessCredit)))
	mux.Handle("GET /api/business/credit/ledger", s.requireUIAuth(http.HandlerFunc(s.handleListBusinessCreditLedger)))
	mux.Handle("POST /api/business/credit/redeem", s.requireUIAuth(http.HandlerFunc(s.handleRedeemBusinessCode)))
	mux.Handle("GET /api/business/affiliate", s.requireUIAuth(http.HandlerFunc(s.handleGetBusinessAffiliateSummary)))
	mux.Handle("GET /api/business/subscription", s.requireUIAuth(http.HandlerFunc(s.handleGetBusinessSubscription)))
	mux.Handle("GET /api/business/billing-levels", s.requireUIAuth(http.HandlerFunc(s.handleGetBusinessBillingLevels)))
	mux.Handle("GET /api/business/payment/packages", s.requireUIAuth(http.HandlerFunc(s.handleListPaymentPackages)))
	mux.Handle("GET /api/business/payment/methods", s.requireUIAuth(http.HandlerFunc(s.handleListPaymentMethods)))
	mux.Handle("GET /api/business/payment/orders", s.requireUIAuth(http.HandlerFunc(s.handleListPaymentOrders)))
	mux.Handle("POST /api/business/payment/orders", s.requireUIAuth(http.HandlerFunc(s.handleCreatePaymentOrder)))
	mux.Handle("GET /api/business/payment/webhook/easypay", http.HandlerFunc(s.handleEasyPayWebhook))
	mux.Handle("POST /api/business/payment/webhook/easypay", http.HandlerFunc(s.handleEasyPayWebhook))
	mux.Handle("GET /api/business/notifications", s.requireUIAuth(http.HandlerFunc(s.handleListBusinessNotifications)))
	mux.Handle("POST /api/business/notifications/read", s.requireUIAuth(http.HandlerFunc(s.handleMarkBusinessNotificationsRead)))
	mux.Handle("GET /api/business/usage", s.requireUIAuth(http.HandlerFunc(s.handleListBusinessUsage)))
	mux.Handle("GET /api/business/assets", s.requireUIAuth(http.HandlerFunc(s.handleListBusinessAssets)))
	mux.Handle("GET /api/business/image-models", s.requireUIAuth(http.HandlerFunc(s.handleListImageModels)))
	mux.Handle("GET /api/business/jobs", s.requireUIAuth(http.HandlerFunc(s.handleListBusinessImageJobs)))
	mux.Handle("GET /api/business/jobs/{id}", s.requireUIAuth(http.HandlerFunc(s.handleGetBusinessImageJob)))
	mux.Handle("POST /api/business/jobs/{id}/cancel", s.requireUIAuth(http.HandlerFunc(s.handleCancelBusinessImageJob)))
	mux.Handle("GET /api/image/conversations", s.requireUIAuth(http.HandlerFunc(s.handleListImageConversations)))
	mux.Handle("DELETE /api/image/conversations", s.requireUIAuth(http.HandlerFunc(s.handleClearImageConversations)))
	mux.Handle("POST /api/image/conversations/import", s.requireUIAuth(http.HandlerFunc(s.handleImportImageConversations)))
	mux.Handle("GET /api/image/conversations/{id}", s.requireUIAuth(http.HandlerFunc(s.handleGetImageConversation)))
	mux.Handle("PUT /api/image/conversations/{id}", s.requireUIAuth(http.HandlerFunc(s.handleSaveImageConversation)))
	mux.Handle("DELETE /api/image/conversations/{id}", s.requireUIAuth(http.HandlerFunc(s.handleDeleteImageConversation)))
	mux.Handle("GET /api/business/image/conversations", s.requireUIAuth(http.HandlerFunc(s.handleListBusinessImageConversations)))
	mux.Handle("DELETE /api/business/image/conversations", s.requireUIAuth(http.HandlerFunc(s.handleClearBusinessImageConversations)))
	mux.Handle("GET /api/business/image/conversations/{id}", s.requireUIAuth(http.HandlerFunc(s.handleGetBusinessImageConversation)))
	mux.Handle("PATCH /api/business/image/conversations/{id}", s.requireUIAuth(http.HandlerFunc(s.handleRenameBusinessImageConversation)))
	mux.Handle("DELETE /api/business/image/conversations/{id}", s.requireUIAuth(http.HandlerFunc(s.handleDeleteBusinessImageConversation)))
	mux.Handle("POST /api/image/generate", s.requireUIAuth(http.HandlerFunc(s.handleProviderImageGenerateSubmit)))

	mux.Handle("POST /v1/chat/completions", s.requireImageAuth(http.HandlerFunc(s.handleImageChatCompletions)))
	mux.Handle("POST /v1/responses", s.requireImageAuth(http.HandlerFunc(s.handleImageResponses)))
	mux.Handle("GET /v1/models", s.requireImageAuth(http.HandlerFunc(s.handleModels)))
	mux.Handle("GET /v1/files/image/", http.HandlerFunc(s.handleImageFile))

	mux.Handle("/", http.HandlerFunc(s.handleWebApp))

	handler := middleware.RequestID(middleware.Logger(mux))
	return middleware.CORS(handler)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email          string `json:"email"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		TurnstileToken string `json:"turnstileToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if !s.verifyTurnstileForSettings(w, r, s.businessSystemSettingsForContext(r.Context()), turnstileActionLogin, body.TurnstileToken) {
		return
	}

	credential := firstNonEmpty(body.Email, body.Username)
	limitKey := loginRateLimitKey(r, credential)
	if !s.loginLimiter.allow(limitKey) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many login attempts, please try again later"})
		return
	}

	account, ok, err := s.loginAccountForCredentials(r.Context(), credential, body.Password)
	if err != nil {
		slog.Error("login store failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "login store failed"})
		return
	}
	if !ok {
		s.loginLimiter.recordFailure(limitKey)
		message, err := s.loginFailureMessage(r.Context(), credential)
		if err != nil {
			slog.Error("login failure lookup failed", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "login store failed"})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": message})
		return
	}
	if account.Role != authRoleAdmin && !isEmailLoginCredential(credential) {
		s.loginLimiter.recordFailure(limitKey)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "请使用邮箱登录"})
		return
	}
	s.loginLimiter.recordSuccess(limitKey)
	token, session, err := s.createAuthSession(r.Context(), account)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create login session failed"})
		return
	}
	s.setAuthSessionCookie(w, r, token, session.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"token":     token,
		"role":      session.Role,
		"username":  session.Username,
		"email":     session.Email,
		"userId":    session.UserID,
		"avatarUrl": session.AvatarURL,
		"expiresAt": session.ExpiresAt.Format(time.RFC3339),
		"version":   buildinfo.ResolveVersion(s.cfg.App.Version),
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	force := strings.EqualFold(r.URL.Query().Get("force"), "true")
	writeJSON(w, http.StatusOK, s.versionPayload(r.Context(), force))
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	store := s.getStore()
	items, err := store.ListAccounts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAccountQuota(w http.ResponseWriter, r *http.Request) {
	store := s.getStore()
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account id is required"})
		return
	}

	account, err := s.findAccountByID(accountID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}

	refreshRequested := shouldRefreshAccountQuota(r)
	refreshed := false
	refreshError := ""
	if refreshRequested {
		_, refreshErrors, refreshErr := store.RefreshAccounts(r.Context(), []string{account.AccessToken})
		if refreshErr != nil {
			refreshError = refreshErr.Error()
		}
		if len(refreshErrors) > 0 {
			refreshError = firstNonEmpty(refreshErrors[0].Error, refreshError)
		}
		if refreshError == "" {
			if updated, updatedErr := store.GetAccountByToken(account.AccessToken); updatedErr == nil && updated != nil {
				account = *updated
			}
			refreshed = true
		}
	}

	imageGenRemaining, imageGenResetAfter := extractAccountQuota(account.LimitsProgress, "image_gen")
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                    account.ID,
		"email":                 account.Email,
		"status":                account.Status,
		"type":                  account.Type,
		"quota":                 account.Quota,
		"image_gen_remaining":   imageGenRemaining,
		"image_gen_reset_after": imageGenResetAfter,
		"refresh_requested":     refreshRequested,
		"refreshed":             refreshed,
		"refresh_error":         refreshError,
	})
}

func (s *Server) handleCreateAccounts(w http.ResponseWriter, r *http.Request) {
	store := s.getStore()
	if s.configuredImageMode() != "studio" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "导入 Token 仅支持 Studio 模式"})
		return
	}
	var body struct {
		Tokens []string `json:"tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if len(nonEmptyStrings(body.Tokens)) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "tokens is required"})
		return
	}
	added, skipped, err := store.AddAccounts(body.Tokens)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	refreshed, refreshErrors, err := store.RefreshAccounts(r.Context(), body.Tokens)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	items, err := store.ListAccounts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":     items,
		"added":     added,
		"skipped":   skipped,
		"refreshed": refreshed,
		"errors":    refreshErrors,
	})
}

func (s *Server) handleImportAccounts(w http.ResponseWriter, r *http.Request) {
	store := s.getStore()
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid multipart form"})
		return
	}

	files, err := readAuthFilesFromMultipart(r.MultipartForm)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "at least one auth json file is required"})
		return
	}

	imported, importedTokens, skipped, importFailures, err := store.ImportAuthFiles(files)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	refreshed := 0
	refreshErrors := []accounts.RefreshError{}
	if len(importedTokens) > 0 {
		refreshed, refreshErrors, err = store.RefreshAccounts(r.Context(), importedTokens)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}

	items, err := store.ListAccounts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	status := http.StatusOK
	if len(importFailures) > 0 {
		status = http.StatusMultiStatus
	}
	writeJSON(w, status, map[string]any{
		"items":          items,
		"imported":       imported,
		"imported_files": len(importedTokens),
		"duplicates":     skipped,
		"refreshed":      refreshed,
		"errors":         refreshErrors,
		"failed":         importFailures,
	})
}

func (s *Server) handleDeleteAccounts(w http.ResponseWriter, r *http.Request) {
	store := s.getStore()
	var body struct {
		Tokens []string `json:"tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	removed, err := store.DeleteAccounts(body.Tokens)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	items, err := store.ListAccounts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "removed": removed})
}

func (s *Server) handleRefreshAccounts(w http.ResponseWriter, r *http.Request) {
	store := s.getStore()
	var body struct {
		AccessTokens []string `json:"access_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	refreshed, refreshErrors, err := store.RefreshAccounts(r.Context(), body.AccessTokens)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	items, err := store.ListAccounts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":     items,
		"refreshed": refreshed,
		"errors":    refreshErrors,
	})
}

func (s *Server) handleRefreshAllAccounts(w http.ResponseWriter, r *http.Request) {
	if current := s.getAccountRefreshRun(); current != nil && current.Running {
		writeJSON(w, http.StatusOK, map[string]any{"progress": current, "alreadyRunning": true})
		return
	}

	items, err := s.getStore().ListAccounts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	tokens := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.AccessToken) == "" {
			continue
		}
		tokens = append(tokens, item.AccessToken)
	}

	startedAt := time.Now().UTC().Format(time.RFC3339)
	run := &accountRefreshRunResult{
		OK:        true,
		Running:   true,
		Total:     len(tokens),
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	s.setAccountRefreshRun(run)

	if len(tokens) == 0 {
		s.finishAccountRefreshRun(run)
		writeJSON(w, http.StatusOK, map[string]any{"progress": run})
		return
	}

	store := s.getStore()
	go func(tokens []string) {
		refreshed, refreshErrors, refreshErr := store.RefreshAccountsWithOptions(context.Background(), tokens, accounts.RefreshOptions{
			MaxWorkers: maxBulkAccountRefreshWorkers,
			Progress: func(progress accounts.RefreshProgress) {
				run.Refreshed = progress.Refreshed
				run.Failed = progress.Failed
				run.Processed = progress.Processed
				run.Current = progress.Current
				run.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				s.setAccountRefreshRun(run)
			},
		})
		if refreshErr != nil {
			run.OK = false
			run.Error = refreshErr.Error()
		} else if len(refreshErrors) > 0 {
			run.OK = false
			run.Error = firstNonEmpty(refreshErrors[0].Error, "")
		}
		run.Refreshed = refreshed
		run.Failed = len(refreshErrors)
		run.Processed = len(tokens)
		s.finishAccountRefreshRun(run)
	}(append([]string(nil), tokens...))

	writeJSON(w, http.StatusOK, map[string]any{"progress": run})
}

func (s *Server) handleAccountRefreshProgress(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"progress": s.getAccountRefreshRun()})
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	store := s.getStore()
	var body struct {
		AccessToken string `json:"access_token"`
		Type        string `json:"type"`
		Status      string `json:"status"`
		Quota       *int   `json:"quota"`
		Note        string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	update := accounts.AccountUpdate{}
	if strings.TrimSpace(body.Type) != "" {
		update.Type = &body.Type
	}
	if strings.TrimSpace(body.Status) != "" {
		update.Status = &body.Status
	}
	if body.Quota != nil {
		update.Quota = body.Quota
	}
	if strings.TrimSpace(body.Note) != "" {
		update.Note = &body.Note
	}

	item, err := store.UpdateAccount(body.AccessToken, update)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	items, err := store.ListAccounts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"item": item, "items": items})
}

func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	source := firstNonEmpty(r.URL.Query().Get("source"), "cpa")
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("progress_only")), "1") ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("progress_only")), "true") {
		writeJSON(w, http.StatusOK, buildSourceSyncProgressStatus(source, s.getSourceSyncRun(normalizeSyncSource(source))))
		return
	}
	status, err := s.buildSourceSyncStatus(r.Context(), source)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleRunSync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source    string `json:"source"`
		Direction string `json:"direction"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	source := firstNonEmpty(body.Source, r.URL.Query().Get("source"), "cpa")
	result, err := s.runSourceSync(r.Context(), source, body.Direction)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	status, statusErr := s.buildSourceSyncStatus(r.Context(), source)
	if statusErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{"result": result})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result, "status": status})
}

type imageRequestMetadata struct {
	size         string
	quality      string
	promptLength int
}

func (m imageRequestMetadata) applyTo(entry *imageRequestLogEntry) {
	if entry == nil {
		return
	}
	entry.Size = strings.TrimSpace(m.size)
	entry.Quality = strings.TrimSpace(m.quality)
	entry.PromptLength = m.promptLength
}

func newImageRequestMetadata(prompt, size, quality string) imageRequestMetadata {
	return imageRequestMetadata{
		size:         strings.TrimSpace(size),
		quality:      strings.TrimSpace(quality),
		promptLength: len([]rune(strings.TrimSpace(prompt))),
	}
}

func (s *Server) withImageResults(ctx context.Context, operation, responseFormat, preferredAccountID, requestedModel string, responsesEligible bool, run func(client imageWorkflowClient, upstreamModel string) ([]handler.ImageResult, error), r *http.Request) ([]map[string]any, error) {
	return s.withImageResultsWithMetadata(ctx, operation, responseFormat, preferredAccountID, requestedModel, responsesEligible, imageRequestMetadata{}, run, r)
}

func (s *Server) withImageResultsWithMetadata(ctx context.Context, operation, responseFormat, preferredAccountID, requestedModel string, responsesEligible bool, metadata imageRequestMetadata, run func(client imageWorkflowClient, upstreamModel string) ([]handler.ImageResult, error), r *http.Request) ([]map[string]any, error) {
	return s.withImageResultsFilteredWithMetadata(ctx, operation, responseFormat, preferredAccountID, requestedModel, responsesEligible, nil, metadata, run, r)
}

func (s *Server) withImageResultsFiltered(
	ctx context.Context,
	operation, responseFormat, preferredAccountID, requestedModel string,
	responsesEligible bool,
	allowAccount func(accounts.PublicAccount) bool,
	run func(client imageWorkflowClient, upstreamModel string) ([]handler.ImageResult, error),
	r *http.Request,
) ([]map[string]any, error) {
	return s.withImageResultsFilteredWithMetadata(ctx, operation, responseFormat, preferredAccountID, requestedModel, responsesEligible, allowAccount, imageRequestMetadata{}, run, r)
}

func (s *Server) withImageResultsFilteredWithMetadata(
	ctx context.Context,
	operation, responseFormat, preferredAccountID, requestedModel string,
	responsesEligible bool,
	allowAccount func(accounts.PublicAccount) bool,
	metadata imageRequestMetadata,
	run func(client imageWorkflowClient, upstreamModel string) ([]handler.ImageResult, error),
	r *http.Request,
) ([]map[string]any, error) {
	store := s.getStore()
	mode := s.configuredImageMode()
	if mode == "cpa" {
		return s.runPureCPAImageRequest(ctx, operation, responseFormat, requestedModel, strings.TrimSpace(preferredAccountID) != "", metadata, run, r)
	}
	policy, err := parseRequestImageAccountRoutingPolicy(r)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(preferredAccountID) != "" {
		authFile, account, releaseLease, err := store.FindImageAuthByIDWithLease(preferredAccountID)
		if err != nil {
			if errors.Is(err, accounts.ErrSourceAccountNotFound) {
				return nil, newRequestError("source_account_not_found", "原始图片所属账号不存在，请使用普通编辑重试")
			}
			return nil, err
		}
		data, _, err := s.runImageRequest(ctx, authFile, account, releaseLease, accounts.ImageAccountRoutingDecision{}, operation, responseFormat, true, requestedModel, responsesEligible, metadata, run, r)
		return data, err
	}

	attempted := map[string]struct{}{}
	var lastRetryableErr error
	for {
		var (
			authFile     *accounts.LocalAuth
			account      accounts.PublicAccount
			releaseLease func()
			decision     accounts.ImageAccountRoutingDecision
			err          error
		)
		if policy != nil {
			authFile, account, decision, releaseLease, err = store.AcquireImageAuthLeaseWithPolicyFilteredWithDisabledOption(attempted, allowAccount, s.allowDisabledStudioImageAccounts(), policy)
		} else {
			authFile, account, releaseLease, err = store.AcquireImageAuthLeaseFilteredWithDisabledOption(attempted, allowAccount, s.allowDisabledStudioImageAccounts())
		}
		if err != nil {
			return nil, resolveImageAcquireError(mode, err, lastRetryableErr)
		}
		attempted[authFile.AccessToken] = struct{}{}

		data, retryable, err := s.runImageRequest(ctx, authFile, account, releaseLease, decision, operation, responseFormat, false, requestedModel, responsesEligible, metadata, run, r)
		if retryable && len(attempted) < 64 {
			lastRetryableErr = err
			continue
		}
		return data, err
	}
}

func (s *Server) newOfficialWorkflowClient(accessToken string, authData map[string]any) imageWorkflowClient {
	if s != nil && s.officialClientFactory != nil {
		return s.officialClientFactory(accessToken, s.cfg.ChatGPTProxyURL(), authData, s.imageRequestConfig())
	}
	return handler.NewChatGPTClientWithProxyAndConfig(
		accessToken,
		firstNonEmpty(stringValue(authData["cookies"]), stringValue(authData["cookie"])),
		s.cfg.ChatGPTProxyURL(),
		s.imageRequestConfig(),
	)
}

func (s *Server) newResponsesWorkflowClient(accessToken string, authData map[string]any) imageWorkflowClient {
	if s != nil && s.responsesClientFactory != nil {
		return s.responsesClientFactory(accessToken, s.cfg.ChatGPTProxyURL(), authData, s.imageRequestConfig())
	}
	return handler.NewResponsesClientWithProxyAndConfig(
		accessToken,
		s.cfg.ChatGPTProxyURL(),
		authData,
		s.imageRequestConfig(),
	)
}

func (s *Server) newCPAWorkflowClient() cpaRouteAwareImageWorkflowClient {
	timeout := time.Duration(max(10, s.cfg.CPAImageRequestTimeout())) * time.Second
	if s != nil && s.cpaClientFactory != nil {
		return s.cpaClientFactory(
			s.cfg.CPAImageBaseURL(),
			s.cfg.CPAImageAPIKey(),
			timeout,
			s.cfg.CPAImageRouteStrategy(),
		)
	}
	return newCPAImageClient(
		s.cfg.CPAImageBaseURL(),
		s.cfg.CPAImageAPIKey(),
		timeout,
		s.cfg.CPAImageRouteStrategy(),
	)
}

func resolveImageAcquireError(mode string, err, lastRetryableErr error) error {
	if errors.Is(err, accounts.ErrSelectedImageGroupsExhausted) {
		return newRequestError("selected_image_groups_exhausted", "当前选中的图片账号分组已经全部用尽，请调整分组或稍后重试")
	}
	if !errors.Is(err, accounts.ErrNoAvailableImageAuth) {
		return err
	}
	if lastRetryableErr != nil {
		return lastRetryableErr
	}
	if mode == "cpa" {
		return newRequestError("no_cpa_image_accounts", "当前没有可用的图片账号用于 CPA 模式")
	}
	return err
}

func (s *Server) runPureCPAImageRequest(
	ctx context.Context,
	operation string,
	responseFormat string,
	requestedModel string,
	preferredAccount bool,
	metadata imageRequestMetadata,
	run func(client imageWorkflowClient, upstreamModel string) ([]handler.ImageResult, error),
	r *http.Request,
) ([]map[string]any, error) {
	startedAt := time.Now()
	if !s.cfg.CPAImageConfigured() {
		err := newRequestError("cpa_image_not_configured", "CPA 图片接口还未配置，请先在配置管理中设置 CPA base_url 与 api_key")
		entry := imageRequestLogEntry{
			StartedAt:      startedAt.Format(time.RFC3339Nano),
			FinishedAt:     time.Now().Format(time.RFC3339Nano),
			Endpoint:       r.URL.Path,
			Operation:      operation,
			ImageMode:      "cpa",
			Direction:      "cpa",
			Route:          "cpa",
			CPASubroute:    s.cfg.CPAImageRouteStrategy(),
			RequestedModel: requestedModel,
			Preferred:      preferredAccount,
			Success:        false,
			Error:          err.Error(),
		}
		metadata.applyTo(&entry)
		s.logImageRequest(entry)
		return nil, err
	}

	admissionInfo, releaseAdmission, admissionErr := s.acquireImageAdmission(ctx)
	if admissionErr != nil {
		err := admissionErr
		if errors.Is(admissionErr, errImageAdmissionQueueFull) {
			err = newRequestError("image_queue_full", "前方爆满，请稍后使用。")
		} else if errors.Is(admissionErr, errImageAdmissionQueueTimeout) {
			err = newRequestError("image_queue_timeout", "前方爆满，请稍后使用。")
		}
		entry := imageRequestLogEntry{
			StartedAt:            startedAt.Format(time.RFC3339Nano),
			FinishedAt:           time.Now().Format(time.RFC3339Nano),
			Endpoint:             r.URL.Path,
			Operation:            operation,
			ImageMode:            "cpa",
			Direction:            "cpa",
			Route:                "cpa",
			CPASubroute:          s.cfg.CPAImageRouteStrategy(),
			RequestedModel:       requestedModel,
			Preferred:            preferredAccount,
			Success:              false,
			Error:                err.Error(),
			QueueWaitMS:          admissionInfo.QueueWaitMS,
			InflightCountAtStart: admissionInfo.InflightCountAtStart,
		}
		if requestErr, ok := err.(*requestError); ok {
			entry.ErrorCode = requestErr.code
		}
		metadata.applyTo(&entry)
		s.logImageRequest(entry)
		return nil, err
	}
	defer releaseAdmission()
	ctx = withImageAdmissionInfo(ctx, admissionInfo)

	client := s.newCPAWorkflowClient()
	upstreamModel := cpaFixedImageModel
	results, err := run(client, upstreamModel)
	cpaSubroute := client.LastRoute()
	if label := strings.TrimSpace(client.LastModelLabel()); label != "" {
		upstreamModel = label
	}
	if err != nil {
		admissionInfo := imageAdmissionFromContext(ctx)
		entry := imageRequestLogEntry{
			StartedAt:            startedAt.Format(time.RFC3339Nano),
			FinishedAt:           time.Now().Format(time.RFC3339Nano),
			Endpoint:             r.URL.Path,
			Operation:            operation,
			ImageMode:            "cpa",
			Direction:            "cpa",
			Route:                "cpa",
			CPASubroute:          cpaSubroute,
			RequestedModel:       requestedModel,
			UpstreamModel:        upstreamModel,
			Preferred:            preferredAccount,
			Success:              false,
			Error:                err.Error(),
			QueueWaitMS:          admissionInfo.QueueWaitMS,
			InflightCountAtStart: admissionInfo.InflightCountAtStart,
		}
		if requestErr, ok := err.(*requestError); ok {
			entry.ErrorCode = requestErr.code
		}
		metadata.applyTo(&entry)
		s.logImageRequest(entry)
		return nil, err
	}

	admissionInfo = imageAdmissionFromContext(ctx)
	entry := imageRequestLogEntry{
		StartedAt:            startedAt.Format(time.RFC3339Nano),
		FinishedAt:           time.Now().Format(time.RFC3339Nano),
		Endpoint:             r.URL.Path,
		Operation:            operation,
		ImageMode:            "cpa",
		Direction:            "cpa",
		Route:                "cpa",
		CPASubroute:          cpaSubroute,
		RequestedModel:       requestedModel,
		UpstreamModel:        upstreamModel,
		Preferred:            preferredAccount,
		Success:              true,
		QueueWaitMS:          admissionInfo.QueueWaitMS,
		InflightCountAtStart: admissionInfo.InflightCountAtStart,
	}
	metadata.applyTo(&entry)
	s.logImageRequest(entry)
	return buildImageResponse(r, client, results, responseFormat, "", s.cfg.ResolvePath(s.cfg.Storage.ImageDir)), nil
}

func (s *Server) runImageRequest(ctx context.Context, authFile *accounts.LocalAuth, account accounts.PublicAccount, releaseLease func(), routingDecision accounts.ImageAccountRoutingDecision, operation, responseFormat string, preferredAccount bool, requestedModel string, responsesEligible bool, metadata imageRequestMetadata, run func(client imageWorkflowClient, upstreamModel string) ([]handler.ImageResult, error), r *http.Request) ([]map[string]any, bool, error) {
	return s.runImageRequestWithAdmission(ctx, authFile, account, releaseLease, routingDecision, operation, responseFormat, preferredAccount, requestedModel, responsesEligible, metadata, run, r, true)
}

func (s *Server) runImageRequestWithAdmission(ctx context.Context, authFile *accounts.LocalAuth, account accounts.PublicAccount, releaseLease func(), routingDecision accounts.ImageAccountRoutingDecision, operation, responseFormat string, preferredAccount bool, requestedModel string, responsesEligible bool, metadata imageRequestMetadata, run func(client imageWorkflowClient, upstreamModel string) ([]handler.ImageResult, error), r *http.Request, useAdmission bool) ([]map[string]any, bool, error) {
	store := s.getStore()
	if releaseLease != nil {
		defer releaseLease()
	}
	startedAt := time.Now()
	now := time.Now()
	refreshRequired := account.SourceKind == accounts.AccountSourceKindToken || accounts.NeedsImageQuotaRefreshWithTTL(account, now, s.cfg.ImageQuotaRefreshTTL())
	if refreshRequired {
		_, refreshErrors, refreshErr := store.RefreshAccounts(ctx, []string{authFile.AccessToken})
		if refreshErr == nil {
			if refreshed, accountErr := store.GetAccountByToken(authFile.AccessToken); accountErr == nil && refreshed != nil {
				account = *refreshed
			}
		}
		if refreshErr != nil {
			if preferredAccount {
				return nil, false, newRequestError("source_account_quota_refresh_failed", "原始图片所属账号额度刷新失败，请稍后重试")
			}
			return nil, true, refreshErr
		}
		if len(refreshErrors) > 0 && isInvalidRefreshError(refreshErrors[0].Error) {
			store.MarkImageTokenAbnormal(authFile.AccessToken)
			if preferredAccount {
				return nil, false, newRequestError("source_account_unavailable", "原始图片所属账号当前不可用，请使用普通编辑重试")
			}
			return nil, true, errors.New(refreshErrors[0].Error)
		}
		if len(refreshErrors) > 0 {
			if preferredAccount {
				return nil, false, newRequestError("source_account_quota_refresh_failed", firstNonEmpty(refreshErrors[0].Error, "原始图片所属账号额度刷新失败，请稍后重试"))
			}
			return nil, true, errors.New(firstNonEmpty(refreshErrors[0].Error, "image account quota refresh failed"))
		}
		if !isImageAccountUsable(account, s.allowDisabledStudioImageAccounts()) {
			if preferredAccount {
				return nil, false, newRequestError("source_account_unavailable", "原始图片所属账号当前不可用，请使用普通编辑重试")
			}
			return nil, true, fmt.Errorf("image account is unavailable")
		}
	} else if !isImageAccountUsable(account, s.allowDisabledStudioImageAccounts()) {
		if preferredAccount {
			return nil, false, newRequestError("source_account_unavailable", "原始图片所属账号当前不可用，请使用普通编辑重试")
		}
		return nil, true, fmt.Errorf("image account is unavailable")
	}
	if preferredAccount && !isImageAccountUsable(account, s.allowDisabledStudioImageAccounts()) {
		return nil, false, newRequestError("source_account_unavailable", "原始图片所属账号当前不可用，请使用普通编辑重试")
	}
	if routingDecision.PolicyApplied && !store.ImageAccountAllowedForPolicy(authFile.AccessToken, account, &accounts.ImageAccountRoutingPolicy{
		Enabled:        true,
		SortMode:       routingDecision.SortMode,
		ReservePercent: routingDecision.ReservePercent,
		ReserveMode:    "daily_first_seen_percent",
	}) {
		return nil, true, newRequestError("image_account_reserved", "当前账号已触发分组保底阈值，正在切换下一个账号")
	}

	mode := s.configuredImageMode()
	admissionInfo := imageAdmissionInfo{}
	releaseAdmission := func() {}
	if useAdmission {
		var admissionErr error
		admissionInfo, releaseAdmission, admissionErr = s.acquireImageAdmission(ctx)
		if admissionErr != nil {
			err := admissionErr
			if errors.Is(admissionErr, errImageAdmissionQueueFull) {
				err = newRequestError("image_queue_full", "前方爆满，请稍后使用。")
			} else if errors.Is(admissionErr, errImageAdmissionQueueTimeout) {
				err = newRequestError("image_queue_timeout", "前方爆满，请稍后使用。")
			}
			entry := imageRequestLogEntry{
				StartedAt:            startedAt.Format(time.RFC3339Nano),
				FinishedAt:           time.Now().Format(time.RFC3339Nano),
				Endpoint:             r.URL.Path,
				Operation:            operation,
				ImageMode:            mode,
				AccountType:          account.Type,
				AccountEmail:         account.Email,
				AccountFile:          authFile.Name,
				RequestedModel:       requestedModel,
				Preferred:            preferredAccount,
				Success:              false,
				Error:                err.Error(),
				LeaseAcquired:        releaseLease != nil,
				QueueWaitMS:          admissionInfo.QueueWaitMS,
				InflightCountAtStart: admissionInfo.InflightCountAtStart,
			}
			if requestErr, ok := err.(*requestError); ok {
				entry.ErrorCode = requestErr.code
			}
			applyImageRoutingLogFields(routingDecision, &entry)
			metadata.applyTo(&entry)
			s.logImageRequest(entry)
			return nil, false, err
		}
	}
	defer releaseAdmission()
	ctx = withImageAdmissionInfo(ctx, admissionInfo)
	var (
		client         imageWorkflowClient
		upstreamModel  string
		route          string
		direction      string
		imageToolModel string
	)
	if shouldUseCPAImageRoute(mode) {
		if !s.cfg.CPAImageConfigured() {
			err := newRequestError("cpa_image_not_configured", "CPA 图片接口还未配置，请先在配置管理中设置 CPA base_url 与 api_key")
			entry := imageRequestLogEntry{
				StartedAt:            startedAt.Format(time.RFC3339Nano),
				FinishedAt:           time.Now().Format(time.RFC3339Nano),
				Endpoint:             r.URL.Path,
				Operation:            operation,
				ImageMode:            mode,
				Direction:            "cpa",
				Route:                "cpa",
				CPASubroute:          s.cfg.CPAImageRouteStrategy(),
				AccountType:          account.Type,
				AccountEmail:         account.Email,
				AccountFile:          authFile.Name,
				RequestedModel:       requestedModel,
				Preferred:            preferredAccount,
				Success:              false,
				Error:                err.Error(),
				LeaseAcquired:        releaseLease != nil,
				QueueWaitMS:          admissionInfo.QueueWaitMS,
				InflightCountAtStart: admissionInfo.InflightCountAtStart,
			}
			if requestErr, ok := err.(*requestError); ok {
				entry.ErrorCode = requestErr.code
			}
			applyImageRoutingLogFields(routingDecision, &entry)
			metadata.applyTo(&entry)
			s.logImageRequest(entry)
			return nil, false, err
		}
		client = s.newCPAWorkflowClient()
		upstreamModel = cpaFixedImageModel
		route = "cpa"
		direction = "cpa"
	} else {
		route = s.configuredImageRoute(account.Type)
		upstreamModel = s.resolveImageUpstreamModel(requestedModel, account.Type)
		direction = "official"
		if shouldUseOfficialResponses(preferredAccount, responsesEligible, route) {
			client = s.newResponsesWorkflowClient(authFile.AccessToken, authFile.Data)
		} else {
			client = s.newOfficialWorkflowClient(authFile.AccessToken, authFile.Data)
		}
	}
	if setter, ok := client.(interface{ SetRequestedImageModel(string) }); ok {
		setter.SetRequestedImageModel(requestedModel)
	}
	if toolModelProvider, ok := client.(interface{ ImageToolModel() string }); ok {
		imageToolModel = strings.TrimSpace(toolModelProvider.ImageToolModel())
	}
	results, err := run(client, upstreamModel)
	cpaSubroute := ""
	if cpaClient, ok := client.(cpaRouteAwareImageWorkflowClient); ok {
		cpaSubroute = cpaClient.LastRoute()
		if label := strings.TrimSpace(cpaClient.LastModelLabel()); label != "" {
			upstreamModel = label
		}
	}
	if route == "legacy" {
		if routeAwareClient, ok := client.(interface{ LastRoute() string }); ok {
			if actualRoute := strings.TrimSpace(routeAwareClient.LastRoute()); actualRoute != "" {
				route = actualRoute
			}
		}
	}
	if imageToolModel == "" {
		if strings.EqualFold(route, "responses") {
			imageToolModel = strings.TrimSpace(upstreamModel)
		} else {
			imageToolModel = strings.TrimSpace(resolveLoggedImageToolModel(requestedModel))
		}
	}
	admissionInfo = imageAdmissionFromContext(ctx)
	if err != nil {
		store.RecordImageResult(authFile.AccessToken, false)
		entry := imageRequestLogEntry{
			StartedAt:            startedAt.Format(time.RFC3339Nano),
			FinishedAt:           time.Now().Format(time.RFC3339Nano),
			Endpoint:             r.URL.Path,
			Operation:            operation,
			ImageMode:            mode,
			Direction:            direction,
			Route:                route,
			CPASubroute:          cpaSubroute,
			AccountType:          account.Type,
			AccountEmail:         account.Email,
			AccountFile:          authFile.Name,
			RequestedModel:       requestedModel,
			UpstreamModel:        upstreamModel,
			ImageToolModel:       imageToolModel,
			Preferred:            preferredAccount,
			Success:              false,
			Error:                err.Error(),
			LeaseAcquired:        releaseLease != nil,
			QueueWaitMS:          admissionInfo.QueueWaitMS,
			InflightCountAtStart: admissionInfo.InflightCountAtStart,
		}
		if requestErr, ok := err.(*requestError); ok {
			entry.ErrorCode = requestErr.code
		}
		applyImageRoutingLogFields(routingDecision, &entry)
		metadata.applyTo(&entry)
		s.logImageRequest(entry)
		if isImageRateLimitError(err) {
			store.MarkImageAccountLimited(authFile.AccessToken)
			if preferredAccount {
				return nil, false, newRequestError("source_account_rate_limited", "原始图片所属账号当前已限流，请稍后重试或使用普通编辑")
			}
			return nil, true, err
		}
		if isTransientImageStreamError(err) {
			return nil, true, err
		}
		if isInvalidImageTokenError(err) {
			store.MarkImageTokenAbnormal(authFile.AccessToken)
			if preferredAccount {
				return nil, false, newRequestError("source_account_unavailable", "原始图片所属账号当前不可用，请使用普通编辑重试")
			}
			return nil, true, err
		}
		if preferredAccount && isConversationContextError(err) {
			return nil, false, newRequestError("source_context_missing", "原始图片对应会话已失效，请使用普通编辑重试")
		}
		return nil, false, err
	}

	store.RecordImageResult(authFile.AccessToken, true)
	entry := imageRequestLogEntry{
		StartedAt:            startedAt.Format(time.RFC3339Nano),
		FinishedAt:           time.Now().Format(time.RFC3339Nano),
		Endpoint:             r.URL.Path,
		Operation:            operation,
		ImageMode:            mode,
		Direction:            direction,
		Route:                route,
		CPASubroute:          cpaSubroute,
		AccountType:          account.Type,
		AccountEmail:         account.Email,
		AccountFile:          authFile.Name,
		RequestedModel:       requestedModel,
		UpstreamModel:        upstreamModel,
		ImageToolModel:       imageToolModel,
		Preferred:            preferredAccount,
		Success:              true,
		LeaseAcquired:        releaseLease != nil,
		QueueWaitMS:          admissionInfo.QueueWaitMS,
		InflightCountAtStart: admissionInfo.InflightCountAtStart,
	}
	applyImageRoutingLogFields(routingDecision, &entry)
	metadata.applyTo(&entry)
	s.logImageRequest(entry)
	return buildImageResponse(r, client, results, responseFormat, account.ID, s.cfg.ResolvePath(s.cfg.Storage.ImageDir)), false, nil
}

func normalizeRequestedImageModel(requested, fallback string) string {
	model := strings.TrimSpace(requested)
	if model != "" {
		return model
	}
	model = strings.TrimSpace(fallback)
	if model != "" {
		return model
	}
	return "gpt-image-2"
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models := s.availableModels()
	items := make([]map[string]any, 0, len(models))
	for index, model := range models {
		items = append(items, map[string]any{
			"id":       model,
			"object":   "model",
			"created":  1700000000 + index,
			"owned_by": "openai",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": items})
}

func (s *Server) availableModels() []string {
	seen := map[string]struct{}{}
	items := make([]string, 0)
	add := func(value string) {
		model := strings.TrimSpace(value)
		if model == "" {
			return
		}
		if _, ok := seen[model]; ok {
			return
		}
		seen[model] = struct{}{}
		items = append(items, model)
	}

	add("gpt-image-2")
	add(strings.TrimSpace(s.cfg.ChatGPT.Model))

	accountsList, err := s.getStore().ListAccounts()
	hasFree := err != nil
	hasPaid := err != nil
	if err == nil {
		hasFree = false
		hasPaid = false
		for _, account := range accountsList {
			switch account.Type {
			case "Plus", "Pro", "Team":
				hasPaid = true
			case "Free":
				hasFree = true
			}
		}
	}

	if hasFree {
		add(s.cfg.ChatGPT.FreeImageModel)
	}
	if hasPaid {
		add(s.cfg.ChatGPT.PaidImageModel)
	}
	if strings.EqualFold(strings.TrimSpace(s.cfg.ChatGPT.FreeImageModel), "auto") {
		add("auto")
	}
	if s.cfg.ChatGPT.PaidImageRoute == "responses" || s.cfg.ChatGPT.FreeImageRoute == "responses" {
		add("gpt-5.4-mini")
		add("gpt-5.4")
		add("gpt-5.5")
		add("gpt-5-5-thinking")
	}
	return items
}

func (s *Server) handleWebApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	requestPath := strings.TrimPrefix(r.URL.Path, "/")
	asset := resolveStaticAsset(s.getStaticDir(), requestPath)
	if asset == "" {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(strings.ToLower(asset), ".html") {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}
	http.ServeFile(w, r, asset)
}

func (s *Server) requireUIAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok, err := s.authSessionForRequest(r)
		if err != nil {
			writeSessionStoreUnavailable(w, err)
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
			return
		}
		if token := bearerFromRequest(r); token != "" {
			s.setAuthSessionCookie(w, r, token, session.ExpiresAt)
		}
		next.ServeHTTP(w, requestWithAuthSession(r, session))
	})
}

func (s *Server) requireAdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok, err := s.authSessionForRequest(r)
		if err != nil {
			writeSessionStoreUnavailable(w, err)
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
			return
		}
		if session.Role != authRoleAdmin {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "admin role is required"})
			return
		}
		if token := bearerFromRequest(r); token != "" {
			s.setAuthSessionCookie(w, r, token, session.ExpiresAt)
		}
		next.ServeHTTP(w, requestWithAuthSession(r, session))
	})
}

func (s *Server) requireImageAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if session, ok, err := s.authSessionForRequest(r); err != nil {
			writeSessionStoreUnavailable(w, err)
			return
		} else if ok {
			next.ServeHTTP(w, requestWithAuthSession(r, session))
			return
		}
		if s.hasAnyBearer(r, parseKeys(s.cfg.App.APIKey)...) {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
	})
}

func writeSessionStoreUnavailable(w http.ResponseWriter, err error) {
	slog.Warn("session store unavailable", slog.Any("error", err))
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "session store unavailable"})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := bearerFromRequest(r)
	if token == "" {
		token = authCookieFromRequest(r)
	}
	if token != "" {
		store, err := s.newBusinessAuthStore()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "session store failed"})
			return
		}
		defer store.Close()
		if err := store.RevokeSessionByToken(r.Context(), token); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "logout failed"})
			return
		}
	}
	s.clearAuthSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) authSessionForRequest(r *http.Request) (authSession, bool, error) {
	token := bearerFromRequest(r)
	if token == "" {
		token = authCookieFromRequest(r)
	}
	return s.authSessionForToken(r.Context(), token)
}

func (s *Server) authSessionForToken(ctx context.Context, token string) (authSession, bool, error) {
	if strings.TrimSpace(token) == "" {
		return authSession{}, false, nil
	}

	session, ok, err := s.persistentAuthSessionForToken(ctx, token)
	if err != nil {
		return authSession{}, false, err
	}
	if ok {
		return session, true, nil
	}

	session, ok = s.legacyAuthSessionForToken(token)
	return session, ok, nil
}

func (s *Server) persistentAuthSessionForToken(ctx context.Context, token string) (authSession, bool, error) {
	store, err := s.newBusinessAuthStore()
	if err != nil {
		return authSession{}, false, err
	}
	defer store.Close()

	session, ok, err := store.GetSessionByToken(ctx, token)
	if err != nil {
		return authSession{}, false, err
	}
	if !ok {
		return authSession{}, false, nil
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if err != nil {
		return authSession{}, false, err
	}
	return authSession{
		Username:  session.User.Username,
		Email:     session.User.Email,
		Role:      session.User.Role,
		UserID:    session.User.ID,
		AvatarURL: session.User.AvatarURL,
		ExpiresAt: expiresAt,
	}, true, nil
}

func (s *Server) legacyAuthSessionForToken(token string) (authSession, bool) {
	if tokenMatchesAny(token, os.Getenv("ADMIN_AUTH_KEY"), s.cfg.App.AuthKey) {
		return authSession{
			Username:  defaultAdminUsername,
			Role:      businessauth.RoleAdmin,
			UserID:    businessUserIDForRole(businessauth.RoleAdmin),
			ExpiresAt: time.Now().Add(sessionTTL()),
		}, true
	}
	if tokenMatchesAny(token, os.Getenv("USER_AUTH_KEY"), defaultUserAuthKey) {
		return authSession{
			Username:  defaultUserUsername,
			Role:      businessauth.RoleUser,
			UserID:    businessUserIDForRole(businessauth.RoleUser),
			ExpiresAt: time.Now().Add(sessionTTL()),
		}, true
	}
	return authSession{}, false
}

func (s *Server) setAuthSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
}

func (s *Server) clearAuthSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
}

func authCookieFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(authSessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) loginAccountForCredentials(ctx context.Context, username, password string) (loginAccount, bool, error) {
	credential := strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if credential == "" || password == "" {
		return loginAccount{}, false, nil
	}
	store, err := s.newBusinessAuthStore()
	if err != nil {
		return loginAccount{}, false, err
	}
	defer store.Close()
	if err := store.EnsureBootstrapUsers(ctx, s.bootstrapUsers()); err != nil {
		return loginAccount{}, false, err
	}
	user, ok, err := store.Authenticate(ctx, credential, password)
	if err != nil || !ok {
		return loginAccount{}, false, err
	}
	return loginAccount{
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		UserID:    user.ID,
		AvatarURL: user.AvatarURL,
	}, true, nil
}

func (s *Server) loginFailureMessage(ctx context.Context, credential string) (string, error) {
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return "该账号未注册", nil
	}
	store, err := s.newBusinessAuthStore()
	if err != nil {
		return "", err
	}
	defer store.Close()
	user, ok, err := store.GetUserByEmail(ctx, credential)
	if err != nil {
		return "", err
	}
	if !ok {
		user, ok, err = store.GetUserByUsername(ctx, credential)
		if err != nil {
			return "", err
		}
	}
	if !ok {
		return "该账号未注册", nil
	}
	if user.Status != businessauth.StatusActive {
		return "该账号已被封禁，请联系管理员", nil
	}
	return "email or password is invalid", nil
}

func (s *Server) bootstrapUsers() []businessauth.BootstrapUser {
	adminPassword := firstNonEmpty(os.Getenv("ADMIN_PASSWORD"), defaultAdminPassword)
	userPassword := firstNonEmpty(os.Getenv("TEST_PASSWORD"), os.Getenv("USER_PASSWORD"), defaultUserPassword)
	s.bootstrapWarningOnce.Do(func() {
		if adminPassword == defaultAdminPassword {
			slog.Warn("default admin password is active; set ADMIN_PASSWORD before exposing the service")
		}
		if userPassword == defaultUserPassword {
			slog.Warn("default test user password is active; set TEST_PASSWORD before exposing the service")
		}
	})
	return []businessauth.BootstrapUser{
		{
			ID:       businessauth.DefaultAdminUserID,
			Username: firstNonEmpty(os.Getenv("ADMIN_USERNAME"), defaultAdminUsername),
			Email:    firstNonEmpty(os.Getenv("ADMIN_EMAIL"), emailFromUsername(firstNonEmpty(os.Getenv("ADMIN_USERNAME"), defaultAdminUsername))),
			Password: adminPassword,
			Role:     businessauth.RoleAdmin,
		},
		{
			ID:       businessauth.DefaultTestUserID,
			Username: firstNonEmpty(os.Getenv("TEST_USERNAME"), os.Getenv("USER_USERNAME"), defaultUserUsername),
			Email:    firstNonEmpty(os.Getenv("TEST_EMAIL"), os.Getenv("USER_EMAIL"), emailFromUsername(firstNonEmpty(os.Getenv("TEST_USERNAME"), os.Getenv("USER_USERNAME"), defaultUserUsername))),
			Password: userPassword,
			Role:     businessauth.RoleUser,
		},
	}
}

func (s *Server) createAuthSession(ctx context.Context, account loginAccount) (string, authSession, error) {
	token, err := randomSessionToken()
	if err != nil {
		return "", authSession{}, err
	}
	sessionID, err := randomSessionToken()
	if err != nil {
		return "", authSession{}, err
	}
	expiresAt := time.Now().Add(sessionTTL())
	store, err := s.newBusinessAuthStore()
	if err != nil {
		return "", authSession{}, err
	}
	defer store.Close()
	session := authSession{
		Username:  account.Username,
		Email:     account.Email,
		Role:      account.Role,
		UserID:    account.UserID,
		AvatarURL: account.AvatarURL,
		ExpiresAt: expiresAt,
	}
	_, err = store.CreateSession(ctx, sessionID, token, businessauth.User{
		ID:        account.UserID,
		Username:  account.Username,
		Email:     account.Email,
		Role:      account.Role,
		Status:    businessauth.StatusActive,
		AvatarURL: account.AvatarURL,
	}, expiresAt)
	if err != nil {
		return "", authSession{}, err
	}
	return token, session, nil
}

func businessUserIDForRole(role string) string {
	return businessauth.UserIDForRole(role)
}

func businessUserIDForRequest(r *http.Request) string {
	if session, ok := requestAuthSession(r); ok {
		return session.UserID
	}
	return businessUserIDForRole(authRoleUser)
}

func requestAuthSession(r *http.Request) (authSession, bool) {
	value := r.Context().Value(authSessionContextKey{})
	session, ok := value.(authSession)
	return session, ok && session.UserID != ""
}

type authSessionContextKey struct{}

func requestWithAuthSession(r *http.Request, session authSession) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), authSessionContextKey{}, session))
}

func sessionTTL() time.Duration {
	hours := normalizeEnvInt("APP_SESSION_TTL_HOURS", 24)
	if hours <= 0 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

func randomSessionToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "sess_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func (s *Server) hasAnyBearer(r *http.Request, keys ...string) bool {
	token := bearerFromRequest(r)
	if token == "" {
		return false
	}
	return tokenMatchesAny(token, keys...)
}

func bearerFromRequest(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func parseKeys(raw string) []string {
	result := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		if cleaned := strings.TrimSpace(item); cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func tokenMatchesAny(token string, keys ...string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	for _, key := range keys {
		if strings.TrimSpace(key) != "" && token == strings.TrimSpace(key) {
			return true
		}
	}
	return false
}

func resolveStaticAsset(staticDir, requestPath string) string {
	if strings.TrimSpace(staticDir) == "" {
		return ""
	}
	cleaned := strings.Trim(strings.TrimSpace(requestPath), "/")
	candidates := []string{}
	if cleaned == "" {
		candidates = append(candidates, filepath.Join(staticDir, "index.html"))
	} else {
		candidates = append(candidates,
			filepath.Join(staticDir, cleaned),
			filepath.Join(staticDir, cleaned, "index.html"),
			filepath.Join(staticDir, cleaned+".html"),
		)
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	if isStaticAssetRequest(cleaned) {
		return ""
	}
	indexPath := filepath.Join(staticDir, "index.html")
	if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
		return indexPath
	}
	return ""
}

func readAuthFilesFromMultipart(form *multipart.Form) ([]accounts.ImportedAuthFile, error) {
	if form == nil {
		return nil, nil
	}

	keys := make([]string, 0, len(form.File))
	for key := range form.File {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, nil
	}
	sort.Strings(keys)

	files := make([]accounts.ImportedAuthFile, 0)
	for _, key := range keys {
		for _, header := range form.File[key] {
			if header == nil {
				continue
			}
			data, err := readMultipartFile(header)
			if err != nil {
				return nil, err
			}
			files = append(files, accounts.ImportedAuthFile{
				Name: header.Filename,
				Data: data,
			})
		}
	}
	return files, nil
}

func readMultipartFile(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func (s *Server) findAccountByID(accountID string) (accounts.PublicAccount, error) {
	items, err := s.getStore().ListAccounts()
	if err != nil {
		return accounts.PublicAccount{}, err
	}

	target := strings.TrimSpace(accountID)
	for _, item := range items {
		if item.ID == target {
			return item, nil
		}
	}
	return accounts.PublicAccount{}, fmt.Errorf("account not found")
}

func extractAccountQuota(limits []map[string]any, featureName string) (*int, string) {
	target := strings.TrimSpace(strings.ToLower(featureName))
	for _, item := range limits {
		if strings.TrimSpace(strings.ToLower(stringValue(item["feature_name"]))) != target {
			continue
		}

		var remaining *int
		switch typed := item["remaining"].(type) {
		case int:
			value := typed
			remaining = &value
		case int64:
			value := int(typed)
			remaining = &value
		case float64:
			value := int(typed)
			remaining = &value
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				value := int(parsed)
				remaining = &value
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				value := parsed
				remaining = &value
			}
		}

		return remaining, strings.TrimSpace(stringValue(item["reset_after"]))
	}

	return nil, ""
}

func shouldRefreshAccountQuota(r *http.Request) bool {
	value := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("refresh")))
	if value == "" {
		return true
	}
	switch value {
	case "0", "false", "no", "off":
		return false
	default:
		return true
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

func emailFromUsername(username string) string {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return ""
	}
	if strings.Count(username, "@") == 1 {
		return username
	}
	return username + "@local.invalid"
}

func isEmailLoginCredential(credential string) bool {
	return strings.Count(strings.TrimSpace(credential), "@") == 1
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func newRequestError(code, message string) error {
	return &requestError{
		code:    strings.TrimSpace(code),
		message: strings.TrimSpace(message),
	}
}

func requestErrorCode(err error) string {
	var typed *requestError
	if errors.As(err, &typed) {
		return typed.code
	}
	return ""
}

func writeImageRequestError(w http.ResponseWriter, err error) {
	if code := requestErrorCode(err); code != "" {
		writeAPIError(w, http.StatusBadGateway, code, err.Error())
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
}

func isInvalidImageTokenError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, token := range []string{"http 401", "status 401", "unauthorized", "invalid authentication", "invalid_token"} {
		if strings.Contains(message, token) {
			return true
		}
	}
	return false
}

func isImageRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, token := range []string{
		"http 429",
		" http 429",
		"http 429:",
		"http 429 ",
		"status 429",
		"too many requests",
		"rate limit",
		"rate_limit",
		"quota exceeded",
		"resource exhausted",
		"temporarily unavailable",
		"image generation limit",
		"image generation quota",
		"限流",
	} {
		if strings.Contains(message, token) {
			return true
		}
	}
	return false
}

func isTransientImageStreamError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, token := range []string{
		"sse read error",
		"responses sse read error",
		"stream error",
		"internal_error",
		"received from peer",
		"unexpected eof",
		"http2: client connection lost",
		"connection reset by peer",
		"stream closed",
	} {
		if strings.Contains(message, token) {
			return true
		}
	}
	return false
}

func isConversationContextError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "conversation not found") ||
		strings.Contains(message, "conversation_not_found")
}

func isInvalidRefreshError(message string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(message)), "封号") ||
		strings.Contains(strings.ToLower(strings.TrimSpace(message)), "http 401")
}

func isImageAccountUsable(account accounts.PublicAccount, allowDisabled bool) bool {
	return (allowDisabled || account.Status != "禁用") &&
		account.Status != "异常" &&
		account.Status != "限流" &&
		account.Quota > 0
}

func (s *Server) allowDisabledStudioImageAccounts() bool {
	return s != nil &&
		s.cfg != nil &&
		s.configuredImageMode() == "studio" &&
		s.cfg.ChatGPT.StudioAllowDisabledImageAccounts
}

func (s *Server) configuredImageMode() string {
	if normalized, ok := config.NormalizeImageModeForAPI(s.cfg.ChatGPT.ImageMode); ok {
		return normalized
	}
	return "studio"
}

func shouldUseCPAImageRoute(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "cpa")
}

func isPaidImageAccountType(accountType string) bool {
	switch strings.TrimSpace(accountType) {
	case "Plus", "Pro", "Team":
		return true
	default:
		return false
	}
}

func shouldUseOfficialResponses(preferredAccount bool, responsesEligible bool, configuredRoute string) bool {
	if preferredAccount {
		return false
	}
	if !responsesEligible {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(configuredRoute), "responses")
}

func (s *Server) configuredImageRoute(accountType string) string {
	switch strings.TrimSpace(accountType) {
	case "Plus", "Pro", "Team":
		return normalizeConfiguredImageRoute(s.cfg.ChatGPT.PaidImageRoute, "responses")
	default:
		return normalizeConfiguredImageRoute(s.cfg.ChatGPT.FreeImageRoute, "legacy")
	}
}

func (s *Server) imageRequestConfig() handler.ImageRequestConfig {
	return handler.ImageRequestConfig{
		RequestTimeout: time.Duration(max(1, s.cfg.ChatGPT.RequestTimeout)) * time.Second,
		SSETimeout:     time.Duration(max(1, s.cfg.ChatGPT.SSETimeout)) * time.Second,
		PollInterval:   time.Duration(max(1, s.cfg.ChatGPT.PollInterval)) * time.Second,
		PollMaxWait:    time.Duration(max(1, s.cfg.ChatGPT.PollMaxWait)) * time.Second,
	}
}

func (s *Server) resolveImageUpstreamModel(requestedModel, accountType string) string {
	return handler.ResolveImageUpstreamModelWithDefaults(
		requestedModel,
		accountType,
		s.cfg.ChatGPT.FreeImageModel,
		s.cfg.ChatGPT.PaidImageModel,
	)
}

func normalizeConfiguredImageRoute(value, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return strings.ToLower(strings.TrimSpace(fallback))
	case "legacy", "conversation":
		return "legacy"
	case "responses":
		return "responses"
	default:
		return strings.ToLower(strings.TrimSpace(fallback))
	}
}

func resolveLoggedImageToolModel(requestedModel string) string {
	switch strings.ToLower(strings.TrimSpace(requestedModel)) {
	case "gpt-image-1":
		return "gpt-image-1"
	case "gpt-image-2":
		return "gpt-image-2"
	default:
		return ""
	}
}

func (s *Server) logImageRequest(entry imageRequestLogEntry) {
	if s == nil || s.reqLogs == nil {
		return
	}
	s.reqLogs.add(entry)
}

func isStaticAssetRequest(path string) bool {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return false
	}
	if strings.HasPrefix(cleaned, "_next/") {
		return true
	}
	return strings.Contains(filepath.Base(cleaned), ".")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if cleaned := strings.TrimSpace(value); cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}
