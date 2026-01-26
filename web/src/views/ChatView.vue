<script setup lang="ts">
import { ref, onMounted, nextTick, watch } from 'vue'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { useChatShortcuts } from '@/composables/useKeyboardShortcuts'
import ConversationList from '@/components/ConversationList.vue'
import ChatMessage from '@/components/ChatMessage.vue'
import ChatInput from '@/components/ChatInput.vue'

const chatStore = useChatStore()
const settingsStore = useSettingsStore()

const messagesContainer = ref<HTMLElement | null>(null)
const chatInputRef = ref<InstanceType<typeof ChatInput> | null>(null)
const showSidebar = ref(true)

// Keyboard shortcuts
useChatShortcuts({
  onNewChat: () => handleCreateConversation(),
  onFocusInput: () => chatInputRef.value?.focus?.(),
  onToggleSidebar: () => toggleSidebar(),
  onCancelStream: () => chatStore.streaming && handleCancel(),
})

// Scroll to bottom when messages change
watch(
  () => chatStore.messages.length,
  async () => {
    await nextTick()
    scrollToBottom()
  }
)

// Also scroll when streaming content updates
watch(
  () => chatStore.streamingContent,
  async () => {
    await nextTick()
    scrollToBottom()
  }
)

function scrollToBottom() {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

// Handle scroll for loading more messages
function handleScroll() {
  if (!messagesContainer.value) return

  // Load more when scrolled near the top
  if (messagesContainer.value.scrollTop < 100 && chatStore.hasMoreMessages && !chatStore.loadingMore) {
    const previousHeight = messagesContainer.value.scrollHeight
    chatStore.loadMoreMessages().then(() => {
      // Maintain scroll position after loading more
      nextTick(() => {
        if (messagesContainer.value) {
          const newHeight = messagesContainer.value.scrollHeight
          messagesContainer.value.scrollTop = newHeight - previousHeight
        }
      })
    })
  }
}

async function handleSend(message: string) {
  await chatStore.sendMessage(message)
}

function handleCancel() {
  chatStore.cancelStreaming()
}

async function handleSelectConversation(id: string) {
  await chatStore.selectConversation(id)
}

async function handleCreateConversation() {
  await chatStore.createConversation()
}

async function handleDeleteConversation(id: string) {
  await chatStore.deleteConversation(id)
}

function handleSearch(query: string) {
  if (query.trim()) {
    chatStore.searchConversations(query)
  }
}

function toggleSidebar() {
  showSidebar.value = !showSidebar.value
}

onMounted(async () => {
  await Promise.all([
    chatStore.fetchConversations(),
    settingsStore.fetchProviders(),
    settingsStore.fetchTools(),
  ])
})
</script>

<template>
  <div class="chat-view h-full flex">
    <!-- Sidebar toggle button (mobile) -->
    <button
      class="fixed top-20 left-4 z-50 md:hidden p-2 bg-gray-800 rounded-lg text-white"
      @click="toggleSidebar"
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
          d="M4 6h16M4 12h16M4 18h16"
        />
      </svg>
    </button>

    <!-- Conversation sidebar -->
    <aside
      class="conversation-sidebar w-80 flex-shrink-0 border-r border-gray-700 transition-transform duration-300 bg-gray-800"
      :class="{
        '-translate-x-full absolute md:relative md:translate-x-0': !showSidebar,
        'translate-x-0': showSidebar,
      }"
    >
      <ConversationList
        :conversations="chatStore.sortedConversations"
        :current-id="chatStore.currentConversationId"
        :loading="chatStore.loading"
        @select="handleSelectConversation"
        @create="handleCreateConversation"
        @delete="handleDeleteConversation"
        @search="handleSearch"
      />
    </aside>

    <!-- Main chat area -->
    <main class="flex-1 flex flex-col min-w-0 bg-gray-900">
      <!-- Chat header -->
      <header class="flex items-center justify-between p-4 border-b border-gray-700 bg-gray-800">
        <div class="flex items-center gap-3">
          <button
            class="md:hidden p-1 text-gray-400 hover:text-white"
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
          <h2 class="text-lg font-semibold text-white truncate">
            {{ chatStore.currentConversation?.title || 'New Chat' }}
          </h2>
        </div>

        <!-- Model selector -->
        <div class="flex items-center gap-2 text-sm">
          <select
            :value="settingsStore.selectedProvider"
            class="bg-gray-700 text-white rounded px-2 py-1 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change="settingsStore.setProvider(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="provider in settingsStore.providers"
              :key="provider.name"
              :value="provider.name"
            >
              {{ provider.name }}
            </option>
          </select>

          <select
            :value="settingsStore.selectedModel"
            class="bg-gray-700 text-white rounded px-2 py-1 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change="settingsStore.setModel(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="model in settingsStore.availableModels"
              :key="model"
              :value="model"
            >
              {{ model }}
            </option>
          </select>
        </div>
      </header>

      <!-- Messages area -->
      <div
        ref="messagesContainer"
        class="flex-1 overflow-y-auto"
        @scroll="handleScroll"
      >
        <!-- Load more indicator -->
        <div
          v-if="chatStore.loadingMore"
          class="flex justify-center py-4"
        >
          <div class="flex items-center gap-2 text-gray-400 text-sm">
            <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Loading older messages...
          </div>
        </div>

        <!-- Load more button -->
        <div
          v-else-if="chatStore.hasMoreMessages && chatStore.messages.length > 0"
          class="flex justify-center py-4"
        >
          <button
            class="text-sm text-blue-400 hover:text-blue-300"
            @click="chatStore.loadMoreMessages()"
          >
            Load older messages
          </button>
        </div>

        <!-- Empty state -->
        <div
          v-if="chatStore.messages.length === 0 && !chatStore.loading"
          class="h-full flex items-center justify-center"
        >
          <div class="text-center text-gray-400 max-w-md px-4">
            <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-8 w-8 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
                />
              </svg>
            </div>
            <h3 class="text-xl font-semibold text-white mb-2">Start a conversation</h3>
            <p>Send a message to begin chatting with the AI assistant.</p>
            <div class="mt-4 text-sm">
              <p>Current model: <span class="text-blue-400">{{ settingsStore.selectedModel }}</span></p>
            </div>
            <div class="mt-6 text-xs text-gray-500">
              <p>Keyboard shortcuts:</p>
              <p class="mt-1">Ctrl+N: New chat | Ctrl+/: Focus input | Ctrl+B: Toggle sidebar</p>
            </div>
          </div>
        </div>

        <!-- Messages list -->
        <div v-else class="pb-4">
          <ChatMessage
            v-for="(message, index) in chatStore.messages"
            :key="message.id"
            :message="message"
            :is-streaming="chatStore.streaming && index === chatStore.messages.length - 1"
          />
        </div>
      </div>

      <!-- Error message -->
      <div
        v-if="chatStore.error"
        class="px-4 py-2 bg-red-900/50 border-t border-red-700 text-red-200 text-sm flex items-center justify-between"
      >
        <span>{{ chatStore.error }}</span>
        <button
          class="text-red-300 hover:text-white"
          @click="chatStore.clearError"
        >
          Dismiss
        </button>
      </div>

      <!-- Input area -->
      <ChatInput
        ref="chatInputRef"
        :disabled="chatStore.sending"
        :streaming="chatStore.streaming"
        @send="handleSend"
        @cancel="handleCancel"
      />
    </main>
  </div>
</template>

<style scoped>
.chat-view {
  height: calc(100vh - 64px);
}

.conversation-sidebar {
  height: 100%;
}

@media (max-width: 768px) {
  .conversation-sidebar {
    position: absolute;
    z-index: 40;
    height: calc(100vh - 64px);
  }
}
</style>
