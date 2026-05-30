package api

import (
	"context"
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

type businessProviderGroupPayload struct {
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	MatchMode   string `json:"matchMode"`
	Enabled     bool   `json:"enabled"`
	IsDefault   bool   `json:"isDefault"`
	Priority    int    `json:"priority"`
}

type businessProviderMemberPayload struct {
	GroupID          string `json:"groupId"`
	Name             string `json:"name"`
	Platform         string `json:"platform"`
	BaseURL          string `json:"baseUrl"`
	APIKey           string `json:"apiKey"`
	DefaultModel     string `json:"defaultModel"`
	Enabled          bool   `json:"enabled"`
	Priority         int    `json:"priority"`
	Weight           int    `json:"weight"`
	MaxConcurrent    int    `json:"maxConcurrent"`
	CooldownSeconds  int    `json:"cooldownSeconds"`
	FailureThreshold int    `json:"failureThreshold"`
	Status           string `json:"status"`
}

type businessProviderDispatchPreviewPayload struct {
	Platform        string   `json:"platform"`
	Role            string   `json:"role"`
	SubscriptionTag string   `json:"subscriptionTag"`
	WalletTag       string   `json:"walletTag"`
	Mode            string   `json:"mode"`
	Quality         string   `json:"quality"`
	Size            string   `json:"size"`
	Model           string   `json:"model"`
	ExtraTags       []string `json:"extraTags"`
}

type businessProviderDispatchPreviewResponse struct {
	OK                bool                                   `json:"ok"`
	Message           string                                 `json:"message"`
	Platform          string                                 `json:"platform"`
	RequestTags       []string                               `json:"requestTags"`
	UserTags          []string                               `json:"userTags"`
	DispatchTags      []string                               `json:"dispatchTags"`
	Strategy          string                                 `json:"strategy,omitempty"`
	Group             *businessproviders.Group               `json:"group,omitempty"`
	Member            *businessproviders.Member              `json:"member,omitempty"`
	FallbackAvailable bool                                   `json:"fallbackAvailable"`
	FallbackSource    string                                 `json:"fallbackSource,omitempty"`
	FallbackName      string                                 `json:"fallbackName,omitempty"`
	Trace             []string                               `json:"trace,omitempty"`
	Pools             []businessProviderDispatchPreviewPool  `json:"pools,omitempty"`
	Issues            []businessProviderDispatchPreviewIssue `json:"issues,omitempty"`
}

type businessProviderDispatchPreviewPool struct {
	ID                      string                                  `json:"id"`
	Name                    string                                  `json:"name"`
	Platform                string                                  `json:"platform"`
	Enabled                 bool                                    `json:"enabled"`
	IsDefault               bool                                    `json:"isDefault"`
	Priority                int                                     `json:"priority"`
	MatchMode               string                                  `json:"matchMode"`
	Tags                    []string                                `json:"tags,omitempty"`
	Role                    string                                  `json:"role"`
	Matched                 bool                                    `json:"matched"`
	Considered              bool                                    `json:"considered"`
	ReasonCode              string                                  `json:"reasonCode"`
	Reason                  string                                  `json:"reason"`
	MemberTotal             int                                     `json:"memberTotal"`
	AvailableMembers        int                                     `json:"availableMembers"`
	DisabledMembers         int                                     `json:"disabledMembers"`
	UnavailableMembers      int                                     `json:"unavailableMembers"`
	LimitedMembers          int                                     `json:"limitedMembers"`
	CoolingMembers          int                                     `json:"coolingMembers"`
	ConcurrencyFullMembers  int                                     `json:"concurrencyFullMembers"`
	PlatformMismatchMembers int                                     `json:"platformMismatchMembers"`
	Members                 []businessProviderDispatchPreviewMember `json:"members,omitempty"`
}

type businessProviderDispatchPreviewMember struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Status        string `json:"status"`
	Priority      int    `json:"priority"`
	Weight        int    `json:"weight"`
	MaxConcurrent int    `json:"maxConcurrent"`
	Running       int64  `json:"running"`
	CooldownUntil string `json:"cooldownUntil,omitempty"`
	Available     bool   `json:"available"`
	ReasonCode    string `json:"reasonCode"`
	Reason        string `json:"reason"`
	LastError     string `json:"lastError,omitempty"`
	LastErrorAt   string `json:"lastErrorAt,omitempty"`
}

type businessProviderDispatchPreviewIssue struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Detail string `json:"detail,omitempty"`
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

