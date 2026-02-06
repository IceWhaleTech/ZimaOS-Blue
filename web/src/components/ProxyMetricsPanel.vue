<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyApi, type MetricsSummary, type ProviderMetrics, type LatencyStats } from '@/api/proxy'

const { t: _t } = useI18n()

const metrics = ref<MetricsSummary | null>(null)
const providerMetrics = ref<Record<string, ProviderMetrics>>({})
const latencyStats = ref<LatencyStats | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const refreshInterval = ref<number | null>(null)
const selectedProvider = ref<string | null>(null)

// Computed
const totalRequests = computed(() => metrics.value?.total_requests ?? 0)
const successRate = computed(() => {
  if (!metrics.value || metrics.value.total_requests === 0) return 0
  return ((metrics.value.successful_requests / metrics.value.total_requests) * 100).toFixed(1)
})
const avgLatency = computed(() => {
  if (!latencyStats.value) return 0
  return latencyStats.value.avg_ms?.toFixed(0) ?? 0
})

const providerList = computed(() => Object.entries(providerMetrics.value))

const formatNumber = (num: number) => {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toString()
}

// Calculate bar heights for percentile visualization
const percentileBars = computed(() => {
  if (!latencyStats.value) return []

  const values = [
    latencyStats.value.min_ms ?? 0,
    latencyStats.value.p50_ms ?? 0,
    latencyStats.value.p90_ms ?? 0,
    latencyStats.value.p95_ms ?? 0,
    latencyStats.value.p99_ms ?? 0,
    latencyStats.value.max_ms ?? 0,
  ]

  const max = Math.max(...values)
  const min = Math.min(...values)
  const range = max - min

  const percentiles = [
    { label: 'Min', value: latencyStats.value.min_ms ?? 0, color: 'bg-green-500' },
    { label: 'P50', value: latencyStats.value.p50_ms ?? 0, color: 'bg-blue-500' },
    { label: 'P90', value: latencyStats.value.p90_ms ?? 0, color: 'bg-yellow-500' },
    { label: 'P95', value: latencyStats.value.p95_ms ?? 0, color: 'bg-orange-500' },
    { label: 'P99', value: latencyStats.value.p99_ms ?? 0, color: 'bg-red-500' },
    { label: 'Max', value: latencyStats.value.max_ms ?? 0, color: 'bg-red-700' },
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

// Methods
async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const [metricsRes, providersRes, latencyRes] = await Promise.all([
      proxyApi.getMetrics(),
      proxyApi.getProviderMetrics(),
      proxyApi.getLatencyStats(),
    ])
    metrics.value = metricsRes.data
    providerMetrics.value = providersRes.data.providers || {}
    latencyStats.value = latencyRes.data
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to fetch metrics'
    console.error('Failed to fetch metrics:', e)
  } finally {
    loading.value = false
  }
}

function startAutoRefresh() {
  refreshInterval.value = window.setInterval(fetchData, 15000)
}

function stopAutoRefresh() {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
    refreshInterval.value = null
  }
}

function selectProvider(name: string) {
  selectedProvider.value = selectedProvider.value === name ? null : name
}

