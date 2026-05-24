package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"imagestudio/internal/buildinfo"
	"imagestudio/internal/database"

	"github.com/redis/go-redis/v9"
)

const (
	checkStatusPass = "pass"
	checkStatusWarn = "warn"
	checkStatusFail = "fail"
)

type startupCheckItem struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
	Hint       string `json:"hint,omitempty"`
	DurationMS int64  `json:"durationMs"`
}

type startupCheckResponse struct {
	StartedAt   string             `json:"startedAt"`
	FinishedAt  string             `json:"finishedAt"`
	Overall     string             `json:"overall"`
	PassCount   int                `json:"passCount"`
	WarnCount   int                `json:"warnCount"`
	FailCount   int                `json:"failCount"`
	Checks      []startupCheckItem `json:"checks"`
	SummaryText string             `json:"summaryText"`
}

type runtimeCPUStatus struct {
	Available    bool     `json:"available"`
	UsagePercent *float64 `json:"usagePercent,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type runtimeMemoryStatus struct {
	Available    bool     `json:"available"`
	UsedBytes    uint64   `json:"usedBytes,omitempty"`
	TotalBytes   uint64   `json:"totalBytes,omitempty"`
	UsagePercent *float64 `json:"usagePercent,omitempty"`
	Source       string   `json:"source,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type runtimeDatabaseStatus struct {
	OK            bool   `json:"ok"`
	Status        string `json:"status"`
	Driver        string `json:"driver"`
	Name          string `json:"name,omitempty"`
	DSN           string `json:"dsn,omitempty"`
	Path          string `json:"path,omitempty"`
	SizeBytes     int64  `json:"sizeBytes,omitempty"`
	OpenConns     int    `json:"openConns,omitempty"`
	InUseConns    int    `json:"inUseConns,omitempty"`
	IdleConns     int    `json:"idleConns,omitempty"`
	MaxOpenConns  int    `json:"maxOpenConns,omitempty"`
	MaxIdleConns  int    `json:"maxIdleConns,omitempty"`
	ConnLifetimeS int    `json:"connLifetimeSeconds,omitempty"`
	Error         string `json:"error,omitempty"`
}

type runtimeRedisStatus struct {
	Enabled bool   `json:"enabled"`
	OK      bool   `json:"ok,omitempty"`
	Status  string `json:"status"`
	Addr    string `json:"addr,omitempty"`
	Error   string `json:"error,omitempty"`
}

type runtimeDiskStatus struct {
	Available   bool     `json:"available"`
	Path        string   `json:"path,omitempty"`
	FreeBytes   uint64   `json:"freeBytes,omitempty"`
	TotalBytes  uint64   `json:"totalBytes,omitempty"`
	UsedPercent *float64 `json:"usedPercent,omitempty"`
	Error       string   `json:"error,omitempty"`
}

type runtimeGoStatus struct {
	Goroutines int `json:"goroutines"`
	Inflight   int `json:"inflight"`
	Queued     int `json:"queued"`
}

type runtimeCapacityStatus struct {
	MaxUserActiveJobs      int    `json:"maxUserActiveJobs"`
	MaxProviderRunningJobs int    `json:"maxProviderRunningJobs"`
	MaxQueuedJobs          int    `json:"maxQueuedJobs"`
	QueuedJobs             int64  `json:"queuedJobs"`
	Error                  string `json:"error,omitempty"`
}

type runtimeSystemStatus struct {
	CPU      runtimeCPUStatus      `json:"cpu"`
	Memory   runtimeMemoryStatus   `json:"memory"`
	Database runtimeDatabaseStatus `json:"database"`
	Redis    runtimeRedisStatus    `json:"redis"`
	Disk     runtimeDiskStatus     `json:"disk"`
	Runtime  runtimeGoStatus       `json:"runtime"`
}

type runtimeStatusResponse struct {
	Timestamp string `json:"timestamp"`
	Admission struct {
		MaxConcurrency int   `json:"maxConcurrency"`
		QueueLimit     int   `json:"queueLimit"`
		QueueTimeoutMS int64 `json:"queueTimeoutMs"`
		Inflight       int   `json:"inflight"`
		Queued         int   `json:"queued"`
	} `json:"admission"`
	Capacity runtimeCapacityStatus `json:"capacity"`
	Recent   struct {
		WindowSeconds    int    `json:"windowSeconds"`
		FailureCount     int    `json:"failureCount"`
		LastError        string `json:"lastError,omitempty"`
		LastErrorCode    string `json:"lastErrorCode,omitempty"`
		LastErrorAt      string `json:"lastErrorAt,omitempty"`
		LastErrorAccount string `json:"lastErrorAccount,omitempty"`
	} `json:"recent"`
	System runtimeSystemStatus `json:"system"`
}

