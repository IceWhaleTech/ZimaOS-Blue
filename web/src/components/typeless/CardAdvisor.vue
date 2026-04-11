<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type {
  AdvisorCandidateItem,
  AdvisorEvidenceItem,
  AdvisorWeightItem,
  TypelessCardAdvisor,
} from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardAdvisor
}>()

const title = computed(() => props.card.title || t('tools.names.advisor', 'Advisor'))
const recommendation = computed(() => {
  const direct = typeof props.card.recommendation === 'string' ? props.card.recommendation.trim() : ''
  if (direct) return direct
  const winner = typeof props.card.winner === 'string' ? props.card.winner.trim() : ''
  return winner
})
const whyItems = computed(() => normalizeStrings(props.card.why))
const tradeoffs = computed(() => normalizeStrings(props.card.tradeoffs))
const risks = computed(() => normalizeStrings(props.card.risks))
const bestPractices = computed(() => normalizeStrings(props.card.best_practices))
const alternatives = computed(() => normalizeStrings(props.card.alternatives))
const candidates = computed(() => normalizeCandidates(props.card.candidates).slice(0, 3))
const weights = computed(() => normalizeWeights(props.card.weights).slice(0, 3))
const evidenceItems = computed(() => normalizeEvidence(props.card.evidence).slice(0, 3))
const evidenceCount = computed(() => {
  if (typeof props.card.evidence_count === 'number' && Number.isFinite(props.card.evidence_count)) {
    return props.card.evidence_count
  }
  return evidenceItems.value.length
})
const confidenceLabel = computed(() => formatPercent(props.card.confidence))
const progressLabel = computed(() => formatPercent(props.card.progress))
const packLabel = computed(() =>
  typeof props.card.pack_id === 'string' ? props.card.pack_id.trim() : ''
)
const secondOpinionText = computed(() => normalizeSecondOpinion(props.card.second_opinion))
const hasDecisionMeta = computed(() => {
  return Boolean(
    props.card.winner ||
      whyItems.value.length ||
      tradeoffs.value.length ||
      risks.value.length ||
      bestPractices.value.length ||
      alternatives.value.length ||
      candidates.value.length ||
      weights.value.length ||
      evidenceCount.value > 0 ||
      typeof props.card.confidence === 'number'
  )
})
const isPending = computed(() => {
  if (props.card.status === 'success') return false
  return Boolean((props.card.job_id || typeof props.card.progress === 'number') && !hasDecisionMeta.value)
})
const progressValue = computed(() => {
  if (typeof props.card.progress !== 'number' || !Number.isFinite(props.card.progress)) return null
  if (props.card.progress <= 1 && props.card.progress >= 0) return Math.round(props.card.progress * 100)
  return clampPercent(props.card.progress)
})

function normalizeStrings(values?: string[] | null): string[] {
  if (!Array.isArray(values)) return []
  return values
    .map((value) => (typeof value === 'string' ? value.trim() : ''))
    .filter((value) => value.length > 0)
}

function normalizeCandidates(values?: AdvisorCandidateItem[] | null): AdvisorCandidateItem[] {
  if (!Array.isArray(values)) return []
  return values.filter((item) => item && typeof item === 'object')
}

function normalizeWeights(values?: AdvisorWeightItem[] | null): AdvisorWeightItem[] {
  if (!Array.isArray(values)) return []
  return values.filter((item) => item && typeof item === 'object')
}

function normalizeEvidence(values?: AdvisorEvidenceItem[] | null): AdvisorEvidenceItem[] {
  if (!Array.isArray(values)) return []
  return values.filter((item) => item && typeof item === 'object')
}

function normalizeSecondOpinion(value: TypelessCardAdvisor['second_opinion']): string {
  if (!value) return ''
  if (typeof value === 'string') return value.trim()
  if (typeof value === 'boolean') return value ? t('advisorCard.secondOpinion', 'Second opinion') : ''
  const summary =
    typeof value.summary === 'string'
      ? value.summary.trim()
      : typeof value.recommendation === 'string'
        ? value.recommendation.trim()
        : typeof value.note === 'string'
          ? value.note.trim()
          : ''
  if (summary) return summary
  return value.used ? t('advisorCard.secondOpinion', 'Second opinion') : ''
}

