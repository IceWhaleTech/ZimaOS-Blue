<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { useMetricsStore } from '@/stores/metrics'
import { systemApi, backupApi } from '@/api/index'
import { companionSettingsApi, type RetentionConfig, type StorageInfo } from '@/api/companion'
import type { LocaleKey } from '@/i18n'
import type { LogEntry } from '@/api/system'
import type { BackupInfo } from '@/api/index'
import ClaudeCodeSettings from '@/components/ClaudeCodeSettings.vue'
import ProviderPoolSection from '@/components/ProviderPoolSection.vue'
import ServiceManagement from '@/components/ServiceManagement.vue'
import MetricsOverview from '@/components/metrics/MetricsOverview.vue'
import TokenUsageChart from '@/components/metrics/TokenUsageChart.vue'
import LatencyChart from '@/components/metrics/LatencyChart.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()
const metricsStore = useMetricsStore()

const saveStatus = ref<string | null>(null)

// Active tab - flattened structure
type TabType = 'general' | 'llm' | 'metrics' | 'config' | 'backup' | 'logs' | 'service' | 'retention'
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

const temperatureDisplay = computed(() => settingsStore.temperature.toFixed(1))

// System Tab - Logs
const logs = ref<LogEntry[]>([])
const logsLoading = ref(false)
const logLevel = ref('all')
const logSearch = ref('')
const logLimit = ref(100)

// System Tab - Config
const config = ref<Record<string, unknown>>({})
const configLoading = ref(false)
const configEditing = ref(false)
const configJson = ref('')
const configError = ref<string | null>(null)

// System Tab - Backup
const backups = ref<BackupInfo[]>([])
const backupsLoading = ref(false)
const backupCreating = ref(false)
const backupRestoring = ref<string | null>(null)

// Data Retention Settings
const retentionLoading = ref(false)
const cleanupLoading = ref(false)
const retentionConfig = ref<RetentionConfig>({
  events_days: 7,
  sessions_days: 30,
  alerts_days: 90,
})
const storageInfo = ref<StorageInfo>({
  session_count: 0,
  alert_count: 0,
  event_count: 0,
})

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
  } else if (tab === 'config' && !configJson.value) {
    fetchConfig()
  } else if (tab === 'backup' && backups.value.length === 0) {
    fetchBackups()
  } else if (tab === 'retention') {
    fetchRetentionSettings()
  } else if (tab === 'metrics') {
    metricsStore.fetchAll()
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
  switch (level.toLowerCase()) {
    case 'error': return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
    case 'warn': return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
    case 'info': return 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300'
    case 'debug': return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
    default: return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  }
}

function isRequestLog(log: LogEntry): boolean {
  return log.message === 'request' && log.fields?.method !== undefined
}

function getMethodColor(method: string): string {
  switch (method?.toUpperCase()) {
    case 'GET': return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
    case 'POST': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    case 'PUT': return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'PATCH': return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
    case 'DELETE': return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
    default: return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
  }
}

