<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { conversationApi, type Conversation } from '@/api/chat'
import { voiceWakeApi, type VoiceWakeStatus } from '@/api/voiceWake'
import { useTauri } from '@/composables/useTauri'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { isCurrentHostLoopback } from '@/utils/localPath'

const settingsStore = useSettingsStore()
const chatStore = useChatStore()
const { openInBrowser, platform } = useTauri()
const { t } = useI18n()

const STATUS_POLL_INTERVAL_MS = 5000
const DEFAULT_WAKE_TRIGGER = 'Hey Blue'

const isDesktop = computed(() => typeof window !== 'undefined' && !!window.__BLUE_DESKTOP__)
const isLocalMacLoopback = computed(
  () => !isDesktop.value && platform.value === 'macos' && isCurrentHostLoopback()
)
const canManageVoiceWake = computed(() => isDesktop.value || isLocalMacLoopback.value)
const loading = ref(false)
const saving = ref(false)
const restarting = ref(false)
const requestError = ref('')
const status = ref<VoiceWakeStatus | null>(null)
const conversations = ref<Conversation[]>([])
const targetSelect = ref<HTMLSelectElement | null>(null)

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
const saveDisabled = computed(
  () => saving.value || (form.enabled && !canResolveTargetConversation.value)
)
const runtimeTriggers = computed(() =>
  status.value?.triggers?.length ? status.value.triggers.join(', ') : DEFAULT_WAKE_TRIGGER
)
const statusReason = computed(() => status.value?.reason?.trim() || '')
type ConversationOption = Conversation & { source?: 'current' | 'saved' }
const requestedConversationIDs = new Set<string>()

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
      return t('speech.voiceWake.messages.disabled')
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

const statusBadgeClass = computed(() => {
  if (!supportsConfiguration.value) {
    return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  }
  if (status.value?.running) {
    return 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400'
  }
  return 'bg-gray-200 text-gray-700 dark:bg-gray-600 dark:text-gray-200'
})
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
const speechStatusChipClass = computed(() =>
  status.value?.speech_authorized
    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
)
const microphoneStatusChipClass = computed(() =>
  status.value?.microphone_ready
    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
)
const platformLabel = computed(() => {
  if (status.value?.platform) return status.value.platform
  if (isDesktop.value) return 'desktop'
  if (isLocalMacLoopback.value) return 'localhost'
  return 'browser'
})
const showPlatformLabel = computed(() => platformLabel.value.trim().toLowerCase() !== 'darwin')

