package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
	"imagestudio/internal/businessjobs"
)

func (s *Server) handleListBusinessImageJobs(w http.ResponseWriter, r *http.Request) {
	s.reconcileStaleBusinessImageJobs(r.Context())
	store, err := businessjobs.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "job store failed"})
		return
	}
	defer store.Close()

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := store.List(
		r.Context(),
		businessUserIDForRequest(r),
		r.URL.Query().Get("conversationId"),
		limit,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminListBusinessImageJobs(w http.ResponseWriter, r *http.Request) {
	s.reconcileStaleBusinessImageJobs(r.Context())
	store, err := businessjobs.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "job store failed"})
		return
	}
	defer store.Close()

	page, pageSize, offset := paginationFromQuery(r, "", 20, 100)
	from, to := usageTimeRangeFromQuery(r)
	items, total, err := store.AdminList(r.Context(), businessjobs.AdminListFilter{
		UserID:   r.URL.Query().Get("userId"),
		Status:   r.URL.Query().Get("status"),
		Platform: r.URL.Query().Get("platform"),
		From:     from,
		To:       to,
		Limit:    pageSize,
		Offset:   offset,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"page": paginationMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	})
}

func (s *Server) handleGetBusinessImageJob(w http.ResponseWriter, r *http.Request) {
	s.reconcileStaleBusinessImageJobs(r.Context())
	store, err := businessjobs.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "job store failed"})
		return
	}
	defer store.Close()

	item, ok, err := store.Get(r.Context(), r.PathValue("id"), businessUserIDForRequest(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeAPIError(w, http.StatusNotFound, "business_job_not_found", "job not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleCancelBusinessImageJob(w http.ResponseWriter, r *http.Request) {
	store, err := businessjobs.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "job store failed"})
		return
	}
	defer store.Close()

	userID := businessUserIDForRequest(r)
	beforeCancel, beforeCancelOK, _ := store.Get(r.Context(), r.PathValue("id"), userID)
	item, ok, err := store.RequestCancel(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeAPIError(w, http.StatusNotFound, "business_job_not_found", "job not found")
		return
	}
	if beforeCancelOK &&
		businessImageJobBeforeUpstream(beforeCancel) &&
		(item.Status == businessjobs.StatusCancelled || item.Status == businessjobs.StatusCancelRequested) {
		item = s.refundCancelledPreUpstreamBusinessImageJob(context.Background(), item)
	}
	activeCancelled := false
	if item.Status == businessjobs.StatusCancelRequested || item.Status == businessjobs.StatusCancelled {
		activeCancelled = s.cancelActiveBusinessImageJob(item.ID)
	}
	if activeCancelled && item.Status == businessjobs.StatusCancelRequested {
		item.Status = businessjobs.StatusCancelled
		item.Stage = "cancelled"
		item.ErrorCode = "cancelled"
		item.ErrorMessage = "任务已取消"
		item.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if saved, saveErr := store.Save(context.Background(), item); saveErr == nil {
			item = saved
		}
	}
	s.markBusinessImageGenerationCancelled(context.Background(), userID, item.GenerationID)
	writeJSON(w, http.StatusOK, map[string]any{
		"item":            item,
		"activeCancelled": activeCancelled,
	})
}

func businessImageJobBeforeUpstream(job businessjobs.Job) bool {
	if businessjobs.HasUpstreamSent(job) {
		return false
	}
	stage := strings.TrimSpace(job.Stage)
	return stage == "" || stage == "queued" || stage == "claimed" || stage == "running" || stage == "credit" || stage == "dispatching"
}

func (s *Server) refundCancelledPreUpstreamBusinessImageJob(ctx context.Context, job businessjobs.Job) businessjobs.Job {
	if job.CreditReserved <= 0 {
		return job
	}
	creditStore, err := businesscredits.NewStore(s.cfg)
	if err != nil {
		return job
	}
	defer creditStore.Close()
	totals, err := creditStore.GenerationTotals(ctx, job.UserID, job.GenerationID)
	if err != nil {
		return job
	}
	if totals.Reserved <= totals.Refunded {
		job.CreditRefunded = totals.Refunded
		return s.saveBusinessImageJobSnapshot(ctx, job)
	}
	refundable := totals.Reserved - totals.Refunded
	if _, _, err := creditStore.Refund(ctx, job.UserID, refundable, job.GenerationID); err != nil {
		return job
	}
	job.CreditRefunded = totals.Refunded + refundable
	return s.saveBusinessImageJobSnapshot(ctx, job)
}

func (s *Server) saveBusinessImageJobSnapshot(ctx context.Context, job businessjobs.Job) businessjobs.Job {
	store, err := businessjobs.NewStore(s.cfg)
	if err != nil {
		return job
	}
	defer store.Close()
	saved, err := store.Save(ctx, job)
	if err != nil {
		return job
	}
	return saved
}

func (s *Server) markBusinessImageGenerationCancelled(ctx context.Context, userID string, generationID string) {
	imageStore, err := businessimage.NewStore(s.cfg)
	if err != nil {
		return
	}
	defer imageStore.Close()
	_, _ = imageStore.MarkGenerationFinished(ctx, userID, generationID, businessjobs.StatusCancelled, "任务已取消")
}
