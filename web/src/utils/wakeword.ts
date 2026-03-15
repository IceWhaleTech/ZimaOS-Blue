// Wake word detection using Web Audio API and simple keyword spotting
// This is a basic implementation that can be enhanced with more sophisticated models

// Type declarations for Web Speech API
interface SpeechRecognitionResult {
  readonly length: number
  item(index: number): SpeechRecognitionAlternative
  [index: number]: SpeechRecognitionAlternative
  readonly isFinal: boolean
}

interface SpeechRecognitionAlternative {
  readonly transcript: string
  readonly confidence: number
}

interface SpeechRecognitionResultList {
  readonly length: number
  item(index: number): SpeechRecognitionResult
  [index: number]: SpeechRecognitionResult
}

interface SpeechRecognitionEvent extends Event {
  readonly resultIndex: number
  readonly results: SpeechRecognitionResultList
}

interface SpeechRecognitionErrorEvent extends Event {
  readonly error: string
  readonly message: string
}

interface SpeechRecognition extends EventTarget {
  continuous: boolean
  interimResults: boolean
  lang: string
  onresult: ((event: SpeechRecognitionEvent) => void) | null
  onerror: ((event: SpeechRecognitionErrorEvent) => void) | null
  onend: (() => void) | null
  start(): void
  stop(): void
  abort(): void
}

interface SpeechRecognitionConstructor {
  new (): SpeechRecognition
}

export interface WakeWordConfig {
  wakeWord: string
  sensitivity: number // 0.0 - 1.0
  sampleRate: number
  bufferSize: number
}

export type WakeWordCallback = () => void

// Simple wake word detector using Web Speech API
export class WakeWordDetector {
  private recognition: SpeechRecognition | null = null
  private isListening = false
  private wakeWord: string
  private sensitivity: number
  private onWakeWord: WakeWordCallback | null = null
  private onError: ((error: string) => void) | null = null

  constructor(config: Partial<WakeWordConfig> = {}) {
    this.wakeWord = (config.wakeWord || 'hey echo').toLowerCase()
    this.sensitivity = config.sensitivity || 0.7

    // Check for Web Speech API support
    const SpeechRecognitionCtor =
      (window as unknown as { SpeechRecognition?: SpeechRecognitionConstructor })
        .SpeechRecognition ||
      (window as unknown as { webkitSpeechRecognition?: SpeechRecognitionConstructor })
        .webkitSpeechRecognition

    if (!SpeechRecognitionCtor) {
      console.warn('Web Speech API not supported')
      return
    }

    this.recognition = new SpeechRecognitionCtor()
    this.recognition.continuous = true
    this.recognition.interimResults = true
    this.recognition.lang = 'en-US'

    this.recognition.onresult = (event: SpeechRecognitionEvent) => {
      this.handleResult(event)
    }

    this.recognition.onerror = (event: SpeechRecognitionErrorEvent) => {
      if (event.error !== 'no-speech') {
        this.onError?.(event.error)
      }
    }

    this.recognition.onend = () => {
      // Restart if still listening
      if (this.isListening) {
        try {
          this.recognition?.start()
        } catch {
          // Ignore errors on restart
        }
      }
    }
  }

  private handleResult(event: SpeechRecognitionEvent) {
    for (let i = event.resultIndex; i < event.results.length; i++) {
      const result = event.results[i]
      if (!result) continue
      const firstAlternative = result[0]
      if (!firstAlternative) continue
      const transcript = firstAlternative.transcript.toLowerCase().trim()
      const confidence = firstAlternative.confidence

      // Check if wake word is detected
      if (this.containsWakeWord(transcript) && confidence >= this.sensitivity) {
        this.onWakeWord?.()
      }
    }
  }

  private containsWakeWord(transcript: string): boolean {
    // Normalize both strings
    const normalizedTranscript = transcript.replace(/[^\w\s]/g, '').toLowerCase()
    const normalizedWakeWord = this.wakeWord.replace(/[^\w\s]/g, '').toLowerCase()

    // Check for exact match or close match
    if (normalizedTranscript.includes(normalizedWakeWord)) {
      return true
    }

    // Check for similar phrases (fuzzy matching)
    const words = normalizedTranscript.split(/\s+/)
    const wakeWords = normalizedWakeWord.split(/\s+/)

    // Simple fuzzy matching
    let matchCount = 0
    for (const wakeWordPart of wakeWords) {
      for (const word of words) {
        if (this.isSimilar(word, wakeWordPart)) {
          matchCount++
          break
        }
      }
    }

    return matchCount >= wakeWords.length * this.sensitivity
  }

  private isSimilar(a: string, b: string): boolean {
    if (a === b) return true
    if (Math.abs(a.length - b.length) > 2) return false

    // Simple Levenshtein distance check
    const distance = this.levenshteinDistance(a, b)
    const maxLength = Math.max(a.length, b.length)
    const similarity = 1 - distance / maxLength

    return similarity >= this.sensitivity
  }

