<template>
  <div class="a2ui-card" :style="componentStyle">
    <div v-if="title || subtitle" class="card-header">
      <h3 v-if="title" class="card-title">{{ title }}</h3>
      <p v-if="subtitle" class="card-subtitle">{{ subtitle }}</p>
    </div>
    <div class="card-content">
      <A2UIComponent
        v-for="child in component.children"
        :key="child.id"
        :component="child"
        :form-data="formData"
        @action="$emit('action', $event)"
        @update:form-data="$emit('update:form-data', $event)"
      />
    </div>
    <div v-if="hasActions" class="card-actions">
      <button
        v-for="action in component.actions"
        :key="action.id"
        class="card-action-btn"
        @click="handleAction(action)"
      >
        {{ action.label || 'Action' }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component, Action } from '@/api/a2ui'
import A2UIComponent from '../A2UIComponent.vue'

const props = defineProps<{
  component: Component
  formData: Record<string, unknown>
}>()

const emit = defineEmits<{
  (e: 'action', payload: { actionId: string; confirmRequired: boolean; confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string } }): void
  (e: 'update:form-data', payload: { key: string; value: unknown }): void
}>()

const title = computed(() => props.component.props?.title as string)
const subtitle = computed(() => props.component.props?.subtitle as string)
const hasActions = computed(() => props.component.actions && props.component.actions.length > 0)

const componentStyle = computed(() => {
  const style = props.component.style || {}
  return {
    backgroundColor: style.backgroundColor,
    borderRadius: style.borderRadius || '0.75rem',
    padding: style.padding || '1.25rem',
  }
})

function handleAction(action: Action) {
  emit('action', {
    actionId: action.id,
    confirmRequired: !!action.confirm,
    confirmDialog: action.confirm,
  })
}
</script>

<style scoped>
.a2ui-card {
  background: var(--color-background-soft, #f9fafb);
  border: 1px solid var(--color-border, #e5e7eb);
}

.card-header {
  margin-bottom: 1rem;
}

.card-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--color-text, #1f2937);
}

.card-subtitle {
  margin: 0.25rem 0 0 0;
  font-size: 0.875rem;
  color: var(--color-text-muted, #6b7280);
}

.card-content {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.card-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--color-border, #e5e7eb);
}

.card-action-btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 0.375rem;
  background: var(--color-primary, #3b82f6);
  color: white;
  font-size: 0.875rem;
  cursor: pointer;
  transition: opacity 0.2s;
}

.card-action-btn:hover {
  opacity: 0.9;
}
</style>
