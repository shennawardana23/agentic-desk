# Explanation

The "why" behind this platform's design — the Diátaxis quadrant that doesn't require running code to write, because it's about reasoning, not usage.

## Why the platform is decomposed into ~9 ordered sub-projects

The original ask described chat agent, voice agent, second-brain memory, multi-agent A2A orchestration, tool-agents (search/fetch/image-gen/review), flow visualization, tracing/evals, and a full docs suite — multiple independent subsystems, not one project. A single spec covering all of it would have guessed at details that depend on earlier subsystems actually existing (e.g. multi-agent orchestration needs the Second Brain and Agent Loop primitive to exist first). Decomposing into ordered, dependency-respecting sub-projects means each spec is grounded in what's actually built so far, not speculative about what a later sub-project will look like. See [`DESIGN.md`](../../DESIGN.md) for the current list and status.

## Why a headless core process instead of a monolith desktop app

The stated goal is that the Second Brain must be reachable by external coding agents "any time" — not gated on the desktop GUI happening to be open. A monolith design (everything inside the Wails process) would tie the MCP server's lifetime to the GUI window's lifetime, directly contradicting that goal. The headless-core architecture (`cmd/core` + thin `cmd/desktop` client) was chosen specifically to avoid this. See [`docs/adr/0005-headless-core-process-architecture.md`](../adr/0005-headless-core-process-architecture.md).

## Why the profile importer is deterministic, not an LLM

An LLM summarizing "what it thinks your rules mean" introduces exactly the kind of hallucination risk this project was explicitly asked to avoid. A regex/heading-based parser can only extract what's literally present in the source files, with an exact traceable location for every extracted rule — a narrower but fully auditable capability. See [`docs/adr/0006-deterministic-profile-importer.md`](../adr/0006-deterministic-profile-importer.md).

## Why the Agent Loop's self-correction is bounded

An agent that retries indefinitely against its own Critique step is a resilience hazard, not a resilience feature — it can burn CPU/API cost without limit and never surface a failure to the person who could actually fix it. Capping iterations and escalating to a human on exhaustion (using ADK Go 2.0's durable pause/resume, not a blocking wait) makes failure visible and boundable instead of silent and open-ended. See [`docs/adr/0007-bounded-agent-loop-honest-rlhf.md`](../adr/0007-bounded-agent-loop-honest-rlhf.md).

## Why "RLHF Annotator" is described the way it is

Calling a prompt/context-level feedback loop "RLHF" without qualification would overclaim a training mechanism this platform doesn't have (no model weight access, hosted API only). The chosen framing — human feedback captured as a retrievable preference signal in the Second Brain — does the same practical job of "the system gets better-informed over time from your corrections," without misrepresenting how. This is a professional-honesty requirement, not a marketing choice.
