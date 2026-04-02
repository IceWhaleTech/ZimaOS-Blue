<script setup lang="ts">
// @ts-nocheck
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useNotificationStore } from '@/stores/notification'
import ProviderIcon from '@/components/ProviderIcon.vue'
import type { Provider, Model, ProviderVerificationResult, APIFormat } from '@/api/providerPool'
import { providerPoolApi } from '@/api/providerPool'
import { getLocaleDirection } from '@/i18n'
import { formatTokens } from '@/utils/format'
import { getLocalizedMediaModelName } from '@/utils/mediaModelLocalization'
import { filterProvidersVisibleInUI, isProviderVisibleInUI } from '@/utils/providerVisibility'

const { t, te, locale } = useI18n()
const route = useRoute()
const store = useProviderPoolStore()
const notification = useNotificationStore()
const isRtl = computed(() => getLocaleDirection(locale.value) === 'rtl')

// Local state
const showAddModal = ref(false)
const addProviderMode = ref<'chooser' | 'custom'>('chooser')
const showKeyModal = ref(false)
const showPricingModal = ref(false)
const showParamsModal = ref(false)
const showAllowedModelsModal = ref(false)
const showIDEDiscoveryModal = ref(false)
const testingProvider = ref<string | null>(null)
const testingKeyId = ref<string | null>(null)
const keyTestResults = ref<Record<string, { healthy: boolean; error?: string }>>({})
const refreshingModels = ref<string | null>(null)
const detectingCapabilities = ref<string | null>(null)
const verifyingProvider = ref<string | null>(null)
const applyingVerification = ref<string | null>(null)
const verificationKeyId = ref('')
const verificationResult = ref<ProviderVerificationResult | null>(null)
const verificationError = ref('')
const searchQuery = ref('')
type ProviderTab = 'all' | 'trial' | 'builtin' | 'platform' | 'other' | 'custom' | 'media'
const activeTab = ref<ProviderTab>('all')
function isBetaProvider(provider: Provider): boolean {
  return provider.beta === true
}
function getProviderTab(provider: Provider): ProviderTab {
  // Beta builtin/platform providers are grouped into "Other".
  if (isBetaProvider(provider) && (provider.type === 'builtin' || provider.type === 'platform')) {
    return 'other'
  }
  switch (provider.type) {
    case 'trial':
      return 'trial'
    case 'builtin':
      return 'builtin'
    case 'platform':
      return 'platform'
    case 'media':
      return 'media'
    default:
      return 'custom'
  }
}
const visibleProviders = computed(() => filterProvidersVisibleInUI(store.providers || []))

function providersForTab(tab: ProviderTab): Provider[] {
  const allProviders = visibleProviders.value
  if (tab === 'all') return allProviders
  return allProviders.filter((provider) => getProviderTab(provider) === tab)
}
function getTabCount(tab: ProviderTab): number {
  return providersForTab(tab).length
}
const availableTabs = computed<ProviderTab[]>(() => {
  const tabs: ProviderTab[] = ['all']
  if (getTabCount('trial') > 0) tabs.push('trial')
  tabs.push('builtin', 'platform')
  tabs.push('custom')
  if (getTabCount('media') > 0) tabs.push('media')
  return tabs
})

function visibleTab(tab: ProviderTab): ProviderTab {
  return tab === 'other' ? 'all' : tab
}
const iconInput = ref<HTMLInputElement | null>(null)
const uploadingIcon = ref(false)

// Drag and drop state
const draggedProvider = ref<Provider | null>(null)
const dragOverProvider = ref<string | null>(null)
const providerDragBaseOrder = ref<Provider[] | null>(null)
const providerPreviewOrder = ref<Provider[] | null>(null)

type EditableCustomProviderFormat = 'auto' | 'openai' | 'responses' | 'anthropic' | 'google'

const editableCustomProviderFormatOptions: Array<{
  value: EditableCustomProviderFormat
  apiFormat?: APIFormat
}> = [
  { value: 'auto' },
  { value: 'openai', apiFormat: 'openai' },
  { value: 'responses', apiFormat: 'responses' },
  { value: 'anthropic', apiFormat: 'anthropic' },
  { value: 'google', apiFormat: 'google' },
]

const customProviderFormatDraft = ref<EditableCustomProviderFormat>('auto')
const savingCustomProviderFormat = ref<string | null>(null)

// New provider form
const newProvider = ref({
  name: '',
  base_url: '',
  api_key: '',
  priority: 50,
  location: 'cloud' as 'cloud' | 'local',
  format: 'auto' as EditableCustomProviderFormat,
})
const showNewProviderApiKey = ref(false)
const addingProvider = ref(false)
const addingStep = ref('') // '', 'adding', 'probing', 'done'

function openAddProviderModal() {
  showNewProviderApiKey.value = false
  addProviderMode.value = 'chooser'
  showAddModal.value = true
}

function closeAddProviderModal() {
  showNewProviderApiKey.value = false
  addProviderMode.value = 'chooser'
  resetNewProviderForm()
  showAddModal.value = false
}

function tr(key: string, fallback = ''): string {
  return te(key) ? t(key) : fallback
}

function trp(key: string, params: Record<string, unknown>, fallback: string): string {
  return te(key) ? t(key, params) : fallback
}

const tabFallbackLabels: Record<ProviderTab, string> = {
  all: 'All',
  trial: 'Trial',
  builtin: 'Built-in',
  platform: 'Platform',
  other: 'Other',
  custom: 'Custom',
  media: 'Media',
}
function getTabLabel(tab: ProviderTab): string {
  return tr(`providerPool.tabs.${tab}`, tabFallbackLabels[tab] || tab)
}

function getLocalizedProviderModelName(model: Model): string {
  return getLocalizedMediaModelName(model, t, te)
}

function canEditProviderLocation(provider?: Provider | null): boolean {
  return !!provider && provider.type === 'custom'
}

function getEffectiveProviderLocation(provider?: Provider | null): 'cloud' | 'local' {
  if (!provider) return 'cloud'
  if (
    provider.location === 'local' &&
    (provider.type === 'custom' || provider.metadata_mode === 'catalog')
  ) {
    return 'local'
  }
  return 'cloud'
}

// New API key form
const newKey = ref({
  providerId: '',
  key: '',
  label: '',
})
const showNewKeyApiKey = ref(false)

function closeKeyModal() {
  showNewKeyApiKey.value = false
  showKeyModal.value = false
}
const selectedKeyId = ref('')
const fetchingKeyModels = ref<Record<string, boolean>>({})

// Pricing form
const pricingForm = ref({
  modelId: '',
  providerId: '',
  inputPrice: 0,
  outputPrice: 0,
  cachePrice: 0,
  isMedia: false,
  pricePerRequest: 0,
  pricingUnit: 'image' as string,
})

// Model params form
const paramsForm = ref({
  temperature: undefined as number | undefined,
  maxTokens: undefined as number | undefined,
  topP: undefined as number | undefined,
})

// Allowed models form
const allowedModelsForm = ref<string[]>([])
const allAvailableModels = ref<Model[]>([])
const loadingAllModels = ref(false)
const savingAllowedModels = ref(false)

// Model params collapsible
const showModelParams = ref(false)
const showDeleteFor = ref<string | null>(null)
let longPressTimer: ReturnType<typeof setTimeout> | null = null

// Model drag state
const draggedModel = ref<Model | null>(null)
const dragOverModel = ref<string | null>(null)

// Provider to display in detail panel - always show selected provider regardless of tab
const displayProvider = computed(() =>
  isProviderVisibleInUI(store.selectedProvider) ? store.selectedProvider : null
)

// Models for selected provider
const selectedProviderModels = computed(() => {
  if (!store.selectedProviderId || !store.models) return []
  return store.models.filter((m) => m.provider_id === store.selectedProviderId)
})

const officialProviderOptions = computed(() =>
  visibleProviders.value.filter(
    (provider) => provider.type === 'builtin' || provider.type === 'platform'
  )
)

const canReorderProviders = computed(() => filteredProviders.value.length > 1)
const providerOutputLimitLabel = computed(() =>
  tr('providerPool.outputLimit', t('providerPool.maxTokens'))
)

// Models to display: always provider-level (union of all keys)
const displayModels = computed(() => {
  return selectedProviderModels.value
})

const selectedKeyModels = computed(() => {
  const provider = displayProvider.value
  if (!provider || !selectedKeyId.value) return []
  const key = provider.api_keys?.find((item) => item.id === selectedKeyId.value)
  return key?.models || []
})

const providerAccountStatus = computed(() => {
  const provider = displayProvider.value
  if (!provider) return null
  return store.accountStatus[provider.id] || null
})

const verificationProbeEntries = computed(() => {
  if (!verificationResult.value?.probes) return []
  const probes = verificationResult.value.probes
  const showResponsesProbes =
    verificationResult.value.recommended_api_format === 'responses' ||
    verificationResult.value.detected_format === 'responses' ||
    verificationResult.value.responses_only
  const preferredOrder = ['models', 'anthropic_messages', 'chat_completions']
  if (showResponsesProbes) {
    preferredOrder.push('responses_v1', 'responses_plain')
  }
  const entries: Array<
    [string, { url: string; status_code?: number; reachable: boolean; error?: string }]
  > = []

  for (const key of preferredOrder) {
    const probe = probes[key]
    if (probe?.reachable) {
      entries.push([key, probe])
    }
  }
  for (const [key, probe] of Object.entries(probes)) {
    if (!showResponsesProbes && (key === 'responses_v1' || key === 'responses_plain')) {
      continue
    }
    if (!preferredOrder.includes(key) && probe?.reachable) {
      entries.push([key, probe])
    }
  }
  return entries
})

const supportsProviderVerification = computed(() => {
  const provider = displayProvider.value
  return (
    !!provider &&
    provider.type !== 'trial' &&
    provider.type !== 'media' &&
    provider.metadata_mode !== 'catalog'
  )
})

const showBaseURLSection = computed(() => {
  const provider = displayProvider.value
  if (!provider || provider.type === 'trial' || provider.metadata_mode === 'catalog') {
    return false
  }

  return (
    !provider.base_url ||
    provider.id === 'azure-openai' ||
    provider.type === 'custom' ||
    provider.type === 'platform'
  )
})

function supportsProviderAccountStatus(provider?: Provider | null): boolean {
  return provider?.id === 'openrouter' || provider?.id === 'openrouter-free' || provider?.id === 'deepseek'
}

function getProviderAccountStatusKey(provider?: Provider | null): string | undefined {
  if (!provider?.api_keys?.length) return undefined
  return provider.api_keys.find((key) => key.enabled)?.id || provider.api_keys[0]?.id
}

const shouldShowProviderAccountStatus = computed(() => {
  const provider = displayProvider.value
  return !!provider && supportsProviderAccountStatus(provider) && !!provider.api_keys?.length
})

const providerAccountStatusPrimaryItem = computed(() => {
  const status = providerAccountStatus.value
  if (!status?.items?.length) return null
  return status.items.find((item) => item.key === status.primary_item_key) || status.items[0] || null
})

const providerAccountStatusSecondaryItems = computed(() => {
  const status = providerAccountStatus.value
  const primaryKey = providerAccountStatusPrimaryItem.value?.key
  if (!status?.items?.length) return []
  return status.items.filter((item) => item.key !== primaryKey)
})

function getCustomProviderFormatValue(provider?: Provider | null): EditableCustomProviderFormat {
  if (!provider || provider.type !== 'custom') return 'auto'
  if (provider.api_format_mode === 'pinned') {
    switch (provider.api_format) {
      case 'openai':
      case 'responses':
      case 'anthropic':
      case 'google':
        return provider.api_format
    }
  }
  return 'auto'
}

let providerSelectionSeq = 0

// Refresh models when provider changes
watch(
  () =>
    [
      displayProvider.value?.id,
      displayProvider.value?.api_format,
      displayProvider.value?.api_format_mode,
    ] as const,
  ([providerId]) => {
    if (providerId && savingCustomProviderFormat.value === providerId) {
      return
    }
    customProviderFormatDraft.value = getCustomProviderFormatValue(displayProvider.value)
  },
  { immediate: true }
)

watch(
  () => displayProvider.value,
  async (provider) => {
    if (!provider) return
    const seq = ++providerSelectionSeq
    const providerId = provider.id

    // Fetch provider-level models (backend unions all keys automatically)
    await store.refreshModels(providerId)
    if (seq !== providerSelectionSeq) return

    fetchProviderUsage(providerId)
    if (supportsProviderAccountStatus(provider) && provider.api_keys?.length) {
      store.fetchAccountStatus(providerId, getProviderAccountStatusKey(provider))
    } else {
      store.clearAccountStatus(providerId)
    }
    // Fetch OAuth quota lazily when provider is selected
    if (provider.oauth?.connected) {
      store.fetchOAuthQuota(providerId)
    }
    // Always fetch OAuth accounts list for OAuth-capable providers
    if (provider.oauth) {
      fetchOAuthAccounts(providerId)
    }
    // Auto-test all API keys when entering provider detail
    if (provider.api_keys?.length) {
      for (const key of provider.api_keys) {
        if (!keyTestResults.value[key.id]) {
          testConnection(providerId, key.id)
        }
      }
    }
  },
  { immediate: true }
)

// Base URL editing
const editingBaseUrl = ref(false)
const baseUrlDraft = ref('')

function startEditBaseUrl(provider: Provider) {
  baseUrlDraft.value = provider.base_url || ''
  editingBaseUrl.value = true
}

async function saveBaseUrl(providerId: string) {
  try {
    await store.updateProvider(providerId, { base_url: baseUrlDraft.value })
    editingBaseUrl.value = false
    // Clear error when updating base URL
    await store.clearProviderError(providerId)
  } catch (e) {
    console.error('Failed to update base URL:', e)
  }
}

function cancelEditBaseUrl() {
  editingBaseUrl.value = false
}

async function saveCustomProviderFormat(nextValue: EditableCustomProviderFormat) {
  const provider = displayProvider.value
  if (!provider || provider.type !== 'custom') return

  const previousValue = getCustomProviderFormatValue(provider)
  if (nextValue === previousValue) {
    customProviderFormatDraft.value = previousValue
    return
  }

  savingCustomProviderFormat.value = provider.id
  try {
    const payload =
      nextValue === 'auto'
        ? { api_format_mode: 'auto' }
        : { api_format: nextValue as APIFormat, api_format_mode: 'pinned' }
    const updatedProvider = await store.updateProvider(provider.id, payload as Partial<Provider>)
    customProviderFormatDraft.value = getCustomProviderFormatValue(updatedProvider)
  } catch (e) {
    console.error('Failed to update provider format:', e)
    customProviderFormatDraft.value = previousValue
  } finally {
    savingCustomProviderFormat.value = null
  }
}

