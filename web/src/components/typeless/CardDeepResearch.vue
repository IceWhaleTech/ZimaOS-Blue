<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CardSearch from '@/components/typeless/CardSearch.vue'
import { usePersistentDisclosureState } from '@/utils/chatCardUiState'
import type {
  TypelessCardDeepResearch,
  TypelessCardSearch,
  DeepResearchCitationItem,
  DeepResearchVerificationItem,
  DeepResearchWorkflowPhase,
  DeepResearchObjectMapItem,
  DeepResearchSourceInventoryItem,
  DeepResearchCalibration,
  DeepResearchTakeawayCandidate,
} from '@/types/typeless'
import {
  localizeDeepResearchGap,
  localizeDeepResearchReportStyle,
  localizeDeepResearchMode,
  localizeDeepResearchSegment,
  localizeDeepResearchSourceType,
  localizeDeepResearchStopReason,
  localizeDeepResearchStructuredValue,
  localizeDeepResearchSummary,
  localizeDeepResearchTimeWindow,
  localizeDeepResearchStatus,
  localizeResearchSurfaceTitle,
} from '@/utils/deepResearchText'

const { t, te, tm } = useI18n()

const props = defineProps<{
  card: TypelessCardDeepResearch
  uiStateKey?: string
}>()

const query = computed(() => props.card.query || '')
const answer = computed(() => props.card.answer || '')
const summaryKey = computed(() => {
  return (
    props.uiStateKey ||
    props.card.id ||
    `deep-research:${props.card.job_id || query.value || answer.value.slice(0, 80) || 'summary'}`
  )
})
const summaryPreview = computed(() => answer.value.replace(/\s+/g, ' ').trim())
const citations = computed(() => (props.card.citations || []).filter(validCitation))
const openQuestions = computed(() => props.card.open_questions || [])
const verificationSummary = computed(() => props.card.verification_summary || null)
const verificationItems = computed(() => verificationSummary.value?.items || [])
const researchTrace = computed(() => props.card.research_trace || [])
const timelineSections = computed(() => props.card.timeline_sections || [])
const stageErrors = computed(() => props.card.stage_errors || [])
const modeLabel = computed(() => localizeDeepResearchMode(props.card.mode || 'standard', tr))
const researchTitle = computed(() => localizeResearchSurfaceTitle(tr))
const searchCards = computed(() => (props.card.search_cards || []).filter(validSearchCard))
const reportStyle = computed(() => props.card.report_style || '')
const reportStyleLabel = computed(() => localizeDeepResearchReportStyle(reportStyle.value, tr))
const isKnowledgeBase = computed(() => reportStyle.value === 'knowledge_base')
const supportCount = computed(() => props.card.support_count || 0)
const conflictCount = computed(() => props.card.conflict_count || 0)
const hasConflict = computed(() => !!props.card.has_conflict || conflictCount.value > 0)
const iterations = computed(() => props.card.iterations || props.card.iteration || 0)
const evidenceCount = computed(() => props.card.evidence_count || 0)
const confidenceLabel = computed(() => formatPercent(props.card.confidence))
const citationCoverageLabel = computed(() => formatPercent(props.card.citation_coverage))
const statusLabel = computed(
  () =>
    localizeDeepResearchStatus(props.card.status || 'completed', tr) ||
    props.card.status ||
    'completed'
)
const timeWindows = computed(() => props.card.time_windows || [])
const workflowPhases = computed(() => (props.card.workflow_phases || []).filter(validWorkflowPhase))
const objectMap = computed(() => (props.card.object_map || []).filter(validObjectMapItem))
const sourceInventory = computed(() =>
  (props.card.source_inventory || []).filter(validSourceInventoryItem)
)
const sourceInventoryPreview = computed(() => sourceInventory.value.slice(0, 6))
const hasResearchDetails = computed(() => {
  return (
    sourceInventory.value.length > sourceInventoryPreview.value.length ||
    researchTrace.value.length > 0 ||
    stageErrors.value.length > 0
  )
})
const coverageSummary = computed(() => props.card.coverage_summary || null)
const calibration = computed(() => props.card.calibration || null)
const takeawayCandidates = computed(() =>
  (calibration.value?.takeaway_candidates || []).filter(validTakeawayCandidate)
)
const { expanded, toggleExpanded } = usePersistentDisclosureState(
  summaryKey,
  Boolean(props.card._streaming === true || !summaryPreview.value)
)
const disclosureLabel = computed(() =>
  expanded.value
    ? t('chat.deepResearchCollapseDetails', 'Collapse research details')
    : t('chat.deepResearchExpandDetails', 'Expand research details')
)

