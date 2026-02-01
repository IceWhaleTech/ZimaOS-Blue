import api from './client'

// Types
export interface SkillParameter {
  name: string
  type: string
  description?: string
  required?: boolean
  default?: unknown
}

export interface Skill {
  id: string
  name: string
  version: string
  description: string
  author?: string
  category?: string
  icon?: string
  tags?: string[]
  enabled: boolean
  builtin: boolean
  inputs?: SkillParameter[]
  outputs?: SkillParameter[]
}

export interface SkillSource {
  id: string
  name: string
  url: string
  type: 'clawdhub' | 'github' | 'custom'
  description?: string
  enabled: boolean
}

export interface RemoteSkill {
  id: string
  name: string
  version: string
  description: string
  summary?: string
  author?: string
  category?: string
  tags?: string[]
  source_id: string
  source_name: string
  download_url?: string
  homepage?: string
  source_url?: string
  stars?: number
  downloads?: number
  reviews?: number
  rating?: number
  versions?: number
  changelog?: string
  readme?: string
  dedup_key?: string
  installed: boolean
  builtin?: boolean
  created_at?: string
  updated_at?: string
  synced_at?: string
}

export interface LocalSkill {
  id: string
  name: string
  description: string
  version?: string
  author?: string
  category?: string
  tags?: string[]
  file_path: string
  discovered_at: string
  last_modified: string
  installed: boolean
  builtin?: boolean
}

export interface BrowseParams {
  source?: string
  category?: string
  search?: string
  page?: number
  page_size?: number
}

export interface SearchParams {
  q?: string
  categories?: string
  sources?: string
  min_stars?: number
  sort_by?: 'relevance' | 'stars' | 'downloads' | 'updated' | 'name'
  sort_order?: 'asc' | 'desc'
  page?: number
  page_size?: number
  cursor?: string
  count?: number
}

export interface SearchResult {
  id: string
  name: string
  version: string
  summary: string
  description: string
  author: string
  category: string
  tags: string
  source_id: string
  source_name: string
  homepage: string
  download_url: string
  stars: number
  downloads: number
  reviews: number
  rating: number
  versions: number
  changelog: string
  installed: boolean
  enabled: boolean
  created_at: string
  updated_at: string
  synced_at: string
  score: number
}

export interface SearchResponse {
  skills: SearchResult[]
  total: number
  page: number
  page_size: number
  total_pages: number
  next_cursor?: string
  has_more: boolean
}

export interface SyncStatus {
  id: number
  source_id: string
  last_sync_at: string
  skill_count: number
  sync_duration_ms: number
  status: 'success' | 'failed' | 'in_progress' | 'pending'
  error_message?: string
  next_sync_at: string
}

export interface BrowseResponse {
  skills: RemoteSkill[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface VerifyResponse {
  id: string
  visible: boolean
  enabled?: boolean
  builtin?: boolean
  name?: string
  version?: string
  description?: string
  error?: string
}

export interface LocalSkillsResponse {
  skills: LocalSkill[]
  count: number
}

export interface InstallFromURLRequest {
  url: string
  name?: string
  description?: string
}

// Skill API
export const skillApi = {
  // Local skills
  list: () => api.get<Skill[]>('/skills'),

  get: (id: string) => api.get<Skill>(`/skills/${id}`),

  enable: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/skills/${id}/enable`),

  disable: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/skills/${id}/disable`),

  // Local skill discovery (v0.10.8)
  listLocal: () => api.get<LocalSkillsResponse>('/skills/local'),

  scanLocal: () =>
    api.post<{ success: boolean; skills_found: number }>('/skills/local/scan'),

  verify: (id: string) => api.get<VerifyResponse>(`/skills/verify/${id}`),

  // Skill store
  listSources: () => api.get<SkillSource[]>('/skill-store/sources'),

  addSource: (source: Omit<SkillSource, 'enabled'> & { enabled?: boolean }) =>
    api.post<{ success: boolean; message: string }>('/skill-store/sources', source),

  removeSource: (id: string) =>
    api.delete<{ success: boolean; message: string }>(`/skill-store/sources/${id}`),

  browse: (params?: BrowseParams) =>
    api.get<BrowseResponse>('/skill-store/browse', { params }),

  // Featured skills (v0.10.8)
  featured: (category?: string) =>
    api.get<RemoteSkill[]>('/skill-store/featured', { params: category ? { category } : undefined }),

  install: (id: string) =>
    api.post<{ success: boolean; message: string; skill?: RemoteSkill }>(`/skill-store/install/${id}`),

  // Install from URL (v0.10.8)
  installFromURL: (req: InstallFromURLRequest) =>
    api.post<{ success: boolean; skill?: { id: string; name: string; version: string; description: string } }>('/skill-store/install-url', req),

  uninstall: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/skill-store/uninstall/${id}`),

  refresh: () =>
    api.post<{ success: boolean; skills_count: number; errors?: string[] }>('/skill-store/refresh'),

  // Sync (v0.10.8)
  sync: (sourceId?: string) =>
    api.post<{ success: boolean; message: string }>('/skill-store/sync', null, { params: sourceId ? { source: sourceId } : undefined }),

  // Upload skill package
  upload: (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post<{ success: boolean; message?: string; skill?: { id: string; name: string; version: string } }>('/skills/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  // Stats and categories
  categories: () => api.get<string[]>('/skill-store/categories'),

  stats: () => api.get<{ total_skills: number; installed: number; by_source: Record<string, number> }>('/skill-store/stats'),

  // Search (v0.10.14)
  search: (params?: SearchParams) =>
    api.get<SearchResponse>('/skill-store/search', { params }),

  // Popular and recent (v0.10.14)
  popular: (limit?: number) =>
    api.get<RemoteSkill[]>('/skill-store/popular', { params: limit ? { limit } : undefined }),

  recent: (limit?: number) =>
    api.get<RemoteSkill[]>('/skill-store/recent', { params: limit ? { limit } : undefined }),

  // Sync status (v0.10.14)
  syncStatus: () => api.get<SyncStatus[]>('/skill-store/sync-status'),
}
