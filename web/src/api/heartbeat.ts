import api from './client'

export interface HeartbeatStatus {
  enabled: boolean
  interval: string
  next_due?: string
  last_event?: {
    timestamp: string
    status: string
    reason?: string
    duration_ms?: number
    indicator_type?: string
  }
}

export const heartbeatApi = {
  getStatus: () => api.get<HeartbeatStatus>('/heartbeat/status', { baseURL: '/api' }),
  trigger: () => api.post<{ status: string }>('/heartbeat/trigger', null, { baseURL: '/api' }),
}
