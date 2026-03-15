/**
 * Energy-based Voice Activity Detection (VAD)
 *
 * Zero-dependency VAD using Web Audio API AnalyserNode.
 * Detects speech start/end via RMS energy thresholds.
 * Uses a single continuous MediaRecorder so all audio chunks share
 * the same WebM init segment, preventing decodeAudioData failures.
 */

export interface VADOptions {
  /** RMS threshold to consider as speech (0-1). Default: 0.015 */
  speechThreshold?: number
  /** RMS threshold to consider as silence (0-1). Default: 0.01 */
  silenceThreshold?: number
  /** Initial ambient noise calibration duration (ms). Default: 400 */
  noiseCalibrationDuration?: number
  /** Duration of silence before triggering speech end (ms). Default: 1500 */
  silenceDuration?: number
  /** Minimum speech duration to be considered valid (ms). Default: 300 */
  minSpeechDuration?: number
  /** Pre-buffer duration in ms to keep before speech detection. Default: 300 */
  preBufferDuration?: number
  /** Called when speech starts */
  onSpeechStart?: () => void
  /** Called when speech ends, with the recorded audio segment */
  onSpeechEnd?: (audio: Blob) => void
  /** Called with current RMS level (0-1) for visualization. Throttled to ~10fps. */
  onVolumeChange?: (level: number) => void
}

type VADState = 'idle' | 'speaking' | 'silence_pending'

const TICK_INTERVAL = 50 // ms
const MIN_SPEECH_FRAMES = 3 // ~150ms at 50ms tick interval
const MIN_RESUME_SPEECH_FRAMES = 2 // ~100ms at 50ms tick interval
const CHUNK_TIMESLICE = 100 // ms per chunk
const NOISE_FLOOR_RISE_ALPHA = 0.22
const NOISE_FLOOR_FALL_ALPHA = 0.08
const SPEECH_NOISE_RATIO = 1.35
const SPEECH_NOISE_OFFSET = 0.003
const SILENCE_NOISE_RATIO = 1.12
const SILENCE_NOISE_OFFSET = 0.0015
const THRESHOLD_HYSTERESIS = 0.002
const RESUME_THRESHOLD_MIN_OFFSET = 0.0025
const RESUME_THRESHOLD_RATIO = 0.12
const DEFAULT_NOISE_CALIBRATION_DURATION = 400

export class EnergyVAD {
  private opts: Required<Omit<VADOptions, 'onSpeechStart' | 'onSpeechEnd' | 'onVolumeChange'>> &
    Pick<VADOptions, 'onSpeechStart' | 'onSpeechEnd' | 'onVolumeChange'>
  private state: VADState = 'idle'
  private stream: MediaStream | null = null
  private audioCtx: AudioContext | null = null
  private sourceNode: MediaStreamAudioSourceNode | null = null
  private analyser: AnalyserNode | null = null
  private dataArray: Uint8Array | null = null
  private tickTimer: number | null = null
  private speechStartTime = 0
  private silenceStartTime = 0
  private speechFrameCount = 0
  private resumeSpeechFrameCount = 0
  private paused = false
  private volumeTickCount = 0 // throttle volume callbacks
  private noiseFloor = 0
  private listeningStartTime = 0

  // Single recorder: all chunks share the same WebM init segment
  private recorder: MediaRecorder | null = null
  private chunks: Blob[] = []
  private speechStartIdx = -1 // index into chunks where speech region begins
  private preBufferMaxChunks = 3 // keep ~300ms at 100ms timeslice

  private _isListening = false
  private _isSpeaking = false

  get isListening() {
    return this._isListening
  }
  get isSpeaking() {
    return this._isSpeaking
  }

  constructor(options?: VADOptions) {
    const preBufferDuration = options?.preBufferDuration ?? 300
    this.opts = {
      speechThreshold: options?.speechThreshold ?? 0.015,
      silenceThreshold: options?.silenceThreshold ?? 0.01,
      noiseCalibrationDuration:
        options?.noiseCalibrationDuration ?? DEFAULT_NOISE_CALIBRATION_DURATION,
      silenceDuration: options?.silenceDuration ?? 1500,
      minSpeechDuration: options?.minSpeechDuration ?? 300,
      preBufferDuration,
      onSpeechStart: options?.onSpeechStart,
      onSpeechEnd: options?.onSpeechEnd,
      onVolumeChange: options?.onVolumeChange,
    }
    this.preBufferMaxChunks = Math.max(1, Math.ceil(preBufferDuration / CHUNK_TIMESLICE))
  }

