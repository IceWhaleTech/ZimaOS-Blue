<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import type { LocaleKey } from '@/i18n'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()

const showApiKey = ref<Record<string, boolean>>({})
const tempApiKeys = ref<Record<string, string>>({})
const tempBaseUrls = ref<Record<string, string>>({})
const saveStatus = ref<string | null>(null)

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

// Providers that support custom base URL
const providersWithBaseUrl = ['ollama', 'custom']

// Default base URLs for providers
const defaultBaseUrls: Record<string, string> = {
  ollama: 'http://localhost:11434',
  openai: 'https://api.openai.com/v1',
  custom: '',
}

const temperatureDisplay = computed(() => settingsStore.temperature.toFixed(1))

// Separate standard providers from custom provider
const standardProviders = computed(() =>
  settingsStore.providers.filter(p => p.name.toLowerCase() !== 'custom')
)

const customProvider = computed(() =>
  settingsStore.providers.find(p => p.name.toLowerCase() === 'custom')
)

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

function handleApiKeyInput(provider: string, value: string) {
  tempApiKeys.value = { ...tempApiKeys.value, [provider]: value }
}

function saveApiKey(provider: string) {
  const key = tempApiKeys.value[provider]
  if (key) {
    settingsStore.setApiKey(provider, key)
    tempApiKeys.value = { ...tempApiKeys.value, [provider]: '' }
    showSaveStatus(t('settings.apiKeySaved'))
  }
}

function clearApiKey(provider: string) {
  settingsStore.clearApiKey(provider)
  showSaveStatus(t('settings.apiKeyCleared'))
}

function showSaveStatus(message: string) {
  saveStatus.value = message
  setTimeout(() => {
    saveStatus.value = null
  }, 2000)
}

function getApiKeyMask(provider: string): string {
  const key = settingsStore.apiKeys[provider]
  if (!key) return ''
  if (key.length <= 8) return '••••••••'
  return key.slice(0, 4) + '••••••••' + key.slice(-4)
}

function getBaseUrl(provider: string): string {
  return settingsStore.baseUrls[provider] || defaultBaseUrls[provider] || ''
}

function handleBaseUrlChange(provider: string, url: string) {
  settingsStore.setBaseUrl(provider, url)
  showSaveStatus(t('settings.baseUrlSaved'))
}

function supportsBaseUrl(providerName: string): boolean {
  return providersWithBaseUrl.includes(providerName.toLowerCase())
}

