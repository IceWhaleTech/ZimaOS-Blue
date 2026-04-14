<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardDiff } from '@/types/typeless'
import { useFullscreen } from '@/composables/useFullscreen'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardDiff
}>()

const { openFullscreen } = useFullscreen()
const viewMode = ref<'split' | 'unified'>(props.card.viewMode ?? 'unified')

function handleDoubleClick() {
  openFullscreen({
    type: 'diff',
    title: props.card.title || props.card.filename,
    language: props.card.language,
    content: '', // Not used for diff
    oldCode: props.card.oldCode,
    newCode: props.card.newCode,
    oldLabel: props.card.oldLabel,
    newLabel: props.card.newLabel,
  })
}

const oldLines = computed(() => props.card.oldCode.split('\n'))
const newLines = computed(() => props.card.newCode.split('\n'))

// Simple diff algorithm - compare line by line
interface DiffLine {
  type: 'unchanged' | 'added' | 'removed'
  content: string
  oldLineNum?: number
  newLineNum?: number
}

const diffLines = computed((): DiffLine[] => {
  const result: DiffLine[] = []
  const oldSet = new Set(oldLines.value)
  const newSet = new Set(newLines.value)

  let oldIdx = 0
  let newIdx = 0

  // Simple LCS-based diff
  while (oldIdx < oldLines.value.length || newIdx < newLines.value.length) {
    const oldLine = oldLines.value[oldIdx] ?? ''
    const newLine = newLines.value[newIdx] ?? ''

    if (oldIdx >= oldLines.value.length) {
      // Only new lines left
      result.push({ type: 'added', content: newLine, newLineNum: newIdx + 1 })
      newIdx++
    } else if (newIdx >= newLines.value.length) {
      // Only old lines left
      result.push({ type: 'removed', content: oldLine, oldLineNum: oldIdx + 1 })
      oldIdx++
    } else if (oldLine === newLine) {
      // Lines match
      result.push({
        type: 'unchanged',
        content: oldLine,
        oldLineNum: oldIdx + 1,
        newLineNum: newIdx + 1,
      })
      oldIdx++
      newIdx++
    } else if (!newSet.has(oldLine)) {
      // Old line was removed
      result.push({ type: 'removed', content: oldLine, oldLineNum: oldIdx + 1 })
      oldIdx++
    } else if (!oldSet.has(newLine)) {
      // New line was added
      result.push({ type: 'added', content: newLine, newLineNum: newIdx + 1 })
      newIdx++
    } else {
      // Both lines exist elsewhere, treat as remove then add
      result.push({ type: 'removed', content: oldLine, oldLineNum: oldIdx + 1 })
      oldIdx++
    }
  }

  return result
})

