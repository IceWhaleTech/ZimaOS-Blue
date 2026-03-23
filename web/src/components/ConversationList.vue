<script setup lang="ts">
import { computed, inject, ref, watch, type ComputedRef } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Conversation } from '@/api/chat'

const { t } = useI18n()

defineProps<{
  conversations: Conversation[]
  currentId: string | null
  executingConversationIds?: string[]
  loading?: boolean
  searching?: boolean
  mobile?: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  create: []
  delete: [id: string]
  search: [query: string]
  pin: [id: string]
  unpin: [id: string]
  'more-actions': []
}>()

const searchQuery = ref('')
const showDeleteConfirm = ref<string | null>(null)
const longPressTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const longPressId = ref<string | null>(null)
const showContextMenu = ref<string | null>(null)
const toggleAppSidebar = inject<() => void>('toggleAppSidebar', () => {})
const hasGlobalMobileSidebarToggle = inject<ComputedRef<boolean>>(
  'hasGlobalMobileSidebarToggle',
  computed(() => false)
)

function formatDate(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))

  if (days === 0) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } else if (days === 1) {
    return t('chat.yesterday')
  } else if (days < 7) {
    return date.toLocaleDateString([], { weekday: 'short' })
  } else {
    return date.toLocaleDateString([], { month: 'short', day: 'numeric' })
  }
}

function handleSelect(id: string) {
  emit('select', id)
}

function handleCreate() {
  emit('create')
}

function handleOpenAppSidebar() {
  toggleAppSidebar()
}

function handleOpenMoreActions() {
  emit('more-actions')
}

function handleDelete(id: string) {
  showDeleteConfirm.value = id
}

function confirmDelete(id: string) {
  emit('delete', id)
  showDeleteConfirm.value = null
}

function cancelDelete() {
  showDeleteConfirm.value = null
}

function handlePin(id: string, isPinned: boolean) {
  if (isPinned) {
    emit('unpin', id)
  } else {
    emit('pin', id)
  }
}

function handlePinFromContextMenu(id: string, isPinned: boolean) {
  handlePin(id, isPinned)
  showContextMenu.value = null
}

function handleDeleteFromContextMenu() {
  if (!showContextMenu.value) return
  handleDelete(showContextMenu.value)
  showContextMenu.value = null
}

function getConversationPreview(title: string): string {
  const compact = String(title || '')
    .replace(/\s+/g, ' ')
    .trim()
  if (!compact) return ''
  if (compact.length <= 42) return `${compact}...`
  return `${compact.slice(0, 42)}...`
}

// Long press handling for mobile - show bottom sheet
function startLongPress(id: string) {
  longPressId.value = id
  longPressTimer.value = setTimeout(() => {
    showContextMenu.value = id
  }, 500) // 500ms long press
}

function cancelLongPress() {
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
    longPressTimer.value = null
  }
  longPressId.value = null
}

// Debounced search
let searchTimeout: ReturnType<typeof setTimeout> | null = null
watch(searchQuery, (query) => {
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  searchTimeout = setTimeout(() => {
    emit('search', query)
  }, 300)
})
</script>

