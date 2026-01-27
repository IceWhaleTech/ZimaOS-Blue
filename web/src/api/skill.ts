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
  author?: string
  category?: string
  tags?: string[]
  source_id: string
  source_name: string
  download_url?: string
  homepage?: string
  stars?: number
  downloads?: number
  installed: boolean
}

export interface BrowseParams {
  source?: string
  category?: string
  search?: string
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

  // Skill store
  listSources: () => api.get<SkillSource[]>('/skill-store/sources'),

  addSource: (source: Omit<SkillSource, 'enabled'> & { enabled?: boolean }) =>
    api.post<{ success: boolean; message: string }>('/skill-store/sources', source),

  removeSource: (id: string) =>
    api.delete<{ success: boolean; message: string }>(`/skill-store/sources/${id}`),

  browse: (params?: BrowseParams) =>
    api.get<RemoteSkill[]>('/skill-store/browse', { params }),

  install: (id: string) =>
    api.post<{ success: boolean; message: string; skill?: RemoteSkill }>(`/skill-store/install/${id}`),

  uninstall: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/skill-store/uninstall/${id}`),

  refresh: () =>
    api.post<{ success: boolean; skills_count: number; errors?: string[] }>('/skill-store/refresh'),
}
