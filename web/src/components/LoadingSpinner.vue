<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    size?: 'sm' | 'md' | 'lg' | 'xl'
    color?: 'primary' | 'secondary' | 'white'
    fullscreen?: boolean
    text?: string
  }>(),
  {
    size: 'md',
    color: 'primary',
    fullscreen: false,
  }
)

const sizeClasses = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'h-4 w-4'
    case 'md':
      return 'h-8 w-8'
    case 'lg':
      return 'h-12 w-12'
    case 'xl':
      return 'h-16 w-16'
    default:
      return 'h-8 w-8'
  }
})

const colorClasses = computed(() => {
  switch (props.color) {
    case 'primary':
      return 'text-gray-900 dark:text-white dark:text-gray-900 dark:text-white'
    case 'secondary':
      return 'text-gray-600 dark:text-gray-400'
    case 'white':
      return 'text-white'
    default:
      return 'text-gray-900 dark:text-white dark:text-gray-900 dark:text-white'
  }
})
</script>

<template>
  <div
    v-if="fullscreen"
    class="fixed inset-0 bg-white/80 dark:bg-gray-700/80 backdrop-blur-sm flex items-center justify-center z-50"
  >
    <div class="flex flex-col items-center gap-4">
      <svg
        class="animate-spin"
        :class="[sizeClasses, colorClasses]"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        ></circle>
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        ></path>
      </svg>
      <p v-if="text" class="text-gray-600 dark:text-gray-400 text-sm font-medium">{{ text }}</p>
    </div>
  </div>

  <div v-else class="inline-flex items-center gap-2">
    <svg
      class="animate-spin"
      :class="[sizeClasses, colorClasses]"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
    >
      <circle
        class="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        stroke-width="4"
      ></circle>
      <path
        class="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      ></path>
    </svg>
    <span v-if="text" class="text-sm" :class="colorClasses">{{ text }}</span>
  </div>
</template>
