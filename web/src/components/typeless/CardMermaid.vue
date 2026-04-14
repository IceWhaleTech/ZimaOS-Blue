<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick, shallowRef, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import TrustedHtml from '@/components/common/TrustedHtml.vue'
import type { TypelessCardMermaid } from '@/types/typeless'
import {
  embeddedMermaidBundleDisabled,
  loadMermaidRuntime,
  type MermaidRuntime,
} from '@/utils/mermaidRuntimeLoader'

const props = defineProps<{
  card: TypelessCardMermaid
}>()

const { t } = useI18n()
const containerRef = ref<HTMLElement | null>(null)
const svgContent = ref('')
const error = ref<string | null>(null)
const copied = ref(false)
const loading = ref(true)
const remoteFallback = ref(false)
// Use shallowRef for the Mermaid runtime to avoid reactivity issues
const mermaidModule = shallowRef<MermaidRuntime | null>(null)
// Track render count to generate unique IDs
let renderCount = 0
// Track latest render request to avoid stale async updates
let renderToken = 0
// Track whether this card is still receiving streaming content
const isStreamingCard = computed(() => props.card._streaming === true)

function isParseLikeError(message: string): boolean {
  return /parse error|lexical error|syntax error/i.test(message)
}

function canAttemptStreamingRender(code: string): boolean {
  if (code.length < 12) return false
  const firstLine = code.split('\n', 1)[0]?.trim().toLowerCase() || ''
  return /^(flowchart|graph|mindmap|sequencediagram|sequence|classdiagram|class|statediagram|state|erdiagram|er|gantt|pie|journey|gitgraph|timeline|quadrantchart|quadrant|sankey|xychart|xy)\b/.test(
    firstLine
  )
}

function scheduleRender(delay = isStreamingCard.value ? 280 : 100): void {
  if (renderTimeout) {
    clearTimeout(renderTimeout)
  }
  renderTimeout = setTimeout(() => {
    renderDiagram()
  }, delay)
}

// Detect diagram type from code
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

// Display name for the diagram type (using i18n)
const diagramTypeDisplay = computed(() => {
  return t(`mermaid.${diagramType.value}`)
})

// Initialize mermaid with theme
async function initMermaid() {
  if (!mermaidModule.value) {
    mermaidModule.value = await loadMermaidRuntime()
  }

  const isDark = document.documentElement.classList.contains('dark')
  const theme = props.card.theme || (isDark ? 'dark' : 'default')

  mermaidModule.value.default.initialize({
    startOnLoad: false,
    theme: theme,
    securityLevel: 'strict',
    fontFamily: 'ui-sans-serif, system-ui, sans-serif',
  })
}

// Render the mermaid diagram
async function renderDiagram() {
  if (!containerRef.value) return

  const code = props.card.code?.trim() || ''
  if (!code) {
    svgContent.value = ''
    error.value = null
    remoteFallback.value = false
    loading.value = false
    return
  }

  if (isStreamingCard.value && !canAttemptStreamingRender(code)) {
    error.value = null
    loading.value = !svgContent.value
    return
  }

  error.value = null
  remoteFallback.value = false
  // Keep old diagram visible during re-render to avoid flicker.
  if (!svgContent.value) {
    loading.value = true
  }

  const token = ++renderToken

  try {
    await initMermaid()

    // Generate unique ID for this render using timestamp and counter
    renderCount++
    const id = `mermaid-${Date.now()}-${renderCount}-${Math.random().toString(36).substring(2, 9)}`

    // Clean up any existing SVG elements with old IDs to prevent conflicts
    const existingSvgs = document.querySelectorAll('[id^="mermaid-"]')
    existingSvgs.forEach((svg) => {
      if (svg.id !== id && !containerRef.value?.contains(svg)) {
        svg.remove()
      }
    })

    // Render the diagram
    const { svg } = await mermaidModule.value!.default.render(id, code)
    if (token !== renderToken) return
    svgContent.value = svg
  } catch (err) {
    if (token !== renderToken) return
    const message = err instanceof Error ? err.message : String(err)
    if (embeddedMermaidBundleDisabled && !mermaidModule.value) {
      svgContent.value = ''
      error.value = null
      remoteFallback.value = true
      return
    }
    // Parse/lexical/syntax errors are expected while streaming; ignore silently and keep last valid render.
    if (isParseLikeError(message)) {
      error.value = null
      return
    }
    if (!isParseLikeError(message)) {
      console.error('Mermaid render error:', err)
    }
    error.value = err instanceof Error ? err.message : 'Failed to render diagram'
  } finally {
    if (token === renderToken) {
      loading.value = false
    }
  }
}

