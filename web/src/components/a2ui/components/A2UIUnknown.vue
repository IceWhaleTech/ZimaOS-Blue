<template>
  <div class="a2ui-unknown">
    <span class="unknown-icon">?</span>
    <span class="unknown-text">Unknown component type: {{ component.type }}</span>
  </div>
</template>

<script setup lang="ts">
import type { Component } from '@/api/a2ui'

defineProps<{
  component: Component
}>()
</script>

<style scoped>
.a2ui-unknown {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem;
  background: #fef3c7;
  border: 1px solid #f59e0b;
  border-radius: 0.5rem;
  color: #92400e;
  font-size: 0.875rem;
}

.unknown-icon {
  width: 1.25rem;
  height: 1.25rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f59e0b;
  color: white;
  border-radius: 50%;
  font-size: 0.75rem;
  font-weight: 700;
}
</style>
