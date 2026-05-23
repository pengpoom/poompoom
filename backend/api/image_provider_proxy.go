package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
	"imagestudio/internal/businessjobs"
	"imagestudio/internal/businessproviders"
	"imagestudio/internal/businesssettings"
	"imagestudio/internal/businesstracker"
)

const maxImageProviderRequestBytes = 96 << 20
const maxImageProviderResponseBytes = 80 << 20
const maxProviderImageGenerationAttempts = 2
const imageProviderOpenAICompatible = "openai_compatible"
const imageProviderGeminiBanana = "gemini_banana"
const defaultGeminiBananaModel = "gemini-2.5-flash-image"

type imageProviderProxyConfig struct {
	Provider       string
	ProviderID     string
	ProviderName   string
	Platform       string
	BaseURL        string
	APIKey         string
	Model          string
	RequestTimeout time.Duration
}

type providerImageGenerateMetadata struct {
	ConversationID string
	TurnID         string
	JobID          string
	Title          string
	Platform       string
}

type providerImageSource struct {
	ID      string
	Role    string
	Name    string
	DataURL string
	URL     string
}

type providerEditAsset struct {
	Source providerImageSource
	Data   []byte
}

type providerResolvedEditInput struct {
	Images []providerEditAsset
	Mask   *providerEditAsset
}

type providerImageGenerateExecution struct {
	Context            context.Context
	UserID             string
	Payload            map[string]any
	StartedAt          time.Time
	AfterRunningMarked func()
}

type providerImageGenerateResult struct {
	StatusCode   int
	ErrorCode    string
	ErrorMessage string
	ContentType  string
	Body         []byte
}

type providerImageGenerateJobPayload struct {
	Job            businessjobs.Job `json:"job"`
	JobID          string           `json:"jobId"`
	ConversationID string           `json:"conversationId,omitempty"`
	TurnID         string           `json:"turnId,omitempty"`
	Status         string           `json:"status"`
}

type providerImageGenerateSubmitError struct {
	result providerImageGenerateResult
}

func (e *providerImageGenerateSubmitError) Error() string {
	return strings.TrimSpace(e.result.ErrorMessage)
}

func newProviderImageGenerateSubmitError(result providerImageGenerateResult) error {
	return &providerImageGenerateSubmitError{result: result}
}

func providerImageGenerateSuccess(body []byte, contentType string) providerImageGenerateResult {
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}
	return providerImageGenerateResult{
		StatusCode:  http.StatusOK,
		ContentType: contentType,
		Body:        body,
	}
}

func providerImageGenerateError(statusCode int, code, message string) providerImageGenerateResult {
	return providerImageGenerateResult{
		StatusCode:   statusCode,
		ErrorCode:    code,
		ErrorMessage: message,
	}
}

func providerImageGenerateAdmissionError(err error) providerImageGenerateResult {
	if errors.Is(err, errImageAdmissionQueueFull) {
		return providerImageGenerateError(http.StatusTooManyRequests, "image_queue_full", "现在使用人数较多，请稍后使用。")
	}
	if errors.Is(err, errImageAdmissionQueueTimeout) {
		return providerImageGenerateError(http.StatusGatewayTimeout, "image_queue_timeout", "现在使用人数较多，请稍后使用。")
	}
	return providerImageGenerateError(http.StatusGatewayTimeout, "image_queue_cancelled", "现在使用人数较多，请稍后使用。")
}

func (result providerImageGenerateResult) write(w http.ResponseWriter) {
	statusCode := result.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	if statusCode >= http.StatusBadRequest {
		writeAPIError(w, statusCode, result.ErrorCode, result.ErrorMessage)
		return
	}
	contentType := strings.TrimSpace(result.ContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	_, _ = w.Write(result.Body)
}

func (s *Server) handleProviderImageGenerate(w http.ResponseWriter, r *http.Request) {
	if s.rejectIfMaintenanceMode(w) {
		return
	}
	startedAt := time.Now().UTC()
	userID := businessUserIDForRequest(r)
	var payload map[string]any
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxImageProviderRequestBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		finishedAt := time.Now().UTC()
		s.saveBusinessImageTracker(context.Background(), businesstracker.Record{
			ID:              businesstracker.NewRecordID(),
			UserID:          userID,
			Status:          businesstracker.StatusFailed,
			Stage:           "decode",
			ErrorCode:       "invalid_request",
			ErrorMessage:    "invalid request body",
			CreatedAt:       startedAt.Format(time.RFC3339Nano),
			FinishedAt:      finishedAt.Format(time.RFC3339Nano),
			TotalDurationMS: finishedAt.Sub(startedAt).Milliseconds(),
		})
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	s.executeProviderImageGenerate(providerImageGenerateExecution{
		Context:   r.Context(),
		UserID:    userID,
		Payload:   payload,
		StartedAt: startedAt,
	}).write(w)
}

func (s *Server) handleProviderImageGenerateSubmit(w http.ResponseWriter, r *http.Request) {
	if s.rejectIfMaintenanceMode(w) {
		return
	}
	startedAt := time.Now().UTC()
	userID := businessUserIDForRequest(r)
	var payload map[string]any
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxImageProviderRequestBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		finishedAt := time.Now().UTC()
		s.saveBusinessImageTracker(context.Background(), businesstracker.Record{
			ID:              businesstracker.NewRecordID(),
			UserID:          userID,
			Status:          businesstracker.StatusFailed,
			Stage:           "decode",
			ErrorCode:       "invalid_request",
			ErrorMessage:    "invalid request body",
			CreatedAt:       startedAt.Format(time.RFC3339Nano),
			FinishedAt:      finishedAt.Format(time.RFC3339Nano),
			TotalDurationMS: finishedAt.Sub(startedAt).Milliseconds(),
		})
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	job, err := s.createQueuedProviderImageJob(r.Context(), userID, payload, startedAt)
	if err != nil {
		var submitErr *providerImageGenerateSubmitError
		if errors.As(err, &submitErr) {
			submitErr.result.write(w)
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "image_job_create_failed", err.Error())
		return
	}

	s.wakeBusinessImageJobWorker()

	writeJSON(w, http.StatusAccepted, providerImageGenerateJobPayload{
		Job:            job,
		JobID:          job.ID,
		ConversationID: job.ConversationID,
		TurnID:         job.TurnID,
		Status:         job.Status,
	})
}

func providerImagePayloadJSON(payload map[string]any) []byte {
	if len(payload) == 0 {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}

func (s *Server) prepareProviderImagePayload(ctx context.Context, userID string, payload map[string]any) (map[string]any, providerResolvedEditInput, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	if !providerPayloadIsEdit(payload) {
		return payload, providerResolvedEditInput{}, nil
	}
	editInput, err := s.resolveProviderEditInputs(payload)
	if err != nil {
		return payload, providerResolvedEditInput{}, err
	}
	nextPayload := cloneProviderPayload(payload)
	sourceImages := make([]map[string]any, 0, len(editInput.Images)+1)
	conversationID := strings.TrimSpace(stringValue(nextPayload["conversationId"]))
	generationID := firstNonEmpty(
		strings.TrimSpace(stringValue(nextPayload["jobId"])),
		strings.TrimSpace(stringValue(nextPayload["turnId"])),
	)
	for index, image := range editInput.Images {
		sourceImages = append(sourceImages, s.providerEditAssetPayload(ctx, userID, conversationID, generationID, index, image))
	}
	if editInput.Mask != nil {
		sourceImages = append(sourceImages, s.providerEditAssetPayload(ctx, userID, conversationID, generationID, len(sourceImages), *editInput.Mask))
	}
	nextPayload["sourceImages"] = sourceImages
	return nextPayload, editInput, nil
}

func cloneProviderPayload(payload map[string]any) map[string]any {
	next := make(map[string]any, len(payload))
	for key, value := range payload {
		next[key] = value
	}
	return next
}

func (s *Server) providerEditAssetPayload(ctx context.Context, userID, conversationID, generationID string, index int, asset providerEditAsset) map[string]any {
	source := map[string]any{
		"id":   strings.TrimSpace(asset.Source.ID),
		"role": firstNonEmpty(strings.TrimSpace(asset.Source.Role), "image"),
		"name": firstNonEmpty(strings.TrimSpace(asset.Source.Name), fmt.Sprintf("source-%d.png", index+1)),
	}
	if url := strings.TrimSpace(asset.Source.URL); url != "" {
		source["url"] = url
		return source
	}
	if len(asset.Data) == 0 || conversationID == "" || generationID == "" {
		if dataURL := strings.TrimSpace(asset.Source.DataURL); dataURL != "" {
			source["dataUrl"] = dataURL
		}
		return source
	}
	mimeType := http.DetectContentType(asset.Data)
	fileName := strings.TrimSpace(asset.Source.Name)
	if fileName == "" {
		fileName = fmt.Sprintf("source-%d%s", index+1, extensionForImageMIME(mimeType))
	}
	if ext := filepath.Ext(fileName); ext == "" {
		fileName += extensionForImageMIME(mimeType)
	}
	url, _, err := s.saveBusinessImageBytes(ctx, asset.Data, userID, conversationID, generationID, index, fileName)
	if err != nil {
		if dataURL := strings.TrimSpace(asset.Source.DataURL); dataURL != "" {
			source["dataUrl"] = dataURL
		}
		return source
	}
	source["url"] = url
	return source
}

func (s *Server) createQueuedProviderImageJob(ctx context.Context, userID string, payload map[string]any, startedAt time.Time) (businessjobs.Job, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	metadata := extractProviderImageGenerateMetadata(payload)
	providerCfg, err := s.imageProviderProxyConfig(metadata.Platform)
	if err != nil {
		return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageGenerateError(http.StatusInternalServerError, "provider_not_configured", err.Error()))
	}
	prompt := strings.TrimSpace(stringValue(payload["prompt"]))
	if prompt == "" {
		return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageGenerateError(http.StatusBadRequest, "invalid_request", "prompt is required"))
	}
	payload["prompt"] = prompt
	requestModel := providerImageRequestModel(payload, providerCfg)
	payload["model"] = requestModel
	if normalizePositiveInt(payload["n"]) <= 0 {
		payload["n"] = 1
	}
	if strings.TrimSpace(metadata.JobID) == "" {
		metadata.JobID = businessjobs.NewJobID()
		payload["jobId"] = metadata.JobID
	}
	requestedCount := normalizePositiveInt(payload["n"])
	if requestedCount <= 0 {
		requestedCount = 1
	}
	systemSettings := s.businessSystemSettingsForContext(ctx)
	if systemSettings.Generation.MaxCount > 0 && requestedCount > systemSettings.Generation.MaxCount {
		return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageGenerateError(http.StatusBadRequest, "invalid_request", fmt.Sprintf("单次最多生成 %d 张图片", systemSettings.Generation.MaxCount)))
	}
	payload, _, err = s.prepareProviderImagePayload(ctx, userID, payload)
	if err != nil {
		if providerErr, ok := err.(*providerGenerationError); ok {
			return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, providerErr.Message))
		}
		return businessjobs.Job{}, err
	}
	generationID := firstNonEmpty(metadata.JobID, metadata.TurnID)
	job := businessjobs.Job{
		ID:             metadata.JobID,
		UserID:         userID,
		ConversationID: metadata.ConversationID,
		GenerationID:   generationID,
		TurnID:         metadata.TurnID,
		Platform:       providerCfg.Platform,
		ProviderID:     providerCfg.ProviderID,
		ProviderName:   providerCfg.ProviderName,
		Model:          requestModel,
		Prompt:         prompt,
		Size:           strings.TrimSpace(stringValue(payload["size"])),
		Quality:        strings.TrimSpace(stringValue(payload["quality"])),
		RequestedCount: requestedCount,
		Status:         businessjobs.StatusQueued,
		Stage:          "queued",
		CreditReserved: businesssettings.CreditCostForPlatform(systemSettings, providerCfg.Platform, requestedCount),
		PayloadJSON:    providerImagePayloadJSON(payload),
		CreatedAt:      startedAt.Format(time.RFC3339Nano),
		QueuedAt:       startedAt.Format(time.RFC3339Nano),
	}
	store, err := businessjobs.NewStore(s.cfg)
	if err != nil {
		return businessjobs.Job{}, err
	}
	defer store.Close()
	saved, err := store.Save(ctx, job)
	if err != nil {
		return businessjobs.Job{}, err
	}
	s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, businessjobs.StatusQueued, "", startedAt)
	return saved, nil
}

