<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Message } from '@/api/chat'
import { cardActionApi } from '@/api/chat'
import { renderMarkdownCached, copyCodeToClipboard } from '@/utils/markdown'
import { useChatStore, type ActiveMessageStreamState } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { useProviderPoolStore } from '@/stores/providerPool'
import type { ToolResultItem } from '@/stores/chat'
import {
  parseTypelessContent,
  parseTypelessContentIncremental,
  splitIntoSegments,
  hasTypelessCards,
  clearIncrementalState,
  clearSplitSegmentsIncrementalState,
} from '@/utils/typeless'
import { stripFirstLineHeading } from '@/utils/chat-message-text'
import { stripDuplicateTodoChecklistForMessage } from '@/utils/todoChecklist'
import type { TypelessCard, TypelessCardChoice, ParsedContent } from '@/types/typeless'
import TypelessCardComponent from '@/components/typeless/TypelessCard.vue'
import ToolDetailCard from '@/components/ToolDetailCard.vue'
import MediaPlaceholder from '@/components/MediaPlaceholder.vue'
import { ttsAudioManager, streamingTTSManager } from '@/api/voice'
import { speechApi } from '@/api/speech'
import { useNotificationStore } from '@/stores/notification'
import { isTtsAutoPlayEnabled, isTtsSpeechMuted } from '@/utils/ttsPreferences'
import { buildChatCardUiStateKey } from '@/utils/chatCardUiState'
import type { ProcessTraceItem } from '@/utils/processTrace'
import { getLocalizedToolName } from '@/utils/toolLocalization'
import { measureChatPerf, recordChatPerfCount } from '@/utils/chatPerf'

const { t, te } = useI18n()
const providerPoolStore = useProviderPoolStore()

const props = defineProps<{
  message: Message
  isStreaming?: boolean
  isLastAssistantMessage?: boolean
  isMobile?: boolean
  isSelected?: boolean
  isMultiSelectMode?: boolean
  disableAutoTTS?: boolean
  showExternalStatusRail?: boolean
  preferImmediateStreamingRender?: boolean
  streamState?: ActiveMessageStreamState | null
}>()

const emit = defineEmits<{
  contextmenu: [event: MouseEvent, messageId: string]
  cardAction: [
    conversationId: string,
    messageId: string,
    cardId: string,
    actionId: string,
    actionLabel?: string,
  ]
  continue: []
  regenerate: []
  'edit-resubmit': [messageId: string, content: string]
}>()

const chatStore = useChatStore()
const settingsStore = useSettingsStore()

// Card action state
const cardActionLoading = ref<{ cardId: string; actionId: string } | null>(null) // active card action in flight
const cardActionError = ref<{ cardId: string; message: string } | null>(null)

const isUser = computed(() => props.message.role === 'user')
const isAssistant = computed(() => props.message.role === 'assistant')
const isSelected = computed(
  () => props.isSelected ?? chatStore.selectedMessageIds.has(props.message.id)
)
const isMultiSelectMode = computed(() => props.isMultiSelectMode ?? chatStore.isMultiSelectMode)
const isMobile = computed(() => !!props.isMobile)
const showLeadingAvatar = computed(() => isAssistant.value)
const showTrailingAvatar = computed(() => isUser.value)
const currentAvatarVariantClass = computed(() => {
  return isAssistant.value ? 'avatar-user' : 'avatar-assistant'
})
const currentAvatarLabel = computed(() => (isAssistant.value ? 'Blue' : 'You'))
const userBubbleClasses = computed(() => [
  'user-message',
  'chat-copy-bubble',
  'px-4',
  'py-2',
  'inline-block',
  'chat-user-bubble',
])

const fallbackStreamState = computed<ActiveMessageStreamState | null>(() => {
  if (!props.isStreaming) return null
  return {
    phase: chatStore.streamUIState?.phase || 'streaming',
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
    showExternalStatusRail: !!props.showExternalStatusRail,
  }
})

const streamState = computed(() => props.streamState ?? fallbackStreamState.value)
const streamPhase = computed(() => streamState.value?.phase || 'idle')
const streamAwaitingConfirmation = computed(() => streamState.value?.awaitingConfirmation === true)
const streamToolExecuting = computed(() => streamState.value?.toolExecuting === true)
const streamToolExecutingCommands = computed(() => streamState.value?.toolExecutingCommands || [])
const streamToolExecutingNames = computed(() => streamState.value?.toolExecutingNames || [])
const streamToolSandboxAvailable = computed(() => streamState.value?.toolSandboxAvailable === true)
const streamStatusSummary = computed(() => streamState.value?.statusSummary || null)
const streamProgress = computed(() => streamState.value?.streamProgress || null)
const streamProcessTrace = computed(() => streamState.value?.processTrace || [])
const streamToolResults = computed(() => streamState.value?.toolResults || [])
const streamStatusStartedAt = computed(() => streamState.value?.statusStartedAt || 0)
const showExternalStatusRail = computed(
  () => streamState.value?.showExternalStatusRail ?? !!props.showExternalStatusRail
)

const trackStreamingState = computed(() => isAssistant.value && !!props.isStreaming)
const hasMediaTask = computed(() => {
  if (!isAssistant.value) return false
  // Server-side: message content is [media_task:uuid]
  return /^\[media_task:[a-f0-9-]+\]$/.test(props.message.content.trim())
})
const mediaTaskId = computed(() => {
  const m = props.message.content.trim().match(/^\[media_task:([a-f0-9-]+)\]$/)
  return m?.[1] ?? ''
})

// Check if user message has attachments
const hasAttachments = computed(
  () => isUser.value && props.message.attachments && props.message.attachments.length > 0
)

const audioAttachments = computed(() => {
  if (!isUser.value || !props.message.attachments) return []
  return props.message.attachments.filter((a) => a.type === 'audio')
})

// Check if this is a voice-only message (audio attachment, no meaningful text)
const isVoiceMessage = computed(() => {
  if (!isUser.value) return false
  if (audioAttachments.value.length === 0) return false
  // Voice-only: no text content or placeholder text
  const text = props.message.content?.trim()
  return !text || isPlaceholderContent(text)
})

const isSpecialUserPlaceholder = computed(() => {
  if (!isUser.value) return false
  const content = props.message.content.trim()
  return content === '[CONTINUE]' || content === '[CONTINUE_AFTER_CANCEL]'
})

type InlineImageInfo = { alt: string; url: string }
type InlineImageParseResult = { images: InlineImageInfo[]; strippedText: string }
type RuntimeProcessMessage = Message & {
  local_process_tool_results?: ToolResultItem[]
}
type MessageAttachment = NonNullable<Message['attachments']>[number]
const INLINE_IMAGE_RE = /!\[([^\]]*)\]\(([^)]+)\)/g
const INLINE_IMAGE_STRIP_RE = /\n*!\[[^\]]*\]\([^)]+\)/g
const INLINE_IMAGE_CACHE_KEY = '__zima_chat_inline_image_cache_v1__'
const INLINE_IMAGE_CACHE_MAX = 300

function getInlineImageCache(): Map<string, InlineImageParseResult> {
  const g = globalThis as Record<string, unknown>
  const existing = g[INLINE_IMAGE_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, InlineImageParseResult>
  }
  const cache = new Map<string, InlineImageParseResult>()
  g[INLINE_IMAGE_CACHE_KEY] = cache
  return cache
}

function evictOldestMapEntry<K, V>(cache: Map<K, V>): void {
  const oldestKey = cache.keys().next().value
  if (oldestKey !== undefined) {
    cache.delete(oldestKey as K)
  }
}

function parseInlineImagesCached(content: string): InlineImageParseResult {
  const cache = getInlineImageCache()
  const cached = cache.get(content)
  if (cached) return cached

  const images: InlineImageInfo[] = []
  let match: RegExpExecArray | null
  INLINE_IMAGE_RE.lastIndex = 0
  while ((match = INLINE_IMAGE_RE.exec(content)) !== null) {
    images.push({ alt: match[1] ?? 'image', url: match[2] ?? '' })
  }

  const strippedText =
    images.length > 0 ? content.replace(INLINE_IMAGE_STRIP_RE, '').trim() : content

  const parsed = { images, strippedText }
  if (cache.size >= INLINE_IMAGE_CACHE_MAX && !cache.has(content)) {
    evictOldestMapEntry(cache)
  }
  cache.set(content, parsed)
  return parsed
}

const parsedInlineContent = computed(() => {
  if (!isUser.value) return null
  return parseInlineImagesCached(props.message.content)
})

// Adaptive streaming reveal: slow deltas feel closer to per-character typing,
// while bursty deltas collapse into short word/sentence chunks.
const revealedStreamingContent = ref(props.message.content)
let streamingRevealRafId: number | null = null
let streamingRevealTarget = props.message.content
let lastStreamingTargetAt = Date.now()

const STREAMING_REVEAL_FAST_THRESHOLD_MS = 28
const STREAMING_REVEAL_MEDIUM_THRESHOLD_MS = 56
const RE_STREAMING_IMMEDIATE_FLUSH = /```|\n\n|\r\n\r\n|\|\s*[-:]+\s*\|/
const STREAMING_REVEAL_MAX_CHARS = 280
const STREAMING_BOUNDARY_CHARS = new Set([
  ' ',
  '\n',
  '\t',
  '.',
  ',',
  '!',
  '?',
  ';',
  ':',
  '，',
  '。',
  '！',
  '？',
  '；',
  '：',
])

function scheduleStreamingRevealFrame(callback: FrameRequestCallback): number {
  if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
    return window.requestAnimationFrame(callback)
  }
  return window.setTimeout(() => callback(Date.now()), 16)
}

function cancelStreamingRevealFrame(handle: number): void {
  if (typeof window !== 'undefined' && typeof window.cancelAnimationFrame === 'function') {
    window.cancelAnimationFrame(handle)
    return
  }
  window.clearTimeout(handle)
}

function clearStreamingRevealRaf() {
  if (streamingRevealRafId === null) return
  cancelStreamingRevealFrame(streamingRevealRafId)
  streamingRevealRafId = null
}

function flushStreamingReveal(nextContent = streamingRevealTarget) {
  clearStreamingRevealRaf()
  streamingRevealTarget = nextContent
  revealedStreamingContent.value = nextContent
}

function takeBoundaryChunk(delta: string, minChars: number, maxChars: number): string {
  const glyphs = Array.from(delta)
  if (glyphs.length <= maxChars) return delta

  for (let index = Math.max(0, minChars - 1); index < Math.min(glyphs.length, maxChars); index++) {
    if (STREAMING_BOUNDARY_CHARS.has(glyphs[index] || '')) {
      return glyphs.slice(0, index + 1).join('')
    }
  }

  return glyphs.slice(0, maxChars).join('')
}

function nextStreamingRevealChunk(delta: string, firstChunk = false): string {
  const glyphs = Array.from(delta)
  if (glyphs.length <= 2) return delta
  if (firstChunk) return glyphs.slice(0, 1).join('')
  if (RE_STREAMING_IMMEDIATE_FLUSH.test(delta)) return delta

  const sinceTarget = Date.now() - lastStreamingTargetAt
  if (sinceTarget < STREAMING_REVEAL_FAST_THRESHOLD_MS || glyphs.length >= 18) {
    return takeBoundaryChunk(delta, 6, 18)
  }
  if (sinceTarget < STREAMING_REVEAL_MEDIUM_THRESHOLD_MS || glyphs.length >= 8) {
    return takeBoundaryChunk(delta, 3, 8)
  }
  return glyphs.slice(0, Math.min(2, glyphs.length)).join('')
}

function hasStreamingRichContentHints(content: string): boolean {
  return (
    content.includes('```') ||
    content.includes('|') ||
    content.includes('![') ||
    content.includes('http') ||
    content.includes('[[TYPELESS_CARD:') ||
    content.includes('<!-- process-start -->')
  )
}

function shouldFlushStreamingRevealImmediately(content: string): boolean {
  return (
    !!props.preferImmediateStreamingRender ||
    content.length >= STREAMING_REVEAL_MAX_CHARS ||
    hasStreamingRichContentHints(content) ||
    streamToolExecuting.value ||
    streamAwaitingConfirmation.value ||
    streamPhase.value === 'recovering' ||
    streamPhase.value === 'interrupted' ||
    streamPhase.value === 'awaiting_confirmation'
  )
}

function advanceStreamingReveal() {
  streamingRevealRafId = null
  if (revealedStreamingContent.value === streamingRevealTarget) return
  if (shouldFlushStreamingRevealImmediately(streamingRevealTarget)) {
    flushStreamingReveal()
    return
  }

  const delta = streamingRevealTarget.slice(revealedStreamingContent.value.length)
  if (!delta) {
    flushStreamingReveal()
    return
  }

  const nextChunk = nextStreamingRevealChunk(delta, revealedStreamingContent.value.length === 0)
  revealedStreamingContent.value += nextChunk

  if (revealedStreamingContent.value !== streamingRevealTarget) {
    streamingRevealRafId = scheduleStreamingRevealFrame(advanceStreamingReveal)
  }
}

function scheduleStreamingReveal(nextContent: string) {
  streamingRevealTarget = nextContent
  lastStreamingTargetAt = Date.now()

  if (!trackStreamingState.value || shouldFlushStreamingRevealImmediately(nextContent)) {
    flushStreamingReveal(nextContent)
    return
  }

  if (
    !nextContent.startsWith(revealedStreamingContent.value) ||
    RE_STREAMING_IMMEDIATE_FLUSH.test(nextContent.slice(revealedStreamingContent.value.length))
  ) {
    flushStreamingReveal(nextContent)
    return
  }

  if (revealedStreamingContent.value === '' && nextContent) {
    revealedStreamingContent.value = nextStreamingRevealChunk(nextContent, true)
  }

  if (revealedStreamingContent.value === nextContent) {
    clearStreamingRevealRaf()
    return
  }

  if (streamingRevealRafId === null) {
    streamingRevealRafId = scheduleStreamingRevealFrame(advanceStreamingReveal)
  }
}

watch(
  () => props.message.content,
  (content) => {
    if (trackStreamingState.value) {
      scheduleStreamingReveal(content)
      return
    }
    flushStreamingReveal(content)
  },
  { immediate: true }
)

watch(
  () => trackStreamingState.value,
  (trackStreaming) => {
    if (!trackStreaming) {
      flushStreamingReveal(props.message.content)
      return
    }
    scheduleStreamingReveal(props.message.content)
  }
)

watch(
  () =>
    [
      trackStreamingState.value,
      streamToolExecuting.value,
      streamAwaitingConfirmation.value,
      streamPhase.value,
      props.preferImmediateStreamingRender ?? false,
    ] as const,
  ([trackStreaming, toolExecuting, awaitingConfirmation, phase]) => {
    if (
      !trackStreaming ||
      toolExecuting ||
      awaitingConfirmation ||
      phase === 'recovering' ||
      phase === 'interrupted'
    ) {
      flushStreamingReveal(props.message.content)
      return
    }
    scheduleStreamingReveal(props.message.content)
  }
)

const renderSourceContent = computed(() =>
  trackStreamingState.value ? revealedStreamingContent.value : props.message.content
)
const showStreamingCaret = computed(() => {
  if (!trackStreamingState.value) return false
  const phase = streamPhase.value
  if (!phase) return true
  return (
    phase === 'connecting' ||
    phase === 'streaming' ||
    phase === 'executing' ||
    phase === 'recovering' ||
    phase === 'awaiting_confirmation'
  )
})

const STREAMING_TYPELESS_HINT_TAIL = 4
const RE_STREAMING_ORDERED_LIST_HINT = /\d+\.\s/
const streamingTypelessHintContent = ref(renderSourceContent.value)
const streamingMayContainTypelessCards = ref(true)

function hasStreamingTypelessHints(content: string): boolean {
  if (
    content.includes('```') ||
    content.includes('|') ||
    content.includes('- ') ||
    content.includes('* ') ||
    content.includes('+ ') ||
    content.includes('![') ||
    content.includes('http')
  ) {
    return true
  }
  return content.includes('.') && RE_STREAMING_ORDERED_LIST_HINT.test(content)
}

watch(
  () => renderSourceContent.value,
  (content) => {
    if (!trackStreamingState.value) {
      streamingTypelessHintContent.value = content
      streamingMayContainTypelessCards.value = true
      return
    }

    const prev = streamingTypelessHintContent.value
    if (content.startsWith(prev)) {
      if (!streamingMayContainTypelessCards.value) {
        const appended = content.slice(prev.length)
        const tailProbe = prev.slice(-STREAMING_TYPELESS_HINT_TAIL) + appended
        if (hasStreamingTypelessHints(tailProbe)) {
          streamingMayContainTypelessCards.value = true
        }
      }
      streamingTypelessHintContent.value = content
      return
    }

    // Fallback for non-append updates/replacements.
    streamingMayContainTypelessCards.value = hasStreamingTypelessHints(content)
    streamingTypelessHintContent.value = content
  },
  { immediate: true }
)

watch(
  () => trackStreamingState.value,
  () => {
    const content = renderSourceContent.value
    streamingTypelessHintContent.value = content
    streamingMayContainTypelessCards.value = trackStreamingState.value
      ? hasStreamingTypelessHints(content)
      : true
  },
  { immediate: true }
)

function renderMarkdownForMessage(content: string, scope: string): string {
  if (!trackStreamingState.value) {
    return renderMarkdownCached(content, scope)
  }
  recordChatPerfCount('chat_message.render_markdown_cached.calls')
  return measureChatPerf('chat_message.render_markdown_cached', () =>
    renderMarkdownCached(content, scope)
  )
}

function hasTypelessCardsForMessage(content: string, useCache = true): boolean {
  if (!trackStreamingState.value) {
    return hasTypelessCards(content, useCache)
  }
  recordChatPerfCount('chat_message.has_typeless_cards.calls')
  return measureChatPerf('chat_message.has_typeless_cards', () =>
    hasTypelessCards(content, useCache)
  )
}

