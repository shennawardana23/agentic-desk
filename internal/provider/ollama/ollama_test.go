package ollama

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/genkit"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		model     string
		wantAddr  string
		wantModel string
	}{
		{"explicit server address", "http://localhost:9999", "llama3.1", "http://localhost:9999", "llama3.1"},
		{"server address empty defaults", "", "llama3.1", defaultServerAddress, "llama3.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvServerAddress, tt.addr)
			t.Setenv(EnvModel, tt.model)

			cfg := ConfigFromEnv()
			if cfg.ServerAddress != tt.wantAddr {
				t.Errorf("ServerAddress = %q, want %q", cfg.ServerAddress, tt.wantAddr)
			}
			if cfg.ModelName != tt.wantModel {
				t.Errorf("ModelName = %q, want %q", cfg.ModelName, tt.wantModel)
			}
		})
	}
}

func TestConfigFromEnv_NoModelDefault(t *testing.T) {
	// Unlike every other provider, Ollama has no catalog default —
	// ModelName must come through verbatim as "" when unset, since the
	// caller (chain.Build) uses that emptiness as the opt-in gate.
	t.Setenv(EnvServerAddress, "")
	t.Setenv(EnvModel, "")

	if got := ConfigFromEnv().ModelName; got != "" {
		t.Errorf("ModelName = %q, want empty (no default)", got)
	}
}

func TestNewModel(t *testing.T) {
	g := genkit.Init(context.Background())
	m := NewModel(context.Background(), g, Config{ServerAddress: "http://localhost:9999", ModelName: "llama3.1"})
	if m == nil {
		t.Fatal("NewModel() returned nil model")
	}
	if got, want := m.Name(), "ollama/llama3.1"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNewModel_EmptyServerAddressDefaults(t *testing.T) {
	g := genkit.Init(context.Background())
	// ServerAddress empty must not panic (the underlying plugin panics on
	// an empty ServerAddress) — NewModel is responsible for defaulting it.
	m := NewModel(context.Background(), g, Config{ModelName: "llama3.1"})
	if m == nil {
		t.Fatal("NewModel() returned nil model")
	}
}