type diagnosticsExportPayload struct {
	GeneratedAt  string                 `json:"generatedAt"`
	Version      map[string]string      `json:"version"`
	StartupCheck startupCheckResponse   `json:"startupCheck"`
	Runtime      runtimeStatusResponse  `json:"runtime"`
	Config       configPayload          `json:"config"`
	RequestLogs  []imageRequestLogEntry `json:"requestLogs"`
}

func (s *Server) handleStartupCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.runStartupCheck(r.Context()))
}

func (s *Server) handleRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.collectRuntimeStatus(r.Context()))
}

func (s *Server) handleExportDiagnostics(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	filename := fmt.Sprintf("image-studio-diagnostics-%s.json", now.Format("20060102-150405"))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	writeJSON(w, http.StatusOK, diagnosticsExportPayload{
		GeneratedAt: now.Format(time.RFC3339Nano),
		Version: map[string]string{
			"version":   buildinfo.ResolveVersion(s.cfg.App.Version),
			"commit":    buildinfo.Commit,
			"buildTime": buildinfo.BuildTime,
		},
		StartupCheck: s.runStartupCheck(r.Context()),
		Runtime:      s.collectRuntimeStatus(r.Context()),
		Config:       s.maskSensitiveConfig(s.buildConfigPayload()),
		RequestLogs:  s.reqLogs.list(100),
	})
}

func (s *Server) collectRuntimeStatus(ctx context.Context) runtimeStatusResponse {
	s.applyPersistedBusinessRuntimeSettings(ctx)
	now := time.Now()
	out := runtimeStatusResponse{
		Timestamp: now.Format(time.RFC3339Nano),
	}

	maxConcurrent, queueLimit, queueTimeout := s.cfg.ImageQueueConfig()
	if s.imageAdmission != nil {
		snapshot := s.imageAdmission.snapshot(maxConcurrent, queueLimit, queueTimeout)
		out.Admission.MaxConcurrency = snapshot.MaxConcurrency
		out.Admission.QueueLimit = snapshot.QueueLimit
		out.Admission.QueueTimeoutMS = snapshot.QueueTimeoutMS
		out.Admission.Inflight = snapshot.Inflight
		out.Admission.Queued = snapshot.Queued
	}
	out.Capacity = s.collectRuntimeCapacityStatus(ctx)
	out.Recent.WindowSeconds = 600
	windowStart := now.Add(-10 * time.Minute)
	for _, item := range s.reqLogs.list(200) {
		if item.Success {
			continue
		}
		if out.Recent.LastError == "" {
			out.Recent.LastError = item.Error
			out.Recent.LastErrorCode = item.ErrorCode
			out.Recent.LastErrorAt = item.FinishedAt
			out.Recent.LastErrorAccount = firstNonEmpty(item.AccountEmail, item.AccountFile)
		}
		parsedAt, parseErr := time.Parse(time.RFC3339Nano, item.StartedAt)
		if parseErr == nil && parsedAt.After(windowStart) {
			out.Recent.FailureCount++
		}
	}

	out.System = s.collectRuntimeSystemStatus(ctx, out.Admission.Inflight, out.Admission.Queued)
	return out
}

