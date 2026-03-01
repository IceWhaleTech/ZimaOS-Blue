import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type { ToolDefinition } from '@/api/chat'
import { toolApi } from '@/api/chat'
import { providerPoolApi, type Provider, type Model } from '@/api/providerPool'
import {
  settingsApi,
  type Settings,
  type NoLLMDegradeMode,
  type SmallModelID,
  type SmallModelRuntime,
  type SmallModelStatus,
  type SmallModelStats,
  type SmallModelUnavailablePolicy,
  type SoulProposal,
} from '@/api/settings'
import { claudeCodeApi } from '@/api/claudecode'

const STORAGE_KEY = 'zimaos-blue-settings'

// Provider info for Chat page (simplified view of Provider Pool data)
export interface ChatModelInfo {
  id: string
  inputPrice?: number
  outputPrice?: number
}

export interface ChatProviderInfo {
  id: string        // Provider ID from Provider Pool
  name: string      // Display name
  models: ChatModelInfo[]
}

// Combined option for single dropdown: Provider(model)
export interface ProviderModelOption {
  value: string       // Format: "providerId:modelId"
  label: string       // Format: "ProviderName(modelId)"
  providerId: string
  providerName: string
  modelId: string
  inputPrice?: number
  outputPrice?: number
}

// Theme style types
export type ThemeStyle = 'default' | 'bubble' | 'minimal' | 'gradient' | 'ocean'
export type CloseBehavior = 'quit' | 'minimize'
export type MemoryRecallMode = 'aggressive' | 'balanced' | 'quality'

export const THEME_STYLES: { id: ThemeStyle; labelKey: string }[] = [
  { id: 'default', labelKey: 'theme.styles.default' },
  { id: 'bubble', labelKey: 'theme.styles.bubble' },
  { id: 'minimal', labelKey: 'theme.styles.minimal' },
  { id: 'gradient', labelKey: 'theme.styles.gradient' },
  { id: 'ocean', labelKey: 'theme.styles.ocean' },
]

const THEME_STYLE_SET = new Set<ThemeStyle>(THEME_STYLES.map(s => s.id))

function isThemeStyle(value: unknown): value is ThemeStyle {
  return typeof value === 'string' && THEME_STYLE_SET.has(value as ThemeStyle)
}

interface StoredSettings {
  selectedProviderModel: string  // Format: "providerId:modelId"
  temperature: number
  maxTokens: number
  themeStyle: ThemeStyle
  closeBehavior: CloseBehavior
  showToolDetails: boolean
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
  const showToolDetails = ref(stored.showToolDetails ?? false)
  const loading = ref(false)
  const refreshing = ref(false)
  const error = ref<string | null>(null)

  // Backend settings (locale, timezone)
  const backendSettings = ref<Settings>({})
  const backendSettingsLoading = ref(false)
  const smallModelStatus = ref<SmallModelStatus | null>(null)
  const smallModelStatusLoading = ref(false)
  const smallModelStatusError = ref<string | null>(null)
  const smallModelStats = ref<SmallModelStats | null>(null)
  const smallModelStatsLoading = ref(false)
  const smallModelStatsError = ref<string | null>(null)
  const soulProposals = ref<SoulProposal[]>([])
  const soulProposalsLoading = ref(false)
  const soulProposalsError = ref<string | null>(null)

