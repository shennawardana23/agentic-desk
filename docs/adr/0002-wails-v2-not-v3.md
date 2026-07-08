# ADR-0002: Wails v2, not v3

Status: Accepted — 2026-07-06

## Context

Wails v3 offers multi-window support, a service-pattern DI model, and future mobile compile targets, but is still alpha as of 2026 (verified against wails.io's v3 docs). The org convention is to use the latest *stable* framework version, not the latest version regardless of maturity.

## Decision

Build the desktop shell (`cmd/desktop`) on Wails **v2**.

## Consequences

- Foregoes v3's multi-window API and mobile targets for now — acceptable, since the current design only needs a single window.
- Avoids building on an API surface that may still change before v3's stable release.
- Revisit this ADR (superseding it, not editing it) once Wails v3 reaches a stable/GA release, if its features become relevant (e.g. multi-window becomes needed).
