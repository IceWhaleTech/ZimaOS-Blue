<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { conversationApi, type Conversation } from '@/api/chat'
import { voiceWakeApi, type VoiceWakeStatus } from '@/api/voiceWake'
import { useSettingsStore } from '@/stores/settings'

const settingsStore = useSettingsStore()

const isDesktop = computed(() => typeof window !== 'undefined' && !!window.__BLUE_DESKTOP__)
const loading = ref(false)
const saving = ref(false)
const restarting = ref(false)
const requestError = ref('')
const status = ref<VoiceWakeStatus | null>(null)
const conversations = ref<Conversation[]>([])

const form = reactive({
  enabled: false,
  triggers: 'Blue',
  locale: '',
  targetConversationID: '',
})

const shouldShow = computed(() => isDesktop.value && !!status.value?.supported)
const hasTargetConversation = computed(() => form.targetConversationID.trim().length > 0)
const enableBlocked = computed(() => !hasTargetConversation.value)
const saveDisabled = computed(() => saving.value || (form.enabled && !hasTargetConversation.value))
const runtimeTriggers = computed(() =>
  status.value?.triggers?.length ? status.value.triggers.join(', ') : 'Blue'
)

const statusMessage = computed(() => {
  const reason = status.value?.reason?.trim() || ''
  switch (reason) {
    case 'running':
      return 'Listening for wake words in the background.'
    case 'disabled':
      return 'VoiceWake is off.'
    case 'target_missing':
      return 'Choose a target conversation before enabling VoiceWake.'
    case 'target_unavailable':
      return 'The selected conversation is unavailable. Pick another conversation and save again.'
    case 'speech_permission_denied':
      return 'Speech recognition permission is missing.'
    case 'microphone_unavailable':
      return 'Microphone input is unavailable.'
    case 'start_failed':
      return 'VoiceWake failed to start.'
    case 'send_failed':
      return 'VoiceWake captured a command but failed to send it.'
    case 'runtime_error':
      return 'VoiceWake stopped after a runtime error.'
    case 'unsupported':
      return 'VoiceWake is only available in the embedded macOS desktop app.'
    default:
      return requestError.value || 'VoiceWake status is unavailable.'
  }
})

const statusToneClass = computed(() => {
  if (status.value?.running) {
    return 'border-emerald-200 bg-emerald-50 text-emerald-800'
  }
  if (status.value?.reason === 'disabled' || status.value?.reason === 'target_missing') {
    return 'border-slate-200 bg-slate-50 text-slate-700'
  }
  return 'border-amber-200 bg-amber-50 text-amber-800'
})

