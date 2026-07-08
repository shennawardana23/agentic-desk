package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shennawardana23/agentic-desk/internal/chat"
)

// fakeChatHistoryStore implements chat.Store with an in-memory map, enough
// to exercise every /chat/sessions route without a real database — mirrors
// fakeStore's pattern in router_test.go.
type fakeChatHistoryStore struct {
	sessions map[string]chat.ChatSession
	messages map[string][]chat.ChatMessage
	nextID   int
	failWith error
}

func newFakeChatHistoryStore() *fakeChatHistoryStore {
	return &fakeChatHistoryStore{sessions: map[string]chat.ChatSession{}, messages: map[string][]chat.ChatMessage{}}
}

var _ chat.Store = (*fakeChatHistoryStore)(nil)

func (s *fakeChatHistoryStore) newID() string {
	s.nextID++
	return "sess-" + string(rune('a'+s.nextID))
}

func (s *fakeChatHistoryStore) ListSessions(context.Context) ([]chat.ChatSession, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	sessions := make([]chat.ChatSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt) })
	return sessions, nil
}

func (s *fakeChatHistoryStore) CreateSession(context.Context) (chat.ChatSession, error) {
	if s.failWith != nil {
		return chat.ChatSession{}, s.failWith
	}
	now := time.Now()
	sess := chat.ChatSession{ID: s.newID(), Title: chat.DefaultTitle, CreatedAt: now, UpdatedAt: now}
	s.sessions[sess.ID] = sess
	return sess, nil
}

