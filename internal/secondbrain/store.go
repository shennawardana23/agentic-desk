package secondbrain

import "context"

// Store persists and retrieves Second Brain domain entities. It is the
// only dependency internal/agentloop, internal/importer, and
// internal/mcp/server take on Second Brain persistence — none of them
// import pgx directly. internal/secondbrain/postgres is the sole real
// implementation.
type Store interface {
	// UpsertProfileRule inserts or updates a rule keyed by
	// (SourceFile, Heading), returning it with ID/timestamps populated.
	UpsertProfileRule(ctx context.Context, rule ProfileRule) (ProfileRule, error)
	GetProfileRule(ctx context.Context, sourceFile, heading string) (ProfileRule, error)
	// ListProfileRules returns a page of stored rules, ordered by
	// (SourceFile, Heading) — the read path for callers (e.g. the MCP
	// server's get_profile tool) that don't know a specific key up
	// front. limit <= 0 defaults to 100; implementations cap limit at
	// 200 regardless of what's requested.
	ListProfileRules(ctx context.Context, limit, offset int) ([]ProfileRule, error)
	SearchProfileRulesByVector(ctx context.Context, embedding []float32, k int) ([]ProfileRule, error)

	// UpsertProjectContext inserts or updates a context keyed by
	// ProjectPath, returning it with ID/timestamps populated.
	UpsertProjectContext(ctx context.Context, projectContext ProjectContext) (ProjectContext, error)
	GetProjectContext(ctx context.Context, projectPath string) (ProjectContext, error)
	SearchProjectContextsByVector(ctx context.Context, embedding []float32, k int) ([]ProjectContext, error)

	// CreateMemoryEntry appends a session turn; memory entries are an
	// append-only log, never updated.
	CreateMemoryEntry(ctx context.Context, entry MemoryEntry) (MemoryEntry, error)
	GetMemoryEntry(ctx context.Context, id int64) (MemoryEntry, error)
	SearchMemoryEntriesByVector(ctx context.Context, embedding []float32, k int) ([]MemoryEntry, error)

	// CreateFeedbackSignal appends a human decision; feedback signals
	// are an append-only log, never updated.
	CreateFeedbackSignal(ctx context.Context, signal FeedbackSignal) (FeedbackSignal, error)
	GetFeedbackSignal(ctx context.Context, id int64) (FeedbackSignal, error)
}
