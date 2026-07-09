package voicelive

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/genai"
)

const maxReconnects = 5

// SessionConfig holds runtime configuration for a live session.
type SessionConfig struct {
	VoiceName   string
	SystemText  string
	Temperature float32
}

// liveSession is the subset of *genai.Session this package uses.
// Interface enables test injection without a real API key.
type liveSession interface {
	SendRealtimeInput(input genai.LiveRealtimeInput) error
	SendToolResponse(params genai.LiveSendToolResponseParameters) error
	Receive() (*genai.LiveServerMessage, error)
	Close() error
}

// Session manages one live conversation from creation through teardown.
type Session struct {
	ent      *LiveSession
	client   *genai.Client
	toolExec *ToolExecutor

	mu     sync.Mutex
	closed bool
}

func newSession(ent *LiveSession, client *genai.Client) *Session {
	return &Session{
		ent:      ent,
		client:   client,
		toolExec: newToolExecutor(),
	}
}

func (s *Session) entity() LiveSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.ent
}

func (s *Session) setState(state SessionState) {
	s.mu.Lock()
	s.ent.State = state
	s.mu.Unlock()
}

func (s *Session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	now := time.Now()
	s.ent.State = SessionStateEnded
	s.ent.EndedAt = &now
	s.ent.DurationMs = now.Sub(s.ent.CreatedAt).Milliseconds()
}

// run starts the bidirectional audio bridge. Matches reference Session.Run() sequence:
//  1. Connect to Gemini Live
//  2. Wait for SetupComplete
//  3. Signal browser "active"
//  4. Relay audio/video/text both ways
//  5. Handle GoAway with reconnect (up to maxReconnects)
func (s *Session) run(ctx context.Context, conn *websocket.Conn, cfg SessionConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer s.close()

	// outCh decouples Gemini receive from browser WS writes.
	// Buffer 256 absorbs audio bursts without blocking the receive loop.
	outCh := make(chan outMsg, 256)
	var writeMu sync.Mutex

	// Sender goroutine — drains outCh → browser WebSocket for the session lifetime.
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
				writeMu.Lock()
				_ = conn.WriteMessage(mt, m.data)
				writeMu.Unlock()
			}
		}
	}()

	// browserSess is the current Gemini session. Swapped atomically on GoAway reconnect
	// so the single browser→Gemini goroutine always forwards to the active session.
	var browserSess atomic.Value // liveSession

	// Single browser→Gemini relay goroutine — lives for the full browser WS connection.
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			mt, data, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
			gs, _ := browserSess.Load().(liveSession)
			if gs == nil {
				continue
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
					s.handleVideoFrame(gs, msg.Payload)
				case WSTypeText:
					if text, _ := msg.Payload["content"].(string); text != "" {
						_ = gs.SendRealtimeInput(genai.LiveRealtimeInput{Text: text})
					}
				}
			}
		}
	}()

	// resumeHandle updated by relayGeminiToBrowser on SessionResumptionUpdate.
	var resumeHandle atomic.Value
	resumeHandle.Store("")

	for attempt := 0; attempt <= maxReconnects; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		handle, _ := resumeHandle.Load().(string)
		lc, err := s.buildConfig(cfg, handle)
		if err != nil {
			return err
		}

		gs, err := s.client.Live.Connect(ctx, s.ent.ModelID, lc)
		if err != nil {
			return fmt.Errorf("live connect: %w", err)
		}

		if err := waitSetup(ctx, gs); err != nil {
			gs.Close()
			return fmt.Errorf("live setup: %w", err)
		}

		browserSess.Store(liveSession(gs))
		s.setState(SessionStateActive)
		sendTextCh(ctx, outCh, WSMessage{
			Type: WSTypeSessionState,
			Payload: map[string]interface{}{
				"state": "active", "model_id": s.ent.ModelID,
				"id": s.ent.ID,
			},
		})

		if attempt > 0 {
			slog.Info("voicelive: GoAway reconnect active", "session", s.ent.ID, "attempt", attempt)
		}

		goawayCh := make(chan struct{}, 1)
		errc := make(chan error, 1)
		go s.relayGeminiToBrowser(ctx, gs, outCh, &resumeHandle, errc, goawayCh)

		select {
		case <-goawayCh:
			gs.Close()
			browserSess.Store(liveSession(nil))
			slog.Info("voicelive: GoAway — reconnecting", "session", s.ent.ID, "attempt", attempt+1)
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue

		case relayErr := <-errc:
			gs.Close()
			if relayErr != nil {
				slog.Error("voicelive: session ended with error", "session", s.ent.ID, "err", relayErr)
			}
			cancel()
			return relayErr
		}
	}

	sendTextCh(ctx, outCh, WSMessage{
		Type:    WSTypeError,
		Payload: map[string]interface{}{"message": "Session expired. Please start a new conversation."},
	})
	return nil
}

