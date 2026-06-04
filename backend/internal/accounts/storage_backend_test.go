package accounts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/config"
)

func TestNewStoreWithCurrentBackendPersistsAccounts(t *testing.T) {
	rootDir := t.TempDir()
	cfg := config.New(rootDir)
	cfg.Storage.Backend = "current"
	cfg.Storage.AuthDir = "data/auths"
	cfg.Storage.StateFile = "data/accounts_state.json"
	cfg.Storage.SyncStateDir = "data/sync_state"
	cfg.Accounts.DefaultQuota = 5
	cfg.Accounts.RefreshWorkers = 1
	cfg.Sync.ProviderType = "codex"

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(current) returned error: %v", err)
	}

	added, skipped, err := store.AddAccounts([]string{"current-token-1"})
	if err != nil {
		t.Fatalf("AddAccounts(current) returned error: %v", err)
	}
	if added != 1 || skipped != 0 {
		t.Fatalf("AddAccounts(current) = added %d skipped %d", added, skipped)
	}

	reloaded, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(current reload) returned error: %v", err)
	}

	items, err := reloaded.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts(current) returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListAccounts(current) count = %d, want 1", len(items))
	}
	if got := strings.TrimSpace(items[0].AccessToken); got != "current-token-1" {
		t.Fatalf("current persisted token = %q, want %q", got, "current-token-1")
	}
}

func TestNewStoreWithRedisBackendPersistsAccounts(t *testing.T) {
	rootDir := t.TempDir()
	cfg := config.New(rootDir)
	cfg.Storage.Backend = "redis"
	cfg.Storage.AuthDir = "data/auths"
	cfg.Storage.StateFile = "data/accounts_state.json"
	cfg.Storage.SyncStateDir = "data/sync_state"
	cfg.Storage.RedisAddr = "127.0.0.1:6379"
	cfg.Storage.RedisPassword = "123456"
	cfg.Storage.RedisDB = 0
	cfg.Storage.RedisPrefix = "imagestudio:studio:test:" + strings.ReplaceAll(rootDir, "\\", ":")
	cfg.Accounts.DefaultQuota = 5
	cfg.Accounts.RefreshWorkers = 1
	cfg.Sync.ProviderType = "codex"

	backend, err := newAccountStorageBackend(cfg, "", "", "", cfg.Sync.ProviderType)
	if err != nil {
		t.Fatalf("newAccountStorageBackend(redis) returned error: %v", err)
	}
	if err := backend.Init(); err != nil {
		t.Skipf("redis backend is not reachable: %v", err)
	}

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(redis) returned error: %v", err)
	}

	added, skipped, err := store.AddAccounts([]string{"redis-token-1"})
	if err != nil {
		t.Fatalf("AddAccounts(redis) returned error: %v", err)
	}
	if added != 1 || skipped != 0 {
		t.Fatalf("AddAccounts(redis) = added %d skipped %d", added, skipped)
	}

	time.Sleep(50 * time.Millisecond)

	reloaded, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(redis reload) returned error: %v", err)
	}

	items, err := reloaded.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts(redis) returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListAccounts(redis) count = %d, want 1", len(items))
	}
	if got := strings.TrimSpace(items[0].AccessToken); got != "redis-token-1" {
		t.Fatalf("redis persisted token = %q, want %q", got, "redis-token-1")
	}
}

func TestUpdateAccountUsesCurrentBackend(t *testing.T) {
	rootDir := t.TempDir()
	cfg := config.New(rootDir)
	cfg.Storage.Backend = "current"
	cfg.Storage.AuthDir = "data/auths"
	cfg.Storage.StateFile = "data/accounts_state.json"
	cfg.Storage.SyncStateDir = "data/sync_state"
	cfg.Accounts.DefaultQuota = 5
	cfg.Accounts.RefreshWorkers = 1
	cfg.Sync.ProviderType = "codex"

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(current) returned error: %v", err)
	}

	if _, _, err := store.AddAccounts([]string{"current-update-token"}); err != nil {
		t.Fatalf("AddAccounts(current) returned error: %v", err)
	}

	note := "updated-note"
	status := "禁用"
	if _, err := store.UpdateAccount("current-update-token", AccountUpdate{Note: &note, Status: &status}); err != nil {
		t.Fatalf("UpdateAccount(current) returned error: %v", err)
	}

	reloaded, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(current reload) returned error: %v", err)
	}

	items, err := reloaded.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts(current) returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListAccounts(current) count = %d, want 1", len(items))
	}
	if items[0].Note != note {
		t.Fatalf("current updated note = %q, want %q", items[0].Note, note)
	}
	if !items[0].Disabled {
		t.Fatal("current updated disabled flag was not persisted")
	}
}

