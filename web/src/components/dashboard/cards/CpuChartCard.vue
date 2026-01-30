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

const cpuChartData = computed<DataPoint[]>(() => {
  return (props.metricsHistory ?? []).map((m) => ({
    timestamp: m.timestamp,
    value: m.cpu_percent ?? 0,
  }))
})
</script>

<template>
  <ResourceChart
    :title="t('system.cpuUsage')"
    :data="cpuChartData"
    unit="%"
    color="blue"
    :max-value="100"
    :format-value="(v: number) => v.toFixed(1)"
  />
</template>