// Copy code to clipboard
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

// Watch for theme changes
watch(
  () => props.card.theme,
  () => {
    scheduleRender(0)
  }
)

// Watch for code changes with debounce to prevent rapid re-renders
let renderTimeout: ReturnType<typeof setTimeout> | null = null
watch(
  () => props.card.code,
  () => {
    scheduleRender()
  }
)

// When streaming finishes, render once with final complete code.
watch(isStreamingCard, (streaming, wasStreaming) => {
  if (wasStreaming && !streaming) {
    scheduleRender(0)
  }
})

onMounted(() => {
  nextTick(() => {
    scheduleRender(0)
  })
})

onUnmounted(() => {
  if (renderTimeout) {
    clearTimeout(renderTimeout)
  }
})
</script>

<template>
  <div
    class="mermaid-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700 flex flex-col h-full"
  >
    <!-- Header -->
    <div
      class="flex items-center justify-between px-3 py-1.5 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700 flex-shrink-0"
    >
      <div class="flex items-center gap-2">
        <!-- Mermaid icon -->
        <svg
          class="w-4 h-4 text-pink-500"
          viewBox="0 0 24 24"
          fill="currentColor"
        >
          <path
            d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"
          />
        </svg>
        <!-- Title -->
        <span
          v-if="card.title"
          class="text-sm text-gray-600 dark:text-gray-400"
        >
          {{ card.title }}
        </span>
        <!-- Diagram type badge -->
        <span
          class="px-2 py-0.5 text-xs rounded bg-pink-100 dark:bg-pink-900/30 text-pink-700 dark:text-pink-300"
        >
          {{ diagramTypeDisplay }}
        </span>
      </div>
      <!-- Copy button -->
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

    <!-- Content -->
    <div
      ref="containerRef"
      class="flex-1 overflow-auto flex items-center justify-center min-h-[200px]"
    >
      <pre
        v-if="remoteFallback"
        class="w-full h-full overflow-auto p-4 text-sm leading-6 bg-gray-50 dark:bg-gray-900/40 text-gray-800 dark:text-gray-100 whitespace-pre-wrap break-words"
      ><code>{{ card.code }}</code></pre>

      <!-- Error state -->
      <div
        v-else-if="error"
        class="flex items-center gap-2 text-red-500 dark:text-red-400 p-4"
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
            d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <span class="text-sm">{{ error }}</span>
      </div>

      <!-- Loading state -->
      <div
        v-else-if="loading"
        class="flex items-center justify-center py-8"
      >
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-pink-500" />
      </div>

      <!-- Rendered diagram -->
      <TrustedHtml
        v-else
        class="mermaid-svg w-full h-full flex items-center justify-center p-4"
        :html="svgContent"
      />
    </div>
  </div>
</template>

<style scoped>
.mermaid-svg :deep(svg) {
  max-width: 100%;
  height: auto;
}

/* Ensure text is readable in dark mode */
.dark .mermaid-svg :deep(.nodeLabel),
.dark .mermaid-svg :deep(.edgeLabel),
.dark .mermaid-svg :deep(.label) {
  color: #e5e7eb;
}
</style>
