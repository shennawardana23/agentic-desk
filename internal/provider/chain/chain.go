// Package chain builds an ordered, env-driven list of ai.Model for use
// with Genkit's official middleware.Fallback. Setting a provider's API
// key env var is enough for it to join the chain — no code change, no
// central provider registry to edit, matching the plug-and-play UX a
// reference implementation (archpublicwebsite-agentic/internal/model)
// established for this pattern.
package chain

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/shennawardana23/agentic-desk/internal/provider/anthropic"
	"github.com/shennawardana23/agentic-desk/internal/provider/custom"
	"github.com/shennawardana23/agentic-desk/internal/provider/deepseek"
	"github.com/shennawardana23/agentic-desk/internal/provider/githubmodels"
	"github.com/shennawardana23/agentic-desk/internal/provider/groq"
	"github.com/shennawardana23/agentic-desk/internal/provider/huggingface"
	"github.com/shennawardana23/agentic-desk/internal/provider/nim"
	"github.com/shennawardana23/agentic-desk/internal/provider/ollama"
	"github.com/shennawardana23/agentic-desk/internal/provider/opencode"
	"github.com/shennawardana23/agentic-desk/internal/provider/openrouter"
)

// Build constructs the fallback chain in priority order: the Gemini
// primary/fallback models (already registered by orchestrator.Init,
// looked up here by name) first, then any other provider whose API-key
// env var is set. Returns an error only when no model is available at
// all — not even Gemini.
func Build(ctx context.Context, g *genkit.Genkit, primaryModel, fallbackModel string) ([]ai.Model, error) {
	var models []ai.Model

	if m := genkit.LookupModel(g, primaryModel); m != nil {
		models = append(models, m)
	} else {
		slog.Warn("chain.Build: gemini primary model not registered", "model", primaryModel)
	}
	if fallbackModel != "" && fallbackModel != primaryModel {
		if m := genkit.LookupModel(g, fallbackModel); m != nil {
			models = append(models, m)
		} else {
			slog.Warn("chain.Build: gemini fallback model not registered", "model", fallbackModel)
		}
	}

	if key := os.Getenv(groq.EnvAPIKey); key != "" {
		cfg := groq.ConfigFromEnv()
		if m, err := groq.NewModel(ctx, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "groq", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: groq unavailable", "err", err)
		}
	}

	if key := os.Getenv(openrouter.EnvAPIKey); key != "" {
		cfg := openrouter.ConfigFromEnv()
		if m, err := openrouter.NewModel(ctx, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "openrouter", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: openrouter unavailable", "err", err)
		}
	}

	if key := os.Getenv(opencode.EnvAPIKey); key != "" {
		cfg := opencode.ConfigFromEnv()
		if m, err := opencode.NewModel(ctx, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "opencode", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: opencode unavailable", "err", err)
		}
	}

	if key := os.Getenv(githubmodels.EnvToken); key != "" {
		cfg := githubmodels.ConfigFromEnv()
		if m, err := githubmodels.NewModel(ctx, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "github-models", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: github-models unavailable", "err", err)
		}
	}

	if key := os.Getenv(nim.EnvAPIKey); key != "" {
		cfg := nim.ConfigFromEnv()
		if m, err := nim.NewModel(ctx, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "nim", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: nim unavailable", "err", err)
		}
	}

	if key := os.Getenv(huggingface.EnvToken); key != "" {
		cfg := huggingface.ConfigFromEnv()
		if m, err := huggingface.NewModel(ctx, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "huggingface", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: huggingface unavailable", "err", err)
		}
	}

	if key := os.Getenv(deepseek.EnvAPIKey); key != "" {
		cfg := deepseek.ConfigFromEnv()
		if m, err := deepseek.NewModel(ctx, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "deepseek", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: deepseek unavailable", "err", err)
		}
	}

	if key := os.Getenv(anthropic.EnvAPIKey); key != "" {
		cfg := anthropic.ConfigFromEnv()
		if m, err := anthropic.NewModel(ctx, g, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "anthropic", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: anthropic unavailable", "err", err)
		}
	}

	// Ollama has no API key — joining is gated on OLLAMA_MODEL being set
	// explicitly instead, since there's no safe default model to try
	// dialing a local server for.
	if model := os.Getenv(ollama.EnvModel); model != "" {
		m := ollama.NewModel(ctx, g, ollama.ConfigFromEnv())
		models = append(models, m)
		slog.Info("chain.Build: provider joined", "provider", "ollama", "model", model)
	}

	if cfg := custom.ConfigFromEnv(); cfg.Ready() {
		if m, err := custom.NewModel(ctx, cfg); err == nil {
			models = append(models, m)
			slog.Info("chain.Build: provider joined", "provider", "custom", "model", cfg.ModelName)
		} else {
			slog.Warn("chain.Build: custom unavailable", "err", err)
		}
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("chain.Build: no providers available — set at least one API key (GEMINI_API_KEY, GROQ_API_KEY, OPENROUTER_API_KEY, OPENCODE_API_KEY, GITHUB_TOKEN, NVIDIA_API_KEY, HF_TOKEN, DEEPSEEK_API_KEY, ANTHROPIC_API_KEY, OLLAMA_MODEL, CUSTOM_OAI_*)")
	}
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name()
	}
	slog.Info("chain.Build: ready", "depth", len(models), "chain", names)
	return models, nil
}

// Refs converts models[1:] into ai.ModelRef values for
// middleware.Fallback.Models. models[0] is the primary, passed
// separately via ai.WithModel — Fallback only holds the *rest* of the
// chain.
func Refs(models []ai.Model) []ai.ModelRef {
	if len(models) <= 1 {
		return nil
	}
	refs := make([]ai.ModelRef, 0, len(models)-1)
	for _, m := range models[1:] {
		refs = append(refs, ai.NewModelRef(m.Name(), nil))
	}
	return refs
}
