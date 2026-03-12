<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { useChatStore } from '@/stores/chat'
import { userDataApi, type UserSettings, type ImportPreview } from '@/api/userdata'
import { companionSettingsApi, type StorageInfo } from '@/api/companion'
import { usePreviewStore } from '@/stores/preview'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()
const chatStore = useChatStore()
const previewStore = usePreviewStore()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

// Tab state
const activeTab = ref<'export' | 'import' | 'cleanup'>('export')
const showDataModal = ref(false)

// Export state
const exportFormat = ref<'json' | 'encrypted'>('json')
const exportPassword = ref('')
const exportConfirmPassword = ref('')
const exporting = ref(false)
const exportError = ref<string | null>(null)

// Import state
const importPassword = ref('')
const importing = ref(false)
const importError = ref<string | null>(null)
const importPreview = ref<ImportPreview | null>(null)
const importFile = ref<File | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)

// Data Cleanup state
const storageInfo = ref<StorageInfo>({
  session_count: 0,
  alert_count: 0,
  event_count: 0,
})

// Cleanup state
const cleanupStep = ref<'select' | 'preview' | 'confirm'>('select')
const cleanupTargets = ref({
  chatHistory: false,
  sessions: false,
  events: false,
  alerts: false,
  settings: false,
  cache: false,
})
const cleanupPassword = ref('')
const cleanupConfirmText = ref('')
const cleanupLoading = ref(false)
const cleanupError = ref<string | null>(null)
const cleanupPreviewData = ref<{
  chatHistory: number
  sessions: number
  events: number
  alerts: number
  settings: boolean
  cache: boolean
} | null>(null)

// Validation
const exportPasswordsMatch = computed(() =>
  exportPassword.value === exportConfirmPassword.value
)

const canExport = computed(() =>
  exportPassword.value.length >= 6 && exportPasswordsMatch.value
)

const canImport = computed(() =>
  importPassword.value.length >= 6 && importFile.value !== null
)

// Cleanup computed
const hasCleanupTargets = computed(() =>
  Object.values(cleanupTargets.value).some(v => v)
)

const canConfirmCleanup = computed(() => {
  if (!hasCleanupTargets.value) return false
  if (previewStore.isPreviewMode) {
    return cleanupConfirmText.value === t('userdata.cleanup.confirmText')
  }
  return cleanupPassword.value.length >= 6
})

// Fetch retention settings on mount
onMounted(async () => {
  await fetchRetentionSettings()
})

async function fetchRetentionSettings() {
  try {
    const response = await companionSettingsApi.getSettings()
    storageInfo.value = response.data.storage_info
  } catch (e) {
    console.error('Failed to fetch retention settings:', e)
  }
}

// Collect current settings
function collectSettings(): UserSettings {
  return {
    selected_provider_model: settingsStore.selectedProviderModel,
    temperature: settingsStore.temperature,
    max_tokens: settingsStore.maxTokens,
    theme_style: settingsStore.themeStyle,
    theme: themeStore.theme,
    locale: localeStore.currentLocale,
    timezone: localStorage.getItem('zimaos-blue-timezone') || undefined,
  }
}

// Export user data
async function handleExport() {
  if (!canExport.value) return

  exporting.value = true
  exportError.value = null

  try {
    const response = await userDataApi.export({
      password: exportPassword.value,
      format: exportFormat.value,
      settings: collectSettings(),
    })

    // Create download
    const blob = new Blob([JSON.stringify(response.data, null, 2)], {
      type: 'application/json'
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    const timestamp = new Date().toISOString().split('T')[0]
    const ext = exportFormat.value === 'encrypted' ? 'enc.json' : 'json'
    a.download = `zimaos-blue-backup-${timestamp}.${ext}`
    a.click()
    URL.revokeObjectURL(url)

    emit('status-change', t('userdata.exportSuccess'))

    // Reset form
    exportPassword.value = ''
    exportConfirmPassword.value = ''
  } catch (e) {
    exportError.value = e instanceof Error ? e.message : t('userdata.exportFailed')
  } finally {
    exporting.value = false
  }
}

// Handle file selection
function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    const file = input.files[0]
    if (file) importFile.value = file
    importPreview.value = null
    importError.value = null
  }
}

