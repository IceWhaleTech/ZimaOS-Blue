<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    value: number
    max: number
    label?: string
    valueLabel?: string
    color?: 'blue' | 'green' | 'red' | 'orange' | 'purple' | 'auto'
    size?: number
    strokeWidth?: number
  }>(),
  {
    label: '',
    valueLabel: '',
    color: 'blue',
    size: 120,
    strokeWidth: 12,
  }
)

const percent = computed(() => {
  const val = props.value ?? 0
  const maxVal = props.max ?? 0
  if (maxVal === 0) return 0
  return Math.min(100, (val / maxVal) * 100)
})

const radius = computed(() => (props.size - props.strokeWidth) / 2)
const circumference = computed(() => 2 * Math.PI * radius.value)
const strokeDashoffset = computed(() => circumference.value * (1 - percent.value / 100))

const colorClass = computed(() => {
  if (props.color === 'auto') {
    if (percent.value >= 90) return 'stroke-red-500'
    if (percent.value >= 70) return 'stroke-orange-500'
    if (percent.value >= 50) return 'stroke-yellow-500'
    return 'stroke-green-500'
  }
  const colors: Record<string, string> = {
    blue: 'stroke-gray-900 dark:stroke-gray-400',
    green: 'stroke-green-500',
    red: 'stroke-red-500',
    orange: 'stroke-orange-500',
    purple: 'stroke-purple-500',
  }
  return colors[props.color] ?? 'stroke-gray-900 dark:stroke-gray-400'
})
</script>

<template>
  <div class="flex flex-col items-center">
    <div class="relative" :style="{ width: `${size}px`, height: `${size}px` }">
      <svg :width="size" :height="size" class="transform -rotate-90">
        <!-- Background circle -->
        <circle
          :cx="size / 2"
          :cy="size / 2"
          :r="radius"
          fill="none"
          class="stroke-gray-200 dark:stroke-gray-700"
          :stroke-width="strokeWidth"
        />
        <!-- Progress circle -->
        <circle
          :cx="size / 2"
          :cy="size / 2"
          :r="radius"
          fill="none"
          :class="colorClass"
          :stroke-width="strokeWidth"
          :stroke-dasharray="circumference"
          :stroke-dashoffset="strokeDashoffset"
          stroke-linecap="round"
          class="transition-all duration-500"
        />
      </svg>
      <!-- Center text -->
      <div class="absolute inset-0 flex flex-col items-center justify-center">
        <span class="text-xl font-bold text-gray-900 dark:text-white">{{ (percent ?? 0).toFixed(0) }}%</span>
        <span v-if="valueLabel" class="text-xs text-gray-500 dark:text-gray-400">{{ valueLabel }}</span>
      </div>
    </div>
    <span v-if="label" class="mt-2 text-sm text-gray-600 dark:text-gray-400">{{ label }}</span>
  </div>
</template>
