<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { conversationApi, type Conversation } from '@/api/chat'
import { speechApi } from '@/api/speech'
import { voiceWakeApi, type VoiceWakeStatus } from '@/api/voiceWake'
import { useTauri } from '@/composables/useTauri'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { isCurrentHostLoopback } from '@/utils/localPath'

const settingsStore = useSettingsStore()
const chatStore = useChatStore()
const { openInBrowser, platform } = useTauri()
const { t, te, locale } = useI18n()

const IDLE_STATUS_POLL_INTERVAL_MS = 5000
const ACTIVE_STATUS_POLL_INTERVAL_MS = 1200
const AUTOSAVE_DELAY_MS = 700
const FEEDBACK_RESET_DELAY_MS = 1800
const RECENT_ACTIVITY_WINDOW_MS = 8000
const DEFAULT_WAKE_TRIGGER = 'Hey Blue'

const isDesktop = computed(() => typeof window !== 'undefined' && !!window.__BLUE_DESKTOP__)
const isLocalMacLoopback = computed(
  () => !isDesktop.value && platform.value === 'macos' && isCurrentHostLoopback()
)
const canManageVoiceWake = computed(() => isDesktop.value || isLocalMacLoopback.value)
const loading = ref(false)
const saving = ref(false)
const requestError = ref('')
const status = ref<VoiceWakeStatus | null>(null)
const conversations = ref<Conversation[]>([])
const offlineLanguages = ref<string[]>([])
const targetSelect = ref<HTMLSelectElement | null>(null)
const syncingFormFromStore = ref(false)
const autoSaveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const recentActivityType = ref<'triggered' | 'sent' | null>(null)

const form = reactive({
  enabled: false,
  triggers: DEFAULT_WAKE_TRIGGER,
  locale: '',
  targetConversationID: '',
})
type VoiceWakeFormState = typeof form

const shouldShow = computed(() => true)
const supportsConfiguration = computed(() => canManageVoiceWake.value && !!status.value?.supported)
const hasTargetConversation = computed(() => form.targetConversationID.trim().length > 0)
const canResolveTargetConversation = computed(
  () => hasTargetConversation.value || !!currentConversationOption.value?.id
)
const enableBlocked = computed(() => !canResolveTargetConversation.value)
const textInputsDirty = computed(
  () =>
    !sameStringArray(
      parseTriggerInput(form.triggers),
      parseTriggerInput(normalizeTriggers(settingsStore.backendSettings.voice_wake_triggers))
    ) || form.locale.trim() !== String(settingsStore.backendSettings.voice_wake_locale ?? '').trim()
)
const runtimeTriggers = computed(() =>
  status.value?.triggers?.length ? status.value.triggers.join(', ') : DEFAULT_WAKE_TRIGGER
)
const statusReason = computed(() => status.value?.reason?.trim() || '')
const offlineLanguageOptions = computed(() => {
  const seen = new Set<string>()
  return offlineLanguages.value.flatMap((value) => {
    const trimmed = normalizeLocaleCode(value)
    if (!trimmed || seen.has(trimmed)) return []
    seen.add(trimmed)
    return [trimmed]
  })
})
const hasOfflineLanguageOptions = computed(() => offlineLanguageOptions.value.length > 0)
type ConversationOption = Conversation & { source?: 'current' | 'saved' }
const requestedConversationIDs = new Set<string>()
const langDisplayNames = computed(() => new Intl.DisplayNames([locale.value], { type: 'language' }))

function createSyntheticConversation(id: string, title = ''): Conversation {
  return {
    id,
    title,
    created_at: '',
    updated_at: '',
  }
}

const currentConversationOption = computed<ConversationOption | null>(() => {
  const activeConversation = chatStore.currentConversation
  if (activeConversation?.id?.trim()) {
    return { ...activeConversation, source: 'current' }
  }

  const activeConversationID = chatStore.currentConversationId?.trim() || ''
  if (!activeConversationID) return null
  const fallback = conversations.value.find(
    (conversation) => conversation.id === activeConversationID
  )
  return fallback
    ? { ...fallback, source: 'current' }
    : { ...createSyntheticConversation(activeConversationID), source: 'current' }
})

