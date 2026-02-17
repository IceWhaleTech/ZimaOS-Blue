<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { memoryApi, memoryServiceApi, type MemorySearchResult, type MemoryStats, type MemoryEntry, type MemoryNamespace, type MemoryEntryStats } from '@/api/memory'
import { encryptionApi, type EncryptionStatus } from '@/api/encryption'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

// View mode: 'search' (vector search) or 'browse' (entry list)
type ViewMode = 'search' | 'browse'
const viewMode = ref<ViewMode>('browse')

// --- Search state (from old MemoryManager) ---
const searchResults = ref<MemorySearchResult[]>([])
const searching = ref(false)
const searchQuery = ref('')
const searchType = ref<'hybrid' | 'vector' | 'keyword'>('hybrid')

// --- Browse state (from old MemoryBrowser) ---
const entries = ref<MemoryEntry[]>([])
const namespaces = ref<MemoryNamespace[]>([])
const entryStats = ref<MemoryEntryStats | null>(null)
const browseLoading = ref(false)
const selectedNamespace = ref('default')
const nextCursor = ref('')
const statusFilter = ref('')

// --- Shared state ---
const stats = ref<MemoryStats | null>(null)
const loading = ref(false)
const showAddModal = ref(false)
const showClearConfirm = ref(false)
const showSettingsModal = ref(false)
const showExportImportModal = ref(false)
const showHistoryModal = ref(false)
const showNamespaceModal = ref(false)

// Add form (rich, from MemoryBrowser)
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
const historyEntries = ref<MemoryEntry[]>([])
const historyLoading = ref(false)

// Namespace form
const newNamespaceName = ref('')

// Backend settings
const activeBackend = ref('local')
const availableBackends = ref<string[]>(['local'])

// Export/Import
const memoryExporting = ref(false)
const memoryImporting = ref(false)
const memoryError = ref<string | null>(null)
const memoryImportFile = ref<File | null>(null)
const memoryImportMode = ref<'append' | 'replace'>('append')
const memoryFileInputRef = ref<HTMLInputElement | null>(null)

// Encryption
const encryptionStatus = ref<EncryptionStatus | null>(null)
const encryptionLoading = ref(true)
const encryptionToggling = ref(false)
const showPassphrase = ref(false)
const passphrase = ref('')
const encryptionPollTimer = ref<ReturnType<typeof setInterval> | null>(null)

const migrationPercent = computed(() => {
  if (!encryptionStatus.value || !encryptionStatus.value.migration_total) return 0
  return Math.round((encryptionStatus.value.migration_progress / encryptionStatus.value.migration_total) * 100)
})

const hasMore = computed(() => nextCursor.value !== '')
const hasMemories = computed(() => entries.value.length > 0 || stats.value?.total_chunks)

// --- Methods ---

async function loadStats() {
  try {
    const response = await memoryApi.stats()
    stats.value = response.data
  } catch (error) {
    console.error('Failed to load memory stats:', error)
  }
}

async function loadBackendStatus() {
  try {
    const response = await memoryApi.getBackendStatus()
    activeBackend.value = response.data.active_backend
    if (response.data.available_backends?.length) {
      availableBackends.value = response.data.available_backends
    }
  } catch (error) {
    console.error('Failed to load backend status:', error)
  }
}

async function loadNamespaces() {
  try {
    const res = await memoryServiceApi.listNamespaces()
    namespaces.value = res.data.namespaces || []
  } catch (e) {
    console.error('Failed to load namespaces:', e)
  }
}

async function loadEntries(append = false) {
  browseLoading.value = true
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
    browseLoading.value = false
  }
}

async function loadEntryStats() {
  try {
    const res = await memoryServiceApi.getStats(selectedNamespace.value)
    entryStats.value = res.data
  } catch (e) {
    console.error('Failed to load entry stats:', e)
  }
}

// Search
async function searchMemories() {
  if (!searchQuery.value.trim()) {
    searchResults.value = []
    return
  }
  searching.value = true
  try {
    const response = await memoryApi.search({
      query: searchQuery.value,
      limit: 50,
      search_type: searchType.value,
    })
    searchResults.value = response.data.results
    viewMode.value = 'search'
  } catch (error) {
    console.error('Failed to search memories:', error)
  } finally {
    searching.value = false
  }
}

