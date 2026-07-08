package adkbridge

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestNewModel_NilModel(t *testing.T) {
	if _, err := NewModel(nil); err == nil {
		t.Fatal("NewModel(nil) error = nil, want error")
	}
}

func TestNewModel_Name(t *testing.T) {
	fake := ai.NewModel("test/fake", nil, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		return &ai.ModelResponse{Message: &ai.Message{Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart("ok")}}}, nil
	})

	llm, err := NewModel(fake)
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}
	if got, want := llm.Name(), "test/fake"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

// TestGenerateContent_RoundTrip proves the request/response translation,
// not just wiring: a fake ai.Model captures the exact *ai.ModelRequest
// adkbridge builds from an ADK *model.LLMRequest, asserting system
// instruction, message roles/content, and tool declarations all survive
// the ADK→Genkit conversion — then the fake's canned response is
// asserted to survive the Genkit→ADK conversion back.
func TestGenerateContent_RoundTrip(t *testing.T) {
	var captured *ai.ModelRequest
	fake := ai.NewModel("test/fake", &ai.ModelOptions{Supports: &ai.ModelSupports{Tools: true, Multiturn: true, SystemRole: true}}, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		captured = req
		return &ai.ModelResponse{
			Message: &ai.Message{Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart("hi back")}},
			Usage:   &ai.GenerationUsage{InputTokens: 10, OutputTokens: 5},
		}, nil
	})

	llm, err := NewModel(fake)
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hello"}}},
		},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: "be nice"}}},
			Tools: []*genai.Tool{{
				FunctionDeclarations: []*genai.FunctionDeclaration{
					{Name: "get_weather", Description: "gets weather"},
				},
			}},
		},
	}

	var got *model.LLMResponse
	var gotErr error
	for resp, err := range llm.GenerateContent(context.Background(), req, false) {
		got, gotErr = resp, err
		break
	}
	if gotErr != nil {
		t.Fatalf("GenerateContent() error = %v, want nil", gotErr)
	}

	// Request side: system instruction prepended, then the user message.
	if captured == nil {
		t.Fatal("fake model was never called")
	}
	if len(captured.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2 (system + user)", len(captured.Messages))
	}
	if captured.Messages[0].Role != ai.RoleSystem {
		t.Errorf("Messages[0].Role = %q, want %q", captured.Messages[0].Role, ai.RoleSystem)
	}
	if captured.Messages[1].Role != ai.RoleUser {
		t.Errorf("Messages[1].Role = %q, want %q", captured.Messages[1].Role, ai.RoleUser)
	}
	if len(captured.Tools) != 1 || captured.Tools[0].Name != "get_weather" {
		t.Errorf("Tools = %+v, want one tool named get_weather", captured.Tools)
	}

	// Response side: translated back into ADK's shape.
	if got == nil || got.Content == nil || len(got.Content.Parts) != 1 {
		t.Fatalf("response = %+v, want one content part", got)
	}
	if got.Content.Parts[0].Text != "hi back" {
		t.Errorf("response text = %q, want %q", got.Content.Parts[0].Text, "hi back")
	}
	if got.UsageMetadata == nil || got.UsageMetadata.TotalTokenCount != 15 {
		t.Errorf("UsageMetadata = %+v, want TotalTokenCount 15", got.UsageMetadata)
	}
}

func TestGenerateContent_ToolCallRoundTrip(t *testing.T) {
	fake := ai.NewModel("test/fake", nil, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		// Assert the incoming tool-response message translated correctly.
		if len(req.Messages) != 1 || req.Messages[0].Role != ai.RoleTool {
			t.Errorf("Messages = %+v, want a single tool-role message", req.Messages)
		}
		return &ai.ModelResponse{
			Message: &ai.Message{Role: ai.RoleModel, Content: []*ai.Part{
				ai.NewToolRequestPart(&ai.ToolRequest{Name: "get_weather", Input: map[string]any{"city": "SF"}, Ref: "call-1"}),
			}},
		}, nil
	})

	llm, err := NewModel(fake)
	if err != nil {
		t.Fatalf("NewModel() error = %v, want nil", err)
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{
				Name:     "get_weather",
				ID:       "call-1",
				Response: map[string]any{"output": "sunny"},
			}}}},
		},
	}

	var got *model.LLMResponse
	for resp, err := range llm.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v, want nil", err)
		}
		got = resp
		break
	}

	if len(got.Content.Parts) != 1 || got.Content.Parts[0].FunctionCall == nil {
		t.Fatalf("response = %+v, want a single function call part", got)
	}
	if got.Content.Parts[0].FunctionCall.Name != "get_weather" {
		t.Errorf("FunctionCall.Name = %q, want %q", got.Content.Parts[0].FunctionCall.Name, "get_weather")
	}
}
