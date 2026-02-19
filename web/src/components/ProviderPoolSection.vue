<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useNotificationStore } from '@/stores/notification'
import ProviderIcon from '@/components/ProviderIcon.vue'
import IDEDiscovery from '@/components/IDEDiscovery.vue'
import type { Provider, Model } from '@/api/providerPool'
import { formatTokens } from '@/utils/format'

const { t } = useI18n()
const store = useProviderPoolStore()
const notification = useNotificationStore()

// Local state
const showAddModal = ref(false)
const showKeyModal = ref(false)
const showPricingModal = ref(false)
const showParamsModal = ref(false)
const showAllowedModelsModal = ref(false)
const showIDEDiscoveryModal = ref(false)
const testingProvider = ref<string | null>(null)
const refreshingModels = ref<string | null>(null)
const detectingCapabilities = ref<string | null>(null)
const searchQuery = ref('')
type ProviderTab = 'all' | 'trial' | 'builtin' | 'custom'
const activeTab = ref<ProviderTab>('all')
const availableTabs = computed<ProviderTab[]>(() => {
  const tabs: ProviderTab[] = ['all']
  if (store.trialProviders?.length) tabs.push('trial')
  tabs.push('builtin', 'custom')
  return tabs
})
const iconInput = ref<HTMLInputElement | null>(null)
const uploadingIcon = ref(false)

// Drag and drop state
const draggedProvider = ref<Provider | null>(null)
const dragOverProvider = ref<string | null>(null)

// New provider form
const newProvider = ref({
  name: '',
  base_url: '',
  priority: 50,
  location: 'cloud' as 'cloud' | 'local',
})

// New API key form
const newKey = ref({
  providerId: '',
  key: '',
  label: '',
})

