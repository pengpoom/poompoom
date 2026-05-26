package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"imagestudio/internal/businesssettings"
)

const turnstileSiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var (
	turnstileHTTPClient = &http.Client{Timeout: 8 * time.Second}
	errTurnstileFailed  = errors.New("turnstile verification failed")
)

type turnstileSiteVerifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

type turnstileAction string

const (
	turnstileActionLogin          turnstileAction = "login"
	turnstileActionRegisterCode   turnstileAction = "register_code"
	turnstileActionRegisterSubmit turnstileAction = "register_submit"
	turnstileActionPasswordReset  turnstileAction = "password_reset"
)

func (s *Server) verifyTurnstileForSettings(w http.ResponseWriter, r *http.Request, settings businesssettings.Settings, action turnstileAction, token string) bool {
	if err := verifyTurnstile(r.Context(), settings, action, token, clientIP(r)); err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, errTurnstileFailed) {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]any{"error": "human verification failed"})
		return false
	}
	return true
}

func verifyTurnstile(ctx context.Context, settings businesssettings.Settings, action turnstileAction, token, remoteIP string) error {
	settings = businesssettings.Normalize(settings)
	if !turnstileActionEnabled(settings, action) {
		return nil
	}
	secret := strings.TrimSpace(settings.User.TurnstileSecretKey)
	token = strings.TrimSpace(token)
	if secret == "" || token == "" {
		return errTurnstileFailed
	}

	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIP = strings.TrimSpace(remoteIP); remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileSiteVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := turnstileHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("turnstile siteverify status %d", resp.StatusCode)
	}

	var result turnstileSiteVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if !result.Success {
		return errTurnstileFailed
	}
	return nil
}

func turnstileActionEnabled(settings businesssettings.Settings, action turnstileAction) bool {
	user := businesssettings.Normalize(settings).User
	if !user.TurnstileEnabled {
		return false
	}
	switch action {
	case turnstileActionLogin:
		return user.TurnstileLogin
	case turnstileActionRegisterCode:
		return user.TurnstileRegisterCode
	case turnstileActionRegisterSubmit:
		return user.TurnstileRegisterSubmit
	case turnstileActionPasswordReset:
		return user.TurnstilePasswordReset
	default:
		return false
	}
}
