package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"imagestudio/internal/businessnotifications"
)

type businessNotificationPayload struct {
	Title      string                          `json:"title"`
	Body       string                          `json:"body"`
	Level      string                          `json:"level"`
	Publish    bool                            `json:"publish"`
	Status     string                          `json:"status"`
	Audience   string                          `json:"audience"`
	NotifyMode string                          `json:"notifyMode"`
	StartsAt   string                          `json:"startsAt"`
	EndsAt     string                          `json:"endsAt"`
	Targeting  businessnotifications.Targeting `json:"targeting"`
}

func (s *Server) handleAdminListBusinessNotifications(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessNotificationStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "notification store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListAdmin(r.Context(), intQuery(r, "limit", 50), businessnotifications.AdminListFilters{
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("search"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminCreateBusinessNotification(w http.ResponseWriter, r *http.Request) {
	var body businessNotificationPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	session, _ := requestAuthSession(r)
	store, err := s.newBusinessNotificationStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "notification store failed"})
		return
	}
	defer store.Close()
	item, err := store.Create(r.Context(), businessnotifications.MutationInput{
		Title:      body.Title,
		Body:       body.Body,
		Level:      body.Level,
		Audience:   body.Audience,
		Publish:    body.Publish,
		Status:     body.Status,
		NotifyMode: body.NotifyMode,
		StartsAt:   body.StartsAt,
		EndsAt:     body.EndsAt,
		Targeting:  body.Targeting,
		CreatedBy:  session.UserID,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleAdminUpdateBusinessNotification(w http.ResponseWriter, r *http.Request) {
	var body businessNotificationPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessNotificationStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "notification store failed"})
		return
	}
	defer store.Close()
	item, ok, err := store.Update(r.Context(), r.PathValue("id"), businessnotifications.MutationInput{
		Title:      body.Title,
		Body:       body.Body,
		Level:      body.Level,
		Audience:   body.Audience,
		Publish:    body.Publish,
		Status:     body.Status,
		NotifyMode: body.NotifyMode,
		StartsAt:   body.StartsAt,
		EndsAt:     body.EndsAt,
		Targeting:  body.Targeting,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "notification not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleAdminDeleteBusinessNotification(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessNotificationStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "notification store failed"})
		return
	}
	defer store.Close()
	deleted, err := store.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "notification not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListBusinessNotifications(w http.ResponseWriter, r *http.Request) {
	userID := businessUserIDForRequest(r)
	store, err := s.newBusinessNotificationStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "notification store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListForUser(r.Context(), userID, intQuery(r, "limit", 20))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	unreadCount, err := store.UnreadCount(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "unreadCount": unreadCount})
}

func (s *Server) handleMarkBusinessNotificationsRead(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessNotificationStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "notification store failed"})
		return
	}
	defer store.Close()
	if err := store.MarkRead(r.Context(), businessUserIDForRequest(r), body.IDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