// Pricing form
const pricingForm = ref({
  modelId: '',
  providerId: '',
  inputPrice: 0,
  outputPrice: 0,
  cachePrice: 0,
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


// Computed
const filteredProviders = computed(() => {
  let providers: Provider[] = []

  switch (activeTab.value) {
    case 'all':
      providers = store.providers || []
      break
    case 'trial':
      providers = store.trialProviders || []
      break
    case 'builtin':
      providers = store.builtinProviders || []
      break
    case 'custom':
      providers = store.customProviders || []
      break
  }

  if (searchQuery.value && providers.length > 0) {
    const query = searchQuery.value.toLowerCase()
    providers = providers.filter(p =>
      p.name.toLowerCase().includes(query) ||
      p.id.toLowerCase().includes(query)
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

// Check if selected provider belongs to current tab
const currentTabSelectedProvider = computed(() => {
  if (!store.selectedProvider) return null
  if (activeTab.value === 'all') return store.selectedProvider

  const providerType = store.selectedProvider.type

  // Map provider type to tab
  const typeToTab: Record<string, string> = {
    'builtin': 'builtin',
    'custom': 'custom',
    'acp': 'custom',
    'ide': 'ide',
    'trial': 'trial',
  }

  const expectedTab = typeToTab[providerType] || 'builtin'
  return expectedTab === activeTab.value ? store.selectedProvider : null
})

const selectedProviderModels = computed(() => {
  if (!store.selectedProviderId || !store.models) return []
  return store.models.filter(m => m.provider_id === store.selectedProviderId)
})

// Get custom pricing for a specific model in the selected provider
function getModelCustomPricing(modelId: string) {
  const providerId = store.selectedProviderId
  return store.customPricing.find(p =>
    p.model_id === modelId && (!p.provider_id || p.provider_id === providerId)
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
  const translated = t(i18nKey)
  // If translation exists and is different from the key, use it
  if (translated && translated !== i18nKey) {
    return translated
  }
  // Fall back to provider's description or base_url
  return provider.description || provider.base_url || ''
}

// Reset to 'all' if current tab is no longer available (e.g., trial removed)
watch(availableTabs, (tabs) => {
  if (!tabs.includes(activeTab.value)) activeTab.value = 'all'
})

// Clear selection when switching to a tab with no matching provider
watch(activeTab, () => {
  if (store.selectedProvider && activeTab.value !== 'all') {
    const providerType = store.selectedProvider.type
    const typeToTab: Record<string, string> = {
      'builtin': 'builtin',
      'custom': 'custom',
      'acp': 'custom',
      'ide': 'ide',
    }
    const expectedTab = typeToTab[providerType] || 'builtin'

    // If selected provider doesn't belong to new tab, clear selection
    if (expectedTab !== activeTab.value) {
      store.selectProvider(null)
    }
  }
})

// Methods
async function loadData() {
  try {
    await store.fetchProviders()
    await store.fetchModels()
    await loadPricingData()
  } catch (e) {
    console.error('Failed to load data:', e)
  }
}

async function loadPricingData() {
  try {
    await store.fetchPricingConfig()
    await store.fetchCustomPricing()
  } catch (e) {
    console.error('Failed to load pricing data:', e)
  }
}

function handleIDEImportSuccess(providerId: string) {
  // Refresh providers after successful import
  loadData()
  notification.success(t('ideDiscovery.importSuccess', { ide: 'IDE', provider: providerId }))
  // Switch to the provider that was imported
  store.selectProvider(providerId)
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
  try {
    await store.updateProvider(providerId, { location })
  } catch (e) {
    console.error('Failed to update provider location:', e)
  }
}

async function testConnection(providerId: string) {
  testingProvider.value = providerId
  try {
    await store.testProvider(providerId)
  } catch (e) {
    console.error('Failed to test provider:', e)
  } finally {
    testingProvider.value = null
  }
}

async function refreshModels(providerId: string) {
  refreshingModels.value = providerId
  try {
    const result = await store.refreshModels(providerId)
    if (!result.success) {
      // Show error as notification instead of blocking UI
      notification.error(
        t('providerPool.refreshModelsFailed'),
        result.error,
        { duration: 8000 }
      )
    }
  } finally {
    refreshingModels.value = null
  }
}

async function addCustomProvider() {
  try {
    await store.addProvider({
      name: newProvider.value.name,
      base_url: newProvider.value.base_url,
      priority: newProvider.value.priority,
      location: newProvider.value.location,
      type: 'custom',
    })
    showAddModal.value = false
    newProvider.value = { name: '', base_url: '', priority: 50, location: 'cloud' }
  } catch (e) {
    console.error('Failed to add provider:', e)
  }
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
  showKeyModal.value = true
}

async function addAPIKey() {
  try {
    await store.addAPIKey(
      newKey.value.providerId,
      newKey.value.key,
      newKey.value.label || undefined
    )
    showKeyModal.value = false
  } catch (e) {
    console.error('Failed to add API key:', e)
  }
}

async function removeAPIKey(providerId: string, keyId: string) {
  if (!confirm(t('providerPool.confirmDeleteKey'))) return
  try {
    await store.removeAPIKey(providerId, keyId)
  } catch (e) {
    console.error('Failed to remove API key:', e)
  }
}

function selectProvider(providerId: string) {
  store.selectProvider(store.selectedProviderId === providerId ? null : providerId)
}

// Drag and drop handlers
function handleDragStart(e: DragEvent, provider: Provider) {
  draggedProvider.value = provider
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', provider.id)
  }
}

function handleDragOver(e: DragEvent, provider: Provider) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
  if (draggedProvider.value && draggedProvider.value.id !== provider.id) {
    dragOverProvider.value = provider.id
  }
}

function handleDragLeave() {
  dragOverProvider.value = null
}

function handleDragEnd() {
  draggedProvider.value = null
  dragOverProvider.value = null
}

async function handleDrop(e: DragEvent, targetProvider: Provider) {
  e.preventDefault()
  dragOverProvider.value = null

  if (!draggedProvider.value || draggedProvider.value.id === targetProvider.id) {
    draggedProvider.value = null
    return
  }

  // Get current list and find indices
  const providers = [...filteredProviders.value]
  const draggedIndex = providers.findIndex(p => p.id === draggedProvider.value!.id)
  const targetIndex = providers.findIndex(p => p.id === targetProvider.id)

  if (draggedIndex === -1 || targetIndex === -1) {
    draggedProvider.value = null
    return
  }

  // Reorder the list
  const [removed] = providers.splice(draggedIndex, 1)
  if (!removed) {
    draggedProvider.value = null
    return
  }
  providers.splice(targetIndex, 0, removed)

  // Calculate new priorities (higher index = lower priority, so we reverse)
  const maxPriority = 100
  const step = Math.floor(maxPriority / (providers.length + 1))

  // Collect updates for batch sync
  const updates: Array<{ id: string; priority: number }> = []

  // Update priorities locally first (instant UI update)
  for (let i = 0; i < providers.length; i++) {
    const provider = providers[i]
    if (!provider) continue
    const newPriority = maxPriority - (i * step)
    if (provider.priority !== newPriority) {
      store.updateProviderPriorityLocal(provider.id, newPriority)
      updates.push({ id: provider.id, priority: newPriority })
    }
  }

  // Sync to backend in background (fire and forget)
  if (updates.length > 0) {
    store.syncPrioritiesToBackend(updates)
  }

  draggedProvider.value = null
}

function getStatusColor(status: string, enabled: boolean) {
  if (!enabled) return 'text-gray-500'
  switch (status) {
    case 'active': return 'text-green-500'
    case 'error': return 'text-red-500'
    default: return 'text-green-500' // enabled but not yet tested
  }
}

function getStatusIcon(status: string, enabled: boolean) {
  if (!enabled) return '○'
  switch (status) {
    case 'active': return '●'
    case 'error': return '⚠'
    default: return '●' // enabled but not yet tested
  }
}

function formatCapabilities(caps: Model['capabilities']) {
  const labels: string[] = []
  if (caps.chat) labels.push('Chat')
  if (caps.vision) labels.push('Vision')
  if (caps.function_call) labels.push('Tools')
  if (caps.thinking) labels.push('Thinking')
  if (caps.streaming) labels.push('Stream')
  return labels.join(', ')
}

function openPricingModal(model?: Model) {
  if (model) {
    const customPricing = getModelCustomPricing(model.id)
    pricingForm.value = {
      modelId: model.id,
      providerId: model.provider_id,
      inputPrice: customPricing?.input_price ?? model.input_price ?? 0,
      outputPrice: customPricing?.output_price ?? model.output_price ?? 0,
      cachePrice: customPricing?.cache_price ?? 0,
    }
  } else {
    pricingForm.value = {
      modelId: '',
      providerId: store.selectedProviderId || '',
      inputPrice: 0,
      outputPrice: 0,
      cachePrice: 0,
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
  const provider = currentTabSelectedProvider.value
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
  const provider = currentTabSelectedProvider.value
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
  const provider = currentTabSelectedProvider.value
  if (!provider) return

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
  const provider = currentTabSelectedProvider.value
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
      allowedModelsForm.value = allAvailableModels.value.map(m => m.id)
    }
  } catch (e) {
    console.error('Failed to load models:', e)
    allAvailableModels.value = selectedProviderModels.value
    // On error, default to all models selected
    allowedModelsForm.value = allAvailableModels.value.map(m => m.id)
  } finally {
    loadingAllModels.value = false
  }
}

function toggleModelInAllowedList(modelId: string) {
  // Simple toggle: checked = in array, unchecked = not in array
  if (allowedModelsForm.value.includes(modelId)) {
    allowedModelsForm.value = allowedModelsForm.value.filter(id => id !== modelId)
  } else {
    allowedModelsForm.value = [...allowedModelsForm.value, modelId]
  }
}

function selectAllModels() {
  allowedModelsForm.value = allAvailableModels.value.map(m => m.id)
}

function clearAllModels() {
  allowedModelsForm.value = []
}

async function saveAllowedModels() {
  const provider = currentTabSelectedProvider.value
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
  if (!file || !currentTabSelectedProvider.value) return

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
      if (dataUrl && currentTabSelectedProvider.value) {
        await store.updateProviderIcon(currentTabSelectedProvider.value.id, dataUrl)
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
          class="px-3 py-1.5 bg-gray-100 dark:bg-slate-700 hover:bg-gray-200 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-300 rounded-lg flex items-center gap-1 text-sm transition-colors"
          @click="showIDEDiscoveryModal = true"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          {{ t('providerPool.scanIDE') }}
        </button>
        <button
          class="px-3 py-1.5 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg flex items-center gap-1 text-sm transition-colors"
          @click="showAddModal = true"
        >
          <span>+</span>
          {{ t('providerPool.addCustom') }}
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
            : 'bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-slate-600'
        ]"
        @click="activeTab = tab"
      >
        {{ t(`providerPool.tabs.${tab}`) }}
        <span class="ml-1 text-xs opacity-70">
          ({{ tab === 'all' ? (store.providers?.length || 0) : tab === 'trial' ? (store.trialProviders?.length || 0) : tab === 'builtin' ? (store.builtinProviders?.length || 0) : (store.customProviders?.length || 0) }})
        </span>
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
        store.trialQuota.exhausted
          ? 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'
          : 'bg-gray-100 dark:bg-gray-700/30 border-gray-200 dark:border-gray-600'
      ]"
    >
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="text-lg">{{ store.trialQuota.exhausted ? '⚠️' : '🎁' }}</span>
          <div>
            <h4 class="font-medium text-sm" :class="store.trialQuota.exhausted ? 'text-red-700 dark:text-red-300' : 'text-gray-600 dark:text-gray-400'">
              {{ t('providerPool.trialQuota.title') }}
            </h4>
            <p class="text-xs" :class="store.trialQuota.exhausted ? 'text-red-600 dark:text-red-400' : 'text-gray-600 dark:text-gray-400'">
              {{ store.trialQuota.exhausted ? t('providerPool.trialQuota.exhausted') : t('providerPool.trialQuota.remaining', { tokens: formatTokens(store.trialQuota.tokens_remaining) }) }}
            </p>
          </div>
        </div>
        <div class="text-right text-xs" :class="store.trialQuota.exhausted ? 'text-red-500 dark:text-red-400' : 'text-gray-600 dark:text-gray-400'">
          <div :title="store.trialQuota.tokens_remaining.toLocaleString() + ' / ' + store.trialQuota.token_limit.toLocaleString()">{{ t('providerPool.trialQuota.tokensRemaining', { remaining: formatTokens(store.trialQuota.tokens_remaining), limit: formatTokens(store.trialQuota.token_limit) }) }}</div>
          <div class="mt-1 w-24 h-1.5 bg-gray-300 dark:bg-gray-600 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full transition-all"
              :class="store.trialQuota.exhausted ? 'bg-red-500' : 'bg-gray-400 dark:bg-gray-500'"
              :style="{ width: `${Math.max(0, Math.min(100, (store.trialQuota.tokens_remaining / store.trialQuota.token_limit) * 100))}%` }"
            ></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="text-center py-8">
      <div class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full mx-auto"></div>
      <p class="text-gray-400 mt-2 text-sm">{{ t('common.loading') }}</p>
    </div>

    <!-- Error -->
    <div v-else-if="store.error" class="bg-red-100 dark:bg-red-900/20 border border-red-300 dark:border-red-500 rounded-lg p-3 mb-4">
      <p class="text-red-600 dark:text-red-400 text-sm">{{ store.error }}</p>
      <button class="text-red-500 dark:text-red-300 underline mt-1 text-sm" @click="store.clearError()">
        {{ t('common.dismiss') }}
      </button>
    </div>

    <!-- Provider List -->
    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <!-- Provider Cards -->
      <div class="space-y-2 max-h-[360px] overflow-y-auto lg:max-h-[480px]">
        <!-- Drag hint -->
        <p v-if="filteredProviders.length > 1 && !searchQuery" class="text-xs text-gray-400 dark:text-gray-500 mb-2 flex items-center gap-1">
          <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
          </svg>
          {{ t('providerPool.dragToReorder') }}
        </p>
        <div
          v-for="provider in filteredProviders"
          :key="provider.id"
          draggable="true"
          :class="[
            'p-3 rounded-lg border transition-all select-none group/card',
            store.selectedProviderId === provider.id
              ? 'bg-gray-100 dark:bg-gray-700/20 border-gray-300 dark:border-gray-600'
              : 'bg-gray-50 dark:bg-slate-800/50 border-gray-200 dark:border-slate-700 hover:border-gray-300 dark:hover:border-slate-600',
            dragOverProvider === provider.id ? 'border-gray-300 dark:border-gray-600 border-dashed bg-gray-100 dark:bg-gray-700/10' : '',
            draggedProvider?.id === provider.id ? 'opacity-50' : '',
            draggedProvider ? 'cursor-grabbing' : 'cursor-pointer'
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
                :class="[
                  'flex-shrink-0 text-gray-400 dark:text-gray-500 cursor-grab transition-opacity',
                  draggedProvider ? 'opacity-100' : 'opacity-0'
                ]"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                </svg>
              </div>
              <ProviderIcon :provider-id="provider.id" :custom-icon="provider.custom_icon" size="lg" />
              <div>
                <h3 class="font-medium text-gray-900 dark:text-white text-sm">{{ getProviderName(provider) }}</h3>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ provider.id }}</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <!-- Free tier badge -->
              <span
                v-if="provider.id === 'nvidia'"
                class="text-[10px] px-1.5 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400"
              >
                {{ t('providerPool.freeTier') }}
              </span>
              <!-- API Keys count -->
              <span
                v-if="provider.api_keys?.length"
                class="text-[10px] px-1.5 py-0.5 rounded bg-gray-200 dark:bg-slate-600 text-gray-600 dark:text-gray-300"
                :title="t('providerPool.apiKeys')"
              >
                🔑 {{ provider.api_keys.length }}
              </span>
              <!-- Location badge -->
              <span
                v-if="provider.location"
                :class="[
                  'text-[10px] px-1.5 py-0.5 rounded',
                  provider.location === 'cloud' ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400' : 'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400'
                ]"
              >
                {{ provider.location === 'cloud' ? '☁️' : '💻' }}
              </span>
              <span
                :class="getStatusColor(provider.status, provider.enabled)"
                class="text-xs"
                :title="provider.status === 'error' && provider.last_error ? provider.last_error : ''"
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
                <div class="w-8 h-4 bg-gray-300 dark:bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-3 after:w-3 after:transition-all peer-checked:bg-green-600 dark:peer-checked:bg-green-500"></div>
              </label>
            </div>
          </div>
        </div>

        <div v-if="filteredProviders.length === 0" class="text-center py-6 text-gray-500 dark:text-gray-400 text-sm">
          {{ t('providerPool.noProviders') }}
        </div>
      </div>

      <!-- Provider Details -->
      <div class="space-y-2 lg:max-h-[480px] lg:overflow-y-auto">
        <div v-if="currentTabSelectedProvider" class="bg-white dark:bg-slate-800/50 rounded-lg border border-gray-200 dark:border-slate-700 p-4">
          <div class="flex items-center justify-between mb-4">
            <div class="flex items-center gap-3">
              <div class="relative group">
                <!-- Builtin provider: clickable logo to website -->
                <a
                  v-if="currentTabSelectedProvider.type === 'builtin' && currentTabSelectedProvider.website"
                  :href="currentTabSelectedProvider.website"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="block cursor-pointer hover:opacity-80 transition-opacity"
                  :title="t('providerPool.visitWebsite')"
                >
                  <ProviderIcon :provider-id="currentTabSelectedProvider.id" :custom-icon="currentTabSelectedProvider.custom_icon" size="xl" />
                </a>
                <!-- Non-builtin or no website: regular icon -->
                <template v-else>
                  <ProviderIcon :provider-id="currentTabSelectedProvider.id" :custom-icon="currentTabSelectedProvider.custom_icon" size="xl" />
                </template>
                <button
                  v-if="currentTabSelectedProvider.type === 'custom'"
                  class="absolute inset-0 flex items-center justify-center bg-black/50 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                  :title="t('providerPool.changeIcon')"
                  @click="openIconUpload"
                >
                  <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
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
              <div>
                <h2 class="font-bold text-gray-900 dark:text-white">{{ getProviderName(currentTabSelectedProvider) }}</h2>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ getProviderDescription(currentTabSelectedProvider) }}</p>
                <!-- Location Toggle -->
                <div class="flex items-center gap-1 mt-1">
                  <span class="text-xs text-gray-400">{{ t('providerPool.location') }}:</span>
                  <button
                    :class="[
                      'px-1.5 py-0.5 rounded text-[10px] transition-colors',
                      currentTabSelectedProvider.location === 'cloud'
                        ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400'
                        : 'bg-gray-200 dark:bg-slate-600 text-gray-600 dark:text-gray-400 hover:bg-gray-300 dark:hover:bg-slate-500'
                    ]"
                    @click="updateProviderLocation(currentTabSelectedProvider!.id, 'cloud')"
                  >
                    ☁️ {{ t('providerPool.locationCloud') }}
                  </button>
                  <button
                    :class="[
                      'px-1.5 py-0.5 rounded text-[10px] transition-colors',
                      currentTabSelectedProvider.location === 'local'
                        ? 'bg-green-500 text-white'
                        : 'bg-gray-200 dark:bg-slate-600 text-gray-600 dark:text-gray-400 hover:bg-gray-300 dark:hover:bg-slate-500'
                    ]"
                    @click="updateProviderLocation(currentTabSelectedProvider!.id, 'local')"
                  >
                    💻 {{ t('providerPool.locationLocal') }}
                  </button>
                </div>
              </div>
            </div>
            <div class="flex gap-1">
              <button
                :disabled="testingProvider === currentTabSelectedProvider!.id"
                class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs disabled:opacity-50"
                @click="testConnection(currentTabSelectedProvider!.id)"
              >
                {{ testingProvider === currentTabSelectedProvider!.id ? t('common.testing') : t('providerPool.test') }}
              </button>
              <button
                :disabled="refreshingModels === currentTabSelectedProvider!.id"
                class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs disabled:opacity-50"
                @click="refreshModels(currentTabSelectedProvider!.id)"
              >
                {{ refreshingModels === currentTabSelectedProvider!.id ? t('common.refreshing') : t('providerPool.refreshModels') }}
              </button>
              <button
                v-if="currentTabSelectedProvider!.type === 'custom'"
                class="px-2 py-1 bg-red-500 hover:bg-red-600 text-white rounded text-xs"
                @click="deleteProvider(currentTabSelectedProvider!.id)"
              >
                {{ t('common.delete') }}
              </button>
            </div>
          </div>

          <!-- Error Banner -->
          <div
            v-if="currentTabSelectedProvider!.status === 'error' && currentTabSelectedProvider!.last_error"
            class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"
          >
            <div class="flex items-start gap-2">
              <span class="text-red-500 flex-shrink-0">⚠</span>
              <p class="text-xs text-red-600 dark:text-red-400 break-all">{{ currentTabSelectedProvider!.last_error }}</p>
            </div>
          </div>

          <!-- Model Parameters Section -->
          <div class="mb-4">
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('providerPool.modelParams') }}</h3>
              <div class="flex gap-1">
                <button
                  :disabled="detectingCapabilities === currentTabSelectedProvider!.id"
                  class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs disabled:opacity-50"
                  @click="detectCapabilities"
                >
                  {{ detectingCapabilities === currentTabSelectedProvider!.id ? t('providerPool.detecting') : t('providerPool.detectCapabilities') }}
                </button>
                <button
                  class="px-2 py-1 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded text-xs"
                  @click="openParamsModal"
                >
                  {{ t('common.edit') }}
                </button>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-2 p-2 bg-white dark:bg-slate-900/50 rounded text-xs">
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('providerPool.temperature') }}:</span>
                <span class="text-gray-900 dark:text-white ml-1">
                  {{ currentTabSelectedProvider!.model_params?.temperature ?? t('providerPool.default') }}
                </span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('providerPool.maxTokens') }}:</span>
                <span
                  v-if="currentTabSelectedProvider!.model_params?.max_tokens"
                  class="text-gray-900 dark:text-white ml-1"
                >
                  {{ currentTabSelectedProvider!.model_params.max_tokens }}
                </span>
                <span
                  v-else-if="currentTabSelectedProvider!.model_params?.detected_max_tokens"
                  class="text-gray-900 dark:text-gray-300 ml-1"
                  :title="t('providerPool.detectedMax')"
                >
                  {{ currentTabSelectedProvider!.model_params.detected_max_tokens }}
                  <span class="text-gray-400 text-[10px]">({{ t('providerPool.detected') }})</span>
                </span>
                <span v-else class="text-gray-900 dark:text-white ml-1">
                  {{ t('providerPool.default') }}
                </span>
              </div>
            </div>
          </div>

          <!-- API Keys Section -->
          <div class="mb-4">
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('providerPool.apiKeys') }}</h3>
              <div class="flex items-center gap-2">
                <a
                  v-if="currentTabSelectedProvider!.id === 'nvidia'"
                  href="https://build.nvidia.com/settings/api-keys"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="px-2 py-1 bg-green-600 hover:bg-green-700 text-white rounded text-xs flex items-center gap-1"
                >
                  {{ t('providerPool.getApiKey') }}
                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                  </svg>
                </a>
                <button
                  v-if="currentTabSelectedProvider!.type !== 'trial'"
                  class="px-2 py-1 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded text-xs"
                  @click="openKeyModal(currentTabSelectedProvider!.id)"
                >
                  + {{ t('providerPool.addKey') }}
                </button>
              </div>
            </div>
            <div class="space-y-1">
              <!-- Trial provider: show placeholder -->
              <div v-if="currentTabSelectedProvider!.type === 'trial'" class="flex items-center justify-between p-2 bg-white dark:bg-slate-900/50 rounded text-xs">
                <div>
                  <span class="text-gray-700 dark:text-white font-mono">••••••••••••••••</span>
                  <span class="ml-2 text-gray-500">({{ t('providerPool.trial.name') }})</span>
                </div>
              </div>
              <!-- Non-trial provider: show actual keys -->
              <template v-else>
                <div
                  v-for="key in currentTabSelectedProvider!.api_keys"
                  :key="key.id"
                  class="flex items-center justify-between p-2 bg-white dark:bg-slate-900/50 rounded text-xs"
                >
                  <div>
                    <span class="text-gray-700 dark:text-white font-mono">{{ key.key_hash }}</span>
                    <span v-if="key.label" class="ml-2 text-gray-500">({{ formatKeyLabel(key.label) }})</span>
                  </div>
                  <div class="flex items-center gap-3">
                    <span v-if="key.usage_count" class="text-gray-500">
                      {{ t('providerPool.usageCount') }}: {{ key.usage_count }}
                    </span>
                    <button
                      class="text-red-400 hover:text-red-300"
                      @click="removeAPIKey(currentTabSelectedProvider!.id, key.id)"
                    >
                      ✕
                    </button>
                  </div>
                </div>
                <div v-if="!currentTabSelectedProvider!.api_keys?.length" class="text-gray-500 dark:text-gray-400 text-center py-2 text-xs">
                  {{ t('providerPool.noKeys') }}
                </div>
              </template>
            </div>
          </div>

          <!-- Trial Quota Section (only for trial providers) -->
          <div v-if="currentTabSelectedProvider!.type === 'trial' && store.trialQuota" class="mb-4 p-3 bg-gray-100 dark:bg-gray-700/30 rounded-lg border border-gray-200 dark:border-gray-600">
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-300 mb-2">{{ t('providerPool.trial.name') }}</h3>
            <div class="space-y-2">
              <div class="flex items-center justify-between text-xs">
                <span class="text-gray-700 dark:text-gray-400" :title="(store.trialQuota.token_limit - store.trialQuota.tokens_used).toLocaleString() + ' / ' + store.trialQuota.token_limit.toLocaleString()">{{ t('providerPool.trial.tokensUsed', { remaining: formatTokens(store.trialQuota.token_limit - store.trialQuota.tokens_used), total: formatTokens(store.trialQuota.token_limit) }) }}</span>
                <div class="relative w-24 h-4 bg-gray-300 dark:bg-gray-600 rounded-full overflow-hidden">
                  <div
                    class="h-full bg-gray-400 dark:bg-gray-500 transition-all"
                    :style="{ width: `${Math.max(3, Math.min(100, ((store.trialQuota.token_limit - store.trialQuota.tokens_used) / store.trialQuota.token_limit) * 100))}%` }"
                  />
                  <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-900 dark:text-gray-700 drop-shadow-sm">
                    {{ Math.round(((store.trialQuota.token_limit - store.trialQuota.tokens_used) / store.trialQuota.token_limit) * 100) }}%
                  </span>
                </div>
              </div>
              <p v-if="store.trialQuota.exhausted" class="text-xs text-red-600 dark:text-red-400 mt-2">
                {{ store.trialQuota.exhausted_by_tokens ? t('providerPool.trial.quotaExhaustedTokens') : t('providerPool.trial.quotaExhaustedConversations') }}
              </p>
            </div>
          </div>

          <!-- Models Section with Pricing -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('providerPool.models') }}
                <span class="text-gray-500 text-xs ml-1">({{ selectedProviderModels.length }})</span>
                <span
                  v-if="currentTabSelectedProvider?.allowed_models?.length"
                  class="text-gray-900 dark:text-gray-300 text-xs ml-1"
                  :title="t('providerPool.filteredModels')"
                >
                  ({{ t('providerPool.filtered') }})
                </span>
              </h3>
              <button
                class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs"
                @click="openAllowedModelsModal"
              >
                {{ t('providerPool.configureModels') }}
              </button>
            </div>
            <div class="space-y-1 max-h-64 overflow-y-auto">
              <div
                v-for="model in selectedProviderModels"
                :key="model.id"
                class="p-2 bg-white dark:bg-slate-900/50 rounded text-xs group"
              >
                <div class="flex items-center justify-between">
                  <div class="flex-1 min-w-0">
                    <span class="text-gray-900 dark:text-white font-medium">{{ model.display_name || model.name }}</span>
                    <span class="text-gray-500 ml-1 truncate">{{ model.id }}</span>
                  </div>
                  <div class="flex items-center gap-2 ml-2">
                    <span v-if="model.context_window" class="text-gray-500 whitespace-nowrap">
                      {{ (model.context_window / 1000).toFixed(0) }}K
                    </span>
                    <!-- Pricing display -->
                    <span
                      :class="hasCustomPricing(model.id) ? 'text-gray-900 dark:text-gray-300' : 'text-green-500'"
                      class="whitespace-nowrap cursor-pointer hover:underline"
                      :title="hasCustomPricing(model.id) ? t('providerPool.customPricing') : t('providerPool.defaultPricing')"
                      @click.stop="openPricingModal(model)"
                    >
                      ${{ (getModelCustomPricing(model.id)?.input_price ?? model.input_price ?? 0).toFixed(2) }}/${{ (getModelCustomPricing(model.id)?.output_price ?? model.output_price ?? 0).toFixed(2) }}
                    </span>
                    <!-- Edit pricing button -->
                    <button
                      class="opacity-0 group-hover:opacity-100 p-1 hover:bg-gray-100 dark:hover:bg-slate-700 rounded transition-opacity"
                      :title="t('providerPool.editPricing')"
                      @click.stop="openPricingModal(model)"
                    >
                      <svg class="w-3 h-3 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                      </svg>
                    </button>
                    <!-- Remove custom pricing button -->
                    <button
                      v-if="hasCustomPricing(model.id)"
                      class="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-100 dark:hover:bg-red-900/30 rounded transition-opacity"
                      :title="t('providerPool.resetPricing')"
                      @click.stop="removeCustomPricing(model.id, store.selectedProviderId ?? undefined)"
                    >
                      <svg class="w-3 h-3 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </div>
                </div>
                <div class="mt-1 text-gray-500">
                  {{ formatCapabilities(model.capabilities) }}
                </div>
              </div>
              <div v-if="selectedProviderModels.length === 0" class="text-gray-500 dark:text-gray-400 text-center py-2 text-xs">
                {{ t('providerPool.noModels') }}
              </div>
            </div>
            <p class="text-xs text-gray-400 mt-2">{{ t('providerPool.pricingHint') }}</p>
          </div>
        </div>

        <div v-else class="bg-white dark:bg-slate-800/30 rounded-lg border border-gray-200 dark:border-slate-700 p-8 text-center">
          <p class="text-gray-500 dark:text-gray-400 text-sm">{{ t('providerPool.selectProvider') }}</p>
        </div>
      </div>
    </div>

    <!-- Add Provider Modal -->
    <div v-if="showAddModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-md mx-4">
        <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-4">{{ t('providerPool.addCustomProvider') }}</h2>
        <form class="space-y-4" @submit.prevent="addCustomProvider">
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.providerName') }}</label>
            <input
              v-model="newProvider.name"
              type="text"
              required
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.baseUrl') }}</label>
            <input
              v-model="newProvider.base_url"
              type="url"
              required
              placeholder="https://api.example.com/v1"
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.location') }}</label>
            <div class="flex gap-2">
              <button
                type="button"
                :class="[
                  'flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg border transition-colors',
                  newProvider.location === 'cloud'
                    ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-400'
                    : 'bg-gray-100 dark:bg-slate-700 border-gray-200 dark:border-slate-600 text-gray-600 dark:text-gray-400 hover:border-gray-300 dark:hover:border-slate-500'
                ]"
                @click="newProvider.location = 'cloud'"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
                </svg>
                {{ t('providerPool.locationCloud') }}
              </button>
              <button
                type="button"
                :class="[
                  'flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg border transition-colors',
                  newProvider.location === 'local'
                    ? 'bg-green-500/20 border-green-500 text-green-500'
                    : 'bg-gray-100 dark:bg-slate-700 border-gray-200 dark:border-slate-600 text-gray-600 dark:text-gray-400 hover:border-gray-300 dark:hover:border-slate-500'
                ]"
                @click="newProvider.location = 'local'"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
                {{ t('providerPool.locationLocal') }}
              </button>
            </div>
            <p class="text-xs text-gray-400 mt-1">{{ t('providerPool.locationHint') }}</p>
          </div>
          <div class="flex justify-end gap-3 mt-6">
            <button
              type="button"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
              @click="showAddModal = false"
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

    <!-- Add API Key Modal -->
    <div v-if="showKeyModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-md mx-4">
        <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-4">{{ t('providerPool.addApiKey') }}</h2>
        <form class="space-y-4" @submit.prevent="addAPIKey">
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.apiKey') }}</label>
            <input
              v-model="newKey.key"
              type="password"
              required
              placeholder="sk-..."
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.keyLabel') }} ({{ t('common.optional') }})</label>
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
              @click="showKeyModal = false"
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

    <!-- Model Pricing Modal -->
    <div v-if="showPricingModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-md mx-4">
        <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-4">{{ t('providerPool.addModelPricing') }}</h2>
        <form class="space-y-4" @submit.prevent="saveModelPricing">
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.modelId') }}</label>
            <input
              v-model="pricingForm.modelId"
              type="text"
              required
              placeholder="gpt-4, claude-3-opus, etc."
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.providerId') }} ({{ t('common.optional') }})</label>
            <input
              v-model="pricingForm.providerId"
              type="text"
              :placeholder="t('providerPool.modelIdPlaceholder')"
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
            />
          </div>
          <div class="grid grid-cols-3 gap-3">
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.inputPrice') }}</label>
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
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.outputPrice') }}</label>
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
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.cachePrice') }}</label>
              <input
                v-model.number="pricingForm.cachePrice"
                type="number"
                step="0.01"
                min="0"
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-400"
              />
            </div>
          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('providerPool.perMillionTokens') }}</p>
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

    <!-- Model Params Modal -->
    <div v-if="showParamsModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-md mx-4">
        <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-4">{{ t('providerPool.editModelParams') }}</h2>
        <form class="space-y-4" @submit.prevent="saveModelParams">
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.temperature') }}</label>
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
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.maxTokens') }}</label>
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
              <span v-if="currentTabSelectedProvider?.model_params?.detected_max_tokens" class="text-gray-900 dark:text-gray-300">
                ({{ t('providerPool.detectedMax') }}: {{ currentTabSelectedProvider.model_params.detected_max_tokens }})
              </span>
            </p>
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.topP') }}</label>
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

    <!-- Allowed Models Modal -->
    <div v-if="showAllowedModelsModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-lg mx-4 max-h-[80vh] flex flex-col">
        <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-2">{{ t('providerPool.configureAllowedModels') }}</h2>
        <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">{{ t('providerPool.allowedModelsHint') }}</p>

        <!-- Loading state -->
        <div v-if="loadingAllModels" class="flex-1 flex items-center justify-center">
          <div class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full"></div>
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
            <span class="text-xs text-gray-500 dark:text-gray-400 ml-auto self-center">
              {{ t('providerPool.selectedCount', { count: allowedModelsForm.length }) }}
            </span>
          </div>

          <!-- Model checkboxes -->
          <div class="flex-1 overflow-y-auto space-y-1 border border-gray-200 dark:border-slate-600 rounded-lg p-2">
            <label
              v-for="model in allAvailableModels"
              :key="model.id"
              class="flex items-center gap-2 p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded cursor-pointer"
            >
              <input
                type="checkbox"
                :checked="allowedModelsForm.includes(model.id)"
                class="w-4 h-4 text-gray-900 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 border-gray-300 dark:border-slate-500 rounded focus:ring-gray-400"
                @change="toggleModelInAllowedList(model.id)"
              />
              <div class="flex-1 min-w-0">
                <span class="text-sm text-gray-900 dark:text-white">{{ model.display_name || model.name }}</span>
                <span class="text-xs text-gray-500 ml-1">{{ model.id }}</span>
              </div>
              <span v-if="model.context_window" class="text-xs text-gray-400">
                {{ (model.context_window / 1000).toFixed(0) }}K
              </span>
            </label>
            <div v-if="allAvailableModels.length === 0" class="text-center py-4 text-gray-500 dark:text-gray-400 text-sm">
              {{ t('providerPool.noModelsAvailable') }}
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="flex justify-end gap-3 mt-4 pt-4 border-t border-gray-200 dark:border-slate-600">
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

    <!-- IDE Discovery Modal -->
    <Teleport to="body">
    <div
      v-if="showIDEDiscoveryModal"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="showIDEDiscoveryModal = false"
    >
      <div class="bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-4xl w-full mx-4 max-h-[90vh] overflow-hidden flex flex-col">
        <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-slate-600 flex-shrink-0">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('ideDiscovery.title') }}
          </h2>
          <button
            class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            @click="showIDEDiscoveryModal = false"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div class="flex-1 overflow-y-auto">
          <IDEDiscovery @import-success="handleIDEImportSuccess" />
        </div>
      </div>
    </div>
    </Teleport>
  </div>
</template>
