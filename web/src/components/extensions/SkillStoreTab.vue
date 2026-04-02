<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  type MarketplaceAdviceResponse,
  type DiscoverStatusResponse,
  type EmbeddingStatusResponse,
  skillApi,
  type MarketSearchParams,
  type MarketplaceSkillDetail,
  type RemoteSkill,
  type SecurityBadge,
  type SkillContractMetadata,
  type SkillFiltersResponse,
  type SkillSecurityEvidence,
} from '@/api/skill'
import SkillContractNotice from '@/components/extensions/SkillContractNotice.vue'
import SemanticSearchField from '@/components/ui/SemanticSearchField.vue'
import { formatVersionLabel } from '@/utils/version-label'
import { getErrorMessage } from '@/utils/error'
import {
  skillStoreSortTranslationPath,
  type SkillStoreSortMode,
} from '@/components/extensions/skillStoreSort'
import { offSSEEvent, onSSEEvent } from '@/composables/useEventStream'

const props = defineProps<{
  initialSearchQuery?: string
}>()

const { t, te, locale } = useI18n()

const loading = ref(false)
const loadingMore = ref(false)
const refreshing = ref(false)
const initializingMarketplace = ref(false)
const error = ref<string | null>(null)
const DISCOVER_POLL_INTERVAL_MS = 2000
let latestSkillsRequestId = 0
let latestDetailRequestId = 0
let latestDiscoverPollId = 0
let latestAdviceRequestId = 0
let preferredSelectedSkillId: string | null = null
let componentDisposed = false
let refreshVisibleResultsTimer: ReturnType<typeof window.setTimeout> | null = null
let discoverActivityId = 0
let lastDiscoverActivitySignature = ''
let lastVisibleResultsRefreshSignature = ''
let marketplaceInitialized = false

const skills = ref<RemoteSkill[]>([])
const filters = ref<SkillFiltersResponse | null>(null)
const discoverStatus = ref<DiscoverStatusResponse | null>(null)
const embeddingStatus = ref<EmbeddingStatusResponse | null>(null)
const discoverActivityViewport = ref<HTMLElement | null>(null)
const discoverActivity = ref<
  Array<{
    id: number
    phase: 'started' | 'batch' | 'source_complete' | 'completed' | 'error' | 'status'
    title: string
    detail: string
    timestamp: number
  }>
>([])
const selectedSkillId = ref<string | null>(null)
const selectedDetail = ref<MarketplaceSkillDetail | null>(null)
const detailLoading = ref(false)
const showDetailModal = ref(false)
const skillAdvice = ref<MarketplaceAdviceResponse | null>(null)
const adviceLoading = ref(false)

const searchQuery = ref(normalizeSearchQuery(props.initialSearchQuery || ''))
const selectedCategory = ref('all')
const selectedSource = ref('all')
const selectedRisk = ref('all')
const defaultSortMode: SkillStoreSortMode = 'trending'
const sortMode = ref<SkillStoreSortMode>(defaultSortMode)

const page = ref(1)
const totalPages = ref(1)
const totalSkills = ref(0)
const pageSize = 20

const installingSkillId = ref<string | null>(null)
const pendingRiskSkill = ref<RemoteSkill | null>(null)

const selectedSkill = computed<RemoteSkill | null>(() => {
  if (!selectedSkillId.value) return null
  return skills.value.find((skill) => skill.id === selectedSkillId.value) ?? null
})

const detailSkill = computed<RemoteSkill | null>(() => {
  const base = selectedDetail.value?.skill
    ? normalizeSkill(selectedDetail.value.skill)
    : selectedSkill.value
  if (!base) return null
  return {
    ...base,
    version: base.version || selectedDetail.value?.version?.version,
    source_url: selectedDetail.value?.version?.source_url || base.source_url,
    installed: selectedDetail.value?.installed ?? base.installed,
  }
})

const selectedSecurity = computed(() => selectedDetail.value?.security ?? null)
const detailInstalled = computed(() => !!detailSkill.value?.installed)
const detailUpdatedAt = computed(
  () =>
    selectedDetail.value?.version?.released_at ||
    detailSkill.value?.updated_at ||
    detailSkill.value?.last_updated ||
    detailSkill.value?.synced_at ||
    ''
)
const detailRiskLabel = computed(() => {
  const value = selectedSecurity.value?.risk_level || detailSkill.value?.risk_level || 'unknown'
  return marketplaceText(`riskLevels.${value}`, value)
})
const detailContract = computed<SkillContractMetadata | null>(() => {
  const detail = selectedDetail.value
  if (!detail) return null

  const mergedNotes = Array.from(
    new Set([...(detail.skill.contract_notes || []), ...(detail.contract_notes || [])])
  ).filter((note) => typeof note === 'string' && note.trim().length > 0)

  const contract = {
    contract_status: detail.contract_status || detail.skill.contract_status,
    contract_source: detail.contract_source || detail.skill.contract_source,
    contract_notes: mergedNotes,
  }

  if (!contract.contract_status && !contract.contract_source && !contract.contract_notes.length) {
    return null
  }

  return contract
})
const catalogCount = computed(() => totalSkills.value || skills.value.length)
const isInitialCatalogLoad = computed(
  () =>
    discoverRunning.value && catalogCount.value === 0 && !hasCompletedDiscover(discoverStatus.value)
)

const resultSubtitle = computed(() => {
  if (catalogCount.value === 0) return ''
  return marketplaceText('results.skillsCount', '{count} skills', {
    count: catalogCount.value,
  })
})

const sourceCount = computed(() => filters.value?.sources?.length || 0)
const installableCount = computed(() => filters.value?.installable?.true || 0)
const greenBadgeCount = computed(() => {
  const badge = filters.value?.risk_badges?.find((item) => item.value === 'green')
  return badge?.count || 0
})
const syncHint = computed(() =>
  marketplaceText(
    'results.syncHint',
    'The catalog refreshes automatically once per day. Use Refresh sources when you need immediate updates.'
  )
)
const discoverRunning = computed(() => discoverStatus.value?.running ?? false)
const embeddingRunning = computed(() => embeddingStatus.value?.running ?? false)
const showDiscoverProgress = computed(
  () => initializingMarketplace.value || refreshing.value || discoverRunning.value
)
const embeddingHasVisibleWork = computed(() => {
  const status = embeddingStatus.value
  if (!status) return false
  return (
    !!status.current_skill_name?.trim() ||
    !!status.current_skill_id?.trim() ||
    !!status.last_error?.trim() ||
    (status.total_skills || 0) > 0 ||
    (status.processed_skills || 0) > 0 ||
    (status.embedded_skills || 0) > 0 ||
    (status.failed_skills || 0) > 0
  )
})
const showEmbeddingProgress = computed(() => {
  const status = embeddingStatus.value
  if (!status) return false
  return embeddingRunning.value || embeddingHasVisibleWork.value
})
const showEmbeddingOnlyProgress = computed(
  () => !showDiscoverProgress.value && showEmbeddingProgress.value
)
const showResultsRefreshing = computed(() => loading.value && skills.value.length > 0)
const discoverProgressPercent = computed(() => {
  if (!showDiscoverProgress.value) return 0
  const total = discoverStatus.value?.total_sources || 0
  const processed = discoverStatus.value?.processed_sources || 0
  if (total > 0) {
    const inFlightUnits = discoverRunning.value && processed < total ? Math.min(0.45, 1 / total) : 0
    const rawPercent = Math.round(((processed + inFlightUnits) / total) * 100)
    return Math.max(processed > 0 ? 14 : 8, Math.min(discoverRunning.value ? 96 : 100, rawPercent))
  }
  return initializingMarketplace.value || refreshing.value || discoverRunning.value ? 12 : 100
})
const discoverProgressLabel = computed(() =>
  isInitialCatalogLoad.value || initializingMarketplace.value
    ? skillStoreText('status.initializing', 'Loading Skill Store')
    : skillStoreText('status.syncing', 'Syncing skill store...')
)
const discoverProgressMeta = computed(() => {
  if (isInitialCatalogLoad.value) {
    return skillStoreText('status.initializingFirstLoad', 'First-time loading can take a while')
  }
  const total = discoverStatus.value?.total_sources || 0
  const processed = discoverStatus.value?.processed_sources || 0
  if (total > 0) {
    return skillStoreText('status.initializingProgress', 'Synced {processed}/{total} sources', {
      processed,
      total,
    })
  }
  return skillStoreText('status.initializingDesc', 'Fetching skill data, this may take a moment...')
})
const discoverProgressDescription = computed(() => {
  const sourceName = discoverStatus.value?.current_source_name
  if (isInitialCatalogLoad.value) {
    if (sourceName) {
      return skillStoreText(
        'status.initializingFirstLoadSource',
        'First-time loading is slower than usual. Importing from {source} now, and you can come back later.',
        {
          source: sourceName,
        }
      )
    }
    return skillStoreText(
      'status.initializingFirstLoadDesc',
      'First-time loading can be slow. You can leave this page and come back later.'
    )
  }
  if (sourceName) {
    return skillStoreText('status.initializingSource', 'Current source: {source}', {
      source: sourceName,
    })
  }
  return skillStoreText('status.initializingDesc', 'Fetching skill data, this may take a moment...')
})
const discoverActivityFeed = computed(() => discoverActivity.value.slice(-4))
const showResultsLoading = computed(
  () =>
    (loading.value && !skills.value.length) ||
    (initializingMarketplace.value && !skills.value.length)
)
const discoverSourceProgressLabel = computed(() => discoverSourceProgress(discoverStatus.value))
const discoverPhaseTone = computed<'initializing' | 'running' | 'completed' | 'error' | 'idle'>(
  () => {
    if (discoverStatus.value?.last_error || discoverStatus.value?.phase === 'error') return 'error'
    if (discoverStatus.value?.phase === 'completed' || hasCompletedDiscover(discoverStatus.value)) {
      return 'completed'
    }
    if (initializingMarketplace.value || isInitialCatalogLoad.value) return 'initializing'
    if (refreshing.value || discoverRunning.value) return 'running'
    return 'idle'
  }
)
const discoverPhaseLabel = computed(() => {
  switch (discoverPhaseTone.value) {
    case 'error':
      return marketplaceText('progress.phaseAttention', 'Attention needed')
    case 'completed':
      return marketplaceText('progress.phaseCompleted', 'Completed')
    case 'initializing':
      return marketplaceText('progress.phaseInitializing', 'Initializing')
    case 'running':
      return marketplaceText('progress.phaseSyncing', 'Syncing now')
    default:
      return marketplaceText('progress.phaseStandby', 'Standby')
  }
})
const discoverCurrentSourceLabel = computed(() => {
  const sourceName = discoverStatus.value?.current_source_name?.trim()
  if (sourceName) return sourceName
  if (discoverPhaseTone.value === 'completed') {
    return marketplaceText('progress.allSourcesProcessed', 'All sources processed')
  }
  if (discoverRunning.value || refreshing.value || initializingMarketplace.value) {
    return marketplaceText('progress.preparingSourceQueue', 'Preparing source queue')
  }
  return marketplaceText('progress.waitingToStart', 'Waiting to start')
})
const discoverProgressFootnote = computed(() => {
  const status = discoverStatus.value
  return (
    discoverTotalsSummary(status) ||
    discoverBatchSummary(status) ||
    status?.message?.trim() ||
    marketplaceText('progress.collectingSignals', 'Collecting update signals...')
  )
})
const discoverActivityHeading = computed(() =>
  discoverRunning.value
    ? marketplaceText('progress.liveActivity', 'Live activity')
    : marketplaceText('progress.recentActivity', 'Recent activity')
)
const latestDiscoverEntry = computed(() => {
  const feed = discoverActivityFeed.value
  return feed.length ? feed[feed.length - 1]! : null
})
const previousDiscoverEntries = computed(() => {
  const feed = discoverActivityFeed.value
  if (feed.length <= 1) return []
  return feed.slice(0, -1)
})
const discoverActivityToggleLabel = computed(() =>
  marketplaceText('progress.moreEvents', 'Show {count} earlier updates', {
    count: previousDiscoverEntries.value.length,
  })
)
const discoverSummaryInline = computed(() => {
  const status = discoverStatus.value
  const result = !status?.running ? status?.result : undefined
  const processed = status?.processed_sources ?? result?.sources_processed ?? 0
  const total = status?.total_sources || 0
  const inserted = result ? result.discovered || 0 : status?.batch_inserted || 0
  const updated = result ? result.updated || 0 : status?.batch_updated || 0
  const failed = result ? result.failed || 0 : status?.batch_failed || 0

  return joinDiscoverParts([
    `${marketplaceText('progress.sourcesLabel', 'Sources')} ${
      total > 0 ? `${processed}/${total}` : formatWholeNumber(processed)
    }`,
    `${marketplaceText('progress.newSkillsLabel', 'New')} ${formatWholeNumber(inserted)}`,
    `${marketplaceText('progress.updatedSkillsLabel', 'Updated')} ${formatWholeNumber(updated)}`,
    `${marketplaceText('progress.failedSkillsLabel', 'Failed')} ${formatWholeNumber(failed)}`,
  ])
})
const discoverProgressCaption = computed(() =>
  joinDiscoverParts([discoverSourceProgressLabel.value, discoverProgressFootnote.value])
)
const embeddingProgressPercent = computed(() => {
  if (!showEmbeddingProgress.value) return 0
  const total = embeddingStatus.value?.total_skills || 0
  const processed = embeddingStatus.value?.processed_skills || 0
  if (total > 0) {
    const rawPercent = Math.round((processed / total) * 100)
    return Math.max(processed > 0 ? 10 : 6, Math.min(embeddingRunning.value ? 96 : 100, rawPercent))
  }
  return embeddingRunning.value ? 12 : 100
})
const embeddingProgressLabel = computed(() =>
  marketplaceText('embedding.progressLabel', 'Embedding skill search index')
)
const embeddingProgressMeta = computed(() => {
  const total = embeddingStatus.value?.total_skills || 0
  const processed = embeddingStatus.value?.processed_skills || 0
  if (total > 0) {
    return marketplaceText('embedding.progressMeta', 'Processed {processed}/{total} skills', {
      processed,
      total,
    })
  }
  return marketplaceText('embedding.progressIdle', 'Preparing queued skills for semantic search')
})
const embeddingPhaseTone = computed<'running' | 'completed' | 'error' | 'idle'>(() => {
  if (embeddingStatus.value?.last_error || embeddingStatus.value?.phase === 'error') return 'error'
  if (
    embeddingStatus.value?.phase === 'completed' ||
    (!!embeddingStatus.value?.finished_at && !embeddingRunning.value)
  ) {
    return 'completed'
  }
  if (embeddingRunning.value) return 'running'
  return 'idle'
})
const embeddingPhaseLabel = computed(() => {
  switch (embeddingPhaseTone.value) {
    case 'error':
      return marketplaceText('embedding.phaseAttention', 'Attention needed')
    case 'completed':
      return marketplaceText('embedding.phaseCompleted', 'Embedding complete')
    case 'running':
      return marketplaceText('embedding.phaseRunning', 'Embedding now')
    default:
      return marketplaceText('embedding.phaseStandby', 'Standby')
  }
})
const embeddingCurrentSkillLabel = computed(() => {
  const skillName = embeddingStatus.value?.current_skill_name?.trim()
  if (skillName) return skillName
  if (embeddingPhaseTone.value === 'completed') {
    return marketplaceText('embedding.allProcessed', 'All queued skills embedded')
  }
  if (embeddingRunning.value) {
    return marketplaceText('embedding.preparingQueue', 'Preparing embedding queue')
  }
  return marketplaceText('embedding.waiting', 'Waiting to embed')
})
const embeddingSummaryInline = computed(() => {
  const total = embeddingStatus.value?.total_skills || 0
  const processed = embeddingStatus.value?.processed_skills || 0
  const embedded = embeddingStatus.value?.embedded_skills || 0
  const failed = embeddingStatus.value?.failed_skills || 0

  return joinDiscoverParts([
    `${marketplaceText('embedding.totalLabel', 'Queued')} ${
      total > 0 ? formatWholeNumber(total) : formatWholeNumber(processed)
    }`,
    `${marketplaceText('embedding.processedLabel', 'Processed')} ${
      total > 0
        ? `${formatWholeNumber(processed)}/${formatWholeNumber(total)}`
        : formatWholeNumber(processed)
    }`,
    `${marketplaceText('embedding.embeddedLabel', 'Embedded')} ${formatWholeNumber(embedded)}`,
    `${marketplaceText('embedding.failedLabel', 'Failed')} ${formatWholeNumber(failed)}`,
  ])
})
const embeddingProgressCaption = computed(
  () =>
    joinDiscoverParts([
      embeddingStatus.value?.current_skill_name || embeddingStatus.value?.current_skill_id,
      embeddingStatus.value?.last_error,
    ]) ||
    marketplaceText('embedding.caption', 'Embeddings improve semantic search quality over time')
)
const sortPillOptions = computed(() => [
  { value: 'trending' as const, label: sortModeLabel('trending') },
  { value: 'newest' as const, label: sortModeLabel('newest') },
  { value: 'most_used' as const, label: sortModeLabel('most_used') },
])