function clampPercent(value: number): number {
  return Math.max(0, Math.min(100, Math.round(value)))
}

function formatPercent(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '--'
  if (value >= 0 && value <= 1) return `${Math.round(value * 100)}%`
  return `${clampPercent(value)}%`
}

function formatWeight(weight?: number): string {
  if (typeof weight !== 'number' || !Number.isFinite(weight)) return '--'
  if (weight >= 0 && weight <= 1) return `${Math.round(weight * 100)}%`
  return `${Math.round(weight)}%`
}

function candidateName(candidate: AdvisorCandidateItem): string {
  return typeof candidate.name === 'string' && candidate.name.trim() ? candidate.name.trim() : '--'
}

function candidateVerdict(candidate: AdvisorCandidateItem): string {
  return typeof candidate.verdict === 'string' ? candidate.verdict.trim() : ''
}

function candidateScore(candidate: AdvisorCandidateItem): string {
  if (typeof candidate.total_score !== 'number' || !Number.isFinite(candidate.total_score)) return ''
  return `${Math.round(candidate.total_score)}`
}

function candidateRank(candidate: AdvisorCandidateItem, index: number): string {
  if (typeof candidate.rank === 'number' && Number.isFinite(candidate.rank)) return `#${candidate.rank}`
  return `#${index + 1}`
}

function weightLabel(weight: AdvisorWeightItem): string {
  if (typeof weight.label === 'string' && weight.label.trim()) return weight.label.trim()
  if (typeof weight.criterion === 'string' && weight.criterion.trim()) {
    return weight.criterion
      .trim()
      .split(/[_-]+/)
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join(' ')
  }
  return '--'
}

function evidenceLabel(item: AdvisorEvidenceItem): string {
  if (typeof item.label === 'string' && item.label.trim()) return item.label.trim()
  if (typeof item.source === 'string' && item.source.trim()) return item.source.trim()
  if (typeof item.domain === 'string' && item.domain.trim()) return item.domain.trim()
  return typeof item.url === 'string' ? item.url : '--'
}

function evidenceHref(item: AdvisorEvidenceItem): string {
  return typeof item.url === 'string' ? item.url : '#'
}

function evidenceDomain(item: AdvisorEvidenceItem): string {
  if (typeof item.domain === 'string' && item.domain.trim()) return item.domain.trim()
  if (typeof item.url !== 'string') return ''
  try {
    return new URL(item.url).hostname.replace(/^www\./, '')
  } catch {
    return ''
  }
}
</script>

