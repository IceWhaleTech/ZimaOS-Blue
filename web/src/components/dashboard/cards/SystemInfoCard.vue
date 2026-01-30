<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'

const { t } = useI18n()
const systemStore = useSystemStore()

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString([], {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}
</script>

<template>
  <div class="grid grid-cols-3 gap-3">
    <!-- Version -->
    <div class="border border-gray-200 dark:border-gray-700 rounded-xl p-3 bg-white dark:bg-gray-800">
      <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">{{ t('system.version') }}</p>
      <p class="text-base font-semibold text-gray-900 dark:text-white">{{ systemStore.health?.version || '-' }}</p>
    </div>

    <!-- Go Version -->
    <div class="border border-gray-200 dark:border-gray-700 rounded-xl p-3 bg-white dark:bg-gray-800">
      <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">{{ t('system.goVersion') }}</p>
      <p class="text-base font-semibold text-gray-900 dark:text-white">{{ systemStore.health?.go_version || '-' }}</p>
    </div>

    <!-- Timestamp -->
    <div class="border border-gray-200 dark:border-gray-700 rounded-xl p-3 bg-white dark:bg-gray-800">
      <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">{{ t('system.timestamp') }}</p>
      <p class="text-base font-semibold text-gray-900 dark:text-white">{{ formatDate(systemStore.health?.timestamp) }}</p>
    </div>
  </div>
</template>
