import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type { ToolDefinition } from '@/api/chat'
import type { Provider, Model } from '@/api/providerPool'
import type {
  ContextCompressionMode,
  Settings,
  NoLLMDegradeMode,
  SmallModelID,
  SmallModelRuntime,
  SmallModelStatus,
  SmallModelStats,
  SmallModelUnavailablePolicy,
} from '@/api/settings'

type ChatApiModule = typeof import('@/api/chat')
type ProviderPoolApiModule = typeof import('@/api/providerPool')
type SettingsApiModule = typeof import('@/api/settings')
type ClaudeCodeApiModule = typeof import('@/api/claudecode')

let chatApiModulePromise: Promise<ChatApiModule> | null = null
let providerPoolApiModulePromise: Promise<ProviderPoolApiModule> | null = null
let settingsApiModulePromise: Promise<SettingsApiModule> | null = null
let claudeCodeApiModulePromise: Promise<ClaudeCodeApiModule> | null = null

function loadChatApiModule(): Promise<ChatApiModule> {
  if (!chatApiModulePromise) {
    chatApiModulePromise = import('@/api/chat')
  }
  return chatApiModulePromise
}

function loadProviderPoolApiModule(): Promise<ProviderPoolApiModule> {
  if (!providerPoolApiModulePromise) {
    providerPoolApiModulePromise = import('@/api/providerPool')
  }
  return providerPoolApiModulePromise
}

function loadSettingsApiModule(): Promise<SettingsApiModule> {
  if (!settingsApiModulePromise) {
    settingsApiModulePromise = import('@/api/settings')
  }
  return settingsApiModulePromise
}

function loadClaudeCodeApiModule(): Promise<ClaudeCodeApiModule> {
  if (!claudeCodeApiModulePromise) {
    claudeCodeApiModulePromise = import('@/api/claudecode')
  }
  return claudeCodeApiModulePromise
}

const STORAGE_KEY = 'zimaos-blue-settings'
const MAX_TOKENS_MIGRATION_KEY_V1 = 'zimaos-blue-max-tokens-migrated-v1'
const MAX_TOKENS_MIGRATION_KEY_V2 = 'zimaos-blue-max-tokens-migrated-v2'
const LEGACY_DEFAULT_MAX_TOKENS = 2048
const PREVIOUS_DEFAULT_MAX_TOKENS = 8192
const DEFAULT_MAX_TOKENS = 16384

// Provider info for Chat page (simplified view of Provider Pool data)
export interface ChatModelInfo {
  id: string
  inputPrice?: number
  outputPrice?: number
}

export interface ChatProviderInfo {
  id: string // Provider ID from Provider Pool
  name: string // Display name
  models: ChatModelInfo[]
}

// Combined option for single dropdown: Provider(model)
export interface ProviderModelOption {
  value: string // Format: "providerId:modelId"
  label: string // Format: "ProviderName(modelId)"
  providerId: string
  providerName: string
  modelId: string
  inputPrice?: number
  outputPrice?: number
}

export type CloseBehavior = 'quit' | 'minimize'
export type MemoryRecallMode = 'aggressive' | 'balanced' | 'quality'

interface StoredSettings {
  selectedProviderModel: string // Format: "providerId:modelId"
  temperature: number
  maxTokens: number
  closeBehavior: CloseBehavior
  showToolDetails: boolean
}

function markMaxTokensMigrationDoneV1() {
  localStorage.setItem(MAX_TOKENS_MIGRATION_KEY_V1, '1')
}

function markMaxTokensMigrationDoneV2() {
  localStorage.setItem(MAX_TOKENS_MIGRATION_KEY_V2, '1')
}

function markAllMaxTokensMigrationsDone() {
  markMaxTokensMigrationDoneV1()
  markMaxTokensMigrationDoneV2()
}