const savedTargetConversationOption = computed<ConversationOption | null>(() => {
  const selectedID = form.targetConversationID.trim()
  if (!selectedID) return null
  if (selectedID === currentConversationOption.value?.id) return null
  const knownConversation = conversations.value.find(
    (conversation) => conversation.id === selectedID
  )
  return knownConversation
    ? { ...knownConversation, source: 'saved' }
    : { ...createSyntheticConversation(selectedID), source: 'saved' }
})

const conversationOptions = computed<ConversationOption[]>(() => {
  const options: ConversationOption[] = []
  const seen = new Set<string>()

  const appendOption = (conversation: ConversationOption | null) => {
    const id = conversation?.id?.trim() || ''
    if (!id || seen.has(id) || !conversation) return
    options.push(conversation)
    seen.add(id)
  }

  appendOption(currentConversationOption.value)
  appendOption(savedTargetConversationOption.value)
  for (const conversation of conversations.value) {
    appendOption(conversation)
  }

  return options
})
const canUseCurrentConversation = computed(() => !!currentConversationOption.value?.id)
const useCurrentConversationDisabled = computed(
  () =>
    !currentConversationOption.value ||
    form.targetConversationID === currentConversationOption.value.id
)
const runtimeTargetConversationLabel = computed(() =>
  formatConversationValue(status.value?.target_conversation_id)
)
const activityState = computed<'idle' | 'listening' | 'triggered' | 'sent'>(() => {
  if (recentActivityType.value === 'sent') return 'sent'
  if (recentActivityType.value === 'triggered') return 'triggered'
  if (status.value?.running) return 'listening'
  if (status.value?.last_sent_at) return 'sent'
  if (status.value?.last_triggered_at) return 'triggered'
  return 'idle'
})
const activityTitle = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return `${t('speech.voiceWake.lastSent')}: ${formatTimestamp(status.value?.last_sent_at)}`
    case 'triggered':
      return `${t('speech.voiceWake.lastTriggered')}: ${formatTimestamp(status.value?.last_triggered_at)}`
    case 'listening':
      return statusMessage.value
    default:
      return statusMessage.value
  }
})
const activityMeta = computed(() => {
  switch (activityState.value) {
    case 'sent':
    case 'triggered':
      return `${t('speech.voiceWake.targetConversation')}: ${runtimeTargetConversationLabel.value}`
    default:
      return `${t('speech.voiceWake.activeTriggers')}: ${runtimeTriggers.value}`
  }
})

const statusMessage = computed(() => {
  if (!canManageVoiceWake.value) {
    return t('speech.voiceWake.messages.unsupported')
  }
  if (loading.value && !status.value) {
    return t('common.checking')
  }
  const reason = statusReason.value
  switch (reason) {
    case 'running':
      return t('speech.voiceWake.messages.running')
    case 'disabled':
      return ''
    case 'target_missing':
      return t('speech.voiceWake.messages.targetMissing')
    case 'target_unavailable':
      return t('speech.voiceWake.messages.targetUnavailable')
    case 'speech_permission_denied':
      return t('speech.voiceWake.messages.speechPermissionDenied')
    case 'microphone_unavailable':
      return t('speech.voiceWake.messages.microphoneUnavailable')
    case 'start_failed':
      return t('speech.voiceWake.messages.startFailed')
    case 'send_failed':
      return t('speech.voiceWake.messages.sendFailed')
    case 'runtime_error':
      return t('speech.voiceWake.messages.runtimeError')
    case 'unsupported':
      return t('speech.voiceWake.messages.unsupported')
    default:
      if (status.value && status.value.supported === false) {
        return t('speech.voiceWake.messages.unsupported')
      }
      return requestError.value || t('speech.voiceWake.messages.unavailable')
  }
})
const hasStatusMessage = computed(() => statusMessage.value.trim().length > 0)

