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
	"imagestudio/internal/businessmodels"
	"imagestudio/internal/businesspayments"
	"imagestudio/internal/businessproviders"
	"imagestudio/internal/businesssettings"
	"imagestudio/internal/businesstracker"
	"imagestudio/internal/riskcontrol"
)

const maxImageProviderRequestBytes = 96 << 20

var imageSourceFetchClient = &http.Client{Timeout: 30 * time.Second}

const maxImageProviderResponseBytes = 80 << 20
const maxProviderImageGenerationAttempts = 2
const imageProviderOpenAICompatible = "openai_compatible"
const imageProviderGeminiBanana = "gemini_banana"
const imageProviderSourcePool = "provider_pool"
const imageProviderSourceLegacy = "legacy_provider"
const imageProviderSourceAPIAccess = "api_access"
const imageProviderSourceEnv = "environment"
const defaultGeminiBananaModel = "gemini-2.5-flash-image"

type imageProviderProxyConfig struct {
	Provider               string
	ProviderID             string
	ProviderName           string
	ProviderSource         string
	ProviderGroupID        string
	ProviderGroupMatchMode string
	ProviderGroupTags      string
	DispatchStrategy       string
	DispatchTrace          []string
	DispatchTags           []string
	Platform               string
	BaseURL                string
	APIKey                 string
	Model                  string
	RequestTimeout         time.Duration
}

type providerImageGenerateMetadata struct {
	ConversationID    string
	TurnID            string
	JobID             string
	Title             string
	Platform          string
	ModelID           string
	ModelLabel        string
	Vendor            string
	VendorLabel       string
	CreditCost        int64
	CompareBatchID    string
	CompareModelIndex int
	CompareModelCount int
	DispatchTags      []string
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
	TrustedPayload     bool
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

type providerImageGenerateJobSubmitError struct {
	result providerImageGenerateResult
	job    businessjobs.Job
}

func (e *providerImageGenerateJobSubmitError) Error() string {
	return strings.TrimSpace(e.result.ErrorMessage)
}

func newProviderImageGenerateJobSubmitError(result providerImageGenerateResult, job businessjobs.Job) error {
	return &providerImageGenerateJobSubmitError{result: result, job: job}
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
		return providerImageGenerateError(http.StatusTooManyRequests, "image_queue_full", imageBusyMessage)
	}
	if errors.Is(err, errImageAdmissionQueueTimeout) {
		return providerImageGenerateError(http.StatusGatewayTimeout, "image_queue_timeout", imageBusyMessage)
	}
	return providerImageGenerateError(http.StatusGatewayTimeout, "image_queue_cancelled", imageBusyMessage)
}

func providerImageCapacityError(code string) providerImageGenerateResult {
	return providerImageGenerateError(http.StatusTooManyRequests, code, businessJobCapacityMessage(code))
}

func riskControlProviderImageMessage(decision riskcontrol.Decision) string {
	if decision.Action == riskcontrol.ActionError {
		switch decision.InternalErrorCode {
		case riskcontrol.DecisionErrorConfig:
			return "风控配置异常，请联系管理员"
		default:
			return "风控服务暂不可用，请稍后重试"
		}
	}
	return firstNonEmpty(strings.TrimSpace(decision.Message), "内容审计命中风险规则，请调整输入后重试")
}

func riskControlProviderImageError(decision riskcontrol.Decision) providerImageGenerateResult {
	if decision.Action == riskcontrol.ActionError {
		code := firstNonEmpty(decision.InternalErrorCode, riskcontrol.DecisionErrorUnavailable)
		statusCode := http.StatusServiceUnavailable
		if code == riskcontrol.DecisionErrorConfig {
			statusCode = http.StatusInternalServerError
		}
		return providerImageGenerateError(statusCode, code, riskControlProviderImageMessage(decision))
	}
	return providerImageGenerateError(http.StatusForbidden, "risk_control_blocked", riskControlProviderImageMessage(decision))
}

func (s *Server) checkProviderImageRiskControl(ctx context.Context, userID string, metadata providerImageGenerateMetadata, providerCfg imageProviderProxyConfig, prompt string, images []riskcontrol.ModerationImage) riskcontrol.Decision {
	store, err := s.newRiskControlStore()
	if err != nil {
		return riskcontrol.ErrorDecision(err)
	}
	defer store.Close()
	service := riskcontrol.NewService(store)
	decision, err := service.Check(ctx, riskcontrol.CheckInput{
		UserID:         userID,
		JobID:          metadata.JobID,
		ConversationID: metadata.ConversationID,
		TurnID:         metadata.TurnID,
		Platform:       providerCfg.Platform,
		Model:          providerCfg.Model,
		Prompt:         prompt,
		Images:         images,
	})
	if err != nil {
		return riskcontrol.ErrorDecision(err)
	}
	return decision
}

func riskControlJobErrorCode(decision riskcontrol.Decision) string {
	if decision.Action == riskcontrol.ActionError {
		return "risk_control_error"
	}
	if decision.Action == riskcontrol.ActionAllow {
		return "risk_control_skipped"
	}
	return "risk_control_blocked"
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
		var jobSubmitErr *providerImageGenerateJobSubmitError
		if errors.As(err, &jobSubmitErr) {
			s.notifyBusinessImageJob(jobSubmitErr.job)
			jobSubmitErr.result.write(w)
			return
		}
		s.recordFailedProviderImageSubmit(r.Context(), userID, payload, err, startedAt)
		var submitErr *providerImageGenerateSubmitError
		if errors.As(err, &submitErr) {
			submitErr.result.write(w)
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "image_job_create_failed", err.Error())
		return
	}

	s.notifyBusinessImageJob(job)

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

func persistProviderConfigInPayload(payload map[string]any, providerCfg imageProviderProxyConfig) {
	if payload == nil {
		return
	}
	delete(payload, "providerSource")
	delete(payload, "providerId")
	delete(payload, "providerName")
	delete(payload, "providerGroupId")
	delete(payload, "providerGroupMatchMode")
	delete(payload, "providerGroupTags")
	delete(payload, "dispatchStrategy")
	delete(payload, "dispatchTrace")
	if providerCfg.ProviderSource != "" {
		payload["providerSource"] = providerCfg.ProviderSource
	}
	if providerCfg.Platform != "" {
		payload["platform"] = providerCfg.Platform
	}
	if providerCfg.ProviderID != "" {
		payload["providerId"] = providerCfg.ProviderID
	}
	if providerCfg.ProviderName != "" {
		payload["providerName"] = providerCfg.ProviderName
	}
	if providerCfg.ProviderGroupID != "" {
		payload["providerGroupId"] = providerCfg.ProviderGroupID
	}
	if providerCfg.ProviderGroupMatchMode != "" {
		payload["providerGroupMatchMode"] = providerCfg.ProviderGroupMatchMode
	}
	if providerCfg.ProviderGroupTags != "" {
		payload["providerGroupTags"] = providerCfg.ProviderGroupTags
	}
	if providerCfg.DispatchStrategy != "" {
		payload["dispatchStrategy"] = providerCfg.DispatchStrategy
	}
	if len(providerCfg.DispatchTrace) > 0 {
		payload["dispatchTrace"] = providerCfg.DispatchTrace
	}
	if len(providerCfg.DispatchTags) > 0 {
		setProviderDispatchTagPayload(payload, "dispatchTags", providerCfg.DispatchTags)
	}
}

func providerPayloadForConfig(payload map[string]any, providerCfg imageProviderProxyConfig) (map[string]any, map[string]any) {
	nextPayload := cloneProviderPayload(payload)
	nextPayload["model"] = providerImageRequestModel(nextPayload, providerCfg)
	persistProviderConfigInPayload(nextPayload, providerCfg)
	return nextPayload, buildProviderImageGeneratePayload(nextPayload)
}

func (s *Server) prepareProviderImageModelPayload(ctx context.Context, payload map[string]any) providerImageGenerateMetadata {
	if payload == nil {
		payload = map[string]any{}
	}
	if item, ok := s.resolveBusinessImageModel(ctx, payload); ok {
		applyBusinessImageModelPayload(payload, item)
		return providerImageGenerateMetadataFromModel(payload, item)
	}
	return extractProviderImageGenerateMetadata(payload)
}

func providerImageCreditCost(settings businesssettings.Settings, metadata providerImageGenerateMetadata, platform string, count int) int64 {
	fallbackUnitCost := businesssettings.CreditCostForPlatform(settings, platform, 1)
	return businessmodels.CreditCostForModel(businessmodels.Model{
		ID:         metadata.ModelID,
		Platform:   platform,
		CreditCost: metadata.CreditCost,
	}, count, fallbackUnitCost)
}

