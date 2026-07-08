<script setup>
// Voice Assistant — realtime rebuild (2026-07-08). Replaces the old
// push-to-talk record→stop→POST/chat flow with a real continuous Gemini
// Live session: no send button, audio streams both ways over one WS for
// the whole conversation. See
// docs/superpowers/specs/2026-07-08-voice-live-realtime-design.md and
// internal/voicelive/voicelive.go for the protocol this implements.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useCoreStore } from '../stores/core'
import { connectLiveWs, createCapture, createPlayback } from '../lib/voiceLive'

const core = useCoreStore()

const DEFAULT_INSTRUCTIONS =
  'You are Sarza, a helpful, warm voice assistant. Reply briefly and conversationally in the language the user spoke.'
const instructions = ref(localStorage.getItem('agentic-desk-voice-instructions') || DEFAULT_INSTRUCTIONS)
watch(instructions, (v) => localStorage.setItem('agentic-desk-voice-instructions', v))

const temperature = ref(Number(localStorage.getItem('agentic-desk-voice-temperature')) || 0.8)
watch(temperature, (v) => localStorage.setItem('agentic-desk-voice-temperature', String(v)))

// Populated from GET /voice/live/config (internal/voicelive.Voices) — the
// real prebuilt Gemini voice catalog, not a fixed local persona list.
const voices = ref([])
const defaultModel = ref('')
const voiceName = ref(localStorage.getItem('agentic-desk-voice-name') || '')
watch(voiceName, (v) => localStorage.setItem('agentic-desk-voice-name', v))

const configError = ref('')

async function loadConfig() {
  try {
    const res = await fetch(`${core.baseUrl}/voice/live/config`)
    if (!res.ok) throw new Error(`config: ${res.status}`)
    const data = await res.json()
    voices.value = data.voices || []
    defaultModel.value = data.defaultModel || ''
    if (!voiceName.value || !voices.value.includes(voiceName.value)) {
      voiceName.value = voices.value[0] || ''
    }
  } catch (err) {
    configError.value = `Couldn't load voice config: ${err.message || err}`
  }
}

// connectionState drives the topbar status badge; agentState drives the
// orb (idle/listening/thinking/speaking), same vocabulary the old
// implementation used so nothing downstream needs to change meaning.
const connectionState = ref('idle') // idle | connecting | active | error
const speaking = ref(false)
const audioLevel = ref(0)
const elapsed = ref(0)
const sessionError = ref('')
const messages = ref([]) // {role:'user'|'agent', text, isFinal}
const scroller = ref(null)

let ws = null
let capture = null
let playback = null
let speakingTimeout = 0
let timerId = 0
let audioCtx = null
let levelRaf = 0

const agentState = computed(() => {
  if (connectionState.value === 'connecting') return 'thinking'
  if (speaking.value) return 'speaking'
  if (connectionState.value === 'active' && audioLevel.value > 0.08) return 'listening'
  return 'idle'
})

const STATE_LABEL = {
  idle: connectionState.value === 'error' ? 'Error' : 'Ready',
  listening: 'Listening…',
  thinking: 'Connecting…',
  speaking: 'Speaking',
}

const statusLabel = computed(() => {
  if (connectionState.value === 'error') return 'Error'
  if (connectionState.value === 'connecting') return 'Connecting…'
  if (connectionState.value === 'active') return STATE_LABEL[agentState.value]
  return 'Ready'
})

function startLevelMeter(stream) {
  audioCtx = new (window.AudioContext || window.webkitAudioContext)()
  const src = audioCtx.createMediaStreamSource(stream)
  const analyser = audioCtx.createAnalyser()
  analyser.fftSize = 256
  src.connect(analyser)
  const buf = new Uint8Array(analyser.frequencyBinCount)
  const tick = () => {
    analyser.getByteTimeDomainData(buf)
    let sum = 0
    for (const v of buf) sum += (v - 128) * (v - 128)
    audioLevel.value = Math.min(1, Math.sqrt(sum / buf.length) / 40)
    levelRaf = requestAnimationFrame(tick)
  }
  levelRaf = requestAnimationFrame(tick)
}

