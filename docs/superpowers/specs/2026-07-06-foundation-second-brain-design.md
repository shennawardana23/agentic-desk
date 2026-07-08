# Design: Foundation Scaffold + Second Brain Core + Agent Loop Primitive

Status: approved (pending final user sign-off on this file)
Date: 2026-07-06
Scope: first of ~9 ordered sub-projects for the Agentic Desk platform (see "Platform Decomposition" below). This spec covers the foundation scaffold, the Second Brain core, and the Agent Loop primitive that every future agent (chat, voice, orchestrator, tool-agents) will run through. Chat/voice behavior, multi-agent A2A orchestration, flow visualization, and the docs suite are separate specs, built in that order.

## 1. Context

Agentic Desk is a personal desktop AI assistant platform: chat agent, voice speech-to-speech agent, and a "Second Brain" that lets *any* coding agent or harness (Claude Code, other CLIs, IDE agents) know the user's coding principles, active project context, and interaction history. The repo starts empty (one placeholder README) — this is a from-scratch build.

The full platform was decomposed into ordered sub-projects because it described multiple independent subsystems:

1. **Foundation scaffold + Second Brain core + Agent Loop primitive** ← this spec
2. Single-agent chat pipeline (Genkit Go flow, Dotprompt persona, provider fallback middleware)
3. Multi-agent orchestration (ADK Go, A2A, MCP tool wiring, skills middleware, handoff/HITL)
4. Voice STS agent (separate real-time audio pipeline)
5. Agentic flow visualization (Vue Flow + tracing/observability)
6. Tool-agents (search, fetch, image-gen, code-review/self-correction)
7. Tracing/Observability + Evals (LLM-as-judge, hardening pass)
8. Docs system (ADRs, Diátaxis, llms.txt, AGENTS.md/SYSTEM.md/etc.)

The Agent Loop primitive (new in this revision) is cross-cutting infrastructure, not a numbered sub-project — every later agent built in sub-projects 2–6 runs through it.

## 2. Decisions and rationale

| Decision | Choice | Why |
| --- | --- | --- |
| Language/runtime | Go 1.26.4 | Current stable (verified via go.dev/blog/go1.26); Green Tea GC now default |
| Desktop shell | Wails **v2** (not v3) | v3 is still alpha (verified via wails.io/v3 docs); org convention is latest-*stable* |
| Frontend | Vue 3 + Pinia + Vue Flow | Composition API pairs naturally with Wails' event/binding bridge; Vue Flow purpose-built for the agent-execution graph in sub-project 5 |
| Second Brain storage | PostgreSQL + pgvector | Matches org-wide DB default (explicit user choice) |
| LLM/agent framework | Genkit Go **1.0 GA** | Verified GA/stable, semver-locked; required by user |
| Multi-agent framework | ADK Go **2.0 GA** (June 30 2026) | Verified GA; graph-based workflow engine with built-in HITL and durable pause/resume — its node/edge model is the substrate the Agent Loop primitive below is built on |
| Embedding model | Gemini **gemini-embedding-2**, 768-dim | Verified current GA multimodal embedder; **open verification item**: confirm exact model-string acceptance in `googlegenai.GoogleAIEmbedder` at implementation time (docs example only shows the older `text-embedding-004`) |
| Process architecture | Headless `core` process + thin Wails GUI client | Only approach where the Second Brain/MCP server stays reachable by external coding agents when the GUI is closed |
| External agent interface | Genkit Go's own `plugins/mcp` (client + server) | User required "MCP Genkit" explicitly; supports both roles natively |
| GitHub integration | Native `google/go-github` SDK, wrapped as a tool | User's explicit correction: Go-native and typed, MCP indirection buys nothing for a first-party Go integration |
| YouTrack integration | YouTrack's official remote MCP server | Official JetBrains-provided (2025.3+); **known constraint**: not available for external users on multi-tenant `youtrack.jetbrains.com` cloud — user must confirm deployment type |
| Profile import | Deterministic parser (not LLM) over CLAUDE.md/RULES.md/PRINCIPLES.md | Zero-trust/no-hallucination requirement; a regex/heading-based parser is auditable and testable, an LLM summarizer is not |
| Provider resilience | Genkit Go **built-in** `middleware.Retry` + `middleware.Fallback`, composed `Fallback{Retry{model}}` (Fallback outer, Retry inner) | Verified via local Genkit Go skill reference (`developing-genkit-go/references/middleware.md`) — these are shipped, production-ready implementations, not something to hand-roll. Order matters: `ai.WithUse(A, B)` expands to `A{B{actual}}`. We want each provider (primary, then each fallback) individually retried before moving to the next, so `Fallback` must be outer and `Retry` inner — the inverse order ("Retry outer") would instead retry the *entire* fallback cascade as one unit, which is a different (wrong, for our case) resilience shape |
| Tool-call HITL gate | Genkit Go **built-in** `middleware.ToolApproval` (allow-list, interrupts non-allowed tool calls) | Verified via the same reference. This is a distinct HITL surface from the Agent Loop's own escalation below: `ToolApproval` gates *whether a specific tool call is allowed to execute at all* (e.g. a destructive action) before it runs; the Agent Loop's HITL escalation gates *whether the overall task output is good enough* after Critique has judged it. Both are real and complementary, not redundant |
| Skill capability declaration | Genkit Go **built-in** `middleware.Skills{SkillPaths: []string{"skills"}}` — a directory of `skills/<name>/SKILL.md` files (YAML frontmatter: `name`, `description`), loaded on demand via a contributed `use_skill` tool | Verified via the same reference. This directly satisfies the user's "SKILL.md middleware" requirement and is a real, shippable primitive, not something needing bespoke design — see corrected root `SKILL.md` for the distinction between that primitive and this repo's own explanatory doc of the same name |
| Agent self-correction | Bounded loop (max N iterations) with Critique step, escalating to HITL on exhaustion | Unbounded self-correction is a real resilience hazard (CPU/cost burn, possible infinite loop) — must be a hard-capped, observable state machine, not an open `while(true)` |
| "RLHF Annotator" | Structured human-feedback capture → preference signal in Second Brain, **not** literal gradient-based RLHF | Honesty requirement: this platform calls hosted models via API with no weight access, so policy-gradient training on human preference pairs is not buildable. What's actually built does the same practical job at the prompt/context layer: corrections and ratings become retrievable preference signals that bias future agent behavior |

