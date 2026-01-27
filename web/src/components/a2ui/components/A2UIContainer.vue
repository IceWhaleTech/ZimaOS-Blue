<template>
  <div class="a2ui-container" :style="componentStyle">
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

const componentStyle = computed(() => {
  const style = props.component.style || {}
  return {
    display: 'flex' as const,
    flexDirection: (style.direction as 'row' | 'column') || 'column',
    gap: (style.gap as string) || '0.75rem',
    padding: style.padding as string | undefined,
    backgroundColor: style.backgroundColor as string | undefined,
    borderRadius: style.borderRadius as string | undefined,
  }
})
</script>

<style scoped>
.a2ui-container {
  width: 100%;
}
</style>
