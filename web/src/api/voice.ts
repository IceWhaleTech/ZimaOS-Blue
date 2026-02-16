import api from './client'

// Types
export interface VoiceSession {
  id: string
  user_id: string
  state: VoiceSessionState
  language: string
  voice: string
  created_at: string
  last_activity: string
  conversation_id?: string
}

export type VoiceSessionState = 'idle' | 'listening' | 'processing' | 'speaking'

export interface TranscribeResponse {
  text: string
  language?: string
  duration?: number
  confidence?: number
}

export interface SynthesizeResponse {
  audio: string // Base64 encoded
  content_type: string
  format: string
}

export interface Voice {
  id: string
  name: string
  language: string
  gender?: string
  description?: string
  preview_url?: string
}

export interface VoiceConfig {
  language: string
  voice: string
  wake_word?: string
  wake_word_enabled: boolean
  continuous_listening: boolean
  auto_play_response: boolean
}

export interface CreateSessionRequest {
  language?: string
  voice?: string
  wake_word?: string
  wake_word_enabled?: boolean
  continuous_listening?: boolean
  auto_play_response?: boolean
}

// WebSocket message types
export type WSMessageType =
  | 'audio'
  | 'transcript'
  | 'response'
  | 'audio_response'
  | 'state_change'
  | 'error'
  | 'config'
  | 'wake_word'
  | 'ping'
  | 'pong'

export interface WSMessage {
  type: WSMessageType
  data?: unknown
  error?: string
}

// Voice API
export const voiceApi = {
  // Transcribe audio
  transcribe: (audio: Blob, format: string, language?: string) => {
    const formData = new FormData()
    formData.append('audio', audio, `audio.${format}`)
    formData.append('format', format)
    if (language) {
      formData.append('language', language)
    }
    return api.post<TranscribeResponse>('/voice/transcribe', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  // Transcribe audio (base64)
  transcribeBase64: (audio: string, format: string, language?: string) =>
    api.post<TranscribeResponse>('/voice/transcribe', {
      audio,
      format,
      language,
    }),

  // Synthesize text to speech
  synthesize: (text: string, voice?: string, format?: string, speed?: number, provider?: string) =>
    api.post<SynthesizeResponse>(
      '/voice/synthesize',
      { text, voice, format, speed, provider },
      { headers: { Accept: 'application/json' } }
    ),

  // Synthesize and get audio blob
  synthesizeAudio: async (
    text: string,
    voice?: string,
    format?: string,
    speed?: number,
    provider?: string
  ): Promise<Blob> => {
    const response = await api.post('/voice/synthesize', { text, voice, format, speed, provider }, {
      responseType: 'blob',
    })
    return response.data
  },

  // List available voices
  listVoices: () => api.get<Voice[]>('/voice/voices'),

  // Streaming TTS via SSE - synthesize text and stream audio chunks
  synthesizeStream: (text: string, format?: string): EventSource => {
    const params = new URLSearchParams({ text, format: format || 'mp3' })
    return new EventSource(`/api/v1/voice/synthesize/stream?${params.toString()}`)
  },

  // Create voice session
  createSession: (data: CreateSessionRequest) =>
    api.post<VoiceSession>('/voice/sessions', data),

  // Get voice session
  getSession: (id: string) => api.get<VoiceSession>(`/voice/sessions/${id}`),

  // Close voice session
  closeSession: (id: string) => api.delete<{ status: string }>(`/voice/sessions/${id}`),
}

// WebSocket connection helper
export class VoiceWebSocket {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private pingInterval: number | null = null

  public onMessage: ((msg: WSMessage) => void) | null = null
  public onStateChange: ((state: VoiceSessionState) => void) | null = null
  public onTranscript: ((text: string) => void) | null = null
  public onResponse: ((text: string) => void) | null = null
  public onAudioResponse: ((audio: string, contentType: string) => void) | null = null
  public onError: ((error: string) => void) | null = null
  public onConnect: (() => void) | null = null
  public onDisconnect: (() => void) | null = null

  connect(language?: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const token = localStorage.getItem('token')
      if (!token) {
        reject(new Error('Not authenticated'))
        return
      }

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const lang = language || 'en'
      const wsUrl = `${protocol}//${window.location.host}/api/v1/voice/stream?token=${token}&language=${lang}`

      this.ws = new WebSocket(wsUrl)

      this.ws.onopen = () => {
        this.reconnectAttempts = 0
        this.startPing()
        this.onConnect?.()
        resolve()
      }

      this.ws.onclose = () => {
        this.stopPing()
        this.onDisconnect?.()
        this.attemptReconnect()
      }

      this.ws.onerror = (event) => {
        console.error('WebSocket error:', event)
        reject(new Error('WebSocket connection failed'))
      }

      this.ws.onmessage = (event) => {
        try {
          const msg: WSMessage = JSON.parse(event.data)
          this.handleMessage(msg)
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e)
        }
      }
    })
  }

  private handleMessage(msg: WSMessage) {
    this.onMessage?.(msg)

    switch (msg.type) {
      case 'state_change':
        if (msg.data && typeof msg.data === 'object' && 'state' in msg.data) {
          this.onStateChange?.(msg.data.state as VoiceSessionState)
        }
        break
      case 'transcript':
        if (msg.data && typeof msg.data === 'object' && 'text' in msg.data) {
          this.onTranscript?.(msg.data.text as string)
        }
        break
      case 'response':
        if (msg.data && typeof msg.data === 'object' && 'text' in msg.data) {
          this.onResponse?.(msg.data.text as string)
        }
        break
      case 'audio_response':
        if (msg.data && typeof msg.data === 'object') {
          const data = msg.data as { audio: string; content_type: string }
          this.onAudioResponse?.(data.audio, data.content_type)
        }
        break
      case 'error':
        this.onError?.(msg.error || 'Unknown error')
        break
      case 'pong':
        // Pong received, connection is alive
        break
    }
  }

  private startPing() {
    this.pingInterval = window.setInterval(() => {
      this.send({ type: 'ping' })
    }, 30000)
  }

  private stopPing() {
    if (this.pingInterval) {
      clearInterval(this.pingInterval)
      this.pingInterval = null
    }
  }

  private attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      return
    }

    this.reconnectAttempts++
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1)

    setTimeout(() => {
      this.connect().catch(() => {
        // Reconnect failed, will try again
      })
    }, delay)
  }

  send(msg: WSMessage) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg))
    }
  }

  sendAudio(audio: string, format: string) {
    this.send({
      type: 'audio',
      data: { audio, format },
    })
  }

  updateConfig(config: Partial<VoiceConfig>) {
    this.send({
      type: 'config',
      data: config,
    })
  }

  disconnect() {
    this.stopPing()
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  get isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN
  }
}

