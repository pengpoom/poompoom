package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/accounts"
	"imagestudio/internal/businessauth"
	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
	"imagestudio/internal/businessjobs"
	"imagestudio/internal/businesssettings"
	"imagestudio/internal/config"
	"imagestudio/internal/database"
	"imagestudio/internal/imagehistory"
)

func newDatabaseServerTestConfig(t *testing.T, rootDir ...string) *config.Config {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	root := t.TempDir()
	if len(rootDir) > 0 {
		root = rootDir[0]
	}
	cfg := config.New(root)
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 4
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60
	resetDatabaseServerTestData(t, cfg)
	return cfg
}

func resetDatabaseServerTestData(t *testing.T, cfg *config.Config) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open postgres database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		t.Fatalf("migrate postgres database: %v", err)
	}
	_, err = db.ExecContext(ctx, `TRUNCATE
		business_image_assets,
		business_image_generations,
		business_image_conversations,
		business_image_jobs,
		business_image_tracker,
		business_notification_reads,
		business_notifications,
		business_credit_ledger,
		business_user_credits,
		business_api_providers,
		business_system_settings,
		email_verification_codes,
		user_sessions,
		business_users
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("reset server test tables: %v", err)
	}
}

func TestShouldUseOfficialResponses(t *testing.T) {
	tests := []struct {
		name              string
		preferredAccount  bool
		responsesEligible bool
		configuredRoute   string
		want              bool
	}{
		{
			name:              "paid account with eligible request uses responses",
			responsesEligible: true,
			configuredRoute:   "responses",
			want:              true,
		},
		{
			name:              "paid account with ineligible payload stays legacy",
			responsesEligible: false,
			configuredRoute:   "responses",
			want:              false,
		},
		{
			name:              "preferred source account stays legacy",
			preferredAccount:  true,
			responsesEligible: true,
			configuredRoute:   "responses",
			want:              false,
		},
		{
			name:              "legacy route stays legacy",
			responsesEligible: true,
			configuredRoute:   "legacy",
			want:              false,
		},
		{
			name:              "unknown route falls back to legacy",
			responsesEligible: true,
			configuredRoute:   "something-else",
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseOfficialResponses(tt.preferredAccount, tt.responsesEligible, tt.configuredRoute); got != tt.want {
				t.Fatalf("shouldUseOfficialResponses() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPasswordLoginIssuesRoleSession(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"owner","password":"owner-pass"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	var loginPayload struct {
		Token  string `json:"token"`
		Role   string `json:"role"`
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login payload: %v", err)
	}
	if loginPayload.Token == "" || loginPayload.Role != authRoleAdmin || loginPayload.UserID != businessauth.DefaultAdminUserID {
		t.Fatalf("login payload = %#v", loginPayload)
	}

	authStore, err := businessauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("New business auth store returned error: %v", err)
	}
	defer authStore.Close()
	user, ok, err := authStore.GetUserByUsername(loginReq.Context(), "owner")
	if err != nil {
		t.Fatalf("GetUserByUsername() returned error: %v", err)
	}
	if !ok || user.PasswordHash == "owner-pass" || !strings.HasPrefix(user.PasswordHash, "$2") {
		t.Fatalf("stored user = %#v", user)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/defaults", nil)
	req.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin request status = %d, body = %s", rec.Code, rec.Body.String())
	}

	cookieReq := httptest.NewRequest(http.MethodGet, "/api/config/defaults", nil)
	for _, cookie := range loginRec.Result().Cookies() {
		cookieReq.AddCookie(cookie)
	}
	cookieRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(cookieRec, cookieReq)
	if cookieRec.Code != http.StatusOK {
		t.Fatalf("cookie admin request status = %d, body = %s", cookieRec.Code, cookieRec.Body.String())
	}

	restartedServer := NewServer(cfg, nil, nil)
	restartedReq := httptest.NewRequest(http.MethodGet, "/api/config/defaults", nil)
	restartedReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	restartedRec := httptest.NewRecorder()
	restartedServer.Handler().ServeHTTP(restartedRec, restartedReq)
	if restartedRec.Code != http.StatusOK {
		t.Fatalf("restarted admin request status = %d, body = %s", restartedRec.Code, restartedRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	logoutRec := httptest.NewRecorder()
	restartedServer.Handler().ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", logoutRec.Code, logoutRec.Body.String())
	}

	revokedReq := httptest.NewRequest(http.MethodGet, "/api/config/defaults", nil)
	revokedReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	revokedRec := httptest.NewRecorder()
	NewServer(cfg, nil, nil).Handler().ServeHTTP(revokedRec, revokedReq)
	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked request status = %d, want %d, body = %s", revokedRec.Code, http.StatusUnauthorized, revokedRec.Body.String())
	}
}

func TestPasswordLoginRejectsUserFromAdminRoutes(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"tester","password":"tester-pass"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	var loginPayload struct {
		Token string `json:"token"`
		Role  string `json:"role"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login payload: %v", err)
	}
	if loginPayload.Token == "" || loginPayload.Role != authRoleUser {
		t.Fatalf("login payload = %#v", loginPayload)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/defaults", nil)
	req.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin request status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestPasswordLoginRateLimitsRepeatedFailures(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)

	for attempt := 0; attempt < 5; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"owner","password":"bad-pass"}`))
		req.RemoteAddr = "198.51.100.10:1234"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d, body = %s", attempt+1, rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"owner","password":"owner-pass"}`))
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login status = %d, want %d, body = %s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
}

func TestPasswordLoginSuccessClearsFailureCount(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)

	for attempt := 0; attempt < 4; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"owner","password":"bad-pass"}`))
		req.RemoteAddr = "198.51.100.11:1234"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d, body = %s", attempt+1, rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
	}

	successReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"owner","password":"owner-pass"}`))
	successReq.RemoteAddr = "198.51.100.11:1234"
	successReq.Header.Set("Content-Type", "application/json")
	successRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(successRec, successReq)
	if successRec.Code != http.StatusOK {
		t.Fatalf("successful login status = %d, want %d, body = %s", successRec.Code, http.StatusOK, successRec.Body.String())
	}

	failReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"owner","password":"bad-pass"}`))
	failReq.RemoteAddr = "198.51.100.11:1234"
	failReq.Header.Set("Content-Type", "application/json")
	failRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(failRec, failReq)
	if failRec.Code != http.StatusUnauthorized {
		t.Fatalf("post-success failed login status = %d, want %d, body = %s", failRec.Code, http.StatusUnauthorized, failRec.Body.String())
	}
}

func TestRegistrationDisabledByDefault(t *testing.T) {
	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/register/code", strings.NewReader(`{"email":"new@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("register code status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestEmailVerificationRegistrationCreatesUserAndSession(t *testing.T) {
	cfg := newDatabaseServerTestConfig(t)
	settingsStore, err := businesssettings.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business settings store: %v", err)
	}
	settings := businesssettings.Defaults()
	settings.User.Registration = true
	settings.User.DefaultCredits = 7
	settings.Site.Name = "ImageStudio"
	settings.Email.SMTPHost = "smtp.example.test"
	settings.Email.SMTPPort = 587
	settings.Email.Username = "mailer@example.test"
	settings.Email.Password = "mailer-pass"
	settings.Email.From = "noreply@example.test"
	settings.Email.FromName = "ImageStudio"
	if _, err := settingsStore.Save(context.Background(), settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if err := settingsStore.Close(); err != nil {
		t.Fatalf("close settings store: %v", err)
	}

	var sentEmail string
	var sentCode string
	var sentMessage string
	previousSMTPSendMail := smtpSendMail
	smtpSendMail = func(_ string, _ smtp.Auth, _ string, to []string, msg []byte) error {
		if len(to) > 0 {
			sentEmail = to[0]
		}
		body := string(msg)
		sentMessage = body
		index := strings.Index(body, "注册验证码是：")
		if index >= 0 && len(body) >= index+len("注册验证码是：")+6 {
			sentCode = body[index+len("注册验证码是：") : index+len("注册验证码是：")+6]
		}
		return nil
	}
	defer func() {
		smtpSendMail = previousSMTPSendMail
	}()
	server := NewServer(cfg, nil, nil)

	codeReq := httptest.NewRequest(http.MethodPost, "/auth/register/code", strings.NewReader(`{"email":"fresh@example.com"}`))
	codeReq.Header.Set("Content-Type", "application/json")
	codeRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(codeRec, codeReq)
	if codeRec.Code != http.StatusOK {
		t.Fatalf("register code status = %d, body = %s", codeRec.Code, codeRec.Body.String())
	}
	if sentEmail != "fresh@example.com" || len(sentCode) != 6 {
		t.Fatalf("sent email/code = %q/%q", sentEmail, sentCode)
	}
	if !strings.Contains(sentMessage, "Subject: =?UTF-8?q?ImageStudio_=E6=B3=A8=E5=86=8C=E9=AA=8C=E8=AF=81=E7=A0=81?=") ||
		!strings.Contains(sentMessage, "你的 ImageStudio 注册验证码是：") {
		t.Fatalf("sent message does not use configured site name: %s", sentMessage)
	}

	registerBody := `{"email":"fresh@example.com","username":"Fresh","password":"fresh-pass","code":"` + sentCode + `"}`
	registerReq := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", registerRec.Code, registerRec.Body.String())
	}
	var payload struct {
		Token    string `json:"token"`
		Role     string `json:"role"`
		Username string `json:"username"`
		UserID   string `json:"userId"`
	}
	if err := json.Unmarshal(registerRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode register payload: %v", err)
	}
	if payload.Token == "" || payload.Role != businessauth.RoleUser || payload.Username != "fresh" || payload.UserID == "" {
		t.Fatalf("register payload = %#v", payload)
	}

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("open credit store: %v", err)
	}
	defer creditStore.Close()
	summary, err := creditStore.Summary(context.Background(), payload.UserID)
	if err != nil {
		t.Fatalf("credit summary: %v", err)
	}
	if summary.Balance != 7 {
		t.Fatalf("registered user balance = %d, want 7", summary.Balance)
	}
}