function browseText(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

const closeDetailLabel = computed(() =>
  browseText('extensions.browse.closeSkillDetails', 'Close skill details')
)
const topTabAriaLabel = computed(() =>
  browseText('extensions.browse.skillCollectionFilters', 'Skill collection filters')
)

type MarketplaceAccentPalette = {
  accent: string
  accentSoft: string
  ring: string
  glow: string
  iconBg: string
  iconBorder: string
  iconText: string
}

const marketplaceAccentPalettes: MarketplaceAccentPalette[] = [
  {
    accent: '#3b82f6',
    accentSoft: 'rgba(59, 130, 246, 0.12)',
    ring: 'rgba(96, 165, 250, 0.34)',
    glow: 'rgba(59, 130, 246, 0.18)',
    iconBg: 'linear-gradient(180deg, rgba(191, 219, 254, 0.28), rgba(147, 197, 253, 0.16))',
    iconBorder: 'rgba(147, 197, 253, 0.54)',
    iconText: '#2563eb',
  },
  {
    accent: '#a855f7',
    accentSoft: 'rgba(168, 85, 247, 0.12)',
    ring: 'rgba(192, 132, 252, 0.34)',
    glow: 'rgba(168, 85, 247, 0.16)',
    iconBg: 'linear-gradient(180deg, rgba(233, 213, 255, 0.3), rgba(216, 180, 254, 0.16))',
    iconBorder: 'rgba(216, 180, 254, 0.54)',
    iconText: '#9333ea',
  },
  {
    accent: '#22c55e',
    accentSoft: 'rgba(34, 197, 94, 0.12)',
    ring: 'rgba(134, 239, 172, 0.34)',
    glow: 'rgba(34, 197, 94, 0.16)',
    iconBg: 'linear-gradient(180deg, rgba(187, 247, 208, 0.28), rgba(134, 239, 172, 0.14))',
    iconBorder: 'rgba(134, 239, 172, 0.54)',
    iconText: '#16a34a',
  },
  {
    accent: '#f97316',
    accentSoft: 'rgba(249, 115, 22, 0.12)',
    ring: 'rgba(253, 186, 116, 0.34)',
    glow: 'rgba(249, 115, 22, 0.18)',
    iconBg: 'linear-gradient(180deg, rgba(254, 215, 170, 0.3), rgba(253, 186, 116, 0.16))',
    iconBorder: 'rgba(253, 186, 116, 0.54)',
    iconText: '#ea580c',
  },
  {
    accent: '#ec4899',
    accentSoft: 'rgba(236, 72, 153, 0.12)',
    ring: 'rgba(249, 168, 212, 0.34)',
    glow: 'rgba(236, 72, 153, 0.16)',
    iconBg: 'linear-gradient(180deg, rgba(251, 207, 232, 0.28), rgba(249, 168, 212, 0.16))',
    iconBorder: 'rgba(249, 168, 212, 0.54)',
    iconText: '#db2777',
  },
  {
    accent: '#14b8a6',
    accentSoft: 'rgba(20, 184, 166, 0.12)',
    ring: 'rgba(94, 234, 212, 0.34)',
    glow: 'rgba(20, 184, 166, 0.16)',
    iconBg: 'linear-gradient(180deg, rgba(153, 246, 228, 0.28), rgba(94, 234, 212, 0.14))',
    iconBorder: 'rgba(94, 234, 212, 0.48)',
    iconText: '#0f766e',
  },
]

function hashSeed(value: string): number {
  return Array.from(value).reduce((total, char, index) => {
    return total + char.charCodeAt(0) * (index + 1)
  }, 0)
}

function accentSeed(skill?: RemoteSkill | null): string {
  return `${skill?.id || ''}:${skill?.category || ''}:${skill?.name || ''}`
}

function skillAccentPalette(skill?: RemoteSkill | null): MarketplaceAccentPalette {
  return marketplaceAccentPalettes[
    hashSeed(accentSeed(skill) || 'skill') % marketplaceAccentPalettes.length
  ]!
}

function skillAccentStyle(skill?: RemoteSkill | null): Record<string, string> {
  const palette = skillAccentPalette(skill)
  return {
    '--market-accent': palette.accent,
    '--market-accent-soft': palette.accentSoft,
    '--market-accent-ring': palette.ring,
    '--market-accent-glow': palette.glow,
    '--market-icon-bg': palette.iconBg,
    '--market-icon-border': palette.iconBorder,
    '--market-icon-fg': palette.iconText,
  }
}

watch(
  skills,
  (items) => {
    if (!items.length) {
      selectedSkillId.value = null
      selectedDetail.value = null
      detailLoading.value = false
      showDetailModal.value = false
      return
    }
    if (preferredSelectedSkillId && items.some((item) => item.id === preferredSelectedSkillId)) {
      if (selectedSkillId.value !== preferredSelectedSkillId) {
        selectedSkillId.value = preferredSelectedSkillId
      }
      preferredSelectedSkillId = null
      return
    }
    preferredSelectedSkillId = null
    if (!selectedSkillId.value || !items.some((item) => item.id === selectedSkillId.value)) {
      selectedSkillId.value = items[0]!.id
    }
  },
  { immediate: true }
)

watch(selectedSkillId, (id) => {
  if (id) {
    void fetchSkillDetail(id)
  } else {
    selectedDetail.value = null
    detailLoading.value = false
  }
})

function interpolateFallback(fallback: string, params?: Record<string, unknown>): string {
  if (!params) return fallback
  return Object.entries(params).reduce((text, [name, value]) => {
    return text.split(`{${name}}`).join(String(value ?? ''))
  }, fallback)
}

function translate(key: string, fallback: string, params?: Record<string, unknown>) {
  if (!te(key)) return interpolateFallback(fallback, params)
  return params ? t(key, params) : t(key)
}

function commonText(path: string, fallback: string, params?: Record<string, unknown>) {
  return translate(`common.${path}`, fallback, params)
}

function skillStoreText(path: string, fallback: string, params?: Record<string, unknown>) {
  return translate(`skillStore.${path}`, fallback, params)
}

function marketplaceText(path: string, fallback: string, params?: Record<string, unknown>) {
  return translate(`skillStore.marketplace.${path}`, fallback, params)
}

function sortModeLabel(mode: SkillStoreSortMode): string {
  const fallback: Record<SkillStoreSortMode, string> = {
    featured: 'Featured',
    trending: 'Trending',
    newest: 'Newest',
    most_used: 'Most used',
  }
  return marketplaceText(skillStoreSortTranslationPath(mode), fallback[mode])
}

function normalizeSearchQuery(value: string): string {
  return value.replace(/\s+/g, ' ').trim()
}

function cloneDiscoverStatus(
  status?: DiscoverStatusResponse | null
): DiscoverStatusResponse | null {
  if (!status) return null
  return {
    ...status,
    result: status.result ? { ...status.result } : undefined,
  }
}

function applyDiscoverStatus(status?: DiscoverStatusResponse | null) {
  discoverStatus.value = cloneDiscoverStatus(status)
}

function cloneEmbeddingStatus(
  status?: EmbeddingStatusResponse | null
): EmbeddingStatusResponse | null {
  if (!status) return null
  return {
    ...status,
  }
}

function applyEmbeddingStatus(status?: EmbeddingStatusResponse | null) {
  embeddingStatus.value = cloneEmbeddingStatus(status)
}

function scrollDiscoverActivityToLatest() {
  const viewport = discoverActivityViewport.value
  if (!viewport) return
  if (typeof viewport.scrollTo === 'function') {
    viewport.scrollTo({
      top: viewport.scrollHeight,
      behavior: 'smooth',
    })
    return
  }
  viewport.scrollTop = viewport.scrollHeight
}

function hasCompletedDiscover(status?: DiscoverStatusResponse | null): boolean {
  return !!status?.finished_at || (!!status?.result && !status?.running)
}

function shouldStopDiscoverPolling(requestId: number): boolean {
  return componentDisposed || latestDiscoverPollId !== requestId
}

function normalizeTags(skill?: RemoteSkill | null): string[] {
  if (!skill?.tags) return []
  if (Array.isArray(skill.tags)) return skill.tags.filter(Boolean)
  return skill.tags
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
}

function sourceLabel(skill?: RemoteSkill | null): string {
  return (
    skill?.source_name ||
    skill?.source_group ||
    skill?.source_id ||
    marketplaceText('defaultSource', 'Marketplace')
  )
}

function badgeLabelByValue(badge?: SecurityBadge | string): string {
  const normalized = (badge || 'yellow') as SecurityBadge | string
  if (normalized === 'green') return marketplaceText('badges.green', 'Security')
  if (normalized === 'red') return marketplaceText('badges.red', 'Blocked')
  return marketplaceText('badges.yellow', 'Warning')
}

function badgeLabel(skill?: RemoteSkill | null): string {
  const badge = (skill?.security_badge || 'yellow') as SecurityBadge | string
  if (badge === 'green') return marketplaceText('badges.green', 'Security')
  if (badge === 'red') return marketplaceText('badges.red', 'Blocked')
  return marketplaceText('badges.yellow', 'Warning')
}

function curatedLabelText(value?: string | null): string {
  const raw = value?.trim()
  if (!raw) return ''
  const normalized = raw.toLowerCase().replace(/[_-]+/g, ' ').replace(/\s+/g, ' ').trim()

  const known: Record<string, { key: string; fallback: string }> = {
    featured: { key: 'featured', fallback: 'Featured' },
    trending: { key: 'trending', fallback: 'Trending' },
    newest: { key: 'newest', fallback: 'Newest' },
    new: { key: 'new', fallback: 'New' },
    recommended: { key: 'recommended', fallback: 'Recommended' },
    verified: { key: 'verified', fallback: 'Verified' },
    'staff pick': { key: 'staffPick', fallback: 'Staff pick' },
    "editor's pick": { key: 'editorsPick', fallback: "Editor's pick" },
  }

  const match = known[normalized]
  if (!match) return raw
  return marketplaceText(`curatedLabels.${match.key}`, match.fallback)
}

function securityBadgeClass(skill?: RemoteSkill | null): string {
  return `badge-${skill?.security_badge || 'yellow'}`
}

function riskLabel(skill?: RemoteSkill | null): string {
  const value = skill?.risk_level || 'unknown'
  return marketplaceText(`riskLevels.${value}`, value)
}

function vulnerabilityLabel(value?: string): string {
  const fallback: Record<string, string> = {
    none: 'None',
    unknown: 'Unknown',
    suspected: 'Suspected',
    detected: 'Detected',
    not_applicable: 'Not applicable',
  }
  return marketplaceText(
    `vulnerabilityStatuses.${value || 'unknown'}`,
    fallback[value || ''] || value || 'Unknown'
  )
}

function installTypeLabel(value?: string): string {
  const fallback: Record<string, string> = {
    builtin_commands: 'Built-in commands',
    raw_skill: 'Raw Skill',
    git_repo: 'Git repository',
    source_archive: 'Source archive',
    script_package: 'Script package',
    binary_package: 'Binary package',
    manual_external: 'Manual external install',
    unknown: 'Unknown',
  }
  const normalized = value || 'unknown'
  const fallbackText: string = fallback[normalized] || value || 'Unknown'
  return marketplaceText(`installTypes.${normalized}`, fallbackText)
}

const marketplaceCategoryAliases: Record<string, string> = {
  development: 'development_tools',
  analytics: 'data_analysis',
  communication: 'communication_collaboration',
  system: 'security_compliance',
  utility: 'productivity',
  information: 'data_analysis',
  integration: 'development_tools',
  extension: 'development_tools',
}

function categoryLabel(value?: string): string {
  const normalized = value ? marketplaceCategoryAliases[value] || value : 'other'
  const fallback: Record<string, string> = {
    ai_intelligence: 'AI Intelligence',
    development_tools: 'Development Tools',
    productivity: 'Productivity',
    data_analysis: 'Data Analysis',
    content_creation: 'Content Creation',
    security_compliance: 'Security & Compliance',
    communication_collaboration: 'Communication & Collaboration',
    other: 'Other',
  }
  const fallbackText: string = fallback[normalized] || value || 'Other'
  return marketplaceText(`categories.${normalized}`, fallbackText)
}

function formatNumber(value?: number): string {
  if (!value) return '0'
  return new Intl.NumberFormat(locale.value || undefined, {
    notation: 'compact',
    compactDisplay: 'short',
    maximumFractionDigits: 1,
  }).format(value)
}

function formatWholeNumber(value?: number): string {
  return new Intl.NumberFormat(locale.value || undefined).format(value || 0)
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

function formatClockTime(value: number): string {
  return new Date(value).toLocaleTimeString(locale.value || undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function cardDescription(skill: RemoteSkill): string {
  return skill.description || skill.summary || translate('plugins.noDescription', 'No description')
}

function skillMonogram(skill?: RemoteSkill | null): string {
  const source = skill?.name?.trim() || skill?.id?.trim() || '?'
  return Array.from(source)[0]?.toLocaleUpperCase(locale.value) || '?'
}

function skillVersionLabel(skill?: RemoteSkill | null): string {
  return formatVersionLabel(
    skill?.version || skill?.latest_version || selectedDetail.value?.version?.version
  )
}

function visibleSkillTags(skill?: RemoteSkill | null, limit = 2): string[] {
  return normalizeTags(skill).slice(0, limit)
}

function openSkillSource(skill?: RemoteSkill | null) {
  const url = skill?.homepage || skill?.download_url || skill?.source_url
  if (url) {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

function buildSearchParams(): MarketSearchParams {
  const params: MarketSearchParams = {
    q: currentSearchQuery.value || undefined,
    category: selectedCategory.value !== 'all' ? selectedCategory.value : undefined,
    categories: selectedCategory.value !== 'all' ? selectedCategory.value : undefined,
    sources: selectedSource.value !== 'all' ? selectedSource.value : undefined,
    sort: sortMode.value,
    page: page.value,
    page_size: pageSize,
    semantic: true,
    risk_badges: selectedRisk.value !== 'all' ? selectedRisk.value : undefined,
  }
  if (sortMode.value === 'featured' && !showDiscoverProgress.value) {
    params.curated = true
  }
  return params
}

function clearSkillAdvice() {
  latestAdviceRequestId += 1
  adviceLoading.value = false
  skillAdvice.value = null
}

async function fetchSkillAdvice(options?: { force?: boolean }) {
  const query = currentSearchQuery.value
  if (!query) {
    clearSkillAdvice()
    return
  }
  if (
    !options?.force &&
    activeSkillAdvice.value &&
    normalizeSearchQuery(activeSkillAdvice.value.query) === query
  ) {
    return
  }

  const requestId = ++latestAdviceRequestId
  adviceLoading.value = true

  try {
    const response = await skillApi.adviseMarket({ query })
    if (requestId !== latestAdviceRequestId) return
    skillAdvice.value = response.data
  } catch (err) {
    if (requestId !== latestAdviceRequestId) return
    skillAdvice.value = {
      query,
      need_store_search: false,
      search_error:
        err instanceof Error
          ? err.message
          : marketplaceText('advisor.fetchError', 'Failed to get skill suggestions'),
    }
  } finally {
    if (requestId === latestAdviceRequestId) {
      adviceLoading.value = false
    }
  }
}

function normalizeSkill(skill: RemoteSkill): RemoteSkill {
  return {
    ...skill,
    version: skill.version || skill.latest_version,
    tags: normalizeTags(skill),
    installed: !!skill.installed,
  }
}

const currentSearchQuery = computed(() => normalizeSearchQuery(searchQuery.value))

const activeSkillAdvice = computed<MarketplaceAdviceResponse | null>(() => {
  const advice = skillAdvice.value
  if (!advice) return null
  return normalizeSearchQuery(advice.query) === currentSearchQuery.value ? advice : null
})

const advisorSuggestedQueries = computed(() => {
  const advice = activeSkillAdvice.value
  const current = currentSearchQuery.value.toLowerCase()
  const seen = new Set<string>()
  return (advice?.search_queries || [])
    .map((value) => normalizeSearchQuery(value))
    .filter((value) => {
      const normalized = value.toLowerCase()
      if (!value || normalized === current || seen.has(normalized)) return false
      seen.add(normalized)
      return true
    })
    .slice(0, 4)
})

const advisorCapabilityTags = computed(() => {
  const seen = new Set<string>()
  return (activeSkillAdvice.value?.capability_tags || [])
    .map((value) => value.trim())
    .filter((value) => {
      const normalized = value.toLowerCase()
      if (!normalized || seen.has(normalized)) return false
      seen.add(normalized)
      return true
    })
    .slice(0, 6)
})

const advisorRecommendedSkills = computed<RemoteSkill[]>(() => {
  const advice = activeSkillAdvice.value
  const results = advice?.results || []
  if (!results.length) return []

  const normalized = results.map((item) => normalizeSkill(item.skill))
  const byID = new Map(normalized.map((skill) => [skill.id, skill] as const))
  const ordered: RemoteSkill[] = []

  for (const id of advice?.recommended_ids || []) {
    const skill = byID.get(id)
    if (skill) ordered.push(skill)
  }
  for (const skill of normalized) {
    if (!ordered.some((item) => item.id === skill.id)) {
      ordered.push(skill)
    }
  }
  return ordered.slice(0, 3)
})

const showSkillAdvice = computed(
  () => !!currentSearchQuery.value && (adviceLoading.value || !!activeSkillAdvice.value)
)

const advisorInstalledSkill = computed(
  () => activeSkillAdvice.value?.installed_decision?.selected_skill?.trim() || ''
)

const advisorFeedbackNote = computed(
  () => activeSkillAdvice.value?.search_error || activeSkillAdvice.value?.skill_selector_error || ''
)

const advisorTitle = computed(() => {
  if (adviceLoading.value && !activeSkillAdvice.value) {
    return marketplaceText('advisor.loadingTitle', 'Analyzing this task')
  }
  if (advisorInstalledSkill.value && !activeSkillAdvice.value?.need_store_search) {
    return marketplaceText('advisor.installedTitle', 'An installed skill may already fit')
  }
  if (advisorRecommendedSkills.value.length) {
    return marketplaceText('advisor.recommendTitle', 'Recommended skills for this task')
  }
  if (advisorSuggestedQueries.value.length) {
    return marketplaceText('advisor.queryTitle', 'Suggested search angles')
  }
  if (advisorCapabilityTags.value.length) {
    return marketplaceText('advisor.capabilityTitle', 'Capability tags to look for')
  }
  return marketplaceText('advisor.emptyTitle', 'No suggestions yet')
})

const advisorDescription = computed(() => {
  if (adviceLoading.value && !activeSkillAdvice.value) {
    return marketplaceText('advisor.loadingBody', 'Generating keywords and hybrid search hints...')
  }
  if (advisorInstalledSkill.value && !activeSkillAdvice.value?.need_store_search) {
    return marketplaceText(
      'advisor.installedBody',
      'The installed skill selector is confident enough, so store search is optional.'
    )
  }
  if (advisorRecommendedSkills.value.length) {
    return marketplaceText(
      'advisor.recommendBody',
      'These recommendations come from real marketplace entries, reranked from keyword and semantic matches.'
    )
  }
  if (advisorSuggestedQueries.value.length) {
    return marketplaceText(
      'advisor.queryBody',
      'Try these search phrases in the marketplace if the first query is too broad.'
    )
  }
  if (advisorCapabilityTags.value.length) {
    return marketplaceText(
      'advisor.capabilityBody',
      'Use these capability tags when you browse or install from the skill store.'
    )
  }
  return marketplaceText(
    'advisor.emptyBody',
    'Keep refining your request and suggestions will appear here.'
  )
})

function discoverSourceProgress(status?: DiscoverStatusResponse | null): string {
  const total = status?.total_sources || 0
  const processed = status?.processed_sources || 0
  if (total > 0) {
    return marketplaceText('progress.sourcesProgress', '{processed}/{total} sources', {
      processed,
      total,
    })
  }
  if (processed > 0) {
    return marketplaceText('progress.sourcesProcessed', '{processed} sources processed', {
      processed,
    })
  }
  return ''
}

function joinDiscoverParts(parts: Array<string | null | undefined>): string {
  return parts
    .map((part) => part?.trim())
    .filter((part): part is string => !!part)
    .join(' · ')
}

function discoverBatchSummary(status?: DiscoverStatusResponse | null): string {
  if (!status) return ''
  const parts: string[] = []
  if (status.batch_inserted) {
    parts.push(
      marketplaceText('progress.batchInserted', '+{count} new', { count: status.batch_inserted })
    )
  }
  if (status.batch_updated) {
    parts.push(
      marketplaceText('progress.batchUpdated', '{count} updated', { count: status.batch_updated })
    )
  }
  if (status.batch_failed) {
    parts.push(
      marketplaceText('progress.batchFailed', '{count} failed', { count: status.batch_failed })
    )
  }
  return parts.join(' · ')
}

function discoverTotalsSummary(status?: DiscoverStatusResponse | null): string {
  const result = status?.result
  if (!result) return ''
  const parts: string[] = []
  if (result.discovered) {
    parts.push(
      marketplaceText('progress.batchInserted', '+{count} new', { count: result.discovered })
    )
  }
  if (result.updated) {
    parts.push(
      marketplaceText('progress.batchUpdated', '{count} updated', { count: result.updated })
    )
  }
  if (result.failed) {
    parts.push(marketplaceText('progress.batchFailed', '{count} failed', { count: result.failed }))
  }
  return parts.join(' · ')
}

function recordDiscoverActivity(
  status?: DiscoverStatusResponse | null,
  phaseOverride?: 'started' | 'batch' | 'source_complete' | 'completed' | 'error' | 'status'
) {
  if (!status) return
  const phase =
    phaseOverride ||
    ((status.phase as 'started' | 'batch' | 'source_complete' | 'completed' | 'error') ??
      (status.running ? 'status' : 'completed'))
  const sourceName = status.current_source_name || marketplaceText('progress.catalog', 'catalog')
  const progressLabel = discoverSourceProgress(status)
  const batchSummary = discoverBatchSummary(status)
  const totalSummary = discoverTotalsSummary(status)

  let title = marketplaceText('progress.refreshingCatalog', 'Refreshing catalog')
  let detail = joinDiscoverParts([progressLabel, batchSummary || totalSummary])

  if (phase === 'started') {
    title = marketplaceText('progress.started', 'Started refreshing sources')
    detail = joinDiscoverParts([
      progressLabel,
      marketplaceText('progress.waitingFirstBatch', 'Waiting for the first batch...'),
    ])
  } else if (phase === 'batch') {
    title = marketplaceText('progress.processingSource', 'Processing {source}', {
      source: sourceName,
    })
    detail = joinDiscoverParts([progressLabel, batchSummary, totalSummary])
  } else if (phase === 'source_complete') {
    title = marketplaceText('progress.completedSource', 'Finished {source}', { source: sourceName })
    detail = joinDiscoverParts([progressLabel, totalSummary || batchSummary])
  } else if (phase === 'completed') {
    title = marketplaceText('progress.completed', 'Catalog refresh complete')
    detail = joinDiscoverParts([progressLabel, totalSummary])
  } else if (phase === 'error') {
    title = marketplaceText('progress.failed', 'Catalog refresh failed')
    detail = joinDiscoverParts([progressLabel, status.last_error])
  } else if (phase === 'status') {
    title = marketplaceText('progress.processingSource', 'Processing {source}', {
      source: sourceName,
    })
    detail =
      joinDiscoverParts([progressLabel, batchSummary || totalSummary]) ||
      marketplaceText('progress.waitingNextStep', 'Waiting for the next update...')
  }

  const signature = [
    phase,
    title,
    detail,
    status.current_source_name || '',
    status.processed_sources || 0,
    status.total_sources || 0,
    status.batch_inserted || 0,
    status.batch_updated || 0,
    status.batch_failed || 0,
    status.result?.discovered || 0,
    status.result?.updated || 0,
    status.result?.failed || 0,
    status.finished_at || '',
    status.last_error || '',
  ].join('|')

  if (signature === lastDiscoverActivitySignature) return
  lastDiscoverActivitySignature = signature

  discoverActivity.value = [
    ...discoverActivity.value.slice(-15),
    {
      id: ++discoverActivityId,
      phase,
      title,
      detail,
      timestamp: Date.now(),
    },
  ]

  void nextTick(() => {
    scrollDiscoverActivityToLatest()
  })
}

async function fetchFilters() {
  try {
    const response = await skillApi.filtersMarket()
    filters.value = response.data
  } catch (err) {
    console.warn('Failed to load skill filters', err)
  }
}

async function fetchSkills(reset = true, options?: { preserveVisible?: boolean }) {
  const requestId = ++latestSkillsRequestId
  const preserveVisible = !!options?.preserveVisible && reset && skills.value.length > 0
  if (reset) {
    preferredSelectedSkillId = selectedSkillId.value
    page.value = 1
    totalPages.value = 1
    if (!preserveVisible) {
      skills.value = []
    }
    loading.value = true
  } else {
    if (loadingMore.value || page.value >= totalPages.value) return
    loadingMore.value = true
    page.value += 1
  }
  error.value = null

  try {
    const response = await skillApi.searchMarket(buildSearchParams())
    if (requestId !== latestSkillsRequestId) return
    const payload = response.data
    const incoming = (payload.skills || []).map((item) => normalizeSkill(item.skill))
    skills.value = reset ? incoming : [...skills.value, ...incoming]
    totalSkills.value = payload.total || incoming.length
    totalPages.value = payload.total_pages || 1
  } catch (err) {
    if (requestId !== latestSkillsRequestId) return
    error.value =
      err instanceof Error ? err.message : skillStoreText('fetchError', 'Failed to fetch skills')
  } finally {
    if (requestId === latestSkillsRequestId) {
      loading.value = false
      loadingMore.value = false
    }
  }
}

async function fetchSkillDetail(id: string) {
  const requestId = ++latestDetailRequestId
  detailLoading.value = true
  selectedDetail.value = null
  try {
    const response = await skillApi.getMarketplaceSkill(id)
    if (requestId !== latestDetailRequestId || selectedSkillId.value !== id) {
      return
    }
    selectedDetail.value = response.data
  } catch (err) {
    if (requestId !== latestDetailRequestId || selectedSkillId.value !== id) {
      return
    }
    selectedDetail.value = null
    console.warn('Failed to load marketplace skill detail', { id, err })
  } finally {
    if (requestId === latestDetailRequestId && selectedSkillId.value === id) {
      detailLoading.value = false
    }
  }
}

function selectSkill(skill: RemoteSkill) {
  selectedSkillId.value = skill.id
  showDetailModal.value = true
}

function closeSkillDetail() {
  showDetailModal.value = false
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function visibleResultsRefreshSignature(status?: DiscoverStatusResponse | null): string {
  if (!status) return ''
  return [
    status.started_at || '',
    status.running ? 'running' : 'completed',
    status.phase || '',
    status.processed_sources || 0,
    status.total_sources || 0,
    status.current_source_id || '',
    status.current_source_name || '',
    status.batch_inserted || 0,
    status.batch_updated || 0,
    status.batch_failed || 0,
    status.result?.sources_processed || 0,
    status.result?.discovered || 0,
    status.result?.updated || 0,
    status.result?.failed || 0,
    status.finished_at || '',
  ].join('|')
}

function hasVisibleResultsRefreshProgress(status?: DiscoverStatusResponse | null): boolean {
  if (!status) return false
  return (
    status.phase === 'batch' ||
    status.phase === 'source_complete' ||
    status.phase === 'completed' ||
    (status.processed_sources || 0) > 0 ||
    (status.batch_inserted || 0) > 0 ||
    (status.batch_updated || 0) > 0 ||
    (status.batch_failed || 0) > 0 ||
    (status.result?.discovered || 0) > 0 ||
    (status.result?.updated || 0) > 0 ||
    (status.result?.failed || 0) > 0 ||
    !!status.finished_at
  )
}

function maybeScheduleVisibleResultsRefresh(
  status?: DiscoverStatusResponse | null,
  options?: { includeCompleted?: boolean }
) {
  if (!status || !hasVisibleResultsRefreshProgress(status)) return
  if (!options?.includeCompleted && !status.running) return
  const signature = visibleResultsRefreshSignature(status)
  if (!signature || signature === lastVisibleResultsRefreshSignature) return
  lastVisibleResultsRefreshSignature = signature
  scheduleVisibleResultsRefresh()
}

function shouldProbeVisibleResults(status?: DiscoverStatusResponse | null): boolean {
  return !!status?.running && skills.value.length === 0
}

async function waitForDiscoverCompletion(initial?: DiscoverStatusResponse | null) {
  const requestId = ++latestDiscoverPollId
  let status = initial ?? null
  if (status) {
    recordDiscoverActivity(
      status,
      (status.phase as 'started' | 'batch' | 'source_complete' | 'completed' | 'error') || 'status'
    )
  }
  while (true) {
    if (shouldStopDiscoverPolling(requestId)) return null
    if (!status || status.running) {
      const response = await skillApi.discoverStatus()
      if (shouldStopDiscoverPolling(requestId)) return null
      status = response.data
    }
    applyDiscoverStatus(status)
    recordDiscoverActivity(
      status,
      (status.phase as 'started' | 'batch' | 'source_complete' | 'completed' | 'error') ||
        (status.running ? 'status' : 'completed')
    )
    maybeScheduleVisibleResultsRefresh(status)
    if (shouldProbeVisibleResults(status)) {
      scheduleVisibleResultsRefresh()
    }
    if (!status.running) {
      if (status.last_error) {
        throw new Error(status.last_error)
      }
      return status
    }
    await sleep(DISCOVER_POLL_INTERVAL_MS)
    status = null
  }
}

async function loadMarketplaceCatalog() {
  await Promise.all([fetchFilters(), fetchSkills(true)])
}

async function loadMarketplaceCatalogPreservingResults() {
  await Promise.all([fetchFilters(), fetchSkills(true, { preserveVisible: true })])
}

async function continueMarketplaceDiscover(initial?: DiscoverStatusResponse | null) {
  initializingMarketplace.value = true
  try {
    let status = initial ?? null
    if (!status || !status.running) {
      const refreshResponse = await skillApi.discoverRefresh()
      if (componentDisposed) return
      status = refreshResponse.data
      applyDiscoverStatus(status)
      recordDiscoverActivity(
        status,
        (status.phase as 'started' | 'batch' | 'source_complete' | 'completed' | 'error') ||
          'started'
      )
    }
    await waitForDiscoverCompletion(status)
    if (componentDisposed) return
    await loadMarketplaceCatalogPreservingResults()
  } catch (err) {
    if (componentDisposed) return
    error.value =
      err instanceof Error ? err.message : skillStoreText('fetchError', 'Failed to fetch skills')
  } finally {
    if (!componentDisposed) {
      initializingMarketplace.value = false
    }
  }
}

async function initializeMarketplaceView() {
  loading.value = true
  error.value = null

  try {
    const [discoverResponse, embeddingResponse] = await Promise.all([
      skillApi.discoverStatus(),
      skillApi.embeddingStatus(),
    ])
    if (componentDisposed) return

    const status = discoverResponse.data
    applyDiscoverStatus(status)
    applyEmbeddingStatus(embeddingResponse.data)
    await loadMarketplaceCatalog()

    if (componentDisposed) return
    if (status.running) {
      void continueMarketplaceDiscover(status)
    } else if (!hasCompletedDiscover(status)) {
      void continueMarketplaceDiscover()
    }
  } catch (err) {
    if (componentDisposed) return
    error.value =
      err instanceof Error ? err.message : skillStoreText('fetchError', 'Failed to fetch skills')
  } finally {
    if (!componentDisposed) {
      loading.value = false
    }
  }
}

async function triggerRefresh() {
  refreshing.value = true
  error.value = null
  try {
    const response = await skillApi.discoverRefresh()
    if (componentDisposed) return
    applyDiscoverStatus(response.data)
    recordDiscoverActivity(
      response.data,
      (response.data.phase as 'started' | 'batch' | 'source_complete' | 'completed' | 'error') ||
        'started'
    )
    void (async () => {
      try {
        await waitForDiscoverCompletion(response.data)
        if (componentDisposed) return
        await loadMarketplaceCatalogPreservingResults()
      } catch (err) {
        if (componentDisposed) return
        error.value =
          err instanceof Error
            ? err.message
            : skillStoreText('fetchError', 'Failed to fetch skills')
      }
    })()
  } catch (err) {
    if (componentDisposed) return
    error.value =
      err instanceof Error ? err.message : skillStoreText('fetchError', 'Failed to fetch skills')
  } finally {
    if (!componentDisposed) {
      refreshing.value = false
    }
  }
}

async function refreshVisibleResults() {
  if (componentDisposed) return
  const visibleCount = skills.value.length > 0 ? skills.value.length : pageSize
  try {
    const response = await skillApi.searchMarket({
      ...buildSearchParams(),
      page: 1,
      page_size: visibleCount,
    })
    if (componentDisposed) return
    const payload = response.data
    const incoming = (payload.skills || []).map((item) => normalizeSkill(item.skill))
    preferredSelectedSkillId = selectedSkillId.value
    skills.value = incoming
    totalSkills.value = payload.total || incoming.length
    totalPages.value = Math.max(1, Math.ceil(Math.max(totalSkills.value, 0) / pageSize))
    page.value = incoming.length ? Math.max(1, Math.ceil(incoming.length / pageSize)) : 1
  } catch (err) {
    console.warn('Failed to refresh visible skill results', err)
  }
}

function scheduleVisibleResultsRefresh() {
  if (refreshVisibleResultsTimer) {
    window.clearTimeout(refreshVisibleResultsTimer)
  }
  refreshVisibleResultsTimer = window.setTimeout(() => {
    refreshVisibleResultsTimer = null
    void refreshVisibleResults()
  }, 350)
}

function handleDiscoverProgressEvent(data: Partial<DiscoverStatusResponse> & { phase?: string }) {
  if (componentDisposed) return
  applyDiscoverStatus(data as DiscoverStatusResponse)
  recordDiscoverActivity(
    data as DiscoverStatusResponse,
    (data.phase as 'started' | 'batch' | 'source_complete' | 'completed' | 'error') ||
      ((data.running ?? false) ? 'status' : 'completed')
  )
  if (shouldProbeVisibleResults(data as DiscoverStatusResponse)) {
    scheduleVisibleResultsRefresh()
  }
  if (data.phase === 'batch' || data.phase === 'source_complete' || data.phase === 'completed') {
    maybeScheduleVisibleResultsRefresh(data as DiscoverStatusResponse, {
      includeCompleted: data.phase === 'completed',
    })
    if (data.phase === 'completed') {
      void fetchFilters()
    }
  }
}

function handleEmbeddingProgressEvent(data: Partial<EmbeddingStatusResponse>) {
  if (componentDisposed) return
  applyEmbeddingStatus(data as EmbeddingStatusResponse)
  if (data.phase === 'completed' && currentSearchQuery.value) {
    scheduleVisibleResultsRefresh()
  }
}

function handleSearch(payload?: Event | { forceAdvice?: boolean }) {
  const forceAdvice = !!(
    payload &&
    typeof payload === 'object' &&
    'forceAdvice' in payload &&
    payload.forceAdvice
  )
  void fetchSkills(true)
  if (currentSearchQuery.value) {
    void fetchSkillAdvice({ force: forceAdvice })
    return
  }
  clearSkillAdvice()
}

function handleSortModeChange(value: SkillStoreSortMode) {
  sortMode.value = value
  handleSearch()
}

function applyAdvisorQuery(query: string) {
  searchQuery.value = query
  handleSearch({ forceAdvice: true })
}

function resetFilters() {
  searchQuery.value = ''
  selectedCategory.value = 'all'
  selectedSource.value = 'all'
  selectedRisk.value = 'all'
  sortMode.value = defaultSortMode
  clearSkillAdvice()
  handleSearch()
}

function clearSearch() {
  searchQuery.value = ''
  clearSkillAdvice()
  handleSearch()
}

async function installSkill(skill: RemoteSkill, ackRisk = false) {
  if (installingSkillId.value === skill.id) return
  if (!skill.installable) {
    openSkillSource(skill)
    return
  }
  const badge = skill.security_badge ?? 'yellow'
  if (badge === 'red') {
    error.value = marketplaceText(
      'messages.blockedByPolicy',
      'This skill is blocked by the security policy.'
    )
    return
  }
  if (badge === 'yellow' && !ackRisk) {
    pendingRiskSkill.value = skill
    return
  }

  installingSkillId.value = skill.id
  error.value = null

  try {
    await skillApi.installMarket({
      id: skill.id,
      ack_risk: ackRisk || badge === 'yellow',
    })
    const current = skills.value.find((item) => item.id === skill.id)
    if (current) current.installed = true
    if (selectedDetail.value?.skill.id === skill.id) {
      selectedDetail.value = {
        ...selectedDetail.value,
        installed: true,
        skill: {
          ...selectedDetail.value.skill,
          installed: true,
        },
      }
    }
    pendingRiskSkill.value = null
  } catch (err) {
    const message =
      getErrorMessage(err) || skillStoreText('installError', 'Failed to install skill')
    const normalized = message.toLowerCase()
    if (
      normalized.includes('risk acknowledgement') ||
      normalized.includes('ack_risk') ||
      normalized.includes('ack risk')
    ) {
      pendingRiskSkill.value = skill
    } else if (
      normalized.includes('blocked by security policy') ||
      normalized.includes('security policy')
    ) {
      // Check if it's specifically a medium risk block
      if (normalized.includes('medium risk')) {
        error.value = marketplaceText(
          'messages.blockedByPolicyMediumRisk',
          'This skill is blocked by security policy (medium risk).'
        )
      } else {
        error.value = marketplaceText(
          'messages.blockedByPolicy',
          'This skill is blocked by the security policy.'
        )
      }
    } else {
      error.value = message
    }
  }
  installingSkillId.value = null
}

function closeRiskModal() {
  pendingRiskSkill.value = null
}

function reviewRiskSkill() {
  const skill = pendingRiskSkill.value
  if (!skill) return
  pendingRiskSkill.value = null
  selectSkill(skill)
}

function confirmRiskInstall() {
  if (!pendingRiskSkill.value) return
  void installSkill(pendingRiskSkill.value, true)
}

function evidenceGroupLabel(type: string) {
  const fallback: Record<string, string> = {
    dangerous_command: 'Dangerous command',
    prompt_injection: 'Prompt injection',
    secret: 'Secrets',
    permission: 'Permissions',
    dependency_manifest: 'Dependencies',
    binary_artifact: 'Binary artifact',
    data_exfiltration: 'Data exfiltration',
    cmd_injection: 'Command injection',
  }
  return marketplaceText(`evidenceTypes.${type}`, fallback[type] || type)
}

function detailStat(value: boolean | undefined, positive = 'Yes', negative = 'No') {
  return value ? commonText('yes', positive) : commonText('no', negative)
}

function detailBadgeClass(kind: SecurityBadge | string | undefined) {
  if (kind === 'green' || kind === 'yellow' || kind === 'red') return `badge-${kind}`
  return 'badge-neutral'
}

function optionLabel(
  options: Array<{ value: string; label: string }> | undefined,
  value: string
): string {
  return options?.find((option) => option.value === value)?.label || value
}

function installHint(skill?: RemoteSkill | null): string {
  if (!skill?.installable) {
    return marketplaceText('detail.sourceOnly', 'Catalog entry only, install from source')
  }
  if (skill.installed) {
    return marketplaceText('detail.installedOnDevice', 'Installed on this device')
  }
  if (skill.security_badge === 'red') {
    return marketplaceText('detail.installBlocked', 'Blocked by security policy')
  }
  if (skill.security_badge === 'yellow') {
    return marketplaceText('detail.reviewRecommended', 'Review security summary before installing')
  }
  return marketplaceText('detail.readyToInstall', 'Ready to install')
}

function activeSkillSignals(skill?: RemoteSkill | null): string[] {
  if (!skill) return []
  const items: string[] = []
  if (
    skill.vulnerability_status &&
    skill.vulnerability_status !== 'none' &&
    skill.vulnerability_status !== 'not_applicable'
  ) {
    items.push(
      `${marketplaceText('filters.vulnerabilities', 'Vulnerabilities')}: ${vulnerabilityLabel(skill.vulnerability_status)}`
    )
  }
  if (skill.has_prompt_injection) {
    items.push(marketplaceText('filters.promptInjection', 'Prompt injection risk'))
  }
  if (skill.has_shell_injection) {
    items.push(marketplaceText('filters.shellInjection', 'Command injection risk'))
  }
  if (skill.has_data_exfiltration) {
    items.push(marketplaceText('filters.dataExfiltration', 'Data exfiltration risk'))
  }
  if (skill.has_binary) {
    items.push(marketplaceText('security.binary', 'Binary'))
  }
  return items
}

function cardSignalSummary(skill?: RemoteSkill | null): string {
  const signals = activeSkillSignals(skill)
  if (!signals.length) return installHint(skill)
  const visible = signals.slice(0, 2)
  if (signals.length > visible.length) {
    visible.push(
      marketplaceText('detail.moreSignals', '+{count} more', {
        count: signals.length - visible.length,
      })
    )
  }
  return visible.join(' · ')
}

const categoryOptions = computed(() => filters.value?.categories || [])
const sourceOptions = computed(() => filters.value?.sources || [])
const riskOptions = computed(() => filters.value?.risk_badges || [])
const activeFilterLabels = computed(() => {
  const labels: string[] = []
  const query = normalizeSearchQuery(searchQuery.value)

  if (query) {
    labels.push(`${commonText('search', 'Search')}: ${query}`)
  }
  if (selectedCategory.value !== 'all') {
    labels.push(
      `${marketplaceText('filters.category', 'Category')}: ${categoryLabel(selectedCategory.value)}`
    )
  }
  if (selectedSource.value !== 'all') {
    labels.push(
      `${marketplaceText('filters.source', 'Source')}: ${optionLabel(sourceOptions.value, selectedSource.value)}`
    )
  }
  if (selectedRisk.value !== 'all') {
    labels.push(
      `${marketplaceText('filters.security', 'Security')}: ${badgeLabelByValue(selectedRisk.value)}`
    )
  }
  if (sortMode.value !== defaultSortMode) {
    labels.push(`${commonText('filter', 'Filter')}: ${sortModeLabel(sortMode.value)}`)
  }

  return labels
})
const pendingRiskSignals = computed(() => {
  if (!pendingRiskSkill.value) return []
  return activeSkillSignals(pendingRiskSkill.value)
})

const selectedSecuritySignals = computed(() => {
  const report = selectedSecurity.value
  if (!report) return []
  const items: Array<{ label: string; className: string }> = []
  if (
    report.vulnerability_status &&
    report.vulnerability_status !== 'none' &&
    report.vulnerability_status !== 'not_applicable'
  ) {
    items.push({
      label: `${marketplaceText('filters.vulnerabilities', 'Vulnerabilities')}: ${vulnerabilityLabel(report.vulnerability_status)}`,
      className:
        report.vulnerability_status === 'detected' ? 'signal-detail-alert' : 'signal-detail-warn',
    })
  }
  if (report.has_prompt_injection) {
    items.push({
      label: marketplaceText('filters.promptInjection', 'Prompt injection risk'),
      className: 'signal-detail-alert',
    })
  }
  if (report.has_shell_injection) {
    items.push({
      label: marketplaceText('filters.shellInjection', 'Command injection risk'),
      className: 'signal-detail-alert',
    })
  }
  if (report.has_data_exfiltration) {
    items.push({
      label: marketplaceText('filters.dataExfiltration', 'Data exfiltration risk'),
      className: 'signal-detail-alert',
    })
  }
  if (report.install_surface?.has_binary) {
    items.push({
      label: marketplaceText('security.binary', 'Binary'),
      className: 'signal-detail-warn',
    })
  }
  if (report.install_surface?.has_scripts) {
    items.push({
      label: marketplaceText('security.scripts', 'Scripts'),
      className: 'signal-detail-warn',
    })
  }
  return items
})

const visibleSecurityEvidence = computed(() => (selectedSecurity.value?.evidence || []).slice(0, 4))
const hiddenSecurityEvidenceCount = computed(() => {
  const total = selectedSecurity.value?.evidence?.length || 0
  return Math.max(0, total - visibleSecurityEvidence.value.length)
})

watch(
  () => normalizeSearchQuery(props.initialSearchQuery || ''),
  (query) => {
    if (!marketplaceInitialized) {
      searchQuery.value = query
      return
    }
    if (query === currentSearchQuery.value) return
    searchQuery.value = query
    handleSearch({ forceAdvice: true })
  }
)

onMounted(() => {
  onSSEEvent('skill.market.discover.progress', handleDiscoverProgressEvent)
  onSSEEvent('skill.market.embedding.progress', handleEmbeddingProgressEvent)
  void initializeMarketplaceView().finally(() => {
    if (componentDisposed) return
    marketplaceInitialized = true
    if (currentSearchQuery.value) {
      void fetchSkillAdvice({ force: true })
    }
  })
})

onBeforeUnmount(() => {
  componentDisposed = true
  latestDiscoverPollId += 1
  offSSEEvent('skill.market.discover.progress', handleDiscoverProgressEvent)
  offSSEEvent('skill.market.embedding.progress', handleEmbeddingProgressEvent)
  if (refreshVisibleResultsTimer) {
    window.clearTimeout(refreshVisibleResultsTimer)
    refreshVisibleResultsTimer = null
  }
})
</script>

<template>
  <div class="skill-tab skill-store-tab skill-store-aggregator">
    <section class="toolbar-panel hero-panel dashboard-card-surface">
      <div class="hero-headline">
        <div class="hero-copy">
          <span class="hero-kicker">{{
            marketplaceText('hero.kicker', 'Skills marketplace')
          }}</span>
          <h2>{{ marketplaceText('hero.title', 'Discover, review, and install agent skills') }}</h2>
          <p>
            {{
              marketplaceText(
                'hero.description',
                'Browse multi-source skills, inspect security posture, and only install what fits your workspace.'
              )
            }}
          </p>
        </div>
      </div>

      <div class="hero-summary-row">
        <span v-if="!isInitialCatalogLoad && catalogCount > 0" class="summary-pill">
          {{
            marketplaceText('results.skillsCount', '{count} skills', {
              count: catalogCount,
            })
          }}
        </span>
        <span v-if="!isInitialCatalogLoad && installableCount > 0" class="summary-pill">
          {{
            marketplaceText('results.installableCount', '{count} installable', {
              count: installableCount,
            })
          }}
        </span>
        <span v-if="!isInitialCatalogLoad && greenBadgeCount > 0" class="summary-pill">
          {{
            marketplaceText('results.safeCount', 'Security {count}', {
              count: greenBadgeCount,
            })
          }}
        </span>
        <span v-if="!isInitialCatalogLoad && sourceCount > 0" class="summary-pill">
          {{
            marketplaceText('results.sources', 'Sources {count}', {
              count: sourceCount,
            })
          }}
        </span>
        <span class="toolbar-note">{{ syncHint }}</span>
      </div>

      <div class="toolbar-search-row">
        <SemanticSearchField
          v-model="searchQuery"
          class="hero-search-field"
          :placeholder="
            translate(
              'skillStore.filters.searchSkillsPlaceholder',
              'Search skills, tags, permissions, or risks'
            )
          "
          :clear-label="translate('common.clear', 'Clear')"
          @clear="clearSearch"
          @submit-shortcut="handleSearch({ forceAdvice: true })"
        />
        <div class="hero-actions">
          <button
            class="btn-ghost"
            :disabled="refreshing || discoverRunning"
            @click="triggerRefresh"
          >
            {{
              refreshing
                ? commonText('refreshing', 'Refreshing...')
                : marketplaceText('actions.refreshSources', 'Refresh sources')
            }}
          </button>
        </div>
      </div>

      <div class="toolbar-controls">
        <div class="sort-pills" role="tablist" :aria-label="topTabAriaLabel">
          <button
            v-for="option in sortPillOptions"
            :key="option.value"
            type="button"
            :class="['sort-pill', { active: sortMode === option.value }]"
            @click="handleSortModeChange(option.value)"
          >
            {{ option.label }}
          </button>
        </div>

        <div class="toolbar-focus-actions">
          <button
            v-if="activeFilterLabels.length"
            type="button"
            class="btn-text"
            @click="resetFilters"
          >
            {{ marketplaceText('actions.clearFilters', 'Clear filters') }}
          </button>
        </div>
      </div>

      <div class="filter-grid">
        <label class="filter-field">
          <span>{{ marketplaceText('filters.category', 'Category') }}</span>
          <select v-model="selectedCategory" class="filter-select" @change="handleSearch">
            <option value="all">
              {{ skillStoreText('filters.allCategories', 'All Categories') }}
            </option>
            <option v-for="option in categoryOptions" :key="option.value" :value="option.value">
              {{ categoryLabel(option.value) }} ({{ option.count }})
            </option>
          </select>
        </label>

        <label class="filter-field">
          <span>{{ marketplaceText('filters.source', 'Source') }}</span>
          <select v-model="selectedSource" class="filter-select" @change="handleSearch">
            <option value="all">{{ skillStoreText('filters.allSources', 'All Sources') }}</option>
            <option v-for="option in sourceOptions" :key="option.value" :value="option.value">
              {{ option.label }} ({{ option.count }})
            </option>
          </select>
        </label>

        <label class="filter-field">
          <span>{{ marketplaceText('filters.security', 'Security') }}</span>
          <select v-model="selectedRisk" class="filter-select" @change="handleSearch">
            <option value="all">{{ marketplaceText('filters.allBadges', 'All badges') }}</option>
            <option v-for="option in riskOptions" :key="option.value" :value="option.value">
              {{ badgeLabelByValue(option.value) }} ({{ option.count }})
            </option>
          </select>
        </label>
      </div>

      <div v-if="activeFilterLabels.length" class="active-filters">
        <span class="section-label">{{
          marketplaceText('results.activeFilters', 'Active filters')
        }}</span>
        <div class="chip-row">
          <span
            v-for="label in activeFilterLabels"
            :key="label"
            class="summary-pill summary-pill-active"
          >
            {{ label }}
          </span>
        </div>
      </div>

      <div v-if="showSkillAdvice" class="advisor-panel dashboard-card-subsurface">
        <div class="advisor-panel__header">
          <div class="advisor-panel__copy">
            <span class="section-label">{{
              marketplaceText('advisor.kicker', 'Skill guidance')
            }}</span>
            <strong>{{ advisorTitle }}</strong>
            <p>{{ advisorDescription }}</p>
          </div>
          <span v-if="adviceLoading" class="summary-pill">
            {{ commonText('loading', 'Loading') }}
          </span>
          <span
            v-else-if="
              advisorInstalledSkill && activeSkillAdvice && !activeSkillAdvice.need_store_search
            "
            class="summary-pill summary-pill-active"
          >
            {{ advisorInstalledSkill }}
          </span>
        </div>

        <div v-if="advisorSuggestedQueries.length" class="advisor-section">
          <span class="section-label">{{
            marketplaceText('advisor.searchQueries', 'Suggested search phrases')
          }}</span>
          <div class="chip-row">
            <button
              v-for="query in advisorSuggestedQueries"
              :key="query"
              type="button"
              class="advisor-chip-button"
              @click="applyAdvisorQuery(query)"
            >
              {{ query }}
            </button>
          </div>
        </div>

        <div v-if="advisorCapabilityTags.length" class="advisor-section">
          <span class="section-label">{{
            marketplaceText('advisor.capabilityTags', 'Capability tags')
          }}</span>
          <div class="chip-row">
            <span v-for="tag in advisorCapabilityTags" :key="tag" class="meta-chip meta-chip-soft">
              {{ tag }}
            </span>
          </div>
        </div>

        <div v-if="advisorRecommendedSkills.length" class="advisor-section">
          <span class="section-label">{{
            marketplaceText('advisor.recommendedSkills', 'Recommended skills')
          }}</span>
          <div class="advisor-skill-grid">
            <article
              v-for="skill in advisorRecommendedSkills"
              :key="`advisor-${skill.id}`"
              class="advisor-skill-card dashboard-card-subsurface"
              :style="skillAccentStyle(skill)"
              tabindex="0"
              role="button"
              @click="selectSkill(skill)"
              @keydown.enter.prevent="selectSkill(skill)"
              @keydown.space.prevent="selectSkill(skill)"
            >
              <div class="advisor-skill-card__top">
                <div class="advisor-skill-card__copy">
                  <strong>{{ skill.name }}</strong>
                  <p>{{ cardDescription(skill) }}</p>
                </div>
                <span :class="['shield-chip', securityBadgeClass(skill)]">
                  {{ badgeLabel(skill) }}
                </span>
              </div>

              <div class="advisor-skill-card__meta">
                <span class="meta-chip meta-chip-soft">{{ sourceLabel(skill) }}</span>
                <span class="meta-chip meta-chip-soft">{{ categoryLabel(skill.category) }}</span>
              </div>

              <div class="advisor-skill-card__actions">
                <button
                  v-if="skill.installable"
                  :class="[
                    'install-button',
                    `install-${skill.security_badge || 'yellow'}`,
                    { busy: installingSkillId === skill.id },
                  ]"
                  :disabled="installingSkillId === skill.id || skill.security_badge === 'red'"
                  @click.stop="installSkill(skill)"
                >
                  <span v-if="installingSkillId === skill.id">{{
                    marketplaceText('actions.installing', 'Installing...')
                  }}</span>
                  <span v-else-if="skill.security_badge === 'red'">{{
                    marketplaceText('actions.blocked', 'Blocked')
                  }}</span>
                  <span v-else-if="skill.installed">{{
                    skillStoreText('installed', 'Installed')
                  }}</span>
                  <span v-else>{{ skillStoreText('install', 'Install') }}</span>
                </button>
                <button v-else class="source-button" @click.stop="openSkillSource(skill)">
                  {{ marketplaceText('actions.viewSource', 'View source') }}
                </button>
                <button type="button" class="btn-text" @click.stop="applyAdvisorQuery(skill.name)">
                  {{ marketplaceText('advisor.searchByName', 'Search by name') }}
                </button>
              </div>
            </article>
          </div>
        </div>

        <p v-if="advisorFeedbackNote" class="advisor-note">
          {{ advisorFeedbackNote }}
        </p>
      </div>

      <div
        v-if="showDiscoverProgress || showEmbeddingProgress"
        :class="[
          'discover-progress',
          'dashboard-card-subsurface',
          {
            'discover-progress--embedding': showEmbeddingOnlyProgress,
          },
        ]"
      >
        <section v-if="showDiscoverProgress" class="discover-progress__segment">
          <div class="discover-progress__header">
            <div class="discover-progress__copy">
              <span class="section-label">{{ discoverProgressLabel }}</span>
              <strong>{{ discoverCurrentSourceLabel }}</strong>
              <p>{{ discoverProgressMeta }}</p>
            </div>
            <div class="discover-progress__percent">
              <strong>{{ discoverProgressPercent }}%</strong>
              <span>{{ discoverPhaseLabel }}</span>
            </div>
          </div>

          <div class="discover-progress__meta">
            <span
              :class="[
                'discover-progress__status-pill',
                `discover-progress__status-pill--${discoverPhaseTone}`,
              ]"
            >
              {{ discoverPhaseLabel }}
            </span>
            <span v-if="discoverSummaryInline" class="discover-progress__summary">
              {{ discoverSummaryInline }}
            </span>
            <span v-if="showResultsRefreshing" class="discover-progress__summary">
              {{ marketplaceText('progress.refreshingVisible', 'Refreshing visible results') }}
            </span>
          </div>

          <div class="discover-progress__track-wrap">
            <div
              class="discover-progress__track"
              role="progressbar"
              :aria-label="discoverProgressLabel"
              :aria-valuenow="discoverProgressPercent"
              aria-valuemin="0"
              aria-valuemax="100"
            >
              <div
                class="discover-progress__fill"
                :style="{ width: `${discoverProgressPercent}%` }"
              ></div>
            </div>
            <p class="discover-progress__caption">{{ discoverProgressCaption }}</p>
          </div>

          <div class="discover-activity">
            <div class="discover-activity__header">
              <span class="section-label">{{ discoverActivityHeading }}</span>
            </div>
            <article
              v-if="latestDiscoverEntry"
              :class="[
                'discover-activity__item',
                'discover-activity__item--latest',
                `discover-activity__item--${latestDiscoverEntry.phase}`,
              ]"
            >
              <span class="discover-activity__dot" aria-hidden="true"></span>
              <div class="discover-activity__body">
                <strong>{{ latestDiscoverEntry.title }}</strong>
                <p>{{ latestDiscoverEntry.detail }}</p>
              </div>
              <time>{{ formatClockTime(latestDiscoverEntry.timestamp) }}</time>
            </article>
            <details v-if="previousDiscoverEntries.length" class="discover-activity__details">
              <summary class="discover-activity__toggle">
                {{ discoverActivityToggleLabel }}
              </summary>
              <div ref="discoverActivityViewport" class="discover-activity__stream">
                <article
                  v-for="entry in previousDiscoverEntries"
                  :key="entry.id"
                  :class="['discover-activity__item', `discover-activity__item--${entry.phase}`]"
                >
                  <span class="discover-activity__dot" aria-hidden="true"></span>
                  <div class="discover-activity__body">
                    <strong>{{ entry.title }}</strong>
                    <p>{{ entry.detail }}</p>
                  </div>
                  <time>{{ formatClockTime(entry.timestamp) }}</time>
                </article>
              </div>
            </details>
            <div
              v-else-if="!latestDiscoverEntry && showDiscoverProgress"
              class="discover-activity__tail"
            >
              <span class="discover-activity__tail-dot" aria-hidden="true"></span>
              <span>{{ discoverProgressDescription }}</span>
            </div>
          </div>
        </section>

        <section
          v-if="showEmbeddingProgress"
          class="discover-progress__segment discover-progress__segment--embedding"
        >
          <div class="discover-progress__header">
            <div class="discover-progress__copy">
              <span class="section-label">{{ embeddingProgressLabel }}</span>
              <strong>{{ embeddingCurrentSkillLabel }}</strong>
              <p>{{ embeddingProgressMeta }}</p>
            </div>
            <div class="discover-progress__percent">
              <strong>{{ embeddingProgressPercent }}%</strong>
              <span>{{ embeddingPhaseLabel }}</span>
            </div>
          </div>

          <div class="discover-progress__meta">
            <span
              :class="[
                'discover-progress__status-pill',
                `discover-progress__status-pill--${embeddingPhaseTone}`,
              ]"
            >
              {{ embeddingPhaseLabel }}
            </span>
            <span v-if="embeddingSummaryInline" class="discover-progress__summary">
              {{ embeddingSummaryInline }}
            </span>
          </div>

          <div class="discover-progress__track-wrap">
            <div
              class="discover-progress__track"
              role="progressbar"
              :aria-label="embeddingProgressLabel"
              :aria-valuenow="embeddingProgressPercent"
              aria-valuemin="0"
              aria-valuemax="100"
            >
              <div
                class="discover-progress__fill"
                :style="{ width: `${embeddingProgressPercent}%` }"
              ></div>
            </div>
            <p class="discover-progress__caption">{{ embeddingProgressCaption }}</p>
          </div>
        </section>
      </div>
    </section>

    <div v-if="error" class="error-banner">
      <span>{{ error }}</span>
      <button type="button" @click="error = null">×</button>
    </div>

    <section class="store-shell">
      <div class="results-panel dashboard-card-surface">
        <header class="panel-header">
          <div>
            <h3>{{ marketplaceText('results.discover', 'Discover') }}</h3>
            <p v-if="resultSubtitle">{{ resultSubtitle }}</p>
          </div>
          <span v-if="page < totalPages" class="summary-pill">{{
            marketplaceText('results.pageState', 'Page {page}/{total}', {
              page: page,
              total: totalPages,
            })
          }}</span>
        </header>

        <div v-if="showResultsLoading" class="loading-state">
          <div class="spinner"></div>
          <span>{{
            showDiscoverProgress ? discoverProgressLabel : commonText('loading', 'Loading')
          }}</span>
        </div>

        <div v-else-if="!skills.length" class="empty-state">
          <h3>{{ translate('skillStore.noResults', 'No matching skills') }}</h3>
          <p>
            {{
              normalizeSearchQuery(searchQuery)
                ? translate(
                    'skillStore.marketplace.empty.broadenSearch',
                    'Try broadening the search or relaxing one of the security filters.'
                  )
                : translate(
                    'skillStore.noSkillsAvailable',
                    'No skills are available from the configured sources yet.'
                  )
            }}
          </p>
          <button type="button" class="btn-ghost" @click="clearSearch">
            {{ skillStoreText('clearSearch', 'Clear Search') }}
          </button>
        </div>

        <div v-else class="results-grid">
          <article
            v-for="skill in skills"
            :key="skill.id"
            :class="[
              'skill-card',
              'dashboard-card-surface',
              { active: selectedSkillId === skill.id },
            ]"
            :style="skillAccentStyle(skill)"
            tabindex="0"
            role="button"
            @click="selectSkill(skill)"
            @keydown.enter.prevent="selectSkill(skill)"
            @keydown.space.prevent="selectSkill(skill)"
          >
            <div class="card-topline">
              <div class="card-topline-left">
                <span class="source-chip">{{ sourceLabel(skill) }}</span>
                <span class="meta-chip meta-chip-soft">{{ categoryLabel(skill.category) }}</span>
                <span v-if="skill.curated_label" class="meta-chip meta-chip-hot">
                  {{ curatedLabelText(skill.curated_label) }}
                </span>
              </div>
              <span :class="['shield-chip', securityBadgeClass(skill)]">
                {{ badgeLabel(skill) }}
              </span>
            </div>

            <div class="card-hero dashboard-card-subsurface">
              <div class="card-hero-main">
                <div class="card-icon" aria-hidden="true">
                  <span>{{ skillMonogram(skill) }}</span>
                </div>
                <div class="card-title-copy">
                  <div class="card-title-row">
                    <h4>{{ skill.name }}</h4>
                    <span v-if="skill.installed" class="meta-chip meta-chip-installed">{{
                      skillStoreText('installed', 'Installed')
                    }}</span>
                  </div>
                  <p>{{ cardDescription(skill) }}</p>
                  <div v-if="visibleSkillTags(skill).length" class="card-tag-row">
                    <span
                      v-for="tag in visibleSkillTags(skill)"
                      :key="`${skill.id}-${tag}`"
                      class="meta-chip meta-chip-soft"
                    >
                      {{ tag }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <p class="card-note">{{ cardSignalSummary(skill) }}</p>

            <div class="card-footer">
              <div class="card-stats-inline">
                <span class="card-stat-inline">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                    <path d="M12 3v12" />
                    <path d="m7 10 5 5 5-5" />
                    <path d="M5 21h14" />
                  </svg>
                  {{ formatNumber(skill.downloads) }}
                </span>
                <span class="card-stat-inline">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                    <path
                      d="m12 3.6 2.6 5.3 5.9.9-4.3 4.2 1 5.9-5.2-2.8-5.2 2.8 1-5.9-4.3-4.2 5.9-.9Z"
                    />
                  </svg>
                  {{ formatNumber(skill.stars) }}
                </span>
                <span class="card-stat-inline">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                    <path
                      d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"
                    />
                    <path d="m3.3 7 8.7 5 8.7-5" />
                    <path d="M12 22V12" />
                  </svg>
                  {{ skillVersionLabel(skill) }}
                </span>
              </div>
              <button
                v-if="skill.installable"
                :class="[
                  'install-button',
                  `install-${skill.security_badge || 'yellow'}`,
                  { busy: installingSkillId === skill.id },
                ]"
                :disabled="installingSkillId === skill.id || skill.security_badge === 'red'"
                @click.stop="installSkill(skill)"
              >
                <span v-if="installingSkillId === skill.id">{{
                  marketplaceText('actions.installing', 'Installing...')
                }}</span>
                <span v-else-if="skill.security_badge === 'red'">{{
                  marketplaceText('actions.blocked', 'Blocked')
                }}</span>
                <span v-else-if="skill.installed">{{
                  skillStoreText('installed', 'Installed')
                }}</span>
                <span v-else>{{ skillStoreText('install', 'Install') }}</span>
              </button>
              <button v-else class="source-button" @click.stop="openSkillSource(skill)">
                {{ marketplaceText('actions.viewSource', 'View source') }}
              </button>
            </div>
          </article>
        </div>

        <div v-if="!loading && page < totalPages" class="load-more">
          <button
            type="button"
            class="btn-ghost"
            :disabled="loadingMore"
            @click="fetchSkills(false)"
          >
            {{
              loadingMore
                ? commonText('loading', 'Loading...')
                : marketplaceText('actions.loadMore', 'Load more')
            }}
          </button>
        </div>
      </div>
    </section>

    <Teleport to="body">
      <div
        v-if="showDetailModal && detailSkill"
        class="skill-tab skill-store-aggregator store-detail-modal-backdrop"
        @click.self="closeSkillDetail"
      >
        <div
          class="store-detail-modal-shell"
          role="dialog"
          aria-modal="true"
          :aria-label="detailSkill.name"
        >
          <div class="store-detail-modal-handle" aria-hidden="true"></div>
          <div
            class="detail-card dashboard-card-surface store-detail-modal-card"
            :style="skillAccentStyle(detailSkill)"
          >
            <header class="detail-header">
              <div class="detail-hero-layout">
                <div class="detail-hero-main">
                  <div class="detail-icon" aria-hidden="true">
                    <span>{{ skillMonogram(detailSkill) }}</span>
                  </div>
                  <div class="detail-main">
                    <h3>{{ detailSkill.name }}</h3>
                    <code class="detail-slug">{{ detailSkill.id }}</code>
                    <div class="detail-pill-row">
                      <span class="detail-version-pill">{{ skillVersionLabel(detailSkill) }}</span>
                      <span v-if="detailInstalled" class="meta-chip meta-chip-installed">{{
                        skillStoreText('installed', 'Installed')
                      }}</span>
                    </div>
                    <p v-if="detailSkill.curated_reason" class="detail-callout">
                      {{ detailSkill.curated_reason }}
                    </p>
                  </div>
                </div>

                <aside class="detail-header-side">
                  <div class="detail-utility-actions">
                    <button
                      type="button"
                      class="detail-utility-button"
                      :aria-label="marketplaceText('actions.viewSource', 'View source')"
                      @click="openSkillSource(detailSkill)"
                    >
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                        <path d="M14 5h5v5" />
                        <path d="M10 14 19 5" />
                        <path d="M19 14v3a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h3" />
                      </svg>
                    </button>
                    <button
                      type="button"
                      class="detail-utility-button"
                      :aria-label="closeDetailLabel"
                      @click="closeSkillDetail"
                    >
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                        <path d="M18 6 6 18" />
                        <path d="m6 6 12 12" />
                      </svg>
                    </button>
                  </div>

                  <div class="detail-hero-stats detail-hero-stats--compact">
                    <article class="detail-hero-stat">
                      <span
                        class="detail-hero-stat__icon detail-hero-stat__icon--downloads"
                        aria-hidden="true"
                      >
                        <svg
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="1.8"
                        >
                          <path d="M12 3v12" />
                          <path d="m7 10 5 5 5-5" />
                          <path d="M5 21h14" />
                        </svg>
                      </span>
                      <strong>{{ formatNumber(detailSkill.downloads) }}</strong>
                      <small>{{ skillStoreText('detail.meta.downloads', 'Downloads') }}</small>
                    </article>
                    <article class="detail-hero-stat">
                      <span
                        class="detail-hero-stat__icon detail-hero-stat__icon--stars"
                        aria-hidden="true"
                      >
                        <svg
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="1.8"
                        >
                          <path
                            d="m12 3.6 2.6 5.3 5.9.9-4.3 4.2 1 5.9-5.2-2.8-5.2 2.8 1-5.9-4.3-4.2 5.9-.9Z"
                          />
                        </svg>
                      </span>
                      <strong>{{ formatNumber(detailSkill.stars) }}</strong>
                      <small>{{ skillStoreText('detail.meta.stars', 'Stars') }}</small>
                    </article>
                  </div>

                  <section class="detail-install-panel">
                    <div class="detail-install-copy">
                      <span class="section-label">{{
                        marketplaceText('detail.installTitle', 'Install')
                      }}</span>
                      <p class="detail-install-headline">
                        {{
                          marketplaceText(
                            'detail.installHeading',
                            'Add this skill to your workspace'
                          )
                        }}
                      </p>
                      <p>{{ installHint(detailSkill) }}</p>
                    </div>
                    <div class="detail-actions detail-actions--inline">
                      <button
                        v-if="detailSkill.installable"
                        :class="[
                          'install-button',
                          `install-${detailSkill.security_badge || 'yellow'}`,
                        ]"
                        :disabled="
                          installingSkillId === detailSkill.id ||
                          detailSkill.security_badge === 'red'
                        "
                        @click="installSkill(detailSkill)"
                      >
                        <span v-if="detailSkill.security_badge === 'red'">{{
                          marketplaceText('actions.blocked', 'Blocked')
                        }}</span>
                        <span v-else-if="detailInstalled">{{
                          skillStoreText('installed', 'Installed')
                        }}</span>
                        <span v-else>{{ skillStoreText('install', 'Install') }}</span>
                      </button>
                      <button v-else class="source-button" @click="openSkillSource(detailSkill)">
                        {{ marketplaceText('actions.viewSource', 'View source') }}
                      </button>
                      <button class="btn-ghost" @click="openSkillSource(detailSkill)">
                        {{ skillStoreText('detail.openLink', 'Open Link') }}
                      </button>
                    </div>
                  </section>
                </aside>
              </div>
            </header>

            <div class="detail-meta">
              <div class="meta-item">
                <span>{{ skillStoreText('detail.meta.updated', 'Last Updated') }}</span>
                <strong>{{ formatDate(detailUpdatedAt) }}</strong>
              </div>
              <div class="meta-item">
                <span>{{ marketplaceText('detail.meta.category', 'Category') }}</span>
                <strong>{{ categoryLabel(detailSkill.category) }}</strong>
              </div>
              <div v-if="detailSkill.author" class="meta-item">
                <span>{{ marketplaceText('detail.meta.author', 'Author') }}</span>
                <strong>{{ detailSkill.author }}</strong>
              </div>
              <div class="meta-item">
                <span>{{ skillStoreText('detail.openLink', 'Open Link') }}</span>
                <strong>{{ sourceLabel(detailSkill) }}</strong>
              </div>
            </div>

            <SkillContractNotice
              v-if="detailContract"
              class="detail-contract-panel"
              :contract="detailContract"
              compact
            />

            <section class="detail-section">
              <div class="section-heading">
                <div>
                  <h4>{{ marketplaceText('security.title', 'Security') }}</h4>
                  <p>{{ installHint(detailSkill) }}</p>
                </div>
                <span :class="['shield-chip', detailBadgeClass(detailSkill.security_badge)]">
                  {{ detailRiskLabel }}
                </span>
              </div>
              <div v-if="detailLoading" class="detail-loading">
                <div class="spinner"></div>
                <span>{{ marketplaceText('security.loading', 'Loading security report...') }}</span>
              </div>
              <template v-else-if="selectedSecurity">
                <div class="security-overview">
                  <div class="score-card score-card-emphasis">
                    <span>{{ marketplaceText('security.score', 'Score') }}</span>
                    <strong>{{ selectedSecurity.score }}</strong>
                  </div>
                  <div class="security-summary-grid">
                    <div class="score-card">
                      <span>{{ marketplaceText('security.badge', 'Badge') }}</span>
                      <strong>{{ badgeLabelByValue(selectedSecurity.security_badge) }}</strong>
                    </div>
                    <div class="score-card">
                      <span>{{
                        marketplaceText('filters.vulnerabilities', 'Vulnerabilities')
                      }}</span>
                      <strong>{{
                        vulnerabilityLabel(selectedSecurity.vulnerability_status)
                      }}</strong>
                    </div>
                    <div class="score-card">
                      <span>{{ marketplaceText('security.installable', 'Installable') }}</span>
                      <strong>{{
                        detailStat(selectedSecurity.install_surface?.installable)
                      }}</strong>
                    </div>
                  </div>
                </div>

                <div class="list-block">
                  <span class="section-label">{{
                    marketplaceText('security.signals', 'Risk signals')
                  }}</span>
                  <div v-if="selectedSecuritySignals.length" class="chip-row">
                    <span
                      v-for="signal in selectedSecuritySignals"
                      :key="signal.label"
                      :class="['signal-chip', signal.className]"
                    >
                      {{ signal.label }}
                    </span>
                  </div>
                  <p v-else class="detail-placeholder detail-placeholder-inline">
                    {{ marketplaceText('security.noMajorWarnings', 'No major warnings detected.') }}
                  </p>
                </div>

                <div v-if="selectedSecurity.permissions?.length" class="list-block">
                  <span class="section-label">{{
                    marketplaceText('security.permissions', 'Permissions')
                  }}</span>
                  <div class="chip-row">
                    <span
                      v-for="permission in selectedSecurity.permissions"
                      :key="permission"
                      class="meta-chip meta-chip-soft"
                    >
                      {{ permission }}
                    </span>
                  </div>
                </div>

                <div
                  v-if="selectedSecurity.install_surface?.dependency_manifests?.length"
                  class="list-block"
                >
                  <span class="section-label">{{
                    marketplaceText('security.dependencyManifests', 'Dependency manifests')
                  }}</span>
                  <div class="chip-row">
                    <span
                      v-for="manifest in selectedSecurity.install_surface.dependency_manifests"
                      :key="manifest"
                      class="meta-chip meta-chip-soft"
                    >
                      {{ manifest }}
                    </span>
                  </div>
                </div>

                <div v-if="visibleSecurityEvidence.length" class="list-block">
                  <span class="section-label">{{
                    marketplaceText('security.evidence', 'Evidence')
                  }}</span>
                  <div class="evidence-list">
                    <article
                      v-for="item in visibleSecurityEvidence as SkillSecurityEvidence[]"
                      :key="`${item.type}-${item.title}-${item.value}`"
                      class="evidence-item"
                    >
                      <header>
                        <strong>{{ evidenceGroupLabel(item.type) }}</strong>
                        <span>{{ item.severity }}</span>
                      </header>
                      <p>{{ item.title }}</p>
                      <small v-if="item.description">{{ item.description }}</small>
                      <code v-if="item.value">{{ item.value }}</code>
                    </article>
                  </div>
                  <p
                    v-if="hiddenSecurityEvidenceCount"
                    class="detail-placeholder detail-placeholder-inline"
                  >
                    {{
                      marketplaceText('security.moreEvidence', '+{count} more evidence items', {
                        count: hiddenSecurityEvidenceCount,
                      })
                    }}
                  </p>
                </div>
              </template>
              <p v-else class="detail-placeholder">
                {{ marketplaceText('security.noReport', 'No security report available yet.') }}
              </p>
            </section>

            <section class="detail-section">
              <h4>{{ marketplaceText('detail.title', 'Details') }}</h4>
              <p>{{ cardDescription(detailSkill) }}</p>
            </section>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="pendingRiskSkill"
        class="skill-store-aggregator risk-modal-backdrop"
        @click.self="closeRiskModal"
      >
        <div class="risk-modal" :style="skillAccentStyle(pendingRiskSkill)">
          <button
            class="risk-modal__close"
            type="button"
            :aria-label="commonText('close', 'Close')"
            @click="closeRiskModal"
          >
            <span aria-hidden="true">&times;</span>
          </button>

          <header class="risk-modal__header">
            <div class="risk-modal__hero">
              <div class="risk-modal__icon" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                  <path
                    d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"
                  />
                  <path d="M12 8v5" />
                  <path d="M12 17h.01" />
                </svg>
              </div>
              <div class="risk-modal__copy">
                <div class="risk-modal__eyebrow">
                  <span class="shield-chip badge-yellow">{{
                    commonText('warning', 'Warning')
                  }}</span>
                  <span :class="['shield-chip', detailBadgeClass(pendingRiskSkill.security_badge)]">
                    {{ badgeLabel(pendingRiskSkill) }}
                  </span>
                </div>
                <h3>
                  {{
                    marketplaceText(
                      'modal.riskAcknowledgementTitle',
                      'Risk acknowledgement required'
                    )
                  }}
                </h3>
                <p>
                  {{
                    marketplaceText(
                      'modal.riskAcknowledgementBody',
                      '{name} requires acknowledgement before installing. Review the security summary first.',
                      { name: pendingRiskSkill.name }
                    )
                  }}
                </p>
              </div>
            </div>
          </header>

          <section class="risk-modal__subject">
            <div class="risk-modal__subject-icon" aria-hidden="true">
              <span>{{ skillMonogram(pendingRiskSkill) }}</span>
            </div>
            <div class="risk-modal__subject-copy">
              <strong>{{ pendingRiskSkill.name }}</strong>
              <div class="chip-row">
                <span class="meta-chip meta-chip-soft">{{ sourceLabel(pendingRiskSkill) }}</span>
                <span class="meta-chip meta-chip-soft">{{
                  skillVersionLabel(pendingRiskSkill)
                }}</span>
              </div>
            </div>
          </section>

          <section class="risk-modal__summary">
            <article class="risk-modal__stat">
              <span class="section-label">{{
                marketplaceText('detail.riskLevel', 'Risk level')
              }}</span>
              <strong>{{ riskLabel(pendingRiskSkill) }}</strong>
            </article>
            <article class="risk-modal__stat">
              <span class="section-label">{{ marketplaceText('security.badge', 'Badge') }}</span>
              <strong>{{ badgeLabel(pendingRiskSkill) }}</strong>
            </article>
            <article v-if="pendingRiskSkill.install_type" class="risk-modal__stat">
              <span class="section-label">{{
                marketplaceText('security.installSurface', 'Install surface')
              }}</span>
              <strong>{{ installTypeLabel(pendingRiskSkill.install_type) }}</strong>
            </article>
            <article
              v-if="typeof pendingRiskSkill.security_score === 'number'"
              class="risk-modal__stat"
            >
              <span class="section-label">{{ marketplaceText('security.score', 'Score') }}</span>
              <strong>{{ pendingRiskSkill.security_score }}</strong>
            </article>
          </section>

          <div v-if="pendingRiskSignals.length" class="modal-signal-block">
            <span class="section-label">{{
              marketplaceText('modal.reviewSignals', 'Review these signals')
            }}</span>
            <div class="risk-modal__signal-list">
              <article
                v-for="signal in pendingRiskSignals"
                :key="signal"
                class="risk-modal__signal"
              >
                <span class="risk-modal__signal-dot" aria-hidden="true"></span>
                <span>{{ signal }}</span>
              </article>
            </div>
          </div>

          <div class="modal-actions risk-modal__actions">
            <button
              class="btn-ghost risk-modal__cancel-button"
              type="button"
              @click="closeRiskModal"
            >
              {{ commonText('cancel', 'Cancel') }}
            </button>
            <button
              class="btn-ghost risk-modal__review-button"
              type="button"
              @click="reviewRiskSkill"
            >
              {{ marketplaceText('modal.reviewSummary', 'Review security summary') }}
            </button>
            <button
              class="btn-primary risk-modal__confirm-button"
              type="button"
              @click="confirmRiskInstall"
            >
              {{ marketplaceText('modal.confirmInstall', 'Confirm install') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
@import './extension-tab.css';

.skill-store-aggregator {
  --panel-bg: var(--glass-bg, rgba(255, 255, 255, 0.05));
  --panel-bg-strong: var(--glass-bg, rgba(255, 255, 255, 0.08));
  --panel-border: var(--border);
  --security-green-bg: rgba(34, 197, 94, 0.14);
  --security-green-border: rgba(34, 197, 94, 0.24);
  --security-green-text: #dcfce7;
  --security-yellow-bg: rgba(245, 158, 11, 0.15);
  --security-yellow-border: rgba(245, 158, 11, 0.28);
  --security-yellow-text: #fef3c7;
  --security-red-bg: rgba(239, 68, 68, 0.14);
  --security-red-border: rgba(239, 68, 68, 0.28);
  --security-red-text: #fee2e2;
  --security-neutral-bg: rgba(148, 163, 184, 0.14);
  --security-neutral-border: rgba(148, 163, 184, 0.22);
  --security-neutral-text: #e2e8f0;
  --card-shadow: 0 18px 36px rgba(15, 23, 42, 0.14);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

:root.light .skill-store-aggregator,
[data-theme='light'] .skill-store-aggregator {
  --security-green-text: #166534;
  --security-yellow-text: #92400e;
  --security-red-text: #b91c1c;
  --security-neutral-text: #475569;
  --card-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
}

.toolbar-panel,
.results-panel,
.detail-card,
.detail-empty {
  border: 1px solid var(--panel-border);
  border-radius: 18px;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--panel-bg-strong) 88%, white 3%) 0%,
    var(--panel-bg) 100%
  );
  box-shadow: var(--card-shadow);
}

