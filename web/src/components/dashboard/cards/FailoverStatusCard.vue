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
  <div class="border border-gray-200 dark:border-gray-700 rounded-xl p-4 bg-white dark:bg-gray-700">
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
      <!-- Left: Key Metrics -->
      <div class="flex items-center gap-6">
        <!-- Success Rate -->
        <div class="flex items-center gap-2">
          <div class="w-2.5 h-2.5 rounded-full flex-shrink-0" :class="[statusDotClass, config?.enabled ? 'animate-pulse' : '']"></div>
          <div>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.failover.successRate', 'Success Rate') }}</p>
            <p class="text-lg font-semibold" :class="successRateColor">{{ loading ? '-' : statusText }}</p>
          </div>
        </div>

        <!-- Divider -->
        <div class="w-px h-10 bg-gray-700 dark:bg-gray-700"></div>

        <!-- Total -->
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.failover.totalFailovers', 'Total') }}</p>
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ metrics?.failover_total || 0 }}</p>
        </div>

        <!-- Anomalies -->
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.failover.streamAnomalies', 'Anomalies') }}</p>
          <p class="text-lg font-semibold" :class="(metrics?.stream_anomalies || 0) > 0 ? 'text-orange-500' : 'text-gray-900 dark:text-white'">
            {{ metrics?.stream_anomalies || 0 }}
          </p>
        </div>

        <!-- Failed -->
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.failover.failedFailovers', 'Failed') }}</p>
          <p class="text-lg font-semibold" :class="(metrics?.failover_failure || 0) > 0 ? 'text-red-500' : 'text-gray-900 dark:text-white'">
            {{ metrics?.failover_failure || 0 }}
          </p>
        </div>
      </div>

      <!-- Right: Status Info -->
      <div class="flex flex-wrap items-center gap-4 text-xs">
        <!-- Circuit Breakers -->
        <div class="flex items-center gap-1.5">
          <span class="text-gray-500 dark:text-gray-400">{{ t('settings.failover.circuitBreakers', 'Breakers') }}:</span>
          <span v-if="Object.keys(circuitBreakers).length === 0" class="text-gray-400">-</span>
          <template v-else>
            <span class="text-green-500 font-medium">{{ Object.keys(circuitBreakers).length - openBreakersCount - halfOpenBreakersCount }} OK</span>
            <span v-if="halfOpenBreakersCount > 0" class="text-yellow-500 font-medium">{{ halfOpenBreakersCount }} Half</span>
            <span v-if="openBreakersCount > 0" class="text-red-500 font-medium">{{ openBreakersCount }} Open</span>
          </template>
        </div>

        <!-- Top Errors -->
        <div class="flex items-center gap-1.5 flex-wrap">
          <span class="text-gray-500 dark:text-gray-400">{{ t('settings.failover.topErrors', 'Errors') }}:</span>
          <span v-if="topErrors.length === 0" class="text-gray-400">-</span>
          <template v-else>
            <span
              v-for="err in topErrors"
              :key="err.type"
              class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
            >
              {{ err.label }} <span class="font-medium">{{ err.count }}</span>
            </span>
          </template>
        </div>

        <!-- Config -->
        <div class="flex items-center gap-1.5">
          <span :class="config?.enabled ? 'text-green-500' : 'text-gray-400'">
            {{ config?.enabled ? 'ON' : 'OFF' }}
          </span>
          <span v-if="config?.circuit_breaker" class="text-gray-900 dark:text-white">CB</span>
          <span v-if="config?.streaming_anomaly?.enabled" class="text-purple-500">AD</span>
        </div>
      </div>
    </div>
  </div>
</template>
