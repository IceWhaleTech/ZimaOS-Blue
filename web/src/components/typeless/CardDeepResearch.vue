<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardDeepResearch, DeepResearchCitationItem } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardDeepResearch
}>()

const query = computed(() => props.card.query || '')
const mode = computed(() => props.card.mode || 'standard')
const answer = computed(() => props.card.answer || '')
const evidenceCount = computed(() => props.card.evidence_count || 0)
const iteration = computed(() => props.card.iteration || 0)
const iterations = computed(() => props.card.iterations || 0)
const supportCount = computed(() => props.card.support_count || 0)
const conflictCount = computed(() => props.card.conflict_count || 0)
const hasConflict = computed(() => !!props.card.has_conflict || conflictCount.value > 0)
const citations = computed(() => (props.card.citations || []).filter(validCitation).slice(0, 8))
const openQuestions = computed(() => props.card.open_questions || [])
const stageErrors = computed(() => props.card.stage_errors || [])
const timelineSections = computed(() => props.card.timeline_sections || [])
const timeWindows = computed(() => props.card.time_windows || [])
const stopReason = computed(() => props.card.stop_reason || '')
const latestGap = computed(() => props.card.latest_gap || '')
const latestAction = computed(() => props.card.latest_action || '')
const researchTrace = computed(() => props.card.research_trace || [])
const verificationSummary = computed(() => props.card.verification_summary || null)
const verificationItems = computed(() => verificationSummary.value?.items || [])
const hasResearchDetails = computed(() => verificationItems.value.length > 0 || researchTrace.value.length > 0)
const citationCoverageText = computed(() => {
  const v = props.card.citation_coverage
  if (typeof v !== 'number') return '--'
  const pct = Math.max(0, Math.min(100, Math.round(v * 100)))
  return `${pct}%`
})
const entitySummary = computed(() => props.card.entity_disambiguation || null)
const confidenceText = computed(() => {
  const v = props.card.confidence
  if (typeof v !== 'number') return '--'
  const pct = Math.max(0, Math.min(100, Math.round(v * 100)))
  return `${pct}%`
})

function validCitation(item: unknown): item is DeepResearchCitationItem {
  if (!item || typeof item !== 'object') return false
  const c = item as Record<string, unknown>
  return typeof c.url === 'string' && c.url.length > 0
}

function domainOf(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
}

function verificationStatusClass(status?: string): string {
  switch (status) {
    case 'resolved':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'conflicted':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    default:
      return 'bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-200'
  }
}

function verificationStatusLabel(status?: string): string {
  switch (status) {
    case 'resolved':
      return t('chat.deepResearchVerificationResolved', 'Resolved')
    case 'conflicted':
      return t('chat.deepResearchVerificationConflicted', 'Conflicted')
    default:
      return t('chat.deepResearchVerificationInsufficient', 'Insufficient')
  }
}

function stopReasonLabel(reason?: string): string {
  switch (reason) {
    case 'coverage_sufficient':
      return t('chat.deepResearchStopReasonCoverage', 'Coverage target reached')
    case 'no_new_canonical_evidence':
      return t('chat.deepResearchStopReasonNoNewEvidence', 'No new canonical evidence found')
    case 'budget_exhausted':
      return t('chat.deepResearchStopReasonBudget', 'Research budget exhausted')
    default:
      return reason || ''
  }
}

function latestActionLabel(action?: string): string {
  switch (action) {
    case 'augment_query':
      return t('chat.deepResearchActionAugmentQuery', 'Augmenting query')
    case 'initial_retrieve':
      return t('chat.deepResearchActionInitialRetrieve', 'Running initial retrieval')
    case 'followup_retrieve':
      return t('chat.deepResearchActionFollowupRetrieve', 'Running follow-up retrieval')
    case 'verification':
      return t('chat.deepResearchActionVerification', 'Verifying evidence')
    case 'verification_completed':
      return t('chat.deepResearchActionVerificationCompleted', 'Verification completed')
    case 'followup_planned':
      return t('chat.deepResearchActionFollowupPlanned', 'Follow-up planned')
    case 'loop_stopped':
      return t('chat.deepResearchActionLoopStopped', 'Research loop stopped')
    case 'synthesizing':
      return t('chat.deepResearchActionSynthesizing', 'Synthesizing report')
    case 'completed':
      return t('chat.deepResearchActionCompleted', 'Completed')
    default:
      return action || ''
  }
}
</script>

