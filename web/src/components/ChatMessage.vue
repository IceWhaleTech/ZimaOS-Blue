<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Message } from '@/api/chat'
import { cardActionApi } from '@/api/chat'
import { renderMarkdownCached, copyCodeToClipboard } from '@/utils/markdown'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { useProviderPoolStore } from '@/stores/providerPool'
import { parseTypelessContent, parseTypelessContentIncremental, splitIntoSegments, hasTypelessCards, clearIncrementalState } from '@/utils/typeless'
import type { TypelessCard, TypelessCardChoice } from '@/types/typeless'
import TypelessCardComponent from '@/components/typeless/TypelessCard.vue'
import ToolDetailCard from '@/components/ToolDetailCard.vue'
import MediaPlaceholder from '@/components/MediaPlaceholder.vue'
import { ttsAudioManager, streamingTTSManager } from '@/api/voice'
import { speechApi } from '@/api/speech'
import { useNotificationStore } from '@/stores/notification'

const { t } = useI18n()
const providerPoolStore = useProviderPoolStore()

const props = defineProps<{
  message: Message
  isStreaming?: boolean
  isLastAssistantMessage?: boolean
  isMobile?: boolean
  isSelected?: boolean
  isMultiSelectMode?: boolean
}>()

const emit = defineEmits<{
  contextmenu: [event: MouseEvent, messageId: string]
  cardAction: [conversationId: string, messageId: string, cardId: string, actionId: string, actionLabel?: string]
  continue: []
  regenerate: []
}>()

const chatStore = useChatStore()
const settingsStore = useSettingsStore()

// Card action state
const cardActionLoading = ref<string | null>(null) // cardId that is loading
const cardActionError = ref<string | null>(null)

const isUser = computed(() => props.message.role === 'user')
const isAssistant = computed(() => props.message.role === 'assistant')
const isSelected = computed(() => props.isSelected ?? chatStore.selectedMessageIds.has(props.message.id))
const isMultiSelectMode = computed(() => props.isMultiSelectMode ?? chatStore.isMultiSelectMode)
const isMobile = computed(() => !!props.isMobile)

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
const hasAttachments = computed(() => isUser.value && props.message.attachments && props.message.attachments.length > 0)

