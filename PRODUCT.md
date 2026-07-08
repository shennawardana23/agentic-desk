# Product: Agentic Desk

## The problem

Every coding agent or harness you use — Claude Code, another CLI, an IDE assistant — starts from zero. Each one re-learns your coding principles (SOLID, style, review heuristics), has to be re-told your active project's context, and has no memory of past corrections you've given it. That context lives scattered across chat transcripts, CLAUDE.md-style config files, and your own head. Nothing carries it forward automatically across tools.

## The product

Agentic Desk is a personal desktop application built around a **Second Brain**: a structured, queryable store of your coding-principles profile, your per-project context, and your interaction history, exposed as an MCP server so *any* MCP-capable agent can read (and write feedback into) it — not just this app's own chat/voice agents.

Three domain surfaces sit on top of the Second Brain:
- a **chat agent** for direct text interaction,
- a **voice speech-to-speech agent** for hands-free interaction,
- and the Second Brain itself, reachable independently by external coding agents through MCP, whether or not the desktop GUI is even open.

## Who it's for

The individual developer who owns this repo. This is not a multi-tenant product — it is one person's second brain, built to their own specification.

## Why it's differentiated

- **Deterministic, non-hallucinated profile import.** Your coding-principles profile is seeded by parsing your actual CLAUDE.md/RULES.md/PRINCIPLES.md files with a literal, auditable, source-traceable parser — not an LLM summarizing "what it thinks you meant." Every imported rule points back to its exact source file, heading, and line range.
- **Always reachable, not just when the app is open.** The architecture runs a headless core process that owns the Second Brain and its MCP server independently of the GUI. Your Second Brain answers Claude Code (or anything else) whether or not you've opened the desktop window that session.
- **A bounded, honest Agent Loop.** Agents in this platform plan, act, observe, and critique their own output, self-correcting within a hard iteration cap before escalating to you as a human — durable escalation, not a blocking hang. Feedback you give at that point (or afterward) is captured and fed back into the Second Brain as a preference signal. This is deliberately *not* marketed as literal RLHF: no model weights are trained here. It does the practical job — future prompts get better-informed context — without overclaiming a training mechanism that isn't actually built.
- **Composable by design.** SOLID/SoC package boundaries mean the Second Brain, the Agent Loop, and each tool integration are independently testable and independently replaceable, so the platform can grow through 9 ordered sub-projects without any one of them becoming an unreviewable rewrite.

## Current state

Pre-implementation. Sub-project 1 (Foundation + Second Brain + Agent Loop) has an approved design and phase plan — see [`DESIGN.md`](DESIGN.md) and [`PLAN.md`](PLAN.md). No functionality described above exists in running code yet.
