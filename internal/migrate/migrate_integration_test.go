//go:build integration

package migrate_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shennawardana23/agentic-desk/internal/migrate"
	"github.com/shennawardana23/agentic-desk/migrations"
)

// Requires DATABASE_URL to point at a real Postgres+pgvector instance.
// Not run by the default `go test ./...` loop — no Docker/testcontainers
// available in this environment, so this exercises the actual local
// Postgres directly instead (see SESSION_HANDOFF.md toolchain notes).
func TestRun_IdempotentAgainstRealPostgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// This targets the actual local Postgres, not a scratch container, so
	// it may already have every migration applied from a prior run — don't
	// assert anything about the first call's count, only that a second,
	// immediately-following call is always a no-op. That's the idempotency
	// property this test actually needs to prove.
	if _, err := migrate.Run(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("first run: %v", err)
	}

	second, err := migrate.Run(ctx, pool, migrations.FS)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected 0 migrations applied on re-run, got %d", second)
	}

	var tableCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_name IN ('profile_rule', 'project_context', 'memory_entry', 'feedback_signal')
	`).Scan(&tableCount); err != nil {
		t.Fatalf("verify tables: %v", err)
	}
	if tableCount != 4 {
		t.Fatalf("expected 4 tables, found %d", tableCount)
	}
}
