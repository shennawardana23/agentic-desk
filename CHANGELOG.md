# CHANGELOG

No implementation history yet. This log will be appended as each `PLAN.md` phase lands — one entry per phase, not per commit.

## 2026-07-06 — Design and documentation, no code

Sub-project 1 (Foundation + Second Brain + Agent Loop primitive) design approved. `PLAN.md` written with a 10-phase execution plan. Full documentation suite created (this file, `README.md`, `AGENTS.md`, `SYSTEM.md`, `MEMORY.md`, `SKILL.md`, `PRODUCT.md`, `DESIGN.md`, `APPEND_SYSTEM.md`, `SESSION_HANDOFF.md`, ADRs under `docs/adr/`, Diátaxis skeleton under `docs/`, `llms.txt`/`llms-full.txt`). No code exists.

Design corrected same day: Genkit Go's built-in `Retry`/`Fallback`/`ToolApproval`/`Skills` middleware verified via a live reference — replaced the design's original hand-rolled-middleware assumption and `SKILL.md`'s speculative capability model.

## 2026-07-06 — Phase 0: repo scaffold

`go.mod` (module `github.com/shennawardana23/agentic-desk`, `go 1.25`), `cmd/core/main.go`, `cmd/desktop/main.go` (placeholder), `internal/config/` (fail-fast env loader + test), `.gitignore`. `gofmt`/`go vet`/`go build`/`go test` clean; fail-fast and clean-exit behavior manually verified. First real code in the repo. No commits made.

## 2026-07-06 — Phase 1: database + migrations

Local Postgres (Homebrew) + pgvector 0.8.1, database `agentic_desk`. `migrations/0001_init.sql`, `migrations/embed.go`, `internal/migrate` (embedded runner). `cmd/core` now connects and migrates on startup — proven idempotent by two consecutive real runs and by an integration test. Docker isn't installed here, so the integration test targets the real local Postgres directly instead of `testcontainers-go`; one real test bug (invalid fresh-DB assumption) found and fixed. No commits made.

## 2026-07-06 — Phase 2: Second Brain domain + Postgres adapter

