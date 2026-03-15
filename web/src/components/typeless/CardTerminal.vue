<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardTerminal } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardTerminal
}>()

const copied = ref(false)

// ANSI color code to CSS class mapping
const ansiColors: Record<number, string> = {
  // Standard colors (foreground)
  30: 'text-black dark:text-gray-900',
  31: 'text-red-600 dark:text-red-400',
  32: 'text-green-600 dark:text-green-400',
  33: 'text-yellow-600 dark:text-yellow-400',
  34: 'text-gray-900 dark:text-white dark:text-white',
  35: 'text-purple-600 dark:text-purple-400',
  36: 'text-cyan-600 dark:text-cyan-400',
  37: 'text-gray-200 dark:text-gray-100',
  // Bright colors (foreground)
  90: 'text-gray-500 dark:text-gray-400',
  91: 'text-red-500 dark:text-red-300',
  92: 'text-green-500 dark:text-green-300',
  93: 'text-yellow-500 dark:text-yellow-300',
  94: 'text-gray-900 dark:text-white dark:text-white',
  95: 'text-purple-500 dark:text-purple-300',
  96: 'text-cyan-500 dark:text-cyan-300',
  97: 'text-white',
  // Background colors
  40: 'bg-black',
  41: 'bg-red-600',
  42: 'bg-green-600',
  43: 'bg-yellow-600',
  44: 'bg-gray-700 dark:bg-gray-500',
  45: 'bg-purple-600',
  46: 'bg-cyan-600',
  47: 'bg-gray-200',
  // Bright background colors
  100: 'bg-gray-700',
  101: 'bg-red-500',
  102: 'bg-green-500',
  103: 'bg-yellow-500',
  104: 'bg-gray-700 dark:bg-gray-500',
  105: 'bg-purple-500',
  106: 'bg-cyan-500',
  107: 'bg-white',
}

// Text style codes
const ansiStyles: Record<number, string> = {
  1: 'font-bold',
  2: 'opacity-75', // Dim
  3: 'italic',
  4: 'underline',
  7: 'ansi-reverse', // Reverse (handled specially)
  9: 'line-through',
}

interface ParsedSegment {
  text: string
  classes: string[]
  isReverse: boolean
}

// Parse ANSI escape codes and convert to HTML spans with Tailwind classes
function parseAnsiToSegments(text: string): ParsedSegment[] {
  const segments: ParsedSegment[] = []
  // Match ANSI escape sequences: ESC[...m
  // eslint-disable-next-line no-control-regex
  const ansiRegex = /\u001b\[([0-9;]*)m/g

  let lastIndex = 0
  let currentClasses: string[] = []
  let isReverse = false
  let match

  while ((match = ansiRegex.exec(text)) !== null) {
    // Add text before this escape sequence
    if (match.index > lastIndex) {
      const segmentText = text.slice(lastIndex, match.index)
      if (segmentText) {
        segments.push({
          text: segmentText,
          classes: [...currentClasses],
          isReverse,
        })
      }
    }

    // Parse the escape codes
    const codes = (match[1] || '').split(';').map((c) => parseInt(c, 10) || 0)

    for (const code of codes) {
      if (code === 0) {
        // Reset all
        currentClasses = []
        isReverse = false
      } else if (code === 7) {
        isReverse = true
      } else if (code === 27) {
        isReverse = false
      } else if (ansiColors[code]) {
        // Remove existing color class of same type (fg or bg)
        const isBg = code >= 40
        currentClasses = currentClasses.filter((c) => {
          if (isBg) return !c.startsWith('bg-')
          return !c.startsWith('text-')
        })
        currentClasses.push(ansiColors[code])
      } else if (ansiStyles[code]) {
        if (!currentClasses.includes(ansiStyles[code])) {
          currentClasses.push(ansiStyles[code])
        }
      }
    }

    lastIndex = match.index + match[0].length
  }

  // Add remaining text
  if (lastIndex < text.length) {
    segments.push({
      text: text.slice(lastIndex),
      classes: [...currentClasses],
      isReverse,
    })
  }

  return segments
}

// Get plain text without ANSI codes (for copying)
function stripAnsi(text: string): string {
  // eslint-disable-next-line no-control-regex
  return text.replace(/\u001b\[[0-9;]*m/g, '')
}

const parsedLines = computed(() => {
  const lines = props.card.content.split('\n')
  return lines.map((line) => parseAnsiToSegments(line))
})

const plainText = computed(() => stripAnsi(props.card.content))

async function copyContent() {
  try {
    await navigator.clipboard.writeText(plainText.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy terminal content:', err)
  }
}

const containerStyle = computed(() => {
  if (props.card.maxHeight) {
    return { maxHeight: `${props.card.maxHeight}px` }
  }
  return {}
})

const themeClasses = computed(() => {
  if (props.card.theme === 'light') {
    return 'bg-gray-100 text-gray-900'
  }
  return 'bg-gray-700 text-gray-100'
})
</script>

<template>
  <div class="terminal-card rounded-lg border border-gray-700 overflow-hidden">
    <!-- Header -->
    <div class="flex items-center justify-between px-4 py-2 bg-gray-700 border-b border-gray-700">
      <div class="flex items-center gap-3">
        <!-- Window controls -->
        <div class="flex gap-1.5">
          <div class="w-3 h-3 rounded-full bg-red-500" />
          <div class="w-3 h-3 rounded-full bg-yellow-500" />
          <div class="w-3 h-3 rounded-full bg-green-500" />
        </div>
        <!-- Title -->
        <span v-if="card.title" class="text-sm text-gray-400">
          {{ card.title }}
        </span>
        <!-- Terminal badge -->
        <span class="px-2 py-0.5 text-xs rounded bg-gray-700 text-gray-300"> Terminal </span>
      </div>
      <!-- Copy button -->
      <button
        class="flex items-center gap-1.5 px-2 py-1 text-xs text-gray-400 hover:text-white transition-colors rounded hover:bg-gray-700"
        @click="copyContent"
      >
        <svg
          v-if="!copied"
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4"
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
          class="h-4 w-4 text-green-400"
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
        <span>{{ copied ? t('common.copied', 'Copied!') : t('common.copy', 'Copy') }}</span>
      </button>
    </div>

    <!-- Terminal content -->
    <div class="overflow-auto" :class="themeClasses" :style="containerStyle">
      <pre
        class="p-4 text-sm leading-relaxed whitespace-pre-wrap break-all"
      ><template v-for="(line, lineIndex) in parsedLines" :key="lineIndex"><span v-if="card.showPrompt && card.prompt && lineIndex === 0" class="text-green-400">{{ card.prompt }}</span><template v-for="(segment, segIndex) in line" :key="segIndex"><span
            :class="[
              ...segment.classes,
              segment.isReverse ? 'ansi-reverse' : ''
            ]"
          >{{ segment.text }}</span></template>{{ lineIndex < parsedLines.length - 1 ? '\n' : '' }}</template></pre>
    </div>
  </div>
</template>

<style scoped>
pre {
  margin: 0;
  font-family: 'Fira Code', 'Monaco', 'Consolas', 'Liberation Mono', 'Courier New', monospace;
}

/* Handle reverse video mode */
.ansi-reverse {
  filter: invert(1);
}

/* Ensure proper line breaks */
pre {
  word-wrap: break-word;
  overflow-wrap: break-word;
}
</style>
