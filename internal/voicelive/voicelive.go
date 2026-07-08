// Package voicelive bridges a browser WS connection to the Gemini Live
// API's bidirectional session (google.golang.org/genai's Live.Connect),
// replacing the old push-to-talk record-then-POST flow with a real
// realtime voice conversation: no send button, continuous audio in both
// directions. See docs/superpowers/specs/2026-07-08-voice-live-realtime-design.md
// for the full protocol/architecture rationale this package implements.
package voicelive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gorilla/websocket"
	"google.golang.org/genai"
)

// DefaultModel is the Live-capable model used when a session doesn't pick
// one explicitly. Preview model IDs churn on Google's own schedule (this
// repo already caught one such deprecation for DeepSeek) — verify this
// still resolves with a real key before treating it as a stable default.
const DefaultModel = "gemini-2.5-flash-native-audio-preview-12-2025"

// Voices lists the real prebuilt Gemini voice names shared by the Live
// API and Cloud Text-to-Speech's Gemini TTS voice catalog (verified live
// against ai.google.dev/gemini-api/docs/live and
// docs.cloud.google.com/text-to-speech/docs/gemini-tts — not invented).
var Voices = []string{
	"Achernar", "Achird", "Algenib", "Algieba", "Alnilam", "Aoede", "Autonoe",
	"Callirrhoe", "Charon", "Despina", "Enceladus", "Erinome", "Fenrir",
	"Gacrux", "Iapetus", "Kore", "Laomedeia", "Leda", "Orus", "Pulcherrima",
	"Puck", "Rasalgethi", "Sadachbia", "Sadaltager", "Schedar", "Sulafat",
	"Umbriel", "Vindemiatrix", "Zephyr", "Zubenelgenubi",
}

// StartPayload configures a session — sent once, as the first text frame
// after the WS connects, mirroring the reference implementation's own
// {"type":"start","payload":{...}} wire shape.
type StartPayload struct {
	Model        string  `json:"model"`
	Voice        string  `json:"voice"`
	Temperature  float32 `json:"temperature"`
	Instructions string  `json:"instructions"`
}

type clientMsg struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ServerMsg is a text control/status frame sent to the browser. Actual
// audio (both directions) travels as raw binary WS frames, never
// base64-wrapped in one of these.
type ServerMsg struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type transcriptPayload struct {
	Role    string `json:"role"` // "user" | "agent"
	Text    string `json:"text"`
	IsFinal bool   `json:"isFinal"`
}

type sessionStatePayload struct {
	State string `json:"state"` // "active"
	Model string `json:"model"`
	Voice string `json:"voice"`
}

type errorPayload struct {
	Message string `json:"message"`
}

// liveSession is the subset of *genai.Session this package actually
// calls — an interface so tests can substitute a fake without a real
// API key or network connection, following this repo's own established
// pattern (Phase 5's fake-model tests, the chat-streaming design's
// fake-chunk tests).
type liveSession interface {
	SendRealtimeInput(input genai.LiveRealtimeInput) error
	Receive() (*genai.LiveServerMessage, error)
	Close() error
}

// liveConnector abstracts genai.Client.Live.Connect for the same
// fake-in-tests reason.
type liveConnector func(ctx context.Context, model string, cfg *genai.LiveConnectConfig) (liveSession, error)

// Bridge holds what's needed to open Gemini Live sessions.
type Bridge struct {
	apiKey  string
	connect liveConnector
}

// NewBridge builds a Bridge that connects to the real Gemini Live API
// with apiKey. connect is nil in production use; tests inject a fake.
func NewBridge(apiKey string) *Bridge {
	return &Bridge{apiKey: apiKey, connect: nil}
}

func (b *Bridge) doConnect(ctx context.Context, model string, cfg *genai.LiveConnectConfig) (liveSession, error) {
	if b.connect != nil {
		return b.connect(ctx, model, cfg)
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: b.apiKey,
		// Live.Connect explicitly requires a non-empty APIVersion and
		// rejects the SDK's own default ("v1beta") for the Gemini API
		// backend — verified against the pinned v1.57.0 source's own
		// error message, not assumed from the general API docs.
		HTTPOptions: genai.HTTPOptions{APIVersion: "v1alpha"},
	})
	if err != nil {
		return nil, fmt.Errorf("live client: %w", err)
	}
	session, err := client.Live.Connect(ctx, model, cfg)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// Serve pumps a single browser WS connection through one Gemini Live
