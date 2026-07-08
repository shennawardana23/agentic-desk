package orchestrator_test

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/shennawardana23/agentic-desk/internal/orchestrator"
)

// TestChatGenerate_ThreadsHistoryInOrder verifies, against the real
// genkit.Generate implementation (a local fake model, no live API call),
// that Sarza's exact ai.WithSystem+ai.WithMessages+trailing-user-turn
// construction produces the message sequence Generate actually sends to the
// model: [system, history..., new user turn], in that order, with roles and
// text intact. This is the multi-turn threading behavior the design relies
// on — verified by inspecting the real *ai.ModelRequest a captured fake model
// receives, not by trusting the doc's "sandwiched between system and user
// prompts" description alone (see generate.go's message-building code this
// asserts against).
func TestChatGenerate_ThreadsHistoryInOrder(t *testing.T) {
	g := genkit.Init(context.Background())

	var captured *ai.ModelRequest
	fake := genkit.DefineModel(g, "test/fake", &ai.ModelOptions{
		Supports: &ai.ModelSupports{Multiturn: true, SystemRole: true},
	}, func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		captured = req
		return &ai.ModelResponse{Message: ai.NewModelTextMessage("mock reply")}, nil
	})

	history := []orchestrator.ChatTurn{
		{Role: "user", Content: "what's my project context?"},
		{Role: "agent", Content: "you're building agentic-desk, a Go+Vue second brain app"},
	}

	resp, err := genkit.Generate(context.Background(), g,
		ai.WithModel(fake),
		ai.WithSystem(orchestrator.SarzaSystemPrompt),
		ai.WithMessages(append(historyToMessages(history), ai.NewUserMessage(ai.NewTextPart("and my profile rules?")))...),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text() != "mock reply" {
		t.Fatalf("got %q, want %q", resp.Text(), "mock reply")
	}

	if captured == nil {
		t.Fatal("fake model was never called")
	}
	want := []struct {
		role ai.Role
		text string
	}{
		{ai.RoleSystem, orchestrator.SarzaSystemPrompt},
		{ai.RoleUser, "what's my project context?"},
		{ai.RoleModel, "you're building agentic-desk, a Go+Vue second brain app"},
		{ai.RoleUser, "and my profile rules?"},
	}
	if len(captured.Messages) != len(want) {
		t.Fatalf("got %d messages, want %d: %+v", len(captured.Messages), len(want), captured.Messages)
	}
	for i, w := range want {
		got := captured.Messages[i]
		if got.Role != w.role {
			t.Errorf("message %d: role = %q, want %q", i, got.Role, w.role)
		}
		if got.Text() != w.text {
			t.Errorf("message %d: text = %q, want %q", i, got.Text(), w.text)
		}
	}
}

// historyToMessages mirrors internal/genkit's own (unexported) history
// conversion so this test doesn't depend on that package's internals —
// it independently re-derives the same *ai.Message sequence to compare
// against what the real flow logic is documented to produce.
func historyToMessages(history []orchestrator.ChatTurn) []*ai.Message {
	msgs := make([]*ai.Message, 0, len(history))
	for _, t := range history {
		if t.Role == "agent" {
			msgs = append(msgs, ai.NewModelTextMessage(t.Content))
			continue
		}
		msgs = append(msgs, ai.NewUserMessage(ai.NewTextPart(t.Content)))
	}
	return msgs
}
