<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import Skeleton from '@/components/Skeleton.vue'

const { t } = useI18n()
const systemStore = useSystemStore()

function formatBytes(bytes: number | undefined | null): string {
  if (bytes == null) return '-'
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}
</script>

<template>
  <div class="flex items-center justify-between">
    <div>
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('system.memory') }}</p>
      <template v-if="systemStore.loading">
        <Skeleton height="1.75rem" width="70%" rounded="md" />
      </template>
      <p v-else class="text-2xl font-bold text-gray-900 dark:text-white">
        {{ formatBytes(systemStore.health?.mem_alloc_bytes) }}
      </p>
    </div>
    <div class="p-3 bg-purple-100 dark:bg-purple-900/30 rounded-full">
      <svg class="w-6 h-6 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
      </svg>
    </div>
  </div>
</template>
