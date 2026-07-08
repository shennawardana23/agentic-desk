package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// Store is the pgvector-backed implementation of secondbrain.Store.
type Store struct {
	pool *pgxpool.Pool
}

var _ secondbrain.Store = (*Store)(nil)

// NewStore wraps an already-connected pool. Use NewPool to obtain one
// with the pgvector types this Store depends on already registered.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func toVectorParam(embedding []float32) *pgvector.Vector {
	if embedding == nil {
		return nil
	}
	v := pgvector.NewVector(embedding)
	return &v
}

func fromVectorParam(v *pgvector.Vector) []float32 {
	if v == nil {
		return nil
	}
	return v.Slice()
}

// maxSearchK bounds every vector search's LIMIT so an external caller
// (e.g. the MCP server's search_memory tool) can't request an
// arbitrarily large result set.
const maxSearchK = 50

func validateSearch(embedding []float32, k int) (int, error) {
	if len(embedding) != secondbrain.EmbeddingDim {
		return 0, fmt.Errorf("search: query embedding must have %d dimensions, got %d", secondbrain.EmbeddingDim, len(embedding))
	}
	if k <= 0 {
		return 0, fmt.Errorf("search: k must be positive, got %d", k)
	}
	if k > maxSearchK {
		k = maxSearchK
	}
	return k, nil
}

func (s *Store) UpsertProfileRule(ctx context.Context, rule secondbrain.ProfileRule) (secondbrain.ProfileRule, error) {
	if err := rule.Validate(); err != nil {
		return secondbrain.ProfileRule{}, err
	}

	var embedding *pgvector.Vector
	err := s.pool.QueryRow(ctx, `
		INSERT INTO profile_rule (source_file, heading, line_start, line_end, content_hash, content, overridden, embedding)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (source_file, heading) DO UPDATE SET
			line_start = EXCLUDED.line_start,
			line_end = EXCLUDED.line_end,
			content_hash = EXCLUDED.content_hash,
			content = EXCLUDED.content,
			overridden = EXCLUDED.overridden,
			embedding = CASE
				WHEN EXCLUDED.content_hash IS DISTINCT FROM profile_rule.content_hash THEN EXCLUDED.embedding
				ELSE COALESCE(EXCLUDED.embedding, profile_rule.embedding)
			END,
			updated_at = now()
		RETURNING id, line_start, line_end, overridden, embedding, created_at, updated_at
	`, rule.SourceFile, rule.Heading, rule.LineStart, rule.LineEnd, rule.ContentHash, rule.Content, rule.Overridden, toVectorParam(rule.Embedding),
	).Scan(&rule.ID, &rule.LineStart, &rule.LineEnd, &rule.Overridden, &embedding, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return secondbrain.ProfileRule{}, fmt.Errorf("upsert profile rule: %w", err)
	}
	rule.Embedding = fromVectorParam(embedding)
	return rule, nil
}

func (s *Store) GetProfileRule(ctx context.Context, sourceFile, heading string) (secondbrain.ProfileRule, error) {
	rule := secondbrain.ProfileRule{SourceFile: sourceFile, Heading: heading}
	var embedding *pgvector.Vector
	err := s.pool.QueryRow(ctx, `
		SELECT id, line_start, line_end, content_hash, content, overridden, embedding, created_at, updated_at
		FROM profile_rule WHERE source_file = $1 AND heading = $2
	`, sourceFile, heading).Scan(&rule.ID, &rule.LineStart, &rule.LineEnd, &rule.ContentHash, &rule.Content, &rule.Overridden, &embedding, &rule.CreatedAt, &rule.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return secondbrain.ProfileRule{}, secondbrain.ErrNotFound
	}
	if err != nil {
		return secondbrain.ProfileRule{}, fmt.Errorf("get profile rule: %w", err)
	}
	rule.Embedding = fromVectorParam(embedding)
	return rule, nil
}

// maxListLimit bounds how many rows a single ListProfileRules call can
// return, so an external caller (e.g. the MCP server's get_profile
// tool) can't force the entire table into one response.
const maxListLimit = 200

