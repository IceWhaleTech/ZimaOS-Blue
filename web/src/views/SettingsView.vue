<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { providerSettingsApi } from '@/api/providers'
import type { ProviderConfigResponse } from '@/api/providers'
import type { LocaleKey } from '@/i18n'
import ClaudeCodeSettings from '@/components/ClaudeCodeSettings.vue'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()

const showApiKey = ref<Record<string, boolean>>({})
const saveStatus = ref<string | null>(null)

// Provider settings state
const providerLoading = ref(false)
const providerSaving = ref(false)
const providerTesting = ref(false)
const providerConfigs = ref<ProviderConfigResponse[]>([])
const providerError = ref<string | null>(null)
const apiKey = ref('')
const baseUrl = ref('')

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
}

// Timezone
const detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const selectedTimezone = ref(localStorage.getItem('zimaos-echo-timezone') || detectedTimezone)

const timezones = computed(() => {
  try {
    const allTimezones = (Intl as unknown as { supportedValuesOf: (key: string) => string[] }).supportedValuesOf('timeZone')
    const filtered = allTimezones.filter((tz: string) => tz !== detectedTimezone)
    return [detectedTimezone, ...filtered]
  } catch {
    // Fallback for browsers that don't support supportedValuesOf
    return [detectedTimezone, 'UTC', 'America/New_York', 'America/Los_Angeles', 'Europe/London', 'Europe/Paris', 'Asia/Tokyo', 'Asia/Shanghai']
  }
})

const temperatureDisplay = computed(() => settingsStore.temperature.toFixed(1))

// Computed
const isCustomProvider = computed(() => settingsStore.selectedProvider === 'custom')

const currentProviderConfig = computed(() => {
  return providerConfigs.value.find(p => p.name === settingsStore.selectedProvider)
})

const currentMeta = computed(() => {
  return providerMeta[settingsStore.selectedProvider] || { requiresApiKey: true, defaultUrl: '', description: '' }
})

const hasProviderChanges = computed(() => {
  if (!currentProviderConfig.value) return false
  const provider = currentProviderConfig.value
  const hasApiKeyChange = apiKey.value !== '' // New API key entered
  const hasBaseUrlChange = baseUrl.value !== (provider.base_url || currentMeta.value.defaultUrl)
  return hasApiKeyChange || hasBaseUrlChange
})

// Watch for provider selection changes to load config
watch(() => settingsStore.selectedProvider, (newProvider) => {
  if (newProvider) {
    loadProviderConfig(newProvider)
  }
})

// Get translated provider name
function getProviderDisplayName(providerName: string): string {
  const key = `settings.providers.${providerName.toLowerCase()}`
  const translated = t(key)
  // If translation key doesn't exist, return original name
  return translated === key ? providerName : translated
}

function toggleShowApiKey(provider: string) {
  showApiKey.value = {
    ...showApiKey.value,
    [provider]: !showApiKey.value[provider],
  }
}

function showSaveStatus(message: string) {
  saveStatus.value = message
  setTimeout(() => {
    saveStatus.value = null
  }, 2000)
}

function getApiKeyMask(provider: string): string {
  const config = providerConfigs.value.find(p => p.name === provider)
  if (!config?.api_key) return ''
  const key = config.api_key
  if (key.length <= 8) return '••••••••'
  return key.slice(0, 4) + '••••••••' + key.slice(-4)
}

async function handleLocaleChange(locale: string) {
  await localeStore.changeLocale(locale as LocaleKey)
  showSaveStatus(t('settings.languageSaved'))
}

function handleTimezoneChange(timezone: string) {
  selectedTimezone.value = timezone
  localStorage.setItem('zimaos-echo-timezone', timezone)
  showSaveStatus(t('settings.timezoneSaved'))
}

// Refresh models for current provider
async function handleRefreshModels() {
  try {
    await settingsStore.refreshProviderModels()
    showSaveStatus(t('settings.modelsRefreshed'))
  } catch {
    // Error is handled in the store
  }
}

// Provider settings functions
async function loadProviderConfigs() {
  try {
    providerLoading.value = true
    providerError.value = null
    const response = await providerSettingsApi.list()
    providerConfigs.value = response.data
    // Load config for current provider
    if (settingsStore.selectedProvider) {
      loadProviderConfig(settingsStore.selectedProvider)
    }
  } catch (e) {
    providerError.value = t('providerSettings.loadError')
    console.error('Failed to load provider configs:', e)
  } finally {
    providerLoading.value = false
  }
}

