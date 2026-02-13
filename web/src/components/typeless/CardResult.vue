<script setup lang="ts">
import { ref } from 'vue'
import type { TypelessCardResult } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardResult
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

const copiedIndex = ref<number | null>(null)

const statusConfig = {
  success: {
    bg: 'bg-green-50 dark:bg-green-900/20',
    border: 'border-green-200 dark:border-green-800',
    icon: '✓',
    iconColor: 'text-green-500',
  },
  error: {
    bg: 'bg-red-50 dark:bg-red-900/20',
    border: 'border-red-200 dark:border-red-800',
    icon: '✗',
    iconColor: 'text-red-500',
  },
  warning: {
    bg: 'bg-amber-50 dark:bg-amber-900/20',
    border: 'border-amber-200 dark:border-amber-800',
    icon: '⚠',
    iconColor: 'text-amber-500',
  },
  info: {
    bg: 'bg-gray-700 dark:bg-gray-500/20',
    border: 'border-gray-900 dark:border-white dark:border-gray-900 dark:border-white',
    icon: 'ℹ',
    iconColor: 'text-gray-900 dark:text-white',
  },
}

const buttonClasses = {
  primary: 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white',
  secondary: 'bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300',
  danger: 'bg-red-500 hover:bg-red-600 text-white',
}

async function copyValue(value: string, index: number) {
  try {
    await navigator.clipboard.writeText(value)
    copiedIndex.value = index
    setTimeout(() => {
      copiedIndex.value = null
    }, 2000)
  } catch {
    console.error('Failed to copy to clipboard')
  }
}

function handleAction(actionId: string) {
  emit('action', actionId, props.card.id)
}
</script>

<template>
  <div
    class="rounded-lg border p-4"
    :class="[statusConfig[card.status].bg, statusConfig[card.status].border]"
  >
    <div class="flex items-start gap-3">
      <span
        class="text-xl flex-shrink-0"
        :class="statusConfig[card.status].iconColor"
      >
        {{ statusConfig[card.status].icon }}
      </span>
      <div class="flex-1 min-w-0">
        <h4 class="font-medium text-gray-900 dark:text-white">
          {{ card.title }}
        </h4>
        <p
          v-if="card.message"
          class="text-sm text-gray-600 dark:text-gray-300 mt-1"
        >
          {{ card.message }}
        </p>

        <!-- Details -->
        <div v-if="card.details && card.details.length > 0" class="mt-3 space-y-2">
          <div
            v-for="(detail, index) in card.details"
            :key="index"
            class="flex items-center justify-between text-sm"
          >
            <span class="text-gray-500 dark:text-gray-400">{{ detail.label }}:</span>
            <div class="flex items-center gap-2">
              <span class="text-gray-700 dark:text-gray-300 font-mono">{{ detail.value }}</span>
              <button
                v-if="detail.copyable"
                class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
                :title="copiedIndex === index ? 'Copied!' : 'Copy'"
                @click="copyValue(detail.value, index)"
              >
                <svg
                  v-if="copiedIndex !== index"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                  />
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 text-green-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </button>
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div v-if="card.actions && card.actions.length > 0" class="mt-4 flex flex-wrap gap-2">
          <button
            v-for="action in card.actions"
            :key="action.id"
            class="px-3 py-1.5 rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            :class="buttonClasses[action.variant || 'secondary']"
            :disabled="action.disabled"
            @click="handleAction(action.id)"
          >
            <span v-if="action.icon" class="mr-1.5">{{ action.icon }}</span>
            {{ action.label }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
