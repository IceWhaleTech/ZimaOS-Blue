<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import KnowledgeGraphMap from '@/components/automation/KnowledgeGraphMap.vue'
import {
  knowledgeApi,
  type KnowledgeAnswerReport,
  type KnowledgeIngestReport,
  type KnowledgeJob,
  type KnowledgeJobReport,
  type KnowledgeLintIssue,
  type KnowledgeLintReport,
  type KnowledgeLogEntry,
  type KnowledgePage,
  type KnowledgePageSummary,
} from '@/api/knowledge'
import { useKnowledgeJobs } from '@/composables/useKnowledgeJobs'
import { pickKnowledgeGraphDefaultFocusSlug } from '@/utils/knowledgeGraph'

type KnowledgePaneSummary = {
  visiblePages: number
  totalPages: number
  conflicts: number
  gaps: number
}

const emit = defineEmits<{
  'summary-change': [summary: KnowledgePaneSummary]
}>()

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()

const KNOWLEDGE_PAGE_GROUPS = [
  'source_summary',
  'entity',
  'concept',
  'comparison',
  'synthesis',
  'decision',
] as const

type KnowledgeGroupName = (typeof KNOWLEDGE_PAGE_GROUPS)[number]

const loading = ref(false)
const pageLoading = ref(false)
const statusMessage = ref('')
const errorMessage = ref('')
const schemaSaving = ref(false)
const promoting = ref(false)
const maintenanceExpanded = ref(false)

const pages = ref<KnowledgePageSummary[]>([])
const recentActivity = ref<KnowledgeLogEntry[]>([])
const schemaContent = ref('')
const selectedPage = ref<KnowledgePage | null>(null)
const selectedSlug = ref('')
const selectedMaintenancePanel = ref<'schema' | 'log'>('schema')
const latestLint = ref<KnowledgeLintReport | null>(null)
const latestIngest = ref<KnowledgeIngestReport | null>(null)
const latestAnswerJobID = ref('')

const pageSearch = ref('')
const pageTypeFilter = ref<'all' | string>('all')
const statusFilter = ref<'all' | string>('all')

const queryText = ref('')
const queryScope = ref<'all' | 'current_page' | 'selected_sources'>('current_page')
const saveAnswer = ref(true)
const answerError = ref('')
const answerReport = ref<KnowledgeAnswerReport | null>(null)
const groupExpanded = ref<Record<KnowledgeGroupName, boolean>>(
  Object.fromEntries(KNOWLEDGE_PAGE_GROUPS.map((group) => [group, false])) as Record<
    KnowledgeGroupName,
    boolean
  >
)

function tr(key: string, fallback: string) {
  return te(key) ? t(key) : fallback
}

function trp(
  key: string,
  fallback: string,
  params: Record<string, string | number | boolean>
): string {
  if (te(key)) return t(key, params)
  let text = fallback
  for (const [name, value] of Object.entries(params)) {
    text = text.split(`{${name}}`).join(String(value))
  }
  return text
}

function pageTypeLabel(pageType: string): string {
  if (!pageType) return ''
  return tr(`knowledge.pageTypes.${pageType}`, pageType)
}

function statusLabel(status: string): string {
  if (!status) return ''
  return tr(`knowledge.status.${status}`, status)
}

function repairJobStageLabel(stage?: string): string {
  const value = String(stage || '').trim().toLowerCase()
  switch (value) {
    case 'queued':
      return tr('knowledge.repairConflictsQueued', 'Queued for repair')
    case 'running':
      return tr('knowledge.repairConflictsRunning', 'Preparing repair')
    case 'scan_conflicts':
      return tr('knowledge.repairConflictsScan', 'Scanning conflict groups')
    case 'resolve_group':
      return tr('knowledge.repairConflictsResolving', 'Repairing current conflict group')
    case 'write_pages':
      return tr('knowledge.repairConflictsPersist', 'Writing repaired pages')
    case 'rerun_lint':
      return tr('knowledge.repairConflictsLint', 'Refreshing lint report')
    default:
      return value ? value.split('_').join(' ') : tr('knowledge.repairConflictsRunning', 'Preparing repair')
  }
}

function confidenceLabel(confidence?: string): string {
  if (!confidence) return tr('knowledge.unknown', 'unknown')
  return tr(`knowledge.confidence.${confidence}`, confidence)
}

function lintIssueKindLabel(kind: string): string {
  if (!kind) return ''
  return tr(`knowledge.lintIssueKinds.${kind}`, kind)
}

function formatDate(value?: string) {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = Date.parse(value)
  return Number.isNaN(parsed) ? value : new Date(parsed).toLocaleString()
}