func (s *Server) handleListBusinessProviderPools(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListPools(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handlePreviewBusinessProviderDispatch(w http.ResponseWriter, r *http.Request) {
	var body businessProviderDispatchPreviewPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	platform := businessproviders.NormalizePlatform(body.Platform)
	if platform == "" {
		platform = businessproviders.PlatformGPTImage
	}
	if platform != businessproviders.PlatformGPTImage && platform != businessproviders.PlatformGeminiBanana {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("unsupported provider platform %q", platform)})
		return
	}
	requestTags := businessProviderPreviewRequestTags(platform, body)
	userTags := businessProviderPreviewUserTags(body)
	dispatchTags := mergeProviderDispatchTags(requestTags, userTags)
	resp := businessProviderDispatchPreviewResponse{
		OK:           false,
		Message:      "当前平台没有可用号池成员，也没有 API 兜底配置",
		Platform:     platform,
		RequestTags:  requestTags,
		UserTags:     userTags,
		DispatchTags: dispatchTags,
	}

	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	selection, selected, err := store.SelectMember(r.Context(), businessproviders.PoolSelectionPolicy{
		Platform: platform,
		Tags:     dispatchTags,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	fallbackCfg := imageProviderProxyConfig{}
	fallbackOK := false
	var fallbackErr error
	if !selected {
		fallbackCfg, fallbackOK, fallbackErr = s.imageProviderProxyConfigWithoutPool(platform, s.imageProviderRequestTimeout())
	}
	resp.Pools, resp.Trace, resp.Issues = s.businessProviderDispatchPreviewExplain(
		r.Context(),
		store,
		platform,
		dispatchTags,
		selection,
		selected,
		fallbackCfg,
		fallbackOK,
		fallbackErr,
	)
	if selected {
		group := selection.Group
		member := selection.Member
		resp.OK = true
		resp.Strategy = selection.Strategy
		resp.Group = &group
		resp.Member = &member
		resp.Message = businessProviderDispatchPreviewMessage(selection.Strategy)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if fallbackErr != nil {
		resp.Message = fallbackErr.Error()
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if fallbackOK {
		resp.OK = true
		resp.Message = "未命中可用号池成员，将走 API 兜底"
		resp.Strategy = "api_fallback"
		resp.FallbackAvailable = true
		resp.FallbackSource = fallbackCfg.ProviderSource
		resp.FallbackName = fallbackCfg.ProviderName
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateBusinessProviderGroup(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeBusinessProviderGroupPayload(w, r)
	if !ok {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, err := store.CreateGroup(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleUpdateBusinessProviderGroup(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeBusinessProviderGroupPayload(w, r)
	if !ok {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, found, err := store.UpdateGroup(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider group not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleSetDefaultBusinessProviderGroup(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, found, err := store.SetDefaultGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider group not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleDeleteBusinessProviderGroup(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	deleted, err := store.DeleteGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider group not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestBusinessProviderGroup(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	group, found, err := store.GetGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider group not found"})
		return
	}
	providerCfg, selectedMember, err := s.imageProviderProxyConfigFromSelectedBusinessProviderGroup(r.Context(), group)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	s.writeBusinessProviderTestResult(w, r, providerCfg, map[string]any{
		"group":  group,
		"member": selectedMember,
	})
}

func (s *Server) handleCreateBusinessProviderMember(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeBusinessProviderMemberPayload(w, r)
	if !ok {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, err := store.CreateMember(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleUpdateBusinessProviderMember(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeBusinessProviderMemberPayload(w, r)
	if !ok {
		return
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, found, err := store.UpdateMember(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider member not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleDeleteBusinessProviderMember(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	deleted, err := store.DeleteMember(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider member not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRecoverBusinessProviderMember(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	item, found, err := store.RecoverMember(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider member not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleTestBusinessProviderMember(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "provider store failed"})
		return
	}
	defer store.Close()
	member, found, err := store.GetMember(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider member not found"})
		return
	}
	providerCfg, err := imageProviderProxyConfigFromBusinessProviderMember(member, s.imageProviderRequestTimeout())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	s.writeBusinessProviderTestResult(w, r, providerCfg, nil)
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
	s.writeBusinessProviderTestResult(w, r, providerCfg, nil)
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
		ProviderSource: imageProviderSourceLegacy,
		Platform:       platform,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		Model:          model,
		RequestTimeout: 180 * time.Second,
	}, nil
}

func imageProviderProxyConfigFromBusinessProviderMember(item businessproviders.Member, timeout time.Duration) (imageProviderProxyConfig, error) {
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
		Provider:        provider,
		ProviderID:      item.ID,
		ProviderName:    item.Name,
		ProviderSource:  imageProviderSourcePool,
		ProviderGroupID: item.GroupID,
		Platform:        platform,
		BaseURL:         baseURL,
		APIKey:          apiKey,
		Model:           model,
		RequestTimeout:  timeout,
	}, nil
}

func (s *Server) imageProviderProxyConfigFromSelectedBusinessProviderGroup(ctx context.Context, group businessproviders.Group) (imageProviderProxyConfig, businessproviders.Member, error) {
	store, err := s.newBusinessProviderStore()
	if err != nil {
		return imageProviderProxyConfig{}, businessproviders.Member{}, err
	}
	defer store.Close()
	selection, ok, err := store.SelectMember(ctx, businessproviders.PoolSelectionPolicy{
		Platform: group.Platform,
		Tags:     []string{fmt.Sprintf("pool:%s", group.ID)},
	})
	if err != nil {
		return imageProviderProxyConfig{}, businessproviders.Member{}, err
	}
	if !ok || selection.Group.ID != group.ID {
		return imageProviderProxyConfig{}, businessproviders.Member{}, fmt.Errorf("provider group has no currently selectable member")
	}
	providerCfg, err := imageProviderProxyConfigFromBusinessProviderMember(selection.Member, s.imageProviderRequestTimeout())
	if err != nil {
		return imageProviderProxyConfig{}, businessproviders.Member{}, err
	}
	providerCfg.ProviderGroupID = selection.Group.ID
	return providerCfg, selection.Member, nil
}

func (s *Server) writeBusinessProviderTestResult(w http.ResponseWriter, r *http.Request, providerCfg imageProviderProxyConfig, extra map[string]any) {
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
		resp := map[string]any{
			"ok":         false,
			"message":    providerErr.Message,
			"code":       providerErr.Code,
			"durationMs": durationMs,
			"imageCount": 0,
		}
		for key, value := range extra {
			resp[key] = value
		}
		writeJSON(w, http.StatusBadGateway, resp)
		return
	}
	imageCount := countProviderImageItems(body)
	if imageCount <= 0 {
		resp := map[string]any{
			"ok":         false,
			"message":    "上游接口返回成功，但没有返回可用图片数据",
			"code":       "provider_empty_response",
			"durationMs": durationMs,
			"imageCount": 0,
		}
		for key, value := range extra {
			resp[key] = value
		}
		writeJSON(w, http.StatusBadGateway, resp)
		return
	}
	resp := map[string]any{
		"ok":         true,
		"message":    "provider test succeeded",
		"durationMs": durationMs,
		"imageCount": imageCount,
	}
	for key, value := range extra {
		resp[key] = value
	}
	writeJSON(w, http.StatusOK, resp)
}

func businessProviderPreviewRequestTags(platform string, body businessProviderDispatchPreviewPayload) []string {
	mode := normalizeDispatchTagPart(body.Mode, "generate")
	quality := normalizeDispatchTagPart(body.Quality, "high")
	size := normalizeDispatchTagPart(body.Size, "")
	model := strings.TrimSpace(body.Model)
	if model == "" {
		model = defaultModelForProviderPlatform(platform)
	}
	tags := []string{
		"mode:" + mode,
		"quality:" + quality,
		"model:" + normalizeDispatchTagPart(model, defaultModelForProviderPlatform(platform)),
	}
	if size != "" {
		tags = append(tags, "size:"+size)
	}
	tags = append(tags, body.ExtraTags...)
	return filterProtectedProviderDispatchTags(tags)
}

func businessProviderPreviewUserTags(body businessProviderDispatchPreviewPayload) []string {
	role := "role:" + normalizeDispatchTagPart(body.Role, authRoleUser)
	subscriptionTag := normalizeProviderDispatchTag(body.SubscriptionTag)
	if subscriptionTag == "" || !strings.HasPrefix(subscriptionTag, "tier:") {
		subscriptionTag = defaultSubscriptionDispatchTag
	}
	walletTag := normalizeProviderDispatchTag(body.WalletTag)
	if walletTag == "" || !strings.HasPrefix(walletTag, "wallet:") {
		walletTag = defaultWalletDispatchTag
	}
	return filterUserProviderDispatchTags([]string{role, subscriptionTag, walletTag})
}

func (s *Server) businessProviderDispatchPreviewExplain(
	ctx context.Context,
	store *businessproviders.Store,
	platform string,
	dispatchTags []string,
	selection businessproviders.PoolSelection,
	selected bool,
	fallbackCfg imageProviderProxyConfig,
	fallbackOK bool,
	fallbackErr error,
) ([]businessProviderDispatchPreviewPool, []string, []businessProviderDispatchPreviewIssue) {
	pools, err := store.ListPools(ctx)
	if err != nil {
		return nil, nil, []businessProviderDispatchPreviewIssue{{
			Code:   "provider_pool_list_failed",
			Label:  "读取池失败",
			Count:  1,
			Detail: err.Error(),
		}}
	}
	now := time.Now()
	jobStore, jobErr := s.newBusinessJobStore()
	if jobErr == nil {
		defer jobStore.Close()
	}
	views := make([]businessProviderDispatchPreviewPool, 0, len(pools))
	issues := make([]businessProviderDispatchPreviewIssue, 0)
	for _, pool := range pools {
		view := businessProviderDispatchPreviewPool{
			ID:        pool.ID,
			Name:      pool.Name,
			Platform:  businessproviders.NormalizePlatform(pool.Platform),
			Enabled:   pool.Enabled,
			IsDefault: pool.IsDefault,
			Priority:  pool.Priority,
			MatchMode: pool.MatchMode,
			Tags:      providerDispatchTagStrings(pool.Tags),
			Role:      "ignored",
		}
		view.Matched, view.Role, view.ReasonCode, view.Reason = businessProviderPreviewPoolMatch(pool.Group, platform, dispatchTags)
		view.Considered = view.Role == "tagged" || view.Role == "fallback"
		for _, member := range pool.Members {
			running := int64(0)
			if jobStore != nil && member.MaxConcurrent > 0 {
				if count, countErr := jobStore.CountProviderRunning(ctx, member.ID, view.Platform); countErr == nil {
					running = count
				}
			}
			memberView := businessProviderPreviewMember(member, view.Platform, now, running)
			view.Members = append(view.Members, memberView)
			view.MemberTotal++
			if memberView.Available && view.Enabled && view.Platform == platform {
				view.AvailableMembers++
				continue
			}
			switch memberView.ReasonCode {
			case "member_disabled":
				view.DisabledMembers++
			case "member_unavailable":
				view.UnavailableMembers++
			case "member_limited":
				view.LimitedMembers++
			case "member_cooling":
				view.CoolingMembers++
			case "member_concurrency_full":
				view.ConcurrencyFullMembers++
			case "member_platform_mismatch":
				view.PlatformMismatchMembers++
			}
		}
		views = append(views, view)
	}
	if jobErr != nil {
		issues = append(issues, businessProviderDispatchPreviewIssue{
			Code:   "job_store_unavailable",
			Label:  "并发计数不可用",
			Count:  1,
			Detail: jobErr.Error(),
		})
	}
	issues = append(issues, businessProviderDispatchPreviewIssues(views, fallbackOK, fallbackErr)...)
	trace := businessProviderDispatchPreviewTrace(views, selection, selected, fallbackCfg, fallbackOK, fallbackErr)
	return views, trace, issues
}

func businessProviderPreviewPoolMatch(group businessproviders.Group, platform string, dispatchTags []string) (bool, string, string, string) {
	groupPlatform := businessproviders.NormalizePlatform(group.Platform)
	if groupPlatform != platform {
		return false, "ignored", "platform_mismatch", "平台不匹配"
	}
	if !group.Enabled {
		return false, "ignored", "group_disabled", "池已禁用"
	}
	groupTags := providerDispatchTagStrings(group.Tags)
	if group.MatchMode == businessproviders.GroupMatchFallback || len(groupTags) == 0 {
		return true, "fallback", "fallback_pool", "fallback 候选池"
	}
	missing := missingDispatchTags(groupTags, dispatchTags)
	if group.MatchMode == businessproviders.GroupMatchAll {
		if len(missing) == 0 {
			return true, "tagged", "tagged_all_matched", "全部标签命中"
		}
		return false, "ignored", "tag_mismatch", "缺少标签：" + strings.Join(missing, ", ")
	}
	if anyDispatchTagMatched(groupTags, dispatchTags) {
		return true, "tagged", "tagged_any_matched", "任一标签命中"
	}
	return false, "ignored", "tag_mismatch", "标签未命中"
}

func businessProviderPreviewMember(member businessproviders.Member, poolPlatform string, now time.Time, running int64) businessProviderDispatchPreviewMember {
	view := businessProviderDispatchPreviewMember{
		ID:            member.ID,
		Name:          member.Name,
		Enabled:       member.Enabled,
		Status:        member.Status,
		Priority:      member.Priority,
		Weight:        member.Weight,
		MaxConcurrent: member.MaxConcurrent,
		Running:       running,
		CooldownUntil: member.CooldownUntil,
		LastError:     member.LastError,
		LastErrorAt:   member.LastErrorAt,
	}
	memberPlatform := businessproviders.NormalizePlatform(member.Platform)
	switch {
	case memberPlatform != poolPlatform:
		view.ReasonCode = "member_platform_mismatch"
		view.Reason = "成员平台不匹配"
	case !member.Enabled:
		view.ReasonCode = "member_disabled"
		view.Reason = "成员已禁用"
	case member.Status == businessproviders.MemberStatusUnavailable:
		view.ReasonCode = "member_unavailable"
		view.Reason = "成员 unavailable"
	case member.Status == businessproviders.MemberStatusLimited:
		view.ReasonCode = "member_limited"
		view.Reason = "成员 limited"
	case providerMemberCooling(member.CooldownUntil, now):
		view.ReasonCode = "member_cooling"
		view.Reason = "成员冷却中"
	case member.MaxConcurrent > 0 && running >= int64(member.MaxConcurrent):
		view.ReasonCode = "member_concurrency_full"
		view.Reason = "成员并发已满"
	default:
		view.Available = true
		view.ReasonCode = "available"
		view.Reason = "可调度"
	}
	return view
}

func businessProviderDispatchPreviewIssues(views []businessProviderDispatchPreviewPool, fallbackOK bool, fallbackErr error) []businessProviderDispatchPreviewIssue {
	counts := map[string]int{}
	for _, pool := range views {
		if pool.ReasonCode == "group_disabled" || pool.ReasonCode == "platform_mismatch" || pool.ReasonCode == "tag_mismatch" {
			counts[pool.ReasonCode]++
		}
		counts["member_disabled"] += pool.DisabledMembers
		counts["member_unavailable"] += pool.UnavailableMembers
		counts["member_limited"] += pool.LimitedMembers
		counts["member_cooling"] += pool.CoolingMembers
		counts["member_concurrency_full"] += pool.ConcurrencyFullMembers
		counts["member_platform_mismatch"] += pool.PlatformMismatchMembers
	}
	labels := []struct {
		code  string
		label string
	}{
		{"tag_mismatch", "标签不匹配的池"},
		{"group_disabled", "池已禁用"},
		{"platform_mismatch", "平台不匹配的池"},
		{"member_disabled", "member 已禁用"},
		{"member_unavailable", "member unavailable"},
		{"member_limited", "member limited"},
		{"member_cooling", "member 冷却中"},
		{"member_concurrency_full", "member 并发已满"},
		{"member_platform_mismatch", "member 平台不匹配"},
	}
	issues := make([]businessProviderDispatchPreviewIssue, 0, len(labels)+1)
	for _, item := range labels {
		if counts[item.code] > 0 {
			issues = append(issues, businessProviderDispatchPreviewIssue{Code: item.code, Label: item.label, Count: counts[item.code]})
		}
	}
	if fallbackErr != nil {
		issues = append(issues, businessProviderDispatchPreviewIssue{
			Code:   "api_fallback_error",
			Label:  "API 兜底异常",
			Count:  1,
			Detail: fallbackErr.Error(),
		})
	} else if !fallbackOK {
		issues = append(issues, businessProviderDispatchPreviewIssue{
			Code:  "no_api_fallback",
			Label: "无 API 兜底配置",
			Count: 1,
		})
	}
	return issues
}

func businessProviderDispatchPreviewTrace(
	views []businessProviderDispatchPreviewPool,
	selection businessproviders.PoolSelection,
	selected bool,
	fallbackCfg imageProviderProxyConfig,
	fallbackOK bool,
	fallbackErr error,
) []string {
	taggedPools := make([]businessProviderDispatchPreviewPool, 0)
	fallbackPools := make([]businessProviderDispatchPreviewPool, 0)
	for _, pool := range views {
		if pool.Role == "tagged" {
			taggedPools = append(taggedPools, pool)
		}
		if pool.Role == "fallback" {
			fallbackPools = append(fallbackPools, pool)
		}
	}
	trace := make([]string, 0, 5)
	if len(taggedPools) > 0 {
		trace = append(trace, fmt.Sprintf("标签命中 %d 个池", len(taggedPools)))
		if selected && selection.Strategy == businessproviders.SelectionStrategyTagged {
			trace = append(trace, "标签池命中成员："+firstNonEmpty(selection.Member.Name, selection.Member.ID))
			return trace
		}
		trace = append(trace, "标签池无可用成员")
	} else {
		trace = append(trace, "没有标签池命中")
	}
	if len(fallbackPools) > 0 {
		trace = append(trace, fmt.Sprintf("fallback 候选池 %d 个", len(fallbackPools)))
		if selected && (selection.Strategy == businessproviders.SelectionStrategyFallback || selection.Strategy == businessproviders.SelectionStrategyTaggedFallback) {
			trace = append(trace, "fallback 池命中成员："+firstNonEmpty(selection.Member.Name, selection.Member.ID))
			return trace
		}
		trace = append(trace, "fallback 池无可用成员")
	} else {
		trace = append(trace, "没有 fallback 池")
	}
	if fallbackErr != nil {
		trace = append(trace, "API 兜底异常："+fallbackErr.Error())
	} else if fallbackOK {
		trace = append(trace, "将走 API 兜底："+firstNonEmpty(fallbackCfg.ProviderName, fallbackCfg.ProviderSource))
	} else {
		trace = append(trace, "无 API 兜底配置")
	}
	return trace
}

func missingDispatchTags(required []string, actual []string) []string {
	actualSet := map[string]struct{}{}
	for _, raw := range actual {
		tag := normalizeProviderDispatchTag(raw)
		if tag != "" {
			actualSet[tag] = struct{}{}
		}
	}
	missing := make([]string, 0)
	for _, raw := range required {
		tag := normalizeProviderDispatchTag(raw)
		if tag == "" {
			continue
		}
		if _, ok := actualSet[tag]; !ok {
			missing = append(missing, tag)
		}
	}
	return missing
}

func anyDispatchTagMatched(required []string, actual []string) bool {
	actualSet := map[string]struct{}{}
	for _, raw := range actual {
		tag := normalizeProviderDispatchTag(raw)
		if tag != "" {
			actualSet[tag] = struct{}{}
		}
	}
	for _, raw := range required {
		tag := normalizeProviderDispatchTag(raw)
		if _, ok := actualSet[tag]; ok {
			return true
		}
	}
	return false
}

func businessProviderDispatchPreviewMessage(strategy string) string {
	switch strategy {
	case businessproviders.SelectionStrategyTagged:
		return "命中标签池成员"
	case businessproviders.SelectionStrategyTaggedFallback:
		return "标签池当前不可用，已回退默认池成员"
	case businessproviders.SelectionStrategyFallback:
		return "命中默认兜底池成员"
	default:
		return "命中号池成员"
	}
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

func decodeBusinessProviderGroupPayload(w http.ResponseWriter, r *http.Request) (businessproviders.GroupInput, bool) {
	var body businessProviderGroupPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return businessproviders.GroupInput{}, false
	}
	input := businessproviders.GroupInput{
		Name:        body.Name,
		Platform:    body.Platform,
		Description: body.Description,
		Tags:        body.Tags,
		MatchMode:   body.MatchMode,
		Enabled:     body.Enabled,
		IsDefault:   body.IsDefault,
		Priority:    body.Priority,
	}
	if strings.TrimSpace(body.Platform) == "" {
		input.Platform = businessproviders.PlatformGPTImage
	}
	return input, true
}

func decodeBusinessProviderMemberPayload(w http.ResponseWriter, r *http.Request) (businessproviders.MemberInput, bool) {
	var body businessProviderMemberPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return businessproviders.MemberInput{}, false
	}
	input := businessproviders.MemberInput{
		GroupID:          body.GroupID,
		Name:             body.Name,
		Platform:         body.Platform,
		BaseURL:          body.BaseURL,
		APIKey:           body.APIKey,
		DefaultModel:     body.DefaultModel,
		Enabled:          body.Enabled,
		Priority:         body.Priority,
		Weight:           body.Weight,
		MaxConcurrent:    body.MaxConcurrent,
		CooldownSeconds:  body.CooldownSeconds,
		FailureThreshold: body.FailureThreshold,
		Status:           body.Status,
	}
	return input, true
}