onMounted(() => {
  fetchData()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<template>
  <div class="proxy-metrics-panel">
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Proxy Metrics</h2>
      <button
        :disabled="loading"
        class="px-3 py-1.5 text-sm bg-gray-100 hover:bg-gray-700 dark:bg-gray-700 dark:hover:bg-gray-600 rounded-md transition-colors disabled:opacity-50"
        @click="fetchData"
      >
        {{ loading ? 'Refreshing...' : 'Refresh' }}
      </button>
    </div>

    <!-- Error -->
    <div
      v-if="error"
      class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400"
    >
      {{ error }}
    </div>

    <!-- Overview Cards -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
      <div class="p-4 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
        <div class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ formatNumber(totalRequests) }}
        </div>
        <div class="text-sm text-gray-500 dark:text-gray-400">Total Requests</div>
      </div>
      <div class="p-4 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
        <div class="text-2xl font-bold" :class="Number(successRate) >= 95 ? 'text-green-600' : Number(successRate) >= 80 ? 'text-yellow-600' : 'text-red-600'">
          {{ successRate }}%
        </div>
        <div class="text-sm text-gray-500 dark:text-gray-400">Success Rate</div>
      </div>
      <div class="p-4 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
        <div class="text-2xl font-bold text-gray-900 dark:text-white dark:text-gray-900 dark:text-white">
          {{ avgLatency }}ms
        </div>
        <div class="text-sm text-gray-500 dark:text-gray-400">Avg Latency</div>
      </div>
      <div class="p-4 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
        <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">
          {{ providerList.length }}
        </div>
        <div class="text-sm text-gray-500 dark:text-gray-400">Active Providers</div>
      </div>
    </div>

    <!-- Latency Percentiles -->
    <div v-if="latencyStats" class="mb-4 p-4 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
      <h3 class="font-medium text-gray-900 dark:text-white mb-3">Latency Distribution</h3>

      <!-- Bar Chart Visualization -->
      <div class="flex justify-around h-32 bg-gray-50 dark:bg-gray-700/30 rounded-lg p-4 mb-4">
        <div
          v-for="bar in percentileBars"
          :key="bar.label"
          class="flex flex-col items-center h-full justify-end"
        >
          <div class="w-8 flex items-end" :style="{ height: bar.height + '%' }">
            <div :class="[bar.color, 'w-full rounded-t transition-all duration-300']" style="height: 100%"></div>
          </div>
          <div class="mt-2 text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">{{ bar.label }}</div>
          <div class="text-xs font-medium text-gray-900 dark:text-white">{{ bar.value.toFixed(0) }}ms</div>
        </div>
      </div>

      <!-- Stats Grid -->
      <div class="grid grid-cols-3 md:grid-cols-6 gap-4 text-sm">
        <div>
          <span class="text-gray-500 dark:text-gray-400">Min:</span>
          <span class="ml-2 font-medium text-green-600 dark:text-green-400">{{ latencyStats.min_ms?.toFixed(0) ?? '-' }}ms</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">P50:</span>
          <span class="ml-2 font-medium text-gray-900 dark:text-white dark:text-gray-900 dark:text-white">{{ latencyStats.p50_ms?.toFixed(0) ?? '-' }}ms</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">P90:</span>
          <span class="ml-2 font-medium text-yellow-600 dark:text-yellow-400">{{ latencyStats.p90_ms?.toFixed(0) ?? '-' }}ms</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">P95:</span>
          <span class="ml-2 font-medium text-orange-600 dark:text-orange-400">{{ latencyStats.p95_ms?.toFixed(0) ?? '-' }}ms</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">P99:</span>
          <span class="ml-2 font-medium text-red-600 dark:text-red-400">{{ latencyStats.p99_ms?.toFixed(0) ?? '-' }}ms</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">Max:</span>
          <span class="ml-2 font-medium text-red-700 dark:text-red-500">{{ latencyStats.max_ms?.toFixed(0) ?? '-' }}ms</span>
        </div>
      </div>
    </div>

    <!-- Token Usage -->
    <div v-if="metrics" class="mb-4 p-4 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
      <h3 class="font-medium text-gray-900 dark:text-white mb-3">Token Usage</h3>
      <div class="grid grid-cols-3 gap-4 text-sm">
        <div>
          <span class="text-gray-500 dark:text-gray-400">Input:</span>
          <span class="ml-2 font-medium text-gray-900 dark:text-white">{{ formatNumber(metrics.total_input_tokens ?? 0) }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">Output:</span>
          <span class="ml-2 font-medium text-gray-900 dark:text-white">{{ formatNumber(metrics.total_output_tokens ?? 0) }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">Total:</span>
          <span class="ml-2 font-medium text-gray-900 dark:text-white">
            {{ formatNumber((metrics.total_input_tokens ?? 0) + (metrics.total_output_tokens ?? 0)) }}
          </span>
        </div>
      </div>
    </div>

    <!-- Provider List -->
    <div class="space-y-2">
      <h3 class="font-medium text-gray-900 dark:text-white">Providers</h3>

      <div v-if="providerList.length === 0 && !loading" class="text-center py-8 text-gray-500 dark:text-gray-400">
        No provider metrics available
      </div>

      <div
        v-for="[name, provider] in providerList"
        :key="name"
        class="p-3 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 cursor-pointer hover:border-gray-900 dark:border-white dark:hover:border-gray-900 dark:border-white transition-colors"
        :class="{ 'border-gray-900 dark:border-white dark:border-gray-900 dark:border-white': selectedProvider === name }"
        @click="selectProvider(name)"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <span class="text-lg">🔌</span>
            <div>
              <div class="font-medium text-gray-900 dark:text-white">{{ name }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ provider.request_count }} requests
              </div>
            </div>
          </div>
          <div class="text-right">
            <div class="text-sm font-medium" :class="provider.success_rate >= 95 ? 'text-green-600' : provider.success_rate >= 80 ? 'text-yellow-600' : 'text-red-600'">
              {{ provider.success_rate?.toFixed(1) ?? 0 }}%
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ provider.avg_latency_ms?.toFixed(0) ?? 0 }}ms avg
            </div>
          </div>
        </div>

        <!-- Expanded Details -->
        <div v-if="selectedProvider === name" class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700">
          <div class="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
            <div>
              <span class="text-gray-500 dark:text-gray-400">Success:</span>
              <span class="ml-1 text-green-600">{{ provider.success_count ?? 0 }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">Errors:</span>
              <span class="ml-1 text-red-600">{{ provider.error_count ?? 0 }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">Input Tokens:</span>
              <span class="ml-1 text-gray-900 dark:text-white">{{ formatNumber(provider.total_input_tokens ?? 0) }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">Output Tokens:</span>
              <span class="ml-1 text-gray-900 dark:text-white">{{ formatNumber(provider.total_output_tokens ?? 0) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.proxy-metrics-panel {
  @apply p-4;
}
</style>
