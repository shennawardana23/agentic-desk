// Package secondbrain holds the domain types and Store interface for
// Archipelago's second brain: imported profile rules, per-project context,
// session memory, and human feedback signals. It depends on nothing but
// the standard library — every framework/database concern lives in
// internal/secondbrain/postgres, the only adapter that implements Store.
package secondbrain

import (
	"errors"
	"fmt"
	"time"
)

// EmbeddingDim is the fixed embedding width used by every table (see
// migrations/0001_init.sql, VECTOR(768)) — gemini-embedding-2's output
// size, confirmed against the live SDK in Phase 4.
const EmbeddingDim = 768

// Role values allowed for MemoryEntry.Role, matching the memory_entry
// table's CHECK constraint.
const (
	RoleUser  = "user"
	RoleAgent = "agent"
)

// Decision values allowed for FeedbackSignal.Decision, matching the
// feedback_signal table's CHECK constraint.
const (
	DecisionApprove = "approve"
	DecisionCorrect = "correct"
	DecisionReject  = "reject"
)

// ErrNotFound is returned by Store Get methods when no matching row exists.
var ErrNotFound = errors.New("secondbrain: not found")

func validateEmbedding(embedding []float32) error {
	if embedding != nil && len(embedding) != EmbeddingDim {
		return fmt.Errorf("embedding must have %d dimensions, got %d", EmbeddingDim, len(embedding))
	}
	return nil
}

// ProfileRule is one imported rule or section, traceable back to its
// source file and heading (populated by internal/importer, Phase 3).
type ProfileRule struct {
	ID          int64
	SourceFile  string
	Heading     string
	LineStart   int
	LineEnd     int
	ContentHash string
	Content     string
	Overridden  bool
	Embedding   []float32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate reports whether r has the fields required to persist it.
func (r ProfileRule) Validate() error {
	if r.SourceFile == "" {
		return errors.New("profile rule: source_file is required")
	}
	if r.Heading == "" {
		return errors.New("profile rule: heading is required")
	}
	if r.ContentHash == "" {
		return errors.New("profile rule: content_hash is required")
	}
	if r.Content == "" {
		return errors.New("profile rule: content is required")
	}
	if r.LineStart < 0 || r.LineEnd < r.LineStart {
		return fmt.Errorf("profile rule: invalid line range [%d, %d]", r.LineStart, r.LineEnd)
	}
	return validateEmbedding(r.Embedding)
}

// ProjectContext is the running summary Second Brain keeps for one
// project directory.
type ProjectContext struct {
	ID          int64
	ProjectPath string
	Summary     string
	Embedding   []float32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate reports whether c has the fields required to persist it.
func (c ProjectContext) Validate() error {
	if c.ProjectPath == "" {
		return errors.New("project context: project_path is required")
	}
	return validateEmbedding(c.Embedding)
}

// MemoryEntry is one turn of a session transcript, embedded for later
// semantic search.
type MemoryEntry struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	Embedding []float32
	CreatedAt time.Time
}

// Validate reports whether m has the fields required to persist it.
func (m MemoryEntry) Validate() error {
	if m.SessionID == "" {
		return errors.New("memory entry: session_id is required")
	}
	if m.Role != RoleUser && m.Role != RoleAgent {
		return fmt.Errorf("memory entry: role must be %q or %q, got %q", RoleUser, RoleAgent, m.Role)
	}
	if m.Content == "" {
		return errors.New("memory entry: content is required")
	}
	return validateEmbedding(m.Embedding)
}

// FeedbackSignal is a human decision (approve/correct/reject) captured at
// an Agent Loop HITL escalation, or an explicit thumbs-up/down on a
// committed result (Phase 6).
type FeedbackSignal struct {
	ID            int64
	MemoryEntryID *int64
	Decision      string
	Note          string
	CreatedAt     time.Time
}

// Validate reports whether f has the fields required to persist it.
func (f FeedbackSignal) Validate() error {
	switch f.Decision {
	case DecisionApprove, DecisionCorrect, DecisionReject:
		return nil
	default:
		return fmt.Errorf("feedback signal: decision must be %q, %q or %q, got %q",
			DecisionApprove, DecisionCorrect, DecisionReject, f.Decision)
	}
}
