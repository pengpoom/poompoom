package api

import (
	"database/sql"
	"net/http"

	"imagestudio/internal/accounts"
	"imagestudio/internal/cliproxy"
	"imagestudio/internal/config"
)

func SetupRouter(cfg *config.Config, store *accounts.Store, syncClient *cliproxy.Client) http.Handler {
	return SetupRouterWithDatabase(cfg, store, syncClient, nil)
}

func SetupRouterWithDatabase(cfg *config.Config, store *accounts.Store, syncClient *cliproxy.Client, db *sql.DB) http.Handler {
	server := NewServerWithDatabase(cfg, store, syncClient, db)
	server.startBusinessImageJobMaintenanceAsync()
	return server.Handler()
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