func TestPasswordResetByEmailVerification(t *testing.T) {
	cfg := newDatabaseServerTestConfig(t)
	settingsStore, err := businesssettings.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business settings store: %v", err)
	}
	settings := businesssettings.Defaults()
	settings.Site.Name = "ImageStudio"
	settings.Email.SMTPHost = "smtp.example.test"
	settings.Email.SMTPPort = 587
	settings.Email.Username = "mailer@example.test"
	settings.Email.Password = "mailer-pass"
	settings.Email.From = "noreply@example.test"
	settings.Email.FromName = "ImageStudio"
	if _, err := settingsStore.Save(context.Background(), settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if err := settingsStore.Close(); err != nil {
		t.Fatalf("close settings store: %v", err)
	}

	authStore, err := businessauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	if _, err := authStore.CreateUserWithUsername(context.Background(), "reset@example.com", "reset-user", "old-pass", businessauth.RoleUser); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := authStore.Close(); err != nil {
		t.Fatalf("close auth store: %v", err)
	}

	var sentEmail string
	var sentCode string
	var sentMessage string
	previousSMTPSendMail := smtpSendMail
	smtpSendMail = func(_ string, _ smtp.Auth, _ string, to []string, msg []byte) error {
		if len(to) > 0 {
			sentEmail = to[0]
		}
		body := string(msg)
		sentMessage = body
		index := strings.Index(body, "密码重置验证码是：")
		if index >= 0 && len(body) >= index+len("密码重置验证码是：")+6 {
			sentCode = body[index+len("密码重置验证码是：") : index+len("密码重置验证码是：")+6]
		}
		return nil
	}
	defer func() {
		smtpSendMail = previousSMTPSendMail
	}()
	server := NewServer(cfg, nil, nil)

	codeReq := httptest.NewRequest(http.MethodPost, "/auth/password-reset/code", strings.NewReader(`{"email":"reset@example.com"}`))
	codeReq.Header.Set("Content-Type", "application/json")
	codeRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(codeRec, codeReq)
	if codeRec.Code != http.StatusOK {
		t.Fatalf("password reset code status = %d, body = %s", codeRec.Code, codeRec.Body.String())
	}
	if sentEmail != "reset@example.com" || len(sentCode) != 6 {
		t.Fatalf("sent reset email/code = %q/%q", sentEmail, sentCode)
	}
	if !strings.Contains(sentMessage, "你的 ImageStudio 密码重置验证码是：") {
		t.Fatalf("sent reset message does not use reset template: %s", sentMessage)
	}

	resetBody := `{"email":"reset@example.com","password":"new-pass","code":"` + sentCode + `"}`
	resetReq := httptest.NewRequest(http.MethodPost, "/auth/password-reset", strings.NewReader(resetBody))
	resetReq.Header.Set("Content-Type", "application/json")
	resetRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("password reset status = %d, body = %s", resetRec.Code, resetRec.Body.String())
	}

	oldLoginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"reset@example.com","password":"old-pass"}`))
	oldLoginReq.Header.Set("Content-Type", "application/json")
	oldLoginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(oldLoginRec, oldLoginReq)
	if oldLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d, want %d, body = %s", oldLoginRec.Code, http.StatusUnauthorized, oldLoginRec.Body.String())
	}

	newLoginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"reset@example.com","password":"new-pass"}`))
	newLoginReq.Header.Set("Content-Type", "application/json")
	newLoginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(newLoginRec, newLoginReq)
	if newLoginRec.Code != http.StatusOK {
		t.Fatalf("new password login status = %d, body = %s", newLoginRec.Code, newLoginRec.Body.String())
	}
}