func (s *Server) prepareProviderImagePayload(ctx context.Context, userID string, payload map[string]any) (map[string]any, providerResolvedEditInput, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	sources := providerImageSourcesFromPayload(payload["sourceImages"])
	if len(sources) == 0 {
		return payload, providerResolvedEditInput{}, nil
	}
	sourceInput, err := s.resolveProviderSourceImages(sources, providerPayloadIsEdit(payload))
	if err != nil {
		return payload, providerResolvedEditInput{}, err
	}
	nextPayload := cloneProviderPayload(payload)
	sourceImageCapacity := len(sourceInput.Images)
	if sourceInput.Mask != nil {
		sourceImageCapacity++
	}
	sourceImages := make([]map[string]any, 0, sourceImageCapacity)
	conversationID := strings.TrimSpace(stringValue(nextPayload["conversationId"]))
	generationID := firstNonEmpty(
		strings.TrimSpace(stringValue(nextPayload["jobId"])),
		strings.TrimSpace(stringValue(nextPayload["turnId"])),
	)
	for index, image := range sourceInput.Images {
		sourceImages = append(sourceImages, s.providerEditAssetPayload(ctx, userID, conversationID, generationID, index, image))
	}
	if sourceInput.Mask != nil {
		sourceImages = append(sourceImages, s.providerEditAssetPayload(ctx, userID, conversationID, generationID, len(sourceImages), *sourceInput.Mask))
	}
	nextPayload["sourceImages"] = sourceImages
	if providerPayloadIsEdit(payload) {
		return nextPayload, sourceInput, nil
	}
	return nextPayload, providerResolvedEditInput{}, nil
}

func (s *Server) providerRiskControlImagesFromPayload(payload map[string]any) ([]riskcontrol.ModerationImage, error) {
	sources := providerImageSourcesFromPayload(payload["sourceImages"])
	if len(sources) == 0 {
		return nil, nil
	}
	images := make([]riskcontrol.ModerationImage, 0, len(sources))
	for _, source := range sources {
		if strings.EqualFold(strings.TrimSpace(source.Role), "mask") {
			continue
		}
		data, err := s.resolveProviderSourceImageBytes(source)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			continue
		}
		images = append(images, riskcontrol.ModerationImage{
			MimeType: http.DetectContentType(data),
			Data:     data,
		})
	}
	return images, nil
}

func (s *Server) resolveProviderEditInputs(payload map[string]any) (providerResolvedEditInput, error) {
	return s.resolveProviderSourceImages(providerImageSourcesFromPayload(payload["sourceImages"]), true)
}

func (s *Server) resolveProviderSourceImages(sources []providerImageSource, requireImage bool) (providerResolvedEditInput, error) {
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
	if requireImage && len(result.Images) == 0 {
		return providerResolvedEditInput{}, &providerGenerationError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "image_required",
			Message:    "编辑模式至少需要一张源图",
		}
	}
	return result, nil
}

func cloneProviderPayload(payload map[string]any) map[string]any {
	next := make(map[string]any, len(payload))
	for key, value := range payload {
		next[key] = value
	}
	return next
}

func sanitizeClientProviderSelectionPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	if strings.TrimSpace(stringValue(payload["providerSource"])) == "" &&
		strings.TrimSpace(stringValue(payload["providerId"])) == "" &&
		strings.TrimSpace(stringValue(payload["providerName"])) == "" &&
		strings.TrimSpace(stringValue(payload["providerGroupId"])) == "" &&
		strings.TrimSpace(stringValue(payload["providerGroupMatchMode"])) == "" &&
		strings.TrimSpace(stringValue(payload["providerGroupTags"])) == "" &&
		strings.TrimSpace(stringValue(payload["dispatchStrategy"])) == "" &&
		len(providerDispatchTagStrings(payload["dispatchTrace"])) == 0 {
		return payload
	}
	next := cloneProviderPayload(payload)
	delete(next, "providerSource")
	delete(next, "providerId")
	delete(next, "providerName")
	delete(next, "providerGroupId")
	delete(next, "providerGroupMatchMode")
	delete(next, "providerGroupTags")
	delete(next, "dispatchStrategy")
	delete(next, "dispatchTrace")
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
	payload = sanitizeClientProviderSelectionPayload(payload)
	metadata := s.prepareProviderImageModelPayload(ctx, payload)
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	metadata = s.enrichProviderImageDispatchTags(ctx, userID, payload, metadata, false)
	providerRequest := append([]string{metadata.Platform}, metadata.DispatchTags...)
	providerCfg, err := s.imageProviderProxyConfig(providerRequest...)
	if err != nil {
		result := providerImageGenerateError(http.StatusInternalServerError, "provider_not_configured", err.Error())
		if job, saveErr := s.saveFailedProviderImageSubmitJob(ctx, userID, payload, metadata, result, startedAt); saveErr == nil && job.ID != "" {
			return businessjobs.Job{}, newProviderImageGenerateJobSubmitError(result, job)
		}
		return businessjobs.Job{}, newProviderImageGenerateSubmitError(result)
	}
	systemSettings := s.businessSystemSettingsForContext(ctx)
	prompt := strings.TrimSpace(stringValue(payload["prompt"]))
	if prompt == "" {
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageGenerateError(http.StatusBadRequest, "invalid_request", "prompt is required"))
	}
	payload["prompt"] = prompt
	requestModel := providerImageRequestModel(payload, providerCfg)
	providerCfg.Model = requestModel
	payload["model"] = requestModel
	persistProviderConfigInPayload(payload, providerCfg)
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
	if systemSettings.Generation.MaxCount > 0 && requestedCount > systemSettings.Generation.MaxCount {
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageGenerateError(http.StatusBadRequest, "invalid_request", fmt.Sprintf("单次最多生成 %d 张图片", systemSettings.Generation.MaxCount)))
	}
	payload, _, err = s.prepareProviderImagePayload(ctx, userID, payload)
	if err != nil {
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		if providerErr, ok := err.(*providerGenerationError); ok {
			return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, providerErr.Message))
		}
		return businessjobs.Job{}, err
	}
	riskImages, err := s.providerRiskControlImagesFromPayload(payload)
	if err != nil {
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		providerErr := providerGenerationErrorDetails(err)
		return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, providerErr.Message))
	}
	if decision := s.checkProviderImageRiskControl(ctx, userID, metadata, providerCfg, prompt, riskImages); !decision.Allowed {
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		result := riskControlProviderImageError(decision)
		jobResult := result
		jobResult.ErrorCode = riskControlJobErrorCode(decision)
		if job, saveErr := s.saveFailedProviderImageSubmitJob(ctx, userID, payload, metadata, jobResult, startedAt); saveErr == nil && job.ID != "" {
			return businessjobs.Job{}, newProviderImageGenerateJobSubmitError(result, job)
		}
		return businessjobs.Job{}, newProviderImageGenerateSubmitError(result)
	}
	generationID := firstNonEmpty(metadata.JobID, metadata.TurnID)
	job := businessjobs.Job{
		ID:                metadata.JobID,
		UserID:            userID,
		ConversationID:    metadata.ConversationID,
		GenerationID:      generationID,
		TurnID:            metadata.TurnID,
		Platform:          providerCfg.Platform,
		ProviderID:        providerCfg.ProviderID,
		ProviderName:      providerCfg.ProviderName,
		Model:             requestModel,
		CompareBatchID:    metadata.CompareBatchID,
		CompareModelIndex: metadata.CompareModelIndex,
		CompareModelCount: metadata.CompareModelCount,
		Prompt:            prompt,
		Size:              strings.TrimSpace(stringValue(payload["size"])),
		Quality:           strings.TrimSpace(stringValue(payload["quality"])),
		RequestedCount:    requestedCount,
		Status:            businessjobs.StatusQueued,
		Stage:             "queued",
		CreditReserved:    providerImageCreditCost(systemSettings, metadata, providerCfg.Platform, requestedCount),
		PayloadJSON:       providerImagePayloadJSON(payload),
		CreatedAt:         startedAt.Format(time.RFC3339Nano),
		QueuedAt:          startedAt.Format(time.RFC3339Nano),
	}
	if keyID, md, ok := externalAttributionFromContext(ctx); ok {
		job.APIKeyID = keyID
		job.APIMetadata = md
	}
	store, err := s.newBusinessJobStore()
	if err != nil {
		return businessjobs.Job{}, err
	}
	defer store.Close()
	saved, err := store.SaveQueuedWithCapacity(ctx, job, businessJobCapacityLimits(systemSettings))
	if err != nil {
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		if capacityCode := businessJobCapacityErrorCode(err); capacityCode != "" {
			return businessjobs.Job{}, newProviderImageGenerateSubmitError(providerImageCapacityError(capacityCode))
		}
		return businessjobs.Job{}, err
	}
	s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, businessjobs.StatusQueued, "", startedAt)
	return saved, nil
}

