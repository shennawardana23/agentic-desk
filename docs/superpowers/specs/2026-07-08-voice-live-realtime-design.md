# Voice Assistant — real-time Gemini Live redesign

**Status:** design only, not yet implemented. Written per explicit user request ("write a design doc first, then build") after two research passes: (1) live docs for Google Cloud TTS and the Gemini Live API, (2) full read of the reference implementation at `archpublicwebsite-mcp` (composables, store, Go backend), (3) direct verification against this repo's own pinned `google.golang.org/genai v1.57.0` source (not the reference repo's — a different module, confirmed to have the same `Live.Connect` surface, but every field name below was checked against *our* pinned version, not assumed from the reference).

## 1. Why this is a real architecture change, not a UI tweak

Today's `VoiceView.vue` is push-to-talk: record → stop → send whole clip → `POST /chat` (multimodal audio, one-shot) → wait → reply. That's why there's a send button and why response feels slow — it's a full HTTP round trip per turn, not a live conversation.

The user wants: no send button, real-time, fast response, persona orb reflecting Gemini's actual Live voices. That requires the **Gemini Live API** — a stateful bidirectional WebSocket (`BidiGenerateContent`), not the request/response `/chat` flow. This is a different product from Cloud Text-to-Speech (confirmed distinct via live docs — Cloud TTS is one-shot synthesis and explicitly "ignores temperature/top_k/top_p"; Live API is the real-time conversational one).

**This is new backend infrastructure**: a WS handler in `cmd/core`, a new `internal/voicelive` package, and a full rewrite of the frontend's audio pipeline (AudioWorklet capture + Web Audio playback, replacing `MediaRecorder`+one-shot POST entirely). Comparable in scope to the original Chat streaming feature (2026-07-07 design doc), not a styling pass.

## 2. Verified facts (not assumed)

### 2.1 This repo's pinned SDK already supports it
`go.mod` pins `google.golang.org/genai v1.57.0`. Checked the **exact pinned version** in the module cache (not a newer one):
- `$(go env GOMODCACHE)/google.golang.org/genai@v1.57.0/live.go` exists.
- `func (r *Live) Connect(ctx context.Context, model string, config *LiveConnectConfig) (*Session, error)` — real, present, verified by `grep` against the file, not the docs.
- `Session` methods: `SendClientContent`, `SendRealtimeInput`, `SendToolResponse`, `Receive() (*LiveServerMessage, error)`, `Close() error`.
- **`LiveConnectConfig.Temperature *float32`** exists directly on the config struct — the reference implementation (`archpublicwebsite-mcp`) parses temperature from a query string but never wires it into `LiveConnectConfig` (a real gap in that codebase). Our version doesn't need that workaround — wire it directly.
- Relevant `LiveConnectConfig` fields confirmed present: `ResponseModalities []Modality`, `Temperature *float32`, `SpeechConfig *SpeechConfig`, `SystemInstruction *Content`, `InputAudioTranscription`/`OutputAudioTranscription *AudioTranscriptionConfig`, `SessionResumption *SessionResumptionConfig`, `ContextWindowCompression *ContextWindowCompressionConfig`, `Tools []*Tool`, `ExplicitVADSignal *bool`.
- `LiveSendRealtimeInputParameters` (used to build the value passed to `SendRealtimeInput`): `Audio *Blob`, `Video *Blob`, `Text string`, `AudioStreamEnd bool`, `ActivityStart`/`ActivityEnd`.
- `LiveServerMessage`: `SetupComplete`, `ServerContent`, `ToolCall`, `GoAway`, `SessionResumptionUpdate`, `VoiceActivity`.

