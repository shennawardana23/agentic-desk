# Design index

Full design documents live under `docs/superpowers/specs/`, one file per sub-project, named `YYYY-MM-DD-<topic>-design.md`. Each design doc is the source of truth for its sub-project's architecture, decisions, and rationale — this file is only an index.

The platform is decomposed into ~9 ordered sub-projects. Specs are written just-in-time, one sub-project ahead of implementation — not all 9 upfront.

| # | Sub-project | Design doc | Status |
|---|---|---|---|
| 1 | Foundation + Second Brain core + Agent Loop primitive | [`docs/superpowers/specs/2026-07-06-foundation-second-brain-design.md`](docs/superpowers/specs/2026-07-06-foundation-second-brain-design.md) | Approved, not yet implemented (see [`PLAN.md`](PLAN.md)) |
| 2 | Single-agent chat pipeline (Genkit flow, Dotprompt, provider fallback) | not yet written | Not started |
| 3 | Multi-agent orchestration (ADK Go, A2A, MCP tool wiring, skills middleware) | not yet written | Not started |
| 4 | Voice STS agent | not yet written | Not started |
| 5 | Agentic flow visualization (Vue Flow, tracing) | not yet written | Not started |
| 6 | Tool-agents (search, fetch, image-gen, review) | not yet written | Not started |
| 7 | Tracing/Observability + Evals (LLM-as-judge) | not yet written | Not started |
| 8 | Docs system hardening | not yet written | Not started (this doc suite is the seed of it) |

When a new sub-project's design is approved, add its row here and link the new spec file — do not let this table drift out of sync with `docs/superpowers/specs/`.
