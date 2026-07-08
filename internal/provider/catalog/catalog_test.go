package catalog

import "testing"

func TestModelEntry_DisplayName(t *testing.T) {
	tests := []struct {
		name string
		e    ModelEntry
		want string
	}{
		{"label set", ModelEntry{ID: "id-1", Label: "Nice Name"}, "Nice Name"},
		{"label empty falls back to id", ModelEntry{ID: "id-1"}, "id-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderCatalog_DefaultModel(t *testing.T) {
	tests := []struct {
		name string
		c    ProviderCatalog
		want string
	}{
		{"no models", ProviderCatalog{}, ""},
		{"no default marked", ProviderCatalog{Models: []ModelEntry{{ID: "a"}, {ID: "b"}}}, ""},
		{"first default wins", ProviderCatalog{Models: []ModelEntry{
			{ID: "a"},
			{ID: "b", Default: true},
			{ID: "c", Default: true},
		}}, "b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.DefaultModel(); got != tt.want {
				t.Errorf("DefaultModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRegisterAndForProvider(t *testing.T) {
	Register(ProviderCatalog{Provider: "test-provider-x", Label: "Test X", Models: []ModelEntry{
		{ID: "m1", Default: true},
	}})

	c, ok := ForProvider("test-provider-x")
	if !ok {
		t.Fatal("ForProvider() ok = false, want true")
	}
	if c.Label != "Test X" {
		t.Errorf("Label = %q, want %q", c.Label, "Test X")
	}
	if c.DefaultModel() != "m1" {
		t.Errorf("DefaultModel() = %q, want %q", c.DefaultModel(), "m1")
	}

	if _, ok := ForProvider("does-not-exist-x"); ok {
		t.Error("ForProvider() ok = true for unregistered provider, want false")
	}
}

func TestAll_ReturnsSnapshotNotSharedSlice(t *testing.T) {
	before := len(All())
	Register(ProviderCatalog{Provider: "test-provider-y"})
	after := All()
	if len(after) != before+1 {
		t.Fatalf("All() len = %d, want %d", len(after), before+1)
	}

	after[0].Provider = "mutated"
	if All()[0].Provider == "mutated" {
		t.Error("All() returned a slice aliasing internal state — mutation leaked")
	}
}
