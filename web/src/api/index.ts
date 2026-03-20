import api from './client'

// Export api client as default for other modules
export default api

// Re-export from other modules
export * from './auth'
export * from './plugin'
export * from './system'
export * from './autoreply'
export * from './mfa'
export * from './workflow'
export * from './cron'
export * from './audit'
export * from './security'
export * from './companion'
export * from './claudecode'
export * from './service'
export * from './userdata'
export * from './proxyCache'
export * from './deepResearch'
export * from './harness'

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
  created_by?: 'manual' | 'auto' | 'checkpoint'
  is_checkpoint?: boolean
  checkpoint_reason?: string
}

export interface PendingRestore {
  backup_id: string
  backup_path: string
  staging_dir: string
  created_at: string
}

export interface BackupRestoreResult {
  success: boolean
  files_restored: number
  files_skipped: number
  checkpoint_id?: string
  checkpoint_at?: string
  checkpoint_reason?: string
  errors?: string[]
}

export interface BackupRestoreRequest {
  force?: boolean
  require_restart?: boolean
  create_checkpoint?: boolean
  auto_restart?: boolean
}

export interface BackupRestoreResponse {
  success: boolean
  message: string
  requires_restart?: boolean
  restarting?: boolean
  pending_restore?: PendingRestore | null
  result?: BackupRestoreResult
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
  getHealth: async () => {
    try {
      // Dashboard cards need runtime stats (uptime/memory/goroutines),
      // which are exposed by /health/stats.
      return await api.get<HealthStatus>('/health/stats')
    } catch (error) {
      const status = (error as { response?: { status?: number } })?.response?.status
      if (status !== 404) {
        throw error
      }

      // Fallback for older/minimal deployments that only expose /health.
      return api.get<HealthStatus>('/health')
    }
  },
  getLiveness: () => api.get<{ status: string }>('/health/live'),
  getReadiness: () => api.get<{ status: string }>('/health/ready'),
  getDetailedHealth: () =>
    api.get<HealthStatus & { runtime: RuntimeStats }>('/system/health/detailed'),
}

export const workerApi = {
  getStats: () => api.get<WorkerStats>('/workers/stats'),
}

export const configApi = {
  getReloadStatus: () => api.get<ConfigReloadStatus>('/config/status'),
  triggerReload: () => api.post<{ success: boolean; message: string }>('/config/reload'),
}

export interface BackupProgress {
  in_progress: boolean
  operation: 'backup' | 'restore' | ''
  progress: number // 0-100
  current_file: string
  files_processed: number
  total_files: number
  bytes_processed: number
  total_bytes: number
  started_at: string
  error?: string
}

export const backupApi = {
  list: () => api.get<BackupInfo[]>('/backup'),
  create: () => api.post<BackupInfo>('/backup', {}, { timeout: 0 }), // No timeout for backup
  restore: (id: string, req: BackupRestoreRequest = {}) =>
    api.post<BackupRestoreResponse>(`/backup/${id}/restore`, req, { timeout: 0 }), // No timeout for restore
  delete: (id: string) => api.delete<{ success: boolean }>(`/backup/${id}`),
  getProgress: () => api.get<BackupProgress>('/backup/progress'),
}
