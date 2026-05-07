import api from './client'

export interface ToolCallSummary {
  name: string
  tool_call_id: string
  latency_ms: number
  success: boolean
}

export interface TurnMetrics {
  id: string
  conversation_id: string
  user_id: string
  turn_id: string
  model: string
  status: string
  latency_ms: number
  llm_latency_ms: number
  ttft_ms: number
  input_tokens: number
  output_tokens: number
  cache_read: number
  cache_write: number
  cost_usd: number
  tool_calls: ToolCallSummary[]
  tool_count: number
  tool_latency_ms: number
  llm_request: string
  llm_response: string
  error_type: string
  created_at: string
}

export interface SessionInfo {
  conversation_id: string
  user_id: string
  turn_count: number
  total_cost: number
  total_tokens: number
  first_turn_at: string
  last_turn_at: string
  model: string
}

export interface AggregateStats {
  total_turns: number
  total_cost: number
  total_tokens: number
  avg_latency_ms: number
  p50_latency_ms: number
  p95_latency_ms: number
  error_rate: number
  cache_hit_rate: number
  session_count: number
  total_tool_calls: number
}

export interface ModelBreakdown {
  model: string
  turn_count: number
  total_cost: number
  avg_latency_ms: number
  total_input_tokens: number
  total_output_tokens: number
  error_count: number
}

export interface ToolStats {
  tool_name: string
  call_count: number
  avg_latency_ms: number
  successes: number
  failures: number
}

export interface TimeseriesPoint {
  bucket: string
  turns: number
  cost: number
  avg_latency_ms: number
  tokens: number
}

export const devApi = {
  getSessions: (limit = 50) =>
    api.get<SessionInfo[]>('/dev/sessions', { params: { limit } }),

  getSessionTurns: (convId: string, limit = 100) =>
    api.get<TurnMetrics[]>(`/dev/sessions/${convId}/turns`, { params: { limit } }),

  getTurnDetail: (turnId: string) =>
    api.get<TurnMetrics>(`/dev/turns/${turnId}`),

  getTurnReplay: (turnId: string) =>
    api.get(`/dev/turns/${turnId}/replay`),

  getAggregateStats: () =>
    api.get<AggregateStats>('/dev/stats/aggregate'),

  getModelBreakdown: () =>
    api.get<ModelBreakdown[]>('/dev/stats/models'),

  getToolStats: () =>
    api.get<ToolStats[]>('/dev/stats/tools'),

  getTimeseries: (hours = 24) =>
    api.get<TimeseriesPoint[]>('/dev/stats/timeseries', { params: { hours } }),

  getUnifiedAudit: (convId: string, limit = 100) =>
    api.get(`/dev/audit/${convId}`, { params: { limit } }),
}
