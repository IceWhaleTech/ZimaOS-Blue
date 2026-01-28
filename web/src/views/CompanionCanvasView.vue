<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCompanionStore } from '@/stores/companion'
import { useCompanionStream } from '@/composables/useCompanionStream'
import FlowViewer from '@/components/companion/FlowViewer.vue'
import type { CompanionSession, ThreatLevel, Platform, FlowNode } from '@/api/companion'

const { t } = useI18n()
const companionStore = useCompanionStore()

// WebSocket stream
const { isConnected, connect, disconnect } = useCompanionStream({
  autoConnect: true,
})

// UI State
const sidebarCollapsed = ref(false)
const selectedSessionId = ref<string | null>(null)
const selectedNode = ref<FlowNode | null>(null)
const showNodeDetail = ref(false)
const searchQuery = ref('')
const platformFilter = ref<Platform | ''>('')

// Computed
const sessions = computed(() => companionStore.sortedSessions)
const filteredSessions = computed(() => {
  let result = sessions.value

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(s =>
      s.id.toLowerCase().includes(query) ||
      s.userId.toLowerCase().includes(query)
    )
  }

  if (platformFilter.value) {
    result = result.filter(s => s.platform === platformFilter.value)
  }

  return result
})

const activeSessions = computed(() =>
  sessions.value.filter(s => s.status === 'active')
)

const currentSession = computed(() =>
  sessions.value.find(s => s.id === selectedSessionId.value)
)

// Lifecycle
onMounted(async () => {
  await companionStore.fetchSessions()
  // Auto-select first active session
  if (activeSessions.value.length > 0) {
    selectSession(activeSessions.value[0].id)
  } else if (sessions.value.length > 0) {
    selectSession(sessions.value[0].id)
  }
})

onUnmounted(() => {
  disconnect()
})

// Methods
function selectSession(sessionId: string) {
  selectedSessionId.value = sessionId
  selectedNode.value = null
  showNodeDetail.value = false
  companionStore.fetchSession(sessionId)
  companionStore.fetchSessionFlow(sessionId)
}

function onNodeClick(node: FlowNode) {
  selectedNode.value = node
  showNodeDetail.value = true
}

function closeNodeDetail() {
  showNodeDetail.value = false
  selectedNode.value = null
}

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
}

function getThreatColor(level: ThreatLevel): string {
  const colors: Record<ThreatLevel, string> = {
    none: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300',
    low: 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300',
    medium: 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300',
    high: 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300',
    critical: 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300',
  }
  return colors[level] || colors.none
}

function getStatusColor(status: string): string {
  const colors: Record<string, string> = {
    active: 'bg-green-500',
    idle: 'bg-yellow-500',
    ended: 'bg-gray-400',
    error: 'bg-red-500',
  }
  return colors[status] ?? 'bg-gray-400'
}

function getPlatformIcon(platform: Platform): string {
  const icons: Record<Platform, string> = {
    whatsapp: 'W',
    telegram: 'T',
    discord: 'D',
    slack: 'S',
    matrix: 'M',
    feishu: 'F',
    web: 'W',
    api: 'A',
  }
  return icons[platform] || '?'
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleTimeString()
}

// Watch for new sessions from realtime stream
watch(() => companionStore.realtimeEvents, (events) => {
  if (events.length > 0) {
    const latestEvent = events[0]
    // If viewing the session that got a new event, refresh the flow
    if (latestEvent.sessionId === selectedSessionId.value) {
      companionStore.fetchSessionFlow(selectedSessionId.value)
    }
  }
}, { deep: true })
</script>

