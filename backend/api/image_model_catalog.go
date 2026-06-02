package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"imagestudio/internal/businessmodels"
	"imagestudio/internal/businessproviders"
)

type imageModelCatalogResponse struct {
	Items []businessmodels.Model `json:"items"`
}

type imageModelMutationPayload struct {
	ID             string                      `json:"id"`
	Vendor         string                      `json:"vendor"`
	VendorLabel    string                      `json:"vendorLabel"`
	DisplayName    string                      `json:"displayName"`
	Adapter        string                      `json:"adapter"`
	Platform       string                      `json:"platform"`
	UpstreamModel  string                      `json:"upstreamModel"`
	Enabled        bool                        `json:"enabled"`
	Preview        bool                        `json:"preview"`
	CompareEnabled bool                        `json:"compareEnabled"`
	Capabilities   businessmodels.Capabilities `json:"capabilities"`
	CreditCost     int64                       `json:"creditCost"`
	IsDefault      bool                        `json:"isDefault"`
	SortOrder      int                         `json:"sortOrder"`
}

func (s *Server) newBusinessModelStore() (*businessmodels.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businessmodels.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return businessmodels.NewStore(s.cfg)
}

func (s *Server) handleListImageModels(w http.ResponseWriter, r *http.Request) {
	includeDisabled := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeDisabled")), "true")
	items, err := s.businessImageModels(r.Context(), includeDisabled)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !includeDisabled {
		filtered := make([]businessmodels.Model, 0, len(items))
		for _, item := range items {
			if item.Enabled && item.Availability.Available {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, imageModelCatalogResponse{Items: items})
}

func (s *Server) handleAdminListImageModels(w http.ResponseWriter, r *http.Request) {
	items, err := s.businessImageModels(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, imageModelCatalogResponse{Items: items})
}

func (s *Server) handleAdminCreateImageModel(w http.ResponseWriter, r *http.Request) {
	var body imageModelMutationPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessModelStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "model store failed"})
		return
	}
	defer store.Close()
	item, err := store.Create(r.Context(), body.toInput())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleAdminUpdateImageModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body imageModelMutationPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessModelStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "model store failed"})
		return
	}
	defer store.Close()
	item, err := store.Update(r.Context(), id, body.toInput())
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "model not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleAdminSetDefaultImageModel(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessModelStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "model store failed"})
		return
	}
	defer store.Close()
	item, ok, err := store.SetDefault(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "model not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleAdminDeleteImageModel(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessModelStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "model store failed"})
		return
	}
	defer store.Close()
	deleted, err := store.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "model not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminTestImageModel(w http.ResponseWriter, r *http.Request) {
	item, ok, err := s.businessImageModelByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "model not found"})
		return
	}
	if !item.Enabled {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "disabled model cannot be tested"})
		return
	}
	providerRequest := []string{
		item.Platform,
		"model:" + normalizeDispatchTagPart(item.UpstreamModel, defaultModelForProviderPlatform(item.Platform)),
	}
	providerCfg, err := s.imageProviderProxyConfig(providerRequest...)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "availability": item.Availability})
		return
	}
	providerCfg.Model = item.UpstreamModel
	s.writeBusinessProviderTestResult(w, r, providerCfg, map[string]any{
		"model":        item,
		"availability": item.Availability,
	})
}

func (body imageModelMutationPayload) toInput() businessmodels.MutationInput {
	return businessmodels.MutationInput{
		ID:             body.ID,
		Vendor:         body.Vendor,
		VendorLabel:    body.VendorLabel,
		DisplayName:    body.DisplayName,
		Adapter:        body.Adapter,
		Platform:       body.Platform,
		UpstreamModel:  body.UpstreamModel,
		Enabled:        body.Enabled,
		Preview:        body.Preview,
		CompareEnabled: body.CompareEnabled,
		Capabilities:   body.Capabilities,
		CreditCost:     body.CreditCost,
		IsDefault:      body.IsDefault,
		SortOrder:      body.SortOrder,
	}
}

func (s *Server) businessImageModels(ctx context.Context, includeDisabled bool) ([]businessmodels.Model, error) {
	store, err := s.newBusinessModelStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	items, err := store.List(ctx, includeDisabled)
	if err != nil {
		return nil, err
	}
	return s.attachBusinessImageModelAvailability(ctx, items)
}

