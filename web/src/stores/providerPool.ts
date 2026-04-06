import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type {
  Provider,
  Model,
  UsageSummary,
  IDEInfo,
  PricingConfig,
  ModelPricing,
  ModelParams,
  RoutingMode,
  LocationStats,
  TrialQuotaStatus,
  OAuthQuotaInfo,
  ProviderAccountStatus,
  ProviderVerificationResult,
  VerifyProviderCandidateRequest,
} from '@/api/providerPool'
import type { MediaProviderConfig } from '@/api/mediaProviders'
import { filterProvidersVisibleInUI } from '@/utils/providerVisibility'

type ProviderPoolApiModule = typeof import('@/api/providerPool')
type MediaProvidersApiModule = typeof import('@/api/mediaProviders')

let providerPoolApiModulePromise: Promise<ProviderPoolApiModule> | null = null
let mediaProvidersApiModulePromise: Promise<MediaProvidersApiModule> | null = null

function createLazyApiProxy<T extends object>(load: () => Promise<T>): T {
  return new Proxy(
    {},
    {
      get(_target, prop) {
        return async (...args: unknown[]) => {
          const api = await load()
          const value = Reflect.get(api as object, prop)
          if (typeof value !== 'function') {
            return value
          }
          return Reflect.apply(value as (...callArgs: unknown[]) => unknown, api, args)
        }
      },
    }
  ) as T
}

async function loadProviderPoolApi() {
  if (!providerPoolApiModulePromise) {
    providerPoolApiModulePromise = import('@/api/providerPool')
  }
  return (await providerPoolApiModulePromise).providerPoolApi
}

async function loadMediaProviderApi() {
  if (!mediaProvidersApiModulePromise) {
    mediaProvidersApiModulePromise = import('@/api/mediaProviders')
  }
  return (await mediaProvidersApiModulePromise).mediaProviderApi
}

const providerPoolApi =
  createLazyApiProxy<ProviderPoolApiModule['providerPoolApi']>(loadProviderPoolApi)
const mediaProviderApi =
  createLazyApiProxy<MediaProvidersApiModule['mediaProviderApi']>(loadMediaProviderApi)

