<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import type { LocaleKey } from '@/i18n'
import ClaudeCodeSettings from '@/components/ClaudeCodeSettings.vue'
import ProviderPoolSection from '@/components/ProviderPoolSection.vue'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()

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

const temperatureDisplay = computed(() => settingsStore.temperature.toFixed(1))

function showSaveStatus(message: string) {
  saveStatus.value = message
  setTimeout(() => {
    saveStatus.value = null
  }, 2000)
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
              {{ t('localeNames.' + option.value) || option.label }}
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

    <!-- Provider Pool -->
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
            d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
          />
        </svg>
        <span class="truncate">{{ t('providerPool.title') }}</span>
      </h2>

      <div class="glass-card p-4">
        <ProviderPoolSection />
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
