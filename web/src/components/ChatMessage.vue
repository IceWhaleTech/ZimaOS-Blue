<script setup lang="ts">
import { computed } from 'vue'
import type { Message } from '@/api/chat'
import { renderMarkdown, copyCodeToClipboard } from '@/utils/markdown'

const props = defineProps<{
  message: Message
  isStreaming?: boolean
}>()

const isUser = computed(() => props.message.role === 'user')
const isAssistant = computed(() => props.message.role === 'assistant')

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
</script>

<template>
  <div
    class="message flex gap-3 p-4"
    :class="{
      'bg-gray-800/50': isAssistant,
      'justify-end': isUser,
    }"
  >
    <!-- Avatar -->
    <div
      v-if="isAssistant"
      class="avatar flex-shrink-0 w-8 h-8 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center text-white text-sm font-bold"
    >
      AI
    </div>

    <!-- Content -->
    <div
      class="content max-w-[80%]"
      :class="{
        'order-first': isUser,
      }"
    >
      <div
        v-if="isUser"
        class="user-message bg-blue-600 text-white rounded-2xl rounded-tr-sm px-4 py-2"
      >
        {{ message.content }}
      </div>

      <div
        v-else
        class="assistant-message prose prose-invert max-w-none"
        @click="handleCopyClick"
        v-html="renderedContent"
      />

      <!-- Streaming indicator -->
      <div v-if="isStreaming && isAssistant" class="streaming-indicator mt-2">
        <span class="inline-flex gap-1">
          <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 0ms" />
          <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 150ms" />
          <span class="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style="animation-delay: 300ms" />
        </span>
      </div>

      <!-- Timestamp -->
      <div
        class="timestamp text-xs text-gray-500 mt-1"
        :class="{ 'text-right': isUser }"
      >
        {{ formattedTime }}
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
</template>

<style scoped>
.prose :deep(pre) {
  margin: 0;
  padding: 0;
  background: transparent;
}

.prose :deep(.code-block) {
  margin: 0.75rem 0;
}

.prose :deep(.inline-code) {
  background: rgba(255, 255, 255, 0.1);
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  font-size: 0.875em;
}

.prose :deep(a) {
  color: #60a5fa;
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
  color: #9ca3af;
}
</style>
