<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardResult } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardResult
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

const copiedIndex = ref<number | null>(null)

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
}

const buttonClasses = {
  primary: 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white',
  secondary: 'bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300',
  danger: 'bg-red-500 hover:bg-red-600 text-white',
}

async function copyValue(value: string, index: number) {
  try {
    await navigator.clipboard.writeText(value)
    copiedIndex.value = index
    setTimeout(() => {
      copiedIndex.value = null
    }, 2000)
  } catch {
    console.error('Failed to copy to clipboard')
  }
}

function handleAction(actionId: string) {
  emit('action', actionId, props.card.id)
}

function tLabel(label: string): string {
  const key = 'resultCard.labels.' + label.toLowerCase().replace(/\s+/g, '_')
  const translated = t(key, label)
  return translated === key ? label : translated
}

function tAction(id: string, fallback: string): string {
  const key = 'resultCard.actions.' + id
  const translated = t(key, fallback)
  return translated === key ? fallback : translated
}

const translatedTitle = computed(() => {
  if (!props.card.title) return ''
  const key = 'resultCard.titles.' + props.card.title.toLowerCase().replace(/\s+/g, '_')
  const translated = t(key, props.card.title)
  return translated === key ? props.card.title : translated
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
  return msg
})

// Filter out details that are redundant with the title/message
const visibleDetails = computed(() => {
  if (!props.card.details) return []
  return props.card.details.filter(d => {
    // Hide status/result fields that just echo the card status
    const lbl = d.label.toLowerCase()
    if (lbl === 'status' || lbl === '状态') return false
    if ((lbl === 'result' || lbl === '结果') && props.card.message) return false
    return true
  })
})
</script>

<template>
  <div
    class="rounded-lg border overflow-hidden bg-white dark:bg-gray-800 shadow-sm"
    :class="statusConfig[card.status].border"
  >
    <!-- Header -->
    <div
      class="flex items-center gap-2.5 px-4 py-2.5 border-b"
      :class="[statusConfig[card.status].headerBg, statusConfig[card.status].headerBorder]"
    >
      <span
        class="w-5 h-5 rounded-full flex items-center justify-center text-xs flex-shrink-0"
        :class="[statusConfig[card.status].iconBg, statusConfig[card.status].iconColor]"
      >
        {{ statusConfig[card.status].icon }}
      </span>
      <span class="text-sm font-medium text-gray-800 dark:text-gray-100 truncate flex-1">
        {{ translatedTitle }}
      </span>
    </div>

    <!-- Body -->
    <div class="px-4 py-3">
      <!-- Message -->
      <p
        v-if="card.message"
        class="text-sm text-gray-600 dark:text-gray-300 leading-relaxed"
      >
        {{ translatedMessage }}
      </p>

      <!-- Details -->
      <div v-if="visibleDetails.length > 0" class="mt-2.5">
        <div class="rounded-md bg-gray-50 dark:bg-gray-900/40 divide-y divide-gray-100 dark:divide-gray-700/50">
          <div
            v-for="(detail, index) in visibleDetails"
            :key="index"
            class="flex items-center justify-between px-3 py-2 text-sm group"
          >
            <span class="text-gray-400 dark:text-gray-500 text-xs">{{ tLabel(detail.label) }}</span>
            <div class="flex items-center gap-1.5">
              <span class="text-gray-700 dark:text-gray-300 font-mono text-xs">{{ detail.value }}<template v-if="detail.suffix"> {{ tLabel(detail.suffix) }}</template></span>
              <button
                v-if="detail.copyable"
                class="p-0.5 rounded opacity-0 group-hover:opacity-100 hover:bg-gray-200 dark:hover:bg-gray-700 transition-all"
                :title="copiedIndex === index ? t('resultCard.copied', 'Copied!') : t('resultCard.copy', 'Copy')"
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
          :class="buttonClasses[action.variant || 'secondary']"
          :disabled="action.disabled"
          @click="handleAction(action.id)"
        >
          <span v-if="action.icon" class="mr-1">{{ action.icon }}</span>
          {{ tAction(action.id, action.label) }}
        </button>
      </div>
    </div>
  </div>
</template>