func (s *Server) runProviderImageGenerateJob(userID string, payload map[string]any, startedAt time.Time) {
	s.executeProviderImageGenerate(providerImageGenerateExecution{
		Context:   context.Background(),
		UserID:    userID,
		Payload:   payload,
		StartedAt: startedAt,
	})
}

func (s *Server) executeProviderImageGenerate(execution providerImageGenerateExecution) providerImageGenerateResult {
	ctx := execution.Context
	if ctx == nil {
		ctx = context.Background()
	}
	startedAt := execution.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	userID := strings.TrimSpace(execution.UserID)
	payload := execution.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	tracker := businesstracker.Record{
		ID:        businesstracker.NewRecordID(),
		UserID:    userID,
		Status:    businesstracker.StatusFailed,
		Stage:     "received",
		CreatedAt: startedAt.Format(time.RFC3339Nano),
	}
	finishTracker := func(status, stage, errorCode, errorMessage string) {
		finishedAt := time.Now().UTC()
		tracker.Status = status
		tracker.Stage = stage
		tracker.ErrorCode = strings.TrimSpace(errorCode)
		tracker.ErrorMessage = strings.TrimSpace(errorMessage)
		tracker.FinishedAt = finishedAt.Format(time.RFC3339Nano)
		tracker.TotalDurationMS = finishedAt.Sub(startedAt).Milliseconds()
		s.saveBusinessImageTracker(context.Background(), tracker)
	}

	metadata := extractProviderImageGenerateMetadata(payload)
	tracker.ConversationID = metadata.ConversationID
	tracker.TurnID = metadata.TurnID
	tracker.GenerationID = firstNonEmpty(metadata.JobID, metadata.TurnID)
	tracker.Platform = businessproviders.NormalizePlatform(metadata.Platform)
	job := businessjobs.Job{
		ID:             firstNonEmpty(metadata.JobID, businessjobs.NewJobID()),
		UserID:         userID,
		ConversationID: metadata.ConversationID,
		GenerationID:   firstNonEmpty(metadata.JobID, metadata.TurnID),
		TurnID:         metadata.TurnID,
		Platform:       tracker.Platform,
		Status:         businessjobs.StatusQueued,
		Stage:          "received",
		PayloadJSON:    providerImagePayloadJSON(payload),
		CreatedAt:      startedAt.Format(time.RFC3339Nano),
		QueuedAt:       startedAt.Format(time.RFC3339Nano),
	}
	jobCtx, cancelJobContext := context.WithCancel(ctx)
	unregisterJob := s.registerActiveBusinessImageJob(job.ID, cancelJobContext)
	defer unregisterJob()
	defer cancelJobContext()

	saveJob := func(next businessjobs.Job) {
		store, err := businessjobs.NewStore(s.cfg)
		if err != nil {
			return
		}
		defer store.Close()
		if current, ok, getErr := store.Get(context.Background(), next.ID, next.UserID); getErr == nil && ok {
			if current.CreditRefunded > next.CreditRefunded {
				next.CreditRefunded = current.CreditRefunded
			}
			if shouldPreserveCurrentBusinessImageJob(current, next) {
				job = current
				return
			}
			if len(next.PayloadJSON) == 0 && len(current.PayloadJSON) > 0 {
				next.PayloadJSON = current.PayloadJSON
			}
		}
		saved, err := store.Save(context.Background(), next)
		if err == nil {
			job = saved
		}
	}
	finishJob := func(status, stage, errorCode, errorMessage string) {
		finishedAt := time.Now().UTC()
		job.Status = status
		job.Stage = stage
		job.ErrorCode = strings.TrimSpace(errorCode)
		job.ErrorMessage = strings.TrimSpace(errorMessage)
		job.FinishedAt = finishedAt.Format(time.RFC3339Nano)
		job.TotalDurationMS = finishedAt.Sub(startedAt).Milliseconds()
		saveJob(job)
	}
	cancelGeneration := func(message string) {
		if strings.TrimSpace(message) == "" {
			message = "任务已取消"
		}
		s.recordProviderImageGeneration(context.Background(), userID, metadata, payload, nil, businessjobs.StatusCancelled, message, startedAt)
		finishJob(businessjobs.StatusCancelled, "cancelled", "cancelled", message)
		finishTracker(businesstracker.StatusFailed, "cancelled", "cancelled", message)
	}
	providerCfg, err := s.imageProviderProxyConfig(metadata.Platform)
	if err != nil {
		job.Status = businessjobs.StatusFailed
		job.Stage = "provider_config"
		job.ErrorCode = "provider_not_configured"
		job.ErrorMessage = err.Error()
		job.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		saveJob(job)
		finishTracker(businesstracker.StatusFailed, "provider_config", "provider_not_configured", err.Error())
		return providerImageGenerateError(http.StatusInternalServerError, "provider_not_configured", err.Error())
	}
	tracker.Platform = providerCfg.Platform
	tracker.ProviderID = providerCfg.ProviderID
	tracker.ProviderName = providerCfg.ProviderName
	job.Platform = providerCfg.Platform
	job.ProviderID = providerCfg.ProviderID
	job.ProviderName = providerCfg.ProviderName
	prompt := strings.TrimSpace(stringValue(payload["prompt"]))
	if prompt == "" {
		job.Prompt = prompt
		finishJob(businessjobs.StatusFailed, "validation", "invalid_request", "prompt is required")
		finishTracker(businesstracker.StatusFailed, "validation", "invalid_request", "prompt is required")
		return providerImageGenerateError(http.StatusBadRequest, "invalid_request", "prompt is required")
	}
	payload["prompt"] = prompt
	requestModel := providerImageRequestModel(payload, providerCfg)
	payload["model"] = requestModel
	tracker.Model = requestModel
	job.Prompt = prompt
	job.Model = requestModel
	job.Size = strings.TrimSpace(stringValue(payload["size"]))
	job.Quality = strings.TrimSpace(stringValue(payload["quality"]))
	if normalizePositiveInt(payload["n"]) <= 0 {
		payload["n"] = 1
	}
	requestedCount := normalizePositiveInt(payload["n"])
	if requestedCount <= 0 {
		requestedCount = 1
	}
	systemSettings := s.businessSystemSettingsForContext(ctx)
	if systemSettings.Generation.MaxCount > 0 && requestedCount > systemSettings.Generation.MaxCount {
		tracker.RequestedCount = requestedCount
		job.RequestedCount = requestedCount
		job.CreditReserved = businesssettings.CreditCostForPlatform(systemSettings, providerCfg.Platform, requestedCount)
		finishJob(businessjobs.StatusFailed, "validation", "invalid_request", fmt.Sprintf("单次最多生成 %d 张图片", systemSettings.Generation.MaxCount))
		finishTracker(businesstracker.StatusFailed, "validation", "invalid_request", fmt.Sprintf("单次最多生成 %d 张图片", systemSettings.Generation.MaxCount))
		return providerImageGenerateError(http.StatusBadRequest, "invalid_request", fmt.Sprintf("单次最多生成 %d 张图片", systemSettings.Generation.MaxCount))
	}
	creditCost := businesssettings.CreditCostForPlatform(systemSettings, providerCfg.Platform, requestedCount)
	generationID := firstNonEmpty(metadata.JobID, metadata.TurnID)
	tracker.RequestedCount = requestedCount
	tracker.CreditReserved = creditCost
	job.GenerationID = generationID
	job.RequestedCount = requestedCount
	job.CreditReserved = creditCost
	if errors.Is(jobCtx.Err(), context.Canceled) {
		cancelGeneration("任务已取消")
		return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
	}
	if latestJob, ok := s.businessImageJobByID(job.ID, userID); ok && businessJobCancelled(latestJob) {
		job = latestJob
		cancelGeneration("任务已取消")
		return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
	}
	job.Status = businessjobs.StatusQueued
	job.Stage = "queued"
	saveJob(job)
	s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "queued", "", startedAt)

	admissionInfo, releaseAdmission, admissionErr := s.acquireImageAdmission(jobCtx)
	if admissionErr != nil {
		tracker.QueueWaitMS = admissionInfo.QueueWaitMS
		job.QueueWaitMS = admissionInfo.QueueWaitMS
		if errors.Is(jobCtx.Err(), context.Canceled) {
			s.recordProviderImageGenerationPlaceholder(context.Background(), userID, metadata, payload, providerCfg, "cancelled", "任务已取消", startedAt)
			finishJob(businessjobs.StatusCancelled, "cancelled", "cancelled", "任务已取消")
			finishTracker(businesstracker.StatusFailed, "cancelled", "cancelled", "任务已取消")
			return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
		}
		s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", "现在使用人数较多，请稍后使用。", startedAt)
		finishJob(businessjobs.StatusFailed, "admission", imageAdmissionErrorCode(admissionErr), "现在使用人数较多，请稍后使用。")
		finishTracker(businesstracker.StatusFailed, "admission", imageAdmissionErrorCode(admissionErr), "现在使用人数较多，请稍后使用。")
		return providerImageGenerateAdmissionError(admissionErr)
	}
	defer releaseAdmission()
	ctx = withImageAdmissionInfo(jobCtx, admissionInfo)
	tracker.QueueWaitMS = admissionInfo.QueueWaitMS
	tracker.AdmittedAt = time.Now().UTC().Format(time.RFC3339Nano)
	job.QueueWaitMS = admissionInfo.QueueWaitMS
	job.Status = businessjobs.StatusRunning
	job.Stage = "running"
	job.StartedAt = tracker.AdmittedAt
	saveJob(job)
	if execution.AfterRunningMarked != nil {
		execution.AfterRunningMarked()
	}
	if latestJob, ok := s.businessImageJobByID(job.ID, userID); ok && businessJobCancelled(latestJob) {
		job = latestJob
		cancelGeneration("任务已取消")
		return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
	}
	s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "running", "", startedAt)

	creditStore, err := businesscredits.NewStore(s.cfg)
	if err != nil {
		s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", "credit store failed", startedAt)
		finishJob(businessjobs.StatusFailed, "credit", "credit_store_failed", "credit store failed")
		finishTracker(businesstracker.StatusFailed, "credit", "credit_store_failed", "credit store failed")
		return providerImageGenerateError(http.StatusInternalServerError, "credit_store_failed", "credit store failed")
	}
	defer creditStore.Close()
	if _, _, err := creditStore.Reserve(ctx, userID, creditCost, generationID); err != nil {
		if errors.Is(err, businesscredits.ErrInsufficientBalance) {
			s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", "点数余额不足", startedAt)
			finishJob(businessjobs.StatusFailed, "credit", "insufficient_credits", "点数余额不足")
			finishTracker(businesstracker.StatusFailed, "credit", "insufficient_credits", "点数余额不足")
			return providerImageGenerateError(http.StatusPaymentRequired, "insufficient_credits", "点数余额不足")
		}
		s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", err.Error(), startedAt)
		finishJob(businessjobs.StatusFailed, "credit", "credit_reserve_failed", err.Error())
		finishTracker(businesstracker.StatusFailed, "credit", "credit_reserve_failed", err.Error())
		return providerImageGenerateError(http.StatusInternalServerError, "credit_reserve_failed", err.Error())
	}
	creditsSettled := false
	var refundedCredits int64
	syncRefundedCredits := func() businesscredits.GenerationTotals {
		totals, err := creditStore.GenerationTotals(context.Background(), userID, generationID)
		if err != nil {
			return businesscredits.GenerationTotals{}
		}
		if totals.Refunded > refundedCredits {
			refundedCredits = totals.Refunded
			tracker.CreditRefunded = refundedCredits
			job.CreditRefunded = refundedCredits
		}
		return totals
	}
	refundReservedCredits := func(amount int64) {
		if amount <= 0 {
			return
		}
		totals := syncRefundedCredits()
		if totals.Reserved <= totals.Refunded {
			return
		}
		refundable := totals.Reserved - totals.Refunded
		if amount > refundable {
			amount = refundable
		}
		if _, _, err := creditStore.Refund(context.Background(), userID, amount, generationID); err == nil {
			refundedCredits = totals.Refunded + amount
			tracker.CreditRefunded = refundedCredits
			job.CreditRefunded = refundedCredits
		}
	}
	refundCredits := func(amount int64) {
		if !systemSettings.Billing.RefundOnFailure {
			syncRefundedCredits()
			return
		}
		refundReservedCredits(amount)
	}
	if strings.TrimSpace(stringValue(payload["response_format"])) == "" {
		payload["response_format"] = "b64_json"
	}
	var editInput providerResolvedEditInput
	payload, editInput, err = s.prepareProviderImagePayload(ctx, userID, payload)
	if err != nil {
		job.PayloadJSON = providerImagePayloadJSON(payload)
		saveJob(job)
		providerErr := providerGenerationErrorDetails(err)
		refundCredits(creditCost)
		job.CreditRefunded = refundedCredits
		s.recordProviderImageGeneration(ctx, userID, metadata, payload, providerErr.ResponseBody, "failed", providerErr.Message, startedAt)
		finishJob(businessjobs.StatusFailed, "validation", providerErr.Code, providerErr.Message)
		finishTracker(businesstracker.StatusFailed, "validation", providerErr.Code, providerErr.Message)
		return providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, providerErr.Message)
	}
	job.PayloadJSON = providerImagePayloadJSON(payload)
	saveJob(job)
	providerPayload := buildProviderImageGeneratePayload(payload)

	if latestJob, ok := s.businessImageJobByID(job.ID, userID); ok && businessJobCancelled(latestJob) {
		job = latestJob
		refundReservedCredits(creditCost)
		job.CreditRefunded = refundedCredits
		cancelGeneration("任务已取消")
		return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
	}
	job.Stage = "dispatching"
	saveJob(job)
	if latestJob, ok := s.businessImageJobByID(job.ID, userID); ok && businessJobCancelled(latestJob) {
		job = latestJob
		refundReservedCredits(creditCost)
		job.CreditRefunded = refundedCredits
		cancelGeneration("任务已取消")
		return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
	}
	var upstreamStarted bool
	var upstreamStartedAt time.Time
	markUpstreamStarted := func() {
		if upstreamStarted {
			return
		}
		upstreamStarted = true
		upstreamStartedAt = time.Now().UTC()
		tracker.UpstreamStartedAt = upstreamStartedAt.Format(time.RFC3339Nano)
		job.UpstreamSent = true
		job.Stage = "upstream"
		saveJob(job)
	}
	var body []byte
	var contentType string
	body, contentType, err = executeProviderImageGenerationWithRetry(ctx, providerCfg, providerPayload, requestedCount, editInput, providerImageGenerationHooks{
		BeforeUpstreamAttempt: markUpstreamStarted,
	})
	upstreamFinishedAt := time.Now().UTC()
	if upstreamStarted {
		tracker.UpstreamFinishedAt = upstreamFinishedAt.Format(time.RFC3339Nano)
		tracker.UpstreamDurationMS = upstreamFinishedAt.Sub(upstreamStartedAt).Milliseconds()
		job.UpstreamDurationMS = tracker.UpstreamDurationMS
	}
	if latestJob, ok := s.businessImageJobByID(job.ID, userID); ok {
		if businessJobCancelled(latestJob) {
			job = latestJob
			if upstreamStarted {
				syncRefundedCredits()
			} else {
				refundReservedCredits(creditCost)
			}
			cancelGeneration("任务已取消")
			return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
		}
		if businessJobFinal(latestJob) {
			job = latestJob
			return providerImageGenerateError(http.StatusConflict, firstNonEmpty(job.ErrorCode, job.Status), firstNonEmpty(job.ErrorMessage, "任务已结束"))
		}
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			if upstreamStarted {
				syncRefundedCredits()
			} else {
				refundReservedCredits(creditCost)
			}
			cancelGeneration("任务已取消")
			return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
		}
		providerErr := providerGenerationErrorDetails(err)
		errorMessage := providerErr.Message
		refundCredits(creditCost)
		job.CreditRefunded = refundedCredits
		s.recordProviderImageGeneration(ctx, userID, metadata, providerPayload, providerErr.ResponseBody, "failed", errorMessage, startedAt)
		finishJob(businessjobs.StatusFailed, "upstream", providerErr.Code, errorMessage)
		finishTracker(businesstracker.StatusFailed, "upstream", providerErr.Code, errorMessage)
		return providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, errorMessage)
	}
	actualCount := countProviderImageItems(body)
	if actualCount <= 0 {
		actualCount = requestedCount
	}
	tracker.ActualCount = actualCount
	job.ActualCount = actualCount
	if actualCount < requestedCount && systemSettings.Billing.RefundPartialCount {
		actualCost := businesssettings.CreditCostForPlatform(systemSettings, providerCfg.Platform, actualCount)
		refundCredits(creditCost - actualCost)
	}
	if latestJob, ok := s.businessImageJobByID(job.ID, userID); ok && businessJobCancelled(latestJob) {
		if upstreamStarted {
			syncRefundedCredits()
		} else {
			refundReservedCredits(creditCost)
		}
		cancelGeneration("任务已取消")
		return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
	}
	creditsSettled = true
	recordResult := s.recordProviderImageGeneration(ctx, userID, metadata, providerPayload, body, "succeeded", "", startedAt)
	tracker.PersistDurationMS = recordResult.PersistDurationMS
	tracker.StorageBytes = recordResult.StorageBytes
	job.PersistDurationMS = recordResult.PersistDurationMS
	job.StorageBytes = recordResult.StorageBytes
	job.CreditRefunded = refundedCredits
	finishJob(businessjobs.StatusSucceeded, "finished", "", "")
	finishTracker(businesstracker.StatusSucceeded, "finished", "", "")
	if !creditsSettled {
		refundCredits(int64(requestedCount))
	}

	return providerImageGenerateSuccess(body, contentType)
}