// Translate backend health check error codes to i18n messages
function translateHealthError(error: string): string {
  if (!error) return ''

  // Extract key info if present (format: "error_message (key: xxxxxxxx)")
  let keyInfo = ''
  const keyMatch = error.match(/\s\(key:\s+([^)]+)\)$/)
  if (keyMatch) {
    keyInfo = ` (key: ${keyMatch[1]})`
    error = error.slice(0, keyMatch.index)
  }

  if (error.startsWith('auth_error:')) {
    const code = error.split(':')[1]
    return t('providerPool.healthErrors.authError', { code }) + keyInfo
  }
  if (error === 'base_url_not_configured:azure') {
    return t('providerPool.healthErrors.baseUrlNotConfiguredAzure')
  }
  if (error === 'base_url_not_configured:bedrock') {
    return t('providerPool.healthErrors.baseUrlNotConfiguredBedrock')
  }
  if (error === 'base_url_not_configured') {
    return t('providerPool.healthErrors.baseUrlNotConfigured')
  }
  if (error === 'network_error') return t('providerPool.healthErrors.networkError') + keyInfo
  if (error === 'certificate_error')
    return t('providerPool.healthErrors.certificateError') + keyInfo
  if (error === 'timeout_error') return t('providerPool.healthErrors.timeoutError') + keyInfo
  if (error === 'connection_error') return t('providerPool.healthErrors.connectionError') + keyInfo
  if (error === 'endpoint_not_found')
    return t('providerPool.healthErrors.endpointNotFound') + keyInfo
  if (error.startsWith('unexpected_status:')) {
    const code = error.split(':')[1]
    return t('providerPool.healthErrors.unexpectedStatus', { code }) + keyInfo
  }
  // Fallback: return raw error for legacy/unknown codes
  return error + keyInfo
}

// Computed
const filteredProviders = computed(() => {
  let providers: Provider[] = providersForTab(activeTab.value)

  if (searchQuery.value && providers.length > 0) {
    const query = searchQuery.value.toLowerCase()
    providers = providers.filter(
      (p) => p.name.toLowerCase().includes(query) || p.id.toLowerCase().includes(query)
    )
  }

  // Sort: enabled first, then by priority (descending)
  return providers.sort((a, b) => {
    if (a.enabled !== b.enabled) {
      return a.enabled ? -1 : 1
    }
    return b.priority - a.priority
  })
})

const displayedProviders = computed(() => providerPreviewOrder.value ?? filteredProviders.value)

// Check if selected provider belongs to current tab
const currentTabSelectedProvider = computed(() => {
  if (!store.selectedProvider) return null
  if (activeTab.value === 'all') return store.selectedProvider
})

// Get custom pricing for a specific model in the selected provider
function getModelCustomPricing(modelId: string) {
  const providerId = store.selectedProviderId
  return store.customPricing.find(
    (p) => p.model_id === modelId && (!p.provider_id || p.provider_id === providerId)
  )
}

// Check if model has custom pricing
function hasCustomPricing(modelId: string): boolean {
  return !!getModelCustomPricing(modelId)
}

// Get localized provider name
function getProviderName(provider: Provider): string {
  // For trial provider, use i18n name
  if (provider.type === 'trial') {
    return t('providerPool.trial.name')
  }
  return provider.name
}

function formatKeyLabel(label: string): string {
  if (label.startsWith('ide_import:')) {
    const ide = label.substring('ide_import:'.length)
    return t('providerPool.ideImportLabel', { ide })
  }
  return label
}

// Get localized provider description
function getProviderDescription(provider: Provider): string {
  // Try to get i18n description first
  const i18nKey = `providerPool.providers.${provider.id}`
  if (te(i18nKey)) return t(i18nKey)
  // Fall back to provider's description. Keep official provider cards free of URL noise.
  if (provider.description) return provider.description
  if (provider.metadata_mode === 'catalog') return ''
  return provider.base_url || ''
}

// Reset to 'all' if current tab is no longer available (e.g., trial removed)
watch(availableTabs, (tabs) => {
  if (!tabs.includes(activeTab.value)) activeTab.value = 'all'
})

// Clear selection when switching to a tab with no matching provider
watch(activeTab, () => {
  if (store.selectedProvider && activeTab.value !== 'all') {
    const expectedTab = getProviderTab(store.selectedProvider)

    // If selected provider doesn't belong to new tab, clear selection
    if (expectedTab !== activeTab.value) {
      store.selectProvider(null)
    }
  }
})

watch(
  () => displayProvider.value?.id,
  () => {
    verificationResult.value = null
    verificationError.value = ''
    verificationKeyId.value = ''
    selectedKeyId.value = ''
    showDeleteFor.value = null
  }
)

function startLongPress(target: string) {
  cancelLongPress()
  longPressTimer = setTimeout(() => {
    showDeleteFor.value = target
  }, 450)
}

function cancelLongPress() {
  if (longPressTimer) {
    clearTimeout(longPressTimer)
    longPressTimer = null
  }
}

// Methods
async function loadData() {
  try {
    await store.fetchProviders()
    await store.fetchCustomPricing()
  } catch (e) {
    console.error('Failed to load data:', e)
  }
}

async function toggleProvider(provider: Provider) {
  try {
    if (provider.enabled) {
      await store.disableProvider(provider.id)
    } else {
      await store.enableProvider(provider.id)
    }
  } catch (e) {
    console.error('Failed to toggle provider:', e)
  }
}

async function updateProviderLocation(providerId: string, location: 'cloud' | 'local') {
  const provider = store.providers.find((item) => item.id === providerId)
  if (!canEditProviderLocation(provider)) return

  try {
    await store.updateProvider(providerId, { location })
  } catch (e) {
    console.error('Failed to update provider location:', e)
  }
}

async function testConnection(providerId: string, keyId?: string) {
  testingProvider.value = providerId
  testingKeyId.value = keyId ?? null
  try {
    const result = await store.testProvider(providerId, keyId)
    if (keyId && result) {
      keyTestResults.value[keyId] = { healthy: result.healthy, error: result.error }
    }
  } catch (e) {
    console.error('Failed to test provider:', e)
    if (keyId) {
      keyTestResults.value[keyId] = { healthy: false, error: 'request_failed' }
    }
  } finally {
    testingProvider.value = null
    testingKeyId.value = null
  }
}

async function clearProviderError(providerId: string) {
  try {
    await store.clearProviderError(providerId)
    notification.success(t('providerPool.errorCleared'), undefined, {
      titleKey: 'providerPool.errorCleared',
    })
  } catch (e) {
    console.error('Failed to clear error:', e)
  }
}

async function refreshModels(providerId: string) {
  refreshingModels.value = providerId
  try {
    const provider = store.providers.find((item) => item.id === providerId)
    if (provider?.metadata_mode === 'catalog') {
      const fetchResult = await store.refreshModels(providerId)
      if (!fetchResult.success) {
        notification.error(t('providerPool.refreshModelsFailed'), fetchResult.error, {
          duration: 8000,
          titleKey: 'providerPool.refreshModelsFailed',
        })
      }
      return
    }

    // Smart refresh: try probe first (tests actual availability), fall back to fetch
    const probeResult = await store.probeModels(providerId)
    if (probeResult.success) {
      notification.success(
        t('providerPool.probeComplete', {
          available: probeResult.available,
          total: probeResult.total,
        }),
        undefined,
        {
          titleKey: 'providerPool.probeComplete',
          titleParams: { available: probeResult.available, total: probeResult.total },
        }
      )
      return
    }

    // Probe failed (maybe no cached models yet) — fall back to fetch
    const fetchResult = await store.refreshModels(providerId)
    if (!fetchResult.success) {
      notification.error(t('providerPool.refreshModelsFailed'), fetchResult.error, {
        duration: 8000,
        titleKey: 'providerPool.refreshModelsFailed',
      })
    }
  } finally {
    refreshingModels.value = null
  }
}

async function runProviderVerification(apply = false) {
  const provider = displayProvider.value
  if (!provider) return
  if (!supportsProviderVerification.value) return
  if (!provider.base_url) {
    notification.error(t('providerPool.verifyFailed'), t('providerPool.baseUrlRequired'), {
      titleKey: 'providerPool.verifyFailed',
      messageKey: 'providerPool.baseUrlRequired',
    })
    return
  }

  if (apply) {
    applyingVerification.value = provider.id
  } else {
    verifyingProvider.value = provider.id
  }
  verificationError.value = ''

  try {
    const result = await store.verifyProviderRecommendation(
      provider.id,
      apply,
      verificationKeyId.value || undefined
    )
    verificationResult.value = result.verification

    if (apply) {
      if (result.applied) {
        notification.success(t('providerPool.verifyApplied'), undefined, {
          titleKey: 'providerPool.verifyApplied',
        })
      } else {
        notification.info(t('providerPool.verifyNoChanges'), undefined, {
          titleKey: 'providerPool.verifyNoChanges',
        })
      }
    } else {
      notification.success(t('providerPool.verifySuccess'), undefined, {
        titleKey: 'providerPool.verifySuccess',
      })
    }
  } catch (e: any) {
    const message =
      e?.response?.data?.error || (e instanceof Error ? e.message : t('providerPool.verifyFailed'))
    verificationError.value = message
    notification.error(t('providerPool.verifyFailed'), message, {
      titleKey: 'providerPool.verifyFailed',
    })
  } finally {
    if (apply) {
      applyingVerification.value = null
    } else {
      verifyingProvider.value = null
    }
  }
}

function getVerificationProbeLabel(key: string): string {
  const labels: Record<string, string> = {
    models: '/v1/models',
    anthropic_messages: '/v1/messages',
    chat_completions: '/v1/chat/completions',
    responses_v1: '/v1/responses',
    responses_plain: '/responses',
  }
  return labels[key] || key
}

async function addCustomProvider() {
  addingProvider.value = true
  addingStep.value = 'adding'
  try {
    const trimmedApiKey = newProvider.value.api_key.trim()
    // Step 1: Create provider (with optional API key — backend saves it atomically)
    const provider = await store.addProvider({
      name: newProvider.value.name,
      base_url: newProvider.value.base_url,
      api_key: trimmedApiKey || undefined,
      priority: newProvider.value.priority,
      location: newProvider.value.location,
      api_format: newProvider.value.format === 'auto' ? undefined : newProvider.value.format,
      api_format_mode: newProvider.value.format === 'auto' ? 'auto' : 'pinned',
      type: 'custom',
    } as any)

    const providerId = provider.id

    // Step 2: Fetch & probe models (with timeout so UI doesn't hang)
    addingStep.value = 'probing'

    let available = 0
    let total = 0
    // Keep the add-flow probe step snappy so the modal does not feel blocked.
    const discoveryDeadline = Date.now() + 12000
    const remainingDiscoveryMs = () => Math.max(0, discoveryDeadline - Date.now())

    try {
      // Fetch models quickly; if the provider is slow/unreachable, continue without blocking.
      const fetchTimeoutMs = remainingDiscoveryMs()
      const fetchResult = await Promise.race([
        store.refreshModels(providerId),
        new Promise<{ success: false; error: string; models: never[] }>((resolve) =>
          setTimeout(
            () => resolve({ success: false, error: 'timeout', models: [] }),
            fetchTimeoutMs
          )
        ),
      ])

      if (fetchResult.success && fetchResult.models && fetchResult.models.length > 0) {
        const probeTimeoutMs = remainingDiscoveryMs()
        if (probeTimeoutMs > 0) {
          const probeResult = await Promise.race([
            // Probe with higher concurrency during add-flow to reduce waiting time.
            store.probeModels(providerId, 10),
            new Promise<{
              success: false
              error: string
              total: number
              available: number
              unavailable: number
              results: never[]
            }>((resolve) =>
              setTimeout(
                () =>
                  resolve({
                    success: false,
                    error: 'timeout',
                    total: 0,
                    available: 0,
                    unavailable: 0,
                    results: [],
                  }),
                probeTimeoutMs
              )
            ),
          ])
          if (probeResult.success) {
            available = probeResult.available
            total = probeResult.total
          } else {
            total = fetchResult.models.length
            available = total
          }
        } else {
          total = fetchResult.models.length
          available = total
        }
      }
    } catch {
      // Fetch/probe failed — provider is still saved, just no model info yet
    }

    // Step 3: Auto-enable if we found available models
    if (available > 0) {
      await store.enableProvider(providerId)
      notification.success(
        t('providerPool.providerAdded'),
        t('providerPool.providerAddedWithModels', { count: available }),
        {
          titleKey: 'providerPool.providerAdded',
          messageKey: 'providerPool.providerAddedWithModels',
          messageParams: { count: available },
        }
      )
    } else if (total > 0) {
      notification.info(t('providerPool.providerAdded'), t('providerPool.noAvailableModels'), {
        titleKey: 'providerPool.providerAdded',
        messageKey: 'providerPool.noAvailableModels',
      })
    } else {
      notification.info(t('providerPool.providerAdded'), t('providerPool.noModelsFound'), {
        titleKey: 'providerPool.providerAdded',
        messageKey: 'providerPool.noModelsFound',
      })
    }

    // Auto-select the new provider
    store.selectProvider(providerId)

    closeAddProviderModal()
    resetNewProviderForm()
  } catch (e) {
    console.error('Failed to add provider:', e)
    notification.error(t('providerPool.addFailed'), e instanceof Error ? e.message : '', {
      titleKey: 'providerPool.addFailed',
    })
  } finally {
    addingProvider.value = false
    addingStep.value = ''
  }
}

function resetNewProviderForm() {
  showNewProviderApiKey.value = false
  newProvider.value = {
    name: '',
    base_url: '',
    api_key: '',
    priority: 50,
    location: 'cloud',
    format: 'auto',
  }
}

function openCustomProviderForm() {
  addProviderMode.value = 'custom'
}

function selectOfficialProviderOption(provider: Provider) {
  activeTab.value = visibleTab(getProviderTab(provider))
  store.selectProvider(provider.id)
  closeAddProviderModal()
  if (!provider.oauth && !(provider.api_keys && provider.api_keys.length > 0)) {
    openKeyModal(provider.id)
  }
}

async function handleIDEImportSuccess(providerId: string) {
  await loadData()

  if (!providerId || providerId === 'oauth-auto') {
    return
  }

  const imported = store.providers.find((p) => p.id === providerId)
  if (!imported || !isProviderVisibleInUI(imported)) {
    return
  }
  activeTab.value = visibleTab(getProviderTab(imported))
  store.selectProvider(providerId)
}

async function deleteProvider(providerId: string) {
  if (!confirm(t('providerPool.confirmDelete'))) return
  try {
    await store.deleteProvider(providerId)
  } catch (e) {
    console.error('Failed to delete provider:', e)
  }
}

function openKeyModal(providerId: string) {
  newKey.value.providerId = providerId
  newKey.value.key = ''
  newKey.value.label = ''
  showNewKeyApiKey.value = false
  showKeyModal.value = true
}

async function addAPIKey() {
  const providerId = newKey.value.providerId
  try {
    const trimmedApiKey = newKey.value.key.trim()
    const apiKey = await store.addAPIKey(providerId, trimmedApiKey, newKey.value.label || undefined)
    showKeyModal.value = false

    // Clear error when adding new API key
    await store.clearProviderError(providerId)

    // Auto-test the newly added key for all provider types
    const keyId = apiKey?.id
    if (keyId) {
      testConnection(providerId, keyId)
    }

    // Dynamic builtin/platform providers still benefit from a background probe.
    const provider = store.providers.find((p) => p.id === providerId)
    if (supportsProviderAccountStatus(provider)) {
      store.fetchAccountStatus(providerId, getProviderAccountStatusKey(provider))
    }
    if (
      provider &&
      provider.metadata_mode !== 'catalog' &&
      (provider.type === 'builtin' || provider.type === 'platform')
    ) {
      store.probeModels(providerId).then((result) => {
        if (result.success && result.available > 0) {
          notification.success(
            t('providerPool.probeComplete', { available: result.available, total: result.total }),
            undefined,
            {
              titleKey: 'providerPool.probeComplete',
              titleParams: { available: result.available, total: result.total },
            }
          )
        }
      })
    }
  } catch (e) {
    console.error('Failed to add API key:', e)
  }
}

