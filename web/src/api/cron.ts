import api from './client'

export interface CronJob {
  id: string
  name: string
  description?: string
  schedule: string
  handler: string
  payload?: Record<string, unknown>
  enabled: boolean
  last_run?: string
  next_run?: string
  created_at: string
  updated_at: string
}

export interface JobExecution {
  id: string
  job_id: string
  status: 'success' | 'failure'
  started_at: string
  completed_at: string
  duration_ms: number
  error?: string
  output?: string
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
  list: () => api.get<CronJob[]>('/cron'),

  // Get cron job by ID
  get: (id: string) => api.get<CronJob>(`/cron/${id}`),

  // Create cron job
  create: (data: CreateCronJobRequest) => api.post<CronJob>('/cron', data),

  // Update cron job
  update: (id: string, data: UpdateCronJobRequest) =>
    api.put<{ status: string }>(`/cron/${id}`, data),

  // Delete cron job
  delete: (id: string) => api.delete<{ status: string }>(`/cron/${id}`),

  // Enable cron job
  enable: (id: string) => api.post<{ status: string }>(`/cron/${id}/enable`),

  // Disable cron job
  disable: (id: string) => api.post<{ status: string }>(`/cron/${id}/disable`),

  // Trigger cron job manually
  trigger: (id: string) => api.post<{ status: string }>(`/cron/${id}/trigger`),

  // Get job executions
  getExecutions: (id: string, limit = 20) =>
    api.get<JobExecution[]>(`/cron/${id}/executions`, { params: { limit } }),
}
