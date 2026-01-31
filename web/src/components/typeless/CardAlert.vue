<script setup lang="ts">
import { ref } from 'vue'
import type { TypelessCardAlert } from '@/types/typeless'

defineProps<{
  card: TypelessCardAlert
}>()

const emit = defineEmits<{
  dismiss: []
  action: [actionId: string]
}>()

const dismissed = ref(false)

const variantStyles = {
  info: {
    bg: 'bg-blue-50 dark:bg-blue-900/20',
    border: 'border-blue-200 dark:border-blue-800',
    icon: 'text-blue-500',
    title: 'text-blue-800 dark:text-blue-200',
    text: 'text-blue-700 dark:text-blue-300',
  },
  success: {
    bg: 'bg-green-50 dark:bg-green-900/20',
    border: 'border-green-200 dark:border-green-800',
    icon: 'text-green-500',
    title: 'text-green-800 dark:text-green-200',
    text: 'text-green-700 dark:text-green-300',
  },
  warning: {
    bg: 'bg-amber-50 dark:bg-amber-900/20',
    border: 'border-amber-200 dark:border-amber-800',
    icon: 'text-amber-500',
    title: 'text-amber-800 dark:text-amber-200',
    text: 'text-amber-700 dark:text-amber-300',
  },
  error: {
    bg: 'bg-red-50 dark:bg-red-900/20',
    border: 'border-red-200 dark:border-red-800',
    icon: 'text-red-500',
    title: 'text-red-800 dark:text-red-200',
    text: 'text-red-700 dark:text-red-300',
  },
}

const defaultIcons = {
  info: 'ℹ️',
  success: '✅',
  warning: '⚠️',
  error: '❌',
}

function handleDismiss() {
  dismissed.value = true
  emit('dismiss')
}

function handleAction(actionId: string) {
  emit('action', actionId)
}
</script>

<template>
  <div
    v-if="!dismissed"
    class="alert-card rounded-lg border p-4"
    :class="[variantStyles[card.variant].bg, variantStyles[card.variant].border]"
  >
    <div class="flex gap-3">
      <!-- Icon -->
      <div class="flex-shrink-0 text-xl" :class="variantStyles[card.variant].icon">
        {{ card.icon || defaultIcons[card.variant] }}
      </div>

      <!-- Content -->
      <div class="flex-1 min-w-0">
        <h4 v-if="card.title" class="font-medium" :class="variantStyles[card.variant].title">
          {{ card.title }}
        </h4>
        <p class="text-sm" :class="[variantStyles[card.variant].text, { 'mt-1': card.title }]">
          {{ card.message }}
        </p>

        <!-- Actions -->
        <div v-if="card.actions?.length" class="mt-3 flex gap-2">
          <button
            v-for="action in card.actions"
            :key="action.id"
            class="px-3 py-1.5 text-sm font-medium rounded-lg transition-colors"
            :class="{
              'bg-blue-600 text-white hover:bg-blue-700': action.variant === 'primary',
              'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700': action.variant !== 'primary' && action.variant !== 'danger',
              'bg-red-600 text-white hover:bg-red-700': action.variant === 'danger',
            }"
            :disabled="action.disabled"
            @click="handleAction(action.id)"
          >
            {{ action.label }}
          </button>
        </div>
      </div>

      <!-- Dismiss button -->
      <button
        v-if="card.dismissible"
        class="flex-shrink-0 p-1 rounded hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
        :class="variantStyles[card.variant].icon"
        @click="handleDismiss"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  </div>
</template>
