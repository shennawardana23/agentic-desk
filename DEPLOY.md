# DEPLOY.md — local manual build & deploy (desktop app)

**No CI/CD pipeline exists for this repo.** There is no `.github/workflows/`, no other CI config, and no automated release process anywhere in this codebase (checked — nothing to find). Every build described below is a manual step run by hand on a developer machine. If you came here looking for an automated pipeline, there isn't one yet — this doc covers what actually exists: how to build and run the two binaries (`cmd/core`, `cmd/desktop`) locally.

Two separate binaries, by design (`cmd/desktop/app.go`'s own doc comment): the desktop app is a **GUI shell only** — it holds no database connection or embedder, and talks to `cmd/core`'s HTTP+WS API over `fetch`/`WebSocket`, same as any other client would. **`cmd/desktop` now auto-launches its own `cmd/core` child process on startup** (see `cmd/desktop/corelauncher.go`) — picking a free loopback port itself, so you no longer need to start `cmd/core` by hand or coordinate ports between the two. `DATABASE_URL`/`GEMINI_API_KEY` still have to be set in whatever environment launches the desktop app, since core needs them regardless of who starts it.

## Prerequisites

- Go (matching `go.mod`'s `go 1.25` line — this repo pins to whatever's actually installed, not a specific patch version)
- Node.js + npm (for `cmd/desktop/frontend`)
- [Wails CLI v2](https://wails.io) (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`) — confirm with `wails doctor`
- Local Postgres with the `pgvector` extension (see `TESTING.md`'s "Local Postgres setup" section)
- `DATABASE_URL` and `GEMINI_API_KEY` env vars — `internal/config` fails fast and names whichever is missing

## 1. Build and run `cmd/core` (the API server)

```sh
export DATABASE_URL="postgres://$(whoami)@localhost:5432/agentic_desk"
export GEMINI_API_KEY="..."   # real key needed for embedding/genkit routes; core still starts without one, those routes just 500

go run ./cmd/core
```

Runs migrations automatically on connect (`internal/app/database.Connect` → `migrate.Run`), then serves the Phase 9 HTTP+WS API on `internal/config.Config.APIAddr` (defaults to `:8080` — confirm the port it actually logs, since something else may already be squatting the default; see "Common failure" below).

To build a standalone binary instead of `go run`:
```sh
go build -o bin/core ./cmd/core
./bin/core
```

## 2. Build and run `cmd/desktop` (the GUI)

```sh
make desktop-build
```

Runs `wails build` (compiles the frontend, packages a real signed `.app` at `cmd/desktop/build/bin/agentic-desk.app`), **plus** builds `cmd/core` as a sibling `agentic-desk-core` binary inside the bundle (`Contents/MacOS/`) and copies `prompts/` next to it. Both are required — `cmd/desktop` auto-launches that sibling binary as its own child process on startup (see `cmd/desktop/corelauncher.go`), so a real end user can just double-click the `.app`, no terminal or second binary required.

```sh
open cmd/desktop/build/bin/agentic-desk.app
```

`DATABASE_URL`/`GEMINI_API_KEY` still need to be set in whatever launches the desktop app — `open`/double-clicking in Finder never inherits your shell's env vars, so export them in a `launchd`/login-item context, or launch the raw binary directly from a terminal that has them set:
```sh
DATABASE_URL="postgres://$(whoami)@localhost:5432/agentic_desk" GEMINI_API_KEY="..." \
  ./cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/agentic-desk
```

`CORE_API_URL` is now a manual override only — set it to point the GUI at a `cmd/core` you're already running/editing separately (e.g. during active core development), which skips auto-launch entirely:
```sh
CORE_API_URL=http://localhost:9020 ./cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/agentic-desk
```

For interactive frontend-only iteration without a full `wails build` each time:
```sh
cd cmd/desktop
wails dev   # hot-reloads frontend/src changes against a real window; auto-launches core via `go run ./cmd/core`
```

## Former failure mode, now fixed: `TypeError: Load failed` on every view

Previously, `cmd/desktop` defaulted to a hardcoded `:8080` if `CORE_API_URL` was unset, requiring `cmd/core` to already be running on exactly that port — any mismatch (or nothing running) showed a generic `Load failed` on every view (`Profile Rules`/`Memory Search`/`Project Context`) and, once Chat shipped, a chat error too. `cmd/desktop` now launches and owns `cmd/core` itself on a dynamically chosen free port, so this class of bug no longer occurs when running from a correctly-built bundle (see "Known gap" below for the fix's own edge cases). If you still see `Load failed`, check `CoreStartupError()`'s message (surfaced into the Chat view's error banner) — it means the auto-launched core itself failed (e.g. missing `DATABASE_URL`), not a port mismatch.

## Known gap

Auto-launch still requires `DATABASE_URL`/`GEMINI_API_KEY` to be present in the environment that starts the desktop app — there's no settings UI or `.env` file support yet, so a truly zero-config double-click (no terminal, no exported env vars anywhere) will still fail fast with a clear `CoreStartupError()` message rather than silently. Building that config UI is out of scope here; not attempted speculatively.

## Rebuilding after a code change

Frontend-only change (`cmd/desktop/frontend/src/**`):
```sh
cd cmd/desktop/frontend && npm run build   # verify Vite compiles clean first
cd .. && wails build                        # then repackage the .app
```

Go-only change (`cmd/desktop/*.go`, `cmd/core/**`, `internal/**`):
```sh
go build ./... && go vet ./... && gofmt -l .   # verify the whole module first
make desktop-build   # not `cd cmd/desktop && wails build` alone — that skips rebuilding the bundled agentic-desk-core sibling binary
```

Always rebuild+relaunch after **any** change on either side before treating a bug as fixed — a stale running `.app` will keep showing the old behavior.
