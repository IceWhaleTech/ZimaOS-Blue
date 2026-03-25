<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

export interface DataPoint {
  timestamp: string
  value: number
}

interface ResourceChartPalette {
  stroke: string
  fill: string
  soft: string
  softBorder: string
  softText: string
  grid: string
}

const defaultResourceChartPalette: ResourceChartPalette = {
  stroke: '#2563eb',
  fill: 'rgba(37, 99, 235, 0.14)',
  soft: 'rgba(219, 234, 254, 0.72)',
  softBorder: 'rgba(147, 197, 253, 0.58)',
  softText: '#1d4ed8',
  grid: 'rgba(148, 163, 184, 0.24)',
}

const resourceChartPalettes: Record<string, ResourceChartPalette> = {
  blue: defaultResourceChartPalette,
  green: {
    stroke: '#16a34a',
    fill: 'rgba(22, 163, 74, 0.14)',
    soft: 'rgba(220, 252, 231, 0.72)',
    softBorder: 'rgba(134, 239, 172, 0.58)',
    softText: '#166534',
    grid: 'rgba(134, 239, 172, 0.18)',
  },
  purple: {
    stroke: '#7c3aed',
    fill: 'rgba(124, 58, 237, 0.14)',
    soft: 'rgba(237, 233, 254, 0.74)',
    softBorder: 'rgba(196, 181, 253, 0.58)',
    softText: '#6d28d9',
    grid: 'rgba(196, 181, 253, 0.2)',
  },
  orange: {
    stroke: '#ea580c',
    fill: 'rgba(234, 88, 12, 0.14)',
    soft: 'rgba(255, 237, 213, 0.76)',
    softBorder: 'rgba(251, 191, 36, 0.58)',
    softText: '#c2410c',
    grid: 'rgba(251, 191, 36, 0.2)',
  },
  red: {
    stroke: '#dc2626',
    fill: 'rgba(220, 38, 38, 0.14)',
    soft: 'rgba(254, 226, 226, 0.76)',
    softBorder: 'rgba(252, 165, 165, 0.56)',
    softText: '#b91c1c',
    grid: 'rgba(252, 165, 165, 0.2)',
  },
}

const props = withDefaults(
  defineProps<{
    title: string
    data: DataPoint[]
    unit?: string
    color?: string
    maxValue?: number
    formatValue?: (value: number) => string
    variant?: 'default' | 'dashboard'
    subtitle?: string
    badge?: string
    caption?: string
    summaryItems?: Array<{
      label: string
      value: string
    }>
  }>(),
  {
    unit: '',
    color: 'blue',
    maxValue: 0,
    formatValue: (v: number) => v.toFixed(1),
    variant: 'default',
    subtitle: '',
    badge: '',
    caption: '',
    summaryItems: () => [],
  }
)

const isDashboardVariant = computed(() => props.variant === 'dashboard')

const colorClasses = computed(() => {
  const colors: Record<string, { line: string; fill: string; text: string }> = {
    blue: {
      line: 'stroke-gray-900 dark:stroke-gray-400',
      fill: 'fill-gray-900/10 dark:fill-gray-400/10',
      text: 'text-gray-900 dark:text-gray-400',
    },
    green: { line: 'stroke-green-500', fill: 'fill-green-500/20', text: 'text-green-400' },
    purple: { line: 'stroke-purple-500', fill: 'fill-purple-500/20', text: 'text-purple-400' },
    orange: { line: 'stroke-orange-500', fill: 'fill-orange-500/20', text: 'text-orange-400' },
    red: { line: 'stroke-red-500', fill: 'fill-red-500/20', text: 'text-red-400' },
  }
  return colors[props.color] ?? colors.blue!
})

const palette = computed<ResourceChartPalette>(
  () => resourceChartPalettes[props.color] ?? defaultResourceChartPalette
)

