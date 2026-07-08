package custom

import (
	"context"
	"testing"
)

func TestConfigFromEnv_Ready(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		model   string
		want    bool
	}{
		{"all set", "https://example.invalid/v1", "fake-key", "fake-model", true},
		{"missing base url", "", "fake-key", "fake-model", false},
		{"missing api key", "https://example.invalid/v1", "", "fake-model", false},
		{"missing model", "https://example.invalid/v1", "fake-key", "", false},
		{"nothing set", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvBaseURL, tt.baseURL)
			t.Setenv(EnvAPIKey, tt.apiKey)
			t.Setenv(EnvModel, tt.model)

			if got := ConfigFromEnv().Ready(); got != tt.want {
				t.Errorf("Ready() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewModel_NotReady(t *testing.T) {
	if _, err := NewModel(context.Background(), Config{BaseURL: "https://example.invalid/v1"}); err == nil {
		t.Fatal("NewModel() error = nil, want error when not fully configured")
	}
}

func TestNewModel(t *testing.T) {
	m, err := NewModel(context.Background(), Config{
		BaseURL:   "https://example.invalid/v1",
		APIKey:    "fake-key",
		ModelName: "fake-model",
	})
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
	if got, want := m.Name(), "custom/fake-model"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}