<template>
  <div
    class="conversation-list h-full flex flex-col"
    :class="{ 'conversation-list-mobile': mobile }"
  >
    <div class="list-header sticky top-0 z-10">
      <div v-if="mobile" class="list-headline">
        <h2 class="list-title truncate">{{ t('nav.chat') }}</h2>
        <div class="list-actions">
          <button
            v-if="!hasGlobalMobileSidebarToggle"
            class="list-menu-btn"
            type="button"
            :aria-label="t('nav.expandSidebar')"
            :title="t('nav.expandSidebar')"
            @click="handleOpenAppSidebar"
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
                stroke-width="1.8"
                d="M4 6h16M4 12h16M4 18h16"
              />
            </svg>
          </button>
          <button
            class="list-menu-btn"
            type="button"
            :aria-label="t('chat.moreActions')"
            :title="t('chat.moreActions')"
            @click="handleOpenMoreActions"
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
                stroke-width="1.8"
                d="M4.75 5.75h4.5v4.5h-4.5zm10 0h4.5v4.5h-4.5zm-10 10h4.5v4.5h-4.5zm10 0h4.5v4.5h-4.5z"
              />
            </svg>
          </button>
        </div>
      </div>

      <div class="search-row">
        <div class="search-input-wrap relative min-w-0 flex-1">
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('chat.searchConversations')"
            class="search-input w-full text-gray-900 dark:text-white focus:outline-none"
          />
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
          <button
            v-if="searchQuery"
            class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded-md text-gray-500 hover:text-gray-800 dark:text-slate-400 dark:hover:text-white hover:bg-gray-200/70 dark:hover:bg-slate-600/70 transition-colors"
            :title="t('common.clear')"
            @click="searchQuery = ''"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
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
        </div>
        <button
          class="create-btn create-btn-inline transition-all duration-200"
          :title="t('chat.newChat')"
          @click="handleCreate"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2.3"
              d="M12 4v16m8-8H4"
            />
          </svg>
        </button>
      </div>
    </div>

    <!-- Conversation list -->
    <div class="convo-scroll flex-1 overflow-y-auto">
      <!-- Loading state -->
      <div v-if="loading || searching" class="p-4 text-center text-gray-500 dark:text-gray-400">
        <div
          class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"
        />
        <p class="mt-2">{{ searching ? t('chat.searching') : t('common.loading') }}</p>
      </div>

      <!-- Empty state -->
      <div
        v-else-if="conversations.length === 0"
        class="empty-state m-3 p-4 text-center text-gray-500 dark:text-gray-400"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-12 w-12 mx-auto mb-2 opacity-50"
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
        <p v-if="searchQuery">{{ t('chat.noConversationsFound') }}</p>
        <p v-else>{{ t('chat.noConversationsYet') }}</p>
        <p class="text-sm mt-1">{{ t('chat.startNewChat') }}</p>
      </div>

      <!-- Conversation items -->
      <div v-else class="convo-stack">
        <div
          v-for="(conversation, index) in conversations"
          :key="conversation.id"
          class="conversation-item relative group"
          :class="{
            'convo-card-active': conversation.id === currentId,
            'convo-card-running': !!executingConversationIds?.includes(conversation.id),
          }"
          :style="{ '--item-index': String(index) }"
        >
          <button
            class="convo-main-btn w-full text-left transition-all duration-200"
            @click="handleSelect(conversation.id)"
            @mousedown="startLongPress(conversation.id)"
            @mouseup="cancelLongPress"
            @mouseleave="cancelLongPress"
            @touchstart="startLongPress(conversation.id)"
            @touchend="cancelLongPress"
          >
            <div class="convo-row flex justify-between">
              <div class="flex-1 min-w-0">
                <h3
                  class="convo-title-row text-gray-900 dark:text-white font-medium flex items-center gap-1.5 min-w-0"
                >
                  <svg
                    v-if="conversation.pinned"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-3.5 w-3.5 text-yellow-500 flex-shrink-0 transform rotate-45"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                  >
                    <path
                      d="M16 9V4h1c.55 0 1-.45 1-1s-.45-1-1-1H7c-.55 0-1 .45-1 1s.45 1 1 1h1v5c0 1.66-1.34 3-3 3v2h5.97v7l1 1 1-1v-7H19v-2c-1.66 0-3-1.34-3-3z"
                    />
                  </svg>
                  <span class="convo-title-scroll" :title="conversation.title">
                    <span class="convo-title">{{ conversation.title }}</span>
                  </span>
                </h3>
                <p class="convo-preview text-gray-500 dark:text-gray-400">
                  {{ getConversationPreview(conversation.title) }}
                </p>
              </div>
              <div class="convo-side">
                <p class="convo-time text-gray-400 dark:text-slate-500">
                  {{ formatDate(conversation.updated_at) }}
                </p>
                <span class="convo-actions-slot" aria-hidden="true" />
              </div>
            </div>
          </button>

          <!-- Pin/Unpin and Delete buttons -->
          <div class="convo-actions absolute flex items-center gap-0.5 transition-opacity">
            <!-- Pin button -->
            <button
              class="action-btn p-1.5 text-gray-400 hover:text-yellow-500 transition-colors"
              :title="conversation.pinned ? t('chat.unpinConversation') : t('chat.pinConversation')"
              @click.stop="handlePin(conversation.id, conversation.pinned || false)"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4 transform rotate-45"
                :class="{ 'text-yellow-500': conversation.pinned }"
                viewBox="0 0 24 24"
                :fill="conversation.pinned ? 'currentColor' : 'none'"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M16 9V4h1c.55 0 1-.45 1-1s-.45-1-1-1H7c-.55 0-1 .45-1 1s.45 1 1 1h1v5c0 1.66-1.34 3-3 3v2h5.97v7l1 1 1-1v-7H19v-2c-1.66 0-3-1.34-3-3z"
                />
              </svg>
            </button>
            <!-- Delete button -->
            <button
              class="action-btn p-1.5 text-gray-400 hover:text-red-500 transition-colors"
              :title="t('chat.deleteConversation')"
              @click.stop="handleDelete(conversation.id)"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            </button>
          </div>

          <!-- Delete confirmation -->
          <div
            v-if="showDeleteConfirm === conversation.id"
            class="delete-mask absolute inset-0 flex items-center justify-center gap-2 p-2 rounded-xl"
          >
            <span class="text-sm text-gray-600 dark:text-gray-300">{{
              t('chat.confirmDelete')
            }}</span>
            <button
              class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-sm rounded transition-colors"
              @click.stop="confirmDelete(conversation.id)"
            >
              {{ t('common.yes') }}
            </button>
            <button
              class="px-3 py-1 bg-gray-100 dark:bg-gray-700/30 hover:bg-gray-200 dark:hover:bg-gray-700/50 text-gray-700 dark:text-gray-300 text-sm rounded transition-colors"
              @click.stop="cancelDelete"
            >
              {{ t('common.no') }}
            </button>
          </div>

          <!-- Long press context menu - removed, now using bottom sheet -->
        </div>
      </div>
    </div>

    <!-- Bottom sheet for long press actions (teleported to body) -->
    <Teleport to="body">
      <Transition name="sheet">
        <div
          v-if="showContextMenu"
          class="fixed inset-0 z-[200] flex items-end"
          @click="showContextMenu = null"
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
              <!-- Pin/Unpin action -->
              <button
                v-for="conv in conversations.filter((c) => c.id === showContextMenu)"
                :key="conv.id"
                class="w-full flex items-center gap-3 px-4 py-3 rounded-xl hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handlePinFromContextMenu(conv.id, conv.pinned || false)"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 transform rotate-45"
                  :class="conv.pinned ? 'text-yellow-500' : 'text-gray-600 dark:text-gray-400'"
                  viewBox="0 0 24 24"
                  :fill="conv.pinned ? 'currentColor' : 'none'"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M16 9V4h1c.55 0 1-.45 1-1s-.45-1-1-1H7c-.55 0-1 .45-1 1s.45 1 1 1h1v5c0 1.66-1.34 3-3 3v2h5.97v7l1 1 1-1v-7H19v-2c-1.66 0-3-1.34-3-3z"
                  />
                </svg>
                <span class="text-base font-medium text-gray-900 dark:text-white">
                  {{ conv.pinned ? t('chat.unpinConversation') : t('chat.pinConversation') }}
                </span>
              </button>
              <!-- Delete action -->
              <button
                class="w-full flex items-center gap-3 px-4 py-3 rounded-xl hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                @click="handleDeleteFromContextMenu"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 text-red-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
                <span class="text-base font-medium text-red-500">
                  {{ t('chat.deleteConversation') }}
                </span>
              </button>
              <!-- Cancel button -->
              <button
                class="w-full px-4 py-3 mt-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-900 dark:text-white text-base font-medium rounded-xl transition-colors"
                @click="showContextMenu = null"
              >
                {{ t('common.cancel') }}
              </button>
            </div>

            <!-- Safe area padding for devices with notches -->
            <div class="h-[env(safe-area-inset-bottom)]" />
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.conversation-list {
  --cl-border: rgba(226, 232, 240, 0.92);
  --cl-border-strong: rgba(59, 130, 246, 0.34);
  --cl-header-px: 0.82rem;
  --cl-header-py: var(--chat-pane-pad-y, 0.96rem);
  --cl-header-row-height: var(--chat-header-row-height, 3.85rem);
  --cl-header-row-pad-y: var(--chat-header-row-pad-y, 0.72rem);
  --cl-header-row-block-size: var(--chat-header-row-block-size, 5.29rem);
  --cl-header-control-size: var(--chat-header-control-size, 1.96rem);
  --cl-item-px: 0.82rem;
  --cl-item-py: 1rem;
  --cl-side-width: 3.75rem;
  background: rgba(255, 255, 255, 0.98);
  overflow-x: hidden;
}

