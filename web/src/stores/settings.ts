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
  const selectedProvider = ref(stored.selectedProvider || 'openai')
  const selectedModel = ref(stored.selectedModel || 'gpt-4o-mini')
  const temperature = ref(stored.temperature ?? 0.7)
  const maxTokens = ref(stored.maxTokens ?? 2048)
  const apiKeys = ref<Record<string, string>>(stored.apiKeys || {})
  const loading = ref(false)
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
    [selectedProvider, selectedModel, temperature, maxTokens, apiKeys],
    () => {
      saveSettings({
        selectedProvider: selectedProvider.value,
        selectedModel: selectedModel.value,
        temperature: temperature.value,
        maxTokens: maxTokens.value,
        apiKeys: apiKeys.value,
      })
    },
    { deep: true }
  )

  // Actions
  async function fetchProviders() {
    try {
      loading.value = true
      error.value = null
      const response = await providerApi.list()
      providers.value = response.data

      // Set default provider if current one is not available
      if (providers.value.length > 0) {
        const providerNames = providers.value.map((p) => p.name)
        if (!providerNames.includes(selectedProvider.value)) {
          selectedProvider.value = providers.value[0].name
        }

        // Set default model if current one is not available
        const currentModels = currentProvider.value?.models || []
        if (currentModels.length > 0 && !currentModels.includes(selectedModel.value)) {
          selectedModel.value = currentModels[0]
        }
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch providers'
    } finally {
      loading.value = false
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
    // Reset model when provider changes
    const models = providers.value.find((p) => p.name === provider)?.models || []
    if (models.length > 0) {
      selectedModel.value = models[0]
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
    loading,
    error,

    // Computed
    currentProvider,
    availableModels,
    hasApiKey,

    // Actions
    fetchProviders,
    fetchTools,
    setProvider,
    setModel,
    setTemperature,
    setMaxTokens,
    setApiKey,
    clearApiKey,
    clearError,
  }
})
