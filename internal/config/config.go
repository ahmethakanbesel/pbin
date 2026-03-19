package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config holds all application configuration. Zero value is invalid — use Parse().
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Storage  StorageConfig
	Auth     AuthConfig
	Upload   UploadConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int    // Default: 8080
	Host string // Default: "0.0.0.0"
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Path string // Default: "./data/pbin.db"
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	Path string // Default: "./data/uploads"
}

// AuthConfig holds optional Basic Auth settings.
type AuthConfig struct {
	Enabled  bool
	Username string
	Password string
}

// UploadConfig holds file upload limits.
type UploadConfig struct {
	MaxBytes int64 // Default: 100 * 1024 * 1024 (100 MB)
}

// defaults returns a Config with all default values pre-populated.
func defaults() Config {
	return Config{
		Server:   ServerConfig{Port: 8080, Host: "0.0.0.0"},
		Database: DatabaseConfig{Path: "./data/pbin.db"},
		Storage:  StorageConfig{Path: "./data/uploads"},
		Auth:     AuthConfig{Enabled: false},
		Upload:   UploadConfig{MaxBytes: 100 * 1024 * 1024},
	}
}

// Parse loads configuration from the TOML file at configPath (if it exists),
// then applies PBIN_-prefixed environment variable overrides.
// Missing config file is not an error — defaults are used.
//
// Environment variable format: PBIN_SERVER_PORT=9090 maps to server.port
// (prefix stripped, lowercased, underscores become dots).
func Parse(configPath string) (Config, error) {
	cfg := defaults()

	k := koanf.New(".")

	// Load from TOML file if provided and the file exists
	if configPath != "" {
		if _, statErr := os.Stat(configPath); statErr == nil {
			if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
				return cfg, fmt.Errorf("loading config file %s: %w", configPath, err)
			}
		}
		// Non-existent file is acceptable — defaults are used
	}

	// Load from environment variables with PBIN_ prefix.
	// Transform: PBIN_SERVER_PORT -> server.port
	if err := k.Load(env.Provider("PBIN_", ".", func(s string) string {
		s = strings.TrimPrefix(s, "PBIN_")
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "_", ".")
		return s
	}), nil); err != nil {
		return cfg, fmt.Errorf("loading env config: %w", err)
	}

	// Apply koanf values on top of defaults (only if present in koanf)
	if k.Exists("server.port") {
		cfg.Server.Port = k.Int("server.port")
	}
	if k.Exists("server.host") {
		cfg.Server.Host = k.String("server.host")
	}
	if k.Exists("database.path") {
		cfg.Database.Path = k.String("database.path")
	}
	if k.Exists("storage.path") {
		cfg.Storage.Path = k.String("storage.path")
	}
	if k.Exists("auth.enabled") {
		cfg.Auth.Enabled = k.Bool("auth.enabled")
	}
	if k.Exists("auth.username") {
		cfg.Auth.Username = k.String("auth.username")
	}
	if k.Exists("auth.password") {
		cfg.Auth.Password = k.String("auth.password")
	}
	if k.Exists("upload.max_bytes") {
		cfg.Upload.MaxBytes = k.Int64("upload.max_bytes")
	}

	return cfg, nil
}
