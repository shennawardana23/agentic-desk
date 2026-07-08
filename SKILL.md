# SKILL.md — agent capability model (planned, not implemented)

**Status: not yet implemented.** Planned for sub-project 3 (multi-agent orchestration). Nothing described here exists in code. This file documents intent so the eventual implementation has a stable target to build against, and so it isn't reinvented differently each time someone picks this up.

Note: this is unrelated to Claude Code's own "Superpowers" skill system used to develop *this* repo. Agentic Desk's own agents do not depend on, invoke, or reference that system — "skill" here means a capability an *Agentic Desk agent* declares and uses at runtime, a separate concept namespaced entirely within this project's own code.

## What a "skill" is, in this project — verified, not speculative

Corrected 2026-07-06: this was originally written as a speculative design. It is now grounded in a verified, authoritative local reference (`developing-genkit-go` skill, `references/middleware.md`), not a hypothesis.

Genkit Go ships a **built-in** `middleware.Skills` middleware. This is a real, shippable primitive — not something this project needs to design from scratch:

```go
ai.WithUse(&middleware.Skills{SkillPaths: []string{"skills"}}) // default: ["skills"]
```

A skill, in Genkit Go's own convention, is a **directory containing a file literally named `SKILL.md`**, optionally with YAML frontmatter (`name`, `description`) — e.g. `skills/github-search/SKILL.md`, `skills/image-gen/SKILL.md`. The middleware injects a system prompt listing available skills; the model calls a contributed `use_skill("name")` tool to pull a skill's full body into the conversation on demand. Heavier persona/capability instructions stay off the hot path until actually loaded — this is the concrete mechanism, not a paraphrase.

**Naming note — no collision despite the shared filename:** this document you're reading (repo-root `SKILL.md`) is a human/agent-facing explanation of the concept, one of this project's doc-suite entry points. It is structurally unrelated to the many per-capability `skills/<name>/SKILL.md` files Genkit actually loads — different paths, different purpose. Don't confuse "the doc that talks about skills" with "a skill."

The `agentskills.io` specification remains a useful cross-reference if skills ever need to be portable to a non-Genkit framework, but it is not required to use Genkit's built-in mechanism — noting it as a secondary consideration, not a dependency.

## What's buildable now vs. deferred

- **Buildable in Foundation (Phase 5 of PLAN.md), cheap**: wiring `middleware.Skills{SkillPaths: []string{"skills"}}` into `internal/genkit`'s app init, and creating an empty `skills/` directory with a README stating the convention. This is a few lines — no reason to defer the wiring itself.
- **Deferred to sub-project 6 (tool-agents) and beyond**: the actual skill *content* — real `skills/github-search/SKILL.md`, `skills/image-gen/SKILL.md`, etc. — because there's nothing to declare a capability for until those agents exist. Writing skill files before the capability exists would be exactly the kind of premature abstraction this project's own conventions warn against.
- ADK Go 2.0's graph-based workflow engine remains the likely execution context for multi-agent skill composition once sub-project 3 lands, but that specific integration is still a hypothesis to validate then, not a commitment made here.

## When this file next changes

When sub-project 6 adds its first real skill file, update this doc's "what's buildable now" section to reflect that skills have moved from "convention defined" to "in use," and link the first real example.
