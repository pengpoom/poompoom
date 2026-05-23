package api

import (
	"context"
	"encoding/json"
	"net/http"

	"imagestudio/internal/businesssettings"
)

func (s *Server) handleGetPublicSiteSettings(w http.ResponseWriter, r *http.Request) {
	settings := businesssettings.Normalize(s.businessSystemSettingsForContext(r.Context()))
	writeJSON(w, http.StatusOK, map[string]any{
		"site": map[string]any{
			"name":     settings.Site.Name,
			"subtitle": settings.Site.Subtitle,
			"logoUrl":  settings.Site.LogoURL,
		},
	})
}

func (s *Server) handleGetBusinessSystemSettings(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessSettingsStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "settings store failed"})
		return
	}
	defer store.Close()
	settings, found, err := store.GetWithFound(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if found {
		s.applyBusinessRuntimeSettings(settings)
	} else {
		settings = businesssettings.WithConfigRuntime(settings, s.cfg)
	}
	writeJSON(w, http.StatusOK, businesssettings.Response{
		Settings: settings,
		Runtime:  businesssettings.Runtime(s.cfg),
	})
}

func (s *Server) handleUpdateBusinessSystemSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Settings businesssettings.Settings `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessSettingsStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "settings store failed"})
		return
	}
	defer store.Close()
	settings, err := store.Save(r.Context(), body.Settings)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	s.applyBusinessRuntimeSettings(settings)
	writeJSON(w, http.StatusOK, businesssettings.Response{
		Settings: settings,
		Runtime:  businesssettings.Runtime(s.cfg),
	})
}

func (s *Server) businessSystemSettings(r *http.Request) businesssettings.Settings {
	return s.businessSystemSettingsForContext(r.Context())
}

func (s *Server) businessSystemSettingsForContext(ctx context.Context) businesssettings.Settings {
	store, err := s.newBusinessSettingsStore()
	if err != nil {
		return businesssettings.Defaults()
	}
	defer store.Close()
	settings, found, err := store.GetWithFound(ctx)
	if err != nil {
		return businesssettings.Defaults()
	}
	if found {
		s.applyBusinessRuntimeSettings(settings)
	}
	return settings
}

func (s *Server) applyPersistedBusinessRuntimeSettings(ctx context.Context) {
	store, err := s.newBusinessSettingsStore()
	if err != nil {
		return
	}
	defer store.Close()
	settings, found, err := store.GetWithFound(ctx)
	if err != nil || !found {
		return
	}
	s.applyBusinessRuntimeSettings(settings)
}

func (s *Server) applyBusinessRuntimeSettings(settings businesssettings.Settings) {
	if s == nil || s.cfg == nil {
		return
	}
	normalized := businesssettings.Normalize(settings)
	s.cfg.Server.MaxImageConcurrency = normalized.Runtime.MaxImageConcurrency
	s.cfg.Server.ImageQueueLimit = normalized.Runtime.ImageQueueLimit
	s.cfg.Server.ImageQueueTimeoutSeconds = normalized.Runtime.ImageQueueTimeoutSeconds
}
