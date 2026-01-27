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
  synthesize: (text: string, voice?: string, format?: string, speed?: number) =>
    api.post<SynthesizeResponse>(
      '/voice/synthesize',
      { text, voice, format, speed },
      { headers: { Accept: 'application/json' } }
    ),

  // Synthesize and get audio blob
  synthesizeAudio: async (
    text: string,
    voice?: string,
    format?: string,
    speed?: number
  ): Promise<Blob> => {
    const response = await api.post('/voice/synthesize', { text, voice, format, speed }, {
      responseType: 'blob',
    })
    return response.data
  },

  // List available voices
  listVoices: () => api.get<Voice[]>('/voice/voices'),

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

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const token = localStorage.getItem('token')
      if (!token) {
        reject(new Error('Not authenticated'))
        return
      }

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.host}/api/v1/voice/stream?token=${token}`

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
