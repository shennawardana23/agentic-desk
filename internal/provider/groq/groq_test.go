package groq

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
		{"explicit model set", "gsk_fake", "llama-3.3-70b-versatile", "gsk_fake", "llama-3.3-70b-versatile"},
		{"model empty resolves catalog default", "gsk_fake", "", "gsk_fake", "llama-3.1-8b-instant"},
		{"api key empty carries through", "", "", "", "llama-3.1-8b-instant"},
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
		t.Fatal("groq provider not registered in catalog — init() didn't run")
	}
	if c.DefaultModel() == "" {
		t.Fatal("groq catalog has no Default-marked model")
	}
}

func TestNewModel(t *testing.T) {
	m, err := NewModel(context.Background(), Config{APIKey: "gsk_fake", ModelName: "llama-3.1-8b-instant"})
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
	if got, want := m.Name(), "groq/llama-3.1-8b-instant"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNewModel_MissingAPIKey(t *testing.T) {
	if _, err := NewModel(context.Background(), Config{ModelName: "llama-3.1-8b-instant"}); err == nil {
		t.Fatal("NewModel() error = nil, want error for missing API key")
	}
}
