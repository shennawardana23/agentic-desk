package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

var errFakeDriver = errors.New("driver error detail")

// fakeStore implements secondbrain.Store with an in-memory map, enough
// to exercise every route without a real database.
type fakeStore struct {
	rules      []secondbrain.ProfileRule
	contexts   map[string]secondbrain.ProjectContext
	entries    map[int64]secondbrain.MemoryEntry
	nextID     int64
	failWith   error
	searchHits []secondbrain.MemoryEntry
}

func newFakeStore() *fakeStore {
	return &fakeStore{contexts: map[string]secondbrain.ProjectContext{}, entries: map[int64]secondbrain.MemoryEntry{}}
}

var _ secondbrain.Store = (*fakeStore)(nil)

func (s *fakeStore) UpsertProfileRule(context.Context, secondbrain.ProfileRule) (secondbrain.ProfileRule, error) {
	panic("not used by this test")
}
func (s *fakeStore) GetProfileRule(_ context.Context, sourceFile, heading string) (secondbrain.ProfileRule, error) {
	for _, r := range s.rules {
		if r.SourceFile == sourceFile && r.Heading == heading {
			return r, nil
		}
	}
	return secondbrain.ProfileRule{}, secondbrain.ErrNotFound
}
func (s *fakeStore) ListProfileRules(context.Context, int, int) ([]secondbrain.ProfileRule, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	return s.rules, nil
}
func (s *fakeStore) SearchProfileRulesByVector(context.Context, []float32, int) ([]secondbrain.ProfileRule, error) {
	panic("not used by this test")
}
func (s *fakeStore) UpsertProjectContext(_ context.Context, pc secondbrain.ProjectContext) (secondbrain.ProjectContext, error) {
	s.contexts[pc.ProjectPath] = pc
	return pc, nil
}
func (s *fakeStore) GetProjectContext(_ context.Context, projectPath string) (secondbrain.ProjectContext, error) {
	pc, ok := s.contexts[projectPath]
	if !ok {
		return secondbrain.ProjectContext{}, secondbrain.ErrNotFound
	}
	return pc, nil
}
func (s *fakeStore) SearchProjectContextsByVector(context.Context, []float32, int) ([]secondbrain.ProjectContext, error) {
	panic("not used by this test")
}
func (s *fakeStore) CreateMemoryEntry(_ context.Context, e secondbrain.MemoryEntry) (secondbrain.MemoryEntry, error) {
	s.nextID++
	e.ID = s.nextID
	s.entries[e.ID] = e
	return e, nil
}
func (s *fakeStore) GetMemoryEntry(_ context.Context, id int64) (secondbrain.MemoryEntry, error) {
	e, ok := s.entries[id]
	if !ok {
		return secondbrain.MemoryEntry{}, secondbrain.ErrNotFound
	}
	return e, nil
}
func (s *fakeStore) SearchMemoryEntriesByVector(context.Context, []float32, int) ([]secondbrain.MemoryEntry, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	return s.searchHits, nil
}
func (s *fakeStore) CreateFeedbackSignal(context.Context, secondbrain.FeedbackSignal) (secondbrain.FeedbackSignal, error) {
	panic("not used by this test")
}
func (s *fakeStore) GetFeedbackSignal(context.Context, int64) (secondbrain.FeedbackSignal, error) {
	panic("not used by this test")
}

type fakeEmbedder struct {
	vector []float32
	err    error
}

func (f fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vector, nil
}

func newTestRouter(store *fakeStore, embedder fakeEmbedder) *gin.Engine {
	return NewRouter(Deps{Store: store, Embedder: embedder, Hub: NewHub()})
}

func doRequest(t *testing.T, router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestHealth_ReturnsOK(t *testing.T) {
	router := newTestRouter(newFakeStore(), fakeEmbedder{})
	w := doRequest(t, router, http.MethodGet, "/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListProfile_ReturnsRules(t *testing.T) {
	store := newFakeStore()
	store.rules = []secondbrain.ProfileRule{{SourceFile: "CLAUDE.md", Heading: "A", Content: "hello"}}
	router := newTestRouter(store, fakeEmbedder{})

	w := doRequest(t, router, http.MethodGet, "/profile", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Rules []profileRuleView `json:"rules"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Rules) != 1 || out.Rules[0].Content != "hello" {
		t.Fatalf("unexpected rules: %+v", out.Rules)
	}
}

func TestGetProfileRule_NotFound(t *testing.T) {
	router := newTestRouter(newFakeStore(), fakeEmbedder{})
	w := doRequest(t, router, http.MethodGet, "/profile/rule?sourceFile=x&heading=y", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestProjectContext_NotFoundThenUpsertThenFound(t *testing.T) {
	router := newTestRouter(newFakeStore(), fakeEmbedder{})

	w := doRequest(t, router, http.MethodGet, "/project-context?projectPath=/tmp/proj", nil)
	var notFound struct {
		Found bool `json:"found"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &notFound); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if notFound.Found {
		t.Fatal("expected found=false before any upsert")
	}

	w = doRequest(t, router, http.MethodPut, "/project-context", map[string]string{
		"projectPath": "/tmp/proj", "summary": "a Go service",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, router, http.MethodGet, "/project-context?projectPath=/tmp/proj", nil)
	var found struct {
		Found   bool   `json:"found"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &found); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !found.Found || found.Summary != "a Go service" {
		t.Fatalf("unexpected result after upsert: %+v", found)
	}
}

func TestCreateMemory_ThenGet(t *testing.T) {
	router := newTestRouter(newFakeStore(), fakeEmbedder{})

	w := doRequest(t, router, http.MethodPost, "/memory", map[string]string{
		"sessionId": "s1", "role": "user", "content": "hello",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created memoryEntryView
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected an assigned ID")
	}

	w = doRequest(t, router, http.MethodGet, "/memory/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateMemory_MissingFieldRejected(t *testing.T) {
	router := newTestRouter(newFakeStore(), fakeEmbedder{})
	w := doRequest(t, router, http.MethodPost, "/memory", map[string]string{"sessionId": "s1"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearchMemory_EmbedsThenSearches(t *testing.T) {
	store := newFakeStore()
	store.searchHits = []secondbrain.MemoryEntry{{ID: 1, SessionID: "s1", Role: "user", Content: "hi"}}
	router := newTestRouter(store, fakeEmbedder{vector: make([]float32, secondbrain.EmbeddingDim)})

	w := doRequest(t, router, http.MethodGet, "/memory/search?query=hello", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Entries []memoryEntryView `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out.Entries))
	}
}

// TestListProfile_ErrorSanitized mirrors the sanitization contract
// established elsewhere this session (internal/mcp/server,
// internal/tools/github) — a store error must not leak into the API
// response body.
func TestListProfile_ErrorSanitized(t *testing.T) {
	store := newFakeStore()
	store.failWith = errFakeDriver
	router := newTestRouter(store, fakeEmbedder{})

	w := doRequest(t, router, http.MethodGet, "/profile", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("driver error detail")) {
		t.Fatalf("inner error leaked to response body: %s", w.Body.String())
	}
}
