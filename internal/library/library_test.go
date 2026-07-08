package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newFixtureLibrary(t *testing.T) Library {
	t.Helper()
	root := t.TempDir()
	skills := filepath.Join(root, "skills")
	prompts := filepath.Join(root, "prompts")
	writeFixture(t, filepath.Join(skills, "alpha-skill", "SKILL.md"),
		"---\nname: alpha-skill\ndescription: First fixture skill.\ncategory: Engineering\n---\n\n# Alpha\nBody here.\n")
	writeFixture(t, filepath.Join(skills, "no-manifest", "notes.txt"), "not a skill")
	writeFixture(t, filepath.Join(prompts, "greet.prompt"),
		"---\ndescription: Fixture prompt.\ncategory: Onboarding\n---\nHello {{name}}\n")
	return Library{SkillsDir: skills, PromptsDir: prompts}
}

func TestListSkills_ParsesFrontmatterAndSkipsNonSkills(t *testing.T) {
	l := newFixtureLibrary(t)
	items, err := l.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1: %+v", len(items), items)
	}
	if items[0].Name != "alpha-skill" || items[0].Description != "First fixture skill." || items[0].Category != "Engineering" {
		t.Fatalf("item = %+v", items[0])
	}
}

func TestGetSkill_ReturnsContentAndGuardsTraversal(t *testing.T) {
	l := newFixtureLibrary(t)
	content, err := l.GetSkill("alpha-skill")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "# Alpha") {
		t.Fatalf("content = %q", content)
	}
	for _, bad := range []string{"../secrets", "alpha-skill/../no-manifest", "..", "nope"} {
		if _, err := l.GetSkill(bad); !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetSkill(%q) err = %v, want ErrNotFound", bad, err)
		}
	}
}

func TestListAndGetPrompts(t *testing.T) {
	l := newFixtureLibrary(t)
	items, err := l.ListPrompts()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "greet" || items[0].Description != "Fixture prompt." || items[0].Category != "Onboarding" {
		t.Fatalf("items = %+v", items)
	}
	content, err := l.GetPrompt("greet")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "Hello {{name}}") {
		t.Fatalf("content = %q", content)
	}
	if _, err := l.GetPrompt("../placeholder"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("traversal err = %v, want ErrNotFound", err)
	}
}

func TestMissingDirsListEmpty(t *testing.T) {
	l := Library{SkillsDir: "/nonexistent/skills", PromptsDir: "/nonexistent/prompts"}
	if items, err := l.ListSkills(); err != nil || len(items) != 0 {
		t.Fatalf("skills = %v, %v", items, err)
	}
	if items, err := l.ListPrompts(); err != nil || len(items) != 0 {
		t.Fatalf("prompts = %v, %v", items, err)
	}
}
