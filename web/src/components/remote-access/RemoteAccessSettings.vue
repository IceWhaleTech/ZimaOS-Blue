<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { onClickOutside } from '@vueuse/core'
import {
  getRemoteAccessStatus,
  getRemoteAccessConfig,
  updateRemoteAccessConfig,
  getTunnelProviders,
  startRemoteAccess,
  stopRemoteAccess,
  type TunnelStatus as TunnelStatusType,
  type TunnelProvider,
  type RemoteAccessConfig
} from '@/api/remote-access'
import TunnelStatus from './TunnelStatus.vue'
import { getTunnelProviderIcon } from '@/utils/channelIcons'

const { t } = useI18n()

type ViewState = 'loading' | 'ready' | 'connecting' | 'connected' | 'error'

const state = ref<ViewState>('loading')
const tunnelStatus = ref<TunnelStatusType | null>(null)
const error = ref<string | null>(null)
const providers = ref<TunnelProvider[]>([])
const config = ref<RemoteAccessConfig | null>(null)

// Form state
const selectedProvider = ref('auto')
const ngrokAuthtoken = ref('')
const ngrokDomain = ref('')
const showAdvanced = ref(false)
const providerDropdownOpen = ref(false)
const providerDropdownRef = ref<HTMLElement | null>(null)

let statusInterval: ReturnType<typeof setInterval> | null = null

// Check if selected provider requires configuration
const selectedProviderInfo = computed(() => {
  return providers.value.find(p => p.id === selectedProvider.value)
})

const requiresNgrokConfig = computed(() => {
  return selectedProvider.value === 'ngrok'
})

async function loadData() {
  try {
    const [statusRes, providersRes, configRes] = await Promise.all([
      getRemoteAccessStatus(),
      getTunnelProviders(),
      getRemoteAccessConfig()
    ])

    tunnelStatus.value = statusRes.data.tunnel
    providers.value = providersRes.data.providers || []
    config.value = configRes.data.config

    // Load saved config values
    if (config.value) {
      selectedProvider.value = config.value.default_provider || 'auto'
      ngrokAuthtoken.value = config.value.ngrok_authtoken || ''
      ngrokDomain.value = config.value.ngrok_domain || ''
    }

    if (statusRes.data.tunnel.connecting) {
      state.value = 'connecting'
      startStatusPolling()
    } else if (statusRes.data.tunnel.active) {
      state.value = 'connected'
    } else {
      state.value = 'ready'
    }
  } catch (e) {
    console.error('Failed to load data:', e)
    error.value = t('remoteAccess.loadError')
    state.value = 'error'
  }
}

