<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useNetwork, type NetworkInterface } from '@/composables/useNetwork'
import { useTauri } from '@/composables/useTauri'
import CopyButton from './CopyButton.vue'

const { t } = useI18n()
const { addresses, loading, error, fetchAddresses, getInterfaceIcon } = useNetwork()
const { isTauri } = useTauri()

function getInterfaceTypeLabel(type: NetworkInterface['type']): string {
  switch (type) {
    case 'wifi':
      return t('network.wifi')
    case 'ethernet':
      return t('network.ethernet')
    case 'loopback':
      return t('network.loopback')
    case 'virtual':
      return t('network.virtual')
    default:
      return t('network.unknown')
  }
}
</script>

<template>
  <!-- Only render in Tauri desktop app -->
  <div
    v-if="isTauri"
    class="bg-white dark:bg-gray-700 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden"
  >
    <!-- Header -->
    <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-5 h-5 text-gray-900 dark:text-white"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0"
          />
        </svg>
        <h3 class="font-medium text-gray-900 dark:text-white">
          {{ t('network.accessFromOtherDevices') }}
        </h3>
      </div>

      <!-- Refresh button -->
      <button
        type="button"
        class="p-1.5 rounded-md text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
        :disabled="loading"
        @click="fetchAddresses"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-4 h-4"
          :class="{ 'animate-spin': loading }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
      </button>
    </div>

    <!-- Content -->
    <div class="p-4 space-y-4">
      <!-- Error state -->
      <div
        v-if="error"
        class="flex items-center gap-2 text-red-600 dark:text-red-400 text-sm"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <span>{{ error }}</span>
      </div>

      <!-- Loading state -->
      <div v-else-if="loading && !addresses" class="space-y-3">
        <div class="h-10 bg-gray-700 dark:bg-gray-500 rounded animate-pulse" />
        <div class="h-10 bg-gray-700 dark:bg-gray-500 rounded animate-pulse" />
      </div>

      <!-- Addresses -->
      <template v-else-if="addresses">
        <!-- Preferred / LAN Address -->
        <div v-if="addresses.lan.length > 0" class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('network.localNetwork') }}
          </label>
          <div
            v-for="iface in addresses.lan"
            :key="iface.interface"
            class="flex items-center gap-2 p-2 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
          >
            <!-- Interface icon -->
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-4 h-4 text-gray-500 dark:text-gray-400 flex-shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path stroke-linecap="round" stroke-linejoin="round" :d="getInterfaceIcon(iface.type)" />
            </svg>

            <!-- Interface type badge -->
            <span
              class="text-xs px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-600 text-gray-600 dark:text-gray-300"
            >
              {{ getInterfaceTypeLabel(iface.type) }}
            </span>

            <!-- Address -->
            <code class="flex-1 text-sm font-mono text-gray-800 dark:text-gray-200 truncate">
              {{ iface.address }}
            </code>

            <!-- Copy button -->
            <CopyButton :text="iface.address" size="sm" />
          </div>
        </div>

        <!-- Hostname -->
        <div v-if="addresses.hostname" class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('network.hostname') }}
          </label>
          <div class="flex items-center gap-2 p-2 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-4 h-4 text-gray-500 dark:text-gray-400 flex-shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
              />
            </svg>
            <code class="flex-1 text-sm font-mono text-gray-800 dark:text-gray-200 truncate">
              {{ addresses.hostname }}
            </code>
            <CopyButton :text="addresses.hostname" size="sm" />
          </div>
        </div>

        <!-- No LAN access -->
        <div
          v-if="addresses.lan.length === 0"
          class="text-center py-4 text-gray-500 dark:text-gray-400"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-8 h-8 mx-auto mb-2 opacity-50"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M18.364 5.636a9 9 0 010 12.728m0 0l-2.829-2.829m2.829 2.829L21 21M15.536 8.464a5 5 0 010 7.072m0 0l-2.829-2.829m-4.243 2.829a4.978 4.978 0 01-1.414-2.83m-1.414 5.658a9 9 0 01-2.167-9.238m7.824 2.167a1 1 0 111.414 1.414m-1.414-1.414L3 3m8.293 8.293l1.414 1.414"
            />
          </svg>
          <p class="text-sm">{{ t('network.noLanAccess') }}</p>
        </div>
      </template>

      <!-- Tip -->
      <div class="flex items-start gap-2 text-xs text-gray-500 dark:text-gray-400 pt-2 border-t border-gray-200 dark:border-gray-700">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-4 h-4 flex-shrink-0 mt-0.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <span>{{ t('network.sameNetworkTip') }}</span>
      </div>
    </div>
  </div>
</template>
