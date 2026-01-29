<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { serviceApi } from '@/api/service'
import type { ServiceInfo, ServiceStatus } from '@/api/service'

const { t } = useI18n()

// State
const serviceInfo = ref<ServiceInfo | null>(null)
const loading = ref(false)
const actionLoading = ref<string | null>(null)
const error = ref<string | null>(null)
const successMessage = ref<string | null>(null)

// Computed
const platformIcon = computed(() => {
  switch (serviceInfo.value?.platform) {
    case 'windows':
      return 'windows'
    case 'darwin':
      return 'apple'
    case 'linux':
      return 'linux'
    default:
      return 'server'
  }
})

const platformName = computed(() => {
  switch (serviceInfo.value?.platform) {
    case 'windows':
      return 'Windows'
    case 'darwin':
      return 'macOS'
    case 'linux':
      return 'Linux'
    default:
      return serviceInfo.value?.platform || 'Unknown'
  }
})

const serviceTypeName = computed(() => {
  switch (serviceInfo.value?.service_type) {
    case 'windows_service':
      return t('service.windowsService')
    case 'launchd':
      return t('service.launchd')
    case 'systemd':
      return t('service.systemd')
    default:
      return serviceInfo.value?.service_type || 'Unknown'
  }
})

const statusColor = computed(() => {
  if (!serviceInfo.value) return 'gray'
  if (serviceInfo.value.running) return 'green'
  if (serviceInfo.value.installed) return 'yellow'
  return 'gray'
})

const statusText = computed(() => {
  if (!serviceInfo.value) return t('service.unknown')
  if (serviceInfo.value.running) return t('service.running')
  if (serviceInfo.value.installed) return t('service.stopped')
  return t('service.notInstalled')
})

// Methods
async function fetchServiceInfo() {
  loading.value = true
  error.value = null
  try {
    const response = await serviceApi.getInfo()
    serviceInfo.value = response.data
  } catch (e) {
    error.value = t('service.fetchFailed') + (e instanceof Error ? `: ${e.message}` : '')
  } finally {
    loading.value = false
  }
}

async function performAction(action: string, apiCall: () => Promise<unknown>) {
  actionLoading.value = action
  error.value = null
  successMessage.value = null
  try {
    const response = await apiCall() as { data: { success: boolean; message: string } }
    if (response.data.success) {
      // Use translated success message
      successMessage.value = t(`service.${action}Success`)
      // Refresh service info after action
      await fetchServiceInfo()
    } else {
      // Use translated error message with fallback to backend message
      error.value = t(`service.${action}Failed`) + (response.data.message ? `: ${response.data.message}` : '')
    }
  } catch (e) {
    error.value = t(`service.${action}Failed`) + (e instanceof Error ? `: ${e.message}` : '')
  } finally {
    actionLoading.value = null
    // Clear success message after 3 seconds
    if (successMessage.value) {
      setTimeout(() => {
        successMessage.value = null
      }, 3000)
    }
  }
}

async function installService() {
  if (!confirm(t('service.confirmInstall'))) return
  await performAction('install', serviceApi.install)
}

async function uninstallService() {
  if (!confirm(t('service.confirmUninstall'))) return
  await performAction('uninstall', serviceApi.uninstall)
}

async function startService() {
  await performAction('start', serviceApi.start)
}

async function stopService() {
  if (!confirm(t('service.confirmStop'))) return
  await performAction('stop', serviceApi.stop)
}

async function restartService() {
  await performAction('restart', serviceApi.restart)
}

async function enableService() {
  await performAction('enable', serviceApi.enable)
}

async function disableService() {
  if (!confirm(t('service.confirmDisable'))) return
  await performAction('disable', serviceApi.disable)
}

onMounted(() => {
  fetchServiceInfo()
})
</script>

