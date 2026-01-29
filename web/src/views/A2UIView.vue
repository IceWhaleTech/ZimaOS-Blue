<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { listCanvases, getCanvas, createCanvas, updateCanvas, deleteCanvas, type Canvas } from '@/api/a2ui'
import A2UICanvas from '@/components/a2ui/A2UICanvas.vue'
import CanvasFormDialog from '@/components/a2ui/CanvasFormDialog.vue'

const { t } = useI18n()

// State
const canvases = ref<string[]>([])
const selectedCanvasId = ref<string | null>(null)
const selectedCanvas = ref<Canvas | null>(null)
const loading = ref(true)
const loadingCanvas = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)

// Dialog state
const showFormDialog = ref(false)
const editingCanvas = ref<Canvas | null>(null)

// Search and filter
const searchQuery = ref('')
const sortBy = ref<'name' | 'date'>('name')
const sortOrder = ref<'asc' | 'desc'>('asc')

// Computed
const filteredCanvases = computed(() => {
  let result = [...canvases.value]

  // Filter by search query
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(id => id.toLowerCase().includes(query))
  }

  // Sort
  result.sort((a, b) => {
    const comparison = a.localeCompare(b)
    return sortOrder.value === 'asc' ? comparison : -comparison
  })

  return result
})

// Fetch canvases list
async function fetchCanvases() {
  loading.value = true
  error.value = null
  try {
    const response = await listCanvases()
    canvases.value = response.canvases || []
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load canvases'
    canvases.value = []
  } finally {
    loading.value = false
  }
}

// Load a specific canvas
async function loadCanvas(canvasId: string) {
  loadingCanvas.value = true
  selectedCanvasId.value = canvasId
  try {
    selectedCanvas.value = await getCanvas(canvasId)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load canvas'
    selectedCanvas.value = null
  } finally {
    loadingCanvas.value = false
  }
}

// Open create dialog
function openCreateDialog() {
  editingCanvas.value = null
  showFormDialog.value = true
}

// Open edit dialog
async function openEditDialog(canvasId: string) {
  loadingCanvas.value = true
  try {
    const canvas = await getCanvas(canvasId)
    editingCanvas.value = canvas
    showFormDialog.value = true
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load canvas'
  } finally {
    loadingCanvas.value = false
  }
}

// Save canvas (create or update)
async function handleSaveCanvas(data: Omit<Canvas, 'created_at'>) {
  saving.value = true
  error.value = null
  try {
    if (editingCanvas.value) {
      // Update existing canvas
      await updateCanvas(data.id, data)
      // Refresh the canvas if it's currently selected
      if (selectedCanvasId.value === data.id) {
        selectedCanvas.value = await getCanvas(data.id)
      }
    } else {
      // Create new canvas
      await createCanvas(data)
      // Add to list if not already present
      if (!canvases.value.includes(data.id)) {
        canvases.value.push(data.id)
      }
    }
    showFormDialog.value = false
    editingCanvas.value = null
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to save canvas'
  } finally {
    saving.value = false
  }
}

