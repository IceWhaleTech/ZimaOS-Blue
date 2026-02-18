<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { providerSettingsApi } from '@/api/providers'
import type { ProviderConfigResponse } from '@/api/providers'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const providers = ref<ProviderConfigResponse[]>([])
const selectedProvider = ref<string>('')
const error = ref<string | null>(null)

// Form state
const apiKey = ref('')
const baseUrl = ref('')
const showApiKey = ref(false)

// Provider metadata
const providerMeta: Record<string, { requiresApiKey: boolean; defaultUrl: string; description: string }> = {
  claude: {
    requiresApiKey: true,
    defaultUrl: 'https://api.anthropic.com',
    description: 'providerSettings.claudeDesc',
  },
  openai: {
    requiresApiKey: true,
    defaultUrl: 'https://api.openai.com',
    description: 'providerSettings.openaiDesc',
  },
  ollama: {
    requiresApiKey: false,
    defaultUrl: 'http://localhost:11434',
    description: 'providerSettings.ollamaDesc',
  },
  custom: {
    requiresApiKey: true,
    defaultUrl: '',
    description: 'providerSettings.customDesc',
  },
  grok: {
    requiresApiKey: true,
    defaultUrl: 'https://api.x.ai',
    description: 'providerSettings.grokDesc',
  },
  qwen: {
    requiresApiKey: true,
    defaultUrl: 'https://dashscope.aliyuncs.com/compatible-mode',
    description: 'providerSettings.qwenDesc',
  },
  siliconflow: {
    requiresApiKey: true,
    defaultUrl: 'https://api.siliconflow.cn/v1',
    description: 'providerSettings.siliconflowDesc',
  },
}

// Computed
const currentProvider = computed(() => {
  return providers.value.find(p => p.name === selectedProvider.value)
})

const currentMeta = computed(() => {
  return providerMeta[selectedProvider.value] || { requiresApiKey: true, defaultUrl: '', description: '' }
})

const hasChanges = computed(() => {
  if (!currentProvider.value) return false
  const provider = currentProvider.value
  const hasApiKeyChange = apiKey.value !== '' // New API key entered
  const hasBaseUrlChange = baseUrl.value !== (provider.base_url || currentMeta.value.defaultUrl)
  return hasApiKeyChange || hasBaseUrlChange
})

// Watch for provider selection changes
watch(selectedProvider, (newProvider) => {
  if (newProvider) {
    loadProviderConfig(newProvider)
  }
})

onMounted(async () => {
  await loadProviders()
})

async function loadProviders() {
  try {
    loading.value = true
    error.value = null
    const response = await providerSettingsApi.list()
    providers.value = response.data
    // Select first provider by default
    const firstProvider = providers.value[0]
    if (firstProvider) {
      selectedProvider.value = firstProvider.name
    }
  } catch (e) {
    error.value = t('providerSettings.loadError')
    console.error('Failed to load providers:', e)
  } finally {
    loading.value = false
  }
}

function loadProviderConfig(providerName: string) {
  const provider = providers.value.find(p => p.name === providerName)
  if (provider) {
    apiKey.value = '' // Always clear API key input
    baseUrl.value = provider.base_url || providerMeta[providerName]?.defaultUrl || ''
    showApiKey.value = false
  }
}

async function saveConfig() {
  if (!selectedProvider.value) return

  try {
    saving.value = true
    error.value = null

    const config: { api_key?: string; base_url?: string } = {}
    if (apiKey.value) {
      config.api_key = apiKey.value
    }
    if (baseUrl.value) {
      config.base_url = baseUrl.value
    }

    const response = await providerSettingsApi.update(selectedProvider.value, config)

    // Update local state
    const index = providers.value.findIndex(p => p.name === selectedProvider.value)
    if (index !== -1) {
      providers.value[index] = response.data
    }

    // Clear API key input after save
    apiKey.value = ''
    emit('status-change', t('providerSettings.saved'))
  } catch (e) {
    error.value = t('providerSettings.saveError')
    emit('status-change', t('providerSettings.saveError'))
    console.error('Failed to save provider config:', e)
  } finally {
    saving.value = false
  }
}

async function testConnection() {
  if (!selectedProvider.value) return

  try {
    testing.value = true
    error.value = null

    const response = await providerSettingsApi.test(selectedProvider.value)
    if (response.data.success) {
      const messageKey = response.data.messageKey || 'testSuccess'
      emit('status-change', t(`providerSettings.${messageKey}`))
    } else {
      const messageKey = response.data.messageKey
      const message = messageKey ? t(`providerSettings.${messageKey}`) : t('providerSettings.testFailed')
      emit('status-change', message)
    }
  } catch (e) {
    error.value = t('providerSettings.testError')
    emit('status-change', t('providerSettings.testError'))
    console.error('Failed to test connection:', e)
  } finally {
    testing.value = false
  }
}

