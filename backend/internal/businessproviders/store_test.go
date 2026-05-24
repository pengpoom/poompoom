package businessproviders

import (
	"context"
	"os"
	"strings"
	"testing"

	"imagestudio/internal/config"
)

func newTestStore(t *testing.T) (*Store, *config.Config) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	cfg := config.New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 4
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := clearTestProviders(context.Background(), store); err != nil {
		t.Fatalf("clear provider store: %v", err)
	}
	return store, cfg
}

func clearTestProviders(ctx context.Context, store *Store) error {
	_, err := store.db.ExecContext(ctx, `TRUNCATE business_api_providers RESTART IDENTITY CASCADE`)
	return err
}

func TestStoreCreatesAndSelectsDefaultProvider(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	first, err := store.Create(ctx, MutationInput{
		Name:         "first",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://first.example/v1",
		APIKey:       "first-key",
		DefaultModel: "gpt-image-2",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create first provider: %v", err)
	}
	if !first.IsDefault {
		t.Fatal("first enabled provider should become default")
	}

	second, err := store.Create(ctx, MutationInput{
		Name:         "second",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://second.example/v1",
		APIKey:       "second-key",
		DefaultModel: "gpt-image-2",
		Enabled:      true,
		IsDefault:    true,
	})
	if err != nil {
		t.Fatalf("create second provider: %v", err)
	}

	defaultProvider, ok, err := store.DefaultForPlatform(ctx, PlatformGPTImage)
	if err != nil {
		t.Fatalf("get default provider: %v", err)
	}
	if !ok {
		t.Fatal("default provider not found")
	}
	if defaultProvider.ID != second.ID {
		t.Fatalf("default provider = %q, want %q", defaultProvider.ID, second.ID)
	}

	items, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	defaultCount := 0
	for _, item := range items {
		if item.Platform == PlatformGPTImage && item.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("default provider count = %d, want 1", defaultCount)
	}
}

func TestEnsureConfigProviderBackfillsLegacyAPIAccess(t *testing.T) {
	ctx := context.Background()
	store, cfg := newTestStore(t)
	defer store.Close()

	cfg.APIAccess.Platform = PlatformGPTImage
	cfg.APIAccess.BaseURL = "https://legacy.example/v1"
	cfg.APIAccess.APIKey = "legacy-key"

	if err := store.EnsureConfigProvider(ctx, cfg); err != nil {
		t.Fatalf("ensure config provider: %v", err)
	}
	defaultProvider, ok, err := store.DefaultForPlatform(ctx, PlatformGPTImage)
	if err != nil {
		t.Fatalf("get default provider: %v", err)
	}
	if !ok {
		t.Fatal("default provider not found")
	}
	if defaultProvider.BaseURL != "https://legacy.example/v1" || defaultProvider.APIKey != "legacy-key" {
		t.Fatalf("default provider = %#v", defaultProvider)
	}
}

func TestGeminiBananaProviderUsesPlatformDefaultModel(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	item, err := store.Create(ctx, MutationInput{
		Name:     "banana",
		Platform: PlatformGeminiBanana,
		BaseURL:  "https://banana.example",
		APIKey:   "banana-key",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create gemini provider: %v", err)
	}
	if item.DefaultModel != "gemini-2.5-flash-image" {
		t.Fatalf("default model = %q, want gemini-2.5-flash-image", item.DefaultModel)
	}
}