function migrateLegacyMaxTokensV1(settings: Partial<StoredSettings>): Partial<StoredSettings> {
  try {
    if (localStorage.getItem(MAX_TOKENS_MIGRATION_KEY_V1) === '1') {
      return settings
    }

    if (settings.maxTokens === LEGACY_DEFAULT_MAX_TOKENS) {
      const next = {
        ...settings,
        maxTokens: PREVIOUS_DEFAULT_MAX_TOKENS,
      }
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
      markMaxTokensMigrationDoneV1()
      return next
    }

    // Mark migration as completed so future manual 2048 choices are not auto-migrated.
    markMaxTokensMigrationDoneV1()
  } catch {
    // Best-effort migration only.
  }
  return settings
}

function migrateLegacyMaxTokensV2(settings: Partial<StoredSettings>): Partial<StoredSettings> {
  try {
    if (localStorage.getItem(MAX_TOKENS_MIGRATION_KEY_V2) === '1') {
      return settings
    }

    if (settings.maxTokens === PREVIOUS_DEFAULT_MAX_TOKENS) {
      const next = {
        ...settings,
        maxTokens: DEFAULT_MAX_TOKENS,
      }
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
      markMaxTokensMigrationDoneV2()
      return next
    }

    // Mark migration as completed so future manual 8192 choices are not auto-migrated.
    markMaxTokensMigrationDoneV2()
  } catch {
    // Best-effort migration only.
  }
  return settings
}

