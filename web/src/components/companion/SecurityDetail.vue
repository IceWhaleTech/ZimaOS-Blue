<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import SecurityBadge from './SecurityBadge.vue'
import type { SecurityData } from '@/api/companion'

const { t } = useI18n()

defineProps<{
  security: SecurityData
}>()

const emit = defineEmits<{
  close: []
}>()

function getActionColor(action: string): string {
  switch (action) {
    case 'blocked':
      return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
    case 'filtered':
      return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
    case 'allowed':
      return 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
    default:
      return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  }
}

function getThreatTypeIcon(type: string): string {
  const icons: Record<string, string> = {
    injection: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
    xss: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4',
    sql_injection: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4',
    path_traversal: 'M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z',
    command_injection: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z',
    prompt_injection: 'M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z',
    brute_force: 'M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z',
    rate_limit: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z',
  }
  return icons[type] || 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z'
}
</script>

<template>
  <div class="bg-white dark:bg-gray-700 rounded-lg shadow-lg overflow-hidden">
    <!-- Header -->
    <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('companion.security.title') }}
      </h3>
      <button
        class="p-1 rounded-lg text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
        @click="emit('close')"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Content -->
    <div class="p-4 space-y-4">
      <!-- Threat Level and Score -->
      <div class="flex items-center justify-between">
        <div>
          <div class="text-sm text-gray-500 dark:text-gray-400 mb-1">
            {{ t('companion.security.threatLevel') }}
          </div>
          <SecurityBadge :level="security.threatLevel" :score="security.threatScore" show-score size="lg" />
        </div>
        <div class="text-right">
          <div class="text-sm text-gray-500 dark:text-gray-400 mb-1">
            {{ t('companion.security.action') }}
          </div>
          <span
            :class="[
              'inline-flex items-center px-3 py-1.5 rounded-full text-sm font-medium capitalize',
              getActionColor(security.action)
            ]"
          >
            {{ security.action }}
          </span>
        </div>
      </div>

      <!-- Threat Score Bar -->
      <div>
        <div class="flex items-center justify-between text-sm mb-1">
          <span class="text-gray-500 dark:text-gray-400">{{ t('companion.security.score') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ security.threatScore }}/100</span>
        </div>
        <div class="h-2 bg-gray-700 dark:bg-gray-500 rounded-full overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-300"
            :class="{
              'bg-green-500': security.threatScore < 25,
              'bg-yellow-500': security.threatScore >= 25 && security.threatScore < 50,
              'bg-orange-500': security.threatScore >= 50 && security.threatScore < 75,
              'bg-red-500': security.threatScore >= 75,
            }"
            :style="{ width: `${security.threatScore}%` }"
          />
        </div>
      </div>

      <!-- Threat Types -->
      <div v-if="security.threatTypes?.length">
        <div class="text-sm text-gray-500 dark:text-gray-400 mb-2">
          {{ t('companion.security.threatTypes') }}
        </div>
        <div class="space-y-2">
          <div
            v-for="type in security.threatTypes"
            :key="type"
            class="flex items-center gap-3 p-2 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
          >
            <div class="p-1.5 bg-red-100 dark:bg-red-900/50 rounded-lg">
              <svg class="w-4 h-4 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getThreatTypeIcon(type)" />
              </svg>
            </div>
            <span class="text-sm text-gray-700 dark:text-gray-300 capitalize">
              {{ type.replace(/_/g, ' ') }}
            </span>
          </div>
        </div>
      </div>

      <!-- Detected Patterns (if available from details) -->
      <div v-if="security.details">
        <div class="text-sm text-gray-500 dark:text-gray-400 mb-2">
          {{ t('companion.security.details') }}
        </div>
        <div class="p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
          <pre class="text-xs text-gray-600 dark:text-gray-400 whitespace-pre-wrap break-words">{{ security.details }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>