async function scrollToPanel(panel: 'schema' | 'log') {
  selectedMaintenancePanel.value = panel
  maintenanceExpanded.value = true
  await nextTick()
  if (typeof document === 'undefined') return
  const target = document.getElementById(`knowledge-panel-${panel}`)
  target?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function loadPage(slug: string, syncRoute = true) {
  if (!slug) {
    selectedSlug.value = ''
    selectedPage.value = null
    return
  }
  selectedSlug.value = slug
  pageLoading.value = true
  try {
    const response = await knowledgeApi.getPage(slug)
    selectedPage.value = response.data
    if (syncRoute) {
      await router.replace({
        query: {
          ...route.query,
          page: slug,
        },
      })
    }
  } finally {
    pageLoading.value = false
  }
}

async function loadKnowledgeSpace() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [pagesResponse, lintResponse, schemaResponse, logResponse] = await Promise.allSettled([
      knowledgeApi.listPages(),
      knowledgeApi.getLatestLint(),
      knowledgeApi.getSchema(),
      knowledgeApi.getLog(),
    ])

    if (pagesResponse.status !== 'fulfilled') throw pagesResponse.reason
    pages.value = pagesResponse.value.data || []

    latestLint.value = lintResponse.status === 'fulfilled' ? lintResponse.value.data : null
    schemaContent.value =
      schemaResponse.status === 'fulfilled' ? schemaResponse.value.data.content : ''
    recentActivity.value = logResponse.status === 'fulfilled' ? logResponse.value.data : []

    const initialPage = Array.isArray(route.query.page) ? route.query.page[0] : route.query.page
    const nextSlug =
      (typeof initialPage === 'string' && initialPage) ||
      selectedSlug.value ||
      pickKnowledgeGraphDefaultFocusSlug(pages.value) ||
      pages.value[0]?.slug
    if (nextSlug) {
      await loadPage(nextSlug, false)
    }
  } catch (error) {
    errorMessage.value = String(error instanceof Error ? error.message : error)
  } finally {
    loading.value = false
  }
}

async function refreshAfterMaintenance() {
  await loadKnowledgeSpace()
}

const maintenanceJobs = useKnowledgeJobs({
  onTerminal: async (job: KnowledgeJob, report: KnowledgeJobReport | null) => {
    if (report?.repair) {
      latestLint.value = report.repair.lint || report.lint || latestLint.value
      statusMessage.value = tr('knowledge.repairConflictsComplete', 'Knowledge conflicts repaired.')
      await refreshAfterMaintenance()
      return
    }
    if (report?.ingest) {
      latestIngest.value = report.ingest
      statusMessage.value = tr('knowledge.ingestComplete', 'Knowledge ingest completed.')
      await refreshAfterMaintenance()
      return
    }
    if (report?.lint) {
      latestLint.value = report.lint
      statusMessage.value = tr('knowledge.lintComplete', 'Knowledge lint completed.')
      await refreshAfterMaintenance()
      return
    }
    if (job.status === 'failed') {
      errorMessage.value = job.error || tr('knowledge.jobFailed', 'Knowledge job failed.')
    }
  },
})

const queryJobs = useKnowledgeJobs({
  onTerminal: async (job: KnowledgeJob, report: KnowledgeJobReport | null) => {
    if (report?.answer) {
      latestAnswerJobID.value = job.id
      answerReport.value = report.answer
      statusMessage.value = tr('knowledge.answerReady', 'Knowledge answer is ready.')
      await loadKnowledgeSpace()
      return
    }
    if (job.status === 'failed') {
      answerError.value = job.error || tr('knowledge.answerFailed', 'Knowledge query failed.')
    }
  },
})

const allPageTypes = ['all', ...KNOWLEDGE_PAGE_GROUPS]

