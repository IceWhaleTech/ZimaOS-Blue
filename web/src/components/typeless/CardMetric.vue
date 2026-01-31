<script setup lang="ts">
import type { TypelessCardMetric, MetricItem } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardMetric
}>()

function getGridCols(): string {
  const cols = props.card.columns || 2
  return {
    2: 'grid-cols-2',
    3: 'grid-cols-3',
    4: 'grid-cols-4',
  }[cols]
}

function formatChange(change: number): string {
  const sign = change >= 0 ? '+' : ''
  return `${sign}${change.toFixed(1)}%`
}

function getChangeColor(item: MetricItem): string {
  if (!item.change) return ''
  if (item.changeDirection === 'neutral') return 'text-gray-500'
  if (item.changeDirection === 'up' || (!item.changeDirection && item.change > 0)) {
    return 'text-green-500'
  }
  return 'text-red-500'
}

function getChangeIcon(item: MetricItem): string {
  if (!item.change) return ''
  if (item.changeDirection === 'neutral') return '→'
  if (item.changeDirection === 'up' || (!item.changeDirection && item.change > 0)) {
    return '↑'
  }
  return '↓'
}
</script>

<template>
  <div class="metric-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
    <!-- Title -->
    <div v-if="card.title" class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
      <h4 class="font-medium text-gray-900 dark:text-white">{{ card.title }}</h4>
    </div>

    <!-- Metrics Grid -->
    <div class="p-4 grid gap-4" :class="getGridCols()">
      <div
        v-for="(metric, index) in card.metrics"
        :key="index"
        class="p-3 rounded-lg bg-gray-50 dark:bg-gray-800/50"
      >
        <!-- Icon and Label -->
        <div class="flex items-center gap-2 mb-2">
          <span v-if="metric.icon" class="text-lg">{{ metric.icon }}</span>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ metric.label }}</span>
        </div>

        <!-- Value -->
        <div class="flex items-baseline gap-1">
          <span class="text-2xl font-bold text-gray-900 dark:text-white">{{ metric.value }}</span>
          <span v-if="metric.unit" class="text-sm text-gray-500 dark:text-gray-400">{{ metric.unit }}</span>
        </div>

        <!-- Change indicator -->
        <div v-if="metric.change !== undefined" class="mt-2 flex items-center gap-1 text-sm" :class="getChangeColor(metric)">
          <span>{{ getChangeIcon(metric) }}</span>
          <span>{{ formatChange(metric.change) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