  async start(): Promise<void> {
    if (this._isListening) return

    // Request mic with echo cancellation + noise suppression for cleaner VAD
    this.stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
      },
    })

    this.audioCtx = new AudioContext()
    this.sourceNode = this.audioCtx.createMediaStreamSource(this.stream)

    this.analyser = this.audioCtx.createAnalyser()
    this.analyser.fftSize = 2048
    this.analyser.smoothingTimeConstant = 0.3
    this.sourceNode.connect(this.analyser)

    this.dataArray = new Uint8Array(this.analyser.fftSize)
    this.state = 'idle'
    this.speechFrameCount = 0
    this.resumeSpeechFrameCount = 0
    this.volumeTickCount = 0
    this.paused = false
    this.noiseFloor = Math.max(0.001, this.opts.silenceThreshold * 0.8)
    this.listeningStartTime = Date.now()
    this._isListening = true
    this._isSpeaking = false

    // Start continuous recorder
    this.startRecorder()

    // Tick at ~20fps (50ms), volume callback throttled to ~10fps
    this.tickTimer = window.setInterval(() => this.tick(), TICK_INTERVAL)
  }

  stop(): void {
    this.cleanup(true)
  }

  /** Pause VAD detection without releasing mic (for TTS playback). */
  pause(): void {
    this.paused = true
    // If currently recording speech, discard it
    if (this.speechStartIdx >= 0) {
      this.speechStartIdx = -1
    }
    // Stop recorder while paused
    this.stopRecorder()
    this.state = 'idle'
    this._isSpeaking = false
    this.speechFrameCount = 0
    this.resumeSpeechFrameCount = 0
  }

  /** Resume VAD detection after pause. Mic stream is still alive. */
  resume(): void {
    if (!this._isListening) return
    this.paused = false
    this.state = 'idle'
    this.speechFrameCount = 0
    this.resumeSpeechFrameCount = 0
    // Restart recorder
    this.startRecorder()
  }

  destroy(): void {
    this.cleanup(true)
  }

  private cleanup(releaseMic: boolean): void {
    if (this.tickTimer !== null) {
      clearInterval(this.tickTimer)
      this.tickTimer = null
    }
    this.stopRecorder()
    if (this.sourceNode) {
      this.sourceNode.disconnect()
      this.sourceNode = null
    }
    if (releaseMic) {
      if (this.audioCtx) {
        this.audioCtx.close().catch(() => {})
        this.audioCtx = null
      }
      if (this.stream) {
        this.stream.getTracks().forEach((t) => t.stop())
        this.stream = null
      }
    }
    this.analyser = null
    this.dataArray = null
    this.state = 'idle'
    this._isListening = false
    this._isSpeaking = false
    this.noiseFloor = 0
    this.resumeSpeechFrameCount = 0
    this.listeningStartTime = 0
  }

  private tick(): void {
    if (!this.analyser || !this.dataArray) return

    // Always compute RMS for volume visualization, even when paused
    this.analyser.getByteTimeDomainData(this.dataArray as unknown as Uint8Array<ArrayBuffer>)
    const rms = this.computeRMS(this.dataArray)

    // Throttle volume callback to every other tick (~10fps)
    this.volumeTickCount++
    if (this.volumeTickCount % 2 === 0) {
      this.opts.onVolumeChange?.(rms)
    }

    if (this.paused) return

    const now = Date.now()
    const isCalibrating = now - this.listeningStartTime < this.opts.noiseCalibrationDuration
    const initialThresholds = this.computeAdaptiveThresholds()

    // Keep tracking ambient floor while not actively in speaking state.
    if (this.state !== 'speaking') {
      // Outside calibration, cap updates at current speech gate to avoid
      // pulling true speech into the ambient noise estimate.
      const noiseSample = isCalibrating ? rms : Math.min(rms, initialThresholds.speech)
      this.updateNoiseFloor(noiseSample)
    }

    if (isCalibrating) return

    const thresholds = this.computeAdaptiveThresholds()
    const speechThreshold = thresholds.speech
    const silenceThreshold = thresholds.silence
    const resumeSpeechThreshold = this.computeResumeThreshold(speechThreshold)

    switch (this.state) {
      case 'idle':
        this.resumeSpeechFrameCount = 0
        if (rms > speechThreshold) {
          this.speechFrameCount++
          if (this.speechFrameCount >= MIN_SPEECH_FRAMES) {
            this.state = 'speaking'
            this.speechStartTime = now - MIN_SPEECH_FRAMES * TICK_INTERVAL
            this._isSpeaking = true
            this.markSpeechStart()
            this.opts.onSpeechStart?.()
          }
        } else {
          this.speechFrameCount = 0
        }
        break

      case 'speaking':
        this.resumeSpeechFrameCount = 0
        if (rms < silenceThreshold) {
          this.state = 'silence_pending'
          this.silenceStartTime = now
        }
        break

      case 'silence_pending':
        if (rms > resumeSpeechThreshold) {
          this.resumeSpeechFrameCount++
          // Require a short, consecutive signal to avoid single-frame
          // noise spikes pulling silence_pending back into speaking.
          if (this.resumeSpeechFrameCount >= MIN_RESUME_SPEECH_FRAMES) {
            this.state = 'speaking'
            this.resumeSpeechFrameCount = 0
          }
        } else if (now - this.silenceStartTime >= this.opts.silenceDuration) {
          const speechDuration = now - this.speechStartTime
          this.state = 'idle'
          this._isSpeaking = false
          this.speechFrameCount = 0
          this.resumeSpeechFrameCount = 0

          if (speechDuration >= this.opts.minSpeechDuration) {
            this.emitSpeech()
          } else {
            this.discardSpeech()
          }
        } else {
          this.resumeSpeechFrameCount = 0
        }
        break
    }
  }

  private computeRMS(data: Uint8Array): number {
    let sum = 0
    for (let i = 0; i < data.length; i++) {
      const sample = ((data[i] ?? 128) - 128) / 128
      sum += sample * sample
    }
    return Math.sqrt(sum / data.length)
  }

  private updateNoiseFloor(sample: number): void {
    if (!Number.isFinite(sample)) return
    const clamped = Math.max(0, sample)
    if (this.noiseFloor <= 0) {
      this.noiseFloor = clamped
      return
    }
    const alpha = clamped > this.noiseFloor ? NOISE_FLOOR_RISE_ALPHA : NOISE_FLOOR_FALL_ALPHA
    this.noiseFloor += (clamped - this.noiseFloor) * alpha
  }

  private computeAdaptiveThresholds(): { speech: number; silence: number } {
    const adaptiveSpeech = this.noiseFloor * SPEECH_NOISE_RATIO + SPEECH_NOISE_OFFSET
    const adaptiveSilence = this.noiseFloor * SILENCE_NOISE_RATIO + SILENCE_NOISE_OFFSET
    const speech = Math.max(this.opts.speechThreshold, adaptiveSpeech)
    const silenceUpper = Math.max(0.001, speech - THRESHOLD_HYSTERESIS)
    const silence = Math.min(Math.max(this.opts.silenceThreshold, adaptiveSilence), silenceUpper)
    return { speech, silence }
  }

  private computeResumeThreshold(speechThreshold: number): number {
    const resumeOffset = Math.max(
      RESUME_THRESHOLD_MIN_OFFSET,
      speechThreshold * RESUME_THRESHOLD_RATIO,
      THRESHOLD_HYSTERESIS
    )
    return speechThreshold + resumeOffset
  }

  /** Create a MediaRecorder for the current stream. */
  private createRecorder(): MediaRecorder {
    if (!this.stream) throw new Error('No stream')
    try {
      return new MediaRecorder(this.stream, {
        mimeType: MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
          ? 'audio/webm;codecs=opus'
          : 'audio/webm',
      })
    } catch {
      return new MediaRecorder(this.stream!)
    }
  }

  /** Start a single continuous recorder. */
  private startRecorder(): void {
    if (!this.stream) return
    this.chunks = []
    this.speechStartIdx = -1

    const recorder = this.createRecorder()
    recorder.ondataavailable = (e) => {
      if (e.data.size > 0) {
        this.chunks.push(e.data)
      }
    }
    recorder.start(CHUNK_TIMESLICE)
    this.recorder = recorder
  }

  /** Stop the continuous recorder. */
  private stopRecorder(): void {
    if (this.recorder && this.recorder.state !== 'inactive') {
      try {
        this.recorder.stop()
      } catch {
        /* ignore */
      }
    }
    this.recorder = null
    this.chunks = []
    this.speechStartIdx = -1
  }

  /** Mark the start of a speech region (includes pre-buffer lookback). */
  private markSpeechStart(): void {
    // Include pre-buffer chunks before the current position
    this.speechStartIdx = Math.max(0, this.chunks.length - this.preBufferMaxChunks)
  }

  /** Stop recorder, extract speech blob, emit, and restart for next utterance. */
  private emitSpeech(): void {
    if (!this.recorder || this.recorder.state === 'inactive' || this.speechStartIdx < 0) {
      this.discardSpeech()
      return
    }

    const startIdx = this.speechStartIdx
    const mr = this.recorder
    this.recorder = null // prevent double-stop in cleanup paths
    this.speechStartIdx = -1

    mr.onstop = () => {
      // Build blob: chunk[0] always has the WebM init segment (EBML header + Tracks).
      // If speech starts after chunk[0], we need chunk[0] as prefix for valid WebM.
      let speechSlice: Blob[]
      if (startIdx === 0) {
        speechSlice = this.chunks
      } else {
        // chunk[0] = init segment, then speech region chunks
        const firstChunk = this.chunks[0]
        speechSlice = firstChunk
          ? [firstChunk, ...this.chunks.slice(startIdx)]
          : this.chunks.slice(startIdx)
      }

      const blob = new Blob(speechSlice, { type: mr.mimeType || 'audio/webm' })
      this.chunks = []
      this.opts.onSpeechEnd?.(blob)

      // Restart recorder for next utterance
      this.startRecorder()
    }
    mr.stop()
  }

  /** Discard current speech and restart for next utterance. */
  private discardSpeech(): void {
    if (!this.recorder || this.recorder.state === 'inactive') {
      // Already stopped or never started — just restart
      this.speechStartIdx = -1
      this.startRecorder()
      return
    }

    const mr = this.recorder
    this.recorder = null
    this.speechStartIdx = -1

    mr.onstop = () => {
      this.chunks = []
      // Restart recorder for next utterance
      this.startRecorder()
    }
    mr.stop()
  }
}
