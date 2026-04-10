<script setup lang="ts">
import {
  ref,
  onMounted,
  nextTick,
  watch,
  computed,
  onUnmounted,
  inject,
  defineAsyncComponent,
  type ComputedRef,
} from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import type { ComponentPublicInstance } from 'vue'
import type { Message as ChatMessageRecord } from '@/api/chat'
import { useChatStore, type ActiveMessageStreamState, type StreamUIState } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { useSystemStore } from '@/stores/system'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useDeepResearchJobsStore } from '@/stores/deepResearchJobs'
import { useTaskProjectionsStore } from '@/stores/taskProjections'
import { useChatShortcuts } from '@/composables/useKeyboardShortcuts'
import { getLocaleDirection } from '@/i18n'
import type { FileAttachment } from '@/components/ChatInput.vue'
import type ChatInputComponent from '@/components/ChatInput.vue'
import type VirtualScrollComponent from '@/components/VirtualScroll.vue'
import {
  settingsApi,
  type AgentcoreRunnerLastRun,
  type AgentcoreRunnerTagList,
} from '@/api/settings'
import type { UserTaskProjection } from '@/api/tasks'
import { useMediaGenerate } from '@/composables/useMediaGenerate'
import { componentPool } from '@/utils/componentPool'
import { clearConversationIncrementalStates } from '@/utils/typeless'
import { formatTokens } from '@/utils/format'
import { findLatestTodoChecklistSummary } from '@/utils/todoChecklist'
import { reportStartupMark } from '@/utils/startupTrace'
import type { Provider } from '@/api/providerPool'
import { rafThrottle } from '@/utils/rafThrottle'
import {
  getCurrentConversationDeepResearchJobs,
  hasCancelableChatWork,
} from '@/utils/chatCancelableWork'
import { useTaskProjectionActions } from '@/composables/useTaskProjectionActions'
import { measureChatPerf, recordChatPerfCount } from '@/utils/chatPerf'
import { scheduleStartupBackgroundTask } from '@/utils/startupBackgroundTask'
import type { TypelessCardRunnerExecution } from '@/types/typeless'

reportStartupMark('chat_view_setup_enter')

const ChatMessage = defineAsyncComponent(() => import('@/components/ChatMessage.vue'))
const ConversationList = defineAsyncComponent(() => import('@/components/ConversationList.vue'))
const ChatInput = defineAsyncComponent(() => import('@/components/ChatInput.vue'))
const VirtualScroll = defineAsyncComponent(() => import('@/components/VirtualScroll.vue'))
const TypelessCardComponent = defineAsyncComponent(
  () => import('@/components/typeless/TypelessCard.vue')
)
const PresetQuestions = defineAsyncComponent(
  () => import('@/components/onboarding/PresetQuestions.vue')
)
const TalkMode = defineAsyncComponent(() => import('@/components/chat/TalkMode.vue'))
const ToolApprovalDialog = defineAsyncComponent(() => import('@/components/ToolApprovalDialog.vue'))
const ExecApprovalDialog = defineAsyncComponent(() => import('@/components/ExecApprovalDialog.vue'))
const TaskActionDialog = defineAsyncComponent(() => import('@/components/TaskActionDialog.vue'))
const MediaParamPanel = defineAsyncComponent(() => import('@/components/MediaParamPanel.vue'))
const UserTaskProjectionCard = defineAsyncComponent(
  () => import('@/components/UserTaskProjectionCard.vue')
)
const UserTaskProjectionDock = defineAsyncComponent(
  () => import('@/components/UserTaskProjectionDock.vue')
)
const DeepResearchTaskDock = defineAsyncComponent(
  () => import('@/components/DeepResearchTaskDock.vue')
)

const { t, te, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const toggleAppSidebar = inject<() => void>('toggleAppSidebar', () => {})
const hasGlobalMobileSidebarToggle = inject<ComputedRef<boolean>>(
  'hasGlobalMobileSidebarToggle',
  computed(() => false)
)
const chatStore = useChatStore()
const settingsStore = useSettingsStore()
const systemStore = useSystemStore()
const providerPoolStore = useProviderPoolStore()
const deepResearchJobs = useDeepResearchJobsStore()
const taskProjections = useTaskProjectionsStore()
const mediaGen = useMediaGenerate()

type VoiceApiModule = typeof import('@/api/voice')
type MarkdownModule = typeof import('@/utils/markdown')
type ApiClientModule = typeof import('@/api/client')

let voiceApiModulePromise: Promise<VoiceApiModule> | null = null
let markdownModulePromise: Promise<MarkdownModule> | null = null
let apiClientModulePromise: Promise<ApiClientModule> | null = null
const startupBackgroundTaskCleanups: Array<() => void> = []
let viewUnmounted = false
let initialChatBootstrapStarted = false

function loadVoiceApiModule(): Promise<VoiceApiModule> {
  if (!voiceApiModulePromise) {
    voiceApiModulePromise = import('@/api/voice')
  }
  return voiceApiModulePromise
}

function loadMarkdownModule(): Promise<MarkdownModule> {
  if (!markdownModulePromise) {
    markdownModulePromise = import('@/utils/markdown')
  }
  return markdownModulePromise
}

function loadApiClientModule(): Promise<ApiClientModule> {
  if (!apiClientModulePromise) {
    apiClientModulePromise = import('@/api/client')
  }
  return apiClientModulePromise
}

function syncStreamingTtsLocale(nextLocale: string) {
  loadVoiceApiModule()
    .then(({ streamingTTSManager }) => {
      streamingTTSManager.setLocale(nextLocale)
    })
    .catch(() => {})
}

function scheduleBackgroundTask(task: () => void, timeout = 1500): () => void {
  return scheduleStartupBackgroundTask(task, timeout)
}

function queueBackgroundTask(task: () => void, timeout = 1500) {
  startupBackgroundTaskCleanups.push(scheduleBackgroundTask(task, timeout))
}

function clearBackgroundTasks() {
  while (startupBackgroundTaskCleanups.length > 0) {
    startupBackgroundTaskCleanups.pop()?.()
  }
}

function queuePostPaintTask(task: () => void) {
  const handle = window.requestAnimationFrame(() => task())
  startupBackgroundTaskCleanups.push(() => {
    window.cancelAnimationFrame(handle)
  })
}
syncStreamingTtsLocale(locale.value)
watch(locale, (newLocale) => {
  syncStreamingTtsLocale(newLocale)
})

// Trial quota animation state
const tokenAnimating = ref(false)
const previousTokens = ref<number | null>(null)
const ACTIVE_TODO_PANEL_COLLAPSED_KEY = 'zima.chat.active_todo_collapsed.v1'

const messagesContainer = ref<HTMLElement | null>(null)
const virtualScrollRef = ref<InstanceType<typeof VirtualScrollComponent> | null>(null)
const chatInputRef = ref<InstanceType<typeof ChatInputComponent> | null>(null)
const messageHeightCache = new Map<string, number>()
const virtualElementToMessageKey = new Map<HTMLElement, string>()
const virtualItemElements = new Map<string, HTMLElement>()
const virtualItemHeightUpdaters = new Map<string, (height: number) => void>()
const virtualItemRefCallbacks = new Map<
  string,
  (el: Element | ComponentPublicInstance | null) => void
>()
let virtualResizeObserver: ResizeObserver | null = null
const showSidebar = ref(false) // Default closed on mobile
// Detect mobile synchronously before first render to avoid layout flash
const _initMobile = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini/i.test(
  navigator.userAgent
)
const _initConvId = new URLSearchParams(window.location.search).get('conversationId')
const isMobile = ref(_initMobile)
const isNarrowScreen = ref(window.innerWidth < 768) // PC narrow screen (<768px)
const shouldCollapseTopbarControls = computed(() => isMobile.value)
const isMac = computed(() => navigator.platform.toUpperCase().indexOf('MAC') >= 0)
// If URL has conversationId on mobile, pre-populate pageStack so we skip list page on first render
const pageStack = ref<string[]>(_initMobile && _initConvId ? [_initConvId] : [])
const showListPage = computed(() => isMobile.value && pageStack.value.length === 0)
const mobileAnimationEnabled = ref(false) // Only animate after user interaction, not on page load
const showTopbarMenu = ref(false)
type MobileFeatureSheetKind = 'deep-research' | 'smart-resume'
const activeMobileFeatureSheet = ref<MobileFeatureSheetKind | null>(null)
const isRtl = computed(() => getLocaleDirection(locale.value) === 'rtl')

const presetQuestionDraft = ref('')
const presetQuestionContextText = computed(() => {
  const recentMessages = chatStore.messages
    .slice(-6)
    .map((message) => String(message.content || '').trim())
    .filter(Boolean)
    .join('\n')
  return [recentMessages, presetQuestionDraft.value.trim()].filter(Boolean).join('\n')
})

const initialPrimaryDataHydrating = ref(false)
const showInitialThreadSkeleton = computed(
  () => initialPrimaryDataHydrating.value && chatStore.messages.length === 0 && !chatStore.error
)

function normalizeRunnerExecutionText(value: unknown): string {
  if (typeof value !== 'string') return ''
  return value.trim()
}

function normalizeAgentcoreRunnerRefValue(value: unknown): string {
  if (typeof value === 'string' && value.trim()) return value.trim()
  return 'main'
}

type AgentcoreRunnerDisplayTone = 'built-in' | 'agentcore'

type AgentcoreRunnerDisplayOption = {
  value: string
  label: string
  tone: AgentcoreRunnerDisplayTone
}

function isVersionLikeAgentcoreRunnerValue(value: string): boolean {
  return /^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(value)
}

function buildRunnerExecutionCard(run: AgentcoreRunnerLastRun): TypelessCardRunnerExecution {
  return {
    type: 'runner-execution',
    id: 'chat-runner-execution-card',
    eyebrow: t('settings.agentcoreRunner.eyebrow', 'Harness · Beta'),
    title: t('settings.agentcoreRunner.title', 'Harness Self-Iterating Agentcore Runner'),
    reason: normalizeRunnerExecutionText(run.reason),
    candidate_id: normalizeRunnerExecutionText(run.candidate_id),
    eval_run_id: normalizeRunnerExecutionText(run.eval_run_id),
    optimization_surface: normalizeRunnerExecutionText(run.optimization_surface),
    runner_protocol: normalizeRunnerExecutionText(run.runner_protocol),
    runner_stop_reason: normalizeRunnerExecutionText(run.runner_stop_reason),
    runner_duration_ms:
      typeof run.runner_duration_ms === 'number' ? run.runner_duration_ms : undefined,
    runner_response_text: normalizeRunnerExecutionText(run.runner_response_text),
    runner_stderr: normalizeRunnerExecutionText(run.runner_stderr),
    runner_error: normalizeRunnerExecutionText(run.runner_error),
    runner_transcript: Array.isArray(run.runner_transcript)
      ? run.runner_transcript.map((entry) => ({
          direction: normalizeRunnerExecutionText(entry.direction),
          method: normalizeRunnerExecutionText(entry.method),
          text: normalizeRunnerExecutionText(entry.text),
        }))
      : [],
  }
}

const runnerExecutionCard = computed<TypelessCardRunnerExecution | null>(() => {
  if (!settingsStore.experimentalAgentcoreRunnerEnabled) return null
  if (chatStore.messages.length === 0) return null
  const run = settingsStore.agentcoreRunnerLastRun
  if (!run) return null
  return buildRunnerExecutionCard(run)
})

const agentcoreRunnerTags = ref<AgentcoreRunnerTagList | null>(null)
const agentcoreRunnerTagsLoading = ref(false)
const agentcoreRunnerSyncing = ref(false)
let agentcoreRunnerSyncPromise: Promise<void> | null = null
let agentcoreRunnerSyncTargetRef = ''

const showAgentcoreRunnerControl = computed(() => settingsStore.experimentalAgentcoreRunnerEnabled)
const agentcoreRunnerSelectValue = computed(() =>
  normalizeAgentcoreRunnerRefValue(chatStore.agentcoreRunnerRef)
)
const agentcoreRunnerRepoDefaultRef = computed(() =>
  normalizeAgentcoreRunnerRefValue(agentcoreRunnerTags.value?.default_ref)
)
const agentcoreRunnerBuiltInVersion = computed(() =>
  normalizeRunnerExecutionText(systemStore.health?.version)
)
const agentcoreRunnerKnownTagSet = computed(() => {
  const knownTags = new Set<string>()
  for (const tag of agentcoreRunnerTags.value?.tags ?? []) {
    knownTags.add(normalizeAgentcoreRunnerRefValue(tag))
  }
  return knownTags
})

function isBuiltInAgentcoreRunnerValue(value: unknown): boolean {
  const normalized = normalizeAgentcoreRunnerRefValue(value)
  if (normalized === agentcoreRunnerBuiltInVersion.value) return true
  if (normalized === agentcoreRunnerRepoDefaultRef.value) return false
  return (
    isVersionLikeAgentcoreRunnerValue(normalized) &&
    !agentcoreRunnerKnownTagSet.value.has(normalized)
  )
}

function getAgentcoreRunnerDisplayTone(value: unknown): AgentcoreRunnerDisplayTone {
  return isBuiltInAgentcoreRunnerValue(value) ? 'built-in' : 'agentcore'
}

function getAgentcoreRunnerDisplayLabel(value: unknown): string {
  const normalized = normalizeAgentcoreRunnerRefValue(value)
  if (isBuiltInAgentcoreRunnerValue(normalized)) return t('common.default', 'Default')
  if (normalized === agentcoreRunnerRepoDefaultRef.value) return t('common.preview', 'Preview')
  return normalized
}

const agentcoreRunnerDisplayOptions = computed<AgentcoreRunnerDisplayOption[]>(() =>
  agentcoreRunnerRefOptions.value.map((value) => ({
    value,
    label: getAgentcoreRunnerDisplayLabel(value),
    tone: getAgentcoreRunnerDisplayTone(value),
  }))
)
const agentcoreRunnerSelectedOption = computed<AgentcoreRunnerDisplayOption>(() => {
  const selectedValue = agentcoreRunnerSelectValue.value
  return (
    agentcoreRunnerDisplayOptions.value.find((option) => option.value === selectedValue) ?? {
      value: selectedValue,
      label: getAgentcoreRunnerDisplayLabel(selectedValue),
      tone: getAgentcoreRunnerDisplayTone(selectedValue),
    }
  )
})
const agentcoreRunnerSelectedLabel = computed(() => agentcoreRunnerSelectedOption.value.label)
const agentcoreRunnerSelectedToneClass = computed(() => ({
  'is-built-in': agentcoreRunnerSelectedOption.value.tone === 'built-in',
  'is-agentcore': agentcoreRunnerSelectedOption.value.tone === 'agentcore',
}))
const agentcoreRunnerButtonTitle = computed(
  () =>
    `${t('settings.agentcoreRunner.refLabel', 'Runner version')} · ${agentcoreRunnerSelectedLabel.value}`
)
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
  push(chatStore.agentcoreRunnerRef)
  push(settingsStore.experimentalAgentcoreRunnerRef)
  for (const tag of agentcoreRunnerTags.value?.tags ?? []) {
    push(tag)
  }
  return options
})
const agentcoreRunnerSelectionBusy = computed(
  () => chatStore.streaming || chatStore.sending || chatStore.toolExecuting
)

async function fetchAgentcoreRunnerTags(
  repoURL = settingsStore.experimentalAgentcoreRunnerRepoURL
) {
  const resolvedRepoURL = repoURL?.trim() || settingsStore.experimentalAgentcoreRunnerRepoURL
  try {
    agentcoreRunnerTagsLoading.value = true
    const response = await settingsApi.getAgentcoreRunnerTags(resolvedRepoURL)
    agentcoreRunnerTags.value = response.data
  } catch {
    agentcoreRunnerTags.value = {
      repo_url: resolvedRepoURL || 'https://github.com/IceWhaleTech/ZimaOS-Blue',
      default_ref: 'main',
      tags: [],
    }
  } finally {
    agentcoreRunnerTagsLoading.value = false
  }
}

async function syncAgentcoreRunnerRefOnce(targetRef: string) {
  const activeSettingRef = normalizeAgentcoreRunnerRefValue(
    settingsStore.experimentalAgentcoreRunnerRef
  )
  const status = settingsStore.agentcoreRunnerStatus
  const hasStatus = status != null
  const resolvedStatusRef = hasStatus ? normalizeAgentcoreRunnerRefValue(status?.resolved_ref) : ''
  const needsRefUpdate = targetRef !== activeSettingRef
  const needsPrepare =
    needsRefUpdate || (hasStatus && (!status?.binary_ready || resolvedStatusRef !== targetRef))

  if (!needsRefUpdate && !needsPrepare) {
    return
  }

  if (needsRefUpdate) {
    await settingsStore.updateBackendSettings({
      experimental_agentcore_runner_ref: targetRef,
    })
  }
  if (needsPrepare) {
    await settingsStore.prepareAgentcoreRunner()
    await settingsStore.fetchAgentcoreRunnerStatus().catch(() => {})
  }
}

async function syncActiveAgentcoreRunnerRef(targetRefRaw: unknown) {
  if (!settingsStore.experimentalAgentcoreRunnerEnabled) return

  agentcoreRunnerSyncTargetRef = normalizeAgentcoreRunnerRefValue(targetRefRaw)
  if (agentcoreRunnerSyncPromise) {
    return agentcoreRunnerSyncPromise
  }

  const task = (async () => {
    try {
      agentcoreRunnerSyncing.value = true
      while (settingsStore.experimentalAgentcoreRunnerEnabled && agentcoreRunnerSyncTargetRef) {
        const nextTargetRef = agentcoreRunnerSyncTargetRef
        agentcoreRunnerSyncTargetRef = ''
        await syncAgentcoreRunnerRefOnce(nextTargetRef)
      }
    } finally {
      agentcoreRunnerSyncing.value = false
      agentcoreRunnerSyncPromise = null
    }
  })()
  agentcoreRunnerSyncPromise = task
  return task
}

async function persistAgentcoreRunnerRefSelection(targetRefRaw: unknown) {
  const targetRef = normalizeAgentcoreRunnerRefValue(targetRefRaw)
  await chatStore.setAgentcoreRunnerRef(targetRef)
}

function handleAgentcoreRunnerRefChange(event: Event) {
  const target = event.target
  if (!(target instanceof HTMLSelectElement)) return
  void persistAgentcoreRunnerRefSelection(target.value).catch(() => {})
}

watch(
  () => settingsStore.experimentalAgentcoreRunnerEnabled,
  (enabled) => {
    if (!enabled) {
      agentcoreRunnerTags.value = null
      return
    }
    void fetchAgentcoreRunnerTags().catch(() => {})
    void syncActiveAgentcoreRunnerRef(chatStore.agentcoreRunnerRef).catch(() => {})
  },
  { immediate: true }
)

watch(
  () => settingsStore.experimentalAgentcoreRunnerRepoURL,
  (repoURL) => {
    if (!settingsStore.experimentalAgentcoreRunnerEnabled) return
    void fetchAgentcoreRunnerTags(repoURL).catch(() => {})
  }
)

watch(
  () => chatStore.agentcoreRunnerRef,
  (value, previousValue) => {
    if (!settingsStore.experimentalAgentcoreRunnerEnabled) return
    if (
      normalizeAgentcoreRunnerRefValue(value) === normalizeAgentcoreRunnerRefValue(previousValue)
    ) {
      return
    }
    void syncActiveAgentcoreRunnerRef(value).catch(() => {})
  }
)

const showRoutingMenu = ref(false)
const routingMenuAnchorEl = ref<HTMLElement | null>(null)
const desktopRoutingMenuAnchorEl = ref<HTMLElement | null>(null)
const routingMenuFloatingRef = ref<HTMLElement | null>(null)
const routingMenuPosition = ref({ x: 0, y: 0 })

function normalizeConversationRouteParam(value: unknown): string {
  if (Array.isArray(value)) {
    return normalizeConversationRouteParam(value[0] ?? '')
  }
  return typeof value === 'string' ? value.trim() : ''
}

// Context trim indicator
const showContextTrim = ref(false)
let contextTrimTimer: ReturnType<typeof setTimeout> | null = null
watch(
  () => chatStore.contextTrimInfo,
  (info) => {
    if (contextTrimTimer) {
      clearTimeout(contextTrimTimer)
      contextTrimTimer = null
    }
    if (info) {
      // For pruned events, only show when this turn actually saved tokens.
      if (info.type === 'pruned') {
        const saved = (info.tokensBefore ?? 0) - (info.tokensAfter ?? 0)
        if (saved <= 0) {
          showContextTrim.value = false
          return
        }
        // Prune notices are quieter; only show when they persist for a moment.
        contextTrimTimer = setTimeout(() => {
          if (chatStore.streaming || info) showContextTrim.value = true
        }, 1000)
        return
      }

      // Compaction status should be visible immediately so users know what is happening.
      showContextTrim.value = true
    } else {
      showContextTrim.value = false
    }
  }
)
// Auto-hide after streaming ends (with a short delay so user can read it)
watch(
  () => chatStore.streaming,
  (val) => {
    if (!val && showContextTrim.value) {
      setTimeout(() => {
        showContextTrim.value = false
      }, 3000)
    }
  }
)

// Talk mode state
const showTalkMode = ref(false)
watch(showTalkMode, (open) => {
  if (open) {
    loadVoiceApiModule()
      .then(({ streamingTTSManager }) => {
        streamingTTSManager.stop()
      })
      .catch(() => {})
  }
})

const currentConversationActiveTasks = computed(() => taskProjections.currentActiveTasks)
const hasBackgroundTasks = computed(() => taskProjections.backgroundTasks.length > 0)
const hasActiveDeepResearchJobs = computed(
  () => Array.isArray(deepResearchJobs.activeJobs) && deepResearchJobs.activeJobs.length > 0
)
const currentConversationDeepResearchJobs = computed(() =>
  getCurrentConversationDeepResearchJobs(
    deepResearchJobs.activeJobs,
    chatStore.currentConversationId
  )
)

const hasCancelableWork = computed(() =>
  hasCancelableChatWork({
    streaming: chatStore.streaming,
    sending: chatStore.sending,
    toolExecuting: chatStore.toolExecuting,
    isRecovering: chatStore.isRecovering,
    mediaGenerating: mediaGen.generating.value,
    streamUIPhase: chatStore.streamUIState.phase,
    currentConversationTaskCount: currentConversationActiveTasks.value.length,
    currentConversationResearchJobCount: currentConversationDeepResearchJobs.value.length,
  })
)

const streamStatusRailState = computed<StreamUIState>(() => {
  const base = chatStore.streamUIState
  if (base.phase !== 'idle' && base.phase !== 'completed') {
    return base
  }
  if (showAwaitingConfirmation.value) {
    return {
      ...base,
      phase: 'awaiting_confirmation',
      label:
        base.label ||
        chatTextWithFallback(
          'chat.awaitingConfirmation',
          'Waiting for your confirmation to continue'
        ),
      detail: base.detail,
      updatedAt: Date.now(),
    }
  }
  if (chatStore.toolExecuting) {
    return {
      ...base,
      phase: 'executing',
      label:
        base.label ||
        chatStore.statusSummary ||
        chatStore.streamProgress ||
        chatTextWithFallback('chat.streamExecuting', 'Processing'),
      detail: base.detail,
      updatedAt: Date.now(),
    }
  }
  if (mediaGen.generating.value) {
    return {
      ...base,
      phase: 'executing',
      label: base.label || chatTextWithFallback('chat.streamExecuting', 'Generating media'),
      detail: base.detail,
      updatedAt: Date.now(),
    }
  }
  if (chatStore.streaming) {
    return {
      ...base,
      phase: 'streaming',
      label:
        base.label ||
        chatStore.statusSummary ||
        chatStore.streamProgress ||
        chatTextWithFallback('chat.waitingThinking', 'Thinking'),
      detail: base.detail,
      updatedAt: Date.now(),
    }
  }
  if (chatStore.sending) {
    return {
      ...base,
      phase: 'connecting',
      label: base.label || chatTextWithFallback('chat.waitingThinking', 'Thinking'),
      detail: base.detail,
      updatedAt: Date.now(),
    }
  }
  return base
})

const chatInputDisabled = computed(
  () => (chatStore.sending && !chatStore.isPreTTFT) || chatStore.isRecovering
)

const showStreamStatusRail = computed(
  () =>
    streamStatusRailState.value.phase !== 'idle' &&
    streamStatusRailState.value.phase !== 'completed'
)
const streamStatusRailPhaseLabel = computed(() => {
  switch (streamStatusRailState.value.phase) {
    case 'connecting':
      return chatTextWithFallback('chat.streamConnecting', 'Connecting')
    case 'streaming':
      return chatTextWithFallback('chat.streamStreaming', 'Streaming')
    case 'executing':
      return chatTextWithFallback('chat.streamExecuting', 'Working')
    case 'recovering':
      return chatTextWithFallback('chat.streamRecovering', 'Recovering')
    case 'awaiting_confirmation':
      return chatTextWithFallback('chat.streamAwaitingConfirmation', 'Waiting')
    case 'interrupted':
      return chatTextWithFallback('chat.streamInterrupted', 'Interrupted')
    default:
      return streamStatusRailState.value.phase.replace('_', ' ')
  }
})

const messageAreaPaddingClass = computed(() => {
  if (isMobile.value) {
    return hasBackgroundTasks.value ? 'pb-16' : 'pb-6'
  }
  return hasBackgroundTasks.value ? 'pb-12' : 'pb-8'
})

const showExternalStreamDockStatus = computed(() => showStreamStatusRail.value)

const executingConversationIds = computed(() => {
  const ids = new Set<string>()

  const localExecutingIds = Array.isArray(chatStore.executingConversationIds)
    ? chatStore.executingConversationIds
    : []
  for (const id of localExecutingIds) {
    const normalizedId = String(id || '').trim()
    if (normalizedId) ids.add(normalizedId)
  }

  for (const task of taskProjections.currentActiveTasks) {
    const normalizedId = String(task.conversation_id || '').trim()
    if (normalizedId) ids.add(normalizedId)
  }

  for (const task of taskProjections.backgroundTasks) {
    const normalizedId = String(task.conversation_id || '').trim()
    if (normalizedId) ids.add(normalizedId)
  }

  return [...ids]
})

async function openProjectedTask(task: UserTaskProjection) {
  try {
    if (isMobile.value && task.conversation_id) {
      pageStack.value = [task.conversation_id]
    }
    await taskProjections.openTask(task)
  } catch (e) {
    console.error('Failed to open projected task:', e)
  }
}

async function navigateProjectedTask(target: string) {
  const href = String(target || '').trim()
  if (!href) return
  try {
    await router.push(href)
  } catch (e) {
    console.error('Failed to navigate projected task:', e)
  }
}

async function handleOpenDeepResearchJob(job: { job_id?: string; conversation_id?: string }) {
  const jobId = String(job?.job_id || '').trim()
  const conversationId = String(job?.conversation_id || '').trim()
  if (!jobId || !conversationId) return
  try {
    await deepResearchJobs.openJob(jobId, conversationId)
  } catch (e) {
    console.error('Failed to open deep research job:', e)
  }
}

// Virtual scroll threshold - use virtual scroll when message count exceeds this
const VIRTUAL_SCROLL_THRESHOLD = 50

// Whether to use virtual scrolling
const useVirtualScroll = computed(() => chatStore.messages.length > VIRTUAL_SCROLL_THRESHOLD)
const virtualScrollOverscan = computed(() => (isMobile.value ? 3 : 5))

// ID of the last assistant message (for Continue/Regenerate in mobile action sheet)
const lastAssistantMessageId = computed(() => {
  const msgs = chatStore.messages
  for (let i = msgs.length - 1; i >= 0; i--) {
    const msg = msgs[i]
    if (msg?.role === 'assistant') return msg.id
  }
  return null
})

const streamingMessageId = computed(() => {
  if (!chatStore.streaming) return null
  for (let i = chatStore.messages.length - 1; i >= 0; i--) {
    const msg = chatStore.messages[i]
    if (msg?.role === 'assistant' && msg.id.startsWith('streaming-')) return msg.id
  }
  for (let i = chatStore.messages.length - 1; i >= 0; i--) {
    const msg = chatStore.messages[i]
    if (msg?.role === 'assistant') return msg.id
  }
  return null
})

type MessageMemoSource = {
  conversation_id: string
  id: string
  render_key?: string
  content: string
  attachments?: unknown[]
  updated_at?: string
  created_at: string
}

function getMessageRenderKey(message: {
  conversation_id: string
  id: string
  render_key?: string
}) {
  return `${message.conversation_id}:${message.render_key || message.id}`
}

type MessageMemoDepsTuple = [
  id: string,
  content: string,
  attachmentsLength: number,
  updatedAt: string,
  isStreaming: boolean,
  isLastAssistant: boolean,
  disableAutoTTS: boolean,
  isMobile: boolean,
  isMultiSelectMode: boolean,
  isSelected: boolean,
  showExternalStatusRail: boolean,
  preferImmediateStreamingRender: boolean,
  streamState: ActiveMessageStreamState | null,
]

type MessageRenderBindings = {
  isStreaming: boolean
  isLastAssistantMessage: boolean
  disableAutoTTS: boolean
  isMobile: boolean
  isSelected: boolean
  isMultiSelectMode: boolean
  showExternalStatusRail: boolean
  preferImmediateStreamingRender: boolean
  streamState: ActiveMessageStreamState | null
}

type MessageRenderMeta = {
  isStreaming: boolean
  isLastAssistant: boolean
  isSelected: boolean
  memoDeps: MessageMemoDepsTuple
  bindings: MessageRenderBindings
}

function getMessageRenderMetaKey(message: MessageMemoSource): string {
  const cachedKey = messageRenderMetaKeyCache.get(message)
  if (cachedKey) return cachedKey
  const key = getMessageRenderKey(message)
  messageRenderMetaKeyCache.set(message, key)
  return key
}

const messageRenderMetaKeyCache = new WeakMap<MessageMemoSource, string>()
let lastRenderMetaLookupKey = ''
let lastRenderMetaLookupMessage: MessageMemoSource | null = null
let lastRenderMetaLookupValue: MessageRenderMeta | null = null
let lastActiveMessageStreamStateDeps:
  | readonly [
      phase: string,
      awaitingConfirmation: boolean,
      toolExecuting: boolean,
      statusSummary: string | null,
      streamProgress: string | null,
      statusStartedAt: number,
      showExternalStatusRail: boolean,
      toolSandboxAvailable: boolean,
      toolExecutingCommands: readonly string[],
      toolExecutingNames: readonly string[],
      processTrace: readonly unknown[],
      toolResults: readonly unknown[],
    ]
  | null = null
let lastActiveMessageStreamStateValue: ActiveMessageStreamState | null = null

