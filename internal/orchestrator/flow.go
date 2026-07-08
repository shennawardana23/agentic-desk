package orchestrator

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/middleware"
)

// PlaceholderFlowName is the name of the wiring-proof flow Init
// registers. No real chat/agent logic lives here — see sub-project 2.
const PlaceholderFlowName = "placeholderFlow"

// PromptName is the Dotprompt file (prompts/placeholder.prompt) the
// placeholder flow executes.
const PromptName = "placeholder"

// placeholderOutput mirrors prompts/placeholder.prompt's output schema.
type placeholderOutput struct {
	Reply string `json:"reply"`
}

// DefinePlaceholderFlow registers a minimal flow that runs the
// "placeholder" Dotprompt wrapped in the Fallback(outer)+Retry(inner)
// resilience stack (see design doc's decision table for that order:
// the primary model is retried before Fallback advances to the next
// model — verified against the live SDK source, see middleware_test.go).
// It exists solely to prove plugin registration, Dotprompt loading, and
// middleware composition work end-to-end.
func DefinePlaceholderFlow(g *genkit.Genkit) *core.Flow[string, string, struct{}] {
	return genkit.DefineFlow(g, PlaceholderFlowName, func(ctx context.Context, topic string) (string, error) {
		prompt := genkit.LookupPrompt(g, PromptName)
		if prompt == nil {
			return "", fmt.Errorf("placeholder flow: prompt %q not loaded — check the prompt directory", PromptName)
		}

		resp, err := prompt.Execute(ctx,
			ai.WithInput(map[string]any{"topic": topic}),
			ai.WithUse(
				&middleware.Fallback{Models: []ai.ModelRef{googlegenai.ModelRef(FallbackModel, nil)}},
				&middleware.Retry{MaxRetries: 3},
			),
		)
		if err != nil {
			return "", fmt.Errorf("placeholder flow: %w", err)
		}

		var out placeholderOutput
		if err := resp.Output(&out); err != nil {
			return "", fmt.Errorf("placeholder flow: parse output: %w", err)
		}
		return out.Reply, nil
	})
}
