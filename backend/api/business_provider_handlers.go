package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"imagestudio/internal/businessproviders"
)

const businessProviderTestPrompt = "a simple red circle on a white background"

type businessProviderPayload struct {
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	BaseURL      string `json:"baseUrl"`
	APIKey       string `json:"apiKey"`
	DefaultModel string `json:"defaultModel"`
	Enabled      bool   `json:"enabled"`
	IsDefault    bool   `json:"isDefault"`
}

func (s *Server) handleListBusinessAPIProviders(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	if err := store.EnsureConfigProvider(r.Context(), s.cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	items, err := store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateBusinessAPIProvider(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeBusinessProviderPayload(w, r)
	if !ok {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, err := store.Create(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleUpdateBusinessAPIProvider(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeBusinessProviderPayload(w, r)
	if !ok {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, found, err := store.Update(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleSetDefaultBusinessAPIProvider(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, found, err := store.SetDefault(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleDeleteBusinessAPIProvider(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	deleted, err := store.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestBusinessAPIProvider(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, found, err := store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}
	providerCfg, err := imageProviderProxyConfigFromBusinessProvider(item)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	payload := map[string]any{
		"prompt":          businessProviderTestPrompt,
		"model":           providerCfg.Model,
		"n":               1,
		"size":            "1024x1024",
		"quality":         "low",
		"response_format": "b64_json",
	}
	started := time.Now()
	body, _, err := executeProviderImageGeneration(r.Context(), providerCfg, payload, 1, providerResolvedEditInput{}, providerImageGenerationHooks{})
	durationMs := time.Since(started).Milliseconds()
	if err != nil {
		providerErr := providerGenerationErrorDetails(err)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":         false,
			"message":    providerErr.Message,
			"code":       providerErr.Code,
			"durationMs": durationMs,
			"imageCount": 0,
		})
		return
	}
	imageCount := countProviderImageItems(body)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         imageCount > 0,
		"message":    "provider test succeeded",
		"durationMs": durationMs,
		"imageCount": imageCount,
	})
}

func imageProviderProxyConfigFromBusinessProvider(item businessproviders.Provider) (imageProviderProxyConfig, error) {
	platform := businessproviders.NormalizePlatform(item.Platform)
	if platform == "" {
		return imageProviderProxyConfig{}, fmt.Errorf("unsupported provider platform %q", item.Platform)
	}
	baseURL := normalizeProviderBaseURL(platform, item.BaseURL)
	apiKey := strings.TrimSpace(item.APIKey)
	model := strings.TrimSpace(item.DefaultModel)
	if model == "" {
		model = defaultModelForProviderPlatform(platform)
	}
	if baseURL == "" {
		return imageProviderProxyConfig{}, fmt.Errorf("baseUrl is required")
	}
	if apiKey == "" {
		return imageProviderProxyConfig{}, fmt.Errorf("apiKey is required")
	}
	provider := imageProviderOpenAICompatible
	if platform == businessproviders.PlatformGeminiBanana {
		provider = imageProviderGeminiBanana
	}
	return imageProviderProxyConfig{
		Provider:       provider,
		ProviderID:     item.ID,
		ProviderName:   item.Name,
		Platform:       platform,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		Model:          model,
		RequestTimeout: 180 * time.Second,
	}, nil
}

func decodeBusinessProviderPayload(w http.ResponseWriter, r *http.Request) (businessproviders.MutationInput, bool) {
	var body businessProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return businessproviders.MutationInput{}, false
	}
	input := businessproviders.MutationInput{
		Name:         body.Name,
		Platform:     body.Platform,
		BaseURL:      body.BaseURL,
		APIKey:       body.APIKey,
		DefaultModel: body.DefaultModel,
		Enabled:      body.Enabled,
		IsDefault:    body.IsDefault,
	}
	if strings.TrimSpace(body.Platform) == "" {
		input.Platform = businessproviders.PlatformGPTImage
	}
	return input, true
}