func (s *Server) recordFailedProviderImageSubmit(ctx context.Context, userID string, payload map[string]any, err error, startedAt time.Time) {
	if payload == nil {
		return
	}
	metadata := s.prepareProviderImageModelPayload(ctx, payload)
	if strings.TrimSpace(metadata.ConversationID) == "" || strings.TrimSpace(metadata.TurnID) == "" {
		return
	}
	prompt := strings.TrimSpace(stringValue(payload["prompt"]))
	if prompt == "" {
		return
	}
	providerCfg := imageProviderProxyConfig{
		Platform: businessproviders.NormalizePlatform(metadata.Platform),
		Model:    strings.TrimSpace(stringValue(payload["model"])),
	}
	if providerCfg.Platform == "" {
		if item, ok := s.resolveBusinessImageModel(ctx, payload); ok {
			providerCfg.Platform = item.Platform
		}
	}
	message := "提交任务失败"
	var submitErr *providerImageGenerateSubmitError
	if errors.As(err, &submitErr) {
		message = providerImageGenerationFailureReason(submitErr.result, message)
	} else if err != nil {
		message = firstNonEmpty(strings.TrimSpace(err.Error()), message)
	}
	s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", message, startedAt)
}

func providerImageGenerationFailureReason(result providerImageGenerateResult, fallback string) string {
	code := strings.TrimSpace(result.ErrorCode)
	if strings.HasPrefix(code, "risk_control_") {
		return code
	}
	return firstNonEmpty(strings.TrimSpace(result.ErrorMessage), fallback)
}