function parseTypelessContentIncrementalForMessage(
  content: string,
  renderMessageId: string,
  conversationId?: string,
  explicitTodoCardId?: string
) {
  recordChatPerfCount('chat_message.parse_typeless_content_incremental.calls')
  return measureChatPerf('chat_message.parse_typeless_content_incremental', () =>
    parseTypelessContentIncremental(content, renderMessageId, conversationId, explicitTodoCardId)
  )
}

// Extract inline markdown images from user message content (e.g. ![image](/api/media/...))
const inlineImages = computed(() => {
  return parsedInlineContent.value?.images ?? []
})

// User message text with inline image markdown stripped
const userTextContent = computed(() => {
  if (!isUser.value) return props.message.content
  return parsedInlineContent.value?.strippedText ?? props.message.content
})

// Copy button state
const copyState = ref<'idle' | 'copied'>('idle')

// Mobile long-press state
const showMobileActions = ref(false)
const longPressTimer = ref<number | null>(null)
const longPressThreshold = 500 // ms
const isEditingUserMessage = ref(false)
const editedUserMessageContent = ref('')
const userEditTextareaRef = ref<HTMLTextAreaElement | null>(null)

const canEditUserMessage = computed(() => {
  if (!isUser.value || props.isStreaming || isMultiSelectMode.value) return false
  if (props.message.id.startsWith('temp-') || props.message.id.startsWith('streaming-'))
    return false
  return !isSpecialUserPlaceholder.value
})

const canSubmitEditedUserMessage = computed(() => {
  if (!isEditingUserMessage.value) return false
  return editedUserMessageContent.value.trim().length > 0 || hasAttachments.value
})

watch(
  () => props.message.content,
  (content) => {
    if (isEditingUserMessage.value) return
    editedUserMessageContent.value = content
  },
  { immediate: true }
)

// TTS playback state
const isSpeaking = ref(false)
const ttsError = ref<string | null>(null)

type AttachmentPreviewState = {
  type: 'image' | 'text' | 'markdown' | 'pdf' | 'file'
  src: string
  name: string
  mimeType?: string
  content?: string
}

// Attachment preview state
const previewAttachment = ref<AttachmentPreviewState | null>(null)

const previewAttachmentHtml = computed(() => {
  if (previewAttachment.value?.type !== 'markdown') return ''
  return renderMarkdownForMessage(
    previewAttachment.value.content || '',
    `attachment-preview:${previewAttachment.value.name}`
  )
})

const previewAttachmentBadge = computed(() => {
  if (!previewAttachment.value) return ''

  const filename = previewAttachment.value.name || ''
  const ext = filename.includes('.') ? filename.split('.').pop()?.trim().toUpperCase() || '' : ''
  if (ext) return ext.length <= 6 ? ext : ext.slice(0, 6)

  if (previewAttachment.value.type === 'markdown') return 'MD'
  if (previewAttachment.value.type === 'pdf') return 'PDF'
  if (previewAttachment.value.type === 'image') return 'IMAGE'
  if (previewAttachment.value.type === 'text') return 'TEXT'
  return 'FILE'
})

const previewAttachmentKindLabel = computed(() => {
  if (!previewAttachment.value) return ''
  if (previewAttachment.value.type === 'markdown') return 'Markdown preview'
  if (previewAttachment.value.type === 'text') return 'Text preview'
  if (previewAttachment.value.type === 'pdf') return 'PDF preview'
  if (previewAttachment.value.type === 'image') return 'Image preview'
  return 'File preview'
})

// Voice message playback state
const playingAudioId = ref<number | null>(null)
const audioProgress = ref(0) // 0-1
let activeAudio: HTMLAudioElement | null = null
let audioProgressTimer: ReturnType<typeof setInterval> | null = null

function playVoiceMessage(
  attachment: { mime_type: string; data: string; duration?: number },
  index: number
) {
  // If same audio is playing, stop it
  if (playingAudioId.value === index) {
    stopVoiceMessage()
    return
  }
  // Stop any currently playing audio
  stopVoiceMessage()

  const src = `data:${attachment.mime_type};base64,${attachment.data}`
  activeAudio = new Audio(src)
  playingAudioId.value = index
  audioProgress.value = 0

  activeAudio.onplay = () => {
    audioProgressTimer = setInterval(() => {
      if (activeAudio && activeAudio.duration) {
        audioProgress.value = activeAudio.currentTime / activeAudio.duration
      }
    }, 50)
  }
  activeAudio.onended = () => {
    stopVoiceMessage()
  }
  activeAudio.onerror = () => {
    stopVoiceMessage()
  }
  activeAudio.play()
}

function stopVoiceMessage() {
  if (activeAudio) {
    activeAudio.pause()
    activeAudio.src = ''
    activeAudio = null
  }
  if (audioProgressTimer) {
    clearInterval(audioProgressTimer)
    audioProgressTimer = null
  }
  playingAudioId.value = null
  audioProgress.value = 0
}

function formatVoiceDuration(seconds?: number): string {
  if (!seconds || seconds < 1) return '1″'
  if (seconds < 60) return `${Math.round(seconds)}″`
  const m = Math.floor(seconds / 60)
  const s = Math.round(seconds % 60)
  return `${m}′${s.toString().padStart(2, '0')}″`
}

// Compute voice bubble width based on duration (WeChat style: longer = wider)
function voiceBubbleWidth(seconds?: number): string {
  const dur = seconds || 1
  // Min 80px, max 240px, logarithmic scale
  const width = Math.min(240, Math.max(80, 80 + Math.log2(dur) * 40))
  return `${Math.round(width)}px`
}

// Derive display names for tool pill: extract skill names from "blue <subcommand>" commands
const toolDisplayNames = computed(() => {
  if (!trackStreamingState.value || !streamToolExecuting.value) return []
  const commands = streamToolExecutingCommands.value
  if (commands.length > 0) {
    // Extract skill name from "blue <subcommand> ..." pattern
    return commands.map((cmd) => {
      const m = cmd.match(/^blue\s+(\S+)/)
      return m ? m[1] : formatToolName('exec')
    })
  }
  // Fallback to tool names
  return streamToolExecutingNames.value.map(formatToolName)
})

const processDetailsExpanded = ref(settingsStore.showToolDetails)

watch(
  () => settingsStore.showToolDetails,
  (show) => {
    processDetailsExpanded.value = show
  }
)

function getLatestProcessTraceByPriority(
  categories: Array<ProcessTraceItem['category']>,
  statuses: Array<ProcessTraceItem['status']> = ['active', 'pending', 'error']
): ProcessTraceItem | null {
  for (let i = streamProcessTrace.value.length - 1; i >= 0; i--) {
    const item = streamProcessTrace.value[i]
    if (!item) continue
    if (!categories.includes(item.category)) continue
    if (!statuses.includes(item.status)) continue
    return item
  }
  return null
}

const activeRecoveryProcessTrace = computed(() =>
  trackStreamingState.value ? getLatestProcessTraceByPriority(['retry', 'recovery']) : null
)

const activeLifecycleProcessTrace = computed(() =>
  trackStreamingState.value ? getLatestProcessTraceByPriority(['lifecycle', 'confirmation']) : null
)

const orderedStreamingProcessTrace = computed(() => {
  if (!trackStreamingState.value || !isAssistant.value) return []
  return [...streamProcessTrace.value].sort((a, b) => {
    const rankA = a.category === 'summary' ? 0 : 1
    const rankB = b.category === 'summary' ? 0 : 1
    if (rankA !== rankB) return rankA - rankB
    return a.timestamp - b.timestamp
  })
})

function getProcessTraceToneClass(item: ProcessTraceItem): string {
  if (item.status === 'error') return 'assistant-process-trace-dot--error'
  if (item.status === 'active' || item.status === 'pending') {
    return 'assistant-process-trace-dot--active'
  }
  if (item.status === 'success') return 'assistant-process-trace-dot--success'
  return 'assistant-process-trace-dot--info'
}

const hasStreamingProcessDetails = computed(
  () =>
    trackStreamingState.value &&
    (orderedStreamingProcessTrace.value.length > 0 || streamToolResults.value.length > 0)
)

const hasPersistedProcessDetails = computed(
  () => !props.isStreaming && effectiveProcessToolResults.value.length > 0
)

const showProcessDetailsToggle = computed(
  () => isAssistant.value && (hasStreamingProcessDetails.value || hasPersistedProcessDetails.value)
)

const showStreamingProcessPanel = computed(
  () => hasStreamingProcessDetails.value && processDetailsExpanded.value
)

const showPersistedProcessPanel = computed(
  () => hasPersistedProcessDetails.value && processDetailsExpanded.value
)

const assistantStatusLabel = computed(() => {
  if (!trackStreamingState.value) return ''
  if (streamAwaitingConfirmation.value) {
    return t('chat.awaitingConfirmation', 'Waiting for your confirmation to continue')
  }
  if (activeRecoveryProcessTrace.value?.label) {
    return activeRecoveryProcessTrace.value.label
  }
  if (streamToolExecuting.value && streamStatusSummary.value) return streamStatusSummary.value
  if (streamStatusSummary.value) return streamStatusSummary.value
  if (streamProgress.value) return streamProgress.value
  if (activeLifecycleProcessTrace.value?.label) {
    return activeLifecycleProcessTrace.value.label
  }
  return t('chat.waitingThinking', 'Thinking...')
})

const showAssistantStatusBar = computed(
  () =>
    !showExternalStatusRail.value &&
    trackStreamingState.value &&
    isAssistant.value &&
    !!assistantStatusLabel.value
)
const showAssistantStatusOnly = computed(() => isContentEmpty.value && showAssistantStatusBar.value)
const assistantStatusVariantClass = computed(() => {
  if (streamAwaitingConfirmation.value) return 'assistant-status-pill-warn'
  if (activeRecoveryProcessTrace.value) return 'assistant-status-pill-active'
  if (streamToolExecuting.value) return 'assistant-status-pill-active'
  return 'assistant-status-pill-idle'
})

function toggleProcessDetails() {
  processDetailsExpanded.value = !processDetailsExpanded.value
}

const assistantStatusElapsedSeconds = ref('')
let assistantStatusTimerHandle: ReturnType<typeof setInterval> | null = null

function syncAssistantStatusElapsed() {
  if (!streamStatusStartedAt.value) {
    assistantStatusElapsedSeconds.value = ''
    return
  }
  assistantStatusElapsedSeconds.value = ((Date.now() - streamStatusStartedAt.value) / 1000).toFixed(1)
}

function stopAssistantStatusTimer() {
  if (!assistantStatusTimerHandle) return
  clearInterval(assistantStatusTimerHandle)
  assistantStatusTimerHandle = null
}

watch(
  () => [showAssistantStatusBar.value, streamStatusStartedAt.value] as const,
  ([visible, startedAt]) => {
    stopAssistantStatusTimer()
    if (!visible || !startedAt) {
      assistantStatusElapsedSeconds.value = ''
      return
    }
    syncAssistantStatusElapsed()
    assistantStatusTimerHandle = setInterval(syncAssistantStatusElapsed, 100)
  },
  { immediate: true }
)

const interruptedIndicatorHtml = computed(
  () =>
    `<div class="response-interrupted-indicator"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg><span>${t('chat.responseInterrupted')}</span></div>`
)

const INTERRUPTED_MARKER = '[Response interrupted]'
const RE_NON_WHITESPACE = /\S/
const RE_EDGE_WHITESPACE = /^\s|\s$/
const INTERRUPTED_STRIP_CACHE_KEY = '__zima_chat_interrupted_strip_cache_v1__'
const INTERRUPTED_STRIP_CACHE_MAX = 300

type InterruptedStripResult = { content: string; interrupted: boolean }

function getInterruptedStripCache(): Map<string, InterruptedStripResult> {
  const g = globalThis as Record<string, unknown>
  const existing = g[INTERRUPTED_STRIP_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, InterruptedStripResult>
  }
  const cache = new Map<string, InterruptedStripResult>()
  g[INTERRUPTED_STRIP_CACHE_KEY] = cache
  return cache
}

function hasNonWhitespace(text: string): boolean {
  return RE_NON_WHITESPACE.test(text)
}

function trimIfNeeded(text: string): string {
  return RE_EDGE_WHITESPACE.test(text) ? text.trim() : text
}

// Strip [Response interrupted] marker from content, returns { content, interrupted }
function stripInterruptedMarker(content: string): InterruptedStripResult {
  const cache = getInterruptedStripCache()
  const cached = cache.get(content)
  if (cached) return cached

  const markerIndex = content.lastIndexOf(INTERRUPTED_MARKER)
  if (markerIndex === -1) {
    const result = { content, interrupted: false }
    if (cache.size >= INTERRUPTED_STRIP_CACHE_MAX && !cache.has(content)) {
      evictOldestMapEntry(cache)
    }
    cache.set(content, result)
    return result
  }

  const markerEnd = markerIndex + INTERRUPTED_MARKER.length
  if (markerEnd < content.length && hasNonWhitespace(content.slice(markerEnd))) {
    const result = { content, interrupted: false }
    if (cache.size >= INTERRUPTED_STRIP_CACHE_MAX && !cache.has(content)) {
      evictOldestMapEntry(cache)
    }
    cache.set(content, result)
    return result
  }

  let cutStart = markerIndex
  if (cutStart > 0 && content.charCodeAt(cutStart - 1) === 10) cutStart -= 1
  if (cutStart > 0 && content.charCodeAt(cutStart - 1) === 10) cutStart -= 1

  const result = {
    content: content.slice(0, cutStart),
    interrupted: true,
  }
  if (cache.size >= INTERRUPTED_STRIP_CACHE_MAX && !cache.has(content)) {
    evictOldestMapEntry(cache)
  }
  cache.set(content, result)
  return result
}

// Cache stripped content to avoid double stripFirstLineHeading + SILENT_REPLY replacement
const strippedContent = computed(() => {
  if (isUser.value) return renderSourceContent.value
  let content = stripFirstLineHeading(renderSourceContent.value)
  if (content.includes('[SILENT_REPLY]')) {
    content = content.replace(/\[SILENT_REPLY\]/g, '💤')
  }
  return content
})

// Regex to strip process content (tool results) and typeless card blocks
const RE_PROCESS_BLOCK = /\n*<!--\s*process-start\s*-->[\s\S]*?<!--\s*process-end\s*-->\n*/g
const RE_PROCESS_FENCE = /```process\s*([\s\S]*?)```/g
const RE_TYPELESS_BLOCK = /\n*```typeless\s*[\s\S]*?```\n*/g
const PROCESS_STRIP_CACHE_KEY = '__zima_chat_process_strip_cache_v1__'
const PROCESS_STRIP_CACHE_MAX = 300
const PROCESS_TOOL_RESULTS_CACHE_KEY = '__zima_chat_process_tool_results_cache_v1__'
const PROCESS_TOOL_RESULTS_CACHE_MAX = 200

function getProcessStripCache(): Map<string, string> {
  const g = globalThis as Record<string, unknown>
  const existing = g[PROCESS_STRIP_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, string>
  }
  const cache = new Map<string, string>()
  g[PROCESS_STRIP_CACHE_KEY] = cache
  return cache
}

function getProcessToolResultsCache(): Map<string, ToolResultItem[]> {
  const g = globalThis as Record<string, unknown>
  const existing = g[PROCESS_TOOL_RESULTS_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, ToolResultItem[]>
  }
  const cache = new Map<string, ToolResultItem[]>()
  g[PROCESS_TOOL_RESULTS_CACHE_KEY] = cache
  return cache
}

function stripProcessBlocks(text: string): string {
  if (!text.includes('<!-- process-start -->')) return text
  return trimIfNeeded(text.replace(RE_PROCESS_BLOCK, '\n'))
}

// Strip process content (tool results + typeless cards) from text
function stripProcessContent(text: string): string {
  const cache = getProcessStripCache()
  const cached = cache.get(text)
  if (cached !== undefined) return cached

  const hasProcessBlock = text.includes('<!-- process-start -->')
  const hasTypelessBlock = text.includes('```typeless')
  let stripped: string

  if (!hasProcessBlock && !hasTypelessBlock) {
    stripped = trimIfNeeded(text)
  } else {
    stripped = hasProcessBlock ? stripProcessBlocks(text) : text
    if (hasTypelessBlock) {
      stripped = stripped.replace(RE_TYPELESS_BLOCK, '\n')
    }
    stripped = trimIfNeeded(stripped)
  }

  if (cache.size >= PROCESS_STRIP_CACHE_MAX && !cache.has(text)) {
    evictOldestMapEntry(cache)
  }
  cache.set(text, stripped)
  return stripped
}

function parsePersistedProcessToolResults(
  content: string,
  messageId: string,
  createdAt?: string
): ToolResultItem[] {
  const cacheKey = `${messageId}:${content}`
  const cache = getProcessToolResultsCache()
  const cached = cache.get(cacheKey)
  if (cached) return cached

  if (!content.includes('```process')) {
    if (cache.size >= PROCESS_TOOL_RESULTS_CACHE_MAX && !cache.has(cacheKey)) {
      evictOldestMapEntry(cache)
    }
    cache.set(cacheKey, [])
    return []
  }

  const timestamp = createdAt ? Date.parse(createdAt) || 0 : 0
  const items: ToolResultItem[] = []
  let match: RegExpExecArray | null
  let blockIndex = 0
  RE_PROCESS_FENCE.lastIndex = 0

  while ((match = RE_PROCESS_FENCE.exec(content)) !== null) {
    const rawPayload = (match[1] || '').trim()
    if (!rawPayload) {
      blockIndex += 1
      continue
    }
    try {
      const parsed = JSON.parse(rawPayload)
      const entries = Array.isArray(parsed) ? parsed : [parsed]
      entries.forEach((entry, itemIndex) => {
        if (!entry || typeof entry !== 'object') return
        const payload = entry as Record<string, unknown>
        const command =
          typeof payload.cmd === 'string'
            ? payload.cmd.trim()
            : typeof payload.command === 'string'
              ? payload.command.trim()
              : ''
        const status = typeof payload.status === 'string' ? payload.status.trim() : ''
        const output = typeof payload.output === 'string' ? payload.output : ''
        const name =
          typeof payload.tool === 'string'
            ? payload.tool.trim()
            : typeof payload.name === 'string'
              ? payload.name.trim()
              : ''
        const rawIcon = typeof payload.icon === 'string' ? payload.icon : ''
        const icon: ToolResultItem['icon'] =
          rawIcon === '✓' || rawIcon === '✗' || rawIcon === '⏳'
            ? rawIcon
            : /(error|fail)/i.test(status)
              ? '✗'
              : status
                ? '✓'
                : '⏳'

        if (!command && !status && !output) return

        items.push({
          name: name || 'tool',
          id: `${messageId}-process-${blockIndex}-${itemIndex}`,
          command,
          icon,
          status,
          output,
          timestamp,
        })
      })
    } catch {
      // Ignore malformed persisted process blocks.
    }
    blockIndex += 1
  }

  if (cache.size >= PROCESS_TOOL_RESULTS_CACHE_MAX && !cache.has(cacheKey)) {
    evictOldestMapEntry(cache)
  }
  cache.set(cacheKey, items)
  return items
}

