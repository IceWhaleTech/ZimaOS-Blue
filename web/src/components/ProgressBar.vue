<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    value: number
    max?: number
    label?: string
    showPercent?: boolean
    color?: 'blue' | 'green' | 'red' | 'orange' | 'purple' | 'auto'
    size?: 'sm' | 'md' | 'lg'
    formatValue?: (value: number) => string
  }>(),
  {
    max: 100,
    label: '',
    showPercent: true,
    color: 'blue',
    size: 'md',
    formatValue: (v: number) => v.toFixed(1),
  }
)

const percent = computed(() => {
  const val = props.value ?? 0
  const maxVal = props.max ?? 0
  if (maxVal === 0) return 0
  return Math.min(100, (val / maxVal) * 100)
})

const colorClass = computed(() => {
  if (props.color === 'auto') {
    // Auto color based on percentage
    if (percent.value >= 90) return 'bg-red-500'
    if (percent.value >= 70) return 'bg-orange-500'
    if (percent.value >= 50) return 'bg-yellow-500'
    return 'bg-green-500'
  }
  const colors: Record<string, string> = {
    blue: 'bg-gray-700 dark:bg-gray-700',
    green: 'bg-green-500',
    red: 'bg-red-500',
    orange: 'bg-orange-500',
    purple: 'bg-purple-500',
  }
  return colors[props.color] ?? 'bg-gray-700 dark:bg-gray-700'
})

const sizeClass = computed(() => {
  const sizes: Record<string, string> = {
    sm: 'h-1.5',
    md: 'h-2.5',
    lg: 'h-4',
  }
  return sizes[props.size] ?? 'h-2.5'
})
</script>

<template>
  <div class="w-full">
    <div v-if="label || showPercent" class="flex justify-between text-sm mb-1">
      <span v-if="label" class="text-gray-500 dark:text-gray-400">{{ label }}</span>
      <span v-if="showPercent" class="text-gray-700 dark:text-gray-300">{{ formatValue(percent) }}%</span>
    </div>
    <div class="w-full bg-gray-700 dark:bg-gray-700 rounded-full overflow-hidden" :class="sizeClass">
      <div
        class="rounded-full transition-all duration-300"
        :class="[colorClass, sizeClass]"
        :style="{ width: `${percent}%` }"
      ></div>
    </div>
  </div>
</template>