// session end to end: reads the start config, connects, blocks for
// SetupComplete, then relays audio/transcripts both ways until the
// client disconnects or Gemini errors. Blocks until the session ends;
// the caller closes conn on return (matches hub.go's serveWS).
func (b *Bridge) Serve(ctx context.Context, conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start, err := readStart(conn)
	if err != nil {
		writeError(conn, err)
		return
	}

	model := start.Model
	if model == "" {
		model = DefaultModel
	}
	temp := start.Temperature

	liveCfg := &genai.LiveConnectConfig{
		ResponseModalities: []genai.Modality{genai.ModalityAudio},
		Temperature:        &temp,
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: start.Voice},
			},
		},
		// Both directions transcribed so the transcript panel shows real
		// text for the user's own words, not just the agent's reply —
		// the gap iteration 12's strict-JSON workaround only patched
		// for the old non-realtime flow.
		InputAudioTranscription:  &genai.AudioTranscriptionConfig{},
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
	}
	if start.Instructions != "" {
		liveCfg.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: start.Instructions}}}
	}

	session, err := b.doConnect(ctx, model, liveCfg)
	if err != nil {
		writeError(conn, fmt.Errorf("live connect: %w", err))
		return
	}
	defer session.Close()

	// Block until SetupComplete before telling the browser it's live —
	// starting mic capture before this arrives wastes the first
	// utterance (design doc §2.4, ported from the reference's own
	// session lifecycle ordering).
	first, err := session.Receive()
	if err != nil {
		writeError(conn, fmt.Errorf("live setup: %w", err))
		return
	}
	if first.SetupComplete == nil {
		writeError(conn, fmt.Errorf("live setup: unexpected first message"))
		return
	}
	writeJSON(conn, ServerMsg{Type: "session_state", Payload: sessionStatePayload{State: "active", Model: model, Voice: start.Voice}})

	errc := make(chan error, 2)
	go relayBrowserToGemini(ctx, conn, session, errc)
	go relayGeminiToBrowser(ctx, conn, session, errc)

	if err := <-errc; err != nil {
		slog.Error("voicelive session ended", "err", err)
	}
	cancel()
}

// relayBrowserToGemini reads the browser's mic frames (binary = raw
// PCM16 16kHz mono) and forwards each as a realtime audio chunk. A text
// {"type":"end"} frame signals the mic was turned off (matches the
// reference's client message vocabulary) — VAD on Gemini's side handles
// turn-taking automatically, no send button or explicit "send" message
// exists in this protocol.
func relayBrowserToGemini(ctx context.Context, conn *websocket.Conn, session liveSession, errc chan<- error) {
	for {
		if ctx.Err() != nil {
			return
		}
		mt, data, err := conn.ReadMessage()
		if err != nil {
			errc <- err
			return
		}
		switch mt {
		case websocket.BinaryMessage:
			if err := session.SendRealtimeInput(genai.LiveRealtimeInput{
				Audio: &genai.Blob{Data: data, MIMEType: "audio/pcm;rate=16000"},
			}); err != nil {
				errc <- err
				return
			}
		case websocket.TextMessage:
			var msg clientMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.Type == "end" {
				if err := session.SendRealtimeInput(genai.LiveRealtimeInput{AudioStreamEnd: true}); err != nil {
					errc <- err
					return
				}
			}
		}
	}
}

// relayGeminiToBrowser drains Gemini's server messages and forwards
// audio (binary), transcripts, interrupts (barge-in), and terminal
// errors/GoAway to the browser as the matching wire-format frame.
func relayGeminiToBrowser(ctx context.Context, conn *websocket.Conn, session liveSession, errc chan<- error) {
	for {
		if ctx.Err() != nil {
			return
		}
		msg, err := session.Receive()
		if err != nil {
			errc <- err
			return
		}

		if msg.GoAway != nil {
			writeJSON(conn, ServerMsg{Type: "session_state", Payload: sessionStatePayload{State: "ending"}})
			errc <- nil
			return
		}

		sc := msg.ServerContent
		if sc == nil {
			continue
		}
		if sc.Interrupted {
			writeJSON(conn, ServerMsg{Type: "interrupt"})
		}
		if sc.InputTranscription != nil && sc.InputTranscription.Text != "" {
			writeJSON(conn, ServerMsg{Type: "transcript", Payload: transcriptPayload{
				Role: "user", Text: sc.InputTranscription.Text, IsFinal: sc.InputTranscription.Finished,
			}})
		}
		if sc.OutputTranscription != nil && sc.OutputTranscription.Text != "" {
			writeJSON(conn, ServerMsg{Type: "transcript", Payload: transcriptPayload{
				Role: "agent", Text: sc.OutputTranscription.Text, IsFinal: sc.OutputTranscription.Finished,
			}})
		}
		if sc.ModelTurn != nil {
			for _, part := range sc.ModelTurn.Parts {
				if part.InlineData != nil && len(part.InlineData.Data) > 0 {
					if err := conn.WriteMessage(websocket.BinaryMessage, part.InlineData.Data); err != nil {
						errc <- err
						return
					}
				}
			}
		}
	}
}

func readStart(conn *websocket.Conn) (StartPayload, error) {
	_, data, err := conn.ReadMessage()
	if err != nil {
		return StartPayload{}, fmt.Errorf("read start: %w", err)
	}
	var msg clientMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		return StartPayload{}, fmt.Errorf("start: invalid JSON: %w", err)
	}
	if msg.Type != "start" {
		return StartPayload{}, fmt.Errorf("start: expected type \"start\", got %q", msg.Type)
	}
	var start StartPayload
	if len(msg.Payload) > 0 {
		if err := json.Unmarshal(msg.Payload, &start); err != nil {
			return StartPayload{}, fmt.Errorf("start: invalid payload: %w", err)
		}
	}
	return start, nil
}

func writeJSON(conn *websocket.Conn, msg ServerMsg) {
	if err := conn.WriteJSON(msg); err != nil {
		slog.Error("voicelive: write failed", "err", err)
	}
}

func writeError(conn *websocket.Conn, err error) {
	slog.Error("voicelive error", "err", err)
	writeJSON(conn, ServerMsg{Type: "error", Payload: errorPayload{Message: "voice session failed to start"}})
}
