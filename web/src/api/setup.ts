import api from './client'

export interface ConsentStatus {
  consented: boolean
  consented_at?: string
  can_revoke: boolean
}

export interface UsageStats {
  total_calls: number
  calls_by_provider: Record<string, number>
  calls_by_model: Record<string, number>
  input_tokens: number
  output_tokens: number
  error_count: number
  estimated_cost_usd: number
  period_start: string
  period_end: string
}

// Statistics API
export const statisticsApi = {
  // Get usage statistics
  getStats: () => api.get<UsageStats>('/stats'),

  // Get recent events
  getRecentEvents: (limit?: number) =>
    api.get<{ events: unknown[] }>('/stats/recent', { params: { limit } }),

  // Get consent status
  getConsentStatus: () => api.get<ConsentStatus>('/stats/consent'),

  // Set consent status
  setConsentStatus: (consented: boolean) =>
    api.post<{ success: boolean }>('/stats/consent', { consented }),

  // Get consent information
  getConsentInfo: () =>
    api.get<{
      what_we_collect: string[]
      what_we_dont_collect: string[]
      how_we_use: string[]
      data_retention: string
    }>('/stats/consent/info'),

  // Export statistics
  exportStats: (format: 'json' | 'csv' = 'json') =>
    api.get('/stats/export', { params: { format }, responseType: 'blob' }),

  // Clear statistics
  clearStats: () => api.delete('/stats'),
}

export default {
  statistics: statisticsApi,
}
