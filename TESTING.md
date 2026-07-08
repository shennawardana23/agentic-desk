# Testing guide

## Quick reference

| Command | What it runs | External deps needed |
|---|---|---|
| `make test` | Unit tests only | None |
| `make test-integration` | Unit + integration tests | Local Postgres+pgvector (`DATABASE_URL`) |
| `make test-race` | Unit tests with `-race` | None |
| `make test-all` | fmt + vet + integration + race | Local Postgres+pgvector |
| `make check` | build + lint + unit tests (pre-commit gate) | None |
| `make lint` | `gofmt -l` + `go vet` | None |
| `make build` | `go build ./...` | None |
| `make run` | Runs `cmd/core` | Postgres + `GEMINI_API_KEY` |

## Two test tiers, by design

This repo has exactly two tiers, distinguished by a Go build tag — no third-party test framework, per this project's own YAGNI stance (see `APPEND_SYSTEM.md`).

**1. Unit tests (default, `go test ./...`)** — no external dependencies, always run, always safe. Every package that talks to Postgres or Gemini has an in-memory or local fake standing in for it (e.g. `internal/secondbrain`'s `memStore`, `internal/mcp/server`'s `fakeStore`/`fakeEmbedder`).

**2. Integration tests (`-tags integration`)** — talk to a *real* local Postgres. Every file is named `*_integration_test.go` and starts with:

```go
//go:build integration
```

and every test function inside skips itself with `t.Skip(...)` if the environment variable it needs isn't set — they never fail loudly just because you forgot to set up Postgres, but they also never silently report success for something they didn't actually run. If you're verifying a change to anything DB-backed, **export `DATABASE_URL` and confirm the test actually executes** (no `SKIP` in the output) — a skipped test looks identical to a passing one in a quick glance at `PASS`, which is the exact mistake this project's own `SESSION_HANDOFF.md` flags as a trap from an earlier session.

## Environment variables

- `DATABASE_URL` — e.g. `postgres://$(whoami)@localhost:5432/agentic_desk`. Required by every `internal/secondbrain/postgres`, `internal/migrate`, and `internal/app/database` integration test.
- `GEMINI_API_KEY` — required only by tests that make a real call to the Gemini API: `internal/embedding`'s `TestGenkitEmbedder_Smoke` and `internal/genkit`'s `TestPlaceholderFlow_RoundTrip`. Both skip cleanly without it. **Nothing else in this repo needs it** — the MCP server's round-trip test (`internal/mcp/server`), for example, is fully offline because it fakes the embedder.

## Notable test techniques used in this repo

- **Exact-count regression tests** (`internal/agentloop`): a bounded retry loop is only correct if it retries *exactly* N times and escalates *exactly* once — tests assert precise call counts, not just "eventually stops."
- **Goroutine-leak checks** (`internal/embedding`): `runtime.NumGoroutine()` diffed before/after repeated pool runs, no `goleak` dependency needed.
- **Real protocol round-trip, no live server** (`internal/mcp/server`): a real `github.com/modelcontextprotocol/go-sdk` client talks to the real Genkit `plugins/mcp` server over actual `os.Pipe()`-backed stdio — genuine wire-protocol coverage without a live network endpoint.
- **Composition-order verification against real middleware** (`internal/genkit`): `Fallback`+`Retry` interaction is proven with local fake models registered via `genkit.DefineModel`, not by reading the docs and trusting the description.

## Local Postgres setup (if you don't already have it)

```sh
brew install postgresql pgvector   # macOS; see pgvector docs for other platforms
createdb agentic_desk
psql agentic_desk -c 'CREATE EXTENSION IF NOT EXISTS vector;'
export DATABASE_URL="postgres://$(whoami)@localhost:5432/agentic_desk"
make test-integration
```

## Debugging the desktop app (`cmd/desktop`)

`cmd/desktop` is a **GUI shell only** — no `secondbrain.Store`/`embedding.Embedder`, no DB connection. It talks to `cmd/core`'s HTTP+WS API over plain `fetch`, the same way any other API client would (`cmd/desktop/app.go`'s own doc comment explains why). **As of the auto-launch fix, `cmd/desktop` also launches and owns `cmd/core` itself** (see `cmd/desktop/corelauncher.go`) — it picks a free loopback port, spawns the bundled sibling `agentic-desk-core` binary (or `go run ./cmd/core` in dev mode) against it, and blocks its `CoreAPIURL()` binding until that process answers or gives up. This closes the port-mismatch class of bug below by construction (no more `:8080` fallback colliding with something unrelated), but leaves two new failure modes worth knowing how to read.

**1. Auto-launched core failed to start.**

Symptom: Chat's error banner shows `Core failed to start: ...` (this is `CoreStartupError()`, surfaced by `stores/core.js`'s `init()` — see `App.vue`/`ChatView.vue`).

Why: the child `cmd/core` process either exited immediately (e.g. `DATABASE_URL`/`GEMINI_API_KEY` missing — `internal/config` fails fast, and `corelauncher.go` captures that stderr into the error message) or never answered `/profile` within 15s. Fix by making sure whatever *launches the desktop app itself* has `DATABASE_URL`/`GEMINI_API_KEY` set — auto-launch inherits the desktop app's own environment, it can't invent credentials that aren't there.

Diagnose directly:
```sh
ps aux | grep agentic-desk-core                              # is a child core process even running?
lsof -nP -iTCP -sTCP:LISTEN | grep agentic-desk-core          # what port did it actually get (it's dynamic now, not :8080)
```

**2. A genuine backend or CORS bug**, independent of auto-launch — same as before: check `NewRouter`'s CORS middleware is still registered and the route exists, by curling the port the log line `api listening on <addr>` reports:
```sh
curl -i http://<addr>/profile -H "Origin: wails://wails.localhost"
```

**Manually overriding auto-launch** (pointing the GUI at a `cmd/core` you're running/editing separately, e.g. active core development): set `CORE_API_URL` before launching the GUI binary directly — not via `open`, which strips env vars, same caveat as always:
```sh
CORE_API_URL=http://localhost:9020 ./cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/agentic-desk
```
Setting `CORE_API_URL` skips auto-launch entirely (see `app.go`'s `startup`) — no child process gets spawned.

**Superseded, kept for history:** the old failure mode here was the GUI silently defaulting to `:8080` and getting either "nothing's listening" or "the wrong process is listening" — root-caused via `lsof`/`curl` in earlier sessions (see `SESSION_HANDOFF.md`'s iteration-4 entry) and now fixed by owning the whole lifecycle instead of guessing a fixed port.

Migrations run automatically on first connect (`internal/app/database.Connect`) — no separate migration step needed.
