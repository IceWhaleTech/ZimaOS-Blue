import api from './client'

// Types
export interface ModelParams {
  temperature?: number
  max_tokens?: number
  top_p?: number
  frequency_penalty?: number
  presence_penalty?: number
  detected_max_tokens?: number
  detected_at?: number
}

export type ProviderLocation = 'cloud' | 'local'
export type RoutingMode = 'auto' | 'cloud' | 'local'
export type APIFormat = 'openai' | 'anthropic' | 'ollama' | 'google' | ''

export interface Provider {
  id: string
  name: string
  type: 'builtin' | 'custom' | 'acp' | 'ide' | 'trial'
  location: ProviderLocation
  enabled: boolean
  status: 'active' | 'inactive' | 'error'
  base_url?: string
  api_version?: string
  api_format?: APIFormat
  api_keys?: APIKey[]
  priority: number
  model_params?: ModelParams
  allowed_models?: string[]
  icon?: string
  custom_icon?: string
  description?: string
  website?: string
  created_at: string
  updated_at: string
  last_health_check?: string
  last_error?: string
}

export interface APIKey {
  id: string
  key_hash: string
  label?: string
  usage_count: number
  last_used?: string
  created_at: string
  enabled: boolean
}

export interface Model {
  id: string
  provider_id: string
  name: string
  display_name: string
  enabled: boolean
  capabilities: ModelCapabilities
  input_price?: number
  output_price?: number
  context_window?: number
  max_output?: number
  description?: string
}

export interface ModelCapabilities {
  chat: boolean
  completion: boolean
  vision: boolean
  function_call: boolean
  streaming: boolean
  thinking: boolean
  json: boolean
  system_prompt: boolean
}

export interface HealthCheckResult {
  provider_id: string
  healthy: boolean
  latency: number
  error?: string
  checked_at: string
}

export interface UsageSummary {
  provider_id?: string
  model_id?: string
  period: string
  total_input_tokens: number
  total_output_tokens: number
  total_requests: number
  successful_requests: number
  failed_requests: number
  total_estimated_cost: number
  avg_latency_ms: number
}

export interface TrialQuotaStatus {
  tokens_used: number
  tokens_remaining: number
  token_limit: number
  conversations_used: number
  conversations_left: number
  conversation_limit: number
  exhausted: boolean
  exhausted_by_tokens: boolean
}

export interface IDEInfo {
  type: string
  name: string
  version?: string
  config_path: string
  proxy_url?: string
  api_key?: string // Masked key for display
  connected: boolean
  models?: string[]
  last_checked: string
  error?: string
  usage?: IDEUsageInfo
}

export interface IDEScanResult {
  ide_type: string
  ide_name: string
  found: boolean
  config_path?: string
  searched_paths?: string[]
  error?: string
}

export interface IDEUsageInfo {
  total_tokens?: number
  input_tokens?: number
  output_tokens?: number
  total_requests?: number
  estimated_cost?: number
  remaining_credits?: number
  usage_limit?: number
  usage_period?: string
  last_updated?: string
}

export interface ImportConfig {
  ide_type: string
  ide_name: string
  api_key?: string // Masked
  base_url?: string
  models?: string[]
  provider?: string
  config_path?: string
  env_var?: string
  source: 'config' | 'env' | 'cc-switch' | 'extension'
  can_import: boolean
  extension_config?: ClaudeCodeExtConfig
}

export interface ClaudeCodeExtConfig {
  env_vars: ClaudeCodeEnvVar[]
  selected_model?: string
}

export interface ClaudeCodeEnvVar {
  name: string
  value: string // Masked for display
}

export interface EnvHint {
  ide: string
  name: string
  env_vars: string[]
  provider: string
  detected: boolean
}

export interface ModelPricing {
  model_id: string
  provider_id?: string
  input_price: number
  output_price: number
  cache_price?: number
  is_custom: boolean
  updated_at: string
}

export interface PricingConfig {
  default_input_price: number
  default_output_price: number
  default_cache_price: number
  custom_pricing: Record<string, ModelPricing>
  updated_at: string
}

export interface LocationStats {
  cloud_count: number
  local_count: number
  cloud_providers: string[]
  local_providers: string[]
  has_cloud: boolean
  has_local: boolean
}

// Failover types
export interface FailoverMetrics {
  errors_by_type: Record<string, number>
  failover_total: number
  failover_success: number
  failover_failure: number
  provider_errors: Record<string, Record<string, number>>
  provider_failovers: Record<string, number>
  stream_anomalies: number
}

export interface ErrorClassification {
  type: string
  category: string
  message: string
  retryable: boolean
  should_failover: boolean
  suggested_context_window?: number
  retry_after?: number
  original_status_code: number
}

export interface FailoverConfig {
  enabled: boolean
  max_retries: number
  retry_delay: string
  circuit_breaker: boolean
  failure_threshold: number
  recovery_timeout: string
  error_classification: {
    enabled: boolean
    failover_errors: string[]
    retryable_errors: string[]
  }
  streaming_anomaly: {
    enabled: boolean
    window_size: number
    repeat_threshold: number
    min_pattern_length: number
    max_pattern_length: number
    recovery_strategy: string
  }
}

