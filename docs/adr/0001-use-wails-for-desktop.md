# ADR-1: Use Wails v2 for Desktop Shell

**Status:** Accepted

**Date:** 2026-07-06

## Context

The application needs a native macOS desktop shell that can:
- Host a Vue.js frontend with full DOM/browser API access
- Run a Go backend process as a child
- Package into a distributable `.app` bundle
- Provide native OS integration (window management, file system, menus)

**Options considered:**
1. **Electron** — heavyweight, requires Node.js runtime, large bundle size (~150MB+)
2. **Tauri** — Rust-based, requires Rust compiler in toolchain
3. **Wails v2** — Go-based, native macOS WebView, Go ↔ JS bindings

## Decision

Use **Wails v2** with the native macOS WebView (WKWebView).

Wails allows:
- Go backend bundled in the same binary
- Vue.js frontend served from embedded filesystem (no separate server)
- Direct Go ↔ JS bindings via generated `wailsjs/go` modules
- Small bundle size (~15MB)

## Consequences

**Positive:**
- Single-language toolchain (Go + standard JS/TS frontend)
- Auto-launch Go child process (`cmd/core`) on app startup
- Embedded assets via `//go:embed`
- Minimal bundle size

**Negative:**
- WKWebView has limitations: no Web Speech API, some CSS quirks
- Wails ecosystem smaller than Electron
- macOS-only (cross-platform deferred)

## Compliance

All desktop builds must use `wails build`. Frontend development can use `make web` for browser-only testing.
