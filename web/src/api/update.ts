import api from './client'

export interface UpdateInfo {
  current_version: string
  latest_version: string
  update_available: boolean
  release_channel: string
  release_notes: string
  download_url: string
  checksum: string
  size: number
  published_at: string
}

export interface UpdateStatus {
  state: 'idle' | 'checking' | 'downloading' | 'applying' | 'failed'
  progress: number
  error?: string
  last_checked: string
  downloaded_path?: string
}

export interface UpdateInfoResponse {
  current_version: string
  status: UpdateStatus
  latest: UpdateInfo | null
  uptime: string
}

export interface OTAStatus {
  current_version: string
  update_available: boolean
  latest_version?: string
  download_urls?: string[]
  release_note_url?: string
  delay?: number
}

const isTauri = typeof window !== 'undefined' && ('__TAURI__' in window || '__TAURI_INTERNALS__' in window)

export const updateApi = {
  check: () => api.get<UpdateInfo>('/system/update/check'),
  info: () => api.get<UpdateInfoResponse>('/system/update/info'),
  download: () => api.post<{ status: string }>('/system/update/download'),
  apply: () => api.post<{ status: string }>('/system/update/apply'),
  rollback: () => api.post<{ status: string }>('/system/update/rollback'),
  history: () => api.get<any[]>('/system/update/history'),
  ota: () => api.get<OTAStatus>('/system/update/ota', { params: isTauri ? { desktop: '1' } : undefined }),
  releaseNotes: (url?: string) => api.get<string>('/system/update/release-notes', { params: url ? { url } : undefined, responseType: 'text' as const }),
}
