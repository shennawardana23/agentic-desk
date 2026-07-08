package config

import (
	"os"
	"strconv"
	"time"
)

// getEnv, getIntEnv, and getDurationEnv are small env-var reader helpers
// shared by Load (config.go) and DatabaseConfig's field defaults
// (database.go) — kept in their own file since they're a distinct
// concern (generic env parsing) from either.

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
