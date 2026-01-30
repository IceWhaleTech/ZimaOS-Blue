<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Message } from '@/api/chat'
import { renderMarkdown, copyCodeToClipboard } from '@/utils/markdown'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()

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

const renderedContent = computed(() => {
  if (isUser.value) {
    return props.message.content
  }
  return renderMarkdown(props.message.content)
})

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
</script>

<template>
  <div
    class="message relative p-4 transition-colors duration-150"
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
        <div
          v-if="isUser"
          class="user-message chat-user-bubble px-4 py-2 inline-block"
        >
          {{ message.content }}
        </div>

        <div
          v-else
          class="assistant-message-wrapper"
        >
          <div
            class="assistant-message chat-assistant-bubble px-4 py-3 prose prose-slate dark:prose-invert max-w-none"
            @click="handleCopyClick"
            v-html="renderedContent"
          />
        </div>

        <!-- Streaming indicator -->
        <div v-if="isStreaming && isAssistant" class="streaming-indicator mt-2">
          <span class="inline-flex gap-1">
            <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 0ms" />
            <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 150ms" />
            <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 300ms" />
          </span>
        </div>

        <!-- Timestamp and Stats -->
        <div
          class="timestamp text-xs text-gray-400 dark:text-gray-500 mt-1 flex items-center gap-2 flex-wrap"
          :class="{ 'justify-end': isUser }"
        >
          <span>{{ formattedTime }}</span>
          <!-- Inline stats for assistant messages -->
          <template v-if="isAssistant && metadata?.stats && !isStreaming">
            <span class="text-gray-300 dark:text-gray-600">·</span>
            <span v-if="formatTokens(metadata.stats.input_tokens)">{{ formatTokens(metadata.stats.input_tokens) }} {{ t('chat.stats.inputTokens') }}</span>
            <span v-if="formatTokens(metadata.stats.output_tokens)">{{ formatTokens(metadata.stats.output_tokens) }} {{ t('chat.stats.outputTokens') }}</span>
            <span v-if="formatTTFT(metadata.stats.ttft_ms)">{{ formatTTFT(metadata.stats.ttft_ms) }} {{ t('chat.stats.ttft') }}</span>
            <span v-if="formatSpeed(metadata.stats.tokens_per_second)">{{ formatSpeed(metadata.stats.tokens_per_second) }}{{ t('chat.stats.speed') }}</span>
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
</style>
