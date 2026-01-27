<template>
  <div class="a2ui-code">
    <div class="code-header">
      <span v-if="language" class="code-language">{{ language }}</span>
      <button class="copy-btn" @click="copyCode">{{ copied ? 'Copied!' : 'Copy' }}</button>
    </div>
    <pre class="code-block"><code>{{ code }}</code></pre>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
}>()

const copied = ref(false)

const code = computed(() => props.component.props?.code as string || '')
const language = computed(() => props.component.props?.language as string)

async function copyCode() {
  try {
    await navigator.clipboard.writeText(code.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy code:', err)
  }
}
</script>

<style scoped>
.a2ui-code {
  border-radius: 0.5rem;
  overflow: hidden;
  background: #1e1e1e;
}

.code-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 1rem;
  background: #2d2d2d;
}

.code-language {
  font-size: 0.75rem;
  color: #9ca3af;
  text-transform: uppercase;
}

.copy-btn {
  padding: 0.25rem 0.5rem;
  border: none;
  border-radius: 0.25rem;
  background: #3b3b3b;
  color: #9ca3af;
  font-size: 0.75rem;
  cursor: pointer;
  transition: all 0.2s;
}

.copy-btn:hover {
  background: #4b4b4b;
  color: white;
}

.code-block {
  margin: 0;
  padding: 1rem;
  overflow-x: auto;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.875rem;
  line-height: 1.5;
  color: #d4d4d4;
}

.code-block code {
  font-family: inherit;
}
</style>
