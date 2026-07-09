package voicelive

import "time"

// =============================================================================
// Session lifecycle
// =============================================================================

type SessionState string

const (
	SessionStateIdle       SessionState = "idle"
	SessionStateConnecting SessionState = "connecting"
	SessionStateActive     SessionState = "active"
	SessionStatePaused     SessionState = "paused"
	SessionStateEnded      SessionState = "ended"
	SessionStateError      SessionState = "error"
)

type LiveSession struct {
	ID           string       `json:"id"`
	ModelID      string       `json:"model_id"`
	ModelName    string       `json:"model_name"`
	State        SessionState `json:"state"`
	CreatedAt    time.Time    `json:"created_at"`
	EndedAt      *time.Time   `json:"ended_at,omitempty"`
	TotalTokens  int64        `json:"total_tokens"`
	DurationMs   int64        `json:"duration_ms"`
	ErrorMessage string       `json:"error_message,omitempty"`
	// ResumeHandle stores the Gemini SessionResumptionUpdate token for GoAway reconnects.
	ResumeHandle string `json:"-"`
}

type LiveSessionSummary struct {
	ID          string       `json:"id"`
	ModelID     string       `json:"model_id"`
	ModelName   string       `json:"model_name"`
	State       SessionState `json:"state"`
	CreatedAt   time.Time    `json:"created_at"`
	DurationMs  int64        `json:"duration_ms"`
	TotalTokens int64        `json:"total_tokens"`
}

// =============================================================================
// Session creation request
// =============================================================================

type CreateSessionRequest struct {
	ModelID     string  `json:"model_id" binding:"required"`
	VoiceName   string  `json:"voice_name,omitempty"`
	SystemText  string  `json:"system_text,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
}

// =============================================================================
// Agent presets
// =============================================================================

type AgentVoicePreset struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Icon        string  `json:"icon"`
	Description string  `json:"description,omitempty"`
	Instruction string  `json:"instruction"`
	VoiceName   string  `json:"voice_name,omitempty"`
	ModelID     string  `json:"model_id,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
	IsSystem    bool    `json:"is_system"`
}

type CreatePresetRequest struct {
	Name        string  `json:"name" binding:"required"`
	Icon        string  `json:"icon"`
	Description string  `json:"description,omitempty"`
	Instruction string  `json:"instruction" binding:"required"`
	VoiceName   string  `json:"voice_name,omitempty"`
	ModelID     string  `json:"model_id,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
}

// =============================================================================
// Model catalog
// =============================================================================

type LiveModelInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	SupportsLive bool     `json:"supports_live"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// =============================================================================
// WebSocket protocol
// =============================================================================

type WSMessageType = string

const (
	// Client → Server
	WSTypeEnd        WSMessageType = "end"
	WSTypeText       WSMessageType = "text"
	WSTypeVideoFrame WSMessageType = "video_frame"

	// Server → Client
	WSTypeInterrupt    WSMessageType = "interrupt"
	WSTypeTranscript   WSMessageType = "transcript"
	WSTypeToolCall     WSMessageType = "tool_call"
	WSTypeToolResult   WSMessageType = "tool_result"
	WSTypeSessionState WSMessageType = "session_state"
	WSTypeError        WSMessageType = "error"
)

type WSMessage struct {
	Type    WSMessageType          `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// outMsg is queued for sending to the browser.
type outMsg struct {
	binary bool
	data   []byte
}
