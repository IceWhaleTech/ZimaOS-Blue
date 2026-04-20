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

export interface WorkspaceMeta {
  dir: string
}

export interface WorkspaceTreeEntry {
  path: string
  abs_path: string
  name: string
  type: 'file' | 'dir' | string
  depth: number
  size_bytes?: number
  modified_at?: string
}

export interface WorkspaceTree {
  root: string
  entries: WorkspaceTreeEntry[]
  next_offset?: number
  has_more?: boolean
}

export interface WorkspaceTreeParams {
  max_depth?: number
  include_hidden?: boolean
  root?: string
  limit?: number
  offset?: number
}

export const workspaceApi = {
  getMeta: () => api.get<WorkspaceMeta>('/workspace/meta'),

  getTree: (params?: WorkspaceTreeParams) => api.get<WorkspaceTree>('/workspace/tree', { params }),

  listFiles: () => api.get<{ files: WorkspaceFile[] }>('/workspace/files'),

  getFile: (name: string) => api.get<WorkspaceFile>(`/workspace/files/${name}`),

  putFile: (name: string, content: string) =>
    api.put<{ status: string; name: string; bytes: number }>(`/workspace/files/${name}`, {
      content,
    }),

  getStats: () => api.get<WorkspaceStats>('/workspace/stats'),
}
