<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, defineAsyncComponent, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settings'
import { type CloseBehavior, type MemoryRecallMode } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore, type Theme } from '@/stores/theme'
import { backupApi } from '@/api/index'
import { settingsApi, type AgentcoreRunnerTagList, type ContextCompressionMode } from '@/api/settings'
import type { LocaleKey } from '@/i18n'
import type { BackupInfo } from '@/api/index'
import ProviderPoolSection from '@/components/ProviderPoolSection.vue'
import UserDataExport from '@/components/UserDataExport.vue'
import NetworkSettings from '@/components/settings/NetworkSettings.vue'
import UpdateSettings from '@/components/settings/UpdateSettings.vue'
import ExternalAgentsSection from '@/components/settings/ExternalAgentsSection.vue'
import MemoryManager from '@/components/MemoryManager.vue'
import BackupManager from '@/components/BackupManager.vue'
import { proxyCacheApi, type PrunerConfig } from '@/api/proxyCache'
import { useTauri } from '@/composables/useTauri'
import { serviceApi } from '@/api/service'
import type { ServiceInfo } from '@/api/service'
import { formatSmallModelFallbackReason } from '@/utils/smallModelFallbackReason'

const SpeechSettings = defineAsyncComponent(() =>
  import('@/components/settings/SpeechSettings.vue').then((module) => module.default)
)
const ApiProxySettings = defineAsyncComponent(() =>
  import('@/components/settings/ApiProxySettings.vue').then((module) => module.default)
)

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const localeStore = useLocaleStore()
const themeStore = useThemeStore()
const { isTauri, platform, setCloseBehavior, restartServerRuntime } = useTauri()

interface SaveStatusAction {
  label: string
  handler: () => void
}

interface SaveStatusToast {
  message: string
  action?: SaveStatusAction
}

const saveStatus = ref<SaveStatusToast | null>(null)
let saveStatusTimer: ReturnType<typeof setTimeout> | null = null

// Auto-start state
const serviceInfo = ref<ServiceInfo | null>(null)
const autoStartLoading = ref(false)
const autoStartEnabled = computed(() => serviceInfo.value?.installed && serviceInfo.value?.enabled)

// Active tab - flattened structure
const SETTINGS_TABS = ['general', 'llm', 'proxy', 'speech', 'userdata'] as const
type TabType = (typeof SETTINGS_TABS)[number]
const initialTabRaw = route.query.tab
const initialTab = Array.isArray(initialTabRaw) ? initialTabRaw[0] : initialTabRaw
const normalizedInitialTab = initialTab === 'memory' ? 'userdata' : initialTab
const hasInitialTabQuery =
  typeof normalizedInitialTab === 'string' &&
  SETTINGS_TABS.includes(normalizedInitialTab as TabType)
const requestedInitialTab: TabType = hasInitialTabQuery
  ? (normalizedInitialTab as TabType)
  : 'general'
const activeTab = ref<TabType>(requestedInitialTab)

// Tab icons (heroicons outline, 16x16)
const tabIcons: Record<TabType, string> = {
  general:
    '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>',
  llm: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 2a5 5 0 0 0-4.8 3.6A3.5 3.5 0 0 0 4 9a3.5 3.5 0 0 0 1.1 2.5A4 4 0 0 0 4 14a4 4 0 0 0 2.6 3.8C7 19.7 8.8 21 11 21h1V2h-1z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 2a5 5 0 0 1 4.8 3.6A3.5 3.5 0 0 1 20 9a3.5 3.5 0 0 1-1.1 2.5A4 4 0 0 1 20 14a4 4 0 0 1-2.6 3.8C17 19.7 15.2 21 13 21h-1"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 9h4m-4 4h4m4-4h-4m4 4h-4"/>',
  proxy:
    '<circle cx="12" cy="13" r="9" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" fill="none"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 13l3.5-5"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 4V2m4.24 3.76l1.42-1.42M20 13h2M4 13H2m3.34-7.66L3.93 3.93"/><circle cx="12" cy="13" r="1.5" stroke-width="0" fill="currentColor"/>',
  speech:
    '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"/>',
  userdata:
    '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>',
}

const themeOptions: Theme[] = ['light', 'dark', 'system']

const themeIcons: Record<Theme, string> = {
  light:
    '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 3v1.5m0 15V21m9-9h-1.5m-15 0H3m15.364 6.364l-1.06-1.06M6.697 6.697l-1.06-1.06m12.727 0l-1.06 1.06M6.697 17.303l-1.06 1.06"/><circle cx="12" cy="12" r="3.5" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" fill="none"/>',
  dark: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 12.79A9 9 0 1111.21 3 7 7 0 0021 12.79z"/>',
  system:
    '<rect x="3.75" y="4.5" width="16.5" height="11.5" rx="2" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" fill="none"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 19.5h6m-4.5-3.5v3.5m3-3.5v3.5"/>',
}

const closeBehaviorIcons: Record<CloseBehavior, string> = {
  quit: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M13.5 5.25H8.25A2.25 2.25 0 006 7.5v9a2.25 2.25 0 002.25 2.25h5.25"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M14.25 8.25L18 12m0 0l-3.75 3.75M18 12H9.75"/>',
  minimize:
    '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M5.25 17.25h13.5"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8.25 10.5L12 14.25l3.75-3.75"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 5.25v9"/>',
}

// Timezone
const detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const selectedTimezone = ref(localStorage.getItem('zimaos-blue-timezone') || detectedTimezone)

const timezones = computed(() => {
  try {
    const allTimezones = (
      Intl as unknown as { supportedValuesOf: (key: string) => string[] }
    ).supportedValuesOf('timeZone')
    const filtered = allTimezones.filter((tz: string) => tz !== detectedTimezone)
    return [detectedTimezone, ...filtered]
  } catch {
    return [
      detectedTimezone,
      'UTC',
      'America/New_York',
      'America/Los_Angeles',
      'Europe/London',
      'Europe/Paris',
      'Asia/Tokyo',
      'Asia/Shanghai',
    ]
  }
})

function tWithFallback(key: string, fallback: string) {
  return te(key) ? t(key) : fallback
}

const defaultSettingsTabDescriptions: Record<TabType, string> = {
  general: 'Language, theme, device behavior, network entry, and updates.',
  llm: 'Manage providers, coding runtime, and model routing capabilities.',
  proxy: 'Tune request routing, lightweight helpers, and fallback behavior.',
  speech: 'Configure speech input, output, and voice pipeline features.',
  userdata: 'Control memory, export, backup, and recovery workflows.',
}

const settingsTabs = computed(() =>
  SETTINGS_TABS.map((tab, index) => ({
    id: tab,
    label: t(`settings.tab.${tab}`),
    description: tWithFallback(
      `settings.tabDescriptions.${tab}`,
      defaultSettingsTabDescriptions[tab]
    ),
    badge: tab === 'proxy' ? 'Beta' : '',
    index: String(index + 1).padStart(2, '0'),
  }))
)

const activeTabMeta = computed(
  () => settingsTabs.value.find((tab) => tab.id === activeTab.value) ?? settingsTabs.value[0]!
)

// System Tab - Backup
const backups = ref<BackupInfo[]>([])
const backupsLoading = ref(false)
const backupCreating = ref(false)
const backupRestoring = ref<string | null>(null)
const backupDeleting = ref<string | null>(null)
const generalTabInitialized = ref(false)
const proxyTabInitialized = ref(false)
const agentcoreRunnerSaving = ref(false)
const agentcoreRunnerPreparing = ref(false)
const agentcoreRunnerRefreshing = ref(false)
const agentcoreRunnerStatusExpanded = ref(false)
const agentcoreRunnerRepoURL = ref('')
const agentcoreRunnerRef = ref('')
const agentcoreRunnerTags = ref<AgentcoreRunnerTagList | null>(null)
const agentcoreRunnerTagsLoading = ref(false)
const agentcoreRunnerStatus = computed(() => settingsStore.agentcoreRunnerStatus)
const agentcoreRunnerLastRun = computed(() => settingsStore.agentcoreRunnerLastRun)
const agentcoreRunnerEnabled = computed(() => settingsStore.experimentalAgentcoreRunnerEnabled)
const agentcoreRunnerLastError = computed(() =>
  normalizeAgentcoreRunnerStatusError(agentcoreRunnerStatus.value?.last_error)
)
const agentcoreRunnerHasLastError = computed(() => agentcoreRunnerLastError.value !== '')
const agentcoreRunnerRefOptions = computed(() => {
  const options: string[] = []
  const seen = new Set<string>()
  const push = (value: unknown) => {
    const normalized = normalizeAgentcoreRunnerRefValue(value)
    if (seen.has(normalized)) return
    seen.add(normalized)
    options.push(normalized)
  }
  push(agentcoreRunnerTags.value?.default_ref)
  push(agentcoreRunnerRef.value)
  for (const tag of agentcoreRunnerTags.value?.tags ?? []) {
    push(tag)
  }
  return options
})
const agentcoreRunnerBusy = computed(
  () =>
    agentcoreRunnerSaving.value ||
    agentcoreRunnerPreparing.value ||
    agentcoreRunnerRefreshing.value
)
const agentcoreRunnerTranscriptExpanded = ref(false)
const agentcoreRunnerTranscriptPreviewCount = 2
const agentcoreRunnerLastRunMeta = computed(() => {
  const run = agentcoreRunnerLastRun.value
  if (run == null) return [] as string[]
  return [
    normalizeEvidenceText(run.reason),
    normalizeEvidenceText(run.candidate_id),
    normalizeEvidenceText(run.eval_run_id),
    typeof run.runner_protocol === 'string' ? run.runner_protocol.trim() : '',
    typeof run.runner_stop_reason === 'string' ? run.runner_stop_reason.trim() : '',
    typeof run.optimization_surface === 'string' ? run.optimization_surface.trim() : '',
    formatDurationMs(run.runner_duration_ms),
  ].filter(Boolean)
})
const agentcoreRunnerLastRunTranscriptEntries = computed(() => {
  const entries = agentcoreRunnerLastRun.value?.runner_transcript ?? []
  const suppressed = new Set(
    [
      normalizeEvidenceText(agentcoreRunnerLastRun.value?.runner_response_text),
      normalizeEvidenceText(agentcoreRunnerLastRun.value?.runner_error),
    ].filter(Boolean)
  )
  const seen = new Set<string>()
  return entries.filter((entry) => {
    const text = normalizeEvidenceText(entry.text)
    if (!text) return false
    if (suppressed.has(text)) return false
    const fingerprint = [
      normalizeEvidenceText(entry.direction),
      normalizeEvidenceText(entry.method),
      text,
    ].join('::')
    if (seen.has(fingerprint)) return false
    seen.add(fingerprint)
    return true
  })
})
const agentcoreRunnerVisibleTranscriptEntries = computed(() => {
  if (agentcoreRunnerTranscriptExpanded.value) {
    return agentcoreRunnerLastRunTranscriptEntries.value
  }
  return agentcoreRunnerLastRunTranscriptEntries.value.slice(0, agentcoreRunnerTranscriptPreviewCount)
})
const agentcoreRunnerHiddenTranscriptCount = computed(() =>
  Math.max(
    agentcoreRunnerLastRunTranscriptEntries.value.length - agentcoreRunnerTranscriptPreviewCount,
    0
  )
)

watch(
  [
    () => settingsStore.experimentalAgentcoreRunnerRepoURL,
    () => settingsStore.experimentalAgentcoreRunnerRef,
  ],
  ([repoURL, refValue]) => {
    agentcoreRunnerRepoURL.value = repoURL
    agentcoreRunnerRef.value = normalizeAgentcoreRunnerRefValue(refValue)
  },
  { immediate: true }
)

watch(
  () => agentcoreRunnerLastRun.value?.id,
  () => {
    agentcoreRunnerTranscriptExpanded.value = false
  }
)

