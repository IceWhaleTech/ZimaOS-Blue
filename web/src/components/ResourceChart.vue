<script setup lang="ts">
import { computed } from 'vue'

export interface DataPoint {
  timestamp: string
  value: number
}

const props = withDefaults(
  defineProps<{
    title: string
    data: DataPoint[]
    unit?: string
    color?: string
    maxValue?: number
    formatValue?: (value: number) => string
  }>(),
  {
    unit: '',
    color: 'blue',
    maxValue: 0,
    formatValue: (v: number) => v.toFixed(1),
  }
)

const colorClasses = computed(() => {
  const colors: Record<string, { line: string; fill: string; text: string }> = {
    blue: { line: 'stroke-blue-500', fill: 'fill-blue-500/20', text: 'text-blue-400' },
    green: { line: 'stroke-green-500', fill: 'fill-green-500/20', text: 'text-green-400' },
    purple: { line: 'stroke-purple-500', fill: 'fill-purple-500/20', text: 'text-purple-400' },
    orange: { line: 'stroke-orange-500', fill: 'fill-orange-500/20', text: 'text-orange-400' },
    red: { line: 'stroke-red-500', fill: 'fill-red-500/20', text: 'text-red-400' },
  }
  return colors[props.color] ?? colors.blue!
})

const chartData = computed(() => {
  if (props.data.length === 0) return { path: '', areaPath: '', points: [] }

  const max = props.maxValue > 0 ? props.maxValue : Math.max(...props.data.map((d) => d.value), 1)
  const width = 300
  const height = 80
  const padding = 2

  const points = props.data.map((d, i) => ({
    x: padding + (i / Math.max(props.data.length - 1, 1)) * (width - padding * 2),
    y: height - padding - (d.value / max) * (height - padding * 2),
    value: d.value,
    timestamp: d.timestamp,
  }))

  if (points.length === 1) {
    return {
      path: '',
      areaPath: '',
      points,
    }
  }

  const pathPoints = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ')
  const lastPoint = points[points.length - 1]
  const firstPoint = points[0]
  const areaPath = lastPoint && firstPoint
    ? `${pathPoints} L ${lastPoint.x} ${height - padding} L ${firstPoint.x} ${height - padding} Z`
    : ''

  return {
    path: pathPoints,
    areaPath,
    points,
  }
})

const currentValue = computed(() => {
  if (props.data.length === 0) return '-'
  const lastData = props.data[props.data.length - 1]
  return lastData ? props.formatValue(lastData.value) : '-'
})

const minValue = computed(() => {
  if (props.data.length === 0) return '-'
  return props.formatValue(Math.min(...props.data.map((d) => d.value)))
})

const maxValueDisplay = computed(() => {
  if (props.data.length === 0) return '-'
  return props.formatValue(Math.max(...props.data.map((d) => d.value)))
})

const avgValue = computed(() => {
  if (props.data.length === 0) return '-'
  const sum = props.data.reduce((acc, d) => acc + d.value, 0)
  return props.formatValue(sum / props.data.length)
})

const lastChartPoint = computed(() => {
  const points = chartData.value.points
  return points.length > 0 ? points[points.length - 1] : null
})
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow">
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ title }}</h3>
      <span :class="['text-lg font-bold', colorClasses.text]">
        {{ currentValue }}{{ unit }}
      </span>
    </div>

    <!-- Chart -->
    <div class="relative h-20 mb-3">
      <svg
        v-if="data.length > 0"
        viewBox="0 0 300 80"
        class="w-full h-full"
        preserveAspectRatio="none"
      >
        <!-- Grid lines -->
        <line
          x1="0"
          y1="20"
          x2="300"
          y2="20"
          class="stroke-gray-200 dark:stroke-gray-700"
          stroke-dasharray="4"
        />
        <line
          x1="0"
          y1="40"
          x2="300"
          y2="40"
          class="stroke-gray-200 dark:stroke-gray-700"
          stroke-dasharray="4"
        />
        <line
          x1="0"
          y1="60"
          x2="300"
          y2="60"
          class="stroke-gray-200 dark:stroke-gray-700"
          stroke-dasharray="4"
        />

        <!-- Area fill -->
        <path
          v-if="chartData.areaPath"
          :d="chartData.areaPath"
          :class="colorClasses.fill"
        />

        <!-- Line -->
        <path
          v-if="chartData.path"
          :d="chartData.path"
          fill="none"
          :class="colorClasses.line"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />

        <!-- Current value dot -->
        <circle
          v-if="lastChartPoint"
          :cx="lastChartPoint.x"
          :cy="lastChartPoint.y"
          r="4"
          :class="colorClasses.line"
          fill="currentColor"
        />
      </svg>

      <!-- No data placeholder -->
      <div
        v-else
        class="absolute inset-0 flex items-center justify-center text-gray-400 dark:text-gray-500 text-sm"
      >
        No data available
      </div>
    </div>

    <!-- Stats -->
    <div class="grid grid-cols-3 gap-2 text-xs">
      <div class="text-center">
        <div class="text-gray-400 dark:text-gray-500">Min</div>
        <div class="text-gray-700 dark:text-gray-300">{{ minValue }}{{ unit }}</div>
      </div>
      <div class="text-center">
        <div class="text-gray-400 dark:text-gray-500">Avg</div>
        <div class="text-gray-700 dark:text-gray-300">{{ avgValue }}{{ unit }}</div>
      </div>
      <div class="text-center">
        <div class="text-gray-400 dark:text-gray-500">Max</div>
        <div class="text-gray-700 dark:text-gray-300">{{ maxValueDisplay }}{{ unit }}</div>
      </div>
    </div>
  </div>
</template>