const statusIconBgClass = computed(() => {
  if (!supportsConfiguration.value) {
    return 'bg-amber-100 dark:bg-amber-900/30'
  }
  if (status.value?.running) {
    return 'bg-green-100 dark:bg-green-900/30'
  }
  return 'bg-blue-100 dark:bg-blue-900/30'
})
const statusIconClass = computed(() => {
  if (!supportsConfiguration.value) {
    return 'text-amber-600 dark:text-amber-300'
  }
  if (status.value?.running) {
    return 'text-green-600 dark:text-green-400'
  }
  return 'text-blue-600 dark:text-blue-400'
})
const statusMessageClass = computed(() => {
  if (!supportsConfiguration.value) {
    return 'text-amber-700 dark:text-amber-300'
  }
  if (status.value?.running) {
    return 'text-gray-700 dark:text-gray-200'
  }
  if (statusReason.value === 'disabled' || statusReason.value === 'target_missing') {
    return 'text-gray-700 dark:text-gray-200'
  }
  return 'text-amber-700 dark:text-amber-300'
})
const activityPanelClass = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return 'rounded-lg border border-green-200 bg-green-50/80 px-3 py-3 dark:border-green-800/60 dark:bg-green-900/20'
    case 'triggered':
      return 'rounded-lg border border-sky-200 bg-sky-50/80 px-3 py-3 dark:border-sky-800/60 dark:bg-sky-900/20'
    case 'listening':
      return 'rounded-lg border border-green-200/80 bg-white px-3 py-3 dark:border-green-800/50 dark:bg-gray-800/80'
    default:
      return 'rounded-lg border border-gray-200 bg-white px-3 py-3 dark:border-gray-600 dark:bg-gray-800'
  }
})
const activityDotClass = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return 'bg-green-500'
    case 'triggered':
      return 'bg-sky-500'
    case 'listening':
      return 'bg-green-500'
    default:
      return 'bg-gray-400 dark:bg-gray-500'
  }
})
const activityPingClass = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return 'bg-green-400'
    case 'triggered':
      return 'bg-sky-400'
    case 'listening':
      return 'bg-green-400'
    default:
      return 'bg-gray-300 dark:bg-gray-500'
  }
})
const activityTitleClass = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return 'text-green-800 dark:text-green-200'
    case 'triggered':
      return 'text-sky-800 dark:text-sky-200'
    default:
      return 'text-gray-900 dark:text-white'
  }
})
const activityMetaClass = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return 'text-green-700/80 dark:text-green-300/80'
    case 'triggered':
      return 'text-sky-700/80 dark:text-sky-300/80'
    default:
      return 'text-gray-500 dark:text-gray-400'
  }
})
const showActivityPulse = computed(() => activityState.value !== 'idle')
const showActivityPanel = computed(
  () =>
    supportsConfiguration.value &&
    (status.value?.running ||
      !!status.value?.last_triggered_at ||
      !!status.value?.last_sent_at ||
      recentActivityType.value !== null)
)
const showGuidanceActions = computed(
  () =>
    supportsConfiguration.value &&
    [
      'speech_permission_denied',
      'microphone_unavailable',
      'target_missing',
      'target_unavailable',
    ].includes(statusReason.value)
)
const selectedLocaleOption = computed({
  get: () => resolveVoiceWakeLocale(form.locale),
  set: (value: string) => {
    form.locale = normalizeLocaleCode(value)
  },
})
const autoSaveFeedbackText = computed(() => {
  if (autoSaveState.value === 'saving') return t('common.saving')
  if (autoSaveState.value === 'saved') return t('common.saved')
  if (autoSaveState.value === 'error')
    return requestError.value || t('speech.voiceWake.messages.unavailable')
  return ''
})
const autoSaveFeedbackClass = computed(() => {
  if (autoSaveState.value === 'saved') {
    return 'text-green-600 dark:text-green-400'
  }
  if (autoSaveState.value === 'error') {
    return 'text-amber-600 dark:text-amber-400'
  }
  return 'text-gray-500 dark:text-gray-400'
})

function normalizeTriggers(triggers?: string[]): string {
  if (!triggers || triggers.length === 0) return DEFAULT_WAKE_TRIGGER
  return triggers.join(', ')
}

function normalizeLocaleCode(value?: string): string {
  return String(value || '').trim()
}

function localeBase(value?: string): string {
  return normalizeLocaleCode(value).split('-')[0]?.toLowerCase() || ''
}

function findLocaleMatch(preferred: string, available: string[]): string {
  const normalizedPreferred = normalizeLocaleCode(preferred)
  if (!normalizedPreferred) return ''
  const exactMatch = available.find(
    (candidate) => candidate.toLowerCase() === normalizedPreferred.toLowerCase()
  )
  if (exactMatch) return exactMatch
  const preferredBase = localeBase(normalizedPreferred)
  if (!preferredBase) return ''
  return available.find((candidate) => localeBase(candidate) === preferredBase) || ''
}

