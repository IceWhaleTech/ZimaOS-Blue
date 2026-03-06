<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settings'
import { THEME_STYLES, type ThemeStyle, type MemoryRecallMode } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { backupApi } from '@/api/index'
import type { LocaleKey } from '@/i18n'
import type { BackupInfo } from '@/api/index'
import ClaudeCodeSettings from '@/components/ClaudeCodeSettings.vue'
import ProviderPoolSection from '@/components/ProviderPoolSection.vue'
import UserDataExport from '@/components/UserDataExport.vue'
import NetworkSettings from '@/components/settings/NetworkSettings.vue'
import SpeechSettings from '@/components/settings/SpeechSettings.vue'
import WorkspaceSettings from '@/components/settings/WorkspaceSettings.vue'
import UpdateSettings from '@/components/settings/UpdateSettings.vue'
import ApiProxySettings from '@/components/settings/ApiProxySettings.vue'
import MemoryManager from '@/components/MemoryManager.vue'
import BackupManager from '@/components/BackupManager.vue'
import { useTauri } from '@/composables/useTauri'
import { serviceApi } from '@/api/service'
import type { ServiceInfo } from '@/api/service'

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()
const { isTauri, setCloseBehavior } = useTauri()

const saveStatus = ref<string | null>(null)

function themeStyleLabel(style: { id: string; labelKey: string }): string {
  return te(style.labelKey) ? t(style.labelKey) : style.id
}

// Auto-start state
const serviceInfo = ref<ServiceInfo | null>(null)
const autoStartLoading = ref(false)
const autoStartEnabled = computed(() => serviceInfo.value?.installed && serviceInfo.value?.enabled)

// Active tab - flattened structure
const SETTINGS_TABS = ['general', 'llm', 'proxy', 'speech', 'network', 'memory', 'userdata'] as const
type TabType = typeof SETTINGS_TABS[number]
const initialTab = route.query.tab
const activeTab = ref<TabType>(
  typeof initialTab === 'string' && SETTINGS_TABS.includes(initialTab as TabType)
    ? (initialTab as TabType)
    : 'general'
)


// Tab icons (heroicons outline, 16x16)
const tabIcons: Record<TabType, string> = {
  general: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>',
  llm: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 2a5 5 0 0 0-4.8 3.6A3.5 3.5 0 0 0 4 9a3.5 3.5 0 0 0 1.1 2.5A4 4 0 0 0 4 14a4 4 0 0 0 2.6 3.8C7 19.7 8.8 21 11 21h1V2h-1z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 2a5 5 0 0 1 4.8 3.6A3.5 3.5 0 0 1 20 9a3.5 3.5 0 0 1-1.1 2.5A4 4 0 0 1 20 14a4 4 0 0 1-2.6 3.8C17 19.7 15.2 21 13 21h-1"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 9h4m-4 4h4m4-4h-4m4 4h-4"/>',
  proxy: '<circle cx="12" cy="13" r="9" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" fill="none"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 13l3.5-5"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 4V2m4.24 3.76l1.42-1.42M20 13h2M4 13H2m3.34-7.66L3.93 3.93"/><circle cx="12" cy="13" r="1.5" stroke-width="0" fill="currentColor"/>',
  speech: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"/>',
  network: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"/>',
  memory: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 3h6a2 2 0 012 2v14l-5-3-5 3V5a2 2 0 012-2z"/>',
  userdata: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>',
}

// Timezone
const detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const selectedTimezone = ref(localStorage.getItem('zimaos-blue-timezone') || detectedTimezone)

const timezones = computed(() => {
  try {
    const allTimezones = (Intl as unknown as { supportedValuesOf: (key: string) => string[] }).supportedValuesOf('timeZone')
    const filtered = allTimezones.filter((tz: string) => tz !== detectedTimezone)
    return [detectedTimezone, ...filtered]
  } catch {
    return [detectedTimezone, 'UTC', 'America/New_York', 'America/Los_Angeles', 'Europe/London', 'Europe/Paris', 'Asia/Tokyo', 'Asia/Shanghai']
  }
})

// System Tab - Backup
const backups = ref<BackupInfo[]>([])
const backupsLoading = ref(false)
const backupCreating = ref(false)
const backupRestoring = ref<string | null>(null)

function showSaveStatus(message: string) {
  saveStatus.value = message
  setTimeout(() => {
    saveStatus.value = null
  }, 2000)
}

async function handleLocaleChange(locale: string) {
  await localeStore.changeLocale(locale as LocaleKey)
  showSaveStatus(t('settings.languageSaved'))
}

function handleTimezoneChange(timezone: string) {
  selectedTimezone.value = timezone
  localStorage.setItem('zimaos-blue-timezone', timezone)
  showSaveStatus(t('settings.timezoneSaved'))
}

function handleCloseBehaviorChange(behavior: 'quit' | 'minimize') {
  settingsStore.setCloseBehavior(behavior)
  setCloseBehavior(behavior)
  showSaveStatus(t('settings.closeBehaviorSaved'))
}

function handleThemeStyleChange(style: ThemeStyle) {
  settingsStore.setThemeStyle(style)
}

async function handleMemoryRecallModeChange(mode: MemoryRecallMode) {
  try {
    await settingsStore.setMemoryRecallMode(mode)
    showSaveStatus(t('settings.memoryRecallMode.saved'))
  } catch {
    showSaveStatus(t('settings.memoryRecallMode.saveFailed'))
  }
}