function normalizeTriggers(triggers?: string[]): string {
  if (!triggers || triggers.length === 0) return 'Blue'
  return triggers.join(', ')
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
  if (!raw) return 'Never'
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

async function refreshStatus() {
  if (!isDesktop.value) return
  loading.value = true
  try {
    const response = await voiceWakeApi.getStatus()
    status.value = response.data
    requestError.value = ''
  } catch (error) {
    requestError.value = extractErrorMessage(error)
    status.value = null
  } finally {
    loading.value = false
  }
}

async function loadConversations() {
  if (!isDesktop.value) return
  try {
    const response = await conversationApi.list(20, 0)
    conversations.value = Array.isArray(response.data) ? response.data : []
  } catch (error) {
    requestError.value = extractErrorMessage(error)
    conversations.value = []
  }
}

async function saveSettings() {
  if (saveDisabled.value) return
  saving.value = true
  try {
    await settingsStore.updateBackendSettings({
      voice_wake_enabled: form.enabled && hasTargetConversation.value,
      voice_wake_triggers: parseTriggerInput(form.triggers),
      voice_wake_locale: form.locale.trim(),
      voice_wake_target_conversation_id: form.targetConversationID.trim(),
    })
    normalizeSettingsFromStore()
    await refreshStatus()
    await loadConversations()
  } catch (error) {
    requestError.value = extractErrorMessage(error)
  } finally {
    saving.value = false
  }
}

async function restartVoiceWake() {
  if (!isDesktop.value) return
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

watch(
  () => settingsStore.backendSettings,
  () => {
    normalizeSettingsFromStore()
  },
  { deep: true, immediate: true }
)

onMounted(async () => {
  if (!isDesktop.value) return
  if (Object.keys(settingsStore.backendSettings).length === 0) {
    await settingsStore.fetchBackendSettings()
  }
  await Promise.all([refreshStatus(), loadConversations()])
})
</script>

<template>
  <section
    v-if="shouldShow"
    data-testid="voicewake-section"
    class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm space-y-4"
  >
    <div class="flex items-start justify-between gap-4">
      <div class="space-y-1">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">System VoiceWake</h4>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          Keep listening in the background and send the captured command to one fixed conversation.
        </p>
      </div>
      <button
        data-testid="voicewake-restart"
        class="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-600 text-xs font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
        :disabled="restarting || loading"
        @click="restartVoiceWake"
      >
        {{ restarting ? 'Retrying...' : 'Retry' }}
      </button>
    </div>

    <div class="rounded-xl border px-3 py-3 text-xs space-y-2" :class="statusToneClass">
      <div class="flex flex-wrap items-center gap-2">
        <span
          class="inline-flex items-center rounded-full px-2 py-0.5 font-medium"
          :class="status?.running ? 'bg-emerald-100 text-emerald-700' : 'bg-white/70 text-current'"
        >
          {{ status?.running ? 'Running' : 'Idle' }}
        </span>
        <span class="inline-flex items-center rounded-full px-2 py-0.5 bg-white/70">
          {{ status?.platform || 'unknown' }}
        </span>
        <span class="inline-flex items-center rounded-full px-2 py-0.5 bg-white/70">
          Speech: {{ status?.speech_authorized ? 'ready' : 'missing' }}
        </span>
        <span class="inline-flex items-center rounded-full px-2 py-0.5 bg-white/70">
          Mic: {{ status?.microphone_ready ? 'ready' : 'not ready' }}
        </span>
      </div>
      <p data-testid="voicewake-status-message">{{ statusMessage }}</p>
      <p v-if="status?.last_error" data-testid="voicewake-status-error" class="font-medium">
        Last error: {{ status.last_error }}
      </p>
      <div class="grid gap-2 text-[11px] text-current/80 sm:grid-cols-2">
        <p>Active triggers: {{ runtimeTriggers }}</p>
        <p>Target conversation: {{ status?.target_conversation_id || 'Not selected' }}</p>
        <p>Last triggered: {{ formatTimestamp(status?.last_triggered_at) }}</p>
        <p>Last sent: {{ formatTimestamp(status?.last_sent_at) }}</p>
      </div>
    </div>

    <div class="grid gap-4 md:grid-cols-2">
      <label class="space-y-1">
        <span class="text-xs font-medium text-gray-700 dark:text-gray-200">Wake words</span>
        <input
          v-model="form.triggers"
          data-testid="voicewake-triggers"
          type="text"
          class="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white"
          placeholder="Blue, Jarvis"
        />
      </label>

      <label class="space-y-1">
        <span class="text-xs font-medium text-gray-700 dark:text-gray-200">Locale override</span>
        <input
          v-model="form.locale"
          data-testid="voicewake-locale"
          type="text"
          class="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white"
          placeholder="en-US"
        />
      </label>
    </div>

    <div class="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
      <label class="space-y-1">
        <span class="text-xs font-medium text-gray-700 dark:text-gray-200"
          >Target conversation</span
        >
        <select
          v-model="form.targetConversationID"
          data-testid="voicewake-target"
          class="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white"
        >
          <option value="">Select a conversation</option>
          <option
            v-for="conversation in conversations"
            :key="conversation.id"
            :value="conversation.id"
          >
            {{ conversation.title || conversation.id }}
          </option>
        </select>
      </label>

      <label
        class="inline-flex items-center gap-3 rounded-xl border border-gray-200 dark:border-gray-600 px-4 py-3"
      >
        <span class="text-sm font-medium text-gray-900 dark:text-white">Enable</span>
        <input
          v-model="form.enabled"
          data-testid="voicewake-enabled"
          type="checkbox"
          class="h-4 w-4 rounded border-gray-300 text-gray-900 focus:ring-gray-500 disabled:cursor-not-allowed"
          :disabled="enableBlocked || saving"
        />
      </label>
    </div>

    <p
      v-if="enableBlocked"
      data-testid="voicewake-target-warning"
      class="text-xs text-amber-600 dark:text-amber-400"
    >
      Pick the fixed target conversation before turning VoiceWake on.
    </p>

    <div class="flex flex-wrap items-center gap-3">
      <button
        data-testid="voicewake-save"
        class="px-4 py-2 rounded-lg bg-gray-900 text-white text-sm font-medium hover:bg-gray-800 disabled:opacity-50"
        :disabled="saveDisabled"
        @click="saveSettings"
      >
        {{ saving ? 'Saving...' : 'Save VoiceWake' }}
      </button>
      <span v-if="loading" class="text-xs text-gray-500 dark:text-gray-400"
        >Refreshing status...</span
      >
      <span
        v-if="requestError && !status?.last_error"
        class="text-xs text-amber-600 dark:text-amber-400"
      >
        {{ requestError }}
      </span>
    </div>
  </section>
</template>
