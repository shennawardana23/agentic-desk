# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the Agentic Desk project.

An ADR documents a significant architectural decision and the rationale behind it. ADRs follow the [established template](https://github.com/joelparkerhenderson/architecture-decision-record).

## Active ADRs

| # | Title | Status |
|---|-------|--------|
| 1 | [Use Wails v2 for Desktop Shell](0001-use-wails-for-desktop.md) | ✅ Accepted |
| 2 | [Gemini Live API for Real-time Voice](0002-gemini-live-for-voice.md) | ✅ Accepted |
| 3 | [AudioWorklet for Mic Capture](0003-audioworklet-for-capture.md) | ✅ Accepted |
| 4 | [Go Backend as Local API Server](0004-go-backend-as-local-api.md) | ✅ Accepted |
| 5 | [PCM16 Binary Frames over WebSocket](0005-pcm16-binary-ws.md) | ✅ Accepted |
| 6 | [Separate Capture and Playback AudioContexts](0006-separate-audio-contexts.md) | ✅ Accepted |
| 7 | [Persisted Env File for Packaged App Keys](0007-persisted-env-file.md) | ✅ Accepted |

## ADR Lifecycle

1. **Proposed** — decision is under discussion
2. **Accepted** — decision is adopted
3. **Deprecated** — decision is superseded
4. **Superseded by** — link to the ADR that replaced it

## Template

```markdown
# ADR-N: Title

**Status:** Proposed | Accepted | Deprecated

**Date:** YYYY-MM-DD

## Context

What is the issue motivating this decision?

## Decision

What is the change being made?

## Consequences

What becomes easier or harder?

## Compliance

How will compliance be enforced?

## Notes

Any additional context.
```
