package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const maintenanceModeMessage = "系统正在升级维护，请稍等几分钟后重新提交"

type maintenanceStatusResponse struct {
	Enabled   bool   `json:"enabled"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	UpdatedBy string `json:"updatedBy,omitempty"`
}

func (s *Server) handleMaintenanceStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.maintenanceStatus())
}

func (s *Server) handleMaintenanceUpdate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	session, _ := requestAuthSession(r)
	writeJSON(w, http.StatusOK, s.setMaintenanceMode(payload.Enabled, session.Username))
}

func (s *Server) maintenanceStatus() maintenanceStatusResponse {
	s.maintenanceMu.RLock()
	defer s.maintenanceMu.RUnlock()
	return s.maintenanceStatusLocked()
}

func (s *Server) setMaintenanceMode(enabled bool, updatedBy string) maintenanceStatusResponse {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	if s.maintenanceMode != enabled {
		s.maintenanceUpdatedAt = time.Now().UTC()
	}
	s.maintenanceMode = enabled
	s.maintenanceUpdatedBy = strings.TrimSpace(updatedBy)
	if s.maintenanceUpdatedAt.IsZero() {
		s.maintenanceUpdatedAt = time.Now().UTC()
	}
	return s.maintenanceStatusLocked()
}

func (s *Server) maintenanceStatusLocked() maintenanceStatusResponse {
	status := maintenanceStatusResponse{
		Enabled: s.maintenanceMode,
		Message: maintenanceModeMessage,
	}
	if !s.maintenanceUpdatedAt.IsZero() {
		status.UpdatedAt = s.maintenanceUpdatedAt.Format(time.RFC3339Nano)
	}
	status.UpdatedBy = strings.TrimSpace(s.maintenanceUpdatedBy)
	return status
}

func (s *Server) maintenanceModeEnabled() bool {
	s.maintenanceMu.RLock()
	defer s.maintenanceMu.RUnlock()
	return s.maintenanceMode
}

func (s *Server) rejectIfMaintenanceMode(w http.ResponseWriter) bool {
	if !s.maintenanceModeEnabled() {
		return false
	}
	writeAPIError(w, http.StatusServiceUnavailable, "image_maintenance_mode", maintenanceModeMessage)
	return true
}
