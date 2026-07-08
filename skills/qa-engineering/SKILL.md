---
name: qa-engineering
description: Guides test strategy across unit and end-to-end layers, flaky-test triage, and regression suite design. Use when deciding what belongs in unit tests versus end-to-end tests, diagnosing a flaky test, building or pruning a regression suite, or defining the testing scope for a release. Use when a test suite has grown slow or untrustworthy and needs a systematic pass rather than ad hoc fixes.
category: Process
---

# QA Engineering

## Overview

QA engineering is deciding what to test, at what layer, and how to keep the suite fast and trustworthy as it grows. This skill covers the unit-vs-E2E trade-off, systematic flaky-test triage, and regression suite curation — the practices that keep a test suite an asset instead of a liability.

## When to Use

- Deciding whether a new test belongs at the unit, integration, or E2E layer
- A test fails intermittently and needs root-cause triage, not a re-run
- The regression suite has grown slow, redundant, or distrusted
- Defining test coverage requirements for a release
- A production bug shipped despite "full test coverage," and the gap needs to be found

## Workflow

### 1. Shape the testing pyramid deliberately

| Layer | Tests | Speed | Failure signal quality |
|---|---|---|---|
| Unit | Pure logic, one function/module, no I/O | Milliseconds | High — failure points to one thing |
| Integration | Component + real dependency (DB, cache) in isolation | Seconds | Medium — points to a boundary |
| Contract/API | Request/response shape against a real or realistic service | Seconds | Medium — points to an interface break |
| End-to-end | Full user journey through the real (or near-real) system | Minutes | Low — failure could be anywhere in the path |

Target shape: many unit tests, a meaningful but smaller set of integration/contract tests, and a deliberately small set of E2E tests covering only the journeys that must never break (checkout, login, booking confirmation). An inverted pyramid — mostly E2E, few unit tests — produces a suite that's slow, flaky, and tells you little about where the bug is.

Decision rule for where a new test belongs: if the behavior can be exercised without I/O, write a unit test. Only step up a layer when the thing under test is the integration itself (e.g., "does this SQL query return the right rows," "does this API contract hold").

### 2. Triage flaky tests with a fixed diagnostic sequence

A flaky test — one that fails intermittently without a code change — is a bug in the test or the system under test, not something to route around with retries. Retries hide the signal and let the underlying issue (often a real race condition) ship to production.

Diagnostic sequence:
1. **Reproduce locally**, running the test in isolation and in the full suite — many flakes are order-dependent (shared state leaking between tests).
2. **Check for time dependence** — hardcoded sleeps, `Date.now()`/`time.Now()` comparisons without tolerance, or assumptions about wall-clock ordering.
3. **Check for concurrency** — race conditions in the system under test, or in the test's own async handling (missing `await`, unresolved promise, goroutine not joined).
4. **Check for external dependency** — network calls, real clocks, real randomness, or shared test infrastructure (a shared staging DB another test suite also writes to).
5. **Check for resource exhaustion** — the flake only appears under CI parallelism (port collisions, connection pool exhaustion), not when run alone.

```go
// Flaky: real time comparison with no tolerance
if time.Since(start) < 100*time.Millisecond { t.Fail() }

// Fixed: inject a fake clock, remove wall-clock dependency from the assertion
fakeClock.Advance(50 * time.Millisecond)
if !handler.ProcessedWithin(fakeClock.Now(), 100*time.Millisecond) { t.Fail() }
```

Rules:
- Quarantine (mark skipped with a tracked ticket) a flaky test immediately rather than letting it erode trust in the whole suite — but quarantine is a holding pattern with an owner and a deadline, not a permanent state.
- Track flake rate per test in CI; a test flaky more than roughly 1% of runs needs triage before it accumulates enough failures to be normalized away.
- Never fix a flaky test by adding a longer sleep or a blanket retry wrapper — that's masking the symptom, not diagnosing the cause (see "Failure Investigation" principle: root cause, not workaround).

### 3. Curate the regression suite instead of only growing it

Regression suites tend to grow monotonically — tests get added, rarely removed — until the suite is slow and partially redundant.

- **Prune duplicates**: multiple tests exercising the same code path with trivially different input add runtime without adding new failure-detection power.
- **Consolidate table-driven cases**: many similar unit test functions can usually collapse into one table-driven test (see backend-engineering) without losing coverage.
- **Tag by risk, not just by feature**: tests covering payment, auth, and data integrity paths should be tagged so they can run on every change, while broader coverage runs on a slower cadence (nightly, pre-release).
- **Delete tests for removed features** immediately — a passing test for dead code gives false confidence and slows the suite for no signal.
- **Re-derive coverage from real incidents**: every production bug that reached users represents a coverage gap; add a regression test for it as part of the fix, not as a follow-up that may never happen.

### 4. Define release testing scope explicitly

Don't let "run the whole suite" stand in for a scoping decision. For each release, decide explicitly:
- Which E2E journeys must pass (usually: the small, critical set)
- Whether a manual exploratory pass is warranted (new, high-risk, or hard-to-automate surface)
- Whether a performance/load test is needed for this specific change (new hot path, schema change on a large table)
- What the rollback trigger is if a post-deploy smoke test fails (see sdlc-practices)

## Checklist

- [ ] New test placed at the lowest layer that can exercise the behavior (unit before integration before E2E)
- [ ] E2E suite covers only journeys that must never break, not every possible path
- [ ] Any flaky test is diagnosed with the fixed sequence above, not retried into passing
- [ ] Flaky tests are quarantined with an owner and deadline, never silently normalized
- [ ] Regression suite reviewed periodically for duplicate, dead, or redundant tests
- [ ] Every shipped production bug gets a regression test as part of its fix
- [ ] Release testing scope (E2E set, manual pass, load test) decided explicitly per release, not assumed

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "Just retry it, CI is flaky sometimes" | Retries convert a real bug (often a race condition) into invisible noise that ships anyway. |
| "More E2E tests means more confidence" | Past a small critical set, E2E tests mostly add runtime and flakiness without proportional bug-catching power. |
| "We never delete tests, more coverage is always good" | Dead and duplicate tests slow feedback loops and dilute attention from tests that matter. |
| "The bug is fixed, we don't need a regression test for it" | Without a regression test, the same class of bug reappears the next time that code path is touched. |

## Red Flags

- CI configuration with automatic retry-until-pass on any test
- E2E suite runtime measured in tens of minutes and growing every sprint
- Tests skipped with `// TODO: fix flaky test` and no ticket or owner
- A production incident whose root cause maps to a code path with zero test coverage

## Verification

- [ ] Test suite runtime is tracked over time and does not silently creep upward release over release
- [ ] Flake rate per test is visible in CI tooling, not just anecdotally known
- [ ] A sample of quarantined tests actually has open tickets with owners, not just a skip annotation
- [ ] Coverage report reviewed for the module touched by the most recent production incident
