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

const hasTokenBreakdown = computed(() =>
  props.data.promptTokens != null || props.data.completionTokens != null
)

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
  if (tokens == null) return t('common.noData')
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
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24">
          <path
            d="M12 7.8L15.64 9.9V14.1L12 16.2L8.36 14.1V9.9L12 7.8Z"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linejoin="round"
          />
          <circle cx="12" cy="6.1" r="1.15" fill="currentColor"/>
          <circle cx="17.05" cy="9.05" r="1.15" fill="currentColor"/>
          <circle cx="17.05" cy="14.95" r="1.15" fill="currentColor"/>
          <circle cx="12" cy="17.9" r="1.15" fill="currentColor"/>
          <circle cx="6.95" cy="14.95" r="1.15" fill="currentColor"/>
          <circle cx="6.95" cy="9.05" r="1.15" fill="currentColor"/>
          <path d="M12 7.2V6.95" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          <path d="M15.24 9.42L16.15 8.9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          <path d="M15.24 14.58L16.15 15.1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          <path d="M12 16.8V17.05" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          <path d="M8.76 14.58L7.85 15.1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          <path d="M8.76 9.42L7.85 8.9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
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
            <span>{{ t('companion.llmDetails.total') }}: {{ formatTokens(data.totalTokens) }}</span>
          </div>
          <div v-if="data.duration" class="flex items-center gap-1">
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>{{ formatDuration(data.duration) }}</span>
          </div>
        </div>

        <!-- Token breakdown -->
        <div v-if="hasTokenBreakdown" class="mt-2 text-[10px] text-gray-400 dark:text-gray-500">
          <span>{{ t('companion.nodes.tokensIn') }}: {{ formatTokens(data.promptTokens) }}</span>
          <span class="mx-1">|</span>
          <span>{{ t('companion.nodes.tokensOut') }}: {{ formatTokens(data.completionTokens) }}</span>
        </div>
      </div>
    </div>

    <Handle type="source" :position="Position.Bottom" class="!bg-gray-400" />
  </div>
</template>
