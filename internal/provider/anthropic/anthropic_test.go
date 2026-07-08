package anthropic

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/genkit"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		model     string
		wantKey   string
		wantModel string
	}{
		{"explicit model set", "sk-ant-fake", "claude-opus-4-8", "sk-ant-fake", "claude-opus-4-8"},
		{"model empty resolves catalog default", "sk-ant-fake", "", "sk-ant-fake", "claude-sonnet-5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvAPIKey, tt.apiKey)
			t.Setenv(EnvModel, tt.model)

			cfg := ConfigFromEnv()
			if cfg.APIKey != tt.wantKey {
				t.Errorf("APIKey = %q, want %q", cfg.APIKey, tt.wantKey)
			}
			if cfg.ModelName != tt.wantModel {
				t.Errorf("ModelName = %q, want %q", cfg.ModelName, tt.wantModel)
			}
		})
	}
}

func TestConfigFromEnv_ResolvesCatalogDefault(t *testing.T) {
	c, ok := catalog.ForProvider(provider)
	if !ok {
		t.Fatal("anthropic provider not registered in catalog — init() didn't run")
	}
	if c.DefaultModel() == "" {
		t.Fatal("anthropic catalog has no Default-marked model")
	}
}

func TestNewModel(t *testing.T) {
	g := genkit.Init(context.Background())
	m, err := NewModel(context.Background(), g, Config{APIKey: "sk-ant-fake", ModelName: "claude-sonnet-5"})
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
	if got, want := m.Name(), "anthropic/claude-sonnet-5"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNewModel_MissingAPIKey(t *testing.T) {
	g := genkit.Init(context.Background())
	if _, err := NewModel(context.Background(), g, Config{ModelName: "claude-sonnet-5"}); err == nil {
		t.Fatal("NewModel() error = nil, want error for missing API key")
	}
}
