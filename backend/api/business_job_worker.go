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
		if s.businessJobWorkerWake == nil {
			s.businessJobWorkerWake = make(chan struct{}, 1)
		}
		go s.businessImageJobWorkerLoop()
	})
}

func (s *Server) wakeBusinessImageJobWorker() {
	s.startBusinessImageJobWorker()
	select {
	case s.businessJobWorkerWake <- struct{}{}:
	default:
	}
}

func (s *Server) businessImageJobWorkerLoop() {
	for range s.businessJobWorkerWake {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		recovered := s.drainQueuedBusinessImageJobs(ctx)
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
	store, err := businessjobs.NewStore(s.cfg)
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
		claimed, ok, err := store.ClaimQueued(ctx, job.ID, job.UserID)
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
	if strings.TrimSpace(stringValue(payload["response_format"])) == "" {
		payload["response_format"] = "url"
	}
	return payload
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
