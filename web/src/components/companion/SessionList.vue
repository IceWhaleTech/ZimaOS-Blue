<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { CompanionSession, ThreatLevel, Platform } from '@/api/companion'

const { t } = useI18n()

defineProps<{
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

function getPlatformName(platform: Platform | string): string {
  return t(`companion.platforms.${platform}`, platform)
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
            <!-- Channel icon with platform color -->
            <div
              :class="[
                'w-10 h-10 flex items-center justify-center rounded-lg',
                getPlatformColor(session.platform)
              ]"
            >
              <!-- WhatsApp -->
              <svg v-if="session.platform === 'whatsapp'" class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 24 24">
                <path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413z"/>
              </svg>
              <!-- Telegram -->
              <svg v-else-if="session.platform === 'telegram'" class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 24 24">
                <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z"/>
              </svg>
              <!-- Discord -->
              <svg v-else-if="session.platform === 'discord'" class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 24 24">
                <path d="M20.317 4.3698a19.7913 19.7913 0 00-4.8851-1.5152.0741.0741 0 00-.0785.0371c-.211.3753-.4447.8648-.6083 1.2495-1.8447-.2762-3.68-.2762-5.4868 0-.1636-.3933-.4058-.8742-.6177-1.2495a.077.077 0 00-.0785-.037 19.7363 19.7363 0 00-4.8852 1.515.0699.0699 0 00-.0321.0277C.5334 9.0458-.319 13.5799.0992 18.0578a.0824.0824 0 00.0312.0561c2.0528 1.5076 4.0413 2.4228 5.9929 3.0294a.0777.0777 0 00.0842-.0276c.4616-.6304.8731-1.2952 1.226-1.9942a.076.076 0 00-.0416-.1057c-.6528-.2476-1.2743-.5495-1.8722-.8923a.077.077 0 01-.0076-.1277c.1258-.0943.2517-.1923.3718-.2914a.0743.0743 0 01.0776-.0105c3.9278 1.7933 8.18 1.7933 12.0614 0a.0739.0739 0 01.0785.0095c.1202.099.246.1981.3728.2924a.077.077 0 01-.0066.1276 12.2986 12.2986 0 01-1.873.8914.0766.0766 0 00-.0407.1067c.3604.698.7719 1.3628 1.225 1.9932a.076.076 0 00.0842.0286c1.961-.6067 3.9495-1.5219 6.0023-3.0294a.077.077 0 00.0313-.0552c.5004-5.177-.8382-9.6739-3.5485-13.6604a.061.061 0 00-.0312-.0286zM8.02 15.3312c-1.1825 0-2.1569-1.0857-2.1569-2.419 0-1.3332.9555-2.4189 2.157-2.4189 1.2108 0 2.1757 1.0952 2.1568 2.419 0 1.3332-.9555 2.4189-2.1569 2.4189zm7.9748 0c-1.1825 0-2.1569-1.0857-2.1569-2.419 0-1.3332.9554-2.4189 2.1569-2.4189 1.2108 0 2.1757 1.0952 2.1568 2.419 0 1.3332-.946 2.4189-2.1568 2.4189z"/>
              </svg>
              <!-- Slack -->
              <svg v-else-if="session.platform === 'slack'" class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 24 24">
                <path d="M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zM6.313 15.165a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.834 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.834 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.834zM8.834 6.313a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.528 2.528 0 0 1 0 8.834a2.528 2.528 0 0 1 2.522-2.521h6.312zM18.956 8.834a2.528 2.528 0 0 1 2.522-2.521A2.528 2.528 0 0 1 24 8.834a2.528 2.528 0 0 1-2.522 2.521h-2.522V8.834zM17.688 8.834a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.165 0a2.528 2.528 0 0 1 2.523 2.522v6.312zM15.165 18.956a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.165 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zM15.165 17.688a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z"/>
              </svg>
              <!-- Matrix -->
              <svg v-else-if="session.platform === 'matrix'" class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 24 24">
                <path d="M.632.55v22.9H2.28V24H0V0h2.28v.55zm7.043 7.26v1.157h.033c.309-.443.683-.784 1.117-1.024.433-.245.936-.365 1.5-.365.54 0 1.033.107 1.481.314.448.208.785.582 1.02 1.108.254-.374.6-.706 1.034-.992.434-.287.95-.43 1.546-.43.453 0 .872.056 1.26.167.388.11.716.286.993.53.276.245.489.559.646.951.152.392.23.863.23 1.417v5.728h-2.349V11.52c0-.286-.01-.559-.032-.812a1.755 1.755 0 00-.18-.66 1.106 1.106 0 00-.438-.448c-.194-.11-.457-.166-.785-.166-.332 0-.6.064-.803.189a1.38 1.38 0 00-.48.499 1.946 1.946 0 00-.231.696 5.56 5.56 0 00-.06.785v4.768h-2.35v-4.8c0-.254-.004-.503-.018-.752a2.074 2.074 0 00-.143-.688 1.052 1.052 0 00-.415-.503c-.194-.125-.476-.19-.854-.19-.111 0-.259.024-.439.074-.18.051-.36.143-.53.282-.171.138-.319.33-.439.576-.12.245-.18.567-.18.958v5.043H4.833V7.81zm14.693 14.39v-22.9H21.72V0H24v24h-2.28v-.55z"/>
              </svg>
              <!-- Feishu -->
              <svg v-else-if="session.platform === 'feishu'" class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 24 24">
                <path d="M6.134 3.158c-.156-.104-.364.052-.312.234l1.82 6.316c.052.182.286.234.416.104l4.316-4.316c.13-.13.078-.364-.104-.416L6.134 3.158zM3.158 6.134c-.182-.052-.338.156-.234.312l1.922 6.136c.052.13.234.182.364.052l4.316-4.316c.13-.13.078-.234-.052-.364L3.158 6.134zm17.684 11.732c.156.104.364-.052.312-.234l-1.82-6.316c-.052-.182-.286-.234-.416-.104l-4.316 4.316c-.13.13-.078.364.104.416l6.136 1.922zm2.976-3.732c.182.052.338-.156.234-.312l-1.922-6.136c-.052-.13-.234-.182-.364-.052l-4.316 4.316c-.13.13-.078.234.052.364l6.316 1.82zM12 8.5L8.5 12 12 15.5 15.5 12 12 8.5z"/>
              </svg>
              <!-- Web -->
              <svg v-else-if="session.platform === 'web' || session.platform === 'web-user'" class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"/>
              </svg>
              <!-- API -->
              <svg v-else-if="session.platform === 'api'" class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"/>
              </svg>
              <!-- Default fallback -->
              <svg v-else class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/>
              </svg>
            </div>
            <div>
              <div class="font-medium text-gray-900 dark:text-white">
                {{ session.user_id || t('companion.anonymous') }}
              </div>
              <div class="text-xs text-gray-500 dark:text-slate-400">
                {{ getPlatformName(session.platform) }} · <span class="font-mono">{{ session.id.slice(0, 8) }}...</span>
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
