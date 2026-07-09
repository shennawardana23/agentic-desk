package voicelive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/genai"
)

// Bridge is the public entry point for the voice WS handler in router.go.
// It wraps Service for production use, but also accepts an injected
// liveConnector for test isolation (matching the previous API).
type Bridge struct {
	svc     *Service
	apiKey  string
	connect liveConnector // non-nil only in tests
}

// liveConnector abstracts genai.Client.Live.Connect for test injection.
type liveConnector func(ctx context.Context, model string, cfg *genai.LiveConnectConfig) (liveSession, error)

// NewBridge creates a production Bridge backed by a full Service.
func NewBridge(apiKey string) *Bridge {
	svc, err := newServiceFromKey(apiKey)
	if err != nil {
		slog.Error("voicelive: bridge init failed", "err", err)
	}
	return &Bridge{svc: svc, apiKey: apiKey}
}

func newServiceFromKey(apiKey string) (*Service, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY empty")
	}
	return &Service{
		sessions:    make(map[string]*Session),
		apiKey:      apiKey,
	}, nil
}

// Serve upgrades the HTTP connection to WebSocket and runs a voice session.
// This is the single-endpoint path used by the existing /voice/live/ws route.
// It reads a {"type":"start",...} frame inline to get session config,
// then runs the full session lifecycle.
func (b *Bridge) Serve(ctx context.Context, conn *websocket.Conn) {
	// Set TCP_NODELAY to flush small JSON frames immediately.
	if tc, ok := conn.NetConn().(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	sp, err := readStartFrame(conn)
	if err != nil {
		writeErrorFrame(conn, err)
		return
	}

	model := sp.Model
	if model == "" {
		model = DefaultModel
	}

	var client *genai.Client
	var connectFn liveConnector

	if b.connect != nil {
		// Test injection path
		connectFn = b.connect
	} else {
		// Production path — create client from service's API key
		key := b.apiKey
		if b.svc != nil {
			key = b.svc.apiKey
		}
		c, err2 := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:      key,
			HTTPOptions: genai.HTTPOptions{APIVersion: liveAPIVersion},
		})
		if err2 != nil {
			writeErrorFrame(conn, fmt.Errorf("live client: %w", err2))
			return
		}
		client = c
		connectFn = func(ctx context.Context, m string, cfg *genai.LiveConnectConfig) (liveSession, error) {
			return client.Live.Connect(ctx, m, cfg)
		}
	}

	ent := &LiveSession{
		ID:        uuid.New().String(),
		ModelID:   model,
		ModelName: func() string {
			if n, ok := LiveCapableModelIDs[model]; ok {
				return n
			}
			return model
		}(),
		State:     SessionStateIdle,
		CreatedAt: time.Now(),
	}

	// Register in service if available
	sess := newSession(ent, client)
	// Inject test connector into session via a local wrapper
	bridgedSess := &bridgeSession{Session: sess, connectFn: connectFn}
	if b.svc != nil {
		b.svc.mu.Lock()
		b.svc.sessions[ent.ID] = sess
		b.svc.mu.Unlock()
	}

	cfg := SessionConfig{
		VoiceName:   sp.Voice,
		SystemText:  sp.Instructions,
		Temperature: sp.Temperature,
	}
	if err := bridgedSess.run(ctx, conn, cfg); err != nil {
		slog.Error("voicelive: bridge session ended", "err", err)
	}
}

// bridgeSession overrides the connect call to support test injection.
type bridgeSession struct {
	*Session
	connectFn liveConnector
}

func (bs *bridgeSession) run(ctx context.Context, conn *websocket.Conn, cfg SessionConfig) error {
	// Temporarily override the session's client with a test-injectable connector.
	if bs.connectFn != nil {
		// Patch: use a thin wrapper that calls connectFn instead of client.Live.Connect.
		return bs.runWithConnector(ctx, conn, cfg, bs.connectFn)
	}
	return bs.Session.run(ctx, conn, cfg)
}