function loadStoredSettings(): Partial<StoredSettings> {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (!stored) {
      // Fresh profile: mark migration done to avoid migrating user-selected 2048 later.
      markAllMaxTokensMigrationsDone()
      return {}
    }
    const parsed = JSON.parse(stored)
    if (parsed && typeof parsed === 'object') {
      return migrateLegacyMaxTokensV2(migrateLegacyMaxTokensV1(parsed as Partial<StoredSettings>))
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
  const selectedProviderModel = ref(stored.selectedProviderModel || '') // Format: "providerId:modelId"
  const temperature = ref(stored.temperature ?? 0.7)
  const maxTokens = ref(stored.maxTokens ?? DEFAULT_MAX_TOKENS)
  const closeBehavior = ref<CloseBehavior>(stored.closeBehavior || 'minimize')
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
    return parts.slice(1).join(':') || '' // Handle model IDs that contain ':'
  })

  const currentProvider = computed(() =>
    providers.value.find((p) => p.id === selectedProvider.value)
  )

  const availableModels = computed(() => currentProvider.value?.models || [])

  // Watch for changes and persist
  watch(
    [selectedProviderModel, temperature, maxTokens, closeBehavior, showToolDetails],
    () => {
      saveSettings({
        selectedProviderModel: selectedProviderModel.value,
        temperature: temperature.value,
        maxTokens: maxTokens.value,
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
      const { providerPoolApi } = await loadProviderPoolApiModule()
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
      models: (provider.models || [])
        .filter((m: Model) => m.enabled)
        .map((m: Model) => ({
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
      const { providerPoolApi } = await loadProviderPoolApiModule()
      const response = await providerPoolApi.fetchProviderModels(targetProviderId)
      const models = response.data.models || []
      const chatModels: ChatModelInfo[] = models
        .filter((m: Model) => m.enabled)
        .map((m: Model) => ({
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
        const modelIds = chatModels.map((m) => m.id)
        if (modelIds.length > 0 && !modelIds.includes(selectedModel.value)) {
          const firstModel = modelIds[0]
          if (firstModel) {
            selectedProviderModel.value = `${targetProviderId}:${firstModel}`
          }
        }
      }

      return { id: targetProviderId, models: chatModels.map((m) => m.id) }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to refresh models'
      throw e
    } finally {
      refreshing.value = false
    }
  }

  async function fetchTools() {
    try {
      const { toolApi } = await loadChatApiModule()
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
    const provider = providers.value.find((p) => p.id === providerId)
    if (provider && provider.models.length > 0) {
      // Try to keep current model if it exists in new provider
      const currentModel = selectedModel.value
      if (provider.models.some((m) => m.id === currentModel)) {
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
    maxTokens.value = DEFAULT_MAX_TOKENS
    closeBehavior.value = 'minimize'
    showToolDetails.value = false
    // Clear stored settings
    localStorage.removeItem(STORAGE_KEY)
  }

  // Fetch backend settings (locale, timezone)
  async function fetchBackendSettings() {
    try {
      backendSettingsLoading.value = true
      const { settingsApi } = await loadSettingsApiModule()
      const response = await settingsApi.get()
      backendSettings.value = response.data
    } catch (e) {
      console.error('Failed to fetch backend settings:', e)
    } finally {
      backendSettingsLoading.value = false
    }
  }

  // Update backend settings
  async function updateBackendSettings(updates: Partial<Settings>) {
    try {
      const { settingsApi } = await loadSettingsApiModule()
      const response = await settingsApi.patch(updates)
      backendSettings.value = response.data
    } catch (e) {
      console.error('Failed to update backend settings:', e)
      throw e
    }
  }

  // Claude Code CLI enhanced mode
  const claudeCodeEnabled = ref(false)
  const claudeCodeEnabledLoaded = ref(false)

  async function fetchClaudeCodeEnabled() {
    try {
      claudeCodeEnabledLoaded.value = false
      const { claudeCodeApi } = await loadClaudeCodeApiModule()
      const response = await claudeCodeApi.getConfig()
      claudeCodeEnabled.value = response.data.enabled
    } catch {
      claudeCodeEnabled.value = false
    } finally {
      claudeCodeEnabledLoaded.value = true
    }
  }

  function setClaudeCodeEnabled(enabled: boolean) {
    claudeCodeEnabled.value = enabled
    claudeCodeEnabledLoaded.value = true
  }

  // Agent mode (from backend settings)
  const agentMode = computed(() => backendSettings.value.agent_mode ?? false)
  const agentAutoReflect = computed(() => backendSettings.value.agent_auto_reflect ?? true)
  const agentAutoConfirm = computed(() => backendSettings.value.agent_auto_confirm ?? false)
  const memoryRecallMode = computed<MemoryRecallMode>(() => {
    const mode = backendSettings.value.memory_recall_mode
    if (mode === 'aggressive' || mode === 'quality') return mode
    return 'balanced'
  })
  const skillRerankEnabled = computed(() => backendSettings.value.skill_rerank_enabled ?? false)
  const skillRerankONNXEnabled = computed(
    () => backendSettings.value.skill_rerank_onnx_enabled ?? false
  )
  const skillRerankONNXAutoDownload = computed(
    () => backendSettings.value.skill_rerank_onnx_auto_download ?? false
  )
  const smallModelEnabled = computed(() => backendSettings.value.small_model_enabled ?? false)
  const smallModelRuntime = computed<SmallModelRuntime>(() => {
    const runtime = backendSettings.value.small_model_runtime
    return runtime === 'llama.cpp' ? runtime : 'llama.cpp'
  })
  const smallModelID = computed<SmallModelID>(() => {
    const id = backendSettings.value.small_model_id
    return id === 'qwen3.5-0.8b-gguf-q4km' ? id : 'qwen3.5-0.8b-gguf-q4km'
  })
  const smallModelAutoDownload = computed(
    () => backendSettings.value.small_model_auto_download ?? true
  )
  const smallModelSummaryEnabled = computed(
    () => backendSettings.value.small_model_summary_enabled ?? false
  )
  const smallModelContextCompressEnabled = computed(
    () => backendSettings.value.small_model_context_compress_enabled ?? false
  )
  const smallModelDocExtractEnabled = computed(
    () => backendSettings.value.small_model_doc_extract_enabled ?? false
  )
  const smallModelRerankEnabled = computed(
    () => backendSettings.value.small_model_rerank_enabled ?? false
  )
  const smartToolSelection = computed(() => backendSettings.value.smart_tool_selection ?? false)
  const smartSkillSelection = computed(() => backendSettings.value.smart_skill_selection ?? false)
  const skillSelectorMode = computed<'hybrid' | 'ir_only' | 'llm_only'>(() => {
    const mode = backendSettings.value.skill_selector_mode
    if (mode === 'ir_only' || mode === 'llm_only') return mode
    return 'hybrid'
  })
  const skillSelectorConfidenceThreshold = computed(
    () => backendSettings.value.skill_selector_confidence_threshold ?? 0.78
  )
  const promptPolicyVersion = computed(
    () => backendSettings.value.prompt_policy_version ?? '2026-03-04'
  )
  const promptPolicyProfile = computed<'default'>(() => {
    const profile = backendSettings.value.prompt_policy_profile
    return profile === 'default' ? profile : 'default'
  })
  const agentLoopPolicyMaxToolRounds = computed(
    () => backendSettings.value.agent_loop_policy_max_tool_rounds ?? 48
  )
  const agentLoopPolicyMaxAutoContinue = computed(
    () => backendSettings.value.agent_loop_policy_max_auto_continue ?? 12
  )
  const agentLoopPolicyPseudoToolCallBudget = computed(
    () => backendSettings.value.agent_loop_policy_pseudo_tool_call_budget ?? 3
  )
  const agentLoopPolicyActionPledgeBudget = computed(
    () => backendSettings.value.agent_loop_policy_action_pledge_budget ?? 3
  )
  const agentLoopPolicyMissingTodoBudget = computed(
    () => backendSettings.value.agent_loop_policy_missing_todo_budget ?? 3
  )
  const agentLoopPolicyPendingTodoBudget = computed(
    () => backendSettings.value.agent_loop_policy_pending_todo_budget ?? 3
  )
  const contextCompressionMode = computed<ContextCompressionMode>(() => {
    const mode = backendSettings.value.context_compression_mode
    if (mode === 'offline' || mode === 'small_model') return mode
    return 'auto'
  })
  const smallModelContextPruneEnabled = computed(
    () => backendSettings.value.small_model_context_prune_enabled ?? false
  )
  const smallModelMediaIntentEnabled = computed(
    () => backendSettings.value.small_model_media_intent_enabled ?? false
  )
  const offlineIRFallbackEnabled = computed(
    () => backendSettings.value.offline_ir_fallback_enabled ?? false
  )
  const featureIntentIREnabled = computed(
    () => backendSettings.value.feature_intent_ir_enabled ?? false
  )
  const smallModelIRFeaturesEnabled = computed(
    () =>
      smallModelContextPruneEnabled.value &&
      smallModelMediaIntentEnabled.value &&
      smartToolSelection.value &&
      offlineIRFallbackEnabled.value &&
      featureIntentIREnabled.value
  )
  const smallModelRouteImageQAEnabled = computed(
    () =>
      backendSettings.value.small_model_route_image_qa_enabled ??
      backendSettings.value.small_model_route_short_qa_enabled ??
      false
  )
  const smallModelRouteShortQAEnabled = computed(
    () => backendSettings.value.small_model_route_short_qa_enabled ?? false
  )
  const smallModelRouteToolDispatchEnabled = computed(
    () => backendSettings.value.small_model_route_tool_dispatch_enabled ?? false
  )
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

  async function setAgentAutoReflect(enabled: boolean) {
    await updateBackendSettings({ agent_auto_reflect: enabled })
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

  async function setSmallModelSummaryEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_summary_enabled: enabled })
  }

  async function setSmallModelContextCompressEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_context_compress_enabled: enabled })
  }

  async function setSmallModelDocExtractEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_doc_extract_enabled: enabled })
  }

  async function setSmallModelRerankEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_rerank_enabled: enabled })
  }

  async function setSmartToolSelection(enabled: boolean) {
    await updateBackendSettings({ smart_tool_selection: enabled })
  }

  async function setSmartSkillSelection(enabled: boolean) {
    await updateBackendSettings({ smart_skill_selection: enabled })
  }

  async function setSkillSelectorMode(mode: 'hybrid' | 'ir_only' | 'llm_only') {
    await updateBackendSettings({ skill_selector_mode: mode })
  }

  async function setSkillSelectorConfidenceThreshold(threshold: number) {
    await updateBackendSettings({ skill_selector_confidence_threshold: threshold })
  }

  async function setPromptPolicyVersion(version: string) {
    await updateBackendSettings({ prompt_policy_version: version })
  }

  async function setPromptPolicyProfile(profile: 'default') {
    await updateBackendSettings({ prompt_policy_profile: profile })
  }

  async function setAgentLoopPolicyMaxToolRounds(value: number) {
    await updateBackendSettings({ agent_loop_policy_max_tool_rounds: value })
  }

  async function setAgentLoopPolicyMaxAutoContinue(value: number) {
    await updateBackendSettings({ agent_loop_policy_max_auto_continue: value })
  }

  async function setAgentLoopPolicyPseudoToolCallBudget(value: number) {
    await updateBackendSettings({ agent_loop_policy_pseudo_tool_call_budget: value })
  }

  async function setAgentLoopPolicyActionPledgeBudget(value: number) {
    await updateBackendSettings({ agent_loop_policy_action_pledge_budget: value })
  }

  async function setAgentLoopPolicyMissingTodoBudget(value: number) {
    await updateBackendSettings({ agent_loop_policy_missing_todo_budget: value })
  }

  async function setAgentLoopPolicyPendingTodoBudget(value: number) {
    await updateBackendSettings({ agent_loop_policy_pending_todo_budget: value })
  }

  async function setContextCompressionMode(mode: ContextCompressionMode) {
    await updateBackendSettings({ context_compression_mode: mode })
  }

  async function setSmallModelContextPruneEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_context_prune_enabled: enabled })
  }

  async function setSmallModelMediaIntentEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_media_intent_enabled: enabled })
  }

  async function setOfflineIRFallbackEnabled(enabled: boolean) {
    await updateBackendSettings({ offline_ir_fallback_enabled: enabled })
  }

  async function setFeatureIntentIREnabled(enabled: boolean) {
    await updateBackendSettings({ feature_intent_ir_enabled: enabled })
  }

  async function setSmallModelIRFeaturesEnabled(enabled: boolean) {
    await updateBackendSettings({
      small_model_context_prune_enabled: enabled,
      small_model_media_intent_enabled: enabled,
      smart_tool_selection: enabled,
      offline_ir_fallback_enabled: enabled,
      feature_intent_ir_enabled: enabled,
    })
  }

  async function setSmallModelRouteShortQAEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_route_short_qa_enabled: enabled })
  }

  async function setSmallModelRouteImageQAEnabled(enabled: boolean) {
    await updateBackendSettings({ small_model_route_image_qa_enabled: enabled })
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
      const { settingsApi } = await loadSettingsApiModule()
      const response = await settingsApi.getSmallModelStatus()
      smallModelStatus.value = response.data
      return response.data
    } catch (e) {
      smallModelStatusError.value =
        e instanceof Error ? e.message : 'Failed to fetch small model status'
      throw e
    } finally {
      smallModelStatusLoading.value = false
    }
  }

  async function startSmallModelDownload() {
    const { settingsApi } = await loadSettingsApiModule()
    await settingsApi.downloadSmallModel()
    return fetchSmallModelStatus()
  }

  async function cancelSmallModelDownload() {
    const { settingsApi } = await loadSettingsApiModule()
    await settingsApi.cancelSmallModelDownload()
    return fetchSmallModelStatus()
  }

  async function fetchSmallModelStats() {
    try {
      smallModelStatsLoading.value = true
      smallModelStatsError.value = null
      const { settingsApi } = await loadSettingsApiModule()
      const response = await settingsApi.getSmallModelStats()
      smallModelStats.value = response.data
      return response.data
    } catch (e) {
      smallModelStatsError.value =
        e instanceof Error ? e.message : 'Failed to fetch small-model stats'
      throw e
    } finally {
      smallModelStatsLoading.value = false
    }
  }

  async function resetSmallModelStats() {
    const { settingsApi } = await loadSettingsApiModule()
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
    claudeCodeEnabled,
    claudeCodeEnabledLoaded,
    agentMode,
    agentAutoReflect,
    agentAutoConfirm,
    memoryRecallMode,
    skillRerankEnabled,
    skillRerankONNXEnabled,
    skillRerankONNXAutoDownload,
    smallModelEnabled,
    smallModelRuntime,
    smallModelID,
    smallModelAutoDownload,
    smallModelSummaryEnabled,
    smallModelContextCompressEnabled,
    smallModelDocExtractEnabled,
    smallModelRerankEnabled,
    smartToolSelection,
    smartSkillSelection,
    skillSelectorMode,
    skillSelectorConfidenceThreshold,
    promptPolicyVersion,
    promptPolicyProfile,
    agentLoopPolicyMaxToolRounds,
    agentLoopPolicyMaxAutoContinue,
    agentLoopPolicyPseudoToolCallBudget,
    agentLoopPolicyActionPledgeBudget,
    agentLoopPolicyMissingTodoBudget,
    agentLoopPolicyPendingTodoBudget,
    contextCompressionMode,
    smallModelContextPruneEnabled,
    smallModelMediaIntentEnabled,
    offlineIRFallbackEnabled,
    featureIntentIREnabled,
    smallModelIRFeaturesEnabled,
    smallModelRouteImageQAEnabled,
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
    setCloseBehavior,
    setShowToolDetails,
    clearError,
    resetToDefaults,
    fetchBackendSettings,
    updateBackendSettings,
    fetchClaudeCodeEnabled,
    setClaudeCodeEnabled,
    setAgentMode,
    setAgentAutoReflect,
    setAgentAutoConfirm,
    setMemoryRecallMode,
    setSkillRerankEnabled,
    setSkillRerankONNXEnabled,
    setSkillRerankONNXAutoDownload,
    setSmallModelEnabled,
    setSmallModelAutoDownload,
    setSmallModelSummaryEnabled,
    setSmallModelContextCompressEnabled,
    setSmallModelDocExtractEnabled,
    setSmallModelRerankEnabled,
    setSmartToolSelection,
    setSmartSkillSelection,
    setSkillSelectorMode,
    setSkillSelectorConfidenceThreshold,
    setPromptPolicyVersion,
    setPromptPolicyProfile,
    setAgentLoopPolicyMaxToolRounds,
    setAgentLoopPolicyMaxAutoContinue,
    setAgentLoopPolicyPseudoToolCallBudget,
    setAgentLoopPolicyActionPledgeBudget,
    setAgentLoopPolicyMissingTodoBudget,
    setAgentLoopPolicyPendingTodoBudget,
    setContextCompressionMode,
    setSmallModelContextPruneEnabled,
    setSmallModelMediaIntentEnabled,
    setOfflineIRFallbackEnabled,
    setFeatureIntentIREnabled,
    setSmallModelIRFeaturesEnabled,
    setSmallModelRouteImageQAEnabled,
    setSmallModelRouteShortQAEnabled,
    setSmallModelRouteToolDispatchEnabled,
    setNoLLMDegradeMode,
    setSmallModelUnavailablePolicy,
    fetchSmallModelStatus,
    startSmallModelDownload,
    cancelSmallModelDownload,
    fetchSmallModelStats,
    resetSmallModelStats,
  }
})
