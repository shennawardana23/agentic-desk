# SYSTEM.md — architecture overview

**This describes the target architecture from the approved design doc. It is not yet built.** See [`PLAN.md`](PLAN.md) for what actually exists at any point in time; if this file and `PLAN.md`'s status ever disagree, `PLAN.md` is correct and this file needs updating.

Full rationale: [sub-project 1 design doc](docs/superpowers/specs/2026-07-06-foundation-second-brain-design.md).

## Process architecture

Two entrypoints, one shared internal codebase. `cmd/core` is a headless process that keeps the Second Brain and its MCP server reachable independent of whether the GUI is open — this was chosen specifically because a monolith-desktop design (everything inside the Wails process) would mean external agents lose access the moment you close the window.

```mermaid
graph TB
    subgraph external["External agents"]
        CC[Claude Code / other coding agents]
    end

    subgraph core["cmd/core — headless process"]
        MCPServer["internal/mcp/server<br/>(Second Brain exposed as MCP tools)"]
        SecondBrain["internal/secondbrain<br/>(domain: Profile, ProjectContext,<br/>MemoryEntry, FeedbackSignal)"]
        AgentLoop["internal/agentloop<br/>(Plan/Act/Observe/Critique)"]
        Eval["internal/eval<br/>(Evaluator interface)"]
        Genkit["internal/genkit<br/>(Genkit Go app, Dotprompt)"]
        Embedding["internal/embedding<br/>(gemini-embedding-2, worker pool)"]
        Importer["internal/importer<br/>(deterministic profile seeding)"]
        GithubTool["internal/tools/github"]
        API["internal/api<br/>(Gin + Gorilla WS)"]
        MCPClient["internal/mcp/client"]
    end

    subgraph gui["cmd/desktop — Wails v2 GUI"]
        Vue["Vue 3 + Pinia + Vue Flow"]
    end

    Postgres[(PostgreSQL + pgvector)]
    Gemini[Gemini API]
    GitHubAPI[GitHub API]
    YouTrackMCP[YouTrack official MCP server]

    CC -->|MCP| MCPServer
    MCPServer --> SecondBrain
    AgentLoop --> Eval
    AgentLoop --> Genkit
    Genkit --> Gemini
    Embedding --> Gemini
    SecondBrain --> Postgres
    Embedding --> SecondBrain
    Importer --> SecondBrain
    GithubTool --> GitHubAPI
    MCPClient -->|MCP| YouTrackMCP
    Vue -->|local Gin/WS API| API
    API --> SecondBrain
    API --> AgentLoop
```

## Agent Loop primitive

Every agent built in later sub-projects (chat, voice, orchestrator, tool-agents) runs through this same loop shape. It is deliberately bounded — unbounded self-correction is a resilience hazard, not a feature (see [`docs/adr/0007-bounded-agent-loop-honest-rlhf.md`](docs/adr/0007-bounded-agent-loop-honest-rlhf.md)).

```mermaid
flowchart TD
    Plan[Plan] --> Act[Act]
    Act --> Observe[Observe]
    Observe --> Critique["Critique / Judge<br/>(Evaluator interface)"]
    Critique -->|pass| Commit[Commit]
    Critique -->|correctable, iterations remain| SelfCorrect["Self-correct<br/>(re-plan with failure reason)"]
    SelfCorrect --> Plan
    Critique -->|iterations exhausted OR<br/>requiresHuman flagged| HITL["HITL escalation<br/>(durable pause via ADK Go 2.0)"]
    HITL --> HumanDecision[Human decision / correction]
    HumanDecision --> Feedback["Feedback Annotator"]
    Feedback --> SecondBrainSignal["FeedbackSignal written<br/>to Second Brain"]
```

Provider-level retry (rate limits, transport errors) is handled inside Genkit's middleware, one layer below this diagram — the Agent Loop never sees a provider failure, only a semantic Critique verdict. Keeping these two resilience layers orthogonal is deliberate; see [`docs/adr/0007-bounded-agent-loop-honest-rlhf.md`](docs/adr/0007-bounded-agent-loop-honest-rlhf.md).

## Related docs

- [`MEMORY.md`](MEMORY.md) — the Second Brain's data model and import pipeline in detail
- [`docs/adr/`](docs/adr/) — one ADR per major architectural decision reflected in the diagrams above
