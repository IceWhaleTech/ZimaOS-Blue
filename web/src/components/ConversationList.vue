<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Conversation } from '@/api/chat'

const { t } = useI18n()

const props = defineProps<{
  conversations: Conversation[]
  currentId: string | null
  loading?: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  create: []
  delete: [id: string]
  search: [query: string]
}>()

const searchQuery = ref('')
const showDeleteConfirm = ref<string | null>(null)

const filteredConversations = computed(() => {
  if (!searchQuery.value.trim()) {
    return props.conversations
  }
  const query = searchQuery.value.toLowerCase()
  return props.conversations.filter((c) => c.title.toLowerCase().includes(query))
})

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
  <div class="conversation-list h-full flex flex-col bg-white dark:bg-gray-900">
    <!-- Header -->
    <div class="p-4 border-b border-gray-200 dark:border-gray-700">
      <button
        class="w-full py-2 px-4 bg-blue-600 hover:bg-blue-700 text-white rounded-lg flex items-center justify-center gap-2 transition-colors"
        @click="handleCreate"
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
            d="M12 4v16m8-8H4"
          />
        </svg>
        {{ t('chat.newChat') }}
      </button>
    </div>

    <!-- Search -->
    <div class="p-4 border-b border-gray-200 dark:border-gray-700">
      <div class="relative">
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="t('chat.searchConversations')"
          class="w-full bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-white rounded-lg px-4 py-2 pl-10 focus:outline-none focus:ring-2 focus:ring-blue-500"
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
      </div>
    </div>

    <!-- Conversation list -->
    <div class="flex-1 overflow-y-auto">
      <!-- Loading state -->
      <div v-if="loading" class="p-4 text-center text-gray-500 dark:text-gray-400">
        <div class="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full mx-auto" />
        <p class="mt-2">{{ t('common.loading') }}</p>
      </div>

      <!-- Empty state -->
      <div
        v-else-if="filteredConversations.length === 0"
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
          v-for="conversation in filteredConversations"
          :key="conversation.id"
          class="conversation-item relative group"
          :class="{ 'bg-blue-50 dark:bg-gray-800': conversation.id === currentId }"
        >
          <button
            class="w-full p-4 text-left hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
            @click="handleSelect(conversation.id)"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="flex-1 min-w-0">
                <h3 class="text-gray-900 dark:text-white font-medium truncate">
                  {{ conversation.title }}
                </h3>
                <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  {{ formatDate(conversation.updated_at) }}
                </p>
              </div>
            </div>
          </button>

          <!-- Delete button -->
          <button
            class="absolute right-2 top-1/2 -translate-y-1/2 p-2 text-gray-400 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity"
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

          <!-- Delete confirmation -->
          <div
            v-if="showDeleteConfirm === conversation.id"
            class="absolute inset-0 bg-white/95 dark:bg-gray-900/95 flex items-center justify-center gap-2 p-2"
          >
            <span class="text-sm text-gray-600 dark:text-gray-300">{{ t('chat.confirmDelete') }}</span>
            <button
              class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-sm rounded transition-colors"
              @click.stop="confirmDelete(conversation.id)"
            >
              {{ t('common.yes') }}
            </button>
            <button
              class="px-3 py-1 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white text-sm rounded transition-colors"
              @click.stop="cancelDelete"
            >
              {{ t('common.no') }}
            </button>
          </div>
        </div>
      </div>
    </div>
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
</style>
