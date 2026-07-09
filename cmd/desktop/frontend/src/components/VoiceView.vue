<script setup>
/**
 * VoiceView.vue — Gemini Live realtime voice assistant.
 * Architecture: Pinia store (useVoiceLiveStore) — session lifecycle,
 * presets, model catalog, transcript, tool pipeline nodes — all mirroring
 * archpublicwebsite-mcp/ui/src/views/AgentLive.vue infra.
 * UI styling: our own (custom orb, sidebar, controls).
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { HotkeyStatus } from '../../wailsjs/go/main/App'
import { createCapture, createPlayback } from '../lib/voiceLive'
import { useCoreStore } from '../stores/core'
import { useVoiceLiveStore } from '../stores/voicelive'

const core = useCoreStore()
const vlStore = useVoiceLiveStore()

// ── Global hotkey status ───────────────────────────────────────────────────
const hotkeyActive = ref(false)
onMounted(async () => {
  try { hotkeyActive.value = await HotkeyStatus() } catch {}
  // Poll once more after 3s in case the hotkey registered just after startup
  setTimeout(async () => {
    try { hotkeyActive.value = await HotkeyStatus() } catch {}
  }, 3000)
})

// ── Agent presets — from Pinia store (fetched from /api/agent-live/presets) ──
// presetIconSvg: returns inline SVG path data for each preset icon name
const PRESET_SVG = {
  'bot':         '<path d="M12 2a2 2 0 0 1 2 2v1h2a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h2V4a2 2 0 0 1 2-2z"/><circle cx="9" cy="11" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="11" r="1" fill="currentColor" stroke="none"/><path d="M8 15h8"/>',
  'languages':   '<path d="M5 8l6 6"/><path d="m4 14 6-6 2-3"/><path d="M2 5h12"/><path d="M7 2h1"/><path d="m22 22-5-10-5 10"/><path d="M14 18h6"/>',
  'palette':     '<circle cx="13.5" cy="6.5" r=".5" fill="currentColor" stroke="none"/><circle cx="17.5" cy="10.5" r=".5" fill="currentColor" stroke="none"/><circle cx="8.5" cy="7.5" r=".5" fill="currentColor" stroke="none"/><circle cx="6.5" cy="12.5" r=".5" fill="currentColor" stroke="none"/><path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.926 0 1.648-.746 1.648-1.688 0-.437-.18-.835-.437-1.125-.29-.289-.438-.652-.438-1.125a1.64 1.64 0 0 1 1.668-1.668h1.996c3.051 0 5.555-2.503 5.555-5.554C21.965 6.012 17.461 2 12 2z"/>',
  'headphones':  '<path d="M3 14h3a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-7a9 9 0 0 1 18 0v7a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2h3"/>',
  'gamepad-2':   '<line x1="6" x2="10" y1="11" y2="11"/><line x1="8" x2="8" y1="9" y2="13"/><line x1="15" x2="15.01" y1="12" y2="12"/><line x1="17" x2="17.01" y1="10" y2="10"/><path d="M6 5H4a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/><rect x="6" y="3" width="12" height="4" rx="2"/>',
  'heart-pulse': '<path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/><path d="M3.22 12H9.5l.5-1 2 4 .5-2 2 2h6.28"/>',
  'smile':       '<circle cx="12" cy="12" r="10"/><path d="M8 13s1.5 2 4 2 4-2 4-2"/><line x1="9" x2="9.01" y1="9" y2="9" stroke-width="3" stroke-linecap="round"/><line x1="15" x2="15.01" y1="9" y2="9" stroke-width="3" stroke-linecap="round"/>',
  'music':       '<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>',
  'pen':         '<path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/>',
}
function presetIconSvg(icon) {
  return PRESET_SVG[icon] || PRESET_SVG['bot']
}

const showVoiceMenu = ref(false)
const showPresetMenu = ref(false)
const presetTriggerEl = ref(null)
const transcriptEl = ref(null)
const barsLeft = ref(null)

// Preset menu: positioned with fixed coords from trigger's getBoundingClientRect
// so it renders outside the sidebar's overflow:auto container without clipping.
const presetMenuStyle = computed(() => {
  if (!presetTriggerEl.value) return {}
  const r = presetTriggerEl.value.getBoundingClientRect()
  return {
    position: 'fixed',
    top: `${r.bottom + 4}px`,
    left: `${r.left}px`,
    width: `${r.width}px`,
    zIndex: 9999,
  }
})
const barsRight = ref(null)
const videoPreviewEl = ref(null)
const videoCanvasEl = ref(null)

function selectPreset(id) {
  vlStore.applyPreset(id)
  showPresetMenu.value = false
}

function onDocClick(e) {
  if (!e.target.closest('.vl__voice-instruction--preset')) showPresetMenu.value = false
  if (!e.target.closest('.vl__voice-persona'))  showVoiceMenu.value = false
}
onMounted(() => document.addEventListener('click', onDocClick, true))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick, true))

// ── Voice config — from Pinia store ───────────────────────────────────────────
const FALLBACK_VOICES = [
  'Puck','Charon','Kore','Fenrir','Aoede','Leda','Orus','Zephyr',
  'Umbriel','Callirrhoe','Autonoe','Enceladus','Iapetus','Despina',
  'Erinome','Algieba','Rasalhague','Laomedeia','Achernar','Sulafat',
  'Schedar','Gacrux','Pulcherrima','Achird','Zubenelgenubi',
  'Vindemiatrix','Sadachbia','Sadaltager','Sheliak',
]
const voices = ref(FALLBACK_VOICES)
const configError = ref('')
const configLoading = ref(true)

// Two-way bindings into the Pinia store (persisted via store actions)
const voiceName = computed({
  get: () => vlStore.voiceName,
  set: v => { vlStore.voiceName = v; localStorage.setItem('vl-voice', v) },
})
const temperature = computed({
  get: () => vlStore.temperature,
  set: v => { vlStore.temperature = v; localStorage.setItem('vl-temp', String(v)) },
})
const instructions = computed({
  get: () => vlStore.instructions,
  set: v => { vlStore.instructions = v; localStorage.setItem('vl-instructions', v) },
})
const defaultModel = computed(() => vlStore.selectedModelId)

async function loadConfig() {
  configLoading.value = true
  try {
    await Promise.all([vlStore.fetchModels(), vlStore.fetchPresets()])
    // Also fetch legacy /voice/live/config for voice list
    try {
      const r = await fetch(`${core.baseUrl}/voice/live/config`)
      if (r.ok) { const d = await r.json(); if (d.voices?.length) voices.value = d.voices }
    } catch {}
    if (!voices.value.includes(voiceName.value)) voiceName.value = voices.value[0]
  } catch (e) { configError.value = e.message }
  finally { configLoading.value = false }
}

// ── Session state ─────────────────────────────────────────────────────────────
const connectionState = ref('idle') // idle | connecting | active | error
const isStarting = ref(false)
const isMuted = ref(false)
const speaking = ref(false)
const audioLevel = ref(0)
const elapsed = ref(0)
const sessionError = ref('')
// transcript from store
const messages = computed(() => vlStore.transcript)


// Noise-floor calibration — now computed IN the worklet (chunk-accurate).
// These module-level vars are kept for the level meter RAF path only.
let noiseFloor = 0.02
let noiseCalSamples = 0
const NOISE_CAL_FRAMES = 20  // kept for level meter; worklet uses its own counter
let prevLevel = 0

let ws = null, capture = null, playback = null
let speakingTimeout = 0, timerId = 0, audioCtx = null, levelRaf = 0
let resetVad = null

// ── Video (camera / screen share) ─────────────────────────────────────────────
const videoMode = ref('off')   // 'off' | 'camera' | 'screen'
let videoStream = null
let videoFrameTimer = null

// Text chat input (reference pattern: collapsible chat box)
const showTextInput = ref(false)
const textMessage = ref('')
function sendTextMessage() {
  const t = textMessage.value.trim()
  if (!t || !ws || ws.readyState !== WebSocket.OPEN) return
  // Add to transcript immediately (same as reference)
  vlStore.transcript.push({ role: 'user', text: t, timestamp: Date.now(), isFinal: true })
  ws.send(JSON.stringify({ type: 'text', payload: { content: t } }))
  textMessage.value = ''
}

async function toggleCamera() {
  if (videoMode.value === 'camera') { stopVideo(); return }
  if (videoMode.value === 'screen') stopVideo()
  try {
    videoStream = await navigator.mediaDevices.getUserMedia({
      video: { width: { ideal: 320 }, height: { ideal: 240 }, facingMode: 'user' },
      audio: false,
    })
    videoMode.value = 'camera'
    startVideoFrameLoop()
    // Wire stream into preview element after next tick (ref not yet mounted for new mode)
    await nextTick()
    if (videoPreviewEl.value) videoPreviewEl.value.srcObject = videoStream
  } catch (err) {
    console.warn('Camera error:', err)
  }
}

async function toggleScreen() {
  if (videoMode.value === 'screen') { stopVideo(); return }
  if (videoMode.value === 'camera') stopVideo()
  try {
    videoStream = await navigator.mediaDevices.getDisplayMedia({
      video: { width: { ideal: 1280 }, height: { ideal: 720 }, frameRate: { ideal: 5, max: 5 } },
      audio: false,
    })
    // User cancelled picker — getTracks() empty
    if (!videoStream.getVideoTracks().length) { videoStream.getTracks().forEach(t => t.stop()); return }
    videoMode.value = 'screen'
    startVideoFrameLoop()
    await nextTick()
    if (videoPreviewEl.value) videoPreviewEl.value.srcObject = videoStream
    // Stop when user ends share via browser/OS chrome
    videoStream.getVideoTracks()[0].addEventListener('ended', () => stopVideo())
  } catch (err) {
    if (err.name !== 'NotAllowedError') console.warn('Screen share error:', err)
  }
}

function startVideoFrameLoop() {
  if (videoFrameTimer) clearInterval(videoFrameTimer)
  videoFrameTimer = setInterval(() => {
    const canvas = videoCanvasEl.value
    const video = videoPreviewEl.value
    if (!canvas || !video || video.readyState < 2 || !ws) return
    const ctx2d = canvas.getContext('2d')
    ctx2d.drawImage(video, 0, 0, canvas.width, canvas.height)
    const dataUrl = canvas.toDataURL('image/jpeg', videoMode.value === 'screen' ? 0.6 : 0.5)
    const base64 = dataUrl.split(',')[1]
    if (base64 && ws?.readyState === WebSocket.OPEN)
      ws.send(JSON.stringify({ type: 'video_frame', payload: { data: base64, mime_type: 'image/jpeg' } }))
  }, 1000)  // 1 FPS — matches Gemini Live API recommendation
}

function stopVideo() {
  if (videoFrameTimer) { clearInterval(videoFrameTimer); videoFrameTimer = null }
  if (videoStream) { videoStream.getTracks().forEach(t => t.stop()); videoStream = null }
  if (videoPreviewEl.value) videoPreviewEl.value.srcObject = null
  videoMode.value = 'off'
}

// ── Computed ──────────────────────────────────────────────────────────────────
const hasActiveSession = computed(() =>
  connectionState.value === 'active' || connectionState.value === 'connecting'
)

const agentState = computed(() => {
  if (connectionState.value === 'connecting') return 'thinking'
  if (speaking.value) return 'speaking'
  // Only enter listening state if audio is genuinely above noise floor
  if (connectionState.value === 'active' && audioLevel.value > 0.12) return 'listening'
  return 'idle'
})

const statusText = computed(() => {
  if (connectionState.value === 'connecting' || isStarting.value) return 'Connecting…'
  if (connectionState.value === 'active') return 'Live'
  return 'Idle'
})

const timer = computed(() => {
  const m = String(Math.floor(elapsed.value / 60)).padStart(2, '0')
  const s = String(elapsed.value % 60).padStart(2, '0')
  return `${m}:${s}`
})

// Per-voice deterministic hue
function voiceHue(name) {
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0
  return h % 360
}
const orbVars = computed(() => {
  const hue = voiceHue(voiceName.value || 'puck')
  return {
    '--c1': `oklch(0.58 0.19 ${hue})`,
    '--c2': `oklch(0.68 0.16 ${(hue + 40) % 360})`,
    '--c3': `oklch(0.78 0.12 ${(hue + 80) % 360})`,
  }
})

// ── Level meter with noise-floor calibration ──────────────────────────────────
function startLevelMeter(stream) {
  if (!stream || !(stream instanceof MediaStream) || !stream.getAudioTracks().length) return
  audioCtx = new (window.AudioContext || window.webkitAudioContext)()
  if (audioCtx.state === 'suspended') audioCtx.resume()

  const src = audioCtx.createMediaStreamSource(stream)
  const analyser = audioCtx.createAnalyser()
  analyser.fftSize = 1024  // higher FFT = better frequency resolution
  src.connect(analyser)

  const buf = new Uint8Array(analyser.frequencyBinCount)
  noiseFloor = 0.04; noiseCalSamples = 0; prevLevel = 0

  const tick = () => {
    if (!audioCtx) return
    analyser.getByteTimeDomainData(buf)

    // Compute RMS
    let sum = 0
    for (const v of buf) sum += (v - 128) ** 2
    const rms = Math.sqrt(sum / buf.length) / 128  // 0–1

    // Noise floor calibration: average first N frames
    if (noiseCalSamples < NOISE_CAL_FRAMES) {
      noiseFloor = ((noiseFloor * noiseCalSamples) + rms) / (noiseCalSamples + 1)
      noiseCalSamples++
    }

    // Gate: only pass signal above noise floor × 2.5 (hysteresis)
    const threshold = noiseFloor * 2.5
    const gated = rms > threshold ? (rms - threshold) / (1 - threshold) : 0

    // Exponential smoothing (attack fast, decay slow)
    const alpha = gated > prevLevel ? 0.6 : 0.15
    prevLevel = prevLevel * (1 - alpha) + gated * alpha

    const level = isMuted.value ? 0 : Math.min(1, prevLevel * 3)
    audioLevel.value = level

    // Drive bar heights directly via DOM — no Vue reactive update,
    // no flexbox relayout on the orb row, no orb jitter.
    // Each bar gets a smooth sine-wave height based on its index + time.
    const t = performance.now() * 0.001
    const leftEl  = barsLeft.value
    const rightEl = barsRight.value
    if (leftEl && rightEl && level > 0) {
      const lBars = leftEl.children
      const rBars = rightEl.children
      for (let i = 0; i < 12; i++) {
        const h = Math.max(3, 4 + Math.abs(Math.sin((i + t) * 0.7)) * level * 44)
        const hr = Math.max(3, 4 + Math.abs(Math.sin((11 - i + t) * 0.7)) * level * 44)
        lBars[i].style.height = h + 'px'
        rBars[i].style.height = hr + 'px'
      }
    } else if (leftEl && rightEl) {
      // Reset to flat baseline when silent / muted
      const lBars = leftEl.children
      const rBars = rightEl.children
      for (let i = 0; i < 12; i++) {
        lBars[i].style.height = '3px'
        rBars[i].style.height = '3px'
      }
    }
    levelRaf = requestAnimationFrame(tick)
  }
  levelRaf = requestAnimationFrame(tick)
}

function stopLevelMeter() {
  cancelAnimationFrame(levelRaf)
  audioLevel.value = 0; prevLevel = 0
  // Reset noise calibration so next session re-calibrates from scratch.
  // Stale noiseFloor from a previous session causes wrong VAD threshold.
  noiseFloor = 0.04; noiseCalSamples = 0
  audioCtx?.close(); audioCtx = null
}

// ── Mute ──────────────────────────────────────────────────────────────────────
function toggleMute() {
  if (!capture) return
  isMuted.value = !isMuted.value
  if (isMuted.value) {
    capture.ctx?.suspend?.()
    // Reset VAD on mute — stale state causes hallucination on unmute:
    // - vadRmsSmooth frozen low  → fires ws.end() on first chunk after unmute
    // - vadSilenceChunks at 5/6 → one more chunk triggers spurious turn-end
    // - vadSentEnd stale         → wrong state for next user turn
    resetVad?.()
  } else {
    capture.ctx?.resume?.()
    // Also reset on unmute so stale smoothed RMS doesn't fire a
    // false turn-end or false speech detection immediately
    resetVad?.()
  }
}

// ── Transcript — decoupled from audio hot path ────────────────────────────────
// appendTranscript is called from ws.onmessage on every interim chunk.
// To keep Vue reactivity + DOM scroll off the audio scheduling path, interim
// payloads are buffered and flushed every 80ms by a timer. Audio chunks in
// the same window are scheduled into Web Audio BEFORE any Vue re-render fires.
// isFinal payloads (turn boundaries) flush immediately — they're infrequent.
const TRANSCRIPT_MAX = 200
let transcriptQueue = []  // pending interim payloads, drained by flushTimer
let transcriptTimer = 0   // setInterval handle, active only during a session

function flushTranscript() {
  if (!transcriptQueue.length) return
  const batch = transcriptQueue.splice(0)
  for (const p of batch) applyTranscript(p)
  nextTick(() => {
    if (transcriptEl.value)
      transcriptEl.value.scrollTop = transcriptEl.value.scrollHeight
  })
}

function appendTranscript(payload) {
  if (payload.text === '<noise>' || payload.text.includes('<noise>')) return
  if (payload.isFinal) {
    if (transcriptQueue.length) flushTranscript()
    applyTranscript(payload)
    nextTick(() => {
      if (transcriptEl.value)
        transcriptEl.value.scrollTop = transcriptEl.value.scrollHeight
    })
    return
  }
  // User transcript: show immediately — it’s their own speech, delay feels wrong.
  // Agent interim: batch in queue (keeps audio scheduling hot path clean).
  if (payload.role === 'user') {
    applyTranscript(payload)
    return
  }
  transcriptQueue.push(payload)
}

function applyTranscript(payload) {
  const t = vlStore.transcript
  const last = t[t.length - 1]
  // Empty isFinal=true = TurnComplete seal
  if (payload.isFinal && payload.text === '' && last && last.role === payload.role) {
    last.isFinal = true
    return
  }
  // Same role → update in-place (typewriter)
  if (last && last.role === payload.role) {
    last.text = payload.text
    last.isFinal = payload.isFinal
    return
  }
  // Role-switch merge
  if (payload.isFinal && last && last.text === payload.text && last.role !== payload.role) {
    last.isFinal = true
    return
  }
  // New turn
  if (payload.text === '') return
  if (t.length >= TRANSCRIPT_MAX) t.splice(0, 1)
  t.push({ role: payload.role, text: payload.text, isFinal: payload.isFinal })
}

// Scroll only on new turn (length change) — interim scroll handled in flushTranscript
watch(() => messages.value.length, () => {
  nextTick(() => {
    if (transcriptEl.value)
      transcriptEl.value.scrollTop = transcriptEl.value.scrollHeight
  })
})

// ── Session lifecycle ─────────────────────────────────────────────────────────
async function startSession() {
  if (hasActiveSession.value || isStarting.value) return
  isStarting.value = true
  sessionError.value = ''
  connectionState.value = 'connecting'
  isMuted.value = false

  // Pre-warm AudioContext inside the user-gesture frame (this click handler).
  // In web/browser mode AudioContext starts suspended until a user gesture.
  // onSessionState runs from a WS message handler — NOT a gesture frame —
  // so resume() there is silently ignored by Chrome/Safari.
  if (!playback) playback = createPlayback()
  if (playback.ctx.state === 'suspended') playback.ctx.resume()

  // ── Reference session lifecycle (POST /sessions → WS /sessions/:id/stream) ──
  // Step 1: Create session via REST (gets a session_id)
  const session = await vlStore.createSession()
  if (!session) {
    isStarting.value = false
    sessionError.value = vlStore.lastError || 'Failed to create session'
    connectionState.value = 'error'
    return
  }

  // Step 2: Connect WebSocket to /api/agent-live/sessions/:id/stream
  // URL logic:
  //   - Desktop (Wails): baseUrl = 'http://127.0.0.1:PORT' → ws://127.0.0.1:PORT
  //   - Web/Vite proxy:  baseUrl = '' → ws://location.host (Vite proxies /api/* to core)
  //   - HTTPS:           baseUrl = 'https://...' → wss://...
  const params = new URLSearchParams({
    voice_name:   voiceName.value,
    system_text:  instructions.value,
    temperature:  String(temperature.value),
  })
  let wsUrl
  if (!core.baseUrl || core.baseUrl === '') {
    // Web/Vite proxy — same origin, let Vite proxy /api to core
    const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    wsUrl = `${wsProto}//${location.host}/api/agent-live/sessions/${session.id}/stream?${params}`
  } else {
    // Desktop or explicit baseUrl
    const wsProto = core.baseUrl.startsWith('https') ? 'wss:' : 'ws:'
    const wsHost = core.baseUrl.replace(/^https?:\/\//, '')
    wsUrl = `${wsProto}//${wsHost}/api/agent-live/sessions/${session.id}/stream?${params}`
  }
  ws = new WebSocket(wsUrl)
  ws.binaryType = 'arraybuffer'

  ws.onerror = () => {
    sessionError.value = 'WebSocket connection error'
    connectionState.value = 'error'
    isStarting.value = false
    teardown()
  }
  ws.onclose = () => {
    if (connectionState.value !== 'error') connectionState.value = 'idle'
    isStarting.value = false
    teardown()
  }
  ws.onmessage = (e) => {
    if (e.data instanceof ArrayBuffer) {
      playback?.playChunk(e.data)
      if (!speaking.value) speaking.value = true
      clearTimeout(speakingTimeout)
      speakingTimeout = setTimeout(() => { speaking.value = false }, 500)
      return
    }
    let msg
    try { msg = JSON.parse(e.data) } catch { return }
    // Route all messages through the store (handles transcript, tool_call, tool_result, session_state, interrupt, error)
    vlStore.handleWSMessage(msg)
    // Supplement store handling with local audio/VAD side-effects
    if (msg.type === 'interrupt') {
      // Kill all scheduled audio immediately
      playback?.flush()
      clearTimeout(speakingTimeout)
      speaking.value = false
      // Reset VAD so barge-in audio flows without false end-of-turn
      resetVad?.()
      // Drop buffered transcript interims from interrupted response
      transcriptQueue = []
    }
    if (msg.type === 'error') {
      sessionError.value = msg.payload?.message || 'Voice session error'
      connectionState.value = 'error'
      isStarting.value = false
      teardown()
    }
    if (msg.type === 'session_state') {
      const payload = msg.payload
      if (payload.state !== 'active') return
      // GoAway reconnect — reset playback only
      if (connectionState.value === 'active') { playback?.flush(); return }
      // First activation
      connectionState.value = 'active'
      isStarting.value = false
      elapsed.value = 0
      timerId = setInterval(() => elapsed.value++, 1000)
      transcriptTimer = setInterval(flushTranscript, 80)
      if (!playback) playback = createPlayback()
      if (playback.ctx.state === 'suspended') playback.ctx.resume()
      ;(async () => { try {
        // CONTINUOUS STREAMING — no client-side VAD or turn-end signal.
        // Gemini Live has its own acoustic VAD. We stream PCM continuously;
        // Gemini decides when the user has finished and responds immediately.
        // Removing our artificial silence gate eliminates 120-400ms of added
        // latency and the hallucination-causing premature AudioStreamEnd signals.
        resetVad = () => {}  // no-op; kept so toggleMute doesn't break

        capture = await createCapture((msg) => {
          if (isMuted.value) return
          // msg = {buffer: ArrayBuffer, rms, noiseFloor, calibrated}
          if (ws?.readyState === WebSocket.OPEN) ws.send(msg.buffer)
        })
        startLevelMeter(capture.stream)
      } catch (err) {
        sessionError.value = err?.message || String(err)
        stopSession()
      } })()  // end async IIFE for capture setup
    }  // end session_state handler
  }  // end ws.onmessage
}

function teardown() {
  clearInterval(timerId)
  clearInterval(transcriptTimer); transcriptTimer = 0
  flushTranscript()
  transcriptQueue = []
  stopLevelMeter()
  stopVideo()                  // stop camera / screen share
  capture?.stop(); capture = null
  playback?.stop(); playback = null
  resetVad = null
  speaking.value = false; isMuted.value = false
}

function stopSession() {
  if (ws?.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: 'end' }))
    ws.close()
  }
  ws = null
  teardown()
  vlStore.endSession()
  if (connectionState.value !== 'error') connectionState.value = 'idle'
}

onMounted(loadConfig)
onBeforeUnmount(stopSession)
</script>

<template>
  <div class="vl">

    <!-- ─── Pattern blur background ─────────────────────────── -->
    <div class="vl__pat" aria-hidden="true">
      <svg class="vl__pat-svg" viewBox="0 0 525 647" preserveAspectRatio="xMidYMid slice"
        xmlns="http://www.w3.org/2000/svg">
        <g opacity="0.09">
          <path d="M50.1406 64.7157V91.9056C50.1406 126.944 78.2762 155.108 113.217 155.108H140.38V127.918C140.38 92.9075 112.272 64.7157 77.3041 64.7157H50.1406Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M213.982 64.7157C179.042 64.7157 150.906 92.9075 150.906 127.918V155.108H178.07C213.01 155.108 241.146 126.916 241.146 91.9056V64.7157H213.982Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M113.217 165.683C78.2762 165.683 50.1406 193.875 50.1406 228.913V256.103H77.3041C112.272 256.103 140.38 227.911 140.38 192.901V165.711H113.217Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M150.906 165.683V192.873C150.906 227.911 179.042 256.075 213.982 256.075H241.146V228.885C241.146 193.847 213.01 165.683 178.07 165.683H150.906Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M300.082 4.083V20.255C300.082 41.095 316.848 57.847 337.668 57.847H353.855V41.675C353.855 20.851 337.106 4.083 316.268 4.083H300.082Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M397.715 4.083C376.895 4.083 360.129 20.851 360.129 41.675V57.847H376.315C397.136 57.847 413.902 41.079 413.902 20.255V4.083H397.715Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M337.668 64.137C316.848 64.137 300.082 80.905 300.082 101.745V117.917H316.268C337.106 117.917 353.855 101.149 353.855 80.326V64.137H337.668Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M360.129 64.137V80.309C360.129 101.149 376.895 117.901 397.715 117.901H413.902V101.729C413.902 80.889 397.136 64.137 376.315 64.137H360.129Z" stroke="currentColor" stroke-miterlimit="10"/>
        </g>
      </svg>
      <div class="vl__pat-frost" />
      <div class="vl__pat-grain" />
    </div>

    <!-- ═══ LEFT SIDEBAR ═══ -->
    <aside class="vl__sidebar">

      <!-- Sidebar header card — board-header pattern with bg-mark watermark -->
      <div class="vl__sb-card">
        <!-- bg watermark -->
        <svg class="vl__sb-card-mark" viewBox="0 0 131 131" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M53.7421 59.6211L29.8369 60.0105L2 32.1736L25.9052 31.7842L53.7421 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M59.6211 53.7421L60.0105 29.8183L32.1736 2L31.7842 25.9052L59.6211 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M77.2579 71.3789L101.163 70.9895L129 98.8264L105.095 99.2158L77.2579 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M71.3789 77.2579L70.9895 101.182L98.8264 129L99.2158 105.095L71.3789 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M59.6211 77.2579L60.0105 101.163L32.1736 129L31.7842 105.095L59.6211 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M53.7421 71.3789L29.8369 70.9895L2 98.8264L25.9052 99.2158L53.7421 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M71.3789 53.7421L70.9895 29.8369L98.8264 2L99.2158 25.9052L71.3789 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M77.2579 59.6211L101.182 60.0105L129 32.1736L105.095 31.7842L77.2579 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M69.9324 69.9324H61.0676V61.0676H69.9324V69.9324Z" stroke="currentColor" stroke-miterlimit="10"/>
        </svg>
      <svg class="vl__sb-card-mark-sm" viewBox="0 0 131 131" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M53.7421 59.6211L29.8369 60.0105L2 32.1736L25.9052 31.7842L53.7421 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 53.7421L60.0105 29.8183L32.1736 2L31.7842 25.9052L59.6211 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 71.3789L101.163 70.9895L129 98.8264L105.095 99.2158L77.2579 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 77.2579L70.9895 101.182L98.8264 129L99.2158 105.095L71.3789 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 77.2579L60.0105 101.163L32.1736 129L31.7842 105.095L59.6211 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M53.7421 71.3789L29.8369 70.9895L2 98.8264L25.9052 99.2158L53.7421 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 53.7421L70.9895 29.8369L98.8264 2L99.2158 25.9052L71.3789 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 59.6211L101.182 60.0105L129 32.1736L105.095 31.7842L77.2579 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M69.9324 69.9324H61.0676V61.0676H69.9324V69.9324Z" stroke="currentColor" stroke-miterlimit="10"/>
      </svg>
      <svg class="vl__sb-card-mark-md" viewBox="0 0 131 131" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M53.7421 59.6211L29.8369 60.0105L2 32.1736L25.9052 31.7842L53.7421 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 53.7421L60.0105 29.8183L32.1736 2L31.7842 25.9052L59.6211 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 71.3789L101.163 70.9895L129 98.8264L105.095 99.2158L77.2579 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 77.2579L70.9895 101.182L98.8264 129L99.2158 105.095L71.3789 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 77.2579L60.0105 101.163L32.1736 129L31.7842 105.095L59.6211 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M53.7421 71.3789L29.8369 70.9895L2 98.8264L25.9052 99.2158L53.7421 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 53.7421L70.9895 29.8369L98.8264 2L99.2158 25.9052L71.3789 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 59.6211L101.182 60.0105L129 32.1736L105.095 31.7842L77.2579 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M69.9324 69.9324H61.0676V61.0676H69.9324V69.9324Z" stroke="currentColor" stroke-miterlimit="10"/>
      </svg>
        <!-- Title + status -->
        <div class="vl__sb-card-inner">
          <div class="vl__sb-title-row">
            <span class="vl__sb-title">Voice Assistant</span>
          </div>
          <span v-if="defaultModel" class="vl__model-pill">{{ defaultModel }}</span>
          <!-- Hotkey pill -->
          <div class="vl__hotkey-pill" :class="{ 'vl__hotkey-pill--active': hotkeyActive }">
            <svg viewBox="0 0 24 24" width="9" height="9" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M6 8h.01M10 8h.01M14 8h.01M18 8h.01M8 12h.01M12 12h.01M16 12h.01M7 16h10"/></svg>
            <span v-if="hotkeyActive">⌘⇧Space — show/hide from anywhere</span>
            <span v-else>Enable Accessibility for ⌘⇧Space shortcut</span>
          </div>
        </div>
      </div>

      <!-- Config fields -->
      <div class="vl__config">

        <!-- Voice custom picker -->
        <div class="vl__field">
          <label class="vl__field-label">Voice</label>
          <div class="vl__voice-persona">
            <button class="vl__voice-persona-btn" :disabled="hasActiveSession"
              @click.stop="showVoiceMenu = !showVoiceMenu">
              <span class="vl__voice-persona-dot" :style="`background:oklch(0.6 0.18 ${voiceHue(voiceName)})`" />
              <span class="vl__voice-persona-name">{{ voiceName || 'Select voice…' }}</span>
              <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="m6 9 6 6 6-6"/>
              </svg>
            </button>
            <div v-if="showVoiceMenu" class="vl__voice-persona-menu">
              <div v-if="configLoading" class="vl__voice-persona-loading">Loading voices…</div>
              <button v-for="v in voices" :key="v"
                :class="['vl__voice-persona-opt', { 'vl__voice-persona-opt--on': voiceName === v }]"
                @click="voiceName = v; showVoiceMenu = false">
                <span class="vl__voice-persona-opt-dot" :style="`background:oklch(0.6 0.18 ${voiceHue(v)})`" />
                <span class="vl__voice-persona-opt-name">{{ v }}</span>
                <svg v-if="voiceName === v" viewBox="0 0 12 12" width="11" height="11" fill="none">
                  <path d="M2 6l3 3 5-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </button>
            </div>
          </div>
          <p v-if="configError" class="vl__field-err">{{ configError }}</p>
        </div>

        <!-- Temperature -->
        <div class="vl__field">
          <label class="vl__field-label">
            Temperature
            <span class="vl__field-badge">{{ temperature.toFixed(1) }}</span>
          </label>
          <div class="vl__slider-row">
            <span class="vl__slider-hint">Precise</span>
            <input v-model.number="temperature" type="range" min="0" max="2" step="0.1"
              :disabled="hasActiveSession" class="vl__range" />
            <span class="vl__slider-hint">Creative</span>
          </div>
        </div>

      </div><!-- /config -->

      <!-- ── INSTRUCTIONS — always a real textarea, always editable when idle ── -->
      <div class="vl__instr">
        <div class="vl__instr-head">
          <span class="vl__instr-label">INSTRUCTIONS</span>
          <!-- Preset selector -->
          <div class="vl__voice-instruction vl__voice-instruction--preset" ref="presetTriggerEl">
            <button class="vl__voice-persona-btn" :disabled="hasActiveSession"
              @click.stop="showPresetMenu = !showPresetMenu">
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" v-html="presetIconSvg(vlStore.selectedPreset?.icon || 'bot')" />
              <span class="vl__voice-persona-name">{{ vlStore.selectedPreset?.name || 'Preset' }}</span>
              <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="m6 9 6 6 6-6"/>
              </svg>
            </button>
            <!-- Teleport dropdown to body so sidebar overflow:auto can't clip it -->
            <Teleport to="body">
              <div v-if="showPresetMenu" class="vl__voice-persona-preset-menu"
                :style="presetMenuStyle">
                <div class="vl__voice-persona-preset-section">Agent Presets</div>
                <button v-for="p in vlStore.presets" :key="p.id"
                  :class="['vl__voice-persona-preset-opt', { 'vl__voice-persona-preset-opt--on': selectedPresetId === p.id }]"
                  @click="selectPreset(p.id)">
                  <svg class="vl__voice-persona-preset-opt-icon" viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" v-html="presetIconSvg(p.icon || 'bot')" />
                  <div>
                    <div class="vl__voice-persona-preset-opt-name">{{ p.name }}</div>
                    <div class="vl__voice-persona-preset-opt-desc">{{ p.desc }}</div>
                  </div>
                </button>
              </div>
            </Teleport>
          </div>
        </div>
        <!-- Always a textarea — editable when idle, read-only (but visible) when active -->
        <textarea
          v-model="instructions"
          :disabled="hasActiveSession"
          placeholder="System instructions for the voice assistant…"
          class="vl__instr-ta"
        />
        <p class="vl__instr-hint">
          <template v-if="hasActiveSession"><svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" style="vertical-align:-1px;margin-right:3px"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>Locked while session active.</template><template v-else>Applies on next session start.</template>
        </p>
      </div>

    </aside><!-- /sidebar -->

    <!-- ═══════════════════════════════════════════════════════
         RIGHT MAIN — topbar · orb-stage · transcript-feed · controls
         ═══════════════════════════════════════════════════════ -->
    <div class="vl__main" :style="orbVars">

      <!-- Error banner only — no topbar, title lives in sidebar -->
      <div v-if="sessionError" class="vl__error-bar" role="alert">
        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        <span>{{ sessionError }}</span>
        <button @click="sessionError = ''" class="vl__error-close">
          <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.8">
            <path d="M4 4l8 8M12 4l-8 8"/>
          </svg>
        </button>
      </div>

      <!-- ── ORB STAGE — orb + bars + inline video preview ── -->
      <div class="vl__orb-stage">

        <!-- Camera / screen preview — floats top-left inside orb stage -->
        <div v-if="videoMode !== 'off'" class="vl__video-preview">
          <video ref="videoPreviewEl" autoplay muted playsinline class="vl__video-preview__video"
            :style="videoMode === 'camera' ? 'transform:scaleX(-1)' : ''" />
          <span class="vl__video-preview__badge">
            <svg viewBox="0 0 8 8" width="5" height="5" fill="currentColor"><circle cx="4" cy="4" r="4"/></svg>
            {{ videoMode === 'camera' ? 'Camera' : 'Screen' }}
          </span>
          <canvas ref="videoCanvasEl" width="320" height="240" style="display:none" />
        </div>

        <!-- Mic bars LEFT -->
        <div class="vl__bars" ref="barsLeft"
          :class="{ 'vl__bars--active': connectionState === 'active' && !isMuted }">
          <div v-for="i in 12" :key="i" class="vl__bar" :class="`vl__bar--${agentState}`" />
        </div>

        <!-- Orb -->
        <div :class="['vl__orb', `vl__orb--${agentState}`]"
          role="img"
          :aria-label="`${voiceName} — ${statusText}`">
          <div class="vl__orb-sphere">
            <div class="vl__blob vl__blob--1" />
            <div class="vl__blob vl__blob--2" />
            <div class="vl__blob vl__blob--3" />
          </div>
          <span v-if="agentState === 'speaking'" class="vl__orb-pulse" />
          <div v-if="isMuted" class="vl__orb-mute">
            <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="white" stroke-width="1.7"
              stroke-linecap="round">
              <line x1="1" y1="1" x2="23" y2="23"/>
              <path d="M9 9v3a3 3 0 0 0 5.12 2.12M15 9.34V5a3 3 0 0 0-5.94-.6"/>
              <path d="M17 16.95A7 7 0 0 1 5 12v-2m14 0v2a7 7 0 0 1-.11 1.23"/>
              <line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/>
            </svg>
          </div>
        </div>

        <!-- Mic bars RIGHT -->
        <div class="vl__bars" ref="barsRight"
          :class="{ 'vl__bars--active': connectionState === 'active' && !isMuted }">
          <div v-for="i in 12" :key="i" class="vl__bar" :class="`vl__bar--${agentState}`" />
        </div>

      </div><!-- /orb-stage -->

      <!-- Status row — below orb, never inside it -->
      <div class="vl__status-row">
        <span class="vl__orb-timer" :class="{ 'vl__orb-timer--active': hasActiveSession }">
          {{ hasActiveSession ? timer : '' }}
        </span>
        <span v-if="connectionState !== 'error'" :class="['vl__badge', `vl__badge--${connectionState}`]">
          <i class="vl__badge-dot" />
          {{ statusText }}
        </span>
      </div>

      <!-- ── TRANSCRIPT — fixed height, scrollable history ── -->
      <div class="vl__transcript" ref="transcriptEl" aria-live="polite" aria-atomic="false">
        <div v-if="!messages.length" class="vl__transcript-empty">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.2">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
          </svg>
          <span>Conversation will appear here</span>
        </div>
        <div v-for="(m, i) in messages" :key="i"
          :class="['vl__turn', `vl__turn--${m.role}`, { 'vl__turn--interim': !m.isFinal }]">
          <span :class="['vl__turn-av', `vl__turn-av--${m.role}`]">
            <svg v-if="m.role === 'user'" viewBox="0 0 24 24" width="11" height="11" fill="none"
              stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="8" r="4"/><path d="M4 20c0-4 3.6-7 8-7s8 3 8 7"/>
            </svg>
            <svg v-else viewBox="0 0 24 24" width="11" height="11" fill="none"
              stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="8" width="18" height="11" rx="2"/>
              <path d="M12 8V4M9 4h6"/>
              <circle cx="9" cy="13.5" r="1" fill="currentColor" stroke="none"/>
              <circle cx="15" cy="13.5" r="1" fill="currentColor" stroke="none"/>
            </svg>
          </span>
          <div class="vl__turn-body">
            <!-- Label only on first turn, or when speaker changes -->
            <span v-if="i === 0 || messages[i-1].role !== m.role" class="vl__turn-who">
              {{ m.role === 'user' ? 'You' : (vlStore.selectedPreset?.name || 'Agent') }}
            </span>
            <p class="vl__turn-text">{{ m.text }}<span v-if="!m.isFinal" class="vl__turn-cursor" /></p>
          </div>
        </div>
      </div>

      <!-- ── CONTROLS — clean, no border ── -->
      <div class="vl__controls">
        <!-- IDLE/ERROR: Start button -->
        <button v-if="!hasActiveSession" class="vl__btn-start"
          :disabled="isStarting || !voiceName" @click="startSession">
          <svg v-if="isStarting" class="vl__spin" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M21 12a9 9 0 1 1-6.22-8.56"/></svg>
          <svg v-else viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/></svg>
          {{ isStarting ? 'Connecting…' : 'Start a conversation' }}
        </button>

        <!-- ACTIVE: tool buttons + Mute + End -->
        <template v-else>

          <!-- Camera -->
          <button :class="['vl__btn-tool', { 'vl__btn-tool--on': videoMode === 'camera' }]"
            title="Toggle camera" @click="toggleCamera">
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <template v-if="videoMode === 'camera'">
              <path d="M16 16v1a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h2"/>
              <path d="m22 8-6 4 6 4V8Z"/><line x1="2" y1="2" x2="22" y2="22"/>
            </template>
            <template v-else>
              <path d="m22 8-6 4 6 4V8Z"/><rect x="2" y="6" width="14" height="12" rx="2"/>
            </template>
          </svg>
          </button>

          <!-- Screen share -->
          <button :class="['vl__btn-tool', { 'vl__btn-tool--on': videoMode === 'screen' }]"
            title="Share screen" @click="toggleScreen">
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
          </button>

          <!-- Mute -->
          <button :class="['vl__btn-mic', `vl__btn-mic--${agentState}`, { 'vl__btn-mic--muted': isMuted }]"
            :title="isMuted ? 'Unmute' : 'Mute'" @click="toggleMute">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <template v-if="isMuted">
              <line x1="2" y1="2" x2="22" y2="22"/>
              <path d="M9 9v3a3 3 0 0 0 5.12 2.12M15 9.34V5a3 3 0 0 0-5.94-.6"/>
              <path d="M17 16.95A7 7 0 0 1 5 12v-2m14 0v2a7 7 0 0 1-.11 1.23"/>
              <line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/>
            </template>
            <template v-else>
              <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/>
              <path d="M19 10v2a7 7 0 0 1-14 0v-2"/>
              <line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/>
            </template>
          </svg>
            <span v-if="!isMuted && agentState === 'listening'" class="vl__btn-mic-ring" />
          </button>

          <!-- Text chat toggle -->
          <button :class="['vl__btn-tool', { 'vl__btn-tool--on': showTextInput }]"
            title="Type a message" @click="showTextInput = !showTextInput">
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M6 8h.01M10 8h.01M14 8h.01M18 8h.01M8 12h.01M12 12h.01M16 12h.01M7 16h10"/></svg>
          </button>

          <!-- End -->
          <button class="vl__btn-end" title="End conversation" @click="stopSession">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.68 13.31a16 16 0 0 0 3.41 2.6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7 2 2 0 0 1 1.72 2v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07A19.42 19.42 0 0 1 4.86 16m-2.67-3.34A19.79 19.79 0 0 1 2 8.63 2 2 0 0 1 4.11 6.5h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 14.4"/><line x1="23" y1="1" x2="1" y2="23"/></svg>
          </button>
        </template>
      </div>

      <!-- Collapsible text chat input (reference pattern) -->
      <div v-if="showTextInput && hasActiveSession" class="vl__text-input">
        <input v-model="textMessage" placeholder="Type a message (or just speak)…"
          @keydown.enter="sendTextMessage" />
        <button :disabled="!textMessage.trim()" @click="sendTextMessage">
          <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>
        </button>
      </div>

    </div><!-- /main -->
  </div>
</template>

<style scoped>
/* ═══════════════════════════════════════════════════════════
   Voice Live — masterclass layout
   Two-column: sidebar (config+instructions) | main (orb+feed+controls)
   Transcript feed is FIXED height — zero layout shift
   ═══════════════════════════════════════════════════════════ */