const smallModelSaving = ref(false)
let smallModelPollInterval: ReturnType<typeof setInterval> | null = null
const smallModelDownloading = computed(() => {
  const status = settingsStore.smallModelStatus
  const state = status?.state
  return status?.downloading || state === 'connecting' || state === 'downloading'
})
const smallModelReady = computed(() => settingsStore.smallModelStatus?.ready ?? false)
const smallModelToggleDisabled = computed(() => smallModelSaving.value)
const soulReviewingId = ref<string | null>(null)
const smallModelStatsResetting = ref(false)
const smallModelAdvancedExpanded = ref(false)
const smallModelStatsExpanded = ref(false)
const shortQASuccessRate = computed(() => {
  const stats = settingsStore.smallModelStats
  if (!stats || stats.short_qa_route_attempts <= 0) return 0
  return Math.round((stats.short_qa_route_success / stats.short_qa_route_attempts) * 100)
})
const toolDispatchSuccessRate = computed(() => {
  const stats = settingsStore.smallModelStats
  if (!stats || stats.tool_dispatch_route_attempts <= 0) return 0
  return Math.round((stats.tool_dispatch_route_success / stats.tool_dispatch_route_attempts) * 100)
})
const fallbackReasonEntries = computed(() => {
  const reasons = settingsStore.smallModelStats?.fallback_reasons || {}
  return Object.entries(reasons).sort((a, b) => b[1] - a[1])
})

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) return (bytes / 1024 / 1024 / 1024).toFixed(1) + ' GB'
  if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return bytes + ' B'
}

async function withSmallModelSave(task: () => Promise<void>) {
  if (smallModelSaving.value) return
  try {
    smallModelSaving.value = true
    await task()
    showSaveStatus(t('settings.saved', 'Saved'))
  } catch {
    showSaveStatus(t('settings.saveFailed', 'Failed to save configuration'))
  } finally {
    smallModelSaving.value = false
  }
}

function stopSmallModelPoll() {
  if (smallModelPollInterval) {
    clearInterval(smallModelPollInterval)
    smallModelPollInterval = null
  }
}

function startSmallModelPoll() {
  if (smallModelPollInterval) return
  smallModelPollInterval = setInterval(() => {
    void fetchSmallModelStatus()
  }, 800)
}

async function fetchSmallModelStatus() {
  try {
    await settingsStore.fetchSmallModelStatus()
    if (smallModelDownloading.value) {
      startSmallModelPoll()
    } else {
      stopSmallModelPoll()
    }
  } catch {
    // ignore
  }
}

async function startSmallModelDownload() {
  try {
    await settingsStore.startSmallModelDownload()
    startSmallModelPoll()
    showSaveStatus(t('settings.smallModel.downloadStarted', 'Small model download started'))
  } catch {
    showSaveStatus(t('settings.smallModel.downloadFailed', 'Failed to start small model download'))
  }
}

async function cancelSmallModelDownload() {
  try {
    await settingsStore.cancelSmallModelDownload()
    await fetchSmallModelStatus()
    showSaveStatus(t('settings.smallModel.downloadCanceled', 'Small model download canceled'))
  } catch {
    showSaveStatus(t('settings.smallModel.downloadFailed', 'Failed to start small model download'))
  }
}

async function handleSmallModelEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelEnabled(next))
}

async function handleSmallModelSummaryEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelSummaryEnabled(next))
}

async function handleSmallModelDocExtractEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelDocExtractEnabled(next))
}

async function handleSmallModelContextPruneEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelContextPruneEnabled(next))
}

async function handleSmallModelMediaIntentEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelMediaIntentEnabled(next))
}

async function handleSmartToolSelectionEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmartToolSelection(next))
}

async function handleOfflineIRFallbackEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setOfflineIRFallbackEnabled(next))
}

async function handleFeatureIntentIREnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setFeatureIntentIREnabled(next))
}

async function handleSmallModelRouteShortQAEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelRouteShortQAEnabled(next))
}

async function handleSmallModelRouteToolDispatchEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelRouteToolDispatchEnabled(next))
}

function formatFallbackReason(reason: string): string {
  return reason.split('_').join(' ')
}

async function fetchSmallModelStats() {
  try {
    await settingsStore.fetchSmallModelStats()
  } catch {
    // ignore
  }
}

async function resetSmallModelStats() {
  if (smallModelStatsResetting.value) return
  try {
    smallModelStatsResetting.value = true
    await settingsStore.resetSmallModelStats()
    showSaveStatus(t('settings.smallModel.statsReset', 'Small-model stats reset'))
  } catch {
    showSaveStatus(t('settings.smallModel.statsResetFailed', 'Failed to reset small-model stats'))
  } finally {
    smallModelStatsResetting.value = false
  }
}

