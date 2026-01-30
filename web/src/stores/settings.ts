import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type { ProviderInfo, ToolDefinition } from '@/api/chat'
import { providerApi, toolApi } from '@/api/chat'

const STORAGE_KEY = 'zimaos-echo-settings'

interface StoredSettings {
  selectedProvider: string
  selectedModel: string
  temperature: number
  maxTokens: number
  apiKeys: Record<string, string>
  baseUrls: Record<string, string>
  // Store last used model per provider
  lastModelPerProvider: Record<string, string>
}

function loadStoredSettings(): Partial<StoredSettings> {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      return JSON.parse(stored)
    }
  } catch {
    // Ignore parse errors
  }
  return {}
}

function saveSettings(settings: StoredSettings) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(settings))
}

export const useSettingsStore = defineStore('settings', () => {
  const stored = loadStoredSettings()

  // State
  const providers = ref<ProviderInfo[]>([])
  const tools = ref<ToolDefinition[]>([])
  const selectedProvider = ref(stored.selectedProvider || 'claude')
  const selectedModel = ref(stored.selectedModel || 'claude-opus-4-5-20251101')
  const temperature = ref(stored.temperature ?? 0.7)
  const maxTokens = ref(stored.maxTokens ?? 2048)
  const apiKeys = ref<Record<string, string>>(stored.apiKeys || {})
  const baseUrls = ref<Record<string, string>>(stored.baseUrls || {})
  const lastModelPerProvider = ref<Record<string, string>>(stored.lastModelPerProvider || {})
  const loading = ref(false)
  const refreshing = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const currentProvider = computed(() =>
    providers.value.find((p) => p.name === selectedProvider.value)
  )

  const availableModels = computed(() => currentProvider.value?.models || [])

  const hasApiKey = computed(() => {
    const provider = selectedProvider.value
    // Ollama doesn't need an API key
    if (provider === 'ollama') return true
    return !!apiKeys.value[provider]
  })

  // Watch for changes and persist
  watch(
    [selectedProvider, selectedModel, temperature, maxTokens, apiKeys, baseUrls, lastModelPerProvider],
    () => {
      saveSettings({
        selectedProvider: selectedProvider.value,
        selectedModel: selectedModel.value,
        temperature: temperature.value,
        maxTokens: maxTokens.value,
        apiKeys: apiKeys.value,
        baseUrls: baseUrls.value,
        lastModelPerProvider: lastModelPerProvider.value,
      })
    },
    { deep: true }
  )

  // Update lastModelPerProvider when model changes
  watch(selectedModel, (newModel) => {
    if (newModel && selectedProvider.value) {
      lastModelPerProvider.value = {
        ...lastModelPerProvider.value,
        [selectedProvider.value]: newModel,
      }
    }
  })

  // Actions
  async function fetchProviders() {
    try {
      loading.value = true
      error.value = null
      const response = await providerApi.list()
      // Handle both array response and object response with providers key
      const data = response.data
      providers.value = Array.isArray(data) ? data : (data?.providers || [])

      // Set default provider if current one is not available
      if (providers.value.length > 0) {
        const providerNames = providers.value.map((p) => p.name)
        if (!providerNames.includes(selectedProvider.value)) {
          // Prefer claude if available, otherwise use first provider
          const claudeProvider = providers.value.find((p) => p.name === 'claude')
          if (claudeProvider) {
            selectedProvider.value = 'claude'
          } else {
            const firstProvider = providers.value[0]
            if (firstProvider) {
              selectedProvider.value = firstProvider.name
            }
          }
        }

        // Set default model - prefer last used model for this provider
        const currentModels = currentProvider.value?.models || []
        if (currentModels.length > 0) {
          const lastUsedModel = lastModelPerProvider.value[selectedProvider.value]
          if (lastUsedModel && currentModels.includes(lastUsedModel)) {
            // Use last used model for this provider
            selectedModel.value = lastUsedModel
          } else if (!currentModels.includes(selectedModel.value)) {
            // Current model not available, use first model
            const firstModel = currentModels[0]
            if (firstModel) {
              selectedModel.value = firstModel
            }
          }
        }
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch providers'
    } finally {
      loading.value = false
    }
  }

  // Refresh models for a specific provider (useful for Ollama)
  async function refreshProviderModels(providerName?: string) {
    const targetProvider = providerName || selectedProvider.value
    try {
      refreshing.value = true
      error.value = null
      const response = await providerApi.refresh(targetProvider)

      // Update the provider's models in the list
      const index = providers.value.findIndex((p) => p.name === targetProvider)
      if (index !== -1) {
        providers.value[index] = response.data
      }

      // If refreshing current provider, check if selected model is still valid
      if (targetProvider === selectedProvider.value) {
        const currentModels = response.data.models || []
        if (currentModels.length > 0 && !currentModels.includes(selectedModel.value)) {
          // Try to use last used model, otherwise use first
          const lastUsedModel = lastModelPerProvider.value[targetProvider]
          if (lastUsedModel && currentModels.includes(lastUsedModel)) {
            selectedModel.value = lastUsedModel
          } else {
            const firstModel = currentModels[0]
            if (firstModel) {
              selectedModel.value = firstModel
            }
          }
        }
      }

      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to refresh models'
      throw e
    } finally {
      refreshing.value = false
    }
  }

  async function fetchTools() {
    try {
      const response = await toolApi.list()
      tools.value = response.data
    } catch (e) {
      console.error('Failed to fetch tools:', e)
    }
  }

  function setProvider(provider: string) {
    selectedProvider.value = provider
    // When changing provider, try to use last used model for that provider
    const models = providers.value.find((p) => p.name === provider)?.models || []
    if (models.length > 0) {
      const lastUsedModel = lastModelPerProvider.value[provider]
      if (lastUsedModel && models.includes(lastUsedModel)) {
        selectedModel.value = lastUsedModel
      } else {
        const firstModel = models[0]
        if (firstModel) {
          selectedModel.value = firstModel
        }
      }
    }
  }

  function setModel(model: string) {
    selectedModel.value = model
  }

  function setTemperature(value: number) {
    temperature.value = Math.max(0, Math.min(2, value))
  }

  function setMaxTokens(value: number) {
    maxTokens.value = Math.max(1, Math.min(128000, value))
  }

  function setApiKey(provider: string, key: string) {
    apiKeys.value = { ...apiKeys.value, [provider]: key }
  }

  function clearApiKey(provider: string) {
    const newKeys = { ...apiKeys.value }
    delete newKeys[provider]
    apiKeys.value = newKeys
  }

  function setBaseUrl(provider: string, url: string) {
    baseUrls.value = { ...baseUrls.value, [provider]: url }
  }

  function clearBaseUrl(provider: string) {
    const newUrls = { ...baseUrls.value }
    delete newUrls[provider]
    baseUrls.value = newUrls
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    providers,
    tools,
    selectedProvider,
    selectedModel,
    temperature,
    maxTokens,
    apiKeys,
    baseUrls,
    lastModelPerProvider,
    loading,
    refreshing,
    error,

    // Computed
    currentProvider,
    availableModels,
    hasApiKey,

    // Actions
    fetchProviders,
    refreshProviderModels,
    fetchTools,
    setProvider,
    setModel,
    setTemperature,
    setMaxTokens,
    setApiKey,
    clearApiKey,
    setBaseUrl,
    clearBaseUrl,
    clearError,
  }
})
