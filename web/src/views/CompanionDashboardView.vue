<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCompanionStore } from '@/stores/companion'
import { useCompanionStream } from '@/composables/useCompanionStream'
import type { CompanionSession, ThreatLevel, Platform } from '@/api/companion'

const { t } = useI18n()
const companionStore = useCompanionStore()

// WebSocket stream
const { isConnected, eventCount } = useCompanionStream({
  autoConnect: true,
})

// Local state
const selectedSession = ref<CompanionSession | null>(null)
const showSessionDetail = ref(false)
const activeTab = ref<'sessions' | 'alerts' | 'realtime'>('sessions')

// Computed
const stats = computed(() => companionStore.stats)
const sessions = computed(() => companionStore.sortedSessions)
const alerts = computed(() => companionStore.alerts)
const realtimeEvents = computed(() => companionStore.realtimeEvents)
const unackedAlerts = computed(() => companionStore.unacknowledgedAlerts)

onMounted(async () => {
  await Promise.all([
    companionStore.fetchStats(),
    companionStore.fetchSessions(),
    companionStore.fetchAlerts(),
  ])
})

function selectSession(session: CompanionSession) {
  selectedSession.value = session
  showSessionDetail.value = true
  companionStore.fetchSession(session.id)
  companionStore.fetchSessionEvents(session.id)
}

function closeSessionDetail() {
  showSessionDetail.value = false
  selectedSession.value = null
  companionStore.clearCurrentSession()
}

async function acknowledgeAlert(alertId: string) {
  await companionStore.acknowledgeAlert(alertId)
}

function exportData(format: 'json' | 'csv') {
  companionStore.exportData({ format })
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
    active: 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300',
    idle: 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300',
    ended: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300',
    error: 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300',
  }
  return colors[status] ?? 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
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

function getEventTypeIcon(type: string): string {
  const icons: Record<string, string> = {
    session_start: '▶',
    session_end: '■',
    message_received: '←',
    message_sent: '→',
    tool_call: '⚙',
    llm_request: '🤖',
    security_threat: '⚠',
    error: '✕',
  }
  return icons[type] || '•'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}
</script>

