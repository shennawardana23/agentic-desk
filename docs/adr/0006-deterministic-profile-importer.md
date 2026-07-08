# ADR-0006: Deterministic parser for profile import, not an LLM

Status: Accepted — 2026-07-06

## Context

The Second Brain's coding-principles profile needs to be seeded from the user's existing `~/.claude/CLAUDE.md`, `RULES.md`, and `PRINCIPLES.md` files rather than authored from scratch. An LLM-based summarizer was one option; the user explicitly required a zero-trust, non-hallucinating approach instead.

## Decision

Seed the profile using a deterministic, regex/heading-based markdown parser (`internal/importer`) that extracts rules keyed by `(source_file, heading, line_range, content_hash)`. No LLM is used anywhere in this import path.

## Consequences

- Every imported rule is traceable to an exact source location — auditable and directly testable with fixture-based table-driven tests, unlike an LLM summarizer's output.
- If the user edits an imported rule inside the app (`overridden=true`), a later source change is never allowed to silently overwrite it; instead a separate "suggested update" row is created for manual review — a one-way sync, not a two-way merge.
- This constrains the importer to what a structural parser can actually extract (headings, priority tags, bullet content) — it cannot infer intent the source text doesn't state explicitly, which is the intended trade-off, not a limitation to work around.
