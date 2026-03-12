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

// Detailed system info types
export interface OSInfo {
  name: string
  version: string
  kernel: string
  architecture: string
  hostname: string
  uptime: number
  uptime_human: string
  boot_time: number
}

export interface CPUInfo {
  model: string
  cores: number
  threads: number
  frequency: number
  usage: number
  vendor_id: string
  cache_size: number
}

export interface MemoryInfo {
  total: number
  available: number
  used: number
  used_percent: number
  swap_total: number
  swap_used: number
}

export interface DiskInfo {
  device: string
  mount_point: string
  fs_type: string
  total: number
  used: number
  available: number
  used_percent: number
}

export interface GPUInfo {
  name: string
  vendor: string
  driver: string
  memory_total: number
  memory_used: number
}

export interface InterfaceInfo {
  name: string
  mac: string
  ipv4: string[]
  ipv6: string[]
  mtu: number
  flags: string[]
  is_up: boolean
  is_loopback: boolean
}

export interface NetworkInfo {
  interfaces: InterfaceInfo[]
  public_ip?: string
}

export interface RuntimeInfo {
  go_version: string
  num_cpu: number
  num_goroutine: number
  gomaxprocs: number
  alloc_mb: number
  total_alloc_mb: number
  sys_mb: number
  num_gc: number
}

export interface HardwareInfo {
  cpu: CPUInfo
  memory: MemoryInfo
  disk: DiskInfo[]
  gpu: GPUInfo[]
}

export interface DetailedSystemInfo {
  os: OSInfo
  hardware: HardwareInfo
  network: NetworkInfo
  runtime: RuntimeInfo
}

export interface SystemInfo {
  version: string
  go_version: string
  build_time: string
  git_commit: string
  os: string
  arch: string
  system?: DetailedSystemInfo
}

export interface LocalFileResolveResponse {
  path: string
  name: string
  size_bytes: number
  mime_type: string
  download_url: string
  thumbnail_url?: string
}

// System API
export const systemApi = {
  getLogs: (params?: LogQueryParams) => api.get<LogEntry[]>('/system/logs', { params }),

  // Write a client-side log entry to the server
  writeLog: (level: 'info' | 'warn' | 'error', message: string, source?: string) =>
    api.post<{ success: boolean }>('/system/logs', { level, message, source: source || 'web-client' }),

  getMetrics: () => api.get<SystemMetrics>('/system/metrics'),

  getMetricsHistory: (duration?: string) =>
    api.get<MetricsHistory>('/system/metrics/history', { params: { duration } }),

  getConfig: () => api.get<Record<string, unknown>>('/system/config'),

  updateConfig: (config: Record<string, unknown>) =>
    api.put<{ success: boolean; message: string }>('/system/config', config),

  restartService: () => api.post<{ success: boolean; message: string }>('/system/restart'),

  getInfo: (detailed?: boolean) =>
    api.get<SystemInfo>('/system/info', { params: detailed ? { detailed: 'true' } : undefined }),

  revealPath: (path: string) =>
    api.post<{ success: boolean }>('/system/reveal-path', { path }),

  resolveLocalFile: (path: string) =>
    api.get<LocalFileResolveResponse>('/system/local-file', { params: { path } }),
}
