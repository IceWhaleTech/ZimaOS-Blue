<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'

withDefaults(
  defineProps<{
    fallbackTitle?: string
    fallbackMessage?: string
    showRetry?: boolean
  }>(),
  {
    fallbackTitle: 'Something went wrong',
    fallbackMessage: 'An unexpected error occurred. Please try again.',
    showRetry: true,
  }
)

const emit = defineEmits<{
  retry: []
}>()

const hasError = ref(false)
const errorMessage = ref('')
const errorStack = ref('')

onErrorCaptured((error: Error) => {
  hasError.value = true
  errorMessage.value = error.message
  errorStack.value = error.stack || ''

  // Log error for debugging
  console.error('ErrorBoundary caught error:', error)

  // Prevent error from propagating
  return false
})

function handleRetry() {
  hasError.value = false
  errorMessage.value = ''
  errorStack.value = ''
  emit('retry')
}
</script>

<template>
  <div v-if="hasError" class="min-h-[200px] flex items-center justify-center p-8">
    <div class="text-center max-w-md">
      <!-- Error icon -->
      <div
        class="mx-auto w-16 h-16 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center mb-4"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-8 w-8 text-red-600 dark:text-red-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
      </div>

      <!-- Error title -->
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">
        {{ fallbackTitle }}
      </h3>

      <!-- Error message -->
      <p class="text-gray-600 dark:text-gray-400 mb-4">
        {{ fallbackMessage }}
      </p>

      <!-- Technical details (collapsible) -->
      <details v-if="errorMessage" class="text-left mb-4">
        <summary
          class="cursor-pointer text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
        >
          Technical details
        </summary>
        <div class="mt-2 p-3 bg-gray-100 dark:bg-gray-700 rounded-lg text-xs font-mono">
          <p class="text-red-600 dark:text-red-400 break-all">{{ errorMessage }}</p>
          <pre
            v-if="errorStack"
            class="mt-2 text-gray-600 dark:text-gray-400 overflow-x-auto whitespace-pre-wrap"
            >{{ errorStack }}</pre
          >
        </div>
      </details>

      <!-- Retry button -->
      <button
        v-if="showRetry"
        class="inline-flex items-center gap-2 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors"
        @click="handleRetry"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
        Try again
      </button>
    </div>
  </div>

  <slot v-else></slot>
</template>