function clearSaveStatus() {
  if (saveStatusTimer) {
    clearTimeout(saveStatusTimer)
    saveStatusTimer = null
  }
  saveStatus.value = null
}

function showSaveStatus(message: string, action?: SaveStatusAction) {
  if (saveStatusTimer) {
    clearTimeout(saveStatusTimer)
  }
  saveStatus.value = { message, action }
  saveStatusTimer = setTimeout(
    () => {
      saveStatus.value = null
      saveStatusTimer = null
    },
    action ? 6000 : 2000
  )
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

function closeBehaviorLabel(behavior: CloseBehavior) {
  if (behavior === 'quit') {
    return t('settings.closeBehaviorQuit')
  }

  if (platform.value === 'macos') {
    return tWithFallback('settings.closeBehaviorMinimizeMenuBar', 'Minimize to Menu Bar')
  }

  return t('settings.closeBehaviorMinimize')
}

async function handleMemoryRecallModeChange(mode: MemoryRecallMode) {
  try {
    await settingsStore.setMemoryRecallMode(mode)
    showSaveStatus(t('settings.memoryRecallMode.saved'))
  } catch {
    showSaveStatus(t('settings.memoryRecallMode.saveFailed'))
  }
}

const agentSettingsSaving = ref(false)

async function withAgentSettingsSave(task: () => Promise<void>) {
  if (agentSettingsSaving.value) return
  try {
    agentSettingsSaving.value = true
    await task()
    showSaveStatus(t('settings.saved', 'Saved'))
  } catch {
    showSaveStatus(t('settings.saveFailed', 'Failed to save configuration'))
  } finally {
    agentSettingsSaving.value = false
  }
}

async function handleAgentModeChange(next: boolean) {
  await withAgentSettingsSave(() => settingsStore.setAgentMode(next))
}

async function handleAgentAutoConfirmChange(next: boolean) {
  await withAgentSettingsSave(() => settingsStore.setAgentAutoConfirm(next))
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
const smallModelStatsResetting = ref(false)
const smallModelStatsExpanded = ref(false)
const smallModelDefaultStorageBytes = Math.round(737.5 * 1024 * 1024)
const smallModelRecommendedRuntimeBytes = 2 * 1024 * 1024 * 1024
const globalPrunerConfig = ref<PrunerConfig | null>(null)
const globalPrunerSaving = ref(false)
const globalPrunerEnabled = computed(() => globalPrunerConfig.value?.enabled === true)
const assistantCapabilitiesEnabled = computed(
  () => settingsStore.smallModelIRFeaturesEnabled && (globalPrunerConfig.value?.enabled ?? true)
)
const assistantCapabilitiesSaving = computed(
  () => smallModelSaving.value || globalPrunerSaving.value
)
const shortQASuccessRate = computed(() => {
  const stats = settingsStore.smallModelStats
  if (!stats || stats.short_qa_route_attempts <= 0) return 0
  return Math.round((stats.short_qa_route_success / stats.short_qa_route_attempts) * 100)
})
const imageQASuccessRate = computed(() => {
  const stats = settingsStore.smallModelStats
  const attempts = stats?.image_qa_route_attempts ?? 0
  const success = stats?.image_qa_route_success ?? 0
  if (attempts <= 0) return 0
  return Math.round((success / attempts) * 100)
})
const contextCompressSuccessRate = computed(() => {
  const stats = settingsStore.smallModelStats
  const attempts = stats?.context_compress_attempts ?? 0
  const success = stats?.context_compress_success ?? 0
  if (attempts <= 0) return 0
  return Math.round((success / attempts) * 100)
})
const fallbackReasonEntries = computed(() => {
  const reasons = settingsStore.smallModelStats?.fallback_reasons || {}
  return Object.entries(reasons).sort((a, b) => b[1] - a[1])
})
const contextCompressionModes: ContextCompressionMode[] = ['auto', 'small_model', 'offline']

function parseHumanSizeToBytes(size: string): number {
  const raw = size.trim().replace(/\s+/g, '')
  const match = raw.match(/^([\d.]+)(B|KB|MB|GB|TB)$/i)
  if (!match) return 0
  const value = Number(match[1])
  if (!Number.isFinite(value) || value <= 0) return 0
  const unitPart = match[2]
  if (!unitPart) return 0
  const unit = unitPart.toUpperCase()
  const multipliers: Record<string, number> = {
    B: 1,
    KB: 1024,
    MB: 1024 * 1024,
    GB: 1024 * 1024 * 1024,
    TB: 1024 * 1024 * 1024 * 1024,
  }
  return Math.round(value * (multipliers[unit] ?? 0))
}

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) return (bytes / 1024 / 1024 / 1024).toFixed(1) + ' GB'
  if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return bytes + ' B'
}

const smallModelStorageBytes = computed(() => {
  const files = settingsStore.smallModelStatus?.files
  if (!files || files.length === 0) return smallModelDefaultStorageBytes
  const total = files.reduce((sum, file) => sum + parseHumanSizeToBytes(file.size), 0)
  return total > 0 ? total : smallModelDefaultStorageBytes
})
const smallModelStorageText = computed(() => formatBytes(smallModelStorageBytes.value))
const smallModelRuntimeHintText = computed(() => formatBytes(smallModelRecommendedRuntimeBytes))

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

async function handleSmallModelContextCompressEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelContextCompressEnabled(next))
}

async function handleSmallModelDocExtractEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelDocExtractEnabled(next))
}

async function handleContextCompressionModeChange(mode: ContextCompressionMode) {
  await withSmallModelSave(() => settingsStore.setContextCompressionMode(mode))
}

async function handleSmallModelContextPruneEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelContextPruneEnabled(next))
}

async function handleSmallModelMediaIntentEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelMediaIntentEnabled(next))
}

async function handleOfflineIRFallbackEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setOfflineIRFallbackEnabled(next))
}

async function handleFeatureIntentIREnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setFeatureIntentIREnabled(next))
}

async function saveGlobalPrunerEnabled(next: boolean) {
  const res = await proxyCacheApi.updatePrunerConfig({ enabled: next })
  globalPrunerConfig.value = res.data.config
}

async function handleAssistantCapabilitiesEnabledChange(next: boolean) {
  if (assistantCapabilitiesSaving.value) return
  try {
    smallModelSaving.value = true
    globalPrunerSaving.value = globalPrunerConfig.value != null

    const writes: Promise<unknown>[] = [settingsStore.setSmallModelIRFeaturesEnabled(next)]
    if (globalPrunerConfig.value && globalPrunerEnabled.value !== next) {
      writes.push(saveGlobalPrunerEnabled(next))
    }
    const results = await Promise.allSettled(writes)
    if (results.some((result) => result.status === 'rejected')) {
      await Promise.allSettled([settingsStore.fetchBackendSettings(), fetchGlobalPrunerState()])
      showSaveStatus(t('settings.saveFailed', 'Failed to save configuration'))
      return
    }
    showSaveStatus(t('settings.saved', 'Saved'))
  } finally {
    globalPrunerSaving.value = false
    smallModelSaving.value = false
  }
}

async function handleSmallModelRouteShortQAEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelRouteShortQAEnabled(next))
}

async function handleSmallModelRouteImageQAEnabledChange(next: boolean) {
  await withSmallModelSave(() => settingsStore.setSmallModelRouteImageQAEnabled(next))
}

function formatFallbackReason(reason: string): string {
  return formatSmallModelFallbackReason(reason, t, te)
}

async function fetchSmallModelStats() {
  try {
    await settingsStore.fetchSmallModelStats()
  } catch {
    // ignore
  }
}

async function fetchGlobalPrunerState() {
  const configRes = await proxyCacheApi.getPrunerConfig().catch(() => null)
  if (configRes) globalPrunerConfig.value = configRes.data
}

async function handleGlobalPrunerEnabledChange(next: boolean) {
  if (globalPrunerSaving.value || !globalPrunerConfig.value) return
  try {
    globalPrunerSaving.value = true
    await saveGlobalPrunerEnabled(next)
    showSaveStatus(
      t(
        next ? 'apiProxy.prunerEnabled' : 'apiProxy.prunerDisabled',
        next ? 'Context pruner enabled' : 'Context pruner disabled'
      )
    )
  } catch {
    showSaveStatus(t('settings.saveFailed', 'Failed to save configuration'))
  } finally {
    globalPrunerSaving.value = false
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

async function fetchServiceInfo() {
  try {
    const res = await serviceApi.getInfo()
    serviceInfo.value = res.data
  } catch {
    /* service API not available */
  }
}

async function ensureGeneralTabDataLoaded() {
  if (generalTabInitialized.value) return
  generalTabInitialized.value = true
  await fetchServiceInfo()
}

async function ensureProxyTabDataLoaded() {
  if (proxyTabInitialized.value) return
  proxyTabInitialized.value = true

  const tasks: Promise<unknown>[] = []
  if (settingsStore.smallModelStatus == null) {
    tasks.push(fetchSmallModelStatus())
  }
  if (settingsStore.smallModelStats == null) {
    tasks.push(fetchSmallModelStats())
  }
  if (globalPrunerConfig.value == null) {
    tasks.push(fetchGlobalPrunerState())
  }
  if (settingsStore.agentcoreRunnerStatus == null) {
    tasks.push(fetchAgentcoreRunnerStatus())
  } else if (
    settingsStore.agentcoreRunnerStatus.last_optimization_run_id &&
    settingsStore.agentcoreRunnerLastRun == null
  ) {
    tasks.push(fetchAgentcoreRunnerLastRun())
  }
  tasks.push(fetchAgentcoreRunnerTags())

  if (tasks.length > 0) {
    await Promise.allSettled(tasks)
  }
}

function formatStatusTime(value?: string) {
  if (!value) return t('settings.agentcoreRunner.empty', 'Not available')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function formatDurationMs(value?: number) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return ''
  if (value < 1000) return `${Math.round(value)}ms`
  return `${(value / 1000).toFixed(1)}s`
}

function normalizeEvidenceText(value: unknown) {
  if (typeof value !== 'string') return ''
  return value.trim()
}

function normalizeAgentcoreRunnerStatusError(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text.toLowerCase() === 'repo url is required') {
    return ''
  }
  return text
}

function normalizeAgentcoreRunnerRepoURLValue(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text) return text
  return 'https://github.com/IceWhaleTech/ZimaOS-Blue'
}

function normalizeAgentcoreRunnerRefValue(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text) return text
  return 'main'
}

async function fetchAgentcoreRunnerStatus() {
  try {
    await settingsStore.fetchAgentcoreRunnerStatus()
  } catch {
    // ignore
  }
}

async function fetchAgentcoreRunnerLastRun() {
  try {
    await settingsStore.fetchAgentcoreRunnerLastRun()
  } catch {
    // ignore
  }
}

async function fetchAgentcoreRunnerTags(repoURL = agentcoreRunnerRepoURL.value) {
  const resolvedRepoURL = normalizeAgentcoreRunnerRepoURLValue(repoURL)
  try {
    agentcoreRunnerTagsLoading.value = true
    const response = await settingsApi.getAgentcoreRunnerTags(resolvedRepoURL)
    agentcoreRunnerTags.value = response.data
  } catch {
    agentcoreRunnerTags.value = {
      repo_url: resolvedRepoURL,
      default_ref: 'main',
      tags: [],
    }
  } finally {
    agentcoreRunnerTagsLoading.value = false
  }
}

async function refreshAgentcoreRunnerStatus() {
  if (agentcoreRunnerRefreshing.value) return
  try {
    agentcoreRunnerRefreshing.value = true
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags()])
  } finally {
    agentcoreRunnerRefreshing.value = false
  }
}

