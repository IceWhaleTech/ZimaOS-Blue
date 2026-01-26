<script setup lang="ts">
import { ref, computed, watch } from 'vue'

export interface LogEntry {
  id: string
  timestamp: Date
  level: 'debug' | 'info' | 'warn' | 'error'
  source: string
  message: string
  metadata?: Record<string, unknown>
}

const props = withDefaults(
  defineProps<{
    entries?: LogEntry[]
    loading?: boolean
    streaming?: boolean
  }>(),
  {
    entries: () => [],
    loading: false,
    streaming: false,
  }
)

const emit = defineEmits<{
  refresh: []
  clear: []
  export: []
  toggleStreaming: []
  filterChange: [filters: { level: string[]; source: string; search: string }]
}>()

const levelFilter = ref<string[]>(['debug', 'info', 'warn', 'error'])
const sourceFilter = ref('')
const searchQuery = ref('')
const autoScroll = ref(true)
const logContainer = ref<HTMLElement | null>(null)

const sources = computed(() => {
  const uniqueSources = new Set(props.entries.map((e) => e.source))
  return Array.from(uniqueSources).sort()
})

const filteredEntries = computed(() => {
  return props.entries.filter((entry) => {
    if (!levelFilter.value.includes(entry.level)) return false
    if (sourceFilter.value && entry.source !== sourceFilter.value) return false
    if (searchQuery.value) {
      const query = searchQuery.value.toLowerCase()
      return (
        entry.message.toLowerCase().includes(query) ||
        entry.source.toLowerCase().includes(query)
      )
    }
    return true
  })
})

watch(
  () => props.entries.length,
  () => {
    if (autoScroll.value && logContainer.value) {
      setTimeout(() => {
        logContainer.value?.scrollTo({
          top: logContainer.value.scrollHeight,
          behavior: 'smooth',
        })
      }, 100)
    }
  }
)

function getLevelColor(level: LogEntry['level']): string {
  switch (level) {
    case 'debug':
      return 'text-gray-500 dark:text-gray-400'
    case 'info':
      return 'text-blue-600 dark:text-blue-400'
    case 'warn':
      return 'text-yellow-600 dark:text-yellow-400'
    case 'error':
      return 'text-red-600 dark:text-red-400'
  }
}

function getLevelBgColor(level: LogEntry['level']): string {
  switch (level) {
    case 'debug':
      return 'bg-gray-100 dark:bg-gray-700'
    case 'info':
      return 'bg-blue-100 dark:bg-blue-900/30'
    case 'warn':
      return 'bg-yellow-100 dark:bg-yellow-900/30'
    case 'error':
      return 'bg-red-100 dark:bg-red-900/30'
  }
}

function formatTimestamp(date: Date): string {
  return new Date(date).toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3,
  })
}

function toggleLevel(level: string) {
  const index = levelFilter.value.indexOf(level)
  if (index === -1) {
    levelFilter.value.push(level)
  } else {
    levelFilter.value.splice(index, 1)
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg shadow flex flex-col h-full">
    <!-- Header -->
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Log Viewer</h2>
        <div class="flex items-center gap-2">
          <button
            class="p-2 rounded-lg transition-colors"
            :class="streaming ? 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700'"
            title="Toggle live streaming"
            @click="emit('toggleStreaming')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.636 18.364a9 9 0 010-12.728m12.728 0a9 9 0 010 12.728m-9.9-2.829a5 5 0 010-7.07m7.072 0a5 5 0 010 7.07M13 12a1 1 0 11-2 0 1 1 0 012 0z" />
            </svg>
          </button>
          <button
            class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            title="Refresh"
            @click="emit('refresh')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
          <button
            class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            title="Export logs"
            @click="emit('export')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>
          <button
            class="p-2 text-gray-500 hover:text-red-600 dark:text-gray-400 dark:hover:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            title="Clear logs"
            @click="emit('clear')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Filters -->
      <div class="flex flex-wrap items-center gap-4">
        <!-- Level filters -->
        <div class="flex items-center gap-1">
          <button
            v-for="level in ['debug', 'info', 'warn', 'error']"
            :key="level"
            class="px-2 py-1 text-xs font-medium rounded transition-colors"
            :class="levelFilter.includes(level) ? getLevelBgColor(level as LogEntry['level']) + ' ' + getLevelColor(level as LogEntry['level']) : 'bg-gray-100 dark:bg-gray-700 text-gray-400'"
            @click="toggleLevel(level)"
          >
            {{ level.toUpperCase() }}
          </button>
        </div>

        <!-- Source filter -->
        <select
          v-model="sourceFilter"
          class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
        >
          <option value="">All sources</option>
          <option v-for="source in sources" :key="source" :value="source">
            {{ source }}
          </option>
        </select>

        <!-- Search -->
        <div class="flex-1 min-w-[200px]">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search logs..."
            class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
          />
        </div>

        <!-- Auto-scroll toggle -->
        <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
          <input
            v-model="autoScroll"
            type="checkbox"
            class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
          />
          Auto-scroll
        </label>
      </div>
    </div>

    <!-- Log entries -->
    <div
      ref="logContainer"
      class="flex-1 overflow-y-auto font-mono text-sm"
    >
      <div v-if="loading" class="p-6 text-center">
        <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"></div>
        <p class="mt-2 text-gray-500 dark:text-gray-400">Loading logs...</p>
      </div>

      <div v-else-if="filteredEntries.length === 0" class="p-6 text-center text-gray-500 dark:text-gray-400">
        No log entries match your filters
      </div>

      <div v-else class="divide-y divide-gray-100 dark:divide-gray-700">
        <div
          v-for="entry in filteredEntries"
          :key="entry.id"
          class="px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-700/50"
        >
          <div class="flex items-start gap-3">
            <span class="text-gray-400 dark:text-gray-500 whitespace-nowrap">
              {{ formatTimestamp(entry.timestamp) }}
            </span>
            <span
              class="px-1.5 py-0.5 text-xs font-medium rounded uppercase"
              :class="getLevelBgColor(entry.level) + ' ' + getLevelColor(entry.level)"
            >
              {{ entry.level }}
            </span>
            <span class="text-gray-500 dark:text-gray-400 whitespace-nowrap">
              [{{ entry.source }}]
            </span>
            <span class="text-gray-900 dark:text-gray-100 break-all">
              {{ entry.message }}
            </span>
          </div>
          <details v-if="entry.metadata" class="ml-[200px] mt-1">
            <summary class="text-xs text-gray-500 dark:text-gray-400 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300">
              Metadata
            </summary>
            <pre class="mt-1 text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded overflow-x-auto">{{ JSON.stringify(entry.metadata, null, 2) }}</pre>
          </details>
        </div>
      </div>
    </div>

    <!-- Footer -->
    <div class="px-6 py-3 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
      <span>{{ filteredEntries.length }} of {{ entries.length }} entries</span>
      <span v-if="streaming" class="flex items-center gap-2">
        <span class="w-2 h-2 bg-green-500 rounded-full animate-pulse"></span>
        Live streaming
      </span>
    </div>
  </div>
</template>