.vl {
  display: flex;
  height: 100%;
  min-height: 0;
  position: relative;
  overflow: hidden;
  background: var(--bg);
  color: var(--ink);
  font-family: -apple-system, BlinkMacSystemFont, 'Inter', sans-serif;
}

/* ── Pattern blur ──────────────────────────────────────── */
.vl__pat {
  position: absolute; inset: 0; pointer-events: none; z-index: 0; overflow: hidden;
}
.vl__pat-svg {
  position: absolute; top: -5%; right: -8%; width: 65%; height: 115%;
  color: var(--accent-ai); opacity: 0.5; filter: blur(100px);
}
.vl__pat-frost {
  position: absolute; inset: 0; background: var(--bg); opacity: 0.76;
  backdrop-filter: blur(40px); -webkit-backdrop-filter: blur(40px);
}
.vl__pat-grain {
  position: absolute; inset: 0; opacity: 0.016;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 400 400' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
}

/* ── LEFT SIDEBAR ──────────────────────────────────────── */
.vl__sidebar {
  position: relative;
  z-index: 2;              /* above main so preset menu renders over it */
  width: 340px;
  flex-shrink: 0;
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  display: flex;
  flex-direction: column;
  overflow-y: auto; scrollbar-width: none;
  overflow-x: visible;     /* allow preset menu to overflow */
}
.vl__sidebar::-webkit-scrollbar { display: none; }