const DASHBOARD_COLUMN_COUNT = 8
const DASHBOARD_CHART_WIDTH = 300
const DASHBOARD_CHART_HEIGHT = 80
const DASHBOARD_CHART_PADDING_X = 5
const DASHBOARD_CHART_PADDING_Y = 3
const DASHBOARD_CHART_GAP = 7

const chartVars = computed<Record<string, string>>(() => ({
  '--resource-chart-stroke': palette.value.stroke,
  '--resource-chart-fill': palette.value.fill,
  '--resource-chart-soft': palette.value.soft,
  '--resource-chart-soft-border': palette.value.softBorder,
  '--resource-chart-soft-text': palette.value.softText,
  '--resource-chart-grid': palette.value.grid,
}))

function sampleDataPoints(data: DataPoint[], count: number): DataPoint[] {
  if (data.length <= count) return data

  return Array.from({ length: count }, (_, index) => {
    const start = Math.floor((index * data.length) / count)
    const end = Math.floor(((index + 1) * data.length) / count)
    const bucket = data.slice(start, Math.max(start + 1, end))
    const total = bucket.reduce((sum, point) => sum + point.value, 0)

    return {
      timestamp: bucket[bucket.length - 1]?.timestamp ?? '',
      value: total / bucket.length,
    }
  })
}

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
  const areaPath =
    lastPoint && firstPoint
      ? `${pathPoints} L ${lastPoint.x} ${height - padding} L ${firstPoint.x} ${height - padding} Z`
      : ''

  return {
    path: pathPoints,
    areaPath,
    points,
  }
})

