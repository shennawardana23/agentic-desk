// Package catalog is a static, extensible registry of known models per
// provider — framework-agnostic, no Genkit/ADK import. It doubles as
// the source of truth each provider's ConfigFromEnv resolves its
// default model from (see internal/provider/*/*.go), not just future
// picker-UI decoration — so updating a deprecated model id is one
// catalog edit, not a hunt through provider files.
package catalog

import "sync"

// ModelEntry describes a single model offered by a provider.
type ModelEntry struct {
	// ID is the exact identifier passed to the provider's API.
	ID string

	// Label is a short human-readable name. Falls back to ID when empty.
	Label string

	// Tags are optional descriptors (e.g. "free", "fast", "reasoning").
	Tags []string

	// Default marks the entry selected when no model override is configured.
	Default bool
}

// DisplayName returns Label when set, otherwise ID.
func (e ModelEntry) DisplayName() string {
	if e.Label != "" {
		return e.Label
	}
	return e.ID
}

// ProviderCatalog groups a provider name with its known models.
type ProviderCatalog struct {
	// Provider is the short lowercase identifier (e.g. "groq", "openrouter").
	Provider string

	// Label is the display name (e.g. "Groq", "OpenRouter").
	Label string

	// Models is the ordered list of known models.
	// The first entry with Default==true is the default selection.
	Models []ModelEntry
}

// DefaultModel returns the catalog's Default-marked entry's ID, or "" if
// none is marked default (or the catalog has no models).
func (c ProviderCatalog) DefaultModel() string {
	for _, m := range c.Models {
		if m.Default {
			return m.ID
		}
	}
	return ""
}

var (
	mu       sync.RWMutex
	catalogs []ProviderCatalog
)

// Register adds a ProviderCatalog to the global registry.
// Safe for concurrent use; typically called from provider init() functions.
func Register(c ProviderCatalog) {
	mu.Lock()
	defer mu.Unlock()
	catalogs = append(catalogs, c)
}

// All returns a snapshot of registered catalogs in registration order.
func All() []ProviderCatalog {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]ProviderCatalog, len(catalogs))
	copy(out, catalogs)
	return out
}

// ForProvider returns the catalog for the named provider, or (zero, false).
func ForProvider(name string) (ProviderCatalog, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, c := range catalogs {
		if c.Provider == name {
			return c, true
		}
	}
	return ProviderCatalog{}, false
}