function resolveVoiceWakeLocale(preferred?: string): string {
  const available = offlineLanguageOptions.value
  const normalizedPreferred = normalizeLocaleCode(preferred)
  if (available.length === 0) return normalizedPreferred
  const firstInstalledLocale = available.find((candidate) => normalizeLocaleCode(candidate) !== '')
  const fallbackLocale = firstInstalledLocale ?? (normalizedPreferred || 'en-US')

  return (
    findLocaleMatch(normalizedPreferred, available) ||
    findLocaleMatch(locale.value, available) ||
    findLocaleMatch('en-US', available) ||
    fallbackLocale
  )
}

function langName(code: string): string {
  const normalizedCode = normalizeLocaleCode(code)
  const i18nKey = `speech.langName.${normalizedCode}`
  if (te(i18nKey)) return t(i18nKey)
  try {
    return langDisplayNames.value.of(normalizedCode) ?? normalizedCode
  } catch {
    return normalizedCode
  }
}

function sameStringArray(left: string[], right: string[]): boolean {
  if (left.length !== right.length) return false
  return left.every((value, index) => value === right[index])
}

function clearTimer(timer: ReturnType<typeof setTimeout> | null) {
  if (timer !== null) {
    clearTimeout(timer)
  }
}

function formatConversationOptionLabel(conversation: ConversationOption): string {
  const baseLabel = conversation.title?.trim() || conversation.id
  if (conversation.source === 'current') {
    return `${t('speech.voiceWake.currentChatPrefix')}: ${baseLabel}`
  }
  if (conversation.source === 'saved') {
    return `${t('speech.voiceWake.savedTargetPrefix')}: ${baseLabel}`
  }
  return baseLabel
}

function findConversationByID(id: string): Conversation | null {
  const trimmedID = id.trim()
  if (!trimmedID) return null
  const activeConversation = chatStore.currentConversation
  if (activeConversation?.id === trimmedID) {
    return activeConversation
  }
  return conversations.value.find((conversation) => conversation.id === trimmedID) || null
}

function formatConversationValue(id?: string): string {
  const trimmedID = String(id || '').trim()
  if (!trimmedID) return t('speech.voiceWake.notSelected')
  const knownConversation = findConversationByID(trimmedID)
  const title = knownConversation?.title?.trim() || ''
  return title || trimmedID
}

function normalizeSettingsFromStore() {
  syncingFormFromStore.value = true
  const settings = settingsStore.backendSettings
  form.enabled = settings.voice_wake_enabled ?? false
  form.triggers = normalizeTriggers(settings.voice_wake_triggers)
  form.locale = settings.voice_wake_locale ?? ''
  form.targetConversationID = settings.voice_wake_target_conversation_id ?? ''
  syncingFormFromStore.value = false
}

function parseTriggerInput(input: string): string[] {
  return input
    .split(/[\n,]/)
    .map((value) => value.trim())
    .filter(Boolean)
}

function formatTimestamp(value?: string): string {
  const raw = String(value || '').trim()
  if (!raw) return t('speech.voiceWake.never')
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return raw
  return parsed.toLocaleString()
}

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message
  }
  return 'Request failed.'
}

async function refreshStatus(options: { silent?: boolean } = {}) {
  if (!canManageVoiceWake.value) return
  if (!options.silent) {
    loading.value = true
  }
  try {
    const response = await voiceWakeApi.getStatus()
    status.value = response.data
    requestError.value = ''
  } catch (error) {
    requestError.value = extractErrorMessage(error)
    status.value = null
  } finally {
    if (!options.silent) {
      loading.value = false
    }
  }
}

function resetAutoSaveFeedback(delay = FEEDBACK_RESET_DELAY_MS) {
  if (delay <= 0) {
    autoSaveState.value = 'idle'
    return
  }
  clearTimer(clearAutoSaveFeedbackTimer)
  clearAutoSaveFeedbackTimer = setTimeout(() => {
    if (autoSaveState.value === 'saved') {
      autoSaveState.value = 'idle'
    }
  }, delay)
}

function setAutoSaveState(next: 'idle' | 'saving' | 'saved' | 'error') {
  clearTimer(clearAutoSaveFeedbackTimer)
  autoSaveState.value = next
  if (next === 'saved') {
    resetAutoSaveFeedback()
  }
}

