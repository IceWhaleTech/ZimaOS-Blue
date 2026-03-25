<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardCollapsibleCode } from '@/types/typeless'
import { useFullscreen } from '@/composables/useFullscreen'
import { renderMarkdown } from '@/utils/markdown'
import { getLocalizedToolName } from '@/utils/toolLocalization'

const { t, te } = useI18n()

const props = defineProps<{
  card: TypelessCardCollapsibleCode
}>()

const { openFullscreen } = useFullscreen()
const expanded = ref(props.card.defaultExpanded ?? false)
const copied = ref(false)
const isMarkdown = computed(
  () => !props.card.language || props.card.language.toLowerCase() === 'markdown'
)
const renderedMarkdown = computed(() => (isMarkdown.value ? renderMarkdown(props.card.code) : ''))
const isFileReadCard = computed(() => {
  const normalizedTitle = (props.card.title || '').trim().toLowerCase().replace(/\s+/g, '_')
  return normalizedTitle === 'file_read' || normalizedTitle === 'read'
})
const filePath = computed(() => (props.card.filename || '').trim())
const fileName = computed(() => {
  const path = filePath.value
  if (!path) return ''
  return path.split(/[\\/]/).filter(Boolean).pop() || path
})
const fileDirectory = computed(() => {
  const path = filePath.value
  const name = fileName.value
  if (!path || !name) return ''
  const idx = path.lastIndexOf(name)
  if (idx <= 0) return ''
  return path.slice(0, idx).replace(/[\\/]$/, '')
})
const fileExtension = computed(() => {
  const name = fileName.value
  const idx = name.lastIndexOf('.')
  if (idx <= 0 || idx === name.length - 1) return ''
  return name.slice(idx + 1).toUpperCase()
})
const localizedTitle = computed(() => {
  if (isFileReadCard.value) {
    return getLocalizedToolName('file_read', t, te)
  }
  return (props.card.title || '').trim()
})

function handleDoubleClick() {
  openFullscreen({
    type: 'code',
    title: fileName.value || props.card.filename || localizedTitle.value || getLanguageDisplay(),
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
  <div
    class="collapsible-code-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden"
    :class="isMarkdown ? 'bg-white dark:bg-gray-800' : 'bg-gray-50 dark:bg-gray-900'"
    @dblclick="handleDoubleClick"
  >
    <!-- Header -->
    <div
      class="flex items-center justify-between px-3 py-1.5 border-b"
      :class="
        isMarkdown
          ? 'bg-gray-50 dark:bg-gray-700/50 border-gray-200 dark:border-gray-700'
          : 'bg-gray-100 dark:bg-gray-800 border-gray-200 dark:border-gray-700'
      "
    >
      <div class="flex items-center gap-2">
        <template v-if="isFileReadCard">
          <span class="text-base">📄</span>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold tracking-wide bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200"
              >
                {{ localizedTitle || card.title }}
              </span>
              <span
                v-if="fileExtension"
                class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold tracking-wide bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-200"
              >
                {{ fileExtension }}
              </span>
              <span class="text-xs text-gray-400 dark:text-gray-500"
                >{{ lines.length }} {{ t('execCard.lines', 'lines') }}</span
              >
            </div>
            <div class="mt-1 min-w-0">
              <div class="truncate text-xs font-medium text-gray-700 dark:text-gray-200">
                {{ fileName || card.filename || localizedTitle || card.title }}
              </div>
              <div
                v-if="fileDirectory || filePath"
                class="truncate text-[11px] font-mono text-gray-400 dark:text-gray-500"
              >
                {{ fileDirectory || filePath }}
              </div>
            </div>
          </div>
        </template>
        <template v-else>
          <!-- Markdown icon -->
          <span v-if="isMarkdown" class="text-base">📄</span>
          <!-- Filename or title -->
          <span
            v-if="card.filename || localizedTitle"
            class="text-xs"
            :class="
              isMarkdown
                ? 'text-gray-700 dark:text-gray-300 font-medium'
                : 'text-gray-500 dark:text-gray-400'
            "
          >
            {{ card.filename || localizedTitle }}
          </span>
          <!-- Language badge -->
          <span
            v-if="card.language && !isMarkdown && !(card.filename || localizedTitle)"
            class="text-xs text-gray-500 dark:text-gray-400"
          >
            {{ getLanguageDisplay() }}
          </span>
          <!-- Fallback label -->
          <span
            v-if="!isMarkdown && !card.filename && !localizedTitle && !card.language"
            class="text-xs text-gray-500 dark:text-gray-400"
            >{{ t('codeBlock.code', 'code') }}</span
          >
          <!-- Lines count -->
          <span v-if="!isMarkdown" class="text-xs text-gray-400 dark:text-gray-500"
            >{{ lines.length }} {{ t('execCard.lines', 'lines') }}</span
          >
        </template>
      </div>
      <div class="flex items-center gap-2">
        <!-- Fullscreen hint -->
        <span
          class="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline"
          :title="t('media.fullscreen', 'Full Screen')"
          >⤢</span
        >
        <!-- Copy button -->
        <button
          class="flex items-center gap-1 px-1.5 py-0.5 text-xs transition-colors rounded"
          :class="
            isMarkdown
              ? 'text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
              : 'text-gray-400 dark:text-gray-500 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-700'
          "
          @click.stop="copyCode"
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
    </div>

    <!-- Markdown rendered content -->
    <div v-if="isMarkdown" class="relative">
      <div
        class="overflow-y-auto px-4 py-3 prose prose-sm dark:prose-invert max-w-none"
        :class="{ 'max-h-48': shouldCollapse && !expanded }"
        v-html="renderedMarkdown"
      />
      <div
        v-if="shouldCollapse && !expanded"
        class="absolute bottom-0 left-0 right-0 h-16 bg-gradient-to-t from-white dark:from-gray-800 to-transparent pointer-events-none"
      />
    </div>

    <!-- Code content -->
    <div v-else class="relative">
      <div class="overflow-x-auto">
        <pre
          class="p-4 text-sm leading-relaxed"
        ><code class="text-gray-800 dark:text-gray-100"><template v-for="(line, index) in displayedLines" :key="index"><span class="inline-block w-full"><span
              v-if="card.showLineNumbers !== false"
              class="inline-block w-8 text-right mr-4 text-gray-400 dark:text-gray-600 select-none"
            >{{ index + 1 }}</span>{{ line }}
</span></template></code></pre>
      </div>

      <!-- Collapsed overlay -->
      <div
        v-if="shouldCollapse && !expanded"
        class="absolute bottom-0 left-0 right-0 h-16 bg-gradient-to-t from-gray-50 dark:from-gray-900 to-transparent pointer-events-none"
      />
    </div>

    <!-- Expand/Collapse button -->
    <div
      v-if="shouldCollapse"
      class="border-t"
      :class="
        isMarkdown ? 'border-gray-200 dark:border-gray-700' : 'border-gray-200 dark:border-gray-700'
      "
    >
      <button
        class="w-full px-4 py-2 text-sm transition-colors flex items-center justify-center gap-2"
        :class="
          isMarkdown
            ? 'text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700/50'
            : 'text-gray-400 dark:text-gray-500 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700'
        "
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
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 9l-7 7-7-7"
          />
        </svg>
        <span v-if="!expanded"
          >{{ t('execCard.expand', 'Show more') }} {{ hiddenLinesCount }}
          {{ t('execCard.lines', 'lines') }}</span
        >
        <span v-else>{{ t('execCard.collapse', 'Collapse') }}</span>
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
