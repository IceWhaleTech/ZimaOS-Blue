import api from './client'

// Types
export interface ToolParameter {
  name: string
  type: string
  description?: string
  required?: boolean
  default?: unknown
}

export interface Tool {
  id: string
  name: string
  version: string
  description: string
  author?: string
  category?: string
  icon?: string
  tags?: string[]
  enabled: boolean
  builtin: boolean
  parameters?: ToolParameter[]
}

export interface ToolSource {
  id: string
  name: string
  url: string
  type: 'registry' | 'github' | 'custom'
  description?: string
  enabled: boolean
}

export interface RemoteTool {
  id: string
  name: string
  version: string
  description: string
  author?: string
  category?: string
  tags?: string[]
  source_id: string
  source_name: string
  download_url?: string
  homepage?: string
  stars?: number
  downloads?: number
  installed: boolean
}

export interface ToolStoreItem {
  id: string
  name: string
  version: string
  description: string
  author?: string
  category?: string
  tags?: string[]
}

export interface BrowseParams {
  source?: string
  category?: string
  search?: string
}

// Tool API
export const toolApi = {
  // Local tools
  list: () => api.get<Tool[]>('/tools'),

  get: (id: string) => api.get<Tool>(`/tools/${id}`),

  enable: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/tools/${id}/enable`),

  disable: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/tools/${id}/disable`),

  // Tool store
  listStore: () => api.get<ToolStoreItem[]>('/tool-store/browse'),

  listSources: () => api.get<ToolSource[]>('/tool-store/sources'),

  addSource: (source: Omit<ToolSource, 'enabled'> & { enabled?: boolean }) =>
    api.post<{ success: boolean; message: string }>('/tool-store/sources', source),

  removeSource: (id: string) =>
    api.delete<{ success: boolean; message: string }>(`/tool-store/sources/${id}`),

  browse: (params?: BrowseParams) =>
    api.get<RemoteTool[]>('/tool-store/browse', { params }),

  install: (id: string) =>
    api.post<{ success: boolean; message: string; tool?: RemoteTool }>(`/tool-store/install/${id}`),

  uninstall: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/tool-store/uninstall/${id}`),

  refresh: () =>
    api.post<{ success: boolean; tools_count: number; errors?: string[] }>('/tool-store/refresh'),
}