async function saveAgentcoreRunnerConfig() {
  if (agentcoreRunnerSaving.value) return
  const repoURL = normalizeAgentcoreRunnerRepoURLValue(agentcoreRunnerRepoURL.value)
  const refValue = normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value)
  agentcoreRunnerRepoURL.value = repoURL
  agentcoreRunnerRef.value = refValue
  try {
    agentcoreRunnerSaving.value = true
    await settingsStore.updateBackendSettings({
      experimental_agentcore_runner_repo_url: repoURL,
      experimental_agentcore_runner_ref: refValue,
    })
    showSaveStatus(t('settings.saved', 'Saved'))
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags(repoURL)])
  } catch {
    showSaveStatus(t('settings.saveFailed', 'Failed to save configuration'))
  } finally {
    agentcoreRunnerSaving.value = false
  }
}

async function handleAgentcoreRunnerEnabledChange(next: boolean) {
  if (agentcoreRunnerSaving.value) return
  try {
    agentcoreRunnerSaving.value = true
    await settingsStore.setExperimentalAgentcoreRunnerEnabled(next)
    showSaveStatus(t('settings.saved', 'Saved'))
    await fetchAgentcoreRunnerStatus()
  } catch {
    showSaveStatus(t('settings.saveFailed', 'Failed to save configuration'))
  } finally {
    agentcoreRunnerSaving.value = false
  }
}

async function prepareAgentcoreRunner() {
  if (agentcoreRunnerPreparing.value) return
  const repoURL = normalizeAgentcoreRunnerRepoURLValue(agentcoreRunnerRepoURL.value)
  const refValue = normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value)
  agentcoreRunnerRepoURL.value = repoURL
  agentcoreRunnerRef.value = refValue
  try {
    agentcoreRunnerPreparing.value = true
    await settingsStore.updateBackendSettings({
      experimental_agentcore_runner_repo_url: repoURL,
      experimental_agentcore_runner_ref: refValue,
    })
    await settingsStore.prepareAgentcoreRunner()
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags(repoURL)])
    showSaveStatus(
      t('settings.agentcoreRunner.prepareSuccess', 'Agentcore Runner prepared successfully')
    )
  } catch {
    showSaveStatus(
      t('settings.agentcoreRunner.prepareFailed', 'Failed to prepare Agentcore Runner')
    )
  } finally {
    agentcoreRunnerPreparing.value = false
  }
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

