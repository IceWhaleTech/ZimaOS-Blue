import api from './index'
import { authFetch } from './client'

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

export interface ASRStatus {
  ready: boolean
  model_dir: string
  model_type: string
  streaming_supported: boolean
  downloading: boolean
  progress?: DownloadProgress
  has_pending: boolean
  pending_model?: string
  saved_progress?: DownloadProgress
}

export interface TTSStatus {
  ready: boolean
  model_dir: string
  model_type: string
  downloading: boolean
  progress?: DownloadProgress
  has_pending: boolean
  pending_model?: string
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

export interface SpeechStatus {
  tts: {
    ready: boolean
    provider: string
    model_type?: string
    downloading?: boolean
    progress?: DownloadProgress
  }
  asr: {
    ready: boolean
    provider: string
    model_type?: string
    streaming_supported: boolean
    edit_before_send: boolean
    downloading: boolean
    progress?: DownloadProgress
    has_pending: boolean
  }
}

export interface SpeechModels {
  tts: ModelInfo[]
  asr: ModelInfo[]
}

export interface ModelInfo {
  id: string
  name: string
  description: string
  type: 'tts' | 'asr'
  languages?: string[]
  size: string
  streaming?: boolean
  downloaded: boolean
}

// Speech API
export const speechApi = {
  // Unified status
  getStatus: () => api.get<SpeechStatus>('/speech/status'),

  // Unified models
  getModels: () => api.get<SpeechModels>('/speech/models'),

  // ASR model management
  getASRStatus: () => api.get<ASRStatus>('/speech/asr/status'),

  listASRModels: () => api.get<{ models: ASRModel[] }>('/speech/asr/models'),

  downloadASRModel: (modelType: string) =>
    api.post<{ status: string; message: string }>('/speech/asr/download', { model_type: modelType }),

  cancelASRDownload: () =>
    api.post<{ status: string; message: string }>('/speech/asr/download/cancel'),

  switchASRModel: (modelType: string) =>
    api.post<{ status: string; message: string }>('/speech/asr/switch', { model_type: modelType }),

  deleteASRModel: (modelType?: string) =>
    api.delete<{ status: string; message: string }>(`/speech/asr/model${modelType ? `?model_type=${modelType}` : ''}`),

  // TTS model management (Sherpa) - unified under /speech/tts/*
  getTTSStatus: () => api.get<TTSStatus>('/speech/tts/status'),

  listTTSModels: () => api.get<{ models: TTSModel[] }>('/speech/tts/models'),

  downloadTTSModel: (modelType: string) =>
    api.post<{ status: string; message: string }>('/speech/tts/download', { model_type: modelType }),

  switchTTSModel: (modelType: string) =>
    api.post<{ status: string; message: string }>('/speech/tts/switch', { model_type: modelType }),

  deleteTTSModel: (modelType?: string) =>
    api.delete<{ status: string; message: string }>(`/speech/tts/model${modelType ? `?model_type=${modelType}` : ''}`),

  switchTTSProvider: (provider: string) =>
    api.post<{ status: string; message: string; provider: string }>('/speech/tts/provider', { provider }),

  getTTSConfig: () =>
    api.get<{ speed: number; pitch: number; volume: number }>('/speech/tts/config'),

  setTTSConfig: (speed: number, pitch: number, volume: number) =>
    api.post<{ status: string; speed: number; pitch: number; volume: number }>('/speech/tts/config', { speed, pitch, volume }),

  // Transcription with edit support
  transcribe: async (audio: Blob, format: string, language?: string): Promise<TranscriptionResult> => {
    const formData = new FormData()
    formData.append('audio', audio, `audio.${format}`)
    formData.append('format', format)
    if (language) {
      formData.append('language', language)
    }

    const response = await authFetch('/api/v1/speech/transcribe', {
      method: 'POST',
      body: formData,
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.error || 'Transcription failed')
    }

    return response.json()
  },

  // Confirm edited transcription
  confirmTranscription: (sessionId: string, text: string) =>
    api.post<{ status: string; text: string }>('/speech/confirm', {
      session_id: sessionId,
      text,
    }),

  // eSpeak-NG language pack management
  listEspeakLanguages: () =>
    api.get<{ languages: Array<{code: string; name: string; downloaded: boolean; size: string}> }>('/speech/espeak/languages'),

  downloadEspeakLanguage: (langCode: string) =>
    api.post<{ status: string; message: string }>('/speech/espeak/download', { lang_code: langCode }),

  downloadAllEspeakLanguages: () =>
    api.post<{ status: string; message: string }>('/speech/espeak/download-all'),

  deleteEspeakLanguage: (langCode: string) =>
    api.delete<{ status: string; message: string }>(`/speech/espeak/language?lang_code=${langCode}`),

  // TTS synthesis
  synthesize: async (text: string): Promise<{ audio: string; content_type: string }> => {
    const response = await authFetch('/api/v1/voice/synthesize', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
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