func TestAdminCanManageBusinessUsers(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	adminToken := loginForTest(t, server, "owner", "owner-pass")

	createReq := httptest.NewRequest(http.MethodPost, "/api/business/users", strings.NewReader(`{"email":"guest@example.com","password":"guest-pass"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var createPayload struct {
		Item businessauth.User `json:"item"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create user payload: %v", err)
	}
	if createPayload.Item.ID == "" || createPayload.Item.Username != "guest" || createPayload.Item.Email != "guest@example.com" || createPayload.Item.Role != businessauth.RoleUser {
		t.Fatalf("created user = %#v", createPayload.Item)
	}
	if createPayload.Item.PasswordHash != "" {
		t.Fatalf("created user leaked password hash: %#v", createPayload.Item)
	}

	imageStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open business image store: %v", err)
	}
	defer imageStore.Close()
	_, err = imageStore.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "guest-conv",
		UserID: createPayload.Item.ID,
		Title:  "Guest session",
	})
	if err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	_, err = imageStore.SaveGeneration(context.Background(), businessimage.Generation{
		ID:             "guest-generation",
		UserID:         createPayload.Item.ID,
		ConversationID: "guest-conv",
		TurnID:         "guest-turn",
		Prompt:         "umbrella",
		Model:          "gpt-image-test",
		Size:           "1024x1024",
		Quality:        "high",
		Count:          1,
		Status:         "succeeded",
		Response:       json.RawMessage(`{"data":[{"url":"/v1/files/image/business-guest.png"}]}`),
		CreatedAt:      "2026-05-14T00:00:00Z",
		FinishedAt:     "2026-05-14T00:00:02Z",
	})
	if err != nil {
		t.Fatalf("save generation: %v", err)
	}
	_, err = imageStore.SaveGeneration(context.Background(), businessimage.Generation{
		ID:             "guest-generation-failed",
		UserID:         createPayload.Item.ID,
		ConversationID: "guest-conv",
		TurnID:         "guest-turn-failed",
		Prompt:         "failed umbrella",
		Model:          "gpt-image-test",
		Size:           "1024x1024",
		Quality:        "high",
		Count:          1,
		Status:         "failed",
		Error:          "upstream failed",
		CreatedAt:      "2026-05-15T00:00:00Z",
		FinishedAt:     "2026-05-15T00:00:01Z",
	})
	if err != nil {
		t.Fatalf("save failed generation: %v", err)
	}
	_, err = imageStore.SaveAsset(context.Background(), businessimage.Asset{
		ID:             "business-guest.png",
		UserID:         createPayload.Item.ID,
		ConversationID: "guest-conv",
		GenerationID:   "guest-generation",
		FileName:       "business-guest.png",
		FilePath:       "/tmp/business-guest.png",
		URL:            "/v1/files/image/business-guest.png",
		MimeType:       "image/png",
		SizeBytes:      12345,
		SHA256:         "hash",
	})
	if err != nil {
		t.Fatalf("save asset: %v", err)
	}
	_, err = imageStore.SaveAsset(context.Background(), businessimage.Asset{
		ID:             "business-guest-2.png",
		UserID:         createPayload.Item.ID,
		ConversationID: "guest-conv",
		GenerationID:   "guest-generation",
		FileName:       "business-guest-2.png",
		FilePath:       "/tmp/business-guest-2.png",
		URL:            "/v1/files/image/business-guest-2.png",
		MimeType:       "image/png",
		SizeBytes:      100,
		SHA256:         "hash-2",
	})
	if err != nil {
		t.Fatalf("save second asset: %v", err)
	}

	creditReq := httptest.NewRequest(http.MethodPut, "/api/business/users/"+createPayload.Item.ID+"/credit", strings.NewReader(`{"balance":12}`))
	creditReq.SetPathValue("id", createPayload.Item.ID)
	creditReq.Header.Set("Content-Type", "application/json")
	creditReq.Header.Set("Authorization", "Bearer "+adminToken)
	creditRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(creditRec, creditReq)
	if creditRec.Code != http.StatusOK {
		t.Fatalf("set credit status = %d, body = %s", creditRec.Code, creditRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/business/users", nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list users status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var listPayload struct {
		Items []struct {
			businessauth.User
			Usage  businessimage.UserUsage `json:"usage"`
			Credit businesscredits.Summary `json:"credit"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode list users payload: %v", err)
	}
	var guestUsage businessimage.UserUsage
	var guestCredit businesscredits.Summary
	for _, item := range listPayload.Items {
		if item.ID == createPayload.Item.ID {
			guestUsage = item.Usage
			guestCredit = item.Credit
			break
		}
	}
	if guestUsage.UserID != createPayload.Item.ID ||
		guestUsage.GenerationCount != 2 ||
		guestUsage.SuccessCount != 1 ||
		guestUsage.FailedCount != 1 ||
		guestUsage.ImageCount != 2 ||
		guestUsage.StorageBytes != 12445 ||
		guestUsage.LastGeneratedAt == "" {
		t.Fatalf("guest usage = %#v", guestUsage)
	}
	if guestCredit.UserID != createPayload.Item.ID ||
		guestCredit.Balance != 12 ||
		guestCredit.Spent != 0 {
		t.Fatalf("guest credit = %#v", guestCredit)
	}

	guestToken := loginForTest(t, server, "guest", "guest-pass")

	selfCreditReq := httptest.NewRequest(http.MethodGet, "/api/business/credit", nil)
	selfCreditReq.Header.Set("Authorization", "Bearer "+guestToken)
	selfCreditRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(selfCreditRec, selfCreditReq)
	if selfCreditRec.Code != http.StatusOK {
		t.Fatalf("self credit status = %d, body = %s", selfCreditRec.Code, selfCreditRec.Body.String())
	}
	var selfCredit businesscredits.Summary
	if err := json.Unmarshal(selfCreditRec.Body.Bytes(), &selfCredit); err != nil {
		t.Fatalf("decode self credit payload: %v", err)
	}
	if selfCredit.UserID != createPayload.Item.ID || selfCredit.Balance != 12 {
		t.Fatalf("self credit = %#v", selfCredit)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/business/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+guestToken)
	meRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meRec.Code, meRec.Body.String())
	}
	var mePayload struct {
		User   businessauth.User       `json:"user"`
		Credit businesscredits.Summary `json:"credit"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &mePayload); err != nil {
		t.Fatalf("decode me payload: %v", err)
	}
	if mePayload.User.ID != createPayload.Item.ID || mePayload.User.Username != "guest" || mePayload.Credit.Balance != 12 {
		t.Fatalf("me payload = %#v", mePayload)
	}

	wrongPasswordReq := httptest.NewRequest(http.MethodPatch, "/api/business/me/password", strings.NewReader(`{"currentPassword":"bad-pass","newPassword":"guest-new-pass"}`))
	wrongPasswordReq.Header.Set("Content-Type", "application/json")
	wrongPasswordReq.Header.Set("Authorization", "Bearer "+guestToken)
	wrongPasswordRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(wrongPasswordRec, wrongPasswordReq)
	if wrongPasswordRec.Code != http.StatusBadRequest {
		t.Fatalf("wrong password status = %d, body = %s", wrongPasswordRec.Code, wrongPasswordRec.Body.String())
	}

	changePasswordReq := httptest.NewRequest(http.MethodPatch, "/api/business/me/password", strings.NewReader(`{"currentPassword":"guest-pass","newPassword":"guest-new-pass"}`))
	changePasswordReq.Header.Set("Content-Type", "application/json")
	changePasswordReq.Header.Set("Authorization", "Bearer "+guestToken)
	changePasswordRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(changePasswordRec, changePasswordReq)
	if changePasswordRec.Code != http.StatusOK {
		t.Fatalf("change password status = %d, body = %s", changePasswordRec.Code, changePasswordRec.Body.String())
	}

	editReq := httptest.NewRequest(http.MethodPatch, "/api/business/users/"+createPayload.Item.ID, strings.NewReader(`{"username":"guest-renamed","password":"guest-edited-pass"}`))
	editReq.Header.Set("Content-Type", "application/json")
	editReq.Header.Set("Authorization", "Bearer "+adminToken)
	editRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(editRec, editReq)
	if editRec.Code != http.StatusOK {
		t.Fatalf("edit user status = %d, body = %s", editRec.Code, editRec.Body.String())
	}
	var editPayload struct {
		Item businessauth.User `json:"item"`
	}
	if err := json.Unmarshal(editRec.Body.Bytes(), &editPayload); err != nil {
		t.Fatalf("decode edit user payload: %v", err)
	}
	if editPayload.Item.Username != "guest-renamed" || editPayload.Item.Email != "guest@example.com" {
		t.Fatalf("edited user = %#v", editPayload.Item)
	}
	_ = loginForTest(t, server, "guest@example.com", "guest-edited-pass")

	stillValidReq := httptest.NewRequest(http.MethodGet, "/api/business/me", nil)
	stillValidReq.Header.Set("Authorization", "Bearer "+guestToken)
	stillValidRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(stillValidRec, stillValidReq)
	if stillValidRec.Code != http.StatusUnauthorized {
		t.Fatalf("current session status = %d, want %d, body = %s", stillValidRec.Code, http.StatusUnauthorized, stillValidRec.Body.String())
	}

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("open credit store: %v", err)
	}
	defer creditStore.Close()
	if _, _, err := creditStore.Reserve(context.Background(), createPayload.Item.ID, 1, "guest-generation"); err != nil {
		t.Fatalf("reserve credit: %v", err)
	}
	guestToken = loginForTest(t, server, "guest@example.com", "guest-edited-pass")

	selfUsageReq := httptest.NewRequest(http.MethodGet, "/api/business/usage", nil)
	selfUsageReq.Header.Set("Authorization", "Bearer "+guestToken)
	selfUsageRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(selfUsageRec, selfUsageReq)
	if selfUsageRec.Code != http.StatusOK {
		t.Fatalf("self usage status = %d, body = %s", selfUsageRec.Code, selfUsageRec.Body.String())
	}
	var selfUsagePayload struct {
		Items []businessimage.UsageRecord `json:"items"`
	}
	if err := json.Unmarshal(selfUsageRec.Body.Bytes(), &selfUsagePayload); err != nil {
		t.Fatalf("decode self usage payload: %v", err)
	}
	if len(selfUsagePayload.Items) != 2 ||
		selfUsagePayload.Items[0].UserID != createPayload.Item.ID ||
		selfUsagePayload.Items[0].Model != "gpt-image-test" {
		t.Fatalf("self usage payload = %#v", selfUsagePayload)
	}

	selfUsageFilteredReq := httptest.NewRequest(http.MethodGet, "/api/business/usage?page=1&pageSize=1&status=succeeded&model=gpt-image-test&from=2026-05-13T00:00:00Z&to=2026-05-14T23:59:59Z", nil)
	selfUsageFilteredReq.Header.Set("Authorization", "Bearer "+guestToken)
	selfUsageFilteredRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(selfUsageFilteredRec, selfUsageFilteredReq)
	if selfUsageFilteredRec.Code != http.StatusOK {
		t.Fatalf("filtered self usage status = %d, body = %s", selfUsageFilteredRec.Code, selfUsageFilteredRec.Body.String())
	}
	var selfUsageFilteredPayload struct {
		Items []businessimage.UsageRecord `json:"items"`
		Page  paginationMeta              `json:"page"`
	}
	if err := json.Unmarshal(selfUsageFilteredRec.Body.Bytes(), &selfUsageFilteredPayload); err != nil {
		t.Fatalf("decode filtered self usage payload: %v", err)
	}
	if selfUsageFilteredPayload.Page.Total != 1 ||
		selfUsageFilteredPayload.Page.PageSize != 1 ||
		len(selfUsageFilteredPayload.Items) != 1 ||
		selfUsageFilteredPayload.Items[0].Status != "succeeded" {
		t.Fatalf("filtered self usage payload = %#v", selfUsageFilteredPayload)
	}

	selfAssetsReq := httptest.NewRequest(http.MethodGet, "/api/business/assets?page=1&pageSize=1", nil)
	selfAssetsReq.Header.Set("Authorization", "Bearer "+guestToken)
	selfAssetsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(selfAssetsRec, selfAssetsReq)
	if selfAssetsRec.Code != http.StatusOK {
		t.Fatalf("self assets status = %d, body = %s", selfAssetsRec.Code, selfAssetsRec.Body.String())
	}
	var selfAssetsPayload businessAssetsResponse
	if err := json.Unmarshal(selfAssetsRec.Body.Bytes(), &selfAssetsPayload); err != nil {
		t.Fatalf("decode self assets payload: %v", err)
	}
	if len(selfAssetsPayload.Items) != 1 ||
		selfAssetsPayload.Page.Total != 2 ||
		selfAssetsPayload.Page.PageSize != 1 ||
		selfAssetsPayload.Items[0].UserID != createPayload.Item.ID ||
		selfAssetsPayload.Items[0].ConversationTitle != "Guest session" ||
		selfAssetsPayload.Items[0].Prompt != "umbrella" ||
		selfAssetsPayload.Items[0].Model != "gpt-image-test" {
		t.Fatalf("self assets payload = %#v", selfAssetsPayload)
	}

	adminAssetsReq := httptest.NewRequest(http.MethodGet, "/api/business/assets", nil)
	adminAssetsReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminAssetsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(adminAssetsRec, adminAssetsReq)
	if adminAssetsRec.Code != http.StatusOK {
		t.Fatalf("admin self assets status = %d, body = %s", adminAssetsRec.Code, adminAssetsRec.Body.String())
	}
	var adminAssetsPayload businessAssetsResponse
	if err := json.Unmarshal(adminAssetsRec.Body.Bytes(), &adminAssetsPayload); err != nil {
		t.Fatalf("decode admin self assets payload: %v", err)
	}
	if len(adminAssetsPayload.Items) != 0 || adminAssetsPayload.Page.Total != 0 {
		t.Fatalf("admin self assets payload = %#v", adminAssetsPayload)
	}

	adminUsageReq := httptest.NewRequest(http.MethodGet, "/api/business/admin/usage", nil)
	adminUsageReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminUsageRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(adminUsageRec, adminUsageReq)
	if adminUsageRec.Code != http.StatusOK {
		t.Fatalf("admin usage status = %d, body = %s", adminUsageRec.Code, adminUsageRec.Body.String())
	}
	var adminUsagePayload struct {
		Items []struct {
			businessimage.UsageRecord
			Username string `json:"username"`
		} `json:"items"`
	}
	if err := json.Unmarshal(adminUsageRec.Body.Bytes(), &adminUsagePayload); err != nil {
		t.Fatalf("decode admin usage payload: %v", err)
	}
	if len(adminUsagePayload.Items) == 0 ||
		adminUsagePayload.Items[0].UserID != createPayload.Item.ID ||
		adminUsagePayload.Items[0].Username != "guest-renamed" ||
		adminUsagePayload.Items[0].Username == "" {
		t.Fatalf("admin usage payload = %#v", adminUsagePayload)
	}

	adminUsageFilteredReq := httptest.NewRequest(http.MethodGet, "/api/business/admin/usage?page=1&pageSize=1&status=failed&model=gpt-image-test&from=2026-05-15T00:00:00Z&to=2026-05-15T23:59:59Z&userId="+createPayload.Item.ID, nil)
	adminUsageFilteredReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminUsageFilteredRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(adminUsageFilteredRec, adminUsageFilteredReq)
	if adminUsageFilteredRec.Code != http.StatusOK {
		t.Fatalf("filtered admin usage status = %d, body = %s", adminUsageFilteredRec.Code, adminUsageFilteredRec.Body.String())
	}
	var adminUsageFilteredPayload struct {
		Items []struct {
			businessimage.UsageRecord
			Username string `json:"username"`
		} `json:"items"`
		Page paginationMeta `json:"page"`
	}
	if err := json.Unmarshal(adminUsageFilteredRec.Body.Bytes(), &adminUsageFilteredPayload); err != nil {
		t.Fatalf("decode filtered admin usage payload: %v", err)
	}
	if adminUsageFilteredPayload.Page.Total != 1 ||
		len(adminUsageFilteredPayload.Items) != 1 ||
		adminUsageFilteredPayload.Items[0].Status != "failed" ||
		adminUsageFilteredPayload.Items[0].Username != "guest-renamed" {
		t.Fatalf("filtered admin usage payload = %#v", adminUsageFilteredPayload)
	}

	dashboardReq := httptest.NewRequest(http.MethodGet, "/api/business/admin/dashboard", nil)
	dashboardReq.Header.Set("Authorization", "Bearer "+adminToken)
	dashboardRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(dashboardRec, dashboardReq)
	if dashboardRec.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, body = %s", dashboardRec.Code, dashboardRec.Body.String())
	}
	var dashboardPayload businessDashboardResponse
	if err := json.Unmarshal(dashboardRec.Body.Bytes(), &dashboardPayload); err != nil {
		t.Fatalf("decode dashboard payload: %v", err)
	}
	if dashboardPayload.Summary.UserCount < 2 ||
		dashboardPayload.Summary.GenerationCount != 2 ||
		dashboardPayload.Summary.ImageCount != 2 ||
		dashboardPayload.Summary.StorageBytes != 12445 ||
		dashboardPayload.Summary.CreditSpent != 1 ||
		dashboardPayload.Summary.ConversationCount != 1 ||
		len(dashboardPayload.Recent) == 0 ||
		dashboardPayload.Recent[0].Username != "guest-renamed" ||
		len(dashboardPayload.TopUsers) == 0 ||
		dashboardPayload.TopUsers[0].Username != "guest-renamed" ||
		len(dashboardPayload.ModelUsage) == 0 ||
		dashboardPayload.ModelUsage[0].Model != "gpt-image-test" {
		t.Fatalf("dashboard payload = %#v", dashboardPayload)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/business/users/"+createPayload.Item.ID+"?usagePage=1&usagePageSize=1&assetsPage=1&assetsPageSize=1&ledgerPage=1&ledgerPageSize=1&status=succeeded&model=gpt-image-test&from=2026-05-14T00:00:00Z&to=2026-05-14T23:59:59Z", nil)
	detailReq.SetPathValue("id", createPayload.Item.ID)
	detailReq.Header.Set("Authorization", "Bearer "+adminToken)
	detailRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("user detail status = %d, body = %s", detailRec.Code, detailRec.Body.String())
	}
	var detailPayload struct {
		User               businessauth.User             `json:"user"`
		Usage              businessimage.UserUsage       `json:"usage"`
		Credit             businesscredits.Summary       `json:"credit"`
		RecentUsage        []businessUsageRecordWithUser `json:"recentUsage"`
		RecentUsagePage    paginationMeta                `json:"recentUsagePage"`
		RecentAssets       []businessimage.Asset         `json:"recentAssets"`
		RecentAssetsPage   paginationMeta                `json:"recentAssetsPage"`
		RecentCreditLedger []businesscredits.LedgerEntry `json:"recentCreditLedger"`
		RecentLedgerPage   paginationMeta                `json:"recentLedgerPage"`
		ConversationCount  int64                         `json:"conversationCount"`
	}
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detailPayload); err != nil {
		t.Fatalf("decode user detail payload: %v", err)
	}
	if detailPayload.User.ID != createPayload.Item.ID ||
		detailPayload.User.Username != "guest-renamed" ||
		detailPayload.User.PasswordHash != "" {
		t.Fatalf("detail user = %#v", detailPayload.User)
	}
	if detailPayload.Usage.UserID != createPayload.Item.ID ||
		detailPayload.Usage.GenerationCount != 2 ||
		detailPayload.Usage.ImageCount != 2 ||
		detailPayload.Usage.StorageBytes != 12445 ||
		detailPayload.ConversationCount != 1 {
		t.Fatalf("detail usage = %#v, conversationCount=%d", detailPayload.Usage, detailPayload.ConversationCount)
	}
	if detailPayload.Credit.UserID != createPayload.Item.ID ||
		detailPayload.Credit.Balance != 11 ||
		detailPayload.Credit.Spent != 1 {
		t.Fatalf("detail credit = %#v", detailPayload.Credit)
	}
	if len(detailPayload.RecentUsage) != 1 ||
		detailPayload.RecentUsage[0].UserID != createPayload.Item.ID ||
		detailPayload.RecentUsage[0].Username != "guest-renamed" ||
		detailPayload.RecentUsage[0].Status != "succeeded" ||
		detailPayload.RecentUsagePage.Total != 1 {
		t.Fatalf("detail recent usage = %#v", detailPayload.RecentUsage)
	}
	if len(detailPayload.RecentAssets) != 1 ||
		detailPayload.RecentAssets[0].UserID != createPayload.Item.ID ||
		detailPayload.RecentAssetsPage.Total != 2 {
		t.Fatalf("detail recent assets = %#v", detailPayload.RecentAssets)
	}
	if len(detailPayload.RecentCreditLedger) != 1 ||
		detailPayload.RecentLedgerPage.Total != 3 ||
		detailPayload.RecentCreditLedger[0].Reason != businesscredits.ReasonImageGenerationReserve {
		t.Fatalf("detail ledger = %#v", detailPayload.RecentCreditLedger)
	}

	clearDataReq := httptest.NewRequest(http.MethodDelete, "/api/business/users/"+createPayload.Item.ID+"/data", nil)
	clearDataReq.SetPathValue("id", createPayload.Item.ID)
	clearDataReq.Header.Set("Authorization", "Bearer "+adminToken)
	clearDataRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(clearDataRec, clearDataReq)
	if clearDataRec.Code != http.StatusOK {
		t.Fatalf("clear user data status = %d, body = %s", clearDataRec.Code, clearDataRec.Body.String())
	}
	guestToken = loginForTest(t, server, "guest@example.com", "guest-edited-pass")
	clearedDetailReq := httptest.NewRequest(http.MethodGet, "/api/business/users/"+createPayload.Item.ID, nil)
	clearedDetailReq.SetPathValue("id", createPayload.Item.ID)
	clearedDetailReq.Header.Set("Authorization", "Bearer "+adminToken)
	clearedDetailRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(clearedDetailRec, clearedDetailReq)
	if clearedDetailRec.Code != http.StatusOK {
		t.Fatalf("cleared user detail status = %d, body = %s", clearedDetailRec.Code, clearedDetailRec.Body.String())
	}
	var clearedDetailPayload struct {
		User               businessauth.User             `json:"user"`
		Usage              businessimage.UserUsage       `json:"usage"`
		Credit             businesscredits.Summary       `json:"credit"`
		RecentUsage        []businessUsageRecordWithUser `json:"recentUsage"`
		RecentAssets       []businessimage.Asset         `json:"recentAssets"`
		RecentCreditLedger []businesscredits.LedgerEntry `json:"recentCreditLedger"`
	}
	if err := json.Unmarshal(clearedDetailRec.Body.Bytes(), &clearedDetailPayload); err != nil {
		t.Fatalf("decode cleared detail payload: %v", err)
	}
	if clearedDetailPayload.User.ID != createPayload.Item.ID ||
		clearedDetailPayload.User.Status != businessauth.StatusActive ||
		clearedDetailPayload.Usage.GenerationCount != 0 ||
		clearedDetailPayload.Credit.Balance != 0 ||
		len(clearedDetailPayload.RecentUsage) != 0 ||
		len(clearedDetailPayload.RecentAssets) != 0 ||
		len(clearedDetailPayload.RecentCreditLedger) != 0 {
		t.Fatalf("cleared detail payload = %#v", clearedDetailPayload)
	}

	statusReq := httptest.NewRequest(http.MethodPatch, "/api/business/users/"+createPayload.Item.ID+"/status", strings.NewReader(`{"status":"disabled"}`))
	statusReq.SetPathValue("id", createPayload.Item.ID)
	statusReq.Header.Set("Content-Type", "application/json")
	statusReq.Header.Set("Authorization", "Bearer "+adminToken)
	statusRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("disable user status = %d, body = %s", statusRec.Code, statusRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/business/image/conversations", nil)
	req.Header.Set("Authorization", "Bearer "+guestToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("disabled user request status = %d, want %d, body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestBusinessSystemSettingsAffectDefaultUserCredits(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	adminToken := loginForTest(t, server, "owner", "owner-pass")

	settingsReq := httptest.NewRequest(
		http.MethodPut,
		"/api/business/system-settings",
		strings.NewReader(`{
			"settings": {
				"site": {"name":"ImageStudio","subtitle":"图片生成工作台","logoUrl":"","contactInfo":""},
				"user": {"defaultRole":"user","defaultCredits":35,"registration":false},
				"generation": {"defaultPlatform":"gpt-image","defaultQuality":"high","defaultSize":"1024x1024","defaultCount":1,"maxCount":8},
				"billing": {"gptImageCost":2,"geminiBananaCost":3,"refundOnFailure":true,"refundPartialCount":true},
				"runtime": {"maxImageConcurrency":5,"imageQueueLimit":17,"imageQueueTimeoutSeconds":9},
				"security": {"imageFileAuthRequired":true}
			}
		}`),
	)
	settingsReq.Header.Set("Content-Type", "application/json")
	settingsReq.Header.Set("Authorization", "Bearer "+adminToken)
	settingsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(settingsRec, settingsReq)
	if settingsRec.Code != http.StatusOK {
		t.Fatalf("update settings status = %d, body = %s", settingsRec.Code, settingsRec.Body.String())
	}
	maxConcurrency, queueLimit, queueTimeout := cfg.ImageQueueConfig()
	if maxConcurrency != 5 || queueLimit != 17 || queueTimeout != 9*time.Second {
		t.Fatalf("runtime queue config = %d/%d/%s, want 5/17/9s", maxConcurrency, queueLimit, queueTimeout)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/business/users", strings.NewReader(`{"username":"guest","password":"guest-pass","role":"user"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var createPayload struct {
		Item businessauth.User `json:"item"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create user payload: %v", err)
	}

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("open credit store: %v", err)
	}
	defer creditStore.Close()
	credit, err := creditStore.Summary(context.Background(), createPayload.Item.ID)
	if err != nil {
		t.Fatalf("get credit: %v", err)
	}
	if credit.Balance != 35 {
		t.Fatalf("default credit balance = %d, want 35", credit.Balance)
	}
}

func TestRuntimeStatusUsesPersistedBusinessSettingsOnFirstLoad(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	rootDir := t.TempDir()
	cfg := newDatabaseServerTestConfig(t, rootDir)
	server := NewServer(cfg, nil, nil)
	adminToken := loginForTest(t, server, "owner", "owner-pass")

	settingsReq := httptest.NewRequest(
		http.MethodPut,
		"/api/business/system-settings",
		strings.NewReader(`{
			"settings": {
				"site": {"name":"ImageStudio","subtitle":"图片生成工作台","logoUrl":"","contactInfo":""},
				"user": {"defaultRole":"user","defaultCredits":20,"registration":false},
				"generation": {"defaultPlatform":"gpt-image","defaultQuality":"high","defaultSize":"1024x1024","defaultCount":1,"maxCount":8},
				"billing": {"gptImageCost":1,"geminiBananaCost":1,"refundOnFailure":true,"refundPartialCount":true},
				"runtime": {"maxImageConcurrency":16,"imageQueueLimit":64,"imageQueueTimeoutSeconds":180},
				"security": {"imageFileAuthRequired":true}
			}
		}`),
	)
	settingsReq.Header.Set("Content-Type", "application/json")
	settingsReq.Header.Set("Authorization", "Bearer "+adminToken)
	settingsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(settingsRec, settingsReq)
	if settingsRec.Code != http.StatusOK {
		t.Fatalf("update settings status = %d, body = %s", settingsRec.Code, settingsRec.Body.String())
	}

	reloadedCfg := newDatabaseServerTestConfig(t, rootDir)
	reloadedServer := NewServer(reloadedCfg, nil, nil)
	reloadedToken := loginForTest(t, reloadedServer, "owner", "owner-pass")

	statusReq := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	statusReq.Header.Set("Authorization", "Bearer "+reloadedToken)
	statusRec := httptest.NewRecorder()
	reloadedServer.Handler().ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("runtime status = %d, body = %s", statusRec.Code, statusRec.Body.String())
	}
	var payload runtimeStatusResponse
	if err := json.Unmarshal(statusRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode runtime status: %v", err)
	}
	if payload.Admission.MaxConcurrency != 16 || payload.Admission.QueueLimit != 64 || payload.Admission.QueueTimeoutMS != 180000 {
		t.Fatalf("runtime admission = %#v, want 16/64/180000", payload.Admission)
	}
	if payload.System.Runtime.Goroutines <= 0 {
		t.Fatalf("runtime goroutines = %d, want positive", payload.System.Runtime.Goroutines)
	}
	if !payload.System.Database.OK {
		t.Fatalf("runtime database status = %#v, want ok", payload.System.Database)
	}
	if payload.System.Redis.Enabled {
		t.Fatalf("runtime redis enabled = true, want false for default config")
	}
}

func TestMaintenanceModeBlocksNewImageSubmissionsAndResetsOnRestart(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	rootDir := t.TempDir()
	cfg := newDatabaseServerTestConfig(t, rootDir)
	server := NewServer(cfg, nil, nil)
	adminToken := loginForTest(t, server, "owner", "owner-pass")
	userToken := loginForTest(t, server, "tester", "tester-pass")

	updateReq := httptest.NewRequest(http.MethodPut, "/api/system/maintenance", strings.NewReader(`{"enabled":true}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+adminToken)
	updateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("maintenance update status = %d, body = %s", updateRec.Code, updateRec.Body.String())
	}
	var updatePayload maintenanceStatusResponse
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updatePayload); err != nil {
		t.Fatalf("decode maintenance update: %v", err)
	}
	if !updatePayload.Enabled {
		t.Fatalf("maintenance enabled = false, want true")
	}

	submitReq := httptest.NewRequest(http.MethodPost, "/api/image/generate", strings.NewReader(`{"prompt":"cat"}`))
	submitReq.Header.Set("Content-Type", "application/json")
	submitReq.Header.Set("Authorization", "Bearer "+userToken)
	submitRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("submit status = %d, body = %s", submitRec.Code, submitRec.Body.String())
	}
	if !strings.Contains(submitRec.Body.String(), "image_maintenance_mode") {
		t.Fatalf("submit body = %s, want maintenance code", submitRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/business/jobs", nil)
	statusReq.Header.Set("Authorization", "Bearer "+userToken)
	statusRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("jobs status = %d, body = %s", statusRec.Code, statusRec.Body.String())
	}

	restarted := NewServer(cfg, nil, nil)
	statusReq = httptest.NewRequest(http.MethodGet, "/api/system/maintenance", nil)
	statusReq.Header.Set("Authorization", "Bearer "+adminToken)
	statusRec = httptest.NewRecorder()
	restarted.Handler().ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("restarted maintenance status = %d, body = %s", statusRec.Code, statusRec.Body.String())
	}
	var restartedPayload maintenanceStatusResponse
	if err := json.Unmarshal(statusRec.Body.Bytes(), &restartedPayload); err != nil {
		t.Fatalf("decode restarted maintenance status: %v", err)
	}
	if restartedPayload.Enabled {
		t.Fatalf("restarted maintenance enabled = true, want false")
	}
}

