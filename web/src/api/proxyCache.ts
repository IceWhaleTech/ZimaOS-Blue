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
  ttl_seconds: number
}

export interface CacheConfigUpdate {
  enabled?: boolean
  max_size?: number
  max_entry_size?: number
  ttl_seconds?: number
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
}
