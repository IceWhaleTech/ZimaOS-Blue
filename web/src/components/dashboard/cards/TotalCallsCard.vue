<script setup lang="ts">
import { computed } from 'vue'
import { useMetricsStore } from '@/stores/metrics'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const metricsStore = useMetricsStore()

const stats = computed(() => {
  const summary = metricsStore.summary
  if (!summary) return null
  return {
    totalCalls: summary.calls?.total_calls ?? 0,
    successRate: summary.calls?.success_rate ?? 0,
  }
})

function formatNumber(num: number): string {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toFixed(0)
}
</script>

<template>
  <div>
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('metrics.totalCalls') }}</p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ stats ? formatNumber(stats.totalCalls) : '-' }}
        </p>
      </div>
      <div class="p-3 bg-blue-100 dark:bg-blue-900/30 rounded-full">
        <svg class="w-6 h-6 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
        </svg>
      </div>
    </div>
    <div class="mt-2 flex items-center text-sm">
      <span :class="stats && stats.successRate >= 95 ? 'text-green-500' : 'text-orange-500'">
        {{ stats ? stats.successRate.toFixed(1) : '-' }}%
      </span>
      <span class="ml-1 text-gray-500 dark:text-gray-400">{{ t('metrics.successRate') }}</span>
    </div>
  </div>
</template>
