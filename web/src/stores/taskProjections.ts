import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import router from '@/router'
import { chatBootstrapApi } from '@/api/chatBootstrap'
import { i18n } from '@/i18n'
import { offSSEEvent, onSSEEvent } from '@/composables/useEventStream'
import type {
  UserTaskActionDescriptor,
  UserTaskActionID,
  UserTaskActions,
  UserTaskProjection,
  UserTaskSubagentSummary,
} from '@/api/tasks'
import { useChatStore } from '@/stores/chat'
import { useNotificationStore } from '@/stores/notification'
import { canOpenTaskConversation } from '@/utils/taskProjectionActions'
import { localizeTaskProjectionPreviewText } from '@/utils/taskProjectionText'

type TasksApiModule = typeof import('@/api/tasks')
type UserTaskActionInput = NonNullable<UserTaskActionDescriptor['input']>
type UserTaskActionInputField = NonNullable<UserTaskActionInput['fields']>[number]
let tasksApiModulePromise: Promise<TasksApiModule> | null = null

function loadTasksApiModule(): Promise<TasksApiModule> {
  if (!tasksApiModulePromise) {
    tasksApiModulePromise = import('@/api/tasks')
  }
  return tasksApiModulePromise
}

function normalizeString(value: unknown): string {
  return String(value ?? '').trim()
}

function normalizeNumber(value: unknown): number {
  const next = Number(value)
  return Number.isFinite(next) ? next : 0
}

function normalizeOptionalNumber(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const next = Number(value)
  return Number.isFinite(next) ? next : undefined
}

function normalizeBoolean(value: unknown): boolean {
  return Boolean(value)
}

function normalizeSubagentSummary(
  summary: Partial<UserTaskSubagentSummary> | null | undefined
): UserTaskSubagentSummary | undefined {
  if (!summary) return undefined
  const total = normalizeNumber(summary.total)
  if (total <= 0) return undefined
  return {
    total,
    running: normalizeOptionalNumber(summary.running),
    waiting_user: normalizeOptionalNumber(summary.waiting_user),
    completed: normalizeOptionalNumber(summary.completed),
    failed: normalizeOptionalNumber(summary.failed),
    cancelled: normalizeOptionalNumber(summary.cancelled),
    latest_title: normalizeString(summary.latest_title) || undefined,
    latest_status: normalizeString(summary.latest_status) || undefined,
    latest_updated_at: normalizeString(summary.latest_updated_at) || undefined,
  }
}

function normalizeTaskActionInputFields(
  fields: UserTaskActionInput['fields'] | null | undefined
): UserTaskActionInputField[] | undefined {
  if (!Array.isArray(fields)) return undefined
  const normalized: UserTaskActionInputField[] = []
  for (const field of fields) {
    const key = normalizeString(field?.key)
    if (!key) continue
    normalized.push({
      key,
      label: normalizeString(field?.label) || key,
      kind: normalizeString(field?.kind) || undefined,
      target: normalizeString(field?.target) || undefined,
      payload_key: normalizeString(field?.payload_key) || undefined,
      required: normalizeBoolean(field?.required),
      placeholder: normalizeString(field?.placeholder) || undefined,
      options: Array.isArray(field?.options)
        ? field.options.map((value) => normalizeString(value)).filter(Boolean)
        : undefined,
    })
  }
  return normalized
}

function normalizeTaskActionDescriptor(
  descriptor: Partial<UserTaskActionDescriptor> | null | undefined
): UserTaskActionDescriptor | null {
  if (!descriptor) return null
  const id = normalizeString(descriptor.id)
  const method = normalizeString(descriptor.method).toUpperCase() || 'POST'
  const path = normalizeString(descriptor.path)
  if (!id || !path) return null
  return {
    id,
    label: normalizeString(descriptor.label) || id,
    method,
    path,
    variant: normalizeString(descriptor.variant) || undefined,
    requires_input: normalizeBoolean(descriptor.requires_input),
    input: descriptor.input
      ? {
          ...descriptor.input,
          fields: normalizeTaskActionInputFields(descriptor.input.fields),
          title: normalizeString(descriptor.input.title) || undefined,
          description: normalizeString(descriptor.input.description) || undefined,
          submit_label: normalizeString(descriptor.input.submit_label) || undefined,
        }
      : undefined,
  }
}

