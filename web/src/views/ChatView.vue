<script setup lang="ts">
import { ref, onMounted, nextTick, watch, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import type { ComponentPublicInstance } from 'vue'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useChatShortcuts } from '@/composables/useKeyboardShortcuts'
import { authFetch } from '@/api/client'
import ConversationList from '@/components/ConversationList.vue'
import ChatMessage from '@/components/ChatMessage.vue'
import ChatInput from '@/components/ChatInput.vue'
import type { FileAttachment } from '@/components/ChatInput.vue'
import PresetQuestions from '@/components/onboarding/PresetQuestions.vue'
import VirtualScroll from '@/components/VirtualScroll.vue'
import TalkMode from '@/components/chat/TalkMode.vue'
import ToolApprovalDialog from '@/components/ToolApprovalDialog.vue'
import ExecApprovalDialog from '@/components/ExecApprovalDialog.vue'
import MediaParamPanel from '@/components/MediaParamPanel.vue'
import AgentTaskPanel from '@/components/AgentTaskPanel.vue'
import { agentApi, type AgentTask, type AgentQuestionAnswer } from '@/api/chat'
import { onSSEEvent, offSSEEvent } from '@/composables/useEventStream'
import { useMediaGenerate } from '@/composables/useMediaGenerate'
import { componentPool } from '@/utils/componentPool'
import { clearConversationIncrementalStates } from '@/utils/typeless'
import { preloadHljs } from '@/utils/markdown'
import { streamingTTSManager } from '@/api/voice'
import { THEME_STYLES, type ThemeStyle } from '@/stores/settings'
import { formatTokens } from '@/utils/format'

const { t, te, locale } = useI18n()
const router = useRouter()
const chatStore = useChatStore()
const settingsStore = useSettingsStore()
const providerPoolStore = useProviderPoolStore()
const mediaGen = useMediaGenerate()
streamingTTSManager.setLocale(locale.value)
watch(locale, (newLocale) => {
  streamingTTSManager.setLocale(newLocale)
})

// Trial quota animation state
const tokenAnimating = ref(false)
const previousTokens = ref<number | null>(null)

// Theme style class for chat interface
const themeStyleClass = computed(() => `theme-style-${settingsStore.themeStyle}`)

const messagesContainer = ref<HTMLElement | null>(null)
const virtualScrollRef = ref<InstanceType<typeof VirtualScroll> | null>(null)
const chatInputRef = ref<InstanceType<typeof ChatInput> | null>(null)
const messageHeightCache = new Map<string, number>()
const virtualElementToMessageKey = new Map<HTMLElement, string>()
const virtualItemElements = new Map<string, HTMLElement>()
const virtualItemHeightUpdaters = new Map<string, (height: number) => void>()
const virtualItemRefCallbacks = new Map<string, (el: Element | ComponentPublicInstance | null) => void>()
let virtualResizeObserver: ResizeObserver | null = null
const showSidebar = ref(false) // Default closed on mobile
// Detect mobile synchronously before first render to avoid layout flash
const _initMobile = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini/i.test(navigator.userAgent)
const _initConvId = new URLSearchParams(window.location.search).get('conversationId')
const isMobile = ref(_initMobile)
const isNarrowScreen = ref(window.innerWidth < 768) // PC narrow screen (<768px)
const isCompact = computed(() => isNarrowScreen.value) // Compact mode = narrow screen
const shouldCollapseTopbarControls = computed(() => isMobile.value || isCompact.value)
const isMac = computed(() => navigator.platform.toUpperCase().indexOf('MAC') >= 0)
// If URL has conversationId on mobile, pre-populate pageStack so we skip list page on first render
const pageStack = ref<string[]>(_initMobile && _initConvId ? [_initConvId] : [])
const showListPage = computed(() => isMobile.value && pageStack.value.length === 0)
const mobileAnimationEnabled = ref(false) // Only animate after user interaction, not on page load
const showTopbarMenu = ref(false)

// Theme style selector state
const showStyleSelector = ref(false)
const styleButtonRef = ref<HTMLElement | null>(null)
const showRoutingMenu = ref(false)
const routingButtonRef = ref<HTMLElement | null>(null)
const routingMenuPosition = ref({ x: 0, y: 0 })

// Context trim indicator with 1-second delay
const showContextTrim = ref(false)
let contextTrimTimer: ReturnType<typeof setTimeout> | null = null
watch(() => chatStore.contextTrimInfo, (info) => {
  if (contextTrimTimer) { clearTimeout(contextTrimTimer); contextTrimTimer = null }
  if (info) {
    // For pruned events, only show when this turn actually saved tokens.
    if (info.type === 'pruned') {
      const saved = (info.tokensBefore ?? 0) - (info.tokensAfter ?? 0)
      if (saved <= 0) {
        showContextTrim.value = false
        return
      }
    }
    // Show after 1s delay (only if still streaming)
    contextTrimTimer = setTimeout(() => {
      if (chatStore.streaming || info) showContextTrim.value = true
    }, 1000)
  } else {
    showContextTrim.value = false
  }
})
// Auto-hide after streaming ends (with a short delay so user can read it)
watch(() => chatStore.streaming, (val) => {
  if (!val && showContextTrim.value) {
    setTimeout(() => { showContextTrim.value = false }, 3000)
  }
})
const styleSelectorPosition = ref({ x: 0, y: 0 })

// Talk mode state
const showTalkMode = ref(false)
watch(showTalkMode, (open) => {
  if (open) {
    streamingTTSManager.stop()
  }
})

// Agent task state
const agentTasks = ref<AgentTask[]>([])

async function fetchAgentTasks() {
  try {
    const resp = await agentApi.listTasks()
    agentTasks.value = resp.data || []
  } catch {
    // Ignore — agent API may not be available
  }
}

async function cancelAgentTask(taskId: string) {
  try {
    await agentApi.cancelTask(taskId)
    await fetchAgentTasks()
  } catch (e) {
    console.error('Failed to cancel agent task:', e)
  }
}

async function deleteAgentTask(taskId: string) {
  try {
    await agentApi.deleteTask(taskId)
    agentTasks.value = agentTasks.value.filter(t => t.id !== taskId)
  } catch (e) {
    console.error('Failed to delete agent task:', e)
  }
}

// SSE listener for real-time agent task updates
function onAgentEvent(data: any) {
  if (!data?.task_id) return
  // Update the matching task in-place, or re-fetch if not found
  const idx = agentTasks.value.findIndex(t => t.id === data.task_id)
  if (idx >= 0) {
    const task = agentTasks.value[idx]!
    if (data.progress !== undefined) task.progress = data.progress
    if (data.event_type === 'task_completed') { task.status = 'completed'; task.result = data.message }
    if (data.event_type === 'task_failed') { task.status = 'failed'; task.error = data.message }
    if (data.event_type === 'task_step_completed' && task.plan?.[data.step_index]) {
      task.plan[data.step_index]!.status = 'completed'
      task.plan[data.step_index]!.output = data.output
      if (data.duration_ms) task.plan[data.step_index]!.completed_at = new Date(Date.now()).toISOString()
    }
    if (data.event_type === 'task_progress' && data.step_index !== undefined && task.plan?.[data.step_index]) {
      if (task.plan[data.step_index]!.status === 'pending') {
        task.plan[data.step_index]!.status = 'running'
        task.plan[data.step_index]!.started_at = new Date(Date.now()).toISOString()
      }
    }
    if (data.event_type === 'task_user_message' && data.message) {
      // Append user message to current step's output so it's visible in the panel
      const stepIdx = task.current_step
      if (task.plan?.[stepIdx]) {
        const prev = task.plan[stepIdx]!.output || ''
        task.plan[stepIdx]!.output = prev + (prev ? '\n' : '') + `[User]: ${data.message}`
      }
    }
    if (data.event_type === 'task_question' && data.questions?.length) {
      task.questions = data.questions
      task.status = 'waiting_input'
    }
    if (data.event_type === 'task_question_answered') {
      task.questions = undefined
      task.status = 'executing'
    }
    // Clear questions when task resumes (any progress/step event after question was answered)
    if (task.questions?.length && (data.event_type === 'task_progress' || data.event_type === 'task_step_completed')) {
      task.questions = undefined
    }
  } else {
    fetchAgentTasks()
  }
}

const agentEventTypes = ['task_created', 'task_planning', 'task_progress', 'task_step_completed', 'task_completed', 'task_failed', 'task_user_message', 'task_question', 'task_question_answered']

// Virtual scroll threshold - use virtual scroll when message count exceeds this
const VIRTUAL_SCROLL_THRESHOLD = 50

// Whether to use virtual scrolling
const useVirtualScroll = computed(() => chatStore.messages.length > VIRTUAL_SCROLL_THRESHOLD)

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
  return chatStore.messages[chatStore.messages.length - 1]?.id ?? null
})

type MessageMemoSource = {
  conversation_id: string
  id: string
  content: string
  attachments?: unknown[]
  updated_at?: string
  created_at: string
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
]

type MessageRenderBindings = {
  isStreaming: boolean
  isLastAssistantMessage: boolean
  disableAutoTTS: boolean
  isMobile: boolean
  isSelected: boolean
  isMultiSelectMode: boolean
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
  const key = `${message.conversation_id}:${message.id}`
  messageRenderMetaKeyCache.set(message, key)
  return key
}

const messageRenderMetaKeyCache = new WeakMap<MessageMemoSource, string>()
let lastRenderMetaLookupKey = ''
let lastRenderMetaLookupMessage: MessageMemoSource | null = null
let lastRenderMetaLookupValue: MessageRenderMeta | null = null

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

  const cached = messageRenderMetaCache.get(cacheKey)
  if (
    cached
    && cached.memoDeps[1] === message.content
    && cached.memoDeps[2] === (message.attachments?.length ?? 0)
    && cached.memoDeps[3] === memoDepsUpdatedAt
    && cached.memoDeps[4] === isStreaming
    && cached.memoDeps[5] === isLastAssistant
    && cached.memoDeps[6] === showTalkMode.value
    && cached.memoDeps[7] === isMobile.value
    && cached.memoDeps[8] === chatStore.isMultiSelectMode
    && cached.memoDeps[9] === isSelected
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
  ]

  const bindings: MessageRenderBindings = {
    isStreaming,
    isLastAssistantMessage: isLastAssistant,
    disableAutoTTS: showTalkMode.value,
    isMobile: isMobile.value,
    isSelected,
    isMultiSelectMode: chatStore.isMultiSelectMode,
  }

  const nextMeta: MessageRenderMeta = {
    isStreaming,
    isLastAssistant,
    isSelected,
    memoDeps: deps,
    bindings,
  }

  if (messageRenderMetaCache.size >= MESSAGE_RENDER_META_CACHE_MAX && !messageRenderMetaCache.has(cacheKey)) {
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

// Provider config dialog state
const showProviderConfigDialog = ref(false)

// Check if Claude Code CLI is enabled (from store, reactive)
const isClaudeCodeEnabled = computed(() => settingsStore.claudeCodeEnabled)

// Provider status computed properties
const hasConfiguredProviders = computed(() => providerPoolStore.enabledProviders.length > 0)
const hasActiveProviders = computed(() => providerPoolStore.activeProviders.length > 0)
const allProvidersFailed = computed(() =>
  hasConfiguredProviders.value && !hasActiveProviders.value &&
  providerPoolStore.enabledProviders.every(p => p.status === 'error')
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
  // Some providers enabled but not yet checked
  return { status: 'pending', color: 'yellow', message: t('chat.providerPending') }
})

