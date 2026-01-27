<template>
  <div class="a2ui-checkbox">
    <label class="checkbox-wrapper">
      <input
        :id="component.id"
        v-model="checked"
        type="checkbox"
        :disabled="disabled"
        class="checkbox-input"
        @change="handleChange"
      />
      <span class="checkbox-custom"></span>
      <span v-if="label" class="checkbox-label">{{ label }}</span>
    </label>
    <p v-if="hint" class="hint">{{ hint }}</p>
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
  (e: 'action', payload: { actionId: string; confirmRequired: boolean; confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string } }): void
}>()

const checked = ref(props.formData[props.component.id] as boolean || props.component.props?.defaultValue as boolean || false)

const label = computed(() => props.component.props?.label as string)
const disabled = computed(() => props.component.props?.disabled as boolean || false)
const hint = computed(() => props.component.props?.hint as string)

watch(() => props.formData[props.component.id], (newVal) => {
  if (newVal !== checked.value) {
    checked.value = newVal as boolean || false
  }
})

function handleChange() {
  emit('update:form-data', { key: props.component.id, value: checked.value })

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
.a2ui-checkbox {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.checkbox-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.checkbox-input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}

.checkbox-custom {
  width: 1.25rem;
  height: 1.25rem;
  border: 2px solid var(--color-border, #e5e7eb);
  border-radius: 0.25rem;
  background: var(--color-background, white);
  transition: all 0.2s;
  position: relative;
}

.checkbox-input:checked + .checkbox-custom {
  background: var(--color-primary, #3b82f6);
  border-color: var(--color-primary, #3b82f6);
}

.checkbox-input:checked + .checkbox-custom::after {
  content: '';
  position: absolute;
  left: 0.35rem;
  top: 0.15rem;
  width: 0.35rem;
  height: 0.6rem;
  border: solid white;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

.checkbox-input:focus + .checkbox-custom {
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.checkbox-input:disabled + .checkbox-custom {
  background: var(--color-background-soft, #f3f4f6);
  cursor: not-allowed;
}

.checkbox-label {
  font-size: 0.875rem;
  color: var(--color-text, #1f2937);
}

.hint {
  margin: 0;
  font-size: 0.75rem;
  color: var(--color-text-muted, #6b7280);
  margin-left: 1.75rem;
}
</style>
