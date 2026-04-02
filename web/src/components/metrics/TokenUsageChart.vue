<script setup lang="ts">
import { computed } from 'vue'
import { useMetricsStore } from '@/stores/metrics'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const metricsStore = useMetricsStore()
const authStore = useAuthStore()

const tokenData = computed(() => {
  const usage = metricsStore.tokenUsage?.usage
  if (!usage) return null

  const total = usage.total_tokens || 1
  const input = usage.input_tokens ?? 0
  const output = usage.output_tokens ?? 0
  const cacheRead = usage.cache_read_tokens ?? 0
  const cacheWrite = usage.cache_write_tokens ?? 0

  return {
    input,
    output,
    cacheRead,
    cacheWrite,
    total: usage.total_tokens ?? 0,
    cost: usage.estimated_cost ?? 0,
    inputPercent: (input / total) * 100,
    outputPercent: (output / total) * 100,
    cacheReadPercent: (cacheRead / total) * 100,
    cacheWritePercent: (cacheWrite / total) * 100,
  }
})

const modelUsage = computed(() => {
  return metricsStore.tokenUsage?.by_model ?? []
})

const breakdownItems = computed(() => {
  if (!tokenData.value) return []

  return [
    {
      key: 'input',
      label: t('metrics.inputTokens'),
      value: tokenData.value.input,
      percent: tokenData.value.inputPercent,
      color: '#3B82F6',
    },
    {
      key: 'output',
      label: t('metrics.outputTokens'),
      value: tokenData.value.output,
      percent: tokenData.value.outputPercent,
      color: '#10B981',
    },
    {
      key: 'cache-read',
      label: t('metrics.cacheRead'),
      value: tokenData.value.cacheRead,
      percent: tokenData.value.cacheReadPercent,
      color: '#8B5CF6',
    },
    {
      key: 'cache-write',
      label: t('metrics.cacheWrite'),
      value: tokenData.value.cacheWrite,
      percent: tokenData.value.cacheWritePercent,
      color: '#F59E0B',
    },
  ]
})

function formatNumber(num: number | undefined | null): string {
  if (num == null) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toFixed(0)
}

function formatCost(cost: number | undefined | null): string {
  if (cost == null) return '$0.0000'
  return '$' + cost.toFixed(4)
}

// Calculate pie chart segments
const pieSegments = computed(() => {
  if (!tokenData.value) return []

  const segments = []
  let currentAngle = 0

  const total = breakdownItems.value.reduce((sum, item) => sum + item.value, 0) || 1

  for (const item of breakdownItems.value) {
    if (item.value === 0) continue

    const percent = item.value / total
    const angle = percent * 360

    segments.push({
      ...item,
      percent: percent * 100,
      startAngle: currentAngle,
      endAngle: currentAngle + angle,
    })

    currentAngle += angle
  }

  return segments
})

function describeArc(
  cx: number,
  cy: number,
  r: number,
  startAngle: number,
  endAngle: number
): string {
  const start = polarToCartesian(cx, cy, r, endAngle)
  const end = polarToCartesian(cx, cy, r, startAngle)
  const largeArcFlag = endAngle - startAngle <= 180 ? '0' : '1'

  return [
    'M',
    cx,
    cy,
    'L',
    start.x,
    start.y,
    'A',
    r,
    r,
    0,
    largeArcFlag,
    0,
    end.x,
    end.y,
    'Z',
  ].join(' ')
}

function polarToCartesian(cx: number, cy: number, r: number, angle: number) {
  const rad = ((angle - 90) * Math.PI) / 180
  return {
    x: cx + r * Math.cos(rad),
    y: cy + r * Math.sin(rad),
  }
}
</script>

