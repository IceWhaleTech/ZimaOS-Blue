import api from './client'

export interface VoiceWakeStatus {
  supported: boolean
  enabled: boolean
  running: boolean
  platform: string
  target_conversation_id?: string
  triggers?: string[]
  locale?: string
  speech_authorized: boolean
  microphone_ready: boolean
  reason?: string
  last_error?: string
  last_triggered_at?: string
  last_sent_at?: string
}

export const voiceWakeApi = {
  getStatus: () => api.get<VoiceWakeStatus>('/voice-wake/status'),
  restart: () => api.post<VoiceWakeStatus>('/voice-wake/restart'),
}