func (s *Server) saveFailedProviderImageSubmitJob(ctx context.Context, userID string, payload map[string]any, metadata providerImageGenerateMetadata, result providerImageGenerateResult, startedAt time.Time) (businessjobs.Job, error) {
	if payload == nil {
		return businessjobs.Job{}, fmt.Errorf("payload is required")
	}
	if strings.TrimSpace(metadata.JobID) == "" {
		metadata.JobID = businessjobs.NewJobID()
		payload["jobId"] = metadata.JobID
	}
	if strings.TrimSpace(metadata.TurnID) == "" {
		metadata.TurnID = metadata.JobID
		payload["turnId"] = metadata.TurnID
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	platform := businessproviders.NormalizePlatform(metadata.Platform)
	if platform == "" {
		if item, ok := s.resolveBusinessImageModel(ctx, payload); ok {
			applyBusinessImageModelPayload(payload, item)
			platform = item.Platform
			metadata = providerImageGenerateMetadataFromModel(payload, item)
		} else {
			platform = businessproviders.PlatformGPTImage
		}
	}
	prompt := strings.TrimSpace(stringValue(payload["prompt"]))
	requestedCount := normalizePositiveInt(payload["n"])
	if requestedCount <= 0 {
		requestedCount = 1
	}
	finishedAt := time.Now().UTC()
	job := businessjobs.Job{
		ID:                metadata.JobID,
		UserID:            userID,
		ConversationID:    metadata.ConversationID,
		GenerationID:      firstNonEmpty(metadata.JobID, metadata.TurnID),
		TurnID:            metadata.TurnID,
		Platform:          platform,
		Model:             strings.TrimSpace(stringValue(payload["model"])),
		CompareBatchID:    metadata.CompareBatchID,
		CompareModelIndex: metadata.CompareModelIndex,
		CompareModelCount: metadata.CompareModelCount,
		Prompt:            prompt,
		Size:              strings.TrimSpace(stringValue(payload["size"])),
		Quality:           strings.TrimSpace(stringValue(payload["quality"])),
		RequestedCount:    requestedCount,
		Status:            businessjobs.StatusFailed,
		Stage:             "dispatch",
		ErrorCode:         strings.TrimSpace(result.ErrorCode),
		ErrorMessage:      strings.TrimSpace(result.ErrorMessage),
		PayloadJSON:       providerImagePayloadJSON(payload),
		CreatedAt:         startedAt.Format(time.RFC3339Nano),
		QueuedAt:          startedAt.Format(time.RFC3339Nano),
		FinishedAt:        finishedAt.Format(time.RFC3339Nano),
		TotalDurationMS:   finishedAt.Sub(startedAt).Milliseconds(),
	}
	store, err := s.newBusinessJobStore()
	if err != nil {
		return businessjobs.Job{}, err
	}
	defer store.Close()
	saved, err := store.Save(ctx, job)
	if err != nil {
		return businessjobs.Job{}, err
	}
	s.recordFailedProviderImageSubmit(ctx, userID, payload, newProviderImageGenerateSubmitError(result), startedAt)
	s.saveBusinessImageTracker(context.Background(), businesstracker.Record{
		ID:              businesstracker.NewRecordID(),
		UserID:          userID,
		Platform:        platform,
		Model:           job.Model,
		Status:          businesstracker.StatusFailed,
		Stage:           job.Stage,
		ErrorCode:       job.ErrorCode,
		ErrorMessage:    job.ErrorMessage,
		CreatedAt:       startedAt.Format(time.RFC3339Nano),
		FinishedAt:      finishedAt.Format(time.RFC3339Nano),
		TotalDurationMS: job.TotalDurationMS,
	})
	return saved, nil
}

func (s *Server) runProviderImageGenerateJob(userID string, payload map[string]any, startedAt time.Time) {
	s.executeProviderImageGenerate(providerImageGenerateExecution{
		Context:        context.Background(),
		UserID:         userID,
		Payload:        payload,
		StartedAt:      startedAt,
		TrustedPayload: true,
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
	if !execution.TrustedPayload {
		execution.Payload = sanitizeClientProviderSelectionPayload(execution.Payload)
	}
	userID := strings.TrimSpace(execution.UserID)
	payload := execution.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	metadata := s.prepareProviderImageModelPayload(ctx, payload)
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

	metadata = s.enrichProviderImageDispatchTags(ctx, userID, payload, metadata, execution.TrustedPayload)
	tracker.ConversationID = metadata.ConversationID
	tracker.TurnID = metadata.TurnID
	tracker.GenerationID = firstNonEmpty(metadata.JobID, metadata.TurnID)
	tracker.Platform = businessproviders.NormalizePlatform(metadata.Platform)
	job := businessjobs.Job{
		ID:                firstNonEmpty(metadata.JobID, businessjobs.NewJobID()),
		UserID:            userID,
		ConversationID:    metadata.ConversationID,
		GenerationID:      firstNonEmpty(metadata.JobID, metadata.TurnID),
		TurnID:            metadata.TurnID,
		Platform:          tracker.Platform,
		CompareBatchID:    metadata.CompareBatchID,
		CompareModelIndex: metadata.CompareModelIndex,
		CompareModelCount: metadata.CompareModelCount,
		Status:            businessjobs.StatusQueued,
		Stage:             "received",
		PayloadJSON:       providerImagePayloadJSON(payload),
		CreatedAt:         startedAt.Format(time.RFC3339Nano),
		QueuedAt:          startedAt.Format(time.RFC3339Nano),
	}
	jobCtx, cancelJobContext := context.WithCancel(ctx)
	unregisterJob := s.registerActiveBusinessImageJob(job.ID, cancelJobContext)
	defer unregisterJob()
	defer cancelJobContext()

	saveJob := func(next businessjobs.Job) {
		store, err := s.newBusinessJobStore()
		if err != nil {
			return
		}
		defer store.Close()
		if current, ok, getErr := store.Get(context.Background(), next.ID, next.UserID); getErr == nil && ok {
			if current.CreditRefunded > next.CreditRefunded {
				next.CreditRefunded = current.CreditRefunded
			}
			if next.ClaimedBy == "" {
				next.ClaimedBy = current.ClaimedBy
			}
			if next.ClaimedAt == "" {
				next.ClaimedAt = current.ClaimedAt
			}
			if next.Attempts < current.Attempts {
				next.Attempts = current.Attempts
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
	saveRunningJob := func(next businessjobs.Job, settings businesssettings.Settings) error {
		store, err := s.newBusinessJobStore()
		if err != nil {
			return err
		}
		defer store.Close()
		saved, err := store.SaveRunningWithProviderCapacity(context.Background(), next, businessJobCapacityLimits(settings))
		if err == nil {
			job = saved
		}
		return err
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
	providerCfg, err := s.imageProviderProxyConfigFromPayload(payload)
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
	blockRiskControl := func(decision riskcontrol.Decision) providerImageGenerateResult {
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		result := riskControlProviderImageError(decision)
		jobErrorCode := riskControlJobErrorCode(decision)
		job.PayloadJSON = providerImagePayloadJSON(payload)
		saveJob(job)
		s.recordProviderImageGeneration(context.Background(), userID, metadata, payload, nil, "failed", jobErrorCode, startedAt)
		finishJob(businessjobs.StatusFailed, "risk_control", jobErrorCode, result.ErrorMessage)
		finishTracker(businesstracker.StatusFailed, "risk_control", jobErrorCode, result.ErrorMessage)
		return result
	}
	prompt := strings.TrimSpace(stringValue(payload["prompt"]))
	if prompt == "" {
		job.Prompt = prompt
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		finishJob(businessjobs.StatusFailed, "validation", "invalid_request", "prompt is required")
		finishTracker(businesstracker.StatusFailed, "validation", "invalid_request", "prompt is required")
		return providerImageGenerateError(http.StatusBadRequest, "invalid_request", "prompt is required")
	}
	payload["prompt"] = prompt
	requestModel := providerImageRequestModel(payload, providerCfg)
	providerCfg.Model = requestModel
	payload["model"] = requestModel
	persistProviderConfigInPayload(payload, providerCfg)
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
		job.CreditReserved = providerImageCreditCost(systemSettings, metadata, providerCfg.Platform, requestedCount)
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		finishJob(businessjobs.StatusFailed, "validation", "invalid_request", fmt.Sprintf("单次最多生成 %d 张图片", systemSettings.Generation.MaxCount))
		finishTracker(businesstracker.StatusFailed, "validation", "invalid_request", fmt.Sprintf("单次最多生成 %d 张图片", systemSettings.Generation.MaxCount))
		return providerImageGenerateError(http.StatusBadRequest, "invalid_request", fmt.Sprintf("单次最多生成 %d 张图片", systemSettings.Generation.MaxCount))
	}
	var editInput providerResolvedEditInput
	payload, editInput, err = s.prepareProviderImagePayload(ctx, userID, payload)
	if err != nil {
		job.PayloadJSON = providerImagePayloadJSON(payload)
		providerErr := providerGenerationErrorDetails(err)
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		s.recordProviderImageGeneration(ctx, userID, metadata, payload, providerErr.ResponseBody, "failed", providerErr.Message, startedAt)
		finishJob(businessjobs.StatusFailed, "validation", providerErr.Code, providerErr.Message)
		finishTracker(businesstracker.StatusFailed, "validation", providerErr.Code, providerErr.Message)
		return providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, providerErr.Message)
	}
	if !execution.TrustedPayload {
		riskImages, err := s.providerRiskControlImagesFromPayload(payload)
		if err != nil {
			providerErr := providerGenerationErrorDetails(err)
			s.reportProviderPoolRelease(context.Background(), providerCfg)
			s.recordProviderImageGeneration(ctx, userID, metadata, payload, providerErr.ResponseBody, "failed", providerErr.Message, startedAt)
			finishJob(businessjobs.StatusFailed, "validation", providerErr.Code, providerErr.Message)
			finishTracker(businesstracker.StatusFailed, "validation", providerErr.Code, providerErr.Message)
			return providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, providerErr.Message)
		}
		if decision := s.checkProviderImageRiskControl(ctx, userID, metadata, providerCfg, prompt, riskImages); !decision.Allowed {
			return blockRiskControl(decision)
		}
	}
	creditCost := providerImageCreditCost(systemSettings, metadata, providerCfg.Platform, requestedCount)
	generationID := firstNonEmpty(metadata.JobID, metadata.TurnID)
	tracker.RequestedCount = requestedCount
	tracker.CreditReserved = creditCost
	job.GenerationID = generationID
	job.RequestedCount = requestedCount
	job.CreditReserved = creditCost
	if errors.Is(jobCtx.Err(), context.Canceled) {
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		cancelGeneration("任务已取消")
		return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
	}
	if latestJob, ok := s.businessImageJobByID(job.ID, userID); ok && businessJobCancelled(latestJob) {
		job = latestJob
		s.reportProviderPoolRelease(context.Background(), providerCfg)
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
			s.reportProviderPoolRelease(context.Background(), providerCfg)
			s.recordProviderImageGenerationPlaceholder(context.Background(), userID, metadata, payload, providerCfg, "cancelled", "任务已取消", startedAt)
			finishJob(businessjobs.StatusCancelled, "cancelled", "cancelled", "任务已取消")
			finishTracker(businesstracker.StatusFailed, "cancelled", "cancelled", "任务已取消")
			return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
		}
		s.reportProviderPoolRelease(context.Background(), providerCfg)
		s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", imageBusyMessage, startedAt)
		finishJob(businessjobs.StatusFailed, "admission", imageAdmissionErrorCode(admissionErr), imageBusyMessage)
		finishTracker(businesstracker.StatusFailed, "admission", imageAdmissionErrorCode(admissionErr), imageBusyMessage)
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
	job.LeaseUntil = time.Now().UTC().Add(businessImageJobRunningLeaseDuration).Format(time.RFC3339Nano)
	if err := saveRunningJob(job, systemSettings); err != nil {
		if capacityCode := businessJobCapacityErrorCode(err); capacityCode != "" {
			message := businessJobCapacityMessage(capacityCode)
			s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", message, startedAt)
			finishJob(businessjobs.StatusFailed, "provider_capacity", capacityCode, message)
			finishTracker(businesstracker.StatusFailed, "provider_capacity", capacityCode, message)
			return providerImageCapacityError(capacityCode)
		}
		s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", err.Error(), startedAt)
		finishJob(businessjobs.StatusFailed, "provider_capacity", "provider_capacity_check_failed", err.Error())
		finishTracker(businesstracker.StatusFailed, "provider_capacity", "provider_capacity_check_failed", err.Error())
		return providerImageGenerateError(http.StatusInternalServerError, "provider_capacity_check_failed", err.Error())
	}
	stopHeartbeat := s.startBusinessImageJobHeartbeat(jobCtx, job.ID, userID)
	defer stopHeartbeat()
	if execution.AfterRunningMarked != nil {
		execution.AfterRunningMarked()
	}
	if latestJob, ok := s.businessImageJobByID(job.ID, userID); ok && businessJobCancelled(latestJob) {
		job = latestJob
		cancelGeneration("任务已取消")
		return providerImageGenerateError(http.StatusConflict, "cancelled", "任务已取消")
	}
	s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "running", "", startedAt)

	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", "credit store failed", startedAt)
		finishJob(businessjobs.StatusFailed, "credit", "credit_store_failed", "credit store failed")
		finishTracker(businesstracker.StatusFailed, "credit", "credit_store_failed", "credit store failed")
		return providerImageGenerateError(http.StatusInternalServerError, "credit_store_failed", "credit store failed")
	}
	defer creditStore.Close()
	paymentStore, err := s.newBusinessPaymentStore()
	if err != nil {
		s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", "payment store failed", startedAt)
		finishJob(businessjobs.StatusFailed, "credit", "payment_store_failed", "payment store failed")
		finishTracker(businesstracker.StatusFailed, "credit", "payment_store_failed", "payment store failed")
		return providerImageGenerateError(http.StatusInternalServerError, "payment_store_failed", "payment store failed")
	}
	defer paymentStore.Close()
	subscriptionReserved := int64(0)
	if subscriptionReserve, err := paymentStore.ReserveSubscriptionCredits(ctx, userID, creditCost, generationID); err != nil {
		s.recordProviderImageGenerationPlaceholder(ctx, userID, metadata, payload, providerCfg, "failed", err.Error(), startedAt)
		finishJob(businessjobs.StatusFailed, "credit", "subscription_credit_reserve_failed", err.Error())
		finishTracker(businesstracker.StatusFailed, "credit", "subscription_credit_reserve_failed", err.Error())
		return providerImageGenerateError(http.StatusInternalServerError, "subscription_credit_reserve_failed", err.Error())
	} else {
		subscriptionReserved = subscriptionReserve.Reserved
	}
	balanceReserved := creditCost - subscriptionReserved
	if balanceReserved > 0 {
		if _, _, err := creditStore.Reserve(ctx, userID, balanceReserved, generationID); err != nil {
			if subscriptionReserved > 0 {
				_, _ = paymentStore.RefundSubscriptionCredits(context.Background(), userID, subscriptionReserved, generationID)
			}
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
	}
	creditsSettled := false
	var refundedCredits int64
	syncRefundedCredits := func() int64 {
		balanceTotals, err := creditStore.GenerationTotals(context.Background(), userID, generationID)
		if err != nil {
			balanceTotals = businesscredits.GenerationTotals{}
		}
		subscriptionTotals, err := paymentStore.SubscriptionGenerationTotals(context.Background(), userID, generationID)
		if err != nil {
			subscriptionTotals = businesspayments.SubscriptionTotals{}
		}
		totalRefunded := balanceTotals.Refunded + subscriptionTotals.Refunded
		if totalRefunded > refundedCredits {
			refundedCredits = totalRefunded
			tracker.CreditRefunded = refundedCredits
			job.CreditRefunded = refundedCredits
		}
		return totalRefunded
	}
	syncReservedCredits := func() int64 {
		balanceTotals, err := creditStore.GenerationTotals(context.Background(), userID, generationID)
		if err != nil {
			balanceTotals = businesscredits.GenerationTotals{}
		}
		subscriptionTotals, err := paymentStore.SubscriptionGenerationTotals(context.Background(), userID, generationID)
		if err != nil {
			subscriptionTotals = businesspayments.SubscriptionTotals{}
		}
		totalReserved := balanceTotals.Reserved + subscriptionTotals.Reserved
		syncRefundedCredits()
		tracker.CreditReserved = totalReserved
		job.CreditReserved = totalReserved
		return totalReserved
	}
	refundReservedCredits := func(amount int64) {
		if amount <= 0 {
			return
		}
		balanceTotals, _ := creditStore.GenerationTotals(context.Background(), userID, generationID)
		subscriptionTotals, _ := paymentStore.SubscriptionGenerationTotals(context.Background(), userID, generationID)
		totalReserved := balanceTotals.Reserved + subscriptionTotals.Reserved
		totalRefunded := balanceTotals.Refunded + subscriptionTotals.Refunded
		if totalReserved <= totalRefunded {
			syncRefundedCredits()
			return
		}
		refundable := totalReserved - totalRefunded
		if amount > refundable {
			amount = refundable
		}
		remaining := amount
		balanceRefundable := balanceTotals.Reserved - balanceTotals.Refunded
		if balanceRefundable > 0 && remaining > 0 {
			refundAmount := remaining
			if refundAmount > balanceRefundable {
				refundAmount = balanceRefundable
			}
			if _, _, err := creditStore.Refund(context.Background(), userID, refundAmount, generationID); err == nil {
				remaining -= refundAmount
			}
		}
		subscriptionRefundable := subscriptionTotals.Reserved - subscriptionTotals.Refunded
		if subscriptionRefundable > 0 && remaining > 0 {
			refundAmount := remaining
			if refundAmount > subscriptionRefundable {
				refundAmount = subscriptionRefundable
			}
			if _, err := paymentStore.RefundSubscriptionCredits(context.Background(), userID, refundAmount, generationID); err == nil {
				remaining -= refundAmount
			}
		}
		syncRefundedCredits()
	}
	refundCredits := func(amount int64) {
		if !systemSettings.Billing.RefundOnFailure {
			syncRefundedCredits()
			return
		}
		refundReservedCredits(amount)
	}
	reserveAdditionalCredits := func(amount int64) error {
		if amount <= 0 {
			syncReservedCredits()
			return nil
		}
		subscriptionReserve, err := paymentStore.ReserveSubscriptionCredits(context.Background(), userID, amount, generationID)
		if err != nil {
			syncReservedCredits()
			return err
		}
		remaining := amount - subscriptionReserve.Reserved
		if remaining > 0 {
			if _, _, err := creditStore.Reserve(context.Background(), userID, remaining, generationID); err != nil {
				if subscriptionReserve.Reserved > 0 {
					_, _ = paymentStore.RefundSubscriptionCredits(context.Background(), userID, subscriptionReserve.Reserved, generationID)
				}
				syncReservedCredits()
				return err
			}
		}
		syncReservedCredits()
		return nil
	}
	if strings.TrimSpace(stringValue(payload["response_format"])) == "" {
		payload["response_format"] = "b64_json"
	}
	job.PayloadJSON = providerImagePayloadJSON(payload)
	saveJob(job)
	var providerPayload map[string]any
	payload, providerPayload = providerPayloadForConfig(payload, providerCfg)
	job.PayloadJSON = providerImagePayloadJSON(payload)
	saveJob(job)

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
	poolFailureReported := false
	body, contentType, err = executeProviderImageGenerationWithRetry(ctx, providerCfg, providerPayload, requestedCount, editInput, providerImageGenerationHooks{
		BeforeUpstreamAttempt: markUpstreamStarted,
	})
	if err != nil && providerCfg.ProviderSource == imageProviderSourcePool && strings.TrimSpace(providerCfg.ProviderID) != "" && ctx.Err() == nil {
		providerErr := providerGenerationErrorDetails(err)
		s.reportProviderPoolFailure(context.Background(), providerCfg, providerErr)
		poolFailureReported = true
		if fallbackCfg, ok, fallbackErr := s.imageProviderProxyFallbackConfig(providerCfg.Platform); fallbackErr != nil {
			err = fallbackErr
		} else if ok {
			fallbackCfg.DispatchTags = providerCfg.DispatchTags
			fallbackCfg.DispatchStrategy = "api_fallback"
			fallbackCfg.DispatchTrace = appendProviderDispatchTrace(
				providerCfg,
				"号池成员失败："+firstNonEmpty(providerCfg.ProviderName, providerCfg.ProviderID),
				"回退 API 接入："+firstNonEmpty(fallbackCfg.ProviderName, fallbackCfg.ProviderSource),
			).DispatchTrace
			providerCfg = fallbackCfg
			tracker.ProviderID = providerCfg.ProviderID
			tracker.ProviderName = providerCfg.ProviderName
			tracker.Platform = providerCfg.Platform
			job.ProviderID = providerCfg.ProviderID
			job.ProviderName = providerCfg.ProviderName
			job.Platform = providerCfg.Platform
			requestModel = providerImageRequestModel(payload, providerCfg)
			providerCfg.Model = requestModel
			payload["model"] = requestModel
			tracker.Model = requestModel
			job.Model = requestModel
			payload, providerPayload = providerPayloadForConfig(payload, providerCfg)
			job.PayloadJSON = providerImagePayloadJSON(payload)
			saveJob(job)
			body, contentType, err = executeProviderImageGenerationWithRetry(ctx, providerCfg, providerPayload, requestedCount, editInput, providerImageGenerationHooks{
				BeforeUpstreamAttempt: markUpstreamStarted,
			})
		} else {
			err = &providerGenerationError{
				HTTPStatus: http.StatusBadGateway,
				Code:       "provider_pool_failed_no_fallback",
				Message:    "号池上游请求失败，且当前未配置 API 接入兜底",
			}
		}
	}
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
		if !poolFailureReported {
			s.reportProviderPoolFailure(context.Background(), providerCfg, providerErr)
		}
		setProviderFailurePayloadMetadata(payload, providerErr)
		job.PayloadJSON = providerImagePayloadJSON(payload)
		refundCredits(creditCost)
		job.CreditRefunded = refundedCredits
		s.recordProviderImageGeneration(ctx, userID, metadata, providerPayload, providerErr.ResponseBody, "failed", errorMessage, startedAt)
		finishJob(businessjobs.StatusFailed, "upstream", providerErr.Code, errorMessage)
		finishTracker(businesstracker.StatusFailed, "upstream", providerErr.Code, errorMessage)
		return providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, errorMessage)
	}
	actualCount := countProviderImageItems(body)
	maxReturnCount := systemSettings.Generation.MaxCount
	if maxReturnCount > 0 && actualCount > maxReturnCount {
		body = limitProviderImageResponseItems(body, maxReturnCount)
		actualCount = countProviderImageItems(body)
	}
	if actualCount <= 0 {
		providerErr := providerGenerationError{
			HTTPStatus:     http.StatusBadGateway,
			UpstreamStatus: http.StatusOK,
			Code:           "provider_empty_response",
			Message:        "上游接口返回成功，但没有返回可用图片数据",
			ResponseBody:   body,
		}
		s.reportProviderPoolFailure(context.Background(), providerCfg, providerErr)
		setProviderFailurePayloadMetadata(payload, providerErr)
		job.PayloadJSON = providerImagePayloadJSON(payload)
		refundCredits(creditCost)
		job.CreditRefunded = refundedCredits
		s.recordProviderImageGeneration(ctx, userID, metadata, providerPayload, providerErr.ResponseBody, "failed", providerErr.Message, startedAt)
		finishJob(businessjobs.StatusFailed, "upstream", providerErr.Code, providerErr.Message)
		finishTracker(businesstracker.StatusFailed, "upstream", providerErr.Code, providerErr.Message)
		return providerImageGenerateError(providerErr.HTTPStatus, providerErr.Code, providerErr.Message)
	}
	if actualCount < requestedCount && systemSettings.Billing.RefundPartialCount {
		actualCost := providerImageCreditCost(systemSettings, metadata, providerCfg.Platform, actualCount)
		refundCredits(creditCost - actualCost)
	}
	if actualCount > requestedCount {
		billedCount := requestedCount
		for billedCount < actualCount {
			currentCost := providerImageCreditCost(systemSettings, metadata, providerCfg.Platform, billedCount)
			nextCost := providerImageCreditCost(systemSettings, metadata, providerCfg.Platform, billedCount+1)
			if err := reserveAdditionalCredits(nextCost - currentCost); err != nil {
				if errors.Is(err, businesscredits.ErrInsufficientBalance) {
					body = limitProviderImageResponseItems(body, billedCount)
					actualCount = countProviderImageItems(body)
					break
				}
				refundCredits(syncReservedCredits())
				job.CreditRefunded = refundedCredits
				s.recordProviderImageGeneration(ctx, userID, metadata, providerPayload, nil, "failed", err.Error(), startedAt)
				finishJob(businessjobs.StatusFailed, "credit", "credit_reserve_failed", err.Error())
				finishTracker(businesstracker.StatusFailed, "credit", "credit_reserve_failed", err.Error())
				return providerImageGenerateError(http.StatusInternalServerError, "credit_reserve_failed", err.Error())
			}
			billedCount++
		}
	}
	tracker.ActualCount = actualCount
	job.ActualCount = actualCount
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
	syncReservedCredits()
	job.CreditRefunded = refundedCredits
	s.reportProviderPoolSuccess(context.Background(), providerCfg)
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
	store, err := s.newBusinessImageStore()
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
	store, err := s.newBusinessTrackerStore()
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
	model := strings.TrimSpace(stringValue(payload["model"]))
	if model != "" {
		return model
	}
	if providerCfg.Provider == imageProviderGeminiBanana {
		model = strings.TrimSpace(providerCfg.Model)
		if model == "" {
			return defaultGeminiBananaModel
		}
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
		ConversationID:    strings.TrimSpace(stringValue(payload["conversationId"])),
		TurnID:            strings.TrimSpace(stringValue(payload["turnId"])),
		JobID:             strings.TrimSpace(stringValue(payload["jobId"])),
		Title:             strings.TrimSpace(stringValue(payload["title"])),
		Platform:          strings.TrimSpace(stringValue(payload["platform"])),
		ModelID:           strings.TrimSpace(stringValue(payload["modelId"])),
		ModelLabel:        strings.TrimSpace(stringValue(payload["modelLabel"])),
		Vendor:            strings.TrimSpace(stringValue(payload["vendor"])),
		VendorLabel:       strings.TrimSpace(stringValue(payload["vendorLabel"])),
		CreditCost:        normalizeNonNegativeInt64(payload["creditCost"]),
		CompareBatchID:    strings.TrimSpace(firstNonEmpty(stringValue(payload["compareBatchId"]), stringValue(payload["compareGroupId"]))),
		CompareModelIndex: normalizeOptionalNonNegativeInt(payload["compareModelIndex"]),
		CompareModelCount: normalizeOptionalNonNegativeInt(payload["compareModelCount"]),
		DispatchTags:      extractProviderDispatchTags(payload),
	}
}

func extractProviderDispatchTags(payload map[string]any) []string {
	tags := make([]string, 0, 8)
	appendTag := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		for _, existing := range tags {
			if existing == value {
				return
			}
		}
		tags = append(tags, value)
	}
	if rawTags, ok := payload["dispatchTags"]; ok {
		for _, raw := range providerDispatchTagStrings(rawTags) {
			appendTag(raw)
		}
	}
	for _, key := range []string{"mode", "quality", "size", "model", "modelId", "vendor", "adapter"} {
		value := strings.TrimSpace(stringValue(payload[key]))
		if value != "" {
			appendTag(key + ":" + value)
		}
	}
	return tags
}

