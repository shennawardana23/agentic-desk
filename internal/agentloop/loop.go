// Package agentloop implements the Plan→Act→Observe→Critique state
// machine: bounded self-correction with human-in-the-loop escalation.
// It depends only on internal/eval's interface — never on a specific
// model provider or on internal/secondbrain directly — so it stays
// reusable outside this app (see design doc Section 5).
package agentloop

import (
	"context"
	"fmt"

	"github.com/shennawardana23/agentic-desk/internal/eval"
)

// Actor performs one Act step: given the prior attempt's failure
// reason (empty on the first attempt), it produces an Observation for
// the loop to judge. This is the seam where a real agent's Genkit flow
// call plugs in — agentloop never talks to a provider directly.
type Actor interface {
	Act(ctx context.Context, retryReason string) (eval.Observation, error)
}

// EscalationHandler is invoked exactly once per Run — either
// immediately on a Verdict flagged RequiresHuman, or once
// MaxIterations is exhausted without a passing Verdict.
//
// This is a stubbed pause for now: durable pause/resume via ADK Go is
// deferred to Phase 6b (design doc Open Item #4) until its actual API
// is verified — the design's original "ADK Go 2.0" reference doesn't
// exist; the live module tops out at v1.5.0. This interface is the
// seam that implementation will plug into later, unchanged.
type EscalationHandler interface {
	Escalate(ctx context.Context, obs eval.Observation, verdict eval.Verdict) error
}

// Result is what Run returns.
type Result struct {
	Observation eval.Observation
	Verdict     eval.Verdict
	Attempts    int
	Escalated   bool
}

// Loop drives bounded self-correction: Act, then Evaluate, retrying
// with the prior failure reason injected into the next Act call — not
// a blind retry — until a Verdict passes, one requires escalation, or
// MaxIterations is exhausted.
type Loop struct {
	Actor         Actor
	Evaluator     eval.Evaluator
	Escalation    EscalationHandler
	MaxIterations int
}

// Run executes the loop. Escalation fires exactly once: immediately
// on a RequiresHuman Verdict, or once after the final attempt if
// MaxIterations is exhausted without a passing Verdict — it never
// loops unbounded.
func (l *Loop) Run(ctx context.Context) (Result, error) {
	maxIterations := l.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 1
	}

	var (
		obs     eval.Observation
		verdict eval.Verdict
		reason  string
	)

	for attempt := 1; attempt <= maxIterations; attempt++ {
		var err error
		obs, err = l.Actor.Act(ctx, reason)
		if err != nil {
			return Result{Attempts: attempt}, fmt.Errorf("act (attempt %d): %w", attempt, err)
		}

		verdict, err = l.Evaluator.Evaluate(ctx, obs)
		if err != nil {
			return Result{Observation: obs, Attempts: attempt}, fmt.Errorf("evaluate (attempt %d): %w", attempt, err)
		}

		if verdict.RequiresHuman {
			return l.escalate(ctx, obs, verdict, attempt)
		}
		if verdict.Passed {
			return Result{Observation: obs, Verdict: verdict, Attempts: attempt}, nil
		}
		reason = verdict.Reason
	}

	return l.escalate(ctx, obs, verdict, maxIterations)
}

func (l *Loop) escalate(ctx context.Context, obs eval.Observation, verdict eval.Verdict, attempts int) (Result, error) {
	if err := l.Escalation.Escalate(ctx, obs, verdict); err != nil {
		return Result{Observation: obs, Verdict: verdict, Attempts: attempts}, fmt.Errorf("escalate: %w", err)
	}
	return Result{Observation: obs, Verdict: verdict, Attempts: attempts, Escalated: true}, nil
}
