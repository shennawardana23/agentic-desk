# Agentic Desk — Documentation

> **Diátaxis structure**: this documentation is organized across four dimensions — Tutorials, How-to Guides, Reference, and Explanation — following the [Diátaxis framework](https://diataxis.fr/).

## Quick Navigation

| Audience | Start Here |
|----------|-----------|
| **New developers** | [`../AGENTS.md`](../AGENTS.md) — entry point |
| **Architecture overview** | [`ARCHITECTURE.md`](ARCHITECTURE.md) |
| **System model** | [`../SYSTEM.md`](../SYSTEM.md) |
| **Data model** | [`../MEMORY.md`](../MEMORY.md) |
| **Current plan** | [`../PLAN.md`](../PLAN.md) |
| **Design documents** | [`designs/`](designs/) |
| **Architecture decisions** | [`adr/`](adr/) |
| **Past sessions** | [`handoffs/`](handoffs/) |
| **Project specs** | [`superpowers/specs/`](superpowers/specs/) |
| **Reviews** | [`reviews/`](reviews/) |

## Documentation Map

```
docs/
├── README.md              ← You are here
├── adr/                   ← Architecture Decision Records
│   ├── README.md
│   ├── 0001-use-wails-for-desktop.md
│   ├── 0002-gemini-live-for-voice.md
│   └── ...
├── designs/               ← Design docs for features
│   └── ...
├── superpowers/           ← Major sub-project specs
│   └── specs/
│       ├── 2026-07-06-foundation-second-brain-design.md
│       └── 2026-07-08-voice-live-realtime-design.md
├── reviews/               ← Code review records
│   └── ...
├── handoffs/              ← Session handoff records
│   └── ...
├── ARCHITECTURE.md        ← Full architecture with diagrams
├── api/                   ← API documentation
│   └── voice-live-api.md
├── operations/            ← Deployment & operations
│   ├── deploy.md
│   └── troubleshooting.md
└── guides/                ← How-to guides
    ├── getting-started.md
    ├── voice-assistant.md
    └── contributing.md
```

## Diátaxis Quadrants

| Quadrant | What | Where |
|----------|------|-------|
| **🔰 Tutorial** | Step-by-step learning | `guides/getting-started.md` |
| **📖 How-to Guide** | Task-oriented recipes | `guides/voice-assistant.md`, `guides/contributing.md` |
| **📚 Reference** | Technical specification | `ARCHITECTURE.md`, `adr/`, `api/` |
| **💡 Explanation** | Background & reasoning | `designs/`, `superpowers/specs/` |