func (s *Store) ListProfileRules(ctx context.Context, limit, offset int) ([]secondbrain.ProfileRule, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source_file, heading, line_start, line_end, content_hash, content, overridden, embedding, created_at, updated_at
		FROM profile_rule
		ORDER BY source_file, heading
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list profile rules: %w", err)
	}
	defer rows.Close()

	var rules []secondbrain.ProfileRule
	for rows.Next() {
		var rule secondbrain.ProfileRule
		var vec *pgvector.Vector
		if err := rows.Scan(&rule.ID, &rule.SourceFile, &rule.Heading, &rule.LineStart, &rule.LineEnd, &rule.ContentHash, &rule.Content, &rule.Overridden, &vec, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan profile rule: %w", err)
		}
		rule.Embedding = fromVectorParam(vec)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list profile rules: %w", err)
	}
	return rules, nil
}

func (s *Store) SearchProfileRulesByVector(ctx context.Context, embedding []float32, k int) ([]secondbrain.ProfileRule, error) {
	k, err := validateSearch(embedding, k)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source_file, heading, line_start, line_end, content_hash, content, overridden, embedding, created_at, updated_at
		FROM profile_rule
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT $2
	`, pgvector.NewVector(embedding), k)
	if err != nil {
		return nil, fmt.Errorf("search profile rules: %w", err)
	}
	defer rows.Close()

	var rules []secondbrain.ProfileRule
	for rows.Next() {
		var rule secondbrain.ProfileRule
		var vec *pgvector.Vector
		if err := rows.Scan(&rule.ID, &rule.SourceFile, &rule.Heading, &rule.LineStart, &rule.LineEnd, &rule.ContentHash, &rule.Content, &rule.Overridden, &vec, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan profile rule: %w", err)
		}
		rule.Embedding = fromVectorParam(vec)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search profile rules: %w", err)
	}
	return rules, nil
}

func (s *Store) UpsertProjectContext(ctx context.Context, pc secondbrain.ProjectContext) (secondbrain.ProjectContext, error) {
	if err := pc.Validate(); err != nil {
		return secondbrain.ProjectContext{}, err
	}

	var embedding *pgvector.Vector
	err := s.pool.QueryRow(ctx, `
		INSERT INTO project_context (project_path, summary, embedding)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_path) DO UPDATE SET
			summary = EXCLUDED.summary,
			embedding = CASE
				WHEN EXCLUDED.summary IS DISTINCT FROM project_context.summary THEN EXCLUDED.embedding
				ELSE COALESCE(EXCLUDED.embedding, project_context.embedding)
			END,
			updated_at = now()
		RETURNING id, summary, embedding, created_at, updated_at
	`, pc.ProjectPath, pc.Summary, toVectorParam(pc.Embedding)).Scan(&pc.ID, &pc.Summary, &embedding, &pc.CreatedAt, &pc.UpdatedAt)
	if err != nil {
		return secondbrain.ProjectContext{}, fmt.Errorf("upsert project context: %w", err)
	}
	pc.Embedding = fromVectorParam(embedding)
	return pc, nil
}

func (s *Store) GetProjectContext(ctx context.Context, projectPath string) (secondbrain.ProjectContext, error) {
	pc := secondbrain.ProjectContext{ProjectPath: projectPath}
	var embedding *pgvector.Vector
	err := s.pool.QueryRow(ctx, `
		SELECT id, summary, embedding, created_at, updated_at FROM project_context WHERE project_path = $1
	`, projectPath).Scan(&pc.ID, &pc.Summary, &embedding, &pc.CreatedAt, &pc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return secondbrain.ProjectContext{}, secondbrain.ErrNotFound
	}
	if err != nil {
		return secondbrain.ProjectContext{}, fmt.Errorf("get project context: %w", err)
	}
	pc.Embedding = fromVectorParam(embedding)
	return pc, nil
}

func (s *Store) SearchProjectContextsByVector(ctx context.Context, embedding []float32, k int) ([]secondbrain.ProjectContext, error) {
	k, err := validateSearch(embedding, k)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_path, summary, embedding, created_at, updated_at FROM project_context
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT $2
	`, pgvector.NewVector(embedding), k)
	if err != nil {
		return nil, fmt.Errorf("search project contexts: %w", err)
	}
	defer rows.Close()

	var contexts []secondbrain.ProjectContext
	for rows.Next() {
		var pc secondbrain.ProjectContext
		var vec *pgvector.Vector
		if err := rows.Scan(&pc.ID, &pc.ProjectPath, &pc.Summary, &vec, &pc.CreatedAt, &pc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project context: %w", err)
		}
		pc.Embedding = fromVectorParam(vec)
		contexts = append(contexts, pc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search project contexts: %w", err)
	}
	return contexts, nil
}

