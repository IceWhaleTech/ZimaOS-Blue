<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  disabled?: boolean
  streaming?: boolean
}>()

const emit = defineEmits<{
  send: [message: string]
  cancel: []
}>()

const message = ref('')
const textareaRef = ref<HTMLTextAreaElement | null>(null)

const canSend = computed(() => message.value.trim().length > 0 && !props.disabled)

function handleSend() {
  if (!canSend.value) return

  emit('send', message.value.trim())
  message.value = ''

  // Reset textarea height
  if (textareaRef.value) {
    textareaRef.value.style.height = 'auto'
  }
}

function handleCancel() {
  emit('cancel')
}

function handleKeydown(event: KeyboardEvent) {
  // Send on Enter (without Shift)
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    handleSend()
  }
}

function handleInput() {
  // Auto-resize textarea
  if (textareaRef.value) {
    textareaRef.value.style.height = 'auto'
    textareaRef.value.style.height = `${Math.min(textareaRef.value.scrollHeight, 200)}px`
  }
}
</script>

<template>
  <div class="chat-input border-t border-gray-700 bg-gray-800 p-4">
    <div class="flex items-end gap-3">
      <div class="flex-1 relative">
        <textarea
          ref="textareaRef"
          v-model="message"
          :disabled="disabled || streaming"
          placeholder="Type a message... (Enter to send, Shift+Enter for new line)"
          class="w-full bg-gray-700 text-white rounded-xl px-4 py-3 pr-12 resize-none focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
          rows="1"
          @keydown="handleKeydown"
          @input="handleInput"
        />
      </div>

      <!-- Send/Cancel button -->
      <button
        v-if="streaming"
        class="flex-shrink-0 w-12 h-12 rounded-xl bg-red-600 hover:bg-red-700 text-white flex items-center justify-center transition-colors"
        title="Cancel"
        @click="handleCancel"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-6 w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>

      <button
        v-else
        :disabled="!canSend"
        class="flex-shrink-0 w-12 h-12 rounded-xl bg-blue-600 hover:bg-blue-700 text-white flex items-center justify-center transition-colors disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-blue-600"
        title="Send"
        @click="handleSend"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-6 w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
          />
        </svg>
      </button>
    </div>

    <!-- Hint text -->
    <div class="text-xs text-gray-500 mt-2 text-center">
      Press <kbd class="px-1 py-0.5 bg-gray-700 rounded text-gray-400">Enter</kbd> to send,
      <kbd class="px-1 py-0.5 bg-gray-700 rounded text-gray-400">Shift + Enter</kbd> for new line
    </div>
  </div>
</template>

<style scoped>
textarea {
  min-height: 48px;
  max-height: 200px;
}

textarea::-webkit-scrollbar {
  width: 6px;
}

textarea::-webkit-scrollbar-track {
  background: transparent;
}

textarea::-webkit-scrollbar-thumb {
  background: #4b5563;
  border-radius: 3px;
}
</style>
