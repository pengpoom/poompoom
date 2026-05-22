package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"mime"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/businessauth"
	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businesssettings"
)

const (
	registrationVerificationTTL      = 10 * time.Minute
	registrationVerificationCooldown = time.Minute
	registrationVerificationAttempts = 5
)

var smtpSendMail = smtp.SendMail

func (s *Server) handleRegistrationOptions(w http.ResponseWriter, r *http.Request) {
	settings := s.businessSystemSettingsForContext(r.Context())
	emailConfigured := registrationEmailConfigFromSettings(settings).configured()
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":                     settings.User.Registration && emailConfigured,
		"registration":                settings.User.Registration,
		"emailVerificationConfigured": emailConfigured,
		"codeTTLSeconds":              int(registrationVerificationTTL / time.Second),
		"codeCooldownSeconds":         int(registrationVerificationCooldown / time.Second),
	})
}

func (s *Server) handleSendRegistrationVerificationCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	settings := s.businessSystemSettingsForContext(r.Context())
	emailConfig := registrationEmailConfigFromSettings(settings)
	if !settings.User.Registration {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "registration is disabled"})
		return
	}
	if !emailConfig.configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "registration email is not configured"})
		return
	}
	store, err := businessauth.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()

	if existing, ok, err := store.GetUserByEmail(r.Context(), body.Email); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	} else if ok {
		if existing.Status == businessauth.StatusDeleted {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "user is deleted and can be restored by an administrator"})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": "email already exists"})
		return
	}

	code, err := newEmailVerificationCode()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create verification code failed"})
		return
	}
	if err := store.CreateEmailVerificationCode(
		r.Context(),
		body.Email,
		businessauth.VerificationPurposeRegistration,
		code,
		time.Now().Add(registrationVerificationTTL),
		registrationVerificationCooldown,
	); errors.Is(err, businessauth.ErrVerificationTooFrequent) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "verification code requested too frequently"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := sendRegistrationVerificationEmail(r.Context(), emailConfig, body.Email, code); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "send verification email failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"expiresIn":       int(registrationVerificationTTL / time.Second),
		"cooldownSeconds": int(registrationVerificationCooldown / time.Second),
	})
}

func (s *Server) handleRegisterBusinessUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	settings := s.businessSystemSettingsForContext(r.Context())
	if !settings.User.Registration {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "registration is disabled"})
		return
	}
	if len(strings.TrimSpace(body.Password)) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "password must be at least 6 characters"})
		return
	}
	store, err := businessauth.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	if err := store.ConsumeEmailVerificationCode(
		r.Context(),
		body.Email,
		businessauth.VerificationPurposeRegistration,
		body.Code,
		registrationVerificationAttempts,
	); errors.Is(err, businessauth.ErrVerificationExpired) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "verification code is expired"})
		return
	} else if errors.Is(err, businessauth.ErrVerificationInvalid) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "verification code is invalid"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	user, err := store.CreateUserWithUsername(r.Context(), body.Email, body.Username, body.Password, businessauth.RoleUser)
	if errors.Is(err, businessauth.ErrUserAlreadyExists) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "email or username already exists"})
		return
	}
	if errors.Is(err, businessauth.ErrUserDeleted) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "user is deleted and can be restored by an administrator"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if settings.User.DefaultCredits > 0 {
		creditStore, err := businesscredits.NewStore(s.cfg)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
			return
		}
		defer creditStore.Close()
		if _, _, err := creditStore.SetBalance(r.Context(), user.ID, settings.User.DefaultCredits, "registration_default"); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}
	token, session, err := s.createAuthSession(r.Context(), loginAccount{
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		UserID:   user.ID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create login session failed"})
		return
	}
	s.setAuthSessionCookie(w, r, token, session.ExpiresAt)
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":        true,
		"token":     token,
		"role":      session.Role,
		"username":  session.Username,
		"email":     session.Email,
		"userId":    session.UserID,
		"expiresAt": session.ExpiresAt.Format(time.RFC3339),
	})
}

func newEmailVerificationCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

type registrationEmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	SiteName string
}

func (c registrationEmailConfig) configured() bool {
	return strings.TrimSpace(c.Host) != "" && strings.TrimSpace(c.From) != ""
}

func registrationEmailConfigFromSettings(settings businesssettings.Settings) registrationEmailConfig {
	normalized := businesssettings.Normalize(settings)
	email := normalized.Email
	port := email.SMTPPort
	if port <= 0 {
		port = 587
	}
	cfg := registrationEmailConfig{
		Host:     email.SMTPHost,
		Port:     port,
		Username: email.Username,
		Password: email.Password,
		From:     email.From,
		FromName: email.FromName,
		SiteName: normalized.Site.Name,
	}
	if !cfg.configured() {
		cfg = registrationEmailConfigFromEnv()
	}
	return cfg
}

func registrationEmailConfigFromEnv() registrationEmailConfig {
	port := 587
	if raw := strings.TrimSpace(os.Getenv("REGISTRATION_SMTP_PORT")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			port = parsed
		}
	}
	return registrationEmailConfig{
		Host:     strings.TrimSpace(os.Getenv("REGISTRATION_SMTP_HOST")),
		Port:     port,
		Username: strings.TrimSpace(os.Getenv("REGISTRATION_SMTP_USERNAME")),
		Password: strings.TrimSpace(os.Getenv("REGISTRATION_SMTP_PASSWORD")),
		From:     strings.TrimSpace(os.Getenv("REGISTRATION_SMTP_FROM")),
		FromName: strings.TrimSpace(os.Getenv("REGISTRATION_SMTP_FROM_NAME")),
		SiteName: strings.TrimSpace(os.Getenv("REGISTRATION_SMTP_FROM_NAME")),
	}
}

func sendRegistrationVerificationEmail(ctx context.Context, cfg registrationEmailConfig, email, code string) error {
	host := strings.TrimSpace(cfg.Host)
	from := strings.TrimSpace(cfg.From)
	if host == "" || from == "" {
		return fmt.Errorf("registration smtp is not configured")
	}
	port := cfg.Port
	if port <= 0 {
		port = 587
	}
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.Password)
	displayName := registrationEmailDisplayName(cfg)
	auth := smtp.Auth(nil)
	if username != "" || password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	subject := fmt.Sprintf("%s 注册验证码", displayName)
	body := fmt.Sprintf("你的 %s 注册验证码是：%s\n\n验证码将在 %d 分钟后失效。如果不是你本人操作，请忽略这封邮件。\n", displayName, code, int(registrationVerificationTTL/time.Minute))
	message := strings.Join([]string{
		"From: " + formatEmailAddressHeader(displayName, from),
		"To: " + email,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	done := make(chan error, 1)
	go func() {
		done <- smtpSendMail(addr, auth, from, []string{email}, []byte(message))
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("smtp send failed to %s via %s:%d: %w", email, host, port, err)
		}
		return nil
	}
}

func registrationEmailDisplayName(cfg registrationEmailConfig) string {
	defaults := businesssettings.Defaults()
	fromName := strings.TrimSpace(cfg.FromName)
	siteName := strings.TrimSpace(cfg.SiteName)
	if fromName != "" && (fromName != defaults.Email.FromName || siteName == "" || siteName == defaults.Site.Name) {
		return fromName
	}
	if siteName != "" {
		return siteName
	}
	if fromName != "" {
		return fromName
	}
	return defaults.Site.Name
}

func formatEmailAddressHeader(name, address string) string {
	encodedName := mime.QEncoding.Encode("UTF-8", strings.TrimSpace(name))
	return encodedName + " <" + strings.TrimSpace(address) + ">"
}