/* ── Sidebar header card — board-header pattern ── */
.vl__sb-card {
  position: relative;
  flex-shrink: 0;
  margin: 12px 12px 0;
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, var(--accent-ai) 7%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--accent-ai) 18%, var(--border));
  overflow: hidden;
  min-height: 80px;
}
.vl__sb-card-mark {
  position: absolute;
  top: 5px; right: -5px;
  width: 110px; height: 111px;
  color: var(--accent-ai); opacity: 0.10;
  pointer-events: none;
}
.vl__sb-card-mark-sm {
  position: absolute; z-index: 0;
  top: 0px;
  right: 102px;
  width: 36px; height: 36px;
  color: var(--accent-ai); opacity: 0.3;
  pointer-events: none;
  transform: none;
}
.vl__sb-card-mark-md {
  position: absolute; z-index: 0;
  top: 30px;
  right: 133px;
  width: 60px; height: 60px;
  color: var(--accent-ai); opacity: 0.18;
  pointer-events: none;
  transform: none;
}
.vl__sb-card-inner {
  position: relative; z-index: 1;
  padding: 14px 14px 12px;
  display: flex; flex-direction: column; gap: 8px;
}
.vl__sb-title-row {
  display: flex; align-items: center; gap: 7px;
}
.vl__sb-mic { color: var(--accent-ai); flex-shrink: 0; }
.vl__sb-title { font-size: 15px; font-weight: 700; color: var(--ink); }
.vl__sb-meta-row {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
}

