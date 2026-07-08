package chain

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/shennawardana23/agentic-desk/internal/provider/anthropic"
	"github.com/shennawardana23/agentic-desk/internal/provider/custom"
	"github.com/shennawardana23/agentic-desk/internal/provider/deepseek"
	"github.com/shennawardana23/agentic-desk/internal/provider/githubmodels"
	"github.com/shennawardana23/agentic-desk/internal/provider/groq"
	"github.com/shennawardana23/agentic-desk/internal/provider/huggingface"
	"github.com/shennawardana23/agentic-desk/internal/provider/nim"
	"github.com/shennawardana23/agentic-desk/internal/provider/ollama"
	"github.com/shennawardana23/agentic-desk/internal/provider/opencode"
	"github.com/shennawardana23/agentic-desk/internal/provider/openrouter"
)

const (
	primaryModel  = "test/primary"
	fallbackModel = "test/fallback"
)

// noopModelFn never runs in these tests — chain.Build only registers/looks
// up models, it never calls Generate.
func noopModelFn(ctx context.Context, req *ai.ModelRequest, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	return nil, nil
}

// clearProviderEnv sets every provider env var chain.Build reads to "" so
// each subtest is hermetic regardless of what's set in the ambient shell —
// t.Setenv restores the prior value after the subtest, but only for vars
// it actually touches.
func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{
		groq.EnvAPIKey, groq.EnvModel,
		openrouter.EnvAPIKey, openrouter.EnvModel,
		opencode.EnvAPIKey, opencode.EnvModel,
		githubmodels.EnvToken, githubmodels.EnvModel,
		nim.EnvAPIKey, nim.EnvModel,
		huggingface.EnvToken, huggingface.EnvModel,
		deepseek.EnvAPIKey, deepseek.EnvModel,
		anthropic.EnvAPIKey, anthropic.EnvModel,
		ollama.EnvModel, ollama.EnvServerAddress,
		custom.EnvBaseURL, custom.EnvAPIKey, custom.EnvModel,
	} {
		t.Setenv(v, "")
	}
}

func TestBuild_NothingSet_Errors(t *testing.T) {
	clearProviderEnv(t)
	g := genkit.Init(context.Background())

	_, err := Build(context.Background(), g, primaryModel, fallbackModel)
	if err == nil {
		t.Fatal("Build() error = nil, want error when no provider is available")
	}
}

func TestBuild_OnlyGeminiSet_OneModel(t *testing.T) {
	clearProviderEnv(t)
	g := genkit.Init(context.Background())
	genkit.DefineModel(g, primaryModel, &ai.ModelOptions{Label: "primary"}, noopModelFn)

	models, err := Build(context.Background(), g, primaryModel, fallbackModel)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	if len(models) != 1 {
		t.Fatalf("Build() len = %d, want 1", len(models))
	}
	if models[0].Name() != primaryModel {
		t.Errorf("Build()[0].Name() = %q, want %q", models[0].Name(), primaryModel)
	}
}

func TestBuild_GeminiPrimaryAndFallback_TwoModels(t *testing.T) {
	clearProviderEnv(t)
	g := genkit.Init(context.Background())
	genkit.DefineModel(g, primaryModel, &ai.ModelOptions{Label: "primary"}, noopModelFn)
	genkit.DefineModel(g, fallbackModel, &ai.ModelOptions{Label: "fallback"}, noopModelFn)

	models, err := Build(context.Background(), g, primaryModel, fallbackModel)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	if len(models) != 2 {
		t.Fatalf("Build() len = %d, want 2", len(models))
	}
}

func TestBuild_GeminiPlusOneOtherProvider_TwoModels(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv(groq.EnvAPIKey, "gsk_fake")
	g := genkit.Init(context.Background())
	genkit.DefineModel(g, primaryModel, &ai.ModelOptions{Label: "primary"}, noopModelFn)

	models, err := Build(context.Background(), g, primaryModel, fallbackModel)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	if len(models) != 2 {
		t.Fatalf("Build() len = %d, want 2", len(models))
	}
	if got, want := models[1].Name(), "groq/"; len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("Build()[1].Name() = %q, want prefix %q", got, want)
	}
}

func TestBuild_AllProvidersSet_TwelveModels(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv(groq.EnvAPIKey, "gsk_fake")
	t.Setenv(openrouter.EnvAPIKey, "sk-or-fake")
	t.Setenv(opencode.EnvAPIKey, "oc-fake")
	t.Setenv(githubmodels.EnvToken, "ghp_fake")
	t.Setenv(nim.EnvAPIKey, "nvapi-fake")
	t.Setenv(huggingface.EnvToken, "hf_fake")
	t.Setenv(deepseek.EnvAPIKey, "sk-fake")
	t.Setenv(anthropic.EnvAPIKey, "sk-ant-fake")
	t.Setenv(ollama.EnvModel, "llama3.1")
	t.Setenv(custom.EnvBaseURL, "https://example.invalid/v1")
	t.Setenv(custom.EnvAPIKey, "fake-key")
	t.Setenv(custom.EnvModel, "fake-model")
	g := genkit.Init(context.Background())
	genkit.DefineModel(g, primaryModel, &ai.ModelOptions{Label: "primary"}, noopModelFn)
	genkit.DefineModel(g, fallbackModel, &ai.ModelOptions{Label: "fallback"}, noopModelFn)

	models, err := Build(context.Background(), g, primaryModel, fallbackModel)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	if len(models) != 12 {
		t.Fatalf("Build() len = %d, want 12 (2 gemini + groq + openrouter + opencode + github-models + nim + huggingface + deepseek + anthropic + ollama + custom)", len(models))
	}
}

func TestBuild_NoFallbackModelConfigured_NoDuplicate(t *testing.T) {
	clearProviderEnv(t)
	g := genkit.Init(context.Background())
	genkit.DefineModel(g, primaryModel, &ai.ModelOptions{Label: "primary"}, noopModelFn)

	// fallbackModel == primaryModel simulates GEMINI_FALLBACK_MODEL unset
	// and defaulting to the same value as primary — Build must not
	// double-count it.
	models, err := Build(context.Background(), g, primaryModel, primaryModel)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	if len(models) != 1 {
		t.Fatalf("Build() len = %d, want 1 (fallback == primary must not duplicate)", len(models))
	}
}

func TestRefs(t *testing.T) {
	clearProviderEnv(t)
	g := genkit.Init(context.Background())
	genkit.DefineModel(g, primaryModel, &ai.ModelOptions{Label: "primary"}, noopModelFn)
	genkit.DefineModel(g, fallbackModel, &ai.ModelOptions{Label: "fallback"}, noopModelFn)

	models, err := Build(context.Background(), g, primaryModel, fallbackModel)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	refs := Refs(models)
	if len(refs) != len(models)-1 {
		t.Fatalf("Refs() len = %d, want %d (tail only, primary excluded)", len(refs), len(models)-1)
	}

	if got, want := len(Refs(models[:1])), 0; got != want {
		t.Errorf("Refs() on a single-model slice = %d, want %d", got, want)
	}
}