function normalizeTaskActions(
  actions: Partial<UserTaskActions> | null | undefined
): UserTaskActions {
  return {
    items: Array.isArray(actions?.items)
      ? actions.items
          .map((descriptor) => normalizeTaskActionDescriptor(descriptor))
          .filter((descriptor): descriptor is UserTaskActionDescriptor => !!descriptor)
      : [],
  }
}

function findTaskActionDescriptor(
  actions: Partial<UserTaskActions> | null | undefined,
  action: UserTaskActionID
): UserTaskActionDescriptor | null {
  const actionID = normalizeString(action)
  if (!actionID || !Array.isArray(actions?.items)) return null
  return actions.items.find((descriptor) => normalizeString(descriptor?.id) === actionID) || null
}

function isTaskActive(task: Pick<UserTaskProjection, 'status'> | null | undefined): boolean {
  if (!task) return false
  return task.status === 'running' || task.status === 'waiting_user'
}

function compareTasksByUpdatedAt(left: UserTaskProjection, right: UserTaskProjection) {
  return Date.parse(right.updated_at || '') - Date.parse(left.updated_at || '')
}

function normalizeTaskSnapshot(
  snapshot: Partial<UserTaskProjection> | null | undefined
): UserTaskProjection | null {
  if (!snapshot) return null
  const id = normalizeString(snapshot.id)
  if (!id) return null
  return {
    id,
    kind: (normalizeString(snapshot.kind) || 'agent_task') as UserTaskProjection['kind'],
    conversation_id: normalizeString(snapshot.conversation_id) || undefined,
    scope: (normalizeString(snapshot.scope) || 'current') as UserTaskProjection['scope'],
    title: normalizeString(snapshot.title),
    subtitle: normalizeString(snapshot.subtitle) || undefined,
    status: (normalizeString(snapshot.status) || 'running') as UserTaskProjection['status'],
    stage: (normalizeString(snapshot.stage) || 'planning') as UserTaskProjection['stage'],
    progress: normalizeNumber(snapshot.progress),
    blocker: snapshot.blocker,
    result_preview: normalizeString(snapshot.result_preview) || undefined,
    error_preview: normalizeString(snapshot.error_preview) || undefined,
    artifacts: Array.isArray(snapshot.artifacts) ? snapshot.artifacts : [],
    research_sources: Array.isArray(snapshot.research_sources) ? snapshot.research_sources : [],
    subagent_summary: normalizeSubagentSummary(snapshot.subagent_summary),
    actions: normalizeTaskActions(snapshot.actions),
    run_status: normalizeString(snapshot.run_status) || undefined,
    verification_status: normalizeString(snapshot.verification_status) || undefined,
    score: normalizeOptionalNumber(snapshot.score),
    evidence_count: normalizeOptionalNumber(snapshot.evidence_count),
    detail_href: normalizeString(snapshot.detail_href) || undefined,
    updated_at: normalizeString(snapshot.updated_at) || new Date().toISOString(),
    finished_at: normalizeString(snapshot.finished_at) || undefined,
  }
}

const TASK_PROJECTION_EVENT_TYPES = [
  'task_created',
  'task_planning',
  'task_progress',
  'task_step_completed',
  'task_reflection_started',
  'task_reflection_completed',
  'task_completed',
  'task_failed',
  'task_cancelled',
  'task_user_message',
  'task_question',
  'task_question_answered',
  'deep_research.job_created',
  'deep_research.job_updated',
  'deep_research.job_completed',
  'deep_research.job_failed',
  'deep_research.job_cancelled',
] as const

