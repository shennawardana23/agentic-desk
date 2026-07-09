/**
 * useVoiceLiveStore — Pinia store for the Voice Live agent session.
 * Adopted from archpublicwebsite-mcp/ui/src/stores/agentlive.js.
 * Manages: session lifecycle, presets, models, transcript, tool pipeline.
 */
import { defineStore } from 'pinia'
import { useCoreStore } from './core'

// api() returns a fully-qualified URL using core.baseUrl so the store works
// in both desktop (Wails, dynamic port) and web (Vite proxy, empty baseUrl).
function api(path) {
  try {
    const core = useCoreStore()
    return `${core.baseUrl || ''}${path}`
  } catch {
    return path
  }
}

export const GEMINI_VOICES = [
  'Puck', 'Charon', 'Kore', 'Fenrir', 'Leda', 'Orus',
  'Aoede', 'Callirrhoe', 'Autonoe', 'Enceladus', 'Iapetus',
  'Umbriel', 'Zephyr', 'Achernar', 'Achird', 'Algenib',
]

export const useVoiceLiveStore = defineStore('voicelive', {
  state: () => ({
    // Models
    models: {},
    liveModels: [],
    selectedModelId: localStorage.getItem('vl-model') || 'gemini-2.5-flash-native-audio-preview-12-2025',

    // Agent Presets
    presets: [],
    selectedPresetId: localStorage.getItem('vl-preset') || 'helpful-ai',

    // Config (persisted)
    voiceName:   localStorage.getItem('vl-voice') || 'Puck',
    instructions: localStorage.getItem('vl-instructions') || '',
    temperature:  Number(localStorage.getItem('vl-temp') || '0.8'),

    // Session
    activeSession:  null,
    sessionState:   'idle',  // idle | connecting | active | paused | ended | error
    isRecording:    false,
    isMuted:        false,
    audioLevel:     0,

    // Transcript: [{role, text, timestamp, isFinal}]
    transcript: [],

    // Tool pipeline nodes for the visual trace
    pipelineNodes: [],

    // WS
    wsConnected: false,
    lastError: null,
  }),

  getters: {
    isActive:    s => s.sessionState === 'active',
    isConnecting: s => s.sessionState === 'connecting',
    hasActiveSession: s => s.activeSession !== null && s.sessionState !== 'ended',
    selectedPreset: s => s.presets.find(p => p.id === s.selectedPresetId) || null,
  },

  actions: {
    // ── Models ──────────────────────────────────────────────────────────────
    async fetchModels() {
      try {
        const r = await fetch(api('/api/agent-live/models'))
        if (r.ok) {
          const d = await r.json()
          this.models = d.models || {}
          this.liveModels = d.live_models || []
        }
      } catch (e) { console.error('[VoiceLive] fetchModels:', e) }
    },

    // ── Presets ─────────────────────────────────────────────────────────────
    async fetchPresets() {
      try {
        const r = await fetch(api('/api/agent-live/presets'))
        if (r.ok) {
          const d = await r.json()
          this.presets = d.presets || []
          if (this.presets.length > 0 && !this.instructions) {
            this.applyPreset(this.presets[0].id)
          }
        }
      } catch (e) { console.error('[VoiceLive] fetchPresets:', e) }
    },

    applyPreset(presetId) {
      const p = this.presets.find(x => x.id === presetId)
      if (!p) return
      this.selectedPresetId = presetId
      localStorage.setItem('vl-preset', presetId)
      this.instructions = p.instruction || ''
      localStorage.setItem('vl-instructions', this.instructions)
      if (p.voice_name) { this.voiceName = p.voice_name; localStorage.setItem('vl-voice', p.voice_name) }
      if (p.temperature > 0) { this.temperature = p.temperature; localStorage.setItem('vl-temp', String(p.temperature)) }
      if (p.model_id) { this.selectedModelId = p.model_id; localStorage.setItem('vl-model', p.model_id) }
    },

    async createPreset(data) {
      try {
        const r = await fetch(api('/api/agent-live/presets'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(data),
        })
        if (r.ok) {
          const p = await r.json()
          this.presets.push(p)
          return p
        }
      } catch (e) { console.error('[VoiceLive] createPreset:', e) }
      return null
    },

    // ── Session ─────────────────────────────────────────────────────────────
    async createSession() {
      try {
        this.sessionState = 'connecting'
        const r = await fetch(api('/api/agent-live/sessions'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            model_id:    this.selectedModelId,
            voice_name:  this.voiceName,
            system_text: this.instructions,
            temperature: this.temperature,
          }),
        })
        if (!r.ok) {
          const e = await r.json()
          throw new Error(e.error || 'Failed to create session')
        }
        const session = await r.json()
        this.activeSession = session
        this.transcript = []
        this.pipelineNodes = []
        return session
      } catch (e) {
        this.sessionState = 'error'
        this.lastError = e.message
        return null
      }
    },

    async endSession() {
      if (!this.activeSession) return
      try {
        await fetch(api(`/api/agent-live/sessions/${this.activeSession.id}/end`), { method: 'POST' })
      } catch {}
      this.sessionState = 'ended'
      this.isRecording = false
      this.wsConnected = false
    },

    // ── WS message handling ─────────────────────────────────────────────────
    handleWSMessage(msg) {
      switch (msg.type) {
        case 'transcript':     this.handleTranscript(msg.payload);      break
        case 'session_state':  this.handleSessionState(msg.payload);    break
        case 'tool_call':      this.handleToolCall(msg.payload);        break
        case 'tool_result':    this.handleToolResult(msg.payload);      break
        case 'interrupt':      this.handleInterrupt();                   break
        case 'error':
          this.lastError = msg.payload?.message || 'Unknown error'
          this.sessionState = 'error'
          break
      }
    },

    handleInterrupt() {
      // Remove non-final agent entries (interrupted streaming partials)
      this.transcript = this.transcript.filter(t => t.isFinal || t.role === 'user')
    },

    handleTranscript(payload) {
      // Empty text + isFinal = TurnComplete seal — just mark last same-role entry final.
      // Never push a blank entry; that creates empty bubbles in the transcript.
      if (payload.is_final && (!payload.text || payload.text === '')) {
        const last = [...this.transcript].reverse().find(t => t.role === payload.role)
        if (last) last.isFinal = true
        return
      }
      if (!payload.text) return  // skip any other empty payload

      const entry = {
        role:      payload.role,
        text:      payload.text,
        timestamp: payload.timestamp,
        isFinal:   payload.is_final,
      }
      // Interim: update last same-role non-final entry in-place (typewriter)
      if (!entry.isFinal) {
        const idx = [...this.transcript].reverse().findIndex(t => t.role === entry.role && !t.isFinal)
        if (idx >= 0) {
          this.transcript[this.transcript.length - 1 - idx] = entry
          return
        }
      }
      this.transcript.push(entry)
    },

    handleSessionState(payload) {
      this.sessionState = payload.state
      if (this.activeSession) {
        this.activeSession.total_tokens = payload.total_tokens || 0
        this.activeSession.duration_ms  = payload.duration_ms  || 0
      }
    },

    handleToolCall(payload) {
      const taskType   = payload.args?.type || payload.name
      const displayName = this._resolveToolName(taskType, payload.name)
      if (this.pipelineNodes.find(n => n.data?.displayName === displayName && n.data?.status === 'running')) return
      this.pipelineNodes.push({
        id:   `tool-${displayName}-${Date.now()}`,
        type: 'trace-tool-node',
        data: { label: payload.name, displayName, taskType, status: 'running', args: payload.args },
      })
    },

    handleToolResult(payload) {
      const output = payload.output || {}
      const idx = this.pipelineNodes.findLastIndex(
        n => (n.data?.label === payload.name || n.data?.displayName === payload.name) && n.data?.status === 'running'
      )
      if (idx >= 0) {
        this.pipelineNodes[idx] = {
          ...this.pipelineNodes[idx],
          data: { ...this.pipelineNodes[idx].data, status: output.error ? 'error' : 'success', output },
        }
      } else {
        this.pipelineNodes.push({
          id: `tool-${payload.name}-${Date.now()}`,
          type: 'trace-tool-node',
          data: { label: payload.name, status: output.error ? 'error' : 'success', output },
        })
      }
    },

    _resolveToolName(taskType, fallback) {
      const map = { image: 'generate_image', code: 'generate_code', music: 'generate_music', video: 'generate_video', fetch: 'fetch_url' }
      return map[taskType] || fallback
    },

    // ── Audio state ─────────────────────────────────────────────────────────
    setRecording(v) { this.isRecording = v },
    setMuted(v)     { this.isMuted = v },
    setAudioLevel(v){ this.audioLevel = v },
    setWsConnected(v){ this.wsConnected = v },

    reset() {
      this.activeSession  = null
      this.sessionState   = 'idle'
      this.isRecording    = false
      this.isMuted        = false
      this.audioLevel     = 0
      this.transcript     = []
      this.pipelineNodes  = []
      this.wsConnected    = false
      this.lastError      = null
    },
  },
})