const dashboardColumns = computed(() => {
  const data = sampleDataPoints(props.data, DASHBOARD_COLUMN_COUNT)
  const slotCount = DASHBOARD_COLUMN_COUNT
  const innerWidth = DASHBOARD_CHART_WIDTH - DASHBOARD_CHART_PADDING_X * 2
  const gap = DASHBOARD_CHART_GAP
  const columnWidth = (innerWidth - gap * Math.max(slotCount - 1, 0)) / slotCount
  const filledOffset = Math.max(0, slotCount - data.length)

  if (data.length === 0) {
    return Array.from({ length: DASHBOARD_COLUMN_COUNT }, (_, index) => {
      const x = DASHBOARD_CHART_PADDING_X + index * (columnWidth + gap)
      return {
        x,
        width: columnWidth,
        markerX1: x + 4,
        markerX2: x + columnWidth - 4,
        markerY: DASHBOARD_CHART_HEIGHT / 2,
        hasValue: false,
      }
    })
  }

  const values = data.map((point) => point.value)
  const rawMin = props.maxValue > 0 ? 0 : Math.min(...values)
  const rawMax = props.maxValue > 0 ? props.maxValue : Math.max(...values)
  const min = Math.min(rawMin, rawMax)
  const max = Math.max(rawMin, rawMax)
  const range = max - min

  return Array.from({ length: DASHBOARD_COLUMN_COUNT }, (_, index) => {
    const point = index >= filledOffset ? (data[index - filledOffset] ?? null) : null
    const x = DASHBOARD_CHART_PADDING_X + index * (columnWidth + gap)
    if (!point) {
      return {
        x,
        width: columnWidth,
        markerX1: x + 4,
        markerX2: x + columnWidth - 4,
        markerY: DASHBOARD_CHART_HEIGHT / 2,
        hasValue: false,
      }
    }

    const markerY =
      range === 0
        ? DASHBOARD_CHART_HEIGHT / 2
        : DASHBOARD_CHART_HEIGHT -
          DASHBOARD_CHART_PADDING_Y -
          ((point.value - min) / range) * (DASHBOARD_CHART_HEIGHT - DASHBOARD_CHART_PADDING_Y * 2)
    const markerInset = Math.max(6.5, columnWidth * 0.22)

    return {
      x,
      width: columnWidth,
      markerX1: x + markerInset,
      markerX2: x + columnWidth - markerInset,
      markerY,
      hasValue: true,
    }
  })
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

const statItems = computed(() => [
  { label: t('metrics.min', 'Min'), value: `${minValue.value}${props.unit}` },
  { label: t('resourceChart.avg', 'Avg'), value: `${avgValue.value}${props.unit}` },
  { label: t('metrics.max', 'Max'), value: `${maxValueDisplay.value}${props.unit}` },
])

const resolvedStatItems = computed(() => {
  return props.summaryItems.length > 0 ? props.summaryItems : statItems.value
})

const resolvedBadge = computed(() => props.badge || `${props.data.length || 0} pts`)

const headlineSubtitle = computed(() => {
  if (!isDashboardVariant.value) return `${currentValue.value}${props.unit}`
  return props.subtitle || props.caption || t('resourceChart.recentTrend', 'Recent trend')
})

const lastChartPoint = computed(() => {
  const points = chartData.value.points
  return points.length > 0 ? points[points.length - 1] : null
})

const summaryGridStyle = computed(() => ({
  '--resource-chart-stat-columns': String(Math.max(1, Math.min(resolvedStatItems.value.length, 4))),
}))
</script>

<template>
  <div
    class="dashboard-card-surface p-4"
    :class="{ 'resource-chart-card-dashboard': isDashboardVariant }"
    :style="chartVars"
  >
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer resource-chart-card-head">
        <div class="dashboard-card-copy min-w-0">
          <p class="dashboard-card-label">{{ title }}</p>
          <p class="dashboard-card-subtitle mt-2">{{ headlineSubtitle }}</p>
        </div>
        <span class="dashboard-card-chip resource-chart-card-chip">{{ resolvedBadge }}</span>
      </div>

      <template v-if="isDashboardVariant">
        <div class="dashboard-card-subsurface resource-chart-panel p-4">
          <div class="resource-chart-panel-head">
            <div class="resource-chart-panel-copy min-w-0">
              <p class="resource-chart-panel-value truncate">{{ currentValue }}{{ unit }}</p>
              <p class="resource-chart-panel-caption">
                {{ caption || subtitle || t('resourceChart.avg', 'Avg') }}
              </p>
            </div>
          </div>

          <div class="relative h-28 sm:h-32">
            <svg
              v-if="data.length > 0"
              viewBox="0 0 300 80"
              class="w-full h-full"
              preserveAspectRatio="none"
            >
              <rect
                v-for="(column, index) in dashboardColumns"
                :key="`column-${index}`"
                :x="column.x"
                y="0.75"
                :width="column.width"
                height="78.5"
                rx="9"
                class="resource-chart-column"
              />
              <template v-for="(column, index) in dashboardColumns" :key="`marker-${index}`">
                <line
                  v-if="column.hasValue"
                  :x1="column.markerX1"
                  :y1="column.markerY"
                  :x2="column.markerX2"
                  :y2="column.markerY"
                  class="resource-chart-column-marker"
                />
              </template>
            </svg>

            <div
              v-else
              class="absolute inset-0 flex items-center justify-center text-gray-400 dark:text-gray-500 text-sm"
            >
              {{ t('metrics.noData', 'No data available') }}
            </div>
          </div>
        </div>

        <div class="resource-chart-stats-grid text-xs" :style="summaryGridStyle">
          <div
            v-for="item in resolvedStatItems"
            :key="item.label"
            class="dashboard-card-subsurface resource-chart-stat p-3"
          >
            <div class="resource-chart-stat-label">{{ item.label }}</div>
            <div class="resource-chart-stat-value">{{ item.value }}</div>
          </div>
        </div>
      </template>

      <template v-else>
        <div class="dashboard-card-subsurface p-3">
          <div class="relative h-20">
            <svg
              v-if="data.length > 0"
              viewBox="0 0 300 80"
              class="w-full h-full"
              preserveAspectRatio="none"
            >
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
              <path v-if="chartData.areaPath" :d="chartData.areaPath" :class="colorClasses.fill" />
              <path
                v-if="chartData.path"
                :d="chartData.path"
                fill="none"
                :class="colorClasses.line"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <circle
                v-if="lastChartPoint"
                :cx="lastChartPoint.x"
                :cy="lastChartPoint.y"
                r="4"
                :class="colorClasses.line"
                fill="currentColor"
              />
            </svg>

            <div
              v-else
              class="absolute inset-0 flex items-center justify-center text-gray-400 dark:text-gray-500 text-sm"
            >
              {{ t('metrics.noData', 'No data available') }}
            </div>
          </div>
        </div>

        <div class="grid grid-cols-3 gap-2 text-xs">
          <div class="dashboard-card-subsurface p-3 text-center">
            <div class="text-gray-400 dark:text-gray-500">{{ t('metrics.min', 'Min') }}</div>
            <div class="mt-1 text-gray-700 dark:text-gray-300">{{ minValue }}{{ unit }}</div>
          </div>
          <div class="dashboard-card-subsurface p-3 text-center">
            <div class="text-gray-400 dark:text-gray-500">{{ t('resourceChart.avg', 'Avg') }}</div>
            <div class="mt-1 text-gray-700 dark:text-gray-300">{{ avgValue }}{{ unit }}</div>
          </div>
          <div class="dashboard-card-subsurface p-3 text-center">
            <div class="text-gray-400 dark:text-gray-500">{{ t('metrics.max', 'Max') }}</div>
            <div class="mt-1 text-gray-700 dark:text-gray-300">{{ maxValueDisplay }}{{ unit }}</div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.resource-chart-card-dashboard {
  border-color: rgba(191, 219, 254, 0.72);
  background:
    radial-gradient(circle at 100% 0%, var(--resource-chart-fill), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(255, 255, 255, 0.42), transparent 42%),
    linear-gradient(180deg, #fcfdff 0%, #f4f8fb 100%);
}

.resource-chart-card-dashboard .dashboard-card-label {
  background: var(--resource-chart-soft);
  color: var(--resource-chart-soft-text);
}

.resource-chart-card-dashboard .resource-chart-card-chip {
  border-color: var(--resource-chart-soft-border);
  background: rgba(255, 255, 255, 0.8);
  color: var(--resource-chart-soft-text);
}

.resource-chart-panel {
  position: relative;
  overflow: hidden;
  border-color: var(--resource-chart-soft-border);
  background:
    radial-gradient(circle at 100% 0%, rgba(255, 255, 255, 0.52), transparent 46%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.9) 0%, rgba(248, 250, 252, 0.92) 100%);
}

.resource-chart-panel::before {
  content: '';
  position: absolute;
  top: 0;
  inset-inline-start: 0;
  inset-inline-end: 40%;
  height: 1px;
  background: linear-gradient(90deg, var(--resource-chart-soft-border) 0%, transparent 100%);
}

.resource-chart-panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.8rem;
  margin-bottom: 0.8rem;
}

.resource-chart-panel-copy {
  min-width: 0;
}

.resource-chart-panel-value {
  margin: 0;
  font-size: clamp(1.9rem, 1.2vw + 1rem, 2.35rem);
  line-height: 1;
  letter-spacing: -0.03em;
  font-weight: 650;
  color: #0f172a;
  word-break: break-word;
}

.resource-chart-panel-caption {
  margin: 0.42rem 0 0;
  font-size: 0.72rem;
  line-height: 1.4;
  color: #64748b;
}

.resource-chart-gridline {
  stroke: var(--resource-chart-grid);
}

.resource-chart-column {
  fill: #f3f5f8;
  stroke: rgba(226, 232, 240, 0.96);
  stroke-width: 0.9;
}

.resource-chart-column-marker {
  stroke: #1f5eff;
  stroke-width: 6.4;
  stroke-linecap: round;
  filter: drop-shadow(0 1px 0 rgba(59, 130, 246, 0.22));
}

.resource-chart-area {
  fill: var(--resource-chart-fill);
}

.resource-chart-line {
  stroke: var(--resource-chart-stroke);
}

.resource-chart-point {
  fill: var(--resource-chart-stroke);
}

.resource-chart-stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.resource-chart-stat {
  position: relative;
  overflow: hidden;
  border-color: var(--resource-chart-soft-border);
  background:
    radial-gradient(circle at 100% 0%, rgba(255, 255, 255, 0.42), transparent 44%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(247, 250, 252, 0.98) 100%);
  min-height: 4.9rem;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  text-align: start;
}

.resource-chart-stat::before {
  content: '';
  position: absolute;
  inset: 0 auto auto 0;
  width: 100%;
  height: 1px;
  background: linear-gradient(90deg, var(--resource-chart-soft-border) 0%, transparent 100%);
}

.resource-chart-stat-label {
  font-size: 0.64rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: #64748b;
}

.resource-chart-stat-value {
  margin-top: 0.5rem;
  font-size: 0.98rem;
  line-height: 1.2;
  font-weight: 650;
  color: #0f172a;
  word-break: break-word;
}

:root.dark .resource-chart-card-dashboard,
[data-theme='dark'] .resource-chart-card-dashboard,
html.dark .resource-chart-card-dashboard {
  border-color: rgba(100, 116, 139, 0.58);
  background:
    radial-gradient(circle at 100% 0%, var(--resource-chart-fill), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(51, 65, 85, 0.2), transparent 42%),
    linear-gradient(180deg, #27374b 0%, #223144 100%);
}

:root.dark .resource-chart-card-dashboard .dashboard-card-label,
[data-theme='dark'] .resource-chart-card-dashboard .dashboard-card-label,
html.dark .resource-chart-card-dashboard .dashboard-card-label {
  background: rgba(15, 23, 42, 0.28);
}

:root.dark .resource-chart-card-dashboard .resource-chart-card-chip,
[data-theme='dark'] .resource-chart-card-dashboard .resource-chart-card-chip,
html.dark .resource-chart-card-dashboard .resource-chart-card-chip {
  background: rgba(15, 23, 42, 0.24);
}

:root.dark .resource-chart-panel,
[data-theme='dark'] .resource-chart-panel,
html.dark .resource-chart-panel {
  background:
    radial-gradient(circle at 100% 0%, rgba(15, 23, 42, 0.26), transparent 46%),
    linear-gradient(180deg, rgba(39, 54, 74, 0.94) 0%, rgba(29, 41, 59, 0.96) 100%);
  border-color: rgba(100, 116, 139, 0.52);
}

:root.dark .resource-chart-column,
[data-theme='dark'] .resource-chart-column,
html.dark .resource-chart-column {
  fill: rgba(148, 163, 184, 0.2);
  stroke: rgba(148, 163, 184, 0.08);
}

:root.dark .resource-chart-panel-value,
[data-theme='dark'] .resource-chart-panel-value,
html.dark .resource-chart-panel-value {
  color: rgb(241 245 249);
}

:root.dark .resource-chart-panel-caption,
[data-theme='dark'] .resource-chart-panel-caption,
html.dark .resource-chart-panel-caption,
:root.dark .resource-chart-stat-label,
[data-theme='dark'] .resource-chart-stat-label,
html.dark .resource-chart-stat-label {
  color: rgb(148 163 184);
}

:root.dark .resource-chart-stat-value,
[data-theme='dark'] .resource-chart-stat-value,
html.dark .resource-chart-stat-value {
  color: rgb(241 245 249);
}

:root.dark .resource-chart-stat,
[data-theme='dark'] .resource-chart-stat,
html.dark .resource-chart-stat {
  background:
    radial-gradient(circle at 100% 0%, rgba(15, 23, 42, 0.24), transparent 44%),
    linear-gradient(180deg, rgba(39, 54, 74, 0.94) 0%, rgba(31, 43, 61, 0.98) 100%);
}

@media (max-width: 640px) {
  .resource-chart-panel-head {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media (min-width: 640px) {
  .resource-chart-stats-grid {
    grid-template-columns: repeat(var(--resource-chart-stat-columns, 3), minmax(0, 1fr));
  }
}
</style>