async function switchTab(tab: TabType) {
  activeTab.value = tab
  router.replace({ query: { ...route.query, tab } })

  // Load data for specific tabs
  if (tab === 'general') {
    void ensureGeneralTabDataLoaded()
  }
  if (tab === 'userdata' && backups.value.length === 0) {
    fetchBackups()
  }
  if (tab === 'proxy') {
    await ensureProxyTabDataLoaded()
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
  if (backupCreating.value) return
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
  void createBackup()
}

function onBackupDownload(_id: string) {
  // Download not implemented in API yet
}

async function restoreBackup(id: string) {
  if (backupRestoring.value) return
  if (!confirm(t('system.confirmRestore'))) return
  backupRestoring.value = id
  try {
    const desktopManagedRestart = isTauri.value
    const response = await backupApi.restore(id, {
      require_restart: true,
      auto_restart: !desktopManagedRestart,
      create_checkpoint: true,
    })

    if (desktopManagedRestart && response.data?.requires_restart) {
      const restarted = await restartServerRuntime(route.fullPath)
      if (!restarted) {
        throw new Error('desktop-managed restore restart failed')
      }
      return
    }

    const checkpointAt = response.data?.result?.checkpoint_at
    if (checkpointAt) {
      showSaveStatus(
        `${response.data?.message || t('system.backupRestored')} (checkpoint: ${new Date(checkpointAt).toLocaleString()})`
      )
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
  if (backupDeleting.value) return
  if (!confirm(t('system.confirmDeleteBackup'))) return
  backupDeleting.value = id
  try {
    await backupApi.delete(id)
    backups.value = backups.value.filter((b) => b.id !== id)
    showSaveStatus(t('system.backupDeleted'))
  } catch (e) {
    console.error('Failed to delete backup:', e)
  } finally {
    backupDeleting.value = null
  }
}

onMounted(async () => {
  await settingsStore.fetchBackendSettings()
  if (requestedInitialTab === 'general') {
    void ensureGeneralTabDataLoaded()
  }
  if (requestedInitialTab === 'proxy') {
    await ensureProxyTabDataLoaded()
  }

  // Load data based on initial tab
  if (hasInitialTabQuery) {
    await switchTab(requestedInitialTab)
  }
})

onUnmounted(() => {
  clearSaveStatus()
  stopSmallModelPoll()
})
</script>

<template>
  <div class="settings-view dashboard-page-frame">
    <section class="settings-stage dashboard-page-stage configuration-page-stage">
      <div class="settings-shell">
        <Transition name="notification">
          <div v-if="saveStatus" class="settings-toast">
            <span class="settings-toast__message">{{ saveStatus.message }}</span>
            <button
              v-if="saveStatus.action"
              type="button"
              class="settings-toast__action"
              @click="saveStatus.action.handler"
            >
              {{ saveStatus.action.label }}
            </button>
          </div>
        </Transition>

        <header class="settings-hero dashboard-page-hero configuration-page-hero">
          <div class="settings-hero__copy dashboard-page-copy configuration-page-copy">
            <span class="settings-hero__eyebrow dashboard-page-eyebrow">
              {{ t('nav.configuration') }}
            </span>
            <h1 class="settings-hero__title dashboard-page-title configuration-page-title">
              {{ t('settings.title') }}
            </h1>
            <p
              class="settings-hero__description dashboard-page-description configuration-page-description"
            >
              {{ activeTabMeta.description }}
            </p>
          </div>
        </header>

        <nav
          class="settings-tab-nav dashboard-card-surface settings-surface-card"
          aria-label="Settings sections"
        >
          <button
            v-for="tab in settingsTabs"
            :key="tab.id"
            type="button"
            class="settings-tab-button dashboard-card-subsurface"
            :class="{ 'settings-tab-button--active': activeTab === tab.id }"
            :data-tab="tab.id"
            @click="switchTab(tab.id)"
          >
            <span class="settings-tab-button__icon">
              <svg
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                v-html="tabIcons[tab.id]"
              />
            </span>
            <span class="settings-tab-button__body">
              <span class="settings-tab-button__label-row">
                <span class="settings-tab-button__label">{{ tab.label }}</span>
                <span v-if="tab.badge" class="settings-tab-beta">{{ tab.badge }}</span>
              </span>
            </span>
            <span class="settings-tab-button__state" aria-hidden="true"></span>
          </button>
        </nav>

        <div
          v-if="activeTab === 'general'"
          class="settings-panel dashboard-card-surface settings-surface-card"
        >
          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.workspaceBasics', '基础设置')
                }}</span>
                <h2 class="settings-module__title">{{ t('settings.tab.general') }}</h2>
              </div>
            </div>

            <div class="settings-card-grid">
              <div class="dashboard-card-subsurface settings-field-card">
                <div class="settings-card-heading">
                  <label class="settings-field-label">{{ t('common.language') }}</label>
                </div>
                <select
                  :value="localeStore.currentLocale"
                  class="settings-select"
                  @change="handleLocaleChange(($event.target as HTMLSelectElement).value)"
                >
                  <option
                    v-for="option in localeStore.options"
                    :key="option.value"
                    :value="option.value"
                  >
                    {{ option.label }}
                  </option>
                </select>
              </div>

              <div class="dashboard-card-subsurface settings-field-card">
                <div class="settings-card-heading">
                  <label class="settings-field-label">{{ t('settings.timezone') }}</label>
                </div>
                <select
                  :value="selectedTimezone"
                  class="settings-select"
                  @change="handleTimezoneChange(($event.target as HTMLSelectElement).value)"
                >
                  <option v-for="tz in timezones" :key="tz" :value="tz">{{ tz }}</option>
                </select>
              </div>

              <div class="dashboard-card-subsurface settings-field-card">
                <div class="settings-card-heading">
                  <label class="settings-field-label">{{ t('common.theme') }}</label>
                </div>
                <div class="settings-theme-group" role="group" :aria-label="t('common.theme')">
                  <button
                    v-for="theme in themeOptions"
                    :key="theme"
                    type="button"
                    class="settings-theme-button"
                    :class="{ 'settings-theme-button--active': themeStore.theme === theme }"
                    :title="t(`common.${theme}`)"
                    :aria-label="t(`common.${theme}`)"
                    :aria-pressed="themeStore.theme === theme"
                    @click="themeStore.setTheme(theme)"
                  >
                    <svg
                      class="settings-theme-button__icon"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      v-html="themeIcons[theme]"
                    />
                  </button>
                </div>
              </div>

              <div v-if="isTauri" class="dashboard-card-subsurface settings-field-card">
                <div class="settings-card-heading">
                  <label class="settings-field-label">{{ t('settings.closeBehavior') }}</label>
                </div>
                <div
                  class="settings-pill-group settings-pill-group--icon-only"
                  role="group"
                  :aria-label="t('settings.closeBehavior')"
                >
                  <button
                    v-for="behavior in ['quit', 'minimize'] as const"
                    :key="behavior"
                    type="button"
                    class="settings-pill-button"
                    :class="{
                      'settings-pill-button--active': settingsStore.closeBehavior === behavior,
                      'settings-pill-button--icon-only': true,
                    }"
                    :title="closeBehaviorLabel(behavior)"
                    :aria-label="closeBehaviorLabel(behavior)"
                    :aria-pressed="settingsStore.closeBehavior === behavior"
                    @click="handleCloseBehaviorChange(behavior)"
                  >
                    <span class="settings-pill-button__icon-shell" aria-hidden="true">
                      <svg
                        class="settings-pill-button__icon"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        v-html="closeBehaviorIcons[behavior]"
                      />
                    </span>
                  </button>
                </div>
              </div>

              <div v-if="serviceInfo" class="dashboard-card-subsurface settings-field-card">
                <div class="settings-field-card__row">
                  <div class="settings-card-heading">
                    <label class="settings-field-label">{{ t('service.autoStart') }}</label>
                    <p class="settings-field-hint">{{ t('service.autoStartDescription') }}</p>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    :aria-checked="autoStartEnabled"
                    :disabled="autoStartLoading"
                    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      autoStartEnabled
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="toggleAutoStart"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="autoStartEnabled ? 'translate-x-5' : 'translate-x-0'"
                    />
                  </button>
                </div>
              </div>
            </div>
          </section>

          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.networkSurface', '网络入口')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.networkTitle', '网络与访问') }}
                </h2>
              </div>
            </div>
            <NetworkSettings
              :show-port-section="true"
              :show-security-sections="false"
              @status-change="showSaveStatus"
            />
          </section>

          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.releaseTrack', '版本管理')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.systemVersion', '更新与版本') }}
                </h2>
              </div>
            </div>
            <div class="dashboard-card-subsurface settings-field-card settings-field-card--flush">
              <UpdateSettings />
            </div>
          </section>
        </div>

        <div
          v-if="activeTab === 'llm'"
          class="settings-panel dashboard-card-surface settings-surface-card"
        >
          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.providerMatrix', '模型来源')
                }}</span>
                <h2 class="settings-module__title">{{ t('settings.tab.llm') }}</h2>
              </div>
            </div>
            <div class="dashboard-card-subsurface settings-field-card settings-field-card--flush">
              <ProviderPoolSection />
            </div>
          </section>

          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.externalAgents.eyebrow')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.externalAgents.title') }}
                </h2>
              </div>
            </div>
            <div class="dashboard-card-subsurface settings-field-card settings-field-card--flush">
              <ExternalAgentsSection />
            </div>
          </section>

          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.codingRuntime', '编码能力')
                }}</span>
                <h2 class="settings-module__title">{{ t('chat.taskLoop') }}</h2>
              </div>
            </div>

            <div class="settings-card-grid">
              <div class="dashboard-card-subsurface settings-field-card">
                <div class="settings-field-card__row">
                  <div class="settings-card-heading">
                    <label class="settings-field-label">{{ t('agent.mode') }}</label>
                    <p class="settings-field-hint">{{ t('agent.modeDescription') }}</p>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    :aria-checked="settingsStore.agentMode"
                    :disabled="agentSettingsSaving"
                    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      settingsStore.agentMode
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="handleAgentModeChange(!settingsStore.agentMode)"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="settingsStore.agentMode ? 'translate-x-5' : 'translate-x-0'"
                    />
                  </button>
                </div>
              </div>

              <div class="dashboard-card-subsurface settings-field-card">
                <div class="settings-field-card__row">
                  <div class="settings-card-heading">
                    <label class="settings-field-label">{{ t('agent.autoConfirm') }}</label>
                    <p class="settings-field-hint">{{ t('agent.autoConfirmDescription') }}</p>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    :aria-checked="settingsStore.agentAutoConfirm"
                    :disabled="agentSettingsSaving"
                    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      settingsStore.agentAutoConfirm
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="handleAgentAutoConfirmChange(!settingsStore.agentAutoConfirm)"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="settingsStore.agentAutoConfirm ? 'translate-x-5' : 'translate-x-0'"
                    />
                  </button>
                </div>
              </div>
            </div>
          </section>
        </div>

        <div
          v-if="activeTab === 'proxy'"
          class="settings-panel dashboard-card-surface settings-surface-card"
        >
          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.requestFlow', '请求流转')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.proxyRouting', '代理与切换') }}
                </h2>
              </div>
            </div>
            <ApiProxySettings @status-change="showSaveStatus" />
          </section>

          <section class="settings-module" data-testid="agentcore-runner-card">
            <div class="settings-module__header">
              <div class="space-y-1.5">
                <span class="settings-module__eyebrow inline-flex w-fit">{{
                  t('settings.agentcoreRunner.eyebrow', 'Harness · Beta')
                }}</span>
                <h2 class="settings-module__title">
                  {{
                    t(
                      'settings.agentcoreRunner.title',
                      'Harness Self-Iterating Agentcore Runner'
                    )
                  }}
                </h2>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{
                    t(
                      'settings.agentcoreRunner.description',
                      'Use Harness to iterate on an Agentcore runner by preparing a standalone runner from a public GitHub repo for local build, evaluation, and optimisation.'
                    )
                  }}
                </p>
              </div>
            </div>

            <div class="dashboard-card-subsurface settings-feature-card p-4 space-y-4">
              <div class="settings-field-card__row">
                <div class="settings-card-heading">
                  <label class="settings-field-label">{{
                    t('settings.agentcoreRunner.enabled', 'Enable Agentcore Runner')
                  }}</label>
                  <p class="settings-field-hint">
                    {{
                      t(
                        'settings.agentcoreRunner.enabledHint',
                        'Allow Harness beta flows to prepare and reuse a managed local runner for self-iteration.'
                      )
                    }}
                  </p>
                </div>
                <button
                  data-testid="agentcore-runner-enabled-switch"
                  type="button"
                  role="switch"
                  :aria-checked="agentcoreRunnerEnabled"
                  :disabled="agentcoreRunnerBusy"
                  class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                  :class="
                    agentcoreRunnerEnabled
                      ? 'bg-green-600 dark:bg-green-500'
                      : 'bg-gray-300 dark:bg-gray-600'
                  "
                  @click="handleAgentcoreRunnerEnabledChange(!agentcoreRunnerEnabled)"
                >
                  <span
                    class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                    :class="agentcoreRunnerEnabled ? 'translate-x-5' : 'translate-x-0'"
                  />
                </button>
              </div>

              <div class="grid gap-3 md:grid-cols-[minmax(0,1fr),180px]">
                <label class="block">
                  <span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
                    {{ t('settings.agentcoreRunner.repoUrl', 'GitHub Repo URL') }}
                  </span>
                  <input
                    data-testid="agentcore-runner-repo-input"
                    v-model="agentcoreRunnerRepoURL"
                    type="text"
                    class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-green-500 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-100"
                    :placeholder="
                      t(
                        'settings.agentcoreRunner.repoPlaceholder',
                        'https://github.com/owner/repo or owner/repo'
                      )
                    "
                    @blur="saveAgentcoreRunnerConfig"
                  />
                </label>

                <label class="block">
                  <span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
                    {{ t('settings.agentcoreRunner.ref', 'Ref') }}
                  </span>
                  <select
                    data-testid="agentcore-runner-ref-input"
                    v-model="agentcoreRunnerRef"
                    class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-green-500 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-100"
                    :disabled="agentcoreRunnerSaving"
                    @change="saveAgentcoreRunnerConfig"
                  >
                    <option v-for="option in agentcoreRunnerRefOptions" :key="option" :value="option">
                      {{ option }}
                    </option>
                  </select>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      agentcoreRunnerTagsLoading
                        ? t('settings.agentcoreRunner.refLoading', 'Loading tags...')
                        : t('settings.agentcoreRunner.refHint', 'Defaults to main and lists tags from the selected repo.')
                    }}
                  </p>
                </label>
              </div>

              <div class="flex items-center justify-between gap-3">
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{
                    t(
                      'settings.agentcoreRunner.prepareHint',
                      'Prepare downloads the repo, installs the required Go toolchain, and builds ./cmd/agentcore-runner in the managed cache.'
                    )
                  }}
                </p>
                <div class="flex items-center gap-2">
                  <button
                    data-testid="agentcore-runner-refresh"
                    type="button"
                    class="rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-slate-800"
                    :disabled="agentcoreRunnerBusy"
                    @click="refreshAgentcoreRunnerStatus"
                  >
                    {{ t('common.refresh', 'Refresh') }}
                  </button>
                  <button
                    data-testid="agentcore-runner-prepare"
                    type="button"
                    class="rounded-lg bg-green-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-green-500 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="agentcoreRunnerBusy"
                    @click="prepareAgentcoreRunner"
                  >
                    {{
                      agentcoreRunnerPreparing
                        ? t('settings.agentcoreRunner.preparing', 'Preparing...')
                        : t('settings.agentcoreRunner.prepare', 'Prepare Runner')
                    }}
                  </button>
                </div>
              </div>

              <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-slate-900/60">
                <button
                  data-testid="agentcore-runner-status-toggle"
                  type="button"
                  class="flex w-full items-center justify-between gap-3 text-left"
                  @click="agentcoreRunnerStatusExpanded = !agentcoreRunnerStatusExpanded"
                >
                  <div class="text-sm font-medium text-gray-800 dark:text-gray-100">
                    {{ t('settings.agentcoreRunner.status', 'Status') }}
                  </div>
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{
                    agentcoreRunnerStatusExpanded
                      ? t('settings.smallModel.collapse', 'Collapse')
                      : t('settings.smallModel.expand', 'Expand')
                  }}</span>
                </button>
                <div
                  v-if="agentcoreRunnerStatusExpanded"
                  data-testid="agentcore-runner-status-content"
                  class="mt-3 grid gap-2 sm:grid-cols-2"
                >
                  <div class="text-sm">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.resolvedCommit', 'Resolved commit')
                    }}</span>
                    <div
                      data-testid="agentcore-runner-resolved-commit"
                      class="text-gray-900 dark:text-gray-100 break-all"
                    >
                      {{
                        agentcoreRunnerStatus?.resolved_commit ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                  </div>
                  <div class="text-sm">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.requiredGoVersion', 'Required Go version')
                    }}</span>
                    <div data-testid="agentcore-runner-required-go" class="text-gray-900 dark:text-gray-100">
                      {{
                        agentcoreRunnerStatus?.required_go_version ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                  </div>
                  <div class="text-sm">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.installedGoVersion', 'Installed Go version')
                    }}</span>
                    <div
                      data-testid="agentcore-runner-installed-go"
                      class="text-gray-900 dark:text-gray-100"
                    >
                      {{
                        agentcoreRunnerStatus?.installed_go_version ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                  </div>
                  <div class="text-sm">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.toolchainReady', 'Toolchain ready')
                    }}</span>
                    <div data-testid="agentcore-runner-toolchain-ready" class="text-gray-900 dark:text-gray-100">
                      {{
                        agentcoreRunnerStatus?.toolchain_ready
                          ? t('common.yes', 'Yes')
                          : t('common.no', 'No')
                      }}
                    </div>
                  </div>
                  <div class="text-sm">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.binaryReady', 'Binary ready')
                    }}</span>
                    <div data-testid="agentcore-runner-binary-ready" class="text-gray-900 dark:text-gray-100">
                      {{
                        agentcoreRunnerStatus?.binary_ready
                          ? t('common.yes', 'Yes')
                          : t('common.no', 'No')
                      }}
                    </div>
                  </div>
                  <div class="text-sm">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.lastPrepareState', 'Last prepare state')
                    }}</span>
                    <div
                      data-testid="agentcore-runner-last-prepare-state"
                      class="text-gray-900 dark:text-gray-100"
                    >
                      {{
                        agentcoreRunnerStatus?.last_prepare_state ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                  </div>
                  <div class="text-sm sm:col-span-2">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.binaryPath', 'Binary path')
                    }}</span>
                    <div
                      data-testid="agentcore-runner-binary-path"
                      class="text-gray-900 dark:text-gray-100 break-all"
                    >
                      {{
                        agentcoreRunnerStatus?.binary_path ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                  </div>
                  <div class="text-sm sm:col-span-2">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.binaryChecksum', 'Binary checksum')
                    }}</span>
                    <div
                      data-testid="agentcore-runner-binary-checksum"
                      class="text-gray-900 dark:text-gray-100 break-all"
                    >
                      {{
                        agentcoreRunnerStatus?.binary_sha256 ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                  </div>
                  <div class="text-sm">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.lastPrepareAt', 'Last prepare time')
                    }}</span>
                    <div class="text-gray-900 dark:text-gray-100">
                      {{ formatStatusTime(agentcoreRunnerStatus?.last_prepare_at) }}
                    </div>
                  </div>
                  <div class="text-sm">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.lastOptimizationRunId', 'Last optimization run ID')
                    }}</span>
                    <div
                      data-testid="agentcore-runner-last-optimization-run-id"
                      class="text-gray-900 dark:text-gray-100 break-all"
                    >
                      {{
                        agentcoreRunnerStatus?.last_optimization_run_id ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                    <div
                      data-testid="agentcore-runner-last-optimization-state"
                      class="mt-1 text-xs text-gray-600 dark:text-gray-300"
                    >
                      {{
                        agentcoreRunnerStatus?.last_optimization_state ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                    <div
                      data-testid="agentcore-runner-last-optimization-time"
                      class="text-xs text-gray-500 dark:text-gray-400"
                    >
                      {{ formatStatusTime(agentcoreRunnerStatus?.last_optimization_at) }}
                    </div>
                    <div
                      data-testid="agentcore-runner-last-optimization-summary"
                      class="mt-1 text-xs text-gray-700 dark:text-gray-200 break-words"
                    >
                      {{
                        agentcoreRunnerStatus?.last_optimization_summary ||
                        t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                    <div
                      v-if="agentcoreRunnerLastRun"
                      data-testid="agentcore-runner-last-run-detail"
                      class="mt-2 space-y-2 rounded-lg border border-gray-200 bg-white/80 p-3 dark:border-gray-700 dark:bg-slate-950/50"
                    >
                      <div
                        v-if="agentcoreRunnerLastRunMeta.length > 0"
                        data-testid="agentcore-runner-last-run-meta"
                        class="flex flex-wrap gap-1.5"
                      >
                        <span
                          v-for="item in agentcoreRunnerLastRunMeta"
                          :key="item"
                          class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-slate-800 dark:text-gray-200"
                        >
                          {{ item }}
                        </span>
                      </div>
                      <div
                        v-if="agentcoreRunnerLastRun?.runner_error"
                        data-testid="agentcore-runner-last-run-error"
                        class="rounded-lg bg-red-50 px-3 py-2 text-[11px] leading-5 text-red-700 whitespace-pre-wrap dark:bg-red-950/30 dark:text-red-300"
                      >
                        {{ agentcoreRunnerLastRun.runner_error }}
                      </div>
                      <div
                        v-if="agentcoreRunnerLastRun?.runner_response_text"
                        data-testid="agentcore-runner-last-run-response"
                        class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-[11px] leading-5 text-gray-700 whitespace-pre-wrap dark:border-gray-700 dark:bg-slate-900 dark:text-gray-200"
                      >
                        {{ agentcoreRunnerLastRun.runner_response_text }}
                      </div>
                      <div
                        v-if="agentcoreRunnerLastRunTranscriptEntries.length > 0"
                        data-testid="agentcore-runner-last-run-transcript"
                        class="max-h-48 space-y-2 overflow-auto"
                      >
                        <div
                          v-for="(entry, index) in agentcoreRunnerVisibleTranscriptEntries"
                          :key="`${entry.direction || 'run'}-${entry.method || 'message'}-${index}`"
                          class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-700 dark:bg-slate-900/70"
                        >
                          <div class="flex flex-wrap gap-1.5 text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                            <span>{{ entry.direction || 'run' }}</span>
                            <span v-if="entry.method">{{ entry.method }}</span>
                          </div>
                          <div class="mt-1 text-[11px] leading-5 text-gray-700 whitespace-pre-wrap dark:text-gray-200">
                            {{ entry.text }}
                          </div>
                        </div>
                      </div>
                      <button
                        v-if="agentcoreRunnerHiddenTranscriptCount > 0"
                        data-testid="agentcore-runner-transcript-toggle"
                        type="button"
                        class="text-xs font-medium text-gray-500 transition hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                        :aria-expanded="agentcoreRunnerTranscriptExpanded ? 'true' : 'false'"
                        @click="agentcoreRunnerTranscriptExpanded = !agentcoreRunnerTranscriptExpanded"
                      >
                        {{
                          agentcoreRunnerTranscriptExpanded
                            ? t('settings.smallModel.collapse', 'Collapse')
                            : t('settings.smallModel.expand', 'Expand')
                        }}
                      </button>
                    </div>
                  </div>
                  <div class="text-sm sm:col-span-2">
                    <span class="text-gray-500 dark:text-gray-400">{{
                      t('settings.agentcoreRunner.lastError', 'Last error')
                    }}</span>
                    <div
                      data-testid="agentcore-runner-last-error"
                      class="break-all"
                      :class="
                        agentcoreRunnerHasLastError
                          ? 'text-red-600 dark:text-red-400'
                          : 'text-gray-500 dark:text-gray-400'
                      "
                    >
                      {{
                        agentcoreRunnerLastError || t('settings.agentcoreRunner.empty', 'Not available')
                      }}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.assistiveRouting', '辅助策略')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.smallModel.irTitle', '辅助功能') }}
                </h2>
              </div>
            </div>

            <div class="dashboard-card-subsurface settings-feature-card p-4">
              <div
                data-testid="small-model-ir-section-header"
                class="mb-4 flex items-start justify-between gap-3"
              >
                <div>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                    {{
                      t(
                        'settings.smallModel.irDesc',
                        'User-facing helpers for context control and tool filtering.'
                      )
                    }}
                  </p>
                </div>
                <button
                  data-testid="small-model-ir-master-switch"
                  type="button"
                  role="switch"
                  :aria-checked="assistantCapabilitiesEnabled"
                  :aria-label="t('settings.smallModel.irMasterTitle', 'Master Switch')"
                  :disabled="assistantCapabilitiesSaving"
                  class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                  :class="
                    assistantCapabilitiesEnabled
                      ? 'bg-green-600 dark:bg-green-500'
                      : 'bg-gray-300 dark:bg-gray-600'
                  "
                  @click="handleAssistantCapabilitiesEnabledChange(!assistantCapabilitiesEnabled)"
                >
                  <span
                    class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                    :class="assistantCapabilitiesEnabled ? 'translate-x-5' : 'translate-x-0'"
                  />
                </button>
              </div>

              <div
                data-testid="small-model-ir-grid"
                class="grid grid-cols-1 gap-3 rounded-lg bg-gray-50 px-3 py-2.5 dark:bg-gray-700/30 sm:grid-cols-2"
              >
                <div
                  class="flex h-full items-start justify-between gap-3 rounded-lg border border-gray-200 bg-white px-2.5 py-2 dark:border-gray-700 dark:bg-slate-800/50"
                >
                  <div class="min-w-0 flex-1">
                    <div class="text-sm text-gray-800 dark:text-gray-100">
                      {{ t('settings.smallModel.irContextPruneTitle', 'Chat Context Compaction') }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      {{
                        t(
                          'settings.smallModel.irContextPruneDesc',
                          'Controls how Blue reduces chat history when context pressure rises.'
                        )
                      }}
                    </div>
                  </div>
                  <button
                    data-testid="small-model-context-prune-switch"
                    type="button"
                    role="switch"
                    :aria-checked="settingsStore.smallModelContextPruneEnabled"
                    :disabled="smallModelSaving"
                    class="relative mt-0.5 inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      settingsStore.smallModelContextPruneEnabled
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="
                      handleSmallModelContextPruneEnabledChange(
                        !settingsStore.smallModelContextPruneEnabled
                      )
                    "
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="
                        settingsStore.smallModelContextPruneEnabled
                          ? 'translate-x-5'
                          : 'translate-x-0'
                      "
                    />
                  </button>
                </div>

                <div
                  v-if="globalPrunerConfig"
                  class="flex h-full items-start justify-between gap-3 rounded-lg border border-gray-200 bg-white px-2.5 py-2 dark:border-gray-700 dark:bg-slate-800/50"
                >
                  <div class="min-w-0 flex-1">
                    <div class="text-sm text-gray-800 dark:text-gray-100">
                      {{ t('apiProxy.prunerTitle', 'Global Context Pruner') }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      {{
                        t(
                          'apiProxy.prunerDesc',
                          'Controls the API proxy pruner for proxied /v1 requests. Blue chat may still skip pruning per request when context pressure is low.'
                        )
                      }}
                    </div>
                  </div>
                  <button
                    data-testid="proxy-pruner-switch"
                    type="button"
                    role="switch"
                    :aria-checked="globalPrunerEnabled"
                    :disabled="globalPrunerSaving"
                    class="relative mt-0.5 inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      globalPrunerEnabled
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="handleGlobalPrunerEnabledChange(!globalPrunerEnabled)"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="globalPrunerEnabled ? 'translate-x-5' : 'translate-x-0'"
                    />
                  </button>
                </div>

                <div
                  class="flex h-full items-start justify-between gap-3 rounded-lg border border-gray-200 bg-white px-2.5 py-2 dark:border-gray-700 dark:bg-slate-800/50"
                >
                  <div class="min-w-0 flex-1">
                    <div class="text-sm text-gray-800 dark:text-gray-100">
                      {{
                        t(
                          'settings.smallModel.mediaIntent',
                          'Media Generation Scenario Recognition'
                        )
                      }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      {{
                        t(
                          'settings.smallModel.irMediaIntentDesc',
                          'Detects media-generation intent to route requests more accurately.'
                        )
                      }}
                    </div>
                  </div>
                  <button
                    data-testid="small-model-media-intent-switch"
                    type="button"
                    role="switch"
                    :aria-checked="settingsStore.smallModelMediaIntentEnabled"
                    :disabled="smallModelSaving"
                    class="relative mt-0.5 inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      settingsStore.smallModelMediaIntentEnabled
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="
                      handleSmallModelMediaIntentEnabledChange(
                        !settingsStore.smallModelMediaIntentEnabled
                      )
                    "
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="
                        settingsStore.smallModelMediaIntentEnabled
                          ? 'translate-x-5'
                          : 'translate-x-0'
                      "
                    />
                  </button>
                </div>

                <div
                  class="flex h-full items-start justify-between gap-3 rounded-lg border border-gray-200 bg-white px-2.5 py-2 dark:border-gray-700 dark:bg-slate-800/50"
                >
                  <div class="min-w-0 flex-1">
                    <div class="text-sm text-gray-800 dark:text-gray-100">
                      {{
                        t('settings.smallModel.irOfflineFallbackTitle', 'Offline Local Fallback')
                      }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      {{
                        t(
                          'settings.smallModel.irOfflineFallbackDesc',
                          'When model fallback is needed, answer from local context recall first.'
                        )
                      }}
                    </div>
                  </div>
                  <button
                    data-testid="offline-ir-fallback-switch"
                    type="button"
                    role="switch"
                    :aria-checked="settingsStore.offlineIRFallbackEnabled"
                    :disabled="smallModelSaving"
                    class="relative mt-0.5 inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      settingsStore.offlineIRFallbackEnabled
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="
                      handleOfflineIRFallbackEnabledChange(!settingsStore.offlineIRFallbackEnabled)
                    "
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="
                        settingsStore.offlineIRFallbackEnabled ? 'translate-x-5' : 'translate-x-0'
                      "
                    />
                  </button>
                </div>

                <div
                  class="flex h-full items-start justify-between gap-3 rounded-lg border border-gray-200 bg-white px-2.5 py-2 dark:border-gray-700 dark:bg-slate-800/50"
                >
                  <div class="min-w-0 flex-1">
                    <div class="text-sm text-gray-800 dark:text-gray-100">
                      {{ t('settings.smallModel.irFeatureHintTitle', 'Feature Hint Detection') }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      {{
                        t('settings.smallModel.irFeatureHintDesc', {
                          deepResearch: te('ui.deepResearchTitle')
                            ? t('ui.deepResearchTitle')
                            : 'Deep Research',
                          agentMode: t('chat.taskLoop'),
                        })
                      }}
                    </div>
                  </div>
                  <button
                    data-testid="feature-intent-ir-switch"
                    type="button"
                    role="switch"
                    :aria-checked="settingsStore.featureIntentIREnabled"
                    :disabled="smallModelSaving"
                    class="relative mt-0.5 inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      settingsStore.featureIntentIREnabled
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="
                      handleFeatureIntentIREnabledChange(!settingsStore.featureIntentIREnabled)
                    "
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="
                        settingsStore.featureIntentIREnabled ? 'translate-x-5' : 'translate-x-0'
                      "
                    />
                  </button>
                </div>
              </div>
            </div>
          </section>

          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.localAcceleration', '本地加速')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.smallModel.title', '轻量加速') }}
                </h2>
              </div>
            </div>

            <div class="dashboard-card-subsurface settings-feature-card p-4">
              <div data-testid="small-model-main-section-header" class="mb-4 space-y-3">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      {{
                        t(
                          'settings.smallModel.description',
                          'Use a lightweight model for faster simple tasks, with automatic fallback if unavailable.'
                        )
                      }}
                    </p>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    :aria-checked="settingsStore.smallModelEnabled"
                    :disabled="smallModelToggleDisabled"
                    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                    :class="
                      settingsStore.smallModelEnabled
                        ? 'bg-green-600 dark:bg-green-500'
                        : 'bg-gray-300 dark:bg-gray-600'
                    "
                    @click="handleSmallModelEnabledChange(!settingsStore.smallModelEnabled)"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="settingsStore.smallModelEnabled ? 'translate-x-5' : 'translate-x-0'"
                    />
                  </button>
                </div>

                <div
                  data-testid="small-model-resource-status-row"
                  class="flex flex-wrap items-start justify-between gap-3"
                >
                  <div class="min-w-0 flex-1">
                    <div class="text-xs font-medium text-gray-700 dark:text-gray-200">
                      {{ t('settings.smallModel.resourceTitle', 'Resource Footprint') }}
                    </div>
                    <div
                      class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400"
                    >
                      <span data-testid="small-model-storage-usage">
                        {{
                          t('settings.smallModel.storageUsage', { storage: smallModelStorageText })
                        }}
                      </span>
                      <span data-testid="small-model-runtime-usage">
                        {{
                          t('settings.smallModel.runtimeUsage', {
                            runtime: smallModelRuntimeHintText,
                          })
                        }}
                      </span>
                    </div>
                  </div>
                  <span
                    data-testid="small-model-status-badge"
                    class="text-xs px-2 py-1 rounded-full whitespace-nowrap"
                    :class="
                      smallModelReady
                        ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
                        : smallModelDownloading
                          ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
                          : 'bg-gray-100 text-gray-600 dark:bg-gray-700/60 dark:text-gray-300'
                    "
                  >
                    {{
                      smallModelReady
                        ? t('settings.smallModel.ready', 'Ready')
                        : smallModelDownloading
                          ? t('settings.smallModel.downloading', 'Downloading')
                          : t('settings.smallModel.notReady', 'Not Ready')
                    }}
                  </span>
                </div>
              </div>

              <div class="space-y-3">
                <div class="py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
                  <h4 class="text-sm text-gray-800 dark:text-gray-100">
                    {{ t('settings.smallModel.userGuideTitle', 'What this does') }}
                  </h4>
                  <ul
                    class="settings-inline-list mt-1.5 list-disc space-y-1 text-xs text-gray-600 dark:text-gray-300"
                  >
                    <li>
                      {{
                        t(
                          'settings.smallModel.userGuideItem1',
                          'Prioritizes the lightweight model for simple tasks to improve response speed.'
                        )
                      }}
                    </li>
                    <li>
                      {{
                        t(
                          'settings.smallModel.userGuideItem2',
                          'Automatically falls back to the main model when the lightweight model is unavailable.'
                        )
                      }}
                    </li>
                    <li>
                      {{
                        t(
                          'settings.smallModel.userGuideItem3',
                          'Download the lightweight model before first use.'
                        )
                      }}
                    </li>
                  </ul>
                </div>

                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <div
                    class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300"
                  >
                    <div class="flex items-center justify-between gap-3">
                      <div class="font-medium text-gray-900 dark:text-white">
                        {{ t('settings.smallModel.summary', 'Summary / Compression') }}
                      </div>
                      <button
                        data-testid="small-model-summary-switch"
                        type="button"
                        role="switch"
                        :aria-checked="settingsStore.smallModelSummaryEnabled"
                        :disabled="smallModelSaving"
                        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                        :class="
                          settingsStore.smallModelSummaryEnabled
                            ? 'bg-green-600 dark:bg-green-500'
                            : 'bg-gray-300 dark:bg-gray-600'
                        "
                        @click="
                          handleSmallModelSummaryEnabledChange(
                            !settingsStore.smallModelSummaryEnabled
                          )
                        "
                      >
                        <span
                          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                          :class="
                            settingsStore.smallModelSummaryEnabled
                              ? 'translate-x-5'
                              : 'translate-x-0'
                          "
                        />
                      </button>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      {{
                        settingsStore.smallModelSummaryEnabled
                          ? t('common.enabled', 'Enabled')
                          : t('common.disabled', 'Disabled')
                      }}
                    </div>
                  </div>

                  <div
                    class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300"
                  >
                    <div class="flex items-center justify-between gap-3">
                      <div class="font-medium text-gray-900 dark:text-white">
                        {{ t('settings.smallModel.contextCompression', 'Context Compression') }}
                      </div>
                      <button
                        data-testid="small-model-context-compress-switch"
                        type="button"
                        role="switch"
                        :aria-checked="settingsStore.smallModelContextCompressEnabled"
                        :disabled="smallModelSaving"
                        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                        :class="
                          settingsStore.smallModelContextCompressEnabled
                            ? 'bg-green-600 dark:bg-green-500'
                            : 'bg-gray-300 dark:bg-gray-600'
                        "
                        @click="
                          handleSmallModelContextCompressEnabledChange(
                            !settingsStore.smallModelContextCompressEnabled
                          )
                        "
                      >
                        <span
                          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                          :class="
                            settingsStore.smallModelContextCompressEnabled
                              ? 'translate-x-5'
                              : 'translate-x-0'
                          "
                        />
                      </button>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      {{
                        settingsStore.smallModelContextCompressEnabled
                          ? t('common.enabled', 'Enabled')
                          : t('common.disabled', 'Disabled')
                      }}
                    </div>
                    <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                      {{
                        t(
                          'settings.smallModel.contextCompressionHint',
                          'Lets the lightweight model compress long history, while the latest user message still decides what happens now.'
                        )
                      }}
                    </div>
                    <div class="mt-3">
                      <label class="text-xs font-medium text-gray-600 dark:text-gray-300">
                        {{ t('settings.smallModel.contextCompressionMode', 'Compression Mode') }}
                      </label>
                      <select
                        data-testid="context-compression-mode-select"
                        :value="settingsStore.contextCompressionMode"
                        class="settings-select"
                        :disabled="smallModelSaving"
                        @change="
                          handleContextCompressionModeChange(
                            ($event.target as HTMLSelectElement).value as ContextCompressionMode
                          )
                        "
                      >
                        <option v-for="mode in contextCompressionModes" :key="mode" :value="mode">
                          {{
                            mode === 'auto'
                              ? t(
                                  'settings.smallModel.contextCompressionModeAuto',
                                  'Auto: prefer small-model compression, fallback to offline'
                                )
                              : mode === 'small_model'
                                ? t(
                                    'settings.smallModel.contextCompressionModeSmallModel',
                                    'Small Model First'
                                  )
                                : t(
                                    'settings.smallModel.contextCompressionModeOffline',
                                    'Offline Deterministic'
                                  )
                          }}
                        </option>
                      </select>
                      <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                        {{
                          t(
                            'settings.smallModel.contextCompressionModeHint',
                            'Compression triggers automatically under context pressure. This only chooses which compression path to prefer.'
                          )
                        }}
                      </div>
                    </div>
                  </div>

                  <div
                    class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300"
                  >
                    <div class="flex items-center justify-between gap-3">
                      <div class="font-medium text-gray-900 dark:text-white">
                        {{ t('settings.smallModel.docExtract', 'Workflow Document Extraction') }}
                      </div>
                      <button
                        data-testid="small-model-doc-extract-switch"
                        type="button"
                        role="switch"
                        :aria-checked="settingsStore.smallModelDocExtractEnabled"
                        :disabled="smallModelSaving"
                        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                        :class="
                          settingsStore.smallModelDocExtractEnabled
                            ? 'bg-green-600 dark:bg-green-500'
                            : 'bg-gray-300 dark:bg-gray-600'
                        "
                        @click="
                          handleSmallModelDocExtractEnabledChange(
                            !settingsStore.smallModelDocExtractEnabled
                          )
                        "
                      >
                        <span
                          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                          :class="
                            settingsStore.smallModelDocExtractEnabled
                              ? 'translate-x-5'
                              : 'translate-x-0'
                          "
                        />
                      </button>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      {{
                        settingsStore.smallModelDocExtractEnabled
                          ? t('common.enabled', 'Enabled')
                          : t('common.disabled', 'Disabled')
                      }}
                    </div>
                  </div>

                  <div
                    class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300"
                  >
                    <div class="flex items-center justify-between gap-3">
                      <div class="font-medium text-gray-900 dark:text-white">
                        {{ t('settings.smallModel.imageQA', 'Image Recognition Acceleration') }}
                      </div>
                      <button
                        data-testid="small-model-image-qa-switch"
                        type="button"
                        role="switch"
                        :aria-checked="settingsStore.smallModelRouteImageQAEnabled"
                        :disabled="smallModelSaving"
                        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                        :class="
                          settingsStore.smallModelRouteImageQAEnabled
                            ? 'bg-green-600 dark:bg-green-500'
                            : 'bg-gray-300 dark:bg-gray-600'
                        "
                        @click="
                          handleSmallModelRouteImageQAEnabledChange(
                            !settingsStore.smallModelRouteImageQAEnabled
                          )
                        "
                      >
                        <span
                          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                          :class="
                            settingsStore.smallModelRouteImageQAEnabled
                              ? 'translate-x-5'
                              : 'translate-x-0'
                          "
                        />
                      </button>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      {{
                        settingsStore.smallModelRouteImageQAEnabled
                          ? t('common.enabled', 'Enabled')
                          : t('common.disabled', 'Disabled')
                      }}
                    </div>
                  </div>

                  <div
                    class="w-full px-3 py-2 rounded-lg text-sm border bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300"
                  >
                    <div class="flex items-center justify-between gap-3">
                      <div class="font-medium text-gray-900 dark:text-white">
                        {{ t('settings.smallModel.shortQA', 'Short QA Routing') }}
                      </div>
                      <button
                        data-testid="small-model-short-qa-switch"
                        type="button"
                        role="switch"
                        :aria-checked="settingsStore.smallModelRouteShortQAEnabled"
                        :disabled="smallModelSaving"
                        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
                        :class="
                          settingsStore.smallModelRouteShortQAEnabled
                            ? 'bg-green-600 dark:bg-green-500'
                            : 'bg-gray-300 dark:bg-gray-600'
                        "
                        @click="
                          handleSmallModelRouteShortQAEnabledChange(
                            !settingsStore.smallModelRouteShortQAEnabled
                          )
                        "
                      >
                        <span
                          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                          :class="
                            settingsStore.smallModelRouteShortQAEnabled
                              ? 'translate-x-5'
                              : 'translate-x-0'
                          "
                        />
                      </button>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      {{
                        settingsStore.smallModelRouteShortQAEnabled
                          ? t('common.enabled', 'Enabled')
                          : t('common.disabled', 'Disabled')
                      }}
                    </div>
                  </div>
                </div>

                <div class="py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
                  <button
                    data-testid="small-model-stats-toggle"
                    type="button"
                    class="w-full flex items-center justify-between gap-2 text-start"
                    @click="smallModelStatsExpanded = !smallModelStatsExpanded"
                  >
                    <h4 class="text-sm text-gray-800 dark:text-gray-100">
                      {{ t('settings.smallModel.statsTitle', 'Runtime Stats') }}
                    </h4>
                    <span class="text-xs text-gray-500 dark:text-gray-400">{{
                      smallModelStatsExpanded
                        ? t('settings.smallModel.collapse', 'Collapse')
                        : t('settings.smallModel.expand', 'Expand')
                    }}</span>
                  </button>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                    {{
                      t(
                        'settings.smallModel.statsHint',
                        'For troubleshooting and tuning. Daily use usually does not require attention.'
                      )
                    }}
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
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.shortQAAttempts', 'Short QA Attempts') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ settingsStore.smallModelStats?.short_qa_route_attempts ?? 0 }}
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.shortQASuccessRate', 'Short QA Success') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ shortQASuccessRate }}%
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{
                            t('settings.smallModel.imageQAAttempts', 'Image Recognition Attempts')
                          }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ settingsStore.smallModelStats?.image_qa_route_attempts ?? 0 }}
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{
                            t('settings.smallModel.imageQASuccessRate', 'Image Recognition Success')
                          }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ imageQASuccessRate }}%
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{
                            t('settings.smallModel.deepResearchFallbacks', 'Research Fallbacks')
                          }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ settingsStore.smallModelStats?.no_provider_deepresearch_total ?? 0 }}
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.irTakeovers', 'Strategy Takeovers') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ settingsStore.smallModelStats?.ir_takeover_total ?? 0 }}
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.autoRollbacks', 'Auto Rollbacks') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ settingsStore.smallModelStats?.auto_rollback_total ?? 0 }}
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.fallbackTotal', 'Fallback Total') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ settingsStore.smallModelStats?.small_model_fallback_total ?? 0 }}
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.timeoutTotal', 'Timeout Total') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ settingsStore.smallModelStats?.small_model_timeout_total ?? 0 }}
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.latencyMs', 'Small-model Latency') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{
                            (settingsStore.smallModelStats?.small_model_latency_ms ?? 0).toFixed(1)
                          }}ms
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.shortQALatencyMs', 'Short QA Latency') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{
                            (settingsStore.smallModelStats?.short_qa_latency_ms ?? 0).toFixed(1)
                          }}ms
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{
                            t('settings.smallModel.imageQALatencyMs', 'Image Recognition Latency')
                          }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{
                            (settingsStore.smallModelStats?.image_qa_latency_ms ?? 0).toFixed(1)
                          }}ms
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.summaryLatencyMs', 'Summary Latency') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{
                            (settingsStore.smallModelStats?.summary_latency_ms ?? 0).toFixed(1)
                          }}ms
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{
                            t(
                              'settings.smallModel.contextCompressionSuccessRate',
                              'Context Compression Hit Rate'
                            )
                          }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{ contextCompressSuccessRate }}%
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{
                            t(
                              'settings.smallModel.contextCompressionLatencyMs',
                              'Context Compression Latency'
                            )
                          }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{
                            (
                              settingsStore.smallModelStats?.context_compress_latency_ms ?? 0
                            ).toFixed(1)
                          }}ms
                        </div>
                      </div>
                      <div
                        class="rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-2"
                      >
                        <div class="text-gray-500 dark:text-gray-400">
                          {{ t('settings.smallModel.docExtractLatencyMs', 'Doc Extract Latency') }}
                        </div>
                        <div class="mt-1 font-medium text-gray-900 dark:text-white">
                          {{
                            (settingsStore.smallModelStats?.doc_extract_latency_ms ?? 0).toFixed(1)
                          }}ms
                        </div>
                      </div>
                    </div>

                    <div>
                      <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">
                        {{ t('settings.smallModel.fallbackReasons', 'Fallback Reasons') }}
                      </div>
                      <div
                        v-if="fallbackReasonEntries.length === 0"
                        class="text-xs text-gray-500 dark:text-gray-400"
                      >
                        {{
                          t('settings.smallModel.noFallbackReasons', 'No fallback reasons recorded')
                        }}
                      </div>
                      <div v-else class="space-y-1">
                        <div
                          v-for="[reason, count] in fallbackReasonEntries"
                          :key="reason"
                          class="flex items-center justify-between text-xs rounded bg-white dark:bg-slate-800/50 border border-gray-200 dark:border-gray-700 px-2.5 py-1.5"
                        >
                          <span class="text-gray-700 dark:text-gray-200">{{
                            formatFallbackReason(reason)
                          }}</span>
                          <span class="text-gray-900 dark:text-white font-medium">{{ count }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <template v-if="settingsStore.smallModelStatus && smallModelDownloading">
                  <div
                    v-if="
                      settingsStore.smallModelStatus.state === 'connecting' &&
                      settingsStore.smallModelStatus.progress
                    "
                    class="space-y-1.5 px-3 py-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg"
                  >
                    <div
                      class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400"
                    >
                      <span
                        >{{ settingsStore.smallModelStatus.progress.file }} ({{
                          settingsStore.smallModelStatus.progress.file_index + 1
                        }}/{{ settingsStore.smallModelStatus.progress.total_files }})</span
                      >
                      <span>{{ t('settings.smallModel.connecting', 'Connecting') }}</span>
                    </div>
                    <div
                      class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden"
                    >
                      <div
                        class="h-full bg-blue-500/50 dark:bg-blue-400/50 rounded-full animate-pulse w-full"
                      ></div>
                    </div>
                  </div>
                  <div
                    v-else-if="
                      settingsStore.smallModelStatus.state === 'downloading' &&
                      settingsStore.smallModelStatus.progress
                    "
                    class="space-y-1.5 px-3 py-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg"
                  >
                    <div
                      class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400"
                    >
                      <span
                        >{{ settingsStore.smallModelStatus.progress.file }} ({{
                          settingsStore.smallModelStatus.progress.file_index + 1
                        }}/{{ settingsStore.smallModelStatus.progress.total_files }})</span
                      >
                      <span
                        >{{ settingsStore.smallModelStatus.progress.percentage.toFixed(1) }}%</span
                      >
                    </div>
                    <div
                      class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden"
                    >
                      <div
                        class="h-full bg-blue-500 dark:bg-blue-400 rounded-full transition-all duration-300"
                        :style="{ width: settingsStore.smallModelStatus.progress.percentage + '%' }"
                      ></div>
                    </div>
                    <div
                      class="flex items-center justify-between text-xs text-gray-400 dark:text-gray-500"
                    >
                      <span
                        >{{ formatBytes(settingsStore.smallModelStatus.progress.downloaded) }} /
                        {{
                          settingsStore.smallModelStatus.progress.total > 0
                            ? formatBytes(settingsStore.smallModelStatus.progress.total)
                            : '...'
                        }}</span
                      >
                      <span
                        >{{ settingsStore.smallModelStatus.progress.speed_human }} &middot;
                        {{ settingsStore.smallModelStatus.progress.eta || '...' }}</span
                      >
                    </div>
                  </div>
                </template>

                <div
                  v-if="
                    settingsStore.smallModelStatus?.state === 'error' &&
                    settingsStore.smallModelStatus.error
                  "
                  class="px-3 py-2 bg-red-50 dark:bg-red-900/20 rounded-lg flex items-center justify-between"
                >
                  <p class="text-xs text-red-600 dark:text-red-400">
                    {{ settingsStore.smallModelStatus.error }}
                  </p>
                  <button
                    class="settings-inline-gap-sm text-xs text-gray-600 dark:text-gray-300 hover:text-gray-800 dark:hover:text-white flex-shrink-0"
                    @click="startSmallModelDownload"
                  >
                    {{ t('common.retry') }}
                  </button>
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
                    {{
                      smallModelReady
                        ? t('settings.smallModel.redownload', 'Re-download')
                        : t('settings.smallModel.download', 'Download Model')
                    }}
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
          </section>
        </div>

        <div
          v-if="activeTab === 'speech'"
          class="settings-panel dashboard-card-surface settings-surface-card"
        >
          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.voicePipeline', '语音能力')
                }}</span>
                <h2 class="settings-module__title">{{ t('settings.tab.speech') }}</h2>
              </div>
            </div>
            <SpeechSettings />
          </section>
        </div>

        <div
          v-if="activeTab === 'userdata'"
          class="settings-panel dashboard-card-surface settings-surface-card"
        >
          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.memorySurface', '记忆管理')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.memoryManagement', '记忆使用与管理') }}
                </h2>
              </div>
            </div>
            <MemoryManager
              class="settings-embedded-section mx-auto w-full max-w-6xl"
              :memory-recall-mode="settingsStore.memoryRecallMode"
              :agent-auto-reflect="settingsStore.agentAutoReflect"
              @status-change="showSaveStatus"
              @memory-recall-mode-change="handleMemoryRecallModeChange"
            />
          </section>

          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.portability', '数据流转')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.userDataExchange', '导入、导出与清理') }}
                </h2>
              </div>
            </div>
            <UserDataExport
              class="settings-embedded-section mx-auto w-full max-w-6xl"
              @status-change="showSaveStatus"
            />
          </section>

          <section class="settings-module">
            <div class="settings-module__header">
              <div>
                <span class="settings-module__eyebrow">{{
                  t('settings.recoveryRail', '恢复保障')
                }}</span>
                <h2 class="settings-module__title">
                  {{ t('settings.backupRecovery', '备份与恢复') }}
                </h2>
              </div>
            </div>
            <BackupManager
              class="settings-embedded-section mx-auto w-full max-w-6xl"
              :backups="backups"
              :loading="backupsLoading"
              :creating="backupCreating"
              :restoring-id="backupRestoring"
              :deleting-id="backupDeleting"
              @create="onBackupCreate"
              @restore="restoreBackup"
              @delete="deleteBackup"
              @download="onBackupDownload"
            />
          </section>
        </div>
      </div>
    </section>
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

