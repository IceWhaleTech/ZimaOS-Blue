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

export const updateApi = {
  check: () => api.get<UpdateInfo>('/system/update/check'),
  info: () => api.get<UpdateInfoResponse>('/system/update/info'),
  download: () => api.post<{ status: string }>('/system/update/download'),
  apply: () => api.post<{ status: string }>('/system/update/apply'),
  rollback: () => api.post<{ status: string }>('/system/update/rollback'),
  history: () => api.get<any[]>('/system/update/history'),
}