function getProviderDisplayName(name: string): string {
  const key = `settings.providers.${name.toLowerCase()}`
  const translated = t(key)
  return translated === key ? name : translated
}

function toggleShowApiKey() {
  showApiKey.value = !showApiKey.value
}

function clearApiKey() {
  apiKey.value = ''
  // Also clear from backend
  providerSettingsApi.update(selectedProvider.value, { api_key: '' })
    .then(() => {
      loadProviders()
      emit('status-change', t('settings.apiKeyCleared'))
    })
    .catch(e => {
      console.error('Failed to clear API key:', e)
    })
}
</script>

<template>
  <section class="mb-6 sm:mb-8">
    <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white mb-3 sm:mb-4 flex items-center gap-2">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-5 w-5 flex-shrink-0"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
        />
      </svg>
      <span class="truncate">{{ t('providerSettings.title') }}</span>
    </h2>

    <div class="glass-card p-4">
      <!-- Loading state -->
      <div v-if="loading" class="flex items-center justify-center py-8">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-gray-700" />
      </div>

      <!-- Error state -->
      <div v-else-if="error && providers.length === 0" class="text-center py-8">
        <p class="text-red-500 dark:text-red-400 mb-4">{{ error }}</p>
        <button
          class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg"
          @click="loadProviders"
        >
          {{ t('common.retry') }}
        </button>
      </div>

      <!-- Provider configuration -->
      <div v-else class="space-y-4">
        <!-- Provider selector -->
        <div>
          <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">
            {{ t('providerSettings.selectProvider') }}
          </label>
          <select
            v-model="selectedProvider"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
          >
            <option
              v-for="provider in providers"
              :key="provider.name"
              :value="provider.name"
            >
              {{ getProviderDisplayName(provider.name) }}
            </option>
          </select>
        </div>

        <!-- Provider status -->
        <div v-if="currentProvider" class="flex items-center gap-2">
          <span
            v-if="currentProvider.has_api_key || !currentMeta.requiresApiKey"
            class="inline-block text-xs text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/50 px-2 py-1 rounded"
          >
            {{ currentMeta.requiresApiKey ? t('settings.configured') : t('settings.noApiKeyRequired') }}
          </span>
          <span
            v-else
            class="inline-block text-xs text-yellow-600 dark:text-yellow-400 bg-yellow-100 dark:bg-yellow-900/50 px-2 py-1 rounded"
          >
            {{ t('settings.notConfigured') }}
          </span>
        </div>

        <!-- Provider description -->
        <p v-if="currentMeta.description" class="text-sm text-gray-500 dark:text-slate-400">
          {{ t(currentMeta.description) }}
        </p>

        <!-- Configuration fields -->
        <div v-if="currentProvider" class="space-y-4 pt-2 border-t border-gray-200 dark:border-slate-600">
          <!-- Base URL -->
          <div>
            <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">
              {{ t('settings.baseUrl') }}
            </label>
            <input
              v-model="baseUrl"
              type="text"
              :placeholder="currentMeta.defaultUrl || t('settings.baseUrlPlaceholder')"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
            />
            <p class="text-xs text-gray-400 dark:text-slate-500 mt-1">
              {{ t('providerSettings.baseUrlHint', { default: currentMeta.defaultUrl }) }}
            </p>
          </div>

          <!-- API Key (for providers that require it) -->
          <div v-if="currentMeta.requiresApiKey">
            <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">
              {{ t('settings.apiKey') }}
            </label>

            <!-- Current key display -->
            <div v-if="currentProvider.has_api_key" class="flex items-center gap-2 mb-2">
              <input
                :type="showApiKey ? 'text' : 'password'"
                :value="showApiKey ? currentProvider.api_key : '••••••••••••'"
                readonly
                class="flex-1 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-200 dark:border-slate-600"
              />
              <button
                class="p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white"
                @click="toggleShowApiKey"
              >
                <svg
                  v-if="showApiKey"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
              </button>
              <button
                class="p-2 text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300"
                :title="t('settings.clearApiKey')"
                @click="clearApiKey"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>

            <!-- New key input -->
            <input
              v-model="apiKey"
              type="password"
              :placeholder="currentProvider.has_api_key ? t('settings.enterNewApiKey') : t('settings.enterApiKey')"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
            />
          </div>

          <!-- Action buttons -->
          <div class="flex items-center gap-3 pt-2">
            <button
              :disabled="!hasChanges || saving"
              class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              @click="saveConfig"
            >
              <span v-if="saving" class="animate-spin rounded-full h-4 w-4 border-b-2 border-white" />
              {{ saving ? t('common.saving') : t('common.save') }}
            </button>
            <button
              :disabled="testing || (!currentProvider.has_api_key && currentMeta.requiresApiKey)"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              @click="testConnection"
            >
              <span v-if="testing" class="animate-spin rounded-full h-4 w-4 border-b-2 border-current" />
              {{ testing ? t('providerSettings.testing') : t('providerSettings.testConnection') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
