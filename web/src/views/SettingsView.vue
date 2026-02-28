<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settings'
import { THEME_STYLES, type ThemeStyle, type MemoryRecallMode } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { backupApi } from '@/api/index'
import type { LocaleKey } from '@/i18n'
import type { BackupInfo } from '@/api/index'
import ClaudeCodeSettings from '@/components/ClaudeCodeSettings.vue'
import ProviderPoolSection from '@/components/ProviderPoolSection.vue'
import UserDataExport from '@/components/UserDataExport.vue'
import NetworkSettings from '@/components/settings/NetworkSettings.vue'
import SpeechSettings from '@/components/settings/SpeechSettings.vue'
import WorkspaceSettings from '@/components/settings/WorkspaceSettings.vue'
import UpdateSettings from '@/components/settings/UpdateSettings.vue'
import ApiProxySettings from '@/components/settings/ApiProxySettings.vue'
import MemoryManager from '@/components/MemoryManager.vue'
import BackupManager from '@/components/BackupManager.vue'
import { useTauri } from '@/composables/useTauri'
import { serviceApi } from '@/api/service'
import type { ServiceInfo } from '@/api/service'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()
const { isTauri, setCloseBehavior } = useTauri()

const saveStatus = ref<string | null>(null)

// Auto-start state
const serviceInfo = ref<ServiceInfo | null>(null)
const autoStartLoading = ref(false)
const autoStartEnabled = computed(() => serviceInfo.value?.installed && serviceInfo.value?.enabled)

// Active tab - flattened structure
const SETTINGS_TABS = ['general', 'llm', 'proxy', 'speech', 'network', 'memory', 'userdata'] as const
type TabType = typeof SETTINGS_TABS[number]
const initialTab = route.query.tab
const activeTab = ref<TabType>(
  typeof initialTab === 'string' && SETTINGS_TABS.includes(initialTab as TabType)
    ? (initialTab as TabType)
    : 'general'
)


// Tab icons (heroicons outline, 16x16)
const tabIcons: Record<TabType, string> = {
  general: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>',
  llm: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 2a5 5 0 0 0-4.8 3.6A3.5 3.5 0 0 0 4 9a3.5 3.5 0 0 0 1.1 2.5A4 4 0 0 0 4 14a4 4 0 0 0 2.6 3.8C7 19.7 8.8 21 11 21h1V2h-1z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 2a5 5 0 0 1 4.8 3.6A3.5 3.5 0 0 1 20 9a3.5 3.5 0 0 1-1.1 2.5A4 4 0 0 1 20 14a4 4 0 0 1-2.6 3.8C17 19.7 15.2 21 13 21h-1"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 9h4m-4 4h4m4-4h-4m4 4h-4"/>',
  proxy: '<circle cx="12" cy="13" r="9" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" fill="none"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 13l3.5-5"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 4V2m4.24 3.76l1.42-1.42M20 13h2M4 13H2m3.34-7.66L3.93 3.93"/><circle cx="12" cy="13" r="1.5" stroke-width="0" fill="currentColor"/>',
  speech: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"/>',
  network: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"/>',
  memory: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 3h6a2 2 0 012 2v14l-5-3-5 3V5a2 2 0 012-2z"/>',
  userdata: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>',
}

// Timezone
const detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const selectedTimezone = ref(localStorage.getItem('zimaos-blue-timezone') || detectedTimezone)

const timezones = computed(() => {
  try {
    const allTimezones = (Intl as unknown as { supportedValuesOf: (key: string) => string[] }).supportedValuesOf('timeZone')
    const filtered = allTimezones.filter((tz: string) => tz !== detectedTimezone)
    return [detectedTimezone, ...filtered]
  } catch {
    return [detectedTimezone, 'UTC', 'America/New_York', 'America/Los_Angeles', 'Europe/London', 'Europe/Paris', 'Asia/Tokyo', 'Asia/Shanghai']
  }
})

// System Tab - Backup
const backups = ref<BackupInfo[]>([])
const backupsLoading = ref(false)
const backupCreating = ref(false)
const backupRestoring = ref<string | null>(null)

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
  localStorage.setItem('zimaos-blue-timezone', timezone)
  showSaveStatus(t('settings.timezoneSaved'))
}

function handleCloseBehaviorChange(behavior: 'quit' | 'minimize') {
  settingsStore.setCloseBehavior(behavior)
  setCloseBehavior(behavior)
  showSaveStatus(t('settings.closeBehaviorSaved'))
}

function handleThemeStyleChange(style: ThemeStyle) {
  settingsStore.setThemeStyle(style)
}

async function handleMemoryRecallModeChange(mode: MemoryRecallMode) {
  try {
    await settingsStore.setMemoryRecallMode(mode)
    showSaveStatus(t('settings.memoryRecallMode.saved'))
  } catch {
    showSaveStatus(t('settings.memoryRecallMode.saveFailed'))
  }
}