// Audio recording helper
export class AudioRecorder {
  private mediaRecorder: MediaRecorder | null = null
  private audioChunks: Blob[] = []
  private stream: MediaStream | null = null

  public onDataAvailable: ((data: Blob) => void) | null = null
  public onStop: ((audio: Blob) => void) | null = null
  public onError: ((error: Error) => void) | null = null

  async start(mimeType?: string): Promise<void> {
    try {
      this.stream = await navigator.mediaDevices.getUserMedia({ audio: true })

      // Determine best supported format
      const supportedTypes = [
        'audio/webm;codecs=opus',
        'audio/webm',
        'audio/ogg;codecs=opus',
        'audio/mp4',
        'audio/wav',
      ]

      let selectedType = mimeType
      if (!selectedType) {
        for (const type of supportedTypes) {
          if (MediaRecorder.isTypeSupported(type)) {
            selectedType = type
            break
          }
        }
      }

      const options: MediaRecorderOptions = {}
      if (selectedType) {
        options.mimeType = selectedType
      }

      this.mediaRecorder = new MediaRecorder(this.stream, options)
      this.audioChunks = []

      this.mediaRecorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          this.audioChunks.push(event.data)
          this.onDataAvailable?.(event.data)
        }
      }

      this.mediaRecorder.onstop = () => {
        const audioBlob = new Blob(this.audioChunks, {
          type: this.mediaRecorder?.mimeType || 'audio/webm',
        })
        this.onStop?.(audioBlob)
      }

      this.mediaRecorder.onerror = (event) => {
        this.onError?.(new Error('Recording error: ' + event.type))
      }

      this.mediaRecorder.start(100) // Collect data every 100ms
    } catch (error) {
      this.onError?.(error as Error)
      throw error
    }
  }

  stop(): void {
    if (this.mediaRecorder && this.mediaRecorder.state !== 'inactive') {
      this.mediaRecorder.stop()
    }
    if (this.stream) {
      this.stream.getTracks().forEach((track) => track.stop())
      this.stream = null
    }
  }

  get isRecording(): boolean {
    return this.mediaRecorder !== null && this.mediaRecorder.state === 'recording'
  }

  get mimeType(): string {
    return this.mediaRecorder?.mimeType || 'audio/webm'
  }
}

// Helper to convert blob to base64
export async function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onloadend = () => {
      const base64 = reader.result as string
      // Remove data URL prefix
      const base64Data = base64.split(',')[1] || ''
      resolve(base64Data)
    }
    reader.onerror = reject
    reader.readAsDataURL(blob)
  })
}

// Helper to convert base64 to blob
export function base64ToBlob(base64: string, contentType: string): Blob {
  const byteCharacters = atob(base64)
  const byteNumbers = new Array(byteCharacters.length)
  for (let i = 0; i < byteCharacters.length; i++) {
    byteNumbers[i] = byteCharacters.charCodeAt(i)
  }
  const byteArray = new Uint8Array(byteNumbers)
  return new Blob([byteArray], { type: contentType })
}

// Global TTS audio manager - ensures only one audio plays at a time
class TTSAudioManager {
  private currentAudio: HTMLAudioElement | null = null
  private currentUrl: string | null = null
  private onStopCallback: (() => void) | null = null
  private pendingReject: ((reason?: unknown) => void) | null = null