func providerDispatchTagStrings(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case []string:
		return typed
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, stringValue(item))
		}
		return items
	case string:
		return strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\t' || r == ' ' || r == '，'
		})
	default:
		return []string{stringValue(typed)}
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
		if (key == "sourceImages" || key == "sourceReference") && !providerPayloadIsEdit(payload) {
			continue
		}
		next[key] = value
	}
	return next
}

func providerPayloadIsEdit(payload map[string]any) bool {
	return strings.EqualFold(strings.TrimSpace(stringValue(payload["mode"])), "edit")
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
	resp, err := imageSourceFetchClient.Get(rawURL)
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
	store, err := s.newBusinessImageStore()
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
	persistedResponse = injectProviderImageResponseModelMetadata(persistedResponse, metadata)
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

func base64ImageFromProviderItem(item map[string]any) string {
	if b64 := strings.TrimSpace(stringValue(item["b64_json"])); b64 != "" {
		return b64
	}
	if u := strings.TrimSpace(stringValue(item["url"])); strings.HasPrefix(strings.ToLower(u), "data:") {
		return u
	}
	return ""
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
		b64 := base64ImageFromProviderItem(item)
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

func injectProviderImageResponseModelMetadata(responseBody []byte, metadata providerImageGenerateMetadata) []byte {
	if len(responseBody) == 0 {
		responseBody = []byte(`{}`)
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return responseBody
	}
	setString := func(key string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			payload[key] = value
		}
	}
	setString("modelId", metadata.ModelID)
	setString("modelLabel", metadata.ModelLabel)
	setString("vendor", metadata.Vendor)
	setString("vendorLabel", metadata.VendorLabel)
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

func setProviderFailurePayloadMetadata(payload map[string]any, providerErr providerGenerationError) {
	if payload == nil {
		return
	}
	if code := strings.TrimSpace(providerErr.Code); code != "" {
		payload["upstreamErrorCode"] = code
	}
	if providerErr.UpstreamStatus > 0 {
		payload["upstreamStatusCode"] = providerErr.UpstreamStatus
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

func limitProviderImageResponseItems(responseBody []byte, limit int) []byte {
	if limit < 0 {
		limit = 0
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return responseBody
	}
	rawItems, ok := payload["data"].([]any)
	if !ok || len(rawItems) <= limit {
		return responseBody
	}
	payload["data"] = rawItems[:limit]
	nextBody, err := json.Marshal(payload)
	if err != nil {
		return responseBody
	}
	return nextBody
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
	store, err := s.newBusinessImageStore()
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
	dispatchTags := []string(nil)
	if len(requestedPlatform) > 1 {
		dispatchTags = mergeProviderDispatchTags(requestedPlatform[1:])
	}
	platformValue := firstNonEmpty(requestedPlatform...)
	if len(requestedPlatform) > 0 {
		platformValue = requestedPlatform[0]
	}
	platform := businessproviders.NormalizePlatform(platformValue)
	if platform == "" {
		platform = businessproviders.PlatformGPTImage
	}
	if !businessproviders.IsSupportedPlatform(platform) {
		return imageProviderProxyConfig{}, fmt.Errorf("unsupported provider platform %q", platform)
	}
	timeout := s.imageProviderRequestTimeout()

	if poolCfg, ok, err := s.defaultBusinessProviderPoolMember(platform, dispatchTags, timeout); err != nil {
		return imageProviderProxyConfig{}, err
	} else if ok {
		return poolCfg, nil
	}
	poolConfigured, err := s.businessProviderPoolConfiguredForPlatform(platform)
	if err != nil {
		return imageProviderProxyConfig{}, err
	}

	providerCfg, ok, err := s.imageProviderProxyConfigWithoutPool(platform, timeout)
	if err != nil {
		return imageProviderProxyConfig{}, err
	}
	if !ok {
		if poolConfigured {
			return imageProviderProxyConfig{}, fmt.Errorf("当前平台没有可用号池成员，请恢复或启用 active 成员，或配置 API 接入作为兜底")
		}
		return imageProviderProxyConfig{}, fmt.Errorf("api_access.base_url or IMAGE_BASE_URL is required")
	}
	providerCfg.DispatchTags = dispatchTags
	if poolConfigured {
		providerCfg.DispatchStrategy = "api_fallback"
	} else {
		providerCfg.DispatchStrategy = "api_access"
	}
	providerCfg.DispatchTrace = s.providerConfigDispatchTrace(platform, dispatchTags, providerCfg)
	return providerCfg, nil
}

func (s *Server) imageProviderProxyFallbackConfig(platform string) (imageProviderProxyConfig, bool, error) {
	platform = businessproviders.NormalizePlatform(platform)
	if platform == "" {
		platform = businessproviders.PlatformGPTImage
	}
	if !businessproviders.IsSupportedPlatform(platform) {
		return imageProviderProxyConfig{}, false, fmt.Errorf("unsupported provider platform %q", platform)
	}
	cfg, ok, err := s.imageProviderProxyConfigWithoutPool(platform, s.imageProviderRequestTimeout())
	if err != nil || !ok {
		return cfg, ok, err
	}
	cfg.DispatchStrategy = "api_fallback"
	cfg.DispatchTrace = s.providerConfigDispatchTrace(platform, cfg.DispatchTags, cfg)
	return cfg, true, nil
}

func (s *Server) imageProviderProxyConfigWithoutPool(platform string, timeout time.Duration) (imageProviderProxyConfig, bool, error) {
	provider := imageProviderOpenAICompatible
	providerID := ""
	providerName := "环境变量 API"
	providerSource := imageProviderSourceEnv
	baseURL := normalizeProviderBaseURL(platform, os.Getenv("IMAGE_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("IMAGE_API_KEY"))
	model := strings.TrimSpace(firstNonEmpty(os.Getenv("IMAGE_MODEL"), defaultModelForProviderPlatform(platform)))
	if platform == businessproviders.PlatformGPTImage {
		provider = strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("IMAGE_PROVIDER"), imageProviderOpenAICompatible)))
	} else if mappedProvider := imageProviderAdapterForPlatform(platform); mappedProvider != "" {
		provider = mappedProvider
	} else {
		provider = imageProviderGeminiBanana
	}

	usedDBProvider := false
	if dbProvider, ok, err := s.defaultBusinessAPIProvider(platform); err != nil {
		return imageProviderProxyConfig{}, false, err
	} else if ok {
		baseURL = normalizeProviderBaseURL(platform, dbProvider.BaseURL)
		apiKey = strings.TrimSpace(dbProvider.APIKey)
		model = strings.TrimSpace(firstNonEmpty(dbProvider.DefaultModel, model))
		providerID = dbProvider.ID
		providerName = dbProvider.Name
		providerSource = imageProviderSourceLegacy
		provider = firstNonEmpty(imageProviderAdapterForPlatform(platform), imageProviderOpenAICompatible)
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
			return imageProviderProxyConfig{}, false, fmt.Errorf("api_access.base_url is required when api_access.api_key is configured")
		}
		if apiAccessAPIKey == "" {
			return imageProviderProxyConfig{}, false, fmt.Errorf("api_access.api_key is required when api_access.base_url is configured")
		}
		if apiAccessPlatform != "" {
			provider = firstNonEmpty(imageProviderAdapterForPlatform(apiAccessPlatform), provider)
		} else if mappedProvider := imageProviderAdapterForPlatform(platform); mappedProvider != "" {
			provider = mappedProvider
		}
		baseURL = apiAccessBaseURL
		apiKey = apiAccessAPIKey
		providerID = "api_access"
		providerName = "API 接入配置"
		providerSource = imageProviderSourceAPIAccess
	}

	if provider != imageProviderOpenAICompatible && provider != imageProviderGeminiBanana {
		return imageProviderProxyConfig{}, false, fmt.Errorf("unsupported IMAGE_PROVIDER %q", provider)
	}
	if baseURL == "" && apiKey == "" {
		return imageProviderProxyConfig{}, false, nil
	}
	if baseURL == "" {
		return imageProviderProxyConfig{}, false, fmt.Errorf("api_access.base_url or IMAGE_BASE_URL is required")
	}
	if apiKey == "" {
		return imageProviderProxyConfig{}, false, fmt.Errorf("api_access.api_key or IMAGE_API_KEY is required")
	}
	if model == "" {
		model = defaultModelForProviderPlatform(platform)
	}

	return imageProviderProxyConfig{
		Provider:       provider,
		ProviderID:     providerID,
		ProviderName:   providerName,
		ProviderSource: providerSource,
		Platform:       platform,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		Model:          model,
		RequestTimeout: timeout,
	}, true, nil
}

func (s *Server) defaultBusinessProviderPoolMember(platform string, dispatchTags []string, timeout time.Duration) (imageProviderProxyConfig, bool, error) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		return imageProviderProxyConfig{}, false, err
	}
	defer store.Close()
	selection, ok, err := store.SelectMember(context.Background(), businessproviders.PoolSelectionPolicy{
		Platform: platform,
		Tags:     dispatchTags,
	})
	if err != nil || !ok {
		return imageProviderProxyConfig{}, ok, err
	}
	member := selection.Member
	baseURL := normalizeProviderBaseURL(platform, member.BaseURL)
	apiKey := strings.TrimSpace(member.APIKey)
	model := strings.TrimSpace(member.DefaultModel)
	if model == "" {
		model = defaultModelForProviderPlatform(platform)
	}
	if baseURL == "" {
		return imageProviderProxyConfig{}, false, fmt.Errorf("provider pool member baseUrl is required")
	}
	if apiKey == "" {
		return imageProviderProxyConfig{}, false, fmt.Errorf("provider pool member apiKey is required")
	}
	provider := firstNonEmpty(imageProviderAdapterForPlatform(platform), imageProviderOpenAICompatible)
	if err := store.MarkMemberAcquired(context.Background(), member.ID); err != nil {
		return imageProviderProxyConfig{}, false, err
	}
	cfg := imageProviderProxyConfig{
		Provider:               provider,
		ProviderID:             member.ID,
		ProviderName:           member.Name,
		ProviderSource:         imageProviderSourcePool,
		ProviderGroupID:        selection.Group.ID,
		ProviderGroupMatchMode: selection.Group.MatchMode,
		ProviderGroupTags:      selection.Group.Tags,
		DispatchStrategy:       selection.Strategy,
		DispatchTags:           mergeProviderDispatchTags(dispatchTags),
		Platform:               platform,
		BaseURL:                baseURL,
		APIKey:                 apiKey,
		Model:                  model,
		RequestTimeout:         timeout,
	}
	cfg.DispatchTrace = s.providerConfigDispatchTrace(platform, dispatchTags, cfg)
	return cfg, true, nil
}

