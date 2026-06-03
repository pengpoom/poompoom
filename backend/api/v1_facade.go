package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/businessapikeys"
	"imagestudio/internal/businessjobs"
)

func toPublicStatus(internal string) string {
	switch internal {
	case "queued":
		return "queued"
	case "succeeded":
		return "succeeded"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	case "running", "cancel_requested":
		return "running"
	default:
		return "running"
	}
}

func signImageFileToken(secret, fileName string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%d", fileName, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyImageFileToken(secret, fileName string, exp int64, sig string) bool {
	want := signImageFileToken(secret, fileName, exp)
	return hmac.Equal([]byte(want), []byte(sig))
}

func signImageFileQuery(secret, fileName string, exp int64) string {
	sig := signImageFileToken(secret, fileName, exp)
	v := url.Values{}
	v.Set("exp", strconv.FormatInt(exp, 10))
	v.Set("sig", sig)
	return "?" + v.Encode()
}

type ctxKey string

const ctxKeyAPIKey ctxKey = "external_api_key"

func apiKeyFromContext(ctx context.Context) (businessapikeys.APIKey, bool) {
	k, ok := ctx.Value(ctxKeyAPIKey).(businessapikeys.APIKey)
	return k, ok
}

func (s *Server) requireExternalAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.ExternalAPI.Enabled {
			writeV1Error(w, http.StatusServiceUnavailable, "service_unavailable", "external api is disabled")
			return
		}
		token := bearerFromRequest(r)
		if token == "" {
			writeV1Error(w, http.StatusUnauthorized, "invalid_api_key", "missing bearer token")
			return
		}
		store, err := s.newBusinessAPIKeyStore()
		if err != nil {
			writeV1Error(w, http.StatusInternalServerError, "store_unavailable", "key store failed")
			return
		}
		defer store.Close()
		key, err := store.Authenticate(r.Context(), token)
		if err != nil {
			writeV1Error(w, http.StatusUnauthorized, "invalid_api_key", "invalid api key")
			return
		}
		if key.Status != businessapikeys.StatusActive {
			writeV1Error(w, http.StatusForbidden, "key_disabled", "api key is not active")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyAPIKey, key)))
	})
}

func writeV1Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": message, "type": v1ErrorType(status), "code": code},
	})
}

func v1ErrorType(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "authentication_error"
	case status >= 400 && status < 500:
		return "invalid_request_error"
	default:
		return "api_error"
	}
}

type externalAttribution struct {
	apiKeyID string
	metadata []byte
}

const ctxKeyExternalAttribution ctxKey = "external_api_attribution"

func withExternalAttribution(ctx context.Context, apiKeyID string, metadata []byte) context.Context {
	return context.WithValue(ctx, ctxKeyExternalAttribution, externalAttribution{apiKeyID: apiKeyID, metadata: metadata})
}

func externalAttributionFromContext(ctx context.Context) (string, []byte, bool) {
	v, ok := ctx.Value(ctxKeyExternalAttribution).(externalAttribution)
	if !ok {
		return "", nil, false
	}
	return v.apiKeyID, v.metadata, true
}

type v1ImageGenerationRequest struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt"`
	N              int             `json:"n"`
	Size           string          `json:"size"`
	Quality        string          `json:"quality"`
	ResponseFormat string          `json:"response_format"`
	Metadata       json.RawMessage `json:"metadata"`
}

type v1ImageOut struct {
	URL     string `json:"url,omitempty"`
	B64JSON string `json:"b64_json,omitempty"`
}