// Check if this is a voice-only message (audio attachment, no meaningful text)
const isVoiceMessage = computed(() => {
  if (!isUser.value || !props.message.attachments) return false
  const audioAttachments = props.message.attachments.filter(a => a.type === 'audio')
  if (audioAttachments.length === 0) return false
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

  const strippedText = images.length > 0
    ? content.replace(INLINE_IMAGE_STRIP_RE, '').trim()
    : content

  const parsed = { images, strippedText }
  if (cache.size >= INLINE_IMAGE_CACHE_MAX) cache.clear()
  cache.set(content, parsed)
  return parsed
}

const parsedInlineContent = computed(() => {
  if (!isUser.value) return null
  return parseInlineImagesCached(props.message.content)
})

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
const previewAttachment = ref<{ type: string; src: string; name: string; content?: string } | null>(null)

// Voice message playback state
const playingAudioId = ref<number | null>(null)
const audioProgress = ref(0) // 0-1
let activeAudio: HTMLAudioElement | null = null
let audioProgressTimer: ReturnType<typeof setInterval> | null = null

function playVoiceMessage(attachment: { mime_type: string; data: string; duration?: number }, index: number) {
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

watch(
  () => [trackStreamingState.value, chatStore.toolExecuting] as const,
  ([trackStreaming, executing]) => {
    if (trackStreaming && executing) {
      startToolTimer()
      return
    }
    stopToolTimer()
  },
  { immediate: true }
)

// Derive display names for tool pill: extract skill names from "blue <subcommand>" commands
const toolDisplayNames = computed(() => {
  if (!trackStreamingState.value || !chatStore.toolExecuting) return []
  const commands = chatStore.toolExecutingCommands
  if (commands.length > 0) {
    // Extract skill name from "blue <subcommand> ..." pattern
    return commands.map(cmd => {
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

// Start/stop waiting timer based on streaming state + empty content
watch(
  () => [trackStreamingState.value, props.message.content, chatStore.toolExecuting] as const,
  ([trackStreaming, content, toolExec]) => {
    if (trackStreaming && !content && !toolExec) {
      if (!waitingTimerHandle) startWaitingTimer()
    } else {
      stopWaitingTimer()
    }
  },
  { immediate: true },
)

// Helper to strip markdown heading from first line if present
function stripFirstLineHeading(content: string): string {
  const lines = content.split('\n')
  const firstLine = lines[0]?.trim() || ''
  if (firstLine.startsWith('#')) {
    // Remove the first line (markdown heading used as title)
    return lines.slice(1).join('\n').trimStart()
  }
  return content
}

const interruptedIndicatorHtml = computed(() =>
  `<div class="response-interrupted-indicator"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg><span>${t('chat.responseInterrupted')}</span></div>`
)

const RE_INTERRUPTED = /\n?\n?\[Response interrupted\]\s*$/

// Strip [Response interrupted] marker from content, returns { content, interrupted }
function stripInterruptedMarker(content: string): { content: string; interrupted: boolean } {
  if (RE_INTERRUPTED.test(content)) {
    return { content: content.replace(RE_INTERRUPTED, ''), interrupted: true }
  }
  return { content, interrupted: false }
}

// Render segment text with [Response interrupted] handling
function renderSegmentHtml(text: string): string {
  // When tool details are hidden, strip process blocks from text segments too
  const effective = settingsStore.showToolDetails ? text : stripProcessContent(text)
  const { content, interrupted } = stripInterruptedMarker(effective)
  let html = renderMarkdownCached(content, 'chat-segment')
  if (interrupted) {
    html += interruptedIndicatorHtml.value
  }
  return html
}

// Cache stripped content to avoid double stripFirstLineHeading + SILENT_REPLY replacement
const strippedContent = computed(() => {
  if (isUser.value) return props.message.content
  let content = stripFirstLineHeading(props.message.content)
  content = content.replace(/\[SILENT_REPLY\]/g, '💤')
  return content
})

// Regex to strip process content (tool results) and typeless card blocks
const RE_PROCESS_BLOCK = /\n*<!-- process-start -->[\s\S]*?<!-- process-end -->\n*/g
const RE_TYPELESS_BLOCK = /\n*```typeless\s*[\s\S]*?```\n*/g
const PROCESS_STRIP_CACHE_KEY = '__zima_chat_process_strip_cache_v1__'
const PROCESS_STRIP_CACHE_MAX = 300

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

// Strip process content (tool results + typeless cards) from text
function stripProcessContent(text: string): string {
  const cache = getProcessStripCache()
  const cached = cache.get(text)
  if (cached !== undefined) return cached

  const stripped = text.replace(RE_PROCESS_BLOCK, '\n').replace(RE_TYPELESS_BLOCK, '\n').trim()
  if (cache.size >= PROCESS_STRIP_CACHE_MAX) cache.clear()
  cache.set(text, stripped)
  return stripped
}

const renderedContent = computed(() => {
  if (isUser.value) {
    return props.message.content
  }
  let text = strippedContent.value
  // When tool details are hidden, strip process blocks and typeless card blocks
  if (!settingsStore.showToolDetails) {
    text = stripProcessContent(text)
  }
  const { content: cleaned, interrupted } = stripInterruptedMarker(text)
  let html = renderMarkdownCached(cleaned, 'chat-message')
  if (interrupted) {
    html += interruptedIndicatorHtml.value
  }
  return html
})

// Whether bubble content is empty (only indicators showing)
const isContentEmpty = computed(() => {
  if (isUser.value) return false
  // If we have result cards to show, content is not empty
  if (effectiveHasCards.value) return false
  let text = strippedContent.value
  if (!settingsStore.showToolDetails) {
    text = stripProcessContent(text)
  }
  return !text?.trim()
})

// Hide empty assistant messages that are not streaming (collapsed empty bubbles)
const shouldHideMessage = computed(() => {
  if (isUser.value || props.isStreaming) return false
  return isContentEmpty.value && !hasMediaTask.value
})

// Parse typeless cards from assistant messages
// Use incremental parsing for streaming messages, regular parsing for completed messages
const parsedContent = computed(() => {
  if (isUser.value || !hasTypelessCards(strippedContent.value)) {
    return null
  }
  // Use incremental parsing for streaming to avoid re-parsing entire content
  // Pass conversation_id to ensure cache key uniqueness across conversations
  if (props.isStreaming) {
    return parseTypelessContentIncremental(strippedContent.value, props.message.id, props.message.conversation_id)
  }
  return parseTypelessContent(strippedContent.value)
})

// Get content segments (text and cards interleaved) with unique keys
const contentSegments = computed(() => {
  if (!parsedContent.value) {
    return null
  }
  const segments = splitIntoSegments(parsedContent.value.text, parsedContent.value.cards)
  // Add unique keys to each segment for proper Vue reactivity
  // Include conversation_id to ensure uniqueness across different conversations
  const convId = props.message.conversation_id || 'unknown'
  return segments.map((segment, index) => ({
    ...segment,
    key: segment.type === 'card'
      ? `${convId}-${props.message.id}-card-${(segment.content as TypelessCard).id || index}`
      : `${convId}-${props.message.id}-text-${index}`
  }))
})

// Card types that represent content or results — always visible even when tool details are hidden.
// Only tool-invocation cards (steps) are hidden when details are off.
const RESULT_CARD_TYPES = new Set([
  'result', 'search', 'weather', 'chart', 'gallery', 'map', 'profile',
  'rating', 'comparison', 'metric', 'link', 'audio', 'video', 'file',
  'media-generate', 'ui-review', 'detection',
  'ui-review-progress', 'analyze-progress', 'browser-progress',
  'list', 'table', 'code', 'terminal', 'mermaid', 'accordion',
])

// Effective values respecting showToolDetails toggle.
// Result-type cards are always shown; process/progress cards are hidden when details are off.
const effectiveContentSegments = computed(() => {
  if (!contentSegments.value) return null
  if (settingsStore.showToolDetails) return contentSegments.value
  // Filter: keep text segments and result-type cards only
  const filtered = contentSegments.value.filter(s => {
    if (s.type !== 'card') return true
    const card = s.content as TypelessCard
    return RESULT_CARD_TYPES.has(card.type)
  })
  // If no cards survived filtering, return null to fall back to plain text
  return filtered.some(s => s.type === 'card') ? filtered : null
})
const effectiveHasCards = computed(() => effectiveContentSegments.value !== null && effectiveContentSegments.value.some(s => s.type === 'card'))

// Card-only: no text segments, only cards — skip assistant bubble wrapper
const isCardOnly = computed(() => {
  if (!effectiveHasCards.value || !effectiveContentSegments.value) return false
  return effectiveContentSegments.value.every(s => s.type !== 'text' || !(s.content as string).trim())
})

const formattedTime = computed(() => {
  const date = new Date(props.message.created_at)
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})

const formattedTimeLong = computed(() => {
  const date = new Date(props.message.created_at)
  return date.toLocaleString([], { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
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
  const provider = providerPoolStore.providers.find(p => p.name === metadata.value?.provider || p.id === metadata.value?.provider)
  return provider?.location || null
})

function handleCopyClick(event: Event) {
  const target = event.target as HTMLElement
  if (target.classList.contains('copy-btn')) {
    const code = target.dataset.code
    if (code) {
      copyCodeToClipboard(code).then(() => {
        target.textContent = 'Copied!'
        setTimeout(() => {
          target.textContent = 'Copy'
        }, 2000)
      })
    }
  }
}

function formatTokens(num: number | undefined): string {
  if (num === undefined || num === null || num === 0) return ''
  return num.toLocaleString()
}

function formatTTFT(ms: number | undefined): string {
  if (ms === undefined || ms === null || ms === 0) return ''
  // Always convert to seconds
  const seconds = ms / 1000
  return seconds.toFixed(2) + 's'
}

function formatSpeed(tps: number | undefined): string {
  if (tps === undefined || tps === null || tps === 0) return ''
  return tps.toFixed(1)
}

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
  clearIncrementalState(props.message.id, props.message.conversation_id)
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
  return text
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
}

// Watch for content changes during streaming to play incrementally
watch(() => props.message.content, (newContent, _oldContent) => {
  if (!trackStreamingState.value) return

  const autoPlayEnabled = localStorage.getItem('tts-auto-play') === 'true'
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
})

// Play remaining text when streaming completes
watch(() => props.isStreaming, async (isStreaming, wasStreaming) => {
  if (wasStreaming && !isStreaming && isAssistant.value) {
    const autoPlayEnabled = localStorage.getItem('tts-auto-play') === 'true'
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
})

// Handle card action (button click)
async function handleCardAction(actionId: string, cardId?: string) {
  if (!cardId) return

  // Find the action label and card metadata (actions exist on result, ui-review, alert, action cards)
  let actionLabel: string | undefined
  const card = parsedContent.value?.cards.find(c => c.id === cardId)
  if (card) {
    if ('actions' in card && Array.isArray((card as any).actions)) {
      const action = (card as any).actions.find((a: any) => a.id === actionId)
      actionLabel = action?.label
    }
  }

  cardActionLoading.value = cardId
  cardActionError.value = null

  try {
    const res = await cardActionApi.submit(props.message.conversation_id, props.message.id, {
      card_id: cardId,
      action_id: actionId,
      action_label: actionLabel,
    })
    // Emit event to parent for potential UI updates
    emit('cardAction', props.message.conversation_id, props.message.id, cardId, actionId, actionLabel)
    // Auto-send the mapped message to trigger a new streaming turn
    if (res.data?.message) {
      await chatStore.sendMessage(res.data.message)
    }
  } catch (error) {
    console.error('Card action failed:', error)
    cardActionError.value = error instanceof Error ? error.message : 'Action failed'
  } finally {
    cardActionLoading.value = null
  }
}

// Handle card selection (choice card)
async function handleCardSelect(cardId: string, selectedIds: string[], otherText?: string) {
  if (!cardId) return

  // Find the choice card
  const choiceCard = parsedContent.value?.cards.find(c => c.id === cardId) as TypelessCardChoice | undefined
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
    .map(id => choiceCard.options?.find(o => o.id === id)?.label)
    .filter(Boolean)
    .join(', ')

  cardActionLoading.value = cardId
  cardActionError.value = null

  try {
    await cardActionApi.submit(props.message.conversation_id, props.message.id, {
      card_id: cardId,
      action_id: 'select',
      action_label: selectedLabels || otherText || 'Selection',
      form_data: formData,
    })
    emit('cardAction', props.message.conversation_id, props.message.id, cardId, 'select', selectedLabels)
  } catch (error) {
    console.error('Card selection failed:', error)
    cardActionError.value = error instanceof Error ? error.message : 'Selection failed'
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

  await playTTSAudio(textContent)
}

let ttsAborted = false

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
      t(stageKey),
      { duration: 0, dismissible: true }
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
            notification.success(t('speech.initComplete'), undefined, { duration: 2000 })
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
function openAttachmentPreview(attachment: { type: string; name: string; mime_type: string; data: string }) {
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
    'text/plain', 'text/html', 'text/css', 'text/javascript',
    'text/csv', 'text/xml', 'text/markdown',
    'application/json', 'application/xml', 'application/javascript',
    'application/x-javascript', 'application/typescript',
    'application/x-yaml', 'application/yaml',
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
    const bytes = Uint8Array.from(utf8Content, c => c.charCodeAt(0))
    return decoder.decode(bytes)
  } catch {
    // If UTF-8 fails, try GBK/GB2312 (common on Windows Chinese systems)
    try {
      const binaryString = atob(base64Data)
      const bytes = Uint8Array.from(binaryString, c => c.charCodeAt(0))
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
  const translated = t(key)
  if (translated !== key) return translated
  // Fallback: title-case the snake_case name
  return name.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

// Get file icon based on extension
function getFileIcon(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() || ''
  const iconMap: Record<string, string> = {
    // Documents
    'pdf': '📄',
    'doc': '📝',
    'docx': '📝',
    'txt': '📄',
    'md': '📝',
    'rtf': '📝',
    // Spreadsheets
    'xls': '📊',
    'xlsx': '📊',
    'csv': '📊',
    // Code
    'js': '💻',
    'ts': '💻',
    'py': '🐍',
    'java': '☕',
    'go': '🔷',
    'rs': '🦀',
    'c': '💻',
    'cpp': '💻',
    'h': '💻',
    'html': '🌐',
    'css': '🎨',
    'json': '📋',
    'xml': '📋',
    'yaml': '📋',
    'yml': '📋',
    // Archives
    'zip': '📦',
    'rar': '📦',
    '7z': '📦',
    'tar': '📦',
    'gz': '📦',
    // Media
    'mp3': '🎵',
    'wav': '🎵',
    'mp4': '🎬',
    'avi': '🎬',
    'mkv': '🎬',
    // Images (shouldn't reach here but just in case)
    'png': '🖼️',
    'jpg': '🖼️',
    'jpeg': '🖼️',
    'gif': '🖼️',
    'svg': '🖼️',
    'webp': '🖼️',
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
    class="message group relative p-2 sm:p-4 transition-colors duration-150"
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
        :class="isSelected ? 'bg-gray-700 dark:bg-gray-500 border-gray-900 dark:border-gray-700' : 'border-gray-400 dark:border-gray-600'"
      >
        <svg v-if="isSelected" class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
        </svg>
      </div>
    </div>

    <!-- Message content wrapper -->
    <div
      class="flex gap-2 sm:gap-3"
      :class="{
        'justify-end': isUser,
        'pl-8': isMultiSelectMode,
      }"
    >
      <!-- Avatar (AI) -->
      <div
        v-if="isAssistant"
        class="avatar flex-shrink-0 w-6 h-6 sm:w-8 sm:h-8 rounded-full flex items-center justify-center text-white text-xs sm:text-sm font-bold"
      >
        AI
      </div>

      <!-- Content -->
      <div
        class="content min-w-0 max-w-[80%]"
      >
        <!-- Provider and Model info (above chat bubble for assistant) -->
        <div v-if="!isMobile && isAssistant && metadata && (metadata.provider || metadata.model)" class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400 mb-1">
          <!-- Cloud/Local icon -->
          <svg v-if="providerLocation === 'cloud'" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
          </svg>
          <svg v-else-if="providerLocation === 'local'" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
          <!-- Provider name -->
          <span v-if="metadata.provider">{{ providerPoolStore.getProviderDisplayName(metadata.provider) }}</span>
          <!-- Model name -->
          <span v-if="metadata.model" class="text-gray-400 dark:text-gray-500">/</span>
          <span v-if="metadata.model">{{ metadata.model }}</span>
        </div>

        <!-- User message bubble -->
        <div
          v-if="isUser"
          class="user-message-wrapper relative"
        >
          <!-- Voice message bubble (WeChat/WhatsApp style) -->
          <div v-if="isVoiceMessage" class="voice-message-container">
            <div
              v-for="(attachment, index) in message.attachments!.filter(a => a.type === 'audio')"
              :key="index"
              class="voice-bubble cursor-pointer select-none"
              :style="{ width: voiceBubbleWidth(attachment.duration) }"
              @click="playVoiceMessage(attachment, index)"
            >
              <div class="voice-bubble-inner">
                <!-- Play/Stop icon -->
                <div class="voice-play-btn">
                  <svg v-if="playingAudioId === index" class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
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
                    :class="{ 'voice-bar-active': playingAudioId === index && (bar - 1) / 12 <= audioProgress }"
                    :style="{ height: `${[40,65,50,80,60,90,55,75,45,85,70,50][bar-1]}%` }"
                  />
                </div>
                <!-- Duration -->
                <span class="voice-duration">{{ formatVoiceDuration(attachment.duration) }}</span>
              </div>
            </div>
          </div>
          <!-- Normal message bubble -->
          <div v-else class="user-message chat-user-bubble px-4 py-2 inline-block">
            <!-- Attachments display inside bubble -->
            <div v-if="hasAttachments" class="mb-2 flex flex-wrap gap-2">
              <div
                v-for="(attachment, index) in message.attachments"
                :key="index"
                class="attachment-preview rounded-lg overflow-hidden border border-gray-300 dark:border-white/20 cursor-pointer hover:opacity-90 transition-opacity bg-white/90 dark:bg-white/10"
                @click="attachment.type === 'audio' ? playVoiceMessage(attachment, index) : openAttachmentPreview(attachment)"
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
                  <svg v-if="playingAudioId === index" class="w-4 h-4 text-green-500" fill="currentColor" viewBox="0 0 24 24">
                    <rect x="6" y="4" width="4" height="16" rx="1" />
                    <rect x="14" y="4" width="4" height="16" rx="1" />
                  </svg>
                  <svg v-else class="w-4 h-4 text-gray-600 dark:text-gray-300" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M8 5v14l11-7z" />
                  </svg>
                  <span class="text-sm text-gray-700 dark:text-white/90">{{ formatVoiceDuration(attachment.duration) }}</span>
                </div>
                <!-- File attachment with icon -->
                <div
                  v-else
                  class="flex items-center gap-2 px-3 py-2"
                >
                  <span class="text-lg">{{ getFileIcon(attachment.name) }}</span>
                  <span class="text-sm text-gray-700 dark:text-white/90 max-w-[150px] truncate">{{ attachment.name }}</span>
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
            <span v-if="userTextContent && !isPlaceholderContent(userTextContent)">{{ userTextContent }}</span>
            <!-- Show continue icon when content is [CONTINUE] and no attachments -->
            <span v-else-if="message.content === '[CONTINUE]' && (!message.attachments || message.attachments.length === 0)" class="flex items-center gap-1 text-gray-500 dark:text-gray-400">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span class="text-sm">{{ t('chat.continueGenerating') }}</span>
            </span>
          </div>
          <!-- Copy button for user message -->
          <button
            v-if="!isStreaming && !isMultiSelectMode"
            class="copy-message-btn absolute -left-8 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 transition-opacity p-1.5 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700"
            :title="t('chat.copyMessage')"
            @click.stop="handleCopyMessage"
          >
            <svg v-if="copyState === 'idle'" class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            <svg v-else class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
          </button>
        </div>

        <!-- Card-only assistant message: render cards directly without bubble wrapper -->
        <div
          v-else-if="isCardOnly && effectiveContentSegments"
          class="assistant-message-wrapper relative"
        >
          <TypelessCardComponent
            v-for="segment in effectiveContentSegments.filter(s => s.type !== 'text')"
            :key="segment.key"
            :card="segment.content as TypelessCard"
            @action="handleCardAction"
            @select="handleCardSelect"
          />
        </div>

        <!-- Assistant message -->
        <div
          v-else
          class="assistant-message-wrapper relative"
        >
          <!-- Action buttons for assistant message -->
          <div
            v-if="!isStreaming && !isMultiSelectMode"
            class="absolute -right-8 top-2 flex flex-col gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
          >
            <!-- Copy button -->
            <button
              class="copy-message-btn p-1.5 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700"
              :title="t('chat.copyMessage')"
              @click.stop="handleCopyMessage"
            >
              <svg v-if="copyState === 'idle'" class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
              <svg v-else class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
            </button>
            <!-- TTS Play button -->
            <button
              class="tts-btn p-1.5 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700"
              :class="{ 'animate-pulse': isSpeaking }"
              :title="isSpeaking ? t('chat.stopTTS') : t('chat.playTTS')"
              @click.stop="handlePlayTTS"
            >
              <svg v-if="isSpeaking" class="w-4 h-4 text-gray-900 dark:text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
              </svg>
              <svg v-else class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
              </svg>
            </button>
            <!-- Export button -->
            <button
              class="export-btn p-1.5 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700"
              :title="t('chat.exportMessage')"
              @click.stop="handleExportMessage"
            >
              <svg class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            </button>
          </div>

          <!-- Media task card (replaces normal assistant content) -->
          <div v-if="hasMediaTask" class="assistant-message chat-assistant-bubble px-4 py-3 max-w-none">
            <MediaPlaceholder :task-id="mediaTaskId" />
          </div>

          <!-- Render with typeless cards embedded in single bubble -->
          <div
            v-else
            :class="['assistant-message chat-assistant-bubble px-4 prose prose-slate dark:prose-invert max-w-none', isContentEmpty && !(isStreaming && chatStore.toolExecuting) ? 'py-0 !border-0 !bg-transparent' : 'py-3']"
            @click="handleCopyClick"
          >
            <template v-if="effectiveHasCards && effectiveContentSegments">
              <template v-for="segment in effectiveContentSegments" :key="segment.key">
                <div
                  v-if="segment.type === 'text'"
                  class="prose-content"
                  v-html="renderSegmentHtml(segment.content as string)"
                />
                <TypelessCardComponent
                  v-else
                  :key="segment.key"
                  :card="segment.content as TypelessCard"
                  class="my-3 -mx-1"
                  @action="handleCardAction"
                  @select="handleCardSelect"
                />
              </template>
            </template>
            <!-- Render without typeless cards -->
            <div
              v-else-if="!isContentEmpty"
              v-html="renderedContent"
            />
            <!-- Tool detail cards (collapsible, shown when toggle is on) -->
            <div v-if="isStreaming && chatStore.toolResults.length > 0 && settingsStore.showToolDetails" class="tool-detail-cards my-2 -mx-1">
              <ToolDetailCard
                v-for="item in chatStore.toolResults"
                :key="item.id"
                :item="item"
              />
            </div>
            <!-- Tool execution indicator (inside bubble) -->
            <div v-if="isStreaming && chatStore.toolExecuting && settingsStore.showToolDetails" :class="['tool-executing-indicator', { 'mt-0': isContentEmpty }]">
              <div class="tool-pill">
                <span class="tool-dots">
                  <span /><span /><span />
                </span>
                <span class="tool-label">{{ t('tools.callingProgress') }}</span>
                <span class="tool-timer tabular-nums">{{ toolElapsedSeconds }}s</span>
                <span v-if="chatStore.toolSandboxAvailable" class="sandbox-badge" :title="t('tools.sandboxProtected')">
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                </span>
              </div>
              <div v-if="toolDisplayNames.length > 0" class="tool-names">
                <span v-for="name in toolDisplayNames" :key="name" class="tool-name-tag">{{ name }}</span>
              </div>
            </div>
            <!-- Minimal thinking indicator when tool details hidden -->
            <div v-else-if="isStreaming && chatStore.toolExecuting && !settingsStore.showToolDetails" class="tool-executing-indicator" :class="{ 'mt-0': isContentEmpty }">
              <div class="tool-pill">
                <span class="tool-dots">
                  <span /><span /><span />
                </span>
                <span class="tool-timer tabular-nums">{{ toolElapsedSeconds }}s</span>
              </div>
            </div>
            <!-- Waiting timer card (>3s with no content) -->
            <div v-if="showWaitingTimer" :class="['-mx-1', isContentEmpty ? 'my-0' : 'my-3']" class="waiting-card">
              <div class="waiting-card-inner">
                <div class="waiting-card-header">
                  <svg class="waiting-card-spinner" width="20" height="20" viewBox="0 0 24 24">
                    <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="2" opacity="0.15" />
                    <circle cx="12" cy="12" r="10" fill="none" stroke="url(#waitGrad)" stroke-width="2.5" stroke-linecap="round" stroke-dasharray="40 23" />
                    <defs>
                      <linearGradient id="waitGrad" x1="0" y1="0" x2="1" y2="1">
                        <stop offset="0%" stop-color="#818cf8" />
                        <stop offset="100%" stop-color="#c084fc" />
                      </linearGradient>
                    </defs>
                  </svg>
                  <span class="waiting-card-title">{{ t('chat.waitingThinking', 'Thinking...') }}</span>
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
        <div v-if="isStreaming && isAssistant && !chatStore.toolExecuting && !showWaitingTimer" class="streaming-indicator mt-2">
          <span class="inline-flex gap-1">
            <span class="w-2 h-2 bg-gray-700 dark:bg-gray-500 rounded-full animate-bounce" style="animation-delay: 0ms" />
            <span class="w-2 h-2 bg-gray-700 dark:bg-gray-500 rounded-full animate-bounce" style="animation-delay: 150ms" />
            <span class="w-2 h-2 bg-gray-700 dark:bg-gray-500 rounded-full animate-bounce" style="animation-delay: 300ms" />
          </span>
        </div>

        <!-- Timestamp -->
        <div
          class="timestamp text-xs text-gray-400 dark:text-gray-500 mt-1 flex items-center gap-2 flex-wrap"
          :class="{ 'justify-end': isUser }"
        >
          <span>{{ formattedTime }}</span>

          <!-- Stats for assistant messages -->
          <template v-if="!isMobile && isAssistant && metadata?.stats">
            <span class="ml-2 flex items-center gap-2">
              <span v-if="formatTokens(metadata.stats.input_tokens)" class="flex items-center gap-0.5">
                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16V4m0 0L3 8m4-4l4 4" />
                </svg>
                {{ formatTokens(metadata.stats.input_tokens) }}
              </span>
              <span v-if="formatTokens(metadata.stats.output_tokens)" class="flex items-center gap-0.5">
                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 8v12m0 0l-4-4m4 4l4-4" />
                </svg>
                {{ formatTokens(metadata.stats.output_tokens) }}
              </span>
              <span v-if="formatTTFT(metadata.stats.ttft_ms)">
                ⏱️ {{ formatTTFT(metadata.stats.ttft_ms) }}
              </span>
              <span v-if="formatSpeed(metadata.stats.tokens_per_second)">
                {{ formatSpeed(metadata.stats.tokens_per_second) }} tokens/s
              </span>
            </span>
          </template>
        </div>
      </div>

      <!-- User avatar -->
      <div
        v-if="isUser"
        class="avatar flex-shrink-0 w-6 h-6 sm:w-8 sm:h-8 rounded-full bg-gray-400 dark:bg-gray-600 flex items-center justify-center text-white text-xs sm:text-sm font-bold"
      >
        U
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
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
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
            <pre class="text-sm text-gray-100 whitespace-pre-wrap font-mono">{{ previewAttachment.content }}</pre>
          </div>
          <!-- Generic file preview -->
          <div
            v-else
            class="bg-gray-700 rounded-lg p-8 flex flex-col items-center gap-4"
          >
            <svg class="w-16 h-16 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
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
              <div class="px-4 py-3 mb-2 bg-gray-50 dark:bg-gray-800/50 rounded-xl text-xs text-gray-500 dark:text-gray-400 space-y-2">
                <div class="flex items-center justify-between">
                  <span class="text-gray-400 dark:text-gray-500">{{ formattedTimeLong }}</span>
                </div>
              </div>

              <!-- Copy action -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileCopy"
              >
                <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{ t('chat.copyMessage') }}</span>
              </button>

              <!-- TTS action (only for assistant messages) -->
              <button
                v-if="isAssistant"
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileTTS"
              >
                <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{ isSpeaking ? t('chat.stopTTS') : t('chat.playTTS') }}</span>
              </button>

              <!-- Continue / Regenerate (only for last assistant message, not while streaming) -->
              <template v-if="isLastAssistantMessage && !isStreaming">
                <button
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                  @click="emit('continue'); closeMobileActions()"
                >
                  <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <span class="text-base font-medium text-gray-900 dark:text-white">{{ t('chat.continueGenerating') }}</span>
                </button>
                <button
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                  @click="emit('regenerate'); closeMobileActions()"
                >
                  <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                  <span class="text-base font-medium text-gray-900 dark:text-white">{{ t('chat.regenerate') }}</span>
                </button>
              </template>

              <!-- Export action -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileExport"
              >
                <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{ t('chat.exportMessage') }}</span>
              </button>

              <!-- Select action (enter multi-select mode) -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handleMobileSelect"
              >
                <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">{{ t('chat.selectMessage') }}</span>
              </button>

              <!-- Delete action -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                @click="handleMobileDelete"
              >
                <svg class="w-5 h-5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
                <span class="text-base font-medium text-red-500">{{ t('common.delete') }}</span>
              </button>

              <!-- Cancel button -->
              <button
                class="w-full mt-2 px-4 py-3 rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
                @click="closeMobileActions"
              >
                <span class="text-base font-medium text-gray-900 dark:text-white">{{ t('common.cancel') }}</span>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.assistant-message {
  background: var(--chat-assistant-bg, rgba(15, 23, 42, 0.5));
  border: var(--chat-assistant-border, 1px solid rgba(148, 163, 184, 0.18));
  color: var(--chat-assistant-text, inherit);
  border-radius: 0.75rem;
  padding: 0.75rem 1rem;
  box-shadow: 0 10px 28px rgba(2, 6, 23, 0.18);
}

.assistant-message.\!bg-transparent {
  padding-top: 0;
  padding-bottom: 0;
}

.chat-user-bubble {
  background: var(--chat-user-bg, linear-gradient(135deg, #0284c7, #0891b2));
  color: var(--chat-user-text, #f8fafc);
  border: var(--chat-user-border, none);
  border-radius: 0.95rem 0.35rem 0.95rem 0.95rem;
  box-shadow: 0 10px 20px rgba(8, 145, 178, 0.24);
}

.chat-assistant-bubble {
  border-radius: 0.95rem;
}

.avatar {
  background: var(--chat-avatar-bg, linear-gradient(135deg, #0ea5e9, #14b8a6));
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.2), 0 8px 16px rgba(15, 23, 42, 0.2);
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
[data-theme="dark"] .prose :deep(.tree-structure) {
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
[data-theme="dark"] .prose :deep(.inline-code) {
  background: rgba(255, 255, 255, 0.1);
}

.prose :deep(a) {
  color: #0284c7;
}

:root.dark .prose :deep(a),
[data-theme="dark"] .prose :deep(a) {
  color: #38bdf8;
}

.prose :deep(a:hover) {
  text-decoration: underline;
}

.prose :deep(ul),
.prose :deep(ol) {
  padding-left: 1.5rem;
}

.prose :deep(blockquote) {
  border-left-width: 4px;
  padding-left: 1rem;
  font-style: italic;
  color: #6b7280;
}

:root.dark .prose :deep(blockquote),
[data-theme="dark"] .prose :deep(blockquote) {
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

/* Metadata tooltip styles */
.metadata-tooltip {
  pointer-events: none;
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
[data-theme="light"] .assistant-message :deep(.response-interrupted-indicator) {
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
[data-theme="light"] .assistant-message :deep(.error-block-indicator) {
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
[data-theme="dark"] .sandbox-badge {
  color: #4ade80;
}

:root.dark .tool-pill,
[data-theme="dark"] .tool-pill {
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

.tool-dots span:nth-child(2) { animation-delay: 0.2s; }
.tool-dots span:nth-child(3) { animation-delay: 0.4s; }

:root.dark .tool-dots span,
[data-theme="dark"] .tool-dots span {
  background: #38bdf8;
}

.tool-label {
  font-size: 0.75rem;
  font-weight: 500;
  color: #0284c7;
}

:root.dark .tool-label,
[data-theme="dark"] .tool-label {
  color: #7dd3fc;
}

.tool-timer {
  font-size: 0.6875rem;
  color: #94a3b8;
}

:root.dark .tool-timer,
[data-theme="dark"] .tool-timer {
  color: #64748b;
}

@keyframes tool-dot-pulse {
  0%, 80%, 100% { opacity: 0.3; transform: scale(0.8); }
  40% { opacity: 1; transform: scale(1); }
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
[data-theme="dark"] .tool-name-tag {
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
[data-theme="dark"] .waiting-card-inner {
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
[data-theme="dark"] .waiting-card-title {
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
[data-theme="dark"] .waiting-card-bar {
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
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes waiting-bar-slide {
  0% { transform: translateX(-100%); }
  50% { transform: translateX(150%); }
  100% { transform: translateX(-100%); }
}

@keyframes waiting-fade-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
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

:global(.chat-view.theme-style-minimal) .voice-bubble {
  box-shadow: none;
}

:global(.light .chat-view.theme-style-minimal) .voice-play-btn,
:global([data-theme="light"] .chat-view.theme-style-minimal) .voice-play-btn {
  background: rgba(15, 23, 42, 0.12);
}

:global(.light .chat-view.theme-style-minimal) .voice-bubble:hover .voice-play-btn,
:global([data-theme="light"] .chat-view.theme-style-minimal) .voice-bubble:hover .voice-play-btn {
  background: rgba(15, 23, 42, 0.2);
}

:global(.light .chat-view.theme-style-minimal) .voice-bar,
:global([data-theme="light"] .chat-view.theme-style-minimal) .voice-bar {
  background: rgba(15, 23, 42, 0.28);
}

:global(.light .chat-view.theme-style-minimal) .voice-bar-active,
:global([data-theme="light"] .chat-view.theme-style-minimal) .voice-bar-active {
  background: rgba(15, 23, 42, 0.75);
}
</style>
