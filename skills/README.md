# Skills

Genkit's `middleware.Skills` (wired in `internal/genkit`, `SkillPaths: []string{"skills"}`) loads any subdirectory here containing a `SKILL.md` as an on-demand system instruction: the model calls `use_skill("name")` to pull the skill body into the conversation, instead of every skill's instructions sitting on every request's hot path.

Convention: one directory per capability, one `SKILL.md` per directory, optional YAML frontmatter (`name`, `description`).

No skill files exist yet — real agent skills are sub-project 3's scope (see `SKILL.md` at the repo root for the corrected placement of that work). This file only states the convention Phase 5 wires the loader against.