.hero-actions,
.modal-actions,
.detail-actions,
.card-badges,
.chip-row,
.card-topline-left {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.btn-primary,
.btn-ghost,
.install-button,
.source-button {
  appearance: none;
  min-height: 26px;
  border-radius: 7px;
  padding: 4px 8px;
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease,
    opacity 0.2s ease;
}

.btn-primary {
  border: 1px solid var(--primary);
  background: var(--primary);
  color: white;
}

.btn-ghost,
.source-button {
  border: 1px solid var(--border);
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  color: var(--text-primary);
}

.btn-ghost:hover:not(:disabled),
.install-button:hover:not(:disabled),
.source-button:hover:not(:disabled) {
  background: var(--bg-hover);
  box-shadow: var(--card-shadow);
}

.btn-primary:hover:not(:disabled) {
  opacity: 0.92;
  box-shadow: var(--card-shadow);
}

.btn-primary:disabled,
.btn-ghost:disabled,
.install-button:disabled,
.source-button:disabled {
  opacity: 0.58;
  cursor: not-allowed;
}

.btn-text {
  border: 0;
  background: transparent;
  color: var(--primary);
  font-size: 10.5px;
  font-weight: 600;
  cursor: pointer;
  padding: 0;
}

.btn-text:hover {
  opacity: 0.9;
}

.hero-panel {
  padding: 12px 14px;
  background: var(--panel-bg-strong);
}

.hero-headline {
  display: block;
}

.hero-copy {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.hero-kicker {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  border-radius: 999px;
  padding: 3px 7px;
  border: 1px solid rgba(59, 130, 246, 0.2);
  background: rgba(59, 130, 246, 0.12);
  color: var(--primary);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.hero-copy h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: 16px;
  line-height: 1.12;
  letter-spacing: -0.02em;
}

.hero-copy p {
  margin: 0;
  max-width: 62ch;
  color: var(--text-secondary);
  font-size: 10.5px;
  line-height: 1.4;
}

.hero-summary-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 8px;
  margin-top: 8px;
}

.toolbar-note,
.section-label {
  display: block;
  color: var(--text-secondary);
  font-size: 9px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.toolbar-note {
  text-transform: none;
  letter-spacing: 0;
  font-size: 10px;
}

.toolbar-search-row,
.toolbar-controls {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
}

.advisor-panel {
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--primary) 12%, var(--panel-border));
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.08), transparent 42%),
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--panel-bg-strong) 92%, white 2%) 0%,
      var(--panel-bg) 100%
    );
}

