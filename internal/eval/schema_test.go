package eval_test

import (
	"context"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/eval"
)

func TestSchemaEvaluator_NilSchemaAlwaysPasses(t *testing.T) {
	v, err := eval.SchemaEvaluator{}.Evaluate(context.Background(), eval.Observation{Output: "anything"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !v.Passed {
		t.Fatalf("expected Passed=true for a nil schema, got %+v", v)
	}
}

func TestSchemaEvaluator_ValidOutputPasses(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"reply"},
		"properties": map[string]any{
			"reply": map[string]any{"type": "string"},
		},
	}
	obs := eval.Observation{
		Output: map[string]any{"reply": "hello"},
		Schema: schema,
	}

	v, err := eval.SchemaEvaluator{}.Evaluate(context.Background(), obs)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !v.Passed {
		t.Fatalf("expected Passed=true, got %+v", v)
	}
}

func TestSchemaEvaluator_MissingRequiredFieldFails(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"reply"},
		"properties": map[string]any{
			"reply": map[string]any{"type": "string"},
		},
	}
	obs := eval.Observation{
		Output: map[string]any{"notReply": "hello"},
		Schema: schema,
	}

	v, err := eval.SchemaEvaluator{}.Evaluate(context.Background(), obs)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v.Passed {
		t.Fatal("expected Passed=false for a missing required field")
	}
	if v.Reason == "" {
		t.Fatal("expected a non-empty Reason explaining the failure")
	}
}

func TestSchemaEvaluator_WrongTypeFails(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"reply"},
		"properties": map[string]any{
			"reply": map[string]any{"type": "string"},
		},
	}
	obs := eval.Observation{
		Output: map[string]any{"reply": 42},
		Schema: schema,
	}

	v, err := eval.SchemaEvaluator{}.Evaluate(context.Background(), obs)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v.Passed {
		t.Fatal("expected Passed=false for a wrong-typed field")
	}
}