const showAwaitingConfirmation = computed(() =>
  chatStore.awaitingConfirmation
  || !!chatStore.pendingQuestion
  || !!chatStore.pendingApproval
  || !!chatStore.pendingExecApproval
)

// Routing mode display info
const routingModeInfo = computed(() => {
  const mode = providerPoolStore.routingMode
  const cloudCount = providerPoolStore.cloudProviders.filter(p => p.status === 'active').length
  const localCount = providerPoolStore.localProviders.filter(p => p.status === 'active').length

  if (mode === 'cloud') {
    return {
      icon: 'cloud',
      label: t('chat.routingMode.cloud'),
      count: cloudCount,
      color: 'gray'
    }
  } else if (mode === 'local') {
    return {
      icon: 'local',
      label: t('chat.routingMode.local'),
      count: localCount,
      color: 'green'
    }
  } else {
    return {
      icon: 'auto',
      label: t('chat.routingMode.auto'),
      count: cloudCount + localCount,
      color: 'accent'
    }
  }
})

const cloudActiveCount = computed(() =>
  providerPoolStore.cloudProviders.filter(p => p.status === 'active').length
)
const localActiveCount = computed(() =>
  providerPoolStore.localProviders.filter(p => p.status === 'active').length
)
const totalActiveProviderCount = computed(() => cloudActiveCount.value + localActiveCount.value)
const isSingleModelMode = computed(() => chatStore.modelPreference !== 'auto')

const fixedModelOptions = computed(() => {
  const enabledProviderIds = new Set(
    providerPoolStore.enabledProviders
      .filter(provider => provider.type !== 'media')
      .map(provider => provider.id)
  )

  const modelIds = new Set<string>()
  for (const model of providerPoolStore.models) {
    if (!model.enabled) continue
    if (!enabledProviderIds.has(model.provider_id)) continue
    modelIds.add(model.id)
  }
  return Array.from(modelIds)
    .sort((a, b) => a.localeCompare(b))
    .map(id => ({ id }))
})

const fixedModelLabel = computed(() => {
  if (chatStore.modelPreference === 'auto') return t('chat.routingMode.highAvailability')
  return chatStore.modelPreference
})

const routingStrategyLabel = computed(() =>
  isSingleModelMode.value ? t('chat.routingMode.fixedModel') : t('chat.routingMode.highAvailability')
)

const currentThemeStyleLabel = computed(() => {
  const current = THEME_STYLES.find(style => style.id === settingsStore.themeStyle)
  if (!current) return settingsStore.themeStyle
  return te(current.labelKey) ? t(current.labelKey) : current.id
})

function themeStyleLabel(style: { id: string; labelKey: string }): string {
  return te(style.labelKey) ? t(style.labelKey) : style.id
}

const currentThemeStyleIconColor = computed(() => {
  switch (settingsStore.themeStyle) {
    case 'bubble':
      return '#a855f7'
    case 'minimal':
      return '#64748b'
    case 'gradient':
      return '#f97316'
    case 'ocean':
      return '#0284c7'
    case 'default':
    default:
      return '#0ea5e9'
  }
})

function getMessageHeightKey(message: { conversation_id: string; id: string }) {
  return `${message.conversation_id}:${message.id}`
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
  updateHeight: (height: number) => void,
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
  const ua = navigator.userAgent.toLowerCase()
  const isMobileUA = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini/i.test(ua)
  isMobile.value = isMobileUA
  isNarrowScreen.value = window.innerWidth < 768
  // Auto-show sidebar on desktop wide screen, hide on narrow
  if (!isMobile.value) {
    showSidebar.value = !isNarrowScreen.value
  }
}

// Keyboard shortcuts
useChatShortcuts({
  onNewChat: () => handleCreateConversation(),
  onFocusInput: () => chatInputRef.value?.focus?.(),
  onToggleSidebar: () => toggleSidebar(),
  onCancelStream: () => chatStore.streaming && handleCancel(),
})

// Scroll to bottom when messages change
let autoScrollRafId: number | null = null
function scheduleScrollToBottom(behavior: 'auto' | 'smooth' = 'auto') {
  if (!isUserNearBottom.value) return
  if (autoScrollRafId !== null) return
  autoScrollRafId = window.requestAnimationFrame(async () => {
    autoScrollRafId = null
    await nextTick()
    if (isUserNearBottom.value) {
      scrollToBottom(behavior)
    }
  })
}

watch(
  () => chatStore.messages.length,
  () => {
    scheduleScrollToBottom('auto')
  }
)

