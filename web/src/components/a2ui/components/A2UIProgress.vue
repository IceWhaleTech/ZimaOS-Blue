<template>
  <div class="a2ui-progress">
    <div v-if="label" class="progress-header">
      <span class="progress-label">{{ label }}</span>
      <span v-if="showValue" class="progress-value">{{ percentage }}%</span>
    </div>
    <div class="progress-bar" :style="{ height: barHeight }">
      <div
        class="progress-fill"
        :class="[`variant-${variant}`]"
        :style="{ width: `${percentage}%` }"
      ></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
}>()

const value = computed(() => props.component.props?.value as number || 0)
const max = computed(() => props.component.props?.max as number || 100)
const label = computed(() => props.component.props?.label as string)
const showValue = computed(() => props.component.props?.showValue as boolean ?? true)
const variant = computed(() => props.component.props?.variant as string || 'primary')
const barHeight = computed(() => props.component.props?.height as string || '0.5rem')

const percentage = computed(() => {
  return Math.min(100, Math.max(0, (value.value / max.value) * 100))
})
</script>

<style scoped>
.a2ui-progress {
  width: 100%;
}

.progress-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 0.375rem;
}

.progress-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text, #1f2937);
}

.progress-value {
  font-size: 0.875rem;
  color: var(--color-text-muted, #6b7280);
}

.progress-bar {
  width: 100%;
  background: var(--color-background-soft, #e5e7eb);
  border-radius: 0.25rem;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  border-radius: 0.25rem;
  transition: width 0.3s ease;
}

.variant-primary {
  background: var(--color-primary, #3b82f6);
}

.variant-success {
  background: #22c55e;
}

.variant-warning {
  background: #f59e0b;
}

.variant-danger {
  background: #ef4444;
}
</style>
