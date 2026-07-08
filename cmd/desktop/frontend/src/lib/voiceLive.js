// voiceLive.js — realtime Gemini Live voice pipeline: mic capture (raw
// PCM16 via AudioWorklet, not MediaRecorder — MediaRecorder only
// produces compressed containers, wrong primitive for a continuous PCM
// stream), gapless playback, and the WS client. See
// docs/superpowers/specs/2026-07-08-voice-live-realtime-design.md and
// internal/voicelive/voicelive.go for the backend half of this protocol.

/**
 * createCapture opens the mic at 16kHz mono and calls onChunk(ArrayBuffer)
 * with each ~128ms batch of raw Int16 PCM samples, ready to send straight
 * over the WS as a binary frame.
 *
 * @param {(chunk: ArrayBuffer) => void} onChunk
 * @returns {Promise<{ stop: () => void }>}
 */
export async function createCapture(onChunk) {
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: { sampleRate: 16000, channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
  })
  const ctx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 16000 })
  await ctx.audioWorklet.addModule('/pcm-capture-worklet.js')

  const source = ctx.createMediaStreamSource(stream)
  const node = new AudioWorkletNode(ctx, 'pcm-capture-processor')
  node.port.onmessage = (e) => onChunk(e.data)

  // AudioWorkletNodes only run while part of a connected graph reaching
  // the destination — route through a silent gain node so capture keeps
  // running without the mic being audible in the output.
  const mute = ctx.createGain()
  mute.gain.value = 0
  source.connect(node)
  node.connect(mute)
  mute.connect(ctx.destination)

  return {
    stop() {
      stream.getTracks().forEach((t) => t.stop())
      ctx.close()
    },
  }
}

/**
 * createPlayback returns a gapless PCM16-at-24kHz player: each call to
 * playChunk schedules its buffer immediately after whatever's already
 * queued (a running `nextPlayTime` cursor), so consecutive chunks sound
 * continuous instead of clicking between them.
 */
export function createPlayback() {
  const ctx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 24000 })
  let nextPlayTime = 0

  return {
    ctx,
    /** @param {ArrayBuffer} chunk raw Int16 PCM at 24kHz mono */
    playChunk(chunk) {
      const int16 = new Int16Array(chunk)
      const float32 = new Float32Array(int16.length)
      for (let i = 0; i < int16.length; i++) {
        const v = int16[i]
        float32[i] = v / (v < 0 ? 0x8000 : 0x7fff)
      }
      const buffer = ctx.createBuffer(1, float32.length, 24000)
      buffer.copyToChannel(float32, 0)
      const source = ctx.createBufferSource()
      source.buffer = buffer
      source.connect(ctx.destination)
      const startAt = Math.max(ctx.currentTime, nextPlayTime)
      source.start(startAt)
      nextPlayTime = startAt + buffer.duration
    },
    /**
     * Barge-in: Web Audio has no API to cancel buffer sources already
     * scheduled in the future, so the only way to hard-stop queued
     * playback is dropping the cursor back to "now" — anything already
     * scheduled still plays out its current buffer, but nothing new
     * queues behind it.
     */
    flush() {
      nextPlayTime = ctx.currentTime
    },
    stop() {
      ctx.close()
    },
  }
}

/**
 * connectLiveWs opens the realtime voice WS, sends the one-time `start`
 * config frame, and dispatches every server message to the matching
 * callback. No "send" message exists in this protocol — once started,
 * audio flows continuously in both directions until `end()`/`close()`.
 */
export function connectLiveWs({ baseUrl, model, voice, temperature, instructions, onOpen, onTranscript, onAudio, onInterrupt, onSessionState, onError, onClose }) {
  const wsUrl = `${baseUrl.replace(/^http/, 'ws')}/voice/live/ws`
  const ws = new WebSocket(wsUrl)
  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    ws.send(JSON.stringify({ type: 'start', payload: { model, voice, temperature, instructions } }))
    onOpen?.()
  }
  ws.onmessage = (e) => {
    if (e.data instanceof ArrayBuffer) {
      onAudio?.(e.data)
      return
    }
    let msg
    try {
      msg = JSON.parse(e.data)
    } catch {
      return
    }
    if (msg.type === 'session_state') onSessionState?.(msg.payload)
    else if (msg.type === 'transcript') onTranscript?.(msg.payload)
    else if (msg.type === 'interrupt') onInterrupt?.()
    else if (msg.type === 'error') onError?.(msg.payload?.message || 'voice session error')
  }
  ws.onerror = () => onError?.('connection error')
  ws.onclose = () => onClose?.()

  return {
    /** @param {ArrayBuffer} chunk */
    sendAudio(chunk) {
      if (ws.readyState === WebSocket.OPEN) ws.send(chunk)
    },
    end() {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'end' }))
    },
    close() {
      ws.close()
    },
  }
}
