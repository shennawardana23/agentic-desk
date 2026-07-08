package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/core"

	"github.com/shennawardana23/agentic-desk/internal/orchestrator"
)

// fakeChatStream satisfies ChatStreamFlow with a canned chunk sequence,
// mirroring what *core.Flow.Stream produces (chunks, then Done with output,
// or an error mid-stream).
type fakeChatStream struct {
	chunks []orchestrator.ChatChunk
	reply  string
	err    error
}

func (f *fakeChatStream) Stream(ctx context.Context, in orchestrator.ChatInput) func(func(*core.StreamingFlowValue[orchestrator.ChatOutput, orchestrator.ChatChunk], error) bool) {
	return func(yield func(*core.StreamingFlowValue[orchestrator.ChatOutput, orchestrator.ChatChunk], error) bool) {
		for _, c := range f.chunks {
			if !yield(&core.StreamingFlowValue[orchestrator.ChatOutput, orchestrator.ChatChunk]{Stream: c}, nil) {
				return
			}
		}
		if f.err != nil {
			yield(nil, f.err)
			return
		}
		yield(&core.StreamingFlowValue[orchestrator.ChatOutput, orchestrator.ChatChunk]{Done: true, Output: orchestrator.ChatOutput{Reply: f.reply}}, nil)
	}
}

func TestChatStream_EmitsSSEEventsInOrder(t *testing.T) {
	r := NewRouter(Deps{ChatStream: &fakeChatStream{
		chunks: []orchestrator.ChatChunk{
			{Type: "reasoning", Content: "thinking hard"},
			{Type: "text", Content: "partial "},
		},
		reply: "partial answer",
	}})

	req := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(`{"history":[],"message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	want := "data: {\"type\":\"reasoning\",\"content\":\"thinking hard\"}\n\n" +
		"data: {\"type\":\"text\",\"content\":\"partial \"}\n\n" +
		"data: {\"reply\":\"partial answer\",\"type\":\"done\"}\n\n"
	if w.Body.String() != want {
		t.Fatalf("body = %q, want %q", w.Body.String(), want)
	}
}

func TestChatStream_ErrorEventSanitized(t *testing.T) {
	r := NewRouter(Deps{ChatStream: &fakeChatStream{err: context.DeadlineExceeded}})

	req := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"type":"error"`) || !strings.Contains(body, "chat: internal error") {
		t.Fatalf("body = %q, want sanitized error event", body)
	}
	if strings.Contains(body, "deadline") {
		t.Fatalf("body leaked the real error: %q", body)
	}
}

func TestChatStream_NotConfiguredReturns503(t *testing.T) {
	r := NewRouter(Deps{})
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}
