package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"imagestudio/internal/businessjobs"
)

const businessImageJobWorkerBatchSize = 100
const businessImageJobMaintenanceInterval = 15 * time.Second
const businessImageJobRunningLeaseDuration = 30 * time.Second
const businessImageJobHeartbeatInterval = 10 * time.Second

func (s *Server) startBusinessImageJobMaintenanceAsync() {
	s.startBusinessImageJobWorker()
	s.wakeBusinessImageJobWorker()
	go func() {
		s.runBusinessImageJobReconcileOnce()

		ticker := time.NewTicker(businessImageJobMaintenanceInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.wakeBusinessImageJobWorker()
			s.runBusinessImageJobReconcileOnce()
		}
	}()
}

func (s *Server) startBusinessImageJobWorker() {
	s.businessJobWorkerOnce.Do(func() {
		if s.businessJobDispatcher == nil {
			dispatcher, err := newBusinessJobDispatcher(s.cfg)
			if err != nil {
				slog.Error("init business job dispatcher failed", slog.Any("error", err))
				dispatcher = newLocalBusinessJobDispatcher()
			}
			s.businessJobDispatcher = dispatcher
		}
		go s.businessImageJobWorkerLoop()
	})
}

func (s *Server) wakeBusinessImageJobWorker() {
	s.notifyBusinessImageJob(businessjobs.Job{})
}

func (s *Server) notifyBusinessImageJob(job businessjobs.Job) {
	s.startBusinessImageJobWorker()
	if s.businessJobDispatcher == nil {
		return
	}
	_ = s.businessJobDispatcher.Notify(context.Background(), businessJobNotification{
		JobID:  cleanJobNotificationID(job.ID),
		UserID: strings.TrimSpace(job.UserID),
	})
}

func (s *Server) businessImageJobWorkerLoop() {
	if s.businessJobDispatcher == nil {
		return
	}
	for notification := range s.businessJobDispatcher.Notifications() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		recovered := 0
		if notification.JobID != "" {
			recovered = s.runQueuedBusinessImageJob(ctx, notification.JobID)
		} else {
			recovered = s.drainQueuedBusinessImageJobs(ctx)
		}
		cancel()
		if recovered > 0 {
			slog.Info("started queued business image jobs", slog.Int("claimed", recovered))
		}
	}
}

func (s *Server) drainQueuedBusinessImageJobs(ctx context.Context) int {
	total := 0
	for {
		if ctx.Err() != nil {
			return total
		}
		recovered := s.recoverQueuedBusinessImageJobs(ctx)
		total += recovered
		if recovered < businessImageJobWorkerBatchSize {
			return total
		}
	}
}

func (s *Server) runBusinessImageJobReconcileOnce() {
	reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), 10*time.Second)
	result := s.reconcileStaleBusinessImageJobs(reconcileCtx)
	reconcileCancel()
	if result.QueuedFailed == 0 && result.RunningFailed == 0 && result.CancelRequestedCancelled == 0 {
		return
	}
	slog.Info(
		"reconciled stale business image jobs",
		slog.Int64("queued_failed", result.QueuedFailed),
		slog.Int64("running_failed", result.RunningFailed),
		slog.Int64("cancel_requested_cancelled", result.CancelRequestedCancelled),
	)
}

func (s *Server) recoverQueuedBusinessImageJobs(ctx context.Context) int {
	store, err := s.newBusinessJobStore()
	if err != nil {
		return 0
	}
	defer store.Close()
	jobs, err := store.ListQueued(ctx, businessImageJobWorkerBatchSize)
	if err != nil {
		return 0
	}
	recovered := 0
	for _, job := range jobs {
		workerID := "local"
		if s.businessJobDispatcher != nil {
			workerID = s.businessJobDispatcher.WorkerID()
		}
		claimed, ok, err := store.ClaimQueuedBy(ctx, job.ID, job.UserID, workerID)
		if err != nil || !ok {
			continue
		}
		payload := payloadForRecoveredBusinessImageJob(claimed)
		startedAt := parseJobCreatedAt(claimed.CreatedAt)
		go s.runProviderImageGenerateJob(claimed.UserID, payload, startedAt)
		recovered++
	}
	return recovered
}

func (s *Server) runQueuedBusinessImageJob(ctx context.Context, jobID string) int {
	store, err := s.newBusinessJobStore()
	if err != nil {
		return 0
	}
	defer store.Close()
	workerID := "local"
	if s.businessJobDispatcher != nil {
		workerID = s.businessJobDispatcher.WorkerID()
	}
	claimed, ok, err := store.ClaimQueuedByJobID(ctx, jobID, workerID)
	if err != nil || !ok {
		return 0
	}
	payload := payloadForRecoveredBusinessImageJob(claimed)
	startedAt := parseJobCreatedAt(claimed.CreatedAt)
	go s.runProviderImageGenerateJob(claimed.UserID, payload, startedAt)
	return 1
}

func payloadForRecoveredBusinessImageJob(job businessjobs.Job) map[string]any {
	payload := map[string]any{}
	if len(job.PayloadJSON) > 0 {
		_ = json.Unmarshal(job.PayloadJSON, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	setIfMissing := func(key string, value any) {
		if strings.TrimSpace(stringValue(payload[key])) != "" {
			return
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				return
			}
		case int:
			if typed <= 0 {
				return
			}
		}
		payload[key] = value
	}
	setIfMissing("jobId", job.ID)
	setIfMissing("conversationId", job.ConversationID)
	setIfMissing("turnId", job.TurnID)
	setIfMissing("platform", job.Platform)
	setIfMissing("prompt", job.Prompt)
	setIfMissing("model", job.Model)
	setIfMissing("size", job.Size)
	setIfMissing("quality", job.Quality)
	setIfMissing("n", job.RequestedCount)
	setIfMissing("providerId", job.ProviderID)
	setIfMissing("providerName", job.ProviderName)
	if strings.TrimSpace(stringValue(payload["response_format"])) == "" {
		payload["response_format"] = "url"
	}
	return payload
}

func cleanJobNotificationID(value string) string {
	return strings.TrimSpace(value)
}

func parseJobCreatedAt(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
		return parsed
	}
	return time.Now().UTC()
}

func (s *Server) startBusinessImageJobHeartbeat(ctx context.Context, jobID string, userID string) context.CancelFunc {
	jobID = cleanJobNotificationID(jobID)
	userID = strings.TrimSpace(userID)
	if jobID == "" || userID == "" {
		return func() {}
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(businessImageJobHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				renewCtx, renewCancel := context.WithTimeout(context.Background(), 5*time.Second)
				s.renewBusinessImageJobLease(renewCtx, jobID, userID)
				renewCancel()
			}
		}
	}()
	return cancel
}

func (s *Server) renewBusinessImageJobLease(ctx context.Context, jobID string, userID string) bool {
	store, err := s.newBusinessJobStore()
	if err != nil {
		return false
	}
	defer store.Close()
	leaseUntil := time.Now().UTC().Add(businessImageJobRunningLeaseDuration).Format(time.RFC3339Nano)
	renewed, err := store.RenewRunningLease(ctx, jobID, userID, leaseUntil)
	return err == nil && renewed
}