// Delete a canvas
async function handleDeleteCanvas(canvasId: string) {
  if (!confirm(t('a2ui.confirmDelete'))) return

  try {
    await deleteCanvas(canvasId)
    canvases.value = canvases.value.filter(id => id !== canvasId)
    if (selectedCanvasId.value === canvasId) {
      selectedCanvasId.value = null
      selectedCanvas.value = null
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to delete canvas'
  }
}

// Duplicate a canvas
async function handleDuplicateCanvas(canvasId: string) {
  loadingCanvas.value = true
  try {
    const original = await getCanvas(canvasId)
    const newId = `${canvasId}-copy-${Date.now()}`
    const duplicate: Omit<Canvas, 'created_at'> = {
      ...original,
      id: newId,
      title: original.title ? `${original.title} (Copy)` : undefined,
    }
    await createCanvas(duplicate)
    canvases.value.push(newId)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to duplicate canvas'
  } finally {
    loadingCanvas.value = false
  }
}

// Export canvas as JSON
async function handleExportCanvas(canvasId: string) {
  try {
    const canvas = await getCanvas(canvasId)
    const json = JSON.stringify(canvas, null, 2)
    const blob = new Blob([json], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${canvasId}.json`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to export canvas'
  }
}

// Import canvas from JSON
function handleImportCanvas() {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = '.json'
  input.onchange = async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return

    try {
      const text = await file.text()
      const canvas = JSON.parse(text) as Canvas
      // Ensure unique ID
      if (canvases.value.includes(canvas.id)) {
        canvas.id = `${canvas.id}-imported-${Date.now()}`
      }
      await createCanvas(canvas)
      canvases.value.push(canvas.id)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to import canvas'
    }
  }
  input.click()
}

// Handle canvas update from action
function handleCanvasUpdate(canvas: Canvas) {
  selectedCanvas.value = canvas
}

// Handle action error
function handleError(errorMsg: string) {
  error.value = errorMsg
}

// Close canvas detail
function closeCanvas() {
  selectedCanvasId.value = null
  selectedCanvas.value = null
}

// Toggle sort order
function toggleSort(field: 'name' | 'date') {
  if (sortBy.value === field) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = field
    sortOrder.value = 'asc'
  }
}

onMounted(() => {
  fetchCanvases()
})
</script>

<template>
  <div class="p-6 max-w-7xl mx-auto">
    <!-- Header -->
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('a2ui.title') }}
        </h1>
        <p class="mt-1 text-gray-500 dark:text-slate-400">
          {{ t('a2ui.subtitle') }}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button
          class="px-4 py-2 rounded-lg bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white transition-colors flex items-center space-x-2"
          @click="handleImportCanvas"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
          </svg>
          <span>{{ t('a2ui.import') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white transition-colors flex items-center space-x-2"
          @click="fetchCanvases"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          <span>{{ t('common.refresh') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white transition-colors flex items-center space-x-2"
          @click="openCreateDialog"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          <span>{{ t('a2ui.createCanvas') }}</span>
        </button>
      </div>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="mb-6 p-4 rounded-lg bg-red-500/10 border border-red-500/30 text-red-500">
      <div class="flex items-center justify-between">
        <div class="flex items-center space-x-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span>{{ error }}</span>
        </div>
        <button class="p-1 hover:bg-red-500/20 rounded" @click="error = null">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
    </div>

    <!-- Canvas Detail View -->
    <div v-else-if="selectedCanvas" class="glass-card">
      <div class="p-4 border-b border-gray-200 dark:border-glass-border flex items-center justify-between">
        <div class="flex items-center space-x-3">
          <button
            class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
            @click="closeCanvas"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
            </svg>
          </button>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ selectedCanvas.title || selectedCanvasId }}
          </h2>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="p-2 rounded-lg text-gray-500 hover:text-blue-500 hover:bg-blue-500/10 transition-colors"
            :title="t('common.edit')"
            @click="openEditDialog(selectedCanvasId!)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
            </svg>
          </button>
          <button
            class="p-2 rounded-lg text-gray-500 hover:text-green-500 hover:bg-green-500/10 transition-colors"
            :title="t('a2ui.export')"
            @click="handleExportCanvas(selectedCanvasId!)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>
          <button
            class="p-2 rounded-lg text-red-500 hover:bg-red-500/10 transition-colors"
            :title="t('common.delete')"
            @click="handleDeleteCanvas(selectedCanvasId!)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>
      </div>
      <A2UICanvas
        :canvas="selectedCanvas"
        @canvas-update="handleCanvasUpdate"
        @error="handleError"
      />
    </div>

    <!-- Canvas List -->
    <div v-else>
      <!-- Search and Filter Bar -->
      <div class="mb-6 flex items-center gap-4">
        <div class="flex-1 relative">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('a2ui.searchPlaceholder')"
            class="w-full pl-10 pr-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
          />
        </div>
        <div class="flex items-center gap-2">
          <button
            class="px-3 py-2 rounded-lg border transition-colors flex items-center gap-1"
            :class="sortBy === 'name' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-600' : 'border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700'"
            @click="toggleSort('name')"
          >
            <span class="text-sm">{{ t('a2ui.sortByName') }}</span>
            <svg v-if="sortBy === 'name'" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="sortOrder === 'asc' ? 'M5 15l7-7 7 7' : 'M19 9l-7 7-7-7'" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Empty State -->
      <div v-if="filteredCanvases.length === 0" class="text-center py-12">
        <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-gray-100 dark:bg-white/10 flex items-center justify-center">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
          </svg>
        </div>
        <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-2">
          {{ searchQuery ? t('a2ui.noSearchResults') : t('a2ui.noCanvases') }}
        </h3>
        <p class="text-gray-500 dark:text-slate-400 mb-4">
          {{ searchQuery ? t('a2ui.tryDifferentSearch') : t('a2ui.noCanvasesDesc') }}
        </p>
        <button
          v-if="!searchQuery"
          class="px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white transition-colors"
          @click="openCreateDialog"
        >
          {{ t('a2ui.createCanvas') }}
        </button>
      </div>

      <!-- Canvas Grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="canvasId in filteredCanvases"
          :key="canvasId"
          class="glass-card p-4 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
          @click="loadCanvas(canvasId)"
        >
          <div class="flex items-start justify-between">
            <div class="flex items-center space-x-3">
              <div class="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-500 to-indigo-500 flex items-center justify-center">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
                </svg>
              </div>
              <div>
                <h3 class="font-medium text-gray-900 dark:text-white group-hover:text-blue-500 transition-colors">
                  {{ canvasId }}
                </h3>
                <p class="text-sm text-gray-500 dark:text-slate-400">
                  {{ t('a2ui.canvas') }}
                </p>
              </div>
            </div>
            <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              <button
                class="p-2 rounded-lg text-gray-400 hover:text-blue-500 hover:bg-blue-500/10 transition-colors"
                :title="t('common.edit')"
                @click.stop="openEditDialog(canvasId)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button
                class="p-2 rounded-lg text-gray-400 hover:text-purple-500 hover:bg-purple-500/10 transition-colors"
                :title="t('a2ui.duplicate')"
                @click.stop="handleDuplicateCanvas(canvasId)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
              </button>
              <button
                class="p-2 rounded-lg text-gray-400 hover:text-green-500 hover:bg-green-500/10 transition-colors"
                :title="t('a2ui.export')"
                @click.stop="handleExportCanvas(canvasId)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>
              </button>
              <button
                class="p-2 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-500/10 transition-colors"
                :title="t('common.delete')"
                @click.stop="handleDeleteCanvas(canvasId)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading Canvas Overlay -->
    <div v-if="loadingCanvas" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="glass-card p-6 flex items-center space-x-3">
        <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600"></div>
        <span class="text-gray-900 dark:text-white">{{ t('common.loading') }}</span>
      </div>
    </div>

    <!-- Canvas Form Dialog -->
    <CanvasFormDialog
      :open="showFormDialog"
      :canvas="editingCanvas"
      :loading="saving"
      @save="handleSaveCanvas"
      @cancel="showFormDialog = false; editingCanvas = null"
    />
  </div>
</template>