<template>
  <div class="companion-view p-4 sm:p-6 max-w-7xl mx-auto">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('companion.title') }}
        </h1>
        <p class="text-sm text-gray-500 dark:text-slate-400 mt-1">
          {{ t('companion.subtitle') }}
        </p>
      </div>
      <div class="flex items-center gap-3">
        <!-- Connection Status -->
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
        </div>
        <!-- Export -->
        <div class="flex gap-2">
          <button
            class="px-3 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
            @click="exportData('csv')"
          >
            {{ t('companion.exportCSV') }}
          </button>
          <button
            class="px-3 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
            @click="exportData('json')"
          >
            {{ t('companion.exportJSON') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Stats Cards -->
    <div v-if="stats" class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-4 mb-6">
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-green-600 dark:text-green-400">
          {{ stats.activeSessions }}
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.activeSessions') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">
          {{ stats.totalSessions }}
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalSessions') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">
          {{ stats.totalEvents }}
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalEvents') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-orange-600 dark:text-orange-400">
          {{ stats.totalAlerts }}
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalAlerts') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-red-600 dark:text-red-400">
          {{ stats.unackedAlerts }}
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.unackedAlerts') }}</div>
      </div>
      <div class="glass-card p-4">
        <div class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ eventCount }}
        </div>
        <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.realtimeEvents') }}</div>
      </div>
    </div>

    <!-- Tabs -->
    <div class="flex gap-2 mb-4 border-b border-gray-200 dark:border-slate-700">
      <button
        :class="[
          'px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
          activeTab === 'sessions'
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
        ]"
        @click="activeTab = 'sessions'"
      >
        {{ t('companion.sessions') }} ({{ sessions.length }})
      </button>
      <button
        :class="[
          'px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
          activeTab === 'alerts'
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
        ]"
        @click="activeTab = 'alerts'"
      >
        {{ t('companion.alerts') }}
        <span v-if="unackedAlerts.length > 0" class="ml-1 px-1.5 py-0.5 bg-red-500 text-white text-xs rounded-full">
          {{ unackedAlerts.length }}
        </span>
      </button>
      <button
        :class="[
          'px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
          activeTab === 'realtime'
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
        ]"
        @click="activeTab = 'realtime'"
      >
        {{ t('companion.realtime') }}
        <span v-if="isConnected()" class="ml-1 w-2 h-2 bg-green-500 rounded-full inline-block animate-pulse" />
      </button>
    </div>

    <!-- Sessions Tab -->
    <div v-if="activeTab === 'sessions'" class="space-y-3">
      <div v-if="companionStore.loading" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="sessions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('companion.noSessions') }}
      </div>
      <div
        v-for="session in sessions"
        :key="session.id"
        class="glass-card p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700/50 transition-colors"
        @click="selectSession(session)"
      >
        <div class="flex items-start justify-between mb-2">
          <div class="flex items-center gap-2">
            <span class="w-8 h-8 flex items-center justify-center bg-gray-200 dark:bg-slate-600 rounded-lg text-sm font-bold">
              {{ getPlatformIcon(session.platform) }}
            </span>
            <div>
              <div class="font-medium text-gray-900 dark:text-white">{{ session.id.slice(0, 8) }}...</div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ session.userId }}</div>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(session.status)]">
              {{ t(`companion.status.${session.status}`) }}
            </span>
            <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getThreatColor(session.threatLevel)]">
              {{ t(`companion.threat.${session.threatLevel}`) }}
            </span>
          </div>
        </div>
        <div class="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-slate-400">
          <span>{{ t('companion.events') }}: {{ session.eventCount }}</span>
          <span>{{ t('companion.started') }}: {{ formatDate(session.startedAt) }}</span>
          <span v-if="session.duration">{{ t('companion.duration') }}: {{ formatDuration(session.duration) }}</span>
        </div>
      </div>

      <button
        v-if="companionStore.hasMoreSessions"
        class="w-full py-2 text-sm text-accent hover:text-accent-hover"
        @click="companionStore.loadMoreSessions()"
      >
        {{ t('common.loadMore') }}
      </button>
    </div>

    <!-- Alerts Tab -->
    <div v-if="activeTab === 'alerts'" class="space-y-3">
      <div v-if="companionStore.loadingAlerts" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="alerts.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('companion.noAlerts') }}
      </div>
      <div
        v-for="alert in alerts"
        :key="alert.id"
        :class="[
          'glass-card p-4',
          alert.acknowledged ? 'opacity-60' : ''
        ]"
      >
        <div class="flex items-start justify-between mb-2">
          <div>
            <div class="font-medium text-gray-900 dark:text-white">{{ alert.title }}</div>
            <div class="text-sm text-gray-500 dark:text-slate-400">{{ alert.description }}</div>
          </div>
          <div class="flex items-center gap-2">
            <span
              :class="[
                'px-2 py-0.5 rounded-full text-xs font-medium',
                alert.severity === 'critical' ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' :
                alert.severity === 'error' ? 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300' :
                alert.severity === 'warning' ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300' :
                'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300'
              ]"
            >
              {{ t(`companion.alerts.${alert.severity}`) }}
            </span>
            <button
              v-if="!alert.acknowledged"
              class="px-2 py-1 bg-accent hover:bg-accent-hover text-white text-xs rounded transition-colors"
              @click.stop="acknowledgeAlert(alert.id)"
            >
              {{ t('companion.acknowledge') }}
            </button>
          </div>
        </div>
        <div class="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-slate-400">
          <span>{{ t('companion.session') }}: {{ alert.sessionId.slice(0, 8) }}...</span>
          <span>{{ formatDate(alert.createdAt) }}</span>
          <span v-if="alert.acknowledged">{{ t('companion.ackedBy') }}: {{ alert.ackedBy }}</span>
        </div>
      </div>

      <button
        v-if="companionStore.hasMoreAlerts"
        class="w-full py-2 text-sm text-accent hover:text-accent-hover"
        @click="companionStore.loadMoreAlerts()"
      >
        {{ t('common.loadMore') }}
      </button>
    </div>

    <!-- Realtime Tab -->
    <div v-if="activeTab === 'realtime'" class="space-y-2">
      <div v-if="realtimeEvents.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('companion.waitingForEvents') }}
      </div>
      <div
        v-for="event in realtimeEvents"
        :key="event.id"
        class="glass-card p-3 text-sm"
      >
        <div class="flex items-center gap-2 mb-1">
          <span class="text-lg">{{ getEventTypeIcon(event.eventType) }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ t(`companion.eventType.${event.eventType}`) }}</span>
          <span class="text-xs text-gray-400 dark:text-slate-500">{{ event.sessionId.slice(0, 8) }}...</span>
          <span class="ml-auto text-xs text-gray-400 dark:text-slate-500">
            {{ formatDate(event.timestamp) }}
          </span>
        </div>
        <div v-if="event.message" class="text-xs text-gray-500 dark:text-slate-400 truncate">
          {{ t(`companion.message.${event.message.direction}`) }}: {{ event.message.content }}
        </div>
        <div v-if="event.toolCall" class="text-xs text-gray-500 dark:text-slate-400">
          {{ event.toolCall.toolName }} - {{ event.toolCall.status }} ({{ formatDuration(event.toolCall.duration) }})
        </div>
        <div v-if="event.llmRequest" class="text-xs text-gray-500 dark:text-slate-400">
          {{ event.llmRequest.provider }}/{{ event.llmRequest.model }} - {{ event.llmRequest.totalTokens }} tokens
        </div>
        <div v-if="event.security" :class="['text-xs', getThreatColor(event.security.threatLevel)]">
          {{ t(`companion.threat.${event.security.threatLevel}`) }}: {{ event.security.threatTypes.join(', ') }}
        </div>
      </div>
    </div>

    <!-- Session Detail Modal -->
    <div
      v-if="showSessionDetail && selectedSession"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeSessionDetail"
    >
      <div class="bg-white dark:bg-slate-800 rounded-xl max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col">
        <!-- Modal Header -->
        <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-slate-700">
          <div>
            <h2 class="text-lg font-bold text-gray-900 dark:text-white">
              {{ t('companion.sessionDetail') }}
            </h2>
            <p class="text-sm text-gray-500 dark:text-slate-400">{{ selectedSession.id }}</p>
          </div>
          <button
            class="p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors"
            @click="closeSessionDetail"
          >
            <span class="text-xl">&times;</span>
          </button>
        </div>

        <!-- Modal Content -->
        <div class="flex-1 overflow-y-auto p-4">
          <!-- Session Info -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.platform') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ selectedSession.platform }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.status') }}</div>
              <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(selectedSession.status)]">
                {{ t(`companion.status.${selectedSession.status}`) }}
              </span>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.threatLevel') }}</div>
              <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getThreatColor(selectedSession.threatLevel)]">
                {{ t(`companion.threat.${selectedSession.threatLevel}`) }}
              </span>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.events') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ selectedSession.eventCount }}</div>
            </div>
          </div>

          <!-- Events List -->
          <h3 class="text-sm font-medium text-gray-900 dark:text-white mb-3">{{ t('companion.eventHistory') }}</h3>
          <div v-if="companionStore.loadingEvents" class="text-center py-4 text-gray-500 dark:text-slate-400">
            {{ t('common.loading') }}
          </div>
          <div v-else class="space-y-2">
            <div
              v-for="event in companionStore.sessionEvents"
              :key="event.id"
              class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg text-sm"
            >
              <div class="flex items-center gap-2 mb-1">
                <span>{{ getEventTypeIcon(event.eventType) }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ t(`companion.eventType.${event.eventType}`) }}</span>
                <span class="ml-auto text-xs text-gray-400 dark:text-slate-500">
                  {{ formatDate(event.timestamp) }}
                </span>
              </div>
              <div v-if="event.message" class="text-xs text-gray-500 dark:text-slate-400">
                {{ t(`companion.message.${event.message.direction}`) }}: {{ event.message.content }}
              </div>
              <div v-if="event.toolCall" class="text-xs text-gray-500 dark:text-slate-400">
                {{ event.toolCall.toolName }} - {{ event.toolCall.status }}
              </div>
              <div v-if="event.llmRequest" class="text-xs text-gray-500 dark:text-slate-400">
                {{ event.llmRequest.provider }}/{{ event.llmRequest.model }}
              </div>
            </div>
          </div>

          <button
            v-if="companionStore.hasMoreEvents"
            class="w-full py-2 text-sm text-accent hover:text-accent-hover mt-4"
            @click="companionStore.loadMoreEvents()"
          >
            {{ t('common.loadMore') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
