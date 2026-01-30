<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyApi, type Session, type SessionStats } from '@/api/proxy'

const { t } = useI18n()

const sessions = ref<Session[]>([])
const stats = ref<SessionStats | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const refreshInterval = ref<number | null>(null)
const showActiveOnly = ref(false)
const selectedSession = ref<string | null>(null)

// Computed
const filteredSessions = computed(() => {
  if (showActiveOnly.value) {
    return sessions.value.filter((s) => s.status === 'active')
  }
  return sessions.value
})

const activeSessions = computed(() => sessions.value.filter((s) => s.status === 'active').length)
const totalSessions = computed(() => sessions.value.length)

const statusColor = (status: string) => {
  switch (status) {
    case 'active':
      return 'text-green-600 bg-green-100 dark:text-green-400 dark:bg-green-900/30'
    case 'completed':
      return 'text-blue-600 bg-blue-100 dark:text-blue-400 dark:bg-blue-900/30'
    case 'error':
      return 'text-red-600 bg-red-100 dark:text-red-400 dark:bg-red-900/30'
    case 'timeout':
      return 'text-yellow-600 bg-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30'
    default:
      return 'text-gray-600 bg-gray-100 dark:text-gray-400 dark:bg-gray-900/30'
  }
}

const formatDuration = (ms: number) => {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}

const formatTime = (timestamp: string) => {
  return new Date(timestamp).toLocaleTimeString()
}

const truncateId = (id: string) => {
  if (id.length <= 12) return id
  return id.substring(0, 8) + '...'
}

// Methods
async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const res = await proxyApi.getSessions(showActiveOnly.value)
    sessions.value = res.data.sessions || []
    stats.value = res.data.stats || null
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to fetch sessions'
    console.error('Failed to fetch sessions:', e)
  } finally {
    loading.value = false
  }
}

async function fetchSessionDetails(sessionId: string) {
  try {
    const res = await proxyApi.getSession(sessionId)
    const index = sessions.value.findIndex((s) => s.id === sessionId)
    if (index !== -1) {
      sessions.value[index] = res.data
    }
  } catch (e) {
    console.error('Failed to fetch session details:', e)
  }
}

function toggleActiveFilter() {
  showActiveOnly.value = !showActiveOnly.value
  fetchData()
}

function selectSession(id: string) {
  if (selectedSession.value === id) {
    selectedSession.value = null
  } else {
    selectedSession.value = id
    fetchSessionDetails(id)
  }
}

function startAutoRefresh() {
  refreshInterval.value = window.setInterval(fetchData, 5000)
}

function stopAutoRefresh() {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
    refreshInterval.value = null
  }
}

