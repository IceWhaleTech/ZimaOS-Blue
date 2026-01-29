import api from './client'

// Types
export interface SetupStatus {
  completed: boolean
  current_step: number
  total_steps: number
  cli_installed: boolean
  cli_version?: string
  ollama_detected: boolean
  ollama_endpoint?: string
  providers_configured: string[]
  statistics_opt_in: boolean
  completed_at?: string
}

export interface ProviderDetectionResult {
  ollama: {
    available: boolean
    endpoint?: string
    version?: string
    models?: string[]
  }
  anthropic: {
    configured: boolean
    masked_key?: string
  }
  openai: {
    configured: boolean
    masked_key?: string
  }
  cc_switch: {
    available: boolean
    active_profile?: string
    profiles?: string[]
  }
}

export interface CLIStatus {
  installed: boolean
  version?: string
  path?: string
  source?: 'embedded' | 'downloaded' | 'system'
  update_available: boolean
  latest_version?: string
}

export interface CLIDownloadInfo {
  version: string
  size: number
  size_human: string
  download_url: string
  checksum: string
  platform: string
}

export interface CLIDownloadProgress {
  downloaded: number
  total: number
  percentage: number
  speed: number
  speed_human: string
  eta: string
  started_at: string
}

export interface FeatureInfo {
  name: string
  description: string
  enabled: boolean
  requires_cli: boolean
  category: string
}

export interface FeatureStatus {
  cli_installed: boolean
  total_features: number
  enabled_count: number
  disabled_count: number
  features: Record<string, FeatureInfo>
  disabled_reasons: Record<string, string>
}

export interface ConsentStatus {
  consented: boolean
  consented_at?: string
  can_revoke: boolean
}

export interface UsageStats {
  total_calls: number
  calls_by_provider: Record<string, number>
  calls_by_model: Record<string, number>
  input_tokens: number
  output_tokens: number
  error_count: number
  estimated_cost_usd: number
  period_start: string
  period_end: string
}

// Setup API
export const setupApi = {
  // Get setup status
  getStatus: () => api.get<SetupStatus>('/setup/status'),

  // Get defaults for setup wizard
  getDefaults: () => api.get<{
    language: string
    languages: { code: string; name: string }[]
    providers: string[]
  }>('/setup/defaults'),

  // Validate a setup step
  validateStep: (step: number, config: Record<string, unknown>) =>
    api.post<{ valid: boolean; errors: Record<string, string> }>('/setup/validate', { step, config }),

  // Test connection
  testConnection: (type: string, config: Record<string, string>) =>
    api.post<{ success: boolean; message: string }>('/setup/test-connection', { type, config }),

  // Complete setup
  complete: (config: Record<string, unknown>) =>
    api.post<{ success: boolean; message?: string }>('/setup/complete', config),

  // Reset setup
  reset: () => api.post('/setup/reset'),

  // Skip CLI download
  skipCLI: () => api.post('/first-run/skip-cli'),

  // Check username availability
  checkUsername: (username: string) =>
    api.post<{ available: boolean; message?: string }>('/setup/check-username', { username }),
}

// Provider Detection API
export const providerDetectionApi = {
  // Detect all providers
  detectAll: () => api.get<ProviderDetectionResult>('/providers/detect'),

  // Get Ollama status
  getOllamaStatus: () => api.get<{
    available: boolean
    endpoint?: string
    version?: string
  }>('/providers/ollama'),

  // Get Ollama models
  getOllamaModels: () => api.get<{ models: string[] }>('/providers/ollama/models'),

  // Get environment config
  getEnvConfig: () => api.get<{
    anthropic_key_set: boolean
    openai_key_set: boolean
    http_proxy?: string
    https_proxy?: string
  }>('/env/config'),

  // Get cc-switch profiles
  getCCSwitchProfiles: () => api.get<{
    available: boolean
    active_profile?: string
    profiles: { name: string; provider: string; model: string }[]
  }>('/cc-switch/profiles'),

  // Activate cc-switch profile
  activateCCSwitchProfile: (profile: string) =>
    api.post<{ success: boolean; message?: string }>('/cc-switch/activate', { profile }),
}

// CLI Download API
export const cliDownloadApi = {
  // Get CLI status
  getStatus: () => api.get<CLIStatus>('/cli/status'),

  // Get download info
  getDownloadInfo: () => api.get<CLIDownloadInfo>('/cli/download/info'),

  // Start download
  startDownload: () => api.post<{ success: boolean; message?: string }>('/cli/download'),

  // Get download progress
  getProgress: () => api.get<CLIDownloadProgress>('/cli/download/progress'),

  // Cancel download
  cancelDownload: () => api.delete('/cli/download'),

  // Get CLI config
  getConfig: () => api.get<{
    enabled: boolean
    features: Record<string, boolean>
  }>('/cli/config'),

  // Update CLI config
  updateConfig: (config: { enabled?: boolean; features?: Record<string, boolean> }) =>
    api.put('/cli/config', config),

  // Enable CLI
  enable: () => api.post<{ success: boolean; warning?: string }>('/cli/enable'),

  // Disable CLI
  disable: () => api.post<{ success: boolean; features_disabled: string[] }>('/cli/disable'),

  // Get feature matrix
  getFeatureMatrix: () => api.get<{
    cli_enabled: boolean
    features: Record<string, { enabled: boolean; requires_cli: boolean }>
  }>('/cli/feature-matrix'),
}

// Features API
export const featuresApi = {
  // Get all features
  getAll: () => api.get<Record<string, FeatureInfo>>('/features'),

  // Get feature status
  getStatus: () => api.get<FeatureStatus>('/features/status'),

  // Get specific feature
  getFeature: (name: string) => api.get<FeatureInfo>(`/features/${name}`),

  // Get CLI-dependent features
  getCLIDependent: () => api.get<FeatureInfo[]>('/features/cli-dependent'),

  // Get features by category
  getByCategory: () => api.get<Record<string, FeatureInfo[]>>('/features/by-category'),
}

// Statistics API
export const statisticsApi = {
  // Get usage statistics
  getStats: () => api.get<UsageStats>('/stats'),

  // Get recent events
  getRecentEvents: (limit?: number) =>
    api.get<{ events: unknown[] }>('/stats/recent', { params: { limit } }),

  // Get consent status
  getConsentStatus: () => api.get<ConsentStatus>('/stats/consent'),

  // Set consent status
  setConsentStatus: (consented: boolean) =>
    api.post<{ success: boolean }>('/stats/consent', { consented }),

  // Get consent information
  getConsentInfo: () => api.get<{
    what_we_collect: string[]
    what_we_dont_collect: string[]
    how_we_use: string[]
    data_retention: string
  }>('/stats/consent/info'),

  // Export statistics
  exportStats: (format: 'json' | 'csv' = 'json') =>
    api.get('/stats/export', { params: { format }, responseType: 'blob' }),

  // Clear statistics
  clearStats: () => api.delete('/stats'),
}

export default {
  setup: setupApi,
  providerDetection: providerDetectionApi,
  cliDownload: cliDownloadApi,
  features: featuresApi,
  statistics: statisticsApi,
}
