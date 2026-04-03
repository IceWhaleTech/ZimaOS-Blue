import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import router from '@/router'
import { i18n } from '@/i18n'
import { offSSEEvent, onSSEEvent } from '@/composables/useEventStream'
import type { DeepResearchJobSummary } from '@/api/deepResearch'
import { useChatStore } from '@/stores/chat'
import { useNotificationStore } from '@/stores/notification'
import { localizeResearchSurfaceTitle } from '@/utils/deepResearchText'

type DeepResearchApiModule = typeof import('@/api/deepResearch')
let deepResearchApiModulePromise: Promise<DeepResearchApiModule> | null = null

function loadDeepResearchApiModule(): Promise<DeepResearchApiModule> {
  if (!deepResearchApiModulePromise) {
    deepResearchApiModulePromise = import('@/api/deepResearch')
  }
  return deepResearchApiModulePromise
}

const TERMINAL_STATUSES = new Set(['completed', 'failed', 'cancelled'])
const EVENT_TO_STATUS: Record<string, string> = {
  'deep_research.job_completed': 'completed',
  'deep_research.job_failed': 'failed',
  'deep_research.job_cancelled': 'cancelled',
}
const DEEP_RESEARCH_JOB_EVENT_TYPES = [
  'deep_research.job_created',
  'deep_research.job_updated',
  'deep_research.job_completed',
  'deep_research.job_failed',
  'deep_research.job_cancelled',
] as const
const DETAIL_HYDRATION_DELAY_MS = 120

function isTerminalStatus(status?: string | null) {
  return TERMINAL_STATUSES.has(
    String(status || '')
      .trim()
      .toLowerCase()
  )
}

function compareJobsByUpdatedAt(a: DeepResearchJobSummary, b: DeepResearchJobSummary) {
  return Date.parse(b.updated_at || '') - Date.parse(a.updated_at || '')
}

function normalizeString(value: unknown): string {
  return String(value ?? '').trim()
}

function normalizeNumber(value: unknown): number {
  const next = Number(value)
  return Number.isFinite(next) ? next : 0
}

function normalizeJobSnapshot(
  snapshot: (Partial<DeepResearchJobSummary> & { id?: string; job_id?: string }) | null | undefined
): DeepResearchJobSummary | null {
  if (!snapshot) return null
  const jobId = normalizeString(snapshot.job_id || snapshot.id)
  if (!jobId) return null
  const updatedAt = normalizeString(snapshot.updated_at) || new Date().toISOString()
  return {
    id: normalizeString(snapshot.id) || jobId,
    job_id: jobId,
    query: normalizeString(snapshot.query),
    status: normalizeString(snapshot.status) || 'running',
    stage: normalizeString(snapshot.stage) || 'running',
    progress: normalizeNumber(snapshot.progress),
    iteration: normalizeNumber(snapshot.iteration),
    latest_action: normalizeString(snapshot.latest_action),
    latest_gap: normalizeString(snapshot.latest_gap),
    conversation_id: normalizeString(snapshot.conversation_id) || undefined,
    updated_at: updatedAt,
  }
}

function shouldPreferIncomingSnapshot(
  existing: DeepResearchJobSummary | undefined,
  incoming: DeepResearchJobSummary
): boolean {
  if (!existing) return true
  const incomingUpdatedAt = Date.parse(incoming.updated_at || '')
  const existingUpdatedAt = Date.parse(existing.updated_at || '')
  if (!Number.isFinite(incomingUpdatedAt) || !Number.isFinite(existingUpdatedAt)) return true
  return incomingUpdatedAt >= existingUpdatedAt
}

function mergeJobSnapshot(
  existing: DeepResearchJobSummary | undefined,
  incoming: DeepResearchJobSummary
): DeepResearchJobSummary {
  if (!existing) return incoming
  const preferIncoming = shouldPreferIncomingSnapshot(existing, incoming)
  const merged = preferIncoming ? { ...existing, ...incoming } : { ...incoming, ...existing }
  return {
    ...merged,
    id: incoming.id || existing.id || incoming.job_id,
    job_id: incoming.job_id,
    query: preferIncoming
      ? incoming.query || existing.query || ''
      : existing.query || incoming.query || '',
    latest_action: preferIncoming
      ? incoming.latest_action || existing.latest_action || ''
      : existing.latest_action || incoming.latest_action || '',
    latest_gap: preferIncoming
      ? incoming.latest_gap || existing.latest_gap || ''
      : existing.latest_gap || incoming.latest_gap || '',
    conversation_id: preferIncoming
      ? incoming.conversation_id || existing.conversation_id
      : existing.conversation_id || incoming.conversation_id,
    updated_at: preferIncoming
      ? incoming.updated_at || existing.updated_at || new Date().toISOString()
      : existing.updated_at || incoming.updated_at || new Date().toISOString(),
  }
}

