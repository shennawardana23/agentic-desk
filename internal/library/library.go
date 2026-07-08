// Package library serves the Skill Catalog and Prompt Catalog from the
// filesystem — skills/<name>/SKILL.md and prompts/<name>.prompt. Browse-only
// by explicit user decision (design doc 2026-07-07 §4/5): the repo/editor
// stays the write path, this package only reads.
package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned for names that don't exist in the catalog.
var ErrNotFound = errors.New("library: not found")

// Item is one catalog entry.
type Item struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// Library reads the two catalog roots. Zero-value dirs simply list empty.
type Library struct {
	SkillsDir  string
	PromptsDir string
}

// parseFrontmatter extracts top-level "name:"/"description:"/"category:"
// scalar lines from a leading --- YAML block. Deliberately not a YAML
// parser — the fields we surface are plain scalars in every real
// SKILL.md/.prompt file here, and a YAML dependency for that would be
// waste. category is optional; callers fall back to a display default
// ("General") when it's empty — that fallback stays a display concern,
// not this package's.
func parseFrontmatter(content string) (name, description, category string) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if v, ok := strings.CutPrefix(line, "name:"); ok {
			name = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "description:"); ok {
			description = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "category:"); ok {
			category = strings.TrimSpace(v)
		}
	}
	return name, description, category
}

// ListSkills returns every directory under SkillsDir that contains a
// SKILL.md, with its frontmatter description.
func (l Library) ListSkills() ([]Item, error) {
	entries, err := os.ReadDir(l.SkillsDir)
	if os.IsNotExist(err) {
		return []Item{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	items := []Item{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(l.SkillsDir, e.Name(), "SKILL.md"))
		if err != nil {
			continue // a dir without SKILL.md isn't a skill
		}
		_, desc, cat := parseFrontmatter(string(content))
		items = append(items, Item{Name: e.Name(), Description: desc, Category: cat})
	}
	return items, nil
}

// GetSkill returns skills/<name>/SKILL.md. The name is validated against
// the real directory listing rather than path-joined and cleaned — user
// input never reaches the filesystem as a path, so traversal is impossible
// by construction.
func (l Library) GetSkill(name string) (string, error) {
	items, err := l.ListSkills()
	if err != nil {
		return "", err
	}
	for _, it := range items {
		if it.Name == name {
			content, err := os.ReadFile(filepath.Join(l.SkillsDir, name, "SKILL.md"))
			if err != nil {
				return "", fmt.Errorf("get skill: %w", err)
			}
			return string(content), nil
		}
	}
	return "", ErrNotFound
}

// ListPrompts returns every *.prompt file under PromptsDir (dotprompt
// format, same leading --- frontmatter convention).
func (l Library) ListPrompts() ([]Item, error) {
	entries, err := os.ReadDir(l.PromptsDir)
	if os.IsNotExist(err) {
		return []Item{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}
	items := []Item{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".prompt") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".prompt")
		var desc, cat string
		if content, err := os.ReadFile(filepath.Join(l.PromptsDir, e.Name())); err == nil {
			_, desc, cat = parseFrontmatter(string(content))
		}
		items = append(items, Item{Name: name, Description: desc, Category: cat})
	}
	return items, nil
}

// GetPrompt returns prompts/<name>.prompt, with the same listing-based
// traversal guard as GetSkill.
func (l Library) GetPrompt(name string) (string, error) {
	items, err := l.ListPrompts()
	if err != nil {
		return "", err
	}
	for _, it := range items {
		if it.Name == name {
			content, err := os.ReadFile(filepath.Join(l.PromptsDir, name+".prompt"))
			if err != nil {
				return "", fmt.Errorf("get prompt: %w", err)
			}
			return string(content), nil
		}
	}
	return "", ErrNotFound
}