func (s *Server) recordProviderImageGenerationPlaceholder(ctx context.Context, userID string, metadata providerImageGenerateMetadata, payload map[string]any, providerCfg imageProviderProxyConfig, status string, errorMessage string, startedAt time.Time) {
	conversationID := strings.TrimSpace(metadata.ConversationID)
	turnID := strings.TrimSpace(metadata.TurnID)
	if conversationID == "" || turnID == "" {
		return
	}
	store, err := businessimage.NewStore(s.cfg)
	if err != nil {
		return
	}
	defer store.Close()

	now := time.Now().UTC()
	if startedAt.IsZero() {
		startedAt = now
	}
	startedAtText := startedAt.Format(time.RFC3339Nano)
	nowText := now.Format(time.RFC3339Nano)
	title := firstNonEmpty(metadata.Title, buildProviderImageConversationTitle(stringValue(payload["prompt"])))
	normalizedStatus := strings.TrimSpace(status)
	if normalizedStatus == "" {
		normalizedStatus = "queued"
	}
	count := normalizePositiveInt(payload["n"])
	if count <= 0 {
		count = 1
	}
	model := strings.TrimSpace(stringValue(payload["model"]))
	if model == "" {
		model = providerCfg.Model
	}
	_, _ = store.UpsertConversation(ctx, businessimage.Conversation{
		ID:        conversationID,
		UserID:    userID,
		Title:     title,
		CreatedAt: startedAtText,
		UpdatedAt: nowText,
	})
	_, _ = store.SaveGeneration(ctx, businessimage.Generation{
		ID:             firstNonEmpty(metadata.JobID, turnID),
		UserID:         userID,
		ConversationID: conversationID,
		TurnID:         turnID,
		Prompt:         strings.TrimSpace(stringValue(payload["prompt"])),
		Model:          model,
		Size:           strings.TrimSpace(stringValue(payload["size"])),
		Quality:        strings.TrimSpace(stringValue(payload["quality"])),
		Count:          count,
		Status:         normalizedStatus,
		Response:       nil,
		Error:          strings.TrimSpace(errorMessage),
		CreatedAt:      startedAtText,
		FinishedAt:     nowText,
	})
}

