package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
	"imagestudio/internal/businessjobs"
	"imagestudio/internal/businessproviders"
	"imagestudio/internal/businesssettings"
	"imagestudio/internal/businesstracker"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
)

func TestPostgresCoreBusinessIntegration(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	cfg := config.New(t.TempDir())
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 8
	cfg.Database.MaxIdleConns = 4
	cfg.Database.ConnMaxLifetimeSeconds = 60

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405")
	userID := "pg_core_" + suffix
	conversationID := "conv_" + suffix
	generationID := "gen_" + suffix
	jobID := "job_" + suffix
	fileName := "business-pg-core-" + suffix + ".png"
	providerID := "provider_" + suffix
	trackerID := "track_" + suffix
	t.Cleanup(func() {
		cleanupPostgresCoreRows(context.Background(), db, userID, jobID, conversationID, generationID, fileName, providerID, trackerID)
	})

	creditStore := businesscredits.NewStoreWithDB(db, cfg.Database.Driver)
	if _, _, err := creditStore.SetBalance(ctx, userID, 10, "test"); err != nil {
		t.Fatalf("SetBalance() returned error: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := creditStore.Reserve(context.Background(), userID, 1, generationID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Reserve() returned error: %v", err)
		}
	}
	summary, err := creditStore.Summary(ctx, userID)
	if err != nil {
		t.Fatalf("Summary() returned error: %v", err)
	}
	if summary.Balance != 4 || summary.Spent != 6 {
		t.Fatalf("summary after concurrent reserve = %#v, want balance 4 spent 6", summary)
	}
	totals, err := creditStore.GenerationTotals(ctx, userID, generationID)
	if err != nil {
		t.Fatalf("GenerationTotals() returned error: %v", err)
	}
	if totals.Reserved != 6 || totals.Refunded != 0 {
		t.Fatalf("generation totals = %#v, want reserved 6 refunded 0", totals)
	}

	jobStore := businessjobs.NewStoreWithDB(db, cfg.Database.Driver)
	if _, err := jobStore.Save(ctx, businessjobs.Job{
		ID:             jobID,
		UserID:         userID,
		ConversationID: conversationID,
		GenerationID:   generationID,
		TurnID:         "turn_" + suffix,
		Status:         businessjobs.StatusQueued,
		Stage:          "queued",
		Prompt:         "postgres core smoke",
		RequestedCount: 1,
	}); err != nil {
		t.Fatalf("Save job returned error: %v", err)
	}
	claims := make(chan bool, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := jobStore.ClaimQueued(context.Background(), jobID, userID)
			if err != nil {
				errs := fmt.Errorf("ClaimQueued() returned error: %w", err)
				t.Error(errs)
				claims <- false
				return
			}
			claims <- ok
		}()
	}
	wg.Wait()
	close(claims)
	claimed := 0
	for ok := range claims {
		if ok {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("claimed count = %d, want 1", claimed)
	}
	claimedJob, ok, err := jobStore.Get(ctx, jobID, userID)
	if err != nil || !ok {
		t.Fatalf("Get claimed job returned ok=%v err=%v", ok, err)
	}
	if claimedJob.ClaimedBy == "" || claimedJob.ClaimedAt == "" || claimedJob.LeaseUntil == "" || claimedJob.Attempts != 1 {
		t.Fatalf("claimed job dispatch fields = %#v", claimedJob)
	}
	if _, err := db.ExecContext(ctx, `UPDATE business_image_jobs SET lease_until = $1 WHERE id = $2`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), jobID); err != nil {
		t.Fatalf("expire job lease returned error: %v", err)
	}
	claimedJob, ok, err = jobStore.ClaimQueuedBy(ctx, jobID, userID, "worker-retry")
	if err != nil || !ok {
		t.Fatalf("ClaimQueuedBy after lease expiry returned ok=%v err=%v", ok, err)
	}
	if claimedJob.ClaimedBy != "worker-retry" || claimedJob.Attempts != 2 {
		t.Fatalf("reclaimed job = %#v, want worker-retry attempt 2", claimedJob)
	}

	deferredJobID := jobID + "_deferred"
	if _, err := jobStore.Save(ctx, businessjobs.Job{
		ID:             deferredJobID,
		UserID:         userID,
		ConversationID: conversationID,
		GenerationID:   deferredJobID,
		TurnID:         "turn_deferred_" + suffix,
		Status:         businessjobs.StatusQueued,
		Stage:          "queued",
		Prompt:         "postgres deferred job smoke",
		RequestedCount: 1,
		NextRunAt:      time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("Save deferred job returned error: %v", err)
	}
	if _, ok, err := jobStore.ClaimQueuedBy(ctx, deferredJobID, userID, "worker-future"); err != nil || ok {
		t.Fatalf("ClaimQueuedBy future job returned ok=%v err=%v, want false nil", ok, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE business_image_jobs SET next_run_at = $1 WHERE id = $2`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), deferredJobID); err != nil {
		t.Fatalf("make deferred job runnable returned error: %v", err)
	}
	if _, ok, err := jobStore.ClaimQueuedBy(ctx, deferredJobID, userID, "worker-now"); err != nil || !ok {
		t.Fatalf("ClaimQueuedBy runnable deferred job returned ok=%v err=%v, want true nil", ok, err)
	}

	imageStore := businessimage.NewStoreWithDB(db, cfg.Database.Driver)
	if _, err := imageStore.UpsertConversation(ctx, businessimage.Conversation{
		ID:     conversationID,
		UserID: userID,
		Title:  "PostgreSQL core",
	}); err != nil {
		t.Fatalf("UpsertConversation() returned error: %v", err)
	}
	if _, err := imageStore.SaveGeneration(ctx, businessimage.Generation{
		ID:             generationID,
		UserID:         userID,
		ConversationID: conversationID,
		TurnID:         "turn_" + suffix,
		Prompt:         "postgres core smoke",
		Model:          "gpt-image-test",
		Count:          1,
		Status:         businessjobs.StatusSucceeded,
	}); err != nil {
		t.Fatalf("SaveGeneration() returned error: %v", err)
	}
	if _, err := imageStore.SaveAsset(ctx, businessimage.Asset{
		ID:             fileName,
		UserID:         userID,
		ConversationID: conversationID,
		GenerationID:   generationID,
		FileName:       fileName,
		FilePath:       "/tmp/" + fileName,
		SizeBytes:      123,
		SHA256:         "sha",
	}); err != nil {
		t.Fatalf("SaveAsset() returned error: %v", err)
	}
	detail, ok, err := imageStore.GetConversationWithGenerations(ctx, conversationID, userID, 10)
	if err != nil || !ok {
		t.Fatalf("GetConversationWithGenerations() ok=%v err=%v", ok, err)
	}
	if len(detail.Generations) != 1 || detail.Generations[0].ID != generationID {
		t.Fatalf("conversation detail = %#v", detail)
	}
	assets, total, err := imageStore.AssetsByUserPage(ctx, userID, 10, 0)
	if err != nil {
		t.Fatalf("AssetsByUserPage() returned error: %v", err)
	}
	if total != 1 || len(assets) != 1 || assets[0].FileName != fileName {
		t.Fatalf("assets total=%d items=%#v", total, assets)
	}

	settingsStore := businesssettings.NewStoreWithDB(db, cfg.Database.Driver)
	originalSettings, restoreSettings, err := snapshotPostgresSystemSettings(ctx, db)
	if err != nil {
		t.Fatalf("snapshot settings returned error: %v", err)
	}
	t.Cleanup(func() {
		restorePostgresSystemSettings(context.Background(), db, originalSettings, restoreSettings)
	})
	settings := businesssettings.Defaults()
	settings.Site.Name = "PostgreSQL Core"
	settings.Generation.MaxCount = 4
	settings.Billing.GPTImageCost = 2
	savedSettings, err := settingsStore.Save(ctx, settings)
	if err != nil {
		t.Fatalf("Save settings returned error: %v", err)
	}
	if savedSettings.Site.Name != "PostgreSQL Core" || savedSettings.Billing.GPTImageCost != 2 {
		t.Fatalf("saved settings = %#v", savedSettings)
	}
	loadedSettings, found, err := settingsStore.GetWithFound(ctx)
	if err != nil || !found {
		t.Fatalf("GetWithFound settings found=%v err=%v", found, err)
	}
	if loadedSettings.Site.Name != "PostgreSQL Core" {
		t.Fatalf("loaded settings = %#v", loadedSettings)
	}

	providerStore := businessproviders.NewStoreWithDB(db, cfg.Database.Driver)
	providerDefaults, err := snapshotPostgresProviderDefaults(ctx, db, businessproviders.PlatformGPTImage)
	if err != nil {
		t.Fatalf("snapshot provider defaults returned error: %v", err)
	}
	t.Cleanup(func() {
		restorePostgresProviderDefaults(context.Background(), db, businessproviders.PlatformGPTImage, providerID, providerDefaults)
	})
	provider, err := providerStore.Create(ctx, businessproviders.MutationInput{
		Name:         "PostgreSQL Provider",
		Platform:     businessproviders.PlatformGPTImage,
		BaseURL:      "https://example.test/v1",
		APIKey:       "test-key",
		DefaultModel: "gpt-image-test",
		Enabled:      true,
		IsDefault:    true,
	})
	if err != nil {
		t.Fatalf("Create provider returned error: %v", err)
	}
	providerID = provider.ID
	defaultProvider, ok, err := providerStore.DefaultForPlatform(ctx, businessproviders.PlatformGPTImage)
	if err != nil || !ok {
		t.Fatalf("DefaultForPlatform ok=%v err=%v", ok, err)
	}
	if defaultProvider.ID != provider.ID || !defaultProvider.IsDefault {
		t.Fatalf("default provider = %#v", defaultProvider)
	}

	trackerStore := businesstracker.NewStoreWithDB(db, cfg.Database.Driver)
	if _, err := trackerStore.Save(ctx, businesstracker.Record{
		ID:                 trackerID,
		UserID:             userID,
		ConversationID:     conversationID,
		GenerationID:       generationID,
		TurnID:             "turn_" + suffix,
		Platform:           businessproviders.PlatformGPTImage,
		ProviderID:         provider.ID,
		ProviderName:       provider.Name,
		Model:              "gpt-image-test",
		Status:             businesstracker.StatusFailed,
		Stage:              "upstream",
		ErrorCode:          "provider_error",
		ErrorMessage:       "failed",
		RequestedCount:     1,
		ActualCount:        0,
		QueueWaitMS:        10,
		UpstreamDurationMS: 20,
		PersistDurationMS:  5,
		TotalDurationMS:    35,
		CreditReserved:     2,
		CreditRefunded:     2,
		StorageBytes:       0,
	}); err != nil {
		t.Fatalf("Save tracker returned error: %v", err)
	}
	trackerSummary, err := trackerStore.Summary(ctx, 3600)
	if err != nil {
		t.Fatalf("Tracker Summary returned error: %v", err)
	}
	if trackerSummary.Total < 1 || len(trackerSummary.RecentFailures) == 0 {
		t.Fatalf("tracker summary = %#v", trackerSummary)
	}
}

func cleanupPostgresCoreRows(ctx context.Context, db *sql.DB, userID, jobID, conversationID, generationID, fileName, providerID, trackerID string) {
	_, _ = db.ExecContext(ctx, `DELETE FROM business_image_tracker WHERE id = $1 OR user_id = $2`, trackerID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_api_providers WHERE id = $1`, providerID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_image_assets WHERE file_name = $1 OR user_id = $2`, fileName, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_image_generations WHERE id = $1 OR user_id = $2`, generationID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_image_conversations WHERE id = $1 OR user_id = $2`, conversationID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_image_jobs WHERE id = $1 OR user_id = $2`, jobID, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_credit_ledger WHERE user_id = $1`, userID)
	_, _ = db.ExecContext(ctx, `DELETE FROM business_user_credits WHERE user_id = $1`, userID)
}

type postgresSystemSettingsSnapshot struct {
	ValueJSON []byte
	CreatedAt string
	UpdatedAt string
}

func snapshotPostgresSystemSettings(ctx context.Context, db *sql.DB) (postgresSystemSettingsSnapshot, bool, error) {
	var snapshot postgresSystemSettingsSnapshot
	err := db.QueryRowContext(ctx, `SELECT value_json, created_at, updated_at FROM business_system_settings WHERE key = $1`, "system").Scan(
		&snapshot.ValueJSON,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return postgresSystemSettingsSnapshot{}, false, nil
	}
	if err != nil {
		return postgresSystemSettingsSnapshot{}, false, err
	}
	snapshot.ValueJSON = append([]byte(nil), snapshot.ValueJSON...)
	return snapshot, true, nil
}

func restorePostgresSystemSettings(ctx context.Context, db *sql.DB, snapshot postgresSystemSettingsSnapshot, restore bool) {
	if !restore {
		_, _ = db.ExecContext(ctx, `DELETE FROM business_system_settings WHERE key = $1`, "system")
		return
	}
	_, _ = db.ExecContext(ctx, `
		INSERT INTO business_system_settings(key, value_json, created_at, updated_at)
		VALUES($1, $2, $3, $4)
		ON CONFLICT(key) DO UPDATE SET
			value_json = excluded.value_json,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`, "system", snapshot.ValueJSON, snapshot.CreatedAt, snapshot.UpdatedAt)
}

func snapshotPostgresProviderDefaults(ctx context.Context, db *sql.DB, platform string) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, is_default FROM business_api_providers WHERE platform = $1`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	snapshot := map[string]int{}
	for rows.Next() {
		var id string
		var isDefault int
		if err := rows.Scan(&id, &isDefault); err != nil {
			return nil, err
		}
		snapshot[id] = isDefault
	}
	return snapshot, rows.Err()
}

func restorePostgresProviderDefaults(ctx context.Context, db *sql.DB, platform, createdProviderID string, snapshot map[string]int) {
	_, _ = db.ExecContext(ctx, `DELETE FROM business_api_providers WHERE id = $1`, createdProviderID)
	_, _ = db.ExecContext(ctx, `UPDATE business_api_providers SET is_default = 0 WHERE platform = $1`, platform)
	for id, isDefault := range snapshot {
		_, _ = db.ExecContext(ctx, `UPDATE business_api_providers SET is_default = $1 WHERE id = $2`, isDefault, id)
	}
}
