<script setup lang="ts">
import { ref, watch } from 'vue'
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
const showDeleteConfirm = ref<string | null>(null)
const longPressTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const longPressId = ref<string | null>(null)
const showContextMenu = ref<string | null>(null)

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
  <div class="conversation-list h-full flex flex-col">
    <div class="list-header sticky top-0 z-10">
      <div class="list-headline">
        <h2 class="list-title truncate">{{ t('chat.conversations') }}</h2>
        <span class="list-count">{{ conversations.length }}</span>
      </div>

      <div class="search-row">
        <div class="search-input-wrap relative flex-1">
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('chat.searchConversations')"
            class="search-input w-full text-gray-900 dark:text-white focus:outline-none"
          />
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
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
  --cl-border-soft: rgba(148, 163, 184, 0.22);
  --cl-border-strong: rgba(14, 165, 233, 0.4);
  --cl-card-bg: transparent;
  --cl-card-bg-hover: rgba(241, 245, 249, 0.72);
  --cl-card-bg-active: rgba(219, 234, 254, 0.42);
  --cl-header-py: calc(var(--chat-pane-pad-y, 0.96rem) - 0.02rem);
  --cl-header-px: var(--chat-pane-pad-x, 1rem);
  --cl-item-py: 0.6rem;
  --cl-item-px: 1rem;
  --cl-actions-width: 2.75rem;
  --cl-side-width: 4.5rem;
  --cl-radius: 0.95rem;
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.95), rgba(244, 247, 251, 0.97));
  overflow-x: hidden;
}

:global(.chat-view.ui-density-compact) .conversation-list {
  --cl-header-py: 0.72rem;
  --cl-header-px: 0.82rem;
  --cl-item-py: 0.48rem;
  --cl-item-px: 0.82rem;
  --cl-actions-width: 2.55rem;
  --cl-side-width: 4.15rem;
  --cl-radius: 0.8rem;
}

:global(.chat-view.ui-density-comfortable) .conversation-list {
  --cl-header-py: 1.08rem;
  --cl-header-px: 1.12rem;
  --cl-item-py: 0.7rem;
  --cl-item-px: 1.08rem;
  --cl-actions-width: 2.9rem;
  --cl-side-width: 4.8rem;
  --cl-radius: 1rem;
}

.list-header {
  border-bottom: 1px solid var(--cl-border-soft);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(249, 250, 251, 0.96));
  backdrop-filter: blur(8px);
  position: relative;
  padding: var(--cl-header-py) var(--cl-header-px);
}

.list-headline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.7rem;
  margin-bottom: 0.78rem;
}

.list-header::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(148, 163, 184, 0.32), transparent);
  pointer-events: none;
}

.list-title {
  font-size: 0.76rem;
  line-height: 1;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(100, 116, 139);
}

.list-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.55rem;
  height: 1.55rem;
  border-radius: 999px;
  padding: 0 0.42rem;
  font-size: 0.68rem;
  font-weight: 700;
  color: rgb(71, 85, 105);
  background: rgba(226, 232, 240, 0.9);
}

.search-row {
  display: flex;
  align-items: center;
  gap: 0.72rem;
}

.create-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.88rem;
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.96), rgba(29, 78, 216, 0.96));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 16px 28px -22px rgba(37, 99, 235, 0.74);
  color: #eff6ff;
  border: 1px solid rgba(37, 99, 235, 0.28);
}

.create-btn-inline {
  width: 2.7rem;
  height: 2.7rem;
  flex-shrink: 0;
}

.create-btn:hover {
  transform: translateY(-1px);
  filter: brightness(1.03);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.22),
    0 18px 30px -22px rgba(37, 99, 235, 0.84);
}

.search-input-wrap {
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: var(--cl-radius);
  background: rgba(255, 255, 255, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.68);
  overflow: hidden;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.search-input-wrap:focus-within {
  border-color: rgba(56, 189, 248, 0.45);
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.11);
}

.search-input {
  border: none;
  border-radius: inherit;
  background: transparent;
  box-shadow: none;
  min-height: 2.7rem;
  outline: none;
  appearance: none;
  -webkit-appearance: none;
  padding: 0.76rem 2.35rem 0.76rem 2.65rem;
  font-size: 0.95rem;
  line-height: 1.2;
}

