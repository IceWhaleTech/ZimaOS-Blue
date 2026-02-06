<script setup lang="ts">
import type { TypelessCardQuote } from '@/types/typeless'

defineProps<{
  card: TypelessCardQuote
}>()

const variantClasses = {
  default: 'border-l-gray-900 dark:border-l-gray-400 bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/20',
  highlight: 'border-l-amber-500 bg-amber-50 dark:bg-amber-900/20',
  warning: 'border-l-red-500 bg-red-50 dark:bg-red-900/20',
}
</script>

<template>
  <div
    class="quote-card rounded-lg border-l-4 p-4"
    :class="variantClasses[card.variant || 'default']"
  >
    <!-- Quote icon -->
    <svg
      xmlns="http://www.w3.org/2000/svg"
      class="h-8 w-8 text-gray-300 dark:text-gray-600 mb-2"
      fill="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        d="M14.017 21v-7.391c0-5.704 3.731-9.57 8.983-10.609l.995 2.151c-2.432.917-3.995 3.638-3.995 5.849h4v10h-9.983zm-14.017 0v-7.391c0-5.704 3.748-9.57 9-10.609l.996 2.151c-2.433.917-3.996 3.638-3.996 5.849h3.983v10h-9.983z"
      />
    </svg>

    <!-- Quote content -->
    <blockquote class="text-lg text-gray-700 dark:text-gray-300 italic leading-relaxed">
      {{ card.content }}
    </blockquote>

    <!-- Author and source -->
    <div v-if="card.author || card.source" class="mt-4 flex items-center gap-2">
      <div class="w-8 h-0.5 bg-gray-300 dark:bg-gray-600" />
      <div class="text-sm">
        <span v-if="card.author" class="font-medium text-gray-900 dark:text-white">{{ card.author }}</span>
        <span v-if="card.author && card.source" class="text-gray-400">, </span>
        <span v-if="card.source" class="text-gray-500 dark:text-gray-400">{{ card.source }}</span>
      </div>
    </div>
  </div>
</template>