// Preview import
async function handlePreview() {
  if (!importFile.value || !importPassword.value) return

  importing.value = true
  importError.value = null

  try {
    const fileContent = await readFileAsText(importFile.value)
    const base64Data = btoa(fileContent)

    const response = await userDataApi.importPreview({
      password: importPassword.value,
      data: base64Data,
    })

    importPreview.value = response.data
  } catch (e) {
    importError.value = e instanceof Error ? e.message : t('userdata.previewFailed')
    importPreview.value = null
  } finally {
    importing.value = false
  }
}

// Import user data
async function handleImport() {
  if (!importFile.value || !importPassword.value) return

  importing.value = true
  importError.value = null

  try {
    const fileContent = await readFileAsText(importFile.value)
    const base64Data = btoa(fileContent)

    const response = await userDataApi.import({
      password: importPassword.value,
      data: base64Data,
    })

    if (response.data.success) {
      // Apply settings if returned
      if (response.data.settings) {
        applySettings(response.data.settings)
      }

      emit('status-change', t('userdata.importSuccess', {
        conversations: response.data.imported.conversations,
        messages: response.data.imported.messages,
      }))

      // Reset form
      importPassword.value = ''
      importFile.value = null
      importPreview.value = null
      if (fileInputRef.value) {
        fileInputRef.value.value = ''
      }
    }
  } catch (e) {
    importError.value = e instanceof Error ? e.message : t('userdata.importFailed')
  } finally {
    importing.value = false
  }
}

// Apply imported settings
function applySettings(settings: UserSettings) {
  if (settings.selected_provider_model) {
    settingsStore.setProviderModel(settings.selected_provider_model)
  }
  if (settings.temperature !== undefined) {
    settingsStore.setTemperature(settings.temperature)
  }
  if (settings.max_tokens !== undefined) {
    settingsStore.setMaxTokens(settings.max_tokens)
  }
  if (settings.theme_style) {
    settingsStore.setThemeStyle(settings.theme_style as 'default' | 'bubble' | 'minimal' | 'gradient' | 'ocean')
  }
  if (settings.theme) {
    themeStore.setTheme(settings.theme as 'light' | 'dark' | 'system')
  }
  if (settings.locale) {
    localeStore.changeLocale(settings.locale as 'en-US' | 'zh-CN' | 'ja-JP')
  }
  if (settings.timezone) {
    localStorage.setItem('zimaos-blue-timezone', settings.timezone)
  }
}

// Helper to read file as text
function readFileAsText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(new Error('Failed to read file'))
    reader.readAsText(file)
  })
}

// Clear import form
function clearImport() {
  importPassword.value = ''
  importFile.value = null
  importPreview.value = null
  importError.value = null
  if (fileInputRef.value) {
    fileInputRef.value.value = ''
  }
}

async function previewCleanup() {
  if (!hasCleanupTargets.value) return

  cleanupLoading.value = true
  cleanupError.value = null

  try {
    // Build preview data based on selected targets
    cleanupPreviewData.value = {
      chatHistory: cleanupTargets.value.chatHistory ? chatStore.conversations.length : 0,
      sessions: cleanupTargets.value.sessions ? storageInfo.value.session_count : 0,
      events: cleanupTargets.value.events ? storageInfo.value.event_count : 0,
      alerts: cleanupTargets.value.alerts ? storageInfo.value.alert_count : 0,
      settings: cleanupTargets.value.settings,
      cache: cleanupTargets.value.cache,
    }
    cleanupStep.value = 'preview'
  } catch (e) {
    cleanupError.value = e instanceof Error ? e.message : t('userdata.cleanupPreviewFailed')
  } finally {
    cleanupLoading.value = false
  }
}

