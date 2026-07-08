package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// Status describes how a freshly parsed Rule compares to what's already
// stored for the same (SourceFile, Heading) key.
type Status string

const (
	// StatusNew has no existing profile_rule row yet; Apply writes it.
	StatusNew Status = "new"
	// StatusChanged differs in content from the stored row, which
	// hasn't been manually overridden; Apply writes it.
	StatusChanged Status = "changed"
	// StatusUnchanged matches the stored row's content hash exactly;
	// Apply leaves it alone.
	StatusUnchanged Status = "unchanged"
	// StatusOverridden means a human has hand-edited this rule in the
	// Second Brain (ProfileRule.Overridden); Apply never overwrites
	// it, only surfaces the freshly parsed content as a suggestion.
	StatusOverridden Status = "overridden"
)

// DiffResult pairs a parsed Rule with how it compares to the stored
// profile_rule row for the same key.
type DiffResult struct {
	Rule   Rule
	Status Status
}

func diffOne(ctx context.Context, store secondbrain.Store, rule Rule) (DiffResult, error) {
	existing, err := store.GetProfileRule(ctx, rule.SourceFile, rule.Heading)
	if errors.Is(err, secondbrain.ErrNotFound) {
		return DiffResult{Rule: rule, Status: StatusNew}, nil
	}
	if err != nil {
		return DiffResult{}, fmt.Errorf("get existing rule %s/%s: %w", rule.SourceFile, rule.Heading, err)
	}
	if existing.Overridden {
		return DiffResult{Rule: rule, Status: StatusOverridden}, nil
	}
	if existing.ContentHash == rule.ContentHash {
		return DiffResult{Rule: rule, Status: StatusUnchanged}, nil
	}
	return DiffResult{Rule: rule, Status: StatusChanged}, nil
}

// Apply diffs every parsed rule against store and writes back only the
// ones that are new or changed. Unchanged rules are left alone;
// overridden rules are never written — the user's own edit wins.
func Apply(ctx context.Context, store secondbrain.Store, rules []Rule) ([]DiffResult, error) {
	results := make([]DiffResult, 0, len(rules))
	for _, rule := range rules {
		result, err := diffOne(ctx, store, rule)
		if err != nil {
			return nil, err
		}
		if result.Status == StatusNew || result.Status == StatusChanged {
			if _, err := store.UpsertProfileRule(ctx, secondbrain.ProfileRule{
				SourceFile:  rule.SourceFile,
				Heading:     rule.Heading,
				LineStart:   rule.LineStart,
				LineEnd:     rule.LineEnd,
				ContentHash: rule.ContentHash,
				Content:     rule.Content,
			}); err != nil {
				return nil, fmt.Errorf("upsert rule %s/%s: %w", rule.SourceFile, rule.Heading, err)
			}
		}
		results = append(results, result)
	}
	return results, nil
}

// DefaultPaths returns the standard profile source files under the
// user's home directory. Callers needing different paths (tests, a
// non-default install) build their own slice for ImportPaths instead.
func DefaultPaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".claude", "RULES.md"),
		filepath.Join(home, ".claude", "PRINCIPLES.md"),
	}, nil
}

// ImportPaths parses every existing file in paths — a missing file is
// skipped, not an error, since paths are configurable and not all of
// them exist on every machine — and applies the diff against store.
func ImportPaths(ctx context.Context, store secondbrain.Store, paths []string) ([]DiffResult, error) {
	var all []Rule
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		rules, err := Parse(path, content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		all = append(all, rules...)
	}
	return Apply(ctx, store, all)
}