function stopLevelMeter() {
  cancelAnimationFrame(levelRaf)
  audioLevel.value = 0
  audioCtx?.close()
  audioCtx = null
}

function appendTranscript(payload) {
  const last = messages.value[messages.value.length - 1]
  if (last && last.role === payload.role && !last.isFinal) {
    last.text = payload.text
    last.isFinal = payload.isFinal
  } else {
    messages.value.push({ role: payload.role, text: payload.text, isFinal: payload.isFinal })
  }
}

async function startSession() {
  if (connectionState.value === 'connecting' || connectionState.value === 'active') return
  sessionError.value = ''
  connectionState.value = 'connecting'

  ws = connectLiveWs({
    baseUrl: core.baseUrl,
    model: defaultModel.value,
    voice: voiceName.value,
    temperature: temperature.value,
    instructions: instructions.value,
    onSessionState: async (payload) => {
      if (payload.state !== 'active') return
      connectionState.value = 'active'
      elapsed.value = 0
      timerId = setInterval(() => elapsed.value++, 1000)
      playback = createPlayback()
      try {
        capture = await createCapture((chunk) => ws.sendAudio(chunk))
        startLevelMeter(capture.stream)
      } catch (err) {
        sessionError.value = `Microphone unavailable: ${err?.message || err}`
        stopSession()
      }
    },
    onTranscript: (payload) => {
      appendTranscript(payload)
    },
    onAudio: (chunk) => {
      playback?.playChunk(chunk)
      speaking.value = true
      clearTimeout(speakingTimeout)
      speakingTimeout = setTimeout(() => (speaking.value = false), 500)
    },
    onInterrupt: () => {
      playback?.flush()
      speaking.value = false
    },
    onError: (msg) => {
      sessionError.value = msg
      connectionState.value = 'error'
      teardown()
    },
    onClose: () => {
      if (connectionState.value !== 'error') connectionState.value = 'idle'
      teardown()
    },
  })
}

function teardown() {
  clearInterval(timerId)
  stopLevelMeter()
  capture?.stop()
  capture = null
  playback?.stop()
  playback = null
  speaking.value = false
}

function stopSession() {
  ws?.end()
  ws?.close()
  ws = null
  teardown()
  if (connectionState.value !== 'error') connectionState.value = 'idle'
}

const timer = computed(() => {
  const m = String(Math.floor(elapsed.value / 60)).padStart(2, '0')
  const s = String(elapsed.value % 60).padStart(2, '0')
  return `${m}:${s}`
})

// Deterministic per-voice orb color so switching the selected Gemini
// voice gives visually distinct feedback without a fixed named-persona
// list (the old Bart/Arnold/Terry/Mark set didn't map to anything real).
function orbColorsFor(name) {
  let hash = 0
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) >>> 0
  const hue = hash % 360
  return [`oklch(0.62 0.16 ${hue})`, `oklch(0.72 0.13 ${(hue + 30) % 360})`, `oklch(0.8 0.1 ${(hue + 60) % 360})`]
}
const orbStyle = computed(() => {
  const [c1, c2, c3] = orbColorsFor(voiceName.value || 'sarza')
  return { '--c1': c1, '--c2': c2, '--c3': c3 }
})

watch(
  () => messages.value.length,
  async () => {
    await nextTick()
    if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
  },
)

onMounted(loadConfig)

onBeforeUnmount(() => {
  stopSession()
})
</script>

