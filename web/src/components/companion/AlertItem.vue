<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Alert, AlertSeverity } from '@/api/companion'

const { t } = useI18n()

const props = defineProps<{
  alert: Alert
  compact?: boolean
}>()

const emit = defineEmits<{
  acknowledge: [id: string]
  viewSession: [sessionId: string]
}>()

const severityConfig = computed(() => {
  const configs: Record<AlertSeverity, { icon: string; color: string; bg: string }> = {
    critical: {
      icon: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
      color: 'text-red-600 dark:text-red-400',
      bg: 'bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-700',
    },
    high: {
      icon: 'M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
      color: 'text-orange-600 dark:text-orange-400',
      bg: 'bg-orange-50 dark:bg-orange-900/30 border-orange-200 dark:border-orange-700',
    },
    warning: {
      icon: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
      color: 'text-yellow-600 dark:text-yellow-400',
      bg: 'bg-yellow-50 dark:bg-yellow-900/30 border-yellow-200 dark:border-yellow-700',
    },
    info: {
      icon: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
      color: 'text-gray-900 dark:text-white dark:text-gray-900 dark:text-white',
      bg: 'bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/30 border-gray-900 dark:border-white dark:border-gray-900 dark:border-white',
    },
    error: {
      icon: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
      color: 'text-red-600 dark:text-red-400',
      bg: 'bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-700',
    },
  }
  return configs[props.alert.severity] || configs.info
})

function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  const now = new Date()
  const diff = now.getTime() - date.getTime()

  if (diff < 60000) {
    return t('companion.justNow')
  } else if (diff < 3600000) {
    const mins = Math.floor(diff / 60000)
    return t('companion.minutesAgo', { n: mins })
  } else if (diff < 86400000) {
    const hours = Math.floor(diff / 3600000)
    return t('companion.hoursAgo', { n: hours })
  }

  return date.toLocaleString()
}
</script>

<template>
  <div
    :class="[
      'rounded-lg border p-4 transition-all',
      severityConfig.bg,
      alert.acknowledged ? 'opacity-60' : '',
      compact ? 'p-3' : 'p-4'
    ]"
  >
    <div class="flex items-start gap-3">
      <!-- Severity Icon -->
      <div :class="['flex-shrink-0 p-2 rounded-full bg-white dark:bg-gray-700', severityConfig.color]">
        <svg :class="compact ? 'w-4 h-4' : 'w-5 h-5'" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="severityConfig.icon" />
        </svg>
      </div>

      <!-- Content -->
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2 mb-1">
          <span
            :class="[
              'px-2 py-0.5 text-xs font-medium rounded-full uppercase',
              severityConfig.color,
              'bg-white/50 dark:bg-gray-700/50'
            ]"
          >
            {{ alert.severity }}
          </span>
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ formatTime(alert.timestamp) }}
          </span>
          <span
            v-if="alert.acknowledged"
            class="px-2 py-0.5 text-xs bg-green-100 dark:bg-green-900/50 text-green-600 dark:text-green-400 rounded-full"
          >
            {{ t('companion.alerts.acknowledged') }}
          </span>
        </div>

        <h4 :class="['font-medium text-gray-900 dark:text-white', compact ? 'text-sm' : 'text-base']">
          {{ alert.title }}
        </h4>

        <p v-if="alert.description && !compact" class="mt-1 text-sm text-gray-600 dark:text-gray-400">
          {{ alert.description }}
        </p>

        <!-- Session Link -->
        <div v-if="alert.session_id" class="mt-2 flex items-center gap-2">
          <button
            class="text-xs text-gray-900 dark:text-gray-300 hover:underline flex items-center gap-1"
            @click="emit('viewSession', alert.session_id)"
          >
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
            </svg>
            {{ t('companion.alerts.viewSession') }}
          </button>
        </div>

        <!-- Acked info -->
        <div v-if="alert.acknowledged && alert.acked_by" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('companion.ackedBy') }}: {{ alert.acked_by }}
        </div>
      </div>

      <!-- Actions -->
      <div v-if="!alert.acknowledged" class="flex-shrink-0">
        <button
          class="px-3 py-1.5 text-sm bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
          @click="emit('acknowledge', alert.id)"
        >
          {{ t('companion.acknowledge') }}
        </button>
      </div>
    </div>
  </div>
</template>
