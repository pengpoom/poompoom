package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/buildinfo"
)

const (
	defaultVersionRepo = "your-org/image-studio"
	versionCacheTTL    = 20 * time.Minute
)

var versionNumberPattern = regexp.MustCompile(`\d+`)

type versionCheckInfo struct {
	CurrentVersion string       `json:"current_version"`
	LatestVersion  string       `json:"latest_version"`
	HasUpdate      bool         `json:"has_update"`
	ReleaseInfo    *releaseInfo `json:"release_info,omitempty"`
	Cached         bool         `json:"cached"`
	Warning        string       `json:"warning,omitempty"`
}

type releaseInfo struct {
	Name        string `json:"name"`
	Body        string `json:"body,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	HTMLURL     string `json:"html_url"`
}

func (s *Server) versionPayload(ctx context.Context, force bool) map[string]any {
	current := buildinfo.ResolveVersion(s.cfg.App.Version)
	info := s.checkLatestVersion(ctx, current, force)
	payload := map[string]any{
		"version":         current,
		"current_version": info.CurrentVersion,
		"latest_version":  info.LatestVersion,
		"has_update":      info.HasUpdate,
		"cached":          info.Cached,
		"commit":          buildinfo.Commit,
		"buildTime":       buildinfo.BuildTime,
	}
	if info.ReleaseInfo != nil {
		payload["release_info"] = info.ReleaseInfo
	}
	if strings.TrimSpace(info.Warning) != "" {
		payload["warning"] = info.Warning
	}
	return payload
}

func (s *Server) checkLatestVersion(ctx context.Context, current string, force bool) versionCheckInfo {
	now := time.Now()
	if !force {
		s.versionMu.Lock()
		if s.versionCache != nil && now.Before(s.versionCacheUntil) {
			cached := *s.versionCache
			s.versionMu.Unlock()
			cached.CurrentVersion = current
			cached.HasUpdate = compareSemanticVersions(current, cached.LatestVersion) < 0
			cached.Cached = true
			return cached
		}
		s.versionMu.Unlock()
	}

	info, err := fetchLatestVersion(ctx, current)
	if err != nil {
		s.versionMu.Lock()
		if s.versionCache != nil {
			cached := *s.versionCache
			s.versionMu.Unlock()
			cached.CurrentVersion = current
			cached.HasUpdate = compareSemanticVersions(current, cached.LatestVersion) < 0
			cached.Cached = true
			cached.Warning = "使用缓存版本信息：" + err.Error()
			return cached
		}
		s.versionMu.Unlock()
		return versionCheckInfo{
			CurrentVersion: current,
			LatestVersion:  current,
			HasUpdate:      false,
			Warning:        err.Error(),
		}
	}

	s.versionMu.Lock()
	cached := info
	cached.Cached = false
	s.versionCache = &cached
	s.versionCacheUntil = now.Add(versionCacheTTL)
	s.versionMu.Unlock()
	return info
}

func fetchLatestVersion(ctx context.Context, current string) (versionCheckInfo, error) {
	repo := apiEnvString("IMAGE_STUDIO_VERSION_REPO", defaultVersionRepo)
	tag, err := fetchGitHubLatestTag(ctx, repo)
	if err != nil {
		return versionCheckInfo{}, fmt.Errorf("检查 GitHub 最新版本失败：%v", err)
	}

	latest := normalizeVersion(tag.Name)
	release, releaseErr := fetchGitHubReleaseByTag(ctx, repo, tag.Name)
	info := &releaseInfo{
		Name:    tag.Name,
		HTMLURL: "https://github.com/" + repo + "/releases/tag/" + tag.Name,
	}
	if releaseErr == nil {
		info.Name = firstNonEmpty(release.Name, release.TagName, tag.Name)
		info.Body = release.Body
		info.PublishedAt = release.PublishedAt
		info.HTMLURL = firstNonEmpty(release.HTMLURL, info.HTMLURL)
	}
	return versionCheckInfo{
		CurrentVersion: current,
		LatestVersion:  latest,
		HasUpdate:      compareSemanticVersions(current, latest) < 0,
		ReleaseInfo:    info,
	}, nil
}

type githubReleaseResponse struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
}

type githubTagResponse struct {
	Name string `json:"name"`
}

func fetchGitHubReleaseByTag(ctx context.Context, repo, tag string) (githubReleaseResponse, error) {
	var release githubReleaseResponse
	if err := fetchGitHubJSON(ctx, "https://api.github.com/repos/"+repo+"/releases/tags/"+tag, &release); err != nil {
		return githubReleaseResponse{}, err
	}
	return release, nil
}

func fetchGitHubLatestTag(ctx context.Context, repo string) (githubTagResponse, error) {
	var tags []githubTagResponse
	if err := fetchGitHubJSON(ctx, "https://api.github.com/repos/"+repo+"/tags?per_page=1", &tags); err != nil {
		return githubTagResponse{}, err
	}
	if len(tags) == 0 || strings.TrimSpace(tags[0].Name) == "" {
		return githubTagResponse{}, fmt.Errorf("latest tag is empty")
	}
	return tags[0], nil
}

func fetchGitHubJSON(ctx context.Context, url string, target any) error {
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "poom-studio-updater")
	if token := strings.TrimSpace(os.Getenv("IMAGE_STUDIO_GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "refs/tags/")
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return "unknown"
	}
	return value
}

func compareSemanticVersions(current, latest string) int {
	currentParts := parseSemanticVersion(current)
	latestParts := parseSemanticVersion(latest)
	for i := 0; i < 3; i++ {
		if currentParts[i] < latestParts[i] {
			return -1
		}
		if currentParts[i] > latestParts[i] {
			return 1
		}
	}
	return 0
}

func parseSemanticVersion(value string) [3]int {
	matches := versionNumberPattern.FindAllString(value, 3)
	result := [3]int{}
	for i, match := range matches {
		parsed, err := strconv.Atoi(match)
		if err == nil {
			result[i] = parsed
		}
	}
	return result
}