.search-input:focus,
.search-input:focus-visible {
  outline: none;
  box-shadow: none;
}

.search-input::placeholder {
  color: rgba(100, 116, 139, 0.95);
}

.convo-stack {
  display: grid;
  gap: 0.22rem;
  padding: 0.55rem 0.5rem 0.72rem;
  padding-right: 0.36rem;
}

.convo-scroll {
  scrollbar-gutter: stable;
  overflow-x: hidden;
}

.conversation-item {
  animation: convo-fade-in 0.24s ease-out both;
  animation-delay: calc(var(--item-index, 0) * 14ms);
  min-width: 0;
}

.convo-main-btn {
  background: var(--cl-card-bg);
  border: 1px solid transparent;
  position: relative;
  overflow: hidden;
  border-radius: var(--cl-radius);
  padding: var(--cl-item-py) var(--cl-item-px);
  min-height: 3.72rem;
  min-width: 0;
  max-width: 100%;
}

.convo-main-btn::before {
  content: none;
}

.convo-main-btn:hover {
  background: var(--cl-card-bg-hover);
  border-color: transparent;
}

.convo-card-active .convo-main-btn {
  background: var(--cl-card-bg-active);
  border-color: rgba(125, 211, 252, 0.28);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.72),
    0 14px 24px -24px rgba(14, 165, 233, 0.36);
}

.conversation-item::before {
  content: '';
  position: absolute;
  left: 0.14rem;
  top: 0.62rem;
  bottom: 0.62rem;
  width: 3px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(56, 189, 248, 0.9), rgba(14, 165, 233, 0.75));
  opacity: 0;
  transform: scaleY(0.7);
  transition: all 0.2s ease;
}

.convo-card-active::before {
  opacity: 0.62;
  transform: scaleY(1);
}

.conversation-item:not(:last-child)::after {
  content: '';
  position: absolute;
  left: 1.08rem;
  right: 0.92rem;
  bottom: -0.04rem;
  height: 1px;
  background: rgba(203, 213, 225, 0.6);
  pointer-events: none;
}

.convo-title {
  display: block;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: -0.015em;
  font-size: 1rem;
  line-height: 1.28;
}

.convo-title-scroll {
  flex: 1;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
}

.convo-row {
  align-items: stretch;
  gap: 0.82rem;
  height: 100%;
}

.convo-preview {
  line-height: 1.28;
  letter-spacing: 0.01em;
  opacity: 0.72;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 0.12rem;
  font-size: 0.73rem;
}

.convo-side {
  display: flex;
  flex-direction: column;
  flex: 0 0 var(--cl-side-width);
  width: var(--cl-side-width);
  min-width: var(--cl-side-width);
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.28rem;
}

.convo-time {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.32rem;
  box-sizing: border-box;
  padding-top: 0.02rem;
  font-size: 0.74rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  text-align: right;
  white-space: nowrap;
}

.convo-actions-slot {
  width: var(--cl-actions-width);
  height: 1.28rem;
  flex-shrink: 0;
}

.convo-card-running .convo-time::before {
  content: '';
  width: 0.34rem;
  height: 0.34rem;
  border-radius: 999px;
  background: rgb(56 189 248);
  box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.12);
  flex-shrink: 0;
}

.convo-actions {
  right: calc(var(--cl-item-px) - 0.02rem);
  top: auto;
  bottom: calc(var(--cl-item-py) - 0.04rem);
  transform: none;
  gap: 0.25rem;
  opacity: 0;
}

.group:hover .convo-actions,
.convo-card-active .convo-actions {
  opacity: 1;
}

.action-btn {
  border-radius: 0.44rem;
}

.action-btn:hover {
  background: rgba(148, 163, 184, 0.1);
}

.delete-mask {
  background: rgba(248, 250, 252, 0.94);
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.empty-state {
  border-radius: 0.85rem;
  border: 1px dashed rgba(148, 163, 184, 0.34);
  background: rgba(255, 255, 255, 0.76);
}

.conversation-list::-webkit-scrollbar {
  width: 6px;
}

.conversation-list::-webkit-scrollbar-track {
  background: transparent;
}

.conversation-list::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.36);
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

:root.light .search-input-wrap,
[data-theme='light'] .search-input-wrap {
  background: rgba(255, 255, 255, 0.88);
  border-color: rgba(148, 163, 184, 0.32);
}