.settings-view {
  width: 100%;
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
}

.settings-stage {
  position: relative;
  padding: 1.15rem 0 0.35rem;
}

.settings-shell {
  --settings-accent: 37, 99, 235;
  --dashboard-page-accent: var(--settings-accent);
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 82rem;
  margin: 0 auto;
}

.settings-surface-card,
.settings-tab-nav,
.settings-panel {
  position: relative;
  overflow: hidden;
  border-radius: 1.5rem;
  box-shadow: none;
}

.settings-hero {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding-bottom: 0.2rem;
}

.settings-hero::after {
  content: none;
  position: absolute;
  inset: auto -2.5rem -4rem auto;
  width: 13rem;
  height: 13rem;
  border-radius: 999px;
  background: none;
  pointer-events: none;
}

.settings-hero__copy {
  position: relative;
  z-index: 1;
  flex: 1 1 0%;
  min-width: 0;
  max-width: 42rem;
  padding-top: 0.1rem;
}

.settings-hero__title {
  margin: 0;
  font-size: clamp(1.34rem, 0.7vw + 0.95rem, 1.9rem);
  line-height: 1.06;
  letter-spacing: -0.04em;
  font-weight: 700;
  color: #111827;
}

.settings-module__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgb(var(--settings-accent));
}

.settings-hero__description {
  margin: 0.42rem 0 0;
  max-width: 34rem;
  font-size: 0.92rem;
  line-height: 1.55;
  color: #9ca3af;
}

