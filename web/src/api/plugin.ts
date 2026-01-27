import api from './client'

// Types
export interface Plugin {
  id: string
  name: string
  version: string
  description: string
  author?: string
  type: 'native' | 'js' | 'wasm'
  enabled: boolean
  status: 'loaded' | 'running' | 'stopped' | 'error'
  config_schema?: PluginConfigSchema
  config?: Record<string, unknown>
  error?: string
  loaded_at?: string
  capabilities?: string[]
}

export interface PluginConfigSchema {
  type: 'object'
  properties: Record<string, PluginConfigProperty>
  required?: string[]
}

export interface PluginConfigProperty {
  type: 'string' | 'number' | 'boolean' | 'array' | 'object'
  title?: string
  description?: string
  default?: unknown
  enum?: unknown[]
  minimum?: number
  maximum?: number
  items?: PluginConfigProperty
}

export interface PluginLog {
  timestamp: string
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  plugin_id: string
}

export interface PluginSource {
  id: string
  name: string
  url: string
  type: 'registry' | 'github' | 'custom'
  description?: string
  enabled: boolean
}

export interface RemotePlugin {
  id: string
  name: string
  version: string
  description: string
  author?: string
  type: 'native' | 'js' | 'wasm'
  capabilities?: string[]
  source_id: string
  source_name: string
  download_url?: string
  homepage?: string
  stars?: number
  downloads?: number
  installed: boolean
}

export interface BrowseParams {
  source?: string
  type?: string
  search?: string
}

// Plugin API
export const pluginApi = {
  list: () => api.get<Plugin[]>('/plugins'),

  get: (id: string) => api.get<Plugin>(`/plugins/${id}`),

  enable: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/plugins/${id}/enable`),

  disable: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/plugins/${id}/disable`),

  updateConfig: (id: string, config: Record<string, unknown>) =>
    api.put<{ success: boolean; message: string }>(`/plugins/${id}/config`, config),

  getLogs: (id: string, params?: { limit?: number; level?: string }) =>
    api.get<PluginLog[]>(`/plugins/${id}/logs`, { params }),

  reload: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/plugins/${id}/reload`),

  // Plugin store
  listSources: () => api.get<PluginSource[]>('/plugin-store/sources'),

  addSource: (source: Omit<PluginSource, 'enabled'> & { enabled?: boolean }) =>
    api.post<{ success: boolean; message: string }>('/plugin-store/sources', source),

  removeSource: (id: string) =>
    api.delete<{ success: boolean; message: string }>(`/plugin-store/sources/${id}`),

  browse: (params?: BrowseParams) =>
    api.get<RemotePlugin[]>('/plugin-store/browse', { params }),

  install: (id: string) =>
    api.post<{ success: boolean; message: string; plugin?: RemotePlugin }>(`/plugin-store/install/${id}`),

  uninstall: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/plugin-store/uninstall/${id}`),

  refresh: () =>
    api.post<{ success: boolean; plugins_count: number; errors?: string[] }>('/plugin-store/refresh'),
}
