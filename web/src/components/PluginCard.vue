<script setup lang="ts">
import { computed } from 'vue'
import type { Plugin } from '@/api/plugin'

const props = defineProps<{
  plugin: Plugin
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'toggle'): void
  (e: 'configure'): void
  (e: 'viewLogs'): void
  (e: 'reload'): void
}>()

const statusColor = computed(() => {
  switch (props.plugin.status) {
    case 'running':
      return 'bg-green-500'
    case 'loaded':
      return 'bg-blue-500'
    case 'stopped':
      return 'bg-gray-500'
    case 'error':
      return 'bg-red-500'
    default:
      return 'bg-gray-500'
  }
})

const statusText = computed(() => {
  switch (props.plugin.status) {
    case 'running':
      return 'Running'
    case 'loaded':
      return 'Loaded'
    case 'stopped':
      return 'Stopped'
    case 'error':
      return 'Error'
    default:
      return 'Unknown'
  }
})

const typeIcon = computed(() => {
  switch (props.plugin.type) {
    case 'native':
      return 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z'
    case 'js':
      return 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4'
    case 'wasm':
      return 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4'
    default:
      return 'M4 6h16M4 12h16M4 18h16'
  }
})

const typeLabel = computed(() => {
  switch (props.plugin.type) {
    case 'native':
      return 'Native'
    case 'js':
      return 'JavaScript'
    case 'wasm':
      return 'WebAssembly'
    default:
      return props.plugin.type
  }
})
</script>

<template>
  <div
    class="bg-gray-800 rounded-lg p-4 border border-gray-700 hover:border-gray-600 transition-colors"
  >
    <!-- Header -->
    <div class="flex items-start justify-between mb-3">
      <div class="flex items-center gap-3">
        <!-- Type Icon -->
        <div class="w-10 h-10 rounded-lg bg-gray-700 flex items-center justify-center">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 text-blue-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="typeIcon" />
          </svg>
        </div>
        <div>
          <h3 class="text-white font-medium">{{ plugin.name }}</h3>
          <div class="flex items-center gap-2 text-xs text-gray-400">
            <span>v{{ plugin.version }}</span>
            <span>•</span>
            <span>{{ typeLabel }}</span>
          </div>
        </div>
      </div>

      <!-- Status Badge -->
      <div class="flex items-center gap-2">
        <span :class="['w-2 h-2 rounded-full', statusColor]"></span>
        <span class="text-xs text-gray-400">{{ statusText }}</span>
      </div>
    </div>

    <!-- Description -->
    <p class="text-sm text-gray-400 mb-4 line-clamp-2">
      {{ plugin.description || 'No description available' }}
    </p>

    <!-- Error Message -->
    <div
      v-if="plugin.error"
      class="mb-4 p-2 bg-red-900/30 border border-red-800 rounded text-xs text-red-300"
    >
      {{ plugin.error }}
    </div>

    <!-- Author & Capabilities -->
    <div class="flex flex-wrap gap-2 mb-4">
      <span v-if="plugin.author" class="text-xs bg-gray-700 px-2 py-1 rounded text-gray-300">
        by {{ plugin.author }}
      </span>
      <span
        v-for="cap in plugin.capabilities?.slice(0, 3)"
        :key="cap"
        class="text-xs bg-blue-900/30 px-2 py-1 rounded text-blue-300"
      >
        {{ cap }}
      </span>
      <span
        v-if="plugin.capabilities && plugin.capabilities.length > 3"
        class="text-xs bg-gray-700 px-2 py-1 rounded text-gray-400"
      >
        +{{ plugin.capabilities.length - 3 }} more
      </span>
    </div>

    <!-- Actions -->
    <div class="flex items-center justify-between pt-3 border-t border-gray-700">
      <!-- Toggle Switch -->
      <label class="relative inline-flex items-center cursor-pointer">
        <input
          type="checkbox"
          :checked="plugin.enabled"
          :disabled="loading"
          class="sr-only peer"
          @change="emit('toggle')"
        />
        <div
          class="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-500 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"
        ></div>
        <span class="ml-2 text-sm text-gray-400">
          {{ plugin.enabled ? 'Enabled' : 'Disabled' }}
        </span>
      </label>

      <!-- Action Buttons -->
      <div class="flex items-center gap-1">
        <!-- Configure -->
        <button
          v-if="plugin.config_schema"
          class="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
          title="Configure"
          @click="emit('configure')"
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
              d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
            />
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
            />
          </svg>
        </button>

        <!-- View Logs -->
        <button
          class="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
          title="View Logs"
          @click="emit('viewLogs')"
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
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
            />
          </svg>
        </button>

        <!-- Reload -->
        <button
          class="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
          :class="{ 'animate-spin': loading }"
          title="Reload"
          :disabled="loading"
          @click="emit('reload')"
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
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
