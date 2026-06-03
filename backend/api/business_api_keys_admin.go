package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"imagestudio/internal/businessapikeys"
)

func (s *Server) newBusinessAPIKeyStore() (*businessapikeys.Store, error) {
	secret := strings.TrimSpace(s.cfg.ExternalAPI.SigningSecret)
	if s != nil && s.db != nil && strings.EqualFold(strings.TrimSpace(s.cfg.Database.Driver), "postgres") {
		return businessapikeys.NewStoreWithDB(s.db, s.cfg.Database.Driver, secret), nil
	}
	return businessapikeys.NewStore(s.cfg)
}

type apiKeyCreatePayload struct {
	UserID             string   `json:"userId"`
	Name               string   `json:"name"`
	Env                string   `json:"env"`
	CreditLimit        int64    `json:"creditLimit"`
	RateLimitPerMinute int      `json:"rateLimitPerMinute"`
	ConcurrencyLimit   int      `json:"concurrencyLimit"`
	AllowedModels      []string `json:"allowedModels"`
}

type apiKeyUpdatePayload struct {
	Name               *string   `json:"name"`
	Status             *string   `json:"status"`
	CreditLimit        *int64    `json:"creditLimit"`
	RateLimitPerMinute *int      `json:"rateLimitPerMinute"`
	ConcurrencyLimit   *int      `json:"concurrencyLimit"`
	AllowedModels      *[]string `json:"allowedModels"`
}

func (s *Server) handleAdminListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "userId is required")
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
	writeJSON(w, http.StatusOK, map[string]any{"items": keys})
}

func (s *Server) handleAdminCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var body apiKeyCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	if strings.TrimSpace(body.UserID) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "userId is required")
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
	key, plaintext, err := store.Create(r.Context(), businessapikeys.CreateInput{
		UserID:             body.UserID,
		Name:               body.Name,
		Env:                body.Env,
		CreditLimit:        body.CreditLimit,
		RateLimitPerMinute: body.RateLimitPerMinute,
		ConcurrencyLimit:   body.ConcurrencyLimit,
		AllowedModels:      body.AllowedModels,
	})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	// secret 仅此一次返回，绝不在后续查询接口回显。
	writeJSON(w, http.StatusOK, map[string]any{"item": key, "secret": plaintext})
}

func (s *Server) handleAdminUpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body apiKeyUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	store, err := s.newBusinessAPIKeyStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "store_unavailable", "api key store failed")
		return
	}
	defer store.Close()
	key, err := store.Update(r.Context(), id, businessapikeys.UpdateInput{
		Name:               body.Name,
		Status:             body.Status,
		CreditLimit:        body.CreditLimit,
		RateLimitPerMinute: body.RateLimitPerMinute,
		ConcurrencyLimit:   body.ConcurrencyLimit,
		AllowedModels:      body.AllowedModels,
	})
	if err == sql.ErrNoRows {
		writeAPIError(w, http.StatusNotFound, "not_found", "api key not found")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": key})
}

func (s *Server) handleAdminRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store, err := s.newBusinessAPIKeyStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "store_unavailable", "api key store failed")
		return
	}
	defer store.Close()
	key, err := store.Revoke(r.Context(), id)
	if err == sql.ErrNoRows {
		writeAPIError(w, http.StatusNotFound, "not_found", "api key not found")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "revoke_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": key})
}
