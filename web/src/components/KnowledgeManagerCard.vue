<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useKnowledgeJobs } from '@/composables/useKnowledgeJobs'
import type { KnowledgeJob, KnowledgeJobReport, KnowledgeLintReport } from '@/api/knowledge'

const props = withDefaults(
  defineProps<{
    latestLint?: KnowledgeLintReport | null
    showBrowseLink?: boolean
  }>(),
  {
    latestLint: null,
    showBrowseLink: true,
  }
)

const emit = defineEmits<{
  (e: 'status-change', message: string): void
  (e: 'job-finished', payload: { job: KnowledgeJob; report: KnowledgeJobReport | null }): void
}>()

const { t, te } = useI18n()
const router = useRouter()
const actionError = ref('')

function tr(key: string, fallback: string) {
  return te(key) ? t(key) : fallback
}

function trp(
  key: string,
  fallback: string,
  params: Record<string, string | number | boolean>
) {
  if (te(key)) return t(key, params)
  let text = fallback
  for (const [name, value] of Object.entries(params)) {
    text = text.split(`{${name}}`).join(String(value))
  }
  return text
}

const { currentJob, isRunning, runJob } = useKnowledgeJobs({
  onTerminal: async (job, report) => {
    const label =
      report?.kind === 'ingest'
        ? tr('knowledge.ingestComplete', 'Knowledge ingest completed.')
        : report?.kind === 'lint'
          ? tr('knowledge.lintComplete', 'Knowledge lint completed.')
          : tr('knowledge.jobComplete', 'Knowledge job completed.')
    emit('status-change', label)
    emit('job-finished', { job, report })
  },
})

const lintSummary = computed(() => {
  if (!props.latestLint) return tr('knowledge.lintUnavailable', 'No lint report yet.')
  const issueCount = props.latestLint.issues?.length ?? 0
  return issueCount > 0
    ? trp('knowledge.lintIssues', '{count} lint issues need review.', { count: issueCount })
    : tr('knowledge.lintHealthy', 'Latest lint found no open issues.')
})

async function startJob(kind: 'compile' | 'lint') {
  actionError.value = ''
  emit(
    'status-change',
    kind === 'compile'
      ? tr('knowledge.ingestStarted', 'Knowledge ingest started.')
      : tr('knowledge.lintStarted', 'Knowledge lint started.')
  )
  try {
    await runJob({ kind: kind === 'compile' ? 'ingest' : 'lint' })
  } catch (error) {
    actionError.value = String(error instanceof Error ? error.message : error)
    emit('status-change', actionError.value)
  }
}

function openKnowledge() {
  void router.push('/operations/knowledge')
}

function formatGeneratedAt(value?: string) {
  if (!value) return tr('knowledge.never', 'Never')
  const parsed = Date.parse(value)
  return Number.isNaN(parsed) ? value : new Date(parsed).toLocaleString()
}

const statusPillClass = computed(() => {
  const status = String(currentJob.value?.status || '').toLowerCase()
  if (status === 'completed') {
    return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  }
  if (status === 'failed' || status === 'cancelled') {
    return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  }
  return 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
})
</script>

<template>
  <div class="dashboard-card-subsurface settings-feature-card p-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="space-y-1">
        <div class="text-[11px] uppercase tracking-[0.22em] text-gray-400 dark:text-gray-500">
          {{ tr('knowledge.controlEyebrow', 'Knowledge Control') }}
        </div>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ tr('knowledge.controlTitle', 'Ingest and maintain Blue knowledge') }}
        </h3>
        <p class="max-w-xl text-sm leading-5 text-gray-600 dark:text-gray-300">
          {{
            tr(
              'knowledge.controlDescription',
              'Ingest curated docs into linked wiki pages, lint knowledge quality, and jump to the browse/query surface.'
            )
          }}
        </p>
      </div>
      <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="statusPillClass">
        {{ currentJob?.status || tr('knowledge.idle', 'idle') }}
      </span>
    </div>

    <div class="mt-4 grid gap-2.5 md:grid-cols-2">
      <div
        class="rounded-2xl border border-gray-200/70 bg-white/70 p-3 dark:border-white/10 dark:bg-slate-900/40"
      >
        <div class="text-[11px] uppercase tracking-[0.2em] text-gray-400 dark:text-gray-500">
          {{ tr('knowledge.latestLint', 'Latest lint') }}
        </div>
        <div class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
          {{ lintSummary }}
        </div>
        <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {{ formatGeneratedAt(latestLint?.generated_at) }}
        </div>
      </div>

      <div
        class="rounded-2xl border border-gray-200/70 bg-white/70 p-3 dark:border-white/10 dark:bg-slate-900/40"
      >
        <div class="text-[11px] uppercase tracking-[0.2em] text-gray-400 dark:text-gray-500">
          {{ tr('knowledge.activeJob', 'Active job') }}
        </div>
	        <div class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
	          {{ currentJob?.kind || tr('knowledge.noActiveJob', 'No active job') }}
	        </div>
        <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {{ currentJob?.stage || tr('knowledge.awaitingAction', 'Awaiting action') }}
        </div>
      </div>
    </div>

    <div
      v-if="actionError"
      class="mt-4 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300"
    >
      {{ actionError }}
    </div>

    <div class="mt-4 flex flex-wrap gap-2.5">
      <button
        data-testid="knowledge-ingest-button"
        type="button"
        class="inline-flex min-h-11 items-center justify-center rounded-2xl bg-slate-900 px-3.5 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        :disabled="isRunning"
        @click="startJob('compile')"
      >
        {{ tr('knowledge.ingest', 'Ingest') }}
      </button>
      <button
        data-testid="knowledge-lint-button"
        type="button"
        class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-gray-300 bg-white px-3.5 py-2 text-sm font-medium text-gray-700 transition hover:border-gray-400 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-60 dark:border-white/10 dark:bg-slate-900 dark:text-gray-100 dark:hover:border-white/20"
        :disabled="isRunning"
        @click="startJob('lint')"
      >
        {{ tr('knowledge.lint', 'Lint') }}
      </button>
      <button
        v-if="showBrowseLink !== false"
        data-testid="knowledge-open-button"
        type="button"
        class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-transparent bg-amber-100 px-3.5 py-2 text-sm font-medium text-amber-900 transition hover:bg-amber-200 dark:bg-amber-500/20 dark:text-amber-200 dark:hover:bg-amber-500/30"
        @click="openKnowledge"
      >
        {{ tr('knowledge.openWorkspace', 'Open Knowledge') }}
      </button>
    </div>
  </div>
</template>
