package api

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
	"imagestudio/internal/businessjobs"
)

const (
	defaultStaleQueuedGrace          = 2 * time.Minute
	defaultStaleRunningGrace         = 30 * time.Second
	defaultStaleCancelRequestedGrace = 5 * time.Second
)

func (s *Server) reconcileStaleBusinessImageJobs(ctx context.Context) businessjobs.ReconcileResult {
	s.businessJobReconcileMu.Lock()
	defer s.businessJobReconcileMu.Unlock()

	options := s.staleBusinessImageJobReconcileOptions()
	result := businessjobs.ReconcileResult{}
	store, err := businessjobs.NewStore(s.cfg)
	if err != nil {
		return result
	}
	defer store.Close()
	staleJobs, err := store.ListStale(ctx, options, 500)
	if err != nil {
		return result
	}
	s.reconcileStaleBusinessImageGenerations(ctx, staleJobs)
	result, err = store.ReconcileStale(ctx, options)
	if err != nil {
		return result
	}
	return result
}

func (s *Server) reconcileStaleBusinessImageJobsAsync() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result := s.reconcileStaleBusinessImageJobs(ctx)
		if result.QueuedFailed == 0 && result.RunningFailed == 0 && result.CancelRequestedCancelled == 0 {
			return
		}
		slog.Info(
			"reconciled stale business image jobs",
			slog.Int64("queued_failed", result.QueuedFailed),
			slog.Int64("running_failed", result.RunningFailed),
			slog.Int64("cancel_requested_cancelled", result.CancelRequestedCancelled),
		)
	}()
}

func (s *Server) staleBusinessImageJobReconcileOptions() businessjobs.ReconcileOptions {
	now := time.Now().UTC()
	requestTimeout := s.imageProviderRequestTimeout()
	return businessjobs.ReconcileOptions{
		Now:                   now,
		QueuedBefore:          now.Add(-(requestTimeout + defaultStaleQueuedGrace)),
		RunningBefore:         now.Add(-(requestTimeout + defaultStaleRunningGrace)),
		CancelRequestedBefore: now.Add(-defaultStaleCancelRequestedGrace),
	}
}

func (s *Server) imageProviderRequestTimeout() time.Duration {
	timeoutSeconds := normalizeEnvInt("IMAGE_REQUEST_TIMEOUT_SECONDS", 180)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 180
	}
	if value := os.Getenv("IMAGE_JOB_STALE_TIMEOUT_SECONDS"); value != "" {
		if parsed := normalizeEnvInt("IMAGE_JOB_STALE_TIMEOUT_SECONDS", 0); parsed > 0 {
			timeoutSeconds = parsed
		}
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func (s *Server) reconcileStaleBusinessImageGenerations(ctx context.Context, jobs []businessjobs.Job) {
	if len(jobs) == 0 {
		return
	}
	imageStore, err := businessimage.NewStore(s.cfg)
	if err != nil {
		return
	}
	defer imageStore.Close()
	creditStore, _ := businesscredits.NewStore(s.cfg)
	if creditStore != nil {
		defer creditStore.Close()
	}

	for _, job := range jobs {
		status, message := staleJobFinalStatus(job)
		if status == "" {
			continue
		}
		if creditStore != nil && shouldRefundStaleBusinessImageJob(job) {
			totals, err := creditStore.GenerationTotals(ctx, job.UserID, job.GenerationID)
			if err == nil && totals.Reserved > totals.Refunded {
				_, _, _ = creditStore.Refund(ctx, job.UserID, totals.Reserved-totals.Refunded, job.GenerationID)
			}
		}
		_, _ = imageStore.MarkGenerationFinished(ctx, job.UserID, job.GenerationID, status, message)
	}
}

func shouldRefundStaleBusinessImageJob(job businessjobs.Job) bool {
	if strings.TrimSpace(job.Status) == businessjobs.StatusCancelRequested {
		return businessImageJobBeforeUpstream(job)
	}
	return true
}

func staleJobFinalStatus(job businessjobs.Job) (string, string) {
	switch strings.TrimSpace(job.Status) {
	case businessjobs.StatusCancelRequested:
		return businessjobs.StatusCancelled, "任务取消超时，已自动标记取消"
	case businessjobs.StatusQueued, businessjobs.StatusRunning:
		return businessjobs.StatusFailed, "任务超时，已自动标记失败"
	default:
		return "", ""
	}
}
