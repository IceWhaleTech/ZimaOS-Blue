<script setup lang="ts">
import { computed } from 'vue'

export interface SparklinePoint {
  timestamp?: string
  value: number
}

const props = withDefaults(
  defineProps<{
    data?: SparklinePoint[]
    maxValue?: number | null
    minValue?: number | null
  }>(),
  {
    data: () => [],
    maxValue: null,
    minValue: null,
  }
)

const WIDTH = 148
const HEIGHT = 74
const PADDING_X = 3
const PADDING_Y = 5
const DISPLAY_COLUMNS = 8
const COLUMN_GAP = 4

const normalizedData = computed<SparklinePoint[]>(() => {
  return (props.data ?? []).filter((point) => Number.isFinite(point.value))
})

function samplePoints(data: SparklinePoint[], count: number): SparklinePoint[] {
  if (data.length <= count) return data

  return Array.from({ length: count }, (_, index) => {
    const start = Math.floor((index * data.length) / count)
    const end = Math.floor(((index + 1) * data.length) / count)
    const bucket = data.slice(start, Math.max(start + 1, end))
    const total = bucket.reduce((sum, point) => sum + point.value, 0)
    const value = total / bucket.length
    return {
      timestamp: bucket[bucket.length - 1]?.timestamp,
      value,
    }
  })
}

const sparklineColumns = computed(() => {
  const data = samplePoints(normalizedData.value, DISPLAY_COLUMNS)
  const slotCount = DISPLAY_COLUMNS
  const innerWidth = WIDTH - PADDING_X * 2
  const gap = COLUMN_GAP
  const columnWidth = (innerWidth - gap * Math.max(slotCount - 1, 0)) / slotCount
  const filledOffset = Math.max(0, slotCount - data.length)

  return Array.from({ length: DISPLAY_COLUMNS }, (_, index) => {
    const point = index >= filledOffset ? (data[index - filledOffset] ?? null) : null
    const x = PADDING_X + index * (columnWidth + COLUMN_GAP)
    if (!point) {
      return {
        x,
        width: columnWidth,
        centerX: x + columnWidth / 2,
        markerY: HEIGHT / 2,
        markerX1: x + 3,
        markerX2: x + columnWidth - 3,
        hasValue: false,
      }
    }

    const values = data.map((point) => point.value)
    const rawMin = props.minValue ?? Math.min(...values)
    const rawMax = props.maxValue ?? Math.max(...values)
    const min = Math.min(rawMin, rawMax)
    const max = Math.max(rawMin, rawMax)
    const range = max - min
    const markerY =
      range === 0
        ? HEIGHT / 2
        : HEIGHT - PADDING_Y - ((point.value - min) / range) * (HEIGHT - PADDING_Y * 2)
    const markerInset = Math.max(3.8, columnWidth * 0.2)

    return {
      x,
      width: columnWidth,
      centerX: x + columnWidth / 2,
      markerY,
      markerX1: x + markerInset,
      markerX2: x + columnWidth - markerInset,
      hasValue: true,
    }
  })
})

const sparklineLinePath = computed(() => {
  const points = sparklineColumns.value.filter((column) => column.hasValue)
  if (points.length < 2) return ''

  return points
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.centerX} ${point.markerY}`)
    .join(' ')
})
</script>

<template>
  <div class="dashboard-mini-sparkline" aria-hidden="true">
    <svg viewBox="0 0 148 74" class="dashboard-mini-sparkline-svg" preserveAspectRatio="none">
      <rect
        v-for="(column, index) in sparklineColumns"
        :key="`bg-${index}`"
        :x="column.x"
        y="0.75"
        :width="column.width"
        :height="HEIGHT - 1.5"
        rx="8"
        class="dashboard-mini-sparkline-column"
      />
      <template v-for="(column, index) in sparklineColumns" :key="`marker-${index}`">
        <line
          v-if="column.hasValue"
          :x1="column.markerX1"
          :y1="column.markerY"
          :x2="column.markerX2"
          :y2="column.markerY"
          class="dashboard-mini-sparkline-marker"
        />
      </template>
      <path
        v-if="sparklineLinePath"
        :d="sparklineLinePath"
        class="dashboard-mini-sparkline-line"
        fill="none"
      />
    </svg>
  </div>
</template>

<style scoped>
.dashboard-mini-sparkline {
  width: clamp(4.8rem, 18vw, 7rem);
  max-width: 100%;
  height: 4.35rem;
  display: block;
  flex-shrink: 0;
}

.dashboard-mini-sparkline-svg {
  width: 100%;
  height: 100%;
  overflow: visible;
}

.dashboard-mini-sparkline-column {
  fill: #f3f5f8;
  stroke: rgba(226, 232, 240, 0.95);
  stroke-width: 0.8;
}

.dashboard-mini-sparkline-marker {
  stroke: #1f5eff;
  stroke-width: 6.25;
  stroke-linecap: round;
  filter: drop-shadow(0 1px 0 rgba(59, 130, 246, 0.22));
}

.dashboard-mini-sparkline-line {
  stroke: rgba(31, 94, 255, 0.72);
  stroke-width: 1.4;
  stroke-linecap: round;
  stroke-linejoin: round;
}

:root.dark .dashboard-mini-sparkline-column,
[data-theme='dark'] .dashboard-mini-sparkline-column,
html.dark .dashboard-mini-sparkline-column {
  fill: rgba(148, 163, 184, 0.2);
  stroke: rgba(148, 163, 184, 0.08);
}

:root.dark .dashboard-mini-sparkline-line,
[data-theme='dark'] .dashboard-mini-sparkline-line,
html.dark .dashboard-mini-sparkline-line {
  stroke: rgba(147, 197, 253, 0.72);
}
</style>