function getActiveMessageStreamState(
  isStreaming: boolean,
  showExternalStatusRail: boolean
): ActiveMessageStreamState | null {
  if (!isStreaming) {
    return null
  }

  const deps = [
    chatStore.streamUIState.phase,
    chatStore.awaitingConfirmation,
    chatStore.toolExecuting,
    chatStore.statusSummary,
    chatStore.streamProgress,
    chatStore.statusStartedAt,
    showExternalStatusRail,
    chatStore.toolSandboxAvailable,
    chatStore.toolExecutingCommands,
    chatStore.toolExecutingNames,
    chatStore.processTrace,
    chatStore.toolResults,
  ] as const

  if (
    lastActiveMessageStreamStateDeps &&
    deps.every((value, index) => value === lastActiveMessageStreamStateDeps?.[index])
  ) {
    return lastActiveMessageStreamStateValue
  }

  const nextState: ActiveMessageStreamState = {
    phase: chatStore.streamUIState.phase,
    awaitingConfirmation: chatStore.awaitingConfirmation,
    toolExecuting: chatStore.toolExecuting,
    toolExecutingCommands: chatStore.toolExecutingCommands,
    toolExecutingNames: chatStore.toolExecutingNames,
    toolSandboxAvailable: chatStore.toolSandboxAvailable,
    statusSummary: chatStore.statusSummary,
    streamProgress: chatStore.streamProgress,
    processTrace: chatStore.processTrace,
    toolResults: chatStore.toolResults,
    statusStartedAt: chatStore.statusStartedAt,
    showExternalStatusRail,
  }

  lastActiveMessageStreamStateDeps = deps
  lastActiveMessageStreamStateValue = nextState
  return nextState
}

function getMessageRenderMeta(message: MessageMemoSource): MessageRenderMeta {
  if (message === lastRenderMetaLookupMessage && lastRenderMetaLookupValue) {
    return lastRenderMetaLookupValue
  }

  const cacheKey = getMessageRenderMetaKey(message)
  if (cacheKey === lastRenderMetaLookupKey && lastRenderMetaLookupValue) {
    lastRenderMetaLookupMessage = message
    return lastRenderMetaLookupValue
  }

  const isStreaming = message.id === streamingMessageId.value
  const memoDepsUpdatedAt = message.updated_at ?? message.created_at
  const isLastAssistant = message.id === lastAssistantMessageId.value
  const isSelected = chatStore.isMultiSelectMode
    ? chatStore.selectedMessageIds.has(message.id)
    : false
  const showExternalStatusRail =
    showExternalStreamDockStatus.value && isStreaming && message.id === streamingMessageId.value
  const streamState = getActiveMessageStreamState(isStreaming, showExternalStatusRail)

  const cached = messageRenderMetaCache.get(cacheKey)
  if (
    cached &&
    cached.memoDeps[1] === message.content &&
    cached.memoDeps[2] === (message.attachments?.length ?? 0) &&
    cached.memoDeps[3] === memoDepsUpdatedAt &&
    cached.memoDeps[4] === isStreaming &&
    cached.memoDeps[5] === isLastAssistant &&
    cached.memoDeps[6] === showTalkMode.value &&
    cached.memoDeps[7] === isMobile.value &&
    cached.memoDeps[8] === chatStore.isMultiSelectMode &&
    cached.memoDeps[9] === isSelected &&
    cached.memoDeps[10] === showExternalStatusRail &&
    cached.memoDeps[11] === useVirtualScroll.value &&
    cached.memoDeps[12] === streamState
  ) {
    lastRenderMetaLookupKey = cacheKey
    lastRenderMetaLookupMessage = message
    lastRenderMetaLookupValue = cached
    return cached
  }

  const deps: MessageMemoDepsTuple = [
    message.id,
    message.content,
    message.attachments?.length ?? 0,
    memoDepsUpdatedAt,
    isStreaming,
    isLastAssistant,
    showTalkMode.value,
    isMobile.value,
    chatStore.isMultiSelectMode,
    isSelected,
    showExternalStatusRail,
    useVirtualScroll.value,
    streamState,
  ]

  const bindings: MessageRenderBindings = {
    isStreaming,
    isLastAssistantMessage: isLastAssistant,
    disableAutoTTS: showTalkMode.value,
    isMobile: isMobile.value,
    isSelected,
    isMultiSelectMode: chatStore.isMultiSelectMode,
    showExternalStatusRail,
    preferImmediateStreamingRender: useVirtualScroll.value,
    streamState,
  }

  const nextMeta: MessageRenderMeta = {
    isStreaming,
    isLastAssistant,
    isSelected,
    memoDeps: deps,
    bindings,
  }

  if (
    messageRenderMetaCache.size >= MESSAGE_RENDER_META_CACHE_MAX &&
    !messageRenderMetaCache.has(cacheKey)
  ) {
    messageRenderMetaCache.clear()
  }
  messageRenderMetaCache.set(cacheKey, nextMeta)
  lastRenderMetaLookupKey = cacheKey
  lastRenderMetaLookupMessage = message
  lastRenderMetaLookupValue = nextMeta
  return nextMeta
}

function messageMemoDeps(message: MessageMemoSource) {
  return getMessageRenderMeta(message).memoDeps
}

function messageRenderBindings(message: MessageMemoSource) {
  return getMessageRenderMeta(message).bindings
}

const messageRenderMetaCache = new Map<string, MessageRenderMeta>()
const MESSAGE_RENDER_META_CACHE_MAX = 1500

// Context menu state
const showContextMenu = ref(false)
const contextMenuPosition = ref({ x: 0, y: 0 })
const contextMenuMessageId = ref<string | null>(null)
const contextMenuSelectedText = ref('')

// Provider config dialog state
const showProviderConfigDialog = ref(false)
type ProviderAttentionMode = 'unconfigured' | 'unavailable' | 'recoverable'
const providerConfigDialogMode = ref<ProviderAttentionMode>('unconfigured')
const providerConfigDialogHasDraft = ref(false)

function providerAppearsAvailable(status?: string) {
  return status !== 'error' && status !== 'inactive'
}

function providerStatusIsPending(status?: string) {
  return status !== 'active' && status !== 'inactive' && status !== 'error'
}

function normalizeProviderIssue(error?: string) {
  return String(error || '').trim()
}

function isRecoverableProviderIssue(error?: string) {
  const normalized = normalizeProviderIssue(error)
  if (!normalized) return false
  if (normalized.startsWith('auth_error:')) return false
  if (normalized === 'certificate_error') return false
  if (normalized === 'endpoint_not_found') return false
  if (normalized.startsWith('base_url_not_configured')) return false
  return true
}

function chatTextWithFallback(key: string, fallback: string) {
  return te(key) ? t(key) : fallback
}

function chatTextWithNamedFallback(
  key: string,
  fallback: string,
  named: Record<string, string | number>
) {
  return te(key) ? String(t(key, named)) : fallback
}

type EdgeQuickNavItem = {
  fullText: string
  messageId: string
  messageIndex: number
  preview: string
}

const EDGE_QUICK_NAV_MIN_ITEMS = 4
const EDGE_QUICK_NAV_PREVIEW_LIMIT = 34
const activeEdgeQuickNavMessageId = ref('')
const visibleMessageRangeStart = ref(0)

function normalizeEdgeQuickNavText(content: string): string {
  return content
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[[^\]]*\]\([^)]+\)/g, ' ')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')
    .replace(/^\s{0,3}>\s?/gm, '')
    .replace(/^\s{0,3}(?:[-*+]|\d+\.)\s+/gm, '')
    .replace(/\s+/g, ' ')
    .trim()
}

function truncateEdgeQuickNavText(text: string): string {
  if (text.length <= EDGE_QUICK_NAV_PREVIEW_LIMIT) return text
  return `${text.slice(0, EDGE_QUICK_NAV_PREVIEW_LIMIT).trimEnd()}…`
}

const edgeQuickNavItems = computed<EdgeQuickNavItem[]>(() => {
  const items: EdgeQuickNavItem[] = []

  for (let index = 0; index < chatStore.messages.length; index += 1) {
    const message = chatStore.messages[index] as ChatMessageRecord
    if (message.role !== 'user') continue

    const fullText = normalizeEdgeQuickNavText(String(message.content || ''))
    if (!fullText) continue

    items.push({
      fullText,
      messageId: message.id,
      messageIndex: index,
      preview: truncateEdgeQuickNavText(fullText),
    })
  }

  return items
})

const edgeQuickNavSignature = computed(() =>
  edgeQuickNavItems.value.map((item) => item.messageId).join('|')
)

const showEdgeQuickNav = computed(
  () => !isMobile.value && edgeQuickNavItems.value.length >= EDGE_QUICK_NAV_MIN_ITEMS
)

const {
  pendingTaskActionDialog,
  taskActionDialogError,
  taskActionDialogSubmitting,
  closeTaskActionDialog,
  performProjectedTaskAction,
  confirmTaskActionDialog,
} = useTaskProjectionActions({
  translate: chatTextWithFallback,
})

