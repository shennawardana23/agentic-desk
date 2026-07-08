---
name: sdlc-practices
description: Guides branching strategy, code review standards, CI/CD gates, and release/rollback procedures. Use when choosing a branching model, setting up pull request review requirements, defining what must pass before a merge or deploy, or planning a release and its rollback path. Use when an incident traces back to a gap in the deployment process rather than the code itself.
category: Process
---

# SDLC Practices

## Overview

Software delivery lifecycle practices are the process scaffolding around writing code: how branches are structured, what a code review must verify, what gates a change must pass before it reaches production, and how a release is rolled back when it goes wrong. Good SDLC practice makes the safe path the easy path.

## When to Use

- Setting up or revising a branching strategy for a team or repo
- Defining pull request review requirements and reviewer responsibilities
- Designing CI/CD pipeline gates (what blocks a merge, what blocks a deploy)
- Planning a release, especially one with schema changes or feature flags
- Writing a rollback plan before a risky change ships
- Investigating why an incident wasn't caught before production

## Workflow

### 1. Choose a branching model that matches release cadence

| Model | Fits | Trade-off |
|---|---|---|
| Trunk-based (short-lived branches, merge to main daily) | Teams deploying multiple times a day, strong CI, feature flags available | Requires discipline to keep branches small and CI fast |
| GitHub Flow (feature branch → PR → main → deploy) | Most product teams with continuous deployment | Needs feature flags for anything spanning multiple days |
| Git Flow (develop/release/hotfix branches) | Scheduled releases, versioned software, multiple supported versions | Heavier process; slows down teams that could ship continuously |

Default to trunk-based or GitHub Flow for continuously deployed services. Reserve Git Flow for software with genuinely versioned, batched releases (e.g., installed clients, firmware). Never let feature branches live longer than a few days — long-lived branches accumulate merge conflicts and integration risk that scale worse than linearly with branch age.

### 2. Make code review a check on correctness, not a formality

A pull request template that forces the author to state intent reduces reviewer guesswork:

```markdown
## What changed and why
## How was this tested
## Rollback plan (if this touches production behavior)
## Screenshots (if UI-facing)
```

Reviewer responsibilities, in order of priority:
1. **Correctness** — does the code do what it claims, including edge cases and error paths?
2. **Safety** — could this leak data across tenants, introduce a security hole, or lose data on failure?
3. **Maintainability** — will the next person understand this without the author present?
4. **Style** — least important; automate this away with linters/formatters so humans don't spend review time on it.

Rules:
- Require at least one approval from someone who did not write the code, on every change to shared/production code.
- Reviewers should read the diff with the PR description open, not skim only the diff — intent matters for judging correctness.
- A review that only says "LGTM" on a non-trivial change is not a review; require a note on what was actually checked (tests run, edge cases considered).
- Block on unaddressed comments, not just "resolved" checkboxes — resolving a comment without responding to it defeats the point.

### 3. Define CI/CD gates as explicit pass/fail criteria

| Gate | Blocks | Typical checks |
|---|---|---|
| Pre-merge | Merging to main | Lint, type-check, unit tests, build succeeds |
| Pre-deploy | Deploying to staging/production | Integration tests, security scan, migration dry-run |
| Post-deploy | Marking a deploy as successful | Smoke tests, health checks, error-rate/latency monitoring window |

Rules:
- Every gate must be automated and enforced by the pipeline, not a manual "did you run the tests" checklist — manual gates get skipped under time pressure.
- A flaky test that's allowed to be re-run until green is not a gate; fix or quarantine flaky tests (see qa-engineering) rather than normalizing re-runs.
- Never allow `--no-verify` or an equivalent skip flag as a normal part of the workflow; if a gate is wrong, fix the gate, don't bypass it individually.
- Security scanning (dependency vulnerabilities, secret scanning) belongs pre-merge, not as a periodic separate audit — catching it at merge time is far cheaper than catching it after a release.

### 4. Plan releases with a rollback path decided in advance

Before merging a change that affects production behavior, answer: if this goes wrong in production, how do we undo it, and how fast?

- **Code-only changes**: rollback is redeploying the previous build. Confirm the previous build artifact is retained and redeployable.
- **Schema changes**: design migrations to be backward compatible with the previous code version for at least one deploy cycle (see backend-engineering) — this makes rollback safe even after the schema changed.
- **Feature flags**: prefer shipping behind a flag for anything risky or user-facing; rollback becomes "flip the flag off" instead of a redeploy, which is faster and lower-risk.
- **Data migrations/backfills**: run them idempotently and in reversible batches; a one-way irreversible data change should never ship without an explicit, reviewed sign-off given the risk.

Document the rollback plan in the PR or release ticket, not just in someone's head — during an incident is the worst time to be reconstructing what "undo" means.

### 5. Treat DNS, infrastructure, and config changes with the same rollback discipline as code

Any change to DNS records, environment variables, feature flag defaults, or infrastructure configuration should have its previous value recorded before the change, not just the new value. Before making the change:

```
# Record current state before changing
dig TXT example.com                 # save output
kubectl get configmap app-config -o yaml > configmap-backup-2026-07-07.yaml
```

This mirrors the org-wide expectation of being able to restore prior DNS/config values quickly — the record has to exist before the change, since it cannot be reconstructed reliably afterward.

## Checklist

- [ ] Branching model matches actual release cadence, not copied from an unrelated team's process
- [ ] Every PR to shared/production code has at least one independent approval
- [ ] Review checked correctness and safety, not only style
- [ ] CI enforces lint/type-check/tests pre-merge and integration/security checks pre-deploy, with no manual bypass
- [ ] Every production-affecting change has a documented rollback plan before merge
- [ ] Schema and data migrations are backward compatible or explicitly reviewed as irreversible
- [ ] Pre-change state (DNS, config, infra) is recorded before any risky change, not reconstructed after

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "It's a small change, we can skip review" | Small changes cause a disproportionate share of incidents precisely because they get less scrutiny. |
| "We'll figure out rollback if it breaks" | Improvising rollback during an active incident is slower and riskier than a plan written calmly beforehand. |
| "Flaky test, just re-run it" | A test that's re-run until green stops testing anything — it becomes a coin flip with extra CI minutes. |
| "We don't need to save the old config, we know what it was" | Memory of "what it was" degrades fast under incident stress; write it down before changing it. |

## Red Flags

- Feature branches open for weeks accumulating conflicts
- PR approvals given without evidence the reviewer read the diff (rubber-stamping)
- CI pipelines with a documented way to skip a failing gate
- Schema migrations that break the currently-deployed code version during rollout
- Infrastructure changes made without recording the previous value first

## Verification

- [ ] A rollback was actually rehearsed (in staging or via a game day) for at least the highest-risk release path
- [ ] CI gate configuration lives in version control, reviewed like code
- [ ] Post-deploy monitoring window is defined with explicit thresholds, not just "watch the dashboards"
- [ ] Previous DNS/config/infra values are retrievable from a record made before the change, not from memory
