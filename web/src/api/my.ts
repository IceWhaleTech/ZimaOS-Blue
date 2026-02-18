import api from './client'

// Types
export interface UserProviderConfig {
  provider_name: string
  has_api_key: boolean
  api_key?: string
  base_url?: string
  enabled: boolean
  configured: boolean
}

export interface UserSkillConfig {
  id: string
  name: string
  summary: string
  category: string
  author: string
  enabled: boolean
  installed: boolean
  user_toggled: boolean
}

export interface UserTokenUsage {
  user_id: string
  input_tokens: number
  output_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  estimated_cost: number
  request_count: number
}

// API functions
export const myApi = {
  // Providers
  listProviders: () => api.get<UserProviderConfig[]>('/my/providers'),
  getProvider: (name: string) => api.get<UserProviderConfig>(`/my/providers/${name}`),
  updateProvider: (name: string, data: { api_key?: string; base_url?: string; enabled?: boolean }) =>
    api.put<UserProviderConfig>(`/my/providers/${name}`, data),
  deleteProvider: (name: string) => api.delete(`/my/providers/${name}`),
  testProvider: (name: string) => api.post<{ success: boolean; messageKey: string; models?: string[] }>(`/my/providers/${name}/test`),

  // Skills
  listSkills: () => api.get<UserSkillConfig[]>('/my/skills'),
  toggleSkill: (id: string, enabled: boolean) => api.put(`/my/skills/${id}`, { enabled }),

  // Usage
  getUsage: () => api.get<UserTokenUsage>('/my/usage'),
}
