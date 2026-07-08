// Package nim provides an ai.Model backed by NVIDIA NIM's hosted API
// catalog (build.nvidia.com), which exposes an OpenAI-compatible chat
// completions endpoint at https://integrate.api.nvidia.com/v1 fronting
// Meta, NVIDIA, Mistral, and other open models — confirmed live via
// NVIDIA's own docs and forum threads (docs.litellm.ai/docs/providers/nvidia_nim,
// ai-sdk.dev/providers/openai-compatible-providers/nim), not assumed.
//
//	export NVIDIA_API_KEY=nvapi-...              # required
//	export NIM_MODEL=meta/llama-3.3-70b-instruct # optional, overrides the catalog default
package nim

import (
	"context"
	"os"

	"github.com/firebase/genkit/go/ai"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
	"github.com/shennawardana23/agentic-desk/internal/provider/oaicompat"
)

const (
	baseURL  = "https://integrate.api.nvidia.com/v1"
	provider = "nim"

	// EnvAPIKey uses NVIDIA's own real env-var name (matches every other
	// NIM-compatible tool: litellm, the official OpenAI SDK usage, etc.),
	// not a repo-invented one.
	EnvAPIKey = "NVIDIA_API_KEY" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel  = "NIM_MODEL"
)

func init() {
	catalog.Register(catalog.ProviderCatalog{
		Provider: provider,
		Label:    "NVIDIA NIM",
		Models: []catalog.ModelEntry{
			{ID: "meta/llama-3.1-8b-instruct", Label: "Llama 3.1 8B Instruct", Tags: []string{"free", "fast"}, Default: true},
			{ID: "meta/llama-3.3-70b-instruct", Label: "Llama 3.3 70B Instruct", Tags: []string{"free"}},
			{ID: "nvidia/llama-3.1-nemotron-70b-instruct", Label: "Nemotron 70B Instruct", Tags: []string{"free", "reasoning"}},
		},
	})
}

// Config holds NVIDIA NIM-specific configuration.
type Config struct {
	// APIKey is the NVIDIA API key (starts with "nvapi-"). Required.
	APIKey string

	// ModelName is the NIM model identifier ("org/name" format). Defaults
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

// NewModel returns an ai.Model backed by NVIDIA NIM.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	return oaicompat.NewModel(ctx, oaicompat.Config{
		Provider:  provider,
		BaseURL:   baseURL,
		APIKey:    cfg.APIKey,
		ModelName: cfg.ModelName,
	})
}