func TestBusinessUserManagementRequiresAdmin(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	userToken := loginForTest(t, server, "tester", "tester-pass")

	req := httptest.NewRequest(http.MethodGet, "/api/business/users", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("list users status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestAdminCanDeleteBusinessUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	adminToken := loginForTest(t, server, "owner", "owner-pass")

	createReq := httptest.NewRequest(http.MethodPost, "/api/business/users", strings.NewReader(`{"email":"delete-me@example.com","password":"delete-pass","balance":8}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var createPayload struct {
		Item businessauth.User `json:"item"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create user payload: %v", err)
	}

	purgeActiveReq := httptest.NewRequest(http.MethodDelete, "/api/business/users/"+createPayload.Item.ID+"/purge", nil)
	purgeActiveReq.SetPathValue("id", createPayload.Item.ID)
	purgeActiveReq.Header.Set("Authorization", "Bearer "+adminToken)
	purgeActiveRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(purgeActiveRec, purgeActiveReq)
	if purgeActiveRec.Code != http.StatusBadRequest {
		t.Fatalf("purge active user status = %d, want %d, body = %s", purgeActiveRec.Code, http.StatusBadRequest, purgeActiveRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/business/users/"+createPayload.Item.ID, nil)
	deleteReq.SetPathValue("id", createPayload.Item.ID)
	deleteReq.Header.Set("Authorization", "Bearer "+adminToken)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete user status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}
	var deletePayload struct {
		Item businessauth.User `json:"item"`
	}
	if err := json.Unmarshal(deleteRec.Body.Bytes(), &deletePayload); err != nil {
		t.Fatalf("decode delete user payload: %v", err)
	}
	if deletePayload.Item.Status != businessauth.StatusDeleted || deletePayload.Item.DeletedAt == "" {
		t.Fatalf("delete payload item = %#v, want soft deleted", deletePayload.Item)
	}
	deletedLoginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"delete-me@example.com","password":"delete-pass"}`))
	deletedLoginReq.Header.Set("Content-Type", "application/json")
	deletedLoginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deletedLoginRec, deletedLoginReq)
	if deletedLoginRec.Code != http.StatusUnauthorized || !strings.Contains(deletedLoginRec.Body.String(), "该账号已被封禁，请联系管理员") {
		t.Fatalf("deleted login status = %d, body = %s", deletedLoginRec.Code, deletedLoginRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/business/users", nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list users status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var listPayload struct {
		Items []businessUserWithUsage `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode list users payload: %v", err)
	}
	for _, item := range listPayload.Items {
		if item.ID == createPayload.Item.ID {
			t.Fatalf("deleted user still listed: %#v", item)
		}
	}

	includeReq := httptest.NewRequest(http.MethodGet, "/api/business/users?includeDeleted=true", nil)
	includeReq.Header.Set("Authorization", "Bearer "+adminToken)
	includeRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(includeRec, includeReq)
	if includeRec.Code != http.StatusOK {
		t.Fatalf("list users include deleted status = %d, body = %s", includeRec.Code, includeRec.Body.String())
	}
	var includePayload struct {
		Items []businessUserWithUsage `json:"items"`
	}
	if err := json.Unmarshal(includeRec.Body.Bytes(), &includePayload); err != nil {
		t.Fatalf("decode include deleted users payload: %v", err)
	}
	foundDeleted := false
	for _, item := range includePayload.Items {
		if item.ID == createPayload.Item.ID && item.Status == businessauth.StatusDeleted {
			foundDeleted = true
			break
		}
	}
	if !foundDeleted {
		t.Fatalf("deleted user missing from includeDeleted list: %#v", includePayload.Items)
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/business/users/"+createPayload.Item.ID+"/restore", nil)
	restoreReq.SetPathValue("id", createPayload.Item.ID)
	restoreReq.Header.Set("Authorization", "Bearer "+adminToken)
	restoreRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("restore user status = %d, body = %s", restoreRec.Code, restoreRec.Body.String())
	}
	var restorePayload struct {
		Item businessauth.User `json:"item"`
	}
	if err := json.Unmarshal(restoreRec.Body.Bytes(), &restorePayload); err != nil {
		t.Fatalf("decode restore payload: %v", err)
	}
	if restorePayload.Item.Status != businessauth.StatusActive || restorePayload.Item.DeletedAt != "" {
		t.Fatalf("restore payload item = %#v, want active", restorePayload.Item)
	}

	deleteAgainReq := httptest.NewRequest(http.MethodDelete, "/api/business/users/"+createPayload.Item.ID, nil)
	deleteAgainReq.SetPathValue("id", createPayload.Item.ID)
	deleteAgainReq.Header.Set("Authorization", "Bearer "+adminToken)
	deleteAgainRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteAgainRec, deleteAgainReq)
	if deleteAgainRec.Code != http.StatusOK {
		t.Fatalf("delete restored user status = %d, body = %s", deleteAgainRec.Code, deleteAgainRec.Body.String())
	}
	purgeReq := httptest.NewRequest(http.MethodDelete, "/api/business/users/"+createPayload.Item.ID+"/purge", nil)
	purgeReq.SetPathValue("id", createPayload.Item.ID)
	purgeReq.Header.Set("Authorization", "Bearer "+adminToken)
	purgeRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(purgeRec, purgeReq)
	if purgeRec.Code != http.StatusOK {
		t.Fatalf("purge deleted user status = %d, body = %s", purgeRec.Code, purgeRec.Body.String())
	}
	userStore, err := businessauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("open user store after purge: %v", err)
	}
	if _, ok, err := userStore.GetUserByID(context.Background(), createPayload.Item.ID); err != nil || ok {
		t.Fatalf("GetUserByID(purged) ok=%v err=%v, want false nil", ok, err)
	}
	if err := userStore.Close(); err != nil {
		t.Fatalf("close user store after purge: %v", err)
	}
	purgedLoginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"delete-me@example.com","password":"delete-pass"}`))
	purgedLoginReq.Header.Set("Content-Type", "application/json")
	purgedLoginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(purgedLoginRec, purgedLoginReq)
	if purgedLoginRec.Code != http.StatusUnauthorized || !strings.Contains(purgedLoginRec.Body.String(), "该账号未注册") {
		t.Fatalf("purged login status = %d, body = %s", purgedLoginRec.Code, purgedLoginRec.Body.String())
	}
	recreateReq := httptest.NewRequest(http.MethodPost, "/api/business/users", strings.NewReader(`{"email":"delete-me@example.com","username":"guest-new","password":"delete-pass"}`))
	recreateReq.Header.Set("Content-Type", "application/json")
	recreateReq.Header.Set("Authorization", "Bearer "+adminToken)
	recreateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(recreateRec, recreateReq)
	if recreateRec.Code != http.StatusCreated {
		t.Fatalf("recreate purged email status = %d, body = %s", recreateRec.Code, recreateRec.Body.String())
	}

	selfDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/business/users/"+businessauth.DefaultAdminUserID, nil)
	selfDeleteReq.SetPathValue("id", businessauth.DefaultAdminUserID)
	selfDeleteReq.Header.Set("Authorization", "Bearer "+adminToken)
	selfDeleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(selfDeleteRec, selfDeleteReq)
	if selfDeleteRec.Code != http.StatusBadRequest {
		t.Fatalf("self delete status = %d, want %d, body = %s", selfDeleteRec.Code, http.StatusBadRequest, selfDeleteRec.Body.String())
	}
}

func TestListBusinessUsersPurgesExpiredDeletedUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")

	rootDir := t.TempDir()
	cfg := newDatabaseServerTestConfig(t, rootDir)
	server := NewServer(cfg, nil, nil)
	adminToken := loginForTest(t, server, "owner", "owner-pass")

	authStore, err := businessauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	user, err := authStore.CreateUser(context.Background(), "expired@example.com", "expired-pass", businessauth.RoleUser)
	if err != nil {
		t.Fatalf("CreateUser() returned error: %v", err)
	}
	if _, ok, err := authStore.DeleteUser(context.Background(), user.ID); err != nil || !ok {
		t.Fatalf("DeleteUser() ok=%v err=%v", ok, err)
	}
	if err := authStore.Close(); err != nil {
		t.Fatalf("close auth store: %v", err)
	}
	db, err := database.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open postgres database: %v", err)
	}
	expiredAt := time.Now().UTC().Add(-businessauth.DeletedUserRetention - time.Hour).Format(time.RFC3339Nano)
	if _, err := db.ExecContext(context.Background(), `UPDATE business_users SET deleted_at = $1 WHERE id = $2`, expiredAt, user.ID); err != nil {
		t.Fatalf("mark user expired: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close postgres database: %v", err)
	}

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("open credit store: %v", err)
	}
	if _, _, err := creditStore.SetBalance(context.Background(), user.ID, 10, "test_balance"); err != nil {
		t.Fatalf("SetBalance() returned error: %v", err)
	}
	if err := creditStore.Close(); err != nil {
		t.Fatalf("close credit store: %v", err)
	}

	imageStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("open image store: %v", err)
	}
	if _, err := imageStore.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "expired-conversation",
		UserID: user.ID,
		Title:  "Expired",
	}); err != nil {
		t.Fatalf("UpsertConversation() returned error: %v", err)
	}
	if _, err := imageStore.SaveGeneration(context.Background(), businessimage.Generation{
		ID:             "expired-generation",
		UserID:         user.ID,
		ConversationID: "expired-conversation",
		TurnID:         "expired-turn",
		Prompt:         "test",
		Model:          "gpt-image-test",
		Count:          1,
		Status:         "succeeded",
		Response:       json.RawMessage(`{"data":[{"url":"/v1/files/image/business-expired.png"}]}`),
	}); err != nil {
		t.Fatalf("SaveGeneration() returned error: %v", err)
	}
	if _, err := imageStore.SaveAsset(context.Background(), businessimage.Asset{
		ID:             "business-expired.png",
		UserID:         user.ID,
		ConversationID: "expired-conversation",
		GenerationID:   "expired-generation",
		FileName:       "business-expired.png",
	}); err != nil {
		t.Fatalf("SaveAsset() returned error: %v", err)
	}
	if err := imageStore.Close(); err != nil {
		t.Fatalf("close image store: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/business/users?includeDeleted=true", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list users status = %d, body = %s", rec.Code, rec.Body.String())
	}

	authStore, err = businessauth.NewStore(cfg)
	if err != nil {
		t.Fatalf("reopen auth store: %v", err)
	}
	if _, ok, err := authStore.GetUserByID(context.Background(), user.ID); err != nil || ok {
		t.Fatalf("GetUserByID(expired purged) ok=%v err=%v, want false nil", ok, err)
	}
	if err := authStore.Close(); err != nil {
		t.Fatalf("close auth store after purge: %v", err)
	}

	creditStore, err = businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("reopen credit store: %v", err)
	}
	ledger, err := creditStore.LedgerEntries(context.Background(), user.ID, 10)
	if err != nil {
		t.Fatalf("LedgerEntries() returned error: %v", err)
	}
	if len(ledger) != 0 {
		t.Fatalf("ledger after purge = %#v, want empty", ledger)
	}
	if err := creditStore.Close(); err != nil {
		t.Fatalf("close credit store after purge: %v", err)
	}

	imageStore, err = businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("reopen image store: %v", err)
	}
	records, total, err := imageStore.UsageRecordsPage(context.Background(), businessimage.UsageRecordFilter{UserID: user.ID}, 10, 0)
	if err != nil {
		t.Fatalf("UsageRecordsPage() returned error: %v", err)
	}
	if total != 0 || len(records) != 0 {
		t.Fatalf("usage records after purge total=%d records=%#v, want empty", total, records)
	}
	assets, _, err := imageStore.AssetsByUserPage(context.Background(), user.ID, 10, 0)
	if err != nil {
		t.Fatalf("AssetsByUserPage() returned error: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("assets after purge = %#v, want empty", assets)
	}
	if err := imageStore.Close(); err != nil {
		t.Fatalf("close image store after purge: %v", err)
	}
}

func TestConfiguredImageRoute(t *testing.T) {
	server := &Server{
		cfg: &config.Config{
			ChatGPT: config.ChatGPTConfig{
				FreeImageRoute: "responses",
				PaidImageRoute: "legacy",
			},
		},
	}

	if got := server.configuredImageRoute("Free"); got != "responses" {
		t.Fatalf("configuredImageRoute(Free) = %q, want %q", got, "responses")
	}
	if got := server.configuredImageRoute("Plus"); got != "legacy" {
		t.Fatalf("configuredImageRoute(Plus) = %q, want %q", got, "legacy")
	}
}

func TestMigrateImageFilesSkipsNestedTargetDirectory(t *testing.T) {
	rootDir := t.TempDir()
	cfg := newDatabaseServerTestConfig(t, rootDir)
	server := NewServer(cfg, nil, nil)

	oldDir := filepath.Join(rootDir, "data", "tmp", "image")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(oldDir) returned error: %v", err)
	}
	sourcePath := filepath.Join(oldDir, "sample.png")
	if err := os.WriteFile(sourcePath, []byte("image"), 0o644); err != nil {
		t.Fatalf("WriteFile(sourcePath) returned error: %v", err)
	}

	previous := configPayload{}
	previous.Storage.ImageDir = "data/tmp/image"
	next := configPayload{}
	next.Storage.ImageDir = "data/tmp/image/nested"

	if err := server.migrateImageFilesIfNeeded(previous, next); err != nil {
		t.Fatalf("migrateImageFilesIfNeeded() returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, "data", "tmp", "image", "nested", "sample.png")); err != nil {
		t.Fatalf("expected migrated file in nested dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "data", "tmp", "image", "nested", "nested", "sample.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no recursive nested file, got err=%v", err)
	}
}

func TestResolveImageFilePathUsesConfiguredAndLegacyImageDirsOnly(t *testing.T) {
	rootDir := t.TempDir()
	cfg := newDatabaseServerTestConfig(t, rootDir)
	cfg.Storage.ImageDir = "data/new-images"
	server := NewServer(cfg, nil, nil)

	currentDir := filepath.Join(rootDir, "data", "new-images")
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(currentDir) returned error: %v", err)
	}
	currentPath := filepath.Join(currentDir, "current.png")
	if err := os.WriteFile(currentPath, []byte("image"), 0o644); err != nil {
		t.Fatalf("WriteFile(currentPath) returned error: %v", err)
	}
	if got := server.resolveImageFilePath("current.png"); !strings.EqualFold(filepath.Clean(got), filepath.Clean(currentPath)) {
		t.Fatalf("resolveImageFilePath(current) = %q, want %q", got, currentPath)
	}

	legacyDir := filepath.Join(rootDir, "data", "old-images")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(legacyDir) returned error: %v", err)
	}
	otherDataPath := filepath.Join(legacyDir, "kept.png")
	if err := os.WriteFile(otherDataPath, []byte("image"), 0o644); err != nil {
		t.Fatalf("WriteFile(otherDataPath) returned error: %v", err)
	}

	if got := server.resolveImageFilePath("kept.png"); got != "" {
		t.Fatalf("resolveImageFilePath(other data dir) = %q, want empty", got)
	}

	legacyImagePath := filepath.Join(rootDir, "data", "tmp", "image", "legacy.png")
	if err := os.MkdirAll(filepath.Dir(legacyImagePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(legacy image dir) returned error: %v", err)
	}
	if err := os.WriteFile(legacyImagePath, []byte("image"), 0o644); err != nil {
		t.Fatalf("WriteFile(legacyImagePath) returned error: %v", err)
	}
	if got := server.resolveImageFilePath("legacy.png"); !strings.EqualFold(filepath.Clean(got), filepath.Clean(legacyImagePath)) {
		t.Fatalf("resolveImageFilePath(legacy) = %q, want %q", got, legacyImagePath)
	}

	configPath := filepath.Join(currentDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile(configPath) returned error: %v", err)
	}
	if got := server.resolveImageFilePath("config.toml"); got != "" {
		t.Fatalf("resolveImageFilePath(non-image) = %q, want empty", got)
	}
}

func TestBusinessImageJobsAreScopedToCurrentUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	adminToken := loginForTest(t, server, "owner", "owner-pass")
	userToken := loginForTest(t, server, "tester", "tester-pass")

	store, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("New job store returned error: %v", err)
	}
	_, err = store.Save(context.Background(), businessjobs.Job{
		ID:             "job-admin",
		UserID:         businessauth.DefaultAdminUserID,
		ConversationID: "conv-admin",
		GenerationID:   "gen-admin",
		TurnID:         "turn-admin",
		Status:         businessjobs.StatusQueued,
		Stage:          "queued",
		Prompt:         "admin prompt",
		RequestedCount: 1,
	})
	if err != nil {
		t.Fatalf("Save admin job returned error: %v", err)
	}
	_, err = store.Save(context.Background(), businessjobs.Job{
		ID:             "job-user",
		UserID:         businessauth.DefaultTestUserID,
		ConversationID: "conv-user",
		GenerationID:   "gen-user",
		TurnID:         "turn-user",
		Status:         businessjobs.StatusRunning,
		Stage:          "running",
		Prompt:         "user prompt",
		RequestedCount: 1,
	})
	if err != nil {
		t.Fatalf("Save user job returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close job store returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/business/jobs?conversationId=conv-user", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("user list jobs status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listPayload struct {
		Items []businessjobs.Job `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode list jobs payload: %v", err)
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].ID != "job-user" {
		t.Fatalf("listed jobs = %#v", listPayload.Items)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/business/jobs/job-user", nil)
	getReq.Header.Set("Authorization", "Bearer "+userToken)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("user get own job status = %d, body = %s", getRec.Code, getRec.Body.String())
	}

	forbiddenReq := httptest.NewRequest(http.MethodGet, "/api/business/jobs/job-user", nil)
	forbiddenReq.Header.Set("Authorization", "Bearer "+adminToken)
	forbiddenRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(forbiddenRec, forbiddenReq)
	if forbiddenRec.Code != http.StatusNotFound {
		t.Fatalf("admin get user job status = %d, want 404, body = %s", forbiddenRec.Code, forbiddenRec.Body.String())
	}
}

