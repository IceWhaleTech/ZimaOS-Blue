<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Conversation } from '@/api/chat'

const { t } = useI18n()

defineProps<{
  conversations: Conversation[]
  currentId: string | null
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
const contextMenuPos = ref({ x: 0, y: 0 })

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
function startLongPress(id: string, event: MouseEvent | TouchEvent) {
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
  <div class="conversation-list h-full flex flex-col bg-white dark:bg-gray-700/30">
    <!-- Header: new chat + search merged into one row -->
    <div class="flex items-center gap-2 p-3 border-b border-gray-200 dark:border-gray-700">
      <!-- Default: New Chat button / Search active: Search input -->
      <div class="flex-1 min-w-0">
        <button
          v-if="!searchActive"
          class="w-full py-2 px-4 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg flex items-center justify-center gap-2 transition-colors"
          @click="handleCreate"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          {{ t('chat.newChat') }}
        </button>
        <div v-else class="relative">
          <input
            ref="searchInputRef"
            v-model="searchQuery"
            type="text"
            :placeholder="t('chat.searchConversations')"
            class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 pl-10 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
            @keydown.escape="toggleSearch"
          />
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
      </div>
      <!-- Search / Close toggle button -->
      <button
        class="shrink-0 p-2 rounded-lg transition-colors"
        :class="searchActive
          ? 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700'
          : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700'"
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
    <div class="flex-1 overflow-y-auto">
      <!-- Loading state -->
      <div v-if="loading || searching" class="p-4 text-center text-gray-500 dark:text-gray-400">
        <div class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto" />
        <p class="mt-2">{{ searching ? t('chat.searching') : t('common.loading') }}</p>
      </div>

      <!-- Empty state -->
      <div
        v-else-if="conversations.length === 0"
        class="p-4 text-center text-gray-500 dark:text-gray-400"
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
      <div v-else class="divide-y divide-gray-100 dark:divide-gray-800">
        <div
          v-for="conversation in conversations"
          :key="conversation.id"
          class="conversation-item relative group"
          :class="{ 'bg-gray-100 dark:bg-gray-700': conversation.id === currentId }"
        >
          <button
            class="w-full p-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
            @click="handleSelect(conversation.id)"
            @mousedown="startLongPress(conversation.id, $event)"
            @mouseup="cancelLongPress"
            @mouseleave="cancelLongPress"
            @touchstart="startLongPress(conversation.id, $event)"
            @touchend="cancelLongPress"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <!-- Pin icon -->
                  <svg
                    v-if="conversation.pinned"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-4 w-4 text-yellow-500 flex-shrink-0"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                  >
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm3.5-9c.83 0 1.5-.67 1.5-1.5S16.33 8 15.5 8 14 8.67 14 9.5s.67 1.5 1.5 1.5zm-7 0c.83 0 1.5-.67 1.5-1.5S9.33 8 8.5 8 7 8.67 7 9.5 7.67 11 8.5 11zm3.5 6.5c2.33 0 4.31-1.46 5.11-3.5H6.89c.8 2.04 2.78 3.5 5.11 3.5z" />
                  </svg>
                </div>
                <h3 class="text-gray-900 dark:text-white font-medium truncate">
                  {{ conversation.title }}
                </h3>
                <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  {{ formatDate(conversation.updated_at) }}
                </p>
              </div>
            </div>
          </button>

          <!-- Pin/Unpin and Delete buttons -->
          <div class="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <!-- Pin button -->
            <button
              class="p-2 text-gray-400 hover:text-yellow-500 transition-colors"
              :title="conversation.pinned ? t('chat.unpinConversation') : t('chat.pinConversation')"
              @click.stop="handlePin(conversation.id, conversation.pinned || false)"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                :fill="conversation.pinned ? 'currentColor' : 'none'"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M5 5a2 2 0 012-2h6a2 2 0 012 2v16l-7-3.5L5 21V5z"
                />
              </svg>
            </button>
            <!-- Delete button -->
            <button
              class="p-2 text-gray-400 hover:text-red-500 transition-colors"
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
            class="absolute inset-0 bg-white/95 dark:bg-gray-700/30/95 flex items-center justify-center gap-2 p-2"
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
              <button
                v-for="conv in conversations.filter(c => c.id === showContextMenu)"
                :key="conv.id"
                class="w-full px-4 py-3 bg-yellow-500 hover:bg-yellow-600 text-white text-base font-medium rounded-xl transition-colors flex items-center justify-center gap-2"
                @click="handlePin(conv.id, conv.pinned || false); showContextMenu = null"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5"
                  :fill="conv.pinned ? 'currentColor' : 'none'"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 5a2 2 0 012-2h6a2 2 0 012 2v16l-7-3.5L5 21V5z"
                  />
                </svg>
                {{ conv.pinned ? t('chat.unpinConversation') : t('chat.pinConversation') }}
              </button>
              <button
                class="w-full px-4 py-3 bg-red-600 hover:bg-red-700 text-white text-base font-medium rounded-xl transition-colors flex items-center justify-center gap-2"
                @click="handleDelete(showContextMenu); showContextMenu = null"
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
                {{ t('chat.deleteConversation') }}
              </button>
              <button
                class="w-full px-4 py-3 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-900 dark:text-white text-base font-medium rounded-xl transition-colors"
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
.conversation-list::-webkit-scrollbar {
  width: 6px;
}

.conversation-list::-webkit-scrollbar-track {
  background: transparent;
}

.conversation-list::-webkit-scrollbar-thumb {
  background: #4b5563;
  border-radius: 3px;
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