func (s *Server) collectRuntimeCapacityStatus(ctx context.Context) runtimeCapacityStatus {
	limits := businessJobCapacityLimits(s.businessSystemSettingsForContext(ctx))
	status := runtimeCapacityStatus{
		MaxUserActiveJobs:      limits.MaxUserActiveJobs,
		MaxProviderRunningJobs: limits.MaxProviderRunningJobs,
		MaxQueuedJobs:          limits.MaxQueuedJobs,
	}
	store, err := s.newBusinessJobStore()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer store.Close()
	queued, err := store.CountQueued(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.QueuedJobs = queued
	return status
}

func (s *Server) collectRuntimeSystemStatus(ctx context.Context, inflight int, queued int) runtimeSystemStatus {
	return runtimeSystemStatus{
		CPU:      collectRuntimeCPUStatus(),
		Memory:   collectRuntimeMemoryStatus(),
		Database: s.collectRuntimeDatabaseStatus(ctx),
		Redis:    s.collectRuntimeRedisStatus(ctx),
		Disk:     s.collectRuntimeDiskStatus(),
		Runtime: runtimeGoStatus{
			Goroutines: runtime.NumGoroutine(),
			Inflight:   inflight,
			Queued:     queued,
		},
	}
}

func collectRuntimeCPUStatus() runtimeCPUStatus {
	first, err := readProcCPUStat()
	if err != nil {
		return runtimeCPUStatus{Available: false, Error: err.Error()}
	}
	time.Sleep(80 * time.Millisecond)
	second, err := readProcCPUStat()
	if err != nil {
		return runtimeCPUStatus{Available: false, Error: err.Error()}
	}
	totalDelta := second.total - first.total
	idleDelta := second.idle - first.idle
	if totalDelta <= 0 || idleDelta < 0 {
		return runtimeCPUStatus{Available: false, Error: "cpu sample is unavailable"}
	}
	value := roundPercent((float64(totalDelta-idleDelta) / float64(totalDelta)) * 100)
	return runtimeCPUStatus{Available: true, UsagePercent: &value}
}

type procCPUStat struct {
	total int64
	idle  int64
}

func readProcCPUStat() (procCPUStat, error) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return procCPUStat{}, err
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "cpu ") {
		return procCPUStat{}, fmt.Errorf("cpu stat is unavailable")
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return procCPUStat{}, fmt.Errorf("cpu stat is incomplete")
	}
	var total int64
	var idle int64
	for i, rawField := range fields[1:] {
		value, parseErr := strconv.ParseInt(rawField, 10, 64)
		if parseErr != nil {
			return procCPUStat{}, parseErr
		}
		total += value
		if i == 3 || i == 4 {
			idle += value
		}
	}
	return procCPUStat{total: total, idle: idle}, nil
}

func collectRuntimeMemoryStatus() runtimeMemoryStatus {
	if used, total, ok := readCgroupMemory(); ok {
		value := roundPercent((float64(used) / float64(total)) * 100)
		return runtimeMemoryStatus{
			Available:    true,
			UsedBytes:    used,
			TotalBytes:   total,
			UsagePercent: &value,
			Source:       "cgroup",
		}
	}
	if used, total, ok := readProcMemory(); ok {
		value := roundPercent((float64(used) / float64(total)) * 100)
		return runtimeMemoryStatus{
			Available:    true,
			UsedBytes:    used,
			TotalBytes:   total,
			UsagePercent: &value,
			Source:       "system",
		}
	}

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	if stats.Sys == 0 {
		return runtimeMemoryStatus{Available: false, Error: "memory sample is unavailable"}
	}
	value := roundPercent((float64(stats.Alloc) / float64(stats.Sys)) * 100)
	return runtimeMemoryStatus{
		Available:    true,
		UsedBytes:    stats.Alloc,
		TotalBytes:   stats.Sys,
		UsagePercent: &value,
		Source:       "go",
	}
}

func readProcMemory() (uint64, uint64, bool) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	var totalKB uint64
	var availableKB uint64
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			totalKB = value
		case "MemAvailable":
			availableKB = value
		}
	}
	if totalKB == 0 || availableKB == 0 || availableKB > totalKB {
		return 0, 0, false
	}
	return (totalKB - availableKB) * 1024, totalKB * 1024, true
}

func readCgroupMemory() (uint64, uint64, bool) {
	if used, ok := readUintFile("/sys/fs/cgroup/memory.current"); ok {
		if total, ok := readUintFile("/sys/fs/cgroup/memory.max"); ok && validMemoryLimit(total) {
			return used, total, true
		}
	}
	if used, ok := readUintFile("/sys/fs/cgroup/memory/memory.usage_in_bytes"); ok {
		if total, ok := readUintFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok && validMemoryLimit(total) {
			return used, total, true
		}
	}
	return 0, 0, false
}

