<script setup lang="ts">
import { ref, computed } from 'vue'
import type { TypelessCardCode } from '@/types/typeless'
import { highlightCode } from '@/utils/markdown'

const props = defineProps<{
  card: TypelessCardCode
}>()

const copied = ref(false)

async function copyCode() {
  try {
    await navigator.clipboard.writeText(props.card.code)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy code:', err)
  }
}

// Get highlighted lines with syntax coloring
const highlightedLines = computed(() => {
  const lang = props.card.language?.toLowerCase() || ''
  const highlighted = lang ? highlightCode(props.card.code, lang) : props.card.code
  return highlighted.split('\n')
})

function isHighlighted(lineNumber: number): boolean {
  return props.card.highlightLines?.includes(lineNumber) ?? false
}

// Language display names
const languageNames: Record<string, string> = {
  js: 'JavaScript',
  javascript: 'JavaScript',
  ts: 'TypeScript',
  typescript: 'TypeScript',
  py: 'Python',
  python: 'Python',
  go: 'Go',
  rust: 'Rust',
  java: 'Java',
  cpp: 'C++',
  c: 'C',
  html: 'HTML',
  css: 'CSS',
  json: 'JSON',
  yaml: 'YAML',
  sql: 'SQL',
  bash: 'Bash',
  shell: 'Shell',
  md: 'Markdown',
  markdown: 'Markdown',
}

function getLanguageDisplay(): string {
  if (!props.card.language) return ''
  return languageNames[props.card.language.toLowerCase()] || props.card.language
}
</script>

<template>
  <div class="code-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-gray-900">
    <!-- Header -->
    <div class="flex items-center justify-between px-3 py-1.5 bg-gray-800 border-b border-gray-700">
      <div class="flex items-center gap-2">
        <!-- Window controls -->
        <div class="flex gap-1">
          <div class="w-2.5 h-2.5 rounded-full bg-red-500" />
          <div class="w-2.5 h-2.5 rounded-full bg-yellow-500" />
          <div class="w-2.5 h-2.5 rounded-full bg-green-500" />
        </div>
        <!-- Filename or title -->
        <span v-if="card.filename || card.title" class="text-sm text-gray-400">
          {{ card.filename || card.title }}
        </span>
        <!-- Language badge -->
        <span v-if="card.language" class="px-2 py-0.5 text-xs rounded bg-gray-700 text-gray-300">
          {{ getLanguageDisplay() }}
        </span>
      </div>
      <!-- Copy button -->
      <button
        class="flex items-center p-1 text-gray-400 hover:text-white transition-colors rounded hover:bg-gray-700"
        title="Copy"
        @click="copyCode"
      >
        <svg v-if="!copied" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
          />
        </svg>
        <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
      </button>
    </div>

    <!-- Code content -->
    <div class="overflow-x-auto">
      <pre class="p-4 text-sm leading-relaxed"><code class="text-gray-100"><template v-for="(line, index) in highlightedLines" :key="index"><span
            class="inline-block w-full"
            :class="{ 'bg-yellow-500/20': isHighlighted(index + 1) }"
          ><span
              v-if="card.showLineNumbers !== false && card.language"
              class="inline-block w-8 text-right mr-4 text-gray-500 select-none"
            >{{ index + 1 }}</span><span v-html="line"></span>
</span></template></code></pre>
    </div>
  </div>
</template>

<style scoped>
pre {
  margin: 0;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
}
</style>