function loadProviderConfig(providerName: string) {
  const provider = providerConfigs.value.find(p => p.name === providerName)
  if (provider) {
    apiKey.value = '' // Always clear API key input
    baseUrl.value = provider.base_url || providerMeta[providerName]?.defaultUrl || ''
    showApiKey.value = { ...showApiKey.value, [providerName]: false }
  }
}

async function saveProviderConfig() {
  if (!settingsStore.selectedProvider) return

  try {
    providerSaving.value = true
    providerError.value = null

    const config: { api_key?: string; base_url?: string } = {}
    if (apiKey.value) {
      config.api_key = apiKey.value
    }
    if (baseUrl.value) {
      config.base_url = baseUrl.value
    }

    const response = await providerSettingsApi.update(settingsStore.selectedProvider, config)

    // Update local state
    const index = providerConfigs.value.findIndex(p => p.name === settingsStore.selectedProvider)
    if (index !== -1) {
      providerConfigs.value[index] = response.data
    }

    // Clear API key input after save
    apiKey.value = ''
    showSaveStatus(t('providerSettings.saved'))

    // Auto-refresh models after saving config (especially for custom providers)
    try {
      await settingsStore.refreshProviderModels()
    } catch {
      // Ignore refresh errors, config was saved successfully
    }
  } catch (e) {
    providerError.value = t('providerSettings.saveError')
    showSaveStatus(t('providerSettings.saveError'))
    console.error('Failed to save provider config:', e)
  } finally {
    providerSaving.value = false
  }
}

async function testProviderConnection() {
  if (!settingsStore.selectedProvider) return

  try {
    providerTesting.value = true
    providerError.value = null

    const response = await providerSettingsApi.test(settingsStore.selectedProvider)
    console.log('Test response:', response.data)
    if (response.data.success) {
      const messageKey = response.data.messageKey || 'testSuccess'
      const message = t(`providerSettings.${messageKey}`)
      console.log('Success message:', messageKey, '->', message)
      showSaveStatus(message)
    } else {
      const messageKey = response.data.messageKey
      const message = messageKey ? t(`providerSettings.${messageKey}`) : t('providerSettings.testFailed')
      console.log('Failed message:', messageKey, '->', message)
      showSaveStatus(message)
    }
  } catch (e) {
    providerError.value = t('providerSettings.testError')
    showSaveStatus(t('providerSettings.testError'))
    console.error('Failed to test connection:', e)
  } finally {
    providerTesting.value = false
  }
}

async function clearProviderApiKey() {
  if (!settingsStore.selectedProvider) return

  apiKey.value = ''
  try {
    await providerSettingsApi.update(settingsStore.selectedProvider, { api_key: '' })
    await loadProviderConfigs()
    showSaveStatus(t('settings.apiKeyCleared'))
  } catch (e) {
    console.error('Failed to clear API key:', e)
  }
}

onMounted(async () => {
  await settingsStore.fetchProviders()
  await loadProviderConfigs()
})
</script>