func (s *Server) saveBusinessImageTracker(ctx context.Context, record businesstracker.Record) {
	store, err := businesstracker.NewStore(s.cfg)
	if err != nil {
		return
	}
	defer store.Close()
	_, _ = store.Save(ctx, record)
}

func imageAdmissionErrorCode(err error) string {
	if errors.Is(err, errImageAdmissionQueueFull) {
		return "image_queue_full"
	}
	if errors.Is(err, errImageAdmissionQueueTimeout) {
		return "image_queue_timeout"
	}
	return "image_queue_cancelled"
}

func providerImageRequestModel(payload map[string]any, providerCfg imageProviderProxyConfig) string {
	if providerCfg.Provider == imageProviderGeminiBanana {
		model := strings.TrimSpace(providerCfg.Model)
		if model == "" {
			return defaultGeminiBananaModel
		}
		return model
	}
	model := strings.TrimSpace(stringValue(payload["model"]))
	if model != "" {
		return model
	}
	model = strings.TrimSpace(providerCfg.Model)
	if model != "" {
		return model
	}
	return cpaFixedImageModel
}

func extractProviderImageGenerateMetadata(payload map[string]any) providerImageGenerateMetadata {
	return providerImageGenerateMetadata{
		ConversationID: strings.TrimSpace(stringValue(payload["conversationId"])),
		TurnID:         strings.TrimSpace(stringValue(payload["turnId"])),
		JobID:          strings.TrimSpace(firstNonEmpty(stringValue(payload["jobId"]), stringValue(payload["taskId"]))),
		Title:          strings.TrimSpace(stringValue(payload["title"])),
		Platform:       strings.TrimSpace(stringValue(payload["platform"])),
	}
}