const EVENT_STATUS_MAP: Partial<Record<(typeof TASK_PROJECTION_EVENT_TYPES)[number], UserTaskProjection['status']>> =
  {
    task_created: 'running',
    task_planning: 'running',
    task_progress: 'running',
    task_step_completed: 'running',
    task_reflection_started: 'running',
    task_reflection_completed: 'running',
    task_user_message: 'running',
    task_question_answered: 'running',
    task_question: 'waiting_user',
    task_completed: 'completed',
    task_failed: 'failed',
    task_cancelled: 'cancelled',
    'deep_research.job_created': 'running',
    'deep_research.job_updated': 'running',
    'deep_research.job_completed': 'completed',
    'deep_research.job_failed': 'failed',
    'deep_research.job_cancelled': 'cancelled',
  }

const EVENT_STAGE_MAP: Partial<Record<(typeof TASK_PROJECTION_EVENT_TYPES)[number], UserTaskProjection['stage']>> =
  {
    task_created: 'planning',
    task_planning: 'planning',
    task_progress: 'working',
    task_step_completed: 'working',
    task_reflection_started: 'verifying',
    task_reflection_completed: 'verifying',
    task_user_message: 'working',
    task_question: 'waiting_user',
    task_question_answered: 'working',
    task_completed: 'completed',
    task_failed: 'failed',
    task_cancelled: 'cancelled',
  }

function normalizeTaskEventID(payload: Record<string, unknown> | null | undefined): string {
  return normalizeString(payload?.task_id || payload?.id || payload?.job_id)
}

function normalizeTaskEventStatus(value: unknown): UserTaskProjection['status'] | undefined {
  switch (normalizeString(value)) {
    case 'running':
    case 'waiting_user':
    case 'completed':
    case 'failed':
    case 'cancelled':
      return normalizeString(value) as UserTaskProjection['status']
    default:
      return undefined
  }
}

function normalizeTaskEventStage(value: unknown): UserTaskProjection['stage'] | undefined {
  switch (normalizeString(value)) {
    case 'planning':
    case 'working':
    case 'verifying':
    case 'waiting_user':
    case 'completed':
    case 'partial':
    case 'failed':
    case 'cancelled':
      return normalizeString(value) as UserTaskProjection['stage']
    default:
      return undefined
  }
}

function isTerminalTaskStatus(status: string | undefined): boolean {
  return status === 'completed' || status === 'failed' || status === 'cancelled'
}

