<script setup lang="ts">
import { ref, onMounted, nextTick, watch, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { useChatShortcuts } from '@/composables/useKeyboardShortcuts'
import ConversationList from '@/components/ConversationList.vue'
import ChatMessage from '@/components/ChatMessage.vue'
import ChatInput from '@/components/ChatInput.vue'

const { t } = useI18n()
const chatStore = useChatStore()
const settingsStore = useSettingsStore()

const messagesContainer = ref<HTMLElement | null>(null)
const chatInputRef = ref<InstanceType<typeof ChatInput> | null>(null)
const showSidebar = ref(false) // Default closed on mobile
const isMobile = ref(false)
const showModelSelector = ref(false)

// Check if mobile on mount and resize
function checkMobile() {
  isMobile.value = window.innerWidth < 768
  // Auto-show sidebar on desktop
  if (!isMobile.value) {
    showSidebar.value = true
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

// Close sidebar when selecting conversation on mobile
watch(
  () => chatStore.currentConversationId,
  () => {
    if (isMobile.value) {
      showSidebar.value = false
    }
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
  await chatStore.createConversation(t('chat.newConversation'))
  if (isMobile.value) {
    showSidebar.value = false
  }
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

function toggleModelSelector() {
  showModelSelector.value = !showModelSelector.value
}

// Close model selector when clicking outside
function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.model-selector-container')) {
    showModelSelector.value = false
  }
}

const currentModelDisplay = computed(() => {
  const model = settingsStore.selectedModel
  if (!model) return t('chat.selectModel')
  // Truncate long model names on mobile
  if (isMobile.value && model.length > 15) {
    return model.substring(0, 12) + '...'
  }
  return model
})

// Get translated provider name
function getProviderDisplayName(providerName: string): string {
  const key = `settings.providers.${providerName.toLowerCase()}`
  const translated = t(key)
  // If translation key doesn't exist, return original name
  return translated === key ? providerName : translated
}

onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  document.addEventListener('click', handleClickOutside)

  await Promise.all([
    chatStore.fetchConversations(),
    settingsStore.fetchProviders(),
    settingsStore.fetchTools(),
  ])
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <div class="chat-view h-full flex relative">
    <!-- Overlay for mobile sidebar -->
    <div
      v-if="showSidebar && isMobile"
      class="fixed inset-0 bg-black/60 backdrop-blur-sm z-30 md:hidden"
      @click="toggleSidebar"
    />

    <!-- Conversation sidebar -->
    <aside
      class="conversation-sidebar flex-shrink-0 border-r border-glass-border transition-transform duration-300 glass-sidebar z-40"
      :class="{
        'w-80': !isMobile,
        'w-[85vw] max-w-80 fixed left-0 top-0 h-full': isMobile,
        '-translate-x-full': !showSidebar && isMobile,
        'translate-x-0': showSidebar || !isMobile,
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
    <main class="flex-1 flex flex-col min-w-0 bg-gray-50 dark:bg-surface-base relative">
      <!-- Chat header -->
      <header class="flex items-center justify-between p-2 sm:p-4 border-b border-gray-200 dark:border-glass-border glass-header gap-2">
        <div class="flex items-center gap-2 sm:gap-3 min-w-0 flex-1">
          <!-- Sidebar toggle button -->
          <button
            class="flex-shrink-0 p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition-all duration-200 cursor-pointer"
            :class="{ 'md:hidden': !isMobile }"
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
          <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white truncate">
            {{ chatStore.currentConversation?.title || t('chat.newChat') }}
          </h2>
        </div>

        <!-- Model selector -->
        <div class="model-selector-container relative flex-shrink-0">
          <!-- Mobile: Compact button -->
          <button
            v-if="isMobile"
            class="flex items-center gap-1 px-3 py-1.5 glass-card text-gray-900 dark:text-white text-sm transition-all duration-200 cursor-pointer"
            @click.stop="toggleModelSelector"
          >
            <span class="truncate max-w-[100px]">{{ currentModelDisplay }}</span>
            <svg
              class="w-4 h-4 flex-shrink-0 transition-transform duration-200"
              :class="{ 'rotate-180': showModelSelector }"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
          </button>

          <!-- Mobile: Dropdown -->
          <div
            v-if="isMobile && showModelSelector"
            class="absolute right-0 top-full mt-2 w-64 glass-card shadow-xl z-50 p-4"
          >
            <div class="space-y-4">
              <div>
                <label class="block text-xs text-gray-500 dark:text-slate-400 mb-2 font-medium">{{ t('chat.provider') }}</label>
                <select
                  :value="settingsStore.selectedProvider"
                  class="w-full glass-input text-gray-900 dark:text-white px-3 py-2 text-sm cursor-pointer"
                  @change="settingsStore.setProvider(($event.target as HTMLSelectElement).value)"
                >
                  <option
                    v-for="provider in settingsStore.providers"
                    :key="provider.name"
                    :value="provider.name"
                    class="bg-white dark:bg-surface-elevated"
                  >
                    {{ getProviderDisplayName(provider.name) }}
                  </option>
                </select>
              </div>
              <div>
                <label class="block text-xs text-gray-500 dark:text-slate-400 mb-2 font-medium">{{ t('chat.model') }}</label>
                <select
                  :value="settingsStore.selectedModel"
                  class="w-full glass-input text-gray-900 dark:text-white px-3 py-2 text-sm cursor-pointer"
                  @change="settingsStore.setModel(($event.target as HTMLSelectElement).value)"
                >
                  <option
                    v-for="model in settingsStore.availableModels"
                    :key="model"
                    :value="model"
                    class="bg-white dark:bg-surface-elevated"
                  >
                    {{ model }}
                  </option>
                </select>
              </div>
            </div>
          </div>

          <!-- Desktop: Inline selectors -->
          <div v-else-if="!isMobile" class="flex items-center gap-2 text-sm">
            <select
              :value="settingsStore.selectedProvider"
              class="glass-input text-gray-900 dark:text-white px-3 py-1.5 cursor-pointer"
              @change="settingsStore.setProvider(($event.target as HTMLSelectElement).value)"
            >
              <option
                v-for="provider in settingsStore.providers"
                :key="provider.name"
                :value="provider.name"
                class="bg-white dark:bg-surface-elevated"
              >
                {{ getProviderDisplayName(provider.name) }}
              </option>
            </select>

            <select
              :value="settingsStore.selectedModel"
              class="glass-input text-gray-900 dark:text-white px-3 py-1.5 cursor-pointer"
              @change="settingsStore.setModel(($event.target as HTMLSelectElement).value)"
            >
              <option
                v-for="model in settingsStore.availableModels"
                :key="model"
                :value="model"
                class="bg-white dark:bg-surface-elevated"
              >
                {{ model }}
              </option>
            </select>
          </div>
        </div>
      </header>

      <!-- Messages area -->
      <div
        ref="messagesContainer"
        class="flex-1 overflow-y-auto overscroll-contain pb-32"
        @scroll="handleScroll"
      >
        <!-- Load more indicator -->
        <div
          v-if="chatStore.loadingMore"
          class="flex justify-center py-4"
        >
          <div class="flex items-center gap-2 text-gray-500 dark:text-slate-400 text-sm">
            <div class="w-4 h-4 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
            {{ t('chat.loadingOlderMessages') }}
          </div>
        </div>

        <!-- Load more button -->
        <div
          v-else-if="chatStore.hasMoreMessages && chatStore.messages.length > 0"
          class="flex justify-center py-4"
        >
          <button
            class="text-sm text-accent hover:text-accent-light px-4 py-2 transition-colors cursor-pointer"
            @click="chatStore.loadMoreMessages()"
          >
            {{ t('chat.loadOlderMessages') }}
          </button>
        </div>

        <!-- Empty state -->
        <div
          v-if="chatStore.messages.length === 0 && !chatStore.loading"
          class="h-full flex items-center justify-center p-4"
        >
          <div class="text-center text-gray-500 dark:text-slate-400 max-w-md">
            <div class="w-16 h-16 sm:w-20 sm:h-20 mx-auto mb-6 rounded-2xl bg-gradient-to-br from-accent to-cta flex items-center justify-center shadow-glow">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-8 w-8 sm:h-10 sm:w-10 text-white"
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
            <h3 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-3">{{ t('chat.startConversation') }}</h3>
            <p class="text-sm sm:text-base text-gray-500 dark:text-slate-400">{{ t('chat.startConversationDesc') }}</p>
            <div class="mt-6 glass-card p-4 inline-block">
              <p class="text-sm">{{ t('chat.currentModel') }}: <span class="text-accent font-medium">{{ settingsStore.selectedModel }}</span></p>
            </div>
            <div v-if="!isMobile" class="mt-6 text-xs text-gray-400 dark:text-slate-500">
              <p class="font-medium mb-2">{{ t('chat.keyboardShortcuts') }}:</p>
              <p class="space-x-4">
                <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">Ctrl+N</span> {{ t('chat.newChatShortcut') }}
                <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">Ctrl+/</span> {{ t('chat.focusInputShortcut') }}
                <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">Ctrl+B</span> {{ t('chat.toggleSidebarShortcut') }}
              </p>
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
        class="px-3 sm:px-4 py-3 bg-red-500/10 border-t border-red-500/30 text-red-400 text-xs sm:text-sm flex items-center justify-between gap-2"
      >
        <span class="truncate">{{ chatStore.error }}</span>
        <button
          class="text-red-400 hover:text-red-300 flex-shrink-0 px-3 py-1 rounded hover:bg-red-500/10 transition-colors cursor-pointer"
          @click="chatStore.clearError"
        >
          {{ t('chat.dismiss') }}
        </button>
      </div>

      <!-- Input area - floating at bottom -->
      <div class="absolute bottom-0 left-0 right-0 z-10">
        <ChatInput
          ref="chatInputRef"
          :disabled="chatStore.sending"
          :streaming="chatStore.streaming"
          @send="handleSend"
          @cancel="handleCancel"
        />
      </div>
    </main>
  </div>
</template>

<style scoped>
.chat-view {
  height: calc(100vh - 64px);
}

/* Safe area for mobile devices with notches */
@supports (padding-bottom: env(safe-area-inset-bottom)) {
  .chat-view {
    padding-bottom: env(safe-area-inset-bottom);
  }
}

.conversation-sidebar {
  height: 100%;
}

/* Prevent body scroll when sidebar is open on mobile */
@media (max-width: 767px) {
  .conversation-sidebar {
    padding-top: 64px; /* Account for header */
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
    background: var(--color-bg-surface);
    border-radius: 4px;
  }

  .overflow-y-auto::-webkit-scrollbar-thumb:hover {
    background: var(--color-text-muted);
  }
}

/* Select dropdown styling */
select {
  appearance: none;
  background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%2394A3B8' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e");
  background-position: right 0.5rem center;
  background-repeat: no-repeat;
  background-size: 1.5em 1.5em;
  padding-right: 2.5rem;
}
</style>
