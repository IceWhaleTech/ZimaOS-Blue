<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TunnelStatus as TunnelStatusType } from '@/api/remote-access'

const { t } = useI18n()

const props = defineProps<{
  status: TunnelStatusType
}>()

const statusColor = computed(() => {
  if (props.status.active) return 'text-green-600 dark:text-green-400'
  return 'text-gray-500 dark:text-gray-400'
})

const statusIcon = computed(() => {
  if (props.status.active) return '🟢'
  return '⚪'
})

const statusText = computed(() => {
  if (props.status.active) return t('remoteAccess.connected')
  return t('remoteAccess.disconnected')
})

function copyUrl() {
  if (props.status.url) {
    navigator.clipboard.writeText(props.status.url)
  }
}

function openUrl() {
  if (props.status.url) {
    window.open(props.status.url, '_blank')
  }
}
</script>

<template>
  <div class="tunnel-status">
    <!-- Status Header -->
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-2">
        <span>{{ statusIcon }}</span>
        <span :class="statusColor" class="font-medium">{{ statusText }}</span>
      </div>
      <span v-if="status.remaining_time" class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('remoteAccess.remainingTime', { time: status.remaining_time }) }}
      </span>
    </div>

    <!-- URL Display -->
    <div v-if="status.active && status.url" class="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 mb-4">
      <div class="text-sm text-gray-500 dark:text-gray-400 mb-2">
        {{ t('remoteAccess.accessUrl') }}
      </div>
      <div class="flex items-center gap-2">
        <code class="flex-1 text-sm bg-white dark:bg-gray-900 px-3 py-2 rounded border border-gray-200 dark:border-gray-700 overflow-x-auto">
          {{ status.url }}
        </code>
        <button
          class="p-2 text-gray-500 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
          :title="t('common.copy')"
          @click="copyUrl"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
        </button>
        <button
          class="p-2 text-gray-500 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
          :title="t('common.openInNewTab')"
          @click="openUrl"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Session Info -->
    <div v-if="status.active" class="grid grid-cols-2 gap-4 text-sm">
      <div>
        <div class="text-gray-500 dark:text-gray-400">{{ t('remoteAccess.startedAt') }}</div>
        <div class="text-gray-900 dark:text-gray-100">
          {{ status.started_at ? new Date(status.started_at).toLocaleString() : '-' }}
        </div>
      </div>
      <div>
        <div class="text-gray-500 dark:text-gray-400">{{ t('remoteAccess.expiresAt') }}</div>
        <div class="text-gray-900 dark:text-gray-100">
          {{ status.expires_at ? new Date(status.expires_at).toLocaleString() : '-' }}
        </div>
      </div>
      <div v-if="status.renewed_count !== undefined && status.renewed_count > 0">
        <div class="text-gray-500 dark:text-gray-400">{{ t('remoteAccess.renewedCount') }}</div>
        <div class="text-gray-900 dark:text-gray-100">{{ status.renewed_count }}</div>
      </div>
    </div>
  </div>
</template>
