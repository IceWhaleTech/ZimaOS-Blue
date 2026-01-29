<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ComponentType } from '@/api/a2ui'
import { getComponentLabel } from '@/api/a2ui'

const { t } = useI18n()

const emit = defineEmits<{
  add: [type: ComponentType]
}>()

// Hover preview state
const hoveredComponent = ref<ComponentType | null>(null)
const hoverPosition = ref({ x: 0, y: 0 })

// Component categories
const componentCategories = [
  {
    name: 'basic',
    label: 'a2ui.palette.basic',
    components: ['text', 'button', 'image'] as ComponentType[],
  },
  {
    name: 'form',
    label: 'a2ui.palette.form',
    components: ['input', 'select', 'checkbox', 'slider'] as ComponentType[],
  },
  {
    name: 'display',
    label: 'a2ui.palette.display',
    components: ['card', 'list', 'table', 'progress', 'alert'] as ComponentType[],
  },
  {
    name: 'content',
    label: 'a2ui.palette.content',
    components: ['code', 'markdown'] as ComponentType[],
  },
  {
    name: 'layout',
    label: 'a2ui.palette.layout',
    components: ['container', 'grid', 'tabs', 'accordion'] as ComponentType[],
  },
]

// Component icons (simple text representations)
function getComponentIcon(type: ComponentType): string {
  const icons: Record<ComponentType, string> = {
    text: 'T',
    button: 'B',
    input: 'I',
    select: 'S',
    checkbox: 'C',
    radio: 'R',
    slider: '—',
    image: '🖼',
    card: '▢',
    list: '≡',
    table: '⊞',
    chart: '📊',
    form: 'F',
    progress: '▰',
    alert: '!',
    code: '</>',
    markdown: 'M↓',
    container: '[ ]',
    grid: '⊞',
    tabs: '⊟',
    accordion: '▼',
  }
  return icons[type] || '?'
}

// Component descriptions for preview
function getComponentDescription(type: ComponentType): string {
  const descriptions: Record<ComponentType, string> = {
    text: 'a2ui.palette.desc.text',
    button: 'a2ui.palette.desc.button',
    input: 'a2ui.palette.desc.input',
    select: 'a2ui.palette.desc.select',
    checkbox: 'a2ui.palette.desc.checkbox',
    radio: 'a2ui.palette.desc.radio',
    slider: 'a2ui.palette.desc.slider',
    image: 'a2ui.palette.desc.image',
    card: 'a2ui.palette.desc.card',
    list: 'a2ui.palette.desc.list',
    table: 'a2ui.palette.desc.table',
    chart: 'a2ui.palette.desc.chart',
    form: 'a2ui.palette.desc.form',
    progress: 'a2ui.palette.desc.progress',
    alert: 'a2ui.palette.desc.alert',
    code: 'a2ui.palette.desc.code',
    markdown: 'a2ui.palette.desc.markdown',
    container: 'a2ui.palette.desc.container',
    grid: 'a2ui.palette.desc.grid',
    tabs: 'a2ui.palette.desc.tabs',
    accordion: 'a2ui.palette.desc.accordion',
  }
  return descriptions[type] || ''
}

// Handle hover events
function handleMouseEnter(type: ComponentType, event: MouseEvent): void {
  hoveredComponent.value = type
  const rect = (event.target as HTMLElement).getBoundingClientRect()
  hoverPosition.value = {
    x: rect.right + 8,
    y: rect.top,
  }
}

function handleMouseLeave(): void {
  hoveredComponent.value = null
}</script>

