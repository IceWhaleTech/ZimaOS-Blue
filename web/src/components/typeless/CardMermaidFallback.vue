<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardMermaid } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardMermaid
}>()

const { t } = useI18n()
const copied = ref(false)

const diagramType = computed(() => {
  const code = props.card.code.trim().toLowerCase()
  if (code.startsWith('flowchart') || code.startsWith('graph')) return 'flowchart'
  if (code.startsWith('mindmap')) return 'mindmap'
  if (code.startsWith('sequencediagram') || code.startsWith('sequence')) return 'sequence'
  if (code.startsWith('classdiagram') || code.startsWith('class')) return 'class'
  if (code.startsWith('statediagram') || code.startsWith('state')) return 'state'
  if (code.startsWith('erdiagram') || code.startsWith('er')) return 'er'
  if (code.startsWith('gantt')) return 'gantt'
  if (code.startsWith('pie')) return 'pie'
  if (code.startsWith('journey')) return 'journey'
  if (code.startsWith('gitgraph') || code.startsWith('git')) return 'gitgraph'
  if (code.startsWith('timeline')) return 'timeline'
  if (code.startsWith('quadrantchart') || code.startsWith('quadrant')) return 'quadrant'
  if (code.startsWith('sankey')) return 'sankey'
  if (code.startsWith('xychart') || code.startsWith('xy')) return 'xychart'
  return 'diagram'
})

const diagramTypeDisplay = computed(() => t(`mermaid.${diagramType.value}`))

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
</script>

<template>
  <div
    class="mermaid-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700 flex flex-col h-full"
  >
    <div
      class="flex items-center justify-between px-3 py-1.5 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700 flex-shrink-0"
    >
      <div class="flex items-center gap-2">
        <svg
          class="w-4 h-4 text-pink-500"
          viewBox="0 0 24 24"
          fill="currentColor"
        >
          <path
            d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"
          />
        </svg>
        <span
          v-if="card.title"
          class="text-sm text-gray-600 dark:text-gray-400"
        >
          {{ card.title }}
        </span>
        <span
          class="px-2 py-0.5 text-xs rounded bg-pink-100 dark:bg-pink-900/30 text-pink-700 dark:text-pink-300"
        >
          {{ diagramTypeDisplay }}
        </span>
      </div>
      <button
        class="flex items-center p-1 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors rounded hover:bg-gray-200 dark:hover:bg-gray-700"
        :title="t('mermaid.copyCode')"
        @click="copyCode"
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
      </button>
    </div>
    <pre
      class="p-4 overflow-auto text-sm leading-6 bg-gray-50 dark:bg-gray-900/40 text-gray-800 dark:text-gray-100 whitespace-pre-wrap break-words"
    ><code>{{ card.code }}</code></pre>
  </div>
</template>
