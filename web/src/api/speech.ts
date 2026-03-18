import type { AxiosProgressEvent } from 'axios'
import api from './index'
import { authFetch } from './client'
import { isTtsSpeechMuted } from '../utils/ttsPreferences'

// ASR Model types
export interface ASRModel {
  id: string
  name: string
  description: string
  languages: string[]
  size: string
  streaming: boolean
  downloaded: boolean
  active: boolean
  permission_denied?: boolean
  recommended?: boolean
}

// TTS Model types
export interface TTSModel {
  id: string
  name: string
  description: string
  languages: string[]
  size: string
  downloaded: boolean
}

export interface DownloadProgress {
  file: string
  downloaded: number
  total: number
  percentage: number
  speed_human: string
  eta: string
}

export interface TranscriptionResult {
  text: string
  language?: string
  duration?: number
  confidence?: number
  editable: boolean
  session_id?: string
}

export interface ComponentDownloadStatus {
  ready: boolean
  downloading: boolean
  progress: number
  error?: string
  init_stage?: string
  speed?: string
  eta?: string
  file?: string
  file_index?: number
  total_files?: number
  downloaded_human?: string
}

export interface SpeechStatus {
  tts: {
    ready: boolean
    provider: string
    model_name?: string
    downloading?: boolean
    progress?: DownloadProgress
    available_providers?: string[]
    models: TTSModel[]
    components?: Record<string, ComponentDownloadStatus>
  }
  asr: {
    ready: boolean
    provider: string
    model_name?: string
    streaming_supported: boolean
    edit_before_send: boolean
    downloading: boolean
    progress?: DownloadProgress
    has_pending: boolean
    permission_denied?: boolean
    permission_error?: string
    permission_app_name?: string
    downloads?: { model_type: string; progress: DownloadProgress }[]
    on_device_supported?: boolean
    on_device_only?: boolean
    dictation_available?: boolean
    offline_languages?: string[]
    models: ASRModel[]
  }
  espeak?: {
    installed: boolean
    path?: string
    language_count: number
    data_size: number
    static_linked: boolean
  }
}

export interface TranscribeOptions {
  onUploadProgress?: (progress: number, event: AxiosProgressEvent) => void
}

// Speech API
export const speechApi = {
  // Unified status (includes models in asr.models / tts.models)
  getStatus: () => api.get<SpeechStatus>('/speech/status'),

  // ASR model management
  downloadASRModel: (modelType: string) =>
    api.post<{ status: string; message: string }>('/speech/asr/download', {
      model_type: modelType,
    }),

  cancelASRDownload: () =>
    api.post<{ status: string; message: string }>('/speech/asr/download/cancel'),

  switchASRModel: (modelType: string) =>
    api.post<{ status: string; message: string }>('/speech/asr/switch', { model_type: modelType }),

  setASROnDevice: (onDeviceOnly: boolean) =>
    api.post<{
      on_device_only: boolean
      on_device_supported: boolean
      dictation_available?: boolean
      error?: string
    }>('/speech/asr/on-device', { on_device_only: onDeviceOnly }),

  setEditBeforeSend: (enabled: boolean) =>
    api.post<{ edit_before_send: boolean }>('/speech/asr/edit-before-send', { enabled }),

  getOfflineLanguages: () =>
    api.get<{ offline_languages: string[] }>('/speech/asr/offline-languages'),

  // TTS provider management
  switchTTSProvider: (provider: string) =>
    api.post<{ status: string; message: string; provider: string }>('/speech/tts/provider', {
      provider,
    }),

  getTTSConfig: () =>
    api.get<{ speed: number; pitch: number; volume: number }>('/speech/tts/config'),

  setTTSConfig: (speed: number, pitch: number, volume: number) =>
    api.post<{ status: string; speed: number; pitch: number; volume: number }>(
      '/speech/tts/config',
      { speed, pitch, volume }
    ),

  // Transcription
  transcribe: async (
    audio: Blob,
    format: string,
    language?: string,
    options?: TranscribeOptions
  ): Promise<TranscriptionResult> => {
    const formData = new FormData()
    formData.append('audio', audio, `audio.${format}`)
    formData.append('format', format)
    if (language) {
      formData.append('language', language)
    }

    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 20000)

    try {
      const response = await api.post<TranscriptionResult>('/speech/transcribe', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 0,
        signal: controller.signal,
        onUploadProgress: (event) => {
          const total = event.total || audio.size || 0
          const loaded = event.loaded || 0
          const progress = total > 0 ? Math.min(1, loaded / total) : 0
          options?.onUploadProgress?.(progress, event)
        },
      })
      options?.onUploadProgress?.(1, { loaded: audio.size, total: audio.size } as AxiosProgressEvent)
      return response.data
    } catch (e: any) {
      if (e.name === 'AbortError') {
        const err = new Error('Transcription timed out') as any
        err.error_code = 'timeout'
        throw err
      }
      const errorCode = e?.response?.data?.error_code
      const message = e?.response?.data?.error || e?.response?.data?.message
      if (message || errorCode) {
        const err = new Error(message || 'Transcription failed') as any
        err.error_code = errorCode
        err.response = e.response
        throw err
      }
      throw e
    } finally {
      clearTimeout(timer)
    }
  },

  // Kokoro model management (status via unified /speech/status components)
  downloadKokoro: () =>
    api.post<{ status: string; message: string }>('/speech/tts/kokoro/download'),

  cancelKokoroDownload: () =>
    api.post<{ status: string; message: string }>('/speech/tts/kokoro/download/cancel'),

  // Vocoder (HiFi-GAN) management (status via unified /speech/status components)
  downloadVocoder: () =>
    api.post<{ status: string; message: string }>('/speech/tts/vocoder/download'),

  cancelVocoderDownload: () =>
    api.post<{ status: string; message: string }>('/speech/tts/vocoder/download/cancel'),

  // TTS synthesis
  synthesize: async (text: string): Promise<{ audio: string; content_type: string }> => {
    if (isTtsSpeechMuted()) {
      return { audio: '', content_type: 'audio/mp3' }
    }
    const response = await authFetch('/api/v1/voice/synthesize', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify({ text }),
    })
    if (!response.ok) {
      throw new Error('TTS synthesis failed')
    }
    return response.json()
  },
}

export default speechApi