func (s *Server) businessProviderPoolConfiguredForPlatform(platform string) (bool, error) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		return false, err
	}
	defer store.Close()
	pools, err := store.ListPools(context.Background())
	if err != nil {
		return false, err
	}
	for _, pool := range pools {
		if businessproviders.NormalizePlatform(pool.Platform) != platform || !pool.Enabled {
			continue
		}
		if len(pool.Members) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) providerConfigDispatchTrace(platform string, dispatchTags []string, providerCfg imageProviderProxyConfig) []string {
	if len(providerCfg.DispatchTrace) > 0 {
		return providerCfg.DispatchTrace
	}
	switch providerCfg.DispatchStrategy {
	case businessproviders.SelectionStrategyTagged:
		return []string{
			"请求标签命中标签池",
			"标签池命中成员：" + firstNonEmpty(providerCfg.ProviderName, providerCfg.ProviderID),
		}
	case businessproviders.SelectionStrategyTaggedFallback:
		return []string{
			"请求标签命中标签池",
			"标签池无可用成员",
			"fallback 池命中成员：" + firstNonEmpty(providerCfg.ProviderName, providerCfg.ProviderID),
		}
	case businessproviders.SelectionStrategyFallback:
		return []string{
			"没有标签池命中",
			"fallback 池命中成员：" + firstNonEmpty(providerCfg.ProviderName, providerCfg.ProviderID),
		}
	case "api_fallback":
		trace := []string{"号池无可用成员"}
		if providerCfg.ProviderSource == imageProviderSourcePool {
			trace = append(trace, "API 兜底未触发")
		} else {
			trace = append(trace, "回退 API 接入："+firstNonEmpty(providerCfg.ProviderName, providerCfg.ProviderSource))
		}
		return trace
	case "api_access":
		return []string{"使用 API 接入：" + firstNonEmpty(providerCfg.ProviderName, providerCfg.ProviderSource)}
	default:
		if providerCfg.ProviderSource == imageProviderSourcePool {
			return []string{"号池命中成员：" + firstNonEmpty(providerCfg.ProviderName, providerCfg.ProviderID)}
		}
		if providerCfg.ProviderSource != "" {
			return []string{"使用上游：" + firstNonEmpty(providerCfg.ProviderName, providerCfg.ProviderSource)}
		}
	}
	_ = platform
	_ = dispatchTags
	return nil
}

