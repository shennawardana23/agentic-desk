// Package deepseek provides an ai.Model backed by DeepSeek's own
// OpenAI-compatible API (https://api.deepseek.com/v1) — confirmed live
// against DeepSeek's official pricing/docs page. Model IDs are current
// as of that check: deepseek-chat/deepseek-reasoner are deprecated
// 2026-07-24 in favor of deepseek-v4-flash/deepseek-v4-pro, so the
// catalog below intentionally lists only the v4 names.
//
//	export DEEPSEEK_API_KEY=sk-...        # required
//	export DEEPSEEK_MODEL=deepseek-v4-pro # optional, overrides the catalog default
package deepseek

import (
	"context"
	"os"

	"github.com/firebase/genkit/go/ai"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
	"github.com/shennawardana23/agentic-desk/internal/provider/oaicompat"
)

const (
	baseURL  = "https://api.deepseek.com/v1"
	provider = "deepseek"

	// EnvAPIKey uses DeepSeek's own real env-var name, not a
	// repo-invented one.
	EnvAPIKey = "DEEPSEEK_API_KEY" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel  = "DEEPSEEK_MODEL"
)

func init() {
	catalog.Register(catalog.ProviderCatalog{
		Provider: provider,
		Label:    "DeepSeek",
		Models: []catalog.ModelEntry{
			{ID: "deepseek-v4-flash", Label: "DeepSeek V4 Flash", Tags: []string{"fast"}, Default: true},
			{ID: "deepseek-v4-pro", Label: "DeepSeek V4 Pro", Tags: []string{"reasoning"}},
		},
	})
}

// Config holds DeepSeek-specific configuration.
type Config struct {
	// APIKey is the DeepSeek API key. Required.
	APIKey string

	// ModelName is the DeepSeek model identifier. Defaults to the
	// catalog's Default entry when empty.
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

// NewModel returns an ai.Model backed by DeepSeek.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	return oaicompat.NewModel(ctx, oaicompat.Config{
		Provider:  provider,
		BaseURL:   baseURL,
		APIKey:    cfg.APIKey,
		ModelName: cfg.ModelName,
	})
}
