<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardCode } from '@/types/typeless'
import { highlightCode } from '@/utils/markdown'

const { t } = useI18n()

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
  const lang = props.card.language.toLowerCase()
  if (lang === 'tool_call') return t('codeBlock.toolCall', 'Tool Call')
  return languageNames[lang] || props.card.language
}

const headerLabel = computed(() => {
  return props.card.filename || props.card.title || getLanguageDisplay() || 'code'
})
</script>

<template>
  <div
    class="code-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-gray-50 dark:bg-gray-900"
  >
    <!-- Header -->
    <div
      class="flex items-center justify-between px-3 py-1.5 bg-gray-100 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700"
    >
      <span class="text-xs text-gray-500 dark:text-gray-400">{{ headerLabel }}</span>
      <!-- Copy button -->
      <button
        class="flex items-center gap-1 px-1.5 py-0.5 text-xs text-gray-400 dark:text-gray-500 hover:text-gray-700 dark:hover:text-gray-200 transition-colors rounded hover:bg-gray-200 dark:hover:bg-gray-700"
        @click="copyCode"
      >
        <svg
          v-if="!copied"
          xmlns="http://www.w3.org/2000/svg"
          class="h-3.5 w-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
          />
        </svg>
        <svg
          v-else
          xmlns="http://www.w3.org/2000/svg"
          class="h-3.5 w-3.5 text-green-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M5 13l4 4L19 7"
          />
        </svg>
        <span>{{ copied ? t('common.copied', 'Copied') : t('common.copy', 'Copy') }}</span>
      </button>
    </div>

    <!-- Code content -->
    <div class="overflow-x-auto">
      <pre
        class="p-4 text-sm leading-relaxed"
      ><code class="text-gray-800 dark:text-gray-100"><template v-for="(line, index) in highlightedLines" :key="index"><span
            class="inline-block w-full"
            :class="{ 'bg-yellow-500/20': isHighlighted(index + 1) }"
          ><span
              v-if="card.showLineNumbers !== false && highlightedLines.length > 1"
              class="inline-block w-8 text-right mr-4 text-gray-400 dark:text-gray-600 select-none"
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