func TestNewStoreLoadsCurrentFiles(t *testing.T) {
	rootDir := t.TempDir()
	authDir := filepath.Join(rootDir, "data", "auths")
	syncDir := filepath.Join(rootDir, "data", "sync_state")
	stateFile := filepath.Join(rootDir, "data", "accounts_state.json")
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatalf("mkdir auth dir: %v", err)
	}
	if err := os.MkdirAll(syncDir, 0o755); err != nil {
		t.Fatalf("mkdir sync dir: %v", err)
	}
	if err := writeJSONFile(filepath.Join(authDir, "legacy.json"), map[string]any{
		"type":         "codex",
		"access_token": "legacy-token",
		"email":        "legacy@example.com",
	}); err != nil {
		t.Fatalf("seed legacy auth: %v", err)
	}
	if err := writeJSONFile(stateFile, stateEnvelope{Accounts: map[string]RuntimeState{
		"legacy.json": {Status: "正常", Quota: 3, QuotaKnown: true},
	}}); err != nil {
		t.Fatalf("seed legacy state: %v", err)
	}
	if err := writeJSONFile(filepath.Join(syncDir, "legacy.json"), SyncState{
		Name:         "legacy.json",
		Origin:       "local",
		LastSyncedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed legacy sync state: %v", err)
	}

	cfg := config.New(rootDir)
	cfg.Storage.Backend = "current"
	cfg.Storage.AuthDir = "data/auths"
	cfg.Storage.StateFile = "data/accounts_state.json"
	cfg.Storage.SyncStateDir = "data/sync_state"
	cfg.Accounts.DefaultQuota = 5
	cfg.Accounts.RefreshWorkers = 1
	cfg.Sync.ProviderType = "codex"

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(current) returned error: %v", err)
	}

	items, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts(current) returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListAccounts(current) count = %d, want 1", len(items))
	}
	if got := strings.TrimSpace(items[0].AccessToken); got != "legacy-token" {
		t.Fatalf("migrated access token = %q, want %q", got, "legacy-token")
	}
	if items[0].Quota != 3 {
		t.Fatalf("migrated quota = %d, want 3", items[0].Quota)
	}
}

func TestReplaceAllDataOverwritesExistingCurrentSnapshot(t *testing.T) {
	rootDir := t.TempDir()

	sourceCfg := config.New(filepath.Join(rootDir, "source"))
	sourceCfg.Storage.Backend = "current"
	sourceCfg.Storage.AuthDir = "data/auths"
	sourceCfg.Storage.StateFile = "data/accounts_state.json"
	sourceCfg.Storage.SyncStateDir = "data/sync_state"
	sourceCfg.Accounts.DefaultQuota = 5
	sourceCfg.Accounts.RefreshWorkers = 1
	sourceCfg.Sync.ProviderType = "codex"

	targetCfg := config.New(filepath.Join(rootDir, "target"))
	targetCfg.Storage.Backend = "current"
	targetCfg.Storage.AuthDir = "data/auths"
	targetCfg.Storage.StateFile = "data/accounts_state.json"
	targetCfg.Storage.SyncStateDir = "data/sync_state"
	targetCfg.Accounts.DefaultQuota = 5
	targetCfg.Accounts.RefreshWorkers = 1
	targetCfg.Sync.ProviderType = "codex"

	sourceStore, err := NewStore(sourceCfg)
	if err != nil {
		t.Fatalf("NewStore(source current) returned error: %v", err)
	}
	if _, _, err := sourceStore.AddAccounts([]string{"source-token"}); err != nil {
		t.Fatalf("AddAccounts(source current) returned error: %v", err)
	}

	targetStore, err := NewStore(targetCfg)
	if err != nil {
		t.Fatalf("NewStore(target current) returned error: %v", err)
	}
	if _, _, err := targetStore.AddAccounts([]string{"stale-target-token"}); err != nil {
		t.Fatalf("AddAccounts(target current) returned error: %v", err)
	}

	snapshot, err := sourceStore.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(source current) returned error: %v", err)
	}
	if err := targetStore.ReplaceAllData(snapshot); err != nil {
		t.Fatalf("ReplaceAllData(target current) returned error: %v", err)
	}

	items, err := targetStore.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts(target current) returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListAccounts(target current) count = %d, want 1", len(items))
	}
	if got := strings.TrimSpace(items[0].AccessToken); got != "source-token" {
		t.Fatalf("replaced current token = %q, want %q", got, "source-token")
	}
}

