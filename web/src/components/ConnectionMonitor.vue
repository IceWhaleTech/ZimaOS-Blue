<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getActiveConnections, getConnectionStats, type Connection, type ConnectionStats, type ConnectionType } from '@/api/connections'

const { t } = useI18n()

const connections = ref<Connection[]>([])
const stats = ref<ConnectionStats | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const filterType = ref<ConnectionType | ''>('')
const refreshInterval = ref<ReturnType<typeof setInterval> | null>(null)

// Computed
const filteredConnections = computed(() => {
  if (!filterType.value) return connections.value
  return connections.value.filter(c => c.type === filterType.value)
})

const httpCount = computed(() => stats.value?.active_http ?? 0)
const wsCount = computed(() => stats.value?.active_websocket ?? 0)
const sseCount = computed(() => stats.value?.active_sse ?? 0)

// Methods
async function fetchData() {
  try {
    loading.value = true
    error.value = null
    const [connResponse, statsResponse] = await Promise.all([
      getActiveConnections(filterType.value || undefined),
      getConnectionStats()
    ])
    connections.value = connResponse.connections || []
    stats.value = statsResponse
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to fetch connections'
  } finally {
    loading.value = false
  }
}

function getTypeColor(type: ConnectionType): string {
  const colors: Record<ConnectionType, string> = {
    http: 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300',
    websocket: 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300',
    sse: 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300'
  }
  return colors[type] || 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
}

function getStatusColor(status: string): string {
  const colors: Record<string, string> = {
    active: 'bg-green-500',
    idle: 'bg-yellow-500',
    completed: 'bg-blue-500',
    closed: 'bg-gray-500'
  }
  return colors[status] || 'bg-gray-500'
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function getRelativeTime(dateStr: string): string {
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diff = now - then

  if (diff < 1000) return 'just now'
  if (diff < 60000) return `${Math.floor(diff / 1000)}s ago`
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  return `${Math.floor(diff / 3600000)}h ago`
}

// Lifecycle
onMounted(() => {
  fetchData()
  // Auto-refresh every 3 seconds
  refreshInterval.value = setInterval(fetchData, 3000)
})

onUnmounted(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
  }
})
</script>

