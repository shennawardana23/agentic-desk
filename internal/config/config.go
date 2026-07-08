package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds settings required at process startup. Missing required
// values fail fast here rather than surfacing as a confusing error deep
// in a dependent package.
type Config struct {
	// Database holds discrete connection settings, all defaulted to a
	// local trust-auth Postgres — unlike GeminiAPIKey, there's a safe
	// default for every field here, so Database is never fail-fast.
	// Ignored when DATABASE_URL is set explicitly (see Load).
	Database DatabaseConfig
	// DatabaseURL is the DSN every existing caller (cmd/core,
	// database.Connect, postgres.NewPool) takes. Load prefers a
	// DATABASE_URL env var verbatim if set (backward compatible with
	// every doc/Makefile target that already exports one), otherwise
	// builds it from Database.DSN().
	DatabaseURL  string
	GeminiAPIKey string
	// APIAddr is where cmd/core's Core↔GUI HTTP+WS server listens.
	// Optional — defaults to "127.0.0.1:8080" rather than failing fast,
	// since unlike GeminiAPIKey there's a safe default for a single-user
	// desktop app's local API port. Loopback-only on purpose: the old
	// ":8080" default bound every interface, exposing the API to the LAN
	// (2026-07-07 security-review finding).
	APIAddr string
}

// Load reads required configuration from the environment and fails fast
// if any required value is missing. Load only reads process env vars —
// it has no opinion on *how* a value got into the environment (a real
// export, a shell profile, or a value injected by the calling binary
// before Load runs). Sourcing GEMINI_API_KEY from a persisted file for a
// double-clicked packaged app is cmd/desktop's concern (see
// cmd/desktop/secrets.go), not this package's.
func Load() (*Config, error) {
	db := DatabaseConfig{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnv("DB_PORT", "5432"),
		Username:        getEnv("DB_USER", defaultDBUsername()),
		Password:        getEnv("DB_PASSWORD", ""),
		Name:            getEnv("DB_NAME", "agentic_desk"),
		SSLMode:         getEnv("DB_SSL_MODE", "disable"),
		MaxOpenConns:    getIntEnv("DB_MAX_OPEN_CONNS", 20),
		MaxIdleConns:    getIntEnv("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 3600*time.Second),
		ConnMaxIdleTime: getDurationEnv("DB_CONN_MAX_IDLE_TIME", 600*time.Second),
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = db.DSN()
	}

	cfg := &Config{
		Database:     db,
		DatabaseURL:  databaseURL,
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		APIAddr:      os.Getenv("API_ADDR"),
	}
	if cfg.APIAddr == "" {
		cfg.APIAddr = "127.0.0.1:8080"
	}

	var missing []string
	if cfg.GeminiAPIKey == "" {
		missing = append(missing, "GEMINI_API_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}