const persistedProcessToolResults = computed(() => {
  if (isUser.value || props.isStreaming) return []
  return parsePersistedProcessToolResults(
    strippedContent.value,
    props.message.id,
    props.message.created_at
  )
})

const localProcessToolResults = computed(() => {
  if (isUser.value || props.isStreaming) return []
  const runtimeMsg = props.message as RuntimeProcessMessage
  return runtimeMsg.local_process_tool_results || []
})

const effectiveProcessToolResults = computed(() =>
  persistedProcessToolResults.value.length > 0
    ? persistedProcessToolResults.value
    : localProcessToolResults.value
)

const contentWithoutProcessBlocks = computed(() => {
  if (isUser.value) return renderSourceContent.value
  return stripProcessBlocks(strippedContent.value)
})

const displayContentWithoutProcessBlocks = computed(() => {
  if (!isAssistant.value) return contentWithoutProcessBlocks.value
  return stripDuplicateTodoChecklistForMessage(chatStore.messages, {
    ...props.message,
    content: contentWithoutProcessBlocks.value,
  })
})

type AssistantTextState = { html: string; isEmpty: boolean }
interface AssistantTextStateCacheEntry {
  text: string
  showToolDetails: boolean
  interruptedHtml: string
  value: AssistantTextState
}

const ASSISTANT_TEXT_STATE_CACHE_KEY = '__zima_chat_assistant_text_state_cache_v1__'
const ASSISTANT_TEXT_STATE_CACHE_MAX = 400

function getAssistantTextStateCache(): Map<string, AssistantTextStateCacheEntry> {
  const g = globalThis as Record<string, unknown>
  const existing = g[ASSISTANT_TEXT_STATE_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, AssistantTextStateCacheEntry>
  }
  const cache = new Map<string, AssistantTextStateCacheEntry>()
  g[ASSISTANT_TEXT_STATE_CACHE_KEY] = cache
  return cache
}

function getAssistantTextStateCacheKey(conversationId: string, messageId: string): string {
  return `${conversationId || 'unknown'}:${messageId}`
}

const assistantTextState = computed<AssistantTextState>(() => {
  if (isUser.value) {
    return { html: props.message.content, isEmpty: false }
  }
  // Card messages are rendered via segment pipeline; skip markdown rendering here.
  if (effectiveHasCards.value) {
    return { html: '', isEmpty: false }
  }

  let text = displayContentWithoutProcessBlocks.value
  // When tool details are hidden, strip process blocks and typeless card blocks
  if (!settingsStore.showToolDetails) {
    text = stripProcessContent(text)
  }

  const showToolDetails = settingsStore.showToolDetails
  const interruptedHtml = interruptedIndicatorHtml.value
  const useCache = !props.isStreaming
  const cacheKey = useCache
    ? getAssistantTextStateCacheKey(props.message.conversation_id, props.message.id)
    : ''
  const cache = useCache ? getAssistantTextStateCache() : null

  if (cache) {
    const cached = cache.get(cacheKey)
    if (
      cached &&
      cached.text === text &&
      cached.showToolDetails === showToolDetails &&
      cached.interruptedHtml === interruptedHtml
    ) {
      return cached.value
    }
  }

  const saveCache = (value: AssistantTextState): AssistantTextState => {
    if (!cache) return value
    if (cache.size >= ASSISTANT_TEXT_STATE_CACHE_MAX && !cache.has(cacheKey)) {
      evictOldestMapEntry(cache)
    }
    cache.set(cacheKey, {
      text,
      showToolDetails,
      interruptedHtml,
      value,
    })
    return value
  }

  if (!text || !hasNonWhitespace(text)) {
    return saveCache({ html: '', isEmpty: true })
  }

  const { content: cleaned, interrupted } = stripInterruptedMarker(text)
  let html = renderMarkdownForMessage(cleaned, `chat-message:${props.message.id}`)
  if (interrupted) {
    html += interruptedHtml
  }
  return saveCache({ html, isEmpty: false })
})

// Whether bubble content is empty (only indicators showing)
const isContentEmpty = computed(() => {
  return (
    assistantTextState.value.isEmpty &&
    !showProcessDetailsToggle.value &&
    (!props.isStreaming || streamToolResults.value.length === 0)
  )
})

// Hide empty assistant messages that are not streaming (collapsed empty bubbles)
const shouldHideMessage = computed(() => {
  if (isUser.value || props.isStreaming) return false
  return isContentEmpty.value && !hasMediaTask.value
})

// Parse typeless cards from assistant messages
// Use incremental parsing for streaming messages, regular parsing for completed messages
const parsedContent = computed(() => {
  const content = displayContentWithoutProcessBlocks.value
  const renderMessageId = props.message.render_key || props.message.id
  const explicitTodoCardId = props.message.todo_card_id?.trim()
  if (isUser.value) {
    return null
  }

  // Streaming fast path: skip full typeless detection until hints appear.
  if (props.isStreaming && !streamingMayContainTypelessCards.value) {
    return null
  }

  if (!hasTypelessCardsForMessage(content, !props.isStreaming)) {
    return null
  }
  // Use incremental parsing for streaming to avoid re-parsing entire content
  // Pass conversation_id to ensure cache key uniqueness across conversations
  if (props.isStreaming) {
    return parseTypelessContentIncrementalForMessage(
      content,
      renderMessageId,
      props.message.conversation_id,
      explicitTodoCardId
    )
  }
  return parseTypelessContent(
    content,
    renderMessageId,
    props.message.conversation_id,
    explicitTodoCardId
  )
})

type ContentSegment = {
  type: 'text' | 'card'
  content: string | TypelessCard
  key: string
}

function getNextTextSegmentIndex(segments: ContentSegment[]): number {
  let maxIndex = -1
  for (const segment of segments) {
    if (segment.type !== 'text') continue
    const marker = segment.key.lastIndexOf('-text-')
    if (marker === -1) continue
    const index = Number.parseInt(segment.key.slice(marker + 6), 10)
    if (Number.isInteger(index) && index > maxIndex) {
      maxIndex = index
    }
  }
  return maxIndex + 1
}

// Card types that represent content or results — always visible even when tool details are hidden.
// Only tool-invocation cards (steps) are hidden when details are off.
const RESULT_CARD_TYPES = new Set([
  'result',
  'search',
  'weather',
  'chart',
  'gallery',
  'map',
  'profile',
  'rating',
  'comparison',
  'metric',
  'link',
  'audio',
  'video',
  'file',
  'media-generate',
  'ui-review',
  'deep-research',
  'detection',
  'ui-review-progress',
  'analyze-progress',
  'browser-progress',
  'deep-research-timeline',
  'deep-research-progress',
  'deep-research-event',
  'web-fetch',
  'convert-task',
  'list',
  'table',
  'code',
  'terminal',
  'mermaid',
  'accordion',
])

function buildEffectiveSegments(
  parsed: ParsedContent | null,
  conversationId: string,
  messageId: string,
  showToolDetails: boolean,
  isStreaming: boolean
): ContentSegment[] | null {
  if (!parsed || parsed.cards.length === 0) return null

  const convId = conversationId || 'unknown'
  const cacheKey = `${convId}:${messageId}:${showToolDetails ? '1' : '0'}`
  const splitIncrementalKey = isStreaming ? `${convId}:${messageId}` : undefined
  const perParsedCache = getEffectiveSegmentsCacheForParsed(parsed)
  if (perParsedCache.has(cacheKey)) {
    return perParsedCache.get(cacheKey) ?? null
  }
  const incrementalCache = getEffectiveSegmentsIncrementalCache()
  const incremental = incrementalCache.get(cacheKey)
  if (
    incremental &&
    incremental.parsedCards === parsed.cards &&
    parsed.text.startsWith(incremental.parsedText)
  ) {
    const delta = parsed.text.slice(incremental.parsedText.length)
    if (!delta.includes('[[TYPELESS_CARD:')) {
      if (incremental.value === null) {
        setEffectiveSegmentsIncrementalCache(
          incrementalCache,
          cacheKey,
          parsed.text,
          parsed.cards,
          null
        )
        setEffectiveSegmentsCacheForParsed(parsed, perParsedCache, cacheKey, null)
        return null
      }

      const previous = incremental.value
      const lastSegment = previous[previous.length - 1]
      if (lastSegment?.type === 'text') {
        const nextTailText = trimIfNeeded((lastSegment.content as string) + delta)
        if (nextTailText === lastSegment.content) {
          setEffectiveSegmentsIncrementalCache(
            incrementalCache,
            cacheKey,
            parsed.text,
            parsed.cards,
            previous
          )
          setEffectiveSegmentsCacheForParsed(parsed, perParsedCache, cacheKey, previous)
          return previous
        }

        const next = previous.slice()
        next[next.length - 1] = {
          ...lastSegment,
          content: nextTailText,
        }
        setEffectiveSegmentsIncrementalCache(
          incrementalCache,
          cacheKey,
          parsed.text,
          parsed.cards,
          next
        )
        setEffectiveSegmentsCacheForParsed(parsed, perParsedCache, cacheKey, next)
        return next
      }

      const deltaText = trimIfNeeded(delta)
      // No trailing text segment to extend; whitespace-only append keeps content unchanged.
      if (!deltaText) {
        setEffectiveSegmentsIncrementalCache(
          incrementalCache,
          cacheKey,
          parsed.text,
          parsed.cards,
          previous
        )
        setEffectiveSegmentsCacheForParsed(parsed, perParsedCache, cacheKey, previous)
        return previous
      }

      const next = previous.slice()
      next.push({
        type: 'text',
        content: deltaText,
        key: `${convId}-${messageId}-text-${getNextTextSegmentIndex(previous)}`,
      })
      setEffectiveSegmentsIncrementalCache(
        incrementalCache,
        cacheKey,
        parsed.text,
        parsed.cards,
        next
      )
      setEffectiveSegmentsCacheForParsed(parsed, perParsedCache, cacheKey, next)
      return next
    }
  }

  // If tool details are hidden and no result-type cards exist, skip split/filter work.
  if (!showToolDetails && !parsed.cards.some((card) => RESULT_CARD_TYPES.has(card.type))) {
    setEffectiveSegmentsIncrementalCache(
      incrementalCache,
      cacheKey,
      parsed.text,
      parsed.cards,
      null
    )
    setEffectiveSegmentsCacheForParsed(parsed, perParsedCache, cacheKey, null)
    return null
  }

  const segments = splitIntoSegments(parsed.text, parsed.cards, splitIncrementalKey)
  if (segments.length === 0) {
    setEffectiveSegmentsIncrementalCache(
      incrementalCache,
      cacheKey,
      parsed.text,
      parsed.cards,
      null
    )
    setEffectiveSegmentsCacheForParsed(parsed, perParsedCache, cacheKey, null)
    return null
  }

  const effective: ContentSegment[] = []
  let hasCards = false

  for (let index = 0; index < segments.length; index++) {
    const segment = segments[index]
    if (!segment) continue

    if (segment.type === 'card') {
      const card = segment.content as TypelessCard
      if (!showToolDetails && !RESULT_CARD_TYPES.has(card.type)) {
        continue
      }
      hasCards = true
      effective.push({
        type: 'card',
        content: card,
        key: `${convId}-${messageId}-card-${card.id || index}`,
      })
      continue
    }

    effective.push({
      type: 'text',
      content: segment.content as string,
      key: `${convId}-${messageId}-text-${index}`,
    })
  }

  // If no cards survived filtering, return null to fall back to plain text rendering.
  const result = hasCards ? effective : null
  setEffectiveSegmentsIncrementalCache(
    incrementalCache,
    cacheKey,
    parsed.text,
    parsed.cards,
    result
  )
  setEffectiveSegmentsCacheForParsed(parsed, perParsedCache, cacheKey, result)
  return result
}

type RenderedContentSegment = ContentSegment | (ContentSegment & { type: 'text'; html: string })
type SegmentRenderState = {
  rendered: RenderedContentSegment[] | null
  cardOnly: ContentSegment[] | null
  hasCards: boolean
  isCardOnly: boolean
}

const EMPTY_SEGMENT_RENDER_STATE: SegmentRenderState = {
  rendered: null,
  cardOnly: null,
  hasCards: false,
  isCardOnly: false,
}

function shouldKeepAssistantBubbleForCardOnly(cards: ContentSegment[]): boolean {
  return (
    cards.length > 0 &&
    cards.every((segment) => (segment.content as TypelessCard).type === 'deep-research-timeline')
  )
}

const SEGMENT_RENDER_CACHE_KEY = '__zima_chat_segment_render_cache_v1__'
const SEGMENT_RENDER_CACHE_MAX = 300
const CHAT_HASH_CACHE_KEY = '__zima_chat_hash_cache_v1__'
const CHAT_HASH_CACHE_MAX = 500
const STREAMING_SEGMENT_HTML_CACHE_KEY = '__zima_chat_streaming_segment_html_cache_v1__'
const STREAMING_SEGMENT_HTML_CACHE_MAX = 600
const STREAMING_SEGMENT_RENDER_INCREMENTAL_CACHE_KEY =
  '__zima_chat_streaming_segment_render_incremental_cache_v1__'
const STREAMING_SEGMENT_RENDER_INCREMENTAL_CACHE_MAX = 300
const EFFECTIVE_SEGMENTS_CACHE_KEY = '__zima_chat_effective_segments_cache_v1__'
const EFFECTIVE_SEGMENTS_CACHE_PER_PARSED_MAX = 8
const EFFECTIVE_SEGMENTS_INCREMENTAL_CACHE_KEY =
  '__zima_chat_effective_segments_incremental_cache_v1__'
const EFFECTIVE_SEGMENTS_INCREMENTAL_CACHE_MAX = 300

interface StreamingSegmentHtmlCacheEntry {
  sourceText: string
  showToolDetails: boolean
  interruptedHtml: string
  html: string
}

type EffectiveSegmentsCacheEntry = Map<string, ContentSegment[] | null>
type EffectiveSegmentsIncrementalCacheEntry = {
  parsedText: string
  parsedCards: TypelessCard[]
  value: ContentSegment[] | null
}
type StreamingSegmentRenderIncrementalCacheEntry = {
  segments: ContentSegment[]
  showToolDetails: boolean
  interruptedHtml: string
  state: SegmentRenderState
}

function getSegmentRenderCache(): Map<string, SegmentRenderState> {
  const g = globalThis as Record<string, unknown>
  const existing = g[SEGMENT_RENDER_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, SegmentRenderState>
  }
  const cache = new Map<string, SegmentRenderState>()
  g[SEGMENT_RENDER_CACHE_KEY] = cache
  return cache
}

function getHashCache(): Map<string, string> {
  const g = globalThis as Record<string, unknown>
  const existing = g[CHAT_HASH_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, string>
  }
  const cache = new Map<string, string>()
  g[CHAT_HASH_CACHE_KEY] = cache
  return cache
}

function getStreamingSegmentHtmlCache(): Map<string, StreamingSegmentHtmlCacheEntry> {
  const g = globalThis as Record<string, unknown>
  const existing = g[STREAMING_SEGMENT_HTML_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, StreamingSegmentHtmlCacheEntry>
  }
  const cache = new Map<string, StreamingSegmentHtmlCacheEntry>()
  g[STREAMING_SEGMENT_HTML_CACHE_KEY] = cache
  return cache
}

function getStreamingSegmentRenderIncrementalCache(): Map<
  string,
  StreamingSegmentRenderIncrementalCacheEntry
> {
  const g = globalThis as Record<string, unknown>
  const existing = g[STREAMING_SEGMENT_RENDER_INCREMENTAL_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, StreamingSegmentRenderIncrementalCacheEntry>
  }
  const cache = new Map<string, StreamingSegmentRenderIncrementalCacheEntry>()
  g[STREAMING_SEGMENT_RENDER_INCREMENTAL_CACHE_KEY] = cache
  return cache
}

function setStreamingSegmentRenderIncrementalCache(
  cache: Map<string, StreamingSegmentRenderIncrementalCacheEntry>,
  key: string,
  value: StreamingSegmentRenderIncrementalCacheEntry
) {
  if (cache.size >= STREAMING_SEGMENT_RENDER_INCREMENTAL_CACHE_MAX && !cache.has(key)) {
    evictOldestMapEntry(cache)
  }
  cache.set(key, value)
}

function clearStreamingSegmentRenderIncrementalState(key?: string) {
  const cache = getStreamingSegmentRenderIncrementalCache()
  if (!key) {
    cache.clear()
    return
  }
  cache.delete(key)
}

function getEffectiveSegmentsCache(): WeakMap<ParsedContent, EffectiveSegmentsCacheEntry> {
  const g = globalThis as Record<string, unknown>
  const existing = g[EFFECTIVE_SEGMENTS_CACHE_KEY]
  if (existing instanceof WeakMap) {
    return existing as WeakMap<ParsedContent, EffectiveSegmentsCacheEntry>
  }
  const cache = new WeakMap<ParsedContent, EffectiveSegmentsCacheEntry>()
  g[EFFECTIVE_SEGMENTS_CACHE_KEY] = cache
  return cache
}

function getEffectiveSegmentsCacheForParsed(parsed: ParsedContent): EffectiveSegmentsCacheEntry {
  const cache = getEffectiveSegmentsCache()
  const existing = cache.get(parsed)
  if (existing) return existing
  const next = new Map<string, ContentSegment[] | null>()
  cache.set(parsed, next)
  return next
}

