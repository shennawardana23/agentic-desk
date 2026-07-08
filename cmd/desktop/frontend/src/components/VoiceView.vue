<script setup>
/**
 * VoiceView.vue — Gemini Live realtime voice assistant.
 *
 * Layout (two-column):
 *   LEFT  280px sidebar: config (voice, temp) + instructions (always shown, editable)
 *   RIGHT flex-column:   topbar → orb-stage (fixed) → transcript feed (fixed, no layout shift) → controls
 *
 * Audio noise handling:
 *   - Noise floor calibration: first 1.5s after capture starts = measure ambient RMS
 *   - Dynamic threshold = noiseFloor * 2.5  (hysteresis prevents flutter)
 *   - audioLevel is only set > 0 when signal exceeds threshold + small decay smoothing
 *
 * Transcript feed:
 *   - Fixed height container, overflow hidden, newest message swaps in from bottom
 *   - Interim (non-final) messages shown at reduced opacity — live "typing" effect
 *   - No scrollbar visible; last 3 messages always visible
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { connectLiveWs, createCapture, createPlayback } from '../lib/voiceLive'
import { useCoreStore } from '../stores/core'

const core = useCoreStore()

// ── Agent presets ──────────────────────────────────────────────────────────
const PRESETS = [
  { id: 'helpful-ai',  name: 'Helpful AI',       desc: 'Warm general assistant',         icon: '🤖', instruction: 'You are a helpful, warm, concise voice assistant built on Gemini Live. Reply in the language the user spoke. Keep answers short — one to three sentences unless asked for more.' },
  { id: 'tutor',       name: 'Language Tutor',    desc: 'Interactive language learning',   icon: '📚', instruction: 'You are a patient language tutor. Correct mistakes gently, explain grammar briefly, and encourage the student. Keep responses short and conversational.' },
  { id: 'creative',    name: 'Creative Partner',  desc: 'Brainstorming & ideas',           icon: '🎨', instruction: 'You are a creative collaborator. Offer imaginative ideas, build on what the user says, and think outside the box. Be enthusiastic and brief.' },
  { id: 'support',     name: 'Support Agent',     desc: 'Professional support',            icon: '🎧', instruction: 'You are a professional customer support agent. Be polite, clear, and solution-focused. Ask clarifying questions when needed.' },
  { id: 'coach',       name: 'Life Coach',        desc: 'Motivational guidance',           icon: '💪', instruction: 'You are an empowering life coach. Ask thoughtful questions, reflect feelings back, and offer actionable guidance. Be warm and concise.' },
  { id: 'meditation',  name: 'Meditation',        desc: 'Guided mindfulness',              icon: '🧘', instruction: 'You are a calm meditation guide. Speak slowly, use soothing language, guide breathing exercises, and help the user find peace.' },
  { id: 'interviewer', name: 'Interviewer',       desc: 'Job interview practice',          icon: '💼', instruction: 'You are a professional interviewer conducting a mock job interview. Ask standard and behavioral questions, give brief constructive feedback after each answer.' },
  { id: 'custom',      name: 'Custom',            desc: 'Write your own instructions',     icon: '✏️', instruction: '' },
]

const selectedPresetId = ref(localStorage.getItem('vv-preset-id') || 'helpful-ai')
const showVoiceMenu = ref(false)
const showPresetMenu = ref(false)
const transcriptEl = ref(null)

const selectedPreset = computed(() => PRESETS.find(p => p.id === selectedPresetId.value) || PRESETS[0])

// Always-editable instruction: single source of truth.
// Pre-filled with preset text; user edits are persisted.
const editableInstruction = ref(
  localStorage.getItem('vv-instruction') ||
  (PRESETS.find(p => p.id === (localStorage.getItem('vv-preset-id') || 'helpful-ai')) || PRESETS[0]).instruction
)
watch(editableInstruction, v => localStorage.setItem('vv-instruction', v))

// When user picks a preset, prefill instruction (unless they've customised it for that preset).
// Effective instructions = always editableInstruction
const instructions = computed(() => editableInstruction.value)

watch(selectedPresetId, (id) => {
  localStorage.setItem('vv-preset-id', id)
  const preset = PRESETS.find(p => p.id === id)
  if (preset && preset.id !== 'custom') editableInstruction.value = preset.instruction
})

function selectPreset(id) { selectedPresetId.value = id; showPresetMenu.value = false }

function onDocClick(e) {
  if (!e.target.closest('.vl__preset')) showPresetMenu.value = false
  if (!e.target.closest('.vl__vpick'))  showVoiceMenu.value = false
}
onMounted(() => document.addEventListener('click', onDocClick, true))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick, true))

// ── Voice config ─────────────────────────────────────────────────────────────
const FALLBACK_VOICES = [
  'Puck','Charon','Kore','Fenrir','Aoede','Leda','Orus','Zephyr',
  'Umbriel','Callirrhoe','Autonoe','Enceladus','Iapetus','Despina',
  'Erinome','Algieba','Rasalhague','Laomedeia','Achernar','Sulafat',
  'Schedar','Gacrux','Pulcherrima','Achird','Zubenelgenubi',
  'Vindemiatrix','Sadachbia','Sadaltager','Sheliak',
]
const voices = ref([])
const defaultModel = ref('')
const voiceName = ref(localStorage.getItem('vv-voice') || '')
const configError = ref('')
const configLoading = ref(true)
watch(voiceName, v => localStorage.setItem('vv-voice', v))

async function loadConfig() {
  configLoading.value = true
  try {
    const res = await fetch(`${core.baseUrl}/voice/live/config`)
    if (!res.ok) throw new Error(`${res.status}`)
    const data = await res.json()
    voices.value = data.voices?.length ? data.voices : FALLBACK_VOICES
    defaultModel.value = data.defaultModel || ''
    if (!voiceName.value || !voices.value.includes(voiceName.value))
      voiceName.value = voices.value[0]
  } catch {
    voices.value = FALLBACK_VOICES
    if (!voiceName.value) voiceName.value = FALLBACK_VOICES[0]
  } finally { configLoading.value = false }
}

// ── Temperature ───────────────────────────────────────────────────────────────
const temperature = ref(Number(localStorage.getItem('vv-temp') || '0.8'))
watch(temperature, v => localStorage.setItem('vv-temp', String(v)))

// ── Session state ─────────────────────────────────────────────────────────────
const connectionState = ref('idle') // idle | connecting | active | error
const isStarting = ref(false)
const isMuted = ref(false)
const speaking = ref(false)
const audioLevel = ref(0)    // 0–1, noise-gated and smoothed
const elapsed = ref(0)
const sessionError = ref('')
const messages = ref([])     // { role, text, isFinal }

// Noise-floor calibration
let noiseFloor = 0.04        // conservative initial value; updated during first 1.5s
let noiseCalSamples = 0
const NOISE_CAL_FRAMES = 30  // ~1.5s at 50ms poll
let prevLevel = 0            // for exponential smoothing

let ws = null, capture = null, playback = null
let speakingTimeout = 0, timerId = 0, audioCtx = null, levelRaf = 0

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
  if (connectionState.value === 'error') return 'Error'
  if (isStarting.value) return 'Connecting…'
  if (connectionState.value === 'active') {
    if (isMuted.value) return 'Muted'
    if (agentState.value === 'speaking') return 'Speaking…'
    if (agentState.value === 'listening') return 'Listening…'
    return 'Live'
  }
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

    audioLevel.value = isMuted.value ? 0 : Math.min(1, prevLevel * 3)
    levelRaf = requestAnimationFrame(tick)
  }
  levelRaf = requestAnimationFrame(tick)
}

function stopLevelMeter() {
  cancelAnimationFrame(levelRaf)
  audioLevel.value = 0; prevLevel = 0
  audioCtx?.close(); audioCtx = null
}

// ── Mute ──────────────────────────────────────────────────────────────────────
function toggleMute() {
  if (!capture) return
  isMuted.value = !isMuted.value
  isMuted.value ? capture.ctx?.suspend?.() : capture.ctx?.resume?.()
}

// ── Transcript — full turn collection, scrollable ─────────────────────────────
// Each entry = one complete speaker turn. Interim chunks update the last entry
// in-place (typewriter effect). On isFinal, the turn is sealed and a new one
// starts on role-switch. Max 200 turns to prevent memory growth.
const TRANSCRIPT_MAX = 200

function appendTranscript(payload) {
  // Skip noise-only transcript events
  if (payload.text === '<noise>' || payload.text.includes('<noise>')) return

  const last = messages.value[messages.value.length - 1]

  // Same role as last entry → update in-place (typewriter effect)
  if (last && last.role === payload.role) {
    last.text = payload.text
    last.isFinal = payload.isFinal
    return
  }

  // Role switch: if last entry was never finalized but has same text, merge
  // This handles WS sending interleaved user/agent interim chunks
  if (payload.isFinal && last && last.text === payload.text && last.role !== payload.role) {
    last.isFinal = true
    return
  }

  // New speaker turn
  if (messages.value.length >= TRANSCRIPT_MAX) messages.value.splice(0, 1)
  messages.value.push({ role: payload.role, text: payload.text, isFinal: payload.isFinal })
}

// Auto-scroll transcript to bottom on new content
watch(() => messages.value.length, async () => {
  await nextTick()
  if (transcriptEl.value) {
    transcriptEl.value.scrollTop = transcriptEl.value.scrollHeight
  }
})
// Also scroll when last message text updates (interim chunks)
watch(() => messages.value[messages.value.length - 1]?.text, async () => {
  await nextTick()
  if (transcriptEl.value) {
    transcriptEl.value.scrollTop = transcriptEl.value.scrollHeight
  }
})

// ── Session lifecycle ─────────────────────────────────────────────────────────
async function startSession() {
  if (hasActiveSession.value || isStarting.value) return
  isStarting.value = true
  sessionError.value = ''
  connectionState.value = 'connecting'
  isMuted.value = false

  ws = connectLiveWs({
    baseUrl: core.baseUrl,
    model: defaultModel.value,
    voice: voiceName.value,
    temperature: temperature.value,
    instructions: instructions.value,
    onSessionState: async (payload) => {
      if (payload.state !== 'active') return
      connectionState.value = 'active'
      isStarting.value = false
      elapsed.value = 0
      timerId = setInterval(() => elapsed.value++, 1000)
      playback = createPlayback()
      try {
        capture = await createCapture((chunk) => {
          if (!isMuted.value) ws?.sendAudio(chunk)
        })
        startLevelMeter(capture.stream)
      } catch (err) {
        sessionError.value = err?.message || String(err)
        stopSession()
      }
    },
    onTranscript: appendTranscript,
    onAudio: (chunk) => {
      playback?.playChunk(chunk)
      speaking.value = true
      clearTimeout(speakingTimeout)
      speakingTimeout = setTimeout(() => (speaking.value = false), 600)
    },
    onInterrupt: () => { playback?.flush(); speaking.value = false },
    onError: (msg) => {
      sessionError.value = msg
      connectionState.value = 'error'
      isStarting.value = false
      teardown()
    },
    onClose: () => {
      if (connectionState.value !== 'error') connectionState.value = 'idle'
      isStarting.value = false
      teardown()
    },
  })
}

function teardown() {
  clearInterval(timerId); stopLevelMeter()
  capture?.stop(); capture = null
  playback?.stop(); playback = null
  speaking.value = false; isMuted.value = false
}

function stopSession() {
  ws?.end(); ws?.close(); ws = null
  teardown()
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
        <!-- Title + status -->
        <div class="vl__sb-card-inner">
          <div class="vl__sb-title-row">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round" class="vl__sb-mic">
              <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/>
              <path d="M19 10v2a7 7 0 0 1-14 0v-2"/>
              <line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/>
            </svg>
            <span class="vl__sb-title">Voice Assistant</span>
          </div>
          <div class="vl__sb-meta-row">
            <span v-if="hasActiveSession" class="vl__timer">{{ timer }}</span>
            <span :class="['vl__badge', `vl__badge--${connectionState}`]">
              <i class="vl__badge-dot" />
              {{ statusText }}
            </span>
            <span v-if="defaultModel" class="vl__model-pill">{{ defaultModel }}</span>
          </div>
        </div>
      </div>

      <!-- Config fields -->
      <div class="vl__config">

        <!-- Voice custom picker -->
        <div class="vl__field">
          <label class="vl__field-label">Voice</label>
          <div class="vl__vpick">
            <button class="vl__vpick-btn" :disabled="hasActiveSession"
              @click.stop="showVoiceMenu = !showVoiceMenu">
              <span class="vl__vpick-dot" :style="`background:oklch(0.6 0.18 ${voiceHue(voiceName)})`" />
              <span class="vl__vpick-name">{{ voiceName || 'Select voice…' }}</span>
              <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="m6 9 6 6 6-6"/>
              </svg>
            </button>
            <div v-if="showVoiceMenu" class="vl__vpick-menu">
              <div v-if="configLoading" class="vl__vpick-loading">Loading voices…</div>
              <button v-for="v in voices" :key="v"
                :class="['vl__vpick-opt', { 'vl__vpick-opt--on': voiceName === v }]"
                @click="voiceName = v; showVoiceMenu = false">
                <span class="vl__vpick-opt-dot" :style="`background:oklch(0.6 0.18 ${voiceHue(v)})`" />
                <span class="vl__vpick-opt-name">{{ v }}</span>
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
          <div class="vl__preset">
            <button class="vl__preset-btn" :disabled="hasActiveSession"
              @click.stop="showPresetMenu = !showPresetMenu">
              <span>{{ selectedPreset.icon }}</span>
              <span class="vl__preset-name">{{ selectedPreset.name }}</span>
              <svg viewBox="0 0 24 24" width="9" height="9" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="m6 9 6 6 6-6"/>
              </svg>
            </button>
            <div v-if="showPresetMenu" class="vl__preset-menu vl__preset-menu--up">
              <div class="vl__preset-section">Agent Presets</div>
              <button v-for="p in PRESETS" :key="p.id"
                :class="['vl__preset-opt', { 'vl__preset-opt--on': selectedPresetId === p.id }]"
                @click="selectPreset(p.id)">
                <span class="vl__preset-opt-icon">{{ p.icon }}</span>
                <div>
                  <div class="vl__preset-opt-name">{{ p.name }}</div>
                  <div class="vl__preset-opt-desc">{{ p.desc }}</div>
                </div>
              </button>
            </div>
          </div>
        </div>
        <!-- Always a textarea — editable when idle, read-only (but visible) when active -->
        <textarea
          v-model="editableInstruction"
          :disabled="hasActiveSession"
          placeholder="System instructions for the voice assistant…"
          class="vl__instr-ta"
        />
        <p class="vl__instr-hint">
          {{ hasActiveSession ? '🔒 Locked while session active.' : 'Applies on next session start.' }}
        </p>
      </div>

    </aside><!-- /sidebar -->

    <!-- ═══════════════════════════════════════════════════════
         RIGHT MAIN — topbar · orb-stage · transcript-feed · controls
         ═══════════════════════════════════════════════════════ -->
    <div class="vl__main">

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

      <!-- ── ORB STAGE — fixed, never grows ── -->
      <div class="vl__orb-stage">

        <!-- Mic bars LEFT — 12 bars, amplitude-driven with baseline -->
        <div class="vl__bars" :class="{ 'vl__bars--active': connectionState === 'active' && !isMuted }">
          <div v-for="i in 12" :key="i" class="vl__bar"
            :class="`vl__bar--${agentState}`"
            :style="connectionState === 'active' && !isMuted
              ? `height:${Math.max(3, 4 + Math.abs(Math.sin((i + Date.now() * 0.001) * 0.7)) * audioLevel * 44)}px`
              : 'height:3px'" />
        </div>

        <!-- Orb — transform-only, zero reflow jitter -->
        <div :class="['vl__orb', `vl__orb--${agentState}`]"
          :style="orbVars"
          role="img"
          :aria-label="`${voiceName} — ${statusText}`">
          <div class="vl__orb-sphere">
            <div class="vl__blob vl__blob--1" />
            <div class="vl__blob vl__blob--2" />
            <div class="vl__blob vl__blob--3" />
          </div>
          <!-- Speaking pulse ring -->
          <span v-if="agentState === 'speaking'" class="vl__orb-pulse" />
          <!-- Muted overlay -->
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
        <div class="vl__bars" :class="{ 'vl__bars--active': connectionState === 'active' && !isMuted }">
          <div v-for="i in 12" :key="i" class="vl__bar"
            :class="`vl__bar--${agentState}`"
            :style="connectionState === 'active' && !isMuted
              ? `height:${Math.max(3, 4 + Math.abs(Math.sin((13 - i + Date.now() * 0.001) * 0.7)) * audioLevel * 44)}px`
              : 'height:3px'" />
        </div>

      </div><!-- /orb-stage -->

      <!-- Voice name + model -->
      <div class="vl__identity">
        <p class="vl__identity-name">{{ voiceName || '—' }}</p>
        <p v-if="defaultModel" class="vl__identity-model">{{ defaultModel }}</p>
      </div>

      <!-- State label -->
      <p :class="['vl__state', `vl__state--${agentState}`,
        { 'vl__state--muted': isMuted, 'vl__state--err': sessionError && !hasActiveSession }]">
        {{ isMuted ? 'Microphone muted'
         : agentState === 'speaking' ? 'Speaking…'
         : agentState === 'listening' ? 'Listening…'
         : agentState === 'thinking' ? 'Connecting…'
         : hasActiveSession ? 'Ready — speak now'
         : 'Start a conversation' }}
      </p>

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
              {{ m.role === 'user' ? 'You' : selectedPreset.name }}
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
          <svg v-if="isStarting" class="vl__spin" viewBox="0 0 24 24" width="16" height="16" fill="none"
            stroke="currentColor" stroke-width="2">
            <path d="M21 12a9 9 0 1 1-6.22-8.56"/>
          </svg>
          <svg v-else viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor"
            stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/>
            <path d="M19 10v2a7 7 0 0 1-14 0v-2"/>
            <line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/>
          </svg>
          {{ isStarting ? 'Connecting…' : 'Start a conversation' }}
        </button>

        <!-- ACTIVE: Mute + End -->
        <template v-else>
          <button :class="['vl__btn-mic', `vl__btn-mic--${agentState}`, { 'vl__btn-mic--muted': isMuted }]"
            :title="isMuted ? 'Unmute microphone' : 'Mute microphone'"
            @click="toggleMute">
            <svg v-if="!isMuted" viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor"
              stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/>
              <path d="M19 10v2a7 7 0 0 1-14 0v-2"/>
              <line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/>
            </svg>
            <svg v-else viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor"
              stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <line x1="1" y1="1" x2="23" y2="23"/>
              <path d="M9 9v3a3 3 0 0 0 5.12 2.12M15 9.34V5a3 3 0 0 0-5.94-.6"/>
              <path d="M17 16.95A7 7 0 0 1 5 12v-2m14 0v2a7 7 0 0 1-.11 1.23"/>
              <line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/>
            </svg>
            <span v-if="!isMuted && agentState === 'listening'" class="vl__btn-mic-ring" />
          </button>
          <button class="vl__btn-end" title="End conversation" @click="stopSession">
            <svg viewBox="0 0 24 24" width="19" height="19" fill="currentColor">
              <path d="M6.6 10.8c1.4 2.8 3.8 5.1 6.6 6.6l2.2-2.2c.28-.28.7-.36 1.06-.2 1.1.37 2.3.57 3.54.57.55 0 1 .45 1 1V20c0 .55-.45 1-1 1C10.18 21 3 13.82 3 5c0-.55.45-1 1-1h3.5c.55 0 1 .45 1 1 0 1.25.2 2.45.57 3.57.11.35.03.74-.2 1.01L6.6 10.8z"/>
            </svg>
          </button>
        </template>
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
  overflow-y: auto;
  overflow-x: visible;     /* allow preset menu to overflow */
}

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
  top: -16px; right: -16px;
  width: 110px; height: 110px;
  color: var(--accent-ai); opacity: 0.10;
  pointer-events: none;
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
.vl__sb-title { font-size: 14px; font-weight: 700; color: var(--ink); }
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
.vl__vpick { position: relative; }
.vl__vpick-btn {
  display: flex; align-items: center; gap: 8px; width: 100%;
  padding: 8px 10px; border: 1px solid var(--border); border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface) 80%, transparent);
  color: var(--ink); font: inherit; font-size: 13px; cursor: pointer;
  transition: border-color 120ms;
}
.vl__vpick-btn:hover:not(:disabled) { border-color: var(--accent-ai); }
.vl__vpick-btn:disabled { opacity: 0.45; cursor: default; }
.vl__vpick-dot {
  width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0;
}
.vl__vpick-name { flex: 1; text-align: left; font-weight: 500; }

