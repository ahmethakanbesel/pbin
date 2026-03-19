package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse("")
	if err != nil {
		t.Fatalf("Parse() with empty path returned error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("default Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("default Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Storage.Path == "" {
		t.Error("default StoragePath is empty, want non-empty")
	}
	if cfg.Database.Path == "" {
		t.Error("default DBPath is empty, want non-empty")
	}
	if cfg.Upload.MaxBytes != 100*1024*1024 {
		t.Errorf("default MaxBytes = %d, want %d", cfg.Upload.MaxBytes, 100*1024*1024)
	}
}

func TestParseTOMLOverride(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "pbin.toml")

	content := "[server]\nport = 9090\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}

	cfg, err := Parse(cfgFile)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Port = %d, want 9090 (from TOML)", cfg.Server.Port)
	}
	// Defaults should still apply for unset values
	if cfg.Storage.Path == "" {
		t.Error("StoragePath is empty, want default value")
	}
	if cfg.Database.Path == "" {
		t.Error("DBPath is empty, want default value")
	}
	if cfg.Upload.MaxBytes != 100*1024*1024 {
		t.Errorf("MaxBytes = %d, want default 100MB", cfg.Upload.MaxBytes)
	}
}

func TestParseEnvOverride(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "pbin.toml")

	content := "[server]\nport = 9090\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}

	t.Setenv("PBIN_SERVER_PORT", "7777")

	cfg, err := Parse(cfgFile)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if cfg.Server.Port != 7777 {
		t.Errorf("Port = %d, want 7777 (from env var, overriding TOML 9090)", cfg.Server.Port)
	}
}

func TestParseNonExistentFileUsesDefaults(t *testing.T) {
	cfg, err := Parse("/nonexistent/path/pbin.toml")
	if err != nil {
		t.Fatalf("Parse() with non-existent file returned error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (default)", cfg.Server.Port)
	}
}

func TestParseStorageAndDBPathNonEmpty(t *testing.T) {
	cfg, err := Parse("")
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if cfg.Storage.Path == "" {
		t.Error("StoragePath is empty")
	}
	if cfg.Database.Path == "" {
		t.Error("DBPath is empty")
	}
}
