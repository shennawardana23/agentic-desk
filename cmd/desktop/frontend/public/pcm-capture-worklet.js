// AudioWorklet processor for the realtime Voice Assistant mic capture
// pipeline. Runs on the audio rendering thread (a separate global scope
// from the page's JS, hence living as a plain static file rather than a
// bundled module — audioContext.audioWorklet.addModule() needs a real
// URL, not a Vite-processed import).
//
// Gemini Live wants raw 16-bit PCM, so this buffers incoming Float32
// render-quantum blocks (128 samples each) until it has a reasonably
// sized chunk (~2048 samples ≈ 128ms at 16kHz), converts to Int16, and
// posts the ArrayBuffer to the main thread — batching avoids one
// postMessage per 128-sample block, which would be excessive overhead.
class PCMCaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super()
    this.buffer = []
    this.bufferedSamples = 0
    this.chunkSize = 512  // ponytail: 32ms at 16kHz instead of 128ms — faster first-word response
  }

  process(inputs) {
    const input = inputs[0]
    if (!input || !input[0]) return true
    const channel = input[0]

    this.buffer.push(channel.slice())
    this.bufferedSamples += channel.length

    if (this.bufferedSamples >= this.chunkSize) {
      const merged = new Float32Array(this.bufferedSamples)
      let offset = 0
      for (const block of this.buffer) {
        merged.set(block, offset)
        offset += block.length
      }
      const pcm16 = new Int16Array(merged.length)
      for (let i = 0; i < merged.length; i++) {
        const s = Math.max(-1, Math.min(1, merged[i]))
        pcm16[i] = s < 0 ? s * 0x8000 : s * 0x7fff
      }
      this.port.postMessage(pcm16.buffer, [pcm16.buffer])
      this.buffer = []
      this.bufferedSamples = 0
    }

    return true
  }
}

registerProcessor('pcm-capture-processor', PCMCaptureProcessor)
