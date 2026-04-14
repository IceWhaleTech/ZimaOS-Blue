<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { conversationApi, type Conversation } from '@/api/chat'
import { voiceWakeApi, type VoiceWakeStatus } from '@/api/voiceWake'
import { useTauri } from '@/composables/useTauri'
import { isCurrentHostLoopback } from '@/utils/localPath'

const { t } = useI18n()
const { platform } = useTauri()
const route = useRoute()
const router = useRouter()

const IDLE_STATUS_POLL_INTERVAL_MS = 5000
const ACTIVE_STATUS_POLL_INTERVAL_MS = 1200
const RECENT_ACTIVITY_WINDOW_MS = 8000
const DEFAULT_WAKE_TRIGGER = 'Hey Blue'

const status = ref<VoiceWakeStatus | null>(null)
const targetConversation = ref<Conversation | null>(null)
const recentActivityType = ref<'triggered' | 'sent' | null>(null)

const isDesktop = computed(() => typeof window !== 'undefined' && !!window.__BLUE_DESKTOP__)
const isLocalMacLoopback = computed(
  () => !isDesktop.value && platform.value === 'macos' && isCurrentHostLoopback()
)
const canManageVoiceWake = computed(() => isDesktop.value || isLocalMacLoopback.value)
const supportsRuntime = computed(() => canManageVoiceWake.value && !!status.value?.supported)
const runtimeTriggers = computed(() =>
  status.value?.triggers?.length ? status.value.triggers.join(', ') : DEFAULT_WAKE_TRIGGER
)
const targetConversationLabel = computed(() => {
  const title = targetConversation.value?.title?.trim()
  if (title) return title
  const targetID = String(status.value?.target_conversation_id || '').trim()
  return targetID || t('speech.voiceWake.notSelected')
})
const targetConversationID = computed(() =>
  String(status.value?.target_conversation_id || '').trim()
)
const isViewingTargetConversation = computed(() => {
  if (route.name !== 'Chat') return false
  const rawConversationID = route.query.conversationId
  const currentConversationID = Array.isArray(rawConversationID)
    ? String(rawConversationID[0] || '').trim()
    : String(rawConversationID || '').trim()
  return !!currentConversationID && currentConversationID === targetConversationID.value
})
const activityState = computed<'idle' | 'listening' | 'triggered' | 'sent'>(() => {
  if (recentActivityType.value === 'sent') return 'sent'
  if (recentActivityType.value === 'triggered') return 'triggered'
  if (status.value?.running) return 'listening'
  return 'idle'
})
const shouldShowBanner = computed(
  () => supportsRuntime.value && (status.value?.running || recentActivityType.value !== null)
)
const bannerTitle = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return `${t('speech.voiceWake.lastSent')}: ${formatTimestamp(status.value?.last_sent_at)}`
    case 'triggered':
      return `${t('speech.voiceWake.lastTriggered')}: ${formatTimestamp(status.value?.last_triggered_at)}`
    case 'listening':
      return t('speech.voiceWake.messages.running')
    default:
      return t('speech.voiceWake.title')
  }
})
const bannerMeta = computed(() => {
  switch (activityState.value) {
    case 'sent':
    case 'triggered':
      return `${t('speech.voiceWake.targetConversation')}: ${targetConversationLabel.value}`
    case 'listening':
      return `${t('speech.voiceWake.activeTriggers')}: ${runtimeTriggers.value}`
    default:
      return ''
  }
})
const containerClass = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return 'voicewake-banner is-sent'
    case 'triggered':
      return 'voicewake-banner is-triggered'
    case 'listening':
      return 'voicewake-banner is-listening'
    default:
      return 'voicewake-banner'
  }
})
const showOpenTargetAction = computed(
  () =>
    recentActivityType.value !== null &&
    !!targetConversationID.value &&
    !isViewingTargetConversation.value
)
const dotClass = computed(() => {
  switch (activityState.value) {
    case 'sent':
      return 'voicewake-banner__dot is-sent'
    case 'triggered':
      return 'voicewake-banner__dot is-triggered'
    case 'listening':
      return 'voicewake-banner__dot is-listening'
    default:
      return 'voicewake-banner__dot'
  }
})

let statusPollTimer: ReturnType<typeof setTimeout> | null = null
let recentActivityTimer: ReturnType<typeof setTimeout> | null = null

function clearTimer(timer: ReturnType<typeof setTimeout> | null) {
  if (timer !== null) {
    clearTimeout(timer)
  }
}

async function openTargetConversation() {
  if (!targetConversationID.value) return
  await router.push({
    name: 'Chat',
    query: { conversationId: targetConversationID.value },
  })
}

