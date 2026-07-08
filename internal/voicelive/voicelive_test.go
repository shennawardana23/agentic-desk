package voicelive

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/genai"
)

// fakeSession is a hand-rolled liveSession — no real API key or network
// connection needed to prove the relay plumbing works, matching this
// repo's established offline-fake-model testing pattern.
type fakeSession struct {
	mu       sync.Mutex
	sent     []genai.LiveRealtimeInput
	toRecv   []*genai.LiveServerMessage
	recvIdx  int
	closed   bool
	recvGate chan struct{} // closed to allow Receive to proceed past setup
}

func (f *fakeSession) SendRealtimeInput(input genai.LiveRealtimeInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, input)
	return nil
}

func (f *fakeSession) Receive() (*genai.LiveServerMessage, error) {
	f.mu.Lock()
	idx := f.recvIdx
	f.recvIdx++
	f.mu.Unlock()
	if idx >= len(f.toRecv) {
		<-f.recvGate // block forever until the test tears the server down
		return nil, io.EOF
	}
	return f.toRecv[idx], nil
}

func (f *fakeSession) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func newTestServer(t *testing.T, fake *fakeSession) (*httptest.Server, *websocket.Conn) {
	t.Helper()
	b := &Bridge{apiKey: "unused", connect: func(_ context.Context, _ string, _ *genai.LiveConnectConfig) (liveSession, error) {
		return fake, nil
	}}

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		b.Serve(context.Background(), conn)
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}
	return srv, client
}

func TestServe_RelaysAudioAndTranscriptFromGemini(t *testing.T) {
	fake := &fakeSession{
		recvGate: make(chan struct{}),
		toRecv: []*genai.LiveServerMessage{
			{SetupComplete: &genai.LiveServerSetupComplete{SessionID: "s1"}},
			{ServerContent: &genai.LiveServerContent{
				OutputTranscription: &genai.Transcription{Text: "hello there", Finished: true},
				ModelTurn: &genai.Content{Parts: []*genai.Part{
					{InlineData: &genai.Blob{Data: []byte{1, 2, 3, 4}, MIMEType: "audio/pcm;rate=24000"}},
				}},
			}},
		},
	}
	defer close(fake.recvGate)

	srv, client := newTestServer(t, fake)
	defer srv.Close()
	defer client.Close()

	if err := client.WriteJSON(clientMsg{Type: "start", Payload: mustJSON(t, StartPayload{Voice: "Kore", Temperature: 0.8})}); err != nil {
		t.Fatalf("write start: %v", err)
	}

	client.SetReadDeadline(time.Now().Add(2 * time.Second))

	var gotSessionState, gotTranscript bool
	var gotAudio []byte
	for i := 0; i < 3; i++ {
		mt, data, err := client.ReadMessage()
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if mt == websocket.BinaryMessage {
			gotAudio = data
			continue
		}
		var msg ServerMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		switch msg.Type {
		case "session_state":
			gotSessionState = true
		case "transcript":
			gotTranscript = true
		}
	}

	if !gotSessionState {
		t.Error("expected a session_state frame")
	}
	if !gotTranscript {
		t.Error("expected a transcript frame")
	}
	if string(gotAudio) != "\x01\x02\x03\x04" {
		t.Errorf("expected audio bytes [1 2 3 4], got %v", gotAudio)
	}
}

func TestServe_ForwardsMicAudioAsRealtimeInput(t *testing.T) {
	fake := &fakeSession{
		recvGate: make(chan struct{}),
		toRecv: []*genai.LiveServerMessage{
			{SetupComplete: &genai.LiveServerSetupComplete{}},
		},
	}
	defer close(fake.recvGate)

	srv, client := newTestServer(t, fake)
	defer srv.Close()
	defer client.Close()

	if err := client.WriteJSON(clientMsg{Type: "start", Payload: mustJSON(t, StartPayload{Voice: "Kore"})}); err != nil {
		t.Fatalf("write start: %v", err)
	}
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := client.ReadMessage(); err != nil { // consume session_state
		t.Fatalf("read session_state: %v", err)
	}

	if err := client.WriteMessage(websocket.BinaryMessage, []byte{9, 9, 9}); err != nil {
		t.Fatalf("write audio: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		fake.mu.Lock()
		n := len(fake.sent)
		fake.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.sent) != 1 {
		t.Fatalf("expected 1 realtime input sent to Gemini, got %d", len(fake.sent))
	}
	if fake.sent[0].Audio == nil || string(fake.sent[0].Audio.Data) != "\x09\x09\x09" {
		t.Errorf("expected audio bytes [9 9 9], got %+v", fake.sent[0].Audio)
	}
}

func TestServe_RejectsNonStartFirstMessage(t *testing.T) {
	fake := &fakeSession{recvGate: make(chan struct{})}
	defer close(fake.recvGate)

	srv, client := newTestServer(t, fake)
	defer srv.Close()
	defer client.Close()

	if err := client.WriteJSON(clientMsg{Type: "end"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg ServerMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "error" {
		t.Errorf("expected an error frame, got %q", msg.Type)
	}
}

func TestServe_ConnectFailureSurfacesAsError(t *testing.T) {
	b := &Bridge{apiKey: "unused", connect: func(_ context.Context, _ string, _ *genai.LiveConnectConfig) (liveSession, error) {
		return nil, errors.New("boom")
	}}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		b.Serve(context.Background(), conn)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	if err := client.WriteJSON(clientMsg{Type: "start", Payload: mustJSON(t, StartPayload{Voice: "Kore"})}); err != nil {
		t.Fatalf("write start: %v", err)
	}
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg ServerMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "error" {
		t.Errorf("expected an error frame, got %q", msg.Type)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
