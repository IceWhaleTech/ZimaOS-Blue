<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { isFullscreen, fullscreenContent } from '@/composables/useFullscreen'

const { t } = useI18n()

const copied = ref(false)
const viewMode = ref<'unified' | 'split'>('unified')

function close() {
  isFullscreen.value = false
  fullscreenContent.value = null
  document.body.style.overflow = ''
}

// Handle escape key
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isFullscreen.value) {
    close()
  }
}

// Handle custom event from functional cards
function handleFullscreenEvent(e: Event) {
  const customEvent = e as CustomEvent<{ type: string; dataJson: string }>
  try {
    const data = JSON.parse(customEvent.detail.dataJson)
    fullscreenContent.value = {
      type: customEvent.detail.type as 'code' | 'diff' | 'terminal',
      ...data,
    }
    isFullscreen.value = true
    document.body.style.overflow = 'hidden'
  } catch (err) {
    console.error('Failed to open fullscreen:', err)
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('typeless-fullscreen', handleFullscreenEvent)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('typeless-fullscreen', handleFullscreenEvent)
})

async function copyContent() {
  if (!fullscreenContent.value) return

  let textToCopy = ''
  if (fullscreenContent.value.type === 'diff') {
    textToCopy = `--- ${fullscreenContent.value.oldLabel || t('diffCard.original', 'Original')}\n+++ ${fullscreenContent.value.newLabel || t('diffCard.modified', 'Modified')}\n\n${fullscreenContent.value.newCode || ''}`
  } else {
    textToCopy = fullscreenContent.value.content
  }

  try {
    await navigator.clipboard.writeText(textToCopy)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (e) {
    console.error('Failed to copy:', e)
  }
}

// Diff computation
interface DiffLine {
  type: 'unchanged' | 'added' | 'removed'
  content: string
  oldLineNum?: number
  newLineNum?: number
}

const diffLines = computed((): DiffLine[] => {
  if (fullscreenContent.value?.type !== 'diff') return []

  const oldLines = (fullscreenContent.value.oldCode || '').split('\n')
  const newLines = (fullscreenContent.value.newCode || '').split('\n')
  const result: DiffLine[] = []
  const oldSet = new Set(oldLines)
  const newSet = new Set(newLines)

  let oldIdx = 0
  let newIdx = 0

  while (oldIdx < oldLines.length || newIdx < newLines.length) {
    const oldLine = oldLines[oldIdx] ?? ''
    const newLine = newLines[newIdx] ?? ''

    if (oldIdx >= oldLines.length) {
      result.push({ type: 'added', content: newLine, newLineNum: newIdx + 1 })
      newIdx++
    } else if (newIdx >= newLines.length) {
      result.push({ type: 'removed', content: oldLine, oldLineNum: oldIdx + 1 })
      oldIdx++
    } else if (oldLine === newLine) {
      result.push({
        type: 'unchanged',
        content: oldLine,
        oldLineNum: oldIdx + 1,
        newLineNum: newIdx + 1,
      })
      oldIdx++
      newIdx++
    } else if (!newSet.has(oldLine)) {
      result.push({ type: 'removed', content: oldLine, oldLineNum: oldIdx + 1 })
      oldIdx++
    } else if (!oldSet.has(newLine)) {
      result.push({ type: 'added', content: newLine, newLineNum: newIdx + 1 })
      newIdx++
    } else {
      result.push({ type: 'removed', content: oldLine, oldLineNum: oldIdx + 1 })
      oldIdx++
    }
  }

  return result
})

const splitDiff = computed(() => {
  const left: (DiffLine | null)[] = []
  const right: (DiffLine | null)[] = []

  for (const line of diffLines.value) {
    if (line.type === 'unchanged') {
      left.push(line)
      right.push(line)
    } else if (line.type === 'removed') {
      left.push(line)
      right.push(null)
    } else if (line.type === 'added') {
      left.push(null)
      right.push(line)
    }
  }

  return { left, right }
})

const stats = computed(() => {
  let added = 0
  let removed = 0
  for (const line of diffLines.value) {
    if (line.type === 'added') added++
    if (line.type === 'removed') removed++
  }
  return { added, removed }
})

function getLineClass(type: string): string {
  switch (type) {
    case 'added':
      return 'bg-green-500/15'
    case 'removed':
      return 'bg-red-500/15'
    default:
      return ''
  }
}

function getLinePrefix(type: string): string {
  switch (type) {
    case 'added':
      return '+'
    case 'removed':
      return '-'
    default:
      return ' '
  }
}

// Code lines
const codeLines = computed(() => {
  if (!fullscreenContent.value || fullscreenContent.value.type === 'diff') return []
  return fullscreenContent.value.content.split('\n')
})

// Language display
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

const languageDisplay = computed(() => {
  const lang = fullscreenContent.value?.language
  if (!lang) return ''
  return languageNames[lang.toLowerCase()] || lang
})
</script>

<template>
  <Teleport to="body">
    <Transition name="fullscreen">
      <div
        v-if="isFullscreen && fullscreenContent"
        class="fixed inset-0 z-[9999] bg-slate-950/95 text-slate-100 backdrop-blur-sm flex flex-col"
        @click.self="close"
      >
        <!-- Header -->
        <div
          class="flex items-center justify-between px-6 py-4 bg-slate-900/95 border-b border-slate-700/60"
        >
          <div class="flex items-center gap-4">
            <!-- Window controls -->
            <div class="flex gap-1.5">
              <div
                class="w-3 h-3 rounded-full bg-red-500 cursor-pointer hover:brightness-110"
                @click="close"
              />
              <div class="w-3 h-3 rounded-full bg-yellow-500" />
              <div class="w-3 h-3 rounded-full bg-green-500" />
            </div>
            <!-- Title -->
            <h2 v-if="fullscreenContent.title" class="text-lg font-medium text-white">
              {{ fullscreenContent.title }}
            </h2>
            <!-- Language badge -->
            <span
              v-if="languageDisplay"
              class="px-2 py-1 text-xs rounded border border-slate-600/80 bg-slate-800/80 text-slate-200"
            >
              {{ languageDisplay }}
            </span>
            <!-- Diff stats -->
            <div v-if="fullscreenContent.type === 'diff'" class="flex items-center gap-2 text-sm">
              <span class="text-green-400">+{{ stats.added }}</span>
              <span class="text-red-400">-{{ stats.removed }}</span>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <!-- View mode toggle for diff -->
            <div
              v-if="fullscreenContent.type === 'diff'"
              class="flex rounded-lg border border-slate-600/70 overflow-hidden"
            >
              <button
                class="px-3 py-1.5 text-sm font-medium transition-colors"
                :class="
                  viewMode === 'unified'
                    ? 'bg-slate-700 text-white'
                    : 'text-slate-300 hover:bg-slate-700/40'
                "
                @click="viewMode = 'unified'"
              >
                {{ t('diffCard.unified', 'Unified') }}
              </button>
              <button
                class="px-3 py-1.5 text-sm font-medium transition-colors"
                :class="
                  viewMode === 'split'
                    ? 'bg-slate-700 text-white'
                    : 'text-slate-300 hover:bg-slate-700/40'
                "
                @click="viewMode = 'split'"
              >
                {{ t('diffCard.split', 'Split') }}
              </button>
            </div>

            <!-- Copy button -->
            <button
              class="flex items-center gap-2 px-3 py-1.5 text-sm text-slate-200 hover:text-white hover:bg-slate-700/60 rounded-lg transition-colors"
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
              {{ copied ? t('common.copied', 'Copied!') : t('common.copy', 'Copy') }}
            </button>

            <!-- Close button -->
            <button
              class="p-2 text-slate-300 hover:text-white hover:bg-slate-700/60 rounded-lg transition-colors"
              @click="close"
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
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
        </div>

        <!-- Labels for diff -->
        <div
          v-if="
            fullscreenContent.type === 'diff' &&
            (fullscreenContent.oldLabel || fullscreenContent.newLabel)
          "
          class="flex border-b border-slate-700/60 text-sm"
        >
          <div
            v-if="viewMode === 'split'"
            class="flex-1 px-6 py-2 bg-red-900/20 text-red-300 font-medium"
          >
            {{ fullscreenContent.oldLabel || t('diffCard.original', 'Original') }}
          </div>
          <div
            v-if="viewMode === 'split'"
            class="fullscreen-modal-border-start flex-1 px-6 py-2 bg-green-900/20 text-green-300 font-medium"
          >
            {{ fullscreenContent.newLabel || t('diffCard.modified', 'Modified') }}
          </div>
        </div>

        <!-- Content -->
        <div class="flex-1 overflow-auto bg-gradient-to-b from-slate-900/50 to-slate-950">
          <!-- Code / Terminal content -->
          <template
            v-if="fullscreenContent.type === 'code' || fullscreenContent.type === 'terminal'"
          >
            <pre
              class="p-6 text-sm leading-relaxed min-h-full"
            ><code class="text-slate-100"><template v-for="(line, index) in codeLines" :key="index"><span class="inline-block w-full hover:bg-slate-700/40"><span class="fullscreen-modal-line-number fullscreen-modal-line-number-gap inline-block w-12 text-slate-500 select-none">{{ index + 1 }}</span>{{ line }}
</span></template></code></pre>
          </template>

          <!-- Diff unified view -->
          <template v-else-if="fullscreenContent.type === 'diff' && viewMode === 'unified'">
            <pre
              class="text-sm min-h-full"
            ><code><template v-for="(line, index) in diffLines" :key="index"><div
                  class="flex hover:bg-slate-700/35"
                  :class="getLineClass(line.type)"
                ><span class="fullscreen-modal-gutter w-16 px-4 text-slate-500 select-none flex-shrink-0">{{ line.oldLineNum || '' }}</span><span class="fullscreen-modal-gutter w-16 px-4 text-slate-500 select-none flex-shrink-0">{{ line.newLineNum || '' }}</span><span
                    class="w-8 text-center flex-shrink-0"
                    :class="{
                      'text-green-400': line.type === 'added',
                      'text-red-400': line.type === 'removed',
                      'text-slate-500': line.type === 'unchanged'
                    }"
                  >{{ getLinePrefix(line.type) }}</span><span class="flex-1 px-4 text-slate-100">{{ line.content }}</span></div></template></code></pre>
          </template>

          <!-- Diff split view -->
          <template v-else-if="fullscreenContent.type === 'diff' && viewMode === 'split'">
            <div class="flex min-h-full">
              <!-- Left (old) -->
              <div class="fullscreen-modal-border-end flex-1">
                <pre
                  class="text-sm"
                ><code><template v-for="(line, index) in splitDiff.left" :key="'left-' + index"><div
                      class="flex hover:bg-slate-700/35"
                      :class="line ? getLineClass(line.type) : 'bg-slate-800/60'"
                    ><span class="fullscreen-modal-gutter w-14 px-4 text-slate-500 select-none flex-shrink-0">{{ line?.oldLineNum || '' }}</span><span
                        v-if="line"
                        class="w-8 text-center flex-shrink-0"
                        :class="line.type === 'removed' ? 'text-red-400' : 'text-slate-500'"
                      >{{ line.type === 'removed' ? '-' : ' ' }}</span><span v-else class="w-8 flex-shrink-0" /><span class="flex-1 px-4 text-slate-100">{{ line?.content || '' }}</span></div></template></code></pre>
              </div>
              <!-- Right (new) -->
              <div class="flex-1">
                <pre
                  class="text-sm"
                ><code><template v-for="(line, index) in splitDiff.right" :key="'right-' + index"><div
                      class="flex hover:bg-slate-700/35"
                      :class="line ? getLineClass(line.type) : 'bg-slate-800/60'"
                    ><span class="fullscreen-modal-gutter w-14 px-4 text-slate-500 select-none flex-shrink-0">{{ line?.newLineNum || '' }}</span><span
                        v-if="line"
                        class="w-8 text-center flex-shrink-0"
                        :class="line.type === 'added' ? 'text-green-400' : 'text-slate-500'"
                      >{{ line.type === 'added' ? '+' : ' ' }}</span><span v-else class="w-8 flex-shrink-0" /><span class="flex-1 px-4 text-slate-100">{{ line?.content || '' }}</span></div></template></code></pre>
              </div>
            </div>
          </template>
        </div>

        <!-- Footer hint -->
        <div class="px-6 py-2 bg-slate-900/95 border-t border-slate-700/60 text-center">
          <i18n-t
            keypath="fullscreenModal.exitHint"
            scope="global"
            tag="span"
            class="text-xs text-slate-300"
          >
            <template #key>
              <kbd class="px-1.5 py-0.5 bg-slate-800 rounded border border-slate-600 text-slate-100"
                >Esc</kbd
              >
            </template>
          </i18n-t>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
pre {
  margin: 0;
  font-family: 'Fira Code', 'Monaco', 'Consolas', 'Liberation Mono', 'Courier New', monospace;
}

.fullscreen-modal-line-number {
  text-align: end;
}

.fullscreen-modal-line-number-gap {
  margin-inline-end: 1.5rem;
}

.fullscreen-modal-gutter {
  text-align: end;
  border-inline-end: 1px solid rgba(51, 65, 85, 0.6);
}

.fullscreen-modal-border-start {
  border-inline-start: 1px solid rgba(51, 65, 85, 0.6);
}

.fullscreen-modal-border-end {
  border-inline-end: 1px solid rgba(51, 65, 85, 0.6);
}

.fullscreen-enter-active,
.fullscreen-leave-active {
  transition: all 0.2s ease;
}

.fullscreen-enter-from,
.fullscreen-leave-to {
  opacity: 0;
  transform: scale(0.95);
}
</style>
