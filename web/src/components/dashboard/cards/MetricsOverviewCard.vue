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
  <div class="dashboard-card-surface p-4">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy">
          <p class="dashboard-card-label">
            {{ t('dashboard.cards.metricsOverview') }}
          </p>
          <p class="dashboard-card-subtitle mt-2">
            {{ t('metrics.cards.overview.subtitle') }}
          </p>
        </div>
      </div>

      <div
        v-if="!stats"
        class="dashboard-card-empty"
      >
        {{ t('metrics.noData') }}
      </div>

      <div
        v-else
        class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4"
      >
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('metrics.totalCalls') }}
          </p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ formatNumber(stats.totalCalls) }}
          </p>
          <p class="dashboard-card-footnote mt-2">
            <span
              :class="
                (stats.successRate ?? 0) >= 95
                  ? 'text-green-600 dark:text-green-400'
                  : 'text-orange-600 dark:text-orange-400'
              "
            >
              {{ (stats.successRate ?? 0).toFixed(1) }}%
            </span>
            {{ t('metrics.successRate') }}
          </p>
        </div>

        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('metrics.tokenUsage') }}
          </p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ formatNumber(stats.totalTokens) }}
          </p>
          <p class="dashboard-card-footnote mt-2">
            {{ formatCost(stats.estimatedCost) }} {{ t('metrics.estimatedCost') }}
          </p>
        </div>

        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('metrics.avgLatency') }}
          </p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ formatLatency(stats.avgLatency) }}
          </p>
          <p class="dashboard-card-footnote mt-2">
            {{ t('metrics.p95') }} {{ formatLatency(stats.p95Latency) }} ·
            {{ t('metrics.p99') }} {{ formatLatency(stats.p99Latency) }}
          </p>
        </div>

        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('metrics.speed') }}
          </p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ (stats.tokensPerSecond ?? 0).toFixed(1) }}
          </p>
          <p class="dashboard-card-footnote mt-2">
            {{ t('metrics.tokensPerSecond') }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
