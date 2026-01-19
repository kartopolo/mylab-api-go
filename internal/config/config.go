package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    string
	Environment string
	JWTSecret   string
	JWTExpiry   int64 // dalam detik

	// Auth session storage (Laravel-like concept).
	// Default: file-based sessions under storage/sessions.
	AuthSessionDriver string
	AuthSessionFiles  string
	AuthSessionTable  string
}

// Load reads configuration from environment variables.
// It first tries to load from .env file (if exists), then from system environment.
// System environment variables take precedence over .env file.
func Load() (Config, error) {
	// Try to load .env file (optional - don't fail if not exists)
	loadEnvFile()

	cfg := Config{
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
		Environment: getenv("ENVIRONMENT", "development"),
		JWTSecret:   getenv("JWT_SECRET", "my_secret_key"),
		JWTExpiry:   getenvInt64("JWT_EXPIRY", 86400), // default 24 jam

		AuthSessionDriver: getenv("AUTH_SESSION_DRIVER", "file"),
		AuthSessionFiles:  getenv("AUTH_SESSION_FILES", "storage/sessions"),
		AuthSessionTable:  getenv("AUTH_SESSION_TABLE", "auth_sessions"),
	}

	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR is required")
	}
	return cfg, nil
}

// getenvInt64 membaca env int64, fallback ke default jika tidak valid
func getenvInt64(key string, def int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	var out int64
	_, err := fmt.Sscanf(val, "%d", &out)
	if err != nil {
		return def
	}
	return out
}

// loadEnvFile attempts to load .env file from current directory.
// Silently ignores if file doesn't exist (not an error).
func loadEnvFile() {
	// Try .env in current directory
	if err := godotenv.Load(".env"); err == nil {
		return
	}

	// Try .env in project root (if running from subdirectory)
	if rootPath, err := findProjectRoot(); err == nil {
		envPath := filepath.Join(rootPath, ".env")
		_ = godotenv.Load(envPath)
	}
}

// findProjectRoot walks up the directory tree to find go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", fmt.Errorf("project root not found")
		}
		dir = parent
	}
}

func getenv(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}