  private levenshteinDistance(a: string, b: string): number {
    const matrix: number[][] = []

    for (let i = 0; i <= b.length; i++) {
      matrix[i] = [i]
    }

    const firstRow = matrix[0]
    if (firstRow) {
      for (let j = 0; j <= a.length; j++) {
        firstRow[j] = j
      }
    }

    for (let i = 1; i <= b.length; i++) {
      for (let j = 1; j <= a.length; j++) {
        const currentRow = matrix[i]
        const prevRow = matrix[i - 1]
        if (!currentRow || !prevRow) continue

        if (b.charAt(i - 1) === a.charAt(j - 1)) {
          currentRow[j] = prevRow[j - 1] ?? 0
        } else {
          currentRow[j] = Math.min(
            (prevRow[j - 1] ?? 0) + 1,
            (currentRow[j - 1] ?? 0) + 1,
            (prevRow[j] ?? 0) + 1
          )
        }
      }
    }

    return matrix[b.length]?.[a.length] ?? 0
  }

  setWakeWord(wakeWord: string) {
    this.wakeWord = wakeWord.toLowerCase()
  }

  setSensitivity(sensitivity: number) {
    this.sensitivity = Math.max(0, Math.min(1, sensitivity))
  }

  setLanguage(lang: string) {
    if (this.recognition) {
      this.recognition.lang = lang
    }
  }

  setOnWakeWord(callback: WakeWordCallback) {
    this.onWakeWord = callback
  }

  setOnError(callback: (error: string) => void) {
    this.onError = callback
  }

  start(): boolean {
    if (!this.recognition) {
      this.onError?.('Speech recognition not supported')
      return false
    }

    if (this.isListening) {
      return true
    }

    try {
      this.recognition.start()
      this.isListening = true
      return true
    } catch (e) {
      this.onError?.(e instanceof Error ? e.message : 'Failed to start')
      return false
    }
  }

  stop() {
    this.isListening = false
    if (this.recognition) {
      try {
        this.recognition.stop()
      } catch {
        // Ignore errors on stop
      }
    }
  }

  get listening(): boolean {
    return this.isListening
  }

  static isSupported(): boolean {
    return !!(
      (window as unknown as { SpeechRecognition?: unknown }).SpeechRecognition ||
      (window as unknown as { webkitSpeechRecognition?: unknown }).webkitSpeechRecognition
    )
  }
}

// Alternative: Audio-based wake word detection using energy levels
// This is useful when Web Speech API is not available
export class AudioWakeWordDetector {
  private audioContext: AudioContext | null = null
  private analyser: AnalyserNode | null = null
  private mediaStream: MediaStream | null = null
  private isListening = false
  private energyThreshold: number
  private silenceTimeout: number
  private lastSpeechTime = 0
  private onSpeechDetected: (() => void) | null = null
  private animationFrame: number | null = null

  constructor(config: { energyThreshold?: number; silenceTimeout?: number } = {}) {
    this.energyThreshold = config.energyThreshold || 0.02
    this.silenceTimeout = config.silenceTimeout || 1000
  }

  async start(): Promise<boolean> {
    try {
      this.mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true })
      this.audioContext = new AudioContext()
      this.analyser = this.audioContext.createAnalyser()
      this.analyser.fftSize = 256

      const source = this.audioContext.createMediaStreamSource(this.mediaStream)
      source.connect(this.analyser)

      this.isListening = true
      this.detectSpeech()
      return true
    } catch (e) {
      console.error('Failed to start audio detection:', e)
      return false
    }
  }

  private detectSpeech() {
    if (!this.isListening || !this.analyser) return

    const dataArray = new Uint8Array(this.analyser.frequencyBinCount)
    this.analyser.getByteFrequencyData(dataArray)

    // Calculate average energy
    const energy = dataArray.reduce((sum, val) => sum + val, 0) / dataArray.length / 255

    if (energy > this.energyThreshold) {
      const now = Date.now()
      if (now - this.lastSpeechTime > this.silenceTimeout) {
        // Speech detected after silence
        this.onSpeechDetected?.()
      }
      this.lastSpeechTime = now
    }

    this.animationFrame = requestAnimationFrame(() => this.detectSpeech())
  }

  stop() {
    this.isListening = false

    if (this.animationFrame) {
      cancelAnimationFrame(this.animationFrame)
      this.animationFrame = null
    }

    if (this.mediaStream) {
      this.mediaStream.getTracks().forEach((track) => track.stop())
      this.mediaStream = null
    }

    if (this.audioContext) {
      this.audioContext.close()
      this.audioContext = null
    }
  }

  setOnSpeechDetected(callback: () => void) {
    this.onSpeechDetected = callback
  }

  get listening(): boolean {
    return this.isListening
  }
}