`internal/secondbrain/` (`ProfileRule`, `ProjectContext`, `MemoryEntry`, `FeedbackSignal` + `Validate()`, `Store` interface, zero framework imports) and `internal/secondbrain/postgres/` (pgvector-backed `Store`, `NewPool` registering pgvector's Go types via `github.com/pgvector/pgvector-go/pgx`). `cmd/core` now builds its pool through `postgres.NewPool`. Unit tests against an in-memory fake `Store`; integration tests run for real against the local Postgres (`-tags integration`, `DATABASE_URL` set) — round-trip, not-found, and upsert-in-place all verified live, not just skip-gated. Dependency note: `pgvector-go` v0.4.0 split its `pgx` support into a separate module path (`github.com/pgvector/pgvector-go/pgx`) — resolved by pulling both. No commits made.

## 2026-07-06 — Phase 3: Importer (deterministic profile seeding)

`internal/importer/`: `Parse` extracts one `Rule` per markdown heading (any ATX level, fenced-code-aware so a `# comment` inside a shell example doesn't fragment a section), `Apply`/`ImportPaths` diff parsed rules against the Store by content hash (`new`/`changed`/`unchanged`/`overridden`, the last never rewritten — the user's own edit wins). Table-driven fixture test asserts exact parsed structs against `testdata/sample.md`; diff/apply logic tested against a local fake `Store`. Fully deterministic, no LLM or network calls in any test. No commits made.

## 2026-07-07 — Doc-research pass: Genkit + ADK live docs vs. Phases 5–6

User asked for deep research across 12 genkit.dev/adk.dev pages, checked against already-built code (not abstract research). Dispatched 6 parallel research agents. Findings, all confirmations or forward-looking notes — no regressions found in Phases 5–7:

- **Middleware composition**: genkit.dev's own docs lead with the opposite `Retry`-outer/`Fallback`-inner example, but explicitly state *why* the reverse (this repo's choice: `Fallback` outer/`Retry` inner) behaves differently — both are legitimate; this repo's choice and its two passing tests are correct per the doc's own stated rationale.
- **Model IDs**: `PrimaryModel`/`FallbackModel` both confirmed real (`FallbackModel` doc-page fetch was inconclusive; source (`plugins/googlegenai/models.go:102`) confirms `gemini-2.5-flash` is real — source checked directly, not left on the doc-fetch alone).
- **Agentic patterns**: no built-in "Plan→Act→Observe→Critique" helper exists in Genkit — the docs' own "Iterative Refinement" pattern is implemented by hand via `genkit.DefineFlow`, confirming `internal/agentloop`'s hand-rolled `Loop` was the right call, not a reinvention.
- **Interrupts**: confirmed scoped to individual tool-call gating only, not multi-step loop suspension — validates the existing design decision to keep `EscalationHandler` a separate, loop-level concept from Genkit's tool-interrupt machinery.
- **Evaluation**: Genkit ships its own `ai.Evaluator`/`genkit.DefineEvaluator` — a dataset/CI-driven batch-evaluation framework, functionally different from `internal/eval.Evaluator`'s per-call runtime judge, but a real naming collision worth flagging. Kept `PLAN.md`'s specified name (no rename — YAGNI, no functional conflict since they're different packages) and added a doc comment in `internal/eval/eval.go` distinguishing the two so a future reader isn't confused.
- **ADK**: confirmed adk.dev is the same Google project as `google.golang.org/adk` (links its own Go SDK, same GitHub org) — consistent with this session's earlier finding that "ADK Go 2.0" doesn't exist (real max is v1.5.0). `internal/agentloop`'s design maps to ADK's own "Iterative Refinement" workflow pattern, not the raw `LoopAgent` primitive (a different mechanism: fixed sub-agent pipeline + explicit escalate flag, vs. this repo's per-call Verdict-driven stop condition) — confirms the current design over switching to mimic `LoopAgent` literally. ADK's own "Human-in-the-Loop" pattern is the eventual Phase 6b target for `EscalationHandler`.
- **Forward-looking, no action taken**: durable streaming (`genkit.DefineStreamingFlow` + `StreamManager`, HTTP-header-based resumption) is relevant to Phase 9's WS design later; `docs/client/` is JS/TS/Dart only, not Go, relevant only to Phase 10's frontend; Genkit's session API doesn't map cleanly onto `MemoryEntry`'s per-row shape, so a future real chat flow needs its own adapter, not a direct `session.Store[S]` implementation.

No commits made.

## 2026-07-07 — Bug/security audit fix pass on Phases 0–7

Resolved every unfixed item the prior session's `SESSION_HANDOFF.md` had flagged before pausing at its cost guard. All four fixes verified against the real local Postgres (`-tags integration`), not just the in-memory fakes, plus `go vet`/`gofmt`/`go test -race` clean.

- **Embedding-wipe bug** (`internal/secondbrain/postgres/store.go`, `UpsertProfileRule`/`UpsertProjectContext`): the handoff's suggested fix — unconditional `COALESCE(EXCLUDED.embedding, embedding)` — was checked against the importer's actual call pattern (`internal/importer/diff.go`'s `Apply` only upserts on `new`/`changed`, never same-content) and rejected: a blanket `COALESCE` would silently attach a stale embedding (computed from old content) to new content on every `changed` upsert — a worse, silent search-correctness bug than the original nil-wipe. Fixed instead with a `CASE`: embedding is cleared when `content_hash`/`summary` actually changed, preserved only on a true no-op re-upsert. First attempt hit a real Postgres error (`column reference "content_hash" is ambiguous`, SQLSTATE 42702) from an unqualified column inside the `CASE` — caught by the integration test, not by review; fixed by qualifying with the table name. New integration test `TestStore_UpsertProfileRule_EmbeddingSemantics` pins both branches of the invariant.
- **Unbounded vector search** (`validateSearch`): added a hard `maxSearchK = 50` clamp shared by all three vector-search call sites (`SearchProfileRulesByVector`/`SearchProjectContextsByVector`/`SearchMemoryEntriesByVector`). New DB-free unit tests (`store_test.go`) cover the clamp, pass-through, and rejection paths.
- **Unbounded `get_profile`**: `Store.ListProfileRules` gained `limit, offset int` (default 100, hard cap 200 at the DB layer) — closes the "entire table in one MCP response" gap. Threaded through the postgres implementation (real `LIMIT`/`OFFSET`), the `secondbrain.Store` interface, three test fakes, and the MCP tool's input schema (`getProfileInput.Limit`/`.Offset`). New integration test `TestStore_ListProfileRules_Pagination`.
- **MCP error message leakage**: confirmed live in Genkit's own source (`plugins/mcp/server.go:167`, pinned v1.10.0) that `mcp.NewToolResultError(err.Error())` forwards a tool's error text verbatim to the external MCP caller — framework-level, not something this repo's error wrapping alone caused, but this repo's tool code controls what `err.Error()` contains. Added `toolErr(tool, err)` in `internal/mcp/server/tools.go`: logs the real error server-side via `slog`, returns a fixed `"<tool>: internal error"` message to the caller. Applied to all four tools' error returns; the legitimate `ErrNotFound → Found:false` mapping on `get_project_context` was left untouched (not an error path).
- **`importer.ImportPaths` arbitrary path reads**: left as-is, confirmed still unreachable from any external caller (no MCP tool or GUI wires it yet) — deferred per the handoff's own note, to be addressed when it's actually wired up rather than speculatively now.
- **Dependency check** (per standing instruction to periodically check for latest stable versions): `go list -m -u` shows all direct dependencies current except `golang.org/x/sync` (v0.19.0 → v0.21.0, patch) and `google.golang.org/genai` (v1.51.0 → v1.62.0, minor). Not upgraded this session — flagged for a deliberate, separate upgrade pass rather than bundled into a bug-fix session.

No commits made.

## 2026-07-07 — Phase 10: Wails v2 GUI shell + cmd/core actually serving Phase 9's API

`frontend/` deviates from PLAN.md's original repo-root path to `cmd/desktop/frontend/` — `go:embed` can't traverse `..` to reach a repo-root directory from `cmd/desktop/main.go`, and this is also just how every real Wails project is laid out (checked by scaffolding a throwaway reference project with `wails init -t vue` before writing this repo's own version, rather than guessing).

`cmd/desktop/`: `app.go`'s `App` holds no direct `Store`/`Embedder` — it's a thin Wails↔JS bridge exposing one bound method, `CoreAPIURL()`, so the frontend knows where `cmd/core`'s API lives without hardcoding it. `frontend/`: Vue 3 + Pinia, `stores/core.js` as the single `fetch` chokepoint, `ProfileView.vue` + `MemorySearch.vue` per PLAN's two minimal screens.

**Necessary but unplanned addition:** `cmd/core/main.go` never actually served Phase 9's API — it only connected to the database. Wired it up for real: `internal/genkit.Init` → `embedding.NewGenkitEmbedder` → `postgres.NewStore` → `api.NewHub`/`NewRouter` → `http.ListenAndServe`. Added `Config.APIAddr` (optional, defaults `:8080`, `API_ADDR` override) since unlike `DATABASE_URL`/`GEMINI_API_KEY` there's a safe default. Live-verified, not just built: ran the compiled binary against the real local Postgres, `curl`'d `/health` (200 ok) and `/profile` (returned real rows this session's own integration tests had written) — genuine end-to-end proof, not a build-only claim.

Verified: `wails build` (bindings generation → `npm install` → `vite build` → `go build` → package → self-sign) succeeded fully on the first attempt, producing a real signed `arm64` Mach-O `.app`. Full `go build`/`vet`/`gofmt`/`test`/`test -race` clean from the repo root afterward — caught and fixed one process mistake along the way: running checks from inside `cmd/desktop` after `cd`ing there for `wails build` silently narrowed `go build ./...` to that one subtree; re-ran from repo root to get a real whole-repo result. Not verified: no `wails dev`, no window ever rendered — no display server in this environment, a gap disclosed and accepted by the user before starting, not discovered after the fact.

No commits made.

## 2026-07-07 — Phase 9: Core↔GUI API

`internal/api/`: Gin router (`health`, `profile` list/get, `project-context` get/put, `memory` create/get/search) backed by `Deps{Store, Embedder, Hub}` — interfaces only, no pgx/model-provider import. `apiErr()` reuses this session's error-sanitization pattern (log real error via `slog`, return a fixed opaque JSON body). `hub.go`: `Hub` is a non-blocking broadcast pub/sub for WS subscribers (a full subscriber buffer drops that subscriber's event, never blocks the publisher); `EscalationHandler` adapts `Hub` to `agentloop.EscalationHandler` so a real agent loop's HITL escalation reaches a connected GUI as an `EventHITLEscalation` — kept in `internal/api`, not `internal/agentloop`, since that package's own doc comment requires it stay transport-free.

Verified: `httptest`-driven route tests (success, not-found, validation-rejection, and error-sanitization paths) plus a genuine WS round trip — `gorilla/websocket`'s real `Dialer` connects to a real `httptest.Server`, and a `Hub.Publish` is asserted received over the actual wire, matching PLAN.md's own stated verify step rather than a same-process shortcut. `go build`/`vet`/`gofmt`/`test`/`test -race` all clean. Not wired into `cmd/core` yet — same "built and tested, not yet the running binary" gap already accepted for Phase 7/8's tools.

No commits made.

## 2026-07-07 — Phase 8 (GitHub half): external tool integration

User asked to complete all remaining phases (8/9/10); clarified scope first via direct questions rather than guessing: YouTrack skipped entirely (deployment type still unknown), GitHub tool scoped to read issues/PRs + create/comment issues + create/merge PRs.

`internal/tools/github/`: `client.go` wraps `google/go-github` v72 behind a `Client` interface (zero genkit/ai imports, same domain/framework split as `internal/secondbrain`); `tools.go` registers 7 Genkit tools (`list_issues`, `get_issue`, `create_issue`, `comment_on_issue`, `list_pull_requests`, `create_pull_request`, `merge_pull_request`) and reuses this session's `toolErr()` sanitization pattern from `internal/mcp/server/tools.go` so a GitHub API error body never reaches an external MCP caller verbatim. `NewClient(token string)` requires a real token — no silent unauthenticated fallback.

Verified: 7 unit tests against a fake `Client` (issue/PR create+read+comment+merge round trips, plus two pinning the error-sanitization contract) — `go build`/`vet`/`gofmt`/`test`/`test -race` all clean. Not verified live against the real GitHub API — no token available in this environment; same disclosed-gap standard as Phase 4/5's untested-live Gemini calls. YouTrack MCP client (`internal/mcp/client/`) not built — explicitly out of scope until deployment type is known.

No commits made.

## 2026-07-07 — Phase 7: MCP server (Second Brain exposed to external agents)

`internal/mcp/server/`: four tools (`secondbrain.get_profile`, `secondbrain.get_project_context`, `secondbrain.log_session`, `secondbrain.search_memory`) defined via `genkit.DefineTool`, backed by `secondbrain.Store`/`embedding.Embedder` interfaces only (never a concrete postgres/genkit-embedder type). `NewMCPServer` wraps them with Genkit's documented `plugins/mcp` server (https://genkit.dev/docs/go/model-context-protocol/#exposing-as-mcp-server).

**Live-verification findings:** (1) `search_memory`'s `k` field needed `json:",omitempty"` — Genkit's schema inference otherwise marks it required, silently making the "defaults to 5" design unreachable; caught by an actual failing test, not inspection. (2) Streamable HTTP is unavailable — `GenkitMCPServer.Serve(transport)` ignores its argument and hardcodes stdio (`plugins/mcp/server.go`), and the pinned `mark3labs/mcp-go` v0.29.0's `StreamableHTTPServer` is an explicit upstream stub with every method a no-op — verified in both source trees, not assumed. Documented as a real gap against PLAN's "stdio + Streamable HTTP" assumption, not silently dropped.

**Round-trip test, per explicit instruction not to depend on `mark3labs/mcp-go` directly:** the production code only ever calls Genkit's own `plugins/mcp` (which uses `mark3labs/mcp-go` transitively — `go.mod` marks it `// indirect`, not something this repo imports). The test client uses the official `github.com/modelcontextprotocol/go-sdk` instead. Result: a genuinely real MCP client↔server round trip over the actual stdio wire protocol (`os.Stdin`/`os.Stdout` swapped to pipes for the test's duration, restored after) — lists all 4 tools, calls `get_profile`, gets real content back — fully offline, no `DATABASE_URL` or `GEMINI_API_KEY` needed at all, exceeding what was expected to require DB gating. Passes under `-race` too.

Also closed a real gap found before writing this phase: `secondbrain.Store` had no way to list all profile rules (`get_profile` needs one, and no `(SourceFile, Heading)` key was knowable up front) — added `ListProfileRules` to the interface, the postgres adapter, and both existing test fakes, with its own new test.

No commits made.

## 2026-07-07 — Architecture check: repository pattern / SoC against 7 reference repos

User asked whether the DB layer should be restructured to resemble `zk-org/zk`, `satellitecomponent/Neurite`, `inkeep/open-knowledge`, `dongdongbh/Mindwtr`, `abhigyanpatwari/GitNexus`, `Graphify-Labs/graphify`, and `livekit/server-sdk-go`. Researched all 7 live via GitHub (language, purpose, folder structure) rather than trusting memory. Findings: only 2 of 7 are Go (`zk-org/zk`, `livekit/server-sdk-go`); only `zk-org/zk` has a real database/repository-pattern layer at all — `livekit/server-sdk-go` is a client SDK wrapping a media server with zero persistence, and the other 5 are TypeScript/Python/Rust projects, several explicitly browser-only/zero-server per their own READMEs. No unfounded cross-language analogy was drawn.

`zk-org/zk`'s verified shape: `internal/core` defines domain types plus a persistence-port interface (`NoteIndex`); `internal/adapter/sqlite/*_dao.go` is the only place the driver appears; `internal/cli` is the separate transport layer. This is structurally identical, port-for-port, to what this repo already had: `internal/secondbrain` (domain + `Store` interface) and `internal/secondbrain/postgres` (the sole `pgx`-importing implementation) — the repository pattern already existed and needed no rebuild.

Concrete action taken: built out `internal/app/database/database.go` (a stray, empty, user-referenced file found earlier) as the connection-lifecycle bootstrap — `Connect(ctx, databaseURL) (*pgxpool.Pool, int, error)`, wrapping `postgres.NewPool` + `migrate.Run` — mirroring the role `internal/adapter/sqlite/db.go` plays in `zk-org/zk` (connection setup kept separate from the DAOs/repository that use it). `cmd/core` now calls `database.Connect` instead of wiring `postgres.NewPool`/`migrate.Run` directly. Also closed a real gap the advisor flagged ahead of Phase 7: `secondbrain.Store` had no way to list all profile rules (`get_profile` needs one) — added `ListProfileRules` to the interface, the postgres adapter, and both test fakes.

Verified: full unit + integration suite green, including a new `database.Connect` idempotency test run for real against local Postgres, and a live `go run ./cmd/core` confirming the refactored bootstrap still logs `migrations applied: 0` on a second run. No commits made.

## 2026-07-06 — Phase 6: Eval interface + Agent Loop primitive

`internal/eval/`: `Evaluator` interface + `SchemaEvaluator`, a deterministic default that validates `Observation.Output` against a JSON Schema via `gojsonschema` — already a transitive dependency via Genkit, so no new one added. `internal/agentloop/`: `Loop` (Plan→Act→Observe→Critique), bounded by `MaxIterations`, injecting the prior `Verdict.Reason` into the next `Act` call rather than blind-retrying; `EscalationHandler` fires exactly once, either immediately on a `RequiresHuman` verdict or once on exhaustion. `internal/agentloop/feedback/`: `SignalWriter` interface + `Recorder`, deliberately decoupled from `internal/secondbrain` (the design doc requires this — an adapter satisfying `SignalWriter` from a real `secondbrain.Store` is deferred to whichever phase wires the full app together, not this one).

**Scope correction, found before writing code:** the design doc's "ADK Go 2.0 durable pause/resume" doesn't exist — `google.golang.org/adk` tops out at v1.5.0. PLAN.md already anticipated this by explicitly deferring real ADK wiring to "Phase 6b," so this phase's `EscalationHandler` stays a plain interface/stub; no ADK import anywhere in this phase, and none was needed.

Verified: 6 exact-count tests on `Loop` (always-fails → exactly `MaxIterations` attempts + exactly 1 escalation; passes on attempt 2 → exactly 2 attempts + 0 escalations; retry-reason propagation; `RequiresHuman` escalates immediately regardless of remaining budget; zero/negative `MaxIterations` treated as 1; a hard `Act` error stops immediately without escalating) — this is the concrete regression test the design doc calls for against unbounded self-correction. `SchemaEvaluator` and `feedback.Recorder` each have their own passing unit tests. Fully offline — no DB, no LLM, no ADK in any test path. No commits made.

**Housekeeping note:** found an empty, untracked, unreferenced `internal/app/database/database.go` of unexplained origin during this phase's verification pass — left in place per the session's permission policy (not something explicitly named for removal); flagged to the user rather than silently deleted.

## 2026-07-06 — Phase 5: Genkit app + Dotprompt + built-in middleware

`internal/genkit/`: `Init` wires the `googlegenai.GoogleAI` plugin, the built-in `middleware.Middleware` plugin (Retry/Fallback/ToolApproval/Skills), Dotprompt loading via `genkit.WithPromptDir`, and registers `DefinePlaceholderFlow` — a wiring-proof flow only, no real chat logic. `prompts/placeholder.prompt` demonstrates input/output JSON schema. `skills/README.md` states the `SKILL.md`-per-directory convention (corrected to point at sub-project 3, matching the repo-root `SKILL.md`'s own correction).

**Live-verification finding:** read `ai/generate.go`'s `buildModelChain` and `plugins/middleware/fallback.go`'s `wrapModel` directly to confirm `ai.WithUse(Fallback, Retry)` composition order, rather than trusting the design doc's prose paraphrase. Confirmed: Fallback is outermost, calls `next()` (Retry-wrapped primary) once; Retry retries the primary `MaxRetries` times; if still failing, Fallback loops its own model list calling each candidate directly (exactly once each, not retried). Two tests reproduce this against real `middleware.Fallback`/`middleware.Retry` implementations with local fake models (no live API calls) and both pass, including the exact call-count assertions (primary called `MaxRetries+1` times, fallback called once). **Not verified live:** `TestPlaceholderFlow_RoundTrip` (`-tags integration`) needs `GEMINI_API_KEY`, unset here — confirmed it skips cleanly. No commits made.

## 2026-07-06 — Phase 4: Embedding pipeline

`internal/embedding/`: `GenkitEmbedder` wraps Genkit's `gemini-embedding-2` via the `googlegenai` plugin; `Pool` is a fixed-N-worker bounded pool (`errgroup.Group`, context-cancel, timeout-bounded shutdown wait). **Live-verification finding (resolves design doc Open Item #1):** `gemini-embedding-2`'s native output is 3072 dimensions, not 768 — confirmed by reading the live SDK source (`plugins/googlegenai/models.go`), not assumed. Kept the existing 768-dim schema unchanged by requesting Matryoshka-truncated output via `genai.EmbedContentConfig.OutputDimensionality`. Pool concurrency-bound, timeout, and goroutine-leak tests pass, including under `-race`. The one test that calls the live Gemini API (`TestGenkitEmbedder_Smoke`, `-tags integration`) is gated on `GEMINI_API_KEY`, which is not set in this environment — confirmed it skips cleanly rather than silently passing something else; genuinely unverified live here. No commits made.

## 2026-07-07 — Chat CoT streaming + all five "Soon" features built (iteration 10)

Chat: `internal/orchestrator.DefineChatFlows` (streaming flow with typed reasoning/text `ChatChunk`s, `thinkingConfig.includeThoughts` enabled — Gemini chain-of-thought surfaces as real `ai.PartReasoning`), `POST /chat/stream` SSE route, frontend streamed CoT panel with spinning brand mark and a working stop button (`AbortController` → request-context cancellation). Genkit tool interrupts checked per the user's docs pointer: chat has zero tools, nothing to resolve yet; flagged for whenever tools land.

Five new features, replacing the sidebar's disabled "Soon" group entirely: Task Management (`migrations/0002_task.sql`, `internal/task`+postgres adapter, `/tasks` CRUD, board view), Knowledge Graph (`internal/graph`+postgres — Second Brain rows as nodes, pgvector cosine edges, `GET /graph`, hand-rolled SVG force layout), Skill + Prompt Catalogs (`internal/library` reading `skills/`+`prompts/` with a listing-based traversal guard, four GET routes, searchable card views), Voice Assistant (push-to-talk `MediaRecorder` → Gemini multimodal audio through `/chat`, `speechSynthesis` replies, `NSMicrophoneUsageDescription` added to both plists). `Makefile desktop-build` now bundles `skills/`. Design doc: `docs/superpowers/specs/2026-07-07-chat-streaming-and-domain-features-design.md`.

Verified: full Go suite (+`-race` on touched packages), Vite build, `make desktop-build`, and a live smoke against the real local Postgres — migration 0002 applied, `/tasks` CRUD round-trip, `/skills`/`/prompts`/`/graph` all returning real data, `/chat/stream` emitting sanitized SSE errors with a placeholder key. Unverified without a real key/mic/display: actual Gemini CoT streaming, mic capture, view pixels. No commits made.
