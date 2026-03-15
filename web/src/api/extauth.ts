import api from './client'

// Types
export interface ProviderInfo {
  id: string
  name: string
  type: ProviderType
  icon_url?: string
  order: number
}

export type ProviderType =
  | 'generic'
  | 'google'
  | 'github'
  | 'microsoft'
  | 'keycloak'
  | 'authentik'
  | 'auth0'

export interface LinkedAccount {
  id: string
  user_id: string
  provider_id: string
  provider_user_id: string
  email?: string
  name?: string
  picture?: string
  token_expiry?: string
  created_at: string
  updated_at: string
}

export interface AuthorizeResponse {
  auth_url: string
  state: string
}

export interface CallbackResponse {
  user: AuthenticatedUser
  access_token: string
  refresh_token?: string
  expires_in: number
  is_new_user: boolean
}

export interface AuthenticatedUser {
  id: string
  email: string
  name: string
  picture?: string
  roles: string[]
  provider_id: string
}

export interface ProviderConfig {
  id: string
  name: string
  type: ProviderType
  enabled: boolean
  client_id: string
  client_secret?: string
  issuer_url?: string
  auth_url?: string
  token_url?: string
  userinfo_url?: string
  scopes?: string[]
  redirect_url: string
  auto_create_user: boolean
  default_role: string
  allowed_domains?: string[]
  icon_url?: string
  order: number
}

export interface CreateProviderRequest {
  id: string
  name: string
  type: ProviderType
  client_id: string
  client_secret: string
  issuer_url?: string
  auth_url?: string
  token_url?: string
  userinfo_url?: string
  scopes?: string[]
  redirect_url: string
  auto_create_user?: boolean
  default_role?: string
  allowed_domains?: string[]
  icon_url?: string
  order?: number
}

export interface UpdateProviderRequest {
  name?: string
  enabled?: boolean
  client_id?: string
  client_secret?: string
  issuer_url?: string
  auth_url?: string
  token_url?: string
  userinfo_url?: string
  scopes?: string[]
  redirect_url?: string
  auto_create_user?: boolean
  default_role?: string
  allowed_domains?: string[]
  icon_url?: string
  order?: number
}

// External Auth API
export const extauthApi = {
  // List all enabled providers (public)
  listProviders: () => api.get<ProviderInfo[]>('/auth/providers'),

  // Start OAuth flow - returns redirect URL
  authorize: (provider: string, redirectUri?: string) => {
    const params = new URLSearchParams()
    if (redirectUri) {
      params.set('redirect_uri', redirectUri)
    }
    const query = params.toString()
    return api.get<AuthorizeResponse>(`/auth/oidc/${provider}/authorize${query ? `?${query}` : ''}`)
  },

  // Exchange code for tokens (used after callback)
  exchangeToken: (provider: string, code: string, state: string) =>
    api.post<CallbackResponse>(`/auth/oidc/${provider}/token`, { code, state }),

  // Get linked accounts for current user
  getLinkedAccounts: () => api.get<LinkedAccount[]>('/auth/linked'),

  // Link an external account
  linkAccount: (provider: string, redirectUri?: string) => {
    const params = new URLSearchParams()
    if (redirectUri) {
      params.set('redirect_uri', redirectUri)
    }
    const query = params.toString()
    return api.post<AuthorizeResponse>(`/auth/link/${provider}${query ? `?${query}` : ''}`)
  },

  // Unlink an external account
  unlinkAccount: (provider: string) => api.delete<{ status: string }>(`/auth/link/${provider}`),
}

// Admin API for managing providers
export const extauthAdminApi = {
  // List all providers (including disabled)
  listAllProviders: () => api.get<ProviderConfig[]>('/admin/auth/providers'),

  // Get provider config
  getProvider: (id: string) => api.get<ProviderConfig>(`/admin/auth/providers/${id}`),

  // Create provider
  createProvider: (data: CreateProviderRequest) =>
    api.post<ProviderConfig>('/admin/auth/providers', data),

  // Update provider
  updateProvider: (id: string, data: UpdateProviderRequest) =>
    api.put<ProviderConfig>(`/admin/auth/providers/${id}`, data),

  // Delete provider
  deleteProvider: (id: string) => api.delete<{ success: boolean }>(`/admin/auth/providers/${id}`),

  // Enable/disable provider
  toggleProvider: (id: string, enabled: boolean) =>
    api.patch<ProviderConfig>(`/admin/auth/providers/${id}`, { enabled }),
}

// Helper to get provider icon
export function getProviderIcon(type: ProviderType): string {
  const icons: Record<ProviderType, string> = {
    google: 'https://www.google.com/favicon.ico',
    github: 'https://github.com/favicon.ico',
    microsoft: 'https://www.microsoft.com/favicon.ico',
    keycloak: '',
    authentik: '',
    auth0: '',
    generic: '',
  }
  return icons[type] || ''
}

// Helper to get provider display name
export function getProviderDisplayName(type: ProviderType): string {
  const names: Record<ProviderType, string> = {
    google: 'Google',
    github: 'GitHub',
    microsoft: 'Microsoft',
    keycloak: 'Keycloak',
    authentik: 'Authentik',
    auth0: 'Auth0',
    generic: 'OIDC',
  }
  return names[type] || type
}
