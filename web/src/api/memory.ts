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
  backend: 'local' | 'supermemory'
  supermemory?: {
    enabled: boolean
    api_key: string
    base_url?: string
  }
}

export interface MemoryBackendStatus {
  active_backend: string
  supermemory_available: boolean
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
  setBackend: (backend: 'local' | 'supermemory') =>
    api.post<{ success: boolean }>('/memory/backend', { backend }),

  /**
   * Configure Supermemory
   */
  configureSupermemory: (config: { enabled: boolean; api_key: string; base_url?: string }) =>
    api.post<{ success: boolean }>('/memory/supermemory/config', config),

  /**
   * Test Supermemory connection
   */
  testSupermemory: () =>
    api.post<{ success: boolean; error?: string }>('/memory/supermemory/test'),
}

export default memoryApi
