import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import router from '@/router'
import { i18n } from '@/i18n'
import type { DeepResearchJobSummary } from '@/api/deepResearch'
import { useChatStore } from '@/stores/chat'
import { useNotificationStore } from '@/stores/notification'

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

export const useDeepResearchJobsStore = defineStore('deepResearchJobs', () => {
  const jobMap = ref<Record<string, DeepResearchJobSummary>>({})
  const activeJobIds = ref<string[]>([])
  const loading = ref(false)
  const hydrated = ref(false)
  const pendingFocusJobId = ref<string | null>(null)
  const terminalNotifiedJobIds = new Set<string>()

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
    const merged: DeepResearchJobSummary = {
      ...previous,
      ...normalized,
      id: normalized.id || previous?.id || normalized.job_id,
      job_id: normalized.job_id,
    }
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

  function replaceActiveJobs(jobs: DeepResearchJobSummary[]) {
    const nextActiveIds: string[] = []
    const nextMap = { ...jobMap.value }

    for (const job of jobs) {
      const normalized = normalizeJobSnapshot(job)
      if (!normalized) continue
      nextMap[normalized.job_id] = {
        ...nextMap[normalized.job_id],
        ...normalized,
      }
      if (!isTerminalStatus(normalized.status)) {
        nextActiveIds.push(normalized.job_id)
      }
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
    loading.value = true
    try {
      const { deepResearchApi } = await loadDeepResearchApiModule()
      const response = await deepResearchApi.listJobs('active')
      replaceActiveJobs(response.data || [])
      hydrated.value = true
    } finally {
      loading.value = false
    }
  }

  function notifyTerminalState(job: DeepResearchJobSummary) {
    if (!job?.job_id || terminalNotifiedJobIds.has(job.job_id)) return
    terminalNotifiedJobIds.add(job.job_id)

    const t = i18n.global.t.bind(i18n.global)
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

    const title = job.query || t('chat.deepResearchTitle', 'Deep Research')
    if (job.status === 'completed') {
      notificationStore.success(title, t('chat.deepResearchTaskCompleted', 'Research completed'), {
        action,
        duration: 8000,
        messageKey: 'chat.deepResearchTaskCompleted',
      })
      return
    }
    if (job.status === 'failed') {
      notificationStore.error(title, t('chat.deepResearchTaskFailed', 'Research failed'), {
        action,
        messageKey: 'chat.deepResearchTaskFailed',
      })
      return
    }
    if (job.status === 'cancelled') {
      notificationStore.info(title, t('chat.deepResearchTaskCancelled', 'Research cancelled'), {
        action,
        duration: 6000,
        messageKey: 'chat.deepResearchTaskCancelled',
      })
    }
  }

  function handleGlobalEvent(
    type: string,
    payload: Partial<DeepResearchJobSummary> & { id?: string; job_id?: string }
  ) {
    const merged = applyJobSnapshot({
      ...payload,
      status: normalizeString(payload.status) || EVENT_TO_STATUS[type] || undefined,
      updated_at: normalizeString(payload.updated_at) || new Date().toISOString(),
    })
    if (merged && isTerminalStatus(merged.status)) {
      notifyTerminalState(merged)
    }
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

  return {
    activeJobs,
    hasActiveJobs,
    jobMap,
    loading,
    hydrated,
    pendingFocusJobId,
    fetchActiveJobs,
    applyJobSnapshot,
    handleGlobalEvent,
    cancelJob,
    openJob,
    consumePendingFocusJobId,
  }
})
