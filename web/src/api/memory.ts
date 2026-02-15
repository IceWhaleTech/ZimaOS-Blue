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
  vector_score?: number
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
  search_type?: 'hybrid' | 'vector' | 'keyword'
}

export interface SearchMemoryResponse {
  results: MemorySearchResult[]
  total: number
}

export interface PruneResponse {
  deleted: number
}

export interface MemoryBackendConfig {
  backend: 'local'
}

export interface MemoryBackendStatus {
  active_backend: string
}

// Memory API
export const memoryApi = {
  /**
   * Store a new memory
   */
  store: (request: StoreMemoryRequest) =>
    api.post<StoreMemoryResponse>('/memory/store', request),

  /**
   * Search memories using hybrid search (vector + keyword)
   */
  search: (request: SearchMemoryRequest) =>
    api.post<SearchMemoryResponse>('/memory/search', request),

  /**
   * Get a memory by ID
   */
  get: (id: string) =>
    api.get<Memory>(`/memory/${id}`),

  /**
   * Delete a memory by ID
   */
  delete: (id: string) =>
    api.delete(`/memory/${id}`),

  /**
   * Prune old memories
   */
  prune: () =>
    api.post<PruneResponse>('/memory/prune'),

  /**
   * Clear all memories
   */
  clear: () =>
    api.delete('/memory'),

  /**
   * Get memory statistics
   */
  stats: () =>
    api.get<MemoryStats>('/memory/stats'),

  /**
   * Get backend status
   */
  getBackendStatus: () =>
    api.get<MemoryBackendStatus>('/memory/backend'),

  /**
   * Set active backend
   */
  setBackend: (backend: 'local') =>
    api.post<{ success: boolean }>('/memory/backend', { backend }),

  /**
   * Export memories as Markdown
   */
  exportMarkdown: () =>
    api.get<string>('/memory/export', {
      responseType: 'text' as const,
    }),

  /**
   * Import memories from Markdown
   */
  importMarkdown: (content: string, mode: 'append' | 'replace' = 'append') =>
    api.post<{ imported: number; skipped: number; errors: string[] }>('/memory/import', {
      content,
      mode,
    }),
}

// ─── v2 Memory Service Types ───

export interface MemoryEntry {
  id: string
  namespace: string
  content: string
  content_type: 'text' | 'json' | 'markdown'
  category: string
  tags: string[]
  metadata: Record<string, any> | null
  importance: number
  source: string
  version: number
  parent_id: string
  status: 'active' | 'expired' | 'deleted'
  ttl: string
  created_at: string
  updated_at: string
  expires_at: string | null
  deleted_at: string | null
}

export interface MemoryEntryListResponse {
  entries: MemoryEntry[]
  next_cursor: string
  count: number
}

export interface MemoryEntryStats {
  namespace: string
  total: number
  active: number
  expired: number
  deleted: number
}

export interface MemoryNamespace {
  id: string
  config: {
    embedding_dim: number
    default_ttl: string
    max_entries: number
    max_size_bytes: number
  }
  created_at: string
  updated_at: string
}

export interface CreateMemoryEntryRequest {
  content: string
  content_type?: 'text' | 'json' | 'markdown'
  category?: string
  tags?: string[]
  metadata?: Record<string, any>
  importance?: number
  source?: string
  ttl?: string
}

// v2 Memory Service API (namespace-aware, versioned entries)
export const memoryServiceApi = {
  // Entries
  createEntry: (data: CreateMemoryEntryRequest, namespace?: string) =>
    api.post<MemoryEntry>('/v2/memories', data, {
      headers: namespace ? { 'X-Namespace': namespace } : undefined,
    }),

  getEntry: (id: string) =>
    api.get<MemoryEntry>(`/v2/memories/${id}`),

  updateEntry: (id: string, content: string) =>
    api.put<MemoryEntry>(`/v2/memories/${id}`, { content }),

  deleteEntry: (id: string) =>
    api.delete(`/v2/memories/${id}`),

  listEntries: (params: {
    namespace?: string
    status?: string
    category?: string
    cursor?: string
    limit?: number
  } = {}) =>
    api.get<MemoryEntryListResponse>('/v2/memories', {
      params,
      headers: params.namespace ? { 'X-Namespace': params.namespace } : undefined,
    }),

  getHistory: (id: string) =>
    api.get<{ versions: MemoryEntry[]; count: number }>(`/v2/memories/${id}/history`),

  getStats: (namespace?: string) =>
    api.get<MemoryEntryStats>('/v2/memories/stats', {
      headers: namespace ? { 'X-Namespace': namespace } : undefined,
    }),

  purgeExpired: () =>
    api.delete<{ purged: number }>('/v2/memories/expired'),

  // Namespaces
  createNamespace: (id: string, config?: Partial<MemoryNamespace['config']>) =>
    api.post<MemoryNamespace>('/v2/namespaces', { id, config }),

  listNamespaces: () =>
    api.get<{ namespaces: MemoryNamespace[]; count: number }>('/v2/namespaces'),

  deleteNamespace: (id: string) =>
    api.delete(`/v2/namespaces/${id}`),
}

export default memoryApi