function getEffectiveSegmentsIncrementalCache(): Map<
  string,
  EffectiveSegmentsIncrementalCacheEntry
> {
  const g = globalThis as Record<string, unknown>
  const existing = g[EFFECTIVE_SEGMENTS_INCREMENTAL_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, EffectiveSegmentsIncrementalCacheEntry>
  }
  const cache = new Map<string, EffectiveSegmentsIncrementalCacheEntry>()
  g[EFFECTIVE_SEGMENTS_INCREMENTAL_CACHE_KEY] = cache
  return cache
}

function setEffectiveSegmentsIncrementalCache(
  cache: Map<string, EffectiveSegmentsIncrementalCacheEntry>,
  key: string,
  parsedText: string,
  parsedCards: TypelessCard[],
  value: ContentSegment[] | null
) {
  if (cache.size >= EFFECTIVE_SEGMENTS_INCREMENTAL_CACHE_MAX && !cache.has(key)) {
    evictOldestMapEntry(cache)
  }
  cache.set(key, {
    parsedText,
    parsedCards,
    value,
  })
}

function setEffectiveSegmentsCacheForParsed(
  parsed: ParsedContent,
  cache: EffectiveSegmentsCacheEntry,
  key: string,
  value: ContentSegment[] | null
) {
  if (cache.size >= EFFECTIVE_SEGMENTS_CACHE_PER_PARSED_MAX && !cache.has(key)) {
    evictOldestMapEntry(cache)
  }
  cache.set(key, value)
  const weakCache = getEffectiveSegmentsCache()
  weakCache.set(parsed, cache)
}

function hashString(value: string): string {
  let hash = 0
  for (let i = 0; i < value.length; i++) {
    hash = ((hash << 5) - hash + value.charCodeAt(i)) | 0
  }
  return hash.toString(36)
}

function hashStringCached(value: string): string {
  const cache = getHashCache()
  const cached = cache.get(value)
  if (cached) return cached
  const hashed = hashString(value)
  if (cache.size >= CHAT_HASH_CACHE_MAX && !cache.has(value)) {
    evictOldestMapEntry(cache)
  }
  cache.set(value, hashed)
  return hashed
}

function setSegmentRenderCache(
  cache: Map<string, SegmentRenderState>,
  key: string,
  value: SegmentRenderState
) {
  if (cache.size >= SEGMENT_RENDER_CACHE_MAX && !cache.has(key)) {
    evictOldestMapEntry(cache)
  }
  cache.set(key, value)
}

function getSegmentRenderCacheEntryKey(
  conversationId: string,
  messageId: string,
  showToolDetails: boolean,
  content: string,
  interruptedHtml: string
): string {
  return `${conversationId}:${messageId}:${showToolDetails ? '1' : '0'}:${hashStringCached(content)}:${hashStringCached(interruptedHtml)}`
}

const segmentRenderState = computed(() => {
  const parsed = parsedContent.value
  if (!parsed || parsed.cards.length === 0) {
    return EMPTY_SEGMENT_RENDER_STATE
  }

  const isStreaming = Boolean(props.isStreaming)
  const useCache = !isStreaming
  const convId = props.message.conversation_id || 'unknown'
  const streamingRenderIncrementalKey = `${convId}:${props.message.id}`
  const streamingRenderIncrementalCache = isStreaming
    ? getStreamingSegmentRenderIncrementalCache()
    : null
  const previousStreamingRender = streamingRenderIncrementalCache
    ? streamingRenderIncrementalCache.get(streamingRenderIncrementalKey)
    : null
  let cache: Map<string, SegmentRenderState> | null = null
  let cacheKey = ''

  if (useCache) {
    cacheKey = getSegmentRenderCacheEntryKey(
      convId,
      props.message.id,
      settingsStore.showToolDetails,
      displayContentWithoutProcessBlocks.value,
      interruptedIndicatorHtml.value
    )
    cache = getSegmentRenderCache()
    const cached = cache.get(cacheKey)
    if (cached) {
      return cached
    }
  }

  const segments = buildEffectiveSegments(
    parsed,
    convId,
    props.message.id,
    settingsStore.showToolDetails,
    Boolean(props.isStreaming)
  )
  if (!segments) {
    if (cache && cacheKey) {
      setSegmentRenderCache(cache, cacheKey, EMPTY_SEGMENT_RENDER_STATE)
    }
    if (streamingRenderIncrementalCache) {
      clearStreamingSegmentRenderIncrementalState(streamingRenderIncrementalKey)
    }
    return EMPTY_SEGMENT_RENDER_STATE
  }

  const showToolDetails = settingsStore.showToolDetails
  const interruptedHtml = interruptedIndicatorHtml.value
  const streamingSegmentHtmlCache = isStreaming ? getStreamingSegmentHtmlCache() : null
  let rendered: RenderedContentSegment[] = []
  let cardCandidates: ContentSegment[] = []
  let hasCards = false
  let hasNonEmptyText = false
  let startIndex = 0

  if (
    previousStreamingRender &&
    previousStreamingRender.showToolDetails === showToolDetails &&
    previousStreamingRender.interruptedHtml === interruptedHtml
  ) {
    const previousSegments = previousStreamingRender.segments
    const previousRendered = previousStreamingRender.state.rendered
    if (previousSegments === segments && previousRendered) {
      return previousStreamingRender.state
    }
    if (previousRendered) {
      const maxPrefix = Math.min(previousSegments.length, segments.length)
      while (startIndex < maxPrefix && previousSegments[startIndex] === segments[startIndex]) {
        startIndex++
      }
      if (startIndex > 0) {
        rendered = previousRendered.slice(0, startIndex)
        for (const reused of rendered) {
          if (reused.type !== 'text') {
            const cardSegment = reused as ContentSegment
            hasCards = true
            if (!hasNonEmptyText) {
              cardCandidates.push(cardSegment)
            }
            continue
          }
          if (!hasNonEmptyText && hasNonWhitespace(reused.content as string)) {
            hasNonEmptyText = true
          }
        }
      }
    }
  }

  for (let index = startIndex; index < segments.length; index++) {
    const segment = segments[index]
    if (!segment) continue
    if (segment.type !== 'text') {
      const cardSegment = segment as ContentSegment
      hasCards = true
      if (!hasNonEmptyText) {
        cardCandidates.push(cardSegment)
      }
      rendered.push(cardSegment)
      continue
    }

    const text = segment.content as string
    if (!hasNonEmptyText && hasNonWhitespace(text)) {
      hasNonEmptyText = true
    }

    if (streamingSegmentHtmlCache) {
      const cached = streamingSegmentHtmlCache.get(segment.key)
      if (
        cached &&
        cached.sourceText === text &&
        cached.showToolDetails === showToolDetails &&
        cached.interruptedHtml === interruptedHtml
      ) {
        rendered.push({ ...segment, html: cached.html })
        continue
      }
    }

    const effective = showToolDetails ? text : stripProcessContent(text)
    const { content, interrupted } = stripInterruptedMarker(effective)
    let html = renderMarkdownForMessage(content, `chat-segment:${segment.key}`)
    if (interrupted) {
      html += interruptedHtml
    }

    if (streamingSegmentHtmlCache) {
      if (
        streamingSegmentHtmlCache.size >= STREAMING_SEGMENT_HTML_CACHE_MAX &&
        !streamingSegmentHtmlCache.has(segment.key)
      ) {
        evictOldestMapEntry(streamingSegmentHtmlCache)
      }
      streamingSegmentHtmlCache.set(segment.key, {
        sourceText: text,
        showToolDetails,
        interruptedHtml,
        html,
      })
    }

    rendered.push({ ...segment, html })
  }

  const isCardOnly =
    hasCards && !hasNonEmptyText && !shouldKeepAssistantBubbleForCardOnly(cardCandidates)
  const nextState: SegmentRenderState = {
    rendered,
    cardOnly: isCardOnly ? cardCandidates : null,
    hasCards,
    isCardOnly,
  }

  if (cache && cacheKey) {
    setSegmentRenderCache(cache, cacheKey, nextState)
  }
  if (streamingRenderIncrementalCache) {
    setStreamingSegmentRenderIncrementalCache(
      streamingRenderIncrementalCache,
      streamingRenderIncrementalKey,
      {
        segments,
        showToolDetails,
        interruptedHtml,
        state: nextState,
      }
    )
  }

  return nextState
})

const renderedContentSegments = computed(() => segmentRenderState.value.rendered)

const cardOnlySegments = computed(() => segmentRenderState.value.cardOnly)

const effectiveHasCards = computed(() => segmentRenderState.value.hasCards)

// Card-only: no text segments, only cards — skip assistant bubble wrapper
const isCardOnly = computed(() => segmentRenderState.value.isCardOnly)

type AssistantBlockItem =
  | { type: 'text'; key: string; html: string }
  | { type: 'card'; key: string; card: TypelessCard }

type AssistantRenderBlock = {
  key: string
  items: AssistantBlockItem[]
}

function buildTextOnlyAssistantBlocks(html: string): AssistantRenderBlock[] {
  if (!html) return []

  return [
    {
      key: `${props.message.id}-text-block-0`,
      items: [
        {
          type: 'text',
          key: `${props.message.id}-text-item-0`,
          html,
        },
      ],
    },
  ]
}

function getRenderedTextSegmentHtml(segment: RenderedContentSegment): string {
  if (segment.type !== 'text') return ''
  return (segment as RenderedContentSegment & { type: 'text'; html: string }).html
}

function buildSegmentedAssistantBlocks(segments: RenderedContentSegment[]): AssistantRenderBlock[] {
  const blocks: AssistantRenderBlock[] = []

  for (const segment of segments) {
    if (segment.type === 'text') {
      const html = getRenderedTextSegmentHtml(segment)
      if (!html) continue
      blocks.push({
        key: `${segment.key}-block`,
        items: [
          {
            type: 'text',
            key: `${segment.key}-text`,
            html,
          },
        ],
      })
      continue
    }

    blocks.push({
      key: `${segment.key}-card-block`,
      items: [
        {
          type: 'card',
          key: segment.key,
          card: segment.content as TypelessCard,
        },
      ],
    })
  }

  return blocks
}

const assistantRenderBlocks = computed<AssistantRenderBlock[]>(() => {
  if (!isAssistant.value || isContentEmpty.value || hasMediaTask.value) return []

  return (
    effectiveHasCards.value && renderedContentSegments.value
      ? buildSegmentedAssistantBlocks(renderedContentSegments.value)
      : buildTextOnlyAssistantBlocks(assistantTextState.value.html)
  )
})

const showProcessOnlyAssistantBubble = computed(
  () =>
    isAssistant.value &&
    !hasMediaTask.value &&
    assistantRenderBlocks.value.length === 0 &&
    (showProcessDetailsToggle.value ||
      showPersistedProcessPanel.value ||
      showAssistantStatusBar.value)
)

function getCardUiStateKey(card: TypelessCard, fallbackKey: string): string {
  return buildChatCardUiStateKey({
    conversationId: props.message.conversation_id,
    messageId: props.message.render_key || props.message.id,
    cardType: card.type,
    cardId: card.id,
    fallbackKey,
  })
}

function isTextAssistantBlock(block: AssistantRenderBlock): boolean {
  return block.items.length === 1 && block.items[0]?.type === 'text'
}

const formattedTime = computed(() => {
  const date = new Date(props.message.created_at)
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})

const formattedTimeLong = computed(() => {
  const date = new Date(props.message.created_at)
  return date.toLocaleString([], {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
})

// Get metadata from store or message itself
const metadata = computed(() => {
  // First check if message has inline metadata
  if (props.message.provider || props.message.model || props.message.stats) {
    return {
      provider: props.message.provider,
      model: props.message.model,
      stats: props.message.stats,
    }
  }
  // Fall back to store metadata
  return chatStore.getMessageMetadata(props.message.id)
})

// Get provider location (cloud/local) for icon display
const providerLocation = computed(() => {
  if (!metadata.value?.provider) return null
  const provider = providerPoolStore.providers.find(
    (p) => p.name === metadata.value?.provider || p.id === metadata.value?.provider
  )
  return provider?.location || null
})

function handleCopyClick(event: Event) {
  const target = event.target as HTMLElement
  if (target.classList.contains('copy-btn')) {
    const code = target.dataset.code
    if (code) {
      copyCodeToClipboard(code).then(() => {
        target.textContent = t('common.copied', 'Copied!')
        setTimeout(() => {
          target.textContent = t('common.copy', 'Copy')
        }, 2000)
      })
    }
  }
}

function formatTokens(num: number | undefined): string {
  if (num === undefined || num === null || num === 0) return ''
  return num.toLocaleString()
}

function formatResponseDuration(ms: number | undefined, fallbackMs?: number): string {
  const value = ms && ms > 0 ? ms : fallbackMs && fallbackMs > 0 ? fallbackMs : 0
  if (!value) return ''
  return (value / 1000).toFixed(2) + 's'
}

function formatSpeed(tps: number | undefined): string {
  if (tps === undefined || tps === null || tps === 0) return ''
  return tps.toFixed(1)
}

type MessageStatItem = {
  key: 'duration' | 'speed' | 'input' | 'output'
  value: string
  icon?: 'input' | 'output'
}

const messageStatItems = computed<MessageStatItem[]>(() => {
  if (isMobile.value || !isAssistant.value) return []
  const stats = metadata.value?.stats
  if (!stats) return []

  const items: MessageStatItem[] = []
  const duration = formatResponseDuration(stats.latency_ms, stats.ttft_ms)
  const speed = formatSpeed(stats.tokens_per_second)
  const inputTokens = formatTokens(stats.input_tokens)
  const outputTokens = formatTokens(stats.output_tokens)

  if (duration) items.push({ key: 'duration', value: duration })
  if (speed) items.push({ key: 'speed', value: `${speed} tokens/s` })
  if (inputTokens) items.push({ key: 'input', value: inputTokens, icon: 'input' })
  if (outputTokens) items.push({ key: 'output', value: outputTokens, icon: 'output' })

  return items
})

const showAssistantStatsBar = computed(() => messageStatItems.value.length > 0)

function handleContextMenu(event: MouseEvent) {
  // Don't trigger context menu during streaming
  if (props.isStreaming) return
  if (isEditingUserMessage.value) return
  // Don't trigger for temp messages
  if (props.message.id.startsWith('temp-') || props.message.id.startsWith('streaming-')) return

  event.preventDefault()
  emit('contextmenu', event, props.message.id)
}

function handleClick() {
  if (isMultiSelectMode.value) {
    chatStore.toggleMessageSelection(props.message.id)
  }
}

// Copy entire message content
async function handleCopyMessage() {
  try {
    await navigator.clipboard.writeText(props.message.content)
    copyState.value = 'copied'
    setTimeout(() => {
      copyState.value = 'idle'
    }, 2000)
  } catch (err) {
    console.error('Failed to copy message:', err)
  }
}

function openUserEdit() {
  if (!canEditUserMessage.value) return
  editedUserMessageContent.value = props.message.content
  isEditingUserMessage.value = true
  nextTick(() => {
    const textarea = userEditTextareaRef.value
    if (!textarea) return
    textarea.focus()
    textarea.setSelectionRange(textarea.value.length, textarea.value.length)
  })
}

function cancelUserEdit() {
  isEditingUserMessage.value = false
  editedUserMessageContent.value = props.message.content
}

function submitUserEdit() {
  if (!canSubmitEditedUserMessage.value) return
  const nextContent = editedUserMessageContent.value.trim()
  isEditingUserMessage.value = false
  emit('edit-resubmit', props.message.id, nextContent)
}

function handleUserEditKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault()
    submitUserEdit()
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    cancelUserEdit()
  }
}

// Export message as markdown file
function handleExportMessage() {
  const content = props.message.content
  if (!content) return
  const id = props.message.id?.slice(0, 8) || 'msg'
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `zimaos-blue-${id}.md`
  a.click()
  URL.revokeObjectURL(url)
}

// Clean up incremental parse state when component is unmounted
onUnmounted(() => {
  clearStreamingRevealRaf()
  stopAssistantStatusTimer()
  clearIncrementalState(props.message.render_key || props.message.id, props.message.conversation_id)
  clearSplitSegmentsIncrementalState(
    `${props.message.conversation_id || 'unknown'}:${props.message.id}`
  )
  clearStreamingSegmentRenderIncrementalState(
    `${props.message.conversation_id || 'unknown'}:${props.message.id}`
  )
  stopVoiceMessage()
  // Only stop manually-triggered TTS (not auto-play streaming which survives component remount)
  // When streaming ends, the message component remounts with a new server ID —
  // we must not kill the streaming TTS manager during that transition.
  if (isSpeaking.value && !hasAutoPlayed.value) {
    ttsAudioManager.stop()
  }
})

// Auto-play TTS - supports streaming mode (play while receiving)
const hasAutoPlayed = ref(false)
const lastPlayedLength = ref(0)

// Strip complex card content (code blocks, mermaid, tables) from text for TTS
function stripComplexCardsForTTS(text: string): string {
  return (
    text
      // Remove process blocks (<!-- process-start -->...<!-- process-end -->)
      .replace(/<!--\s*process-start\s*-->[\s\S]*?<!--\s*process-end\s*-->/g, '')
      // Remove fenced code blocks (```...```)
      .replace(/```[\s\S]*?```/g, '')
      // Remove HTML comments
      .replace(/<!--[\s\S]*?-->/g, '')
      // Remove markdown tables (lines starting with |)
      .replace(/^\|.*\|$/gm, '')
      // Remove table separator lines (|---|---|)
      .replace(/^\|[-:\s|]+\|$/gm, '')
      .replace(/\n{3,}/g, '\n\n')
      .trim()
  )
}

