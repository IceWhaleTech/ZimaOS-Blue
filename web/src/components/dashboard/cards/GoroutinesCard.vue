<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DashboardSparkline from '@/components/dashboard/DashboardSparkline.vue'
import type { SparklinePoint } from '@/components/dashboard/DashboardSparkline.vue'
import { useSystemStore } from '@/stores/system'
import Skeleton from '@/components/Skeleton.vue'

const props = withDefaults(
  defineProps<{
    metricsHistory?: Array<{
      timestamp: string
      cpu_percent: number
      memory_used_bytes: number
      goroutines: number
      heap_alloc_bytes: number
    }>
  }>(),
  {
    metricsHistory: () => [],
  }
)

const { t } = useI18n()
const systemStore = useSystemStore()

const workerTrendData = computed<SparklinePoint[]>(() => {
  return (props.metricsHistory ?? []).map((entry) => ({
    timestamp: entry.timestamp,
    value: Math.max(0, entry.goroutines ?? 0),
  }))
})
</script>

<template>
  <div class="dashboard-card-stack">
    <div class="dashboard-card-footer">
      <div class="dashboard-card-copy">
        <p class="dashboard-card-label">{{ t('system.goroutines') }}</p>
        <p class="dashboard-card-subtitle mt-2">Scheduler load</p>
      </div>
      <span class="dashboard-card-chip">Live</span>
    </div>

    <div class="dashboard-card-footer goroutines-card-main">
      <div class="dashboard-card-copy">
        <template v-if="systemStore.loading">
          <Skeleton height="1.75rem" width="50%" rounded="md" />
        </template>
        <p v-else class="dashboard-card-value text-gray-900 dark:text-white">
          {{ systemStore.health?.goroutines ?? '-' }}
        </p>
        <p class="dashboard-card-footnote mt-3">Active routines in the current scheduler window</p>
      </div>

      <DashboardSparkline :data="workerTrendData" />
    </div>
  </div>
</template>

<style scoped>
.goroutines-card-main {
  align-items: center;
}
</style>
