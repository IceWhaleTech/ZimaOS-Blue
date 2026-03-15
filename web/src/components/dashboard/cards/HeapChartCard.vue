<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ResourceChart from '@/components/ResourceChart.vue'
import type { DataPoint } from '@/components/ResourceChart.vue'

const props = defineProps<{
  metricsHistory: Array<{
    timestamp: string
    cpu_percent: number
    memory_used_bytes: number
    goroutines: number
    heap_alloc_bytes: number
  }>
}>()

const { t } = useI18n()

const heapChartData = computed<DataPoint[]>(() => {
  return (props.metricsHistory ?? []).map((m) => ({
    timestamp: m.timestamp,
    value: (m.heap_alloc_bytes ?? 0) / (1024 * 1024), // Convert to MB
  }))
})

const currentHeapMb = computed<number | null>(() => {
  const latest = props.metricsHistory[props.metricsHistory.length - 1]
  if (!latest) return null
  return Math.max(0, (latest.heap_alloc_bytes ?? 0) / (1024 * 1024))
})

const averageHeapMb = computed<number | null>(() => {
  if (heapChartData.value.length === 0) return null
  const total = heapChartData.value.reduce((sum, point) => sum + point.value, 0)
  return total / heapChartData.value.length
})

const peakHeapMb = computed<number | null>(() => {
  if (heapChartData.value.length === 0) return null
  return Math.max(...heapChartData.value.map((point) => point.value))
})

function formatMbValue(value: number | null): string {
  if (value == null) return '-'
  if (value < 1024) return `${value.toFixed(1)}MB`
  return `${(value / 1024).toFixed(1)}GB`
}

const heapSummaryItems = computed(() => [
  {
    label: t('system.latest'),
    value: formatMbValue(currentHeapMb.value),
  },
  {
    label: t('resourceChart.avg', 'Avg'),
    value: formatMbValue(averageHeapMb.value),
  },
  {
    label: t('metrics.max', 'Max'),
    value: formatMbValue(peakHeapMb.value),
  },
])
</script>

<template>
  <ResourceChart
    :title="t('system.heapAllocation')"
    subtitle="Managed heap allocation across the latest 5 minutes"
    :data="heapChartData"
    variant="dashboard"
    unit=""
    color="orange"
    badge="5m"
    caption="Tracked heap growth and retention inside the most recent runtime window"
    :format-value="(v: number) => formatMbValue(v)"
    :summary-items="heapSummaryItems"
  />
</template>