.settings-tab-nav {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.settings-tab-button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  min-height: 100%;
  padding: 10px 12px;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1rem;
  text-align: start;
  background: #ffffff;
  color: #0f172a;
  box-shadow: none;
  transition:
    border-color 0.22s ease,
    background-color 0.22s ease,
    color 0.22s ease;
}

.settings-tab-button:hover {
  border-color: rgba(148, 163, 184, 0.52);
  background: #f8fafc;
}

.settings-tab-button--active {
  border-color: rgba(148, 163, 184, 0.58);
  background: #f1f5f9;
  color: #0f172a;
  box-shadow: none;
}

.settings-tab-button__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  border-radius: 0.65rem;
  background: rgba(148, 163, 184, 0.16);
  color: #64748b;
}

.settings-tab-button__icon svg {
  width: 1rem;
  height: 1rem;
}

.settings-tab-button--active .settings-tab-button__icon {
  background: rgba(148, 163, 184, 0.22);
  color: #475569;
}

.settings-tab-button__body {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.32rem;
}

.settings-tab-button__label-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.settings-tab-button__label {
  font-size: 0.9rem;
  font-weight: 700;
  line-height: 1.25;
}

.settings-tab-button__state {
  width: 9px;
  height: 9px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.35);
  transition:
    transform 0.22s ease,
    background-color 0.22s ease;
}

