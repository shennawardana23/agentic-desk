# ADR-0005: Headless core process + thin GUI shell

Status: Accepted — 2026-07-06

## Context

Three process architectures were considered: (A) a monolith desktop process (everything inside Wails), (B) a headless core process with the GUI as a thin client of the same API external agents use, (C) full microservices per agent domain. The stated goal is that the Second Brain must be reachable by external coding agents "any time," not only while the desktop GUI happens to be open.

## Decision

Adopt Approach B: `cmd/core` is a headless process owning the Second Brain, Genkit runtime, and MCP server/client; `cmd/desktop` (Wails v2) launches or attaches to it and talks to it over the same local Gin/WS API that external agents use.

## Consequences

- Approach A was rejected because the MCP server would only be reachable while the GUI window is open — a direct contradiction of the stated goal.
- Approach C was rejected as over-engineering for a single-user local tool — unnecessary network-distributed complexity with no corresponding benefit at this scale.
- The GUI becomes a dogfooding client of the app's own public API rather than having a separate internal protocol — one API surface to maintain and test, not two.
