# PLAN — Sub-project 1: Foundation + Second Brain + Agent Loop

Design reference: `docs/superpowers/specs/2026-07-06-foundation-second-brain-design.md`
Status: not started. Update each phase's status inline as work lands — this file is a living execution record, not a one-time snapshot.

Rule for every phase: land it, verify it (test/command listed), commit only that phase's diff before starting the next. No phase depends on a later one — each is independently mergeable.

## Phase 0 — Repo scaffold

**Status: done, verified 2026-07-06.** `gofmt`/`go vet`/`go build`/`go test` all clean; fail-fast and clean-exit behavior manually confirmed. Module path resolved from the actual git remote: `github.com/shennawardana23/agentic-desk`. Toolchain note: installed Go is `1.25.7`, not the `1.26.4` the design doc specifies — `go.mod` uses `go 1.25` (matches what's installed) since nothing in this phase needs 1.26-specific features; upgrading is a non-blocking follow-up, not deferred silently. Also added `.gitignore` (not in the original phase list — standard Go project hygiene, one file, directly serves "repo scaffold").

- `go.mod` (module path, Go 1.26), `cmd/core/main.go` (empty entrypoint), `cmd/desktop/main.go` (empty Wails v2 entrypoint), `internal/config/` (env loader, fail-fast on missing required vars: `DATABASE_URL`, `GEMINI_API_KEY`).
- Verify: `go build ./...` succeeds; `go run ./cmd/core` exits cleanly with a "config loaded" log line.

## Phase 1 — Database + migrations

**Status: done, verified 2026-07-06.** Local Postgres (Homebrew, not Docker — none installed) with `agentic_desk` database and pgvector 0.8.1 enabled. Migration runner (`internal/migrate`, embedded via `migrations/embed.go`) proven idempotent by two consecutive real runs (`go run ./cmd/core`: applied 1, then applied 0) and by the integration test. Deviation from the original plan: the integration test was conceived around `testcontainers-go` but Docker isn't installed here, so it runs directly against the local Postgres via `DATABASE_URL` instead — same build-tag isolation (`-tags integration`), just not containerized. One real bug found and fixed during verification: the test initially asserted the first run must apply ≥1 migration, true only against a fresh/ephemeral database — against this persistent local instance it fails on every run after the very first ever. Fixed to only assert the second-of-two-consecutive-runs is a no-op, which holds regardless of prior state.

- `migrations/0001_init.sql`: `profile_rule`, `project_context`, `memory_entry`, `feedback_signal` tables; pgvector extension + vector column (768-dim) + index on `memory_entry.embedding` and `profile_rule.embedding`.
- Migration runner (simple embedded-SQL runner, no heavyweight framework needed for 4 tables — YAGNI).
- Verify: `docker run` (or existing local) Postgres + pgvector, run migrations, `\d+` shows expected schema.

## Phase 2 — Second Brain domain + Postgres adapter

**Status: done, verified live 2026-07-06.** `internal/secondbrain/` and `internal/secondbrain/postgres/` built as specified (entity names matched to the migration: `ProfileRule` not `Profile`). Unit tests pass against an in-memory fake `Store`; integration tests were actually *run* against the local Postgres (`DATABASE_URL` exported, not left unset) so they exercised real SQL, not a skip. Deviation: no `testcontainers-go` — same real-local-Postgres substitution as Phase 1, for the same reason (no Docker). Dependency note: `pgvector-go` v0.4.0 moved its `pgx` support to a separate module path, `github.com/pgvector/pgvector-go/pgx`.

- `internal/secondbrain/`: `Profile`, `ProjectContext`, `MemoryEntry`, `FeedbackSignal` types + `Store` interface (Get/Upsert/SearchByVector per entity). Zero framework imports.
- `internal/secondbrain/postgres/`: `Store` implementation using `pgx`.
- Verify: unit tests against in-memory fake `Store` (domain logic); integration test (build-tag gated) against real Postgres via testcontainers-go for the adapter.

## Phase 3 — Importer (deterministic profile seeding)

**Status: done, verified 2026-07-06.** `Parse` is a pure function (no I/O), one `Rule` per markdown heading at any ATX level, fenced-code-aware. `Apply`/`ImportPaths` implement the new/changed/unchanged/overridden diff exactly as specified, overridden rules never rewritten. All tests deterministic — fixture-based exact-struct comparison, a fenced-code case, a determinism case, and diff/apply against a local fake `Store`.

- `internal/importer/`: markdown parser walking `~/.claude/CLAUDE.md`, `RULES.md`, `PRINCIPLES.md` (paths configurable), extracting `(source_file, heading, line_range, content_hash, content)` per rule/section.
- Diff-by-hash logic: new/changed/unchanged/user-overridden-so-suggest-only, writing to `profile_rule` via `Store`.
- Verify: table-driven tests with fixture markdown files → exact expected parsed structs. No live LLM calls in this test path — must be 100% deterministic.

## Phase 4 — Embedding pipeline

**Status: done, pool verified live (incl. `-race`); live Gemini smoke test unverified 2026-07-06.** Model string confirmed against the live `github.com/firebase/genkit/go` v1.10.0 source — `gemini-embedding-2` is real, resolving Open Item #1. Correction to the plan's own assumption: that model's *native* output is 3072 dims, not 768; requesting `OutputDimensionality: 768` via `genai.EmbedContentConfig` (also verified against the live `google.golang.org/genai` source) keeps the already-built schema unchanged. Pool built with `errgroup.Group` + timeout-bounded shutdown wait exactly as specified. `TestGenkitEmbedder_Smoke` requires `GEMINI_API_KEY`, unset in this sandbox — confirmed it skips cleanly rather than silently passing.

- `internal/embedding/`: Genkit `googlegenai.GoogleAIEmbedder` wrapper around `gemini-embedding-2` (confirm model string against live SDK first — Open Item #1 in design doc), 768-dim output.
- Bounded worker pool: fixed-N goroutines, `errgroup.Group`, channel of pending IDs, context-cancel + `WaitGroup.Wait(timeout)` shutdown.
- Verify: goroutine-leak test (`goleak` or `runtime.NumGoroutine()` diff) around pool start/stop; a smoke test embedding one fixture string against a live (or recorded) API call.

## Phase 5 — Genkit app + Dotprompt + built-in middleware

**Status: done, composition order verified against live SDK source; live flow round-trip unverified 2026-07-06 (no `GEMINI_API_KEY`).** Read `ai/generate.go` (`buildModelChain`) and `plugins/middleware/fallback.go` directly rather than trusting the design doc's own prose — confirmed `ai.WithUse(Fallback, Retry)` makes Fallback outermost, and that each fallback candidate gets exactly one attempt (not retried), correcting the plan's "fallback also retried" phrasing. Two tests against the real middleware with local fake models prove exact call counts, no live API needed for that part.

- `internal/genkit/`: Genkit Go app initialization with `ai.WithUse(&middleware.Fallback{Models: [...]}, &middleware.Retry{MaxRetries: 3})` (Fallback outer, Retry inner — see design doc decisions table for why) and `&middleware.Skills{SkillPaths: []string{"skills"}}` wired in, Dotprompt (`.prompt`) file loader, one placeholder flow to prove wiring (no real chat logic yet — that's sub-project 2).
- `prompts/`: one example `.prompt` file with input/output JSON schema.
- `skills/README.md`: states the convention (one `SKILL.md` per capability directory) — no actual skill files yet, deferred to sub-project 6 per corrected `SKILL.md`.
- Verify: `go test` round-trips the placeholder flow through the Genkit dev harness; a forced-failure test confirms `Fallback`+`Retry` composition order behaves as designed (primary retried before falling back, fallback also retried).

## Phase 6 — Agent Loop primitive + Eval interface

**Status: done, verified 2026-07-06.** Design doc correction found before coding: "ADK Go 2.0" doesn't exist (module tops out at v1.5.0) — matches this phase's own deferral of real ADK wiring to "Phase 6b," so no ADK import was needed here. `agentloop/feedback` stayed decoupled from `secondbrain` per Section 5 — `SignalWriter` interface only, no adapter yet. Six exact-count tests pin the bounded-retry + escalate-exactly-once property.

- `internal/eval/`: `Evaluator` interface + one deterministic default implementation (schema/structural validation against a Dotprompt output schema).
- `internal/agentloop/`: Plan→Act→Observe→Critique state machine, `maxIterations`-bounded self-correction, HITL escalation hook (stubbed pause — real ADK Go 2.0 durable pause/resume wiring is Phase 6b once Open Item #4 is verified, including the Python-vs-Go GA maturity caution now in design doc Section 10).
- `internal/agentloop/feedback/`: capture human decision at escalation/commit, write `FeedbackSignal` via `Store`.
- Tool-call-level gating uses the built-in `middleware.ToolApproval` (allow-list) — separate from, and complementary to, this loop's own output-quality HITL escalation.
- Verify: unit tests driving the state machine with a scripted fake `Evaluator` — assert the retry cap is honored and HITL escalation fires exactly once at exhaustion, never loops unbounded.

## Phase 7 — MCP server (Second Brain exposed to external agents)

**Status: done, verified live 2026-07-07.** Correction to this phase's own assumption: Streamable HTTP isn't available (verified against both `plugins/mcp/server.go` and the pinned `mark3labs/mcp-go`'s stub HTTP server) — stdio only. Test client uses `github.com/modelcontextprotocol/go-sdk` (official SDK), not `mark3labs/mcp-go` directly, per explicit direction — production code only touches Genkit's own `plugins/mcp`. Real client↔server round trip over actual stdio, fully offline, passes under `-race`.

- `internal/mcp/server/`: Genkit `plugins/mcp` server exposing `secondbrain.get_profile`, `secondbrain.search_memory`, `secondbrain.log_session`, `secondbrain.get_project_context` as MCP tools over stdio + Streamable HTTP.
- Verify: integration test — a real MCP client dials in, lists tools, round-trips one call.

## Phase 8 — External tool integrations

**Status: GitHub half done, verified 2026-07-07; YouTrack half explicitly skipped by user decision (deployment type not provided).**

- `internal/tools/github/`: `google/go-github` **v72** (confirmed latest via `go list -m -versions`), wrapped as 7 Genkit tools (`github.list_issues`, `.get_issue`, `.create_issue`, `.comment_on_issue`, `.list_pull_requests`, `.create_pull_request`, `.merge_pull_request`) behind a `Client` interface — `client.go` has zero genkit/ai imports (mirrors `internal/secondbrain`'s domain/framework split), `tools.go` does the Genkit wiring, `toolErr()` sanitizes GitHub API error bodies before they reach an external MCP caller (same contract as `internal/mcp/server/tools.go`'s fix this session). Auth is `NewClient(token string)` — no fallback to unauthenticated requests. Not yet wired into a running Genkit instance/MCP server (same as Phase 7's tools, which are also only exercised by tests today — no phase has built a "run the MCP server" `cmd/` binary yet).
- `internal/mcp/client/` (YouTrack): **not built.** User explicitly deferred this — deployment type (self-hosted vs. `youtrack.jetbrains.com` cloud) still unanswered. Revisit when that's known.
- Verify: 7 unit tests against a fake `Client` (no real GitHub API calls) — round-trips for issue/PR create+read, comment, merge, plus two tests pinning the error-sanitization contract. `go build`/`vet`/`gofmt`/`test`/`test -race` all clean. Not verified live against the real GitHub API (no token available in this environment) — same honesty standard as Phase 4/5's unverified live Gemini calls.

## Phase 9 — Core↔GUI API

**Status: done, verified 2026-07-07.**

- `internal/api/`: Gin routes (`health`, `profile`, `profile/rule`, `project-context` GET/PUT, `memory` POST/GET/`search`) — `Deps{Store, Embedder, Hub}` interfaces only, mirroring `internal/mcp/server`'s dependency shape. `apiErr()` sanitizes every backend error before it reaches the response body (same contract as `internal/mcp/server` and `internal/tools/github`'s tool errors this session). `hub.go`'s `Hub` is a non-blocking broadcast pub/sub (`Publish`/`Subscribe`); `EscalationHandler` adapts it to `agentloop.EscalationHandler` so a real `Loop.Run`'s HITL escalation publishes an `EventHITLEscalation` a connected GUI can render — `agentloop` itself stays free of any transport import, per its own package doc.
- Verify: `httptest` covers every route including the not-found/validation/error-sanitization paths; a real WS client (`gorilla/websocket`'s `Dialer`) dials a real `httptest.Server`, and a `Hub.Publish` is asserted received over the actual wire — not a fake in-process shortcut.

## Phase 10 — Wails v2 GUI shell (minimal)

**Status: done, build-verified 2026-07-07** (user-approved scope: build-verify only, no `wails dev`/visual check — no display server in this environment).

**Deviation from this bullet's original `frontend/` path, and why:** Go's `//go:embed` can only reach files at or below the embedding source file's own directory — it cannot traverse `..` to a repo-root `frontend/`. So `frontend/` lives at `cmd/desktop/frontend/`, co-located with `main.go`'s `//go:embed all:frontend/dist`, matching every real Wails project's own layout (confirmed by scaffolding a reference project with `wails init -t vue` and inspecting its structure before writing this repo's version).

- `cmd/desktop/main.go` + `app.go`: Wails v2 app. `App` (the bound Go↔JS backend) deliberately holds no `secondbrain.Store`/`embedding.Embedder` directly — the frontend talks to `cmd/core`'s HTTP+WS API (Phase 9) over plain `fetch`, the same as any other API client, rather than duplicating a second in-process backend inside the desktop shell. The one bound method, `CoreAPIURL()`, tells the frontend where that API lives (env `CORE_API_URL`, default `http://localhost:8080`) so the URL isn't hardcoded twice.
- **New this phase, not originally scoped:** `cmd/core/main.go` didn't actually serve Phase 9's API — it only connected to the DB. For "connects to Phase 9's API" to be literally true, `cmd/core` needed to actually start it. Added: `internal/genkit.Init`, `postgres.NewStore`, `embedding.NewGenkitEmbedder`, `api.NewHub`/`NewRouter`, and `http.ListenAndServe` on a new optional `Config.APIAddr` (defaults `:8080`, `API_ADDR` env override — not fail-fast like `DATABASE_URL`/`GEMINI_API_KEY`, since it has a safe default). Live-verified: ran the built binary against the real local Postgres, `curl`'d `/health` and `/profile`, got real data back from rows written by this session's own integration tests.
- `cmd/desktop/frontend/`: Vue 3 + Pinia. `stores/core.js` centralizes every `fetch` to `cmd/core`'s API — components never fetch directly. Two screens per this bullet's own scope: `ProfileView.vue` (read imported rules via `GET /profile`), `MemorySearch.vue` (search box via `GET /memory/search`). Vue Flow not added — still correctly deferred to sub-project 5, nothing changed there.
- Verify: `wails build` (full pipeline — generate bindings, `npm install`, `vite build`, `go build`, package, self-sign) succeeded end-to-end on the first real attempt, producing a genuine signed `arm64` Mach-O `.app` bundle. `go build`/`vet`/`gofmt`/`test`/`test -race` clean from the repo root (not just `cmd/desktop`'s own subtree) after the build. Not verified: no `wails dev` run, no actual window rendered or manually clicked through — this environment has no display server, exactly the gap flagged before starting and accepted by the user up front.

## Out of scope here (see design doc Section 9)

Chat/voice behavior, multi-agent A2A orchestration, LLM-as-judge evaluator, flow visualization, full docs suite.

**Update 2026-07-07: text chat is no longer out of scope — built, real, not a stub.** `internal/genkit.DefineChatFlow` (Sarza's persona, real `genkit.Generate` call, same Fallback+Retry middleware the placeholder flow already proved) + `POST /chat` + `ChatView.vue` (default screen now) — see `SESSION_HANDOFF.md`'s dedicated entry for what's verified vs. untestable here (no real `GEMINI_API_KEY` in this sandbox). **Voice is still genuinely out of scope** — this update is text-chat only.
