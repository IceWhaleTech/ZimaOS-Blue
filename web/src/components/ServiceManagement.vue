<script setup lang="ts">
import { ref, onMounted, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { serviceApi } from '@/api/service'
import { systemApi } from '@/api/system'
import type { ServiceInfo } from '@/api/service'

const { t } = useI18n()

// State
const serviceInfo = ref<ServiceInfo | null>(null)
const loading = ref(false)
const actionLoading = ref<string | null>(null)
const error = ref<string | null>(null)
const successMessage = ref<string | null>(null)

// Port configuration state
interface ServerConfig {
  host: string
  port: number
  actual_port: number
  port_auto_fallback: boolean
}
const serverConfig = ref<ServerConfig | null>(null)
const portInput = ref('')
const portEditing = ref(false)

// Port change confirmation state
const portChangeConfirm = ref(false)
const portChangeCountdown = ref(0)
const previousPort = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

// Computed - auto-start is ON when service is both installed and enabled
const autoStartEnabled = computed(() => {
  return serviceInfo.value?.installed && serviceInfo.value?.enabled
})

const portChanged = computed(() => {
  if (!serverConfig.value) return false
  return serverConfig.value.port !== serverConfig.value.actual_port
})

// Methods
async function fetchServiceInfo() {
  loading.value = true
  error.value = null
  try {
    const [serviceResponse, configResponse] = await Promise.all([
      serviceApi.getInfo(),
      systemApi.getConfig(),
    ])
    serviceInfo.value = serviceResponse.data
    const config = configResponse.data as { server?: ServerConfig }
    if (config.server) {
      serverConfig.value = config.server
      portInput.value = String(config.server.port)
    }
  } catch (e) {
    error.value = t('service.fetchFailed') + (e instanceof Error ? `: ${e.message}` : '')
  } finally {
    loading.value = false
  }
}

async function toggleAutoStart() {
  if (autoStartEnabled.value) {
    // Disable: disable auto-start, then uninstall
    actionLoading.value = 'autoStart'
    error.value = null
    successMessage.value = null
    try {
      const disableRes = (await serviceApi.disable()) as { data: { success: boolean; message: string } }
      if (!disableRes.data.success) {
        error.value = t('service.disableFailed') + (disableRes.data.message ? `: ${disableRes.data.message}` : '')
        return
      }
      const uninstallRes = (await serviceApi.uninstall()) as { data: { success: boolean; message: string } }
      if (!uninstallRes.data.success) {
        error.value = t('service.uninstallFailed') + (uninstallRes.data.message ? `: ${uninstallRes.data.message}` : '')
        return
      }
      successMessage.value = t('service.disableSuccess')
      await fetchServiceInfo()
    } catch (e) {
      error.value = t('service.disableFailed') + (e instanceof Error ? `: ${e.message}` : '')
    } finally {
      actionLoading.value = null
      if (successMessage.value) {
        setTimeout(() => { successMessage.value = null }, 3000)
      }
    }
  } else {
    // Enable: install service, then enable auto-start
    actionLoading.value = 'autoStart'
    error.value = null
    successMessage.value = null
    try {
      // Install if not already installed
      if (!serviceInfo.value?.installed) {
        const installRes = (await serviceApi.install()) as { data: { success: boolean; message: string } }
        if (!installRes.data.success) {
          error.value = t('service.installFailed') + (installRes.data.message ? `: ${installRes.data.message}` : '')
          return
        }
      }
      const enableRes = (await serviceApi.enable()) as { data: { success: boolean; message: string } }
      if (!enableRes.data.success) {
        error.value = t('service.enableFailed') + (enableRes.data.message ? `: ${enableRes.data.message}` : '')
        return
      }
      successMessage.value = t('service.enableSuccess')
      await fetchServiceInfo()
    } catch (e) {
      error.value = t('service.enableFailed') + (e instanceof Error ? `: ${e.message}` : '')
    } finally {
      actionLoading.value = null
      if (successMessage.value) {
        setTimeout(() => { successMessage.value = null }, 3000)
      }
    }
  }
}

function startEditPort() {
  if (serverConfig.value) {
    portInput.value = String(serverConfig.value.port)
    portEditing.value = true
  }
}

function cancelEditPort() {
  if (serverConfig.value) {
    portInput.value = String(serverConfig.value.port)
  }
  portEditing.value = false
}

function validatePort(value: string): boolean {
  const port = parseInt(value, 10)
  return !isNaN(port) && port >= 1 && port <= 65535
}

async function savePort() {
  if (!validatePort(portInput.value)) {
    error.value = t('service.invalidPort')
    return
  }

  const newPort = parseInt(portInput.value, 10)
  const currentPort = serverConfig.value?.actual_port || serverConfig.value?.port || 23456

  if (newPort === currentPort) {
    portEditing.value = false
    return
  }

  actionLoading.value = 'savePort'
  error.value = null
  successMessage.value = null

  // Save previous port for potential revert
  previousPort.value = currentPort

  try {
    const response = await systemApi.updateConfig({
      server: {
        port: newPort
      }
    })

    if (response.data.success) {
      portEditing.value = false

      // Store port change info in localStorage for the new page
      localStorage.setItem('portChangeInfo', JSON.stringify({
        previousPort: currentPort,
        newPort: newPort,
        timestamp: Date.now()
      }))

      // Restart service to apply new port
      await systemApi.restartService()

      // Wait for service to restart, then redirect to new port
      setTimeout(() => {
        const currentUrl = new URL(window.location.href)
        currentUrl.port = String(newPort)
        window.location.href = currentUrl.toString()
      }, 2000)
    } else {
      error.value = t('service.portSaveFailed') + (response.data.message ? `: ${response.data.message}` : '')
    }
  } catch (e) {
    error.value = t('service.portSaveFailed') + (e instanceof Error ? `: ${e.message}` : '')
  } finally {
    actionLoading.value = null
  }
}

function checkPortChangeConfirmation() {
  const portChangeInfoStr = localStorage.getItem('portChangeInfo')
  if (!portChangeInfoStr) return

  try {
    const portChangeInfo = JSON.parse(portChangeInfoStr)
    const elapsed = Date.now() - portChangeInfo.timestamp

    // Only show confirmation if within 30 seconds of port change
    if (elapsed < 30000) {
      previousPort.value = portChangeInfo.previousPort
      portChangeConfirm.value = true
      portChangeCountdown.value = Math.max(1, Math.floor((30000 - elapsed) / 1000))

      // Start countdown timer
      countdownTimer = setInterval(() => {
        portChangeCountdown.value--
        if (portChangeCountdown.value <= 0) {
          // Auto-revert if not confirmed
          revertPort()
        }
      }, 1000)
    } else {
      // Expired, clean up
      localStorage.removeItem('portChangeInfo')
    }
  } catch {
    localStorage.removeItem('portChangeInfo')
  }
}

function confirmPortChange() {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  portChangeConfirm.value = false
  localStorage.removeItem('portChangeInfo')
  successMessage.value = t('service.portChangeConfirmed')
  setTimeout(() => {
    successMessage.value = null
  }, 3000)
}

async function revertPort() {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  portChangeConfirm.value = false

  const portChangeInfoStr = localStorage.getItem('portChangeInfo')
  let prevPort = previousPort.value

  if (portChangeInfoStr) {
    try {
      const portChangeInfo = JSON.parse(portChangeInfoStr)
      prevPort = portChangeInfo.previousPort
    } catch {
      // Use previousPort.value
    }
  }

  localStorage.removeItem('portChangeInfo')

  try {
    // Revert to previous port
    await systemApi.updateConfig({
      server: {
        port: prevPort
      }
    })
    await systemApi.restartService()

    // Redirect back to previous port
    setTimeout(() => {
      const currentUrl = new URL(window.location.href)
      currentUrl.port = String(prevPort)
      window.location.href = currentUrl.toString()
    }, 2000)
  } catch {
    error.value = t('service.portRevertFailed')
  }
}

onMounted(() => {
  fetchServiceInfo()
  // Check if we need to show port change confirmation
  checkPortChangeConfirmation()
})

onUnmounted(() => {
  if (countdownTimer) {
    clearInterval(countdownTimer)
  }
})
</script>

<template>
  <div class="service-management">
    <!-- Header -->
    <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4">
      {{ t('service.title') }}
    </h3>

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

    <!-- Port Change Confirmation Dialog -->
    <div
      v-if="portChangeConfirm"
      class="mb-4 p-4 bg-gray-700 dark:bg-gray-500/30 border border-gray-900 dark:border-white dark:border-gray-900 dark:border-white rounded-lg"
    >
      <div class="flex items-center justify-between">
        <div>
          <div class="font-medium text-gray-900 dark:text-white dark:text-white">
            {{ t('service.portChangeConfirmTitle') }}
          </div>
          <div class="text-sm text-gray-900 dark:text-white dark:text-white mt-1">
            {{ t('service.portChangeConfirmDesc', { seconds: portChangeCountdown }) }}
          </div>
        </div>
        <div class="flex items-center gap-3">
          <div class="text-2xl font-bold text-gray-900 dark:text-white dark:text-white w-10 text-center">
            {{ portChangeCountdown }}
          </div>
          <button
            class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded-lg text-sm transition-colors"
            @click="confirmPortChange"
          >
            {{ t('service.confirmKeep') }}
          </button>
          <button
            class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg text-sm transition-colors"
            @click="revertPort"
          >
            {{ t('service.revertNow') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading && !serviceInfo" class="text-center py-8 text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>

    <!-- Service Info -->
    <div v-else-if="serviceInfo" class="space-y-4">
      <!-- Port Configuration -->
      <div class="bg-gray-100 dark:bg-gray-700/30 rounded-lg p-4">
        <div class="flex items-center justify-between">
          <div>
            <div class="font-medium text-gray-900 dark:text-white">{{ t('service.port') }}</div>
            <div class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('service.portDescription') }}
            </div>
          </div>
          <div class="flex items-center gap-3">
            <template v-if="!portEditing">
              <div class="text-right">
                <div class="font-mono text-lg text-gray-900 dark:text-white">
                  {{ serverConfig?.actual_port || serverConfig?.port || '-' }}
                </div>
                <div
                  v-if="portChanged"
                  class="text-xs text-yellow-600 dark:text-yellow-400"
                >
                  {{ t('service.configuredPort') }}: {{ serverConfig?.port }}
                </div>
              </div>
              <button
                class="px-3 py-1.5 text-sm bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg transition-colors"
                @click="startEditPort"
              >
                {{ t('common.edit') }}
              </button>
            </template>
            <template v-else>
              <input
                v-model="portInput"
                type="number"
                min="1"
                max="65535"
                class="w-24 px-3 py-1.5 text-sm font-mono bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                :class="{ 'border-red-500': !validatePort(portInput) }"
                @keyup.enter="savePort"
                @keyup.escape="cancelEditPort"
              />
              <button
                class="px-3 py-1.5 text-sm bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors flex items-center gap-1"
                :disabled="!validatePort(portInput) || actionLoading === 'savePort'"
                @click="savePort"
              >
                <svg
                  v-if="actionLoading === 'savePort'"
                  class="animate-spin h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                {{ t('common.save') }}
              </button>
              <button
                class="px-3 py-1.5 text-sm bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg transition-colors"
                @click="cancelEditPort"
              >
                {{ t('common.cancel') }}
              </button>
            </template>
          </div>
        </div>
        <!-- Port auto-fallback info -->
        <div
          v-if="serverConfig?.port_auto_fallback && portChanged"
          class="mt-3 p-2 bg-yellow-50 dark:bg-yellow-900/20 rounded text-xs text-yellow-700 dark:text-yellow-300"
        >
          {{ t('service.portAutoFallbackInfo') }}
        </div>
        <!-- Port edit hint -->
        <div v-if="portEditing" class="mt-3 text-xs text-gray-500 dark:text-gray-400">
          {{ t('service.portEditHint') }}
        </div>
      </div>

      <!-- Auto-start Toggle -->
      <div class="bg-gray-100 dark:bg-gray-700/30 rounded-lg p-4">
        <div class="flex items-center justify-between">
          <div>
            <div class="font-medium text-gray-900 dark:text-white">{{ t('service.autoStart') }}</div>
            <div class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('service.autoStartDescription') }}
            </div>
          </div>
          <button
            class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:ring-offset-2"
            :class="autoStartEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
            :disabled="actionLoading !== null"
            @click="toggleAutoStart"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="autoStartEnabled ? 'translate-x-5' : 'translate-x-0'"
            ></span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
