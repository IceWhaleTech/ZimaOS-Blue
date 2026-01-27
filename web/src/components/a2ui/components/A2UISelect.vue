<template>
  <div class="a2ui-select">
    <label v-if="label" :for="component.id" class="select-label">
      {{ label }}
      <span v-if="validation?.required" class="required">*</span>
    </label>
    <select
      :id="component.id"
      v-model="selectedValue"
      :disabled="disabled"
      class="select-field"
      @change="handleChange"
    >
      <option v-if="placeholder" value="" disabled>{{ placeholder }}</option>
      <option
        v-for="option in options"
        :key="option.value"
        :value="option.value"
        :disabled="option.disabled"
      >
        {{ option.label }}
      </option>
    </select>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Component } from '@/api/a2ui'

interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

const props = defineProps<{
  component: Component
  formData: Record<string, unknown>
}>()

const emit = defineEmits<{
  (e: 'update:form-data', payload: { key: string; value: unknown }): void
  (e: 'action', payload: { actionId: string; confirmRequired: boolean; confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string } }): void
}>()

const selectedValue = ref(props.formData[props.component.id] as string || props.component.props?.defaultValue as string || '')

const label = computed(() => props.component.props?.label as string)
const placeholder = computed(() => props.component.props?.placeholder as string)
const disabled = computed(() => props.component.props?.disabled as boolean || false)
const validation = computed(() => props.component.validation)
const options = computed(() => {
  const opts = props.component.props?.options
  if (Array.isArray(opts)) {
    return opts as SelectOption[]
  }
  return []
})

watch(() => props.formData[props.component.id], (newVal) => {
  if (newVal !== selectedValue.value) {
    selectedValue.value = newVal as string || ''
  }
})

function handleChange() {
  emit('update:form-data', { key: props.component.id, value: selectedValue.value })

  const changeAction = props.component.actions?.find(a => a.type === 'change')
  if (changeAction) {
    emit('action', {
      actionId: changeAction.id,
      confirmRequired: !!changeAction.confirm,
      confirmDialog: changeAction.confirm,
    })
  }
}
</script>

<style scoped>
.a2ui-select {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.select-label {
  font-weight: 500;
  font-size: 0.875rem;
  color: var(--color-text, #1f2937);
}

.required {
  color: #ef4444;
  margin-left: 0.125rem;
}

.select-field {
  padding: 0.625rem 0.875rem;
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 0.5rem;
  font-size: 1rem;
  background: var(--color-background, white);
  color: var(--color-text, #1f2937);
  cursor: pointer;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.select-field:focus {
  outline: none;
  border-color: var(--color-primary, #3b82f6);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.select-field:disabled {
  background: var(--color-background-soft, #f3f4f6);
  cursor: not-allowed;
}
</style>
