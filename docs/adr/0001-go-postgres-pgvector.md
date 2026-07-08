# ADR-0001: Go + PostgreSQL + pgvector for the Second Brain

Status: Accepted — 2026-07-06

## Context

Agentic Desk needs a persistence and vector-search layer for the Second Brain (profile, project context, memory, feedback signals). A local-first embedded option (SQLite + a vector extension) was offered as the lower-friction default for a single-user desktop tool, but PostgreSQL + pgvector was explicitly chosen instead.

## Decision

Use PostgreSQL with the pgvector extension as the Second Brain's storage engine, accessed only through `internal/secondbrain/postgres` (the domain package `internal/secondbrain` itself has zero storage-library dependencies).

## Consequences

- Matches the org-wide PostgreSQL default and gives access to standard `psql` tooling.
- Requires a running Postgres instance alongside the desktop app — heavier than an embedded option for a single-user local tool, accepted as a deliberate trade-off.
- Vector columns use 768 dimensions (of the `gemini-embedding-2` model's supported 128–3072 range) for pgvector index efficiency.