## 3. Architecture

Two entrypoints, one shared internal codebase:

- **`cmd/core`** — headless Go process. Owns the Second Brain, the Genkit runtime, the Agent Loop primitive, and both MCP roles (server exposing Second Brain tools, client consuming YouTrack's MCP server). Runs independently of the GUI.
- **`cmd/desktop`** — Wails v2 GUI. Launches `core` as a subprocess (or attaches if already running) and talks to it over the *same* local Gin/WS API external agents use.

```
                        ┌─────────────────────────┐
 Claude Code / other    │   cmd/core (headless)    │
 coding agents  ───MCP──▶  internal/mcp/server     │
                        │  internal/secondbrain    │◀── internal/mcp/client ──▶ YouTrack MCP
                        │  internal/agentloop      │
                        │  internal/eval           │
                        │  internal/genkit         │
                        │  internal/embedding      │
                        │  internal/importer       │
                        │  internal/tools/github   │──▶ google/go-github SDK ──▶ GitHub API
                        │  internal/api (Gin/WS)   │
                        └───────────▲──────────────┘
                                    │ local Gin/WS API
                        ┌───────────┴──────────────┐
                        │   cmd/desktop (Wails v2)  │
                        │   Vue 3 + Pinia + VueFlow │
                        └───────────────────────────┘
```

## 4. Agent Loop primitive (cross-cutting, present from Foundation)

Every agent built in later sub-projects (chat, voice, orchestrator, tool-agents) runs through the same loop shape, defined once here so behavior is consistent and each stage is independently testable:

```
Plan ──▶ Act ──▶ Observe ──▶ Critique/Judge ──▶ pass? ──yes──▶ Commit
  ▲                                     │no (correctable)
  └─────────────── self-correct ────────┘
                    (bounded, max N iterations)
                                     │no (exhausted / low confidence)
                                     ▼
                          HITL escalation (pause via ADK Go 2.0's
                          durable pause/resume, surface to GUI
                          thinking-log terminal)
                                     │
                          human decision/correction
                                     ▼
                     Feedback Annotator ──▶ preference signal
                                             written to Second Brain
```

Package: `internal/agentloop`.

- **Plan/Act/Observe**: thin wrappers around a Genkit flow call. `agentloop` never talks to a provider directly — that's Genkit's job (including sub-project 2's provider-fallback middleware). This keeps the two resilience layers orthogonal: provider-level retry handles transport/rate-limit failures; Agent Loop's self-correction handles *semantic* failures (a Critique step judging the output wrong), and never needs to know a provider even exists.
- **Critique/Judge**: an `Evaluator` interface, `Evaluate(ctx, Observation) (Verdict, error)`. Foundation ships one trivial default implementation — deterministic schema/structural validation (does the output conform to the Dotprompt output schema, are required fields present) — not an LLM-as-judge yet. That's an intentional YAGNI call: building the full LLM-judge harness now, before any real agent exists to judge, would be premature; the interface being stable now means sub-project 7 swaps in a smarter `Evaluator` with zero call-site changes (Open/Closed).
- **Self-correction**: capped at `maxIterations` (config, default low single digits). Each retry re-plans with the prior Verdict's failure reason injected into context — not a blind retry.
- **HITL escalation**: on exhausting `maxIterations`, or on a Verdict flagged `requiresHuman` (e.g. destructive action, low confidence), the loop pauses using ADK Go 2.0's durable pause/resume rather than blocking a goroutine — the loop's state is persisted and can resume after the process restarts, not just after an in-memory wait. This is loop-level (output-quality) HITL. Individual tool calls get a separate, lower-level gate via the built-in `middleware.ToolApproval` (allow-list, interrupts anything not on it) — a destructive tool call can be blocked before it ever runs, independent of whether the eventual output would have passed Critique.
- **Feedback Annotator**: `internal/agentloop/feedback` captures the human's decision (approve / correct / reject) at an escalation point, or an explicit thumbs-up/down on a committed result, and writes it as a `MemoryEntry`-linked preference signal into the Second Brain — this is the concrete, honest version of "RLHF annotator" described in decision table row above.

## 5. Components (Go package layout, Separation of Concerns)

```
internal/secondbrain/            domain types (Profile, ProjectContext, MemoryEntry) + Store interface — zero framework deps
internal/secondbrain/postgres/   Store adapter: Postgres + pgvector (only place pgx/sql appears)
internal/agentloop/              Plan/Act/Observe/Critique state machine, bounded self-correction, HITL escalation
internal/agentloop/feedback/     human feedback capture → Second Brain preference signal
internal/eval/                   Evaluator interface + default deterministic implementation (LLM-as-judge lands in sub-project 7)
internal/importer/               deterministic markdown parser: CLAUDE.md/RULES.md/PRINCIPLES.md → source-traceable Rule records
internal/embedding/              Genkit gemini-embedding-2 wrapper + bounded worker pool for batch embedding
internal/genkit/                 Genkit Go app init, Dotprompt loader, flow registration
internal/mcp/server/             Genkit MCP plugin: expose Second Brain as MCP tools (stdio + Streamable HTTP)
internal/mcp/client/             Genkit MCP plugin: client manager for YouTrack's official MCP server
internal/tools/github/           google/go-github SDK wrapped as a Genkit/ADK tool
internal/api/                    Gin + Gorilla WS: core<->GUI protocol, live thinking-log stream, health
internal/config/                 env/config loading, fail-fast on missing required values
migrations/                      SQL: profile/project_context/memory_entry/feedback_signal tables + pgvector index
prompts/                         .prompt Dotprompt files (persona + JSON schemas)
frontend/                        Vue 3 + Pinia + Vue Flow (Wails frontend)
```

`internal/secondbrain` depends on nothing but the standard library and its own `Store` interface. `internal/agentloop` depends on `internal/eval`'s interface and Genkit's flow abstraction, never on a specific provider or on `internal/secondbrain` directly — feedback flows into Second Brain through the `feedback` sub-package's own narrow interface, keeping the loop reusable outside this app if ever needed.

## 6. Data flow

1. **Startup**: `core` loads config, connects to Postgres, verifies migrations are current.
2. **Import**: deterministic parser reads CLAUDE.md/RULES.md/PRINCIPLES.md → candidate rules keyed by `(source_file, heading, line_range, content_hash)`, diffed by hash (unchanged/new/changed-but-overridden handled as in prior revision).
3. **Embedding**: bounded worker pool (fixed N goroutines, `errgroup.Group` + `context.Context`), clean cancel-then-wait-with-timeout shutdown.
4. **Agent Loop execution** (used by sub-project 2 onward, primitive lives here): Plan→Act→Observe→Critique, bounded self-correction, HITL escalation with durable pause via ADK Go 2.0, Feedback Annotator writes preference signals back to Second Brain.
5. **MCP server** exposes `secondbrain.get_profile`, `secondbrain.search_memory(query, k)`, `secondbrain.log_session(...)`, `secondbrain.get_project_context(project_id)`, backed by pgvector cosine similarity.
6. **MCP client** connects lazily to YouTrack's MCP server, health-checked, failure-isolated.
7. **GUI**, when open, is a client of `core`'s same Gin/WS API, plus subscribes to a WS channel for live agent-thinking-log streaming and HITL escalation prompts.

## 7. Error handling

- Postgres unreachable at boot: bounded retry with exponential backoff, then fail fast (non-zero exit; a process supervisor can restart it).
- Embedding/LLM transport calls: `ai.WithUse(&middleware.Fallback{Models: [...]}, &middleware.Retry{MaxRetries: 3})` — Fallback outer, Retry inner (see decisions table for why this order, not the reverse). Built-in, not hand-rolled. Foundation wires this for the placeholder flow; sub-project 2 supplies the real fallback model list.
- Agent Loop semantic failures: handled by bounded self-correction + HITL escalation (Section 4) — a distinct resilience layer from provider-level retry, deliberately kept orthogonal.
- External MCP client failures (YouTrack): isolated, logged, surfaced via health check, never propagated into the Second Brain's own MCP server responses.

## 8. Testing

- `internal/secondbrain`: unit tests against an in-memory fake `Store`.
- `internal/agentloop`: unit tests driving the state machine with a fake `Evaluator` that returns scripted Verdicts — assert bounded-retry cap is honored and HITL escalation fires exactly once at exhaustion (a concrete regression test against "unbounded self-correction").
- `internal/importer`: table-driven tests, fixture markdown → expected parsed structs (deterministic, no LLM in the loop).
- `internal/embedding`: goroutine-leak test around pool startup/shutdown (`goleak` or manual `runtime.NumGoroutine()` diffing).
- `internal/secondbrain/postgres`: integration tests behind a build tag, real Postgres+pgvector via `testcontainers-go`.
- `internal/mcp/server`: integration test with a real MCP client dialing in, asserting tool list and round-trip call.

## 9. Non-goals for this sub-project

- Chat/voice agent behavior (sub-projects 2, 4)
- Multi-agent A2A orchestration logic (sub-project 3) — only the MCP server/client plumbing and the Agent Loop primitive it will depend on are built here
- LLM-as-judge evaluator implementation (sub-project 7) — only the `Evaluator` interface and a trivial deterministic default are built here
- Flow visualization UI (sub-project 5)
- Full docs suite: AGENTS.md/SYSTEM.md/ADRs/llms.txt (sub-project 8) — this spec is the only doc artifact produced now

## 10. Open items requiring verification at implementation time (not assumptions)

- Exact Genkit Go embedder model-string acceptance for `gemini-embedding-2`
- Current major version tag of `google/go-github`
- Which YouTrack deployment the user runs (self-hosted vs `youtrack.jetbrains.com` cloud) — determines whether that MCP client integration is usable today
- Exact ADK Go 2.0 API shape for durable pause/resume (used by HITL escalation) — verify against `google.golang.org/adk` docs before implementation, not assumed from the announcement blog post. Additional caution: the local ADK skill reference (`google-agents-cli-adk-code`) describes ADK 2.0's graph Workflow API as **experimental, pre-GA, opt-in** — but that reference is Python-SDK-specific and explicitly scoped ("Python only for now"). The Go SDK's GA status was verified independently via `pkg.go.dev/google.golang.org/adk` and a Google Developers Blog post, but the two SDKs may not be at the same maturity level for the same feature. Do not assume Go's durable pause/resume has the same stability guarantees as a GA label implies until checked directly against the Go package's own docs/changelog