func appendProviderDispatchTrace(providerCfg imageProviderProxyConfig, items ...string) imageProviderProxyConfig {
	next := append([]string(nil), providerCfg.DispatchTrace...)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			next = append(next, item)
		}
	}
	providerCfg.DispatchTrace = next
	return providerCfg
}

func (s *Server) imageProviderProxyConfigFromPayload(payload map[string]any) (imageProviderProxyConfig, error) {
	metadata := extractProviderImageGenerateMetadata(payload)
	source := strings.TrimSpace(stringValue(payload["providerSource"]))
	providerID := strings.TrimSpace(stringValue(payload["providerId"]))
	if source == imageProviderSourcePool && providerID != "" {
		if poolCfg, err := s.imageProviderProxyConfigFromPoolMember(providerID); err == nil {
			return applyProviderConfigPayloadMetadata(poolCfg, payload), nil
		}
	}
	providerRequest := append([]string{metadata.Platform}, metadata.DispatchTags...)
	cfg, err := s.imageProviderProxyConfig(providerRequest...)
	if err != nil {
		return imageProviderProxyConfig{}, err
	}
	return applyProviderConfigPayloadMetadata(cfg, payload), nil
}

func applyProviderConfigPayloadMetadata(providerCfg imageProviderProxyConfig, payload map[string]any) imageProviderProxyConfig {
	if payload == nil {
		return providerCfg
	}
	if len(providerCfg.DispatchTags) == 0 {
		providerCfg.DispatchTags = mergeProviderDispatchTags(providerDispatchTagStrings(payload["dispatchTags"]))
	}
	if providerCfg.DispatchStrategy == "" {
		providerCfg.DispatchStrategy = strings.TrimSpace(stringValue(payload["dispatchStrategy"]))
	}
	if len(providerCfg.DispatchTrace) == 0 {
		providerCfg.DispatchTrace = providerDispatchTagStrings(payload["dispatchTrace"])
	}
	if providerCfg.ProviderGroupMatchMode == "" {
		providerCfg.ProviderGroupMatchMode = strings.TrimSpace(stringValue(payload["providerGroupMatchMode"]))
	}
	if providerCfg.ProviderGroupTags == "" {
		providerCfg.ProviderGroupTags = strings.TrimSpace(stringValue(payload["providerGroupTags"]))
	}
	return providerCfg
}

