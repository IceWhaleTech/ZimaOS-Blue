<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { CompanionSession, ThreatLevel, Platform } from '@/api/companion'

const { t } = useI18n()

const props = defineProps<{
  sessions: CompanionSession[]
  loading?: boolean
  hasMore?: boolean
  selectedId?: string | null
}>()

const emit = defineEmits<{
  select: [session: CompanionSession]
  loadMore: []
}>()

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

function getPlatformColor(platform: Platform): string {
  const colors: Record<Platform, string> = {
    whatsapp: 'bg-green-500',
    telegram: 'bg-blue-500',
    discord: 'bg-indigo-500',
    slack: 'bg-purple-500',
    matrix: 'bg-teal-500',
    feishu: 'bg-blue-600',
    web: 'bg-gray-500',
    api: 'bg-orange-500',
  }
  return colors[platform] || 'bg-gray-500'
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now.getTime() - date.getTime()

  // Less than 1 minute
  if (diff < 60000) {
    return t('companion.justNow')
  }
  // Less than 1 hour
  if (diff < 3600000) {
    const mins = Math.floor(diff / 60000)
    return t('companion.minutesAgo', { n: mins })
  }
  // Less than 24 hours
  if (diff < 86400000) {
    const hours = Math.floor(diff / 3600000)
    return t('companion.hoursAgo', { n: hours })
  }
  // Otherwise show date
  return date.toLocaleDateString()
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}
</script>

<template>
  <div class="session-list">
    <!-- Loading State -->
    <div v-if="loading && sessions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
      {{ t('common.loading') }}
    </div>

    <!-- Empty State -->
    <div v-else-if="sessions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
      {{ t('companion.noSessions') }}
    </div>

    <!-- Session List -->
    <div v-else class="space-y-3">
      <div
        v-for="session in sessions"
        :key="session.id"
        :class="[
          'glass-card p-4 cursor-pointer transition-all',
          selectedId === session.id
            ? 'ring-2 ring-accent bg-accent/5'
            : 'hover:bg-gray-50 dark:hover:bg-slate-700/50'
        ]"
        @click="emit('select', session)"
      >
        <div class="flex items-start justify-between mb-2">
          <!-- Platform & User -->
          <div class="flex items-center gap-3">
            <span
              :class="[
                'w-10 h-10 flex items-center justify-center rounded-lg text-white font-bold',
                getPlatformColor(session.platform)
              ]"
            >
              {{ getPlatformIcon(session.platform) }}
            </span>
            <div>
              <div class="font-medium text-gray-900 dark:text-white">
                {{ session.user_id || t('companion.anonymous') }}
              </div>
              <div class="text-xs text-gray-500 dark:text-slate-400 font-mono">
                {{ session.id.slice(0, 8) }}...
              </div>
            </div>
          </div>

          <!-- Status Badges -->
          <div class="flex items-center gap-2">
            <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(session.status)]">
              {{ t(`companion.status.${session.status}`) }}
            </span>
            <span
              v-if="session.threat_level !== 'none'"
              :class="['px-2 py-0.5 rounded-full text-xs font-medium', getThreatColor(session.threat_level)]"
            >
              {{ t(`companion.threat.${session.threat_level}`) }}
            </span>
          </div>
        </div>

        <!-- Session Stats -->
        <div class="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-slate-400">
          <span class="flex items-center gap-1">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            {{ session.event_count }} {{ t('companion.events') }}
          </span>
          <span class="flex items-center gap-1">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {{ formatDate(session.started_at) }}
          </span>
          <span v-if="session.duration" class="flex items-center gap-1">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
            {{ formatDuration(session.duration) }}
          </span>
          <span v-if="session.metadata?.message_count" class="flex items-center gap-1">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            {{ session.metadata.message_count }} {{ t('companion.messages') }}
          </span>
        </div>

        <!-- Active Indicator -->
        <div v-if="session.status === 'active'" class="mt-2 flex items-center gap-2">
          <span class="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
          <span class="text-xs text-green-600 dark:text-green-400">{{ t('companion.liveSession') }}</span>
        </div>
      </div>

      <!-- Load More -->
      <button
        v-if="hasMore"
        class="w-full py-3 text-sm text-accent hover:text-accent-hover transition-colors"
        :disabled="loading"
        @click="emit('loadMore')"
      >
        <span v-if="loading">{{ t('common.loading') }}</span>
        <span v-else>{{ t('common.loadMore') }}</span>
      </button>
    </div>
  </div>
</template>