// Watch for content changes during streaming to play incrementally
watch(
  () => props.message.content,
  (newContent, _oldContent) => {
    if (!trackStreamingState.value) return

    const autoPlayEnabled = isAutoPlayEnabled()
    if (!autoPlayEnabled) return

    // Strip complex cards (code, mermaid, tables) before extracting sentences
    const ttsText = stripComplexCardsForTTS(newContent)

    // Find new complete sentences (backend humanizer will clean markdown)
    const sentences = ttsText.match(/[^.!?。！？]+[.!?。！？]+/g) || []
    const completeSentences = sentences.join('')

    // Play new sentences that haven't been played yet
    if (completeSentences.length > lastPlayedLength.value) {
      const newText = completeSentences.slice(lastPlayedLength.value)
      lastPlayedLength.value = completeSentences.length

      if (newText.trim()) {
        isSpeaking.value = true
        hasAutoPlayed.value = true
        // Don't await — let playback run in background so watcher can fire again
        streamingTTSManager.streamAndPlay(newText)
      }
    }
  }
)

// Play remaining text when streaming completes
watch(
  () => props.isStreaming,
  async (isStreaming, wasStreaming) => {
    if (wasStreaming && !isStreaming && isAssistant.value) {
      const autoPlayEnabled = isAutoPlayEnabled()
      if (!autoPlayEnabled) return

      const textContent = stripComplexCardsForTTS(props.message.content)

      if (!textContent) return

      // If we already played during streaming, only play remaining incomplete sentence
      if (hasAutoPlayed.value) {
        const sentences = textContent.match(/[^.!?。！？]+[.!?。！？]+/g) || []
        const completeSentences = sentences.join('')
        const remaining = textContent.slice(completeSentences.length).trim()

        // Set onComplete BEFORE streamAndPlay to avoid race where queue
        // drains synchronously before the callback is set
        streamingTTSManager.onComplete = () => {
          isSpeaking.value = false
          hasAutoPlayed.value = false
          lastPlayedLength.value = 0
          streamingTTSManager.onComplete = null
        }

        if (remaining) {
          // Append remaining text to the streaming queue (non-blocking)
          streamingTTSManager.streamAndPlay(remaining)
        }
      } else {
        // Play full text if nothing was played during streaming
        hasAutoPlayed.value = true
        isSpeaking.value = true
        await playTTSAudio(textContent)
        isSpeaking.value = false
        hasAutoPlayed.value = false
        lastPlayedLength.value = 0
      }
    }
  }
)

function isCardActionLoading(cardId?: string, actionId?: string): boolean {
  if (!cardId || cardActionLoading.value?.cardId !== cardId) return false
  return actionId ? cardActionLoading.value.actionId === actionId : true
}

function activeCardActionId(cardId?: string): string | undefined {
  if (!cardId || cardActionLoading.value?.cardId !== cardId) return undefined
  return cardActionLoading.value.actionId
}

function cardActionErrorMessage(cardId?: string): string | undefined {
  if (!cardId || cardActionError.value?.cardId !== cardId) return undefined
  return cardActionError.value.message
}

// Handle card action (button click)
async function handleCardAction(actionId: string, cardId?: string) {
  if (!cardId || isCardActionLoading(cardId)) return

  // Find the action label and structured metadata.
  let actionLabel: string | undefined
  let formData: Record<string, unknown> | undefined
  let cardType: string | undefined
  let cardTitle: string | undefined
  const card = parsedContent.value?.cards.find((c) => c.id === cardId)
  if (card) {
    cardType = card.type
    if ('title' in card && typeof (card as any).title === 'string') {
      cardTitle = (card as any).title
    }
    if ('actions' in card && Array.isArray((card as any).actions)) {
      const action = (card as any).actions.find((a: any) => a.id === actionId)
      actionLabel = action?.label
      if (
        action?.form_data &&
        typeof action.form_data === 'object' &&
        !Array.isArray(action.form_data)
      ) {
        formData = action.form_data as Record<string, unknown>
      }
    }
  }

  cardActionLoading.value = { cardId, actionId }
  cardActionError.value = null

  try {
    const res = await cardActionApi.submit(props.message.conversation_id, props.message.id, {
      card_id: cardId,
      action_id: actionId,
      action_label: actionLabel,
      card_type: cardType,
      card_title: cardTitle,
      form_data: formData,
    })
    // Emit event to parent for potential UI updates
    emit(
      'cardAction',
      props.message.conversation_id,
      props.message.id,
      cardId,
      actionId,
      actionLabel
    )
    // Auto-send the mapped message to trigger a new streaming turn
    if (res.data?.message) {
      await chatStore.sendMessage(res.data.message)
    }
  } catch (error) {
    console.error('Card action failed:', error)
    cardActionError.value = {
      cardId,
      message: error instanceof Error ? error.message : 'Action failed',
    }
  } finally {
    cardActionLoading.value = null
  }
}

// Handle card selection (choice card)
async function handleCardSelect(cardId: string, selectedIds: string[], otherText?: string) {
  if (!cardId || isCardActionLoading(cardId)) return

  // Find the choice card
  const choiceCard = parsedContent.value?.cards.find((c) => c.id === cardId) as
    | TypelessCardChoice
    | undefined
  if (!choiceCard) return

  // Build form data with selections
  const formData: Record<string, unknown> = {
    selected_ids: selectedIds,
  }
  if (otherText) {
    formData.other_text = otherText
  }

  // Get selected labels for the action label
  const selectedLabels = selectedIds
    .map((id) => choiceCard.options?.find((o) => o.id === id)?.label)
    .filter(Boolean)
    .join(', ')

  cardActionLoading.value = { cardId, actionId: 'select' }
  cardActionError.value = null

  try {
    await cardActionApi.submit(props.message.conversation_id, props.message.id, {
      card_id: cardId,
      action_id: 'select',
      action_label: selectedLabels || otherText || 'Selection',
      form_data: formData,
    })
    emit(
      'cardAction',
      props.message.conversation_id,
      props.message.id,
      cardId,
      'select',
      selectedLabels
    )
  } catch (error) {
    console.error('Card selection failed:', error)
    cardActionError.value = {
      cardId,
      message: error instanceof Error ? error.message : 'Selection failed',
    }
  } finally {
    cardActionLoading.value = null
  }
}

// TTS playback function
async function handlePlayTTS() {
  if (isSpeaking.value) {
    // Stop current playback (both manual and streaming)
    ttsAborted = true
    streamingTTSManager.stop()
    ttsAudioManager.stop()
    isSpeaking.value = false
    return
  }

  ttsError.value = null

  const textContent = props.message.content

  if (!textContent) {
    ttsError.value = t('chat.ttsNoContent')
    return
  }
  if (isTtsSpeechMuted()) {
    return
  }

  await playTTSAudio(textContent)
}

let ttsAborted = false

function isAutoPlayEnabled(): boolean {
  if (props.disableAutoTTS) return false
  if (isTtsSpeechMuted()) return false
  return isTtsAutoPlayEnabled()
}

watch(
  () => props.disableAutoTTS,
  (disabled) => {
    if (!disabled) return
    ttsAborted = true
    streamingTTSManager.stop()
    isSpeaking.value = false
    hasAutoPlayed.value = false
    lastPlayedLength.value = 0
  }
)

// Show init progress toast when Kokoro is still loading
async function showInitProgressToast(): Promise<void> {
  const notification = useNotificationStore()
  const provider = localStorage.getItem('tts-provider') || ''
  if (provider !== 'kokoro') return

  try {
    const res = await speechApi.getStatus()
    const stage = res.data.tts?.components?.kokoro?.init_stage
    if (!stage || stage === 'ready') return

    const stageKey = `speech.initStage.${stage}`
    const toastId = notification.info(
      t('speech.initProgress'),
      te(stageKey) ? t(stageKey) : stage,
      {
        duration: 0,
        dismissible: true,
        titleKey: 'speech.initProgress',
        messageKey: te(stageKey) ? stageKey : undefined,
      }
    )

    // Poll until ready
    const poll = setInterval(async () => {
      try {
        const r = await speechApi.getStatus()
        const s = r.data.tts?.components?.kokoro?.init_stage
        if (!s || s === 'ready' || s === 'error') {
          clearInterval(poll)
          notification.remove(toastId)
          if (s === 'ready') {
            notification.success(t('speech.initComplete'), undefined, {
              duration: 2000,
              titleKey: 'speech.initComplete',
            })
          }
        }
      } catch {
        clearInterval(poll)
        notification.remove(toastId)
      }
    }, 500)
  } catch {
    // Ignore — status endpoint may not be available
  }
}

// Stream text sentence-by-sentence for fast first-audio and overlapping fetch/playback
async function playTTSAudio(textContent: string) {
  if (isTtsSpeechMuted()) {
    ttsAborted = true
    isSpeaking.value = false
    return
  }
  isSpeaking.value = true
  ttsAborted = false

  // Show init progress toast if Kokoro is still loading
  showInitProgressToast()

  // Use streaming manager: splits into sentences, prefetches 2 ahead,
  // plays each as soon as ready — first audio arrives much faster than
  // waiting for the entire text to be synthesized.
  streamingTTSManager.onComplete = () => {
    if (!ttsAborted) {
      isSpeaking.value = false
    }
    streamingTTSManager.onComplete = null
  }

  try {
    // Reset queue for fresh manual playback, prefetch all sentences, then start playing
    streamingTTSManager.reset()
    await streamingTTSManager.streamText(textContent)
    streamingTTSManager.play()
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') return
    console.error('TTS error:', e)
    ttsError.value = t('chat.ttsError')
    isSpeaking.value = false
  }
}

// Open attachment preview modal
function openAttachmentPreview(attachment: {
  type: string
  name: string
  mime_type: string
  data: string
}) {
  if (attachment.type === 'image') {
    previewAttachment.value = {
      type: 'image',
      src: `data:${attachment.mime_type};base64,${attachment.data}`,
      name: attachment.name,
      mimeType: attachment.mime_type,
    }
  } else if (attachment.type === 'file') {
    if (isPdfMimeType(attachment.mime_type)) {
      previewAttachment.value = {
        type: 'pdf',
        src: `data:${attachment.mime_type};base64,${attachment.data}`,
        name: attachment.name,
        mimeType: attachment.mime_type,
      }
      return
    }

    // For text files, decode and show content
    if (isTextMimeType(attachment.mime_type)) {
      try {
        const content = decodeTextContent(attachment.data)
        previewAttachment.value = {
          type: isMarkdownAttachment(attachment.name, attachment.mime_type) ? 'markdown' : 'text',
          src: '',
          name: attachment.name,
          mimeType: attachment.mime_type,
          content: content,
        }
      } catch {
        previewAttachment.value = {
          type: 'file',
          src: '',
          name: attachment.name,
          mimeType: attachment.mime_type,
        }
      }
    } else {
      previewAttachment.value = {
        type: 'file',
        src: '',
        name: attachment.name,
        mimeType: attachment.mime_type,
      }
    }
  }
}

// Check if MIME type is text-based
function isTextMimeType(mimeType: string): boolean {
  const textTypes = [
    'text/plain',
    'text/html',
    'text/css',
    'text/javascript',
    'text/csv',
    'text/xml',
    'text/markdown',
    'application/json',
    'application/xml',
    'application/javascript',
    'application/x-javascript',
    'application/typescript',
    'application/x-yaml',
    'application/yaml',
  ]
  if (textTypes.includes(mimeType)) return true
  if (mimeType.startsWith('text/')) return true
  if (mimeType.includes('+xml') || mimeType.includes('+json')) return true
  return false
}

function isMarkdownAttachment(filename: string, mimeType: string): boolean {
  const normalizedMimeType = mimeType.toLowerCase()
  if (normalizedMimeType.includes('markdown')) return true

  const normalizedFilename = filename.toLowerCase()
  return (
    normalizedFilename.endsWith('.md') ||
    normalizedFilename.endsWith('.markdown') ||
    normalizedFilename.endsWith('.mdown') ||
    normalizedFilename.endsWith('.mkd') ||
    normalizedFilename.endsWith('.mdx')
  )
}

function isPdfMimeType(mimeType: string): boolean {
  return mimeType.toLowerCase() === 'application/pdf'
}

// Decode text content with encoding detection
function decodeTextContent(base64Data: string): string {
  try {
    // First try UTF-8 decoding
    const utf8Content = atob(base64Data)
    // Check if it looks like valid UTF-8 (no replacement characters after decode)
    const decoder = new TextDecoder('utf-8', { fatal: true })
    const bytes = Uint8Array.from(utf8Content, (c) => c.charCodeAt(0))
    return decoder.decode(bytes)
  } catch {
    // If UTF-8 fails, try GBK/GB2312 (common on Windows Chinese systems)
    try {
      const binaryString = atob(base64Data)
      const bytes = Uint8Array.from(binaryString, (c) => c.charCodeAt(0))
      const decoder = new TextDecoder('gbk')
      return decoder.decode(bytes)
    } catch {
      // Fallback to raw decode
      return atob(base64Data)
    }
  }
}

// Check if content is a placeholder pattern (should not be displayed)
// Matches: [CONTINUE], [filename.txt], [Attachments:...], etc.
function isPlaceholderContent(content: string): boolean {
  if (!content) return true
  const trimmed = content.trim()
  // Check if entire content is a single bracketed placeholder
  if (trimmed.startsWith('[') && trimmed.endsWith(']') && !trimmed.includes('\n')) {
    return true
  }
  // Check for specific patterns
  if (trimmed.startsWith('[Attachments:')) return true
  return false
}

// Format internal or compat tool names via the shared localization helper.
function formatToolName(name: string): string {
  return getLocalizedToolName(name, t, te)
}

// Get file icon based on extension
function getFileIcon(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() || ''
  const iconMap: Record<string, string> = {
    // Documents
    pdf: '📄',
    doc: '📝',
    docx: '📝',
    txt: '📄',
    md: '📝',
    rtf: '📝',
    // Spreadsheets
    xls: '📊',
    xlsx: '📊',
    csv: '📊',
    // Code
    js: '💻',
    ts: '💻',
    py: '🐍',
    java: '☕',
    go: '🔷',
    rs: '🦀',
    c: '💻',
    cpp: '💻',
    h: '💻',
    html: '🌐',
    css: '🎨',
    json: '📋',
    xml: '📋',
    yaml: '📋',
    yml: '📋',
    // Archives
    zip: '📦',
    rar: '📦',
    '7z': '📦',
    tar: '📦',
    gz: '📦',
    // Media
    mp3: '🎵',
    wav: '🎵',
    mp4: '🎬',
    avi: '🎬',
    mkv: '🎬',
    // Images (shouldn't reach here but just in case)
    png: '🖼️',
    jpg: '🖼️',
    jpeg: '🖼️',
    gif: '🖼️',
    svg: '🖼️',
    webp: '🖼️',
  }
  return iconMap[ext] || '📎'
}

// Close attachment preview modal
function closeAttachmentPreview() {
  previewAttachment.value = null
}

// Mobile touch handlers for long-press
function handleTouchStart(_event: TouchEvent) {
  // Start long-press timer
  longPressTimer.value = window.setTimeout(() => {
    showMobileActions.value = true
    // Haptic feedback if available
    if ('vibrate' in navigator) {
      navigator.vibrate(50)
    }
  }, longPressThreshold)
}

function handleTouchEnd() {
  // Cancel long-press timer if touch ends before threshold
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
    longPressTimer.value = null
  }
}

function handleTouchMove() {
  // Cancel long-press if user moves finger (scrolling)
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
    longPressTimer.value = null
  }
}

function closeMobileActions() {
  showMobileActions.value = false
}

function handleAttachmentPreviewClick(attachment: MessageAttachment, index: number) {
  if (attachment.type === 'audio') {
    playVoiceMessage(attachment, index)
    return
  }

  openAttachmentPreview(attachment)
}

async function handleMobileCopy() {
  await handleCopyMessage()
  closeMobileActions()
}

async function handleMobileTTS() {
  await handlePlayTTS()
  closeMobileActions()
}

function handleMobileEdit() {
  openUserEdit()
  closeMobileActions()
}

function handleMobileExport() {
  handleExportMessage()
  closeMobileActions()
}

function handleMobileSelect() {
  chatStore.enterMultiSelectMode(props.message.id)
  closeMobileActions()
}

function handleMobileContinue() {
  emit('continue')
  closeMobileActions()
}

function handleMobileRegenerate() {
  emit('regenerate')
  closeMobileActions()
}

async function handleMobileDelete() {
  chatStore.enterMultiSelectMode(props.message.id)
  try {
    await chatStore.deleteSelectedMessages()
  } catch {
    // Error handled in store
  }
  closeMobileActions()
}
</script>