<template>
  <div class="deep-research-card rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm">
    <div class="px-4 py-3 border-b border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/60">
      <div class="flex items-center justify-between gap-3">
        <div class="text-sm font-semibold text-gray-800 dark:text-gray-100">
          {{ t('chat.deepResearchTitle', 'Deep Research') }}
        </div>
        <div class="flex items-center gap-2 text-xs">
          <span class="px-2 py-0.5 rounded bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
            {{ mode }}
          </span>
          <span v-if="card.report_style" class="px-2 py-0.5 rounded bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300">
            {{ card.report_style }}
          </span>
          <span class="px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
            {{ confidenceText }}
          </span>
        </div>
      </div>
      <div v-if="query" class="mt-1 text-xs text-gray-500 dark:text-gray-400 truncate">
        {{ query }}
      </div>
    </div>

    <div class="px-4 py-3 space-y-3">
      <p v-if="answer" class="text-sm leading-6 text-gray-700 dark:text-gray-200 whitespace-pre-wrap">
        {{ answer }}
      </p>

      <div class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('chat.deepResearchEvidence', 'Evidence') }}: {{ evidenceCount }}
      </div>
      <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ t('chat.deepResearchSupport', 'Support') }}: {{ supportCount }}</span>
        <span>{{ t('chat.deepResearchConflict', 'Conflict') }}: {{ conflictCount }}</span>
        <span>{{ t('chat.deepResearchCitationCoverage', 'Citation Coverage') }}: {{ citationCoverageText }}</span>
        <span
          v-if="hasConflict"
          class="px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300"
        >
          {{ t('chat.deepResearchHasConflict', 'Conflicting signals') }}
        </span>
      </div>

      <div v-if="card.status || timeWindows.length > 0 || card.strict_entity || entitySummary || iterations || stopReason || latestGap || latestAction || iteration" class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
        <div v-if="card.status">{{ t('chat.deepResearchStatus', 'Status') }}: {{ card.status }}</div>
        <div v-if="iteration">{{ t('chat.deepResearchCurrentIteration', 'Current iteration') }}: {{ iteration }}</div>
        <div v-if="iterations">{{ t('chat.deepResearchIterations', 'Iterations') }}: {{ iterations }}</div>
        <div v-if="stopReason">{{ t('chat.deepResearchStopReason', 'Stop reason') }}: {{ stopReasonLabel(stopReason) }}</div>
        <div v-if="latestAction">{{ t('chat.deepResearchLatestAction', 'Latest action') }}: {{ latestActionLabel(latestAction) }}</div>
        <div v-if="latestGap">{{ t('chat.deepResearchLatestGap', 'Latest gap') }}: {{ latestGap }}</div>
        <div v-if="timeWindows.length > 0">{{ t('chat.deepResearchTimeWindows', 'Time Windows') }}: {{ timeWindows.join(', ') }}</div>
        <div v-if="card.strict_entity">{{ t('chat.deepResearchStrictEntity', 'Strict entity matching enabled') }}</div>
        <div v-if="entitySummary?.enabled">
          {{ t('chat.deepResearchEntitySummary', 'Entity filtering') }}:
          {{ t('chat.deepResearchEntityThreshold', { threshold: entitySummary.threshold ?? '--' }) }}
          <span v-if="entitySummary.filtered_count != null"> · {{ t('chat.deepResearchEntityFiltered', { count: entitySummary.filtered_count }) }}</span>
          <span v-if="entitySummary.ambiguous_count != null"> · {{ t('chat.deepResearchEntityAmbiguous', { count: entitySummary.ambiguous_count }) }}</span>
        </div>
      </div>

      <div v-if="citations.length > 0" class="space-y-2">
        <div class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('chat.deepResearchCitations', 'Citations') }}</div>
        <a
          v-for="(c, idx) in citations"
          :key="idx"
          :href="c.url"
          target="_blank"
          rel="noopener noreferrer"
          class="block px-2 py-1.5 rounded border border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors"
        >
          <div class="text-sm text-blue-600 dark:text-blue-400 line-clamp-1">
            {{ c.title || c.url }}
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
            {{ domainOf(c.url) }}
          </div>
        </a>
      </div>

      <div v-if="openQuestions.length > 0" class="space-y-1">
        <div class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('chat.deepResearchOpenQuestions', 'Open questions') }}</div>
        <ul class="text-xs text-gray-500 dark:text-gray-400 list-disc pl-4">
          <li v-for="(q, idx) in openQuestions" :key="idx">{{ q }}</li>
        </ul>
      </div>

      <div v-if="timelineSections.length > 0" class="space-y-2">
        <div class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('chat.deepResearchTimeline', 'Timeline') }}</div>
        <div
          v-for="(section, idx) in timelineSections"
          :key="idx"
          class="rounded border border-gray-100 dark:border-gray-700 px-3 py-2"
        >
          <div class="text-sm text-gray-700 dark:text-gray-200 font-medium">{{ section.label }}</div>
          <ul v-if="section.highlights?.length" class="mt-1 text-xs text-gray-500 dark:text-gray-400 list-disc pl-4">
            <li v-for="(item, itemIdx) in section.highlights" :key="itemIdx">{{ item }}</li>
          </ul>
        </div>
      </div>

      <div v-if="stageErrors.length > 0" class="space-y-1">
        <div class="text-xs font-medium text-amber-700 dark:text-amber-300">{{ t('chat.deepResearchStageErrors', 'Stage warnings') }}</div>
        <ul class="text-xs text-amber-600 dark:text-amber-300 list-disc pl-4">
          <li v-for="(item, idx) in stageErrors" :key="idx">{{ item }}</li>
        </ul>
      </div>

      <details v-if="hasResearchDetails" class="rounded border border-gray-100 dark:border-gray-700 px-3 py-2">
        <summary class="cursor-pointer text-xs font-medium text-gray-600 dark:text-gray-300">
          {{ t('chat.deepResearchTrace', 'Research trace') }}
        </summary>

        <div v-if="verificationSummary" class="mt-3 space-y-2">
          <div class="flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span>{{ t('chat.deepResearchVerificationSummary', 'Verification') }}</span>
            <span>{{ t('chat.deepResearchSupport', 'Support') }}: {{ verificationSummary.resolved_count || 0 }}</span>
            <span>{{ t('chat.deepResearchConflict', 'Conflict') }}: {{ verificationSummary.conflicted_count || 0 }}</span>
            <span>{{ t('chat.deepResearchVerificationInsufficient', 'Insufficient') }}: {{ verificationSummary.insufficient_count || 0 }}</span>
          </div>
          <div v-if="verificationItems.length > 0" class="space-y-2">
            <div
              v-for="(item, idx) in verificationItems"
              :key="idx"
              class="rounded border border-gray-100 dark:border-gray-700 px-3 py-2"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="text-sm text-gray-700 dark:text-gray-200 font-medium">{{ item.focus || item.gap || t('chat.deepResearchVerificationSummary', 'Verification') }}</div>
                <span class="px-2 py-0.5 rounded text-xs" :class="verificationStatusClass(item.status)">
                  {{ verificationStatusLabel(item.status) }}
                </span>
              </div>
              <div v-if="item.summary" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.summary }}</div>
              <div v-if="item.gap && item.gap !== item.focus" class="mt-1 text-xs text-amber-600 dark:text-amber-300">{{ item.gap }}</div>
            </div>
          </div>
        </div>

        <div v-if="researchTrace.length > 0" class="mt-3 space-y-2">
          <div class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('chat.deepResearchTraceEntries', 'Iterations') }}</div>
          <div
            v-for="(entry, idx) in researchTrace"
            :key="idx"
            class="rounded border border-gray-100 dark:border-gray-700 px-3 py-2"
          >
            <div class="flex items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('chat.deepResearchIteration', 'Iteration') }} {{ entry.iteration }}</span>
              <span v-if="entry.verification_outcome" class="px-2 py-0.5 rounded" :class="verificationStatusClass(entry.verification_outcome)">
                {{ verificationStatusLabel(entry.verification_outcome) }}
              </span>
            </div>
            <div v-if="entry.focus" class="mt-1 text-sm text-gray-700 dark:text-gray-200 font-medium">{{ entry.focus }}</div>
            <div v-if="entry.gap" class="mt-1 text-xs text-amber-600 dark:text-amber-300">{{ entry.gap }}</div>
            <div v-if="entry.follow_up_query" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ entry.follow_up_query }}</div>
            <div v-if="entry.evidence_added != null" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('chat.deepResearchEvidenceAdded', 'Evidence added') }}: {{ entry.evidence_added }}
            </div>
          </div>
        </div>
      </details>
    </div>
  </div>
</template>
