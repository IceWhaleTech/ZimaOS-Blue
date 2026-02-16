import api from './client'

export interface CacheStats {
  enabled: boolean
  entries: number
  max_entries: number
  hits: number
  misses: number
  evictions?: number
  bypasses: number
  hit_rate: number
  ttl_seconds?: number
}

export interface CacheConfig {
  enabled: boolean
  max_size: number
  max_entry_size: number
  ttl: number
  skip_streaming: boolean
  storage_type: string
}

export interface CacheConfigUpdate {
  enabled?: boolean
  max_size?: number
  max_entry_size?: number
  ttl_seconds?: number
  skip_streaming?: boolean
}

export interface PrunerStats {
  enabled: boolean
  stats?: {
    total_requests: number
    pruned_requests: number
    passthrough_requests: number
    total_tokens_before: number
    total_tokens_after: number
    tokens_saved: number
    avg_compression_rate: number
    avg_latency_ms: number
  }
}

export interface PrunerConfig {
  enabled: boolean
  backend: string
  threshold: number
  min_lines: number
  timeout_ms: number
}

export interface PrunerConfigUpdate {
  enabled?: boolean
  threshold?: number
  backend?: string
}

export interface PrunerModelStatus {
  ready: boolean
  downloading: boolean
  progress?: {
    file: string
    file_index: number
    total_files: number
    downloaded: number
    total: number
    percentage: number
    speed_human: string
    eta: string
  }
  files?: {
    filename: string
    downloaded: boolean
    size: string
  }[]
}

export interface RoutingRule {
  name: string
  priority: number
  condition: {
    header?: string
    header_value?: string
    max_body_bytes?: number
    tool_pattern?: string
    system_tag?: string
  }
  target_model: string
  origin: string
  tier?: string
  fallback?: string
  enabled?: boolean
}

// Proxy cache API (for /v1/* OpenAI-compatible endpoints)
// All chat requests now route through the proxy, so this is the unified cache
export const proxyCacheApi = {
  getStats: () => api.get<CacheStats>('/proxy/cache/stats'),
  getConfig: () => api.get<CacheConfig>('/proxy/cache/config'),
  updateConfig: (config: CacheConfigUpdate) =>
    api.put<{ success: boolean; config: CacheConfig }>('/proxy/cache/config', config),
  clearCache: () => api.post<{ success: boolean; message: string }>('/proxy/cache/clear'),
  deleteEntry: (key: string) =>
    api.delete<{ success: boolean }>(`/proxy/cache/entry/${encodeURIComponent(key)}`),
  getPrunerStats: () => api.get<PrunerStats>('/proxy/pruner/stats'),
  getPrunerConfig: () => api.get<PrunerConfig>('/proxy/pruner/config'),
  updatePrunerConfig: (config: PrunerConfigUpdate) =>
    api.put<{ success: boolean; config: PrunerConfig }>('/proxy/pruner/config', config),
  getPrunerModelStatus: () => api.get<PrunerModelStatus>('/proxy/pruner/model/status'),
  downloadPrunerModel: () => api.post<{ success: boolean }>('/proxy/pruner/model/download'),
  cancelPrunerModelDownload: () => api.post<{ success: boolean }>('/proxy/pruner/model/cancel'),
  getRoutingConfig: () => api.get<{ enabled: boolean }>('/proxy/routing/config'),
  updateRoutingConfig: (config: { enabled: boolean }) =>
    api.put<{ success: boolean; enabled: boolean }>('/proxy/routing/config', config),
  getRoutingRules: () => api.get<{ rules: RoutingRule[] }>('/proxy/routing/rules'),
  updateRoutingRule: (name: string, config: { enabled: boolean }) =>
    api.put<{ success: boolean; name: string; enabled: boolean }>(`/proxy/routing/rules/${encodeURIComponent(name)}`, config),
}
