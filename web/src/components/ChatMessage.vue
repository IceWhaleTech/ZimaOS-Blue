<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Message } from '@/api/chat'
import { renderMarkdown, copyCodeToClipboard } from '@/utils/markdown'
import { useChatStore } from '@/stores/chat'
import { useProviderPoolStore } from '@/stores/providerPool'
import { parseTypelessContent, parseTypelessContentIncremental, splitIntoSegments, hasTypelessCards, clearIncrementalState } from '@/utils/typeless'
import type { TypelessCard } from '@/types/typeless'
import TypelessCardComponent from '@/components/typeless/TypelessCard.vue'
import { voiceApi, playAudioFromBase64 } from '@/api/voice'

const { t } = useI18n()
const providerPoolStore = useProviderPoolStore()

const props = defineProps<{
  message: Message
  isStreaming?: boolean
}>()

const emit = defineEmits<{
  contextmenu: [event: MouseEvent, messageId: string]
}>()

const chatStore = useChatStore()

const isUser = computed(() => props.message.role === 'user')
const isAssistant = computed(() => props.message.role === 'assistant')
const isSelected = computed(() => chatStore.selectedMessageIds.has(props.message.id))
const isMultiSelectMode = computed(() => chatStore.isMultiSelectMode)

// Copy button state
const copyState = ref<'idle' | 'copied'>('idle')

// TTS playback state
const isSpeaking = ref(false)
const ttsError = ref<string | null>(null)
let currentAudio: HTMLAudioElement | null = null

const renderedContent = computed(() => {
  if (isUser.value) {
    return props.message.content
  }
  return renderMarkdown(props.message.content)
})

// Parse typeless cards from assistant messages
// Use incremental parsing for streaming messages, regular parsing for completed messages
const parsedContent = computed(() => {
  if (isUser.value || !hasTypelessCards(props.message.content)) {
    return null
  }
  // Use incremental parsing for streaming to avoid re-parsing entire content
  if (props.isStreaming) {
    return parseTypelessContentIncremental(props.message.content, props.message.id)
  }
  return parseTypelessContent(props.message.content)
})

// Get content segments (text and cards interleaved)
const contentSegments = computed(() => {
  if (!parsedContent.value) {
    return null
  }
  return splitIntoSegments(parsedContent.value.text, parsedContent.value.cards)
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
  clearIncrementalState(props.message.id)
  // Stop any playing audio
  if (currentAudio) {
    currentAudio.pause()
    currentAudio = null
  }
})

// TTS playback function
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

    const response = await voiceApi.synthesize(textContent)
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
            {{ message.content }}
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
              <template v-for="(segment, index) in contentSegments" :key="index">
                <div
                  v-if="segment.type === 'text'"
                  class="prose-content"
                  v-html="renderMarkdown(segment.content as string)"
                />
                <TypelessCardComponent
                  v-else
                  :card="segment.content as TypelessCard"
                  class="my-3 -mx-1"
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
