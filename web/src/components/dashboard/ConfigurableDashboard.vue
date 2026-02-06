<script setup lang="ts">
import { computed, markRaw, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDashboardStore } from '@/stores/dashboard'
import DashboardCustomizer from './DashboardCustomizer.vue'

// Import card components
import SystemStatusCard from './cards/SystemStatusCard.vue'
import UptimeCard from './cards/UptimeCard.vue'
import MemoryUsageCard from './cards/MemoryUsageCard.vue'
import GoroutinesCard from './cards/GoroutinesCard.vue'
import SystemInfoCard from './cards/SystemInfoCard.vue'
import CpuChartCard from './cards/CpuChartCard.vue'
import MemoryChartCard from './cards/MemoryChartCard.vue'
import GoroutinesChartCard from './cards/GoroutinesChartCard.vue'
import HeapChartCard from './cards/HeapChartCard.vue'
import FailoverStatusCard from './cards/FailoverStatusCard.vue'
// Metrics cards
import MetricsOverviewCard from './cards/MetricsOverviewCard.vue'
import TokenUsageChartCard from './cards/TokenUsageChartCard.vue'
import LatencyChartCard from './cards/LatencyChartCard.vue'
import ModelStatsCard from './cards/ModelStatsCard.vue'
import CacheStatsCard from './cards/CacheStatsCard.vue'

defineProps<{
  metricsHistory?: Array<{
    timestamp: string
    cpu_percent: number
    memory_used_bytes: number
    goroutines: number
    heap_alloc_bytes: number
  }>
}>()

const { t } = useI18n()
const dashboardStore = useDashboardStore()

// Map string component names to actual components
const componentMap: Record<string, Component> = {
  SystemStatusCard: markRaw(SystemStatusCard),
  UptimeCard: markRaw(UptimeCard),
  MemoryUsageCard: markRaw(MemoryUsageCard),
  GoroutinesCard: markRaw(GoroutinesCard),
  SystemInfoCard: markRaw(SystemInfoCard),
  CpuChartCard: markRaw(CpuChartCard),
  MemoryChartCard: markRaw(MemoryChartCard),
  GoroutinesChartCard: markRaw(GoroutinesChartCard),
  HeapChartCard: markRaw(HeapChartCard),
  FailoverStatusCard: markRaw(FailoverStatusCard),
  // Metrics cards
  MetricsOverviewCard: markRaw(MetricsOverviewCard),
  TokenUsageChartCard: markRaw(TokenUsageChartCard),
  LatencyChartCard: markRaw(LatencyChartCard),
  ModelStatsCard: markRaw(ModelStatsCard),
  CacheStatsCard: markRaw(CacheStatsCard),
}

// Get component from config
function getComponent(config: { component: Component | string }): Component | null {
  if (typeof config.component === 'string') {
    return componentMap[config.component] ?? null
  }
  return config.component
}

// Get grid column class based on minWidth
function getGridClass(minWidth?: number): string {
  switch (minWidth) {
    case 1:
      return 'col-span-1'
    case 2:
      return 'col-span-1 lg:col-span-2'
    case 3:
      return 'col-span-1 lg:col-span-3'
    case 4:
      return 'col-span-1 lg:col-span-4'
    default:
      return 'col-span-1'
  }
}

// Check if card needs metricsHistory prop
function needsMetricsHistory(cardId: string): boolean {
  return ['cpu-chart', 'memory-chart', 'goroutines-chart', 'heap-chart'].includes(cardId)
}

// Group cards by size for better layout
const smallCards = computed(() => {
  return dashboardStore.enabledCards.filter((card) => card.config && (card.config.minWidth ?? 1) === 1)
})

const largeCards = computed(() => {
  return dashboardStore.enabledCards.filter((card) => card.config && (card.config.minWidth ?? 1) > 1)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header with Customizer -->
    <div class="flex items-center justify-between">
      <slot name="header-left"></slot>
      <DashboardCustomizer />
    </div>

    <!-- Small Cards Grid (1-column cards) -->
    <div v-if="smallCards.length > 0" class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <div
        v-for="card in smallCards"
        :key="card.id"
        class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow"
      >
        <component
          :is="getComponent(card.config!)"
          v-if="card.config && getComponent(card.config)"
          v-bind="needsMetricsHistory(card.id) ? { metricsHistory } : {}"
        />
      </div>
    </div>

    <!-- Large Cards Grid (2+ column cards) -->
    <div v-if="largeCards.length > 0" class="grid grid-cols-1 lg:grid-cols-4 gap-4">
      <div
        v-for="card in largeCards"
        :key="card.id"
        :class="getGridClass(card.config?.minWidth)"
      >
        <component
          :is="getComponent(card.config!)"
          v-if="card.config && getComponent(card.config)"
          v-bind="needsMetricsHistory(card.id) ? { metricsHistory } : {}"
        />
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-if="dashboardStore.enabledCards.length === 0"
      class="text-center py-12 bg-white dark:bg-gray-700 rounded-lg shadow"
    >
      <svg
        class="mx-auto h-12 w-12 text-gray-400"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z"
        />
      </svg>
      <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
        {{ t('dashboard.noCardsEnabled') || 'No cards enabled' }}
      </h3>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('dashboard.clickCustomize') || 'Click the customize button to add cards' }}
      </p>
    </div>
  </div>
</template>
