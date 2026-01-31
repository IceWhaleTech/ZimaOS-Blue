<script setup lang="ts">
import type { PresetQuestion } from '@/api/preview'

defineProps<{
  question: PresetQuestion
}>()

const emit = defineEmits<{
  click: [question: PresetQuestion]
}>()
</script>

<template>
  <button
    class="group flex items-center gap-3 px-4 py-3 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:border-accent dark:hover:border-accent hover:shadow-md transition-all duration-200 text-left w-full"
    @click="emit('click', question)"
  >
    <span v-if="question.icon" class="text-xl flex-shrink-0">{{ question.icon }}</span>
    <span class="text-sm text-gray-700 dark:text-gray-300 group-hover:text-accent transition-colors">
      {{ question.text }}
    </span>
  </button>
</template>
