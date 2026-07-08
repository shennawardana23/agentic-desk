package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/orchestrator"
)

// fakeChatFlow implements ChatFlow without a real Gemini call — mirrors
// fakeStore/fakeEmbedder's pattern in router_test.go.
type fakeChatFlow struct {
	lastIn orchestrator.ChatInput
	out    orchestrator.ChatOutput
	err    error
}

func (f *fakeChatFlow) Run(_ context.Context, in orchestrator.ChatInput) (orchestrator.ChatOutput, error) {
	f.lastIn = in
	if f.err != nil {
		return orchestrator.ChatOutput{}, f.err
	}
	return f.out, nil
}

func TestChat_ReturnsReplyAndThreadsHistory(t *testing.T) {
	flow := &fakeChatFlow{out: orchestrator.ChatOutput{Reply: "hi there"}}
	router := NewRouter(Deps{Store: newFakeStore(), Embedder: fakeEmbedder{}, Hub: NewHub(), Chat: flow})

	body := orchestrator.ChatInput{
		History: []orchestrator.ChatTurn{{Role: "user", Content: "earlier question"}},
		Message: "what's next",
	}
	w := doRequest(t, router, http.MethodPost, "/chat", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got == "" {
		t.Fatal("expected a non-empty response body")
	}
	if flow.lastIn.Message != "what's next" {
		t.Errorf("flow received Message = %q, want %q", flow.lastIn.Message, "what's next")
	}
	if len(flow.lastIn.History) != 1 || flow.lastIn.History[0].Content != "earlier question" {
		t.Errorf("flow received History = %+v, want history threaded through", flow.lastIn.History)
	}
}

func TestChat_FlowErrorSanitized(t *testing.T) {
	// orchestrator.ChatInput has no `binding:"required"` tag on Message — the flow
	// itself validates non-empty (see chat.go's DefineChatFlow) — so a flow
	// error is what this route needs to surface correctly: a 500 with the
	// same sanitized apiErr body every other route uses, not a panic or a
	// fake 200.
	flow := &fakeChatFlow{err: errFakeDriver}
	router := NewRouter(Deps{Store: newFakeStore(), Embedder: fakeEmbedder{}, Hub: NewHub(), Chat: flow})

	w := doRequest(t, router, http.MethodPost, "/chat", orchestrator.ChatInput{})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for a flow error, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChat_NotConfiguredReturns503(t *testing.T) {
	router := NewRouter(Deps{Store: newFakeStore(), Embedder: fakeEmbedder{}, Hub: NewHub()}) // Chat left nil
	w := doRequest(t, router, http.MethodPost, "/chat", orchestrator.ChatInput{Message: "hello"})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when Chat is unconfigured, got %d: %s", w.Code, w.Body.String())
	}
}