// Also scroll when streaming content updates
watch(
  () => chatStore.streamingContent,
  () => {
    scheduleScrollToBottom('auto')
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

function checkIfNearBottom() {
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
      chatStore.loadMoreMessages().then(() => {
        // Maintain scroll position after loading more
        nextTick(() => {
          if (messagesContainer.value) {
            const newHeight = messagesContainer.value.scrollHeight
            messagesContainer.value.scrollTop = newHeight - previousHeight
          }
        })
      }).finally(() => {
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
  // Load more when scrolled near the top in virtual scroll mode.
  // Add a short cooldown + in-flight lock to avoid duplicate triggers
  // caused by visible range jitter near the boundary.
  if (start >= 5 || !chatStore.hasMoreMessages || chatStore.loadingMore || virtualLoadMoreInFlight) {
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
  })
}

async function handleSend(message: string, attachments?: FileAttachment[]) {
  // Check for media generation intent before sending to chat
  const hasImages = attachments?.some((a) => a.type.startsWith('image/')) || false
  const imageCount = attachments?.filter((a) => a.type.startsWith('image/')).length || 0
  const imageFiles = attachments?.filter((a) => a.type.startsWith('image/')).map((a) => a.file) || []
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

function handleMediaDismiss() {
  // User chose "No, just chat" — send the original message to chat instead.
  const prompt = mediaGen.intent.value?.prompt
  mediaGen.reset()
  if (prompt) {
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
  if (text.trim()) {
    await chatStore.sendMessage(text)
  }
}

function handleCancel() {
  const hadMediaGen = mediaGen.generating.value
  const mediaType = mediaGen.task.value?.type // 'image' | 'video'
  const runningAgents = agentTasks.value.filter(at => at.status === 'executing' || at.status === 'planning' || at.status === 'pending')

  // 1. Cancel chat streaming
  chatStore.cancelStreaming()

  // 2. Cancel active media generation task
  if (hadMediaGen) {
    mediaGen.cancel()
  }

  // 3. Cancel running agent tasks
  for (const at of runningAgents) {
    agentApi.cancelTask(at.id).catch(() => {})
  }

  // 4. Append a stop notification message if any async task was cancelled
  if (hadMediaGen || runningAgents.length > 0) {
    const parts: string[] = []
    if (hadMediaGen) {
      parts.push(t(mediaType === 'video' ? 'chat.videoGenStopped' : 'chat.imageGenStopped'))
    }
    for (const at of runningAgents) {
      parts.push(t('chat.agentTaskStopped', { goal: at.goal }))
    }

    const cleaned = chatStore.messages.filter(m => !m.id.startsWith('streaming-'))
    cleaned.push({
      id: `stop-${Date.now()}`,
      conversation_id: chatStore.currentConversationId || '',
      role: 'assistant',
      content: parts.join('\n'),
      created_at: new Date().toISOString(),
    })
    chatStore.messages = cleaned

    if (runningAgents.length > 0) {
      fetchAgentTasks()
    }
  }
}

function handleInject(message: string) {
  chatStore.injectMessage(message)
}

async function sendAgentMessage(taskId: string, message: string) {
  try {
    await agentApi.sendMessage(taskId, message)
  } catch (e) {
    console.error('Failed to send message to agent task:', e)
  }
}

async function submitAgentAnswer(taskId: string, answers: AgentQuestionAnswer[]) {
  try {
    await agentApi.submitAnswers(taskId, answers)
    // Clear questions locally after successful submit
    const task = agentTasks.value.find(t => t.id === taskId)
    if (task) task.questions = undefined
  } catch (e) {
    console.error('Failed to submit agent answers:', e)
  }
}

async function handleSelectConversation(id: string) {
  if (isMobile.value) {
    mobileAnimationEnabled.value = true
    pageStack.value.push(id)
    // Set query param to trigger AppHeader hide
    await router.push({ query: { conversationId: id } })
  }
  await chatStore.selectConversation(id)
  chatStore.resetWarmup()
  chatInputRef.value?.resetWarmup?.()
}

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
    // Clear query param to show AppHeader
    router.push({ query: {} })
  } else {
    showSidebar.value = !showSidebar.value
  }
}

function toggleRoutingMenu() {
  showTopbarMenu.value = false
  showStyleSelector.value = false
  if (!showRoutingMenu.value && !isMobile.value && routingButtonRef.value) {
    const rect = routingButtonRef.value.getBoundingClientRect()
    const menuWidth = 320 // w-80
    const horizontalPadding = 8
    const x = Math.min(
      Math.max(horizontalPadding, rect.right - menuWidth),
      window.innerWidth - menuWidth - horizontalPadding
    )
    const y = rect.bottom + 8
    routingMenuPosition.value = { x, y }
  }
  showRoutingMenu.value = !showRoutingMenu.value
}

function selectRoutingMode(mode: 'auto' | 'cloud' | 'local') {
  if (mode === 'cloud' && !providerPoolStore.hasCloudProviders) return
  if (mode === 'local' && !providerPoolStore.hasLocalProviders) return
  providerPoolStore.setRoutingMode(mode)
  if (isMobile.value) showRoutingMenu.value = false
}

function selectFixedModel(modelId: string) {
  chatStore.setModelPreference(modelId)
  if (isMobile.value) showRoutingMenu.value = false
}

function setAutoModelPreference() {
  chatStore.setModelPreference('auto')
  if (isMobile.value) showRoutingMenu.value = false
}

function enableSingleModelMode() {
  if (chatStore.modelPreference !== 'auto') return
  const firstModel = fixedModelOptions.value[0]?.id
  if (firstModel) {
    chatStore.setModelPreference(firstModel)
  }
}

// Theme style selector functions
function toggleStyleSelector() {
  showTopbarMenu.value = false
  showRoutingMenu.value = false
  // On mobile, no need to calculate position (bottom sheet)
  if (!showStyleSelector.value && !isMobile.value && styleButtonRef.value) {
    const rect = styleButtonRef.value.getBoundingClientRect()
    styleSelectorPosition.value = {
      x: rect.right,
      y: rect.bottom + 8
    }
  }
  showStyleSelector.value = !showStyleSelector.value
}

function selectThemeStyle(style: ThemeStyle) {
  settingsStore.setThemeStyle(style)
  showStyleSelector.value = false
  showTopbarMenu.value = false
}

function toggleDeepResearch() {
  chatStore.setDeepResearchEnabled(!chatStore.deepResearchEnabled)
}

function toggleAgentMode() {
  settingsStore.setAgentMode(!settingsStore.agentMode)
}

function toggleToolDetails() {
  settingsStore.setShowToolDetails(!settingsStore.showToolDetails)
}

function toggleTopbarMenu() {
  showRoutingMenu.value = false
  showStyleSelector.value = false
  showTopbarMenu.value = !showTopbarMenu.value
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement

  if (!target.closest('.topbar-more-container') && !target.closest('.topbar-sheet')) {
    showTopbarMenu.value = false
  }
  if (!target.closest('.routing-menu-container') && !target.closest('.routing-menu-floating')) {
    showRoutingMenu.value = false
  }

  // On mobile, style sheets handle their own click-outside via backdrop
  if (isMobile.value) {
    return
  }
  // Close style selector when clicking outside
  if (!target.closest('.style-selector-container')) {
    showStyleSelector.value = false
  }
  // Close context menu when clicking outside
  if (!target.closest('.context-menu') && !contextMenuJustOpened.value) {
    showContextMenu.value = false
  }
}

// Context menu handlers
const contextMenuJustOpened = ref(false)

function handleMessageContextMenu(event: MouseEvent, messageId: string) {
  event.preventDefault()

  // On mobile, context menu is handled by ChatMessage's own long-press menu
  if (isMobile.value) return

  contextMenuMessageId.value = messageId
  contextMenuPosition.value = { x: event.clientX, y: event.clientY }
  showContextMenu.value = true
  // Prevent the subsequent click event from immediately closing the menu
  contextMenuJustOpened.value = true
  setTimeout(() => { contextMenuJustOpened.value = false }, 200)
}

function handleSelectMessage() {
  if (contextMenuMessageId.value) {
    chatStore.enterMultiSelectMode(contextMenuMessageId.value)
  }
  showContextMenu.value = false
}

// Check if the context-menu'd message is the last assistant message (for Continue/Regenerate)
const isContextMenuLastAssistant = computed(() => {
  return !!contextMenuMessageId.value && contextMenuMessageId.value === lastAssistantMessageId.value
})

function handleContextContinue() {
  chatStore.continueMessage()
  showContextMenu.value = false
}

function handleContextRegenerate() {
  chatStore.regenerateMessage()
  showContextMenu.value = false
}

function handleMessageContinue() {
  chatStore.continueMessage()
}

function handleMessageRegenerate() {
  chatStore.regenerateMessage()
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

// Handle preset question selection - check provider availability first
async function handlePresetQuestionSelect(text: string, attachments?: FileAttachment[]) {
  // Ensure providers and trial quota are loaded
  if (providerPoolStore.providers.length === 0) {
    await providerPoolStore.fetchProviders()
  }
  if (!providerPoolStore.trialQuota) {
    await providerPoolStore.fetchTrialQuota()
  }

  // Check if user has configured any providers
  const hasUserProviders = providerPoolStore.enabledProviders.length > 0
  // Check if trial quota is available and not exhausted
  const trialAvailable = providerPoolStore.trialQuota && !providerPoolStore.trialQuota.is_exhausted

  // If no user providers and no trial quota, show dialog to guide user
  if (!hasUserProviders && !trialAvailable) {
    showProviderConfigDialog.value = true
    return
  }

  await handleSend(text, attachments)
}

onMounted(async () => {
  // Preload syntax highlighting only once for chat page.
  preloadHljs().catch(() => {})

  checkMobile()
  window.addEventListener('resize', checkMobile)
  document.addEventListener('click', handleClickOutside)

  // Preload common card components for better UX
  componentPool.preload(['progress', 'chart', 'gallery', 'link', 'file', 'deep-research', 'deep-research-progress'])

  // Initialize speech services lazily (TTS/STT)
  authFetch('/api/v1/speech/init', { method: 'POST' }).catch(() => {})

  // Fetch agent tasks (non-blocking) + subscribe to SSE updates
  fetchAgentTasks()
  for (const evt of agentEventTypes) onSSEEvent(evt, onAgentEvent)

  // Fetch conversations and (if URL has conversationId) messages in parallel.
  // selectConversation only needs the ID, not the conversation list.
  if (_initConvId) {
    await Promise.all([
      chatStore.fetchConversations(),
      chatStore.selectConversation(_initConvId),
    ])
  } else {
    await chatStore.fetchConversations()
    // Auto-select first conversation if available and none selected (desktop only)
    if (!isMobile.value && !chatStore.currentConversationId && chatStore.sortedConversations.length > 0 && chatStore.sortedConversations[0]) {
      await chatStore.selectConversation(chatStore.sortedConversations[0].id)
    }
  }

  // Fire secondary data fetches in background — skip if App.vue already loaded them
  if (providerPoolStore.providers.length === 0) {
    providerPoolStore.fetchProviders().then(() => {
      const llmProviders = providerPoolStore.providers.filter((p: any) => p.type !== 'media')
      settingsStore.updateFromPoolProviders(llmProviders)
    }).catch(() => {})
  }
  settingsStore.fetchTools().catch(() => {})
  providerPoolStore.fetchRoutingMode().catch(() => {})

  // Restore pending confirmations after refresh or missed SSE events.
  chatStore.recoverPendingConfirmations(false)
})

onUnmounted(() => {
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
  clearVirtualItemObservers()
  if (autoScrollRafId !== null) {
    window.cancelAnimationFrame(autoScrollRafId)
    autoScrollRafId = null
  }
  window.removeEventListener('resize', checkMobile)
  document.removeEventListener('click', handleClickOutside)
  for (const evt of agentEventTypes) offSSEEvent(evt, onAgentEvent)
})
</script>

<template>
  <div class="chat-view ui-density-standard flex relative" :class="[themeStyleClass, { 'mobile-view': isMobile && !showListPage }]">
    <!-- Overlay for narrow screen sidebar -->
    <div
      v-if="!isMobile && isNarrowScreen && showSidebar"
      class="fixed inset-0 bg-black/60 backdrop-blur-sm z-30"
      @click="toggleSidebar"
    />

    <!-- Sidebar / Conversation List -->
    <aside
      v-show="isMobile ? showListPage : showSidebar"
      class="conversation-sidebar chat-sidebar-shell flex-shrink-0 border-r border-glass-border transition-transform duration-300 glass-sidebar"
      :class="{
        'w-[22rem]': !isMobile,
        'z-40': !isMobile,
        'fixed left-0 top-0 h-full': !isMobile && isNarrowScreen,
        '-translate-x-full': !isMobile && isNarrowScreen && !showSidebar,
        'mobile-list-page': isMobile && showListPage,
      }"
    >
      <ConversationList
        :conversations="chatStore.sortedConversations"
        :current-id="chatStore.currentConversationId"
        :loading="chatStore.loading"
        :searching="chatStore.searching"
        @select="handleSelectConversation"
        @create="handleCreateConversation"
        @delete="handleDeleteConversation"
        @search="handleSearch"
        @pin="handlePinConversation"
        @unpin="handleUnpinConversation"
      />
    </aside>

    <!-- Main chat area -->
    <main
      :class="isMobile ? ['mobile-chat', { 'mobile-chat-offscreen': showListPage, 'mobile-chat-animated': mobileAnimationEnabled }] : ''"
      class="flex-1 flex flex-col min-w-0 min-h-0 h-full relative"
      @dragover.prevent="chatInputRef?.handleDragOver($event)"
      @dragleave="chatInputRef?.handleDragLeave()"
      @drop.prevent="chatInputRef?.handleDrop($event)"
    >
      <!-- Chat header -->
      <header class="chat-topbar relative z-50 flex items-center justify-between px-4 py-3 sm:p-4 border-b border-gray-200 dark:border-glass-border gap-2"
        :class="isMobile ? 'bg-white dark:bg-gray-900' : 'glass-header'"
      >
        <div class="chat-title-wrap flex items-center gap-2 sm:gap-3 min-w-0 flex-1">
          <!-- Back/Sidebar toggle button -->
          <button
            class="chat-nav-btn flex-shrink-0 p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white rounded-lg transition-all duration-200 cursor-pointer"
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
          <h2 class="chat-title text-base sm:text-lg font-semibold text-gray-900 dark:text-white truncate">
            {{ chatStore.currentConversation?.title || t('chat.newChat') }}
          </h2>
          <!-- Enhanced Mode badge -->
          <span
            v-if="isClaudeCodeEnabled"
            class="relative group hidden sm:inline-flex items-center gap-1 text-xs font-medium text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/30 rounded-full flex-shrink-0 cursor-default"
            :class="isCompact || isMobile ? 'px-1.5 py-0.5' : 'px-2 py-0.5'"
          >
            <svg :class="isCompact || isMobile ? 'w-2.5 h-2.5' : 'w-3 h-3'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z"/>
            </svg>
            <span v-if="!isCompact && !isMobile">{{ t('chat.enhancedMode') }}</span>
            <!-- Tooltip (positioned right to avoid sidebar clipping) -->
            <span class="pointer-events-none absolute left-full top-1/2 -translate-y-1/2 ml-2 w-max max-w-xs px-3 py-2 rounded-lg text-xs font-normal text-white bg-gray-800 dark:bg-gray-700 shadow-lg opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50">
              {{ t('chat.enhancedModeDesc') }}
            </span>
          </span>
          <!-- Enable Enhanced Mode prompt -->
          <router-link
            v-else
            to="/settings?tab=llm#claude-code-settings"
            class="relative group hidden sm:inline-flex items-center gap-1 text-xs font-medium text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full flex-shrink-0 transition-colors cursor-pointer"
            :class="isCompact || isMobile ? 'px-1.5 py-0.5' : 'px-2 py-0.5'"
          >
            <svg :class="isCompact || isMobile ? 'w-2.5 h-2.5' : 'w-3 h-3'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z"/>
            </svg>
            <span v-if="!isCompact && !isMobile">{{ t('chat.enableEnhancedMode') }}</span>
            <!-- Tooltip (positioned right to avoid sidebar clipping) -->
            <span class="pointer-events-none absolute left-full top-1/2 -translate-y-1/2 ml-2 w-max max-w-xs px-3 py-2 rounded-lg text-xs font-normal text-white bg-gray-800 dark:bg-gray-700 shadow-lg opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50">
              {{ t('chat.enableEnhancedModeDesc') }}
            </span>
          </router-link>
        </div>

        <!-- Routing Mode Switch & Provider Status -->
        <div class="chat-tools ml-auto flex items-center gap-2 flex-shrink-0">
          <button
            v-if="!shouldCollapseTopbarControls"
            class="topbar-icon-btn p-2 rounded-lg transition-colors cursor-pointer"
            :class="chatStore.deepResearchEnabled
              ? 'text-emerald-500 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-900/30'
              : 'text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white'"
            :title="t('ui.deepResearchTitle')"
            @click="toggleDeepResearch"
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 6.253v13m0-13C10.832 5.483 9.246 5 7.5 5S4.168 5.483 3 6.253v13C4.168 18.483 5.754 18 7.5 18s3.332.483 4.5 1.253m0-13C13.168 5.483 14.754 5 16.5 5s3.332.483 4.5 1.253v13C19.832 18.483 18.246 18 16.5 18s-3.332.483-4.5 1.253" />
            </svg>
          </button>

          <!-- Agent mode toggle (click: on/off, right-click: toggle auto-confirm) -->
          <div class="relative">
            <button
              v-if="!shouldCollapseTopbarControls"
              class="topbar-icon-btn p-2 rounded-lg transition-colors"
              :class="settingsStore.agentMode
                ? 'text-blue-500 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30'
                : 'text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white'"
              :title="settingsStore.agentMode
                ? (settingsStore.agentAutoConfirm ? t('agent.mode') + ' (Auto)' : t('agent.mode') + ' (Confirm)')
                : t('agent.mode')"
              @click="settingsStore.setAgentMode(!settingsStore.agentMode)"
              @contextmenu.prevent="settingsStore.agentMode && settingsStore.setAgentAutoConfirm(!settingsStore.agentAutoConfirm)"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
            </button>
            <!-- Auto-confirm dot indicator -->
            <span
              v-if="settingsStore.agentMode && settingsStore.agentAutoConfirm"
              class="absolute top-0.5 right-0.5 w-2 h-2 bg-orange-400 rounded-full pointer-events-none"
            />
          </div>

          <!-- Tool details toggle (brain icon) -->
          <button
            v-if="!shouldCollapseTopbarControls"
            class="topbar-icon-btn p-2 rounded-lg transition-colors cursor-pointer"
            :class="settingsStore.showToolDetails
              ? 'text-purple-500 dark:text-purple-400 bg-purple-50 dark:bg-purple-900/30'
              : 'text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white'"
            :title="settingsStore.showToolDetails ? t('chat.hideToolDetails') : t('chat.showToolDetails')"
            @click="settingsStore.setShowToolDetails(!settingsStore.showToolDetails)"
          >
            <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <!-- Brain shape -->
              <path d="M12 2a5 5 0 00-4.78 3.56A3.5 3.5 0 004 9a3.5 3.5 0 00.68 2.07A3.5 3.5 0 004 13.5 3.5 3.5 0 006.5 17h.28A5 5 0 0012 20" />
              <path d="M12 2a5 5 0 014.78 3.56A3.5 3.5 0 0120 9a3.5 3.5 0 01-.68 2.07A3.5 3.5 0 0120 13.5a3.5 3.5 0 01-2.5 3.5h-.28A5 5 0 0112 20" />
              <path d="M12 2v18" />
            </svg>
          </button>

          <!-- Routing mode + model preference (dropdown) -->
          <div class="routing-menu-container relative">
            <button
              v-if="!isMobile"
              ref="routingButtonRef"
              class="routing-trigger-btn topbar-icon-btn p-2 rounded-lg transition-colors cursor-pointer"
              :class="{
                'is-error': providerStatus.status === 'error',
                'is-pending': providerStatus.status === 'pending',
                'is-none': providerStatus.status === 'none',
              }"
              :title="`${t('chat.routingMode.title')} · ${routingModeInfo.label} · ${fixedModelLabel}`"
              @click.stop="toggleRoutingMenu"
            >
              <svg v-if="routingModeInfo.icon === 'cloud'" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
              </svg>
              <svg v-else-if="routingModeInfo.icon === 'local'" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
              <svg v-else class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
          </div>

          <!-- Theme style selector -->
          <div class="style-selector-container relative">
            <button
              v-if="!shouldCollapseTopbarControls"
              ref="styleButtonRef"
              class="routing-trigger-btn topbar-icon-btn p-2 rounded-lg transition-colors cursor-pointer"
              :title="t('theme.styles.title')"
              @click.stop="toggleStyleSelector"
            >
              <svg class="h-5 w-5" :style="{ color: currentThemeStyleIconColor }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
              </svg>
            </button>
          </div>

          <!-- Mobile: collapsed topbar actions trigger -->
          <div v-if="shouldCollapseTopbarControls" class="topbar-more-container relative">
            <button
              class="topbar-icon-btn p-2 rounded-lg text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white transition-colors cursor-pointer"
              :class="{ 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-white': showTopbarMenu }"
              :title="t('chat.moreActions')"
              @click.stop="toggleTopbarMenu"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M4 5a1 1 0 011-1h5a1 1 0 011 1v5a1 1 0 01-1 1H5a1 1 0 01-1-1V5zm9 0a1 1 0 011-1h5a1 1 0 011 1v5a1 1 0 01-1 1h-5a1 1 0 01-1-1V5zM4 14a1 1 0 011-1h5a1 1 0 011 1v5a1 1 0 01-1 1H5a1 1 0 01-1-1v-5zm9 0a1 1 0 011-1h5a1 1 0 011 1v5a1 1 0 01-1 1h-5a1 1 0 01-1-1v-5z" />
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
            class="routing-menu-floating fixed w-80 rounded-2xl shadow-2xl border overflow-hidden z-[20000] bg-white dark:bg-slate-900 border-gray-300 dark:border-slate-500"
            :style="{ left: `${routingMenuPosition.x}px`, top: `${routingMenuPosition.y}px` }"
          >
            <div class="px-3 py-2 border-b border-gray-200 dark:border-slate-700">
              <div class="text-xs font-medium text-gray-700 dark:text-slate-300">
                {{ providerStatus.message }}
              </div>
            </div>

            <router-link
              v-if="providerStatus.status === 'none'"
              to="/settings?tab=llm"
              class="routing-manage-row w-full px-4 py-3 flex items-center justify-between text-sm text-gray-900 dark:text-slate-100 transition-colors"
              @click="showRoutingMenu = false"
            >
              <span class="inline-flex items-center gap-2">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5h2m-6 0h2m6 0h2m-5 0v2m0 10v2m0-2h2m-2 0h-2m6-10a2 2 0 012 2v8a2 2 0 01-2 2H7a2 2 0 01-2-2V9a2 2 0 012-2h10z" />
                </svg>
                {{ t('chat.addProvider') }}
              </span>
              <span class="text-xs text-gray-600 dark:text-slate-400">{{ t('chat.manageProviders') }}</span>
            </router-link>

            <template v-else>
              <div class="p-3 space-y-1.5">
                <button
                  class="routing-option-row w-full"
                  :class="{ 'is-active': providerPoolStore.routingMode === 'auto' }"
                  @click="selectRoutingMode('auto')"
                >
                  <div class="routing-option-main">
                    <svg class="routing-option-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                    <div class="routing-option-copy">
                      <div class="routing-option-title">{{ t('chat.routingMode.auto') }}</div>
                      <div class="routing-option-desc">{{ t('chat.routingMode.autoDesc') }}</div>
                    </div>
                  </div>
                  <div class="routing-option-side">
                    <span class="routing-option-count">{{ totalActiveProviderCount }}</span>
                    <span v-if="providerPoolStore.routingMode === 'auto'" class="routing-option-status-chip">
                      {{ routingStrategyLabel }}
                    </span>
                  </div>
                </button>

                <button
                  class="routing-option-row w-full"
                  :class="{
                    'is-active': providerPoolStore.routingMode === 'cloud',
                    'is-disabled': !providerPoolStore.hasCloudProviders,
                  }"
                  :disabled="!providerPoolStore.hasCloudProviders"
                  @click="selectRoutingMode('cloud')"
                >
                  <div class="routing-option-main">
                    <svg class="routing-option-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
                    </svg>
                    <div class="routing-option-copy">
                      <div class="routing-option-title">{{ t('chat.routingMode.cloud') }}</div>
                      <div class="routing-option-desc">{{ t('chat.routingMode.cloudDesc') }}</div>
                    </div>
                  </div>
                  <div class="routing-option-side">
                    <span class="routing-option-count">{{ cloudActiveCount }}</span>
                    <span v-if="providerPoolStore.routingMode === 'cloud'" class="routing-option-status-chip">
                      {{ routingStrategyLabel }}
                    </span>
                  </div>
                </button>

                <button
                  class="routing-option-row w-full"
                  :class="{
                    'is-active': providerPoolStore.routingMode === 'local',
                    'is-disabled': !providerPoolStore.hasLocalProviders,
                  }"
                  :disabled="!providerPoolStore.hasLocalProviders"
                  @click="selectRoutingMode('local')"
                >
                  <div class="routing-option-main">
                    <svg class="routing-option-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                    </svg>
                    <div class="routing-option-copy">
                      <div class="routing-option-title">{{ t('chat.routingMode.local') }}</div>
                      <div class="routing-option-desc">{{ t('chat.routingMode.localDesc') }}</div>
                    </div>
                  </div>
                  <div class="routing-option-side">
                    <span class="routing-option-count">{{ localActiveCount }}</span>
                    <span v-if="providerPoolStore.routingMode === 'local'" class="routing-option-status-chip">
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
                <div v-if="!isSingleModelMode" class="mt-2 text-xs text-gray-600 dark:text-slate-400">
                  {{ t('chat.routingMode.modelAuto') }}
                </div>
                <div v-else class="mt-2 max-h-40 overflow-y-auto space-y-1">
                  <button
                    v-for="option in fixedModelOptions"
                    :key="option.id"
                    class="routing-model-row w-full"
                    :class="{ 'is-selected': chatStore.modelPreference === option.id }"
                    @click="selectFixedModel(option.id)"
                  >
                    <span class="truncate pr-3" :title="option.id">{{ option.id }}</span>
                    <svg v-if="chatStore.modelPreference === option.id" class="w-4 h-4 text-emerald-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
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
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.983 5.5a1.75 1.75 0 0 1 3.034 0l.25.433a1.75 1.75 0 0 0 1.514.875h.5a1.75 1.75 0 0 1 1.517 2.625l-.25.433a1.75 1.75 0 0 0 0 1.75l.25.433a1.75 1.75 0 0 1-1.517 2.625h-.5a1.75 1.75 0 0 0-1.514.875l-.25.433a1.75 1.75 0 0 1-3.034 0l-.25-.433a1.75 1.75 0 0 0-1.514-.875h-.5a1.75 1.75 0 0 1-1.517-2.625l.25-.433a1.75 1.75 0 0 0 0-1.75l-.25-.433a1.75 1.75 0 0 1 1.517-2.625h.5a1.75 1.75 0 0 0 1.514-.875l.25-.433Z" />
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.5 12a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0Z" />
                  </svg>
                  {{ t('chat.manageProviders') }}
                </span>
                <span class="text-xs text-gray-500 dark:text-slate-500">→</span>
              </router-link>
            </template>
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
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('chat.routingMode.title') }}</h3>
              </div>
              <div class="p-4 space-y-2">
                <router-link
                  v-if="providerStatus.status === 'none'"
                  to="/settings?tab=llm"
                  class="w-full px-4 py-3 flex items-center justify-between rounded-xl text-sm text-gray-800 dark:text-slate-100 border border-gray-200 dark:border-slate-600 bg-gray-50 dark:bg-slate-800"
                  @click="showRoutingMenu = false"
                >
                  <span>{{ t('chat.addProvider') }}</span>
                  <span class="text-xs text-gray-600 dark:text-slate-300">{{ t('chat.manageProviders') }}</span>
                </router-link>

                <template v-else>
                  <div class="text-xs text-gray-700 dark:text-slate-300 px-1 pb-1">
                    {{ providerStatus.message }}
                  </div>

                  <button
                    class="routing-option-row w-full"
                    :class="{ 'is-active': providerPoolStore.routingMode === 'auto' }"
                    @click="selectRoutingMode('auto')"
                  >
                    <div class="routing-option-main">
                      <svg class="routing-option-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                      </svg>
                      <div class="routing-option-copy">
                        <div class="routing-option-title">{{ t('chat.routingMode.auto') }}</div>
                        <div class="routing-option-desc">{{ t('chat.routingMode.autoDesc') }}</div>
                      </div>
                    </div>
                    <div class="routing-option-side">
                      <span class="routing-option-count">{{ totalActiveProviderCount }}</span>
                      <span v-if="providerPoolStore.routingMode === 'auto'" class="routing-option-status-chip">
                        {{ routingStrategyLabel }}
                      </span>
                    </div>
                  </button>

                  <button
                    class="routing-option-row w-full"
                    :class="{
                      'is-active': providerPoolStore.routingMode === 'cloud',
                      'is-disabled': !providerPoolStore.hasCloudProviders,
                    }"
                    :disabled="!providerPoolStore.hasCloudProviders"
                    @click="selectRoutingMode('cloud')"
                  >
                    <div class="routing-option-main">
                      <svg class="routing-option-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
                      </svg>
                      <div class="routing-option-copy">
                        <div class="routing-option-title">{{ t('chat.routingMode.cloud') }}</div>
                        <div class="routing-option-desc">{{ t('chat.routingMode.cloudDesc') }}</div>
                      </div>
                    </div>
                    <div class="routing-option-side">
                      <span class="routing-option-count">{{ cloudActiveCount }}</span>
                      <span v-if="providerPoolStore.routingMode === 'cloud'" class="routing-option-status-chip">
                        {{ routingStrategyLabel }}
                      </span>
                    </div>
                  </button>

                  <button
                    class="routing-option-row w-full"
                    :class="{
                      'is-active': providerPoolStore.routingMode === 'local',
                      'is-disabled': !providerPoolStore.hasLocalProviders,
                    }"
                    :disabled="!providerPoolStore.hasLocalProviders"
                    @click="selectRoutingMode('local')"
                  >
                    <div class="routing-option-main">
                      <svg class="routing-option-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                      </svg>
                      <div class="routing-option-copy">
                        <div class="routing-option-title">{{ t('chat.routingMode.local') }}</div>
                        <div class="routing-option-desc">{{ t('chat.routingMode.localDesc') }}</div>
                      </div>
                    </div>
                    <div class="routing-option-side">
                      <span class="routing-option-count">{{ localActiveCount }}</span>
                      <span v-if="providerPoolStore.routingMode === 'local'" class="routing-option-status-chip">
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

                  <div v-if="!isSingleModelMode" class="px-1 pt-2 text-xs text-gray-600 dark:text-slate-400">
                    {{ t('chat.routingMode.modelAuto') }}
                  </div>
                  <div v-else class="max-h-44 overflow-y-auto space-y-1 pt-2">
                    <button
                      v-for="option in fixedModelOptions"
                      :key="option.id"
                      class="routing-model-row w-full"
                      :class="{ 'is-selected': chatStore.modelPreference === option.id }"
                      @click="selectFixedModel(option.id)"
                    >
                      <span class="truncate pr-3" :title="option.id">{{ option.id }}</span>
                      <svg v-if="chatStore.modelPreference === option.id" class="w-4 h-4 text-emerald-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
                      </svg>
                    </button>
                  </div>

                  <router-link
                    to="/settings?tab=llm"
                    class="routing-manage-row w-full px-4 py-3 mt-1 flex items-center justify-between rounded-xl text-sm text-gray-800 dark:text-slate-200 border border-gray-200 dark:border-slate-600 bg-gray-50 dark:bg-slate-800"
                    @click="showRoutingMenu = false"
                  >
                    <span class="inline-flex items-center gap-2">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.983 5.5a1.75 1.75 0 0 1 3.034 0l.25.433a1.75 1.75 0 0 0 1.514.875h.5a1.75 1.75 0 0 1 1.517 2.625l-.25.433a1.75 1.75 0 0 0 0 1.75l.25.433a1.75 1.75 0 0 1-1.517 2.625h-.5a1.75 1.75 0 0 0-1.514.875l-.25.433a1.75 1.75 0 0 1-3.034 0l-.25-.433a1.75 1.75 0 0 0-1.514-.875h-.5a1.75 1.75 0 0 1-1.517-2.625l.25-.433a1.75 1.75 0 0 0 0-1.75l-.25-.433a1.75 1.75 0 0 1 1.517-2.625h.5a1.75 1.75 0 0 0 1.514-.875l.25-.433Z" />
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.5 12a1.5 1.5 0 1 1-3 0 1.5 1.5 0 0 1 3 0Z" />
                      </svg>
                      {{ t('chat.manageProviders') }}
                    </span>
                    <span class="text-xs text-gray-500 dark:text-slate-400">→</span>
                  </router-link>
                </template>
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
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('chat.moreActions') }}</h3>
              </div>
              <div class="p-4">
                <div class="grid grid-cols-2 gap-3">
                  <button
                    class="quick-action-tile"
                    :class="{ 'is-active': chatStore.deepResearchEnabled }"
                    @click="toggleDeepResearch"
                  >
                    <div class="flex items-center justify-between">
                      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 6.253v13m0-13C10.832 5.483 9.246 5 7.5 5S4.168 5.483 3 6.253v13C4.168 18.483 5.754 18 7.5 18s3.332.483 4.5 1.253m0-13C13.168 5.483 14.754 5 16.5 5s3.332.483 4.5 1.253v13C19.832 18.483 18.246 18 16.5 18s-3.332.483-4.5 1.253" />
                      </svg>
                      <span class="quick-action-pill">{{ chatStore.deepResearchEnabled ? t('common.enabled') : t('common.disabled') }}</span>
                    </div>
                    <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">{{ t('ui.deepResearchTitle') }}</div>
                  </button>

                  <button
                    class="quick-action-tile"
                    :class="{ 'is-active': settingsStore.agentMode }"
                    @click="toggleAgentMode"
                  >
                    <div class="flex items-center justify-between">
                      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                      </svg>
                      <span class="quick-action-pill">{{ settingsStore.agentMode ? t('common.enabled') : t('common.disabled') }}</span>
                    </div>
                    <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">{{ t('agent.mode') }}</div>
                  </button>

                  <button
                    class="quick-action-tile"
                    :class="{ 'is-active': settingsStore.showToolDetails }"
                    @click="toggleToolDetails"
                  >
                    <div class="flex items-center justify-between">
                      <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M12 2a5 5 0 00-4.78 3.56A3.5 3.5 0 004 9a3.5 3.5 0 00.68 2.07A3.5 3.5 0 004 13.5 3.5 3.5 0 006.5 17h.28A5 5 0 0012 20" />
                        <path d="M12 2a5 5 0 014.78 3.56A3.5 3.5 0 0120 9a3.5 3.5 0 01-.68 2.07A3.5 3.5 0 0120 13.5a3.5 3.5 0 01-2.5 3.5h-.28A5 5 0 0112 20" />
                        <path d="M12 2v18" />
                      </svg>
                      <span class="quick-action-pill">{{ settingsStore.showToolDetails ? t('common.enabled') : t('common.disabled') }}</span>
                    </div>
                    <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">{{ t('chat.showToolDetails') }}</div>
                  </button>

                  <button
                    class="quick-action-tile"
                    @click="toggleRoutingMenu"
                  >
                    <div class="flex items-center justify-between">
                      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                      </svg>
                      <span class="quick-action-pill">{{ routingModeInfo.label }}</span>
                    </div>
                    <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">{{ t('chat.routingMode.title') }}</div>
                  </button>

                  <button
                    class="quick-action-tile"
                    @click="toggleStyleSelector"
                  >
                    <div class="flex items-center justify-between">
                      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
                      </svg>
                      <span class="quick-action-pill">{{ currentThemeStyleLabel }}</span>
                    </div>
                    <div class="mt-2 text-sm font-semibold text-gray-800 dark:text-slate-100">{{ t('theme.styles.title') }}</div>
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
          v-if="providerPoolStore.trialQuota && providerPoolStore.trialProviders?.length > 0 && !providerPoolStore.trialQuota.is_exhausted"
          class="px-4 py-2 flex items-center justify-between text-sm border-b bg-gray-100 dark:bg-gray-700/20 border-gray-200 dark:border-gray-700 text-gray-700 dark:text-white"
          :class="{ 'trial-quota-pulse': tokenAnimating }"
        >
          <div class="flex items-center gap-2">
            <span :class="{ 'animate-bounce': tokenAnimating }">🎁</span>
            <span
              class="tabular-nums transition-all duration-300 font-medium"
              :class="{ 'token-change-animation': tokenAnimating }"
              :title="providerPoolStore.trialQuota.tokens_remaining.toLocaleString() + ' tokens'"
            >{{ t('chat.trialQuota.remaining', { tokens: formatTokens(providerPoolStore.trialQuota.tokens_remaining) }) }}</span>
          </div>
          <div class="flex items-center gap-2">
            <div class="relative w-20 h-4 bg-gray-300 dark:bg-gray-700 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500 ease-out bg-gray-700 dark:bg-gray-400"
                :style="{ width: `${Math.max(3, Math.min(100, (providerPoolStore.trialQuota.tokens_remaining / providerPoolStore.trialQuota.token_limit) * 100))}%` }"
              ></div>
              <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-white drop-shadow-sm">
                {{ Math.round((providerPoolStore.trialQuota.tokens_remaining / providerPoolStore.trialQuota.token_limit) * 100) }}%
              </span>
            </div>
            <router-link
              to="/settings?tab=llm"
              class="text-xs underline hover:no-underline"
            >
              {{ t('chat.trialQuota.configure') }}
            </router-link>
          </div>
        </div>
      </Transition>

      <!-- Messages area -->
      <div
        ref="messagesContainer"
        class="flex-1 min-h-0 overflow-y-auto overscroll-contain chat-messages-area bg-surface-base"
        :class="isMobile ? 'pb-4' : 'pb-32'"
        @scroll.passive="handleScroll"
      >
        <!-- Load more indicator -->
        <div
          v-if="chatStore.loadingMore"
          class="flex justify-center py-4"
        >
          <div class="flex items-center gap-2 text-gray-500 dark:text-slate-400 text-sm">
            <div class="w-4 h-4 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full animate-spin"></div>
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

        <!-- Empty state -->
        <div
          v-if="chatStore.messages.length === 0 && !chatStore.loading"
          class="h-full flex flex-col items-center justify-center p-4"
        >
          <div class="text-center text-gray-500 dark:text-slate-400 max-w-md mb-8">
            <img src="/logo.svg" alt="Logo" class="w-12 h-12 sm:w-14 sm:h-14 mx-auto mb-4 opacity-80 dark:opacity-60" />
            <h3 class="text-lg sm:text-xl font-semibold text-gray-900 dark:text-white mb-1">{{ t('chat.startConversation') }}</h3>
            <p class="text-sm text-gray-500 dark:text-slate-400">{{ t('chat.startConversationDesc') }}</p>
          </div>

          <!-- Preset Questions -->
          <PresetQuestions @select="handlePresetQuestionSelect" />

          <div v-if="!isMobile" class="mt-8 text-xs text-gray-400 dark:text-slate-500 text-center">
            <p class="font-medium mb-2">{{ t('chat.keyboardShortcuts') }}:</p>
            <p class="space-x-4">
              <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">{{ isMac ? '⌘N' : 'Alt+N' }}</span> {{ t('chat.newChatShortcut') }}
              <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">/</span> {{ t('chat.focusInputShortcut') }}
              <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">{{ isMac ? '⌘B' : 'Alt+B' }}</span> {{ t('chat.toggleSidebarShortcut') }}
            </p>
            <p class="mt-3 text-gray-400 dark:text-slate-500">
              <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">Enter</span> {{ t('chat.sendMessage') }}
              <span class="ml-4 px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">Shift+Enter</span> {{ t('chat.newLine') }}
            </p>
            <p class="mt-2 text-gray-400 dark:text-slate-500">
              {{ t('chat.dragDropHint') }}
            </p>
          </div>
        </div>

        <!-- Messages list - Virtual scroll for large lists -->
        <template v-if="chatStore.messages.length > 0">
          <!-- Use virtual scroll for large message lists -->
          <VirtualScroll
            v-if="useVirtualScroll"
            ref="virtualScrollRef"
            :item-count="chatStore.messages.length"
            :items="chatStore.messages"
            :estimated-item-height="120"
            :overscan="5"
            class="h-full pb-4"
            @visible-range-change="handleVisibleRangeChange"
          >
            <template #default="{ item: message, updateHeight }">
              <div
                v-if="message"
                :key="`${message.conversation_id}-${message.id}`"
                :ref="bindVirtualItemHeight(message, updateHeight)"
              >
                <ChatMessage
                  v-memo="messageMemoDeps(message)"
                  :message="message"
                  v-bind="messageRenderBindings(message)"
                  @contextmenu="handleMessageContextMenu"
                  @continue="handleMessageContinue"
                  @regenerate="handleMessageRegenerate"
                />
              </div>
            </template>
          </VirtualScroll>

          <!-- Regular rendering for small lists -->
          <div v-else class="pb-4">
            <div
              v-for="message in chatStore.messages"
              :key="`${message.conversation_id}-${message.id}`"
            >
              <ChatMessage
                v-memo="messageMemoDeps(message)"
                :message="message"
                v-bind="messageRenderBindings(message)"
                @contextmenu="handleMessageContextMenu"
                @continue="handleMessageContinue"
                @regenerate="handleMessageRegenerate"
              />
            </div>
          </div>

          <!-- Agent task panels -->
          <div v-if="agentTasks.length > 0" class="px-4">
            <AgentTaskPanel
              v-for="task in agentTasks"
              :key="task.id"
              :task="task"
              @cancel="cancelAgentTask"
              @delete="deleteAgentTask"
              @message="sendAgentMessage"
              @answer="submitAgentAnswer"
            />
          </div>

          <!-- Context trim indicator (pruning/compaction) -->
          <Transition name="fade">
            <div
              v-if="showContextTrim"
              class="flex justify-center py-2"
            >
              <div class="flex items-center gap-2 px-3 py-1.5 text-xs text-gray-400/70 dark:text-gray-500/70 bg-gray-100/30 dark:bg-gray-800/30 rounded-full">
                <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.121 14.121L19 19m-7-7l7-7m-7 7l-2.879 2.879M12 12L9.121 9.121m0 5.758a3 3 0 10-4.243 4.243 3 3 0 004.243-4.243zm0-5.758a3 3 0 10-4.243-4.243 3 3 0 004.243 4.243z" />
                </svg>
                <span v-if="chatStore.contextTrimInfo?.type === 'pruned'">
                  {{ t('chat.contextPruned', { tokens: formatTokens((chatStore.contextTrimInfo.tokensBefore ?? 0) - (chatStore.contextTrimInfo.tokensAfter ?? 0)) }) }}
                </span>
                <span v-else-if="chatStore.contextTrimInfo?.type === 'compacted'">
                  {{ t('chat.contextCompacted', { before: chatStore.contextTrimInfo.before ?? 0, after: chatStore.contextTrimInfo.after ?? 0 }) }}
                </span>
              </div>
            </div>
          </Transition>

          <!-- Stream error display (shown in chat area with gray text) -->
          <div
            v-if="chatStore.streamError"
            class="flex justify-center py-4"
          >
            <div class="flex items-center gap-2 px-4 py-2 text-gray-400 dark:text-gray-500 text-sm">
              <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <span class="break-all">{{
                chatStore.streamError === 'streamEmpty' ? t('chat.streamEmpty') :
                chatStore.streamError === 'streamError' ? t('chat.streamError') :
                chatStore.streamError === 'providerNoResponse' ? t('chat.providerNoResponse') :
                chatStore.streamError === 'providerReturnedEmpty' ? t('chat.providerReturnedEmpty') :
                chatStore.streamError === 'noResponseBody' ? t('chat.noResponseBody') :
                chatStore.streamError === 'trial_service_busy' ? t('chat.trialServiceBusy') :
                chatStore.streamError === 'provider_tool_unsupported' ? t('chat.providerToolUnsupported') :
                chatStore.streamError === 'provider_unavailable' ? t('chat.providerUnavailable') :
                chatStore.streamError === 'provider_auth_error' ? t('chat.providerAuthError') :
                chatStore.streamError === 'provider_rate_limited' ? t('chat.providerRateLimited') :
                chatStore.streamError === 'provider_openrouter_privacy_policy' ? t('chat.providerOpenRouterPrivacyPolicy') :
                chatStore.streamError
              }}</span>
              <button
                class="ml-1 px-2 py-0.5 rounded text-gray-400 hover:text-blue-400 hover:bg-blue-500/10 cursor-pointer transition-colors text-xs"
                @click="chatStore.clearStreamError(); chatStore.regenerateMessage()"
              >
                {{ t('common.retry') }}
              </button>
              <button
                class="text-gray-400 hover:text-gray-300 cursor-pointer"
                @click="chatStore.clearStreamError"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>
          <div
            v-else-if="chatStore.streamProgress"
            class="flex justify-center py-3"
          >
            <div class="flex items-center gap-2 px-3 py-1.5 text-xs text-gray-400/80 dark:text-gray-500/80 bg-gray-100/30 dark:bg-gray-800/30 rounded-full">
              <svg class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 12a8 8 0 018-8m8 8a8 8 0 01-8 8" />
              </svg>
              <span>{{ chatStore.streamProgress }}</span>
            </div>
          </div>

          <!-- Awaiting-user-input indicator -->
          <div
            v-if="showAwaitingConfirmation"
            class="flex justify-center py-2"
          >
            <div class="flex items-center gap-2 px-3 py-1.5 text-xs text-amber-500 dark:text-amber-300 bg-amber-500/10 rounded-full">
              <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M7 4h10l3 5v11H4V9l3-5z" />
              </svg>
              <span>Waiting for your confirmation to continue</span>
            </div>
          </div>

          <!-- "I'm listening" indicator (shown when pre-TTFT cancel is active) -->
          <div
            v-if="chatStore.preTTFTCancelActive"
            class="flex justify-center py-3"
          >
            <div class="flex items-center gap-2 px-4 py-2 glass-card rounded-lg text-sm text-blue-400">
              <span class="flex gap-1">
                <span class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 0ms" />
                <span class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 150ms" />
                <span class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 300ms" />
              </span>
              {{ t('chat.stillListening') }}
            </div>
          </div>

          <!-- Streaming action buttons -->
          <div
            v-if="chatStore.messages.length > 0 && !chatStore.isMultiSelectMode"
            class="flex justify-center gap-2 py-4"
          >
            <!-- Stop button (shown during streaming or async tasks) -->
            <button
              v-if="chatStore.streaming || mediaGen.generating.value || agentTasks.some(at => at.status === 'executing' || at.status === 'planning')"
              class="flex items-center gap-2 px-4 py-2 glass-card text-red-400 hover:bg-red-500/10 rounded-lg text-sm transition-colors cursor-pointer"
              @click="handleCancel"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
              </svg>
              {{ t('chat.stopGenerating') }}
            </button>

            <!-- Continue/Regenerate buttons removed — will be replaced by suggested follow-up prompts -->
          </div>
        </template>
      </div>

      <!-- Context menu -->
      <Teleport to="body">
        <!-- Desktop: Floating menu -->
        <div
          v-if="showContextMenu && !isMobile"
          class="context-menu fixed z-[100] glass-card shadow-xl py-1 min-w-[160px]"
          :style="{ left: `${contextMenuPosition.x}px`, top: `${contextMenuPosition.y}px` }"
        >
          <button
            class="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
            @click="handleSelectMessage"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            {{ t('chat.selectMessage') }}
          </button>
          <!-- Continue / Regenerate — only for last assistant message, not while streaming -->
          <template v-if="isContextMenuLastAssistant && !chatStore.streaming">
            <div class="border-t border-white/10 my-1" />
            <button
              class="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
              @click="handleContextContinue"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              {{ t('chat.continueGenerating') }}
            </button>
            <button
              class="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
              @click="handleContextRegenerate"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              {{ t('chat.regenerate') }}
            </button>
          </template>
        </div>

      </Teleport>

      <!-- Theme style selector (teleported to body for proper z-index) -->
      <Teleport to="body">
        <!-- Desktop: Dropdown menu -->
        <div
          v-if="showStyleSelector && !isMobile"
          class="style-selector-container fixed z-[100] glass-card shadow-xl p-3 min-w-[180px]"
          :style="{ left: `${Math.max(8, styleSelectorPosition.x - 180)}px`, top: `${styleSelectorPosition.y}px` }"
        >
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-2 font-medium">{{ t('theme.styles.title') }}</div>
          <div class="flex flex-col gap-1">
            <button
              v-for="style in THEME_STYLES"
              :key="style.id"
              class="flex items-center gap-2 px-2 py-1.5 rounded-md text-sm text-left transition-colors cursor-pointer"
              :class="settingsStore.themeStyle === style.id ? 'bg-gray-100 dark:bg-gray-700/30 text-gray-900 dark:text-gray-300' : 'hover:bg-white/10 text-gray-700 dark:text-gray-300'"
              @click="selectThemeStyle(style.id)"
            >
              <span
                class="theme-style-btn flex-shrink-0"
                :class="`theme-style-btn-${style.id}`"
              />
              <span>{{ themeStyleLabel(style) }}</span>
            </button>
          </div>
        </div>

        <!-- Mobile: Bottom sheet -->
        <Transition name="sheet">
          <div
            v-if="showStyleSelector && isMobile"
            class="fixed inset-0 z-[9999] flex items-end"
            @click="showStyleSelector = false"
          >
            <!-- Backdrop -->
            <div class="absolute inset-0 bg-black/50" />

            <!-- Sheet content -->
            <div
              class="relative w-full bg-white dark:bg-gray-800 rounded-t-2xl shadow-2xl"
              @click.stop
            >
              <!-- Handle bar -->
              <div class="flex justify-center pt-3 pb-2">
                <div class="w-10 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
              </div>

              <!-- Title -->
              <div class="px-4 pb-2">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('theme.styles.title') }}</h3>
              </div>

              <!-- Actions -->
              <div class="p-4 space-y-2">
                <button
                  v-for="style in THEME_STYLES"
                  :key="style.id"
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left transition-colors border border-gray-200 dark:border-gray-700"
                  :class="settingsStore.themeStyle === style.id ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white' : 'bg-gray-50 dark:bg-gray-700/40 text-gray-700 dark:text-gray-300'"
                  @click="selectThemeStyle(style.id)"
                >
                  <span
                    class="theme-style-btn flex-shrink-0"
                    :class="`theme-style-btn-${style.id}`"
                  />
                  <span class="font-medium">{{ themeStyleLabel(style) }}</span>
                </button>
              </div>

              <!-- Safe area padding -->
              <div class="h-[env(safe-area-inset-bottom)]" />
            </div>
          </div>
        </Transition>
      </Teleport>

      <!-- Multi-select action bar -->
      <Transition name="slide-up">
        <div
          v-if="chatStore.isMultiSelectMode"
          class="absolute bottom-20 left-1/2 -translate-x-1/2 z-20 glass-card shadow-xl px-4 py-3 flex items-center gap-4"
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
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
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
          chatStore.error === 'trial_service_busy' ? t('chat.trialServiceBusy') :
          chatStore.error === 'provider_tool_unsupported' ? t('chat.providerToolUnsupported') :
          chatStore.error === 'provider_unavailable' ? t('chat.providerUnavailable') :
          chatStore.error === 'provider_auth_error' ? t('chat.providerAuthError') :
          chatStore.error === 'provider_rate_limited' ? t('chat.providerRateLimited') :
          chatStore.error === 'provider_openrouter_privacy_policy' ? t('chat.providerOpenRouterPrivacyPolicy') :
          chatStore.error
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
          <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <span class="truncate">{{ chatStore.securityBlocked.message }}</span>
          <span class="text-yellow-500/70 text-xs">({{ t('chat.threatLevel') }}: {{ chatStore.securityBlocked.threatLevel }})</span>
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
          <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
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
      <div :class="isMobile ? 'flex-shrink-0 border-t border-gray-200 dark:border-glass-border' : 'absolute bottom-0 left-0 right-0 z-10'">
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
        <ChatInput
          ref="chatInputRef"
          :disabled="chatStore.sending && !chatStore.isPreTTFT"
          :streaming="chatStore.streaming"
          @send="handleSend"
          @inject="handleInject"
          @cancel="handleCancel"
          @cancel-pre-ttft="chatStore.cancelPreTTFT()"
          @warmup="chatStore.warmupConversation()"
          @open-talk-mode="showTalkMode = true"
        />
      </div>

      <!-- Talk Mode -->
      <TalkMode
        v-model="showTalkMode"
        :conversation-id="chatStore.currentConversationId || undefined"
        @transcript="handleVoiceTranscript"
      />
    </main>

    <!-- Tool call approval dialog -->
    <ToolApprovalDialog />

    <!-- Exec directory approval dialog -->
    <ExecApprovalDialog />

    <!-- Provider config required dialog -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="showProviderConfigDialog"
          class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 backdrop-blur-sm"
          @click.self="showProviderConfigDialog = false"
        >
          <div class="w-full max-w-sm mx-4 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden">
            <div class="p-6 text-center">
              <div class="w-14 h-14 mx-auto mb-4 rounded-full bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center">
                <svg class="w-7 h-7 text-amber-600 dark:text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
              </div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">
                {{ t('chat.noProvider.title') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">
                {{ t('chat.noProvider.description') }}
              </p>
              <div class="flex gap-3">
                <button
                  class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors cursor-pointer"
                  @click="showProviderConfigDialog = false"
                >
                  {{ t('common.cancel') }}
                </button>
                <button
                  class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors cursor-pointer"
                  @click="showProviderConfigDialog = false; router.push('/settings?tab=llm')"
                >
                  {{ t('chat.noProvider.configure') }}
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
  --ct-border-soft: rgba(148, 163, 184, 0.22);
  --ct-border-strong: rgba(56, 189, 248, 0.55);
  --ct-chip-bg: rgba(15, 23, 42, 0.12);
  --ui-gap: 0.5rem;
  --ui-chip-h: 2rem;
  --ui-icon-size: 2rem;
  --ui-radius: 0.5rem;
  min-height: 0;
  flex: 1;
  height: 100%;
  overflow: hidden;
  background: radial-gradient(circle at 20% -20%, rgba(14, 165, 233, 0.16), transparent 42%), radial-gradient(circle at 100% 0%, rgba(20, 184, 166, 0.12), transparent 36%), var(--color-bg-base);
}

.chat-view.ui-density-compact {
  --ui-gap: 0.35rem;
  --ui-chip-h: 1.8rem;
  --ui-icon-size: 1.8rem;
  --ui-radius: 0.45rem;
}

.chat-view.ui-density-comfortable {
  --ui-gap: 0.65rem;
  --ui-chip-h: 2.2rem;
  --ui-icon-size: 2.2rem;
  --ui-radius: 0.62rem;
}

.chat-view::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: linear-gradient(rgba(148, 163, 184, 0.05) 1px, transparent 1px), linear-gradient(90deg, rgba(148, 163, 184, 0.05) 1px, transparent 1px);
  background-size: 28px 28px;
  opacity: 0.25;
  pointer-events: none;
  z-index: 0;
}