### 2.2 Gemini Live API protocol (ai.google.dev/gemini-api/docs/live, /live-guide)
- Audio in: raw 16-bit PCM, **16kHz**, mono, little-endian (`audio/pcm;rate=16000`). Audio out: raw 16-bit PCM, **24kHz**, mono. (Docs note the API will resample non-16kHz input, but sending native 16kHz avoids that cost.)
- Automatic server-side VAD by default — no client VAD code needed for v1. Manual VAD available later via `ActivityStart`/`ActivityEnd` if needed (`ExplicitVADSignal: true`).
- Session duration caps without extensions: 15 min audio-only, 2 min audio+video. `ContextWindowCompression{SlidingWindow: &SlidingWindow{}}` removes the cap — **out of scope for v1** (YAGNI: a desktop dev tool's voice sessions are short; add if it becomes a real complaint).
- No official Go quickstart in the docs (only Python/JS/ADK) — this repo's own pinned SDK is the actual proof Go works, not the docs.

### 2.3 Google Cloud TTS voices (docs.cloud.google.com/text-to-speech/docs/gemini-tts) — this is the voice list Gemini Live also draws from
28 voices: Achernar, Achird, Algenib, Algieba, Alnilam, **Aoede**, **Autonoe**, **Callirrhoe**, **Charon**, Despina, **Enceladus**, Erinome, **Fenrir**, Gacrux, **Iapetus**, **Kore**, Laomedeia, **Leda**, **Orus**, Pulcherrima, **Puck**, Rasalgethi, Sadachbia, Sadaltager, Schedar, Sulafat, **Umbriel**, Vindemiatrix, **Zephyr**, **Zubenelgenubi**. (Bold = also shown in the user's own reference screenshot of the Live config voice picker — confirms this is the right list, not the Cloud TTS product's separate/possibly-differently-versioned list.)

Model IDs actually used for Live (from the reference Go backend's `LiveCapableModelIDs`, cross-referenced against "explicitly documented as Live-capable" — do not invent names): `gemini-2.5-flash-native-audio-preview-12-2025` is the safest v1 default. `gemini-3.1-flash-live-preview` (what the reference and the user's screenshot show as "Gemini 3.1 Flash Live Preview") — **verify this exact string resolves via a live API call before shipping it as default**, preview model IDs churn; don't hardcode from a screenshot alone.

### 2.4 Reference implementation's exact wire protocol (archpublicwebsite-mcp)
Full file list read: `ui/src/composables/useAgentLiveWs.js`, `ui/src/stores/agentlive.js`, `ui/src/lib/voice-live-sdk.js`, `ui/src/composables/useMediaStream.js`, `internal/entity/agentlive.go`, `internal/controller/agentlive/controller.go`, `internal/service/agentlive/{live_service,live_session,audio_processor,tools}.go`.

- WS frames: **binary = raw PCM16 audio bytes**, **text = JSON `{"type": ..., "payload": {...}}`**.
- Client→server types: `start`, `end`, `text`, `pause`, `resume`.
- Server→client types: `transcript` (`{role, text, timestamp, is_final}`), `session_state` (`{id, state, model_id, model_name, duration_ms}`), `interrupt` (barge-in), `error` (`{message}`).
- Client audio capture: `getUserMedia({audio:{sampleRate:16000, channelCount:1, echoCancellation:true, noiseSuppression:true, autoGainControl:true}})` → **AudioWorklet** (not `MediaRecorder` — `MediaRecorder` produces compressed containers, wrong for raw PCM streaming) → binary ArrayBuffer frames over the WS.
- Client audio playback: `AudioContext({sampleRate:24000})`, each incoming chunk decoded Int16→Float32→`AudioBuffer`, scheduled gapless via a running `nextPlayTime` cursor (`source.start(nextPlayTime); nextPlayTime += buffer.duration`) — this is what makes it sound continuous instead of clicking between chunks.
- Barge-in: on Gemini's `ServerContent.Interrupted`, flush by zeroing gain then recreating the `AudioContext` (Web Audio has no way to cancel already-scheduled future buffer sources).
- Backend session lifecycle: connect → block on `Receive()` until `SetupComplete` (only then tell the browser "active" and let it start capturing mic — this ordering matters, starting mic before setup completes wastes the first utterance) → three loops (drain-to-browser, Gemini→browser, browser→Gemini) → first error/EOF tears down all three via context cancel.

## 3. Scope for v1 (what we're building vs. explicitly deferring)

**Building now:**
- Real-time bidirectional voice: WS backend proxying `Live.Connect`, PCM16 both directions, automatic server-side VAD, barge-in.
- Config panel: Model select, Voice select (real 28-voice list), Temperature slider (0–2, step 0.1) — matching the user's reference screenshots exactly.
- Persona orb reactive to live audio level + connection state (already exists in some form — restyle to match the reference's organic-blob approach, or keep the current gradient orb with `--state-scale`; the org's `--accent`/OKLCH token system stays, this is not a copy of the reference's raw hex-color CSS).
- No send button. Start/mic/end controls only, per the reference's `voice-controls__primary` pattern (camera toggle deferred, see below).
- Transcript panel spacing (already fixed this session) carries over unchanged.

**Explicitly deferred (YAGNI for v1, revisit if actually needed):**
- Video/camera input (`toggleCamera`, video frames) — this app has no existing camera feature anywhere; adding it here would be scope creep beyond "make voice real-time."
- Function/tool calling during voice (`creative_tool`, image/music/video generation cards) — this app's chat flow itself has zero tools defined yet (per the 2026-07-07 chat design doc's own note); voice getting tools before chat does would be backwards.
- `ContextWindowCompression`/unlimited session length — 15-minute default cap is fine for a dev tool's voice sessions.
- Session resumption after server-initiated disconnect — v1 just surfaces the error and lets the user click Start again.
- Multiple system agent presets (the reference's preset dropdown/create-agent modal) — this app has one persona config (Instructions textarea), matching what already exists; presets are a separate feature request if wanted later.

## 4. Proposed architecture

### 4.1 Backend — `internal/voicelive` (new package)
- `Session` type wrapping a `*genai.Session` (from `Live.Connect`) + the client WS connection, mirroring the reference's 3-goroutine pattern (drain-to-client, Gemini→client, client→Gemini), context-cancel teardown on first error.
- `Config` struct: `ModelID`, `VoiceName`, `Temperature float32`, `SystemInstruction string`.
- Builds `LiveConnectConfig` with: `ResponseModalities: []genai.Modality{genai.ModalityAudio}`, `Temperature: &cfg.Temperature` (real fix vs. the reference's gap), `SpeechConfig: &genai.SpeechConfig{VoiceConfig: &genai.VoiceConfig{PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: cfg.VoiceName}}}`, `SystemInstruction`, `InputAudioTranscription`/`OutputAudioTranscription: &genai.AudioTranscriptionConfig{}` (so the transcript panel gets real text for both sides, fixing the current "user's own words never shown" gap for good, not just the strict-JSON workaround from iteration 12).
- `internal/api`: new `GET /voice/live/ws` route (gorilla/websocket — already a dependency, currently only used by `hub.go` for agent-log broadcast, unrelated), upgrades and hands off to `voicelive.Session`.
- Wire format: same as the reference (binary = PCM16, text = JSON `{type, payload}`) — no reason to invent a different one, and it's already proven correct.

### 4.2 Frontend — `VoiceView.vue` rewrite + two new composables
- `useVoiceLiveWs.js`: WS connect/send/receive, mirroring `useAgentLiveWs.js`'s message-type handling (transcript, session_state, interrupt, error) minus the tool-call/video branches (deferred, §3).
- `useVoiceCapture.js`: `getUserMedia` at 16kHz mono + AudioWorklet → binary frames out. Replaces the current `MediaRecorder`-based `useMediaStream`-equivalent entirely (that approach cannot produce raw PCM16 — it's built for one-shot blob capture, wrong primitive for streaming).
- Playback: `AudioContext({sampleRate:24000})` + gapless scheduling, as in §2.4 — this is the part most likely to need real device testing (this sandbox has no audio output to verify against; flag as untested-here in the eventual PR).
- Config panel: replace the current `speakAloud` checkbox‑only sidebar with Model/Voice `<select>`s + Temperature `<input type=range>`, matching the reference's plain-control style but using this project's own token CSS, not copying hex colors.
- Remove: the mic-record → `sendVoiceMessage` → `/chat` flow, the send button, `cancelTalk`'s "discard segment before onstop" logic (no longer applicable — there's no discrete segment to discard, the stream is continuous).

### 4.3 What does NOT change
- `internal/orchestrator/chat.go`'s text chat flow — untouched, this is a parallel path, not a replacement.
- The Skill/Prompt Catalog, Knowledge Graph, Tasks — unrelated.

## 5. Open questions to resolve before/during implementation (not blocking the doc, but no code should guess these)

1. **Exact default model ID** — verify `gemini-2.5-flash-native-audio-preview-12-2025` (or whatever is current at implementation time) actually connects with a real key before hardcoding it as the default; preview IDs deprecate on Google's own schedule (this repo already caught one such deprecation for DeepSeek in 2026-07-07 — same discipline applies here).
2. **AudioWorklet browser support inside Wails' WKWebView** — the existing `VoiceView.vue` already notes WKWebView has no Web Speech recognition API; AudioWorklet is a *different*, more widely-supported API, but this needs a real device check, not an assumption, before committing to it as the only capture path.
3. Whether `Temperature` on a Live session behaves like generation temperature (creativity) as documented, given it's a real-time model — worth a quick live test once built, not just trusting the field's doc comment.

## 6. Testing approach

Following this repo's own established pattern (Phase 5's fake-model offline tests, the 2026-07-07 streaming design's fake-chunk tests): the WS message-shape logic and session state machine can be unit-tested offline with a fake `genai.Session`-shaped interface (define a small interface `liveSession` with just the methods `internal/voicelive` actually calls, fake it in tests) — no real API key needed to prove the plumbing. The actual audio round-trip (capture → Gemini → playback) can only be verified with a real key and real hardware, same standing gap as every other live-API feature in this project.