function tr(key: string, fallback: string): string {
  if (!te(key)) return fallback
  const value = tm(key)
  return typeof value === 'string' ? value : String(t(key))
}

function validCitation(item: unknown): item is DeepResearchCitationItem {
  if (!item || typeof item !== 'object') return false
  const citation = item as Record<string, unknown>
  return typeof citation.url === 'string' && citation.url.length > 0
}

function validSearchCard(item: unknown): item is TypelessCardSearch {
  if (!item || typeof item !== 'object') return false
  const card = item as Record<string, unknown>
  return card.type === 'search' && typeof card.query === 'string' && Array.isArray(card.results)
}

function validWorkflowPhase(item: unknown): item is DeepResearchWorkflowPhase {
  return !!item && typeof item === 'object'
}

function validObjectMapItem(item: unknown): item is DeepResearchObjectMapItem {
  return !!item && typeof item === 'object'
}

function validSourceInventoryItem(item: unknown): item is DeepResearchSourceInventoryItem {
  return !!item && typeof item === 'object'
}

function validTakeawayCandidate(item: unknown): item is DeepResearchTakeawayCandidate {
  return !!item && typeof item === 'object'
}

function formatPercent(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '--'
  return `${Math.max(0, Math.min(100, Math.round(value * 100)))}%`
}

function formatScore(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '--'
  return value.toFixed(2)
}

function domainOf(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
}

function formatDate(value?: string): string {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString()
}

function verificationStatusClass(status?: string): string {
  switch (status) {
    case 'resolved':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
    case 'conflicted':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    case 'current':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
    default:
      return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200'
  }
}

function verificationStatusLabel(status?: string): string {
  switch (status) {
    case 'resolved':
      return t('chat.deepResearchVerificationResolved', 'Resolved')
    case 'conflicted':
      return t('chat.deepResearchVerificationConflicted', 'Conflicted')
    case 'completed':
      return t('chat.deepResearchVerificationResolved', 'Resolved')
    case 'current':
      return t('chat.deepResearchStatus', 'Status')
    default:
      return t('chat.deepResearchVerificationInsufficient', 'Insufficient')
  }
}

function stopReasonLabel(reason?: string): string {
  return localizeDeepResearchStopReason(reason, tr) || reason || '--'
}

function verificationTitle(item: DeepResearchVerificationItem): string {
  return (
    localizeDeepResearchStructuredValue(item.focus, tr) ||
    item.focus ||
    localizeDeepResearchGap(item.gap, tr) ||
    t('chat.deepResearchVerificationSummary', 'Verification')
  )
}

function verificationSummaryText(summary?: string): string {
  return localizeDeepResearchSummary(summary, tr) || summary || ''
}

function openQuestionText(question?: string): string {
  return localizeDeepResearchSummary(question, tr) || question || ''
}

function workflowPhaseStatusLabel(status?: string): string {
  switch (status) {
    case 'completed':
      return t('chat.deepResearchWorkflowCompleted', 'Completed')
    case 'current':
      return t('chat.deepResearchWorkflowCurrent', 'Current')
    default:
      return t('chat.deepResearchWorkflowPending', 'Pending')
  }
}

function workflowPhaseLabel(phase: DeepResearchWorkflowPhase): string {
  return (
    localizeDeepResearchStructuredValue(phase.label || phase.id, tr) ||
    phase.label ||
    phase.id ||
    '--'
  )
}

function sourceTypeLabel(sourceType?: string): string {
  return localizeDeepResearchSourceType(sourceType || 'web', tr) || sourceType || 'web'
}

