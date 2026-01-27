<template>
  <div class="a2ui-markdown" v-html="renderedContent"></div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
}>()

const content = computed(() => props.component.props?.content as string || '')

// Simple markdown rendering (basic support)
const renderedContent = computed(() => {
  let html = content.value
    // Escape HTML
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    // Headers
    .replace(/^### (.*$)/gm, '<h3>$1</h3>')
    .replace(/^## (.*$)/gm, '<h2>$1</h2>')
    .replace(/^# (.*$)/gm, '<h1>$1</h1>')
    // Bold
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    // Italic
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    // Code
    .replace(/`(.*?)`/g, '<code>$1</code>')
    // Links
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
    // Line breaks
    .replace(/\n/g, '<br>')

  return html
})
</script>

<style scoped>
.a2ui-markdown {
  line-height: 1.6;
}

.a2ui-markdown :deep(h1) {
  font-size: 1.5rem;
  font-weight: 700;
  margin: 1rem 0 0.5rem 0;
}

.a2ui-markdown :deep(h2) {
  font-size: 1.25rem;
  font-weight: 600;
  margin: 0.875rem 0 0.5rem 0;
}

.a2ui-markdown :deep(h3) {
  font-size: 1.125rem;
  font-weight: 600;
  margin: 0.75rem 0 0.5rem 0;
}

.a2ui-markdown :deep(code) {
  padding: 0.125rem 0.375rem;
  background: var(--color-background-soft, #f3f4f6);
  border-radius: 0.25rem;
  font-family: monospace;
  font-size: 0.875em;
}

.a2ui-markdown :deep(a) {
  color: var(--color-primary, #3b82f6);
  text-decoration: none;
}

.a2ui-markdown :deep(a:hover) {
  text-decoration: underline;
}

.a2ui-markdown :deep(strong) {
  font-weight: 600;
}
</style>
