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
}

// API functions

/**
 * Start remote access tunnel
 */
export function startRemoteAccess(port?: number, authtoken?: string) {
  return api.post<{ success: boolean; message: string; tunnel?: TunnelStatus }>('/remote-access/start', {
    port: port || 8080,
    authtoken
  })
}

/**
 * Stop remote access tunnel
 */
export function stopRemoteAccess() {
  return api.post<{ success: boolean; message: string }>('/remote-access/stop')
}

/**
 * Get remote access status
 */
export function getRemoteAccessStatus() {
  return api.get<RemoteAccessStatus>('/remote-access/status')
}

/**
 * Get remote access configuration
 */
export function getRemoteAccessConfig() {
  return api.get<RemoteAccessConfig>('/remote-access/config')
}

/**
 * Update remote access configuration
 */
export function updateRemoteAccessConfig(config: Partial<RemoteAccessConfig>) {
  return api.put<{ success: boolean }>('/remote-access/config', config)
}

/**
 * Get QR code for current tunnel URL
 */
export function getRemoteAccessQRCode() {
  return api.get<{ qrcode: string; url: string }>('/remote-access/qrcode')
}

/**
 * Get diagnostics information for troubleshooting
 */
export function getRemoteAccessDiagnostics() {
  return api.get<{
    success: boolean
    diagnostics: {
      firewall_exception: boolean
      tunnel_running: boolean
      recent_errors?: Array<{
        id: string
        event_type: string
        message: string
        created_at: string
      }>
      active_session?: {
        id: string
        status: string
        started_at: string
        error_message?: string
      }
      os: {
        platform: string
      }
      hints: string[]
    }
  }>('/remote-access/diagnostics')
}
