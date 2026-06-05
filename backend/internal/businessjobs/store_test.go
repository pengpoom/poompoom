package businessjobs

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/config"
)

func TestStoreClaimQueuedDispatchFields(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	job, err := store.Save(ctx, Job{
		ID:             "job_dispatch_fields",
		UserID:         "user_dispatch_fields",
		ConversationID: "conv_dispatch_fields",
		GenerationID:   "gen_dispatch_fields",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
	})
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	claimed, ok, err := store.ClaimQueuedBy(ctx, job.ID, job.UserID, "worker-one")
	if err != nil || !ok {
		t.Fatalf("ClaimQueuedBy() returned ok=%v err=%v", ok, err)
	}
	if claimed.ClaimedBy != "worker-one" || claimed.ClaimedAt == "" || claimed.LeaseUntil == "" || claimed.Attempts != 1 {
		t.Fatalf("claimed job dispatch fields = %#v", claimed)
	}
	if leaseUntil, err := time.Parse(time.RFC3339Nano, claimed.LeaseUntil); err != nil || !leaseUntil.After(now) {
		t.Fatalf("LeaseUntil = %q, parse err=%v, want future timestamp", claimed.LeaseUntil, err)
	}

	if _, ok, err := store.ClaimQueuedBy(ctx, job.ID, job.UserID, "worker-two"); err != nil || ok {
		t.Fatalf("second ClaimQueuedBy() returned ok=%v err=%v, want false nil", ok, err)
	}

	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err := store.db.ExecContext(ctx, `UPDATE business_image_jobs SET lease_until = ? WHERE id = ?`, past, job.ID); err != nil {
		t.Fatalf("expire lease returned error: %v", err)
	}
	reclaimed, ok, err := store.ClaimQueuedBy(ctx, job.ID, job.UserID, "worker-two")
	if err != nil || !ok {
		t.Fatalf("reclaim after lease expiry returned ok=%v err=%v", ok, err)
	}
	if reclaimed.ClaimedBy != "worker-two" || reclaimed.Attempts != 2 {
		t.Fatalf("reclaimed job = %#v, want worker-two attempt 2", reclaimed)
	}
}

func TestStoreClaimQueuedByJobIDDoesNotRequireUserID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	job, err := store.Save(ctx, Job{
		ID:             "job_dispatch_id_only",
		UserID:         "user_dispatch_id_only",
		ConversationID: "conv_dispatch_id_only",
		GenerationID:   "gen_dispatch_id_only",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
	})
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	claimed, ok, err := store.ClaimQueuedByJobID(ctx, job.ID, "redis-worker")
	if err != nil || !ok {
		t.Fatalf("ClaimQueuedByJobID() returned ok=%v err=%v", ok, err)
	}
	if claimed.ID != job.ID || claimed.UserID != job.UserID || claimed.ClaimedBy != "redis-worker" {
		t.Fatalf("claimed job = %#v", claimed)
	}

	if _, ok, err := store.ClaimQueuedByJobID(ctx, job.ID, "redis-worker-two"); err != nil || ok {
		t.Fatalf("second ClaimQueuedByJobID() returned ok=%v err=%v, want false nil", ok, err)
	}
}

