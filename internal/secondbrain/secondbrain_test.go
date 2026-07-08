package secondbrain_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// memStore is a minimal in-memory secondbrain.Store used to test domain
// validation without a real database. Every Upsert/Create call runs the
// same Validate() the postgres adapter runs before it touches SQL, so a
// test against this fake exercises the same domain rule.
type memStore struct {
	profileRules    map[string]secondbrain.ProfileRule
	projectContexts map[string]secondbrain.ProjectContext
	memoryEntries   map[int64]secondbrain.MemoryEntry
	feedbackSignals map[int64]secondbrain.FeedbackSignal
	nextID          int64
}

func newMemStore() *memStore {
	return &memStore{
		profileRules:    map[string]secondbrain.ProfileRule{},
		projectContexts: map[string]secondbrain.ProjectContext{},
		memoryEntries:   map[int64]secondbrain.MemoryEntry{},
		feedbackSignals: map[int64]secondbrain.FeedbackSignal{},
	}
}

func (s *memStore) nextIDValue() int64 {
	s.nextID++
	return s.nextID
}

func profileRuleKey(sourceFile, heading string) string {
	return sourceFile + "\x00" + heading
}

var _ secondbrain.Store = (*memStore)(nil)

func (s *memStore) UpsertProfileRule(_ context.Context, rule secondbrain.ProfileRule) (secondbrain.ProfileRule, error) {
	if err := rule.Validate(); err != nil {
		return secondbrain.ProfileRule{}, err
	}
	key := profileRuleKey(rule.SourceFile, rule.Heading)
	if existing, ok := s.profileRules[key]; ok {
		rule.ID, rule.CreatedAt = existing.ID, existing.CreatedAt
	} else {
		rule.ID, rule.CreatedAt = s.nextIDValue(), time.Now()
	}
	rule.UpdatedAt = time.Now()
	s.profileRules[key] = rule
	return rule, nil
}

func (s *memStore) GetProfileRule(_ context.Context, sourceFile, heading string) (secondbrain.ProfileRule, error) {
	rule, ok := s.profileRules[profileRuleKey(sourceFile, heading)]
	if !ok {
		return secondbrain.ProfileRule{}, secondbrain.ErrNotFound
	}
	return rule, nil
}

func (s *memStore) ListProfileRules(_ context.Context, limit, offset int) ([]secondbrain.ProfileRule, error) {
	rules := make([]secondbrain.ProfileRule, 0, len(s.profileRules))
	for _, r := range s.profileRules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].SourceFile != rules[j].SourceFile {
			return rules[i].SourceFile < rules[j].SourceFile
		}
		return rules[i].Heading < rules[j].Heading
	})
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rules) {
		return nil, nil
	}
	end := offset + limit
	if end > len(rules) {
		end = len(rules)
	}
	return rules[offset:end], nil
}

func (s *memStore) SearchProfileRulesByVector(_ context.Context, _ []float32, k int) ([]secondbrain.ProfileRule, error) {
	var rules []secondbrain.ProfileRule
	for _, r := range s.profileRules {
		rules = append(rules, r)
		if len(rules) == k {
			break
		}
	}
	return rules, nil
}

func (s *memStore) UpsertProjectContext(_ context.Context, pc secondbrain.ProjectContext) (secondbrain.ProjectContext, error) {
	if err := pc.Validate(); err != nil {
		return secondbrain.ProjectContext{}, err
	}
	if existing, ok := s.projectContexts[pc.ProjectPath]; ok {
		pc.ID, pc.CreatedAt = existing.ID, existing.CreatedAt
	} else {
		pc.ID, pc.CreatedAt = s.nextIDValue(), time.Now()
	}
	pc.UpdatedAt = time.Now()
	s.projectContexts[pc.ProjectPath] = pc
	return pc, nil
}

func (s *memStore) GetProjectContext(_ context.Context, projectPath string) (secondbrain.ProjectContext, error) {
	pc, ok := s.projectContexts[projectPath]
	if !ok {
		return secondbrain.ProjectContext{}, secondbrain.ErrNotFound
	}
	return pc, nil
}

func (s *memStore) SearchProjectContextsByVector(_ context.Context, _ []float32, k int) ([]secondbrain.ProjectContext, error) {
	var contexts []secondbrain.ProjectContext
	for _, c := range s.projectContexts {
		contexts = append(contexts, c)
		if len(contexts) == k {
			break
		}
	}
	return contexts, nil
}

