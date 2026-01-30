import { apiClient } from './client'

// Types
export interface Session {
  id: string
  request_id?: string
  created_at: string
  start_time: string
  last_activity: string
  provider: string
  model: string
  tokens_in: number
  tokens_out: number
  input_tokens?: number
  output_tokens?: number
  duration_ms?: number
  status: 'active' | 'completed' | 'failed' | 'timeout' | 'error'
  request_count: number
  error_count: number
  client_ip: string
  user_agent: string
  streaming?: boolean
  error?: string
  messages?: Array<{ role: string; content: string }>
}

export interface SessionStats {
  enabled: boolean
  total_sessions: number
  active_sessions: number
  completed: number
  failed: number
  total_tokens_in: number
  total_tokens_out: number
  total_requests: number
  avg_duration_ms?: number
}

export interface LatencyStats {
  avg_ms?: number
  min_ms?: number
  max_ms?: number
  p50_ms?: number
  p90_ms?: number
  p95_ms?: number
  p99_ms?: number
  sample_count?: number
}

export interface MetricsSummary {
  enabled: boolean
  total_requests: number
  successful_requests: number
  total_success: number
  total_errors: number
  success_rate: number
  avg_latency_ms: number
  avg_ttft_ms: number
  avg_proxy_overhead: number
  p95_latency_ms: number
  p99_latency_ms: number
  p95_ttft_ms: number
  p99_ttft_ms: number
  tokens_per_sec: number
  total_tokens_in: number
  total_tokens_out: number
  total_input_tokens?: number
  total_output_tokens?: number
  total_bytes: number
  provider_count: number
  bucket_count: number
}

export interface ProviderMetrics {
  name: string
  request_count: number
  success_count: number
  error_count: number
  success_rate: number
  total_latency_ms: number
  avg_latency_ms: number
  total_ttft_ms: number
  avg_ttft_ms: number
  total_tokens_in: number
  total_tokens_out: number
  total_input_tokens?: number
  total_output_tokens?: number
  tokens_per_sec: number
}

export interface GuardStats {
  enabled: boolean
  block_on_detect: boolean
  pattern_count: number
  whitelist_count: number
  detection_count: number
  blocked_count: number
  max_prompt_length: number
}

export interface GuardRule {
  id: string
  name: string
  pattern: string
  description?: string
  enabled: boolean
  risk_level: 'low' | 'medium' | 'high'
  action: 'log' | 'warn' | 'block'
}

export interface AuthStats {
  auth_enabled: boolean
  auth_type: string
  api_key_count: number
  allowed_ip_count: number
  auth_failures: number
  rate_limit_enabled: boolean
  requests_per_min: number
  burst_size: number
  rate_limit_hits: number
  active_limiters: number
}

export interface APIKey {
  id: number
  key: string
  active: boolean
}

export interface SecurityAlert {
  id: string
  type: 'injection' | 'rate_limit' | 'auth_failure' | 'anomaly'
  severity: 'low' | 'medium' | 'high' | 'critical'
  message: string
  details?: string
  timestamp: string
  source_ip?: string
  resolved: boolean
}

export interface MaskingRule {
  id: string
  name: string
  category: 'pii' | 'credentials' | 'financial' | 'custom'
  pattern: string
  replacement: string
  direction: 'request' | 'response' | 'both'
  enabled: boolean
}

export interface MaskingStats {
  enabled: boolean
  rule_count: number
  total_masks: number
  mask_counts: Record<string, number>
  status: string
}

