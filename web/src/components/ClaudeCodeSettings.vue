<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { claudeCodeApi } from '@/api/claudecode'
import type { ClaudeCodeVersionResponse, CheckUpdateResponse, ClaudeCodeConfigResponse, DirectoryWhitelistEntry } from '@/api/claudecode'

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
const togglingWhitelist = ref(false)

// Collapsible state - expand when enabled
const isExpanded = ref(false)

const versionInfo = ref<ClaudeCodeVersionResponse | null>(null)
const updateInfo = ref<CheckUpdateResponse | null>(null)
const configInfo = ref<ClaudeCodeConfigResponse | null>(null)
const error = ref<string | null>(null)

// Directory whitelist state
const savingWhitelist = ref(false)
const newDirPath = ref('')
const newDirAlias = ref('')
const editingIndex = ref<number | null>(null)
const editPath = ref('')
const editAlias = ref('')
const isWhitelistExpanded = ref(false)

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

const isWhitelistEnabled = computed(() => {
  return configInfo.value?.whitelist_enabled ?? false
})

// Computed: check if update is available
const hasUpdate = computed(() => {
  if (!versionInfo.value?.active_version) return false
  const latestVer = updateInfo.value?.latest_version || versionInfo.value?.latest_version
  if (!latestVer) return false
  return latestVer !== versionInfo.value.active_version
})

const latestVersionDisplay = computed(() => {
  return updateInfo.value?.latest_version || versionInfo.value?.latest_version || null
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
  } catch (_e) {
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
  } catch (_e) {
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
  } catch (_e) {
    error.value = t('claudecode.toggleError')
    emit('status-change', t('claudecode.toggleError'))
  } finally {
    togglingNetwork.value = false
  }
}

async function toggleWhitelist() {
  if (!configInfo.value) return
  try {
    togglingWhitelist.value = true
    error.value = null
    const newWhitelistEnabled = !configInfo.value.whitelist_enabled
    const response = await claudeCodeApi.setConfig({ whitelist_enabled: newWhitelistEnabled })
    configInfo.value = response.data
    emit('status-change', newWhitelistEnabled ? t('claudecode.whitelistEnabled') : t('claudecode.whitelistDisabled'))
  } catch (_e) {
    error.value = t('claudecode.toggleError')
    emit('status-change', t('claudecode.toggleError'))
  } finally {
    togglingWhitelist.value = false
  }
}

// Directory whitelist functions
async function addDirectory() {
  if (!configInfo.value || !newDirPath.value.trim()) return
  try {
    savingWhitelist.value = true
    error.value = null
    const currentList = configInfo.value.directory_whitelist || []
    const newEntry: DirectoryWhitelistEntry = {
      path: newDirPath.value.trim(),
      alias: newDirAlias.value.trim() || undefined,
    }
    const response = await claudeCodeApi.setConfig({
      directory_whitelist: [...currentList, newEntry],
    })
    configInfo.value = response.data
    newDirPath.value = ''
    newDirAlias.value = ''
    emit('status-change', t('claudecode.directoryAdded'))
  } catch (_e) {
    error.value = t('claudecode.directoryAddError')
    emit('status-change', t('claudecode.directoryAddError'))
  } finally {
    savingWhitelist.value = false
  }
}

async function removeDirectory(index: number) {
  if (!configInfo.value) return
  try {
    savingWhitelist.value = true
    error.value = null
    const currentList = [...(configInfo.value.directory_whitelist || [])]
    currentList.splice(index, 1)
    const response = await claudeCodeApi.setConfig({
      directory_whitelist: currentList,
    })
    configInfo.value = response.data
    emit('status-change', t('claudecode.directoryRemoved'))
  } catch (_e) {
    error.value = t('claudecode.directoryRemoveError')
    emit('status-change', t('claudecode.directoryRemoveError'))
  } finally {
    savingWhitelist.value = false
  }
}