/* Config section below card */
.vl__config {
  padding: 0 0 4px;
  display: flex; flex-direction: column;
}

/* Fields */
.vl__field { padding: 12px 14px 0; display: flex; flex-direction: column; gap: 6px; }
.vl__field-label {
  display: flex; align-items: center; justify-content: space-between;
  font-size: 9.5px; font-weight: 700; text-transform: uppercase;
  letter-spacing: 0.5px; color: var(--ink-faint);
}
.vl__field-badge {
  font-size: 10.5px; font-weight: 700; color: var(--accent-ai);
  background: var(--accent-ai-soft); padding: 1px 6px; border-radius: 5px;
  font-variant-numeric: tabular-nums;
}
.vl__field-err { margin: 0; font-size: 10px; color: #ef4444; line-height: 1.4; }

/* ── Voice custom picker — never spawns native OS dropdown ── */
.vl__voice-persona { position: relative; }
.vl__voice-instruction { position: relative; flex: 1; }
.vl__voice-persona-btn {
  display: flex; align-items: center; gap: 8px; width: 100%;
  padding: 8px 10px; border: 1px solid var(--border); border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface) 80%, transparent);
  color: var(--ink); font: inherit; font-size: 13px; cursor: pointer;
  transition: border-color 120ms;
}
.vl__voice-persona-btn:hover:not(:disabled) { border-color: var(--accent-ai); }
.vl__voice-persona-btn:disabled { opacity: 0.45; cursor: default; }
.vl__voice-persona-dot {
  width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0;
}
.vl__voice-persona-name { flex: 1; text-align: left; font-weight: 500; }