function formatTimestamp(value?: string): string {
  const raw = String(value || '').trim()
  if (!raw) return t('speech.voiceWake.never')
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return raw
  return parsed.toLocaleTimeString()
}

async function refreshStatus() {
  if (!canManageVoiceWake.value) return
  try {
    const response = await voiceWakeApi.getStatus()
    status.value = response.data
  } catch (error) {
    console.error('Failed to refresh global VoiceWake status:', error)
    status.value = null
  }
}

async function loadTargetConversation() {
  const targetID = String(status.value?.target_conversation_id || '').trim()
  if (!targetID) {
    targetConversation.value = null
    return
  }
  if (targetConversation.value?.id === targetID && targetConversation.value.title?.trim()) {
    return
  }
  try {
    const response = await conversationApi.get(targetID)
    targetConversation.value = response.data
  } catch {
    targetConversation.value = {
      id: targetID,
      title: '',
      created_at: '',
      updated_at: '',
    }
  }
}

function isRecentTimestamp(value?: string): boolean {
  const parsed = Date.parse(String(value || ''))
  return Number.isFinite(parsed) && Date.now() - parsed <= RECENT_ACTIVITY_WINDOW_MS
}

function markRecentActivity(type: 'triggered' | 'sent') {
  clearTimer(recentActivityTimer)
  recentActivityType.value = type
  recentActivityTimer = setTimeout(() => {
    recentActivityType.value = null
  }, RECENT_ACTIVITY_WINDOW_MS)
}

function nextStatusPollInterval() {
  return status.value?.running ? ACTIVE_STATUS_POLL_INTERVAL_MS : IDLE_STATUS_POLL_INTERVAL_MS
}

function stopStatusPolling() {
  clearTimer(statusPollTimer)
  statusPollTimer = null
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
    await refreshStatus()
    scheduleStatusPolling()
  }, nextStatusPollInterval())
}

watch(
  () => status.value?.running,
  () => {
    if (!canManageVoiceWake.value) return
    scheduleStatusPolling()
  }
)

