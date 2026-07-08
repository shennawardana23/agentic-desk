# Multi-provider AI model layer + ADK bridge — design

**Date:** 2026-07-07
**Status:** Approved, not yet planned/implemented.
**Depends on:** `internal/orchestrator` (formerly `internal/genkit` — renamed this session), Phase 5's Fallback/Retry middleware composition.

## Why

`internal/orchestrator/app.go` currently wires exactly one provider — `googlegenai.GoogleAI` (Gemini) — with two hardcoded model consts (`PrimaryModel`, `FallbackModel`) and a single-model `middleware.Fallback`. The user wants Sarza's chat (and any future flow) able to fall back across *providers*, not just across two Gemini model tiers, and pointed at a working reference implementation (`archpublicwebsite-agentic/internal/model/*`, `internal/middleware/fallback.go`) as the shape to draw from.

## Reference repo — what's reusable, what isn't

The reference is built on **ADK** (`google.golang.org/adk`'s `model.LLM` interface), which agentic-desk has never adopted (`PLAN.md` defers it to "Phase 6b"). Its `oaibridge` package exists *only* to translate Genkit `ai.Model` into ADK's `model.LLM` shape for ADK agents to consume — agentic-desk's chat flow calls `genkit.Generate`/`ai.WithModel` directly and needs no such translation to *use* a model.

Reusable near-verbatim: `catalog` package (provider/model registry, zero framework dependency).
Reusable as a *ported concept*, not code: `chain.Build`'s env-var auto-discovery ("set the API key, provider joins the chain, no code change"), and the general shape of per-provider `Config`/`ConfigFromEnv`/`NewModel` packages.
Not reused as-is: `oaibridge`'s ADK↔Genkit type translation (ADK-specific), `failover`'s circuit breaker/cooldown tracking (explicitly deferred, see below).

## Scope (confirmed with the user)

**Providers, v1:**
- Gemini (existing, `googlegenai` plugin) — primary + fallback tier, unchanged mechanism.
- Groq, GitHub Models, OpenRouter, NVIDIA NIM, OpenCode, HuggingFace, DeepSeek — via Genkit's `compat_oai.OpenAICompatible` plugin (all seven expose an OpenAI-compatible `/v1/chat/completions` endpoint; DeepSeek is OAI-compatible, verified before committing to this list rather than assumed).
- Anthropic, Ollama — via Genkit's own **native** plugins. Verified present in the pinned `github.com/firebase/genkit/go@v1.10.0` module (`plugins/anthropic`, `plugins/ollama` both exist for real — checked the actual module cache, not the docs alone).
- One generic/custom OpenAI-compatible provider slot, config purely from env vars (name/base URL/API key/model) — covers "any provider using the OpenAI-compatible shape" without a new code package per future provider.

**Explicitly dropped from v1:** AWS Bedrock. Verified: no native Genkit Go plugin and no OpenAI-compatible endpoint exist for it in the pinned SDK version — wiring it would mean hand-writing a custom `ai.Model` backed by the AWS SDK directly, real extra engineering, not a config entry like the others. Revisit as its own scoped task if actually needed later.

**Failover mechanism:** Genkit's official `middleware.Fallback` (already proven in this repo since Phase 5 — the exact "Fallback outer, Retry inner" composition order was verified against live SDK source then). No circuit breaker, no cooldown-window tracking — that sophistication in the reference's `failover` package is a real, deliberate deferral, not an oversight. Genkit's Fallback already treats `core.NOT_FOUND`/`RESOURCE_EXHAUSTED`/`UNAVAILABLE`/etc. as retryable-move-to-next-model by default, which covers "the configured model got deprecated/removed" and "provider rate-limited us" without any new state to build or test.

**ADK bridge:** built now, as an internal second phase of this same feature, but **not wired into the existing chat flow** — the chat flow keeps calling `genkit.Generate` directly and needs zero ADK. This is forward-looking plumbing for the still-deferred Phase 6b agent runtime: a new `internal/adkbridge` package translating any Genkit `ai.Model` into ADK's `model.LLM`, pulling in `google.golang.org/adk` as a new direct dependency, unit-tested standalone against a fake `ai.Model`. Nothing in `cmd/core` constructs or calls it yet.

## Package layout

```
internal/provider/
  catalog/           — ModelEntry, ProviderCatalog, Register/All/ForProvider (ported near-verbatim)
  oaicompat/         — shared helper: wraps compat_oai.OpenAICompatible.DefineModel, Retry middleware,
                        per-provider concurrency semaphore, Fallback-compatible error wrapping
                        (this repo's Genkit-native equivalent of the reference's oaibridge, minus
                        the ADK translation half — that lives in internal/adkbridge instead)
  groq/              — Config, ConfigFromEnv, NewModel(ctx) (ai.Model, error); registers its catalog
  githubmodels/       — same shape
  openrouter/         — same shape
  nvidia/             — same shape
  opencode/           — same shape
  huggingface/        — same shape
  deepseek/           — same shape
  anthropic/          — same shape, wraps plugins/anthropic (native) instead of oaicompat
  ollama/             — same shape, wraps plugins/ollama (native) instead of oaicompat
  custom/             — generic OpenAI-compatible provider from env vars, no catalog (unknown models)
  chain/              — Build(ctx) ([]ai.Model, error): Gemini primary+fallback first (unchanged),
                        then each other provider only if its own API-key env var is set

internal/adkbridge/  — NewModel(m ai.Model) (adkmodel.LLM, error)-shaped translation layer,
                        ported from the reference's proven request/response mapping code.
                        New google.golang.org/adk dependency. Standalone; nothing calls it yet.
```

## Model configurability & deprecation resilience

Three properties, working together, not one big new subsystem:

1. **Env-var override per provider** (`ConfigFromEnv()`, already the reference's own pattern): every provider's model ID is overridable without a rebuild. The one real gap — `internal/orchestrator/app.go`'s `PrimaryModel`/`FallbackModel` consts have no such override today — gets closed with `GEMINI_PRIMARY_MODEL`/`GEMINI_FALLBACK_MODEL` env vars, defaulting to the current consts, making the pattern uniform.
2. **Catalog as the actual default-resolution source, not decoration**: each provider's `ConfigFromEnv` resolves its default model via `catalog.ForProvider(name)`'s `Default: true` entry, instead of a separate hardcoded const duplicating the same information. Updating a deprecated model going forward is one catalog edit, in one place, not a hunt through provider files.
3. **Fallback already covers the failure case**: a deprecated/removed model 404s → `compat_oai`'s error wrapping maps that to `core.NOT_FOUND` → Genkit's official `middleware.Fallback` (default status list includes `NOT_FOUND`) automatically advances to the next model in `chain.Build`'s list. No new detection code needed — this is a property of the design above, verified against the reference's own `fallback.go`, which documents exact parity with Genkit's real defaults.

**Deliberately not built:** a startup-time live `/models`-endpoint check per provider (ping each provider's model-list endpoint before first real use, warn if the configured model is gone). Real idea, adds a startup network dependency and latency for a failure mode Fallback already degrades gracefully from at request time — deferred, documented so it isn't silently forgotten, not rejected outright.

## Wiring into the existing chat flow

`internal/orchestrator/chat.go`'s `DefineChatFlow` currently does:
```go
ai.WithModelName(PrimaryModel),
ai.WithUse(&middleware.Fallback{Models: []ai.ModelRef{googlegenai.ModelRef(FallbackModel, nil)}}),
```
This becomes: call `chain.Build(ctx)` once, at `Init` time (same place `PrimaryModel`/`FallbackModel` are read today). The first `ai.Model` in the returned slice becomes the primary passed to generation; the rest populate `middleware.Fallback.Models`. Same middleware, same proven composition order — just a longer, env-driven chain instead of a fixed two-tier one.

## Config / env vars

No new *required* env vars. Every new provider is opt-in, auto-discovered by its own API key var being set — matching the reference's "set the key, restart, it joins" UX exactly. `internal/config` does not need to know about any of them individually; `chain.Build` reads its own env vars directly.

## Testing

- Each provider package: unit test confirming `ConfigFromEnv()` reads the right vars and resolves the catalog default correctly; `NewModel` rejects missing required fields — adapted from the reference's own test files for each provider, not invented from scratch.
- `chain.Build`: table-driven test using `t.Setenv`, proving "only Gemini set → 1-2 models", "Gemini + OpenRouter set → 3 models, right order", "nothing set → error" (mirrors the reference's own pattern for this exact function).
- `internal/adkbridge`: unit test translating a fake `ai.Model`'s request/response round-trip — no live network, no live ADK agent (that consumption path is Phase 6b's job, still deferred).
- No new live-network tests beyond what this repo already has for Gemini (skip cleanly without the relevant API key set, same standing convention).

## Explicitly out of scope (so it isn't silently assumed later)

- Circuit breaker / cooldown-aware retry (confirmed: official `middleware.Fallback` only).
- AWS Bedrock (confirmed dropped from v1 — no plugin, no OAI-compat endpoint).
- Wiring `internal/adkbridge` into an actual running ADK agent (Phase 6b, still deferred by `PLAN.md`).
- Any model-picker UI (`catalog` exists for that future use per the reference's own doc comment; no `/model` picker exists in this app, none requested here).
- Live startup model-existence validation (see "deliberately not built" above).
