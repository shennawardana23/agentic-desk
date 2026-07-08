# ADR-0004: Genkit's own `plugins/mcp`, not a third-party MCP SDK

Status: Accepted — 2026-07-06

## Context

The Second Brain needs to act as both an MCP server (exposing itself to external coding agents) and an MCP client (consuming YouTrack's official MCP server). A bare third-party Go MCP SDK was one option; the user explicitly required using "MCP Genkit" instead.

## Decision

Use Genkit Go's own `github.com/firebase/genkit/go/plugins/mcp` package for both roles — server (exposing Second Brain tools) and client (connecting to YouTrack's MCP server) — rather than a separate third-party MCP SDK.

## Consequences

- One fewer dependency; MCP handling stays inside the same framework already used for flows, Dotprompt, and middleware.
- Verified this package supports both client and server roles before committing to this decision (not assumed).
- GitHub integration deliberately does *not* go through this MCP client layer — the design doc's decisions table (Section 2) instead calls for the native `google/go-github` SDK, wrapped as a tool, to avoid MCP indirection where a first-party Go SDK already exists.
