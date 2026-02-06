<script setup lang="ts">
import type { TypelessCardInfo } from '@/types/typeless'

defineProps<{
  card: TypelessCardInfo
}>()

const variantClasses = {
  default: 'bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/20 border-gray-900 dark:border-white dark:border-gray-900 dark:border-white',
  warning: 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800',
  success: 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800',
  error: 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800',
}

const iconClasses = {
  default: 'text-gray-900 dark:text-white',
  warning: 'text-amber-500',
  success: 'text-green-500',
  error: 'text-red-500',
}
</script>

<template>
  <div
    class="rounded-lg border p-4"
    :class="variantClasses[card.variant || 'default']"
  >
    <div class="flex items-start gap-3">
      <span
        v-if="card.icon"
        class="text-xl flex-shrink-0"
        :class="iconClasses[card.variant || 'default']"
      >
        {{ card.icon }}
      </span>
      <div class="flex-1 min-w-0">
        <h4
          v-if="card.title"
          class="font-medium text-gray-900 dark:text-white mb-1"
        >
          {{ card.title }}
        </h4>
        <p class="text-sm text-gray-600 dark:text-gray-300 whitespace-pre-wrap">
          {{ card.content }}
        </p>
      </div>
    </div>
  </div>
</template>