<template>
  <div class="service-management">
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('service.title') }}
      </h3>
      <button
        class="px-3 py-1.5 text-sm bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg transition-colors"
        :disabled="loading"
        @click="fetchServiceInfo"
      >
        {{ loading ? t('common.loading') : t('common.refresh') }}
      </button>
    </div>

    <!-- Error Message -->
    <div
      v-if="error"
      class="mb-4 p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-300 text-sm"
    >
      {{ error }}
    </div>

    <!-- Success Message -->
    <div
      v-if="successMessage"
      class="mb-4 p-3 bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-800 rounded-lg text-green-700 dark:text-green-300 text-sm"
    >
      {{ successMessage }}
    </div>

    <!-- Loading State -->
    <div v-if="loading && !serviceInfo" class="text-center py-8 text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>

    <!-- Service Info -->
    <div v-else-if="serviceInfo" class="space-y-4">
      <!-- Status Card -->
      <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <!-- Platform Icon -->
            <div class="w-12 h-12 rounded-lg bg-gray-200 dark:bg-gray-600 flex items-center justify-center">
              <svg v-if="platformIcon === 'windows'" class="w-6 h-6 text-blue-500" viewBox="0 0 24 24" fill="currentColor">
                <path d="M0 3.449L9.75 2.1v9.451H0m10.949-9.602L24 0v11.4H10.949M0 12.6h9.75v9.451L0 20.699M10.949 12.6H24V24l-12.9-1.801"/>
              </svg>
              <svg v-else-if="platformIcon === 'apple'" class="w-6 h-6 text-gray-700 dark:text-gray-300" viewBox="0 0 24 24" fill="currentColor">
                <path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.81-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z"/>
              </svg>
              <svg v-else-if="platformIcon === 'linux'" class="w-6 h-6 text-yellow-500" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12.504 0c-.155 0-.311.001-.465.003-.653.014-1.283.07-1.879.18-.596.11-1.158.27-1.68.48-.522.21-.998.47-1.426.78-.428.31-.806.67-1.134 1.08-.328.41-.604.87-.828 1.38-.224.51-.394 1.07-.51 1.68-.116.61-.174 1.27-.174 1.98 0 .71.058 1.37.174 1.98.116.61.286 1.17.51 1.68.224.51.5.97.828 1.38.328.41.706.77 1.134 1.08.428.31.904.57 1.426.78.522.21 1.084.37 1.68.48.596.11 1.226.166 1.879.18.154.002.31.003.465.003.155 0 .311-.001.465-.003.653-.014 1.283-.07 1.879-.18.596-.11 1.158-.27 1.68-.48.522-.21.998-.47 1.426-.78.428-.31.806-.67 1.134-1.08.328-.41.604-.87.828-1.38.224-.51.394-1.07.51-1.68.116-.61.174-1.27.174-1.98 0-.71-.058-1.37-.174-1.98-.116-.61-.286-1.17-.51-1.68-.224-.51-.5-.97-.828-1.38-.328-.41-.706-.77-1.134-1.08-.428-.31-.904-.57-1.426-.78-.522-.21-1.084-.37-1.68-.48-.596-.11-1.226-.166-1.879-.18-.154-.002-.31-.003-.465-.003z"/>
              </svg>
              <svg v-else class="w-6 h-6 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/>
              </svg>
            </div>
            <div>
              <div class="font-medium text-gray-900 dark:text-white">{{ platformName }}</div>
              <div class="text-sm text-gray-500 dark:text-gray-400">{{ serviceTypeName }}</div>
            </div>
          </div>
          <!-- Status Badge -->
          <div class="flex items-center gap-2">
            <span
              class="w-3 h-3 rounded-full"
              :class="{
                'bg-green-500': statusColor === 'green',
                'bg-yellow-500': statusColor === 'yellow',
                'bg-gray-400': statusColor === 'gray',
              }"
            ></span>
            <span
              class="text-sm font-medium"
              :class="{
                'text-green-600 dark:text-green-400': statusColor === 'green',
                'text-yellow-600 dark:text-yellow-400': statusColor === 'yellow',
                'text-gray-500 dark:text-gray-400': statusColor === 'gray',
              }"
            >
              {{ statusText }}
            </span>
          </div>
        </div>
      </div>

      <!-- Service Details -->
      <div class="grid grid-cols-2 gap-4 text-sm">
        <div>
          <span class="text-gray-500 dark:text-gray-400">{{ t('service.serviceName') }}:</span>
          <span class="text-gray-900 dark:text-white ml-2">{{ serviceInfo.service_name }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">{{ t('service.startType') }}:</span>
          <span class="text-gray-900 dark:text-white ml-2">
            {{ serviceInfo.start_type === 'automatic' ? t('service.automatic') : t('service.manual') }}
          </span>
        </div>
        <div v-if="serviceInfo.install_path" class="col-span-2">
          <span class="text-gray-500 dark:text-gray-400">{{ t('service.installPath') }}:</span>
          <span class="text-gray-900 dark:text-white ml-2 font-mono text-xs">{{ serviceInfo.install_path }}</span>
        </div>
        <div v-if="serviceInfo.config_path" class="col-span-2">
          <span class="text-gray-500 dark:text-gray-400">{{ t('service.configPath') }}:</span>
          <span class="text-gray-900 dark:text-white ml-2 font-mono text-xs">{{ serviceInfo.config_path }}</span>
        </div>
      </div>

      <!-- Auto-start Toggle -->
      <div class="flex items-center justify-between py-3 border-t border-gray-200 dark:border-gray-700">
        <div>
          <div class="font-medium text-gray-900 dark:text-white">{{ t('service.autoStart') }}</div>
          <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('service.autoStartDescription') }}</div>
        </div>
        <button
          v-if="serviceInfo.installed"
          class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          :class="serviceInfo.enabled ? 'bg-blue-600' : 'bg-gray-200 dark:bg-gray-600'"
          :disabled="actionLoading !== null"
          @click="serviceInfo.enabled ? disableService() : enableService()"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
            :class="serviceInfo.enabled ? 'translate-x-5' : 'translate-x-0'"
          ></span>
        </button>
        <span v-else class="text-sm text-gray-400">{{ t('service.installFirst') }}</span>
      </div>

      <!-- Action Buttons -->
      <div class="flex flex-wrap gap-2 pt-4 border-t border-gray-200 dark:border-gray-700">
        <!-- Install/Uninstall -->
        <template v-if="!serviceInfo.installed">
          <button
            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
            :disabled="actionLoading !== null"
            @click="installService"
          >
            <svg v-if="actionLoading === 'install'" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ t('service.install') }}
          </button>
        </template>
        <template v-else>
          <!-- Start/Stop/Restart -->
          <button
            v-if="!serviceInfo.running"
            class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
            :disabled="actionLoading !== null"
            @click="startService"
          >
            <svg v-if="actionLoading === 'start'" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ t('service.start') }}
          </button>
          <button
            v-if="serviceInfo.running"
            class="px-4 py-2 bg-yellow-600 hover:bg-yellow-700 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
            :disabled="actionLoading !== null"
            @click="stopService"
          >
            <svg v-if="actionLoading === 'stop'" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ t('service.stop') }}
          </button>
          <button
            v-if="serviceInfo.running"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
            :disabled="actionLoading !== null"
            @click="restartService"
          >
            <svg v-if="actionLoading === 'restart'" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ t('service.restart') }}
          </button>
          <!-- Uninstall -->
          <button
            class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
            :disabled="actionLoading !== null"
            @click="uninstallService"
          >
            <svg v-if="actionLoading === 'uninstall'" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ t('service.uninstall') }}
          </button>
        </template>
      </div>

      <!-- Platform-specific Help -->
      <div class="mt-4 p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg text-sm text-blue-700 dark:text-blue-300">
        <div class="font-medium mb-1">{{ t('service.platformHelp') }}</div>
        <div v-if="serviceInfo.platform === 'windows'" class="text-xs space-y-1">
          <p>{{ t('service.windowsHelp1') }}</p>
          <p>{{ t('service.windowsHelp2') }}</p>
        </div>
        <div v-else-if="serviceInfo.platform === 'darwin'" class="text-xs space-y-1">
          <p>{{ t('service.macosHelp1') }}</p>
          <p>{{ t('service.macosHelp2') }}</p>
        </div>
        <div v-else-if="serviceInfo.platform === 'linux'" class="text-xs space-y-1">
          <p>{{ t('service.linuxHelp1') }}</p>
          <p>{{ t('service.linuxHelp2') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
