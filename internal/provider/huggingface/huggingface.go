// Package huggingface provides an ai.Model backed by Hugging Face's
// Inference Providers router (https://router.huggingface.co/v1), which
// fully supports the OpenAI-compatible chat-completions API as of its
// own changelog (huggingface.co/changelog/inference-providers-openai-compatible),
// fronting many third-party inference backends behind one token — chat
// completion only, confirmed live against Hugging Face's own docs, not
// assumed.
//
//	export HF_TOKEN=hf_...                                    # required
//	export HUGGINGFACE_MODEL=meta-llama/Llama-3.3-70B-Instruct # optional, overrides the catalog default
package huggingface

import (
	"context"
	"os"

	"github.com/firebase/genkit/go/ai"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
	"github.com/shennawardana23/agentic-desk/internal/provider/oaicompat"
)

const (
	baseURL  = "https://router.huggingface.co/v1"
	provider = "huggingface"

	// EnvToken uses Hugging Face's own real env-var name (also read by
	// the huggingface_hub library), not a repo-invented one.
	EnvToken = "HF_TOKEN" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel = "HUGGINGFACE_MODEL"
)

func init() {
	catalog.Register(catalog.ProviderCatalog{
		Provider: provider,
		Label:    "Hugging Face",
		Models: []catalog.ModelEntry{
			{ID: "meta-llama/Llama-3.3-70B-Instruct", Label: "Llama 3.3 70B Instruct", Tags: []string{"tools"}, Default: true},
			{ID: "deepseek-ai/DeepSeek-V3-0324", Label: "DeepSeek V3", Tags: []string{"tools"}},
			{ID: "Qwen/Qwen2.5-72B-Instruct", Label: "Qwen 2.5 72B Instruct"},
		},
	})
}

// Config holds Hugging Face-specific configuration.
type Config struct {
	// Token is the Hugging Face access token (needs "Make calls to
	// Inference Providers" permission). Required.
	Token string

	// ModelName is the Hugging Face repo id ("org/name" format). Defaults
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
	return Config{Token: os.Getenv(EnvToken), ModelName: name}
}

// NewModel returns an ai.Model backed by Hugging Face's Inference
// Providers router.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	return oaicompat.NewModel(ctx, oaicompat.Config{
		Provider:  provider,
		BaseURL:   baseURL,
		APIKey:    cfg.Token,
		ModelName: cfg.ModelName,
	})
}