export const useTaskProjectionsStore = defineStore('taskProjections', () => {
  const RECENT_OUTCOME_TIMEOUT_MS = 5 * 60 * 1000
  const DETAIL_HYDRATION_DELAY_MS = 120
  const currentConversationId = ref('')
  const currentTasks = ref<UserTaskProjection[]>([])
  const backgroundTasks = ref<UserTaskProjection[]>([])
  const recentOutcome = ref<UserTaskProjection | null>(null)
  const loading = ref(false)
  const hydrated = ref(false)
  const activeRefresh = ref<Promise<void> | null>(null)
  const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)
  const notifiedTerminalTaskIds = new Set<string>()
  let recentOutcomeTimer: ReturnType<typeof setTimeout> | null = null
  let previousCurrentTasks: UserTaskProjection[] = []
  let previousBackgroundTasks: UserTaskProjection[] = []
  let previousConversationId = ''
  let sseListening = false
  const sseHandlers = new Map<string, (payload: unknown) => void>()
  const detailHydrationTimers = new Map<string, ReturnType<typeof setTimeout>>()

  const currentActiveTasks = computed(() => currentTasks.value.filter((task) => isTaskActive(task)))
  const currentTerminalTasks = computed(() =>
    currentTasks.value.filter((task) => !isTaskActive(task))
  )
  const hasActiveTasks = computed(
    () =>
      currentActiveTasks.value.length > 0 ||
      backgroundTasks.value.some((task) => isTaskActive(task))
  )

  function stopPolling() {
    if (pollTimer.value) {
      clearInterval(pollTimer.value)
      pollTimer.value = null
    }
  }

  function clearRecentOutcomeTimer() {
    if (!recentOutcomeTimer) return
    clearTimeout(recentOutcomeTimer)
    recentOutcomeTimer = null
  }

  function setRecentOutcome(task: UserTaskProjection | null) {
    clearRecentOutcomeTimer()
    recentOutcome.value = task
    if (!task) return
    recentOutcomeTimer = setTimeout(() => {
      recentOutcome.value = null
      recentOutcomeTimer = null
    }, RECENT_OUTCOME_TIMEOUT_MS)
  }

  function dismissRecentOutcome() {
    setRecentOutcome(null)
  }

  function syncPreviousTaskSnapshots() {
    previousCurrentTasks = [...currentTasks.value]
    previousBackgroundTasks = [...backgroundTasks.value]
    previousConversationId = normalizeString(currentConversationId.value)
  }

  function syncPolling() {
    if (!hasActiveTasks.value) {
      stopPolling()
      return
    }
    if (pollTimer.value) return
    pollTimer.value = setInterval(() => {
      void refreshNow().catch(() => {})
    }, 3000)
  }

  function findExistingTask(taskID: string): UserTaskProjection | null {
    return (
      currentTasks.value.find((task) => task.id === taskID) ||
      backgroundTasks.value.find((task) => task.id === taskID) ||
      null
    )
  }

  function shouldTreatAsCurrentTask(
    conversationId: string | undefined,
    existing: UserTaskProjection | null
  ): boolean {
    if (existing?.scope === 'current') return true
    const activeConversationId = normalizeString(currentConversationId.value)
    const taskConversationId = normalizeString(conversationId)
    return !!activeConversationId && !!taskConversationId && activeConversationId === taskConversationId
  }

  function upsertTaskSnapshot(snapshot: Partial<UserTaskProjection>) {
    const taskID = normalizeString(snapshot.id)
    if (!taskID) return null

    const existing = findExistingTask(taskID)
    const normalized = normalizeTaskSnapshot({
      ...existing,
      ...snapshot,
      id: taskID,
      actions: snapshot.actions ?? existing?.actions ?? { items: [] },
      artifacts: snapshot.artifacts ?? existing?.artifacts ?? [],
      research_sources: snapshot.research_sources ?? existing?.research_sources ?? [],
      updated_at: normalizeString(snapshot.updated_at) || new Date().toISOString(),
    })
    if (!normalized) return null

    const keepInCurrent = shouldTreatAsCurrentTask(normalized.conversation_id, existing)
    const keepInBackground = !keepInCurrent && isTaskActive(normalized)

    let nextCurrent = currentTasks.value.filter((task) => task.id !== normalized.id)
    let nextBackground = backgroundTasks.value.filter((task) => task.id !== normalized.id)

    if (keepInCurrent) {
      normalized.scope = 'current'
      nextCurrent = [...nextCurrent, normalized].sort(compareTasksByUpdatedAt)
    } else if (keepInBackground) {
      normalized.scope = 'background'
      nextBackground = [...nextBackground, normalized].sort(compareTasksByUpdatedAt)
    } else if (existing?.scope === 'current') {
      normalized.scope = 'current'
      nextCurrent = [...nextCurrent, normalized].sort(compareTasksByUpdatedAt)
    } else {
      normalized.scope = 'background'
    }

    currentTasks.value = nextCurrent
    backgroundTasks.value = nextBackground
    return {
      task: normalized,
      previous: existing,
    }
  }

  function hydrateConversationIdForTask(conversationId: string | undefined): string | undefined {
    const normalizedConversationId = normalizeString(conversationId)
    const activeConversationId = normalizeString(currentConversationId.value)
    if (!normalizedConversationId || !activeConversationId) return undefined
    return normalizedConversationId === activeConversationId ? activeConversationId : undefined
  }

  async function hydrateTaskByID(taskID: string, conversationId?: string) {
    const normalizedTaskID = normalizeString(taskID)
    if (!normalizedTaskID) return
    const { taskProjectionApi } = await loadTasksApiModule()
    const response = await taskProjectionApi.getTask(
      normalizedTaskID,
      hydrateConversationIdForTask(conversationId)
    )
    const detail = normalizeTaskSnapshot(response.data)
    if (!detail) return
    const merged = upsertTaskSnapshot(detail)
    if (!merged) return
    if (merged.previous && isTaskActive(merged.previous) && !isTaskActive(merged.task)) {
      if (merged.previous.scope === 'background' || merged.task.scope === 'background') {
        notifyTaskTerminalState(merged.task)
      }
      setRecentOutcome(merged.task)
    }
    syncPreviousTaskSnapshots()
    syncPolling()
  }

  function scheduleTaskDetailHydration(taskID: string, conversationId?: string, delayMs = DETAIL_HYDRATION_DELAY_MS) {
    const normalizedTaskID = normalizeString(taskID)
    if (!normalizedTaskID || detailHydrationTimers.has(normalizedTaskID)) return
    detailHydrationTimers.set(
      normalizedTaskID,
      setTimeout(() => {
        detailHydrationTimers.delete(normalizedTaskID)
        void hydrateTaskByID(normalizedTaskID, conversationId).catch(() => {})
      }, Math.max(0, delayMs))
    )
  }

  function handleProjectionEvent(type: string, rawPayload: unknown) {
    const payload = (rawPayload || {}) as Record<string, unknown>
    const taskID = normalizeTaskEventID(payload)
    if (!taskID) return

    const existing = findExistingTask(taskID)
    const status =
      normalizeTaskEventStatus(payload.status) ||
      EVENT_STATUS_MAP[type as keyof typeof EVENT_STATUS_MAP] ||
      existing?.status ||
      'running'
    const stage =
      normalizeTaskEventStage(payload.stage) ||
      EVENT_STAGE_MAP[type as keyof typeof EVENT_STAGE_MAP] ||
      existing?.stage ||
      'working'
    const progress =
      normalizeOptionalNumber(payload.progress) ??
      (isTerminalTaskStatus(status) ? 100 : existing?.progress ?? 0)
    const conversationId = normalizeString(payload.conversation_id) || existing?.conversation_id
    const title = existing?.title || normalizeString(payload.query) || undefined
    const eventMessage = normalizeString(payload.message)

    const merged = upsertTaskSnapshot({
      id: taskID,
      kind: existing?.kind || (type.startsWith('deep_research.') ? 'research' : 'agent_task'),
      conversation_id: conversationId,
      title,
      subtitle: existing?.subtitle,
      status,
      stage,
      progress,
      updated_at: normalizeString(payload.updated_at) || new Date().toISOString(),
      result_preview: status === 'completed' ? eventMessage || existing?.result_preview : existing?.result_preview,
      error_preview: status === 'failed' ? eventMessage || existing?.error_preview : existing?.error_preview,
    })
    if (!merged) return

    if (
      (!existing && isTerminalTaskStatus(status)) ||
      (existing && isTaskActive(existing) && !isTaskActive(merged.task))
    ) {
      if (existing?.scope === 'background' || merged.task.scope === 'background') {
        notifyTaskTerminalState(merged.task)
      }
      setRecentOutcome(merged.task)
    }

    syncPreviousTaskSnapshots()
    syncPolling()

    const shouldHydrate =
      !existing ||
      isTerminalTaskStatus(status) ||
      type === 'task_created' ||
      type === 'task_question' ||
      type === 'task_question_answered' ||
      type.startsWith('deep_research.')

    if (shouldHydrate) {
      scheduleTaskDetailHydration(taskID, conversationId)
    }
  }

  function ensureSSEListeners() {
    if (sseListening) return
    for (const eventType of TASK_PROJECTION_EVENT_TYPES) {
      const handler = (payload: unknown) => {
        handleProjectionEvent(eventType, payload)
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

  function normalizeTaskProjectionList(
    tasks: UserTaskProjection[] | Partial<UserTaskProjection>[] | null | undefined
  ): UserTaskProjection[] {
    return (tasks || [])
      .map((task) => normalizeTaskSnapshot(task))
      .filter((task): task is UserTaskProjection => !!task)
      .sort(compareTasksByUpdatedAt)
  }

  async function fetchBootstrapTaskSnapshot(conversationId: string): Promise<{
    currentTasks: UserTaskProjection[]
    backgroundTasks: UserTaskProjection[]
  }> {
    if (!conversationId) {
      return {
        currentTasks: [],
        backgroundTasks: [],
      }
    }
    try {
      const response = await chatBootstrapApi.getConversationBootstrap(conversationId)
      return {
        currentTasks: normalizeTaskProjectionList(response.data?.current_tasks),
        backgroundTasks: normalizeTaskProjectionList(response.data?.background_tasks),
      }
    } catch {
      return {
        currentTasks: [],
        backgroundTasks: [],
      }
    }
  }

  function notifyTaskTerminalState(task: UserTaskProjection) {
    if (!task?.id || notifiedTerminalTaskIds.has(task.id)) return
    notifiedTerminalTaskIds.add(task.id)

    const t = i18n.global.t.bind(i18n.global)
    const translate = (key: string, fallback: string) =>
      i18n.global.te(key) ? String(t(key)) : fallback
    const notificationStore = useNotificationStore()
    const action = canOpenTaskConversation(task)
      ? {
          label: t('chat.taskBackToConversation', 'Back to task'),
          handler: () => {
            void openTask(task)
          },
        }
      : undefined

    const title = task.title || t('chat.taskNotificationTitle', 'Background task')
    if (task.status === 'completed') {
      notificationStore.success(title, t('chat.taskCompleted', 'Task completed'), {
        action,
        duration: 8000,
      })
      return
    }
    if (task.status === 'failed') {
      const message =
        localizeTaskProjectionPreviewText(task.error_preview, translate) ||
        t('chat.taskFailed', 'Task failed')
      notificationStore.error(title, message, {
        action,
      })
      return
    }
    if (task.status === 'cancelled') {
      notificationStore.info(title, t('chat.taskCancelled', 'Task cancelled'), {
        action,
        duration: 6000,
      })
    }
  }

  async function collectTerminalBackgroundTransitions(): Promise<UserTaskProjection[]> {
    const terminalTransitions: UserTaskProjection[] = []
    const nextIds = new Set(backgroundTasks.value.map((task) => task.id))
    const removed = previousBackgroundTasks.filter((task) => !nextIds.has(task.id))
    if (removed.length === 0) return terminalTransitions

    const { taskProjectionApi } = await loadTasksApiModule()
    await Promise.all(
      removed.map(async (task) => {
        try {
          const response = await taskProjectionApi.getTask(task.id)
          const detail = normalizeTaskSnapshot(response.data)
          if (!detail || isTaskActive(detail)) return
          terminalTransitions.push(detail)
          notifyTaskTerminalState(detail)
        } catch {
          // Ignore missing task details
        }
      })
    )
    return terminalTransitions
  }

  function collectCurrentTerminalTransitions(): UserTaskProjection[] {
    const previousActiveIds = new Set(
      previousCurrentTasks.filter((task) => isTaskActive(task)).map((task) => task.id)
    )
    if (previousActiveIds.size === 0) return []
    return currentTasks.value.filter(
      (task) => !isTaskActive(task) && previousActiveIds.has(task.id)
    )
  }

  async function handleTaskTransitions(conversationId: string) {
    if (previousConversationId !== conversationId) {
      previousConversationId = conversationId
      previousCurrentTasks = [...currentTasks.value]
      previousBackgroundTasks = [...backgroundTasks.value]
      return
    }

    const currentTransitions = collectCurrentTerminalTransitions()
    const backgroundTransitions = await collectTerminalBackgroundTransitions()
    const latestTransition = [...currentTransitions, ...backgroundTransitions].sort(
      compareTasksByUpdatedAt
    )[0]

    previousCurrentTasks = [...currentTasks.value]
    previousBackgroundTasks = [...backgroundTasks.value]
    previousConversationId = conversationId
    if (latestTransition) {
      setRecentOutcome(latestTransition)
    }
  }

  async function refreshNow() {
    ensureSSEListeners()
    if (activeRefresh.value) {
      return activeRefresh.value
    }
    const conversationId = normalizeString(currentConversationId.value)
    activeRefresh.value = (async () => {
      loading.value = true
      try {
        const bootstrapSnapshot = await fetchBootstrapTaskSnapshot(conversationId)
        currentTasks.value = bootstrapSnapshot.currentTasks
        backgroundTasks.value = bootstrapSnapshot.backgroundTasks
        hydrated.value = true
        await handleTaskTransitions(conversationId)
      } finally {
        loading.value = false
        activeRefresh.value = null
        syncPolling()
      }
    })()
    return activeRefresh.value
  }

  async function setConversation(conversationId: string) {
    ensureSSEListeners()
    currentConversationId.value = normalizeString(conversationId)
    await refreshNow()
  }

  function resolveTask(taskOrId: string | Pick<UserTaskProjection, 'id' | 'actions'>) {
    if (typeof taskOrId !== 'string') {
      return {
        id: normalizeString(taskOrId.id),
        task: taskOrId,
      }
    }
    const id = normalizeString(taskOrId)
    const task =
      currentTasks.value.find((candidate) => candidate.id === id) ||
      backgroundTasks.value.find((candidate) => candidate.id === id)
    return {
      id,
      task,
    }
  }

  async function performTaskAction(
    taskOrId: string | Pick<UserTaskProjection, 'id' | 'actions'>,
    action: UserTaskActionID,
    payload?: unknown
  ) {
    const actionID = normalizeString(action)
    const resolved = resolveTask(taskOrId)
    const normalizedTaskID = normalizeString(resolved.id)
    if (!normalizedTaskID) return
    const { taskProjectionApi } = await loadTasksApiModule()
    const descriptor =
      findTaskActionDescriptor(resolved.task?.actions, actionID) ||
      (typeof taskOrId !== 'string' ? findTaskActionDescriptor(taskOrId.actions, actionID) : null)
    if (descriptor?.path) {
      await taskProjectionApi.performTaskActionDescriptor(descriptor, payload)
    } else {
      await taskProjectionApi.performTaskAction(normalizedTaskID, actionID, payload)
    }
    await refreshNow()
  }

  async function cancelTask(taskOrId: string | Pick<UserTaskProjection, 'id' | 'actions'>) {
    await performTaskAction(taskOrId, 'cancel')
  }

  async function resumeTask(
    taskOrId: string | Pick<UserTaskProjection, 'id' | 'actions'>,
    payload?: unknown
  ) {
    await performTaskAction(taskOrId, 'resume', payload)
  }

  async function openTask(task: Pick<UserTaskProjection, 'conversation_id'>) {
    const conversationId = normalizeString(task.conversation_id)
    if (!conversationId) return
    const chatStore = useChatStore()
    await router.push({ name: 'Chat', query: { conversationId } })
    await chatStore.selectConversation(conversationId)
  }

  function reset() {
    stopPolling()
    stopSSEListeners()
    clearRecentOutcomeTimer()
    for (const timer of detailHydrationTimers.values()) {
      clearTimeout(timer)
    }
    detailHydrationTimers.clear()
    currentConversationId.value = ''
    currentTasks.value = []
    backgroundTasks.value = []
    recentOutcome.value = null
    loading.value = false
    hydrated.value = false
    previousCurrentTasks = []
    previousBackgroundTasks = []
    previousConversationId = ''
  }

  ensureSSEListeners()

  return {
    currentConversationId,
    currentTasks,
    currentActiveTasks,
    currentTerminalTasks,
    backgroundTasks,
    recentOutcome,
    loading,
    hydrated,
    hasActiveTasks,
    refreshNow,
    setConversation,
    performTaskAction,
    cancelTask,
    resumeTask,
    openTask,
    dismissRecentOutcome,
    reset,
    stopPolling,
  }
})
