package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/businessjobs"
)

type businessImageJobView struct {
	businessjobs.Job
	Payload map[string]any `json:"payload,omitempty"`
}

func businessImageJobViewFromJob(job businessjobs.Job) businessImageJobView {
	return businessImageJobView{
		Job:     job,
		Payload: businessImageJobPublicPayload(job),
	}
}

func businessImageJobPublicPayload(job businessjobs.Job) map[string]any {
	if len(job.PayloadJSON) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
		return nil
	}
	if payload == nil {
		return nil
	}
	return sanitizeBusinessImageJobPayload(payload)
}

func sanitizeBusinessImageJobPayload(payload map[string]any) map[string]any {
	next := make(map[string]any, len(payload))
	for key, value := range payload {
		if key == "sourceImages" {
			next[key] = sanitizeBusinessImageJobSourceImages(value)
			continue
		}
		next[key] = value
	}
	return next
}

func sanitizeBusinessImageJobSourceImages(value any) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	next := make([]map[string]any, 0, len(items))
	for _, item := range items {
		source, ok := item.(map[string]any)
		if !ok {
			continue
		}
		clean := make(map[string]any, len(source))
		for key, value := range source {
			if key == "dataUrl" {
				continue
			}
			clean[key] = value
		}
		next = append(next, clean)
	}
	return next
}

func (s *Server) handleListBusinessImageJobs(w http.ResponseWriter, r *http.Request) {
	s.reconcileStaleBusinessImageJobs(r.Context())
	store, err := s.newBusinessJobStore()
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
	views := make([]businessImageJobView, 0, len(items))
	for _, item := range items {
		views = append(views, businessImageJobViewFromJob(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": views})
}

func (s *Server) handleAdminListBusinessImageJobs(w http.ResponseWriter, r *http.Request) {
	s.reconcileStaleBusinessImageJobs(r.Context())
	store, err := s.newBusinessJobStore()
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
	views := make([]businessImageJobView, 0, len(items))
	for _, item := range items {
		views = append(views, businessImageJobViewFromJob(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": views,
		"page": paginationMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	})
}

func (s *Server) handleGetBusinessImageJob(w http.ResponseWriter, r *http.Request) {
	s.reconcileStaleBusinessImageJobs(r.Context())
	store, err := s.newBusinessJobStore()
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
	writeJSON(w, http.StatusOK, map[string]any{"item": businessImageJobViewFromJob(item)})
}

func (s *Server) handleCancelBusinessImageJob(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessJobStore()
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
		"item":            businessImageJobViewFromJob(item),
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
	creditStore, err := s.newBusinessCreditStore()
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
	store, err := s.newBusinessJobStore()
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
	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		return
	}
	defer imageStore.Close()
	_, _ = imageStore.MarkGenerationFinished(ctx, userID, generationID, businessjobs.StatusCancelled, "任务已取消")
}