<template>
  <div class="voice-live">
    <!-- CONFIG + TRANSCRIPT SIDEBAR — always visible, no collapse toggle. -->
    <aside class="config-sidebar">
      <div class="config-sidebar__header"><span>CONFIGURATION</span></div>
      <div class="config-sidebar__body">
        <label class="field">
          <span class="field__label">Voice</span>
          <select v-model="voiceName" :disabled="connectionState !== 'idle'">
            <option v-for="v in voices" :key="v" :value="v">{{ v }}</option>
          </select>
        </label>
        <label class="field">
          <span class="field__label">Temperature <span class="field__value">{{ temperature.toFixed(1) }}</span></span>
          <input v-model.number="temperature" type="range" min="0" max="2" step="0.1" />
        </label>
        <p v-if="defaultModel" class="model-note">Model: {{ defaultModel }}</p>
        <p v-if="configError" class="voice-error">{{ configError }}</p>
      </div>

      <div class="sidebar-divider" />

      <div class="sidebar-transcript">
        <div class="sidebar-transcript__header">
          <span>TRANSCRIPT</span>
          <span v-if="messages.length" class="sidebar-transcript__count">{{ messages.length }}</span>
        </div>

        <div ref="scroller" class="sidebar-transcript__scroll">
          <div v-if="!messages.length" class="sidebar-transcript__empty">Conversation will appear here</div>

          <div v-for="(m, i) in messages" :key="i" class="chat-bubble" :class="`chat-bubble--${m.role}`">
            <span class="chat-bubble__avatar" :class="`chat-bubble__avatar--${m.role}`">
              <svg v-if="m.role === 'user'" viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="8" r="4" />
                <path d="M4 20c0-4 3.5-7 8-7s8 3 8 7" />
              </svg>
              <svg v-else viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="4" y="8" width="16" height="11" rx="2" />
                <path d="M12 8V4M9 4h6" />
                <circle cx="9" cy="13.2" r="1" fill="currentColor" stroke="none" />
                <circle cx="15" cy="13.2" r="1" fill="currentColor" stroke="none" />
              </svg>
            </span>
            <span class="chat-bubble__body">
              <span class="chat-bubble__sender">{{ m.role === 'user' ? 'You' : 'Agent' }}</span>
              <span class="chat-bubble__text">{{ m.text }}</span>
            </span>
          </div>
        </div>
      </div>
    </aside>

    <!-- MAIN COLUMN -->
    <div class="voice-live__main">
      <header class="topbar">
        <div class="topbar__left">
          <svg class="topbar__icon" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="2" />
            <path d="M8.5 8.5a5 5 0 000 7M15.5 8.5a5 5 0 010 7M5.5 5.5a9 9 0 000 13M18.5 5.5a9 9 0 010 13" />
          </svg>
          <span class="topbar__title">Voice Assistant</span>
        </div>
        <div class="topbar__right">
          <span v-if="connectionState === 'active'" class="timer">{{ timer }}</span>
          <span class="status-badge" :data-state="connectionState === 'active' ? agentState : connectionState">
            <i></i>
            {{ statusLabel }}
          </span>
        </div>
      </header>

      <!-- Instructions panel — full-width, directly under the topbar. -->
      <div class="instructions-panel">
        <span class="instructions-panel__label">INSTRUCTIONS</span>
        <textarea
          v-model="instructions"
          :disabled="connectionState !== 'idle'"
          placeholder="System instructions for the voice assistant..."
          rows="3"
        />
      </div>

      <p v-if="sessionError" class="voice-error" role="alert">{{ sessionError }}</p>

      <div class="conversation">
        <div class="orb-stage">
          <div
            class="orb"
            :data-state="agentState"
            :style="{ '--level': audioLevel, ...orbStyle }"
            role="img"
            :aria-label="`${voiceName || 'Sarza'} is ${(connectionState === 'active' ? agentState : connectionState) }`"
          >
            <span class="blob blob-1"></span>
            <span class="blob blob-2"></span>
            <span class="blob blob-3"></span>
          </div>
          <p class="orb-name">{{ voiceName || 'Sarza' }}</p>
          <p class="orb-state-label" :data-state="agentState">{{ statusLabel }}</p>
        </div>
      </div>

      <!-- No send button: one Start/Stop toggle drives the whole realtime
           session — audio streams continuously in both directions once
           started, Gemini's own VAD handles turn-taking. -->
      <div class="voice-controls">
        <button
          type="button"
          class="ctl ctl-mic"
          :class="{ 'is-active': connectionState === 'active' || connectionState === 'connecting' }"
          :disabled="connectionState === 'connecting' || !voiceName"
          :title="connectionState === 'idle' || connectionState === 'error' ? 'Start conversation' : 'End conversation'"
          @click="connectionState === 'idle' || connectionState === 'error' ? startSession() : stopSession()"
        >
          <svg v-if="connectionState === 'idle' || connectionState === 'error'" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <rect x="9" y="3" width="6" height="11" rx="3" />
            <path d="M5.5 11a6.5 6.5 0 0 0 13 0" />
            <line x1="12" y1="17.5" x2="12" y2="21" />
          </svg>
          <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
            <rect x="7" y="7" width="10" height="10" rx="1.5" />
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.voice-live {
  display: flex;
  height: 100%;
  min-height: 600px;
}