/* Voice picker menu — scrollable, below the button, max 220px */
.vl__vpick-menu {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0;
  background: var(--surface); border: 1px solid var(--border);
  border-radius: var(--radius-md); box-shadow: 0 8px 28px oklch(0 0 0 / 0.18);
  z-index: 400; max-height: 220px; overflow-y: auto; padding: 4px 0;
}
.vl__vpick-loading { padding: 10px 12px; font-size: 11px; color: var(--ink-faint); }
.vl__vpick-opt {
  display: flex; align-items: center; gap: 8px;
  width: 100%; padding: 7px 12px;
  background: none; border: none; cursor: pointer; text-align: left;
  color: var(--ink); font-size: 12.5px; font: inherit;
  transition: background 80ms;
}
.vl__vpick-opt:hover { background: var(--surface-hover); }
.vl__vpick-opt--on { background: var(--accent-ai-soft); color: var(--accent-ai); font-weight: 600; }
.vl__vpick-opt-dot {
  width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
}
.vl__vpick-opt-name { flex: 1; }

/* Slider */
.vl__slider-row { display: flex; align-items: center; gap: 6px; }
.vl__slider-hint { font-size: 9px; color: var(--ink-faint); white-space: nowrap; flex-shrink: 0; }
.vl__range { flex: 1; accent-color: var(--accent-ai); cursor: pointer; }
.vl__range:disabled { opacity: 0.45; cursor: default; }

