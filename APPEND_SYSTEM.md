# APPEND_SYSTEM.md — standing project directives

Append-only log of project-specific working rules that supplement [`AGENTS.md`](AGENTS.md), so contributors/agents see accumulated conventions in one place instead of digging through chat history. Add to the bottom; never delete or rewrite an existing entry — if a rule is superseded, add a new entry noting the supersession.

## 2026-07-06 — Always write `.md` for plans/changes
Every plan, design, or non-trivial change gets a durable `.md` file — never left only in chat/conversation output. Design docs go to `docs/superpowers/specs/`; execution plans go in `PLAN.md`-style files; this documentation suite is itself an application of the rule.

## 2026-07-06 — Zero-trust on external facts
Before asserting an SDK method signature, model identifier, library version, or third-party API behavior — in code or in docs — verify it against a live, current source. Do not extrapolate from a blog post, an older doc example, or memory of a prior version. See design doc Section 10 ("Open items requiring verification") for the standing list of currently-unverified assumptions that must be resolved before the code depending on them is written.

## 2026-07-06 — No unbounded self-correction loops
Any agent self-correction / retry loop must have a hard iteration cap and an explicit escalation path (HITL) when that cap is hit. This applies to `internal/agentloop` and to any future loop built on top of it — an open `while(true)` retry is never acceptable, regardless of how unlikely the failure case seems.