function loadActiveTodoPanelCollapsed(): boolean {
  try {
    return localStorage.getItem(ACTIVE_TODO_PANEL_COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}

function persistActiveTodoPanelCollapsed(value: boolean) {
  try {
    if (value) {
      localStorage.setItem(ACTIVE_TODO_PANEL_COLLAPSED_KEY, '1')
      return
    }
    localStorage.removeItem(ACTIVE_TODO_PANEL_COLLAPSED_KEY)
  } catch {
    // Ignore storage errors
  }
}

function getChatMessageElementId(messageId: string): string {
  return `chat-message-${encodeURIComponent(messageId.trim())}`
}

function providerStatusLabel(status: Provider['status']) {
  if (status === 'active') return chatTextWithFallback('chat.noProvider.statusActive', 'Active')
  if (status === 'inactive')
    return chatTextWithFallback('chat.noProvider.statusInactive', 'Inactive')
  return chatTextWithFallback('chat.noProvider.statusError', 'Error')
}

function summarizeProviderIssue(error?: string, status?: Provider['status']) {
  const normalized = normalizeProviderIssue(error)
  if (!normalized) {
    if (status === 'inactive') {
      return chatTextWithFallback(
        'chat.noProvider.issueInactive',
        'Check whether the API key, OAuth account, or allowed models are ready.'
      )
    }
    return chatTextWithFallback(
      'chat.noProvider.issueGeneric',
      'Open provider settings to review the latest health status.'
    )
  }

  if (normalized.startsWith('auth_error:')) {
    return chatTextWithFallback(
      'chat.noProvider.issueAuth',
      'Authentication failed. Recheck the API key or OAuth connection.'
    )
  }
  if (normalized === 'network_error' || normalized === 'connection_error') {
    return chatTextWithFallback(
      'chat.noProvider.issueNetwork',
      'Network connection failed. Verify the endpoint and local network.'
    )
  }
  if (normalized === 'timeout_error') {
    return chatTextWithFallback(
      'chat.noProvider.issueTimeout',
      'The provider timed out. Try again or switch to another route.'
    )
  }
  if (normalized === 'certificate_error') {
    return chatTextWithFallback(
      'chat.noProvider.issueCertificate',
      'TLS certificate verification failed for this provider.'
    )
  }
  if (normalized === 'endpoint_not_found' || normalized.startsWith('base_url_not_configured')) {
    return chatTextWithFallback(
      'chat.noProvider.issueEndpoint',
      'The provider endpoint is not configured correctly.'
    )
  }
  if (normalized.startsWith('unexpected_status:')) {
    const code = normalized.split(':')[1] || ''
    return chatTextWithNamedFallback(
      'chat.noProvider.issueUnexpectedStatus',
      `The provider returned an unexpected status${code ? ` (${code})` : ''}.`,
      {
        code: code ? `(${code})` : '',
      }
    )
  }
  return normalized
}

function buildProviderGuidanceCopy(mode: ProviderAttentionMode) {
  if (mode === 'recoverable') {
    return {
      eyebrow: chatTextWithFallback(
        'chat.noProvider.recoverableEyebrow',
        'Temporary provider issue'
      ),
      title: chatTextWithFallback('chat.noProvider.recoverableTitle', 'Provider needs attention'),
      description: chatTextWithFallback(
        'chat.noProvider.recoverableDescription',
        'Your configured provider hit a temporary error. Review the latest reason below, reset the provider, and then try again.'
      ),
      primaryAction: chatTextWithFallback('chat.noProvider.retry', 'Retry Provider'),
      secondaryAction: chatTextWithFallback('chat.noProvider.reviewSingle', 'Review Provider'),
      iconWrapperClass: 'bg-amber-100 dark:bg-amber-900/30',
      iconClass: 'text-amber-600 dark:text-amber-400',
    }
  }

  if (mode === 'unavailable') {
    return {
      eyebrow: chatTextWithFallback(
        'chat.noProvider.unavailableEyebrow',
        'Temporarily unavailable'
      ),
      title: chatTextWithFallback(
        'chat.noProvider.unavailableTitle',
        'No AI provider is available right now'
      ),
      description: chatTextWithFallback(
        'chat.noProvider.unavailableDescription',
        'Your configured providers are currently unavailable. Check their connection, API key, or model status in Settings and then try again.'
      ),
      primaryAction: chatTextWithFallback('chat.noProvider.review', 'Review Providers'),
      secondaryAction: chatTextWithFallback('chat.noProvider.dismiss', 'Not now'),
      iconWrapperClass: 'bg-amber-100 dark:bg-amber-900/30',
      iconClass: 'text-amber-600 dark:text-amber-400',
    }
  }

  return {
    eyebrow: chatTextWithFallback('chat.noProvider.unconfiguredEyebrow', 'One quick step'),
    title: chatTextWithFallback(
      'chat.noProvider.unconfiguredTitle',
      'Set up an AI provider to start chatting'
    ),
    description: chatTextWithFallback(
      'chat.noProvider.unconfiguredDescription',
      'You do not have an available LLM provider yet. Add one in Settings and you can continue right where you left off.'
    ),
    primaryAction: chatTextWithFallback('chat.noProvider.configure', 'Configure Provider'),
    secondaryAction: chatTextWithFallback('chat.noProvider.dismiss', 'Not now'),
    iconWrapperClass: 'bg-sky-100 dark:bg-sky-900/30',
    iconClass: 'text-sky-600 dark:text-sky-300',
  }
}

const providerConfigDialogCopy = computed(() =>
  buildProviderGuidanceCopy(providerConfigDialogMode.value)
)
const modelAutoFallbackDialogState = computed(() => {
  const pending = chatStore.pendingModelAutoFallback
  if (!pending) return null
  if (pending.conversationId !== (chatStore.currentConversationId || '')) return null
  return pending
})
const modelAutoFallbackTargetLabel = computed(() => {
  const pending = modelAutoFallbackDialogState.value
  if (!pending) return ''
  if (pending.requestedProviderId) {
    return `${pending.requestedProviderId}/${pending.requestedModelId}`
  }
  return pending.requestedModelId
})
const activeTodoPanelCollapsed = ref(loadActiveTodoPanelCollapsed())
const focusedTodoMessageId = ref<string | null>(null)
let focusedTodoMessageTimer: ReturnType<typeof setTimeout> | null = null
type TodoAwareToolResult = {
  name?: unknown
  icon?: unknown
}

type TodoAwareMessage = ChatMessageRecord & {
  local_process_tool_results?: TodoAwareToolResult[]
}

const TODO_COMPLETION_WRITE_TOOL_NAMES = new Set([
  'apply_patch',
  'append',
  'edit',
  'file_write',
  'write_commit',
])

function normalizeTodoSignalText(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function isTodoChecklistMessageContent(content?: string): boolean {
  return /(^|\n)[ \t]*[-*]\s+\[(?: |x|X)\]\s+/.test(content || '')
}

function matchesTodoCompletionSignal(
  message: TodoAwareMessage,
  completionSignal: { messageId?: string; todoCardId?: string } | null | undefined
): boolean {
  if (!completionSignal) return false
  const normalizedMessageId = normalizeTodoSignalText(completionSignal.messageId)
  const normalizedTodoCardId = normalizeTodoSignalText(completionSignal.todoCardId)
  if (!normalizedMessageId && !normalizedTodoCardId) return false

  if (
    normalizedTodoCardId &&
    normalizeTodoSignalText(message.todo_card_id) === normalizedTodoCardId
  ) {
    return true
  }

  if (!normalizedMessageId) return false
  return (
    normalizedMessageId === normalizeTodoSignalText(message.id) ||
    normalizedMessageId === normalizeTodoSignalText(message.render_key)
  )
}

function isSuccessfulTodoArtifactWriteResult(
  result: TodoAwareToolResult | null | undefined
): boolean {
  if (!result) return false
  const name = normalizeTodoSignalText(result.name).toLowerCase()
  const icon = normalizeTodoSignalText(result.icon)
  return icon === '✓' && TODO_COMPLETION_WRITE_TOOL_NAMES.has(name)
}

function hasTodoCompletionArtifactWriteSuccess(
  messages: TodoAwareMessage[],
  completionSignal: { messageId?: string; todoCardId?: string } | null | undefined
): boolean {
  if (!completionSignal) return false
  return messages.some((message) => {
    if (message.role !== 'assistant') return false
    if (!matchesTodoCompletionSignal(message, completionSignal)) return false
    if (!Array.isArray(message.local_process_tool_results)) return false
    return message.local_process_tool_results.some((result) =>
      isSuccessfulTodoArtifactWriteResult(result)
    )
  })
}

const todoCompletionArtifactWriteSucceeded = computed(() =>
  hasTodoCompletionArtifactWriteSuccess(
    chatStore.messages as TodoAwareMessage[],
    chatStore.recentTodoCompletion
  )
)
const shouldDeferTodoFinalization = computed(
  () => showStreamStatusRail.value && !todoCompletionArtifactWriteSucceeded.value
)
const activeTodoMessages = computed(() => {
  if (!shouldDeferTodoFinalization.value) return chatStore.messages
  return chatStore.messages.filter((message) => {
    if (message.role !== 'assistant') return true
    const messageId = normalizeTodoSignalText(message.id)
    if (!messageId.startsWith('streaming-')) return true
    return isTodoChecklistMessageContent(message.content)
  })
})
const activeTodoCompletionSignal = computed(() =>
  shouldDeferTodoFinalization.value ? null : chatStore.recentTodoCompletion
)
const activeTodoSummary = computed(() =>
  findLatestTodoChecklistSummary(activeTodoMessages.value, activeTodoCompletionSignal.value, {
    allowCompletedChecklist: shouldDeferTodoFinalization.value,
  })
)
const activeTodoProgressText = computed(() => {
  const summary = activeTodoSummary.value
  if (!summary) return ''

  return chatTextWithNamedFallback(
    'chat.activeTodo.progress',
    `${summary.completedCount} out of ${summary.totalCount} tasks completed`,
    {
      completed: summary.completedCount,
      total: summary.totalCount,
    }
  )
})
const activeTodoJumpMessageId = computed(
  () => activeTodoSummary.value?.focusMessageId || activeTodoSummary.value?.messageId || ''
)
const activeTodoPanelToggleTitle = computed(() =>
  activeTodoPanelCollapsed.value
    ? chatTextWithFallback('chat.activeTodo.expand', 'Expand todo list')
    : chatTextWithFallback('chat.activeTodo.collapse', 'Collapse todo list')
)
const activeTodoPanelJumpTitle = computed(() =>
  chatTextWithFallback('chat.activeTodo.jumpToMessage', 'Jump to checklist message')
)

// Provider status computed properties
const enabledLlmProviders = computed(() =>
  providerPoolStore.enabledProviders.filter((provider) => provider.type !== 'media')
)
const activeLlmProviders = computed(() =>
  enabledLlmProviders.value.filter((provider) => provider.status === 'active')
)
const hasProvisionallyAvailableProviders = computed(() =>
  enabledLlmProviders.value.some((provider) => providerAppearsAvailable(provider.status))
)
const recoverableAttentionProvider = computed<Provider | null>(() => {
  if (enabledLlmProviders.value.length !== 1) return null
  const provider = enabledLlmProviders.value[0]
  if (!provider) return null
  if (provider.status !== 'error') return null
  return isRecoverableProviderIssue(provider.last_error) ? provider : null
})
const hasConfiguredProviders = computed(() => enabledLlmProviders.value.length > 0)
const hasActiveProviders = computed(() => activeLlmProviders.value.length > 0)
const hasPendingProviderStatus = computed(() =>
  enabledLlmProviders.value.some((provider) => providerStatusIsPending(provider.status))
)
const allProvidersFailed = computed(
  () =>
    hasConfiguredProviders.value &&
    !hasActiveProviders.value &&
    enabledLlmProviders.value.every((provider) => provider.status === 'error')
)

// Provider status indicator
const providerStatus = computed(() => {
  if (!hasConfiguredProviders.value) {
    return { status: 'none', color: 'gray', message: t('chat.noProviderConfigured') }
  }
  if (allProvidersFailed.value) {
    return { status: 'error', color: 'red', message: t('chat.allProvidersFailed') }
  }
  if (hasActiveProviders.value) {
    return { status: 'active', color: 'green', message: t('chat.providerActive') }
  }
  if (hasPendingProviderStatus.value) {
    return { status: 'pending', color: 'yellow', message: t('chat.providerPending') }
  }
  return {
    status: 'error',
    color: 'red',
    message: chatTextWithFallback(
      'chat.providerNeedsAttention',
      'Provider needs attention. Click to check settings.'
    ),
  }
})

const providerAttentionMode = computed<ProviderAttentionMode>(() => {
  if (recoverableAttentionProvider.value) {
    return 'recoverable'
  }
  return hasConfiguredProviders.value ? 'unavailable' : 'unconfigured'
})
const needsProviderAttention = computed(() => !hasProvisionallyAvailableProviders.value)
const providerInlineGuidanceCopy = computed(() =>
  buildProviderGuidanceCopy(providerAttentionMode.value)
)
const providerAttentionItems = computed(() =>
  enabledLlmProviders.value.slice(0, 3).map((provider) => ({
    id: provider.id,
    name: providerPoolStore.getProviderDisplayName(provider.id),
    statusLabel: providerStatusLabel(provider.status),
    status: provider.status,
    reason: summarizeProviderIssue(provider.last_error, provider.status),
  }))
)
const providerAttentionOverflowCount = computed(() =>
  Math.max(0, enabledLlmProviders.value.length - providerAttentionItems.value.length)
)

const showAwaitingConfirmation = computed(
  () =>
    chatStore.awaitingConfirmation ||
    !!chatStore.pendingQuestion ||
    !!chatStore.pendingApproval ||
    !!chatStore.pendingExecApproval
)

// Routing mode display info
const routingModeInfo = computed(() => {
  const mode = providerPoolStore.routingMode
  const cloudCount = providerPoolStore.cloudProviders.filter((p) => p.status === 'active').length
  const localCount = providerPoolStore.localProviders.filter((p) => p.status === 'active').length

  if (mode === 'cloud') {
    return {
      icon: 'cloud',
      label: t('chat.routingMode.cloud'),
      count: cloudCount,
      color: 'gray',
    }
  } else if (mode === 'local') {
    return {
      icon: 'local',
      label: t('chat.routingMode.local'),
      count: localCount,
      color: 'green',
    }
  } else {
    return {
      icon: 'auto',
      label: t('chat.routingMode.auto'),
      count: cloudCount + localCount,
      color: 'accent',
    }
  }
})

const cloudActiveCount = computed(
  () => providerPoolStore.cloudProviders.filter((p) => p.status === 'active').length
)
const localActiveCount = computed(
  () => providerPoolStore.localProviders.filter((p) => p.status === 'active').length
)
const enabledChatProviders = computed(() =>
  providerPoolStore.enabledProviders.filter((provider) => provider.type !== 'media')
)
const enabledChatProviderIds = computed(
  () => new Set(enabledChatProviders.value.map((provider) => provider.id))
)
const totalActiveProviderCount = computed(() => cloudActiveCount.value + localActiveCount.value)
const isSingleModelMode = computed(() => chatStore.modelPreference !== 'auto')
const providerScopedAutoProviderLabel = computed(() => {
  const providerId = chatStore.selectedProviderId?.trim() || ''
  if (
    !chatStore.providerPinOnlyActive ||
    !providerId ||
    !enabledChatProviderIds.value.has(providerId)
  ) {
    return ''
  }
  return providerPoolStore.getProviderDisplayName(providerId)
})
const providerScopedAutoStrategyLabel = computed(() => {
  if (!providerScopedAutoProviderLabel.value) return ''
  return `${t('chat.routingMode.auto')} · ${providerScopedAutoProviderLabel.value}`
})
const showProviderScopedAutoNotice = computed(() => providerScopedAutoProviderLabel.value !== '')
const providerScopedAutoNoticeTitle = computed(() =>
  chatTextWithFallback('chat.routingMode.providerPinnedTitle', 'Pinned Provider')
)
const providerScopedAutoNoticeDescription = computed(() => {
  if (!providerScopedAutoProviderLabel.value) return ''
  return chatTextWithNamedFallback(
    'chat.routingMode.providerPinnedDesc',
    `This conversation stays on ${providerScopedAutoProviderLabel.value}, but the model is still automatic.`,
    {
      provider: providerScopedAutoProviderLabel.value,
    }
  )
})
const providerScopedAutoResetLabel = computed(() =>
  chatTextWithFallback('chat.routingMode.providerPinnedReset', 'Use all providers')
)
const providerScopedAutoDescription = computed(() => {
  if (!providerScopedAutoProviderLabel.value) return t('chat.routingMode.modelAuto')
  return `${t('chat.routingMode.modelAuto')} · ${providerScopedAutoProviderLabel.value}`
})
const providerScopedAutoOptions = computed(() =>
  enabledChatProviders.value.map((provider) => ({
    id: provider.id,
    label: providerPoolStore.getProviderDisplayName(provider.id),
  }))
)
const showProviderScopedAutoOptions = computed(
  () => !isSingleModelMode.value && providerScopedAutoOptions.value.length > 1
)

const fixedModelOptions = computed(() => {
  const enabledProviderIds = new Set(
    providerPoolStore.enabledProviders
      .filter((provider) => provider.type !== 'media')
      .map((provider) => provider.id)
  )

  const availableModels = providerPoolStore.models.filter(
    (model) => model.enabled && enabledProviderIds.has(model.provider_id)
  )
  const duplicateCount = new Map<string, number>()
  for (const model of availableModels) {
    duplicateCount.set(model.id, (duplicateCount.get(model.id) || 0) + 1)
  }

  return availableModels.map((model) => ({
    id: model.id,
    providerId: model.provider_id,
    value: `${model.provider_id}/${model.id}`,
    label:
      (duplicateCount.get(model.id) || 0) > 1
        ? `${providerPoolStore.getProviderDisplayName(model.provider_id)} · ${model.display_name || model.id}`
        : model.display_name || model.id,
  }))
})

const showRoutingControl = computed(() => enabledChatProviders.value.length > 0)
const showLocationRoutingOptions = computed(
  () =>
    enabledChatProviders.value.length > 1 &&
    providerPoolStore.hasCloudProviders &&
    providerPoolStore.hasLocalProviders
)

const fixedModelLabel = computed(() => {
  if (providerScopedAutoProviderLabel.value) return providerScopedAutoProviderLabel.value
  if (chatStore.modelPreference === 'auto') return t('chat.routingMode.highAvailability')
  return (
    chatStore.splitModelPreference(chatStore.modelPreference).selected_model_id ||
    chatStore.modelPreference
  )
})

const routingButtonTitle = computed(
  () => `${t('chat.routingMode.title')} · ${routingModeInfo.value.label} · ${fixedModelLabel.value}`
)

const routingMenuStatusMessage = computed(() =>
  needsProviderAttention.value
    ? chatTextWithFallback(
        'chat.providerNeedsAttention',
        'Provider needs attention. Click to check settings.'
      )
    : providerStatus.value.message
)

const routingStrategyLabel = computed(() =>
  isSingleModelMode.value
    ? t('chat.routingMode.fixedModel')
    : providerScopedAutoStrategyLabel.value || t('chat.routingMode.highAvailability')
)

const deepResearchTitle = computed(() =>
  chatTextWithFallback('ui.deepResearchTitle', 'Deep Research')
)
const smartResumeStateLabel = computed(() =>
  settingsStore.agentMode
    ? chatTextWithFallback('common.enabled', 'Enabled')
    : chatTextWithFallback('common.disabled', 'Disabled')
)
const smartResumeAutoConfirmStateLabel = computed(() =>
  settingsStore.agentAutoConfirm
    ? chatTextWithFallback('common.enabled', 'Enabled')
    : chatTextWithFallback('common.disabled', 'Disabled')
)
const mobileFeatureSheetMeta = computed(() => {
  if (activeMobileFeatureSheet.value === 'deep-research') {
    return {
      kind: 'deep-research' as const,
      state: chatTextWithFallback('chat.alwaysOn', 'Automatic'),
      title: deepResearchTitle.value,
      description: chatTextWithFallback(
        'chat.deepResearchHoverDescription',
        'Launch a structured research workflow with retrieval, verification, traceable runs, and linked run details.'
      ),
      tags: [
        chatTextWithFallback('chat.deepResearchStageRetrieve', 'Retrieve'),
        chatTextWithFallback('chat.deepResearchCitations', 'Citations'),
        chatTextWithFallback('chat.deepResearchStageVerify', 'Verify'),
      ],
      primaryActionLabel: null,
    }
  }
  if (activeMobileFeatureSheet.value === 'smart-resume') {
    return {
      kind: 'smart-resume' as const,
      state: smartResumeStateLabel.value,
      title: t('chat.taskLoop'),
      description: chatTextWithFallback(
        'chat.ralphLoopHoverDescription',
        "Let the agent plan, use tools, apply changes, and keep iterating until the task lands cleanly. Inspired by Ralph's relentless persistence."
      ),
      tags: [
        chatTextWithFallback('chat.ralphLoopHoverPlan', 'Plan'),
        chatTextWithFallback('chat.ralphLoopHoverAct', 'Act'),
        chatTextWithFallback('chat.ralphLoopHoverCheck', 'Check'),
        `${t('agent.autoConfirm')}: ${smartResumeAutoConfirmStateLabel.value}`,
      ],
      primaryActionLabel: settingsStore.agentMode
        ? chatTextWithFallback(
            'chat.mobileFeatureDisableSmartResume',
            `Disable ${t('chat.taskLoop')}`
          )
        : chatTextWithFallback(
            'chat.mobileFeatureEnableSmartResume',
            `Enable ${t('chat.taskLoop')}`
          ),
    }
  }
  return null
})

function getMessageHeightKey(message: {
  conversation_id: string
  id: string
  render_key?: string
}) {
  return getMessageRenderKey(message)
}

function getVirtualResizeObserver() {
  if (virtualResizeObserver) return virtualResizeObserver
  virtualResizeObserver = new ResizeObserver((entries) => {
    for (const entry of entries) {
      const target = entry.target
      if (!(target instanceof HTMLElement)) continue
      const key = virtualElementToMessageKey.get(target)
      if (!key) continue
      const next = Math.ceil(entry.contentRect.height)
      if (next <= 0) continue
      if (messageHeightCache.get(key) !== next) {
        messageHeightCache.set(key, next)
      }
      virtualItemHeightUpdaters.get(key)?.(next)
    }
  })
  return virtualResizeObserver
}

function clearVirtualItemObservers() {
  if (virtualResizeObserver) {
    virtualResizeObserver.disconnect()
    virtualResizeObserver = null
  }
  virtualElementToMessageKey.clear()
  virtualItemElements.clear()
  virtualItemHeightUpdaters.clear()
  virtualItemRefCallbacks.clear()
}

function bindVirtualItemHeight(
  message: { conversation_id: string; id: string },
  updateHeight: (height: number) => void
) {
  const key = getMessageHeightKey(message)
  virtualItemHeightUpdaters.set(key, updateHeight)

  const existingCallback = virtualItemRefCallbacks.get(key)
  if (existingCallback) return existingCallback

  const callback = (el: Element | ComponentPublicInstance | null) => {
    const existingElement = virtualItemElements.get(key)
    if (!el) {
      if (existingElement) {
        virtualResizeObserver?.unobserve(existingElement)
        virtualElementToMessageKey.delete(existingElement)
        virtualItemElements.delete(key)
      }
      virtualItemHeightUpdaters.delete(key)
      return
    }

    const maybeElement = (el as ComponentPublicInstance)?.$el ?? el
    if (!(maybeElement instanceof HTMLElement)) return
    const target = maybeElement
    const cached = messageHeightCache.get(key)
    if (cached && cached > 0) {
      virtualItemHeightUpdaters.get(key)?.(cached)
    }

    // Keep existing observer when ref callback is re-run for the same DOM node.
    // This avoids unobserve/observe churn during parent re-renders.
    if (existingElement === target) {
      return
    }

    if (existingElement) {
      virtualResizeObserver?.unobserve(existingElement)
      virtualElementToMessageKey.delete(existingElement)
    }

    const previousKey = virtualElementToMessageKey.get(target)
    if (previousKey && previousKey !== key) {
      virtualItemElements.delete(previousKey)
    }

    virtualItemElements.set(key, target)
    virtualElementToMessageKey.set(target, key)

    const syncHeight = () => {
      const next = Math.ceil(target.getBoundingClientRect().height)
      if (next <= 0) return
      if (messageHeightCache.get(key) !== next) {
        messageHeightCache.set(key, next)
      }
      virtualItemHeightUpdaters.get(key)?.(next)
    }

    // Initial sync after mount
    syncHeight()

    getVirtualResizeObserver().observe(target)
  }

  virtualItemRefCallbacks.set(key, callback)
  return callback
}

// Check if mobile device (UA detection)
function checkMobile() {
  isMobile.value = _initMobile
  isNarrowScreen.value = window.innerWidth < 768
  // Auto-show sidebar on desktop wide screen, hide on narrow
  if (!isMobile.value) {
    showSidebar.value = !isNarrowScreen.value
  }
  if (isMobile.value && showRoutingMenu.value) {
    showRoutingMenu.value = false
    return
  }
  if (showRoutingMenu.value) {
    scheduleRoutingMenuReposition()
  }
}

const checkMobileOnResize = rafThrottle(checkMobile)

// Keyboard shortcuts
useChatShortcuts({
  onNewChat: () => handleCreateConversation(),
  onFocusInput: () => chatInputRef.value?.focus?.(),
  onToggleSidebar: () => toggleSidebar(),
  onCancelStream: () => hasCancelableWork.value && handleCancel(),
})

// Scroll to bottom when messages change
let autoScrollRafId: number | null = null
function scheduleFollowScrollToBottom(behavior: 'auto' | 'smooth' = 'auto') {
  if (!isUserNearBottom.value) return
  if (autoScrollRafId !== null) return
  recordChatPerfCount('chat_view.follow_scroll.scheduled')
  autoScrollRafId = window.requestAnimationFrame(async () => {
    autoScrollRafId = null
    await nextTick()
    if (isUserNearBottom.value) {
      recordChatPerfCount('chat_view.follow_scroll.executed')
      scrollToBottom(behavior)
    }
  })
}

watch(
  () => chatStore.messages.length,
  () => {
    scheduleFollowScrollToBottom('auto')
  }
)

// Also scroll when streaming content updates
watch(
  () => chatStore.streamingContent,
  () => {
    scheduleFollowScrollToBottom('auto')
  }
)

// Close sidebar when selecting conversation on mobile
// Also clear incremental parse states for the previous conversation
watch(
  () => chatStore.currentConversationId,
  async (newId, oldId) => {
    messageRenderMetaCache.clear()
    lastRenderMetaLookupKey = ''
    lastRenderMetaLookupMessage = null
    lastRenderMetaLookupValue = null
    lastActiveMessageStreamStateDeps = null
    lastActiveMessageStreamStateValue = null
    clearVirtualItemObservers()
    if (isMobile.value) {
      showSidebar.value = false
    }
    // Clear incremental parse states for the old conversation to free memory
    if (oldId && oldId !== newId) {
      clearConversationIncrementalStates(oldId)
    }
    // Scroll to bottom when entering a conversation
    if (newId) {
      isUserNearBottom.value = true
      await nextTick()
      scrollToBottom()
    }
    void taskProjections.setConversation(newId || '').catch(() => {})
  }
)

// Watch for trial quota token changes and trigger animation
watch(
  () => providerPoolStore.trialQuota?.tokens_remaining,
  (newTokens, oldTokens) => {
    if (newTokens !== undefined && oldTokens !== undefined && newTokens !== oldTokens) {
      // Trigger animation when tokens change
      tokenAnimating.value = true
      previousTokens.value = oldTokens
      setTimeout(() => {
        tokenAnimating.value = false
        previousTokens.value = null
      }, 600)
    }
  }
)

// Track whether user is near the bottom of the chat (for auto-scroll during streaming)
const isUserNearBottom = ref(true)
const NEAR_BOTTOM_THRESHOLD = 80 // px from bottom to consider "at bottom"
let normalScrollRafId: number | null = null
let normalLoadMoreInFlight = false
const scheduleEdgeQuickNavSync = rafThrottle(() => {
  syncEdgeQuickNavActiveMessage()
})

function checkIfNearBottom() {
  return measureChatPerf('chat_view.check_if_near_bottom', () => {
    recordChatPerfCount('chat_view.check_if_near_bottom.calls')
    if (useVirtualScroll.value && virtualScrollRef.value) {
      // For virtual scroll, use exposed scroll container directly.
      const container = virtualScrollRef.value.getContainer?.()
      if (container) {
        const { scrollTop, scrollHeight, clientHeight } = container
        return scrollHeight - scrollTop - clientHeight < NEAR_BOTTOM_THRESHOLD
      }
      return true
    } else if (messagesContainer.value) {
      const { scrollTop, scrollHeight, clientHeight } = messagesContainer.value
      return scrollHeight - scrollTop - clientHeight < NEAR_BOTTOM_THRESHOLD
    }
    return true
  })
}

function scrollToBottom(behavior: 'auto' | 'smooth' = 'auto') {
  if (useVirtualScroll.value && virtualScrollRef.value) {
    virtualScrollRef.value.scrollToBottom(behavior)
  } else if (messagesContainer.value) {
    messagesContainer.value.scrollTo({
      top: messagesContainer.value.scrollHeight,
      behavior,
    })
  }
  isUserNearBottom.value = true
}

// Handle scroll for loading more messages
function handleScroll() {
  if (normalScrollRafId !== null) return
  normalScrollRafId = window.requestAnimationFrame(() => {
    normalScrollRafId = null

    // Update near-bottom tracking
    isUserNearBottom.value = checkIfNearBottom()
    scheduleEdgeQuickNavSync()

    // Skip for virtual scroll - it handles its own scrolling
    if (useVirtualScroll.value) return
    if (!messagesContainer.value) return

    // Load more when scrolled near the top
    if (
      messagesContainer.value.scrollTop < 100 &&
      chatStore.hasMoreMessages &&
      !chatStore.loadingMore &&
      !normalLoadMoreInFlight
    ) {
      normalLoadMoreInFlight = true
      const previousHeight = messagesContainer.value.scrollHeight
      chatStore
        .loadMoreMessages()
        .then(() => {
          // Maintain scroll position after loading more
          nextTick(() => {
            if (messagesContainer.value) {
              const newHeight = messagesContainer.value.scrollHeight
              messagesContainer.value.scrollTop = newHeight - previousHeight
            }
            isUserNearBottom.value = checkIfNearBottom()
          })
        })
        .finally(() => {
          normalLoadMoreInFlight = false
        })
    }
  })
}

// Handle virtual scroll visible range change
const VIRTUAL_LOAD_MORE_COOLDOWN_MS = 250
let virtualLoadMoreInFlight = false
let virtualLoadMoreLastAt = 0
let virtualLoadMoreRetryTimer: ReturnType<typeof setTimeout> | null = null

function handleVisibleRangeChange(start: number, _end: number) {
  visibleMessageRangeStart.value = start
  scheduleEdgeQuickNavSync()

  // Keep near-bottom state in sync for virtual scroll mode; this prevents
  // auto-scroll from forcing users back to bottom while they read older messages.
  isUserNearBottom.value = checkIfNearBottom()

  // Load more when scrolled near the top in virtual scroll mode.
  // Add a short cooldown + in-flight lock to avoid duplicate triggers
  // caused by visible range jitter near the boundary.
  if (
    start >= 5 ||
    !chatStore.hasMoreMessages ||
    chatStore.loadingMore ||
    virtualLoadMoreInFlight
  ) {
    return
  }

  const now = Date.now()
  const elapsed = now - virtualLoadMoreLastAt
  if (elapsed < VIRTUAL_LOAD_MORE_COOLDOWN_MS) {
    if (!virtualLoadMoreRetryTimer) {
      virtualLoadMoreRetryTimer = setTimeout(() => {
        virtualLoadMoreRetryTimer = null
        handleVisibleRangeChange(start, _end)
      }, VIRTUAL_LOAD_MORE_COOLDOWN_MS - elapsed)
    }
    return
  }

  virtualLoadMoreInFlight = true
  virtualLoadMoreLastAt = now
  chatStore.loadMoreMessages().finally(() => {
    virtualLoadMoreInFlight = false
    isUserNearBottom.value = checkIfNearBottom()
  })
}

async function handleSend(message: string, attachments?: FileAttachment[]) {
  if (!(await ensureLlmProviderConfigured(message))) {
    return
  }

  // Check for media generation intent before sending to chat
  const hasImages = attachments?.some((a) => a.type.startsWith('image/')) || false
  const imageCount = attachments?.filter((a) => a.type.startsWith('image/')).length || 0
  const imageFiles =
    attachments?.filter((a) => a.type.startsWith('image/')).map((a) => a.file) || []
  const detected = await mediaGen.classify(message, hasImages, imageCount, locale.value, imageFiles)
  if (detected) {
    // Media intent detected — show param panel instead of sending to chat.
    // Invalidate warmup cache since no LLM chat request will follow.
    chatStore.resetWarmup()
    chatInputRef.value?.resetWarmup?.()
    return
  }
  await chatStore.sendMessage(message, attachments)
}

async function handleMediaGenerate() {
  await mediaGen.generate()
}

async function handleMediaDismiss() {
  // User chose "No, just chat" — send the original message to chat instead.
  const prompt = mediaGen.intent.value?.prompt
  mediaGen.reset()
  if (prompt && (await ensureLlmProviderConfigured(prompt))) {
    chatStore.sendMessage(prompt)
  }
  nextTick(() => chatInputRef.value?.focus?.())
}

function handleMediaClose() {
  // Close button (✕) — just dismiss the panel, do nothing else.
  mediaGen.reset()
  nextTick(() => chatInputRef.value?.focus?.())
}

async function handleMediaConfirm() {
  await mediaGen.confirmAmbiguous()
}

// Handle voice transcript from TalkMode - auto send to AI
async function handleVoiceTranscript(text: string) {
  if (text.trim() && (await ensureLlmProviderConfigured(text))) {
    if (chatStore.streaming) {
      await chatStore.injectMessage(text)
    } else {
      await chatStore.sendMessage(text)
    }
  }
}

function handleCancel() {
  const hadMediaGen = mediaGen.generating.value
  const mediaType = mediaGen.task.value?.type // 'image' | 'video'
  const runningTasks = [...currentConversationActiveTasks.value]
  const runningDeepResearchJobs = [...currentConversationDeepResearchJobs.value]

  // 1. Cancel chat streaming
  chatStore.cancelStreaming()

  // 2. Cancel active media generation task
  if (hadMediaGen) {
    mediaGen.cancel()
  }

  // 3. Cancel running projected tasks in the current conversation
  if (runningTasks.length > 0) {
    void Promise.allSettled(
      runningTasks.map((task) => taskProjections.performTaskAction(task, 'cancel'))
    ).catch(() => {})
  }

  // 4. Cancel running deep research jobs in the current conversation
  if (runningDeepResearchJobs.length > 0) {
    void Promise.allSettled(
      runningDeepResearchJobs.map((job) => deepResearchJobs.cancelJob(job.job_id))
    ).catch(() => {})
  }

  // 5. Append a stop notification message if any async task was cancelled
  if (hadMediaGen || runningTasks.length > 0 || runningDeepResearchJobs.length > 0) {
    const parts: string[] = []
    if (hadMediaGen) {
      parts.push(t(mediaType === 'video' ? 'chat.videoGenStopped' : 'chat.imageGenStopped'))
    }
    for (const task of runningTasks) {
      parts.push(t('chat.agentTaskStopped', { goal: task.title }))
    }
    if (runningDeepResearchJobs.length > 0) {
      parts.push(t('chat.deepResearchActionLoopStopped', 'Research loop stopped'))
    }

    const cleaned = chatStore.messages.filter((m) => !m.id.startsWith('streaming-'))
    cleaned.push({
      id: `stop-${Date.now()}`,
      conversation_id: chatStore.currentConversationId || '',
      role: 'assistant',
      content: parts.join('\n'),
      created_at: new Date().toISOString(),
    })
    chatStore.messages = cleaned

    if (runningTasks.length > 0) {
      void taskProjections.refreshNow().catch(() => {})
    }
  }
}

function handleInject(message: string) {
  chatStore.injectMessage(message)
}

async function handleSelectConversation(id: string) {
  await chatStore.selectConversation(id)
  chatStore.resetWarmup()
  chatInputRef.value?.resetWarmup?.()
  const normalizedConversationId = normalizeConversationRouteParam(id)
  const currentRouteConversationId = normalizeConversationRouteParam(route.query.conversationId)
  if (normalizedConversationId && normalizedConversationId !== currentRouteConversationId) {
    const nextQuery = {
      ...route.query,
      conversationId: normalizedConversationId,
    }
    if (isMobile.value) {
      mobileAnimationEnabled.value = true
      pageStack.value.push(id)
      // Keep query param for deep-link restore on reload.
      await router.push({ query: nextQuery })
    } else {
      await router.replace({ query: nextQuery })
    }
    return
  }
  if (isMobile.value) {
    mobileAnimationEnabled.value = true
    pageStack.value.push(id)
  }
}

watch(
  () => route.query.conversationId,
  async (value, previous) => {
    const conversationId = normalizeConversationRouteParam(value)
    const previousConversationId = normalizeConversationRouteParam(previous)
    if (conversationId === previousConversationId) return

    if (isMobile.value) {
      mobileAnimationEnabled.value = true
      pageStack.value = conversationId ? [conversationId] : []
    }

    if (!conversationId || conversationId === (chatStore.currentConversationId || '')) return
    await chatStore.selectConversation(conversationId)
    chatStore.resetWarmup()
    chatInputRef.value?.resetWarmup?.()
  }
)

async function handleCreateConversation() {
  chatStore.resetWarmup()
  chatInputRef.value?.resetWarmup?.()
  const conv = await chatStore.createConversation(t('chat.newConversation'))
  if (isMobile.value && conv?.id) {
    mobileAnimationEnabled.value = true
    pageStack.value.push(conv.id)
    await router.push({ query: { conversationId: conv.id } })
  }
}

async function handleDeleteConversation(id: string) {
  await chatStore.deleteConversation(id)
}

async function handlePinConversation(id: string) {
  await chatStore.pinConversation(id)
}

async function handleUnpinConversation(id: string) {
  await chatStore.unpinConversation(id)
}

function handleSearch(query: string) {
  chatStore.searchConversations(query)
}

function toggleSidebar() {
  if (isMobile.value && pageStack.value.length > 0) {
    // Mobile: return to list page
    pageStack.value = []
    chatStore.currentConversationId = null
    // Clear conversation query on return to list.
    router.push({ query: {} })
  } else {
    showSidebar.value = !showSidebar.value
  }
}

function openAppSidebar() {
  toggleAppSidebar()
}

function resolveRoutingMenuAnchor(trigger?: HTMLElement | null) {
  return trigger ?? routingMenuAnchorEl.value ?? desktopRoutingMenuAnchorEl.value
}

function positionRoutingMenu(trigger: HTMLElement | null = resolveRoutingMenuAnchor()) {
  if (!trigger) return

  const rect = trigger.getBoundingClientRect()
  const menuWidth = routingMenuFloatingRef.value?.offsetWidth ?? 320
  const menuHeight = routingMenuFloatingRef.value?.offsetHeight ?? 420
  const viewportPadding = 8
  const menuGap = 12
  const maxX = Math.max(viewportPadding, window.innerWidth - menuWidth - viewportPadding)
  const maxY = Math.max(viewportPadding, window.innerHeight - menuHeight - viewportPadding)
  const x = Math.min(Math.max(viewportPadding, rect.right - menuWidth), maxX)
  const preferredTop = rect.top - menuHeight - menuGap
  const fallbackTop = rect.bottom + menuGap
  const y =
    preferredTop >= viewportPadding
      ? preferredTop
      : Math.max(viewportPadding, Math.min(fallbackTop, maxY))

  routingMenuPosition.value = { x, y }
}

function updateRoutingMenuPosition() {
  if (isMobile.value || !showRoutingMenu.value) return
  nextTick(() => {
    positionRoutingMenu()
  })
}

const scheduleRoutingMenuReposition = rafThrottle(() => {
  positionRoutingMenu()
})

function toggleRoutingMenu(trigger?: HTMLElement | null) {
  showTopbarMenu.value = false
  activeMobileFeatureSheet.value = null
  const anchor = resolveRoutingMenuAnchor(trigger)
  if (anchor) {
    routingMenuAnchorEl.value = anchor
  }
  if (showRoutingMenu.value) {
    showRoutingMenu.value = false
    return
  }
  showRoutingMenu.value = true
  updateRoutingMenuPosition()
}

function handleRoutingMenuButtonClick(event: MouseEvent) {
  const trigger = event.currentTarget
  if (trigger instanceof HTMLElement) {
    toggleRoutingMenu(trigger)
    return
  }
  toggleRoutingMenu()
}

function selectRoutingMode(mode: 'auto' | 'cloud' | 'local') {
  if (mode === 'cloud' && !providerPoolStore.hasCloudProviders) return
  if (mode === 'local' && !providerPoolStore.hasLocalProviders) return
  providerPoolStore.setRoutingMode(mode)
  if (isMobile.value) showRoutingMenu.value = false
}

function selectFixedModel(modelPreference: string) {
  chatStore.setModelPreference(modelPreference)
  if (isMobile.value) showRoutingMenu.value = false
}

function setAutoModelPreference() {
  chatStore.setModelPreference('auto')
  if (isMobile.value) showRoutingMenu.value = false
}

function selectProviderScopedAuto(providerId: string) {
  const normalizedProviderId = providerId.trim()
  if (!normalizedProviderId) return
  if (chatStore.providerPinOnlyActive && chatStore.selectedProviderId === normalizedProviderId) {
    setAutoModelPreference()
    return
  }
  chatStore.setProviderPinOnly(normalizedProviderId)
}

function enableSingleModelMode() {
  if (chatStore.modelPreference !== 'auto') return
  const firstModel = fixedModelOptions.value[0]?.value
  if (firstModel) {
    chatStore.setModelPreference(firstModel)
  }
}

function toggleToolDetails() {
  settingsStore.setShowToolDetails(!settingsStore.showToolDetails)
}

watch(
  () =>
    [
      showRoutingMenu.value,
      isMobile.value,
      showRoutingControl.value,
      isSingleModelMode.value,
      fixedModelOptions.value.length,
      chatStore.modelPreference,
    ] as const,
  ([open, mobile, routingVisible]) => {
    if (!routingVisible && open) {
      showRoutingMenu.value = false
      return
    }
    if (!open || mobile) return
    updateRoutingMenuPosition()
  }
)

function toggleTopbarMenu() {
  showRoutingMenu.value = false
  activeMobileFeatureSheet.value = null
  showTopbarMenu.value = !showTopbarMenu.value
}

function openTopbarMenu() {
  showRoutingMenu.value = false
  activeMobileFeatureSheet.value = null
  showTopbarMenu.value = true
}

function openMobileFeatureSheet(kind: MobileFeatureSheetKind) {
  showRoutingMenu.value = false
  showTopbarMenu.value = false
  activeMobileFeatureSheet.value = kind
}

function closeMobileFeatureSheet() {
  activeMobileFeatureSheet.value = null
}

function handleMobileFeaturePrimaryAction() {
  if (activeMobileFeatureSheet.value !== 'smart-resume') return
  settingsStore.setAgentMode(!settingsStore.agentMode).catch((err) => {
    console.error('Failed to update Smart Resume mode:', err)
  })
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement

  if (!target.closest('.topbar-more-container') && !target.closest('.topbar-sheet')) {
    showTopbarMenu.value = false
  }
  if (
    !target.closest('.routing-menu-container') &&
    !target.closest('.routing-menu-floating') &&
    !target.closest('.routing-menu-anchor')
  ) {
    showRoutingMenu.value = false
  }

  // On mobile, sheets handle their own click-outside via backdrop
  if (isMobile.value) {
    return
  }
  // Close context menu when clicking outside
  if (!target.closest('.context-menu') && !contextMenuJustOpened.value) {
    showContextMenu.value = false
    contextMenuSelectedText.value = ''
  }
}

// Context menu handlers
const contextMenuJustOpened = ref(false)

function getSelectedTextWithinElement(container: HTMLElement | null): string {
  const selection = window.getSelection()
  if (!selection || selection.rangeCount === 0 || selection.isCollapsed) return ''
  const selectedText = selection.toString()
  if (!selectedText.trim()) return ''
  if (!container) return ''

  const isInside = (node: Node | null) => !!node && container.contains(node)
  if (isInside(selection.anchorNode) || isInside(selection.focusNode)) {
    return selectedText
  }

  for (let i = 0; i < selection.rangeCount; i += 1) {
    const range = selection.getRangeAt(i)
    if (isInside(range.commonAncestorContainer)) {
      return selectedText
    }
  }
  return ''
}

function handleMessageContextMenu(event: MouseEvent, messageId: string) {
  event.preventDefault()

  // On mobile, context menu is handled by ChatMessage's own long-press menu
  if (isMobile.value) return

  const eventTarget = event.target
  const messageElement =
    eventTarget instanceof Element
      ? (eventTarget.closest('.message') as HTMLElement | null)
      : eventTarget instanceof Node
        ? (eventTarget.parentElement?.closest('.message') as HTMLElement | null)
        : null
  contextMenuSelectedText.value = getSelectedTextWithinElement(messageElement)
  contextMenuMessageId.value = messageId
  contextMenuPosition.value = { x: event.clientX, y: event.clientY }
  showContextMenu.value = true
  // Prevent the subsequent click event from immediately closing the menu
  contextMenuJustOpened.value = true
  setTimeout(() => {
    contextMenuJustOpened.value = false
  }, 200)
}

async function handleContextCopy() {
  const selectedText = contextMenuSelectedText.value
  const hasSelectedText = selectedText.trim().length > 0
  const fallbackMessageContent = contextMenuMessageId.value
    ? (chatStore.messages.find((message) => message.id === contextMenuMessageId.value)?.content ??
      '')
    : ''
  const textToCopy = hasSelectedText ? selectedText : fallbackMessageContent
  showContextMenu.value = false
  contextMenuSelectedText.value = ''
  if (!textToCopy) return

  try {
    await navigator.clipboard.writeText(textToCopy)
  } catch (err) {
    console.error('Failed to copy from context menu:', err)
  }
}

function handleSelectMessage() {
  if (contextMenuMessageId.value) {
    chatStore.enterMultiSelectMode(contextMenuMessageId.value)
  }
  showContextMenu.value = false
  contextMenuSelectedText.value = ''
}

// Check if the context-menu'd message is the last assistant message (for Continue/Regenerate)
const isContextMenuLastAssistant = computed(() => {
  return !!contextMenuMessageId.value && contextMenuMessageId.value === lastAssistantMessageId.value
})

function handleContextContinue() {
  chatStore.continueMessage()
  showContextMenu.value = false
  contextMenuSelectedText.value = ''
}

function handleContextRegenerate() {
  chatStore.regenerateMessage()
  showContextMenu.value = false
  contextMenuSelectedText.value = ''
}

function handleMessageContinue() {
  chatStore.continueMessage()
}

function handleMessageRegenerate() {
  chatStore.regenerateMessage()
}

async function handleMessageEditResubmit(messageId: string, content: string) {
  await chatStore.editMessageAndResubmit(messageId, content)
}

function handleStreamRetry() {
  if (chatStore.isStreamInterrupted) {
    chatStore.retryInterruptedStreamRecovery()
    return
  }
  chatStore.clearStreamError()
  chatStore.regenerateMessage()
}

function handleOpenProviderSettings(providerId?: string) {
  closeProviderConfigDialog()
  void router.push({
    path: '/settings',
    query: providerId ? { tab: 'llm', provider: providerId } : { tab: 'llm' },
  })
}

async function handleProviderGuidanceRetry() {
  const provider = recoverableAttentionProvider.value
  if (!provider) return
  await providerPoolStore.clearProviderError(provider.id)
}

function dismissModelAutoFallbackDialog() {
  chatStore.dismissModelAutoFallbackRetry()
}

function confirmModelAutoFallbackDialog() {
  void chatStore.confirmModelAutoFallbackRetry()
}

function closeProviderConfigDialog() {
  showProviderConfigDialog.value = false
}

async function handleDeleteSelectedMessages() {
  if (chatStore.selectedMessageIds.size === 0) return

  try {
    await chatStore.deleteSelectedMessages()
  } catch {
    // Error is handled in store
  }
}

function handleCancelSelection() {
  chatStore.exitMultiSelectMode()
}

function getMessageIndex(messageId: string): number {
  const normalizedMessageId = messageId.trim()
  if (!normalizedMessageId) return -1
  return chatStore.messages.findIndex(
    (message) => message.id === normalizedMessageId || message.render_key === normalizedMessageId
  )
}

function findMessageElement(messageId: string): HTMLElement | null {
  const normalizedMessageId = messageId.trim()
  if (!normalizedMessageId) return null
  const container = messagesContainer.value
  if (container) {
    const localMatch =
      Array.from(container.querySelectorAll<HTMLElement>('[data-message-id]')).find(
        (node) => node.dataset.messageId === normalizedMessageId
      ) || null
    if (localMatch) return localMatch
  }
  return document.getElementById(getChatMessageElementId(normalizedMessageId))
}

function resolveEdgeQuickNavItemByMessageIndex(messageIndex: number): EdgeQuickNavItem | null {
  if (edgeQuickNavItems.value.length === 0) return null

  let candidate: EdgeQuickNavItem | null = edgeQuickNavItems.value[0] ?? null
  for (const item of edgeQuickNavItems.value) {
    if (item.messageIndex > messageIndex) break
    candidate = item
  }
  return candidate
}

function resolveEdgeQuickNavActiveMessageId(): string {
  const firstItem = edgeQuickNavItems.value[0]
  if (!firstItem) return ''

  if (useVirtualScroll.value) {
    return (
      resolveEdgeQuickNavItemByMessageIndex(visibleMessageRangeStart.value + 1)?.messageId ??
      firstItem.messageId
    )
  }

  const container = messagesContainer.value
  if (!container || container.clientHeight <= 0) {
    return firstItem.messageId
  }

  const renderedMessages = Array.from(container.querySelectorAll<HTMLElement>('[data-message-id]'))
  if (renderedMessages.length === 0) return firstItem.messageId

  const containerRect = container.getBoundingClientRect()
  const anchorY = containerRect.top + Math.min(Math.max(containerRect.height * 0.22, 72), 128)
  let activeMessageId = renderedMessages[0]?.dataset.messageId || firstItem.messageId

  for (const element of renderedMessages) {
    const messageId = element.dataset.messageId
    if (!messageId) continue
    if (element.getBoundingClientRect().top <= anchorY) {
      activeMessageId = messageId
      continue
    }
    break
  }

  return (
    resolveEdgeQuickNavItemByMessageIndex(getMessageIndex(activeMessageId))?.messageId ??
    firstItem.messageId
  )
}

function syncEdgeQuickNavActiveMessage() {
  if (!showEdgeQuickNav.value) {
    activeEdgeQuickNavMessageId.value = ''
    return
  }
  activeEdgeQuickNavMessageId.value = resolveEdgeQuickNavActiveMessageId()
}

function handleEdgeQuickNavJump(messageId: string) {
  activeEdgeQuickNavMessageId.value = messageId
  void focusMessageById(messageId)
}

function messageShellClasses(message: MessageMemoSource) {
  return {
    'chat-message-shell': true,
    'is-todo-focused': focusedTodoMessageId.value === message.id,
  }
}

function markTodoMessageFocused(messageId: string) {
  focusedTodoMessageId.value = messageId
  if (focusedTodoMessageTimer) {
    clearTimeout(focusedTodoMessageTimer)
  }
  focusedTodoMessageTimer = setTimeout(() => {
    focusedTodoMessageId.value = null
    focusedTodoMessageTimer = null
  }, 2200)
}

function waitForAnimationFrame(): Promise<void> {
  return new Promise((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
}

async function focusMessageById(messageId: string): Promise<boolean> {
  const normalizedMessageId = messageId.trim()
  if (!normalizedMessageId) return false

  const existingTarget = findMessageElement(normalizedMessageId)
  if (existingTarget) {
    existingTarget.scrollIntoView({ block: 'center', behavior: 'smooth' })
    markTodoMessageFocused(normalizedMessageId)
    return true
  }

  const messageIndex = getMessageIndex(normalizedMessageId)
  if (messageIndex < 0) return false

  if (useVirtualScroll.value && virtualScrollRef.value) {
    virtualScrollRef.value.scrollToItem(messageIndex, 'smooth')
  }

  for (let attempt = 0; attempt < 6; attempt++) {
    await nextTick()
    await waitForAnimationFrame()
    const target = findMessageElement(normalizedMessageId)
    if (!target) continue
    target.scrollIntoView({ block: 'center', behavior: 'smooth' })
    markTodoMessageFocused(normalizedMessageId)
    return true
  }

  return false
}

function toggleActiveTodoPanel() {
  activeTodoPanelCollapsed.value = !activeTodoPanelCollapsed.value
}

function handleActiveTodoPanelJump() {
  const messageId = activeTodoJumpMessageId.value
  if (!messageId) return
  void focusMessageById(messageId)
}

watch(activeTodoPanelCollapsed, (value) => {
  persistActiveTodoPanelCollapsed(value)
})

watch(
  () => activeTodoSummary.value?.messageId ?? '',
  (messageId, previousMessageId) => {
    if (messageId && messageId !== previousMessageId) {
      activeTodoPanelCollapsed.value = false
    }
  }
)

watch(
  () => chatStore.currentConversationId,
  () => {
    focusedTodoMessageId.value = null
  }
)

watch(
  [
    () => showEdgeQuickNav.value,
    () => chatStore.currentConversationId,
    () => edgeQuickNavSignature.value,
  ],
  ([enabled]) => {
    if (!enabled) {
      activeEdgeQuickNavMessageId.value = ''
      return
    }
    visibleMessageRangeStart.value = 0
    nextTick(() => {
      scheduleEdgeQuickNavSync()
    })
  },
  { immediate: true }
)

async function ensureLlmProviderConfigured(messageToRestore?: string) {
  if (providerPoolStore.providers.length === 0) {
    try {
      await providerPoolStore.fetchProviders()
    } catch {
      // Fall through to the provider check below.
    }
  }

  const enabledLlmProviders = providerPoolStore.enabledProviders.filter(
    (provider) => provider.type !== 'media'
  )
  if (enabledLlmProviders.length === 0) {
    // When no LLM provider is configured yet, let the request continue so the
    // backend can fall back to web search instead of hard-blocking the chat UI.
    return true
  }

  const hasActiveLlmProvider = enabledLlmProviders.some((provider) => provider.status === 'active')
  if (hasActiveLlmProvider) {
    return true
  }

  const hasProvisionallyAvailableProvider = enabledLlmProviders.some((provider) =>
    providerAppearsAvailable(provider.status)
  )
  if (hasProvisionallyAvailableProvider) {
    return true
  }

  // Configured providers may be unhealthy while chat/media fallbacks remain usable.
  // Keep inline provider attention guidance visible, but don't hard-block the request here.
  void messageToRestore
  return true
}

// Handle preset question selection
async function handlePresetQuestionSelect(text: string, attachments?: FileAttachment[]) {
  await handleSend(text, attachments)
}

function handleDraftChange(message: string) {
  presetQuestionDraft.value = message
}

async function hydrateInitialChatState() {
  if (initialChatBootstrapStarted || viewUnmounted) return
  initialChatBootstrapStarted = true
  initialPrimaryDataHydrating.value = true
  reportStartupMark('chat_view_primary_data_start')

  try {
    // Fetch conversations and (if URL has conversationId) messages in parallel.
    // selectConversation only needs the ID, not the conversation list.
    if (_initConvId) {
      await Promise.all([chatStore.fetchConversations(), chatStore.selectConversation(_initConvId)])
    } else {
      await chatStore.fetchConversations()
      if (viewUnmounted) return

      // Auto-select first conversation if available and none selected (desktop only)
      if (
        !isMobile.value &&
        !chatStore.currentConversationId &&
        chatStore.sortedConversations.length > 0 &&
        chatStore.sortedConversations[0]
      ) {
        await chatStore.selectConversation(chatStore.sortedConversations[0].id)
      }
    }

    reportStartupMark('chat_view_primary_data_ready', {
      conversation_count: chatStore.sortedConversations.length,
      message_count: chatStore.messages.length,
      has_current_conversation: !!chatStore.currentConversationId,
    })

    await nextTick()
    if (viewUnmounted) return
    reportStartupMark('chat_view_dom_ready', {
      conversation_count: chatStore.sortedConversations.length,
      message_count: chatStore.messages.length,
    })
  } finally {
    initialPrimaryDataHydrating.value = false
  }
}

onMounted(async () => {
  reportStartupMark('chat_view_mounted')

  checkMobile()
  window.addEventListener('resize', checkMobileOnResize)
  document.addEventListener('click', handleClickOutside)
  void taskProjections.setConversation(chatStore.currentConversationId || '').catch(() => {})

  await nextTick()
  reportStartupMark('chat_view_shell_ready', {
    has_initial_conversation_id: !!_initConvId,
  })
  queuePostPaintTask(() => {
    void hydrateInitialChatState()
  })

  queueBackgroundTask(() => {
    loadMarkdownModule()
      .then(({ preloadHljs }) => preloadHljs())
      .catch(() => {})
  }, 1500)

  queueBackgroundTask(() => {
    componentPool
      .preload([
        'progress',
        'chart',
        'gallery',
        'link',
        'file',
        'deep-research',
        'deep-research-timeline',
        'deep-research-progress',
        'deep-research-event',
      ])
      .catch(() => {})
  }, 2000)

  queueBackgroundTask(() => {
    loadApiClientModule()
      .then(({ authFetch }) => authFetch('/api/v1/speech/init', { method: 'POST' }))
      .catch(() => {})
  }, 2500)

  queueBackgroundTask(() => {
    if (providerPoolStore.providers.length === 0) {
      providerPoolStore
        .fetchProviders()
        .then(() => {
          const llmProviders = providerPoolStore.providers.filter((p: any) => p.type !== 'media')
          settingsStore.updateFromPoolProviders(llmProviders)
        })
        .catch(() => {})
    }
    settingsStore.fetchTools().catch(() => {})
    providerPoolStore.fetchRoutingMode().catch(() => {})
    if (settingsStore.experimentalAgentcoreRunnerEnabled) {
      settingsStore.fetchAgentcoreRunnerStatus().catch(() => {})
    }

    // Restore pending confirmations after refresh or missed SSE events.
    chatStore.recoverPendingConfirmations(false)
  }, 800)
})

onUnmounted(() => {
  viewUnmounted = true
  clearBackgroundTasks()
  messageRenderMetaCache.clear()
  lastRenderMetaLookupKey = ''
  lastRenderMetaLookupMessage = null
  lastRenderMetaLookupValue = null
  if (normalScrollRafId !== null) {
    window.cancelAnimationFrame(normalScrollRafId)
    normalScrollRafId = null
  }
  if (virtualLoadMoreRetryTimer) {
    clearTimeout(virtualLoadMoreRetryTimer)
    virtualLoadMoreRetryTimer = null
  }
  if (focusedTodoMessageTimer) {
    clearTimeout(focusedTodoMessageTimer)
    focusedTodoMessageTimer = null
  }
  clearVirtualItemObservers()
  if (autoScrollRafId !== null) {
    window.cancelAnimationFrame(autoScrollRafId)
    autoScrollRafId = null
  }
  taskProjections.stopPolling()
  window.removeEventListener('resize', checkMobileOnResize)
  checkMobileOnResize.cancel()
  scheduleRoutingMenuReposition.cancel()
  scheduleEdgeQuickNavSync.cancel()
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <div
    class="chat-view ui-density-standard relative flex min-w-0 w-full"
    :class="{ 'mobile-view': isMobile && !showListPage, 'chat-desktop-shell': !isMobile }"
  >
    <!-- Overlay for narrow screen sidebar -->
    <div
      v-if="!isMobile && isNarrowScreen && showSidebar"
      class="fixed inset-0 bg-black/60 backdrop-blur-sm z-30"
      @click="toggleSidebar"
    />

    <div class="chat-workspace flex min-h-0 min-w-0 w-full flex-1 flex-col">
      <header v-if="!isMobile" class="chat-page-header">
        <div class="chat-page-header-inner">
          <div class="chat-page-heading">
            <button
              v-if="isNarrowScreen"
              class="chat-nav-btn text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white transition-all duration-200 cursor-pointer"
              :title="showSidebar ? '关闭会话列表' : '打开会话列表'"
              @click="toggleSidebar"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 6h16M4 12h16M4 18h16"
                />
              </svg>
            </button>
            <h1 class="chat-page-title text-gray-900 dark:text-white">
              {{ t('nav.chat') }}
            </h1>
          </div>
          <div v-if="isNarrowScreen" class="chat-page-actions">
            <button
              class="topbar-icon-btn text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white transition-colors cursor-pointer"
              :title="t('nav.expandSidebar')"
              @click="openAppSidebar"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.9"
                  d="M4.5 6.75A2.25 2.25 0 016.75 4.5h10.5a2.25 2.25 0 012.25 2.25v10.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 17.25V6.75zm4.5-2.25v15"
                />
              </svg>
            </button>
          </div>
        </div>
      </header>

      <div
        class="chat-body-shell flex min-h-0 min-w-0 w-full flex-1"
        :class="{ 'chat-body-shell-mobile': isMobile }"
      >
        <!-- Sidebar / Conversation List -->
        <aside
          v-show="isMobile ? showListPage : showSidebar"
          class="conversation-sidebar chat-sidebar-shell min-w-0 flex-shrink-0 transition-transform duration-300"
          :style="!isMobile && isNarrowScreen ? { insetInlineStart: '0' } : undefined"
          :class="{
            'w-[16.75rem]': !isMobile,
            'w-full max-w-full': isMobile,
            'z-40': !isMobile,
            'absolute inset-y-0': !isMobile && isNarrowScreen,
            '-translate-x-full': !isMobile && isNarrowScreen && !showSidebar && !isRtl,
            'translate-x-full': !isMobile && isNarrowScreen && !showSidebar && isRtl,
            'mobile-list-page': isMobile && showListPage,
            'chat-sidebar-mobile': isMobile,
          }"
        >
          <ConversationList
            :conversations="chatStore.sortedConversations"
            :current-id="chatStore.currentConversationId"
            :executing-conversation-ids="executingConversationIds"
            :loading="chatStore.loading"
            :searching="chatStore.searching"
            :mobile="isMobile"
            @select="handleSelectConversation"
            @create="handleCreateConversation"
            @delete="handleDeleteConversation"
            @search="handleSearch"
            @pin="handlePinConversation"
            @unpin="handleUnpinConversation"
            @more-actions="openTopbarMenu"
          />
        </aside>

        <!-- Main chat area -->
        <main
          :class="
            isMobile
              ? [
                  'mobile-chat',
                  {
                    'mobile-chat-offscreen': showListPage,
                    'mobile-chat-animated': mobileAnimationEnabled,
                  },
                ]
              : ''
          "
          class="chat-main-shell flex-1 flex flex-col min-w-0 min-h-0 h-full relative"
          @dragover.prevent="chatInputRef?.handleDragOver($event)"
          @dragleave="chatInputRef?.handleDragLeave()"
          @drop.prevent="chatInputRef?.handleDrop($event)"
        >
          <!-- Chat header -->
          <header
            v-if="isMobile"
            class="chat-topbar relative z-50 flex items-center justify-between border-b"
            :class="
              isMobile
                ? 'chat-topbar-mobile bg-white dark:bg-slate-900 border-gray-200 dark:border-slate-700'
                : 'border-gray-200/80 dark:border-slate-700/80'
            "
          >
            <div class="chat-title-wrap flex items-center min-w-0 flex-1">
              <!-- Back/Sidebar toggle button -->
              <button
                class="chat-nav-btn flex-shrink-0 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white transition-all duration-200 cursor-pointer"
                :class="{ 'md:hidden': !isMobile }"
                @click="toggleSidebar"
                :title="showSidebar ? '关闭会话列表' : '打开会话列表'"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15 19l-7-7 7-7"
                  />
                </svg>
              </button>
              <div class="chat-title-stack min-w-0">
                <span class="chat-section-label">{{ t('nav.chat') }}</span>
                <h2 class="chat-title text-gray-900 dark:text-white truncate">
                  {{ chatStore.currentConversation?.title || t('chat.newChat') }}
                </h2>
              </div>
            </div>

            <!-- Topbar actions -->
            <div class="chat-tools flex items-center flex-shrink-0">
              <button
                v-if="isMobile && !showListPage && !hasGlobalMobileSidebarToggle"
                class="topbar-icon-btn text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white transition-colors cursor-pointer"
                :title="t('nav.expandSidebar')"
                @click="openAppSidebar"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M4 6h16M4 12h16M4 18h16"
                  />
                </svg>
              </button>

              <!-- Mobile: collapsed topbar actions trigger -->
              <div v-if="shouldCollapseTopbarControls" class="topbar-more-container relative">
                <button
                  class="topbar-icon-btn text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white transition-colors cursor-pointer"
                  :class="{
                    'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-white': showTopbarMenu,
                  }"
                  data-testid="chat-topbar-more-actions"
                  :title="t('chat.moreActions')"
                  @click.stop="toggleTopbarMenu"
                >
                  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.8"
                      d="M4 5a1 1 0 011-1h5a1 1 0 011 1v5a1 1 0 01-1 1H5a1 1 0 01-1-1V5zm9 0a1 1 0 011-1h5a1 1 0 011 1v5a1 1 0 01-1 1h-5a1 1 0 01-1-1V5zM4 14a1 1 0 011-1h5a1 1 0 011 1v5a1 1 0 01-1 1H5a1 1 0 01-1-1v-5zm9 0a1 1 0 011-1h5a1 1 0 011 1v5a1 1 0 01-1 1h-5a1 1 0 01-1-1v-5z"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </header>
          <Teleport to="body">
            <Transition
              enter-active-class="transition ease-out duration-150"
              enter-from-class="opacity-0 translate-y-1"
              enter-to-class="opacity-100 translate-y-0"
              leave-active-class="transition ease-in duration-100"
              leave-from-class="opacity-100 translate-y-0"
              leave-to-class="opacity-0 translate-y-1"
            >
              <div
                v-if="showRoutingMenu && !isMobile"
                ref="routingMenuFloatingRef"
                class="routing-menu-floating fixed w-80 rounded-2xl shadow-2xl border overflow-hidden z-[20000] bg-white dark:bg-slate-900 border-gray-300 dark:border-slate-500"
                :style="{
                  /* rtl-audit-ignore-next-line: positioned from viewport coordinates */
                  left: `${routingMenuPosition.x}px`,
                  top: `${routingMenuPosition.y}px`,
                }"
              >
                <div class="px-3 py-2 border-b border-gray-200 dark:border-slate-700">
                  <div class="text-xs font-medium text-gray-700 dark:text-slate-300">
                    {{ routingMenuStatusMessage }}
                  </div>
                </div>

                <router-link
                  v-if="needsProviderAttention"
                  to="/settings?tab=llm"
                  class="routing-manage-row w-full px-4 py-3 flex items-center justify-between text-sm text-gray-900 dark:text-slate-100 transition-colors"
                  @click="showRoutingMenu = false"
                >
                  <span class="inline-flex items-center gap-2">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M11 5h2m-6 0h2m6 0h2m-5 0v2m0 10v2m0-2h2m-2 0h-2m6-10a2 2 0 012 2v8a2 2 0 01-2 2H7a2 2 0 01-2-2V9a2 2 0 012-2h10z"
                      />
                    </svg>
                    {{ providerInlineGuidanceCopy.primaryAction }}
                  </span>
                  <span class="text-xs text-gray-600 dark:text-slate-400">{{
                    t('chat.manageProviders')
                  }}</span>
                </router-link>

                <div>
                  <div class="p-3 space-y-1.5">
                    <button
                      class="routing-option-row w-full"
                      :class="{ 'is-active': providerPoolStore.routingMode === 'auto' }"
                      @click="selectRoutingMode('auto')"
                    >
                      <div class="routing-option-main">
                        <svg
                          class="routing-option-icon"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                          />
                        </svg>
                        <div class="routing-option-copy">
                          <div class="routing-option-title">{{ t('chat.routingMode.auto') }}</div>
                          <div class="routing-option-desc">
                            {{ t('chat.routingMode.autoDesc') }}
                          </div>
                        </div>
                      </div>
                      <div class="routing-option-side">
                        <span class="routing-option-count">{{ totalActiveProviderCount }}</span>
                        <span
                          v-if="providerPoolStore.routingMode === 'auto'"
                          class="routing-option-status-chip"
                        >
                          {{ routingStrategyLabel }}
                        </span>
                      </div>
                    </button>

                    <button
                      v-if="showLocationRoutingOptions"
                      class="routing-option-row w-full"
                      :class="{
                        'is-active': providerPoolStore.routingMode === 'cloud',
                        'is-disabled': !providerPoolStore.hasCloudProviders,
                      }"
                      :disabled="!providerPoolStore.hasCloudProviders"
                      @click="selectRoutingMode('cloud')"
                    >
                      <div class="routing-option-main">
                        <svg
                          class="routing-option-icon"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"
                          />
                        </svg>
                        <div class="routing-option-copy">
                          <div class="routing-option-title">{{ t('chat.routingMode.cloud') }}</div>
                          <div class="routing-option-desc">
                            {{ t('chat.routingMode.cloudDesc') }}
                          </div>
                        </div>
                      </div>
                      <div class="routing-option-side">
                        <span class="routing-option-count">{{ cloudActiveCount }}</span>
                        <span
                          v-if="providerPoolStore.routingMode === 'cloud'"
                          class="routing-option-status-chip"
                        >
                          {{ routingStrategyLabel }}
                        </span>
                      </div>
                    </button>

                    <button
                      v-if="showLocationRoutingOptions"
                      class="routing-option-row w-full"
                      :class="{
                        'is-active': providerPoolStore.routingMode === 'local',
                        'is-disabled': !providerPoolStore.hasLocalProviders,
                      }"
                      :disabled="!providerPoolStore.hasLocalProviders"
                      @click="selectRoutingMode('local')"
                    >
                      <div class="routing-option-main">
                        <svg
                          class="routing-option-icon"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                          />
                        </svg>
                        <div class="routing-option-copy">
                          <div class="routing-option-title">{{ t('chat.routingMode.local') }}</div>
                          <div class="routing-option-desc">
                            {{ t('chat.routingMode.localDesc') }}
                          </div>
                        </div>
                      </div>
                      <div class="routing-option-side">
                        <span class="routing-option-count">{{ localActiveCount }}</span>
                        <span
                          v-if="providerPoolStore.routingMode === 'local'"
                          class="routing-option-status-chip"
                        >
                          {{ routingStrategyLabel }}
                        </span>
                      </div>
                    </button>
                  </div>

                  <div class="px-3 pt-1 pb-3 border-t border-gray-200 dark:border-slate-700">
                    <div class="grid grid-cols-2 gap-2">
                      <button
                        class="routing-strategy-chip"
                        :class="{ 'is-active-ha': !isSingleModelMode }"
                        @click="setAutoModelPreference"
                      >
                        {{ t('chat.routingMode.highAvailability') }}
                      </button>
                      <button
                        class="routing-strategy-chip"
                        :class="{ 'is-active-fixed': isSingleModelMode }"
                        :disabled="fixedModelOptions.length === 0"
                        @click="enableSingleModelMode"
                      >
                        {{ t('chat.routingMode.fixedModel') }}
                      </button>
                    </div>
                    <div v-if="!isSingleModelMode">
                      <div
                        v-if="!showProviderScopedAutoNotice"
                        class="mt-2 text-xs text-gray-600 dark:text-slate-400"
                      >
                        {{ providerScopedAutoDescription }}
                      </div>
                      <div
                        v-if="showProviderScopedAutoNotice"
                        data-testid="routing-provider-pin-notice"
                        class="mt-2 rounded-xl border border-sky-200/80 bg-sky-50/90 px-3 py-2 text-xs text-sky-900 dark:border-sky-900/60 dark:bg-sky-950/40 dark:text-sky-100"
                      >
                        <div class="font-semibold">
                          {{ providerScopedAutoNoticeTitle }}
                        </div>
                        <div class="mt-1 leading-5">
                          {{ providerScopedAutoNoticeDescription }}
                        </div>
                        <button
                          data-testid="routing-provider-pin-clear"
                          class="mt-2 inline-flex items-center text-[11px] font-semibold text-sky-700 transition-colors hover:text-sky-900 dark:text-sky-300 dark:hover:text-sky-100"
                          @click="setAutoModelPreference"
                        >
                          {{ providerScopedAutoResetLabel }}
                        </button>
                      </div>
                      <div v-if="showProviderScopedAutoOptions" class="mt-3 space-y-1">
                        <div
                          class="px-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500 dark:text-slate-400"
                        >
                          {{ providerScopedAutoNoticeTitle }}
                        </div>
                        <button
                          v-for="option in providerScopedAutoOptions"
                          :key="option.id"
                          :data-testid="`routing-provider-pin-row-${option.id}`"
                          class="routing-model-row w-full"
                          :class="{
                            'is-selected':
                              chatStore.providerPinOnlyActive &&
                              chatStore.selectedProviderId === option.id,
                          }"
                          @click="selectProviderScopedAuto(option.id)"
                        >
                          <span class="truncate" style="padding-inline-end: 0.75rem">
                            {{ option.label }}
                          </span>
                          <svg
                            v-if="
                              chatStore.providerPinOnlyActive &&
                              chatStore.selectedProviderId === option.id
                            "
                            class="w-4 h-4 text-sky-500 flex-shrink-0"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M5 13l4 4L19 7"
                            />
                          </svg>
                        </button>
                      </div>
                    </div>
                    <div v-else class="mt-2 max-h-40 overflow-y-auto space-y-1">
                      <button
                        v-for="option in fixedModelOptions"
                        :key="option.value"
                        class="routing-model-row w-full"
                        :class="{ 'is-selected': chatStore.modelPreference === option.value }"
                        @click="selectFixedModel(option.value)"
                      >
                        <span
                          class="truncate"
                          style="padding-inline-end: 0.75rem"
                          :title="option.value"
                          >{{ option.label }}</span
                        >
                        <svg
                          v-if="chatStore.modelPreference === option.value"
                          class="w-4 h-4 text-emerald-500 flex-shrink-0"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2.5"
                            d="M5 13l4 4L19 7"
                          />
                        </svg>
                      </button>
                    </div>
                  </div>

                  <div class="border-t border-gray-200 dark:border-slate-700" />
                  <router-link
                    to="/settings?tab=llm"
                    class="routing-manage-row w-full px-4 py-3 flex items-center justify-between text-sm text-gray-800 dark:text-slate-200 transition-colors"
                    @click="showRoutingMenu = false"
                  >
                    <span class="inline-flex items-center gap-2">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M11.983 5.5a1.75 1.75 0 0 1 3.034 0l.25.433a1.75 1.75 0 0 0 1.514.875h.5a1.75 1.75 0 0 1 1.517 2.625l-.25.433a1.75 1.75 0 0 0 0 1.75l.25.433a1.75 1.75 0 0 1-1.517 2.625h-.5a1.75 1.75 0 0 0-1.514.875l-.25.433a1.75 1.75 0 0 1-3.034 0l-.25-.433a1.75 1.75 0 0 0-1.514-.875h-.5a1.75 1.75 0 0 1-1.517-2.625l.25-.433a1.75 1.75 0 0 0 0-1.75l-.25-.433a1.75 1.75 0 0 1 1.517-2.625h.5a1.75 1.75 0 0 0 1.514-.875l.25-.433Z"
                        />
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M13.5 12a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0Z"
                        />
                      </svg>
                      {{ t('chat.manageProviders') }}
                    </span>
                    <span class="text-xs text-gray-500 dark:text-slate-500">→</span>
                  </router-link>
                </div>
              </div>
            </Transition>
          </Teleport>
          <Teleport to="body">
            <Transition name="sheet">
              <div
                v-if="showRoutingMenu && isMobile"
                class="routing-sheet fixed inset-0 z-[9999] flex items-end"
                @click="showRoutingMenu = false"
              >
                <div class="absolute inset-0 bg-black/60" />
                <div
                  class="routing-sheet-panel relative w-full rounded-t-2xl shadow-2xl"
                  @click.stop
                >
                  <div class="flex justify-center pt-3 pb-2">
                    <div class="w-10 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
                  </div>
                  <div class="px-4 pb-2">
                    <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                      {{ t('chat.routingMode.title') }}
                    </h3>
                  </div>
                  <div class="p-4 space-y-2">
                    <router-link
                      v-if="needsProviderAttention"
                      to="/settings?tab=llm"
                      class="w-full px-4 py-3 flex items-center justify-between rounded-xl text-sm text-gray-800 dark:text-slate-100 border border-gray-200 dark:border-slate-600 bg-gray-50 dark:bg-slate-800"
                      @click="showRoutingMenu = false"
                    >
                      <span>{{ providerInlineGuidanceCopy.primaryAction }}</span>
                      <span class="text-xs text-gray-600 dark:text-slate-300">{{
                        t('chat.manageProviders')
                      }}</span>
                    </router-link>

                    <div>
                      <div class="text-xs text-gray-700 dark:text-slate-300 px-1 pb-1">
                        {{ routingMenuStatusMessage }}
                      </div>

                      <button
                        class="routing-option-row w-full"
                        :class="{ 'is-active': providerPoolStore.routingMode === 'auto' }"
                        @click="selectRoutingMode('auto')"
                      >
                        <div class="routing-option-main">
                          <svg
                            class="routing-option-icon"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                            />
                          </svg>
                          <div class="routing-option-copy">
                            <div class="routing-option-title">{{ t('chat.routingMode.auto') }}</div>
                            <div class="routing-option-desc">
                              {{ t('chat.routingMode.autoDesc') }}
                            </div>
                          </div>
                        </div>
                        <div class="routing-option-side">
                          <span class="routing-option-count">{{ totalActiveProviderCount }}</span>
                          <span
                            v-if="providerPoolStore.routingMode === 'auto'"
                            class="routing-option-status-chip"
                          >
                            {{ routingStrategyLabel }}
                          </span>
                        </div>
                      </button>

                      <button
                        v-if="showLocationRoutingOptions"
                        class="routing-option-row w-full"
                        :class="{
                          'is-active': providerPoolStore.routingMode === 'cloud',
                          'is-disabled': !providerPoolStore.hasCloudProviders,
                        }"
                        :disabled="!providerPoolStore.hasCloudProviders"
                        @click="selectRoutingMode('cloud')"
                      >
                        <div class="routing-option-main">
                          <svg
                            class="routing-option-icon"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"
                            />
                          </svg>
                          <div class="routing-option-copy">
                            <div class="routing-option-title">
                              {{ t('chat.routingMode.cloud') }}
                            </div>
                            <div class="routing-option-desc">
                              {{ t('chat.routingMode.cloudDesc') }}
                            </div>
                          </div>
                        </div>
                        <div class="routing-option-side">
                          <span class="routing-option-count">{{ cloudActiveCount }}</span>
                          <span
                            v-if="providerPoolStore.routingMode === 'cloud'"
                            class="routing-option-status-chip"
                          >
                            {{ routingStrategyLabel }}
                          </span>
                        </div>
                      </button>

                      <button
                        v-if="showLocationRoutingOptions"
                        class="routing-option-row w-full"
                        :class="{
                          'is-active': providerPoolStore.routingMode === 'local',
                          'is-disabled': !providerPoolStore.hasLocalProviders,
                        }"
                        :disabled="!providerPoolStore.hasLocalProviders"
                        @click="selectRoutingMode('local')"
                      >
                        <div class="routing-option-main">
                          <svg
                            class="routing-option-icon"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                            />
                          </svg>
                          <div class="routing-option-copy">
                            <div class="routing-option-title">
                              {{ t('chat.routingMode.local') }}
                            </div>
                            <div class="routing-option-desc">
                              {{ t('chat.routingMode.localDesc') }}
                            </div>
                          </div>
                        </div>
                        <div class="routing-option-side">
                          <span class="routing-option-count">{{ localActiveCount }}</span>
                          <span
                            v-if="providerPoolStore.routingMode === 'local'"
                            class="routing-option-status-chip"
                          >
                            {{ routingStrategyLabel }}
                          </span>
                        </div>
                      </button>

                      <div class="grid grid-cols-2 gap-2 pt-2">
                        <button
                          class="routing-strategy-chip"
                          :class="{ 'is-active-ha': !isSingleModelMode }"
                          @click="setAutoModelPreference"
                        >
                          {{ t('chat.routingMode.highAvailability') }}
                        </button>
                        <button
                          class="routing-strategy-chip"
                          :class="{ 'is-active-fixed': isSingleModelMode }"
                          :disabled="fixedModelOptions.length === 0"
                          @click="enableSingleModelMode"
                        >
                          {{ t('chat.routingMode.fixedModel') }}
                        </button>
                      </div>

                      <div v-if="!isSingleModelMode">
                        <div
                          v-if="!showProviderScopedAutoNotice"
                          class="px-1 pt-2 text-xs text-gray-600 dark:text-slate-400"
                        >
                          {{ providerScopedAutoDescription }}
                        </div>
                        <div
                          v-if="showProviderScopedAutoNotice"
                          data-testid="routing-provider-pin-notice"
                          class="mx-1 mt-2 rounded-xl border border-sky-200/80 bg-sky-50/90 px-3 py-2 text-xs text-sky-900 dark:border-sky-900/60 dark:bg-sky-950/40 dark:text-sky-100"
                        >
                          <div class="font-semibold">
                            {{ providerScopedAutoNoticeTitle }}
                          </div>
                          <div class="mt-1 leading-5">
                            {{ providerScopedAutoNoticeDescription }}
                          </div>
                          <button
                            data-testid="routing-provider-pin-clear"
                            class="mt-2 inline-flex items-center text-[11px] font-semibold text-sky-700 transition-colors hover:text-sky-900 dark:text-sky-300 dark:hover:text-sky-100"
                            @click="setAutoModelPreference"
                          >
                            {{ providerScopedAutoResetLabel }}
                          </button>
                        </div>
                        <div v-if="showProviderScopedAutoOptions" class="space-y-1 pt-3">
                          <div
                            class="px-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500 dark:text-slate-400"
                          >
                            {{ providerScopedAutoNoticeTitle }}
                          </div>
                          <button
                            v-for="option in providerScopedAutoOptions"
                            :key="option.id"
                            :data-testid="`routing-provider-pin-row-${option.id}`"
                            class="routing-model-row w-full"
                            :class="{
                              'is-selected':
                                chatStore.providerPinOnlyActive &&
                                chatStore.selectedProviderId === option.id,
                            }"
                            @click="selectProviderScopedAuto(option.id)"
                          >
                            <span class="truncate" style="padding-inline-end: 0.75rem">
                              {{ option.label }}
                            </span>
                            <svg
                              v-if="
                                chatStore.providerPinOnlyActive &&
                                chatStore.selectedProviderId === option.id
                              "
                              class="w-4 h-4 text-sky-500 flex-shrink-0"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M5 13l4 4L19 7"
                              />
                            </svg>
                          </button>
                        </div>
                      </div>
                      <div v-else class="max-h-44 overflow-y-auto space-y-1 pt-2">
                        <button
                          v-for="option in fixedModelOptions"
                          :key="option.value"
                          class="routing-model-row w-full"
                          :class="{ 'is-selected': chatStore.modelPreference === option.value }"
                          @click="selectFixedModel(option.value)"
                        >
                          <span
                            class="truncate"
                            style="padding-inline-end: 0.75rem"
                            :title="option.value"
                            >{{ option.label }}</span
                          >
                          <svg
                            v-if="chatStore.modelPreference === option.value"
                            class="w-4 h-4 text-emerald-500 flex-shrink-0"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2.5"
                              d="M5 13l4 4L19 7"
                            />
                          </svg>
                        </button>
                      </div>

                      <router-link
                        to="/settings?tab=llm"
                        class="routing-manage-row w-full px-4 py-3 mt-1 flex items-center justify-between rounded-xl text-sm text-gray-800 dark:text-slate-200 border border-gray-200 dark:border-slate-600 bg-gray-50 dark:bg-slate-800"
                        @click="showRoutingMenu = false"
                      >
                        <span class="inline-flex items-center gap-2">
                          <svg
                            class="w-4 h-4"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M11.983 5.5a1.75 1.75 0 0 1 3.034 0l.25.433a1.75 1.75 0 0 0 1.514.875h.5a1.75 1.75 0 0 1 1.517 2.625l-.25.433a1.75 1.75 0 0 0 0 1.75l.25.433a1.75 1.75 0 0 1-1.517 2.625h-.5a1.75 1.75 0 0 0-1.514.875l-.25.433a1.75 1.75 0 0 1-3.034 0l-.25-.433a1.75 1.75 0 0 0-1.514-.875h-.5a1.75 1.75 0 0 1-1.517-2.625l.25-.433a1.75 1.75 0 0 0 0-1.75l-.25-.433a1.75 1.75 0 0 1 1.517-2.625h.5a1.75 1.75 0 0 0 1.514-.875l.25-.433Z"
                            />
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M13.5 12a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0Z"
                            />
                          </svg>
                          {{ t('chat.manageProviders') }}
                        </span>
                        <span class="text-xs text-gray-500 dark:text-slate-400">→</span>
                      </router-link>
                    </div>
                  </div>
                  <div class="h-[env(safe-area-inset-bottom)]" />
                </div>
              </div>
            </Transition>
          </Teleport>
          <Teleport to="body">
            <Transition name="sheet">
              <div
                v-if="showTopbarMenu"
                class="topbar-sheet fixed inset-0 z-[9999] flex items-end"
                data-testid="mobile-topbar-sheet"
                @click="showTopbarMenu = false"
              >
                <div class="absolute inset-0 bg-black/50" />
                <div
                  class="relative w-full bg-white dark:bg-gray-800 rounded-t-2xl shadow-2xl"
                  @click.stop
                >
                  <div class="flex justify-center pt-3 pb-2">
                    <div class="w-10 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
                  </div>
                  <div class="px-4 pb-2">
                    <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                      {{ t('chat.moreActions') }}
                    </h3>
                  </div>
                  <div class="p-4">
                    <div class="grid grid-cols-2 gap-3">
                      <button
                        data-testid="mobile-topbar-quick-action-tool-details"
                        class="quick-action-tile"
                        :class="{ 'is-active': settingsStore.showToolDetails }"
                        @click="toggleToolDetails"
                      >
                        <div class="flex items-center justify-between">
                          <svg
                            class="w-5 h-5"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="1.5"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                          >
                            <path
                              d="M12 2a5 5 0 00-4.78 3.56A3.5 3.5 0 004 9a3.5 3.5 0 00.68 2.07A3.5 3.5 0 004 13.5 3.5 3.5 0 006.5 17h.28A5 5 0 0012 20"
                            />
                            <path
                              d="M12 2a5 5 0 014.78 3.56A3.5 3.5 0 0120 9a3.5 3.5 0 01-.68 2.07A3.5 3.5 0 0120 13.5a3.5 3.5 0 01-2.5 3.5h-.28A5 5 0 0112 20"
                            />
                            <path d="M12 2v18" />
                          </svg>
                          <span class="quick-action-pill">{{
                            settingsStore.showToolDetails
                              ? t('common.enabled')
                              : t('common.disabled')
                          }}</span>
                        </div>
                        <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">
                          {{ t('chat.showToolDetails') }}
                        </div>
                      </button>

                      <button
                        v-if="showRoutingControl"
                        data-testid="mobile-topbar-quick-action-routing"
                        class="quick-action-tile"
                        :class="{ 'is-active': showRoutingMenu }"
                        :title="routingButtonTitle"
                        @click="handleRoutingMenuButtonClick"
                      >
                        <div class="flex items-center justify-between">
                          <svg
                            class="w-5 h-5"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <circle cx="7" cy="7" r="1.5" stroke-width="1.8" />
                            <circle cx="17" cy="7" r="1.5" stroke-width="1.8" />
                            <circle cx="12" cy="17" r="1.5" stroke-width="1.8" />
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="1.8"
                              d="M7 8.5v1.5A2 2 0 009 12h6a2 2 0 002-2V8.5M12 12v3.5"
                            />
                          </svg>
                          <span class="quick-action-pill">{{ routingModeInfo.label }}</span>
                        </div>
                        <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">
                          {{ t('chat.routingMode.title') }}
                        </div>
                      </button>

                      <button
                        data-testid="mobile-topbar-quick-action-deep-research"
                        class="quick-action-tile"
                        @click="openMobileFeatureSheet('deep-research')"
                      >
                        <div class="flex items-center justify-between">
                          <svg
                            class="w-5 h-5"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <circle cx="10.5" cy="10.5" r="4.75" stroke-width="1.7" />
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="1.7"
                              d="M14 14l4 4M16 5.25h3M17.5 3.75v3"
                            />
                          </svg>
                          <span class="quick-action-pill">{{
                            chatTextWithFallback('chat.alwaysOn', 'Automatic')
                          }}</span>
                        </div>
                        <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">
                          {{ deepResearchTitle }}
                        </div>
                      </button>

                      <button
                        data-testid="mobile-topbar-quick-action-smart-resume"
                        class="quick-action-tile"
                        :class="{ 'is-active': settingsStore.agentMode }"
                        @click="openMobileFeatureSheet('smart-resume')"
                      >
                        <div class="flex items-center justify-between">
                          <svg
                            class="w-5 h-5"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="1.7"
                              d="M9 4.75h6a1.75 1.75 0 011.75 1.75v10.75A1.75 1.75 0 0115 19H9a1.75 1.75 0 01-1.75-1.75V6.5A1.75 1.75 0 019 4.75z"
                            />
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="1.7"
                              d="M9.75 3h4.5M10.25 9h4M10.25 12h4M10.25 15h2.5"
                            />
                          </svg>
                          <span class="quick-action-pill">{{ smartResumeStateLabel }}</span>
                        </div>
                        <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">
                          {{ t('chat.taskLoop') }}
                        </div>
                      </button>
                    </div>
                    <div
                      v-if="showAgentcoreRunnerControl"
                      class="quick-action-tile chat-mobile-runner-card mt-4"
                      :class="agentcoreRunnerSelectedToneClass"
                    >
                      <div class="chat-mobile-runner-card__header">
                        <div class="chat-mobile-runner-card__icon" aria-hidden="true">
                          <svg
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                            stroke-width="1.8"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              d="M7 7h10M7 12h7M7 17h4"
                            />
                          </svg>
                        </div>
                        <div class="min-w-0 flex-1">
                          <div class="text-sm font-semibold text-gray-900 dark:text-white">
                            {{ t('common.version', 'Version') }}
                          </div>
                          <div class="chat-mobile-runner-card__description">
                            {{
                              t(
                                'settings.agentcoreRunner.mobileHint',
                                'Saved for this chat and switched automatically when you change sessions.'
                              )
                            }}
                          </div>
                        </div>
                        <span
                          class="quick-action-pill chat-mobile-runner-card__pill"
                          :class="agentcoreRunnerSelectedToneClass"
                        >
                          {{ agentcoreRunnerSelectedLabel }}
                        </span>
                      </div>
                      <select
                        data-testid="mobile-topbar-agentcore-runner-select"
                        class="chat-mobile-runner-select mt-3"
                        :class="agentcoreRunnerSelectedToneClass"
                        :value="agentcoreRunnerSelectValue"
                        :disabled="agentcoreRunnerSelectionBusy"
                        :aria-label="t('settings.agentcoreRunner.refLabel', 'Runner version')"
                        @change="handleAgentcoreRunnerRefChange"
                      >
                        <option
                          v-for="option in agentcoreRunnerDisplayOptions"
                          :key="`mobile-runner-ref-${option.value}`"
                          :value="option.value"
                        >
                          {{ option.label }}
                        </option>
                      </select>
                    </div>
                  </div>
                  <div class="h-[env(safe-area-inset-bottom)]" />
                </div>
              </div>
            </Transition>
          </Teleport>
          <Teleport to="body">
            <Transition name="sheet">
              <div
                v-if="mobileFeatureSheetMeta"
                class="topbar-sheet fixed inset-0 z-[10000] flex items-end"
                data-testid="mobile-feature-sheet"
                @click="closeMobileFeatureSheet"
              >
                <div class="absolute inset-0 bg-black/55" />
                <div class="mobile-feature-sheet-panel relative w-full shadow-2xl" @click.stop>
                  <div class="flex justify-center pt-3 pb-2">
                    <div class="w-10 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
                  </div>
                  <div class="mobile-feature-sheet-content px-4 pb-4">
                    <div class="mobile-feature-sheet-hero">
                      <div
                        class="mobile-feature-sheet-icon"
                        :class="`is-${mobileFeatureSheetMeta.kind}`"
                      >
                        <svg
                          v-if="mobileFeatureSheetMeta.kind === 'deep-research'"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <circle cx="10.5" cy="10.5" r="4.75" stroke-width="1.7" />
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M14 14l4 4M16 5.25h3M17.5 3.75v3"
                          />
                        </svg>
                        <svg v-else fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M9 4.75h6a1.75 1.75 0 011.75 1.75v10.75A1.75 1.75 0 0115 19H9a1.75 1.75 0 01-1.75-1.75V6.5A1.75 1.75 0 019 4.75z"
                          />
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M9.75 3h4.5M10.25 9h4M10.25 12h4M10.25 15h2.5"
                          />
                        </svg>
                      </div>
                      <div class="min-w-0 flex-1">
                        <span class="mobile-feature-sheet-state">{{
                          mobileFeatureSheetMeta.state
                        }}</span>
                        <h3 class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
                          {{ mobileFeatureSheetMeta.title }}
                        </h3>
                        <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-slate-300">
                          {{ mobileFeatureSheetMeta.description }}
                        </p>
                      </div>
                    </div>

                    <div class="mobile-feature-sheet-chips">
                      <span
                        v-for="tag in mobileFeatureSheetMeta.tags"
                        :key="tag"
                        class="mobile-feature-sheet-chip"
                      >
                        {{ tag }}
                      </span>
                    </div>

                    <div class="mobile-feature-sheet-actions">
                      <button
                        v-if="mobileFeatureSheetMeta.primaryActionLabel"
                        data-testid="mobile-feature-primary-action"
                        class="mobile-feature-sheet-primary"
                        @click="handleMobileFeaturePrimaryAction"
                      >
                        {{ mobileFeatureSheetMeta.primaryActionLabel }}
                      </button>
                      <button
                        data-testid="mobile-feature-sheet-close"
                        class="mobile-feature-sheet-secondary"
                        @click="closeMobileFeatureSheet"
                      >
                        {{ t('common.close') }}
                      </button>
                    </div>
                  </div>
                  <div class="h-[env(safe-area-inset-bottom)]" />
                </div>
              </div>
            </Transition>
          </Teleport>

          <!-- Trial Quota Banner (show when trial provider is active and quota not exhausted) -->
          <Transition name="slide-fade">
            <div
              v-if="
                isMobile &&
                providerPoolStore.trialQuota &&
                providerPoolStore.trialProviders?.length > 0 &&
                !providerPoolStore.trialQuota.is_exhausted
              "
              class="px-4 py-2 flex items-center justify-between text-sm border-b bg-gray-100 dark:bg-gray-700/20 border-gray-200 dark:border-gray-700 text-gray-700 dark:text-white"
              :class="{ 'trial-quota-pulse': tokenAnimating }"
            >
              <div class="flex items-center gap-2">
                <span :class="{ 'animate-bounce': tokenAnimating }">🎁</span>
                <span
                  class="tabular-nums transition-all duration-300 font-medium"
                  :class="{ 'token-change-animation': tokenAnimating }"
                  :title="
                    providerPoolStore.trialQuota.tokens_remaining.toLocaleString() + ' tokens'
                  "
                  >{{
                    t('chat.trialQuota.remaining', {
                      tokens: formatTokens(providerPoolStore.trialQuota.tokens_remaining),
                    })
                  }}</span
                >
              </div>
              <div class="flex items-center gap-2">
                <div
                  class="relative w-20 h-4 bg-gray-300 dark:bg-gray-700 rounded-full overflow-hidden"
                >
                  <div
                    class="h-full rounded-full transition-all duration-500 ease-out bg-gray-700 dark:bg-gray-400"
                    :style="{
                      width: `${Math.max(3, Math.min(100, (providerPoolStore.trialQuota.tokens_remaining / providerPoolStore.trialQuota.token_limit) * 100))}%`,
                    }"
                  ></div>
                  <span
                    class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-white drop-shadow-sm"
                  >
                    {{
                      Math.round(
                        (providerPoolStore.trialQuota.tokens_remaining /
                          providerPoolStore.trialQuota.token_limit) *
                          100
                      )
                    }}%
                  </span>
                </div>
                <router-link to="/settings?tab=llm" class="text-xs underline hover:no-underline">
                  {{ t('chat.trialQuota.configure') }}
                </router-link>
              </div>
            </div>
          </Transition>

          <section
            class="chat-thread-shell flex min-h-0 flex-1 flex-col"
            :class="{ 'chat-thread-shell-desktop': !isMobile }"
          >
            <div
              v-if="!isMobile"
              class="chat-thread-header flex items-center justify-between gap-4"
            >
              <div class="chat-thread-heading min-w-0">
                <h2 class="chat-thread-title text-gray-900 dark:text-white truncate">
                  {{ chatStore.currentConversation?.title || t('chat.newChat') }}
                </h2>
              </div>

              <div class="chat-thread-actions flex items-center gap-2 flex-shrink-0">
                <button
                  class="chat-thread-detail-btn inline-flex items-center gap-2 transition-colors cursor-pointer"
                  :class="{ 'is-active': settingsStore.showToolDetails }"
                  :aria-pressed="settingsStore.showToolDetails"
                  :title="
                    settingsStore.showToolDetails
                      ? t('chat.hideToolDetails')
                      : t('chat.showToolDetails')
                  "
                  @click="toggleToolDetails"
                >
                  <svg
                    class="w-4 h-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.75"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path
                      d="M12 2a5 5 0 00-4.78 3.56A3.5 3.5 0 004 9a3.5 3.5 0 00.68 2.07A3.5 3.5 0 004 13.5 3.5 3.5 0 006.5 17h.28A5 5 0 0012 20"
                    />
                    <path
                      d="M12 2a5 5 0 014.78 3.56A3.5 3.5 0 0120 9a3.5 3.5 0 01-.68 2.07A3.5 3.5 0 0120 13.5a3.5 3.5 0 01-2.5 3.5h-.28A5 5 0 0112 20"
                    />
                    <path d="M12 2v18" />
                  </svg>
                  <span>{{
                    settingsStore.showToolDetails
                      ? t('chat.hideToolDetails')
                      : t('chat.showToolDetails')
                  }}</span>
                </button>
                <label
                  v-if="showAgentcoreRunnerControl"
                  class="chat-thread-detail-btn chat-thread-runner-select inline-flex items-center transition-colors cursor-pointer"
                  :class="agentcoreRunnerSelectedToneClass"
                  :title="agentcoreRunnerButtonTitle"
                >
                  <svg
                    class="chat-thread-runner-select__icon"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.9"
                    aria-hidden="true"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M7 7h10M7 12h7M7 17h4"
                    />
                  </svg>
                  <select
                    data-testid="chat-runner-ref-select"
                    class="chat-thread-runner-select__control"
                    :value="agentcoreRunnerSelectValue"
                    :disabled="agentcoreRunnerSelectionBusy"
                    :aria-label="t('settings.agentcoreRunner.refLabel', 'Runner version')"
                    :title="agentcoreRunnerButtonTitle"
                    @change="handleAgentcoreRunnerRefChange"
                  >
                    <option
                      v-for="option in agentcoreRunnerDisplayOptions"
                      :key="`desktop-runner-ref-${option.value}`"
                      :value="option.value"
                    >
                      {{ option.label }}
                    </option>
                  </select>
                </label>
                <button
                  v-if="showRoutingControl"
                  ref="desktopRoutingMenuAnchorEl"
                  class="chat-thread-detail-btn chat-thread-routing-btn inline-flex items-center gap-2 transition-colors cursor-pointer routing-menu-anchor"
                  :class="{ 'is-active': showRoutingMenu }"
                  :title="routingButtonTitle"
                  @click.stop="handleRoutingMenuButtonClick"
                >
                  <svg
                    class="w-4 h-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.8"
                  >
                    <circle cx="7" cy="7" r="1.5" />
                    <circle cx="17" cy="7" r="1.5" />
                    <circle cx="12" cy="17" r="1.5" />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M7 8.5v1.5A2 2 0 009 12h6a2 2 0 002-2V8.5M12 12v3.5"
                    />
                  </svg>
                  <span>{{ t('chat.routingMode.title') }}</span>
                </button>
              </div>
            </div>
            <Transition name="slide-fade">
              <div
                v-if="
                  !isMobile &&
                  providerPoolStore.trialQuota &&
                  providerPoolStore.trialProviders?.length > 0 &&
                  !providerPoolStore.trialQuota.is_exhausted
                "
                class="chat-thread-subrow"
              >
                <div
                  class="chat-trial-inline flex items-center gap-3 text-sm text-gray-600 dark:text-slate-300"
                  :class="{ 'trial-quota-pulse': tokenAnimating }"
                >
                  <div class="chat-trial-inline-copy flex items-center gap-2 min-w-0">
                    <span
                      class="chat-trial-inline-gift"
                      :class="{ 'animate-bounce': tokenAnimating }"
                    >
                      🎁
                    </span>
                    <span
                      class="chat-trial-inline-text tabular-nums transition-all duration-300 font-medium truncate"
                      :class="{ 'token-change-animation': tokenAnimating }"
                      :title="
                        providerPoolStore.trialQuota.tokens_remaining.toLocaleString() + ' tokens'
                      "
                    >
                      {{
                        t('chat.trialQuota.remaining', {
                          tokens: formatTokens(providerPoolStore.trialQuota.tokens_remaining),
                        })
                      }}
                    </span>
                  </div>
                  <div class="chat-trial-inline-side flex items-center gap-2 flex-shrink-0">
                    <div class="chat-trial-inline-progress relative overflow-hidden">
                      <div
                        class="chat-trial-inline-progress-bar h-full transition-all duration-500 ease-out"
                        :style="{
                          width: `${Math.max(3, Math.min(100, (providerPoolStore.trialQuota.tokens_remaining / providerPoolStore.trialQuota.token_limit) * 100))}%`,
                        }"
                      />
                    </div>
                    <span class="chat-trial-inline-percent tabular-nums">
                      {{
                        Math.round(
                          (providerPoolStore.trialQuota.tokens_remaining /
                            providerPoolStore.trialQuota.token_limit) *
                            100
                        )
                      }}%
                    </span>
                    <router-link
                      to="/settings?tab=llm"
                      class="chat-trial-inline-link text-xs underline hover:no-underline"
                    >
                      {{ t('chat.trialQuota.configure') }}
                    </router-link>
                  </div>
                </div>
              </div>
            </Transition>

            <div
              v-if="showEdgeQuickNav"
              class="chat-edge-quick-nav-layer"
              data-testid="chat-edge-quick-nav"
            >
              <aside
                class="chat-edge-quick-nav"
                :aria-label="chatTextWithFallback('chat.quickNav.title', 'Quick navigation')"
              >
                <div class="chat-edge-quick-nav__list">
                  <button
                    v-for="item in edgeQuickNavItems"
                    :key="`edge-quick-nav-${item.messageId}`"
                    type="button"
                    class="chat-edge-quick-nav__item"
                    :class="{ 'is-active': item.messageId === activeEdgeQuickNavMessageId }"
                    :data-testid="`chat-edge-quick-nav-item-${item.messageId}`"
                    :aria-current="
                      item.messageId === activeEdgeQuickNavMessageId ? 'location' : undefined
                    "
                    :aria-label="
                      chatTextWithNamedFallback(
                        'chat.quickNav.jumpToMessage',
                        'Jump to {preview}',
                        {
                          preview: item.preview,
                        }
                      )
                    "
                    :title="item.fullText"
                    @click="handleEdgeQuickNavJump(item.messageId)"
                  >
                    <span class="chat-edge-quick-nav__text">{{ item.preview }}</span>
                    <span class="chat-edge-quick-nav__marker" aria-hidden="true" />
                  </button>
                </div>
              </aside>
            </div>

            <!-- Messages area -->
            <div
              ref="messagesContainer"
              class="flex-1 min-h-0 overflow-y-auto overscroll-contain chat-messages-area bg-surface-base px-2 sm:px-3 md:px-4"
              :class="messageAreaPaddingClass"
              @scroll.passive="handleScroll"
            >
              <!-- Load more indicator -->
              <div v-if="chatStore.loadingMore" class="flex justify-center py-4">
                <div class="flex items-center gap-2 text-gray-500 dark:text-slate-400 text-sm">
                  <div
                    class="w-4 h-4 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full animate-spin"
                  ></div>
                  {{ t('chat.loadingOlderMessages') }}
                </div>
              </div>

              <!-- Load more button -->
              <div
                v-else-if="chatStore.hasMoreMessages && chatStore.messages.length > 0"
                class="flex justify-center py-4"
              >
                <button
                  class="text-sm text-gray-900 dark:text-gray-300 hover:text-gray-700 dark:hover:text-gray-200 px-4 py-2 transition-colors cursor-pointer"
                  @click="chatStore.loadMoreMessages()"
                >
                  {{ t('chat.loadOlderMessages') }}
                </button>
              </div>

              <div
                v-if="showInitialThreadSkeleton"
                class="h-full flex items-center justify-center p-4 sm:p-6"
              >
                <div class="w-full max-w-3xl space-y-4">
                  <div class="flex justify-start">
                    <div
                      class="h-12 w-44 rounded-[1.4rem] bg-gray-200/85 dark:bg-slate-800/80 animate-pulse"
                    />
                  </div>
                  <div class="flex justify-end">
                    <div
                      class="h-16 w-64 rounded-[1.6rem] bg-sky-100/80 dark:bg-slate-800/70 animate-pulse"
                    />
                  </div>
                  <div
                    class="rounded-[1.8rem] border border-slate-200/80 bg-white/85 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/70"
                  >
                    <div class="space-y-3">
                      <div
                        class="h-4 w-28 rounded-full bg-slate-200/90 dark:bg-slate-700/80 animate-pulse"
                      />
                      <div
                        class="h-4 w-full rounded-full bg-slate-200/90 dark:bg-slate-700/80 animate-pulse"
                      />
                      <div
                        class="h-4 w-5/6 rounded-full bg-slate-200/90 dark:bg-slate-700/80 animate-pulse"
                      />
                    </div>
                  </div>
                </div>
              </div>

              <!-- Empty state -->
              <div
                v-else-if="chatStore.messages.length === 0 && !chatStore.loading"
                class="h-full flex flex-col items-center justify-center p-4"
              >
                <div class="text-center text-gray-500 dark:text-slate-400 max-w-md mb-5">
                  <img
                    src="/logo.svg"
                    alt="Logo"
                    class="w-10 h-10 sm:w-12 sm:h-12 mx-auto mb-3 opacity-80 dark:opacity-60"
                  />
                  <h3 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white mb-1">
                    {{ t('chat.startConversation') }}
                  </h3>
                  <p class="text-[13px] text-gray-500 dark:text-slate-400">
                    {{ t('chat.startConversationDesc') }}
                  </p>
                </div>

                <div
                  v-if="needsProviderAttention"
                  data-testid="chat-provider-guidance-card"
                  class="w-full max-w-xl mb-8 rounded-3xl border border-slate-200 bg-white/92 p-5 text-start shadow-sm backdrop-blur dark:border-slate-700 dark:bg-slate-900/78"
                >
                  <div class="flex items-start gap-4">
                    <div
                      class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-2xl"
                      :class="providerInlineGuidanceCopy.iconWrapperClass"
                    >
                      <svg
                        class="h-6 w-6"
                        :class="providerInlineGuidanceCopy.iconClass"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                        />
                      </svg>
                    </div>
                    <div class="min-w-0 flex-1">
                      <p
                        class="mb-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-slate-400 dark:text-slate-500"
                      >
                        {{ providerInlineGuidanceCopy.eyebrow }}
                      </p>
                      <h4 class="text-base font-semibold text-slate-900 dark:text-white">
                        {{ providerInlineGuidanceCopy.title }}
                      </h4>
                      <p class="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
                        {{ providerInlineGuidanceCopy.description }}
                      </p>
                      <div
                        v-if="
                          providerAttentionMode !== 'unconfigured' &&
                          providerAttentionItems.length > 0
                        "
                        class="mt-4 rounded-2xl border border-slate-200 bg-slate-50/90 p-3 dark:border-slate-700 dark:bg-slate-950/50"
                      >
                        <div
                          class="mb-2 text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400 dark:text-slate-500"
                        >
                          {{
                            chatTextWithFallback(
                              'chat.noProvider.providerSummary',
                              'Configured providers needing attention'
                            )
                          }}
                        </div>
                        <div class="space-y-2">
                          <div
                            v-for="item in providerAttentionItems"
                            :key="item.id"
                            class="rounded-xl border border-slate-200/80 bg-white px-3 py-2.5 dark:border-slate-700 dark:bg-slate-900/70"
                          >
                            <div class="flex items-center justify-between gap-3">
                              <div
                                class="min-w-0 text-sm font-medium text-slate-900 dark:text-white"
                              >
                                {{ item.name }}
                              </div>
                              <span
                                class="flex-shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.16em]"
                                :class="
                                  item.status === 'error'
                                    ? 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-200'
                                    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
                                "
                              >
                                {{ item.statusLabel }}
                              </span>
                            </div>
                            <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">
                              {{ item.reason }}
                            </p>
                          </div>
                        </div>
                        <p
                          v-if="providerAttentionOverflowCount > 0"
                          class="mt-2 text-xs text-slate-500 dark:text-slate-400"
                        >
                          {{
                            chatTextWithFallback(
                              'chat.noProvider.moreProviders',
                              '{count} more providers also need attention.'
                            ).replace('{count}', String(providerAttentionOverflowCount))
                          }}
                        </p>
                      </div>
                      <div class="mt-4 flex flex-wrap gap-3">
                        <button
                          v-if="providerAttentionMode === 'recoverable'"
                          data-testid="chat-provider-guidance-retry"
                          class="inline-flex items-center justify-center rounded-xl bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-slate-700 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200"
                          @click="handleProviderGuidanceRetry"
                        >
                          {{ providerInlineGuidanceCopy.primaryAction }}
                        </button>
                        <button
                          :data-testid="
                            providerAttentionMode === 'recoverable'
                              ? 'chat-provider-guidance-review'
                              : 'chat-provider-guidance-action'
                          "
                          :class="
                            providerAttentionMode === 'recoverable'
                              ? 'inline-flex items-center justify-center rounded-xl border border-slate-300 bg-white px-4 py-2.5 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-100 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800'
                              : 'inline-flex items-center justify-center rounded-xl bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-slate-700 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200'
                          "
                          @click="
                            handleOpenProviderSettings(
                              recoverableAttentionProvider?.id || providerAttentionItems[0]?.id
                            )
                          "
                        >
                          {{
                            providerAttentionMode === 'recoverable'
                              ? providerInlineGuidanceCopy.secondaryAction
                              : providerInlineGuidanceCopy.primaryAction
                          }}
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Preset Questions -->
                <PresetQuestions
                  v-if="!needsProviderAttention"
                  :context-text="presetQuestionContextText"
                  @select="handlePresetQuestionSelect"
                />

                <div
                  v-if="!isMobile"
                  class="mt-5 text-[11px] text-gray-400 dark:text-slate-500 text-center"
                >
                  <p class="font-medium mb-2">{{ t('chat.keyboardShortcuts') }}:</p>
                  <p class="space-x-4">
                    <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">{{
                      isMac ? '⌘N' : 'Alt+N'
                    }}</span>
                    {{ t('chat.newChatShortcut') }}
                    <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">/</span>
                    {{ t('chat.focusInputShortcut') }}
                    <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">{{
                      isMac ? '⌘B' : 'Alt+B'
                    }}</span>
                    {{ t('chat.toggleSidebarShortcut') }}
                  </p>
                  <p class="mt-3 text-gray-400 dark:text-slate-500">
                    <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300"
                      >Enter</span
                    >
                    {{ t('chat.sendMessage') }}
                    <span
                      class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300"
                      style="margin-inline-start: 1rem"
                      >Shift+Enter</span
                    >
                    {{ t('chat.newLine') }}
                  </p>
                  <p class="mt-2 text-gray-400 dark:text-slate-500">
                    {{ t('chat.dragDropHint') }}
                  </p>
                </div>
              </div>

              <!-- Messages list - Virtual scroll for large lists -->
              <template v-if="chatStore.messages.length > 0">
                <div v-if="runnerExecutionCard" class="chat-message-shell">
                  <TypelessCardComponent :card="runnerExecutionCard" />
                </div>

                <!-- Use virtual scroll for large message lists -->
                <VirtualScroll
                  v-if="useVirtualScroll"
                  ref="virtualScrollRef"
                  :item-count="chatStore.messages.length"
                  :items="chatStore.messages"
                  :item-key="getMessageRenderKey"
                  :estimated-item-height="120"
                  :overscan="virtualScrollOverscan"
                  :scroll-container="messagesContainer"
                  class="pb-4"
                  @visible-range-change="handleVisibleRangeChange"
                >
                  <template #default="{ item: message, updateHeight }">
                    <div
                      v-if="message"
                      :key="getMessageRenderKey(message)"
                      :ref="bindVirtualItemHeight(message, updateHeight)"
                      :id="getChatMessageElementId(message.id)"
                      :data-message-id="message.id"
                      :class="messageShellClasses(message)"
                    >
                      <ChatMessage
                        v-memo="messageMemoDeps(message)"
                        :message="message"
                        v-bind="messageRenderBindings(message)"
                        @contextmenu="handleMessageContextMenu"
                        @continue="handleMessageContinue"
                        @regenerate="handleMessageRegenerate"
                        @edit-resubmit="handleMessageEditResubmit"
                      />
                    </div>
                  </template>
                </VirtualScroll>

                <!-- Regular rendering for small lists -->
                <div v-else class="pb-4">
                  <div
                    v-for="message in chatStore.messages"
                    :key="getMessageRenderKey(message)"
                    :id="getChatMessageElementId(message.id)"
                    :data-message-id="message.id"
                    :class="messageShellClasses(message)"
                  >
                    <ChatMessage
                      v-memo="messageMemoDeps(message)"
                      :message="message"
                      v-bind="messageRenderBindings(message)"
                      @contextmenu="handleMessageContextMenu"
                      @continue="handleMessageContinue"
                      @regenerate="handleMessageRegenerate"
                      @edit-resubmit="handleMessageEditResubmit"
                    />
                  </div>
                </div>

                <!-- Current conversation task projections -->
                <div v-if="taskProjections.currentTasks.length > 0" class="px-4">
                  <UserTaskProjectionCard
                    v-for="task in taskProjections.currentTasks"
                    :key="task.id"
                    :task="task"
                    :collapse-by-default="true"
                    @action="performProjectedTaskAction"
                    @open="openProjectedTask"
                    @navigate="navigateProjectedTask"
                  />
                </div>

                <Transition name="fade">
                  <div v-if="showStreamStatusRail" class="flex justify-center py-2">
                    <div class="chat-stream-status-rail">
                      <div class="chat-stream-status-rail__copy">
                        <span class="chat-stream-status-rail__badge">
                          {{ streamStatusRailPhaseLabel }}
                        </span>
                        <span class="chat-stream-status-rail__label">
                          {{ streamStatusRailState.label || t('chat.waitingThinking') }}
                        </span>
                        <span
                          v-if="streamStatusRailState.detail"
                          class="chat-stream-status-rail__detail"
                        >
                          {{ streamStatusRailState.detail }}
                        </span>
                      </div>
                      <div class="chat-stream-status-rail__actions">
                        <button
                          v-if="
                            streamStatusRailState.phase === 'interrupted' &&
                            streamStatusRailState.canRetry
                          "
                          class="chat-stream-status-rail__action"
                          @click="handleStreamRetry"
                        >
                          {{ t('common.retry') }}
                        </button>
                        <button
                          v-if="chatStore.isRecovering"
                          class="chat-stream-status-rail__action is-danger"
                          @click="handleCancel"
                        >
                          {{ t('chat.stopGenerating') }}
                        </button>
                      </div>
                    </div>
                  </div>
                </Transition>

                <!-- Context trim indicator (pruning/compaction) -->
                <Transition name="fade">
                  <div v-if="showContextTrim" class="flex justify-center py-2">
                    <div
                      class="flex items-center gap-2 px-3 py-1.5 text-xs text-gray-400/70 dark:text-gray-500/70 bg-gray-100/30 dark:bg-gray-800/30 rounded-full"
                    >
                      <svg
                        class="w-3.5 h-3.5 flex-shrink-0"
                        :class="{
                          'animate-pulse': chatStore.contextTrimInfo?.type === 'compacting',
                        }"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M14.121 14.121L19 19m-7-7l7-7m-7 7l-2.879 2.879M12 12L9.121 9.121m0 5.758a3 3 0 10-4.243 4.243 3 3 0 004.243-4.243zm0-5.758a3 3 0 10-4.243-4.243 3 3 0 004.243 4.243z"
                        />
                      </svg>
                      <span v-if="chatStore.contextTrimInfo?.type === 'compacting'">
                        {{ t('chat.contextCompacting') }}
                      </span>
                      <span v-else-if="chatStore.contextTrimInfo?.type === 'pruned'">
                        {{
                          t('chat.contextPruned', {
                            tokens: formatTokens(
                              (chatStore.contextTrimInfo.tokensBefore ?? 0) -
                                (chatStore.contextTrimInfo.tokensAfter ?? 0)
                            ),
                          })
                        }}
                      </span>
                      <span v-else-if="chatStore.contextTrimInfo?.type === 'compacted'">
                        {{
                          t('chat.contextCompacted', {
                            before: chatStore.contextTrimInfo.before ?? 0,
                            after: chatStore.contextTrimInfo.after ?? 0,
                          })
                        }}
                      </span>
                    </div>
                  </div>
                </Transition>

                <!-- Stream error display (shown in chat area with gray text) -->
                <div v-if="chatStore.streamError" class="flex justify-center py-4">
                  <div
                    class="flex items-center gap-2 px-4 py-2 text-gray-400 dark:text-gray-500 text-sm"
                  >
                    <svg
                      class="w-4 h-4 flex-shrink-0"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                      />
                    </svg>
                    <span class="break-all">{{
                      chatStore.streamError === 'streamEmpty'
                        ? t('chat.streamEmpty')
                        : chatStore.streamError === 'streamError'
                          ? t('chat.streamError')
                          : chatStore.streamError === 'contextWindowExceeded'
                            ? t('chat.contextWindowExceeded')
                            : chatStore.streamError === 'requestBuildFailed'
                              ? t('chat.requestBuildFailed')
                              : chatStore.streamError === 'requestTooLarge'
                                ? t('chat.requestTooLarge')
                                : chatStore.streamError === 'providerNoResponse'
                                  ? t('chat.providerNoResponse')
                                  : chatStore.streamError === 'providerReturnedEmpty'
                                    ? t('chat.providerReturnedEmpty')
                                    : chatStore.streamError === 'noResponseBody'
                                      ? t('chat.noResponseBody')
                                      : chatStore.streamError === 'trial_service_busy'
                                        ? t('chat.trialServiceBusy')
                                        : chatStore.streamError === 'provider_tool_unsupported'
                                          ? t('chat.providerToolUnsupported')
                                          : chatStore.streamError === 'provider_unavailable'
                                            ? t('chat.providerUnavailable')
                                            : chatStore.streamError === 'provider_auth_error'
                                              ? t('chat.providerAuthError')
                                              : chatStore.streamError === 'provider_rate_limited'
                                                ? t('chat.providerRateLimited')
                                                : chatStore.streamError ===
                                                    'provider_openrouter_privacy_policy'
                                                  ? t('chat.providerOpenRouterPrivacyPolicy')
                                                  : chatStore.streamError ===
                                                      'execDirectoryApprovalTimeout'
                                                    ? t('chat.execDirectoryApprovalTimeout')
                                                    : chatStore.streamError
                    }}</span>
                    <button
                      class="px-2 py-0.5 rounded text-gray-400 hover:text-blue-400 hover:bg-blue-500/10 cursor-pointer transition-colors text-xs"
                      style="margin-inline-start: 0.25rem"
                      @click="handleStreamRetry"
                    >
                      {{ t('common.retry') }}
                    </button>
                    <button
                      class="text-gray-400 hover:text-gray-300 cursor-pointer"
                      @click="chatStore.clearStreamError"
                    >
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
                <!-- Awaiting-user-input indicator -->
                <div
                  v-if="showAwaitingConfirmation && !chatStore.streaming"
                  class="flex justify-center py-2"
                >
                  <div
                    class="flex items-center gap-2 px-3 py-1.5 text-xs text-amber-500 dark:text-amber-300 bg-amber-500/10 rounded-full"
                  >
                    <svg
                      class="w-3.5 h-3.5 flex-shrink-0"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 8v4m0 4h.01M7 4h10l3 5v11H4V9l3-5z"
                      />
                    </svg>
                    <span>{{
                      t('chat.awaitingConfirmation', 'Waiting for your confirmation to continue')
                    }}</span>
                  </div>
                </div>

                <!-- "I'm listening" indicator (shown when pre-TTFT cancel is active) -->
                <div v-if="chatStore.preTTFTCancelActive" class="flex justify-center py-3">
                  <div
                    class="flex items-center gap-2 px-4 py-2 glass-card rounded-lg text-sm text-blue-400"
                  >
                    <span class="flex gap-1">
                      <span
                        class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce"
                        style="animation-delay: 0ms"
                      />
                      <span
                        class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce"
                        style="animation-delay: 150ms"
                      />
                      <span
                        class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce"
                        style="animation-delay: 300ms"
                      />
                    </span>
                    {{ t('chat.stillListening') }}
                  </div>
                </div>
              </template>
            </div>

            <!-- Context menu -->
            <Teleport to="body">
              <!-- Desktop: Floating menu -->
              <div
                v-if="showContextMenu && !isMobile"
                class="context-menu fixed z-[100] glass-card shadow-xl py-1 min-w-[160px]"
                :style="{
                  /* rtl-audit-ignore-next-line: positioned from pointer coordinates */
                  left: `${contextMenuPosition.x}px`,
                  top: `${contextMenuPosition.y}px`,
                }"
              >
                <button
                  class="w-full px-4 py-2 text-start text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
                  @click="handleContextCopy"
                >
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                    />
                  </svg>
                  {{ t('chat.copyMessage') }}
                </button>
                <button
                  class="w-full px-4 py-2 text-start text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
                  @click="handleSelectMessage"
                >
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                    />
                  </svg>
                  {{ t('chat.selectMessage') }}
                </button>
                <!-- Continue / Regenerate — only for last assistant message, not while streaming -->
                <template v-if="isContextMenuLastAssistant && !chatStore.streaming">
                  <div class="border-t border-white/10 my-1" />
                  <button
                    class="w-full px-4 py-2 text-start text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
                    @click="handleContextContinue"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
                      />
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                    {{ t('chat.continueGenerating') }}
                  </button>
                  <button
                    class="w-full px-4 py-2 text-start text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
                    @click="handleContextRegenerate"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                      />
                    </svg>
                    {{ t('chat.regenerate') }}
                  </button>
                </template>
              </div>
            </Teleport>

            <!-- Multi-select action bar -->
            <Transition name="slide-up">
              <div
                v-if="chatStore.isMultiSelectMode"
                class="absolute bottom-20 start-1/2 -translate-x-1/2 z-20 glass-card shadow-xl px-4 py-3 flex items-center gap-4"
              >
                <span class="text-sm text-gray-600 dark:text-gray-300">
                  {{ chatStore.selectedMessageIds.size }} {{ t('chat.messagesSelected') }}
                </span>
                <div class="flex items-center gap-2">
                  <button
                    class="px-3 py-1.5 text-sm bg-red-500/20 hover:bg-red-500/30 text-red-400 rounded-lg flex items-center gap-1.5 transition-colors cursor-pointer"
                    :disabled="chatStore.selectedMessageIds.size === 0"
                    @click="handleDeleteSelectedMessages"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                      />
                    </svg>
                    {{ t('common.delete') }}
                  </button>
                  <button
                    class="px-3 py-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 rounded-lg transition-colors cursor-pointer"
                    @click="handleCancelSelection"
                  >
                    {{ t('common.cancel') }}
                  </button>
                </div>
              </div>
            </Transition>

            <!-- Error message -->
            <div
              v-if="chatStore.error"
              class="px-3 sm:px-4 py-3 bg-red-500/10 border-t border-red-500/30 text-red-400 text-xs sm:text-sm flex items-center justify-between gap-2"
            >
              <span class="truncate">{{
                chatStore.error === 'trial_service_busy'
                  ? t('chat.trialServiceBusy')
                  : chatStore.error === 'provider_tool_unsupported'
                    ? t('chat.providerToolUnsupported')
                    : chatStore.error === 'provider_unavailable'
                      ? t('chat.providerUnavailable')
                      : chatStore.error === 'provider_auth_error'
                        ? t('chat.providerAuthError')
                        : chatStore.error === 'provider_rate_limited'
                          ? t('chat.providerRateLimited')
                          : chatStore.error === 'provider_openrouter_privacy_policy'
                            ? t('chat.providerOpenRouterPrivacyPolicy')
                            : chatStore.error === 'execDirectoryApprovalTimeout'
                              ? t('chat.execDirectoryApprovalTimeout')
                              : chatStore.error
              }}</span>
              <button
                class="text-red-400 hover:text-red-300 flex-shrink-0 px-3 py-1 rounded hover:bg-red-500/10 transition-colors cursor-pointer"
                @click="chatStore.clearError"
              >
                {{ t('chat.dismiss') }}
              </button>
            </div>

            <!-- Security blocked warning -->
            <div
              v-if="chatStore.securityBlocked"
              class="px-3 sm:px-4 py-3 bg-yellow-500/10 border-t border-yellow-500/30 text-yellow-400 text-xs sm:text-sm flex items-center justify-between gap-2"
            >
              <div class="flex items-center gap-2">
                <svg
                  class="w-4 h-4 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <span class="truncate">{{ chatStore.securityBlocked.message }}</span>
                <span class="text-yellow-500/70 text-xs"
                  >({{ t('chat.threatLevel') }}: {{ chatStore.securityBlocked.threatLevel }})</span
                >
              </div>
              <button
                class="text-yellow-400 hover:text-yellow-300 flex-shrink-0 px-3 py-1 rounded hover:bg-yellow-500/10 transition-colors cursor-pointer"
                @click="chatStore.clearSecurityBlocked"
              >
                {{ t('chat.dismissWarning') }}
              </button>
            </div>

            <!-- Trial Exhausted Banner -->
            <div
              v-if="chatStore.trialExhausted"
              class="px-3 sm:px-4 py-3 bg-gray-500/10 border-t border-gray-500/30 text-gray-400 text-xs sm:text-sm flex items-center justify-between gap-2"
            >
              <div class="flex items-center gap-2">
                <svg
                  class="w-4 h-4 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <span>{{ t('chat.trialExhausted') }}</span>
              </div>
              <router-link
                to="/settings?tab=llm"
                class="text-gray-900 dark:text-white hover:text-gray-900 dark:text-white flex-shrink-0 px-3 py-1 rounded hover:bg-gray-100 dark:bg-gray-600/10 transition-colors"
              >
                {{ t('chat.configureProvider') }}
              </router-link>
            </div>

            <!-- Input area - floating at bottom (desktop), flex at bottom (mobile) -->
            <div class="chat-input-dock flex-shrink-0">
              <div v-if="hasBackgroundTasks" class="max-w-5xl mx-auto px-3 sm:px-4 py-1">
                <UserTaskProjectionDock
                  :tasks="taskProjections.backgroundTasks"
                  @open="openProjectedTask"
                  @action="performProjectedTaskAction"
                  @navigate="navigateProjectedTask"
                />
              </div>
              <div v-if="hasActiveDeepResearchJobs" class="max-w-5xl mx-auto px-3 sm:px-4 py-1">
                <DeepResearchTaskDock @view="handleOpenDeepResearchJob" />
              </div>
              <!-- Media generation param panel -->
              <div v-if="mediaGen.showPanel.value" class="max-w-4xl mx-auto px-3 sm:px-4">
                <MediaParamPanel
                  :intent="mediaGen.intent.value!"
                  :models="mediaGen.models.value"
                  :selected-model="mediaGen.selectedModel.value"
                  :generating="mediaGen.generating.value"
                  :ambiguous="mediaGen.ambiguous.value"
                  @update:selected-model="mediaGen.selectedModel.value = $event"
                  @generate="handleMediaGenerate"
                  @dismiss="handleMediaDismiss"
                  @close="handleMediaClose"
                  @confirm="handleMediaConfirm"
                  @switch-category="mediaGen.switchCategory($event)"
                />
              </div>
              <div
                v-if="activeTodoSummary"
                class="active-todo-panel-wrap w-full max-w-3xl mx-auto px-2.5 sm:px-3.5"
                data-testid="active-todo-panel"
              >
                <section
                  class="active-todo-panel"
                  :class="{
                    'is-complete': activeTodoSummary.allCompleted,
                    'is-expanded': !activeTodoPanelCollapsed,
                  }"
                  aria-live="polite"
                >
                  <div class="active-todo-panel__header">
                    <button
                      type="button"
                      class="active-todo-panel__summary active-todo-panel__summary--interactive"
                      :title="activeTodoPanelJumpTitle"
                      data-testid="active-todo-panel-jump"
                      @click="handleActiveTodoPanelJump"
                    >
                      <span class="active-todo-panel__icon" aria-hidden="true">
                        <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.8"
                            d="M8.75 6.75h10.5M8.75 12h10.5m-10.5 5.25h10.5M4.75 6.75h.01M4.75 12h.01M4.75 17.25h.01"
                          />
                        </svg>
                      </span>
                      <span class="active-todo-panel__progress">{{ activeTodoProgressText }}</span>
                    </button>
                    <div class="active-todo-panel__header-actions">
                      <button
                        type="button"
                        class="active-todo-panel__icon-btn"
                        :title="activeTodoPanelToggleTitle"
                        data-testid="active-todo-panel-toggle"
                        @click="toggleActiveTodoPanel"
                      >
                        <svg
                          class="active-todo-panel__toggle-icon"
                          :class="{ 'is-collapsed': activeTodoPanelCollapsed }"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M6 9l6 6 6-6"
                          />
                        </svg>
                      </button>
                    </div>
                  </div>
                  <ol v-if="!activeTodoPanelCollapsed" class="active-todo-panel__list">
                    <li
                      v-for="(item, index) in activeTodoSummary.items"
                      :key="`${activeTodoSummary.messageId}-${index}`"
                      class="active-todo-panel__item"
                      :class="{ 'is-checked': item.checked }"
                    >
                      <span
                        class="active-todo-panel__check"
                        :class="{ 'is-checked': item.checked }"
                        aria-hidden="true"
                      >
                        <svg
                          v-if="item.checked"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2.4"
                            d="M5 12.5l4.2 4.2L19 7.5"
                          />
                        </svg>
                      </span>
                      <span class="active-todo-panel__index">{{ index + 1 }}.</span>
                      <span class="active-todo-panel__text">{{ item.text }}</span>
                    </li>
                  </ol>
                </section>
              </div>
              <ChatInput
                ref="chatInputRef"
                :disabled="chatInputDisabled"
                :streaming="chatStore.streaming"
                :can-cancel="hasCancelableWork"
                :conversation-id="chatStore.currentConversationId || undefined"
                @send="handleSend"
                @draft-change="handleDraftChange"
                @inject="handleInject"
                @cancel="handleCancel"
                @cancel-pre-ttft="chatStore.cancelPreTTFT()"
                @warmup="chatStore.warmupConversation()"
                @open-talk-mode="showTalkMode = true"
              />
            </div>
          </section>

          <!-- Talk Mode -->
          <TalkMode
            v-if="showTalkMode"
            v-model="showTalkMode"
            :conversation-id="chatStore.currentConversationId || undefined"
            @transcript="handleVoiceTranscript"
          />
        </main>
      </div>
    </div>

    <!-- Tool call approval dialog -->
    <ToolApprovalDialog v-if="chatStore.pendingApproval" />

    <!-- Exec directory approval dialog -->
    <ExecApprovalDialog v-if="chatStore.pendingExecApproval" />

    <!-- Fixed-model unavailable dialog -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="modelAutoFallbackDialogState"
          role="dialog"
          aria-modal="true"
          aria-labelledby="model-auto-fallback-dialog-title"
          class="fixed inset-0 z-[10000] flex items-center justify-center bg-black/40 backdrop-blur-sm"
          @click.self="dismissModelAutoFallbackDialog"
        >
          <div
            class="w-full max-w-md mx-4 rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden"
          >
            <div class="p-6 text-center">
              <p
                class="mb-3 text-[11px] font-semibold uppercase tracking-[0.24em] text-amber-500 dark:text-amber-300"
              >
                {{ chatTextWithFallback('chat.modelFallback.eyebrow', 'Fixed model unavailable') }}
              </p>
              <div
                class="w-14 h-14 mx-auto mb-4 rounded-full flex items-center justify-center bg-amber-100 dark:bg-amber-900/30"
              >
                <svg
                  class="w-7 h-7 text-amber-600 dark:text-amber-300"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
              </div>
              <h3
                id="model-auto-fallback-dialog-title"
                class="text-lg font-semibold text-gray-900 dark:text-white mb-2"
              >
                {{
                  chatTextWithFallback(
                    'chat.modelFallback.title',
                    'Switch to auto routing and retry?'
                  )
                }}
              </h3>
              <p class="text-sm leading-6 text-gray-500 dark:text-gray-400 mb-3">
                {{
                  chatTextWithNamedFallback(
                    'chat.modelFallback.description',
                    'The fixed model "{model}" is no longer available. Switch this chat back to auto routing and retry your last request?',
                    { model: modelAutoFallbackTargetLabel }
                  )
                }}
              </p>
              <p
                class="mb-6 rounded-xl border border-slate-200/80 bg-slate-50 px-4 py-3 text-xs leading-5 text-slate-600 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-300 break-all"
              >
                {{ modelAutoFallbackTargetLabel }}
              </p>
              <div class="flex gap-3">
                <button
                  class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors cursor-pointer"
                  @click="dismissModelAutoFallbackDialog"
                >
                  {{
                    chatTextWithFallback('chat.modelFallback.secondaryAction', 'Keep current model')
                  }}
                </button>
                <button
                  class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors cursor-pointer"
                  @click="confirmModelAutoFallbackDialog"
                >
                  {{ chatTextWithFallback('chat.modelFallback.primaryAction', 'Switch and retry') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <TaskActionDialog
      :open="!!pendingTaskActionDialog"
      :action="pendingTaskActionDialog?.action || null"
      :submitting="taskActionDialogSubmitting"
      :submit-error="taskActionDialogError"
      @close="closeTaskActionDialog()"
      @confirm="confirmTaskActionDialog"
    />

    <!-- Provider config required dialog -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="showProviderConfigDialog"
          role="dialog"
          aria-modal="true"
          aria-labelledby="provider-config-dialog-title"
          class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 backdrop-blur-sm"
          @click.self="closeProviderConfigDialog"
        >
          <div
            class="w-full max-w-md mx-4 rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden"
          >
            <div class="p-6 text-center">
              <p
                class="mb-3 text-[11px] font-semibold uppercase tracking-[0.24em] text-gray-400 dark:text-gray-500"
              >
                {{ providerConfigDialogCopy.eyebrow }}
              </p>
              <div
                class="w-14 h-14 mx-auto mb-4 rounded-full flex items-center justify-center"
                :class="providerConfigDialogCopy.iconWrapperClass"
              >
                <svg
                  class="w-7 h-7"
                  :class="providerConfigDialogCopy.iconClass"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
              </div>
              <h3
                id="provider-config-dialog-title"
                class="text-lg font-semibold text-gray-900 dark:text-white mb-2"
              >
                {{ providerConfigDialogCopy.title }}
              </h3>
              <p class="text-sm leading-6 text-gray-500 dark:text-gray-400 mb-5">
                {{ providerConfigDialogCopy.description }}
              </p>
              <p
                v-if="providerConfigDialogHasDraft"
                class="mb-6 rounded-xl border border-slate-200/80 bg-slate-50 px-4 py-3 text-xs leading-5 text-slate-600 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-300"
              >
                {{
                  chatTextWithFallback(
                    'chat.noProvider.draftSaved',
                    'Your draft has been kept locally so you can continue after setup.'
                  )
                }}
              </p>
              <div class="flex gap-3">
                <button
                  class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors cursor-pointer"
                  @click="closeProviderConfigDialog"
                >
                  {{ providerConfigDialogCopy.secondaryAction }}
                </button>
                <button
                  class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors cursor-pointer"
                  @click="handleOpenProviderSettings()"
                >
                  {{ providerConfigDialogCopy.primaryAction }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.chat-view {
  --chat-pane-pad-x: 1rem;
  --chat-pane-pad-y: 0.96rem;
  --chat-thread-pad-x: 0.82rem;
  --chat-header-row-height: 3.22rem;
  --chat-header-row-pad-y: 0.44rem;
  --chat-header-control-size: 1.96rem;
  --chat-header-row-block-size: calc(
    var(--chat-header-row-height) + (var(--chat-header-row-pad-y) * 2)
  );
  --ct-border-soft: rgba(148, 163, 184, 0.3);
  --ct-border-strong: rgba(59, 130, 246, 0.45);
  --ct-chip-bg: rgba(255, 255, 255, 0.84);
  --ui-gap: 0.34rem;
  --ui-chip-h: 1.7rem;
  --ui-icon-size: 1.72rem;
  --ui-radius: 0.4rem;
  min-height: 0;
  flex: 1;
  height: 100%;
  overflow: hidden;
  background: transparent;
}

.chat-view.ui-density-compact {
  --chat-pane-pad-x: 0.82rem;
  --chat-pane-pad-y: 0.78rem;
  --chat-thread-pad-x: 0.72rem;
  --ui-gap: 0.35rem;
  --ui-chip-h: 1.58rem;
  --ui-icon-size: 1.6rem;
  --ui-radius: 0.36rem;
}

.chat-view.ui-density-comfortable {
  --chat-pane-pad-x: 1.12rem;
  --chat-pane-pad-y: 1.02rem;
  --chat-thread-pad-x: 0.92rem;
  --ui-gap: 0.65rem;
  --ui-chip-h: 2.2rem;
  --ui-icon-size: 2.2rem;
  --ui-radius: 0.62rem;
}

.chat-view::before {
  content: none;
}

:root.dark .chat-view,
[data-theme='dark'] .chat-view {
  --ct-border-soft: rgba(71, 85, 105, 0.42);
  --ct-border-strong: rgba(56, 189, 248, 0.56);
  --ct-chip-bg: rgba(15, 23, 42, 0.48);
  background: transparent;
}

:root.dark .chat-view::before,
[data-theme='dark'] .chat-view::before {
  content: none;
}

.conversation-sidebar,
.chat-messages-area,
header,
.chat-input-wrapper {
  position: relative;
  z-index: 1;
}

.chat-desktop-shell {
  height: 100%;
}

.chat-workspace {
  min-height: 0;
  min-width: 0;
  width: 100%;
}

.chat-desktop-shell .chat-workspace {
  border: none;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  overflow: hidden;
}

.chat-body-shell {
  min-height: 0;
  min-width: 0;
  width: 100%;
  position: relative;
  overflow-x: hidden;
}

.chat-desktop-shell .chat-body-shell {
  padding-bottom: 0;
}

.chat-main-shell {
  background: transparent;
}

.chat-sidebar-shell {
  background: transparent;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.chat-desktop-shell .chat-sidebar-shell {
  border-inline-end: 1px solid rgba(226, 232, 240, 0.92);
  background: transparent;
}

.chat-page-header {
  position: relative;
  z-index: 3;
  padding-block: 0;
  padding-inline: 0.96rem 0.9rem;
  border-bottom: none;
  background: transparent;
}

.chat-page-header::after {
  content: none;
}

.chat-page-header-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 2.5rem;
}

.chat-page-heading {
  display: inline-flex;
  align-items: center;
  gap: 0.8rem;
  min-width: 0;
}

.chat-page-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
}

.chat-page-title {
  font-size: clamp(0.96rem, 0.92vw, 1.12rem);
  line-height: 1.08;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.chat-thread-shell {
  position: relative;
}

.chat-thread-shell-desktop {
  overflow: hidden;
  background: transparent;
  border: none;
  border-radius: 0;
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

html[data-blue-macos-glass='true'] .chat-desktop-shell .chat-workspace {
  border: none;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

html[data-blue-macos-glass='true'] .chat-desktop-shell .chat-sidebar-shell {
  border-inline-end-color: rgba(186, 203, 223, 0.48);
}

html[data-blue-macos-glass='true'] .chat-page-header {
  padding-block: 0;
  padding-inline: 0.88rem 0.82rem;
}

html[data-blue-macos-glass='true'] .chat-page-header-inner {
  height: 2.12rem;
}

html[data-blue-macos-glass='true'] .chat-page-title {
  font-size: clamp(0.9rem, 0.88vw, 1.02rem);
}

html[data-blue-macos-glass='true'] .chat-desktop-shell .chat-thread-shell-desktop {
  padding-top: 0;
}

html[data-blue-macos-glass='true'] .chat-desktop-shell .chat-main-shell {
  height: auto;
}

.chat-thread-header {
  box-sizing: border-box;
  height: var(--chat-header-row-block-size);
  min-height: var(--chat-header-row-block-size);
  padding: var(--chat-header-row-pad-y) var(--chat-thread-pad-x);
  border: none;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: inset 0 -1px 0 rgba(226, 232, 240, 0.92);
}

.chat-thread-heading {
  display: flex;
  align-items: center;
  min-width: 0;
  flex: 1;
}

.chat-thread-subrow {
  margin-top: -1px;
  padding: 0.38rem var(--chat-thread-pad-x);
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 0;
  background: rgba(248, 250, 252, 0.8);
}

.chat-thread-title {
  font-size: clamp(0.98rem, 1.02vw, 1.2rem);
  line-height: 1.08;
  font-weight: 720;
  letter-spacing: -0.028em;
}

.chat-trial-inline {
  min-height: 0;
  flex-wrap: wrap;
}

.chat-thread-subrow .chat-trial-inline {
  width: 100%;
  justify-content: space-between;
  gap: 0.4rem 0.85rem;
}

.chat-trial-inline-copy {
  min-width: 0;
}

.chat-trial-inline-gift {
  flex-shrink: 0;
}

.chat-trial-inline-text {
  font-size: 0.75rem;
  line-height: 1.15;
}

.chat-trial-inline-side {
  min-width: 0;
}

.chat-trial-inline-progress {
  width: 4.6rem;
  height: 0.28rem;
  border-radius: 999px;
  background: rgba(226, 232, 240, 0.94);
}

.chat-trial-inline-progress-bar {
  border-radius: inherit;
  background: rgb(37, 99, 235);
}

.chat-trial-inline-percent {
  font-size: 0.72rem;
  color: rgb(148, 163, 184);
}

.chat-trial-inline-link {
  color: rgb(100, 116, 139);
}

.chat-thread-actions {
  flex-wrap: nowrap;
  justify-content: flex-end;
  gap: 0.62rem;
}

.chat-thread-runner-select {
  position: relative;
  box-sizing: border-box;
  min-height: var(--chat-header-control-size);
  min-width: 8.4rem;
  gap: 0.42rem;
  padding: 0 0.8rem;
  white-space: nowrap;
}

.chat-thread-runner-select__icon {
  width: 0.92rem;
  height: 0.92rem;
  flex-shrink: 0;
  color: currentColor;
  opacity: 0.9;
  pointer-events: none;
}

.chat-thread-runner-select__control,
.chat-mobile-runner-select {
  min-width: 0;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 0.9rem;
  background: rgba(255, 255, 255, 0.96);
  color: rgb(15, 23, 42);
  padding: 0.55rem 2rem 0.55rem 0.8rem;
  font-size: 0.78rem;
  font-weight: 600;
}

.chat-thread-runner-select__control {
  border: none;
  border-radius: 0;
  background: transparent;
  background-image: none;
  box-shadow: none;
  flex: 1 1 auto;
  width: 100%;
  color: inherit;
  padding: 0;
  margin: 0;
  min-width: 0;
  font: inherit;
  line-height: inherit;
  letter-spacing: inherit;
  outline: none;
  cursor: pointer;
  text-overflow: ellipsis;
}

.chat-thread-runner-select:focus-within {
  border-color: rgba(56, 189, 248, 0.48);
  background: rgba(224, 242, 254, 0.92);
  color: rgb(3, 105, 161);
  box-shadow:
    inset 0 0 0 1px rgba(186, 230, 253, 0.78),
    0 10px 22px -20px rgba(14, 165, 233, 0.42);
}

.chat-thread-runner-select.is-built-in {
  border-color: rgba(96, 165, 250, 0.3);
  background: rgba(239, 246, 255, 0.96);
  color: rgb(37, 99, 235);
}

.chat-thread-runner-select.is-built-in:hover {
  border-color: rgba(59, 130, 246, 0.34);
  background: rgba(219, 234, 254, 0.98);
  color: rgb(29, 78, 216);
}

.chat-thread-runner-select.is-built-in:focus-within {
  border-color: rgba(59, 130, 246, 0.42);
  background: rgba(219, 234, 254, 0.98);
  color: rgb(29, 78, 216);
  box-shadow:
    inset 0 0 0 1px rgba(191, 219, 254, 0.92),
    0 10px 22px -20px rgba(37, 99, 235, 0.34);
}

.chat-thread-runner-select.is-agentcore {
  border-color: rgba(245, 158, 11, 0.28);
  background: rgba(255, 251, 235, 0.98);
  color: rgb(180, 83, 9);
}

.chat-thread-runner-select.is-agentcore:hover {
  border-color: rgba(245, 158, 11, 0.34);
  background: rgba(254, 243, 199, 0.98);
  color: rgb(146, 64, 14);
}

.chat-thread-runner-select.is-agentcore:focus-within {
  border-color: rgba(245, 158, 11, 0.42);
  background: rgba(254, 243, 199, 0.98);
  color: rgb(146, 64, 14);
  box-shadow:
    inset 0 0 0 1px rgba(253, 230, 138, 0.76),
    0 10px 22px -20px rgba(217, 119, 6, 0.34);
}

.chat-thread-runner-select__control:disabled,
.chat-mobile-runner-select:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.chat-mobile-runner-card {
  padding: 0.9rem;
}

.chat-mobile-runner-card.is-built-in {
  border-color: rgba(147, 197, 253, 0.7);
  background: rgba(239, 246, 255, 0.92);
}

.chat-mobile-runner-card.is-agentcore {
  border-color: rgba(252, 211, 77, 0.72);
  background: rgba(255, 251, 235, 0.96);
}

.chat-mobile-runner-card__header {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
}

.chat-mobile-runner-card__icon {
  width: 2rem;
  height: 2rem;
  border-radius: 0.8rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: rgb(59, 130, 246);
  background: rgba(219, 234, 254, 0.86);
  flex-shrink: 0;
}

.chat-mobile-runner-card.is-agentcore .chat-mobile-runner-card__icon {
  color: rgb(217, 119, 6);
  background: rgba(254, 243, 199, 0.92);
}

.chat-mobile-runner-card__icon svg {
  width: 1rem;
  height: 1rem;
}

.chat-mobile-runner-card__description {
  margin-top: 0.22rem;
  font-size: 0.75rem;
  line-height: 1.45;
  color: rgb(100, 116, 139);
}

.chat-mobile-runner-card__pill.is-built-in {
  color: rgb(29, 78, 216);
  background: rgba(219, 234, 254, 0.94);
}

.chat-mobile-runner-card__pill.is-agentcore {
  color: rgb(146, 64, 14);
  background: rgba(254, 243, 199, 0.94);
}

.chat-mobile-runner-select.is-built-in {
  border-color: rgba(96, 165, 250, 0.3);
  background: rgba(255, 255, 255, 0.98);
  color: rgb(29, 78, 216);
}

.chat-mobile-runner-select.is-agentcore {
  border-color: rgba(245, 158, 11, 0.32);
  background: rgba(255, 255, 255, 0.98);
  color: rgb(146, 64, 14);
}

.chat-thread-detail-btn {
  min-height: var(--chat-header-control-size);
  padding: 0 0.8rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: rgba(255, 255, 255, 0.88);
  color: rgb(71, 85, 105);
  font-size: 0.72rem;
  font-weight: 650;
  letter-spacing: 0.01em;
}

.chat-thread-detail-btn:hover {
  border-color: rgba(125, 211, 252, 0.38);
  background: rgba(248, 250, 252, 0.98);
  color: rgb(51, 65, 85);
}

.chat-thread-detail-btn.is-active {
  border-color: rgba(56, 189, 248, 0.48);
  background: rgba(224, 242, 254, 0.92);
  color: rgb(3, 105, 161);
  box-shadow:
    inset 0 0 0 1px rgba(186, 230, 253, 0.78),
    0 10px 22px -20px rgba(14, 165, 233, 0.42);
}

.chat-thread-routing-btn {
  margin-inline-start: 0.12rem;
  position: relative;
}

.chat-topbar {
  --chat-header-pad-x: var(--chat-pane-pad-x);
  --chat-header-pad-y: var(--chat-pane-pad-y);
  padding: var(--chat-header-pad-y) var(--chat-header-pad-x);
  gap: 0.85rem;
  min-height: 5rem;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(249, 250, 251, 0.95));
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  box-shadow: none;
  overflow: visible;
}

.chat-topbar::after {
  content: '';
  position: absolute;
  inset-inline: 0;
  bottom: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(148, 163, 184, 0.3), transparent);
  pointer-events: none;
}

.chat-topbar::before {
  content: none;
}

.chat-title-wrap {
  position: relative;
  gap: 0.85rem;
  min-width: 0;
}

.chat-title-stack {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 0.34rem;
}

.chat-section-label {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  font-size: 0.71rem;
  line-height: 1;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(100, 116, 139);
}

.chat-title {
  font-size: clamp(1.02rem, 1.12vw, 1.34rem);
  line-height: 1.14;
  font-weight: 720;
  letter-spacing: -0.03em;
  text-shadow: none;
}

.chat-title::before {
  content: none;
}

.chat-nav-btn,
.topbar-icon-btn {
  padding: 0.45rem;
  border: 1px solid transparent;
}

.topbar-chip {
  padding: 0.45rem;
  border: 1px solid transparent;
}

.chat-nav-btn:hover,
.topbar-icon-btn:hover {
  border-color: rgba(148, 163, 184, 0.3);
  background: rgba(241, 245, 249, 0.74);
}

.topbar-chip {
  border-color: var(--ct-border-soft);
  background: var(--ct-chip-bg);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05);
  min-height: var(--ui-chip-h);
  border-radius: var(--ui-radius);
}

.topbar-chip:hover {
  border-color: rgba(125, 211, 252, 0.42);
}

.chat-tools {
  margin-inline-start: auto;
  justify-content: flex-end;
  align-self: stretch;
  padding-inline-start: 0.95rem;
  margin-inline-start: 0.95rem;
  border-inline-start: 1px solid rgba(148, 163, 184, 0.16);
  gap: 0.55rem;
}

.topbar-icon-btn {
  box-shadow: none;
  background: transparent;
  min-width: 2.25rem;
  min-height: 2.25rem;
  border-radius: 0.9rem;
  border-color: transparent;
}

.topbar-icon-btn:active,
.topbar-chip:active,
.chat-nav-btn:active {
  transform: translateY(0);
}

.topbar-icon-btn:focus-visible,
.topbar-chip:focus-visible,
.chat-nav-btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1.5px rgba(56, 189, 248, 0.32);
  border-color: var(--ct-border-strong);
}

.topbar-icon-btn.topbar-icon-active {
  border-color: rgba(125, 211, 252, 0.3);
  background: rgba(241, 245, 249, 0.56);
}

.topbar-icon-btn.topbar-icon-active-research {
  color: rgb(6, 120, 95);
}

.topbar-icon-btn.topbar-icon-active-agent {
  color: rgb(29, 78, 216);
}

.topbar-icon-btn.topbar-icon-active-tools {
  color: rgb(3, 105, 161);
}

.bg-surface-base {
  background: transparent;
}

.chat-messages-area {
  scroll-padding-top: 1rem;
  scroll-padding-bottom: 10.75rem;
  padding-inline: clamp(0.45rem, 1.3vw, 1.2rem);
}

.chat-edge-quick-nav-layer {
  position: absolute;
  inset-block: var(--chat-header-row-block-size) 10.75rem;
  inset-inline-end: 0;
  width: 2.5rem;
  overflow: visible;
  z-index: 5;
  pointer-events: auto;
}

.chat-edge-quick-nav {
  position: absolute;
  inset-inline-end: 0;
  top: 50%;
  width: min(15rem, calc(100vw - 5rem));
  max-height: min(24rem, 100%);
  overflow: hidden;
  border: 1px solid transparent;
  border-radius: 1.75rem;
  background: rgba(255, 255, 255, 0.02);
  box-shadow: none;
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
  pointer-events: auto;
  transform: translate3d(calc(100% - 2.15rem), -50%, 0) scale(0.94);
  transform-origin: right center;
  transition:
    transform 0.22s ease,
    border-color 0.22s ease,
    background-color 0.22s ease,
    box-shadow 0.22s ease;
}

.chat-edge-quick-nav-layer:hover .chat-edge-quick-nav,
.chat-edge-quick-nav-layer:focus-within .chat-edge-quick-nav {
  border-color: rgba(226, 232, 240, 0.94);
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 24px 56px -36px rgba(15, 23, 42, 0.3);
  transform: translate3d(0, -50%, 0) scale(1.03);
}

.chat-edge-quick-nav__list {
  display: flex;
  flex-direction: column;
  gap: 0.18rem;
  max-height: inherit;
  overflow-y: auto;
  padding: 0.82rem 0.62rem;
}

.chat-edge-quick-nav__item {
  display: flex;
  align-items: center;
  gap: 0.72rem;
  width: 100%;
  padding: 0.52rem 0.46rem 0.52rem 0.82rem;
  border: none;
  border-radius: 1rem;
  background: transparent;
  color: rgba(100, 116, 139, 0.94);
  cursor: pointer;
  text-align: start;
  transition:
    background-color 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

.chat-edge-quick-nav__item:hover {
  background: rgba(248, 250, 252, 0.94);
  color: rgba(51, 65, 85, 0.98);
}

.chat-edge-quick-nav__item:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1.5px rgba(96, 165, 250, 0.44);
}

.chat-edge-quick-nav__item.is-active {
  background: transparent;
  color: rgb(15, 23, 42);
}

.chat-edge-quick-nav__text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  font-size: 0.95rem;
  font-weight: 400;
  line-height: 1.22;
  text-overflow: ellipsis;
  white-space: nowrap;
  opacity: 0.12;
  transform: translateX(0.4rem);
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}