/* Model pill */
.vl__model-pill {
  padding: 2px 8px; border-radius: 100px;
  background: color-mix(in srgb, var(--surface-hover) 80%, transparent);
  font-size: 9.5px; color: var(--ink-faint);
  font-family: 'SF Mono','Fira Code',monospace;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  max-width: 160px; flex-shrink: 1;
}


/* ── Instructions ──────────────────────────────────────── */
.vl__instr {
  padding: 12px 14px;
  display: flex; flex-direction: column; gap: 8px;
  flex: 1; min-height: 0;
}
.vl__instr-head {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
}
.vl__instr-label {
  font-size: 9px; font-weight: 700; letter-spacing: 0.6px;
  color: var(--accent-ai); text-transform: uppercase; flex-shrink: 0;
}

/* Preset selector — contained in sidebar, menu opens upward */
.vl__preset { position: relative; }
.vl__preset-btn {
  display: flex; align-items: center; gap: 4px;
  padding: 4px 8px; border: 1px solid var(--border); border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--surface) 80%, transparent);
  color: var(--ink-muted); font: inherit; font-size: 11px; cursor: pointer;
  white-space: nowrap; max-width: 140px;
  transition: border-color 120ms, color 120ms;
}
.vl__preset-btn:hover:not(:disabled) { border-color: var(--accent-ai); color: var(--ink); }
.vl__preset-btn:disabled { opacity: 0.4; cursor: default; }
.vl__preset-name { flex: 1; overflow: hidden; text-overflow: ellipsis; min-width: 0; }

