<script setup lang="ts">
import { ref, computed, watch, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCompanionStore } from '@/stores/companion'
import type { CompanionSession, ThreatLevel, SessionEvent } from '@/api/companion'

// Lazy load heavy component
const SessionFlowCanvas = defineAsyncComponent(() => import('./SessionFlowCanvas.vue'))

const { t } = useI18n()
const companionStore = useCompanionStore()

const props = defineProps<{
  session: CompanionSession
}>()

const emit = defineEmits<{
  close: []
}>()

// View mode: 'timeline' or 'flow'
const viewMode = ref<'timeline' | 'flow'>('flow')
const selectedEventId = ref<string | undefined>(undefined)

// Load session details when session changes
watch(
  () => props.session.id,
  (id) => {
    if (id) {
      companionStore.fetchSession(id)
      companionStore.fetchSessionEvents(id)
      companionStore.fetchSessionFlow(id)
    }
  },
  { immediate: true }
)

const events = computed(() => companionStore.sessionEvents)
const loading = computed(() => companionStore.loadingSession || companionStore.loadingEvents)

// Handle node click in flow canvas
function handleNodeClick(event: SessionEvent) {
  selectedEventId.value = event.id
}

function handleNodeHover(_event: SessionEvent | null) {
  // Could show additional info on hover
}

function getThreatColor(level: ThreatLevel): string {
  const colors: Record<ThreatLevel, string> = {
    none: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300',
    low: 'bg-gray-700 dark:bg-gray-500/50 text-gray-900 dark:text-white dark:text-white',
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
    success: 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300',
  }
  return colors[status] ?? 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
}

function getEventIcon(type: string): string {
  const icons: Record<string, string> = {
    session_start: '▶️',
    session_end: '⏹️',
    message_received: '📥',
    message_sent: '📤',
    tool_call: '🔧',
    llm_request: '🤖',
    security_threat: '⚠️',
    error: '❌',
    custom: '📌',
  }
  return icons[type] || '•'
}

