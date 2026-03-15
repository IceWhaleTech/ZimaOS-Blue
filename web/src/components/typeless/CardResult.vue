<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardResult } from '@/types/typeless'
import { formatToolWarningCodeLabel } from '@/utils/toolWarnings'
import { translateCardActionLabel } from '@/utils/cardActionLabels'
import { useTauri } from '@/composables/useTauri'
import { isApiPath, isHttpUrl, isLocalAbsolutePath } from '@/utils/localPath'

const { t, te } = useI18n()
const { openInBrowser } = useTauri()

const props = defineProps<{
  card: TypelessCardResult
  actionLoading?: boolean
  activeActionId?: string
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

const copiedIndex = ref<number | null>(null)
const titleCopied = ref(false)

// Title is explicitly marked as copyable by the backend (e.g. exec command)
const isTitleCopyable = computed(() => !!(props.card as any).title_copyable)

const statusConfig = {
  success: {
    headerBg: 'bg-emerald-50 dark:bg-emerald-900/20',
    headerBorder: 'border-emerald-100 dark:border-emerald-800/50',
    border: 'border-emerald-200 dark:border-emerald-800/60',
    icon: '✓',
    iconBg: 'bg-emerald-100 dark:bg-emerald-900/40',
    iconColor: 'text-emerald-600 dark:text-emerald-400',
  },
  error: {
    headerBg: 'bg-red-50 dark:bg-red-900/20',
    headerBorder: 'border-red-100 dark:border-red-800/50',
    border: 'border-red-200 dark:border-red-800/60',
    icon: '✗',
    iconBg: 'bg-red-100 dark:bg-red-900/40',
    iconColor: 'text-red-600 dark:text-red-400',
  },
  warning: {
    headerBg: 'bg-amber-50 dark:bg-amber-900/20',
    headerBorder: 'border-amber-100 dark:border-amber-800/50',
    border: 'border-amber-200 dark:border-amber-800/60',
    icon: '⚠',
    iconBg: 'bg-amber-100 dark:bg-amber-900/40',
    iconColor: 'text-amber-600 dark:text-amber-400',
  },
  info: {
    headerBg: 'bg-blue-50 dark:bg-blue-900/20',
    headerBorder: 'border-blue-100 dark:border-blue-800/50',
    border: 'border-blue-200 dark:border-blue-800/60',
    icon: 'ℹ',
    iconBg: 'bg-blue-100 dark:bg-blue-900/40',
    iconColor: 'text-blue-600 dark:text-blue-400',
  },
} as const

type StatusKey = keyof typeof statusConfig

const cardStatus = computed<StatusKey>(() => {
  const s = props.card.status
  return s && s in statusConfig ? (s as StatusKey) : 'info'
})

const buttonClasses = {
  primary: 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white',
  secondary:
    'bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300',
  danger: 'bg-red-500 hover:bg-red-600 text-white',
}

/** Check if a value is a plain object (map) */
function isMapValue(val: unknown): val is Record<string, unknown> {
  return val !== null && typeof val === 'object' && !Array.isArray(val)
}

/** Try to parse a string as JSON object; returns the object or null */
function tryParseObject(val: unknown): Record<string, unknown> | null {
  if (isMapValue(val)) return val
  if (typeof val === 'string' && val.startsWith('{')) {
    try {
      const o = JSON.parse(val)
      if (isMapValue(o)) return o
    } catch {
      /* not JSON */
    }
  }
  return null
}

/** Flatten a value to a copyable string */
function toDisplayString(val: unknown): string {
  if (typeof val === 'string') return val
  if (val === null || val === undefined) return ''
  return JSON.stringify(val, null, 2)
}

async function copyValue(value: unknown, index: number) {
  try {
    await navigator.clipboard.writeText(toDisplayString(value))
    copiedIndex.value = index
    setTimeout(() => {
      copiedIndex.value = null
    }, 2000)
  } catch {
    console.error('Failed to copy to clipboard')
  }
}

async function copyTitle() {
  try {
    await navigator.clipboard.writeText(props.card.title)
    titleCopied.value = true
    setTimeout(() => {
      titleCopied.value = false
    }, 2000)
  } catch {
    console.error('Failed to copy title')
  }
}

function isActionActive(actionId: string): boolean {
  return props.actionLoading === true && props.activeActionId === actionId
}

function isActionDisabled(action: { disabled?: boolean }): boolean {
  return props.actionLoading === true || action.disabled === true
}

function actionButtonLabel(action: { id: string; label: string }): string {
  return isActionActive(action.id)
    ? t('common.processing', 'Processing...')
    : tAction(action.id, action.label)
}

function handleAction(actionId: string, disabled = false) {
  if (props.actionLoading || disabled) return
  emit('action', actionId, props.card.id)
}

function normalizeResultCardKey(input: string): string {
  return input
    .toLowerCase()
    .trim()
    .replace(/[\s-]+/g, '_')
    .replace(/[^a-z0-9_]+/g, '')
    .replace(/_+/g, '_')
    .replace(/^_+|_+$/g, '')
}

function tLabel(label: string): string {
  const key = 'resultCard.labels.' + normalizeResultCardKey(label)
  return te(key) ? t(key) : label
}

function tDetailValue(label: string, value: unknown): string {
  if (typeof value === 'boolean') {
    return value ? t('common.yes', 'Yes') : t('common.no', 'No')
  }

  const rawValue = typeof value === 'string' ? value.trim() : toDisplayString(value)
  if (!rawValue) return ''

  const lowered = rawValue.toLowerCase()
  if (lowered === 'true') return t('common.yes', 'Yes')
  if (lowered === 'false') return t('common.no', 'No')

  const normalizedLabel = normalizeResultCardKey(label)
  const normalizedValue = normalizeResultCardKey(rawValue)
  if (!normalizedValue) return rawValue

  const scopedKey = `resultCard.values.${normalizedLabel}.${normalizedValue}`
  if (te(scopedKey)) return t(scopedKey, rawValue)

  const genericKey = `resultCard.values.${normalizedValue}`
  if (te(genericKey)) return t(genericKey, rawValue)

  return rawValue
}

function tAction(id: string, fallback: string): string {
  return translateCardActionLabel({
    id,
    fallback,
    t,
    te,
    scopes: ['resultCard.actions'],
  })
}

async function handleRevealLocation(value: unknown) {
  if (typeof value !== 'string') return
  const path = value.trim()
  if (!isLocalAbsolutePath(path)) return
  await openInBrowser(path)
}

const translatedTitle = computed(() => {
  if (!props.card.title) return ''
  const rawTitle = props.card.title.trim()
  const normalizedTitle = rawTitle.toLowerCase().replace(/\s+/g, '_')
  const toolKeys = [`tools.names.${rawTitle}`, `tools.names.${normalizedTitle}`]
  for (const key of toolKeys) {
    if (te(key)) return t(key)
  }
  const key = 'resultCard.titles.' + normalizedTitle
  const translated = t(key, rawTitle)
  return translated === key ? rawTitle : translated
})

const errorKeyMap: [RegExp, string][] = [
  [/browser start failed/i, 'uiReview.errors.browserStartFailed'],
  [/browser service not available/i, 'uiReview.errors.browserNotAvailable'],
  [/navigation failed/i, 'uiReview.errors.navigationFailed'],
  [/proxy bridge not available/i, 'uiReview.errors.vlmNotAvailable'],
]

const translatedMessage = computed(() => {
  const msg = props.card.message
  if (!msg) return ''
  if (props.card.status === 'error') {
    for (const [re, key] of errorKeyMap) {
      if (re.test(msg)) return t(key, msg)
    }
  }
  const normalizedMessage = msg
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
  const key = 'resultCard.messages.' + normalizedMessage
  return te(key) ? t(key) : msg
})

const warningText = computed(() => (props.card.warning || '').trim())
const warningCodeLabel = computed(() => formatToolWarningCodeLabel(props.card.warning_code, t))
const resolvedImageSrc = computed(() => {
  const raw = (props.card.image || '').trim()
  if (!raw) return ''
  if (
    raw.startsWith('data:image/') ||
    raw.startsWith('http://') ||
    raw.startsWith('https://') ||
    raw.startsWith('/')
  ) {
    return raw
  }
  return `data:image/png;base64,${raw}`
})

// Filter out details that are redundant with the title/message
const visibleDetails = computed(() => {
  if (!props.card.details) return []
  return props.card.details
    .filter((d) => {
      const lbl = d.label.toLowerCase()
      if (lbl === 'status' || lbl === '状态') return false
      if (lbl === 'warning' || lbl === 'warning_code') return false
      if ((lbl === 'result' || lbl === '结果') && props.card.message) return false
      return true
    })
    .map((d) => ({
      ...d,
      parsedObject: tryParseObject(d.value),
      isMultiline: d.multiline || (typeof d.value === 'string' && d.value.includes('\n')),
      isLink: typeof d.value === 'string' && (isHttpUrl(d.value) || isApiPath(d.value)),
      isLocalPath: typeof d.value === 'string' && isLocalAbsolutePath(d.value),
    }))
})
</script>

<template>
  <div
    class="rounded-lg border overflow-hidden bg-white dark:bg-gray-800 shadow-sm"
    :class="statusConfig[cardStatus].border"
  >
    <!-- Header -->
    <div
      class="flex items-center gap-2.5 px-4 py-2.5 border-b group/header"
      :class="[statusConfig[cardStatus].headerBg, statusConfig[cardStatus].headerBorder]"
    >
      <span
        class="w-5 h-5 rounded-full flex items-center justify-center text-xs flex-shrink-0"
        :class="[statusConfig[cardStatus].iconBg, statusConfig[cardStatus].iconColor]"
      >
        {{ statusConfig[cardStatus].icon }}
      </span>
      <span
        class="text-sm font-medium text-gray-800 dark:text-gray-100 truncate flex-1"
        :class="{ 'font-mono': isTitleCopyable }"
      >
        {{ translatedTitle }}
      </span>
      <span
        v-if="warningCodeLabel"
        class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200"
      >
        {{ warningCodeLabel }}
      </span>
      <button
        v-if="isTitleCopyable"
        class="p-1 rounded opacity-0 group-hover/header:opacity-100 hover:bg-black/10 dark:hover:bg-white/10 transition-all flex-shrink-0"
        :title="titleCopied ? t('resultCard.copied', 'Copied!') : t('resultCard.copy', 'Copy')"
        @click="copyTitle"
      >
        <svg
          v-if="!titleCopied"
          xmlns="http://www.w3.org/2000/svg"
          class="h-3.5 w-3.5 text-gray-400"
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
          class="h-3.5 w-3.5 text-emerald-500"
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

    <!-- Body -->
    <div class="px-4 py-3">
      <div
        v-if="resolvedImageSrc"
        class="rounded-md overflow-hidden border border-gray-200 dark:border-gray-700/60 bg-gray-50 dark:bg-gray-900/60"
      >
        <img
          :src="resolvedImageSrc"
          :alt="translatedTitle || 'image'"
          class="w-full max-h-[22rem] object-contain"
          loading="lazy"
        />
      </div>

      <!-- Message -->
      <p
        v-if="card.message"
        class="text-sm text-gray-600 dark:text-gray-300 leading-relaxed"
        :class="resolvedImageSrc ? 'mt-3' : ''"
      >
        {{ translatedMessage }}
      </p>

      <div
        v-if="warningText || warningCodeLabel"
        class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 dark:border-amber-800/60 dark:bg-amber-900/20"
        :class="card.message ? 'mt-3' : ''"
      >
        <div
          class="flex items-center gap-2 text-xs font-semibold text-amber-800 dark:text-amber-200"
        >
          <span>{{ warningCodeLabel || t('toolWarnings.warning', 'Warning') }}</span>
        </div>
        <p
          v-if="warningText"
          class="mt-1 text-sm leading-relaxed text-amber-900 dark:text-amber-100"
        >
          {{ warningText }}
        </p>
      </div>

      <!-- Details -->
      <div v-if="visibleDetails.length > 0" class="mt-2.5">
        <div
          class="rounded-md bg-gray-50 dark:bg-gray-900/40 divide-y divide-gray-100 dark:divide-gray-700/50"
        >
          <div
            v-for="(detail, index) in visibleDetails"
            :key="index"
            class="px-3.5 py-2.5 text-sm group"
            :class="
              detail.parsedObject || detail.isMultiline
                ? 'flex flex-col gap-1.5'
                : 'flex items-center justify-between'
            "
          >
            <span class="text-gray-400 dark:text-gray-500 text-xs flex-shrink-0">{{
              tLabel(detail.label)
            }}</span>
            <!-- Nested table for map/object values -->
            <div
              v-if="detail.parsedObject"
              class="rounded border border-gray-200 dark:border-gray-700/60 bg-white dark:bg-gray-800/60 divide-y divide-gray-100 dark:divide-gray-700/40 overflow-hidden"
            >
              <div
                v-for="(subVal, subKey) in detail.parsedObject"
                :key="String(subKey)"
                class="flex items-center justify-between px-3 py-1.5 text-xs"
              >
                <span class="text-gray-400 dark:text-gray-500">{{ tLabel(String(subKey)) }}</span>
                <span
                  class="text-gray-700 dark:text-gray-300 font-mono text-right max-w-[70%] break-all"
                  >{{ toDisplayString(subVal) }}</span
                >
              </div>
            </div>
            <!-- Multiline text value (e.g. stdout) -->
            <div
              v-else-if="detail.isMultiline"
              class="rounded border border-gray-200 dark:border-gray-700/60 bg-gray-50 dark:bg-gray-900/60 overflow-hidden"
            >
              <pre
                class="px-3 py-2 text-xs text-gray-700 dark:text-gray-300 font-mono whitespace-pre-wrap break-all overflow-x-auto max-h-64 overflow-y-auto leading-relaxed"
                >{{ detail.value }}</pre
              >
            </div>
            <!-- Link value -->
            <div v-else-if="detail.isLink" class="flex items-center gap-1.5">
              <a
                :href="String(detail.value)"
                target="_blank"
                rel="noopener noreferrer"
                class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                >{{ t('resultCard.openLink', 'Open') }} ↗</a
              >
            </div>
            <div v-else-if="detail.isLocalPath" class="flex items-center gap-1.5">
              <button
                class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                @click="handleRevealLocation(detail.value)"
              >
                {{ t('common.openLocation', 'Open location') }}
              </button>
            </div>
            <!-- Simple string value -->
            <div v-else class="flex items-center gap-1.5">
              <span class="text-gray-700 dark:text-gray-300 font-mono text-xs"
                >{{ tDetailValue(detail.label, detail.value)
                }}<template v-if="detail.suffix"> {{ tLabel(detail.suffix) }}</template></span
              >
              <button
                v-if="detail.copyable"
                class="p-0.5 rounded opacity-0 group-hover:opacity-100 hover:bg-gray-200 dark:hover:bg-gray-700 transition-all"
                :title="
                  copiedIndex === index
                    ? t('resultCard.copied', 'Copied!')
                    : t('resultCard.copy', 'Copy')
                "
                @click="copyValue(detail.value, index)"
              >
                <svg
                  v-if="copiedIndex !== index"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-3.5 w-3.5 text-gray-400"
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
                  class="h-3.5 w-3.5 text-emerald-500"
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
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div v-if="card.actions && card.actions.length > 0" class="mt-3 flex flex-wrap gap-2">
        <button
          v-for="action in card.actions"
          :key="action.id"
          class="px-3 py-1.5 rounded-md text-xs font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          :class="[
            buttonClasses[action.variant || 'secondary'],
            { 'opacity-60 cursor-wait': actionLoading },
          ]"
          :disabled="isActionDisabled(action)"
          :aria-busy="isActionActive(action.id) ? 'true' : undefined"
          @click="handleAction(action.id, !!action.disabled)"
        >
          <span
            v-if="isActionActive(action.id)"
            class="mr-1 inline-block h-3 w-3 animate-spin rounded-full border border-current border-r-transparent align-[-2px]"
          />
          <span v-else-if="action.icon" class="mr-1">{{ action.icon }}</span>
          {{ actionButtonLabel(action) }}
        </button>
      </div>
    </div>
  </div>
</template>
