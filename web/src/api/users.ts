import api from './client'

// Types
export interface User {
  id: string
  username: string
  email?: string
  role: 'admin' | 'user' | 'guest'
  status: 'active' | 'locked' | 'disabled'
  mfa_enabled: boolean
  last_login_at?: string
  created_at: string
  updated_at: string
}

export interface CreateUserRequest {
  username: string
  email?: string
  password: string
  role?: 'user' | 'guest'
  permissions?: string[]
}

export interface UpdateUserRequest {
  email?: string
  role?: 'user' | 'guest'
  status?: 'active' | 'locked' | 'disabled'
  permissions?: string[]
}

export interface ResetPasswordRequest {
  new_password: string
}

export interface ListUsersParams {
  page?: number
  page_size?: number
  search?: string
  role?: string
  status?: string
  sort_by?: string
  sort_dir?: 'asc' | 'desc'
}

export interface ListUsersResponse {
  users: User[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface PermissionInfo {
  key: string
  name: string
  description: string
  category: string
}

export interface PermissionsResponse {
  user_id: string
  role: string
  permissions: string[]
}

// User Management API
export const usersApi = {
  // List users with pagination
  list: (params?: ListUsersParams) =>
    api.get<ListUsersResponse>('/users', { params }),

  // Get user by ID
  get: (id: string) => api.get<User>(`/users/${id}`),

  // Create new user
  create: (data: CreateUserRequest) => api.post<User>('/users', data),

  // Update user
  update: (id: string, data: UpdateUserRequest) =>
    api.put<User>(`/users/${id}`, data),

  // Delete user
  delete: (id: string) => api.delete<void>(`/users/${id}`),

  // Lock user
  lock: (id: string) => api.post<void>(`/users/${id}/lock`),

  // Unlock user
  unlock: (id: string) => api.post<void>(`/users/${id}/unlock`),

  // Reset user password (admin only)
  resetPassword: (id: string, data: ResetPasswordRequest) =>
    api.post<void>(`/users/${id}/reset-password`, data),
}

// Permissions API
export const permissionsApi = {
  // Get current user's permissions
  getMyPermissions: () => api.get<PermissionsResponse>('/users/me/permissions'),

  // Get user's permissions (admin only)
  getUserPermissions: (id: string) =>
    api.get<PermissionsResponse>(`/users/${id}/permissions`),

  // Set user's permissions (admin only)
  setUserPermissions: (id: string, permissions: string[]) =>
    api.put<PermissionsResponse>(`/users/${id}/permissions`, { permissions }),

  // Get available permissions (admin only)
  getAvailablePermissions: () =>
    api.get<{ permissions: PermissionInfo[] }>('/permissions/available'),
}

// Page permission constants (must match backend)
export const PagePermissions = {
  CHAT: 'page.chat',
  HOME: 'page.home',
  CHANNELS: 'page.channels',
  SETTINGS: 'page.settings',
  SECURITY: 'page.security',
  USERS: 'page.users',
  PROFILE: 'page.profile',
  PROVIDERS: 'page.providers',
  AUTOMATION: 'page.automation',
  PLUGINS: 'page.plugins',
  TOOLS: 'page.tools',
  SKILLS: 'page.skills',
} as const

export type PagePermission = (typeof PagePermissions)[keyof typeof PagePermissions]