/* ── Config + transcript sidebar — fixed, always visible ── */
.config-sidebar {
  width: 300px;
  flex-shrink: 0;
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.config-sidebar__header {
  padding: 12px 14px 4px;
}

.config-sidebar__header span {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 1px;
  color: var(--ink-faint);
}

.config-sidebar__body {
  padding: 8px 14px 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex-shrink: 0;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.field__label {
  font-size: 11px;
  font-weight: 600;
  color: var(--ink-muted);
  display: flex;
  justify-content: space-between;
}

.field__value {
  font-variant-numeric: tabular-nums;
  color: var(--ink-faint);
}

.field select {
  padding: 6px 8px;
  font-size: 12.5px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--ink);
}

.field select:disabled {
  opacity: 0.6;
}

.field input[type='range'] {
  accent-color: var(--accent);
}

.model-note {
  margin: 0;
  font-size: 11px;
  color: var(--ink-faint);
}

.sidebar-divider {
  height: 1px;
  background: var(--border);
  margin: 4px 14px 8px;
  flex-shrink: 0;
}

.sidebar-transcript {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}

.sidebar-transcript__header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 14px 8px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 1px;
  color: var(--ink-faint);
  flex-shrink: 0;
}

.sidebar-transcript__count {
  margin-left: auto;
  font-size: 10px;
  font-weight: 700;
  background: var(--surface-hover);
  color: var(--ink-muted);
  padding: 1px 7px;
  border-radius: 8px;
}

.sidebar-transcript__scroll {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 4px 10px 12px;
}

.sidebar-transcript__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 56px;
  font-size: 11px;
  color: var(--ink-faint);
  text-align: center;
}

.chat-bubble {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
}

.chat-bubble--user {
  background: var(--accent-soft);
}

.chat-bubble--agent {
  background: var(--surface-hover);
}

.chat-bubble__avatar {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.chat-bubble__avatar--user {
  background: var(--accent-soft);
  color: var(--accent);
}

.chat-bubble__avatar--agent {
  background: var(--accent-ai-soft);
  color: var(--accent-ai);
}

.chat-bubble__body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.chat-bubble__sender {
  font-size: 9px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--ink-faint);
}

.chat-bubble__text {
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink);
  white-space: pre-wrap;
  word-break: break-word;
}

/* ── Main column ── */
.voice-live__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px 16px;
}

.topbar__left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.topbar__icon {
  color: var(--accent);
}

.topbar__title {
  font-size: 15px;
  font-weight: 700;
  color: var(--ink);
}

.topbar__right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.timer {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: var(--ink-muted);
}

.instructions-panel {
  margin: 0 20px 12px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  flex-shrink: 0;
}

