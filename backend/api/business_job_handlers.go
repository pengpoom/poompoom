package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/businessjobs"
)

type businessImageJobView struct {
	businessjobs.Job
	ModelID                string                            `json:"modelId,omitempty"`
	ModelLabel             string                            `json:"modelLabel,omitempty"`
	Vendor                 string                            `json:"vendor,omitempty"`
	VendorLabel            string                            `json:"vendorLabel,omitempty"`
	UpstreamModel          string                            `json:"upstreamModel,omitempty"`
	UpstreamStatusCode     int                               `json:"upstreamStatusCode,omitempty"`
	ProviderSource         string                            `json:"providerSource,omitempty"`
	ProviderGroupID        string                            `json:"providerGroupId,omitempty"`
	ProviderGroupName      string                            `json:"providerGroupName,omitempty"`
	ProviderGroupMatchMode string                            `json:"providerGroupMatchMode,omitempty"`
	ProviderGroupTags      []string                          `json:"providerGroupTags,omitempty"`
	ProviderMemberID       string                            `json:"providerMemberId,omitempty"`
	ProviderMemberName     string                            `json:"providerMemberName,omitempty"`
	HasAttachment          bool                              `json:"hasAttachment,omitempty"`
	DispatchStrategy       string                            `json:"dispatchStrategy,omitempty"`
	DispatchTrace          []string                          `json:"dispatchTrace,omitempty"`
	RequestDispatchTags    []string                          `json:"requestDispatchTags,omitempty"`
	UserDispatchTags       []string                          `json:"userDispatchTags,omitempty"`
	DispatchTags           []string                          `json:"dispatchTags,omitempty"`
	UserErrorType          string                            `json:"userErrorType,omitempty"`
	UserErrorMessage       string                            `json:"userErrorMessage,omitempty"`
	FailureReasonCode      string                            `json:"failureReasonCode,omitempty"`
	FailureReasonMessage   string                            `json:"failureReasonMessage,omitempty"`
	CompareBatchStatus     *businessjobs.CompareBatchSummary `json:"compareBatchStatus,omitempty"`
	Payload                map[string]any                    `json:"payload,omitempty"`
}

type businessImageCompareBatchView struct {
	Summary businessjobs.CompareBatchSummary `json:"summary"`
	Items   []businessImageJobView           `json:"items"`
}

func (s *Server) businessImageJobViewsFromJobs(ctx context.Context, jobs []businessjobs.Job, userID string) []businessImageJobView {
	groupNames := s.businessProviderGroupNamesByID(ctx, jobs)
	batchSummaries := s.businessImageCompareBatchSummaries(ctx, jobs, userID)
	views := make([]businessImageJobView, 0, len(jobs))
	for _, job := range jobs {
		view := businessImageJobViewFromJob(job, groupNames)
		if summary, ok := batchSummaries[view.CompareBatchID]; ok && view.CompareBatchID != "" {
			copied := summary
			view.CompareBatchStatus = &copied
		}
		views = append(views, view)
	}
	return views
}

