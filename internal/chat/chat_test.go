package chat_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/shennawardana23/agentic-desk/internal/chat"
)

// memStore is a minimal in-memory chat.Store used to test domain behavior
// without a real database — same approach as
// internal/secondbrain/secondbrain_test.go's memStore. It reimplements
// AppendMessage's bump-updated_at/auto-title contract exactly as the
// postgres adapter must, so a test against this fake exercises the same
// domain rule the real adapter is on the hook for.
type memStore struct {
	sessions map[string]chat.ChatSession
	messages map[string][]chat.ChatMessage
	nextID   int
}

func newMemStore() *memStore {
	return &memStore{sessions: map[string]chat.ChatSession{}, messages: map[string][]chat.ChatMessage{}}
}

var _ chat.Store = (*memStore)(nil)

func (s *memStore) newID() string {
	s.nextID++
	return "id-" + time.Now().Format("150405") + "-" + string(rune('a'+s.nextID))
}

func (s *memStore) ListSessions(_ context.Context) ([]chat.ChatSession, error) {
	sessions := make([]chat.ChatSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

func (s *memStore) CreateSession(_ context.Context) (chat.ChatSession, error) {
	now := time.Now()
	sess := chat.ChatSession{ID: s.newID(), Title: chat.DefaultTitle, CreatedAt: now, UpdatedAt: now}
	s.sessions[sess.ID] = sess
	return sess, nil
}

func (s *memStore) RenameSession(_ context.Context, id, title string) (chat.ChatSession, error) {
	sess, ok := s.sessions[id]
	if !ok {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	sess.Title = title
	sess.UpdatedAt = time.Now()
	s.sessions[id] = sess
	return sess, nil
}

func (s *memStore) DeleteSession(_ context.Context, id string) error {
	if _, ok := s.sessions[id]; !ok {
		return chat.ErrNotFound
	}
	delete(s.sessions, id)
	delete(s.messages, id)
	return nil
}

func (s *memStore) ListMessages(_ context.Context, sessionID string) ([]chat.ChatMessage, error) {
	if _, ok := s.sessions[sessionID]; !ok {
		return nil, chat.ErrNotFound
	}
	msgs := s.messages[sessionID]
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].CreatedAt.Before(msgs[j].CreatedAt) })
	return msgs, nil
}

func (s *memStore) AppendMessage(_ context.Context, sessionID, role, content, reasoning string) (chat.ChatMessage, error) {
	sess, ok := s.sessions[sessionID]
	if !ok {
		return chat.ChatMessage{}, chat.ErrNotFound
	}
	if err := chat.ValidateRole(role); err != nil {
		return chat.ChatMessage{}, err
	}
	now := time.Now()
	msg := chat.ChatMessage{
		ID: s.newID(), SessionID: sessionID, Role: role, Content: content, Reasoning: reasoning, CreatedAt: now,
	}
	s.messages[sessionID] = append(s.messages[sessionID], msg)

	sess.UpdatedAt = now
	if sess.Title == chat.DefaultTitle && role == chat.RoleUser {
		runes := []rune(content)
		if len(runes) > chat.TitleMaxLen {
			runes = runes[:chat.TitleMaxLen]
		}
		sess.Title = string(runes)
	}
	s.sessions[sessionID] = sess

	return msg, nil
}

func TestValidateRole(t *testing.T) {
	cases := []struct {
		role    string
		wantErr bool
	}{
		{chat.RoleUser, false},
		{chat.RoleAgent, false},
		{"admin", true},
		{"", true},
	}
	for _, c := range cases {
		if err := chat.ValidateRole(c.role); (err != nil) != c.wantErr {
			t.Errorf("ValidateRole(%q) = %v, wantErr %v", c.role, err, c.wantErr)
		}
	}
}

func TestMemStore_CreateSession_DefaultsTitle(t *testing.T) {
	store := newMemStore()
	sess, err := store.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.Title != chat.DefaultTitle {
		t.Fatalf("expected default title %q, got %q", chat.DefaultTitle, sess.Title)
	}
	if sess.ID == "" {
		t.Fatal("expected an assigned ID")
	}
}