<template>
  <div class="companion-canvas h-screen flex overflow-hidden bg-gray-100 dark:bg-gray-900">
    <!-- Sidebar -->
    <aside
      :class="[
        'flex flex-col bg-white dark:bg-slate-800 border-r border-gray-200 dark:border-slate-700 transition-all duration-300',
        sidebarCollapsed ? 'w-16' : 'w-80'
      ]"
    >
      <!-- Sidebar Header -->
      <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-slate-700">
        <div v-if="!sidebarCollapsed" class="flex items-center gap-2">
          <svg class="w-5 h-5 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" />
          </svg>
          <span class="font-semibold text-gray-900 dark:text-white">{{ t('companion.canvas.title') }}</span>
        </div>
        <button
          class="p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors"
          @click="toggleSidebar"
        >
          <svg
            :class="['w-5 h-5 text-gray-500 transition-transform', sidebarCollapsed ? 'rotate-180' : '']"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
          </svg>
        </button>
      </div>

      <!-- Connection Status -->
      <div v-if="!sidebarCollapsed" class="px-4 py-2 border-b border-gray-200 dark:border-slate-700">
        <div class="flex items-center gap-2">
          <span
            :class="[
              'w-2 h-2 rounded-full',
              isConnected() ? 'bg-green-500 animate-pulse' : 'bg-gray-400'
            ]"
          />
          <span class="text-xs text-gray-500 dark:text-slate-400">
            {{ isConnected() ? t('companion.connected') : t('companion.disconnected') }}
          </span>
          <span class="ml-auto text-xs text-gray-400">
            {{ activeSessions.length }} {{ t('companion.canvas.active') }}
          </span>
        </div>
      </div>

      <!-- Search & Filter -->
      <div v-if="!sidebarCollapsed" class="p-3 border-b border-gray-200 dark:border-slate-700 space-y-2">
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="t('companion.canvas.searchSessions')"
          class="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-accent"
        />
        <select
          v-model="platformFilter"
          class="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-slate-700 border border-gray-200 dark:border-slate-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-accent"
        >
          <option value="">{{ t('companion.canvas.allPlatforms') }}</option>
          <option value="whatsapp">WhatsApp</option>
          <option value="telegram">Telegram</option>
          <option value="discord">Discord</option>
          <option value="slack">Slack</option>
          <option value="web">Web</option>
          <option value="api">API</option>
        </select>
      </div>

      <!-- Session List -->
      <div class="flex-1 overflow-y-auto">
        <div v-if="companionStore.loading" class="p-4 text-center text-gray-500 dark:text-slate-400">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="filteredSessions.length === 0" class="p-4 text-center text-gray-500 dark:text-slate-400">
          {{ t('companion.noSessions') }}
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-slate-700">
          <button
            v-for="session in filteredSessions"
            :key="session.id"
            :class="[
              'w-full text-left transition-colors',
              sidebarCollapsed ? 'p-3 flex justify-center' : 'p-3 hover:bg-gray-50 dark:hover:bg-slate-700/50',
              selectedSessionId === session.id ? 'bg-accent/10 border-l-2 border-accent' : ''
            ]"
            @click="selectSession(session.id)"
          >
            <!-- Collapsed view -->
            <div v-if="sidebarCollapsed" class="relative">
              <span
                :class="[
                  'w-8 h-8 flex items-center justify-center rounded-lg text-sm font-bold',
                  selectedSessionId === session.id
                    ? 'bg-accent text-white'
                    : 'bg-gray-200 dark:bg-slate-600 text-gray-700 dark:text-gray-300'
                ]"
              >
                {{ getPlatformIcon(session.platform) }}
              </span>
              <span
                :class="[
                  'absolute -top-1 -right-1 w-3 h-3 rounded-full border-2 border-white dark:border-slate-800',
                  getStatusColor(session.status)
                ]"
              />
            </div>

            <!-- Expanded view -->
            <div v-else class="flex items-start gap-3">
              <div class="relative flex-shrink-0">
                <span class="w-10 h-10 flex items-center justify-center bg-gray-200 dark:bg-slate-600 rounded-lg text-sm font-bold">
                  {{ getPlatformIcon(session.platform) }}
                </span>
                <span
                  :class="[
                    'absolute -bottom-1 -right-1 w-3 h-3 rounded-full border-2 border-white dark:border-slate-800',
                    getStatusColor(session.status)
                  ]"
                />
              </div>
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white truncate">
                    {{ session.id.slice(0, 8) }}
                  </span>
                  <span :class="['px-1.5 py-0.5 text-[10px] rounded-full font-medium', getThreatColor(session.threatLevel)]">
                    {{ session.threatLevel }}
                  </span>
                </div>
                <div class="text-xs text-gray-500 dark:text-slate-400 truncate">
                  {{ session.userId }}
                </div>
                <div class="flex items-center gap-2 mt-1 text-xs text-gray-400 dark:text-slate-500">
                  <span>{{ session.eventCount }} {{ t('companion.events') }}</span>
                  <span v-if="session.duration">{{ formatDuration(session.duration) }}</span>
                </div>
              </div>
            </div>
          </button>
        </div>

        <!-- Load More -->
        <button
          v-if="companionStore.hasMoreSessions && !sidebarCollapsed"
          class="w-full py-3 text-sm text-accent hover:text-accent-hover border-t border-gray-200 dark:border-slate-700"
          @click="companionStore.loadMoreSessions()"
        >
          {{ t('companion.loadMore') }}
        </button>
      </div>
    </aside>

    <!-- Main Canvas Area -->
    <main class="flex-1 flex flex-col overflow-hidden">
      <!-- Canvas Header -->
      <header class="flex items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-gray-200 dark:border-slate-700">
        <div v-if="currentSession" class="flex items-center gap-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('companion.canvas.sessionFlow') }}
            </h2>
            <p class="text-sm text-gray-500 dark:text-slate-400">
              {{ currentSession.id }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <span :class="['px-2 py-1 text-xs rounded-full font-medium', getThreatColor(currentSession.threatLevel)]">
              {{ currentSession.threatLevel }}
            </span>
            <span class="text-sm text-gray-500 dark:text-slate-400">
              {{ currentSession.eventCount }} {{ t('companion.events') }}
            </span>
          </div>
        </div>
        <div v-else class="text-gray-500 dark:text-slate-400">
          {{ t('companion.canvas.selectSession') }}
        </div>

        <!-- Canvas Actions -->
        <div class="flex items-center gap-2">
          <button
            v-if="currentSession"
            class="px-3 py-2 text-sm bg-gray-100 dark:bg-slate-700 hover:bg-gray-200 dark:hover:bg-slate-600 rounded-lg transition-colors"
            @click="companionStore.fetchSessionFlow(currentSession.id)"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
        </div>
      </header>

      <!-- Flow Canvas -->
      <div class="flex-1 relative">
        <div v-if="!selectedSessionId" class="absolute inset-0 flex items-center justify-center">
          <div class="text-center">
            <svg class="mx-auto h-16 w-16 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" />
            </svg>
            <p class="mt-4 text-lg text-gray-500 dark:text-slate-400">{{ t('companion.canvas.selectSession') }}</p>
            <p class="mt-2 text-sm text-gray-400 dark:text-slate-500">{{ t('companion.canvas.selectSessionHint') }}</p>
          </div>
        </div>

        <FlowViewer
          v-else
          :session-id="selectedSessionId"
          class="h-full"
          @node-click="onNodeClick"
        />
      </div>
    </main>

    <!-- Node Detail Panel -->
    <aside
      v-if="showNodeDetail && selectedNode"
      class="w-80 bg-white dark:bg-slate-800 border-l border-gray-200 dark:border-slate-700 flex flex-col"
    >
      <!-- Panel Header -->
      <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-slate-700">
        <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('companion.canvas.nodeDetail') }}</h3>
        <button
          class="p-1 hover:bg-gray-100 dark:hover:bg-slate-700 rounded transition-colors"
          @click="closeNodeDetail"
        >
          <svg class="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Panel Content -->
      <div class="flex-1 overflow-y-auto p-4 space-y-4">
        <!-- Node Type -->
        <div>
          <label class="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
            {{ t('companion.canvas.nodeType') }}
          </label>
          <div class="mt-1 flex items-center gap-2">
            <span
              :class="[
                'px-2 py-1 text-sm rounded-lg font-medium',
                selectedNode.type === 'message' ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' :
                selectedNode.type === 'tool_call' ? 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300' :
                selectedNode.type === 'llm_request' ? 'bg-indigo-100 dark:bg-indigo-900/50 text-indigo-700 dark:text-indigo-300' :
                selectedNode.type === 'security_check' ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' :
                'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
              ]"
            >
              {{ selectedNode.type }}
            </span>
          </div>
        </div>

        <!-- Node Label -->
        <div>
          <label class="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
            {{ t('companion.canvas.label') }}
          </label>
          <p class="mt-1 text-sm text-gray-900 dark:text-white">{{ selectedNode.label }}</p>
        </div>

        <!-- Timestamp -->
        <div>
          <label class="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
            {{ t('companion.canvas.timestamp') }}
          </label>
          <p class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatDate(selectedNode.timestamp) }}</p>
        </div>

        <!-- Duration -->
        <div v-if="selectedNode.duration">
          <label class="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
            {{ t('companion.canvas.duration') }}
          </label>
          <p class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatDuration(selectedNode.duration) }}</p>
        </div>

        <!-- Status -->
        <div>
          <label class="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
            {{ t('companion.canvas.status') }}
          </label>
          <div class="mt-1">
            <span
              :class="[
                'px-2 py-1 text-xs rounded-full font-medium',
                selectedNode.status === 'completed' || selectedNode.status === 'success'
                  ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                  : selectedNode.status === 'failed' || selectedNode.status === 'error'
                    ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
              ]"
            >
              {{ selectedNode.status }}
            </span>
          </div>
        </div>

        <!-- Additional Data -->
        <div v-if="selectedNode.data && Object.keys(selectedNode.data).length > 0">
          <label class="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
            {{ t('companion.canvas.details') }}
          </label>
          <div class="mt-2 space-y-2">
            <div
              v-for="(value, key) in selectedNode.data"
              :key="key"
              class="p-2 bg-gray-50 dark:bg-slate-700/50 rounded-lg"
            >
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ key }}</div>
              <div class="text-sm text-gray-900 dark:text-white break-all">
                {{ typeof value === 'object' ? JSON.stringify(value, null, 2) : value }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.companion-canvas {
  /* Ensure full viewport height */
  height: 100vh;
  height: 100dvh;
}
</style>
