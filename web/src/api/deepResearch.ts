import api from './client'

export interface DeepResearchJobSummary {
  id: string
  job_id: string
  query: string
  status: string
  stage: string
  progress: number
  iteration: number
  latest_action?: string
  latest_gap?: string
  conversation_id?: string
  updated_at: string
}

export interface DeepResearchJob extends DeepResearchJobSummary {
  mode?: 'fast' | 'standard' | 'deep' | string
  lang?: string
  error?: string
  report?: Record<string, unknown> | null
  [key: string]: unknown
}

export const deepResearchApi = {
  listJobs: (status?: 'active') =>
    api.get<DeepResearchJobSummary[]>('/deep-research/jobs', {
      params: status ? { status } : undefined,
    }),

  getJob: (id: string) => api.get<DeepResearchJob>(`/deep-research/jobs/${id}`),

  cancelJob: (id: string) => api.post<{ status: string }>(`/deep-research/jobs/${id}/cancel`),
}
