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
	ProviderSource         string         `json:"providerSource,omitempty"`
	ProviderGroupID        string         `json:"providerGroupId,omitempty"`
	ProviderGroupName      string         `json:"providerGroupName,omitempty"`
	ProviderGroupMatchMode string         `json:"providerGroupMatchMode,omitempty"`
	ProviderGroupTags      []string       `json:"providerGroupTags,omitempty"`
	ProviderMemberID       string         `json:"providerMemberId,omitempty"`
	ProviderMemberName     string         `json:"providerMemberName,omitempty"`
	DispatchStrategy       string         `json:"dispatchStrategy,omitempty"`
	DispatchTrace          []string       `json:"dispatchTrace,omitempty"`
	RequestDispatchTags    []string       `json:"requestDispatchTags,omitempty"`
	UserDispatchTags       []string       `json:"userDispatchTags,omitempty"`
	DispatchTags           []string       `json:"dispatchTags,omitempty"`
	UserErrorType          string         `json:"userErrorType,omitempty"`
	UserErrorMessage       string         `json:"userErrorMessage,omitempty"`
	Payload                map[string]any `json:"payload,omitempty"`
}

func (s *Server) businessImageJobViewsFromJobs(ctx context.Context, jobs []businessjobs.Job) []businessImageJobView {
	groupNames := s.businessProviderGroupNamesByID(ctx, jobs)
	views := make([]businessImageJobView, 0, len(jobs))
	for _, job := range jobs {
		views = append(views, businessImageJobViewFromJob(job, groupNames))
	}
	return views
}

func businessImageJobViewFromJob(job businessjobs.Job, groupNames map[string]string) businessImageJobView {
	payload := businessImageJobRawPayload(job)
	source := strings.TrimSpace(stringValue(payload["providerSource"]))
	groupID := strings.TrimSpace(stringValue(payload["providerGroupId"]))
	memberID := strings.TrimSpace(firstNonEmpty(stringValue(payload["providerId"]), job.ProviderID))
	memberName := strings.TrimSpace(firstNonEmpty(stringValue(payload["providerName"]), job.ProviderName))
	if source == "" {
		source = inferBusinessImageJobProviderSource(job, groupID)
	}
	return businessImageJobView{
		Job:               job,
		ProviderSource:    source,
		ProviderGroupID:   groupID,
		ProviderGroupName: strings.TrimSpace(groupNames[groupID]),
		ProviderGroupMatchMode: strings.TrimSpace(
			stringValue(payload["providerGroupMatchMode"]),
		),
		ProviderGroupTags:   providerDispatchTagStrings(payload["providerGroupTags"]),
		ProviderMemberID:    memberID,
		ProviderMemberName:  memberName,
		DispatchStrategy:    strings.TrimSpace(stringValue(payload["dispatchStrategy"])),
		DispatchTrace:       providerDispatchTagStrings(payload["dispatchTrace"]),
		RequestDispatchTags: providerDispatchTagStrings(payload["requestDispatchTags"]),
		UserDispatchTags:    providerDispatchTagStrings(payload["userDispatchTags"]),
		DispatchTags:        providerDispatchTagStrings(payload["dispatchTags"]),
		UserErrorType:       businessjobs.JobUserErrorType(job),
		UserErrorMessage:    businessjobs.JobUserErrorMessage(job),
		Payload:             sanitizeBusinessImageJobPayload(payload),
	}
}

func businessImageJobRawPayload(job businessjobs.Job) map[string]any {
	if len(job.PayloadJSON) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
		return nil
	}
	return payload
}

func businessImageJobPublicPayload(job businessjobs.Job) map[string]any {
	return sanitizeBusinessImageJobPayload(businessImageJobRawPayload(job))
}

func sanitizeBusinessImageJobPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	next := make(map[string]any, len(payload))
	for key, value := range payload {
		if key == "providerSource" || key == "providerId" || key == "providerName" || key == "providerGroupId" ||
			key == "providerGroupMatchMode" || key == "providerGroupTags" || key == "dispatchStrategy" ||
			key == "dispatchTrace" || key == "requestDispatchTags" || key == "userDispatchTags" || key == "dispatchTags" {
			continue
		}
		if key == "sourceImages" {
			next[key] = sanitizeBusinessImageJobSourceImages(value)
			continue
		}
		next[key] = value
	}
	return next
}

func inferBusinessImageJobProviderSource(job businessjobs.Job, groupID string) string {
	if strings.TrimSpace(groupID) != "" {
		return imageProviderSourcePool
	}
	if strings.TrimSpace(job.ProviderID) != "" {
		return imageProviderSourceLegacy
	}
	if strings.TrimSpace(job.ProviderName) == "API 接入配置" {
		return imageProviderSourceAPIAccess
	}
	if strings.TrimSpace(job.ProviderName) == "环境变量 API" {
		return imageProviderSourceEnv
	}
	return ""
}

func (s *Server) businessProviderGroupNamesByID(ctx context.Context, jobs []businessjobs.Job) map[string]string {
	ids := make(map[string]struct{})
	for _, job := range jobs {
		payload := businessImageJobRawPayload(job)
		if groupID := strings.TrimSpace(stringValue(payload["providerGroupId"])); groupID != "" {
			ids[groupID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		return nil
	}
	defer store.Close()
	names := make(map[string]string, len(ids))
	for id := range ids {
		group, ok, err := store.GetGroup(ctx, id)
		if err == nil && ok {
			names[id] = group.Name
		}
	}
	return names
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
	views := s.businessImageJobViewsFromJobs(r.Context(), items)
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
		UserID:    r.URL.Query().Get("userId"),
		Status:    r.URL.Query().Get("status"),
		Platform:  r.URL.Query().Get("platform"),
		ErrorType: r.URL.Query().Get("errorType"),
		From:      from,
		To:        to,
		Limit:     pageSize,
		Offset:    offset,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	views := s.businessImageJobViewsFromJobs(r.Context(), items)
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
	writeJSON(w, http.StatusOK, map[string]any{"item": businessImageJobViewFromJob(item, s.businessProviderGroupNamesByID(r.Context(), []businessjobs.Job{item}))})
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
		"item":            businessImageJobViewFromJob(item, s.businessProviderGroupNamesByID(r.Context(), []businessjobs.Job{item})),
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
	totalRefunded := int64(0)
	totals, err := creditStore.GenerationTotals(ctx, job.UserID, job.GenerationID)
	if err != nil {
		return job
	}
	totalRefunded += totals.Refunded
	if totals.Reserved > totals.Refunded {
		refundable := totals.Reserved - totals.Refunded
		if _, _, err := creditStore.Refund(ctx, job.UserID, refundable, job.GenerationID); err == nil {
			totalRefunded += refundable
		}
	}
	paymentStore, err := s.newBusinessPaymentStore()
	if err == nil {
		defer paymentStore.Close()
		if subscriptionTotals, totalsErr := paymentStore.SubscriptionGenerationTotals(ctx, job.UserID, job.GenerationID); totalsErr == nil {
			totalRefunded += subscriptionTotals.Refunded
			if subscriptionTotals.Reserved > subscriptionTotals.Refunded {
				refundable := subscriptionTotals.Reserved - subscriptionTotals.Refunded
				if _, refundErr := paymentStore.RefundSubscriptionCredits(ctx, job.UserID, refundable, job.GenerationID); refundErr == nil {
					totalRefunded += refundable
				}
			}
		}
	}
	job.CreditRefunded = totalRefunded
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
