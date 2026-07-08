.PHONY: build run web fmt vet lint test test-integration test-race test-all check tidy clean \
	desktop-frontend desktop-build desktop-dev desktop-run desktop build-all configure-key

DATABASE_URL ?= postgres://$(shell whoami)@localhost:5432/agentic_desk

# CORE_API_URL is a manual override only — cmd/desktop auto-launches its
# own cmd/core child process on a free port by default (see
# cmd/desktop/corelauncher.go). Set this to point the GUI at a core
# you're already running/editing separately instead, which skips
# auto-launch entirely.
CORE_API_URL ?=

# WEB_PORT/WEB_CORE_PORT are the ports `make web` binds — separate from the
# default :8080/desktop-auto-launch ports so this can run alongside a
# packaged .app without colliding.
WEB_PORT ?= 5199
WEB_CORE_PORT ?= 9317
WEB_API_ADDR := 127.0.0.1:$(WEB_CORE_PORT)

## build: compile all packages
build:
	go build ./...

## run: run the core binary (requires GEMINI_API_KEY; DATABASE_URL is
## optional now — internal/config defaults to the same local trust-auth
## Postgres this Makefile's own DATABASE_URL var points at, via split
## DB_HOST/DB_PORT/DB_USER/DB_NAME env vars)
run:
	go run ./cmd/core

## web: run this app in a plain browser instead of the native Wails window —
## cmd/core (real backend, requires GEMINI_API_KEY) + the Vite dev server
## together, wired via main.js's DEV-only `window.go` shim (stripped from
## production builds). Kills whatever's already bound to WEB_CORE_PORT/
## WEB_PORT first, and again on exit/Ctrl-C — by PORT, not by process-name
## pattern. This repo's own prior docs said to match "exe/core"; live-
## verified while building this target that the real `go run` child binary
## path on the current Go toolchain is ".../<hash>-d/core" — no "exe" in
## it at all, so that pattern was a silent no-op and left a real orphaned
## "core" binary squatting the port on the very next run (caught by
## actually re-running this target twice in a row, not assumed correct).
## `trap 'kill 0'` (process-group kill) was tried first and also proved
## unreliable across invocation contexts — kill-by-port is what's actually
## verified working here.
web:
	@lsof -tiTCP:$(WEB_CORE_PORT) -sTCP:LISTEN 2>/dev/null | xargs -r kill -9
	@lsof -tiTCP:$(WEB_PORT) -sTCP:LISTEN 2>/dev/null | xargs -r kill -9
	@trap '\
		lsof -tiTCP:$(WEB_CORE_PORT) -sTCP:LISTEN 2>/dev/null | xargs -r kill -9; \
		lsof -tiTCP:$(WEB_PORT) -sTCP:LISTEN 2>/dev/null | xargs -r kill -9 \
	' EXIT INT TERM; \
	(API_ADDR=$(WEB_API_ADDR) go run ./cmd/core) & \
	(cd cmd/desktop/frontend && npm run dev -- --port $(WEB_PORT) --strictPort) & \
	wait

## fmt: check formatting (fails if any file needs gofmt)
fmt:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

## vet: run go vet
vet:
	go vet ./...

## lint: fmt + vet together
lint: fmt vet

## test: unit tests only (no external DB/API needed, always safe to run)
test:
	go test ./...

## test-integration: unit + integration tests against a real local Postgres.
## Live-API tests (embedding, genkit flow) additionally need GEMINI_API_KEY
## and will skip cleanly without it.
test-integration:
	DATABASE_URL="$(DATABASE_URL)" go test -tags integration ./...

## test-race: unit tests with the race detector
test-race:
	go test -race ./...

## test-all: everything — fmt, vet, unit, integration, race
test-all: lint test-integration test-race

## check: the pre-commit-equivalent gate — build + lint + unit tests
check: build lint test

## tidy: sync go.mod/go.sum with actual imports
tidy:
	go mod tidy

## desktop-frontend: install deps + build the Vue frontend standalone —
## catches frontend-only errors (Vite/import issues) before a full,
## slower wails build; `desktop-build` runs this internally too.
desktop-frontend:
	cd cmd/desktop/frontend && npm install && npm run build

## desktop-build: package the Wails desktop app (.app on macOS) —
## runs the frontend build itself, desktop-frontend isn't a prerequisite.
## Also builds cmd/core as a sibling "agentic-desk-core" binary inside
## the .app bundle (Contents/MacOS/), plus copies prompts/ next to it
## (cmd/core's Genkit prompt loader needs it on disk, and CWD inside the
## bundle isn't the repo root) — so cmd/desktop can auto-launch core on
## startup (see cmd/desktop/corelauncher.go) instead of requiring the
## user to run it separately first.
desktop-build:
	cd cmd/desktop && wails build
	go build -o cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/agentic-desk-core ./cmd/core
	rm -rf cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/prompts
	cp -R prompts cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/prompts
	rm -rf cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/skills
	cp -R skills cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/skills

## desktop-dev: run the desktop app in Wails' hot-reload dev mode against
## a real window (requires a display — not usable in a headless sandbox).
desktop-dev:
	cd cmd/desktop && wails dev

## desktop-run: launch the already-packaged .app directly (also works via
## `open cmd/desktop/build/bin/agentic-desk.app` or double-clicking in
## Finder now — cmd/desktop auto-launches its own bundled core, no env
## var required). CORE_API_URL here is only for pointing at a
## separately-run core instead. Run `make desktop-build` first if you
## haven't, or after any code change.
desktop-run:
	CORE_API_URL="$(CORE_API_URL)" ./cmd/desktop/build/bin/agentic-desk.app/Contents/MacOS/agentic-desk

## desktop: build + launch the desktop app in one step
desktop: desktop-build desktop-run

## build-all: everything buildable in this repo — Go binaries and the
## packaged desktop app
build-all: build desktop-build

## configure-key: persist GEMINI_API_KEY for the packaged desktop app so
## a real double-click launch works with no terminal/export needed —
## Finder-launched processes never inherit shell env vars, so cmd/desktop
## reads this file as a fallback (see cmd/desktop/secrets.go). A real
## exported GEMINI_API_KEY always wins over this file (dev debugging via
## `make run`/an exported var is unaffected). Usage:
##   make configure-key KEY=your-real-gemini-api-key
configure-key:
	@test -n "$(KEY)" || (echo "usage: make configure-key KEY=your-gemini-api-key" && exit 1)
	@install -d -m 700 "$(HOME)/Library/Application Support/agentic-desk"
	@umask 077 && printf 'GEMINI_API_KEY=%s\n' "$(KEY)" > "$(HOME)/Library/Application Support/agentic-desk/.env"
	@chmod 600 "$(HOME)/Library/Application Support/agentic-desk/.env"
	@echo "Saved. Relaunch agentic-desk.app directly (no export needed)."

## clean: remove build artifacts
clean:
	go clean ./...
	rm -rf cmd/desktop/build/bin
