<template>
  <div class="a2ui-slider">
    <div class="slider-header">
      <label v-if="label" :for="component.id" class="slider-label">{{ label }}</label>
      <span class="slider-value">{{ sliderValue }}</span>
    </div>
    <input
      :id="component.id"
      v-model.number="sliderValue"
      type="range"
      :min="min"
      :max="max"
      :step="step"
      :disabled="disabled"
      class="slider-input"
      @input="handleInput"
    />
    <div class="slider-range">
      <span>{{ min }}</span>
      <span>{{ max }}</span>
    </div>
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

const defaultValue = computed(() => {
  const def = props.component.props?.defaultValue
  return typeof def === 'number' ? def : 50
})

const sliderValue = ref(props.formData[props.component.id] as number || defaultValue.value)

const label = computed(() => props.component.props?.label as string)
const min = computed(() => props.component.props?.min as number || 0)
const max = computed(() => props.component.props?.max as number || 100)
const step = computed(() => props.component.props?.step as number || 1)
const disabled = computed(() => props.component.props?.disabled as boolean || false)

watch(() => props.formData[props.component.id], (newVal) => {
  if (newVal !== sliderValue.value) {
    sliderValue.value = newVal as number || defaultValue.value
  }
})

function handleInput() {
  emit('update:form-data', { key: props.component.id, value: sliderValue.value })
}
</script>

<style scoped>
.a2ui-slider {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.slider-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.slider-label {
  font-weight: 500;
  font-size: 0.875rem;
  color: var(--color-text, #1f2937);
}

.slider-value {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--color-primary, #3b82f6);
}

.slider-input {
  width: 100%;
  height: 0.5rem;
  border-radius: 0.25rem;
  background: var(--color-background-soft, #e5e7eb);
  appearance: none;
  cursor: pointer;
}

.slider-input::-webkit-slider-thumb {
  appearance: none;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 50%;
  background: var(--color-primary, #3b82f6);
  cursor: pointer;
  transition: transform 0.2s;
}

.slider-input::-webkit-slider-thumb:hover {
  transform: scale(1.1);
}

.slider-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.slider-range {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
  color: var(--color-text-muted, #6b7280);
}
</style>