/* Voice picker menu — scrollable, below the button, max 220px */
.vl__voice-persona-menu {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0;
  background: var(--surface); border: 1px solid var(--border);
  border-radius: var(--radius-md); box-shadow: 0 8px 28px oklch(0 0 0 / 0.18);
  z-index: 400; max-height: 220px; overflow-y: auto; scrollbar-width: none; padding: 4px 0;
}
.vl__voice-persona-menu::-webkit-scrollbar { display: none; }
.vl__voice-persona-loading { padding: 10px 12px; font-size: 11px; color: var(--ink-faint); }
.vl__voice-persona-opt {
  display: flex; align-items: center; gap: 8px;
  width: 100%; padding: 7px 12px;
  background: none; border: none; cursor: pointer; text-align: left;
  color: var(--ink); font-size: 12.5px; font: inherit;
  transition: background 80ms;
}
.vl__voice-persona-opt:hover { background: var(--surface-hover); }
.vl__voice-persona-opt--on { background: var(--accent-ai-soft); color: var(--accent-ai); font-weight: 600; }
.vl__voice-persona-opt-dot {
  width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
}
.vl__voice-persona-opt-name { flex: 1; }

/* Slider */
.vl__slider-row { display: flex; align-items: center; gap: 6px; }
.vl__slider-hint { font-size: 9px; color: var(--ink-faint); white-space: nowrap; flex-shrink: 0; }
.vl__range { flex: 1; accent-color: var(--accent-ai); cursor: pointer; }
.vl__range:disabled { opacity: 0.45; cursor: default; }

