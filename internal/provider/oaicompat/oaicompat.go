// Package oaicompat is the shared construction helper for any provider
// exposing an OpenAI-compatible chat-completions API (Groq, OpenRouter,
// OpenCode, GitHub Models, NVIDIA NIM, HuggingFace, DeepSeek, and any
// future custom endpoint). It wraps Genkit's own compat_oai plugin —
// this package exists only to give every provider package the same
// small Config/NewModel shape, not to add behavior compat_oai lacks.
//
// Unlike a reference implementation this was modeled on (which layers
// an ADK model.LLM translation at this same point), agentic-desk's chat
// flow calls genkit.Generate/ai.WithModel directly — no ADK bridge is
// needed to *use* the model this returns. See
// docs/superpowers/specs/2026-07-07-multi-provider-model-layer-design.md.
package oaicompat

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/compat_oai"
)

// Config holds everything needed to define an ai.Model backed by an
// OpenAI-compatible provider.
type Config struct {
	// Provider is the short lowercase identifier used as the Genkit action
	// namespace (e.g. "groq", "openrouter").
	Provider string

	// BaseURL is the provider's OpenAI-compatible REST endpoint.
	BaseURL string

	// APIKey is the bearer token for authentication.
	APIKey string

	// ModelName is the model identifier the provider accepts.
	ModelName string

	// Label is a human-readable name for Genkit tooling.
	// Defaults to "<Provider> / <ModelName>" when empty.
	Label string
}

// NewModel defines and returns an ai.Model backed by the provider
// described in cfg.
func NewModel(ctx context.Context, cfg Config) (ai.Model, error) {
	switch {
	case cfg.Provider == "":
		return nil, fmt.Errorf("oaicompat.NewModel: Provider is required")
	case cfg.BaseURL == "":
		return nil, fmt.Errorf("oaicompat.NewModel: BaseURL is required")
	case cfg.APIKey == "":
		return nil, fmt.Errorf("oaicompat.NewModel: APIKey is required")
	case cfg.ModelName == "":
		return nil, fmt.Errorf("oaicompat.NewModel: ModelName is required")
	}

	plugin := &compat_oai.OpenAICompatible{
		Provider: cfg.Provider,
		APIKey:   cfg.APIKey,
		BaseURL:  cfg.BaseURL,
	}
	plugin.Init(ctx)

	label := cfg.Label
	if label == "" {
		label = fmt.Sprintf("%s / %s", cfg.Provider, cfg.ModelName)
	}

	return plugin.DefineModel(cfg.Provider, cfg.ModelName, ai.ModelOptions{
		Label:    label,
		Supports: &compat_oai.BasicText,
	}), nil
}
