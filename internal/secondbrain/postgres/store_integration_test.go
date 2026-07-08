//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/shennawardana23/agentic-desk/internal/migrate"
	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
	"github.com/shennawardana23/agentic-desk/internal/secondbrain/postgres"
	"github.com/shennawardana23/agentic-desk/migrations"
)

// Requires DATABASE_URL to point at a real Postgres+pgvector instance. Not
// run by the default `go test ./...` loop — no Docker/testcontainers
// available in this environment (see SESSION_HANDOFF.md). Runs against the
// actual local Postgres, which persists across test runs, so every
// assertion here is keyed by a fresh, unique identifier rather than an
// assumption about starting from an empty table.
func newTestStore(t *testing.T) (*postgres.Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := migrate.Run(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return postgres.NewStore(pool), ctx
}

func uniqueEmbedding(seed float32) []float32 {
	v := make([]float32, secondbrain.EmbeddingDim)
	v[0] = seed
	return v
}

func TestStore_ProfileRuleRoundTrip(t *testing.T) {
	store, ctx := newTestStore(t)
	key := fmt.Sprintf("integration-test-%d", time.Now().UnixNano())

	created, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
		SourceFile:  key,
		Heading:     "Section",
		LineStart:   1,
		LineEnd:     3,
		ContentHash: "abc123",
		Content:     "some rule content",
		Embedding:   uniqueEmbedding(1),
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected an assigned ID")
	}

	got, err := store.GetProfileRule(ctx, key, "Section")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ContentHash != "abc123" || len(got.Embedding) != secondbrain.EmbeddingDim {
		t.Fatalf("unexpected round-trip result: %+v", got)
	}

	// Upsert again with the same key updates in place, not a duplicate.
	updated, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
		SourceFile:  key,
		Heading:     "Section",
		LineStart:   1,
		LineEnd:     5,
		ContentHash: "def456",
		Content:     "updated rule content",
		Embedding:   uniqueEmbedding(2),
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("expected same ID %d on upsert, got %d", created.ID, updated.ID)
	}

	results, err := store.SearchProfileRulesByVector(ctx, uniqueEmbedding(2), 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
}

// TestStore_UpsertProfileRule_EmbeddingSemantics pins the fix for the
// embedding-wipe bug documented in SESSION_HANDOFF.md: a nil Embedding
// on upsert must NOT silently clobber a previously computed one when
// the row's content is unchanged, but MUST be cleared (not carried
// forward attached to content it was never computed from) when the
// content actually changed — a stale embedding matching the wrong
// content would be a worse, silent correctness bug than an honest nil.
func TestStore_UpsertProfileRule_EmbeddingSemantics(t *testing.T) {
	store, ctx := newTestStore(t)
	key := fmt.Sprintf("integration-test-embed-%d", time.Now().UnixNano())

	if _, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
		SourceFile: key, Heading: "Section",
		LineStart: 1, LineEnd: 2, ContentHash: "h1", Content: "original content",
		Embedding: uniqueEmbedding(10),
	}); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	// Same content_hash, no embedding supplied (e.g. a metadata-only
	// re-import) -> the existing embedding must survive.
	unchanged, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
		SourceFile: key, Heading: "Section",
		LineStart: 1, LineEnd: 2, ContentHash: "h1", Content: "original content",
	})
	if err != nil {
		t.Fatalf("no-op upsert: %v", err)
	}
	if len(unchanged.Embedding) != secondbrain.EmbeddingDim {
		t.Fatalf("expected embedding preserved when content unchanged, got %v", unchanged.Embedding)
	}

	// Different content_hash, no embedding supplied (the importer's
	// real path on a changed rule) -> the stale embedding must be
	// cleared, not carried forward onto content it doesn't match.
	changed, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
		SourceFile: key, Heading: "Section",
		LineStart: 1, LineEnd: 2, ContentHash: "h2", Content: "changed content",
	})
	if err != nil {
		t.Fatalf("changed-content upsert: %v", err)
	}
	if changed.Embedding != nil {
		t.Fatalf("expected embedding cleared when content changed, got %v", changed.Embedding)
	}
}