function objectStatusBadges(item: DeepResearchObjectMapItem): string[] {
  const counts = item.status_counts || {}
  return Object.entries(counts)
    .filter(([, count]) => typeof count === 'number' && count > 0)
    .map(([status, count]) => `${localizeDeepResearchSegment(status, tr) || status}: ${count}`)
}

function calibrationToneClass(calibrationItem: DeepResearchCalibration | null): string {
  switch (calibrationItem?.recommended_action) {
    case 'publish':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
    case 'caution':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    case 'insufficient':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
    default:
      return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200'
  }
}

function conflictRiskClass(risk?: string): string {
  switch (risk) {
    case 'blocking':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
    case 'medium':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    default:
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  }
}

function recommendedActionLabel(action?: string): string {
  switch (action) {
    case 'publish':
      return tr('chat.deepResearchCalibrationPublish', 'Publish-ready')
    case 'caution':
      return tr('chat.deepResearchCalibrationCaution', 'Use caution')
    case 'insufficient':
      return t('chat.deepResearchVerificationInsufficient', 'Insufficient')
    default:
      return action || '--'
  }
}

function conflictRiskLabel(risk?: string): string {
  switch (risk) {
    case 'blocking':
      return tr('chat.deepResearchCalibrationConflictBlocking', 'Blocking conflict')
    case 'medium':
      return tr('memory.proposalConflictRisk', 'Conflict risk')
    case 'low':
      return tr('chat.deepResearchCalibrationConflictLow', 'Low conflict risk')
    default:
      return risk || '--'
  }
}
</script>