.chat-edge-quick-nav__marker {
  width: 0.9rem;
  height: 0.2rem;
  border-radius: 999px;
  background: rgba(203, 213, 225, 0.96);
  flex-shrink: 0;
  opacity: 0.38;
  transition:
    width 0.18s ease,
    background-color 0.18s ease,
    opacity 0.18s ease;
}

.chat-edge-quick-nav__item:hover .chat-edge-quick-nav__marker {
  background: rgba(148, 163, 184, 0.96);
}

.chat-edge-quick-nav__item.is-active .chat-edge-quick-nav__marker {
  width: 1.22rem;
  background: rgb(37, 99, 235);
  opacity: 0.92;
}

.chat-edge-quick-nav-layer:hover .chat-edge-quick-nav__text,
.chat-edge-quick-nav-layer:focus-within .chat-edge-quick-nav__text {
  opacity: 1;
  transform: translateX(0);
}

.chat-edge-quick-nav-layer:hover .chat-edge-quick-nav__item.is-active,
.chat-edge-quick-nav-layer:focus-within .chat-edge-quick-nav__item.is-active {
  background: rgba(241, 245, 249, 0.96);
}

.chat-edge-quick-nav-layer:hover .chat-edge-quick-nav__marker,
.chat-edge-quick-nav-layer:focus-within .chat-edge-quick-nav__marker {
  opacity: 1;
}

