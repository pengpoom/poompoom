package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"imagestudio/internal/businessapikeys"
	"imagestudio/internal/businesssettings"
)

const maxSelfServeAPIKeys = 5

// currentUserWithAPIAccess 返回当前登录用户 ID，以及其 api_access_enabled 是否为真。
func (s *Server) currentUserWithAPIAccess(r *http.Request) (userID string, enabled bool, err error) {
	session, ok := requestAuthSession(r)
	if !ok {
		return "", false, nil
	}
	store, err := s.newBusinessAuthStore()
	if err != nil {
		return "", false, err
	}
	defer store.Close()
	on, err := store.IsUserAPIAccessEnabled(r.Context(), session.UserID)
	if err != nil {
		return "", false, err
	}
	return session.UserID, on, nil
}

// guardSelfServe 统一处理「未登录 / 未开通 / store 故障」三种短路；返回 ("", false) 表示已写响应、调用方应 return。
func (s *Server) guardSelfServe(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, enabled, err := s.currentUserWithAPIAccess(r)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "store_unavailable", "user store failed")
		return "", false
	}
	if userID == "" {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "authorization is invalid")
		return "", false
	}
	if !enabled {
		writeAPIError(w, http.StatusForbidden, "api_access_disabled", "API access is not enabled for this account")
		return "", false
	}
	return userID, true
}

func (s *Server) handleListMyAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.guardSelfServe(w, r)
	if !ok {
		return
	}
	store, err := s.newBusinessAPIKeyStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "store_unavailable", "api key store failed")
		return
	}
	defer store.Close()
	keys, err := store.ListByUser(r.Context(), userID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	baseURL := businesssettings.ResolveAPIBaseURL(s.businessSystemSettings(r), s.cfg.ExternalAPI.BaseURL)
	writeJSON(w, http.StatusOK, map[string]any{"items": keys, "baseUrl": baseURL})
}

func (s *Server) handleCreateMyAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.guardSelfServe(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	store, err := s.newBusinessAPIKeyStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "store_unavailable", "api key store failed")
		return
	}
	defer store.Close()
	existing, err := store.ListByUser(r.Context(), userID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	active := 0
	for _, k := range existing {
		if k.Status != businessapikeys.StatusRevoked {
			active++
		}
	}
	if active >= maxSelfServeAPIKeys {
		writeAPIError(w, http.StatusBadRequest, "limit_reached", "too many API keys")
		return
	}
	key, plaintext, err := store.Create(r.Context(), businessapikeys.CreateInput{
		UserID: userID,
		Name:   body.Name,
		Env:    businessapikeys.EnvLive,
	})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": key, "secret": plaintext})
}

func (s *Server) handleUpdateMyAPIKeyStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.guardSelfServe(w, r)
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	status := strings.TrimSpace(body.Status)
	if status != businessapikeys.StatusActive && status != businessapikeys.StatusDisabled {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "status must be active or disabled")
		return
	}
	id := r.PathValue("id")
	store, err := s.newBusinessAPIKeyStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "store_unavailable", "api key store failed")
		return
	}
	defer store.Close()
	existing, err := store.GetByID(r.Context(), id)
	if err != nil || existing.UserID != userID {
		writeAPIError(w, http.StatusNotFound, "not_found", "api key not found")
		return
	}
	if existing.Status == businessapikeys.StatusRevoked {
		writeAPIError(w, http.StatusBadRequest, "invalid_state", "revoked key cannot be toggled")
		return
	}
	key, err := store.Update(r.Context(), id, businessapikeys.UpdateInput{Status: &status})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": key})
}

func (s *Server) handleDeleteMyAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.guardSelfServe(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	store, err := s.newBusinessAPIKeyStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "store_unavailable", "api key store failed")
		return
	}
	defer store.Close()
	existing, err := store.GetByID(r.Context(), id)
	if err != nil || existing.UserID != userID {
		writeAPIError(w, http.StatusNotFound, "not_found", "api key not found")
		return
	}
	if err := store.Delete(r.Context(), id); err != nil {
		writeAPIError(w, http.StatusBadRequest, "delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRevokeMyAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.guardSelfServe(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	store, err := s.newBusinessAPIKeyStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "store_unavailable", "api key store failed")
		return
	}
	defer store.Close()
	existing, err := store.GetByID(r.Context(), id)
	if err != nil || existing.UserID != userID {
		writeAPIError(w, http.StatusNotFound, "not_found", "api key not found")
		return
	}
	key, err := store.Revoke(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "revoke_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": key})
}
