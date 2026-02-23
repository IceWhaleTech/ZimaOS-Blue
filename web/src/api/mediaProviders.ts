import api from './client'

// MediaProviderConfig matches the backend MediaProviderConfig struct
export interface MediaProviderConfig {
  id: string
  name: string
  enabled: boolean
  base_url?: string
  has_api_key: boolean
  key_hash?: string
  icon?: string
  description?: string
  website?: string
  api_key_url?: string
  models?: MediaModelInfo[]
}

export interface MediaModelInfo {
  id: string
  name: string
  type: 'image' | 'video' | 'audio'
  provider: string
  max_resolution?: string
  supported_sizes?: string[]
}

export interface MediaTestResult {
  healthy: boolean
  latency_ms?: number
  error?: string
  checked_at?: string
}

export const mediaProviderApi = {
  list: () =>
    api.get<{ providers: MediaProviderConfig[] }>('/media/providers'),

  get: (id: string) =>
    api.get<MediaProviderConfig>(`/media/providers/${id}`),

  update: (id: string, data: { base_url?: string }) =>
    api.put<MediaProviderConfig>(`/media/providers/${id}`, data),

  enable: (id: string) =>
    api.post<{ status: string }>(`/media/providers/${id}/enable`),

  disable: (id: string) =>
    api.post<{ status: string }>(`/media/providers/${id}/disable`),

  setKey: (id: string, key: string) =>
    api.post<MediaProviderConfig>(`/media/providers/${id}/keys`, { key }),

  removeKey: (id: string) =>
    api.delete(`/media/providers/${id}/keys`),

  test: (id: string) =>
    api.post<MediaTestResult>(`/media/providers/${id}/test`),
}