func TestMemStore_RenameSession(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	sess, _ := store.CreateSession(ctx)

	renamed, err := store.RenameSession(ctx, sess.ID, "My renamed chat")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Title != "My renamed chat" {
		t.Fatalf("expected renamed title, got %q", renamed.Title)
	}
}

func TestMemStore_RenameSession_NotFound(t *testing.T) {
	store := newMemStore()
	_, err := store.RenameSession(context.Background(), "missing", "x")
	if !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemStore_DeleteSession_CascadesMessages(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	sess, _ := store.CreateSession(ctx)
	if _, err := store.AppendMessage(ctx, sess.ID, chat.RoleUser, "hello", ""); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := store.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.ListMessages(ctx, sess.ID); !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMemStore_DeleteSession_NotFound(t *testing.T) {
	store := newMemStore()
	if err := store.DeleteSession(context.Background(), "missing"); !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemStore_AppendMessage_AutoTitlesFromFirstUserMessage(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	sess, _ := store.CreateSession(ctx)

	longContent := "this is a rather long first message that will exceed the sixty character title budget for sure"
	msg, err := store.AppendMessage(ctx, sess.ID, chat.RoleUser, longContent, "")
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if msg.Role != chat.RoleUser || msg.Content != longContent {
		t.Fatalf("unexpected message: %+v", msg)
	}

	updated := store.sessions[sess.ID]
	want := string([]rune(longContent)[:chat.TitleMaxLen])
	if updated.Title != want {
		t.Fatalf("expected title %q, got %q", want, updated.Title)
	}
}

func TestMemStore_AppendMessage_DoesNotRetitleAfterFirstUserMessage(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	sess, _ := store.CreateSession(ctx)

	if _, err := store.AppendMessage(ctx, sess.ID, chat.RoleUser, "first message", ""); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := store.AppendMessage(ctx, sess.ID, chat.RoleUser, "second message", ""); err != nil {
		t.Fatalf("append: %v", err)
	}

	updated := store.sessions[sess.ID]
	if updated.Title != "first message" {
		t.Fatalf("expected title to stay as first message, got %q", updated.Title)
	}
}

func TestMemStore_AppendMessage_AgentMessageDoesNotSetTitle(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	sess, _ := store.CreateSession(ctx)

	if _, err := store.AppendMessage(ctx, sess.ID, chat.RoleAgent, "an agent reply", "thinking..."); err != nil {
		t.Fatalf("append: %v", err)
	}

	updated := store.sessions[sess.ID]
	if updated.Title != chat.DefaultTitle {
		t.Fatalf("expected title to remain default, got %q", updated.Title)
	}
}

func TestMemStore_AppendMessage_NotFound(t *testing.T) {
	store := newMemStore()
	_, err := store.AppendMessage(context.Background(), "missing", chat.RoleUser, "hi", "")
	if !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemStore_AppendMessage_InvalidRole(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	sess, _ := store.CreateSession(ctx)
	if _, err := store.AppendMessage(ctx, sess.ID, "admin", "hi", ""); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestMemStore_ListSessions_NewestFirst(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()

	first, _ := store.CreateSession(ctx)
	time.Sleep(2 * time.Millisecond)
	second, _ := store.CreateSession(ctx)
	time.Sleep(2 * time.Millisecond)
	// Bump first's updated_at past second's by appending a message to it.
	if _, err := store.AppendMessage(ctx, first.ID, chat.RoleUser, "hi", ""); err != nil {
		t.Fatalf("append: %v", err)
	}

	sessions, err := store.ListSessions(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].ID != first.ID {
		t.Fatalf("expected most recently updated session (%s) first, got %s", first.ID, sessions[0].ID)
	}
	_ = second
}

func TestMemStore_ListMessages_OldestFirst(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	sess, _ := store.CreateSession(ctx)

	if _, err := store.AppendMessage(ctx, sess.ID, chat.RoleUser, "one", ""); err != nil {
		t.Fatalf("append: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := store.AppendMessage(ctx, sess.ID, chat.RoleAgent, "two", "reasoning"); err != nil {
		t.Fatalf("append: %v", err)
	}

	msgs, err := store.ListMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Content != "one" || msgs[1].Content != "two" {
		t.Fatalf("unexpected message order: %+v", msgs)
	}
}
