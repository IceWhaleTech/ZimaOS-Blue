<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { listCanvases, getCanvas, deleteCanvas, type Canvas } from '@/api/a2ui'
import A2UICanvas from '@/components/a2ui/A2UICanvas.vue'

const { t } = useI18n()

// State
const canvases = ref<string[]>([])
const selectedCanvasId = ref<string | null>(null)
const selectedCanvas = ref<Canvas | null>(null)
const loading = ref(true)
const loadingCanvas = ref(false)
const error = ref<string | null>(null)

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
      <button
        class="px-4 py-2 rounded-lg bg-accent text-white hover:bg-accent/90 transition-colors flex items-center space-x-2"
        @click="fetchCanvases"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        <span>{{ t('common.refresh') }}</span>
      </button>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="mb-6 p-4 rounded-lg bg-red-500/10 border border-red-500/30 text-red-500">
      <div class="flex items-center space-x-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span>{{ error }}</span>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-accent"></div>
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
        <button
          class="p-2 rounded-lg text-red-500 hover:bg-red-500/10 transition-colors"
          @click="handleDeleteCanvas(selectedCanvasId!)"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>
      </div>
      <A2UICanvas
        :canvas="selectedCanvas"
        @canvas-update="handleCanvasUpdate"
        @error="handleError"
      />
    </div>

    <!-- Canvas List -->
    <div v-else>
      <!-- Empty State -->
      <div v-if="canvases.length === 0" class="text-center py-12">
        <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-gray-100 dark:bg-white/10 flex items-center justify-center">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
          </svg>
        </div>
        <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-2">
          {{ t('a2ui.noCanvases') }}
        </h3>
        <p class="text-gray-500 dark:text-slate-400">
          {{ t('a2ui.noCanvasesDesc') }}
        </p>
      </div>

      <!-- Canvas Grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="canvasId in canvases"
          :key="canvasId"
          class="glass-card p-4 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
          @click="loadCanvas(canvasId)"
        >
          <div class="flex items-start justify-between">
            <div class="flex items-center space-x-3">
              <div class="w-10 h-10 rounded-lg bg-gradient-to-br from-rose-500 to-red-500 flex items-center justify-center">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
                </svg>
              </div>
              <div>
                <h3 class="font-medium text-gray-900 dark:text-white group-hover:text-rose-500 transition-colors">
                  {{ canvasId }}
                </h3>
                <p class="text-sm text-gray-500 dark:text-slate-400">
                  {{ t('a2ui.canvas') }}
                </p>
              </div>
            </div>
            <button
              class="p-2 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-500/10 transition-colors opacity-0 group-hover:opacity-100"
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

    <!-- Loading Canvas Overlay -->
    <div v-if="loadingCanvas" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="glass-card p-6 flex items-center space-x-3">
        <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-accent"></div>
        <span class="text-gray-900 dark:text-white">{{ t('common.loading') }}</span>
      </div>
    </div>
  </div>
</template>
