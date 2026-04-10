import { computed, onBeforeUnmount, ref } from 'vue'
import { offSSEEvent, onSSEEvent } from '@/composables/useEventStream'
import {
  knowledgeApi,
  type KnowledgeCreateJobRequest,
  type KnowledgeJob,
  type KnowledgeJobReport,
} from '@/api/knowledge'

const KNOWLEDGE_JOB_EVENT_TYPES = [
  'knowledge.job_created',
  'knowledge.job_started',
  'knowledge.job_progress',
  'knowledge.job_completed',
  'knowledge.job_failed',
  'knowledge.job_cancelled',
] as const

const TERMINAL_STATUSES = new Set(['completed', 'failed', 'cancelled'])
const EVENT_TO_STATUS: Record<string, string> = {
  'knowledge.job_started': 'running',
  'knowledge.job_progress': 'running',
  'knowledge.job_completed': 'completed',
  'knowledge.job_failed': 'failed',
  'knowledge.job_cancelled': 'cancelled',
}

function normalizeString(value: unknown): string {
  return String(value ?? '').trim()
}

function normalizeNumber(value: unknown): number {
  const next = Number(value)
  return Number.isFinite(next) ? next : 0
}

function normalizeKind(value: unknown): KnowledgeJob['kind'] {
  const kind = normalizeString(value).toLowerCase()
  if (!kind || kind === 'compile') return 'ingest'
  if (kind === 'lint' || kind === 'answer' || kind === 'ingest' || kind === 'repair_conflicts') {
    return kind
  }
  return 'ingest'
}

function normalizeJobSnapshot(snapshot: any): KnowledgeJob | null {
  if (!snapshot || typeof snapshot !== 'object') return null
  const id = normalizeString(snapshot.id || snapshot.job_id)
  if (!id) return null
  return {
    id,
    job_id: normalizeString(snapshot.job_id) || id,
    query: normalizeString(snapshot.query),
    kind: normalizeKind(snapshot.kind),
    status: normalizeString(snapshot.status) || 'pending',
    progress: normalizeNumber(snapshot.progress),
    stage: normalizeString(snapshot.stage),
    detail: normalizeString(snapshot.detail) || undefined,
    updated_at: normalizeString(snapshot.updated_at) || new Date().toISOString(),
    created_at: normalizeString(snapshot.created_at) || new Date().toISOString(),
    completed_at: normalizeString(snapshot.completed_at) || undefined,
    error: normalizeString(snapshot.error) || undefined,
    report: snapshot.report ?? null,
  }
}

function isTerminalStatus(status?: string | null) {
  return TERMINAL_STATUSES.has(normalizeString(status).toLowerCase())
}

export function useKnowledgeJobs(options?: {
  onTerminal?: (job: KnowledgeJob, report: KnowledgeJobReport | null) => void | Promise<void>
}) {
  const currentJob = ref<KnowledgeJob | null>(null)
  const latestReport = ref<KnowledgeJobReport | null>(null)
  const trackedJobIDs = new Set<string>()

  async function tryHydrateJob(jobID: string) {
    try {
      return await hydrateJob(jobID)
    } catch {
      return null
    }
  }

  async function hydrateJob(jobID: string) {
    const response = await knowledgeApi.getJob(jobID)
    const normalized = normalizeJobSnapshot(response.data)
    if (normalized) currentJob.value = normalized
    return normalized
  }

  async function hydrateReport(jobID: string) {
    try {
      const response = await knowledgeApi.getReport(jobID)
      latestReport.value = response.data
      return response.data
    } catch {
      return null
    }
  }

  async function handleEvent(type: string, payload: unknown) {
    const normalized = normalizeJobSnapshot(payload)
    if (!normalized || !trackedJobIDs.has(normalized.id)) return
    normalized.status = EVENT_TO_STATUS[type] || normalized.status
    currentJob.value = normalized
    if (!isTerminalStatus(normalized.status)) return
    trackedJobIDs.delete(normalized.id)
    const hydratedJob = (await hydrateJob(normalized.id)) ?? normalized
    const report = hydratedJob.report ?? (await hydrateReport(normalized.id))
    if (options?.onTerminal) {
      await options.onTerminal(hydratedJob, report)
    }
  }

  async function hydrateLatestActiveJob(options?: { kinds?: KnowledgeJob['kind'][] }) {
    try {
      const response = await knowledgeApi.listJobs('active')
      const allowedKinds = new Set(
        (options?.kinds || []).map((kind) => normalizeKind(kind)).filter(Boolean)
      )
      const matches = (response.data || [])
        .map((snapshot) => normalizeJobSnapshot(snapshot))
        .filter((job): job is KnowledgeJob => !!job)
        .filter((job) => allowedKinds.size === 0 || allowedKinds.has(job.kind))
        .sort((left, right) => {
          return Date.parse(right.updated_at || '') - Date.parse(left.updated_at || '')
        })
      const next = matches[0] || null
      if (!next) return null
      trackedJobIDs.add(next.id)
      currentJob.value = next
      return (await tryHydrateJob(next.id)) ?? next
    } catch {
      return null
    }
  }

  const listeners = KNOWLEDGE_JOB_EVENT_TYPES.map((eventType) => ({
    eventType,
    handler: (payload: unknown) => {
      void handleEvent(eventType, payload)
    },
  }))

  for (const { eventType, handler } of listeners) {
    onSSEEvent(eventType, handler)
  }

  onBeforeUnmount(() => {
    for (const { eventType, handler } of listeners) {
      offSSEEvent(eventType, handler)
    }
  })

  async function runJob(request: KnowledgeCreateJobRequest) {
    const response = await knowledgeApi.createJob(request)
    const normalized = normalizeJobSnapshot(response.data)
    if (normalized) {
      trackedJobIDs.add(normalized.id)
      currentJob.value = normalized
      latestReport.value = normalized.report ?? null
    }
    return normalized
  }

  return {
    currentJob,
    latestReport,
    isRunning: computed(() => !!currentJob.value && !isTerminalStatus(currentJob.value.status)),
    runJob,
    hydrateLatestActiveJob,
    hydrateJob,
    hydrateReport,
  }
}