function clearSearch() {
  searchQuery.value = ''
  searchResults.value = []
  viewMode.value = 'browse'
}

// Add entry (rich form)
async function addEntry() {
  if (!form.value.content.trim()) return
  loading.value = true
  try {
    const tags = form.value.tags.split(',').map((t) => t.trim()).filter(Boolean)
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
    await loadEntryStats()
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
    await loadEntryStats()
  } catch (e) {
    console.error('Failed to delete entry:', e)
  }
}

async function deleteSearchResult(id: string) {
  if (!confirm(t('memory.confirmDelete'))) return
  try {
    await memoryApi.delete(id)
    searchResults.value = searchResults.value.filter((m) => m.id !== id)
    await loadStats()
  } catch (error) {
    console.error('Failed to delete memory:', error)
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
    await loadEntryStats()
  } catch (e) {
    console.error('Failed to purge:', e)
  }
}

async function pruneMemories() {
  loading.value = true
  try {
    const response = await memoryApi.prune()
    await loadStats()
    alert(t('memory.pruneSuccess', { count: response.data.deleted }))
  } catch (error) {
    console.error('Failed to prune memories:', error)
  } finally {
    loading.value = false
  }
}

async function clearAllMemories() {
  loading.value = true
  try {
    await memoryApi.clear()
    searchResults.value = []
    stats.value = null
    showClearConfirm.value = false
    await loadStats()
    await loadEntries()
    await loadEntryStats()
  } catch (error) {
    console.error('Failed to clear memories:', error)
  } finally {
    loading.value = false
  }
}

// Namespace
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
    if (selectedNamespace.value === id) selectedNamespace.value = 'default'
    await loadNamespaces()
    await loadEntries()
    await loadEntryStats()
  } catch (e) {
    console.error('Failed to delete namespace:', e)
  }
}

// Backend
async function switchBackend(backend: string) {
  try {
    await memoryApi.setBackend(backend)
    activeBackend.value = backend
    await loadStats()
  } catch (error) {
    console.error('Failed to switch backend:', error)
  }
}

// Export/Import
function readFileAsText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(new Error('Failed to read file'))
    reader.readAsText(file)
  })
}

async function handleMemoryExport() {
  memoryExporting.value = true
  memoryError.value = null
  try {
    const response = await memoryApi.exportMarkdown()
    const content = typeof response.data === 'string' ? response.data : JSON.stringify(response.data)
    const blob = new Blob([content], { type: 'text/markdown; charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `memory-export-${new Date().toISOString().split('T')[0]}.md`
    a.click()
    URL.revokeObjectURL(url)
    emit('status-change', t('userdata.memory.exportSuccess'))
  } catch (e) {
    memoryError.value = e instanceof Error ? e.message : t('userdata.memory.exportFailed')
  } finally {
    memoryExporting.value = false
  }
}

function handleMemoryFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    const file = input.files[0]
    if (file) memoryImportFile.value = file
    memoryError.value = null
  }
}

async function handleMemoryImport() {
  if (!memoryImportFile.value) return
  memoryImporting.value = true
  memoryError.value = null
  try {
    const content = await readFileAsText(memoryImportFile.value)
    const response = await memoryApi.importMarkdown(content, memoryImportMode.value)
    if (response.data.errors && response.data.errors.length > 0) {
      memoryError.value = response.data.errors.join(', ')
    }
    emit('status-change', t('userdata.memory.importSuccess', { count: response.data.imported }))
    memoryImportFile.value = null
    if (memoryFileInputRef.value) memoryFileInputRef.value.value = ''
    await loadStats()
  } catch (e) {
    memoryError.value = e instanceof Error ? e.message : t('userdata.memory.importFailed')
  } finally {
    memoryImporting.value = false
  }
}

function closeExportImportModal() {
  showExportImportModal.value = false
  memoryError.value = null
  memoryImportFile.value = null
  if (memoryFileInputRef.value) memoryFileInputRef.value.value = ''
}

