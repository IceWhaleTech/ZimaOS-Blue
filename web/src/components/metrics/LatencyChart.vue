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

function formatCount(value: number | undefined | null): string {
  if (value == null) return '0'
  return value.toLocaleString()
}

function formatRate(value: number | undefined | null): string {
  if (value == null) return '-'
  return value.toFixed(1)
}

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
    return percentiles.map((p) => ({
      ...p,
      height: p.value > 0 ? 80 : 5,
    }))
  }

  // Otherwise, scale based on range with minimum 10% height for non-zero values
  return percentiles.map((p) => ({
    ...p,
    height: p.value === 0 ? 5 : Math.max(10, ((p.value - min) / range) * 90 + 10),
  }))
})
</script>

<template>
  <div class="dashboard-card-surface latency-performance-card p-4">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy min-w-0">
          <p class="dashboard-card-label">Metrics</p>
          <p class="dashboard-card-subtitle mt-2">{{ t('metrics.latencyPerformance') }}</p>
        </div>
        <div class="flex flex-wrap items-center justify-end gap-2">
          <span v-if="latencyData" class="dashboard-card-chip">
            {{ formatCount(latencyData.samples ?? 0) }} {{ t('metrics.requests', 'Requests') }}
          </span>
        </div>
      </div>

      <div v-if="!latencyData" class="dashboard-card-empty">
        {{ t('metrics.noData') }}
      </div>

      <div v-else class="space-y-4">
        <div class="dashboard-card-subsurface latency-performance-hero p-4">
          <div class="latency-performance-head">
            <div class="dashboard-card-copy min-w-0">
              <p class="dashboard-card-value">{{ formatLatency(latencyData.avg ?? 0) }}</p>
              <p class="dashboard-card-footnote mt-2">{{ t('metrics.avgLatency') }}</p>
            </div>
            <div class="latency-performance-pill-group">
              <span class="latency-performance-pill">
                P95 {{ formatLatency(latencyData.p95 ?? 0) }}
              </span>
              <span class="latency-performance-pill">
                P99 {{ formatLatency(latencyData.p99 ?? 0) }}
              </span>
              <span class="latency-performance-pill">
                {{ formatLatency(latencyData.min ?? 0) }} -
                {{ formatLatency(latencyData.max ?? 0) }}
              </span>
            </div>
          </div>

          <div class="latency-performance-bars">
            <div
              v-for="bar in percentileBars"
              :key="bar.label"
              class="latency-performance-bar-column"
            >
              <div class="latency-performance-bar-track">
                <div
                  :class="[bar.color, 'latency-performance-bar-fill']"
                  :style="{ height: bar.height + '%' }"
                ></div>
              </div>
              <div
                class="mt-3 text-xs font-semibold text-gray-700 dark:text-gray-300 whitespace-nowrap"
              >
                {{ bar.label }}
              </div>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">
                {{ formatLatency(bar.value) }}
              </div>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
          <div class="dashboard-card-subsurface latency-performance-stat p-3 text-center">
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('metrics.avgLatency') }}
            </div>
            <div class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
              {{ formatLatency(latencyData.avg ?? 0) }}
            </div>
          </div>
          <div class="dashboard-card-subsurface latency-performance-stat p-3 text-center">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('metrics.p95') }}</div>
            <div class="mt-1 text-lg font-bold text-orange-600 dark:text-orange-400">
              {{ formatLatency(latencyData.p95 ?? 0) }}
            </div>
          </div>
          <div class="dashboard-card-subsurface latency-performance-stat p-3 text-center">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('metrics.p50') }}</div>
            <div class="mt-1 text-lg font-bold text-blue-600 dark:text-blue-400">
              {{ formatLatency(latencyData.p50 ?? 0) }}
            </div>
          </div>
          <div class="dashboard-card-subsurface latency-performance-stat p-3 text-center">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('metrics.p99') }}</div>
            <div class="mt-1 text-lg font-bold text-red-600 dark:text-red-400">
              {{ formatLatency(latencyData.p99 ?? 0) }}
            </div>
          </div>
        </div>

        <div class="space-y-3">
          <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('metrics.speedStats') }}
          </h4>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div class="dashboard-card-subsurface latency-performance-speed-card p-4">
              <div class="text-sm text-blue-600 dark:text-blue-400">
                {{ t('metrics.currentSpeed') }}
              </div>
              <div class="text-2xl font-bold text-blue-700 dark:text-blue-300">
                {{ formatRate(speedData?.current?.tokens_per_second) }}
              </div>
              <div class="text-xs text-blue-500 dark:text-blue-400">
                {{ t('metrics.tokensPerSecond') }}
              </div>
            </div>
            <div class="dashboard-card-subsurface latency-performance-speed-card p-4">
              <div class="text-sm text-green-600 dark:text-green-400">
                {{ t('metrics.avgSpeed') }}
              </div>
              <div class="text-2xl font-bold text-green-700 dark:text-green-300">
                {{ formatRate(speedData?.average?.tokens_per_second) }}
              </div>
              <div class="text-xs text-green-500 dark:text-green-400">
                {{ t('metrics.tokensPerSecond') }}
              </div>
            </div>
            <div class="dashboard-card-subsurface latency-performance-speed-card p-4">
              <div class="text-sm text-purple-600 dark:text-purple-400">
                {{ t('metrics.timeToFirstToken') }}
              </div>
              <div class="text-2xl font-bold text-purple-700 dark:text-purple-300">
                {{
                  speedData?.current?.time_to_first_token_ms == null
                    ? '-'
                    : formatLatency(speedData.current.time_to_first_token_ms)
                }}
              </div>
              <div class="text-xs text-purple-500 dark:text-purple-400">TTFT</div>
            </div>
          </div>

          <div class="text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('metrics.basedOn', { count: formatCount(latencyData.samples ?? 0) }) }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.latency-performance-card {
  border-color: rgba(196, 181, 253, 0.74);
  background:
    radial-gradient(circle at 100% 0%, rgba(196, 181, 253, 0.26), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(191, 219, 254, 0.22), transparent 40%),
    linear-gradient(180deg, #fdfdff 0%, #f3f4fb 100%);
}

