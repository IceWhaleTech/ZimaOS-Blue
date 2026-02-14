<script setup lang="ts">
import type { TypelessCardAction } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardAction
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

const buttonClasses = {
  primary: 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white',
  secondary: 'bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300',
  danger: 'bg-red-500 hover:bg-red-600 text-white',
}

function handleClick(actionId: string) {
  emit('action', actionId, props.card.id)
}
</script>

<template>
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-700 p-4">
    <h4 class="font-medium text-gray-900 dark:text-white mb-1">
      {{ card.title }}
    </h4>
    <p
      v-if="card.description"
      class="text-sm text-gray-500 dark:text-gray-400 mb-4"
    >
      {{ card.description }}
    </p>

    <div class="flex flex-wrap gap-2">
      <button
        v-for="action in card.actions"
        :key="action.id"
        class="px-4 py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        :class="buttonClasses[action.variant || 'secondary']"
        :disabled="action.disabled"
        @click="handleClick(action.id)"
      >
        <span v-if="action.icon" class="mr-1.5">{{ action.icon }}</span>
        {{ action.label }}
      </button>
    </div>
  </div>
</template>