async function loadConversations() {
  if (!canManageVoiceWake.value) return
  try {
    const response = await conversationApi.list(20, 0)
    const nextConversations = Array.isArray(response.data) ? response.data : []
    const mergedByID = new Map<string, Conversation>()
    for (const conversation of conversations.value) {
      if (conversation.id?.trim()) {
        mergedByID.set(conversation.id, conversation)
      }
    }
    for (const conversation of nextConversations) {
      if (conversation.id?.trim()) {
        mergedByID.set(conversation.id, {
          ...(mergedByID.get(conversation.id) || {}),
          ...conversation,
        })
      }
    }
    conversations.value = [...mergedByID.values()]
    await ensureConversationDetails([
      chatStore.currentConversationId,
      form.targetConversationID,
      status.value?.target_conversation_id,
    ])
  } catch (error) {
    requestError.value = extractErrorMessage(error)
    conversations.value = []
  }
}

async function loadOfflineLanguages() {
  if (!canManageVoiceWake.value) return
  try {
    const response = await speechApi.getOfflineLanguages()
    offlineLanguages.value = Array.isArray(response.data?.offline_languages)
      ? response.data.offline_languages
      : []
  } catch {
    offlineLanguages.value = []
  }
}

function upsertConversation(conversation: Conversation) {
  const id = conversation.id?.trim()
  if (!id) return
  const index = conversations.value.findIndex((item) => item.id === id)
  if (index === -1) {
    conversations.value = [...conversations.value, conversation]
    return
  }
  const next = [...conversations.value]
  next[index] = { ...next[index], ...conversation }
  conversations.value = next
}

async function ensureConversationDetails(ids: Array<string | null | undefined>) {
  const missingIDs = ids
    .map((value) => String(value || '').trim())
    .filter((value, index, values) => value && values.indexOf(value) === index)
    .filter((id) => {
      if (requestedConversationIDs.has(id)) return false
      const knownConversation = findConversationByID(id)
      return !knownConversation?.title?.trim()
    })

  if (missingIDs.length === 0) return

  await Promise.all(
    missingIDs.map(async (id) => {
      requestedConversationIDs.add(id)
      try {
        const response = await conversationApi.get(id)
        if (response.data?.id) {
          upsertConversation(response.data)
        }
      } catch {
        // Keep the raw ID fallback when the conversation no longer exists.
      }
    })
  )
}

type PersistOptions = {
  revertOnError?: boolean
}

async function persistSettings(
  overrides: Partial<VoiceWakeFormState> = {},
  options: PersistOptions = {}
) {
  if (saving.value) return false
  let nextTargetConversationID = String(
    overrides.targetConversationID ?? form.targetConversationID
  ).trim()
  const nextEnabled = Boolean(overrides.enabled ?? form.enabled)

  if (!nextTargetConversationID && nextEnabled && currentConversationOption.value?.id) {
    nextTargetConversationID = currentConversationOption.value.id
    form.targetConversationID = nextTargetConversationID
  }

  if (nextEnabled && !nextTargetConversationID) {
    if (options.revertOnError) {
      normalizeSettingsFromStore()
      focusTargetConversation()
    }
    return false
  }

  saving.value = true
  const nextLocale = hasOfflineLanguageOptions.value
    ? resolveVoiceWakeLocale(String(overrides.locale ?? form.locale))
    : normalizeLocaleCode(String(overrides.locale ?? form.locale))
  try {
    await settingsStore.updateBackendSettings({
      voice_wake_enabled: nextEnabled && !!nextTargetConversationID,
      voice_wake_triggers: parseTriggerInput(String(overrides.triggers ?? form.triggers)),
      voice_wake_locale: nextLocale,
      voice_wake_target_conversation_id: nextTargetConversationID,
    })
    normalizeSettingsFromStore()
    await refreshStatus()
    await loadConversations()
    return true
  } catch (error) {
    requestError.value = extractErrorMessage(error)
    if (options.revertOnError) {
      normalizeSettingsFromStore()
    }
    return false
  } finally {
    saving.value = false
  }
}

async function persistSettingsWithFeedback(
  overrides: Partial<VoiceWakeFormState> = {},
  options: PersistOptions = {}
) {
  setAutoSaveState('saving')
  const ok = await persistSettings(overrides, options)
  setAutoSaveState(ok ? 'saved' : 'error')
  return ok
}

async function openSpeechSettings() {
  await openInBrowser(
    'x-apple.systempreferences:com.apple.preference.security?Privacy_SpeechRecognition'
  )
}

async function openMicrophoneSettings() {
  await openInBrowser('x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone')
}

function focusTargetConversation() {
  targetSelect.value?.focus()
}