async function fetchServiceInfo() {
  try {
    const res = await serviceApi.getInfo()
    serviceInfo.value = res.data
  } catch { /* service API not available */ }
}

async function toggleAutoStart() {
  autoStartLoading.value = true
  try {
    if (autoStartEnabled.value) {
      await serviceApi.disable()
      await serviceApi.uninstall()
      showSaveStatus(t('service.disableSuccess'))
    } else {
      if (!serviceInfo.value?.installed) await serviceApi.install()
      await serviceApi.enable()
      showSaveStatus(t('service.enableSuccess'))
    }
    await fetchServiceInfo()
  } catch (e) {
    showSaveStatus(autoStartEnabled.value ? t('service.disableFailed') : t('service.enableFailed'))
  } finally {
    autoStartLoading.value = false
  }
}

function switchTab(tab: TabType) {
  activeTab.value = tab
  router.replace({ query: { tab } })

  // Load data for specific tabs
  if (tab === 'userdata' && backups.value.length === 0) {
    fetchBackups()
  }
}

// Backup functions
async function fetchBackups() {
  backupsLoading.value = true
  try {
    const response = await backupApi.list()
    backups.value = response.data || []
  } catch (e) {
    console.error('Failed to fetch backups:', e)
  } finally {
    backupsLoading.value = false
  }
}

async function createBackup() {
  backupCreating.value = true
  try {
    await backupApi.create()
    await fetchBackups()
    showSaveStatus(t('system.backupCreated'))
  } catch (e) {
    console.error('Failed to create backup:', e)
  } finally {
    backupCreating.value = false
  }
}

function onBackupCreate(_type: string, _name: string) {
  createBackup()
}

function onBackupDownload(_id: string) {
  // Download not implemented in API yet
}

async function restoreBackup(id: string) {
  if (!confirm(t('system.confirmRestore'))) return
  backupRestoring.value = id
  try {
    await backupApi.restore(id)
    showSaveStatus(t('system.backupRestored'))
  } catch (e) {
    console.error('Failed to restore backup:', e)
  } finally {
    backupRestoring.value = null
  }
}

async function deleteBackup(id: string) {
  if (!confirm(t('system.confirmDeleteBackup'))) return
  try {
    await backupApi.delete(id)
    backups.value = backups.value.filter(b => b.id !== id)
    showSaveStatus(t('system.backupDeleted'))
  } catch (e) {
    console.error('Failed to delete backup:', e)
  }
}

