<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ResourceChart from '@/components/ResourceChart.vue'
import type { DataPoint } from '@/components/ResourceChart.vue'
import { useSystemStore } from '@/stores/system'

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
const systemStore = useSystemStore()

const cpuChartData = computed<DataPoint[]>(() => {
  return (props.metricsHistory ?? []).map((m) => ({
    timestamp: m.timestamp,
    value: m.cpu_percent ?? 0,
  }))
})

const cpuCurrent = computed<number | null>(() => {
  const latest = props.metricsHistory[props.metricsHistory.length - 1]
  if (!latest) return null
  return Math.max(0, Math.min(100, latest.cpu_percent ?? 0))
})

const cpuAverage = computed(() => {
  if (cpuChartData.value.length === 0) return null
  const total = cpuChartData.value.reduce((sum, point) => sum + point.value, 0)
  return total / cpuChartData.value.length
})

const cpuPeak = computed(() => {
  if (cpuChartData.value.length === 0) return null
  return Math.max(...cpuChartData.value.map((point) => point.value))
})

const cpuBars = computed(() => {
  const values = props.metricsHistory
    .slice(-8)
    .map((m) => Math.max(0, Math.min(100, m.cpu_percent ?? 0)))
  if (values.length === 0) return [32, 40, 56, 48, 62, 44, 52, 36]

  const max = Math.max(...values, 1)
  const normalized = values.map((v) => Math.round(18 + (v / max) * 62))
  while (normalized.length < 8) normalized.unshift(normalized[0] ?? 34)
  return normalized.slice(-8)
})

const cpuSummaryItems = computed(() => [
  {
    label: t('resourceChart.avg', 'Avg'),
    value: cpuAverage.value == null ? '-' : `${cpuAverage.value.toFixed(1)}%`,
  },
  {
    label: t('metrics.max', 'Max'),
    value: cpuPeak.value == null ? '-' : `${cpuPeak.value.toFixed(1)}%`,
  },
  {
    label: t('system.cpuCores'),
    value: `${systemStore.health?.num_cpu ?? '-'}`,
  },
])
</script>

<template>
  <div v-if="compact" class="dashboard-card-stack">
    <div class="dashboard-card-footer">
      <div class="dashboard-card-copy">
        <p class="dashboard-card-label">{{ t('system.cpuUsage') }}</p>
        <p class="dashboard-card-subtitle mt-2">
          {{ t('system.cpuCores') }} · {{ systemStore.health?.num_cpu ?? '-' }}
        </p>
      </div>
      <span class="dashboard-card-chip">5m</span>
    </div>

    <div class="dashboard-card-footer">
      <div class="dashboard-card-copy">
        <p class="dashboard-card-value text-gray-900 dark:text-white">
          {{ cpuCurrent == null ? '-' : `${cpuCurrent.toFixed(0)}%` }}
        </p>
        <p class="dashboard-card-footnote">
          {{
            cpuPeak == null ? 'Awaiting peak sample' : `Peak ${cpuPeak.toFixed(0)}% over 5 minutes`
          }}
        </p>
      </div>

      <div class="dashboard-mini-bars flex-shrink-0">
        <div
          v-for="(level, index) in cpuBars"
          :key="index"
          class="dashboard-mini-bar"
          :style="{ '--bar-level': `${level}%` }"
        ></div>
      </div>
    </div>
  </div>

  <ResourceChart
    v-else
    :title="t('system.cpuUsage')"
    :subtitle="`${t('system.cpuCores')} · ${systemStore.health?.num_cpu ?? '-'}`"
    :data="cpuChartData"
    variant="dashboard"
    unit="%"
    color="blue"
    badge="5m"
    :caption="`Rolling compute pressure across the most recent 5 minute window`"
    :max-value="100"
    :format-value="(v: number) => v.toFixed(1)"
    :summary-items="cpuSummaryItems"
  />
</template>