function proceedToConfirm() {
  cleanupStep.value = 'confirm'
  cleanupPassword.value = ''
  cleanupConfirmText.value = ''
}

async function executeCleanup() {
  if (!canConfirmCleanup.value) return

  cleanupLoading.value = true
  cleanupError.value = null

  try {
    // Execute cleanup based on selected targets
    if (cleanupTargets.value.chatHistory) {
      // Clear chat history
      chatStore.clearAllConversations()
    }

    if (cleanupTargets.value.sessions || cleanupTargets.value.events || cleanupTargets.value.alerts) {
      // Trigger backend cleanup
      await companionSettingsApi.triggerCleanup()
    }

    if (cleanupTargets.value.settings) {
      // Reset settings to defaults
      settingsStore.resetToDefaults()
      themeStore.setTheme('system')
    }

    if (cleanupTargets.value.cache) {
      // Clear local storage cache (except essential items)
      const keysToKeep = ['zimaos-blue-locale', 'zimaos-blue-theme']
      const allKeys = Object.keys(localStorage)
      allKeys.forEach(key => {
        if (key.startsWith('zimaos-blue-') && !keysToKeep.includes(key)) {
          localStorage.removeItem(key)
        }
      })
    }

    emit('status-change', t('userdata.cleanupSuccess'))

    // Refresh storage info
    await fetchRetentionSettings()

    // Reset cleanup state
    cleanupStep.value = 'select'
    cleanupTargets.value = {
      chatHistory: false,
      sessions: false,
      events: false,
      alerts: false,
      settings: false,
      cache: false,
    }
    cleanupPassword.value = ''
    cleanupConfirmText.value = ''
    cleanupPreviewData.value = null
  } catch (e) {
    cleanupError.value = e instanceof Error ? e.message : t('userdata.cleanupFailed')
  } finally {
    cleanupLoading.value = false
  }
}

function cancelCleanup() {
  cleanupStep.value = 'select'
  cleanupPassword.value = ''
  cleanupConfirmText.value = ''
  cleanupError.value = null
}

function openDataModal(tab: 'export' | 'import' | 'cleanup' = 'export') {
  activeTab.value = tab
  showDataModal.value = true
  if (tab === 'cleanup') {
    cleanupStep.value = 'select'
    void fetchRetentionSettings()
  }
}

function closeDataModal() {
  showDataModal.value = false
  cleanupStep.value = 'select'
  cleanupError.value = null
}
</script>

