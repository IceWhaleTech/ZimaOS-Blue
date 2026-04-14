<script setup lang="ts">
import { computed } from 'vue'
import type { TypelessCardChart, ChartDataPoint } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardChart
}>()

const total = computed(() => props.card.data.reduce((sum, d) => sum + d.value, 0))
const maxValue = computed(() => Math.max(...props.card.data.map((d) => d.value)))

const defaultColors = [
  '#3b82f6', // blue
  '#22c55e', // green
  '#f59e0b', // amber
  '#ef4444', // red
  '#8b5cf6', // violet
  '#06b6d4', // cyan
  '#ec4899', // pink
  '#f97316', // orange
]

function getColor(item: ChartDataPoint, index: number): string {
  return item.color || defaultColors[index % defaultColors.length] || '#3b82f6'
}

function getBarWidth(value: number): string {
  return `${(value / maxValue.value) * 100}%`
}

function getPieAngle(value: number, startAngle: number): { start: number; end: number } {
  const angle = (value / total.value) * 360
  return { start: startAngle, end: startAngle + angle }
}

function getPieSlicePath(
  value: number,
  startAngle: number,
  radius: number,
  cx: number,
  cy: number
): string {
  const { start, end } = getPieAngle(value, startAngle)
  const startRad = ((start - 90) * Math.PI) / 180
  const endRad = ((end - 90) * Math.PI) / 180

  const x1 = cx + radius * Math.cos(startRad)
  const y1 = cy + radius * Math.sin(startRad)
  const x2 = cx + radius * Math.cos(endRad)
  const y2 = cy + radius * Math.sin(endRad)

  const largeArc = end - start > 180 ? 1 : 0

  return `M ${cx} ${cy} L ${x1} ${y1} A ${radius} ${radius} 0 ${largeArc} 1 ${x2} ${y2} Z`
}

function getDonutSlicePath(
  value: number,
  startAngle: number,
  outerRadius: number,
  innerRadius: number,
  cx: number,
  cy: number
): string {
  const { start, end } = getPieAngle(value, startAngle)
  const startRad = ((start - 90) * Math.PI) / 180
  const endRad = ((end - 90) * Math.PI) / 180

  const x1 = cx + outerRadius * Math.cos(startRad)
  const y1 = cy + outerRadius * Math.sin(startRad)
  const x2 = cx + outerRadius * Math.cos(endRad)
  const y2 = cy + outerRadius * Math.sin(endRad)
  const x3 = cx + innerRadius * Math.cos(endRad)
  const y3 = cy + innerRadius * Math.sin(endRad)
  const x4 = cx + innerRadius * Math.cos(startRad)
  const y4 = cy + innerRadius * Math.sin(startRad)

  const largeArc = end - start > 180 ? 1 : 0

  return `M ${x1} ${y1} A ${outerRadius} ${outerRadius} 0 ${largeArc} 1 ${x2} ${y2} L ${x3} ${y3} A ${innerRadius} ${innerRadius} 0 ${largeArc} 0 ${x4} ${y4} Z`
}

const pieSlices = computed(() => {
  let currentAngle = 0
  return props.card.data.map((item, index) => {
    const slice = {
      path:
        props.card.chartType === 'donut'
          ? getDonutSlicePath(item.value, currentAngle, 80, 50, 100, 100)
          : getPieSlicePath(item.value, currentAngle, 80, 100, 100),
      color: getColor(item, index),
      label: item.label,
      value: item.value,
      percentage: ((item.value / total.value) * 100).toFixed(1),
    }
    currentAngle += (item.value / total.value) * 360
    return slice
  })
})

// Line chart points
const linePoints = computed(() => {
  if (props.card.chartType !== 'line') return ''
  const width = 280
  const height = 120
  const padding = 20
  const chartWidth = width - padding * 2
  const chartHeight = height - padding * 2

  return props.card.data
    .map((item, index) => {
      const x = padding + (index / (props.card.data.length - 1)) * chartWidth
      const y = padding + chartHeight - (item.value / maxValue.value) * chartHeight
      return `${x},${y}`
    })
    .join(' ')
})
</script>

