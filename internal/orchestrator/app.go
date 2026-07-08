// Package orchestrator initializes the Genkit app foundation for this
// project: the GoogleAI model plugin, built-in resilience/skills
// middleware, Dotprompt (.prompt file) loading, and one placeholder
// flow that proves the wiring works end-to-end. No real chat/agent
// behavior lives here — that's sub-project 2's concern.
package orchestrator

import (
	"context"
	"os"

	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/middleware"
)

// PrimaryModel and FallbackModel back the placeholder flow's Fallback
// middleware, so the resilience composition can be exercised
// end-to-end, and are also the Gemini tier of chat.go's multi-provider
// chain (see internal/provider/chain). Overridable via
// GEMINI_PRIMARY_MODEL/GEMINI_FALLBACK_MODEL so a deprecated model id
// doesn't need a rebuild — the same ConfigFromEnv-override pattern
// every other provider in internal/provider/* already uses.
var (
	PrimaryModel  = envOr("GEMINI_PRIMARY_MODEL", "googleai/gemini-flash-latest")
	FallbackModel = envOr("GEMINI_FALLBACK_MODEL", "googleai/gemini-2.5-flash")
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Init creates the Genkit app: the GoogleAI model plugin, the built-in
// middleware plugin (registers Retry/Fallback/ToolApproval/Skills so
// they're visible in the Dev UI), Dotprompt loading from promptDir, and
// the placeholder flow that proves all of it is wired correctly. It
// returns that flow too, since genkit has no generic "look up a flow by
// name" accessor — callers that want to run it (cmd/core, tests) need
// the reference Init already has on hand.
func Init(ctx context.Context, apiKey, promptDir string) (*genkit.Genkit, *core.Flow[string, string, struct{}]) {
	g := genkit.Init(ctx,
		genkit.WithPlugins(
			&googlegenai.GoogleAI{APIKey: apiKey},
			&middleware.Middleware{},
		),
		genkit.WithPromptDir(promptDir),
	)
	return g, DefinePlaceholderFlow(g)
}