:root.light .convo-main-btn:hover,
[data-theme='light'] .convo-main-btn:hover {
  border-color: rgba(148, 163, 184, 0.35);
}

:root.light .conversation-list,
[data-theme='light'] .conversation-list {
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.95), rgba(244, 247, 251, 0.94));
}

:root.light .list-count,
[data-theme='light'] .list-count {
  color: rgb(71, 85, 105);
  background: rgba(226, 232, 240, 0.9);
}

:root.light .list-title,
[data-theme='light'] .list-title {
  color: rgb(100, 116, 139);
}

:root.dark .conversation-list,
[data-theme='dark'] .conversation-list {
  --cl-border-soft: rgba(71, 85, 105, 0.44);
  --cl-border-strong: rgba(56, 189, 248, 0.5);
  --cl-card-bg: transparent;
  --cl-card-bg-hover: rgba(30, 41, 59, 0.66);
  --cl-card-bg-active: rgba(8, 47, 73, 0.46);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.93), rgba(15, 23, 42, 0.91));
}

:root.dark .list-header,
[data-theme='dark'] .list-header {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.92), rgba(15, 23, 42, 0.84));
}

:root.dark .list-title,
[data-theme='dark'] .list-title {
  color: rgb(148, 163, 184);
}

:root.dark .list-count,
[data-theme='dark'] .list-count {
  color: rgb(203, 213, 225);
  background: rgba(51, 65, 85, 0.9);
}

:root.dark .search-input-wrap,
[data-theme='dark'] .search-input-wrap {
  border-color: rgba(71, 85, 105, 0.68);
  background: rgba(15, 23, 42, 0.58);
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.08);
}

:root.dark .search-input::placeholder,
[data-theme='dark'] .search-input::placeholder {
  color: rgba(148, 163, 184, 0.95);
}

:root.dark .convo-main-btn,
[data-theme='dark'] .convo-main-btn {
  border-color: transparent;
}

:root.dark .convo-main-btn:hover,
[data-theme='dark'] .convo-main-btn:hover {
  border-color: transparent;
}

:root.dark .conversation-item:not(:last-child)::after,
[data-theme='dark'] .conversation-item:not(:last-child)::after {
  background: rgba(51, 65, 85, 0.72);
}

:root.dark .create-btn,
[data-theme='dark'] .create-btn {
  color: rgb(224, 242, 254);
  border-color: rgba(37, 99, 235, 0.4);
  background: linear-gradient(135deg, rgba(29, 78, 216, 0.94), rgba(30, 64, 175, 0.94));
}

:root.dark .create-btn:hover,
[data-theme='dark'] .create-btn:hover {
  border-color: rgba(96, 165, 250, 0.56);
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
  color: rgb(100, 116, 139);
}

:root.dark .delete-mask,
[data-theme='dark'] .delete-mask {
  background: rgba(15, 23, 42, 0.95);
  border-color: rgba(71, 85, 105, 0.45);
}

:root.dark .empty-state,
[data-theme='dark'] .empty-state {
  border-color: rgba(71, 85, 105, 0.64);
  background: rgba(15, 23, 42, 0.66);
}

:global(.chat-desktop-shell) .conversation-list {
  background: rgba(248, 250, 252, 0.72);
}

:global(.chat-desktop-shell) .list-header {
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(10px);
}

:global(.chat-desktop-shell) .empty-state {
  background: rgba(255, 255, 255, 0.58);
}

:global([data-theme='dark'] .chat-desktop-shell) .conversation-list {
  background: rgba(15, 23, 42, 0.44);
}

:global([data-theme='dark'] .chat-desktop-shell) .list-header {
  background: rgba(15, 23, 42, 0.86);
}

:global([data-theme='dark'] .chat-desktop-shell) .empty-state {
  background: rgba(15, 23, 42, 0.54);
}

@media (prefers-reduced-motion: reduce) {
  .conversation-item {
    animation: none !important;
  }

  .convo-main-btn,
  .create-btn,
  .action-btn {
    transition: none !important;
  }
}

@media (max-width: 640px) {
  .list-header {
    padding-top: calc(max(env(safe-area-inset-top), 0px) + 0.85rem);
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
