<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import Skeleton from '@/components/Skeleton.vue'

const { t } = useI18n()
const systemStore = useSystemStore()
</script>

<template>
  <div class="flex items-center justify-between">
    <div>
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('system.uptime') }}</p>
      <template v-if="systemStore.loading">
        <Skeleton height="1.75rem" width="80%" rounded="md" />
      </template>
      <p v-else class="text-2xl font-bold text-gray-900 dark:text-white truncate">
        {{ systemStore.health?.uptime || '-' }}
      </p>
    </div>
    <div class="p-3 bg-cyan-100 dark:bg-cyan-900/30 rounded-full">
      <svg class="w-6 h-6 text-cyan-600 dark:text-cyan-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    </div>
  </div>
</template>
