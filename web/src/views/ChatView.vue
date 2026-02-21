<script setup lang="ts">
import { ref, onMounted, nextTick, watch, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useChatShortcuts } from '@/composables/useKeyboardShortcuts'
import { claudeCodeApi } from '@/api/claudecode'
import type { ClaudeCodeConfigResponse } from '@/api/claudecode'
import { authFetch } from '@/api/client'
import ConversationList from '@/components/ConversationList.vue'
import ChatMessage from '@/components/ChatMessage.vue'
import ChatInput from '@/components/ChatInput.vue'
import type { FileAttachment } from '@/components/ChatInput.vue'
import PresetQuestions from '@/components/onboarding/PresetQuestions.vue'
import VirtualScroll from '@/components/VirtualScroll.vue'
import TalkMode from '@/components/chat/TalkMode.vue'
import { componentPool } from '@/utils/componentPool'
import { clearConversationIncrementalStates } from '@/utils/typeless'
import { THEME_STYLES, type ThemeStyle } from '@/stores/settings'
import { formatTokens } from '@/utils/format'

const { t } = useI18n()
const router = useRouter()
const chatStore = useChatStore()
const settingsStore = useSettingsStore()
const providerPoolStore = useProviderPoolStore()

// Trial quota animation state
const tokenAnimating = ref(false)
const previousTokens = ref<number | null>(null)

// Theme style class for chat interface
const themeStyleClass = computed(() => `theme-style-${settingsStore.themeStyle}`)

const messagesContainer = ref<HTMLElement | null>(null)
const virtualScrollRef = ref<InstanceType<typeof VirtualScroll> | null>(null)
const chatInputRef = ref<InstanceType<typeof ChatInput> | null>(null)
const showSidebar = ref(false) // Default closed on mobile
const isMobile = ref(false)
const isNarrowScreen = ref(false) // PC narrow screen (<768px)
const isCompact = computed(() => isNarrowScreen.value) // Compact mode = narrow screen
const isMac = computed(() => navigator.platform.toUpperCase().indexOf('MAC') >= 0)
const pageStack = ref<string[]>([]) // Mobile page stack: conversation IDs
const showListPage = computed(() => isMobile.value && pageStack.value.length === 0)
const showRoutingMenu = ref(false)
const routingMenuPosition = ref({ x: 0, y: 0 })
const routingButtonRef = ref<HTMLElement | null>(null)
const claudeCodeConfig = ref<ClaudeCodeConfigResponse | null>(null)

// Theme style selector state
const showStyleSelector = ref(false)
const styleButtonRef = ref<HTMLElement | null>(null)
const styleSelectorPosition = ref({ x: 0, y: 0 })

// Talk mode state
const showTalkMode = ref(false)

// Virtual scroll threshold - use virtual scroll when message count exceeds this
const VIRTUAL_SCROLL_THRESHOLD = 50

// Whether to use virtual scrolling
const useVirtualScroll = computed(() => chatStore.messages.length > VIRTUAL_SCROLL_THRESHOLD)

// Context menu state
const showContextMenu = ref(false)
const contextMenuPosition = ref({ x: 0, y: 0 })
const contextMenuMessageId = ref<string | null>(null)

// Check if Claude Code CLI is enabled
const isClaudeCodeEnabled = computed(() => claudeCodeConfig.value?.enabled ?? false)

// Provider status computed properties
const hasConfiguredProviders = computed(() => providerPoolStore.enabledProviders.length > 0)
const hasActiveProviders = computed(() => providerPoolStore.activeProviders.length > 0)
const allProvidersFailed = computed(() =>
  hasConfiguredProviders.value && !hasActiveProviders.value &&
  providerPoolStore.enabledProviders.every(p => p.status === 'error')
)

// Provider status indicator
const providerStatus = computed(() => {
  if (!hasConfiguredProviders.value) {
    return { status: 'none', color: 'gray', message: t('chat.noProviderConfigured') }
  }
  if (allProvidersFailed.value) {
    return { status: 'error', color: 'red', message: t('chat.allProvidersFailed') }
  }
  if (hasActiveProviders.value) {
    return { status: 'active', color: 'green', message: t('chat.providerActive') }
  }
  // Some providers enabled but not yet checked
  return { status: 'pending', color: 'yellow', message: t('chat.providerPending') }
})

// Routing mode display info
const routingModeInfo = computed(() => {
  const mode = providerPoolStore.routingMode
  const cloudCount = providerPoolStore.cloudProviders.filter(p => p.status === 'active').length
  const localCount = providerPoolStore.localProviders.filter(p => p.status === 'active').length

  if (mode === 'cloud') {
    return {
      icon: 'cloud',
      label: t('chat.routingMode.cloud'),
      count: cloudCount,
      color: 'gray'
    }
  } else if (mode === 'local') {
    return {
      icon: 'local',
      label: t('chat.routingMode.local'),
      count: localCount,
      color: 'green'
    }
  } else {
    return {
      icon: 'auto',
      label: t('chat.routingMode.auto'),
      count: cloudCount + localCount,
      color: 'accent'
    }
  }
})

// Available counts for routing menu
const cloudActiveCount = computed(() =>
  providerPoolStore.cloudProviders.filter(p => p.status === 'active').length
)
const localActiveCount = computed(() =>
  providerPoolStore.localProviders.filter(p => p.status === 'active').length
)

// Check if mobile device (UA detection)
function checkMobile() {
  const ua = navigator.userAgent.toLowerCase()
  const isMobileUA = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini/i.test(ua)
  isMobile.value = isMobileUA
  isNarrowScreen.value = window.innerWidth < 768
  // Auto-show sidebar on desktop wide screen, hide on narrow
  if (!isMobile.value) {
    showSidebar.value = !isNarrowScreen.value
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
    if (isUserNearBottom.value) {
      scrollToBottom()
    }
  }
)

// Also scroll when streaming content updates
watch(
  () => chatStore.streamingContent,
  async () => {
    await nextTick()
    if (isUserNearBottom.value) {
      scrollToBottom()
    }
  }
)

// Close sidebar when selecting conversation on mobile
// Also clear incremental parse states for the previous conversation
watch(
  () => chatStore.currentConversationId,
  (newId, oldId) => {
    if (isMobile.value) {
      showSidebar.value = false
    }
    // Clear incremental parse states for the old conversation to free memory
    if (oldId && oldId !== newId) {
      clearConversationIncrementalStates(oldId)
    }
  }
)

