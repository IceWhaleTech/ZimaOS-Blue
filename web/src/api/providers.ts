import api from './client'

// Provider configuration response
export interface ProviderConfigResponse {
  name: string
  api_key?: string
  base_url?: string
  enabled: boolean
  has_api_key: boolean
  default_url?: string
}

// Provider configuration update request
export interface ProviderConfigRequest {
  api_key?: string
  base_url?: string
  enabled?: boolean
}

// Test connection response
export interface TestConnectionResponse {
  success: boolean
  message: string
  messageKey?: string
}

// Provider settings API
export const providerSettingsApi = {
  // List all provider configurations
  list: () => api.get<ProviderConfigResponse[]>('/providers/settings'),

  // Get configuration for a specific provider
  get: (name: string) => api.get<ProviderConfigResponse>(`/providers/settings/${name}`),

  // Update configuration for a specific provider
  update: (name: string, config: ProviderConfigRequest) =>
    api.put<ProviderConfigResponse>(`/providers/settings/${name}`, config),

  // Test connection for a provider
  test: (name: string) =>
    api.post<TestConnectionResponse>(`/providers/settings/${name}/test`),
}
