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
    tokensPerSecond: summary.speed?.tokens_per_second ?? 0,
  }
})
</script>

<template>
  <div>
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('metrics.speed') }}</p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ stats ? stats.tokensPerSecond.toFixed(1) : '-' }}
        </p>
      </div>
      <div class="p-3 bg-orange-100 dark:bg-orange-900/30 rounded-full">
        <svg class="w-6 h-6 text-orange-600 dark:text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
        </svg>
      </div>
    </div>
    <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
      <span>{{ t('metrics.tokensPerSecond') }}</span>
    </div>
  </div>
</template>
