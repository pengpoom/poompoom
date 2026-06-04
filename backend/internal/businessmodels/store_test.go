package businessmodels

import (
	"context"
	"os"
	"strings"
	"testing"

	"imagestudio/internal/businessproviders"
	"imagestudio/internal/config"
)

func newTestStore(t *testing.T) *Store {
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
		t.Fatalf("open model store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.db.ExecContext(context.Background(), `TRUNCATE business_image_models`); err != nil {
		t.Fatalf("clear model store: %v", err)
	}
	return store
}

func TestStoreSeedsDefaultModels(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	items, err := store.List(ctx, false)
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(items) < 5 {
		t.Fatalf("models = %d, want defaults", len(items))
	}
	if items[0].ID != "openai/gpt-image-2" || !items[0].IsDefault {
		t.Fatalf("first model = %#v, want default gpt-image-2", items[0])
	}
}

func TestStoreUpdatesDefaultAndCreditCost(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if _, err := store.List(ctx, false); err != nil {
		t.Fatalf("seed models: %v", err)
	}

	item, ok, err := store.SetDefault(ctx, "google/gemini-2.5-flash-image")
	if err != nil {
		t.Fatalf("set default: %v", err)
	}
	if !ok || !item.IsDefault {
		t.Fatalf("default result = %#v ok=%v", item, ok)
	}

	updated, err := store.Update(ctx, "google/gemini-2.5-flash-image", MutationInput{
		ID:             "google/gemini-2.5-flash-image",
		Vendor:         "google",
		VendorLabel:    "Google",
		DisplayName:    "Gemini 2.5 Flash Image",
		Adapter:        AdapterGemini,
		Platform:       businessproviders.PlatformGeminiBanana,
		UpstreamModel:  "gemini-2.5-flash-image",
		Enabled:        true,
		CompareEnabled: true,
		Capabilities:   DefaultCapabilities(),
		CreditCost:     7,
		IsDefault:      true,
		SortOrder:      50,
	})
	if err != nil {
		t.Fatalf("update model: %v", err)
	}
	if updated.CreditCost != 7 {
		t.Fatalf("credit cost = %d, want 7", updated.CreditCost)
	}
	if cost := CreditCostForModel(updated, 3, 1); cost != 21 {
		t.Fatalf("cost = %d, want 21", cost)
	}
}
