// Package githubmodels provides an ai.Model backed by the GitHub
// Models inference API (https://models.inference.ai.azure.com), which
// exposes an OpenAI-compatible endpoint fronting OpenAI, Anthropic,
// Meta, Google, Mistral, Cohere, and more via a single GitHub token.
//
//	export GITHUB_TOKEN=github_pat_...             # required
//	export GITHUB_MODEL=Llama-3.3-70B-Instruct     # optional, overrides the catalog default
package githubmodels

import (
	"context"
	"fmt"
	"os"

	"github.com/firebase/genkit/go/ai"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
	"github.com/shennawardana23/agentic-desk/internal/provider/oaicompat"
)

const (
	baseURL  = "https://models.inference.ai.azure.com"
	provider = "github-models"

	// EnvToken uses the same GITHUB_TOKEN name this repo's existing
	// internal/tools/github integration convention would use — a
	// single PAT can cover both if it has the right scopes.
	EnvToken = "GITHUB_TOKEN" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel = "GITHUB_MODEL"
)

func init() {
	catalog.Register(catalog.ProviderCatalog{
		Provider: provider,
		Label:    "GitHub Models",
		Models: []catalog.ModelEntry{
			{ID: "Llama-3.3-70B-Instruct", Label: "Llama 3.3 70B", Tags: []string{"tools"}, Default: true},
			{ID: "gpt-4o-mini", Label: "GPT-4o Mini", Tags: []string{"tools", "fast"}},
			{ID: "DeepSeek-V3-0324", Label: "DeepSeek V3", Tags: []string{"tools"}},
		},
	})
}

// Config holds GitHub Models configuration.
type Config struct {
	// Token is the GitHub Personal Access Token (needs "models" read
	// permission). Required.
	Token string

	// ModelName is the model identifier from GitHub Marketplace.
	// Defaults to the catalog's Default entry when empty.
	ModelName string
}

// ConfigFromEnv returns a Config from environment variables.
func ConfigFromEnv() Config {
	name := os.Getenv(EnvModel)
	if name == "" {
		c, _ := catalog.ForProvider(provider)
		name = c.DefaultModel()
	}
	return Config{Token: os.Getenv(EnvToken), ModelName: name}
}

// NewModel returns an ai.Model backed by GitHub Models.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("githubmodels.NewModel: %s is required", EnvToken)
	}
	return oaicompat.NewModel(ctx, oaicompat.Config{
		Provider:  provider,
		BaseURL:   baseURL,
		APIKey:    cfg.Token,
		ModelName: cfg.ModelName,
	})
}