function getEventColor(type: string): string {
  const colors: Record<string, string> = {
    session_start: 'border-l-green-500',
    session_end: 'border-l-gray-500',
    message_received: 'border-l-gray-900 dark:border-l-gray-400',
    message_sent: 'border-l-indigo-500',
    tool_call: 'border-l-purple-500',
    llm_request: 'border-l-orange-500',
    security_threat: 'border-l-red-500',
    error: 'border-l-red-600',
    custom: 'border-l-gray-400',
  }
  return colors[type] || 'border-l-gray-400'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function formatTime(dateStr: string): string {
  return new Date(dateStr).toLocaleTimeString()
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}
</script>

<template>
  <div class="session-detail bg-white dark:bg-slate-800 rounded-xl shadow-lg overflow-hidden flex flex-col h-full">
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-slate-700">
      <div>
        <h2 class="text-lg font-bold text-gray-900 dark:text-white">
          {{ t('companion.sessionDetail') }}
        </h2>
        <p class="text-sm text-gray-500 dark:text-slate-400 font-mono">{{ session.id }}</p>
      </div>
      <button
        class="p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors"
        @click="emit('close')"
      >
        <svg class="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-4 flex flex-col min-h-0">
      <!-- Session Info Grid -->
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('companion.platform') }}</div>
          <div class="font-medium text-gray-900 dark:text-white capitalize">{{ session.platform }}</div>
        </div>
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('companion.statusLabel') }}</div>
          <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(session.status)]">
            {{ t(`companion.status.${session.status}`) }}
          </span>
        </div>
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('companion.threat_level') }}</div>
          <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getThreatColor(session.threat_level)]">
            {{ t(`companion.threat.${session.threat_level}`) }}
          </span>
        </div>
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('companion.threat_score') }}</div>
          <div class="font-medium text-gray-900 dark:text-white">{{ session.threat_score }}</div>
        </div>
      </div>

      <!-- Session Metadata -->
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('companion.events') }}</div>
          <div class="font-medium text-gray-900 dark:text-white">{{ session.event_count }}</div>
        </div>
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('companion.messages') }}</div>
          <div class="font-medium text-gray-900 dark:text-white">{{ session.metadata?.message_count || 0 }}</div>
        </div>
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('companion.toolCalls') }}</div>
          <div class="font-medium text-gray-900 dark:text-white">{{ session.metadata?.tool_call_count || 0 }}</div>
        </div>
        <div class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('companion.tokens') }}</div>
          <div class="font-medium text-gray-900 dark:text-white">{{ session.metadata?.total_tokens || 0 }}</div>
        </div>
      </div>

      <!-- Time Info -->
      <div class="flex flex-wrap gap-4 text-sm text-gray-500 dark:text-slate-400 mb-6">
        <span>{{ t('companion.started') }}: {{ formatDate(session.started_at) }}</span>
        <span v-if="session.ended_at">{{ t('companion.ended') }}: {{ formatDate(session.ended_at) }}</span>
        <span v-if="session.duration">{{ t('companion.duration') }}: {{ formatDuration(session.duration) }}</span>
      </div>

      <!-- View Mode Toggle -->
      <div class="flex items-center gap-2 mb-4">
        <div class="flex bg-gray-100 dark:bg-slate-700 rounded-lg p-1">
          <button
            :class="[
              'px-3 py-1.5 text-sm font-medium rounded-md transition-colors',
              viewMode === 'flow'
                ? 'bg-white dark:bg-slate-600 text-gray-900 dark:text-white shadow-sm'
                : 'text-gray-600 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white'
            ]"
            @click="viewMode = 'flow'"
          >
            <svg class="w-4 h-4 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" />
            </svg>
            {{ t('companion.viewMode.flow') }}
          </button>
          <button
            :class="[
              'px-3 py-1.5 text-sm font-medium rounded-md transition-colors',
              viewMode === 'timeline'
                ? 'bg-white dark:bg-slate-600 text-gray-900 dark:text-white shadow-sm'
                : 'text-gray-600 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white'
            ]"
            @click="viewMode = 'timeline'"
          >
            <svg class="w-4 h-4 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
            </svg>
            {{ t('companion.viewMode.timeline') }}
          </button>
        </div>
      </div>

      <!-- Flow View -->
      <div v-if="viewMode === 'flow'" class="flex-1 min-h-0">
        <div class="h-full border border-gray-200 dark:border-slate-700 rounded-lg overflow-hidden">
          <SessionFlowCanvas
            :events="events"
            :selected-event-id="selectedEventId"
            :auto-scroll="true"
            @node-click="handleNodeClick"
            @node-hover="handleNodeHover"
          />
        </div>
      </div>

      <!-- Events Timeline -->
      <div v-else class="mb-6">
        <h3 class="text-sm font-medium text-gray-900 dark:text-white mb-3">
          {{ t('companion.eventHistory') }}
        </h3>

        <div v-if="loading" class="text-center py-4 text-gray-500 dark:text-slate-400">
          {{ t('common.loading') }}
        </div>

        <div v-else-if="events.length === 0" class="text-center py-4 text-gray-500 dark:text-slate-400">
          {{ t('companion.noEvents') }}
        </div>

        <div v-else class="space-y-2">
          <div
            v-for="event in events"
            :key="event.id"
            :class="[
              'p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg border-l-4',
              getEventColor(event.event_type)
            ]"
          >
            <div class="flex items-center gap-2 mb-1">
              <span class="text-base">{{ getEventIcon(event.event_type) }}</span>
              <span class="font-medium text-gray-900 dark:text-white text-sm">
                {{ t(`companion.eventType.${event.event_type}`) }}
              </span>
              <span
                v-if="event.status"
                :class="['px-1.5 py-0.5 rounded text-xs', getStatusColor(event.status)]"
              >
                {{ event.status }}
              </span>
              <span class="ml-auto text-xs text-gray-400 dark:text-slate-500">
                {{ formatTime(event.timestamp) }}
              </span>
            </div>

            <!-- Message Event -->
            <div v-if="event.message" class="text-sm text-gray-600 dark:text-slate-300">
              <div class="flex items-center gap-2 mb-1">
                <span class="text-xs px-1.5 py-0.5 bg-gray-200 dark:bg-slate-600 rounded">
                  {{ t('companion.direction.' + event.message.direction) }}
                </span>
                <span class="text-xs text-gray-400">{{ event.message.contentType }}</span>
              </div>
              <div class="text-xs text-gray-500 dark:text-slate-400 truncate">
                {{ event.message.content }}
              </div>
            </div>

            <!-- Tool Call Event -->
            <div v-if="event.tool_call" class="text-sm text-gray-600 dark:text-slate-300">
              <div class="flex items-center gap-2 mb-1">
                <span class="font-mono text-xs">{{ event.tool_call.toolName }}</span>
                <span v-if="event.tool_call.sandboxUsed" class="text-xs px-1.5 py-0.5 bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300 rounded">
                  sandbox
                </span>
                <span class="text-xs text-gray-400">{{ formatDuration(event.tool_call.duration) }}</span>
              </div>
              <div v-if="event.tool_call.inputPreview" class="text-xs text-gray-500 dark:text-slate-400 truncate">
                {{ t('companion.llmDetails.input') }}: {{ event.tool_call.inputPreview }}
              </div>
            </div>

            <!-- LLM Request Event -->
            <div v-if="event.llm_request" class="text-sm text-gray-600 dark:text-slate-300">
              <div class="flex items-center gap-2 mb-1">
                <span class="font-mono text-xs">{{ event.llm_request.provider }}/{{ event.llm_request.model }}</span>
                <span class="text-xs text-gray-400">{{ formatDuration(event.llm_request.duration) }}</span>
              </div>
              <div class="flex gap-3 text-xs text-gray-500 dark:text-slate-400">
                <span>{{ t('companion.llmDetails.prompt') }}: {{ event.llm_request.promptTokens }}</span>
                <span>{{ t('companion.llmDetails.completion') }}: {{ event.llm_request.completionTokens }}</span>
                <span>{{ t('companion.llmDetails.total') }}: {{ event.llm_request.totalTokens }}</span>
              </div>
            </div>

            <!-- Security Event -->
            <div v-if="event.security" class="text-sm">
              <div class="flex items-center gap-2 mb-1">
                <span :class="['px-1.5 py-0.5 rounded text-xs', getThreatColor(event.security.threatLevel)]">
                  {{ event.security.threatLevel }}
                </span>
                <span class="text-xs text-gray-400">{{ t('companion.llmDetails.score') }}: {{ event.security.threatScore }}</span>
              </div>
              <div class="text-xs text-gray-500 dark:text-slate-400">
                {{ event.security.threatTypes.join(', ') }}
              </div>
              <div v-if="event.security.details" class="text-xs text-gray-500 dark:text-slate-400 mt-1">
                {{ event.security.details }}
              </div>
            </div>

            <!-- Error Event -->
            <div v-if="event.error" class="text-sm text-red-600 dark:text-red-400">
              {{ event.error }}
            </div>
          </div>

          <!-- Load More -->
          <button
            v-if="companionStore.hasMoreEvents"
            class="w-full py-2 text-sm text-gray-900 dark:text-gray-300 hover:text-gray-900 dark:text-gray-300-hover"
            @click="companionStore.loadMoreEvents()"
          >
            {{ t('common.loadMore') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