async function useCurrentConversationAsTarget() {
  const option = currentConversationOption.value
  if (!option) return
  form.targetConversationID = option.id
  await persistSettingsWithFeedback({ targetConversationID: option.id }, { revertOnError: true })
}

async function handleTargetConversationChange() {
  await persistSettingsWithFeedback(
    { targetConversationID: form.targetConversationID },
    { revertOnError: true }
  )
}

async function handleEnabledChange() {
  if (form.enabled && !form.targetConversationID.trim() && currentConversationOption.value?.id) {
    form.targetConversationID = currentConversationOption.value.id
  }
  await persistSettingsWithFeedback(
    {
      enabled: form.enabled,
      targetConversationID: form.targetConversationID,
    },
    { revertOnError: true }
  )
}

async function flushDraftSettings() {
  clearTimer(textAutoSaveTimer)
  textAutoSaveTimer = null
  if (!supportsConfiguration.value || !textInputsDirty.value) return
  if (saving.value) {
    scheduleDraftAutosave(250)
    return
  }
  const ok = await persistSettingsWithFeedback()
  if (ok && textInputsDirty.value) {
    scheduleDraftAutosave(250)
  }
}

function scheduleDraftAutosave(delay = AUTOSAVE_DELAY_MS) {
  clearTimer(textAutoSaveTimer)
  if (!supportsConfiguration.value || !textInputsDirty.value) return
  textAutoSaveTimer = setTimeout(() => {
    void flushDraftSettings()
  }, delay)
}

function markRecentActivity(type: 'triggered' | 'sent') {
  clearTimer(recentActivityTimer)
  recentActivityType.value = type
  recentActivityTimer = setTimeout(() => {
    recentActivityType.value = null
  }, RECENT_ACTIVITY_WINDOW_MS)
}

function isRecentTimestamp(value?: string): boolean {
  const ms = Date.parse(String(value || ''))
  return Number.isFinite(ms) && Date.now() - ms <= RECENT_ACTIVITY_WINDOW_MS
}

let statusPollTimer: ReturnType<typeof setTimeout> | null = null
let textAutoSaveTimer: ReturnType<typeof setTimeout> | null = null
let recentActivityTimer: ReturnType<typeof setTimeout> | null = null
let clearAutoSaveFeedbackTimer: ReturnType<typeof setTimeout> | null = null

function stopStatusPolling() {
  if (statusPollTimer) {
    clearTimeout(statusPollTimer)
    statusPollTimer = null
  }
}

function nextStatusPollInterval(): number {
  return status.value?.running ? ACTIVE_STATUS_POLL_INTERVAL_MS : IDLE_STATUS_POLL_INTERVAL_MS
}

function scheduleStatusPolling() {
  stopStatusPolling()
  if (!canManageVoiceWake.value) return
  statusPollTimer = setTimeout(async () => {
    statusPollTimer = null
    if (typeof document !== 'undefined' && document.hidden) {
      scheduleStatusPolling()
      return
    }
    await refreshStatus({ silent: true })
    scheduleStatusPolling()
  }, nextStatusPollInterval())
}

function startStatusPolling() {
  if (!canManageVoiceWake.value || statusPollTimer) return
  scheduleStatusPolling()
}

watch(
  () => settingsStore.backendSettings,
  () => {
    normalizeSettingsFromStore()
  },
  { deep: true, immediate: true }
)

watch([() => form.triggers, () => form.locale], () => {
  if (syncingFormFromStore.value) return
  if (!textInputsDirty.value) {
    if (autoSaveState.value !== 'saving') {
      setAutoSaveState('idle')
    }
    clearTimer(textAutoSaveTimer)
    return
  }
  if (autoSaveState.value !== 'saving') {
    setAutoSaveState('idle')
  }
  scheduleDraftAutosave()
})

watch(
  () => status.value?.running,
  () => {
    if (!canManageVoiceWake.value) return
    scheduleStatusPolling()
  }
)

watch(
  () => status.value?.last_triggered_at,
  (next, prev) => {
    if (!next || next === prev) return
    if (prev || isRecentTimestamp(next)) {
      markRecentActivity('triggered')
    }
  }
)

watch(
  () => status.value?.last_sent_at,
  (next, prev) => {
    if (!next || next === prev) return
    if (prev || isRecentTimestamp(next)) {
      markRecentActivity('sent')
    }
  }
)