export const useDeepResearchJobsStore = defineStore('deepResearchJobs', () => {
  const jobMap = ref<Record<string, DeepResearchJobSummary>>({})
  const activeJobIds = ref<string[]>([])
  const loading = ref(false)
  const hydrated = ref(false)
  const pendingFocusJobId = ref<string | null>(null)
  const terminalNotifiedJobIds = new Set<string>()
  const detailHydrationTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const sseHandlers = new Map<string, (payload: unknown) => void>()
  let sseListening = false
  let activeJobsRefresh: Promise<void> | null = null

  const activeJobs = computed(() => {
    return activeJobIds.value
      .map((jobId) => jobMap.value[jobId])
      .filter((job): job is DeepResearchJobSummary => !!job)
      .sort(compareJobsByUpdatedAt)
  })
  const hasActiveJobs = computed(() => activeJobs.value.length > 0)

  function upsertActiveJobId(jobId: string) {
    if (!jobId) return
    if (!activeJobIds.value.includes(jobId)) {
      activeJobIds.value = [...activeJobIds.value, jobId]
    }
    activeJobIds.value = [...activeJobIds.value].sort((left, right) => {
      const leftJob = jobMap.value[left]
      const rightJob = jobMap.value[right]
      if (!leftJob || !rightJob) return 0
      return compareJobsByUpdatedAt(leftJob, rightJob)
    })
  }

  function removeActiveJobId(jobId: string) {
    activeJobIds.value = activeJobIds.value.filter((id) => id !== jobId)
  }

  function applyJobSnapshot(
    snapshot:
      | (Partial<DeepResearchJobSummary> & { id?: string; job_id?: string })
      | null
      | undefined
  ) {
    const normalized = normalizeJobSnapshot(snapshot)
    if (!normalized) return null

    const previous = jobMap.value[normalized.job_id]
    const merged = mergeJobSnapshot(previous, normalized)
    jobMap.value = {
      ...jobMap.value,
      [normalized.job_id]: merged,
    }

    if (isTerminalStatus(merged.status)) {
      removeActiveJobId(merged.job_id)
    } else {
      upsertActiveJobId(merged.job_id)
    }

    return merged
  }

  function replaceActiveJobs(jobs: DeepResearchJobSummary[], preservedActiveIds: string[] = []) {
    const nextActiveIds: string[] = []
    const nextMap = { ...jobMap.value }

    for (const job of jobs) {
      const normalized = normalizeJobSnapshot(job)
      if (!normalized) continue
      const merged = mergeJobSnapshot(nextMap[normalized.job_id], normalized)
      nextMap[normalized.job_id] = merged
      if (!isTerminalStatus(merged.status)) {
        nextActiveIds.push(normalized.job_id)
      }
    }

    for (const jobId of preservedActiveIds) {
      const job = nextMap[jobId]
      if (!job || isTerminalStatus(job.status) || nextActiveIds.includes(jobId)) continue
      nextActiveIds.push(jobId)
    }

    jobMap.value = nextMap
    activeJobIds.value = nextActiveIds.sort((left, right) => {
      const leftJob = nextMap[left]
      const rightJob = nextMap[right]
      if (!leftJob || !rightJob) return 0
      return compareJobsByUpdatedAt(leftJob, rightJob)
    })
  }

  async function fetchActiveJobs() {
    if (activeJobsRefresh) return activeJobsRefresh

    const activeIdsAtRequestStart = new Set(activeJobIds.value)
    loading.value = true
    activeJobsRefresh = (async () => {
      const { deepResearchApi } = await loadDeepResearchApiModule()
      const response = await deepResearchApi.listJobs('active')
      const preservedActiveIds = activeJobIds.value.filter((jobId) => {
        const job = jobMap.value[jobId]
        return !activeIdsAtRequestStart.has(jobId) && !!job && !isTerminalStatus(job.status)
      })
      replaceActiveJobs(response.data || [], preservedActiveIds)
      hydrated.value = true
    })()

    try {
      await activeJobsRefresh
    } finally {
      loading.value = false
      activeJobsRefresh = null
    }
  }

  async function ensureHydrated() {
    if (hydrated.value) return
    await fetchActiveJobs()
  }

  async function hydrateJobByID(jobId: string) {
    const normalizedJobId = normalizeString(jobId)
    if (!normalizedJobId) return
    const { deepResearchApi } = await loadDeepResearchApiModule()
    const response = await deepResearchApi.getJob(normalizedJobId)
    const merged = applyJobSnapshot(response.data)
    if (merged && isTerminalStatus(merged.status)) {
      notifyTerminalState(merged)
    }
  }

  function scheduleJobHydration(jobId: string, delayMs = DETAIL_HYDRATION_DELAY_MS) {
    const normalizedJobId = normalizeString(jobId)
    if (!normalizedJobId || detailHydrationTimers.has(normalizedJobId)) return
    detailHydrationTimers.set(
      normalizedJobId,
      setTimeout(() => {
        detailHydrationTimers.delete(normalizedJobId)
        void hydrateJobByID(normalizedJobId).catch(() => {})
      }, Math.max(0, delayMs))
    )
  }

  function notifyTerminalState(job: DeepResearchJobSummary) {
    if (!job?.job_id || terminalNotifiedJobIds.has(job.job_id)) return
    terminalNotifiedJobIds.add(job.job_id)

    const t = i18n.global.t.bind(i18n.global)
    const te = i18n.global.te.bind(i18n.global)
    const translate = (key: string, fallback: string) => (te(key) ? String(t(key)) : fallback)
    const notificationStore = useNotificationStore()
    const action = job.conversation_id
      ? {
          label: t('chat.deepResearchBackToTask', 'View result'),
          labelKey: 'chat.deepResearchBackToTask',
          handler: () => {
            void openJob(job.job_id, job.conversation_id)
          },
        }
      : undefined

    const title = job.query || localizeResearchSurfaceTitle(translate)
    if (job.status === 'completed') {
      notificationStore.success(
        title,
        t('chat.deepResearchTaskCompleted', 'Deep Research completed'),
        {
          action,
          duration: 8000,
          messageKey: 'chat.deepResearchTaskCompleted',
        }
      )
      return
    }
    if (job.status === 'failed') {
      notificationStore.error(title, t('chat.deepResearchTaskFailed', 'Deep Research failed'), {
        action,
        messageKey: 'chat.deepResearchTaskFailed',
      })
      return
    }
    if (job.status === 'cancelled') {
      notificationStore.info(
        title,
        t('chat.deepResearchTaskCancelled', 'Deep Research cancelled'),
        {
          action,
          duration: 6000,
          messageKey: 'chat.deepResearchTaskCancelled',
        }
      )
    }
  }

  function handleGlobalEvent(
    type: string,
    payload: Partial<DeepResearchJobSummary> & { id?: string; job_id?: string }
  ) {
    const jobId = normalizeString(payload.job_id || payload.id)
    if (!jobId) return
    const existing = jobMap.value[jobId]
    const merged = applyJobSnapshot({
      ...existing,
      ...payload,
      id: normalizeString(payload.id) || existing?.id || jobId,
      job_id: jobId,
      status: normalizeString(payload.status) || EVENT_TO_STATUS[type] || existing?.status || undefined,
      updated_at:
        normalizeString(payload.updated_at) ||
        existing?.updated_at ||
        (type === 'deep_research.job_created'
          ? '1970-01-01T00:00:00.000Z'
          : new Date().toISOString()),
    })
    const shouldHydrate =
      !existing ||
      type === 'deep_research.job_created' ||
      (merged ? isTerminalStatus(merged.status) : false)

    if (shouldHydrate) {
      scheduleJobHydration(jobId)
    }
    if (merged && isTerminalStatus(merged.status)) {
      notifyTerminalState(merged)
    }
  }

  function ensureSSEListeners() {
    if (sseListening) return
    for (const eventType of DEEP_RESEARCH_JOB_EVENT_TYPES) {
      const handler = (payload: unknown) => {
        handleGlobalEvent(eventType, (payload || {}) as Partial<DeepResearchJobSummary>)
      }
      sseHandlers.set(eventType, handler)
      onSSEEvent(eventType, handler)
    }
    sseListening = true
  }

  function stopSSEListeners() {
    if (!sseListening) return
    for (const [eventType, handler] of sseHandlers.entries()) {
      offSSEEvent(eventType, handler)
    }
    sseHandlers.clear()
    sseListening = false
  }

  async function cancelJob(jobId: string) {
    const normalizedJobId = normalizeString(jobId)
    if (!normalizedJobId) return
    const { deepResearchApi } = await loadDeepResearchApiModule()
    await deepResearchApi.cancelJob(normalizedJobId)
    const existing = jobMap.value[normalizedJobId]
    applyJobSnapshot({
      ...existing,
      id: existing?.id || normalizedJobId,
      job_id: normalizedJobId,
      status: 'cancelled',
      stage: 'cancelled',
      updated_at: new Date().toISOString(),
    })
  }

  async function openJob(jobId: string, conversationId?: string) {
    const normalizedConversationId = normalizeString(conversationId)
    if (!normalizedConversationId) return
    pendingFocusJobId.value = normalizeString(jobId)
    const chatStore = useChatStore()
    await router.push({ name: 'Chat', query: { conversationId: normalizedConversationId } })
    await chatStore.selectConversation(normalizedConversationId)
  }

  function consumePendingFocusJobId() {
    pendingFocusJobId.value = null
  }

  function reset() {
    stopSSEListeners()
    for (const timer of detailHydrationTimers.values()) {
      clearTimeout(timer)
    }
    detailHydrationTimers.clear()
    activeJobsRefresh = null
    jobMap.value = {}
    activeJobIds.value = []
    loading.value = false
    hydrated.value = false
    pendingFocusJobId.value = null
    terminalNotifiedJobIds.clear()
    ensureSSEListeners()
  }

  ensureSSEListeners()
  void ensureHydrated().catch(() => {})

  return {
    activeJobs,
    hasActiveJobs,
    jobMap,
    loading,
    hydrated,
    pendingFocusJobId,
    ensureHydrated,
    fetchActiveJobs,
    applyJobSnapshot,
    handleGlobalEvent,
    cancelJob,
    openJob,
    consumePendingFocusJobId,
    reset,
  }
})
