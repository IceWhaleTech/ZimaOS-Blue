<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ActionButton, TypelessCardWebFetch } from '@/types/typeless'
import { renderMarkdown } from '@/utils/markdown'
import { formatToolWarningCodeLabel } from '@/utils/toolWarnings'
import { translateCardActionLabel } from '@/utils/cardActionLabels'
import { usePersistentDisclosureState } from '@/utils/chatCardUiState'

const { t, te } = useI18n()

const props = defineProps<{
  card: TypelessCardWebFetch
  actionLoading?: boolean
  activeActionId?: string
  uiStateKey?: string
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

function unescapeBackticks(s: string): string {
  return s.replace(/`​``/g, '```')
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
  return hostFromUrl(props.card.url) || t('webFetchCard.title', 'Web fetch')
})
const url = computed(() => unescapeBackticks((props.card.url || '').trim()))
const content = computed(() => unescapeBackticks(props.card.content || ''))
const warning = computed(() => unescapeBackticks((props.card.warning || '').trim()))
const warningCodeLabel = computed(() => formatToolWarningCodeLabel(props.card.warning_code, t))
const hostname = computed(() => hostFromUrl(url.value))
const isMarkdown = computed(() => (props.card.extract_mode || '').toLowerCase() === 'markdown')
const useBrowserAction = computed<ActionButton | null>(() => {
  const configured = props.card.actions?.find((action) => action.id === 'use_browser')
  if (configured) return configured
  if (!url.value) return null
  return {
    id: 'use_browser',
    label: t('webFetchCard.actions.use_browser', 'Use browser'),
    variant: 'primary',
  }
})
const renderedMarkdown = computed(() =>
  isMarkdown.value && content.value ? renderMarkdown(content.value) : ''
)
const summaryKey = computed(() => props.uiStateKey || props.card.id || `web-fetch:${url.value}`)
const { expanded, toggleExpanded } = usePersistentDisclosureState(summaryKey, false)
const copiedUrl = ref(false)
const copiedContent = ref(false)
const hasContent = computed(() => Boolean(content.value))

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
    setTimeout(() => {
      copiedUrl.value = false
    }, 1600)
  } catch {
    console.error('Failed to copy URL')
  }
}

