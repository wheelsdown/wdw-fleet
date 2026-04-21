package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	ListenAddr  string
	LogLevel    string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL: getenv("WDW_DATABASE_URL", ""),
		ListenAddr:  getenv("WDW_LISTEN_ADDR", ":8080"),
		LogLevel:    getenv("WDW_LOG_LEVEL", "info"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("WDW_DATABASE_URL is required")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
