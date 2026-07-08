// Package openrouter provides an ai.Model backed by OpenRouter, which
// routes requests to the most cost-effective available provider for
// the requested model.
//
//	export OPENROUTER_API_KEY=sk-or-...   # required
//	export OPENROUTER_MODEL=...           # optional, overrides the catalog default
package openrouter

import (
	"context"
	"os"

	"github.com/firebase/genkit/go/ai"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
	"github.com/shennawardana23/agentic-desk/internal/provider/oaicompat"
)

const (
	baseURL  = "https://openrouter.ai/api/v1"
	provider = "openrouter"

	EnvAPIKey = "OPENROUTER_API_KEY" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel  = "OPENROUTER_MODEL"
)

func init() {
	catalog.Register(catalog.ProviderCatalog{
		Provider: provider,
		Label:    "OpenRouter",
		Models: []catalog.ModelEntry{
			{ID: "meta-llama/llama-3.3-70b-instruct:free", Label: "Llama 3.3 70B (free)", Tags: []string{"free"}, Default: true},
			{ID: "google/gemini-2.0-flash-exp:free", Label: "Gemini 2.0 Flash (free)", Tags: []string{"free"}},
			{ID: "deepseek/deepseek-r1:free", Label: "DeepSeek R1 (free)", Tags: []string{"free", "reasoning"}},
		},
	})
}

// Config holds OpenRouter-specific configuration.
type Config struct {
	// APIKey is the OpenRouter API key. Required.
	APIKey string

	// ModelName is the OpenRouter model id ("org/name" format). Defaults
	// to the catalog's Default entry when empty.
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

// NewModel returns an ai.Model backed by OpenRouter.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	return oaicompat.NewModel(ctx, oaicompat.Config{
		Provider:  provider,
		BaseURL:   baseURL,
		APIKey:    cfg.APIKey,
		ModelName: cfg.ModelName,
	})
}
