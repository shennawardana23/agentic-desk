// Package chat is the Chat History domain: a local, single-user log of
// chat sessions and their messages backed by Postgres
// (migrations/0003_chat.sql). Same domain/adapter split as internal/task —
// this file is driver-free, postgres/ is the only pgx importer. A
// separate package (not a widening of secondbrain.Store or task.Store) so
// existing fakes in api/mcp tests stay untouched.
package chat

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound mirrors task.ErrNotFound's role for this domain.
var ErrNotFound = errors.New("chat session not found")

// Roles a chat message may have — mirrors 0003_chat.sql's CHECK constraint.
const (
	RoleUser  = "user"
	RoleAgent = "agent"
)

// DefaultTitle is the title a session is created with, and the sentinel
// AppendMessage checks against to decide whether to auto-title a session
// from its first user message.
const DefaultTitle = "Untitled Chat"

// TitleMaxLen is how many characters of a first user message become the
// auto-generated session title (see Store.AppendMessage).
const TitleMaxLen = 60

// ChatSession is one conversation thread in the chat-history panel.
type ChatSession struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ChatMessage is one turn within a ChatSession.
type ChatMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Reasoning string    `json:"reasoning"`
	CreatedAt time.Time `json:"createdAt"`
}

// ValidateRole mirrors the DB's own CHECK constraint so a bad write fails
// with a clear domain error before touching the database.
func ValidateRole(role string) error {
	switch role {
	case RoleUser, RoleAgent:
		return nil
	default:
		return fmt.Errorf("chat: invalid role %q", role)
	}
}

// Store is the port the API layer depends on; postgres.Store implements it.
type Store interface {
	// ListSessions returns every session, newest-first (by updated_at).
	ListSessions(ctx context.Context) ([]ChatSession, error)
	// CreateSession starts a new session with the default title.
	CreateSession(ctx context.Context) (ChatSession, error)
	// RenameSession sets a session's title. Returns ErrNotFound if the
	// session does not exist.
	RenameSession(ctx context.Context, id, title string) (ChatSession, error)
	// DeleteSession removes a session and, via ON DELETE CASCADE, its
	// messages. Returns ErrNotFound if the session does not exist.
	DeleteSession(ctx context.Context, id string) error
	// ListMessages returns a session's messages, oldest-first.
	ListMessages(ctx context.Context, sessionID string) ([]ChatMessage, error)
	// AppendMessage adds a message to a session, bumps the session's
	// updated_at, and — if the session's title is still DefaultTitle and
	// role is RoleUser — sets the title to the first TitleMaxLen
	// characters of content. Returns ErrNotFound if the session does not
	// exist.
	AppendMessage(ctx context.Context, sessionID, role, content, reasoning string) (ChatMessage, error)
}
