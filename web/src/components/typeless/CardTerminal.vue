<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useFullscreen } from '@/composables/useFullscreen'
import type { TypelessCardTerminal } from '@/types/typeless'

const { t } = useI18n()
const { openFullscreen } = useFullscreen()

const props = defineProps<{
  card: TypelessCardTerminal
}>()

const copied = ref(false)

const ansiColors: Record<number, string> = {
  30: 'text-black dark:text-gray-900',
  31: 'text-red-600 dark:text-red-400',
  32: 'text-green-600 dark:text-green-400',
  33: 'text-yellow-600 dark:text-yellow-400',
  34: 'text-blue-600 dark:text-blue-300',
  35: 'text-purple-600 dark:text-purple-400',
  36: 'text-cyan-600 dark:text-cyan-400',
  37: 'text-gray-200 dark:text-gray-100',
  90: 'text-gray-500 dark:text-gray-400',
  91: 'text-red-500 dark:text-red-300',
  92: 'text-green-500 dark:text-green-300',
  93: 'text-yellow-500 dark:text-yellow-300',
  94: 'text-blue-500 dark:text-blue-200',
  95: 'text-purple-500 dark:text-purple-300',
  96: 'text-cyan-500 dark:text-cyan-300',
  97: 'text-white',
  40: 'bg-black',
  41: 'bg-red-600',
  42: 'bg-green-600',
  43: 'bg-yellow-600',
  44: 'bg-blue-600 dark:bg-blue-500',
  45: 'bg-purple-600',
  46: 'bg-cyan-600',
  47: 'bg-gray-200',
  100: 'bg-gray-700',
  101: 'bg-red-500',
  102: 'bg-green-500',
  103: 'bg-yellow-500',
  104: 'bg-blue-500 dark:bg-blue-400',
  105: 'bg-purple-500',
  106: 'bg-cyan-500',
  107: 'bg-white',
}

const ansiStyles: Record<number, string> = {
  1: 'font-bold',
  2: 'opacity-75',
  3: 'italic',
  4: 'underline',
  7: 'ansi-reverse',
  9: 'line-through',
}

interface ParsedSegment {
  text: string
  classes: string[]
  isReverse: boolean
}

interface SummaryItem {
  key: string
  text: string
  tone?: 'neutral' | 'accent' | 'info'
  monospace?: boolean
}

