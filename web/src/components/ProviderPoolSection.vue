<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useNotificationStore } from '@/stores/notification'
import ProviderIcon from '@/components/ProviderIcon.vue'
import type { Provider, Model } from '@/api/providerPool'

const { t } = useI18n()
const store = useProviderPoolStore()
const notification = useNotificationStore()

// Local state
const showAddModal = ref(false)
const showKeyModal = ref(false)
const showPricingModal = ref(false)
const showParamsModal = ref(false)
const testingProvider = ref<string | null>(null)
const refreshingModels = ref<string | null>(null)
const detectingCapabilities = ref<string | null>(null)
const searchQuery = ref('')
const activeTab = ref<'all' | 'builtin' | 'custom' | 'ide'>('all')
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


// Computed
const filteredProviders = computed(() => {
  let providers: Provider[] = []

  switch (activeTab.value) {
    case 'all':
      providers = store.providers || []
      break
    case 'builtin':
      providers = store.builtinProviders || []
      break
    case 'custom':
      providers = store.customProviders || []
      break
    case 'ide':
      providers = store.ideProviders || []
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
  providers.splice(targetIndex, 0, removed)

  // Calculate new priorities (higher index = lower priority, so we reverse)
  const maxPriority = 100
  const step = Math.floor(maxPriority / (providers.length + 1))

  // Collect updates for batch sync
  const updates: Array<{ id: string; priority: number }> = []

  // Update priorities locally first (instant UI update)
  for (let i = 0; i < providers.length; i++) {
    const newPriority = maxPriority - (i * step)
    if (providers[i].priority !== newPriority) {
      store.updateProviderPriorityLocal(providers[i].id, newPriority)
      updates.push({ id: providers[i].id, priority: newPriority })
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
      <button
        @click="showAddModal = true"
        class="px-3 py-1.5 bg-accent hover:bg-accent-hover text-white rounded-lg flex items-center gap-1 text-sm transition-colors"
      >
        <span>+</span>
        {{ t('providerPool.addCustom') }}
      </button>
    </div>

    <!-- Tabs -->
    <div class="flex gap-2 mb-4">
      <button
        v-for="tab in ['all', 'builtin', 'custom', 'ide'] as const"
        :key="tab"
        @click="activeTab = tab"
        :class="[
          'px-3 py-1.5 rounded-lg transition-colors text-sm',
          activeTab === tab
            ? 'bg-accent text-white'
            : 'bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-slate-600'
        ]"
      >
        {{ t(`providerPool.tabs.${tab}`) }}
        <span class="ml-1 text-xs opacity-70">
          ({{ tab === 'all' ? store.providers.length : tab === 'builtin' ? store.builtinProviders.length : tab === 'custom' ? store.customProviders.length : store.ideProviders.length }})
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
        class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-accent text-sm"
      />
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="text-center py-8">
      <div class="animate-spin w-6 h-6 border-2 border-accent border-t-transparent rounded-full mx-auto"></div>
      <p class="text-gray-400 mt-2 text-sm">{{ t('common.loading') }}</p>
    </div>

    <!-- Error -->
    <div v-else-if="store.error" class="bg-red-100 dark:bg-red-900/20 border border-red-300 dark:border-red-500 rounded-lg p-3 mb-4">
      <p class="text-red-600 dark:text-red-400 text-sm">{{ store.error }}</p>
      <button @click="store.clearError()" class="text-red-500 dark:text-red-300 underline mt-1 text-sm">
        {{ t('common.dismiss') }}
      </button>
    </div>

    <!-- Provider List -->
    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <!-- Provider Cards -->
      <div class="space-y-2 max-h-[480px] overflow-y-auto">
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
          @click="selectProvider(provider.id)"
          @dragstart="handleDragStart($event, provider)"
          @dragover="handleDragOver($event, provider)"
          @dragleave="handleDragLeave"
          @dragend="handleDragEnd"
          @drop="handleDrop($event, provider)"
          :class="[
            'p-3 rounded-lg border transition-all select-none group/card',
            store.selectedProviderId === provider.id
              ? 'bg-accent/10 dark:bg-accent/20 border-accent'
              : 'bg-gray-50 dark:bg-slate-800/50 border-gray-200 dark:border-slate-700 hover:border-gray-300 dark:hover:border-slate-600',
            dragOverProvider === provider.id ? 'border-accent border-dashed bg-accent/5' : '',
            draggedProvider?.id === provider.id ? 'opacity-50' : '',
            draggedProvider ? 'cursor-grabbing' : 'cursor-pointer'
          ]"
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
                <h3 class="font-medium text-gray-900 dark:text-white text-sm">{{ provider.name }}</h3>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ provider.id }}</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
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
              <span :class="getStatusColor(provider.status, provider.enabled)" class="text-xs">
                {{ getStatusIcon(provider.status, provider.enabled) }}
              </span>
              <label class="relative inline-flex items-center cursor-pointer" @click.stop>
                <input
                  type="checkbox"
                  :checked="provider.enabled"
                  @change="toggleProvider(provider)"
                  class="sr-only peer"
                />
                <div class="w-8 h-4 bg-gray-300 dark:bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-3 after:w-3 after:transition-all peer-checked:bg-accent"></div>
              </label>
            </div>
          </div>
        </div>

        <div v-if="filteredProviders.length === 0" class="text-center py-6 text-gray-500 dark:text-gray-400 text-sm">
          {{ t('providerPool.noProviders') }}
        </div>
      </div>

      <!-- Provider Details -->
      <div>
        <div v-if="currentTabSelectedProvider" class="bg-gray-50 dark:bg-slate-800/50 rounded-lg border border-gray-200 dark:border-slate-700 p-4">
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
                  @click="openIconUpload"
                  class="absolute inset-0 flex items-center justify-center bg-black/50 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                  :title="t('providerPool.changeIcon')"
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
                <h2 class="font-bold text-gray-900 dark:text-white">{{ currentTabSelectedProvider.name }}</h2>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ currentTabSelectedProvider.description || currentTabSelectedProvider.base_url }}</p>
                <!-- Location Toggle -->
                <div class="flex items-center gap-1 mt-1">
                  <span class="text-xs text-gray-400">{{ t('providerPool.location') }}:</span>
                  <button
                    @click="updateProviderLocation(currentTabSelectedProvider!.id, 'cloud')"
                    :class="[
                      'px-1.5 py-0.5 rounded text-[10px] transition-colors',
                      currentTabSelectedProvider.location === 'cloud'
                        ? 'bg-blue-500 text-white'
                        : 'bg-gray-200 dark:bg-slate-600 text-gray-600 dark:text-gray-400 hover:bg-gray-300 dark:hover:bg-slate-500'
                    ]"
                  >
                    ☁️ {{ t('providerPool.locationCloud') }}
                  </button>
                  <button
                    @click="updateProviderLocation(currentTabSelectedProvider!.id, 'local')"
                    :class="[
                      'px-1.5 py-0.5 rounded text-[10px] transition-colors',
                      currentTabSelectedProvider.location === 'local'
                        ? 'bg-green-500 text-white'
                        : 'bg-gray-200 dark:bg-slate-600 text-gray-600 dark:text-gray-400 hover:bg-gray-300 dark:hover:bg-slate-500'
                    ]"
                  >
                    💻 {{ t('providerPool.locationLocal') }}
                  </button>
                </div>
              </div>
            </div>
            <div class="flex gap-1">
              <button
                @click="testConnection(currentTabSelectedProvider!.id)"
                :disabled="testingProvider === currentTabSelectedProvider!.id"
                class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs disabled:opacity-50"
              >
                {{ testingProvider === currentTabSelectedProvider!.id ? t('common.testing') : t('providerPool.test') }}
              </button>
              <button
                @click="refreshModels(currentTabSelectedProvider!.id)"
                :disabled="refreshingModels === currentTabSelectedProvider!.id"
                class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs disabled:opacity-50"
              >
                {{ refreshingModels === currentTabSelectedProvider!.id ? t('common.refreshing') : t('providerPool.refreshModels') }}
              </button>
              <button
                v-if="currentTabSelectedProvider!.type === 'custom'"
                @click="deleteProvider(currentTabSelectedProvider!.id)"
                class="px-2 py-1 bg-red-500 hover:bg-red-600 text-white rounded text-xs"
              >
                {{ t('common.delete') }}
              </button>
            </div>
          </div>

          <!-- Model Parameters Section -->
          <div class="mb-4">
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('providerPool.modelParams') }}</h3>
              <div class="flex gap-1">
                <button
                  @click="detectCapabilities"
                  :disabled="detectingCapabilities === currentTabSelectedProvider!.id"
                  class="px-2 py-1 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded text-xs disabled:opacity-50"
                >
                  {{ detectingCapabilities === currentTabSelectedProvider!.id ? t('providerPool.detecting') : t('providerPool.detectCapabilities') }}
                </button>
                <button
                  @click="openParamsModal"
                  class="px-2 py-1 bg-accent hover:bg-accent-hover text-white rounded text-xs"
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
                  class="text-accent ml-1"
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
              <button
                @click="openKeyModal(currentTabSelectedProvider!.id)"
                class="px-2 py-1 bg-accent hover:bg-accent-hover text-white rounded text-xs"
              >
                + {{ t('providerPool.addKey') }}
              </button>
            </div>
            <div class="space-y-1">
              <div
                v-for="key in currentTabSelectedProvider!.api_keys"
                :key="key.id"
                class="flex items-center justify-between p-2 bg-white dark:bg-slate-900/50 rounded text-xs"
              >
                <div>
                  <span class="text-gray-700 dark:text-white font-mono">{{ key.key_hash }}</span>
                  <span v-if="key.label" class="ml-2 text-gray-500">({{ key.label }})</span>
                </div>
                <div class="flex items-center gap-3">
                  <span class="text-gray-500">
                    {{ t('providerPool.usageCount') }}: {{ key.usage_count }}
                  </span>
                  <button
                    @click="removeAPIKey(currentTabSelectedProvider!.id, key.id)"
                    class="text-red-400 hover:text-red-300"
                  >
                    ✕
                  </button>
                </div>
              </div>
              <div v-if="!currentTabSelectedProvider!.api_keys?.length" class="text-gray-500 dark:text-gray-400 text-center py-2 text-xs">
                {{ t('providerPool.noKeys') }}
              </div>
            </div>
          </div>

          <!-- Models Section with Pricing -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('providerPool.models') }}
                <span class="text-gray-500 text-xs ml-1">({{ selectedProviderModels.length }})</span>
              </h3>
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
                      :class="hasCustomPricing(model.id) ? 'text-accent' : 'text-green-500'"
                      class="whitespace-nowrap cursor-pointer hover:underline"
                      :title="hasCustomPricing(model.id) ? t('providerPool.customPricing') : t('providerPool.defaultPricing')"
                      @click.stop="openPricingModal(model)"
                    >
                      ${{ (getModelCustomPricing(model.id)?.input_price ?? model.input_price ?? 0).toFixed(2) }}/${{ (getModelCustomPricing(model.id)?.output_price ?? model.output_price ?? 0).toFixed(2) }}
                    </span>
                    <!-- Edit pricing button -->
                    <button
                      @click.stop="openPricingModal(model)"
                      class="opacity-0 group-hover:opacity-100 p-1 hover:bg-gray-100 dark:hover:bg-slate-700 rounded transition-opacity"
                      :title="t('providerPool.editPricing')"
                    >
                      <svg class="w-3 h-3 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                      </svg>
                    </button>
                    <!-- Remove custom pricing button -->
                    <button
                      v-if="hasCustomPricing(model.id)"
                      @click.stop="removeCustomPricing(model.id, store.selectedProviderId)"
                      class="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-100 dark:hover:bg-red-900/30 rounded transition-opacity"
                      :title="t('providerPool.resetPricing')"
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

        <div v-else class="bg-gray-50 dark:bg-slate-800/30 rounded-lg border border-gray-200 dark:border-slate-700 p-8 text-center">
          <p class="text-gray-500 dark:text-gray-400 text-sm">{{ t('providerPool.selectProvider') }}</p>
        </div>
      </div>
    </div>

    <!-- Add Provider Modal -->
    <div v-if="showAddModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="bg-white dark:bg-slate-800 rounded-lg p-5 w-full max-w-md mx-4">
        <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-4">{{ t('providerPool.addCustomProvider') }}</h2>
        <form @submit.prevent="addCustomProvider" class="space-y-4">
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.providerName') }}</label>
            <input
              v-model="newProvider.name"
              type="text"
              required
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.baseUrl') }}</label>
            <input
              v-model="newProvider.base_url"
              type="url"
              required
              placeholder="https://api.example.com/v1"
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.location') }}</label>
            <div class="flex gap-2">
              <button
                type="button"
                @click="newProvider.location = 'cloud'"
                :class="[
                  'flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg border transition-colors',
                  newProvider.location === 'cloud'
                    ? 'bg-blue-500/20 border-blue-500 text-blue-500'
                    : 'bg-gray-100 dark:bg-slate-700 border-gray-200 dark:border-slate-600 text-gray-600 dark:text-gray-400 hover:border-gray-300 dark:hover:border-slate-500'
                ]"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
                </svg>
                {{ t('providerPool.locationCloud') }}
              </button>
              <button
                type="button"
                @click="newProvider.location = 'local'"
                :class="[
                  'flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg border transition-colors',
                  newProvider.location === 'local'
                    ? 'bg-green-500/20 border-green-500 text-green-500'
                    : 'bg-gray-100 dark:bg-slate-700 border-gray-200 dark:border-slate-600 text-gray-600 dark:text-gray-400 hover:border-gray-300 dark:hover:border-slate-500'
                ]"
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
              @click="showAddModal = false"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg"
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
        <form @submit.prevent="addAPIKey" class="space-y-4">
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.apiKey') }}</label>
            <input
              v-model="newKey.key"
              type="password"
              required
              placeholder="sk-..."
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.keyLabel') }} ({{ t('common.optional') }})</label>
            <input
              v-model="newKey.label"
              type="text"
              placeholder="Primary, Backup, etc."
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
            />
          </div>
          <div class="flex justify-end gap-3 mt-6">
            <button
              type="button"
              @click="showKeyModal = false"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg"
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
        <form @submit.prevent="saveModelPricing" class="space-y-4">
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.modelId') }}</label>
            <input
              v-model="pricingForm.modelId"
              type="text"
              required
              placeholder="gpt-4, claude-3-opus, etc."
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">Provider ID ({{ t('common.optional') }})</label>
            <input
              v-model="pricingForm.providerId"
              type="text"
              placeholder="openai, anthropic, etc."
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
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
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
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
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
              />
            </div>
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.cachePrice') }}</label>
              <input
                v-model.number="pricingForm.cachePrice"
                type="number"
                step="0.01"
                min="0"
                class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
              />
            </div>
          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('providerPool.perMillionTokens') }}</p>
          <div class="flex justify-end gap-3 mt-6">
            <button
              type="button"
              @click="showPricingModal = false"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg"
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
        <form @submit.prevent="saveModelParams" class="space-y-4">
          <div>
            <label class="block text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('providerPool.temperature') }}</label>
            <input
              v-model.number="paramsForm.temperature"
              type="number"
              step="0.1"
              min="0"
              max="2"
              :placeholder="t('providerPool.defaultPlaceholder')"
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
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
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
            />
            <p class="text-xs text-gray-400 mt-1">
              {{ t('providerPool.maxTokensHint') }}
              <span v-if="currentTabSelectedProvider?.model_params?.detected_max_tokens" class="text-accent">
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
              class="w-full px-3 py-2 bg-gray-100 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-accent"
            />
            <p class="text-xs text-gray-400 mt-1">{{ t('providerPool.topPHint') }}</p>
          </div>
          <div class="flex justify-end gap-3 mt-6">
            <button
              type="button"
              @click="showParamsModal = false"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg"
            >
              {{ t('common.save') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
