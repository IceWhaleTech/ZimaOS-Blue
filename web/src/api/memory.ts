import api from './client'

// Types
export interface Memory {
  id: string
  content: string
  metadata?: Record<string, string>
  created_at: string
  updated_at?: string
}

export interface MemorySearchResult {
  id: string
  content: string
  score: number
  keyword_score?: number
  match_types: string[]
  metadata?: Record<string, string>
  created_at: string
}

export interface MemoryStats {
  total_chunks: number
  total_size_bytes: number
  oldest_chunk?: string
  newest_chunk?: string
  backend?: string
  total_display_count?: number
  total_display_size_bytes?: number
  daily_logs_count?: number
  daily_entries_count?: number
  daily_total_size_bytes?: number
}

export interface StoreMemoryRequest {
  content: string
  tags?: string[]
}

export interface StoreMemoryResponse {
  id: string
  content: string
  created_at: string
}

export interface SearchMemoryRequest {
  query: string
  limit?: number
}

export interface SearchMemoryResponse {
  results: MemorySearchResult[]
  total: number
}

export interface PruneResponse {
  deleted: number
}

// Memory API
export const memoryApi = {
  /** Store a new memory */
  store: (request: StoreMemoryRequest) => api.post<StoreMemoryResponse>('/memory/store', request),

  /** Search memories by keyword */
  search: (request: SearchMemoryRequest) =>
    api.post<SearchMemoryResponse>('/memory/search', request),

  /** Get a memory by ID */
  get: (id: string) => api.get<Memory>(`/memory/${encodeURIComponent(id)}`),

  /** Delete a memory by ID */
  delete: (id: string) => api.delete(`/memory/${encodeURIComponent(id)}`),

  /** Prune old memories */
  prune: () => api.post<PruneResponse>('/memory/prune'),

  /** Clear all memories */
  clear: () => api.delete('/memory'),

  /** Get memory statistics */
  stats: () => api.get<MemoryStats>('/memory/stats'),

  /** Export memories as Markdown */
  exportMarkdown: () =>
    api.get<string>('/memory/export', {
      responseType: 'text' as const,
    }),

  /** Import memories from Markdown */
  importMarkdown: (content: string, mode: 'append' | 'replace' = 'append') =>
    api.post<{ imported: number; skipped: number; errors: string[] }>('/memory/import', {
      content,
      mode,
    }),
}

export default memoryApi