// buildConfig constructs the LiveConnectConfig.
// Key features: ContextWindowCompression (unlimited duration), SessionResumption,
// GoogleSearch + creative_tool, both transcriptions.
func (s *Session) buildConfig(cfg SessionConfig, resumeHandle string) (*genai.LiveConnectConfig, error) {
	lc := &genai.LiveConnectConfig{
		ResponseModalities:       []genai.Modality{genai.ModalityAudio},
		// OutputAudioTranscription only — we don’t need to transcribe the user’s
		// input; removing InputAudioTranscription reduces Gemini’s per-chunk
		// processing overhead and shaves latency off first-word response.
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
		Tools:                    BuildLiveTools(),
		// Context window compression enables unlimited session duration.
		// Without this: audio-only sessions cap at ~15 min.
		// TargetTokens=32768 is explicit — empty SlidingWindow{} sends 0 which is invalid.
		ContextWindowCompression: &genai.ContextWindowCompressionConfig{
			TriggerTokens: genai.Ptr[int64](40960),
			SlidingWindow: &genai.SlidingWindow{
				TargetTokens: genai.Ptr[int64](32768),
			},
		},
		// Session resumption for transparent GoAway reconnects.
		SessionResumption: &genai.SessionResumptionConfig{Handle: resumeHandle},
	}

	if cfg.VoiceName != "" {
		lc.SpeechConfig = &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: cfg.VoiceName},
			},
		}
	}

	systemText := cfg.SystemText
	if systemText == "" {
		systemText = "You are a helpful, warm, concise voice assistant. Listen carefully, respond naturally. Keep answers short and conversational unless asked for detail."
	}
	lc.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: systemText}}}

	if cfg.Temperature > 0 {
		lc.Temperature = &cfg.Temperature
	}

	return lc, nil
}

// relayGeminiToBrowser drains one Gemini session into outCh.
func (s *Session) relayGeminiToBrowser(
	ctx context.Context,
	gs liveSession,
	outCh chan outMsg,
	resumeHandle *atomic.Value,
	errc chan<- error,
	goawayCh chan<- struct{},
) {
	for {
		if ctx.Err() != nil {
			return
		}
		msg, err := gs.Receive()
		if err != nil {
			errc <- err
			return
		}
		if msg == nil {
			continue
		}

		if msg.GoAway != nil {
			slog.Info("voicelive: GoAway", "timeLeft", msg.GoAway.TimeLeft, "session", s.ent.ID)
			select {
			case goawayCh <- struct{}{}:
			default:
			}
			return
		}

		if msg.SessionResumptionUpdate != nil && msg.SessionResumptionUpdate.NewHandle != "" {
			resumeHandle.Store(msg.SessionResumptionUpdate.NewHandle)
		}

		if msg.ToolCall != nil {
			s.notifyToolCalls(ctx, outCh, msg.ToolCall)
			go func(tc *genai.LiveServerToolCall) {
				handleToolCalls(ctx, gs, tc, s.toolExec, outCh)
			}(msg.ToolCall)
		}

		if msg.UsageMetadata != nil {
			s.mu.Lock()
			if msg.UsageMetadata.TotalTokenCount > 0 {
				s.ent.TotalTokens = int64(msg.UsageMetadata.TotalTokenCount)
			}
			s.mu.Unlock()
		}

		sc := msg.ServerContent
		if sc == nil {
			continue
		}

		if sc.Interrupted {
			s.handleInterrupt(outCh)
		}

		// Audio first — browser schedules playback before Vue re-renders transcript.
		if sc.ModelTurn != nil {
			for _, part := range sc.ModelTurn.Parts {
				if part.InlineData != nil && len(part.InlineData.Data) > 0 &&
					strings.HasPrefix(part.InlineData.MIMEType, "audio/") {
					select {
					case outCh <- outMsg{binary: true, data: part.InlineData.Data}:
					default:
					}
				}
				if part.Text != "" {
					sendTextCh(ctx, outCh, WSMessage{
						Type: WSTypeTranscript,
						Payload: map[string]interface{}{
							"role": "agent", "text": part.Text,
							"timestamp": time.Now().UnixMilli(), "is_final": false,
						},
					})
				}
			}
		}

		if sc.InputTranscription != nil && sc.InputTranscription.Text != "" {
			sendTextCh(ctx, outCh, WSMessage{
				Type: WSTypeTranscript,
				Payload: map[string]interface{}{
					"role": "user", "text": sc.InputTranscription.Text,
					"timestamp": time.Now().UnixMilli(), "is_final": sc.InputTranscription.Finished,
				},
			})
		}
		if sc.OutputTranscription != nil && sc.OutputTranscription.Text != "" {
			sendTextCh(ctx, outCh, WSMessage{
				Type: WSTypeTranscript,
				Payload: map[string]interface{}{
					"role": "agent", "text": sc.OutputTranscription.Text,
					"timestamp": time.Now().UnixMilli(), "is_final": sc.OutputTranscription.Finished,
				},
			})
		}
		if sc.TurnComplete {
			// Seal last agent transcript entry (isFinal=true).
			sendTextCh(ctx, outCh, WSMessage{
				Type: WSTypeTranscript,
				Payload: map[string]interface{}{
					"role": "agent", "text": "",
					"timestamp": time.Now().UnixMilli(), "is_final": true,
				},
			})
		}
	}
}

