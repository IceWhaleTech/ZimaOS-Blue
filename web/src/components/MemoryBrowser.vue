<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  memoryServiceApi,
  type MemoryEntry,
  type MemoryNamespace,
  type MemoryEntryStats,
} from '@/api/memory'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

// State
const entries = ref<MemoryEntry[]>([])
const namespaces = ref<MemoryNamespace[]>([])
const stats = ref<MemoryEntryStats | null>(null)
const loading = ref(false)
const selectedNamespace = ref('default')
const nextCursor = ref('')
const statusFilter = ref('')

// Modals
const showAddModal = ref(false)
const showHistoryModal = ref(false)
const showNamespaceModal = ref(false)
const historyEntries = ref<MemoryEntry[]>([])
const historyLoading = ref(false)

// Add form
const form = ref({
  content: '',
  content_type: 'text' as 'text' | 'json' | 'markdown',
  category: '',
  tags: '',
  source: '',
  importance: 0.5,
  ttl: '',
})

// Edit state
const editingId = ref<string | null>(null)
const editContent = ref('')

// Namespace form
const newNamespaceName = ref('')

// Computed
const hasMore = computed(() => nextCursor.value !== '')

// Methods
async function loadNamespaces() {
  try {
    const res = await memoryServiceApi.listNamespaces()
    namespaces.value = res.data.namespaces || []
  } catch (e) {
    console.error('Failed to load namespaces:', e)
  }
}

async function loadEntries(append = false) {
  loading.value = true
  try {
    const res = await memoryServiceApi.listEntries({
      namespace: selectedNamespace.value,
      status: statusFilter.value || undefined,
      cursor: append ? nextCursor.value : undefined,
      limit: 20,
    })
    const data = res.data
    if (append) {
      entries.value = [...entries.value, ...(data.entries || [])]
    } else {
      entries.value = data.entries || []
    }
    nextCursor.value = data.next_cursor || ''
  } catch (e) {
    console.error('Failed to load entries:', e)
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const res = await memoryServiceApi.getStats(selectedNamespace.value)
    stats.value = res.data
  } catch (e) {
    console.error('Failed to load stats:', e)
  }
}

function loadMore() {
  loadEntries(true)
}

async function addEntry() {
  if (!form.value.content.trim()) return
  loading.value = true
  try {
    const tags = form.value.tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean)
    await memoryServiceApi.createEntry(
      {
        content: form.value.content,
        content_type: form.value.content_type,
        category: form.value.category || undefined,
        tags: tags.length > 0 ? tags : undefined,
        source: form.value.source || undefined,
        importance: form.value.importance,
        ttl: form.value.ttl || undefined,
      },
      selectedNamespace.value,
    )
    form.value = { content: '', content_type: 'text', category: '', tags: '', source: '', importance: 0.5, ttl: '' }
    showAddModal.value = false
    await loadEntries()
    await loadStats()
    emit('status-change', t('memoryService.addEntry') + ' OK')
  } catch (e) {
    console.error('Failed to add entry:', e)
  } finally {
    loading.value = false
  }
}

function startEdit(entry: MemoryEntry) {
  editingId.value = entry.id
  editContent.value = entry.content
}

async function saveEdit(id: string) {
  if (!editContent.value.trim()) return
  try {
    await memoryServiceApi.updateEntry(id, editContent.value)
    editingId.value = null
    await loadEntries()
    emit('status-change', t('memoryService.editEntry') + ' OK')
  } catch (e) {
    console.error('Failed to update entry:', e)
  }
}

async function deleteEntry(id: string) {
  if (!confirm(t('memoryService.confirmDelete'))) return
  try {
    await memoryServiceApi.deleteEntry(id)
    entries.value = entries.value.filter((e) => e.id !== id)
    await loadStats()
  } catch (e) {
    console.error('Failed to delete entry:', e)
  }
}

async function viewHistory(id: string) {
  historyLoading.value = true
  showHistoryModal.value = true
  try {
    const res = await memoryServiceApi.getHistory(id)
    historyEntries.value = res.data.versions || []
  } catch (e) {
    console.error('Failed to load history:', e)
  } finally {
    historyLoading.value = false
  }
}

async function purgeExpired() {
  try {
    const res = await memoryServiceApi.purgeExpired()
    const purged = res.data.purged || 0
    emit('status-change', t('memoryService.purgeSuccess', { count: purged }))
    await loadEntries()
    await loadStats()
  } catch (e) {
    console.error('Failed to purge:', e)
  }
}

async function createNamespace() {
  if (!newNamespaceName.value.trim()) return
  try {
    await memoryServiceApi.createNamespace(newNamespaceName.value.trim())
    newNamespaceName.value = ''
    showNamespaceModal.value = false
    await loadNamespaces()
  } catch (e) {
    console.error('Failed to create namespace:', e)
  }
}

