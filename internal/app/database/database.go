// Package database is this app's single connection-lifecycle bootstrap:
// open a pgvector-aware Postgres pool and bring the schema up to date.
// It deliberately owns nothing else — the repository pattern itself
// (domain types + Store interface, and the only pgx-importing
// implementation) lives in internal/secondbrain and
// internal/secondbrain/postgres. This mirrors the split verified in
// zk-org/zk (a comparable Go project with a real database layer):
// internal/adapter/sqlite/db.go there owns connection setup, while
// separate *_dao.go files in the same adapter package are the
// repository implementation. Keeping that boundary here means cmd/core
// depends on "how do I get a ready pool" without also depending on
// which migrations exist or how pgvector types get registered.
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shennawardana23/agentic-desk/internal/migrate"
	"github.com/shennawardana23/agentic-desk/internal/secondbrain/postgres"
	"github.com/shennawardana23/agentic-desk/migrations"
)

// Connect opens a pgvector-aware connection pool against databaseURL
// and applies any pending migrations, returning a pool ready for
// repository use (e.g. postgres.NewStore(pool)) plus the count of
// migrations applied this call (0 on an already-current database —
// callers use this to confirm idempotency the same way Phase 1's
// verification did).
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, int, error) {
	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("database: connect: %w", err)
	}

	applied, err := migrate.Run(ctx, pool, migrations.FS)
	if err != nil {
		pool.Close()
		return nil, 0, fmt.Errorf("database: migrate: %w", err)
	}
	return pool, applied, nil
}
