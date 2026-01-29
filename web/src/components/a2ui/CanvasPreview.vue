<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Canvas, Component } from '@/api/a2ui'
import A2UICanvas from './A2UICanvas.vue'

const { t } = useI18n()

const props = defineProps<{
  canvas: Partial<Canvas>
  components: Component[]
}>()

// Responsive preview modes
type PreviewMode = 'desktop' | 'tablet' | 'mobile'
const previewMode = ref<PreviewMode>('desktop')

// Preview dimensions
const previewDimensions = computed(() => {
  switch (previewMode.value) {
    case 'mobile':
      return { width: '375px', height: '667px' }
    case 'tablet':
      return { width: '768px', height: '1024px' }
    default:
      return { width: '100%', height: '100%' }
  }
})

// Build a preview canvas from props
const previewCanvas = computed<Canvas>(() => ({
  id: props.canvas.id || 'preview',
  title: props.canvas.title,
  description: props.canvas.description,
  components: props.components || [],
  layout: props.canvas.layout || 'vertical',
  metadata: props.canvas.metadata,
  created_at: new Date().toISOString(),
}))

// Scale factor for preview
const scale = ref(1)
const previewContainer = ref<HTMLElement | null>(null)

// Auto-scale for mobile/tablet views
watch([previewMode, previewContainer], () => {
  if (!previewContainer.value) return

  if (previewMode.value === 'desktop') {
    scale.value = 1
    return
  }

  const containerWidth = previewContainer.value.clientWidth
  const targetWidth = previewMode.value === 'mobile' ? 375 : 768

  if (containerWidth < targetWidth) {
    scale.value = containerWidth / targetWidth
  } else {
    scale.value = 1
  }
}, { immediate: true })
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Preview Controls -->
    <div class="flex items-center justify-between mb-3">
      <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('a2ui.preview.title') }}
      </div>
      <div class="flex items-center gap-1 bg-gray-100 dark:bg-gray-700 rounded-lg p-1">
        <button
          v-for="mode in ['desktop', 'tablet', 'mobile'] as const"
          :key="mode"
          type="button"
          class="p-1.5 rounded transition-colors"
          :class="previewMode === mode
            ? 'bg-white dark:bg-gray-600 shadow-sm'
            : 'hover:bg-gray-200 dark:hover:bg-gray-600'"
          :title="t(`a2ui.preview.${mode}`)"
          @click="previewMode = mode"
        >
          <!-- Desktop Icon -->
          <svg v-if="mode === 'desktop'" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
          <!-- Tablet Icon -->
          <svg v-else-if="mode === 'tablet'" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 18h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
          </svg>
          <!-- Mobile Icon -->
          <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Preview Container -->
    <div
      ref="previewContainer"
      class="flex-1 overflow-auto bg-gray-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700"
    >
      <div
        class="mx-auto transition-all duration-200"
        :class="{
          'border-x border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 shadow-lg': previewMode !== 'desktop'
        }"
        :style="{
          width: previewDimensions.width,
          minHeight: previewMode === 'desktop' ? '100%' : previewDimensions.height,
          maxHeight: previewMode === 'desktop' ? 'none' : previewDimensions.height,
          transform: `scale(${scale})`,
          transformOrigin: 'top center',
        }"
      >
        <!-- Empty State -->
        <div
          v-if="!components || components.length === 0"
          class="h-full min-h-[200px] flex flex-col items-center justify-center text-gray-400 dark:text-gray-500"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
          </svg>
          <span class="text-sm">{{ t('a2ui.preview.empty') }}</span>
        </div>

        <!-- Canvas Preview -->
        <A2UICanvas
          v-else
          :canvas="previewCanvas"
          class="preview-canvas"
        />
      </div>
    </div>

    <!-- Preview Info -->
    <div class="mt-2 text-xs text-gray-400 dark:text-gray-500 text-center">
      {{ t('a2ui.preview.info', { count: components?.length || 0 }) }}
    </div>
  </div>
</template>

<style scoped>
.preview-canvas {
  pointer-events: none;
}

.preview-canvas :deep(button) {
  pointer-events: none;
}

.preview-canvas :deep(input),
.preview-canvas :deep(select),
.preview-canvas :deep(textarea) {
  pointer-events: none;
}
</style>