// TestStore_ListProfileRules_Pagination pins the fix for the unbounded
// get_profile finding in SESSION_HANDOFF.md: limit must actually
// truncate the result set, not just be accepted-and-ignored.
func TestStore_ListProfileRules_Pagination(t *testing.T) {
	store, ctx := newTestStore(t)
	prefix := fmt.Sprintf("integration-test-page-%d", time.Now().UnixNano())

	for _, heading := range []string{"A", "B", "C"} {
		if _, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
			SourceFile: prefix, Heading: heading,
			LineStart: 1, LineEnd: 2, ContentHash: "h-" + heading, Content: "content " + heading,
		}); err != nil {
			t.Fatalf("upsert %s: %v", heading, err)
		}
	}

	limited, err := store.ListProfileRules(ctx, 1, 0)
	if err != nil {
		t.Fatalf("list limit=1: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("expected exactly 1 row with limit=1, got %d", len(limited))
	}

	// offset must actually advance the cursor, not just be accepted
	// and ignored — the mechanism that makes a full read still
	// possible past the per-call limit cap.
	nextPage, err := store.ListProfileRules(ctx, 1, 1)
	if err != nil {
		t.Fatalf("list limit=1 offset=1: %v", err)
	}
	if len(nextPage) != 1 {
		t.Fatalf("expected exactly 1 row with limit=1 offset=1, got %d", len(nextPage))
	}
	if nextPage[0].ID == limited[0].ID {
		t.Fatalf("expected offset=1 to return a different row than offset=0, got the same ID %d", limited[0].ID)
	}

	unlimited, err := store.ListProfileRules(ctx, 0, 0)
	if err != nil {
		t.Fatalf("list default limit: %v", err)
	}
	if len(unlimited) < 3 {
		t.Fatalf("expected default limit (100) to return at least the 3 rows just inserted, got %d", len(unlimited))
	}
}

func TestStore_ProfileRuleNotFound(t *testing.T) {
	store, ctx := newTestStore(t)
	_, err := store.GetProfileRule(ctx, fmt.Sprintf("does-not-exist-%d", time.Now().UnixNano()), "nope")
	if !errors.Is(err, secondbrain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_ProjectContextRoundTrip(t *testing.T) {
	store, ctx := newTestStore(t)
	path := fmt.Sprintf("/tmp/integration-test-%d", time.Now().UnixNano())

	created, err := store.UpsertProjectContext(ctx, secondbrain.ProjectContext{
		ProjectPath: path,
		Summary:     "initial summary",
		Embedding:   uniqueEmbedding(3),
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := store.GetProjectContext(ctx, path)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID || got.Summary != "initial summary" {
		t.Fatalf("unexpected round-trip result: %+v", got)
	}
}

func TestStore_MemoryEntryAndFeedbackSignal(t *testing.T) {
	store, ctx := newTestStore(t)
	sessionID := fmt.Sprintf("integration-test-%d", time.Now().UnixNano())

	entry, err := store.CreateMemoryEntry(ctx, secondbrain.MemoryEntry{
		SessionID: sessionID,
		Role:      secondbrain.RoleUser,
		Content:   "hello",
		Embedding: uniqueEmbedding(4),
	})
	if err != nil {
		t.Fatalf("create memory entry: %v", err)
	}

	gotEntry, err := store.GetMemoryEntry(ctx, entry.ID)
	if err != nil {
		t.Fatalf("get memory entry: %v", err)
	}
	if gotEntry.Content != "hello" {
		t.Fatalf("unexpected memory entry: %+v", gotEntry)
	}

	signal, err := store.CreateFeedbackSignal(ctx, secondbrain.FeedbackSignal{
		MemoryEntryID: &entry.ID,
		Decision:      secondbrain.DecisionApprove,
		Note:          "looks right",
	})
	if err != nil {
		t.Fatalf("create feedback signal: %v", err)
	}

	gotSignal, err := store.GetFeedbackSignal(ctx, signal.ID)
	if err != nil {
		t.Fatalf("get feedback signal: %v", err)
	}
	if gotSignal.MemoryEntryID == nil || *gotSignal.MemoryEntryID != entry.ID {
		t.Fatalf("unexpected feedback signal: %+v", gotSignal)
	}
}
