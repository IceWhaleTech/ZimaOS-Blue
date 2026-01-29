<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { claudeCodeApi } from '@/api/claudecode'
import type { ClaudeCodeVersionResponse, CheckUpdateResponse, ClaudeCodeConfigResponse } from '@/api/claudecode'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

const loading = ref(false)
const checking = ref(false)
const updating = ref(false)
const validating = ref(false)
const clearing = ref(false)
const downloading = ref(false)
const togglingEnabled = ref(false)
const togglingSandbox = ref(false)
const togglingNetwork = ref(false)

const versionInfo = ref<ClaudeCodeVersionResponse | null>(null)
const updateInfo = ref<CheckUpdateResponse | null>(null)
const configInfo = ref<ClaudeCodeConfigResponse | null>(null)
const error = ref<string | null>(null)

// Computed properties
const sourceLabel = computed(() => {
  if (!versionInfo.value) return ''
  switch (versionInfo.value.source) {
    case 'embedded':
      return t('claudecode.sourceEmbedded')
    case 'downloaded':
      return t('claudecode.sourceDownloaded')
    case 'system':
      return t('claudecode.sourceSystem')
    default:
      return versionInfo.value.source
  }
})

const statusColor = computed(() => {
  if (!versionInfo.value) return 'bg-gray-500'
  if (!versionInfo.value.active_version) return 'bg-red-500' // Not installed
  if (versionInfo.value.validated) return 'bg-green-500'
  return 'bg-yellow-500'
})

const statusText = computed(() => {
  if (!versionInfo.value) return t('claudecode.statusUnknown')
  if (!versionInfo.value.active_version) return t('claudecode.statusNotInstalled')
  if (versionInfo.value.validated) return t('claudecode.statusReady')
  return t('claudecode.statusNotValidated')
})

const isInstalled = computed(() => {
  return versionInfo.value?.active_version && versionInfo.value?.binary_path
})

const isEnabled = computed(() => {
  return configInfo.value?.enabled && isInstalled.value
})

onMounted(async () => {
  await Promise.all([loadVersionInfo(), loadConfig()])
})

async function loadConfig() {
  try {
    const response = await claudeCodeApi.getConfig()
    configInfo.value = response.data
  } catch (e) {
    // Config load failure is not critical
    console.error('Failed to load Claude Code config:', e)
  }
}

async function toggleEnabled() {
  if (!configInfo.value) return
  // If not installed, don't allow enabling
  if (!isInstalled.value && !configInfo.value.enabled) {
    emit('status-change', t('claudecode.installFirst'))
    return
  }
  try {
    togglingEnabled.value = true
    error.value = null
    const newEnabled = !configInfo.value.enabled
    const response = await claudeCodeApi.setConfig({ enabled: newEnabled })
    configInfo.value = response.data
    emit('status-change', newEnabled ? t('claudecode.enabled') : t('claudecode.disabled'))
  } catch (e) {
    error.value = t('claudecode.toggleError')
    emit('status-change', t('claudecode.toggleError'))
  } finally {
    togglingEnabled.value = false
  }
}

async function toggleSandbox() {
  if (!configInfo.value) return
  try {
    togglingSandbox.value = true
    error.value = null
    const newSandboxEnabled = !configInfo.value.sandbox_enabled
    const response = await claudeCodeApi.setConfig({ sandbox_enabled: newSandboxEnabled })
    configInfo.value = response.data
    emit('status-change', newSandboxEnabled ? t('claudecode.sandboxEnabled') : t('claudecode.sandboxDisabled'))
  } catch (e) {
    error.value = t('claudecode.toggleError')
    emit('status-change', t('claudecode.toggleError'))
  } finally {
    togglingSandbox.value = false
  }
}

async function toggleNetwork() {
  if (!configInfo.value) return
  try {
    togglingNetwork.value = true
    error.value = null
    const newNetworkEnabled = !configInfo.value.network_enabled
    const response = await claudeCodeApi.setConfig({ network_enabled: newNetworkEnabled })
    configInfo.value = response.data
    emit('status-change', newNetworkEnabled ? t('claudecode.networkEnabled') : t('claudecode.networkDisabled'))
  } catch (e) {
    error.value = t('claudecode.toggleError')
    emit('status-change', t('claudecode.toggleError'))
  } finally {
    togglingNetwork.value = false
  }
}

