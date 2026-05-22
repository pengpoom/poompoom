package api

import (
	"net/http"

	"imagestudio/internal/accounts"
	"imagestudio/internal/cliproxy"
	"imagestudio/internal/config"
)

func SetupRouter(cfg *config.Config, store *accounts.Store, syncClient *cliproxy.Client) http.Handler {
	server := NewServer(cfg, store, syncClient)
	server.startBusinessImageJobMaintenanceAsync()
	return server.Handler()
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