.advisor-panel__header,
.advisor-skill-card__top,
.advisor-skill-card__actions {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  justify-content: space-between;
}

.advisor-panel__copy,
.advisor-section,
.advisor-skill-card {
  display: flex;
  flex-direction: column;
}

.advisor-panel__copy {
  gap: 4px;
}

.advisor-panel__copy strong {
  color: var(--text-primary);
  font-size: 11.5px;
  line-height: 1.2;
}

.advisor-panel__copy p,
.advisor-note,
.advisor-skill-card__copy p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 10px;
  line-height: 1.45;
}

.advisor-section {
  gap: 6px;
  margin-top: 10px;
}

.advisor-chip-button {
  appearance: none;
  border: 1px solid rgba(59, 130, 246, 0.24);
  background: rgba(59, 130, 246, 0.1);
  color: var(--text-primary);
  border-radius: 999px;
  padding: 4px 8px;
  font-size: 9.5px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

.advisor-chip-button:hover {
  background: rgba(59, 130, 246, 0.16);
  border-color: rgba(59, 130, 246, 0.34);
  transform: translateY(-1px);
}

.advisor-skill-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.advisor-skill-card {
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border: 1px solid color-mix(in srgb, var(--market-accent, var(--primary)) 16%, var(--border));
  cursor: pointer;
  transition:
    transform 0.2s ease,
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}

.advisor-skill-card:hover {
  transform: translateY(-1px);
  box-shadow: var(--card-shadow);
}

.advisor-skill-card__copy {
  min-width: 0;
}

.advisor-skill-card__copy strong {
  display: block;
  color: var(--text-primary);
  font-size: 10.5px;
  line-height: 1.25;
}

.advisor-skill-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.advisor-skill-card__actions {
  align-items: center;
}

.advisor-note {
  margin-top: 10px;
}

.hero-search-field {
  min-width: 0;
  flex: 1;
}

.hero-search-field :deep(.semantic-search-field) {
  border-radius: 14px;
}

.hero-search-field :deep(.semantic-search-shell) {
  min-height: 30px;
  gap: 6px;
  padding: 5px 8px 5px 9px;
  border-radius: 12px;
}

.hero-search-field :deep(.semantic-search-icon) {
  width: 14px;
  height: 14px;
}

.hero-search-field :deep(.semantic-search-input) {
  height: 18px;
  font-size: 0.7rem;
}

.hero-search-field :deep(.semantic-search-clear) {
  width: 22px;
  height: 22px;
}

.sort-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.sort-pill {
  appearance: none;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-secondary);
  border-radius: 999px;
  padding: 3px 7px;
  font-size: 9px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease;
}