async function removeAPIKey(providerId: string, keyId: string) {
  try {
    await store.removeAPIKey(providerId, keyId)
    // Clear test result for removed key
    delete keyTestResults.value[keyId]
    const provider = store.providers.find((p) => p.id === providerId)
    if (supportsProviderAccountStatus(provider) && provider?.api_keys?.length) {
      store.fetchAccountStatus(providerId, getProviderAccountStatusKey(provider))
    } else {
      store.clearAccountStatus(providerId)
    }
  } catch (e) {
    console.error('Failed to remove API key:', e)
  }
}

// OAuth state
const connectingOAuth = ref<string | null>(null)
const disconnectingOAuth = ref<string | null>(null)
const disconnectingAccountId = ref<string | null>(null)
const deviceFlowState = ref<{
  userCode: string
  verificationUri: string
  deviceCode: string
  providerId: string
} | null>(null)
const oauthAccounts = ref<Record<string, import('@/api/providerPool').OAuthAccount[]>>({})

function sanitizeDeviceUserCode(raw: string): string {
  return raw
    .normalize('NFKC')
    .replace(/[\u200B-\u200D\uFEFF]/g, '')
    .replace(/[‐‑‒–—―]/g, '-')
    .replace(/\s+/g, '')
    .toUpperCase()
}

// Provider usage state
interface ProviderUsageSummary {
  total_input_tokens: number
  total_output_tokens: number
  total_requests: number
  successful_requests: number
  failed_requests: number
  total_estimated_cost: number
  avg_latency_ms: number
}
const providerUsage = ref<ProviderUsageSummary | null>(null)
const loadingUsage = ref(false)

async function fetchProviderUsage(providerId: string) {
  loadingUsage.value = true
  providerUsage.value = null
  try {
    const res = await providerPoolApi.getProviderUsage(providerId)
    providerUsage.value = res.data.summary || null
  } catch {
    providerUsage.value = null
  } finally {
    loadingUsage.value = false
  }
}

function tierBadgeClass(tier: string): string {
  const t = tier.toUpperCase()
  if (t === 'FREE') return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
  if (t === 'PRO' || t === 'BUSINESS')
    return 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
  if (t === 'ULTRA' || t === 'PREMIUM' || t === 'ENTERPRISE' || t === 'MAX')
    return 'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300'
  return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
}

function tierListBadgeClass(tier: string): string {
  const t = tier.toUpperCase()
  if (t === 'PRO' || t === 'PRO+' || t === 'BUSINESS')
    return 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400'
  if (t === 'ULTRA' || t === 'PREMIUM' || t === 'ENTERPRISE' || t === 'MAX')
    return 'bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400'
  return 'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400'
}

function formatAccountStatusAmount(item?: { value: number; currency?: string } | null): string {
  if (!item) return '-'
  const value = Number(item.value || 0)
  const formatted = value.toLocaleString(undefined, {
    minimumFractionDigits: value >= 100 ? 0 : 2,
    maximumFractionDigits: value >= 100 ? 2 : 4,
  })
  const currency = (item.currency || '').toUpperCase()
  if (currency === 'USD') return `$${formatted}`
  if (currency === 'CNY') return `¥${formatted}`
  return currency ? `${formatted} ${currency}` : formatted
}

function getAccountStatusItemLabel(itemKey: string): string {
  switch (itemKey) {
    case 'remaining':
      return tr('providerPool.accountStatus.items.remaining', 'Remaining')
    case 'used':
      return tr('providerPool.accountStatus.items.used', 'Used')
    case 'limit':
      return tr('providerPool.accountStatus.items.limit', 'Limit')
    case 'granted':
      return tr('providerPool.accountStatus.items.granted', 'Granted')
    case 'topped_up':
      return tr('providerPool.accountStatus.items.toppedUp', 'Top-up')
    default:
      return itemKey
  }
}

function getAccountStatusKeyLabel(keyHash?: string): string {
  if (!keyHash) return ''
  return trp('providerPool.accountStatus.usingKey', { key: keyHash }, `Using key ${keyHash}`)
}

async function startOAuthConnect(providerId: string) {
  connectingOAuth.value = providerId
  try {
    const result = await providerPoolApi.startOAuth(providerId)
    if (result.data.auth_url) {
      // Auth code flow — open browser
      window.open(result.data.auth_url, '_blank', 'width=600,height=700')
      notification.success(t('providerPool.oauth.browserOpened'), undefined, {
        titleKey: 'providerPool.oauth.browserOpened',
      })
      // Poll for status
      pollOAuthStatus(providerId)
    } else if (result.data.device_code && result.data.user_code) {
      // Device code flow — show code to user
      deviceFlowState.value = {
        userCode: sanitizeDeviceUserCode(result.data.user_code),
        verificationUri: result.data.verification_uri || 'https://github.com/login/device',
        deviceCode: result.data.device_code,
        providerId,
      }
    }
  } catch (e: any) {
    notification.error(
      e?.response?.data?.error || t('providerPool.oauth.connectFailed'),
      undefined,
      { titleKey: e?.response?.data?.error ? undefined : 'providerPool.oauth.connectFailed' }
    )
  } finally {
    connectingOAuth.value = null
  }
}

async function copyDeviceFlowCode() {
  if (!deviceFlowState.value) return
  try {
    await navigator.clipboard.writeText(sanitizeDeviceUserCode(deviceFlowState.value.userCode))
    notification.success(t('common.copied'), undefined, { titleKey: 'common.copied' })
  } catch (e) {
    console.error('Failed to copy device flow code:', e)
    notification.error(t('providerPool.oauth.connectFailed'), undefined, {
      titleKey: 'providerPool.oauth.connectFailed',
    })
  }
}

async function completeDeviceFlow() {
  if (!deviceFlowState.value) return
  const { providerId, deviceCode } = deviceFlowState.value
  connectingOAuth.value = providerId
  try {
    await providerPoolApi.completeDeviceFlow(providerId, deviceCode)
    notification.success(t('providerPool.oauth.connected'), undefined, {
      titleKey: 'providerPool.oauth.connected',
    })
    deviceFlowState.value = null
    await store.fetchProviders()
    await fetchOAuthAccounts(providerId)
  } catch (e: any) {
    notification.error(
      e?.response?.data?.error || t('providerPool.oauth.connectFailed'),
      undefined,
      { titleKey: e?.response?.data?.error ? undefined : 'providerPool.oauth.connectFailed' }
    )
  } finally {
    connectingOAuth.value = null
  }
}

async function disconnectOAuth(providerId: string, accountId?: string) {
  disconnectingOAuth.value = providerId
  disconnectingAccountId.value = accountId || null
  try {
    await providerPoolApi.disconnectOAuth(providerId, accountId)
    notification.success(t('providerPool.oauth.disconnected'), undefined, {
      titleKey: 'providerPool.oauth.disconnected',
    })
    await store.fetchProviders()
    // Refresh accounts list
    await fetchOAuthAccounts(providerId)
  } catch (e: any) {
    notification.error(
      e?.response?.data?.error || t('providerPool.oauth.disconnectFailed'),
      undefined,
      { titleKey: e?.response?.data?.error ? undefined : 'providerPool.oauth.disconnectFailed' }
    )
  } finally {
    disconnectingOAuth.value = null
    disconnectingAccountId.value = null
  }
}

async function fetchOAuthAccounts(providerId: string) {
  try {
    const result = await providerPoolApi.getOAuthAccounts(providerId)
    oauthAccounts.value[providerId] = result.data.accounts || []
  } catch {
    oauthAccounts.value[providerId] = []
  }
}

function pollOAuthStatus(providerId: string, attempts = 0) {
  if (attempts > 60) return // 5 min timeout
  setTimeout(async () => {
    try {
      const result = await providerPoolApi.getOAuthStatus(providerId)
      if (result.data.connected) {
        notification.success(t('providerPool.oauth.connected'), undefined, {
          titleKey: 'providerPool.oauth.connected',
        })
        await store.fetchProviders()
        await fetchOAuthAccounts(providerId)
        return
      }
    } catch {
      /* ignore */
    }
    pollOAuthStatus(providerId, attempts + 1)
  }, 5000)
}

function selectProvider(providerId: string) {
  store.selectProvider(store.selectedProviderId === providerId ? null : providerId)
}

// Drag and drop handlers
function resetProviderDragState() {
  draggedProvider.value = null
  dragOverProvider.value = null
  providerDragBaseOrder.value = null
  providerPreviewOrder.value = null
}

function reorderProviderList(providers: Provider[], draggedId: string, targetId: string) {
  const reorderedProviders = [...providers]
  const draggedIndex = reorderedProviders.findIndex((provider) => provider.id === draggedId)
  const targetIndex = reorderedProviders.findIndex((provider) => provider.id === targetId)

  if (draggedIndex === -1 || targetIndex === -1 || draggedIndex === targetIndex) {
    return null
  }

  const [removed] = reorderedProviders.splice(draggedIndex, 1)
  if (!removed) {
    return null
  }
  reorderedProviders.splice(targetIndex, 0, removed)
  return reorderedProviders
}

function syncProviderOrder(providers: Provider[]) {
  // Keep the current priority spacing so persisted order still matches the existing sort rules.
  const maxPriority = 100
  const step = Math.floor(maxPriority / (providers.length + 1))
  const updates: Array<{ id: string; priority: number }> = []

  for (let i = 0; i < providers.length; i++) {
    const provider = providers[i]
    if (!provider) continue
    const newPriority = maxPriority - i * step
    if (provider.priority !== newPriority) {
      store.updateProviderPriorityLocal(provider.id, newPriority)
      updates.push({ id: provider.id, priority: newPriority })
    }
  }

  if (updates.length > 0) {
    store.syncPrioritiesToBackend(updates)
  }
}

function handleDragStart(e: DragEvent, provider: Provider) {
  if (!canReorderProviders.value) return
  draggedProvider.value = provider
  providerDragBaseOrder.value = [...filteredProviders.value]
  providerPreviewOrder.value = null
  dragOverProvider.value = null
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', provider.id)
  }
}

function handleDragOver(e: DragEvent, provider: Provider) {
  if (!canReorderProviders.value) return
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
  if (
    draggedProvider.value &&
    draggedProvider.value.id !== provider.id &&
    dragOverProvider.value !== provider.id
  ) {
    const baseOrder = providerDragBaseOrder.value ?? filteredProviders.value
    const previewOrder = reorderProviderList(baseOrder, draggedProvider.value.id, provider.id)
    if (previewOrder) {
      providerPreviewOrder.value = previewOrder
      dragOverProvider.value = provider.id
    }
  }
}

function handleDragLeave() {
  if (!canReorderProviders.value) return
  if (draggedProvider.value) return
  dragOverProvider.value = null
}

function handleDragEnd() {
  if (!canReorderProviders.value) return
  resetProviderDragState()
}

async function handleDrop(e: DragEvent, targetProvider: Provider) {
  if (!canReorderProviders.value) return
  e.preventDefault()

  if (!draggedProvider.value || draggedProvider.value.id === targetProvider.id) {
    resetProviderDragState()
    return
  }

  const baseOrder = providerDragBaseOrder.value ?? filteredProviders.value
  const providers =
    providerPreviewOrder.value ??
    reorderProviderList(baseOrder, draggedProvider.value.id, targetProvider.id)

  if (!providers) {
    resetProviderDragState()
    return
  }

  syncProviderOrder(providers)
  resetProviderDragState()
}

function getStatusColor(status: string, enabled: boolean) {
  if (!enabled) return 'text-gray-500'
  switch (status) {
    case 'active':
      return 'text-green-500'
    case 'error':
      return 'text-red-500'
    default:
      return 'text-green-500' // enabled but not yet tested
  }
}

function getStatusIcon(status: string, enabled: boolean) {
  if (!enabled) return '○'
  switch (status) {
    case 'active':
      return '●'
    case 'error':
      return '⚠'
    default:
      return '●' // enabled but not yet tested
  }
}

