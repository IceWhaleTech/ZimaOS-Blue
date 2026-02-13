<script setup lang="ts">
import type { AutoReplyRule } from '@/api/autoreply'

defineProps<{
  rule: AutoReplyRule
  loading?: boolean
}>()

const emit = defineEmits<{
  toggle: []
  edit: []
  delete: []
  test: []
}>()

function getTriggerTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    keyword: 'Keyword',
    regex: 'Regex',
    contains: 'Contains',
    prefix: 'Prefix',
    suffix: 'Suffix',
  }
  return labels[type] || type
}

function getTriggerTypeColor(type: string): string {
  const colors: Record<string, string> = {
    keyword: 'bg-gray-200 dark:bg-gray-600/20 text-gray-900 dark:text-white',
    regex: 'bg-purple-500/20 text-purple-400',
    contains: 'bg-green-500/20 text-green-400',
    prefix: 'bg-yellow-500/20 text-yellow-400',
    suffix: 'bg-orange-500/20 text-orange-400',
  }
  return colors[type] || 'bg-gray-500/20 text-gray-400'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString([], {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}
</script>

<template>
  <div
    class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 transition-colors shadow"
    :class="{ 'opacity-60': !rule.enabled }"
  >
    <!-- Header -->
    <div class="flex items-start justify-between mb-3">
      <div class="flex-1 min-w-0">
        <h3 class="text-gray-900 dark:text-white font-medium truncate">{{ rule.name }}</h3>
        <div class="flex items-center gap-2 mt-1">
          <span
            :class="getTriggerTypeColor(rule.trigger_type)"
            class="px-2 py-0.5 rounded text-xs font-medium"
          >
            {{ getTriggerTypeLabel(rule.trigger_type) }}
          </span>
          <span class="text-gray-400 dark:text-gray-500 text-xs">Priority: {{ rule.priority }}</span>
        </div>
      </div>
      <button
        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:ring-offset-2 focus:ring-offset-white dark:focus:ring-offset-gray-800"
        :class="rule.enabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
        :disabled="loading"
        @click="emit('toggle')"
      >
        <span
          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
          :class="rule.enabled ? 'translate-x-5' : 'translate-x-0'"
        />
      </button>
    </div>

    <!-- Trigger Value -->
    <div class="mb-3">
      <div class="text-xs text-gray-400 dark:text-gray-500 mb-1">Trigger</div>
      <code class="text-sm text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded block truncate">
        {{ rule.trigger_value }}
      </code>
    </div>

    <!-- Responses Preview -->
    <div class="mb-3">
      <div class="text-xs text-gray-400 dark:text-gray-500 mb-1">
        Responses ({{ rule.responses.length }})
      </div>
      <div class="text-sm text-gray-600 dark:text-gray-400 truncate">
        {{ rule.responses[0] || 'No responses' }}
        <span v-if="rule.responses.length > 1" class="text-gray-400 dark:text-gray-500">
          +{{ rule.responses.length - 1 }} more
        </span>
      </div>
    </div>

    <!-- Channels -->
    <div v-if="rule.channels.length > 0" class="mb-3">
      <div class="text-xs text-gray-400 dark:text-gray-500 mb-1">Channels</div>
      <div class="flex flex-wrap gap-1">
        <span
          v-for="channel in rule.channels"
          :key="channel"
          class="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded text-xs"
        >
          {{ channel }}
        </span>
      </div>
    </div>

    <!-- Stats -->
    <div class="flex items-center justify-between text-xs text-gray-400 dark:text-gray-500 mb-3">
      <span>Matched {{ rule.match_count }} times</span>
      <span>Updated {{ formatDate(rule.updated_at) }}</span>
    </div>

    <!-- Actions -->
    <div class="flex items-center gap-2 pt-3 border-t border-gray-200 dark:border-gray-700">
      <button
        class="flex-1 px-3 py-1.5 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded text-sm transition-colors"
        :disabled="loading"
        @click="emit('edit')"
      >
        Edit
      </button>
      <button
        class="flex-1 px-3 py-1.5 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded text-sm transition-colors"
        :disabled="loading"
        @click="emit('test')"
      >
        Test
      </button>
      <button
        class="px-3 py-1.5 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded text-sm transition-colors"
        :disabled="loading"
        @click="emit('delete')"
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
            d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
          />
        </svg>
      </button>
    </div>
  </div>
</template>