<template>
  <div
    v-show="!shouldHideMessage"
    class="message group relative p-2 sm:px-3.5 sm:py-3 transition-colors duration-150"
    :class="{
      'bg-gray-100 dark:bg-gray-600/10': isSelected,
      'cursor-pointer': isMultiSelectMode,
    }"
    @contextmenu="handleContextMenu"
    @click="handleClick"
    @touchstart="handleTouchStart"
    @touchend="handleTouchEnd"
    @touchmove="handleTouchMove"
  >
    <!-- Selection checkbox in multi-select mode (absolute left) -->
    <div
      v-if="isMultiSelectMode"
      class="message-select-checkbox absolute top-1/2 -translate-y-1/2 flex items-center z-10"
    >
      <div
        class="w-5 h-5 rounded border-2 flex items-center justify-center transition-colors"
        :class="
          isSelected
            ? 'bg-gray-700 dark:bg-gray-500 border-gray-900 dark:border-gray-700'
            : 'border-gray-400 dark:border-gray-600'
        "
      >
        <svg
          v-if="isSelected"
          class="w-3 h-3 text-white"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="3"
            d="M5 13l4 4L19 7"
          />
        </svg>
      </div>
    </div>

    <!-- Message content wrapper -->
    <div
      class="flex gap-2 sm:gap-3.5"
      :class="{
        'justify-end': isUser,
        'message-content-multi-select-offset': isMultiSelectMode,
      }"
    >
      <div
        v-if="showLeadingAvatar"
        :class="[
          'avatar flex-shrink-0 w-6 h-6 sm:w-7 sm:h-7 rounded-full flex items-center justify-center text-xs sm:text-sm font-bold',
          currentAvatarVariantClass,
        ]"
      >
        <span class="sr-only">{{ currentAvatarLabel }}</span>
      </div>

      <!-- Content -->
      <div
        class="content min-w-0"
        :class="isUser ? 'max-w-[70%] sm:max-w-[64%]' : 'max-w-[86%] sm:max-w-[78%] lg:max-w-[72%]'"
      >
        <!-- Provider and Model info (above chat bubble for assistant) -->
        <div
          v-if="!isMobile && isAssistant && metadata && (metadata.provider || metadata.model)"
          class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400 mb-1"
        >
          <!-- Cloud/Local icon -->
          <svg
            v-if="providerLocation === 'cloud'"
            class="w-3 h-3"
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
          <svg
            v-else-if="providerLocation === 'local'"
            class="w-3 h-3"
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
          <!-- Provider name -->
          <span v-if="metadata.provider">{{
            providerPoolStore.getProviderDisplayName(metadata.provider)
          }}</span>
          <!-- Model name -->
          <span v-if="metadata.model" class="text-gray-400 dark:text-gray-500">/</span>
          <span v-if="metadata.model">{{ metadata.model }}</span>
        </div>

        <!-- User message bubble -->
        <div v-if="isUser" class="user-message-wrapper relative">
          <!-- Voice message bubble (WeChat/WhatsApp style) -->
          <div v-if="isVoiceMessage" class="voice-message-container">
            <div
              v-for="(attachment, index) in audioAttachments"
              :key="index"
              class="voice-bubble cursor-pointer select-none"
              :style="{ width: voiceBubbleWidth(attachment.duration) }"
              @click="playVoiceMessage(attachment, index)"
            >
              <div class="voice-bubble-inner">
                <!-- Play/Stop icon -->
                <div class="voice-play-btn">
                  <svg
                    v-if="playingAudioId === index"
                    class="w-5 h-5"
                    fill="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <rect x="6" y="4" width="4" height="16" rx="1" />
                    <rect x="14" y="4" width="4" height="16" rx="1" />
                  </svg>
                  <svg v-else class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M8 5v14l11-7z" />
                  </svg>
                </div>
                <!-- Waveform visualization -->
                <div class="voice-waveform">
                  <div
                    v-for="bar in 12"
                    :key="bar"
                    class="voice-bar"
                    :class="{
                      'voice-bar-active':
                        playingAudioId === index && (bar - 1) / 12 <= audioProgress,
                    }"
                    :style="{
                      height: `${[40, 65, 50, 80, 60, 90, 55, 75, 45, 85, 70, 50][bar - 1]}%`,
                    }"
                  />
                </div>
                <!-- Duration -->
                <span class="voice-duration">{{ formatVoiceDuration(attachment.duration) }}</span>
              </div>
            </div>
          </div>
          <!-- Normal message bubble -->
          <div v-else :class="userBubbleClasses">
            <!-- Attachments display inside bubble -->
            <div v-if="hasAttachments" class="mb-2 flex flex-wrap gap-2">
              <div
                v-for="(attachment, index) in message.attachments"
                :key="index"
                class="attachment-preview rounded-lg overflow-hidden border border-gray-300 dark:border-white/20 cursor-pointer hover:opacity-90 transition-opacity bg-white/90 dark:bg-white/10"
                @click="handleAttachmentPreviewClick(attachment, index)"
              >
                <!-- Image attachment -->
                <img
                  v-if="attachment.type === 'image'"
                  :src="`data:${attachment.mime_type};base64,${attachment.data}`"
                  :alt="attachment.name"
                  class="max-w-[200px] max-h-[150px] object-cover"
                  :title="attachment.name"
                />
                <!-- Audio attachment (inline mini player) -->
                <div
                  v-else-if="attachment.type === 'audio'"
                  class="flex items-center gap-2 px-3 py-2"
                >
                  <svg
                    v-if="playingAudioId === index"
                    class="w-4 h-4 text-green-500"
                    fill="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <rect x="6" y="4" width="4" height="16" rx="1" />
                    <rect x="14" y="4" width="4" height="16" rx="1" />
                  </svg>
                  <svg
                    v-else
                    class="w-4 h-4 text-gray-600 dark:text-gray-300"
                    fill="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path d="M8 5v14l11-7z" />
                  </svg>
                  <span class="text-sm text-gray-700 dark:text-white/90">{{
                    formatVoiceDuration(attachment.duration)
                  }}</span>
                </div>
                <!-- File attachment with icon -->
                <div v-else class="flex items-center gap-2 px-3 py-2">
                  <span class="text-lg">{{ getFileIcon(attachment.name) }}</span>
                  <span class="text-sm text-gray-700 dark:text-white/90 max-w-[150px] truncate">{{
                    attachment.name
                  }}</span>
                </div>
              </div>
            </div>
            <template v-if="isEditingUserMessage">
              <textarea
                ref="userEditTextareaRef"
                v-model="editedUserMessageContent"
                class="user-message-edit-input w-full min-h-[5.5rem] rounded-xl border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-gray-400 dark:border-white/15 dark:bg-white/8 dark:text-white"
                :placeholder="t('common.edit')"
                @keydown="handleUserEditKeydown"
              />
              <div class="user-message-edit-actions mt-3 flex items-center justify-end gap-2">
                <button
                  class="px-3 py-1.5 text-sm text-gray-600 transition hover:text-gray-800 dark:text-gray-300 dark:hover:text-white"
                  @click.stop="cancelUserEdit"
                >
                  {{ t('common.cancel') }}
                </button>
                <button
                  class="rounded-lg bg-gray-900 px-3 py-1.5 text-sm text-white transition hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
                  :disabled="!canSubmitEditedUserMessage"
                  @click.stop="submitUserEdit"
                >
                  {{ te('chat.saveAndResubmit') ? t('chat.saveAndResubmit') : t('common.save') }}
                </button>
              </div>
            </template>
            <template v-else>
              <!-- Inline images extracted from message content (e.g. media generation reference images) -->
              <div v-if="inlineImages.length > 0" class="mb-2 flex flex-wrap gap-2">
                <img
                  v-for="(img, idx) in inlineImages"
                  :key="idx"
                  :src="img.url"
                  :alt="img.alt"
                  class="rounded-lg max-w-[200px] max-h-[150px] object-cover border border-gray-300 dark:border-white/20"
                />
              </div>
              <!-- Text content (hide placeholder patterns like [filename.txt], [Attachments:...]) -->
              <span v-if="userTextContent && !isPlaceholderContent(userTextContent)">{{
                userTextContent
              }}</span>
              <!-- Show continue icon when content is [CONTINUE] and no attachments -->
              <span
                v-else-if="
                  message.content === '[CONTINUE]' &&
                  (!message.attachments || message.attachments.length === 0)
                "
                class="flex items-center gap-1 text-gray-500 dark:text-gray-400"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <span class="text-sm">{{ t('chat.continueGenerating') }}</span>
              </span>
            </template>
          </div>
          <div
            v-if="!isStreaming && !isMultiSelectMode && !isEditingUserMessage"
            class="user-actions"
          >
            <button
              v-if="canEditUserMessage"
              class="user-action-btn"
              :title="te('chat.editAndResubmit') ? t('chat.editAndResubmit') : t('common.edit')"
              @click.stop="openUserEdit"
            >
              <svg
                class="w-4 h-4 text-gray-500 dark:text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5M16.586 3.586a2 2 0 112.828 2.828L11 14.828 7 16l1.172-4L16.586 3.586z"
                />
              </svg>
            </button>
            <button
              class="copy-message-btn user-action-btn"
              :title="t('chat.copyMessage')"
              @click.stop="handleCopyMessage"
            >
              <svg
                v-if="copyState === 'idle'"
                class="w-4 h-4 text-gray-500 dark:text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                />
              </svg>
              <svg
                v-else
                class="w-4 h-4 text-green-500"
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

        <!-- Card-only assistant message: render cards directly without bubble wrapper -->
        <div v-else-if="isCardOnly && cardOnlySegments" class="assistant-message-wrapper relative">
          <TypelessCardComponent
            v-for="segment in cardOnlySegments"
            :key="segment.key"
            :card="segment.content as TypelessCard"
            :action-loading="isCardActionLoading((segment.content as TypelessCard).id)"
            :active-action-id="activeCardActionId((segment.content as TypelessCard).id)"
            :action-error="cardActionErrorMessage((segment.content as TypelessCard).id)"
            :ui-state-key="getCardUiStateKey(segment.content as TypelessCard, segment.key)"
            @action="handleCardAction"
            @select="handleCardSelect"
          />
        </div>

        <!-- Assistant message -->
        <div
          v-else
          :class="[
            'assistant-message-wrapper relative',
            { 'assistant-message-with-actions': !isMobile && !isMultiSelectMode },
          ]"
        >
          <!-- Action buttons for assistant message -->
          <div v-if="!isStreaming && !isMultiSelectMode" class="assistant-actions">
            <!-- Copy button -->
            <button
              class="copy-message-btn assistant-action-btn"
              :title="t('chat.copyMessage')"
              @click.stop="handleCopyMessage"
            >
              <svg
                v-if="copyState === 'idle'"
                class="w-4 h-4 text-gray-500 dark:text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                />
              </svg>
              <svg
                v-else
                class="w-4 h-4 text-green-500"
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
            <!-- TTS Play button -->
            <button
              class="tts-btn assistant-action-btn"
              :class="{ 'animate-pulse': isSpeaking }"
              :title="isSpeaking ? t('chat.stopTTS') : t('chat.playTTS')"
              @click.stop="handlePlayTTS"
            >
              <svg
                v-if="isSpeaking"
                class="w-4 h-4 text-gray-900 dark:text-gray-300"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z"
                />
              </svg>
              <svg
                v-else
                class="w-4 h-4 text-gray-500 dark:text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
                />
              </svg>
            </button>
            <!-- Export button -->
            <button
              class="export-btn assistant-action-btn"
              :title="t('chat.exportMessage')"
              @click.stop="handleExportMessage"
            >
              <svg
                class="w-4 h-4 text-gray-500 dark:text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
            </button>
          </div>

          <!-- Media task card (replaces normal assistant content) -->
          <div v-if="hasMediaTask" class="assistant-message assistant-message--media max-w-none">
            <MediaPlaceholder :task-id="mediaTaskId" />
          </div>

          <div v-else class="assistant-message-stack" @click="handleCopyClick">
            <div
              v-if="assistantRenderBlocks.length > 0 || showProcessOnlyAssistantBubble"
              class="assistant-message assistant-message-shell chat-copy-bubble chat-assistant-bubble max-w-none px-4 py-3"
            >
              <div
                v-for="block in assistantRenderBlocks"
                :key="block.key"
                :class="[
                  'assistant-message-block',
                  isTextAssistantBlock(block)
                    ? 'assistant-message-block--text prose prose-slate dark:prose-invert max-w-none'
                    : 'assistant-message-block--card',
                ]"
              >
                <template v-for="item in block.items" :key="item.key">
                  <div v-if="item.type === 'text'" class="prose-content" v-html="item.html" />
                  <TypelessCardComponent
                    v-else
                    :card="item.card"
                    :action-loading="isCardActionLoading(item.card.id)"
                    :active-action-id="activeCardActionId(item.card.id)"
                    :action-error="cardActionErrorMessage(item.card.id)"
                    :ui-state-key="getCardUiStateKey(item.card, item.key)"
                    class="my-1.5 -mx-1"
                    @action="handleCardAction"
                    @select="handleCardSelect"
                  />
                </template>
              </div>
              <div v-if="showProcessDetailsToggle" class="assistant-process-toggle-row">
                <button class="assistant-process-toggle" @click.stop="toggleProcessDetails">
                  {{
                    processDetailsExpanded ? t('chat.hideToolDetails') : t('chat.showToolDetails')
                  }}
                </button>
              </div>
              <div v-if="showPersistedProcessPanel" class="tool-detail-cards my-2 -mx-1">
                <ToolDetailCard
                  v-for="item in effectiveProcessToolResults"
                  :key="item.id"
                  :item="item"
                />
              </div>
              <div
                v-if="showStreamingProcessPanel && orderedStreamingProcessTrace.length > 0"
                class="assistant-process-trace-panel my-2"
              >
                <div
                  v-for="item in orderedStreamingProcessTrace"
                  :key="item.id"
                  class="assistant-process-trace-item"
                >
                  <span
                    class="assistant-process-trace-dot"
                    :class="getProcessTraceToneClass(item)"
                    aria-hidden="true"
                  />
                  <div class="assistant-process-trace-main">
                    <div class="assistant-process-trace-label">{{ item.label }}</div>
                    <div v-if="item.command" class="assistant-process-trace-command">
                      <span class="assistant-process-trace-command-prefix">$</span>
                      <span class="assistant-process-trace-command-text">{{ item.command }}</span>
                    </div>
                    <div v-if="item.detail" class="assistant-process-trace-detail">
                      {{ item.detail }}
                    </div>
                  </div>
                </div>
              </div>
              <div
                v-if="showStreamingProcessPanel && streamToolResults.length > 0"
                class="tool-detail-cards my-2 -mx-1"
              >
                <ToolDetailCard v-for="item in streamToolResults" :key="item.id" :item="item" />
              </div>
              <div
                v-if="showAssistantStatusBar"
                :class="['assistant-status-bar', { 'mt-0': showAssistantStatusOnly }]"
              >
                <div class="tool-pill assistant-status-pill" :class="assistantStatusVariantClass">
                  <span class="tool-dots"> <span /><span /><span /> </span>
                  <span class="tool-label assistant-status-label">{{ assistantStatusLabel }}</span>
                  <span
                    v-if="assistantStatusElapsedSeconds"
                    class="tool-timer assistant-status-timer tabular-nums"
                    >{{ assistantStatusElapsedSeconds }}s</span
                  >
                  <span
                    v-if="streamToolSandboxAvailable"
                    class="sandbox-badge"
                    :title="t('tools.sandboxProtected')"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="h-3.5 w-3.5"
                      viewBox="0 0 20 20"
                      fill="currentColor"
                    >
                      <path
                        fill-rule="evenodd"
                        d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </span>
                </div>
                <div
                  v-if="streamToolExecuting && toolDisplayNames.length > 0"
                  class="tool-names"
                >
                  <span v-for="name in toolDisplayNames" :key="name" class="tool-name-tag">{{
                    name
                  }}</span>
                </div>
              </div>
              <div v-if="showStreamingCaret" class="assistant-message-caret-row" aria-hidden="true">
                <span class="streaming-caret" />
              </div>
            </div>
          </div>
        </div>

        <!-- Timestamp -->
        <div
          class="timestamp text-xs text-gray-400 dark:text-gray-500 mt-1 flex items-center gap-x-3 gap-y-1 flex-wrap"
          :class="showAssistantStatsBar ? 'justify-between' : isUser ? 'justify-end' : ''"
        >
          <template v-if="showAssistantStatsBar">
            <div class="message-stats-inline">
              <span v-for="item in messageStatItems" :key="item.key" class="message-stat-chip">
                <svg
                  v-if="item.icon === 'input'"
                  class="message-stat-icon"
                  viewBox="0 0 12 12"
                  fill="currentColor"
                  aria-hidden="true"
                >
                  <path d="M6 2l3.5 4h-7L6 2z" />
                </svg>
                <svg
                  v-else-if="item.icon === 'output'"
                  class="message-stat-icon"
                  viewBox="0 0 12 12"
                  fill="currentColor"
                  aria-hidden="true"
                >
                  <path d="M2.5 6h7L6 10 2.5 6z" />
                </svg>
                <span>{{ item.value }}</span>
              </span>
            </div>
            <span class="message-meta-time">{{ formattedTime }}</span>
          </template>
          <template v-else>
            <span class="message-meta-time">{{ formattedTime }}</span>
          </template>
        </div>
      </div>

      <div
        v-if="showTrailingAvatar"
        :class="[
          'avatar flex-shrink-0 w-6 h-6 sm:w-7 sm:h-7 rounded-full flex items-center justify-center text-xs sm:text-sm font-bold',
          currentAvatarVariantClass,
        ]"
      >
        <span class="sr-only">{{ currentAvatarLabel }}</span>
      </div>
    </div>

    <!-- Attachment Preview Modal -->
    <Teleport to="body">
      <div
        v-if="previewAttachment"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
        @click.self="closeAttachmentPreview"
      >
        <div class="relative max-w-[90vw] max-h-[90vh]">
          <button
            class="attachment-preview-close absolute -top-10 text-white hover:text-gray-300 transition-colors"
            @click="closeAttachmentPreview"
          >
            <svg class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
          <!-- Image preview -->
          <div
            v-if="previewAttachment.type === 'image'"
            class="attachment-preview-shell attachment-preview-shell--image"
          >
            <div class="attachment-preview-header">
              <div class="attachment-preview-meta">
                <span class="attachment-preview-badge">{{ previewAttachmentBadge }}</span>
                <div class="attachment-preview-copy">
                  <span class="attachment-preview-title">{{ previewAttachment.name }}</span>
                  <span class="attachment-preview-kind">{{ previewAttachmentKindLabel }}</span>
                </div>
              </div>
            </div>
            <div class="attachment-preview-body attachment-preview-body--visual">
              <img
                :src="previewAttachment.src"
                :alt="previewAttachment.name"
                class="attachment-image-preview"
              />
            </div>
          </div>
          <!-- Text file preview -->
          <div
            v-else-if="previewAttachment.type === 'markdown'"
            class="attachment-preview-shell attachment-preview-shell--markdown"
          >
            <div class="attachment-preview-header">
              <div class="attachment-preview-meta">
                <span class="attachment-preview-badge">{{ previewAttachmentBadge }}</span>
                <div class="attachment-preview-copy">
                  <span class="attachment-preview-title">{{ previewAttachment.name }}</span>
                  <span class="attachment-preview-kind">{{ previewAttachmentKindLabel }}</span>
                </div>
              </div>
            </div>
            <div
              class="attachment-preview-body attachment-markdown-preview prose prose-slate dark:prose-invert prose-content max-w-none"
              v-html="previewAttachmentHtml"
            />
          </div>
          <!-- PDF preview -->
          <div
            v-else-if="previewAttachment.type === 'pdf'"
            class="attachment-preview-shell attachment-preview-shell--pdf"
          >
            <div class="attachment-preview-header">
              <div class="attachment-preview-meta">
                <span class="attachment-preview-badge">{{ previewAttachmentBadge }}</span>
                <div class="attachment-preview-copy">
                  <span class="attachment-preview-title">{{ previewAttachment.name }}</span>
                  <span class="attachment-preview-kind">{{ previewAttachmentKindLabel }}</span>
                </div>
              </div>
            </div>
            <div class="attachment-preview-body attachment-preview-body--pdf">
              <iframe
                :src="previewAttachment.src"
                :title="previewAttachment.name"
                class="attachment-pdf-frame"
              />
            </div>
          </div>
          <!-- Text file preview -->
          <div
            v-else-if="previewAttachment.type === 'text'"
            class="attachment-preview-shell attachment-preview-shell--text"
          >
            <div class="attachment-preview-header">
              <div class="attachment-preview-meta">
                <span class="attachment-preview-badge">{{ previewAttachmentBadge }}</span>
                <div class="attachment-preview-copy">
                  <span class="attachment-preview-title">{{ previewAttachment.name }}</span>
                  <span class="attachment-preview-kind">{{ previewAttachmentKindLabel }}</span>
                </div>
              </div>
            </div>
            <div class="attachment-preview-body">
              <pre class="attachment-text-preview">{{ previewAttachment.content }}</pre>
            </div>
          </div>
          <!-- Generic file preview -->
          <div v-else class="attachment-preview-shell attachment-preview-shell--file">
            <div class="attachment-preview-header">
              <div class="attachment-preview-meta">
                <span class="attachment-preview-badge">{{ previewAttachmentBadge }}</span>
                <div class="attachment-preview-copy">
                  <span class="attachment-preview-title">{{ previewAttachment.name }}</span>
                  <span class="attachment-preview-kind">{{ previewAttachmentKindLabel }}</span>
                </div>
              </div>
            </div>
            <div class="attachment-preview-body attachment-preview-body--empty">
              <div class="attachment-file-placeholder">
                <svg
                  class="w-16 h-16 text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                  />
                </svg>
                <span class="attachment-file-placeholder-text">{{
                  t('chat.filePreviewNotSupported')
                }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Mobile Action Menu (Bottom Sheet) -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="showMobileActions"
          class="fixed inset-0 z-50 flex items-end justify-center bg-black/50 md:hidden"
          @click="closeMobileActions"
        >
          <div
            class="w-full bg-white dark:bg-gray-700 rounded-t-2xl shadow-xl transform transition-transform"
            @click.stop
          >
            <!-- Handle bar -->
            <div class="flex justify-center pt-3 pb-2">
              <div class="w-12 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
            </div>

            <!-- Actions -->
            <div class="px-4 pb-6">
              <!-- Message info card -->
              <div
                class="px-4 py-3 mb-2 bg-gray-50 dark:bg-gray-800/50 rounded-xl text-xs text-gray-500 dark:text-gray-400 space-y-2"
              >
                <div class="flex items-center justify-between">
                  <span class="text-gray-400 dark:text-gray-500">{{ formattedTimeLong }}</span>
                </div>
              </div>

              <!-- Copy action -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileCopy"
              >
                <svg
                  class="w-5 h-5 text-gray-600 dark:text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                  />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{
                  t('chat.copyMessage')
                }}</span>
              </button>

              <button
                v-if="canEditUserMessage"
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileEdit"
              >
                <svg
                  class="w-5 h-5 text-gray-600 dark:text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5M16.586 3.586a2 2 0 112.828 2.828L11 14.828 7 16l1.172-4L16.586 3.586z"
                  />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{
                  te('chat.editAndResubmit') ? t('chat.editAndResubmit') : t('common.edit')
                }}</span>
              </button>

              <!-- TTS action (only for assistant messages) -->
              <button
                v-if="isAssistant"
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileTTS"
              >
                <svg
                  class="w-5 h-5 text-gray-600 dark:text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
                  />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{
                  isSpeaking ? t('chat.stopTTS') : t('chat.playTTS')
                }}</span>
              </button>

              <!-- Continue / Regenerate (only for last assistant message, not while streaming) -->
              <template v-if="isLastAssistantMessage && !isStreaming">
                <button
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                  @click="handleMobileContinue"
                >
                  <svg
                    class="w-5 h-5 text-gray-600 dark:text-gray-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
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
                  <span class="text-base font-medium text-gray-900 dark:text-white">{{
                    t('chat.continueGenerating')
                  }}</span>
                </button>
                <button
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                  @click="handleMobileRegenerate"
                >
                  <svg
                    class="w-5 h-5 text-gray-600 dark:text-gray-400"
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
                  <span class="text-base font-medium text-gray-900 dark:text-white">{{
                    t('chat.regenerate')
                  }}</span>
                </button>
              </template>

              <!-- Export action -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileExport"
              >
                <svg
                  class="w-5 h-5 text-gray-600 dark:text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                  />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{
                  t('chat.exportMessage')
                }}</span>
              </button>

              <!-- Select action (enter multi-select mode) -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileSelect"
              >
                <svg
                  class="w-5 h-5 text-gray-600 dark:text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
                  />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{
                  t('chat.selectMessage')
                }}</span>
              </button>

              <!-- Delete action -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                @click="handleMobileDelete"
              >
                <svg
                  class="w-5 h-5 text-red-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
                <span class="text-base font-medium text-red-500">{{ t('common.delete') }}</span>
              </button>

              <!-- Cancel button -->
              <button
                class="w-full mt-2 px-4 py-3 rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
                @click="closeMobileActions"
              >
                <span class="text-base font-medium text-gray-900 dark:text-white">{{
                  t('common.cancel')
                }}</span>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.chat-copy-bubble {
  font-family:
    'SF Pro Text', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Noto Sans SC',
    var(--font-sans);
  font-size: 0.94rem;
  line-height: 1.64;
  font-weight: 400;
  word-break: break-word;
}