func (s *memStore) CreateMemoryEntry(_ context.Context, entry secondbrain.MemoryEntry) (secondbrain.MemoryEntry, error) {
	if err := entry.Validate(); err != nil {
		return secondbrain.MemoryEntry{}, err
	}
	entry.ID, entry.CreatedAt = s.nextIDValue(), time.Now()
	s.memoryEntries[entry.ID] = entry
	return entry, nil
}

func (s *memStore) GetMemoryEntry(_ context.Context, id int64) (secondbrain.MemoryEntry, error) {
	entry, ok := s.memoryEntries[id]
	if !ok {
		return secondbrain.MemoryEntry{}, secondbrain.ErrNotFound
	}
	return entry, nil
}

func (s *memStore) SearchMemoryEntriesByVector(_ context.Context, _ []float32, k int) ([]secondbrain.MemoryEntry, error) {
	var entries []secondbrain.MemoryEntry
	for _, e := range s.memoryEntries {
		entries = append(entries, e)
		if len(entries) == k {
			break
		}
	}
	return entries, nil
}

func (s *memStore) CreateFeedbackSignal(_ context.Context, signal secondbrain.FeedbackSignal) (secondbrain.FeedbackSignal, error) {
	if err := signal.Validate(); err != nil {
		return secondbrain.FeedbackSignal{}, err
	}
	signal.ID, signal.CreatedAt = s.nextIDValue(), time.Now()
	s.feedbackSignals[signal.ID] = signal
	return signal, nil
}

func (s *memStore) GetFeedbackSignal(_ context.Context, id int64) (secondbrain.FeedbackSignal, error) {
	signal, ok := s.feedbackSignals[id]
	if !ok {
		return secondbrain.FeedbackSignal{}, secondbrain.ErrNotFound
	}
	return signal, nil
}

func validEmbedding() []float32 {
	return make([]float32, secondbrain.EmbeddingDim)
}