async function loadVersionInfo() {
  try {
    loading.value = true
    error.value = null
    const response = await claudeCodeApi.getVersion()
    versionInfo.value = response.data
  } catch (e) {
    error.value = t('claudecode.loadError')
    versionInfo.value = null
  } finally {
    loading.value = false
  }
}

async function checkForUpdates() {
  try {
    checking.value = true
    error.value = null
    const response = await claudeCodeApi.checkForUpdates()
    updateInfo.value = response.data
    // Reload version info to get updated last_check
    await loadVersionInfo()
    if (response.data.update_available) {
      emit('status-change', t('claudecode.updateAvailable', { version: response.data.latest_version }))
    } else {
      emit('status-change', t('claudecode.upToDate'))
    }
  } catch (e) {
    error.value = t('claudecode.checkError')
    emit('status-change', t('claudecode.checkError'))
  } finally {
    checking.value = false
  }
}

async function updateCLI() {
  try {
    updating.value = true
    error.value = null
    const response = await claudeCodeApi.update('latest')
    if (response.data.success) {
      emit('status-change', t('claudecode.updateSuccess', { version: response.data.new_version }))
      updateInfo.value = null
      await loadVersionInfo()
    } else {
      error.value = response.data.message
      emit('status-change', response.data.message)
    }
  } catch (e) {
    error.value = t('claudecode.updateError')
    emit('status-change', t('claudecode.updateError'))
  } finally {
    updating.value = false
  }
}

async function validateCLI() {
  try {
    validating.value = true
    error.value = null
    const response = await claudeCodeApi.validate()
    if (response.data.valid) {
      emit('status-change', t('claudecode.validateSuccess', { version: response.data.version }))
      await loadVersionInfo()
    } else {
      error.value = response.data.message
      emit('status-change', response.data.message)
    }
  } catch (e) {
    error.value = t('claudecode.validateError')
    emit('status-change', t('claudecode.validateError'))
  } finally {
    validating.value = false
  }
}

async function clearCache() {
  try {
    clearing.value = true
    error.value = null
    const response = await claudeCodeApi.clearCache()
    if (response.data.success) {
      emit('status-change', t('claudecode.cacheCleared'))
      await loadVersionInfo()
    } else {
      error.value = response.data.message
    }
  } catch (e) {
    error.value = t('claudecode.clearCacheError')
    emit('status-change', t('claudecode.clearCacheError'))
  } finally {
    clearing.value = false
  }
}

async function downloadCLI() {
  try {
    downloading.value = true
    error.value = null
    // Use update with 'latest' to trigger download
    const response = await claudeCodeApi.update('latest')
    if (response.data.success) {
      emit('status-change', t('claudecode.downloadSuccess', { version: response.data.new_version }))
      await loadVersionInfo()
    } else {
      error.value = response.data.message
      emit('status-change', response.data.message)
    }
  } catch (e) {
    error.value = t('claudecode.downloadError')
    emit('status-change', t('claudecode.downloadError'))
  } finally {
    downloading.value = false
  }
}

function formatDate(dateStr?: string) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}
</script>

