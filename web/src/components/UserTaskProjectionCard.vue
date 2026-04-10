<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserTaskActionID, UserTaskProjection, UserTaskResearchSource } from '@/api/tasks'
import {
  localizeTaskProjectionSubtitle,
  localizeTaskProjectionPreviewText,
  localizeTaskProjectionTitle,
} from '@/utils/taskProjectionText'
import {
  canOpenTaskConversation,
  projectTaskControlActions,
  type ProjectedUserTaskAction,
} from '@/utils/taskProjectionActions'
import {
  projectHarnessQuickLinks,
  projectHarnessSummary,
  type ProjectedHarnessQuickLink,
} from '@/utils/taskProjectionHarness'

const { t, te } = useI18n()

const props = withDefaults(
  defineProps<{
    task: UserTaskProjection
    collapseByDefault?: boolean
  }>(),
  {
    collapseByDefault: false,
  }
)

const emit = defineEmits<{
  action: [task: UserTaskProjection, actionId: UserTaskActionID, payload?: unknown]
  open: [task: UserTaskProjection]
  navigate: [href: string]
}>()

const expanded = ref(true)

const isTerminal = computed(() =>
  ['completed', 'failed', 'cancelled'].includes(String(props.task.status || ''))
)

watch(
  () => [props.task.id, props.task.status, props.collapseByDefault] as const,
  () => {
    expanded.value = !(props.collapseByDefault && isTerminal.value)
  },
  { immediate: true }
)

const progressWidth = computed(
  () => `${Math.max(0, Math.min(100, Math.round(Number(props.task.progress || 0))))}%`
)

const stageLabel = computed(() => {
  switch (props.task.stage) {
    case 'planning':
      return t('chat.taskStagePlanning', 'Planning')
    case 'working':
      return t('chat.taskStageWorking', 'Working')
    case 'verifying':
      return t('chat.taskStageVerifying', 'Verifying')
    case 'waiting_user':
      return t('chat.taskStageWaiting', 'Waiting')
    case 'completed':
      return t('chat.taskStageCompleted', 'Completed')
    case 'partial':
      return t('chat.taskStagePartial', 'Partially passed')
    case 'failed':
      return t('chat.taskStageFailed', 'Failed')
    case 'cancelled':
      return t('chat.taskStageCancelled', 'Cancelled')
    default:
      return props.task.stage
  }
})

const stageClass = computed(() => {
  switch (props.task.stage) {
    case 'completed':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
    case 'partial':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200'
    case 'failed':
      return 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-200'
    case 'cancelled':
      return 'bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
    case 'waiting_user':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    default:
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
  }
})

const progressClass = computed(() => {
  switch (props.task.stage) {
    case 'completed':
      return 'bg-emerald-500'
    case 'partial':
      return 'bg-amber-500'
    case 'failed':
      return 'bg-rose-500'
    case 'cancelled':
      return 'bg-slate-400'
    case 'waiting_user':
      return 'bg-amber-500'
    default:
      return 'bg-blue-500'
  }
})