onMounted(() => {
  fetchData()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<template>
  <div class="session-monitor">
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Session Monitor</h2>
      <div class="flex items-center gap-2">
        <button
          @click="toggleActiveFilter"
          :class="showActiveOnly ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'"
          class="px-3 py-1.5 text-sm rounded-md transition-colors"
        >
          {{ showActiveOnly ? 'Active Only' : 'All Sessions' }}
        </button>
        <button
          @click="fetchData"
          :disabled="loading"
          class="px-3 py-1.5 text-sm bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 rounded-md transition-colors disabled:opacity-50"
        >
          {{ loading ? 'Refreshing...' : 'Refresh' }}
        </button>
      </div>
    </div>

    <!-- Error -->
    <div
      v-if="error"
      class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400"
    >
      {{ error }}
    </div>

    <!-- Stats Overview -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
      <div class="p-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
        <div class="text-2xl font-bold text-green-600 dark:text-green-400">
          {{ activeSessions }}
        </div>
        <div class="text-sm text-green-600/70 dark:text-green-400/70">Active</div>
      </div>
      <div class="p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
        <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">
          {{ totalSessions }}
        </div>
        <div class="text-sm text-blue-600/70 dark:text-blue-400/70">Total</div>
      </div>
      <div class="p-3 bg-purple-50 dark:bg-purple-900/20 rounded-lg">
        <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">
          {{ stats?.avg_duration_ms ? formatDuration(stats.avg_duration_ms) : '-' }}
        </div>
        <div class="text-sm text-purple-600/70 dark:text-purple-400/70">Avg Duration</div>
      </div>
      <div class="p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
        <div class="text-2xl font-bold text-gray-600 dark:text-gray-400">
          {{ stats?.total_requests ?? 0 }}
        </div>
        <div class="text-sm text-gray-600/70 dark:text-gray-400/70">Total Requests</div>
      </div>
    </div>

    <!-- Sessions List -->
    <div class="space-y-2">
      <div v-if="filteredSessions.length === 0 && !loading" class="text-center py-8 text-gray-500 dark:text-gray-400">
        {{ showActiveOnly ? 'No active sessions' : 'No sessions found' }}
      </div>

      <div
        v-for="session in filteredSessions"
        :key="session.id"
        class="p-3 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 cursor-pointer hover:border-blue-300 dark:hover:border-blue-600 transition-colors"
        :class="{ 'border-blue-500 dark:border-blue-500': selectedSession === session.id }"
        @click="selectSession(session.id)"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center gap-3">
            <span class="text-lg">{{ session.status === 'active' ? '🟢' : session.status === 'error' ? '🔴' : '⚪' }}</span>
            <div>
              <div class="flex items-center gap-2">
                <span class="font-mono text-sm text-gray-900 dark:text-white">{{ truncateId(session.id) }}</span>
                <span
                  :class="statusColor(session.status)"
                  class="px-2 py-0.5 text-xs font-medium rounded-full uppercase"
                >
                  {{ session.status }}
                </span>
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                <span v-if="session.provider">{{ session.provider }}</span>
                <span v-if="session.model" class="ml-2">{{ session.model }}</span>
              </div>
            </div>
          </div>
          <div class="text-right text-sm">
            <div class="text-gray-900 dark:text-white">{{ formatTime(session.created_at) }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ session.duration_ms ? formatDuration(session.duration_ms) : 'ongoing' }}
            </div>
          </div>
        </div>

        <!-- Expanded Details -->
        <div v-if="selectedSession === session.id" class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700">
          <div class="grid grid-cols-2 md:grid-cols-3 gap-3 text-sm">
            <div>
              <span class="text-gray-500 dark:text-gray-400">Request ID:</span>
              <span class="ml-1 font-mono text-xs text-gray-900 dark:text-white">{{ session.request_id || '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">Client IP:</span>
              <span class="ml-1 text-gray-900 dark:text-white">{{ session.client_ip || '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">Input Tokens:</span>
              <span class="ml-1 text-gray-900 dark:text-white">{{ session.input_tokens ?? '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">Output Tokens:</span>
              <span class="ml-1 text-gray-900 dark:text-white">{{ session.output_tokens ?? '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">Streaming:</span>
              <span class="ml-1 text-gray-900 dark:text-white">{{ session.streaming ? 'Yes' : 'No' }}</span>
            </div>
            <div v-if="session.error">
              <span class="text-gray-500 dark:text-gray-400">Error:</span>
              <span class="ml-1 text-red-600">{{ session.error }}</span>
            </div>
          </div>

          <!-- Messages Preview -->
          <div v-if="session.messages && session.messages.length > 0" class="mt-3">
            <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">Messages ({{ session.messages.length }})</div>
            <div class="max-h-32 overflow-y-auto bg-gray-50 dark:bg-gray-900 rounded p-2 text-xs font-mono">
              <div v-for="(msg, idx) in session.messages.slice(0, 3)" :key="idx" class="mb-1">
                <span class="text-blue-600 dark:text-blue-400">{{ msg.role }}:</span>
                <span class="text-gray-700 dark:text-gray-300 ml-1">{{ msg.content?.substring(0, 100) }}{{ msg.content?.length > 100 ? '...' : '' }}</span>
              </div>
              <div v-if="session.messages.length > 3" class="text-gray-400">
                ... and {{ session.messages.length - 3 }} more messages
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.session-monitor {
  @apply p-4;
}
</style>