:root.light .chat-view,
[data-theme="light"] .chat-view {
  --ct-border-soft: rgba(148, 163, 184, 0.35);
  --ct-border-strong: rgba(2, 132, 199, 0.45);
  --ct-chip-bg: rgba(255, 255, 255, 0.72);
  background: radial-gradient(circle at 15% -15%, rgba(14, 165, 233, 0.12), transparent 40%), radial-gradient(circle at 100% 0%, rgba(16, 185, 129, 0.1), transparent 34%), #f8fbff;
}

:root.light .chat-view::before,
[data-theme="light"] .chat-view::before {
  opacity: 0.4;
}

.conversation-sidebar,
.chat-messages-area,
header,
.chat-input-wrapper {
  position: relative;
  z-index: 1;
}

.glass-sidebar {
  background: var(--chat-sidebar-bg, rgba(15, 23, 42, 0.82));
  backdrop-filter: blur(18px);
}

.glass-header {
  background: rgba(15, 23, 42, 0.74);
  backdrop-filter: blur(14px);
}

.chat-sidebar-shell {
  box-shadow: inset -1px 0 0 var(--chat-sidebar-border-soft, rgba(148, 163, 184, 0.22)), 14px 0 26px rgba(2, 6, 23, 0.16);
}

.chat-topbar {
  background-image: linear-gradient(180deg, rgba(15, 23, 42, 0.52), rgba(15, 23, 42, 0.28));
  box-shadow: 0 8px 22px rgba(2, 6, 23, 0.18);
  overflow: visible;
}