type v1JobError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type v1ImageJobResponse struct {
	ID        string          `json:"id"`
	Object    string          `json:"object"`
	Status    string          `json:"status"`
	Model     string          `json:"model,omitempty"`
	Created   int64           `json:"created"`
	StatusURL string          `json:"status_url,omitempty"`
	Images    []v1ImageOut    `json:"images,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	Error     *v1JobError     `json:"error,omitempty"`
}

func (s *Server) v1StatusURL(jobID string) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.ExternalAPI.BaseURL), "/")
	return base + "/v1/images/jobs/" + jobID
}

func (s *Server) handleV1CreateImageGeneration(w http.ResponseWriter, r *http.Request) {
	key, ok := apiKeyFromContext(r.Context())
	if !ok {
		writeV1Error(w, http.StatusUnauthorized, "invalid_api_key", "missing api key")
		return
	}
	startedAt := time.Now().UTC()
	var req v1ImageGenerationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxImageProviderRequestBytes)).Decode(&req); err != nil {
		writeV1Error(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		writeV1Error(w, http.StatusBadRequest, "invalid_request", "prompt is required")
		return
	}
	if maxP := s.cfg.ExternalAPI.MaxPromptBytes; maxP > 0 && len(prompt) > maxP {
		writeV1Error(w, http.StatusBadRequest, "invalid_request", "prompt too long")
		return
	}
	md := []byte(req.Metadata)
	if len(md) == 0 {
		md = []byte("{}")
	}
	if maxM := s.cfg.ExternalAPI.MaxMetadataBytes; maxM > 0 && len(md) > maxM {
		writeV1Error(w, http.StatusBadRequest, "invalid_request", "metadata too large")
		return
	}
	payload := map[string]any{"prompt": prompt}
	if m := strings.TrimSpace(req.Model); m != "" {
		payload["model"] = m
	}
	if req.N > 0 {
		payload["n"] = req.N
	}
	if sz := strings.TrimSpace(req.Size); sz != "" {
		payload["size"] = sz
	}
	if q := strings.TrimSpace(req.Quality); q != "" {
		payload["quality"] = q
	}
	ctx := withExternalAttribution(r.Context(), key.ID, md)
	job, err := s.createQueuedProviderImageJob(ctx, key.UserID, payload, startedAt)
	if err != nil {
		if translateProviderSubmitError(w, err) {
			return
		}
		writeV1Error(w, http.StatusInternalServerError, "image_job_create_failed", err.Error())
		return
	}
	s.notifyBusinessImageJob(job)
	writeJSON(w, http.StatusAccepted, v1ImageJobResponse{
		ID:        job.ID,
		Object:    "image.generation.job",
		Status:    toPublicStatus(job.Status),
		Model:     job.Model,
		Created:   startedAt.Unix(),
		StatusURL: s.v1StatusURL(job.ID),
	})
}

func translateProviderSubmitError(w http.ResponseWriter, err error) bool {
	var submitErr *providerImageGenerateSubmitError
	if errors.As(err, &submitErr) {
		writeProviderResultAsV1(w, submitErr.result)
		return true
	}
	var jobErr *providerImageGenerateJobSubmitError
	if errors.As(err, &jobErr) {
		writeProviderResultAsV1(w, jobErr.result)
		return true
	}
	return false
}

func writeProviderResultAsV1(w http.ResponseWriter, res providerImageGenerateResult) {
	status := res.StatusCode
	if status == 0 {
		status = http.StatusBadRequest
	}
	writeV1Error(w, status, firstNonEmpty(res.ErrorCode, "image_job_failed"), firstNonEmpty(res.ErrorMessage, "image job submit failed"))
}

func (s *Server) handleV1GetImageJob(w http.ResponseWriter, r *http.Request) {
	key, ok := apiKeyFromContext(r.Context())
	if !ok {
		writeV1Error(w, http.StatusUnauthorized, "invalid_api_key", "missing api key")
		return
	}
	jobStore, err := s.newBusinessJobStore()
	if err != nil {
		writeV1Error(w, http.StatusInternalServerError, "store_unavailable", "job store failed")
		return
	}
	defer jobStore.Close()
	s.reconcileStaleBusinessImageJobs(r.Context())
	job, found, err := jobStore.Get(r.Context(), r.PathValue("id"), key.UserID)
	if err != nil {
		writeV1Error(w, http.StatusInternalServerError, "job_lookup_failed", err.Error())
		return
	}
	if !found || job.APIKeyID != key.ID {
		writeV1Error(w, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	public := toPublicStatus(job.Status)
	resp := v1ImageJobResponse{
		ID:       job.ID,
		Object:   "image.generation.job",
		Status:   public,
		Model:    job.Model,
		Created:  parseRFC3339Unix(job.CreatedAt),
		Metadata: metadataOrNil(job.APIMetadata),
	}
	switch public {
	case "succeeded":
		images, ierr := s.v1JobImages(r.Context(), job, key.UserID, strings.EqualFold(r.URL.Query().Get("response_format"), "b64_json"))
		if ierr != nil {
			writeV1Error(w, http.StatusInternalServerError, "asset_lookup_failed", ierr.Error())
			return
		}
		resp.Images = images
	case "failed":
		if job.ErrorCode != "" || job.ErrorMessage != "" {
			resp.Error = &v1JobError{Code: job.ErrorCode, Message: job.ErrorMessage}
		}
	case "queued", "running":
		resp.StatusURL = s.v1StatusURL(job.ID)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) v1JobImages(ctx context.Context, job businessjobs.Job, userID string, asB64 bool) ([]v1ImageOut, error) {
	imgStore, err := s.newBusinessImageStore()
	if err != nil {
		return nil, err
	}
	defer imgStore.Close()
	assets, err := imgStore.AssetsByGeneration(ctx, job.GenerationID, userID)
	if err != nil {
		return nil, err
	}
	out := make([]v1ImageOut, 0, len(assets))
	secret := strings.TrimSpace(s.cfg.ExternalAPI.SigningSecret)
	ttl := int64(s.cfg.ExternalAPI.SignedURLTTLSeconds)
	if ttl <= 0 {
		ttl = 3600
	}
	base := strings.TrimRight(strings.TrimSpace(s.cfg.ExternalAPI.BaseURL), "/")
	for _, a := range assets {
		if asB64 {
			b, rerr := os.ReadFile(s.resolveImageFilePath(a.FileName))
			if rerr != nil {
				return nil, rerr
			}
			out = append(out, v1ImageOut{B64JSON: base64.StdEncoding.EncodeToString(b)})
			continue
		}
		exp := time.Now().UTC().Add(time.Duration(ttl) * time.Second).Unix()
		out = append(out, v1ImageOut{URL: base + "/v1/files/image/" + a.FileName + signImageFileQuery(secret, a.FileName, exp)})
	}
	return out, nil
}

func metadataOrNil(raw []byte) json.RawMessage {
	if len(raw) == 0 || string(raw) == "{}" {
		return nil
	}
	return json.RawMessage(raw)
}

func parseRFC3339Unix(s string) int64 {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return t.Unix()
}

func (s *Server) handleV1CancelImageJob(w http.ResponseWriter, r *http.Request) {
	key, ok := apiKeyFromContext(r.Context())
	if !ok {
		writeV1Error(w, http.StatusUnauthorized, "invalid_api_key", "missing api key")
		return
	}
	store, err := s.newBusinessJobStore()
	if err != nil {
		writeV1Error(w, http.StatusInternalServerError, "store_unavailable", "job store failed")
		return
	}
	defer store.Close()
	id := r.PathValue("id")
	before, beforeOK, _ := store.Get(r.Context(), id, key.UserID)
	if !beforeOK || before.APIKeyID != key.ID {
		writeV1Error(w, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	item, ok2, err := store.RequestCancel(r.Context(), id, key.UserID)
	if err != nil {
		writeV1Error(w, http.StatusInternalServerError, "cancel_failed", err.Error())
		return
	}
	if !ok2 {
		writeV1Error(w, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	if businessImageJobBeforeUpstream(before) &&
		(item.Status == businessjobs.StatusCancelled || item.Status == businessjobs.StatusCancelRequested) {
		item = s.refundCancelledPreUpstreamBusinessImageJob(context.Background(), item)
	}
	if item.Status == businessjobs.StatusCancelRequested || item.Status == businessjobs.StatusCancelled {
		if s.cancelActiveBusinessImageJob(item.ID) && item.Status == businessjobs.StatusCancelRequested {
			item.Status = businessjobs.StatusCancelled
			item.Stage = "cancelled"
			item.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if saved, saveErr := store.Save(context.Background(), item); saveErr == nil {
				item = saved
			}
		}
	}
	s.markBusinessImageGenerationCancelled(context.Background(), key.UserID, item.GenerationID)
	writeJSON(w, http.StatusOK, v1ImageJobResponse{
		ID:       item.ID,
		Object:   "image.generation.job",
		Status:   toPublicStatus(item.Status),
		Model:    item.Model,
		Created:  parseRFC3339Unix(item.CreatedAt),
		Metadata: metadataOrNil(item.APIMetadata),
	})
}
