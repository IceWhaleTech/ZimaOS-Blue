<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Message } from '@/api/chat'
import { cardActionApi } from '@/api/chat'
import { renderMarkdownCached, copyCodeToClipboard } from '@/utils/markdown'
import { useChatStore } from '@/stores/chat'
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
const userCopyButtonPositionClass = '-left-8 top-1/2 -translate-y-1/2'

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

type InlineImageInfo = { alt: string; url: string }
type InlineImageParseResult = { images: InlineImageInfo[]; strippedText: string }
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

// Throttle streaming content updates to one render per animation frame.
const throttledStreamingContent = ref(props.message.content)
let streamingRenderRafId: number | null = null
let streamingRenderFallbackTimer: ReturnType<typeof setTimeout> | null = null
let pendingStreamingContent = props.message.content
let lastStreamingFlushedContent = props.message.content
let lastStreamingFlushAt = Date.now()

const STREAMING_RENDER_MIN_DELTA = 12
const STREAMING_RENDER_MAX_DEFER_MS = 72
const RE_STREAMING_RENDER_FLUSH_HINT = /[.!?。！？\n\r`*_#\[\]|]/

function hasStreamingRenderFlushHint(content: string): boolean {
  return RE_STREAMING_RENDER_FLUSH_HINT.test(content)
}

function clearStreamingRenderFallbackTimer() {
  if (!streamingRenderFallbackTimer) return
  clearTimeout(streamingRenderFallbackTimer)
  streamingRenderFallbackTimer = null
}

function flushStreamingContent(nextContent: string) {
  throttledStreamingContent.value = nextContent
  lastStreamingFlushedContent = nextContent
  lastStreamingFlushAt = Date.now()
}

function syncStreamingContentNow(nextContent: string) {
  pendingStreamingContent = nextContent
  clearStreamingRenderFallbackTimer()
  if (streamingRenderRafId !== null) {
    window.cancelAnimationFrame(streamingRenderRafId)
    streamingRenderRafId = null
  }
  flushStreamingContent(nextContent)
}

function scheduleStreamingContentUpdate(nextContent: string) {
  pendingStreamingContent = nextContent

  // Skip immediate frame updates for tiny plain-text appends.
  if (nextContent.startsWith(lastStreamingFlushedContent)) {
    const delta = nextContent.slice(lastStreamingFlushedContent.length)
    const shouldDefer =
      delta.length > 0 &&
      delta.length < STREAMING_RENDER_MIN_DELTA &&
      !hasStreamingRenderFlushHint(delta) &&
      Date.now() - lastStreamingFlushAt < STREAMING_RENDER_MAX_DEFER_MS

    if (shouldDefer) {
      if (!streamingRenderFallbackTimer) {
        const wait = STREAMING_RENDER_MAX_DEFER_MS - (Date.now() - lastStreamingFlushAt)
        streamingRenderFallbackTimer = setTimeout(
          () => {
            streamingRenderFallbackTimer = null
            if (streamingRenderRafId !== null) return
            streamingRenderRafId = window.requestAnimationFrame(() => {
              streamingRenderRafId = null
              flushStreamingContent(pendingStreamingContent)
            })
          },
          Math.max(8, wait)
        )
      }
      return
    }
  }

  clearStreamingRenderFallbackTimer()
  if (streamingRenderRafId !== null) return
  streamingRenderRafId = window.requestAnimationFrame(() => {
    streamingRenderRafId = null
    flushStreamingContent(pendingStreamingContent)
  })
}

watch(
  () => props.message.content,
  (content) => {
    if (trackStreamingState.value) {
      scheduleStreamingContentUpdate(content)
      return
    }
    syncStreamingContentNow(content)
  },
  { immediate: true }
)

watch(
  () => trackStreamingState.value,
  () => {
    syncStreamingContentNow(props.message.content)
  }
)

watch(
  () => [trackStreamingState.value, chatStore.toolExecuting] as const,
  ([trackStreaming, toolExecuting]) => {
    if (trackStreaming && toolExecuting) {
      syncStreamingContentNow(props.message.content)
    }
  }
)

const renderSourceContent = computed(() =>
  trackStreamingState.value ? throttledStreamingContent.value : props.message.content
)

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

// TTS playback state
const isSpeaking = ref(false)
const ttsError = ref<string | null>(null)

// Attachment preview state
const previewAttachment = ref<{ type: string; src: string; name: string; content?: string } | null>(
  null
)

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

// Tool execution elapsed timer
const toolElapsedSeconds = ref('0.0')
let toolTimerHandle: ReturnType<typeof setInterval> | null = null
let stopToolExecutingWatch: (() => void) | null = null

function stopToolTimer() {
  if (toolTimerHandle) {
    clearInterval(toolTimerHandle)
    toolTimerHandle = null
  }
}

function startToolTimer() {
  if (toolTimerHandle) return
  toolElapsedSeconds.value = '0.0'
  toolTimerHandle = setInterval(() => {
    if (chatStore.toolExecutingStartTime > 0) {
      toolElapsedSeconds.value = ((Date.now() - chatStore.toolExecutingStartTime) / 1000).toFixed(1)
    }
  }, 100)
}

function stopToolExecutingStateWatch() {
  if (stopToolExecutingWatch) {
    stopToolExecutingWatch()
    stopToolExecutingWatch = null
  }
}

watch(
  () => trackStreamingState.value,
  (trackStreaming) => {
    stopToolTimer()
    stopToolExecutingStateWatch()

    if (!trackStreaming) return

    stopToolExecutingWatch = watch(
      () => chatStore.toolExecuting,
      (executing) => {
        if (executing) {
          startToolTimer()
          return
        }
        stopToolTimer()
      },
      { immediate: true }
    )
  },
  { immediate: true }
)

// Derive display names for tool pill: extract skill names from "blue <subcommand>" commands
const toolDisplayNames = computed(() => {
  if (!trackStreamingState.value || !chatStore.toolExecuting) return []
  const commands = chatStore.toolExecutingCommands
  if (commands.length > 0) {
    // Extract skill name from "blue <subcommand> ..." pattern
    return commands.map((cmd) => {
      const m = cmd.match(/^blue\s+(\S+)/)
      return m ? m[1] : formatToolName('exec')
    })
  }
  // Fallback to tool names
  return chatStore.toolExecutingNames.map(formatToolName)
})

// Waiting timer — shows elapsed time when response takes >3s with no content
const waitingElapsed = ref('')
const showWaitingTimer = ref(false)
let waitingTimerHandle: ReturnType<typeof setInterval> | null = null
let waitingStartTime = 0
let stopWaitingStateWatch: (() => void) | null = null

function startWaitingTimer() {
  waitingStartTime = Date.now()
  showWaitingTimer.value = false
  waitingElapsed.value = ''
  waitingTimerHandle = setInterval(() => {
    const elapsed = (Date.now() - waitingStartTime) / 1000
    if (elapsed >= 3) {
      showWaitingTimer.value = true
      waitingElapsed.value = elapsed.toFixed(1)
    }
  }, 100)
}

function stopWaitingTimer() {
  showWaitingTimer.value = false
  if (waitingTimerHandle) {
    clearInterval(waitingTimerHandle)
    waitingTimerHandle = null
  }
}

function stopWaitingTimerStateWatch() {
  if (stopWaitingStateWatch) {
    stopWaitingStateWatch()
    stopWaitingStateWatch = null
  }
}

// Start/stop waiting timer only for the active streaming assistant message
watch(
  () => trackStreamingState.value,
  (trackStreaming) => {
    stopWaitingTimer()
    stopWaitingTimerStateWatch()

    if (!trackStreaming) return

    stopWaitingStateWatch = watch(
      () => [props.message.content, chatStore.toolExecuting] as const,
      ([content, toolExec]) => {
        if (!content && !toolExec) {
          if (!waitingTimerHandle) startWaitingTimer()
          return
        }
        stopWaitingTimer()
      },
      { immediate: true }
    )
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
  let html = renderMarkdownCached(cleaned, `chat-message:${props.message.id}`)
  if (interrupted) {
    html += interruptedHtml
  }
  return saveCache({ html, isEmpty: false })
})

const renderedContent = computed(() => assistantTextState.value.html)

// Whether bubble content is empty (only indicators showing)
const isContentEmpty = computed(() => {
  return (
    assistantTextState.value.isEmpty &&
    (!settingsStore.showToolDetails || persistedProcessToolResults.value.length === 0)
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

  if (!hasTypelessCards(content, !props.isStreaming)) {
    return null
  }
  // Use incremental parsing for streaming to avoid re-parsing entire content
  // Pass conversation_id to ensure cache key uniqueness across conversations
  if (props.isStreaming) {
    return parseTypelessContentIncremental(
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
    let html = renderMarkdownCached(content, `chat-segment:${segment.key}`)
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

  const isCardOnly = hasCards && !hasNonEmptyText
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
  clearStreamingRenderFallbackTimer()
  if (streamingRenderRafId !== null) {
    window.cancelAnimationFrame(streamingRenderRafId)
    streamingRenderRafId = null
  }
  stopToolExecutingStateWatch()
  stopWaitingTimerStateWatch()
  clearIncrementalState(props.message.render_key || props.message.id, props.message.conversation_id)
  clearSplitSegmentsIncrementalState(
    `${props.message.conversation_id || 'unknown'}:${props.message.id}`
  )
  clearStreamingSegmentRenderIncrementalState(
    `${props.message.conversation_id || 'unknown'}:${props.message.id}`
  )
  stopToolTimer()
  stopWaitingTimer()
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
    }
  } else if (attachment.type === 'file') {
    // For text files, decode and show content
    if (isTextMimeType(attachment.mime_type)) {
      try {
        const content = decodeTextContent(attachment.data)
        previewAttachment.value = {
          type: 'text',
          src: '',
          name: attachment.name,
          content: content,
        }
      } catch {
        previewAttachment.value = {
          type: 'file',
          src: '',
          name: attachment.name,
        }
      }
    } else {
      previewAttachment.value = {
        type: 'file',
        src: '',
        name: attachment.name,
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

// Format internal tool name via i18n (e.g. "web_search" → "网页搜索" in zh-CN)
function formatToolName(name: string): string {
  const key = `tools.names.${name}`
  if (te(key)) return t(key)
  // Fallback: title-case the snake_case name
  return name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
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

async function handleMobileCopy() {
  await handleCopyMessage()
  closeMobileActions()
}

async function handleMobileTTS() {
  await handlePlayTTS()
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
      class="absolute left-2 top-1/2 -translate-y-1/2 flex items-center z-10"
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
        'pl-8': isMultiSelectMode,
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
                @click="
                  attachment.type === 'audio'
                    ? playVoiceMessage(attachment, index)
                    : openAttachmentPreview(attachment)
                "
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
          </div>
          <!-- Copy button for user message -->
          <button
            v-if="!isStreaming && !isMultiSelectMode"
            :class="[
              'copy-message-btn absolute opacity-0 group-hover:opacity-100 transition-opacity p-1.5 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700',
              userCopyButtonPositionClass,
            ]"
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

        <!-- Card-only assistant message: render cards directly without bubble wrapper -->
        <div v-else-if="isCardOnly && cardOnlySegments" class="assistant-message-wrapper relative">
          <TypelessCardComponent
            v-for="segment in cardOnlySegments"
            :key="segment.key"
            :card="segment.content as TypelessCard"
            :action-loading="isCardActionLoading((segment.content as TypelessCard).id)"
            :active-action-id="activeCardActionId((segment.content as TypelessCard).id)"
            :action-error="cardActionErrorMessage((segment.content as TypelessCard).id)"
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
          <div
            v-if="hasMediaTask"
            class="assistant-message chat-assistant-bubble px-4 py-3 max-w-none"
          >
            <MediaPlaceholder :task-id="mediaTaskId" />
          </div>

          <!-- Render with typeless cards embedded in single bubble -->
          <div
            v-else
            :class="[
              'assistant-message chat-copy-bubble chat-assistant-bubble px-4 prose prose-slate dark:prose-invert max-w-none',
              {
                'assistant-message-indicator-only':
                  showWaitingTimer || (isContentEmpty && !(isStreaming && chatStore.toolExecuting)),
                'py-3': !(
                  showWaitingTimer ||
                  (isContentEmpty && !(isStreaming && chatStore.toolExecuting))
                ),
              },
            ]"
            @click="handleCopyClick"
          >
            <template v-if="effectiveHasCards && renderedContentSegments">
              <template v-for="segment in renderedContentSegments" :key="segment.key">
                <div
                  v-if="segment.type === 'text'"
                  class="prose-content"
                  v-html="'html' in segment ? segment.html : ''"
                />
                <TypelessCardComponent
                  v-else
                  :key="segment.key"
                  :card="segment.content as TypelessCard"
                  :action-loading="isCardActionLoading((segment.content as TypelessCard).id)"
                  :active-action-id="activeCardActionId((segment.content as TypelessCard).id)"
                  :action-error="cardActionErrorMessage((segment.content as TypelessCard).id)"
                  class="my-3 -mx-1"
                  @action="handleCardAction"
                  @select="handleCardSelect"
                />
              </template>
            </template>
            <!-- Render without typeless cards -->
            <div v-else-if="!isContentEmpty" class="prose-content" v-html="renderedContent" />
            <div
              v-if="
                !isStreaming &&
                persistedProcessToolResults.length > 0 &&
                settingsStore.showToolDetails
              "
              class="tool-detail-cards my-2 -mx-1"
            >
              <ToolDetailCard
                v-for="item in persistedProcessToolResults"
                :key="item.id"
                :item="item"
              />
            </div>
            <!-- Tool detail cards (collapsible, shown when toggle is on) -->
            <div
              v-if="
                isStreaming && chatStore.toolResults.length > 0 && settingsStore.showToolDetails
              "
              class="tool-detail-cards my-2 -mx-1"
            >
              <ToolDetailCard v-for="item in chatStore.toolResults" :key="item.id" :item="item" />
            </div>
            <!-- Tool execution indicator (inside bubble) -->
            <div
              v-if="isStreaming && chatStore.toolExecuting && settingsStore.showToolDetails"
              :class="['tool-executing-indicator', { 'mt-0': isContentEmpty }]"
            >
              <div class="tool-pill">
                <span class="tool-dots"> <span /><span /><span /> </span>
                <span class="tool-label">{{ t('tools.callingProgress') }}</span>
                <span class="tool-timer tabular-nums">{{ toolElapsedSeconds }}s</span>
                <span
                  v-if="chatStore.toolSandboxAvailable"
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
              <div v-if="toolDisplayNames.length > 0" class="tool-names">
                <span v-for="name in toolDisplayNames" :key="name" class="tool-name-tag">{{
                  name
                }}</span>
              </div>
            </div>
            <!-- Minimal thinking indicator when tool details hidden -->
            <div
              v-else-if="isStreaming && chatStore.toolExecuting && !settingsStore.showToolDetails"
              class="tool-executing-indicator"
              :class="{ 'mt-0': isContentEmpty }"
            >
              <div class="tool-pill">
                <span class="tool-dots"> <span /><span /><span /> </span>
                <span class="tool-timer tabular-nums">{{ toolElapsedSeconds }}s</span>
              </div>
            </div>
            <!-- Waiting timer card (>3s with no content) -->
            <div
              v-if="showWaitingTimer"
              :class="['-mx-1', isContentEmpty ? 'my-0' : 'my-3']"
              class="waiting-card"
            >
              <div class="waiting-card-inner">
                <div class="waiting-card-header">
                  <svg class="waiting-card-spinner" width="20" height="20" viewBox="0 0 24 24">
                    <circle
                      cx="12"
                      cy="12"
                      r="10"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                      opacity="0.15"
                    />
                    <circle
                      cx="12"
                      cy="12"
                      r="10"
                      fill="none"
                      stroke="url(#waitGrad)"
                      stroke-width="2.5"
                      stroke-linecap="round"
                      stroke-dasharray="40 23"
                    />
                    <defs>
                      <linearGradient id="waitGrad" x1="0" y1="0" x2="1" y2="1">
                        <stop offset="0%" stop-color="#818cf8" />
                        <stop offset="100%" stop-color="#c084fc" />
                      </linearGradient>
                    </defs>
                  </svg>
                  <span class="waiting-card-title">{{
                    t('chat.waitingThinking', 'Thinking...')
                  }}</span>
                  <span class="waiting-card-timer tabular-nums">{{ waitingElapsed }}s</span>
                </div>
                <div class="waiting-card-bar">
                  <div class="waiting-card-bar-fill" />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Streaming indicator -->
        <div
          v-if="isStreaming && isAssistant && !chatStore.toolExecuting && !showWaitingTimer"
          class="streaming-indicator mt-2"
        >
          <span class="inline-flex gap-1">
            <span
              class="w-2 h-2 bg-gray-700 dark:bg-gray-500 rounded-full animate-bounce"
              style="animation-delay: 0ms"
            />
            <span
              class="w-2 h-2 bg-gray-700 dark:bg-gray-500 rounded-full animate-bounce"
              style="animation-delay: 150ms"
            />
            <span
              class="w-2 h-2 bg-gray-700 dark:bg-gray-500 rounded-full animate-bounce"
              style="animation-delay: 300ms"
            />
          </span>
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
            class="absolute -top-10 right-0 text-white hover:text-gray-300 transition-colors"
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
          <img
            v-if="previewAttachment.type === 'image'"
            :src="previewAttachment.src"
            :alt="previewAttachment.name"
            class="max-w-full max-h-[85vh] object-contain rounded-lg"
          />
          <!-- Text file preview -->
          <div
            v-else-if="previewAttachment.type === 'text'"
            class="bg-gray-200 rounded-lg p-4 max-w-[80vw] max-h-[80vh] overflow-auto"
          >
            <pre class="text-sm text-gray-100 whitespace-pre-wrap font-mono">{{
              previewAttachment.content
            }}</pre>
          </div>
          <!-- Generic file preview -->
          <div v-else class="bg-gray-700 rounded-lg p-8 flex flex-col items-center gap-4">
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
            <span class="text-gray-300">{{ t('chat.filePreviewNotSupported') }}</span>
          </div>
          <p class="text-center text-white text-sm mt-2">{{ previewAttachment.name }}</p>
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
                  @click="emit('continue'); closeMobileActions()"
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
                  @click="emit('regenerate'); closeMobileActions()"
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
  padding-left: 1.5rem;
}

.assistant-message :deep(p) {
  line-height: 1.64;
  margin-block: 0.38rem;
}

.assistant-message :deep(ul),
.assistant-message :deep(ol) {
  margin-block: 0.46rem;
  padding: 0.48rem 0.78rem 0.48rem 1.24rem;
  border: 1px solid rgba(203, 213, 225, 0.64);
  border-radius: 0.68rem;
  background: rgba(248, 250, 252, 0.62);
}

:root.dark .assistant-message :deep(ul),
:root.dark .assistant-message :deep(ol),
[data-theme='dark'] .assistant-message :deep(ul),
[data-theme='dark'] .assistant-message :deep(ol) {
  border-color: rgba(71, 85, 105, 0.72);
  background: rgba(15, 23, 42, 0.58);
}

.prose :deep(blockquote) {
  border-left-width: 3px;
  border-left-color: rgba(148, 163, 184, 0.7);
  padding-left: 0.9rem;
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

/* Copy button styles */
.copy-message-btn {
  background: transparent;
}

.assistant-message-with-actions {
  padding-right: 2.4rem;
}

.assistant-actions {
  position: absolute;
  top: 0.1rem;
  right: 0;
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

@media (max-width: 639px) {
  .assistant-message-with-actions {
    padding-right: 0;
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

/* Tool execution indicator */
.tool-executing-indicator {
  margin-top: 0.625rem;
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

.sandbox-badge {
  display: inline-flex;
  align-items: center;
  color: #16a34a;
  margin-left: -0.125rem;
}

:root.dark .sandbox-badge,
[data-theme='dark'] .sandbox-badge {
  color: #4ade80;
}

:root.dark .tool-pill,
[data-theme='dark'] .tool-pill {
  background: rgba(56, 189, 248, 0.14);
  border-color: rgba(56, 189, 248, 0.24);
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

:root.dark .tool-label,
[data-theme='dark'] .tool-label {
  color: #7dd3fc;
}

.tool-timer {
  font-size: 0.6875rem;
  color: #94a3b8;
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

/* Waiting timer pill — appears after 3s of no response */
/* Waiting timer card */
.waiting-card {
  animation: waiting-fade-in 0.3s ease;
}

.waiting-card-inner {
  border-radius: 0.75rem;
  border: 1px solid transparent;
  background:
    linear-gradient(#fff, #fff) padding-box,
    linear-gradient(135deg, #0ea5e9, #14b8a6, #22c55e) border-box;
  padding: 0.75rem 1rem;
  box-shadow: 0 1px 3px rgba(129, 140, 248, 0.12);
}

:root.dark .waiting-card-inner,
[data-theme='dark'] .waiting-card-inner {
  background:
    linear-gradient(#1e293b, #1e293b) padding-box,
    linear-gradient(135deg, #0ea5e9, #14b8a6, #22c55e) border-box;
  box-shadow: 0 1px 6px rgba(129, 140, 248, 0.15);
}

.waiting-card-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.waiting-card-spinner {
  animation: waiting-spin 1.2s linear infinite;
  color: #0ea5e9;
  flex-shrink: 0;
}

.waiting-card-title {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #0284c7;
  flex: 1;
}

:root.dark .waiting-card-title,
[data-theme='dark'] .waiting-card-title {
  color: #7dd3fc;
}

.waiting-card-timer {
  font-size: 0.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: #14b8a6;
  min-width: 2.5rem;
  text-align: right;
}

.waiting-card-bar {
  margin-top: 0.5rem;
  height: 3px;
  border-radius: 2px;
  background: #e2e8f0;
  overflow: hidden;
}

:root.dark .waiting-card-bar,
[data-theme='dark'] .waiting-card-bar {
  background: #334155;
}

.waiting-card-bar-fill {
  height: 100%;
  border-radius: 2px;
  background: linear-gradient(90deg, #0ea5e9, #14b8a6, #22c55e);
  animation: waiting-bar-slide 2s ease-in-out infinite;
  width: 40%;
}

@keyframes waiting-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@keyframes waiting-bar-slide {
  0% {
    transform: translateX(-100%);
  }
  50% {
    transform: translateX(150%);
  }
  100% {
    transform: translateX(-100%);
  }
}

@keyframes waiting-fade-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
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
