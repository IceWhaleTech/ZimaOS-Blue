import api from './client'

// Types
export interface LogEntry {
  timestamp: string
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  source?: string
  fields?: Record<string, unknown>
}

export interface LogQueryParams {
  level?: string
  source?: string
  search?: string
  limit?: number
  offset?: number
  start_time?: string
  end_time?: string
}

export interface SystemMetrics {
  timestamp: string
  cpu_percent: number
  memory_used_bytes: number
  memory_total_bytes: number
  goroutines: number
  gc_pause_ns: number
  heap_alloc_bytes: number
  heap_sys_bytes: number
  stack_inuse_bytes: number
}

export interface MetricsHistory {
  metrics: SystemMetrics[]
  interval_seconds: number
}

// System API
export const systemApi = {
  getLogs: (params?: LogQueryParams) => api.get<LogEntry[]>('/system/logs', { params }),

  getMetrics: () => api.get<SystemMetrics>('/system/metrics'),

  getMetricsHistory: (duration?: string) =>
    api.get<MetricsHistory>('/system/metrics/history', { params: { duration } }),

  getConfig: () => api.get<Record<string, unknown>>('/system/config'),

  updateConfig: (config: Record<string, unknown>) =>
    api.put<{ success: boolean; message: string }>('/system/config', config),

  restartService: () => api.post<{ success: boolean; message: string }>('/system/restart'),

  getInfo: () =>
    api.get<{
      version: string
      go_version: string
      build_time: string
      git_commit: string
      os: string
      arch: string
    }>('/system/info'),
}
