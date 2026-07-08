# Agentic Desk

A personal agentic desktop assistant: chat agent, voice speech-to-speech agent, and a **Second Brain** that lets any coding agent or harness — Claude Code, other CLIs, IDE agents — know your actual coding principles, active project context, and interaction history, instead of starting from zero every session.

**Status: pre-implementation.** No code exists yet. What exists is the approved design and execution plan for the first sub-project. Nothing in this repo currently runs.

## Start here

- [`PRODUCT.md`](PRODUCT.md) — what this is, who it's for, why it's differentiated
- [`SYSTEM.md`](SYSTEM.md) — target architecture (headless core + thin GUI shell)
- [`DESIGN.md`](DESIGN.md) — index of design specs, one per sub-project
- [`PLAN.md`](PLAN.md) — the phase-by-phase build plan for the current sub-project
- [`AGENTS.md`](AGENTS.md) — entry point for coding agents working in this repo
- [`docs/README.md`](docs/README.md) — full documentation map (Diátaxis)
- [`llms.txt`](llms.txt) / [`llms-full.txt`](llms-full.txt) — machine-readable project summary

## Get started

There is nothing to build or run yet. The first actionable work is **Phase 0** in [`PLAN.md`](PLAN.md): repo scaffold (`go.mod`, `cmd/core`, `cmd/desktop`, `internal/config`). Once Phase 0 lands, this section will be replaced with real build/run instructions — see the standing rule in [`AGENTS.md`](AGENTS.md) that documentation must be updated alongside every implementation change.

The platform is deliberately decomposed into ~9 ordered sub-projects (see [`DESIGN.md`](DESIGN.md)) so no single change is unreviewably large. Sub-project 1 — Foundation + Second Brain + Agent Loop — is the only one currently spec'd and planned.

## Pitfalls (read before you assume anything)

- **Wails v3 is alpha.** This project intentionally targets Wails **v2** (stable). Don't upgrade to v3 assuming it's a drop-in improvement — its API is still moving.
- **The embedding model string is unverified.** The design targets Gemini `gemini-embedding-2`, but the only worked example in the current Genkit Go docs shows the older `text-embedding-004`. Confirm against the live SDK before assuming the newer model string works as-is (see design doc, Open Item #1).
- **YouTrack MCP may not be usable for you.** JetBrains' official MCP server isn't available for external users on the multi-tenant `youtrack.jetbrains.com` cloud. Confirm your deployment type before building on this integration (Open Item #3).
- **GitHub integration is not MCP.** It deliberately uses the native `google/go-github` SDK wrapped as a tool, not an MCP client — don't reintroduce MCP indirection here without a reason.
- **The Agent Loop's self-correction is bounded on purpose.** If you're extending it, do not remove the `maxIterations` cap — unbounded self-correction is a resilience hazard, not a feature.
- **"RLHF Annotator" is not literal RLHF.** This platform calls hosted models via API with no weight access. What's built is human-feedback capture that biases future prompts/context — don't describe it as gradient-based preference training in docs or to users.

## Contributing

This is a personal project, not currently open for outside contribution. If that changes, this section and `docs/how-to/` will be updated first.
