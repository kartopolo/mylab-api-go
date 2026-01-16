package config

import (
	"fmt"
	"os"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
	}

	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR is required")
	}

	return cfg, nil
}

func getenv(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}
