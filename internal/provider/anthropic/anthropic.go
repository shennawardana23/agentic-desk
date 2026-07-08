// Package anthropic provides an ai.Model backed by Genkit's native
// plugins/anthropic (not the OpenAI-compatible route — Anthropic's Claude
// API has its own message format, so it gets a real Genkit plugin
// instead of oaicompat). Confirmed present at the pinned
// github.com/firebase/genkit/go@v1.10.0 module — plugins/anthropic exists
// in the local module cache, not assumed from the newer genkit.dev docs
// (which reference the same package under the renamed genkit-ai/genkit
// import path; the pinned dependency here is still firebase/genkit/go).
//
//	export ANTHROPIC_API_KEY=sk-ant-...   # required
//	export ANTHROPIC_MODEL=claude-opus-4-8 # optional, overrides the catalog default
package anthropic

import (
	"context"
	"fmt"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/anthropic"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
)

const provider = "anthropic"

const (
	// EnvAPIKey matches the pinned plugin's own default env var
	// (plugins/anthropic falls back to this exact name when Anthropic.APIKey
	// is empty), not a repo-invented one.
	EnvAPIKey = "ANTHROPIC_API_KEY" // #nosec G101 -- env-var *name*, not a credential value
	EnvModel  = "ANTHROPIC_MODEL"
)

// modelSupports mirrors the pinned plugin's own unexported
// defaultClaudeOpts.Supports (plugins/anthropic/models.go) — duplicated
// here because it isn't exported, not because it might differ.
var modelSupports = ai.ModelSupports{
	Multiturn:   true,
	Tools:       true,
	ToolChoice:  true,
	SystemRole:  true,
	Media:       true,
	Constrained: ai.ConstrainedSupportAll,
}

func init() {
	// Model IDs current as of the 2026-07-07 verification against
	// Anthropic's own model list (platform.claude.com/docs) — see
	// SESSION_HANDOFF.md for the check.
	catalog.Register(catalog.ProviderCatalog{
		Provider: provider,
		Label:    "Anthropic",
		Models: []catalog.ModelEntry{
			{ID: "claude-sonnet-5", Label: "Claude Sonnet 5", Tags: []string{"balanced"}, Default: true},
			{ID: "claude-opus-4-8", Label: "Claude Opus 4.8", Tags: []string{"reasoning"}},
			{ID: "claude-haiku-4-5-20251001", Label: "Claude Haiku 4.5", Tags: []string{"fast"}},
		},
	})
}

// Config holds Anthropic-specific configuration.
type Config struct {
	// APIKey is the Anthropic API key. Required.
	APIKey string

	// ModelName is the Claude model identifier. Defaults to the
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

// NewModel returns an ai.Model backed by Anthropic. Unlike the
// OpenAI-compatible providers, Anthropic's plugin requires the
// *genkit.Genkit instance at DefineModel time, so callers must pass it.
func NewModel(ctx context.Context, g *genkit.Genkit, cfg Config) (ai.Model, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic.NewModel: %s is required", EnvAPIKey)
	}

	plugin := &anthropic.Anthropic{APIKey: cfg.APIKey}
	plugin.Init(ctx)

	return plugin.DefineModel(g, cfg.ModelName, &ai.ModelOptions{
		Label:    "Anthropic - " + cfg.ModelName,
		Supports: &modelSupports,
	})
}
