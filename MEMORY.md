# MEMORY.md — the Second Brain's memory model

Not to be confused with Claude Code's own session-memory system — this documents *this project's* domain model: what the Second Brain actually stores, in `internal/secondbrain` (target, not yet implemented — see [`PLAN.md`](PLAN.md) Phase 1-2).

## Entities

| Entity | Purpose | Key fields (target, `migrations/0001_init.sql`) |
|---|---|---|
| `Profile` (via `profile_rule` table) | Your coding-principles profile | `source_file`, `heading`, `line_range`, `content_hash`, `content`, `overridden bool`, `embedding vector(768)` |
| `ProjectContext` (`project_context`) | Per-project facts: stack, architecture decisions, active tasks | `project_id`, `content`, `embedding vector(768)` |
| `MemoryEntry` (`memory_entry`) | Semantic-searchable log of past agent interactions | `session_id`, `content`, `embedding vector(768)`, `created_at` |
| `FeedbackSignal` (`feedback_signal`) | Human corrections/ratings from Agent Loop HITL escalations or explicit approve/reject | `related_memory_entry_id`, `decision`, `reason`, `created_at` |

All four share the same storage shape: content + a 768-dim `gemini-embedding-2` vector, indexed in pgvector for cosine-similarity search (`<=>` operator).

## Import pipeline (profile seeding)

The `Profile` is seeded — not hand-typed from an empty form — by a **deterministic** parser (`internal/importer`), never an LLM summarizer. This is a hard requirement: every imported rule must be traceable to an exact source file, heading, and line range, with no possibility of a hallucinated mapping.

```d2
source_files: {
  label: "~/.claude/CLAUDE.md\nRULES.md\nPRINCIPLES.md"
  shape: page
}
importer: {
  label: "internal/importer\n(deterministic markdown parser)"
}
diff: {
  label: "diff by content_hash"
  shape: diamond
}
skip: "unchanged\n(skip)"
suggest: "changed but user\noverrode in-app\n(suggest only, don't clobber)"
insert: "new / changed\n(insert or update)"
store: {
  label: "secondbrain.Store"
  shape: cylinder
}
embed_pool: {
  label: "internal/embedding\n(bounded worker pool)"
}
gemini: "gemini-embedding-2 API"
pgvector: {
  label: "PostgreSQL + pgvector"
  shape: cylinder
}

source_files -> importer
importer -> diff
diff -> skip
diff -> suggest
diff -> insert
insert -> store
store -> embed_pool: "needs embedding"
embed_pool -> gemini: "batch embed"
gemini -> embed_pool: "768-dim vector"
embed_pool -> pgvector: "write vector"
```

Diff-by-hash outcomes:
- **Unchanged** (hash matches what's stored) → skipped, no write.
- **New or changed, not overridden by the user in-app** → inserted/updated, queued for embedding.
- **Changed at the source, but the user has edited this rule in-app** (`overridden=true`) → the source change is never allowed to silently clobber the user's edit. Instead a separate "suggested update" row is created for manual review.

## Embedding pipeline

`internal/embedding` wraps Genkit's `googlegenai.GoogleAIEmbedder` around `gemini-embedding-2`, requesting 768-dim output (chosen from the model's supported {768, 1536, 3072} range for pgvector index efficiency — see [`docs/adr/0001-go-postgres-pgvector.md`](docs/adr/0001-go-postgres-pgvector.md)). Embedding is batched through a **bounded** worker pool: a fixed number of goroutines drain a channel of "needs embedding" IDs, coordinated with `errgroup.Group` and a `context.Context`; shutdown cancels the context and then `WaitGroup.Wait()`s with a timeout — no goroutine is ever left running after the pool is told to stop. See [`PLAN.md`](PLAN.md) Phase 4 for the concrete leak-test requirement.

## Retrieval

All four entities are searched the same way: cosine similarity over their `embedding` column via pgvector's `<=>` operator, exposed to external agents through the MCP server's `secondbrain.search_memory(query, k)` tool (see [`SYSTEM.md`](SYSTEM.md)).
