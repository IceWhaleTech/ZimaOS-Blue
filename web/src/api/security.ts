import api from './client'

// Types
export interface Session {
  id: string
  user_id: string
  ip_address: string
  user_agent: string
  created_at: string
  last_activity: string
  expires_at: string
  is_current: boolean
}

export interface SecuritySettings {
  password_min_length: number
  password_require_uppercase: boolean
  password_require_lowercase: boolean
  password_require_numbers: boolean
  password_require_special: boolean
  session_timeout_minutes: number
  max_login_attempts: number
  lockout_duration_minutes: number
  mfa_required: boolean
  api_rate_limit: number
}

export interface SecurityEvent {
  id: string
  type: string
  user_id?: string
  ip_address: string
  user_agent?: string
  details?: Record<string, unknown>
  timestamp: string
  severity: 'low' | 'medium' | 'high' | 'critical'
}

export interface SecurityStats {
  active_sessions: number
  failed_logins_24h: number
  blocked_ips: number
  mfa_enabled_users: number
  total_users: number
  api_keys_active: number
}

export interface BlockedIP {
  ip_address: string
  reason: string
  blocked_at: string
  expires_at?: string
  permanent: boolean
}

// Threat detection types
export type ThreatType =
  | 'injection'
  | 'xss'
  | 'sql_injection'
  | 'path_traversal'
  | 'command_injection'
  | 'prompt_injection'
  | 'brute_force'
  | 'rate_limit'
  | 'suspicious_ip'

export type ThreatSeverity = 'low' | 'medium' | 'high' | 'critical'

export interface ThreatEvent {
  id: string
  type: ThreatType
  severity: ThreatSeverity
  source: string
  ip_address: string
  user_id?: string
  description: string
  details?: string
  blocked: boolean
  timestamp: string
}

export interface ThreatStats {
  total_threats_24h: number
  blocked_threats_24h: number
  critical_threats_24h: number
  high_threats_24h: number
  top_threat_types: Record<string, number>
  top_source_ips: Record<string, number>
  is_secure: boolean
  risk_level: 'safe' | 'low' | 'medium' | 'high' | 'critical'
}

export interface ScanRequest {
  input: string
  source?: string
}

export interface ScanResponse {
  safe: boolean
  threats: ThreatEvent[]
}

// Security API
export const securityApi = {
  // Sessions
  listSessions: () => api.get<Session[]>('/security/sessions'),

  revokeSession: (id: string) =>
    api.delete<{ success: boolean }>(`/security/sessions/${id}`),

  revokeAllSessions: () =>
    api.post<{ success: boolean; revoked_count: number }>('/security/sessions/revoke-all'),

  // Security settings
  getSettings: () => api.get<SecuritySettings>('/security/settings'),

  updateSettings: (settings: Partial<SecuritySettings>) =>
    api.put<SecuritySettings>('/security/settings', settings),

  // Security events
  getEvents: (params?: { limit?: number; severity?: string; type?: string }) =>
    api.get<SecurityEvent[]>('/security/events', { params }),

  // Security stats
  getStats: () => api.get<SecurityStats>('/security/stats'),

  // IP blocking
  listBlockedIPs: () => api.get<BlockedIP[]>('/security/blocked-ips'),

  blockIP: (ip: string, reason: string, permanent?: boolean) =>
    api.post<BlockedIP>('/security/blocked-ips', { ip_address: ip, reason, permanent }),

  unblockIP: (ip: string) =>
    api.delete<{ success: boolean }>(`/security/blocked-ips/${encodeURIComponent(ip)}`),

  // Threat detection
  getThreatStats: () => api.get<ThreatStats>('/security/threats/stats'),

  getRecentThreats: (limit?: number) =>
    api.get<ThreatEvent[]>('/security/threats', { params: { limit } }),

  scanInput: (request: ScanRequest) =>
    api.post<ScanResponse>('/security/scan', request),
}
