# ADR-2: Gemini Live API for Real-time Voice

**Status:** Accepted

**Date:** 2026-07-08

## Context

The voice assistant needed to move from push-to-talk (record → HTTP POST → wait → reply) to real-time bidirectional audio. Options:

1. **Web Speech API** — not available in WKWebView (desktop app's WebView)
2. **Google Cloud Text-to-Speech** — one-shot synthesis only, not streaming
3. **Gemini Live API** — stateful bidirectional WebSocket, built for real-time conversation
4. **OpenAI Realtime API** — alternative but requires different SDK, key management

## Decision

Use **Gemini Live API** via `google.golang.org/genai` SDK (`v1.57.0`).

The Live API provides:
- Bidirectional audio streaming over a single WebSocket
- Automatic server-side VAD (Voice Activity Detection)
- Barge-in (interrupt) support
- Built-in speech recognition + synthesis
- Same Gemini model family as the existing chat backend

## Consequences

**Positive:**
- No send button — continuous audio in both directions
- Barge-in for natural conversation flow
- Server-side VAD eliminates need for client VAD in v1
- Go SDK support via pinned `google.golang.org/genai`

**Negative:**
- Requires internet connection to Google API
- 15-minute session cap (no `ContextWindowCompression` in v1)
- Preview model IDs churn — must verify before releases
- 1-3s processing latency from LLM inference (hard ceiling)

## Compliance

All voice sessions must use `internal/voicelive` package. The wire protocol must match: binary = PCM16 audio, text = JSON `{type, payload}`.