func TestAdminCanListBusinessImageJobsWithFilters(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "owner")
	t.Setenv("ADMIN_PASSWORD", "owner-pass")
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	adminToken := loginForTest(t, server, "owner", "owner-pass")

	store, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("New job store returned error: %v", err)
	}
	jobs := []businessjobs.Job{
		{
			ID:             "job-admin",
			UserID:         businessauth.DefaultAdminUserID,
			ConversationID: "conv-admin",
			GenerationID:   "gen-admin",
			TurnID:         "turn-admin",
			Platform:       "gpt-image",
			Status:         businessjobs.StatusSucceeded,
			Stage:          "finished",
			Prompt:         "admin prompt",
			RequestedCount: 1,
			CreatedAt:      "2026-05-15T00:00:00Z",
		},
		{
			ID:             "job-user-failed",
			UserID:         businessauth.DefaultTestUserID,
			ConversationID: "conv-user",
			GenerationID:   "gen-user-failed",
			TurnID:         "turn-user-failed",
			Platform:       "gemini-banana",
			Status:         businessjobs.StatusFailed,
			Stage:          "upstream",
			ErrorCode:      "provider_error",
			Prompt:         "user prompt failed",
			RequestedCount: 1,
			CreatedAt:      "2026-05-16T00:00:00Z",
		},
		{
			ID:             "job-user-running",
			UserID:         businessauth.DefaultTestUserID,
			ConversationID: "conv-user",
			GenerationID:   "gen-user-running",
			TurnID:         "turn-user-running",
			Platform:       "gpt-image",
			Status:         businessjobs.StatusRunning,
			Stage:          "upstream",
			Prompt:         "user prompt running",
			RequestedCount: 1,
			CreatedAt:      "2026-05-17T00:00:00Z",
		},
	}
	for _, job := range jobs {
		if _, err := store.Save(context.Background(), job); err != nil {
			t.Fatalf("Save job %s returned error: %v", job.ID, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close job store returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/business/admin/jobs?userId="+businessauth.DefaultTestUserID+"&status=failed&platform=gemini-banana&page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin list jobs status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Items []businessjobs.Job `json:"items"`
		Page  paginationMeta     `json:"page"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode admin job payload: %v", err)
	}
	if payload.Page.Total != 1 ||
		len(payload.Items) != 1 ||
		payload.Items[0].ID != "job-user-failed" ||
		payload.Items[0].UserID != businessauth.DefaultTestUserID ||
		!payload.Items[0].UpstreamSent ||
		payload.Items[0].UpstreamStatus != "sent" {
		t.Fatalf("admin job payload = %#v", payload)
	}
}

func TestCancelBusinessImageJobMarksQueuedJobCancelled(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	userToken := loginForTest(t, server, "tester", "tester-pass")

	store, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("New job store returned error: %v", err)
	}
	_, err = store.Save(context.Background(), businessjobs.Job{
		ID:             "job-cancel-queued",
		UserID:         businessauth.DefaultTestUserID,
		ConversationID: "conv-cancel",
		GenerationID:   "gen-cancel",
		TurnID:         "turn-cancel",
		Status:         businessjobs.StatusQueued,
		Stage:          "queued",
		Prompt:         "cancel me",
		RequestedCount: 1,
	})
	if err != nil {
		t.Fatalf("Save queued job returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close job store returned error: %v", err)
	}
	imageStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("New image store returned error: %v", err)
	}
	_, err = imageStore.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "conv-cancel",
		UserID: businessauth.DefaultTestUserID,
		Title:  "cancel",
	})
	if err != nil {
		t.Fatalf("UpsertConversation returned error: %v", err)
	}
	_, err = imageStore.SaveGeneration(context.Background(), businessimage.Generation{
		ID:             "gen-cancel",
		UserID:         businessauth.DefaultTestUserID,
		ConversationID: "conv-cancel",
		TurnID:         "turn-cancel",
		Prompt:         "cancel me",
		Model:          "gpt-image-2",
		Count:          1,
		Status:         businessjobs.StatusQueued,
	})
	if err != nil {
		t.Fatalf("SaveGeneration returned error: %v", err)
	}
	if err := imageStore.Close(); err != nil {
		t.Fatalf("Close image store returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/business/jobs/job-cancel-queued/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Item businessjobs.Job `json:"item"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode cancel payload: %v", err)
	}
	if payload.Item.Status != businessjobs.StatusCancelled {
		t.Fatalf("cancelled job status = %q, want %q", payload.Item.Status, businessjobs.StatusCancelled)
	}
	imageStore, err = businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("New image store after cancel returned error: %v", err)
	}
	detail, ok, err := imageStore.GetConversationWithGenerations(context.Background(), "conv-cancel", businessauth.DefaultTestUserID, 10)
	if err != nil || !ok {
		t.Fatalf("GetConversationWithGenerations ok=%v err=%v", ok, err)
	}
	if len(detail.Generations) != 1 || detail.Generations[0].Status != businessjobs.StatusCancelled {
		t.Fatalf("cancelled generations = %#v", detail.Generations)
	}
	_ = imageStore.Close()
}

