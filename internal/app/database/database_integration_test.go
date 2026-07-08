//go:build integration

package database_test

import (
	"context"
	"os"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/app/database"
)

// Requires DATABASE_URL to point at a real Postgres+pgvector instance —
// same convention as every other integration test in this repo.
func TestConnect_IdempotentAgainstRealPostgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, _, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	pool.Close()

	// A second connect against the same, now-current database must
	// apply zero migrations — the idempotency property, proven live.
	pool2, applied, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("second connect: %v", err)
	}
	defer pool2.Close()
	if applied != 0 {
		t.Fatalf("expected 0 migrations applied on the second connect, got %d", applied)
	}

	if err := pool2.Ping(ctx); err != nil {
		t.Fatalf("expected a usable pool: %v", err)
	}
}