.sort-pill:hover {
  background: var(--bg-hover);
}

.sort-pill.active {
  border-color: rgba(59, 130, 246, 0.32);
  background: rgba(59, 130, 246, 0.14);
  color: var(--text-primary);
}

.toolbar-focus-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.filter-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: 8px;
}

.filter-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: var(--text-secondary);
  font-size: 8px;
}

.filter-select {
  min-width: 0;
  height: 26px;
  padding: 0 6px;
  border-radius: 7px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  color: var(--text-primary);
  font-size: 10px;
}

.active-filters {
  display: grid;
  gap: 6px;
  margin-top: 8px;
}

.discover-progress {
  display: grid;
  gap: 0;
  margin-top: 8px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--panel-bg) 94%, rgba(59, 130, 246, 0.06));
}

.discover-progress__segment {
  display: grid;
  gap: 10px;
}

.discover-progress__segment + .discover-progress__segment {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid color-mix(in srgb, var(--border) 86%, rgba(14, 165, 233, 0.14));
}

.discover-progress--embedding {
  background: color-mix(in srgb, var(--panel-bg) 94%, rgba(14, 165, 233, 0.08));
}

.discover-progress__header {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: start;
}

.discover-progress__copy {
  display: grid;
  gap: 4px;
}

.discover-progress__copy strong {
  color: var(--text-primary);
  font-size: 11px;
  line-height: 1.35;
}

