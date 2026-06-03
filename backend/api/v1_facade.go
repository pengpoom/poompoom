package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"imagestudio/internal/businessapikeys"
)

func toPublicStatus(internal string) string {
	switch internal {
	case "queued":
		return "queued"
	case "succeeded":
		return "succeeded"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	case "running", "cancel_requested":
		return "running"
	default:
		return "running"
	}
}

func signImageFileToken(secret, fileName string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%d", fileName, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyImageFileToken(secret, fileName string, exp int64, sig string) bool {
	want := signImageFileToken(secret, fileName, exp)
	return hmac.Equal([]byte(want), []byte(sig))
}

func signImageFileQuery(secret, fileName string, exp int64) string {
	sig := signImageFileToken(secret, fileName, exp)
	v := url.Values{}
	v.Set("exp", strconv.FormatInt(exp, 10))
	v.Set("sig", sig)
	return "?" + v.Encode()
}

type ctxKey string

const ctxKeyAPIKey ctxKey = "external_api_key"

func apiKeyFromContext(ctx context.Context) (businessapikeys.APIKey, bool) {
	k, ok := ctx.Value(ctxKeyAPIKey).(businessapikeys.APIKey)
	return k, ok
}

func (s *Server) requireExternalAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.ExternalAPI.Enabled {
			writeV1Error(w, http.StatusServiceUnavailable, "service_unavailable", "external api is disabled")
			return
		}
		token := bearerFromRequest(r)
		if token == "" {
			writeV1Error(w, http.StatusUnauthorized, "invalid_api_key", "missing bearer token")
			return
		}
		store, err := s.newBusinessAPIKeyStore()
		if err != nil {
			writeV1Error(w, http.StatusInternalServerError, "store_unavailable", "key store failed")
			return
		}
		defer store.Close()
		key, err := store.Authenticate(r.Context(), token)
		if err != nil {
			writeV1Error(w, http.StatusUnauthorized, "invalid_api_key", "invalid api key")
			return
		}
		if key.Status != businessapikeys.StatusActive {
			writeV1Error(w, http.StatusForbidden, "key_disabled", "api key is not active")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyAPIKey, key)))
	})
}

func writeV1Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": message, "type": v1ErrorType(status), "code": code},
	})
}

func v1ErrorType(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "authentication_error"
	case status >= 400 && status < 500:
		return "invalid_request_error"
	default:
		return "api_error"
	}
}
