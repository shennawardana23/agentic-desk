# Documentation map

This project's docs follow the [Diátaxis](https://diataxis.fr/) framework (external convention — cited, not fetched live in this session): four quadrants, each answering a different kind of question, kept deliberately separate rather than mixed into one big manual.

| Quadrant | Answers | Where |
|---|---|---|
| **Tutorials** | "Teach me, step by step" | [`tutorials/`](tutorials/) |
| **How-to guides** | "Help me do this specific thing" | [`how-to/`](how-to/) |
| **Reference** | "Tell me the exact facts (API, config, schema)" | [`reference/`](reference/) |
| **Explanation** | "Help me understand why" | [`explanation/`](explanation/) |

Right now only `explanation/` has real content, because the "why" behind sub-project 1's design already exists (it's in the approved design doc) — tutorials, how-tos, and reference material don't exist yet because there's no running code for them to describe. See each subdirectory's `README.md` for what's expected to land there and when.

Other documentation entry points at the repo root: [`PRODUCT.md`](../PRODUCT.md), [`SYSTEM.md`](../SYSTEM.md), [`MEMORY.md`](../MEMORY.md), [`SKILL.md`](../SKILL.md), [`AGENTS.md`](../AGENTS.md), [`DESIGN.md`](../DESIGN.md), [`PLAN.md`](../PLAN.md). Historical/decision records: [`docs/adr/`](adr/) and [`docs/reviews/`](reviews/).
