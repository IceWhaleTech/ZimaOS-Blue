import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type { ToolDefinition } from '@/api/chat'
import { toolApi } from '@/api/chat'
import { providerPoolApi, type Provider, type Model } from '@/api/providerPool'

const STORAGE_KEY = 'zimaos-echo-settings'

// Provider info for Chat page (simplified view of Provider Pool data)
export interface ChatProviderInfo {
  id: string        // Provider ID from Provider Pool
  name: string      // Display name
  models: string[]  // Model IDs
}

// Combined option for single dropdown: Provider(model)
export interface ProviderModelOption {
  value: string       // Format: "providerId:modelId"
  label: string       // Format: "ProviderName(modelId)"
  providerId: string
  providerName: string
  modelId: string
}

// Theme style types
export type ThemeStyle = 'default' | 'bubble' | 'minimal' | 'gradient' | 'ocean'
export type CloseBehavior = 'quit' | 'minimize'

export const THEME_STYLES: { id: ThemeStyle; labelKey: string }[] = [
  { id: 'default', labelKey: 'theme.styles.default' },
  { id: 'bubble', labelKey: 'theme.styles.bubble' },
  { id: 'minimal', labelKey: 'theme.styles.minimal' },
  { id: 'gradient', labelKey: 'theme.styles.gradient' },
  { id: 'ocean', labelKey: 'theme.styles.ocean' },
]

interface StoredSettings {
  selectedProviderModel: string  // Format: "providerId:modelId"
  temperature: number
  maxTokens: number
  themeStyle: ThemeStyle
  closeBehavior: CloseBehavior
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
  const providers = ref<ChatProviderInfo[]>([])
  const tools = ref<ToolDefinition[]>([])
  const selectedProviderModel = ref(stored.selectedProviderModel || '')  // Format: "providerId:modelId"
  const temperature = ref(stored.temperature ?? 0.7)
  const maxTokens = ref(stored.maxTokens ?? 2048)
  const themeStyle = ref<ThemeStyle>(stored.themeStyle || 'default')
  const closeBehavior = ref<CloseBehavior>(stored.closeBehavior || 'quit')
  const loading = ref(false)
  const refreshing = ref(false)
  const error = ref<string | null>(null)

  // Computed: All provider-model options for dropdown
  const providerModelOptions = computed<ProviderModelOption[]>(() => {
    const options: ProviderModelOption[] = []
    for (const provider of providers.value) {
      for (const modelId of provider.models) {
        options.push({
          value: `${provider.id}:${modelId}`,
          label: `${provider.name}(${modelId})`,
          providerId: provider.id,
          providerName: provider.name,
          modelId: modelId,
        })
      }
    }
    return options
  })

  // Computed: Parse selected provider and model from combined value
  const selectedProvider = computed(() => {
    const parts = selectedProviderModel.value.split(':')
    return parts[0] || ''
  })

  const selectedModel = computed(() => {
    const parts = selectedProviderModel.value.split(':')
    return parts.slice(1).join(':') || ''  // Handle model IDs that contain ':'
  })

  const currentProvider = computed(() =>
    providers.value.find((p) => p.id === selectedProvider.value)
  )

  const availableModels = computed(() => currentProvider.value?.models || [])

  // Watch for changes and persist
  watch(
    [selectedProviderModel, temperature, maxTokens, themeStyle, closeBehavior],
    () => {
      saveSettings({
        selectedProviderModel: selectedProviderModel.value,
        temperature: temperature.value,
        maxTokens: maxTokens.value,
        themeStyle: themeStyle.value,
        closeBehavior: closeBehavior.value,
      })
    },
    { deep: true }
  )

