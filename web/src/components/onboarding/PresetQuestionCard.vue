<script setup lang="ts">
import { computed } from 'vue'
import type { PresetQuestion } from '@/api/preview'

const props = defineProps<{
  question: PresetQuestion
}>()

const emit = defineEmits<{
  click: [question: PresetQuestion]
}>()

const hasAttachments = computed(() => props.question.attachments && props.question.attachments.length > 0)
const hasImageAttachment = computed(() => props.question.attachments?.some(a => a.type === 'image'))
</script>

<template>
  <button
    class="group flex items-center gap-3 px-4 py-3 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:border-accent dark:hover:border-accent hover:shadow-md transition-all duration-200 text-left w-full"
    @click="emit('click', question)"
  >
    <span v-if="question.icon" class="text-xl flex-shrink-0">{{ question.icon }}</span>
    <span class="flex-1 text-sm text-gray-700 dark:text-gray-300 group-hover:text-accent transition-colors">
      {{ question.text }}
    </span>
    <!-- Attachment indicator -->
    <span v-if="hasAttachments" class="flex-shrink-0 flex items-center gap-1">
      <svg
        v-if="hasImageAttachment"
        class="w-4 h-4 text-gray-400 dark:text-gray-500"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
      </svg>
      <svg
        v-else
        class="w-4 h-4 text-gray-400 dark:text-gray-500"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
      </svg>
    </span>
  </button>
</template>