<template>
  <section
    :id="card.id"
    class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-700/70 dark:bg-slate-900"
  >
    <div
      class="border-b border-slate-200 bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.14),_transparent_52%),linear-gradient(135deg,rgba(15,23,42,0.03),rgba(14,165,233,0.08))] px-4 py-4 dark:border-slate-700/60 dark:bg-[radial-gradient(circle_at_top_left,_rgba(56,189,248,0.16),_transparent_48%),linear-gradient(135deg,rgba(15,23,42,0.78),rgba(8,47,73,0.72))]"
    >
      <div class="flex items-start gap-3">
        <div
          class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-2xl bg-sky-100 text-xs font-semibold uppercase tracking-[0.18em] text-sky-700 dark:bg-sky-900/40 dark:text-sky-200"
        >
          AD
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="truncate text-sm font-semibold text-slate-900 dark:text-slate-50">
              {{ title }}
            </h3>
            <span
              v-if="isPending"
              class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
            >
              {{ t('advisorCard.pending', 'Advisor in progress') }}
            </span>
            <span
              v-else-if="packLabel"
              class="rounded-full bg-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-700 dark:bg-slate-700 dark:text-slate-200"
            >
              {{ packLabel }}
            </span>
          </div>

          <div class="mt-3 space-y-2">
            <p class="text-[11px] font-medium uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
              {{ t('advisorCard.recommendation', 'Recommendation') }}
            </p>
            <p class="text-sm leading-6 text-slate-800 dark:text-slate-100">
              {{ recommendation || '—' }}
            </p>
          </div>

          <div class="mt-4 flex flex-wrap gap-2 text-xs">
            <span
              class="rounded-full bg-slate-100 px-2.5 py-1 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
            >
              {{ t('advisorCard.confidence', 'Confidence') }} · {{ confidenceLabel }}
            </span>
            <span
              class="rounded-full bg-slate-100 px-2.5 py-1 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
            >
              {{ t('advisorCard.evidence', 'Evidence') }} · {{ evidenceCount }}
            </span>
            <span
              v-if="progressValue !== null"
              class="rounded-full bg-slate-100 px-2.5 py-1 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
            >
              {{ t('advisorCard.progress', 'Progress') }} · {{ progressLabel }}
            </span>
            <span
              v-if="card.job_id"
              class="rounded-full bg-slate-100 px-2.5 py-1 font-mono text-slate-700 dark:bg-slate-800 dark:text-slate-200"
            >
              {{ t('advisorCard.jobId', 'Job ID') }} · {{ card.job_id }}
            </span>
            <span
              v-if="card.winner && card.winner !== recommendation"
              class="rounded-full bg-emerald-100 px-2.5 py-1 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200"
            >
              {{ t('advisorCard.winner', 'Winner') }} · {{ card.winner }}
            </span>
          </div>

          <div
            v-if="progressValue !== null && isPending"
            class="mt-4 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800"
          >
            <div
              class="h-2 rounded-full bg-gradient-to-r from-sky-500 to-cyan-400 transition-[width] duration-300"
              :style="{ width: `${progressValue}%` }"
            />
          </div>
        </div>
      </div>
    </div>

    <div class="space-y-4 px-4 py-4">
      <div v-if="whyItems.length" class="space-y-2">
        <p class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
          {{ t('advisorCard.rationale', 'Why') }}
        </p>
        <ul class="space-y-2 text-sm leading-6 text-slate-700 dark:text-slate-200">
          <li v-for="item in whyItems" :key="item" class="rounded-xl bg-slate-50 px-3 py-2 dark:bg-slate-800/60">
            {{ item }}
          </li>
        </ul>
      </div>

      <div v-if="candidates.length || weights.length" class="grid gap-4 md:grid-cols-2">
        <div v-if="candidates.length" class="rounded-2xl border border-slate-200/80 p-3 dark:border-slate-700/70">
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
            {{ t('advisorCard.topCandidates', 'Top candidates') }}
          </p>
          <ul class="mt-3 space-y-2">
            <li
              v-for="(candidate, index) in candidates"
              :key="`${candidateName(candidate)}-${index}`"
              class="rounded-xl bg-slate-50 px-3 py-2 dark:bg-slate-800/60"
            >
              <div class="flex items-center justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-slate-900 dark:text-slate-50">
                    {{ candidateName(candidate) }}
                  </p>
                  <p v-if="candidateVerdict(candidate)" class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                    {{ candidateVerdict(candidate) }}
                  </p>
                </div>
                <div class="text-right text-xs text-slate-500 dark:text-slate-400">
                  <p>{{ candidateRank(candidate, index) }}</p>
                  <p v-if="candidateScore(candidate)">{{ candidateScore(candidate) }}</p>
                </div>
              </div>
            </li>
          </ul>
        </div>

        <div v-if="weights.length" class="rounded-2xl border border-slate-200/80 p-3 dark:border-slate-700/70">
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
            {{ t('advisorCard.weightedCriteria', 'Weighted criteria') }}
          </p>
          <ul class="mt-3 space-y-2">
            <li
              v-for="(weight, index) in weights"
              :key="`${weightLabel(weight)}-${index}`"
              class="flex items-center justify-between rounded-xl bg-slate-50 px-3 py-2 text-sm text-slate-700 dark:bg-slate-800/60 dark:text-slate-200"
            >
              <span class="truncate">{{ weightLabel(weight) }}</span>
              <span class="ml-3 rounded-full bg-sky-100 px-2 py-0.5 text-xs font-medium text-sky-700 dark:bg-sky-900/40 dark:text-sky-200">
                {{ formatWeight(weight.weight) }}
              </span>
            </li>
          </ul>
        </div>
      </div>

      <div
        v-if="tradeoffs.length || risks.length || bestPractices.length || alternatives.length || secondOpinionText"
        class="grid gap-4 md:grid-cols-2"
      >
        <div v-if="tradeoffs.length" class="rounded-2xl border border-amber-200/80 p-3 dark:border-amber-800/60">
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-amber-700 dark:text-amber-200">
            {{ t('advisorCard.tradeoffs', 'Tradeoffs') }}
          </p>
          <ul class="mt-3 space-y-2 text-sm leading-6 text-slate-700 dark:text-slate-200">
            <li v-for="item in tradeoffs" :key="item">{{ item }}</li>
          </ul>
        </div>

        <div v-if="risks.length" class="rounded-2xl border border-rose-200/80 p-3 dark:border-rose-800/60">
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-rose-700 dark:text-rose-200">
            {{ t('advisorCard.risks', 'Risks') }}
          </p>
          <ul class="mt-3 space-y-2 text-sm leading-6 text-slate-700 dark:text-slate-200">
            <li v-for="item in risks" :key="item">{{ item }}</li>
          </ul>
        </div>

        <div v-if="bestPractices.length" class="rounded-2xl border border-emerald-200/80 p-3 dark:border-emerald-800/60">
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-emerald-700 dark:text-emerald-200">
            {{ t('advisorCard.bestPractices', 'Best practices') }}
          </p>
          <ul class="mt-3 space-y-2 text-sm leading-6 text-slate-700 dark:text-slate-200">
            <li v-for="item in bestPractices" :key="item">{{ item }}</li>
          </ul>
        </div>

        <div v-if="alternatives.length || secondOpinionText" class="rounded-2xl border border-slate-200/80 p-3 dark:border-slate-700/70">
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
            {{
              alternatives.length
                ? t('advisorCard.alternatives', 'Alternatives')
                : t('advisorCard.secondOpinion', 'Second opinion')
            }}
          </p>
          <ul v-if="alternatives.length" class="mt-3 space-y-2 text-sm leading-6 text-slate-700 dark:text-slate-200">
            <li v-for="item in alternatives" :key="item">{{ item }}</li>
          </ul>
          <p
            v-if="secondOpinionText"
            class="mt-3 rounded-xl bg-slate-50 px-3 py-2 text-sm leading-6 text-slate-700 dark:bg-slate-800/60 dark:text-slate-200"
          >
            <span class="mr-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
              {{ t('advisorCard.secondOpinion', 'Second opinion') }}
            </span>
            {{ secondOpinionText }}
          </p>
        </div>
      </div>

      <div v-if="evidenceItems.length" class="space-y-2">
        <p class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
          {{ t('advisorCard.evidence', 'Evidence') }}
        </p>
        <div class="space-y-2">
          <a
            v-for="(item, index) in evidenceItems"
            :key="`${evidenceLabel(item)}-${index}`"
            :href="evidenceHref(item)"
            target="_blank"
            rel="noreferrer"
            class="flex items-center justify-between gap-3 rounded-xl border border-slate-200 px-3 py-2 text-sm text-slate-700 transition-colors hover:border-sky-300 hover:text-sky-700 dark:border-slate-700 dark:text-slate-200 dark:hover:border-sky-700 dark:hover:text-sky-200"
          >
            <span class="min-w-0 truncate">{{ evidenceLabel(item) }}</span>
            <span v-if="evidenceDomain(item)" class="flex-shrink-0 text-xs text-slate-500 dark:text-slate-400">
              {{ evidenceDomain(item) }}
            </span>
          </a>
        </div>
      </div>
    </div>
  </section>
</template>