.chat-stream-status-rail {
  width: min(100%, 56rem);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.9rem;
  padding: 0.72rem 0.9rem;
  border: 1px solid rgba(191, 219, 254, 0.78);
  border-radius: 1rem;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(239, 246, 255, 0.92));
  box-shadow: 0 18px 36px -34px rgba(37, 99, 235, 0.42);
  backdrop-filter: blur(10px);
}

.chat-stream-status-rail__copy {
  min-width: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem 0.6rem;
}

.chat-stream-status-rail__badge {
  display: inline-flex;
  align-items: center;
  padding: 0.18rem 0.48rem;
  border-radius: 999px;
  background: rgba(219, 234, 254, 0.96);
  color: rgb(29, 78, 216);
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.chat-stream-status-rail__label {
  min-width: 0;
  color: rgb(15, 23, 42);
  font-size: 0.92rem;
  font-weight: 600;
}

.chat-stream-status-rail__detail {
  min-width: 0;
  color: rgba(71, 85, 105, 0.96);
  font-size: 0.82rem;
}

.chat-stream-status-rail__actions {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  flex-shrink: 0;
}

.chat-stream-status-rail__action {
  border: 1px solid rgba(148, 163, 184, 0.34);
  border-radius: 999px;
  padding: 0.34rem 0.76rem;
  background: rgba(255, 255, 255, 0.86);
  color: rgb(51, 65, 85);
  font-size: 0.78rem;
  font-weight: 600;
  transition:
    background-color 0.16s ease,
    border-color 0.16s ease,
    color 0.16s ease;
}

.chat-stream-status-rail__action:hover {
  background: rgba(248, 250, 252, 1);
  border-color: rgba(96, 165, 250, 0.48);
  color: rgb(30, 64, 175);
}

.chat-stream-status-rail__action.is-danger:hover {
  border-color: rgba(248, 113, 113, 0.5);
  color: rgb(185, 28, 28);
}

.chat-thread-shell-desktop .chat-messages-area {
  scroll-padding-bottom: 10.75rem;
}

.chat-input-dock {
  position: relative;
  z-index: 4;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin-top: -1.1rem;
  padding-bottom: 0.85rem;
  background: transparent;
}

.active-todo-panel-wrap {
  --active-todo-panel-collapsed-height: 2.52rem;
  position: relative;
  z-index: 0;
  width: 100%;
  min-width: 0;
  height: var(--active-todo-panel-collapsed-height);
  margin-bottom: -0.42rem;
}

.active-todo-panel {
  position: absolute;
  inset-inline: 0;
  bottom: 0;
  overflow: hidden;
  border: 1px solid rgba(214, 219, 227, 0.96);
  border-radius: 1.42rem 1.42rem 1rem 1rem;
  background: rgba(248, 250, 252, 0.97);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.82),
    0 18px 38px -30px rgba(15, 23, 42, 0.22);
  backdrop-filter: blur(18px);
}

