package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// SchemaEvaluator is the deterministic default Evaluator: it checks
// only that Observation.Output conforms to Observation.Schema. No
// live calls, no LLM-as-judge — see the package doc for why.
type SchemaEvaluator struct{}

var _ Evaluator = SchemaEvaluator{}

// Evaluate validates obs.Output against obs.Schema. A nil Schema
// always passes — there's nothing to validate against.
func (SchemaEvaluator) Evaluate(_ context.Context, obs Observation) (Verdict, error) {
	if obs.Schema == nil {
		return Verdict{Passed: true}, nil
	}

	result, err := gojsonschema.Validate(
		gojsonschema.NewGoLoader(obs.Schema),
		gojsonschema.NewGoLoader(obs.Output),
	)
	if err != nil {
		return Verdict{}, fmt.Errorf("schema evaluate: %w", err)
	}
	if result.Valid() {
		return Verdict{Passed: true}, nil
	}

	reasons := make([]string, 0, len(result.Errors()))
	for _, e := range result.Errors() {
		reasons = append(reasons, e.String())
	}
	return Verdict{Passed: false, Reason: strings.Join(reasons, "; ")}, nil
}
