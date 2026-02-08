import api from './client'

export interface Personality {
  id: string
  name: string
  description: string
  system_prompt: string
  is_active: boolean
  traits: PersonalityTrait[]
  created_at: string
  updated_at: string
}

export interface PersonalityTrait {
  key: string
  value: string
  weight: number
}

export const personalityApi = {
  list: () => api.get<Personality[]>('/personalities'),

  get: (id: string) => api.get<Personality>(`/personalities/${id}`),

  create: (data: { name: string; description: string; system_prompt: string }) =>
    api.post<Personality>('/personalities', data),

  update: (id: string, data: { name: string; description: string; system_prompt: string }) =>
    api.put<Personality>(`/personalities/${id}`, data),

  delete: (id: string) => api.delete<{ message: string }>(`/personalities/${id}`),

  activate: (id: string) => api.post<{ message: string }>(`/personalities/${id}/activate`),

  getActive: () => api.get<Personality>('/personalities/active'),
}