func TestStoreCompareBatchFieldsAndSummary(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	batchID := "compare_batch_store"
	jobs := []Job{
		{
			ID:                "job_compare_one",
			UserID:            "user_compare",
			ConversationID:    "conv_compare",
			GenerationID:      "gen_compare_one",
			CompareBatchID:    batchID,
			CompareModelIndex: 0,
			CompareModelCount: 3,
			Status:            StatusSucceeded,
			Stage:             "done",
			RequestedCount:    1,
			ActualCount:       1,
			CreditReserved:    5,
		},
		{
			ID:                "job_compare_two",
			UserID:            "user_compare",
			ConversationID:    "conv_compare",
			GenerationID:      "gen_compare_two",
			CompareBatchID:    batchID,
			CompareModelIndex: 1,
			CompareModelCount: 3,
			Status:            StatusFailed,
			Stage:             "upstream",
			RequestedCount:    1,
			CreditReserved:    5,
			CreditRefunded:    5,
		},
		{
			ID:                "job_compare_three",
			UserID:            "user_compare",
			ConversationID:    "conv_compare",
			GenerationID:      "gen_compare_three",
			CompareBatchID:    batchID,
			CompareModelIndex: 2,
			CompareModelCount: 3,
			Status:            StatusRunning,
			Stage:             "running",
			RequestedCount:    1,
			CreditReserved:    5,
		},
	}
	for _, job := range jobs {
		if _, err := store.Save(ctx, job); err != nil {
			t.Fatalf("Save(%s) returned error: %v", job.ID, err)
		}
	}

	items, total, err := store.AdminList(ctx, AdminListFilter{CompareBatchID: batchID, Limit: 10})
	if err != nil {
		t.Fatalf("AdminList() returned error: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("AdminList() total=%d len=%d, want 3", total, len(items))
	}
	for _, item := range items {
		if item.CompareBatchID != batchID || item.CompareModelCount != 3 {
			t.Fatalf("compare fields = %#v", item)
		}
	}

	summaries, err := store.CompareBatchSummaries(ctx, "user_compare", []string{batchID})
	if err != nil {
		t.Fatalf("CompareBatchSummaries() returned error: %v", err)
	}
	summary := summaries[batchID]
	if summary.Total != 3 || summary.Succeeded != 1 || summary.Failed != 1 || summary.Running != 1 || summary.Status != StatusRunning {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.Reserved != 15 || summary.Refunded != 5 {
		t.Fatalf("summary credits = reserved:%d refunded:%d", summary.Reserved, summary.Refunded)
	}
}

func TestStoreClaimQueuedRespectsNextRunAt(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)

	job, err := store.Save(ctx, Job{
		ID:             "job_deferred",
		UserID:         "user_deferred",
		ConversationID: "conv_deferred",
		GenerationID:   "gen_deferred",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
		NextRunAt:      future,
	})
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if queued, err := store.ListQueued(ctx, 10); err != nil {
		t.Fatalf("ListQueued() returned error: %v", err)
	} else if len(queued) != 0 {
		t.Fatalf("ListQueued() returned %d jobs, want 0 before next_run_at", len(queued))
	}
	if _, ok, err := store.ClaimQueuedBy(ctx, job.ID, job.UserID, "worker-future"); err != nil || ok {
		t.Fatalf("ClaimQueuedBy() future job returned ok=%v err=%v, want false nil", ok, err)
	}

	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err := store.db.ExecContext(ctx, `UPDATE business_image_jobs SET next_run_at = ? WHERE id = ?`, past, job.ID); err != nil {
		t.Fatalf("make job runnable returned error: %v", err)
	}
	queued, err := store.ListQueued(ctx, 10)
	if err != nil {
		t.Fatalf("ListQueued() returned error: %v", err)
	}
	if len(queued) != 1 || queued[0].ID != job.ID {
		t.Fatalf("ListQueued() = %#v, want deferred job", queued)
	}
	if claimed, ok, err := store.ClaimQueuedBy(ctx, job.ID, job.UserID, "worker-now"); err != nil || !ok || claimed.ClaimedBy != "worker-now" {
		t.Fatalf("ClaimQueuedBy() runnable job returned claimed=%#v ok=%v err=%v", claimed, ok, err)
	}
}

func TestStoreReconcileRunningUsesLeaseUntil(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	job, err := store.Save(ctx, Job{
		ID:             "job_running_lease",
		UserID:         "user_running_lease",
		ConversationID: "conv_running_lease",
		GenerationID:   "gen_running_lease",
		Status:         StatusRunning,
		Stage:          "running",
		RequestedCount: 1,
		LeaseUntil:     now.Add(-time.Minute).Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	stale, err := store.ListStale(ctx, ReconcileOptions{
		Now:           now,
		RunningBefore: now.Add(-time.Hour),
	}, 10)
	if err != nil {
		t.Fatalf("ListStale() returned error: %v", err)
	}
	if len(stale) != 1 || stale[0].ID != job.ID {
		t.Fatalf("ListStale() = %#v, want running lease job", stale)
	}

	result, err := store.ReconcileStale(ctx, ReconcileOptions{
		Now:           now,
		RunningBefore: now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("ReconcileStale() returned error: %v", err)
	}
	if result.RunningFailed != 1 {
		t.Fatalf("RunningFailed = %d, want 1", result.RunningFailed)
	}
	updated, ok, err := store.Get(ctx, job.ID, job.UserID)
	if err != nil || !ok {
		t.Fatalf("Get() after reconcile returned ok=%v err=%v", ok, err)
	}
	if updated.Status != StatusFailed || updated.LastError == "" || updated.LeaseUntil != "" {
		t.Fatalf("updated stale job = %#v", updated)
	}
}

func TestStoreRenewRunningLeaseKeepsJobActive(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	job, err := store.Save(ctx, Job{
		ID:             "job_running_renew",
		UserID:         "user_running_renew",
		ConversationID: "conv_running_renew",
		GenerationID:   "gen_running_renew",
		Status:         StatusRunning,
		Stage:          "running",
		RequestedCount: 1,
		LeaseUntil:     now.Add(-time.Minute).Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	futureLease := now.Add(time.Minute).Format(time.RFC3339Nano)
	renewed, err := store.RenewRunningLease(ctx, job.ID, job.UserID, futureLease)
	if err != nil || !renewed {
		t.Fatalf("RenewRunningLease() returned renewed=%v err=%v", renewed, err)
	}
	stale, err := store.ListStale(ctx, ReconcileOptions{
		Now:           now,
		RunningBefore: now.Add(-time.Hour),
	}, 10)
	if err != nil {
		t.Fatalf("ListStale() returned error: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("ListStale() returned %#v, want no stale jobs after renew", stale)
	}
	updated, ok, err := store.Get(ctx, job.ID, job.UserID)
	if err != nil || !ok {
		t.Fatalf("Get() after renew returned ok=%v err=%v", ok, err)
	}
	if updated.LeaseUntil != futureLease {
		t.Fatalf("LeaseUntil = %q, want %q", updated.LeaseUntil, futureLease)
	}
}

func TestStoreSaveQueuedWithCapacityLimitsUserActiveJobs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	limits := CapacityLimits{
		MaxUserActiveJobs: 1,
		MaxQueuedJobs:     100,
	}

	first, err := store.SaveQueuedWithCapacity(ctx, Job{
		ID:             "job_user_active_first",
		UserID:         "user_active_limit",
		ConversationID: "conv_user_active_first",
		GenerationID:   "gen_user_active_first",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
	}, limits)
	if err != nil {
		t.Fatalf("first SaveQueuedWithCapacity() returned error: %v", err)
	}
	if _, err := store.SaveQueuedWithCapacity(ctx, Job{
		ID:             "job_user_active_second",
		UserID:         "user_active_limit",
		ConversationID: "conv_user_active_second",
		GenerationID:   "gen_user_active_second",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
	}, limits); !capacityErrorCodeIs(err, CapacityUserActiveLimit) {
		t.Fatalf("second SaveQueuedWithCapacity() error = %v, want %s", err, CapacityUserActiveLimit)
	}

	first.Status = StatusSucceeded
	first.Stage = "done"
	first.ActualCount = 1
	if _, err := store.Save(ctx, first); err != nil {
		t.Fatalf("release first job returned error: %v", err)
	}
	if _, err := store.SaveQueuedWithCapacity(ctx, Job{
		ID:             "job_user_active_after_release",
		UserID:         "user_active_limit",
		ConversationID: "conv_user_active_after_release",
		GenerationID:   "gen_user_active_after_release",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
	}, limits); err != nil {
		t.Fatalf("SaveQueuedWithCapacity() after release returned error: %v", err)
	}
}

func TestStoreSaveQueuedWithCapacityLimitsGlobalQueuedJobs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	limits := CapacityLimits{
		MaxUserActiveJobs: 100,
		MaxQueuedJobs:     1,
	}

	first, err := store.SaveQueuedWithCapacity(ctx, Job{
		ID:             "job_queue_full_first",
		UserID:         "user_queue_full_one",
		ConversationID: "conv_queue_full_first",
		GenerationID:   "gen_queue_full_first",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
	}, limits)
	if err != nil {
		t.Fatalf("first SaveQueuedWithCapacity() returned error: %v", err)
	}
	if _, err := store.SaveQueuedWithCapacity(ctx, Job{
		ID:             "job_queue_full_second",
		UserID:         "user_queue_full_two",
		ConversationID: "conv_queue_full_second",
		GenerationID:   "gen_queue_full_second",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
	}, limits); !capacityErrorCodeIs(err, CapacityQueueFull) {
		t.Fatalf("second SaveQueuedWithCapacity() error = %v, want %s", err, CapacityQueueFull)
	}

	first.Status = StatusRunning
	first.Stage = "running"
	if _, err := store.Save(ctx, first); err != nil {
		t.Fatalf("move first job to running returned error: %v", err)
	}
	if _, err := store.SaveQueuedWithCapacity(ctx, Job{
		ID:             "job_queue_full_after_release",
		UserID:         "user_queue_full_three",
		ConversationID: "conv_queue_full_after_release",
		GenerationID:   "gen_queue_full_after_release",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
	}, limits); err != nil {
		t.Fatalf("SaveQueuedWithCapacity() after queued release returned error: %v", err)
	}
}

func TestStoreSaveRunningWithCapacityLimitsProviderRunningJobs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	limits := CapacityLimits{MaxProviderRunningJobs: 1}

	first, err := store.SaveRunningWithProviderCapacity(ctx, Job{
		ID:             "job_provider_running_first",
		UserID:         "user_provider_running_one",
		ConversationID: "conv_provider_running_first",
		GenerationID:   "gen_provider_running_first",
		Platform:       "gpt-image",
		ProviderID:     "provider-running-limit",
		Status:         StatusRunning,
		Stage:          "running",
		RequestedCount: 1,
	}, limits)
	if err != nil {
		t.Fatalf("first SaveRunningWithProviderCapacity() returned error: %v", err)
	}
	if _, err := store.SaveRunningWithProviderCapacity(ctx, Job{
		ID:             "job_provider_running_second",
		UserID:         "user_provider_running_two",
		ConversationID: "conv_provider_running_second",
		GenerationID:   "gen_provider_running_second",
		Platform:       "gpt-image",
		ProviderID:     "provider-running-limit",
		Status:         StatusRunning,
		Stage:          "running",
		RequestedCount: 1,
	}, limits); !capacityErrorCodeIs(err, CapacityProviderRunningFull) {
		t.Fatalf("second SaveRunningWithProviderCapacity() error = %v, want %s", err, CapacityProviderRunningFull)
	}

	first.Status = StatusSucceeded
	first.Stage = "done"
	first.ActualCount = 1
	if _, err := store.Save(ctx, first); err != nil {
		t.Fatalf("release first running job returned error: %v", err)
	}
	if _, err := store.SaveRunningWithProviderCapacity(ctx, Job{
		ID:             "job_provider_running_after_release",
		UserID:         "user_provider_running_three",
		ConversationID: "conv_provider_running_after_release",
		GenerationID:   "gen_provider_running_after_release",
		Platform:       "gpt-image",
		ProviderID:     "provider-running-limit",
		Status:         StatusRunning,
		Stage:          "running",
		RequestedCount: 1,
	}, limits); err != nil {
		t.Fatalf("SaveRunningWithProviderCapacity() after release returned error: %v", err)
	}
}

func capacityErrorCodeIs(err error, code string) bool {
	capacityErr, ok := err.(*CapacityError)
	return ok && capacityErr.Code == code
}

func TestSaveAndGetJobWithAPIKeyAttribution(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	saved, err := store.SaveQueuedWithCapacity(ctx, Job{
		ID:             "job_apikey_attr",
		UserID:         "user_apikey_attr",
		ConversationID: "conv_apikey_attr",
		GenerationID:   "gen_apikey_attr",
		Status:         StatusQueued,
		Stage:          "queued",
		RequestedCount: 1,
		APIKeyID:       "apikey_test_1",
		APIMetadata:    []byte(`{"scene":"demo"}`),
	}, CapacityLimits{})
	if err != nil {
		t.Fatalf("SaveQueuedWithCapacity() error: %v", err)
	}
	got, ok, err := store.Get(ctx, saved.ID, "user_apikey_attr")
	if err != nil || !ok {
		t.Fatalf("Get() err=%v ok=%v", err, ok)
	}
	if got.APIKeyID != "apikey_test_1" {
		t.Fatalf("APIKeyID = %q, want apikey_test_1", got.APIKeyID)
	}
	if string(got.APIMetadata) != `{"scene":"demo"}` {
		t.Fatalf("APIMetadata = %q, want {\"scene\":\"demo\"}", string(got.APIMetadata))
	}
}

func TestAdminListFilterBySource(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if _, err := store.Save(ctx, Job{
		ID: "job_src_web", UserID: "u_src", GenerationID: "gen_src_web",
		Status: StatusQueued, Stage: "queued", RequestedCount: 1,
	}); err != nil {
		t.Fatalf("Save(web) error: %v", err)
	}
	if _, err := store.Save(ctx, Job{
		ID: "job_src_api", UserID: "u_src", GenerationID: "gen_src_api",
		Status: StatusQueued, Stage: "queued", RequestedCount: 1, APIKeyID: "apikey_src",
	}); err != nil {
		t.Fatalf("Save(api) error: %v", err)
	}

	apiItems, apiTotal, err := store.AdminList(ctx, AdminListFilter{Source: "api"})
	if err != nil {
		t.Fatalf("AdminList(api) error: %v", err)
	}
	if apiTotal != 1 || len(apiItems) != 1 || apiItems[0].ID != "job_src_api" {
		t.Fatalf("AdminList(api) = %d items (total %d), want only job_src_api", len(apiItems), apiTotal)
	}

	webItems, webTotal, err := store.AdminList(ctx, AdminListFilter{Source: "web"})
	if err != nil {
		t.Fatalf("AdminList(web) error: %v", err)
	}
	if webTotal != 1 || len(webItems) != 1 || webItems[0].ID != "job_src_web" {
		t.Fatalf("AdminList(web) = %d items (total %d), want only job_src_web", len(webItems), webTotal)
	}

	_, allTotal, err := store.AdminList(ctx, AdminListFilter{})
	if err != nil {
		t.Fatalf("AdminList(all) error: %v", err)
	}
	if allTotal != 2 {
		t.Fatalf("AdminList(all) total = %d, want 2", allTotal)
	}
}

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
		t.Fatalf("NewStore() returned error: %v", err)
	}
	if _, err := store.db.ExecContext(context.Background(), `TRUNCATE business_image_jobs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset job test table: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}
