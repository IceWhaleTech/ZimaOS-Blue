<template>
  <div class="a2ui-input">
    <label v-if="label" :for="component.id" class="input-label">
      {{ label }}
      <span v-if="validation?.required" class="required">*</span>
    </label>
    <input
      :id="component.id"
      v-model="inputValue"
      :type="inputType"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :minlength="validation?.min_length"
      :maxlength="validation?.max_length"
      :min="validation?.min"
      :max="validation?.max"
      :pattern="validation?.pattern"
      class="input-field"
      :class="{ error: hasError }"
      @input="handleInput"
      @blur="validate"
    />
    <p v-if="hasError" class="error-message">{{ errorMessage }}</p>
    <p v-if="hint && !hasError" class="hint">{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
  formData: Record<string, unknown>
}>()

const emit = defineEmits<{
  (e: 'update:form-data', payload: { key: string; value: unknown }): void
}>()

const inputValue = ref(props.formData[props.component.id] as string || props.component.props?.defaultValue as string || '')
const hasError = ref(false)
const errorMessage = ref('')

const label = computed(() => props.component.props?.label as string)
const placeholder = computed(() => props.component.props?.placeholder as string || '')
const inputType = computed(() => props.component.props?.type as string || 'text')
const disabled = computed(() => props.component.props?.disabled as boolean || false)
const readonly = computed(() => props.component.props?.readonly as boolean || false)
const hint = computed(() => props.component.props?.hint as string)
const validation = computed(() => props.component.validation)

watch(() => props.formData[props.component.id], (newVal) => {
  if (newVal !== inputValue.value) {
    inputValue.value = newVal as string || ''
  }
})

function handleInput() {
  emit('update:form-data', { key: props.component.id, value: inputValue.value })
  if (hasError.value) {
    validate()
  }
}

function validate() {
  hasError.value = false
  errorMessage.value = ''

  if (!validation.value) return

  const val = inputValue.value

  if (validation.value.required && !val) {
    hasError.value = true
    errorMessage.value = validation.value.message || 'This field is required'
    return
  }

  if (validation.value.min_length && val.length < validation.value.min_length) {
    hasError.value = true
    errorMessage.value = validation.value.message || `Minimum ${validation.value.min_length} characters required`
    return
  }

  if (validation.value.max_length && val.length > validation.value.max_length) {
    hasError.value = true
    errorMessage.value = validation.value.message || `Maximum ${validation.value.max_length} characters allowed`
    return
  }

  if (validation.value.pattern) {
    const regex = new RegExp(validation.value.pattern)
    if (!regex.test(val)) {
      hasError.value = true
      errorMessage.value = validation.value.message || 'Invalid format'
    }
  }
}
</script>

<style scoped>
.a2ui-input {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.input-label {
  font-weight: 500;
  font-size: 0.875rem;
  color: var(--color-text, #1f2937);
}

.required {
  color: #ef4444;
  margin-left: 0.125rem;
}

.input-field {
  padding: 0.625rem 0.875rem;
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 0.5rem;
  font-size: 1rem;
  background: var(--color-background, white);
  color: var(--color-text, #1f2937);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.input-field:focus {
  outline: none;
  border-color: var(--color-primary, #3b82f6);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.input-field:disabled {
  background: var(--color-background-soft, #f3f4f6);
  cursor: not-allowed;
}

.input-field.error {
  border-color: #ef4444;
}

.input-field.error:focus {
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.1);
}

.error-message {
  margin: 0;
  font-size: 0.75rem;
  color: #ef4444;
}

.hint {
  margin: 0;
  font-size: 0.75rem;
  color: var(--color-text-muted, #6b7280);
}
</style>
