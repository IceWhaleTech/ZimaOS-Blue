<script setup lang="ts">
import { ref, computed } from 'vue'
import type { TypelessCardCollapsibleCode } from '@/types/typeless'
import { useFullscreen } from '@/composables/useFullscreen'

const props = defineProps<{
  card: TypelessCardCollapsibleCode
}>()

const { openFullscreen } = useFullscreen()
const expanded = ref(props.card.defaultExpanded ?? false)
const copied = ref(false)

function handleDoubleClick() {
  openFullscreen({
    type: 'code',
    title: props.card.filename || props.card.title || getLanguageDisplay(),
    language: props.card.language,
    content: props.card.code,
  })
}

const lines = computed(() => props.card.code.split('\n'))
const maxCollapsedLines = computed(() => props.card.maxCollapsedLines || 5)
const shouldCollapse = computed(() => lines.value.length > maxCollapsedLines.value)

const displayedLines = computed(() => {
  if (!shouldCollapse.value || expanded.value) {
    return lines.value
  }
  return lines.value.slice(0, maxCollapsedLines.value)
})

const hiddenLinesCount = computed(() => {
  if (!shouldCollapse.value || expanded.value) return 0
  return lines.value.length - maxCollapsedLines.value
})

function toggleExpand() {
  expanded.value = !expanded.value
}

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
}

function getLanguageDisplay(): string {
  if (!props.card.language) return ''
  return languageNames[props.card.language.toLowerCase()] || props.card.language
}
</script>

<template>
  <div class="collapsible-code-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-gray-200" @dblclick="handleDoubleClick">
    <!-- Header -->
    <div class="flex items-center justify-between px-4 py-2 bg-gray-700 border-b border-gray-700">
      <div class="flex items-center gap-3">
        <!-- Window controls -->
        <div class="flex gap-1.5">
          <div class="w-3 h-3 rounded-full bg-red-500" />
          <div class="w-3 h-3 rounded-full bg-yellow-500" />
          <div class="w-3 h-3 rounded-full bg-green-500" />
        </div>
        <!-- Filename or title -->
        <span v-if="card.filename || card.title" class="text-sm text-gray-400">
          {{ card.filename || card.title }}
        </span>
        <!-- Language badge -->
        <span v-if="card.language" class="px-2 py-0.5 text-xs rounded bg-gray-700 text-gray-300">
          {{ getLanguageDisplay() }}
        </span>
        <!-- Lines count -->
        <span class="text-xs text-gray-500">{{ lines.length }} lines</span>
      </div>
      <div class="flex items-center gap-2">
        <!-- Fullscreen hint -->
        <span class="text-xs text-gray-500 hidden sm:inline" title="Double-click to fullscreen">⤢</span>
        <!-- Copy button -->
        <button
          class="flex items-center gap-1.5 px-2 py-1 text-xs text-gray-400 hover:text-white transition-colors rounded hover:bg-gray-700"
          @click.stop="copyCode"
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
          <span>{{ copied ? 'Copied!' : 'Copy' }}</span>
        </button>
      </div>
    </div>

    <!-- Code content -->
    <div class="relative">
      <div class="overflow-x-auto">
        <pre class="p-4 text-sm leading-relaxed"><code class="text-gray-100"><template v-for="(line, index) in displayedLines" :key="index"><span class="inline-block w-full"><span
              v-if="card.showLineNumbers !== false"
              class="inline-block w-8 text-right mr-4 text-gray-500 select-none"
            >{{ index + 1 }}</span>{{ line }}
</span></template></code></pre>
      </div>

      <!-- Collapsed overlay -->
      <div
        v-if="shouldCollapse && !expanded"
        class="absolute bottom-0 left-0 right-0 h-16 bg-gradient-to-t from-gray-900 to-transparent pointer-events-none"
      />
    </div>

    <!-- Expand/Collapse button -->
    <div v-if="shouldCollapse" class="border-t border-gray-700">
      <button
        class="w-full px-4 py-2 text-sm text-gray-400 hover:text-white hover:bg-gray-700 transition-colors flex items-center justify-center gap-2"
        @click.stop="toggleExpand"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 transition-transform"
          :class="{ 'rotate-180': expanded }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
        <span v-if="!expanded">Show {{ hiddenLinesCount }} more lines</span>
        <span v-else>Collapse</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
pre {
  margin: 0;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
}
</style>
