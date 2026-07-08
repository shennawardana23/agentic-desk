// Package groq provides an ai.Model backed by Groq's OpenAI-compatible
// inference API.
//
//	export GROQ_API_KEY=gsk_...      # required
//	export GROQ_MODEL=llama-...      # optional, overrides the catalog default
package groq

import (
	"context"
	"os"

	"github.com/firebase/genkit/go/ai"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
	"github.com/shennawardana23/agentic-desk/internal/provider/oaicompat"
)

const (
	baseURL  = "https://api.groq.com/openai/v1"
	provider = "groq"

	// EnvAPIKey and EnvModel are the env-var names read by [ConfigFromEnv].
	EnvAPIKey = "GROQ_API_KEY" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel  = "GROQ_MODEL"
)

func init() {
	catalog.Register(catalog.ProviderCatalog{
		Provider: provider,
		Label:    "Groq",
		Models: []catalog.ModelEntry{
			{ID: "llama-3.1-8b-instant", Label: "Llama 3.1 8B Instant", Tags: []string{"fast"}, Default: true},
			{ID: "llama-3.3-70b-versatile", Label: "Llama 3.3 70B Versatile"},
			{ID: "gemma2-9b-it", Label: "Gemma 2 9B IT"},
		},
	})
}

// Config holds Groq-specific configuration.
type Config struct {
	// APIKey is the Groq API key. Required.
	APIKey string

	// ModelName is the Groq model identifier. Defaults to the catalog's
	// Default entry when empty.
	ModelName string
}

// ConfigFromEnv returns a Config from environment variables.
func ConfigFromEnv() Config {
	name := os.Getenv(EnvModel)
	if name == "" {
		c, _ := catalog.ForProvider(provider)
		name = c.DefaultModel()
	}
	return Config{APIKey: os.Getenv(EnvAPIKey), ModelName: name}
}

// NewModel returns an ai.Model backed by Groq.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	return oaicompat.NewModel(ctx, oaicompat.Config{
		Provider:  provider,
		BaseURL:   baseURL,
		APIKey:    cfg.APIKey,
		ModelName: cfg.ModelName,
	})
}
