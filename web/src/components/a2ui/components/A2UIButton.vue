<template>
  <button
    class="a2ui-button"
    :class="[`variant-${variant}`, `size-${size}`, { disabled: disabled }]"
    :disabled="disabled"
    :style="componentStyle"
    @click="handleClick"
  >
    {{ label }}
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
}>()

const emit = defineEmits<{
  (e: 'action', payload: { actionId: string; confirmRequired: boolean; confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string } }): void
}>()

const label = computed(() => props.component.props?.label as string || 'Button')
const variant = computed(() => props.component.props?.variant as string || 'primary')
const size = computed(() => props.component.props?.size as string || 'medium')
const disabled = computed(() => props.component.props?.disabled as boolean || false)

const componentStyle = computed(() => {
  const style = props.component.style || {}
  return {
    backgroundColor: style.backgroundColor as string | undefined,
    color: style.color as string | undefined,
    borderRadius: style.borderRadius as string | undefined,
  }
})

function handleClick() {
  const clickAction = props.component.actions?.find(a => a.type === 'click')
  if (clickAction) {
    emit('action', {
      actionId: clickAction.id,
      confirmRequired: !!clickAction.confirm,
      confirmDialog: clickAction.confirm,
    })
  }
}
</script>

<style scoped>
.a2ui-button {
  border: none;
  cursor: pointer;
  font-weight: 500;
  transition: all 0.2s;
}

.a2ui-button.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.size-small {
  padding: 0.375rem 0.75rem;
  font-size: 0.875rem;
}

.size-medium {
  padding: 0.625rem 1.25rem;
  font-size: 1rem;
}

.size-large {
  padding: 0.875rem 1.75rem;
  font-size: 1.125rem;
}

.variant-primary {
  background: var(--color-primary, #3b82f6);
  color: white;
  border-radius: 0.5rem;
}

.variant-primary:hover:not(.disabled) {
  opacity: 0.9;
}

.variant-secondary {
  background: var(--color-background-soft, #f3f4f6);
  color: var(--color-text, #1f2937);
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 0.5rem;
}

.variant-secondary:hover:not(.disabled) {
  background: var(--color-background-mute, #e5e7eb);
}

.variant-danger {
  background: #ef4444;
  color: white;
  border-radius: 0.5rem;
}

.variant-danger:hover:not(.disabled) {
  background: #dc2626;
}

.variant-success {
  background: #22c55e;
  color: white;
  border-radius: 0.5rem;
}

.variant-success:hover:not(.disabled) {
  background: #16a34a;
}

.variant-ghost {
  background: transparent;
  color: var(--color-primary, #3b82f6);
}

.variant-ghost:hover:not(.disabled) {
  background: var(--color-background-soft, #f3f4f6);
}
</style>
