package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed config.defaults.toml
var defaultConfigTemplate string

type BootstrapResult struct {
	ExampleWritten  bool
	OverrideWritten bool
}

func DefaultTemplate() string {
	return defaultConfigTemplate
}

func EnsureRuntimeFiles(paths Paths) (BootstrapResult, error) {
	result := BootstrapResult{}
	template := strings.TrimSpace(defaultConfigTemplate)
	if template == "" {
		return result, fmt.Errorf("embedded default config template is empty")
	}

	dataDir := filepath.Dir(paths.Override)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return result, fmt.Errorf("create data dir: %w", err)
	}

	if !fileExists(paths.Defaults) {
		if err := os.WriteFile(paths.Defaults, []byte(defaultConfigTemplate), 0o644); err != nil {
			if !fileExists(paths.Override) {
				return result, fmt.Errorf("write example config: %w", err)
			}
		} else {
			result.ExampleWritten = true
		}
	}

	if !fileExists(paths.Override) {
		if err := os.WriteFile(paths.Override, []byte(defaultConfigTemplate), 0o644); err != nil {
			return result, fmt.Errorf("write local config: %w", err)
		}
		result.OverrideWritten = true
	}

	return result, nil
}