const filteredPages = computed(() => {
  const query = pageSearch.value.trim().toLowerCase()
  return pages.value.filter((page) => {
    if (pageTypeFilter.value !== 'all' && page.page_type !== pageTypeFilter.value) return false
    if (statusFilter.value !== 'all' && page.status !== statusFilter.value) return false
    if (!query) return true
    const haystack = [
      page.title,
      page.summary,
      page.page_type,
      page.status,
      ...(page.source_refs || []),
      ...(page.keywords || []),
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
    return haystack.includes(query)
  })
})

const groupedPages = computed<Record<KnowledgeGroupName, KnowledgePageSummary[]>>(() => {
  const groups = Object.fromEntries(
    KNOWLEDGE_PAGE_GROUPS.map((group) => [group, [] as KnowledgePageSummary[]])
  ) as Record<KnowledgeGroupName, KnowledgePageSummary[]>
  for (const page of filteredPages.value) {
    if (KNOWLEDGE_PAGE_GROUPS.includes(page.page_type as KnowledgeGroupName)) {
      groups[page.page_type as KnowledgeGroupName].push(page)
    }
  }
  return groups
})

const selectedKeywords = computed(() => selectedPage.value?.keywords || [])
const selectedAnswers = computed(() => selectedPage.value?.answers || [])
const conflictIssueCount = computed(
  () =>
    pages.value.filter((page) => page.status === 'conflicted').length ||
    latestLint.value?.issues?.filter((issue) => issue.category === 'review_required').length ||
    0
)
const gapIssueCount = computed(
  () =>
    latestLint.value?.issues?.filter((issue) => issue.category === 'research_suggestions').length ||
    0
)
const latestIngestEntry = computed(
  () => recentActivity.value.find((entry) => entry.operation === 'ingest') || null
)
const activeRepairJob = computed(() => {
  const job = maintenanceJobs.currentJob.value
  if (!job || job.kind !== 'repair_conflicts') return null
  const status = String(job.status || '').toLowerCase()
  if (status === 'completed' || status === 'failed' || status === 'cancelled') return null
  return job
})
const repairProgressPercent = computed(() => {
  const next = Number(activeRepairJob.value?.progress ?? 0)
  if (!Number.isFinite(next)) return 0
  return Math.max(0, Math.min(100, Math.round(next)))
})
const repairProgressStage = computed(() => repairJobStageLabel(activeRepairJob.value?.stage))
const repairProgressDetail = computed(() => String(activeRepairJob.value?.detail || '').trim())
const selectedRefsForQuery = computed(() => {
  if (queryScope.value !== 'selected_sources') return undefined
  return selectedPage.value?.source_refs?.length ? [...selectedPage.value.source_refs] : undefined
})
const knowledgeSummary = computed<KnowledgePaneSummary>(() => ({
  visiblePages: filteredPages.value.length,
  totalPages: pages.value.length,
  conflicts: conflictIssueCount.value,
  gaps: gapIssueCount.value,
}))

async function startMaintenance(kind: 'ingest' | 'lint' | 'repair_conflicts') {
  errorMessage.value = ''
  statusMessage.value = ''
  try {
    await maintenanceJobs.runJob({ kind })
    if (kind === 'ingest') {
      statusMessage.value = tr('knowledge.ingestStarted', 'Knowledge ingest started.')
      return
    }
    if (kind === 'repair_conflicts') {
      statusMessage.value = tr(
        'knowledge.repairConflictsStarted',
        'Knowledge conflict repair started.'
      )
      return
    }
    statusMessage.value = tr('knowledge.lintStarted', 'Knowledge lint started.')
  } catch (error) {
    errorMessage.value = String(error instanceof Error ? error.message : error)
  }
}

async function submitQuery() {
  const query = queryText.value.trim()
  if (!query || queryJobs.isRunning.value) return
  answerError.value = ''
  answerReport.value = null
  try {
    await queryJobs.runJob({
      kind: 'answer',
      query,
      page_slug: selectedSlug.value || undefined,
      archive_answer: saveAnswer.value,
      query_scope: queryScope.value,
      selected_refs: selectedRefsForQuery.value,
    })
    statusMessage.value = tr('knowledge.answerStarted', 'Knowledge query started.')
  } catch (error) {
    answerError.value = String(error instanceof Error ? error.message : error)
  }
}

async function saveSchema() {
  schemaSaving.value = true
  errorMessage.value = ''
  try {
    const response = await knowledgeApi.updateSchema(schemaContent.value)
    schemaContent.value = response.data.content
    statusMessage.value = tr('knowledge.schemaSaved', 'Knowledge schema saved.')
    recentActivity.value = (await knowledgeApi.getLog()).data
  } catch (error) {
    errorMessage.value = String(error instanceof Error ? error.message : error)
  } finally {
    schemaSaving.value = false
  }
}

async function promoteToWiki() {
  if (!latestAnswerJobID.value || promoting.value) return
  promoting.value = true
  errorMessage.value = ''
  try {
    const response = await knowledgeApi.promoteQuery(latestAnswerJobID.value)
    answerReport.value = {
      ...(answerReport.value as KnowledgeAnswerReport),
      promoted_page_slug: response.data.page.slug,
    }
    statusMessage.value = tr('knowledge.promoted', 'Query promoted to the wiki.')
    await loadKnowledgeSpace()
    await loadPage(response.data.page.slug, true)
  } catch (error) {
    errorMessage.value = String(error instanceof Error ? error.message : error)
  } finally {
    promoting.value = false
  }
}

function issueChipClass(issue: KnowledgeLintIssue) {
  if (issue.category === 'research_suggestions') return 'bg-sky-50 text-sky-700'
  if (issue.category === 'auto_fixed') return 'bg-emerald-50 text-emerald-700'
  return 'bg-rose-50 text-rose-700'
}

function isGroupExpanded(groupName: string) {
  return Boolean(groupExpanded.value[groupName as KnowledgeGroupName])
}

function toggleGroup(groupName: string) {
  const key = groupName as KnowledgeGroupName
  groupExpanded.value = {
    ...groupExpanded.value,
    [key]: !groupExpanded.value[key],
  }
}

watch(
  knowledgeSummary,
  (summary) => {
    emit('summary-change', summary)
  },
  { immediate: true }
)

watch(
  () => route.query.page,
  async (page) => {
    const next = Array.isArray(page) ? page[0] : page
    if (typeof next === 'string' && next && next !== selectedSlug.value) {
      await loadPage(next, false)
    }
  }
)

onMounted(async () => {
  await loadKnowledgeSpace()
})
</script>

<template>
  <div class="space-y-4">
    <div
      v-if="statusMessage"
      class="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ statusMessage }}
    </div>

    <div
      v-if="activeRepairJob"
      data-testid="knowledge-repair-progress"
      class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-4 text-rose-900"
    >
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div class="text-[11px] uppercase tracking-[0.2em] text-rose-600">
            {{ tr('knowledge.repairConflictsProgress', 'Conflict repair in progress') }}
          </div>
          <div class="mt-2 text-sm font-semibold">
            {{ repairProgressStage }}
          </div>
          <p v-if="repairProgressDetail" class="mt-1 text-sm text-rose-800">
            {{ tr('knowledge.repairConflictsCurrent', 'Currently processing') }}:
            {{ repairProgressDetail }}
          </p>
        </div>
        <span class="rounded-full bg-white px-3 py-1 text-xs font-semibold text-rose-700 shadow-sm">
          {{ repairProgressPercent }}%
        </span>
      </div>

      <div class="mt-3 h-2 overflow-hidden rounded-full bg-rose-100">
        <div
          class="h-full rounded-full bg-rose-500 transition-[width] duration-300"
          :style="{ width: `${repairProgressPercent}%` }"
        />
      </div>
    </div>

    <div
      v-if="errorMessage"
      class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700"
    >
      {{ errorMessage }}
    </div>

    <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
      <div class="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div>
          <div class="text-[11px] uppercase tracking-[0.2em] text-slate-500">
            {{ tr('knowledge.eyebrow', 'Knowledge Space') }}
          </div>
          <h2 class="mt-2 text-[15px] font-semibold text-slate-950">
            {{ tr('knowledge.mapTitle', 'Knowledge map') }}
          </h2>
          <p class="mt-1 text-[13px] leading-5 text-slate-600 sm:text-sm">
            {{
              tr(
                'knowledge.mapHint',
                'Browse durable wiki pages by type, search across sources and summaries, and inspect evidence state.'
              )
            }}
          </p>
        </div>

        <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap xl:justify-end">
          <button
            data-testid="knowledge-ingest-button"
            type="button"
            class="inline-flex min-h-11 items-center justify-center rounded-2xl bg-slate-900 px-3.5 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-60"
            :disabled="maintenanceJobs.isRunning.value"
            @click="startMaintenance('ingest')"
          >
            {{ tr('knowledge.ingest', 'Ingest') }}
          </button>
          <button
            data-testid="knowledge-lint-button"
            type="button"
            class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-slate-300 bg-white px-3.5 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-950 disabled:opacity-60"
            :disabled="maintenanceJobs.isRunning.value"
            @click="startMaintenance('lint')"
          >
            {{ tr('knowledge.lint', 'Lint') }}
          </button>
          <button
            data-testid="knowledge-repair-conflicts-button"
            type="button"
            class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-rose-200 bg-rose-50 px-3.5 py-2 text-sm font-medium text-rose-700 transition hover:bg-rose-100 disabled:opacity-60"
            :disabled="maintenanceJobs.isRunning.value || conflictIssueCount === 0"
            @click="startMaintenance('repair_conflicts')"
          >
            {{ tr('knowledge.repairConflicts', 'Repair conflicts') }}
          </button>
          <button
            data-testid="knowledge-maintenance-toggle"
            type="button"
            class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-amber-200 bg-amber-50 px-3.5 py-2 text-sm font-medium text-amber-900 transition hover:bg-amber-100"
            :aria-expanded="maintenanceExpanded ? 'true' : 'false'"
            @click="maintenanceExpanded = !maintenanceExpanded"
          >
            {{ tr('knowledge.controlTitle', 'Ingest and maintain Blue knowledge') }}
          </button>
        </div>
      </div>

      <div class="mt-4 flex flex-wrap gap-2">
        <span class="rounded-full bg-slate-100 px-3 py-1 text-xs text-slate-700">
          {{
            trp('knowledge.pageCount', '{count} pages', {
              count: knowledgeSummary.visiblePages,
            })
          }}
        </span>
        <span class="rounded-full bg-rose-50 px-3 py-1 text-xs text-rose-700">
          {{
            trp('knowledge.conflictCount', '{count} unresolved conflicts', {
              count: knowledgeSummary.conflicts,
            })
          }}
        </span>
        <span class="rounded-full bg-sky-50 px-3 py-1 text-xs text-sky-700">
          {{
            trp('knowledge.gapCount', '{count} open gaps', {
              count: knowledgeSummary.gaps,
            })
          }}
        </span>
      </div>

      <div class="mt-4 grid gap-3 sm:grid-cols-3">
        <input
          v-model="pageSearch"
          type="search"
          class="rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-900 shadow-sm outline-none transition focus:border-amber-300 focus:ring-2 focus:ring-amber-200"
          :placeholder="tr('knowledge.searchPlaceholder', 'Search titles, sources, keywords')"
        />
        <select
          v-model="pageTypeFilter"
          class="rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-900 shadow-sm outline-none transition focus:border-amber-300 focus:ring-2 focus:ring-amber-200"
        >
          <option v-for="pageType in allPageTypes" :key="pageType" :value="pageType">
            {{ pageTypeLabel(pageType) }}
          </option>
        </select>
        <select
          v-model="statusFilter"
          class="rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-900 shadow-sm outline-none transition focus:border-amber-300 focus:ring-2 focus:ring-amber-200"
        >
          <option value="all">{{ tr('knowledge.allStatuses', 'all statuses') }}</option>
          <option value="active">{{ statusLabel('active') }}</option>
          <option value="superseded">{{ statusLabel('superseded') }}</option>
          <option value="conflicted">{{ statusLabel('conflicted') }}</option>
        </select>
      </div>

      <div
        v-if="loading"
        class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-4 py-10 text-center text-sm text-slate-500"
      >
        {{ tr('common.loading', 'Loading...') }}
      </div>

      <div v-else class="mt-4 space-y-4">
        <KnowledgeGraphMap
          :pages="filteredPages"
          :selected-slug="selectedSlug"
          :page-type-label="pageTypeLabel"
          :status-label="statusLabel"
          @select="loadPage"
        />

        <div class="grid gap-4 xl:grid-cols-2">
          <div
            v-for="(groupPages, groupName) in groupedPages"
            :key="groupName"
            class="rounded-[1.35rem] border border-slate-200 bg-slate-50/70 p-2.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.55)]"
          >
            <button
              :data-testid="`knowledge-group-toggle-${groupName}`"
              type="button"
              class="flex min-h-11 w-full items-center justify-between gap-3 rounded-[1.1rem] px-3 py-2.5 text-left transition hover:bg-white/70 focus:outline-none focus:ring-2 focus:ring-amber-200"
              :aria-controls="`knowledge-group-panel-${groupName}`"
              :aria-expanded="isGroupExpanded(groupName) ? 'true' : 'false'"
              :aria-label="`${isGroupExpanded(groupName) ? tr('common.collapse', 'Collapse') : tr('common.expand', 'Expand')} ${pageTypeLabel(groupName)}`"
              @click="toggleGroup(groupName)"
            >
              <h3 class="text-[13px] font-semibold uppercase tracking-[0.14em] text-slate-600">
                {{ pageTypeLabel(groupName) }}
              </h3>

              <span class="flex items-center gap-2">
                <span class="rounded-full bg-white px-2.5 py-1 text-xs text-slate-500">
                  {{ groupPages.length }}
                </span>
                <svg
                  class="h-4 w-4 text-slate-400 transition-transform duration-200"
                  :class="{ 'rotate-180': isGroupExpanded(groupName) }"
                  viewBox="0 0 16 16"
                  fill="none"
                  aria-hidden="true"
                >
                  <path
                    d="M4 6.5 8 10l4-3.5"
                    stroke="currentColor"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.5"
                  />
                </svg>
              </span>
            </button>

            <div
              v-if="isGroupExpanded(groupName)"
              :id="`knowledge-group-panel-${groupName}`"
              :data-testid="`knowledge-group-panel-${groupName}`"
              class="mt-3 space-y-2 px-0.5 pb-0.5"
            >
              <button
                v-for="page in groupPages"
                :key="page.slug"
                type="button"
                class="w-full rounded-2xl border px-3 py-3 text-left transition"
                :class="
                  page.slug === selectedSlug
                    ? 'border-amber-300 bg-amber-50 text-amber-950 shadow-sm'
                    : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300 hover:bg-slate-50'
                "
                @click="loadPage(page.slug)"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="truncate text-sm font-semibold">{{ page.title }}</div>
                    <div class="mt-1 line-clamp-2 text-xs leading-5 text-slate-500">
                      {{ page.summary }}
                    </div>
                  </div>
                  <span
                    class="rounded-full px-2 py-1 text-[10px] uppercase tracking-[0.14em]"
                    :class="
                      page.status === 'conflicted'
                        ? 'bg-rose-100 text-rose-700'
                        : page.status === 'superseded'
                          ? 'bg-slate-200 text-slate-700'
                          : 'bg-emerald-100 text-emerald-700'
                    "
                  >
                    {{ statusLabel(page.status) }}
                  </span>
                </div>
              </button>

              <div v-if="groupPages.length === 0" class="text-sm text-slate-500">
                {{ tr('knowledge.emptyGroup', 'No pages in this group yet.') }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <div class="grid items-start gap-4">
      <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="text-[15px] font-semibold text-slate-950">
              {{ tr('knowledge.queryTitle', 'Query') }}
            </h2>
            <p class="mt-1 text-[13px] leading-5 text-slate-600 sm:text-sm">
              {{
                tr(
                  'knowledge.queryHint',
                  'Query the wiki, save good answers, and promote durable syntheses back into the space.'
                )
              }}
            </p>
          </div>

          <select
            data-testid="knowledge-query-scope"
            v-model="queryScope"
            class="rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-900 shadow-sm outline-none transition focus:border-amber-300 focus:ring-2 focus:ring-amber-200"
          >
            <option value="all">{{ tr('knowledge.scopeAll', 'all') }}</option>
            <option value="current_page">
              {{ tr('knowledge.scopeCurrentPage', 'current page') }}
            </option>
            <option value="selected_sources">
              {{ tr('knowledge.scopeSelectedSources', 'selected sources') }}
            </option>
          </select>
        </div>

        <textarea
          data-testid="knowledge-query-input"
          v-model="queryText"
          rows="4"
          class="mt-4 w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-900 shadow-sm outline-none transition focus:border-amber-300 focus:ring-2 focus:ring-amber-200"
          :placeholder="
            tr(
              'knowledge.queryPlaceholder',
              'What changed in this knowledge space, and what should I trust or investigate next?'
            )
          "
        />

        <label class="mt-4 flex items-center gap-3 text-sm text-slate-600">
          <input
            v-model="saveAnswer"
            type="checkbox"
            class="h-4 w-4 rounded border-slate-300 text-amber-600 focus:ring-amber-500"
          />
          {{ tr('knowledge.saveAnswer', 'Save answer history') }}
        </label>

        <div
          v-if="answerError"
          class="mt-4 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700"
        >
          {{ answerError }}
        </div>

        <div class="mt-4 flex flex-wrap gap-2.5">
          <button
            data-testid="knowledge-query-button"
            type="button"
            class="inline-flex min-h-11 items-center justify-center rounded-2xl bg-slate-900 px-3.5 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-60"
            :disabled="queryJobs.isRunning.value || !queryText.trim()"
            @click="submitQuery"
          >
            {{ tr('knowledge.query', 'Query') }}
          </button>
          <button
            v-if="answerReport"
            type="button"
            class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-amber-200 bg-amber-50 px-3.5 py-2 text-sm font-medium text-amber-900 transition hover:bg-amber-100 disabled:opacity-60"
            :disabled="promoting"
            @click="promoteToWiki"
          >
            {{ tr('knowledge.promoteToWiki', 'Promote to Wiki') }}
          </button>
        </div>

        <div
          v-if="answerReport"
          class="mt-5 rounded-2xl border border-amber-200/70 bg-amber-50/70 p-4"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="text-[11px] uppercase tracking-[0.2em] text-amber-700">
                {{ tr('knowledge.answerResult', 'Query result') }}
              </div>
              <p class="mt-2 whitespace-pre-wrap text-[13px] leading-5 text-amber-950 sm:text-sm">
                {{ answerReport.answer }}
              </p>
            </div>
            <span class="rounded-full bg-white px-3 py-1 text-xs font-medium text-amber-700">
              {{ answerReport.confidence || tr('knowledge.unknown', 'unknown') }}
            </span>
          </div>

          <div v-if="answerReport.citations?.length" class="mt-4 flex flex-wrap gap-2">
            <span
              v-for="citation in answerReport.citations"
              :key="`${citation.page_slug}-${citation.title}`"
              class="rounded-full bg-white px-3 py-1 text-xs text-amber-800 shadow-sm"
            >
              {{ citation.title }}
            </span>
          </div>

          <div
            v-if="answerReport.conflict_notes?.length"
            class="mt-4 rounded-2xl border border-rose-200 bg-rose-50 px-3 py-3 text-sm text-rose-700"
          >
            {{ answerReport.conflict_notes.join(' · ') }}
          </div>

          <div
            v-if="answerReport.open_questions?.length"
            class="mt-4 rounded-2xl border border-sky-200 bg-sky-50 px-3 py-3 text-sm text-sky-700"
          >
            {{ answerReport.open_questions.join(' · ') }}
          </div>
        </div>
      </section>

      <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
        <div
          v-if="pageLoading"
          class="rounded-2xl bg-slate-50 px-4 py-10 text-center text-sm text-slate-500"
        >
          {{ tr('common.loading', 'Loading...') }}
        </div>

        <div v-else-if="selectedPage" class="space-y-4">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-lg font-semibold text-slate-950 sm:text-xl">
                  {{ selectedPage.title }}
                </h2>
                <span
                  class="rounded-full bg-slate-950 px-3 py-1 text-xs uppercase tracking-[0.18em] text-white"
                >
                  {{ selectedPage.page_type }}
                </span>
              </div>
              <p class="mt-2 max-w-2xl text-[13px] leading-5 text-slate-600 sm:text-sm">
                {{ selectedPage.summary }}
              </p>
              <div class="mt-3 flex flex-wrap gap-2">
                <span class="rounded-full bg-slate-100 px-3 py-1 text-xs text-slate-600">
                  {{ tr('common.updatedAt', 'Updated') }}: {{ formatDate(selectedPage.updated_at) }}
                </span>
                <span
                  v-for="keyword in selectedKeywords"
                  :key="keyword"
                  class="rounded-full bg-white px-3 py-1 text-xs text-slate-500 shadow-sm"
                >
                  {{ keyword }}
                </span>
              </div>
            </div>
          </div>

          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-2xl border border-slate-200 bg-slate-50/80 p-3">
              <div class="text-[11px] uppercase tracking-[0.2em] text-slate-500">
                {{ tr('knowledge.evidenceState', 'Evidence & State') }}
              </div>
              <div class="mt-3 space-y-2 text-[13px] text-slate-700 sm:text-sm">
                <div>
                  <strong>{{ tr('knowledge.evidence.status', 'status') }}:</strong>
                  {{ statusLabel(selectedPage.status) }}
                </div>
                <div>
                  <strong>{{ tr('knowledge.evidence.confidence', 'confidence') }}:</strong>
                  {{ confidenceLabel(selectedPage.confidence) }}
                </div>
                <div>
                  <strong>{{ tr('knowledge.evidence.sourceRefs', 'source_refs') }}:</strong>
                  {{ selectedPage.source_refs.join(', ') || tr('knowledge.none', 'none') }}
                </div>
                <div>
                  <strong>{{ tr('knowledge.evidence.updatedAt', 'updated_at') }}:</strong>
                  {{ formatDate(selectedPage.updated_at) }}
                </div>
                <div>
                  <strong>{{ tr('knowledge.evidence.conflictsWith', 'conflicts_with') }}:</strong>
                  {{ selectedPage.conflicts_with?.join(', ') || tr('knowledge.none', 'none') }}
                </div>
                <div>
                  <strong>{{ tr('knowledge.evidence.supersededBy', 'superseded_by') }}:</strong>
                  {{ selectedPage.superseded_by?.join(', ') || tr('knowledge.none', 'none') }}
                </div>
              </div>
            </div>

            <div class="rounded-2xl border border-slate-200 bg-slate-50/80 p-3">
              <div class="text-[11px] uppercase tracking-[0.2em] text-slate-500">
                {{ tr('knowledge.relatedPages', 'Related pages') }}
              </div>
              <div class="mt-3 flex max-h-32 flex-wrap content-start gap-2 overflow-y-auto pr-1">
                <button
                  v-for="slug in selectedPage.backlinks"
                  :key="slug"
                  type="button"
                  class="rounded-full bg-white px-3 py-1 text-xs text-slate-700 shadow-sm transition hover:bg-amber-50"
                  @click="loadPage(slug)"
                >
                  {{ slug }}
                </button>
                <span v-if="selectedPage.backlinks.length === 0" class="text-sm text-slate-500">
                  {{ tr('knowledge.noRelatedPages', 'No related pages linked yet.') }}
                </span>
              </div>
            </div>

            <div class="rounded-2xl border border-slate-200 bg-slate-50/80 p-3">
              <div class="text-[11px] uppercase tracking-[0.2em] text-slate-500">
                {{ tr('knowledge.archivedAnswers', 'Saved answers') }}
              </div>
              <div class="mt-3 space-y-2">
                <div
                  v-for="answer in selectedAnswers"
                  :key="answer.path"
                  class="rounded-2xl bg-white px-3 py-2 text-[13px] text-slate-700 shadow-sm sm:text-sm"
                >
                  <div class="font-medium">{{ answer.query }}</div>
                  <div class="mt-1 text-xs text-slate-500">{{ answer.summary }}</div>
                </div>
                <div v-if="selectedAnswers.length === 0" class="text-sm text-slate-500">
                  {{
                    tr('knowledge.noArchivedAnswers', 'No saved answers linked to this page yet.')
                  }}
                </div>
              </div>
            </div>

            <div class="rounded-2xl border border-slate-200 bg-slate-50/80 p-3">
              <div class="text-[11px] uppercase tracking-[0.2em] text-slate-500">
                {{ tr('knowledge.lintHealth', 'Lint health') }}
              </div>
              <div
                class="mt-3 flex h-32 flex-wrap content-start items-start gap-2 overflow-y-auto pr-1"
              >
                <span
                  v-for="issue in latestLint?.issues || []"
                  :key="`${issue.kind}-${issue.message}`"
                  class="rounded-full px-2.5 py-1 text-[11px] font-medium"
                  :class="issueChipClass(issue)"
                >
                  {{ lintIssueKindLabel(issue.kind) }}
                </span>
                <span v-if="!latestLint?.issues?.length" class="text-sm text-slate-500">
                  {{ tr('knowledge.lintHealthy', 'Latest lint found no open issues.') }}
                </span>
              </div>
            </div>
          </div>

          <div class="rounded-2xl border border-slate-200 bg-slate-950 p-4 shadow-inner">
            <div class="text-[11px] uppercase tracking-[0.2em] text-slate-400">
              {{ tr('knowledge.pageBody', 'Compiled markdown') }}
            </div>
            <pre
              class="mt-3 overflow-x-auto whitespace-pre-wrap text-[13px] leading-5 text-slate-100 sm:text-sm"
              >{{ selectedPage.content }}</pre
            >
          </div>
        </div>

        <div
          v-else
          class="rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-4 py-10 text-center text-sm text-slate-500"
        >
          {{ tr('knowledge.selectPage', 'Select a page to inspect its detail view.') }}
        </div>
      </section>
    </div>

    <section
      v-if="maintenanceExpanded"
      data-testid="knowledge-maintenance-panel"
      class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm"
    >
      <div class="grid gap-4 xl:grid-cols-[18rem_minmax(0,1fr)]">
        <aside class="space-y-4">
          <section class="rounded-2xl border border-slate-200 bg-slate-50/70 p-4">
            <div class="text-[11px] uppercase tracking-[0.2em] text-slate-500">
              {{ tr('knowledge.controlEyebrow', 'Knowledge Control') }}
            </div>
            <h2 class="mt-2 text-[15px] font-semibold text-slate-950">
              {{ tr('knowledge.controlTitle', 'Ingest and maintain Blue knowledge') }}
            </h2>
            <p class="mt-1 text-[13px] leading-5 text-slate-600 sm:text-sm">
              {{
                tr(
                  'knowledge.controlDescription',
                  'Run ingest and lint jobs, then jump into the schema and activity panels.'
                )
              }}
            </p>

            <div class="mt-4 flex flex-col gap-2">
              <button
                type="button"
                class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-amber-200 bg-amber-50 px-3.5 py-2 text-sm font-medium text-amber-900 transition hover:bg-amber-100"
                @click="scrollToPanel('schema')"
              >
                {{ tr('knowledge.openSchema', 'Open Schema') }}
              </button>
              <button
                type="button"
                class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-sky-200 bg-sky-50 px-3.5 py-2 text-sm font-medium text-sky-900 transition hover:bg-sky-100"
                @click="scrollToPanel('log')"
              >
                {{ tr('knowledge.openLog', 'Open Log') }}
              </button>
            </div>

            <div
              v-if="latestIngest"
              class="mt-4 rounded-2xl border border-slate-200 bg-white/80 p-3 text-xs text-slate-600"
            >
              <div class="font-semibold text-slate-900">
                {{ tr('knowledge.latestIngestDelta', 'Latest ingest delta') }}
              </div>
              <div class="mt-2">
                {{
                  trp('knowledge.deltaSummary', '{newPages} new, {updatedPages} updated', {
                    newPages: latestIngest.new_pages?.length || 0,
                    updatedPages: latestIngest.updated_pages?.length || 0,
                  })
                }}
              </div>
            </div>

            <div
              v-else-if="latestIngestEntry"
              class="mt-4 rounded-2xl border border-slate-200 bg-white/80 p-3 text-xs text-slate-600"
            >
              <div class="font-semibold text-slate-900">
                {{ tr('knowledge.latestIngestDelta', 'Latest ingest delta') }}
              </div>
              <div class="mt-2 space-y-1">
                <div>
                  <strong>{{ tr('knowledge.changedPages', 'Changed pages') }}:</strong>
                  {{
                    latestIngestEntry.new_pages
                      ?.concat(latestIngestEntry.updated_pages || [])
                      .join(', ') || tr('knowledge.none', 'none')
                  }}
                </div>
                <div>
                  <strong>{{ tr('knowledge.conflictsLabel', 'Conflicts') }}:</strong>
                  {{ latestIngestEntry.conflicts?.join(', ') || tr('knowledge.none', 'none') }}
                </div>
                <div>
                  <strong>{{ tr('knowledge.gapsLabel', 'Gaps') }}:</strong>
                  {{ latestIngestEntry.gaps?.join(', ') || tr('knowledge.none', 'none') }}
                </div>
              </div>
            </div>
          </section>

          <section
            id="knowledge-panel-schema"
            class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"
          >
            <div class="flex items-center justify-between gap-3">
              <div>
                <h2 class="text-[15px] font-semibold text-slate-950">
                  {{ tr('knowledge.schemaTitle', 'Schema') }}
                </h2>
                <p class="mt-1 text-[13px] text-slate-600 sm:text-sm">
                  {{ tr('knowledge.schemaHint', 'Define how the wiki should be organized.') }}
                </p>
              </div>
              <span
                class="rounded-full px-2.5 py-1 text-[11px] font-medium"
                :class="
                  selectedMaintenancePanel === 'schema'
                    ? 'bg-amber-100 text-amber-700'
                    : 'bg-slate-100 text-slate-600'
                "
              >
                {{ tr('knowledge.active', 'Active') }}
              </span>
            </div>

            <textarea
              data-testid="knowledge-schema-editor"
              v-model="schemaContent"
              rows="10"
              class="mt-3 w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-900 shadow-sm outline-none transition focus:border-amber-300 focus:ring-2 focus:ring-amber-200"
            />

            <button
              type="button"
              class="mt-3 inline-flex min-h-11 items-center justify-center rounded-2xl bg-slate-900 px-3.5 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-60"
              :disabled="schemaSaving"
              @click="saveSchema"
            >
              {{ tr('knowledge.saveSchema', 'Save Schema') }}
            </button>
          </section>
        </aside>

        <section
          id="knowledge-panel-log"
          data-testid="knowledge-panel-log"
          class="flex min-h-[22rem] flex-col overflow-hidden rounded-2xl border border-slate-200 bg-white p-4 shadow-sm xl:h-[36rem]"
        >
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-[15px] font-semibold text-slate-950">
                {{ tr('knowledge.activityTitle', 'Recent activity') }}
              </h2>
              <p class="mt-1 text-[13px] text-slate-600 sm:text-sm">
                {{
                  tr(
                    'knowledge.activityHint',
                    'Read the latest ingest, query, lint, and schema events.'
                  )
                }}
              </p>
            </div>
            <span
              class="rounded-full px-2.5 py-1 text-[11px] font-medium"
              :class="
                selectedMaintenancePanel === 'log'
                  ? 'bg-sky-100 text-sky-700'
                  : 'bg-slate-100 text-slate-600'
              "
            >
              {{ recentActivity.length }}
            </span>
          </div>

          <div class="mt-4 min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
            <article
              v-for="entry in recentActivity"
              :key="`${entry.timestamp}-${entry.operation}-${entry.title}`"
              class="rounded-2xl border border-slate-200 bg-slate-50/80 p-3"
            >
              <div class="flex items-center justify-between gap-3">
                <div class="text-sm font-semibold text-slate-900">
                  {{ entry.title }}
                </div>
                <span
                  class="rounded-full bg-white px-2.5 py-1 text-[11px] uppercase tracking-[0.14em] text-slate-500"
                >
                  {{ entry.operation }}
                </span>
              </div>
              <div class="mt-2 text-xs text-slate-500">
                {{ formatDate(entry.timestamp) }}
              </div>
              <div v-if="entry.reason" class="mt-2 text-sm text-slate-600">
                {{ entry.reason }}
              </div>
            </article>
          </div>
        </section>
      </div>
    </section>
  </div>
</template>
