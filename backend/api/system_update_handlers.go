package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultUpdaterURL = "http://updater:7071"

type systemUpdateJob struct {
	ID         string   `json:"id,omitempty"`
	Status     string   `json:"status"`
	Phase      string   `json:"phase,omitempty"`
	Error      string   `json:"error,omitempty"`
	Logs       []string `json:"logs,omitempty"`
	StartedAt  string   `json:"started_at,omitempty"`
	FinishedAt string   `json:"finished_at,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

type systemUpdateStatus struct {
	Enabled        bool             `json:"enabled"`
	Reason         string           `json:"reason,omitempty"`
	DeployDir      string           `json:"deploy_dir,omitempty"`
	ComposeFile    string           `json:"compose_file,omitempty"`
	ComposeProject string           `json:"compose_project,omitempty"`
	Service        string           `json:"service,omitempty"`
	Job            *systemUpdateJob `json:"job,omitempty"`
}

type systemUpdateStartResponse struct {
	Started bool                `json:"started"`
	Job     *systemUpdateJob    `json:"job,omitempty"`
	Status  *systemUpdateStatus `json:"status,omitempty"`
}

func (s *Server) handleSystemUpdateStatus(w http.ResponseWriter, r *http.Request) {
	status := s.fetchSystemUpdateStatus(r.Context())
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleSystemUpdateStart(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(systemUpdaterToken()) == "" {
		status := s.systemUpdateUnavailable("IMAGE_STUDIO_UPDATER_TOKEN is not configured")
		s.setSystemUpdateSnapshot(status)
		writeSystemUpdateUnavailable(w, status)
		return
	}

	var payload systemUpdateStartResponse
	statusCode, err := callUpdaterJSON(r.Context(), http.MethodPost, "/update", nil, &payload)
	if err != nil {
		status := s.systemUpdateUnavailable(err.Error())
		s.setSystemUpdateSnapshot(status)
		writeSystemUpdateUnavailable(w, status)
		return
	}
	if payload.Status != nil {
		s.setSystemUpdateSnapshot(*payload.Status)
	}
	if payload.Status == nil && payload.Job != nil {
		s.setSystemUpdateSnapshot(systemUpdateStatus{
			Enabled: true,
			Job:     payload.Job,
		})
	}
	writeJSON(w, statusCode, payload)
}

func (s *Server) fetchSystemUpdateStatus(ctx context.Context) systemUpdateStatus {
	if strings.TrimSpace(systemUpdaterToken()) == "" {
		status := s.systemUpdateUnavailable("IMAGE_STUDIO_UPDATER_TOKEN is not configured")
		s.setSystemUpdateSnapshot(status)
		return status
	}

	var status systemUpdateStatus
	_, err := callUpdaterJSON(ctx, http.MethodGet, "/status", nil, &status)
	if err != nil {
		cached, ok := s.getSystemUpdateSnapshot()
		if ok {
			cached.Enabled = false
			cached.Reason = err.Error()
			return cached
		}
		status := s.systemUpdateUnavailable(err.Error())
		s.setSystemUpdateSnapshot(status)
		return status
	}
	s.setSystemUpdateSnapshot(status)
	return status
}

func (s *Server) systemUpdateUnavailable(reason string) systemUpdateStatus {
	return systemUpdateStatus{
		Enabled: false,
		Reason:  reason,
	}
}

func writeSystemUpdateUnavailable(w http.ResponseWriter, status systemUpdateStatus) {
	reason := strings.TrimSpace(status.Reason)
	if reason == "" {
		reason = "system update is unavailable"
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"enabled": false,
		"reason":  reason,
		"message": reason,
		"error":   reason,
	})
}

func (s *Server) getSystemUpdateSnapshot() (systemUpdateStatus, bool) {
	s.systemUpdateMu.RLock()
	defer s.systemUpdateMu.RUnlock()
	if s.systemUpdateSnapshot == nil {
		return systemUpdateStatus{}, false
	}
	return *s.systemUpdateSnapshot, true
}

func (s *Server) setSystemUpdateSnapshot(status systemUpdateStatus) {
	s.systemUpdateMu.Lock()
	defer s.systemUpdateMu.Unlock()
	copyStatus := status
	s.systemUpdateSnapshot = &copyStatus
}

func callUpdaterJSON(ctx context.Context, method, path string, body any, target any) (int, error) {
	updaterURL := strings.TrimRight(apiEnvString("IMAGE_STUDIO_UPDATER_URL", defaultUpdaterURL), "/")
	if updaterURL == "" {
		return http.StatusServiceUnavailable, fmt.Errorf("IMAGE_STUDIO_UPDATER_URL is not configured")
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		reader = bytes.NewReader(data)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, updaterURL+path, reader)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+systemUpdaterToken())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if len(data) > 0 && target != nil {
		if err := json.Unmarshal(data, target); err != nil {
			return resp.StatusCode, err
		}
	}
	if resp.StatusCode >= 400 {
		return resp.StatusCode, errors.New(updaterErrorMessage(data, resp.StatusCode))
	}
	return resp.StatusCode, nil
}

func updaterErrorMessage(data []byte, status int) string {
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err == nil && strings.TrimSpace(payload.Error) != "" {
		return payload.Error
	}
	return fmt.Sprintf("updater request failed with status %d", status)
}

func systemUpdaterToken() string {
	return strings.TrimSpace(os.Getenv("IMAGE_STUDIO_UPDATER_TOKEN"))
}

func apiEnvString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
