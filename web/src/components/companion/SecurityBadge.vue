<script setup lang="ts">
import { computed } from 'vue'
import type { ThreatLevel } from '@/api/companion'

const props = defineProps<{
  level: ThreatLevel
  score?: number
  showScore?: boolean
  size?: 'sm' | 'md' | 'lg'
}>()

const sizeClasses = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'px-1.5 py-0.5 text-xs'
    case 'lg':
      return 'px-3 py-1.5 text-sm'
    default:
      return 'px-2 py-1 text-xs'
  }
})

const colorClasses = computed(() => {
  switch (props.level) {
    case 'critical':
      return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 border-red-200 dark:border-red-800'
    case 'high':
      return 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300 border-orange-200 dark:border-orange-800'
    case 'medium':
      return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300 border-yellow-200 dark:border-yellow-800'
    case 'low':
      return 'bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/50 text-gray-900 dark:text-white dark:text-gray-900 dark:text-white border-gray-900 dark:border-white dark:border-gray-900 dark:border-white'
    default:
      return 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 border-green-200 dark:border-green-800'
  }
})

const iconPath = computed(() => {
  if (props.level === 'none') {
    return 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z'
  }
  return 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z'
})

const iconSize = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'w-3 h-3'
    case 'lg':
      return 'w-5 h-5'
    default:
      return 'w-4 h-4'
  }
})
</script>

<template>
  <span
    :class="[
      'inline-flex items-center gap-1 rounded-full border font-medium capitalize',
      sizeClasses,
      colorClasses
    ]"
  >
    <svg :class="iconSize" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="iconPath" />
    </svg>
    <span>{{ level }}</span>
    <span v-if="showScore && score !== undefined" class="opacity-75">({{ score }})</span>
  </span>
</template>