func (s *Store) CreateMemoryEntry(ctx context.Context, entry secondbrain.MemoryEntry) (secondbrain.MemoryEntry, error) {
	if err := entry.Validate(); err != nil {
		return secondbrain.MemoryEntry{}, err
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO memory_entry (session_id, role, content, embedding)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, entry.SessionID, entry.Role, entry.Content, toVectorParam(entry.Embedding)).Scan(&entry.ID, &entry.CreatedAt)
	if err != nil {
		return secondbrain.MemoryEntry{}, fmt.Errorf("create memory entry: %w", err)
	}
	return entry, nil
}

func (s *Store) GetMemoryEntry(ctx context.Context, id int64) (secondbrain.MemoryEntry, error) {
	var entry secondbrain.MemoryEntry
	var embedding *pgvector.Vector
	err := s.pool.QueryRow(ctx, `
		SELECT id, session_id, role, content, embedding, created_at FROM memory_entry WHERE id = $1
	`, id).Scan(&entry.ID, &entry.SessionID, &entry.Role, &entry.Content, &embedding, &entry.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return secondbrain.MemoryEntry{}, secondbrain.ErrNotFound
	}
	if err != nil {
		return secondbrain.MemoryEntry{}, fmt.Errorf("get memory entry: %w", err)
	}
	entry.Embedding = fromVectorParam(embedding)
	return entry, nil
}

func (s *Store) SearchMemoryEntriesByVector(ctx context.Context, embedding []float32, k int) ([]secondbrain.MemoryEntry, error) {
	k, err := validateSearch(embedding, k)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, session_id, role, content, embedding, created_at FROM memory_entry
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT $2
	`, pgvector.NewVector(embedding), k)
	if err != nil {
		return nil, fmt.Errorf("search memory entries: %w", err)
	}
	defer rows.Close()

	var entries []secondbrain.MemoryEntry
	for rows.Next() {
		var entry secondbrain.MemoryEntry
		var vec *pgvector.Vector
		if err := rows.Scan(&entry.ID, &entry.SessionID, &entry.Role, &entry.Content, &vec, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory entry: %w", err)
		}
		entry.Embedding = fromVectorParam(vec)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search memory entries: %w", err)
	}
	return entries, nil
}

func (s *Store) CreateFeedbackSignal(ctx context.Context, signal secondbrain.FeedbackSignal) (secondbrain.FeedbackSignal, error) {
	if err := signal.Validate(); err != nil {
		return secondbrain.FeedbackSignal{}, err
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO feedback_signal (memory_entry_id, decision, note)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, signal.MemoryEntryID, signal.Decision, signal.Note).Scan(&signal.ID, &signal.CreatedAt)
	if err != nil {
		return secondbrain.FeedbackSignal{}, fmt.Errorf("create feedback signal: %w", err)
	}
	return signal, nil
}

func (s *Store) GetFeedbackSignal(ctx context.Context, id int64) (secondbrain.FeedbackSignal, error) {
	var signal secondbrain.FeedbackSignal
	err := s.pool.QueryRow(ctx, `
		SELECT id, memory_entry_id, decision, note, created_at FROM feedback_signal WHERE id = $1
	`, id).Scan(&signal.ID, &signal.MemoryEntryID, &signal.Decision, &signal.Note, &signal.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return secondbrain.FeedbackSignal{}, secondbrain.ErrNotFound
	}
	if err != nil {
		return secondbrain.FeedbackSignal{}, fmt.Errorf("get feedback signal: %w", err)
	}
	return signal, nil
}
