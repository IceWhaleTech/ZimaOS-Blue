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
}