.active-todo-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.36rem;
  min-height: var(--active-todo-panel-collapsed-height);
  padding: 0.24rem 0.58rem 0.22rem;
}

.active-todo-panel.is-expanded .active-todo-panel__header {
  border-bottom: 1px solid rgba(226, 232, 240, 0.94);
}

.active-todo-panel__summary {
  display: flex;
  align-items: center;
  gap: 0.36rem;
  min-width: 0;
}

.active-todo-panel__summary--interactive {
  width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  text-align: start;
  cursor: pointer;
  transition: opacity 0.16s ease;
}

.active-todo-panel__summary--interactive:hover {
  opacity: 0.86;
}

.active-todo-panel__icon {
  width: 0.86rem;
  height: 0.86rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: rgba(71, 85, 105, 0.86);
  flex-shrink: 0;
}

.active-todo-panel__icon svg {
  width: 100%;
  height: 100%;
}

.active-todo-panel__progress {
  color: rgba(30, 41, 59, 0.94);
  font-size: 0.76rem;
  font-weight: 600;
  line-height: 1.1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.active-todo-panel__header-actions {
  display: inline-flex;
  align-items: center;
  gap: 0;
  flex-shrink: 0;
}

.active-todo-panel__icon-btn {
  width: 1.24rem;
  height: 1.24rem;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.92);
  background: rgba(255, 255, 255, 0.88);
  color: rgba(71, 85, 105, 0.9);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition:
    transform 0.16s ease,
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease;
}