.latency-performance-card .dashboard-card-label {
  background: rgba(237, 233, 254, 0.86);
  color: #6d28d9;
}

.latency-performance-hero {
  display: grid;
  gap: 1.1rem;
  border-color: rgba(196, 181, 253, 0.7);
  background:
    radial-gradient(circle at 100% 0%, rgba(237, 233, 254, 0.72), transparent 42%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(246, 247, 254, 0.98) 100%);
}

.latency-performance-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.9rem;
}

.latency-performance-pill-group {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.55rem;
}

.latency-performance-pill {
  display: inline-flex;
  align-items: center;
  padding: 0.4rem 0.72rem;
  border-radius: 999px;
  border: 1px solid rgba(196, 181, 253, 0.78);
  background: rgba(255, 255, 255, 0.8);
  color: #6b21a8;
  font-size: 0.74rem;
  font-weight: 600;
  white-space: nowrap;
}

.latency-performance-bars {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.8rem;
  min-height: 12.5rem;
}

.latency-performance-bar-column {
  display: flex;
  flex: 1 1 0;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
  min-width: 0;
  height: 100%;
}

.latency-performance-bar-track {
  display: flex;
  align-items: flex-end;
  width: 100%;
  max-width: 3rem;
  height: 8.75rem;
  padding: 0.28rem;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.88);
}

.latency-performance-bar-fill {
  width: 100%;
  border-radius: 999px;
  transition: height 0.3s ease;
}

.latency-performance-stat,
.latency-performance-speed-card {
  min-height: 5.35rem;
}

:root.dark .latency-performance-card,
[data-theme='dark'] .latency-performance-card,
html.dark .latency-performance-card {
  border-color: rgba(167, 139, 250, 0.24);
  background:
    radial-gradient(circle at 100% 0%, rgba(109, 40, 217, 0.12), transparent 46%),
    linear-gradient(180deg, rgba(30, 41, 59, 0.94) 0%, rgba(15, 23, 42, 0.96) 100%);
}

:root.dark .latency-performance-card .dashboard-card-label,
[data-theme='dark'] .latency-performance-card .dashboard-card-label,
html.dark .latency-performance-card .dashboard-card-label {
  background: rgba(76, 29, 149, 0.56);
  color: #ddd6fe;
}

:root.dark .latency-performance-hero,
[data-theme='dark'] .latency-performance-hero,
html.dark .latency-performance-hero {
  border-color: rgba(167, 139, 250, 0.18);
  background:
    radial-gradient(circle at 100% 0%, rgba(76, 29, 149, 0.22), transparent 42%),
    linear-gradient(180deg, rgba(30, 41, 59, 0.8) 0%, rgba(15, 23, 42, 0.88) 100%);
}

:root.dark .latency-performance-pill,
[data-theme='dark'] .latency-performance-pill,
html.dark .latency-performance-pill {
  border-color: rgba(71, 85, 105, 0.38);
  background: rgba(15, 23, 42, 0.62);
  color: #ddd6fe;
}

:root.dark .latency-performance-bar-track,
[data-theme='dark'] .latency-performance-bar-track,
html.dark .latency-performance-bar-track {
  background: rgba(15, 23, 42, 0.7);
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.05);
}

@media (max-width: 640px) {
  .latency-performance-head {
    flex-direction: column;
  }

  .latency-performance-pill-group {
    justify-content: flex-start;
  }

  .latency-performance-bars {
    gap: 0.55rem;
  }

  .latency-performance-bar-track {
    max-width: 2.3rem;
  }
}
</style>
