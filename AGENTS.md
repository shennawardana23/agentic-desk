# AGENTS.md — entry point for coding agents

Read this before making changes in this repo. It's the map of where things are, what conventions to follow, and what's actually runnable right now.

## Current build/test status

**Nothing is runnable yet.** There is no `go.mod`, no `cmd/`, no code. The first task is Phase 0 in [`PLAN.md`](PLAN.md). Do not write commands here that don't exist — this section must be updated the moment Phase 0 lands with the real `go build`/`go test` invocations.

Until then, per-phase verification steps are defined in [`PLAN.md`](PLAN.md) — follow those, in order, one phase at a time. Do not skip ahead to a later phase's files.

## Package layout convention (Separation of Concerns)

Target layout, defined in the [sub-project 1 design doc](docs/superpowers/specs/2026-07-06-foundation-second-brain-design.md) Section 5:

```
internal/secondbrain/            domain types + Store interface — ZERO framework deps (no pgx, no Genkit types)
internal/secondbrain/postgres/   the only package allowed to import pgx/database-sql
internal/agentloop/              Plan/Act/Observe/Critique loop, bounded self-correction, HITL escalation
internal/eval/                   Evaluator interface + default deterministic implementation
internal/importer/               deterministic markdown parser (no LLM calls, ever, in this package)
internal/embedding/              embedder wrapper + bounded worker pool
internal/genkit/                 Genkit app init, Dotprompt loader, flow registration
internal/mcp/server/, internal/mcp/client/   Genkit plugins/mcp usage
internal/tools/github/           native google/go-github SDK, wrapped as a tool
internal/api/                    Gin + Gorilla WS
internal/config/                 env loading, fail-fast
```

Rule of thumb enforced across this layout: **domain packages (`secondbrain`, `agentloop`, `eval`) never import a specific storage or LLM-provider library directly.** They depend on interfaces; adapters live in their own sub-package. If you find yourself importing `pgx` outside `internal/secondbrain/postgres`, or importing a provider SDK outside `internal/genkit`/`internal/embedding`, stop — that's a SoC violation, not a shortcut.

## Engineering conventions

- SOLID, especially Dependency Inversion (interfaces owned by the domain package, implemented by adapters) and Open/Closed (the `Evaluator` interface exists so sub-project 7's LLM-as-judge can replace the default implementation with zero call-site changes).
- YAGNI: don't build sub-project N+1's functionality while implementing sub-project N. The design doc's "Non-goals" section for each sub-project is binding, not a suggestion.
- Goroutines: every pool must be bounded (fixed worker count), every pool must shut down via context-cancel + `WaitGroup.Wait(timeout)`, and every pool needs a goroutine-leak test. See `internal/embedding`'s plan (Phase 4) as the template.
- Zero-trust on external facts: before asserting an SDK method signature, model identifier, or library version in code or docs, verify it against the live source (package docs, source code) — don't extrapolate from a blog post or an older doc example. The design doc's "Open items requiring verification" list exists for exactly this reason — resolve each one with a real check before writing the code that depends on it, and update the design doc with what you found.

## Documentation obligation

**Every implementation change updates documentation in the same change** — this is a standing project rule, not optional. If you land Phase 0, update this file's build/test section, update `SESSION_HANDOFF.md`, and add a `docs/reviews/` entry if the change had a review. Documentation drift is treated as a bug.

## Where to look next

- [`SYSTEM.md`](SYSTEM.md) — architecture diagrams
- [`MEMORY.md`](MEMORY.md) — Second Brain data model
- [`SKILL.md`](SKILL.md) — planned agent-skill model (not yet implemented)
- [`SESSION_HANDOFF.md`](SESSION_HANDOFF.md) — what the last session left off at, and exactly what to do next
- [`docs/adr/`](docs/adr/) — why each major decision was made