// Encryption
async function fetchEncryptionStatus() {
  try {
    const res = await encryptionApi.getStatus()
    encryptionStatus.value = res.data
    if (res.data.migrating && !encryptionPollTimer.value) {
      encryptionPollTimer.value = setInterval(fetchEncryptionStatus, 2000)
    } else if (!res.data.migrating && encryptionPollTimer.value) {
      clearInterval(encryptionPollTimer.value)
      encryptionPollTimer.value = null
    }
  } catch {
    encryptionStatus.value = null
  }
}

async function toggleEncryption() {
  if (encryptionToggling.value || !encryptionStatus.value) return
  if (encryptionStatus.value.enabled) {
    encryptionToggling.value = true
    try {
      await encryptionApi.disable()
      emit('status-change', t('encryption.disabled'))
      await fetchEncryptionStatus()
    } finally {
      encryptionToggling.value = false
    }
  } else {
    showPassphrase.value = true
  }
}

async function enableWithPassphrase() {
  if (!passphrase.value.trim()) return
  encryptionToggling.value = true
  try {
    await encryptionApi.enable(passphrase.value)
    passphrase.value = ''
    showPassphrase.value = false
    emit('status-change', t('encryption.enabled'))
    await fetchEncryptionStatus()
  } finally {
    encryptionToggling.value = false
  }
}

