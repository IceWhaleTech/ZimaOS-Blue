import api from './index'

// ASR Model types
export interface ASRModel {
  id: string
  name: string
  description: string
  languages: string[]
  size: string
  streaming: boolean
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

  switchASRModel: (modelType: string) =>
    api.post<{ status: string; message: string }>('/speech/asr/switch', { model_type: modelType }),

  deleteASRModel: (modelType?: string) =>
    api.delete<{ status: string; message: string }>(`/speech/asr/model${modelType ? `?model_type=${modelType}` : ''}`),

  // Transcription with edit support
  transcribe: async (audio: Blob, format: string, language?: string): Promise<TranscriptionResult> => {
    const formData = new FormData()
    formData.append('audio', audio, `audio.${format}`)
    formData.append('format', format)
    if (language) {
      formData.append('language', language)
    }

    const response = await fetch('/api/v1/speech/transcribe', {
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
}

export default speechApi
