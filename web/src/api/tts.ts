import api from './client'

// Types
export interface TTSProvider {
  type: string
  enabled: boolean
  model_ready?: boolean
  model_dir?: string
  native?: boolean // True for sherpa-based providers (no Python)
}

export interface TTSProvidersResponse {
  providers: TTSProvider[]
  default_provider: string
}

export interface TTSVoice {
  id: string
  name: string
  language: string
  gender?: string
  description?: string
  preview_url?: string
}

export interface TTSVoicesResponse {
  voices: TTSVoice[]
}

// Sherpa model types (native Go, no Python)
export interface SherpaDownloadProgress {
  file: string
  downloaded: number
  total: number
  percentage: number
  speed: number
  speed_human: string
  eta: string
  started_at: string
}

export interface SherpaModelStatus {
  ready: boolean
  model_dir: string
  model_type: string
  downloading: boolean
  progress?: SherpaDownloadProgress
  has_pending?: boolean
  pending_model?: string
  saved_progress?: SherpaDownloadProgress
}

export interface SherpaAvailableModel {
  id: string
  name: string
  description: string
  languages: string[]
  size: string
  downloaded: boolean
}

// Legacy types for backward compatibility
export interface KokoroModelFileStatus {
  name: string
  expected: number
  exists: boolean
  size: number
}

export interface KokoroModelStatus {
  ready: boolean
  model_dir: string
  files: KokoroModelFileStatus[]
  downloading: boolean
  progress?: SherpaDownloadProgress
}

export interface KokoroSetupStatus {
  model_status: KokoroModelStatus
  dependencies: Record<string, boolean>
  python_path: string
  ready: boolean
}

export interface KokoroDependenciesResponse {
  dependencies: Record<string, boolean>
  all_installed: boolean
}

// TTS API
export const ttsApi = {
  // List available TTS providers
  listProviders: () => api.get<TTSProvidersResponse>('/tts/providers'),

  // List available voices
  listVoices: () => api.get<TTSVoicesResponse>('/tts/voices'),

  // Sherpa model management (native Go, no Python dependency)
  sherpa: {
    // Get Sherpa model status
    getStatus: () => api.get<SherpaModelStatus>('/tts/sherpa/status'),

    // Start downloading Sherpa model
    downloadModel: (modelType = 'kokoro-en') =>
      api.post<{ status: string; message: string; model_type: string }>('/tts/sherpa/download', {
        model_type: modelType,
      }),

    // Delete Sherpa model
    deleteModel: () => api.delete<{ status: string; message: string }>('/tts/sherpa/model'),

    // Get available models
    getAvailableModels: () => api.get<{ models: SherpaAvailableModel[] }>('/tts/sherpa/models'),

    // Switch to a different model
    switchModel: (modelType: string) =>
      api.post<{ status: string; message: string }>('/tts/sherpa/switch', {
        model_type: modelType,
      }),
  },

  // Legacy Kokoro API (redirects to Sherpa internally)
  kokoro: {
    // Get Kokoro setup status (uses Sherpa backend)
    getStatus: () => api.get<SherpaModelStatus>('/tts/kokoro/status'),

    // Start downloading Kokoro model
    downloadModel: (_preferModelScope = false) =>
      api.post<{ status: string; message: string }>('/tts/kokoro/download', {
        model_type: 'kokoro-en',
      }),

    // Delete Kokoro model
    deleteModel: () => api.delete<{ status: string; message: string }>('/tts/kokoro/model'),

    // Legacy: Check Python dependencies (no longer needed, returns empty)
    checkDependencies: () => Promise.resolve({ data: { dependencies: {}, all_installed: true } }),

    // Legacy: Install Python dependencies (no longer needed, no-op)
    installDependencies: () => Promise.resolve({ data: { status: 'ok', message: 'No dependencies needed' } }),
  },
}

export default ttsApi