watch(
  () => status.value?.target_conversation_id,
  () => {
    void loadTargetConversation()
  },
  { immediate: true }
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

onMounted(async () => {
  if (!canManageVoiceWake.value) return
  await refreshStatus()
  await loadTargetConversation()
  scheduleStatusPolling()
})

onUnmounted(() => {
  stopStatusPolling()
  clearTimer(recentActivityTimer)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="voicewake-banner-transition">
      <div
        v-if="shouldShowBanner"
        data-testid="global-voicewake-banner"
        class="voicewake-banner-shell"
      >
        <div :class="containerClass">
          <div class="voicewake-banner__inner">
            <span :class="dotClass" />
            <div class="voicewake-banner__content">
              <p
                data-testid="global-voicewake-title"
                class="voicewake-banner__title"
              >
                {{ bannerTitle }}
              </p>
              <p
                data-testid="global-voicewake-meta"
                class="voicewake-banner__meta"
              >
                {{ bannerMeta }}
              </p>
            </div>
            <button
              v-if="showOpenTargetAction"
              data-testid="global-voicewake-open-target"
              type="button"
              class="voicewake-banner__action"
              @click="openTargetConversation"
            >
              {{ t('speech.voiceWake.openTargetChat') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.voicewake-banner-shell {
  position: fixed;
  top: 0.9rem;
  inset-inline: 0;
  margin-inline: auto;
  z-index: 40;
  width: min(calc(100vw - 1.25rem), 34rem);
  pointer-events: none;
}

.voicewake-banner {
  border: 1px solid rgba(148, 163, 184, 0.22);
  background: rgba(255, 255, 255, 0.88);
  color: #0f172a;
  border-radius: 1.1rem;
  box-shadow: 0 18px 40px -30px rgba(15, 23, 42, 0.42);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
  pointer-events: auto;
}

.voicewake-banner.is-listening {
  border-color: rgba(34, 197, 94, 0.28);
  background: rgba(240, 253, 244, 0.9);
}

.voicewake-banner.is-triggered {
  border-color: rgba(14, 165, 233, 0.34);
  background: rgba(240, 249, 255, 0.92);
}

.voicewake-banner.is-sent {
  border-color: rgba(34, 197, 94, 0.34);
  background: rgba(240, 253, 244, 0.94);
}

.voicewake-banner__inner {
  display: flex;
  align-items: flex-start;
  gap: 0.8rem;
  padding: 0.78rem 0.95rem;
}

.voicewake-banner__dot {
  width: 0.75rem;
  height: 0.75rem;
  margin-top: 0.25rem;
  border-radius: 999px;
  background: #94a3b8;
  box-shadow: 0 0 0 0 rgba(148, 163, 184, 0.28);
}

.voicewake-banner__dot.is-listening {
  background: #16a34a;
  animation: voicewake-pulse-green 1.8s ease-out infinite;
}

.voicewake-banner__dot.is-triggered {
  background: #0ea5e9;
  animation: voicewake-pulse-sky 0.95s ease-out 3;
}

.voicewake-banner__dot.is-sent {
  background: #16a34a;
  animation: voicewake-pulse-green 0.95s ease-out 3;
}

.voicewake-banner__content {
  min-width: 0;
  flex: 1;
}

.voicewake-banner__title {
  font-size: 0.92rem;
  font-weight: 600;
  line-height: 1.3;
}

.voicewake-banner__meta {
  margin-top: 0.16rem;
  font-size: 0.78rem;
  line-height: 1.35;
  color: rgba(51, 65, 85, 0.82);
}

.voicewake-banner__action {
  flex: none;
  align-self: center;
  border: 1px solid rgba(148, 163, 184, 0.26);
  background: rgba(255, 255, 255, 0.82);
  color: inherit;
  border-radius: 999px;
  padding: 0.42rem 0.72rem;
  font-size: 0.74rem;
  font-weight: 600;
  line-height: 1;
  transition:
    background-color 0.16s ease,
    border-color 0.16s ease,
    transform 0.16s ease;
}

.voicewake-banner__action:hover {
  background: rgba(255, 255, 255, 0.96);
  border-color: rgba(100, 116, 139, 0.38);
  transform: translateY(-1px);
}

.voicewake-banner-transition-enter-active,
.voicewake-banner-transition-leave-active {
  transition:
    opacity 0.22s ease,
    transform 0.22s ease;
}

.voicewake-banner-transition-enter-from,
.voicewake-banner-transition-leave-to {
  opacity: 0;
  transform: translate(-50%, -10px);
}

@keyframes voicewake-pulse-green {
  0% {
    box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.34);
  }
  100% {
    box-shadow: 0 0 0 14px rgba(34, 197, 94, 0);
  }
}

@keyframes voicewake-pulse-sky {
  0% {
    box-shadow: 0 0 0 0 rgba(14, 165, 233, 0.38);
  }
  100% {
    box-shadow: 0 0 0 16px rgba(14, 165, 233, 0);
  }
}

:root.dark .voicewake-banner,
[data-theme='dark'] .voicewake-banner,
html.dark .voicewake-banner {
  border-color: rgba(100, 116, 139, 0.36);
  background: rgba(15, 23, 42, 0.82);
  color: #e2e8f0;
  box-shadow: 0 24px 44px -30px rgba(2, 6, 23, 0.78);
}

:root.dark .voicewake-banner.is-listening,
[data-theme='dark'] .voicewake-banner.is-listening,
html.dark .voicewake-banner.is-listening {
  border-color: rgba(34, 197, 94, 0.28);
  background: rgba(20, 83, 45, 0.38);
}

:root.dark .voicewake-banner.is-triggered,
[data-theme='dark'] .voicewake-banner.is-triggered,
html.dark .voicewake-banner.is-triggered {
  border-color: rgba(14, 165, 233, 0.3);
  background: rgba(12, 74, 110, 0.42);
}

:root.dark .voicewake-banner.is-sent,
[data-theme='dark'] .voicewake-banner.is-sent,
html.dark .voicewake-banner.is-sent {
  border-color: rgba(34, 197, 94, 0.3);
  background: rgba(20, 83, 45, 0.42);
}

:root.dark .voicewake-banner__meta,
[data-theme='dark'] .voicewake-banner__meta,
html.dark .voicewake-banner__meta {
  color: rgba(226, 232, 240, 0.78);
}

:root.dark .voicewake-banner__action,
[data-theme='dark'] .voicewake-banner__action,
html.dark .voicewake-banner__action {
  border-color: rgba(100, 116, 139, 0.38);
  background: rgba(30, 41, 59, 0.82);
}

:root.dark .voicewake-banner__action:hover,
[data-theme='dark'] .voicewake-banner__action:hover,
html.dark .voicewake-banner__action:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(148, 163, 184, 0.44);
}

@media (max-width: 640px) {
  .voicewake-banner-shell {
    top: 0.75rem;
    width: min(calc(100vw - 1rem), 30rem);
  }

  .voicewake-banner__inner {
    padding: 0.72rem 0.82rem;
  }

  .voicewake-banner__inner {
    flex-wrap: wrap;
  }

  .voicewake-banner__action {
    margin-inline-start: 1.55rem;
  }
}
</style>
