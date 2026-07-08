package orchestrator_test

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/middleware"
)

func defineTestModel(t *testing.T, g *genkit.Genkit, name string, fn ai.ModelFunc) ai.Model {
	t.Helper()
	return genkit.DefineModel(g, name, &ai.ModelOptions{
		Supports: &ai.ModelSupports{Multiturn: true, SystemRole: true},
	}, fn)
}

// TestFallbackOuterRetryInner_RetriesPrimaryBeforeFallingBack verifies,
// against the real middleware.Fallback/middleware.Retry implementations
// (no live API calls — models are local test doubles), the composition
// order the design doc calls for: ai.WithUse(Fallback, Retry) puts
// Fallback outermost and Retry innermost, so the primary model is
// retried MaxRetries times before Fallback ever tries the next model —
// and each fallback candidate then gets exactly one attempt, since
// Fallback dispatches to them directly rather than through Retry.
// Verified by reading buildModelChain in ai/generate.go (mws[0] ends up
// outermost) and middleware/fallback.go's wrapModel (calls next() once,
// then loops its own Models list via direct m.Generate() calls) —
// not assumed from the design doc's prose alone.
func TestFallbackOuterRetryInner_RetriesPrimaryBeforeFallingBack(t *testing.T) {
	g := genkit.Init(context.Background())

	var primaryCalls, secondaryCalls int
	primary := defineTestModel(t, g, "test/primary", func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		primaryCalls++
		return nil, core.NewError(core.UNAVAILABLE, "primary down")
	})
	secondary := defineTestModel(t, g, "test/secondary", func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		secondaryCalls++
		return &ai.ModelResponse{Message: ai.NewModelTextMessage("secondary ok")}, nil
	})

	const maxRetries = 2
	resp, err := genkit.Generate(context.Background(), g,
		ai.WithModel(primary),
		ai.WithPrompt("hello"),
		ai.WithUse(
			&middleware.Fallback{Models: []ai.ModelRef{ai.NewModelRef(secondary.Name(), nil)}},
			&middleware.Retry{MaxRetries: maxRetries, InitialDelayMs: 1, MaxDelayMs: 5, NoJitter: true},
		),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text() != "secondary ok" {
		t.Fatalf("got %q, want %q", resp.Text(), "secondary ok")
	}
	if primaryCalls != maxRetries+1 {
		t.Errorf("expected primary retried %d times (1 initial + %d retries) before falling back, got %d calls", maxRetries+1, maxRetries, primaryCalls)
	}
	if secondaryCalls != 1 {
		t.Errorf("expected the fallback model called exactly once (no retry applied to fallback candidates), got %d calls", secondaryCalls)
	}
}

// TestFallback_AdvancesOnResourceExhaustedAnd500 verifies the exact two
// failure modes a real Gemini rate limit/quota outage surfaces as —
// 429 (core.RESOURCE_EXHAUSTED) and 500 (core.INTERNAL) — both trigger
// Retry's own retry-the-primary loop (verified via exact call count, not
// just "it eventually recovers") AND, once Retry gives up, Fallback's
// advance-to-next-model path. Verified against the pinned v1.10.0 source:
// both codes are in middleware/retry.go's defaultRetryStatuses AND
// middleware/fallback.go's defaultFallbackStatuses, alongside
// UNAVAILABLE/DEADLINE_EXCEEDED/ABORTED (Retry+Fallback) and NOT_FOUND/
// UNIMPLEMENTED (Fallback only). The existing tests in this file only
// exercise UNAVAILABLE — this closes that gap for the two statuses a
// real provider outage/quota-limit actually returns.
func TestFallback_AdvancesOnResourceExhaustedAnd500(t *testing.T) {
	for _, status := range []core.StatusName{core.RESOURCE_EXHAUSTED, core.INTERNAL} {
		t.Run(string(status), func(t *testing.T) {
			g := genkit.Init(context.Background())

			const maxRetries = 2
			var primaryCalls, secondaryCalls int
			primary := defineTestModel(t, g, "test/primary-"+string(status), func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
				primaryCalls++
				return nil, core.NewError(status, "primary exhausted")
			})
			secondary := defineTestModel(t, g, "test/secondary-"+string(status), func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
				secondaryCalls++
				return &ai.ModelResponse{Message: ai.NewModelTextMessage("fallback provider ok")}, nil
			})

			resp, err := genkit.Generate(context.Background(), g,
				ai.WithModel(primary),
				ai.WithPrompt("hello"),
				ai.WithUse(
					&middleware.Fallback{Models: []ai.ModelRef{ai.NewModelRef(secondary.Name(), nil)}},
					&middleware.Retry{MaxRetries: maxRetries, InitialDelayMs: 1, MaxDelayMs: 5, NoJitter: true},
				),
			)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if resp.Text() != "fallback provider ok" {
				t.Fatalf("got %q, want the fallback model's reply — Fallback did not advance on status %s", resp.Text(), status)
			}
			if primaryCalls != maxRetries+1 {
				t.Errorf("expected Retry to retry the primary %d times (1 initial + %d retries) on status %s before giving up, got %d calls", maxRetries+1, maxRetries, status, primaryCalls)
			}
			if secondaryCalls != 1 {
				t.Errorf("expected fallback model called exactly once, got %d", secondaryCalls)
			}
		})
	}
}

// TestFallbackOuterRetryInner_SucceedsWithoutFallingBack proves Retry
// alone can recover the primary without ever invoking Fallback's model
// list, when the primary succeeds inside the retry budget.
func TestFallbackOuterRetryInner_SucceedsWithoutFallingBack(t *testing.T) {
	g := genkit.Init(context.Background())

	var primaryCalls, secondaryCalls int
	primary := defineTestModel(t, g, "test/primary", func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		primaryCalls++
		if primaryCalls < 2 {
			return nil, core.NewError(core.UNAVAILABLE, "transient")
		}
		return &ai.ModelResponse{Message: ai.NewModelTextMessage("primary ok")}, nil
	})
	secondary := defineTestModel(t, g, "test/secondary", func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		secondaryCalls++
		return &ai.ModelResponse{Message: ai.NewModelTextMessage("secondary ok")}, nil
	})

	resp, err := genkit.Generate(context.Background(), g,
		ai.WithModel(primary),
		ai.WithPrompt("hello"),
		ai.WithUse(
			&middleware.Fallback{Models: []ai.ModelRef{ai.NewModelRef(secondary.Name(), nil)}},
			&middleware.Retry{MaxRetries: 3, InitialDelayMs: 1, MaxDelayMs: 5, NoJitter: true},
		),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text() != "primary ok" {
		t.Fatalf("got %q, want %q", resp.Text(), "primary ok")
	}
	if primaryCalls != 2 {
		t.Errorf("expected primary called twice (1 failure + 1 success), got %d", primaryCalls)
	}
	if secondaryCalls != 0 {
		t.Errorf("expected fallback never invoked when retry recovers the primary, got %d calls", secondaryCalls)
	}
}
