<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Conversation } from '@/api/chat'

const { t } = useI18n()

defineProps<{
  conversations: Conversation[]
  currentId: string | null
  executingConversationIds?: string[]
  loading?: boolean
  searching?: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  create: []
  delete: [id: string]
  search: [query: string]
  pin: [id: string]
  unpin: [id: string]
}>()

const searchQuery = ref('')
const searchActive = ref(false)
const showDeleteConfirm = ref<string | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
const longPressTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const longPressId = ref<string | null>(null)
const showContextMenu = ref<string | null>(null)

function toggleSearch() {
  searchActive.value = !searchActive.value
  if (searchActive.value) {
    nextTick(() => searchInputRef.value?.focus())
  } else {
    searchQuery.value = ''
  }
}

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
  <div class="conversation-list h-full flex flex-col">
    <!-- Header: new chat + search merged into one row -->
    <div class="list-header sticky top-0 z-10 flex items-center gap-2 p-3 sm:p-4">
      <!-- Default: New Chat button / Search active: Search input -->
      <div class="flex-1 min-w-0">
        <button
          v-if="!searchActive"
          class="create-btn w-full py-2 px-3.5 text-sm text-white rounded-xl flex items-center justify-center gap-1.5 transition-all duration-200"
          @click="handleCreate"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          {{ t('chat.newChat') }}
          <span class="create-count" :title="`${conversations.length}`">{{ conversations.length }}</span>
        </button>
        <div v-else class="search-input-wrap relative">
          <input
            ref="searchInputRef"
            v-model="searchQuery"
            type="text"
            :placeholder="t('chat.searchConversations')"
            class="search-input w-full text-sm text-gray-900 dark:text-white rounded-xl px-4 py-2 pl-10 focus:outline-none"
            @keydown.escape="toggleSearch"
          />
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
      </div>
      <!-- Search / Close toggle button -->
      <button
        class="search-toggle-btn shrink-0 p-2 rounded-lg transition-colors"
        :title="searchActive ? t('common.cancel') : t('chat.searchConversations')"
        @click="toggleSearch"
      >
        <!-- Search icon -->
        <svg v-if="!searchActive" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <!-- Close icon -->
        <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Conversation list -->
    <div class="convo-scroll flex-1 overflow-y-auto">
      <!-- Loading state -->
      <div v-if="loading || searching" class="p-4 text-center text-gray-500 dark:text-gray-400">
        <div class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto" />
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
      <div v-else class="convo-stack p-2">
        <div
          v-for="(conversation, index) in conversations"
          :key="conversation.id"
          class="conversation-item relative group rounded-xl"
          :class="{
            'convo-card-active': conversation.id === currentId,
            'convo-card-running': !!executingConversationIds?.includes(conversation.id),
          }"
          :style="{ '--item-index': String(index) }"
        >
          <button
            class="convo-main-btn w-full p-3.5 text-left rounded-xl transition-all duration-200"
            @click="handleSelect(conversation.id)"
            @mousedown="startLongPress(conversation.id)"
            @mouseup="cancelLongPress"
            @mouseleave="cancelLongPress"
            @touchstart="startLongPress(conversation.id)"
            @touchend="cancelLongPress"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="flex-1 min-w-0">
                <h3 class="convo-title-row text-gray-900 dark:text-white font-medium flex items-center gap-1.5 min-w-0">
                  <!-- Pin icon inline with title -->
                  <svg
                    v-if="conversation.pinned"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-3.5 w-3.5 text-yellow-500 flex-shrink-0 transform rotate-45"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                  >
                    <path d="M16 9V4h1c.55 0 1-.45 1-1s-.45-1-1-1H7c-.55 0-1 .45-1 1s.45 1 1 1h1v5c0 1.66-1.34 3-3 3v2h5.97v7l1 1 1-1v-7H19v-2c-1.66 0-3-1.34-3-3z" />
                  </svg>
                  <span class="convo-title-scroll" :title="conversation.title">
                    <span class="convo-title">{{ conversation.title }}</span>
                  </span>
                  <span
                    v-if="conversation.pinned"
                    class="pin-chip hidden sm:inline-flex"
                  >
                    {{ t('chat.pinConversation') }}
                  </span>
                </h3>
                <p class="convo-meta text-sm text-gray-500 dark:text-gray-400 mt-1">
                  {{ formatDate(conversation.updated_at) }}
                </p>
              </div>
            </div>
          </button>

          <!-- Pin/Unpin and Delete buttons -->
          <div class="convo-actions absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1 transition-opacity">
            <!-- Pin button -->
            <button
              class="action-btn p-2 text-gray-400 hover:text-yellow-500 transition-colors"
              :title="conversation.pinned ? t('chat.unpinConversation') : t('chat.pinConversation')"
              @click.stop="handlePin(conversation.id, conversation.pinned || false)"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5 transform rotate-45"
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
              class="action-btn p-2 text-gray-400 hover:text-red-500 transition-colors"
              :title="t('chat.deleteConversation')"
              @click.stop="handleDelete(conversation.id)"
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
            <span class="text-sm text-gray-600 dark:text-gray-300">{{ t('chat.confirmDelete') }}</span>
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
                v-for="conv in conversations.filter(c => c.id === showContextMenu)"
                :key="conv.id"
                class="w-full flex items-center gap-3 px-4 py-3 rounded-xl hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="handlePin(conv.id, conv.pinned || false); showContextMenu = null"
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
                @click="handleDelete(showContextMenu); showContextMenu = null"
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
  --cl-border-soft: var(--chat-sidebar-border-soft, rgba(148, 163, 184, 0.22));
  --cl-border-strong: var(--chat-sidebar-border-strong, rgba(56, 189, 248, 0.42));
  --cl-card-bg: var(--chat-sidebar-card-bg, rgba(15, 23, 42, 0.18));
  --cl-card-bg-hover: var(--chat-sidebar-card-bg-hover, rgba(30, 41, 59, 0.42));
  --cl-card-bg-active: var(--chat-sidebar-card-bg-active, linear-gradient(135deg, rgba(14, 116, 144, 0.25), rgba(15, 23, 42, 0.5)));
  --cl-header-py: 0.85rem;
  --cl-header-px: 0.9rem;
  --cl-item-py: 0.8rem;
  --cl-item-px: 0.85rem;
  --cl-radius: 0.75rem;
  background: var(--chat-sidebar-bg, linear-gradient(180deg, rgba(15, 23, 42, 0.32), rgba(15, 23, 42, 0.12)));
  overflow-x: hidden;
}