/* Menu opens UPWARD — bottom anchored to the trigger */
.vl__preset-menu {
  position: absolute; right: 0;
  width: 240px; background: var(--surface); border: 1px solid var(--border);
  border-radius: var(--radius-md); box-shadow: 0 -8px 28px oklch(0 0 0 / 0.16);
  z-index: 500; padding: 4px 0; max-height: 280px; overflow-y: auto;
}
.vl__preset-menu--up { bottom: calc(100% + 4px); top: auto; }

.vl__preset-section {
  padding: 6px 10px 2px; font-size: 9px; font-weight: 700;
  color: var(--ink-faint); text-transform: uppercase; letter-spacing: 0.5px;
}
.vl__preset-opt {
  display: flex; align-items: flex-start; gap: 8px;
  width: 100%; padding: 7px 10px;
  background: none; border: none; cursor: pointer; text-align: left; color: var(--ink);
  transition: background 100ms;
}
.vl__preset-opt:hover { background: var(--surface-hover); }
.vl__preset-opt--on { background: var(--accent-ai-soft); color: var(--accent-ai); }
.vl__preset-opt-icon { font-size: 14px; flex-shrink: 0; margin-top: 1px; }
.vl__preset-opt-name { font-size: 11.5px; font-weight: 600; display: block; }
.vl__preset-opt-desc {
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
  /* flex:1 lets it grow to fill sidebar — min keeps it usable */
  flex: 1; min-height: 120px;
  box-sizing: border-box; transition: border-color 120ms;
}
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
.vl__timer {
  font-size: 11px; font-variant-numeric: tabular-nums;
  color: var(--ink-muted); font-family: 'SF Mono','Fira Code',monospace;
}

