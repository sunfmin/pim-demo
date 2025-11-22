package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration
type Config struct {
	DatabaseURL       string
	Port              string
	Environment       string
	JaegerEndpoint    string
	JaegerServiceName string
	AssetStoragePath  string
	LogLevel          string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		Port:              getEnv("PORT", "8080"),
		Environment:       getEnv("ENV", "development"),
		JaegerEndpoint:    getEnv("JAEGER_ENDPOINT", ""),
		JaegerServiceName: getEnv("JAEGER_SERVICE_NAME", "pim-api"),
		AssetStoragePath:  getEnv("ASSET_STORAGE_PATH", "/var/pim/assets"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
	}

	// Validate required configuration
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	return cfg, nil
}

// getEnv retrieves an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
