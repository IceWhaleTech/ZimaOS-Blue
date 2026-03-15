import api from './client'

// Types
export interface CallStats {
  total_calls: number
  successful_calls: number
  failed_calls: number
  success_rate: number
  error_rate: number
  errors_by_type: Record<string, number>
}

export interface HourlyStats {
  hour: string // ISO timestamp string
  calls: number
  success_rate: number
}

export interface CallStatsResponse {
  period: string
  stats: CallStats
  by_hour: HourlyStats[]
}

export interface ModelStats {
  model: string
  calls: number
  success_rate: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  estimated_cost: number
  avg_latency_ms: number
}

export interface StatsSummary {
  total_calls: number
  total_tokens: number
  total_cost: number
}

export interface ModelStatsResponse {
  period: string
  models: ModelStats[]
  summary: StatsSummary
}

export interface TokenUsage {
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  total_tokens: number
  estimated_cost: number
}

export interface ModelTokenUsage {
  model: string
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  total_tokens: number
  estimated_cost: number
}

export interface TokenUsageResponse {
  period: string
  usage: TokenUsage
  by_model: ModelTokenUsage[]
}

export interface UserTokenUsage {
  user_id: string
  input_tokens: number
  output_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  estimated_cost: number
  request_count: number
}

export interface UserUsageSummary {
  total_users: number
  total_tokens: number
  total_cost: number
  total_requests: number
}

export interface UserTokenUsageResponse {
  period: string
  users: UserTokenUsage[]
  summary: UserUsageSummary
}

export interface LatencyStats {
  min_ms: number
  max_ms: number
  avg_ms: number
  p50_ms: number
  p90_ms: number
  p95_ms: number
  p99_ms: number
  samples: number
}

export interface SpeedStats {
  tokens_per_second: number
  time_to_first_token_ms: number
  decode_speed: number
}

export interface SpeedPercentiles {
  ttft_p50_ms: number
  ttft_p95_ms: number
  ttft_p99_ms: number
  tps_p50: number
  tps_p95: number
  tps_p99: number
}

export interface SpeedResponse {
  current: SpeedStats
  average: SpeedStats
  percentiles: SpeedPercentiles
}

export interface SystemResourceMetrics {
  cpu_count: number
  cpu_usage_percent: number
  load_avg_1: number
  load_avg_5: number
  load_avg_15: number
  memory_total_bytes: number
  memory_used_bytes: number
  memory_free_bytes: number
  memory_percent: number
  disk_total_bytes: number
  disk_used_bytes: number
  disk_free_bytes: number
  disk_percent: number
  network_bytes_sent: number
  network_bytes_recv: number
}

export interface ResourceHistory {
  timestamp: string
  cpu_percent: number
  memory_percent: number
  disk_percent: number
}

export interface ProcessMetrics {
  pid: number
  command: string
  state: string
  cpu_percent: number
  cpu_time_s: number
  memory_rss_bytes: number
  memory_vms_bytes: number
  memory_percent: number
  io_read_bytes: number
  io_write_bytes: number
  num_threads: number
  start_time: string
  uptime: number
}

export interface TokenPricing {
  pattern: string
  input_price: number
  output_price: number
  cache_read: number
  cache_write: number
}

export interface MetricsSummary {
  calls: CallStats
  tokens: TokenUsage
  latency: LatencyStats
  speed: SpeedStats
  system?: SystemResourceMetrics
}

// Aggregated metrics response - all metrics in one call
export interface AggregatedMetrics {
  calls: CallStatsResponse
  models: ModelStatsResponse
  tokens: TokenUsageResponse
  latency: LatencyStats
  speed: SpeedResponse
  system: SystemResourceMetrics | null
  resource_history: ResourceHistory[]
  process: ProcessMetrics | null
  pricing: TokenPricing[]
}

// Metrics API
export const metricsApi = {
  // Summary
  getSummary: () => api.get<MetricsSummary>('/metrics/summary'),

  // Aggregated - all metrics in one call
  getAll: () => api.get<AggregatedMetrics>('/metrics/all'),

  // Call statistics
  getCallStats: (period?: string) =>
    api.get<CallStatsResponse>('/metrics/calls', { params: period ? { period } : undefined }),

  // Model statistics
  getModelStats: (period?: string) =>
    api.get<ModelStatsResponse>('/metrics/models', { params: period ? { period } : undefined }),

  getModelStatsByName: (model: string) => api.get<ModelStats>(`/metrics/models/${model}`),

  // Token usage
  getTokenUsage: (period?: string) =>
    api.get<TokenUsageResponse>('/metrics/tokens', { params: period ? { period } : undefined }),

  // User token usage
  getUserTokenUsage: (period?: string) =>
    api.get<UserTokenUsageResponse>('/metrics/tokens/users', {
      params: period ? { period } : undefined,
    }),

  getUserTokenUsageById: (userId: string) =>
    api.get<UserTokenUsage>(`/metrics/tokens/users/${userId}`),

  // Latency
  getLatencyStats: (model?: string) =>
    api.get<LatencyStats>('/metrics/latency', { params: model ? { model } : undefined }),

  // Speed
  getSpeedStats: (model?: string) =>
    api.get<SpeedResponse>('/metrics/speed', { params: model ? { model } : undefined }),

  getModelSpeedStats: () => api.get<SpeedResponse[]>('/metrics/speed/models'),

  // System metrics
  getSystemMetrics: () => api.get<SystemResourceMetrics>('/metrics/system'),

  getResourceHistory: () => api.get<ResourceHistory[]>('/metrics/system/history'),

  // Process metrics
  getProcessMetrics: () => api.get<ProcessMetrics>('/metrics/process'),

  // Pricing
  getPricing: () => api.get<TokenPricing[]>('/metrics/pricing'),

  getPricingForModel: (model: string) => api.get<TokenPricing>(`/metrics/pricing/${model}`),

  // Admin
  resetMetrics: () => api.post<{ status: string; message: string }>('/metrics/reset'),
}