function startEditDirectory(index: number) {
  const entry = configInfo.value?.directory_whitelist?.[index]
  if (!entry) return
  editingIndex.value = index
  editPath.value = entry.path
  editAlias.value = entry.alias || ''
}

function cancelEditDirectory() {
  editingIndex.value = null
  editPath.value = ''
  editAlias.value = ''
}

async function saveEditDirectory() {
  if (!configInfo.value || editingIndex.value === null || !editPath.value.trim()) return
  try {
    savingWhitelist.value = true
    error.value = null
    const currentList = [...(configInfo.value.directory_whitelist || [])]
    currentList[editingIndex.value] = {
      path: editPath.value.trim(),
      alias: editAlias.value.trim() || undefined,
    }
    const response = await claudeCodeApi.setConfig({
      directory_whitelist: currentList,
    })
    configInfo.value = response.data
    cancelEditDirectory()
    emit('status-change', t('claudecode.directoryUpdated'))
  } catch (_e) {
    error.value = t('claudecode.directoryUpdateError')
    emit('status-change', t('claudecode.directoryUpdateError'))
  } finally {
    savingWhitelist.value = false
  }
}

async function loadVersionInfo() {
  try {
    loading.value = true
    error.value = null
    const response = await claudeCodeApi.getVersion()
    versionInfo.value = response.data
  } catch (_e) {
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
  } catch (_e) {
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
  } catch (_e) {
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
  } catch (_e) {
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
  } catch (_e) {
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
  } catch (_e) {
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
    <div class="glass-card">
      <div class="flex items-center justify-between p-4 cursor-pointer" @click="isExpanded = !isExpanded">
        <div class="flex items-center gap-3">
          <!-- Expand/Collapse Arrow -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 text-gray-400 transition-transform duration-200"
            :class="{ 'rotate-90': isExpanded }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 flex-shrink-0 text-gray-600 dark:text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white truncate">{{ t('claudecode.title') }}</h2>
        </div>
        <!-- Main Toggle -->
        <button
          :disabled="togglingEnabled || loading"
          class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          :class="isEnabled ? 'bg-accent' : 'bg-gray-200 dark:bg-slate-600'"
          role="switch"
          :aria-checked="isEnabled ? 'true' : 'false'"
          @click.stop="toggleEnabled"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
            :class="isEnabled ? 'translate-x-5' : 'translate-x-0'"
          />
        </button>
      </div>

      <!-- Collapsible Content -->
      <Transition name="collapse">
        <div v-show="isExpanded" class="border-t border-gray-200 dark:border-slate-600">
          <div class="p-4 sm:p-6">
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

          <!-- Sandbox Toggle with Collapsible Content -->
          <div class="mb-4 bg-gray-50 dark:bg-slate-700/50 rounded-lg overflow-hidden">
            <div class="flex items-center justify-between p-3">
              <div class="flex items-center gap-3">
                <span class="text-gray-900 dark:text-white font-medium">{{ t('claudecode.sandboxMode') }}</span>
                <span class="text-xs text-gray-500 dark:text-slate-400">{{ t('claudecode.sandboxModeDesc') }}</span>
              </div>
              <button
                :disabled="togglingSandbox || !isInstalled"
                class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
                :class="configInfo?.sandbox_enabled ? 'bg-accent' : 'bg-gray-200 dark:bg-slate-600'"
                role="switch"
                :aria-checked="configInfo?.sandbox_enabled ? 'true' : 'false'"
                @click="toggleSandbox"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                  :class="configInfo?.sandbox_enabled ? 'translate-x-5' : 'translate-x-0'"
                />
              </button>
            </div>

            <!-- Sandbox nested content (only shown when sandbox is enabled) -->
            <Transition name="collapse">
              <div v-if="configInfo?.sandbox_enabled" class="border-t border-gray-200 dark:border-slate-600 p-3 space-y-3">
                <!-- Network Access Toggle -->
                <div class="flex items-center justify-between p-3 bg-white dark:bg-slate-800 rounded-lg">
                  <div class="flex items-center gap-3">
                    <span class="text-gray-900 dark:text-white font-medium">{{ t('claudecode.networkAccess') }}</span>
                    <span class="text-xs text-gray-500 dark:text-slate-400">{{ t('claudecode.networkAccessDesc') }}</span>
                  </div>
                  <button
                    :disabled="togglingNetwork || !isInstalled"
                    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
                    :class="configInfo?.network_enabled ? 'bg-accent' : 'bg-gray-200 dark:bg-slate-600'"
                    role="switch"
                    :aria-checked="configInfo?.network_enabled ? 'true' : 'false'"
                    @click="toggleNetwork"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="configInfo?.network_enabled ? 'translate-x-5' : 'translate-x-0'"
                    />
                  </button>
                </div>

                <!-- Directory Whitelist with Toggle -->
                <div class="bg-white dark:bg-slate-800 rounded-lg overflow-hidden">
                  <div class="flex items-center justify-between p-3 cursor-pointer" @click="isWhitelistExpanded = !isWhitelistExpanded">
                    <div class="flex items-center gap-3">
                      <!-- Expand/Collapse Arrow -->
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        class="h-4 w-4 text-gray-400 transition-transform duration-200"
                        :class="{ 'rotate-90': isWhitelistExpanded }"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                      </svg>
                      <span class="text-gray-900 dark:text-white font-medium">{{ t('claudecode.directoryWhitelist') }}</span>
                      <span class="text-xs text-gray-500 dark:text-slate-400">{{ t('claudecode.directoryWhitelistDesc') }}</span>
                    </div>
                    <button
                      :disabled="togglingWhitelist || !isInstalled"
                      class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-cta focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
                      :class="isWhitelistEnabled ? 'bg-cta' : 'bg-gray-200 dark:bg-slate-600'"
                      role="switch"
                      :aria-checked="isWhitelistEnabled ? 'true' : 'false'"
                      @click.stop="toggleWhitelist"
                    >
                      <span
                        class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                        :class="isWhitelistEnabled ? 'translate-x-5' : 'translate-x-0'"
                      />
                    </button>
                  </div>

                  <!-- Directory Whitelist Content (shown when expanded) -->
                  <Transition name="collapse">
                    <div v-if="isWhitelistExpanded" class="border-t border-gray-200 dark:border-slate-600 p-3">
                      <!-- Existing directories list -->
                      <div v-if="configInfo?.directory_whitelist?.length" class="space-y-2 mb-3">
                        <div
                          v-for="(entry, index) in configInfo.directory_whitelist"
                          :key="index"
                          class="flex items-center gap-2 p-2 bg-gray-50 dark:bg-slate-700 rounded-lg border border-gray-200 dark:border-slate-600"
                        >
                          <!-- View mode -->
                          <template v-if="editingIndex !== index">
                            <div class="flex-1 min-w-0">
                              <div class="flex items-center gap-2">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                                </svg>
                                <span v-if="entry.alias" class="text-sm font-medium text-gray-900 dark:text-white">{{ entry.alias }}</span>
                                <span class="text-sm text-gray-600 dark:text-slate-300 font-mono truncate" :class="{ 'text-gray-400 dark:text-slate-500': entry.alias }">{{ entry.path }}</span>
                              </div>
                            </div>
                            <button
                              class="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-slate-200 transition-colors"
                              :title="t('common.edit')"
                              @click="startEditDirectory(index)"
                            >
                              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                              </svg>
                            </button>
                            <button
                              :disabled="savingWhitelist"
                              class="p-1.5 text-red-400 hover:text-red-600 dark:hover:text-red-300 transition-colors disabled:opacity-50"
                              :title="t('common.delete')"
                              @click="removeDirectory(index)"
                            >
                              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                              </svg>
                            </button>
                          </template>

                          <!-- Edit mode -->
                          <template v-else>
                            <div class="flex-1 flex flex-col sm:flex-row gap-2">
                              <input
                                v-model="editPath"
                                type="text"
                                class="flex-1 px-2 py-1 text-sm border border-gray-300 dark:border-slate-500 rounded bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-1 focus:ring-accent focus:border-accent"
                                :placeholder="t('claudecode.directoryPathPlaceholder')"
                              />
                              <input
                                v-model="editAlias"
                                type="text"
                                class="sm:w-32 px-2 py-1 text-sm border border-gray-300 dark:border-slate-500 rounded bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-1 focus:ring-accent focus:border-accent"
                                :placeholder="t('claudecode.directoryAliasPlaceholder')"
                              />
                            </div>
                            <button
                              :disabled="savingWhitelist || !editPath.trim()"
                              class="p-1.5 text-green-500 hover:text-green-600 dark:hover:text-green-400 transition-colors disabled:opacity-50"
                              :title="t('common.save')"
                              @click="saveEditDirectory"
                            >
                              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                              </svg>
                            </button>
                            <button
                              class="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-slate-200 transition-colors"
                              :title="t('common.cancel')"
                              @click="cancelEditDirectory"
                            >
                              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                              </svg>
                            </button>
                          </template>
                        </div>
                      </div>

                      <!-- Empty state -->
                      <div v-else class="text-center py-4 text-gray-500 dark:text-slate-400 text-sm">
                        {{ t('claudecode.noDirectoriesWhitelisted') }}
                      </div>

                      <!-- Add new directory form -->
                      <div class="flex flex-col sm:flex-row gap-2 pt-3 border-t border-gray-200 dark:border-slate-600">
                        <input
                          v-model="newDirPath"
                          type="text"
                          class="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-slate-500 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-accent focus:border-accent"
                          :placeholder="t('claudecode.directoryPathPlaceholder')"
                          @keyup.enter="addDirectory"
                        />
                        <input
                          v-model="newDirAlias"
                          type="text"
                          class="sm:w-32 px-3 py-2 text-sm border border-gray-300 dark:border-slate-500 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-accent focus:border-accent"
                          :placeholder="t('claudecode.directoryAliasPlaceholder')"
                          @keyup.enter="addDirectory"
                        />
                        <button
                          :disabled="savingWhitelist || !newDirPath.trim()"
                          class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
                          @click="addDirectory"
                        >
                          <svg v-if="savingWhitelist" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                          </svg>
                          <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                          </svg>
                          {{ t('common.add') }}
                        </button>
                      </div>
                    </div>
                  </Transition>
                </div>
              </div>
            </Transition>
          </div>

          <!-- Version Details - Simplified -->
          <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3 mb-4">
            <p class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('claudecode.version') }}</p>
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-gray-900 dark:text-white font-mono">{{ versionInfo.active_version || '-' }}</span>
              <a
                v-if="hasUpdate && latestVersionDisplay"
                href="https://docs.anthropic.com/en/docs/claude-code"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-500 hover:text-blue-600 dark:text-blue-400 dark:hover:text-blue-300 text-sm font-mono transition-colors"
                :title="t('claudecode.viewDocs')"
              >
                ({{ latestVersionDisplay }} {{ t('claudecode.available') }})
              </a>
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
        </div>
      </Transition>
    </div>
  </section>
</template>

<style scoped>
.glass-card {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(8px);
  border-radius: 0.75rem;
  border: 1px solid rgb(229, 231, 235);
}

:root.dark .glass-card {
  background: rgba(30, 41, 59, 0.8);
  border-color: rgb(51, 65, 85);
}

/* Collapse transition */
.collapse-enter-active,
.collapse-leave-active {
  transition: all 0.3s ease;
  overflow: hidden;
}

.collapse-enter-from,
.collapse-leave-to {
  opacity: 0;
  max-height: 0;
}

.collapse-enter-to,
.collapse-leave-from {
  opacity: 1;
  max-height: 2000px;
}
</style>