.active-todo-panel__icon-btn:hover {
  transform: translateY(-1px);
  border-color: rgba(148, 163, 184, 0.96);
  background: rgba(255, 255, 255, 0.98);
  color: rgba(30, 41, 59, 0.96);
}

.active-todo-panel__toggle-icon {
  width: 0.74rem;
  height: 0.74rem;
  transition: transform 0.16s ease;
}

.active-todo-panel__toggle-icon.is-collapsed {
  transform: rotate(180deg);
}

.active-todo-panel__list {
  display: flex;
  flex-direction: column;
  gap: 0.52rem;
  max-height: min(9.75rem, 24vh);
  overflow-y: auto;
  padding: 0.68rem 0.95rem 0.8rem;
}

.active-todo-panel__item {
  display: flex;
  align-items: flex-start;
  gap: 0.56rem;
  color: rgba(51, 65, 85, 0.95);
  font-size: 0.95rem;
  line-height: 1.45;
}

.active-todo-panel__item.is-checked {
  color: rgba(100, 116, 139, 0.94);
}

.active-todo-panel__check {
  width: 1.08rem;
  height: 1.08rem;
  margin-top: 0.13rem;
  border-radius: 999px;
  border: 1.5px solid rgba(148, 163, 184, 0.82);
  background: rgba(255, 255, 255, 0.92);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: transparent;
  flex-shrink: 0;
}

.active-todo-panel__check.is-checked {
  border-color: rgba(34, 197, 94, 0.78);
  background: rgba(220, 252, 231, 0.96);
  color: rgb(22, 101, 52);
}

.active-todo-panel__check svg {
  width: 0.78rem;
  height: 0.78rem;
}

.active-todo-panel__index {
  min-width: 1.4rem;
  font-weight: 600;
  color: inherit;
  flex-shrink: 0;
}

.active-todo-panel__text {
  flex: 1;
  min-width: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

.chat-message-shell {
  position: relative;
  border-radius: 1.4rem;
  scroll-margin-block: 7.5rem;
  transition:
    background-color 0.24s ease,
    box-shadow 0.24s ease;
}

.chat-message-shell.is-todo-focused {
  background: rgba(239, 246, 255, 0.7);
  box-shadow:
    0 0 0 1px rgba(147, 197, 253, 0.42),
    0 18px 36px -34px rgba(37, 99, 235, 0.42);
}

.chat-desktop-shell .chat-input-dock {
  padding-bottom: 0;
}

html[data-blue-macos-glass='true'] .chat-desktop-shell .chat-messages-area {
  scroll-padding-bottom: 11.35rem;
}

html[data-blue-macos-glass='true'] .chat-desktop-shell .chat-input-dock {
  margin-top: 0;
  padding-bottom: 0.42rem;
}

.border-glass-border {
  border-color: rgba(186, 203, 223, 0.76);
}

:root.dark .chat-desktop-shell,
[data-theme='dark'] .chat-desktop-shell {
  background: transparent;
}

:root.dark .chat-stream-status-rail,
[data-theme='dark'] .chat-stream-status-rail {
  border-color: rgba(59, 130, 246, 0.3);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.9), rgba(30, 41, 59, 0.86));
  box-shadow: 0 20px 38px -32px rgba(2, 6, 23, 0.72);
}

:root.dark .chat-stream-status-rail__badge,
[data-theme='dark'] .chat-stream-status-rail__badge {
  background: rgba(30, 64, 175, 0.36);
  color: rgb(191, 219, 254);
}

:root.dark .chat-stream-status-rail__label,
[data-theme='dark'] .chat-stream-status-rail__label {
  color: rgb(226, 232, 240);
}

:root.dark .chat-stream-status-rail__detail,
[data-theme='dark'] .chat-stream-status-rail__detail {
  color: rgba(191, 219, 254, 0.78);
}

:root.dark .chat-stream-status-rail__action,
[data-theme='dark'] .chat-stream-status-rail__action {
  border-color: rgba(148, 163, 184, 0.24);
  background: rgba(15, 23, 42, 0.78);
  color: rgb(226, 232, 240);
}

:root.dark .chat-stream-status-rail__action:hover,
[data-theme='dark'] .chat-stream-status-rail__action:hover {
  border-color: rgba(96, 165, 250, 0.42);
  color: rgb(191, 219, 254);
}

:root.dark .chat-desktop-shell .chat-workspace,
[data-theme='dark'] .chat-desktop-shell .chat-workspace {
  background: transparent;
  box-shadow: none;
}

html[data-blue-macos-glass='true']:root.dark .chat-desktop-shell .chat-workspace,
html[data-blue-macos-glass='true'][data-theme='dark'] .chat-desktop-shell .chat-workspace,
html.dark[data-blue-macos-glass='true'] .chat-desktop-shell .chat-workspace {
  border: none;
  background: transparent;
  box-shadow: none;
}

:root.dark .chat-main-shell,
[data-theme='dark'] .chat-main-shell {
  background: transparent;
}

:root.dark .chat-edge-quick-nav,
[data-theme='dark'] .chat-edge-quick-nav {
  border-color: transparent;
  background: rgba(15, 23, 42, 0.04);
  box-shadow: none;
}

:root.dark .chat-edge-quick-nav-layer:hover .chat-edge-quick-nav,
[data-theme='dark'] .chat-edge-quick-nav-layer:hover .chat-edge-quick-nav,
:root.dark .chat-edge-quick-nav-layer:focus-within .chat-edge-quick-nav,
[data-theme='dark'] .chat-edge-quick-nav-layer:focus-within .chat-edge-quick-nav {
  border-color: rgba(51, 65, 85, 0.88);
  background: rgba(15, 23, 42, 0.84);
  box-shadow: 0 28px 58px -38px rgba(2, 6, 23, 0.72);
}

:root.dark .chat-edge-quick-nav__item,
[data-theme='dark'] .chat-edge-quick-nav__item {
  color: rgba(148, 163, 184, 0.96);
}

:root.dark .chat-edge-quick-nav__item:hover,
[data-theme='dark'] .chat-edge-quick-nav__item:hover {
  background: rgba(30, 41, 59, 0.92);
  color: rgb(226, 232, 240);
}

:root.dark .chat-edge-quick-nav__item.is-active,
[data-theme='dark'] .chat-edge-quick-nav__item.is-active {
  background: transparent;
  color: rgb(248, 250, 252);
}

:root.dark .chat-edge-quick-nav-layer:hover .chat-edge-quick-nav__item.is-active,
[data-theme='dark'] .chat-edge-quick-nav-layer:hover .chat-edge-quick-nav__item.is-active,
:root.dark .chat-edge-quick-nav-layer:focus-within .chat-edge-quick-nav__item.is-active,
[data-theme='dark'] .chat-edge-quick-nav-layer:focus-within .chat-edge-quick-nav__item.is-active {
  background: rgba(30, 41, 59, 0.98);
}

:root.dark .chat-edge-quick-nav__marker,
[data-theme='dark'] .chat-edge-quick-nav__marker {
  background: rgba(100, 116, 139, 0.96);
}

:root.dark .chat-edge-quick-nav__item:hover .chat-edge-quick-nav__marker,
[data-theme='dark'] .chat-edge-quick-nav__item:hover .chat-edge-quick-nav__marker {
  background: rgba(148, 163, 184, 0.98);
}

:root.dark .chat-edge-quick-nav__item.is-active .chat-edge-quick-nav__marker,
[data-theme='dark'] .chat-edge-quick-nav__item.is-active .chat-edge-quick-nav__marker {
  background: rgb(96, 165, 250);
}

:root.dark .chat-sidebar-shell,
[data-theme='dark'] .chat-sidebar-shell {
  background: transparent;
}

:root.dark .chat-desktop-shell .chat-sidebar-shell,
[data-theme='dark'] .chat-desktop-shell .chat-sidebar-shell {
  border-right-color: rgba(71, 85, 105, 0.76);
  background: transparent;
}

:root.dark .chat-page-header,
[data-theme='dark'] .chat-page-header {
  background: transparent;
}

:root.dark .chat-thread-header,
[data-theme='dark'] .chat-thread-header,
:root.dark .chat-thread-subrow,
[data-theme='dark'] .chat-thread-subrow {
  border-color: rgba(51, 65, 85, 0.88);
}

:root.dark .chat-thread-header,
[data-theme='dark'] .chat-thread-header {
  background: rgba(15, 23, 42, 0.92);
  box-shadow: inset 0 -1px 0 rgba(51, 65, 85, 0.88);
}

:root.dark .chat-thread-subrow,
[data-theme='dark'] .chat-thread-subrow {
  background: rgba(15, 23, 42, 0.58);
}

:root.dark .chat-trial-inline-progress,
[data-theme='dark'] .chat-trial-inline-progress {
  background: rgba(51, 65, 85, 0.92);
}

:root.dark .chat-trial-inline-progress-bar,
[data-theme='dark'] .chat-trial-inline-progress-bar {
  background: rgb(96, 165, 250);
}

:root.dark .chat-trial-inline-percent,
[data-theme='dark'] .chat-trial-inline-percent {
  color: rgb(148, 163, 184);
}

:root.dark .chat-trial-inline-link,
[data-theme='dark'] .chat-trial-inline-link {
  color: rgb(148, 163, 184);
}

:root.dark .chat-topbar,
[data-theme='dark'] .chat-topbar {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.93), rgba(15, 23, 42, 0.86));
  box-shadow: none;
}

:root.dark .chat-page-header::after,
[data-theme='dark'] .chat-page-header::after {
  content: none;
}

:root.dark .chat-thread-shell-desktop,
[data-theme='dark'] .chat-thread-shell-desktop {
  background: transparent;
  border: none;
  box-shadow: none;
}

:root.dark .bg-surface-base,
[data-theme='dark'] .bg-surface-base {
  background: transparent;
}

:root.dark .chat-input-dock,
[data-theme='dark'] .chat-input-dock {
  background: transparent;
}

:root.dark .active-todo-panel,
[data-theme='dark'] .active-todo-panel {
  border-color: rgba(71, 85, 105, 0.78);
  background: rgba(15, 23, 42, 0.95);
  box-shadow:
    inset 0 1px 0 rgba(148, 163, 184, 0.08),
    0 18px 38px -28px rgba(2, 6, 23, 0.82);
}

:root.dark .active-todo-panel__icon,
[data-theme='dark'] .active-todo-panel__icon {
  color: rgba(148, 163, 184, 0.88);
}

:root.dark .active-todo-panel__progress,
[data-theme='dark'] .active-todo-panel__progress {
  color: rgba(226, 232, 240, 0.98);
}

:root.dark .active-todo-panel__icon-btn,
[data-theme='dark'] .active-todo-panel__icon-btn {
  border-color: rgba(71, 85, 105, 0.82);
  background: rgba(15, 23, 42, 0.88);
  color: rgba(203, 213, 225, 0.92);
}

:root.dark .active-todo-panel__icon-btn:hover,
[data-theme='dark'] .active-todo-panel__icon-btn:hover {
  border-color: rgba(100, 116, 139, 0.9);
  background: rgba(30, 41, 59, 0.94);
  color: rgba(241, 245, 249, 0.98);
}

:root.dark .active-todo-panel__item,
[data-theme='dark'] .active-todo-panel__item {
  color: rgba(226, 232, 240, 0.94);
}

:root.dark .active-todo-panel__item.is-checked,
[data-theme='dark'] .active-todo-panel__item.is-checked {
  color: rgba(148, 163, 184, 0.88);
}

:root.dark .active-todo-panel__check,
[data-theme='dark'] .active-todo-panel__check {
  border-color: rgba(100, 116, 139, 0.9);
  background: rgba(15, 23, 42, 0.82);
}

:root.dark .active-todo-panel__check.is-checked,
[data-theme='dark'] .active-todo-panel__check.is-checked {
  border-color: rgba(74, 222, 128, 0.42);
  background: rgba(6, 78, 59, 0.38);
  color: rgb(110, 231, 183);
}

:root.dark .chat-message-shell.is-todo-focused,
[data-theme='dark'] .chat-message-shell.is-todo-focused {
  background: rgba(8, 47, 73, 0.24);
  box-shadow:
    0 0 0 1px rgba(56, 189, 248, 0.28),
    0 18px 36px -32px rgba(14, 165, 233, 0.34);
}

:root.dark .topbar-icon-btn,
[data-theme='dark'] .topbar-icon-btn {
  background: transparent;
  box-shadow: none;
  border-color: transparent;
}

:root.dark .chat-nav-btn:hover,
:root.dark .topbar-icon-btn:hover,
[data-theme='dark'] .chat-nav-btn:hover,
[data-theme='dark'] .topbar-icon-btn:hover {
  background: rgba(30, 41, 59, 0.74);
  border-color: rgba(100, 116, 139, 0.5);
}

:root.dark .topbar-icon-btn.topbar-icon-active,
[data-theme='dark'] .topbar-icon-btn.topbar-icon-active {
  border-color: rgba(56, 189, 248, 0.3);
  background: rgba(8, 47, 73, 0.3);
}

:root.dark .topbar-icon-btn.topbar-icon-active-research,
[data-theme='dark'] .topbar-icon-btn.topbar-icon-active-research {
  color: rgb(134, 239, 172);
}

:root.dark .topbar-icon-btn.topbar-icon-active-agent,
[data-theme='dark'] .topbar-icon-btn.topbar-icon-active-agent {
  color: rgb(125, 211, 252);
}

