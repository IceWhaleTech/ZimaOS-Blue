<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  data: {
    label: string
    toolId?: string
    sandboxUsed?: boolean
    status: string
    duration?: number
  }
}>()

const statusColor = computed(() => {
  switch (props.data.status) {
    case 'completed':
    case 'success':
      return 'bg-green-50 dark:bg-green-900/30 border-green-200 dark:border-green-700'
    case 'running':
    case 'pending':
      return 'bg-yellow-50 dark:bg-yellow-900/30 border-yellow-200 dark:border-yellow-700'
    case 'failed':
    case 'error':
      return 'bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-700'
    default:
      return 'bg-purple-50 dark:bg-purple-900/30 border-purple-200 dark:border-purple-700'
  }
})

const statusIcon = computed(() => {
  switch (props.data.status) {
    case 'completed':
    case 'success':
      return 'M5 13l4 4L19 7'
    case 'running':
    case 'pending':
      return 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z'
    case 'failed':
    case 'error':
      return 'M6 18L18 6M6 6l12 12'
    default:
      return 'M13 10V3L4 14h7v7l9-11h-7z'
  }
})

const iconColor = computed(() => {
  switch (props.data.status) {
    case 'completed':
    case 'success':
      return 'text-green-500 dark:text-green-400'
    case 'running':
    case 'pending':
      return 'text-yellow-500 dark:text-yellow-400'
    case 'failed':
    case 'error':
      return 'text-red-500 dark:text-red-400'
    default:
      return 'text-purple-500 dark:text-purple-400'
  }
})

function formatDuration(ms?: number): string {
  if (!ms) return ''
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(2)}s`
}
</script>

<template>
  <div
    :class="['px-4 py-3 rounded-lg border-2 shadow-sm min-w-[200px] max-w-[300px]', statusColor]"
  >
    <Handle
      type="target"
      :position="Position.Top"
      class="!bg-gray-400"
    />

    <div class="flex items-start gap-3">
      <div :class="['p-2 rounded-full bg-white dark:bg-gray-700', iconColor]">
        <svg
          class="w-4 h-4"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            :d="statusIcon"
          />
        </svg>
      </div>

      <div class="flex-1 min-w-0">
        <div
          class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1 flex items-center gap-2"
        >
          <span>{{ t('companion.nodes.toolCall') }}</span>
          <span
            v-if="data.sandboxUsed"
            class="px-1.5 py-0.5 text-[10px] bg-orange-100 dark:bg-orange-900/50 text-orange-600 dark:text-orange-400 rounded"
          >
            {{ t('companion.nodes.sandbox') }}
          </span>
        </div>
        <div class="text-sm font-medium text-gray-900 dark:text-white truncate">
          {{ data.label }}
        </div>
        <div class="flex items-center gap-2 mt-1">
          <span
            :class="[
              'px-1.5 py-0.5 text-[10px] rounded',
              data.status === 'completed' || data.status === 'success'
                ? 'bg-green-100 dark:bg-green-900/50 text-green-600 dark:text-green-400'
                : data.status === 'failed' || data.status === 'error'
                  ? 'bg-red-100 dark:bg-red-900/50 text-red-600 dark:text-red-400'
                  : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400',
            ]"
          >
            {{ t(`companion.nodes.status.${data.status}`) }}
          </span>
          <span
            v-if="data.duration"
            class="text-xs text-gray-400 dark:text-gray-500"
          >
            {{ formatDuration(data.duration) }}
          </span>
        </div>
      </div>
    </div>

    <Handle
      type="source"
      :position="Position.Bottom"
      class="!bg-gray-400"
    />
  </div>
</template>
