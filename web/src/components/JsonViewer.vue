<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import hljs from 'highlight.js/lib/core'
import json from 'highlight.js/lib/languages/json'

hljs.registerLanguage('json', json)

const props = defineProps<{
  data: unknown
  maxHeight?: string
}>()

const viewMode = ref<'formatted' | 'raw'>('formatted')

const formattedJson = computed(() => {
  if (props.data === null || props.data === undefined) return ''
  const obj = typeof props.data === 'string' ? tryParse(props.data) : props.data
  return JSON.stringify(obj, null, 2)
})

const rawJson = computed(() => {
  if (props.data === null || props.data === undefined) return ''
  const obj = typeof props.data === 'string' ? tryParse(props.data) : props.data
  return JSON.stringify(obj)
})

const highlighted = computed(() => {
  const text = viewMode.value === 'formatted' ? formattedJson.value : rawJson.value
  if (!text) return ''
  return hljs.highlight(text, { language: 'json' }).value
})

function tryParse(s: string): unknown {
  try { return JSON.parse(s) } catch { return s }
}

function copyToClipboard() {
  navigator.clipboard.writeText(formattedJson.value)
}
</script>

<template>
  <div class="json-viewer rounded-lg bg-gray-50 dark:bg-gray-900/50 border border-gray-200 dark:border-gray-700 overflow-hidden">
    <div class="flex items-center gap-1 px-2 py-1 border-b border-gray-200 dark:border-gray-700 bg-gray-100 dark:bg-gray-800/50">
      <button
        class="px-2 py-0.5 text-[10px] rounded font-medium transition-colors"
        :class="viewMode === 'formatted'
          ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
          : 'text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200'"
        @click="viewMode = 'formatted'"
      >
        Pretty
      </button>
      <button
        class="px-2 py-0.5 text-[10px] rounded font-medium transition-colors"
        :class="viewMode === 'raw'
          ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
          : 'text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200'"
        @click="viewMode = 'raw'"
      >
        Raw
      </button>
      <div class="flex-1" />
      <button
        class="p-0.5 rounded text-gray-400 dark:text-slate-500 hover:text-gray-600 dark:hover:text-slate-300 transition-colors"
        title="Copy JSON"
        @click="copyToClipboard"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
      </button>
    </div>
    <div
      class="overflow-auto p-2.5"
      :style="{ maxHeight: maxHeight || '24rem' }"
    >
      <pre class="text-[11px] font-mono leading-relaxed whitespace-pre-wrap break-all"><code v-html="highlighted" /></pre>
    </div>
  </div>
</template>

<style scoped>
.json-viewer :deep(.hljs) {
  background: transparent;
}
.json-viewer :deep(.hljs-string) {
  color: #16a34a;
}
.json-viewer :deep(.hljs-number) {
  color: #2563eb;
}
.json-viewer :deep(.hljs-literal) {
  color: #9333ea;
}
.json-viewer :deep(.hljs-keyword) {
  color: #dc2626;
}
.json-viewer :deep(.hljs-attr) {
  color: #0891b2;
}
.dark .json-viewer :deep(.hljs-string) {
  color: #4ade80;
}
.dark .json-viewer :deep(.hljs-number) {
  color: #60a5fa;
}
.dark .json-viewer :deep(.hljs-literal) {
  color: #c084fc;
}
.dark .json-viewer :deep(.hljs-keyword) {
  color: #f87171;
}
.dark .json-viewer :deep(.hljs-attr) {
  color: #22d3ee;
}
</style>