function formatProposalTime(value: string): string {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

async function fetchSoulProposals() {
  try {
    await settingsStore.fetchSoulProposals()
  } catch {
    // ignore
  }
}

async function approveSoulProposal(id: string) {
  if (soulReviewingId.value) return
  try {
    soulReviewingId.value = id
    await settingsStore.approveSoulProposal(id)
    showSaveStatus(t('settings.smallModel.soulApproved', 'SOUL proposal approved'))
  } catch {
    showSaveStatus(t('settings.smallModel.soulReviewFailed', 'Failed to review SOUL proposal'))
  } finally {
    soulReviewingId.value = null
  }
}

async function rejectSoulProposal(id: string) {
  if (soulReviewingId.value) return
  try {
    soulReviewingId.value = id
    await settingsStore.rejectSoulProposal(id)
    showSaveStatus(t('settings.smallModel.soulRejected', 'SOUL proposal rejected'))
  } catch {
    showSaveStatus(t('settings.smallModel.soulReviewFailed', 'Failed to review SOUL proposal'))
  } finally {
    soulReviewingId.value = null
  }
}

async function fetchServiceInfo() {
  try {
    const res = await serviceApi.getInfo()
    serviceInfo.value = res.data
  } catch { /* service API not available */ }
}

async function toggleAutoStart() {
  autoStartLoading.value = true
  try {
    if (autoStartEnabled.value) {
      await serviceApi.disable()
      await serviceApi.uninstall()
      showSaveStatus(t('service.disableSuccess'))
    } else {
      if (!serviceInfo.value?.installed) await serviceApi.install()
      await serviceApi.enable()
      showSaveStatus(t('service.enableSuccess'))
    }
    await fetchServiceInfo()
  } catch (e) {
    showSaveStatus(autoStartEnabled.value ? t('service.disableFailed') : t('service.enableFailed'))
  } finally {
    autoStartLoading.value = false
  }
}

function switchTab(tab: TabType) {
  activeTab.value = tab
  router.replace({ query: { tab } })

  // Load data for specific tabs
  if (tab === 'userdata' && backups.value.length === 0) {
    fetchBackups()
  }
  if (tab === 'proxy' && settingsStore.smallModelStats == null) {
    void fetchSmallModelStats()
  }
  if (tab === 'memory' && settingsStore.soulProposals.length === 0) {
    void fetchSoulProposals()
  }
}

// Backup functions
async function fetchBackups() {
  backupsLoading.value = true
  try {
    const response = await backupApi.list()
    backups.value = response.data || []
  } catch (e) {
    console.error('Failed to fetch backups:', e)
  } finally {
    backupsLoading.value = false
  }
}

async function createBackup() {
  backupCreating.value = true
  try {
    await backupApi.create()
    await fetchBackups()
    showSaveStatus(t('system.backupCreated'))
  } catch (e) {
    console.error('Failed to create backup:', e)
  } finally {
    backupCreating.value = false
  }
}

function onBackupCreate(_type: string, _name: string) {
  createBackup()
}

function onBackupDownload(_id: string) {
  // Download not implemented in API yet
}

async function restoreBackup(id: string) {
  if (!confirm(t('system.confirmRestore'))) return
  backupRestoring.value = id
  try {
    const response = await backupApi.restore(id, {
      require_restart: true,
      auto_restart: true,
      create_checkpoint: true,
    })
    const checkpointAt = response.data?.result?.checkpoint_at
    if (checkpointAt) {
      showSaveStatus(`${response.data?.message || t('system.backupRestored')} (checkpoint: ${new Date(checkpointAt).toLocaleString()})`)
    } else {
      showSaveStatus(response.data?.message || t('system.backupRestored'))
    }
  } catch (e) {
    console.error('Failed to restore backup:', e)
    showSaveStatus(t('system.backupRestoreFailed'))
  } finally {
    backupRestoring.value = null
  }
}

async function deleteBackup(id: string) {
  if (!confirm(t('system.confirmDeleteBackup'))) return
  try {
    await backupApi.delete(id)
    backups.value = backups.value.filter(b => b.id !== id)
    showSaveStatus(t('system.backupDeleted'))
  } catch (e) {
    console.error('Failed to delete backup:', e)
  }
}

onMounted(async () => {
  await settingsStore.fetchProviders()
  await settingsStore.fetchBackendSettings()
  await fetchSmallModelStatus()
  await fetchSmallModelStats()
  await fetchSoulProposals()
  fetchServiceInfo()

  // Load data based on initial tab
  const tab = route.query.tab
  if (typeof tab === 'string' && SETTINGS_TABS.includes(tab as TabType)) {
    switchTab(tab as TabType)
  }
})

onUnmounted(() => {
  stopSmallModelPoll()
})
</script>

<template>
  <div class="settings-view p-4 sm:p-6 max-w-4xl mx-auto">
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('settings.title') }}</h1>

    <!-- Save status notification -->
    <Transition name="notification">
      <div
        v-if="saveStatus"
        class="fixed top-20 right-4 bg-green-600 text-white px-4 py-3 rounded-lg shadow-xl z-[9999]"
      >
        {{ saveStatus }}
      </div>
    </Transition>

    <!-- Main Tabs -->
    <div class="flex overflow-x-auto border-b border-gray-200 dark:border-gray-700 mb-6 -mx-4 px-4 sm:mx-0 sm:px-0">
      <button
        v-for="tab in SETTINGS_TABS"
        :key="tab"
        class="px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap flex-shrink-0 flex items-center gap-1.5"
        :class="
          activeTab === tab
            ? 'text-gray-900 dark:text-white border-b-2 border-gray-900 dark:border-white'
            : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white'
        "
        @click="switchTab(tab)"
      >
        <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" v-html="tabIcons[tab]" />
        <span class="inline-flex items-center gap-1.5">
          <span>{{ t(`settings.tab.${tab}`) }}</span>
          <span
            v-if="tab === 'proxy'"
            class="settings-tab-beta"
          >
            Beta
          </span>
        </span>
      </button>
    </div>

    <!-- General Tab -->
    <div v-if="activeTab === 'general'" class="space-y-6">
      <!-- Language -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('common.language') }}</label>
        <select
          :value="localeStore.currentLocale"
          class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
          @change="handleLocaleChange(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="option in localeStore.options" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </div>

      <!-- Timezone -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.timezone') }}</label>
        <select
          :value="selectedTimezone"
          class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-slate-600"
          @change="handleTimezoneChange(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="tz in timezones" :key="tz" :value="tz">{{ tz }}</option>
        </select>
      </div>

      <!-- Theme -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('common.theme') }}</label>
        <div class="flex gap-2">
          <button
            v-for="theme in ['light', 'dark', 'system'] as const"
            :key="theme"
            :class="[
              'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
              themeStore.theme === theme
                ? 'bg-gray-700 dark:bg-gray-500 text-white'
                : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
            ]"
            @click="themeStore.setTheme(theme)"
          >
            {{ t(`common.${theme}`) }}
          </button>
        </div>
      </div>

      <!-- Chat Theme Style -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('theme.styles.title') }}</label>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <button
            v-for="style in THEME_STYLES"
            :key="style.id"
            class="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors border"
            :class="settingsStore.themeStyle === style.id
              ? 'bg-gray-100 dark:bg-gray-700/30 border-gray-300 dark:border-gray-500 text-gray-900 dark:text-white'
              : 'bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-700/60'"
            @click="handleThemeStyleChange(style.id)"
          >
            <span class="theme-style-btn-preview flex-shrink-0" :class="`theme-style-btn-${style.id}`" />
            <span class="font-medium">{{ themeStyleLabel(style) }}</span>
          </button>
        </div>
      </div>

      <!-- Close Behavior (Tauri only) -->
      <div v-if="isTauri" class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.closeBehavior') }}</label>
        <div class="flex gap-2">
          <button
            v-for="behavior in ['quit', 'minimize'] as const"
            :key="behavior"
            :class="[
              'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
              settingsStore.closeBehavior === behavior
                ? 'bg-gray-700 dark:bg-gray-500 text-white'
                : 'bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-slate-600'
            ]"
            @click="handleCloseBehaviorChange(behavior)"
          >
            {{ t(`settings.closeBehavior${behavior === 'quit' ? 'Quit' : 'Minimize'}`) }}
          </button>
        </div>
      </div>

      <!-- Auto-start -->
      <div v-if="serviceInfo" class="glass-card p-4">
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('service.autoStart') }}</h3>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('service.autoStartDescription') }}</p>
          </div>
          <button
            type="button"
            role="switch"
            :aria-checked="autoStartEnabled"
            :disabled="autoStartLoading"
            class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
            :class="autoStartEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
            @click="toggleAutoStart"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="autoStartEnabled ? 'translate-x-5' : 'translate-x-0'"
            />
          </button>
        </div>
      </div>

      <!-- About / Version -->
      <div class="glass-card p-4">
        <UpdateSettings />
      </div>
    </div>

    <!-- LLM Tab -->
    <div v-if="activeTab === 'llm'" class="space-y-6">
      <!-- Provider Pool -->
      <div class="glass-card p-4">
        <ProviderPoolSection />
      </div>

      <!-- Tool Call Approval (hidden for now) -->
      <!-- <ToolApprovalSettings @status-change="showSaveStatus" /> -->

      <!-- Claude Code CLI Settings -->
      <ClaudeCodeSettings @status-change="showSaveStatus" />

    </div>

    <!-- Optimization Tab -->
    <div v-if="activeTab === 'proxy'" class="space-y-6">
      <ApiProxySettings @status-change="showSaveStatus" />

      <!-- Assistant Capabilities -->
      <div class="glass-card p-4">
        <div class="mb-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('settings.smallModel.irTitle', 'Assistant Capabilities') }}</h3>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('settings.smallModel.irDesc', 'User-facing helpers for context control and tool filtering.') }}</p>
        </div>

        <div class="py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg space-y-3">
          <div class="flex items-center justify-between py-2 px-2.5 bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 rounded-lg">
            <div>
              <div class="text-sm text-gray-800 dark:text-gray-100">{{ t('settings.smallModel.irContextPruneTitle', 'Context Trimming') }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('settings.smallModel.irContextPruneDesc', 'Automatically trims less relevant history to reduce token use.') }}</div>
            </div>
            <button
              data-testid="small-model-context-prune-switch"
              type="button"
              role="switch"
              :aria-checked="settingsStore.smallModelContextPruneEnabled"
              :disabled="smallModelSaving"
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
              :class="settingsStore.smallModelContextPruneEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
              @click="handleSmallModelContextPruneEnabledChange(!settingsStore.smallModelContextPruneEnabled)"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                :class="settingsStore.smallModelContextPruneEnabled ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>

          <div class="flex items-center justify-between py-2 px-2.5 bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 rounded-lg">
            <div>
              <div class="text-sm text-gray-800 dark:text-gray-100">{{ t('settings.smallModel.mediaIntent', 'Media Generation Scenario Recognition') }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('settings.smallModel.irMediaIntentDesc', 'Detects media-generation intent to route requests more accurately.') }}</div>
            </div>
            <button
              data-testid="small-model-media-intent-switch"
              type="button"
              role="switch"
              :aria-checked="settingsStore.smallModelMediaIntentEnabled"
              :disabled="smallModelSaving"
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
              :class="settingsStore.smallModelMediaIntentEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
              @click="handleSmallModelMediaIntentEnabledChange(!settingsStore.smallModelMediaIntentEnabled)"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                :class="settingsStore.smallModelMediaIntentEnabled ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>

          <div class="flex items-center justify-between py-2 px-2.5 bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 rounded-lg">
            <div>
              <div class="text-sm text-gray-800 dark:text-gray-100">{{ t('apiProxy.smartToolsTitle', 'Smart Tool Selection') }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.smartToolsDesc', 'Send only relevant tools per query, reducing token usage') }}</div>
            </div>
            <button
              data-testid="smart-tool-selection-switch"
              type="button"
              role="switch"
              :aria-checked="settingsStore.smartToolSelection"
              :disabled="smallModelSaving"
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
              :class="settingsStore.smartToolSelection ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
              @click="handleSmartToolSelectionEnabledChange(!settingsStore.smartToolSelection)"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                :class="settingsStore.smartToolSelection ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>

          <div class="flex items-center justify-between py-2 px-2.5 bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 rounded-lg">
            <div>
              <div class="text-sm text-gray-800 dark:text-gray-100">{{ t('settings.smallModel.irOfflineFallbackTitle', 'Offline Local Fallback') }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('settings.smallModel.irOfflineFallbackDesc', 'When model fallback is needed, answer from local context recall first.') }}</div>
            </div>
            <button
              data-testid="offline-ir-fallback-switch"
              type="button"
              role="switch"
              :aria-checked="settingsStore.offlineIRFallbackEnabled"
              :disabled="smallModelSaving"
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
              :class="settingsStore.offlineIRFallbackEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
              @click="handleOfflineIRFallbackEnabledChange(!settingsStore.offlineIRFallbackEnabled)"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                :class="settingsStore.offlineIRFallbackEnabled ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>

          <div class="flex items-center justify-between py-2 px-2.5 bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 rounded-lg">
            <div>
              <div class="text-sm text-gray-800 dark:text-gray-100">{{ t('settings.smallModel.irFeatureHintTitle', 'Feature Hint Detection') }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('settings.smallModel.irFeatureHintDesc', { deepResearch: t('ui.deepResearchTitle'), agentMode: t('agent.mode') }) }}</div>
            </div>
            <button
              data-testid="feature-intent-ir-switch"
              type="button"
              role="switch"
              :aria-checked="settingsStore.featureIntentIREnabled"
              :disabled="smallModelSaving"
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
              :class="settingsStore.featureIntentIREnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
              @click="handleFeatureIntentIREnabledChange(!settingsStore.featureIntentIREnabled)"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                :class="settingsStore.featureIntentIREnabled ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>
        </div>
      </div>

      <!-- Small Model Control -->
      <div class="glass-card p-4">
      <div class="mb-3 flex items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('settings.smallModel.title', 'Lightweight Acceleration') }}</h3>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('settings.smallModel.description', 'Use a lightweight model for faster simple tasks, with automatic fallback if unavailable.') }}</p>
        </div>
        <button
          type="button"
          role="switch"
          :aria-checked="settingsStore.smallModelEnabled"
          :disabled="smallModelToggleDisabled"
          class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
          :class="settingsStore.smallModelEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
          @click="handleSmallModelEnabledChange(!settingsStore.smallModelEnabled)"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
            :class="settingsStore.smallModelEnabled ? 'translate-x-5' : 'translate-x-0'"
          />
        </button>
      </div>

      <div class="space-y-3">
        <div class="flex items-center justify-end">
          <span
            class="text-xs px-2 py-1 rounded-full whitespace-nowrap"
            :class="smallModelReady ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300' : smallModelDownloading ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300' : 'bg-gray-100 text-gray-600 dark:bg-gray-700/60 dark:text-gray-300'"
          >
            {{ smallModelReady ? t('settings.smallModel.ready', 'Ready') : smallModelDownloading ? t('settings.smallModel.downloading', 'Downloading') : t('settings.smallModel.notReady', 'Not Ready') }}
          </span>
        </div>

        <div class="py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
          <h4 class="text-sm text-gray-800 dark:text-gray-100">{{ t('settings.smallModel.userGuideTitle', 'What this does') }}</h4>
          <ul class="mt-1.5 list-disc pl-4 space-y-1 text-xs text-gray-600 dark:text-gray-300">
            <li>{{ t('settings.smallModel.userGuideItem1', 'Prioritizes the lightweight model for simple tasks to improve response speed.') }}</li>
            <li>{{ t('settings.smallModel.userGuideItem2', 'Automatically falls back to the main model when the lightweight model is unavailable.') }}</li>
            <li>{{ t('settings.smallModel.userGuideItem3', 'Download the lightweight model before first use.') }}</li>
          </ul>
        </div>

        <div class="py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
          <button
            data-testid="small-model-advanced-toggle"
            type="button"
            class="w-full flex items-center justify-between gap-2 text-left"
            @click="smallModelAdvancedExpanded = !smallModelAdvancedExpanded"
          >
            <h4 class="text-sm text-gray-800 dark:text-gray-100">{{ t('settings.smallModel.advancedTitle', 'Advanced Parameters (Usually no change needed)') }}</h4>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ smallModelAdvancedExpanded ? t('settings.smallModel.collapse', 'Collapse') : t('settings.smallModel.expand', 'Expand') }}</span>
          </button>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
            {{ t('settings.smallModel.advancedHint', 'Only adjust these when troubleshooting or running controlled rollout tests.') }}
          </p>

          <div v-if="smallModelAdvancedExpanded" class="mt-3 space-y-3">
            <div class="py-2.5 px-3 bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 rounded-lg space-y-1.5 text-xs">
              <div class="flex items-center justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.runtime', 'Runtime') }}</span>
                <span class="font-mono text-gray-800 dark:text-gray-100">{{ settingsStore.smallModelRuntime }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.modelId', 'Model ID') }}</span>
                <span class="font-mono text-gray-800 dark:text-gray-100">{{ settingsStore.smallModelID }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.noLLMDegrade', 'No LLM Degrade') }}</span>
                <span class="font-mono text-gray-800 dark:text-gray-100">{{ settingsStore.noLLMDegradeMode }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.unavailablePolicy', 'Unavailable Policy') }}</span>
                <span class="font-mono text-gray-800 dark:text-gray-100">{{ settingsStore.smallModelUnavailablePolicy }}</span>
              </div>
            </div>

          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <div class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300">
            <div class="flex items-center justify-between gap-3">
              <div class="font-medium text-gray-900 dark:text-white">{{ t('settings.smallModel.summary', 'Summary / Compression') }}</div>
              <button
                data-testid="small-model-summary-switch"
                type="button"
                role="switch"
                :aria-checked="settingsStore.smallModelSummaryEnabled"
                :disabled="smallModelSaving"
                class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                :class="settingsStore.smallModelSummaryEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
                @click="handleSmallModelSummaryEnabledChange(!settingsStore.smallModelSummaryEnabled)"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                  :class="settingsStore.smallModelSummaryEnabled ? 'translate-x-5' : 'translate-x-0'"
                />
              </button>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ settingsStore.smallModelSummaryEnabled ? t('common.enabled', 'Enabled') : t('common.disabled', 'Disabled') }}</div>
          </div>

          <div class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300">
            <div class="flex items-center justify-between gap-3">
              <div class="font-medium text-gray-900 dark:text-white">{{ t('settings.smallModel.docExtract', 'Workflow Document Extraction') }}</div>
              <button
                data-testid="small-model-doc-extract-switch"
                type="button"
                role="switch"
                :aria-checked="settingsStore.smallModelDocExtractEnabled"
                :disabled="smallModelSaving"
                class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                :class="settingsStore.smallModelDocExtractEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
                @click="handleSmallModelDocExtractEnabledChange(!settingsStore.smallModelDocExtractEnabled)"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                  :class="settingsStore.smallModelDocExtractEnabled ? 'translate-x-5' : 'translate-x-0'"
                />
              </button>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ settingsStore.smallModelDocExtractEnabled ? t('common.enabled', 'Enabled') : t('common.disabled', 'Disabled') }}</div>
          </div>

          <div class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300">
            <div class="flex items-center justify-between gap-3">
              <div class="font-medium text-gray-900 dark:text-white">{{ t('settings.smallModel.shortQA', 'Short QA Routing') }}</div>
              <button
                data-testid="small-model-short-qa-switch"
                type="button"
                role="switch"
                :aria-checked="settingsStore.smallModelRouteShortQAEnabled"
                :disabled="smallModelSaving"
                class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                :class="settingsStore.smallModelRouteShortQAEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
                @click="handleSmallModelRouteShortQAEnabledChange(!settingsStore.smallModelRouteShortQAEnabled)"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                  :class="settingsStore.smallModelRouteShortQAEnabled ? 'translate-x-5' : 'translate-x-0'"
                />
              </button>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ settingsStore.smallModelRouteShortQAEnabled ? t('common.enabled', 'Enabled') : t('common.disabled', 'Disabled') }}</div>
          </div>

          <div class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300">
            <div class="flex items-center justify-between gap-3">
              <div class="font-medium text-gray-900 dark:text-white">{{ t('settings.smallModel.toolDispatch', 'Tool Dispatch Routing') }}</div>
              <button
                data-testid="small-model-tool-dispatch-switch"
                type="button"
                role="switch"
                :aria-checked="settingsStore.smallModelRouteToolDispatchEnabled"
                :disabled="smallModelSaving"
                class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                :class="settingsStore.smallModelRouteToolDispatchEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
                @click="handleSmallModelRouteToolDispatchEnabledChange(!settingsStore.smallModelRouteToolDispatchEnabled)"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                  :class="settingsStore.smallModelRouteToolDispatchEnabled ? 'translate-x-5' : 'translate-x-0'"
                />
              </button>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ settingsStore.smallModelRouteToolDispatchEnabled ? t('common.enabled', 'Enabled') : t('common.disabled', 'Disabled') }}</div>
          </div>
        </div>

        <div class="py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
          <button
            data-testid="small-model-stats-toggle"
            type="button"
            class="w-full flex items-center justify-between gap-2 text-left"
            @click="smallModelStatsExpanded = !smallModelStatsExpanded"
          >
            <h4 class="text-sm text-gray-800 dark:text-gray-100">{{ t('settings.smallModel.statsTitle', 'Runtime Stats') }}</h4>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ smallModelStatsExpanded ? t('settings.smallModel.collapse', 'Collapse') : t('settings.smallModel.expand', 'Expand') }}</span>
          </button>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
            {{ t('settings.smallModel.statsHint', 'For troubleshooting and tuning. Daily use usually does not require attention.') }}
          </p>

          <div v-if="smallModelStatsExpanded" class="mt-3 space-y-3">
            <div class="flex items-center justify-end gap-2">
              <button
                data-testid="small-model-stats-refresh"
                class="px-2 py-1 rounded border border-gray-200 dark:border-gray-600 text-xs text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50"
                :disabled="settingsStore.smallModelStatsLoading"
                @click="fetchSmallModelStats"
              >
                {{ t('common.refresh', 'Refresh') }}
              </button>
              <button
                data-testid="small-model-stats-reset"
                class="px-2 py-1 rounded border border-red-200 dark:border-red-800 text-xs text-red-600 dark:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-50"
                :disabled="smallModelStatsResetting"
                @click="resetSmallModelStats"
              >
                {{ t('settings.smallModel.resetStats', 'Reset') }}
              </button>
            </div>

            <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.shortQAAttempts', 'Short QA Attempts') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ settingsStore.smallModelStats?.short_qa_route_attempts ?? 0 }}</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.shortQASuccessRate', 'Short QA Success') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ shortQASuccessRate }}%</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.toolDispatchAttempts', 'Tool Dispatch Attempts') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ settingsStore.smallModelStats?.tool_dispatch_route_attempts ?? 0 }}</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.toolDispatchSuccessRate', 'Tool Dispatch Success') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ toolDispatchSuccessRate }}%</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.deepResearchFallbacks', 'DeepResearch Fallbacks') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ settingsStore.smallModelStats?.no_provider_deepresearch_total ?? 0 }}</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.irTakeovers', 'Strategy Takeovers') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ settingsStore.smallModelStats?.ir_takeover_total ?? 0 }}</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.autoRollbacks', 'Auto Rollbacks') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ settingsStore.smallModelStats?.auto_rollback_total ?? 0 }}</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.fallbackTotal', 'Fallback Total') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ settingsStore.smallModelStats?.small_model_fallback_total ?? 0 }}</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.timeoutTotal', 'Timeout Total') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ settingsStore.smallModelStats?.small_model_timeout_total ?? 0 }}</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.latencyMs', 'Small-model Latency') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ ((settingsStore.smallModelStats?.small_model_latency_ms ?? 0)).toFixed(1) }}ms</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.shortQALatencyMs', 'Short QA Latency') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ ((settingsStore.smallModelStats?.short_qa_latency_ms ?? 0)).toFixed(1) }}ms</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.toolDispatchLatencyMs', 'Tool Dispatch Latency') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ ((settingsStore.smallModelStats?.tool_dispatch_latency_ms ?? 0)).toFixed(1) }}ms</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.summaryLatencyMs', 'Summary Latency') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ ((settingsStore.smallModelStats?.summary_latency_ms ?? 0)).toFixed(1) }}ms</div>
              </div>
              <div class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2">
                <div class="text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.docExtractLatencyMs', 'Doc Extract Latency') }}</div>
                <div class="mt-1 font-medium text-gray-900 dark:text-white">{{ ((settingsStore.smallModelStats?.doc_extract_latency_ms ?? 0)).toFixed(1) }}ms</div>
              </div>
            </div>

            <div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">{{ t('settings.smallModel.fallbackReasons', 'Fallback Reasons') }}</div>
              <div v-if="fallbackReasonEntries.length === 0" class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('settings.smallModel.noFallbackReasons', 'No fallback reasons recorded') }}
              </div>
              <div v-else class="space-y-1">
                <div
                  v-for="[reason, count] in fallbackReasonEntries"
                  :key="reason"
                  class="flex items-center justify-between text-xs rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-1.5"
                >
                  <span class="text-gray-700 dark:text-gray-200 font-mono">{{ formatFallbackReason(reason) }}</span>
                  <span class="text-gray-900 dark:text-white font-medium">{{ count }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <template v-if="settingsStore.smallModelStatus && smallModelDownloading">
          <div v-if="settingsStore.smallModelStatus.state === 'connecting' && settingsStore.smallModelStatus.progress" class="space-y-1.5 px-3 py-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
            <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
              <span>{{ settingsStore.smallModelStatus.progress.file }} ({{ settingsStore.smallModelStatus.progress.file_index + 1 }}/{{ settingsStore.smallModelStatus.progress.total_files }})</span>
              <span>{{ t('settings.smallModel.connecting', 'Connecting') }}</span>
            </div>
            <div class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
              <div class="h-full bg-blue-500/50 dark:bg-blue-400/50 rounded-full animate-pulse w-full"></div>
            </div>
          </div>
          <div v-else-if="settingsStore.smallModelStatus.state === 'downloading' && settingsStore.smallModelStatus.progress" class="space-y-1.5 px-3 py-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
            <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
              <span>{{ settingsStore.smallModelStatus.progress.file }} ({{ settingsStore.smallModelStatus.progress.file_index + 1 }}/{{ settingsStore.smallModelStatus.progress.total_files }})</span>
              <span>{{ settingsStore.smallModelStatus.progress.percentage.toFixed(1) }}%</span>
            </div>
            <div class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
              <div class="h-full bg-blue-500 dark:bg-blue-400 rounded-full transition-all duration-300" :style="{ width: settingsStore.smallModelStatus.progress.percentage + '%' }"></div>
            </div>
            <div class="flex items-center justify-between text-xs text-gray-400 dark:text-gray-500">
              <span>{{ formatBytes(settingsStore.smallModelStatus.progress.downloaded) }} / {{ settingsStore.smallModelStatus.progress.total > 0 ? formatBytes(settingsStore.smallModelStatus.progress.total) : '...' }}</span>
              <span>{{ settingsStore.smallModelStatus.progress.speed_human }} &middot; {{ settingsStore.smallModelStatus.progress.eta || '...' }}</span>
            </div>
          </div>
        </template>

        <div v-if="settingsStore.smallModelStatus?.state === 'error' && settingsStore.smallModelStatus.error" class="px-3 py-2 bg-red-50 dark:bg-red-900/20 rounded-lg flex items-center justify-between">
          <p class="text-xs text-red-600 dark:text-red-400">{{ settingsStore.smallModelStatus.error }}</p>
          <button class="text-xs text-gray-600 dark:text-gray-300 hover:text-gray-800 dark:hover:text-white ml-2 flex-shrink-0" @click="startSmallModelDownload">{{ t('common.retry') }}</button>
        </div>

        <div class="flex items-center justify-end gap-2">
          <button
            data-testid="small-model-status-refresh"
            class="px-3 py-1.5 rounded-md border border-gray-200 dark:border-gray-600 text-xs text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
            @click="fetchSmallModelStatus"
          >
            {{ t('common.refresh', 'Refresh') }}
          </button>
          <button
            v-if="!smallModelDownloading"
            data-testid="small-model-download"
            class="px-3 py-1.5 rounded-md text-xs text-white bg-gray-800 hover:bg-gray-900 dark:bg-gray-200 dark:text-gray-900 dark:hover:bg-white"
            @click="startSmallModelDownload"
          >
            {{ smallModelReady ? t('settings.smallModel.redownload', 'Re-download') : t('settings.smallModel.download', 'Download Model') }}
          </button>
          <button
            v-else
            data-testid="small-model-download-cancel"
            class="px-3 py-1.5 rounded-md text-xs text-red-600 border border-red-200 dark:text-red-300 dark:border-red-800 hover:bg-red-50 dark:hover:bg-red-900/20"
            @click="cancelSmallModelDownload"
          >
            {{ t('common.cancel') }}
          </button>
        </div>
        </div>
      </div>
    </div>

    <!-- Network Tab -->
    <div v-if="activeTab === 'network'">
      <NetworkSettings @status-change="showSaveStatus" />
    </div>

    <!-- Speech Tab -->
    <div v-if="activeTab === 'speech'">
      <SpeechSettings />
    </div>

    <!-- Memory Tab -->
    <div v-if="activeTab === 'memory'" class="space-y-6">
      <!-- Memory Management -->
      <MemoryManager @status-change="showSaveStatus" />

      <!-- Memory Recall Mode -->
      <div class="glass-card p-4">
        <label class="block text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('settings.memoryRecallMode.title') }}</label>
        <p class="text-xs text-gray-500 dark:text-gray-400 mb-3">{{ t('settings.memoryRecallMode.description') }}</p>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
          <button
            v-for="mode in ['aggressive', 'balanced', 'quality'] as const"
            :key="mode"
            class="w-full px-3 py-2 rounded-lg text-sm transition-colors border text-left"
            :class="settingsStore.memoryRecallMode === mode
              ? 'bg-gray-100 dark:bg-gray-700/30 border-gray-300 dark:border-gray-500 text-gray-900 dark:text-white'
              : 'bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-700/60'"
            @click="handleMemoryRecallModeChange(mode)"
          >
            <div class="font-medium">{{ t(`settings.memoryRecallMode.options.${mode}.label`) }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ t(`settings.memoryRecallMode.options.${mode}.hint`) }}</div>
          </button>
        </div>
      </div>

      <!-- SOUL Proposal Review -->
      <div class="glass-card p-4">
        <div class="flex items-start justify-between gap-3 mb-3">
          <div>
            <label class="block text-sm text-gray-500 dark:text-slate-400">{{ t('settings.smallModel.soulTitle', 'SOUL Proposals (Manual Review)') }}</label>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ t('settings.smallModel.soulHint', 'Self-evolution writes require explicit approval before persistence.') }}</p>
          </div>
          <button class="px-2.5 py-1.5 rounded-md border border-gray-200 dark:border-gray-600 text-xs text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700" @click="fetchSoulProposals">
            {{ t('common.refresh', 'Refresh') }}
          </button>
        </div>

        <div v-if="settingsStore.soulProposalsLoading" class="text-xs text-gray-500 dark:text-gray-400 py-1">
          {{ t('common.loading', 'Loading...') }}
        </div>
        <div v-else-if="settingsStore.soulProposals.length === 0" class="text-xs text-gray-500 dark:text-gray-400 py-1">
          {{ t('settings.smallModel.soulEmpty', 'No proposals yet.') }}
        </div>
        <div v-else class="space-y-2">
          <div
            v-for="proposal in settingsStore.soulProposals"
            :key="proposal.id"
            class="rounded-lg border border-gray-200 dark:border-gray-700 p-3 bg-gray-50 dark:bg-slate-800/30"
          >
            <div class="flex items-start justify-between gap-3">
              <div>
                <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ proposal.title }}</h4>
                <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  <span class="mr-2">{{ t('settings.smallModel.createdAt', 'Created') }}: {{ formatProposalTime(proposal.created_at) }}</span>
                  <span v-if="proposal.source">{{ t('settings.smallModel.source', 'Source') }}: {{ proposal.source }}</span>
                </div>
              </div>
              <span
                class="text-[11px] px-2 py-0.5 rounded-full whitespace-nowrap"
                :class="proposal.status === 'approved'
                  ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
                  : proposal.status === 'rejected'
                    ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
                    : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300'"
              >
                {{ proposal.status }}
              </span>
            </div>
            <p class="text-xs text-gray-700 dark:text-gray-200 mt-2 whitespace-pre-wrap">{{ proposal.content }}</p>

            <div v-if="proposal.status === 'pending'" class="mt-3 flex items-center justify-end gap-2">
              <button
                :data-testid="`soul-reject-${proposal.id}`"
                class="px-2.5 py-1.5 rounded-md text-xs text-red-600 border border-red-200 dark:text-red-300 dark:border-red-800 hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-50"
                :disabled="soulReviewingId === proposal.id"
                @click="rejectSoulProposal(proposal.id)"
              >
                {{ t('common.reject', 'Reject') }}
              </button>
              <button
                :data-testid="`soul-approve-${proposal.id}`"
                class="px-2.5 py-1.5 rounded-md text-xs text-white bg-green-600 hover:bg-green-700 dark:bg-green-500 dark:text-white dark:hover:bg-green-400 disabled:opacity-50"
                :disabled="soulReviewingId === proposal.id"
                @click="approveSoulProposal(proposal.id)"
              >
                {{ t('common.approve', 'Approve') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- User Data Tab -->
    <div v-if="activeTab === 'userdata'" class="space-y-6">
      <!-- Workspace Files -->
      <div class="glass-card p-4">
        <WorkspaceSettings @status-change="showSaveStatus" />
      </div>

      <!-- Backup -->
      <BackupManager
        :backups="backups"
        :loading="backupsLoading"
        :restoring="backupRestoring !== null"
        @create="onBackupCreate"
        @restore="restoreBackup"
        @delete="deleteBackup"
        @download="onBackupDownload"
      />

      <UserDataExport @status-change="showSaveStatus" />
    </div>

  </div>
</template>

<style scoped>
input[type='range'] {
  -webkit-appearance: none;
}

input[type='range']::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  background: var(--color-gray-900, #3b82f6);
  border-radius: 50%;
  cursor: pointer;
}

input[type='range']::-moz-range-thumb {
  width: 16px;
  height: 16px;
  background: var(--color-gray-900, #3b82f6);
  border-radius: 50%;
  cursor: pointer;
  border: none;
}

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

.theme-style-btn-preview {
  width: 18px;
  height: 18px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.4);
}

.settings-tab-beta {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.125rem 0.375rem;
  border-radius: 999px;
  font-size: 0.625rem;
  font-weight: 700;
  letter-spacing: 0.01em;
  color: rgb(29, 78, 216);
  background: rgb(219, 234, 254);
  border: 1px solid rgb(147, 197, 253);
  line-height: 1;
}

:root.dark .settings-tab-beta {
  color: rgb(191, 219, 254);
  background: rgba(30, 64, 175, 0.25);
  border-color: rgba(147, 197, 253, 0.45);
}

/* Notification transition */
.notification-enter-active,
.notification-leave-active {
  transition: all 0.3s ease;
}

.notification-enter-from {
  opacity: 0;
  transform: translateX(100px);
}

.notification-leave-to {
  opacity: 0;
  transform: translateX(100px);
}
</style>
