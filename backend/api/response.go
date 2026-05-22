package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeAPIError(w, status, "", msg)
}

func writeAPIError(w http.ResponseWriter, status int, code, msg string) {
	errorBody := map[string]any{
		"message": msg,
		"type":    "invalid_request_error",
	}
	if strings.TrimSpace(code) != "" {
		errorBody["code"] = strings.TrimSpace(code)
	}
	writeJSON(w, status, map[string]any{
		"error": errorBody,
	})
}
