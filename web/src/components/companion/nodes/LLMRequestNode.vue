<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  data: {
    label: string
    provider?: string
    promptTokens?: number
    completionTokens?: number
    totalTokens?: number
    status: string
    duration?: number
  }
}>()

const statusColor = computed(() => {
  switch (props.data.status) {
    case 'completed':
    case 'success':
      return 'bg-indigo-50 dark:bg-indigo-900/30 border-indigo-200 dark:border-indigo-700'
    case 'failed':
    case 'error':
      return 'bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-700'
    default:
      return 'bg-indigo-50 dark:bg-indigo-900/30 border-indigo-200 dark:border-indigo-700'
  }
})

function formatDuration(ms?: number): string {
  if (!ms) return ''
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

function formatTokens(tokens?: number): string {
  if (!tokens) return '0'
  return tokens.toLocaleString()
}
</script>

<template>
  <div
    :class="[
      'px-4 py-3 rounded-lg border-2 shadow-sm min-w-[220px] max-w-[320px]',
      statusColor
    ]"
  >
    <Handle type="target" :position="Position.Top" class="!bg-gray-400" />

    <div class="flex items-start gap-3">
      <div class="p-2 rounded-full bg-white dark:bg-gray-700 text-indigo-500 dark:text-indigo-400">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
      </div>

      <div class="flex-1 min-w-0">
        <div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1 flex items-center gap-2">
          <span>{{ t('companion.nodes.llmRequest') }}</span>
          <span
            v-if="data.provider"
            class="px-1.5 py-0.5 text-[10px] bg-indigo-100 dark:bg-indigo-900/50 text-indigo-600 dark:text-indigo-400 rounded capitalize"
          >
            {{ data.provider }}
          </span>
        </div>
        <div class="text-sm font-medium text-gray-900 dark:text-white truncate">
          {{ data.label }}
        </div>

        <!-- Token info -->
        <div class="flex items-center gap-3 mt-2 text-xs text-gray-500 dark:text-gray-400">
          <div class="flex items-center gap-1">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
            </svg>
            <span>{{ formatTokens(data.totalTokens) }}</span>
          </div>
          <div v-if="data.duration" class="flex items-center gap-1">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>{{ formatDuration(data.duration) }}</span>
          </div>
        </div>

        <!-- Token breakdown -->
        <div v-if="data.promptTokens || data.completionTokens" class="mt-2 text-[10px] text-gray-400 dark:text-gray-500">
          <span>{{ t('companion.nodes.tokensIn') }}: {{ formatTokens(data.promptTokens) }}</span>
          <span class="mx-1">|</span>
          <span>{{ t('companion.nodes.tokensOut') }}: {{ formatTokens(data.completionTokens) }}</span>
        </div>
      </div>
    </div>

    <Handle type="source" :position="Position.Bottom" class="!bg-gray-400" />
  </div>
</template>
