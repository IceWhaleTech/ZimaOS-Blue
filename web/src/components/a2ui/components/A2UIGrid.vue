<template>
  <div class="a2ui-grid" :style="gridStyle">
    <A2UIComponent
      v-for="child in component.children"
      :key="child.id"
      :component="child"
      :form-data="formData"
      @action="$emit('action', $event)"
      @update:form-data="$emit('update:form-data', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from '@/api/a2ui'
import A2UIComponent from '../A2UIComponent.vue'

const props = defineProps<{
  component: Component
  formData: Record<string, unknown>
}>()

defineEmits<{
  (e: 'action', payload: { actionId: string; confirmRequired: boolean; confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string } }): void
  (e: 'update:form-data', payload: { key: string; value: unknown }): void
}>()

const columns = computed(() => props.component.props?.columns as number || 2)
const gap = computed(() => props.component.props?.gap as string || '1rem')

const gridStyle = computed(() => ({
  display: 'grid',
  gridTemplateColumns: `repeat(${columns.value}, 1fr)`,
  gap: gap.value,
}))
</script>

<style scoped>
.a2ui-grid {
  width: 100%;
}

@media (max-width: 640px) {
  .a2ui-grid {
    grid-template-columns: 1fr !important;
  }
}
</style>
