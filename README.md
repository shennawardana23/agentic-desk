# Agentic Desk

**Desktop-native AI second brain** — real-time voice assistant, knowledge graph, skill/prompt catalogs.

---

## Architecture

```
┌─────────────────┐     ┌──────────────────────┐     ┌────────────────┐
│   Desktop App   │────▶│   Core Backend (Go)  │────▶│  Gemini Live   │
│  (Wails v2)     │     │                      │     │  + Genkit API  │
│                 │     │  HTTP + WebSocket     │     │                │
│  Vue.js +       │     │  Second Brain        │     │  Voice (Live)  │
│  AudioWorklet   │     │  Voice Live Relay    │     │  Text (Genkit) │
│  WKWebView      │◀────│  Genkit Flows        │◀────│  Embeddings    │
└─────────────────┘     └──────────┬───────────┘     └────────────────┘
                                   │
                                   ▼
                          ┌────────────────┐
                          │  PostgreSQL    │
                          │  (local DB)    │
                          └────────────────┘
```

## Features

- 🎙️ **Real-time voice assistant** — Gemini Live API, no send button, full-duplex audio, barge-in
- 🧠 **Second Brain** — persistent knowledge store with embeddings and semantic search
- 📚 **Skill & Prompt Catalogs** — searchable libraries with metadata
- 📋 **Task Management** — board-style task tracking
- 🔍 **Memory Search** — full-text + vector search across indexed content
- 👤 **Profile Management** — user configuration and persona settings
- 🗺️ **Knowledge Graph** — visualize relationships across stored content
- 🖥️ **Desktop native** — macOS `.app`, auto-launch on login, single-instance

## Quick start

```bash
# Build everything
make desktop-frontend   # Vue.js SPA
make desktop-build      # Package desktop .app

# Configure API key (once)
make configure-key KEY=sk-YOUR-GEMINI-API-KEY

# Run
make desktop-run        # Single instance (no duplicates)

# Or run frontend in browser for development
make web
```

## Technical stack

| Layer | Technology |
|-------|-----------|
| Desktop shell | Wails v2 (Go → native macOS) |
| Frontend | Vue.js 3 (Composition API), Vite, Pinia |
| Backend | Go 1.22+, Gin, Gorilla WebSocket |
| AI provider | Google Gemini (Live API + Genkit SDK) |
| Database | PostgreSQL (local, trust-auth) |
| Audio mic | AudioWorklet (raw PCM16 @16kHz) |
| Audio playback | Web Audio API (gapless scheduling) |
| Voice protocol | Binary PCM16 + JSON WS frames |

## Latency optimizations

| Optimization | Before | After | Impact |
|-------------|--------|-------|--------|
| Chunk size | 2048 / 128ms | **512 / 32ms** | -96ms first-word |
| Playback scheduling | cumulative drift | **adaptive reset** | -370ms drift |
| Noise calibration | 30 frames / ~500ms | **10 frames / ~167ms** | -333ms startup |
| Level meter context | double-close on shared | **owns its own** | crash fix |

## Documentation

See [`docs/README.md`](docs/README.md) for the full documentation index (Diátaxis structure), and [`AGENTS.md`](AGENTS.md) if you're an AI agent or developer making changes.

## Project

A product of **Arcipelago International** — building the future of AI-augmented knowledge work.