<template>
  <div class="connection-monitor">
    <!-- Stats Cards -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
      <div class="p-4 bg-white dark:bg-slate-800 rounded-xl shadow-sm">
        <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ stats?.total_connections ?? 0 }}</div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('connections.total') }}</div>
      </div>
      <div class="p-4 bg-white dark:bg-slate-800 rounded-xl shadow-sm">
        <div class="flex items-center gap-2">
          <span class="w-3 h-3 rounded-full bg-blue-500"></span>
          <span class="text-2xl font-bold text-gray-900 dark:text-white">{{ httpCount }}</span>
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">HTTP</div>
      </div>
      <div class="p-4 bg-white dark:bg-slate-800 rounded-xl shadow-sm">
        <div class="flex items-center gap-2">
          <span class="w-3 h-3 rounded-full bg-green-500"></span>
          <span class="text-2xl font-bold text-gray-900 dark:text-white">{{ wsCount }}</span>
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">WebSocket</div>
      </div>
      <div class="p-4 bg-white dark:bg-slate-800 rounded-xl shadow-sm">
        <div class="flex items-center gap-2">
          <span class="w-3 h-3 rounded-full bg-purple-500"></span>
          <span class="text-2xl font-bold text-gray-900 dark:text-white">{{ sseCount }}</span>
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">SSE</div>
      </div>
    </div>

    <!-- Traffic Stats -->
    <div class="grid grid-cols-2 gap-4 mb-6">
      <div class="p-4 bg-white dark:bg-slate-800 rounded-xl shadow-sm">
        <div class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ formatBytes(stats?.total_bytes_sent ?? 0) }}
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('connections.bytesSent') }}</div>
      </div>
      <div class="p-4 bg-white dark:bg-slate-800 rounded-xl shadow-sm">
        <div class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ formatBytes(stats?.total_bytes_recv ?? 0) }}
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('connections.bytesRecv') }}</div>
      </div>
    </div>

    <!-- Filter -->
    <div class="flex items-center gap-2 mb-4">
      <span class="text-sm text-gray-500 dark:text-slate-400">{{ t('connections.filter') }}:</span>
      <div class="flex bg-gray-100 dark:bg-slate-700 rounded-lg p-1">
        <button
          :class="[
            'px-3 py-1 text-sm rounded-md transition-colors',
            filterType === '' ? 'bg-white dark:bg-slate-600 shadow-sm' : 'hover:bg-gray-200 dark:hover:bg-slate-600'
          ]"
          @click="filterType = ''"
        >
          {{ t('connections.all') }}
        </button>
        <button
          :class="[
            'px-3 py-1 text-sm rounded-md transition-colors',
            filterType === 'http' ? 'bg-white dark:bg-slate-600 shadow-sm' : 'hover:bg-gray-200 dark:hover:bg-slate-600'
          ]"
          @click="filterType = 'http'"
        >
          HTTP
        </button>
        <button
          :class="[
            'px-3 py-1 text-sm rounded-md transition-colors',
            filterType === 'websocket' ? 'bg-white dark:bg-slate-600 shadow-sm' : 'hover:bg-gray-200 dark:hover:bg-slate-600'
          ]"
          @click="filterType = 'websocket'"
        >
          WebSocket
        </button>
        <button
          :class="[
            'px-3 py-1 text-sm rounded-md transition-colors',
            filterType === 'sse' ? 'bg-white dark:bg-slate-600 shadow-sm' : 'hover:bg-gray-200 dark:hover:bg-slate-600'
          ]"
          @click="filterType = 'sse'"
        >
          SSE
        </button>
      </div>
    </div>

    <!-- Connection List -->
    <div class="bg-white dark:bg-slate-800 rounded-xl shadow-sm overflow-hidden">
      <div class="p-4 border-b border-gray-200 dark:border-slate-700">
        <h3 class="font-medium text-gray-900 dark:text-white">
          {{ t('connections.activeConnections') }}
          <span class="text-sm text-gray-500 dark:text-slate-400 ml-2">
            ({{ filteredConnections.length }})
          </span>
        </h3>
      </div>

      <div v-if="loading && connections.length === 0" class="p-8 text-center text-gray-500 dark:text-slate-400">
        {{ t('common.loading') }}
      </div>

      <div v-else-if="error" class="p-8 text-center text-red-500">
        {{ error }}
      </div>

      <div v-else-if="filteredConnections.length === 0" class="p-8 text-center text-gray-500 dark:text-slate-400">
        {{ t('connections.noConnections') }}
      </div>

      <div v-else class="divide-y divide-gray-200 dark:divide-slate-700 max-h-96 overflow-y-auto">
        <div
          v-for="conn in filteredConnections"
          :key="conn.id"
          class="p-4 hover:bg-gray-50 dark:hover:bg-slate-700/50 transition-colors"
        >
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <span :class="['w-2 h-2 rounded-full', getStatusColor(conn.status)]"></span>
              <span :class="['px-2 py-0.5 rounded text-xs font-medium uppercase', getTypeColor(conn.type)]">
                {{ conn.type }}
              </span>
              <span class="text-sm font-mono text-gray-600 dark:text-slate-300">
                {{ conn.method }} {{ conn.path }}
              </span>
            </div>
            <span class="text-xs text-gray-400 dark:text-slate-500">
              {{ getRelativeTime(conn.last_activity) }}
            </span>
          </div>
          <div class="flex items-center gap-4 text-xs text-gray-500 dark:text-slate-400">
            <span>{{ conn.client_ip }}</span>
            <span>{{ conn.request_count }} {{ t('connections.requests') }}</span>
            <span>{{ formatBytes(conn.bytes_sent) }} {{ t('connections.sent') }}</span>
            <span>{{ formatBytes(conn.bytes_recv) }} {{ t('connections.recv') }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
