package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRuntimeFilesDoesNotRewriteExistingExampleConfig(t *testing.T) {
	rootDir := t.TempDir()
	paths := Paths{
		Root:     rootDir,
		Defaults: filepath.Join(rootDir, "data", exampleConfigFile),
		Override: filepath.Join(rootDir, "data", userConfigFile),
	}

	if err := os.MkdirAll(filepath.Dir(paths.Defaults), 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := os.WriteFile(paths.Defaults, []byte("existing example"), 0o444); err != nil {
		t.Fatalf("write example config: %v", err)
	}

	result, err := EnsureRuntimeFiles(paths)
	if err != nil {
		t.Fatalf("EnsureRuntimeFiles returned error: %v", err)
	}
	if result.ExampleWritten {
		t.Fatal("expected existing example config to be preserved without rewrite")
	}
	if !result.OverrideWritten {
		t.Fatal("expected missing config.toml to be generated")
	}

	raw, err := os.ReadFile(paths.Defaults)
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	if string(raw) != "existing example" {
		t.Fatalf("expected example config contents to stay unchanged, got %q", string(raw))
	}
}