.assistant-message {
  background: transparent;
  color: rgb(30, 41, 59);
  padding: 0.05rem 0;
  box-shadow: none;
}

.assistant-message--media {
  padding: 0;
}

.assistant-message-stack {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.assistant-message-shell {
  display: flex;
  flex-direction: column;
  gap: 0.82rem;
}

.assistant-message-block {
  position: relative;
  padding: 0;
}

.assistant-message-block--card {
  min-width: 0;
}

.assistant-message-caret-row {
  display: flex;
  align-items: center;
  min-height: 1rem;
  margin-top: -0.22rem;
}

.assistant-message.assistant-message-indicator-only {
  padding-top: 0;
  padding-bottom: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.chat-user-bubble {
  background: linear-gradient(180deg, rgba(219, 234, 254, 0.92), rgba(191, 219, 254, 0.92));
  color: rgb(30, 41, 59);
  border: 1px solid rgba(147, 197, 253, 0.66);
  border-radius: 0.84rem 0.84rem 0.3rem 0.84rem;
  box-shadow: 0 8px 16px -24px rgba(37, 99, 235, 0.72);
  padding: 0.7rem 0.95rem;
}

.chat-assistant-bubble {
  background: var(--chat-assistant-bg, rgba(255, 255, 255, 0.96));
  border: var(--chat-assistant-border, 1px solid rgba(148, 163, 184, 0.24));
  border-radius: 0.84rem 0.84rem 0.84rem 0.3rem;
  box-shadow: 0 8px 16px -24px rgba(15, 23, 42, 0.28);
  padding: 0.8rem 1rem;
}

.streaming-caret {
  display: inline-flex;
  width: 0.58rem;
  height: 1.15rem;
  margin-inline-start: 0.14rem;
  vertical-align: text-bottom;
  border-radius: 999px;
  background: rgba(14, 165, 233, 0.85);
  animation: streaming-caret-blink 1s steps(1, end) infinite;
}

.avatar {
  border: 1px solid rgba(148, 163, 184, 0.24);
  box-shadow: none;
}

.avatar-assistant {
  background: linear-gradient(180deg, #e5e7eb, #d1d5db);
}

.avatar-user {
  background: linear-gradient(180deg, #3b82f6, #2563eb);
}

:root.dark .assistant-message,
[data-theme='dark'] .assistant-message {
  background: transparent;
  color: rgb(226, 232, 240);
  box-shadow: none;
}

:root.dark .chat-assistant-bubble,
[data-theme='dark'] .chat-assistant-bubble {
  box-shadow: 0 10px 24px -26px rgba(2, 6, 23, 0.78);
}

:root.dark .streaming-caret,
[data-theme='dark'] .streaming-caret {
  background: rgba(125, 211, 252, 0.96);
}

:root.dark .chat-user-bubble,
[data-theme='dark'] .chat-user-bubble {
  background: linear-gradient(160deg, rgba(37, 99, 235, 0.92), rgba(30, 64, 175, 0.94));
  color: rgb(239, 246, 255);
  border-color: rgba(96, 165, 250, 0.46);
  box-shadow: 0 9px 20px -22px rgba(37, 99, 235, 0.82);
}

:root.dark .avatar,
[data-theme='dark'] .avatar {
  border-color: rgba(71, 85, 105, 0.52);
}

:root.dark .avatar-assistant,
[data-theme='dark'] .avatar-assistant {
  background: linear-gradient(180deg, #475569, #334155);
}

/* Fade transition for mobile action menu */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.fade-enter-active > div,
.fade-leave-active > div {
  transition: transform 0.3s ease;
}

.fade-enter-from > div {
  transform: translateY(100%);
}

.fade-leave-to > div {
  transform: translateY(100%);
}

@keyframes streaming-caret-blink {
  0%,
  49% {
    opacity: 1;
  }
  50%,
  100% {
    opacity: 0.18;
  }
}

.message {
  max-width: 64rem;
  margin: 0 auto;
}

.prose :deep(pre) {
  margin: 0;
  padding: 0;
  background: transparent;
}

/* Tree structure whitespace preservation */
.prose :deep(.tree-structure) {
  white-space: pre !important;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  background: rgba(0, 0, 0, 0.05);
  border-radius: 0.375rem;
  padding: 0.75rem;
  margin: 0.5rem 0;
  overflow-x: auto;
}

:root.dark .prose :deep(.tree-structure),
[data-theme='dark'] .prose :deep(.tree-structure) {
  background: rgba(255, 255, 255, 0.05);
}

.prose :deep(.code-block) {
  margin: 0.75rem 0;
}

.prose :deep(.inline-code) {
  background: rgba(0, 0, 0, 0.1);
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  font-size: 0.875em;
}

:root.dark .prose :deep(.inline-code),
[data-theme='dark'] .prose :deep(.inline-code) {
  background: rgba(255, 255, 255, 0.1);
}

.prose :deep(a) {
  color: #0284c7;
}

:root.dark .prose :deep(a),
[data-theme='dark'] .prose :deep(a) {
  color: #38bdf8;
}

.prose :deep(a:hover) {
  text-decoration: underline;
}

.prose :deep(ul),
.prose :deep(ol) {
  padding-inline-start: 1.5rem;
}

.assistant-message :deep(p) {
  line-height: 1.64;
  margin-block: 0.38rem;
}

.prose-content :deep(ul),
.prose-content :deep(ol) {
  margin-block: 0.46rem;
  padding-block: 0.48rem;
  padding-inline-start: 1.24rem;
  padding-inline-end: 0.78rem;
  border: 1px solid rgba(203, 213, 225, 0.64);
  border-radius: 0.68rem;
  background: rgba(248, 250, 252, 0.62);
}

:root.dark .prose-content :deep(ul),
:root.dark .prose-content :deep(ol),
[data-theme='dark'] .prose-content :deep(ul),
[data-theme='dark'] .prose-content :deep(ol) {
  border-color: rgba(71, 85, 105, 0.72);
  background: rgba(15, 23, 42, 0.58);
}

.prose :deep(blockquote) {
  border-inline-start-width: 3px;
  border-inline-start-color: rgba(148, 163, 184, 0.7);
  padding-inline-start: 0.9rem;
  color: #64748b;
}

:root.dark .prose :deep(blockquote),
[data-theme='dark'] .prose :deep(blockquote) {
  color: #9ca3af;
}

/* Embedded card styles */
.prose-content :deep(p:first-child) {
  margin-top: 0;
}

.prose-content :deep(p:last-child) {
  margin-bottom: 0;
}

.attachment-preview-shell {
  display: flex;
  flex-direction: column;
  width: min(80vw, 56rem);
  max-width: 80vw;
  max-height: 80vh;
  border: 1px solid rgba(203, 213, 225, 0.92);
  border-radius: 1rem;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.995), rgba(246, 250, 253, 0.985)),
    rgba(255, 255, 255, 0.98);
  color: rgb(15, 23, 42);
  box-shadow: 0 24px 48px -32px rgba(15, 23, 42, 0.6);
  overflow: hidden;
}

.attachment-preview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.9rem 1rem 0.82rem;
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.88), rgba(241, 245, 249, 0.88)),
    rgba(248, 250, 252, 0.92);
  backdrop-filter: blur(10px);
}

.attachment-preview-meta {
  display: flex;
  align-items: center;
  gap: 0.72rem;
  min-width: 0;
}

.attachment-preview-copy {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.attachment-preview-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 3.1rem;
  padding: 0.28rem 0.52rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.5);
  background: rgba(226, 232, 240, 0.7);
  color: rgb(51, 65, 85);
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.attachment-preview-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: inherit;
  font-size: 0.92rem;
  font-weight: 600;
}

.attachment-preview-kind {
  color: rgba(71, 85, 105, 0.9);
  font-size: 0.76rem;
  line-height: 1.25;
}

.attachment-preview-body {
  flex: 1;
  overflow: auto;
  padding: 1rem 1.05rem 1.15rem;
}

.attachment-preview-body--visual,
.attachment-preview-body--pdf,
.attachment-preview-body--empty {
  display: flex;
  align-items: center;
  justify-content: center;
}

.attachment-preview-body--visual {
  padding: 1rem;
  background:
    radial-gradient(circle at top, rgba(226, 232, 240, 0.72), rgba(241, 245, 249, 0.22) 48%),
    rgba(248, 250, 252, 0.6);
}

.attachment-preview-body--pdf {
  padding: 0.8rem;
  background: rgba(241, 245, 249, 0.72);
}

.attachment-preview-body--empty {
  padding: 1.25rem;
}

.attachment-image-preview {
  max-width: 100%;
  max-height: 68vh;
  object-fit: contain;
  border-radius: 0.88rem;
  box-shadow: 0 18px 36px -26px rgba(15, 23, 42, 0.45);
}

.attachment-pdf-frame {
  width: min(78vw, 54rem);
  min-height: 70vh;
  border: 1px solid rgba(203, 213, 225, 0.92);
  border-radius: 0.85rem;
  background: white;
}

.attachment-file-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.9rem;
  width: 100%;
  padding: 2.3rem 1.4rem;
  border: 1px dashed rgba(148, 163, 184, 0.56);
  border-radius: 1rem;
  background: rgba(248, 250, 252, 0.82);
}

.attachment-file-placeholder-text {
  color: rgb(71, 85, 105);
  font-size: 0.9rem;
}

.attachment-preview-shell--text {
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.995), rgba(248, 250, 252, 0.985)),
    rgba(255, 255, 255, 0.98);
}

.attachment-markdown-preview {
  color: inherit;
  font-size: 0.95rem;
  line-height: 1.74;
}

.attachment-markdown-preview :deep(h1),
.attachment-markdown-preview :deep(h2),
.attachment-markdown-preview :deep(h3),
.attachment-markdown-preview :deep(h4),
.attachment-markdown-preview :deep(h5),
.attachment-markdown-preview :deep(h6) {
  color: inherit;
  line-height: 1.25;
  letter-spacing: -0.015em;
}

.attachment-markdown-preview :deep(h1) {
  margin-top: 0;
  margin-bottom: 0.9rem;
  padding-bottom: 0.6rem;
  border-bottom: 1px solid rgba(226, 232, 240, 0.92);
  font-size: 1.48rem;
}

.attachment-markdown-preview :deep(h2) {
  margin-top: 1.35rem;
  margin-bottom: 0.72rem;
  font-size: 1.22rem;
}

.attachment-markdown-preview :deep(h3) {
  margin-top: 1.15rem;
  margin-bottom: 0.56rem;
  font-size: 1.04rem;
}

.attachment-markdown-preview :deep(hr) {
  margin: 1rem 0;
  border-color: rgba(203, 213, 225, 0.92);
}

.attachment-markdown-preview :deep(a) {
  color: #0369a1;
  font-weight: 500;
}

.attachment-markdown-preview :deep(table) {
  border-radius: 0.9rem;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.74);
}

.attachment-markdown-preview :deep(th) {
  color: rgb(15, 23, 42);
  background: rgba(241, 245, 249, 0.92);
}

.attachment-markdown-preview :deep(td) {
  color: rgba(30, 41, 59, 0.96);
}

.attachment-markdown-preview :deep(blockquote) {
  margin: 0.92rem 0;
  border-left-width: 4px;
  border-left-color: rgba(14, 116, 144, 0.34);
  background: rgba(240, 249, 255, 0.75);
  border-radius: 0 0.8rem 0.8rem 0;
  padding: 0.72rem 0.9rem;
  color: rgb(71, 85, 105);
}

.attachment-markdown-preview :deep(.inline-code) {
  background: rgba(226, 232, 240, 0.72);
  color: rgb(15, 23, 42);
}

.attachment-markdown-preview :deep(.code-block),
.attachment-markdown-preview :deep(.tree-structure) {
  border: 1px solid rgba(203, 213, 225, 0.92);
  box-shadow: 0 12px 24px -28px rgba(15, 23, 42, 0.35);
}

.attachment-markdown-preview :deep(.code-block) {
  margin: 0.9rem 0;
}

.attachment-markdown-preview :deep(.code-header) {
  background: rgba(30, 41, 59, 0.96);
}

.attachment-markdown-preview :deep(.copy-btn) {
  opacity: 0.86;
}

.attachment-markdown-preview :deep(.copy-btn:hover) {
  opacity: 1;
}

.attachment-text-preview {
  margin: 0;
  color: inherit;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.875rem;
  line-height: 1.68;
}

