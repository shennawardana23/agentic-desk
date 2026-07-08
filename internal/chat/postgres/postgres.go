// Package postgres is internal/chat's sole pgx-importing adapter, same
// layering as internal/task/postgres.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shennawardana23/agentic-desk/internal/chat"
)

// Store implements chat.Store against the pool cmd/core already opens.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps an existing pool; it does not own its lifecycle.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const sessionColumns = "id, title, created_at, updated_at"
const messageColumns = "id, session_id, role, content, reasoning, created_at"

func scanSession(row pgx.Row) (chat.ChatSession, error) {
	var s chat.ChatSession
	err := row.Scan(&s.ID, &s.Title, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func scanMessage(row pgx.Row) (chat.ChatMessage, error) {
	var m chat.ChatMessage
	err := row.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Reasoning, &m.CreatedAt)
	return m, err
}

func (s *Store) ListSessions(ctx context.Context) ([]chat.ChatSession, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+sessionColumns+" FROM chat_session ORDER BY updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list chat sessions: %w", err)
	}
	defer rows.Close()
	var sessions []chat.ChatSession
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("list chat sessions: %w", err)
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

func (s *Store) CreateSession(ctx context.Context) (chat.ChatSession, error) {
	created, err := scanSession(s.pool.QueryRow(ctx,
		"INSERT INTO chat_session DEFAULT VALUES RETURNING "+sessionColumns))
	if err != nil {
		return chat.ChatSession{}, fmt.Errorf("create chat session: %w", err)
	}
	return created, nil
}

func (s *Store) RenameSession(ctx context.Context, id, title string) (chat.ChatSession, error) {
	updated, err := scanSession(s.pool.QueryRow(ctx,
		"UPDATE chat_session SET title = $2, updated_at = now() WHERE id = $1 RETURNING "+sessionColumns,
		id, title))
	if errors.Is(err, pgx.ErrNoRows) {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	if err != nil {
		return chat.ChatSession{}, fmt.Errorf("rename chat session: %w", err)
	}
	return updated, nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM chat_session WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete chat session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return chat.ErrNotFound
	}
	return nil
}

func (s *Store) ListMessages(ctx context.Context, sessionID string) ([]chat.ChatMessage, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+messageColumns+" FROM chat_message WHERE session_id = $1 ORDER BY created_at ASC",
		sessionID)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()
	var messages []chat.ChatMessage
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("list chat messages: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// AppendMessage inserts the message and updates the parent session's
// updated_at (and, on a session's first user message, its title) in one
// transaction — chat.Store's contract requires both effects to happen
// atomically with the session-existence check.
func (s *Store) AppendMessage(ctx context.Context, sessionID, role, content, reasoning string) (chat.ChatMessage, error) {
	if err := chat.ValidateRole(role); err != nil {
		return chat.ChatMessage{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return chat.ChatMessage{}, fmt.Errorf("append chat message: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE chat_session
		 SET updated_at = now(),
		     title = CASE WHEN title = $2 AND $3 = $4 THEN LEFT($5, $6) ELSE title END
		 WHERE id = $1`,
		sessionID, chat.DefaultTitle, role, chat.RoleUser, content, chat.TitleMaxLen)
	if err != nil {
		return chat.ChatMessage{}, fmt.Errorf("append chat message: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return chat.ChatMessage{}, chat.ErrNotFound
	}

	created, err := scanMessage(tx.QueryRow(ctx,
		"INSERT INTO chat_message (session_id, role, content, reasoning) VALUES ($1, $2, $3, $4) RETURNING "+messageColumns,
		sessionID, role, content, reasoning))
	if err != nil {
		return chat.ChatMessage{}, fmt.Errorf("append chat message: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return chat.ChatMessage{}, fmt.Errorf("append chat message: %w", err)
	}
	return created, nil
}