// Watch for trial quota token changes and trigger animation
watch(
  () => providerPoolStore.trialQuota?.tokens_remaining,
  (newTokens, oldTokens) => {
    if (newTokens !== undefined && oldTokens !== undefined && newTokens !== oldTokens) {
      // Trigger animation when tokens change
      tokenAnimating.value = true
      previousTokens.value = oldTokens
      setTimeout(() => {
        tokenAnimating.value = false
        previousTokens.value = null
      }, 600)
    }
  }
)

// Track whether user is near the bottom of the chat (for auto-scroll during streaming)
const isUserNearBottom = ref(true)
const NEAR_BOTTOM_THRESHOLD = 80 // px from bottom to consider "at bottom"

function checkIfNearBottom() {
  if (useVirtualScroll.value && virtualScrollRef.value) {
    // For virtual scroll, delegate to its container
    const container = (virtualScrollRef.value as any).$el?.querySelector?.('.overflow-y-auto') ?? (virtualScrollRef.value as any).containerRef
    if (container) {
      const { scrollTop, scrollHeight, clientHeight } = container
      return scrollHeight - scrollTop - clientHeight < NEAR_BOTTOM_THRESHOLD
    }
    return true
  } else if (messagesContainer.value) {
    const { scrollTop, scrollHeight, clientHeight } = messagesContainer.value
    return scrollHeight - scrollTop - clientHeight < NEAR_BOTTOM_THRESHOLD
  }
  return true
}

function scrollToBottom() {
  if (useVirtualScroll.value && virtualScrollRef.value) {
    virtualScrollRef.value.scrollToBottom('smooth')
  } else if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
  isUserNearBottom.value = true
}

// Handle scroll for loading more messages
function handleScroll() {
  // Update near-bottom tracking
  isUserNearBottom.value = checkIfNearBottom()

  // Skip for virtual scroll - it handles its own scrolling
  if (useVirtualScroll.value) return

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

// Handle virtual scroll visible range change
function handleVisibleRangeChange(start: number, _end: number) {
  // Load more when scrolled near the top in virtual scroll mode
  if (start < 5 && chatStore.hasMoreMessages && !chatStore.loadingMore) {
    chatStore.loadMoreMessages()
  }
}

async function handleSend(message: string, attachments?: FileAttachment[]) {
  await chatStore.sendMessage(message, attachments)
}

// Handle voice transcript from TalkMode - auto send to AI
async function handleVoiceTranscript(text: string) {
  if (text.trim()) {
    await chatStore.sendMessage(text)
  }
}

function handleCancel() {
  chatStore.cancelStreaming()
}

function handleContinue() {
  chatStore.continueMessage()
}

function handleRegenerate() {
  chatStore.regenerateMessage()
}

async function handleSelectConversation(id: string) {
  if (isMobile.value) {
    pageStack.value.push(id)
    // Set query param to trigger AppHeader hide
    await router.push({ query: { conversationId: id } })
  }
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

async function handlePinConversation(id: string) {
  await chatStore.pinConversation(id)
}

async function handleUnpinConversation(id: string) {
  await chatStore.unpinConversation(id)
}

function handleSearch(query: string) {
  chatStore.searchConversations(query)
}

function toggleSidebar() {
  if (isMobile.value && pageStack.value.length > 0) {
    // Mobile: return to list page
    pageStack.value = []
    chatStore.currentConversationId = null
    // Clear query param to show AppHeader
    router.push({ query: {} })
  } else {
    showSidebar.value = !showSidebar.value
  }
}

function toggleRoutingMenu() {
  // On mobile, no need to calculate position (bottom sheet)
  if (!showRoutingMenu.value && !isMobile.value && routingButtonRef.value) {
    const rect = routingButtonRef.value.getBoundingClientRect()
    routingMenuPosition.value = {
      x: rect.right,
      y: rect.bottom + 8
    }
  }
  showRoutingMenu.value = !showRoutingMenu.value
}

function selectRoutingMode(mode: 'auto' | 'cloud' | 'local') {
  providerPoolStore.setRoutingMode(mode)
  showRoutingMenu.value = false
}

// Theme style selector functions
function toggleStyleSelector() {
  // On mobile, no need to calculate position (bottom sheet)
  if (!showStyleSelector.value && !isMobile.value && styleButtonRef.value) {
    const rect = styleButtonRef.value.getBoundingClientRect()
    styleSelectorPosition.value = {
      x: rect.right,
      y: rect.bottom + 8
    }
  }
  showStyleSelector.value = !showStyleSelector.value
}

function selectThemeStyle(style: ThemeStyle) {
  settingsStore.setThemeStyle(style)
  showStyleSelector.value = false
}

// Close routing menu when clicking outside
function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement

  // On mobile, bottom sheets handle their own click-outside via backdrop
  if (isMobile.value) {
    return
  }

  if (!target.closest('.routing-menu-container')) {
    showRoutingMenu.value = false
  }
  // Close style selector when clicking outside
  if (!target.closest('.style-selector-container')) {
    showStyleSelector.value = false
  }
  // Close context menu when clicking outside
  if (!target.closest('.context-menu')) {
    showContextMenu.value = false
  }
}

// Context menu handlers
function handleMessageContextMenu(event: MouseEvent, messageId: string) {
  event.preventDefault()

  // On mobile, only respond to long press (synthetic 'longpress' event)
  // Ignore native contextmenu events on mobile
  if (isMobile.value && event.type === 'contextmenu') {
    return
  }

  contextMenuMessageId.value = messageId
  contextMenuPosition.value = { x: event.clientX, y: event.clientY }
  showContextMenu.value = true
}

// Mobile long press handlers
const longPressTimer = ref<number | null>(null)
const longPressMessageId = ref<string | null>(null)

function handleTouchStart(event: TouchEvent, messageId: string) {
  longPressMessageId.value = messageId
  longPressTimer.value = window.setTimeout(() => {
    // Trigger context menu on long press
    const touch = event.touches[0]
    if (touch) {
      // Create a synthetic MouseEvent for the context menu
      // Mark it as from touch so handleMessageContextMenu can identify it
      const syntheticEvent = new MouseEvent('longpress', {
        clientX: touch.clientX,
        clientY: touch.clientY,
        bubbles: true,
        cancelable: true
      })
      handleMessageContextMenu(syntheticEvent, messageId)
    }
  }, 500) // 500ms long press
}

function handleTouchEnd() {
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
    longPressTimer.value = null
  }
  longPressMessageId.value = null
}

function handleSelectMessage() {
  if (contextMenuMessageId.value) {
    chatStore.enterMultiSelectMode(contextMenuMessageId.value)
  }
  showContextMenu.value = false
}

async function handleDeleteSelectedMessages() {
  if (chatStore.selectedMessageIds.size === 0) return

  try {
    await chatStore.deleteSelectedMessages()
  } catch {
    // Error is handled in store
  }
}