:global(.chat-view.ui-density-compact) .conversation-list {
  --cl-header-px: 0.72rem;
  --cl-header-py: 0.76rem;
  --cl-item-px: 0.72rem;
  --cl-item-py: 0.8rem;
  --cl-side-width: 3.4rem;
}

:global(.chat-view.ui-density-comfortable) .conversation-list {
  --cl-header-px: 0.92rem;
  --cl-header-py: 1.08rem;
  --cl-item-px: 0.92rem;
  --cl-item-py: 1.08rem;
  --cl-side-width: 4rem;
}

.list-header {
  position: sticky;
  top: 0;
  z-index: 10;
  background: rgba(255, 255, 255, 0.96);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

.list-headline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.7rem;
  min-height: 3.35rem;
  padding: calc(max(env(safe-area-inset-top), 0px) + 0.72rem) var(--cl-header-px) 0.72rem;
}

.list-title {
  font-size: clamp(1.48rem, 5.7vw, 1.9rem);
  line-height: 0.94;
  font-weight: 700;
  letter-spacing: -0.045em;
  color: rgb(15, 23, 42);
}

.list-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  flex-shrink: 0;
}

.list-menu-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.3rem;
  height: 2.3rem;
  flex-shrink: 0;
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 0.78rem;
  background: rgba(255, 255, 255, 0.94);
  color: rgb(100, 116, 139);
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

