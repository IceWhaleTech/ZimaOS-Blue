import api from './client'

const HARNESS_RESEARCH_BASE = '/harness/research/jobs'

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
    api.get<DeepResearchJobSummary[]>(HARNESS_RESEARCH_BASE, {
      params: status ? { status } : undefined,
    }),

  getJob: (id: string) => api.get<DeepResearchJob>(`${HARNESS_RESEARCH_BASE}/${id}`),

  cancelJob: (id: string) => api.post<{ status: string }>(`${HARNESS_RESEARCH_BASE}/${id}/cancel`),
}
