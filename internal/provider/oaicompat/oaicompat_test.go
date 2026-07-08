package oaicompat

import (
	"context"
	"strings"
	"testing"
)

func TestNewModel_RequiredFields(t *testing.T) {
	base := Config{
		Provider:  "test-provider",
		BaseURL:   "https://example.invalid/v1",
		APIKey:    "fake-key",
		ModelName: "fake-model",
	}

	tests := []struct {
		name    string
		mutate  func(c Config) Config
		wantErr string
	}{
		{"missing provider", func(c Config) Config { c.Provider = ""; return c }, "Provider is required"},
		{"missing base url", func(c Config) Config { c.BaseURL = ""; return c }, "BaseURL is required"},
		{"missing api key", func(c Config) Config { c.APIKey = ""; return c }, "APIKey is required"},
		{"missing model name", func(c Config) Config { c.ModelName = ""; return c }, "ModelName is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewModel(context.Background(), tt.mutate(base))
			if err == nil {
				t.Fatal("NewModel() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("NewModel() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNewModel_Success(t *testing.T) {
	m, err := NewModel(context.Background(), Config{
		Provider:  "test-provider",
		BaseURL:   "https://example.invalid/v1",
		APIKey:    "fake-key",
		ModelName: "fake-model",
	})
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
	if m == nil {
		t.Fatal("NewModel() returned nil model")
	}
	if got, want := m.Name(), "test-provider/fake-model"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNewModel_DefaultLabel(t *testing.T) {
	// Label defaults to "<Provider> / <ModelName>" when empty — this only
	// affects ai.ModelOptions.Label, which isn't exposed on ai.Model itself,
	// so this test exercises the code path without asserting the label
	// value directly (no accessor exists to read it back).
	if _, err := NewModel(context.Background(), Config{
		Provider:  "test-provider",
		BaseURL:   "https://example.invalid/v1",
		APIKey:    "fake-key",
		ModelName: "fake-model",
		Label:     "",
	}); err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
}
