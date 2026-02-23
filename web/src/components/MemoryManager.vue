<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { memoryApi, type MemorySearchResult, type MemoryStats } from '@/api/memory'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

// State
const searchResults = ref<MemorySearchResult[]>([])
const searching = ref(false)
const searchQuery = ref('')
const stats = ref<MemoryStats | null>(null)
const loading = ref(false)
const showAddModal = ref(false)
const showClearConfirm = ref(false)
const showExportImportModal = ref(false)

// Add form
const addContent = ref('')
const addTags = ref('')

// Export/Import
const memoryExporting = ref(false)
const memoryImporting = ref(false)
const memoryError = ref<string | null>(null)
const memoryImportFile = ref<File | null>(null)
const memoryImportMode = ref<'append' | 'replace'>('append')
const memoryFileInputRef = ref<HTMLInputElement | null>(null)

const hasMemories = computed(() => (stats.value?.total_chunks ?? 0) > 0)

// --- Methods ---

async function loadStats() {
  try {
    const response = await memoryApi.stats()
    stats.value = response.data
  } catch (error) {
    console.error('Failed to load memory stats:', error)
  }
}

async function searchMemories() {
  if (!searchQuery.value.trim()) {
    searchResults.value = []
    return
  }
  searching.value = true
  try {
    const response = await memoryApi.search({ query: searchQuery.value, limit: 50 })
    searchResults.value = response.data.results
  } catch (error) {
    console.error('Failed to search memories:', error)
  } finally {
    searching.value = false
  }
}

function clearSearch() {
  searchQuery.value = ''
  searchResults.value = []
}

async function addMemory() {
  if (!addContent.value.trim()) return
  loading.value = true
  try {
    const tags = addTags.value.split(',').map((t) => t.trim()).filter(Boolean)
    await memoryApi.store({ content: addContent.value, tags: tags.length > 0 ? tags : undefined })
    addContent.value = ''
    addTags.value = ''
    showAddModal.value = false
    await loadStats()
    emit('status-change', t('memory.add') + ' OK')
  } catch (e) {
    console.error('Failed to add memory:', e)
  } finally {
    loading.value = false
  }
}

async function deleteMemory(id: string) {
  if (!confirm(t('memory.confirmDelete'))) return
  try {
    await memoryApi.delete(id)
    searchResults.value = searchResults.value.filter((m) => m.id !== id)
    await loadStats()
  } catch (error) {
    console.error('Failed to delete memory:', error)
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
  } catch (error) {
    console.error('Failed to clear memories:', error)
  } finally {
    loading.value = false
  }
}

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

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function getScoreColor(score: number): string {
  if (score >= 0.8) return 'text-green-600 dark:text-green-400'
  if (score >= 0.5) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-gray-500 dark:text-gray-400'
}

onMounted(() => {
  loadStats()
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
            {{ t('memory.add') }}
          </button>
          <button class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors" :title="t('userdata.memory.title')" @click="showExportImportModal = true">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" /></svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Search bar -->
    <div class="px-6 py-3 border-b border-gray-200 dark:border-gray-700">
      <div class="flex gap-2">
        <input v-model="searchQuery" type="text" :placeholder="t('memory.searchPlaceholder')" class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400" @keyup.enter="searchMemories" />
        <button class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white text-sm rounded-lg transition-colors disabled:opacity-50" :disabled="searching || !searchQuery.trim()" @click="searchMemories">
          {{ searching ? t('memory.searching') : t('common.search') }}
        </button>
        <button v-if="searchResults.length > 0" class="px-3 py-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 text-sm" @click="clearSearch">
          {{ t('common.cancel') }}
        </button>
      </div>
    </div>

    <!-- Stats bar -->
    <div v-if="stats" class="px-6 py-2 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700 flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
      <span>{{ stats.total_chunks }} {{ t('memory.totalMemories') }}</span>
      <span v-if="stats.total_size_bytes">{{ (stats.total_size_bytes / 1024).toFixed(1) }} KB</span>
    </div>

    <!-- Search Results -->
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
          <button class="p-1.5 text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg" @click="deleteMemory(memory.id)">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
          </button>
        </div>
      </div>
    </div>
    <div v-else-if="searchQuery.trim()" class="p-6 text-center text-gray-500 dark:text-gray-400">{{ t('memory.noResults') }}</div>
    <div v-else class="p-6 text-center text-gray-500 dark:text-gray-400">{{ t('memory.searchHint') }}</div>

    <!-- Bottom actions -->
    <div v-if="hasMemories" class="px-6 py-3 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
      <button class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" :disabled="loading" @click="pruneMemories">{{ t('memory.prune') }}</button>
      <button class="px-4 py-2 text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 rounded-lg" :disabled="loading" @click="showClearConfirm = true">{{ t('memory.clearAll') }}</button>
    </div>

    <!-- Add Memory Modal -->
    <Teleport to="body">
      <div v-if="showAddModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showAddModal = false">
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memory.addTitle') }}</h3>
          </div>
          <div class="p-6 space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memory.content') }} *</label>
              <textarea v-model="addContent" rows="4" :placeholder="t('memory.contentPlaceholder')" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white resize-none"></textarea>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('memory.tags') }}</label>
              <input v-model="addTags" type="text" :placeholder="t('memory.tagsPlaceholder')" class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" />
            </div>
          </div>
          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg" @click="showAddModal = false">{{ t('common.cancel') }}</button>
            <button class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg disabled:opacity-50" :disabled="loading || !addContent.trim()" @click="addMemory">{{ t('memory.add') }}</button>
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
