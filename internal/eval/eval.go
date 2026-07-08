// Package eval defines the Evaluator interface Agent Loop's Critique
// step uses to judge an Act step's output, plus one deterministic
// default implementation. Building an LLM-as-judge harness now, before
// any real agent exists to judge, would be premature — see design doc
// Section 4; the interface being stable is what lets a smarter
// Evaluator swap in later with zero call-site changes.
//
// Not to be confused with Genkit's own github.com/firebase/genkit/go/ai
// Evaluator/DefineEvaluator — that's a dataset/CI-driven framework for
// offline batch-evaluating flow outputs (genkit eval:run), a different
// concern from this package's per-call, runtime Agent Loop judge. The
// name collision is coincidental (PLAN.md specified "Evaluator" before
// this distinction was checked against live docs); once a real
// LLM-as-judge phase lands, the two are meant to coexist — this
// package for live escalation decisions, genkit.DefineEvaluator
// separately for CI-time flow regression testing.
package eval

import "context"

// Observation is what an Evaluator judges: an Act step's output, plus
// the JSON Schema (nil if none applies) it's expected to conform to.
type Observation struct {
	Output any
	Schema map[string]any
}

// Verdict is Evaluate's judgment.
type Verdict struct {
	// Passed is false when Output failed evaluation; Reason then
	// explains why, and gets injected into the next Act attempt's
	// context by the loop — not a blind retry.
	Passed bool
	Reason string
	// RequiresHuman flags an outcome that must escalate to a human
	// regardless of Passed — e.g. a destructive action or low
	// confidence signal a future, smarter Evaluator might raise.
	RequiresHuman bool
}

// Evaluator judges whether an Observation is acceptable.
type Evaluator interface {
	Evaluate(ctx context.Context, obs Observation) (Verdict, error)
}