  // Actions
  async function fetchProviders() {
    try {
      loading.value = true
      error.value = null

      // Fetch providers from Provider Pool
      const response = await providerPoolApi.listProviders()
      const poolProviders = response.data.providers || []

      // Filter enabled providers and convert to ChatProviderInfo
      const enabledProviders = poolProviders.filter((p: Provider) => p.enabled)

      // Fetch models for each enabled provider
      const providerInfos: ChatProviderInfo[] = []
      for (const provider of enabledProviders) {
        try {
          const modelsResponse = await providerPoolApi.listProviderModels(provider.id)
          const models = modelsResponse.data.models || []
          providerInfos.push({
            id: provider.id,
            name: provider.name,
            models: models.filter((m: Model) => m.enabled).map((m: Model) => m.id),
          })
        } catch {
          // If fetching models fails, still add provider with empty models
          providerInfos.push({
            id: provider.id,
            name: provider.name,
            models: [],
          })
        }
      }

      providers.value = providerInfos

      // Set default selection if current one is not available
      if (providerModelOptions.value.length > 0) {
        const currentOption = providerModelOptions.value.find(
          (opt) => opt.value === selectedProviderModel.value
        )
        if (!currentOption) {
          // Prefer anthropic provider if available
          const anthropicOption = providerModelOptions.value.find(
            (opt) => opt.providerId === 'anthropic'
          )
          if (anthropicOption) {
            selectedProviderModel.value = anthropicOption.value
          } else {
            // Use first available option
            const firstOption = providerModelOptions.value[0]
            if (firstOption) {
              selectedProviderModel.value = firstOption.value
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

  // Refresh models for a specific provider
  async function refreshProviderModels(providerId?: string) {
    const targetProviderId = providerId || selectedProvider.value
    try {
      refreshing.value = true
      error.value = null

      // Fetch models from Provider Pool
      const response = await providerPoolApi.fetchProviderModels(targetProviderId)
      const models = response.data.models || []
      const modelIds = models.filter((m: Model) => m.enabled).map((m: Model) => m.id)

      // Update the provider's models in the list
      const index = providers.value.findIndex((p) => p.id === targetProviderId)
      if (index !== -1) {
        const existingProvider = providers.value[index]
        if (existingProvider) {
          providers.value[index] = {
            id: existingProvider.id,
            name: existingProvider.name,
            models: modelIds,
          }
        }
      }

      // If current selection is no longer valid, select first model of this provider
      if (targetProviderId === selectedProvider.value) {
        if (modelIds.length > 0 && !modelIds.includes(selectedModel.value)) {
          const firstModel = modelIds[0]
          if (firstModel) {
            selectedProviderModel.value = `${targetProviderId}:${firstModel}`
          }
        }
      }

      return { id: targetProviderId, models: modelIds }
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

  // Set combined provider:model selection
  function setProviderModel(value: string) {
    selectedProviderModel.value = value
  }

  // Set provider (keeps current model if available, otherwise selects first model)
  function setProvider(providerId: string) {
    const provider = providers.value.find(p => p.id === providerId)
    if (provider && provider.models.length > 0) {
      // Try to keep current model if it exists in new provider
      const currentModel = selectedModel.value
      if (provider.models.includes(currentModel)) {
        selectedProviderModel.value = `${providerId}:${currentModel}`
      } else {
        // Select first model of new provider
        const firstModel = provider.models[0]
        if (firstModel) {
          selectedProviderModel.value = `${providerId}:${firstModel}`
        }
      }
    }
  }

  // Set model (keeps current provider)
  function setModel(modelId: string) {
    const currentProviderId = selectedProvider.value
    if (currentProviderId) {
      selectedProviderModel.value = `${currentProviderId}:${modelId}`
    }
  }

  function setTemperature(value: number) {
    temperature.value = Math.max(0, Math.min(2, value))
  }

  function setMaxTokens(value: number) {
    maxTokens.value = Math.max(1, Math.min(128000, value))
  }

  function setThemeStyle(style: ThemeStyle) {
    themeStyle.value = style
  }

  function setCloseBehavior(behavior: CloseBehavior) {
    closeBehavior.value = behavior
  }

  function clearError() {
    error.value = null
  }

  function resetToDefaults() {
    temperature.value = 0.7
    maxTokens.value = 2048
    themeStyle.value = 'default'
    closeBehavior.value = 'quit'
    // Clear stored settings
    localStorage.removeItem(STORAGE_KEY)
  }

  return {
    // State
    providers,
    tools,
    selectedProviderModel,
    temperature,
    maxTokens,
    themeStyle,
    closeBehavior,
    loading,
    refreshing,
    error,

    // Computed
    providerModelOptions,
    selectedProvider,
    selectedModel,
    currentProvider,
    availableModels,

    // Actions
    fetchProviders,
    refreshProviderModels,
    fetchTools,
    setProviderModel,
    setProvider,
    setModel,
    setTemperature,
    setMaxTokens,
    setThemeStyle,
    setCloseBehavior,
    clearError,
    resetToDefaults,
  }
})