<template>
  <div class="dashboard-card-surface token-usage-card p-4">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy min-w-0">
          <p class="dashboard-card-label">{{ t('metrics.label') }}</p>
          <p class="dashboard-card-subtitle mt-2">{{ t('metrics.tokenUsage') }}</p>
        </div>
        <div class="flex flex-wrap items-center justify-end gap-2">
          <span v-if="tokenData" class="dashboard-card-chip">
            {{ formatNumber(tokenData.total) }} {{ t('metrics.tokens') }}
          </span>
          <router-link
            v-if="authStore.isAdmin"
            to="/billing"
            class="dashboard-card-chip token-usage-card-link no-underline"
          >
            {{ t('nav.billing') }}
          </router-link>
        </div>
      </div>

      <div v-if="!tokenData" class="dashboard-card-empty">
        {{ t('metrics.noData') }}
      </div>

      <div v-else class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div class="dashboard-card-subsurface token-usage-hero p-4">
          <div class="token-usage-hero-copy">
            <p class="dashboard-card-value">{{ formatNumber(tokenData.total) }}</p>
            <p class="dashboard-card-footnote">
              {{ formatCost(tokenData.cost) }} {{ t('metrics.estimatedCost') }}
            </p>
          </div>

          <div class="token-usage-hero-chart">
            <svg viewBox="0 0 200 200" class="h-48 w-48">
              <path
                v-for="(segment, index) in pieSegments"
                :key="index"
                :d="describeArc(100, 100, 80, segment.startAngle, segment.endAngle)"
                :fill="segment.color"
                class="transition-all duration-300 hover:opacity-80"
              />
              <circle cx="100" cy="100" r="40" class="token-usage-ring-core" />
              <text
                x="100"
                y="95"
                text-anchor="middle"
                class="fill-gray-900 dark:fill-white text-sm font-bold"
              >
                {{ formatNumber(tokenData.total) }}
              </text>
              <text
                x="100"
                y="112"
                text-anchor="middle"
                class="fill-gray-500 dark:fill-gray-400 text-xs"
              >
                {{ t('metrics.total') }}
              </text>
            </svg>
          </div>

          <div class="token-usage-legend">
            <div
              v-for="segment in breakdownItems"
              :key="segment.key"
              class="token-usage-legend-item"
            >
              <div class="flex items-center gap-2 min-w-0">
                <span
                  class="token-usage-legend-dot"
                  :style="{ backgroundColor: segment.color }"
                ></span>
                <span class="truncate text-sm text-gray-700 dark:text-gray-300">{{
                  segment.label
                }}</span>
              </div>
              <div class="text-end">
                <div class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ formatNumber(segment.value) }}
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ segment.percent.toFixed(0) }}%
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="space-y-3">
          <div class="dashboard-card-subsurface token-usage-stat p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('metrics.estimatedCost') }}
            </p>
            <p class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">
              {{ formatCost(tokenData.cost) }}
            </p>
          </div>

          <div class="dashboard-card-subsurface token-usage-breakdown p-4 space-y-2">
            <p class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('metrics.tokenUsage') }}
            </p>
            <div
              v-for="segment in breakdownItems"
              :key="segment.key"
              class="token-usage-breakdown-row"
            >
              <div class="flex items-center gap-2 min-w-0">
                <span
                  class="token-usage-legend-dot"
                  :style="{ backgroundColor: segment.color }"
                ></span>
                <span class="truncate text-sm text-gray-600 dark:text-gray-400">{{
                  segment.label
                }}</span>
              </div>
              <span class="text-sm font-medium text-gray-900 dark:text-white">
                {{ formatNumber(segment.value) }}
              </span>
            </div>
            <div class="token-usage-breakdown-row">
              <span class="text-sm text-gray-600 dark:text-gray-400">
                {{ t('metrics.cacheWrite') }}
              </span>
              <span class="text-sm font-medium text-gray-900 dark:text-white">
                {{ formatNumber(tokenData.cacheWrite) }}
              </span>
            </div>
          </div>

          <div class="dashboard-card-subsurface p-4">
            <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('metrics.byModel') }}
            </h4>
            <div v-if="modelUsage.length > 0" class="space-y-2 max-h-40 overflow-y-auto">
              <div
                v-for="model in modelUsage"
                :key="model.model"
                class="flex justify-between items-center text-sm"
              >
                <span class="text-gray-600 dark:text-gray-400 truncate pe-3">{{
                  model.model
                }}</span>
                <div class="flex items-center gap-2">
                  <span class="text-gray-900 dark:text-white">{{
                    formatNumber(model.total_tokens)
                  }}</span>
                  <span class="text-green-600 dark:text-green-400 text-xs">{{
                    formatCost(model.estimated_cost)
                  }}</span>
                </div>
              </div>
            </div>
            <div v-else class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('metrics.noData') }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.token-usage-card {
  border-color: rgba(167, 243, 208, 0.76);
  background:
    radial-gradient(circle at 100% 0%, rgba(167, 243, 208, 0.26), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(219, 234, 254, 0.22), transparent 42%),
    linear-gradient(180deg, #fcfffd 0%, #f2f8f4 100%);
}

.token-usage-card .dashboard-card-label {
  background: rgba(220, 252, 231, 0.86);
  color: #166534;
}

.token-usage-card-link {
  color: #166534;
}

.token-usage-hero {
  display: grid;
  gap: 1rem;
  align-content: start;
  border-color: rgba(167, 243, 208, 0.72);
  background:
    radial-gradient(circle at 100% 0%, rgba(220, 252, 231, 0.7), transparent 42%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(244, 251, 246, 0.98) 100%);
}

.token-usage-hero-copy {
  min-width: 0;
}

.token-usage-hero-chart {
  display: flex;
  justify-content: center;
}

.token-usage-ring-core {
  fill: rgba(255, 255, 255, 0.98);
}

.token-usage-legend {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.token-usage-legend-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.72rem 0.82rem;
  border-radius: 0.88rem;
  border: 1px solid rgba(187, 247, 208, 0.78);
  background: rgba(255, 255, 255, 0.72);
}

.token-usage-legend-dot {
  width: 0.65rem;
  height: 0.65rem;
  flex: 0 0 auto;
  border-radius: 999px;
  box-shadow: 0 0 0 3px rgba(255, 255, 255, 0.72);
}

.token-usage-stat {
  min-height: 6rem;
}

.token-usage-breakdown {
  border-color: rgba(187, 247, 208, 0.72);
}

.token-usage-breakdown-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

:root.dark .token-usage-card,
[data-theme='dark'] .token-usage-card,
html.dark .token-usage-card {
  border-color: rgba(52, 211, 153, 0.24);
  background:
    radial-gradient(circle at 100% 0%, rgba(16, 185, 129, 0.1), transparent 48%),
    linear-gradient(180deg, rgba(30, 41, 59, 0.94) 0%, rgba(15, 23, 42, 0.96) 100%);
}

:root.dark .token-usage-card .dashboard-card-label,
[data-theme='dark'] .token-usage-card .dashboard-card-label,
html.dark .token-usage-card .dashboard-card-label {
  background: rgba(20, 83, 45, 0.52);
  color: #bbf7d0;
}

:root.dark .token-usage-card-link,
[data-theme='dark'] .token-usage-card-link,
html.dark .token-usage-card-link {
  color: rgb(226 232 240);
}

:root.dark .token-usage-hero,
[data-theme='dark'] .token-usage-hero,
html.dark .token-usage-hero {
  border-color: rgba(52, 211, 153, 0.18);
  background:
    radial-gradient(circle at 100% 0%, rgba(20, 83, 45, 0.22), transparent 42%),
    linear-gradient(180deg, rgba(30, 41, 59, 0.8) 0%, rgba(15, 23, 42, 0.88) 100%);
}

:root.dark .token-usage-legend-item,
[data-theme='dark'] .token-usage-legend-item,
html.dark .token-usage-legend-item {
  border-color: rgba(71, 85, 105, 0.36);
  background: rgba(15, 23, 42, 0.56);
}

:root.dark .token-usage-legend-dot,
[data-theme='dark'] .token-usage-legend-dot,
html.dark .token-usage-legend-dot {
  box-shadow: 0 0 0 3px rgba(15, 23, 42, 0.78);
}

:root.dark .token-usage-ring-core,
[data-theme='dark'] .token-usage-ring-core,
html.dark .token-usage-ring-core {
  fill: rgba(15, 23, 42, 0.96);
}

@media (max-width: 640px) {
  .token-usage-legend {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