// API functions
export const providerPoolApi = {
  // Provider operations
  listProviders: () =>
    api.get<{ providers: Provider[]; total: number }>('/providers'),

  getProvider: (id: string) =>
    api.get<{ provider: Provider; health?: HealthCheckResult }>(`/providers/${id}`),

  addProvider: (provider: Partial<Provider>) =>
    api.post<Provider>('/providers', provider),

  updateProvider: (id: string, updates: Partial<Provider>) =>
    api.put<Provider>(`/providers/${id}`, updates),

  deleteProvider: (id: string) =>
    api.delete(`/providers/${id}`),

  enableProvider: (id: string) =>
    api.post<{ status: string }>(`/providers/${id}/enable`),

  disableProvider: (id: string) =>
    api.post<{ status: string }>(`/providers/${id}/disable`),

  testProvider: (id: string) =>
    api.post<HealthCheckResult>(`/providers/${id}/test`),

  updateModelParams: (id: string, params: ModelParams) =>
    api.put<{ message: string; model_params: ModelParams }>(`/providers/${id}/params`, params),

  updateAllowedModels: (id: string, allowedModels: string[]) =>
    api.put<{ message: string; allowed_models: string[] }>(`/providers/${id}/allowed-models`, { allowed_models: allowedModels }),

  detectCapabilities: (id: string) =>
    api.post<{ message: string; detected_max_tokens?: number; detected_at?: number }>(`/providers/${id}/detect`),

  updateProviderIcon: (id: string, icon: string) =>
    api.put<{ message: string; custom_icon: string }>(`/providers/${id}/icon`, { icon }),

  deleteProviderIcon: (id: string) =>
    api.delete(`/providers/${id}/icon`),

  // Model operations
  listProviderModels: (providerId: string) =>
    api.get<{ models: Model[]; total: number }>(`/providers/${providerId}/models`),

  fetchProviderModels: (providerId: string) =>
    api.post<{ models: Model[]; total: number }>(`/providers/${providerId}/models/fetch`),

  listAllModels: () =>
    api.get<{ models: Model[]; total: number }>('/models'),

  // API Key operations
  addAPIKey: (providerId: string, key: string, label?: string) =>
    api.post<APIKey>(`/providers/${providerId}/keys`, { key, label }),

  removeAPIKey: (providerId: string, keyId: string) =>
    api.delete(`/providers/${providerId}/keys/${keyId}`),

  // Usage operations
  getUsageStats: (period?: string) =>
    api.get<{ current: Record<string, UsageSummary>; providers: Record<string, UsageSummary> }>(
      '/providers/usage',
      { params: { period } }
    ),

  getProviderUsage: (providerId: string) =>
    api.get<{ summary: UsageSummary; models: Record<string, UsageSummary> }>(
      `/providers/${providerId}/usage`
    ),

  // Trial quota
  getTrialQuota: () =>
    api.get<TrialQuotaStatus>('/providers/trial/quota'),

  // IDE operations
  scanIDEs: () =>
    api.get<{ ides: IDEInfo[]; scan_results: IDEScanResult[]; total: number }>('/ide/scan'),

  connectIDE: (ideType: string) =>
    api.post<IDEInfo>(`/ide/${ideType}/connect`),

  getImportableConfigs: () =>
    api.get<{ configs: ImportConfig[]; total: number }>('/ide/importable'),

  importIDEConfig: (ideType: string) =>
    api.post<{ message: string; provider_id: string; ide_type: string }>(`/ide/import/${ideType}`),

  importExtensionConfig: (ideType: string) =>
    api.post<{ message: string; providers: string[]; ide_type: string }>(`/ide/import-ext/${ideType}`),

  importFromCCSwitch: (output: string) =>
    api.post<{ config: ImportConfig; message: string }>('/ide/import-cc-switch', { output }),

  getEnvHints: () =>
    api.get<{ hints: EnvHint[] }>('/ide/env-hints'),

  // Pricing operations
  getPricingConfig: () =>
    api.get<PricingConfig>('/pricing'),

  setDefaultPricing: (inputPrice: number, outputPrice: number, cachePrice: number) =>
    api.put<{ message: string; input_price: number; output_price: number; cache_price: number }>(
      '/pricing/default',
      { input_price: inputPrice, output_price: outputPrice, cache_price: cachePrice }
    ),

  listModelPricing: () =>
    api.get<{ pricing: ModelPricing[]; total: number }>('/pricing/models'),

  setModelPricing: (modelId: string, pricing: { provider_id?: string; input_price: number; output_price: number; cache_price?: number }) =>
    api.put<ModelPricing>(`/pricing/models/${encodeURIComponent(modelId)}`, pricing),

  removeModelPricing: (modelId: string, providerId?: string) =>
    api.delete(`/pricing/models/${encodeURIComponent(modelId)}`, { params: { provider_id: providerId } }),

  recalculateCosts: (period?: string) =>
    api.post<{
      period: string
      records_processed: number
      old_total_cost: number
      new_total_cost: number
      cost_difference: number
      message: string
    }>('/pricing/recalculate', null, { params: { period } }),

  // Failover operations
  getFailoverMetrics: () =>
    api.get<FailoverMetrics>('/proxy/failover/metrics'),

  getFailoverConfig: () =>
    api.get<FailoverConfig>('/proxy/failover/config'),

  updateFailoverConfig: (config: Partial<FailoverConfig>) =>
    api.put<FailoverConfig>('/proxy/failover/config', config),

  resetCircuitBreakers: () =>
    api.post<{ message: string }>('/proxy/failover/reset'),

  getCircuitBreakerStatus: () =>
    api.get<Record<string, { state: string; failures: number; last_failure?: string }>>('/proxy/failover/breakers'),

  // Config operations
  getRoutingMode: () =>
    api.get<{ mode: RoutingMode }>('/config/routing-mode'),

  setRoutingMode: (mode: RoutingMode) =>
    api.put<{ mode: RoutingMode }>('/config/routing-mode', { mode }),

  getLocationStats: () =>
    api.get<LocationStats>('/config/location-stats'),
}

export default providerPoolApi