  // Computed: All provider-model options for dropdown
  const providerModelOptions = computed<ProviderModelOption[]>(() => {
    const options: ProviderModelOption[] = []
    for (const provider of providers.value) {
      for (const model of provider.models) {
        options.push({
          value: `${provider.id}:${model.id}`,
          label: `${provider.name}(${model.id})`,
          providerId: provider.id,
          providerName: provider.name,
          modelId: model.id,
          inputPrice: model.inputPrice,
          outputPrice: model.outputPrice,
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
    [selectedProviderModel, temperature, maxTokens, themeStyle, closeBehavior, showToolDetails],
    () => {
      saveSettings({
        selectedProviderModel: selectedProviderModel.value,
        temperature: temperature.value,
        maxTokens: maxTokens.value,
        themeStyle: themeStyle.value,
        closeBehavior: closeBehavior.value,
        showToolDetails: showToolDetails.value,
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
      applyPoolProviders(poolProviders)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch providers'
    } finally {
      loading.value = false
    }
  }

  /** Update chat provider list from already-fetched Provider Pool data (avoids duplicate API call). */
  function updateFromPoolProviders(poolProviders: Provider[]) {
    applyPoolProviders(poolProviders)
  }

  /** Shared logic: convert raw Provider[] → ChatProviderInfo[] and set default selection. */
  function applyPoolProviders(poolProviders: Provider[]) {
    // Filter enabled providers and convert to ChatProviderInfo
    const enabledProviders = poolProviders.filter((p: Provider) => p.enabled)

    // Use inlined models from provider list response (no extra requests)
    const providerInfos: ChatProviderInfo[] = enabledProviders.map((provider: Provider) => ({
      id: provider.id,
      name: provider.name,
      models: (provider.models || []).filter((m: Model) => m.enabled).map((m: Model) => ({
        id: m.id,
        inputPrice: m.input_price,
        outputPrice: m.output_price,
      })),
    }))

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
      const chatModels: ChatModelInfo[] = models.filter((m: Model) => m.enabled).map((m: Model) => ({
        id: m.id,
        inputPrice: m.input_price,
        outputPrice: m.output_price,
      }))

      // Update the provider's models in the list
      const index = providers.value.findIndex((p) => p.id === targetProviderId)
      if (index !== -1) {
        const existingProvider = providers.value[index]
        if (existingProvider) {
          providers.value[index] = {
            id: existingProvider.id,
            name: existingProvider.name,
            models: chatModels,
          }
        }
      }

      // If current selection is no longer valid, select first model of this provider
      if (targetProviderId === selectedProvider.value) {
        const modelIds = chatModels.map(m => m.id)
        if (modelIds.length > 0 && !modelIds.includes(selectedModel.value)) {
          const firstModel = modelIds[0]
          if (firstModel) {
            selectedProviderModel.value = `${targetProviderId}:${firstModel}`
          }
        }
      }

      return { id: targetProviderId, models: chatModels.map(m => m.id) }
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
      if (provider.models.some(m => m.id === currentModel)) {
        selectedProviderModel.value = `${providerId}:${currentModel}`
      } else {
        // Select first model of new provider
        const firstModel = provider.models[0]
        if (firstModel) {
          selectedProviderModel.value = `${providerId}:${firstModel.id}`
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
    // Persist to backend best-effort; local state still updates immediately.
    updateBackendSettings({ theme_style: style }).catch(() => {})
  }

  function setCloseBehavior(behavior: CloseBehavior) {
    closeBehavior.value = behavior
  }

  function setShowToolDetails(show: boolean) {
    showToolDetails.value = show
  }

  function clearError() {
    error.value = null
  }

  function resetToDefaults() {
    temperature.value = 0.7
    maxTokens.value = 2048
    themeStyle.value = 'default'
    closeBehavior.value = 'quit'
    showToolDetails.value = false
    // Clear stored settings
    localStorage.removeItem(STORAGE_KEY)
  }

  // Fetch backend settings (locale, timezone)
  async function fetchBackendSettings() {
    try {
      backendSettingsLoading.value = true
      const response = await settingsApi.get()
      backendSettings.value = response.data
      if (isThemeStyle(response.data.theme_style)) {
        themeStyle.value = response.data.theme_style
      }
    } catch (e) {
      console.error('Failed to fetch backend settings:', e)
    } finally {
      backendSettingsLoading.value = false
    }
  }

  // Update backend settings
  async function updateBackendSettings(updates: Partial<Settings>) {
    try {
      const response = await settingsApi.patch(updates)
      backendSettings.value = response.data
    } catch (e) {
      console.error('Failed to update backend settings:', e)
      throw e
    }
  }

  // Claude Code CLI enhanced mode
  const claudeCodeEnabled = ref(false)

  async function fetchClaudeCodeEnabled() {
    try {
      const response = await claudeCodeApi.getConfig()
      claudeCodeEnabled.value = response.data.enabled
    } catch {
      claudeCodeEnabled.value = false
    }
  }

  function setClaudeCodeEnabled(enabled: boolean) {
    claudeCodeEnabled.value = enabled
  }

  // Agent mode (from backend settings)
  const agentMode = computed(() => backendSettings.value.agent_mode ?? false)
  const agentAutoConfirm = computed(() => backendSettings.value.agent_auto_confirm ?? false)
  const memoryRecallMode = computed<MemoryRecallMode>(() => {
    const mode = backendSettings.value.memory_recall_mode
    if (mode === 'aggressive' || mode === 'quality') return mode
    return 'balanced'
  })
  const skillRerankEnabled = computed(() => backendSettings.value.skill_rerank_enabled ?? true)
  const skillRerankONNXEnabled = computed(() => backendSettings.value.skill_rerank_onnx_enabled ?? false)
  const skillRerankONNXAutoDownload = computed(() => backendSettings.value.skill_rerank_onnx_auto_download ?? false)
  const smallModelEnabled = computed(() => backendSettings.value.small_model_enabled ?? false)
  const smallModelRuntime = computed<SmallModelRuntime>(() => {
    const runtime = backendSettings.value.small_model_runtime
    return runtime === 'llama_cpp_native' ? runtime : 'llama_cpp_native'
  })
  const smallModelID = computed<SmallModelID>(() => {
    const id = backendSettings.value.small_model_id
    return id === 'lfm2.5-1.2b-instruct-q4km' ? id : 'lfm2.5-1.2b-instruct-q4km'
  })
  const smallModelAutoDownload = computed(() => backendSettings.value.small_model_auto_download ?? true)
  const smallModelShadowRatio = computed(() => {
    const ratio = backendSettings.value.small_model_shadow_ratio
    if (typeof ratio !== 'number' || ratio <= 0 || ratio > 1) return 0.1
    return ratio
  })
  const smallModelSummaryEnabled = computed(() => backendSettings.value.small_model_summary_enabled ?? true)
  const smallModelDocExtractEnabled = computed(() => backendSettings.value.small_model_doc_extract_enabled ?? true)
  const smallModelRerankEnabled = computed(() => backendSettings.value.small_model_rerank_enabled ?? true)
  const smallModelContextPruneEnabled = computed(() => backendSettings.value.small_model_context_prune_enabled ?? true)
  const smallModelRouteShortQAEnabled = computed(() => backendSettings.value.small_model_route_short_qa_enabled ?? false)
  const smallModelRouteToolDispatchEnabled = computed(() => backendSettings.value.small_model_route_tool_dispatch_enabled ?? false)
  const noLLMDegradeMode = computed<NoLLMDegradeMode>(() => {
    return backendSettings.value.no_llm_degrade_mode === 'deepresearch'
      ? backendSettings.value.no_llm_degrade_mode
      : 'deepresearch'
  })
  const smallModelUnavailablePolicy = computed<SmallModelUnavailablePolicy>(() => {
    return backendSettings.value.small_model_unavailable_policy === 'ir_first'
      ? backendSettings.value.small_model_unavailable_policy
      : 'ir_first'
  })

  async function setAgentMode(enabled: boolean) {
    await updateBackendSettings({ agent_mode: enabled })
  }

  async function setAgentAutoConfirm(enabled: boolean) {
    await updateBackendSettings({ agent_auto_confirm: enabled })
  }

  async function setMemoryRecallMode(mode: MemoryRecallMode) {
    await updateBackendSettings({ memory_recall_mode: mode })
  }

  async function setSkillRerankEnabled(enabled: boolean) {
    await updateBackendSettings({ skill_rerank_enabled: enabled })
  }

  async function setSkillRerankONNXEnabled(enabled: boolean) {
    await updateBackendSettings({ skill_rerank_onnx_enabled: enabled })
  }

  async function setSkillRerankONNXAutoDownload(enabled: boolean) {
    await updateBackendSettings({ skill_rerank_onnx_auto_download: enabled })
  }

  async function setSmallModelEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_enabled: enabled })
  }

  async function setSmallModelAutoDownload(enabled: boolean) {
    await updateBackendSettings({ small_model_auto_download: enabled })
  }

  async function setSmallModelShadowRatio(ratio: number) {
    const normalized = Math.max(0.01, Math.min(1, ratio))
    await updateBackendSettings({ small_model_shadow_ratio: normalized })
  }

  async function setSmallModelSummaryEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_summary_enabled: enabled })
  }

  async function setSmallModelDocExtractEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_doc_extract_enabled: enabled })
  }