.list-menu-btn:hover {
  background: rgba(248, 250, 252, 0.98);
  border-color: rgba(148, 163, 184, 0.45);
  color: rgb(30, 41, 59);
  transform: translateY(-1px);
}

.search-row {
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 0.54rem;
  height: var(--cl-header-row-block-size);
  min-height: var(--cl-header-row-block-size);
  padding: var(--cl-header-row-pad-y) var(--cl-header-px);
  border-top: none;
  box-shadow: inset 0 -1px 0 var(--cl-border);
}

.conversation-list-mobile .search-row {
  border-top: 1px solid var(--cl-border);
}

.search-input-wrap {
  border: 1px solid var(--cl-border);
  border-radius: 0.65rem;
  background: rgba(255, 255, 255, 0.98);
  overflow: hidden;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.search-input-wrap:focus-within {
  border-color: rgba(59, 130, 246, 0.42);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.08);
}

.search-input {
  border: none;
  border-radius: inherit;
  background: transparent;
  box-shadow: none;
  min-height: var(--cl-header-control-size);
  outline: none;
  appearance: none;
  -webkit-appearance: none;
  padding: 0.46rem 1.92rem 0.46rem 2.18rem;
  font-size: 0.82rem;
  line-height: 1.15;
}

.search-input:focus,
.search-input:focus-visible {
  outline: none;
  box-shadow: none;
}

.search-input::placeholder {
  color: rgba(148, 163, 184, 0.98);
}

.create-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.58rem;
  background: rgb(37, 99, 235);
  box-shadow: none;
  color: rgb(255, 255, 255);
  border: 1px solid rgba(37, 99, 235, 0.2);
}

.create-btn-inline {
  width: var(--cl-header-control-size);
  height: var(--cl-header-control-size);
  flex-shrink: 0;
}

.create-btn:hover {
  transform: translateY(-1px);
  filter: brightness(1.04);
}

.convo-scroll {
  overflow-x: hidden;
  scrollbar-gutter: stable;
  background: rgba(255, 255, 255, 0.98);
}

.convo-stack {
  display: block;
  padding: 0 0 1rem;
}

.conversation-item {
  position: relative;
  min-width: 0;
  border-bottom: 1px solid var(--cl-border);
  animation: convo-fade-in 0.2s ease-out both;
  animation-delay: calc(var(--item-index, 0) * 12ms);
}

.convo-main-btn {
  position: relative;
  width: 100%;
  min-width: 0;
  min-height: 3.55rem;
  padding: 0.7rem var(--cl-item-px) 0.64rem;
  border: none;
  border-radius: 0;
  background: transparent;
  transition: background-color 0.18s ease;
}

.convo-main-btn:hover {
  background: rgba(248, 250, 252, 0.96);
}

.convo-card-active .convo-main-btn {
  background: rgba(239, 246, 255, 0.9);
}

.conversation-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0.58rem;
  bottom: 0.58rem;
  width: 3px;
  border-radius: 999px;
  background: rgb(59, 130, 246);
  opacity: 0;
  transition: opacity 0.18s ease;
}

.convo-card-active::before {
  opacity: 1;
}

.convo-title {
  display: block;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: -0.015em;
  font-size: 0.9rem;
  line-height: 1.18;
  font-weight: 600;
}

.convo-title-scroll {
  flex: 1;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
}

.convo-row {
  align-items: flex-start;
  gap: 0.8rem;
  height: 100%;
}

.convo-preview {
  margin-top: 0.08rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.69rem;
  line-height: 1.22;
  letter-spacing: 0.01em;
  color: rgb(156, 163, 175);
}

.convo-side {
  display: flex;
  flex-direction: column;
  flex: 0 0 var(--cl-side-width);
  width: var(--cl-side-width);
  min-width: var(--cl-side-width);
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.16rem;
}

