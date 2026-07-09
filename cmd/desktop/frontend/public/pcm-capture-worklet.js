// AudioWorklet processor for the realtime Voice Assistant mic capture pipeline.
// Runs on the audio rendering thread (separate global scope — needs a real URL).
//
// Key design decisions:
// - chunkSize=256 samples = 16ms at 16kHz (fast enough for VAD, low queue time)
// - PCM16 conversion in worklet (off main thread)
// - Calibration counter in worklet (avoids RAF/worklet clock mismatch)
class PCMCaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super()
    this.buffer = []
    this.bufferedSamples = 0
    this.chunkSize = 320  // 20ms at 16kHz — optimal Gemini Live chunk size.
                           // Gemini's ASR is tuned for 20ms frames. 16ms was
                           // too small — excess postMessage overhead, more
                           // queue jitter on the main thread.
    this.calChunks = 0    // calibration counter (chunk-accurate, not RAF-based)
    this.calTarget = 5    // 5 chunks × 20ms = 100ms — enough to measure noise floor
    this.noiseSum = 0
    this.noiseFloor = 0.02
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

      // Compute RMS in worklet (off main thread, no postMessage overhead)
      let sumSq = 0
      for (const s of merged) sumSq += s * s
      const rms = Math.sqrt(sumSq / merged.length)

      // Calibration: measure noise floor for first calTarget chunks
      if (this.calChunks < this.calTarget) {
        this.noiseSum += rms
        this.calChunks++
        this.noiseFloor = this.noiseSum / this.calChunks
      }

      // Convert Float32 → Int16 PCM
      const pcm16 = new Int16Array(merged.length)
      for (let i = 0; i < merged.length; i++) {
        const s = Math.max(-1, Math.min(1, merged[i]))
        pcm16[i] = s < 0 ? s * 0x8000 : s * 0x7fff
      }

      // Post chunk + metadata to main thread
      this.port.postMessage({
        buffer: pcm16.buffer,
        rms,
        noiseFloor: this.noiseFloor,
        calibrated: this.calChunks >= this.calTarget,
      }, [pcm16.buffer])

      this.buffer = []
      this.bufferedSamples = 0
    }

    return true
  }
}

registerProcessor('pcm-capture-processor', PCMCaptureProcessor)