function handleCancelSelection() {
  chatStore.exitMultiSelectMode()
}

// Handle preset question selection - directly send the message with optional attachments
async function handlePresetQuestionSelect(text: string, attachments?: FileAttachment[]) {
  await handleSend(text, attachments)
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

  // Preload common card components for better UX
  componentPool.preload(['progress', 'chart', 'gallery', 'link', 'file'])

  // Initialize speech services lazily (TTS/STT)
  authFetch('/api/v1/speech/init', { method: 'POST' }).catch(() => {})

  await Promise.all([
    chatStore.fetchConversations(),
    settingsStore.fetchProviders(),
    settingsStore.fetchTools(),
    providerPoolStore.fetchProviders(),
    providerPoolStore.fetchRoutingMode(),
    fetchClaudeCodeConfig(),
  ])

  // Auto-select first conversation if available and none selected (desktop only)
  if (!isMobile.value && !chatStore.currentConversationId && chatStore.sortedConversations.length > 0 && chatStore.sortedConversations[0]) {
    await chatStore.selectConversation(chatStore.sortedConversations[0].id)
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  document.removeEventListener('click', handleClickOutside)
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
  }
})
</script>

<template>
  <!-- Desktop View -->
  <div v-if="!isMobile" class="chat-view flex relative" :class="themeStyleClass">
    <!-- Overlay for narrow screen sidebar -->
    <div
      v-if="isNarrowScreen && showSidebar"
      class="fixed inset-0 bg-black/60 backdrop-blur-sm z-30"
      @click="toggleSidebar"
    />

    <!-- Desktop: Sidebar (always visible on wide screen, collapsible on narrow) -->
    <aside
      class="conversation-sidebar flex-shrink-0 border-r border-glass-border transition-transform duration-300 glass-sidebar z-40 w-80"
      :class="{
        'fixed left-0 top-0 h-full': isNarrowScreen,
        '-translate-x-full': isNarrowScreen && !showSidebar
      }"
    >
      <ConversationList
        :conversations="chatStore.sortedConversations"
        :current-id="chatStore.currentConversationId"
        :loading="chatStore.loading"
        :searching="chatStore.searching"
        @select="handleSelectConversation"
        @create="handleCreateConversation"
        @delete="handleDeleteConversation"
        @search="handleSearch"
        @pin="handlePinConversation"
        @unpin="handleUnpinConversation"
      />
    </aside>

    <!-- Desktop: Main chat area -->
    <main
      class="flex-1 flex flex-col min-w-0 relative"
      @dragover.prevent="chatInputRef?.handleDragOver($event)"
      @dragleave="chatInputRef?.handleDragLeave()"
      @drop.prevent="chatInputRef?.handleDrop($event)"
    >
      <!-- Chat header -->
      <header class="flex items-center justify-between p-2 sm:p-4 border-b border-gray-200 dark:border-glass-border glass-header gap-2">
        <div class="flex items-center gap-2 sm:gap-3 min-w-0 flex-1">
          <!-- Back/Sidebar toggle button -->
          <button
            class="flex-shrink-0 p-2 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition-all duration-200 cursor-pointer"
            :class="{ 'md:hidden': !isMobile }"
            @click="toggleSidebar"
            :title="showSidebar ? '关闭会话列表' : '打开会话列表'"
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
                d="M15 19l-7-7 7-7"
              />
            </svg>
          </button>
          <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white truncate">
            {{ chatStore.currentConversation?.title || t('chat.newChat') }}
          </h2>
          <!-- Powered by Claude Code CLI badge -->
          <span
            v-if="isClaudeCodeEnabled"
            class="hidden sm:inline-flex items-center gap-1 text-xs font-medium text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/30 rounded-full flex-shrink-0"
            :class="isCompact || isMobile ? 'px-1.5 py-0.5' : 'px-2 py-0.5'"
            :title="t('chat.poweredByClaudeCodeDesc', { name: 'Claude Code CLI' })"
          >
            <svg :class="isCompact || isMobile ? 'w-2.5 h-2.5' : 'w-3 h-3'" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
            </svg>
            <span v-if="!isCompact && !isMobile">{{ t('chat.poweredByClaudeCode', { name: 'Claude Code CLI' }) }}</span>
          </span>
          <!-- Enable Claude Code CLI prompt -->
          <router-link
            v-else
            to="/settings?tab=llm#claude-code-settings"
            class="hidden sm:inline-flex items-center gap-1 text-xs font-medium text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full flex-shrink-0 transition-colors cursor-pointer"
            :class="isCompact || isMobile ? 'px-1.5 py-0.5' : 'px-2 py-0.5'"
            :title="t('chat.enableClaudeCodeDesc', { name: 'Claude Code CLI' })"
          >
            <svg :class="isCompact || isMobile ? 'w-2.5 h-2.5' : 'w-3 h-3'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z"/>
            </svg>
            <span v-if="!isCompact && !isMobile">{{ t('chat.enableClaudeCodePrompt', { name: 'Claude Code CLI' }) }}</span>
          </router-link>
        </div>

        <!-- Routing Mode Switch & Provider Status -->
        <div class="flex items-center gap-2 flex-shrink-0">
          <!-- Routing Mode Dropdown (replaces both the three-state buttons and status indicator) -->
          <div class="routing-menu-container relative">
            <!-- No providers configured - link to settings -->
            <router-link
              v-if="providerStatus.status === 'none'"
              to="/settings?tab=llm"
              class="flex items-center gap-2 px-3 py-1.5 glass-card text-sm transition-all duration-200 hover:bg-white/10 text-gray-500 dark:text-gray-400"
              :title="providerStatus.message"
            >
              <span class="w-2 h-2 rounded-full bg-gray-400" />
              <span v-if="!isCompact && !isMobile" class="hidden sm:inline text-xs">{{ t('chat.addProvider') }}</span>
            </router-link>

            <!-- Has providers - show routing menu trigger -->
            <button
              v-else
              ref="routingButtonRef"
              class="flex items-center gap-2 px-3 py-1.5 glass-card text-sm transition-all duration-200 hover:bg-white/10 cursor-pointer"
              :class="{
                'text-red-500 dark:text-red-400': providerStatus.status === 'error',
                'text-green-500 dark:text-green-400': providerStatus.status === 'active' && routingModeInfo.color === 'green',
                'text-gray-900 dark:text-gray-300': providerStatus.status === 'active' && (routingModeInfo.color === 'gray' || routingModeInfo.color === 'accent'),
                'text-yellow-500 dark:text-yellow-400': providerStatus.status === 'pending',
              }"
              :title="providerStatus.message"
              @click.stop="toggleRoutingMenu"
            >
              <!-- Status light -->
              <span
                class="w-2 h-2 rounded-full"
                :class="{
                  'bg-red-500 animate-pulse': providerStatus.status === 'error',
                  'bg-green-500': providerStatus.status === 'active' && routingModeInfo.color === 'green',
                  'bg-gray-700 dark:bg-gray-500': providerStatus.status === 'active' && (routingModeInfo.color === 'gray' || routingModeInfo.color === 'accent'),
                  'bg-yellow-500 animate-pulse': providerStatus.status === 'pending',
                }"
              />
              <!-- Mode icon and label -->
              <span class="hidden sm:flex items-center gap-1 text-xs">
                <!-- Cloud icon -->
                <svg v-if="routingModeInfo.icon === 'cloud'" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
                </svg>
                <!-- Local icon -->
                <svg v-else-if="routingModeInfo.icon === 'local'" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
                <!-- Auto icon -->
                <svg v-else class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                <span v-if="!isCompact && !isMobile">{{ routingModeInfo.label }}</span>
                <span v-if="!isCompact && !isMobile && routingModeInfo.count > 0" class="opacity-60">({{ routingModeInfo.count }})</span>
              </span>
              <!-- Dropdown arrow -->
              <svg class="w-3 h-3 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
          </div>

          <!-- Theme style selector -->
          <div class="style-selector-container relative">
            <button
              ref="styleButtonRef"
              class="p-2 rounded-lg text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white transition-colors"
              :title="t('theme.styles.title')"
              @click.stop="toggleStyleSelector"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
              </svg>
            </button>
          </div>
        </div>
      </header>

      <!-- Trial Quota Banner (only show when not exhausted and user hasn't configured their own providers) -->
      <Transition name="slide-fade">
        <div
          v-if="providerPoolStore.trialQuota && providerPoolStore.trialProviders?.length > 0 && !providerPoolStore.trialQuota.exhausted && !providerPoolStore.hasUserConfiguredProviders"
          class="px-4 py-2 flex items-center justify-between text-sm border-b bg-gray-100 dark:bg-gray-700/20 border-gray-200 dark:border-gray-700 text-gray-700 dark:text-white"
          :class="{ 'trial-quota-pulse': tokenAnimating }"
        >
          <div class="flex items-center gap-2">
            <span :class="{ 'animate-bounce': tokenAnimating }">🎁</span>
            <span
              class="tabular-nums transition-all duration-300 font-medium"
              :class="{ 'token-change-animation': tokenAnimating }"
              :title="providerPoolStore.trialQuota.tokens_remaining.toLocaleString() + ' tokens'"
            >{{ t('chat.trialQuota.remaining', { tokens: formatTokens(providerPoolStore.trialQuota.tokens_remaining) }) }}</span>
          </div>
          <div class="flex items-center gap-2">
            <div class="relative w-20 h-4 bg-gray-300 dark:bg-gray-700 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500 ease-out bg-gray-700 dark:bg-gray-400"
                :style="{ width: `${Math.max(3, Math.min(100, (providerPoolStore.trialQuota.tokens_remaining / providerPoolStore.trialQuota.token_limit) * 100))}%` }"
              ></div>
              <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-white drop-shadow-sm">
                {{ Math.round((providerPoolStore.trialQuota.tokens_remaining / providerPoolStore.trialQuota.token_limit) * 100) }}%
              </span>
            </div>
            <router-link
              to="/settings?tab=llm"
              class="text-xs underline hover:no-underline"
            >
              {{ t('chat.trialQuota.configure') }}
            </router-link>
          </div>
        </div>
      </Transition>

      <!-- Messages area -->
      <div
        ref="messagesContainer"
        class="flex-1 overflow-y-auto overscroll-contain pb-32 chat-messages-area bg-surface-base"
        @scroll="handleScroll"
      >
        <!-- Load more indicator -->
        <div
          v-if="chatStore.loadingMore"
          class="flex justify-center py-4"
        >
          <div class="flex items-center gap-2 text-gray-500 dark:text-slate-400 text-sm">
            <div class="w-4 h-4 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full animate-spin"></div>
            {{ t('chat.loadingOlderMessages') }}
          </div>
        </div>

        <!-- Load more button -->
        <div
          v-else-if="chatStore.hasMoreMessages && chatStore.messages.length > 0"
          class="flex justify-center py-4"
        >
          <button
            class="text-sm text-gray-900 dark:text-gray-300 hover:text-gray-700 dark:hover:text-gray-200 px-4 py-2 transition-colors cursor-pointer"
            @click="chatStore.loadMoreMessages()"
          >
            {{ t('chat.loadOlderMessages') }}
          </button>
        </div>

        <!-- Empty state -->
        <div
          v-if="chatStore.messages.length === 0 && !chatStore.loading"
          class="h-full flex flex-col items-center justify-center p-4"
        >
          <div class="text-center text-gray-500 dark:text-slate-400 max-w-md mb-8">
            <img src="/logo.svg" alt="Logo" class="w-12 h-12 sm:w-14 sm:h-14 mx-auto mb-4 opacity-80 dark:opacity-60" />
            <h3 class="text-lg sm:text-xl font-semibold text-gray-900 dark:text-white mb-1">{{ t('chat.startConversation') }}</h3>
            <p class="text-sm text-gray-500 dark:text-slate-400">{{ t('chat.startConversationDesc') }}</p>
          </div>

          <!-- Preset Questions -->
          <PresetQuestions @select="handlePresetQuestionSelect" />

          <div v-if="!isMobile" class="mt-8 text-xs text-gray-400 dark:text-slate-500 text-center">
            <p class="font-medium mb-2">{{ t('chat.keyboardShortcuts') }}:</p>
            <p class="space-x-4">
              <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">{{ isMac ? '⌘N' : 'Alt+N' }}</span> {{ t('chat.newChatShortcut') }}
              <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">/</span> {{ t('chat.focusInputShortcut') }}
              <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">{{ isMac ? '⌘B' : 'Alt+B' }}</span> {{ t('chat.toggleSidebarShortcut') }}
            </p>
            <p class="mt-3 text-gray-400 dark:text-slate-500">
              <span class="px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">Enter</span> {{ t('chat.sendMessage') }}
              <span class="ml-4 px-2 py-1 glass rounded text-gray-600 dark:text-slate-300">Shift+Enter</span> {{ t('chat.newLine') }}
            </p>
            <p class="mt-2 text-gray-400 dark:text-slate-500">
              {{ t('chat.dragDropHint') }}
            </p>
          </div>
        </div>

        <!-- Messages list - Virtual scroll for large lists -->
        <template v-if="chatStore.messages.length > 0">
          <!-- Use virtual scroll for large message lists -->
          <VirtualScroll
            v-if="useVirtualScroll"
            ref="virtualScrollRef"
            :item-count="chatStore.messages.length"
            :estimated-item-height="120"
            :overscan="5"
            class="h-full pb-4"
            @visible-range-change="handleVisibleRangeChange"
          >
            <template #default="{ index }">
              <ChatMessage
                v-if="chatStore.messages[index]"
                :key="`${chatStore.messages[index]!.conversation_id}-${chatStore.messages[index]!.id}`"
                :message="chatStore.messages[index]!"
                :is-streaming="chatStore.streaming && index === chatStore.messages.length - 1"
                @contextmenu="handleMessageContextMenu"
              />
            </template>
          </VirtualScroll>

          <!-- Regular rendering for small lists -->
          <div v-else class="pb-4">
            <ChatMessage
              v-for="(message, index) in chatStore.messages"
              :key="`${message.conversation_id}-${message.id}`"
              :message="message"
              :is-streaming="chatStore.streaming && index === chatStore.messages.length - 1"
              @contextmenu="handleMessageContextMenu"
            />
          </div>

          <!-- Stream error display (shown in chat area with gray text) -->
          <div
            v-if="chatStore.streamError"
            class="flex justify-center py-4"
          >
            <div class="flex items-center gap-2 px-4 py-2 text-gray-400 dark:text-gray-500 text-sm">
              <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <span class="break-all">{{
                chatStore.streamError === 'streamEmpty' ? t('chat.streamEmpty') :
                chatStore.streamError === 'providerNoResponse' ? t('chat.providerNoResponse') :
                chatStore.streamError === 'providerReturnedEmpty' ? t('chat.providerReturnedEmpty') :
                chatStore.streamError === 'noResponseBody' ? t('chat.noResponseBody') :
                chatStore.streamError === 'trial_service_busy' ? t('chat.trialServiceBusy') :
                chatStore.streamError
              }}</span>
              <button
                class="ml-2 text-gray-400 hover:text-gray-300 cursor-pointer"
                @click="chatStore.clearStreamError"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          <!-- Streaming action buttons -->
          <div
            v-if="chatStore.messages.length > 0 && !chatStore.isMultiSelectMode"
            class="flex justify-center gap-2 py-4"
          >
            <!-- Stop button (shown during streaming) -->
            <button
              v-if="chatStore.streaming"
              class="flex items-center gap-2 px-4 py-2 glass-card text-red-400 hover:bg-red-500/10 rounded-lg text-sm transition-colors cursor-pointer"
              @click="handleCancel"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
              </svg>
              {{ t('chat.stopGenerating') }}
            </button>

            <!-- Continue and Regenerate buttons (shown when not streaming and last message is from assistant) -->
            <template v-else-if="chatStore.messages[chatStore.messages.length - 1]?.role === 'assistant'">
              <button
                class="flex items-center gap-2 px-4 py-2 glass-card text-gray-600 dark:text-gray-300 hover:bg-white/10 rounded-lg text-sm transition-colors cursor-pointer"
                :disabled="chatStore.sending"
                @click="handleContinue"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3" />
                </svg>
                {{ t('chat.continueGenerating') }}
              </button>
              <button
                class="flex items-center gap-2 px-4 py-2 glass-card text-gray-600 dark:text-gray-300 hover:bg-white/10 rounded-lg text-sm transition-colors cursor-pointer"
                :disabled="chatStore.sending"
                @click="handleRegenerate"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                {{ t('chat.regenerate') }}
              </button>
            </template>
          </div>
        </template>
      </div>

      <!-- Context menu -->\n      <Teleport to="body">
        <!-- Desktop: Floating menu -->
        <div
          v-if="showContextMenu && !isMobile"
          class="context-menu fixed z-[100] glass-card shadow-xl py-1 min-w-[160px]"
          :style="{ left: `${contextMenuPosition.x}px`, top: `${contextMenuPosition.y}px` }"
        >
          <button
            class="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
            @click="handleSelectMessage"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            {{ t('chat.selectMessage') }}
          </button>
        </div>

        <!-- Mobile: Bottom sheet -->
        <Transition name="sheet">
          <div
            v-if="showContextMenu && isMobile"
            class="fixed inset-0 z-[9999] flex items-end"
            @click="showContextMenu = false"
          >
            <!-- Backdrop -->
            <div class="absolute inset-0 bg-black/50" />

            <!-- Sheet content -->
            <div
              class="relative w-full bg-white dark:bg-gray-800 rounded-t-2xl shadow-2xl"
              @click.stop
            >
              <!-- Handle bar -->
              <div class="flex justify-center pt-3 pb-2">
                <div class="w-10 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
              </div>

              <!-- Actions -->
              <div class="p-4 space-y-2">
                <button
                  class="w-full flex items-center gap-3 px-4 py-3 bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-900 dark:text-white rounded-xl transition-colors"
                  @click="handleSelectMessage"
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                  </svg>
                  <span class="font-medium">{{ t('chat.selectMessage') }}</span>
                </button>
              </div>

              <!-- Safe area padding -->
              <div class="h-[env(safe-area-inset-bottom)]" />
            </div>
          </div>
        </Transition>
      </Teleport>

      <!-- Routing mode menu (teleported to body for proper z-index) -->
      <Teleport to="body">
        <!-- Desktop: Dropdown menu -->
        <div
          v-if="showRoutingMenu && !isMobile"
          class="routing-menu-container fixed z-[100] glass-card shadow-xl p-2 min-w-[200px]"
          :style="{ left: `${Math.max(8, routingMenuPosition.x - 200)}px`, top: `${routingMenuPosition.y}px` }"
        >
          <!-- Auto mode -->
          <button
            class="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-left transition-colors cursor-pointer"
            :class="providerPoolStore.routingMode === 'auto' ? 'bg-gray-100 dark:bg-gray-700/30 text-gray-900 dark:text-gray-300' : 'hover:bg-white/10 text-gray-700 dark:text-gray-300'"
            @click="selectRoutingMode('auto')"
          >
            <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            <div class="flex-1">
              <div class="font-medium">{{ t('chat.routingMode.auto') }}</div>
              <div class="text-xs opacity-60">{{ t('chat.routingMode.autoDesc') }}</div>
            </div>
            <span class="text-xs opacity-60">{{ cloudActiveCount + localActiveCount }}</span>
          </button>

          <!-- Cloud mode -->
          <button
            class="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-left transition-colors cursor-pointer"
            :class="providerPoolStore.routingMode === 'cloud' ? 'bg-gray-100 dark:bg-gray-700/30 text-gray-900 dark:text-gray-300' : 'hover:bg-white/10 text-gray-700 dark:text-gray-300'"
            :disabled="!providerPoolStore.hasCloudProviders"
            @click="selectRoutingMode('cloud')"
          >
            <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
            </svg>
            <div class="flex-1">
              <div class="font-medium" :class="{ 'opacity-50': !providerPoolStore.hasCloudProviders }">{{ t('chat.routingMode.cloud') }}</div>
              <div class="text-xs opacity-60">{{ t('chat.routingMode.cloudDesc') }}</div>
            </div>
            <span class="text-xs" :class="cloudActiveCount > 0 ? 'text-green-500' : 'opacity-40'">{{ cloudActiveCount }}</span>
          </button>

          <!-- Local mode -->
          <button
            class="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-left transition-colors cursor-pointer"
            :class="providerPoolStore.routingMode === 'local' ? 'bg-gray-100 dark:bg-gray-700/30 text-gray-900 dark:text-gray-300' : 'hover:bg-white/10 text-gray-700 dark:text-gray-300'"
            :disabled="!providerPoolStore.hasLocalProviders"
            @click="selectRoutingMode('local')"
          >
            <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            <div class="flex-1">
              <div class="font-medium" :class="{ 'opacity-50': !providerPoolStore.hasLocalProviders }">{{ t('chat.routingMode.local') }}</div>
              <div class="text-xs opacity-60">{{ t('chat.routingMode.localDesc') }}</div>
            </div>
            <span class="text-xs" :class="localActiveCount > 0 ? 'text-green-500' : 'opacity-40'">{{ localActiveCount }}</span>
          </button>

          <!-- Divider -->
          <div class="border-t border-gray-200 dark:border-gray-700 my-2"></div>

          <!-- Link to LLM settings -->
          <router-link
            to="/settings?tab=llm"
            class="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-left transition-colors hover:bg-white/10 text-gray-500 dark:text-gray-400"
            @click="showRoutingMenu = false"
          >
            <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            <span>{{ t('chat.manageProviders') }}</span>
          </router-link>
        </div>

        <!-- Mobile: Bottom sheet -->
        <Transition name="sheet">
          <div
            v-if="showRoutingMenu && isMobile"
            class="fixed inset-0 z-[9999] flex items-end"
            @click="showRoutingMenu = false"
          >
            <!-- Backdrop -->
            <div class="absolute inset-0 bg-black/50" />

            <!-- Sheet content -->
            <div
              class="relative w-full bg-white dark:bg-gray-800 rounded-t-2xl shadow-2xl"
              @click.stop
            >
              <!-- Handle bar -->
              <div class="flex justify-center pt-3 pb-2">
                <div class="w-10 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
              </div>

              <!-- Title -->
              <div class="px-4 pb-2">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('chat.routingMode.title') }}</h3>
              </div>

              <!-- Actions -->
              <div class="p-4 space-y-2">
                <!-- Auto mode -->
                <button
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left transition-colors"
                  :class="providerPoolStore.routingMode === 'auto' ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white' : 'bg-gray-50 dark:bg-gray-700/50 text-gray-700 dark:text-gray-300'"
                  @click="selectRoutingMode('auto')"
                >
                  <svg class="w-5 h-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                  <div class="flex-1">
                    <div class="font-medium">{{ t('chat.routingMode.auto') }}</div>
                    <div class="text-xs opacity-60">{{ t('chat.routingMode.autoDesc') }}</div>
                  </div>
                  <span class="text-sm opacity-60">{{ cloudActiveCount + localActiveCount }}</span>
                </button>

                <!-- Cloud mode -->
                <button
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left transition-colors"
                  :class="providerPoolStore.routingMode === 'cloud' ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white' : 'bg-gray-50 dark:bg-gray-700/50 text-gray-700 dark:text-gray-300'"
                  :disabled="!providerPoolStore.hasCloudProviders"
                  @click="selectRoutingMode('cloud')"
                >
                  <svg class="w-5 h-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
                  </svg>
                  <div class="flex-1">
                    <div class="font-medium" :class="{ 'opacity-50': !providerPoolStore.hasCloudProviders }">{{ t('chat.routingMode.cloud') }}</div>
                    <div class="text-xs opacity-60">{{ t('chat.routingMode.cloudDesc') }}</div>
                  </div>
                  <span class="text-sm" :class="cloudActiveCount > 0 ? 'text-green-500' : 'opacity-40'">{{ cloudActiveCount }}</span>
                </button>

                <!-- Local mode -->
                <button
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left transition-colors"
                  :class="providerPoolStore.routingMode === 'local' ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white' : 'bg-gray-50 dark:bg-gray-700/50 text-gray-700 dark:text-gray-300'"
                  :disabled="!providerPoolStore.hasLocalProviders"
                  @click="selectRoutingMode('local')"
                >
                  <svg class="w-5 h-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                  </svg>
                  <div class="flex-1">
                    <div class="font-medium" :class="{ 'opacity-50': !providerPoolStore.hasLocalProviders }">{{ t('chat.routingMode.local') }}</div>
                    <div class="text-xs opacity-60">{{ t('chat.routingMode.localDesc') }}</div>
                  </div>
                  <span class="text-sm" :class="localActiveCount > 0 ? 'text-green-500' : 'opacity-40'">{{ localActiveCount }}</span>
                </button>

                <!-- Link to LLM settings -->
                <router-link
                  to="/settings?tab=llm"
                  class="w-full flex items-center justify-center gap-2 px-4 py-3 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-900 dark:text-white rounded-xl transition-colors"
                  @click="showRoutingMenu = false"
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                  <span>{{ t('chat.manageProviders') }}</span>
                </router-link>
              </div>

              <!-- Safe area padding -->
              <div class="h-[env(safe-area-inset-bottom)]" />
            </div>
          </div>
        </Transition>
      </Teleport>

      <!-- Theme style selector (teleported to body for proper z-index) -->
      <Teleport to="body">
        <!-- Desktop: Dropdown menu -->
        <div
          v-if="showStyleSelector && !isMobile"
          class="style-selector-container fixed z-[100] glass-card shadow-xl p-3 min-w-[180px]"
          :style="{ left: `${Math.max(8, styleSelectorPosition.x - 180)}px`, top: `${styleSelectorPosition.y}px` }"
        >
          <div class="text-xs text-gray-500 dark:text-slate-400 mb-2 font-medium">{{ t('theme.styles.title') }}</div>
          <div class="flex flex-col gap-1">
            <button
              v-for="style in THEME_STYLES"
              :key="style.id"
              class="flex items-center gap-2 px-2 py-1.5 rounded-md text-sm text-left transition-colors cursor-pointer"
              :class="settingsStore.themeStyle === style.id ? 'bg-gray-100 dark:bg-gray-700/30 text-gray-900 dark:text-gray-300' : 'hover:bg-white/10 text-gray-700 dark:text-gray-300'"
              @click="selectThemeStyle(style.id)"
            >
              <span
                class="theme-style-btn flex-shrink-0"
                :class="`theme-style-btn-${style.id}`"
              />
              <span>{{ t(style.labelKey) }}</span>
            </button>
          </div>
        </div>

        <!-- Mobile: Bottom sheet -->
        <Transition name="sheet">
          <div
            v-if="showStyleSelector && isMobile"
            class="fixed inset-0 z-[9999] flex items-end"
            @click="showStyleSelector = false"
          >
            <!-- Backdrop -->
            <div class="absolute inset-0 bg-black/50" />

            <!-- Sheet content -->
            <div
              class="relative w-full bg-white dark:bg-gray-800 rounded-t-2xl shadow-2xl"
              @click.stop
            >
              <!-- Handle bar -->
              <div class="flex justify-center pt-3 pb-2">
                <div class="w-10 h-1 bg-gray-300 dark:bg-gray-600 rounded-full" />
              </div>

              <!-- Title -->
              <div class="px-4 pb-2">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('theme.styles.title') }}</h3>
              </div>

              <!-- Actions -->
              <div class="p-4 space-y-2">
                <button
                  v-for="style in THEME_STYLES"
                  :key="style.id"
                  class="w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left transition-colors"
                  :class="settingsStore.themeStyle === style.id ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white' : 'bg-gray-50 dark:bg-gray-700/50 text-gray-700 dark:text-gray-300'"
                  @click="selectThemeStyle(style.id)"
                >
                  <span
                    class="theme-style-btn flex-shrink-0"
                    :class="`theme-style-btn-${style.id}`"
                  />
                  <span class="font-medium">{{ t(style.labelKey) }}</span>
                </button>
              </div>

              <!-- Safe area padding -->
              <div class="h-[env(safe-area-inset-bottom)]" />
            </div>
          </div>
        </Transition>
      </Teleport>

      <!-- Multi-select action bar -->
      <Transition name="slide-up">
        <div
          v-if="chatStore.isMultiSelectMode"
          class="absolute bottom-20 left-1/2 -translate-x-1/2 z-20 glass-card shadow-xl px-4 py-3 flex items-center gap-4"
        >
          <span class="text-sm text-gray-600 dark:text-gray-300">
            {{ chatStore.selectedMessageIds.size }} {{ t('chat.messagesSelected') }}
          </span>
          <div class="flex items-center gap-2">
            <button
              class="px-3 py-1.5 text-sm bg-red-500/20 hover:bg-red-500/30 text-red-400 rounded-lg flex items-center gap-1.5 transition-colors cursor-pointer"
              :disabled="chatStore.selectedMessageIds.size === 0"
              @click="handleDeleteSelectedMessages"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
              {{ t('common.delete') }}
            </button>
            <button
              class="px-3 py-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 rounded-lg transition-colors cursor-pointer"
              @click="handleCancelSelection"
            >
              {{ t('common.cancel') }}
            </button>
          </div>
        </div>
      </Transition>

      <!-- Error message -->
      <div
        v-if="chatStore.error"
        class="px-3 sm:px-4 py-3 bg-red-500/10 border-t border-red-500/30 text-red-400 text-xs sm:text-sm flex items-center justify-between gap-2"
      >
        <span class="truncate">{{ chatStore.error === 'trial_service_busy' ? t('chat.trialServiceBusy') : chatStore.error }}</span>
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

      <!-- Trial Exhausted Banner -->
      <div
        v-if="chatStore.trialExhausted"
        class="px-3 sm:px-4 py-3 bg-gray-500/10 border-t border-gray-500/30 text-gray-400 text-xs sm:text-sm flex items-center justify-between gap-2"
      >
        <div class="flex items-center gap-2">
          <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span>{{ t('chat.trialExhausted') }}</span>
        </div>
        <router-link
          to="/settings?tab=llm"
          class="text-gray-900 dark:text-white hover:text-gray-900 dark:text-white flex-shrink-0 px-3 py-1 rounded hover:bg-gray-100 dark:bg-gray-600/10 transition-colors"
        >
          {{ t('chat.configureProvider') }}
        </router-link>
      </div>

      <!-- Input area - floating at bottom -->
      <div class="absolute bottom-0 left-0 right-0 z-10">
        <ChatInput
          ref="chatInputRef"
          :disabled="chatStore.sending"
          :streaming="chatStore.streaming"
          @send="handleSend"
          @cancel="handleCancel"
          @open-talk-mode="showTalkMode = true"
        />
      </div>

      <!-- Talk Mode -->
      <TalkMode
        v-model="showTalkMode"
        :conversation-id="chatStore.currentConversationId || undefined"
        @transcript="handleVoiceTranscript"
      />
    </main>
  </div>

  <!-- Mobile View (Teleported to body for complete independence) -->
  <Teleport to="body">
    <!-- Mobile: Conversation list page -->
    <div
      v-if="isMobile && showListPage"
      class="fixed inset-0 flex flex-col bg-white dark:bg-gray-900"
      style="z-index: 50;"
    >
      <ConversationList
        :conversations="chatStore.sortedConversations"
        :current-id="chatStore.currentConversationId"
        :loading="chatStore.loading"
        :searching="chatStore.searching"
        @select="handleSelectConversation"
        @create="handleCreateConversation"
        @delete="handleDeleteConversation"
        @search="handleSearch"
        @pin="handlePinConversation"
        @unpin="handleUnpinConversation"
      />
    </div>

    <!-- Mobile: Chat overlay -->
    <Transition name="slide-only">
      <div
        v-if="isMobile && !showListPage"
        class="fixed inset-0 flex flex-col bg-surface-base"
        :class="themeStyleClass"
        style="z-index: 50;"
      >
        <!-- Mobile header -->
        <header class="flex-shrink-0 flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-glass-border glass-header bg-surface-base">
          <div class="flex items-center gap-3 flex-1 min-w-0">
            <button
              class="p-1.5 -ml-1.5 text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white rounded-lg transition-colors cursor-pointer"
              @click="toggleSidebar"
            >
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white truncate">
              {{ chatStore.currentConversation?.title || t('chat.newChat') }}
            </h2>
          </div>
          <div class="flex items-center gap-1 flex-shrink-0">
            <button
              class="p-1.5 text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition-colors cursor-pointer"
              @click.stop="toggleRoutingMenu"
            >
              <span
                class="w-2 h-2 rounded-full inline-block"
                :class="{
                  'bg-red-500 animate-pulse': providerStatus.status === 'error',
                  'bg-green-500': providerStatus.status === 'active',
                  'bg-gray-400': providerStatus.status === 'none',
                  'bg-yellow-500 animate-pulse': providerStatus.status === 'pending'
                }"
              />
            </button>
            <button
              class="p-1.5 text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition-colors cursor-pointer"
              @click.stop="toggleStyleSelector"
            >
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
              </svg>
            </button>
          </div>
        </header>

        <!-- Messages area -->
        <div class="flex-1 overflow-y-auto overscroll-contain px-4 pb-32 bg-surface-base">
          <!-- Empty state -->
          <div v-if="chatStore.messages.length === 0 && !chatStore.loading" class="h-full flex flex-col items-center justify-center">
            <img src="/logo.svg" alt="Logo" class="w-12 h-12 mb-4 opacity-60" />
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">{{ t('chat.startConversation') }}</h3>
            <p class="text-sm text-gray-500 dark:text-slate-400 mb-6">{{ t('chat.startConversationDesc') }}</p>
            <PresetQuestions @select="handlePresetQuestionSelect" />
          </div>

          <!-- Messages -->
          <template v-else>
            <div
              v-for="(message, index) in chatStore.messages"
              :key="`${message.conversation_id}-${message.id}`"
              @touchstart="(e) => handleTouchStart(e, message.id)"
              @touchend="handleTouchEnd"
              @touchmove="handleTouchEnd"
            >
              <ChatMessage
                :message="message"
                :is-streaming="chatStore.streaming && index === chatStore.messages.length - 1"
                @contextmenu="handleMessageContextMenu"
              />
            </div>

            <!-- Action buttons -->
            <div v-if="chatStore.messages.length > 0" class="flex justify-center gap-2 py-4">
              <button
                v-if="chatStore.streaming"
                class="flex items-center gap-2 px-4 py-2 bg-red-500/10 text-red-400 rounded-lg text-sm cursor-pointer"
                @click="handleCancel"
              >
                <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                  <rect x="6" y="6" width="12" height="12" rx="1" />
                </svg>
                {{ t('chat.stopGenerating') }}
              </button>
              <template v-else-if="chatStore.messages[chatStore.messages.length - 1]?.role === 'assistant'">
                <button
                  class="flex items-center gap-2 px-4 py-2 glass-card text-gray-600 dark:text-gray-300 rounded-lg text-sm cursor-pointer"
                  @click="handleContinue"
                >
                  {{ t('chat.continueGenerating') }}
                </button>
                <button
                  class="flex items-center gap-2 px-4 py-2 glass-card text-gray-600 dark:text-gray-300 rounded-lg text-sm cursor-pointer"
                  @click="handleRegenerate"
                >
                  {{ t('chat.regenerate') }}
                </button>
              </template>
            </div>
          </template>
        </div>

        <!-- Input area -->
        <div class="flex-shrink-0 border-t border-gray-200 dark:border-glass-border bg-surface-base">
          <ChatInput
            :disabled="chatStore.sending"
            :streaming="chatStore.streaming"
            @send="handleSend"
            @cancel="handleCancel"
            @open-talk-mode="showTalkMode = true"
          />
        </div>

        <!-- Talk Mode -->
        <TalkMode
          v-model="showTalkMode"
          :conversation-id="chatStore.currentConversationId || undefined"
          @transcript="handleVoiceTranscript"
        />
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.chat-view {
  min-height: 0;
  flex: 1;
}

