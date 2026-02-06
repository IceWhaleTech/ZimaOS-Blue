<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { resetPreviewModeStatus } from '@/router'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const retrying = ref(false)
const autoRetrying = ref(false)

// Storage key for the previous route
const PREVIOUS_ROUTE_KEY = 'connection_error_previous_route'

// Get the previous route from query param or storage
function getPreviousRoute(): string {
  // First check query param
  const fromQuery = route.query.from as string
  if (fromQuery) {
    // Save to storage for page refresh
    sessionStorage.setItem(PREVIOUS_ROUTE_KEY, fromQuery)
    return fromQuery
  }
  // Then check storage
  return sessionStorage.getItem(PREVIOUS_ROUTE_KEY) || '/chat'
}

async function retry(isAutoRetry = false) {
  if (isAutoRetry) {
    autoRetrying.value = true
  } else {
    retrying.value = true
  }

  // Reset the cached status so it will check again
  resetPreviewModeStatus()

  try {
    const response = await fetch('/api/v1/system/mode')
    if (response.ok || response.status < 500) {
      // Connection restored, go to previous route
      const previousRoute = getPreviousRoute()
      // Clear the stored route
      sessionStorage.removeItem(PREVIOUS_ROUTE_KEY)
      router.push(previousRoute)
      return
    }
  } catch {
    // Still failing
  }

  retrying.value = false
  autoRetrying.value = false
}

// Auto retry on mount (page load/refresh)
onMounted(() => {
  retry(true)
})
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-surface-base p-4">
    <div class="max-w-md w-full text-center">
      <!-- Auto-retry loading state -->
      <template v-if="autoRetrying">
        <div class="w-20 h-20 mx-auto mb-6 flex items-center justify-center">
          <svg class="animate-spin h-12 w-12 text-gray-900 dark:text-gray-300" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
        </div>
        <h1 class="text-xl font-medium text-gray-900 dark:text-white mb-2">
          {{ t('errors.checkingConnection') }}
        </h1>
      </template>

      <!-- Error state -->
      <template v-else>
        <!-- Icon -->
        <div class="w-20 h-20 mx-auto mb-6 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        </div>

        <!-- Title -->
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white mb-2">
          {{ t('errors.connectionFailed') }}
        </h1>

        <!-- Description -->
        <p class="text-gray-600 dark:text-gray-400 mb-8">
          {{ t('errors.connectionFailedDesc') }}
        </p>

        <!-- Retry button -->
        <button
          class="px-6 py-3 bg-gray-700 dark:bg-gray-700 hover:bg-gray-700 dark:bg-gray-700/90 text-white font-medium rounded-lg transition-colors disabled:opacity-50"
          :disabled="retrying"
          @click="retry(false)"
        >
          <span v-if="retrying" class="flex items-center justify-center gap-2">
            <svg class="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ t('common.retrying') }}
          </span>
          <span v-else>{{ t('common.retry') }}</span>
        </button>
      </template>
    </div>
  </div>
</template>
