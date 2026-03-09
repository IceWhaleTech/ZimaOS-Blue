<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ActionButton, TypelessCardWebFetch } from '@/types/typeless'
import { renderMarkdown } from '@/utils/markdown'

const props = defineProps<{
  card: TypelessCardWebFetch
  actionLoading?: boolean
  activeActionId?: string
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

function unescapeBackticks(s: string): string {
  return s.replace(/`​``/g, '```')
}

function formatWarningCodeLabel(code?: string): string {
  switch ((code || '').trim()) {
    case 'login_wall':
      return 'Login wall'
    case 'challenge':
      return 'Challenge'
    case 'browser_required':
      return 'Browser required'
    default:
      return code ? `warning_code=${code}` : ''
  }
}

function hostFromUrl(url?: string): string {
  try {
    return new URL(url || '').hostname.replace(/^www\./, '')
  } catch {
    return ''
  }
}

const title = computed(() => {
  const raw = unescapeBackticks((props.card.title || '').trim())
  if (raw && raw !== 'web_fetch') return raw
  return hostFromUrl(props.card.url) || 'Web fetch'
})
const url = computed(() => unescapeBackticks((props.card.url || '').trim()))
const content = computed(() => unescapeBackticks(props.card.content || ''))
const warning = computed(() => unescapeBackticks((props.card.warning || '').trim()))
const warningCodeLabel = computed(() => formatWarningCodeLabel(props.card.warning_code))
const hostname = computed(() => hostFromUrl(url.value))
const isMarkdown = computed(() => (props.card.extract_mode || '').toLowerCase() === 'markdown')
const useBrowserAction = computed<ActionButton | null>(() => {
  const configured = props.card.actions?.find(action => action.id === 'use_browser')
  if (configured) return configured
  if (!url.value) return null
  return { id: 'use_browser', label: 'Use browser', variant: 'primary' }
})
const renderedMarkdown = computed(() => isMarkdown.value && content.value ? renderMarkdown(content.value) : '')
const lines = computed(() => content.value ? content.value.split('\n') : [])
const shouldCollapse = computed(() => lines.value.length > 18 || content.value.length > 1800)
const expanded = ref(false)
const copiedUrl = ref(false)
const copiedContent = ref(false)

const visibleTextContent = computed(() => {
  if (!content.value) return ''
  if (expanded.value || !shouldCollapse.value) return content.value
  return lines.value.slice(0, 18).join('\n')
})

const toneClasses = computed(() => {
  switch (props.card.status) {
    case 'warning':
      return {
        border: 'border-amber-200 dark:border-amber-800/60',
        header: 'bg-amber-50 dark:bg-amber-900/20 border-amber-100 dark:border-amber-800/50',
        dot: 'bg-amber-500',
      }
    case 'error':
      return {
        border: 'border-red-200 dark:border-red-800/60',
        header: 'bg-red-50 dark:bg-red-900/20 border-red-100 dark:border-red-800/50',
        dot: 'bg-red-500',
      }
    default:
      return {
        border: 'border-sky-200 dark:border-sky-800/60',
        header: 'bg-sky-50 dark:bg-sky-900/20 border-sky-100 dark:border-sky-800/50',
        dot: 'bg-sky-500',
      }
  }
})

async function copyUrl() {
  if (!url.value) return
  try {
    await navigator.clipboard.writeText(url.value)
    copiedUrl.value = true
    setTimeout(() => { copiedUrl.value = false }, 1600)
  } catch {
    console.error('Failed to copy URL')
  }
}

async function copyContent() {
  if (!content.value) return
  try {
    await navigator.clipboard.writeText(content.value)
    copiedContent.value = true
    setTimeout(() => { copiedContent.value = false }, 1600)
  } catch {
    console.error('Failed to copy content')
  }
}

function isActionActive(actionId: string): boolean {
  return props.actionLoading === true && props.activeActionId === actionId
}

function isActionDisabled(action: { disabled?: boolean }): boolean {
  return props.actionLoading === true || action.disabled === true
}

function actionButtonLabel(action: { id: string; label: string }): string {
  return isActionActive(action.id) ? 'Working...' : action.label
}

function triggerAction(actionId: string, disabled = false) {
  if (props.actionLoading || disabled) return
  emit('action', actionId, props.card.id)
}

function actionButtonClasses(variant?: ActionButton['variant']): string {
  switch (variant) {
    case 'danger':
      return 'bg-red-600 text-white border-red-600 hover:bg-red-700'
    case 'secondary':
      return 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'
    default:
      return 'bg-gray-900 text-white border-gray-900 hover:bg-black dark:bg-gray-100 dark:text-gray-900 dark:border-gray-100 dark:hover:bg-white'
  }
}
</script>

<template>
  <div class="rounded-xl border overflow-hidden bg-white dark:bg-gray-800 shadow-sm" :class="toneClasses.border">
    <div class="px-4 py-3 border-b" :class="toneClasses.header">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <span class="inline-flex h-2.5 w-2.5 rounded-full flex-shrink-0" :class="toneClasses.dot" />
            <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">
              {{ title }}
            </h3>
            <span
              v-if="warningCodeLabel"
              class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200"
            >
              {{ warningCodeLabel }}
            </span>
            <span
              v-if="card._streaming"
              class="inline-flex h-2 w-2 rounded-full bg-gray-400 animate-pulse flex-shrink-0"
            />
          </div>

          <div class="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-gray-500 dark:text-gray-400">
            <span v-if="hostname" class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5">{{ hostname }}</span>
            <span v-if="card.extract_mode" class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5">{{ card.extract_mode }}</span>
            <span v-if="card.extractor" class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5">{{ card.extractor }}</span>
            <span v-if="card.content_type" class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5">{{ card.content_type }}</span>
            <span v-if="card.truncated" class="rounded-full bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200 px-2 py-0.5">truncated</span>
          </div>

          <a
            v-if="url"
            :href="url"
            target="_blank"
            rel="noopener noreferrer"
            class="mt-2 block text-xs text-blue-600 dark:text-blue-400 truncate hover:underline"
          >
            {{ url }}
          </a>
        </div>

        <div class="flex items-center gap-2 flex-shrink-0">
          <button
            v-if="useBrowserAction"
            class="rounded-md border px-2.5 py-1 text-xs font-medium transition-colors disabled:opacity-60 disabled:cursor-wait"
            :class="[actionButtonClasses(useBrowserAction.variant), { 'opacity-60 cursor-wait': actionLoading }]"
            :disabled="isActionDisabled(useBrowserAction)"
            :aria-busy="isActionActive(useBrowserAction.id) ? 'true' : undefined"
            @click="triggerAction(useBrowserAction.id, !!useBrowserAction.disabled)"
          >
            <span
              v-if="isActionActive(useBrowserAction.id)"
              class="mr-1 inline-block h-3 w-3 animate-spin rounded-full border border-current border-r-transparent align-[-2px]"
            />
            {{ actionButtonLabel(useBrowserAction) }}
          </button>
          <button
            v-if="url"
            class="rounded-md border border-gray-200 dark:border-gray-700 px-2.5 py-1 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
            @click="copyUrl"
          >
            {{ copiedUrl ? 'Copied' : 'Copy URL' }}
          </button>
          <button
            v-if="content"
            class="rounded-md border border-gray-200 dark:border-gray-700 px-2.5 py-1 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
            @click="copyContent"
          >
            {{ copiedContent ? 'Copied' : 'Copy text' }}
          </button>
        </div>
      </div>
    </div>

    <div class="px-4 py-3">
      <div
        v-if="warning || warningCodeLabel"
        class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 dark:border-amber-800/60 dark:bg-amber-900/20"
      >
        <div class="text-xs font-semibold uppercase tracking-wide text-amber-800 dark:text-amber-200">
          {{ warningCodeLabel || 'Warning' }}
        </div>
        <p v-if="warning" class="mt-1 text-sm leading-relaxed text-amber-900 dark:text-amber-100">
          {{ warning }}
        </p>
      </div>

      <div v-if="content" class="mt-3">
        <div v-if="isMarkdown" class="relative">
          <div
            class="prose prose-sm dark:prose-invert max-w-none rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 overflow-hidden"
            :class="{ 'max-h-96': shouldCollapse && !expanded }"
            v-html="renderedMarkdown"
          />
          <div
            v-if="shouldCollapse && !expanded"
            class="pointer-events-none absolute inset-x-0 bottom-0 h-20 rounded-b-xl bg-gradient-to-t from-white dark:from-gray-800 to-transparent"
          />
        </div>
        <div v-else class="rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/40 overflow-hidden">
          <pre class="px-4 py-3 text-sm leading-6 text-gray-700 dark:text-gray-200 whitespace-pre-wrap break-words">{{ visibleTextContent }}</pre>
        </div>

        <button
          v-if="shouldCollapse"
          class="mt-2 text-xs font-medium text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
          @click="expanded = !expanded"
        >
          {{ expanded ? 'Show less' : 'Show more' }}
        </button>
      </div>

      <div v-else class="mt-3 rounded-xl border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
        No extracted content
      </div>
    </div>
  </div>
</template>
