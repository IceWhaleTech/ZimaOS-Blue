<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { useMetricsStore } from '@/stores/metrics'
import { systemApi, backupApi } from '@/api/index'
import type { LogEntry, SystemMetrics, DetailedSystemInfo } from '@/api/system'
import type { BackupInfo } from '@/api/index'
import Skeleton from '@/components/Skeleton.vue'
import ProgressBar from '@/components/ProgressBar.vue'
import DonutChart from '@/components/DonutChart.vue'
import MetricsOverview from '@/components/metrics/MetricsOverview.vue'
import TokenUsageChart from '@/components/metrics/TokenUsageChart.vue'
import LatencyChart from '@/components/metrics/LatencyChart.vue'
import ServiceManagement from '@/components/ServiceManagement.vue'
import { ConfigurableDashboard } from '@/components/dashboard'

const { t } = useI18n()
const systemStore = useSystemStore()
const metricsStore = useMetricsStore()

// Tabs
const activeTab = ref<'overview' | 'metrics' | 'logs' | 'config' | 'backup' | 'service'>('overview')

// Metrics
const metricsHistory = ref<SystemMetrics[]>([])
const autoRefresh = ref(true)
let refreshInterval: ReturnType<typeof setInterval> | null = null

// Detailed system info
const detailedInfo = ref<DetailedSystemInfo | null>(null)
const detailedInfoLoading = ref(false)
const showDetailedInfo = ref(false)

// Logs
const logs = ref<LogEntry[]>([])
const logsLoading = ref(false)
const logLevel = ref('all')
const logSearch = ref('')
const logLimit = ref(100)

// Config
const config = ref<Record<string, unknown>>({})
const configLoading = ref(false)
const configEditing = ref(false)
const configJson = ref('')
const configError = ref<string | null>(null)
const configSaveStatus = ref<string | null>(null)

// Backup
const backups = ref<BackupInfo[]>([])
const backupsLoading = ref(false)
const backupCreating = ref(false)
const backupRestoring = ref<string | null>(null)

async function handleResetMetrics() {
  if (confirm(t('metrics.confirmReset'))) {
    await metricsStore.resetMetrics()
  }
}

