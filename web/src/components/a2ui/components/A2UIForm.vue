<template>
  <form class="a2ui-form" @submit.prevent="handleSubmit">
    <A2UIComponent
      v-for="child in component.children"
      :key="child.id"
      :component="child"
      :form-data="formData"
      @action="$emit('action', $event)"
      @update:form-data="$emit('update:form-data', $event)"
    />
    <div class="form-actions">
      <button type="submit" class="submit-btn" :disabled="submitting">
        {{ submitLabel }}
      </button>
      <button v-if="showReset" type="reset" class="reset-btn" @click="handleReset">
        {{ resetLabel }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Component } from '@/api/a2ui'
import A2UIComponent from '../A2UIComponent.vue'

const props = defineProps<{
  component: Component
  formData: Record<string, unknown>
}>()

const emit = defineEmits<{
  (e: 'action', payload: { actionId: string; confirmRequired: boolean; confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string } }): void
  (e: 'update:form-data', payload: { key: string; value: unknown }): void
}>()

const submitting = ref(false)

const submitLabel = computed(() => props.component.props?.submitLabel as string || 'Submit')
const resetLabel = computed(() => props.component.props?.resetLabel as string || 'Reset')
const showReset = computed(() => props.component.props?.showReset as boolean || false)

function handleSubmit() {
  const submitAction = props.component.actions?.find(a => a.type === 'submit')
  if (submitAction) {
    emit('action', {
      actionId: submitAction.id,
      confirmRequired: !!submitAction.confirm,
      confirmDialog: submitAction.confirm,
    })
  }
}

function handleReset() {
  // Reset form data for all children
  props.component.children?.forEach(child => {
    emit('update:form-data', { key: child.id, value: undefined })
  })
}
</script>

<style scoped>
.a2ui-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 0.5rem;
}

.submit-btn {
  padding: 0.625rem 1.25rem;
  border: none;
  border-radius: 0.5rem;
  background: var(--color-primary, #3b82f6);
  color: white;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}

.submit-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.submit-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.reset-btn {
  padding: 0.625rem 1.25rem;
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 0.5rem;
  background: var(--color-background, white);
  color: var(--color-text, #1f2937);
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.reset-btn:hover {
  background: var(--color-background-soft, #f3f4f6);
}
</style>