/* Safe area for mobile devices with notches */
@supports (padding-bottom: env(safe-area-inset-bottom)) {
  .chat-view {
    padding-bottom: env(safe-area-inset-bottom);
  }
}

/* Slide animation for mobile chat overlay (no fade) */
.slide-only-enter-active,
.slide-only-leave-active {
  transition: transform 0.3s ease-out;
}

.slide-only-enter-from {
  transform: translateX(100%);
}

.slide-only-enter-to {
  transform: translateX(0);
}

.slide-only-leave-from {
  transform: translateX(0);
}

.slide-only-leave-to {
  transform: translateX(100%);
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

/* Slide up transition */
.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.3s ease;
}

.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translate(-50%, 20px);
}

/* Slide fade transition for trial quota banner */
.slide-fade-enter-active {
  transition: all 0.3s ease-out;
}
.slide-fade-leave-active {
  transition: all 0.3s ease-in;
}
.slide-fade-enter-from,
.slide-fade-leave-to {
  opacity: 0;
  transform: translateY(-100%);
  max-height: 0;
  padding-top: 0;
  padding-bottom: 0;
  border-bottom-width: 0;
}

/* Bottom sheet animation for mobile menus */
.sheet-enter-active,
.sheet-leave-active {
  transition: opacity 0.3s ease;
}