  async function setSmallModelRerankEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_rerank_enabled: enabled })
  }

  async function setSmallModelContextPruneEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_context_prune_enabled: enabled })
  }

  async function setSmallModelRouteShortQAEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_route_short_qa_enabled: enabled })
  }

  async function setSmallModelRouteToolDispatchEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_route_tool_dispatch_enabled: enabled })
  }

  async function setNoLLMDegradeMode(mode: NoLLMDegradeMode) {
    await updateBackendSettings({ no_llm_degrade_mode: mode })
  }

  async function setSmallModelUnavailablePolicy(policy: SmallModelUnavailablePolicy) {
    await updateBackendSettings({ small_model_unavailable_policy: policy })
  }

  async function fetchSmallModelStatus() {
    try {
      smallModelStatusLoading.value = true
      smallModelStatusError.value = null
      const response = await settingsApi.getSmallModelStatus()
      smallModelStatus.value = response.data
      return response.data
    } catch (e) {
      smallModelStatusError.value = e instanceof Error ? e.message : 'Failed to fetch small model status'
      throw e
    } finally {
      smallModelStatusLoading.value = false
    }
  }

  async function startSmallModelDownload() {
    await settingsApi.downloadSmallModel()
    return fetchSmallModelStatus()
  }

  async function cancelSmallModelDownload() {
    await settingsApi.cancelSmallModelDownload()
    return fetchSmallModelStatus()
  }

  async function fetchSoulProposals() {
    try {
      soulProposalsLoading.value = true
      soulProposalsError.value = null
      const response = await settingsApi.listSoulProposals()
      soulProposals.value = response.data.proposals || []
      return soulProposals.value
    } catch (e) {
      soulProposalsError.value = e instanceof Error ? e.message : 'Failed to fetch SOUL proposals'
      throw e
    } finally {
      soulProposalsLoading.value = false
    }
  }

  async function approveSoulProposal(id: string) {
    const response = await settingsApi.approveSoulProposal(id)
    const idx = soulProposals.value.findIndex((p) => p.id === response.data.id)
    if (idx >= 0) {
      soulProposals.value[idx] = response.data
    } else {
      soulProposals.value.unshift(response.data)
    }
    return response.data
  }

  async function rejectSoulProposal(id: string) {
    const response = await settingsApi.rejectSoulProposal(id)
    const idx = soulProposals.value.findIndex((p) => p.id === response.data.id)
    if (idx >= 0) {
      soulProposals.value[idx] = response.data
    } else {
      soulProposals.value.unshift(response.data)
    }
    return response.data
  }

  async function fetchSmallModelStats() {
    try {
      smallModelStatsLoading.value = true
      smallModelStatsError.value = null
      const response = await settingsApi.getSmallModelStats()
      smallModelStats.value = response.data
      return response.data
    } catch (e) {
      smallModelStatsError.value = e instanceof Error ? e.message : 'Failed to fetch small-model stats'
      throw e
    } finally {
      smallModelStatsLoading.value = false
    }
  }

  async function resetSmallModelStats() {
    await settingsApi.resetSmallModelStats()
    return fetchSmallModelStats()
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
    backendSettings,
    backendSettingsLoading,
    smallModelStatus,
    smallModelStatusLoading,
    smallModelStatusError,
    smallModelStats,
    smallModelStatsLoading,
    smallModelStatsError,
    soulProposals,
    soulProposalsLoading,
    soulProposalsError,
    claudeCodeEnabled,
    agentMode,
    agentAutoConfirm,
    memoryRecallMode,
    skillRerankEnabled,
    skillRerankONNXEnabled,
    skillRerankONNXAutoDownload,
    smallModelEnabled,
    smallModelRuntime,
    smallModelID,
    smallModelAutoDownload,
    smallModelShadowRatio,
    smallModelSummaryEnabled,
    smallModelDocExtractEnabled,
    smallModelRerankEnabled,
    smallModelContextPruneEnabled,
    smallModelRouteShortQAEnabled,
    smallModelRouteToolDispatchEnabled,
    noLLMDegradeMode,
    smallModelUnavailablePolicy,
    showToolDetails,

    // Computed
    providerModelOptions,
    selectedProvider,
    selectedModel,
    currentProvider,
    availableModels,

    // Actions
    fetchProviders,
    updateFromPoolProviders,
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
    fetchBackendSettings,
    updateBackendSettings,
    fetchClaudeCodeEnabled,
    setClaudeCodeEnabled,
    setAgentMode,
    setAgentAutoConfirm,
    setMemoryRecallMode,
    setSkillRerankEnabled,
    setSkillRerankONNXEnabled,
    setSkillRerankONNXAutoDownload,
    setSmallModelEnabled,
    setSmallModelAutoDownload,
    setSmallModelShadowRatio,
    setSmallModelSummaryEnabled,
    setSmallModelDocExtractEnabled,
    setSmallModelRerankEnabled,
    setSmallModelContextPruneEnabled,
    setSmallModelRouteShortQAEnabled,
    setSmallModelRouteToolDispatchEnabled,
    setNoLLMDegradeMode,
    setSmallModelUnavailablePolicy,
    fetchSmallModelStatus,
    startSmallModelDownload,
    cancelSmallModelDownload,
    fetchSoulProposals,
    approveSoulProposal,
    rejectSoulProposal,
    fetchSmallModelStats,
    resetSmallModelStats,
    setShowToolDetails,
  }
})