export const useProviderPoolStore = defineStore('providerPool', () => {
  // State
  const providers = ref<Provider[]>([])
  const models = ref<Model[]>([])
  const usageStats = ref<Record<string, UsageSummary>>({})
  const ides = ref<IDEInfo[]>([])
  const pricingConfig = ref<PricingConfig | null>(null)
  const customPricing = ref<ModelPricing[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const selectedProviderId = ref<string | null>(null)
  const routingMode = ref<RoutingMode>('auto')
  const locationStats = ref<LocationStats | null>(null)
  const trialQuota = ref<TrialQuotaStatus | null>(null)
  const oauthQuota = ref<Record<string, OAuthQuotaInfo>>({})
  const loadingQuota = ref<string | null>(null)
  const accountStatus = ref<Record<string, ProviderAccountStatus>>({})
  const loadingAccountStatus = ref<string | null>(null)

  // Computed
  const visibleProviders = computed(() => filterProvidersVisibleInUI(providers.value))

  const enabledProviders = computed(() => visibleProviders.value.filter((p) => p.enabled))

  const activeProviders = computed(() =>
    visibleProviders.value.filter((p) => p.enabled && p.status === 'active')
  )

  const builtinProviders = computed(() => visibleProviders.value.filter((p) => p.type === 'builtin'))

  const platformProviders = computed(() =>
    visibleProviders.value.filter((p) => p.type === 'platform')
  )

  const customProviders = computed(() => visibleProviders.value.filter((p) => p.type === 'custom'))

  const ideProviders = computed(() => visibleProviders.value.filter((p) => p.type === 'ide'))

  const trialProviders = computed(() => visibleProviders.value.filter((p) => p.type === 'trial'))

  const mediaProviders = computed(() => visibleProviders.value.filter((p) => p.type === 'media'))

  const oauthProviders = computed(() => providers.value.filter((p) => !!p.oauth))

  const cloudProviders = computed(() =>
    visibleProviders.value.filter((p) => p.enabled && p.location === 'cloud')
  )

  const localProviders = computed(() =>
    visibleProviders.value.filter((p) => p.enabled && p.location === 'local')
  )

  const hasCloudProviders = computed(() => cloudProviders.value.length > 0)
  const hasLocalProviders = computed(() => localProviders.value.length > 0)

  // Check if user has configured their own providers (non-trial)
  const hasUserConfiguredProviders = computed(() =>
    visibleProviders.value.some((p) => p.enabled && p.type !== 'trial')
  )

  const selectedProvider = computed(() =>
    providers.value.find((p) => p.id === selectedProviderId.value)
  )

  const providerModels = computed(() => {
    if (!selectedProviderId.value) return []
    return models.value.filter((m) => m.provider_id === selectedProviderId.value)
  })

  // Get provider display name by ID
  function getProviderDisplayName(providerId: string): string {
    const provider = providers.value.find((p) => p.id === providerId)
    return provider?.name || providerId
  }

  // Convert MediaProviderConfig to Provider for unified UI
  function mediaConfigToProvider(cfg: MediaProviderConfig): Provider {
    const provider: Provider = {
      id: cfg.id,
      name: cfg.name,
      type: 'media',
      location: 'cloud',
      enabled: cfg.enabled,
      status: cfg.enabled && cfg.has_api_key ? 'active' : 'inactive',
      base_url: cfg.base_url,
      icon: cfg.icon,
      description: cfg.description,
      website: cfg.website,
      api_key_url: cfg.api_key_url,
      priority: cfg.priority,
      api_keys: cfg.has_api_key
        ? [
            {
              id: 'default',
              key_hash: cfg.key_hash || '***',
              usage_count: 0,
              created_at: '',
              enabled: true,
            },
          ]
        : [],
      is_builtin: true,
    }
    // Map media models to provider models
    if (cfg.models?.length) {
      provider.models = cfg.models.map((m) => ({
        id: m.id,
        provider_id: cfg.id,
        name: m.name,
        display_name: m.name,
        enabled: true,
        capabilities: [
          m.type === 'video'
            ? 'video_generation'
            : m.type === 'image'
              ? 'image_generation'
              : m.type,
        ],
        price_per_request: m.price,
        pricing_unit: m.pricing_unit,
      }))
    }
    return provider
  }

  function mapMediaModelsToProviderModels(cfg: MediaProviderConfig): Model[] {
    if (!cfg.models?.length) return []
    return cfg.models.map((m) => ({
      id: m.id,
      provider_id: cfg.id,
      name: m.name,
      display_name: m.name,
      enabled: true,
      capabilities: [
        m.type === 'video' ? 'video_generation' : m.type === 'image' ? 'image_generation' : m.type,
      ],
      price_per_request: m.price,
      pricing_unit: m.pricing_unit,
    }))
  }

  function isMediaProvider(providerId: string): boolean {
    return providers.value.find((p) => p.id === providerId)?.type === 'media'
  }

  function syncMediaProviderModels(providerId: string, modelsForProvider: Model[]) {
    const provider = providers.value.find((p) => p.id === providerId)
    if (provider) {
      provider.models = [...modelsForProvider]
    }
    models.value = models.value.filter((m) => m.provider_id !== providerId)
    models.value.push(...modelsForProvider)
  }

  // Actions
  async function fetchProviders() {
    loading.value = true
    error.value = null
    try {
      const response = await providerPoolApi.listProviders()
      const llmProviders = response.data.providers || []

      // Also fetch media providers and merge
      let mediaAsProviders: Provider[] = []
      try {
        const mediaResp = await mediaProviderApi.list()
        const mediaConfigs = mediaResp.data.providers || []
        mediaAsProviders = mediaConfigs.map(mediaConfigToProvider)
      } catch {
        // Media API may not be available — ignore
      }

      providers.value = [...llmProviders, ...mediaAsProviders]

      // Populate models from inlined provider data
      const allModels: Model[] = []
      for (const p of providers.value) {
        if (p.models && p.models.length > 0) {
          allModels.push(...p.models)
        }
      }
      models.value = allModels
      // Also fetch trial quota
      fetchTrialQuota()
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch providers'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchTrialQuota() {
    try {
      const response = await providerPoolApi.getTrialQuota()
      trialQuota.value = response.data
    } catch {
      // Ignore errors - trial quota is optional
    }
  }

  async function fetchOAuthQuota(providerId: string) {
    loadingQuota.value = providerId
    try {
      const response = await providerPoolApi.getOAuthQuota(providerId)
      oauthQuota.value[providerId] = response.data
    } catch {
      // Silent fail - quota is optional info
    } finally {
      loadingQuota.value = null
    }
  }

  async function fetchAccountStatus(providerId: string, keyId?: string) {
    loadingAccountStatus.value = providerId
    try {
      const response = await providerPoolApi.getAccountStatus(providerId, keyId)
      accountStatus.value[providerId] = response.data
      return response.data
    } catch {
      return undefined
    } finally {
      loadingAccountStatus.value = null
    }
  }

  function clearAccountStatus(providerId: string) {
    delete accountStatus.value[providerId]
  }

  async function fetchModels(providerId?: string) {
    loading.value = true
    error.value = null
    try {
      if (providerId) {
        if (isMediaProvider(providerId)) {
          const response = await mediaProviderApi.get(providerId)
          const mediaModels = mapMediaModelsToProviderModels(response.data)
          syncMediaProviderModels(providerId, mediaModels)
          return
        }
        const response = await providerPoolApi.listProviderModels(providerId)
        const providerModels = response.data.models || []
        const provider = providers.value.find((p) => p.id === providerId)
        if (provider) {
          provider.models = [...providerModels]
        }
        // Update models for this provider
        models.value = models.value.filter((m) => m.provider_id !== providerId)
        models.value.push(...providerModels)
      } else {
        const response = await providerPoolApi.listAllModels()
        models.value = response.data.models || []
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch models'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function refreshModels(providerId: string) {
    // Don't set loading or clear error - this is a background refresh
    // that shouldn't block the UI
    try {
      if (isMediaProvider(providerId)) {
        const response = await mediaProviderApi.get(providerId)
        const mediaModels = mapMediaModelsToProviderModels(response.data)
        syncMediaProviderModels(providerId, mediaModels)
        return { success: true, models: mediaModels }
      }
      const response = await providerPoolApi.fetchProviderModels(providerId)
      // Update models for this provider
      models.value = models.value.filter((m) => m.provider_id !== providerId)
      models.value.push(...(response.data.models || []))
      return { success: true, models: response.data.models || [] }
    } catch (e) {
      // Return error instead of setting store.error to avoid blocking UI
      const errorMessage = e instanceof Error ? e.message : 'Failed to refresh models'
      return { success: false, error: errorMessage }
    }
  }

  async function probeModels(providerId: string, concurrency = 5) {
    try {
      if (isMediaProvider(providerId)) {
        const mediaModels = models.value.filter((m) => m.provider_id === providerId)
        return {
          success: true,
          total: mediaModels.length,
          available: mediaModels.length,
          unavailable: 0,
          results: [],
        }
      }
      const response = await providerPoolApi.probeProviderModels(providerId, concurrency)
      // After probing, refresh the model list to reflect enabled/disabled state
      await fetchModels(providerId)
      return {
        success: true,
        total: response.data.total,
        available: response.data.available,
        unavailable: response.data.unavailable,
        results: response.data.results,
      }
    } catch (e) {
      const errorMessage = e instanceof Error ? e.message : 'Failed to probe models'
      return {
        success: false,
        error: errorMessage,
        total: 0,
        available: 0,
        unavailable: 0,
        results: [],
      }
    }
  }

  async function fetchKeyModels(providerId: string, keyId: string) {
    try {
      const response = await providerPoolApi.fetchKeyModels(providerId, keyId)
      const keyModels = response.data.models || []
      // Update the provider's API key with its models
      const provider = providers.value.find((p) => p.id === providerId)
      if (provider && provider.api_keys) {
        const key = provider.api_keys.find((k) => k.id === keyId)
        if (key) {
          key.models = keyModels
          key.models_updated_at = new Date().toISOString()
        }
      }
      return { success: true, models: keyModels }
    } catch (e) {
      const errorMessage = e instanceof Error ? e.message : 'Failed to fetch key models'
      return { success: false, error: errorMessage, models: [] }
    }
  }

  async function listKeyModels(providerId: string, keyId: string) {
    try {
      const response = await providerPoolApi.listKeyModels(providerId, keyId)
      const keyModels = response.data.models || []
      // Update the provider's API key with its models
      const provider = providers.value.find((p) => p.id === providerId)
      if (provider && provider.api_keys) {
        const key = provider.api_keys.find((k) => k.id === keyId)
        if (key) {
          key.models = keyModels
        }
      }
      return { success: true, models: keyModels }
    } catch (e) {
      const errorMessage = e instanceof Error ? e.message : 'Failed to list key models'
      return { success: false, error: errorMessage, models: [] }
    }
  }

  async function addProvider(provider: Partial<Provider>) {
    loading.value = true
    error.value = null
    try {
      const response = await providerPoolApi.addProvider(provider)
      providers.value.push(response.data)
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to add provider'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function updateProvider(id: string, updates: Partial<Provider>) {
    loading.value = true
    error.value = null
    try {
      const response = await providerPoolApi.updateProvider(id, updates)
      const index = providers.value.findIndex((p) => p.id === id)
      if (index !== -1) {
        providers.value[index] = response.data
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to update provider'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Update provider priority locally (for drag-and-drop reordering)
  // This updates the frontend immediately and syncs to backend in background
  function updateProviderPriorityLocal(id: string, priority: number) {
    const provider = providers.value.find((p) => p.id === id)
    if (provider) {
      provider.priority = priority
    }
  }

  // Batch update priorities to backend (fire and forget)
  async function syncPrioritiesToBackend(updates: Array<{ id: string; priority: number }>) {
    const requests = updates.map(({ id, priority }) => {
      const provider = providers.value.find((p) => p.id === id)
      const request =
        provider?.type === 'media'
          ? mediaProviderApi.update(id, { priority })
          : providerPoolApi.updateProvider(id, { priority })

      return request.catch((err) => {
        console.error(`Failed to sync priority for ${id}:`, err)
      })
    })

    await Promise.allSettled(requests)
  }

  async function deleteProvider(id: string) {
    loading.value = true
    error.value = null
    try {
      const provider = providers.value.find((p) => p.id === id)
      if (provider?.type === 'media') {
        throw new Error('Media providers are not deletable')
      }
      await providerPoolApi.deleteProvider(id)
      providers.value = providers.value.filter((p) => p.id !== id)
      if (selectedProviderId.value === id) {
        selectedProviderId.value = null
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete provider'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function enableProvider(id: string) {
    try {
      const provider = providers.value.find((p) => p.id === id)
      if (provider?.type === 'media') {
        await mediaProviderApi.enable(id)
      } else {
        await providerPoolApi.enableProvider(id)
      }
      if (provider) {
        provider.enabled = true
        if (provider.type === 'media' && provider.api_keys?.length) {
          provider.status = 'active'
        }
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to enable provider'
      throw e
    }
  }

  async function disableProvider(id: string) {
    try {
      const provider = providers.value.find((p) => p.id === id)
      if (provider?.type === 'media') {
        await mediaProviderApi.disable(id)
      } else {
        await providerPoolApi.disableProvider(id)
      }
      if (provider) {
        provider.enabled = false
        provider.status = 'inactive'
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to disable provider'
      throw e
    }
  }

  async function clearProviderError(id: string) {
    try {
      const provider = providers.value.find((p) => p.id === id)
      if (provider?.type === 'media') {
        // Media providers don't have a clear-error endpoint
        // Just update local state
        if (provider) {
          provider.status = 'active'
          provider.last_error = ''
        }
        return
      }
      await providerPoolApi.clearError(id)
      if (provider) {
        provider.status = 'active'
        provider.last_error = ''
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to clear error'
      throw e
    }
  }

  async function verifyProviderCandidate(
    payload: VerifyProviderCandidateRequest
  ): Promise<ProviderVerificationResult> {
    try {
      const response = await providerPoolApi.verifyProviderCandidate(payload)
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to verify provider candidate'
      throw e
    }
  }

  async function verifyProviderRecommendation(providerId: string, apply = false, keyId?: string) {
    try {
      if (isMediaProvider(providerId)) {
        throw new Error('Provider verification is not available for media providers')
      }
      const response = await providerPoolApi.verifyProviderByID(providerId, {
        apply,
        key_id: keyId || undefined,
      })

      const updatedProvider = response.data.provider
      if (updatedProvider) {
        const index = providers.value.findIndex((p) => p.id === providerId)
        if (index !== -1) {
          providers.value[index] = {
            ...providers.value[index],
            ...updatedProvider,
          }
        }
      }

      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to verify provider'
      throw e
    }
  }

  async function updateModelParams(id: string, params: ModelParams) {
    try {
      const response = await providerPoolApi.updateModelParams(id, params)
      const provider = providers.value.find((p) => p.id === id)
      if (provider) {
        provider.model_params = response.data.model_params
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to update model params'
      throw e
    }
  }

  async function detectCapabilities(id: string) {
    try {
      const response = await providerPoolApi.detectCapabilities(id)
      const provider = providers.value.find((p) => p.id === id)
      if (provider) {
        if (!provider.model_params) {
          provider.model_params = {}
        }
        provider.model_params.detected_max_tokens = response.data.detected_max_tokens
        provider.model_params.detected_at = response.data.detected_at
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to detect capabilities'
      throw e
    }
  }

  async function updateAllowedModels(id: string, allowedModels: string[]) {
    try {
      const response = await providerPoolApi.updateAllowedModels(id, allowedModels)
      const provider = providers.value.find((p) => p.id === id)
      if (provider) {
        provider.allowed_models = response.data.allowed_models
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to update allowed models'
      throw e
    }
  }

  async function updateProviderIcon(id: string, icon: string) {
    try {
      const response = await providerPoolApi.updateProviderIcon(id, icon)
      const provider = providers.value.find((p) => p.id === id)
      if (provider) {
        provider.custom_icon = response.data.custom_icon
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to update provider icon'
      throw e
    }
  }

  async function deleteProviderIcon(id: string) {
    try {
      await providerPoolApi.deleteProviderIcon(id)
      const provider = providers.value.find((p) => p.id === id)
      if (provider) {
        provider.custom_icon = undefined
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete provider icon'
      throw e
    }
  }

  async function addAPIKey(providerId: string, key: string, label?: string) {
    try {
      const provider = providers.value.find((p) => p.id === providerId)
      if (provider?.type === 'media') {
        const resp = await mediaProviderApi.setKey(providerId, key)
        const cfg = resp.data
        const keyHash = cfg.key_hash || '***'
        if (provider) {
          provider.api_keys = [
            { id: 'default', key_hash: keyHash, usage_count: 0, created_at: '', enabled: true },
          ]
          if (provider.enabled) provider.status = 'active'
          // Update models from response
          if (cfg.models?.length) {
            provider.models = cfg.models.map((m) => ({
              id: m.id,
              provider_id: providerId,
              name: m.name,
              display_name: m.name,
              enabled: true,
              capabilities: [m.type],
            }))
          }
        }
        return { id: 'default', key_hash: keyHash, usage_count: 0, created_at: '', enabled: true }
      }
      const response = await providerPoolApi.addAPIKey(providerId, key, label)
      if (provider) {
        if (!provider.api_keys) {
          provider.api_keys = []
        }
        provider.api_keys.push(response.data)
        provider.enabled = true
      }
      // Backend auto-triggers model fetch for the new key.
      // Fetch key models after a short delay to let the backend finish.
      const keyId = response.data.id
      if (keyId) {
        setTimeout(() => fetchKeyModels(providerId, keyId), 2000)
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to add API key'
      throw e
    }
  }

  async function removeAPIKey(providerId: string, keyId: string) {
    try {
      const provider = providers.value.find((p) => p.id === providerId)
      if (provider?.type === 'media') {
        await mediaProviderApi.removeKey(providerId)
        if (provider) {
          provider.api_keys = []
          provider.status = 'inactive'
        }
        return
      }
      await providerPoolApi.removeAPIKey(providerId, keyId)
      if (provider && provider.api_keys) {
        provider.api_keys = provider.api_keys.filter((k) => k.id !== keyId)
        if (provider.api_keys.length === 0 && !provider.oauth?.connected) {
          provider.enabled = false
          provider.status = 'inactive'
        }
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to remove API key'
      throw e
    }
  }

  async function fetchUsageStats(period?: string) {
    try {
      const response = await providerPoolApi.getUsageStats(period)
      usageStats.value = response.data.providers
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch usage stats'
      throw e
    }
  }

  async function scanIDEs() {
    loading.value = true
    error.value = null
    try {
      const response = await providerPoolApi.scanIDEs()
      ides.value = response.data.ides
      return response.data.ides
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to scan IDEs'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function connectIDE(ideType: string) {
    try {
      const response = await providerPoolApi.connectIDE(ideType)
      const index = ides.value.findIndex((i) => i.type === ideType)
      if (index !== -1) {
        ides.value[index] = response.data
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to connect IDE'
      throw e
    }
  }

  function selectProvider(id: string | null) {
    selectedProviderId.value = id
  }

  function clearError() {
    error.value = null
  }

  // Pricing actions
  async function fetchPricingConfig() {
    try {
      const response = await providerPoolApi.getPricingConfig()
      pricingConfig.value = response.data
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch pricing config'
      throw e
    }
  }

  async function setDefaultPricing(inputPrice: number, outputPrice: number, cachePrice: number) {
    try {
      const response = await providerPoolApi.setDefaultPricing(inputPrice, outputPrice, cachePrice)
      if (pricingConfig.value) {
        pricingConfig.value.default_input_price = inputPrice
        pricingConfig.value.default_output_price = outputPrice
        pricingConfig.value.default_cache_price = cachePrice
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to set default pricing'
      throw e
    }
  }

  async function fetchCustomPricing() {
    try {
      const response = await providerPoolApi.listModelPricing()
      customPricing.value = response.data.pricing || []
      return response.data.pricing
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch custom pricing'
      throw e
    }
  }

  async function setModelPricing(
    modelId: string,
    pricing: {
      provider_id?: string
      input_price: number
      output_price: number
      cache_price?: number
    }
  ) {
    try {
      const response = await providerPoolApi.setModelPricing(modelId, pricing)
      // Update local state
      const key = pricing.provider_id ? `${pricing.provider_id}:${modelId}` : modelId
      const index = customPricing.value.findIndex(
        (p) => (p.provider_id ? `${p.provider_id}:${p.model_id}` : p.model_id) === key
      )
      if (index !== -1) {
        customPricing.value[index] = response.data
      } else {
        customPricing.value.push(response.data)
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to set model pricing'
      throw e
    }
  }

  async function removeModelPricing(modelId: string, providerId?: string) {
    try {
      await providerPoolApi.removeModelPricing(modelId, providerId)
      // Update local state
      const key = providerId ? `${providerId}:${modelId}` : modelId
      customPricing.value = customPricing.value.filter(
        (p) => (p.provider_id ? `${p.provider_id}:${p.model_id}` : p.model_id) !== key
      )
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to remove model pricing'
      throw e
    }
  }

  async function recalculateCosts(period?: string) {
    try {
      const response = await providerPoolApi.recalculateCosts(period)
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to recalculate costs'
      throw e
    }
  }

  // Routing mode actions
  async function fetchRoutingMode() {
    try {
      const response = await providerPoolApi.getRoutingMode()
      routingMode.value = response.data.mode
      return response.data.mode
    } catch {
      // Default to auto if fetch fails
      routingMode.value = 'auto'
      return 'auto'
    }
  }

  async function setRoutingMode(mode: RoutingMode) {
    try {
      const response = await providerPoolApi.setRoutingMode(mode)
      routingMode.value = response.data.mode
      return response.data.mode
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to set routing mode'
      throw e
    }
  }

  async function fetchLocationStats() {
    try {
      const response = await providerPoolApi.getLocationStats()
      locationStats.value = response.data
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch location stats'
      throw e
    }
  }

  /** Update a single provider's status in-place (called from SSE events). */
  function updateProviderStatus(providerId: string, status: string) {
    const provider = providers.value.find((p) => p.id === providerId)
    if (provider) {
      provider.status = status as Provider['status']
    }
  }

  return {
    // State
    providers,
    models,
    usageStats,
    ides,
    pricingConfig,
    customPricing,
    loading,
    error,
    selectedProviderId,
    routingMode,
    locationStats,
    trialQuota,
    oauthQuota,
    loadingQuota,
    accountStatus,
    loadingAccountStatus,

    // Computed
    enabledProviders,
    activeProviders,
    builtinProviders,
    platformProviders,
    customProviders,
    ideProviders,
    trialProviders,
    mediaProviders,
    oauthProviders,
    cloudProviders,
    localProviders,
    hasCloudProviders,
    hasLocalProviders,
    hasUserConfiguredProviders,
    selectedProvider,
    providerModels,

    // Helpers
    getProviderDisplayName,

    // Actions
    fetchProviders,
    fetchModels,
    refreshModels,
    probeModels,
    fetchKeyModels,
    listKeyModels,
    addProvider,
    updateProvider,
    updateProviderPriorityLocal,
    syncPrioritiesToBackend,
    deleteProvider,
    enableProvider,
    disableProvider,
    clearProviderError,
    verifyProviderCandidate,
    verifyProviderRecommendation,
    updateModelParams,
    detectCapabilities,
    updateAllowedModels,
    updateProviderIcon,
    deleteProviderIcon,
    addAPIKey,
    removeAPIKey,
    fetchUsageStats,
    scanIDEs,
    connectIDE,
    selectProvider,
    clearError,
    fetchPricingConfig,
    setDefaultPricing,
    fetchCustomPricing,
    setModelPricing,
    removeModelPricing,
    recalculateCosts,
    fetchRoutingMode,
    setRoutingMode,
    fetchLocationStats,
    fetchTrialQuota,
    fetchOAuthQuota,
    fetchAccountStatus,
    clearAccountStatus,
    updateProviderStatus,
  }
})