/* Model pill */
.vl__model-pill {
  padding: 7px 1px; border-radius: 100px;
  /* background: color-mix(in srgb, var(--surface-hover) 80%, transparent); */
  font-size: 9.5px; color: var(--ink-faint);
  font-family: 'SF Mono','Fira Code',monospace;
  display: inline-flex; align-items: center;
  gap: 4px;
}


/* ── Instructions ──────────────────────────────────────── */
.vl__instr {
  padding: 12px 14px;
  display: flex; flex-direction: column; gap: 8px;
  flex: 1; min-height: 0;
}
.vl__instr-head {
  display: flex; flex-direction: column; gap: 6px;
}
.vl__instr-label {
  font-size: 9px; font-weight: 700; letter-spacing: 0.6px;
  color: var(--accent-ai); text-transform: uppercase; flex-shrink: 0;
}

/* Preset selector — contained in sidebar, menu opens upward */
.vl__voice-persona-preset { position: relative; }
/* Menu opens UPWARD — bottom anchored to the trigger */
.vl__voice-persona-preset-menu {
  /* position/top/left/width set inline via presetMenuStyle computed (fixed coords) */
  background: var(--surface); border: 1px solid var(--border);
  border-radius: var(--radius-md); box-shadow: 0 8px 28px oklch(0 0 0 / 0.22);
  padding: 4px 0; max-height: 280px; overflow-y: auto; scrollbar-width: none;
}
.vl__voice-persona-preset-menu::-webkit-scrollbar { display: none; }

.vl__voice-persona-preset-section {
  padding: 6px 10px 2px; font-size: 9px; font-weight: 700;
  color: var(--ink-faint); text-transform: uppercase; letter-spacing: 0.5px;
}
.vl__voice-persona-preset-opt {
  display: flex; align-items: flex-start; gap: 8px;
  width: 100%; padding: 7px 10px;
  background: none; border: none; cursor: pointer; text-align: left; color: var(--ink);
  transition: background 100ms;
}
.vl__voice-persona-preset-opt:hover { background: var(--surface-hover); }
.vl__voice-persona-preset-opt--on { background: var(--accent-ai-soft); color: var(--accent-ai); }
.vl__voice-persona-preset-opt-icon { display: flex; align-items: center; justify-content: center; width: 18px; height: 18px; flex-shrink: 0; margin-top: 1px; }
.vl__voice-persona-preset-opt-name { font-size: 11.5px; font-weight: 600; display: block; }
.vl__voice-persona-preset-opt-desc {
  font-size: 9px; color: var(--ink-faint);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: block;
}

/* Instructions textarea (custom) */
.vl__instr-ta {
  width: 100%; padding: 9px 11px; font-size: 11.5px; line-height: 1.6;
  font-family: 'SF Mono','Fira Code',monospace;
  border: 1px solid var(--border); border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--bg) 75%, transparent);
  color: var(--ink); resize: vertical;
  flex: 1; min-height: 120px;
  box-sizing: border-box; transition: border-color 120ms;
  scrollbar-width: none;
}
.vl__instr-ta::-webkit-scrollbar { display: none; }
.vl__instr-ta:focus { outline: none; border-color: var(--accent-ai); box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-ai) 12%, transparent); }
.vl__instr-ta:disabled { opacity: 0.5; cursor: default; background: color-mix(in srgb, var(--surface) 60%, transparent); }
.vl__instr-hint {
  font-size: 9.5px; color: var(--ink-faint); line-height: 1.4; margin: 0;
}

/* ── RIGHT MAIN ───────────────────────────────────────── */
.vl__main {
  position: relative; z-index: 1;
  flex: 1; min-width: 0;
  display: flex; flex-direction: column;
  /* No overflow:hidden here — it clips the orb when scale(1.10) on speaking.
     Individual children (feed, controls) are fixed-size so no scroll risk. */
  overflow: clip; /* clips scroll but NOT transforms — CSS Overflow L4 */
}
/* Timer — inline in orb-stage row */
/* Status row — sits below the orb stage, never inside it */
.vl__status-row {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 0 20px 8px;
}
.vl__orb-timer {
  font-size: 11px; font-variant-numeric: tabular-nums;
  font-family: 'SF Mono','Fira Code',monospace;
  color: var(--ink-faint); min-width: 36px; text-align: right;
  opacity: 0; transition: opacity 300ms;
}
.vl__orb-timer--active { opacity: 1; }