<template>
  <div class="settings-view p-4 sm:p-6 max-w-4xl mx-auto">
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('settings.title') }}</h1>

    <!-- Save status notification -->
    <Transition name="notification">
      <div
        v-if="saveStatus"
        class="fixed top-20 right-4 bg-green-600 text-white px-4 py-3 rounded-lg shadow-xl z-[9999]"
      >
        {{ saveStatus }}
      </div>
    </Transition>

    <!-- General Settings -->
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
            d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
          />
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
          />
        </svg>
        <span class="truncate">{{ t('settings.general') }}</span>
      </h2>

      <div class="glass-card p-4 space-y-4">
        <!-- Language -->
        <div>
          <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('common.language') }}</label>
          <select
            :value="localeStore.currentLocale"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
            @change="handleLocaleChange(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="option in localeStore.options"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
        </div>

        <!-- Timezone -->
        <div>
          <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.timezone') }}</label>
          <select
            :value="selectedTimezone"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
            @change="handleTimezoneChange(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="tz in timezones"
              :key="tz"
              :value="tz"
            >
              {{ tz }}
            </option>
          </select>
        </div>

        <!-- Theme -->
        <div>
          <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('common.theme') }}</label>
          <div class="flex gap-2">
            <button
              :class="[
                'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                themeStore.theme === 'light'
                  ? 'bg-accent text-white'
                  : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
              ]"
              @click="themeStore.setTheme('light')"
            >
              {{ t('common.light') }}
            </button>
            <button
              :class="[
                'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                themeStore.theme === 'dark'
                  ? 'bg-accent text-white'
                  : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
              ]"
              @click="themeStore.setTheme('dark')"
            >
              {{ t('common.dark') }}
            </button>
            <button
              :class="[
                'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                themeStore.theme === 'system'
                  ? 'bg-accent text-white'
                  : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
              ]"
              @click="themeStore.setTheme('system')"
            >
              {{ t('common.system') }}
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- LLM Provider Settings (Unified) -->
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
            d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
          />
        </svg>
        <span class="truncate">{{ t('settings.llmProvider') }}</span>
      </h2>

      <div class="glass-card p-4 space-y-4">
        <!-- Provider selection -->
        <div>
          <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.provider') }}</label>
          <select
            :value="settingsStore.selectedProvider"
            data-form-filler-ignore="true"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
            @change="settingsStore.setProvider(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="provider in settingsStore.providers"
              :key="provider.name"
              :value="provider.name"
            >
              {{ getProviderDisplayName(provider.name) }}
            </option>
          </select>
        </div>

        <!-- Model selection (for non-custom providers, show before config) -->
        <div v-if="!isCustomProvider">
          <div class="flex items-center justify-between mb-2">
            <label class="text-sm text-gray-500 dark:text-slate-400">{{ t('settings.model') }}</label>
            <button
              class="text-xs text-accent hover:text-accent-light transition-colors cursor-pointer flex items-center gap-1"
              :disabled="settingsStore.refreshing"
              @click="handleRefreshModels"
            >
              <svg
                class="w-3 h-3"
                :class="{ 'animate-spin': settingsStore.refreshing }"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              {{ t('chat.refreshModels') }}
            </button>
          </div>
          <select
            v-if="settingsStore.availableModels.length > 0"
            :value="settingsStore.selectedModel"
            data-form-filler-ignore="true"
            class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
            @change="settingsStore.setModel(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="model in settingsStore.availableModels"
              :key="model"
              :value="model"
            >
              {{ model }}
            </option>
          </select>
          <p v-else class="text-sm text-yellow-600 dark:text-yellow-400 italic">
            {{ t('providerSettings.noModelsAvailable') }}
          </p>
        </div>

        <!-- Provider status -->
        <div v-if="currentProviderConfig" class="flex items-center gap-2">
          <span
            v-if="currentProviderConfig.has_api_key || !currentMeta.requiresApiKey"
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

        <!-- Dynamic Provider Configuration -->
        <div v-if="currentProviderConfig" class="space-y-4 pt-2 border-t border-gray-200 dark:border-slate-600">
          <!-- Base URL -->
          <div>
            <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">
              {{ t('settings.baseUrl') }}
            </label>
            <input
              v-model="baseUrl"
              type="text"
              name="base_url"
              :placeholder="currentMeta.defaultUrl || t('settings.baseUrlPlaceholder')"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
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
            <div v-if="currentProviderConfig.has_api_key" class="flex items-center gap-2 mb-2">
              <input
                :type="showApiKey[settingsStore.selectedProvider] ? 'text' : 'password'"
                :value="showApiKey[settingsStore.selectedProvider] ? currentProviderConfig.api_key : getApiKeyMask(settingsStore.selectedProvider)"
                readonly
                class="flex-1 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-200 dark:border-slate-600"
              />
              <button
                class="p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white"
                @click="toggleShowApiKey(settingsStore.selectedProvider)"
              >
                <svg v-if="showApiKey[settingsStore.selectedProvider]" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                </svg>
                <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
              </button>
              <button
                class="p-2 text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300"
                :title="t('settings.clearApiKey')"
                @click="clearProviderApiKey"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>

            <!-- New key input -->
            <div class="relative">
              <input
                v-model="apiKey"
                name="api_key"
                :type="showApiKey[settingsStore.selectedProvider] ? 'text' : 'password'"
                :placeholder="currentProviderConfig.has_api_key ? t('settings.enterNewApiKey') : t('settings.enterApiKey')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 pr-10 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
              />
              <button
                type="button"
                class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                @click="showApiKey[settingsStore.selectedProvider] = !showApiKey[settingsStore.selectedProvider]"
              >
                <!-- Eye icon (visible) -->
                <svg v-if="showApiKey[settingsStore.selectedProvider]" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
                <!-- Eye-off icon (hidden) -->
                <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                </svg>
              </button>
            </div>
          </div>

          <!-- Action buttons -->
          <div class="flex items-center gap-3 pt-2">
            <button
              :disabled="!hasProviderChanges || providerSaving"
              class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              @click="saveProviderConfig"
            >
              <span v-if="providerSaving" class="animate-spin rounded-full h-4 w-4 border-b-2 border-white" />
              {{ providerSaving ? t('common.saving') : t('common.save') }}
            </button>
            <button
              :disabled="providerTesting || (!currentProviderConfig.has_api_key && currentMeta.requiresApiKey)"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              @click="testProviderConnection"
            >
              <span v-if="providerTesting" class="animate-spin rounded-full h-4 w-4 border-b-2 border-current" />
              {{ providerTesting ? t('providerSettings.testing') : t('providerSettings.testConnection') }}
            </button>
          </div>

          <!-- Model selection (for custom provider, show after config is saved) -->
          <div v-if="isCustomProvider && currentProviderConfig.has_api_key" class="pt-4 border-t border-gray-200 dark:border-slate-600">
            <div class="flex items-center justify-between mb-2">
              <label class="text-sm text-gray-500 dark:text-slate-400">{{ t('settings.model') }}</label>
              <button
                class="text-xs text-accent hover:text-accent-light transition-colors cursor-pointer flex items-center gap-1"
                :disabled="settingsStore.refreshing"
                @click="handleRefreshModels"
              >
                <svg
                  class="w-3 h-3"
                  :class="{ 'animate-spin': settingsStore.refreshing }"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                {{ t('chat.refreshModels') }}
              </button>
            </div>
            <select
              :value="settingsStore.selectedModel"
              data-form-filler-ignore="true"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
              @change="settingsStore.setModel(($event.target as HTMLSelectElement).value)"
            >
              <option
                v-for="model in settingsStore.availableModels"
                :key="model"
                :value="model"
              >
                {{ model }}
              </option>
            </select>
            <p v-if="settingsStore.availableModels.length === 1 && settingsStore.availableModels[0] === 'default'" class="text-xs text-yellow-600 dark:text-yellow-400 mt-2">
              {{ t('providerSettings.clickRefreshToLoadModels') }}
            </p>
          </div>

          <!-- Hint for custom provider without API key -->
          <div v-if="isCustomProvider && !currentProviderConfig.has_api_key" class="pt-4 border-t border-gray-200 dark:border-slate-600">
            <p class="text-sm text-gray-500 dark:text-slate-400 italic">
              {{ t('providerSettings.saveConfigToSelectModel') }}
            </p>
          </div>
        </div>
      </div>
    </section>

    <!-- Claude Code CLI Settings -->
    <section class="mb-6 sm:mb-8">
      <ClaudeCodeSettings @status-change="showSaveStatus" />
    </section>

    <!-- Model Parameters -->
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
            d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
          />
        </svg>
        <span class="truncate">{{ t('settings.modelParameters') }}</span>
      </h2>

      <div class="glass-card p-3 sm:p-4 space-y-4 sm:space-y-6">
        <!-- Temperature -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <label class="text-sm text-gray-500 dark:text-slate-400">{{ t('settings.temperature') }}</label>
            <span class="text-sm text-gray-900 dark:text-white font-mono">{{ temperatureDisplay }}</span>
          </div>
          <input
            type="range"
            :value="settingsStore.temperature"
            min="0"
            max="2"
            step="0.1"
            class="w-full h-2 bg-gray-200 dark:bg-slate-700 rounded-lg appearance-none cursor-pointer"
            @input="settingsStore.setTemperature(parseFloat(($event.target as HTMLInputElement).value))"
          />
          <div class="flex justify-between text-xs text-gray-400 dark:text-slate-500 mt-1">
            <span>{{ t('settings.precise') }}</span>
            <span>{{ t('settings.creative') }}</span>
          </div>
        </div>

        <!-- Max Tokens -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <label class="text-sm text-gray-500 dark:text-slate-400">{{ t('settings.maxTokens') }}</label>
            <span class="text-sm text-gray-900 dark:text-white font-mono">{{ settingsStore.maxTokens }}</span>
          </div>
          <input
            type="range"
            :value="settingsStore.maxTokens"
            min="256"
            max="8192"
            step="256"
            class="w-full h-2 bg-gray-200 dark:bg-slate-700 rounded-lg appearance-none cursor-pointer"
            @input="settingsStore.setMaxTokens(parseInt(($event.target as HTMLInputElement).value))"
          />
          <div class="flex justify-between text-xs text-gray-400 dark:text-slate-500 mt-1">
            <span>256</span>
            <span>8192</span>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
input[type='range'] {
  -webkit-appearance: none;
}

input[type='range']::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  background: var(--color-accent, #3b82f6);
  border-radius: 50%;
  cursor: pointer;
}

input[type='range']::-moz-range-thumb {
  width: 16px;
  height: 16px;
  background: var(--color-accent, #3b82f6);
  border-radius: 50%;
  cursor: pointer;
  border: none;
}

/* Notification transition */
.notification-enter-active,
.notification-leave-active {
  transition: all 0.3s ease;
}

.notification-enter-from {
  opacity: 0;
  transform: translateX(100px);
}

.notification-leave-to {
  opacity: 0;
  transform: translateX(100px);
}
</style>