// For split view
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

  // Compact: merge adjacent null entries
  const compactLeft: (DiffLine | null)[] = []
  const compactRight: (DiffLine | null)[] = []

  let i = 0
  while (i < left.length) {
    const leftItem = left[i]
    const rightItem = right[i]
    if (leftItem === null && rightItem !== null) {
      // Look ahead for removed lines to pair with
      let j = i
      while (j < left.length && left[j] === null && right[j] !== null) j++
      let k = i
      const prevItem = left[k - 1]
      while (k > 0 && prevItem !== null && prevItem?.type === 'removed') k--

      // Just add as-is for now
      compactLeft.push(leftItem ?? null)
      compactRight.push(rightItem ?? null)
      i++
    } else {
      compactLeft.push(left[i] ?? null)
      compactRight.push(right[i] ?? null)
      i++
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
      return 'bg-green-100 dark:bg-green-900/30'
    case 'removed':
      return 'bg-red-100 dark:bg-red-900/30'
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
</script>

<template>
  <div
    class="diff-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
    @dblclick="handleDoubleClick"
  >
    <!-- Header -->
    <div
      class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
    >
      <div class="flex items-center gap-3">
        <h4
          v-if="card.title || card.filename"
          class="font-medium text-gray-900 dark:text-white"
        >
          {{ card.title || card.filename }}
        </h4>
        <span
          v-if="card.language"
          class="px-2 py-0.5 text-xs rounded bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300"
        >
          {{ card.language }}
        </span>
      </div>
      <div class="flex items-center gap-4">
        <!-- Stats -->
        <div class="flex items-center gap-2 text-sm">
          <span class="text-green-600 dark:text-green-400">+{{ stats.added }}</span>
          <span class="text-red-600 dark:text-red-400">-{{ stats.removed }}</span>
        </div>
        <!-- Fullscreen hint -->
        <span
          class="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline"
          :title="t('media.fullscreen', 'Full Screen')"
        >⤢</span>
        <!-- View mode toggle -->
        <div class="flex rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
          <button
            class="px-3 py-1 text-xs font-medium transition-colors"
            :class="
              viewMode === 'unified'
                ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white'
                : 'text-gray-500 hover:bg-gray-50 dark:hover:bg-gray-700'
            "
            @click.stop="viewMode = 'unified'"
          >
            {{ t('diffCard.unified', 'Unified') }}
          </button>
          <button
            class="px-3 py-1 text-xs font-medium transition-colors"
            :class="
              viewMode === 'split'
                ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white'
                : 'text-gray-500 hover:bg-gray-50 dark:hover:bg-gray-700'
            "
            @click.stop="viewMode = 'split'"
          >
            {{ t('diffCard.split', 'Split') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Labels -->
    <div
      v-if="card.oldLabel || card.newLabel"
      class="flex border-b border-gray-200 dark:border-gray-700 text-xs"
    >
      <div
        v-if="viewMode === 'split'"
        class="flex-1 px-4 py-2 bg-red-50 dark:bg-red-900/10 text-red-700 dark:text-red-300 font-medium"
      >
        {{ card.oldLabel || t('diffCard.original', 'Original') }}
      </div>
      <div
        v-if="viewMode === 'split'"
        class="card-diff-border-start flex-1 px-4 py-2 bg-green-50 dark:bg-green-900/10 text-green-700 dark:text-green-300 font-medium"
      >
        {{ card.newLabel || t('diffCard.modified', 'Modified') }}
      </div>
    </div>

    <!-- Unified view -->
    <div
      v-if="viewMode === 'unified'"
      class="overflow-x-auto"
    >
      <pre class="text-sm"><code><template
        v-for="(line, index) in diffLines"
        :key="index"
      ><div
        class="flex"
        :class="getLineClass(line.type)"
      ><span class="card-diff-gutter w-12 px-2 text-gray-400 select-none flex-shrink-0">{{ line.oldLineNum || '' }}</span><span class="card-diff-gutter w-12 px-2 text-gray-400 select-none flex-shrink-0">{{ line.newLineNum || '' }}</span><span
        class="w-6 text-center flex-shrink-0"
        :class="{
          'text-green-600 dark:text-green-400': line.type === 'added',
          'text-red-600 dark:text-red-400': line.type === 'removed',
          'text-gray-400': line.type === 'unchanged'
        }"
      >{{ getLinePrefix(line.type) }}</span><span class="flex-1 px-2">{{ line.content }}</span></div></template></code></pre>
    </div>

    <!-- Split view -->
    <div
      v-else
      class="flex overflow-x-auto"
    >
      <!-- Left (old) -->
      <div class="card-diff-border-end flex-1">
        <pre
          class="text-sm"
        ><code><template
          v-for="(line, index) in splitDiff.left"
          :key="'left-' + index"
        ><div
          class="flex"
          :class="line ? getLineClass(line.type) : 'bg-gray-50 dark:bg-gray-700/50'"
        ><span class="card-diff-gutter w-10 px-2 text-gray-400 select-none flex-shrink-0">{{ line?.oldLineNum || '' }}</span><span
          v-if="line"
          class="w-6 text-center flex-shrink-0"
          :class="line.type === 'removed' ? 'text-red-600 dark:text-red-400' : 'text-gray-400'"
        >{{ line.type === 'removed' ? '-' : ' ' }}</span><span
          v-else
          class="w-6 flex-shrink-0"
        /><span class="flex-1 px-2">{{ line?.content || '' }}</span></div></template></code></pre>
      </div>
      <!-- Right (new) -->
      <div class="flex-1">
        <pre
          class="text-sm"
        ><code><template
          v-for="(line, index) in splitDiff.right"
          :key="'right-' + index"
        ><div
          class="flex"
          :class="line ? getLineClass(line.type) : 'bg-gray-50 dark:bg-gray-700/50'"
        ><span class="card-diff-gutter w-10 px-2 text-gray-400 select-none flex-shrink-0">{{ line?.newLineNum || '' }}</span><span
          v-if="line"
          class="w-6 text-center flex-shrink-0"
          :class="line.type === 'added' ? 'text-green-600 dark:text-green-400' : 'text-gray-400'"
        >{{ line.type === 'added' ? '+' : ' ' }}</span><span
          v-else
          class="w-6 flex-shrink-0"
        /><span class="flex-1 px-2">{{ line?.content || '' }}</span></div></template></code></pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
pre {
  margin: 0;
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
}

.card-diff-gutter {
  text-align: end;
  border-inline-end: 1px solid rgb(229 231 235);
}

:global(.dark) .card-diff-gutter {
  border-inline-end-color: rgb(55 65 81);
}

.card-diff-border-start {
  border-inline-start: 1px solid rgb(229 231 235);
}

.card-diff-border-end {
  border-inline-end: 1px solid rgb(229 231 235);
}

:global(.dark) .card-diff-border-start,
:global(.dark) .card-diff-border-end {
  border-inline-color: rgb(55 65 81);
}
</style>
