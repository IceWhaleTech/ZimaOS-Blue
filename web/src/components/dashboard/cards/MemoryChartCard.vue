<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ResourceChart from '@/components/ResourceChart.vue'
import type { DataPoint } from '@/components/ResourceChart.vue'

const props = withDefaults(
  defineProps<{
    metricsHistory?: Array<{
      timestamp: string
      cpu_percent: number
      memory_used_bytes: number
      goroutines: number
      heap_alloc_bytes: number
    }>
    compact?: boolean
  }>(),
  {
    metricsHistory: () => [],
    compact: false,
  }
)

const { t } = useI18n()

const memoryChartData = computed<DataPoint[]>(() => {
  return (props.metricsHistory ?? []).map((m) => ({
    timestamp: m.timestamp,
    value: (m.memory_used_bytes ?? 0) / (1024 * 1024), // Convert to MB
  }))
})

const memoryAverageBytes = computed<number | null>(() => {
  const values = props.metricsHistory.map((m) => Math.max(0, m.memory_used_bytes ?? 0))
  if (values.length === 0) return null
  return values.reduce((sum, value) => sum + value, 0) / values.length
})

const memoryCurrentBytes = computed<number | null>(() => {
  const latest = props.metricsHistory[props.metricsHistory.length - 1]
  if (!latest) return null
  return Math.max(0, latest.memory_used_bytes ?? 0)
})

const memoryPeakBytes = computed<number | null>(() => {
  const values = props.metricsHistory.map((m) => Math.max(0, m.memory_used_bytes ?? 0))
  if (values.length === 0) return null
  return Math.max(...values)
})

const memoryBars = computed(() => {
  const values = props.metricsHistory.slice(-8).map((m) => Math.max(0, m.memory_used_bytes ?? 0))
  if (values.length === 0) return [58, 36, 74, 48, 58, 33, 29, 26]

  const max = Math.max(...values, 1)
  const normalized = values.map((v) => Math.round(18 + (v / max) * 62))
  while (normalized.length < 8) normalized.unshift(normalized[0] ?? 36)
  return normalized.slice(-8)
})

function formatCompactBytes(bytes: number | null): string {
  if (bytes == null) return '-'
  if (bytes < 1024) return `${bytes.toFixed(0)}B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)}KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(0)}MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}GB`
}

function formatMbValue(value: number): string {
  if (value < 1024) return `${value.toFixed(0)}MB`
  return `${(value / 1024).toFixed(1)}GB`
}

const memorySummaryItems = computed(() => [
  {
    label: t('system.latest'),
    value: formatCompactBytes(memoryCurrentBytes.value),
  },
  {
    label: t('resourceChart.avg', 'Avg'),
    value: formatCompactBytes(memoryAverageBytes.value),
  },
  {
    label: t('metrics.max', 'Max'),
    value: formatCompactBytes(memoryPeakBytes.value),
  },
])
</script>

<template>
  <div v-if="compact" class="dashboard-card-stack">
    <div class="dashboard-card-footer">
      <div class="dashboard-card-copy">
        <p class="dashboard-card-label">{{ t('system.memoryUsage') }}</p>
        <p class="dashboard-card-subtitle mt-2">5 minute high-water mark</p>
      </div>
      <span class="dashboard-card-chip">Max</span>
    </div>

    <div class="dashboard-card-footer">
      <div class="dashboard-card-copy">
        <p class="dashboard-card-value text-gray-900 dark:text-white">
          {{ formatCompactBytes(memoryPeakBytes) }}
        </p>
        <p class="dashboard-card-footnote">
          {{
            memoryCurrentBytes == null
              ? 'Awaiting recent sample'
              : `Current ${formatCompactBytes(memoryCurrentBytes)}`
          }}
        </p>
      </div>

      <div class="dashboard-mini-bars flex-shrink-0">
        <div
          v-for="(level, index) in memoryBars"
          :key="index"
          class="dashboard-mini-bar"
          :style="{ '--bar-level': `${level}%` }"
        ></div>
      </div>
    </div>
  </div>

  <ResourceChart
    v-else
    :title="t('system.memoryUsage')"
    subtitle="Allocator footprint over the latest 5 minutes"
    :data="memoryChartData"
    variant="dashboard"
    unit=""
    color="purple"
    badge="5m"
    caption="Heap-backed allocation sampled across the most recent runtime window"
    :format-value="formatMbValue"
    :summary-items="memorySummaryItems"
  />
</template>
