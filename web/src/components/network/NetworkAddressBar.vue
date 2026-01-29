<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useNetwork } from '@/composables/useNetwork'
import { useTauri } from '@/composables/useTauri'
import CopyButton from './CopyButton.vue'

const { t } = useI18n()
const { addresses, loading } = useNetwork()
const { isTauri } = useTauri()

const preferredAddress = computed(() => {
  if (!addresses.value) return null
  return addresses.value.preferred
})

const showBar = computed(() => {
  // Only show in Tauri desktop app and when LAN addresses are available
  return isTauri.value && addresses.value && addresses.value.lan.length > 0
})
</script>

<template>
  <div
    v-if="showBar"
    class="flex items-center gap-2 px-3 py-1.5 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800"
  >
    <!-- Globe icon -->
    <svg
      xmlns="http://www.w3.org/2000/svg"
      class="w-4 h-4 text-blue-500 dark:text-blue-400 flex-shrink-0"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
      />
    </svg>

    <span class="text-sm text-blue-700 dark:text-blue-300 whitespace-nowrap">
      {{ t('network.accessFrom') }}:
    </span>

    <code
      class="text-sm font-mono text-blue-800 dark:text-blue-200 bg-blue-100 dark:bg-blue-800/30 px-2 py-0.5 rounded"
    >
      {{ preferredAddress }}
    </code>

    <CopyButton v-if="preferredAddress" :text="preferredAddress" size="sm" />

    <!-- Loading indicator -->
    <div v-if="loading" class="animate-spin">
      <svg class="w-4 h-4 text-blue-500" fill="none" viewBox="0 0 24 24">
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        />
      </svg>
    </div>
  </div>
</template>
