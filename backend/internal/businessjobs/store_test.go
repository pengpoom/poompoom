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
