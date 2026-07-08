package orchestrator_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/shennawardana23/agentic-desk/internal/orchestrator"
)

// TestChatStreamFlow_ForwardsReasoningAndTextChunks drives the real
// streaming flow end-to-end against a local fake model (no live API call):
// the fake emits one reasoning chunk and two text chunks through the real
// ai.ModelStreamCallback plumbing, and the test asserts the flow's own
// stream callback receives them as typed ChatChunks, in order, with a
// non-reasoning/non-text part (media) filtered out — plus that the final
// ChatOutput carries the full reply.
func TestChatStreamFlow_ForwardsReasoningAndTextChunks(t *testing.T) {
	g := genkit.Init(context.Background())

	var capturedConfig any
	fakeName := "googleai/" + "gemini-flash-latest" // matches PrimaryModel's default so chain.Build finds it
	genkit.DefineModel(g, fakeName, &ai.ModelOptions{
		Supports: &ai.ModelSupports{Multiturn: true, SystemRole: true},
	}, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		capturedConfig = req.Config
		if cb != nil {
			chunks := []*ai.ModelResponseChunk{
				{Content: []*ai.Part{ai.NewReasoningPart("weighing the options", nil)}},
				{Content: []*ai.Part{ai.NewTextPart("hello ")}},
				{Content: []*ai.Part{ai.NewMediaPart("image/png", "data:image/png;base64,x")}}, // must be filtered
				{Content: []*ai.Part{ai.NewTextPart("world")}},
			}
			for _, c := range chunks {
				if err := cb(ctx, c); err != nil {
					return nil, err
				}
			}
		}
		return &ai.ModelResponse{Message: ai.NewModelTextMessage("hello world")}, nil
	})

	flows := orchestrator.DefineChatFlows(g)

	var got []orchestrator.ChatChunk
	var out orchestrator.ChatOutput
	for val, err := range flows.Stream.Stream(context.Background(), orchestrator.ChatInput{Message: "hi"}) {
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		if val.Done {
			out = val.Output
			continue
		}
		got = append(got, val.Stream)
	}

	want := []orchestrator.ChatChunk{
		{Type: "reasoning", Content: "weighing the options"},
		{Type: "text", Content: "hello "},
		{Type: "text", Content: "world"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chunks = %+v, want %+v", got, want)
	}
	if out.Reply != "hello world" {
		t.Fatalf("reply = %q, want %q", out.Reply, "hello world")
	}

	// The request must carry thinkingConfig.includeThoughts — that's the
	// switch that makes the real googlegenai plugin emit reasoning parts
	// at all (config_overrides.go in the pinned SDK).
	cfg, ok := capturedConfig.(map[string]any)
	if !ok {
		t.Fatalf("config = %T, want map[string]any", capturedConfig)
	}
	tc, ok := cfg["thinkingConfig"].(map[string]any)
	if !ok || tc["includeThoughts"] != true {
		t.Fatalf("thinkingConfig = %+v, want includeThoughts=true", cfg["thinkingConfig"])
	}
}

// TestChatStreamFlow_EmptyMessageRejected mirrors the non-streaming flow's
// validation on the streaming path.
func TestChatStreamFlow_EmptyMessageRejected(t *testing.T) {
	g := genkit.Init(context.Background())
	genkit.DefineModel(g, "googleai/gemini-flash-latest", &ai.ModelOptions{
		Supports: &ai.ModelSupports{Multiturn: true, SystemRole: true},
	}, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return &ai.ModelResponse{Message: ai.NewModelTextMessage("never")}, nil
	})

	flows := orchestrator.DefineChatFlows(g)
	if _, err := flows.Stream.Run(context.Background(), orchestrator.ChatInput{Message: "   "}); err == nil {
		t.Fatal("want error for blank message, got nil")
	}
}
