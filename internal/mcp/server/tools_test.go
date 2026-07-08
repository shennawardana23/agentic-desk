package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/genkit"

	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// TestToolErr_DoesNotLeakInnerError pins the fix for the MCP error
// message leakage finding in SESSION_HANDOFF.md: Genkit's MCP server
// plugin forwards a tool's error text verbatim to the external caller
// (plugins/mcp/server.go's mcp.NewToolResultError(err.Error())), so
// toolErr must never let driver/SQL text through.
func TestToolErr_DoesNotLeakInnerError(t *testing.T) {
	inner := errors.New(`pq: relation "profile_rule" does not exist`)
	got := toolErr("get_profile", inner).Error()
	if strings.Contains(got, "profile_rule") || strings.Contains(got, "pq:") {
		t.Fatalf("inner error leaked to caller: %q", got)
	}
}

// fakeStore implements secondbrain.Store with an in-memory map, enough
// to exercise every tool without a real database.
type fakeStore struct {
	rules    []secondbrain.ProfileRule
	contexts map[string]secondbrain.ProjectContext
	entries  []secondbrain.MemoryEntry
	nextID   int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{contexts: map[string]secondbrain.ProjectContext{}}
}

var _ secondbrain.Store = (*fakeStore)(nil)

func (s *fakeStore) UpsertProfileRule(context.Context, secondbrain.ProfileRule) (secondbrain.ProfileRule, error) {
	panic("not used by this test")
}
func (s *fakeStore) GetProfileRule(context.Context, string, string) (secondbrain.ProfileRule, error) {
	panic("not used by this test")
}
func (s *fakeStore) ListProfileRules(_ context.Context, limit, offset int) ([]secondbrain.ProfileRule, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(s.rules) {
		return nil, nil
	}
	end := offset + limit
	if end > len(s.rules) {
		end = len(s.rules)
	}
	return s.rules[offset:end], nil
}
func (s *fakeStore) SearchProfileRulesByVector(context.Context, []float32, int) ([]secondbrain.ProfileRule, error) {
	panic("not used by this test")
}
func (s *fakeStore) UpsertProjectContext(context.Context, secondbrain.ProjectContext) (secondbrain.ProjectContext, error) {
	panic("not used by this test")
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
func (s *fakeStore) CreateMemoryEntry(_ context.Context, entry secondbrain.MemoryEntry) (secondbrain.MemoryEntry, error) {
	s.nextID++
	entry.ID = s.nextID
	s.entries = append(s.entries, entry)
	return entry, nil
}
func (s *fakeStore) GetMemoryEntry(context.Context, int64) (secondbrain.MemoryEntry, error) {
	panic("not used by this test")
}
func (s *fakeStore) SearchMemoryEntriesByVector(_ context.Context, _ []float32, k int) ([]secondbrain.MemoryEntry, error) {
	entries := s.entries
	if len(entries) > k {
		entries = entries[:k]
	}
	return entries, nil
}
func (s *fakeStore) CreateFeedbackSignal(context.Context, secondbrain.FeedbackSignal) (secondbrain.FeedbackSignal, error) {
	panic("not used by this test")
}
func (s *fakeStore) GetFeedbackSignal(context.Context, int64) (secondbrain.FeedbackSignal, error) {
	panic("not used by this test")
}

// fakeEmbedder returns a fixed-length vector regardless of input, so
// search_memory can be tested without a live embedding API call.
type fakeEmbedder struct{}

func (fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	return make([]float32, secondbrain.EmbeddingDim), nil
}

func newTestTools(store *fakeStore) Tools {
	g := genkit.Init(context.Background())
	return DefineTools(g, Deps{Store: store, Embedder: fakeEmbedder{}})
}

// runTool calls a tool exactly like an external MCP client would — JSON
// in, JSON out — then decodes the result into Out for assertions.
func runTool[Out any](t *testing.T, tool interface {
	RunRaw(context.Context, any) (any, error)
}, input any) Out {
	t.Helper()
	var zero Out

	raw, err := tool.RunRaw(context.Background(), input)
	if err != nil {
		t.Fatalf("RunRaw: %v", err)
		return zero
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal tool output: %v", err)
		return zero
	}
	var out Out
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal tool output: %v", err)
		return zero
	}
	return out
}

func TestGetProfile_ReturnsAllRules(t *testing.T) {
	store := newFakeStore()
	store.rules = []secondbrain.ProfileRule{
		{SourceFile: "CLAUDE.md", Heading: "A", Content: "a"},
		{SourceFile: "RULES.md", Heading: "B", Content: "b", Overridden: true},
	}
	tools := newTestTools(store)

	out := runTool[getProfileOutput](t, tools.GetProfile, map[string]any{})
	if len(out.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d: %+v", len(out.Rules), out.Rules)
	}
	if !out.Rules[1].Overridden {
		t.Error("expected second rule to carry Overridden=true through")
	}
}

func TestGetProjectContext_Found(t *testing.T) {
	store := newFakeStore()
	store.contexts["/tmp/proj"] = secondbrain.ProjectContext{Summary: "a project"}
	tools := newTestTools(store)

	out := runTool[getProjectContextOutput](t, tools.GetProjectContext, map[string]any{"projectPath": "/tmp/proj"})
	if !out.Found || out.Summary != "a project" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestGetProjectContext_NotFound(t *testing.T) {
	store := newFakeStore()
	tools := newTestTools(store)

	out := runTool[getProjectContextOutput](t, tools.GetProjectContext, map[string]any{"projectPath": "/does/not/exist"})
	if out.Found {
		t.Fatalf("expected Found=false, got %+v", out)
	}
}

func TestLogSession_AssignsID(t *testing.T) {
	store := newFakeStore()
	tools := newTestTools(store)

	out := runTool[logSessionOutput](t, tools.LogSession, map[string]any{
		"sessionId": "s1", "role": secondbrain.RoleUser, "content": "hello",
	})
	if out.ID == 0 {
		t.Fatal("expected an assigned ID")
	}
	if len(store.entries) != 1 || store.entries[0].Content != "hello" {
		t.Fatalf("expected the entry to be stored, got %+v", store.entries)
	}
}

func TestSearchMemory_EmbedsQueryThenSearches(t *testing.T) {
	store := newFakeStore()
	store.entries = []secondbrain.MemoryEntry{
		{ID: 1, SessionID: "s1", Role: secondbrain.RoleUser, Content: "hello"},
		{ID: 2, SessionID: "s1", Role: secondbrain.RoleAgent, Content: "hi there"},
	}
	tools := newTestTools(store)

	out := runTool[searchMemoryOutput](t, tools.SearchMemory, map[string]any{"query": "greeting", "k": 1})
	if len(out.Entries) != 1 {
		t.Fatalf("expected k=1 to limit results to 1, got %d", len(out.Entries))
	}
}

func TestSearchMemory_DefaultsKWhenZero(t *testing.T) {
	store := newFakeStore()
	store.entries = []secondbrain.MemoryEntry{
		{ID: 1, SessionID: "s1", Role: secondbrain.RoleUser, Content: "a"},
	}
	tools := newTestTools(store)

	out := runTool[searchMemoryOutput](t, tools.SearchMemory, map[string]any{"query": "x"})
	if len(out.Entries) != 1 {
		t.Fatalf("expected the default k to still return the one entry, got %d", len(out.Entries))
	}
}
