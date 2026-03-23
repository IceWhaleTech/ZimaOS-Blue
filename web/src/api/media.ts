import api, { ensureFreshToken } from './client'

// Types
export type MediaType = 'image' | 'video'
export type TaskStatus = 'pending' | 'processing' | 'succeeded' | 'failed' | 'cancelled'
export type MediaCategory = 't2i' | 't2v' | 'i2v' | 'i2i' | 'kf2v'

export interface MediaRequest {
  prompt: string
  negative_prompt?: string
  model?: string
  n?: number
  size?: string
  quality?: string
  style?: string
  duration?: number
  type?: MediaType
}

export interface MediaResult {
  url: string
  thumbnail_url?: string
  revised_prompt?: string
  content_type?: string
  width?: number
  height?: number
  duration_sec?: number
}

export interface MediaResponse {
  created: number
  data: MediaResult[]
}

export interface MediaTask {
  id: string
  status: TaskStatus
  type: MediaType
  category?: MediaCategory
  provider: string
  model: string
  progress: number
  message_id?: string
  response?: MediaResponse
  error?: string
  fallback_info?: MediaFallbackInfo
  created_at: string
  completed_at?: string
}

export interface MediaModelInfo {
  id: string
  name: string
  type: MediaType
  category?: MediaCategory
  provider: string
  max_resolution?: string
  supported_sizes?: string[]
  price?: number
  pricing_unit?: string
  is_fallback?: boolean
  fallback_strategy?: 'web_canvas' | 'public_space' | 'native_timeline'
}

export interface MediaFallbackInfo {
  used: boolean
  strategy: 'web_canvas' | 'public_space' | 'native_timeline'
  display_name: string
  source_urls?: string[]
  sources?: MediaFallbackSource[]
  space_url?: string
  disclosure: string
  render_mode?: 'slide' | 'poster'
  template_id?: 'cover' | 'split' | 'text_only' | 'poster'
  style_preset?: string
}

export interface MediaFallbackSource {
  provider: string
  title?: string
  page_url?: string
  asset_url?: string
  thumbnail_url?: string
  creator?: string
  license?: string
  license_url?: string
  note?: string
  verified_license?: boolean
}

// IR-based intent classification
export interface MediaIntent {
  category: MediaCategory
  confidence: number
  prompt: string
  has_image: boolean
  image_count: number
  alternative_category?: MediaCategory // e.g. kf2v when 2 images could be i2v or kf2v
  params?: Record<string, any>
}

export interface ClassifyResponse {
  intent: MediaIntent | null
  models?: MediaModelInfo[]
}

export interface DirectGenerateRequest {
  category: MediaCategory
  prompt: string
  model?: string
  params?: Record<string, any>
  reference_images?: string[]
  conversation_id?: string
  message_id?: string
  source?: string
}

export interface DirectGenerateResponse {
  task_id: string
  message_id: string
  status: string
  category: string
  model: string
}

// SSE progress event
export interface MediaProgressEvent {
  id: string
  status: TaskStatus
  progress: number
  type?: MediaType
  response?: MediaResponse
  error?: string
  fallback_info?: MediaFallbackInfo
}

// API functions

export async function generateImage(req: MediaRequest) {
  const { data } = await api.post<MediaResponse>('/media/images/generations', req)
  return data
}

export async function generateVideo(req: MediaRequest) {
  const { data } = await api.post<{
    task_id: string
    status: string
    type: string
    progress: number
    message: string
  }>('/media/videos/generations', req)
  return data
}

export async function getTask(taskId: string) {
  const { data } = await api.get<MediaTask>(`/media/tasks/${taskId}`)
  return data
}

export async function listModels(type?: MediaType) {
  const params = type ? { type } : {}
  const { data } = await api.get<{ data: MediaModelInfo[] }>('/media/models', { params })
  return data.data
}

// IR-based API functions

export async function classifyIntent(
  message: string,
  hasImages = false,
  imageCount = 0,
  locale = ''
) {
  const { data } = await api.post<ClassifyResponse>('/media/classify', {
    message,
    has_images: hasImages,
    image_count: imageCount,
    locale,
  })
  return data
}

export async function directGenerate(req: DirectGenerateRequest) {
  const { data } = await api.post<DirectGenerateResponse>('/media/generate', req)
  return data
}

export async function retryTask(taskId: string) {
  const { data } = await api.post<{ task_id: string; message_id: string; status: string }>(
    `/media/tasks/${taskId}/retry`
  )
  return data
}

