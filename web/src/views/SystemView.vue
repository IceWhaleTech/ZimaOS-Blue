<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useSystemStore } from '@/stores/system'
import { systemApi, backupApi } from '@/api/index'
import type { LogEntry, SystemMetrics } from '@/api/system'
import type { BackupInfo } from '@/api/index'

const systemStore = useSystemStore()

// Tabs
const activeTab = ref<'overview' | 'logs' | 'config' | 'backup'>('overview')

// Metrics
const metricsHistory = ref<SystemMetrics[]>([])
const autoRefresh = ref(true)
let refreshInterval: ReturnType<typeof setInterval> | null = null

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

// Computed
const memoryUsagePercent = computed(() => {
  if (!systemStore.health) return 0
  // Approximate based on typical system memory
  return Math.min(100, (systemStore.health.mem_alloc_bytes / (512 * 1024 * 1024)) * 100)
})

const cpuUsagePercent = computed(() => {
  // This would need actual CPU metrics from the backend
  return 0
})

onMounted(async () => {
  await systemStore.fetchAll()
  await fetchMetricsHistory()

  refreshInterval = setInterval(() => {
    if (autoRefresh.value) {
      systemStore.fetchAll()
      if (activeTab.value === 'overview') {
        fetchMetricsHistory()
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
  if (!confirm('Are you sure you want to restore this backup? Current data will be overwritten.')) {
    return
  }

  backupRestoring.value = id
  try {
    await backupApi.restore(id)
    alert('Backup restored successfully. The service may restart.')
  } catch {
    alert('Failed to restore backup')
  } finally {
    backupRestoring.value = null
  }
}

async function deleteBackup(id: string) {
  if (!confirm('Are you sure you want to delete this backup?')) {
    return
  }

  try {
    await backupApi.delete(id)
    backups.value = backups.value.filter((b) => b.id !== id)
  } catch {
    alert('Failed to delete backup')
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
  return new Date(dateStr).toLocaleString()
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

// Load data when tab changes
function switchTab(tab: 'overview' | 'logs' | 'config' | 'backup') {
  activeTab.value = tab
  if (tab === 'logs' && logs.value.length === 0) {
    fetchLogs()
  } else if (tab === 'config' && Object.keys(config.value).length === 0) {
    fetchConfig()
  } else if (tab === 'backup' && backups.value.length === 0) {
    fetchBackups()
  }
}
</script>

<template>
  <div class="system-view p-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-white">System</h1>
      <label class="flex items-center space-x-2">
        <input
          v-model="autoRefresh"
          type="checkbox"
          class="rounded border-gray-600 bg-gray-700 text-blue-600 focus:ring-blue-500"
        />
        <span class="text-sm text-gray-400">Auto refresh (5s)</span>
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
    <div class="flex border-b border-gray-700 mb-6">
      <button
        v-for="tab in ['overview', 'logs', 'config', 'backup'] as const"
        :key="tab"
        class="px-4 py-2 text-sm font-medium transition-colors"
        :class="
          activeTab === tab
            ? 'text-blue-400 border-b-2 border-blue-400'
            : 'text-gray-400 hover:text-white'
        "
        @click="switchTab(tab)"
      >
        {{ tab.charAt(0).toUpperCase() + tab.slice(1) }}
      </button>
    </div>

    <!-- Overview Tab -->
    <div v-if="activeTab === 'overview'" class="space-y-6">
      <!-- Stats Grid -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div class="bg-gray-800 rounded-lg p-4">
          <div class="text-sm text-gray-400 mb-1">Status</div>
          <div
            class="text-xl font-bold"
            :class="systemStore.health?.status === 'ok' ? 'text-green-400' : 'text-red-400'"
          >
            {{ systemStore.health?.status || 'Unknown' }}
          </div>
        </div>
        <div class="bg-gray-800 rounded-lg p-4">
          <div class="text-sm text-gray-400 mb-1">Uptime</div>
          <div class="text-xl font-bold text-white">
            {{ systemStore.health?.uptime || '-' }}
          </div>
        </div>
        <div class="bg-gray-800 rounded-lg p-4">
          <div class="text-sm text-gray-400 mb-1">Memory</div>
          <div class="text-xl font-bold text-white">
            {{ systemStore.health ? formatBytes(systemStore.health.mem_alloc_bytes) : '-' }}
          </div>
        </div>
        <div class="bg-gray-800 rounded-lg p-4">
          <div class="text-sm text-gray-400 mb-1">Goroutines</div>
          <div class="text-xl font-bold text-white">
            {{ systemStore.health?.goroutines || '-' }}
          </div>
        </div>
      </div>

      <!-- Resource Usage -->
      <div class="grid md:grid-cols-2 gap-6">
        <!-- Memory Usage -->
        <div class="bg-gray-800 rounded-lg p-6">
          <h3 class="text-lg font-semibold text-white mb-4">Memory Usage</h3>
          <div class="space-y-4">
            <div>
              <div class="flex justify-between text-sm mb-1">
                <span class="text-gray-400">Allocated</span>
                <span class="text-white">{{ memoryUsagePercent.toFixed(1) }}%</span>
              </div>
              <div class="w-full bg-gray-700 rounded-full h-3">
                <div
                  class="bg-blue-600 h-3 rounded-full transition-all"
                  :style="{ width: `${memoryUsagePercent}%` }"
                ></div>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span class="text-gray-400">Alloc:</span>
                <span class="text-white ml-2">
                  {{ systemStore.health ? formatBytes(systemStore.health.mem_alloc_bytes) : '-' }}
                </span>
              </div>
              <div>
                <span class="text-gray-400">CPUs:</span>
                <span class="text-white ml-2">{{ systemStore.health?.num_cpu || '-' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Worker Pool -->
        <div class="bg-gray-800 rounded-lg p-6">
          <h3 class="text-lg font-semibold text-white mb-4">Worker Pool</h3>
          <div v-if="systemStore.workerStats" class="space-y-4">
            <div>
              <div class="flex justify-between text-sm mb-1">
                <span class="text-gray-400">Pool Usage</span>
                <span class="text-white">
                  {{ systemStore.workerStats.running }}/{{ systemStore.workerStats.pool_size }}
                </span>
              </div>
              <div class="w-full bg-gray-700 rounded-full h-3">
                <div
                  class="bg-green-600 h-3 rounded-full transition-all"
                  :style="{
                    width: `${(systemStore.workerStats.running / systemStore.workerStats.pool_size) * 100}%`,
                  }"
                ></div>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span class="text-gray-400">Running:</span>
                <span class="text-white ml-2">{{ systemStore.workerStats.running }}</span>
              </div>
              <div>
                <span class="text-gray-400">Total Tasks:</span>
                <span class="text-white ml-2">{{ systemStore.workerStats.total }}</span>
              </div>
            </div>
          </div>
          <div v-else class="text-gray-400">Loading...</div>
        </div>
      </div>

      <!-- System Info -->
      <div class="bg-gray-800 rounded-lg p-6">
        <h3 class="text-lg font-semibold text-white mb-4">System Information</h3>
        <div class="grid md:grid-cols-3 gap-4 text-sm">
          <div>
            <span class="text-gray-400">Version:</span>
            <span class="text-white ml-2">{{ systemStore.health?.version || '-' }}</span>
          </div>
          <div>
            <span class="text-gray-400">Go Version:</span>
            <span class="text-white ml-2">{{ systemStore.health?.go_version || '-' }}</span>
          </div>
          <div>
            <span class="text-gray-400">Timestamp:</span>
            <span class="text-white ml-2">
              {{ systemStore.health?.timestamp ? formatDate(systemStore.health.timestamp) : '-' }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Logs Tab -->
    <div v-if="activeTab === 'logs'" class="space-y-4">
      <!-- Filters -->
      <div class="bg-gray-800 rounded-lg p-4 flex flex-wrap gap-4">
        <div class="flex-1 min-w-[200px]">
          <input
            v-model="logSearch"
            type="text"
            placeholder="Search logs..."
            class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @keyup.enter="fetchLogs"
          />
        </div>
        <select
          v-model="logLevel"
          class="bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @change="fetchLogs"
        >
          <option value="all">All Levels</option>
          <option value="error">Error</option>
          <option value="warn">Warning</option>
          <option value="info">Info</option>
          <option value="debug">Debug</option>
        </select>
        <select
          v-model="logLimit"
          class="bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @change="fetchLogs"
        >
          <option :value="50">50 entries</option>
          <option :value="100">100 entries</option>
          <option :value="200">200 entries</option>
          <option :value="500">500 entries</option>
        </select>
        <button
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
          :disabled="logsLoading"
          @click="fetchLogs"
        >
          {{ logsLoading ? 'Loading...' : 'Refresh' }}
        </button>
      </div>

      <!-- Log Entries -->
      <div class="bg-gray-800 rounded-lg overflow-hidden">
        <div v-if="logsLoading" class="p-8 text-center text-gray-400">Loading logs...</div>
        <div v-else-if="logs.length === 0" class="p-8 text-center text-gray-400">
          No logs found
        </div>
        <div v-else class="divide-y divide-gray-700 max-h-[600px] overflow-y-auto font-mono text-sm">
          <div
            v-for="(log, index) in logs"
            :key="index"
            class="p-3 hover:bg-gray-700/50 flex gap-3"
          >
            <span class="text-gray-500 flex-shrink-0 w-20">
              {{ new Date(log.timestamp).toLocaleTimeString() }}
            </span>
            <span
              :class="getLogLevelClass(log.level)"
              class="px-2 py-0.5 rounded text-xs uppercase font-medium flex-shrink-0"
            >
              {{ log.level }}
            </span>
            <span v-if="log.source" class="text-purple-400 flex-shrink-0">[{{ log.source }}]</span>
            <span class="text-gray-300 break-all">{{ log.message }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Config Tab -->
    <div v-if="activeTab === 'config'" class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">Configuration</h2>
        <div class="flex gap-2">
          <button
            v-if="!configEditing"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm transition-colors"
            @click="configEditing = true"
          >
            Edit
          </button>
          <template v-else>
            <button
              class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded-lg text-sm transition-colors"
              @click="saveConfig"
            >
              Save
            </button>
            <button
              class="px-4 py-2 bg-gray-600 hover:bg-gray-500 text-white rounded-lg text-sm transition-colors"
              @click="
                configEditing = false
                configJson = JSON.stringify(config, null, 2)
                configError = null
              "
            >
              Cancel
            </button>
          </template>
        </div>
      </div>

      <div v-if="configError" class="bg-red-900/30 border border-red-800 rounded-lg p-4 text-red-300">
        {{ configError }}
      </div>

      <div class="bg-gray-800 rounded-lg overflow-hidden">
        <div v-if="configLoading" class="p-8 text-center text-gray-400">Loading configuration...</div>
        <textarea
          v-else
          v-model="configJson"
          :readonly="!configEditing"
          class="w-full h-[500px] bg-gray-800 text-gray-300 p-4 font-mono text-sm focus:outline-none resize-none"
          :class="{ 'bg-gray-700': configEditing }"
        ></textarea>
      </div>
    </div>

    <!-- Backup Tab -->
    <div v-if="activeTab === 'backup'" class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">Backups</h2>
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
          {{ backupCreating ? 'Creating...' : 'Create Backup' }}
        </button>
      </div>

      <div class="bg-gray-800 rounded-lg overflow-hidden">
        <div v-if="backupsLoading" class="p-8 text-center text-gray-400">Loading backups...</div>
        <div v-else-if="backups.length === 0" class="p-8 text-center text-gray-400">
          No backups found. Create one to get started.
        </div>
        <div v-else class="divide-y divide-gray-700">
          <div
            v-for="backup in backups"
            :key="backup.id"
            class="p-4 flex items-center justify-between"
          >
            <div>
              <div class="text-white font-medium">{{ backup.id }}</div>
              <div class="text-sm text-gray-400 flex items-center gap-4 mt-1">
                <span>{{ formatDate(backup.created_at) }}</span>
                <span>{{ formatBytes(backup.size_bytes) }}</span>
                <span class="text-blue-400">{{ backup.type }}</span>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <button
                class="px-3 py-1.5 bg-green-600 hover:bg-green-700 text-white rounded text-sm transition-colors"
                :disabled="backupRestoring === backup.id"
                @click="restoreBackup(backup.id)"
              >
                {{ backupRestoring === backup.id ? 'Restoring...' : 'Restore' }}
              </button>
              <button
                class="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white rounded text-sm transition-colors"
                @click="deleteBackup(backup.id)"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
