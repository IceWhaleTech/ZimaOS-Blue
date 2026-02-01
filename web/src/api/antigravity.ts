import api from './index'

// Types
export interface ModelQuota {
  name: string
  percentage: number
  reset_time?: string
  group: 'gemini' | 'claude'
}

export interface QuotaData {
  models: ModelQuota[]
  last_updated: number
  subscription_tier?: string
  error?: string
}

export interface AntigravityQuotaResponse {
  success: boolean
  data?: QuotaData
  error?: string
}

export interface APIKeyInfo {
  provider: string
  env_var?: string
  source: 'env' | 'ide-config' | 'file'
  ide_name?: string
  configured: boolean
  masked_value?: string
}

export interface DetectedKeysResponse {
  keys: APIKeyInfo[]
}

// API functions
export const antigravityApi = {
  /**
   * Get Antigravity quota information
   * @param accessToken - Antigravity access token
   */
  getQuota(accessToken: string) {
    return api.post<AntigravityQuotaResponse>('/claudecode/antigravity/quota', {
      access_token: accessToken
    })
  },

  /**
   * Get detected API keys from environment and IDE configs
   */
  getDetectedKeys() {
    return api.get<DetectedKeysResponse>('/claudecode/env-keys')
  }
}
