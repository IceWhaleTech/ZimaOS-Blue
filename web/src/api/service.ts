import api from './client'

// Types
export interface ServiceInfo {
  platform: string
  service_name: string
  service_type: string
  executable_path: string
  installed: boolean
  running: boolean
  enabled: boolean
  status: string
  start_type: string
  description?: string
  install_path?: string
  config_path?: string
  log_path?: string
}

export interface ServiceStatus {
  platform: string
  status: string
  running: boolean
  installed: boolean
  enabled: boolean
}

export interface ServiceResponse {
  success: boolean
  message: string
  output?: string
}

export interface InstallCheckResult {
  can_install: boolean
  method: 'standard' | 'sysext' | 'none'
  path?: string
  requires_root: boolean
  message: string
  message_key: string
}

// Service API
export const serviceApi = {
  getInfo: () => api.get<ServiceInfo>('/service/info'),

  getStatus: () => api.get<ServiceStatus>('/service/status'),

  checkInstall: () => api.get<InstallCheckResult>('/service/install/check'),

  install: async () => {
    // Windows Tauri: use elevated Rust command for Windows Service installation
    if (window.__TAURI_INTERNALS__ && navigator.userAgent.toLowerCase().includes('win')) {
      try {
        const { invoke } = await import('@tauri-apps/api/core')
        const result = await invoke<string>('install_windows_service')
        return { data: { success: true, message: result } }
      } catch (error) {
        return { data: { success: false, message: String(error) } }
      }
    }
    // macOS/Linux (and standalone CLI): use HTTP API (launchd/systemd)
    return api.post<ServiceResponse>('/service/install')
  },

  uninstall: async () => {
    // Windows Tauri: use elevated Rust command for Windows Service removal
    if (window.__TAURI_INTERNALS__ && navigator.userAgent.toLowerCase().includes('win')) {
      try {
        const { invoke } = await import('@tauri-apps/api/core')
        const result = await invoke<string>('uninstall_windows_service')
        return { data: { success: true, message: result } }
      } catch (error) {
        return { data: { success: false, message: String(error) } }
      }
    }
    // macOS/Linux (and standalone CLI): use HTTP API (launchd/systemd)
    return api.post<ServiceResponse>('/service/uninstall')
  },

  start: () => api.post<ServiceResponse>('/service/start'),

  stop: () => api.post<ServiceResponse>('/service/stop'),

  restart: () => api.post<ServiceResponse>('/service/restart'),

  enable: () => api.post<ServiceResponse>('/service/enable'),

  disable: () => api.post<ServiceResponse>('/service/disable'),
}
