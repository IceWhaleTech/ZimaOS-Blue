import api from './client'

// Types
export interface TTSProvider {
  type: string
  name: string
}

export interface TTSVoice {
  id: string
  name: string
  language: string
  gender?: string
}

export interface LanguagePack {
  language: string
  name: string
  size_kb: number
  downloaded: boolean
  downloading?: boolean
}

export interface ConsentResponse {
  service: string
  consent_given: boolean
  message?: string
}

// TTS API
export const ttsApi = {
  // Consent management
  getConsent: (service: string) =>
    api.get<ConsentResponse>('/tts/consent', { params: { service } }),

  setConsent: (service: string, given: boolean) =>
    api.post<ConsentResponse>('/tts/consent', { service, given }),

  // Provider management
  listProviders: () =>
    api.get<{ providers: TTSProvider[]; preferred: string }>('/tts/providers'),

  setPreferredProvider: (provider: string) =>
    api.post<{ message: string; provider: string }>('/tts/providers/preferred', {
      provider,
    }),

  // Voice management
  listVoices: (provider?: string) =>
    api.get<{ provider: string; voices: TTSVoice[] }>('/tts/voices', {
      params: { provider },
    }),

  // eSpeak-NG language packs
  listLanguagePacks: () =>
    api.get<{ packs: LanguagePack[] }>('/tts/espeak/packs'),

  downloadLanguagePack: (language: string) =>
    api.post<{ message: string; language: string }>(`/tts/espeak/packs/${language}`),

  downloadAllLanguagePacks: () =>
    api.post<{ status: string; message: string }>('/speech/espeak/download-all'),

  deleteLanguagePack: (language: string) =>
    api.delete<{ message: string; language: string }>(`/tts/espeak/packs/${language}`),

  // Synthesis
  synthesize: (text: string, provider?: string, voice?: string, rate?: number) =>
    api.post(
      '/tts/synthesize',
      { text, provider, voice, rate },
      { responseType: 'blob' }
    ),
}

export default ttsApi