<template>
  <section class="mb-6 sm:mb-8">
    <!-- Header with Toggle -->
    <div class="flex items-center justify-between mb-3 sm:mb-4">
      <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
        <span class="truncate">{{ t('claudecode.title') }}</span>
      </h2>
      <!-- Main Toggle -->
      <button
        :disabled="togglingEnabled || loading"
        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
        :class="isEnabled ? 'bg-accent' : 'bg-gray-200 dark:bg-slate-600'"
        role="switch"
        :aria-checked="isEnabled"
        @click="toggleEnabled"
      >
        <span
          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
          :class="isEnabled ? 'translate-x-5' : 'translate-x-0'"
        />
      </button>
    </div>

    <div class="glass-card p-4 sm:p-6">
      <!-- Loading state -->
      <div v-if="loading" class="text-gray-500 dark:text-slate-400 text-center py-4">
        {{ t('common.loading') }}
      </div>

      <!-- Error state -->
      <div v-else-if="error && !versionInfo" class="text-center py-4">
        <p class="text-red-500 dark:text-red-400 mb-4">{{ error }}</p>
        <button
          class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors"
          @click="loadVersionInfo"
        >
          {{ t('common.retry') }}
        </button>
      </div>

      <!-- Version Info -->
      <div v-else-if="versionInfo">
        <!-- Disabled Warning Banner -->
        <div v-if="!isEnabled" class="bg-yellow-50 dark:bg-yellow-900/30 border border-yellow-300 dark:border-yellow-600 rounded-lg p-4 mb-4">
          <div class="flex items-start gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-yellow-600 dark:text-yellow-400 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <div class="flex-1">
              <p class="text-yellow-800 dark:text-yellow-200 font-medium">
                {{ t('claudecode.disabledWarningTitle') }}
              </p>
              <p class="text-yellow-700 dark:text-yellow-300 text-sm mt-1">
                {{ t('claudecode.disabledWarningDesc') }}
              </p>
              <ul class="text-yellow-700 dark:text-yellow-300 text-sm mt-2 space-y-1">
                <li class="flex items-center gap-2">
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                  {{ t('claudecode.missingSkills') }}
                </li>
                <li class="flex items-center gap-2">
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                  {{ t('claudecode.missingToolCalling') }}
                </li>
                <li class="flex items-center gap-2">
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                  {{ t('claudecode.missingFileOps') }}
                </li>
              </ul>
            </div>
          </div>
        </div>

        <!-- Not Installed State -->
        <div v-if="!isInstalled" class="text-center py-6">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400 dark:text-slate-500 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
          </svg>
          <p class="text-gray-500 dark:text-slate-400 mb-2">{{ t('claudecode.notInstalledDescription') }}</p>
          <p class="text-sm text-gray-400 dark:text-slate-500 mb-4">{{ t('claudecode.platform') }}: {{ versionInfo.platform }}</p>
          <button
            :disabled="downloading"
            class="px-6 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors disabled:opacity-50 flex items-center gap-2 mx-auto"
            @click="downloadCLI"
          >
            <svg v-if="downloading" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            {{ downloading ? t('claudecode.downloading') : t('claudecode.download') }}
          </button>
          <!-- Error Message -->
          <div v-if="error" class="bg-red-50 dark:bg-red-900/30 border border-red-300 dark:border-red-600 rounded-lg p-3 mt-4 mx-auto max-w-md">
            <p class="text-red-800 dark:text-red-200 text-sm">{{ error }}</p>
          </div>
        </div>

        <!-- Installed State -->
        <template v-else>
          <!-- Status Header -->
          <div class="flex items-center justify-between mb-4">
            <div class="flex items-center gap-3">
              <div :class="['w-3 h-3 rounded-full', statusColor]" />
              <span class="text-gray-900 dark:text-white font-medium">
                {{ statusText }}
              </span>
            </div>
            <span v-if="sourceLabel" class="text-sm px-2 py-1 rounded-full bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-slate-300">
              {{ sourceLabel }}
            </span>
          </div>

          <!-- Sandbox Toggle -->
          <div class="flex items-center justify-between mb-4 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg">
            <div class="flex items-center gap-3">
              <span class="text-gray-900 dark:text-white font-medium">{{ t('claudecode.sandboxMode') }}</span>
              <span class="text-xs text-gray-500 dark:text-slate-400">{{ t('claudecode.sandboxModeDesc') }}</span>
            </div>
            <button
              :disabled="togglingSandbox || !isInstalled"
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
              :class="configInfo?.sandbox_enabled ? 'bg-accent' : 'bg-gray-200 dark:bg-slate-600'"
              role="switch"
              :aria-checked="configInfo?.sandbox_enabled"
              @click="toggleSandbox"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                :class="configInfo?.sandbox_enabled ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>

          <!-- Network Access Toggle (only shown when sandbox is enabled) -->
          <div v-if="configInfo?.sandbox_enabled" class="flex items-center justify-between mb-4 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg ml-4">
            <div class="flex items-center gap-3">
              <span class="text-gray-900 dark:text-white font-medium">{{ t('claudecode.networkAccess') }}</span>
              <span class="text-xs text-gray-500 dark:text-slate-400">{{ t('claudecode.networkAccessDesc') }}</span>
            </div>
            <button
              :disabled="togglingNetwork || !isInstalled"
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
              :class="configInfo?.network_enabled ? 'bg-accent' : 'bg-gray-200 dark:bg-slate-600'"
              role="switch"
              :aria-checked="configInfo?.network_enabled"
              @click="toggleNetwork"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                :class="configInfo?.network_enabled ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>

          <!-- Version Details -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
            <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <p class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('claudecode.activeVersion') }}</p>
              <p class="text-gray-900 dark:text-white font-mono">{{ versionInfo.active_version || '-' }}</p>
            </div>
            <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <p class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('claudecode.platform') }}</p>
              <p class="text-gray-900 dark:text-white font-mono">{{ versionInfo.platform }}</p>
            </div>
            <div v-if="versionInfo.system_version" class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <p class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('claudecode.systemVersion') }}</p>
              <p class="text-gray-900 dark:text-white font-mono">{{ versionInfo.system_version }}</p>
            </div>
            <div v-if="versionInfo.embedded_version" class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <p class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('claudecode.embeddedVersion') }}</p>
              <p class="text-gray-900 dark:text-white font-mono">{{ versionInfo.embedded_version }}</p>
            </div>
          </div>

          <!-- Binary Path -->
          <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3 mb-4">
            <p class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('claudecode.binaryPath') }}</p>
            <p class="text-gray-900 dark:text-white font-mono text-sm break-all">{{ versionInfo.binary_path }}</p>
          </div>

          <!-- Update Available Banner -->
          <div
            v-if="updateInfo?.update_available || versionInfo.update_available"
            class="bg-blue-50 dark:bg-blue-900/30 border border-blue-300 dark:border-blue-600 rounded-lg p-4 mb-4"
          >
            <div class="flex items-center justify-between flex-wrap gap-3">
              <div>
                <p class="text-blue-800 dark:text-blue-200 font-medium">
                  {{ t('claudecode.updateAvailableBanner') }}
                </p>
                <p class="text-blue-600 dark:text-blue-300 text-sm">
                  {{ t('claudecode.latestVersion') }}: {{ updateInfo?.latest_version || t('common.unknown') }}
                </p>
              </div>
              <button
                :disabled="updating"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm transition-colors disabled:opacity-50 flex items-center gap-2"
                @click="updateCLI"
              >
                <svg v-if="updating" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                {{ updating ? t('claudecode.updating') : t('claudecode.updateNow') }}
              </button>
            </div>
          </div>

          <!-- Last Check -->
          <p v-if="versionInfo.last_check" class="text-xs text-gray-500 dark:text-slate-400 mb-4">
            {{ t('claudecode.lastCheck') }}: {{ formatDate(versionInfo.last_check) }}
          </p>

          <!-- Error Message -->
          <div v-if="error" class="bg-red-50 dark:bg-red-900/30 border border-red-300 dark:border-red-600 rounded-lg p-3 mb-4">
            <p class="text-red-800 dark:text-red-200 text-sm">{{ error }}</p>
          </div>

          <!-- Action Buttons -->
          <div class="flex flex-wrap gap-3">
            <button
              :disabled="checking"
              class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors disabled:opacity-50 flex items-center gap-2"
              @click="checkForUpdates"
            >
              <svg v-if="checking" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {{ checking ? t('claudecode.checking') : t('claudecode.checkForUpdates') }}
            </button>

            <button
              :disabled="validating"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors disabled:opacity-50 flex items-center gap-2"
              @click="validateCLI"
            >
              <svg v-if="validating" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {{ validating ? t('claudecode.validating') : t('claudecode.validate') }}
            </button>

            <button
              :disabled="clearing"
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors disabled:opacity-50"
              @click="clearCache"
            >
              {{ clearing ? t('claudecode.clearing') : t('claudecode.clearCache') }}
            </button>
          </div>
        </template>
      </div>
    </div>
  </section>
</template>
