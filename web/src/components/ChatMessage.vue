<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Message } from '@/api/chat'
import { cardActionApi } from '@/api/chat'
import { renderMarkdown, copyCodeToClipboard } from '@/utils/markdown'
import { useChatStore } from '@/stores/chat'
import { useProviderPoolStore } from '@/stores/providerPool'
import { parseTypelessContent, parseTypelessContentIncremental, splitIntoSegments, hasTypelessCards, clearIncrementalState } from '@/utils/typeless'
import type { TypelessCard, TypelessCardAction, TypelessCardChoice } from '@/types/typeless'
import TypelessCardComponent from '@/components/typeless/TypelessCard.vue'
import { voiceApi, playAudioFromBase64 } from '@/api/voice'
import { speechApi } from '@/api/speech'
import ModelDownloadPrompt from '@/components/speech/ModelDownloadPrompt.vue'

const { t } = useI18n()
const providerPoolStore = useProviderPoolStore()

const props = defineProps<{
  message: Message
  isStreaming?: boolean
}>()

const emit = defineEmits<{
  contextmenu: [event: MouseEvent, messageId: string]
  cardAction: [conversationId: string, messageId: string, cardId: string, actionId: string, actionLabel?: string]
}>()

const chatStore = useChatStore()

// Card action state
const cardActionLoading = ref<string | null>(null) // cardId that is loading
const cardActionError = ref<string | null>(null)

const isUser = computed(() => props.message.role === 'user')
const isAssistant = computed(() => props.message.role === 'assistant')
const isSelected = computed(() => chatStore.selectedMessageIds.has(props.message.id))
const isMultiSelectMode = computed(() => chatStore.isMultiSelectMode)

// Check if user message has attachments
const hasAttachments = computed(() => isUser.value && props.message.attachments && props.message.attachments.length > 0)

// Copy button state
const copyState = ref<'idle' | 'copied'>('idle')

// TTS playback state
const isSpeaking = ref(false)
const ttsError = ref<string | null>(null)
let currentAudio: HTMLAudioElement | null = null

// TTS model download prompt
const showTTSDownloadPrompt = ref(false)

// Attachment preview state
const previewAttachment = ref<{ type: string; src: string; name: string; content?: string } | null>(null)

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

const renderedContent = computed(() => {
  if (isUser.value) {
    return props.message.content
  }
  // Strip first line if it's a markdown heading (used as conversation title)
  const content = stripFirstLineHeading(props.message.content)
  return renderMarkdown(content)
})

