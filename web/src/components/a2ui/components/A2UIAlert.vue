<template>
  <div class="a2ui-alert" :class="[`variant-${variant}`]" role="alert">
    <div class="alert-icon">{{ icon }}</div>
    <div class="alert-content">
      <div v-if="title" class="alert-title">{{ title }}</div>
      <div class="alert-message">{{ message }}</div>
    </div>
    <button v-if="dismissible" class="alert-dismiss" @click="handleDismiss">&times;</button>
  </div>
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

const variant = computed(() => props.component.props?.type as string || 'info')
const title = computed(() => props.component.props?.title as string)
const message = computed(() => props.component.props?.message as string || '')
const dismissible = computed(() => props.component.props?.dismissible as boolean || false)

const icon = computed(() => {
  const icons: Record<string, string> = {
    info: 'i',
    success: '✓',
    warning: '!',
    error: '✕',
  }
  return icons[variant.value] || 'i'
})

function handleDismiss() {
  const dismissAction = props.component.actions?.find(a => a.type === 'dismiss')
  if (dismissAction) {
    emit('action', {
      actionId: dismissAction.id,
      confirmRequired: false,
    })
  }
}
</script>

<style scoped>
.a2ui-alert {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 1rem;
  border-radius: 0.5rem;
}

.alert-icon {
  width: 1.5rem;
  height: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 0.75rem;
  font-weight: 700;
  flex-shrink: 0;
}

.alert-content {
  flex: 1;
}

.alert-title {
  font-weight: 600;
  margin-bottom: 0.25rem;
}

.alert-message {
  font-size: 0.875rem;
}

.alert-dismiss {
  background: none;
  border: none;
  font-size: 1.25rem;
  cursor: pointer;
  opacity: 0.5;
  transition: opacity 0.2s;
}

.alert-dismiss:hover {
  opacity: 1;
}

.variant-info {
  background: #eff6ff;
  color: #1e40af;
}

.variant-info .alert-icon {
  background: #3b82f6;
  color: white;
}

.variant-success {
  background: #f0fdf4;
  color: #166534;
}

.variant-success .alert-icon {
  background: #22c55e;
  color: white;
}

.variant-warning {
  background: #fffbeb;
  color: #92400e;
}

.variant-warning .alert-icon {
  background: #f59e0b;
  color: white;
}

.variant-error {
  background: #fef2f2;
  color: #991b1b;
}

.variant-error .alert-icon {
  background: #ef4444;
  color: white;
}
</style>
