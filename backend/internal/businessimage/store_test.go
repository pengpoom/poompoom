package businessimage

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/businessjobs"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func newUsageTestStore(t *testing.T) (*Store, *businessjobs.Store, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	cfg := config.New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	suffix := time.Now().UTC().Format("20060102150405.000000")
	userID := "usage_owner_" + suffix
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM business_image_jobs WHERE user_id = $1", userID)
		_, _ = db.Exec("DELETE FROM business_image_generations WHERE user_id = $1", userID)
	})
	return NewStoreWithDB(db, cfg.Database.Driver), businessjobs.NewStoreWithDB(db, cfg.Database.Driver), userID
}

func TestUsageRecordsPageReportsSource(t *testing.T) {
	imgStore, jobStore, userID := newUsageTestStore(t)
	ctx := context.Background()
	suffix := time.Now().UTC().Format("150405.000000")
	convID := "usage_conv_" + suffix

	webGen, err := imgStore.SaveGeneration(ctx, Generation{
		ID: "gen_web_" + suffix, UserID: userID, ConversationID: convID, TurnID: "turn_web",
		Prompt: "from web", Model: "m", Status: "succeeded",
	})
	if err != nil {
		t.Fatalf("SaveGeneration(web) error: %v", err)
	}
	if _, err := jobStore.Save(ctx, businessjobs.Job{
		UserID: userID, GenerationID: webGen.ID, ConversationID: convID, Status: "succeeded",
	}); err != nil {
		t.Fatalf("Save(web job) error: %v", err)
	}

	apiGen, err := imgStore.SaveGeneration(ctx, Generation{
		ID: "gen_api_" + suffix, UserID: userID, ConversationID: convID, TurnID: "turn_api",
		Prompt: "from api", Model: "m", Status: "succeeded",
	})
	if err != nil {
		t.Fatalf("SaveGeneration(api) error: %v", err)
	}
	if _, err := jobStore.Save(ctx, businessjobs.Job{
		UserID: userID, GenerationID: apiGen.ID, ConversationID: convID, Status: "succeeded",
		APIKeyID: "apikey_source_test",
	}); err != nil {
		t.Fatalf("Save(api job) error: %v", err)
	}

	items, total, err := imgStore.UsageRecordsPage(ctx, UsageRecordFilter{UserID: userID}, 50, 0)
	if err != nil {
		t.Fatalf("UsageRecordsPage() error: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	byID := map[string]UsageRecord{}
	for _, it := range items {
		byID[it.GenerationID] = it
	}
	if got := byID[apiGen.ID].APIKeyID; got != "apikey_source_test" {
		t.Fatalf("api generation APIKeyID = %q, want apikey_source_test", got)
	}
	if got := byID[webGen.ID].APIKeyID; got != "" {
		t.Fatalf("web generation APIKeyID = %q, want empty", got)
	}
}

func TestUsageRecordsPageFilterBySource(t *testing.T) {
	imgStore, jobStore, userID := newUsageTestStore(t)
	ctx := context.Background()
	suffix := time.Now().UTC().Format("150405.000000")
	convID := "usage_src_conv_" + suffix

	webGen, err := imgStore.SaveGeneration(ctx, Generation{
		ID: "gen_src_web_" + suffix, UserID: userID, ConversationID: convID, TurnID: "turn_web",
		Prompt: "from web", Model: "m", Status: "succeeded",
	})
	if err != nil {
		t.Fatalf("SaveGeneration(web) error: %v", err)
	}
	if _, err := jobStore.Save(ctx, businessjobs.Job{
		UserID: userID, GenerationID: webGen.ID, ConversationID: convID, Status: "succeeded",
	}); err != nil {
		t.Fatalf("Save(web job) error: %v", err)
	}

	apiGen, err := imgStore.SaveGeneration(ctx, Generation{
		ID: "gen_src_api_" + suffix, UserID: userID, ConversationID: convID, TurnID: "turn_api",
		Prompt: "from api", Model: "m", Status: "succeeded",
	})
	if err != nil {
		t.Fatalf("SaveGeneration(api) error: %v", err)
	}
	if _, err := jobStore.Save(ctx, businessjobs.Job{
		UserID: userID, GenerationID: apiGen.ID, ConversationID: convID, Status: "succeeded",
		APIKeyID: "apikey_src",
	}); err != nil {
		t.Fatalf("Save(api job) error: %v", err)
	}

	apiItems, apiTotal, err := imgStore.UsageRecordsPage(ctx, UsageRecordFilter{UserID: userID, Source: "api"}, 50, 0)
	if err != nil {
		t.Fatalf("UsageRecordsPage(api) error: %v", err)
	}
	if apiTotal != 1 || len(apiItems) != 1 || apiItems[0].GenerationID != apiGen.ID {
		t.Fatalf("UsageRecordsPage(api) = %d items (total %d), want only api generation", len(apiItems), apiTotal)
	}

	webItems, webTotal, err := imgStore.UsageRecordsPage(ctx, UsageRecordFilter{UserID: userID, Source: "web"}, 50, 0)
	if err != nil {
		t.Fatalf("UsageRecordsPage(web) error: %v", err)
	}
	if webTotal != 1 || len(webItems) != 1 || webItems[0].GenerationID != webGen.ID {
		t.Fatalf("UsageRecordsPage(web) = %d items (total %d), want only web generation", len(webItems), webTotal)
	}

	_, allTotal, err := imgStore.UsageRecordsPage(ctx, UsageRecordFilter{UserID: userID}, 50, 0)
	if err != nil {
		t.Fatalf("UsageRecordsPage(all) error: %v", err)
	}
	if allTotal != 2 {
		t.Fatalf("UsageRecordsPage(all) total = %d, want 2", allTotal)
	}
}
