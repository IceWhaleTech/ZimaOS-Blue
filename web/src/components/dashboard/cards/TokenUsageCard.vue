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
    totalTokens: summary.tokens?.total_tokens ?? 0,
    estimatedCost: summary.tokens?.estimated_cost ?? 0,
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
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('metrics.tokenUsage') }}</p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ stats ? formatNumber(stats.totalTokens) : '-' }}
        </p>
      </div>
      <div class="p-3 bg-green-100 dark:bg-green-900/30 rounded-full">
        <svg class="w-6 h-6 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
        </svg>
      </div>
    </div>
    <div class="mt-2 flex items-center text-sm">
      <span class="text-green-500">${{ stats ? stats.estimatedCost.toFixed(4) : '-' }}</span>
      <span class="ml-1 text-gray-500 dark:text-gray-400">{{ t('metrics.estimatedCost') }}</span>
    </div>
  </div>
</template>
