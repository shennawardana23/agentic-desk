# ADR-6: Separate Capture and Playback AudioContexts

**Status:** Accepted

**Date:** 2026-07-09

## Context

The voice pipeline needs three audio processing paths:
1. **Capture**: Mic input → AudioWorklet → WS binary frames to Go backend
2. **Level meter**: AnalyserNode for orb visual feedback
3. **Playback**: WS binary frames from Go → AudioContext → gapless scheduling → speakers

Each path has a different sample rate requirement: capture at 16kHz (Gemini Live API input standard), playback at 24kHz (Gemini Live API output standard), level meter at hardware default rate.

## Decision

Use **three separate AudioContext objects**, each at its required sample rate:

| Context | Sample Rate | Purpose | Connected to destination? |
|---------|-------------|---------|--------------------------|
| Capture | 16kHz | AudioWorklet → WS | **No** |
| Level meter | default | AnalyserNode for orb | No (analyser only) |
| Playback | 24kHz | gapless scheduling | **Yes** (speakers) |

The capture AudioContext is intentionally **not connected to `destination`** (speakers). AudioWorkletNode process() stays alive from its input stream (mic) alone — the output connection to destination was removed to eliminate:
- Sample-rate conversion artifacts (16kHz→44.1kHz)
- CPU overhead from maintaining a silent output pipeline
- Potential feedback path through the audio driver

## Consequences

**Positive:**
- No sample-rate conversion in the capture path
- Cleaner audio separation (capture never reaches speakers)
- Lower CPU usage

**Negative:**
- Some browsers may stop AudioWorklet processing without destination connection (verified working: Chrome 120+, Safari 17+, WebKit on macOS Sequoia)
- Three separate AudioContexts use more memory than one shared context

## Compliance

The capture AudioContext must never connect to `ctx.destination`. The playback AudioContext is the only context that reaches the speakers.