func businessImageJobViewFromJob(job businessjobs.Job, groupNames map[string]string) businessImageJobView {
	payload := businessImageJobRawPayload(job)
	if job.CompareBatchID == "" {
		job.CompareBatchID = strings.TrimSpace(firstNonEmpty(stringValue(payload["compareBatchId"]), stringValue(payload["compareGroupId"])))
	}
	if job.CompareModelCount <= 0 {
		job.CompareModelCount = businessImageJobPayloadInt(payload, "compareModelCount")
	}
	if job.CompareModelIndex <= 0 {
		job.CompareModelIndex = businessImageJobPayloadInt(payload, "compareModelIndex")
	}
	source := strings.TrimSpace(stringValue(payload["providerSource"]))
	groupID := strings.TrimSpace(stringValue(payload["providerGroupId"]))
	memberID := strings.TrimSpace(firstNonEmpty(stringValue(payload["providerId"]), job.ProviderID))
	memberName := strings.TrimSpace(firstNonEmpty(stringValue(payload["providerName"]), job.ProviderName))
	if source == "" {
		source = inferBusinessImageJobProviderSource(job, groupID)
	}
	upstreamStatusCode := businessImageJobPayloadInt(payload, "upstreamStatusCode")
	failureReasonCode, failureReasonMessage := businessImageJobFailureReason(job, upstreamStatusCode)
	return businessImageJobView{
		Job:                job,
		ModelID:            strings.TrimSpace(stringValue(payload["modelId"])),
		ModelLabel:         strings.TrimSpace(stringValue(payload["modelLabel"])),
		Vendor:             strings.TrimSpace(stringValue(payload["vendor"])),
		VendorLabel:        strings.TrimSpace(stringValue(payload["vendorLabel"])),
		UpstreamModel:      strings.TrimSpace(firstNonEmpty(job.Model, stringValue(payload["model"]))),
		UpstreamStatusCode: upstreamStatusCode,
		ProviderSource:     source,
		ProviderGroupID:    groupID,
		ProviderGroupName:  strings.TrimSpace(groupNames[groupID]),
		ProviderGroupMatchMode: strings.TrimSpace(
			stringValue(payload["providerGroupMatchMode"]),
		),
		ProviderGroupTags:    providerDispatchTagStrings(payload["providerGroupTags"]),
		ProviderMemberID:     memberID,
		ProviderMemberName:   memberName,
		HasAttachment:        businessImageJobHasAttachment(payload),
		DispatchStrategy:     strings.TrimSpace(stringValue(payload["dispatchStrategy"])),
		DispatchTrace:        providerDispatchTagStrings(payload["dispatchTrace"]),
		RequestDispatchTags:  providerDispatchTagStrings(payload["requestDispatchTags"]),
		UserDispatchTags:     providerDispatchTagStrings(payload["userDispatchTags"]),
		DispatchTags:         providerDispatchTagStrings(payload["dispatchTags"]),
		UserErrorType:        businessjobs.JobUserErrorType(job),
		UserErrorMessage:     businessjobs.JobUserErrorMessage(job),
		FailureReasonCode:    failureReasonCode,
		FailureReasonMessage: failureReasonMessage,
		Payload:              sanitizeBusinessImageJobPayload(payload),
	}
}

func (s *Server) businessImageCompareBatchSummaries(ctx context.Context, jobs []businessjobs.Job, userID string) map[string]businessjobs.CompareBatchSummary {
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		if id := strings.TrimSpace(job.CompareBatchID); id != "" {
			ids = append(ids, id)
			continue
		}
		payload := businessImageJobRawPayload(job)
		if id := strings.TrimSpace(firstNonEmpty(stringValue(payload["compareBatchId"]), stringValue(payload["compareGroupId"]))); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	store, err := s.newBusinessJobStore()
	if err != nil {
		return nil
	}
	defer store.Close()
	summaries, err := store.CompareBatchSummaries(ctx, userID, ids)
	if err != nil {
		return nil
	}
	return summaries
}

func sortBusinessImageCompareBatchJobs(jobs []businessjobs.Job) {
	sort.SliceStable(jobs, func(i, j int) bool {
		left := jobs[i]
		right := jobs[j]
		if left.CompareModelIndex != right.CompareModelIndex {
			return left.CompareModelIndex < right.CompareModelIndex
		}
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt < right.CreatedAt
		}
		return left.ID < right.ID
	})
}

func businessImageJobPayloadInt(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	switch value := payload[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		return parsed
	default:
		return 0
	}
}

func businessImageJobFailureReason(job businessjobs.Job, upstreamStatusCode int) (string, string) {
	status := strings.TrimSpace(job.Status)
	if status != businessjobs.StatusFailed && status != businessjobs.StatusCancelled && status != businessjobs.StatusCancelRequested {
		return "", ""
	}
	errorCode := strings.TrimSpace(job.ErrorCode)
	errorText := strings.ToLower(strings.Join([]string{job.ErrorCode, job.ErrorMessage, job.LastError}, " "))
	switch {
	case errorCode == "insufficient_credits" || strings.Contains(errorText, "余额不足"):
		return "insufficient_credits", "点数不足：余额或订阅额度不足。"
	case errorCode == "provider_empty_response":
		return "provider_empty_response", "上游成功但没有返回可用图片。"
	case strings.Contains(errorText, "cooling down") || strings.Contains(errorText, "冷却"):
		return "provider_member_cooling", "号池成员冷却中：请恢复成员或等待冷却结束。"
	case strings.Contains(errorText, "没有可用号池") || strings.Contains(errorText, "no available pool") || strings.Contains(errorText, "pool member not available"):
		return "no_available_pool", "没有可用号池成员：请启用 active 成员或配置 API 兜底。"
	case errorCode == "provider_pool_failed_no_fallback":
		return "provider_pool_failed_no_fallback", "号池成员请求失败，且没有可用 API 接入兜底。"
	case errorCode == "provider_not_configured":
		return "provider_not_configured", "上游接入未配置或不可用。"
	case errorCode == "provider_error" && upstreamStatusCode > 0:
		return businessImageJobUpstreamStatusReason(upstreamStatusCode)
	case errorCode == "provider_request_failed":
		return "provider_request_failed", "上游请求失败：网络、DNS、TLS 或连接超时。"
	case errorCode == "provider_response_failed":
		return "provider_response_failed", "上游响应读取或解析失败。"
	case errorCode == "cancelled":
		return "cancelled", "任务已取消。"
	case errorCode != "":
		return errorCode, firstNonEmpty(strings.TrimSpace(job.ErrorMessage), "生成失败。")
	default:
		return businessjobs.JobUserErrorType(job), businessjobs.JobUserErrorMessage(job)
	}
}

