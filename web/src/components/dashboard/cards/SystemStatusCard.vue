<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'

const { t } = useI18n()
const systemStore = useSystemStore()

const statusText = computed(() => {
  if (systemStore.loading) return '-'
  const status = systemStore.health?.status
  if (!status) return '-'
  switch (status.toLowerCase()) {
    case 'ok':
      return t('system.statusOk')
    case 'error':
      return t('system.statusError')
    case 'degraded':
      return t('system.statusDegraded')
    default:
      return status
  }
})

const statusColorClass = computed(() => {
  if (systemStore.loading || !systemStore.health?.status) {
    return 'text-gray-400 dark:text-gray-500'
  }
  return systemStore.health.status === 'ok'
    ? 'text-green-600 dark:text-green-400'
    : 'text-red-600 dark:text-red-400'
})

const statusDotClass = computed(() => {
  if (systemStore.loading || !systemStore.health?.status) {
    return 'bg-gray-400'
  }
  return systemStore.health.status === 'ok' ? 'bg-green-500' : 'bg-red-500'
})
</script>

<template>
  <div class="flex items-center justify-between">
    <div>
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('system.status') }}</p>
      <div class="flex items-center gap-2">
        <div class="w-3 h-3 rounded-full animate-pulse" :class="statusDotClass"></div>
        <p class="text-2xl font-bold" :class="statusColorClass">
          {{ statusText }}
        </p>
      </div>
    </div>
    <div class="p-3 bg-green-100 dark:bg-green-900/30 rounded-full">
      <svg class="w-6 h-6 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    </div>
  </div>
</template>