func readUintFile(path string) (uint64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func validMemoryLimit(value uint64) bool {
	return value > 0 && value < 1<<60
}

func (s *Server) collectRuntimeDatabaseStatus(ctx context.Context) runtimeDatabaseStatus {
	driver := strings.ToLower(strings.TrimSpace(s.cfg.Database.Driver))
	if driver == "" {
		driver = "postgres"
	}
	if database.IsPostgres(driver) {
		return s.collectRuntimePostgresStatus(ctx)
	}
	return runtimeDatabaseStatus{
		OK:     true,
		Status: "configured",
		Driver: driver,
		Name:   strings.TrimSpace(driver),
	}
}

func (s *Server) collectRuntimePostgresStatus(ctx context.Context) runtimeDatabaseStatus {
	status := runtimeDatabaseStatus{
		Status:        "checking",
		Driver:        "postgres",
		Name:          "PostgreSQL",
		DSN:           maskDatabaseDSN(s.cfg.Database.DSN),
		MaxOpenConns:  s.cfg.Database.MaxOpenConns,
		MaxIdleConns:  s.cfg.Database.MaxIdleConns,
		ConnLifetimeS: s.cfg.Database.ConnMaxLifetimeSeconds,
	}
	db := s.db
	var closeDB func()
	if db == nil {
		openCtx, cancel := context.WithTimeout(ctx, 900*time.Millisecond)
		opened, err := database.Open(openCtx, s.cfg)
		cancel()
		if err != nil {
			status.Status = "error"
			status.Error = err.Error()
			return status
		}
		db = opened
		closeDB = func() { _ = opened.Close() }
	}
	if closeDB != nil {
		defer closeDB()
	}
	pingCtx, cancel := context.WithTimeout(ctx, 900*time.Millisecond)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		status.Status = "error"
		status.Error = err.Error()
		return status
	}
	stats := db.Stats()
	status.OpenConns = stats.OpenConnections
	status.InUseConns = stats.InUse
	status.IdleConns = stats.Idle
	status.OK = true
	status.Status = "ok"
	return status
}

func maskDatabaseDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.User == nil {
		return dsn
	}
	if _, ok := parsed.User.Password(); ok {
		parsed.User = url.UserPassword(parsed.User.Username(), "******")
	}
	return parsed.String()
}

func (s *Server) collectRuntimeDiskStatus() runtimeDiskStatus {
	probePath := nearestExistingPath(s.cfg.ResolvePath(s.cfg.Storage.ImageDir))
	status := runtimeDiskStatus{
		Path: probePath,
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(probePath, &stat); err != nil {
		status.Error = err.Error()
		return status
	}
	if stat.Bsize <= 0 || stat.Blocks == 0 {
		status.Error = "disk sample is unavailable"
		return status
	}
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	free := stat.Bavail * blockSize
	if free > total {
		free = total
	}
	usedPercent := roundPercent((float64(total-free) / float64(total)) * 100)
	status.Available = true
	status.FreeBytes = free
	status.TotalBytes = total
	status.UsedPercent = &usedPercent
	return status
}

func nearestExistingPath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" || cleaned == "." {
		return "."
	}
	for {
		if _, err := os.Stat(cleaned); err == nil {
			return cleaned
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			return cleaned
		}
		cleaned = parent
	}
}

func (s *Server) collectRuntimeRedisStatus(ctx context.Context) runtimeRedisStatus {
	status := runtimeRedisStatus{
		Enabled: runtimeRedisEnabled(s),
		Status:  "disabled",
		Addr:    strings.TrimSpace(s.cfg.Storage.RedisAddr),
	}
	if !status.Enabled {
		return status
	}
	if status.Addr == "" {
		status.Status = "error"
		status.Error = "redis addr is empty"
		return status
	}

	client := redis.NewClient(&redis.Options{
		Addr:     status.Addr,
		Password: s.cfg.Storage.RedisPassword,
		DB:       s.cfg.Storage.RedisDB,
	})
	defer client.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 900*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		status.Status = "error"
		status.Error = err.Error()
		return status
	}
	status.OK = true
	status.Status = "ok"
	return status
}

func runtimeRedisEnabled(s *Server) bool {
	return strings.EqualFold(strings.TrimSpace(s.cfg.Storage.Backend), "redis") ||
		strings.EqualFold(strings.TrimSpace(s.cfg.Storage.ConfigBackend), "redis") ||
		strings.EqualFold(strings.TrimSpace(s.cfg.Storage.ImageStorage), "redis") ||
		strings.EqualFold(strings.TrimSpace(s.cfg.Storage.ImageConversationStorage), "redis") ||
		strings.EqualFold(strings.TrimSpace(s.cfg.Storage.ImageDataStorage), "redis") ||
		strings.EqualFold(strings.TrimSpace(s.cfg.JobQueue.Backend), "redis")
}

func roundPercent(value float64) float64 {
	return math.Round(value*10) / 10
}

