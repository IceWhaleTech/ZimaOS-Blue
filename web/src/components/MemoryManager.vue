<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { memoryApi, type MemorySearchResult, type MemoryStats } from '@/api/memory'
import SemanticSearchField from '@/components/ui/SemanticSearchField.vue'
import type { MemoryRecallMode } from '@/stores/settings'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    memoryRecallMode?: MemoryRecallMode
  }>(),
  {
    memoryRecallMode: 'balanced',
  }
)

const emit = defineEmits<{
  (e: 'status-change', message: string): void
  (e: 'memory-recall-mode-change', mode: MemoryRecallMode): void
}>()

const searchResults = ref<MemorySearchResult[]>([])
const searching = ref(false)
const searchQuery = ref('')
const stats = ref<MemoryStats | null>(null)
const loading = ref(false)
const deletingMemoryId = ref<string | null>(null)
const showClearConfirm = ref(false)
const showComposer = ref(false)
const activeActionModal = ref<'import' | 'recall' | 'cleanup' | null>(null)

const addContent = ref('')
const addTags = ref('')

const memoryExporting = ref(false)
const memoryImporting = ref(false)
const memoryError = ref<string | null>(null)
const memoryImportFile = ref<File | null>(null)
const memoryImportMode = ref<'append' | 'replace'>('append')
const memoryFileInputRef = ref<HTMLInputElement | null>(null)

const operationMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

const hasMemories = computed(() => (stats.value?.total_chunks ?? 0) > 0)
const hasSearchQuery = computed(() => searchQuery.value.trim().length > 0)
const displayCount = computed(
  () => stats.value?.total_display_count ?? stats.value?.total_chunks ?? 0
)
const totalSizeText = computed(() =>
  formatBytes(stats.value?.total_display_size_bytes ?? stats.value?.total_size_bytes ?? 0)
)

let searchDebounceTimer: number | null = null
let messageTimer: number | null = null
let searchSequence = 0

function clearSearchDebounce() {
  if (searchDebounceTimer !== null) {
    window.clearTimeout(searchDebounceTimer)
    searchDebounceTimer = null
  }
}

function resetSearchState() {
  searchSequence += 1
  searchResults.value = []
  searching.value = false
}

