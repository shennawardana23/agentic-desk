// Package opencode provides an ai.Model backed by OpenCode's AI
// gateway (https://opencode.ai), which routes to curated models, many
// on a free tier.
//
//	export OPENCODE_API_KEY=...   # required
//	export OPENCODE_MODEL=...     # optional, overrides the catalog default
package opencode

import (
	"context"
	"os"

	"github.com/firebase/genkit/go/ai"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
	"github.com/shennawardana23/agentic-desk/internal/provider/oaicompat"
)

const (
	baseURL  = "https://opencode.ai/zen/v1"
	provider = "opencode"

	EnvAPIKey = "OPENCODE_API_KEY" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel  = "OPENCODE_MODEL"
)

func init() {
	catalog.Register(catalog.ProviderCatalog{
		Provider: provider,
		Label:    "OpenCode",
		Models: []catalog.ModelEntry{
			{ID: "minimax-m2.5-free", Label: "MiniMax M2.5 (free)", Tags: []string{"free"}, Default: true},
			{ID: "hy3-preview-free", Label: "HunYuan 3 Preview (free)", Tags: []string{"free"}},
		},
	})
}

// Config holds OpenCode-specific configuration.
type Config struct {
	// APIKey is the OpenCode API key. Required.
	APIKey string

	// ModelName is the model identifier. Defaults to the catalog's
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

// NewModel returns an ai.Model backed by OpenCode.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	return oaicompat.NewModel(ctx, oaicompat.Config{
		Provider:  provider,
		BaseURL:   baseURL,
		APIKey:    cfg.APIKey,
		ModelName: cfg.ModelName,
	})
}
