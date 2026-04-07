import api from './client'

const KNOWLEDGE_BASE = '/knowledge'

export type KnowledgeJobKind = 'ingest' | 'compile' | 'lint' | 'answer'
export type KnowledgeConfidence = 'low' | 'medium' | 'high'
export type KnowledgeStatus = 'active' | 'superseded' | 'conflicted'

export interface KnowledgePageSummary {
  title: string
  slug: string
  page_type: string
  summary: string
  source_refs: string[]
  keywords: string[]
  backlinks: string[]
  generated_at: string
  updated_at: string
  source_hash: string
  status: KnowledgeStatus
  confidence: KnowledgeConfidence
  conflicts_with?: string[]
  superseded_by?: string[]
  derived_from_query?: string
}

export interface KnowledgeArchivedAnswer {
  title: string
  path: string
  page_slug: string
  query: string
  summary: string
  generated_at: string
}

export interface KnowledgePage extends KnowledgePageSummary {
  content: string
  answers?: KnowledgeArchivedAnswer[]
}

export interface KnowledgeLintIssue {
  kind: string
  page_slug?: string
  message: string
  auto_fixed?: boolean
  severity?: string
  category?: string
  related_pages?: string[]
  suggested_action?: string
}

export interface KnowledgeLintReport {
  generated_at: string
  issues: KnowledgeLintIssue[]
  fixed_paths?: string[]
}

export interface KnowledgeCitation {
  page_slug: string
  title: string
  source_refs?: string[]
}

export interface KnowledgeAnswerReport {
  generated_at: string
  query: string
  answer: string
  citations: KnowledgeCitation[]
  related_pages?: KnowledgePageSummary[]
  archived_path?: string
  confidence?: KnowledgeConfidence
  conflict_notes?: string[]
  open_questions?: string[]
  promoted_page_slug?: string
}

export interface KnowledgeIngestReport {
  generated_at: string
  pages: KnowledgePageSummary[]
  new_pages?: KnowledgePageSummary[]
  updated_pages?: KnowledgePageSummary[]
  skipped_sources?: string[]
  conflicts?: string[]
  gaps?: string[]
  manifest_path: string
  index_path: string
  schema_path: string
  log_path: string
  fallback_mode?: boolean
}

export type KnowledgeCompileReport = KnowledgeIngestReport

export interface KnowledgeJobReport {
  kind: KnowledgeJobKind
  ingest?: KnowledgeIngestReport | null
  compile?: KnowledgeCompileReport | null
  lint?: KnowledgeLintReport | null
  answer?: KnowledgeAnswerReport | null
}

export interface KnowledgeJobSummary {
  id: string
  job_id: string
  provider_id?: string
  query?: string
  kind: KnowledgeJobKind
  status: string
  progress: number
  stage?: string
  updated_at: string
}

export interface KnowledgeJob extends KnowledgeJobSummary {
  created_at: string
  completed_at?: string
  error?: string
  report?: KnowledgeJobReport | null
}

export interface KnowledgeCreateJobRequest {
  kind: KnowledgeJobKind
  provider_id?: string
  query?: string
  target_paths?: string[]
  page_slug?: string
  archive_answer?: boolean
  query_scope?: 'all' | 'current_page' | 'selected_sources'
  selected_refs?: string[]
}

export interface KnowledgeSchemaDocument {
  content: string
}

export interface KnowledgeLogEntry {
  timestamp: string
  operation: string
  title: string
  sources?: string[]
  new_pages?: string[]
  updated_pages?: string[]
  conflicts?: string[]
  gaps?: string[]
  reason?: string
}

export interface PromoteQueryResult {
  page: KnowledgePageSummary
}

export const knowledgeApi = {
  createJob: (request: KnowledgeCreateJobRequest) =>
    api.post<KnowledgeJob>(`${KNOWLEDGE_BASE}/jobs`, request),

  listJobs: (status?: 'active') =>
    api.get<KnowledgeJobSummary[]>(`${KNOWLEDGE_BASE}/jobs`, {
      params: status ? { status } : undefined,
    }),

  getJob: (id: string) => api.get<KnowledgeJob>(`${KNOWLEDGE_BASE}/jobs/${id}`),

  getReport: (id: string) => api.get<KnowledgeJobReport>(`${KNOWLEDGE_BASE}/jobs/${id}/report`),

  cancelJob: (id: string) => api.post<{ status: string }>(`${KNOWLEDGE_BASE}/jobs/${id}/cancel`),

  listPages: () => api.get<KnowledgePageSummary[]>(`${KNOWLEDGE_BASE}/pages`),

  getPage: (slug: string) => api.get<KnowledgePage>(`${KNOWLEDGE_BASE}/pages/${slug}`),

  getIndex: () => api.get<{ content: string }>(`${KNOWLEDGE_BASE}/index`),

  getSchema: () => api.get<KnowledgeSchemaDocument>(`${KNOWLEDGE_BASE}/schema`),

  updateSchema: (content: string) =>
    api.put<KnowledgeSchemaDocument>(`${KNOWLEDGE_BASE}/schema`, { content }),

  getLog: () => api.get<KnowledgeLogEntry[]>(`${KNOWLEDGE_BASE}/log`),

  promoteQuery: (jobId: string) =>
    api.post<PromoteQueryResult>(`${KNOWLEDGE_BASE}/query/${jobId}/promote`),

  getLatestLint: () => api.get<KnowledgeLintReport>(`${KNOWLEDGE_BASE}/lint/latest`),
}

export default knowledgeApi