func buildProviderImageGeneratePayload(payload map[string]any) map[string]any {
	allowed := map[string]struct{}{
		"prompt":          {},
		"model":           {},
		"mode":            {},
		"n":               {},
		"size":            {},
		"quality":         {},
		"response_format": {},
		"background":      {},
		"style":           {},
		"moderation":      {},
		"user":            {},
		"sourceImages":    {},
		"sourceReference": {},
	}
	next := map[string]any{}
	for key, value := range payload {
		if _, ok := allowed[key]; !ok {
			continue
		}
		next[key] = value
	}
	return next
}

func providerPayloadIsEdit(payload map[string]any) bool {
	if strings.EqualFold(strings.TrimSpace(stringValue(payload["mode"])), "edit") {
		return true
	}
	return len(providerImageSourcesFromPayload(payload["sourceImages"])) > 0
}

func providerImageSourcesFromPayload(raw any) []providerImageSource {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	sources := make([]providerImageSource, 0, len(items))
	for _, item := range items {
		source, ok := item.(map[string]any)
		if !ok {
			continue
		}
		normalized := providerImageSource{
			ID:      strings.TrimSpace(stringValue(source["id"])),
			Role:    firstNonEmpty(strings.TrimSpace(stringValue(source["role"])), "image"),
			Name:    strings.TrimSpace(stringValue(source["name"])),
			DataURL: strings.TrimSpace(stringValue(source["dataUrl"])),
			URL:     strings.TrimSpace(stringValue(source["url"])),
		}
		if normalized.DataURL == "" && normalized.URL == "" {
			continue
		}
		sources = append(sources, normalized)
	}
	return sources
}

func (s *Server) resolveProviderEditInputs(payload map[string]any) (providerResolvedEditInput, error) {
	sources := providerImageSourcesFromPayload(payload["sourceImages"])
	result := providerResolvedEditInput{Images: make([]providerEditAsset, 0, len(sources))}
	for _, source := range sources {
		data, err := s.resolveProviderSourceImageBytes(source)
		if err != nil {
			return providerResolvedEditInput{}, err
		}
		asset := providerEditAsset{
			Source: source,
			Data:   data,
		}
		if strings.EqualFold(strings.TrimSpace(source.Role), "mask") {
			result.Mask = &asset
			continue
		}
		result.Images = append(result.Images, asset)
	}
	if len(result.Images) == 0 {
		return providerResolvedEditInput{}, &providerGenerationError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "image_required",
			Message:    "编辑模式至少需要一张源图",
		}
	}
	return result, nil
}

func (s *Server) resolveProviderSourceImageBytes(source providerImageSource) ([]byte, error) {
	if strings.TrimSpace(source.DataURL) != "" {
		payload, _, err := decodeBase64ImagePayload(source.DataURL)
		if err != nil {
			return nil, &providerGenerationError{
				HTTPStatus: http.StatusBadRequest,
				Code:       "invalid_source_image",
				Message:    err.Error(),
			}
		}
		return payload, nil
	}
	rawURL := strings.TrimSpace(source.URL)
	if rawURL == "" {
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_source_image",
			Message:    "source image is empty",
		}
	}
	if index := strings.Index(rawURL, "/v1/files/image/"); index >= 0 {
		name := rawURL[index+len("/v1/files/image/"):]
		name = strings.ReplaceAll(name, "/", "-")
		path := s.resolveImageFilePath(name)
		if path == "" {
			return nil, &providerGenerationError{
				HTTPStatus: http.StatusBadRequest,
				Code:       "source_image_not_found",
				Message:    "source image not found",
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, &providerGenerationError{
				HTTPStatus: http.StatusBadRequest,
				Code:       "source_image_not_found",
				Message:    err.Error(),
			}
		}
		return data, nil
	}
	resp, err := compatImageFetchClient.Get(rawURL)
	if err != nil {
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "source_image_fetch_failed",
			Message:    err.Error(),
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &providerGenerationError{
			HTTPStatus:     http.StatusBadGateway,
			UpstreamStatus: resp.StatusCode,
			Code:           "source_image_fetch_failed",
			Message:        fmt.Sprintf("fetch image returned %d", resp.StatusCode),
		}
	}
	return io.ReadAll(io.LimitReader(resp.Body, int64(max(1, s.cfg.App.MaxUploadSizeMB))<<20))
}

type providerImageGenerationRecordResult struct {
	PersistDurationMS int64
	StorageBytes      int64
}

type providerImagePersistenceStats struct {
	ImageCount   int
	StorageBytes int64
}

func (s *Server) recordProviderImageGeneration(ctx context.Context, userID string, metadata providerImageGenerateMetadata, payload map[string]any, responseBody []byte, status string, errorMessage string, startedAt time.Time) (result providerImageGenerationRecordResult) {
	recordStartedAt := time.Now()
	defer func() {
		result.PersistDurationMS = time.Since(recordStartedAt).Milliseconds()
	}()
	conversationID := strings.TrimSpace(metadata.ConversationID)
	turnID := strings.TrimSpace(metadata.TurnID)
	if conversationID == "" || turnID == "" {
		return result
	}
	store, err := businessimage.NewStore(s.cfg)
	if err != nil {
		return result
	}
	defer store.Close()

	finishedAt := time.Now().UTC()
	if startedAt.IsZero() {
		startedAt = finishedAt
	}
	startedAtText := startedAt.Format(time.RFC3339Nano)
	finishedAtText := finishedAt.Format(time.RFC3339Nano)
	title := firstNonEmpty(metadata.Title, buildProviderImageConversationTitle(stringValue(payload["prompt"])))
	normalizedStatus := strings.TrimSpace(status)
	if normalizedStatus == "" {
		normalizedStatus = "succeeded"
	}
	count := normalizePositiveInt(payload["n"])
	if count <= 0 {
		count = 1
	}
	persistedResponse := responseBody
	if normalizedStatus == "succeeded" {
		var stats providerImagePersistenceStats
		persistedResponse, stats = s.persistProviderImageResponse(ctx, responseBody, userID, conversationID, firstNonEmpty(metadata.JobID, turnID))
		result.StorageBytes = stats.StorageBytes
	}
	persistedResponse = injectProviderImageResponsePlatform(persistedResponse, metadata.Platform)
	_, _ = store.UpsertConversation(ctx, businessimage.Conversation{
		ID:        conversationID,
		UserID:    userID,
		Title:     title,
		CreatedAt: startedAtText,
		UpdatedAt: finishedAtText,
	})
	_, _ = store.SaveGeneration(ctx, businessimage.Generation{
		ID:             firstNonEmpty(metadata.JobID, turnID),
		UserID:         userID,
		ConversationID: conversationID,
		TurnID:         turnID,
		Prompt:         strings.TrimSpace(stringValue(payload["prompt"])),
		Model:          strings.TrimSpace(stringValue(payload["model"])),
		Size:           strings.TrimSpace(stringValue(payload["size"])),
		Quality:        strings.TrimSpace(stringValue(payload["quality"])),
		Count:          count,
		Status:         normalizedStatus,
		Response:       append([]byte(nil), persistedResponse...),
		Error:          strings.TrimSpace(errorMessage),
		CreatedAt:      startedAtText,
		FinishedAt:     finishedAtText,
	})
	return result
}

func (s *Server) persistProviderImageResponse(ctx context.Context, responseBody []byte, userID, conversationID, generationID string) ([]byte, providerImagePersistenceStats) {
	stats := providerImagePersistenceStats{}
	if len(responseBody) == 0 {
		return responseBody, stats
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return responseBody, stats
	}
	data, ok := payload["data"].([]any)
	if !ok || len(data) == 0 {
		return responseBody, stats
	}
	changed := false
	for index, raw := range data {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		b64 := strings.TrimSpace(stringValue(item["b64_json"]))
		if b64 == "" {
			continue
		}
		url, sizeBytes, err := s.saveBusinessImageBase64(ctx, b64, userID, conversationID, generationID, index)
		if err != nil {
			continue
		}
		delete(item, "b64_json")
		item["url"] = url
		stats.ImageCount++
		stats.StorageBytes += sizeBytes
		changed = true
	}
	if !changed {
		return responseBody, stats
	}
	next, err := json.Marshal(payload)
	if err != nil {
		return responseBody, stats
	}
	return next, stats
}

