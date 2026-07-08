package importer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/importer"
	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// fakeStore implements secondbrain.Store just enough for importer
// tests: ProfileRule methods are real (backed by a map), everything
// else importer never calls and is left as an unreachable stub.
type fakeStore struct {
	rules map[string]secondbrain.ProfileRule
}

func newFakeStore() *fakeStore {
	return &fakeStore{rules: map[string]secondbrain.ProfileRule{}}
}

func key(sourceFile, heading string) string { return sourceFile + "\x00" + heading }

var _ secondbrain.Store = (*fakeStore)(nil)

func (s *fakeStore) UpsertProfileRule(_ context.Context, rule secondbrain.ProfileRule) (secondbrain.ProfileRule, error) {
	if err := rule.Validate(); err != nil {
		return secondbrain.ProfileRule{}, err
	}
	s.rules[key(rule.SourceFile, rule.Heading)] = rule
	return rule, nil
}

func (s *fakeStore) GetProfileRule(_ context.Context, sourceFile, heading string) (secondbrain.ProfileRule, error) {
	rule, ok := s.rules[key(sourceFile, heading)]
	if !ok {
		return secondbrain.ProfileRule{}, secondbrain.ErrNotFound
	}
	return rule, nil
}

func (s *fakeStore) ListProfileRules(context.Context, int, int) ([]secondbrain.ProfileRule, error) {
	panic("not used by importer")
}

func (s *fakeStore) SearchProfileRulesByVector(context.Context, []float32, int) ([]secondbrain.ProfileRule, error) {
	panic("not used by importer")
}
func (s *fakeStore) UpsertProjectContext(context.Context, secondbrain.ProjectContext) (secondbrain.ProjectContext, error) {
	panic("not used by importer")
}
func (s *fakeStore) GetProjectContext(context.Context, string) (secondbrain.ProjectContext, error) {
	panic("not used by importer")
}
func (s *fakeStore) SearchProjectContextsByVector(context.Context, []float32, int) ([]secondbrain.ProjectContext, error) {
	panic("not used by importer")
}
func (s *fakeStore) CreateMemoryEntry(context.Context, secondbrain.MemoryEntry) (secondbrain.MemoryEntry, error) {
	panic("not used by importer")
}
func (s *fakeStore) GetMemoryEntry(context.Context, int64) (secondbrain.MemoryEntry, error) {
	panic("not used by importer")
}
func (s *fakeStore) SearchMemoryEntriesByVector(context.Context, []float32, int) ([]secondbrain.MemoryEntry, error) {
	panic("not used by importer")
}
func (s *fakeStore) CreateFeedbackSignal(context.Context, secondbrain.FeedbackSignal) (secondbrain.FeedbackSignal, error) {
	panic("not used by importer")
}
func (s *fakeStore) GetFeedbackSignal(context.Context, int64) (secondbrain.FeedbackSignal, error) {
	panic("not used by importer")
}

func TestApply_NewRuleIsWritten(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	results, err := importer.Apply(ctx, store, []importer.Rule{
		{SourceFile: "CLAUDE.md", Heading: "H1", LineStart: 1, LineEnd: 2, ContentHash: "h1", Content: "body"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(results) != 1 || results[0].Status != importer.StatusNew {
		t.Fatalf("expected 1 StatusNew result, got %+v", results)
	}
	stored, err := store.GetProfileRule(ctx, "CLAUDE.md", "H1")
	if err != nil {
		t.Fatalf("expected rule to be written, got: %v", err)
	}
	if stored.Content != "body" {
		t.Fatalf("expected written content %q, got %q", "body", stored.Content)
	}
}

func TestApply_UnchangedRuleIsNotRewritten(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()
	rule := importer.Rule{SourceFile: "CLAUDE.md", Heading: "H1", LineStart: 1, LineEnd: 2, ContentHash: "h1", Content: "body"}

	if _, err := importer.Apply(ctx, store, []importer.Rule{rule}); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	results, err := importer.Apply(ctx, store, []importer.Rule{rule})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if len(results) != 1 || results[0].Status != importer.StatusUnchanged {
		t.Fatalf("expected StatusUnchanged, got %+v", results)
	}
}

func TestApply_ChangedRuleIsRewritten(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	if _, err := importer.Apply(ctx, store, []importer.Rule{
		{SourceFile: "CLAUDE.md", Heading: "H1", LineStart: 1, LineEnd: 2, ContentHash: "h1", Content: "body v1"},
	}); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	results, err := importer.Apply(ctx, store, []importer.Rule{
		{SourceFile: "CLAUDE.md", Heading: "H1", LineStart: 1, LineEnd: 3, ContentHash: "h2", Content: "body v2"},
	})
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if len(results) != 1 || results[0].Status != importer.StatusChanged {
		t.Fatalf("expected StatusChanged, got %+v", results)
	}
	stored, err := store.GetProfileRule(ctx, "CLAUDE.md", "H1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Content != "body v2" {
		t.Fatalf("expected rewritten content %q, got %q", "body v2", stored.Content)
	}
}

func TestApply_OverriddenRuleIsNeverRewritten(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	if _, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
		SourceFile: "CLAUDE.md", Heading: "H1", LineStart: 1, LineEnd: 2,
		ContentHash: "h1", Content: "user's own version", Overridden: true,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	results, err := importer.Apply(ctx, store, []importer.Rule{
		{SourceFile: "CLAUDE.md", Heading: "H1", LineStart: 1, LineEnd: 2, ContentHash: "h2", Content: "freshly parsed version"},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(results) != 1 || results[0].Status != importer.StatusOverridden {
		t.Fatalf("expected StatusOverridden, got %+v", results)
	}

	stored, err := store.GetProfileRule(ctx, "CLAUDE.md", "H1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Content != "user's own version" {
		t.Fatalf("overridden rule must not be rewritten, got content %q", stored.Content)
	}
}

func TestImportPaths_SkipsMissingFiles(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	results, err := importer.ImportPaths(ctx, store, []string{
		filepath.Join(t.TempDir(), "does-not-exist.md"),
	})
	if err != nil {
		t.Fatalf("ImportPaths: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results for a missing file, got %+v", results)
	}
}

func TestImportPaths_ParsesAndAppliesRealFiles(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "CLAUDE.md")
	path2 := filepath.Join(dir, "RULES.md")

	if err := os.WriteFile(path1, []byte("## Rule One\n\nDo the first thing.\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path1, err)
	}
	if err := os.WriteFile(path2, []byte("## Rule Two\n\nDo the second thing.\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path2, err)
	}

	store := newFakeStore()
	ctx := context.Background()

	results, err := importer.ImportPaths(ctx, store, []string{path1, path2})
	if err != nil {
		t.Fatalf("ImportPaths: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %+v", results)
	}
	for _, r := range results {
		if r.Status != importer.StatusNew {
			t.Errorf("expected StatusNew for %s/%s, got %s", r.Rule.SourceFile, r.Rule.Heading, r.Status)
		}
	}

	if _, err := store.GetProfileRule(ctx, path1, "Rule One"); err != nil {
		t.Errorf("expected Rule One to be stored: %v", err)
	}
	if _, err := store.GetProfileRule(ctx, path2, "Rule Two"); err != nil {
		t.Errorf("expected Rule Two to be stored: %v", err)
	}
}

func TestDefaultPaths(t *testing.T) {
	paths, err := importer.DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected 3 default paths, got %d: %v", len(paths), paths)
	}
}