// handleInterrupt drains buffered audio on barge-in, sends interrupt to browser.
func (s *Session) handleInterrupt(outCh chan outMsg) {
	drained := 0
	for {
		select {
		case m, ok := <-outCh:
			if !ok {
				goto done
			}
			if m.binary {
				drained++
			} else {
				select {
				case outCh <- m:
				default:
				}
			}
		default:
			goto done
		}
	}
done:
	if drained > 0 {
		slog.Debug("voicelive: barge-in drained audio", "chunks", drained, "session", s.ent.ID)
	}
	data, _ := json.Marshal(WSMessage{
		Type:    WSTypeInterrupt,
		Payload: map[string]interface{}{"timestamp": time.Now().UnixMilli()},
	})
	// Blocking send — interrupt must never be dropped.
	outCh <- outMsg{data: data}
}

// handleVideoFrame decodes a base64 JPEG and forwards to Gemini via Video field.
// Uses SendRealtimeInput.Video (NOT .Media) — per BidiGenerateContent protocol.
func (s *Session) handleVideoFrame(gs liveSession, payload map[string]interface{}) {
	data, _ := payload["data"].(string)
	mime, _ := payload["mime_type"].(string)
	if data == "" {
		return
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	imgBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		slog.Debug("voicelive: bad video frame", "err", err)
		return
	}
	_ = gs.SendRealtimeInput(genai.LiveRealtimeInput{
		Video: &genai.Blob{Data: imgBytes, MIMEType: mime},
	})
}

// notifyToolCalls sends tool_call notifications to browser for UI indicators.
func (s *Session) notifyToolCalls(ctx context.Context, outCh chan outMsg, tc *genai.LiveServerToolCall) {
	for _, fc := range tc.FunctionCalls {
		sendTextCh(ctx, outCh, WSMessage{
			Type: WSTypeToolCall,
			Payload: map[string]interface{}{
				"id": fc.ID, "name": fc.Name, "args": fc.Args,
			},
		})
	}
}

// waitSetup drains Gemini messages until SetupComplete.
func waitSetup(ctx context.Context, gs liveSession) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg, err := gs.Receive()
		if err != nil {
			return err
		}
		if msg != nil && msg.SetupComplete != nil {
			return nil
		}
	}
}

// sendTextCh marshals msg and sends it to outCh (non-blocking).
func sendTextCh(ctx context.Context, outCh chan outMsg, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case outCh <- outMsg{data: data}:
	default:
	}
}
