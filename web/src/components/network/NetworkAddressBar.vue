<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPreferredNetworkAddress, useNetwork } from '@/composables/useNetwork'
import { useTauri } from '@/composables/useTauri'

const { t } = useI18n()
const { addresses, loading } = useNetwork()
const { isTauri, browserName, openInBrowser } = useTauri()

const opening = ref(false)

const preferredAddress = computed(() =>
  getPreferredNetworkAddress(addresses.value, { isTauri: isTauri.value })
)

const showButton = computed(() => {
  // Only show in Tauri desktop app and when LAN addresses are available
  return isTauri.value && addresses.value && addresses.value.lan.length > 0
})

async function handleOpenInBrowser() {
  if (!preferredAddress.value || opening.value) return

  opening.value = true
  try {
    await openInBrowser(preferredAddress.value)
  } finally {
    opening.value = false
  }
}
</script>

<template>
  <button
    v-if="showButton"
    type="button"
    :disabled="opening || loading"
    class="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-sm font-medium text-gray-900 dark:text-gray-200 bg-gray-100 dark:bg-gray-700/30 hover:bg-gray-200 dark:hover:bg-gray-700/40 rounded-lg border border-gray-200 dark:border-gray-600 transition-colors disabled:opacity-50"
    :title="t('network.openInBrowser')"
    @click="handleOpenInBrowser"
  >
    <!-- External link icon -->
    <svg
      xmlns="http://www.w3.org/2000/svg"
      class="w-4 h-4"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
      />
    </svg>
    <span>{{ t('network.openIn', { browser: browserName }) }}</span>
    <!-- Loading spinner -->
    <svg v-if="opening || loading" class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
      <path
        class="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      />
    </svg>
  </button>
</template>
