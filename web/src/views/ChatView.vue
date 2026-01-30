<script setup lang="ts">
import { ref, onMounted, nextTick, watch, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useChatShortcuts } from '@/composables/useKeyboardShortcuts'
import { claudeCodeApi } from '@/api/claudecode'
import type { ClaudeCodeConfigResponse } from '@/api/claudecode'
import ConversationList from '@/components/ConversationList.vue'
import ChatMessage from '@/components/ChatMessage.vue'
import ChatInput from '@/components/ChatInput.vue'

const { t } = useI18n()
const chatStore = useChatStore()
const settingsStore = useSettingsStore()
const providerPoolStore = useProviderPoolStore()

const messagesContainer = ref<HTMLElement | null>(null)
const chatInputRef = ref<InstanceType<typeof ChatInput> | null>(null)
const showSidebar = ref(false) // Default closed on mobile
const isMobile = ref(false)
const showModelSelector = ref(false)
const claudeCodeConfig = ref<ClaudeCodeConfigResponse | null>(null)
const autoSwitchEnabled = ref(localStorage.getItem('autoSwitchProvider') === 'true')

// Check if Claude Code CLI is enabled
const isClaudeCodeEnabled = computed(() => claudeCodeConfig.value?.enabled ?? false)

// Configured providers - use Provider Pool's enabled providers
const configuredProviders = computed(() => {
  // First check Provider Pool for enabled providers
  if (providerPoolStore.enabledProviders.length > 0) {
    return providerPoolStore.enabledProviders.map(p => ({
      name: p.id,
      displayName: p.name,
      models: [] // Models are fetched separately
    }))
  }
  // Fallback to legacy settings store
  return settingsStore.providers.filter(p => {
    // Ollama doesn't need API key
    if (p.name === 'ollama') return true
    // Check if API key is configured in settings
    return !!settingsStore.apiKeys[p.name]
  })
})

// Check if any providers are configured
const hasConfiguredProviders = computed(() => configuredProviders.value.length > 0)

// Toggle auto-switch
function toggleAutoSwitch() {
  autoSwitchEnabled.value = !autoSwitchEnabled.value
  localStorage.setItem('autoSwitchProvider', String(autoSwitchEnabled.value))
}

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
function getProviderDisplayName(providerName: string | undefined): string {
  if (!providerName) return ''
  // First check if it's a Provider Pool provider with a display name
  const poolProvider = providerPoolStore.providers.find(p => p.id === providerName)
  if (poolProvider) {
    return poolProvider.name
  }
  // Fallback to i18n translation
  const key = `settings.providers.${providerName.toLowerCase()}`
  const translated = t(key)
  // If translation key doesn't exist, return original name
  return translated === key ? providerName : translated
}

// Refresh models for current provider
async function handleRefreshModels() {
  try {
    await settingsStore.refreshProviderModels()
  } catch {
    // Error is handled in the store
  }
}

// Fetch Claude Code CLI config
async function fetchClaudeCodeConfig() {
  try {
    const response = await claudeCodeApi.getConfig()
    claudeCodeConfig.value = response.data
  } catch {
    // Silently fail - CLI might not be available
    claudeCodeConfig.value = null
  }
}

onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  document.addEventListener('click', handleClickOutside)

  await Promise.all([
    chatStore.fetchConversations(),
    settingsStore.fetchProviders(),
    settingsStore.fetchTools(),
    providerPoolStore.fetchProviders(),
    fetchClaudeCodeConfig(),
  ])

  // Auto-select first enabled provider if current provider is not in the enabled list
  if (providerPoolStore.enabledProviders.length > 0) {
    const enabledIds = providerPoolStore.enabledProviders.map(p => p.id)
    if (!enabledIds.includes(settingsStore.selectedProvider)) {
      // Select the first enabled provider
      const firstEnabled = providerPoolStore.enabledProviders[0]
      if (firstEnabled) {
        settingsStore.setProvider(firstEnabled.id)
      }
    }
  }

  // Auto-select first conversation if available and none selected
  if (!chatStore.currentConversationId && chatStore.sortedConversations.length > 0) {
    await chatStore.selectConversation(chatStore.sortedConversations[0].id)
  }
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
          <!-- Powered by Claude Code CLI badge -->
          <span
            v-if="isClaudeCodeEnabled"
            class="hidden sm:inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/30 rounded-full flex-shrink-0"
            :title="t('chat.poweredByClaudeCodeDesc')"
          >
            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
            </svg>
            {{ t('chat.poweredByClaudeCode') }}
          </span>
          <!-- Enable Claude Code CLI prompt -->
          <router-link
            v-else
            to="/settings?tab=claudecode"
            class="hidden sm:inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full flex-shrink-0 transition-colors cursor-pointer"
            :title="t('chat.enableClaudeCodeDesc')"
          >
            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z"/>
            </svg>
            {{ t('chat.enableClaudeCode') }}
          </router-link>
        </div>

        <!-- Model selector -->
        <div class="model-selector-container relative flex-shrink-0">
          <!-- No configured providers: Show add provider prompt -->
          <router-link
            v-if="!hasConfiguredProviders"
            to="/settings"
            class="flex items-center gap-2 px-3 py-1.5 glass-card text-amber-600 dark:text-amber-400 text-sm hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-all duration-200"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {{ t('chat.addProvider') }}
          </router-link>

          <!-- Has configured providers -->
          <template v-else>
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
                      v-for="provider in configuredProviders"
                      :key="provider.name"
                      :value="provider.name"
                      class="bg-white dark:bg-surface-elevated"
                    >
                      {{ getProviderDisplayName(provider.name) }}
                    </option>
                  </select>
                </div>
                <div>
                  <div class="flex items-center justify-between mb-2">
                    <label class="text-xs text-gray-500 dark:text-slate-400 font-medium">{{ t('chat.model') }}</label>
                    <button
                      class="text-xs text-accent hover:text-accent-light transition-colors cursor-pointer flex items-center gap-1"
                      :disabled="settingsStore.refreshing"
                      @click="handleRefreshModels"
                    >
                      <svg
                        class="w-3 h-3"
                        :class="{ 'animate-spin': settingsStore.refreshing }"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                      </svg>
                      {{ t('chat.refreshModels') }}
                    </button>
                  </div>
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
                <!-- Auto-switch checkbox (mobile) -->
                <div class="flex items-center gap-2 pt-2 border-t border-glass-border">
                  <input
                    type="checkbox"
                    :checked="autoSwitchEnabled"
                    class="w-4 h-4 rounded border-gray-300 dark:border-slate-600 text-accent focus:ring-accent cursor-pointer"
                    @change="toggleAutoSwitch"
                  />
                  <label class="text-xs text-gray-500 dark:text-slate-400 font-medium cursor-pointer" @click="toggleAutoSwitch">{{ t('chat.autoSwitch') }}</label>
                </div>
              </div>
            </div>

            <!-- Desktop: Inline selectors -->
            <div v-else-if="!isMobile" class="flex items-center gap-2 text-sm">
              <!-- Auto-switch checkbox at front -->
              <label
                class="flex items-center gap-1.5 cursor-pointer"
                :title="t('chat.autoSwitchDesc')"
              >
                <input
                  type="checkbox"
                  :checked="autoSwitchEnabled"
                  class="w-3.5 h-3.5 rounded border-gray-300 dark:border-slate-600 text-accent focus:ring-accent cursor-pointer"
                  @change="toggleAutoSwitch"
                />
                <span class="text-xs text-gray-500 dark:text-slate-400">{{ t('chat.auto') }}</span>
              </label>

              <select
                :value="settingsStore.selectedProvider"
                class="glass-input text-gray-900 dark:text-white px-3 py-1.5 cursor-pointer"
                @change="settingsStore.setProvider(($event.target as HTMLSelectElement).value)"
              >
                <option
                  v-for="provider in configuredProviders"
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

            <!-- Refresh button for desktop -->
            <button
              class="p-1.5 text-gray-500 dark:text-slate-400 hover:text-accent transition-colors cursor-pointer"
              :disabled="settingsStore.refreshing"
              :title="t('chat.refreshModels')"
              @click="handleRefreshModels"
            >
              <svg
                class="w-4 h-4"
                :class="{ 'animate-spin': settingsStore.refreshing }"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
          </div>
          </template>
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

      <!-- Security blocked warning -->
      <div
        v-if="chatStore.securityBlocked"
        class="px-3 sm:px-4 py-3 bg-yellow-500/10 border-t border-yellow-500/30 text-yellow-400 text-xs sm:text-sm flex items-center justify-between gap-2"
      >
        <div class="flex items-center gap-2">
          <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <span class="truncate">{{ chatStore.securityBlocked.message }}</span>
          <span class="text-yellow-500/70 text-xs">({{ t('chat.threatLevel') }}: {{ chatStore.securityBlocked.threatLevel }})</span>
        </div>
        <button
          class="text-yellow-400 hover:text-yellow-300 flex-shrink-0 px-3 py-1 rounded hover:bg-yellow-500/10 transition-colors cursor-pointer"
          @click="chatStore.clearSecurityBlocked"
        >
          {{ t('chat.dismissWarning') }}
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
