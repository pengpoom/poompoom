package api

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleBusinessTrackerSummary(w http.ResponseWriter, r *http.Request) {
	windowSeconds := int64(600)
	if raw := strings.TrimSpace(r.URL.Query().Get("windowSeconds")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			windowSeconds = parsed
		}
	}
	store, err := s.newBusinessTrackerStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "tracker store failed"})
		return
	}
	defer store.Close()
	summary, err := store.Summary(r.Context(), windowSeconds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