// Model drag-to-reorder
function handleModelDragStart(e: DragEvent, model: Model) {
  draggedModel.value = model
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function handleModelDragOver(e: DragEvent, model: Model) {
  e.preventDefault()
  if (draggedModel.value && draggedModel.value.id !== model.id) {
    dragOverModel.value = model.id
  }
}

function handleModelDragLeave() {
  dragOverModel.value = null
}

async function handleModelDrop(e: DragEvent, targetModel: Model) {
  e.preventDefault()
  dragOverModel.value = null
  if (!draggedModel.value || draggedModel.value.id === targetModel.id) {
    draggedModel.value = null
    return
  }
  const models = [...displayModels.value]
  const dragIdx = models.findIndex((m) => m.id === draggedModel.value!.id)
  const targetIdx = models.findIndex((m) => m.id === targetModel.id)
  if (dragIdx === -1 || targetIdx === -1) {
    draggedModel.value = null
    return
  }
  const [moved] = models.splice(dragIdx, 1)
  if (!moved) {
    draggedModel.value = null
    return
  }
  models.splice(targetIdx, 0, moved)
  draggedModel.value = null

  // Save reordered model IDs as allowed_models (order = priority)
  const provider = displayProvider.value
  if (provider) {
    const orderedIds = models.map((m) => m.id)
    await store.updateProvider(provider.id, { allowed_models: orderedIds })
  }
}

function handleModelDragEnd() {
  draggedModel.value = null
  dragOverModel.value = null
}

const capabilityKeys = [
  'chat',
  'completion',
  'vision',
  'function_call',
  'thinking',
  'streaming',
  'json',
  'system_prompt',
  'image_generation',
  'video_generation',
  'audio_generation',
]

const capabilityEmoji: Record<string, string> = {
  chat: '💬',
  completion: '📝',
  vision: '👁',
  function_call: '🔧',
  thinking: '🧠',
  streaming: '⚡',
  json: '📋',
  system_prompt: '📌',
  image_generation: '🖼',
  video_generation: '🎬',
  audio_generation: '🔊',
}

const capabilityI18nKey: Record<string, string> = {
  chat: 'providerPool.capChat',
  completion: 'providerPool.capCompletion',
  vision: 'providerPool.capVision',
  function_call: 'providerPool.capFunctionCall',
  thinking: 'providerPool.capThinking',
  streaming: 'providerPool.capStreaming',
  json: 'providerPool.capJSON',
  system_prompt: 'providerPool.capSystemPrompt',
  image_generation: 'providerPool.capImageGeneration',
  video_generation: 'providerPool.capVideoGeneration',
  audio_generation: 'providerPool.capAudioGeneration',
}

function capabilityTip(caps: string[]): string {
  return caps
    .map((c) => {
      const key = capabilityI18nKey[c]
      const fallback = c
      return `${capabilityEmoji[c] || '•'} ${key ? tr(key, fallback) : fallback}`
    })
    .join('\n')
}

// Tooltip state for capability hover
const capTipVisible = ref(false)
const capTipText = ref('')
const capTipStyle = ref<Record<string, string>>({ top: '0px', insetInlineStart: '0px' })

function showCapTip(ev: MouseEvent, caps: string[]) {
  const rect = (ev.currentTarget as HTMLElement).getBoundingClientRect()
  const inlineStartOffset = isRtl.value
    ? `${window.innerWidth - rect.x + 8}px`
    : `${rect.x + rect.width + 8}px`
  capTipText.value = capabilityTip(caps)
  capTipStyle.value = {
    top: `${rect.top + rect.height / 2}px`,
    insetInlineStart: inlineStartOffset,
  }
  capTipVisible.value = true
}

function hideCapTip() {
  capTipVisible.value = false
}

function isMediaModel(model: Model): boolean {
  const caps = model.capabilities || []
  return caps.some(
    (c) => c === 'image_generation' || c === 'video_generation' || c === 'audio_generation'
  )
}

function mediaPricingLabel(model: Model): string {
  const price = model.price_per_request ?? 0
  const unit = model.pricing_unit || 'image'
  if (price === 0) return '—'
  const unitLabel: Record<string, string> = {
    image: t('providerPool.perImage', '/img'),
    second: t('providerPool.perSecond', '/sec'),
    video: t('providerPool.perVideo', '/vid'),
  }
  return `$${price.toFixed(3)}${unitLabel[unit] || `/${unit}`}`
}

function openPricingModal(model?: Model) {
  if (model) {
    const customPricing = getModelCustomPricing(model.id)
    const media = isMediaModel(model)
    pricingForm.value = {
      modelId: model.id,
      providerId: model.provider_id,
      inputPrice: customPricing?.input_price ?? model.input_price ?? 0,
      outputPrice: customPricing?.output_price ?? model.output_price ?? 0,
      cachePrice: customPricing?.cache_price ?? 0,
      isMedia: media,
      pricePerRequest: model.price_per_request ?? 0,
      pricingUnit: model.pricing_unit || 'image',
    }
  } else {
    pricingForm.value = {
      modelId: '',
      providerId: store.selectedProviderId || '',
      inputPrice: 0,
      outputPrice: 0,
      cachePrice: 0,
      isMedia: false,
      pricePerRequest: 0,
      pricingUnit: 'image',
    }
  }
  showPricingModal.value = true
}

async function saveModelPricing() {
  try {
    await store.setModelPricing(pricingForm.value.modelId, {
      provider_id: pricingForm.value.providerId || undefined,
      input_price: pricingForm.value.inputPrice,
      output_price: pricingForm.value.outputPrice,
      cache_price: pricingForm.value.cachePrice,
    })
    showPricingModal.value = false
  } catch (e) {
    console.error('Failed to save model pricing:', e)
  }
}

async function removeCustomPricing(modelId: string, providerId?: string) {
  if (!confirm(t('providerPool.confirmRemovePricing'))) return
  try {
    await store.removeModelPricing(modelId, providerId)
  } catch (e) {
    console.error('Failed to remove custom pricing:', e)
  }
}

// Model params methods
function openParamsModal() {
  const provider = displayProvider.value
  if (provider) {
    paramsForm.value = {
      temperature: provider.model_params?.temperature,
      maxTokens: provider.model_params?.max_tokens,
      topP: provider.model_params?.top_p,
    }
  }
  showParamsModal.value = true
}

async function saveModelParams() {
  const provider = displayProvider.value
  if (!provider) return

  try {
    await store.updateModelParams(provider.id, {
      temperature: paramsForm.value.temperature,
      max_tokens: paramsForm.value.maxTokens,
      top_p: paramsForm.value.topP,
    })
    showParamsModal.value = false
  } catch (e) {
    console.error('Failed to save model params:', e)
  }
}

async function detectCapabilities() {
  const provider = displayProvider.value
  if (!provider || provider.metadata_mode === 'catalog') return

  detectingCapabilities.value = provider.id
  try {
    await store.detectCapabilities(provider.id)
  } catch (e) {
    console.error('Failed to detect capabilities:', e)
  } finally {
    detectingCapabilities.value = null
  }
}

// Allowed models methods
async function openAllowedModelsModal() {
  const provider = displayProvider.value
  if (!provider) return

  loadingAllModels.value = true
  showAllowedModelsModal.value = true

  try {
    // Fetch all models from API (unfiltered) by refreshing
    const result = await store.refreshModels(provider.id)
    if (result.success) {
      allAvailableModels.value = result.models || []
    } else {
      // Fall back to currently displayed models
      allAvailableModels.value = selectedProviderModels.value
    }

    // Initialize form: if allowed_models has values, use them; otherwise default to all models
    if (provider.allowed_models && provider.allowed_models.length > 0) {
      allowedModelsForm.value = [...provider.allowed_models]
    } else {
      // No restriction configured - default to all models selected
      allowedModelsForm.value = allAvailableModels.value.map((m) => m.id)
    }
  } catch (e) {
    console.error('Failed to load models:', e)
    allAvailableModels.value = selectedProviderModels.value
    // On error, default to all models selected
    allowedModelsForm.value = allAvailableModels.value.map((m) => m.id)
  } finally {
    loadingAllModels.value = false
  }
}

function toggleModelInAllowedList(modelId: string) {
  // Simple toggle: checked = in array, unchecked = not in array
  if (allowedModelsForm.value.includes(modelId)) {
    allowedModelsForm.value = allowedModelsForm.value.filter((id) => id !== modelId)
  } else {
    allowedModelsForm.value = [...allowedModelsForm.value, modelId]
  }
}

function selectAllModels() {
  allowedModelsForm.value = allAvailableModels.value.map((m) => m.id)
}

function clearAllModels() {
  allowedModelsForm.value = []
}

async function saveAllowedModels() {
  const provider = displayProvider.value
  if (!provider) return

  savingAllowedModels.value = true
  try {
    await store.updateAllowedModels(provider.id, allowedModelsForm.value)
    // Refresh models to show filtered list
    await store.fetchModels(provider.id)
    showAllowedModelsModal.value = false
  } catch (e) {
    console.error('Failed to save allowed models:', e)
  } finally {
    savingAllowedModels.value = false
  }
}

function openIconUpload() {
  iconInput.value?.click()
}

async function handleIconUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !displayProvider.value) return

  // Validate file size (max 500KB)
  if (file.size > 500 * 1024) {
    console.error('Icon file too large (max 500KB)')
    return
  }

  // Validate file type
  if (!file.type.startsWith('image/')) {
    console.error('Invalid file type')
    return
  }

  uploadingIcon.value = true
  try {
    // Convert to base64 data URL
    const reader = new FileReader()
    reader.onload = async (e) => {
      const dataUrl = e.target?.result as string
      if (dataUrl && displayProvider.value) {
        await store.updateProviderIcon(displayProvider.value.id, dataUrl)
      }
      uploadingIcon.value = false
    }
    reader.onerror = () => {
      console.error('Failed to read file')
      uploadingIcon.value = false
    }
    reader.readAsDataURL(file)
  } catch (e) {
    console.error('Failed to upload icon:', e)
    uploadingIcon.value = false
  }

  // Reset input
  input.value = ''
}

