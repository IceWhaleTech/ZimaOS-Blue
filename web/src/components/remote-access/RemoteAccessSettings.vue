<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getRemoteAccessStatus,
  startRemoteAccess,
  stopRemoteAccess,
  type TunnelStatus as TunnelStatusType
} from '@/api/remote-access'
import TunnelStatus from './TunnelStatus.vue'

const { t } = useI18n()

type ViewState = 'loading' | 'ready' | 'connecting' | 'connected' | 'error'

const state = ref<ViewState>('loading')
const tunnelStatus = ref<TunnelStatusType | null>(null)
const error = ref<string | null>(null)

let statusInterval: ReturnType<typeof setInterval> | null = null

async function loadStatus() {
  try {
    const statusRes = await getRemoteAccessStatus()
    tunnelStatus.value = statusRes.data.tunnel

    if (statusRes.data.tunnel.connecting) {
      state.value = 'connecting'
      // Start polling to detect when connection is established
      startStatusPolling()
    } else if (statusRes.data.tunnel.active) {
      state.value = 'connected'
    } else {
      state.value = 'ready'
    }
  } catch (e) {
    console.error('Failed to load status:', e)
    error.value = t('remoteAccess.loadError')
    state.value = 'error'
  }
}

async function handleStart() {
  state.value = 'connecting'
  error.value = null

  try {
    const response = await startRemoteAccess()
    if (response.data.success) {
      // Start polling for status
      startStatusPolling()
    } else {
      throw new Error(response.data.message || 'Failed to start tunnel')
    }
  } catch (e: unknown) {
    console.error('Failed to start tunnel:', e)
    const err = e as { response?: { data?: { error_code?: string; error?: string } }; message?: string }
    state.value = 'error'
    error.value = err.response?.data?.error || err.message || t('remoteAccess.startError')
  }
}

async function handleStop() {
  try {
    await stopRemoteAccess()
    stopStatusPolling()
    tunnelStatus.value = { active: false }
    state.value = 'ready'
  } catch (e) {
    console.error('Failed to stop tunnel:', e)
    error.value = t('remoteAccess.stopError')
  }
}

function startStatusPolling() {
  // Use adaptive polling: faster when connecting, slower when connected
  const pollInterval = state.value === 'connecting' ? 5000 : 15000

  statusInterval = setInterval(async () => {
    try {
      const response = await getRemoteAccessStatus()
      tunnelStatus.value = response.data.tunnel

      if (response.data.tunnel.active) {
        state.value = 'connected'
        // Switch to slower polling when connected
        if (statusInterval) {
          clearInterval(statusInterval)
          statusInterval = setInterval(async () => {
            try {
              const response = await getRemoteAccessStatus()
              tunnelStatus.value = response.data.tunnel
              if (!response.data.tunnel.active) {
                state.value = 'ready'
                stopStatusPolling()
              }
            } catch (e) {
              console.error('Failed to poll status:', e)
            }
          }, 15000) // Poll every 15 seconds when connected
        }
      } else if (state.value === 'connecting') {
        // Still waiting for tunnel to start
      } else {
        state.value = 'ready'
        stopStatusPolling()
      }
    } catch (e) {
      console.error('Failed to poll status:', e)
    }
  }, pollInterval)
}

function stopStatusPolling() {
  if (statusInterval) {
    clearInterval(statusInterval)
    statusInterval = null
  }
}

onMounted(() => {
  loadStatus()
})

onUnmounted(() => {
  stopStatusPolling()
})

// Watch for tunnel becoming active
watch(() => tunnelStatus.value?.active, (active) => {
  if (active && state.value === 'connecting') {
    state.value = 'connected'
  }
})
</script>

<template>
  <div class="remote-access-settings">
    <div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
      <!-- Header -->
      <div class="flex items-center justify-between mb-6">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-blue-100 dark:bg-blue-900 rounded-lg">
            <svg class="h-6 w-6 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
            </svg>
          </div>
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {{ t('remoteAccess.title') }}
            </h3>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('remoteAccess.description') }}
            </p>
          </div>
        </div>

        <!-- Toggle Button (when connected) -->
        <button
          v-if="state === 'connected'"
          class="px-4 py-2 text-sm text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 border border-red-300 dark:border-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
          @click="handleStop"
        >
          {{ t('remoteAccess.disable') }}
        </button>
      </div>

      <!-- Loading State -->
      <div v-if="state === 'loading'" class="flex items-center justify-center py-8">
        <svg class="animate-spin h-8 w-8 text-blue-600" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
      </div>

      <!-- Ready State -->
      <div v-else-if="state === 'ready'" class="space-y-4">
        <div class="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
          <p class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('remoteAccess.readyDescription') }}
          </p>
        </div>

        <button
          class="w-full px-4 py-3 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium transition-colors flex items-center justify-center gap-2"
          @click="handleStart"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
          {{ t('remoteAccess.enable') }}
        </button>

        <div class="text-xs text-gray-500 dark:text-gray-400 text-center">
          {{ t('remoteAccess.securityWarning') }}
        </div>
      </div>

      <!-- Connecting State -->
      <div v-else-if="state === 'connecting'" class="space-y-4">
        <div class="flex items-center justify-center py-8">
          <div class="text-center">
            <svg class="animate-spin h-8 w-8 text-blue-600 mx-auto mb-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
            </svg>
            <p class="text-gray-600 dark:text-gray-400">{{ t('remoteAccess.connecting') }}</p>
          </div>
        </div>

        <!-- Antivirus Warning -->
        <div class="bg-amber-50 dark:bg-amber-900/20 rounded-lg p-4 border border-amber-200 dark:border-amber-800">
          <div class="flex items-start gap-3">
            <svg class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <div>
              <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
                {{ t('remoteAccess.antivirusWarningTitle') }}
              </p>
              <p class="mt-1 text-sm text-amber-700 dark:text-amber-300">
                {{ t('remoteAccess.antivirusWarningDesc') }}
              </p>
              <ul class="mt-2 text-sm text-amber-700 dark:text-amber-300 list-disc list-inside space-y-1">
                <li>{{ t('remoteAccess.antivirusHint1') }}</li>
                <li>{{ t('remoteAccess.antivirusHint2') }}</li>
                <li>{{ t('remoteAccess.antivirusHint3') }}</li>
              </ul>
            </div>
          </div>
        </div>
      </div>

      <!-- Connected State -->
      <div v-else-if="state === 'connected' && tunnelStatus" class="space-y-4">
        <TunnelStatus :status="tunnelStatus" />
      </div>

      <!-- Error State -->
      <div v-else-if="state === 'error'" class="space-y-4">
        <div class="bg-red-50 dark:bg-red-900/20 rounded-lg p-4">
          <div class="flex items-start gap-3">
            <svg class="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <div>
              <p class="text-sm text-red-800 dark:text-red-200">{{ error }}</p>
            </div>
          </div>
        </div>

        <button
          class="w-full px-4 py-3 bg-gray-600 hover:bg-gray-700 text-white rounded-lg font-medium transition-colors"
          @click="loadStatus"
        >
          {{ t('common.retry') }}
        </button>
      </div>
    </div>
  </div>
</template>
