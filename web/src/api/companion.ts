import api from './client'

// Types matching backend companion/types.go

export type Platform = 'whatsapp' | 'telegram' | 'discord' | 'slack' | 'matrix' | 'feishu' | 'web' | 'api'
export type SessionStatus = 'active' | 'idle' | 'ended' | 'error'
export type ThreatLevel = 'none' | 'low' | 'medium' | 'high' | 'critical'
export type AlertSeverity = 'info' | 'warning' | 'high' | 'error' | 'critical'

export type SessionEventType =
  | 'session_start'
  | 'session_end'
  | 'message_received'
  | 'message_sent'
  | 'tool_call'
  | 'llm_request'
  | 'security_threat'
  | 'sandbox_exec'
  | 'error'
  | 'custom'

export interface SessionMetadata {
  message_count: number
  tool_call_count: number
  llm_call_count: number
  total_tokens: number
  client_ip?: string
  custom_data?: Record<string, unknown>
}

export interface CompanionSession {
  id: string
  platform: Platform
  user_id: string
  tenant_id?: string
  status: SessionStatus
  started_at: string
  ended_at?: string
  duration: number
  event_count: number
  threat_level: ThreatLevel
  threat_score: number
  metadata: SessionMetadata
}

export interface MessageData {
  direction: 'inbound' | 'outbound'
  contentType: string
  length: number
  content: string
  truncated: boolean
}

export interface ToolCallData {
  toolId: string
  toolName: string
  status: string
  duration: number
  sandboxUsed: boolean
  inputPreview: string
  outputPreview: string
}

export interface LLMRequestData {
  provider: string
  model: string
  promptTokens: number
  completionTokens: number
  totalTokens: number
  duration: number
  status: string
}

export interface SecurityData {
  threatLevel: ThreatLevel
  threatScore: number
  threatTypes: string[]
  action: string
  details: string
}

export interface SessionEvent {
  id: string
  session_id: string
  timestamp: string
  event_type: SessionEventType
  platform: Platform
  user_id: string
  tenant_id: string
  duration: number
  status: string
  message?: MessageData
  tool_call?: ToolCallData
  llm_request?: LLMRequestData
  security?: SecurityData
  error?: string
  custom_data?: Record<string, unknown>
}

export interface Alert {
  id: string
  session_id: string
  event_id: string
  severity: AlertSeverity
  title: string
  description: string
  timestamp: string
  acknowledged: boolean
  acked_at?: string
  acked_by?: string
  threat_level?: ThreatLevel
  details?: Record<string, unknown>
}

export interface FlowNode {
  id: string
  type: string
  label: string
  timestamp: string
  duration: number
  status: string
  data?: Record<string, unknown>
  position?: { x: number; y: number }
}

export interface FlowEdge {
  id: string
  source: string
  target: string
  label?: string
}

export interface FlowGraph {
  sessionId: string
  nodes: FlowNode[]
  edges: FlowEdge[]
}

export interface Stats {
  active_sessions: number
  total_sessions: number
  total_events: number
  total_alerts: number
  unacked_alerts: number
  avg_session_duration: number
  sessions_by_platform: Record<Platform, number>
  threats_by_level: Record<ThreatLevel, number>
  events_by_type: Record<SessionEventType, number>
  last_updated: string
}

export interface ListOptions {
  offset?: number
  limit?: number
  sort?: string
  order?: 'asc' | 'desc'
  platform?: Platform
  userId?: string
  status?: SessionStatus
  from?: string
  to?: string
}

export interface ListResponse<T> {
  sessions?: T[]
  events?: T[]
  alerts?: T[]
  total: number
  offset: number
  limit: number
}

// API endpoints

export const companionApi = {
  // Sessions
  listSessions: (opts?: ListOptions) =>
    api.get<ListResponse<CompanionSession>>('/companion/sessions', { params: opts }),

  getSession: (id: string) =>
    api.get<CompanionSession>(`/companion/sessions/${id}`),

  getSessionEvents: (id: string, opts?: ListOptions) =>
    api.get<ListResponse<SessionEvent>>(`/companion/sessions/${id}/events`, { params: opts }),

  getSessionFlow: (id: string) =>
    api.get<FlowGraph>(`/companion/sessions/${id}/flow`),

  deleteSession: (id: string) =>
    api.delete<{ message: string }>(`/companion/sessions/${id}`),

  // Alerts
  listAlerts: (opts?: ListOptions & { severity?: AlertSeverity; acknowledged?: boolean }) =>
    api.get<ListResponse<Alert>>('/companion/alerts', { params: opts }),

  acknowledgeAlert: (id: string) =>
    api.put<Alert>(`/companion/alerts/${id}/ack`),

  bulkAcknowledgeAlerts: (ids: string[]) =>
    api.put<{ acknowledged: number }>('/companion/alerts/bulk-ack', { alert_ids: ids }),

  // Stats
  getStats: () =>
    api.get<Stats>('/companion/stats'),

  // Export
  exportData: (opts?: { format?: 'json' | 'csv'; sessionIds?: string[]; from?: string; to?: string }) => {
    const params = new URLSearchParams()
    if (opts?.format) params.append('format', opts.format)
    if (opts?.from) params.append('from', opts.from)
    if (opts?.to) params.append('to', opts.to)
    opts?.sessionIds?.forEach(id => params.append('session_id', id))
    return api.get(`/companion/export?${params.toString()}`, { responseType: 'blob' })
  },
}

// WebSocket URL helper
export function getCompanionStreamUrl(sessionId?: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const path = sessionId
    ? `/api/v1/companion/session/${sessionId}`
    : '/api/v1/companion/stream'
  return `${protocol}//${host}${path}`
}

// Retention configuration
export interface RetentionConfig {
  events_days: number
  sessions_days: number
  alerts_days: number
}

// Storage info
export interface StorageInfo {
  session_count: number
  alert_count: number
  event_count: number
}

// Settings response
export interface CompanionSettings {
  retention: RetentionConfig
  storage_info: StorageInfo
}

// Settings API
export const companionSettingsApi = {
  getSettings: () =>
    api.get<CompanionSettings>('/companion/settings'),

  updateSettings: (retention: RetentionConfig) =>
    api.put<{ message: string; retention: RetentionConfig }>('/companion/settings', { retention }),

  triggerCleanup: () =>
    api.post<{ message: string; success: boolean }>('/companion/cleanup'),
}
