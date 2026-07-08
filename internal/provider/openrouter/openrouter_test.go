package openrouter

import (
	"context"
	"testing"

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
		{"explicit model set", "sk-or-fake", "google/gemini-2.0-flash-exp:free", "sk-or-fake", "google/gemini-2.0-flash-exp:free"},
		{"model empty resolves catalog default", "sk-or-fake", "", "sk-or-fake", "meta-llama/llama-3.3-70b-instruct:free"},
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
		t.Fatal("openrouter provider not registered in catalog — init() didn't run")
	}
	if c.DefaultModel() == "" {
		t.Fatal("openrouter catalog has no Default-marked model")
	}
}

func TestNewModel(t *testing.T) {
	m, err := NewModel(context.Background(), Config{APIKey: "sk-or-fake", ModelName: "meta-llama/llama-3.3-70b-instruct:free"})
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
	if got, want := m.Name(), "openrouter/meta-llama/llama-3.3-70b-instruct:free"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNewModel_MissingAPIKey(t *testing.T) {
	if _, err := NewModel(context.Background(), Config{ModelName: "meta-llama/llama-3.3-70b-instruct:free"}); err == nil {
		t.Fatal("NewModel() error = nil, want error for missing API key")
	}
}
