import api, { ensureFreshToken } from './client'

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
  audio?: string // Base64 encoded
  content_type?: string
  format?: string
  played_locally?: boolean // Server played through local speakers
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
      timeout: 20000,
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
  synthesize: (text: string) =>
    api.post<SynthesizeResponse>(
      '/voice/synthesize',
      { text },
      { headers: { Accept: 'application/json' }, timeout: 120000 }
    ),

  // Synthesize and get audio blob
  synthesizeAudio: async (text: string): Promise<Blob> => {
    const response = await api.post('/voice/synthesize', { text }, {
      responseType: 'blob',
      timeout: 120000,
    })
    return response.data
  },

  // List available voices
  listVoices: () => api.get<Voice[]>('/voice/voices'),

  // Stop local speech playback
  stopSpeaking: () => api.post('/voice/synthesize/stop'),

  // Streaming TTS via SSE - synthesize text and stream audio chunks
  synthesizeStream: (text: string, format?: string): EventSource => {
    const params = new URLSearchParams({ text, ...(format ? { format } : {}) })
    const token = localStorage.getItem('token')
    if (token) params.set('token', token)
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

    setTimeout(async () => {
      // Refresh token before reconnecting — expired tokens cause repeated failures
      await ensureFreshToken().catch(() => {})
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

// Human-like speech preprocessing - converts technical content to natural speech
function preprocessForSpeech(text: string, locale: string = 'en-US'): string {
  const isZh = locale.startsWith('zh')

  // Replace mermaid diagrams first (before general code blocks)
  text = text.replace(/```mermaid[\s\S]*?```/g, () => {
    return isZh ? '流程图。' : 'Diagram.'
  })

  // Replace code blocks with language-specific description
  text = text.replace(/```(\w+)?\s*\n[\s\S]*?```/g, (_, lang) => {
    if (lang) {
      return isZh ? `${lang}代码块。` : `${lang} code block.`
    }
    return isZh ? '代码块。' : 'Code block.'
  })

  // Replace inline code with description
  text = text.replace(/`([^`]+)`/g, (_match, code) => {
    // Keep short inline code (< 15 chars), replace long ones
    if (code.length > 15) {
      return isZh ? '代码' : 'code'
    }
    // Remove backticks but keep the content for short code
    return code
  })

  // Replace tables with description
  text = text.replace(/\|[^\n]*\|[\s\S]*?\n\s*\n/g, () => {
    return isZh ? '表格。' : 'Table.'
  })

  // Replace markdown images with description
  text = text.replace(/!\[([^\]]*)\]\([^)]+\)/g, (_, alt) => {
    if (alt && alt.trim()) {
      return isZh ? `图片：${alt}。` : `Image: ${alt}.`
    }
    return isZh ? '图片。' : 'Image.'
  })

  // Replace URLs with description (keep domain for context)
  text = text.replace(/https?:\/\/[^\s<>]+/g, (url) => {
    try {
      const domain = new URL(url).hostname.replace('www.', '')
      return isZh ? `链接到${domain}` : `link to ${domain}`
    } catch {
      return isZh ? '链接' : 'link'
    }
  })

  // Replace markdown links - keep link text, remove URL
  text = text.replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')

  // Replace HTML tags
  text = text.replace(/<[^>]+>/g, '')

  // Replace markdown headers with just the text
  text = text.replace(/^#{1,6}\s+/gm, '')

  // Replace markdown bold/italic
  text = text.replace(/\*\*([^*]+)\*\*/g, '$1')
  text = text.replace(/\*([^*]+)\*/g, '$1')
  text = text.replace(/__([^_]+)__/g, '$1')
  text = text.replace(/_([^_]+)_/g, '$1')

  // Replace markdown strikethrough
  text = text.replace(/~~([^~]+)~~/g, '$1')

  // Replace markdown lists - keep content, remove markers
  text = text.replace(/^\s*[-*+]\s+/gm, '')
  text = text.replace(/^\s*\d+\.\s+/gm, '')

  // Replace multiple spaces/newlines with single space
  text = text.replace(/\s+/g, ' ')

  return text.trim()
}

// Streaming TTS Queue Manager - plays sentences as they arrive
class StreamingTTSManager {
  private queue: Array<{
    text: string
    segments?: Array<{ audio: string; contentType: string }>
    failed?: boolean
    playedLocally?: boolean
    fetching?: boolean
    fetchDone?: boolean
    _retried?: boolean
  }> = []
  private playIndex = 0 // cursor: next item to play
  private fetchIndex = 0 // cursor: next item to fetch
  private isPlaying = false
  private isStopped = true // start as stopped so first streamText resets
  private playbackStarted = false // whether play() has been called
  private generation = 0 // increments on reset/stop — stale async callbacks bail out
  private activeFetches = 0 // concurrent fetch limiter
  private static readonly MAX_CONCURRENT_FETCHES = 2
  private currentEventSource: EventSource | null = null
  private locale: string = 'en-US'
  public onSentenceStart: ((index: number) => void) | null = null
  public onComplete: (() => void) | null = null

  setLocale(locale: string) {
    this.locale = locale
  }

  private splitIntoSentences(text: string): string[] {
    const sentences = text.match(/[^.!?。！？]+[.!?。！？]+|[^.!?。！？]+$/g) || [text]
    return sentences.map(s => s.trim()).filter(s => s.length > 0)
  }

  // Reset state for a fresh session
  reset() {
    this.generation++
    this.isStopped = false
    this.playbackStarted = false
    this.queue = []
    this.playIndex = 0
    this.fetchIndex = 0
    this.isPlaying = false
    this.activeFetches = 0
  }

  // Prefetch only — enqueue sentences and start fetching audio, but don't play
  async streamText(text: string): Promise<void> {
    if (this.isStopped) {
      this.isStopped = false
      this.queue = []
      this.playIndex = 0
      this.fetchIndex = 0
    }

    const processedText = preprocessForSpeech(text, this.locale)
    const sentences = this.splitIntoSentences(processedText)
    if (sentences.length === 0) return

    for (const s of sentences) {
      this.queue.push({ text: s, segments: [] })
    }

    // Kick off fetch pipeline (prefetch only)
    this.fetchNext()

    // If playback was already started (e.g. auto-play streaming), keep playing
    if (this.playbackStarted && !this.isPlaying) {
      this.playNext()
    }
  }

  // Start playback of the prefetched queue
  play() {
    this.playbackStarted = true
    if (!this.isPlaying) {
      this.playNext()
    }
  }

  // Convenience: prefetch + play in one call (for auto-play streaming)
  async streamAndPlay(text: string): Promise<void> {
    this.playbackStarted = true
    await this.streamText(text)
  }

  private fetchNext() {
    if (this.isStopped) return
    if (this.fetchIndex >= this.queue.length) return
    if (this.activeFetches >= StreamingTTSManager.MAX_CONCURRENT_FETCHES) return

    const item = this.queue[this.fetchIndex]
    if (!item) {
      this.fetchIndex++
      this.fetchNext()
      return
    }
    if (item.fetching || item.fetchDone || item.playedLocally || item.failed) {
      this.fetchIndex++
      this.fetchNext()
      return
    }

    item.fetching = true
    this.activeFetches++
    this.fetchAudio(item.text, this.fetchIndex)
    // Try to fill up to MAX_CONCURRENT_FETCHES
    this.fetchIndex++
    this.fetchNext()
  }

  private async fetchAudio(text: string, index: number) {
    if (this.isStopped) { this.activeFetches--; return }
    const gen = this.generation

    const onDone = () => {
      if (gen === this.generation) this.activeFetches--
    }

    try {
      const token = localStorage.getItem('token')
      const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''
      const es = new EventSource(`/api/v1/voice/synthesize/stream?text=${encodeURIComponent(text)}${tokenParam}`)
      let received = false
      let closed = false

      es.addEventListener('audio', (e) => {
        if (gen !== this.generation) { es.close(); closed = true; onDone(); return }
        if (this.isStopped) { es.close(); closed = true; onDone(); return }
        const data = JSON.parse(e.data)
        if (this.queue[index]) {
          if (data.played_locally) {
            this.queue[index].playedLocally = true
            this.queue[index].fetchDone = true
            this.queue[index].fetching = false
          } else {
            received = true
            const segments = this.queue[index].segments || (this.queue[index].segments = [])
            segments.push({
              audio: data.audio,
              contentType: data.content_type,
            })
          }
        }
        if (this.playbackStarted && !this.isPlaying) this.playNext()
      })

      es.addEventListener('error', () => {
        if (closed) return // Ignore error events after intentional close
        es.close()
        closed = true
        if (gen !== this.generation) { onDone(); return }
        if (this.queue[index]) {
          this.queue[index].fetching = false
          this.queue[index].fetchDone = true
          if (!received && !this.queue[index].playedLocally) {
            this.queue[index].failed = true
          }
        }
        onDone()
        if (gen === this.generation) {
          this.fetchNext()
          if (this.playbackStarted && !this.isPlaying) this.playNext()
        }
      })

      es.addEventListener('done', () => {
        if (closed) return
        es.close()
        closed = true
        if (gen !== this.generation) { onDone(); return }
        if (this.queue[index]) {
          this.queue[index].fetching = false
          this.queue[index].fetchDone = true
          if (!received && !this.queue[index].playedLocally) {
            this.queue[index].failed = true
          }
        }
        onDone()
        if (gen === this.generation) {
          this.fetchNext()
          if (this.playbackStarted && !this.isPlaying) this.playNext()
        }
      })
    } catch (e) {
      onDone()
      if (gen !== this.generation) return
      console.error('Failed to fetch TTS audio:', e)
      if (this.queue[index]) {
        this.queue[index].failed = true
        this.queue[index].fetching = false
        this.queue[index].fetchDone = true
        this.fetchNext()
        if (this.playbackStarted && !this.isPlaying) this.playNext()
      }
    }
  }

  private async playNext() {
    if (this.isStopped || this.isPlaying || !this.playbackStarted) return
    const gen = this.generation
    if (this.playIndex >= this.queue.length) {
      this.onComplete?.()
      return
    }

    const item = this.queue[this.playIndex]
    if (!item) {
      this.onComplete?.()
      return
    }

    if (item.failed) {
      // Retry failed fetches once before skipping
      if (!item._retried) {
        item._retried = true
        item.failed = false
        item.fetching = false
        item.fetchDone = false
        item.segments = []
        // Re-fetch this item
        this.fetchAudio(item.text, this.playIndex)
        // Wait for re-fetch with polling
        const retryGen = this.generation
        setTimeout(() => {
          if (retryGen === this.generation) this.playNext()
        }, 200)
        return
      }
      this.playIndex++
      this.playNext()
      return
    }

    if (item.playedLocally) {
      this.playIndex++
      this.playNext()
      return
    }

    if (!item.fetchDone) {
      const retryKey = `_retries_${this.playIndex}`
      const retries = (this as any)[retryKey] || 0
      if (retries > 150) {
        console.warn('TTS audio fetch timeout, skipping sentence:', item.text)
        delete (this as any)[retryKey]
        this.playIndex++
        this.playNext()
        return
      }
      (this as any)[retryKey] = retries + 1
      setTimeout(() => {
        if (gen === this.generation) this.playNext()
      }, 100)
      return
    }

    delete (this as any)[`_retries_${this.playIndex}`]

    const segments = item.segments || []
    if (segments.length === 0) {
      this.playIndex++
      this.playNext()
      return
    }

    this.isPlaying = true
    this.onSentenceStart?.(this.playIndex)

    try {
      for (const segment of segments) {
        if (gen !== this.generation || this.isStopped) {
          this.isPlaying = false
          return
        }
        await ttsAudioManager.play(segment.audio, segment.contentType)
      }
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') {
        this.isPlaying = false
        return
      }
      console.error('Failed to play audio:', e)
    }

    this.isPlaying = false
    if (gen !== this.generation) return
    this.playIndex++

    if (!this.isStopped) {
      this.playNext()
    }
  }

  stop() {
    const shouldStopServerSpeech = this.activeFetches > 0
    this.generation++
    this.isStopped = true
    this.isPlaying = false
    this.playbackStarted = false
    this.queue = []
    this.playIndex = 0
    this.fetchIndex = 0
    this.activeFetches = 0
    this.currentEventSource?.close()
    ttsAudioManager.stop()
    if (shouldStopServerSpeech) {
      voiceApi.stopSpeaking().catch(() => {})
    }
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
