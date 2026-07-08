package main

import (
	"context"
	"os"
)

// App is Wails' bound backend — its exported methods become callable
// from the frontend's JS via the generated wailsjs/go/main bindings.
// It deliberately holds no secondbrain.Store/embedding.Embedder
// directly: the frontend talks to cmd/core's HTTP+WS API over
// plain fetch/WebSocket, the same way any other API client
// would, rather than duplicating a second in-process backend here.
type App struct {
	ctx       context.Context
	core      *coreProcess
	coreReady chan struct{}
}

// NewApp constructs an App ready for Wails to bind.
func NewApp() *App {
	return &App{}
}

// startup saves the Wails runtime context and launches cmd/core as a
// managed child process (see corelauncher.go) — a single-user desktop
// app has no reason to require the user to start a second binary
// themselves first. CORE_API_URL remains a manual escape hatch: set it
// to point the GUI at a core you're already running/editing separately
// (e.g. during `wails dev`), which skips auto-launch entirely.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.coreReady = make(chan struct{})
	if override := os.Getenv("CORE_API_URL"); override != "" {
		close(a.coreReady)
		return
	}
	go func() {
		a.core = startCore(ctx)
		close(a.coreReady)
	}()
}

// shutdown kills the auto-launched core process, if any, so closing the
// window doesn't leave an orphaned server listening.
func (a *App) shutdown(ctx context.Context) {
	a.core.stop()
}

// CoreAPIURL returns the base URL of cmd/core's HTTP+WS API. Blocks
// until the auto-launched core (started in startup) has either become
// ready or given up — the frontend already awaits this call before
// mounting (see main.js), so there is nothing useful to show before it
// resolves anyway.
func (a *App) CoreAPIURL() string {
	if override := os.Getenv("CORE_API_URL"); override != "" {
		return override
	}
	<-a.coreReady
	if a.core != nil && a.core.err == nil {
		return a.core.addr
	}
	return "http://localhost:8080"
}

// CoreStartupError reports why the auto-launched core isn't reachable,
// if it isn't — empty string means core started fine (or CORE_API_URL
// override is in effect). The frontend surfaces this so a real cause
// (e.g. missing DATABASE_URL) is visible instead of a generic fetch
// failure on every view.
func (a *App) CoreStartupError() string {
	<-a.coreReady
	if a.core != nil && a.core.err != nil {
		return a.core.err.Error()
	}
	return ""
}