<template>
  <div
    class="rounded-xl border border-slate-200/80 dark:border-slate-700/80 bg-white/95 dark:bg-slate-900/85 shadow-sm overflow-hidden"
  >
    <button
      type="button"
      data-testid="deep-research-summary-toggle"
      class="deep-research-summary-button w-full"
      :aria-expanded="expanded ? 'true' : 'false'"
      :aria-label="disclosureLabel"
      :title="disclosureLabel"
      @click="toggleExpanded"
    >
      <div
        class="px-4 py-3.5 bg-slate-50/90 dark:bg-slate-950/60"
        :class="{ 'border-b border-slate-100 dark:border-slate-800': expanded }"
      >
        <div class="flex flex-col gap-2.5 lg:flex-row lg:items-start lg:justify-between">
          <div class="min-w-0 flex-1">
            <div class="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">
              {{ researchTitle }}
            </div>
            <div
              v-if="query"
              class="mt-1.5 text-base font-semibold text-slate-900 dark:text-white break-words"
            >
              {{ query }}
            </div>
            <div
              v-if="summaryPreview && !expanded"
              class="mt-2 line-clamp-2 text-sm leading-5 text-slate-600 dark:text-slate-300"
            >
              {{ summaryPreview }}
            </div>
          </div>
          <div class="flex items-start gap-2.5">
            <div class="flex flex-wrap justify-end gap-1.5 text-xs">
              <span
                class="rounded-full bg-blue-100 px-2.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
              >{{ modeLabel }}</span>
              <span
                v-if="reportStyleLabel"
                class="rounded-full bg-purple-100 px-2.5 py-0.5 text-purple-700 dark:bg-purple-900/40 dark:text-purple-200"
              >{{ reportStyleLabel }}</span>
              <span
                class="rounded-full bg-slate-100 px-2.5 py-0.5 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >{{ t('chat.deepResearchCitationCoverage', 'Citation coverage') }}
                {{ citationCoverageLabel }}</span>
              <span
                class="rounded-full bg-slate-100 px-2.5 py-0.5 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >{{ t('chat.deepResearchStatus', 'Status') }} {{ statusLabel }}</span>
              <span
                class="rounded-full px-2.5 py-0.5"
                :class="
                  hasConflict
                    ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
                    : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
                "
              >
                {{
                  hasConflict
                    ? t('chat.deepResearchHasConflict', 'Conflicting signals')
                    : t('chat.deepResearchVerificationResolved', 'Resolved')
                }}
              </span>
            </div>
            <span
              class="pt-1 text-slate-400 transition-transform duration-200"
              :class="{ 'rotate-180': expanded }"
            >⌄</span>
          </div>
        </div>
      </div>
    </button>

    <div
      v-if="expanded"
      class="px-4 py-4 space-y-4"
    >
      <div class="grid gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <div class="space-y-4">
          <section class="rounded-xl bg-slate-50 px-3.5 py-3.5 dark:bg-slate-950/50">
            <div class="flex flex-wrap gap-2 text-xs text-slate-500 dark:text-slate-400">
              <span>{{ t('chat.deepResearchEvidence', 'Evidence') }} {{ evidenceCount }}</span>
              <span>{{ t('chat.deepResearchIterations', 'Iterations') }} {{ iterations }}</span>
              <span>{{ t('chat.deepResearchSupport', 'Support') }} {{ supportCount }}</span>
              <span>{{ t('chat.deepResearchConflict', 'Conflict') }} {{ conflictCount }}</span>
              <span>{{ confidenceLabel }}</span>
            </div>
            <div
              class="mt-2.5 whitespace-pre-wrap break-words text-sm leading-6 text-slate-800 dark:text-slate-100"
            >
              {{ answer || t('chat.waitingThinking', 'Thinking...') }}
            </div>
            <div
              v-if="timeWindows.length"
              class="mt-2.5 flex flex-wrap gap-2"
            >
              <span
                v-for="window in timeWindows"
                :key="window"
                class="rounded-full bg-white px-2.5 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300"
              >
                {{ localizeDeepResearchTimeWindow(window, tr) }}
              </span>
            </div>
            <div
              v-if="workflowPhases.length"
              class="mt-4 space-y-2"
            >
              <div
                class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400"
              >
                {{ t('chat.deepResearchWorkflowPhases', 'Workflow phases') }}
              </div>
              <div class="flex flex-wrap gap-2">
                <span
                  v-for="phase in workflowPhases"
                  :key="`${phase.id || phase.label}-${phase.status}`"
                  class="rounded-full px-2.5 py-0.5 text-xs"
                  :class="verificationStatusClass(phase.status)"
                >
                  {{ workflowPhaseLabel(phase) }} ·
                  {{ workflowPhaseStatusLabel(phase.status) }}
                </span>
              </div>
            </div>
          </section>

          <section
            v-if="citations.length > 0"
            class="space-y-2.5"
          >
            <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ t('chat.deepResearchCitations', 'Citations') }}
            </div>
            <div class="grid gap-2.5 md:grid-cols-2">
              <a
                v-for="(citation, index) in citations"
                :key="`${citation.url}-${index}`"
                :href="citation.url"
                target="_blank"
                rel="noopener noreferrer"
                class="rounded-xl border border-slate-200 bg-white px-3.5 py-2.5 transition-colors hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-950/50 dark:hover:bg-slate-900/70"
              >
                <div class="text-sm font-medium text-blue-600 dark:text-blue-300 break-words">
                  {{ citation.title || citation.url }}
                </div>
                <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  {{ domainOf(citation.url) }}
                </div>
              </a>
            </div>
          </section>

          <section
            v-if="searchCards.length > 0"
            class="rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
          >
            <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ tr('chat.deepResearchRetainedSearches', 'Retained web searches') }}
            </div>
            <div class="mt-2.5 space-y-3">
              <CardSearch
                v-for="(searchCard, index) in searchCards"
                :key="searchCard.id || `${searchCard.query}-${index}`"
                :card="searchCard"
                :ui-state-key="`${summaryKey}-search-${index}`"
              />
            </div>
          </section>

          <section
            v-if="sourceInventoryPreview.length"
            class="rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
                {{ t('chat.deepResearchSourceInventory', 'Source Inventory') }}
              </div>
              <div class="text-xs text-slate-500 dark:text-slate-400">
                {{ sourceInventory.length }}
              </div>
            </div>
            <div class="mt-2.5 space-y-2.5">
              <a
                v-for="(source, index) in sourceInventoryPreview"
                :key="`${source.source_id || source.url || index}`"
                :href="source.url || undefined"
                :target="source.url ? '_blank' : undefined"
                :rel="source.url ? 'noopener noreferrer' : undefined"
                class="block rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60"
              >
                <div class="text-sm font-medium text-blue-600 dark:text-blue-300 break-words">
                  {{ source.title || source.url || '--' }}
                </div>
                <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  {{ source.domain || domainOf(source.url || '') }} ·
                  {{ sourceTypeLabel(source.source_type) }}
                </div>
                <div
                  class="mt-1 flex flex-wrap gap-2 text-[11px] text-slate-400 dark:text-slate-500"
                >
                  <span v-if="source.published_at">{{ t('chat.deepResearchPublishedAt', 'Published') }}
                    {{ formatDate(source.published_at) }}</span>
                  <span v-if="source.fetched_at">{{ t('chat.deepResearchFetchedAt', 'Fetched') }}
                    {{ formatDate(source.fetched_at) }}</span>
                  <span>{{ t('chat.deepResearchRelevance', 'Rel') }}
                    {{ formatScore(source.relevance_score) }}</span>
                  <span>{{ t('chat.deepResearchCredibility', 'Cred') }}
                    {{ formatScore(source.credibility_score) }}</span>
                </div>
              </a>
            </div>
          </section>
        </div>

        <div class="space-y-3.5">
          <section
            v-if="calibration"
            class="rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
                {{ tr('memory.proposalCalibration', 'Calibration summary') }}
              </div>
              <span
                class="rounded-full px-2.5 py-1 text-xs font-medium"
                :class="calibrationToneClass(calibration)"
              >
                {{ recommendedActionLabel(calibration.recommended_action) }}
              </span>
            </div>
            <div class="mt-2.5 grid grid-cols-2 gap-2.5 text-xs">
              <div class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60">
                <div class="text-slate-500 dark:text-slate-400">
                  {{ tr('memory.proposalCoverage', 'Coverage') }}
                </div>
                <div class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">
                  {{ formatPercent(calibration.coverage) }}
                </div>
              </div>
              <div class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60">
                <div class="text-slate-500 dark:text-slate-400">
                  {{ tr('memory.proposalGroundedness', 'Groundedness') }}
                </div>
                <div class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">
                  {{ formatPercent(calibration.groundedness) }}
                </div>
              </div>
              <div class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60">
                <div class="text-slate-500 dark:text-slate-400">
                  {{ tr('memory.proposalFreshness', 'Freshness') }}
                </div>
                <div class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">
                  {{ formatPercent(calibration.freshness) }}
                </div>
              </div>
              <div class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60">
                <div class="text-slate-500 dark:text-slate-400">
                  {{ tr('memory.proposalConfidence', 'Confidence') }}
                </div>
                <div class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">
                  {{ formatPercent(calibration.confidence) }}
                </div>
              </div>
            </div>
            <div class="mt-3 flex flex-wrap gap-2 text-xs">
              <span
                class="rounded-full px-2.5 py-1"
                :class="conflictRiskClass(calibration.conflict_risk)"
              >
                {{ conflictRiskLabel(calibration.conflict_risk) }}
              </span>
              <span
                class="rounded-full bg-slate-100 px-2.5 py-1 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >
                {{ tr('memory.proposalCandidateCount', 'Takeaway candidates') }}
                {{ takeawayCandidates.length }}
              </span>
            </div>
            <div
              v-if="takeawayCandidates.length"
              class="mt-3 space-y-3"
            >
              <div
                v-for="(candidate, index) in takeawayCandidates"
                :key="`${candidate.lesson}-${index}`"
                class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60"
              >
                <div class="text-sm font-medium text-slate-800 dark:text-slate-100 break-words">
                  {{ candidate.lesson }}
                </div>
                <div
                  v-if="candidate.when_to_apply"
                  class="mt-1 text-xs text-slate-500 dark:text-slate-400 break-words"
                >
                  {{ candidate.when_to_apply }}
                </div>
                <div class="mt-1 text-xs text-slate-500 dark:text-slate-400 break-words">
                  {{ candidate.evidence }}
                </div>
              </div>
            </div>
          </section>

          <section
            v-if="isKnowledgeBase && coverageSummary"
            class="rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
          >
            <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ t('chat.deepResearchCoverageSummary', 'Coverage Summary') }}
            </div>
            <div class="mt-2.5 grid grid-cols-2 gap-2.5 text-xs">
              <div class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60">
                <div class="text-slate-500 dark:text-slate-400">
                  {{ t('chat.deepResearchTasks', 'Tasks') }}
                </div>
                <div class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">
                  {{ coverageSummary.task_count || 0 }}
                </div>
              </div>
              <div class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60">
                <div class="text-slate-500 dark:text-slate-400">
                  {{ t('chat.deepResearchEvidence', 'Evidence') }}
                </div>
                <div class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">
                  {{ coverageSummary.evidence_count || evidenceCount }}
                </div>
              </div>
              <div class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60">
                <div class="text-slate-500 dark:text-slate-400">
                  {{ t('chat.deepResearchDomains', 'Domains') }}
                </div>
                <div class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">
                  {{ coverageSummary.distinct_domain_count || 0 }}
                </div>
              </div>
              <div class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60">
                <div class="text-slate-500 dark:text-slate-400">
                  {{ t('chat.deepResearchOpenQuestions', 'Open questions') }}
                </div>
                <div class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">
                  {{ coverageSummary.open_question_count || openQuestions.length }}
                </div>
              </div>
            </div>
          </section>

          <section
            v-if="isKnowledgeBase && objectMap.length"
            class="rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
          >
            <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ t('chat.deepResearchObjectMap', 'Object Map') }}
            </div>
            <div class="mt-2.5 space-y-2.5">
              <div
                v-for="item in objectMap"
                :key="item.id || item.label"
                class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="text-sm font-medium text-slate-800 dark:text-slate-100 break-words">
                    {{
                      localizeDeepResearchStructuredValue(item.label || item.id, tr) ||
                        localizeDeepResearchSegment(item.label || item.id, tr) ||
                        item.label ||
                        item.id ||
                        '--'
                    }}
                  </div>
                  <span
                    class="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs text-slate-700 dark:bg-slate-800 dark:text-slate-200"
                  >
                    {{ t('chat.deepResearchTasks', 'Tasks') }} {{ item.task_count || 0 }}
                  </span>
                </div>
                <div
                  v-if="item.time_windows?.length"
                  class="mt-2 flex flex-wrap gap-2"
                >
                  <span
                    v-for="window in item.time_windows"
                    :key="window"
                    class="rounded-full bg-white px-2 py-1 text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                  >{{ localizeDeepResearchTimeWindow(window, tr) }}</span>
                </div>
                <div
                  v-if="objectStatusBadges(item).length"
                  class="mt-2 flex flex-wrap gap-2"
                >
                  <span
                    v-for="badge in objectStatusBadges(item)"
                    :key="badge"
                    class="rounded-full bg-white px-2 py-1 text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                  >{{ badge }}</span>
                </div>
                <ul
                  v-if="item.questions?.length"
                  class="deep-research-list mt-2 list-disc space-y-1 text-xs text-slate-500 dark:text-slate-400"
                >
                  <li
                    v-for="question in item.questions.slice(0, 3)"
                    :key="question"
                  >
                    {{ question }}
                  </li>
                </ul>
              </div>
            </div>
          </section>

          <section
            class="rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
          >
            <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ t('chat.deepResearchVerificationSummary', 'Verification') }}
            </div>
            <div class="mt-3 flex flex-wrap gap-2 text-xs">
              <span
                class="rounded-full bg-emerald-100 px-2.5 py-1 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
              >
                {{ t('chat.deepResearchVerificationResolved', 'Resolved') }}
                {{ verificationSummary?.resolved_count || 0 }}
              </span>
              <span
                class="rounded-full bg-amber-100 px-2.5 py-1 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
              >
                {{ t('chat.deepResearchVerificationConflicted', 'Conflicted') }}
                {{ verificationSummary?.conflicted_count || 0 }}
              </span>
              <span
                class="rounded-full bg-slate-100 px-2.5 py-1 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >
                {{ t('chat.deepResearchVerificationInsufficient', 'Insufficient') }}
                {{ verificationSummary?.insufficient_count || 0 }}
              </span>
            </div>
            <div
              v-if="verificationItems.length"
              class="mt-2.5 space-y-2.5"
            >
              <div
                v-for="(item, index) in verificationItems"
                :key="index"
                class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="text-sm font-medium text-slate-800 dark:text-slate-100 break-words">
                    {{ verificationTitle(item) }}
                  </div>
                  <span
                    class="rounded-full px-2.5 py-0.5 text-xs"
                    :class="verificationStatusClass(item.status)"
                  >{{ verificationStatusLabel(item.status) }}</span>
                </div>
                <div
                  v-if="item.summary"
                  class="mt-2 text-xs text-slate-500 dark:text-slate-400 break-words"
                >
                  {{ verificationSummaryText(item.summary) }}
                </div>
                <div
                  v-if="item.gap && item.gap !== item.focus"
                  class="mt-1 text-xs text-amber-700 dark:text-amber-200 break-words"
                >
                  {{ localizeDeepResearchGap(item.gap, tr) }}
                </div>
              </div>
            </div>
          </section>

          <section
            v-if="timelineSections.length"
            class="rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
          >
            <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ t('chat.deepResearchTimeline', 'Timeline') }}
            </div>
            <div class="mt-2.5 space-y-2.5">
              <div
                v-for="(section, index) in timelineSections"
                :key="index"
                class="rounded-lg bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60"
              >
                <div class="text-sm font-medium text-slate-800 dark:text-slate-100 break-words">
                  {{
                    localizeDeepResearchStructuredValue(section.label, tr) ||
                      localizeDeepResearchSegment(section.label, tr) ||
                      section.label
                  }}
                </div>
                <ul
                  v-if="section.highlights?.length"
                  class="deep-research-list mt-2 list-disc space-y-1 text-xs text-slate-500 dark:text-slate-400"
                >
                  <li
                    v-for="(item, itemIndex) in section.highlights"
                    :key="itemIndex"
                  >
                    {{ item }}
                  </li>
                </ul>
              </div>
            </div>
          </section>

          <section
            class="rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
          >
            <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ t('chat.deepResearchOpenQuestions', 'Open questions') }}
            </div>
            <ul
              v-if="openQuestions.length"
              class="deep-research-list mt-3 list-disc space-y-1 text-sm text-slate-600 dark:text-slate-300"
            >
              <li
                v-for="question in openQuestions"
                :key="question"
              >
                {{ openQuestionText(question) }}
              </li>
            </ul>
            <div
              v-else
              class="mt-3 text-sm text-slate-500 dark:text-slate-400"
            >
              {{ stopReasonLabel(props.card.stop_reason) }}
            </div>
          </section>
        </div>
      </div>

      <details
        v-if="hasResearchDetails"
        data-testid="deep-research-details"
        class="group rounded-xl border border-slate-200 bg-white px-3.5 py-3.5 dark:border-slate-800 dark:bg-slate-950/50"
      >
        <summary
          class="list-none cursor-pointer flex items-center justify-between gap-2 text-sm font-semibold text-slate-800 dark:text-slate-100"
        >
          <div class="min-w-0">
            <div>{{ tr('chat.deepResearchDetails', 'Research details') }}</div>
            <div
              class="mt-1 flex flex-wrap gap-2 text-xs font-normal text-slate-500 dark:text-slate-400"
            >
              <span v-if="sourceInventory.length">
                {{ t('chat.deepResearchSourceInventory', 'Source Inventory') }}
                {{ sourceInventory.length }}
              </span>
              <span v-if="researchTrace.length">
                {{ t('chat.deepResearchTraceEntries', 'Iterations') }}
                {{ researchTrace.length }}
              </span>
              <span v-if="stageErrors.length">
                {{ t('chat.deepResearchStageErrors', 'Stage warnings') }}
                {{ stageErrors.length }}
              </span>
            </div>
          </div>
          <span class="text-slate-400 transition-transform group-open:rotate-180">⌄</span>
        </summary>
        <div class="mt-4 space-y-3">
          <div
            v-if="sourceInventory.length"
            data-testid="deep-research-source-inventory-full"
            class="rounded-xl bg-slate-50 px-3 py-3 dark:bg-slate-900/60"
          >
            <div class="text-sm font-medium text-slate-800 dark:text-slate-100">
              {{ t('chat.deepResearchSourceInventory', 'Source Inventory') }}
            </div>
            <div class="mt-3 space-y-3">
              <a
                v-for="(source, index) in sourceInventory"
                :key="`${source.source_id || source.url || index}-full`"
                :href="source.url || undefined"
                :target="source.url ? '_blank' : undefined"
                :rel="source.url ? 'noopener noreferrer' : undefined"
                class="block rounded-xl border border-slate-200 bg-white px-3 py-3 dark:border-slate-800 dark:bg-slate-950/60"
              >
                <div class="text-sm font-medium text-blue-600 dark:text-blue-300 break-words">
                  {{ source.title || source.url || '--' }}
                </div>
                <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  {{ source.domain || domainOf(source.url || '') }} ·
                  {{ sourceTypeLabel(source.source_type) }}
                </div>
                <div
                  class="mt-1 flex flex-wrap gap-2 text-[11px] text-slate-400 dark:text-slate-500"
                >
                  <span v-if="source.published_at">{{ t('chat.deepResearchPublishedAt', 'Published') }}
                    {{ formatDate(source.published_at) }}</span>
                  <span v-if="source.fetched_at">{{ t('chat.deepResearchFetchedAt', 'Fetched') }}
                    {{ formatDate(source.fetched_at) }}</span>
                  <span>{{ t('chat.deepResearchRelevance', 'Rel') }}
                    {{ formatScore(source.relevance_score) }}</span>
                  <span>{{ t('chat.deepResearchCredibility', 'Cred') }}
                    {{ formatScore(source.credibility_score) }}</span>
                </div>
              </a>
            </div>
          </div>

          <div
            v-if="stageErrors.length"
            class="rounded-xl border border-amber-200 bg-amber-50 px-3 py-3 dark:border-amber-900/60 dark:bg-amber-950/30"
          >
            <div class="text-sm font-medium text-amber-700 dark:text-amber-200">
              {{ t('chat.deepResearchStageErrors', 'Stage warnings') }}
            </div>
            <ul
              class="deep-research-list mt-2 list-disc space-y-1 text-sm text-amber-700 dark:text-amber-200"
            >
              <li
                v-for="warning in stageErrors"
                :key="warning"
              >
                {{ warning }}
              </li>
            </ul>
          </div>

          <div
            v-for="(entry, index) in researchTrace"
            :key="index"
            class="rounded-xl bg-slate-50 px-3 py-3 dark:bg-slate-900/60"
          >
            <div
              class="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500 dark:text-slate-400"
            >
              <span>{{ t('chat.deepResearchIteration', 'Iteration') }} {{ entry.iteration }}</span>
              <span
                v-if="entry.verification_outcome"
                class="rounded-full px-2.5 py-1"
                :class="verificationStatusClass(entry.verification_outcome)"
              >
                {{ verificationStatusLabel(entry.verification_outcome) }}
              </span>
            </div>
            <div
              v-if="entry.focus"
              class="mt-2 text-sm font-medium text-slate-800 dark:text-slate-100 break-words"
            >
              {{
                localizeDeepResearchStructuredValue(entry.focus, tr) ||
                  localizeDeepResearchSegment(entry.focus, tr) ||
                  entry.focus
              }}
            </div>
            <div
              v-if="entry.gap"
              class="mt-1 text-xs text-amber-700 dark:text-amber-200 break-words"
            >
              {{ localizeDeepResearchGap(entry.gap, tr) }}
            </div>
            <div
              v-if="entry.follow_up_query"
              class="mt-1 text-xs text-slate-500 dark:text-slate-400 break-words"
            >
              {{ entry.follow_up_query }}
            </div>
            <div
              v-if="entry.evidence_added != null"
              class="mt-1 text-xs text-slate-500 dark:text-slate-400"
            >
              {{ t('chat.deepResearchEvidenceAdded', 'Evidence added') }} {{ entry.evidence_added }}
            </div>
          </div>
        </div>
      </details>
    </div>
  </div>
</template>

<style scoped>
.deep-research-summary-button {
  text-align: start;
}

.deep-research-list {
  padding-inline-start: 1rem;
}
</style>
