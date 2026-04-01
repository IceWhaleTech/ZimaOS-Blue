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

const goroutinesChartData = computed<DataPoint[]>(() => {
  return (props.metricsHistory ?? []).map((m) => ({
    timestamp: m.timestamp,
    value: m.goroutines ?? 0,
  }))
})

const currentGoroutines = computed<number | null>(() => {
  const latest = props.metricsHistory[props.metricsHistory.length - 1]
  if (!latest) return null
  return Math.max(0, latest.goroutines ?? 0)
})

const averageGoroutines = computed<number | null>(() => {
  if (goroutinesChartData.value.length === 0) return null
  const total = goroutinesChartData.value.reduce((sum, point) => sum + point.value, 0)
  return total / goroutinesChartData.value.length
})

const peakGoroutines = computed<number | null>(() => {
  if (goroutinesChartData.value.length === 0) return null
  return Math.max(...goroutinesChartData.value.map((point) => point.value))
})

const goroutinesSummaryItems = computed(() => [
  {
    label: t('system.latest'),
    value: currentGoroutines.value == null ? '-' : currentGoroutines.value.toFixed(0),
  },
  {
    label: t('resourceChart.avg'),
    value: averageGoroutines.value == null ? '-' : averageGoroutines.value.toFixed(0),
  },
  {
    label: t('metrics.max'),
    value: peakGoroutines.value == null ? '-' : peakGoroutines.value.toFixed(0),
  },
])
</script>

<template>
  <ResourceChart
    :title="t('system.goroutines')"
    :subtitle="t('system.cards.goroutinesChart.chartSubtitle')"
    :data="goroutinesChartData"
    variant="dashboard"
    unit=""
    color="purple"
    badge="5m"
    :caption="t('system.cards.goroutinesChart.chartCaption')"
    :format-value="(v: number) => v.toFixed(0)"
    :summary-items="goroutinesSummaryItems"
  />
</template>