.settings-tab-button--active .settings-tab-button__state {
  transform: scale(1.05);
  background: #64748b;
}

.settings-panel {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.1rem;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #f8fafc;
}

.settings-module {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.settings-module__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
}

.settings-module__title {
  margin: 0.35rem 0 0;
  font-size: 1.15rem;
  font-weight: 700;
  color: #0f172a;
}

.settings-module__description {
  max-width: 44rem;
  margin: 0.45rem 0 0;
  font-size: 0.9rem;
  line-height: 1.6;
  color: #64748b;
}

.settings-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 1rem;
}

.settings-field-card,
.settings-feature-card {
  border-radius: 1.25rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #ffffff;
  box-shadow: none;
}

.settings-field-card {
  padding: 1rem;
}

.settings-field-card--flush {
  padding: 1rem;
}

.settings-field-card__row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.settings-card-heading {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.settings-field-label {
  font-size: 0.82rem;
  font-weight: 700;
  color: #334155;
}

.settings-field-hint {
  margin: 0;
  font-size: 0.8rem;
  line-height: 1.5;
  color: #64748b;
}

.settings-select {
  width: 100%;
  min-height: 2.85rem;
  margin-top: 0.9rem;
  padding: 0.72rem 0.95rem;
  border-radius: 0.95rem;
  border: 1px solid rgba(148, 163, 184, 0.34);
  background: rgba(255, 255, 255, 0.9);
  color: #0f172a;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    background-color 160ms ease;
}

.settings-select:focus {
  outline: none;
  border-color: rgba(var(--settings-accent), 0.45);
  box-shadow: 0 0 0 4px rgba(var(--settings-accent), 0.12);
}

.settings-pill-group {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(156px, 1fr));
  gap: 0.6rem;
  margin-top: 0.9rem;
}