/* Badge */
.vl__badge {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 3px 9px; border-radius: 100px;
  font-size: 10px; font-weight: 700;
  background: color-mix(in srgb, var(--surface) 85%, transparent);
  backdrop-filter: blur(2px);
  color: var(--ink-faint);
}
.vl__badge-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--ink-faint); }
/* Badge dot for active state — uses persona color so it matches the orb */
.vl__badge--active { color: var(--c1, var(--accent-ai)); }
.vl__badge--active .vl__badge-dot { background: var(--c1, var(--accent-ai)); animation: dot-blink 2s ease infinite; }
.vl__badge--connecting { color: #f59e0b; }
.vl__badge--connecting .vl__badge-dot { background: #f59e0b; animation: dot-blink 0.7s ease infinite; }
/* Hotkey status pill */
.vl__hotkey-pill {
  display: flex; align-items: center; gap: 5px;
  margin-top: 6px; padding: 3px 8px;
  border-radius: 999px; font-size: 10px;
  background: color-mix(in srgb, var(--border) 60%, transparent);
  color: var(--ink-faint); width: fit-content;
}
.vl__hotkey-pill--active {
  background: color-mix(in srgb, var(--accent-ai) 12%, transparent);
  color: var(--accent-ai);
}
.vl__badge--error { color: #ef4444; }
.vl__badge--error .vl__badge-dot { background: #ef4444; }
@keyframes dot-blink { 0%,100%{opacity:1} 50%{opacity:0.3} }

/* Error bar */
.vl__error-bar {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 16px; flex-shrink: 0;
  background: #fee2e2; color: #dc2626; font-size: 12px;
  border-bottom: 1px solid #fca5a5;
}
.vl__error-bar span { flex: 1; }
.vl__error-close {
  border: none; background: transparent; color: inherit; cursor: pointer;
  opacity: 0.65; display: flex;
}
.vl__error-close:hover { opacity: 1; }

/* ── ORB STAGE — fixed, centered, never grows ──────────── */
.vl__orb-stage {
  flex: 1;
  min-height: 0;
  position: relative;   /* anchor for the floating video preview */
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 20px;
  padding: 24px 16px;
  overflow: visible;
}

/* Camera / screen-share preview — floats inside orb stage, top-left corner.
   position:absolute keeps it out of the flex flow so bars + orb are untouched. */
.vl__video-preview {
  position: absolute;
  top: 12px;
  left: 12px;
  width: 120px;
  border-radius: 10px;
  overflow: hidden;
  border: 1.5px solid var(--accent-ai);
  box-shadow: 0 4px 16px rgba(0,0,0,.25);
  z-index: 10;
  background: #000;
}
.vl__video-preview__video {
  width: 100%;
  display: block;
  aspect-ratio: 4/3;
  object-fit: cover;
}
.vl__video-preview__badge {
  position: absolute;
  bottom: 4px;
  left: 4px;
  display: flex;
  align-items: center;
  gap: 3px;
  padding: 2px 5px;
  font-size: 7px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: .4px;
  background: color-mix(in srgb, var(--accent-ai) 85%, transparent);
  color: #fff;
  border-radius: 3px;
}

/* Mic activity bars */
.vl__bars {
  display: flex; align-items: center; gap: 2.5px; width: 50px;
  height: 56px; flex-shrink: 0;
}
.vl__bar {
  width: 3px; border-radius: 2px; min-height: 3px;
  background: var(--accent-ai); opacity: 0.3;
  transition: height 60ms ease, opacity 300ms ease;
}
.vl__bars--active .vl__bar { opacity: 0.7; }
/* Bars use persona color for all active states */
.vl__bar--listening { background: var(--c1, var(--accent-ai)); }
.vl__bar--speaking  { background: var(--c1, var(--accent-ai)); }
.vl__bar--thinking  { background: #f59e0b; animation: bar-glow 0.8s ease-in-out infinite alternate; }
@keyframes bar-glow { 0%{opacity:0.4} 100%{opacity:0.9} }

/* ── Orb — transform-only, ZERO layout reflow ──────────── */
.vl__orb {
  position: relative;
  /* 200px base. At scale(1.08) = 216px visual.
     orb-stage padding:32px gives 264px available — no clipping ever. */
  width: 200px; height: 200px; flex-shrink: 0;
  transition: transform 0.5s cubic-bezier(0.34, 1.56, 0.64, 1);
}
.vl__orb--idle     { transform: scale(0.86); }
.vl__orb--thinking { transform: scale(0.93); }
.vl__orb--listening { transform: scale(1.00); }
.vl__orb--speaking  { transform: scale(1.08); }

/* Sphere tints — persona color, visible in both light and dark */
.vl__orb-sphere {
  position: absolute; inset: 0; border-radius: 50%; overflow: hidden;
  transition: background 0.5s;
  background: color-mix(in srgb, var(--c1, var(--accent-ai)) 6%, transparent);
}
/* listening: persona tint, normal energy */
.vl__orb--listening .vl__orb-sphere {
  background: color-mix(in srgb, var(--c1, var(--accent-ai)) 10%, transparent);
}
/* speaking: persona tint, more vivid */
.vl__orb--speaking  .vl__orb-sphere {
  background: color-mix(in srgb, var(--c1, var(--accent-ai)) 14%, transparent);
}
/* thinking: amber — system state, not persona */
.vl__orb--thinking  .vl__orb-sphere { background: color-mix(in srgb, #f59e0b 8%, transparent); }

/* Blobs — FIXED size/position, only border-radius + color animate */
.vl__blob { position: absolute; opacity: 0.5; transition: background 0.5s, opacity 0.5s; }
.vl__blob--1 { width:70%; height:70%; top:15%; left:15%; animation:b1 8s ease-in-out infinite; }
.vl__blob--2 { width:60%; height:60%; top:22%; left:22%; animation:b2 6s ease-in-out infinite reverse; }
.vl__blob--3 { width:50%; height:50%; top:28%; left:28%; animation:b3 4s ease-in-out infinite; }

/* State blob colors */
.vl__orb--idle .vl__blob--1 { background:radial-gradient(circle at 30% 30%,var(--c1,#8b5cf6),transparent 68%); opacity:.30; }
.vl__orb--idle .vl__blob--2 { background:radial-gradient(circle at 70% 40%,var(--c2,#a78bfa),transparent 68%); opacity:.25; }
.vl__orb--idle .vl__blob--3 { background:radial-gradient(circle at 50% 70%,var(--c3,#c4b5fd),transparent 68%); opacity:.20; }
.vl__orb--thinking .vl__blob--1 { background:radial-gradient(circle at 30% 30%,#f59e0b,transparent 68%); animation-duration:2.2s; }
.vl__orb--thinking .vl__blob--2 { background:radial-gradient(circle at 70% 40%,#fbbf24,transparent 68%); animation-duration:1.6s; }
.vl__orb--thinking .vl__blob--3 { background:radial-gradient(circle at 50% 70%,#fcd34d,transparent 68%); animation-duration:1.1s; }
/* Listening — persona color, steady medium pace */
.vl__orb--listening .vl__blob--1 { background:radial-gradient(circle at 30% 30%,var(--c1,var(--accent-ai)),transparent 68%); opacity:.50; animation-duration:3.5s; }
.vl__orb--listening .vl__blob--2 { background:radial-gradient(circle at 70% 40%,var(--c2,var(--accent-ai)),transparent 68%); opacity:.42; animation-duration:2.8s; }
.vl__orb--listening .vl__blob--3 { background:radial-gradient(circle at 50% 70%,var(--c3,var(--accent-ai)),transparent 68%); opacity:.35; animation-duration:2.2s; }
/* Speaking — persona color, full brightness + fast pulse */
.vl__orb--speaking .vl__blob--1 { background:radial-gradient(circle at 30% 30%,var(--c1,var(--accent-ai)),transparent 68%); opacity:.65; animation-duration:2s; }
.vl__orb--speaking .vl__blob--2 { background:radial-gradient(circle at 70% 40%,var(--c2,var(--accent-ai)),transparent 68%); opacity:.58; animation-duration:1.6s; }
.vl__orb--speaking .vl__blob--3 { background:radial-gradient(circle at 50% 70%,var(--c3,var(--accent-ai)),transparent 68%); opacity:.50; animation-duration:1.2s; }

@keyframes b1 {
  0%  { border-radius:42% 58% 55% 45% / 50% 42% 58% 50%; }
  25% { border-radius:55% 45% 42% 58% / 45% 55% 42% 58%; }
  50% { border-radius:48% 52% 60% 40% / 55% 45% 50% 50%; }
  75% { border-radius:40% 60% 45% 55% / 50% 48% 55% 45%; }
  100%{ border-radius:42% 58% 55% 45% / 50% 42% 58% 50%; }
}
@keyframes b2 {
  0%  { border-radius:50% 50% 45% 55% / 55% 45% 50% 50%; }
  33% { border-radius:45% 55% 50% 50% / 50% 50% 55% 45%; }
  66% { border-radius:55% 45% 50% 50% / 45% 55% 50% 50%; }
  100%{ border-radius:50% 50% 45% 55% / 55% 45% 50% 50%; }
}
@keyframes b3 {
  0%  { border-radius:45% 55% 50% 50% / 50% 50% 45% 55%; }
  50% { border-radius:55% 45% 55% 45% / 45% 55% 50% 50%; }
  100%{ border-radius:45% 55% 50% 50% / 50% 50% 45% 55%; }
}

/* Speaking pulse ring — uses transform only, no inset bleed,
   so overflow:clip on the parent column cannot crop it. */
.vl__orb-pulse {
  position: absolute; inset: 0; border-radius: 50%;
  border: 2px solid var(--c1, var(--accent-ai)); opacity: 0;
  animation: orb-ring 1.6s ease-out infinite;
}
@keyframes orb-ring {
  0%   { opacity: .55; transform: scale(0.96); }
  100% { opacity: 0;   transform: scale(1.28); }
}

/* Muted overlay */
.vl__orb-mute {
  position:absolute; inset:0; border-radius:50%;
  background:oklch(0.18 0.01 0 / 0.62);
  display:flex; align-items:center; justify-content:center;
}

/* Voice identity */
.vl__identity {
  display: flex; flex-direction: column; align-items: center; gap: 2px;
  flex-shrink: 0; padding: 0 16px 4px;
}
.vl__identity-name {
  margin: 0; font-size: 17px; font-weight: 700; letter-spacing: -0.02em;
}
.vl__identity-model {
  margin: 0; font-size: 9.5px; color: var(--ink-faint);
  font-family: 'SF Mono','Fira Code',monospace;
}

/* State label */
.vl__state {
  margin: 0; flex-shrink: 0;
  font-size: 12.5px; font-weight: 500; text-align: center;
  color: var(--ink-faint); transition: color 0.3s;
  padding: 0 16px 8px;
}
.vl__state--listening { color: #3b82f6; }
.vl__state--speaking  { color: #10b981; }
.vl__state--thinking  { color: #f59e0b; }
.vl__state--muted     { color: var(--ink-faint); font-style: italic; }
.vl__state--err       { color: #ef4444; }

/* ── TRANSCRIPT — fixed height, scrollable history ── */
.vl__transcript {
  flex-shrink: 0;
  height: 150px;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 6px 16px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  scroll-behavior: smooth;
  scrollbar-width: none;
  /* Frosted glass blur */
  backdrop-filter: blur(28px) saturate(1.5);
  -webkit-backdrop-filter: blur(28px) saturate(1.5);
  /* background: color-mix(in srgb, var(--surface) 50%, transparent); */
  border-radius: var(--radius-lg, 12px);
  border: 0.05px solid color-mix(in srgb, var(--border, #ffffff20) 35%, transparent);
}
.vl__transcript::-webkit-scrollbar { display: none; }

.vl__transcript-empty {
  display: flex; flex-direction: column; align-items: center; gap: 8px;
  padding: 32px 20px; font-size: 11px; color: var(--ink-faint);
  justify-content: center; height: 100%;
}

/* One entry per speaker turn; typewriter update in-place */
.vl__turn { display: flex; align-items: flex-start; gap: 8px; animation: turn-in 0.12s ease; }
.vl__turn--user { flex-direction: row-reverse; }
/* Interim: fade only, no blinking cursor */
.vl__turn--interim .vl__turn-text { opacity: 0.55; }
/* Consecutive same-speaker turns sit closer together */
.vl__turn:not(:first-child) { margin-top: 2px; }
/* When speaker switches, more breathing room */
.vl__turn-who + .vl__turn-text { margin-top: 0; }

.vl__turn-av {
  width: 22px; height: 22px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center; margin-top: 2px;
}
.vl__turn-av--user  { background: color-mix(in srgb, var(--accent) 12%, transparent); color: color-mix(in srgb, var(--accent) 60%, var(--ink-faint)); }
.vl__turn-av--agent { background: color-mix(in srgb, var(--accent-ai) 12%, transparent); color: color-mix(in srgb, var(--accent-ai) 60%, var(--ink-faint)); }

.vl__turn-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 3px; max-width: 88%; }
.vl__turn--user .vl__turn-body { align-items: flex-end; }

.vl__turn-who {
  font-size: 9px; font-weight: 700; text-transform: uppercase;
  letter-spacing: 0.4px; color: var(--ink-faint);
}
.vl__turn-text {
  margin: 0; font-size: 13px; line-height: 1.55; color: var(--ink);
  word-break: break-word; white-space: pre-wrap;
  padding: 8px 12px; border-radius: var(--radius-md);
}
.vl__turn--user .vl__turn-text {
  background: color-mix(in srgb, var(--accent) 4%, transparent);
  color: color-mix(in srgb, var(--ink) 60%, transparent);
  border-radius: var(--radius-md) var(--radius-sm) var(--radius-md) var(--radius-md);
  backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px);
}
.vl__turn--agent .vl__turn-text {
  background: color-mix(in srgb, var(--accent-ai) 4%, transparent);
  color: color-mix(in srgb, var(--ink) 55%, transparent);
  border-radius: var(--radius-sm) var(--radius-md) var(--radius-md) var(--radius-md);
  backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px);
}
@supports not (color: color-mix(in srgb, red 50%, blue)) {
  .vl__turn--user .vl__turn-text  { background: var(--accent-soft); }
  .vl__turn--agent .vl__turn-text { background: var(--accent-ai-soft); }
}
/* Cursor hidden — opacity fade on .vl__turn--interim is enough */
.vl__turn-cursor { display: none; }
@keyframes turn-in { from{opacity:0;transform:translateY(3px)} to{opacity:1;transform:translateY(0)} }

/* ── CONTROLS ──────────────────────────────────────────── */
.vl__controls {
  flex-shrink: 0;
  padding: 14px 20px 20px;
  display: flex; align-items: center; justify-content: center; gap: 14px;
}

/* Start button */
.vl__btn-start {
  display: flex; align-items: center; gap: 9px;
  padding: 12px 30px; border: none; border-radius: 100px;
  background: var(--accent-ai); color: #fff;
  font-size: 14px; font-weight: 600; cursor: pointer;
  box-shadow: 0 2px 14px color-mix(in srgb, var(--accent-ai) 38%, transparent);
  transition: background 150ms, transform 100ms, box-shadow 150ms;
}
.vl__btn-start:hover:not(:disabled) {
  background: color-mix(in srgb, var(--accent-ai) 82%, #000);
  transform: translateY(-1px);
  box-shadow: 0 5px 20px color-mix(in srgb, var(--accent-ai) 48%, transparent);
}
.vl__btn-start:disabled { opacity: 0.4; cursor: default; box-shadow: none; }

/* Mute button */
.vl__btn-mic {
  position: relative; width: 54px; height: 54px; border-radius: 50%;
  border: 2px solid var(--border); background: var(--surface); color: var(--ink-muted);
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; transition: background 150ms, border-color 150ms, color 150ms;
}
.vl__btn-mic--listening { border-color: #3b82f6; color: #3b82f6; background: rgba(59,130,246,.09); }
.vl__btn-mic--speaking  { border-color: #10b981; color: #10b981; background: rgba(16,185,129,.09); }
.vl__btn-mic--thinking  { border-color: #f59e0b; color: #f59e0b; background: rgba(245,158,11,.09); }
.vl__btn-mic--muted { border-color: var(--border); color: var(--ink-faint); background: var(--surface-hover); }
.vl__btn-mic:hover { transform: scale(1.06); }

.vl__btn-mic-ring {
  position: absolute; inset: -7px; border-radius: 50%;
  border: 2px solid #3b82f6; opacity: 0; animation: orb-ring 2s ease-out infinite;
}

/* End button */
.vl__btn-end {
  width: 54px; height: 54px; border-radius: 50%; border: none;
  background: rgba(239,68,68,.12); color: #ef4444;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; transition: background 150ms, transform 100ms;
}
.vl__btn-end:hover { background: rgba(239,68,68,.22); transform: scale(1.06); }

/* Tool buttons (camera, screen, keyboard) */
.vl__btn-tool {
  width: 38px; height: 38px; border-radius: 50%;
  border: 1px solid var(--border); background: var(--surface);
  color: var(--ink-faint); font-size: 13px;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; transition: all 120ms;
}
.vl__btn-tool:hover { background: var(--surface-hover); color: var(--ink); }
.vl__btn-tool--on { border-color: var(--accent-ai); background: color-mix(in srgb, var(--accent-ai) 10%, transparent); color: var(--accent-ai); }

/* Collapsible text chat input */
.vl__text-input {
  flex-shrink: 0;
  display: flex; gap: 6px;
  padding: 0 20px 14px;
}
.vl__text-input input {
  flex: 1; padding: 8px 12px; font-size: 12px;
  border: 1px solid var(--border); border-radius: 20px;
  background: var(--surface); color: var(--ink);
  outline: none;
}
.vl__text-input input:focus { border-color: var(--accent-ai); }
.vl__text-input button {
  width: 32px; height: 32px; border-radius: 50%; flex-shrink: 0;
  border: none; background: var(--accent-ai); color: #fff;
  font-size: 11px; cursor: pointer; display: flex; align-items: center; justify-content: center;
  transition: opacity 150ms;
}
.vl__text-input button:disabled { opacity: 0.35; cursor: default; }

/* Spinner */
.vl__spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