async function copyContent() {
  if (!content.value) return
  try {
    await navigator.clipboard.writeText(content.value)
    copiedContent.value = true
    setTimeout(() => {
      copiedContent.value = false
    }, 1600)
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
  if (isActionActive(action.id)) return t('common.processing', 'Processing...')
  return translateCardActionLabel({
    id: action.id,
    fallback: action.label,
    t,
    te,
    scopes: ['webFetchCard.actions'],
  })
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
  <div
    class="rounded-xl border overflow-hidden bg-white dark:bg-gray-800 shadow-sm"
    :class="toneClasses.border"
  >
    <button
      class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/30"
      :aria-expanded="expanded ? 'true' : 'false'"
      @click="toggleExpanded"
    >
      <span
        class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-sky-100 text-sky-600 dark:bg-sky-900/40 dark:text-sky-300"
      >
        <svg
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
            d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5A3.375 3.375 0 0010.125 2.25H8.25m0 12.75h7.5m-7.5 3h4.5M6.375 3h3.75A2.25 2.25 0 0112.375 5.25V7.5a2.25 2.25 0 002.25 2.25h2.25a2.25 2.25 0 012.25 2.25v6.75A2.25 2.25 0 0116.875 21H6.375a2.25 2.25 0 01-2.25-2.25V5.25A2.25 2.25 0 016.375 3z"
          />
        </svg>
      </span>
      <span class="min-w-0 flex-1">
        <span
          class="block text-[11px] font-semibold uppercase tracking-[0.08em] text-gray-500 dark:text-gray-400"
          >{{ t('webFetchCard.title', 'Web fetch') }}</span
        >
        <span class="flex min-w-0 items-center gap-2">
          <span
            class="inline-flex h-2.5 w-2.5 rounded-full flex-shrink-0"
            :class="toneClasses.dot"
          />
          <span class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
            {{ title }}
          </span>
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
        </span>
        <span
          v-if="hostname || url"
          class="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400"
        >
          {{ hostname || url }}
        </span>
      </span>
      <span class="flex flex-shrink-0 items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span
          v-if="card.extract_mode"
          class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5"
          >{{ card.extract_mode }}</span
        >
        <span
          v-if="card.truncated"
          class="rounded-full bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200 px-2 py-0.5"
          >{{ t('execCard.outputTruncated', 'truncated') }}</span
        >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-gray-400 transition-transform duration-200"
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
      </span>
    </button>

    <transition name="web-fetch-content">
      <div v-if="expanded" class="border-t border-gray-100 dark:border-gray-700/60">
        <div class="px-4 py-3 border-b" :class="toneClasses.header">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2 text-[11px] text-gray-500 dark:text-gray-400">
                <span
                  v-if="hostname"
                  class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5"
                  >{{ hostname }}</span
                >
                <span
                  v-if="card.extract_mode"
                  class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5"
                  >{{ card.extract_mode }}</span
                >
                <span
                  v-if="card.extractor"
                  class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5"
                  >{{ card.extractor }}</span
                >
                <span
                  v-if="card.content_type"
                  class="rounded-full bg-black/5 dark:bg-white/10 px-2 py-0.5"
                  >{{ card.content_type }}</span
                >
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
                :class="[
                  actionButtonClasses(useBrowserAction.variant),
                  { 'opacity-60 cursor-wait': actionLoading },
                ]"
                :disabled="isActionDisabled(useBrowserAction)"
                :aria-busy="isActionActive(useBrowserAction.id) ? 'true' : undefined"
                @click.stop="triggerAction(useBrowserAction.id, !!useBrowserAction.disabled)"
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
                @click.stop="copyUrl"
              >
                {{
                  copiedUrl ? t('common.copied', 'Copied') : t('webFetchCard.copyUrl', 'Copy URL')
                }}
              </button>
              <button
                v-if="content"
                class="rounded-md border border-gray-200 dark:border-gray-700 px-2.5 py-1 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
                @click.stop="copyContent"
              >
                {{
                  copiedContent
                    ? t('common.copied', 'Copied')
                    : t('webFetchCard.copyText', 'Copy text')
                }}
              </button>
            </div>
          </div>
        </div>

        <div class="px-4 py-3">
          <div
            v-if="warning || warningCodeLabel"
            class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 dark:border-amber-800/60 dark:bg-amber-900/20"
          >
            <div
              class="text-xs font-semibold uppercase tracking-wide text-amber-800 dark:text-amber-200"
            >
              {{ warningCodeLabel || t('toolWarnings.warning', 'Warning') }}
            </div>
            <p
              v-if="warning"
              class="mt-1 text-sm leading-relaxed text-amber-900 dark:text-amber-100"
            >
              {{ warning }}
            </p>
          </div>

          <div
            v-if="hasContent"
            class="mt-3 rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/40 overflow-hidden"
          >
            <div
              v-if="isMarkdown"
              class="prose prose-sm dark:prose-invert max-w-none px-4 py-3"
              v-html="renderedMarkdown"
            />
            <pre
              v-else
              class="px-4 py-3 text-sm leading-6 text-gray-700 dark:text-gray-200 whitespace-pre-wrap break-words"
              >{{ content }}</pre
            >
          </div>

          <div
            v-else
            class="mt-3 rounded-xl border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400"
          >
            {{ t('webFetchCard.noContent', 'No extracted content') }}
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.web-fetch-content-enter-active,
.web-fetch-content-leave-active {
  overflow: hidden;
  transition:
    max-height 220ms ease,
    opacity 180ms ease,
    transform 180ms ease;
}

.web-fetch-content-enter-from,
.web-fetch-content-leave-to {
  max-height: 0;
  opacity: 0;
  transform: translateY(-4px);
}

.web-fetch-content-enter-to,
.web-fetch-content-leave-from {
  max-height: 960px;
  opacity: 1;
  transform: translateY(0);
}
</style>