:root.dark .topbar-icon-btn.topbar-icon-active-tools,
[data-theme='dark'] .topbar-icon-btn.topbar-icon-active-tools {
  color: rgb(103, 232, 249);
}

:root.dark .chat-thread-detail-btn,
[data-theme='dark'] .chat-thread-detail-btn {
  border-color: rgba(71, 85, 105, 0.58);
  background: rgba(30, 41, 59, 0.76);
  color: rgb(203, 213, 225);
}

:root.dark .chat-thread-detail-btn:hover,
[data-theme='dark'] .chat-thread-detail-btn:hover {
  border-color: rgba(100, 116, 139, 0.68);
  background: rgba(30, 41, 59, 0.96);
  color: rgb(241, 245, 249);
}

:root.dark .chat-thread-detail-btn.is-active,
[data-theme='dark'] .chat-thread-detail-btn.is-active {
  border-color: rgba(56, 189, 248, 0.58);
  background: rgba(8, 47, 73, 0.92);
  color: rgb(125, 211, 252);
  box-shadow:
    inset 0 0 0 1px rgba(14, 165, 233, 0.28),
    0 12px 24px -22px rgba(14, 165, 233, 0.42);
}

:root.dark .chat-section-label,
[data-theme='dark'] .chat-section-label {
  color: rgb(148, 163, 184);
}

.quick-action-tile {
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(255, 255, 255, 0.92);
  border-radius: 0.9rem;
  padding: 0.75rem;
  text-align: start;
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    background-color 0.18s ease;
}

.quick-action-tile:active {
  transform: scale(0.98);
}

.quick-action-tile.is-active {
  border-color: rgba(56, 189, 248, 0.58);
  box-shadow: 0 6px 16px rgba(14, 165, 233, 0.18);
  background: rgba(236, 253, 255, 0.95);
}

.quick-action-pill {
  display: inline-flex;
  align-items: center;
  height: 1.2rem;
  border-radius: 999px;
  padding: 0 0.42rem;
  font-size: 0.65rem;
  font-weight: 600;
  color: rgb(71, 85, 105);
  background: rgba(226, 232, 240, 0.9);
}

:root.dark .quick-action-tile,
[data-theme='dark'] .quick-action-tile {
  background: rgba(30, 41, 59, 0.92);
  border-color: rgba(100, 116, 139, 0.45);
}

:root.dark .quick-action-tile.is-active,
[data-theme='dark'] .quick-action-tile.is-active {
  border-color: rgba(56, 189, 248, 0.62);
  background: rgba(15, 23, 42, 0.94);
  box-shadow: 0 8px 18px rgba(2, 132, 199, 0.22);
}

:root.dark .quick-action-pill,
[data-theme='dark'] .quick-action-pill {
  color: rgb(203, 213, 225);
  background: rgba(51, 65, 85, 0.88);
}

:root.dark .chat-mobile-runner-card.is-built-in,
[data-theme='dark'] .chat-mobile-runner-card.is-built-in {
  border-color: rgba(59, 130, 246, 0.4);
  background: rgba(15, 23, 42, 0.94);
}

:root.dark .chat-mobile-runner-card.is-agentcore,
[data-theme='dark'] .chat-mobile-runner-card.is-agentcore {
  border-color: rgba(245, 158, 11, 0.38);
  background: rgba(69, 26, 3, 0.34);
}

:root.dark .chat-thread-runner-select__icon,
[data-theme='dark'] .chat-thread-runner-select__icon {
  color: rgba(226, 232, 240, 0.92);
}

:root.dark .chat-thread-runner-select:focus-within,
[data-theme='dark'] .chat-thread-runner-select:focus-within {
  border-color: rgba(56, 189, 248, 0.58);
  background: rgba(8, 47, 73, 0.92);
  color: rgb(125, 211, 252);
  box-shadow:
    inset 0 0 0 1px rgba(14, 165, 233, 0.28),
    0 12px 24px -22px rgba(14, 165, 233, 0.42);
}

:root.dark .chat-thread-runner-select.is-built-in,
[data-theme='dark'] .chat-thread-runner-select.is-built-in {
  border-color: rgba(59, 130, 246, 0.4);
  background: rgba(15, 23, 42, 0.96);
  color: rgb(147, 197, 253);
}

:root.dark .chat-thread-runner-select.is-built-in:hover,
[data-theme='dark'] .chat-thread-runner-select.is-built-in:hover {
  border-color: rgba(96, 165, 250, 0.5);
  background: rgba(30, 41, 59, 0.96);
  color: rgb(191, 219, 254);
}

:root.dark .chat-thread-runner-select.is-built-in:focus-within,
[data-theme='dark'] .chat-thread-runner-select.is-built-in:focus-within {
  border-color: rgba(96, 165, 250, 0.54);
  background: rgba(8, 47, 73, 0.88);
  color: rgb(191, 219, 254);
  box-shadow:
    inset 0 0 0 1px rgba(96, 165, 250, 0.24),
    0 12px 24px -22px rgba(37, 99, 235, 0.42);
}

:root.dark .chat-thread-runner-select.is-agentcore,
[data-theme='dark'] .chat-thread-runner-select.is-agentcore {
  border-color: rgba(245, 158, 11, 0.38);
  background: rgba(69, 26, 3, 0.34);
  color: rgb(253, 186, 116);
}

:root.dark .chat-thread-runner-select.is-agentcore:hover,
[data-theme='dark'] .chat-thread-runner-select.is-agentcore:hover {
  border-color: rgba(251, 191, 36, 0.46);
  background: rgba(120, 53, 15, 0.34);
  color: rgb(254, 215, 170);
}

:root.dark .chat-thread-runner-select.is-agentcore:focus-within,
[data-theme='dark'] .chat-thread-runner-select.is-agentcore:focus-within {
  border-color: rgba(251, 191, 36, 0.54);
  background: rgba(120, 53, 15, 0.38);
  color: rgb(254, 215, 170);
  box-shadow:
    inset 0 0 0 1px rgba(245, 158, 11, 0.22),
    0 12px 24px -22px rgba(217, 119, 6, 0.42);
}

:root.dark .chat-mobile-runner-card__icon,
[data-theme='dark'] .chat-mobile-runner-card__icon {
  color: rgb(191, 219, 254);
  background: rgba(30, 64, 175, 0.28);
}

:root.dark .chat-mobile-runner-card.is-agentcore .chat-mobile-runner-card__icon,
[data-theme='dark'] .chat-mobile-runner-card.is-agentcore .chat-mobile-runner-card__icon {
  color: rgb(253, 186, 116);
  background: rgba(120, 53, 15, 0.44);
}

:root.dark .chat-mobile-runner-card__description,
[data-theme='dark'] .chat-mobile-runner-card__description {
  color: rgb(148, 163, 184);
}

:root.dark .chat-mobile-runner-card__pill.is-built-in,
[data-theme='dark'] .chat-mobile-runner-card__pill.is-built-in {
  color: rgb(191, 219, 254);
  background: rgba(30, 64, 175, 0.32);
}

:root.dark .chat-mobile-runner-card__pill.is-agentcore,
[data-theme='dark'] .chat-mobile-runner-card__pill.is-agentcore {
  color: rgb(253, 186, 116);
  background: rgba(120, 53, 15, 0.42);
}

:root.dark .chat-mobile-runner-select.is-built-in,
[data-theme='dark'] .chat-mobile-runner-select.is-built-in {
  border-color: rgba(59, 130, 246, 0.38);
  background: rgba(15, 23, 42, 0.96);
  color: rgb(191, 219, 254);
}

:root.dark .chat-mobile-runner-select.is-agentcore,
[data-theme='dark'] .chat-mobile-runner-select.is-agentcore {
  border-color: rgba(245, 158, 11, 0.38);
  background: rgba(30, 41, 59, 0.96);
  color: rgb(253, 186, 116);
}

.mobile-feature-sheet-panel {
  border-radius: 1.6rem 1.6rem 0 0;
  border: 1px solid rgba(226, 232, 240, 0.94);
  border-bottom: none;
  background: rgba(255, 255, 255, 0.98);
}

.mobile-feature-sheet-content {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.mobile-feature-sheet-hero {
  display: flex;
  align-items: flex-start;
  gap: 0.9rem;
}

.mobile-feature-sheet-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  flex-shrink: 0;
  border-radius: 1rem;
  border: 1px solid rgba(226, 232, 240, 0.94);
  background: rgba(248, 250, 252, 0.94);
  color: rgb(71, 85, 105);
}

.mobile-feature-sheet-icon svg {
  width: 1.35rem;
  height: 1.35rem;
}

.mobile-feature-sheet-icon.is-deep-research {
  border-color: rgba(134, 239, 172, 0.42);
  background: rgba(240, 253, 244, 0.94);
  color: rgb(22, 101, 52);
}

.mobile-feature-sheet-icon.is-smart-resume {
  border-color: rgba(125, 211, 252, 0.42);
  background: rgba(240, 249, 255, 0.94);
  color: rgb(3, 105, 161);
}

.mobile-feature-sheet-state {
  display: inline-flex;
  align-items: center;
  min-height: 1.35rem;
  border-radius: 999px;
  padding: 0 0.55rem;
  font-size: 0.72rem;
  font-weight: 600;
  color: rgb(71, 85, 105);
  background: rgba(226, 232, 240, 0.84);
}

.mobile-feature-sheet-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
}

.mobile-feature-sheet-chip {
  display: inline-flex;
  align-items: center;
  min-height: 1.6rem;
  border-radius: 999px;
  padding: 0 0.7rem;
  font-size: 0.72rem;
  font-weight: 600;
  color: rgb(71, 85, 105);
  border: 1px solid rgba(203, 213, 225, 0.88);
  background: rgba(248, 250, 252, 0.94);
}

.mobile-feature-sheet-actions {
  display: flex;
  gap: 0.75rem;
}

.mobile-feature-sheet-primary,
.mobile-feature-sheet-secondary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.85rem;
  border-radius: 0.95rem;
  padding: 0 1rem;
  font-size: 0.92rem;
  font-weight: 600;
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

.mobile-feature-sheet-primary {
  flex: 1 1 auto;
  border: 1px solid rgba(2, 132, 199, 0.22);
  background: rgb(2, 132, 199);
  color: rgb(255, 255, 255);
}

.mobile-feature-sheet-secondary {
  flex: 1 1 auto;
  border: 1px solid rgba(203, 213, 225, 0.94);
  background: rgba(248, 250, 252, 0.94);
  color: rgb(51, 65, 85);
}

.mobile-feature-sheet-primary:active,
.mobile-feature-sheet-secondary:active {
  transform: scale(0.98);
}

:root.dark .mobile-feature-sheet-panel,
[data-theme='dark'] .mobile-feature-sheet-panel {
  border-color: rgba(51, 65, 85, 0.92);
  background: rgba(15, 23, 42, 0.98);
}

:root.dark .mobile-feature-sheet-icon,
[data-theme='dark'] .mobile-feature-sheet-icon {
  border-color: rgba(71, 85, 105, 0.88);
  background: rgba(30, 41, 59, 0.92);
  color: rgb(203, 213, 225);
}

:root.dark .mobile-feature-sheet-icon.is-deep-research,
[data-theme='dark'] .mobile-feature-sheet-icon.is-deep-research {
  border-color: rgba(74, 222, 128, 0.28);
  background: rgba(20, 83, 45, 0.42);
  color: rgb(187, 247, 208);
}

:root.dark .mobile-feature-sheet-icon.is-smart-resume,
[data-theme='dark'] .mobile-feature-sheet-icon.is-smart-resume {
  border-color: rgba(56, 189, 248, 0.28);
  background: rgba(8, 47, 73, 0.46);
  color: rgb(186, 230, 253);
}

:root.dark .mobile-feature-sheet-state,
[data-theme='dark'] .mobile-feature-sheet-state {
  color: rgb(226, 232, 240);
  background: rgba(51, 65, 85, 0.88);
}

:root.dark .mobile-feature-sheet-chip,
[data-theme='dark'] .mobile-feature-sheet-chip {
  color: rgb(203, 213, 225);
  border-color: rgba(71, 85, 105, 0.88);
  background: rgba(30, 41, 59, 0.9);
}

:root.dark .mobile-feature-sheet-secondary,
[data-theme='dark'] .mobile-feature-sheet-secondary {
  color: rgb(226, 232, 240);
  border-color: rgba(71, 85, 105, 0.88);
  background: rgba(30, 41, 59, 0.92);
}

.routing-trigger-btn {
  color: rgb(71, 85, 105);
  border-color: rgba(148, 163, 184, 0.28);
  background: rgba(255, 255, 255, 0.76);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.62);
}

.routing-trigger-btn:hover {
  border-color: rgba(148, 163, 184, 0.35);
  background: rgba(241, 245, 249, 0.98);
  color: rgb(51, 65, 85);
}

.routing-trigger-btn.is-error {
  color: rgb(220, 38, 38);
  border-color: rgba(248, 113, 113, 0.55);
  background: rgba(254, 242, 242, 0.86);
}

.routing-trigger-btn.is-pending {
  color: rgb(202, 138, 4);
  border-color: rgba(250, 204, 21, 0.58);
  background: rgba(254, 249, 195, 0.84);
}

.routing-trigger-btn.is-none {
  color: rgb(71, 85, 105);
  border-color: rgba(148, 163, 184, 0.4);
  background: rgba(248, 250, 252, 0.86);
}

:root.dark .routing-trigger-btn,
[data-theme='dark'] .routing-trigger-btn {
  color: rgb(148, 163, 184);
  border-color: rgba(71, 85, 105, 0.58);
  background: rgba(15, 23, 42, 0.74);
  box-shadow:
    inset 0 1px 0 rgba(148, 163, 184, 0.12),
    0 1px 3px rgba(2, 6, 23, 0.45);
}

:root.dark .routing-trigger-btn:hover,
[data-theme='dark'] .routing-trigger-btn:hover {
  color: rgb(226, 232, 240);
  border-color: rgba(100, 116, 139, 0.48);
  background: rgba(30, 41, 59, 0.92);
}

:root.dark .routing-trigger-btn.is-error,
[data-theme='dark'] .routing-trigger-btn.is-error {
  color: rgb(252, 165, 165);
  border-color: rgba(248, 113, 113, 0.58);
  background: rgba(69, 10, 10, 0.42);
}

:root.dark .routing-trigger-btn.is-pending,
[data-theme='dark'] .routing-trigger-btn.is-pending {
  color: rgb(253, 224, 71);
  border-color: rgba(250, 204, 21, 0.54);
  background: rgba(113, 63, 18, 0.42);
}

:root.dark .routing-trigger-btn.is-none,
[data-theme='dark'] .routing-trigger-btn.is-none {
  color: rgb(203, 213, 225);
  border-color: rgba(100, 116, 139, 0.55);
  background: rgba(30, 41, 59, 0.92);
}

.routing-option-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  width: 100%;
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 0.9rem;
  background: #fff;
  padding: 0.68rem 0.72rem;
  color: rgb(31, 41, 55);
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    box-shadow 0.16s ease,
    transform 0.16s ease;
}

.routing-option-row:hover {
  border-color: rgba(59, 130, 246, 0.38);
  background: rgba(248, 250, 252, 0.95);
}

.routing-option-row:active {
  transform: scale(0.995);
}

.routing-option-row.is-active {
  border-color: rgba(96, 165, 250, 0.72);
  background: rgba(219, 234, 254, 0.72);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.75);
}

.routing-option-row.is-disabled {
  opacity: 0.52;
  cursor: not-allowed;
}

.routing-option-main {
  display: flex;
  align-items: flex-start;
  gap: 0.6rem;
  min-width: 0;
}

.routing-option-icon {
  width: 1.15rem;
  height: 1.15rem;
  flex-shrink: 0;
  margin-top: 0.06rem;
  color: rgb(71, 85, 105);
}

.routing-option-copy {
  min-width: 0;
}

.routing-option-title {
  font-size: 0.88rem;
  font-weight: 700;
  line-height: 1.2;
  color: rgb(30, 41, 59);
}

.routing-option-desc {
  margin-top: 0.18rem;
  font-size: 0.7rem;
  line-height: 1.35;
  color: rgb(100, 116, 139);
}

.routing-option-side {
  display: inline-flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.28rem;
  flex-shrink: 0;
}

.routing-option-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.35rem;
  height: 1.35rem;
  padding: 0 0.35rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 700;
  color: rgb(22, 101, 52);
  background: rgba(220, 252, 231, 0.95);
}

.routing-option-status-chip {
  display: inline-flex;
  align-items: center;
  height: 1rem;
  padding: 0 0.38rem;
  border-radius: 999px;
  font-size: 0.62rem;
  font-weight: 700;
  color: rgb(29, 78, 216);
  background: rgba(191, 219, 254, 0.9);
}

.routing-strategy-chip {
  border: 1px solid rgba(203, 213, 225, 0.9);
  border-radius: 0.65rem;
  padding: 0.45rem 0.6rem;
  font-size: 0.72rem;
  font-weight: 650;
  text-align: center;
  color: rgb(71, 85, 105);
  background: rgba(248, 250, 252, 0.96);
  transition: all 0.16s ease;
}

.routing-strategy-chip:hover {
  border-color: rgba(96, 165, 250, 0.5);
}

.routing-strategy-chip:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.routing-strategy-chip.is-active-ha {
  color: rgb(29, 78, 216);
  border-color: rgba(59, 130, 246, 0.6);
  background: rgba(219, 234, 254, 0.78);
}

.routing-strategy-chip.is-active-fixed {
  color: rgb(4, 120, 87);
  border-color: rgba(16, 185, 129, 0.55);
  background: rgba(209, 250, 229, 0.78);
}

.routing-model-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 0.65rem;
  background: rgba(255, 255, 255, 0.98);
  padding: 0.46rem 0.6rem;
  font-size: 0.76rem;
  color: rgb(31, 41, 55);
  transition: all 0.16s ease;
}

.routing-model-row:hover {
  border-color: rgba(96, 165, 250, 0.45);
}

.routing-model-row.is-selected {
  border-color: rgba(16, 185, 129, 0.62);
  background: rgba(236, 253, 245, 0.95);
}

.routing-manage-row:hover {
  background: rgba(248, 250, 252, 0.92);
}

.routing-menu-floating {
  background: #f8fafc;
  border-color: rgba(148, 163, 184, 0.52);
  box-shadow: 0 20px 52px rgba(15, 23, 42, 0.26);
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
  opacity: 1 !important;
}

.routing-sheet-panel {
  background: #ffffff;
  border-top: 1px solid rgba(148, 163, 184, 0.4);
  box-shadow: 0 -16px 38px rgba(15, 23, 42, 0.35);
}

:root.dark .routing-menu-floating,
[data-theme='dark'] .routing-menu-floating {
  background: rgb(15, 23, 42);
  border-color: rgba(100, 116, 139, 0.72);
  box-shadow: 0 22px 54px rgba(2, 6, 23, 0.62);
  opacity: 1 !important;
}

:root.dark .routing-sheet-panel,
[data-theme='dark'] .routing-sheet-panel {
  background: rgb(15, 23, 42);
  border-top-color: rgba(100, 116, 139, 0.5);
  box-shadow: 0 -16px 40px rgba(2, 6, 23, 0.72);
}

:root.dark .routing-option-row,
[data-theme='dark'] .routing-option-row {
  border-color: rgba(71, 85, 105, 0.75);
  background: rgba(30, 41, 59, 0.96);
  color: rgb(226, 232, 240);
}

:root.dark .routing-option-row:hover,
[data-theme='dark'] .routing-option-row:hover {
  border-color: rgba(56, 189, 248, 0.42);
  background: rgba(30, 41, 59, 1);
}

:root.dark .routing-option-row.is-active,
[data-theme='dark'] .routing-option-row.is-active {
  border-color: rgba(56, 189, 248, 0.66);
  background: rgba(8, 47, 73, 0.72);
}

:root.dark .routing-option-icon,
[data-theme='dark'] .routing-option-icon {
  color: rgb(148, 163, 184);
}

:root.dark .routing-option-title,
[data-theme='dark'] .routing-option-title {
  color: rgb(241, 245, 249);
}

:root.dark .routing-option-desc,
[data-theme='dark'] .routing-option-desc {
  color: rgb(148, 163, 184);
}

:root.dark .routing-option-count,
[data-theme='dark'] .routing-option-count {
  color: rgb(134, 239, 172);
  background: rgba(6, 78, 59, 0.78);
}

:root.dark .routing-option-status-chip,
[data-theme='dark'] .routing-option-status-chip {
  color: rgb(186, 230, 253);
  background: rgba(12, 74, 110, 0.82);
}

:root.dark .routing-strategy-chip,
[data-theme='dark'] .routing-strategy-chip {
  border-color: rgba(71, 85, 105, 0.8);
  color: rgb(203, 213, 225);
  background: rgba(30, 41, 59, 0.95);
}

:root.dark .routing-strategy-chip.is-active-ha,
[data-theme='dark'] .routing-strategy-chip.is-active-ha {
  color: rgb(186, 230, 253);
  border-color: rgba(56, 189, 248, 0.68);
  background: rgba(12, 74, 110, 0.52);
}

:root.dark .routing-strategy-chip.is-active-fixed,
[data-theme='dark'] .routing-strategy-chip.is-active-fixed {
  color: rgb(167, 243, 208);
  border-color: rgba(16, 185, 129, 0.66);
  background: rgba(6, 78, 59, 0.5);
}

:root.dark .routing-model-row,
[data-theme='dark'] .routing-model-row {
  border-color: rgba(71, 85, 105, 0.75);
  color: rgb(226, 232, 240);
  background: rgba(30, 41, 59, 0.95);
}

:root.dark .routing-model-row.is-selected,
[data-theme='dark'] .routing-model-row.is-selected {
  border-color: rgba(16, 185, 129, 0.66);
  background: rgba(6, 78, 59, 0.5);
}

:root.dark .routing-manage-row:hover,
[data-theme='dark'] .routing-manage-row:hover {
  background: rgba(30, 41, 59, 0.92);
}

:root.light .chat-title::before,
[data-theme='light'] .chat-title::before {
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
}

:root.light .chat-nav-btn:hover,
:root.light .topbar-icon-btn:hover,
[data-theme='light'] .chat-nav-btn:hover,
[data-theme='light'] .topbar-icon-btn:hover {
  border-color: rgba(148, 163, 184, 0.35);
}

:root.light .topbar-chip,
[data-theme='light'] .topbar-chip {
  border-color: var(--ct-border-soft);
  background: var(--ct-chip-bg);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

:root.light .topbar-icon-btn,
[data-theme='light'] .topbar-icon-btn {
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

:root.light .chat-tools,
[data-theme='light'] .chat-tools {
  border-left-color: rgba(148, 163, 184, 0.22);
}

:root.light .bg-surface-base,
[data-theme='light'] .bg-surface-base {
  background: transparent;
}

:root.light .chat-input-dock,
[data-theme='light'] .chat-input-dock {
  background: transparent;
}

:root.light .border-glass-border,
[data-theme='light'] .border-glass-border {
  border-color: rgba(186, 203, 223, 0.65);
}

@media (max-width: 1024px) {
  .chat-page-header {
    padding-inline: 1rem;
  }

  .chat-thread-shell-desktop {
    padding-inline: var(--chat-thread-pad-x);
  }
}

@media (max-width: 640px) {
  .chat-stream-status-rail {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.7rem;
    padding: 0.72rem 0.8rem;
  }

  .chat-stream-status-rail__actions {
    width: 100%;
    justify-content: flex-end;
  }

  .chat-topbar {
    --chat-header-pad-x: 0.82rem;
    --chat-header-pad-y: 0.72rem;
    min-height: auto;
  }

  .chat-title-stack {
    gap: 0.22rem;
  }

  .chat-section-label {
    font-size: 0.68rem;
  }

  .chat-title {
    font-size: 1rem;
  }

  .chat-tools {
    margin-inline-start: 0;
    padding-inline-start: 0.1rem;
    border-inline-start: none;
  }

  .chat-topbar::after {
    opacity: 0.7;
  }

  .active-todo-panel-wrap {
    --active-todo-panel-collapsed-height: 2.38rem;
    margin-bottom: 0.12rem;
  }

  .active-todo-panel {
    border-radius: 1.18rem;
  }

  .active-todo-panel__header {
    padding: 0.2rem 0.5rem 0.18rem;
  }

  .active-todo-panel__progress {
    font-size: 0.72rem;
  }

  .active-todo-panel__header-actions {
    gap: 0;
  }

  .active-todo-panel__icon-btn {
    width: 1.16rem;
    height: 1.16rem;
  }

  .active-todo-panel__list {
    max-height: min(8rem, 20vh);
    padding: 0.6rem 0.82rem 0.72rem;
  }

  .active-todo-panel__item {
    font-size: 0.9rem;
    gap: 0.48rem;
  }
}

@keyframes topbar-sheen {
  0% {
    transform: translateX(-120%);
  }
  25% {
    transform: translateX(100%);
  }
  100% {
    transform: translateX(100%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .chat-topbar::before {
    animation: none !important;
  }

  .chat-nav-btn,
  .topbar-icon-btn,
  .topbar-chip {
    transition: none !important;
    transform: none !important;
  }
}

/* Mobile view styles */
.mobile-view {
  position: fixed;
  inset: 0;
  z-index: 10;
  background: transparent;
}

/* Mobile conversation list page */
.mobile-list-page {
  width: 100%;
  max-width: 100%;
  height: 100%;
  border: none;
  background: transparent;
  overflow-x: hidden;
}

:global(.dark) .mobile-list-page {
  background: transparent;
}

.chat-sidebar-mobile {
  width: 100%;
  max-width: 100%;
  padding-top: max(env(safe-area-inset-top), 0px);
  padding-bottom: max(env(safe-area-inset-bottom), 0px);
  overflow-x: hidden;
}

/* Mobile main chat area */
.mobile-chat {
  position: fixed !important;
  inset: 0;
  z-index: 20;
  background: var(--chat-bg);
  flex-direction: column !important;
  transform: translateX(0);
}

.mobile-chat-animated {
  transition: transform 0.26s cubic-bezier(0.22, 1, 0.36, 1);
}

.mobile-chat-offscreen {
  transform: translateX(100%);
  pointer-events: none;
}

/* Mobile chat messages area - ensure scrolling works */
.mobile-chat .chat-messages-area {
  flex: 1;
  min-height: 0;
  overflow-y: auto !important;
  -webkit-overflow-scrolling: touch;
  overscroll-behavior: contain;
  scroll-padding-bottom: 8.2rem;
}

/* Mobile input area - stick to bottom */
.mobile-chat .chat-input-dock {
  position: relative !important;
  bottom: auto !important;
  margin-top: 0;
  background: transparent;
  padding-bottom: calc(max(env(safe-area-inset-bottom), 0px) + 0.75rem);
}

.mobile-chat .active-todo-panel-wrap {
  margin-bottom: 0.18rem;
}

@media (max-width: 767px) {
  .chat-nav-btn,
  .topbar-icon-btn {
    min-width: 2.75rem;
    min-height: 2.75rem;
    padding: 0.6rem;
    border-radius: 0.95rem;
  }
}

.chat-topbar-mobile {
  padding-top: calc(max(env(safe-area-inset-top), 0px) + var(--chat-header-pad-y));
  padding-bottom: 0.78rem;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

/* Slide animation for mobile chat */
.conversation-sidebar {
  height: 100%;
}

/* Safe area for mobile devices with notches */
@supports (padding-bottom: env(safe-area-inset-bottom)) {
  .chat-view {
    padding-bottom: env(safe-area-inset-bottom);
  }
}

/* Smooth scrolling for messages */
.overscroll-contain {
  overscroll-behavior: contain;
  -webkit-overflow-scrolling: touch;
}

/* Custom scrollbar for desktop */
@media (min-width: 768px) {
  .overflow-y-auto::-webkit-scrollbar {
    width: 8px;
  }

  .overflow-y-auto::-webkit-scrollbar-track {
    background: transparent;
  }

  .overflow-y-auto::-webkit-scrollbar-thumb {
    background: rgba(125, 211, 252, 0.28);
    border-radius: 4px;
  }

  .overflow-y-auto::-webkit-scrollbar-thumb:hover {
    background: rgba(125, 211, 252, 0.45);
  }
}

/* Slide up transition */
.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.3s ease;
}

.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translate(-50%, 20px);
}

/* Slide fade transition for trial quota banner */
.slide-fade-enter-active {
  transition: all 0.3s ease-out;
}
.slide-fade-leave-active {
  transition: all 0.3s ease-in;
}
.slide-fade-enter-from,
.slide-fade-leave-to {
  opacity: 0;
  transform: translateY(-100%);
  max-height: 0;
  padding-top: 0;
  padding-bottom: 0;
  border-bottom-width: 0;
}

/* Bottom sheet animation for mobile menus */
.sheet-enter-active,
.sheet-leave-active {
  transition: opacity 0.3s ease;
}

.sheet-enter-active > div:last-child,
.sheet-leave-active > div:last-child {
  transition: transform 0.3s ease;
}

.sheet-enter-from,
.sheet-leave-to {
  opacity: 0;
}

.sheet-enter-from > div:last-child,
.sheet-leave-to > div:last-child {
  transform: translateY(100%);
}

.sheet-enter-to,
.sheet-leave-from {
  opacity: 1;
}

.sheet-enter-to > div:last-child,
.sheet-leave-from > div:last-child {
  transform: translateY(0);
}

/* Token change animation */
.token-change-animation {
  animation: token-pulse 0.8s ease-out;
}

@keyframes token-pulse {
  0% {
    transform: scale(1);
    color: inherit;
    text-shadow: none;
  }
  20% {
    transform: scale(1.3);
    color: #ef4444;
    text-shadow:
      0 0 10px rgba(239, 68, 68, 0.8),
      0 0 20px rgba(239, 68, 68, 0.5);
  }
  50% {
    transform: scale(1.15);
    color: #f59e0b;
    text-shadow: 0 0 8px rgba(245, 158, 11, 0.6);
  }
  100% {
    transform: scale(1);
    color: inherit;
    text-shadow: none;
  }
}

/* Trial quota banner pulse when tokens change */
.trial-quota-pulse {
  animation: banner-pulse 0.8s ease-out;
}

@keyframes banner-pulse {
  0%,
  100% {
    background-color: rgb(239 246 255 / var(--tw-bg-opacity, 1));
  }
  30% {
    background-color: rgb(254 243 199 / var(--tw-bg-opacity, 1));
  }
}

:root.dark .trial-quota-pulse {
  animation: banner-pulse-dark 0.8s ease-out;
}

@keyframes banner-pulse-dark {
  0%,
  100% {
    background-color: rgb(30 58 138 / 0.2);
  }
  30% {
    background-color: rgb(180 83 9 / 0.3);
  }
}
</style>
