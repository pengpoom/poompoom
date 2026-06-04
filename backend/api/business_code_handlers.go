package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"imagestudio/internal/businesscodes"
)

type businessCodePayload struct {
	Code      string `json:"code"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Credits   int64  `json:"credits"`
	MaxUses   int    `json:"maxUses"`
	Status    string `json:"status"`
	StartsAt  string `json:"startsAt"`
	ExpiresAt string `json:"expiresAt"`
	Note      string `json:"note"`
}

type businessCodeBatchStatusPayload struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

func (s *Server) handleAdminListBusinessCodes(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer store.Close()
	items, err := store.List(r.Context(), intQuery(r, "limit", 100), businesscodes.ListFilters{
		Type:   r.URL.Query().Get("type"),
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("search"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminCreateBusinessCode(w http.ResponseWriter, r *http.Request) {
	var body businessCodePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	session, _ := requestAuthSession(r)
	store, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer store.Close()
	item, rawCode, err := store.Create(r.Context(), businesscodes.CreateInput{
		Code:      body.Code,
		Type:      body.Type,
		Title:     body.Title,
		Credits:   body.Credits,
		MaxUses:   body.MaxUses,
		Status:    body.Status,
		StartsAt:  body.StartsAt,
		ExpiresAt: body.ExpiresAt,
		CreatedBy: session.UserID,
		Note:      body.Note,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": businessCodeErrorMessage(err)})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item, "code": rawCode})
}

func (s *Server) handleAdminUpdateBusinessCode(w http.ResponseWriter, r *http.Request) {
	var body businessCodePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer store.Close()
	item, ok, err := store.Update(r.Context(), r.PathValue("id"), businesscodes.UpdateInput{
		Title:     body.Title,
		Credits:   body.Credits,
		MaxUses:   body.MaxUses,
		Status:    body.Status,
		StartsAt:  body.StartsAt,
		ExpiresAt: body.ExpiresAt,
		Note:      body.Note,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": businessCodeErrorMessage(err)})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "code not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleAdminBatchUpdateBusinessCodeStatus(w http.ResponseWriter, r *http.Request) {
	var body businessCodeBatchStatusPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if len(body.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "ids are required"})
		return
	}
	store, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer store.Close()
	updated, err := store.UpdateStatusBatch(r.Context(), body.IDs, body.Status)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": businessCodeErrorMessage(err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": updated})
}

func (s *Server) handleAdminDeleteBusinessCode(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer store.Close()
	deleted, err := store.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "code not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminListBusinessCodeUsages(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListUsages(r.Context(), r.PathValue("id"), intQuery(r, "limit", 100))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminListBusinessAffiliateReferrals(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListAffiliateReferrals(r.Context(), businesscodes.AffiliateReferralFilters{}, intQuery(r, "limit", 100))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleRedeemBusinessCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	codeStore, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer codeStore.Close()
	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	userID := businessUserIDForRequest(r)
	result, err := codeStore.ConsumeRedeemCode(r.Context(), body.Code, userID, func(ctx context.Context, tx *sql.Tx, userID string, amount int64, reason string) (string, error) {
		entry, err := creditStore.AddWithTx(ctx, tx, userID, amount, reason, false)
		if err != nil {
			return "", err
		}
		return entry.ID, nil
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": businessCodeErrorMessage(err)})
		return
	}
	credit, err := creditStore.Summary(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result, "credit": credit})
}

func (s *Server) handleGetBusinessAffiliateSummary(w http.ResponseWriter, r *http.Request) {
	settings := s.businessSystemSettingsForContext(r.Context())
	store, err := s.newBusinessCodeStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "code store failed"})
		return
	}
	defer store.Close()
	summary, err := store.AffiliateSummary(
		r.Context(),
		businessUserIDForRequest(r),
		settings.Affiliate.Enabled,
		settings.Affiliate.RegistrationRewardEnabled,
		settings.Affiliate.RegistrationRewardCredits,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func businessCodeErrorMessage(err error) string {
	switch {
	case errors.Is(err, businesscodes.ErrCodeInvalid):
		return "码无效"
	case errors.Is(err, businesscodes.ErrCodeInactive):
		return "码未生效或已失效"
	case errors.Is(err, businesscodes.ErrCodeExhausted):
		return "码已用完"
	case errors.Is(err, businesscodes.ErrCodeUsed):
		return "码已使用"
	case errors.Is(err, businesscodes.ErrCodeWrongType):
		return "码类型不匹配"
	case errors.Is(err, businesscodes.ErrCreditsInvalid):
		return "点数必须大于等于 0"
	default:
		return err.Error()
	}
}
