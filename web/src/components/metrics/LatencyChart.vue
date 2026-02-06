<script setup lang="ts">
import { computed } from 'vue'
import { useMetricsStore } from '@/stores/metrics'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const metricsStore = useMetricsStore()

const latencyData = computed(() => {
  const stats = metricsStore.latencyStats
  if (!stats) return null

  return {
    avg: stats.avg_ms ?? 0,
    min: stats.min_ms ?? 0,
    max: stats.max_ms ?? 0,
    p50: stats.p50_ms ?? 0,
    p90: stats.p90_ms ?? 0,
    p95: stats.p95_ms ?? 0,
    p99: stats.p99_ms ?? 0,
    samples: stats.samples ?? 0,
  }
})

const speedData = computed(() => {
  const stats = metricsStore.speedStats
  if (!stats) return null

  return {
    current: stats.current,
    average: stats.average,
    percentiles: stats.percentiles,
  }
})

function formatLatency(ms: number | undefined | null): string {
  if (ms == null) return '0ms'
  if (ms >= 1000) return (ms / 1000).toFixed(2) + 's'
  return ms.toFixed(0) + 'ms'
}

// Calculate bar heights for percentile visualization
const percentileBars = computed(() => {
  if (!latencyData.value) return []

  const values = [
    latencyData.value.min ?? 0,
    latencyData.value.p50 ?? 0,
    latencyData.value.p90 ?? 0,
    latencyData.value.p95 ?? 0,
    latencyData.value.p99 ?? 0,
    latencyData.value.max ?? 0,
  ]

  const max = Math.max(...values)
  const min = Math.min(...values)
  const range = max - min

  const percentiles = [
    { label: t('metrics.min'), value: latencyData.value.min ?? 0, color: 'bg-green-500' },
    { label: t('metrics.p50'), value: latencyData.value.p50 ?? 0, color: 'bg-blue-500' },
    { label: t('metrics.p90'), value: latencyData.value.p90 ?? 0, color: 'bg-yellow-500' },
    { label: t('metrics.p95'), value: latencyData.value.p95 ?? 0, color: 'bg-orange-500' },
    { label: t('metrics.p99'), value: latencyData.value.p99 ?? 0, color: 'bg-red-500' },
    { label: t('metrics.max'), value: latencyData.value.max ?? 0, color: 'bg-red-700' },
  ]

  // If all values are the same or range is very small, show all bars at 80% height
  if (range < 1 || max === 0) {
    return percentiles.map(p => ({
      ...p,
      height: p.value > 0 ? 80 : 5,
    }))
  }

  // Otherwise, scale based on range with minimum 10% height for non-zero values
  return percentiles.map(p => ({
    ...p,
    height: p.value === 0 ? 5 : Math.max(10, ((p.value - min) / range) * 90 + 10),
  }))
})
</script>

<template>
  <div class="bg-white dark:bg-gray-700 rounded-lg p-6 shadow">
    <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
      {{ t('metrics.latencyPerformance') }}
    </h3>

    <div v-if="!latencyData" class="text-center py-8 text-gray-500 dark:text-gray-400">
      {{ t('metrics.noData') }}
    </div>

    <div v-else class="space-y-6">
      <!-- Percentile Bar Chart -->
      <div>
        <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
          {{ t('metrics.latencyDistribution') }}
        </h4>
        <div class="flex justify-around h-32 bg-gray-50 dark:bg-gray-700/30 rounded-lg p-4">
          <div
            v-for="bar in percentileBars"
            :key="bar.label"
            class="flex flex-col items-center h-full justify-end"
          >
            <div class="w-8 flex items-end" :style="{ height: bar.height + '%' }">
              <div :class="[bar.color, 'w-full rounded-t transition-all duration-300']" style="height: 100%"></div>
            </div>
            <div class="mt-2 text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">{{ bar.label }}</div>
            <div class="text-xs font-medium text-gray-900 dark:text-white">{{ formatLatency(bar.value) }}</div>
          </div>
        </div>
      </div>

      <!-- Stats Grid -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 text-center">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('metrics.avgLatency') }}</div>
          <div class="text-lg font-bold text-gray-900 dark:text-white">{{ formatLatency(latencyData.avg ?? 0) }}</div>
        </div>
        <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 text-center">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('metrics.p50') }}</div>
          <div class="text-lg font-bold text-gray-900 dark:text-white dark:text-gray-900 dark:text-white">{{ formatLatency(latencyData.p50 ?? 0) }}</div>
        </div>
        <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 text-center">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('metrics.p95') }}</div>
          <div class="text-lg font-bold text-orange-600 dark:text-orange-400">{{ formatLatency(latencyData.p95 ?? 0) }}</div>
        </div>
        <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 text-center">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('metrics.p99') }}</div>
          <div class="text-lg font-bold text-red-600 dark:text-red-400">{{ formatLatency(latencyData.p99 ?? 0) }}</div>
        </div>
      </div>

      <!-- Speed Stats -->
      <div v-if="speedData && speedData.current && speedData.average">
        <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
          {{ t('metrics.speedStats') }}
        </h4>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="bg-gradient-to-br from-blue-50 to-blue-100 dark:from-blue-900/30 dark:to-blue-800/30 rounded-lg p-4">
            <div class="text-sm text-blue-600 dark:text-blue-400">{{ t('metrics.currentSpeed') }}</div>
            <div class="text-2xl font-bold text-blue-700 dark:text-blue-300">
              {{ (speedData.current.tokens_per_second ?? 0).toFixed(1) }}
            </div>
            <div class="text-xs text-blue-500 dark:text-blue-400">{{ t('metrics.tokensPerSecond') }}</div>
          </div>
          <div class="bg-gradient-to-br from-green-50 to-green-100 dark:from-green-900/30 dark:to-green-800/30 rounded-lg p-4">
            <div class="text-sm text-green-600 dark:text-green-400">{{ t('metrics.avgSpeed') }}</div>
            <div class="text-2xl font-bold text-green-700 dark:text-green-300">
              {{ (speedData.average.tokens_per_second ?? 0).toFixed(1) }}
            </div>
            <div class="text-xs text-green-500 dark:text-green-400">{{ t('metrics.tokensPerSecond') }}</div>
          </div>
          <div class="bg-gradient-to-br from-purple-50 to-purple-100 dark:from-purple-900/30 dark:to-purple-800/30 rounded-lg p-4">
            <div class="text-sm text-purple-600 dark:text-purple-400">{{ t('metrics.timeToFirstToken') }}</div>
            <div class="text-2xl font-bold text-purple-700 dark:text-purple-300">
              {{ formatLatency(speedData.current.time_to_first_token_ms ?? 0) }}
            </div>
            <div class="text-xs text-purple-500 dark:text-purple-400">TTFT</div>
          </div>
        </div>
      </div>

      <!-- Sample Count -->
      <div class="text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('metrics.basedOn', { count: latencyData.samples ?? 0 }) }}
      </div>
    </div>
  </div>
</template>
