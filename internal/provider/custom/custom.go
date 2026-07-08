// Package custom is the generic OpenAI-compatible provider slot — any
// future endpoint not worth its own named package (a self-hosted vLLM
// server, a corporate proxy, a provider not yet built here) joins the
// fallback chain purely through env vars, zero new code. It wraps the
// same oaicompat helper every named provider in this tree uses.
//
//	export CUSTOM_OAI_BASE_URL=https://my-endpoint/v1  # required
//	export CUSTOM_OAI_API_KEY=...                       # required
//	export CUSTOM_OAI_MODEL=my-model-id                 # required
package custom

import (
	"context"
	"fmt"
	"os"

	"github.com/firebase/genkit/go/ai"

	"github.com/shennawardana23/agentic-desk/internal/provider/oaicompat"
)

const provider = "custom"

const (
	EnvBaseURL = "CUSTOM_OAI_BASE_URL"
	EnvAPIKey  = "CUSTOM_OAI_API_KEY" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel   = "CUSTOM_OAI_MODEL"
)

// Config holds the generic slot's configuration. Unlike every named
// provider, there is no catalog — the endpoint is arbitrary, so all
// three fields are read verbatim from env with no fallback.
type Config struct {
	BaseURL   string
	APIKey    string
	ModelName string
}

// ConfigFromEnv returns a Config from environment variables.
func ConfigFromEnv() Config {
	return Config{
		BaseURL:   os.Getenv(EnvBaseURL),
		APIKey:    os.Getenv(EnvAPIKey),
		ModelName: os.Getenv(EnvModel),
	}
}

// Ready reports whether all three required env vars are set — the
// gate chain.Build uses instead of checking a single API-key var, since
// this slot has no meaningful default for BaseURL or ModelName.
func (c Config) Ready() bool {
	return c.BaseURL != "" && c.APIKey != "" && c.ModelName != ""
}

// NewModel returns an ai.Model backed by whatever OpenAI-compatible
// endpoint cfg points at.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	if !cfg.Ready() {
		return nil, fmt.Errorf("custom.NewModel: %s, %s, and %s are all required", EnvBaseURL, EnvAPIKey, EnvModel)
	}
	return oaicompat.NewModel(ctx, oaicompat.Config{
		Provider:  provider,
		BaseURL:   cfg.BaseURL,
		APIKey:    cfg.APIKey,
		ModelName: cfg.ModelName,
	})
}