func (s *Server) runStartupCheck(ctx context.Context) startupCheckResponse {
	startedAt := time.Now()
	result := startupCheckResponse{
		StartedAt: startedAt.Format(time.RFC3339Nano),
		Checks:    make([]startupCheckItem, 0, 3),
	}

	addCheck := func(key, label string, run func() (string, string, string)) {
		checkStartedAt := time.Now()
		status, detail, hint := run()
		result.Checks = append(result.Checks, startupCheckItem{
			Key:        key,
			Label:      label,
			Status:     status,
			Detail:     detail,
			Hint:       hint,
			DurationMS: time.Since(checkStartedAt).Milliseconds(),
		})
	}

	addCheck("server", "后端服务", func() (string, string, string) {
		return checkStatusPass, fmt.Sprintf("服务已启动：%s:%d", strings.TrimSpace(s.cfg.Server.Host), s.cfg.Server.Port), ""
	})

	addCheck("api_providers", "API 接入", func() (string, string, string) {
		store, err := s.newBusinessProviderStore()
		if err != nil {
			return checkStatusFail, fmt.Sprintf("读取 API 接入失败：%v", err), ""
		}
		defer store.Close()
		if err := store.EnsureConfigProvider(ctx, s.cfg); err != nil {
			return checkStatusFail, fmt.Sprintf("初始化 API 接入失败：%v", err), ""
		}
		items, err := store.List(ctx)
		if err != nil {
			return checkStatusFail, fmt.Sprintf("读取 API 接入失败：%v", err), ""
		}
		available := 0
		for _, item := range items {
			if item.Enabled {
				available++
			}
		}
		if len(items) == 0 {
			return checkStatusFail, "当前没有 API 接入", "请先在账号管理里添加 gpt-image 或 gemini-banana 接入"
		}
		if available == 0 {
			return checkStatusFail, fmt.Sprintf("API 接入总数 %d，启用 0", len(items)), "请至少启用一个 API 接入"
		}
		return checkStatusPass, fmt.Sprintf("API 接入总数 %d，启用 %d", len(items), available), ""
	})

	for _, item := range result.Checks {
		switch item.Status {
		case checkStatusPass:
			result.PassCount++
		case checkStatusWarn:
			result.WarnCount++
		case checkStatusFail:
			result.FailCount++
		}
	}

	switch {
	case result.FailCount > 0:
		result.Overall = checkStatusFail
		result.SummaryText = fmt.Sprintf("检测完成：%d 项通过，%d 项警告，%d 项失败", result.PassCount, result.WarnCount, result.FailCount)
	case result.WarnCount > 0:
		result.Overall = checkStatusWarn
		result.SummaryText = fmt.Sprintf("检测完成：%d 项通过，%d 项警告", result.PassCount, result.WarnCount)
	default:
		result.Overall = checkStatusPass
		result.SummaryText = fmt.Sprintf("检测完成：%d 项全部通过", result.PassCount)
	}
	result.FinishedAt = time.Now().Format(time.RFC3339Nano)
	return result
}

func (s *Server) maskSensitiveConfig(payload configPayload) configPayload {
	payload.App.APIKey = maskSecret(payload.App.APIKey)
	payload.App.AuthKey = maskSecret(payload.App.AuthKey)
	payload.Sync.ManagementKey = maskSecret(payload.Sync.ManagementKey)
	payload.Proxy.URL = maskURLAuth(payload.Proxy.URL)
	payload.APIAccess.APIKey = maskSecret(payload.APIAccess.APIKey)
	payload.NewAPI.Password = maskSecret(payload.NewAPI.Password)
	payload.NewAPI.AccessToken = maskSecret(payload.NewAPI.AccessToken)
	payload.NewAPI.SessionCookie = maskSecret(payload.NewAPI.SessionCookie)
	payload.Sub2API.Password = maskSecret(payload.Sub2API.Password)
	payload.Sub2API.APIKey = maskSecret(payload.Sub2API.APIKey)
	return payload
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 6 {
		return "***"
	}
	return trimmed[:3] + strings.Repeat("*", len(trimmed)-6) + trimmed[len(trimmed)-3:]
}

func maskURLAuth(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	if parsed.User == nil {
		return trimmed
	}
	username := parsed.User.Username()
	if username == "" {
		username = "***"
	}
	parsed.User = url.UserPassword(username, "***")
	return parsed.String()
}
