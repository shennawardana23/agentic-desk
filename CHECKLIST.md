# CHECKLIST.md

Verification checklist for this repo. Run top to bottom. Every item below has been run and passed at least once as of 2026-07-07 (see `CHANGELOG.md`/`SESSION_HANDOFF.md` for the detailed run history) — this file is the reusable procedure, not a one-time log.

## Prerequisites

- [ ] Go 1.25+ installed (`go version`)
- [ ] Local Postgres running, `agentic_desk` database exists, `pgvector` extension enabled
- [ ] `DATABASE_URL` exported, e.g. `export DATABASE_URL="postgres://$(whoami)@localhost:5432/agentic_desk"`
- [ ] `GEMINI_API_KEY` exported (a real key for live Gemini calls; any non-empty value works for wiring-only checks — the plugin validates lazily)
- [ ] For Phase 10 only: Node.js + npm installed, Wails CLI installed (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

## 1. Static checks

```bash
go build ./...     # exit 0, no output
go vet ./...        # exit 0, no output
gofmt -l .           # empty output = clean
```

## 2. Unit tests

```bash
go test ./... -count=1
```
All packages `ok`, zero `FAIL`. Packages with no test files (`cmd/core`, `cmd/desktop`, `internal/app/database`, `internal/migrate`, `migrations`) show `?` — expected, not a failure.

## 3. Race detector

```bash
go test -race ./...
```

## 4. Integration tests (real Postgres)

```bash
go test -tags integration ./...
```
Covers: migration idempotency, full CRUD round trips, embedding preserve/clear semantics on upsert, pagination limit/offset, `validateSearch` clamp.

## 5. One-shot full gate

```bash
make test-all
```
Runs lint (fmt+vet) → integration → race, in that order.

## 6. Live server smoke test

```bash
export API_ADDR=":8080"
go run ./cmd/core &
sleep 2
curl http://localhost:8080/health              # {"status":"ok"}
curl http://localhost:8080/profile              # real rows from the DB
kill %1
```

## 7. Desktop GUI build (needs a real display for step 7b — build itself doesn't)

```bash
cd cmd/desktop
wails build                                     # 7a: build-only, no display needed
open build/bin/agentic-desk.app                 # 7b: needs a real display; cmd/core (step 6) must be running first
```

## 8. GitHub tool (not live-tested by default — needs a real token)

```go
client, err := github.NewClient(os.Getenv("GITHUB_TOKEN"))
issues, err := client.ListIssues(ctx, "owner", "repo", "open")
```
Only fake-client unit tests exist in `internal/tools/github`. Ask for a `-tags integration` test against the real API if you want that covered too.

## Known, disclosed gaps (not failures — see SESSION_HANDOFF.md)

- [ ] YouTrack MCP client — not built, deployment type unanswered
- [ ] `wails dev` / manual click-through of the GUI — no display server in the dev sandbox this was built in
- [ ] `golang.org/x/sync`, `google.golang.org/genai` have newer versions available — not upgraded, deliberately deferred
