---
name: product-manager
description: Guides product requirement documents, prioritization frameworks, discovery, and metrics definition. Use when writing a PRD, prioritizing a backlog with RICE or MoSCoW, running discovery interviews before committing to a solution, or defining success metrics for a feature before it ships. Use when stakeholders disagree on what to build next and a structured decision is needed.
category: Product
---

# Product Management

## Overview

Product management here is the discipline of deciding what to build, in what order, and how to know if it worked — before engineering time is spent. This skill covers writing PRDs that engineers can actually build from, prioritization frameworks (RICE, MoSCoW), discovery practices that de-risk a solution before committing to it, and defining metrics that make success falsifiable.

## When to Use

- Writing a PRD for a new feature or significant change
- Prioritizing a backlog with more candidate work than capacity
- Deciding whether a problem is well enough understood to start building
- Defining what "success" means for a feature before it ships
- Resolving a disagreement between stakeholders about what to build next

## Workflow

### 1. Run discovery before writing the PRD

Discovery answers: is this a real problem, for whom, and how are they solving it today? Skipping this step is the most common cause of built-but-unused features.

Minimum discovery bar before writing a PRD:
- At least 3-5 conversations with people who have the problem (users, internal stakeholders who see the pattern, support/sales who hear it repeatedly)
- A documented current workaround — if people have no workaround at all, question whether the problem is painful enough to solve
- A rough sense of frequency and severity: does this happen daily and block work, or monthly and mildly annoy?

Discovery interview structure:
- Ask about past behavior, not hypothetical future behavior ("tell me about the last time you hit this" beats "would you use a feature that...")
- Ask "what did you do instead" — reveals the real workaround and its cost
- Avoid pitching the solution during discovery; a user agreeing your idea sounds good is not validation

### 2. Write PRDs engineers can build from, not just leadership can approve

A PRD that only describes the vision and skips constraints forces engineering to reverse-engineer scope during implementation. Structure:

```markdown
# PRD: <Feature Name>

## Problem
What problem, for whom, evidenced by what (discovery findings, data, support tickets).

## Goal
The single primary outcome this should produce. One sentence.

## Non-goals
Explicitly out of scope for this iteration — prevents scope creep mid-build.

## Requirements
- Must: <requirement> — the feature fails without this
- Should: <requirement> — meaningfully better with this, not blocking
- Could: <requirement> — nice to have if time allows

## User flows
Step-by-step for each primary flow, including error/empty states.

## Success metrics
- Primary metric: <specific, measurable, has a baseline>
- Guardrail metric(s): <what must NOT regress>

## Open questions
Explicitly unresolved items, owner, and decision deadline.
```

Rules:
- Non-goals are as important as goals — they are what keeps a PRD from expanding mid-build.
- Every "must" requirement should be testable; if you can't write a test for it, it's not specific enough yet.
- Include error and empty states in the user flow, not just the happy path — this is usually where scope surprises happen during implementation.

### 3. Prioritize with RICE for comparing many candidates

RICE scores: `(Reach × Impact × Confidence) / Effort`

| Factor | Definition | Example scale |
|---|---|---|
| Reach | How many users/events affected per time period | Number of users per quarter |
| Impact | How much it moves the goal per user affected | 3 = massive, 2 = high, 1 = medium, 0.5 = low, 0.25 = minimal |
| Confidence | How sure you are about Reach and Impact estimates | 100% = high, 80% = medium, 50% = low |
| Effort | Person-time to build | Person-months |

```
Feature A: Reach 5000, Impact 2, Confidence 0.8, Effort 2  → RICE = (5000×2×0.8)/2 = 4000
Feature B: Reach 500,  Impact 3, Confidence 1.0, Effort 1  → RICE = (500×3×1.0)/1  = 1500
```

RICE is best for comparing many candidate features against each other with rough but consistent estimates. It is not a substitute for judgment when confidence is low across the board — a low-confidence RICE score should trigger discovery, not a build decision.

### 4. Prioritize with MoSCoW for scoping a single release

MoSCoW splits a fixed set of requirements for one release into Must, Should, Could, Won't. Use this once you've already decided to build something (post-RICE) and need to draw the line for this iteration.

- **Must**: the release fails its purpose without it
- **Should**: important but the release still delivers value without it
- **Could**: desirable, cut first under time pressure
- **Won't** (this time): explicitly deferred — write it down so it doesn't get silently dropped or silently re-litigated

Rule: if more than ~60% of requirements land in "Must," the scope is too big for one release — split it.

### 5. Define metrics before launch, not after

A feature without a pre-defined success metric cannot be evaluated objectively after launch — post-hoc metric selection tends to find whatever number looks good.

- Choose one primary metric tied directly to the goal statement in the PRD.
- Choose at least one guardrail metric that must not regress (e.g., latency, error rate, unsubscribe rate) — optimizing the primary metric can silently damage something else.
- Record the baseline value before launch; "improved" is only meaningful relative to a number you wrote down beforehand.
- Set a decision date: when will you look at the metric and decide keep/iterate/kill? An open-ended "we'll monitor it" rarely results in a revisit.

## Checklist

- [ ] Discovery included real conversations with people who have the problem, not just internal assumption
- [ ] PRD has explicit non-goals, not just goals
- [ ] Every "must" requirement is testable
- [ ] Error and empty states are covered in the user flow, not left implicit
- [ ] Backlog candidates scored with RICE using consistent estimation scales
- [ ] Release scope split with MoSCoW, with "Won't (this time)" items written down
- [ ] Primary and guardrail metrics defined and baselined before launch, with a decision date set

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "We already know the problem, no need for discovery" | Internal assumption is frequently wrong about frequency, severity, or who actually has the problem. |
| "We'll figure out success metrics once it's live" | Post-hoc metric selection is prone to confirmation bias — pick the metric that already looks good. |
| "Everything is a Must, it's all important" | If everything is a Must, nothing has been prioritized — the exercise wasn't done, just labeled. |
| "RICE score is close enough, ship the top one" | A low-confidence RICE score on the top candidate is a signal to de-risk with discovery, not a green light. |

## Red Flags

- PRD with a features list but no problem statement or success metric
- Prioritization based on whoever asked most recently or most loudly (recency/authority bias)
- No guardrail metric — only a metric the team is trying to move upward
- Discovery consisting entirely of surveys asking "would you use this" about a proposed solution

## Verification

- [ ] PRD reviewed by an engineer who confirms the requirements are specific enough to estimate
- [ ] Success metric has a recorded baseline and a decision date on the calendar
- [ ] Non-goals section exists and was referenced at least once during scope discussions
- [ ] Prioritization decision is explainable to a stakeholder using the framework's actual inputs, not post-hoc justification