func (s *Server) imageProviderProxyConfigFromPoolMember(memberID string) (imageProviderProxyConfig, error) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		return imageProviderProxyConfig{}, err
	}
	defer store.Close()
	member, ok, err := store.GetMember(context.Background(), memberID)
	if err != nil {
		return imageProviderProxyConfig{}, err
	}
	if !ok || !member.Enabled {
		return imageProviderProxyConfig{}, fmt.Errorf("provider pool member not available")
	}
	platform := businessproviders.NormalizePlatform(member.Platform)
	if platform == "" {
		return imageProviderProxyConfig{}, fmt.Errorf("unsupported provider platform %q", member.Platform)
	}
	if member.Status == businessproviders.MemberStatusUnavailable {
		return imageProviderProxyConfig{}, fmt.Errorf("provider pool member not available")
	}
	if cooldownUntilAfterNow(member.CooldownUntil) {
		return imageProviderProxyConfig{}, fmt.Errorf("provider pool member is cooling down")
	}
	baseURL := normalizeProviderBaseURL(platform, member.BaseURL)
	apiKey := strings.TrimSpace(member.APIKey)
	model := strings.TrimSpace(member.DefaultModel)
	if model == "" {
		model = defaultModelForProviderPlatform(platform)
	}
	if baseURL == "" {
		return imageProviderProxyConfig{}, fmt.Errorf("provider pool member baseUrl is required")
	}
	if apiKey == "" {
		return imageProviderProxyConfig{}, fmt.Errorf("provider pool member apiKey is required")
	}
	provider := firstNonEmpty(imageProviderAdapterForPlatform(platform), imageProviderOpenAICompatible)
	groupMatchMode := ""
	groupTags := ""
	if group, ok, err := store.GetGroup(context.Background(), member.GroupID); err == nil && ok {
		groupMatchMode = group.MatchMode
		groupTags = group.Tags
	}
	return imageProviderProxyConfig{
		Provider:               provider,
		ProviderID:             member.ID,
		ProviderName:           member.Name,
		ProviderSource:         imageProviderSourcePool,
		ProviderGroupID:        member.GroupID,
		ProviderGroupMatchMode: groupMatchMode,
		ProviderGroupTags:      groupTags,
		Platform:               platform,
		BaseURL:                baseURL,
		APIKey:                 apiKey,
		Model:                  model,
		RequestTimeout:         s.imageProviderRequestTimeout(),
	}, nil
}

func (s *Server) reportProviderPoolSuccess(ctx context.Context, providerCfg imageProviderProxyConfig) {
	if providerCfg.ProviderSource != imageProviderSourcePool || strings.TrimSpace(providerCfg.ProviderID) == "" {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		return
	}
	defer store.Close()
	_ = store.ReportMemberSuccess(ctx, providerCfg.ProviderID)
}

func (s *Server) reportProviderPoolRelease(ctx context.Context, providerCfg imageProviderProxyConfig) {
	if providerCfg.ProviderSource != imageProviderSourcePool || strings.TrimSpace(providerCfg.ProviderID) == "" {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		return
	}
	defer store.Close()
	_ = store.ReleaseMember(ctx, providerCfg.ProviderID)
}

func (s *Server) reportProviderPoolFailure(ctx context.Context, providerCfg imageProviderProxyConfig, providerErr providerGenerationError) {
	if providerCfg.ProviderSource != imageProviderSourcePool || strings.TrimSpace(providerCfg.ProviderID) == "" {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		return
	}
	defer store.Close()
	_ = store.ReportMemberFailure(ctx, providerCfg.ProviderID, businessproviders.MemberFailure{
		Status:          providerPoolFailureStatus(providerErr),
		CooldownSeconds: providerPoolFailureCooldownSeconds(providerErr),
		ErrorMessage:    providerErr.Message,
	})
}

func providerPoolFailureStatus(providerErr providerGenerationError) string {
	if providerErr.UpstreamStatus == http.StatusUnauthorized || providerErr.UpstreamStatus == http.StatusForbidden {
		return businessproviders.MemberStatusUnavailable
	}
	message := strings.ToLower(strings.TrimSpace(providerErr.Message))
	if strings.Contains(message, "invalid api key") ||
		strings.Contains(message, "incorrect api key") ||
		strings.Contains(message, "unauthorized") ||
		strings.Contains(message, "forbidden") {
		return businessproviders.MemberStatusUnavailable
	}
	return businessproviders.MemberStatusLimited
}

func providerPoolFailureCooldownSeconds(providerErr providerGenerationError) int {
	if providerPoolFailureStatus(providerErr) == businessproviders.MemberStatusUnavailable {
		return 0
	}
	if providerErr.UpstreamStatus == http.StatusTooManyRequests {
		return 600
	}
	return 0
}

func cooldownUntilAfterNow(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return false
	}
	return parsed.After(time.Now().UTC())
}

func (s *Server) defaultBusinessAPIProvider(platform string) (businessproviders.Provider, bool, error) {
	store, err := s.newBusinessProviderStore()
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
	switch businessproviders.NormalizePlatform(platform) {
	case businessproviders.PlatformGeminiBanana:
		return defaultGeminiBananaModel
	case businessproviders.PlatformDoubao:
		return "doubao-seedream-image"
	case businessproviders.PlatformQwen:
		return "qwen-image"
	case businessproviders.PlatformBaidu:
		return "baidu-image"
	case businessproviders.PlatformZAI:
		return "z-ai-image"
	case businessproviders.PlatformTencent:
		return "tencent-image"
	case businessproviders.PlatformKling:
		return "kling-image"
	case businessproviders.PlatformGrok:
		return "grok-image"
	}
	return cpaFixedImageModel
}

func normalizeProviderBaseURL(platform string, value string) string {
	switch businessproviders.NormalizePlatform(platform) {
	case businessproviders.PlatformGeminiBanana:
		return normalizeGeminiBananaBaseURL(value)
	case businessproviders.PlatformGPTImage,
		businessproviders.PlatformDoubao,
		businessproviders.PlatformQwen,
		businessproviders.PlatformBaidu,
		businessproviders.PlatformZAI,
		businessproviders.PlatformTencent,
		businessproviders.PlatformKling,
		businessproviders.PlatformGrok:
		return normalizeOpenAICompatibleBaseURL(value)
	}
	return normalizeOpenAICompatibleBaseURL(value)
}

func imageProviderAdapterForPlatform(platform string) string {
	switch businessproviders.NormalizePlatform(platform) {
	case businessproviders.PlatformGeminiBanana:
		return imageProviderGeminiBanana
	case businessproviders.PlatformGPTImage,
		businessproviders.PlatformDoubao,
		businessproviders.PlatformQwen,
		businessproviders.PlatformBaidu,
		businessproviders.PlatformZAI,
		businessproviders.PlatformTencent,
		businessproviders.PlatformKling,
		businessproviders.PlatformGrok:
		return imageProviderOpenAICompatible
	default:
		return ""
	}
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

func normalizeOptionalNonNegativeInt(value any) int {
	switch typed := value.(type) {
	case int:
		if typed >= 0 {
			return typed
		}
	case int64:
		if typed >= 0 {
			return int(typed)
		}
	case float64:
		if typed >= 0 {
			return int(typed)
		}
	case json.Number:
		parsed, _ := typed.Int64()
		if parsed >= 0 {
			return int(parsed)
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		parsed, _ := strconv.Atoi(trimmed)
		if parsed >= 0 {
			return parsed
		}
	}
	return 0
}

func normalizeNonNegativeInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return int64(typed)
		}
	case int64:
		if typed > 0 {
			return typed
		}
	case float64:
		if typed > 0 {
			return int64(typed)
		}
	case json.Number:
		parsed, _ := typed.Int64()
		if parsed > 0 {
			return parsed
		}
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if parsed > 0 {
			return parsed
		}
	}
	return 0
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