.settings-theme-group {
  display: flex;
  gap: 0.75rem;
  margin-top: 0.9rem;
}

.settings-pill-group--icon-only {
  display: flex;
  gap: 0.75rem;
}

.settings-theme-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: rgba(255, 255, 255, 0.72);
  color: #475569;
  transition:
    transform 160ms ease,
    box-shadow 160ms ease,
    border-color 160ms ease,
    background-color 160ms ease,
    color 160ms ease;
}

.settings-theme-button:hover {
  transform: translateY(-1px);
  border-color: rgba(var(--settings-accent), 0.26);
  color: #0f172a;
}

.settings-theme-button:focus-visible {
  outline: none;
  border-color: rgba(var(--settings-accent), 0.45);
  box-shadow: 0 0 0 4px rgba(var(--settings-accent), 0.12);
}

.settings-theme-button__icon {
  width: 1.3rem;
  height: 1.3rem;
}

.settings-theme-button--active {
  border-color: rgba(var(--settings-accent), 0.36);
  background: rgba(var(--settings-accent), 0.14);
  color: #0f172a;
  box-shadow: none;
}

.settings-pill-button {
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  gap: 0.75rem;
  width: 100%;
  min-height: 2.7rem;
  padding: 0.75rem 0.9rem;
  border-radius: 0.95rem;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: rgba(255, 255, 255, 0.72);
  color: #475569;
  font-size: 0.85rem;
  font-weight: 700;
  line-height: 1.35;
  text-align: start;
  transition:
    transform 160ms ease,
    box-shadow 160ms ease,
    border-color 160ms ease,
    background-color 160ms ease,
    color 160ms ease;
}

.settings-pill-button:hover {
  transform: translateY(-1px);
  border-color: rgba(var(--settings-accent), 0.26);
}

.settings-pill-button:focus-visible {
  outline: none;
  border-color: rgba(var(--settings-accent), 0.45);
  box-shadow: 0 0 0 4px rgba(var(--settings-accent), 0.12);
}

.settings-pill-button__icon-shell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 0.8rem;
  background: rgba(148, 163, 184, 0.12);
  color: inherit;
  flex-shrink: 0;
  transition:
    background-color 160ms ease,
    color 160ms ease;
}

.settings-pill-button__icon {
  width: 1rem;
  height: 1rem;
}

.settings-pill-button--icon-only {
  justify-content: center;
  width: 3rem;
  min-width: 3rem;
  min-height: 3rem;
  padding: 0;
}

.settings-pill-button--active {
  border-color: rgba(var(--settings-accent), 0.36);
  background: rgba(var(--settings-accent), 0.14);
  color: #0f172a;
  box-shadow: none;
}

.settings-pill-button--active .settings-pill-button__icon-shell {
  background: rgba(var(--settings-accent), 0.16);
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

.settings-panel :deep(.glass-card) {
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1.25rem;
  background: #ffffff;
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.settings-panel :deep(.glass-card:hover) {
  border-color: rgba(203, 213, 225, 0.96);
}

.settings-panel :deep(.settings-embedded-section) {
  border-color: rgba(203, 213, 225, 0.72);
  border-radius: 1rem;
  background: transparent;
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.settings-panel :deep(.settings-embedded-section:hover) {
  border-color: rgba(148, 163, 184, 0.36);
  box-shadow: none;
}

:root.dark .settings-view,
[data-theme='dark'] .settings-view,
html.dark .settings-view {
  color: #e2e8f0;
}

:root.dark .settings-tab-nav,
[data-theme='dark'] .settings-tab-nav,
html.dark .settings-tab-nav,
:root.dark .settings-panel,
[data-theme='dark'] .settings-panel,
html.dark .settings-panel {
  border-color: rgba(71, 85, 105, 0.58);
  box-shadow: none;
}

:root.dark .settings-hero__title,
[data-theme='dark'] .settings-hero__title,
html.dark .settings-hero__title,
:root.dark .settings-module__title,
[data-theme='dark'] .settings-module__title,
html.dark .settings-module__title {
  color: #f8fafc;
}

:root.dark .settings-hero__description,
[data-theme='dark'] .settings-hero__description,
html.dark .settings-hero__description,
:root.dark .settings-module__description,
[data-theme='dark'] .settings-module__description,
html.dark .settings-module__description,
:root.dark .settings-field-hint,
[data-theme='dark'] .settings-field-hint,
html.dark .settings-field-hint {
  color: #94a3b8;
}

:root.dark .settings-tab-button,
[data-theme='dark'] .settings-tab-button,
html.dark .settings-tab-button,
:root.dark .settings-field-card,
[data-theme='dark'] .settings-field-card,
html.dark .settings-field-card,
:root.dark .settings-feature-card,
[data-theme='dark'] .settings-feature-card,
html.dark .settings-feature-card,
:root.dark .settings-panel :deep(.settings-embedded-section),
[data-theme='dark'] .settings-panel :deep(.settings-embedded-section),
html.dark .settings-panel :deep(.settings-embedded-section),
:root.dark .settings-panel :deep(.glass-card),
[data-theme='dark'] .settings-panel :deep(.glass-card),
html.dark .settings-panel :deep(.glass-card) {
  border-color: rgba(71, 85, 105, 0.46);
  background: transparent;
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

:root.dark .settings-tab-button,
[data-theme='dark'] .settings-tab-button,
html.dark .settings-tab-button {
  color: #cbd5e1;
}

:root.dark .settings-tab-button--active,
[data-theme='dark'] .settings-tab-button--active,
html.dark .settings-tab-button--active {
  color: #f8fafc;
  border-color: rgba(148, 163, 184, 0.4);
  background: #1f2937;
}

:root.dark .settings-tab-button:hover,
[data-theme='dark'] .settings-tab-button:hover,
html.dark .settings-tab-button:hover {
  border-color: rgba(148, 163, 184, 0.28);
  background: #1f2937;
}

:root.dark .settings-tab-nav,
[data-theme='dark'] .settings-tab-nav,
html.dark .settings-tab-nav,
:root.dark .settings-panel,
[data-theme='dark'] .settings-panel,
html.dark .settings-panel {
  background: #1e293b;
}

:root.dark .settings-tab-nav,
[data-theme='dark'] .settings-tab-nav,
html.dark .settings-tab-nav {
  border: 0;
  background: transparent;
  box-shadow: none;
}

:root.dark .settings-select,
[data-theme='dark'] .settings-select,
html.dark .settings-select,
:root.dark .settings-theme-button,
[data-theme='dark'] .settings-theme-button,
html.dark .settings-theme-button,
:root.dark .settings-pill-button,
[data-theme='dark'] .settings-pill-button,
html.dark .settings-pill-button {
  border-color: rgba(71, 85, 105, 0.5);
  background: rgba(15, 23, 42, 0.72);
  color: #e2e8f0;
}

:root.dark .settings-field-label,
[data-theme='dark'] .settings-field-label,
html.dark .settings-field-label {
  color: #e2e8f0;
}

:root.dark .settings-pill-button--active,
[data-theme='dark'] .settings-pill-button--active,
html.dark .settings-pill-button--active,
:root.dark .settings-theme-button--active,
[data-theme='dark'] .settings-theme-button--active,
html.dark .settings-theme-button--active {
  color: #f8fafc;
  background: rgba(var(--settings-accent), 0.18);
}

:root.dark .settings-pill-button__icon-shell,
[data-theme='dark'] .settings-pill-button__icon-shell,
html.dark .settings-pill-button__icon-shell {
  background: rgba(148, 163, 184, 0.14);
}

:root.dark .settings-pill-button--active .settings-pill-button__icon-shell,
[data-theme='dark'] .settings-pill-button--active .settings-pill-button__icon-shell,
html.dark .settings-pill-button--active .settings-pill-button__icon-shell {
  background: rgba(var(--settings-accent), 0.22);
}

:root.dark .settings-tab-beta,
[data-theme='dark'] .settings-tab-beta,
html.dark .settings-tab-beta {
  color: rgb(191, 219, 254);
  background: rgba(30, 64, 175, 0.25);
  border-color: rgba(147, 197, 253, 0.45);
}

:root.dark .settings-tab-button__state,
[data-theme='dark'] .settings-tab-button__state,
html.dark .settings-tab-button__state {
  background: rgba(148, 163, 184, 0.32);
}

:root.dark .settings-tab-button__icon,
[data-theme='dark'] .settings-tab-button__icon,
html.dark .settings-tab-button__icon {
  background: rgba(148, 163, 184, 0.18);
  color: #cbd5e1;
}

:root.dark .settings-tab-button--active .settings-tab-button__icon,
[data-theme='dark'] .settings-tab-button--active .settings-tab-button__icon,
html.dark .settings-tab-button--active .settings-tab-button__icon {
  background: rgba(148, 163, 184, 0.24);
  color: #f8fafc;
}

:root.dark .settings-tab-button--active .settings-tab-button__state,
[data-theme='dark'] .settings-tab-button--active .settings-tab-button__state,
html.dark .settings-tab-button--active .settings-tab-button__state {
  background: #cbd5e1;
}

.settings-toast {
  position: fixed;
  top: 5rem;
  inset-inline-end: 1rem;
  z-index: 9999;
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
  max-width: min(92vw, 34rem);
  padding: 0.9rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(34, 197, 94, 0.35);
  background: rgba(22, 163, 74, 0.92);
  color: white;
  box-shadow: 0 22px 38px -24px rgba(22, 163, 74, 0.8);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
}

.settings-toast__message {
  font-size: 0.92rem;
  font-weight: 600;
}

.settings-toast__action {
  flex-shrink: 0;
  color: inherit;
  font-size: 0.86rem;
  font-weight: 700;
  text-decoration: underline;
  text-underline-offset: 0.18rem;
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

.settings-inline-list {
  padding-inline-start: 1rem;
}

.settings-inline-gap-sm {
  margin-inline-start: 0.5rem;
}

:global(html[dir='rtl']) .settings-view [role='switch'] .translate-x-5 {
  transform: translateX(-1.25rem);
}

@media (max-width: 1100px) {
  .settings-tab-nav {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .settings-tab-nav {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .settings-view {
    padding: 0.75rem;
  }

  .settings-panel {
    padding: 1rem;
    border-radius: 1.35rem;
  }

  .settings-hero {
    padding-bottom: 0.9rem;
  }

  .settings-tab-nav {
    grid-template-columns: 1fr;
    padding: 0;
  }

  .settings-field-card__row,
  .settings-module__header {
    flex-direction: column;
    align-items: stretch;
  }

  .settings-toast {
    inset-inline: 0.75rem;
    top: 4.5rem;
    width: auto;
  }
}
</style>
