<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { memoryApi, type MemorySearchResult, type MemoryStats } from '@/api/memory'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'status-change', message: string): void
}>()

// State
const memories = ref<MemorySearchResult[]>([])
const stats = ref<MemoryStats | null>(null)
const loading = ref(false)
const searching = ref(false)
const searchQuery = ref('')
const searchType = ref<'hybrid' | 'vector' | 'keyword'>('hybrid')
const showAddModal = ref(false)
const showClearConfirm = ref(false)
const showSettingsModal = ref(false)
const showExportImportModal = ref(false)
const newMemoryContent = ref('')
const newMemoryTags = ref('')

// Supermemory settings
const activeBackend = ref<'local' | 'supermemory'>('local')
const supermemoryEnabled = ref(false)
const supermemoryApiKey = ref('')
const supermemoryBaseUrl = ref('')
const testingConnection = ref(false)
const connectionTestResult = ref<{ success: boolean; error?: string } | null>(null)

// Export/Import state
const memoryExporting = ref(false)
const memoryImporting = ref(false)
const memoryError = ref<string | null>(null)
const memoryImportFile = ref<File | null>(null)
const memoryImportMode = ref<'append' | 'replace'>('append')
const memoryFileInputRef = ref<HTMLInputElement | null>(null)

// Computed
const hasMemories = computed(() => memories.value.length > 0 || stats.value?.total_chunks)

// Methods
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
    activeBackend.value = response.data.active_backend as 'local' | 'supermemory'
    supermemoryEnabled.value = response.data.supermemory_available
  } catch (error) {
    console.error('Failed to load backend status:', error)
  }
}

async function searchMemories() {
  if (!searchQuery.value.trim()) {
    memories.value = []
    return
  }

  searching.value = true
  try {
    const response = await memoryApi.search({
      query: searchQuery.value,
      limit: 50,
      search_type: searchType.value,
    })
    memories.value = response.data.results
  } catch (error) {
    console.error('Failed to search memories:', error)
  } finally {
    searching.value = false
  }
}

async function addMemory() {
  if (!newMemoryContent.value.trim()) return

  loading.value = true
  try {
    const tags = newMemoryTags.value
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean)

    await memoryApi.store({
      content: newMemoryContent.value,
      tags: tags.length > 0 ? tags : undefined,
    })

    newMemoryContent.value = ''
    newMemoryTags.value = ''
    showAddModal.value = false
    await loadStats()

    if (searchQuery.value) {
      await searchMemories()
    }
  } catch (error) {
    console.error('Failed to add memory:', error)
  } finally {
    loading.value = false
  }
}