function normalizeTriggers(triggers?: string[]): string {
  if (!triggers || triggers.length === 0) return DEFAULT_WAKE_TRIGGER
  return triggers.join(', ')
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
  const settings = settingsStore.backendSettings
  form.enabled = settings.voice_wake_enabled ?? false
  form.triggers = normalizeTriggers(settings.voice_wake_triggers)
  form.locale = settings.voice_wake_locale ?? ''
  form.targetConversationID = settings.voice_wake_target_conversation_id ?? ''
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
  try {
    await settingsStore.updateBackendSettings({
      voice_wake_enabled: nextEnabled && !!nextTargetConversationID,
      voice_wake_triggers: parseTriggerInput(String(overrides.triggers ?? form.triggers)),
      voice_wake_locale: String(overrides.locale ?? form.locale).trim(),
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

async function saveSettings() {
  if (saveDisabled.value) return
  await persistSettings()
}

async function restartVoiceWake() {
  if (!canManageVoiceWake.value) return
  restarting.value = true
  try {
    const response = await voiceWakeApi.restart()
    status.value = response.data
    requestError.value = ''
  } catch (error) {
    requestError.value = extractErrorMessage(error)
  } finally {
    restarting.value = false
  }
}

async function recheckVoiceWake() {
  await refreshStatus()
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
  await persistSettings({ targetConversationID: option.id }, { revertOnError: true })
}

async function handleTargetConversationChange() {
  await persistSettings(
    { targetConversationID: form.targetConversationID },
    { revertOnError: true }
  )
}

async function handleEnabledChange() {
  if (form.enabled && !form.targetConversationID.trim() && currentConversationOption.value?.id) {
    form.targetConversationID = currentConversationOption.value.id
  }
  await persistSettings(
    {
      enabled: form.enabled,
      targetConversationID: form.targetConversationID,
    },
    { revertOnError: true }
  )
}

let statusPollTimer: ReturnType<typeof setInterval> | null = null

function stopStatusPolling() {
  if (statusPollTimer) {
    clearInterval(statusPollTimer)
    statusPollTimer = null
  }
}

function startStatusPolling() {
  if (!canManageVoiceWake.value || statusPollTimer) return
  statusPollTimer = setInterval(() => {
    if (typeof document !== 'undefined' && document.hidden) return
    void refreshStatus({ silent: true })
  }, STATUS_POLL_INTERVAL_MS)
}

watch(
  () => settingsStore.backendSettings,
  () => {
    normalizeSettingsFromStore()
  },
  { deep: true, immediate: true }
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
  await Promise.all([refreshStatus(), loadConversations()])
  startStatusPolling()
})

onUnmounted(() => {
  stopStatusPolling()
})
</script>

<template>
  <section
    v-if="shouldShow"
    data-testid="voicewake-section"
    class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm space-y-4"
  >
    <div class="flex items-start justify-between gap-4">
      <div class="flex items-start gap-3">
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
        <div class="space-y-1">
          <h4 class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('speech.voiceWake.title') }}
          </h4>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.description') }}
          </p>
        </div>
      </div>
      <div class="flex flex-wrap items-center justify-end gap-2">
        <span class="text-xs px-2 py-1 rounded-full font-medium" :class="statusBadgeClass">
          {{
            status?.running ? t('speech.voiceWake.statusRunning') : t('speech.voiceWake.statusIdle')
          }}
        </span>
        <button
          v-if="supportsConfiguration"
          data-testid="voicewake-restart"
          class="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-600 text-xs font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
          :disabled="restarting || loading"
          @click="restartVoiceWake"
        >
          {{ restarting ? t('common.retrying') : t('common.retry') }}
        </button>
      </div>
    </div>

    <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 space-y-4">
      <div class="flex flex-wrap items-center gap-2">
        <span
          v-if="showPlatformLabel"
          data-testid="voicewake-platform-status"
          class="inline-flex items-center rounded-full px-2 py-0.5 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200"
        >
          {{ platformLabel }}
        </span>
        <span
          data-testid="voicewake-speech-status"
          class="inline-flex items-center rounded-full px-2 py-0.5"
          :class="speechStatusChipClass"
        >
          {{ t('speech.voiceWake.speechStatusLabel') }}:
          {{
            status?.speech_authorized
              ? t('speech.voiceWake.statusReady')
              : t('speech.voiceWake.statusMissing')
          }}
        </span>
        <span
          data-testid="voicewake-mic-status"
          class="inline-flex items-center rounded-full px-2 py-0.5"
          :class="microphoneStatusChipClass"
        >
          {{ t('speech.voiceWake.microphoneStatusLabel') }}:
          {{
            status?.microphone_ready
              ? t('speech.voiceWake.statusReady')
              : t('speech.voiceWake.statusNotReady')
          }}
        </span>
      </div>

      <p data-testid="voicewake-status-message" class="text-sm" :class="statusMessageClass">
        {{ statusMessage }}
      </p>

      <div
        v-if="status?.last_error"
        data-testid="voicewake-status-error"
        class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-300"
      >
        <span class="font-medium">{{ t('speech.voiceWake.lastError') }}:</span>
        {{ status.last_error }}
      </div>

      <div v-if="supportsConfiguration" class="grid gap-3 sm:grid-cols-2">
        <div
          class="rounded-lg border border-gray-200 bg-white px-3 py-3 dark:border-gray-600 dark:bg-gray-800"
        >
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.activeTriggers') }}
          </p>
          <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
            {{ runtimeTriggers }}
          </p>
        </div>
        <div
          class="rounded-lg border border-gray-200 bg-white px-3 py-3 dark:border-gray-600 dark:bg-gray-800"
        >
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.targetConversation') }}
          </p>
          <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
            {{ runtimeTargetConversationLabel }}
          </p>
        </div>
        <div
          class="rounded-lg border border-gray-200 bg-white px-3 py-3 dark:border-gray-600 dark:bg-gray-800"
        >
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.lastTriggered') }}
          </p>
          <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
            {{ formatTimestamp(status?.last_triggered_at) }}
          </p>
        </div>
        <div
          class="rounded-lg border border-gray-200 bg-white px-3 py-3 dark:border-gray-600 dark:bg-gray-800"
        >
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.lastSent') }}
          </p>
          <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
            {{ formatTimestamp(status?.last_sent_at) }}
          </p>
        </div>
      </div>
    </div>

    <div v-if="supportsConfiguration" class="grid gap-4 lg:grid-cols-2">
      <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 space-y-4">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-1">
            <h5 class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('speech.voiceWake.enableTitle') }}
            </h5>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('speech.voiceWake.enableDescription') }}
            </p>
          </div>
          <label class="flex items-center gap-3">
            <span class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('common.enable') }}
            </span>
            <span
              class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
              :class="
                form.enabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'
              "
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

        <p
          v-if="enableBlocked"
          data-testid="voicewake-target-warning"
          class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-300"
        >
          {{ t('speech.voiceWake.fixedTargetWarning') }}
        </p>

        <div class="space-y-1">
          <h6 class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('speech.voiceWake.listeningTitle') }}
          </h6>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.listeningDescription') }}
          </p>
        </div>

        <label class="block space-y-2">
          <span class="text-xs font-medium text-gray-700 dark:text-gray-200">
            {{ t('speech.voiceWake.wakeWords') }}
          </span>
          <textarea
            v-model="form.triggers"
            data-testid="voicewake-triggers"
            rows="3"
            class="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white resize-y"
            :placeholder="`${t('voiceView.wakeWordPlaceholder')}, Jarvis`"
          />
          <p class="text-[11px] text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.wakeWordsHelp') }}
          </p>
        </label>

        <label class="block space-y-2">
          <span class="text-xs font-medium text-gray-700 dark:text-gray-200">
            {{ t('speech.voiceWake.localeOverride') }}
          </span>
          <input
            v-model="form.locale"
            data-testid="voicewake-locale"
            type="text"
            class="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white"
            placeholder="zh-CN"
          />
          <p class="text-[11px] text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.localeHelp') }}
          </p>
        </label>
      </div>

      <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 space-y-4">
        <div class="space-y-1">
          <h5 class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('speech.voiceWake.routingTitle') }}
          </h5>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('speech.voiceWake.targetDescription') }}
          </p>
        </div>

        <label class="block space-y-2">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-gray-700 dark:text-gray-200">
              {{ t('speech.voiceWake.targetFieldLabel') }}
            </span>
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

        <p class="text-[11px] text-gray-500 dark:text-gray-400">
          {{ t('speech.voiceWake.desktopNoteDescription') }}
        </p>
      </div>
    </div>

    <div v-if="supportsConfiguration" class="flex flex-wrap items-center gap-3">
      <button
        data-testid="voicewake-save"
        class="px-4 py-2 rounded-lg bg-gray-900 text-white text-sm font-medium hover:bg-gray-800 disabled:opacity-50"
        :disabled="saveDisabled"
        @click="saveSettings"
      >
        {{ saving ? t('common.saving') : t('common.save') }}
      </button>
      <span v-if="loading" class="text-xs text-gray-500 dark:text-gray-400">{{
        t('common.refreshing')
      }}</span>
      <span
        v-if="requestError && !status?.last_error"
        class="text-xs text-amber-600 dark:text-amber-400"
      >
        {{ requestError }}
      </span>
    </div>

    <div
      v-if="supportsConfiguration && statusReason !== 'running' && (statusReason || requestError)"
      class="flex flex-wrap items-center gap-2 pt-1"
    >
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
      <button
        data-testid="voicewake-recheck"
        class="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-600 text-xs font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700"
        @click="recheckVoiceWake"
      >
        {{ t('cardActions.recheck') }}
      </button>
      <button
        v-if="
          statusReason === 'start_failed' ||
          statusReason === 'runtime_error' ||
          statusReason === 'send_failed' ||
          statusReason === 'microphone_unavailable'
        "
        data-testid="voicewake-inline-retry"
        class="px-3 py-1.5 rounded-lg bg-gray-900 text-white text-xs font-medium hover:bg-gray-800 disabled:opacity-50"
        :disabled="restarting || loading"
        @click="restartVoiceWake"
      >
        {{ restarting ? t('common.retrying') : t('common.retry') }}
      </button>
    </div>
  </section>
</template>