watch(
  [
    () => chatStore.currentConversationId,
    () => form.targetConversationID,
    () => status.value?.target_conversation_id,
  ],
  (ids) => {
    void ensureConversationDetails(ids)
  },
  { immediate: true }
)

onMounted(async () => {
  if (!canManageVoiceWake.value) return
  if (Object.keys(settingsStore.backendSettings).length === 0) {
    await settingsStore.fetchBackendSettings()
  }
  await Promise.all([refreshStatus(), loadConversations(), loadOfflineLanguages()])
  startStatusPolling()
})

onUnmounted(() => {
  stopStatusPolling()
  clearTimer(textAutoSaveTimer)
  clearTimer(recentActivityTimer)
  clearTimer(clearAutoSaveFeedbackTimer)
})
</script>

<template>
  <section
    v-if="shouldShow"
    data-testid="voicewake-section"
    class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm space-y-4"
  >
    <div class="flex items-start justify-between gap-4">
      <div class="flex min-w-0 items-start gap-3">
        <div class="w-8 h-8 rounded-lg flex items-center justify-center" :class="statusIconBgClass">
          <svg
            class="w-4 h-4"
            :class="statusIconClass"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d="M12 4v16" />
            <path d="M8 8v8" />
            <path d="M16 8v8" />
            <path d="M5 11v2" />
            <path d="M19 11v2" />
          </svg>
        </div>
        <div class="min-w-0 space-y-1">
          <h4
            data-testid="voicewake-title"
            class="text-sm font-semibold leading-5 text-gray-900 dark:text-white"
          >
            {{ t('speech.voiceWake.title') }}
          </h4>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.description') }}
          </p>
        </div>
      </div>
      <label v-if="supportsConfiguration" data-testid="voicewake-header-toggle" class="shrink-0">
        <span class="sr-only">{{ `${t('common.enable')} ${t('speech.voiceWake.title')}` }}</span>
        <span
          class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
          :class="form.enabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
        >
          <input
            v-model="form.enabled"
            data-testid="voicewake-enabled"
            type="checkbox"
            class="peer sr-only"
            :disabled="enableBlocked || saving"
            @change="handleEnabledChange"
          />
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
            :class="form.enabled ? 'translate-x-5' : 'translate-x-0.5'"
          />
        </span>
      </label>
    </div>

    <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 space-y-4">
      <p
        v-if="hasStatusMessage"
        data-testid="voicewake-status-message"
        class="text-sm"
        :class="statusMessageClass"
      >
        {{ statusMessage }}
      </p>

      <div
        v-if="showActivityPanel"
        data-testid="voicewake-activity-panel"
        :class="activityPanelClass"
      >
        <div class="flex items-start gap-3">
          <span class="relative mt-1 inline-flex h-3 w-3 flex-none">
            <span
              v-if="showActivityPulse"
              class="absolute inline-flex h-full w-full rounded-full opacity-70 animate-ping"
              :class="activityPingClass"
            />
            <span class="relative inline-flex h-3 w-3 rounded-full" :class="activityDotClass" />
          </span>
          <div class="space-y-1">
            <p
              data-testid="voicewake-activity-title"
              class="text-sm font-medium"
              :class="activityTitleClass"
            >
              {{ activityTitle }}
            </p>
            <p data-testid="voicewake-activity-meta" class="text-xs" :class="activityMetaClass">
              {{ activityMeta }}
            </p>
          </div>
        </div>
      </div>

      <div
        v-if="status?.last_error"
        data-testid="voicewake-status-error"
        class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-300"
      >
        <span class="font-medium">{{ t('speech.voiceWake.lastError') }}:</span>
        {{ status.last_error }}
      </div>

      <div
        v-if="supportsConfiguration"
        data-testid="voicewake-inline-config-grid"
        class="grid gap-3 lg:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]"
      >
        <div
          data-testid="voicewake-wakeword-config"
          class="rounded-lg border border-gray-200 bg-white px-3 py-3 dark:border-gray-600 dark:bg-gray-800"
        >
          <div class="flex flex-col gap-3 lg:flex-row lg:items-start">
            <label class="min-w-0 flex-1 space-y-2">
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('speech.voiceWake.wakeWords') }}
              </span>
              <textarea
                v-model="form.triggers"
                data-testid="voicewake-triggers"
                rows="3"
                class="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white resize-y"
                :placeholder="`${t('voiceView.wakeWordPlaceholder')}, Jarvis`"
                @blur="flushDraftSettings"
              />
              <p class="text-[11px] text-gray-500 dark:text-gray-400">
                {{ t('speech.voiceWake.wakeWordsHelp') }}
              </p>
            </label>

            <label
              v-if="hasOfflineLanguageOptions"
              data-testid="voicewake-locale-inline"
              class="w-full space-y-2 lg:w-48 lg:flex-none"
            >
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('speech.voiceWake.localeOverride') }}
              </span>
              <select
                v-model="selectedLocaleOption"
                data-testid="voicewake-locale"
                class="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white"
                :disabled="saving"
                @change="flushDraftSettings"
              >
                <option v-for="lang in offlineLanguageOptions" :key="lang" :value="lang">
                  {{ langName(lang) }}
                </option>
              </select>
            </label>
          </div>
        </div>
        <div
          data-testid="voicewake-target-config"
          class="rounded-lg border border-gray-200 bg-white px-3 py-3 dark:border-gray-600 dark:bg-gray-800"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="space-y-1">
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('speech.voiceWake.targetConversation') }}
              </p>
              <p class="text-[11px] text-gray-500 dark:text-gray-400">
                {{ t('speech.voiceWake.targetDescription') }}
              </p>
            </div>
            <button
              v-if="canUseCurrentConversation"
              data-testid="voicewake-use-current"
              type="button"
              class="text-[11px] font-medium text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
              :disabled="useCurrentConversationDisabled"
              @click="useCurrentConversationAsTarget"
            >
              {{ t('speech.voiceWake.useCurrentChat') }}
            </button>
          </div>

          <p
            v-if="enableBlocked"
            data-testid="voicewake-target-warning"
            class="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-300"
          >
            {{ t('speech.voiceWake.fixedTargetWarning') }}
          </p>

          <label class="mt-3 block space-y-2">
            <span class="text-xs font-medium text-gray-700 dark:text-gray-200">
              {{ t('speech.voiceWake.targetFieldLabel') }}
            </span>
            <select
              ref="targetSelect"
              v-model="form.targetConversationID"
              data-testid="voicewake-target"
              class="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white"
              :disabled="saving"
              @change="handleTargetConversationChange"
            >
              <option value="">{{ t('speech.voiceWake.selectConversation') }}</option>
              <option
                v-for="conversation in conversationOptions"
                :key="conversation.id"
                :value="conversation.id"
              >
                {{ formatConversationOptionLabel(conversation) }}
              </option>
            </select>
          </label>

          <p class="mt-3 text-[11px] text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.desktopNoteDescription') }}
          </p>
        </div>
      </div>
    </div>

    <div
      v-if="
        supportsConfiguration &&
        (autoSaveState !== 'idle' || loading || (requestError && !status?.last_error))
      "
      class="flex flex-wrap items-center gap-3"
    >
      <span
        v-if="autoSaveState !== 'idle'"
        data-testid="voicewake-autosave-status"
        class="text-xs font-medium"
        :class="autoSaveFeedbackClass"
      >
        {{ autoSaveFeedbackText }}
      </span>
      <span v-if="loading" class="text-xs text-gray-500 dark:text-gray-400">{{
        t('common.refreshing')
      }}</span>
      <span
        v-if="requestError && !status?.last_error && autoSaveState !== 'error'"
        class="text-xs text-amber-600 dark:text-amber-400"
      >
        {{ requestError }}
      </span>
    </div>

    <div v-if="showGuidanceActions" class="flex flex-wrap items-center gap-2 pt-1">
      <button
        v-if="statusReason === 'speech_permission_denied'"
        data-testid="voicewake-open-speech-settings"
        class="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-600 text-xs font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700"
        @click="openSpeechSettings"
      >
        {{ t('speech.voiceWake.openSpeechSettings') }}
      </button>
      <button
        v-if="statusReason === 'microphone_unavailable'"
        data-testid="voicewake-open-mic-settings"
        class="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-600 text-xs font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700"
        @click="openMicrophoneSettings"
      >
        {{ t('speech.voiceWake.openMicrophoneSettings') }}
      </button>
      <button
        v-if="statusReason === 'target_missing' || statusReason === 'target_unavailable'"
        data-testid="voicewake-pick-target"
        class="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-600 text-xs font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700"
        @click="focusTargetConversation"
      >
        {{ t('speech.voiceWake.chooseConversation') }}
      </button>
    </div>
  </section>
</template>