func injectProviderImageResponsePlatform(responseBody []byte, platform string) []byte {
	normalizedPlatform := businessproviders.NormalizePlatform(platform)
	if normalizedPlatform == "" {
		normalizedPlatform = businessproviders.PlatformGPTImage
	}
	if len(responseBody) == 0 {
		responseBody = []byte(`{}`)
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return responseBody
	}
	payload["platform"] = normalizedPlatform
	next, err := json.Marshal(payload)
	if err != nil {
		return responseBody
	}
	return next
}

type providerGenerationError struct {
	HTTPStatus     int
	UpstreamStatus int
	Code           string
	Message        string
	ResponseBody   []byte
}

type providerImageGenerationHooks struct {
	BeforeUpstreamAttempt func()
}

func (e *providerGenerationError) Error() string {
	return e.Message
}

func providerGenerationErrorDetails(err error) providerGenerationError {
	var providerErr *providerGenerationError
	if errors.As(err, &providerErr) && providerErr != nil {
		return *providerErr
	}
	return providerGenerationError{
		HTTPStatus: http.StatusBadGateway,
		Code:       "provider_request_failed",
		Message:    err.Error(),
	}
}

func executeProviderImageGenerationWithRetry(ctx context.Context, cfg imageProviderProxyConfig, payload map[string]any, requestedCount int, editInput providerResolvedEditInput, hooks providerImageGenerationHooks) ([]byte, string, error) {
	var (
		body        []byte
		contentType string
		err         error
	)
	for attempt := 1; attempt <= maxProviderImageGenerationAttempts; attempt++ {
		body, contentType, err = executeProviderImageGeneration(ctx, cfg, payload, requestedCount, editInput, hooks)
		if err == nil {
			return body, contentType, nil
		}
		if !isRetryableProviderGenerationError(ctx, err) || attempt >= maxProviderImageGenerationAttempts {
			return body, contentType, err
		}
		select {
		case <-ctx.Done():
			return body, contentType, err
		case <-time.After(time.Duration(attempt) * 300 * time.Millisecond):
		}
	}
	return body, contentType, err
}

func isRetryableProviderGenerationError(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	details := providerGenerationErrorDetails(err)
	switch details.Code {
	case "provider_request_failed", "provider_response_failed":
		return true
	case "provider_error":
		return details.UpstreamStatus == 0 || details.UpstreamStatus == http.StatusTooManyRequests || details.UpstreamStatus >= http.StatusInternalServerError
	default:
		return false
	}
}

func executeProviderImageGeneration(ctx context.Context, cfg imageProviderProxyConfig, payload map[string]any, requestedCount int, editInput providerResolvedEditInput, hooks providerImageGenerationHooks) ([]byte, string, error) {
	switch cfg.Provider {
	case imageProviderGeminiBanana:
		body, err := executeGeminiBananaImageGeneration(ctx, cfg, payload, requestedCount, editInput, hooks)
		return body, "application/json", err
	case imageProviderOpenAICompatible:
		if len(editInput.Images) > 0 {
			return executeOpenAICompatibleImageEdit(ctx, cfg, payload, editInput, hooks)
		}
		return executeOpenAICompatibleImageGeneration(ctx, cfg, payload, hooks)
	default:
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusInternalServerError,
			Code:       "provider_not_configured",
			Message:    fmt.Sprintf("unsupported provider %q", cfg.Provider),
		}
	}
}

func executeOpenAICompatibleImageEdit(ctx context.Context, cfg imageProviderProxyConfig, payload map[string]any, editInput providerResolvedEditInput, hooks providerImageGenerationHooks) ([]byte, string, error) {
	if len(editInput.Images) == 0 {
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "image_required",
			Message:    "编辑模式至少需要一张源图",
		}
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"prompt":          strings.TrimSpace(stringValue(payload["prompt"])),
		"model":           strings.TrimSpace(stringValue(payload["model"])),
		"response_format": strings.TrimSpace(stringValue(payload["response_format"])),
		"size":            strings.TrimSpace(stringValue(payload["size"])),
		"quality":         strings.TrimSpace(stringValue(payload["quality"])),
	}
	for key, value := range fields {
		if value != "" {
			if err := writer.WriteField(key, value); err != nil {
				return nil, "", &providerGenerationError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: err.Error()}
			}
		}
	}
	for index, image := range editInput.Images {
		name := fmt.Sprintf("image-%d%s", index+1, extensionForImageMIME(http.DetectContentType(image.Data)))
		part, err := writer.CreateFormFile("image", name)
		if err != nil {
			return nil, "", &providerGenerationError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: err.Error()}
		}
		if _, err := part.Write(image.Data); err != nil {
			return nil, "", &providerGenerationError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: err.Error()}
		}
	}
	if editInput.Mask != nil && len(editInput.Mask.Data) > 0 {
		part, err := writer.CreateFormFile("mask", "mask.png")
		if err != nil {
			return nil, "", &providerGenerationError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: err.Error()}
		}
		if _, err := part.Write(editInput.Mask.Data); err != nil {
			return nil, "", &providerGenerationError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: err.Error()}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", &providerGenerationError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: err.Error()}
	}

	upstreamURL := cfg.BaseURL + "/images/edits"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, &body)
	if err != nil {
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusInternalServerError,
			Code:       "provider_request_failed",
			Message:    "create provider request failed",
		}
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: cfg.RequestTimeout}
	if hooks.BeforeUpstreamAttempt != nil {
		hooks.BeforeUpstreamAttempt()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "provider_request_failed",
			Message:    err.Error(),
		}
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxImageProviderResponseBytes))
	if err != nil {
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "provider_response_failed",
			Message:    err.Error(),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", &providerGenerationError{
			HTTPStatus:     http.StatusBadGateway,
			UpstreamStatus: resp.StatusCode,
			Code:           "provider_error",
			Message:        summarizeCPAError(responseBody),
			ResponseBody:   responseBody,
		}
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	return responseBody, contentType, nil
}

func executeOpenAICompatibleImageGeneration(ctx context.Context, cfg imageProviderProxyConfig, payload map[string]any, hooks providerImageGenerationHooks) ([]byte, string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "invalid request payload",
		}
	}

	upstreamURL := cfg.BaseURL + "/images/generations"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(raw))
	if err != nil {
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusInternalServerError,
			Code:       "provider_request_failed",
			Message:    "create provider request failed",
		}
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: cfg.RequestTimeout}
	if hooks.BeforeUpstreamAttempt != nil {
		hooks.BeforeUpstreamAttempt()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "provider_request_failed",
			Message:    err.Error(),
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageProviderResponseBytes))
	if err != nil {
		return nil, "", &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "provider_response_failed",
			Message:    err.Error(),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", &providerGenerationError{
			HTTPStatus:     http.StatusBadGateway,
			UpstreamStatus: resp.StatusCode,
			Code:           "provider_error",
			Message:        summarizeCPAError(body),
			ResponseBody:   body,
		}
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	return body, contentType, nil
}

func executeGeminiBananaImageGeneration(ctx context.Context, cfg imageProviderProxyConfig, payload map[string]any, requestedCount int, editInput providerResolvedEditInput, hooks providerImageGenerationHooks) ([]byte, error) {
	prompt := strings.TrimSpace(stringValue(payload["prompt"]))
	if prompt == "" {
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "prompt is required",
		}
	}
	if requestedCount <= 0 {
		requestedCount = 1
	}
	client := &http.Client{Timeout: cfg.RequestTimeout}
	data := make([]map[string]any, 0, requestedCount)
	var lastErr error
	for index := 0; index < requestedCount; index++ {
		items, err := executeSingleGeminiBananaImageGeneration(ctx, client, cfg, payload, prompt, editInput, hooks)
		if err != nil {
			lastErr = err
			if len(data) == 0 {
				return nil, err
			}
			break
		}
		data = append(data, items...)
		if len(data) >= requestedCount {
			break
		}
	}
	if len(data) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "provider_response_failed",
			Message:    "Gemini response did not include image data",
		}
	}
	response := map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
	}
	body, err := json.Marshal(response)
	if err != nil {
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "provider_response_failed",
			Message:    err.Error(),
		}
	}
	return body, nil
}

