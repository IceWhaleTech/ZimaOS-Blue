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

// Security scan types
export interface SecurityScanItem {
  id: string
  category: string
  name: string
  description: string
  status: 'passed' | 'warning' | 'failed'
  details?: string
  risk?: string
  impact?: string
  remediation?: string
  auto_fixable?: boolean
  fix_action?: string
}

export interface FixPreviewResponse {
  fix_action: string
  description: string
  changes: string[]
  reversible: boolean
  warning?: string
}

export interface SecurityScanSummary {
  total: number
  passed: number
  warnings: number
  failed: number
}

export interface SecurityScanResult {
  items: SecurityScanItem[]
  summary: SecurityScanSummary
  timestamp: string
}

export interface FixScanIssueResponse {
  success: boolean
  message: string
  details?: string
}

// CORS configuration types
export interface CORSConfig {
  allowed_origins: string[]
  dynamic_origins: string[]
  allow_localhost: boolean
}

export interface CORSConfigUpdate {
  add_origins?: string[]
  remove_origins?: string[]
}

// TLS configuration types
export interface CertificateInfo {
  subject: string
  issuer: string
  domains: string[]
  not_before: string
  not_after: string
  is_ca: boolean
  is_self_signed: boolean
  serial_number: string
  fingerprint: string
}

export interface TLSConfig {
  enabled: boolean
  port: number
  has_cert: boolean
  cert_info?: CertificateInfo
  auto_cert: boolean
  acme_provider?: string
  acme_domains?: string[]
  self_signed: boolean
  https_only: boolean
  https_port: number
}

export interface ACMEStatus {
  configured: boolean
  email: string
  domains: string[]
  provider: string
  challenge_type?: string
  dns_provider?: string
  cert_info?: CertificateInfo
  error?: string
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

  // Security scan
  runSecurityScan: () =>
    api.get<SecurityScanResult>('/security/scan/run'),

  previewScanFix: (fixAction: string) =>
    api.post<FixPreviewResponse>('/security/scan/preview', { fix_action: fixAction }),

  fixScanIssue: (fixAction: string) =>
    api.post<FixScanIssueResponse>('/security/scan/fix', { fix_action: fixAction }),

  // CORS configuration
  getCORSConfig: () =>
    api.get<CORSConfig>('/security/cors'),

  updateCORSConfig: (config: CORSConfigUpdate) =>
    api.put<CORSConfig>('/security/cors', config),

  // TLS configuration
  getTLSConfig: () =>
    api.get<TLSConfig>('/security/tls'),

  uploadTLSCert: (certPem: string, keyPem: string) =>
    api.post<TLSConfig>('/security/tls/upload', { cert_pem: certPem, key_pem: keyPem }),

  generateSelfSignedCert: (domains: string[], validDays: number) =>
    api.post<TLSConfig>('/security/tls/self-signed', { domains, valid_days: validDays }),

  parseCertificate: (certPem: string) =>
    api.post<CertificateInfo>('/security/tls/parse', { cert_pem: certPem }),

  // ACME certificate
  getACMEStatus: () =>
    api.get<ACMEStatus>('/security/tls/acme'),

  requestACMECert: (email: string, domains: string[], provider: string, challengeType?: string, dnsProvider?: string, dnsCredentials?: Record<string, string>) =>
    api.post<ACMEStatus>('/security/tls/acme', { email, domains, provider, challenge_type: challengeType, dns_provider: dnsProvider, dns_credentials: dnsCredentials }),

  updateTLSSettings: (httpsOnly: boolean, httpsPort: number) =>
    api.put<TLSConfig>('/security/tls/settings', { https_only: httpsOnly, https_port: httpsPort }),

  reloadTLSCert: () =>
    api.post<TLSConfig>('/security/tls/reload'),
}