func (s *fakeChatHistoryStore) RenameSession(_ context.Context, id, title string) (chat.ChatSession, error) {
	sess, ok := s.sessions[id]
	if !ok {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	sess.Title = title
	sess.UpdatedAt = time.Now()
	s.sessions[id] = sess
	return sess, nil
}

func (s *fakeChatHistoryStore) DeleteSession(_ context.Context, id string) error {
	if _, ok := s.sessions[id]; !ok {
		return chat.ErrNotFound
	}
	delete(s.sessions, id)
	delete(s.messages, id)
	return nil
}

func (s *fakeChatHistoryStore) ListMessages(_ context.Context, sessionID string) ([]chat.ChatMessage, error) {
	if _, ok := s.sessions[sessionID]; !ok {
		return nil, chat.ErrNotFound
	}
	return s.messages[sessionID], nil
}

func (s *fakeChatHistoryStore) AppendMessage(_ context.Context, sessionID, role, content, reasoning string) (chat.ChatMessage, error) {
	sess, ok := s.sessions[sessionID]
	if !ok {
		return chat.ChatMessage{}, chat.ErrNotFound
	}
	if err := chat.ValidateRole(role); err != nil {
		return chat.ChatMessage{}, err
	}
	now := time.Now()
	msg := chat.ChatMessage{ID: s.newID(), SessionID: sessionID, Role: role, Content: content, Reasoning: reasoning, CreatedAt: now}
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

func newChatHistoryTestRouter(store *fakeChatHistoryStore) *gin.Engine {
	return NewRouter(Deps{Store: newFakeStore(), Embedder: fakeEmbedder{}, Hub: NewHub(), ChatHistory: store})
}

func TestChatSessions_NotConfiguredReturns503(t *testing.T) {
	router := newTestRouter(newFakeStore(), fakeEmbedder{}) // ChatHistory left nil
	w := doRequest(t, router, http.MethodGet, "/chat/sessions", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChatSessions_CreateThenList(t *testing.T) {
	store := newFakeChatHistoryStore()
	router := newChatHistoryTestRouter(store)

	w := doRequest(t, router, http.MethodPost, "/chat/sessions", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created chat.ChatSession
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.ID == "" || created.Title != chat.DefaultTitle {
		t.Fatalf("unexpected created session: %+v", created)
	}

	w = doRequest(t, router, http.MethodGet, "/chat/sessions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Sessions []chat.ChatSession `json:"sessions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Sessions) != 1 || out.Sessions[0].ID != created.ID {
		t.Fatalf("unexpected sessions list: %+v", out.Sessions)
	}
}

func TestChatSessions_RenameSession(t *testing.T) {
	store := newFakeChatHistoryStore()
	router := newChatHistoryTestRouter(store)
	sess, _ := store.CreateSession(context.Background())

	w := doRequest(t, router, http.MethodPatch, "/chat/sessions/"+sess.ID, map[string]string{"title": "Renamed"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated chat.ChatSession
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.Title != "Renamed" {
		t.Fatalf("expected renamed title, got %q", updated.Title)
	}
}

func TestChatSessions_RenameMissingReturns404(t *testing.T) {
	store := newFakeChatHistoryStore()
	router := newChatHistoryTestRouter(store)

	w := doRequest(t, router, http.MethodPatch, "/chat/sessions/missing", map[string]string{"title": "x"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChatSessions_RenameMissingTitleRejected(t *testing.T) {
	store := newFakeChatHistoryStore()
	router := newChatHistoryTestRouter(store)
	sess, _ := store.CreateSession(context.Background())

	w := doRequest(t, router, http.MethodPatch, "/chat/sessions/"+sess.ID, map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChatSessions_DeleteSession(t *testing.T) {
	store := newFakeChatHistoryStore()
	router := newChatHistoryTestRouter(store)
	sess, _ := store.CreateSession(context.Background())

	w := doRequest(t, router, http.MethodDelete, "/chat/sessions/"+sess.ID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, router, http.MethodDelete, "/chat/sessions/"+sess.ID, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on second delete, got %d", w.Code)
	}
}

func TestChatMessages_AppendThenList(t *testing.T) {
	store := newFakeChatHistoryStore()
	router := newChatHistoryTestRouter(store)
	sess, _ := store.CreateSession(context.Background())

	w := doRequest(t, router, http.MethodPost, "/chat/sessions/"+sess.ID+"/messages", map[string]string{
		"role": "user", "content": "hello there",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created chat.ChatMessage
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Content != "hello there" || created.Role != "user" {
		t.Fatalf("unexpected created message: %+v", created)
	}

	w = doRequest(t, router, http.MethodGet, "/chat/sessions/"+sess.ID+"/messages", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Messages []chat.ChatMessage `json:"messages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Messages) != 1 || out.Messages[0].Content != "hello there" {
		t.Fatalf("unexpected messages list: %+v", out.Messages)
	}

	// The session's title should now be auto-set from this first user message.
	sessions, err := store.ListSessions(context.Background())
	if err != nil || len(sessions) != 1 {
		t.Fatalf("list sessions: %v, %+v", err, sessions)
	}
	if sessions[0].Title != "hello there" {
		t.Fatalf("expected auto-titled session, got %q", sessions[0].Title)
	}
}

func TestChatMessages_AppendMissingSessionReturns404(t *testing.T) {
	store := newFakeChatHistoryStore()
	router := newChatHistoryTestRouter(store)

	w := doRequest(t, router, http.MethodPost, "/chat/sessions/missing/messages", map[string]string{
		"role": "user", "content": "hi",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChatMessages_AppendMissingContentRejected(t *testing.T) {
	store := newFakeChatHistoryStore()
	router := newChatHistoryTestRouter(store)
	sess, _ := store.CreateSession(context.Background())

	w := doRequest(t, router, http.MethodPost, "/chat/sessions/"+sess.ID+"/messages", map[string]string{"role": "user"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChatSessions_ListErrorSanitized(t *testing.T) {
	store := newFakeChatHistoryStore()
	store.failWith = errFakeDriver
	router := newChatHistoryTestRouter(store)

	w := doRequest(t, router, http.MethodGet, "/chat/sessions", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("driver error detail")) {
		t.Fatalf("inner error leaked to response body: %s", w.Body.String())
	}
}