func executeSingleGeminiBananaImageGeneration(ctx context.Context, client *http.Client, cfg imageProviderProxyConfig, payload map[string]any, prompt string, editInput providerResolvedEditInput, hooks providerImageGenerationHooks) ([]map[string]any, error) {
	raw, err := json.Marshal(buildGeminiBananaRequestPayload(payload, prompt, cfg.Model, editInput))
	if err != nil {
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "invalid Gemini request payload",
		}
	}
	upstreamURL := cfg.BaseURL + "/models/" + url.PathEscape(normalizeGeminiBananaModel(cfg.Model)) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(raw))
	if err != nil {
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusInternalServerError,
			Code:       "provider_request_failed",
			Message:    "create Gemini request failed",
		}
	}
	req.Header.Set("x-goog-api-key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if hooks.BeforeUpstreamAttempt != nil {
		hooks.BeforeUpstreamAttempt()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "provider_request_failed",
			Message:    err.Error(),
		}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageProviderResponseBytes))
	if err != nil {
		return nil, &providerGenerationError{
			HTTPStatus: http.StatusBadGateway,
			Code:       "provider_response_failed",
			Message:    err.Error(),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &providerGenerationError{
			HTTPStatus:     http.StatusBadGateway,
			UpstreamStatus: resp.StatusCode,
			Code:           "provider_error",
			Message:        summarizeCPAError(body),
			ResponseBody:   body,
		}
	}
	items, err := parseGeminiBananaImageItems(body)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func buildGeminiBananaRequestPayload(payload map[string]any, prompt string, model string, editInput providerResolvedEditInput) map[string]any {
	parts := []map[string]any{{"text": prompt}}
	for _, image := range editInput.Images {
		if len(image.Data) == 0 {
			continue
		}
		parts = append(parts, map[string]any{
			"inlineData": map[string]any{
				"mimeType": http.DetectContentType(image.Data),
				"data":     base64.StdEncoding.EncodeToString(image.Data),
			},
		})
	}
	if editInput.Mask != nil && len(editInput.Mask.Data) > 0 {
		parts = append(parts, map[string]any{
			"text": "The next image is an edit mask. Only modify the masked area and preserve unmasked content.",
		})
		parts = append(parts, map[string]any{
			"inlineData": map[string]any{
				"mimeType": http.DetectContentType(editInput.Mask.Data),
				"data":     base64.StdEncoding.EncodeToString(editInput.Mask.Data),
			},
		})
	}
	request := map[string]any{
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": parts,
			},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"TEXT", "IMAGE"},
		},
	}
	imageConfig := buildGeminiBananaImageConfig(payload, model)
	if len(imageConfig) > 0 {
		request["generationConfig"].(map[string]any)["imageConfig"] = imageConfig
	}
	return request
}

func buildGeminiBananaImageConfig(payload map[string]any, model string) map[string]any {
	aspectRatio := geminiAspectRatioFromSize(stringValue(payload["size"]))
	if aspectRatio == "" {
		return nil
	}
	image := map[string]any{"aspectRatio": aspectRatio}
	if imageSize := geminiImageSizeFromSize(stringValue(payload["size"])); imageSize != "" && !strings.Contains(normalizeGeminiBananaModel(model), "2.5") {
		image["imageSize"] = imageSize
	}
	return image
}

