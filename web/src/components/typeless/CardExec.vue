<script setup lang="ts">
import { ref, computed, onMounted, shallowRef, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardExec } from '@/types/typeless'
import { renderMarkdown } from '@/utils/markdown'

const { t } = useI18n()

// Unescape backticks that were escaped to prevent regex matching issues
// The backend escapes ``` with `​`` (backtick + zero-width space + backticks)
function unescapeBackticks(s: string): string {
  return s.replace(/`​``/g, '```')
}

// Get stdout with backticks unescaped
const stdout = computed(() => unescapeBackticks(props.card.stdout || ''))

// Get command with backticks unescaped
const command = computed(() => unescapeBackticks(props.card.command || ''))

// Mermaid rendering state
const mermaidModule = shallowRef<typeof import('mermaid') | null>(null)
const mermaidReady = ref(false)
let mermaidRenderCount = 0

async function initMermaid() {
  if (mermaidModule.value) return
  mermaidModule.value = await import('mermaid')
  const isDark = document.documentElement.classList.contains('dark')
  mermaidModule.value.default.initialize({
    startOnLoad: false,
    theme: isDark ? 'dark' : 'default',
    securityLevel: 'strict',
    fontFamily: 'ui-sans-serif, system-ui, sans-serif',
    suppressErrorRendering: true,
  })
  mermaidReady.value = true
}

// Render mermaid code blocks in the container
async function renderMermaidBlocks(container: HTMLElement) {
  if (!mermaidReady.value) await initMermaid()

  // Find all code blocks with mermaid language
  // The markdown renderer outputs: <div class="code-block"><div class="code-header"><span>mermaid</span></div><pre><code>...</code></pre></div>
  const codeBlocks = container.querySelectorAll('.code-block')

  for (const block of codeBlocks) {
    // Check if this is a mermaid code block by looking at the header
    const header = block.querySelector('.code-header span')
    if (!header || header.textContent?.toLowerCase() !== 'mermaid') continue

    // Get the code content
    const codeEl = block.querySelector('pre code')
    if (!codeEl) continue
    const code = codeEl.textContent || ''
    if (!code.trim()) continue

    // Skip if already rendered
    if (block.dataset.mermaidRendered) continue
    block.dataset.mermaidRendered = 'true'

    try {
      mermaidRenderCount++
      const id = `exec-mermaid-${Date.now()}-${mermaidRenderCount}`
      const { svg } = await mermaidModule.value!.default.render(id, code)

      // Replace the entire code-block div with rendered SVG
      const wrapper = document.createElement('div')
      wrapper.className = 'mermaid-diagram'
      wrapper.innerHTML = svg
      wrapper.style.cssText = 'display: flex; justify-content: center; padding: 1rem; background: #1f2937; border-radius: 0.375rem; margin: 0.5rem 0;'
      block.replaceWith(wrapper)
    } catch (err) {
      console.error('Mermaid render error in exec:', err)
      // Keep original code block on error
    }
  }
}

const props = defineProps<{
  card: TypelessCardExec
}>()

const copied = ref(false)
const expanded = ref(false)

const isRunning = computed(() => props.card.status === 'running' || props.card._streaming)
const isError = computed(() => props.card.status === 'error' || (props.card.exit_code != null && props.card.exit_code !== 0))
const isSandbox = computed(() => props.card.host === 'sandbox')
const riskLevel = computed(() => props.card.risk_level || 'low')
const showRiskBadge = computed(() => riskLevel.value !== 'low')

// Extract skill name from "blue <subcommand> ..." pattern
const skillName = computed(() => {
  if (props.card.skill_name) return props.card.skill_name
  const cmd = command.value
  const m = cmd.match(/^blue\s+(\S+)/)
  return m ? m[1] : null
})

// Border color based on host + risk + error + running
const borderClass = computed(() => {
  if (isRunning.value) return 'border-indigo-500/50'
  if (isError.value) return 'border-red-500/60'
  if (isSandbox.value && (riskLevel.value === 'high' || riskLevel.value === 'critical')) return 'border-red-500/50'
  if (isSandbox.value) return 'border-indigo-500/50'
  if (riskLevel.value === 'critical') return 'border-red-500/50'
  if (riskLevel.value === 'high') return 'border-orange-500/50'
  if (riskLevel.value === 'medium') return 'border-amber-500/50'
  return 'border-gray-700'
})

// Header accent bar color
const accentClass = computed(() => {
  if (isRunning.value) return 'bg-indigo-500 exec-accent-pulse'
  if (isError.value) return 'bg-red-500'
  if (isSandbox.value && (riskLevel.value === 'high' || riskLevel.value === 'critical')) return 'bg-red-500'
  if (isSandbox.value) return 'bg-indigo-500'
  if (riskLevel.value === 'critical') return 'bg-red-500'
  if (riskLevel.value === 'high') return 'bg-orange-500'
  if (riskLevel.value === 'medium') return 'bg-amber-500'
  return 'bg-emerald-500'
})

const riskBadgeClass = computed(() => {
  switch (riskLevel.value) {
    case 'critical': return 'bg-red-500/20 text-red-300 ring-red-500/30'
    case 'high': return 'bg-orange-500/20 text-orange-300 ring-orange-500/30'
    case 'medium': return 'bg-amber-500/20 text-amber-300 ring-amber-500/30'
    default: return 'bg-emerald-500/20 text-emerald-300 ring-emerald-500/30'
  }
})

const riskLabel = computed(() => {
  return t(`execCard.risk.${riskLevel.value}`, riskLevel.value)
})

const hasOutput = computed(() => !!(stdout.value || props.card.stderr))
const outputLines = computed(() => {
  const lines = stdout.value.split('\n')
  return lines.length
})

// Detect if stdout contains markdown that should be rendered
const stdoutIsMarkdown = computed(() => {
  const stdoutVal = stdout.value
  // Check for common markdown patterns
  const markdownPatterns = [
    /^#{1,6}\s/m,           // Headers (# Heading)
    /^\s*[-*+]\s/m,          // Unordered lists
    /^\s*\d+\.\s/m,          // Ordered lists
    /^\s*>/m,                // Blockquotes
    /^\s*\|/m,               // Tables
    /```\w*/,                // Code blocks (including mermaid)
    /\[.*\]\(.*\)/m,         // Links
    /\*\*.*\*\*/m,           // Bold
    /\*.*\*/m,               // Italic
  ]
  // Also check for mermaid specifically
  const hasMermaid = /```mermaid/i.test(stdoutVal)
  const hasMarkdown = markdownPatterns.some(pattern => pattern.test(stdoutVal))
  return hasMermaid || hasMarkdown
})

// Rendered stdout (markdown or plain)
const renderedStdout = computed(() => {
  if (!stdout.value) return ''
  if (stdoutIsMarkdown.value) {
    return renderMarkdown(stdout.value)
  }
  // Plain text - escape HTML and preserve whitespace
  return stdout.value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
})
const shouldCollapse = computed(() => outputLines.value > 12)
const isExpanded = computed(() => expanded.value || !shouldCollapse.value)

const displayCommand = computed(() => {
  const cmd = command.value
  return cmd.length > 120 ? cmd.slice(0, 120) + '...' : cmd
})

const durationDisplay = computed(() => {
  const ms = props.card.duration_ms
  if (ms == null) return ''
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
})

async function copyCommand() {
  try {
    await navigator.clipboard.writeText(command.value)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch { /* ignore */ }
}

// Watch for markdown content changes and render mermaid diagrams
watch(stdoutIsMarkdown, async (isMarkdown) => {
  if (isMarkdown && stdout.value) {
    await nextTick()
    await nextTick() // Double tick to ensure DOM is updated
    const container = document.querySelector(`#exec-${props.card.session_id || ''} .markdown-content`)
    if (container) {
      await renderMermaidBlocks(container as HTMLElement)
    }
  }
})
</script>

<template>
  <div :id="`exec-${card.session_id || ''}`" class="exec-card rounded-lg border overflow-hidden" :class="borderClass">
    <!-- Accent bar -->
    <div class="h-0.5" :class="accentClass" />

    <!-- Header -->
    <div class="flex items-center gap-2 px-3 py-2 bg-gray-800">
      <!-- Traffic lights / Running indicator -->
      <div class="flex gap-1 flex-shrink-0">
        <template v-if="isRunning">
          <div class="w-2.5 h-2.5 rounded-full bg-indigo-500 exec-dot-pulse" />
          <div class="w-2.5 h-2.5 rounded-full bg-indigo-500/40" />
          <div class="w-2.5 h-2.5 rounded-full bg-indigo-500/20" />
        </template>
        <template v-else>
          <div class="w-2.5 h-2.5 rounded-full" :class="isError ? 'bg-red-500' : 'bg-emerald-500'" />
          <div class="w-2.5 h-2.5 rounded-full bg-yellow-500" />
          <div class="w-2.5 h-2.5 rounded-full bg-gray-600" />
        </template>
      </div>

      <!-- Skill name badge (when detected) -->
      <span
        v-if="skillName"
        class="inline-flex items-center px-1.5 py-0.5 text-[10px] font-medium rounded bg-indigo-500/15 text-indigo-300 ring-1 ring-inset ring-indigo-500/25 flex-shrink-0"
      >
        {{ skillName }}
      </span>

      <!-- Command -->
      <div class="flex-1 min-w-0 flex items-center gap-2">
        <span class="text-gray-400 text-xs flex-shrink-0">$</span>
        <span class="text-gray-100 text-xs font-mono truncate" :title="card.command">{{ displayCommand }}</span>
      </div>

      <!-- Badges -->
      <div class="flex items-center gap-1.5 flex-shrink-0">
        <!-- Sandbox badge -->
        <span
          v-if="isSandbox"
          class="inline-flex items-center gap-1 px-1.5 py-0.5 text-[10px] font-medium rounded bg-indigo-500/20 text-indigo-300 ring-1 ring-inset ring-indigo-500/30"
        >
          <svg class="w-3 h-3" viewBox="0 0 16 16" fill="currentColor">
            <path fill-rule="evenodd" d="M8 1a3.5 3.5 0 00-3.5 3.5V7H3a1 1 0 00-1 1v5a3 3 0 003 3h6a3 3 0 003-3V8a1 1 0 00-1-1h-1.5V4.5A3.5 3.5 0 008 1zm2 6V4.5a2 2 0 10-4 0V7h4z" clip-rule="evenodd" />
          </svg>
          {{ t('execCard.sandbox', 'Sandbox') }}
        </span>

        <!-- Risk badge -->
        <span
          v-if="showRiskBadge"
          class="inline-flex items-center px-1.5 py-0.5 text-[10px] font-medium rounded ring-1 ring-inset"
          :class="riskBadgeClass"
        >
          {{ riskLabel }}
        </span>

        <!-- Copy button -->
        <button
          class="p-1 rounded text-gray-500 hover:text-gray-300 hover:bg-gray-700 transition-colors"
          :title="copied ? t('execCard.copied', 'Copied!') : t('execCard.copyCommand', 'Copy command')"
          @click="copyCommand"
        >
          <svg v-if="!copied" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
          <svg v-else class="w-3.5 h-3.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Output -->
    <div v-if="hasOutput || isRunning" class="bg-gray-900 relative">
      <div
        class="overflow-auto px-3 py-2 text-xs leading-relaxed"
        :class="{ 'max-h-[300px]': isExpanded, 'max-h-[180px] overflow-hidden': !isExpanded, 'font-mono': !stdoutIsMarkdown }"
      >
        <!-- stdout - rendered as markdown or plain text -->
        <div v-if="stdout && stdoutIsMarkdown" class="markdown-content text-gray-300" v-html="renderedStdout + (isRunning ? '<span class=&quot;exec-cursor&quot;>█</span>' : '')" />
        <pre v-else-if="stdout" class="text-gray-300 whitespace-pre-wrap break-all m-0"><span v-html="renderedStdout" /><span v-if="isRunning" class="exec-cursor">█</span></pre>
        <!-- Running with no stdout yet -->
        <pre v-else-if="isRunning" class="text-gray-500 whitespace-pre-wrap m-0"><span class="exec-cursor">&#9608;</span></pre>
        <!-- stderr separator -->
        <div v-if="stdout && card.stderr" class="border-t border-gray-700/50 my-1.5" />
        <!-- stderr -->
        <pre v-if="card.stderr" class="text-red-400/90 whitespace-pre-wrap break-all m-0">{{ card.stderr }}</pre>
      </div>

      <!-- Collapse/expand toggle -->
      <div v-if="shouldCollapse" class="relative">
        <div v-if="!isExpanded" class="absolute bottom-full left-0 right-0 h-8 bg-gradient-to-t from-gray-900 to-transparent pointer-events-none" />
        <button
          class="w-full py-1 text-[10px] text-gray-500 hover:text-gray-300 bg-gray-900 hover:bg-gray-800 transition-colors text-center"
          @click="expanded = !expanded"
        >
          {{ isExpanded ? t('execCard.collapse', 'Show less') : t('execCard.expand', 'Show more') }} ({{ outputLines }} {{ t('execCard.lines', 'lines') }})
        </button>
      </div>
    </div>

    <!-- No output placeholder (only when finished with no output) -->
    <div v-else-if="!isRunning" class="bg-gray-900 px-3 py-2">
      <span class="text-xs text-gray-600 italic">{{ t('execCard.noOutput', 'No output') }}</span>
    </div>

    <!-- Footer -->
    <div class="flex items-center justify-between px-3 py-1.5 bg-gray-800/80 text-[10px]">
      <div class="flex items-center gap-2">
        <!-- Running indicator -->
        <span v-if="isRunning" class="inline-flex items-center gap-1 text-indigo-400">
          <span class="exec-spinner" />
          {{ t('execCard.running', 'Running...') }}
        </span>

        <!-- Exit code pill -->
        <span
          v-if="!isRunning && card.exit_code != null"
          class="inline-flex items-center px-1.5 py-0.5 rounded font-mono font-medium"
          :class="card.exit_code === 0
            ? 'bg-emerald-500/15 text-emerald-400'
            : 'bg-red-500/15 text-red-400'"
        >
          {{ t('execCard.exitCode', 'exit') }} {{ card.exit_code }}
        </span>

        <!-- Duration -->
        <span v-if="durationDisplay" class="text-gray-500">
          {{ durationDisplay }}
        </span>

        <!-- Truncated warning -->
        <span v-if="card.truncated" class="text-amber-500">
          {{ t('execCard.outputTruncated', 'truncated') }}
        </span>
      </div>

      <!-- Warnings -->
      <div v-if="card.warnings?.length" class="text-amber-500/80 truncate max-w-[50%]" :title="card.warnings.join('; ')">
        {{ card.warnings.join('; ') }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.exec-card {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
.exec-card pre {
  font-family: 'Fira Code', 'Monaco', 'Consolas', 'Liberation Mono', 'Courier New', monospace;
  margin: 0;
}

/* Pulsing dot for running state */
.exec-dot-pulse {
  animation: exec-pulse 1.5s ease-in-out infinite;
}
@keyframes exec-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

/* Accent bar pulse */
.exec-accent-pulse {
  animation: exec-accent 2s ease-in-out infinite;
}
@keyframes exec-accent {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

/* Blinking cursor */
.exec-cursor {
  color: #818cf8;
  animation: exec-blink 1s step-end infinite;
  font-size: 0.7em;
  line-height: 1;
}
@keyframes exec-blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

/* Spinner for footer */
.exec-spinner {
  display: inline-block;
  width: 8px;
  height: 8px;
  border: 1.5px solid currentColor;
  border-top-color: transparent;
  border-radius: 50%;
  animation: exec-spin 0.8s linear infinite;
}
@keyframes exec-spin {
  to { transform: rotate(360deg); }
}

/* Markdown content styling */
.markdown-content {
  /* Headers */
  --tw-heading-color: #e5e7eb;
}
.markdown-content :deep(h1) { font-size: 1.5em; font-weight: bold; margin: 0.5em 0; color: #e5e7eb; }
.markdown-content :deep(h2) { font-size: 1.3em; font-weight: bold; margin: 0.5em 0; color: #e5e7eb; }
.markdown-content :deep(h3) { font-size: 1.1em; font-weight: bold; margin: 0.5em 0; color: #e5e7eb; }
.markdown-content :deep(h4),
.markdown-content :deep(h5),
.markdown-content :deep(h6) { font-weight: bold; margin: 0.5em 0; color: #e5e7eb; }

/* Lists */
.markdown-content :deep(ul),
.markdown-content :deep(ol) { margin: 0.5em 0; padding-left: 1.5em; }
.markdown-content :deep(li) { margin: 0.25em 0; }

/* Code blocks */
.markdown-content :deep(.code-block) {
  margin: 0.75em 0;
  border-radius: 0.375rem;
  overflow: hidden;
  background: #374151;
}
.markdown-content :deep(.code-header) {
  padding: 0.5rem 1rem;
  background: #4b5563;
  color: #9ca3af;
  font-size: 0.75rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.markdown-content :deep(.code-block pre) {
  padding: 1rem;
  margin: 0;
  overflow-x: auto;
  background: #374151;
}
.markdown-content :deep(.code-block code) {
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.8em;
  color: #e5e7eb;
}

/* Inline code */
.markdown-content :deep(.inline-code) {
  background: #374151;
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  font-family: 'Fira Code', monospace;
  font-size: 0.875em;
  color: #fca5a5;
}

/* Mermaid diagrams */
.markdown-content :deep(.mermaid-svg) {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background: #1f2937;
  border-radius: 0.375rem;
  margin: 0.75em 0;
}
.markdown-content :deep(.mermaid-svg svg) {
  max-width: 100%;
  height: auto;
}

/* Tables */
.markdown-content :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0.75em 0;
}
.markdown-content :deep(th),
.markdown-content :deep(td) {
  border: 1px solid #4b5563;
  padding: 0.5rem;
  text-align: left;
}
.markdown-content :deep(th) {
  background: #374151;
}

/* Blockquotes */
.markdown-content :deep(blockquote) {
  border-left: 4px solid #6b7280;
  padding-left: 1rem;
  margin: 0.5em 0;
  color: #9ca3af;
  font-style: italic;
}

/* Links */
.markdown-content :deep(a) {
  color: #60a5fa;
  text-decoration: underline;
}

/* Horizontal rules */
.markdown-content :deep(hr) {
  border-color: #4b5563;
  margin: 1em 0;
}
</style>