export async function cancelTask(taskId: string) {
  const { data } = await api.post<{ success: boolean; task_id: string; status: string }>(
    `/media/tasks/${taskId}/cancel`
  )
  return data
}

export async function getTasksByMessage(messageId: string) {
  const { data } = await api.get<{ tasks: MediaTask[] }>(`/media/tasks/by-message/${messageId}`)
  return data.tasks
}

/**
 * Stream media generation progress via SSE.
 * Uses fetch (not EventSource) for auth header support.
 */
export function streamMediaTask(
  taskId: string,
  callbacks: {
    onProgress?: (evt: MediaProgressEvent) => void
    onComplete?: (evt: MediaProgressEvent) => void
    onCancelled?: (evt: MediaProgressEvent) => void
    onError?: (error: string) => void
  }
): AbortController {
  const controller = new AbortController()

  ;(async () => {
    const token = localStorage.getItem('token')
    const headers: Record<string, string> = { Accept: 'text/event-stream' }
    if (token) headers['Authorization'] = `Bearer ${token}`

    try {
      let response = await fetch(`/api/v1/media/tasks/${taskId}/stream`, {
        headers,
        signal: controller.signal,
      })

      if (response.status === 401) {
        const newToken = await ensureFreshToken()
        if (newToken) {
          headers['Authorization'] = `Bearer ${newToken}`
          response = await fetch(`/api/v1/media/tasks/${taskId}/stream`, {
            headers,
            signal: controller.signal,
          })
        }
      }

      if (!response.ok || !response.body) {
        callbacks.onError?.(`HTTP ${response.status}`)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let receivedTerminal = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        let currentEvent = ''
        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim()
          } else if (line.startsWith('data: ')) {
            const raw = line.slice(6).trim()
            if (raw === '[DONE]') {
              receivedTerminal = true
              return
            }

            try {
              const data = JSON.parse(raw) as MediaProgressEvent
              switch (currentEvent) {
                case 'progress':
                  callbacks.onProgress?.(data)
                  break
                case 'complete':
                  receivedTerminal = true
                  callbacks.onComplete?.(data)
                  return
                case 'cancelled':
                  receivedTerminal = true
                  callbacks.onCancelled?.(data)
                  return
                case 'error':
                  receivedTerminal = true
                  callbacks.onError?.(data.error || 'Generation failed')
                  return
              }
            } catch {
              // ignore parse errors
            }
            currentEvent = ''
          }
        }
      }

      // Stream ended without a terminal event — connection dropped
      if (!receivedTerminal) {
        callbacks.onError?.('stream_disconnected')
      }
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') return
      callbacks.onError?.(err instanceof Error ? err.message : String(err))
    }
  })()

  return controller
}

/**
 * Stream image generation with inline SSE (POST with ?stream=true).
 */
export function streamImageGeneration(
  req: MediaRequest,
  callbacks: {
    onStarted?: (taskId: string) => void
    onProgress?: (evt: MediaProgressEvent) => void
    onComplete?: (evt: MediaProgressEvent) => void
    onError?: (error: string) => void
  }
): AbortController {
  const controller = new AbortController()

  ;(async () => {
    const token = localStorage.getItem('token')
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    }
    if (token) headers['Authorization'] = `Bearer ${token}`

    try {
      let response = await fetch('/api/v1/media/images/generations?stream=true', {
        method: 'POST',
        headers,
        body: JSON.stringify(req),
        signal: controller.signal,
      })

      if (response.status === 401) {
        const newToken = await ensureFreshToken()
        if (newToken) {
          headers['Authorization'] = `Bearer ${newToken}`
          response = await fetch('/api/v1/media/images/generations?stream=true', {
            method: 'POST',
            headers,
            body: JSON.stringify(req),
            signal: controller.signal,
          })
        }
      }

      if (!response.ok || !response.body) {
        callbacks.onError?.(`HTTP ${response.status}`)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        let currentEvent = ''
        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim()
          } else if (line.startsWith('data: ')) {
            const raw = line.slice(6).trim()
            if (raw === '[DONE]') return

            try {
              const data = JSON.parse(raw)
              switch (currentEvent) {
                case 'started':
                  callbacks.onStarted?.(data.task_id)
                  break
                case 'progress':
                  callbacks.onProgress?.(data)
                  break
                case 'complete':
                  callbacks.onComplete?.(data)
                  return
                case 'error':
                  callbacks.onError?.(data.error || 'Generation failed')
                  return
              }
            } catch {
              // ignore
            }
            currentEvent = ''
          }
        }
      }
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') return
      callbacks.onError?.(err instanceof Error ? err.message : String(err))
    }
  })()

  return controller
}
