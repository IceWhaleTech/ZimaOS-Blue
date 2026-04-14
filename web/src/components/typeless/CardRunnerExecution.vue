<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardRunnerExecution } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardRunnerExecution
}>()

const transcriptExpanded = ref(false)
const transcriptPreviewCount = 2

watch(
  () => props.card.id,
  () => {
    transcriptExpanded.value = false
  }
)

function normalizeText(value: unknown): string {
  if (typeof value !== 'string') return ''
  return value.trim()
}

function formatDurationMs(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return ''
  if (value < 1000) return `${Math.round(value)}ms`
  return `${(value / 1000).toFixed(value >= 10_000 ? 0 : 1)}s`
}

const eyebrow = computed(
  () => normalizeText(props.card.eyebrow) || t('settings.agentcoreRunner.eyebrow', 'Harness · Beta')
)
const title = computed(
  () =>
    normalizeText(props.card.title) ||
    t('settings.agentcoreRunner.title', 'Harness Self-Iterating Agentcore Runner')
)
const responseText = computed(() => normalizeText(props.card.runner_response_text))
const runnerError = computed(() => normalizeText(props.card.runner_error))
const runnerStderr = computed(() => normalizeText(props.card.runner_stderr))

const metaItems = computed(() =>
  [
    normalizeText(props.card.reason),
    normalizeText(props.card.candidate_id),
    normalizeText(props.card.eval_run_id),
    normalizeText(props.card.runner_protocol),
    normalizeText(props.card.runner_stop_reason),
    normalizeText(props.card.optimization_surface),
    formatDurationMs(props.card.runner_duration_ms),
  ].filter(Boolean)
)

const transcriptEntries = computed(() => {
  const entries = props.card.runner_transcript ?? []
  const suppressed = new Set([responseText.value, runnerError.value].filter(Boolean))
  const seen = new Set<string>()

  return entries.filter((entry) => {
    const text = normalizeText(entry.text)
    if (!text || suppressed.has(text)) return false

    const fingerprint = [
      normalizeText(entry.direction),
      normalizeText(entry.method),
      text,
    ].join('::')

    if (seen.has(fingerprint)) return false
    seen.add(fingerprint)
    return true
  })
})

const visibleTranscriptEntries = computed(() => {
  if (transcriptExpanded.value) return transcriptEntries.value
  return transcriptEntries.value.slice(0, transcriptPreviewCount)
})

const hiddenTranscriptCount = computed(() =>
  Math.max(transcriptEntries.value.length - transcriptPreviewCount, 0)
)
</script>

<template>
  <article
    data-testid="chat-runner-execution-card"
    class="overflow-hidden rounded-[1.35rem] border border-slate-200/80 bg-white/95 shadow-sm backdrop-blur dark:border-slate-700/70 dark:bg-slate-900/80"
  >
    <header
      class="border-b border-slate-200/80 bg-slate-50/90 px-4 py-3 dark:border-slate-700/70 dark:bg-slate-900/70"
    >
      <p class="text-[11px] font-semibold uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">
        {{ eyebrow }}
      </p>
      <h3 class="mt-1 text-sm font-semibold text-slate-900 dark:text-slate-100">
        {{ title }}
      </h3>
    </header>

    <div class="space-y-4 px-4 py-4">
      <div
        v-if="metaItems.length > 0"
        class="flex flex-wrap gap-2"
      >
        <span
          v-for="item in metaItems"
          :key="item"
          class="inline-flex items-center rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] font-medium text-slate-700 dark:border-slate-700 dark:bg-slate-800/80 dark:text-slate-200"
        >
          {{ item }}
        </span>
      </div>

      <div
        v-if="runnerError"
        class="rounded-2xl border border-rose-200 bg-rose-50/80 px-4 py-3 dark:border-rose-800/60 dark:bg-rose-900/20"
      >
        <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-rose-700 dark:text-rose-200">
          {{ t('settings.agentcoreRunner.lastError', 'Last error') }}
        </div>
        <p class="mt-2 whitespace-pre-wrap text-sm leading-6 text-rose-900 dark:text-rose-100">
          {{ runnerError }}
        </p>
      </div>

      <div
        v-if="responseText"
        class="rounded-2xl border border-slate-200 bg-slate-50/80 px-4 py-3 dark:border-slate-700 dark:bg-slate-800/70"
      >
        <p class="whitespace-pre-wrap text-sm leading-6 text-slate-800 dark:text-slate-100">
          {{ responseText }}
        </p>
      </div>

      <div
        v-if="runnerStderr && runnerStderr !== runnerError"
        class="rounded-2xl border border-amber-200 bg-amber-50/80 px-4 py-3 dark:border-amber-800/60 dark:bg-amber-900/20"
      >
        <p class="whitespace-pre-wrap text-sm leading-6 text-amber-900 dark:text-amber-100">
          {{ runnerStderr }}
        </p>
      </div>

      <section
        v-if="transcriptEntries.length > 0"
        class="space-y-2"
      >
        <div class="flex items-center justify-between gap-3">
          <h4 class="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
            {{ t('transcript', 'Transcript') }}
          </h4>
          <button
            v-if="hiddenTranscriptCount > 0"
            type="button"
            data-testid="chat-runner-execution-transcript-toggle"
            class="inline-flex items-center rounded-full border border-slate-200 px-2.5 py-1 text-[11px] font-medium text-slate-600 transition-colors hover:border-slate-300 hover:text-slate-900 dark:border-slate-700 dark:text-slate-300 dark:hover:border-slate-600 dark:hover:text-slate-100"
            :aria-expanded="transcriptExpanded ? 'true' : 'false'"
            :aria-label="transcriptExpanded ? 'Collapse transcript' : 'Expand transcript'"
            @click="transcriptExpanded = !transcriptExpanded"
          >
            {{ transcriptExpanded ? '-' : `+${hiddenTranscriptCount}` }}
          </button>
        </div>

        <div class="space-y-2">
          <div
            v-for="(entry, index) in visibleTranscriptEntries"
            :key="`${entry.direction || 'runner'}-${entry.method || 'event'}-${index}`"
            class="rounded-2xl border border-slate-200/90 bg-white px-3.5 py-3 dark:border-slate-700 dark:bg-slate-900/70"
          >
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-600 dark:bg-slate-800 dark:text-slate-300"
              >
                {{ entry.direction || 'runner' }}
              </span>
              <span
                v-if="entry.method"
                class="text-[11px] font-medium text-slate-500 dark:text-slate-400"
              >
                {{ entry.method }}
              </span>
            </div>
            <p class="mt-2 whitespace-pre-wrap text-sm leading-6 text-slate-700 dark:text-slate-200">
              {{ entry.text }}
            </p>
          </div>
        </div>
      </section>
    </div>
  </article>
</template>