// API functions
export const proxyApi = {
  // Sessions
  getSessions: (activeOnly = false) =>
    apiClient.get<{ sessions: Session[]; stats: SessionStats }>(
      `/api/v1/proxy/sessions${activeOnly ? '?active=true' : ''}`
    ),

  getSession: (id: string) => apiClient.get<Session>(`/api/v1/proxy/sessions/${id}`),

  // Metrics
  getMetrics: () => apiClient.get<MetricsSummary>('/api/v1/proxy/metrics'),

  getProviderMetrics: (provider?: string) =>
    apiClient.get<{ providers: Record<string, ProviderMetrics> }>(
      `/api/v1/proxy/metrics/providers${provider ? `?provider=${provider}` : ''}`
    ),

  getLatencyStats: () => apiClient.get<LatencyStats>('/api/v1/proxy/metrics/latency'),

  getTimeSeries: (start?: string, end?: string) => {
    const params = new URLSearchParams()
    if (start) params.set('start', start)
    if (end) params.set('end', end)
    return apiClient.get(`/api/v1/proxy/metrics/timeseries?${params.toString()}`)
  },

  // Guard
  getGuardStats: () => apiClient.get<GuardStats>('/api/v1/proxy/guard/stats'),

  getGuardRules: () => apiClient.get<{ rules: GuardRule[] }>('/api/v1/proxy/guard/rules'),

  addGuardRule: (rule: Omit<GuardRule, 'id'>) =>
    apiClient.post<{ message: string; rule: GuardRule }>('/api/v1/proxy/guard/rules', rule),

  // Auth
  getAuthStats: () => apiClient.get<AuthStats>('/api/v1/proxy/auth/stats'),

  getAPIKeys: () => apiClient.get<{ keys: APIKey[] }>('/api/v1/proxy/auth/keys'),

  addAPIKey: (key: string) =>
    apiClient.post<{ message: string }>('/api/v1/proxy/auth/keys', { key }),

  removeAPIKey: (key: string) =>
    apiClient.delete<{ message: string }>(`/api/v1/proxy/auth/keys?key=${encodeURIComponent(key)}`),

  // Models
  getModels: () => apiClient.get('/api/v1/proxy/models'),

  getModelFeatures: (model: string) => apiClient.get(`/api/v1/proxy/models?model=${model}`),

  // Mock
  getMockEndpoints: () => apiClient.get('/api/v1/proxy/mock'),

  addMockEndpoint: (endpoint: {
    path: string
    method: string
    response: unknown
    status_code?: number
    enabled?: boolean
  }) => apiClient.post('/api/v1/proxy/mock', endpoint),

  // Config
  reloadConfig: () => apiClient.post<{ message: string; reloaded_at: string }>('/api/v1/proxy/config/reload', {}),

  // Data Masking (reserved for future implementation)
  getMaskingStats: () => apiClient.get<MaskingStats>('/api/v1/proxy/masking/stats'),

  getMaskingRules: () => apiClient.get<{ rules: MaskingRule[]; default_rules: MaskingRule[] }>('/api/v1/proxy/masking/rules'),

  addMaskingRule: (rule: Omit<MaskingRule, 'id'>) =>
    apiClient.post<{ message: string; rule: MaskingRule }>('/api/v1/proxy/masking/rules', rule),

  removeMaskingRule: (id: string) =>
    apiClient.delete<{ message: string }>(`/api/v1/proxy/masking/rules?id=${encodeURIComponent(id)}`),

  // Security Alerts (aggregated from guard stats)
  getSecurityAlerts: async (): Promise<{ data: SecurityAlert[] }> => {
    const [guardStats, authStats] = await Promise.all([
      proxyApi.getGuardStats(),
      proxyApi.getAuthStats(),
    ])

    const alerts: SecurityAlert[] = []
    const now = new Date().toISOString()

    // Generate alerts based on stats
    if (guardStats.data.blocked_count > 0) {
      alerts.push({
        id: 'guard-blocked',
        type: 'injection',
        severity: 'high',
        message: `${guardStats.data.blocked_count} prompt injection attempts blocked`,
        timestamp: now,
        resolved: false,
      })
    }

    if (guardStats.data.detection_count > guardStats.data.blocked_count) {
      const detected = guardStats.data.detection_count - guardStats.data.blocked_count
      alerts.push({
        id: 'guard-detected',
        type: 'injection',
        severity: 'medium',
        message: `${detected} potential prompt injection attempts detected`,
        timestamp: now,
        resolved: false,
      })
    }

    if (authStats.data.auth_failures > 0) {
      alerts.push({
        id: 'auth-failures',
        type: 'auth_failure',
        severity: authStats.data.auth_failures > 10 ? 'high' : 'medium',
        message: `${authStats.data.auth_failures} authentication failures`,
        timestamp: now,
        resolved: false,
      })
    }

    if (authStats.data.rate_limit_hits > 0) {
      alerts.push({
        id: 'rate-limit',
        type: 'rate_limit',
        severity: authStats.data.rate_limit_hits > 100 ? 'high' : 'low',
        message: `${authStats.data.rate_limit_hits} requests rate limited`,
        timestamp: now,
        resolved: false,
      })
    }

    return { data: alerts }
  },
}