.chat-topbar::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(56, 189, 248, 0.4), transparent);
  pointer-events: none;
}

.chat-topbar::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(110deg, transparent 0%, rgba(125, 211, 252, 0.08) 42%, transparent 70%);
  transform: translateX(-120%);
  animation: topbar-sheen 6s ease-in-out infinite;
  pointer-events: none;
}

.chat-title-wrap {
  position: relative;
}

.chat-title {
  letter-spacing: 0.01em;
  text-shadow: 0 1px 0 rgba(2, 6, 23, 0.12);
}

.chat-title::before {
  content: '';
  display: inline-block;
  width: 0.38rem;
  height: 0.38rem;
  margin-right: 0.55rem;
  border-radius: 999px;
  background: linear-gradient(135deg, #38bdf8, #2dd4bf);
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.18);
  vertical-align: middle;
}

.chat-nav-btn,
.topbar-icon-btn,
.topbar-chip {
  border: 1px solid transparent;
}

.chat-nav-btn:hover,
.topbar-icon-btn:hover {
  border-color: rgba(148, 163, 184, 0.3);
  transform: translateY(-1px);
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
  transform: translateY(-1px);
}

.chat-tools {
  margin-left: auto;
  justify-content: flex-end;
  padding-left: calc(var(--ui-gap) * 0.5);
  border-left: 1px solid rgba(148, 163, 184, 0.16);
  gap: var(--ui-gap);
}