func TestCancelBusinessImageJobRefundsBeforeUpstreamOnly(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	userToken := loginForTest(t, server, "tester", "tester-pass")

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("New credit store returned error: %v", err)
	}
	if _, _, err := creditStore.SetBalance(context.Background(), businessauth.DefaultTestUserID, 10, "test"); err != nil {
		t.Fatalf("SetBalance returned error: %v", err)
	}
	if _, _, err := creditStore.Reserve(context.Background(), businessauth.DefaultTestUserID, 1, "gen-cancel-running"); err != nil {
		t.Fatalf("Reserve running returned error: %v", err)
	}
	if _, _, err := creditStore.Reserve(context.Background(), businessauth.DefaultTestUserID, 1, "gen-cancel-upstream"); err != nil {
		t.Fatalf("Reserve upstream returned error: %v", err)
	}
	if err := creditStore.Close(); err != nil {
		t.Fatalf("Close credit store returned error: %v", err)
	}

	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("New job store returned error: %v", err)
	}
	for _, job := range []businessjobs.Job{
		{
			ID:             "job-cancel-running",
			UserID:         businessauth.DefaultTestUserID,
			ConversationID: "conv-cancel-running",
			GenerationID:   "gen-cancel-running",
			TurnID:         "turn-cancel-running",
			Status:         businessjobs.StatusRunning,
			Stage:          "running",
			Prompt:         "cancel before upstream",
			RequestedCount: 1,
			CreditReserved: 1,
			CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
			UpdatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		},
		{
			ID:             "job-cancel-upstream",
			UserID:         businessauth.DefaultTestUserID,
			ConversationID: "conv-cancel-upstream",
			GenerationID:   "gen-cancel-upstream",
			TurnID:         "turn-cancel-upstream",
			Status:         businessjobs.StatusRunning,
			Stage:          "upstream",
			Prompt:         "cancel after upstream",
			RequestedCount: 1,
			CreditReserved: 1,
			CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
			UpdatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		},
	} {
		if _, err := jobStore.Save(context.Background(), job); err != nil {
			t.Fatalf("Save job %s returned error: %v", job.ID, err)
		}
	}
	if err := jobStore.Close(); err != nil {
		t.Fatalf("Close job store returned error: %v", err)
	}

	cancelJob := func(jobID string) businessjobs.Job {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/business/jobs/"+jobID+"/cancel", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("cancel %s status = %d, body = %s", jobID, rec.Code, rec.Body.String())
		}
		var payload struct {
			Item businessjobs.Job `json:"item"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode cancel %s payload: %v", jobID, err)
		}
		return payload.Item
	}

	running := cancelJob("job-cancel-running")
	if running.Status != businessjobs.StatusCancelRequested || running.CreditRefunded != 1 {
		t.Fatalf("running cancel item = %#v, want cancel_requested with refund 1", running)
	}
	if running.UpstreamSent || running.UpstreamStatus != "pending" {
		t.Fatalf("running upstream state = %t/%q, want pending", running.UpstreamSent, running.UpstreamStatus)
	}
	upstream := cancelJob("job-cancel-upstream")
	if upstream.Status != businessjobs.StatusCancelRequested || upstream.CreditRefunded != 0 {
		t.Fatalf("upstream cancel item = %#v, want cancel_requested with refund 0", upstream)
	}
	if !upstream.UpstreamSent || upstream.UpstreamStatus != "sent" {
		t.Fatalf("upstream state = %t/%q, want sent", upstream.UpstreamSent, upstream.UpstreamStatus)
	}

	creditStore, err = businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("New credit store after cancel returned error: %v", err)
	}
	defer creditStore.Close()
	runningTotals, err := creditStore.GenerationTotals(context.Background(), businessauth.DefaultTestUserID, "gen-cancel-running")
	if err != nil {
		t.Fatalf("running generation totals: %v", err)
	}
	if runningTotals.Reserved != 1 || runningTotals.Refunded != 1 {
		t.Fatalf("running generation totals = %#v, want reserved 1 refunded 1", runningTotals)
	}
	upstreamTotals, err := creditStore.GenerationTotals(context.Background(), businessauth.DefaultTestUserID, "gen-cancel-upstream")
	if err != nil {
		t.Fatalf("upstream generation totals: %v", err)
	}
	if upstreamTotals.Reserved != 1 || upstreamTotals.Refunded != 0 {
		t.Fatalf("upstream generation totals = %#v, want reserved 1 refunded 0", upstreamTotals)
	}
}

func TestCancelBusinessImageJobActiveRunningJobFinalizesCancelled(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	userToken := loginForTest(t, server, "tester", "tester-pass")

	store, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("New job store returned error: %v", err)
	}
	_, err = store.Save(context.Background(), businessjobs.Job{
		ID:             "job-cancel-running",
		UserID:         businessauth.DefaultTestUserID,
		ConversationID: "conv-cancel",
		GenerationID:   "gen-cancel",
		TurnID:         "turn-cancel",
		Status:         businessjobs.StatusRunning,
		Stage:          "upstream",
		Prompt:         "cancel me",
		RequestedCount: 1,
	})
	if err != nil {
		t.Fatalf("Save running job returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close job store returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	unregister := server.registerActiveBusinessImageJob("job-cancel-running", cancel)
	defer unregister()

	req := httptest.NewRequest(http.MethodPost, "/api/business/jobs/job-cancel-running/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Item            businessjobs.Job `json:"item"`
		ActiveCancelled bool             `json:"activeCancelled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode cancel payload: %v", err)
	}
	if !payload.ActiveCancelled {
		t.Fatal("activeCancelled = false, want true")
	}
	if payload.Item.Status != businessjobs.StatusCancelled {
		t.Fatalf("cancelled job status = %q, want %q", payload.Item.Status, businessjobs.StatusCancelled)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("active job context was not cancelled")
	}
}

