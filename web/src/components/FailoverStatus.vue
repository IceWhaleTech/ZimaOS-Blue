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
const loading = ref(false)
const error = ref<string | null>(null)
const refreshInterval = ref<number | null>(null)

// Error type labels
const errorTypeLabels: Record<string, string> = {
  context_too_long: 'Context Too Long',
  max_tokens_exceeded: 'Content Limit Exceeded',
  rate_limited: 'Rate Limited',
  quota_exceeded: 'Quota Exceeded',
  concurrency_limit: 'Concurrency Limit',
  model_overloaded: 'Model Overloaded',
  service_unavailable: 'Service Unavailable',
  timeout: 'Timeout',
  repetitive_output: 'Repetitive Output',
  infinite_loop: 'Infinite Loop',
  invalid_request: 'Invalid Request',
  auth_failed: 'Auth Failed',
  model_not_found: 'Model Not Found',
  unknown: 'Unknown',
}

// Computed
const failoverSuccessRate = computed(() => {
  if (!metrics.value || metrics.value.failover_total === 0) return 0
  return Math.round((metrics.value.failover_success / metrics.value.failover_total) * 100)
})

const topErrors = computed(() => {
  if (!metrics.value) return []
  return Object.entries(metrics.value.errors_by_type)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([type, count]) => ({
      type,
      label: errorTypeLabels[type] || type,
      count,
    }))
})

const providerFailoverStats = computed(() => {
  if (!metrics.value) return []
  return Object.entries(metrics.value.provider_failovers)
    .sort((a, b) => b[1] - a[1])
    .map(([provider, count]) => ({ provider, count }))
})

// Methods
async function fetchMetrics() {
  try {
    const response = await providerPoolApi.getFailoverMetrics()
    metrics.value = response.data
  } catch (e) {
    console.error('Failed to fetch failover metrics:', e)
  }
}

async function fetchConfig() {
  try {
    const response = await providerPoolApi.getFailoverConfig()
    config.value = response.data
  } catch (e) {
    console.error('Failed to fetch failover config:', e)
  }
}

async function fetchCircuitBreakers() {
  try {
    const response = await providerPoolApi.getCircuitBreakerStatus()
    circuitBreakers.value = response.data
  } catch (e) {
    console.error('Failed to fetch circuit breakers:', e)
  }
}

async function resetBreakers() {
  loading.value = true
  try {
    await providerPoolApi.resetCircuitBreakers()
    await fetchCircuitBreakers()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to reset circuit breakers'
  } finally {
    loading.value = false
  }
}

async function refresh() {
  await Promise.all([fetchMetrics(), fetchCircuitBreakers()])
}

function getStateColor(state: string): string {
  switch (state) {
    case 'closed':
      return 'text-green-500'
    case 'half-open':
      return 'text-yellow-500'
    case 'open':
      return 'text-red-500'
    default:
      return 'text-gray-500'
  }
}

function getStateBgColor(state: string): string {
  switch (state) {
    case 'closed':
      return 'bg-green-500/10'
    case 'half-open':
      return 'bg-yellow-500/10'
    case 'open':
      return 'bg-red-500/10'
    default:
      return 'bg-gray-500/10'
  }
}

// Lifecycle
onMounted(async () => {
  await Promise.all([fetchMetrics(), fetchConfig(), fetchCircuitBreakers()])
  // Auto-refresh every 30 seconds
  refreshInterval.value = window.setInterval(refresh, 30000)
})