func businessImageJobUpstreamStatusReason(statusCode int) (string, string) {
	switch statusCode {
	case http.StatusUnauthorized:
		return "upstream_unauthorized", "上游 401：API Key 无效或未授权。"
	case http.StatusForbidden:
		return "upstream_forbidden", "上游 403：账号无权限访问该模型或接口。"
	case http.StatusNotFound:
		return "upstream_not_found", "上游 404：模型或接口不存在。"
	case http.StatusTooManyRequests:
		return "upstream_rate_limited", "上游 429：触发限流或额度不足。"
	}
	if statusCode >= http.StatusInternalServerError {
		return "upstream_server_error", "上游 5xx：服务端错误或网关异常。"
	}
	if statusCode >= http.StatusBadRequest {
		return "upstream_client_error", "上游 4xx：请求参数、权限或模型配置异常。"
	}
	return "upstream_error", "上游请求失败。"
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
			key == "dispatchTrace" || key == "requestDispatchTags" || key == "userDispatchTags" || key == "dispatchTags" ||
			key == "upstreamStatusCode" || key == "upstreamErrorCode" {
			continue
		}
		if key == "sourceImages" {
			next[key] = sanitizeBusinessImageJobSourceImages(value)
			continue
		}
		if key == "sourceReference" || key == "hasAttachment" {
			continue
		}
		next[key] = value
	}
	return next
}

func businessImageJobHasAttachment(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	if value, ok := payload["hasAttachment"].(bool); ok && value {
		return true
	}
	return len(providerImageSourcesFromPayload(payload["sourceImages"])) > 0
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
	views := s.businessImageJobViewsFromJobs(r.Context(), items, businessUserIDForRequest(r))
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
		UserID:         r.URL.Query().Get("userId"),
		Status:         r.URL.Query().Get("status"),
		Platform:       r.URL.Query().Get("platform"),
		ErrorType:      r.URL.Query().Get("errorType"),
		CompareBatchID: r.URL.Query().Get("compareBatchId"),
		From:           from,
		To:             to,
		Limit:          pageSize,
		Offset:         offset,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	views := s.businessImageJobViewsFromJobs(r.Context(), items, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"items": views,
		"page": paginationMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	})
}

func (s *Server) handleAdminGetBusinessImageCompareBatch(w http.ResponseWriter, r *http.Request) {
	s.reconcileStaleBusinessImageJobs(r.Context())
	batchID := strings.TrimSpace(r.PathValue("id"))
	if batchID == "" {
		writeAPIError(w, http.StatusBadRequest, "compare_batch_id_required", "compare batch id is required")
		return
	}

	store, err := s.newBusinessJobStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "job store failed"})
		return
	}
	defer store.Close()

	summaries, err := store.CompareBatchSummaries(r.Context(), "", []string{batchID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	summary := summaries[batchID]
	if summary.Total <= 0 {
		writeAPIError(w, http.StatusNotFound, "compare_batch_not_found", "compare batch not found")
		return
	}

	items, _, err := store.AdminList(r.Context(), businessjobs.AdminListFilter{
		CompareBatchID: batchID,
		Limit:          200,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	sortBusinessImageCompareBatchJobs(items)
	views := s.businessImageJobViewsFromJobs(r.Context(), items, "")
	writeJSON(w, http.StatusOK, businessImageCompareBatchView{
		Summary: summary,
		Items:   views,
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
	views := s.businessImageJobViewsFromJobs(r.Context(), []businessjobs.Job{item}, businessUserIDForRequest(r))
	if len(views) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"item": businessImageJobViewFromJob(item, s.businessProviderGroupNamesByID(r.Context(), []businessjobs.Job{item}))})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": views[0]})
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
	views := s.businessImageJobViewsFromJobs(r.Context(), []businessjobs.Job{item}, userID)
	view := businessImageJobViewFromJob(item, s.businessProviderGroupNamesByID(r.Context(), []businessjobs.Job{item}))
	if len(views) > 0 {
		view = views[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"item":            view,
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