  play(base64: string, contentType: string, onStop?: () => void): Promise<void> {
    // Stop any currently playing audio
    this.stop()

    return new Promise((resolve, reject) => {
      const blob = base64ToBlob(base64, contentType)
      const url = URL.createObjectURL(blob)
      const audio = new Audio(url)

      this.currentAudio = audio
      this.currentUrl = url
      this.onStopCallback = onStop || null
      this.pendingReject = reject

      audio.onended = () => {
        this.pendingReject = null
        this.cleanup()
        resolve()
      }

      audio.onerror = (e) => {
        this.pendingReject = null
        this.cleanup()
        reject(e)
      }

      audio.play().catch((e) => {
        this.pendingReject = null
        this.cleanup()
        reject(e)
      })
    })
  }

  stop(): boolean {
    if (this.currentAudio) {
      this.currentAudio.pause()
      const rejectFn = this.pendingReject
      this.pendingReject = null
      this.cleanup()
      // Reject the pending promise so callers (e.g. playNext) don't hang
      if (rejectFn) rejectFn(new DOMException('Playback stopped', 'AbortError'))
      return true
    }
    return false
  }

  private cleanup() {
    if (this.currentUrl) {
      URL.revokeObjectURL(this.currentUrl)
      this.currentUrl = null
    }
    this.currentAudio = null
    if (this.onStopCallback) {
      this.onStopCallback()
      this.onStopCallback = null
    }
  }

  isPlaying(): boolean {
    return this.currentAudio !== null && !this.currentAudio.paused
  }
}

// Singleton instance
export const ttsAudioManager = new TTSAudioManager()

// Streaming TTS Queue Manager - plays sentences as they arrive
class StreamingTTSManager {
  private queue: Array<{ text: string; audio?: string; contentType?: string }> = []
  private playIndex = 0 // cursor: next item to play
  private isPlaying = false
  private isStopped = false
  private currentEventSource: EventSource | null = null
  public onSentenceStart: ((index: number) => void) | null = null
  public onComplete: (() => void) | null = null

  // Split text into sentences
  private splitIntoSentences(text: string): string[] {
    // Split by sentence-ending punctuation, keeping the punctuation
    const sentences = text.match(/[^.!?。！？]+[.!?。！？]+|[^.!?。！？]+$/g) || [text]
    return sentences.map(s => s.trim()).filter(s => s.length > 0)
  }

  // Append new text to the queue without stopping current playback
  async streamText(text: string): Promise<void> {
    if (this.isStopped) {
      // First call or after stop — reset state
      this.isStopped = false
      this.queue = []
      this.playIndex = 0
    }

    const sentences = this.splitIntoSentences(text)
    if (sentences.length === 0) return

    // Append new sentences to queue, fetch audio for each
    const startIndex = this.queue.length
    for (const s of sentences) {
      this.queue.push({ text: s })
    }
    sentences.forEach((sentence, i) => {
      this.fetchAudio(sentence, startIndex + i)
    })

    // Kick off playback if not already running
    if (!this.isPlaying) {
      this.playNext()
    }
  }

  private async fetchAudio(text: string, index: number) {
    if (this.isStopped) return

    try {
      const es = new EventSource(`/api/v1/voice/synthesize/stream?text=${encodeURIComponent(text)}&format=mp3`)

      es.addEventListener('audio', (e) => {
        if (this.isStopped) { es.close(); return }
        const data = JSON.parse(e.data)
        if (this.queue[index]) {
          this.queue[index].audio = data.audio
          this.queue[index].contentType = data.content_type
        }
        es.close()
        // Nudge playback in case it was waiting for this audio
        if (!this.isPlaying) this.playNext()
      })

      es.addEventListener('error', () => es.close())
    } catch (e) {
      console.error('Failed to fetch TTS audio:', e)
    }
  }

  private async playNext() {
    if (this.isStopped || this.isPlaying) return
    if (this.playIndex >= this.queue.length) {
      // All items played — check if more might arrive
      // (caller may append more via streamText)
      this.onComplete?.()
      return
    }

    const item = this.queue[this.playIndex]
    if (!item.audio || !item.contentType) {
      // Audio not ready yet, wait and retry
      setTimeout(() => this.playNext(), 100)
      return
    }

    this.isPlaying = true
    this.onSentenceStart?.(this.playIndex)

    try {
      await ttsAudioManager.play(item.audio, item.contentType)
    } catch (e) {
      console.error('Failed to play audio:', e)
    }

    this.isPlaying = false
    this.playIndex++

    if (!this.isStopped) {
      this.playNext()
    }
  }

  stop() {
    this.isStopped = true
    this.isPlaying = false
    this.queue = []
    this.playIndex = 0
    this.currentEventSource?.close()
    ttsAudioManager.stop()
  }
}

export const streamingTTSManager = new StreamingTTSManager()

// Helper to play audio from base64
export function playAudioFromBase64(base64: string, contentType: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const blob = base64ToBlob(base64, contentType)
    const url = URL.createObjectURL(blob)
    const audio = new Audio(url)

    audio.onended = () => {
      URL.revokeObjectURL(url)
      resolve()
    }

    audio.onerror = (e) => {
      URL.revokeObjectURL(url)
      reject(e)
    }

    audio.play().catch(reject)
  })
}