<template>
  <div class="h-full flex flex-col relative">
    <div class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
      {{ t('a2ui.palette.title') }}
    </div>

    <div class="flex-1 overflow-y-auto space-y-3">
      <div v-for="category in componentCategories" :key="category.name">
        <div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5 uppercase tracking-wide">
          {{ t(category.label) }}
        </div>
        <div class="grid grid-cols-2 gap-1.5">
          <button
            v-for="type in category.components"
            :key="type"
            type="button"
            class="flex flex-col items-center gap-1 p-2 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 hover:border-blue-300 dark:hover:border-blue-600 transition-colors group"
            @click="emit('add', type)"
            @mouseenter="handleMouseEnter(type, $event)"
            @mouseleave="handleMouseLeave"
          >
            <div class="w-8 h-8 rounded bg-gray-100 dark:bg-gray-700 group-hover:bg-blue-100 dark:group-hover:bg-blue-900/30 flex items-center justify-center text-xs font-mono text-gray-600 dark:text-gray-400 group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors">
              {{ getComponentIcon(type) }}
            </div>
            <span class="text-xs text-gray-600 dark:text-gray-400 group-hover:text-gray-900 dark:group-hover:text-white truncate w-full text-center transition-colors">
              {{ getComponentLabel(type) }}
            </span>
          </button>
        </div>
      </div>
    </div>

    <!-- Hover Preview Tooltip -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="hoveredComponent"
          class="fixed z-50 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 p-3 w-56 pointer-events-none"
          :style="{ left: `${hoverPosition.x}px`, top: `${hoverPosition.y}px` }"
        >
          <!-- Preview Header -->
          <div class="flex items-center gap-2 mb-2">
            <div class="w-8 h-8 rounded bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-sm font-mono text-blue-600 dark:text-blue-400">
              {{ getComponentIcon(hoveredComponent) }}
            </div>
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ getComponentLabel(hoveredComponent) }}
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ hoveredComponent }}
              </div>
            </div>
          </div>
          <!-- Preview Description -->
          <p class="text-xs text-gray-600 dark:text-gray-300 mb-2">
            {{ t(getComponentDescription(hoveredComponent)) }}
          </p>
          <!-- Mini Preview -->
          <div class="bg-gray-50 dark:bg-gray-900 rounded p-2 border border-gray-200 dark:border-gray-700">
            <!-- Text Preview -->
            <div v-if="hoveredComponent === 'text'" class="text-sm text-gray-700 dark:text-gray-300">
              Sample text content
            </div>
            <!-- Button Preview -->
            <div v-else-if="hoveredComponent === 'button'" class="flex justify-center">
              <span class="px-3 py-1 bg-blue-600 text-white text-xs rounded">Button</span>
            </div>
            <!-- Input Preview -->
            <div v-else-if="hoveredComponent === 'input'">
              <div class="text-xs text-gray-500 mb-1">Label</div>
              <div class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded px-2 py-1 text-xs text-gray-400">
                Placeholder...
              </div>
            </div>
            <!-- Select Preview -->
            <div v-else-if="hoveredComponent === 'select'">
              <div class="text-xs text-gray-500 mb-1">Label</div>
              <div class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded px-2 py-1 text-xs text-gray-400 flex justify-between items-center">
                <span>Select...</span>
                <span>▼</span>
              </div>
            </div>
            <!-- Checkbox Preview -->
            <div v-else-if="hoveredComponent === 'checkbox'" class="flex items-center gap-2">
              <div class="w-4 h-4 border-2 border-blue-500 rounded bg-blue-500 flex items-center justify-center">
                <span class="text-white text-xs">✓</span>
              </div>
              <span class="text-xs text-gray-700 dark:text-gray-300">Checkbox label</span>
            </div>
            <!-- Slider Preview -->
            <div v-else-if="hoveredComponent === 'slider'">
              <div class="h-2 bg-gray-300 dark:bg-gray-600 rounded-full relative">
                <div class="absolute left-0 top-0 h-2 w-1/2 bg-blue-500 rounded-full"></div>
                <div class="absolute top-1/2 left-1/2 -translate-y-1/2 w-3 h-3 bg-white border-2 border-blue-500 rounded-full"></div>
              </div>
            </div>
            <!-- Image Preview -->
            <div v-else-if="hoveredComponent === 'image'" class="flex justify-center">
              <div class="w-16 h-12 bg-gray-200 dark:bg-gray-600 rounded flex items-center justify-center text-gray-400">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
              </div>
            </div>
            <!-- Card Preview -->
            <div v-else-if="hoveredComponent === 'card'" class="bg-white dark:bg-gray-700 rounded border border-gray-200 dark:border-gray-600 p-2">
              <div class="text-xs font-medium text-gray-900 dark:text-white">Card Title</div>
              <div class="text-xs text-gray-500">Subtitle</div>
            </div>
            <!-- Progress Preview -->
            <div v-else-if="hoveredComponent === 'progress'">
              <div class="h-2 bg-gray-200 dark:bg-gray-600 rounded-full overflow-hidden">
                <div class="h-full w-2/3 bg-blue-500 rounded-full"></div>
              </div>
              <div class="text-xs text-gray-500 text-center mt-1">67%</div>
            </div>
            <!-- Alert Preview -->
            <div v-else-if="hoveredComponent === 'alert'" class="bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-800 rounded p-2">
              <div class="text-xs text-blue-700 dark:text-blue-300">Info message</div>
            </div>
            <!-- Code Preview -->
            <div v-else-if="hoveredComponent === 'code'" class="bg-gray-900 rounded p-2 font-mono text-xs text-green-400">
              const x = 1;
            </div>
            <!-- Markdown Preview -->
            <div v-else-if="hoveredComponent === 'markdown'">
              <div class="text-sm font-bold text-gray-900 dark:text-white"># Heading</div>
              <div class="text-xs text-gray-600 dark:text-gray-300">Paragraph text</div>
            </div>
            <!-- Default Preview -->
            <div v-else class="text-xs text-gray-500 text-center py-2">
              {{ getComponentLabel(hoveredComponent) }}
            </div>
          </div>
          <!-- Click hint -->
          <div class="mt-2 text-xs text-gray-400 dark:text-gray-500 text-center">
            {{ t('a2ui.palette.clickToAdd') }}
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
