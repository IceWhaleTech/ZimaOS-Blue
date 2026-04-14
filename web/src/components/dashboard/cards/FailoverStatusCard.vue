<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { providerPoolApi, type FailoverMetrics, type FailoverConfig } from '@/api/providerPool'

const { t } = useI18n()

const metrics = ref<FailoverMetrics | null>(null)
const config = ref<FailoverConfig | null>(null)
const circuitBreakers = ref<
  Record<string, { state: string; failures: number; last_failure?: string }>
>({})
const loading = ref(true)
const refreshInterval = ref<number | null>(null)

const errorTypeKeyMap: Record<string, string> = {
  context_too_long: 'settings.failover.errorTypes.context_too_long',
  max_tokens_exceeded: 'settings.failover.errorTypes.max_tokens_exceeded',
  rate_limited: 'settings.failover.errorTypes.rate_limited',
  quota_exceeded: 'settings.failover.errorTypes.quota_exceeded',
  concurrency_limit: 'settings.failover.errorTypes.concurrency_limit',
  model_overloaded: 'settings.failover.errorTypes.model_overloaded',
  service_unavailable: 'settings.failover.errorTypes.service_unavailable',
  timeout: 'settings.failover.errorTypes.timeout',
  repetitive_output: 'settings.failover.errorTypes.repetitive_output',
  infinite_loop: 'settings.failover.errorTypes.infinite_loop',
  invalid_request: 'settings.failover.errorTypes.invalid_request',
  auth_failed: 'settings.failover.errorTypes.auth_failed',
  model_not_found: 'settings.failover.errorTypes.model_not_found',
  unknown: 'settings.failover.errorTypes.unknown',
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
  if (!config.value?.enabled) return t('common.disabled')
  if (!metrics.value) return '-'
  if (metrics.value.failover_total === 0) return t('settings.failover.noFailovers')
  return `${failoverSuccessRate.value}%`
})

const topErrors = computed(() => {
  if (!metrics.value) return []
  return Object.entries(metrics.value.errors_by_type)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 3)
    .map(([type, count]) => ({
      type,
      label: errorTypeKeyMap[type] ? t(errorTypeKeyMap[type]) : type,
      count,
    }))
})

const openBreakersCount = computed(() => {
  return Object.values(circuitBreakers.value).filter((b) => b.state === 'open').length
})

const halfOpenBreakersCount = computed(() => {
  return Object.values(circuitBreakers.value).filter((b) => b.state === 'half-open').length
})

const totalBreakers = computed(() => Object.keys(circuitBreakers.value).length)

const healthyBreakersCount = computed(() => {
  return totalBreakers.value - openBreakersCount.value - halfOpenBreakersCount.value
})

// Methods
async function fetchData() {
  try {
    const overviewRes = await providerPoolApi.getFailoverOverview()
    metrics.value = overviewRes.data.metrics
    config.value = overviewRes.data.config
    circuitBreakers.value = overviewRes.data.circuit_breakers
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
  <div class="dashboard-card-surface p-4">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy min-w-0">
          <p class="dashboard-card-label">
            {{ t('dashboard.cards.failoverStatus') }}
          </p>
          <p class="dashboard-card-subtitle mt-2">
            {{ t('settings.failover.successRate') }}
            <span
              class="ms-1"
              :class="successRateColor"
            >{{ loading ? '-' : statusText }}</span>
          </p>
        </div>
        <div class="flex flex-wrap items-center justify-end gap-2 text-xs">
          <span class="dashboard-card-chip">{{
            config?.enabled ? t('common.enabled') : t('common.disabled')
          }}</span>
          <span
            v-if="config?.circuit_breaker"
            class="dashboard-card-chip"
          >{{
            t('settings.failover.circuitBreakerEnabled')
          }}</span>
          <span
            v-if="config?.streaming_anomaly?.enabled"
            class="dashboard-card-chip"
          >{{
            t('settings.failover.anomalyDetection')
          }}</span>
        </div>
      </div>

      <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.totalFailovers') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ metrics?.failover_total || 0 }}
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.failedFailovers') }}
          </p>
          <p
            class="mt-1 text-lg font-semibold"
            :class="
              (metrics?.failover_failure || 0) > 0
                ? 'text-red-500'
                : 'text-gray-900 dark:text-white'
            "
          >
            {{ metrics?.failover_failure || 0 }}
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.streamAnomalies') }}
          </p>
          <p
            class="mt-1 text-lg font-semibold"
            :class="
              (metrics?.stream_anomalies || 0) > 0
                ? 'text-orange-500'
                : 'text-gray-900 dark:text-white'
            "
          >
            {{ metrics?.stream_anomalies || 0 }}
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('common.success') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-green-600 dark:text-green-400">
            {{ metrics?.failover_success || 0 }}
          </p>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-3 text-xs">
        <div class="dashboard-card-subsurface p-3">
          <p class="mb-2 text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.circuitBreakers') }}
          </p>
          <div
            v-if="totalBreakers === 0"
            class="text-gray-400"
          >
            {{ t('settings.failover.noBreakers') }}
          </div>
          <div
            v-else
            class="flex flex-wrap gap-2"
          >
            <span class="dashboard-card-chip text-green-700 dark:text-green-300">{{ healthyBreakersCount }} {{ t('settings.failover.chips.healthy') }}</span>
            <span
              v-if="halfOpenBreakersCount > 0"
              class="dashboard-card-chip text-yellow-700 dark:text-yellow-300"
            >
              {{ halfOpenBreakersCount }} {{ t('settings.failover.chips.halfOpen') }}
            </span>
            <span
              v-if="openBreakersCount > 0"
              class="dashboard-card-chip text-red-700 dark:text-red-300"
            >
              {{ openBreakersCount }} {{ t('settings.failover.chips.open') }}
            </span>
          </div>
        </div>

        <div class="dashboard-card-subsurface p-3">
          <p class="mb-2 text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.topErrors') }}
          </p>
          <div
            v-if="topErrors.length === 0"
            class="text-gray-400"
          >
            {{ t('settings.failover.noErrors') }}
          </div>
          <div
            v-else
            class="flex flex-wrap gap-2"
          >
            <span
              v-for="err in topErrors"
              :key="err.type"
              class="dashboard-card-chip"
            >
              {{ err.label }} <span class="font-semibold">{{ err.count }}</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