:root.dark .attachment-preview-shell,
[data-theme='dark'] .attachment-preview-shell {
  border-color: rgba(71, 85, 105, 0.92);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.985), rgba(11, 18, 32, 0.975)),
    rgba(15, 23, 42, 0.96);
  color: rgb(226, 232, 240);
  box-shadow: 0 24px 52px -28px rgba(2, 6, 23, 0.9);
}

:root.dark .attachment-preview-header,
[data-theme='dark'] .attachment-preview-header {
  border-bottom-color: rgba(51, 65, 85, 0.92);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.94), rgba(19, 33, 54, 0.9)), rgba(15, 23, 42, 0.92);
}

:root.dark .attachment-preview-badge,
[data-theme='dark'] .attachment-preview-badge {
  border-color: rgba(71, 85, 105, 0.94);
  background: rgba(30, 41, 59, 0.9);
  color: rgb(191, 219, 254);
}

:root.dark .attachment-preview-kind,
[data-theme='dark'] .attachment-preview-kind {
  color: rgba(148, 163, 184, 0.9);
}

:root.dark .attachment-preview-body--visual,
[data-theme='dark'] .attachment-preview-body--visual {
  background:
    radial-gradient(circle at top, rgba(30, 41, 59, 0.76), rgba(15, 23, 42, 0.28) 48%),
    rgba(2, 6, 23, 0.22);
}

:root.dark .attachment-preview-body--pdf,
[data-theme='dark'] .attachment-preview-body--pdf {
  background: rgba(2, 6, 23, 0.38);
}

:root.dark .attachment-pdf-frame,
[data-theme='dark'] .attachment-pdf-frame {
  border-color: rgba(71, 85, 105, 0.84);
}

:root.dark .attachment-file-placeholder,
[data-theme='dark'] .attachment-file-placeholder {
  border-color: rgba(71, 85, 105, 0.72);
  background: rgba(15, 23, 42, 0.54);
}

:root.dark .attachment-file-placeholder-text,
[data-theme='dark'] .attachment-file-placeholder-text {
  color: rgb(148, 163, 184);
}

:root.dark .attachment-markdown-preview :deep(h1),
[data-theme='dark'] .attachment-markdown-preview :deep(h1) {
  border-bottom-color: rgba(51, 65, 85, 0.92);
}

:root.dark .attachment-markdown-preview :deep(hr),
[data-theme='dark'] .attachment-markdown-preview :deep(hr) {
  border-color: rgba(71, 85, 105, 0.84);
}

:root.dark .attachment-markdown-preview :deep(a),
[data-theme='dark'] .attachment-markdown-preview :deep(a) {
  color: #7dd3fc;
}

:root.dark .attachment-markdown-preview :deep(table),
[data-theme='dark'] .attachment-markdown-preview :deep(table) {
  background: rgba(15, 23, 42, 0.56);
}

:root.dark .attachment-markdown-preview :deep(th),
[data-theme='dark'] .attachment-markdown-preview :deep(th) {
  color: rgb(226, 232, 240);
  background: rgba(30, 41, 59, 0.88);
}

:root.dark .attachment-markdown-preview :deep(td),
[data-theme='dark'] .attachment-markdown-preview :deep(td) {
  color: rgba(226, 232, 240, 0.96);
}

:root.dark .attachment-markdown-preview :deep(blockquote),
[data-theme='dark'] .attachment-markdown-preview :deep(blockquote) {
  border-left-color: rgba(56, 189, 248, 0.38);
  background: rgba(12, 74, 110, 0.18);
  color: rgb(148, 163, 184);
}

:root.dark .attachment-markdown-preview :deep(.inline-code),
[data-theme='dark'] .attachment-markdown-preview :deep(.inline-code) {
  background: rgba(51, 65, 85, 0.78);
  color: rgb(226, 232, 240);
}

:root.dark .attachment-markdown-preview :deep(.code-block),
:root.dark .attachment-markdown-preview :deep(.tree-structure),
[data-theme='dark'] .attachment-markdown-preview :deep(.code-block),
[data-theme='dark'] .attachment-markdown-preview :deep(.tree-structure) {
  border-color: rgba(71, 85, 105, 0.84);
}

/* Copy button styles */
.copy-message-btn {
  background: transparent;
}

.message-select-checkbox {
  inset-inline-start: 0.5rem;
}

.message-content-multi-select-offset {
  padding-inline-start: 2rem;
}

.attachment-preview-close {
  inset-inline-end: 0;
}

.assistant-message-with-actions {
  padding-inline-end: 2.4rem;
}

.assistant-actions {
  position: absolute;
  top: 0.1rem;
  inset-inline-end: 0;
  display: flex;
  flex-direction: column;
  gap: 0.16rem;
  padding: 0.22rem;
  border-radius: 0.62rem;
  border: 1px solid rgba(203, 213, 225, 0.58);
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(6px);
  box-shadow: 0 4px 10px rgba(15, 23, 42, 0.06);
  opacity: 0;
  transform: translateX(2px);
  transition:
    opacity 0.16s ease,
    transform 0.16s ease;
}

.message:hover .assistant-actions,
.message:focus-within .assistant-actions {
  opacity: 1;
  transform: translateX(0);
}

.assistant-action-btn {
  width: 1.62rem;
  height: 1.62rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.46rem;
  background: transparent;
  transition: background-color 0.12s ease;
}

.assistant-action-btn:hover {
  background: rgba(148, 163, 184, 0.18);
}

:root.dark .user-action-btn:hover,
[data-theme='dark'] .user-action-btn:hover {
  background: rgba(100, 116, 139, 0.35);
}

:root.dark .assistant-actions,
[data-theme='dark'] .assistant-actions {
  border-color: rgba(71, 85, 105, 0.66);
  background: rgba(30, 41, 59, 0.72);
  box-shadow: 0 6px 12px rgba(2, 6, 23, 0.28);
}

:root.dark .assistant-action-btn:hover,
[data-theme='dark'] .assistant-action-btn:hover {
  background: rgba(100, 116, 139, 0.35);
}

:global(html[dir='rtl']) .assistant-actions {
  transform: translateX(-2px);
}

@media (max-width: 639px) {
  .assistant-message-with-actions {
    padding-inline-end: 0;
  }
}

/* Metadata tooltip styles */
.metadata-tooltip {
  pointer-events: none;
}

.message-stats-inline {
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.25rem 1rem;
  font-variant-numeric: tabular-nums;
}

.message-stat-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  min-width: 0;
}

.message-stat-icon {
  width: 0.75rem;
  height: 0.75rem;
  flex-shrink: 0;
  opacity: 0.72;
}

.message-meta-time {
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
}

/* Fade transition */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* User message wrapper for positioning copy button */
.user-message-wrapper {
  display: inline-block;
}

.user-actions {
  position: absolute;
  top: 50%;
  inset-inline-start: -0.7rem;
  display: flex;
  flex-direction: column;
  gap: 0.16rem;
  padding: 0.22rem;
  border-radius: 0.62rem;
  border: 1px solid rgba(203, 213, 225, 0.58);
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(6px);
  box-shadow: 0 4px 10px rgba(15, 23, 42, 0.06);
  opacity: 0;
  transform: translate(-100%, -50%);
  transition:
    opacity 0.16s ease,
    transform 0.16s ease;
}

:global(html[dir='rtl']) .user-actions {
  transform: translate(100%, -50%);
}

.message:hover .user-actions,
.message:focus-within .user-actions {
  opacity: 1;
  transform: translate(calc(-100% - 0.18rem), -50%);
}

:global(html[dir='rtl']) .message:hover .user-actions,
:global(html[dir='rtl']) .message:focus-within .user-actions {
  transform: translate(calc(100% + 0.18rem), -50%);
}

.user-action-btn {
  width: 1.62rem;
  height: 1.62rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.46rem;
  background: transparent;
  transition: background-color 0.12s ease;
}

.user-action-btn:hover {
  background: rgba(148, 163, 184, 0.18);
}

:root.dark .user-actions,
[data-theme='dark'] .user-actions {
  border-color: rgba(71, 85, 105, 0.66);
  background: rgba(30, 41, 59, 0.72);
  box-shadow: 0 6px 12px rgba(2, 6, 23, 0.28);
}

/* Response interrupted indicator */
.assistant-message :deep(.response-interrupted-indicator) {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  margin-top: 0.75rem;
  padding: 0.25rem 0.625rem;
  font-size: 0.75rem;
  color: #94a3b8;
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 999px;
}

:root.light .assistant-message :deep(.response-interrupted-indicator),
[data-theme='light'] .assistant-message :deep(.response-interrupted-indicator) {
  color: #64748b;
  border-color: rgba(100, 116, 139, 0.25);
}

/* Error block indicator (tool_use_error etc.) */
.assistant-message :deep(.error-block-indicator) {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.25);
  color: #fca5a5;
}

:root.light .assistant-message :deep(.error-block-indicator),
[data-theme='light'] .assistant-message :deep(.error-block-indicator) {
  background: rgba(239, 68, 68, 0.06);
  border-color: rgba(239, 68, 68, 0.2);
  color: #dc2626;
}

/* Assistant status pill */
.assistant-status-bar {
  margin-top: 0.625rem;
}

.assistant-process-toggle-row {
  display: flex;
  justify-content: flex-start;
  margin-top: 0.625rem;
}

.assistant-process-toggle {
  font-size: 0.76rem;
  line-height: 1.2;
  color: #475569;
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(248, 250, 252, 0.92);
  border-radius: 999px;
  padding: 0.28rem 0.62rem;
  cursor: pointer;
  transition:
    background-color 0.15s ease,
    border-color 0.15s ease,
    color 0.15s ease;
}

.assistant-process-toggle:hover {
  background: rgba(241, 245, 249, 1);
  border-color: rgba(100, 116, 139, 0.3);
  color: #334155;
}

.assistant-process-trace-panel {
  display: flex;
  flex-direction: column;
  gap: 0;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 0.84rem;
  background: rgba(255, 255, 255, 0.78);
  overflow: hidden;
}

.assistant-process-trace-item {
  display: flex;
  align-items: flex-start;
  gap: 0.72rem;
  padding: 0.82rem 0.95rem;
}

.assistant-process-trace-item + .assistant-process-trace-item {
  border-top: 1px solid rgba(226, 232, 240, 0.92);
}

.assistant-process-trace-dot {
  width: 0.58rem;
  height: 0.58rem;
  margin-top: 0.38rem;
  border-radius: 999px;
  flex-shrink: 0;
  background: rgba(148, 163, 184, 0.82);
  box-shadow: 0 0 0 3px rgba(148, 163, 184, 0.12);
}

.assistant-process-trace-dot--info {
  background: rgba(59, 130, 246, 0.92);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12);
}

.assistant-process-trace-dot--active {
  background: rgba(245, 158, 11, 0.96);
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.14);
}

.assistant-process-trace-dot--success {
  background: rgba(34, 197, 94, 0.96);
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.14);
}

.assistant-process-trace-dot--error {
  background: rgba(239, 68, 68, 0.96);
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.12);
}

.assistant-process-trace-main {
  min-width: 0;
  flex: 1;
}

.assistant-process-trace-label {
  font-size: 0.82rem;
  line-height: 1.45;
  font-weight: 600;
  color: #334155;
}

.assistant-process-trace-command {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  margin-top: 0.42rem;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  font-size: 0.76rem;
  line-height: 1.5;
  color: #475569;
}

.assistant-process-trace-command-prefix {
  flex-shrink: 0;
  color: #64748b;
  user-select: none;
}

.assistant-process-trace-command-text {
  min-width: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

.assistant-process-trace-detail {
  margin-top: 0.42rem;
  font-size: 0.78rem;
  line-height: 1.55;
  color: #64748b;
  white-space: pre-wrap;
  word-break: break-word;
}

.tool-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  background: rgba(14, 165, 233, 0.1);
  border: 1px solid rgba(14, 165, 233, 0.2);
}

.assistant-status-pill {
  max-width: 100%;
}

.assistant-status-pill-idle {
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.2);
}

.assistant-status-pill-active {
  background: rgba(14, 165, 233, 0.1);
  border-color: rgba(14, 165, 233, 0.2);
}

.assistant-status-pill-warn {
  background: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.24);
}

.sandbox-badge {
  display: inline-flex;
  align-items: center;
  color: #16a34a;
  margin-inline-start: -0.125rem;
}

:root.dark .sandbox-badge,
[data-theme='dark'] .sandbox-badge {
  color: #4ade80;
}

:root.dark .assistant-process-trace-panel,
[data-theme='dark'] .assistant-process-trace-panel {
  border-color: rgba(71, 85, 105, 0.72);
  background: rgba(15, 23, 42, 0.56);
}

:root.dark .assistant-process-trace-item + .assistant-process-trace-item,
[data-theme='dark'] .assistant-process-trace-item + .assistant-process-trace-item {
  border-top-color: rgba(51, 65, 85, 0.88);
}

:root.dark .assistant-process-trace-label,
[data-theme='dark'] .assistant-process-trace-label {
  color: rgba(226, 232, 240, 0.96);
}

:root.dark .assistant-process-trace-command,
[data-theme='dark'] .assistant-process-trace-command {
  color: rgba(203, 213, 225, 0.84);
}

:root.dark .assistant-process-trace-command-prefix,
[data-theme='dark'] .assistant-process-trace-command-prefix {
  color: rgba(148, 163, 184, 0.82);
}

:root.dark .assistant-process-trace-detail,
[data-theme='dark'] .assistant-process-trace-detail {
  color: rgba(148, 163, 184, 0.94);
}

:root.dark .tool-pill,
[data-theme='dark'] .tool-pill {
  background: rgba(56, 189, 248, 0.14);
  border-color: rgba(56, 189, 248, 0.24);
}

:root.dark .assistant-status-pill-idle,
[data-theme='dark'] .assistant-status-pill-idle {
  background: rgba(71, 85, 105, 0.4);
  border-color: rgba(100, 116, 139, 0.38);
}

:root.dark .assistant-process-toggle,
[data-theme='dark'] .assistant-process-toggle {
  color: rgba(226, 232, 240, 0.9);
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(30, 41, 59, 0.55);
}

:root.dark .assistant-process-toggle:hover,
[data-theme='dark'] .assistant-process-toggle:hover {
  color: #fff;
  border-color: rgba(148, 163, 184, 0.36);
  background: rgba(51, 65, 85, 0.8);
}

:root.dark .assistant-status-pill-warn,
[data-theme='dark'] .assistant-status-pill-warn {
  background: rgba(217, 119, 6, 0.18);
  border-color: rgba(245, 158, 11, 0.28);
}

.tool-dots {
  display: inline-flex;
  gap: 3px;
  align-items: center;
}

.tool-dots span {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #0ea5e9;
  animation: tool-dot-pulse 1.2s ease-in-out infinite;
}

.tool-dots span:nth-child(2) {
  animation-delay: 0.2s;
}
.tool-dots span:nth-child(3) {
  animation-delay: 0.4s;
}

:root.dark .tool-dots span,
[data-theme='dark'] .tool-dots span {
  background: #38bdf8;
}

.tool-label {
  font-size: 0.75rem;
  font-weight: 500;
  color: #0284c7;
}

.assistant-status-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:root.dark .tool-label,
[data-theme='dark'] .tool-label {
  color: #7dd3fc;
}

.assistant-status-pill-idle .assistant-status-label {
  color: #475569;
}

.assistant-status-pill-warn .assistant-status-label {
  color: #b45309;
}

:root.dark .assistant-status-pill-idle .assistant-status-label,
[data-theme='dark'] .assistant-status-pill-idle .assistant-status-label {
  color: #cbd5e1;
}

:root.dark .assistant-status-pill-warn .assistant-status-label,
[data-theme='dark'] .assistant-status-pill-warn .assistant-status-label {
  color: #fcd34d;
}

.tool-timer {
  font-size: 0.6875rem;
  color: #94a3b8;
}

.assistant-status-timer {
  flex-shrink: 0;
}

:root.dark .tool-timer,
[data-theme='dark'] .tool-timer {
  color: #64748b;
}

@keyframes tool-dot-pulse {
  0%,
  80%,
  100% {
    opacity: 0.3;
    transform: scale(0.8);
  }
  40% {
    opacity: 1;
    transform: scale(1);
  }
}

.tool-names {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-top: 0.375rem;
}

.tool-name-tag {
  display: inline-block;
  font-size: 0.6875rem;
  font-weight: 500;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  background: rgba(14, 165, 233, 0.08);
  color: #0284c7;
  border: 1px solid rgba(14, 165, 233, 0.16);
}

:root.dark .tool-name-tag,
[data-theme='dark'] .tool-name-tag {
  background: rgba(56, 189, 248, 0.12);
  color: #7dd3fc;
  border-color: rgba(56, 189, 248, 0.22);
}

/* Voice message bubble — WeChat/WhatsApp style */
.voice-message-container {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.25rem;
}

.voice-bubble {
  border-radius: 0.95rem 0.35rem 0.95rem 0.95rem;
  background: var(--chat-user-bg, linear-gradient(135deg, #0284c7, #0891b2));
  border: var(--chat-user-border, none);
  padding: 0.5rem 0.75rem;
  color: var(--chat-user-text, #f8fafc);
  box-shadow: 0 10px 20px rgba(8, 145, 178, 0.24);
  transition: all 0.15s ease;
  min-width: 80px;
}

.voice-bubble:hover {
  filter: brightness(1.05);
}

.voice-bubble:active {
  transform: scale(0.98);
}

.voice-bubble-inner {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.voice-play-btn {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.24);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
}

.voice-bubble:hover .voice-play-btn {
  background: rgba(255, 255, 255, 0.34);
}

.voice-waveform {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 2px;
  height: 24px;
  min-width: 0;
}

.voice-bar {
  flex: 1;
  min-width: 2px;
  max-width: 4px;
  background: rgba(255, 255, 255, 0.45);
  border-radius: 1px;
  transition: background 0.1s;
}

.voice-bar-active {
  background: rgba(255, 255, 255, 0.95);
}

.voice-duration {
  flex-shrink: 0;
  font-size: 0.75rem;
  font-weight: 500;
  opacity: 0.9;
  font-variant-numeric: tabular-nums;
}
</style>