.instructions-panel__label {
  display: block;
  margin-bottom: 4px;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.6px;
  color: var(--accent);
}

.instructions-panel textarea {
  width: 100%;
  padding: 6px 8px;
  font-size: 11px;
  line-height: 1.45;
  font-family: 'SF Mono', 'Fira Code', monospace;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--ink);
  resize: vertical;
  outline: none;
}

.instructions-panel textarea:focus {
  border-color: var(--accent);
}

.instructions-panel textarea:disabled {
  opacity: 0.5;
}

.status-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface);
  font-size: 11.5px;
  font-weight: 600;
  color: var(--ink-muted);
}

.status-badge i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--ink-faint);
}

.status-badge[data-state='listening'] i {
  background: #10b981;
}

.status-badge[data-state='connecting'] i,
.status-badge[data-state='thinking'] i {
  background: #f59e0b;
}

.status-badge[data-state='speaking'] i {
  background: var(--accent);
}

.status-badge[data-state='error'] i {
  background: var(--danger);
}

.voice-error {
  margin: 0 20px 10px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 12px;
}

.conversation {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.orb-stage {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 18px 0 10px;
}

.orb {
  position: relative;
  width: 300px;
  height: 300px;
  --state-scale: 1;
  transform: scale(calc((1 + var(--level, 0) * 0.25) * var(--state-scale)));
  transition: transform 300ms var(--ease-out-expo);
}

.orb[data-state='thinking'] {
  --state-scale: 1.05;
}

.orb[data-state='listening'] {
  --state-scale: 1.1;
}

.orb[data-state='speaking'] {
  --state-scale: 1.18;
}

.blob {
  position: absolute;
  inset: 10%;
  border-radius: 50%;
  opacity: 0.55;
  filter: blur(1px);
}

.blob-1 {
  background: radial-gradient(circle at 30% 30%, var(--c1, var(--accent)), transparent 70%);
  animation: orb-rotate 7s linear infinite;
}

.blob-2 {
  background: radial-gradient(circle at 70% 40%, var(--c2, var(--accent-ai)), transparent 70%);
  animation: orb-rotate 9s linear infinite reverse;
}

.blob-3 {
  background: radial-gradient(circle at 50% 70%, var(--c3, #a78bfa), transparent 70%);
  animation: orb-rotate 11s linear infinite;
}

.orb-name {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--ink);
}

.orb-state-label {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--ink-faint);
}

.orb-state-label[data-state='listening'] {
  color: #10b981;
}

.orb-state-label[data-state='thinking'] {
  color: #f59e0b;
}

.orb-state-label[data-state='speaking'] {
  color: var(--accent);
}

@keyframes orb-rotate {
  from {
    transform: rotate(0deg) translateX(6px) rotate(0deg);
  }
  to {
    transform: rotate(360deg) translateX(6px) rotate(-360deg);
  }
}

.orb[data-state='idle'] .blob {
  opacity: 0.3;
}

.orb[data-state='thinking'] .blob {
  animation-duration: 2.5s;
}

.orb[data-state='speaking'] {
  animation: orb-pulse 1.1s ease-in-out infinite;
}

@keyframes orb-pulse {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.12);
  }
}

@media (prefers-reduced-motion: reduce) {
  .blob,
  .orb {
    animation: none !important;
    transition: none;
  }
}

.voice-controls {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 14px;
  padding: 10px 0 26px;
  flex-shrink: 0;
}

.ctl {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border);
  border-radius: 50%;
  background: var(--surface);
  color: var(--ink);
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.ctl-mic {
  width: 62px;
  height: 62px;
}

.ctl-mic:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent);
}

.ctl-mic:disabled {
  opacity: 0.4;
  cursor: default;
}

.ctl-mic.is-active {
  background: var(--danger, #b3261e);
  border-color: var(--danger, #b3261e);
  color: #fff;
}
</style>
