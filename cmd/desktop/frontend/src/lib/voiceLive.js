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
  // getUserMedia can resolve with null or an empty-track stream in some
  // WebView environments (Wails on macOS, sandboxed Electron builds) even
  // when permission appears granted. Validate before touching AudioContext
  // so the error is "no mic track" rather than a cryptic DOM exception.
  let stream
  try {
    stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        sampleRate: 16000,
        channelCount: 1,
        // echoCancellation MUST be true on laptops — without it the mic
        // picks up the agent's speaker output, Gemini transcribes the agent's
        // own voice as user input, and the agent hallucinates/responds to itself.
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
      },
    })
  } catch (err) {
    // NotAllowedError = permission denied, NotFoundError = no mic hardware
    const msg = err?.name === 'NotAllowedError'
      ? 'Microphone permission denied — allow access in System Settings → Privacy → Microphone.'
      : err?.name === 'NotFoundError'
        ? 'No microphone found. Plug one in and try again.'
        : `Microphone unavailable: ${err?.message || err}`
    throw new Error(msg)
  }

  if (!stream || stream.getAudioTracks().length === 0) {
    stream?.getTracks().forEach((t) => t.stop())
    throw new Error('Microphone unavailable: browser returned an empty audio stream. Check System Settings → Privacy → Microphone.')
  }

  const ctx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 16000 })
  // Resume in case the AudioContext starts suspended (autoplay policy)
  if (ctx.state === 'suspended') await ctx.resume()

  await ctx.audioWorklet.addModule('/pcm-capture-worklet.js')

  const source = ctx.createMediaStreamSource(stream)
  const node = new AudioWorkletNode(ctx, 'pcm-capture-processor')
  // Worklet posts {buffer, rms, noiseFloor, calibrated} — pass the whole
  // message object so VoiceView can use worklet-computed VAD metadata.
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
    stream,
    ctx,   // exposed so callers can suspend/resume without destroying the stream
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
  let ctx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 24000 })
  let gain = null
  let nextPlayTime = 0
  let needsReinit = false  // false = already initialized below

  function init() {
    ctx = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: 24000 })
    if (ctx.state === 'suspended') ctx.resume()
    gain = ctx.createGain()
    gain.gain.value = 1
    gain.connect(ctx.destination)
    nextPlayTime = 0
    needsReinit = false
  }

  // Pre-warm on construction — AudioContext() takes 5-15ms to allocate
  // the hardware audio thread. Doing it now means the first chunk of every
  // response plays immediately instead of paying that cost mid-stream.
  init()

  return {
    get ctx() { return ctx },
    /** @param {ArrayBuffer} chunk raw Int16 PCM at 24kHz mono */
    playChunk(chunk) {
      if (needsReinit) init()

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
      source.connect(gain)
      const now = ctx.currentTime
      if (nextPlayTime < now) nextPlayTime = now
      source.start(nextPlayTime)
      nextPlayTime += buffer.duration
    },
    /**
     * Barge-in: closes the entire AudioContext — this is the ONLY reliable
     * way to kill already-scheduled BufferSource nodes in Web Audio API.
     * The next playChunk() call lazily recreates the context. Matches the
     * production reference pattern (archpublicwebsite-mcp).
     */
    flush() {
      if (gain) gain.gain.setValueAtTime(0, ctx.currentTime)
      ctx.close()
      // Re-init immediately — don't wait for next playChunk. This pre-warms
      // the new AudioContext so the first chunk of the new response plays
      // without paying the 5-15ms AudioContext construction cost mid-stream.
      init()
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
    /** @param {string} base64Jpeg base64-encoded JPEG frame */
    sendVideoFrame(base64Jpeg) {
      if (ws.readyState === WebSocket.OPEN)
        ws.send(JSON.stringify({ type: 'video_frame', payload: { data: base64Jpeg, mime_type: 'image/jpeg' } }))
    },
    end() {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'end' }))
    },
    close() {
      ws.close()
    },
  }
}