.discover-progress__copy p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 9.5px;
  line-height: 1.4;
}

.discover-progress__percent {
  display: grid;
  gap: 2px;
  justify-items: end;
  text-align: end;
}

.discover-progress__percent strong {
  color: var(--text-primary);
  font-size: 18px;
  line-height: 1;
}

.discover-progress__percent span {
  color: var(--text-secondary);
  font-size: 9px;
  line-height: 1.3;
}

.discover-progress__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px 8px;
}

.discover-progress__status-pill {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 7px;
  border-radius: 999px;
  border: 1px solid var(--border);
  font-size: 9px;
  font-weight: 700;
  color: var(--text-secondary);
  background: rgba(148, 163, 184, 0.1);
}

.discover-progress__status-pill--initializing,
.discover-progress__status-pill--running {
  border-color: rgba(59, 130, 246, 0.18);
  background: rgba(59, 130, 246, 0.08);
  color: var(--text-primary);
}

.discover-progress__status-pill--completed {
  border-color: rgba(34, 197, 94, 0.18);
  background: rgba(34, 197, 94, 0.08);
  color: var(--text-primary);
}

.discover-progress__status-pill--error {
  border-color: rgba(239, 68, 68, 0.18);
  background: rgba(239, 68, 68, 0.08);
  color: var(--text-primary);
}