<template>
  <div
    class="chart-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
  >
    <!-- Title -->
    <div
      v-if="card.title"
      class="px-4 py-3 border-b border-gray-200 dark:border-gray-700"
    >
      <h4 class="font-medium text-gray-900 dark:text-white">
        {{ card.title }}
      </h4>
    </div>

    <!-- Bar Chart -->
    <div
      v-if="card.chartType === 'bar'"
      class="p-4 space-y-3"
    >
      <div
        v-for="(item, index) in card.data"
        :key="index"
        class="space-y-1"
      >
        <div class="flex justify-between text-sm">
          <span class="text-gray-600 dark:text-gray-400">{{ item.label }}</span>
          <span
            v-if="card.showValues"
            class="text-gray-900 dark:text-white font-medium"
          >{{
            item.value
          }}</span>
        </div>
        <div class="h-4 bg-gray-100 dark:bg-gray-700 rounded-full overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-500"
            :style="{ width: getBarWidth(item.value), backgroundColor: getColor(item, index) }"
          />
        </div>
      </div>
    </div>

    <!-- Line Chart -->
    <div
      v-else-if="card.chartType === 'line'"
      class="p-4"
    >
      <svg
        viewBox="0 0 280 120"
        class="w-full h-32"
      >
        <!-- Grid lines -->
        <line
          x1="20"
          y1="20"
          x2="20"
          y2="100"
          stroke="currentColor"
          class="text-gray-200 dark:text-gray-700"
        />
        <line
          x1="20"
          y1="100"
          x2="260"
          y2="100"
          stroke="currentColor"
          class="text-gray-200 dark:text-gray-700"
        />

        <!-- Line -->
        <polyline
          :points="linePoints"
          fill="none"
          stroke="#3b82f6"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />

        <!-- Points -->
        <circle
          v-for="(item, index) in card.data"
          :key="index"
          :cx="20 + (index / (card.data.length - 1)) * 240"
          :cy="20 + 80 - (item.value / maxValue) * 80"
          r="4"
          :fill="getColor(item, index)"
        />
      </svg>

      <!-- Labels -->
      <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400 mt-2 px-2">
        <span
          v-for="(item, index) in card.data"
          :key="index"
        >{{ item.label }}</span>
      </div>
    </div>

    <!-- Pie/Donut Chart -->
    <div
      v-else-if="card.chartType === 'pie' || card.chartType === 'donut'"
      class="p-4 flex items-center justify-center"
    >
      <svg
        viewBox="0 0 200 200"
        class="w-40 h-40"
      >
        <path
          v-for="(slice, index) in pieSlices"
          :key="index"
          :d="slice.path"
          :fill="slice.color"
          class="transition-opacity hover:opacity-80 cursor-pointer"
        />
        <!-- Center text for donut -->
        <text
          v-if="card.chartType === 'donut'"
          x="100"
          y="100"
          text-anchor="middle"
          dominant-baseline="middle"
          class="text-2xl font-bold fill-gray-900 dark:fill-white"
        >
          {{ total }}
        </text>
      </svg>
    </div>

    <!-- Legend -->
    <div
      v-if="card.showLegend !== false"
      class="px-4 pb-4 flex flex-wrap gap-3"
    >
      <div
        v-for="(item, index) in card.data"
        :key="index"
        class="flex items-center gap-2 text-sm"
      >
        <div
          class="w-3 h-3 rounded-full"
          :style="{ backgroundColor: getColor(item, index) }"
        />
        <span class="text-gray-600 dark:text-gray-400">{{ item.label }}</span>
        <span
          v-if="card.showValues"
          class="text-gray-900 dark:text-white font-medium"
        >({{ item.value }})</span>
      </div>
    </div>
  </div>
</template>