function setOperationMessage(type: 'success' | 'error', text: string) {
  operationMessage.value = { type, text }
  if (messageTimer !== null) window.clearTimeout(messageTimer)
  messageTimer = window.setTimeout(() => {
    operationMessage.value = null
  }, 3200)
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

async function loadStats() {
  try {
    const response = await memoryApi.stats()
    stats.value = response.data
  } catch (error) {
    console.error('Failed to load memory stats:', error)
    setOperationMessage('error', t('common.error'))
  }
}

async function searchMemories() {
  clearSearchDebounce()
  const query = searchQuery.value.replace(/\s+/g, ' ').trim()
  if (!query) {
    resetSearchState()
    return
  }

  const current = ++searchSequence
  searching.value = true

  try {
    const response = await memoryApi.search({ query, limit: 50 })
    if (current !== searchSequence) return
    searchResults.value = response.data.results
  } catch (error) {
    if (current !== searchSequence) return
    console.error('Failed to search memories:', error)
    setOperationMessage('error', t('common.error'))
  } finally {
    if (current === searchSequence) searching.value = false
  }
}

function clearSearch() {
  clearSearchDebounce()
  searchQuery.value = ''
  resetSearchState()
}

function resetComposer() {
  addContent.value = ''
  addTags.value = ''
}

function handleMemoryRecallModeSelect(mode: MemoryRecallMode) {
  if (mode === props.memoryRecallMode) return
  emit('memory-recall-mode-change', mode)
}

function selectMemoryRecallMode(mode: MemoryRecallMode) {
  handleMemoryRecallModeSelect(mode)
  activeActionModal.value = null
}

async function addMemory() {
  if (loading.value || !addContent.value.trim()) return

  loading.value = true
  try {
    const tags = addTags.value
      .split(',')
      .map((v) => v.trim())
      .filter(Boolean)
    await memoryApi.store({ content: addContent.value, tags: tags.length > 0 ? tags : undefined })
    resetComposer()
    showComposer.value = false
    await loadStats()
    emit('status-change', t('memory.add') + ' OK')
    setOperationMessage('success', t('memory.add') + ' OK')
  } catch (e) {
    console.error('Failed to add memory:', e)
    setOperationMessage('error', t('common.error'))
  } finally {
    loading.value = false
  }
}

async function deleteMemory(id: string) {
  if (loading.value || deletingMemoryId.value) return
  if (!confirm(t('memory.confirmDelete'))) return

  deletingMemoryId.value = id
  try {
    await memoryApi.delete(id)
    searchResults.value = searchResults.value.filter((m) => m.id !== id)
    await loadStats()
    setOperationMessage('success', t('common.delete') + ' OK')
  } catch (error) {
    console.error('Failed to delete memory:', error)
    setOperationMessage('error', t('common.error'))
  } finally {
    deletingMemoryId.value = null
  }
}

async function pruneMemories() {
  if (loading.value) return
  loading.value = true
  try {
    const response = await memoryApi.prune()
    await loadStats()
    setOperationMessage('success', t('memory.pruneSuccess', { count: response.data.deleted }))
  } catch (error) {
    console.error('Failed to prune memories:', error)
    setOperationMessage('error', t('common.error'))
  } finally {
    loading.value = false
  }
}

async function clearAllMemories() {
  if (loading.value) return
  loading.value = true
  try {
    await memoryApi.clear()
    searchResults.value = []
    stats.value = null
    showClearConfirm.value = false
    await loadStats()
    setOperationMessage('success', t('memory.clearAll'))
  } catch (error) {
    console.error('Failed to clear memories:', error)
    setOperationMessage('error', t('common.error'))
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
  if (memoryExporting.value) return
  memoryExporting.value = true
  memoryError.value = null
  try {
    const response = await memoryApi.exportMarkdown()
    const content =
      typeof response.data === 'string' ? response.data : JSON.stringify(response.data)
    const blob = new Blob([content], { type: 'text/markdown; charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `memory-export-${new Date().toISOString().split('T')[0]}.md`
    a.click()
    URL.revokeObjectURL(url)
    emit('status-change', t('userdata.memory.exportSuccess'))
    setOperationMessage('success', t('userdata.memory.exportSuccess'))
  } catch (e) {
    memoryError.value = e instanceof Error ? e.message : t('userdata.memory.exportFailed')
    setOperationMessage('error', memoryError.value)
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
  if (memoryImporting.value || !memoryImportFile.value) return

  memoryImporting.value = true
  memoryError.value = null
  try {
    const content = await readFileAsText(memoryImportFile.value)
    const response = await memoryApi.importMarkdown(content, memoryImportMode.value)
    if (response.data.errors && response.data.errors.length > 0) {
      memoryError.value = response.data.errors.join(', ')
    }
    emit('status-change', t('userdata.memory.importSuccess', { count: response.data.imported }))
    setOperationMessage(
      'success',
      t('userdata.memory.importSuccess', { count: response.data.imported })
    )
    memoryImportFile.value = null
    if (memoryFileInputRef.value) memoryFileInputRef.value.value = ''
    await loadStats()
  } catch (e) {
    memoryError.value = e instanceof Error ? e.message : t('userdata.memory.importFailed')
    setOperationMessage('error', t('userdata.memory.importFailed'))
  } finally {
    memoryImporting.value = false
  }
}

function closeActionModal() {
  if (memoryImporting.value) return
  activeActionModal.value = null
  memoryError.value = null
  memoryImportFile.value = null
  if (memoryFileInputRef.value) memoryFileInputRef.value.value = ''
}

async function handlePruneFromModal() {
  await pruneMemories()
  activeActionModal.value = null
}

function openClearConfirmFromModal() {
  activeActionModal.value = null
  showClearConfirm.value = true
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}

function getScoreColor(score: number): string {
  if (score >= 0.8) return 'text-green-600 dark:text-green-400'
  if (score >= 0.5) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-gray-500 dark:text-gray-400'
}

function formatMatchType(type: string): string {
  switch (type.trim().toLowerCase()) {
    case 'keyword':
      return t('memory.matchTypes.keyword')
    case 'vector':
      return t('memory.matchTypes.vector')
    case 'exact':
      return t('memory.matchTypes.exact')
    case 'partial':
      return t('memory.matchTypes.partial')
    case 'heading':
      return t('memory.matchTypes.heading')
    default:
      return type
  }
}

watch(searchQuery, () => {
  clearSearchDebounce()

  if (!searchQuery.value.trim()) {
    resetSearchState()
    return
  }

  searchDebounceTimer = window.setTimeout(() => {
    searchDebounceTimer = null
    void searchMemories()
  }, 300)
})

onMounted(() => {
  void loadStats()
})

onBeforeUnmount(() => {
  if (searchDebounceTimer !== null) window.clearTimeout(searchDebounceTimer)
  if (messageTimer !== null) window.clearTimeout(messageTimer)
})
</script>

<template>
  <div
    class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/50 shadow-sm overflow-hidden"
  >
    <div class="px-5 py-4 border-b border-gray-200 dark:border-gray-700">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('memory.title') }}
          </h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
            {{ t('memory.description') }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            data-testid="memory-toggle-composer"
            class="px-3.5 py-1.5 bg-gray-800 dark:bg-gray-500 hover:bg-gray-900 dark:hover:bg-gray-400 text-white text-sm font-medium rounded-lg transition-colors"
            @click="showComposer = !showComposer"
          >
            {{ t('common.add') }}
          </button>
          <button
            data-testid="memory-open-import"
            class="px-3.5 py-1.5 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-200 text-sm font-medium rounded-lg transition-colors"
            @click="activeActionModal = 'import'"
          >
            {{ t('userdata.tabs.import') }}
          </button>
          <button
            data-testid="memory-export-button-inline"
            :disabled="memoryExporting"
            class="px-3.5 py-1.5 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-200 text-sm font-medium rounded-lg transition-colors disabled:opacity-50"
            @click="handleMemoryExport"
          >
            {{ memoryExporting ? t('userdata.exporting') : t('userdata.tabs.export') }}
          </button>
          <button
            data-testid="memory-open-recall-settings"
            class="px-3.5 py-1.5 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-200 text-sm font-medium rounded-lg transition-colors"
            @click="activeActionModal = 'recall'"
          >
            {{ t('memory.recallSettings') }}
          </button>
          <button
            data-testid="memory-open-cleanup"
            class="px-3.5 py-1.5 bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30 text-red-700 dark:text-red-300 text-sm font-medium rounded-lg transition-colors"
            @click="activeActionModal = 'cleanup'"
          >
            {{ t('userdata.tabs.cleanup') }}
          </button>
        </div>
      </div>

      <div class="grid grid-cols-2 lg:grid-cols-4 gap-2 mt-3">
        <div
          class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/20 px-3 py-2.5"
        >
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('memory.totalMemories') }}
          </div>
          <div class="text-base font-semibold text-gray-900 dark:text-white">
            {{ displayCount }}
          </div>
        </div>
        <div
          class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/20 px-3 py-2.5"
        >
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('memory.totalSize') }}
          </div>
          <div class="text-base font-semibold text-gray-900 dark:text-white">
            {{ totalSizeText }}
          </div>
        </div>
        <div
          class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/20 px-3 py-2.5"
        >
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('memory.oldest') }}
          </div>
          <div class="text-sm font-medium text-gray-900 dark:text-white truncate">
            {{ formatDate(stats?.oldest_chunk) }}
          </div>
        </div>
        <div
          class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/20 px-3 py-2.5"
        >
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('memory.newest') }}
          </div>
          <div class="text-sm font-medium text-gray-900 dark:text-white truncate">
            {{ formatDate(stats?.newest_chunk) }}
          </div>
        </div>
      </div>

      <div
        v-if="operationMessage"
        class="mt-3 rounded-lg px-3 py-2 text-sm border"
        :class="
          operationMessage.type === 'success'
            ? 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900/20 dark:text-green-300'
            : 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
        "
      >
        {{ operationMessage.text }}
      </div>
    </div>

    <div class="p-5 space-y-5">
      <div
        v-if="showComposer"
        class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/10 p-3.5 space-y-3"
      >
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('memory.addTitle') }}
        </h3>
        <div>
          <label class="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">{{
            t('memory.content')
          }}</label>
          <textarea
            v-model="addContent"
            data-testid="memory-add-content"
            rows="4"
            :placeholder="t('memory.contentPlaceholder')"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white resize-none"
          />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">{{
            t('memory.tags')
          }}</label>
          <input
            v-model="addTags"
            data-testid="memory-add-tags"
            type="text"
            :placeholder="t('memory.tagsPlaceholder')"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
          >
        </div>
        <div class="flex gap-2">
          <button
            data-testid="memory-add-cancel"
            class="flex-1 px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700"
            @click="showComposer = false"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            data-testid="memory-add-submit"
            class="flex-1 px-3 py-2 text-sm rounded-lg bg-gray-800 dark:bg-gray-500 hover:bg-gray-900 dark:hover:bg-gray-400 text-white disabled:opacity-50"
            :disabled="loading || !addContent.trim()"
            @click="addMemory"
          >
            {{ loading ? t('common.saving') : t('memory.add') }}
          </button>
        </div>
      </div>

      <section>
        <div>
          <SemanticSearchField
            v-model="searchQuery"
            test-id="memory-search-input"
            class="memory-search-field flex-1"
            :placeholder="t('memory.searchPlaceholder')"
            :clear-label="t('common.clear')"
            @clear="clearSearch"
            @submit-shortcut="searchMemories"
          />
        </div>

        <div
          v-if="searching || hasSearchQuery"
          data-testid="memory-search-results-panel"
          class="mt-3 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden min-h-[280px]"
        >
          <div
            v-if="searching"
            class="p-8 text-center"
          >
            <div
              class="animate-spin h-8 w-8 border-4 border-gray-900 dark:border-white border-t-transparent rounded-full mx-auto"
            />
          </div>

          <template v-else-if="searchResults.length > 0">
            <div
              class="px-4 py-2 text-xs text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/20"
            >
              {{ searchResults.length }} {{ t('memory.totalMemories') }}
            </div>
            <div
              class="max-h-[480px] overflow-y-auto divide-y divide-gray-200 dark:divide-gray-700"
            >
              <article
                v-for="memory in searchResults"
                :key="memory.id"
                class="p-3.5 hover:bg-gray-50 dark:hover:bg-gray-800/40 transition-colors"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0 flex-1">
                    <p
                      class="text-sm text-gray-900 dark:text-white whitespace-pre-wrap break-words leading-relaxed"
                    >
                      {{ memory.content }}
                    </p>
                    <div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
                      <span class="text-gray-500 dark:text-gray-400">{{
                        formatDate(memory.created_at)
                      }}</span>
                      <span :class="getScoreColor(memory.score)">{{ (memory.score * 100).toFixed(1) }}%</span>
                      <span
                        v-for="mt in memory.match_types"
                        :key="mt"
                        class="px-2 py-0.5 font-medium rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300"
                      >
                        {{ formatMatchType(mt) }}
                      </span>
                    </div>
                  </div>
                  <button
                    :data-testid="`memory-delete-${memory.id}`"
                    :disabled="loading || deletingMemoryId !== null"
                    class="p-1.5 text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed"
                    @click="deleteMemory(memory.id)"
                  >
                    <svg
                      v-if="deletingMemoryId === memory.id"
                      xmlns="http://www.w3.org/2000/svg"
                      class="h-4 w-4 animate-spin"
                      fill="none"
                      viewBox="0 0 24 24"
                    >
                      <circle
                        class="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        stroke-width="4"
                      />
                      <path
                        class="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      />
                    </svg>
                    <svg
                      v-else
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
              </article>
            </div>
          </template>

          <div
            v-else
            class="p-8 text-center text-gray-500 dark:text-gray-400"
          >
            {{ t('memory.noResults') }}
          </div>
        </div>
      </section>
    </div>

    <Teleport to="body">
      <div
        v-if="showClearConfirm"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="showClearConfirm = false"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 class="text-lg font-semibold text-red-600 dark:text-red-400">
              {{ t('memory.clearAllTitle') }}
            </h3>
          </div>
          <div class="p-6">
            <p class="text-gray-700 dark:text-gray-300">
              {{ t('memory.clearAllWarning') }}
            </p>
          </div>
          <div
            class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3"
          >
            <button
              data-testid="memory-clear-cancel"
              class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
              :disabled="loading"
              @click="showClearConfirm = false"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              data-testid="memory-clear-confirm"
              class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg disabled:opacity-50"
              :disabled="loading"
              @click="clearAllMemories"
            >
              {{ loading ? t('common.processing') : t('memory.clearAll') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="activeActionModal === 'import'"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="closeActionModal"
      >
        <div
          class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 max-h-[80vh] overflow-y-auto"
        >
          <div
            class="sticky top-0 bg-white dark:bg-gray-800 px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
          >
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('userdata.memory.importSection') }}
            </h3>
            <button
              data-testid="memory-import-close"
              class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              :disabled="memoryImporting"
              @click="closeActionModal"
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
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <div class="p-6 space-y-4">
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('userdata.memory.importDesc') }}
            </p>
            <div class="flex items-center gap-2">
              <input
                ref="memoryFileInputRef"
                data-testid="memory-import-file-input"
                type="file"
                accept=".md,.markdown,.txt"
                class="hidden"
                @change="handleMemoryFileSelect"
              >
              <button
                data-testid="memory-import-file-picker"
                class="px-4 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg text-sm"
                :disabled="memoryImporting"
                @click="memoryFileInputRef?.click()"
              >
                {{ t('userdata.chooseFile') }}
              </button>
              <span
                v-if="memoryImportFile"
                class="text-sm text-gray-600 dark:text-gray-300 truncate flex-1"
              >{{ memoryImportFile.name }}</span>
            </div>
            <div>
              <label class="block text-sm text-gray-500 dark:text-gray-400 mb-2">{{
                t('userdata.memory.importMode')
              }}</label>
              <div class="flex gap-2">
                <button
                  data-testid="memory-import-mode-append"
                  :disabled="memoryImporting"
                  :class="[
                    'flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors',
                    memoryImportMode === 'append'
                      ? 'bg-gray-700 dark:bg-gray-500 text-white'
                      : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300',
                  ]"
                  @click="memoryImportMode = 'append'"
                >
                  {{ t('userdata.memory.modeAppend') }}
                </button>
                <button
                  data-testid="memory-import-mode-replace"
                  :disabled="memoryImporting"
                  :class="[
                    'flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors',
                    memoryImportMode === 'replace'
                      ? 'bg-red-600 text-white'
                      : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300',
                  ]"
                  @click="memoryImportMode = 'replace'"
                >
                  {{ t('userdata.memory.modeReplace') }}
                </button>
              </div>
            </div>
            <button
              data-testid="memory-import-button"
              :disabled="!memoryImportFile || memoryImporting"
              class="w-full px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg text-sm font-medium disabled:opacity-50 flex items-center justify-center gap-2"
              @click="handleMemoryImport"
            >
              <svg
                v-if="memoryImporting"
                class="animate-spin h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
              {{ memoryImporting ? t('userdata.importing') : t('userdata.memory.importButton') }}
            </button>
            <div
              v-if="memoryError"
              class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-300 text-sm"
            >
              {{ memoryError }}
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="activeActionModal === 'recall'"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="closeActionModal"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div
            class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
          >
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('settings.memoryRecallMode.title') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
                {{ t('settings.memoryRecallMode.description') }}
              </p>
            </div>
            <button
              class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              @click="closeActionModal"
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
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <div class="p-6 grid grid-cols-1 gap-2">
            <button
              v-for="mode in ['aggressive', 'balanced', 'quality'] as const"
              :key="mode"
              :data-testid="`memory-recall-mode-${mode}`"
              class="w-full px-3 py-2 rounded-lg text-sm transition-colors border text-start"
              :class="
                props.memoryRecallMode === mode
                  ? 'bg-gray-100 dark:bg-gray-700/30 border-gray-300 dark:border-gray-500 text-gray-900 dark:text-white'
                  : 'bg-gray-50 dark:bg-slate-700/30 border-gray-200 dark:border-slate-600 text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-700/60'
              "
              @click="selectMemoryRecallMode(mode)"
            >
              <div class="font-medium">
                {{ t(`settings.memoryRecallMode.options.${mode}.label`) }}
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                {{ t(`settings.memoryRecallMode.options.${mode}.hint`) }}
              </div>
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="activeActionModal === 'cleanup'"
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="closeActionModal"
      >
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4">
          <div
            class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
          >
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('userdata.tabs.cleanup') }}
            </h3>
            <button
              class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              @click="closeActionModal"
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
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <div class="p-6 space-y-3">
            <button
              data-testid="memory-prune"
              class="w-full px-3 py-2 text-sm rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-200 transition-colors disabled:opacity-50"
              :disabled="loading || !hasMemories"
              @click="handlePruneFromModal"
            >
              {{ loading ? t('common.processing') : t('memory.prune') }}
            </button>
            <button
              data-testid="memory-open-clear"
              class="w-full px-3 py-2 text-sm rounded-lg bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30 text-red-700 dark:text-red-300 transition-colors disabled:opacity-50"
              :disabled="loading || !hasMemories"
              @click="openClearConfirmFromModal"
            >
              {{ t('memory.clearAll') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.memory-search-field {
  min-width: 0;
}

.memory-search-field :deep(.semantic-search-shell) {
  box-sizing: border-box;
  height: 44px;
  min-height: 44px;
  padding: 9px 12px 9px 14px;
  gap: 10px;
  border-radius: 18px;
}

.memory-search-field :deep(.semantic-search-input) {
  height: 22px;
  font-size: 0.92rem;
}

.memory-search-field :deep(.semantic-search-icon) {
  width: 16px;
  height: 16px;
}

.memory-search-field :deep(.semantic-search-clear) {
  width: 28px;
  height: 28px;
}
</style>