func TestStaleBusinessImageJobReconcileMarksRunningFailedAndRefundsCredits(t *testing.T) {
	t.Setenv("TEST_USERNAME", "tester")
	t.Setenv("TEST_PASSWORD", "tester-pass")
	t.Setenv("IMAGE_JOB_STALE_TIMEOUT_SECONDS", "1")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)
	userToken := loginForTest(t, server, "tester", "tester-pass")

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("New credit store returned error: %v", err)
	}
	if _, _, err := creditStore.SetBalance(context.Background(), businessauth.DefaultTestUserID, 5, "test"); err != nil {
		t.Fatalf("SetBalance returned error: %v", err)
	}
	if _, _, err := creditStore.Reserve(context.Background(), businessauth.DefaultTestUserID, 2, "gen-stale"); err != nil {
		t.Fatalf("Reserve returned error: %v", err)
	}
	if err := creditStore.Close(); err != nil {
		t.Fatalf("Close credit store returned error: %v", err)
	}

	staleUpdatedAt := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("New job store returned error: %v", err)
	}
	_, err = jobStore.Save(context.Background(), businessjobs.Job{
		ID:             "job-stale",
		UserID:         businessauth.DefaultTestUserID,
		ConversationID: "conv-stale",
		GenerationID:   "gen-stale",
		TurnID:         "turn-stale",
		Status:         businessjobs.StatusRunning,
		Stage:          "upstream",
		Prompt:         "stale job",
		RequestedCount: 1,
		CreditReserved: 2,
		CreatedAt:      staleUpdatedAt,
		QueuedAt:       staleUpdatedAt,
		StartedAt:      staleUpdatedAt,
	})
	if err != nil {
		t.Fatalf("Save running job returned error: %v", err)
	}
	if err := jobStore.Close(); err != nil {
		t.Fatalf("Close job store returned error: %v", err)
	}
	db, err := database.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open postgres database returned error: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE business_image_jobs SET updated_at = $1 WHERE id = $2`, staleUpdatedAt, "job-stale"); err != nil {
		t.Fatalf("force stale updated_at returned error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close postgres database returned error: %v", err)
	}

	imageStore, err := businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("New image store returned error: %v", err)
	}
	_, err = imageStore.UpsertConversation(context.Background(), businessimage.Conversation{
		ID:     "conv-stale",
		UserID: businessauth.DefaultTestUserID,
		Title:  "stale",
	})
	if err != nil {
		t.Fatalf("UpsertConversation returned error: %v", err)
	}
	_, err = imageStore.SaveGeneration(context.Background(), businessimage.Generation{
		ID:             "gen-stale",
		UserID:         businessauth.DefaultTestUserID,
		ConversationID: "conv-stale",
		TurnID:         "turn-stale",
		Prompt:         "stale job",
		Model:          "gpt-image-2",
		Count:          1,
		Status:         businessjobs.StatusRunning,
		CreatedAt:      staleUpdatedAt,
	})
	if err != nil {
		t.Fatalf("SaveGeneration returned error: %v", err)
	}
	if err := imageStore.Close(); err != nil {
		t.Fatalf("Close image store returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/business/jobs/job-stale", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get stale job status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Item businessjobs.Job `json:"item"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode stale job payload: %v", err)
	}
	if payload.Item.Status != businessjobs.StatusFailed || payload.Item.ErrorCode != "stale_running" {
		t.Fatalf("stale job = %#v", payload.Item)
	}

	imageStore, err = businessimage.NewStore(cfg)
	if err != nil {
		t.Fatalf("New image store after reconcile returned error: %v", err)
	}
	detail, ok, err := imageStore.GetConversationWithGenerations(context.Background(), "conv-stale", businessauth.DefaultTestUserID, 10)
	if err != nil || !ok {
		t.Fatalf("GetConversationWithGenerations ok=%v err=%v", ok, err)
	}
	if len(detail.Generations) != 1 ||
		detail.Generations[0].Status != businessjobs.StatusFailed ||
		!strings.Contains(detail.Generations[0].Error, "自动标记失败") {
		t.Fatalf("generation after reconcile = %#v", detail.Generations)
	}
	if err := imageStore.Close(); err != nil {
		t.Fatalf("Close image store after reconcile returned error: %v", err)
	}

	creditStore, err = businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("New credit store after reconcile returned error: %v", err)
	}
	summary, err := creditStore.Summary(context.Background(), businessauth.DefaultTestUserID)
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}
	if summary.Balance != 5 || summary.Spent != 0 {
		t.Fatalf("credit summary after reconcile = %#v", summary)
	}
	if err := creditStore.Close(); err != nil {
		t.Fatalf("Close credit store after reconcile returned error: %v", err)
	}
}