async function deleteMemory(id: string) {
  if (!confirm(t('memory.confirmDelete'))) return

  try {
    await memoryApi.delete(id)
    memories.value = memories.value.filter((m) => m.id !== id)
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
    if (searchQuery.value) {
      await searchMemories()
    }
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
    memories.value = []
    stats.value = null
    showClearConfirm.value = false
    await loadStats()
  } catch (error) {
    console.error('Failed to clear memories:', error)
  } finally {
    loading.value = false
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function formatSize(bytes: number | undefined | null): string {
  if (bytes === undefined || bytes === null || isNaN(bytes)) return '-'
  if (bytes === 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function getScoreColor(score: number): string {
  if (score >= 0.8) return 'text-green-600 dark:text-green-400'
  if (score >= 0.5) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-gray-500 dark:text-gray-400'
}

async function switchBackend(backend: 'local' | 'supermemory') {
  try {
    await memoryApi.setBackend(backend)
    activeBackend.value = backend
    await loadStats()
  } catch (error) {
    console.error('Failed to switch backend:', error)
  }
}

async function saveSupermemoryConfig() {
  loading.value = true
  try {
    await memoryApi.configureSupermemory({
      enabled: supermemoryEnabled.value,
      api_key: supermemoryApiKey.value,
      base_url: supermemoryBaseUrl.value || undefined,
    })
    await loadBackendStatus()
    showSettingsModal.value = false
  } catch (error) {
    console.error('Failed to save supermemory config:', error)
  } finally {
    loading.value = false
  }
}

async function testSupermemoryConnection() {
  testingConnection.value = true
  connectionTestResult.value = null
  try {
    const response = await memoryApi.testSupermemory()
    connectionTestResult.value = response.data
  } catch (error) {
    connectionTestResult.value = { success: false, error: String(error) }
  } finally {
    testingConnection.value = false
  }
}

// Export/Import functions
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
    const timestamp = new Date().toISOString().split('T')[0]
    a.download = `memory-export-${timestamp}.md`
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
    if (memoryFileInputRef.value) {
      memoryFileInputRef.value.value = ''
    }

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
  if (memoryFileInputRef.value) {
    memoryFileInputRef.value.value = ''
  }
}

onMounted(() => {
  loadStats()
  loadBackendStatus()
})
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg shadow">
    <!-- Header -->
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('memory.title') }}
          </h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
            {{ t('memory.description') }}
          </p>
        </div>
        <button
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors"
          @click="showAddModal = true"
        >
          {{ t('memory.add') }}
        </button>
        <button
          class="p-2 text-purple-500 hover:text-purple-700 dark:text-purple-400 dark:hover:text-purple-200 hover:bg-purple-50 dark:hover:bg-purple-900/20 rounded-lg transition-colors"
          :title="t('userdata.memory.title')"
          @click="showExportImportModal = true"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
          </svg>
        </button>
        <button
          class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          :title="t('common.settings')"
          @click="showSettingsModal = true"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Backend indicator -->
    <div v-if="stats?.backend" class="px-6 py-2 bg-blue-50 dark:bg-blue-900/20 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center gap-2 text-sm">
        <span class="text-gray-600 dark:text-gray-400">{{ t('memory.backend') }}:</span>
        <span class="font-medium text-blue-600 dark:text-blue-400">
          {{ stats.backend === 'supermemory' ? 'Supermemory' : t('memory.localBackend') }}
        </span>
      </div>
    </div>

    <!-- Stats -->
    <div v-if="stats" class="px-6 py-4 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700">
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div>
          <div class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ stats.total_chunks }}
          </div>
          <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('memory.totalMemories') }}</div>
        </div>
        <div>
          <div class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ formatSize(stats.total_size_bytes) }}
          </div>
          <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('memory.totalSize') }}</div>
        </div>
        <div v-if="stats.oldest_chunk">
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ formatDate(stats.oldest_chunk) }}
          </div>
          <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('memory.oldest') }}</div>
        </div>
        <div v-if="stats.newest_chunk">
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ formatDate(stats.newest_chunk) }}
          </div>
          <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('memory.newest') }}</div>
        </div>
      </div>
    </div>

    <!-- Search -->
    <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
      <div class="flex gap-3">
        <div class="flex-1">
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('memory.searchPlaceholder')"
            class="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            @keyup.enter="searchMemories"
          />
        </div>
        <select
          v-model="searchType"
          class="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
        >
          <option value="hybrid">{{ t('memory.searchTypes.hybrid') }}</option>
          <option value="vector">{{ t('memory.searchTypes.vector') }}</option>
          <option value="keyword">{{ t('memory.searchTypes.keyword') }}</option>
        </select>
        <button
          class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50"
          :disabled="searching || !searchQuery.trim()"
          @click="searchMemories"
        >
          <span v-if="searching">{{ t('common.searching') }}</span>
          <span v-else>{{ t('common.search') }}</span>
        </button>
      </div>
    </div>

    <!-- Results -->
    <div v-if="searching" class="p-6 text-center">
      <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"></div>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('memory.searching') }}</p>
    </div>

    <div v-else-if="memories.length > 0" class="divide-y divide-gray-200 dark:divide-gray-700">
      <div
        v-for="memory in memories"
        :key="memory.id"
        class="px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-700/50"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="flex-1 min-w-0">
            <p class="text-gray-900 dark:text-white whitespace-pre-wrap break-words">
              {{ memory.content }}
            </p>
            <div class="mt-2 flex flex-wrap items-center gap-3 text-sm">
              <span class="text-gray-500 dark:text-gray-400">
                {{ formatDate(memory.created_at) }}
              </span>
              <span :class="getScoreColor(memory.score)">
                {{ t('memory.score') }}: {{ (memory.score * 100).toFixed(1) }}%
              </span>
              <span
                v-for="matchType in memory.match_types"
                :key="matchType"
                class="px-2 py-0.5 text-xs font-medium rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400"
              >
                {{ matchType }}
              </span>
            </div>
          </div>
          <button
            class="p-2 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors flex-shrink-0"
            :title="t('common.delete')"
            @click="deleteMemory(memory.id)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>
      </div>
    </div>

    <div v-else-if="searchQuery && !searching" class="p-6 text-center">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('memory.noResults') }}</p>
    </div>

    <div v-else class="p-6 text-center">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
      </svg>
      <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('memory.searchHint') }}</p>
    </div>

    <!-- Actions -->
    <div v-if="hasMemories" class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
      <button
        class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
        :disabled="loading"
        @click="pruneMemories"
      >
        {{ t('memory.prune') }}
      </button>
      <button
        class="px-4 py-2 text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 rounded-lg transition-colors"
        :disabled="loading"
        @click="showClearConfirm = true"
      >
        {{ t('memory.clearAll') }}
      </button>
    </div>

    <!-- Add Memory Modal -->
    <Teleport to="body">
      <div
        v-if="showAddModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="showAddModal = false"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memory.addTitle') }}</h3>
          </div>

          <div class="p-6 space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('memory.content') }} *
              </label>
              <textarea
                v-model="newMemoryContent"
                rows="4"
                :placeholder="t('memory.contentPlaceholder')"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
              ></textarea>
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {{ t('memory.tags') }}
              </label>
              <input
                v-model="newMemoryTags"
                type="text"
                :placeholder="t('memory.tagsPlaceholder')"
                class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
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
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50"
              :disabled="loading || !newMemoryContent.trim()"
              @click="addMemory"
            >
              {{ t('memory.add') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Clear Confirm Modal -->
    <Teleport to="body">
      <div
        v-if="showClearConfirm"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="showClearConfirm = false"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-red-600 dark:text-red-400">{{ t('memory.clearAllTitle') }}</h3>
          </div>

          <div class="p-6">
            <p class="text-gray-700 dark:text-gray-300">{{ t('memory.clearAllWarning') }}</p>
          </div>

          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button
              class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              @click="showClearConfirm = false"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg transition-colors disabled:opacity-50"
              :disabled="loading"
              @click="clearAllMemories"
            >
              {{ t('memory.clearAll') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Settings Modal -->
    <Teleport to="body">
      <div
        v-if="showSettingsModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="showSettingsModal = false"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('memory.settingsTitle') }}</h3>
          </div>

          <div class="p-6 space-y-6">
            <!-- Backend Selection -->
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                {{ t('memory.backend') }}
              </label>
              <div class="flex gap-2">
                <button
                  class="flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors"
                  :class="activeBackend === 'local' ? 'bg-blue-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
                  @click="switchBackend('local')"
                >
                  {{ t('memory.localBackend') }}
                </button>
                <button
                  class="flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors"
                  :class="activeBackend === 'supermemory' ? 'bg-blue-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
                  :disabled="!supermemoryEnabled"
                  @click="switchBackend('supermemory')"
                >
                  Supermemory
                </button>
              </div>
            </div>

            <!-- Supermemory Config -->
            <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">{{ t('memory.supermemoryConfig') }}</h4>

              <div class="space-y-4">
                <div>
                  <label class="block text-sm text-gray-600 dark:text-gray-400 mb-1">
                    {{ t('memory.apiKey') }}
                  </label>
                  <input
                    v-model="supermemoryApiKey"
                    type="password"
                    :placeholder="t('memory.apiKeyPlaceholder')"
                    class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                  />
                </div>

                <div>
                  <label class="block text-sm text-gray-600 dark:text-gray-400 mb-1">
                    {{ t('memory.baseUrl') }} ({{ t('common.optional') }})
                  </label>
                  <input
                    v-model="supermemoryBaseUrl"
                    type="text"
                    placeholder="https://api.supermemory.ai/v3"
                    class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                  />
                </div>

                <div class="flex items-center gap-3">
                  <button
                    class="px-4 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-lg transition-colors"
                    :disabled="testingConnection || !supermemoryApiKey"
                    @click="testSupermemoryConnection"
                  >
                    {{ testingConnection ? t('common.testing') : t('memory.testConnection') }}
                  </button>
                  <span v-if="connectionTestResult" :class="connectionTestResult.success ? 'text-green-600' : 'text-red-600'" class="text-sm">
                    {{ connectionTestResult.success ? t('memory.connectionSuccess') : connectionTestResult.error }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button
              class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              @click="showSettingsModal = false"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50"
              :disabled="loading"
              @click="saveSupermemoryConfig"
            >
              {{ t('common.save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Export/Import Modal -->
    <Teleport to="body">
      <div
        v-if="showExportImportModal"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="closeExportImportModal"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 max-h-[80vh] overflow-y-auto">
          <div class="sticky top-0 bg-white dark:bg-gray-800 px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
              </svg>
              {{ t('userdata.memory.title') }}
            </h3>
            <button class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" @click="closeExportImportModal">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <div class="p-6 space-y-6">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('userdata.memory.description') }}</p>

            <!-- Memory Stats -->
            <div v-if="stats" class="p-3 bg-purple-50 dark:bg-purple-900/20 rounded-lg">
              <div class="flex justify-between text-sm">
                <span class="text-purple-700 dark:text-purple-300">{{ t('userdata.memory.totalMemories') }}</span>
                <span class="font-medium text-purple-800 dark:text-purple-200">{{ stats.total_chunks }}</span>
              </div>
              <div class="flex justify-between text-sm mt-1">
                <span class="text-purple-700 dark:text-purple-300">{{ t('userdata.memory.backend') }}</span>
                <span class="font-medium text-purple-800 dark:text-purple-200">{{ stats.backend || 'local' }}</span>
              </div>
            </div>

            <!-- Export Section -->
            <div class="space-y-2">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.memory.exportSection') }}</h4>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('userdata.memory.exportDesc') }}</p>
              <button
                :disabled="memoryExporting"
                class="w-full px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                @click="handleMemoryExport"
              >
                <svg v-if="memoryExporting" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                </svg>
                {{ memoryExporting ? t('userdata.exporting') : t('userdata.memory.exportButton') }}
              </button>
            </div>

            <hr class="border-gray-200 dark:border-gray-700" />

            <!-- Import Section -->
            <div class="space-y-3">
              <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userdata.memory.importSection') }}</h4>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('userdata.memory.importDesc') }}</p>

              <!-- File Selection -->
              <div class="flex items-center gap-2">
                <input
                  ref="memoryFileInputRef"
                  type="file"
                  accept=".md,.markdown,.txt"
                  class="hidden"
                  @change="handleMemoryFileSelect"
                />
                <button
                  class="px-4 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg text-sm transition-colors"
                  @click="memoryFileInputRef?.click()"
                >
                  {{ t('userdata.chooseFile') }}
                </button>
                <span v-if="memoryImportFile" class="text-sm text-gray-600 dark:text-gray-300 truncate flex-1">
                  {{ memoryImportFile.name }}
                </span>
              </div>

              <!-- Import Mode -->
              <div>
                <label class="block text-sm text-gray-500 dark:text-gray-400 mb-2">{{ t('userdata.memory.importMode') }}</label>
                <div class="flex gap-2">
                  <button
                    :class="[
                      'flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors',
                      memoryImportMode === 'append'
                        ? 'bg-purple-600 text-white'
                        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                    ]"
                    @click="memoryImportMode = 'append'"
                  >
                    {{ t('userdata.memory.modeAppend') }}
                  </button>
                  <button
                    :class="[
                      'flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors',
                      memoryImportMode === 'replace'
                        ? 'bg-red-600 text-white'
                        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                    ]"
                    @click="memoryImportMode = 'replace'"
                  >
                    {{ t('userdata.memory.modeReplace') }}
                  </button>
                </div>
                <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">
                  {{ memoryImportMode === 'append' ? t('userdata.memory.modeAppendDesc') : t('userdata.memory.modeReplaceDesc') }}
                </p>
              </div>

              <!-- Import Button -->
              <button
                :disabled="!memoryImportFile || memoryImporting"
                class="w-full px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                @click="handleMemoryImport"
              >
                <svg v-if="memoryImporting" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>
                {{ memoryImporting ? t('userdata.importing') : t('userdata.memory.importButton') }}
              </button>
            </div>

            <!-- Error -->
            <div v-if="memoryError" class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-300 text-sm">
              {{ memoryError }}
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