.discover-progress__status-pill--idle {
  color: var(--text-secondary);
}

.discover-progress__summary {
  color: var(--text-secondary);
  font-size: 9px;
  line-height: 1.4;
}

.discover-progress__track-wrap {
  display: grid;
  gap: 6px;
}

.discover-progress__track {
  position: relative;
  overflow: hidden;
  width: 100%;
  height: 8px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
}

.discover-progress__fill {
  height: 100%;
  border-radius: inherit;
  background: #3b82f6;
  transition: width 0.25s ease;
}

.discover-progress__segment--embedding .discover-progress__fill {
  background: #0ea5e9;
}

.discover-progress__caption {
  margin: 0;
  color: var(--text-secondary);
  font-size: 8.5px;
  line-height: 1.4;
}

.discover-activity {
  display: grid;
  gap: 4px;
}

.discover-activity__header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.discover-activity__details {
  display: grid;
  gap: 4px;
}

.discover-activity__toggle {
  cursor: pointer;
  color: var(--text-secondary);
  font-size: 9px;
  line-height: 1.4;
  list-style: none;
}

.discover-activity__toggle::-webkit-details-marker {
  display: none;
}

.discover-activity__stream {
  display: grid;
  gap: 4px;
  max-height: 132px;
  overflow-y: auto;
  padding-inline-end: 2px;
}

.discover-activity__item,
.discover-activity__tail {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: flex-start;
  gap: 7px;
  padding: 6px 2px;
  border-radius: 0;
  border: 0;
  background: transparent;
}

.discover-activity__item--latest {
  padding-top: 2px;
}

.discover-activity__body {
  display: grid;
  gap: 2px;
}

.discover-activity__item strong,
.discover-activity__body strong {
  color: var(--text-primary);
  font-size: 10.5px;
  line-height: 1.35;
}

.discover-activity__item p,
.discover-activity__body p {
  margin: 2px 0 0;
  color: var(--text-secondary);
  font-size: 9.5px;
  line-height: 1.45;
}

.discover-activity__item time {
  color: var(--text-tertiary);
  font-size: 8.5px;
  white-space: nowrap;
}

.discover-activity__dot,
.discover-activity__tail-dot {
  width: 5px;
  height: 5px;
  margin-top: 6px;
  border-radius: 999px;
  background: #38bdf8;
}

.discover-activity__tail {
  color: var(--text-secondary);
  font-size: 9.5px;
}

.discover-activity__item--batch {
  color: inherit;
}

.discover-activity__item--source_complete {
  color: inherit;
}

.discover-activity__item--completed .discover-activity__dot {
  background: #22c55e;
}

.discover-activity__item--completed {
  color: inherit;
}

.discover-activity__item--error .discover-activity__dot {
  background: #ef4444;
}

.discover-activity__item--error {
  color: inherit;
}

.chip-button {
  appearance: none;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  color: var(--text-secondary);
  border-radius: 999px;
  padding: 2px 6px;
  font-size: 9px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

.chip-button small {
  color: inherit;
  opacity: 0.8;
  font-size: 8px;
}

.chip-button:hover {
  transform: translateY(-1px);
  background: var(--bg-hover);
}

.chip-button.active {
  border-color: rgba(59, 130, 246, 0.28);
  background: rgba(59, 130, 246, 0.14);
  color: var(--text-primary);
}

.chip-button-quiet {
  padding: 5px 9px;
}

.store-shell {
  display: block;
}

.results-panel,
.detail-card,
.detail-empty {
  padding: 10px;
  box-shadow: var(--card-shadow);
}

.panel-header,
.detail-header {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: flex-start;
}

.panel-header h3,
.detail-header h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: 14px;
}

.panel-header p,
.detail-empty p,
.detail-placeholder {
  margin: 4px 0 0;
  color: var(--text-secondary);
  line-height: 1.4;
  font-size: 10.5px;
}

.detail-placeholder-inline {
  margin-top: 8px;
}

.summary-pill,
.source-chip,
.shield-chip,
.meta-chip,
.tag-chip,
.signal-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 999px;
  padding: 2px 5px;
  font-size: 8px;
  font-weight: 600;
}

.summary-pill,
.source-chip {
  background: var(--panel-bg);
  color: var(--text-secondary);
  border: 1px solid var(--border);
}

.shield-chip {
  border: 1px solid transparent;
}

.badge-green {
  background: var(--security-green-bg);
  border-color: var(--security-green-border);
  color: var(--security-green-text);
}

.badge-yellow {
  background: var(--security-yellow-bg);
  border-color: var(--security-yellow-border);
  color: var(--security-yellow-text);
}

.badge-red {
  background: var(--security-red-bg);
  border-color: var(--security-red-border);
  color: var(--security-red-text);
}

.badge-neutral {
  background: var(--security-neutral-bg);
  border-color: var(--security-neutral-border);
  color: var(--security-neutral-text);
}

.summary-pill-active {
  background: rgba(59, 130, 246, 0.12);
  border-color: rgba(59, 130, 246, 0.2);
  color: var(--text-primary);
}

.meta-chip-soft {
  background: var(--panel-bg);
  color: var(--text-primary);
  border: 1px solid var(--border);
}

.meta-chip-hot {
  background: rgba(59, 130, 246, 0.12);
  color: var(--primary);
  border: 1px solid rgba(59, 130, 246, 0.18);
}

.meta-chip-installed {
  background: rgba(34, 197, 94, 0.12);
  color: var(--success);
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.tag-chip {
  background: var(--panel-bg);
  color: var(--text-secondary);
  border: 1px solid var(--border);
}

.signal-chip {
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.04);
}

.signal-warn {
  color: var(--security-yellow-text);
  border-color: var(--security-yellow-border);
  background: var(--security-yellow-bg);
}

.results-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 12px;
}

.skill-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px;
  border-radius: 20px;
  border: 1px solid var(--border);
  background:
    radial-gradient(circle at top right, var(--market-accent-soft) 0%, transparent 42%),
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--panel-bg-strong) 92%, white 2%) 0%,
      var(--panel-bg) 100%
    );
  cursor: pointer;
  min-height: 0;
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease,
    transform 0.2s ease,
    box-shadow 0.2s ease;
}

.skill-card:hover,
.skill-card.active {
  border-color: var(--market-accent-ring);
  transform: translateY(-2px);
  box-shadow:
    var(--card-shadow),
    0 0 0 1px color-mix(in srgb, var(--market-accent-ring) 44%, transparent);
}

.card-topline,
.card-footer,
.evidence-item header {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
}

.card-title-row {
  display: flex;
  justify-content: space-between;
  gap: 6px;
  align-items: flex-start;
}

.card-title-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.card-hero {
  padding: 0;
  border-radius: 0;
  border: 0;
  background: transparent;
}

.card-hero-main {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.card-icon,
.detail-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 16px;
  background: var(--market-icon-bg);
  border: 1px solid var(--market-icon-border);
  color: var(--market-icon-fg);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 18px 28px -24px var(--market-accent-glow);
  font-size: 18px;
  font-weight: 700;
  line-height: 1;
  flex-shrink: 0;
}