/* Status badge */
.vl__badge {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 3px 9px; border-radius: 100px;
  font-size: 10px; font-weight: 700;
  background: var(--surface-hover); color: var(--ink-faint);
}
.vl__badge-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--ink-faint); }
.vl__badge--active { color: #22c55e; }
.vl__badge--active .vl__badge-dot { background: #22c55e; animation: dot-blink 2s ease infinite; }
.vl__badge--connecting { color: #f59e0b; }
.vl__badge--connecting .vl__badge-dot { background: #f59e0b; animation: dot-blink 0.7s ease infinite; }
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
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 20px;
  padding: 28px 20px 16px;
  overflow: visible;
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
.vl__bar--listening { background: #3b82f6; }
.vl__bar--speaking  { background: #10b981; }
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

.vl__orb-sphere {
  position: absolute; inset: 0; border-radius: 50%; overflow: hidden;
  transition: background 0.5s;
  background: color-mix(in srgb, var(--accent-ai) 5%, transparent);
}
.vl__orb--listening .vl__orb-sphere { background: rgba(59,130,246,0.07); }
.vl__orb--speaking  .vl__orb-sphere { background: rgba(16,185,129,0.09); }
.vl__orb--thinking  .vl__orb-sphere { background: rgba(245,158,11,0.06); }

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
.vl__orb--listening .vl__blob--1 { background:radial-gradient(circle at 30% 30%,#3b82f6,transparent 68%); animation-duration:3.5s; }
.vl__orb--listening .vl__blob--2 { background:radial-gradient(circle at 70% 40%,#60a5fa,transparent 68%); animation-duration:2.8s; }
.vl__orb--listening .vl__blob--3 { background:radial-gradient(circle at 50% 70%,#93c5fd,transparent 68%); animation-duration:2.2s; }
.vl__orb--speaking .vl__blob--1 { background:radial-gradient(circle at 30% 30%,#10b981,transparent 68%); opacity:.55; animation-duration:3s; }
.vl__orb--speaking .vl__blob--2 { background:radial-gradient(circle at 70% 40%,#34d399,transparent 68%); opacity:.5; animation-duration:2.4s; }
.vl__orb--speaking .vl__blob--3 { background:radial-gradient(circle at 50% 70%,#6ee7b7,transparent 68%); opacity:.45; animation-duration:1.9s; }

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

/* Speaking pulse ring */
.vl__orb-pulse {
  position:absolute; inset:-10px; border-radius:50%;
  border:2px solid #10b981; opacity:0;
  animation: orb-ring 1.6s ease-out infinite;
}
@keyframes orb-ring { 0%{opacity:.5;transform:scale(.94)} 100%{opacity:0;transform:scale(1.2)} }

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
  /* thin scrollbar that doesn't shift layout */
  scrollbar-width: none;
}

.vl__transcript-empty {
  display: flex; flex-direction: column; align-items: center; gap: 8px;
  padding: 32px 20px; font-size: 11px; color: var(--ink-faint);
  justify-content: center; height: 100%;
}

/* One entry per speaker turn; typewriter update in-place */
.vl__turn { display: flex; align-items: flex-start; gap: 8px; animation: turn-in 0.12s ease; }
.vl__turn--user { flex-direction: row-reverse; }
.vl__turn--interim .vl__turn-text { opacity: 0.6; }
/* Consecutive same-speaker turns sit closer together */
.vl__turn:not(:first-child) { margin-top: 2px; }
/* When speaker switches, more breathing room */
.vl__turn-who + .vl__turn-text { margin-top: 0; }

.vl__turn-av {
  width: 22px; height: 22px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center; margin-top: 2px;
}
.vl__turn-av--user  { background: var(--accent-soft); color: var(--accent); }
.vl__turn-av--agent { background: var(--accent-ai-soft); color: var(--accent-ai); }

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
  background: color-mix(in srgb, var(--accent) 8%, transparent);
  border-radius: var(--radius-md) var(--radius-sm) var(--radius-md) var(--radius-md);
}
.vl__turn--agent .vl__turn-text {
  background: color-mix(in srgb, var(--accent-ai) 8%, transparent);
  border-radius: var(--radius-sm) var(--radius-md) var(--radius-md) var(--radius-md);
}
@supports not (color: color-mix(in srgb, red 50%, blue)) {
  .vl__turn--user .vl__turn-text  { background: var(--accent-soft); }
  .vl__turn--agent .vl__turn-text { background: var(--accent-ai-soft); }
}
/* Blinking cursor on interim streaming turns */
.vl__turn-cursor {
  display: inline-block; width: 2px; height: 13px;
  background: currentColor; margin-left: 2px; vertical-align: text-bottom;
  animation: blink-cursor 0.8s ease-in-out infinite;
}
@keyframes blink-cursor { 0%,100%{opacity:1} 50%{opacity:0} }
@keyframes turn-in { from{opacity:0;transform:translateY(4px)} to{opacity:1;transform:translateY(0)} }

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

/* Spinner */
.vl__spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
