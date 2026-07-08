package deepseek

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
		{"explicit model set", "sk-fake", "deepseek-v4-pro", "sk-fake", "deepseek-v4-pro"},
		{"model empty resolves catalog default", "sk-fake", "", "sk-fake", "deepseek-v4-flash"},
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
		t.Fatal("deepseek provider not registered in catalog — init() didn't run")
	}
	if c.DefaultModel() == "" {
		t.Fatal("deepseek catalog has no Default-marked model")
	}
}

func TestNewModel(t *testing.T) {
	m, err := NewModel(context.Background(), Config{APIKey: "sk-fake", ModelName: "deepseek-v4-flash"})
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
	if got, want := m.Name(), "deepseek/deepseek-v4-flash"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNewModel_MissingAPIKey(t *testing.T) {
	if _, err := NewModel(context.Background(), Config{ModelName: "deepseek-v4-flash"}); err == nil {
		t.Fatal("NewModel() error = nil, want error for missing API key")
	}
}