.detail-icon {
  width: 74px;
  height: 74px;
  border-radius: 22px;
  font-size: 36px;
}

.card-hero h4 {
  margin: 0 0 4px;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.15;
  letter-spacing: -0.02em;
}

.card-hero p {
  margin: 0;
  min-height: 44px;
  color: var(--text-secondary);
  line-height: 1.4;
  font-size: 10.5px;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.skill-card:hover .card-hero p,
.skill-card.active .card-hero p {
  -webkit-line-clamp: 6;
}

.card-tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.card-note {
  margin: 0;
  min-height: 24px;
  color: var(--text-secondary);
  font-size: 9px;
  line-height: 1.35;
}

.card-footer {
  margin-top: auto;
  align-items: center;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}

.card-stats-inline {
  display: flex;
  flex-wrap: wrap;
  gap: 5px 6px;
}

.card-stat-inline {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 9px;
  font-weight: 600;
}

.card-stat-inline svg {
  width: 11px;
  height: 11px;
  color: var(--market-accent);
  flex-shrink: 0;
}

.install-button.install-green {
  border: 1px solid var(--security-green-border);
  background: var(--security-green-bg);
  color: var(--security-green-text);
}

.install-button.install-yellow {
  border: 1px solid var(--security-yellow-border);
  background: var(--security-yellow-bg);
  color: var(--security-yellow-text);
}

.install-button.install-red {
  border: 1px solid var(--security-red-border);
  background: var(--security-red-bg);
  color: var(--security-red-text);
}

.load-more,
.loading-state,
.detail-loading,
.empty-state,
.detail-empty {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
  justify-content: center;
  min-height: 100px;
  text-align: center;
}

.spinner {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 2px solid var(--border);
  border-top-color: var(--primary);
  animation: spin 0.9s linear infinite;
}

.detail-meta {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin-top: 8px;
}

.detail-contract-panel {
  margin-top: 8px;
}

.store-detail-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 30;
  display: flex;
  flex-direction: row;
  align-items: flex-start;
  justify-content: center;
  gap: 0;
  padding: 18px;
  overflow-y: auto;
  overscroll-behavior: contain;
  background: rgba(15, 23, 42, 0.54);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

.store-detail-modal-shell {
  width: min(920px, 100%);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.store-detail-modal-handle {
  width: 56px;
  height: 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.38);
  margin: 0 auto;
}

.store-detail-modal-card {
  position: relative;
}

.detail-card {
  background:
    radial-gradient(circle at top right, var(--market-accent-soft) 0%, transparent 42%),
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--panel-bg-strong) 88%, white 3%) 0%,
      var(--panel-bg) 100%
    );
}

.detail-hero-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(250px, 290px);
  gap: 12px;
  width: 100%;
}

.detail-hero-main {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}

.detail-main {
  display: flex;
  flex-direction: column;
  gap: 7px;
  min-width: 0;
}

.detail-header-side {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: stretch;
}

.detail-utility-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.detail-utility-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.92);
  color: var(--text-secondary);
  cursor: pointer;
  box-shadow: 0 14px 24px -22px rgba(15, 23, 42, 0.32);
  transition:
    transform 0.2s ease,
    color 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.detail-utility-button:hover {
  color: var(--market-accent);
  border-color: color-mix(in srgb, var(--market-accent) 24%, var(--border));
  background: white;
  transform: translateY(-1px);
}

.detail-utility-button svg {
  width: 17px;
  height: 17px;
}

.detail-main h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: 1.14rem;
  line-height: 1.12;
  letter-spacing: -0.03em;
}

.detail-slug {
  display: inline-flex;
  width: fit-content;
  padding: 3px 7px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  color: var(--text-secondary);
  font-size: 10px;
  line-height: 1;
}

.detail-pill-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.detail-version-pill {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 5px 10px;
  border: 1px solid rgba(34, 197, 94, 0.22);
  background: rgba(34, 197, 94, 0.12);
  color: #16a34a;
  font-size: 12px;
  font-weight: 700;
}

.detail-actions {
  flex-direction: column;
  align-items: stretch;
  min-width: 122px;
}

.detail-actions--inline {
  flex-direction: column;
  flex-wrap: wrap;
  align-items: stretch;
  justify-content: flex-start;
  min-width: 0;
}

.detail-hero-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 0;
}

.detail-hero-stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-height: 80px;
  justify-content: center;
  align-items: center;
  text-align: center;
  padding: 10px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background:
    radial-gradient(circle at top right, var(--market-accent-soft) 0%, transparent 56%),
    var(--panel-bg);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.detail-hero-stats--compact .detail-hero-stat {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 2px 8px;
  align-items: center;
  text-align: start;
}

.detail-hero-stat__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 999px;
  margin-bottom: 0;
  grid-row: span 2;
}

.detail-hero-stat__icon svg {
  width: 14px;
  height: 14px;
}

.detail-hero-stat__icon--downloads {
  color: #2563eb;
  background: rgba(59, 130, 246, 0.12);
}

.detail-hero-stat__icon--stars {
  color: #ea580c;
  background: rgba(249, 115, 22, 0.12);
}

.detail-hero-stat strong {
  color: var(--text-primary);
  font-size: clamp(0.94rem, 0.45vw + 0.88rem, 1.15rem);
  line-height: 1;
  letter-spacing: -0.02em;
}

.detail-hero-stat small {
  color: var(--text-secondary);
  font-size: 8px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.detail-install-panel {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  justify-content: flex-start;
  gap: 8px;
  margin-top: 0;
  padding: 10px;
  border-radius: 14px;
  border: 1px solid var(--border);
  background:
    radial-gradient(circle at top right, var(--market-accent-soft) 0%, transparent 56%),
    color-mix(in srgb, var(--panel-bg) 92%, white 5%);
}

.detail-install-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.detail-install-headline {
  margin: 0;
  color: var(--text-primary);
  font-size: 11px;
  line-height: 1.35;
  font-weight: 700;
}

.detail-install-copy p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 10px;
  line-height: 1.4;
}

.detail-callout,
.section-heading p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 10.5px;
  line-height: 1.55;
}

.detail-callout {
  padding: 8px 10px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--market-accent) 18%, var(--border));
  background: color-mix(in srgb, var(--market-accent-soft) 76%, var(--panel-bg));
}

.meta-item,
.score-card {
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--panel-bg);
  border: 1px solid var(--border);
}

.meta-item {
  min-height: 48px;
}

.meta-item span,
.score-card span,
.section-label {
  margin-bottom: 4px;
  color: var(--text-secondary);
  font-size: 9px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.meta-item strong,
.score-card strong {
  color: var(--text-primary);
}

.signal-detail-safe {
  border-color: rgba(34, 197, 94, 0.18);
  background: rgba(34, 197, 94, 0.08);
}

.signal-detail-alert {
  border-color: rgba(239, 68, 68, 0.24);
  background: rgba(239, 68, 68, 0.1);
}

.signal-detail-warn {
  border-color: rgba(245, 158, 11, 0.24);
  background: rgba(245, 158, 11, 0.1);
}

.section-heading {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: flex-start;
  margin-bottom: 8px;
}

.detail-section {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}

.detail-section h4 {
  margin: 0 0 8px;
  color: var(--text-primary);
  font-size: 12.5px;
}

.security-overview {
  display: grid;
  grid-template-columns: minmax(96px, 118px) minmax(0, 1fr);
  gap: 8px;
}

.score-card-emphasis {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  min-height: 92px;
  background: var(--panel-bg);
}

.score-card-emphasis strong {
  font-size: 24px;
  line-height: 1;
}

.security-summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.list-block {
  margin-top: 8px;
}

.evidence-list {
  display: grid;
  gap: 6px;
  margin-top: 6px;
}

.evidence-item {
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--panel-bg);
  border: 1px solid var(--border);
}

.evidence-item p,
.evidence-item small {
  display: block;
  margin-top: 4px;
  color: var(--text-secondary);
}

.evidence-item code {
  display: block;
  margin-top: 4px;
  padding: 4px 6px;
  border-radius: 8px;
  background: var(--bg-hover);
  color: var(--text-primary);
  overflow-x: auto;
  font-size: 10px;
}

.risk-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  background: rgba(15, 23, 42, 0.54);
  backdrop-filter: blur(8px);
}

.risk-modal {
  position: relative;
  overflow: hidden;
  width: min(560px, calc(100vw - 24px));
  display: grid;
  gap: 18px;
  padding: 22px;
  border-radius: 28px;
  border: 1px solid color-mix(in srgb, var(--security-yellow-border) 78%, var(--border));
  background:
    radial-gradient(circle at top right, rgba(245, 158, 11, 0.24) 0%, transparent 38%),
    radial-gradient(
      circle at top left,
      color-mix(in srgb, var(--market-accent-soft) 80%, transparent) 0%,
      transparent 28%
    ),
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--panel-bg-strong) 94%, white 4%) 0%,
      var(--panel-bg) 100%
    );
  box-shadow:
    0 32px 80px rgba(15, 23, 42, 0.34),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
  animation: risk-modal-rise 0.22s ease;
}

.risk-modal__close {
  position: absolute;
  top: 16px;
  inset-inline-end: 16px;
  z-index: 1;
  width: 34px;
  height: 34px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-secondary);
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

.risk-modal__close:hover {
  color: var(--text-primary);
  border-color: color-mix(in srgb, var(--security-yellow-border) 72%, var(--border));
  background: rgba(255, 255, 255, 0.14);
  transform: translateY(-1px);
}

.risk-modal__close span {
  display: block;
  transform: translateY(-1px);
}

.risk-modal__header {
  padding-inline-end: 42px;
}

.risk-modal__hero {
  display: flex;
  gap: 14px;
  align-items: flex-start;
}

.risk-modal__icon,
.risk-modal__subject-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.risk-modal__icon {
  width: 54px;
  height: 54px;
  border-radius: 18px;
  border: 1px solid var(--security-yellow-border);
  background: linear-gradient(180deg, rgba(251, 191, 36, 0.24), rgba(245, 158, 11, 0.12));
  color: var(--security-yellow-text);
  box-shadow: 0 20px 40px -28px rgba(245, 158, 11, 0.6);
}

.risk-modal__icon svg {
  width: 22px;
  height: 22px;
}

.risk-modal__copy {
  display: grid;
  gap: 8px;
}

.risk-modal__eyebrow {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.risk-modal__copy h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: clamp(1.08rem, 0.8vw + 0.92rem, 1.35rem);
  line-height: 1.08;
  letter-spacing: -0.03em;
}

.risk-modal__copy p,
.risk-modal__signal {
  color: var(--text-secondary);
  font-size: 10.5px;
  line-height: 1.55;
}

.risk-modal__subject {
  display: flex;
  gap: 14px;
  align-items: center;
  padding: 14px;
  border-radius: 22px;
  border: 1px solid color-mix(in srgb, var(--market-accent-ring) 36%, var(--border));
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--market-accent-soft) 84%, transparent) 0%,
      transparent 54%
    ),
    color-mix(in srgb, var(--panel-bg) 90%, white 4%);
}

.risk-modal__subject-icon {
  width: 58px;
  height: 58px;
  border-radius: 18px;
  background: var(--market-icon-bg);
  border: 1px solid var(--market-icon-border);
  color: var(--market-icon-fg);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 18px 28px -24px var(--market-accent-glow);
  font-size: 20px;
  font-weight: 700;
}

.risk-modal__subject-copy {
  min-width: 0;
  display: grid;
  gap: 8px;
}

.risk-modal__subject-copy strong {
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.25;
}

.risk-modal__summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.risk-modal__stat {
  display: grid;
  gap: 8px;
  min-height: 88px;
  padding: 14px;
  border-radius: 18px;
  border: 1px solid var(--border);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.06), transparent),
    color-mix(in srgb, var(--panel-bg) 94%, white 2%);
}

.risk-modal__stat strong {
  color: var(--text-primary);
  font-size: 0.96rem;
  line-height: 1.35;
}

.risk-modal__signal-list {
  display: grid;
  gap: 8px;
}

.risk-modal__signal {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid var(--security-yellow-border);
  background: color-mix(in srgb, var(--security-yellow-bg) 88%, var(--panel-bg));
}

.risk-modal__signal-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  flex-shrink: 0;
  margin-top: 6px;
  background: #f59e0b;
  box-shadow: 0 0 0 4px rgba(245, 158, 11, 0.14);
}

.risk-modal__actions {
  justify-content: flex-end;
  gap: 8px;
}

.risk-modal__actions button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.risk-modal__cancel-button {
  border-color: var(--border);
  background: var(--panel-bg);
}
.risk-modal__review-button {
  border-color: color-mix(in srgb, var(--security-yellow-border) 72%, var(--border));
  background: color-mix(in srgb, var(--security-yellow-bg) 62%, var(--panel-bg));
}

.risk-modal__confirm-button {
  background: #0f172a;
  border-color: #0f172a;
  color: #ffffff;
  font-weight: 600;
  box-shadow: 0 18px 32px -26px rgba(59, 130, 246, 0.58);
}

.modal-signal-block {
  display: grid;
  gap: 8px;
}

.error-banner {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #f87171;
  font-size: 11px;
}

.error-banner button {
  border: 0;
  background: transparent;
  color: inherit;
  font-size: 20px;
  cursor: pointer;
}

@media (max-width: 1400px) {
  .results-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 1200px) {
  .hero-headline,
  .security-overview {
    grid-template-columns: 1fr;
  }

  .detail-hero-layout {
    grid-template-columns: 1fr;
  }

  .detail-header-side {
    max-width: none;
  }

  .detail-actions--inline {
    flex-direction: row;
    align-items: center;
  }

  .advisor-skill-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .security-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 980px) {
  .discover-progress__header {
    align-items: flex-start;
  }

  .detail-meta {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .detail-hero-stats--compact {
    grid-template-columns: 1fr;
  }

  .advisor-skill-grid {
    grid-template-columns: 1fr;
  }

  .results-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .toolbar-panel,
  .results-panel,
  .detail-card,
  .detail-empty {
    padding: 10px;
    border-radius: 16px;
  }

  .filter-grid,
  .detail-meta,
  .results-grid,
  .security-summary-grid {
    grid-template-columns: 1fr;
  }

  .hero-headline,
  .toolbar-search-row,
  .toolbar-controls,
  .advisor-panel__header,
  .advisor-skill-card__top,
  .advisor-skill-card__actions,
  .discover-progress__header,
  .discover-progress__meta,
  .discover-activity__header,
  .panel-header,
  .detail-header,
  .card-topline,
  .card-footer,
  .detail-actions,
  .section-heading {
    flex-direction: column;
    align-items: stretch;
  }

  .discover-progress__percent {
    justify-items: start;
    text-align: start;
  }

  .discover-activity__item,
  .discover-activity__tail {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .discover-activity__item time {
    grid-column: 2;
  }

  .card-hero-main,
  .detail-hero-main {
    flex-direction: column;
  }

  .detail-header-side {
    justify-content: flex-start;
  }

  .detail-actions--inline {
    flex-direction: column;
    align-items: stretch;
  }

  .card-icon {
    width: 56px;
    height: 56px;
    border-radius: 18px;
  }

  .detail-icon {
    width: 62px;
    height: 62px;
    border-radius: 18px;
  }

  .detail-utility-button {
    width: 46px;
    height: 46px;
  }

  .store-detail-modal-backdrop {
    padding: 10px;
  }

  .store-detail-modal-shell {
    gap: 10px;
  }

  .store-detail-modal-card {
    border-radius: 18px;
  }

  .risk-modal {
    width: min(100vw - 20px, 560px);
    padding: 18px;
    gap: 14px;
    border-radius: 24px;
  }

  .risk-modal__hero,
  .risk-modal__subject,
  .risk-modal__actions {
    flex-direction: column;
    align-items: stretch;
  }

  .risk-modal__header {
    padding-inline-end: 34px;
  }

  .risk-modal__summary {
    grid-template-columns: 1fr;
  }

  .risk-modal__actions button {
    width: 100%;
  }
}

@keyframes risk-modal-rise {
  from {
    opacity: 0;
    transform: translateY(12px) scale(0.98);
  }

  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
