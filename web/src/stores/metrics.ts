import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  metricsApi,
  type MetricsSummary,
  type CallStatsResponse,
  type ModelStatsResponse,
  type TokenUsageResponse,
  type LatencyStats,
  type SpeedResponse,
  type TokenPricing,
} from '@/api/metrics'

export const useMetricsStore = defineStore('metrics', () => {
  // State
  const summary = ref<MetricsSummary | null>(null)
  const callStats = ref<CallStatsResponse | null>(null)
  const modelStats = ref<ModelStatsResponse | null>(null)
  const tokenUsage = ref<TokenUsageResponse | null>(null)
  const latencyStats = ref<LatencyStats | null>(null)
  const speedStats = ref<SpeedResponse | null>(null)
  const pricing = ref<TokenPricing[]>([])

  const loading = ref(false)
  const error = ref<string | null>(null)
  const lastUpdated = ref<Date | null>(null)

  // Auto-refresh
  const autoRefreshEnabled = ref(false)
  const autoRefreshInterval = ref(5000) // 5 seconds
  let refreshTimer: ReturnType<typeof setInterval> | null = null

  // Computed
  const successRate = computed(() => {
    if (!summary.value?.calls) return 0
    return summary.value.calls.success_rate
  })

  const totalCost = computed(() => {
    if (!summary.value?.tokens) return 0
    return summary.value.tokens.estimated_cost
  })

  const avgLatency = computed(() => {
    if (!summary.value?.latency) return 0
    return summary.value.latency.avg_ms
  })

  const tokensPerSecond = computed(() => {
    if (!summary.value?.speed) return 0
    return summary.value.speed.tokens_per_second
  })

  // Actions - Individual fetchers (kept for backward compatibility)
  async function fetchSummary() {
    try {
      const response = await metricsApi.getSummary()
      summary.value = response.data
    } catch (e) {
      console.error('Failed to fetch metrics summary:', e)
    }
  }

  async function fetchCallStats(period?: string) {
    try {
      const response = await metricsApi.getCallStats(period)
      callStats.value = response.data
    } catch (e) {
      console.error('Failed to fetch call stats:', e)
    }
  }

  async function fetchModelStats(period?: string) {
    try {
      const response = await metricsApi.getModelStats(period)
      modelStats.value = response.data
    } catch (e) {
      console.error('Failed to fetch model stats:', e)
    }
  }

  async function fetchTokenUsage(period?: string) {
    try {
      const response = await metricsApi.getTokenUsage(period)
      tokenUsage.value = response.data
    } catch (e) {
      console.error('Failed to fetch token usage:', e)
    }
  }

  async function fetchLatencyStats(model?: string) {
    try {
      const response = await metricsApi.getLatencyStats(model)
      latencyStats.value = response.data
    } catch (e) {
      console.error('Failed to fetch latency stats:', e)
    }
  }

  async function fetchSpeedStats(model?: string) {
    try {
      const response = await metricsApi.getSpeedStats(model)
      speedStats.value = response.data
    } catch (e) {
      console.error('Failed to fetch speed stats:', e)
    }
  }

  async function fetchPricing() {
    try {
      const response = await metricsApi.getPricing()
      pricing.value = response.data || []
    } catch (e) {
      console.error('Failed to fetch pricing:', e)
      pricing.value = []
    }
  }

  // Fetch all metrics using aggregated endpoint (single API call)
  async function fetchAll() {
    loading.value = true
    error.value = null
    try {
      const response = await metricsApi.getAll()
      const data = response.data

      // Update all state from aggregated response
      callStats.value = data.calls
      modelStats.value = data.models
      tokenUsage.value = data.tokens
      latencyStats.value = data.latency
      speedStats.value = data.speed
      pricing.value = data.pricing || []

      // Build summary from aggregated data
      summary.value = {
        calls: data.calls?.stats || {
          total_calls: 0,
          successful_calls: 0,
          failed_calls: 0,
          success_rate: 0,
          error_rate: 0,
          errors_by_type: {},
        },
        tokens: data.tokens?.usage || {
          input_tokens: 0,
          output_tokens: 0,
          cache_read_tokens: 0,
          cache_write_tokens: 0,
          total_tokens: 0,
          estimated_cost: 0,
        },
        latency: data.latency || {
          min_ms: 0,
          max_ms: 0,
          avg_ms: 0,
          p50_ms: 0,
          p90_ms: 0,
          p95_ms: 0,
          p99_ms: 0,
          samples: 0,
        },
        speed: data.speed?.current || {
          tokens_per_second: 0,
          time_to_first_token_ms: 0,
          decode_speed: 0,
        },
      }

      lastUpdated.value = new Date()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch metrics'
      console.error('Failed to fetch all metrics:', e)
    } finally {
      loading.value = false
    }
  }

  async function resetMetrics() {
    try {
      await metricsApi.resetMetrics()
      await fetchAll()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to reset metrics'
    }
  }

  function startAutoRefresh(interval?: number) {
    if (interval) {
      autoRefreshInterval.value = interval
    }
    stopAutoRefresh()
    autoRefreshEnabled.value = true
    refreshTimer = setInterval(() => {
      fetchAll()
    }, autoRefreshInterval.value)
  }

  function stopAutoRefresh() {
    autoRefreshEnabled.value = false
    if (refreshTimer) {
      clearInterval(refreshTimer)
      refreshTimer = null
    }
  }

  function setAutoRefreshInterval(interval: number) {
    autoRefreshInterval.value = interval
    if (autoRefreshEnabled.value) {
      startAutoRefresh()
    }
  }

  return {
    // State
    summary,
    callStats,
    modelStats,
    tokenUsage,
    latencyStats,
    speedStats,
    pricing,
    loading,
    error,
    lastUpdated,
    autoRefreshEnabled,
    autoRefreshInterval,

    // Computed
    successRate,
    totalCost,
    avgLatency,
    tokensPerSecond,

    // Actions
    fetchSummary,
    fetchCallStats,
    fetchModelStats,
    fetchTokenUsage,
    fetchLatencyStats,
    fetchSpeedStats,
    fetchPricing,
    fetchAll,
    resetMetrics,
    startAutoRefresh,
    stopAutoRefresh,
    setAutoRefreshInterval,
  }
})
