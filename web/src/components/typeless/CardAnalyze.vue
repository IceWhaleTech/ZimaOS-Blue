<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TypelessCardAnalyze } from '@/types/typeless'

const { t, te } = useI18n()

const props = defineProps<{
  card: TypelessCardAnalyze
}>()

const title = computed(
  () => props.card.title || props.card.topic || t('analyze.title', 'Analysis Report')
)

const summary = computed(() => {
  const answer = typeof props.card.answer === 'string' ? props.card.answer.trim() : ''
  if (answer) return answer
  const message = typeof props.card.message === 'string' ? props.card.message.trim() : ''
  return message
})

const outputMode = computed(() => {
  const raw = typeof props.card.output_mode === 'string' ? props.card.output_mode.trim() : ''
  if (raw) return raw
  return props.card.report_url ? 'report' : 'inline'
})

const modeLabel = computed(() => {
  const key = `analyze.outputMode.${outputMode.value}`
  if (te(key)) return t(key)
  if (outputMode.value === 'report') return t('analyze.outputMode.report', 'Report output')
  return t('analyze.outputMode.inline', 'Inline answer')
})

const hasReportLink = computed(
  () => typeof props.card.report_url === 'string' && props.card.report_url.trim().length > 0
)
const reportStyle = computed(() =>
  typeof props.card.report_style === 'string' ? props.card.report_style.trim() : ''
)
const isError = computed(() => props.card.status === 'error')
</script>

<template>
  <div
    class="overflow-hidden rounded-xl border bg-white shadow-sm dark:bg-slate-900"
    :class="
      isError
        ? 'border-red-200 dark:border-red-800/60'
        : 'border-slate-200 dark:border-slate-700/70'
    "
  >
    <div
      class="flex items-start gap-3 border-b px-4 py-3"
      :class="
        isError
          ? 'border-red-100 bg-red-50/80 dark:border-red-800/40 dark:bg-red-950/20'
          : 'border-slate-100 bg-slate-50/90 dark:border-slate-700/50 dark:bg-slate-800/60'
      "
    >
      <div
        class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl text-sm font-semibold"
        :class="
          isError
            ? 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-300'
            : 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
        "
      >
        {{ isError ? '!' : 'AI' }}
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-2">
          <h3 class="truncate text-sm font-semibold text-slate-900 dark:text-slate-50">
            {{ title }}
          </h3>
          <span
            class="rounded-full px-2 py-0.5 text-[11px] font-medium"
            :class="
              isError
                ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200'
                : 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-200'
            "
          >
            {{ modeLabel }}
          </span>
          <span
            v-if="reportStyle"
            class="rounded-full bg-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-700 dark:bg-slate-700 dark:text-slate-200"
          >
            {{ t('analyze.reportStyle', 'Style') }} · {{ reportStyle }}
          </span>
        </div>
        <p
          v-if="summary"
          class="mt-2 whitespace-pre-wrap text-sm leading-6 text-slate-600 dark:text-slate-300"
        >
          {{ summary }}
        </p>
      </div>
    </div>

    <div
      v-if="hasReportLink"
      class="px-4 py-3"
    >
      <a
        :href="card.report_url"
        class="inline-flex items-center gap-2 text-sm font-medium text-indigo-600 underline-offset-4 hover:underline dark:text-indigo-300"
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
            stroke-width="1.8"
            d="M13.5 4.75H7.75A1.75 1.75 0 006 6.5v11A1.75 1.75 0 007.75 19.25h8.5A1.75 1.75 0 0018 17.5V9.25M13.5 4.75l4.5 4.5M13.5 4.75V9.5H18"
          />
        </svg>
        {{ t('analyze.reportLink', 'Open report') }}
      </a>
    </div>
  </div>
</template>