onMounted(() => {
  loadData()
  // If route has section=media (e.g. from "Go to Settings" link), switch to media tab once available
  if (route.query.section === 'media') {
    const stop = watch(
      availableTabs,
      (tabs) => {
        if (tabs.includes('media')) {
          activeTab.value = 'media'
          stop()
        }
      },
      { immediate: true }
    )
  }
})
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <div>
        <p class="text-sm text-gray-500 dark:text-slate-400">{{ t('providerPool.description') }}</p>
      </div>
      <div class="flex gap-2">
        <button
          class="px-3 py-1.5 bg-gray-200 dark:bg-slate-700 hover:bg-gray-300 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-200 rounded-lg flex items-center gap-1 text-sm transition-colors"
          @click="showIDEDiscoveryModal = true"
        >
          <span>⌁</span>
          {{ t('providerPool.scanIDE') }}
        </button>
        <button
          class="px-3 py-1.5 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg flex items-center gap-1 text-sm transition-colors"
          @click="openAddProviderModal"
        >
          <span>+</span>
          {{ tr('providerPool.addProvider', t('providerPool.addCustom')) }}
        </button>
      </div>
    </div>

    <!-- Tabs -->
    <div class="flex gap-2 mb-4">
      <button
        v-for="tab in availableTabs"
        :key="tab"
        :class="[
          'px-3 py-1.5 rounded-lg transition-colors text-sm',
          activeTab === tab
            ? 'bg-gray-700 dark:bg-gray-500 text-white'
            : 'bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-slate-600',
        ]"
        @click="activeTab = tab"
      >
        {{ getTabLabel(tab) }}
        <span class="provider-inline-start-1 text-xs opacity-70"> ({{ getTabCount(tab) }}) </span>
      </button>
    </div>

    <!-- Search -->
    <div class="mb-4">
      <input
        v-model="searchQuery"
        type="text"
        :placeholder="t('providerPool.search')"
        autocomplete="off"
        data-form-filler-ignore
        class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-gray-400 text-sm"
      />
    </div>

    <!-- Trial Quota Banner -->
    <div
      v-if="store.trialQuota && store.trialProviders?.length > 0"
      :class="[
        'mb-4 p-3 rounded-lg border',
        store.trialQuota.is_exhausted
          ? 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'
          : 'bg-gray-100 dark:bg-gray-700/30 border-gray-200 dark:border-gray-600',
      ]"
    >
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="text-lg">{{ store.trialQuota.is_exhausted ? '⚠️' : '🎁' }}</span>
          <div>
            <h4
              class="font-medium text-sm"
              :class="
                store.trialQuota.is_exhausted
                  ? 'text-red-700 dark:text-red-300'
                  : 'text-gray-600 dark:text-gray-400'
              "
            >
              {{ t('providerPool.trialQuota.title') }}
            </h4>
            <p
              class="text-xs"
              :class="
                store.trialQuota.is_exhausted
                  ? 'text-red-600 dark:text-red-400'
                  : 'text-gray-600 dark:text-gray-400'
              "
            >
              {{
                store.trialQuota.is_exhausted
                  ? t('providerPool.trialQuota.exhausted')
                  : t('providerPool.trialQuota.remaining', {
                      tokens: formatTokens(store.trialQuota.tokens_remaining),
                    })
              }}
            </p>
          </div>
        </div>
        <div
          class="provider-text-end text-xs"
          :class="
            store.trialQuota.is_exhausted
              ? 'text-red-500 dark:text-red-400'
              : 'text-gray-600 dark:text-gray-400'
          "
        >
          <div
            :title="
              store.trialQuota.tokens_remaining.toLocaleString() +
              ' / ' +
              store.trialQuota.token_limit.toLocaleString()
            "
          >
            {{
              t('providerPool.trialQuota.tokensRemaining', {
                remaining: formatTokens(store.trialQuota.tokens_remaining),
                limit: formatTokens(store.trialQuota.token_limit),
              })
            }}
          </div>
          <div class="mt-1 w-24 h-1.5 bg-gray-300 dark:bg-gray-600 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full transition-all"
              :class="store.trialQuota.is_exhausted ? 'bg-red-500' : 'bg-gray-400 dark:bg-gray-500'"
              :style="{
                width: `${Math.max(0, Math.min(100, (store.trialQuota.tokens_remaining / store.trialQuota.token_limit) * 100))}%`,
              }"
            ></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="text-center py-8">
      <div
        class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full mx-auto"
      ></div>
      <p class="text-gray-400 mt-2 text-sm">{{ t('common.loading') }}</p>
    </div>

    <!-- Error -->
    <div
      v-else-if="store.error"
      class="bg-red-100 dark:bg-red-900/20 border border-red-300 dark:border-red-500 rounded-lg p-3 mb-4"
    >
      <p class="text-red-600 dark:text-red-400 text-sm">{{ store.error }}</p>
      <button
        class="text-red-500 dark:text-red-300 underline mt-1 text-sm"
        @click="store.clearError()"
      >
        {{ t('common.dismiss') }}
      </button>
    </div>

    <!-- Provider List -->
    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <!-- Provider Cards -->
      <div class="space-y-2 max-h-[360px] overflow-y-auto lg:max-h-[480px]">
        <!-- Drag hint -->
        <p
          v-if="canReorderProviders && !searchQuery"
          class="text-xs text-gray-400 dark:text-gray-500 mb-2 flex items-center gap-1"
        >
          <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4"
            />
          </svg>
          {{ t('providerPool.dragToReorder') }}
        </p>
        <div
          v-for="provider in displayedProviders"
          :key="provider.id"
          :draggable="canReorderProviders"
          :class="[
            'p-3 rounded-lg border transition-all select-none group/card',
            store.selectedProviderId === provider.id
              ? 'bg-gray-100 dark:bg-gray-700/20 border-gray-300 dark:border-gray-600'
              : 'bg-gray-50 dark:bg-slate-800/50 border-gray-200 dark:border-slate-700 hover:border-gray-300 dark:hover:border-slate-600',
            dragOverProvider === provider.id
              ? 'border-gray-300 dark:border-gray-600 border-dashed bg-gray-100 dark:bg-gray-700/10'
              : '',
            draggedProvider?.id === provider.id ? 'opacity-50' : '',
            canReorderProviders && draggedProvider ? 'cursor-grabbing' : 'cursor-pointer',
          ]"
          @click="selectProvider(provider.id)"
          @dragstart="handleDragStart($event, provider)"
          @dragover="handleDragOver($event, provider)"
          @dragleave="handleDragLeave"
          @dragend="handleDragEnd"
          @drop="handleDrop($event, provider)"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <!-- Drag handle - only visible during drag -->
              <div
                v-if="canReorderProviders"
                :class="[
                  'flex-shrink-0 text-gray-400 dark:text-gray-500 cursor-grab transition-opacity',
                  draggedProvider ? 'opacity-100' : 'opacity-0',
                ]"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M4 8h16M4 16h16"
                  />
                </svg>
              </div>
              <ProviderIcon
                :provider-id="provider.id"
                :custom-icon="provider.custom_icon"
                size="lg"
              />
              <div>
                <div class="flex items-center gap-1.5">
                  <h3 class="font-medium text-gray-900 dark:text-white text-sm">
                    {{ getProviderName(provider) }}
                  </h3>
                  <span
                    v-if="provider.beta"
                    class="text-[10px] px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 tracking-wide"
                  >
                    {{ tr('providerPool.beta', 'Beta') }}
                  </span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ provider.id }}</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <!-- Free tier badge -->
              <span
                v-if="
                  provider.id === 'nvidia' ||
                  provider.id === 'github-copilot' ||
                  provider.id === 'google-antigravity' ||
                  provider.id === 'google-gemini-cli' ||
                  provider.id === 'openrouter-free'
                "
                class="text-[10px] px-1.5 py-0.5 rounded"
                :class="
                  store.oauthQuota[provider.id]?.tier &&
                  store.oauthQuota[provider.id].tier !== 'Free' &&
                  store.oauthQuota[provider.id].tier !== 'FREE'
                    ? tierListBadgeClass(store.oauthQuota[provider.id].tier)
                    : 'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400'
                "
              >
                {{ store.oauthQuota[provider.id]?.tier || t('providerPool.freeTier') }}
              </span>
              <!-- API Keys count -->
              <span
                v-if="provider.api_keys?.length"
                class="text-[10px] px-1.5 py-0.5 rounded bg-gray-200 dark:bg-slate-600 text-gray-600 dark:text-gray-300"
                :title="t('providerPool.apiKeys')"
              >
                🔑 {{ provider.api_keys.length }}
              </span>
              <!-- OAuth badge — show count only when connected -->
              <span
                v-if="provider.oauth?.connected"
                class="text-[10px] px-1.5 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400"
                :title="t('providerPool.oauth.title')"
              >
                🔗 {{ provider.oauth.account_count || 1 }}
              </span>
              <!-- Location badge -->
              <span
                :class="[
                  'text-[10px] px-1.5 py-0.5 rounded',
                  getEffectiveProviderLocation(provider) === 'cloud'
                    ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400'
                    : 'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400',
                ]"
              >
                {{ getEffectiveProviderLocation(provider) === 'cloud' ? '☁️' : '💻' }}
              </span>
              <span
                :class="getStatusColor(provider.status, provider.enabled)"
                class="text-xs"
                :title="
                  provider.status === 'error' && provider.last_error
                    ? translateHealthError(provider.last_error)
                    : ''
                "
              >
                {{ getStatusIcon(provider.status, provider.enabled) }}
              </span>
              <label class="relative inline-flex items-center cursor-pointer" @click.stop>
                <input
                  type="checkbox"
                  :checked="provider.enabled"
                  class="sr-only peer"
                  @change="toggleProvider(provider)"
                />
                <div
                  class="provider-switch-track w-8 h-4 bg-gray-300 dark:bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:bg-green-600 dark:peer-checked:bg-green-500"
                ></div>
              </label>
            </div>
          </div>
        </div>

        <div
          v-if="filteredProviders.length === 0"
          class="text-center py-6 text-gray-500 dark:text-gray-400 text-sm"
        >
          {{ t('providerPool.noProviders') }}
        </div>
      </div>

      <!-- Provider Details -->
      <div class="space-y-2 lg:max-h-[480px] lg:overflow-y-auto">
        <div
          v-if="displayProvider"
          class="bg-white dark:bg-slate-800/50 rounded-lg border border-gray-200 dark:border-slate-700 p-4"
        >
          <div
            class="flex items-center justify-between mb-4 group/header"
            @touchstart="startLongPress('provider-' + displayProvider!.id)"
            @touchend="cancelLongPress()"
            @touchcancel="cancelLongPress()"
          >
            <div class="flex items-center gap-3">
              <div class="relative group">
                <!-- Builtin provider: clickable logo to website -->
                <a
                  v-if="
                    (displayProvider.type === 'builtin' ||
                      displayProvider.type === 'platform' ||
                      displayProvider.type === 'media') &&
                    displayProvider.website
                  "
                  :href="displayProvider.website"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="block cursor-pointer hover:opacity-80 transition-opacity"
                  :title="t('providerPool.visitWebsite')"
                >
                  <ProviderIcon
                    :provider-id="displayProvider.id"
                    :custom-icon="displayProvider.custom_icon"
                    size="xl"
                  />
                </a>
                <!-- Non-builtin or no website: regular icon -->
                <template v-else>
                  <ProviderIcon
                    :provider-id="displayProvider.id"
                    :custom-icon="displayProvider.custom_icon"
                    size="xl"
                  />
                </template>
                <button
                  v-if="displayProvider.type === 'custom'"
                  class="absolute inset-0 flex items-center justify-center bg-black/50 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                  :title="t('providerPool.changeIcon')"
                  @click="openIconUpload"
                >
                  <svg
                    class="w-4 h-4 text-white"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
                    />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"
                    />
                  </svg>
                </button>
                <input
                  ref="iconInput"
                  type="file"
                  accept="image/*"
                  class="hidden"
                  @change="handleIconUpload"
                />
              </div>
              <div class="cursor-pointer select-none" @click="showModelParams = !showModelParams">
                <div class="flex items-center gap-1.5">
                  <h2 class="font-bold text-gray-900 dark:text-white">
                    {{ getProviderName(displayProvider) }}
                  </h2>
                  <span
                    v-if="displayProvider.beta"
                    class="text-[10px] px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 tracking-wide"
                  >
                    {{ tr('providerPool.beta', 'Beta') }}
                  </span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ getProviderDescription(displayProvider) }}
                </p>
                <!-- Inline model params summary -->
                <div
                  class="flex items-center gap-2 mt-1 text-[10px] text-gray-400 dark:text-gray-500"
                >
                  <span
                    >{{ t('providerPool.temperature') }}:
                    {{
                      displayProvider!.model_params?.temperature ?? t('providerPool.default')
                    }}</span
                  >
                  <span>·</span>
                  <span>
                    {{ providerOutputLimitLabel }}:
                    {{
                      displayProvider!.model_params?.max_tokens ||
                      displayProvider!.model_params?.detected_max_tokens ||
                      t('providerPool.default')
                    }}
                  </span>
                  <svg
                    :class="['w-3 h-3 transition-transform', showModelParams ? 'rotate-180' : '']"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </div>
                <!-- Location Toggle -->
                <div class="flex items-center gap-1 mt-1" @click.stop>
                  <span class="text-xs text-gray-400">{{ t('providerPool.location') }}:</span>
                  <template v-if="canEditProviderLocation(displayProvider)">
                    <button
                      data-testid="provider-location-cloud-button"
                      :class="[
                        'px-1.5 py-0.5 rounded text-[10px] transition-colors',
                        getEffectiveProviderLocation(displayProvider) === 'cloud'
                          ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400'
                          : 'bg-gray-200 dark:bg-slate-600 text-gray-600 dark:text-gray-400 hover:bg-gray-300 dark:hover:bg-slate-500',
                      ]"
                      @click="updateProviderLocation(displayProvider!.id, 'cloud')"
                    >
                      ☁️ {{ t('providerPool.locationCloud') }}
                    </button>
                    <button
                      data-testid="provider-location-local-button"
                      :class="[
                        'px-1.5 py-0.5 rounded text-[10px] transition-colors',
                        getEffectiveProviderLocation(displayProvider) === 'local'
                          ? 'bg-green-500 text-white'
                          : 'bg-gray-200 dark:bg-slate-600 text-gray-600 dark:text-gray-400 hover:bg-gray-300 dark:hover:bg-slate-500',
                      ]"
                      @click="updateProviderLocation(displayProvider!.id, 'local')"
                    >
                      💻 {{ t('providerPool.locationLocal') }}
                    </button>
                  </template>
                  <span
                    v-else
                    :data-testid="
                      getEffectiveProviderLocation(displayProvider) === 'local'
                        ? 'provider-location-fixed-local'
                        : 'provider-location-fixed-cloud'
                    "
                    :class="[
                      'px-1.5 py-0.5 rounded text-[10px] transition-colors',
                      getEffectiveProviderLocation(displayProvider) === 'local'
                        ? 'bg-green-500 text-white'
                        : 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400',
                    ]"
                  >
                    {{
                      getEffectiveProviderLocation(displayProvider) === 'local'
                        ? `💻 ${t('providerPool.locationLocal')}`
                        : `☁️ ${t('providerPool.locationCloud')}`
                    }}
                  </span>
                </div>
              </div>
            </div>
            <div class="flex gap-1">
              <button
                v-if="
                  !displayProvider!.is_builtin &&
                  displayProvider!.type !== 'trial' &&
                  displayProvider!.type !== 'media'
                "
                :class="[
                  'px-2 py-1 bg-red-500 hover:bg-red-600 text-white rounded text-xs transition-opacity',
                  showDeleteFor === 'provider-' + displayProvider!.id
                    ? 'opacity-100'
                    : 'opacity-0 group-hover/header:opacity-100',
                ]"
                @click="deleteProvider(displayProvider!.id)"
              >
                {{ t('common.delete') }}
              </button>
            </div>
          </div>

          <!-- Collapsible Model Parameters (expanded by clicking provider title) -->
          <div
            v-if="showModelParams"
            class="mb-4 p-3 bg-gray-50 dark:bg-slate-900/30 rounded-lg border border-gray-100 dark:border-slate-700/50"
          >
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('providerPool.modelParams') }}
              </h3>
              <div class="flex gap-1">
                <button
                  v-if="displayProvider!.metadata_mode !== 'catalog'"
                  :disabled="
                    detectingCapabilities === displayProvider!.id || !displayProvider!.base_url
                  "
                  class="px-2 py-0.5 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-[10px] disabled:opacity-50"
                  :title="!displayProvider!.base_url ? t('providerPool.baseUrlRequired') : ''"
                  @click="detectCapabilities"
                >
                  {{
                    detectingCapabilities === displayProvider!.id
                      ? t('providerPool.detecting')
                      : t('providerPool.detectCapabilities')
                  }}
                </button>
                <button
                  class="px-2 py-0.5 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded text-[10px]"
                  @click="openParamsModal"
                >
                  {{ t('common.edit') }}
                </button>
              </div>
            </div>
            <div class="grid grid-cols-3 gap-2 text-xs">
              <div>
                <span class="text-gray-500 dark:text-gray-400"
                  >{{ t('providerPool.temperature') }}:</span
                >
                <span class="text-gray-900 dark:text-white provider-inline-start-1">
                  {{ displayProvider!.model_params?.temperature ?? t('providerPool.default') }}
                </span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400"
                  >{{ providerOutputLimitLabel }}:</span
                >
                <span
                  v-if="displayProvider!.model_params?.max_tokens"
                  class="text-gray-900 dark:text-white provider-inline-start-1"
                >
                  {{ displayProvider!.model_params.max_tokens }}
                </span>
                <span
                  v-else-if="displayProvider!.model_params?.detected_max_tokens"
                  class="text-gray-900 dark:text-gray-300 provider-inline-start-1"
                  :title="t('providerPool.detectedMax')"
                >
                  {{ displayProvider!.model_params.detected_max_tokens }}
                  <span class="text-gray-400 text-[10px]">({{ t('providerPool.detected') }})</span>
                </span>
                <span v-else class="text-gray-900 dark:text-white provider-inline-start-1">
                  {{ t('providerPool.default') }}
                </span>
              </div>
              <div v-if="displayProvider!.model_params?.top_p != null">
                <span class="text-gray-500 dark:text-gray-400">{{ t('providerPool.topP') }}:</span>
                <span class="text-gray-900 dark:text-white provider-inline-start-1">{{
                  displayProvider!.model_params.top_p
                }}</span>
              </div>
            </div>
          </div>

          <!-- Error Banner -->
          <div
            v-if="displayProvider!.status === 'error' && displayProvider!.last_error"
            class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"
          >
            <div class="flex items-start gap-2">
              <span class="text-red-500 flex-shrink-0">⚠</span>
              <p class="text-xs text-red-600 dark:text-red-400 break-all flex-1">
                {{ translateHealthError(displayProvider!.last_error!) }}
              </p>
              <button
                class="flex-shrink-0 px-2 py-1 text-xs bg-red-100 dark:bg-red-800 hover:bg-red-200 dark:hover:bg-red-700 text-red-600 dark:text-red-300 rounded transition-colors"
                @click="clearProviderError(displayProvider!.id)"
              >
                {{ t('providerPool.retry') }}
              </button>
            </div>
          </div>

          <!-- Base URL Section (for providers that need configuration) -->
          <div v-if="showBaseURLSection" data-testid="provider-base-url-section" class="mb-4">
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('providerPool.baseUrl') }}
              </h3>
              <button
                v-if="!editingBaseUrl"
                class="px-2 py-1 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded text-xs"
                @click="startEditBaseUrl(displayProvider!)"
              >
                {{ displayProvider!.base_url ? t('common.edit') : t('providerPool.configure') }}
              </button>
            </div>
            <div v-if="editingBaseUrl" class="flex gap-2">
              <input
                v-model="baseUrlDraft"
                type="url"
                :placeholder="
                  displayProvider!.id === 'azure-openai'
                    ? 'https://your-resource.openai.azure.com'
                    : displayProvider!.id === 'bedrock'
                      ? 'https://bedrock-runtime.us-east-1.amazonaws.com'
                      : 'https://api.example.com/v1'
                "
                class="flex-1 px-2 py-1 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded text-xs text-gray-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-gray-400"
              />
              <button
                class="px-2 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-xs"
                @click="saveBaseUrl(displayProvider!.id)"
              >
                {{ t('common.save') }}
              </button>
              <button
                class="px-2 py-1 bg-gray-500 hover:bg-gray-600 text-white rounded text-xs"
                @click="cancelEditBaseUrl()"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
            <div v-else class="text-xs">
              <span
                v-if="displayProvider!.base_url"
                class="text-gray-600 dark:text-gray-400 font-mono break-all"
              >
                {{ displayProvider!.base_url }}
              </span>
              <span v-else class="text-amber-500 dark:text-amber-400">
                {{ t('providerPool.baseUrlNotConfigured') }}
              </span>
            </div>
          </div>

          <!-- Custom Provider API Format -->
          <div v-if="displayProvider!.type === 'custom'" class="mb-4">
            <div class="flex items-start justify-between gap-2 mb-2">
              <div>
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('providerPool.apiFormatLabel') }}
                </h3>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('providerPool.apiFormatHint') }}
                </p>
              </div>
              <span
                v-if="savingCustomProviderFormat === displayProvider!.id"
                class="text-xs text-gray-500 dark:text-gray-400"
              >
                {{ t('common.saving') }}
              </span>
            </div>
            <select
              v-model="customProviderFormatDraft"
              data-testid="custom-provider-format-select"
              :disabled="savingCustomProviderFormat === displayProvider!.id"
              class="w-full px-3 py-2 bg-white dark:bg-slate-800 border border-gray-200 dark:border-slate-600 rounded-lg text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400 disabled:opacity-50"
              @change="saveCustomProviderFormat(customProviderFormatDraft)"
            >
              <option
                v-for="option in editableCustomProviderFormatOptions"
                :key="option.value"
                :value="option.value"
              >
                {{ t(`providerPool.apiFormatOptions.${option.value}`) }}
              </option>
            </select>
            <p
              v-if="customProviderFormatDraft === 'auto' && displayProvider!.api_format"
              class="mt-2 text-xs text-gray-500 dark:text-gray-400"
            >
              {{ t('providerPool.apiFormatAutoDetected', { format: displayProvider!.api_format }) }}
            </p>
          </div>

          <!-- Provider Verification & Recommendation -->
          <div
            v-if="supportsProviderVerification"
            class="mb-4 p-3 bg-gray-50 dark:bg-slate-900/30 rounded-lg border border-gray-100 dark:border-slate-700/50"
          >
            <div class="flex items-start justify-between gap-2 mb-2">
              <div>
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('providerPool.verifySection') }}
                </h3>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('providerPool.verifyHint') }}
                </p>
              </div>
              <div class="flex gap-1">
                <button
                  :disabled="
                    verifyingProvider === displayProvider!.id ||
                    applyingVerification === displayProvider!.id ||
                    !displayProvider!.base_url
                  "
                  class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs disabled:opacity-50"
                  @click="runProviderVerification(false)"
                >
                  {{
                    verifyingProvider === displayProvider!.id
                      ? t('providerPool.verifyRunning')
                      : t('providerPool.verifyRun')
                  }}
                </button>
                <button
                  :disabled="
                    verifyingProvider === displayProvider!.id ||
                    applyingVerification === displayProvider!.id ||
                    !displayProvider!.base_url
                  "
                  class="px-2 py-1 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded text-xs disabled:opacity-50"
                  @click="runProviderVerification(true)"
                >
                  {{
                    applyingVerification === displayProvider!.id
                      ? t('providerPool.verifyApplying')
                      : t('providerPool.verifyApply')
                  }}
                </button>
              </div>
            </div>

            <div v-if="displayProvider!.api_keys?.length" class="mb-2">
              <label class="block text-[10px] text-gray-500 dark:text-gray-400 mb-1">{{
                t('providerPool.verifyKeyLabel')
              }}</label>
              <select
                v-model="verificationKeyId"
                class="w-full px-2 py-1 bg-white dark:bg-slate-800 border border-gray-200 dark:border-slate-600 rounded text-xs text-gray-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-gray-400"
              >
                <option value="">{{ t('providerPool.verifyKeyAuto') }}</option>
                <option v-for="key in displayProvider!.api_keys" :key="key.id" :value="key.id">
                  {{ key.key_hash }}{{ key.label ? ` (${formatKeyLabel(key.label)})` : '' }}
                </option>
              </select>
            </div>

            <div
              v-if="verificationError"
              class="text-xs text-red-600 dark:text-red-400 break-all mb-2"
            >
              {{ verificationError }}
            </div>

            <div v-if="verificationResult" class="space-y-2">
              <div class="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs">
                <div
                  class="p-2 rounded bg-white dark:bg-slate-800 border border-gray-200 dark:border-slate-700"
                >
                  <div class="text-gray-500 dark:text-gray-400">
                    {{ t('providerPool.verifyDetectedFormat') }}
                  </div>
                  <div class="font-mono text-gray-900 dark:text-white">
                    {{ verificationResult.detected_format || '-' }}
                  </div>
                </div>
                <div
                  class="p-2 rounded bg-white dark:bg-slate-800 border border-gray-200 dark:border-slate-700"
                >
                  <div class="text-gray-500 dark:text-gray-400">
                    {{ t('providerPool.verifyRecommendedFormat') }}
                  </div>
                  <div class="font-mono text-gray-900 dark:text-white">
                    {{ verificationResult.recommended_api_format || '-' }}
                  </div>
                </div>
              </div>

              <div class="text-xs">
                <div class="text-gray-500 dark:text-gray-400">
                  {{ t('providerPool.verifyRecommendedBaseURL') }}
                </div>
                <div class="font-mono text-gray-900 dark:text-white break-all">
                  {{ verificationResult.recommended_base_url || '-' }}
                </div>
              </div>

              <div
                v-if="verificationResult.chat_error"
                class="text-xs text-amber-600 dark:text-amber-400 break-all"
              >
                {{ t('providerPool.verifyChatError') }}: {{ verificationResult.chat_error }}
              </div>

              <div>
                <div
                  class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-1"
                >
                  {{ t('providerPool.verifyProbes') }}
                </div>
                <div class="space-y-1">
                  <div
                    v-for="[probeKey, probe] in verificationProbeEntries"
                    :key="probeKey"
                    class="p-2 rounded bg-white dark:bg-slate-800 border border-gray-200 dark:border-slate-700 text-xs"
                  >
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium text-gray-900 dark:text-white">{{
                        getVerificationProbeLabel(probeKey)
                      }}</span>
                      <span
                        class="font-mono text-gray-500 dark:text-gray-400"
                        v-if="probe.status_code"
                        >HTTP {{ probe.status_code }}</span
                      >
                    </div>
                    <div class="font-mono text-[10px] text-gray-500 dark:text-gray-400 break-all">
                      {{ probe.url }}
                    </div>
                    <div
                      class="mt-1"
                      :class="
                        probe.reachable
                          ? 'text-green-600 dark:text-green-400'
                          : 'text-red-600 dark:text-red-400'
                      "
                    >
                      {{
                        probe.reachable
                          ? t('providerPool.verifyProbeReachable')
                          : t('providerPool.verifyProbeUnreachable')
                      }}
                    </div>
                    <div v-if="probe.error" class="text-red-600 dark:text-red-400 break-all">
                      {{ probe.error }}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- OAuth Section (for OAuth-capable providers) — shown before API Keys -->
          <div v-if="displayProvider?.oauth" class="mb-4">
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('providerPool.oauth.title') }}
              </h3>
            </div>
            <div class="space-y-2">
              <!-- Account list -->
              <div
                v-for="account in oauthAccounts[displayProvider.id] || []"
                :key="account.id"
                class="p-3 bg-white dark:bg-slate-900/50 rounded-lg border border-gray-200 dark:border-gray-700"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="w-2 h-2 rounded-full bg-green-500"></span>
                    <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                      account.email || account.provider_type
                    }}</span>
                    <span v-if="account.project_id" class="text-xs text-gray-500"
                      >({{ account.project_id }})</span
                    >
                  </div>
                  <button
                    :disabled="disconnectingAccountId === account.id"
                    class="px-2 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-xs disabled:opacity-50"
                    @click="disconnectOAuth(displayProvider.id, account.id)"
                  >
                    {{
                      disconnectingAccountId === account.id
                        ? '...'
                        : t('providerPool.oauth.disconnect')
                    }}
                  </button>
                </div>
              </div>
              <!-- Connect another account (always shown at bottom) -->
              <div
                class="p-3 bg-white dark:bg-slate-900/50 rounded-lg border border-dashed border-gray-300 dark:border-gray-600 flex items-center justify-between"
              >
                <span class="text-sm text-gray-500 dark:text-gray-400">
                  {{
                    (oauthAccounts[displayProvider.id] || []).length
                      ? t('providerPool.oauth.addAccount')
                      : t('providerPool.oauth.notConnected')
                  }}
                </span>
                <button
                  :disabled="connectingOAuth === displayProvider.id"
                  class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded text-xs disabled:opacity-50"
                  @click="startOAuthConnect(displayProvider.id)"
                >
                  {{
                    connectingOAuth === displayProvider.id ? '...' : t('providerPool.oauth.connect')
                  }}
                </button>
              </div>
              <!-- Subscription Tier & Quota (shown once for the provider) -->
              <div
                v-if="store.loadingQuota === displayProvider.id"
                class="text-xs text-gray-400 px-1"
              >
                {{ t('providerPool.oauth.loadingQuota') }}
              </div>
              <template v-else-if="store.oauthQuota[displayProvider.id]">
                <div v-if="store.oauthQuota[displayProvider.id].tier" class="px-1">
                  <span
                    class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                    :class="tierBadgeClass(store.oauthQuota[displayProvider.id].tier)"
                  >
                    {{
                      store.oauthQuota[displayProvider.id].tier_name ||
                      store.oauthQuota[displayProvider.id].tier
                    }}
                  </span>
                </div>
                <div
                  v-if="
                    store.oauthQuota[displayProvider.id].error &&
                    !store.oauthQuota[displayProvider.id].tier
                  "
                  class="text-xs text-gray-400 px-1"
                >
                  {{ t('providerPool.oauth.quotaUnavailable') }}
                </div>
                <!-- Per-Model Quota Bars -->
                <div
                  v-if="store.oauthQuota[displayProvider.id].model_quotas?.length"
                  class="space-y-1 px-1"
                >
                  <div class="text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                    {{ t('providerPool.oauth.modelQuota') }}
                  </div>
                  <div
                    v-for="mq in store.oauthQuota[displayProvider.id].model_quotas"
                    :key="mq.model"
                    class="flex items-center gap-2 text-xs"
                  >
                    <span
                      class="text-gray-600 dark:text-gray-400 w-36 truncate"
                      :title="mq.model"
                      >{{ mq.model }}</span
                    >
                    <div
                      class="flex-1 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden"
                    >
                      <div
                        class="h-full rounded-full transition-all"
                        :class="
                          mq.remaining_percent > 20
                            ? 'bg-green-500'
                            : mq.remaining_percent > 5
                              ? 'bg-yellow-500'
                              : 'bg-red-500'
                        "
                        :style="{ width: mq.remaining_percent + '%' }"
                      />
                    </div>
                    <span class="provider-text-end text-gray-500 w-8"
                      >{{ mq.remaining_percent }}%</span
                    >
                  </div>
                </div>
              </template>
            </div>
          </div>

          <!-- API Keys Section — hidden for pure-OAuth providers with no keys -->
          <div
            v-if="
              !displayProvider?.oauth ||
              displayProvider!.api_keys?.length ||
              displayProvider!.type === 'trial'
            "
            class="mb-4"
          >
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('providerPool.apiKeys') }}
              </h3>
              <div class="flex items-center gap-2">
                <a
                  v-if="displayProvider!.api_key_url"
                  :href="displayProvider!.api_key_url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="px-2 py-1 bg-green-600 hover:bg-green-700 text-white rounded text-xs flex items-center gap-1"
                >
                  {{ t('providerPool.getApiKey') }}
                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                    />
                  </svg>
                </a>
                <button
                  v-if="displayProvider!.type !== 'trial'"
                  class="px-2 py-1 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded text-xs"
                  @click="openKeyModal(displayProvider!.id)"
                >
                  + {{ t('providerPool.addKey') }}
                </button>
              </div>
            </div>
            <div class="space-y-1">
              <!-- Trial provider: show placeholder -->
              <div
                v-if="displayProvider!.type === 'trial'"
                class="flex items-center justify-between p-2 bg-white dark:bg-slate-900/50 rounded text-xs"
              >
                <div>
                  <span class="text-gray-700 dark:text-white font-mono">••••••••••••••••</span>
                  <span class="provider-inline-start-2 text-gray-500"
                    >({{ t('providerPool.trial.name') }})</span
                  >
                </div>
              </div>
              <!-- Non-trial provider: show actual keys -->
              <template v-else>
                <div
                  v-for="key in displayProvider!.api_keys"
                  :key="key.id"
                  class="group/key flex items-center justify-between p-2 rounded text-xs bg-white dark:bg-slate-900/50 hover:bg-gray-50 dark:hover:bg-slate-800/50 transition-colors"
                  @touchstart="startLongPress('key-' + key.id)"
                  @touchend="cancelLongPress()"
                  @touchcancel="cancelLongPress()"
                >
                  <div class="flex items-center gap-1.5 min-w-0">
                    <button
                      :class="[
                        'text-red-400 hover:text-red-300 transition-opacity flex-shrink-0',
                        showDeleteFor === 'key-' + key.id
                          ? 'opacity-100'
                          : 'opacity-0 group-hover/key:opacity-100',
                      ]"
                      @click.stop="removeAPIKey(displayProvider!.id, key.id)"
                    >
                      <svg
                        class="w-3.5 h-3.5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                        />
                      </svg>
                    </button>
                    <span class="text-gray-700 dark:text-white font-mono truncate">{{
                      key.key_hash
                    }}</span>
                    <span v-if="key.label" class="text-gray-500 truncate"
                      >({{ formatKeyLabel(key.label) }})</span
                    >
                  </div>
                  <div class="flex items-center gap-2 flex-shrink-0">
                    <span v-if="key.usage_count" class="text-gray-500">
                      {{ key.usage_count }}
                    </span>
                    <!-- Test result indicator -->
                    <span v-if="testingKeyId === key.id" class="text-gray-400">...</span>
                    <span
                      v-else-if="keyTestResults[key.id]?.healthy === true"
                      class="text-green-500"
                      title="Healthy"
                      >✓</span
                    >
                    <span
                      v-else-if="keyTestResults[key.id]?.healthy === false"
                      class="text-red-400"
                      :title="keyTestResults[key.id]?.error || 'Failed'"
                      >✗</span
                    >
                  </div>
                </div>
                <div
                  v-if="!displayProvider!.api_keys?.length"
                  class="text-gray-500 dark:text-gray-400 text-center py-2 text-xs"
                >
                  {{ t('providerPool.noKeys') }}
                </div>
              </template>
            </div>
          </div>

          <!-- Device Flow Modal -->
          <Teleport to="body">
            <div
              v-if="deviceFlowState"
              class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
            >
              <div class="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full mx-4">
                <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
                  {{ t('providerPool.oauth.deviceFlow') }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-gray-400 mb-3">
                  {{ t('providerPool.oauth.deviceFlowInstructions') }}
                </p>
                <div class="flex items-center justify-center gap-2 mb-4">
                  <input
                    :value="deviceFlowState.userCode"
                    readonly
                    class="w-full max-w-xs text-center text-2xl font-bold tracking-widest text-gray-900 dark:text-white bg-gray-100 dark:bg-gray-700 px-4 py-2 rounded border border-gray-200 dark:border-gray-600 select-all"
                    @focus="($event.target as HTMLInputElement).select()"
                    @click="($event.target as HTMLInputElement).select()"
                  />
                  <button
                    class="px-3 py-2 text-sm bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded"
                    @click="copyDeviceFlowCode"
                  >
                    {{ t('common.copy') }}
                  </button>
                </div>
                <a
                  :href="deviceFlowState.verificationUri"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="block w-full text-center px-4 py-2 bg-gray-900 dark:bg-gray-600 hover:bg-gray-800 dark:hover:bg-gray-500 text-white rounded mb-3"
                >
                  {{ t('providerPool.oauth.openGitHub') }}
                </a>
                <div class="flex gap-2">
                  <button
                    class="flex-1 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm disabled:opacity-50"
                    :disabled="connectingOAuth === deviceFlowState.providerId"
                    @click="completeDeviceFlow"
                  >
                    {{ connectingOAuth ? '...' : t('providerPool.oauth.done') }}
                  </button>
                  <button
                    class="px-4 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded text-sm"
                    @click="deviceFlowState = null"
                  >
                    {{ t('common.cancel') }}
                  </button>
                </div>
              </div>
            </div>
          </Teleport>

          <!-- Trial Quota Section (only for trial providers) -->
          <div
            v-if="displayProvider!.type === 'trial' && store.trialQuota"
            class="mb-4 p-3 bg-gray-100 dark:bg-gray-700/30 rounded-lg border border-gray-200 dark:border-gray-600"
          >
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-300 mb-2">
              {{ t('providerPool.trial.name') }}
            </h3>
            <div class="space-y-2">
              <div class="flex items-center justify-between text-xs">
                <span
                  class="text-gray-700 dark:text-gray-400"
                  :title="
                    (store.trialQuota.token_limit - store.trialQuota.tokens_used).toLocaleString() +
                    ' / ' +
                    store.trialQuota.token_limit.toLocaleString()
                  "
                  >{{
                    t('providerPool.trial.tokensUsed', {
                      remaining: formatTokens(
                        store.trialQuota.token_limit - store.trialQuota.tokens_used
                      ),
                      total: formatTokens(store.trialQuota.token_limit),
                    })
                  }}</span
                >
                <div
                  class="relative w-24 h-4 bg-gray-300 dark:bg-gray-600 rounded-full overflow-hidden"
                >
                  <div
                    class="h-full bg-gray-400 dark:bg-gray-500 transition-all"
                    :style="{
                      width: `${Math.max(3, Math.min(100, ((store.trialQuota.token_limit - store.trialQuota.tokens_used) / store.trialQuota.token_limit) * 100))}%`,
                    }"
                  />
                  <span
                    class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-900 dark:text-gray-700 drop-shadow-sm"
                  >
                    {{
                      Math.round(
                        ((store.trialQuota.token_limit - store.trialQuota.tokens_used) /
                          store.trialQuota.token_limit) *
                          100
                      )
                    }}%
                  </span>
                </div>
              </div>
              <p
                v-if="store.trialQuota.is_exhausted"
                class="text-xs text-red-600 dark:text-red-400 mt-2"
              >
                {{
                  store.trialQuota.exhausted_reason === 'token_limit'
                    ? t('providerPool.trial.quotaExhaustedTokens')
                    : t('providerPool.trial.quotaExhaustedConversations')
                }}
              </p>
            </div>
          </div>

          <!-- Usage Section -->
          <div v-if="providerUsage && !loadingUsage" class="mb-4">
            <div class="mb-2 flex items-center justify-between gap-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('providerPool.usage.title') }}
              </h3>
              <router-link
                to="/billing"
                class="inline-flex items-center rounded-md border border-gray-300 dark:border-gray-600 bg-white/80 dark:bg-slate-800/70 px-2.5 py-1 text-xs font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-slate-700 transition-colors"
              >
                {{ t('nav.billing') }}
              </router-link>
            </div>
            <div class="grid grid-cols-3 gap-2">
              <div
                class="p-2 bg-white dark:bg-slate-900/50 rounded-lg border border-gray-200 dark:border-gray-700 text-center"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('providerPool.usage.requests') }}
                </div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ providerUsage.total_requests.toLocaleString() }}
                </div>
              </div>
              <div
                class="p-2 bg-white dark:bg-slate-900/50 rounded-lg border border-gray-200 dark:border-gray-700 text-center"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('providerPool.usage.inputTokens') }}
                </div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ formatTokens(providerUsage.total_input_tokens) }}
                </div>
              </div>
              <div
                class="p-2 bg-white dark:bg-slate-900/50 rounded-lg border border-gray-200 dark:border-gray-700 text-center"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('providerPool.usage.outputTokens') }}
                </div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ formatTokens(providerUsage.total_output_tokens) }}
                </div>
              </div>
            </div>
            <div
              v-if="providerUsage.total_estimated_cost > 0"
              class="provider-text-end mt-2 text-xs text-gray-500 dark:text-gray-400"
            >
              {{ t('providerPool.usage.estimatedCost') }}: ${{
                providerUsage.total_estimated_cost.toFixed(4)
              }}
            </div>
          </div>
          <div v-else-if="loadingUsage" class="mb-4 text-center text-xs text-gray-400 py-2">
            {{ t('providerPool.usage.loading') }}
          </div>

          <div
            v-if="shouldShowProviderAccountStatus"
            data-testid="provider-account-status-section"
            class="mb-4 p-3 bg-gray-50 dark:bg-slate-900/30 rounded-lg border border-gray-100 dark:border-slate-700/50"
          >
            <div class="flex items-center justify-between gap-2 mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ tr('providerPool.accountStatus.title', 'Account Status') }}
              </h3>
              <span v-if="providerAccountStatus?.key_hash" class="text-[10px] text-gray-500">
                {{ getAccountStatusKeyLabel(providerAccountStatus.key_hash) }}
              </span>
            </div>
            <div v-if="store.loadingAccountStatus === displayProvider!.id" class="text-xs text-gray-400">
              {{ tr('providerPool.accountStatus.loading', 'Loading account status...') }}
            </div>
            <div v-else-if="providerAccountStatus?.items?.length" class="space-y-3">
              <div
                v-if="providerAccountStatusPrimaryItem"
                class="p-3 bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ getAccountStatusItemLabel(providerAccountStatusPrimaryItem.key) }}
                </div>
                <div class="text-lg font-semibold text-gray-900 dark:text-white mt-1">
                  {{ formatAccountStatusAmount(providerAccountStatusPrimaryItem) }}
                </div>
              </div>
              <div v-if="providerAccountStatusSecondaryItems.length" class="grid grid-cols-2 gap-2">
                <div
                  v-for="item in providerAccountStatusSecondaryItems"
                  :key="item.key"
                  class="p-2 bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700"
                >
                  <div class="text-[11px] text-gray-500 dark:text-gray-400">
                    {{ getAccountStatusItemLabel(item.key) }}
                  </div>
                  <div class="text-sm font-medium text-gray-900 dark:text-white mt-1">
                    {{ formatAccountStatusAmount(item) }}
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="text-xs text-gray-400">
              {{ tr('providerPool.accountStatus.unavailable', 'Account status unavailable') }}
            </div>
          </div>

          <!-- Models Section — shows models for selected key or provider-level fallback -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('providerPool.models') }}
                <span class="text-gray-500 text-xs provider-inline-start-1"
                  >({{
                    (selectedKeyId && displayProvider!.type !== 'trial'
                      ? selectedKeyModels
                      : selectedProviderModels
                    ).length
                  }})</span
                >
                <span
                  v-if="displayProvider?.allowed_models?.length"
                  class="text-gray-900 dark:text-gray-300 text-xs provider-inline-start-1"
                  :title="t('providerPool.filteredModels')"
                >
                  ({{ t('providerPool.filtered') }})
                </span>
              </h3>
              <button
                v-if="displayProvider!.type !== 'media'"
                class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs"
                @click="openAllowedModelsModal"
              >
                {{ t('providerPool.configureModels') }}
              </button>
            </div>

            <!-- Loading state for selected key -->
            <div
              v-if="selectedKeyId && fetchingKeyModels[selectedKeyId]"
              class="text-gray-500 dark:text-gray-400 text-center py-4 text-xs"
            >
              {{ t('providerPool.fetchingKeyModels') }}
            </div>

            <!-- Model list -->
            <div v-else class="space-y-1 max-h-64 overflow-y-auto">
              <div
                v-for="model in selectedKeyId && displayProvider!.type !== 'trial'
                  ? selectedKeyModels
                  : selectedProviderModels"
                :key="model.id"
                class="p-2 bg-white dark:bg-slate-900/50 rounded text-xs group"
                @mouseenter="model.capabilities?.length && showCapTip($event, model.capabilities)"
                @mouseleave="hideCapTip"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center flex-1 min-w-0 gap-1.5">
                    <span class="text-gray-400 cursor-grab select-none" title="Drag to reorder"
                      >⠿</span
                    >
                    <span class="text-gray-900 dark:text-white font-medium">{{
                      getLocalizedProviderModelName(model)
                    }}</span>
                    <span
                      v-if="model.capabilities?.length"
                      class="text-[10px] leading-none text-gray-400 provider-inline-start-0_5"
                      >✦</span
                    >
                    <span class="text-gray-500 provider-inline-start-1 truncate">{{
                      model.id
                    }}</span>
                  </div>
                  <div class="flex items-center gap-2 provider-inline-start-2">
                    <span v-if="model.context_window" class="text-gray-500 whitespace-nowrap">
                      {{ (model.context_window / 1000).toFixed(0) }}K
                    </span>
                    <span
                      :class="
                        hasCustomPricing(model.id)
                          ? 'text-gray-900 dark:text-gray-300'
                          : 'text-green-500'
                      "
                      class="whitespace-nowrap cursor-pointer hover:underline"
                      :title="
                        hasCustomPricing(model.id)
                          ? t('providerPool.customPricing')
                          : t('providerPool.defaultPricing')
                      "
                      @click.stop="openPricingModal(model)"
                    >
                      <template v-if="isMediaModel(model)">
                        {{ mediaPricingLabel(model) }}
                      </template>
                      <template v-else>
                        ${{
                          (
                            getModelCustomPricing(model.id)?.input_price ??
                            model.input_price ??
                            0
                          ).toFixed(2)
                        }}/${{
                          (
                            getModelCustomPricing(model.id)?.output_price ??
                            model.output_price ??
                            0
                          ).toFixed(2)
                        }}
                      </template>
                    </span>
                    <button
                      class="opacity-0 group-hover:opacity-100 p-1 hover:bg-gray-100 dark:hover:bg-slate-700 rounded transition-opacity"
                      :title="t('providerPool.editPricing')"
                      @click.stop="openPricingModal(model)"
                    >
                      <svg
                        class="w-3 h-3 text-gray-500"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
                        />
                      </svg>
                    </button>
                    <button
                      v-if="hasCustomPricing(model.id)"
                      class="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-100 dark:hover:bg-red-900/30 rounded transition-opacity"
                      :title="t('providerPool.resetPricing')"
                      @click.stop="
                        removeCustomPricing(model.id, store.selectedProviderId ?? undefined)
                      "
                    >
                      <svg
                        class="w-3 h-3 text-red-500"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>
              <div
                v-if="
                  (selectedKeyId && displayProvider!.type !== 'trial'
                    ? selectedKeyModels
                    : selectedProviderModels
                  ).length === 0
                "
                class="text-gray-500 dark:text-gray-400 text-center py-2 text-xs"
              >
                {{ t('providerPool.noModels') }}
              </div>
            </div>
            <p class="text-xs text-gray-400 mt-2">{{ t('providerPool.pricingHint') }}</p>
          </div>
        </div>

        <div
          v-else
          class="bg-white dark:bg-slate-800/30 rounded-lg border border-gray-200 dark:border-slate-700 p-8 text-center"
        >
          <p class="text-gray-500 dark:text-gray-400 text-sm">
            {{ t('providerPool.selectProvider') }}
          </p>
        </div>
      </div>
    </div>

    <!-- IDE Discovery Modal -->
    <Teleport to="body">
      <div
        v-if="showIDEDiscoveryModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      >
        <div
          class="bg-white dark:bg-slate-800 rounded-lg w-full max-w-4xl mx-4 max-h-[90vh] flex flex-col"
        >
          <div
            class="flex items-center justify-between p-5 border-b border-gray-200 dark:border-slate-700"
          >
            <h2 class="text-lg font-bold text-gray-900 dark:text-white">
              {{ t('providerPool.scanIDE') }}
            </h2>
            <button
              class="px-2 py-1 bg-gray-200 dark:bg-slate-700 hover:bg-gray-300 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-200 rounded text-sm"
              @click="showIDEDiscoveryModal = false"
            >
              {{ t('common.cancel') }}
            </button>
          </div>
          <div class="flex-1 overflow-y-auto">
            <IDEDiscovery @import-success="handleIDEImportSuccess" />
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Add Provider Modal -->
    <Teleport to="body">
      <div
        v-if="showAddModal"
        class="fixed inset-0 z-50 overflow-y-auto bg-black/50 px-4 py-4 sm:py-6"
      >
        <div class="flex min-h-full items-start justify-center sm:items-center">
          <div
            :class="[
              'flex max-h-[calc(100vh-2rem)] w-full flex-col overflow-hidden rounded-lg bg-white shadow-xl dark:bg-slate-800 sm:max-h-[90vh]',
              addProviderMode === 'chooser' ? 'max-w-5xl' : 'max-w-2xl',
            ]"
          >
            <template v-if="addProviderMode === 'chooser'">
              <div
                class="flex items-start justify-between gap-4 border-b border-gray-200 p-5 dark:border-slate-700"
              >
                <div>
                  <h2 class="text-lg font-bold text-gray-900 dark:text-white">
                    {{ tr('providerPool.addProvider', t('providerPool.addCustomProvider')) }}
                  </h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                    {{
                      tr(
                        'providerPool.addProviderHint',
                        'Select an official provider or add a custom compatible endpoint.'
                      )
                    }}
                  </p>
                </div>
                <button
                  class="px-3 py-1.5 bg-gray-200 dark:bg-slate-700 hover:bg-gray-300 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-200 rounded-lg text-sm"
                  @click="closeAddProviderModal"
                >
                  {{ t('common.cancel') }}
                </button>
              </div>

              <div class="flex-1 overflow-y-auto p-5">
                <div class="space-y-3">
                  <div>
                    <h3 class="mb-2 text-sm font-medium text-gray-900 dark:text-white">
                      {{ tr('providerPool.officialProvider', 'Official Provider') }}
                    </h3>
                    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                      <button
                        v-for="provider in officialProviderOptions"
                        :key="provider.id"
                        type="button"
                        class="text-left p-3 rounded-lg border border-gray-200 dark:border-slate-600 bg-gray-50 dark:bg-slate-900/30 hover:border-gray-300 dark:hover:border-slate-500 transition-colors"
                        @click="selectOfficialProviderOption(provider)"
                      >
                        <div class="flex items-start gap-3">
                          <ProviderIcon
                            :provider-id="provider.id"
                            :custom-icon="provider.custom_icon"
                            size="lg"
                          />
                          <div class="min-w-0 flex-1">
                            <div class="flex items-center gap-2">
                              <div class="font-medium text-gray-900 dark:text-white truncate">
                                {{ getProviderName(provider) }}
                              </div>
                              <span
                                v-if="provider.enabled"
                                class="text-[10px] px-1.5 py-0.5 rounded bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400"
                              >
                                {{ tr('providerPool.configured', 'Configured') }}
                              </span>
                            </div>
                            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400 truncate">
                              {{
                                getProviderDescription(provider) ||
                                provider.api_format ||
                                provider.location
                              }}
                            </div>
                            <div class="mt-1 text-[11px] text-gray-400 dark:text-gray-500">
                              {{ provider.api_format || 'openai' }} · {{ provider.location }}
                            </div>
                          </div>
                        </div>
                      </button>
                    </div>
                  </div>

                  <div class="border-t border-gray-200 pt-3 dark:border-slate-700">
                    <button
                      type="button"
                      class="w-full text-left p-3 rounded-lg border border-dashed border-gray-300 dark:border-slate-600 hover:border-gray-400 dark:hover:border-slate-500 transition-colors"
                      @click="openCustomProviderForm"
                    >
                      <div class="font-medium text-gray-900 dark:text-white">
                        {{
                          tr(
                            'providerPool.customCompatibleProvider',
                            'Custom Compatible Provider'
                          )
                        }}
                      </div>
                      <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                        {{
                          tr(
                            'providerPool.customCompatibleProviderHint',
                            'Bring your own compatible endpoint and keep dynamic verify/probe support.'
                          )
                        }}
                      </div>
                    </button>
                  </div>
                </div>
              </div>
            </template>

            <template v-else>
              <div
                class="flex items-start justify-between gap-4 border-b border-gray-200 p-5 dark:border-slate-700"
              >
                <div class="flex min-w-0 items-start gap-2">
                  <button
                    type="button"
                    class="px-2 py-1 bg-gray-200 dark:bg-slate-700 hover:bg-gray-300 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-200 rounded text-sm"
                    @click="addProviderMode = 'chooser'"
                  >
                    ←
                  </button>
                  <div class="min-w-0">
                    <h2 class="text-lg font-bold text-gray-900 dark:text-white">
                      {{ t('providerPool.addCustomProvider') }}
                    </h2>
                    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                      {{
                        tr(
                          'providerPool.customCompatibleProviderHint',
                          'Bring your own compatible endpoint and keep dynamic verify/probe support.'
                        )
                      }}
                    </p>
                  </div>
                </div>
                <button
                  class="px-3 py-1.5 bg-gray-200 dark:bg-slate-700 hover:bg-gray-300 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-200 rounded-lg text-sm"
                  @click="closeAddProviderModal"
                >
                  {{ t('common.cancel') }}
                </button>
              </div>

              <form class="flex min-h-0 flex-1 flex-col" @submit.prevent="addCustomProvider">
                <div class="flex-1 overflow-y-auto p-5">
                  <div class="space-y-4">
                    <div>
                      <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                        t('providerPool.providerName')
                      }}</label>
                      <input
                        v-model="newProvider.name"
                        type="text"
                        required
                        :disabled="addingProvider"
                        class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400 disabled:opacity-50"
                      />
                    </div>
                    <div>
                      <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                        t('providerPool.baseUrl')
                      }}</label>
                      <input
                        v-model="newProvider.base_url"
                        type="url"
                        required
                        :disabled="addingProvider"
                        placeholder="https://api.example.com/v1"
                        class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400 disabled:opacity-50"
                      />
                    </div>
                    <div>
                      <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                        t('providerPool.apiFormatLabel')
                      }}</label>
                      <select
                        v-model="newProvider.format"
                        data-testid="new-provider-format-select"
                        :disabled="addingProvider"
                        class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400 disabled:opacity-50"
                      >
                        <option
                          v-for="option in editableCustomProviderFormatOptions"
                          :key="option.value"
                          :value="option.value"
                        >
                          {{ t(`providerPool.apiFormatOptions.${option.value}`) }}
                        </option>
                      </select>
                      <p class="mt-1 text-xs text-gray-400">
                        {{ t('providerPool.apiFormatHint') }}
                      </p>
                    </div>
                    <div>
                      <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                        t('providerPool.apiKeyOptional')
                      }}</label>
                      <div class="relative">
                        <input
                          v-model.trim="newProvider.api_key"
                          :type="showNewProviderApiKey ? 'text' : 'password'"
                          :disabled="addingProvider"
                          placeholder="sk-..."
                          class="provider-input-with-action w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400 disabled:opacity-50"
                        />
                        <button
                          type="button"
                          :disabled="addingProvider"
                          class="provider-input-action absolute inset-y-0 px-3 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 disabled:opacity-50"
                          :title="showNewProviderApiKey ? 'Hide API Key' : 'Show API Key'"
                          @click="showNewProviderApiKey = !showNewProviderApiKey"
                        >
                          <svg
                            v-if="showNewProviderApiKey"
                            xmlns="http://www.w3.org/2000/svg"
                            class="h-4 w-4"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.542-7a10.05 10.05 0 012.132-3.368m3.1-2.329A9.96 9.96 0 0112 5c4.478 0 8.268 2.943 9.542 7a10.036 10.036 0 01-4.293 5.232M15 12a3 3 0 11-4.243-4.243m0 0L3 3m7.757 4.757L21 21"
                            />
                          </svg>
                          <svg
                            v-else
                            xmlns="http://www.w3.org/2000/svg"
                            class="h-4 w-4"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                            />
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M2.458 12C3.732 7.943 7.522 5 12 5s8.268 2.943 9.542 7c-1.274 4.057-5.064 7-9.542 7s-8.268-2.943-9.542-7z"
                            />
                          </svg>
                        </button>
                      </div>
                      <p class="mt-1 text-xs text-gray-400">{{ t('providerPool.apiKeyHint') }}</p>
                    </div>
                    <div>
                      <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                        t('providerPool.location')
                      }}</label>
                      <div class="flex gap-2">
                        <button
                          type="button"
                          :disabled="addingProvider"
                          :class="[
                            'flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg border transition-colors',
                            newProvider.location === 'cloud'
                              ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-400'
                              : 'bg-gray-100 dark:bg-slate-700 border-gray-200 dark:border-slate-600 text-gray-600 dark:text-gray-400 hover:border-gray-300 dark:hover:border-slate-500',
                          ]"
                          @click="newProvider.location = 'cloud'"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"
                            />
                          </svg>
                          {{ t('providerPool.locationCloud') }}
                        </button>
                        <button
                          type="button"
                          :disabled="addingProvider"
                          :class="[
                            'flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg border transition-colors',
                            newProvider.location === 'local'
                              ? 'bg-green-500/20 border-green-500 text-green-500'
                              : 'bg-gray-100 dark:bg-slate-700 border-gray-200 dark:border-slate-600 text-gray-600 dark:text-gray-400 hover:border-gray-300 dark:hover:border-slate-500',
                          ]"
                          @click="newProvider.location = 'local'"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                            />
                          </svg>
                          {{ t('providerPool.locationLocal') }}
                        </button>
                      </div>
                      <p class="mt-1 text-xs text-gray-400">{{ t('providerPool.locationHint') }}</p>
                    </div>

                    <div
                      v-if="addingProvider"
                      class="flex items-center gap-2 rounded-lg bg-gray-100 p-3 dark:bg-slate-700"
                    >
                      <div
                        class="animate-spin w-4 h-4 border-2 border-gray-900 dark:border-gray-300 border-t-transparent rounded-full flex-shrink-0"
                      ></div>
                      <span class="text-sm text-gray-600 dark:text-gray-300">
                        {{
                          addingStep === 'adding'
                            ? t('providerPool.addingProvider')
                            : addingStep === 'probing'
                              ? t('providerPool.probingModels')
                              : ''
                        }}
                      </span>
                    </div>
                  </div>
                </div>

                <div
                  class="flex justify-end gap-3 border-t border-gray-200 px-5 py-4 dark:border-slate-700"
                >
                  <button
                    type="button"
                    :disabled="addingProvider"
                    class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg disabled:opacity-50"
                    @click="closeAddProviderModal"
                  >
                    {{ t('common.cancel') }}
                  </button>
                  <button
                    type="submit"
                    :disabled="addingProvider"
                    class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg disabled:opacity-50"
                  >
                    {{ addingProvider ? t('common.processing') : t('common.add') }}
                  </button>
                </div>
              </form>
            </template>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Add API Key Modal -->
    <Teleport to="body">
      <div
        v-if="showKeyModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      >
        <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-md mx-4">
          <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-4">
            {{ t('providerPool.addApiKey') }}
          </h2>
          <form class="space-y-4" @submit.prevent="addAPIKey">
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                t('providerPool.apiKey')
              }}</label>
              <div class="relative">
                <input
                  v-model.trim="newKey.key"
                  :type="showNewKeyApiKey ? 'text' : 'password'"
                  required
                  placeholder="sk-..."
                  class="provider-input-with-action w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
                />
                <button
                  type="button"
                  class="provider-input-action absolute inset-y-0 px-3 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                  :title="showNewKeyApiKey ? 'Hide API Key' : 'Show API Key'"
                  @click="showNewKeyApiKey = !showNewKeyApiKey"
                >
                  <svg
                    v-if="showNewKeyApiKey"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-4 w-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.542-7a10.05 10.05 0 012.132-3.368m3.1-2.329A9.96 9.96 0 0112 5c4.478 0 8.268 2.943 9.542 7a10.036 10.036 0 01-4.293 5.232M15 12a3 3 0 11-4.243-4.243m0 0L3 3m7.757 4.757L21 21"
                    />
                  </svg>
                  <svg
                    v-else
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-4 w-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                    />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M2.458 12C3.732 7.943 7.522 5 12 5s8.268 2.943 9.542 7c-1.274 4.057-5.064 7-9.542 7s-8.268-2.943-9.542-7z"
                    />
                  </svg>
                </button>
              </div>
            </div>
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1"
                >{{ t('providerPool.keyLabel') }} ({{ t('common.optional') }})</label
              >
              <input
                v-model="newKey.label"
                type="text"
                :placeholder="t('providerPool.keyLabelPlaceholder')"
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
              />
            </div>
            <div class="flex justify-end gap-3 mt-6">
              <button
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
                @click="closeKeyModal"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="submit"
                class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg"
              >
                {{ t('common.add') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- Model Pricing Modal -->
    <Teleport to="body">
      <div
        v-if="showPricingModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      >
        <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-md mx-4">
          <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-4">
            {{ t('providerPool.addModelPricing') }}
          </h2>
          <form class="space-y-4" @submit.prevent="saveModelPricing">
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                t('providerPool.modelId')
              }}</label>
              <input
                v-model="pricingForm.modelId"
                type="text"
                required
                placeholder="gpt-4, claude-3-opus, etc."
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
              />
            </div>
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1"
                >{{ t('providerPool.providerId') }} ({{ t('common.optional') }})</label
              >
              <input
                v-model="pricingForm.providerId"
                type="text"
                :placeholder="t('providerPool.modelIdPlaceholder')"
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
              />
            </div>
            <template v-if="pricingForm.isMedia">
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                    t('providerPool.pricePerUnit')
                  }}</label>
                  <input
                    v-model.number="pricingForm.pricePerRequest"
                    type="number"
                    step="0.001"
                    min="0"
                    required
                    class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
                  />
                </div>
                <div>
                  <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                    t('providerPool.pricingUnitLabel')
                  }}</label>
                  <select
                    v-model="pricingForm.pricingUnit"
                    class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
                  >
                    <option value="image">{{ t('providerPool.unitImage') }}</option>
                    <option value="second">{{ t('providerPool.unitSecond') }}</option>
                    <option value="video">{{ t('providerPool.unitVideo') }}</option>
                  </select>
                </div>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('providerPool.mediaUnitHint') }}
              </p>
            </template>
            <template v-else>
              <div class="grid grid-cols-3 gap-3">
                <div>
                  <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                    t('providerPool.inputPrice')
                  }}</label>
                  <input
                    v-model.number="pricingForm.inputPrice"
                    type="number"
                    step="0.01"
                    min="0"
                    required
                    class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
                  />
                </div>
                <div>
                  <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                    t('providerPool.outputPrice')
                  }}</label>
                  <input
                    v-model.number="pricingForm.outputPrice"
                    type="number"
                    step="0.01"
                    min="0"
                    required
                    class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
                  />
                </div>
                <div>
                  <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                    t('providerPool.cachePrice')
                  }}</label>
                  <input
                    v-model.number="pricingForm.cachePrice"
                    type="number"
                    step="0.01"
                    min="0"
                    class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
                  />
                </div>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('providerPool.perMillionTokens') }}
              </p>
            </template>
            <div class="flex justify-end gap-3 mt-6">
              <button
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
                @click="showPricingModal = false"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="submit"
                class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg"
              >
                {{ t('common.save') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- Model Params Modal -->
    <Teleport to="body">
      <div
        v-if="showParamsModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      >
        <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-md mx-4">
          <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-4">
            {{ t('providerPool.editModelParams') }}
          </h2>
          <form class="space-y-4" @submit.prevent="saveModelParams">
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                t('providerPool.temperature')
              }}</label>
              <input
                v-model.number="paramsForm.temperature"
                type="number"
                step="0.1"
                min="0"
                max="2"
                :placeholder="t('providerPool.defaultPlaceholder')"
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
              />
              <p class="text-xs text-gray-400 mt-1">{{ t('providerPool.temperatureHint') }}</p>
            </div>
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                providerOutputLimitLabel
              }}</label>
              <input
                v-model.number="paramsForm.maxTokens"
                type="number"
                step="1"
                min="1"
                :placeholder="t('providerPool.defaultPlaceholder')"
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
              />
              <p class="text-xs text-gray-400 mt-1">
                {{ t('providerPool.maxTokensHint') }}
                <span
                  v-if="displayProvider?.model_params?.detected_max_tokens"
                  class="text-gray-900 dark:text-gray-300"
                >
                  ({{ t('providerPool.detectedMax') }}:
                  {{ displayProvider.model_params.detected_max_tokens }})
                </span>
              </p>
            </div>
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{
                t('providerPool.topP')
              }}</label>
              <input
                v-model.number="paramsForm.topP"
                type="number"
                step="0.1"
                min="0"
                max="1"
                :placeholder="t('providerPool.defaultPlaceholder')"
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
              />
              <p class="text-xs text-gray-400 mt-1">{{ t('providerPool.topPHint') }}</p>
            </div>
            <div class="flex justify-end gap-3 mt-6">
              <button
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
                @click="showParamsModal = false"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="submit"
                class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg"
              >
                {{ t('common.save') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- Allowed Models Modal -->
    <Teleport to="body">
      <div
        v-if="showAllowedModelsModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      >
        <div
          class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-lg mx-4 max-h-[80vh] flex flex-col"
        >
          <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-2">
            {{ t('providerPool.selectPreferredModels') }}
          </h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
            {{ t('providerPool.preferredModelsHint') }}
          </p>

          <!-- Loading state -->
          <div v-if="loadingAllModels" class="flex-1 flex items-center justify-center">
            <div
              class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full"
            ></div>
          </div>

          <!-- Model list -->
          <div v-else class="flex-1 overflow-hidden flex flex-col">
            <!-- Quick actions -->
            <div class="flex gap-2 mb-3">
              <button
                type="button"
                class="px-2 py-1 text-xs bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded"
                @click="selectAllModels"
              >
                {{ t('providerPool.selectAll') }}
              </button>
              <button
                type="button"
                class="px-2 py-1 text-xs bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded"
                @click="clearAllModels"
              >
                {{ t('providerPool.clearAll') }}
              </button>
              <span
                class="provider-inline-start-auto text-xs text-gray-500 dark:text-gray-400 self-center"
              >
                {{ t('providerPool.selectedCount', { count: allowedModelsForm.length }) }}
              </span>
            </div>

            <!-- Model checkboxes -->
            <div
              class="flex-1 overflow-y-auto space-y-1 border border-gray-200 dark:border-slate-600 rounded-lg p-2"
            >
              <label
                v-for="model in allAvailableModels"
                :key="model.id"
                class="flex items-center gap-2 p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded cursor-pointer"
                @mouseenter="model.capabilities?.length && showCapTip($event, model.capabilities)"
                @mouseleave="hideCapTip"
              >
                <input
                  type="checkbox"
                  :checked="allowedModelsForm.includes(model.id)"
                  class="w-4 h-4 text-gray-900 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 border-gray-300 dark:border-slate-500 rounded focus:ring-gray-400"
                  @change="toggleModelInAllowedList(model.id)"
                />
                <div class="flex-1 min-w-0">
                  <span class="text-sm text-gray-900 dark:text-white">{{
                    getLocalizedProviderModelName(model)
                  }}</span>
                  <span
                    v-if="model.capabilities?.length"
                    class="text-[10px] leading-none text-gray-400 provider-inline-start-0_5"
                    >✦</span
                  >
                  <span class="text-xs text-gray-500 provider-inline-start-1">{{ model.id }}</span>
                </div>
                <span v-if="model.context_window" class="text-xs text-gray-400">
                  {{ (model.context_window / 1000).toFixed(0) }}K
                </span>
              </label>
              <div
                v-if="allAvailableModels.length === 0"
                class="text-center py-4 text-gray-500 dark:text-gray-400 text-sm"
              >
                {{ t('providerPool.noModelsAvailable') }}
              </div>
            </div>
          </div>

          <!-- Actions -->
          <div
            class="flex justify-end gap-3 mt-4 pt-4 border-t border-gray-200 dark:border-slate-600"
          >
            <button
              type="button"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
              @click="showAllowedModelsModal = false"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              type="button"
              :disabled="savingAllowedModels"
              class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg disabled:opacity-50"
              @click="saveAllowedModels"
            >
              {{ savingAllowedModels ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Capability tooltip (teleported to body to escape overflow) -->
    <Teleport to="body">
      <div v-if="capTipVisible" class="cap-tooltip" :style="capTipStyle">{{ capTipText }}</div>
    </Teleport>
  </div>
</template>

<style scoped>
.provider-inline-start-0_5 {
  margin-inline-start: 0.125rem;
}

.provider-inline-start-1 {
  margin-inline-start: 0.25rem;
}

.provider-inline-start-2 {
  margin-inline-start: 0.5rem;
}

.provider-inline-start-auto {
  margin-inline-start: auto;
}

.provider-text-end {
  text-align: end;
}

.provider-input-with-action {
  padding-inline-end: 2.5rem;
}

.provider-input-action {
  inset-inline-end: 0;
}

.provider-switch-track {
  position: relative;
}

.provider-switch-track::after {
  content: '';
  position: absolute;
  top: 2px;
  inset-inline-start: 2px;
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 9999px;
  background: #fff;
  transition: transform 150ms ease-in-out;
}

.peer:checked + .provider-switch-track::after {
  transform: translateX(100%);
}

:global(html[dir='rtl']) .peer:checked + .provider-switch-track::after {
  transform: translateX(-100%);
}

.cap-tooltip {
  position: fixed;
  z-index: 99999;
  background: #1e293b;
  color: #fff;
  font-size: 11px;
  line-height: 1.6;
  padding: 6px 10px;
  border-radius: 6px;
  white-space: pre;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  pointer-events: none;
  transform: translateY(-50%);
}
</style>
