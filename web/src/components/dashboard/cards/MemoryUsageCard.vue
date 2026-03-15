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

function formatBytes(bytes: number | undefined | null): string {
  if (bytes == null) return '-'
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const allocationTrendData = computed<SparklinePoint[]>(() => {
  return (props.metricsHistory ?? []).map((entry) => ({
    timestamp: entry.timestamp,
    value: Math.max(0, entry.heap_alloc_bytes ?? 0),
  }))
})
</script>

<template>
  <div class="dashboard-card-stack">
    <div class="dashboard-card-footer">
      <div class="dashboard-card-copy">
        <p class="dashboard-card-label">{{ t('system.heapAllocation') }}</p>
        <p class="dashboard-card-subtitle mt-2">Current heap alloc</p>
      </div>
      <span class="dashboard-card-chip">Now</span>
    </div>

    <div class="dashboard-card-footer memory-card-main">
      <div class="dashboard-card-copy">
        <template v-if="systemStore.loading">
          <Skeleton height="1.75rem" width="70%" rounded="md" />
        </template>
        <p v-else class="dashboard-card-value text-gray-900 dark:text-white">
          {{ formatBytes(systemStore.health?.mem_alloc_bytes) }}
        </p>
        <p class="dashboard-card-footnote mt-3">Live heap allocator footprint</p>
      </div>

      <DashboardSparkline :data="allocationTrendData" />
    </div>
  </div>
</template>

<style scoped>
.memory-card-main {
  align-items: center;
}
</style>