onUnmounted(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('settings.failover.title', 'Smart Failover Status') }}
      </h3>
      <button
        class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
        @click="refresh"
      >
        {{ t('common.refresh', 'Refresh') }}
      </button>
    </div>

    <!-- Overview Cards -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <!-- Total Failovers -->
      <div
        class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700"
      >
        <div class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('settings.failover.totalFailovers', 'Total Failovers') }}
        </div>
        <div class="text-2xl font-semibold text-gray-900 dark:text-white mt-1">
          {{ metrics?.failover_total || 0 }}
        </div>
      </div>

      <!-- Success Rate -->
      <div
        class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700"
      >
        <div class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('settings.failover.successRate', 'Success Rate') }}
        </div>
        <div
          class="text-2xl font-semibold mt-1"
          :class="
            failoverSuccessRate >= 80
              ? 'text-green-500'
              : failoverSuccessRate >= 50
                ? 'text-yellow-500'
                : 'text-red-500'
          "
        >
          {{ failoverSuccessRate }}%
        </div>
      </div>

      <!-- Stream Anomalies -->
      <div
        class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700"
      >
        <div class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('settings.failover.streamAnomalies', 'Stream Anomalies') }}
        </div>
        <div class="text-2xl font-semibold text-gray-900 dark:text-white mt-1">
          {{ metrics?.stream_anomalies || 0 }}
        </div>
      </div>

      <!-- Failed Failovers -->
      <div
        class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700"
      >
        <div class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('settings.failover.failedFailovers', 'Failed Failovers') }}
        </div>
        <div class="text-2xl font-semibold text-red-500 mt-1">
          {{ metrics?.failover_failure || 0 }}
        </div>
      </div>
    </div>

    <!-- Circuit Breakers -->
    <div class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
      <div
        class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
      >
        <h4 class="font-medium text-gray-900 dark:text-white">
          {{ t('settings.failover.circuitBreakers', 'Circuit Breakers') }}
        </h4>
        <button
          :disabled="loading"
          class="px-3 py-1 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors disabled:opacity-50"
          @click="resetBreakers"
        >
          {{ t('settings.failover.resetAll', 'Reset All') }}
        </button>
      </div>
      <div class="p-4">
        <div
          v-if="Object.keys(circuitBreakers).length === 0"
          class="text-center text-gray-500 dark:text-gray-400 py-4"
        >
          {{ t('settings.failover.noBreakers', 'No circuit breakers active') }}
        </div>
        <div v-else class="space-y-2">
          <div
            v-for="(breaker, provider) in circuitBreakers"
            :key="provider"
            class="flex items-center justify-between p-3 rounded-lg"
            :class="getStateBgColor(breaker.state)"
          >
            <div class="flex items-center gap-3">
              <div
                class="w-2 h-2 rounded-full"
                :class="getStateColor(breaker.state).replace('text-', 'bg-')"
              ></div>
              <span class="font-medium text-gray-900 dark:text-white">{{ provider }}</span>
            </div>
            <div class="flex items-center gap-4 text-sm">
              <span :class="getStateColor(breaker.state)">{{ breaker.state }}</span>
              <span class="text-gray-500 dark:text-gray-400">
                {{ breaker.failures }} {{ t('settings.failover.failures', 'failures') }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Error Distribution -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <!-- Top Errors -->
      <div class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
        <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <h4 class="font-medium text-gray-900 dark:text-white">
            {{ t('settings.failover.topErrors', 'Top Error Types') }}
          </h4>
        </div>
        <div class="p-4">
          <div
            v-if="topErrors.length === 0"
            class="text-center text-gray-500 dark:text-gray-400 py-4"
          >
            {{ t('settings.failover.noErrors', 'No errors recorded') }}
          </div>
          <div v-else class="space-y-3">
            <div
              v-for="error in topErrors"
              :key="error.type"
              class="flex items-center justify-between"
            >
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ error.label }}</span>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                error.count
              }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Provider Failovers -->
      <div class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
        <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <h4 class="font-medium text-gray-900 dark:text-white">
            {{ t('settings.failover.providerFailovers', 'Failovers by Provider') }}
          </h4>
        </div>
        <div class="p-4">
          <div
            v-if="providerFailoverStats.length === 0"
            class="text-center text-gray-500 dark:text-gray-400 py-4"
          >
            {{ t('settings.failover.noFailovers', 'No failovers recorded') }}
          </div>
          <div v-else class="space-y-3">
            <div
              v-for="stat in providerFailoverStats"
              :key="stat.provider"
              class="flex items-center justify-between"
            >
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ stat.provider }}</span>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                stat.count
              }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Configuration Summary -->
    <div
      v-if="config"
      class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700"
    >
      <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h4 class="font-medium text-gray-900 dark:text-white">
          {{ t('settings.failover.configuration', 'Configuration') }}
        </h4>
      </div>
      <div class="p-4 grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <div class="text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.enabled', 'Enabled') }}
          </div>
          <div class="font-medium" :class="config.enabled ? 'text-green-500' : 'text-red-500'">
            {{ config.enabled ? t('common.yes', 'Yes') : t('common.no', 'No') }}
          </div>
        </div>
        <div>
          <div class="text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.maxRetries', 'Max Retries') }}
          </div>
          <div class="font-medium text-gray-900 dark:text-white">{{ config.max_retries }}</div>
        </div>
        <div>
          <div class="text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.circuitBreakerEnabled', 'Circuit Breaker') }}
          </div>
          <div
            class="font-medium"
            :class="config.circuit_breaker ? 'text-green-500' : 'text-gray-500'"
          >
            {{
              config.circuit_breaker
                ? t('common.enabled', 'Enabled')
                : t('common.disabled', 'Disabled')
            }}
          </div>
        </div>
        <div>
          <div class="text-gray-500 dark:text-gray-400">
            {{ t('settings.failover.anomalyDetection', 'Anomaly Detection') }}
          </div>
          <div
            class="font-medium"
            :class="config.streaming_anomaly?.enabled ? 'text-green-500' : 'text-gray-500'"
          >
            {{
              config.streaming_anomaly?.enabled
                ? t('common.enabled', 'Enabled')
                : t('common.disabled', 'Disabled')
            }}
          </div>
        </div>
      </div>
    </div>

    <!-- Error Alert -->
    <div
      v-if="error"
      class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4"
    >
      <p class="text-red-600 dark:text-red-400">{{ error }}</p>
    </div>
  </div>
</template>