.convo-time {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.32rem;
  padding-top: 0.08rem;
  font-size: 0.68rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  text-align: right;
  white-space: nowrap;
  color: rgb(156, 163, 175);
}

.convo-actions-slot {
  width: 1.8rem;
  height: 1.35rem;
  flex-shrink: 0;
}

.convo-card-running .convo-time::before {
  content: '';
  width: 0.32rem;
  height: 0.32rem;
  border-radius: 999px;
  background: rgb(59 130 246);
  flex-shrink: 0;
}

.convo-actions {
  right: 0.72rem;
  bottom: 0.56rem;
  gap: 0.22rem;
  opacity: 0;
  transition: opacity 0.18s ease;
}

.group:hover .convo-actions {
  opacity: 1;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.6rem;
  height: 1.6rem;
  padding: 0;
  border-radius: 0.4rem;
  border: 1px solid var(--cl-border);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: none;
}

.action-btn:hover {
  background: rgba(255, 255, 255, 1);
  border-color: rgba(148, 163, 184, 0.5);
}

.delete-mask {
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid var(--cl-border);
  border-radius: 0.8rem;
  backdrop-filter: blur(8px);
}

.empty-state {
  margin: 0.8rem;
  padding: 1.1rem;
  border-radius: 0.95rem;
  border: 1px dashed rgba(203, 213, 225, 0.9);
  background: rgba(248, 250, 252, 0.72);
}

.convo-scroll::-webkit-scrollbar {
  width: 8px;
}

.convo-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.convo-scroll::-webkit-scrollbar-thumb {
  background: rgba(203, 213, 225, 0.82);
  border: 2px solid transparent;
  border-radius: 999px;
  background-clip: padding-box;
}

@keyframes convo-fade-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

:root.dark .conversation-list,
[data-theme='dark'] .conversation-list {
  --cl-border: rgba(51, 65, 85, 0.88);
  background: rgba(15, 23, 42, 0.96);
}

:root.dark .list-header,
[data-theme='dark'] .list-header {
  background: rgba(15, 23, 42, 0.92);
}

:root.dark .list-title,
[data-theme='dark'] .list-title {
  color: rgb(241, 245, 249);
}

:root.dark .list-menu-btn,
[data-theme='dark'] .list-menu-btn {
  border-color: rgba(51, 65, 85, 0.88);
  background: rgba(30, 41, 59, 0.88);
  color: rgb(148, 163, 184);
}

:root.dark .list-menu-btn:hover,
[data-theme='dark'] .list-menu-btn:hover {
  border-color: rgba(100, 116, 139, 0.72);
  background: rgba(51, 65, 85, 0.94);
  color: rgb(241, 245, 249);
}

:root.dark .search-input-wrap,
[data-theme='dark'] .search-input-wrap {
  background: rgba(30, 41, 59, 0.82);
}

:root.dark .search-input::placeholder,
[data-theme='dark'] .search-input::placeholder {
  color: rgba(148, 163, 184, 0.95);
}

:root.dark .convo-scroll,
[data-theme='dark'] .convo-scroll {
  background: rgba(15, 23, 42, 0.96);
}

:root.dark .convo-main-btn:hover,
[data-theme='dark'] .convo-main-btn:hover {
  background: rgba(30, 41, 59, 0.76);
}

:root.dark .convo-card-active .convo-main-btn,
[data-theme='dark'] .convo-card-active .convo-main-btn {
  background: rgba(8, 47, 73, 0.56);
}

:root.dark .convo-title-row,
[data-theme='dark'] .convo-title-row {
  color: rgb(241, 245, 249);
}

:root.dark .convo-preview,
[data-theme='dark'] .convo-preview {
  color: rgb(148, 163, 184);
}

:root.dark .convo-time,
[data-theme='dark'] .convo-time {
  color: rgb(148, 163, 184);
}

:root.dark .action-btn,
[data-theme='dark'] .action-btn {
  background: rgba(30, 41, 59, 0.92);
}

:root.dark .empty-state,
[data-theme='dark'] .empty-state {
  border-color: rgba(71, 85, 105, 0.82);
  background: rgba(15, 23, 42, 0.68);
}

@media (max-width: 768px) {
  .list-header {
    --cl-header-px: 0.85rem;
  }

  .convo-main-btn {
    min-height: 3.35rem;
  }

  .convo-actions {
    right: 0.7rem;
    bottom: 0.5rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .conversation-item {
    animation: none !important;
  }

  .convo-main-btn,
  .search-input-wrap,
  .create-btn,
  .action-btn {
    transition: none !important;
  }
}

/* Bottom sheet animation */
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
</style>
