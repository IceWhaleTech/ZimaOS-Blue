<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { providerPoolApi, type FailoverMetrics, type FailoverConfig } from '@/api/providerPool'

const { t } = useI18n()

const metrics = ref<FailoverMetrics | null>(null)
const config = ref<FailoverConfig | null>(null)
const circuitBreakers = ref<Record<string, { state: string; failures: number; last_failure?: string }>>({})
const loading = ref(true)
const refreshInterval = ref<number | null>(null)

// Error type labels
const errorTypeLabels: Record<string, string> = {
  context_too_long: 'Context Too Long',
  max_tokens_exceeded: 'Content Limit',
  rate_limited: 'Rate Limited',
  quota_exceeded: 'Quota Exceeded',
  concurrency_limit: 'Concurrency',
  model_overloaded: 'Overloaded',
  service_unavailable: 'Unavailable',
  timeout: 'Timeout',
  repetitive_output: 'Repetitive',
  infinite_loop: 'Loop',
  invalid_request: 'Invalid',
  auth_failed: 'Auth Failed',
  model_not_found: 'Not Found',
  unknown: 'Unknown',
}

// Computed
const failoverSuccessRate = computed(() => {
  if (!metrics.value || metrics.value.failover_total === 0) return 100
  return Math.round((metrics.value.failover_success / metrics.value.failover_total) * 100)
})

const successRateColor = computed(() => {
  const rate = failoverSuccessRate.value
  if (rate >= 90) return 'text-green-600 dark:text-green-400'
  if (rate >= 70) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
})

const statusText = computed(() => {
  if (!config.value?.enabled) return t('common.disabled', 'Disabled')
  if (!metrics.value) return '-'
  if (metrics.value.failover_total === 0) return t('settings.failover.noFailovers', 'No failovers')
  return `${failoverSuccessRate.value}%`
})

const statusDotClass = computed(() => {
  if (!config.value?.enabled) return 'bg-gray-400'
  const rate = failoverSuccessRate.value
  if (rate >= 90) return 'bg-green-500'
  if (rate >= 70) return 'bg-yellow-500'
  return 'bg-red-500'
})

const topErrors = computed(() => {
  if (!metrics.value) return []
  return Object.entries(metrics.value.errors_by_type)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 3)
    .map(([type, count]) => ({
      type,
      label: errorTypeLabels[type] || type,
      count,
    }))
})

const openBreakersCount = computed(() => {
  return Object.values(circuitBreakers.value).filter(b => b.state === 'open').length
})

const halfOpenBreakersCount = computed(() => {
  return Object.values(circuitBreakers.value).filter(b => b.state === 'half-open').length
})

const totalBreakers = computed(() => Object.keys(circuitBreakers.value).length)

const healthyBreakersCount = computed(() => {
  return totalBreakers.value - openBreakersCount.value - halfOpenBreakersCount.value
})

// Methods
async function fetchData() {
  try {
    const [metricsRes, configRes, breakersRes] = await Promise.all([
      providerPoolApi.getFailoverMetrics(),
      providerPoolApi.getFailoverConfig(),
      providerPoolApi.getCircuitBreakerStatus(),
    ])
    metrics.value = metricsRes.data
    config.value = configRes.data
    circuitBreakers.value = breakersRes.data
  } catch (e) {
    console.error('Failed to fetch failover data:', e)
  } finally {
    loading.value = false
  }
}

// Lifecycle
onMounted(() => {
  fetchData()
  refreshInterval.value = window.setInterval(fetchData, 30000)
})

onUnmounted(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
  }
})
</script>

<template>
  <div class="rounded-xl border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-700 p-4 space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <div class="w-2.5 h-2.5 rounded-full flex-shrink-0" :class="[statusDotClass, config?.enabled ? 'animate-pulse' : '']"></div>
        <p class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('settings.failover.successRate', 'Success Rate') }}
          <span class="ml-1" :class="successRateColor">{{ loading ? '-' : statusText }}</span>
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2 text-xs">
        <span class="inline-flex items-center rounded-full px-2 py-1 border border-gray-200 dark:border-gray-500 text-gray-700 dark:text-gray-200">
          {{ config?.enabled ? 'ON' : 'OFF' }}
        </span>
        <span
          v-if="config?.circuit_breaker"
          class="inline-flex items-center rounded-full px-2 py-1 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300"
        >
          CB
        </span>
        <span
          v-if="config?.streaming_anomaly?.enabled"
          class="inline-flex items-center rounded-full px-2 py-1 bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300"
        >
          AD
        </span>
      </div>
    </div>

    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.failover.totalFailovers', 'Total') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ metrics?.failover_total || 0 }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.failover.failedFailovers', 'Failed') }}</p>
        <p
          class="mt-1 text-lg font-semibold"
          :class="(metrics?.failover_failure || 0) > 0 ? 'text-red-500' : 'text-gray-900 dark:text-white'"
        >
          {{ metrics?.failover_failure || 0 }}
        </p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.failover.streamAnomalies', 'Anomalies') }}</p>
        <p
          class="mt-1 text-lg font-semibold"
          :class="(metrics?.stream_anomalies || 0) > 0 ? 'text-orange-500' : 'text-gray-900 dark:text-white'"
        >
          {{ metrics?.stream_anomalies || 0 }}
        </p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.failover.circuitBreakers', 'Breakers') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ totalBreakers }}</p>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-3 text-xs">
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 p-3">
        <p class="mb-2 text-gray-500 dark:text-gray-400">{{ t('settings.failover.circuitBreakers', 'Breakers') }}</p>
        <div v-if="totalBreakers === 0" class="text-gray-400">-</div>
        <div v-else class="flex flex-wrap gap-2">
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300">
            {{ healthyBreakersCount }} OK
          </span>
          <span
            v-if="halfOpenBreakersCount > 0"
            class="inline-flex items-center rounded-md px-2 py-1 bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300"
          >
            {{ halfOpenBreakersCount }} Half
          </span>
          <span
            v-if="openBreakersCount > 0"
            class="inline-flex items-center rounded-md px-2 py-1 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300"
          >
            {{ openBreakersCount }} Open
          </span>
        </div>
      </div>

      <div class="rounded-lg border border-gray-200 dark:border-gray-600 p-3">
        <p class="mb-2 text-gray-500 dark:text-gray-400">{{ t('settings.failover.topErrors', 'Errors') }}</p>
        <div v-if="topErrors.length === 0" class="text-gray-400">-</div>
        <div v-else class="flex flex-wrap gap-2">
          <span
            v-for="err in topErrors"
            :key="err.type"
            class="inline-flex items-center gap-1 rounded-md px-2 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300"
          >
            {{ err.label }} <span class="font-semibold">{{ err.count }}</span>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