.sheet-enter-active > div:last-child,
.sheet-leave-active > div:last-child {
  transition: transform 0.3s ease;
}

.sheet-enter-from,
.sheet-leave-to {
  opacity: 0;
}

.sheet-enter-from > div:last-child,
.sheet-leave-to > div:last-child {
  transform: translateY(100%);
}

.sheet-enter-to,
.sheet-leave-from {
  opacity: 1;
}

.sheet-enter-to > div:last-child,
.sheet-leave-from > div:last-child {
  transform: translateY(0);
}

/* Token change animation */
.token-change-animation {
  animation: token-pulse 0.8s ease-out;
}

@keyframes token-pulse {
  0% {
    transform: scale(1);
    color: inherit;
    text-shadow: none;
  }
  20% {
    transform: scale(1.3);
    color: #ef4444;
    text-shadow: 0 0 10px rgba(239, 68, 68, 0.8), 0 0 20px rgba(239, 68, 68, 0.5);
  }
  50% {
    transform: scale(1.15);
    color: #f59e0b;
    text-shadow: 0 0 8px rgba(245, 158, 11, 0.6);
  }
  100% {
    transform: scale(1);
    color: inherit;
    text-shadow: none;
  }
}

/* Trial quota banner pulse when tokens change */
.trial-quota-pulse {
  animation: banner-pulse 0.8s ease-out;
}

@keyframes banner-pulse {
  0%, 100% {
    background-color: rgb(239 246 255 / var(--tw-bg-opacity, 1));
  }
  30% {
    background-color: rgb(254 243 199 / var(--tw-bg-opacity, 1));
  }
}

:root.dark .trial-quota-pulse {
  animation: banner-pulse-dark 0.8s ease-out;
}

@keyframes banner-pulse-dark {
  0%, 100% {
    background-color: rgb(30 58 138 / 0.2);
  }
  30% {
    background-color: rgb(180 83 9 / 0.3);
  }
}
</style>