async function handleStart() {
  state.value = 'connecting'
  error.value = null

  try {
    // Save config first if ngrok is selected
    if (selectedProvider.value === 'ngrok' && (ngrokAuthtoken.value || ngrokDomain.value)) {
      await updateRemoteAccessConfig({
        ngrok_authtoken: ngrokAuthtoken.value,
        ngrok_domain: ngrokDomain.value,
        default_provider: selectedProvider.value
      })
    }

    const response = await startRemoteAccess(
      selectedProvider.value,
      undefined,
      selectedProvider.value === 'ngrok' ? ngrokAuthtoken.value : undefined,
      undefined,
      selectedProvider.value === 'ngrok' ? ngrokDomain.value : undefined
    )

    if (response.data.success) {
      // Use tunnel status from start response if available
      if (response.data.tunnel) {
        tunnelStatus.value = response.data.tunnel
        if (response.data.tunnel.active && response.data.tunnel.url) {
          state.value = 'connected'
        }
      }

      // Immediately fetch latest status (URL may appear in logs before status API; ensure UI updates)
      try {
        const statusRes = await getRemoteAccessStatus()
        tunnelStatus.value = statusRes.data.tunnel
        if (statusRes.data.tunnel.active || statusRes.data.tunnel.url) {
          state.value = 'connected'
        }
      } catch {
        // Ignore; polling will retry
      }

      // Start polling to get updates (URL may not be ready yet for async providers)
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
  // Faster when connecting (2s), slower when connected (15s)
  const pollInterval = state.value === 'connecting' ? 2000 : 15000

  statusInterval = setInterval(async () => {
    try {
      const response = await getRemoteAccessStatus()
      tunnelStatus.value = response.data.tunnel

      if (response.data.tunnel.active || response.data.tunnel.url) {
        state.value = 'connected'
        // Switch to slower polling when connected
        if (statusInterval) {
          clearInterval(statusInterval)
          statusInterval = setInterval(pollWhenConnected, 15000)
        }
      } else if (state.value === 'connecting') {
        // Still waiting for tunnel to start
      } else {
        state.value = 'ready'
        stopStatusPolling()
      }
    } catch (e) {
      console.error('Failed to poll status:', e)
      // On API error: keep current state (don't flip to ready)
    }
  }, pollInterval)
}

async function pollWhenConnected() {
  try {
    const response = await getRemoteAccessStatus()
    tunnelStatus.value = response.data.tunnel
    // Only transition to ready when both active and url are gone (connection actually stopped)
    if (!response.data.tunnel.active && !response.data.tunnel.url) {
      state.value = 'ready'
      stopStatusPolling()
    }
  } catch (e) {
    console.error('Failed to poll status:', e)
    // On API error: stay connected, keep last known tunnelStatus
  }
}

function stopStatusPolling() {
  if (statusInterval) {
    clearInterval(statusInterval)
    statusInterval = null
  }
}

onMounted(() => {
  loadData()
})

// Close provider dropdown when clicking outside
onClickOutside(providerDropdownRef, () => {
  providerDropdownOpen.value = false
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

        <!-- Provider Selection (custom dropdown so each option shows logo in front) -->
        <div class="space-y-3">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('remoteAccess.provider') }}
          </label>
          <div ref="providerDropdownRef" class="relative">
            <button
              type="button"
              class="w-full flex items-center gap-2 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 text-left"
              @click.stop="providerDropdownOpen = !providerDropdownOpen"
            >
              <img
                v-if="getTunnelProviderIcon(selectedProvider)"
                :src="getTunnelProviderIcon(selectedProvider)!"
                :alt="selectedProvider"
                class="h-5 w-5 shrink-0 rounded object-contain flex-shrink-0"
              />
              <span v-else class="w-5 h-5 shrink-0 block flex-shrink-0" />
              <span class="flex-1 min-w-0 truncate">
                {{ selectedProviderInfo ? selectedProviderInfo.name : selectedProvider }}
                <template v-if="selectedProviderInfo?.requires_key"> ({{ t('remoteAccess.requiresKey') }})</template>
              </span>
              <svg class="h-4 w-4 shrink-0 text-gray-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            <Transition name="dropdown">
              <div
                v-show="providerDropdownOpen"
                class="absolute z-50 mt-1 w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 shadow-lg max-h-56 overflow-auto"
              >
                <button
                  v-for="provider in providers"
                  :key="provider.id"
                  type="button"
                  class="w-full flex items-center gap-2 px-3 py-2 text-left text-sm text-gray-900 dark:text-gray-100 hover:bg-gray-100 dark:hover:bg-gray-700 first:rounded-t-lg last:rounded-b-lg"
                  :class="{ 'bg-blue-50 dark:bg-blue-900/20': selectedProvider === provider.id }"
                  @click.stop="selectedProvider = provider.id; providerDropdownOpen = false"
                >
                  <img
                    v-if="getTunnelProviderIcon(provider.id)"
                    :src="getTunnelProviderIcon(provider.id)!"
                    :alt="provider.id"
                    class="h-5 w-5 shrink-0 rounded object-contain flex-shrink-0"
                  />
                  <span v-else class="w-5 h-5 shrink-0 block flex-shrink-0" aria-hidden="true" />
                  <span>
                    {{ provider.name }}
                    <template v-if="provider.requires_key"> ({{ t('remoteAccess.requiresKey') }})</template>
                  </span>
                </button>
              </div>
            </Transition>
          </div>
          <p v-if="selectedProviderInfo" class="text-xs text-gray-500 dark:text-gray-400">
            {{ selectedProviderInfo.description }}
          </p>
        </div>

        <!-- ngrok Configuration -->
        <div v-if="requiresNgrokConfig" class="space-y-3 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
          <div class="flex items-center gap-2 text-sm font-medium text-blue-800 dark:text-blue-200">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {{ t('remoteAccess.ngrokConfig') }}
          </div>

          <div class="space-y-2">
            <label class="block text-sm text-gray-700 dark:text-gray-300">
              {{ t('remoteAccess.ngrokAuthtoken') }}
              <span class="text-red-500">*</span>
            </label>
            <input
              v-model="ngrokAuthtoken"
              type="password"
              :placeholder="t('remoteAccess.ngrokAuthtokenPlaceholder')"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 text-sm"
            />
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('remoteAccess.ngrokAuthtokenHint') }}
              <a href="https://dashboard.ngrok.com/get-started/your-authtoken" target="_blank" class="text-blue-600 dark:text-blue-400 hover:underline">
                ngrok.com/dashboard
              </a>
            </p>
          </div>

          <div class="space-y-2">
            <label class="block text-sm text-gray-700 dark:text-gray-300">
              {{ t('remoteAccess.ngrokDomain') }}
              <span class="text-gray-400">({{ t('common.optional') }})</span>
            </label>
            <input
              v-model="ngrokDomain"
              type="text"
              :placeholder="t('remoteAccess.ngrokDomainPlaceholder')"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 text-sm"
            />
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('remoteAccess.ngrokDomainHint') }}
            </p>
          </div>
        </div>

        <button
          class="w-full px-4 py-3 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium transition-colors flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="requiresNgrokConfig && !ngrokAuthtoken"
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
        <!-- Show TunnelStatus with connecting state -->
        <TunnelStatus v-if="tunnelStatus" :status="tunnelStatus" />

        <div v-else class="flex items-center justify-center py-8">
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
          @click="loadData"
        >
          {{ t('common.retry') }}
        </button>
      </div>
    </div>
  </div>
</template>
