# ADR-0007: Bounded Agent Loop + honest (non-literal) RLHF framing

Status: Accepted — 2026-07-06

## Context

The platform needs a self-correcting agent loop with resilience against runaway retries, and a mechanism for incorporating human feedback into future agent behavior. The user asked for an "RLHF Annotator." Literal reinforcement learning from human feedback requires training on model weights the platform does not own — Agentic Desk calls hosted models (Gemini) over an API with no gradient/weight access.

## Decision

1. The Agent Loop (`internal/agentloop`) runs Plan→Act→Observe→Critique with self-correction capped at a configurable `maxIterations`. On exhaustion, or when a Verdict is flagged `requiresHuman`, the loop escalates to a human via durable pause/resume (ADK Go 2.0) rather than blocking a goroutine indefinitely.
2. Human decisions/corrections at escalation points, and explicit approve/reject feedback on committed results, are captured by a Feedback Annotator (`internal/agentloop/feedback`) and written to the Second Brain as a `FeedbackSignal` — a retrievable preference signal that biases future prompts/context, not a training signal that updates model weights.
3. This mechanism is documented and communicated as exactly what it is — a feedback-informed context loop — never described as literal gradient-based RLHF in code, docs, or to the user.

## Consequences

- Self-correction cannot run away: the cap is a hard requirement, verified by a dedicated unit test asserting HITL escalation fires exactly once at exhaustion (see `PLAN.md` Phase 6).
- Provider-level retry (rate limits, transport errors) stays a separate, orthogonal resilience layer inside Genkit's middleware — the Agent Loop's Critique step never needs to know a provider exists.
- The "RLHF" framing is deliberately narrower than the term usually implies; any future documentation, UI copy, or user-facing description of this feature must preserve that honesty rather than overclaim a training mechanism that isn't built.
