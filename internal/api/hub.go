package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/shennawardana23/agentic-desk/internal/eval"
)

// Event is one message published to every connected GUI client over
// the WS channel — either an agent-thinking-log line or a
// human-in-the-loop escalation.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

const (
	EventAgentLog         = "agent_log"
	EventHITLEscalation   = "hitl_escalation"
	bufferedEventsPerConn = 16
)

// Hub is a simple broadcast pub/sub: every Publish is fanned out to
// every currently-registered subscriber channel. A slow or gone
// subscriber never blocks the publisher — its channel is buffered and
// a full channel just drops the event for that one subscriber.
type Hub struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

// NewHub returns an empty Hub, ready to accept subscribers.
func NewHub() *Hub {
	return &Hub{subscribers: make(map[chan Event]struct{})}
}

// Subscribe registers a new listener and returns its channel plus an
// unsubscribe func the caller must invoke when done (typically via
// defer) to avoid leaking the channel and its map entry.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, bufferedEventsPerConn)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		delete(h.subscribers, ch)
		h.mu.Unlock()
		close(ch)
	}
	return ch, unsubscribe
}

// Publish fans e out to every current subscriber. Non-blocking per
// subscriber: a full buffer drops the event for that subscriber only.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}

// EscalationHandler adapts Hub to agentloop.EscalationHandler,
// publishing every HITL escalation as an EventHITLEscalation so a
// connected GUI can render it. Kept in this package (not
// internal/agentloop) since agentloop must stay free of any transport
// concern per its own package doc.
type EscalationHandler struct {
	Hub *Hub
}

type hitlEscalationPayload struct {
	Output        any    `json:"output"`
	Reason        string `json:"reason"`
	RequiresHuman bool   `json:"requiresHuman"`
}

// Escalate implements agentloop.EscalationHandler.
func (h EscalationHandler) Escalate(_ context.Context, obs eval.Observation, verdict eval.Verdict) error {
	h.Hub.Publish(Event{
		Type: EventHITLEscalation,
		Payload: hitlEscalationPayload{
			Output: obs.Output, Reason: verdict.Reason, RequiresHuman: verdict.RequiresHuman,
		},
	})
	return nil
}

var upgrader = websocket.Upgrader{
	// Same-machine desktop app talking to its own locally-spawned
	// core process — no cross-origin browser client exists, so the
	// default same-origin check would reject the Wails webview's
	// origin unnecessarily. Revisit if this API is ever exposed
	// beyond localhost.
	CheckOrigin: func(*http.Request) bool { return true },
}

// serveWS upgrades the request to a WS connection, subscribes it to
// hub, and pumps every published Event to the client as JSON until
// the connection closes or the write fails.
func serveWS(c *gin.Context, hub *Hub) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "err", err)
		return
	}
	defer conn.Close()

	events, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	for event := range events {
		if err := conn.WriteJSON(event); err != nil {
			return
		}
	}
}
