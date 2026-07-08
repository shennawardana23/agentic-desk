package agentloop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/agentloop"
	"github.com/shennawardana23/agentic-desk/internal/eval"
)

// scriptedActor records every retryReason it was called with and
// returns a fixed Observation each time.
type scriptedActor struct {
	retryReasons []string
	output       any
	actErr       error
}

func (a *scriptedActor) Act(_ context.Context, retryReason string) (eval.Observation, error) {
	a.retryReasons = append(a.retryReasons, retryReason)
	if a.actErr != nil {
		return eval.Observation{}, a.actErr
	}
	return eval.Observation{Output: a.output}, nil
}

// scriptedEvaluator returns verdicts in order, repeating the last one
// if Evaluate is called more times than there are scripted verdicts.
type scriptedEvaluator struct {
	verdicts []eval.Verdict
	calls    int
}

func (e *scriptedEvaluator) Evaluate(context.Context, eval.Observation) (eval.Verdict, error) {
	i := e.calls
	if i >= len(e.verdicts) {
		i = len(e.verdicts) - 1
	}
	e.calls++
	return e.verdicts[i], nil
}

type countingEscalation struct {
	calls int
	err   error
}

func (h *countingEscalation) Escalate(context.Context, eval.Observation, eval.Verdict) error {
	h.calls++
	return h.err
}

func TestLoop_AlwaysFails_ExhaustsMaxIterationsThenEscalatesExactlyOnce(t *testing.T) {
	actor := &scriptedActor{output: "x"}
	evaluator := &scriptedEvaluator{verdicts: []eval.Verdict{{Passed: false, Reason: "always fails"}}}
	escalation := &countingEscalation{}

	loop := &agentloop.Loop{Actor: actor, Evaluator: evaluator, Escalation: escalation, MaxIterations: 3}
	result, err := loop.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(actor.retryReasons) != 3 {
		t.Fatalf("expected exactly 3 Act attempts, got %d", len(actor.retryReasons))
	}
	if escalation.calls != 1 {
		t.Fatalf("expected escalation to fire exactly once, got %d calls", escalation.calls)
	}
	if !result.Escalated {
		t.Error("expected Result.Escalated=true")
	}
	if result.Attempts != 3 {
		t.Errorf("expected Attempts=3, got %d", result.Attempts)
	}
}

func TestLoop_PassesOnSecondAttempt_StopsWithoutEscalating(t *testing.T) {
	actor := &scriptedActor{output: "x"}
	evaluator := &scriptedEvaluator{verdicts: []eval.Verdict{
		{Passed: false, Reason: "first attempt bad"},
		{Passed: true},
	}}
	escalation := &countingEscalation{}

	loop := &agentloop.Loop{Actor: actor, Evaluator: evaluator, Escalation: escalation, MaxIterations: 5}
	result, err := loop.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(actor.retryReasons) != 2 {
		t.Fatalf("expected exactly 2 Act attempts, got %d", len(actor.retryReasons))
	}
	if escalation.calls != 0 {
		t.Fatalf("expected escalation never called, got %d calls", escalation.calls)
	}
	if result.Escalated {
		t.Error("expected Result.Escalated=false")
	}
	if !result.Verdict.Passed {
		t.Error("expected the final Verdict to be Passed=true")
	}
	if result.Attempts != 2 {
		t.Errorf("expected Attempts=2, got %d", result.Attempts)
	}
}

func TestLoop_RetryReasonInjectedFromPriorVerdict(t *testing.T) {
	actor := &scriptedActor{output: "x"}
	evaluator := &scriptedEvaluator{verdicts: []eval.Verdict{
		{Passed: false, Reason: "missing field foo"},
		{Passed: true},
	}}
	loop := &agentloop.Loop{Actor: actor, Evaluator: evaluator, Escalation: &countingEscalation{}, MaxIterations: 5}

	if _, err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"", "missing field foo"}
	if len(actor.retryReasons) != len(want) {
		t.Fatalf("got %d retry reasons %v, want %d", len(actor.retryReasons), actor.retryReasons, len(want))
	}
	for i := range want {
		if actor.retryReasons[i] != want[i] {
			t.Errorf("retryReasons[%d] = %q, want %q", i, actor.retryReasons[i], want[i])
		}
	}
}

func TestLoop_RequiresHuman_EscalatesImmediatelyRegardlessOfBudget(t *testing.T) {
	actor := &scriptedActor{output: "x"}
	evaluator := &scriptedEvaluator{verdicts: []eval.Verdict{{Passed: false, RequiresHuman: true, Reason: "destructive action"}}}
	escalation := &countingEscalation{}

	loop := &agentloop.Loop{Actor: actor, Evaluator: evaluator, Escalation: escalation, MaxIterations: 10}
	result, err := loop.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(actor.retryReasons) != 1 {
		t.Fatalf("expected exactly 1 Act attempt before escalating, got %d", len(actor.retryReasons))
	}
	if escalation.calls != 1 {
		t.Fatalf("expected escalation to fire exactly once, got %d", escalation.calls)
	}
	if !result.Escalated {
		t.Error("expected Result.Escalated=true")
	}
}

func TestLoop_ZeroOrNegativeMaxIterationsTreatedAsOne(t *testing.T) {
	actor := &scriptedActor{output: "x"}
	evaluator := &scriptedEvaluator{verdicts: []eval.Verdict{{Passed: false, Reason: "nope"}}}
	escalation := &countingEscalation{}

	loop := &agentloop.Loop{Actor: actor, Evaluator: evaluator, Escalation: escalation, MaxIterations: 0}
	if _, err := loop.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(actor.retryReasons) != 1 {
		t.Fatalf("expected exactly 1 Act attempt, got %d", len(actor.retryReasons))
	}
	if escalation.calls != 1 {
		t.Fatalf("expected escalation to fire exactly once, got %d", escalation.calls)
	}
}

func TestLoop_ActError_StopsImmediatelyWithoutEscalating(t *testing.T) {
	actor := &scriptedActor{actErr: errors.New("boom")}
	evaluator := &scriptedEvaluator{verdicts: []eval.Verdict{{Passed: true}}}
	escalation := &countingEscalation{}

	loop := &agentloop.Loop{Actor: actor, Evaluator: evaluator, Escalation: escalation, MaxIterations: 3}
	_, err := loop.Run(context.Background())
	if err == nil {
		t.Fatal("expected an error when Act fails")
	}
	if len(actor.retryReasons) != 1 {
		t.Fatalf("expected Act called exactly once before stopping, got %d", len(actor.retryReasons))
	}
	if escalation.calls != 0 {
		t.Fatalf("expected escalation not called on a hard Act error, got %d", escalation.calls)
	}
}
