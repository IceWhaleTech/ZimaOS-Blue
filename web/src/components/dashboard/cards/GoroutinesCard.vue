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
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-1">{{ t('system.goroutines') }}</p>
      <template v-if="systemStore.loading">
        <Skeleton height="1.75rem" width="50%" rounded="md" />
      </template>
      <p v-else class="text-2xl font-bold text-gray-900 dark:text-white">
        {{ systemStore.health?.goroutines ?? '-' }}
      </p>
    </div>
    <div class="p-3 bg-orange-100 dark:bg-orange-900/30 rounded-full">
      <svg class="w-6 h-6 text-orange-600 dark:text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
      </svg>
    </div>
  </div>
</template>
