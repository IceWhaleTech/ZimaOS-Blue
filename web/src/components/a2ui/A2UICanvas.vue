<template>
  <div class="a2ui-canvas" :class="[`layout-${canvas.layout || 'vertical'}`]">
    <div v-if="canvas.title || canvas.description" class="canvas-header">
      <h2 v-if="canvas.title" class="canvas-title">{{ canvas.title }}</h2>
      <p v-if="canvas.description" class="canvas-description">{{ canvas.description }}</p>
    </div>
    <div class="canvas-content">
      <A2UIComponent
        v-for="component in canvas.components"
        :key="component.id"
        :component="component"
        :form-data="formData"
        @action="handleAction"
        @update:form-data="updateFormData"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Canvas, ActionResult } from '@/api/a2ui'
import { executeAction } from '@/api/a2ui'
import A2UIComponent from './A2UIComponent.vue'

const props = defineProps<{
  canvas: Canvas
}>()

const emit = defineEmits<{
  (e: 'action-result', result: ActionResult): void
  (e: 'canvas-update', canvas: Canvas): void
  (e: 'error', error: string): void
}>()

const formData = ref<Record<string, unknown>>({})

watch(
  () => props.canvas.id,
  () => {
    formData.value = {}
  }
)

function updateFormData(key: string, value: unknown) {
  formData.value[key] = value
}

async function handleAction(actionId: string, confirmRequired: boolean, confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string }) {
  if (confirmRequired && confirmDialog) {
    const confirmed = window.confirm(`${confirmDialog.title}\n\n${confirmDialog.message}`)
    if (!confirmed) return
  }

  try {
    const result = await executeAction(props.canvas.id, actionId, formData.value)
    emit('action-result', result)

    if (result.new_canvas) {
      emit('canvas-update', result.new_canvas)
    }

    if (!result.success && result.error) {
      emit('error', result.error)
    }
  } catch (error) {
    emit('error', error instanceof Error ? error.message : 'Action failed')
  }
}
</script>

<style scoped>
.a2ui-canvas {
  padding: 1rem;
}

.canvas-header {
  margin-bottom: 1.5rem;
}

.canvas-title {
  margin: 0 0 0.5rem 0;
  font-size: 1.5rem;
  font-weight: 600;
}

.canvas-description {
  margin: 0;
  color: var(--color-text-muted);
}

.canvas-content {
  display: flex;
  gap: 1rem;
}

.layout-vertical .canvas-content {
  flex-direction: column;
}

.layout-horizontal .canvas-content {
  flex-direction: row;
  flex-wrap: wrap;
}

.layout-grid .canvas-content {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
}
</style>