async function deleteNamespace(id: string) {
  if (!confirm(t('memoryService.confirmDeleteNamespace', { name: id }))) return
  try {
    await memoryServiceApi.deleteNamespace(id)
    if (selectedNamespace.value === id) {
      selectedNamespace.value = 'default'
    }
    await loadNamespaces()
    await loadEntries()
    await loadStats()
  } catch (e) {
    console.error('Failed to delete namespace:', e)
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

watch(selectedNamespace, () => {
  loadEntries()
  loadStats()
})

onMounted(() => {
  loadNamespaces()
  loadEntries()
  loadStats()
})
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg shadow">
    <!-- Header -->
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('memoryService.title') }}
          </h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
            {{ t('memoryService.description') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white text-sm font-medium rounded-lg transition-colors"
            @click="showAddModal = true"
          >
            {{ t('memoryService.addEntry') }}
          </button>
          <button
            class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            :title="t('memoryService.createNamespace')"
            @click="showNamespaceModal = true"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
            </svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Filters bar -->
    <div class="px-6 py-3 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700 flex flex-wrap items-center gap-3">
      <!-- Namespace selector -->
      <select
        v-model="selectedNamespace"
        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
        @change="loadEntries()"
      >
        <option value="default">{{ t('memoryService.defaultNamespace') }}</option>
        <option v-for="ns in namespaces" :key="ns.id" :value="ns.id">{{ ns.id }}</option>
      </select>

      <!-- Status filter -->
      <select
        v-model="statusFilter"
        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
        @change="loadEntries()"
      >
        <option value="">{{ t('memoryService.status') }}: All</option>
        <option value="active">{{ t('memoryService.active') }}</option>
        <option value="expired">{{ t('memoryService.expired') }}</option>
      </select>

      <!-- Stats -->
      <div v-if="stats" class="ml-auto flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
        <span>{{ t('memoryService.totalEntries') }}: {{ stats.total }}</span>
        <span>{{ t('memoryService.activeEntries') }}: {{ stats.active }}</span>
        <span v-if="stats.expired > 0">{{ t('memoryService.expiredEntries') }}: {{ stats.expired }}</span>
      </div>

      <!-- Purge button -->
      <button
        v-if="stats && stats.expired > 0"
        class="px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 rounded-lg transition-colors"
        @click="purgeExpired"
      >
        {{ t('memoryService.purgeExpired') }}
      </button>
    </div>

    <!-- Entry list -->
    <div v-if="loading && entries.length === 0" class="p-6 text-center">
      <div class="animate-spin h-8 w-8 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"></div>
    </div>

    <div v-else-if="entries.length === 0" class="p-6 text-center">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
      </svg>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('memoryService.noEntries') }}</p>
    </div>

    <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
      <div
        v-for="entry in entries"
        :key="entry.id"
        class="px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-700/50"
      >
        <!-- Inline edit mode -->
        <div v-if="editingId === entry.id" class="space-y-2">
          <textarea
            v-model="editContent"
            rows="3"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white resize-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
          ></textarea>
          <div class="flex gap-2 justify-end">
            <button
              class="px-3 py-1.5 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
              @click="editingId = null"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              class="px-3 py-1.5 text-sm bg-gray-700 dark:bg-gray-500 text-white rounded-lg hover:bg-gray-800 dark:hover:bg-gray-400"
              @click="saveEdit(entry.id)"
            >
              {{ t('common.save') }}
            </button>
          </div>
        </div>

        <!-- Normal display -->
        <div v-else class="flex items-start justify-between gap-4">
          <div class="flex-1 min-w-0">
            <p class="text-gray-900 dark:text-white whitespace-pre-wrap break-words text-sm">
              {{ entry.content }}
            </p>
            <div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
              <!-- Status badge -->
              <span
                :class="[
                  'px-2 py-0.5 rounded-full font-medium',
                  entry.status === 'active' ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' :
                  entry.status === 'expired' ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' :
                  'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'
                ]"
              >
                {{ t(`memoryService.${entry.status}`) }}
              </span>
              <!-- Version -->
              <span class="text-gray-500 dark:text-gray-400">
                {{ t('memoryService.version', { n: entry.version }) }}
              </span>
              <!-- Category -->
              <span v-if="entry.category" class="px-2 py-0.5 bg-gray-100 dark:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-full">
                {{ entry.category }}
              </span>
              <!-- Tags -->
              <span
                v-for="tag in (entry.tags || [])"
                :key="tag"
                class="px-2 py-0.5 bg-gray-700/10 dark:bg-gray-500/20 text-gray-600 dark:text-gray-400 rounded-full"
              >
                {{ tag }}
              </span>
              <!-- TTL / Expiry -->
              <span v-if="entry.expires_at" class="text-gray-400 dark:text-gray-500">
                {{ t('memoryService.expiresAt') }}: {{ formatDate(entry.expires_at) }}
              </span>
              <!-- Created -->
              <span class="text-gray-400 dark:text-gray-500">
                {{ formatDate(entry.created_at) }}
              </span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-1 flex-shrink-0">
            <!-- History -->
            <button
              v-if="entry.version > 1"
              class="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              :title="t('memoryService.viewHistory')"
              @click="viewHistory(entry.id)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <!-- Edit -->
            <button
              class="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              :title="t('memoryService.editEntry')"
              @click="startEdit(entry)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
              </svg>
            </button>
            <!-- Delete -->
            <button
              class="p-1.5 text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
              :title="t('common.delete')"
              @click="deleteEntry(entry.id)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Load more -->
    <div v-if="hasMore" class="px-6 py-3 border-t border-gray-200 dark:border-gray-700 text-center">
      <button
        class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
        :disabled="loading"
        @click="loadMore"
      >
        {{ loading ? t('common.loading') : t('memoryService.loadMore') }}
      </button>
    </div>
  </div>

  <!-- Add Entry Modal -->
  <Teleport to="body">
    <div
      v-if="showAddModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showAddModal = false"
    >
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4 max-h-[80vh] overflow-y-auto">
        <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memoryService.addEntryTitle') }}</h3>
        </div>
        <div class="p-6 space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.content') }} *</label>
            <textarea
              v-model="form.content"
              rows="4"
              :placeholder="t('memoryService.contentPlaceholder')"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 resize-none"
            ></textarea>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.category') }}</label>
              <input
                v-model="form.category"
                type="text"
                :placeholder="t('memoryService.categoryPlaceholder')"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.contentType') }}</label>
              <select
                v-model="form.content_type"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="text">Text</option>
                <option value="json">JSON</option>
                <option value="markdown">Markdown</option>
              </select>
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.tags') }}</label>
            <input
              v-model="form.tags"
              type="text"
              :placeholder="t('memoryService.tagsPlaceholder')"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
            />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.source') }}</label>
              <input
                v-model="form.source"
                type="text"
                :placeholder="t('memoryService.sourcePlaceholder')"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.ttl') }}</label>
              <input
                v-model="form.ttl"
                type="text"
                :placeholder="t('memoryService.ttlPlaceholder')"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {{ t('memoryService.importance') }}: {{ form.importance.toFixed(1) }}
            </label>
            <input
              v-model.number="form.importance"
              type="range"
              min="0"
              max="1"
              step="0.1"
              class="w-full"
            />
          </div>
        </div>
        <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
          <button
            class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            @click="showAddModal = false"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
            :disabled="loading || !form.content.trim()"
            @click="addEntry"
          >
            {{ t('memoryService.addEntry') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>

  <!-- History Modal -->
  <Teleport to="body">
    <div
      v-if="showHistoryModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showHistoryModal = false"
    >
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4 max-h-[80vh] overflow-y-auto">
        <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memoryService.historyTitle') }}</h3>
          <button class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" @click="showHistoryModal = false">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div v-if="historyLoading" class="p-6 text-center">
          <div class="animate-spin h-6 w-6 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"></div>
        </div>
        <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
          <div v-for="ver in historyEntries" :key="ver.id" class="px-6 py-3">
            <div class="flex items-center justify-between mb-1">
              <span class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('memoryService.version', { n: ver.version }) }}
              </span>
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDate(ver.created_at) }}</span>
            </div>
            <p class="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap">{{ ver.content }}</p>
          </div>
        </div>
      </div>
    </div>
  </Teleport>

  <!-- Namespace Modal -->
  <Teleport to="body">
    <div
      v-if="showNamespaceModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-[60]"
      @click.self="showNamespaceModal = false"
    >
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
        <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memoryService.namespace') }}</h3>
        </div>
        <div class="p-6 space-y-4">
          <!-- Create new -->
          <div class="flex gap-2">
            <input
              v-model="newNamespaceName"
              type="text"
              :placeholder="t('memoryService.namespaceName')"
              class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              @keyup.enter="createNamespace"
            />
            <button
              class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
              :disabled="!newNamespaceName.trim()"
              @click="createNamespace"
            >
              {{ t('memoryService.createNamespace') }}
            </button>
          </div>
          <!-- List -->
          <div v-if="namespaces.length > 0" class="space-y-2">
            <div
              v-for="ns in namespaces"
              :key="ns.id"
              class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
            >
              <div>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ ns.id }}</span>
                <span class="text-xs text-gray-500 dark:text-gray-400 ml-2">{{ formatDate(ns.created_at) }}</span>
              </div>
              <button
                class="p-1.5 text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
                @click="deleteNamespace(ns.id)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
          </div>
          <p v-else class="text-sm text-gray-500 dark:text-gray-400 text-center py-2">
            {{ t('memoryService.defaultNamespace') }} only
          </p>
        </div>
        <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end">
          <button
            class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            @click="showNamespaceModal = false"
          >
            {{ t('common.close') || 'Close' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
