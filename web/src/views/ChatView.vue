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
import ToolApprovalDialog from '@/components/ToolApprovalDialog.vue'
import MediaParamPanel from '@/components/MediaParamPanel.vue'
import { useMediaGenerate } from '@/composables/useMediaGenerate'
import { componentPool } from '@/utils/componentPool'
import { clearConversationIncrementalStates } from '@/utils/typeless'
import { THEME_STYLES, type ThemeStyle } from '@/stores/settings'
import { formatTokens } from '@/utils/format'

const { t, locale } = useI18n()
const router = useRouter()
const chatStore = useChatStore()
const settingsStore = useSettingsStore()
const providerPoolStore = useProviderPoolStore()
const mediaGen = useMediaGenerate()

// Trial quota animation state
const tokenAnimating = ref(false)
const previousTokens = ref<number | null>(null)

// Theme style class for chat interface
const themeStyleClass = computed(() => `theme-style-${settingsStore.themeStyle}`)

const messagesContainer = ref<HTMLElement | null>(null)
const virtualScrollRef = ref<InstanceType<typeof VirtualScroll> | null>(null)
const chatInputRef = ref<InstanceType<typeof ChatInput> | null>(null)
const showSidebar = ref(false) // Default closed on mobile
// Detect mobile synchronously before first render to avoid layout flash
const _initMobile = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini/i.test(navigator.userAgent)
const _initConvId = new URLSearchParams(window.location.search).get('conversationId')
const isMobile = ref(_initMobile)
const isNarrowScreen = ref(window.innerWidth < 768) // PC narrow screen (<768px)
const isCompact = computed(() => isNarrowScreen.value) // Compact mode = narrow screen
const isMac = computed(() => navigator.platform.toUpperCase().indexOf('MAC') >= 0)
// If URL has conversationId on mobile, pre-populate pageStack so we skip list page on first render
const pageStack = ref<string[]>(_initMobile && _initConvId ? [_initConvId] : [])
const showListPage = computed(() => isMobile.value && pageStack.value.length === 0)
const mobileAnimationEnabled = ref(false) // Only animate after user interaction, not on page load
const showRoutingMenu = ref(false)
const routingMenuPosition = ref({ x: 0, y: 0 })
const routingButtonRef = ref<HTMLElement | null>(null)
const claudeCodeConfig = ref<ClaudeCodeConfigResponse | null>(null)

// Theme style selector state
const showStyleSelector = ref(false)
const styleButtonRef = ref<HTMLElement | null>(null)

// Context trim indicator with 1-second delay
const showContextTrim = ref(false)
let contextTrimTimer: ReturnType<typeof setTimeout> | null = null
watch(() => chatStore.contextTrimInfo, (info) => {
  if (contextTrimTimer) { clearTimeout(contextTrimTimer); contextTrimTimer = null }
  if (info) {
    // Skip if pruned but nothing actually saved (pruner not active)
    if (info.type === 'pruned') {
      const saved = (info.tokensBefore ?? 0) - (info.tokensAfter ?? 0)
      if (saved <= 0 && (info.messagesPruned ?? 0) <= 0) {
        showContextTrim.value = false
        return
      }
    }
    // Show after 1s delay (only if still streaming)
    contextTrimTimer = setTimeout(() => {
      if (chatStore.streaming || info) showContextTrim.value = true
    }, 1000)
  } else {
    showContextTrim.value = false
  }
})
// Auto-hide after streaming ends (with a short delay so user can read it)
watch(() => chatStore.streaming, (val) => {
  if (!val && showContextTrim.value) {
    setTimeout(() => { showContextTrim.value = false }, 3000)
  }
})
const styleSelectorPosition = ref({ x: 0, y: 0 })

// Talk mode state
const showTalkMode = ref(false)
const editBeforeSend = ref(false)

// Virtual scroll threshold - use virtual scroll when message count exceeds this
const VIRTUAL_SCROLL_THRESHOLD = 50

