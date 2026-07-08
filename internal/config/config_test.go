package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		dbURL     string
		geminiKey string
		wantErr   bool
	}{
		{"all set, explicit DATABASE_URL", "postgres://x", "key", false},
		{"gemini key set, DATABASE_URL unset — DB defaults cover it", "", "key", false},
		{"missing gemini key", "postgres://x", "", true},
		{"missing gemini key, DATABASE_URL also unset", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", tt.dbURL)
			t.Setenv("GEMINI_API_KEY", tt.geminiKey)

			_, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_DatabaseURLPrecedence(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://explicit-url")
	t.Setenv("GEMINI_API_KEY", "key")
	t.Setenv("DB_HOST", "should-be-ignored")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.DatabaseURL != "postgres://explicit-url" {
		t.Errorf("DatabaseURL = %q, want explicit DATABASE_URL to win over DB_* vars", cfg.DatabaseURL)
	}
}

func TestLoad_BuildsDatabaseURLFromSplitFields(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("GEMINI_API_KEY", "key")
	t.Setenv("DB_HOST", "dbhost")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "dbuser")
	t.Setenv("DB_NAME", "dbname")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if !strings.Contains(cfg.DatabaseURL, "dbhost:5433") || !strings.Contains(cfg.DatabaseURL, "dbuser@") || !strings.Contains(cfg.DatabaseURL, "/dbname") {
		t.Errorf("DatabaseURL = %q, want it built from DB_HOST/DB_PORT/DB_USER/DB_NAME", cfg.DatabaseURL)
	}
	if cfg.Database.Host != "dbhost" || cfg.Database.Port != "5433" {
		t.Errorf("Database = %+v, want Host=dbhost Port=5433", cfg.Database)
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	d := DatabaseConfig{
		Host: "localhost", Port: "5432", Username: "alice", Password: "s3cret",
		Name: "agentic_desk", SSLMode: "disable",
		MaxOpenConns: 20, MaxIdleConns: 5,
		ConnMaxLifetime: 3600 * time.Second, ConnMaxIdleTime: 600 * time.Second,
	}
	dsn := d.DSN()
	for _, want := range []string{"postgres://alice:s3cret@localhost:5432/agentic_desk", "sslmode=disable", "pool_max_conns=20", "pool_min_conns=5"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN() = %q, want it to contain %q", dsn, want)
		}
	}
}

func TestDatabaseConfig_DSN_NoPasswordOmitsColon(t *testing.T) {
	d := DatabaseConfig{Host: "localhost", Port: "5432", Username: "alice", Name: "agentic_desk", SSLMode: "disable"}
	if dsn := d.DSN(); !strings.Contains(dsn, "postgres://alice@localhost") {
		t.Errorf("DSN() = %q, want no-password form \"alice@localhost\"", dsn)
	}
}