func TestProfileRule_Validate(t *testing.T) {
	base := secondbrain.ProfileRule{
		SourceFile: "CLAUDE.md", Heading: "Rules", LineStart: 1, LineEnd: 3,
		ContentHash: "abc", Content: "do the thing",
	}

	tests := []struct {
		name    string
		mutate  func(r secondbrain.ProfileRule) secondbrain.ProfileRule
		wantErr bool
	}{
		{"valid", func(r secondbrain.ProfileRule) secondbrain.ProfileRule { return r }, false},
		{"valid with embedding", func(r secondbrain.ProfileRule) secondbrain.ProfileRule {
			r.Embedding = validEmbedding()
			return r
		}, false},
		{"missing source file", func(r secondbrain.ProfileRule) secondbrain.ProfileRule {
			r.SourceFile = ""
			return r
		}, true},
		{"missing heading", func(r secondbrain.ProfileRule) secondbrain.ProfileRule {
			r.Heading = ""
			return r
		}, true},
		{"missing content hash", func(r secondbrain.ProfileRule) secondbrain.ProfileRule {
			r.ContentHash = ""
			return r
		}, true},
		{"missing content", func(r secondbrain.ProfileRule) secondbrain.ProfileRule {
			r.Content = ""
			return r
		}, true},
		{"inverted line range", func(r secondbrain.ProfileRule) secondbrain.ProfileRule {
			r.LineStart = 5
			r.LineEnd = 1
			return r
		}, true},
		{"negative line start", func(r secondbrain.ProfileRule) secondbrain.ProfileRule {
			r.LineStart = -1
			return r
		}, true},
		{"wrong embedding dimension", func(r secondbrain.ProfileRule) secondbrain.ProfileRule {
			r.Embedding = []float32{1, 2, 3}
			return r
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(base).Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryEntry_Validate(t *testing.T) {
	base := secondbrain.MemoryEntry{SessionID: "s1", Role: secondbrain.RoleUser, Content: "hi"}

	tests := []struct {
		name    string
		mutate  func(m secondbrain.MemoryEntry) secondbrain.MemoryEntry
		wantErr bool
	}{
		{"valid user", func(m secondbrain.MemoryEntry) secondbrain.MemoryEntry { return m }, false},
		{"valid agent", func(m secondbrain.MemoryEntry) secondbrain.MemoryEntry {
			m.Role = secondbrain.RoleAgent
			return m
		}, false},
		{"missing session id", func(m secondbrain.MemoryEntry) secondbrain.MemoryEntry {
			m.SessionID = ""
			return m
		}, true},
		{"invalid role", func(m secondbrain.MemoryEntry) secondbrain.MemoryEntry {
			m.Role = "admin"
			return m
		}, true},
		{"missing content", func(m secondbrain.MemoryEntry) secondbrain.MemoryEntry {
			m.Content = ""
			return m
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(base).Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFeedbackSignal_Validate(t *testing.T) {
	for _, decision := range []string{secondbrain.DecisionApprove, secondbrain.DecisionCorrect, secondbrain.DecisionReject} {
		if err := (secondbrain.FeedbackSignal{Decision: decision}).Validate(); err != nil {
			t.Errorf("decision %q: unexpected error: %v", decision, err)
		}
	}
	if err := (secondbrain.FeedbackSignal{Decision: "maybe"}).Validate(); err == nil {
		t.Error("expected error for invalid decision")
	}
}

func TestProjectContext_Validate(t *testing.T) {
	if err := (secondbrain.ProjectContext{ProjectPath: "/tmp/x"}).Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := (secondbrain.ProjectContext{}).Validate(); err == nil {
		t.Error("expected error for missing project path")
	}
}

func TestMemStore_UpsertProfileRule_RejectsInvalid(t *testing.T) {
	store := newMemStore()
	_, err := store.UpsertProfileRule(context.Background(), secondbrain.ProfileRule{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestMemStore_UpsertProfileRule_UpdatesInPlace(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	rule := secondbrain.ProfileRule{
		SourceFile: "RULES.md", Heading: "Git", LineStart: 1, LineEnd: 2,
		ContentHash: "h1", Content: "commit often",
	}

	first, err := store.UpsertProfileRule(ctx, rule)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	rule.ContentHash = "h2"
	rule.Content = "commit often, with meaning"
	second, err := store.UpsertProfileRule(ctx, rule)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("expected same ID on upsert of same key, got %d and %d", first.ID, second.ID)
	}
	if len(store.profileRules) != 1 {
		t.Fatalf("expected 1 stored rule, got %d", len(store.profileRules))
	}

	got, err := store.GetProfileRule(ctx, "RULES.md", "Git")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ContentHash != "h2" {
		t.Fatalf("expected updated content hash, got %q", got.ContentHash)
	}
}

func TestMemStore_Get_NotFound(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()

	if _, err := store.GetProfileRule(ctx, "missing", "missing"); !errors.Is(err, secondbrain.ErrNotFound) {
		t.Errorf("GetProfileRule: expected ErrNotFound, got %v", err)
	}
	if _, err := store.GetProjectContext(ctx, "missing"); !errors.Is(err, secondbrain.ErrNotFound) {
		t.Errorf("GetProjectContext: expected ErrNotFound, got %v", err)
	}
	if _, err := store.GetMemoryEntry(ctx, 999); !errors.Is(err, secondbrain.ErrNotFound) {
		t.Errorf("GetMemoryEntry: expected ErrNotFound, got %v", err)
	}
	if _, err := store.GetFeedbackSignal(ctx, 999); !errors.Is(err, secondbrain.ErrNotFound) {
		t.Errorf("GetFeedbackSignal: expected ErrNotFound, got %v", err)
	}
}

func TestMemStore_MemoryEntryAndFeedbackSignal(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()

	entry, err := store.CreateMemoryEntry(ctx, secondbrain.MemoryEntry{
		SessionID: "s1", Role: secondbrain.RoleUser, Content: "hello",
	})
	if err != nil {
		t.Fatalf("create memory entry: %v", err)
	}
	if entry.ID == 0 {
		t.Fatal("expected assigned ID")
	}

	signal, err := store.CreateFeedbackSignal(ctx, secondbrain.FeedbackSignal{
		MemoryEntryID: &entry.ID, Decision: secondbrain.DecisionApprove, Note: "good",
	})
	if err != nil {
		t.Fatalf("create feedback signal: %v", err)
	}
	if signal.MemoryEntryID == nil || *signal.MemoryEntryID != entry.ID {
		t.Fatalf("expected feedback signal linked to memory entry %d, got %+v", entry.ID, signal.MemoryEntryID)
	}
}

func TestMemStore_ListProfileRules(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()

	if rules, err := store.ListProfileRules(ctx, 0, 0); err != nil || len(rules) != 0 {
		t.Fatalf("expected an empty list before any rule exists, got %+v, err=%v", rules, err)
	}

	if _, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
		SourceFile: "CLAUDE.md", Heading: "A", LineStart: 1, LineEnd: 2, ContentHash: "h1", Content: "a",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
		SourceFile: "RULES.md", Heading: "B", LineStart: 1, LineEnd: 2, ContentHash: "h2", Content: "b",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	rules, err := store.ListProfileRules(ctx, 0, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}
