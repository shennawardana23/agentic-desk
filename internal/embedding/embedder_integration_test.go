//go:build integration

package embedding_test

import (
	"context"
	"os"
	"testing"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	"github.com/shennawardana23/agentic-desk/internal/embedding"
	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// Requires GEMINI_API_KEY and makes a real, billed call to the Gemini
// API. Not run by the default `go test ./...` loop. See
// SESSION_HANDOFF.md — this is the one Phase 4 test that could not be
// verified live in this environment (no GEMINI_API_KEY set).
func TestGenkitEmbedder_Smoke(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	ctx := context.Background()
	g := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{APIKey: apiKey}))

	emb := embedding.NewGenkitEmbedder(g)
	vector, err := emb.Embed(ctx, "hello, second brain")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vector) != secondbrain.EmbeddingDim {
		t.Fatalf("expected %d dims, got %d", secondbrain.EmbeddingDim, len(vector))
	}
}
