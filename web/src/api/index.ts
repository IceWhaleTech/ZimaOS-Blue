import api from './client'

// Export api client as default for other modules
export default api

// Re-export from other modules
export * from './auth'
export * from './plugin'
export * from './system'
export * from './gateway'
export * from './autoreply'
export * from './mfa'
export * from './workflow'
export * from './webhook'
export * from './cron'
export * from './audit'
export * from './security'
export * from './companion'
export * from './claudecode'

export interface HealthStatus {
  status: string
  timestamp: string
  uptime: string
  version: string
  go_version: string
  num_cpu: number
  goroutines: number
  mem_alloc_bytes: number
}

export interface WorkerStats {
  pool_size: number
  running: number
  total: number
}

export interface ConfigReloadStatus {
  enabled: boolean
  config_path: string
  last_reload: string
  reload_count: number
  is_reloading: boolean
  last_error?: string
}

export interface BackupInfo {
  id: string
  created_at: string
  size_bytes: number
  path: string
  type: string
}

export interface RuntimeStats {
  goroutines: number
  num_cpu: number
  gomaxprocs: number
  alloc_bytes: number
  sys_bytes: number
  heap_alloc: number
  heap_sys: number
  heap_idle: number
  heap_inuse: number
  heap_released: number
  heap_objects: number
  gc_cycles: number
  gc_pause_total: number
}

export const healthApi = {
  getHealth: () => api.get<HealthStatus>('/health'),
  getLiveness: () => api.get<{ status: string }>('/health/live'),
  getReadiness: () => api.get<{ status: string }>('/health/ready'),
  getDetailedHealth: () => api.get<HealthStatus & { runtime: RuntimeStats }>('/system/health/detailed'),
}

export const workerApi = {
  getStats: () => api.get<WorkerStats>('/workers/stats'),
}

export const configApi = {
  getReloadStatus: () => api.get<ConfigReloadStatus>('/config/status'),
  triggerReload: () => api.post<{ success: boolean; message: string }>('/config/reload'),
}

export const backupApi = {
  list: () => api.get<BackupInfo[]>('/backup'),
  create: () => api.post<BackupInfo>('/backup'),
  restore: (id: string) => api.post<{ success: boolean; message: string }>(`/backup/${id}/restore`),
  delete: (id: string) => api.delete<{ success: boolean }>(`/backup/${id}`),
}
