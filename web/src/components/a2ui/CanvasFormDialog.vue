<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Canvas, Component, ComponentType } from '@/api/a2ui'
import { getComponentLabel } from '@/api/a2ui'
import ComponentPalette from './ComponentPalette.vue'
import ComponentEditor from './ComponentEditor.vue'
import CanvasPreview from './CanvasPreview.vue'

const { t } = useI18n()

const props = defineProps<{
  canvas?: Canvas | null
  loading?: boolean
  open: boolean
}>()

const emit = defineEmits<{
  save: [data: Omit<Canvas, 'created_at'>]
  cancel: []
}>()

// Form state
const id = ref('')
const title = ref('')
const description = ref('')
const layout = ref<'vertical' | 'horizontal' | 'grid'>('vertical')
const components = ref<Component[]>([])
const metadataJson = ref('{}')
const ttlHours = ref(0)

// UI state
const activeTab = ref<'basic' | 'components' | 'preview' | 'advanced'>('basic')
const metadataError = ref('')
const selectedComponentIndex = ref<number | null>(null)

// Computed
const isEditMode = computed(() => !!props.canvas)

const isValid = computed(() => {
  return id.value.trim() !== '' && !metadataError.value
})

// Watch for canvas changes (edit mode)
watch(
  () => props.canvas,
  (newCanvas) => {
    if (newCanvas) {
      id.value = newCanvas.id
      title.value = newCanvas.title || ''
      description.value = newCanvas.description || ''
      layout.value = (newCanvas.layout as 'vertical' | 'horizontal' | 'grid') || 'vertical'
      components.value = JSON.parse(JSON.stringify(newCanvas.components || []))
      metadataJson.value = JSON.stringify(newCanvas.metadata || {}, null, 2)
      ttlHours.value = 0
    } else {
      resetForm()
    }
  },
  { immediate: true }
)

// Watch for dialog open
watch(
  () => props.open,
  (isOpen) => {
    if (isOpen && !props.canvas) {
      resetForm()
    }
  }
)

// Validate metadata JSON
watch(metadataJson, (value) => {
  try {
    JSON.parse(value)
    metadataError.value = ''
  } catch {
    metadataError.value = t('a2ui.form.invalidJson')
  }
})

function resetForm(): void {
  id.value = ''
  title.value = ''
  description.value = ''
  layout.value = 'vertical'
  components.value = []
  metadataJson.value = '{}'
  ttlHours.value = 0
  activeTab.value = 'basic'
  selectedComponentIndex.value = null
  metadataError.value = ''
}

function generateId(): void {
  id.value = `canvas-${Date.now()}-${Math.random().toString(36).substring(2, 8)}`
}

function addComponent(type: ComponentType): void {
  const newComponent: Component = {
    id: `${type}-${Date.now()}`,
    type,
    props: getDefaultProps(type),
  }
  components.value.push(newComponent)
  selectedComponentIndex.value = components.value.length - 1
}

function getDefaultProps(type: ComponentType): Record<string, unknown> {
  switch (type) {
    case 'text':
      return { content: 'Text content' }
    case 'button':
      return { label: 'Button' }
    case 'input':
      return { label: 'Input', placeholder: 'Enter value...' }
    case 'select':
      return { label: 'Select', options: [] }
    case 'checkbox':
      return { label: 'Checkbox' }
    case 'slider':
      return { min: 0, max: 100, value: 50 }
    case 'image':
      return { src: '', alt: 'Image' }
    case 'card':
      return { title: 'Card Title' }
    case 'list':
      return { items: [] }
    case 'table':
      return { columns: [], rows: [] }
    case 'progress':
      return { value: 0, max: 100 }
    case 'alert':
      return { type: 'info', message: 'Alert message' }
    case 'code':
      return { language: 'javascript', code: '' }
    case 'markdown':
      return { content: '# Markdown' }
    default:
      return {}
  }
}

function removeComponent(index: number): void {
  components.value.splice(index, 1)
  if (selectedComponentIndex.value === index) {
    selectedComponentIndex.value = null
  } else if (selectedComponentIndex.value !== null && selectedComponentIndex.value > index) {
    selectedComponentIndex.value--
  }
}

