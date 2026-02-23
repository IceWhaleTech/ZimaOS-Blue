import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { providerPoolApi, type Provider, type Model, type UsageSummary, type IDEInfo, type PricingConfig, type ModelPricing, type ModelParams, type RoutingMode, type LocationStats, type TrialQuotaStatus } from '@/api/providerPool'

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

  // Computed
  const enabledProviders = computed(() =>
    providers.value.filter(p => p.enabled)
  )

  const activeProviders = computed(() =>
    providers.value.filter(p => p.enabled && p.status === 'active')
  )

  const builtinProviders = computed(() =>
    providers.value.filter(p => p.type === 'builtin')
  )

  const platformProviders = computed(() =>
    providers.value.filter(p => p.type === 'platform')
  )

  const customProviders = computed(() =>
    providers.value.filter(p => p.type === 'custom')
  )

  const ideProviders = computed(() =>
    providers.value.filter(p => p.type === 'ide')
  )

  const trialProviders = computed(() =>
    providers.value.filter(p => p.type === 'trial')
  )

  const mediaProviders = computed(() =>
    providers.value.filter(p => p.type === 'media')
  )

  const oauthProviders = computed(() =>
    providers.value.filter(p => p.oauth?.connected)
  )

  const cloudProviders = computed(() =>
    providers.value.filter(p => p.enabled && p.location === 'cloud')
  )

  const localProviders = computed(() =>
    providers.value.filter(p => p.enabled && p.location === 'local')
  )

  const hasCloudProviders = computed(() => cloudProviders.value.length > 0)
  const hasLocalProviders = computed(() => localProviders.value.length > 0)

  // Check if user has configured their own providers (non-trial)
  const hasUserConfiguredProviders = computed(() =>
    providers.value.some(p => p.enabled && p.type !== 'trial')
  )

  const selectedProvider = computed(() =>
    providers.value.find(p => p.id === selectedProviderId.value)
  )

  const providerModels = computed(() => {
    if (!selectedProviderId.value) return []
    return models.value.filter(m => m.provider_id === selectedProviderId.value)
  })

  // Get provider display name by ID
  function getProviderDisplayName(providerId: string): string {
    const provider = providers.value.find(p => p.id === providerId)
    return provider?.name || providerId
  }

  // Actions
  async function fetchProviders() {
    loading.value = true
    error.value = null
    try {
      const response = await providerPoolApi.listProviders()
      providers.value = response.data.providers || []
      // Populate models from inlined provider data
      const allModels: Model[] = []
      for (const p of providers.value) {
        if (p.models && p.models.length > 0) {
          allModels.push(...p.models)
        }
      }
      if (allModels.length > 0) {
        models.value = allModels
      }
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

  async function fetchModels(providerId?: string) {
    loading.value = true
    error.value = null
    try {
      if (providerId) {
        const response = await providerPoolApi.listProviderModels(providerId)
        // Update models for this provider
        models.value = models.value.filter(m => m.provider_id !== providerId)
        models.value.push(...(response.data.models || []))
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
      const response = await providerPoolApi.fetchProviderModels(providerId)
      // Update models for this provider
      models.value = models.value.filter(m => m.provider_id !== providerId)
      models.value.push(...(response.data.models || []))
      return { success: true, models: response.data.models || [] }
    } catch (e) {
      // Return error instead of setting store.error to avoid blocking UI
      const errorMessage = e instanceof Error ? e.message : 'Failed to refresh models'
      return { success: false, error: errorMessage }
    }
  }

  async function probeModels(providerId: string) {
    try {
      const response = await providerPoolApi.probeProviderModels(providerId)
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
      return { success: false, error: errorMessage, total: 0, available: 0, unavailable: 0, results: [] }
    }
  }

  async function fetchKeyModels(providerId: string, keyId: string) {
    try {
      const response = await providerPoolApi.fetchKeyModels(providerId, keyId)
      const keyModels = response.data.models || []
      // Update the provider's API key with its models
      const provider = providers.value.find(p => p.id === providerId)
      if (provider && provider.api_keys) {
        const key = provider.api_keys.find(k => k.id === keyId)
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
      const provider = providers.value.find(p => p.id === providerId)
      if (provider && provider.api_keys) {
        const key = provider.api_keys.find(k => k.id === keyId)
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
      const index = providers.value.findIndex(p => p.id === id)
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
    const provider = providers.value.find(p => p.id === id)
    if (provider) {
      provider.priority = priority
    }
  }

  // Batch update priorities to backend (fire and forget)
  async function syncPrioritiesToBackend(updates: Array<{ id: string; priority: number }>) {
    // Update backend in background without blocking UI
    for (const { id, priority } of updates) {
      providerPoolApi.updateProvider(id, { priority }).catch(err => {
        console.error(`Failed to sync priority for ${id}:`, err)
      })
    }
  }

  async function deleteProvider(id: string) {
    loading.value = true
    error.value = null
    try {
      await providerPoolApi.deleteProvider(id)
      providers.value = providers.value.filter(p => p.id !== id)
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
      await providerPoolApi.enableProvider(id)
      const provider = providers.value.find(p => p.id === id)
      if (provider) {
        provider.enabled = true
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to enable provider'
      throw e
    }
  }

  async function disableProvider(id: string) {
    try {
      await providerPoolApi.disableProvider(id)
      const provider = providers.value.find(p => p.id === id)
      if (provider) {
        provider.enabled = false
        provider.status = 'inactive'
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to disable provider'
      throw e
    }
  }

  async function testProvider(id: string, keyId?: string) {
    try {
      const response = await providerPoolApi.testProvider(id, keyId)
      const provider = providers.value.find(p => p.id === id)
      if (provider) {
        provider.status = response.data.healthy ? 'active' : 'error'
        provider.last_health_check = response.data.checked_at
        if (response.data.error) {
          provider.last_error = response.data.error
        }
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to test provider'
      throw e
    }
  }

  async function updateModelParams(id: string, params: ModelParams) {
    try {
      const response = await providerPoolApi.updateModelParams(id, params)
      const provider = providers.value.find(p => p.id === id)
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
      const provider = providers.value.find(p => p.id === id)
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
      const provider = providers.value.find(p => p.id === id)
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
      const provider = providers.value.find(p => p.id === id)
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
      const provider = providers.value.find(p => p.id === id)
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
      const response = await providerPoolApi.addAPIKey(providerId, key, label)
      const provider = providers.value.find(p => p.id === providerId)
      if (provider) {
        if (!provider.api_keys) {
          provider.api_keys = []
        }
        provider.api_keys.push(response.data)
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
      await providerPoolApi.removeAPIKey(providerId, keyId)
      const provider = providers.value.find(p => p.id === providerId)
      if (provider && provider.api_keys) {
        provider.api_keys = provider.api_keys.filter(k => k.id !== keyId)
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
      const index = ides.value.findIndex(i => i.type === ideType)
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

  async function setModelPricing(modelId: string, pricing: { provider_id?: string; input_price: number; output_price: number; cache_price?: number }) {
    try {
      const response = await providerPoolApi.setModelPricing(modelId, pricing)
      // Update local state
      const key = pricing.provider_id ? `${pricing.provider_id}:${modelId}` : modelId
      const index = customPricing.value.findIndex(p =>
        (p.provider_id ? `${p.provider_id}:${p.model_id}` : p.model_id) === key
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
      customPricing.value = customPricing.value.filter(p =>
        (p.provider_id ? `${p.provider_id}:${p.model_id}` : p.model_id) !== key
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
    testProvider,
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
  }
})
