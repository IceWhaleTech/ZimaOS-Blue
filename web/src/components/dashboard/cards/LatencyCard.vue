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
    avgLatency: summary.latency?.avg_ms ?? 0,
    p95Latency: summary.latency?.p95_ms ?? 0,
    p99Latency: summary.latency?.p99_ms ?? 0,
  }
})

function formatLatency(ms: number): string {
  if (ms >= 1000) return (ms / 1000).toFixed(2) + 's'
  return ms.toFixed(0) + 'ms'
}
</script>

<template>
  <div>
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('metrics.avgLatency') }}</p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ stats ? formatLatency(stats.avgLatency) : '-' }}
        </p>
      </div>
      <div class="p-3 bg-purple-100 dark:bg-purple-900/30 rounded-full">
        <svg class="w-6 h-6 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      </div>
    </div>
    <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
      <span>P95: {{ stats ? formatLatency(stats.p95Latency) : '-' }}</span>
      <span class="mx-2">|</span>
      <span>P99: {{ stats ? formatLatency(stats.p99Latency) : '-' }}</span>
    </div>
  </div>
</template>