onMounted(async () => {
  await settingsStore.fetchProviders()
  fetchServiceInfo()

  // Load data based on initial tab
  const tab = route.query.tab
  if (typeof tab === 'string' && SETTINGS_TABS.includes(tab as TabType)) {
    switchTab(tab as TabType)
  }
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

    <!-- Main Tabs -->
    <div class="flex overflow-x-auto border-b border-gray-200 dark:border-gray-700 mb-6 -mx-4 px-4 sm:mx-0 sm:px-0">
      <button
        v-for="tab in SETTINGS_TABS"
        :key="tab"
        class="px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap flex-shrink-0 flex items-center gap-1.5"
        :class="
          activeTab === tab
            ? 'text-gray-900 dark:text-white border-b-2 border-gray-900 dark:border-white'
            : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white'
        "
        @click="switchTab(tab)"
      >
        <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" v-html="tabIcons[tab]" />
        {{ t(`settings.tab.${tab}`) }}
      </button>
    </div>

    <!-- General Tab -->
    <div v-if="activeTab === 'general'" class="space-y-6">
      <!-- Language -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('common.language') }}</label>
        <select
          :value="localeStore.currentLocale"
          class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
          @change="handleLocaleChange(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="option in localeStore.options" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </div>

      <!-- Timezone -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.timezone') }}</label>
        <select
          :value="selectedTimezone"
          class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
          @change="handleTimezoneChange(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="tz in timezones" :key="tz" :value="tz">{{ tz }}</option>
        </select>
      </div>

      <!-- Theme -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('common.theme') }}</label>
        <div class="flex gap-2">
          <button
            v-for="theme in ['light', 'dark', 'system'] as const"
            :key="theme"
            :class="[
              'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
              themeStore.theme === theme
                ? 'bg-gray-700 dark:bg-gray-500 text-white'
                : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
            ]"
            @click="themeStore.setTheme(theme)"
          >
            {{ t(`common.${theme}`) }}
          </button>
        </div>
      </div>

      <!-- Chat Theme Style -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('theme.styles.title') }}</label>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <button
            v-for="style in THEME_STYLES"
            :key="style.id"
            class="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors border"
            :class="settingsStore.themeStyle === style.id
              ? 'bg-gray-100 dark:bg-gray-700/30 border-gray-300 dark:border-gray-500 text-gray-900 dark:text-white'
              : 'bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-700/60'"
            @click="handleThemeStyleChange(style.id)"
          >
            <span class="theme-style-btn-preview flex-shrink-0" :class="`theme-style-btn-${style.id}`" />
            <span class="font-medium">{{ t(style.labelKey) }}</span>
          </button>
        </div>
      </div>

      <!-- Close Behavior (Tauri only) -->
      <div v-if="isTauri" class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.closeBehavior') }}</label>
        <div class="flex gap-2">
          <button
            v-for="behavior in ['quit', 'minimize'] as const"
            :key="behavior"
            :class="[
              'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
              settingsStore.closeBehavior === behavior
                ? 'bg-gray-700 dark:bg-gray-500 text-white'
                : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
            ]"
            @click="handleCloseBehaviorChange(behavior)"
          >
            {{ t(`settings.closeBehavior${behavior === 'quit' ? 'Quit' : 'Minimize'}`) }}
          </button>
        </div>
      </div>

      <!-- Auto-start -->
      <div v-if="serviceInfo" class="glass-card p-4">
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('service.autoStart') }}</h3>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('service.autoStartDescription') }}</p>
          </div>
          <button
            type="button"
            role="switch"
            :aria-checked="autoStartEnabled"
            :disabled="autoStartLoading"
            class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
            :class="autoStartEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
            @click="toggleAutoStart"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="autoStartEnabled ? 'translate-x-5' : 'translate-x-0'"
            />
          </button>
        </div>
      </div>

      <!-- About / Version -->
      <div class="glass-card p-4">
        <UpdateSettings />
      </div>
    </div>

    <!-- LLM Tab -->
    <div v-if="activeTab === 'llm'" class="space-y-6">
      <!-- Provider Pool -->
      <div class="glass-card p-4">
        <ProviderPoolSection />
      </div>

      <!-- Tool Call Approval (hidden for now) -->
      <!-- <ToolApprovalSettings @status-change="showSaveStatus" /> -->

      <!-- Claude Code CLI Settings -->
      <ClaudeCodeSettings @status-change="showSaveStatus" />
    </div>

    <!-- Optimization Tab -->
    <div v-if="activeTab === 'proxy'">
      <ApiProxySettings @status-change="showSaveStatus" />
    </div>

    <!-- Network Tab -->
    <div v-if="activeTab === 'network'">
      <NetworkSettings @status-change="showSaveStatus" />
    </div>

    <!-- Speech Tab -->
    <div v-if="activeTab === 'speech'">
      <SpeechSettings />
    </div>

    <!-- Memory Tab -->
    <div v-if="activeTab === 'memory'" class="space-y-6">
      <!-- Memory Management -->
      <MemoryManager @status-change="showSaveStatus" />

      <!-- Memory Recall Mode -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.memoryRecallMode.title') }}</label>
        <p class="text-xs text-gray-500 dark:text-gray-400 mb-3">{{ t('settings.memoryRecallMode.description') }}</p>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
          <button
            v-for="mode in ['aggressive', 'balanced', 'quality'] as const"
            :key="mode"
            class="w-full px-3 py-2 rounded-lg text-sm transition-colors border text-left"
            :class="settingsStore.memoryRecallMode === mode
              ? 'bg-gray-100 dark:bg-gray-700/30 border-gray-300 dark:border-gray-500 text-gray-900 dark:text-white'
              : 'bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-700/60'"
            @click="handleMemoryRecallModeChange(mode)"
          >
            <div class="font-medium">{{ t(`settings.memoryRecallMode.options.${mode}.label`) }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ t(`settings.memoryRecallMode.options.${mode}.hint`) }}</div>
          </button>
        </div>
      </div>
    </div>

    <!-- User Data Tab -->
    <div v-if="activeTab === 'userdata'" class="space-y-6">
      <!-- Workspace Files -->
      <div class="glass-card p-4">
        <WorkspaceSettings @status-change="showSaveStatus" />
      </div>

      <!-- Backup -->
      <BackupManager
        :backups="backups"
        :loading="backupsLoading"
        :restoring="backupRestoring !== null"
        @create="onBackupCreate"
        @restore="restoreBackup"
        @delete="deleteBackup"
        @download="onBackupDownload"
      />

      <UserDataExport @status-change="showSaveStatus" />
    </div>

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
  background: var(--color-gray-900, #3b82f6);
  border-radius: 50%;
  cursor: pointer;
}

input[type='range']::-moz-range-thumb {
  width: 16px;
  height: 16px;
  background: var(--color-gray-900, #3b82f6);
  border-radius: 50%;
  cursor: pointer;
  border: none;
}

.glass-card {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(8px);
  border-radius: 0.75rem;
  border: 1px solid rgb(229, 231, 235);
}

:root.dark .glass-card {
  background: rgba(30, 41, 59, 0.8);
  border-color: rgb(51, 65, 85);
}

.theme-style-btn-preview {
  width: 18px;
  height: 18px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.4);
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