function cancelPassphrase() {
  showPassphrase.value = false
  passphrase.value = ''
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function getScoreColor(score: number): string {
  if (score >= 0.8) return 'text-green-600 dark:text-green-400'
  if (score >= 0.5) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-gray-500 dark:text-gray-400'
}

watch(selectedNamespace, () => {
  loadEntries()
  loadEntryStats()
})

onMounted(async () => {
  loadStats()
  loadBackendStatus()
  loadNamespaces()
  loadEntries()
  loadEntryStats()
  await fetchEncryptionStatus()
  encryptionLoading.value = false
})
</script>

<template>
  <div class="bg-white dark:bg-gray-700/30 rounded-lg shadow">
    <!-- Header -->
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('memory.title') }}</h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">{{ t('memory.description') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <button class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white text-sm font-medium rounded-lg transition-colors" @click="showAddModal = true">
            {{ t('memoryService.addEntry') }}
          </button>
          <button class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors" :title="t('userdata.memory.title')" @click="showExportImportModal = true">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" /></svg>
          </button>
          <button class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors" :title="t('common.settings')" @click="showSettingsModal = true">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Search bar -->
    <div class="px-6 py-3 border-b border-gray-200 dark:border-gray-700">
      <div class="flex gap-2">
        <input v-model="searchQuery" type="text" :placeholder="t('memory.searchPlaceholder')" class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400" @keyup.enter="searchMemories" />
        <select v-model="searchType" class="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm">
          <option value="hybrid">{{ t('memory.searchTypes.hybrid') }}</option>
          <option value="vector">{{ t('memory.searchTypes.vector') }}</option>
          <option value="keyword">{{ t('memory.searchTypes.keyword') }}</option>
        </select>
        <button class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white text-sm rounded-lg transition-colors disabled:opacity-50" :disabled="searching || !searchQuery.trim()" @click="searchMemories">
          {{ searching ? t('common.searching') : t('common.search') }}
        </button>
        <button v-if="viewMode === 'search'" class="px-3 py-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 text-sm" @click="clearSearch">
          {{ t('common.cancel') }}
        </button>
      </div>
    </div>

    <!-- Filter bar (browse mode) -->
    <div v-if="viewMode === 'browse'" class="px-6 py-2 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700 flex flex-wrap items-center gap-3">
      <select v-model="statusFilter" class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" @change="loadEntries()">
        <option value="">{{ t('memoryService.status') }}: {{ t('memoryService.statusAll') }}</option>
        <option value="active">{{ t('memoryService.active') }}</option>
        <option value="expired">{{ t('memoryService.expired') }}</option>
      </select>
      <div v-if="entryStats" class="ml-auto flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ entryStats.total }} {{ t('memoryService.totalEntries') }}</span>
        <span>{{ entryStats.active }} {{ t('memoryService.activeEntries') }}</span>
      </div>
      <button v-if="entryStats && entryStats.expired > 0" class="px-3 py-1 text-xs text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 rounded-lg" @click="purgeExpired">
        {{ t('memoryService.purgeExpired') }}
      </button>
    </div>

    <!-- Search Results -->
    <div v-if="viewMode === 'search'">
      <div v-if="searching" class="p-6 text-center">
        <div class="animate-spin h-8 w-8 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"></div>
      </div>
      <div v-else-if="searchResults.length > 0" class="divide-y divide-gray-200 dark:divide-gray-700">
        <div v-for="memory in searchResults" :key="memory.id" class="px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-700/50">
          <div class="flex items-start justify-between gap-4">
            <div class="flex-1 min-w-0">
              <p class="text-gray-900 dark:text-white whitespace-pre-wrap break-words text-sm">{{ memory.content }}</p>
              <div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
                <span class="text-gray-500 dark:text-gray-400">{{ formatDate(memory.created_at) }}</span>
                <span :class="getScoreColor(memory.score)">{{ (memory.score * 100).toFixed(1) }}%</span>
                <span v-for="mt in memory.match_types" :key="mt" class="px-2 py-0.5 font-medium rounded-full bg-gray-700/10 dark:bg-gray-500/30 text-gray-600 dark:text-gray-300">{{ mt }}</span>
              </div>
            </div>
            <button class="p-1.5 text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg" @click="deleteSearchResult(memory.id)">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
            </button>
          </div>
        </div>
      </div>
      <div v-else class="p-6 text-center text-gray-500 dark:text-gray-400">{{ t('memory.noResults') }}</div>
    </div>

    <!-- Browse Entry List -->
    <div v-if="viewMode === 'browse'">
      <div v-if="browseLoading && entries.length === 0" class="p-6 text-center">
        <div class="animate-spin h-8 w-8 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"></div>
      </div>
      <div v-else-if="entries.length === 0" class="p-6 text-center text-gray-500 dark:text-gray-400">
        <svg class="w-10 h-10 mx-auto mb-2 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
        </svg>
        <p>{{ t('memoryService.noEntries') }}</p>
      </div>
      <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
        <div v-for="entry in entries" :key="entry.id" class="px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-700/50">
          <!-- Inline edit -->
          <div v-if="editingId === entry.id" class="space-y-2">
            <textarea v-model="editContent" rows="3" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white resize-none text-sm"></textarea>
            <div class="flex gap-2 justify-end">
              <button class="px-3 py-1.5 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" @click="editingId = null">{{ t('common.cancel') }}</button>
              <button class="px-3 py-1.5 text-sm bg-gray-700 dark:bg-gray-500 text-white rounded-lg hover:bg-gray-800 dark:hover:bg-gray-400" @click="saveEdit(entry.id)">{{ t('common.save') }}</button>
            </div>
          </div>
          <!-- Normal display -->
          <div v-else class="flex items-start justify-between gap-4">
            <div class="flex-1 min-w-0">
              <p class="text-gray-900 dark:text-white whitespace-pre-wrap break-words text-sm">{{ entry.content }}</p>
              <div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
                <span :class="['px-2 py-0.5 rounded-full font-medium', entry.status === 'active' ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' : entry.status === 'expired' ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400']">{{ t(`memoryService.${entry.status}`) }}</span>
                <span class="text-gray-500 dark:text-gray-400">v{{ entry.version }}</span>
                <span v-if="entry.category" class="px-2 py-0.5 bg-gray-100 dark:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-full">{{ entry.category }}</span>
                <span v-for="tag in (entry.tags || [])" :key="tag" class="px-2 py-0.5 bg-gray-700/10 dark:bg-gray-500/20 text-gray-600 dark:text-gray-400 rounded-full">{{ tag }}</span>
                <span v-if="entry.expires_at" class="text-gray-400">{{ formatDate(entry.expires_at) }}</span>
                <span class="text-gray-400">{{ formatDate(entry.created_at) }}</span>
              </div>
            </div>
            <div class="flex items-center gap-1 flex-shrink-0">
              <button v-if="entry.version > 1" class="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" @click="viewHistory(entry.id)">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </button>
              <button class="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" @click="startEdit(entry)">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
              </button>
              <button class="p-1.5 text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg" @click="deleteEntry(entry.id)">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
              </button>
            </div>
          </div>
        </div>
      </div>
      <!-- Load more -->
      <div v-if="hasMore" class="px-6 py-3 border-t border-gray-200 dark:border-gray-700 text-center">
        <button class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" :disabled="browseLoading" @click="loadEntries(true)">
          {{ browseLoading ? t('common.loading') : t('memoryService.loadMore') }}
        </button>
      </div>
    </div>

    <!-- Bottom actions -->
    <div v-if="hasMemories" class="px-6 py-3 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
      <button class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" :disabled="loading" @click="pruneMemories">{{ t('memory.prune') }}</button>
      <button class="px-4 py-2 text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 rounded-lg" :disabled="loading" @click="showClearConfirm = true">{{ t('memory.clearAll') }}</button>
    </div>

    <!-- Add Entry Modal -->
    <Teleport to="body">
      <div v-if="showAddModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showAddModal = false">
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4 max-h-[80vh] overflow-y-auto">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memoryService.addEntryTitle') }}</h3>
          </div>
          <div class="p-6 space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.content') }} *</label>
              <textarea v-model="form.content" rows="4" :placeholder="t('memoryService.contentPlaceholder')" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white resize-none"></textarea>
            </div>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.category') }}</label>
                <input v-model="form.category" type="text" :placeholder="t('memoryService.categoryPlaceholder')" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.contentType') }}</label>
                <select v-model="form.content_type" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white">
                  <option value="text">Text</option>
                  <option value="json">JSON</option>
                  <option value="markdown">Markdown</option>
                </select>
              </div>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.tags') }}</label>
              <input v-model="form.tags" type="text" :placeholder="t('memoryService.tagsPlaceholder')" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" />
            </div>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.source') }}</label>
                <input v-model="form.source" type="text" :placeholder="t('memoryService.sourcePlaceholder')" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.ttl') }}</label>
                <input v-model="form.ttl" type="text" :placeholder="t('memoryService.ttlPlaceholder')" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" />
              </div>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memoryService.importance') }}: {{ form.importance.toFixed(1) }}</label>
              <input v-model.number="form.importance" type="range" min="0" max="1" step="0.1" class="w-full" />
            </div>
          </div>
          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" @click="showAddModal = false">{{ t('common.cancel') }}</button>
            <button class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg disabled:opacity-50" :disabled="loading || !form.content.trim()" @click="addEntry">{{ t('memoryService.addEntry') }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Clear Confirm Modal -->
    <Teleport to="body">
      <div v-if="showClearConfirm" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showClearConfirm = false">
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-red-600 dark:text-red-400">{{ t('memory.clearAllTitle') }}</h3>
          </div>
          <div class="p-6"><p class="text-gray-700 dark:text-gray-300">{{ t('memory.clearAllWarning') }}</p></div>
          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" @click="showClearConfirm = false">{{ t('common.cancel') }}</button>
            <button class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg disabled:opacity-50" :disabled="loading" @click="clearAllMemories">{{ t('memory.clearAll') }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- History Modal -->
    <Teleport to="body">
      <div v-if="showHistoryModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showHistoryModal = false">
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4 max-h-[80vh] overflow-y-auto">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memoryService.historyTitle') }}</h3>
            <button class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" @click="showHistoryModal = false">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
          <div v-if="historyLoading" class="p-6 text-center"><div class="animate-spin h-6 w-6 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"></div></div>
          <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
            <div v-for="ver in historyEntries" :key="ver.id" class="px-6 py-3">
              <div class="flex items-center justify-between mb-1">
                <span class="text-sm font-medium text-gray-900 dark:text-white">v{{ ver.version }}</span>
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
      <div v-if="showNamespaceModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-[60]" @click.self="showNamespaceModal = false">
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memoryService.namespace') }}</h3>
          </div>
          <div class="p-6 space-y-4">
            <div class="flex gap-2">
              <input v-model="newNamespaceName" type="text" :placeholder="t('memoryService.namespaceName')" class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" @keyup.enter="createNamespace" />
              <button class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg disabled:opacity-50" :disabled="!newNamespaceName.trim()" @click="createNamespace">{{ t('memoryService.createNamespace') }}</button>
            </div>
            <div v-if="namespaces.length > 0" class="space-y-2">
              <div v-for="ns in namespaces" :key="ns.id" class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                <div>
                  <span class="text-sm font-medium text-gray-900 dark:text-white">{{ ns.id }}</span>
                  <span class="text-xs text-gray-500 dark:text-gray-400 ml-2">{{ formatDate(ns.created_at) }}</span>
                </div>
                <button class="p-1.5 text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg" @click="deleteNamespace(ns.id)">
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                </button>
              </div>
            </div>
            <p v-else class="text-sm text-gray-500 dark:text-gray-400 text-center py-2">{{ t('memoryService.defaultNamespaceOnly') }}</p>
          </div>
          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end">
            <button class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" @click="showNamespaceModal = false">{{ t('common.close') || 'Close' }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Settings Modal -->
    <Teleport to="body">
      <div v-if="showSettingsModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showSettingsModal = false">
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memory.settingsTitle') }}</h3>
          </div>
          <div class="p-6 space-y-6">
            <!-- Namespace -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('memoryService.namespace') }}</label>
              <div class="flex gap-2 mb-2">
                <select v-model="selectedNamespace" class="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white">
                  <option value="default">{{ t('memoryService.defaultNamespace') }}</option>
                  <option v-for="ns in namespaces" :key="ns.id" :value="ns.id">{{ ns.id }}</option>
                </select>
                <button class="px-3 py-2 text-sm bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-lg" @click="showNamespaceModal = true">{{ t('memoryService.createNamespace') }}</button>
              </div>
              <div v-if="namespaces.length > 0" class="space-y-1">
                <div v-for="ns in namespaces" :key="ns.id" class="flex items-center justify-between px-3 py-1.5 bg-gray-50 dark:bg-gray-700/50 rounded-lg text-sm">
                  <span class="text-gray-900 dark:text-white">{{ ns.id }}</span>
                  <button class="p-1 text-red-400 hover:text-red-600 dark:hover:text-red-300 rounded" @click="deleteNamespace(ns.id)">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                  </button>
                </div>
              </div>
              <p v-else class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ t('memoryService.defaultNamespaceOnly') }}</p>
            </div>
            <!-- Backend Selection -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('memory.backend') }}</label>
              <div class="flex gap-2">
                <button v-for="backend in availableBackends" :key="backend" class="flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors" :class="activeBackend === backend ? 'bg-gray-700 dark:bg-gray-500 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'" @click="switchBackend(backend)">{{ t(`memory.backendLabel.${backend}`) }}</button>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">{{ t(`memory.backendDesc.${activeBackend}`) }}</p>
            </div>
            <!-- Encryption -->
            <div>
              <div class="flex items-center justify-between mb-2">
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('encryption.title') }}</label>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('encryption.desc') }}</p>
                </div>
                <button v-if="encryptionStatus && !encryptionStatus.migrating" class="relative w-10 h-5 rounded-full transition-colors" :class="encryptionStatus.enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'" :disabled="encryptionToggling" @click="toggleEncryption">
                  <span class="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform" :class="encryptionStatus.enabled ? 'translate-x-5' : ''" />
                </button>
              </div>
              <div v-if="showPassphrase" class="flex gap-2 mb-2">
                <input v-model="passphrase" type="password" :placeholder="t('encryption.passphrasePlaceholder')" class="flex-1 px-3 py-1.5 text-sm bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded-lg text-gray-700 dark:text-gray-200" @keyup.enter="enableWithPassphrase" />
                <button class="px-3 py-1.5 text-xs bg-green-600 hover:bg-green-700 text-white rounded-lg" :disabled="encryptionToggling || !passphrase.trim()" @click="enableWithPassphrase">{{ t('encryption.confirm') }}</button>
                <button class="px-3 py-1.5 text-xs bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-200 rounded-lg" @click="cancelPassphrase">{{ t('common.cancel') }}</button>
              </div>
              <div v-if="encryptionStatus?.migrating" class="mb-2">
                <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400 mb-1"><span>{{ t('encryption.migrating') }}</span><span>{{ migrationPercent }}%</span></div>
                <div class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden"><div class="h-full bg-blue-500 rounded-full transition-all" :style="{ width: migrationPercent + '%' }" /></div>
              </div>
              <div v-if="encryptionStatus && !encryptionLoading" class="grid grid-cols-3 gap-3 text-center">
                <div class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-2"><div class="text-lg font-bold text-gray-700 dark:text-white">{{ encryptionStatus.encrypted_count }}</div><div class="text-xs text-gray-500 dark:text-gray-400">{{ t('encryption.encrypted') }}</div></div>
                <div class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-2"><div class="text-lg font-bold text-gray-700 dark:text-white">{{ encryptionStatus.plaintext_count }}</div><div class="text-xs text-gray-500 dark:text-gray-400">{{ t('encryption.plaintext') }}</div></div>
                <div class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-2"><div class="text-lg font-bold text-gray-700 dark:text-white">{{ encryptionStatus.enabled ? 'AES-256' : 'OFF' }}</div><div class="text-xs text-gray-500 dark:text-gray-400">{{ t('encryption.algorithm') }}</div></div>
              </div>
              <div v-if="encryptionLoading" class="text-center text-sm text-gray-400 py-2">{{ t('common.loading') }}</div>
            </div>
          </div>
          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end">
            <button class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" @click="showSettingsModal = false">{{ t('common.close') || 'Close' }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Export/Import Modal -->
    <Teleport to="body">
      <div v-if="showExportImportModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="closeExportImportModal">
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 max-h-[80vh] overflow-y-auto">
          <div class="sticky top-0 bg-white dark:bg-gray-800 px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('userdata.memory.title') }}</h3>
            <button class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" @click="closeExportImportModal">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
          <div class="p-6 space-y-6">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('userdata.memory.description') }}</p>
            <!-- Export -->
            <div class="space-y-2">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.memory.exportSection') }}</h4>
              <button :disabled="memoryExporting" class="w-full px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm font-medium disabled:opacity-50 flex items-center justify-center gap-2" @click="handleMemoryExport">
                <svg v-if="memoryExporting" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                {{ memoryExporting ? t('userdata.exporting') : t('userdata.memory.exportButton') }}
              </button>
            </div>
            <hr class="border-gray-200 dark:border-gray-700" />
            <!-- Import -->
            <div class="space-y-3">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.memory.importSection') }}</h4>
              <div class="flex items-center gap-2">
                <input ref="memoryFileInputRef" type="file" accept=".md,.markdown,.txt" class="hidden" @change="handleMemoryFileSelect" />
                <button class="px-4 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg text-sm" @click="memoryFileInputRef?.click()">{{ t('userdata.chooseFile') }}</button>
                <span v-if="memoryImportFile" class="text-sm text-gray-600 dark:text-gray-300 truncate flex-1">{{ memoryImportFile.name }}</span>
              </div>
              <div>
                <label class="block text-sm text-gray-500 dark:text-gray-400 mb-2">{{ t('userdata.memory.importMode') }}</label>
                <div class="flex gap-2">
                  <button :class="['flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors', memoryImportMode === 'append' ? 'bg-gray-700 dark:bg-gray-500 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300']" @click="memoryImportMode = 'append'">{{ t('userdata.memory.modeAppend') }}</button>
                  <button :class="['flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors', memoryImportMode === 'replace' ? 'bg-red-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300']" @click="memoryImportMode = 'replace'">{{ t('userdata.memory.modeReplace') }}</button>
                </div>
              </div>
              <button :disabled="!memoryImportFile || memoryImporting" class="w-full px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm font-medium disabled:opacity-50 flex items-center justify-center gap-2" @click="handleMemoryImport">
                <svg v-if="memoryImporting" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                {{ memoryImporting ? t('userdata.importing') : t('userdata.memory.importButton') }}
              </button>
            </div>
            <div v-if="memoryError" class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-300 text-sm">{{ memoryError }}</div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
