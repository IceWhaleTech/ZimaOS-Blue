<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

export interface ActivityLogEntry {
  id: string
  timestamp: Date
  level: 'info' | 'warning' | 'error' | 'debug'
  source: string
  message: string
  details?: Record<string, unknown>
}

const props = withDefaults(
  defineProps<{
    entries?: ActivityLogEntry[]
    loading?: boolean
    maxEntries?: number
  }>(),
  {
    entries: () => [],
    loading: false,
    maxEntries: 50,
  }
)

const emit = defineEmits<{
  refresh: []
  clear: []
}>()

const displayedEntries = computed(() => {
  return props.entries.slice(0, props.maxEntries)
})

function getLevelColor(level: ActivityLogEntry['level']): string {
  switch (level) {
    case 'error':
      return 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20'
    case 'warning':
      return 'text-yellow-600 dark:text-yellow-400 bg-yellow-50 dark:bg-yellow-900/20'
    case 'info':
      return 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20'
    case 'debug':
      return 'text-gray-600 dark:text-gray-400 bg-gray-50 dark:bg-gray-700'
  }
}

function getLevelIcon(level: ActivityLogEntry['level']): string {
  switch (level) {
    case 'error':
      return 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z'
    case 'warning':
      return 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z'
    case 'info':
      return 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
    case 'debug':
      return 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4'
  }
}

function formatTime(date: Date): string {
  return new Date(date).toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg shadow">
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('common.recentActivityTitle') }}</h2>
      <div class="flex items-center gap-2">
        <button
          class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700"
          :title="t('common.refresh')"
          @click="emit('refresh')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
        <button
          class="p-2 text-gray-500 hover:text-red-600 dark:text-gray-400 dark:hover:text-red-400 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700"
          :title="t('common.clear')"
          @click="emit('clear')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>
      </div>
    </div>

    <div v-if="loading" class="p-6 text-center">
      <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"></div>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('common.loadingActivity') }}</p>
    </div>

    <div v-else-if="entries.length === 0" class="p-6 text-center text-gray-500 dark:text-gray-400">
      {{ t('common.noRecentActivity') }}
    </div>

    <div v-else class="divide-y divide-gray-200 dark:divide-gray-700 max-h-96 overflow-y-auto">
      <div
        v-for="entry in displayedEntries"
        :key="entry.id"
        class="px-6 py-3 hover:bg-gray-50 dark:hover:bg-gray-700/50"
      >
        <div class="flex items-start gap-3">
          <!-- Level icon -->
          <div
            class="flex-shrink-0 p-1.5 rounded-full"
            :class="getLevelColor(entry.level)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getLevelIcon(entry.level)" />
            </svg>
          </div>

          <!-- Content -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ entry.source }}
              </span>
              <span class="text-xs text-gray-400 dark:text-gray-500">
                {{ formatTime(entry.timestamp) }}
              </span>
            </div>
            <p class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
              {{ entry.message }}
            </p>
            <details v-if="entry.details" class="mt-1">
              <summary class="text-xs text-gray-500 dark:text-gray-400 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300">
                {{ t('common.details') }}
              </summary>
              <pre class="mt-1 text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded overflow-x-auto">{{ JSON.stringify(entry.details, null, 2) }}</pre>
            </details>
          </div>
        </div>
      </div>
    </div>

    <div v-if="entries.length > maxEntries" class="px-6 py-3 border-t border-gray-200 dark:border-gray-700 text-center">
      <span class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('common.showingEntries', { shown: maxEntries, total: entries.length }) }}
      </span>
    </div>
  </div>
</template>