func parseGeminiBananaImageItems(body []byte) ([]map[string]any, error) {
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						MimeType      string `json:"mimeType"`
						MimeTypeSnake string `json:"mime_type"`
						Data          string `json:"data"`
					} `json:"inlineData"`
					InlineDataSnake *struct {
						MimeType      string `json:"mimeType"`
						MimeTypeSnake string `json:"mime_type"`
						Data          string `json:"data"`
					} `json:"inline_data"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, &providerGenerationError{
			HTTPStatus:   http.StatusBadGateway,
			Code:         "provider_response_failed",
			Message:      "decode Gemini response: " + err.Error(),
			ResponseBody: body,
		}
	}
	textParts := make([]string, 0)
	items := make([]map[string]any, 0)
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if trimmed := strings.TrimSpace(part.Text); trimmed != "" {
				textParts = append(textParts, trimmed)
				continue
			}
			inlineData := part.InlineData
			if inlineData == nil {
				inlineData = part.InlineDataSnake
			}
			if inlineData == nil || strings.TrimSpace(inlineData.Data) == "" {
				continue
			}
			item := map[string]any{"b64_json": strings.TrimSpace(inlineData.Data)}
			mimeType := firstNonEmpty(inlineData.MimeType, inlineData.MimeTypeSnake)
			if mimeType != "" {
				item["mime_type"] = mimeType
			}
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil, &providerGenerationError{
			HTTPStatus:   http.StatusBadGateway,
			Code:         "provider_response_failed",
			Message:      "Gemini response did not include image data",
			ResponseBody: body,
		}
	}
	revisedPrompt := strings.Join(textParts, "\n")
	if revisedPrompt != "" {
		for _, item := range items {
			item["revised_prompt"] = revisedPrompt
		}
	}
	return items, nil
}

func countProviderImageItems(responseBody []byte) int {
	if len(responseBody) == 0 {
		return 0
	}
	var payload struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return 0
	}
	count := 0
	for _, item := range payload.Data {
		if strings.TrimSpace(item.URL) != "" || strings.TrimSpace(item.B64JSON) != "" {
			count++
		}
	}
	return count
}

func (s *Server) saveBusinessImageBase64(ctx context.Context, raw, userID, conversationID, generationID string, index int) (string, int64, error) {
	payload, _, err := decodeBase64ImagePayload(raw)
	if err != nil {
		return "", 0, err
	}
	if len(payload) == 0 {
		return "", 0, fmt.Errorf("image is empty")
	}
	return s.saveBusinessImageBytes(ctx, payload, userID, conversationID, generationID, index, "")
}

func (s *Server) saveBusinessImageBytes(ctx context.Context, payload []byte, userID, conversationID, generationID string, index int, nameHint string) (string, int64, error) {
	if len(payload) == 0 {
		return "", 0, fmt.Errorf("image is empty")
	}
	mimeType := http.DetectContentType(payload)
	sum := sha256.Sum256(payload)
	shaHex := hex.EncodeToString(sum[:])
	ext := extensionForImageMIME(mimeType)
	kind := sanitizeFileTokenWithFallback(strings.TrimSuffix(filepath.Base(nameHint), filepath.Ext(nameHint)), "")
	if kind == "" {
		kind = "image"
	}
	filename := fmt.Sprintf("business-%s-%s-%s-%s-%d-%x%s", sanitizeFileToken(userID), sanitizeFileToken(conversationID), sanitizeFileToken(generationID), kind, index, sum[:8], ext)
	dir := s.cfg.ResolvePath(s.cfg.Storage.ImageDir)
	if strings.TrimSpace(dir) == "" {
		dir = s.cfg.ResolvePath(defaultImageDir)
	}
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err == nil {
		s.recordBusinessImageAsset(ctx, userID, conversationID, generationID, filename, path, mimeType, int64(len(payload)), shaHex)
		return "/v1/files/image/" + filename, int64(len(payload)), nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return "", 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	s.recordBusinessImageAsset(ctx, userID, conversationID, generationID, filename, path, mimeType, int64(len(payload)), shaHex)
	return "/v1/files/image/" + filename, int64(len(payload)), nil
}

func (s *Server) recordBusinessImageAsset(ctx context.Context, userID, conversationID, generationID, fileName, path, mimeType string, sizeBytes int64, sha256Hex string) {
	store, err := businessimage.NewStore(s.cfg)
	if err != nil {
		return
	}
	defer store.Close()
	_, _ = store.SaveAsset(ctx, businessimage.Asset{
		ID:             fileName,
		UserID:         userID,
		ConversationID: conversationID,
		GenerationID:   generationID,
		FileName:       fileName,
		FilePath:       path,
		URL:            "/v1/files/image/" + fileName,
		MimeType:       mimeType,
		SizeBytes:      sizeBytes,
		SHA256:         sha256Hex,
	})
}

func decodeBase64ImagePayload(raw string) ([]byte, string, error) {
	trimmed := strings.TrimSpace(raw)
	mimeType := ""
	if strings.HasPrefix(strings.ToLower(trimmed), "data:") {
		comma := strings.Index(trimmed, ",")
		if comma < 0 {
			return nil, "", fmt.Errorf("invalid data url")
		}
		meta := trimmed[:comma]
		if !strings.Contains(strings.ToLower(meta), ";base64") {
			return nil, "", fmt.Errorf("only base64 data urls are supported")
		}
		mimeType = strings.TrimPrefix(strings.Split(meta, ";")[0], "data:")
		trimmed = trimmed[comma+1:]
	}
	payload, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		payload, err = base64.RawStdEncoding.DecodeString(trimmed)
		if err != nil {
			return nil, "", fmt.Errorf("decode image: %w", err)
		}
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(payload)
	}
	return payload, mimeType, nil
}

func extensionForImageMIME(mimeType string) string {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	switch normalized {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/png":
		return ".png"
	}
	if !strings.HasPrefix(normalized, "image/") {
		return ".png"
	}
	if extensions, err := mime.ExtensionsByType(normalized); err == nil && len(extensions) > 0 {
		return extensions[0]
	}
	return ".png"
}

func sanitizeFileToken(value string) string {
	return sanitizeFileTokenWithFallback(value, "unknown")
}

func sanitizeFileTokenWithFallback(value string, fallback string) string {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return fallback
	}
	var builder strings.Builder
	for _, r := range cleaned {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return fallback
	}
	if len(result) > 80 {
		return result[:80]
	}
	return result
}

func buildProviderImageConversationTitle(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "生成"
	}
	runes := []rune(trimmed)
	if len(runes) <= 12 {
		return "生成 · " + trimmed
	}
	return "生成 · " + string(runes[:12])
}

func (s *Server) imageProviderProxyConfig(requestedPlatform ...string) (imageProviderProxyConfig, error) {
	platform := businessproviders.NormalizePlatform(firstNonEmpty(requestedPlatform...))
	if platform == "" {
		platform = businessproviders.PlatformGPTImage
	}
	if platform != businessproviders.PlatformGPTImage && platform != businessproviders.PlatformGeminiBanana {
		return imageProviderProxyConfig{}, fmt.Errorf("unsupported provider platform %q", platform)
	}

	provider := imageProviderOpenAICompatible
	providerID := ""
	providerName := "环境变量 API"
	baseURL := normalizeProviderBaseURL(platform, os.Getenv("IMAGE_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("IMAGE_API_KEY"))
	model := strings.TrimSpace(firstNonEmpty(os.Getenv("IMAGE_MODEL"), defaultModelForProviderPlatform(platform)))
	if platform == businessproviders.PlatformGPTImage {
		provider = strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("IMAGE_PROVIDER"), imageProviderOpenAICompatible)))
	} else {
		provider = imageProviderGeminiBanana
	}
	timeoutSeconds := normalizeEnvInt("IMAGE_REQUEST_TIMEOUT_SECONDS", 180)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 180
	}

	usedDBProvider := false
	if dbProvider, ok, err := s.defaultBusinessAPIProvider(platform); err != nil {
		return imageProviderProxyConfig{}, err
	} else if ok {
		baseURL = normalizeProviderBaseURL(platform, dbProvider.BaseURL)
		apiKey = strings.TrimSpace(dbProvider.APIKey)
		model = strings.TrimSpace(firstNonEmpty(dbProvider.DefaultModel, model))
		providerID = dbProvider.ID
		providerName = dbProvider.Name
		if platform == businessproviders.PlatformGeminiBanana {
			provider = imageProviderGeminiBanana
		} else {
			provider = imageProviderOpenAICompatible
		}
		usedDBProvider = true
	}

	apiAccessPlatform := businessproviders.NormalizePlatform(s.cfg.APIAccess.Platform)
	apiAccessBaseURL := ""
	apiAccessAPIKey := ""
	if !usedDBProvider && (apiAccessPlatform == "" || apiAccessPlatform == platform) {
		apiAccessBaseURL = normalizeProviderBaseURL(platform, s.cfg.APIAccess.BaseURL)
		apiAccessAPIKey = strings.TrimSpace(s.cfg.APIAccess.APIKey)
	}
	if apiAccessBaseURL != "" || apiAccessAPIKey != "" {
		if apiAccessBaseURL == "" {
			return imageProviderProxyConfig{}, fmt.Errorf("api_access.base_url is required when api_access.api_key is configured")
		}
		if apiAccessAPIKey == "" {
			return imageProviderProxyConfig{}, fmt.Errorf("api_access.api_key is required when api_access.base_url is configured")
		}
		switch strings.ToLower(strings.TrimSpace(s.cfg.APIAccess.Platform)) {
		case "", "gpt-image":
			if platform == businessproviders.PlatformGPTImage {
				provider = imageProviderOpenAICompatible
			}
		case "gemini-banana":
			provider = imageProviderGeminiBanana
		default:
			return imageProviderProxyConfig{}, fmt.Errorf("unsupported api_access.platform %q", s.cfg.APIAccess.Platform)
		}
		baseURL = apiAccessBaseURL
		apiKey = apiAccessAPIKey
		providerID = "api_access"
		providerName = "API 接入配置"
	}

	if provider != imageProviderOpenAICompatible && provider != imageProviderGeminiBanana {
		return imageProviderProxyConfig{}, fmt.Errorf("unsupported IMAGE_PROVIDER %q", provider)
	}
	if baseURL == "" {
		return imageProviderProxyConfig{}, fmt.Errorf("api_access.base_url or IMAGE_BASE_URL is required")
	}
	if apiKey == "" {
		return imageProviderProxyConfig{}, fmt.Errorf("api_access.api_key or IMAGE_API_KEY is required")
	}
	if model == "" {
		model = defaultModelForProviderPlatform(platform)
	}

	return imageProviderProxyConfig{
		Provider:       provider,
		ProviderID:     providerID,
		ProviderName:   providerName,
		Platform:       platform,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		Model:          model,
		RequestTimeout: time.Duration(timeoutSeconds) * time.Second,
	}, nil
}

func (s *Server) defaultBusinessAPIProvider(platform string) (businessproviders.Provider, bool, error) {
	store, err := businessproviders.NewStore(s.cfg)
	if err != nil {
		return businessproviders.Provider{}, false, err
	}
	defer store.Close()
	if err := store.EnsureConfigProvider(context.Background(), s.cfg); err != nil {
		return businessproviders.Provider{}, false, err
	}
	return store.DefaultForPlatform(context.Background(), platform)
}

func defaultModelForProviderPlatform(platform string) string {
	if businessproviders.NormalizePlatform(platform) == businessproviders.PlatformGeminiBanana {
		return defaultGeminiBananaModel
	}
	return cpaFixedImageModel
}

func normalizeProviderBaseURL(platform string, value string) string {
	if businessproviders.NormalizePlatform(platform) == businessproviders.PlatformGeminiBanana {
		return normalizeGeminiBananaBaseURL(value)
	}
	return normalizeOpenAICompatibleBaseURL(value)
}

func normalizeOpenAICompatibleBaseURL(value string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return ""
	}
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed
	}
	return trimmed + "/v1"
}

func normalizeGeminiBananaBaseURL(value string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return ""
	}
	if strings.HasSuffix(trimmed, "/v1beta") || strings.HasSuffix(trimmed, "/v1") {
		return trimmed
	}
	return trimmed + "/v1beta"
}

func normalizeGeminiBananaModel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultGeminiBananaModel
	}
	return strings.TrimPrefix(trimmed, "models/")
}

func geminiAspectRatioFromSize(value string) string {
	width, height, ok := parseImageSize(value)
	if !ok {
		return ""
	}
	ratio := float64(width) / float64(height)
	candidates := []struct {
		label string
		ratio float64
	}{
		{"1:1", 1},
		{"1:4", 0.25},
		{"1:8", 0.125},
		{"2:3", float64(2) / 3},
		{"3:2", 1.5},
		{"3:4", 0.75},
		{"4:3", float64(4) / 3},
		{"4:5", 0.8},
		{"5:4", 1.25},
		{"9:16", float64(9) / 16},
		{"16:9", float64(16) / 9},
		{"21:9", float64(21) / 9},
	}
	best := candidates[0]
	bestDiff := math.Abs(ratio - best.ratio)
	for _, candidate := range candidates[1:] {
		diff := math.Abs(ratio - candidate.ratio)
		if diff < bestDiff {
			best = candidate
			bestDiff = diff
		}
	}
	return best.label
}

func geminiImageSizeFromSize(value string) string {
	width, height, ok := parseImageSize(value)
	if !ok {
		return ""
	}
	maxSide := width
	if height > maxSide {
		maxSide = height
	}
	switch {
	case maxSide >= 3000:
		return "4K"
	case maxSide >= 1800:
		return "2K"
	default:
		return "1K"
	}
}

func parseImageSize(value string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func normalizePositiveInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func normalizeEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
