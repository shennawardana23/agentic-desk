//go:build integration

package orchestrator_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/orchestrator"
)

// Requires GEMINI_API_KEY and makes a real, billed call to the Gemini
// API. Not run by the default `go test ./...` loop. This is the one
// Phase 5 test that could not be verified live in this environment (no
// GEMINI_API_KEY set) — see SESSION_HANDOFF.md. It exercises the exact
// same *core.Flow.Run the Genkit Dev UI/CLI would call via
// genkit.Handler, just in-process rather than over HTTP — no genkit
// CLI is installed here either.
func TestPlaceholderFlow_RoundTrip(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	ctx := context.Background()
	_, flow := orchestrator.Init(ctx, apiKey, "../../prompts")

	reply, err := flow.Run(ctx, "Genkit wiring")
	if err != nil {
		t.Fatalf("flow.Run: %v", err)
	}
	if strings.TrimSpace(reply) == "" {
		t.Fatal("expected a non-empty reply")
	}
}