function requiresApiKey(providerName: string): boolean {
  return providerName.toLowerCase() !== 'ollama'
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

onMounted(async () => {
  await settingsStore.fetchProviders()
})
</script>

<template>
  <div class="settings-view p-4 sm:p-6 max-w-4xl mx-auto">
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('settings.title') }}</h1>

    <!-- Save status notification -->
    <div
      v-if="saveStatus"
      class="fixed top-20 right-4 bg-green-600 text-white px-4 py-2 rounded-lg shadow-lg z-50"
    >
      {{ saveStatus }}
    </div>

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

    <!-- LLM Provider Settings -->
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

        <!-- Model selection -->
        <div>
          <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.model') }}</label>
          <select
            :value="settingsStore.selectedModel"
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
        </div>
      </div>
    </section>

    <!-- API Keys & Provider Configuration -->
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
        <span class="truncate">{{ t('settings.providerConfiguration') }}</span>
      </h2>

      <!-- Standard Providers Grid -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 sm:gap-4 mb-4">
        <div
          v-for="provider in standardProviders"
          :key="provider.name"
          class="glass-card p-3 sm:p-4"
        >
          <!-- Provider header with title and status on separate lines on mobile -->
          <div class="mb-3">
            <h3 class="text-gray-900 dark:text-white font-medium text-sm sm:text-base mb-2">
              {{ getProviderDisplayName(provider.name) }}
            </h3>
            <span
              v-if="!requiresApiKey(provider.name)"
              class="inline-block text-xs text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/50 px-2 py-1 rounded"
            >
              {{ t('settings.noApiKeyRequired') }}
            </span>
            <span
              v-else-if="settingsStore.apiKeys[provider.name]"
              class="inline-block text-xs text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/50 px-2 py-1 rounded"
            >
              {{ t('settings.configured') }}
            </span>
            <span
              v-else
              class="inline-block text-xs text-yellow-600 dark:text-yellow-400 bg-yellow-100 dark:bg-yellow-900/50 px-2 py-1 rounded"
            >
              {{ t('settings.notConfigured') }}
            </span>
          </div>

          <!-- API Key section (for providers that require it) -->
          <div v-if="requiresApiKey(provider.name)" class="space-y-2 sm:space-y-3 mb-3 sm:mb-4">
            <label class="block text-xs sm:text-sm text-gray-500 dark:text-slate-400">{{ t('settings.apiKey') }}</label>
            <!-- Current key display -->
            <div v-if="settingsStore.apiKeys[provider.name]" class="flex items-center gap-1 sm:gap-2">
              <input
                :type="showApiKey[provider.name] ? 'text' : 'password'"
                :value="showApiKey[provider.name] ? settingsStore.apiKeys[provider.name] : getApiKeyMask(provider.name)"
                readonly
                class="flex-1 min-w-0 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm border border-gray-200 dark:border-slate-600"
              />
              <button
                class="p-1.5 sm:p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white flex-shrink-0"
                @click="toggleShowApiKey(provider.name)"
              >
                <svg
                  v-if="showApiKey[provider.name]"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 sm:h-5 sm:w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                  />
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 sm:h-5 sm:w-5"
                  fill="none"
                  viewBox="0 0 24 24"
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
                    d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                  />
                </svg>
              </button>
              <button
                class="p-1.5 sm:p-2 text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300 flex-shrink-0"
                :title="t('settings.clearApiKey')"
                @click="clearApiKey(provider.name)"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 sm:h-5 sm:w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
              </button>
            </div>

            <!-- New key input -->
            <div class="flex items-center gap-1 sm:gap-2">
              <input
                type="password"
                :value="tempApiKeys[provider.name] || ''"
                :placeholder="settingsStore.apiKeys[provider.name] ? t('settings.enterNewApiKey') : t('settings.enterApiKey')"
                class="flex-1 min-w-0 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
                @input="handleApiKeyInput(provider.name, ($event.target as HTMLInputElement).value)"
              />
              <button
                :disabled="!tempApiKeys[provider.name]"
                class="px-2 sm:px-4 py-1.5 sm:py-2 bg-accent hover:bg-accent-hover text-white rounded text-xs sm:text-sm disabled:opacity-50 disabled:cursor-not-allowed flex-shrink-0"
                @click="saveApiKey(provider.name)"
              >
                {{ t('common.save') }}
              </button>
            </div>
          </div>

          <!-- No API key message for Ollama -->
          <p v-else class="text-xs sm:text-sm text-gray-500 dark:text-slate-400 mb-2 sm:mb-3">
            {{ t('settings.ollamaNoKey') }}
          </p>

          <!-- Base URL configuration (for providers that support it) -->
          <div v-if="supportsBaseUrl(provider.name)" class="space-y-1.5 sm:space-y-2">
            <label class="block text-xs sm:text-sm text-gray-500 dark:text-slate-400">{{ t('settings.baseUrl') }}</label>
            <input
              type="text"
              :value="getBaseUrl(provider.name)"
              :placeholder="defaultBaseUrls[provider.name.toLowerCase()] || t('settings.baseUrlPlaceholder')"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
              @change="handleBaseUrlChange(provider.name, ($event.target as HTMLInputElement).value)"
            />
            <p class="text-xs text-gray-400 dark:text-slate-500">
              {{ provider.name.toLowerCase() === 'ollama' ? t('settings.ollamaBaseUrlHint') : t('settings.customBaseUrlHint') }}
            </p>
          </div>
        </div>
      </div>

      <!-- Custom OpenAI-Compatible Provider (Full Width) -->
      <div v-if="customProvider" class="glass-card p-3 sm:p-4">
        <div class="mb-3">
          <h3 class="text-gray-900 dark:text-white font-medium text-sm sm:text-base mb-2">
            {{ getProviderDisplayName(customProvider.name) }}
          </h3>
          <span
            v-if="settingsStore.apiKeys[customProvider.name] && settingsStore.baseUrls[customProvider.name]"
            class="inline-block text-xs text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/50 px-2 py-1 rounded"
          >
            {{ t('settings.configured') }}
          </span>
          <span
            v-else
            class="inline-block text-xs text-yellow-600 dark:text-yellow-400 bg-yellow-100 dark:bg-yellow-900/50 px-2 py-1 rounded"
          >
            {{ t('settings.notConfigured') }}
          </span>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
          <!-- Base URL (Required for custom) -->
          <div class="space-y-1.5 sm:space-y-2">
            <label class="block text-xs sm:text-sm text-gray-500 dark:text-slate-400">{{ t('settings.baseUrl') }} *</label>
            <input
              type="text"
              :value="getBaseUrl(customProvider.name)"
              :placeholder="t('settings.baseUrlPlaceholder')"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
              @change="handleBaseUrlChange(customProvider.name, ($event.target as HTMLInputElement).value)"
            />
            <p class="text-xs text-gray-400 dark:text-slate-500">
              {{ t('settings.customBaseUrlHint') }}
            </p>
          </div>

          <!-- API Key -->
          <div class="space-y-1.5 sm:space-y-2">
            <label class="block text-xs sm:text-sm text-gray-500 dark:text-slate-400">{{ t('settings.apiKey') }}</label>
            <!-- Current key display -->
            <div v-if="settingsStore.apiKeys[customProvider.name]" class="flex items-center gap-1 sm:gap-2">
              <input
                :type="showApiKey[customProvider.name] ? 'text' : 'password'"
                :value="showApiKey[customProvider.name] ? settingsStore.apiKeys[customProvider.name] : getApiKeyMask(customProvider.name)"
                readonly
                class="flex-1 min-w-0 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm border border-gray-200 dark:border-slate-600"
              />
              <button
                class="p-1.5 sm:p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white flex-shrink-0"
                @click="toggleShowApiKey(customProvider.name)"
              >
                <svg
                  v-if="showApiKey[customProvider.name]"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 sm:h-5 sm:w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                  />
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 sm:h-5 sm:w-5"
                  fill="none"
                  viewBox="0 0 24 24"
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
                    d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                  />
                </svg>
              </button>
              <button
                class="p-1.5 sm:p-2 text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300 flex-shrink-0"
                :title="t('settings.clearApiKey')"
                @click="clearApiKey(customProvider.name)"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 sm:h-5 sm:w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
              </button>
            </div>

            <!-- New key input -->
            <div v-else class="flex items-center gap-1 sm:gap-2">
              <input
                type="password"
                :value="tempApiKeys[customProvider.name] || ''"
                :placeholder="t('settings.enterApiKey')"
                class="flex-1 min-w-0 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
                @input="handleApiKeyInput(customProvider.name, ($event.target as HTMLInputElement).value)"
              />
              <button
                :disabled="!tempApiKeys[customProvider.name]"
                class="px-2 sm:px-4 py-1.5 sm:py-2 bg-accent hover:bg-accent-hover text-white rounded text-xs sm:text-sm disabled:opacity-50 disabled:cursor-not-allowed flex-shrink-0"
                @click="saveApiKey(customProvider.name)"
              >
                {{ t('common.save') }}
              </button>
            </div>
          </div>
        </div>
      </div>
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
</style>
