<script setup lang="ts">
import { computed } from 'vue'
import { useMetricsStore } from '@/stores/metrics'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const metricsStore = useMetricsStore()

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

  const data = [
    { label: t('metrics.inputTokens'), value: tokenData.value.input, color: '#3B82F6' },
    { label: t('metrics.outputTokens'), value: tokenData.value.output, color: '#10B981' },
    { label: t('metrics.cacheRead'), value: tokenData.value.cacheRead, color: '#8B5CF6' },
    { label: t('metrics.cacheWrite'), value: tokenData.value.cacheWrite, color: '#F59E0B' },
  ]

  const total = data.reduce((sum, d) => sum + d.value, 0) || 1

  for (const item of data) {
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

function describeArc(cx: number, cy: number, r: number, startAngle: number, endAngle: number): string {
  const start = polarToCartesian(cx, cy, r, endAngle)
  const end = polarToCartesian(cx, cy, r, startAngle)
  const largeArcFlag = endAngle - startAngle <= 180 ? '0' : '1'

  return [
    'M', cx, cy,
    'L', start.x, start.y,
    'A', r, r, 0, largeArcFlag, 0, end.x, end.y,
    'Z'
  ].join(' ')
}

function polarToCartesian(cx: number, cy: number, r: number, angle: number) {
  const rad = (angle - 90) * Math.PI / 180
  return {
    x: cx + r * Math.cos(rad),
    y: cy + r * Math.sin(rad)
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
    <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
      {{ t('metrics.tokenUsage') }}
    </h3>

    <div v-if="!tokenData" class="text-center py-8 text-gray-500 dark:text-gray-400">
      {{ t('metrics.noData') }}
    </div>

    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Pie Chart -->
      <div class="flex flex-col items-center">
        <svg viewBox="0 0 200 200" class="w-48 h-48">
          <path
            v-for="(segment, index) in pieSegments"
            :key="index"
            :d="describeArc(100, 100, 80, segment.startAngle, segment.endAngle)"
            :fill="segment.color"
            class="transition-all duration-300 hover:opacity-80"
          />
          <!-- Center hole for donut effect -->
          <circle cx="100" cy="100" r="40" class="fill-white dark:fill-gray-800" />
          <!-- Center text -->
          <text x="100" y="95" text-anchor="middle" class="fill-gray-900 dark:fill-white text-sm font-bold">
            {{ formatNumber(tokenData.total) }}
          </text>
          <text x="100" y="112" text-anchor="middle" class="fill-gray-500 dark:fill-gray-400 text-xs">
            {{ t('metrics.total') }}
          </text>
        </svg>

        <!-- Legend -->
        <div class="grid grid-cols-2 gap-2 mt-4 text-sm">
          <div v-for="segment in pieSegments" :key="segment.label" class="flex items-center gap-2">
            <div class="w-3 h-3 rounded-full" :style="{ backgroundColor: segment.color }"></div>
            <span class="text-gray-600 dark:text-gray-400">{{ segment.label }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ segment.percent.toFixed(1) }}%</span>
          </div>
        </div>
      </div>

      <!-- Stats & Model Breakdown -->
      <div class="space-y-4">
        <!-- Total Cost -->
        <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
          <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('metrics.estimatedCost') }}</div>
          <div class="text-2xl font-bold text-green-600 dark:text-green-400">
            {{ formatCost(tokenData.cost) }}
          </div>
        </div>

        <!-- Token Breakdown -->
        <div class="space-y-2">
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('metrics.inputTokens') }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ formatNumber(tokenData.input) }}</span>
          </div>
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('metrics.outputTokens') }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ formatNumber(tokenData.output) }}</span>
          </div>
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('metrics.cacheRead') }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ formatNumber(tokenData.cacheRead) }}</span>
          </div>
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('metrics.cacheWrite') }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ formatNumber(tokenData.cacheWrite) }}</span>
          </div>
        </div>

        <!-- Model Usage -->
        <div v-if="modelUsage.length > 0" class="mt-4">
          <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('metrics.byModel') }}</h4>
          <div class="space-y-2 max-h-40 overflow-y-auto">
            <div
              v-for="model in modelUsage"
              :key="model.model"
              class="flex justify-between items-center text-sm bg-gray-50 dark:bg-gray-700/30 rounded px-2 py-1"
            >
              <span class="text-gray-600 dark:text-gray-400">{{ model.model }}</span>
              <div class="flex items-center gap-2">
                <span class="text-gray-900 dark:text-white">{{ formatNumber(model.total_tokens) }}</span>
                <span class="text-green-600 dark:text-green-400 text-xs">{{ formatCost(model.estimated_cost) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