onMounted(async () => {
  await systemStore.fetchAll()
  await fetchMetricsHistory()
  // Also fetch metrics store data
  metricsStore.fetchAll()

  refreshInterval = setInterval(() => {
    if (autoRefresh.value) {
      systemStore.fetchAll()
      if (activeTab.value === 'overview') {
        fetchMetricsHistory()
      }
      if (activeTab.value === 'metrics') {
        metricsStore.fetchAll()
      }
    }
  }, 5000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})

async function fetchMetricsHistory() {
  try {
    const response = await systemApi.getMetricsHistory('5m')
    metricsHistory.value = response.data.metrics || []
  } catch {
    // Metrics history might not be available
    metricsHistory.value = []
  }
}

async function fetchDetailedInfo() {
  detailedInfoLoading.value = true
  try {
    const response = await systemApi.getInfo(true)
    detailedInfo.value = response.data.system || null
  } catch {
    detailedInfo.value = null
  } finally {
    detailedInfoLoading.value = false
  }
}

function toggleDetailedInfo() {
  showDetailedInfo.value = !showDetailedInfo.value
  if (showDetailedInfo.value && !detailedInfo.value) {
    fetchDetailedInfo()
  }
}

async function fetchLogs() {
  logsLoading.value = true
  try {
    const params: Record<string, unknown> = { limit: logLimit.value }
    if (logLevel.value !== 'all') {
      params.level = logLevel.value
    }
    if (logSearch.value) {
      params.search = logSearch.value
    }
    const response = await systemApi.getLogs(params)
    logs.value = response.data
  } catch {
    logs.value = []
  } finally {
    logsLoading.value = false
  }
}

async function fetchConfig() {
  configLoading.value = true
  try {
    const response = await systemApi.getConfig()
    config.value = response.data
    configJson.value = JSON.stringify(response.data, null, 2)
  } catch {
    config.value = {}
    configJson.value = '{}'
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
    showConfigStatus('Configuration saved successfully')
  } catch (e) {
    configError.value = e instanceof Error ? e.message : 'Invalid JSON'
  }
}

function showConfigStatus(message: string) {
  configSaveStatus.value = message
  setTimeout(() => {
    configSaveStatus.value = null
  }, 3000)
}

function copyConfigToClipboard() {
  navigator.clipboard.writeText(configJson.value)
}

async function fetchBackups() {
  backupsLoading.value = true
  try {
    const response = await backupApi.list()
    backups.value = response.data
  } catch {
    backups.value = []
  } finally {
    backupsLoading.value = false
  }
}

async function createBackup() {
  backupCreating.value = true
  try {
    const response = await backupApi.create()
    backups.value = [response.data, ...backups.value]
  } catch {
    // Handle error
  } finally {
    backupCreating.value = false
  }
}

async function restoreBackup(id: string) {
  if (!confirm(t('system.backup.restoreConfirm'))) {
    return
  }

  backupRestoring.value = id
  try {
    await backupApi.restore(id)
    alert(t('system.backup.backupRestored'))
  } catch {
    alert(t('system.backup.backupRestoreFailed'))
  } finally {
    backupRestoring.value = null
  }
}

async function deleteBackup(id: string) {
  if (!confirm(t('system.backup.deleteConfirm'))) {
    return
  }

  try {
    await backupApi.delete(id)
    backups.value = backups.value.filter((b) => b.id !== id)
  } catch {
    alert(t('system.backup.backupDeleteFailed'))
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString([], {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function formatLogTime(timestamp: string): string {
  return new Date(timestamp).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function getLogLevelClass(level: string): string {
  switch (level) {
    case 'error':
      return 'text-red-400 bg-red-900/30'
    case 'warn':
      return 'text-yellow-400 bg-yellow-900/30'
    case 'info':
      return 'text-blue-400 bg-blue-900/30'
    case 'debug':
      return 'text-gray-400 bg-gray-700'
    default:
      return 'text-gray-300 bg-gray-700'
  }
}

function isRequestLog(log: LogEntry): boolean {
  return log.message === 'request' && log.fields?.method !== undefined
}

function getMethodColor(method: string): string {
  switch (method?.toUpperCase()) {
    case 'GET':
      return 'bg-green-900/30 text-green-400'
    case 'POST':
      return 'bg-blue-900/30 text-blue-400'
    case 'PUT':
      return 'bg-yellow-900/30 text-yellow-400'
    case 'PATCH':
      return 'bg-orange-900/30 text-orange-400'
    case 'DELETE':
      return 'bg-red-900/30 text-red-400'
    default:
      return 'bg-gray-700 text-gray-300'
  }
}

function getStatusColor(status: number): string {
  if (status >= 500) {
    return 'bg-red-900/30 text-red-400'
  } else if (status >= 400) {
    return 'bg-yellow-900/30 text-yellow-400'
  } else if (status >= 300) {
    return 'bg-blue-900/30 text-blue-400'
  } else if (status >= 200) {
    return 'bg-green-900/30 text-green-400'
  }
  return 'bg-gray-700 text-gray-300'
}

function formatLatency(latency: number): string {
  if (typeof latency !== 'number') return ''
  // latency is in nanoseconds from Go's time.Duration
  if (latency >= 1_000_000_000) {
    return `${(latency / 1_000_000_000).toFixed(2)}s`
  } else if (latency >= 1_000_000) {
    return `${(latency / 1_000_000).toFixed(0)}ms`
  } else if (latency >= 1_000) {
    return `${(latency / 1_000).toFixed(0)}µs`
  }
  return `${latency}ns`
}

function exportLogs() {
  if (logs.value.length === 0) return

  const content = logs.value
    .map((log) => {
      const time = new Date(log.timestamp).toISOString()
      const source = log.source ? `[${log.source}]` : ''
      return `${time} [${log.level.toUpperCase()}] ${source} ${log.message}`
    })
    .join('\n')

  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `logs-${new Date().toISOString().slice(0, 10)}.txt`
  a.click()
  URL.revokeObjectURL(url)
}

function clearLogs() {
  logs.value = []
}

// Load data when tab changes
function switchTab(tab: 'overview' | 'metrics' | 'logs' | 'config' | 'backup' | 'service') {
  activeTab.value = tab
  if (tab === 'logs' && logs.value.length === 0) {
    fetchLogs()
  } else if (tab === 'config' && Object.keys(config.value).length === 0) {
    fetchConfig()
  } else if (tab === 'backup' && backups.value.length === 0) {
    fetchBackups()
  } else if (tab === 'metrics') {
    metricsStore.fetchAll()
  }
}
</script>

<template>
  <div class="system-view max-w-6xl mx-auto">
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-6">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">{{ t('system.title') }}</h1>
      <label class="flex items-center space-x-2">
        <input
          v-model="autoRefresh"
          type="checkbox"
          class="rounded border-gray-300 dark:border-gray-600 bg-gray-100 dark:bg-gray-700 text-blue-600 focus:ring-blue-500"
        />
        <span class="text-sm text-gray-500 dark:text-gray-400">{{ t('system.autoRefresh') }}</span>
      </label>
    </div>

    <!-- Status notification -->
    <div
      v-if="configSaveStatus"
      class="fixed top-20 right-4 bg-green-600 text-white px-4 py-2 rounded-lg shadow-lg z-50"
    >
      {{ configSaveStatus }}
    </div>

    <!-- Tabs -->
    <div class="flex overflow-x-auto border-b border-gray-200 dark:border-gray-700 mb-6 -mx-4 px-4 sm:mx-0 sm:px-0">
      <button
        v-for="tab in ['overview', 'metrics', 'logs', 'config', 'backup', 'service'] as const"
        :key="tab"
        class="px-3 sm:px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap flex-shrink-0"
        :class="
          activeTab === tab
            ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400'
            : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white'
        "
        @click="switchTab(tab)"
      >
        {{ tab === 'service' ? t('service.title') : t(`system.${tab}`) }}
      </button>
    </div>

    <!-- Overview Tab -->
    <div v-if="activeTab === 'overview'" class="space-y-4 sm:space-y-6">
      <!-- Configurable Dashboard -->
      <ConfigurableDashboard :metrics-history="metricsHistory" />

      <!-- Detailed System Info Toggle -->
      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('system.detailedSystemInfo') }}</h3>
          <button
            class="px-3 py-1.5 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
            @click="toggleDetailedInfo"
          >
            {{ showDetailedInfo ? t('common.close') : t('system.detailedInfo') }}
          </button>
        </div>
      </div>

      <!-- Detailed System Info -->
      <div v-if="showDetailedInfo" class="space-y-4">
        <div v-if="detailedInfoLoading" class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow text-center text-gray-500 dark:text-gray-400">
          {{ t('system.loadingDetailedInfo') }}
        </div>
        <template v-else-if="detailedInfo">
          <!-- OS Info -->
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
            <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.osInfo') }}</h4>
            <div class="grid md:grid-cols-3 gap-4 text-sm">
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.osVersion') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.version || '-' }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.kernel') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.kernel || '-' }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.architecture') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.architecture || '-' }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.hostname') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.hostname || '-' }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.uptime') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.uptime_human || '-' }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.bootTime') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.boot_time ? new Date(detailedInfo.os.boot_time * 1000).toLocaleString() : '-' }}</span>
              </div>
            </div>
          </div>

          <!-- CPU Info -->
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
            <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.cpuInfo') }}</h4>
            <div class="flex flex-col md:flex-row gap-6">
              <!-- CPU Usage Donut Chart -->
              <div class="flex-shrink-0 flex justify-center">
                <DonutChart
                  :value="detailedInfo.hardware.cpu.usage || 0"
                  :max="100"
                  :label="t('system.cpuUsage')"
                  color="auto"
                  :size="140"
                />
              </div>
              <!-- CPU Details -->
              <div class="flex-1 grid sm:grid-cols-2 gap-4 text-sm">
                <div class="sm:col-span-2">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpuModel') }}:</span>
                  <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.hardware.cpu.model || '-' }}</span>
                </div>
                <div>
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpuVendor') }}:</span>
                  <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.hardware.cpu.vendor_id || '-' }}</span>
                </div>
                <div>
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpuCores') }}:</span>
                  <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.hardware.cpu.cores || '-' }} {{ t('system.cores') }} / {{ detailedInfo.hardware.cpu.threads || '-' }} {{ t('system.threads') }}</span>
                </div>
                <div>
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpuFrequency') }}:</span>
                  <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.hardware.cpu.frequency ? `${detailedInfo.hardware.cpu.frequency.toFixed(0)} MHz` : '-' }}</span>
                </div>
                <div>
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpuCache') }}:</span>
                  <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.hardware.cpu.cache_size ? `${detailedInfo.hardware.cpu.cache_size} KB` : '-' }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Memory Info -->
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
            <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.memoryInfo') }}</h4>
            <div class="flex flex-col md:flex-row gap-6">
              <!-- Memory Usage Donut Charts -->
              <div class="flex-shrink-0 flex gap-6 justify-center">
                <DonutChart
                  :value="detailedInfo.hardware.memory.used"
                  :max="detailedInfo.hardware.memory.total"
                  :label="t('system.ram')"
                  :value-label="formatBytes(detailedInfo.hardware.memory.used)"
                  color="auto"
                  :size="120"
                />
                <DonutChart
                  v-if="detailedInfo.hardware.memory.swap_total > 0"
                  :value="detailedInfo.hardware.memory.swap_used"
                  :max="detailedInfo.hardware.memory.swap_total"
                  :label="t('system.swap')"
                  :value-label="formatBytes(detailedInfo.hardware.memory.swap_used)"
                  color="purple"
                  :size="120"
                />
              </div>
              <!-- Memory Details -->
              <div class="flex-1 space-y-4">
                <div>
                  <div class="flex justify-between text-sm mb-1">
                    <span class="text-gray-500 dark:text-gray-400">{{ t('system.ram') }}</span>
                    <span class="text-gray-900 dark:text-white">{{ formatBytes(detailedInfo.hardware.memory.used) }} / {{ formatBytes(detailedInfo.hardware.memory.total) }}</span>
                  </div>
                  <ProgressBar
                    :value="detailedInfo.hardware.memory.used"
                    :max="detailedInfo.hardware.memory.total"
                    :show-percent="false"
                    color="auto"
                    size="md"
                  />
                </div>
                <div v-if="detailedInfo.hardware.memory.swap_total > 0">
                  <div class="flex justify-between text-sm mb-1">
                    <span class="text-gray-500 dark:text-gray-400">{{ t('system.swap') }}</span>
                    <span class="text-gray-900 dark:text-white">{{ formatBytes(detailedInfo.hardware.memory.swap_used) }} / {{ formatBytes(detailedInfo.hardware.memory.swap_total) }}</span>
                  </div>
                  <ProgressBar
                    :value="detailedInfo.hardware.memory.swap_used"
                    :max="detailedInfo.hardware.memory.swap_total"
                    :show-percent="false"
                    color="purple"
                    size="md"
                  />
                </div>
                <div class="grid grid-cols-2 gap-4 text-sm pt-2">
                  <div>
                    <span class="text-gray-500 dark:text-gray-400">{{ t('system.availableMemory') }}:</span>
                    <span class="text-gray-900 dark:text-white ml-2">{{ formatBytes(detailedInfo.hardware.memory.available) }}</span>
                  </div>
                  <div>
                    <span class="text-gray-500 dark:text-gray-400">{{ t('system.cached') }}:</span>
                    <span class="text-gray-900 dark:text-white ml-2">{{ formatBytes(detailedInfo.hardware.memory.cached || 0) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Disk Info -->
          <div v-if="detailedInfo.hardware.disk?.length" class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
            <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.diskInfo') }}</h4>
            <!-- Disk Usage Visual Cards -->
            <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
              <div
                v-for="disk in detailedInfo.hardware.disk.slice(0, 6)"
                :key="disk.device"
                class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4"
              >
                <div class="flex items-center justify-between mb-2">
                  <span class="font-medium text-gray-900 dark:text-white text-sm truncate" :title="disk.mount_point">{{ disk.mount_point }}</span>
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ disk.fs_type }}</span>
                </div>
                <ProgressBar
                  :value="disk.used"
                  :max="disk.total"
                  :show-percent="false"
                  color="auto"
                  size="lg"
                />
                <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400 mt-2">
                  <span>{{ formatBytes(disk.used) }} {{ t('system.used') }}</span>
                  <span>{{ formatBytes(disk.available) }} {{ t('system.free') }}</span>
                </div>
              </div>
            </div>
            <!-- Disk Table -->
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="text-left text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                    <th class="pb-2 pr-4">{{ t('system.device') }}</th>
                    <th class="pb-2 pr-4">{{ t('system.mountPoint') }}</th>
                    <th class="pb-2 pr-4">{{ t('system.fileSystem') }}</th>
                    <th class="pb-2 pr-4">{{ t('system.totalSpace') }}</th>
                    <th class="pb-2 pr-4">{{ t('system.usedSpace') }}</th>
                    <th class="pb-2 pr-4">{{ t('system.usage') }}</th>
                    <th class="pb-2">{{ t('system.availableSpace') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="disk in detailedInfo.hardware.disk" :key="disk.device" class="text-gray-900 dark:text-white border-b border-gray-100 dark:border-gray-700/50">
                    <td class="py-2 pr-4 font-mono text-xs">{{ disk.device }}</td>
                    <td class="py-2 pr-4 font-mono text-xs">{{ disk.mount_point }}</td>
                    <td class="py-2 pr-4">{{ disk.fs_type }}</td>
                    <td class="py-2 pr-4">{{ formatBytes(disk.total) }}</td>
                    <td class="py-2 pr-4">{{ formatBytes(disk.used) }}</td>
                    <td class="py-2 pr-4 w-32">
                      <ProgressBar
                        :value="disk.used"
                        :max="disk.total"
                        :show-percent="true"
                        color="auto"
                        size="sm"
                      />
                    </td>
                    <td class="py-2">{{ formatBytes(disk.available) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- GPU Info -->
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
            <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.gpuInfo') }}</h4>
            <div v-if="detailedInfo.hardware.gpu?.length" class="space-y-6">
              <div v-for="(gpu, index) in detailedInfo.hardware.gpu" :key="index" class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                <div class="flex flex-col md:flex-row gap-4">
                  <!-- GPU Memory Donut (if available) -->
                  <div v-if="gpu.memory_total" class="flex-shrink-0 flex justify-center">
                    <DonutChart
                      :value="gpu.memory_used || 0"
                      :max="gpu.memory_total"
                      :label="t('system.vram')"
                      :value-label="formatBytes(gpu.memory_used || 0)"
                      color="green"
                      :size="100"
                    />
                  </div>
                  <!-- GPU Details -->
                  <div class="flex-1 grid sm:grid-cols-2 gap-3 text-sm">
                    <div class="sm:col-span-2">
                      <span class="text-gray-500 dark:text-gray-400">{{ t('system.gpuName') }}:</span>
                      <span class="text-gray-900 dark:text-white ml-2 font-medium">{{ gpu.name || '-' }}</span>
                    </div>
                    <div>
                      <span class="text-gray-500 dark:text-gray-400">{{ t('system.gpuVendor') }}:</span>
                      <span class="text-gray-900 dark:text-white ml-2">{{ gpu.vendor || '-' }}</span>
                    </div>
                    <div>
                      <span class="text-gray-500 dark:text-gray-400">{{ t('system.gpuDriver') }}:</span>
                      <span class="text-gray-900 dark:text-white ml-2">{{ gpu.driver || '-' }}</span>
                    </div>
                    <div v-if="gpu.memory_total">
                      <span class="text-gray-500 dark:text-gray-400">{{ t('system.gpuMemory') }}:</span>
                      <span class="text-gray-900 dark:text-white ml-2">{{ formatBytes(gpu.memory_total) }}</span>
                    </div>
                    <div v-if="gpu.temperature">
                      <span class="text-gray-500 dark:text-gray-400">{{ t('system.gpuTemperature') }}:</span>
                      <span class="text-gray-900 dark:text-white ml-2">{{ gpu.temperature }}°C</span>
                    </div>
                  </div>
                </div>
                <!-- GPU Memory Bar -->
                <div v-if="gpu.memory_total" class="mt-4">
                  <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400 mb-1">
                    <span>{{ t('system.vramUsage') }}</span>
                    <span>{{ formatBytes(gpu.memory_used || 0) }} / {{ formatBytes(gpu.memory_total) }}</span>
                  </div>
                  <ProgressBar
                    :value="gpu.memory_used || 0"
                    :max="gpu.memory_total"
                    :show-percent="false"
                    color="green"
                    size="sm"
                  />
                </div>
              </div>
            </div>
            <div v-else class="text-gray-500 dark:text-gray-400 text-sm flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
              {{ t('system.noGpuDetected') }}
            </div>
          </div>

          <!-- Network Info -->
          <div v-if="detailedInfo.network?.interfaces?.length" class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
            <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.networkInfo') }}</h4>
            <div class="space-y-4">
              <div v-for="iface in detailedInfo.network.interfaces.filter(i => !i.is_loopback && i.is_up)" :key="iface.name" class="border-b border-gray-100 dark:border-gray-700/50 pb-4 last:border-0 last:pb-0">
                <div class="flex items-center gap-2 mb-2">
                  <span class="font-medium text-gray-900 dark:text-white">{{ iface.name }}</span>
                  <span class="px-2 py-0.5 text-xs rounded" :class="iface.is_up ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' : 'bg-gray-100 dark:bg-gray-700 text-gray-500'">
                    {{ iface.is_up ? t('system.interfaceUp') : t('system.interfaceDown') }}
                  </span>
                </div>
                <div class="grid md:grid-cols-3 gap-2 text-sm">
                  <div v-if="iface.mac">
                    <span class="text-gray-500 dark:text-gray-400">{{ t('system.macAddress') }}:</span>
                    <span class="text-gray-900 dark:text-white ml-2 font-mono text-xs">{{ iface.mac }}</span>
                  </div>
                  <div v-if="iface.ipv4?.length">
                    <span class="text-gray-500 dark:text-gray-400">{{ t('system.ipv4Address') }}:</span>
                    <span class="text-gray-900 dark:text-white ml-2 font-mono text-xs">{{ iface.ipv4.join(', ') }}</span>
                  </div>
                  <div>
                    <span class="text-gray-500 dark:text-gray-400">{{ t('system.mtu') }}:</span>
                    <span class="text-gray-900 dark:text-white ml-2">{{ iface.mtu }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Runtime Info -->
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
            <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.runtimeInfo') }}</h4>
            <div class="grid md:grid-cols-4 gap-4 text-sm">
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.goVersion') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.runtime.go_version }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.numGoroutines') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.runtime.num_goroutine }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.goMaxProcs') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.runtime.gomaxprocs }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpus') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.runtime.num_cpu }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.heapAlloc') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.runtime.alloc_mb }} MB</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.totalAlloc') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.runtime.total_alloc_mb }} MB</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.sysMemory') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.runtime.sys_mb }} MB</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.gcCount') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.runtime.num_gc }}</span>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Metrics Tab -->
    <div v-if="activeTab === 'metrics'" class="space-y-6">
      <!-- Metrics Header -->
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <span v-if="metricsStore.lastUpdated" class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('metrics.lastUpdated') }}: {{ metricsStore.lastUpdated.toLocaleTimeString() }}
          </span>
        </div>
        <button
          @click="handleResetMetrics"
          class="px-3 py-1.5 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
        >
          {{ t('metrics.reset') }}
        </button>
      </div>

      <!-- Metrics Overview Cards -->
      <MetricsOverview />

      <!-- Token Usage and Latency Charts -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <TokenUsageChart />
        <LatencyChart />
      </div>

      <!-- Model Statistics -->
      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
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
                <td class="py-3 px-2 text-right text-gray-700 dark:text-gray-300">{{ (model.avg_latency ?? 0).toFixed(0) }}ms</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
          {{ t('metrics.noData') }}
        </div>
      </div>
    </div>

    <!-- Logs Tab -->
    <div v-if="activeTab === 'logs'" class="space-y-4">
      <!-- Filters -->
      <div class="bg-white dark:bg-gray-800 rounded-lg p-4 flex flex-wrap gap-4 items-center shadow">
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
          class="bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
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
          class="bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
          @change="fetchLogs"
        >
          <option :value="50">50 {{ t('system.entries', { count: '' }) }}</option>
          <option :value="100">100 {{ t('system.entries', { count: '' }) }}</option>
          <option :value="200">200 {{ t('system.entries', { count: '' }) }}</option>
          <option :value="500">500 {{ t('system.entries', { count: '' }) }}</option>
        </select>
        <div class="flex gap-2">
          <button
            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
            :disabled="logsLoading"
            @click="fetchLogs"
          >
            {{ logsLoading ? t('common.loading') : t('system.refresh') }}
          </button>
          <button
            class="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg transition-colors"
            :disabled="logs.length === 0"
            :title="t('system.exportLogs')"
            @click="exportLogs"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>
          <button
            class="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-red-100 dark:hover:bg-red-600 text-gray-700 dark:text-white rounded-lg transition-colors"
            :disabled="logs.length === 0"
            :title="t('system.clearLogs')"
            @click="clearLogs"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Log Stats -->
      <div class="flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
        <span>{{ t('system.logEntries', { count: logs.length }) }}</span>
        <span v-if="logs.length > 0 && logs[0]?.timestamp">
          {{ t('system.latest') }}: {{ new Date(logs[0].timestamp).toLocaleString() }}
        </span>
      </div>

      <!-- Log Entries -->
      <div class="bg-white dark:bg-gray-800 rounded-lg overflow-hidden shadow">
        <div v-if="logsLoading" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('system.loadingLogs') }}</div>
        <div v-else-if="logs.length === 0" class="p-8 text-center text-gray-500 dark:text-gray-400">
          {{ t('system.noLogsFound') }}
        </div>
        <div v-else class="divide-y divide-gray-200 dark:divide-gray-700 max-h-[600px] overflow-y-auto font-mono text-sm">
          <div
            v-for="(log, index) in logs"
            :key="index"
            class="p-3 hover:bg-gray-50 dark:hover:bg-gray-700/50 flex gap-3 items-start"
          >
            <span class="text-gray-400 dark:text-gray-500 flex-shrink-0 w-20">
              {{ formatLogTime(log.timestamp) }}
            </span>
            <span
              :class="getLogLevelClass(log.level)"
              class="px-2 py-0.5 rounded text-xs uppercase font-medium flex-shrink-0"
            >
              {{ log.level }}
            </span>
            <span v-if="log.source" class="text-purple-600 dark:text-purple-400 flex-shrink-0">[{{ log.source.split('/').pop()?.split(':')[0] }}]</span>
            <!-- Request log with tags -->
            <template v-if="isRequestLog(log)">
              <span
                class="px-2 py-0.5 rounded text-xs font-medium flex-shrink-0"
                :class="getMethodColor(log.fields?.method as string)"
              >
                {{ log.fields?.method }}
              </span>
              <span class="text-gray-300 flex-1 truncate" :title="log.fields?.uri as string">
                {{ log.fields?.uri }}
              </span>
              <span
                class="px-2 py-0.5 rounded text-xs font-medium flex-shrink-0"
                :class="getStatusColor(log.fields?.status as number)"
              >
                {{ log.fields?.status }}
              </span>
              <span class="text-gray-500 text-xs flex-shrink-0">
                {{ formatLatency(log.fields?.latency as number) }}
              </span>
            </template>
            <!-- Regular log message -->
            <span v-else class="text-gray-700 dark:text-gray-300 break-all">{{ log.message }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Config Tab -->
    <div v-if="activeTab === 'config'" class="space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('system.configuration') }}</h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">{{ t('system.configDescription') }}</p>
        </div>
        <div class="flex gap-2">
          <button
            class="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg text-sm transition-colors"
            :title="t('system.formatJson')"
            @click="configJson = JSON.stringify(JSON.parse(configJson), null, 2)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16m-7 6h7" />
            </svg>
          </button>
          <button
            class="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg text-sm transition-colors"
            :title="t('system.copyToClipboard')"
            @click="copyConfigToClipboard"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
          </button>
          <button
            v-if="!configEditing"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm transition-colors"
            @click="configEditing = true"
          >
            {{ t('system.edit') }}
          </button>
          <template v-else>
            <button
              class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded-lg text-sm transition-colors"
              @click="saveConfig"
            >
              {{ t('system.save') }}
            </button>
            <button
              class="px-4 py-2 bg-gray-300 dark:bg-gray-600 hover:bg-gray-400 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg text-sm transition-colors"
              @click="configEditing = false; configJson = JSON.stringify(config, null, 2); configError = null"
            >
              {{ t('system.cancel') }}
            </button>
          </template>
        </div>
      </div>

      <div v-if="configError" class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-4 text-red-700 dark:text-red-300 flex items-start gap-3">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <div>
          <div class="font-medium">{{ t('system.invalidJson') }}</div>
          <div class="text-sm mt-1">{{ configError }}</div>
        </div>
      </div>

      <div class="bg-white dark:bg-gray-800 rounded-lg overflow-hidden shadow">
        <div v-if="configLoading" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('system.loadingConfig') }}</div>
        <div v-else class="relative">
          <!-- Line numbers -->
          <div class="absolute left-0 top-0 bottom-0 w-12 bg-gray-100 dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700 overflow-hidden pointer-events-none">
            <div class="p-4 font-mono text-sm text-gray-400 dark:text-gray-500 leading-6">
              <div v-for="n in configJson.split('\n').length" :key="n">{{ n }}</div>
            </div>
          </div>
          <textarea
            v-model="configJson"
            :readonly="!configEditing"
            class="w-full h-[500px] bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 p-4 pl-16 font-mono text-sm focus:outline-none resize-none leading-6"
            :class="{ 'bg-gray-50 dark:bg-gray-700': configEditing }"
            spellcheck="false"
          ></textarea>
        </div>
      </div>

      <!-- Config Info -->
      <div class="flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
        <span>{{ configJson.split('\n').length }} {{ t('system.lines', { count: '' }) }}</span>
        <span v-if="configEditing" class="text-yellow-600 dark:text-yellow-400">{{ t('system.editingMode') }}</span>
      </div>
    </div>

    <!-- Backup Tab -->
    <div v-if="activeTab === 'backup'" class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('system.backups') }}</h2>
        <button
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
          :disabled="backupCreating"
          @click="createBackup"
        >
          <svg
            v-if="backupCreating"
            class="animate-spin h-4 w-4"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{ backupCreating ? t('system.creating') : t('system.createBackup') }}
        </button>
      </div>

      <div class="bg-white dark:bg-gray-800 rounded-lg overflow-hidden shadow">
        <div v-if="backupsLoading" class="p-8 text-center text-gray-500 dark:text-gray-400">{{ t('system.loadingBackups') }}</div>
        <div v-else-if="backups.length === 0" class="p-8 text-center text-gray-500 dark:text-gray-400">
          {{ t('system.noBackupsFound') }}
        </div>
        <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
          <div
            v-for="backup in backups"
            :key="backup.id"
            class="p-4 flex items-center justify-between"
          >
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
                class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white rounded text-sm transition-colors"
                :disabled="backupRestoring === backup.id"
                @click="restoreBackup(backup.id)"
              >
                {{ backupRestoring === backup.id ? t('system.restoring') : t('system.restore') }}
              </button>
              <button
                class="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white rounded text-sm transition-colors"
                @click="deleteBackup(backup.id)"
              >
                {{ t('system.delete') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Service Tab -->
    <div v-if="activeTab === 'service'" class="space-y-4">
      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
        <ServiceManagement />
      </div>
    </div>
  </div>
</template>
