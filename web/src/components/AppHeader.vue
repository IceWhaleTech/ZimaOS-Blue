<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { storeToRefs } from 'pinia'

const { t } = useI18n()
const systemStore = useSystemStore()
const { health } = storeToRefs(systemStore)

// Emit event to toggle sidebar
const emit = defineEmits<{
  toggleSidebar: []
}>()
</script>

<template>
  <header class="glass-header px-4 sm:px-6 py-3 sm:py-4 sticky top-0 z-40">
    <div class="flex items-center justify-between">
      <div class="flex items-center space-x-3 sm:space-x-4">
        <!-- Mobile menu button -->
        <button
          class="lg:hidden p-2 -ml-2 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
          @click="emit('toggleSidebar')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <h1 class="text-lg sm:text-xl font-bold text-gray-900 dark:text-white">ZimaOS Echo</h1>
        <span
          v-if="health"
          class="hidden sm:inline-flex px-3 py-1 text-xs font-medium rounded-full transition-all duration-200"
          :class="health.status === 'ok'
            ? 'bg-cta/20 text-cta border border-cta/30'
            : 'bg-red-500/20 text-red-400 border border-red-500/30'"
        >
          <span class="flex items-center space-x-1.5">
            <span
              class="w-2 h-2 rounded-full animate-pulse"
              :class="health.status === 'ok' ? 'bg-cta' : 'bg-red-400'"
            ></span>
            <span>{{ health.status === 'ok' ? t('common.online') : health.status }}</span>
          </span>
        </span>
        <!-- Mobile status indicator (smaller) -->
        <span
          v-if="health"
          class="sm:hidden w-2.5 h-2.5 rounded-full animate-pulse"
          :class="health.status === 'ok' ? 'bg-cta' : 'bg-red-400'"
        ></span>
      </div>
      <div class="flex items-center space-x-2 sm:space-x-4">
        <span v-if="health" class="text-xs sm:text-sm text-gray-500 dark:text-slate-400 font-medium">
          v{{ health.version }}
        </span>
      </div>
    </div>
  </header>
</template>
