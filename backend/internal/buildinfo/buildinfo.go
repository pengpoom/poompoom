package buildinfo

import (
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"

	versionPattern   = regexp.MustCompile(`(?i)^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
	versionOnce      sync.Once
	cachedGitVersion string
)

func ResolveVersion(fallback string) string {
	version := strings.TrimSpace(Version)
	fallback = strings.TrimSpace(fallback)

	if strings.EqualFold(version, "dev") || version == "" {
		return resolveVersionValue(version, fallback, detectGitVersion())
	}

	return resolveVersionValue(version, fallback, "")
}

func resolveVersionValue(version, fallback, gitVersion string) string {
	version = strings.TrimSpace(version)
	fallback = strings.TrimSpace(fallback)
	gitVersion = strings.TrimSpace(gitVersion)

	if isSemanticVersionLike(version) {
		return version
	}
	if gitVersion != "" {
		return gitVersion
	}
	if version != "" && !strings.EqualFold(version, "dev") {
		return version
	}
	if fallback != "" {
		return fallback
	}
	if version != "" {
		return version
	}
	return "unknown"
}

func detectGitVersion() string {
	versionOnce.Do(func() {
		output, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
		if err != nil {
			return
		}
		cachedGitVersion = strings.TrimSpace(string(output))
	})
	return cachedGitVersion
}

func isSemanticVersionLike(value string) bool {
	return versionPattern.MatchString(strings.TrimSpace(value))
}