function getStatusColor(status: number): string {
  if (status >= 500) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  if (status >= 400) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  if (status >= 300) return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
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

// Config functions
async function fetchConfig() {
  configLoading.value = true
  try {
    const response = await systemApi.getConfig()
    config.value = response.data
    configJson.value = JSON.stringify(response.data, null, 2)
  } catch (e) {
    console.error('Failed to fetch config:', e)
  } finally {
    configLoading.value = false
  }
}

async function saveConfig() {
  configError.value = null
  try {
    const parsed = JSON.parse(configJson.value)
    await systemApi.updateConfig(parsed)
    config.value = parsed
    configEditing.value = false
    showSaveStatus(t('system.configSaved'))
  } catch (e) {
    if (e instanceof SyntaxError) {
      configError.value = t('system.invalidJson')
    } else {
      configError.value = t('system.configSaveFailed')
    }
  }
}

async function copyConfigToClipboard() {
  try {
    await navigator.clipboard.writeText(configJson.value)
    showSaveStatus(t('common.copied'))
  } catch (e) {
    console.error('Failed to copy:', e)
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

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString()
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

// Retention functions
async function fetchRetentionSettings() {
  retentionLoading.value = true
  try {
    const response = await companionSettingsApi.getSettings()
    retentionConfig.value = response.data.retention
    storageInfo.value = response.data.storage_info
  } catch (e) {
    console.error('Failed to fetch retention settings:', e)
  } finally {
    retentionLoading.value = false
  }
}

async function saveRetentionSettings() {
  retentionLoading.value = true
  try {
    await companionSettingsApi.updateSettings(retentionConfig.value)
    showSaveStatus(t('common.saved'))
  } catch (e) {
    console.error('Failed to save retention settings:', e)
  } finally {
    retentionLoading.value = false
  }
}

async function triggerCleanup() {
  cleanupLoading.value = true
  try {
    await companionSettingsApi.triggerCleanup()
    await fetchRetentionSettings()
    showSaveStatus(t('security.settings.cleanupSuccess'))
  } catch (e) {
    console.error('Failed to trigger cleanup:', e)
  } finally {
    cleanupLoading.value = false
  }
}

async function handleResetMetrics() {
  if (!confirm(t('metrics.confirmReset'))) return
  try {
    await metricsStore.resetMetrics()
    showSaveStatus(t('metrics.resetSuccess'))
  } catch (e) {
    console.error('Failed to reset metrics:', e)
  }
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
        v-for="tab in ['general', 'llm', 'metrics', 'config', 'backup', 'logs', 'service', 'retention'] as const"
        :key="tab"
        class="px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap flex-shrink-0"
        :class="
          activeTab === tab
            ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400'
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
          class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
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
          class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
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
                ? 'bg-accent text-white'
                : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
            ]"
            @click="themeStore.setTheme(theme)"
          >
            {{ t(`common.${theme}`) }}
          </button>
        </div>
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

      <!-- Model Parameters -->
      <div class="glass-card p-4 space-y-6">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
          {{ t('settings.modelParameters') }}
        </h3>

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
    </div>

    <!-- Metrics Tab -->
    <div v-if="activeTab === 'metrics'" class="space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <span v-if="metricsStore.lastUpdated" class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('metrics.lastUpdated') }}: {{ metricsStore.lastUpdated.toLocaleTimeString() }}
          </span>
        </div>
        <button
          class="px-3 py-1.5 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
          @click="handleResetMetrics"
        >
          {{ t('metrics.reset') }}
        </button>
      </div>

      <MetricsOverview />

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <TokenUsageChart />
        <LatencyChart />
      </div>

      <!-- Model Statistics -->
      <div class="glass-card p-6">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          {{ t('metrics.modelStats') }}
        </h3>
        <div v-if="metricsStore.modelStats?.models?.length" class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 dark:border-gray-700">
                <th class="text-left py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.model') }}</th>
                <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.calls') }}</th>
                <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.successRate') }}</th>
                <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.tokens') }}</th>
                <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.cost') }}</th>
                <th class="text-right py-3 px-2 text-gray-500 dark:text-gray-400">{{ t('metrics.avgLatency') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="model in metricsStore.modelStats.models"
                :key="model.model"
                class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50"
              >
                <td class="py-3 px-2 font-medium text-gray-900 dark:text-white">{{ model.model }}</td>
                <td class="py-3 px-2 text-right text-gray-700 dark:text-gray-300">{{ model.calls ?? 0 }}</td>
                <td class="py-3 px-2 text-right">
                  <span :class="(model.success_rate ?? 0) >= 95 ? 'text-green-600 dark:text-green-400' : 'text-orange-600 dark:text-orange-400'">
                    {{ (model.success_rate ?? 0).toFixed(1) }}%
                  </span>
                </td>
                <td class="py-3 px-2 text-right text-gray-700 dark:text-gray-300">{{ (model.total_tokens ?? 0).toLocaleString() }}</td>
                <td class="py-3 px-2 text-right text-green-600 dark:text-green-400">${{ (model.estimated_cost ?? 0).toFixed(4) }}</td>
                <td class="py-3 px-2 text-right text-gray-700 dark:text-gray-300">{{ (model.avg_latency_ms ?? 0).toFixed(0) }}ms</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
          {{ t('metrics.noData') }}
        </div>
      </div>
    </div>

    <!-- Config Tab -->
    <div v-if="activeTab === 'config'" class="glass-card p-4">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('system.configuration') }}</h3>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">{{ t('system.configDescription') }}</p>
        </div>
        <div class="flex gap-2">
          <button
            class="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg text-sm"
            @click="copyConfigToClipboard"
          >
            {{ t('common.copy') }}
          </button>
          <button
            v-if="!configEditing"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm"
            @click="configEditing = true"
          >
            {{ t('system.edit') }}
          </button>
          <template v-else>
            <button class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded-lg text-sm" @click="saveConfig">
              {{ t('system.save') }}
            </button>
            <button
              class="px-4 py-2 bg-gray-300 dark:bg-gray-600 hover:bg-gray-400 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg text-sm"
              @click="configEditing = false; configJson = JSON.stringify(config, null, 2); configError = null"
            >
              {{ t('system.cancel') }}
            </button>
          </template>
        </div>
      </div>

      <div v-if="configError" class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-4 text-red-700 dark:text-red-300 mb-4">
        {{ configError }}
      </div>

      <div v-if="configLoading" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('system.loadingConfig') }}</div>
      <div v-else class="relative">
        <div class="absolute left-0 top-0 bottom-0 w-12 bg-gray-100 dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700 overflow-hidden pointer-events-none rounded-l-lg">
          <div class="p-4 font-mono text-sm text-gray-400 dark:text-gray-500 leading-6">
            <div v-for="n in configJson.split('\n').length" :key="n">{{ n }}</div>
          </div>
        </div>
        <textarea
          v-model="configJson"
          :readonly="!configEditing"
          class="w-full h-[400px] bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 p-4 pl-16 font-mono text-sm focus:outline-none resize-none leading-6 rounded-lg border border-gray-200 dark:border-gray-700"
          :class="{ 'bg-gray-50 dark:bg-gray-700': configEditing }"
          spellcheck="false"
        ></textarea>
      </div>
    </div>

    <!-- Backup Tab -->
    <div v-if="activeTab === 'backup'" class="glass-card p-4">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('system.backups') }}</h3>
        <button
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm flex items-center gap-2"
          :disabled="backupCreating"
          @click="createBackup"
        >
          <svg v-if="backupCreating" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          {{ backupCreating ? t('system.creating') : t('system.createBackup') }}
        </button>
      </div>

      <div v-if="backupsLoading" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('system.loadingBackups') }}</div>
      <div v-else-if="backups.length === 0" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('system.noBackupsFound') }}</div>
      <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
        <div v-for="backup in backups" :key="backup.id" class="py-4 flex items-center justify-between">
          <div>
            <div class="text-gray-900 dark:text-white font-medium">{{ backup.id }}</div>
            <div class="text-sm text-gray-500 dark:text-gray-400 flex items-center gap-4 mt-1">
              <span>{{ formatDate(backup.created_at) }}</span>
              <span>{{ formatBytes(backup.size_bytes) }}</span>
              <span class="text-blue-600 dark:text-blue-400">{{ backup.type }}</span>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <button
              class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white rounded text-sm"
              :disabled="backupRestoring === backup.id"
              @click="restoreBackup(backup.id)"
            >
              {{ backupRestoring === backup.id ? t('system.restoring') : t('system.restore') }}
            </button>
            <button
              class="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white rounded text-sm"
              @click="deleteBackup(backup.id)"
            >
              {{ t('system.delete') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Logs Tab -->
    <div v-if="activeTab === 'logs'" class="glass-card p-4">
      <div class="flex flex-wrap gap-4 items-center mb-4">
        <div class="flex-1 min-w-[200px]">
          <input
            v-model="logSearch"
            type="text"
            :placeholder="t('system.searchLogs')"
            class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
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
          <button class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg" :disabled="logsLoading" @click="fetchLogs">
            {{ logsLoading ? t('common.loading') : t('system.refresh') }}
          </button>
          <button class="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg" :disabled="logs.length === 0" @click="exportLogs">
            {{ t('system.exportLogs') }}
          </button>
          <button class="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-red-100 dark:hover:bg-red-600 text-gray-700 dark:text-white rounded-lg" :disabled="logs.length === 0" @click="clearLogs">
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

    <!-- Service Tab -->
    <div v-if="activeTab === 'service'" class="glass-card p-4">
      <ServiceManagement />
    </div>

    <!-- Retention Tab -->
    <div v-if="activeTab === 'retention'" class="space-y-4">
      <div class="glass-card p-4">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('security.settings.title') }}</h3>
            <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">{{ t('security.settings.description') }}</p>
          </div>
        </div>

        <!-- Storage Info -->
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg mb-4">
          <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-2">{{ t('security.settings.storageInfo') }}</h4>
          <div class="grid grid-cols-3 gap-2 text-sm">
            <div>
              <div class="text-gray-500 dark:text-slate-400">{{ t('security.settings.sessions') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ storageInfo.session_count }}</div>
            </div>
            <div>
              <div class="text-gray-500 dark:text-slate-400">{{ t('security.settings.events') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ storageInfo.event_count }}</div>
            </div>
            <div>
              <div class="text-gray-500 dark:text-slate-400">{{ t('security.settings.alerts') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ storageInfo.alert_count }}</div>
            </div>
          </div>
        </div>

        <!-- Retention Settings -->
        <div class="space-y-3 mb-4">
          <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('security.settings.retentionPolicy') }}</h4>
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <label class="text-sm text-gray-700 dark:text-slate-300">{{ t('security.settings.sessionsRetention') }}</label>
              <div class="flex items-center gap-2">
                <input
                  v-model.number="retentionConfig.sessions_days"
                  type="number"
                  min="1"
                  max="365"
                  class="w-20 px-2 py-1 text-sm border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
                <span class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.settings.days') }}</span>
              </div>
            </div>
            <div class="flex items-center justify-between">
              <label class="text-sm text-gray-700 dark:text-slate-300">{{ t('security.settings.eventsRetention') }}</label>
              <div class="flex items-center gap-2">
                <input
                  v-model.number="retentionConfig.events_days"
                  type="number"
                  min="1"
                  max="365"
                  class="w-20 px-2 py-1 text-sm border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
                <span class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.settings.days') }}</span>
              </div>
            </div>
            <div class="flex items-center justify-between">
              <label class="text-sm text-gray-700 dark:text-slate-300">{{ t('security.settings.alertsRetention') }}</label>
              <div class="flex items-center gap-2">
                <input
                  v-model.number="retentionConfig.alerts_days"
                  type="number"
                  min="1"
                  max="365"
                  class="w-20 px-2 py-1 text-sm border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
                <span class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.settings.days') }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="flex items-center justify-between pt-4 border-t border-gray-200 dark:border-slate-700">
          <button
            class="px-4 py-2 text-sm bg-red-100 dark:bg-red-900/30 hover:bg-red-200 dark:hover:bg-red-900/50 text-red-700 dark:text-red-300 rounded-lg transition-colors disabled:opacity-50"
            :disabled="cleanupLoading"
            @click="triggerCleanup"
          >
            {{ cleanupLoading ? t('common.loading') : t('security.settings.cleanupNow') }}
          </button>
          <button
            class="px-4 py-2 text-sm bg-accent hover:bg-accent-hover text-white rounded-lg transition-colors disabled:opacity-50"
            :disabled="retentionLoading"
            @click="saveRetentionSettings"
          >
            {{ retentionLoading ? t('common.loading') : t('common.save') }}
          </button>
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
