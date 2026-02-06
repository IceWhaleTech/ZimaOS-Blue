<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { systemApi, backupApi } from '@/api/index'
import type { LocaleKey } from '@/i18n'
import type { LogEntry } from '@/api/system'
import type { BackupInfo } from '@/api/index'
import ClaudeCodeSettings from '@/components/ClaudeCodeSettings.vue'
import ProviderPoolSection from '@/components/ProviderPoolSection.vue'
import ServiceManagement from '@/components/ServiceManagement.vue'
import UserDataExport from '@/components/UserDataExport.vue'
import NetworkSettings from '@/components/settings/NetworkSettings.vue'
import SpeechSettings from '@/components/settings/SpeechSettings.vue'
import UpdateSettings from '@/components/settings/UpdateSettings.vue'
import MemoryManager from '@/components/MemoryManager.vue'
import BackupManager from '@/components/BackupManager.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()

const saveStatus = ref<string | null>(null)

// Active tab - flattened structure
type TabType = 'general' | 'llm' | 'network' | 'speech' | 'userdata' | 'update' | 'logs'
const activeTab = ref<TabType>((route.query.tab as TabType) || 'general')

// Timezone
const detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const selectedTimezone = ref(localStorage.getItem('zimaos-echo-timezone') || detectedTimezone)

const timezones = computed(() => {
  try {
    const allTimezones = (Intl as unknown as { supportedValuesOf: (key: string) => string[] }).supportedValuesOf('timeZone')
    const filtered = allTimezones.filter((tz: string) => tz !== detectedTimezone)
    return [detectedTimezone, ...filtered]
  } catch {
    return [detectedTimezone, 'UTC', 'America/New_York', 'America/Los_Angeles', 'Europe/London', 'Europe/Paris', 'Asia/Tokyo', 'Asia/Shanghai']
  }
})

// System Tab - Logs
const logs = ref<LogEntry[]>([])
const logsLoading = ref(false)
const logLevel = ref('all')
const logSearch = ref('')
const logLimit = ref(100)

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
  localStorage.setItem('zimaos-echo-timezone', timezone)
  showSaveStatus(t('settings.timezoneSaved'))
}

function switchTab(tab: TabType) {
  activeTab.value = tab
  router.replace({ query: { tab } })

  // Load data for specific tabs
  if (tab === 'logs' && logs.value.length === 0) {
    fetchLogs()
  } else if (tab === 'userdata' && backups.value.length === 0) {
    fetchBackups()
  }
}

// Logs functions
async function fetchLogs() {
  logsLoading.value = true
  try {
    const response = await systemApi.getLogs({
      level: logLevel.value === 'all' ? undefined : logLevel.value,
      search: logSearch.value || undefined,
      limit: logLimit.value,
    })
    logs.value = response.data || []
  } catch (e) {
    console.error('Failed to fetch logs:', e)
  } finally {
    logsLoading.value = false
  }
}

function formatLogTime(timestamp: string) {
  return new Date(timestamp).toLocaleTimeString()
}

function getLogLevelClass(level: string) {
  if (!level) return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  switch (level.toLowerCase()) {
    case 'error': return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
    case 'warn': return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
    case 'info': return 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
    case 'debug': return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
    default: return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  }
}

function isRequestLog(log: LogEntry): boolean {
  return log.message === 'request' && log.fields?.method !== undefined
}

function getMethodColor(method: string | undefined): string {
  if (!method) return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
  switch (method.toUpperCase()) {
    case 'GET': return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
    case 'POST': return 'bg-gray-700 dark:bg-gray-700 text-gray-900 dark:text-white dark:bg-gray-700 dark:bg-gray-700/30 dark:text-gray-900 dark:text-white'
    case 'PUT': return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'PATCH': return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
    case 'DELETE': return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
    default: return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
  }
}

function getStatusColor(status: number): string {
  if (status >= 500) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  if (status >= 400) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  if (status >= 300) return 'bg-gray-700 dark:bg-gray-700 text-gray-900 dark:text-white dark:bg-gray-700 dark:bg-gray-700/30 dark:text-gray-900 dark:text-white'
  if (status >= 200) return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
  return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
}

function formatLatency(latency: number): string {
  if (typeof latency !== 'number') return ''
  if (latency >= 1_000_000_000) return `${(latency / 1_000_000_000).toFixed(2)}s`
  if (latency >= 1_000_000) return `${(latency / 1_000_000).toFixed(0)}ms`
  if (latency >= 1_000) return `${(latency / 1_000).toFixed(0)}µs`
  return `${latency}ns`
}

async function exportLogs() {
  const content = logs.value.map(log => `[${log.timestamp}] [${log.level}] ${log.source ? `[${log.source}] ` : ''}${log.message}`).join('\n')
  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `echo-logs-${new Date().toISOString().split('T')[0]}.txt`
  a.click()
  URL.revokeObjectURL(url)
}

function clearLogs() {
  if (!confirm(t('system.confirmClearLogs'))) return
  // Clear logs locally (backend API not available)
  logs.value = []
  showSaveStatus(t('system.logsCleared'))
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

function _formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString()
}

function _formatBytes(bytes: number) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