// Whether to use virtual scrolling
const useVirtualScroll = computed(() => chatStore.messages.length > VIRTUAL_SCROLL_THRESHOLD)

// ID of the last assistant message (for Continue/Regenerate in mobile action sheet)
const lastAssistantMessageId = computed(() => {
  const msgs = chatStore.messages
  for (let i = msgs.length - 1; i >= 0; i--) {
    if (msgs[i].role === 'assistant') return msgs[i].id
  }
  return null
})

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
  async (newId, oldId) => {
    if (isMobile.value) {
      showSidebar.value = false
    }
    // Clear incremental parse states for the old conversation to free memory
    if (oldId && oldId !== newId) {
      clearConversationIncrementalStates(oldId)
    }
    // Scroll to bottom when entering a conversation
    if (newId) {
      isUserNearBottom.value = true
      await nextTick()
      scrollToBottom()
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
  // Check for media generation intent before sending to chat
  const hasImages = attachments?.some((a) => a.type.startsWith('image/')) || false
  const imageCount = attachments?.filter((a) => a.type.startsWith('image/')).length || 0
  const imageFiles = attachments?.filter((a) => a.type.startsWith('image/')).map((a) => a.file) || []
  const detected = await mediaGen.classify(message, hasImages, imageCount, locale.value, imageFiles)
  if (detected) {
    // Media intent detected — show param panel instead of sending to chat.
    // Invalidate warmup cache since no LLM chat request will follow.
    chatStore.resetWarmup()
    chatInputRef.value?.resetWarmup?.()
    return
  }
  await chatStore.sendMessage(message, attachments)
}

async function handleMediaGenerate() {
  await mediaGen.generate()
}

function handleMediaDismiss() {
  // User chose "No, just chat" — send the original message to chat instead.
  const prompt = mediaGen.intent.value?.prompt
  mediaGen.reset()
  if (prompt) {
    chatStore.sendMessage(prompt)
  }
  nextTick(() => chatInputRef.value?.focus?.())
}

function handleMediaClose() {
  // Close button (✕) — just dismiss the panel, do nothing else.
  mediaGen.reset()
  nextTick(() => chatInputRef.value?.focus?.())
}

async function handleMediaConfirm() {
  await mediaGen.confirmAmbiguous()
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

async function handleSelectConversation(id: string) {
  if (isMobile.value) {
    mobileAnimationEnabled.value = true
    pageStack.value.push(id)
    // Set query param to trigger AppHeader hide
    await router.push({ query: { conversationId: id } })
  }
  await chatStore.selectConversation(id)
  chatStore.resetWarmup()
  chatInputRef.value?.resetWarmup?.()
}

async function handleCreateConversation() {
  chatStore.resetWarmup()
  chatInputRef.value?.resetWarmup?.()
  const conv = await chatStore.createConversation(t('chat.newConversation'))
  if (isMobile.value && conv?.id) {
    mobileAnimationEnabled.value = true
    pageStack.value.push(conv.id)
    await router.push({ query: { conversationId: conv.id } })
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
  if (!target.closest('.context-menu') && !contextMenuJustOpened.value) {
    showContextMenu.value = false
  }
}

// Context menu handlers
const contextMenuJustOpened = ref(false)

function handleMessageContextMenu(event: MouseEvent, messageId: string) {
  event.preventDefault()

  // On mobile, context menu is handled by ChatMessage's own long-press menu
  if (isMobile.value) return

  contextMenuMessageId.value = messageId
  contextMenuPosition.value = { x: event.clientX, y: event.clientY }
  showContextMenu.value = true
  // Prevent the subsequent click event from immediately closing the menu
  contextMenuJustOpened.value = true
  setTimeout(() => { contextMenuJustOpened.value = false }, 200)
}

// Mobile long press handlers
const longPressTimer = ref<number | null>(null)
const longPressMessageId = ref<string | null>(null)

function handleTouchStart(event: TouchEvent, messageId: string) {
  longPressMessageId.value = messageId
  longPressTimer.value = window.setTimeout(() => {
    const touch = event.touches[0]
    if (touch) {
      contextMenuMessageId.value = messageId
      contextMenuPosition.value = { x: touch.clientX, y: touch.clientY }
      showContextMenu.value = true
    }
  }, 500)
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

// Check if the context-menu'd message is the last assistant message (for Continue/Regenerate)
const isContextMenuLastAssistant = computed(() => {
  return !!contextMenuMessageId.value && contextMenuMessageId.value === lastAssistantMessageId.value
})

function handleContextContinue() {
  chatStore.continueMessage()
  showContextMenu.value = false
}

function handleContextRegenerate() {
  chatStore.regenerateMessage()
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
  // Fetch edit-before-send setting for TalkMode
  authFetch('/api/v1/speech/status').then(r => r.json()).then(s => {
    editBeforeSend.value = s?.asr?.edit_before_send ?? false
  }).catch(() => {})

  // Fetch conversations and (if URL has conversationId) messages in parallel.
  // selectConversation only needs the ID, not the conversation list.
  if (_initConvId) {
    await Promise.all([
      chatStore.fetchConversations(),
      chatStore.selectConversation(_initConvId),
    ])
  } else {
    await chatStore.fetchConversations()
    // Auto-select first conversation if available and none selected (desktop only)
    if (!isMobile.value && !chatStore.currentConversationId && chatStore.sortedConversations.length > 0 && chatStore.sortedConversations[0]) {
      await chatStore.selectConversation(chatStore.sortedConversations[0].id)
    }
  }

  // Fire secondary data fetches in background — don't block first paint
  providerPoolStore.fetchProviders().then(() => {
    const llmProviders = providerPoolStore.providers.filter((p: any) => p.type !== 'media')
    settingsStore.updateFromPoolProviders(llmProviders)
  }).catch(() => {})
  settingsStore.fetchTools().catch(() => {})
  providerPoolStore.fetchRoutingMode().catch(() => {})
  fetchClaudeCodeConfig()

  // Check for any pending tool approvals (e.g. page was refreshed while waiting)
  chatStore.checkPendingApprovals()
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
  <div class="chat-view flex relative" :class="[themeStyleClass, { 'mobile-view': isMobile && !showListPage }]">
    <!-- Overlay for narrow screen sidebar -->
    <div
      v-if="!isMobile && isNarrowScreen && showSidebar"
      class="fixed inset-0 bg-black/60 backdrop-blur-sm z-30"
      @click="toggleSidebar"
    />

    <!-- Sidebar / Conversation List -->
    <aside
      v-show="isMobile ? showListPage : showSidebar"
      class="conversation-sidebar flex-shrink-0 border-r border-glass-border transition-transform duration-300 glass-sidebar"
      :class="{
        'w-80': !isMobile,
        'z-40': !isMobile,
        'fixed left-0 top-0 h-full': !isMobile && isNarrowScreen,
        '-translate-x-full': !isMobile && isNarrowScreen && !showSidebar,
        'mobile-list-page': isMobile && showListPage,
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

    <!-- Main chat area -->
    <main
      :class="isMobile ? ['mobile-chat', { 'mobile-chat-offscreen': showListPage, 'mobile-chat-animated': mobileAnimationEnabled }] : ''"
      class="flex-1 flex flex-col min-w-0 min-h-0 h-full relative"
      @dragover.prevent="chatInputRef?.handleDragOver($event)"
      @dragleave="chatInputRef?.handleDragLeave()"
      @drop.prevent="chatInputRef?.handleDrop($event)"
    >
      <!-- Chat header -->
      <header class="flex items-center justify-between px-4 py-3 sm:p-4 border-b border-gray-200 dark:border-glass-border gap-2"
        :class="isMobile ? 'bg-white dark:bg-gray-900' : 'glass-header'"
      >
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
          <!-- Enable Claude Code CLI prompt -->
          <router-link
            v-if="!isClaudeCodeEnabled"
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

      <!-- Trial Quota Banner (show when trial provider is active and quota not exhausted) -->
      <Transition name="slide-fade">
        <div
          v-if="providerPoolStore.trialQuota && providerPoolStore.trialProviders?.length > 0 && !providerPoolStore.trialQuota.is_exhausted"
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
        class="flex-1 min-h-0 overflow-y-auto overscroll-contain chat-messages-area bg-surface-base"
        :class="isMobile ? 'pb-4' : 'pb-32'"
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
                :is-last-assistant-message="chatStore.messages[index]!.id === lastAssistantMessageId"
                @contextmenu="handleMessageContextMenu"
                @continue="chatStore.continueMessage()"
                @regenerate="chatStore.regenerateMessage()"
              />
            </template>
          </VirtualScroll>

          <!-- Regular rendering for small lists -->
          <div v-else class="pb-4">
            <div
              v-for="(message, index) in chatStore.messages"
              :key="`${message.conversation_id}-${message.id}`"
            >
              <ChatMessage
                :message="message"
                :is-streaming="chatStore.streaming && index === chatStore.messages.length - 1"
                :is-last-assistant-message="message.id === lastAssistantMessageId"
                @contextmenu="handleMessageContextMenu"
                @continue="chatStore.continueMessage()"
                @regenerate="chatStore.regenerateMessage()"
              />
            </div>
          </div>

          <!-- Context trim indicator (pruning/compaction) -->
          <Transition name="fade">
            <div
              v-if="showContextTrim"
              class="flex justify-center py-2"
            >
              <div class="flex items-center gap-2 px-3 py-1.5 text-xs text-gray-400/70 dark:text-gray-500/70 bg-gray-100/30 dark:bg-gray-800/30 rounded-full">
                <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.121 14.121L19 19m-7-7l7-7m-7 7l-2.879 2.879M12 12L9.121 9.121m0 5.758a3 3 0 10-4.243 4.243 3 3 0 004.243-4.243zm0-5.758a3 3 0 10-4.243-4.243 3 3 0 004.243 4.243z" />
                </svg>
                <span v-if="chatStore.contextTrimInfo?.type === 'pruned'">
                  <template v-if="((chatStore.contextTrimInfo.tokensBefore ?? 0) - (chatStore.contextTrimInfo.tokensAfter ?? 0)) > 0">
                    {{ t('chat.contextPruned', { tokens: (chatStore.contextTrimInfo.tokensBefore ?? 0) - (chatStore.contextTrimInfo.tokensAfter ?? 0) }) }}
                  </template>
                  <template v-else>
                    {{ t('chat.contextPrunedLight') }}
                  </template>
                </span>
                <span v-else-if="chatStore.contextTrimInfo?.type === 'compacted'">
                  {{ t('chat.contextCompacted', { before: chatStore.contextTrimInfo.before ?? 0, after: chatStore.contextTrimInfo.after ?? 0 }) }}
                </span>
              </div>
            </div>
          </Transition>

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

          <!-- "I'm listening" indicator (shown when pre-TTFT cancel is active) -->
          <div
            v-if="chatStore.preTTFTCancelActive"
            class="flex justify-center py-3"
          >
            <div class="flex items-center gap-2 px-4 py-2 glass-card rounded-lg text-sm text-blue-400">
              <span class="flex gap-1">
                <span class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 0ms" />
                <span class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 150ms" />
                <span class="w-1.5 h-1.5 rounded-full bg-blue-400 animate-bounce" style="animation-delay: 300ms" />
              </span>
              {{ t('chat.stillListening') }}
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

            <!-- Continue/Regenerate buttons removed — will be replaced by suggested follow-up prompts -->
          </div>
        </template>
      </div>

      <!-- Context menu -->
      <Teleport to="body">
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
          <!-- Continue / Regenerate — only for last assistant message, not while streaming -->
          <template v-if="isContextMenuLastAssistant && !chatStore.streaming">
            <div class="border-t border-white/10 my-1" />
            <button
              class="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
              @click="handleContextContinue"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              {{ t('chat.continueGenerating') }}
            </button>
            <button
              class="w-full px-4 py-2 text-left text-sm text-gray-700 dark:text-gray-200 hover:bg-white/10 flex items-center gap-2 cursor-pointer"
              @click="handleContextRegenerate"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              {{ t('chat.regenerate') }}
            </button>
          </template>
        </div>

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

      <!-- Input area - floating at bottom (desktop), flex at bottom (mobile) -->
      <div :class="isMobile ? 'flex-shrink-0 border-t border-gray-200 dark:border-glass-border' : 'absolute bottom-0 left-0 right-0 z-10'">
        <!-- Media generation param panel -->
        <div v-if="mediaGen.showPanel.value" class="max-w-4xl mx-auto px-3 sm:px-4">
          <MediaParamPanel
            :intent="mediaGen.intent.value!"
            :models="mediaGen.models.value"
            :selected-model="mediaGen.selectedModel.value"
            :generating="mediaGen.generating.value"
            :ambiguous="mediaGen.ambiguous.value"
            @update:selected-model="mediaGen.selectedModel.value = $event"
            @generate="handleMediaGenerate"
            @dismiss="handleMediaDismiss"
            @close="handleMediaClose"
            @confirm="handleMediaConfirm"
            @switch-category="mediaGen.switchCategory($event)"
          />
        </div>
        <ChatInput
          ref="chatInputRef"
          :disabled="chatStore.sending && !chatStore.isPreTTFT"
          :streaming="chatStore.streaming"
          @send="handleSend"
          @cancel="handleCancel"
          @cancel-pre-ttft="chatStore.cancelPreTTFT()"
          @warmup="chatStore.warmupConversation()"
          @open-talk-mode="showTalkMode = true"
        />
      </div>

      <!-- Talk Mode -->
      <TalkMode
        v-model="showTalkMode"
        :conversation-id="chatStore.currentConversationId || undefined"
        :edit-before-send="editBeforeSend"
        @transcript="handleVoiceTranscript"
      />
    </main>

    <!-- Tool call approval dialog -->
    <ToolApprovalDialog />
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.chat-view {
  min-height: 0;
  flex: 1;
  height: 100%;
  overflow: hidden;
}

/* Mobile view styles */
.mobile-view {
  position: fixed;
  inset: 0;
  z-index: 10;
  background: var(--surface-base);
}

/* Mobile conversation list page */
/* Mobile conversation list page - stay in normal flow so AppHeader shows above */
.mobile-list-page {
  width: 100%;
  height: 100%;
  border: none;
  background: white;
}

:global(.dark) .mobile-list-page {
  background: rgb(17 24 39); /* dark:bg-gray-900 */
}

/* Mobile main chat area */
.mobile-chat {
  position: fixed !important;
  inset: 0;
  z-index: 20;
  background: var(--surface-base);
  flex-direction: column !important;
  transform: translateX(0);
}

.mobile-chat-animated {
  transition: transform 0.3s ease-out;
}

.mobile-chat-offscreen {
  transform: translateX(100%);
  pointer-events: none;
}

/* Mobile chat messages area - ensure scrolling works */
.mobile-chat .chat-messages-area {
  flex: 1;
  min-height: 0;
  overflow-y: auto !important;
  -webkit-overflow-scrolling: touch;
  overscroll-behavior: contain;
}

/* Mobile input area - stick to bottom */
.mobile-chat > div:last-child {
  position: relative !important;
  bottom: auto !important;
}

/* Slide animation for mobile chat */
.conversation-sidebar {
  height: 100%;
}

/* Safe area for mobile devices with notches */
@supports (padding-bottom: env(safe-area-inset-bottom)) {
  .chat-view {
    padding-bottom: env(safe-area-inset-bottom);
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
