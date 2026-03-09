import api from './client'

export interface CronJob {
  id: string
  name: string
  description?: string
  schedule: string
  handler: string
  payload?: Record<string, unknown>
  enabled: boolean
  status: 'active' | 'paused' | 'running'
  last_run_at?: string
  next_run_at?: string
  created_at: string
  updated_at: string
  run_count: number
  fail_count: number
}

export interface JobExecution {
  id: string
  job_id: string
  status: 'running' | 'completed' | 'failed'
  started_at: string
  ended_at?: string
  duration?: number
  error?: string
  result?: unknown
}

export interface CreateCronJobRequest {
  name: string
  description?: string
  schedule: string
  handler: string
  payload?: Record<string, unknown>
}

export interface UpdateCronJobRequest {
  name: string
  description?: string
  schedule: string
  payload?: Record<string, unknown>
}

export const cronApi = {
  // List cron jobs
  // Note: Cron routes are registered under /api/cron/* (without /v1)
  list: () => api.get<CronJob[]>('/cron', { baseURL: '/api' }),

  // Get cron job by ID
  get: (id: string) => api.get<CronJob>(`/cron/${id}`, { baseURL: '/api' }),

  // Create cron job
  create: (data: CreateCronJobRequest) => api.post<CronJob>('/cron', data, { baseURL: '/api' }),

  // Update cron job
  update: (id: string, data: UpdateCronJobRequest) =>
    api.put<{ status: string }>(`/cron/${id}`, data, { baseURL: '/api' }),

  // Delete cron job
  delete: (id: string) => api.delete<{ status: string }>(`/cron/${id}`, { baseURL: '/api' }),

  // Enable cron job
  enable: (id: string) => api.post<{ status: string }>(`/cron/${id}/enable`, null, { baseURL: '/api' }),

  // Disable cron job
  disable: (id: string) => api.post<{ status: string }>(`/cron/${id}/disable`, null, { baseURL: '/api' }),

  // Trigger cron job manually
  trigger: (id: string) => api.post<{ status: string }>(`/cron/${id}/trigger`, null, { baseURL: '/api' }),

  // Get job executions
  getExecutions: (id: string, limit = 20) =>
    api.get<JobExecution[]>(`/cron/${id}/executions`, { params: { limit }, baseURL: '/api' }),
}