onMounted(async () => {
  await settingsStore.fetchProviders()

  // Load data based on initial tab
  const tab = route.query.tab as TabType
  if (tab) {
    switchTab(tab)
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
        v-for="tab in ['general', 'llm', 'speech', 'network', 'userdata', 'update', 'logs'] as const"
        :key="tab"
        class="px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap flex-shrink-0"
        :class="
          activeTab === tab
            ? 'text-gray-900 dark:text-white dark:text-gray-900 dark:text-white border-b-2 border-gray-900 dark:border-white dark:border-gray-900 dark:border-white'
            : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white'
        "
        @click="switchTab(tab)"
      >
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
                ? 'bg-gray-700 dark:bg-gray-700 text-white'
                : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
            ]"
            @click="themeStore.setTheme(theme)"
          >
            {{ t(`common.${theme}`) }}
          </button>
        </div>
      </div>

      <!-- Service Management -->
      <div class="glass-card p-4">
        <ServiceManagement />
      </div>
    </div>

    <!-- LLM Tab -->
    <div v-if="activeTab === 'llm'" class="space-y-6">
      <!-- Provider Pool -->
      <div class="glass-card p-4">
        <ProviderPoolSection />
      </div>

      <!-- Claude Code CLI Settings -->
      <ClaudeCodeSettings @status-change="showSaveStatus" />
    </div>

    <!-- Network Tab -->
    <div v-if="activeTab === 'network'">
      <NetworkSettings @status-change="showSaveStatus" />
    </div>

    <!-- Speech Tab -->
    <div v-if="activeTab === 'speech'">
      <SpeechSettings />
    </div>

    <!-- User Data Tab -->
    <div v-if="activeTab === 'userdata'" class="space-y-6">
      <MemoryManager @status-change="showSaveStatus" />

      <UserDataExport @status-change="showSaveStatus" />

      <!-- System Backup Section -->
      <div class="mt-6">
        <BackupManager
          :backups="backups"
          :loading="backupsLoading"
          :restoring="backupRestoring"
          @create="onBackupCreate"
          @restore="restoreBackup"
          @delete="deleteBackup"
          @download="onBackupDownload"
        />
      </div>
    </div>

    <!-- Update Tab -->
    <div v-if="activeTab === 'update'">
      <UpdateSettings />
    </div>

    <!-- Logs Tab -->
    <div v-if="activeTab === 'logs'" class="glass-card p-4">
      <div class="flex flex-wrap gap-4 items-center mb-4">
        <div class="flex-1 min-w-[200px]">
          <input
            v-model="logSearch"
            type="text"
            :placeholder="t('system.searchLogs')"
            class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
            @keyup.enter="fetchLogs"
          />
        </div>
        <select
          v-model="logLevel"
          class="bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600"
          @change="fetchLogs"
        >
          <option value="all">{{ t('system.allLevels') }}</option>
          <option value="error">{{ t('system.error') }}</option>
          <option value="warn">{{ t('system.warning') }}</option>
          <option value="info">{{ t('system.info') }}</option>
          <option value="debug">{{ t('system.debug') }}</option>
        </select>
        <select
          v-model="logLimit"
          class="bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600"
          @change="fetchLogs"
        >
          <option :value="50">50</option>
          <option :value="100">100</option>
          <option :value="200">200</option>
        </select>
        <div class="flex gap-2">
          <button class="px-4 py-2 bg-gray-700 dark:bg-gray-700 hover:bg-gray-700 dark:bg-gray-700 text-white rounded-lg" :disabled="logsLoading" @click="fetchLogs">
            {{ logsLoading ? t('common.loading') : t('system.refresh') }}
          </button>
          <button class="px-3 py-2 bg-gray-100 dark:bg-gray-700/30 hover:bg-gray-200 dark:hover:bg-gray-700/50 text-gray-700 dark:text-gray-300 rounded-lg" :disabled="logs.length === 0" @click="exportLogs">
            {{ t('system.exportLogs') }}
          </button>
          <button class="px-3 py-2 bg-red-100 dark:bg-red-900/30 hover:bg-red-200 dark:hover:bg-red-900/50 text-red-700 dark:text-red-400 rounded-lg" :disabled="logs.length === 0" @click="clearLogs">
            {{ t('system.clearLogs') }}
          </button>
        </div>
      </div>

      <div v-if="logsLoading" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('system.loadingLogs') }}</div>
      <div v-else-if="logs.length === 0" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('system.noLogsFound') }}</div>
      <div v-else class="divide-y divide-gray-200 dark:divide-gray-700 max-h-[400px] overflow-y-auto font-mono text-sm">
        <div v-for="(log, index) in logs" :key="index" class="p-3 hover:bg-gray-50 dark:hover:bg-gray-700/50 flex gap-3 items-start">
          <span class="text-gray-400 dark:text-gray-500 flex-shrink-0 w-20">{{ formatLogTime(log.timestamp) }}</span>
          <span :class="getLogLevelClass(log.level)" class="px-2 py-0.5 rounded text-xs uppercase font-medium flex-shrink-0">{{ log.level }}</span>
          <span v-if="log.source" class="text-purple-600 dark:text-purple-400 flex-shrink-0">[{{ log.source.split('/').pop()?.split(':')[0] }}]</span>
          <!-- Request log with tags -->
          <template v-if="isRequestLog(log)">
            <span class="px-1.5 py-0.5 text-xs font-medium rounded" :class="getMethodColor(log.fields?.method as string)">
              {{ log.fields?.method }}
            </span>
            <span class="text-gray-900 dark:text-gray-100 break-all flex-1 truncate" :title="log.fields?.uri as string">
              {{ log.fields?.uri }}
            </span>
            <span class="px-1.5 py-0.5 text-xs font-medium rounded" :class="getStatusColor(log.fields?.status as number)">
              {{ log.fields?.status }}
            </span>
            <span class="text-gray-500 dark:text-gray-400 text-xs whitespace-nowrap">
              {{ formatLatency(log.fields?.latency as number) }}
            </span>
          </template>
          <!-- Regular log message -->
          <span v-else class="text-gray-700 dark:text-gray-300 break-all">{{ log.message }}</span>
        </div>
      </div>
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