function translate(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

const controlActions = computed<ProjectedUserTaskAction[]>(() =>
  projectTaskControlActions(props.task, translate)
)
const harnessSummary = computed(() => projectHarnessSummary(props.task, translate))
const harnessQuickLinks = computed<ProjectedHarnessQuickLink[]>(() =>
  projectHarnessQuickLinks(props.task, translate)
)
const canOpenConversation = computed(() => canOpenTaskConversation(props.task))

const kindLabel = computed(() => {
  switch (props.task.kind) {
    case 'research':
      return translate('chat.taskKindResearch', 'Deep Research')
    case 'workflow':
      return translate('chat.taskKindWorkflow', 'Workflow')
    default:
      return translate('chat.taskKindAgent', 'Agent')
  }
})

const kindIcon = computed(() => {
  switch (props.task.kind) {
    case 'research':
      return 'R'
    case 'workflow':
      return 'W'
    default:
      return 'A'
  }
})
const localizedTitle = computed(() =>
  localizeTaskProjectionTitle(props.task.title, props.task.kind, translate)
)
const localizedSubtitle = computed(() =>
  localizeTaskProjectionSubtitle(props.task.subtitle, props.task.kind, translate)
)

const previewText = computed(() => {
  if (props.task.error_preview) {
    return localizeTaskProjectionPreviewText(props.task.error_preview, translate)
  }
  return props.task.result_preview || ''
})
const usesCollapsedHeaderOnly = computed(() => isTerminal.value && props.collapseByDefault)
const researchSources = computed<UserTaskResearchSource[]>(() =>
  props.task.kind === 'research' ? props.task.research_sources || [] : []
)
const subagentSummary = computed(() => props.task.subagent_summary || null)
const subagentSummaryChips = computed(() => {
  const summary = subagentSummary.value
  if (!summary?.total) return []

  const chips = [`${summary.total} ${summary.total === 1 ? 'subagent' : 'subagents'}`]
  if (summary.running) chips.push(`${summary.running} running`)
  if (summary.waiting_user) chips.push(`${summary.waiting_user} waiting`)
  if (summary.failed) chips.push(`${summary.failed} failed`)
  if (summary.completed) chips.push(`${summary.completed} completed`)
  if (summary.cancelled) chips.push(`${summary.cancelled} cancelled`)
  return chips
})
const subagentLatestLabel = computed(() => {
  const summary = subagentSummary.value
  if (!summary?.latest_title) return ''
  const parts = [summary.latest_title]
  if (summary.latest_status) parts.push(summary.latest_status.replace(/_/g, ' '))
  return parts.join(' · ')
})

const blockerLabel = computed(() => {
  if (!props.task.blocker) return ''
  switch (props.task.blocker.kind) {
    case 'approval':
      return t('chat.taskWaitingForApproval', 'Waiting for your approval')
    case 'question':
      return t('chat.taskWaitingForAnswer', 'Waiting for your answer')
    default:
      return props.task.blocker.label || ''
  }
})

function formatDate(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString()
}

function formatScore(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '--'
  return value.toFixed(2)
}

function domainOf(source: UserTaskResearchSource): string {
  if (source.domain) return source.domain
  if (!source.url) return ''
  try {
    return new URL(source.url).hostname.replace(/^www\./, '')
  } catch {
    return source.url
  }
}

function actionButtonClass(action: ProjectedUserTaskAction): string {
  switch (String(action.variant || '').trim()) {
    case 'primary':
      return 'rounded-full border border-blue-200 px-3 py-1.5 text-xs font-medium text-blue-700 hover:bg-blue-50 dark:border-blue-900/60 dark:text-blue-200 dark:hover:bg-blue-950/30'
    case 'danger':
      return 'rounded-full border border-rose-200 px-3 py-1.5 text-xs font-medium text-rose-700 hover:bg-rose-50 dark:border-rose-900/60 dark:text-rose-200 dark:hover:bg-rose-950/30'
    default:
      return 'rounded-full border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800'
  }
}

function emitTaskAction(actionID: UserTaskActionID) {
  emit('action', props.task, actionID)
}
</script>

<template>
  <section
    class="my-2 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900/70"
  >
    <button
      v-if="isTerminal && collapseByDefault"
      type="button"
      class="flex w-full items-center justify-between gap-3 px-4 py-3 text-start"
      @click="expanded = !expanded"
    >
      <span class="flex min-w-0 items-center gap-3">
        <span
          class="inline-flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-semibold text-slate-700 dark:bg-slate-800 dark:text-slate-200"
        >
          {{ kindIcon }}
        </span>
        <span class="min-w-0">
          <span class="block truncate text-sm font-semibold text-slate-900 dark:text-slate-100">
            {{ localizedTitle }}
          </span>
          <span class="block text-xs text-slate-500 dark:text-slate-400">
            {{ stageLabel }}
          </span>
        </span>
      </span>
      <svg
        class="h-4 w-4 flex-shrink-0 text-slate-400 transition-transform"
        :class="{ 'rotate-180': expanded }"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <div v-show="expanded" class="px-4 py-4">
      <div v-if="!usesCollapsedHeaderOnly" class="flex items-start justify-between gap-3">
        <div class="flex min-w-0 items-start gap-3">
          <span
            class="inline-flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-semibold text-slate-700 dark:bg-slate-800 dark:text-slate-200"
          >
            {{ kindIcon }}
          </span>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="truncate text-sm font-semibold text-slate-900 dark:text-slate-100">
                {{ localizedTitle }}
              </span>
              <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="stageClass">
                {{ stageLabel }}
              </span>
              <span class="text-[11px] text-slate-500 dark:text-slate-400">{{ kindLabel }}</span>
            </div>
            <p v-if="localizedSubtitle" class="mt-1 text-xs text-slate-500 dark:text-slate-400">
              {{ localizedSubtitle }}
            </p>
            <div
              v-if="subagentSummaryChips.length"
              class="mt-2 flex flex-wrap gap-2 text-[11px] text-slate-500 dark:text-slate-400"
            >
              <span
                v-for="item in subagentSummaryChips"
                :key="`${task.id}-${item}`"
                class="rounded-full bg-slate-100 px-2.5 py-1 dark:bg-slate-800"
              >
                {{ item }}
              </span>
            </div>
            <p
              v-if="subagentLatestLabel"
              class="mt-2 text-[11px] text-slate-500 dark:text-slate-400"
            >
              {{ subagentLatestLabel }}
            </p>
          </div>
        </div>

        <div class="flex shrink-0 items-center gap-2">
          <button
            v-if="canOpenConversation"
            type="button"
            class="rounded-full border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
            @click="emit('open', task)"
          >
            {{ t('chat.taskOpenConversation', 'Open conversation') }}
          </button>
          <button
            v-for="link in harnessQuickLinks"
            :key="`${task.id}-${link.id}`"
            type="button"
            class="rounded-full border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
            @click="emit('navigate', link.href)"
          >
            {{ link.label }}
          </button>
          <button
            v-for="action in controlActions"
            :key="`${task.id}-${action.id}`"
            type="button"
            :class="actionButtonClass(action)"
            @click="emitTaskAction(action.id)"
          >
            {{ action.label }}
          </button>
        </div>
      </div>

      <div v-else class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <p v-if="localizedSubtitle" class="text-xs text-slate-500 dark:text-slate-400">
            {{ localizedSubtitle }}
          </p>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <button
            v-if="canOpenConversation"
            type="button"
            class="rounded-full border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
            @click="emit('open', task)"
          >
            {{ t('chat.taskOpenConversation', 'Open conversation') }}
          </button>
          <button
            v-for="link in harnessQuickLinks"
            :key="`${task.id}-collapsed-${link.id}`"
            type="button"
            class="rounded-full border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
            @click="emit('navigate', link.href)"
          >
            {{ link.label }}
          </button>
          <button
            v-for="action in controlActions"
            :key="`${task.id}-collapsed-${action.id}`"
            type="button"
            :class="actionButtonClass(action)"
            @click="emitTaskAction(action.id)"
          >
            {{ action.label }}
          </button>
        </div>
      </div>

      <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
        <div
          class="h-full transition-all duration-300"
          :class="progressClass"
          :style="{ width: progressWidth }"
        />
      </div>

      <div class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">
        {{ Math.max(0, Math.min(100, Math.round(Number(task.progress || 0)))) }}%
      </div>

      <div
        v-if="harnessSummary.length"
        class="mt-3 flex flex-wrap gap-2 text-[11px] text-slate-500 dark:text-slate-400"
      >
        <span
          v-for="item in harnessSummary"
          :key="`${task.id}-${item}`"
          class="rounded-full bg-slate-100 px-2.5 py-1 dark:bg-slate-800"
        >
          {{ item }}
        </span>
      </div>

      <div
        v-if="task.blocker"
        class="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200"
      >
        {{ blockerLabel }}
      </div>

      <div
        v-if="researchSources.length"
        class="mt-3 rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3 dark:border-slate-800 dark:bg-slate-950/50"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="text-xs font-semibold text-slate-700 dark:text-slate-200">
            {{ t('chat.deepResearchSourceInventory', 'Source Inventory') }}
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400">
            {{ researchSources.length }}
          </div>
        </div>
        <div class="mt-3 space-y-2">
          <a
            v-for="(source, index) in researchSources"
            :key="`${source.url || source.title}-${index}`"
            :href="source.url || undefined"
            :target="source.url ? '_blank' : undefined"
            :rel="source.url ? 'noopener noreferrer' : undefined"
            class="block rounded-xl border border-slate-200 bg-white px-3 py-3 dark:border-slate-800 dark:bg-slate-900/70"
          >
            <div class="text-sm font-medium text-slate-800 dark:text-slate-100 break-words">
              {{ source.title }}
            </div>
            <div class="mt-1 text-xs text-slate-500 dark:text-slate-400 break-words">
              {{ domainOf(source) || source.source_type || '--' }}
              <template v-if="domainOf(source) && source.source_type">
                · {{ source.source_type }}
              </template>
            </div>
            <div class="mt-2 flex flex-wrap gap-2 text-[11px] text-slate-500 dark:text-slate-400">
              <span v-if="source.published_at">
                {{ t('chat.deepResearchPublishedAt', 'Published') }}
                {{ formatDate(source.published_at) }}
              </span>
              <span v-if="source.fetched_at">
                {{ t('chat.deepResearchFetchedAt', 'Fetched') }} {{ formatDate(source.fetched_at) }}
              </span>
              <span>
                {{ t('chat.deepResearchRelevance', 'Rel') }}
                {{ formatScore(source.relevance_score) }}
              </span>
              <span>
                {{ t('chat.deepResearchCredibility', 'Cred') }}
                {{ formatScore(source.credibility_score) }}
              </span>
            </div>
          </a>
        </div>
      </div>

      <div
        v-if="previewText"
        class="mt-3 rounded-xl bg-slate-50 px-3 py-2 text-sm text-slate-700 dark:bg-slate-800/80 dark:text-slate-200"
      >
        {{ previewText }}
      </div>

      <div v-if="task.artifacts?.length" class="mt-3 flex flex-wrap gap-2">
        <template
          v-for="artifact in task.artifacts"
          :key="`${task.id}-${artifact.kind}-${artifact.label}`"
        >
          <a
            v-if="artifact.url"
            :href="artifact.url"
            target="_blank"
            rel="noreferrer"
            class="rounded-full border border-slate-200 px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
          >
            {{ artifact.label }}
          </a>
          <span
            v-else
            class="rounded-full border border-slate-200 px-3 py-1.5 text-xs text-slate-600 dark:border-slate-700 dark:text-slate-300"
          >
            {{ artifact.label }}
          </span>
        </template>
      </div>
    </div>
  </section>
</template>