func TestImageRoutingPolicyPersistsInCurrentBackend(t *testing.T) {
	rootDir := t.TempDir()
	cfg := config.New(rootDir)
	cfg.Storage.Backend = "current"
	cfg.Storage.AuthDir = "data/auths"
	cfg.Storage.StateFile = "data/accounts_state.json"
	cfg.Storage.SyncStateDir = "data/sync_state"
	cfg.Accounts.DefaultQuota = 5
	cfg.Accounts.RefreshWorkers = 1
	cfg.Sync.ProviderType = "codex"

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(current) returned error: %v", err)
	}

	policy := ImageAccountRoutingPolicy{
		Enabled:             true,
		SortMode:            "quota",
		GroupSize:           7,
		EnabledGroupIndexes: []int{1, 3},
		ReserveMode:         "daily_first_seen_percent",
		ReservePercent:      25,
	}
	if err := store.SaveImageRoutingPolicy(policy); err != nil {
		t.Fatalf("SaveImageRoutingPolicy(current) returned error: %v", err)
	}

	reloaded, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(current reload) returned error: %v", err)
	}
	got, err := reloaded.GetImageRoutingPolicy()
	if err != nil {
		t.Fatalf("GetImageRoutingPolicy(current) returned error: %v", err)
	}
	if !got.Enabled || got.SortMode != "quota" || got.GroupSize != 7 || got.ReservePercent != 25 {
		t.Fatalf("current persisted policy = %#v", got)
	}
	if len(got.EnabledGroupIndexes) != 2 || got.EnabledGroupIndexes[0] != 1 || got.EnabledGroupIndexes[1] != 3 {
		t.Fatalf("current persisted group indexes = %#v", got.EnabledGroupIndexes)
	}
}

func TestImageRoutingPolicyPersistsInRedisBackend(t *testing.T) {
	rootDir := t.TempDir()
	cfg := config.New(rootDir)
	cfg.Storage.Backend = "redis"
	cfg.Storage.AuthDir = "data/auths"
	cfg.Storage.StateFile = "data/accounts_state.json"
	cfg.Storage.SyncStateDir = "data/sync_state"
	cfg.Storage.RedisAddr = "127.0.0.1:6379"
	cfg.Storage.RedisPassword = "123456"
	cfg.Storage.RedisDB = 0
	cfg.Storage.RedisPrefix = "imagestudio:studio:test:policy:" + strings.ReplaceAll(rootDir, "\\", ":")
	cfg.Accounts.DefaultQuota = 5
	cfg.Accounts.RefreshWorkers = 1
	cfg.Sync.ProviderType = "codex"

	backend, err := newAccountStorageBackend(cfg, "", "", "", cfg.Sync.ProviderType)
	if err != nil {
		t.Fatalf("newAccountStorageBackend(redis) returned error: %v", err)
	}
	if err := backend.Init(); err != nil {
		t.Skipf("redis backend is not reachable: %v", err)
	}

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(redis) returned error: %v", err)
	}

	policy := ImageAccountRoutingPolicy{
		Enabled:             true,
		SortMode:            "imported_at",
		GroupSize:           9,
		EnabledGroupIndexes: []int{2, 4},
		ReserveMode:         "daily_first_seen_percent",
		ReservePercent:      30,
	}
	if err := store.SaveImageRoutingPolicy(policy); err != nil {
		t.Fatalf("SaveImageRoutingPolicy(redis) returned error: %v", err)
	}

	reloaded, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore(redis reload) returned error: %v", err)
	}
	got, err := reloaded.GetImageRoutingPolicy()
	if err != nil {
		t.Fatalf("GetImageRoutingPolicy(redis) returned error: %v", err)
	}
	if !got.Enabled || got.SortMode != "imported_at" || got.GroupSize != 9 || got.ReservePercent != 30 {
		t.Fatalf("redis persisted policy = %#v", got)
	}
	if len(got.EnabledGroupIndexes) != 2 || got.EnabledGroupIndexes[0] != 2 || got.EnabledGroupIndexes[1] != 4 {
		t.Fatalf("redis persisted group indexes = %#v", got.EnabledGroupIndexes)
	}
}