func (s *Server) businessImageModelByID(ctx context.Context, id string) (businessmodels.Model, bool, error) {
	store, err := s.newBusinessModelStore()
	if err != nil {
		return businessmodels.Model{}, false, err
	}
	defer store.Close()
	item, ok, err := store.Get(ctx, id)
	if err != nil || !ok {
		return item, ok, err
	}
	items, err := s.attachBusinessImageModelAvailability(ctx, []businessmodels.Model{item})
	if err != nil || len(items) == 0 {
		return item, true, err
	}
	return items[0], true, nil
}

func (s *Server) attachBusinessImageModelAvailability(ctx context.Context, items []businessmodels.Model) ([]businessmodels.Model, error) {
	if len(items) == 0 {
		return items, nil
	}
	store, err := s.newBusinessProviderStore()
	if err != nil {
		for index := range items {
			items[index].Availability = businessmodels.Availability{
				Status:    "unknown",
				Available: false,
				Message:   "无法读取上游配置",
				Issues:    []string{err.Error()},
			}
		}
		return items, nil
	}
	defer store.Close()
	if err := store.EnsureConfigProvider(ctx, s.cfg); err != nil {
		for index := range items {
			items[index].Availability = businessmodels.Availability{
				Status:    "unknown",
				Available: false,
				Message:   "无法同步旧 API 接入配置",
				Issues:    []string{err.Error()},
			}
		}
		return items, nil
	}
	for index := range items {
		items[index].Availability = s.businessImageModelAvailability(ctx, store, items[index])
	}
	return items, nil
}

func (s *Server) businessImageModelAvailability(ctx context.Context, store *businessproviders.Store, item businessmodels.Model) businessmodels.Availability {
	platform := businessproviders.NormalizePlatform(item.Platform)
	availability := businessmodels.Availability{
		Status:    "unavailable",
		Available: false,
		Message:   "没有可用上游配置",
		Issues:    []string{},
	}
	if !item.Enabled {
		availability.Status = "disabled"
		availability.Message = "模型已停用"
		availability.Issues = append(availability.Issues, "模型已停用，工作台不会展示")
		return availability
	}
	if platform == "" {
		availability.Message = "平台不受支持"
		availability.Issues = append(availability.Issues, "模型平台不受支持")
		return availability
	}
	if platform == businessproviders.PlatformGeminiBanana && item.Adapter != businessmodels.AdapterGemini {
		availability.Issues = append(availability.Issues, "Google 平台通常应使用 Gemini 适配器")
	}
	if platform != businessproviders.PlatformGeminiBanana && item.Adapter == businessmodels.AdapterGemini {
		availability.Issues = append(availability.Issues, "非 Google 平台通常不应使用 Gemini 适配器")
	}
	if provider, ok, err := store.DefaultForPlatform(ctx, platform); err == nil && ok {
		availability.APIProviderAvailable = true
		availability.APIProviderName = provider.Name
		availability.APIProviderDefaultModel = provider.DefaultModel
		if strings.TrimSpace(provider.DefaultModel) != "" && !strings.EqualFold(provider.DefaultModel, item.UpstreamModel) {
			availability.APIProviderModelMismatch = true
			availability.Issues = append(availability.Issues, "API 接入默认模型与模型目录不同，生图会按模型目录覆盖")
		}
	} else if err != nil {
		availability.Issues = append(availability.Issues, "读取 API 接入失败："+err.Error())
	}
	if selection, ok, err := store.SelectMember(ctx, businessproviders.PoolSelectionPolicy{
		Platform: platform,
		Tags:     []string{"model:" + normalizeDispatchTagPart(item.UpstreamModel, defaultModelForProviderPlatform(platform))},
	}); err == nil && ok {
		availability.PoolAvailable = true
		availability.PoolName = selection.Group.Name
		availability.PoolMemberID = selection.Member.ID
		availability.PoolMemberName = selection.Member.Name
		availability.PoolMemberDefaultModel = selection.Member.DefaultModel
		if strings.TrimSpace(selection.Member.DefaultModel) != "" && !strings.EqualFold(selection.Member.DefaultModel, item.UpstreamModel) {
			availability.PoolMemberModelMismatch = true
			availability.Issues = append(availability.Issues, "号池成员默认模型与模型目录不同，生图会按模型目录覆盖")
		}
	} else if err != nil {
		availability.Issues = append(availability.Issues, "读取号池失败："+err.Error())
	}
	availability.Available = availability.APIProviderAvailable || availability.PoolAvailable
	switch {
	case availability.PoolAvailable && availability.APIProviderAvailable:
		availability.Status = "pool_and_api"
		availability.Message = "可用：号池可调度，API 接入可兜底"
	case availability.PoolAvailable:
		availability.Status = "pool"
		availability.Message = "可用：号池可调度"
	case availability.APIProviderAvailable:
		availability.Status = "api_fallback"
		availability.Message = "可用：仅 API 接入兜底"
	default:
		availability.Status = "unavailable"
		availability.Message = "不可用：没有启用的 API 接入或可调度号池成员"
		availability.Issues = append(availability.Issues, "请在上游管理中配置该平台的 API 接入或号池成员")
	}
	return availability
}