function parseAnsiToSegments(text: string): ParsedSegment[] {
  const segments: ParsedSegment[] = []
  // eslint-disable-next-line no-control-regex
  const ansiRegex = /\u001b\[([0-9;]*)m/g

  let lastIndex = 0
  let currentClasses: string[] = []
  let isReverse = false
  let match

  while ((match = ansiRegex.exec(text)) !== null) {
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

    const codes = (match[1] || '').split(';').map((value) => parseInt(value, 10) || 0)

    for (const code of codes) {
      if (code === 0) {
        currentClasses = []
        isReverse = false
      } else if (code === 7) {
        isReverse = true
      } else if (code === 27) {
        isReverse = false
      } else if (ansiColors[code]) {
        const isBg = code >= 40
        currentClasses = currentClasses.filter((className) => {
          if (isBg) return !className.startsWith('bg-')
          return !className.startsWith('text-')
        })
        currentClasses.push(ansiColors[code])
      } else if (ansiStyles[code] && !currentClasses.includes(ansiStyles[code])) {
        currentClasses.push(ansiStyles[code])
      }
    }

    lastIndex = match.index + match[0].length
  }

  if (lastIndex < text.length) {
    segments.push({
      text: text.slice(lastIndex),
      classes: [...currentClasses],
      isReverse,
    })
  }

  return segments
}

function stripAnsi(text: string): string {
  // eslint-disable-next-line no-control-regex
  return text.replace(/\u001b\[[0-9;]*m/g, '')
}

function summaryChipClass(tone: SummaryItem['tone'] = 'neutral'): string {
  switch (tone) {
    case 'accent':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800/50 dark:bg-emerald-900/20 dark:text-emerald-200'
    case 'info':
      return 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800/50 dark:bg-blue-900/20 dark:text-blue-200'
    default:
      return 'border-gray-200 bg-white text-gray-700 dark:border-gray-700 dark:bg-gray-800/80 dark:text-gray-200'
  }
}

const localizedTitle = computed(() => t('terminalCard.title', 'Terminal'))
const normalizedTitle = computed(() => (props.card.title || '').trim())
const promptToken = computed(() => (props.card.prompt || '').replace(/\s+$/, ''))
const displayTitle = computed(() => {
  if (!normalizedTitle.value) return localizedTitle.value

  const normalized = normalizedTitle.value.toLowerCase().replace(/\s+/g, '_')
  if (normalized === 'terminal') return localizedTitle.value

  return normalizedTitle.value
})
const themeLabel = computed(() => (props.card.theme === 'light' ? 'light' : 'dark'))
const chromeLabel = computed(() =>
  props.card.showPrompt && promptToken.value ? promptToken.value : displayTitle.value
)
const lineCount = computed(() => {
  const content = props.card.content || ''
  if (!content) return props.card.showPrompt && promptToken.value ? 1 : 0
  return content.split('\n').length
})
const summaryItems = computed<SummaryItem[]>(() => {
  const items: SummaryItem[] = []

  if (props.card.showPrompt && promptToken.value) {
    items.push({
      key: 'prompt',
      text: promptToken.value,
      tone: 'accent',
      monospace: true,
    })
  }

  if (lineCount.value > 0) {
    items.push({
      key: 'lines',
      text: `${lineCount.value} ${t('execCard.lines', 'lines')}`,
    })
  }

  if (props.card.maxHeight) {
    items.push({
      key: 'max-height',
      text: `${props.card.maxHeight}px`,
      tone: 'info',
      monospace: true,
    })
  }

  return items
})
const parsedLines = computed(() => {
  const lines = (props.card.content || '').split('\n')
  return lines.map((line) => parseAnsiToSegments(line))
})
const plainText = computed(() => stripAnsi(props.card.content || ''))

const containerStyle = computed(() =>
  props.card.maxHeight ? { maxHeight: `${props.card.maxHeight}px` } : {}
)

const surfaceTone = computed(() => {
  if (props.card.theme === 'light') {
    return {
      shell: 'border-gray-200 bg-white',
      chrome:
        'border-gray-200 bg-gray-50 text-gray-500 dark:border-gray-200 dark:bg-gray-50 dark:text-gray-500',
      body: 'bg-stone-50 text-gray-900',
      prompt: 'text-emerald-700',
    }
  }

  return {
    shell: 'border-slate-800 bg-slate-950',
    chrome:
      'border-slate-800 bg-slate-900 text-slate-400 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-400',
    body: 'bg-slate-950 text-slate-100',
    prompt: 'text-emerald-400',
  }
})

async function copyContent() {
  if (!plainText.value) return

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

function openTerminalFullscreen() {
  openFullscreen({
    type: 'terminal',
    title: displayTitle.value || localizedTitle.value,
    content: plainText.value,
  })
}
</script>

<template>
  <div
    class="terminal-card overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800"
    @dblclick="openTerminalFullscreen"
  >
    <div
      class="flex items-start justify-between gap-3 border-b border-gray-100 bg-gray-50/80 px-4 py-3 dark:border-gray-700/60 dark:bg-gray-900/30"
    >
      <div class="flex min-w-0 items-start gap-3">
        <span
          class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.8"
              d="M8.25 7.5L4.5 11.25l3.75 3.75m4.5-7.5h6.75m-6.75 9h6.75"
            />
          </svg>
        </span>

        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="text-[11px] font-semibold uppercase tracking-[0.08em] text-gray-500 dark:text-gray-400"
            >
              {{ localizedTitle }}
            </span>
            <span
              v-if="card._streaming"
              class="inline-flex h-2 w-2 rounded-full bg-amber-400 animate-pulse"
            />
          </div>

          <div class="mt-1 truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
            {{ displayTitle }}
          </div>

          <div v-if="summaryItems.length" class="mt-2 flex flex-wrap items-center gap-2">
            <span
              v-for="item in summaryItems"
              :key="item.key"
              class="inline-flex items-center rounded-full border px-2.5 py-1 text-[11px] font-medium"
              :class="[summaryChipClass(item.tone), item.monospace ? 'font-mono' : '']"
            >
              {{ item.text }}
            </span>
          </div>
        </div>
      </div>

      <div class="flex shrink-0 items-center gap-2">
        <button
          data-testid="terminal-fullscreen"
          class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white/90 px-2.5 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-white"
          :title="t('media.fullscreen', 'Full Screen')"
          @click.stop="openTerminalFullscreen"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-3.5 w-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.8"
              d="M8.25 3.75H4.5v3.75m0 9V20.25h3.75m7.5 0h3.75V16.5m0-9V3.75h-3.75"
            />
          </svg>
          <span class="hidden sm:inline">{{ t('media.fullscreen', 'Full Screen') }}</span>
        </button>

        <button
          data-testid="terminal-copy"
          class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white/90 px-2.5 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-white"
          :disabled="!plainText"
          :title="copied ? t('common.copied', 'Copied') : t('common.copy', 'Copy')"
          @click.stop="copyContent"
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
              stroke-width="1.8"
              d="M8.25 7.5H6.75A2.25 2.25 0 004.5 9.75v8.25a2.25 2.25 0 002.25 2.25H15a2.25 2.25 0 002.25-2.25V16.5m-6-9h6.75A2.25 2.25 0 0120.25 9.75v6.75A2.25 2.25 0 0118 18.75h-6.75A2.25 2.25 0 019 16.5V9.75A2.25 2.25 0 0111.25 7.5z"
            />
          </svg>
          <svg
            v-else
            xmlns="http://www.w3.org/2000/svg"
            class="h-3.5 w-3.5 text-emerald-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.8"
              d="M4.5 12.75l6 6 9-13.5"
            />
          </svg>
          <span class="hidden sm:inline">
            {{ copied ? t('common.copied', 'Copied') : t('common.copy', 'Copy') }}
          </span>
        </button>
      </div>
    </div>

    <div class="p-3">
      <div class="overflow-hidden rounded-xl border shadow-sm" :class="surfaceTone.shell">
        <div
          class="flex items-center justify-between gap-3 border-b px-3 py-2 text-[11px] font-medium uppercase tracking-[0.08em]"
          :class="surfaceTone.chrome"
        >
          <span class="inline-flex items-center gap-2">
            <span class="flex gap-1.5">
              <span class="h-2 w-2 rounded-full bg-rose-400/80" />
              <span class="h-2 w-2 rounded-full bg-amber-400/80" />
              <span class="h-2 w-2 rounded-full bg-emerald-400/80" />
            </span>
            <span>{{ themeLabel }}</span>
          </span>
          <span class="truncate font-mono normal-case tracking-normal opacity-80">
            {{ chromeLabel }}
          </span>
        </div>

        <div class="overflow-auto" :class="surfaceTone.body" :style="containerStyle">
          <pre class="p-4 text-sm leading-relaxed whitespace-pre-wrap break-words"
            ><template v-for="(line, lineIndex) in parsedLines" :key="lineIndex"
              ><span
                v-if="card.showPrompt && promptToken && lineIndex === 0"
                :class="surfaceTone.prompt"
                >{{ card.prompt }}</span
              ><template v-for="(segment, segIndex) in line" :key="segIndex"
                ><span
                  :class="[...segment.classes, segment.isReverse ? 'ansi-reverse' : '']"
                  >{{ segment.text }}</span
                ></template
              >{{ lineIndex < parsedLines.length - 1 ? '\n' : '' }}</template
            ></pre
          >
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
pre {
  margin: 0;
  font-family: 'Fira Code', 'Monaco', 'Consolas', 'Liberation Mono', 'Courier New', monospace;
  overflow-wrap: anywhere;
}

.ansi-reverse {
  filter: invert(1);
}
</style>
