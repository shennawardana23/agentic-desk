// Package ollama provides an ai.Model backed by Genkit's native
// plugins/ollama, talking to a local Ollama server — confirmed present
// at the pinned github.com/firebase/genkit/go@v1.10.0 module cache.
// Unlike every other provider in this package tree, Ollama has no API
// key: joining the fallback chain is gated on OLLAMA_MODEL being set
// explicitly (an empty default would mean every desk without a local
// Ollama server silently tries to dial localhost:11434 on every chat).
//
//	export OLLAMA_MODEL=llama3.1                    # required to opt in — no catalog default
//	export OLLAMA_SERVER_ADDRESS=http://localhost:11434 # optional, this is the plugin's own default
package ollama

import (
	"context"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/ollama"
)

const (
	defaultServerAddress = "http://localhost:11434"

	// EnvModel has no repo-invented "resolve from catalog" fallback —
	// Ollama models are whatever the user has pulled locally, so there
	// is no meaningful static default.
	EnvModel         = "OLLAMA_MODEL"
	EnvServerAddress = "OLLAMA_SERVER_ADDRESS"
)

// Config holds Ollama-specific configuration.
type Config struct {
	// ServerAddress is the local Ollama server's base URL. Defaults to
	// http://localhost:11434 when empty.
	ServerAddress string

	// ModelName is the locally-pulled Ollama model name (e.g. "llama3.1").
	// Required — there is no catalog default.
	ModelName string
}

// ConfigFromEnv returns a Config from environment variables.
func ConfigFromEnv() Config {
	addr := os.Getenv(EnvServerAddress)
	if addr == "" {
		addr = defaultServerAddress
	}
	return Config{ServerAddress: addr, ModelName: os.Getenv(EnvModel)}
}

// NewModel returns an ai.Model backed by a local Ollama server. Like
// Anthropic's plugin, Ollama's DefineModel requires the *genkit.Genkit
// instance, so callers must pass it.
func NewModel(ctx context.Context, g *genkit.Genkit, cfg Config) ai.Model {
	addr := cfg.ServerAddress
	if addr == "" {
		addr = defaultServerAddress
	}

	plugin := &ollama.Ollama{ServerAddress: addr}
	plugin.Init(ctx)

	return plugin.DefineModel(g, ollama.ModelDefinition{
		Name: cfg.ModelName,
		Type: "chat",
	}, nil)
}