func (s *Server) resolveBusinessImageModel(ctx context.Context, payload map[string]any) (businessmodels.Model, bool) {
	if payload == nil {
		return businessmodels.Model{}, false
	}
	store, err := s.newBusinessModelStore()
	if err != nil {
		return fallbackBusinessImageModel(payload)
	}
	defer store.Close()
	if item, ok, err := store.Get(ctx, businessmodels.NormalizeModelID(stringValue(payload["modelId"]))); err == nil && ok {
		return item, true
	}
	platform := businessproviders.NormalizePlatform(stringValue(payload["platform"]))
	model := strings.TrimSpace(stringValue(payload["model"]))
	if item, ok, err := store.FindByPlatformModel(ctx, platform, model); err == nil && ok {
		return item, true
	}
	return fallbackBusinessImageModel(payload)
}

func applyBusinessImageModelPayload(payload map[string]any, item businessmodels.Model) {
	if payload == nil || item.ID == "" {
		return
	}
	payload["modelId"] = item.ID
	payload["model"] = item.UpstreamModel
	payload["modelLabel"] = item.DisplayName
	payload["vendor"] = item.Vendor
	payload["vendorLabel"] = item.VendorLabel
	payload["adapter"] = item.Adapter
	payload["platform"] = item.Platform
	payload["creditCost"] = item.CreditCost
}

func providerImageGenerateMetadataFromModel(payload map[string]any, item businessmodels.Model) providerImageGenerateMetadata {
	metadata := extractProviderImageGenerateMetadata(payload)
	if item.ID != "" {
		metadata.ModelID = item.ID
	}
	if item.DisplayName != "" {
		metadata.ModelLabel = item.DisplayName
	}
	if item.Vendor != "" {
		metadata.Vendor = item.Vendor
	}
	if item.VendorLabel != "" {
		metadata.VendorLabel = item.VendorLabel
	}
	if item.Platform != "" {
		metadata.Platform = item.Platform
	}
	if item.CreditCost > 0 {
		metadata.CreditCost = item.CreditCost
	}
	return metadata
}

func fallbackBusinessImageModel(payload map[string]any) (businessmodels.Model, bool) {
	modelID := businessmodels.NormalizeModelID(stringValue(payload["modelId"]))
	platform := businessproviders.NormalizePlatform(stringValue(payload["platform"]))
	model := strings.TrimSpace(stringValue(payload["model"]))
	for _, input := range businessmodels.DefaultModels() {
		item := fallbackModelFromInput(input)
		if modelID != "" && businessmodels.NormalizeModelID(item.ID) == modelID {
			return item, true
		}
		if model != "" && item.Platform == platform && strings.EqualFold(item.UpstreamModel, model) {
			return item, true
		}
	}
	if model != "" {
		for _, input := range businessmodels.DefaultModels() {
			item := fallbackModelFromInput(input)
			if strings.EqualFold(item.UpstreamModel, model) {
				return item, true
			}
		}
	}
	return businessmodels.Model{}, false
}

func fallbackModelFromInput(input businessmodels.MutationInput) businessmodels.Model {
	return businessmodels.Model{
		ID:             input.ID,
		Vendor:         input.Vendor,
		VendorLabel:    input.VendorLabel,
		DisplayName:    input.DisplayName,
		Adapter:        input.Adapter,
		Platform:       input.Platform,
		UpstreamModel:  input.UpstreamModel,
		Enabled:        input.Enabled,
		Preview:        input.Preview,
		CompareEnabled: input.CompareEnabled,
		Capabilities:   input.Capabilities,
		CreditCost:     input.CreditCost,
		IsDefault:      input.IsDefault,
		SortOrder:      input.SortOrder,
	}
}