.topbar-icon-btn {
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  min-width: var(--ui-icon-size);
  min-height: var(--ui-icon-size);
  border-radius: var(--ui-radius);
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
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.35);
  border-color: var(--ct-border-strong);
}

.bg-surface-base {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.08), transparent 32%);
}

.border-glass-border {
  border-color: rgba(148, 163, 184, 0.22);
}

:root.light .glass-sidebar,
[data-theme="light"] .glass-sidebar {
  background: var(--chat-sidebar-bg, rgba(255, 255, 255, 0.95));
}

:root.light .glass-header,
[data-theme="light"] .glass-header {
  background: rgba(255, 255, 255, 0.93);
}

:root.light .chat-sidebar-shell,
[data-theme="light"] .chat-sidebar-shell {
  box-shadow: inset -1px 0 0 var(--chat-sidebar-border-soft, rgba(186, 203, 223, 0.65)), 10px 0 20px rgba(30, 41, 59, 0.08);
}

:root.light .chat-topbar,
[data-theme="light"] .chat-topbar {
  background-image: linear-gradient(180deg, rgba(255, 255, 255, 0.93), rgba(248, 250, 252, 0.8));
  box-shadow: 0 8px 20px rgba(30, 41, 59, 0.08);
}

.quick-action-tile {
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(255, 255, 255, 0.92);
  border-radius: 0.9rem;
  padding: 0.75rem;
  text-align: left;
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease, background-color 0.18s ease;
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
[data-theme="dark"] .quick-action-tile {
  background: rgba(30, 41, 59, 0.92);
  border-color: rgba(100, 116, 139, 0.45);
}

:root.dark .quick-action-tile.is-active,
[data-theme="dark"] .quick-action-tile.is-active {
  border-color: rgba(56, 189, 248, 0.62);
  background: rgba(15, 23, 42, 0.94);
  box-shadow: 0 8px 18px rgba(2, 132, 199, 0.22);
}

:root.dark .quick-action-pill,
[data-theme="dark"] .quick-action-pill {
  color: rgb(203, 213, 225);
  background: rgba(51, 65, 85, 0.88);
}

.routing-trigger-btn {
  color: rgb(71, 85, 105);
  border-color: transparent;
  background: transparent;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.routing-trigger-btn:hover {
  border-color: rgba(148, 163, 184, 0.35);
  background: rgba(241, 245, 249, 0.95);
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
[data-theme="dark"] .routing-trigger-btn {
  color: rgb(148, 163, 184);
  border-color: transparent;
  background: transparent;
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.12), 0 1px 3px rgba(2, 6, 23, 0.45);
}

:root.dark .routing-trigger-btn:hover,
[data-theme="dark"] .routing-trigger-btn:hover {
  color: rgb(226, 232, 240);
  border-color: rgba(100, 116, 139, 0.48);
  background: rgba(30, 41, 59, 0.92);
}

:root.dark .routing-trigger-btn.is-error,
[data-theme="dark"] .routing-trigger-btn.is-error {
  color: rgb(252, 165, 165);
  border-color: rgba(248, 113, 113, 0.58);
  background: rgba(69, 10, 10, 0.42);
}

:root.dark .routing-trigger-btn.is-pending,
[data-theme="dark"] .routing-trigger-btn.is-pending {
  color: rgb(253, 224, 71);
  border-color: rgba(250, 204, 21, 0.54);
  background: rgba(113, 63, 18, 0.42);
}

:root.dark .routing-trigger-btn.is-none,
[data-theme="dark"] .routing-trigger-btn.is-none {
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
  transition: border-color 0.16s ease, background-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
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
[data-theme="dark"] .routing-menu-floating {
  background: rgb(15, 23, 42);
  border-color: rgba(100, 116, 139, 0.72);
  box-shadow: 0 22px 54px rgba(2, 6, 23, 0.62);
  opacity: 1 !important;
}

:root.dark .routing-sheet-panel,
[data-theme="dark"] .routing-sheet-panel {
  background: rgb(15, 23, 42);
  border-top-color: rgba(100, 116, 139, 0.5);
  box-shadow: 0 -16px 40px rgba(2, 6, 23, 0.72);
}

:root.dark .routing-option-row,
[data-theme="dark"] .routing-option-row {
  border-color: rgba(71, 85, 105, 0.75);
  background: rgba(30, 41, 59, 0.96);
  color: rgb(226, 232, 240);
}

:root.dark .routing-option-row:hover,
[data-theme="dark"] .routing-option-row:hover {
  border-color: rgba(56, 189, 248, 0.42);
  background: rgba(30, 41, 59, 1);
}

:root.dark .routing-option-row.is-active,
[data-theme="dark"] .routing-option-row.is-active {
  border-color: rgba(56, 189, 248, 0.66);
  background: rgba(8, 47, 73, 0.72);
}

:root.dark .routing-option-icon,
[data-theme="dark"] .routing-option-icon {
  color: rgb(148, 163, 184);
}

:root.dark .routing-option-title,
[data-theme="dark"] .routing-option-title {
  color: rgb(241, 245, 249);
}

:root.dark .routing-option-desc,
[data-theme="dark"] .routing-option-desc {
  color: rgb(148, 163, 184);
}

:root.dark .routing-option-count,
[data-theme="dark"] .routing-option-count {
  color: rgb(134, 239, 172);
  background: rgba(6, 78, 59, 0.78);
}

:root.dark .routing-option-status-chip,
[data-theme="dark"] .routing-option-status-chip {
  color: rgb(186, 230, 253);
  background: rgba(12, 74, 110, 0.82);
}

:root.dark .routing-strategy-chip,
[data-theme="dark"] .routing-strategy-chip {
  border-color: rgba(71, 85, 105, 0.8);
  color: rgb(203, 213, 225);
  background: rgba(30, 41, 59, 0.95);
}

:root.dark .routing-strategy-chip.is-active-ha,
[data-theme="dark"] .routing-strategy-chip.is-active-ha {
  color: rgb(186, 230, 253);
  border-color: rgba(56, 189, 248, 0.68);
  background: rgba(12, 74, 110, 0.52);
}

:root.dark .routing-strategy-chip.is-active-fixed,
[data-theme="dark"] .routing-strategy-chip.is-active-fixed {
  color: rgb(167, 243, 208);
  border-color: rgba(16, 185, 129, 0.66);
  background: rgba(6, 78, 59, 0.5);
}

:root.dark .routing-model-row,
[data-theme="dark"] .routing-model-row {
  border-color: rgba(71, 85, 105, 0.75);
  color: rgb(226, 232, 240);
  background: rgba(30, 41, 59, 0.95);
}

:root.dark .routing-model-row.is-selected,
[data-theme="dark"] .routing-model-row.is-selected {
  border-color: rgba(16, 185, 129, 0.66);
  background: rgba(6, 78, 59, 0.5);
}

:root.dark .routing-manage-row:hover,
[data-theme="dark"] .routing-manage-row:hover {
  background: rgba(30, 41, 59, 0.92);
}

:root.light .chat-title::before,
[data-theme="light"] .chat-title::before {
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
}

:root.light .chat-nav-btn:hover,
:root.light .topbar-icon-btn:hover,
[data-theme="light"] .chat-nav-btn:hover,
[data-theme="light"] .topbar-icon-btn:hover {
  border-color: rgba(148, 163, 184, 0.35);
}

:root.light .topbar-chip,
[data-theme="light"] .topbar-chip {
  border-color: var(--ct-border-soft);
  background: var(--ct-chip-bg);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

:root.light .topbar-icon-btn,
[data-theme="light"] .topbar-icon-btn {
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

:root.light .chat-tools,
[data-theme="light"] .chat-tools {
  border-left-color: rgba(148, 163, 184, 0.22);
}

:root.light .bg-surface-base,
[data-theme="light"] .bg-surface-base {
  background: linear-gradient(180deg, rgba(236, 246, 255, 0.75), rgba(255, 255, 255, 0.7) 32%, rgba(255, 255, 255, 0.35));
}

:root.light .border-glass-border,
[data-theme="light"] .border-glass-border {
  border-color: rgba(186, 203, 223, 0.65);
}

@media (max-width: 640px) {
  .chat-tools {
    margin-left: 0;
    padding-left: 0.1rem;
    border-left: none;
  }

  .chat-topbar::after {
    opacity: 0.7;
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

/* Theme style presets */
.theme-style-btn {
  width: 14px;
  height: 14px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.35);
}

.theme-style-btn-default {
  background: linear-gradient(135deg, #0ea5e9, #14b8a6);
}

.theme-style-btn-bubble {
  background: linear-gradient(135deg, #8b5cf6, #ec4899);
}

.theme-style-btn-minimal {
  background: linear-gradient(135deg, #64748b, #94a3b8);
}

.theme-style-btn-gradient {
  background: linear-gradient(135deg, #f43f5e, #f59e0b, #22c55e);
}

.theme-style-btn-ocean {
  background: linear-gradient(135deg, #0ea5e9, #0284c7, #0369a1);
}

.chat-view.theme-style-bubble :deep(.chat-user-bubble) {
  border-radius: 1.1rem 1.1rem 0.45rem 1.1rem;
  background: var(--chat-user-bg);
  box-shadow: 0 12px 24px rgba(139, 92, 246, 0.24);
}

.chat-view.theme-style-bubble {
  background: radial-gradient(circle at 8% -18%, rgba(139, 92, 246, 0.28), transparent 36%), radial-gradient(circle at 100% 0%, rgba(236, 72, 153, 0.22), transparent 34%), #121426;
}

:root.light .chat-view.theme-style-bubble,
[data-theme="light"] .chat-view.theme-style-bubble {
  background: radial-gradient(circle at 8% -18%, rgba(139, 92, 246, 0.18), transparent 36%), radial-gradient(circle at 100% 0%, rgba(236, 72, 153, 0.14), transparent 34%), #fdf4ff;
}

.chat-view.theme-style-bubble :deep(.assistant-message) {
  border-radius: 1.1rem;
  background: var(--chat-assistant-bg);
  border: var(--chat-assistant-border);
}

.chat-view.theme-style-bubble :deep(.avatar) {
  background: var(--chat-avatar-bg);
}

.chat-view.theme-style-bubble :deep(.chat-input-container) {
  border-color: rgba(192, 132, 252, 0.4);
}

.chat-view.theme-style-bubble :deep(.chat-textarea:focus) {
  border-color: rgba(168, 85, 247, 0.5);
  box-shadow: 0 0 0 3px rgba(168, 85, 247, 0.16);
}

.chat-view.theme-style-minimal::before {
  display: none;
}

.chat-view.theme-style-minimal {
  background: var(--color-bg-base);
}

:root.light .chat-view.theme-style-minimal,
[data-theme="light"] .chat-view.theme-style-minimal {
  background: #f8fafc;
}

.chat-view.theme-style-minimal :deep(.glass-header),
.chat-view.theme-style-minimal :deep(.glass-sidebar) {
  backdrop-filter: none;
}

.chat-view.theme-style-minimal :deep(.chat-user-bubble) {
  background: var(--chat-user-bg);
  color: var(--chat-user-text);
  border: var(--chat-user-border);
  box-shadow: none;
  border-radius: 0.75rem;
}

.chat-view.theme-style-minimal :deep(.assistant-message) {
  background: transparent;
  box-shadow: none;
  border: 1px solid rgba(148, 163, 184, 0.25);
}

:root.light .chat-view.theme-style-minimal :deep(.chat-user-bubble),
[data-theme="light"] .chat-view.theme-style-minimal :deep(.chat-user-bubble) {
  background: var(--chat-user-bg);
  color: var(--chat-user-text);
  border: var(--chat-user-border);
}

:root.light .chat-view.theme-style-minimal :deep(.assistant-message),
[data-theme="light"] .chat-view.theme-style-minimal :deep(.assistant-message) {
  background: #ffffff;
}

.chat-view.theme-style-gradient {
  background: radial-gradient(circle at -10% -20%, rgba(244, 63, 94, 0.2), transparent 36%), radial-gradient(circle at 100% 0%, rgba(245, 158, 11, 0.18), transparent 34%), radial-gradient(circle at 100% 100%, rgba(34, 197, 94, 0.14), transparent 42%), var(--color-bg-base);
}

.chat-view.theme-style-gradient :deep(.chat-user-bubble) {
  background: linear-gradient(135deg, #f43f5e, #f59e0b);
  box-shadow: 0 12px 24px rgba(244, 63, 94, 0.26);
}

.chat-view.theme-style-gradient :deep(.avatar) {
  background: linear-gradient(135deg, #f97316, #22c55e);
}

.chat-view.theme-style-gradient :deep(.assistant-message) {
  border-color: rgba(251, 191, 36, 0.26);
}

.chat-view.theme-style-gradient :deep(.chat-textarea:focus) {
  border-color: rgba(244, 114, 182, 0.5);
  box-shadow: 0 0 0 3px rgba(244, 114, 182, 0.16);
}

.chat-view.theme-style-ocean {
  background: radial-gradient(circle at 15% -20%, rgba(56, 189, 248, 0.24), transparent 40%), radial-gradient(circle at 100% 0%, rgba(45, 212, 191, 0.18), transparent 36%), #12314a;
}

:root.light .chat-view.theme-style-ocean,
[data-theme="light"] .chat-view.theme-style-ocean {
  background: radial-gradient(circle at 15% -20%, rgba(14, 116, 144, 0.12), transparent 38%), radial-gradient(circle at 100% 0%, rgba(14, 165, 233, 0.14), transparent 34%), #f0f9ff;
}

.chat-view.theme-style-ocean :deep(.chat-user-bubble) {
  background: linear-gradient(135deg, #0284c7, #0369a1);
}

.chat-view.theme-style-ocean :deep(.assistant-message) {
  background: rgba(2, 44, 84, 0.45);
  border-color: rgba(56, 189, 248, 0.24);
}

.chat-view.theme-style-ocean :deep(.avatar) {
  background: linear-gradient(135deg, #0284c7, #0ea5e9);
}

/* Mobile view styles */
.mobile-view {
  position: fixed;
  inset: 0;
  z-index: 10;
  background: var(--color-bg-base);
}

/* Mobile conversation list page */
/* Mobile conversation list page - stay in normal flow so AppHeader shows above */
.mobile-list-page {
  width: 100%;
  height: 100%;
  border: none;
  background: transparent;
}

:global(.dark) .mobile-list-page {
  background: transparent;
}

/* Mobile main chat area */
.mobile-chat {
  position: fixed !important;
  inset: 0;
  z-index: 20;
  background: var(--color-bg-base);
  flex-direction: column !important;
  transform: translateX(0);
}

.mobile-chat-animated {
  transition: transform 0.3s ease-out;
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
}

/* Mobile input area - stick to bottom */
.mobile-chat > div:last-child {
  position: relative !important;
  bottom: auto !important;
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
    text-shadow: 0 0 10px rgba(239, 68, 68, 0.8), 0 0 20px rgba(239, 68, 68, 0.5);
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
  0%, 100% {
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
  0%, 100% {
    background-color: rgb(30 58 138 / 0.2);
  }
  30% {
    background-color: rgb(180 83 9 / 0.3);
  }
}
</style>
