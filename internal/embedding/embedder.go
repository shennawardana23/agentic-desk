// Package embedding wraps Genkit's gemini-embedding-2 model behind a
// small Embedder interface, plus a bounded worker pool for embedding
// many pieces of text concurrently without unbounded goroutine growth.
package embedding

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"google.golang.org/genai"

	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

// ModelName is the Genkit-registered, provider-prefixed name for
// Gemini's embedding model — verified against the live SDK source
// (plugins/googlegenai/models.go in github.com/firebase/genkit/go),
// resolving the design doc's Open Item #1 rather than assuming it.
const ModelName = "googleai/gemini-embedding-2"

// Embedder produces a single embedding vector for a piece of text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// GenkitEmbedder implements Embedder using Genkit + the googlegenai
// plugin. gemini-embedding-2's native output is 3072 dimensions, but it
// supports Matryoshka-truncated output via
// genai.EmbedContentConfig.OutputDimensionality (also verified against
// the live google.golang.org/genai source); this wrapper requests
// secondbrain.EmbeddingDim (768) to match the fixed-width vector
// columns migrations/0001_init.sql already defines.
type GenkitEmbedder struct {
	g *genkit.Genkit
}

// NewGenkitEmbedder wraps an initialized Genkit app (with the
// googlegenai plugin registered) as an Embedder.
func NewGenkitEmbedder(g *genkit.Genkit) *GenkitEmbedder {
	return &GenkitEmbedder{g: g}
}

var _ Embedder = (*GenkitEmbedder)(nil)

// Embed returns a secondbrain.EmbeddingDim-length vector for text.
func (e *GenkitEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	dims := int32(secondbrain.EmbeddingDim)
	resp, err := genkit.Embed(ctx, e.g,
		ai.WithEmbedderName(ModelName),
		ai.WithTextDocs(text),
		ai.WithConfig(&genai.EmbedContentConfig{OutputDimensionality: &dims}),
	)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("embed: no embeddings returned")
	}
	return resp.Embeddings[0].Embedding, nil
}
