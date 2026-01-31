import api from './client'

// Types
export interface TunnelStatus {
  active: boolean
  connecting?: boolean
  url?: string
  started_at?: string
  expires_at?: string
  remaining_time?: string
  renewed_count?: number
  provider?: string
  tunnel_password?: string // For LocalTunnel (loca.lt): password from mytunnelpassword
}

export interface RemoteAccessStatus {
  tunnel: TunnelStatus
}

export interface RemoteAccessConfig {
  enabled: boolean
  notification_email?: string
  notify_on_url_change?: boolean
  notify_on_expiry_warning?: boolean
  notify_on_error?: boolean
  ngrok_authtoken?: string
  ngrok_domain?: string
  cloudflare_token?: string
  default_provider?: string
  tunnel_subdomain?: string
}

export interface TunnelProvider {
  id: string
  name: string
  description: string
  requires_key: boolean
  key_label?: string
  key_hint?: string
  doc_url?: string
}

// API functions

/**
 * Get available tunnel providers
 */
export function getTunnelProviders() {
  return api.get<{ success: boolean; providers: TunnelProvider[] }>('/tunnel/providers')
}

/**
 * Start remote access tunnel
 */
export function startRemoteAccess(provider?: string, port?: number, authtoken?: string, cloudflareToken?: string, ngrokDomain?: string) {
  return api.post<{ success: boolean; message: string; tunnel?: TunnelStatus; provider?: string }>('/tunnel/start', {
    provider: provider || 'auto',
    port: port || 8080,
    ngrok_authtoken: authtoken,
    ngrok_domain: ngrokDomain,
    cloudflare_token: cloudflareToken
  })
}

/**
 * Stop remote access tunnel
 */
export function stopRemoteAccess() {
  return api.post<{ success: boolean; message: string }>('/tunnel/stop')
}

/**
 * Get remote access status
 */
export function getRemoteAccessStatus() {
  return api.get<{ success: boolean; tunnel: TunnelStatus }>('/tunnel/status')
}

/**
 * Get remote access configuration
 */
export function getRemoteAccessConfig() {
  return api.get<{ success: boolean; config: RemoteAccessConfig }>('/tunnel/config')
}

/**
 * Update remote access configuration
 */
export function updateRemoteAccessConfig(config: Partial<RemoteAccessConfig>) {
  return api.put<{ success: boolean }>('/tunnel/config', config)
}

/**
 * Get QR code for current tunnel URL
 */
export function getRemoteAccessQRCode() {
  return api.get<{ success: boolean; qrcode: string; url: string }>('/tunnel/qrcode')
}

/**
 * Get diagnostics information for troubleshooting
 */
export function getRemoteAccessDiagnostics() {
  return api.get<{
    success: boolean
    diagnostics: {
      tunnel_running: boolean
      firewall_exception: boolean
      ssh_available: boolean
      cloudflared_installed: boolean
      active_provider?: string
      recent_errors?: Array<{
        id: string
        event_type: string
        message: string
        created_at: string
      }>
      os: {
        platform: string
      }
      hints: string[]
    }
  }>('/tunnel/diagnostics')
}

export interface RemoteAccessLog {
  id: string
  session_id?: string
  event_type: string
  message: string
  created_at: string
}

/**
 * Get remote access logs
 */
export function getRemoteAccessLogs(limit = 50, offset = 0) {
  return api.get<{
    success: boolean
    logs: RemoteAccessLog[]
    limit: number
    offset: number
  }>('/tunnel/logs', { params: { limit, offset } })
}