:global(.chat-view.ui-density-compact) .conversation-list {
  --cl-header-py: 0.65rem;
  --cl-header-px: 0.75rem;
  --cl-item-py: 0.65rem;
  --cl-item-px: 0.72rem;
  --cl-radius: 0.65rem;
}

:global(.chat-view.ui-density-comfortable) .conversation-list {
  --cl-header-py: 1rem;
  --cl-header-px: 1.05rem;
  --cl-item-py: 0.95rem;
  --cl-item-px: 1rem;
  --cl-radius: 0.85rem;
}

.list-header {
  border-bottom: 1px solid var(--cl-border-soft);
  background: var(--chat-sidebar-header-bg, linear-gradient(180deg, rgba(15, 23, 42, 0.42), rgba(15, 23, 42, 0.24)));
  backdrop-filter: blur(10px);
  position: relative;
  padding: var(--cl-header-py) var(--cl-header-px);
}

.list-header::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(56, 189, 248, 0.38), transparent);
  pointer-events: none;
}

.create-btn {
  border-radius: var(--cl-radius);
  background: var(--chat-sidebar-create-bg, linear-gradient(135deg, #0f172a, #334155));
  box-shadow: 0 7px 16px rgba(15, 23, 42, 0.24);
}

.create-count {
  min-width: 1.5rem;
  height: 1.5rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  font-size: 0.68rem;
  font-weight: 700;
  color: rgb(203 213 225);
  border: 1px solid rgba(148, 163, 184, 0.3);
  background: rgba(15, 23, 42, 0.34);
}

.create-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 20px rgba(15, 23, 42, 0.28);
}

.search-input-wrap {
  border: 1px solid rgba(148, 163, 184, 0.26);
  border-radius: var(--cl-radius);
  background: rgba(255, 255, 255, 0.06);
}

.search-input-wrap:focus-within {
  border-color: rgba(125, 211, 252, 0.52);
}

.search-input {
  background: transparent;
}

.search-input:focus {
  box-shadow: 0 0 0 2px rgba(125, 211, 252, 0.35);
}

.search-toggle-btn {
  color: rgb(148 163 184);
}

.search-toggle-btn:hover {
  color: rgb(226 232 240);
  background: rgba(148, 163, 184, 0.14);
}

.convo-stack {
  display: grid;
  gap: 0.4rem;
  padding-right: 0.35rem;
}

.convo-scroll {
  scrollbar-gutter: stable;
  overflow-x: hidden;
}

.conversation-item {
  animation: convo-fade-in 0.24s ease-out both;
  animation-delay: calc(var(--item-index, 0) * 20ms);
  min-width: 0;
}

.convo-main-btn {
  background: var(--cl-card-bg);
  border: 1px solid transparent;
  position: relative;
  overflow: hidden;
  border-radius: var(--cl-radius);
  padding: var(--cl-item-py) var(--cl-item-px);
  min-width: 0;
  max-width: 100%;
}

.convo-main-btn::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(130deg, rgba(125, 211, 252, 0.06), transparent 34%, rgba(45, 212, 191, 0.06));
  opacity: 0;
  transition: opacity 0.2s ease;
  pointer-events: none;
}

