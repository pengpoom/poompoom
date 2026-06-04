package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"imagestudio/internal/riskcontrol"
)

func (s *Server) newRiskControlStore() (*riskcontrol.Store, error) {
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return riskcontrol.NewStoreWithDB(s.db, s.cfg.Database.Driver), nil
	}
	return riskcontrol.NewStore(s.cfg)
}

func (s *Server) handleGetRiskControlConfig(w http.ResponseWriter, r *http.Request) {
	store, err := s.newRiskControlStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "risk_control_store_failed", err.Error())
		return
	}
	defer store.Close()
	cfg, err := store.GetConfig(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "risk_control_config_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg.View()})
}

func (s *Server) handleUpdateRiskControlConfig(w http.ResponseWriter, r *http.Request) {
	var body riskcontrol.UpdateConfigInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	store, err := s.newRiskControlStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "risk_control_store_failed", err.Error())
		return
	}
	defer store.Close()
	cfg, err := store.UpdateConfig(r.Context(), body)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "risk_control_config_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg.View()})
}

func (s *Server) handleGetRiskControlStatus(w http.ResponseWriter, r *http.Request) {
	store, err := s.newRiskControlStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "risk_control_store_failed", err.Error())
		return
	}
	defer store.Close()
	status, err := store.Status(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "risk_control_status_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}

func (s *Server) handleListRiskControlLogs(w http.ResponseWriter, r *http.Request) {
	page := intQuery(r, "page", 1)
	pageSize := intQuery(r, "pageSize", 50)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	filter := riskcontrol.ListFilter{
		Result:   r.URL.Query().Get("result"),
		Platform: r.URL.Query().Get("platform"),
		Search:   r.URL.Query().Get("search"),
		From:     r.URL.Query().Get("from"),
		To:       r.URL.Query().Get("to"),
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	}
	store, err := s.newRiskControlStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "risk_control_store_failed", err.Error())
		return
	}
	defer store.Close()
	items, total, err := store.ListLogs(r.Context(), filter)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "risk_control_logs_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"pages":    pagesForTotal(total, pageSize),
	})
}

func (s *Server) handleTestRiskControl(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt       string              `json:"prompt"`
		Mode         *string             `json:"mode"`
		BaseURL      *string             `json:"baseUrl"`
		APIKey       *string             `json:"apiKey"`
		ClearAPIKey  bool                `json:"clearApiKey"`
		Model        *string             `json:"model"`
		TimeoutMS    *int                `json:"timeoutMs"`
		BlockMessage *string             `json:"blockMessage"`
		Thresholds   *map[string]float64 `json:"thresholds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	store, err := s.newRiskControlStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "risk_control_store_failed", err.Error())
		return
	}
	defer store.Close()
	service := riskcontrol.NewService(store)
	result, err := service.Test(r.Context(), body.Prompt, riskcontrol.TestConfigOverride{
		Mode:         body.Mode,
		BaseURL:      body.BaseURL,
		APIKey:       body.APIKey,
		ClearAPIKey:  body.ClearAPIKey,
		Model:        body.Model,
		TimeoutMS:    body.TimeoutMS,
		BlockMessage: body.BlockMessage,
		Thresholds:   body.Thresholds,
	})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "risk_control_test_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func pagesForTotal(total int64, pageSize int) int64 {
	if pageSize <= 0 {
		return 0
	}
	pages := total / int64(pageSize)
	if total%int64(pageSize) != 0 {
		pages++
	}
	return pages
}