// runWithConnector runs the session using a custom connector (for tests).
func (bs *bridgeSession) runWithConnector(ctx context.Context, conn *websocket.Conn, cfg SessionConfig, connector liveConnector) error {
	lc, err := bs.Session.buildConfig(cfg, "")
	if err != nil {
		return err
	}
	gs, err := connector(ctx, bs.Session.ent.ModelID, lc)
	if err != nil {
		writeErrorFrame(conn, fmt.Errorf("live connect: %w", err))
		return err
	}
	defer gs.Close()

	if err := waitSetup(ctx, gs); err != nil {
		writeErrorFrame(conn, fmt.Errorf("live setup: %w", err))
		return err
	}

	bs.Session.setState(SessionStateActive)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	outCh := make(chan outMsg, 256)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m, ok := <-outCh:
				if !ok {
					return
				}
				mt := websocket.TextMessage
				if m.binary {
					mt = websocket.BinaryMessage
				}
				_ = conn.WriteMessage(mt, m.data)
			}
		}
	}()

	sendTextCh(ctx, outCh, WSMessage{
		Type: WSTypeSessionState,
		Payload: map[string]interface{}{
			"state": "active", "model_id": bs.Session.ent.ModelID,
			"model": bs.Session.ent.ModelID, "voice": cfg.VoiceName,
		},
	})

	errc := make(chan error, 2)
	var nopHandle atomic.Value
	nopHandle.Store("")
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			mt, data, rerr := conn.ReadMessage()
			if rerr != nil {
				cancel()
				return
			}
			switch mt {
			case websocket.BinaryMessage:
				_ = gs.SendRealtimeInput(genai.LiveRealtimeInput{
					Audio: &genai.Blob{Data: data, MIMEType: "audio/pcm;rate=16000"},
				})
			case websocket.TextMessage:
				var msg WSMessage
				if json.Unmarshal(data, &msg) != nil {
					continue
				}
				switch msg.Type {
				case WSTypeEnd:
					_ = gs.SendRealtimeInput(genai.LiveRealtimeInput{AudioStreamEnd: true})
				case WSTypeVideoFrame:
					bs.Session.handleVideoFrame(gs, msg.Payload)
				}
			}
		}
	}()

	goawayCh := make(chan struct{}, 1)
	go bs.Session.relayGeminiToBrowser(ctx, gs, outCh, &nopHandle, errc, goawayCh)

	select {
	case <-goawayCh:
		return nil
	case relayErr := <-errc:
		return relayErr
	}
}

// StartPayload is the first text frame sent by the browser after WS connects.
type StartPayload struct {
	Model        string  `json:"model"`
	Voice        string  `json:"voice"`
	Temperature  float32 `json:"temperature"`
	Instructions string  `json:"instructions"`
}

func readStartFrame(conn *websocket.Conn) (StartPayload, error) {
	_, data, err := conn.ReadMessage()
	if err != nil {
		return StartPayload{}, fmt.Errorf("read start: %w", err)
	}
	var msg struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return StartPayload{}, fmt.Errorf("start: bad JSON: %w", err)
	}
	if msg.Type != "start" {
		return StartPayload{}, fmt.Errorf("start: expected %q got %q", "start", msg.Type)
	}
	var sp StartPayload
	if len(msg.Payload) > 0 {
		_ = json.Unmarshal(msg.Payload, &sp)
	}
	return sp, nil
}

func writeErrorFrame(conn *websocket.Conn, err error) {
	slog.Error("voicelive error", "err", err)
	data, _ := json.Marshal(WSMessage{
		Type:    WSTypeError,
		Payload: map[string]interface{}{"message": "voice session failed to start"},
	})
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

// atomic.Value alias removed — using sync/atomic directly above

// Service-delegation methods on Bridge — used by router.go REST endpoints.

func (b *Bridge) GetAllPresets() []AgentVoicePreset {
	if b.svc == nil { return SystemPresets() }
	return b.svc.GetAllPresets()
}

func (b *Bridge) CreatePreset(ctx context.Context, req CreatePresetRequest) (*AgentVoicePreset, error) {
	if b.svc == nil { return nil, fmt.Errorf("service not initialized") }
	return b.svc.CreatePreset(ctx, req)
}

func (b *Bridge) CreateSession(ctx context.Context, req CreateSessionRequest) (*LiveSession, error) {
	if b.svc == nil { return nil, fmt.Errorf("service not initialized") }
	return b.svc.CreateSession(ctx, req)
}

func (b *Bridge) ListSessions(ctx context.Context) []LiveSessionSummary {
	if b.svc == nil { return nil }
	return b.svc.ListSessions(ctx)
}

func (b *Bridge) HandleStream(ctx context.Context, sessionID string, conn *websocket.Conn, cfg SessionConfig) error {
	if b.svc == nil { return fmt.Errorf("service not initialized") }
	return b.svc.HandleStream(ctx, sessionID, conn, cfg)
}

func (b *Bridge) EndSession(ctx context.Context, sessionID string) error {
	if b.svc == nil { return fmt.Errorf("service not initialized") }
	return b.svc.EndSession(ctx, sessionID)
}
