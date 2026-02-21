import api from './client'

export interface WorkspaceFile {
  name: string
  content: string
  missing?: boolean
}

export interface FileTokenStat {
  name: string
  bytes: number
  tokens: number
}

export interface WorkspaceStats {
  files: FileTokenStat[]
  total_tokens: number
  total_bytes: number
}

export const workspaceApi = {
  listFiles: () => api.get<{ files: WorkspaceFile[] }>('/workspace/files'),

  getFile: (name: string) => api.get<WorkspaceFile>(`/workspace/files/${name}`),

  putFile: (name: string, content: string) =>
    api.put<{ status: string; name: string; bytes: number }>(`/workspace/files/${name}`, { content }),

  getStats: () => api.get<WorkspaceStats>('/workspace/stats'),
}
