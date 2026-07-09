# AGENTS.md — entry point for coding agents

> **Last updated:** 2026-07-09
> **Audience:** AI agents and human developers making changes in this repo.
> **Purpose:** Map of where things are, what conventions to follow, and what's actually runnable.

---

## Quick start

```bash
# Prerequisites
go version        # needs 1.22+
node --version    # needs 20+
psql --version    # optional — core auto-launches with defaults

# Clone + build
git clone <repo-url> agentic-desk
cd agentic-desk
make build-all    # everything

# Run standalone web frontend (browser dev)
make web

# Package + run desktop app
make desktop-build
make configure-key KEY=sk-...   # once
make desktop-run                # single instance via `open`
```

## Build status

| Command | What | Status |
|---------|------|--------|
| `make build` | Go packages | ✅ |
| `make desktop-frontend` | Vue.js SPA | ✅ |
| `make desktop-build` | Wails `.app` | ✅ |
| `make desktop-run` | Open desktop app | ✅ (no duplicate) |
| `make web` | Browser dev server | ✅ |
| `make test` | Go unit tests | ✅ |
| `make build-all` | Full CI build | ✅ |

## Package layout

```
internal/
├── secondbrain/           Domain types + Store interface  (pure Go, no framework deps)
├── secondbrain/postgres/  Postgres adapter                (only pgx/import allowed)
├── agentloop/             Plan → Act → Observe → Critique
├── voicelive/             Gemini Live API WebSocket relay
├── eval/                  Evaluator interface + default impl
├── importer/              Markdown parser                 (no LLM calls)
├── embedding/             Embedder + bounded worker pool
├── genkit/                Genkit init + flow registration
├── api/                   HTTP+WS routes (Gin + Gorilla WS)
└── config/                Env loading, fail-fast

cmd/
├── desktop/               Wails desktop shell
│   └── frontend/          Vue.js SPA
│       ├── src/components/    All views
│       ├── src/lib/           SDK + utilities (voiceLive.js, markdown.js)
│       ├── src/stores/        Pinia stores
│       └── public/            Static assets (pcm-capture-worklet.js)
└── core/                   Standalone backend server
```

**Rule of thumb:** domain packages never import storage or LLM-provider libraries directly. Interfaces live in the domain package; implementations live in sub-packages.

## Voice pipeline — real-time audio architecture

```
Mic (16kHz) → AudioWorklet (512-sample chunks) → WS binary → Go relay → Gemini Live API
                                                                              ↓
Speaker (24kHz) ← AudioContext (adaptive cursor) ← WS binary ←───────────────+
```

**Latency optimizations applied (2026-07-09):**

| Optimization | Before | After | Saving |
|-------------|--------|-------|--------|
| AudioWorklet chunk size | 2048 samples (128ms) | **512 samples (32ms)** | **-96ms** first-word |
| Playback cursor drift | cumulative | **adaptive reset at 300ms** | **-370ms** drift |
| Noise calibration | 30 frames (~500ms) | **10 frames (~167ms)** | **-333ms** startup |
| Level meter double-close | closed shared context | **owns its own context** | crash fixed |

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for full diagrams.

## Documentation (Diátaxis structure)

| Quadrant | Contents | Location |
|----------|----------|----------|
| 🔰 Tutorial | Getting started | `docs/guides/getting-started.md` |
| 📖 How-to | Voice setup, contributing | `docs/guides/` |
| 📚 Reference | Architecture, ADRs | `docs/ARCHITECTURE.md`, `docs/adr/` |
| 💡 Explanation | Design documents | `docs/designs/`, `docs/superpowers/specs/` |
| 🤖 AI-context | LLM-optimized overview | `llms.txt`, `llms-full.txt` |

## Where to look next

| File | What's inside |
|------|--------------|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Full architecture with Mermaid diagrams |
| [`SYSTEM.md`](SYSTEM.md) | System context and model |
| [`MEMORY.md`](MEMORY.md) | Second Brain data model |
| [`SKILL.md`](SKILL.md) | Agent skill model |
| [`PLAN.md`](PLAN.md) | Current implementation phases |
| [`SESSION_HANDOFF.md`](SESSION_HANDOFF.md) | Last session's state |
| [`docs/adr/`](docs/adr/) | Architecture Decision Records |
| [`docs/README.md`](docs/README.md) | Full documentation index |

## Common pitfalls

1. **Don't run `make desktop-run` repeatedly** — use `open` (handled by Makefile) to prevent duplicate instances.
2. **Don't import `pgx` outside `internal/secondbrain/postgres/`** — that's a SoC violation.
3. **Don't add dependencies for what 10 lines of code can do** — check stdlib first.
4. **Don't skip verifying SDK method signatures** — verify against live source code, not docs.
5. **Don't let docs drift** — every change updates docs in the same commit.

## Engineering conventions

- **SOLID** — Dependency Inversion (interfaces owned by domain, adapters in sub-packages)
- **YAGNI** — build sub-project N before sub-project N+1
- **Goroutines** — bounded pools, context-cancel shutdown, goroutine-leak tests
- **Zero-trust** — verify SDK signatures against live source, not blogs
- **Documentation** — every change updates docs, drift is a bug