<template>
  <div class="rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/50 shadow-sm overflow-hidden">
    <div class="px-6 py-5 border-b border-gray-200 dark:border-gray-700">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('userdata.chatDataTitle', 'Chat Data') }}</h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">{{ t('userdata.chatDataDescription', 'Import, export, and cleanup chat history and related data from one place.') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            data-testid="userdata-open-export-import"
            class="px-4 py-2 bg-gray-800 dark:bg-gray-500 hover:bg-gray-900 dark:hover:bg-gray-400 text-white text-sm font-medium rounded-lg transition-colors"
            @click="openDataModal('export')"
          >
            {{ t('userdata.manageData', 'Manage Data') }}
          </button>
          <button
            data-testid="userdata-open-cleanup"
            class="px-4 py-2 bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30 text-red-700 dark:text-red-300 text-sm font-medium rounded-lg transition-colors"
            @click="openDataModal('cleanup')"
          >
            {{ t('userdata.tabs.cleanup') }}
          </button>
        </div>
      </div>
    </div>
    <div class="px-6 py-4">
      <div class="rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
        {{ t('userdata.cleanup.warningDetail') }}
      </div>
    </div>
  </div>

  <Teleport to="body">
    <div v-if="showDataModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="closeDataModal">
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[86vh] overflow-y-auto">
        <div class="sticky top-0 bg-white dark:bg-gray-800 px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('userdata.chatDataTitle', 'Chat Data') }}</h3>
          <button class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" @click="closeDataModal">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>
        </div>

        <div class="p-6 space-y-4">
          <div class="flex border-b border-gray-200 dark:border-slate-700">
            <button
              v-for="tab in (['export', 'import', 'cleanup'] as const)"
              :key="tab"
              :class="[
                'flex-1 px-4 py-3 text-sm font-medium transition-colors border-b-2 -mb-px flex items-center justify-center gap-2',
                activeTab === tab
                  ? 'border-gray-900 dark:border-gray-700 text-gray-900 dark:text-gray-300'
                  : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
              ]"
              @click="activeTab = tab"
            >
              {{ t(`userdata.tabs.${tab}`) }}
            </button>
          </div>

          <!-- Export Tab Content -->
          <div v-show="activeTab === 'export'" class="glass-card p-4">
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">{{ t('userdata.exportDescription') }}</p>

            <!-- Format Selection -->
            <div class="mb-4">
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('userdata.format') }}</label>
              <div class="flex gap-2">
                <button
                  :class="[
                    'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                    exportFormat === 'json'
                      ? 'bg-gray-700 dark:bg-gray-500 text-white'
                      : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
                  ]"
                  @click="exportFormat = 'json'"
                >
                  {{ t('userdata.formatJson') }}
                </button>
                <button
                  :class="[
                    'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                    exportFormat === 'encrypted'
                      ? 'bg-gray-700 dark:bg-gray-500 text-white'
                      : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
                  ]"
                  @click="exportFormat = 'encrypted'"
                >
                  {{ t('userdata.formatEncrypted') }}
                </button>
              </div>
              <p class="text-xs text-gray-400 dark:text-slate-500 mt-1">
                {{ exportFormat === 'json' ? t('userdata.formatJsonDesc') : t('userdata.formatEncryptedDesc') }}
              </p>
            </div>

            <!-- Password -->
            <div class="space-y-3 mb-4">
              <div>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ t('userdata.password') }}</label>
                <input
                  v-model="exportPassword"
                  type="password"
                  :placeholder="t('userdata.passwordPlaceholder')"
                  class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
                />
              </div>
              <div>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ t('userdata.confirmPassword') }}</label>
                <input
                  v-model="exportConfirmPassword"
                  type="password"
                  :placeholder="t('userdata.confirmPasswordPlaceholder')"
                  class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
                  :class="{ 'border-red-500': exportConfirmPassword && !exportPasswordsMatch }"
                />
                <p v-if="exportConfirmPassword && !exportPasswordsMatch" class="text-xs text-red-500 mt-1">
                  {{ t('userdata.passwordMismatch') }}
                </p>
              </div>
            </div>

            <!-- Error -->
            <div v-if="exportError" class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-300 text-sm mb-4">
              {{ exportError }}
            </div>

            <!-- Export Button -->
            <button
              :disabled="!canExport || exporting"
              class="w-full px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
              @click="handleExport"
            >
              <svg v-if="exporting" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {{ exporting ? t('userdata.exporting') : t('userdata.exportButton') }}
            </button>
          </div>

          <!-- Import Tab Content -->
          <div v-show="activeTab === 'import'" class="glass-card p-4">
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">{{ t('userdata.importDescription') }}</p>

            <!-- File Selection -->
            <div class="mb-4">
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('userdata.selectFile') }}</label>
              <div class="flex items-center gap-2">
                <input
                  ref="fileInputRef"
                  type="file"
                  accept=".json"
                  class="hidden"
                  @change="handleFileSelect"
                />
                <button
                  class="px-4 py-2 bg-gray-100 dark:bg-slate-700 hover:bg-gray-200 dark:hover:bg-slate-600 text-gray-700 dark:text-white rounded-lg text-sm transition-colors"
                  @click="fileInputRef?.click()"
                >
                  {{ t('userdata.chooseFile') }}
                </button>
                <span v-if="importFile" class="text-sm text-gray-600 dark:text-gray-300 truncate flex-1">
                  {{ importFile.name }}
                </span>
                <button
                  v-if="importFile"
                  class="p-1 text-gray-400 hover:text-red-500 transition-colors"
                  @click="clearImport"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>

            <!-- Password -->
            <div class="mb-4">
              <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ t('userdata.password') }}</label>
              <input
                v-model="importPassword"
                type="password"
                :placeholder="t('userdata.importPasswordPlaceholder')"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
              />
            </div>

            <!-- Preview -->
            <div v-if="importPreview" class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-4 mb-4">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-2">{{ t('userdata.previewTitle') }}</h4>
              <div class="space-y-2 text-sm">
                <div class="flex justify-between">
                  <span class="text-gray-500 dark:text-slate-400">{{ t('userdata.version') }}</span>
                  <span class="text-gray-900 dark:text-white">{{ importPreview.version }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-gray-500 dark:text-slate-400">{{ t('userdata.exportedAt') }}</span>
                  <span class="text-gray-900 dark:text-white">{{ new Date(importPreview.exported_at).toLocaleString() }}</span>
                </div>
                <div v-if="importPreview.has_settings" class="flex justify-between">
                  <span class="text-gray-500 dark:text-slate-400">{{ t('userdata.hasSettings') }}</span>
                  <span class="text-green-600 dark:text-green-400">{{ t('common.yes') }}</span>
                </div>
                <div v-if="importPreview.chat_preview" class="flex justify-between">
                  <span class="text-gray-500 dark:text-slate-400">{{ t('userdata.conversations') }}</span>
                  <span class="text-gray-900 dark:text-white">{{ importPreview.chat_preview.conversations }}</span>
                </div>
                <div v-if="importPreview.chat_preview" class="flex justify-between">
                  <span class="text-gray-500 dark:text-slate-400">{{ t('userdata.messages') }}</span>
                  <span class="text-gray-900 dark:text-white">{{ importPreview.chat_preview.messages }}</span>
                </div>
              </div>
            </div>

            <!-- Error -->
            <div v-if="importError" class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-300 text-sm mb-4">
              {{ importError }}
            </div>

            <!-- Buttons -->
            <div class="flex gap-2">
              <button
                :disabled="!canImport || importing"
                class="flex-1 px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                @click="handlePreview"
              >
                {{ t('userdata.preview') }}
              </button>
              <button
                :disabled="!canImport || importing || !importPreview"
                class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                @click="handleImport"
              >
                <svg v-if="importing" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                {{ importing ? t('userdata.importing') : t('userdata.importButton') }}
              </button>
            </div>
          </div>

          <!-- Cleanup Tab Content -->
          <div v-show="activeTab === 'cleanup'" class="glass-card p-4 space-y-4">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('userdata.cleanup.description') }}</p>

            <!-- Warning Banner -->
            <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
              <div class="flex items-start gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-amber-500 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <div class="text-sm text-amber-700 dark:text-amber-300">
                  <p class="font-medium">{{ t('userdata.cleanup.warning') }}</p>
                  <p class="mt-1 text-amber-600 dark:text-amber-400">{{ t('userdata.cleanup.warningDetail') }}</p>
                </div>
              </div>
            </div>

            <!-- Step 1: Select Data to Clean -->
            <div v-if="cleanupStep === 'select'" class="space-y-4">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.cleanup.selectData') }}</h4>
              <div class="space-y-2">
                <label class="flex items-center gap-3 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg cursor-pointer hover:bg-gray-100 dark:hover:bg-slate-700">
                  <input v-model="cleanupTargets.chatHistory" type="checkbox" class="w-4 h-4 text-red-600 rounded border-gray-300 focus:ring-red-500" />
                  <div class="flex-1">
                    <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.cleanup.chatHistory') }}</div>
                    <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('userdata.cleanup.chatHistoryDesc') }}</div>
                  </div>
                  <span class="text-sm text-gray-500 dark:text-slate-400">{{ chatStore.conversations.length }} {{ t('userdata.conversations') }}</span>
                </label>
                <label class="flex items-center gap-3 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg cursor-pointer hover:bg-gray-100 dark:hover:bg-slate-700">
                  <input v-model="cleanupTargets.sessions" type="checkbox" class="w-4 h-4 text-red-600 rounded border-gray-300 focus:ring-red-500" />
                  <div class="flex-1">
                    <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.cleanup.sessions') }}</div>
                    <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('userdata.cleanup.sessionsDesc') }}</div>
                  </div>
                  <span class="text-sm text-gray-500 dark:text-slate-400">{{ storageInfo.session_count }}</span>
                </label>
                <label class="flex items-center gap-3 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg cursor-pointer hover:bg-gray-100 dark:hover:bg-slate-700">
                  <input v-model="cleanupTargets.events" type="checkbox" class="w-4 h-4 text-red-600 rounded border-gray-300 focus:ring-red-500" />
                  <div class="flex-1">
                    <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.cleanup.events') }}</div>
                    <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('userdata.cleanup.eventsDesc') }}</div>
                  </div>
                  <span class="text-sm text-gray-500 dark:text-slate-400">{{ storageInfo.event_count }}</span>
                </label>
                <label class="flex items-center gap-3 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg cursor-pointer hover:bg-gray-100 dark:hover:bg-slate-700">
                  <input v-model="cleanupTargets.alerts" type="checkbox" class="w-4 h-4 text-red-600 rounded border-gray-300 focus:ring-red-500" />
                  <div class="flex-1">
                    <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.cleanup.alerts') }}</div>
                    <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('userdata.cleanup.alertsDesc') }}</div>
                  </div>
                  <span class="text-sm text-gray-500 dark:text-slate-400">{{ storageInfo.alert_count }}</span>
                </label>
                <label class="flex items-center gap-3 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg cursor-pointer hover:bg-gray-100 dark:hover:bg-slate-700">
                  <input v-model="cleanupTargets.settings" type="checkbox" class="w-4 h-4 text-red-600 rounded border-gray-300 focus:ring-red-500" />
                  <div class="flex-1">
                    <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.cleanup.settings') }}</div>
                    <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('userdata.cleanup.settingsDesc') }}</div>
                  </div>
                </label>
                <label class="flex items-center gap-3 p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg cursor-pointer hover:bg-gray-100 dark:hover:bg-slate-700">
                  <input v-model="cleanupTargets.cache" type="checkbox" class="w-4 h-4 text-red-600 rounded border-gray-300 focus:ring-red-500" />
                  <div class="flex-1">
                    <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.cleanup.cache') }}</div>
                    <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('userdata.cleanup.cacheDesc') }}</div>
                  </div>
                </label>
              </div>
              <button
                :disabled="!hasCleanupTargets || cleanupLoading"
                class="w-full px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                @click="previewCleanup"
              >
                {{ t('userdata.cleanup.previewButton') }}
              </button>
            </div>

            <!-- Step 2: Preview -->
            <div v-else-if="cleanupStep === 'preview'" class="space-y-4">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.cleanup.previewTitle') }}</h4>
              <div v-if="cleanupPreviewData" class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 space-y-2">
                <div v-if="cleanupPreviewData.chatHistory > 0" class="flex justify-between text-sm">
                  <span class="text-red-700 dark:text-red-300">{{ t('userdata.cleanup.chatHistory') }}</span>
                  <span class="font-medium text-red-800 dark:text-red-200">{{ cleanupPreviewData.chatHistory }} {{ t('userdata.conversations') }}</span>
                </div>
                <div v-if="cleanupPreviewData.sessions > 0" class="flex justify-between text-sm">
                  <span class="text-red-700 dark:text-red-300">{{ t('userdata.cleanup.sessions') }}</span>
                  <span class="font-medium text-red-800 dark:text-red-200">{{ cleanupPreviewData.sessions }}</span>
                </div>
                <div v-if="cleanupPreviewData.events > 0" class="flex justify-between text-sm">
                  <span class="text-red-700 dark:text-red-300">{{ t('userdata.cleanup.events') }}</span>
                  <span class="font-medium text-red-800 dark:text-red-200">{{ cleanupPreviewData.events }}</span>
                </div>
                <div v-if="cleanupPreviewData.alerts > 0" class="flex justify-between text-sm">
                  <span class="text-red-700 dark:text-red-300">{{ t('userdata.cleanup.alerts') }}</span>
                  <span class="font-medium text-red-800 dark:text-red-200">{{ cleanupPreviewData.alerts }}</span>
                </div>
                <div v-if="cleanupPreviewData.settings" class="flex justify-between text-sm">
                  <span class="text-red-700 dark:text-red-300">{{ t('userdata.cleanup.settings') }}</span>
                  <span class="font-medium text-red-800 dark:text-red-200">{{ t('userdata.cleanup.willReset') }}</span>
                </div>
                <div v-if="cleanupPreviewData.cache" class="flex justify-between text-sm">
                  <span class="text-red-700 dark:text-red-300">{{ t('userdata.cleanup.cache') }}</span>
                  <span class="font-medium text-red-800 dark:text-red-200">{{ t('userdata.cleanup.willClear') }}</span>
                </div>
              </div>
              <div class="flex gap-2">
                <button class="flex-1 px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg text-sm font-medium transition-colors" @click="cancelCleanup">
                  {{ t('common.cancel') }}
                </button>
                <button class="flex-1 px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm font-medium transition-colors" @click="proceedToConfirm">
                  {{ t('userdata.cleanup.proceedToConfirm') }}
                </button>
              </div>
            </div>

            <!-- Step 3: Confirm -->
            <div v-else-if="cleanupStep === 'confirm'" class="space-y-4">
              <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3">
                <div class="text-sm text-red-700 dark:text-red-300">
                  <p class="font-medium">{{ t('userdata.cleanup.confirmStep') }}</p>
                  <p v-if="previewStore.isPreviewMode" class="mt-1 text-red-600 dark:text-red-400">{{ t('userdata.cleanup.confirmHintPreview', { confirmText: t('userdata.cleanup.confirmText') }) }}</p>
                  <p v-else class="mt-1 text-red-600 dark:text-red-400">{{ t('userdata.cleanup.confirmHint') }}</p>
                </div>
              </div>
              <div v-if="previewStore.isPreviewMode">
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ t('userdata.cleanup.typeToConfirm') }}</label>
                <input v-model="cleanupConfirmText" type="text" :placeholder="t('userdata.cleanup.typeToConfirmPlaceholder', { confirmText: t('userdata.cleanup.confirmText') })" class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-red-500 border border-gray-200 dark:border-slate-600" />
              </div>
              <div v-else>
                <label class="block text-sm text-gray-500 dark:text-slate-400 mb-1">{{ t('userdata.cleanup.enterPassword') }}</label>
                <input v-model="cleanupPassword" type="password" :placeholder="t('userdata.cleanup.passwordPlaceholder')" class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-red-500 border border-gray-200 dark:border-slate-600" />
              </div>
              <div v-if="cleanupError" class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-300 text-sm">
                {{ cleanupError }}
              </div>
              <div class="flex gap-2">
                <button class="flex-1 px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-700 dark:text-white rounded-lg text-sm font-medium transition-colors" @click="cancelCleanup">
                  {{ t('common.cancel') }}
                </button>
                <button :disabled="!canConfirmCleanup || cleanupLoading" class="flex-1 px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2" @click="executeCleanup">
                  <svg v-if="cleanupLoading" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                  {{ cleanupLoading ? t('userdata.cleanup.deleting') : t('userdata.cleanup.confirmDelete') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
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
</style>
