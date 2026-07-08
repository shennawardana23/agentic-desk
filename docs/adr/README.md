# Architecture Decision Records

Numbered, immutable once accepted. If a decision is later reversed, add a new ADR and mark the old one "Superseded by ADR-00XX" — never edit an accepted ADR's Decision section after the fact.

Format and numbering convention follow the standard ADR practice described at [adr.github.io](https://adr.github.io/) and the `adr-tools` conventions at [github.com/npryce/adr-tools](https://github.com/npryce/adr-tools) (external references — cited as the convention this project follows, not fetched/verified live in this session).

## Index

| ADR | Title | Status |
|---|---|---|
| [0001](0001-go-postgres-pgvector.md) | Go + PostgreSQL + pgvector for the Second Brain | Accepted |
| [0002](0002-wails-v2-not-v3.md) | Wails v2, not v3 | Accepted |
| [0003](0003-genkit-go-adk-go.md) | Genkit Go 1.0 GA + ADK Go 2.0 GA | Accepted |
| [0004](0004-mcp-genkit-not-third-party-sdk.md) | Genkit's own `plugins/mcp`, not a third-party MCP SDK | Accepted |
| [0005](0005-headless-core-process-architecture.md) | Headless core process + thin GUI shell | Accepted |
| [0006](0006-deterministic-profile-importer.md) | Deterministic parser for profile import, not an LLM | Accepted |
| [0007](0007-bounded-agent-loop-honest-rlhf.md) | Bounded Agent Loop + honest (non-literal) RLHF framing | Accepted |

All seven derive directly from the [sub-project 1 design doc](../superpowers/specs/2026-07-06-foundation-second-brain-design.md), Section 2 — this ADR set is that table reformatted per-decision, not new rationale.
