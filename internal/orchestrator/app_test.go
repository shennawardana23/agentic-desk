package orchestrator_test

import (
	"context"
	"testing"

	genkitwrap "github.com/firebase/genkit/go/genkit"

	"github.com/shennawardana23/agentic-desk/internal/orchestrator"
)

// TestInit_WiresPromptAndPlaceholderFlow exercises Init end-to-end
// without any live API call: a dummy API key is enough for the
// googlegenai plugin to construct its client (it only validates the
// key lazily, on an actual Generate call), so this proves plugin
// registration, Dotprompt loading, and flow registration all work.
func TestInit_WiresPromptAndPlaceholderFlow(t *testing.T) {
	g, flow := orchestrator.Init(context.Background(), "dummy-key-not-used", "testdata/prompts")

	if prompt := genkitwrap.LookupPrompt(g, orchestrator.PromptName); prompt == nil {
		t.Fatalf("expected prompt %q to be loaded from testdata/prompts", orchestrator.PromptName)
	}
	if flow == nil || flow.Name() != orchestrator.PlaceholderFlowName {
		t.Fatalf("expected Init to return the registered placeholder flow, got %v", flow)
	}
}

func TestDefinePlaceholderFlow_Registered(t *testing.T) {
	g := genkitwrap.Init(context.Background(), genkitwrap.WithPromptDir("testdata/prompts"))
	flow := orchestrator.DefinePlaceholderFlow(g)
	if flow == nil {
		t.Fatal("expected a non-nil flow")
	}
	if flow.Name() != orchestrator.PlaceholderFlowName {
		t.Fatalf("got flow name %q, want %q", flow.Name(), orchestrator.PlaceholderFlowName)
	}
}
