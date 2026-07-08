package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/shennawardana23/agentic-desk/internal/eval"
)

func TestHub_PublishReachesSubscriber(t *testing.T) {
	hub := NewHub()
	events, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	hub.Publish(Event{Type: EventAgentLog, Payload: "thinking..."})

	select {
	case got := <-events:
		if got.Type != EventAgentLog || got.Payload != "thinking..." {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestHub_PublishDoesNotBlockOnFullSubscriber(t *testing.T) {
	hub := NewHub()
	_, unsubscribe := hub.Subscribe() // never drained
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		for i := 0; i < bufferedEventsPerConn+5; i++ {
			hub.Publish(Event{Type: EventAgentLog, Payload: i})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a full subscriber buffer")
	}
}

func TestEscalationHandler_PublishesHITLEvent(t *testing.T) {
	hub := NewHub()
	events, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	handler := EscalationHandler{Hub: hub}
	if err := handler.Escalate(context.Background(),
		eval.Observation{Output: "risky action"},
		eval.Verdict{Passed: false, Reason: "needs review", RequiresHuman: true},
	); err != nil {
		t.Fatalf("Escalate: %v", err)
	}

	select {
	case got := <-events:
		if got.Type != EventHITLEscalation {
			t.Fatalf("expected %s, got %s", EventHITLEscalation, got.Type)
		}
		payload, ok := got.Payload.(hitlEscalationPayload)
		if !ok || payload.Reason != "needs review" || !payload.RequiresHuman {
			t.Fatalf("unexpected payload: %+v", got.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for HITL event")
	}
}

// TestServeWS_ClientReceivesPublishedEvent is the real client↔server
// round trip PLAN.md's Phase 9 verify step asks for: a real WS client
// dials in over a real httptest server, and a Publish reaches it.
func TestServeWS_ClientReceivesPublishedEvent(t *testing.T) {
	hub := NewHub()
	router := NewRouter(Deps{Store: newFakeStore(), Embedder: fakeEmbedder{}, Hub: hub})
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Give serveWS a moment to register the subscription before
	// publishing — otherwise Publish could fire before Subscribe runs.
	time.Sleep(50 * time.Millisecond)
	hub.Publish(Event{Type: EventAgentLog, Payload: "step 1: reading file"})

	var got Event
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Type != EventAgentLog || got.Payload != "step 1: reading file" {
		t.Fatalf("unexpected event over the wire: %+v", got)
	}
}