// Parse typeless cards from assistant messages
// Use incremental parsing for streaming messages, regular parsing for completed messages
const parsedContent = computed(() => {
  const content = isUser.value ? props.message.content : stripFirstLineHeading(props.message.content)
  if (isUser.value || !hasTypelessCards(content)) {
    return null
  }
  // Use incremental parsing for streaming to avoid re-parsing entire content
  // Pass conversation_id to ensure cache key uniqueness across conversations
  if (props.isStreaming) {
    return parseTypelessContentIncremental(content, props.message.id, props.message.conversation_id)
  }
  return parseTypelessContent(content)
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

// Check if message has typeless cards
const hasCards = computed(() => parsedContent.value !== null && parsedContent.value.cards.length > 0)

const formattedTime = computed(() => {
  const date = new Date(props.message.created_at)
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
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
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'k'
  }
  return String(Math.round(num))
}

function formatTTFT(ms: number | undefined): string {
  if (ms === undefined || ms === null) return ''
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

// Clean up incremental parse state when component is unmounted
onUnmounted(() => {
  clearIncrementalState(props.message.id, props.message.conversation_id)
  // Stop any playing audio
  if (currentAudio) {
    currentAudio.pause()
    currentAudio = null
  }
})

// Handle card action (button click)
async function handleCardAction(actionId: string, cardId?: string) {
  if (!cardId) return

  // Find the action label from the card
  let actionLabel: string | undefined
  const card = parsedContent.value?.cards.find(c => c.id === cardId) as TypelessCardAction | undefined
  if (card?.type === 'action') {
    const action = card.actions?.find(a => a.id === actionId)
    actionLabel = action?.label
  }

  cardActionLoading.value = cardId
  cardActionError.value = null

  try {
    await cardActionApi.submit(props.message.conversation_id, props.message.id, {
      card_id: cardId,
      action_id: actionId,
      action_label: actionLabel,
    })
    // Emit event to parent for potential UI updates
    emit('cardAction', props.message.conversation_id, props.message.id, cardId, actionId, actionLabel)
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
async function checkTTSModelReady(): Promise<boolean> {
  try {
    const res = await speechApi.getTTSStatus()
    return res.data?.ready ?? false
  } catch {
    return true // Assume ready if check fails
  }
}

async function handlePlayTTS() {
  if (isSpeaking.value) {
    // Stop current playback
    if (currentAudio) {
      currentAudio.pause()
      currentAudio = null
    }
    isSpeaking.value = false
    return
  }

  // Check if TTS model is ready
  const ready = await checkTTSModelReady()
  if (!ready) {
    showTTSDownloadPrompt.value = true
    return
  }

  ttsError.value = null
  isSpeaking.value = true

  try {
    // Get plain text content (strip markdown)
    const textContent = props.message.content
      .replace(/```[\s\S]*?```/g, '') // Remove code blocks
      .replace(/`[^`]+`/g, '') // Remove inline code
      .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1') // Convert links to text
      .replace(/[#*_~]/g, '') // Remove markdown formatting
      .trim()

    if (!textContent) {
      ttsError.value = t('chat.ttsNoContent')
      isSpeaking.value = false
      return
    }

    // Get speech speed from settings
    const speed = parseFloat(localStorage.getItem('tts-speech-speed') || '1.0')
    const provider = localStorage.getItem('tts-provider') || 'espeak-ng'
    const response = await voiceApi.synthesize(textContent, undefined, undefined, speed, provider)
    if (response.data.audio) {
      await playAudioFromBase64(response.data.audio, response.data.content_type)
    }
  } catch (error) {
    console.error('TTS error:', error)
    ttsError.value = t('chat.ttsError')
  } finally {
    isSpeaking.value = false
    currentAudio = null
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
</script>

<template>
  <div
    class="message group relative p-4 transition-colors duration-150"
    :class="{
      'bg-accent/10': isSelected,
      'cursor-pointer': isMultiSelectMode,
    }"
    @contextmenu="handleContextMenu"
    @click="handleClick"
  >
    <!-- Selection checkbox in multi-select mode (absolute left) -->
    <div
      v-if="isMultiSelectMode"
      class="absolute left-2 top-1/2 -translate-y-1/2 flex items-center z-10"
    >
      <div
        class="w-5 h-5 rounded border-2 flex items-center justify-center transition-colors"
        :class="isSelected ? 'bg-accent border-accent' : 'border-gray-400 dark:border-gray-600'"
      >
        <svg v-if="isSelected" class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
        </svg>
      </div>
    </div>

    <!-- Message content wrapper -->
    <div
      class="flex gap-3"
      :class="{
        'justify-end': isUser,
        'pl-8': isMultiSelectMode,
      }"
    >
      <!-- Avatar (AI) -->
      <div
        v-if="isAssistant"
        class="avatar flex-shrink-0 w-8 h-8 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center text-white text-sm font-bold"
      >
        AI
      </div>

      <!-- Content -->
      <div
        class="content min-w-0 max-w-[80%]"
      >
        <!-- Provider and Model info (above chat bubble for assistant) -->
        <div v-if="isAssistant && metadata && (metadata.provider || metadata.model)" class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400 mb-1">
          <!-- Cloud/Local icon -->
          <svg v-if="providerLocation === 'cloud'" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
          </svg>
          <svg v-else-if="providerLocation === 'local'" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
          <!-- Provider name -->
          <span v-if="metadata.provider">{{ metadata.provider }}</span>
          <!-- Model name -->
          <span v-if="metadata.model" class="text-gray-400 dark:text-gray-500">/</span>
          <span v-if="metadata.model">{{ metadata.model }}</span>
        </div>

        <!-- User message bubble -->
        <div
          v-if="isUser"
          class="user-message-wrapper relative"
        >
          <div class="user-message chat-user-bubble px-4 py-2 inline-block">
            <!-- Attachments display inside bubble -->
            <div v-if="hasAttachments" class="mb-2 flex flex-wrap gap-2">
              <div
                v-for="(attachment, index) in message.attachments"
                :key="index"
                class="attachment-preview rounded-lg overflow-hidden border border-gray-300 dark:border-white/20 cursor-pointer hover:opacity-90 transition-opacity bg-white/90 dark:bg-white/10"
                @click="openAttachmentPreview(attachment)"
              >
                <!-- Image attachment -->
                <img
                  v-if="attachment.type === 'image'"
                  :src="`data:${attachment.mime_type};base64,${attachment.data}`"
                  :alt="attachment.name"
                  class="max-w-[200px] max-h-[150px] object-cover"
                  :title="attachment.name"
                />
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
            <!-- Text content (hide placeholder patterns like [filename.txt], [Attachments:...]) -->
            <span v-if="message.content && !isPlaceholderContent(message.content)">{{ message.content }}</span>
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
              <svg v-if="isSpeaking" class="w-4 h-4 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
              </svg>
              <svg v-else class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
              </svg>
            </button>
          </div>

          <!-- Render with typeless cards embedded in single bubble -->
          <div
            class="assistant-message chat-assistant-bubble px-4 py-3 prose prose-slate dark:prose-invert max-w-none"
            @click="handleCopyClick"
          >
            <template v-if="hasCards && contentSegments">
              <template v-for="segment in contentSegments" :key="segment.key">
                <div
                  v-if="segment.type === 'text'"
                  class="prose-content"
                  v-html="renderMarkdown(segment.content as string)"
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
              v-else
              v-html="renderedContent"
            />
          </div>
        </div>

        <!-- Streaming indicator -->
        <div v-if="isStreaming && isAssistant" class="streaming-indicator mt-2">
          <span class="inline-flex gap-1">
            <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 0ms" />
            <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 150ms" />
            <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 300ms" />
          </span>
        </div>

        <!-- Timestamp (always visible) -->
        <div
          class="timestamp text-xs text-gray-400 dark:text-gray-500 mt-1 flex items-center gap-2 flex-wrap"
          :class="{ 'justify-end': isUser }"
        >
          <span>{{ formattedTime }}</span>

          <!-- Stats for assistant messages -->
          <template v-if="isAssistant && metadata?.stats">
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
        class="avatar flex-shrink-0 w-8 h-8 rounded-full bg-gradient-to-br from-green-500 to-teal-500 flex items-center justify-center text-white text-sm font-bold"
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
            class="bg-gray-900 rounded-lg p-4 max-w-[80vw] max-h-[80vh] overflow-auto"
          >
            <pre class="text-sm text-gray-100 whitespace-pre-wrap font-mono">{{ previewAttachment.content }}</pre>
          </div>
          <!-- Generic file preview -->
          <div
            v-else
            class="bg-gray-800 rounded-lg p-8 flex flex-col items-center gap-4"
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

    <!-- TTS Model Download Prompt -->
    <ModelDownloadPrompt
      v-model:model-visible="showTTSDownloadPrompt"
      type="tts"
      @downloaded="handlePlayTTS"
    />
  </div>
</template>

<style scoped>
.assistant-message {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 0.75rem;
  padding: 0.75rem 1rem;
}

:root.light .assistant-message,
[data-theme="light"] .assistant-message {
  background: #F8FAFC;
  border: 1px solid #E2E8F0;
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
  color: #3b82f6;
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
</style>
