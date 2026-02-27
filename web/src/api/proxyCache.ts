import api from './client'

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
  state?: string
  error?: string
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

export interface RoutingStats {
  routed_requests: number
  tokens_routed: number
  cost_saved_usd: number
}

export interface PromptCacheStats {
  requests: number
  cache_hits: number
  cache_misses: number
  total_cache_read_tokens: number
  total_cache_creation_tokens: number
  total_input_tokens: number
  hit_rate: number
  reuse_ratio: number
}

export interface MemoryRecallSourceStats {
  total: number
  recalled: number
  skipped: number
  recall_rate: number
  injected_contexts: number
  injected_tokens: number
  avg_injected_tokens: number
  estimated_saved_tokens: number
  reason_counts: Record<string, number>
}

export interface MemoryRecallStats {
  total: number
  recalled: number
  skipped: number
  recall_rate: number
  injected_contexts: number
  injected_tokens: number
  avg_injected_tokens: number
  estimated_saved_tokens: number
  reason_counts: Record<string, number>
  by_source: Record<string, MemoryRecallSourceStats>
}

export interface ContextStats {
  memory_recall_mode: 'aggressive' | 'balanced' | 'quality'
  memory_recall_min_score: number
  memory_recall_limits: {
    max_results: number
    chunk_runes: number
    total_runes: number
  }
  memory_recall: MemoryRecallStats
}

// Proxy API (pruner, routing, prompt cache)
export const proxyCacheApi = {
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
  getRoutingStats: () => api.get<RoutingStats>('/proxy/routing/stats'),
  getPromptCacheConfig: () => api.get<{ enabled: boolean }>('/proxy/prompt-cache/config'),
  getPromptCacheStats: () => api.get<PromptCacheStats>('/proxy/prompt-cache/stats'),
  updatePromptCacheConfig: (config: { enabled: boolean }) =>
    api.put<{ success: boolean; enabled: boolean }>('/proxy/prompt-cache/config', config),
  getContextStats: () => api.get<ContextStats>('/context/stats'),
  resetContextStats: () => api.post<{ success: boolean }>('/context/stats/reset'),
}