func TestStaleBusinessImageJobReconcileDoesNotRefundUpstreamCancel(t *testing.T) {
	t.Setenv("IMAGE_JOB_STALE_TIMEOUT_SECONDS", "1")

	cfg := newDatabaseServerTestConfig(t)
	server := NewServer(cfg, nil, nil)

	creditStore, err := businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("New credit store returned error: %v", err)
	}
	if _, _, err := creditStore.SetBalance(context.Background(), businessauth.DefaultTestUserID, 5, "test"); err != nil {
		t.Fatalf("SetBalance returned error: %v", err)
	}
	if _, _, err := creditStore.Reserve(context.Background(), businessauth.DefaultTestUserID, 2, "gen-stale-upstream-cancel"); err != nil {
		t.Fatalf("Reserve returned error: %v", err)
	}
	if err := creditStore.Close(); err != nil {
		t.Fatalf("Close credit store returned error: %v", err)
	}

	staleUpdatedAt := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	jobStore, err := businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("New job store returned error: %v", err)
	}
	_, err = jobStore.Save(context.Background(), businessjobs.Job{
		ID:             "job-stale-upstream-cancel",
		UserID:         businessauth.DefaultTestUserID,
		ConversationID: "conv-stale-upstream-cancel",
		GenerationID:   "gen-stale-upstream-cancel",
		TurnID:         "turn-stale-upstream-cancel",
		Status:         businessjobs.StatusCancelRequested,
		Stage:          "upstream",
		ErrorCode:      "cancel_requested",
		ErrorMessage:   "正在取消任务",
		Prompt:         "stale upstream cancel",
		RequestedCount: 2,
		CreditReserved: 2,
		CreatedAt:      staleUpdatedAt,
		QueuedAt:       staleUpdatedAt,
		StartedAt:      staleUpdatedAt,
	})
	if err != nil {
		t.Fatalf("Save cancel requested job returned error: %v", err)
	}
	if err := jobStore.Close(); err != nil {
		t.Fatalf("Close job store returned error: %v", err)
	}
	db, err := database.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open postgres database returned error: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE business_image_jobs SET updated_at = $1 WHERE id = $2`, staleUpdatedAt, "job-stale-upstream-cancel"); err != nil {
		t.Fatalf("force stale updated_at returned error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close postgres database returned error: %v", err)
	}

	result := server.reconcileStaleBusinessImageJobs(context.Background())
	if result.CancelRequestedCancelled != 1 {
		t.Fatalf("reconcile result = %#v, want one cancel_requested cancelled", result)
	}

	jobStore, err = businessjobs.NewStore(cfg)
	if err != nil {
		t.Fatalf("Reopen job store returned error: %v", err)
	}
	job, ok, err := jobStore.Get(context.Background(), "job-stale-upstream-cancel", businessauth.DefaultTestUserID)
	if err != nil || !ok {
		t.Fatalf("Get reconciled job ok=%v err=%v", ok, err)
	}
	if job.Status != businessjobs.StatusCancelled || job.CreditRefunded != 0 {
		t.Fatalf("reconciled job = %#v, want cancelled without refund", job)
	}
	if err := jobStore.Close(); err != nil {
		t.Fatalf("Close reopened job store returned error: %v", err)
	}

	creditStore, err = businesscredits.NewStore(cfg)
	if err != nil {
		t.Fatalf("New credit store after reconcile returned error: %v", err)
	}
	defer creditStore.Close()
	totals, err := creditStore.GenerationTotals(context.Background(), businessauth.DefaultTestUserID, "gen-stale-upstream-cancel")
	if err != nil {
		t.Fatalf("GenerationTotals returned error: %v", err)
	}
	if totals.Reserved != 2 || totals.Refunded != 0 {
		t.Fatalf("generation totals = %#v, want reserved 2 refunded 0", totals)
	}
	summary, err := creditStore.Summary(context.Background(), businessauth.DefaultTestUserID)
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}
	if summary.Balance != 3 || summary.Spent != 2 {
		t.Fatalf("credit summary after reconcile = %#v", summary)
	}
}

func loginForTest(t *testing.T, server *Server, username, password string) string {
	t.Helper()
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login %s status = %d, body = %s", username, loginRec.Code, loginRec.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode login payload: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("login token is empty")
	}
	return payload.Token
}

func TestImportImageConversationsIntoServerTarget(t *testing.T) {
	rootDir := t.TempDir()
	cfg := newDatabaseServerTestConfig(t, rootDir)
	cfg.App.AuthKey = "test-auth"
	server := NewServer(cfg, nil, nil)

	body := map[string]any{
		"items": []map[string]any{
			{
				"id":        "conv-1",
				"title":     "生成",
				"mode":      "generate",
				"prompt":    "test",
				"model":     "gpt-image-2",
				"count":     1,
				"createdAt": "2026-04-26T00:00:00Z",
				"status":    "success",
				"turns": []map[string]any{
					{
						"id":        "turn-1",
						"title":     "生成",
						"mode":      "generate",
						"prompt":    "test",
						"model":     "gpt-image-2",
						"count":     1,
						"createdAt": "2026-04-26T00:00:00Z",
						"status":    "success",
						"images": []map[string]any{
							{
								"id":       "img-1",
								"status":   "success",
								"b64_json": "aW1hZ2U=",
							},
						},
					},
				},
			},
		},
		"storage": map[string]any{
			"backend":                  "current",
			"imageDir":                 "data/import-images",
			"imageConversationStorage": "server",
			"imageDataStorage":         "server",
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/image/conversations/import", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+cfg.App.AuthKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	verifyCfg := config.New(rootDir)
	verifyCfg.Storage.Backend = "current"
	verifyCfg.Storage.ImageDir = "data/import-images"
	store, err := imagehistory.NewStore(verifyCfg)
	if err != nil {
		t.Fatalf("NewStore(verify current) returned error: %v", err)
	}
	defer store.Close()

	items, err := store.List(req.Context())
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != "conv-1" {
		t.Fatalf("imported items = %#v", items)
	}
}

func TestConfiguredImageModeTreatsLegacyMixAsStudio(t *testing.T) {
	server := &Server{
		cfg: &config.Config{
			ChatGPT: config.ChatGPTConfig{
				ImageMode: "mix",
			},
		},
	}

	if got := server.configuredImageMode(); got != "studio" {
		t.Fatalf("configuredImageMode() = %q, want %q", got, "studio")
	}
}

func TestLegacyAccountManagementRoutesAreRemoved(t *testing.T) {
	rootDir := t.TempDir()
	cfg := config.New(rootDir)
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	cfg.App.AuthKey = "test-auth"

	server := NewServer(cfg, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(`{"tokens":["token-1"]}`))
	req.Header.Set("Authorization", "Bearer "+cfg.App.AuthKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestResolveImageUpstreamModelFromConfig(t *testing.T) {
	server := &Server{
		cfg: &config.Config{
			ChatGPT: config.ChatGPTConfig{
				FreeImageModel: "auto",
				PaidImageModel: "gpt-5.4",
			},
		},
	}

	if got := server.resolveImageUpstreamModel("gpt-image-1", "Plus"); got != "gpt-5.4" {
		t.Fatalf("resolveImageUpstreamModel() = %q, want %q", got, "gpt-5.4")
	}
	if got := server.resolveImageUpstreamModel("gpt-image-2", "Free"); got != "auto" {
		t.Fatalf("resolveImageUpstreamModel() = %q, want %q", got, "auto")
	}
}

func TestResolveImageAcquireError(t *testing.T) {
	lastErr := errors.New("refresh failed")
	noAvailableErr := errors.New("read dir failed")

	tests := []struct {
		name             string
		mode             string
		err              error
		lastRetryableErr error
		wantMessage      string
		wantCode         string
	}{
		{
			name:        "cpa mode still maps empty pool when helper is used",
			mode:        "cpa",
			err:         accounts.ErrNoAvailableImageAuth,
			wantMessage: "当前没有可用的图片账号用于 CPA 模式",
			wantCode:    "no_cpa_image_accounts",
		},
		{
			name:             "retry exhaustion keeps last real error",
			mode:             "cpa",
			err:              accounts.ErrNoAvailableImageAuth,
			lastRetryableErr: lastErr,
			wantMessage:      lastErr.Error(),
		},
		{
			name:        "non sentinel error passes through",
			mode:        "cpa",
			err:         noAvailableErr,
			wantMessage: noAvailableErr.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveImageAcquireError(tt.mode, tt.err, tt.lastRetryableErr)
			if got == nil {
				t.Fatal("resolveImageAcquireError() returned nil")
			}
			if got.Error() != tt.wantMessage {
				t.Fatalf("resolveImageAcquireError() error = %q, want %q", got.Error(), tt.wantMessage)
			}
			if tt.wantCode != "" && requestErrorCode(got) != tt.wantCode {
				t.Fatalf("resolveImageAcquireError() code = %q, want %q", requestErrorCode(got), tt.wantCode)
			}
		})
	}
}

func TestIsImageRateLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "http 429", err: errors.New("backend-api failed: HTTP 429"), want: true},
		{name: "too many requests", err: errors.New("Too Many Requests"), want: true},
		{name: "rate limit", err: errors.New("rate limit exceeded"), want: true},
		{name: "quota exceeded", err: errors.New("image generation quota exceeded"), want: true},
		{name: "non rate error", err: errors.New("internal server error"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isImageRateLimitError(tt.err); got != tt.want {
				t.Fatalf("isImageRateLimitError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTransientImageStreamError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "responses sse internal error", err: errors.New("responses SSE read error: stream error: stream ID 1; INTERNAL_ERROR; received from peer"), want: true},
		{name: "unexpected eof", err: errors.New("SSE read error: unexpected EOF"), want: true},
		{name: "http2 connection lost", err: errors.New("http2: client connection lost"), want: true},
		{name: "non transient", err: errors.New("no images generated"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientImageStreamError(tt.err); got != tt.want {
				t.Fatalf("isTransientImageStreamError() = %v, want %v", got, tt.want)
			}
		})
	}
}
