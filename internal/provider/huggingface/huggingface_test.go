package huggingface

import (
	"context"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/provider/catalog"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		model     string
		wantToken string
		wantModel string
	}{
		{"explicit model set", "hf_fake", "Qwen/Qwen2.5-72B-Instruct", "hf_fake", "Qwen/Qwen2.5-72B-Instruct"},
		{"model empty resolves catalog default", "hf_fake", "", "hf_fake", "meta-llama/Llama-3.3-70B-Instruct"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvToken, tt.token)
			t.Setenv(EnvModel, tt.model)

			cfg := ConfigFromEnv()
			if cfg.Token != tt.wantToken {
				t.Errorf("Token = %q, want %q", cfg.Token, tt.wantToken)
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
		t.Fatal("huggingface provider not registered in catalog — init() didn't run")
	}
	if c.DefaultModel() == "" {
		t.Fatal("huggingface catalog has no Default-marked model")
	}
}

func TestNewModel(t *testing.T) {
	m, err := NewModel(context.Background(), Config{Token: "hf_fake", ModelName: "meta-llama/Llama-3.3-70B-Instruct"})
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
	if got, want := m.Name(), "huggingface/meta-llama/Llama-3.3-70B-Instruct"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNewModel_MissingToken(t *testing.T) {
	if _, err := NewModel(context.Background(), Config{ModelName: "meta-llama/Llama-3.3-70B-Instruct"}); err == nil {
		t.Fatal("NewModel() error = nil, want error for missing token")
	}
}