.convo-main-btn:hover {
  background: var(--cl-card-bg-hover);
  border-color: rgba(148, 163, 184, 0.2);
  transform: translateX(1px);
}

.convo-main-btn:hover::before {
  opacity: 1;
}

.convo-card-active .convo-main-btn {
  background: var(--cl-card-bg-active);
  border-color: var(--cl-border-strong);
  box-shadow: 0 12px 22px rgba(2, 132, 199, 0.2);
}

.convo-card-running .convo-main-btn::after {
  content: '';
  position: absolute;
  right: 0.4rem;
  top: 0.4rem;
  width: 0.36rem;
  height: 0.36rem;
  border-radius: 999px;
  background: rgb(56 189 248);
  box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.14);
}

.conversation-item::before {
  content: '';
  position: absolute;
  left: 0.1rem;
  top: 18%;
  bottom: 18%;
  width: 2px;
  border-radius: 999px;
  background: linear-gradient(180deg, #38bdf8, #22d3ee);
  opacity: 0;
  transform: scaleY(0.6);
  transition: all 0.2s ease;
}

.convo-card-active::before {
  opacity: 0.95;
  transform: scaleY(1);
}

.convo-title {
  display: block;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: 0.01em;
}

.convo-title-scroll {
  flex: 1;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
}

.convo-meta {
  font-size: 0.76rem;
}

.pin-chip {
  font-size: 0.62rem;
  line-height: 1;
  padding: 0.22rem 0.38rem;
  border-radius: 999px;
  color: rgb(250 204 21);
  background: rgba(234, 179, 8, 0.16);
  border: 1px solid rgba(234, 179, 8, 0.34);
}

.convo-actions {
  opacity: 0;
}

.group:hover .convo-actions,
.convo-card-active .convo-actions {
  opacity: 1;
}

.action-btn {
  border-radius: 0.55rem;
}

.action-btn:hover {
  background: rgba(148, 163, 184, 0.14);
}

.delete-mask {
  background: rgba(15, 23, 42, 0.9);
}

.empty-state {
  border-radius: 0.85rem;
  border: 1px dashed rgba(148, 163, 184, 0.34);
  background: rgba(15, 23, 42, 0.22);
}

.conversation-list::-webkit-scrollbar {
  width: 6px;
}

.conversation-list::-webkit-scrollbar-track {
  background: transparent;
}

.conversation-list::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.42);
  border-radius: 3px;
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

:root.light .create-count,
[data-theme="light"] .create-count {
  color: rgb(71 85 105);
  border-color: rgba(148, 163, 184, 0.34);
  background: rgba(255, 255, 255, 0.88);
}

:root.light .search-input-wrap,
[data-theme="light"] .search-input-wrap {
  background: rgba(255, 255, 255, 0.88);
  border-color: rgba(148, 163, 184, 0.3);
}

:root.light .search-toggle-btn,
[data-theme="light"] .search-toggle-btn {
  color: rgb(100 116 139);
}

:root.light .search-toggle-btn:hover,
[data-theme="light"] .search-toggle-btn:hover {
  color: rgb(30 41 59);
  background: rgba(148, 163, 184, 0.16);
}

:root.light .convo-main-btn:hover,
[data-theme="light"] .convo-main-btn:hover {
  border-color: rgba(148, 163, 184, 0.35);
}

:root.light .pin-chip,
[data-theme="light"] .pin-chip {
  background: rgba(250, 204, 21, 0.12);
  border-color: rgba(245, 158, 11, 0.3);
  color: rgb(180 83 9);
}

:root.light .delete-mask,
[data-theme="light"] .delete-mask {
  background: rgba(248, 250, 252, 0.96);
}

:root.light .empty-state,
[data-theme="light"] .empty-state {
  border-color: rgba(148, 163, 184, 0.38);
  background: rgba(255, 255, 255, 0.86);
}

@media (prefers-reduced-motion: reduce) {
  .conversation-item {
    animation: none !important;
  }

  .convo-main-btn,
  .create-btn,
  .search-toggle-btn,
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
