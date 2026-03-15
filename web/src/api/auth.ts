import api from './client'

// Types
export interface User {
  id: string
  username: string
  email?: string
  role: string
  created_at: string
  updated_at: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface AuthTokenResponse {
  token: string
  refresh_token: string
  expires_at: string
  user: User
}

export interface ApiKey {
  id: string
  name: string
  key_prefix: string
  scopes: string[]
  created_at: string
  expires_at?: string
  last_used_at?: string
}

export interface CreateApiKeyRequest {
  name: string
  scopes: string[]
  expires_in?: string
}

export interface CreateApiKeyResponse {
  api_key: ApiKey
  key: string // Full key, only shown once
}

export interface PasswordPolicy {
  min_length: number
  require_uppercase: boolean
  require_lowercase: boolean
  require_letter: boolean
  require_number: boolean
  require_special: boolean
}

// Auth API
export const authApi = {
  login: (data: LoginRequest) => api.post<AuthTokenResponse>('/auth/login', data),

  logout: () => {
    const refreshToken = localStorage.getItem('refresh_token')
    return api.post<{ success: boolean }>('/auth/logout', null, {
      headers: refreshToken ? { 'X-Refresh-Token': refreshToken } : {},
    })
  },

  refresh: (refreshToken: string) =>
    api.post<AuthTokenResponse>('/auth/refresh', { refresh_token: refreshToken }),

  me: () => api.get<User>('/users/me'),

  updateProfile: (data: { email?: string; password?: string }) => api.put<User>('/users/me', data),

  getPasswordPolicy: () => api.get<PasswordPolicy>('/auth/password-policy'),
}

// API Keys API
export const apiKeyApi = {
  list: () => api.get<ApiKey[]>('/apikeys'),

  create: (data: CreateApiKeyRequest) => api.post<CreateApiKeyResponse>('/apikeys', data),

  delete: (id: string) => api.delete<{ success: boolean }>(`/apikeys/${id}`),

  regenerate: (id: string) => api.post<CreateApiKeyResponse>(`/apikeys/${id}/regenerate`),
}
