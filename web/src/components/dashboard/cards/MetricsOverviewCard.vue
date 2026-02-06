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
    totalTokens: summary.tokens?.total_tokens ?? 0,
    estimatedCost: summary.tokens?.estimated_cost ?? 0,
    avgLatency: summary.latency?.avg_ms ?? 0,
    p95Latency: summary.latency?.p95_ms ?? 0,
    p99Latency: summary.latency?.p99_ms ?? 0,
    tokensPerSecond: summary.speed?.tokens_per_second ?? 0,
  }
})

function formatNumber(num: number | undefined | null): string {
  if (num == null) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toFixed(0)
}

function formatCost(cost: number | undefined | null): string {
  if (cost == null) return '$0.0000'
  return '$' + cost.toFixed(4)
}

function formatLatency(ms: number | undefined | null): string {
  if (ms == null) return '0ms'
  if (ms >= 1000) return (ms / 1000).toFixed(2) + 's'
  return ms.toFixed(0) + 'ms'
}
</script>

<template>
  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
    <!-- Total Calls Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
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
        <span :class="stats && (stats.successRate ?? 0) >= 95 ? 'text-green-500' : 'text-orange-500'">
          {{ stats ? (stats.successRate ?? 0).toFixed(1) : '-' }}%
        </span>
        <span class="ml-1 text-gray-500 dark:text-gray-400">{{ t('metrics.successRate') }}</span>
      </div>
    </div>

    <!-- Token Usage Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
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
        <span class="text-green-500">{{ stats ? formatCost(stats.estimatedCost) : '-' }}</span>
        <span class="ml-1 text-gray-500 dark:text-gray-400">{{ t('metrics.estimatedCost') }}</span>
      </div>
    </div>

    <!-- Latency Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
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

    <!-- Speed Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('metrics.speed') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ stats ? (stats.tokensPerSecond ?? 0).toFixed(1) : '-' }}
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
  </div>
</template>