function moveComponent(index: number, direction: 'up' | 'down'): void {
  const newIndex = direction === 'up' ? index - 1 : index + 1
  if (newIndex < 0 || newIndex >= components.value.length) return

  const currentItem = components.value[index]
  const swapItem = components.value[newIndex]
  if (currentItem && swapItem) {
    components.value[index] = swapItem
    components.value[newIndex] = currentItem
  }

  if (selectedComponentIndex.value === index) {
    selectedComponentIndex.value = newIndex
  } else if (selectedComponentIndex.value === newIndex) {
    selectedComponentIndex.value = index
  }
}

function updateComponent(index: number, updated: Component): void {
  components.value[index] = updated
}

function handleSubmit(): void {
  if (!isValid.value) return

  let metadata: Record<string, unknown> = {}
  try {
    metadata = JSON.parse(metadataJson.value)
  } catch {
    return
  }

  const data: Omit<Canvas, 'created_at'> = {
    id: id.value.trim(),
    title: title.value.trim() || undefined,
    description: description.value.trim() || undefined,
    components: components.value,
    layout: layout.value,
    metadata: Object.keys(metadata).length > 0 ? metadata : undefined,
  }

  // Add expiration if set
  if (ttlHours.value > 0) {
    const expiresAt = new Date()
    expiresAt.setHours(expiresAt.getHours() + ttlHours.value)
    data.expires_at = expiresAt.toISOString()
  }

  emit('save', data)
}
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center">
    <!-- Backdrop -->
    <div class="absolute inset-0 bg-black/50" @click="emit('cancel')" />

    <!-- Dialog -->
    <div class="relative bg-white dark:bg-gray-800 rounded-xl shadow-xl w-full max-w-4xl max-h-[90vh] overflow-hidden flex flex-col">
      <!-- Header -->
      <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
          {{ isEditMode ? t('a2ui.form.editCanvas') : t('a2ui.form.createCanvas') }}
        </h2>
        <button
          class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          @click="emit('cancel')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Tabs -->
      <div class="px-6 pt-4 border-b border-gray-200 dark:border-gray-700">
        <div class="flex space-x-4">
          <button
            v-for="tab in ['basic', 'components', 'preview', 'advanced'] as const"
            :key="tab"
            class="px-4 py-2 text-sm font-medium border-b-2 transition-colors"
            :class="activeTab === tab
              ? 'border-blue-500 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'"
            @click="activeTab = tab"
          >
            {{ t(`a2ui.form.tabs.${tab}`) }}
          </button>
        </div>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto p-6">
        <!-- Basic Tab -->
        <div v-show="activeTab === 'basic'" class="space-y-4">
          <!-- ID -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {{ t('a2ui.form.canvasId') }} <span class="text-red-500">*</span>
            </label>
            <div class="flex gap-2">
              <input
                v-model="id"
                type="text"
                :placeholder="t('a2ui.form.canvasIdPlaceholder')"
                :disabled="isEditMode"
                class="flex-1 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600 disabled:opacity-50"
                required
              />
              <button
                v-if="!isEditMode"
                type="button"
                class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg transition-colors"
                @click="generateId"
              >
                {{ t('a2ui.form.generate') }}
              </button>
            </div>
          </div>

          <!-- Title -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {{ t('a2ui.form.title') }}
            </label>
            <input
              v-model="title"
              type="text"
              :placeholder="t('a2ui.form.titlePlaceholder')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
            />
          </div>

          <!-- Description -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {{ t('a2ui.form.description') }}
            </label>
            <textarea
              v-model="description"
              rows="3"
              :placeholder="t('a2ui.form.descriptionPlaceholder')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600 resize-none"
            />
          </div>

          <!-- Layout -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {{ t('a2ui.form.layout') }}
            </label>
            <div class="flex gap-3">
              <label
                v-for="layoutOption in ['vertical', 'horizontal', 'grid'] as const"
                :key="layoutOption"
                class="flex items-center gap-2 px-4 py-2 rounded-lg border cursor-pointer transition-colors"
                :class="layout === layoutOption
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400'
                  : 'border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700'"
              >
                <input
                  v-model="layout"
                  type="radio"
                  :value="layoutOption"
                  class="sr-only"
                />
                <span class="text-sm">{{ t(`a2ui.form.layouts.${layoutOption}`) }}</span>
              </label>
            </div>
          </div>
        </div>

        <!-- Components Tab -->
        <div v-show="activeTab === 'components'" class="flex gap-4 h-[400px]">
          <!-- Component Palette -->
          <div class="w-48 flex-shrink-0">
            <ComponentPalette @add="addComponent" />
          </div>

          <!-- Component List -->
          <div class="flex-1 flex flex-col">
            <div class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('a2ui.form.componentList') }} ({{ components.length }})
            </div>
            <div v-if="components.length === 0" class="flex-1 flex items-center justify-center text-gray-400 dark:text-gray-500 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg">
              {{ t('a2ui.form.noComponents') }}
            </div>
            <div v-else class="flex-1 overflow-y-auto space-y-2">
              <div
                v-for="(component, index) in components"
                :key="component.id"
                class="flex items-center gap-2 p-3 rounded-lg border cursor-pointer transition-colors"
                :class="selectedComponentIndex === index
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                  : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'"
                @click="selectedComponentIndex = index"
              >
                <div class="w-8 h-8 rounded bg-gray-200 dark:bg-gray-600 flex items-center justify-center text-xs font-mono">
                  {{ component.type.substring(0, 2).toUpperCase() }}
                </div>
                <div class="flex-1 min-w-0">
                  <div class="text-sm font-medium text-gray-900 dark:text-white truncate">
                    {{ getComponentLabel(component.type) }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400 truncate">
                    {{ component.id }}
                  </div>
                </div>
                <div class="flex items-center gap-1">
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-30"
                    :disabled="index === 0"
                    @click.stop="moveComponent(index, 'up')"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-30"
                    :disabled="index === components.length - 1"
                    @click.stop="moveComponent(index, 'down')"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/30 text-red-500"
                    @click.stop="removeComponent(index)"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Component Editor -->
          <div class="w-72 flex-shrink-0 border-l border-gray-200 dark:border-gray-700 pl-4">
            <ComponentEditor
              v-if="selectedComponentIndex !== null && components[selectedComponentIndex]"
              :component="components[selectedComponentIndex]!"
              @update="(c) => updateComponent(selectedComponentIndex!, c)"
            />
            <div v-else class="h-full flex items-center justify-center text-gray-400 dark:text-gray-500 text-sm">
              {{ t('a2ui.form.selectComponent') }}
            </div>
          </div>
        </div>

        <!-- Preview Tab -->
        <div v-show="activeTab === 'preview'" class="h-[400px]">
          <CanvasPreview
            :canvas="{ id, title, description, layout }"
            :components="components"
          />
        </div>

        <!-- Advanced Tab -->
        <div v-show="activeTab === 'advanced'" class="space-y-4">
          <!-- TTL -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {{ t('a2ui.form.expiration') }}
            </label>
            <div class="flex items-center gap-2">
              <input
                v-model.number="ttlHours"
                type="number"
                min="0"
                class="w-24 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              />
              <span class="text-sm text-gray-500 dark:text-gray-400">{{ t('a2ui.form.hours') }}</span>
              <span class="text-xs text-gray-400 dark:text-gray-500">({{ t('a2ui.form.zeroForNoExpiration') }})</span>
            </div>
          </div>

          <!-- Metadata -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {{ t('a2ui.form.metadata') }}
            </label>
            <textarea
              v-model="metadataJson"
              rows="8"
              :placeholder="t('a2ui.form.metadataPlaceholder')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 border border-gray-300 dark:border-gray-600 font-mono text-sm resize-none"
              :class="metadataError ? 'ring-2 ring-red-500' : 'focus:ring-blue-500'"
            />
            <p v-if="metadataError" class="mt-1 text-sm text-red-500 dark:text-red-400">
              {{ metadataError }}
            </p>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
        <button
          type="button"
          class="px-4 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg transition-colors"
          :disabled="loading"
          @click="emit('cancel')"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="loading || !isValid"
          @click="handleSubmit"
        >
          {{ loading ? t('common.saving') : isEditMode ? t('common.save') : t('common.create') }}
        </button>
      </div>
    </div>
  </div>
</template>
