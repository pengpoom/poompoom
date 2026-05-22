package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultUpdaterPort = 7071
	maxLogBytes        = 64 * 1024
)

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type config struct {
	host           string
	port           int
	token          string
	deployDir      string
	composeFile    string
	composeProject string
	service        string
	timeout        time.Duration
}

type updateJob struct {
	ID         string   `json:"id,omitempty"`
	Status     string   `json:"status"`
	Phase      string   `json:"phase,omitempty"`
	Error      string   `json:"error,omitempty"`
	Logs       []string `json:"logs,omitempty"`
	StartedAt  string   `json:"started_at,omitempty"`
	FinishedAt string   `json:"finished_at,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

type updater struct {
	cfg config
	mu  sync.Mutex
	job updateJob
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := loadConfig()
	u := &updater{
		cfg: cfg,
		job: updateJob{Status: "idle", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("GET /status", u.withAuth(u.handleStatus))
	mux.HandleFunc("POST /update", u.withAuth(u.handleUpdate))

	addr := net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port))
	slog.Info("image-studio updater listening", slog.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("updater stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func loadConfig() config {
	port := defaultUpdaterPort
	if value := strings.TrimSpace(os.Getenv("UPDATER_PORT")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			port = parsed
		}
	}
	timeoutSeconds := envInt("IMAGE_STUDIO_UPDATE_TIMEOUT_SECONDS", 900)
	return config{
		host:           envString("UPDATER_HOST", "0.0.0.0"),
		port:           port,
		token:          strings.TrimSpace(os.Getenv("IMAGE_STUDIO_UPDATER_TOKEN")),
		deployDir:      strings.TrimSpace(os.Getenv("IMAGE_STUDIO_DEPLOY_DIR")),
		composeFile:    envString("IMAGE_STUDIO_COMPOSE_FILE", "docker-compose.yml"),
		composeProject: envString("IMAGE_STUDIO_COMPOSE_PROJECT", envString("COMPOSE_PROJECT_NAME", "image-studio")),
		service:        envString("IMAGE_STUDIO_COMPOSE_SERVICE", "studio"),
		timeout:        time.Duration(timeoutSeconds) * time.Second,
	}
}

func (u *updater) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(u.cfg.token) == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"error": "updater token is not configured",
			})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if token == "" {
			token = strings.TrimSpace(r.Header.Get("X-Updater-Token"))
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(u.cfg.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "updater authorization is invalid"})
			return
		}
		next(w, r)
	}
}

func (u *updater) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, u.statusPayload())
}

func (u *updater) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if err := u.validateRuntime(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	u.mu.Lock()
	if u.job.Status == "running" {
		job := u.job
		u.mu.Unlock()
		writeJSON(w, http.StatusAccepted, map[string]any{
			"started": false,
			"job":     job,
		})
		return
	}

	job := updateJob{
		ID:        "upd-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Status:    "running",
		Phase:     "queued",
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	u.job = job
	u.mu.Unlock()

	go u.runUpdate(job.ID)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"started": true,
		"job":     job,
	})
}

func (u *updater) runUpdate(jobID string) {
	slog.Info("docker update started", slog.String("job", jobID))
	ctx, cancel := context.WithTimeout(context.Background(), u.cfg.timeout)
	defer cancel()

	if err := u.runStep(ctx, jobID, "pull", "docker", u.composeArgs("pull", u.cfg.service)...); err != nil {
		u.finishJob(jobID, "failed", err)
		return
	}
	if err := u.runStep(ctx, jobID, "recreate", "docker", u.composeArgs("up", "-d", "--remove-orphans", u.cfg.service)...); err != nil {
		u.finishJob(jobID, "failed", err)
		return
	}
	if err := u.runStep(ctx, jobID, "inspect", "docker", u.composeArgs("ps", u.cfg.service)...); err != nil {
		u.finishJob(jobID, "failed", err)
		return
	}
	u.finishJob(jobID, "succeeded", nil)
	slog.Info("docker update finished", slog.String("job", jobID))
}

func (u *updater) composeArgs(args ...string) []string {
	base := []string{"compose", "-p", u.cfg.composeProject, "-f", u.cfg.composeFile}
	return append(base, args...)
}

func (u *updater) runStep(ctx context.Context, jobID, phase, name string, args ...string) error {
	u.updateJob(jobID, func(job *updateJob) {
		job.Phase = phase
	})

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = u.cfg.deployDir
	out, err := cmd.CombinedOutput()
	logText := trimLog(string(out))

	u.updateJob(jobID, func(job *updateJob) {
		if logText != "" {
			job.Logs = append(job.Logs, fmt.Sprintf("$ %s %s\n%s", name, strings.Join(args, " "), logText))
			if len(job.Logs) > 8 {
				job.Logs = job.Logs[len(job.Logs)-8:]
			}
		}
	})

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		if logText != "" {
			return fmt.Errorf("%s failed: %w: %s", phase, err, logText)
		}
		return fmt.Errorf("%s failed: %w", phase, err)
	}
	return nil
}

func (u *updater) finishJob(jobID, status string, err error) {
	u.updateJob(jobID, func(job *updateJob) {
		job.Status = status
		job.Phase = status
		job.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err != nil {
			job.Error = err.Error()
		}
	})
	if err != nil {
		slog.Error("docker update failed", slog.String("job", jobID), slog.Any("error", err))
	}
}

func (u *updater) updateJob(jobID string, mutate func(*updateJob)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.job.ID != jobID {
		return
	}
	mutate(&u.job)
	u.job.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func (u *updater) statusPayload() map[string]any {
	u.mu.Lock()
	job := u.job
	u.mu.Unlock()

	payload := map[string]any{
		"enabled":         u.runtimeReady(),
		"deploy_dir":      u.cfg.deployDir,
		"compose_file":    u.cfg.composeFile,
		"compose_project": u.cfg.composeProject,
		"service":         u.cfg.service,
		"job":             job,
	}
	if err := u.validateRuntime(); err != nil {
		payload["reason"] = err.Error()
	}
	return payload
}

func (u *updater) runtimeReady() bool {
	return u.validateRuntime() == nil
}

func (u *updater) validateRuntime() error {
	if strings.TrimSpace(u.cfg.token) == "" {
		return errors.New("IMAGE_STUDIO_UPDATER_TOKEN is not configured")
	}
	if strings.TrimSpace(u.cfg.deployDir) == "" {
		return errors.New("IMAGE_STUDIO_DEPLOY_DIR is not configured")
	}
	if !filepath.IsAbs(u.cfg.deployDir) {
		return errors.New("IMAGE_STUDIO_DEPLOY_DIR must be an absolute host path")
	}
	if !serviceNamePattern.MatchString(u.cfg.service) {
		return errors.New("IMAGE_STUDIO_COMPOSE_SERVICE contains invalid characters")
	}
	if !serviceNamePattern.MatchString(u.cfg.composeProject) {
		return errors.New("IMAGE_STUDIO_COMPOSE_PROJECT contains invalid characters")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("docker CLI is not available in updater container")
	}
	if stat, err := os.Stat(u.cfg.deployDir); err != nil || !stat.IsDir() {
		return fmt.Errorf("deployment directory is unavailable: %s", u.cfg.deployDir)
	}
	composePath := filepath.Join(u.cfg.deployDir, u.cfg.composeFile)
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("compose file is unavailable: %s", composePath)
	}
	return nil
}

func trimLog(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLogBytes {
		return value
	}
	return "...日志已截断...\n" + value[len(value)-maxLogBytes:]
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
