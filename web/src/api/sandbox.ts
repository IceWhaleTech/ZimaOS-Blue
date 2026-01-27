import api from './client'

// Types
export interface SandboxInfo {
  supported: boolean
  default_timeout: string
  max_timeout: string
  memory_limit: number
  cpu_limit: number
  process_limit: number
  network_enabled: boolean
}

export interface ResourceUsage {
  cpu_time_ns: number
  memory_peak_bytes: number
  io_read_bytes: number
  io_write_bytes: number
}

export interface ExecutionResult {
  id: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'timeout' | 'killed'
  exit_code: number
  stdout: string
  stderr: string
  start_time: string
  end_time: string
  duration: number
  resource_usage?: ResourceUsage
  error?: string
}

export interface ExecuteRequest {
  command: string
  args?: string[]
  env?: Record<string, string>
  work_dir?: string
  stdin?: string
  timeout_secs?: number
  memory_mb?: number
}

// Sandbox API
export const sandboxApi = {
  // Get sandbox info and configuration
  getInfo: () => api.get<SandboxInfo>('/sandbox/info'),

  // Execute a command in the sandbox
  execute: (request: ExecuteRequest) =>
    api.post<ExecutionResult>('/sandbox/execute', request),

  // Get execution status
  getStatus: (id: string) =>
    api.get<ExecutionResult>(`/sandbox/status/${id}`),

  // Kill a running execution
  kill: (id: string) =>
    api.post<{ status: string; message: string }>(`/sandbox/kill/${id}`),
}
