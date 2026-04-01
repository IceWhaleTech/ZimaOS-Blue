import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import router from '@/router'
import { i18n } from '@/i18n'
import type {
  UserTaskActionDescriptor,
  UserTaskActionID,
  UserTaskActions,
  UserTaskProjection,
} from '@/api/tasks'
import { useChatStore } from '@/stores/chat'
import { useNotificationStore } from '@/stores/notification'
import { canOpenTaskConversation } from '@/utils/taskProjectionActions'

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

function normalizeBoolean(value: unknown): boolean {
  return Boolean(value)
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
    actions: normalizeTaskActions(snapshot.actions),
    updated_at: normalizeString(snapshot.updated_at) || new Date().toISOString(),
    finished_at: normalizeString(snapshot.finished_at) || undefined,
  }
}

export const useTaskProjectionsStore = defineStore('taskProjections', () => {
  const currentConversationId = ref('')
  const currentTasks = ref<UserTaskProjection[]>([])
  const backgroundTasks = ref<UserTaskProjection[]>([])
  const loading = ref(false)
  const hydrated = ref(false)
  const activeRefresh = ref<Promise<void> | null>(null)
  const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)
  const notifiedTerminalTaskIds = new Set<string>()
  let previousBackgroundTasks: UserTaskProjection[] = []
  let previousConversationId = ''

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

  async function fetchCurrentTasks(conversationId: string) {
    if (!conversationId) {
      currentTasks.value = []
      return
    }
    const { taskProjectionApi } = await loadTasksApiModule()
    const response = await taskProjectionApi.listTasks({
      conversation_id: conversationId,
      scope: 'current',
      limit: 10,
    })
    currentTasks.value = (response.data || [])
      .map((task) => normalizeTaskSnapshot(task))
      .filter((task): task is UserTaskProjection => !!task)
      .sort(compareTasksByUpdatedAt)
  }

  async function fetchBackgroundTasks(conversationId: string) {
    const { taskProjectionApi } = await loadTasksApiModule()
    const response = await taskProjectionApi.listTasks({
      conversation_id: conversationId || undefined,
      scope: 'background',
      limit: 5,
    })
    backgroundTasks.value = (response.data || [])
      .map((task) => normalizeTaskSnapshot(task))
      .filter((task): task is UserTaskProjection => !!task)
      .sort(compareTasksByUpdatedAt)
  }

  function notifyTaskTerminalState(task: UserTaskProjection) {
    if (!task?.id || notifiedTerminalTaskIds.has(task.id)) return
    notifiedTerminalTaskIds.add(task.id)

    const t = i18n.global.t.bind(i18n.global)
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
      notificationStore.error(title, task.error_preview || t('chat.taskFailed', 'Task failed'), {
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

  async function notifyTerminalBackgroundTransitions(conversationId: string) {
    if (previousConversationId !== conversationId) {
      previousConversationId = conversationId
      previousBackgroundTasks = [...backgroundTasks.value]
      return
    }
    const nextIds = new Set(backgroundTasks.value.map((task) => task.id))
    const removed = previousBackgroundTasks.filter((task) => !nextIds.has(task.id))
    previousBackgroundTasks = [...backgroundTasks.value]
    previousConversationId = conversationId
    if (removed.length === 0) return

    const { taskProjectionApi } = await loadTasksApiModule()
    await Promise.all(
      removed.map(async (task) => {
        try {
          const response = await taskProjectionApi.getTask(task.id)
          const detail = normalizeTaskSnapshot(response.data)
          if (!detail || isTaskActive(detail)) return
          notifyTaskTerminalState(detail)
        } catch {
          // Ignore missing task details
        }
      })
    )
  }

  async function refreshNow() {
    if (activeRefresh.value) {
      return activeRefresh.value
    }
    const conversationId = normalizeString(currentConversationId.value)
    activeRefresh.value = (async () => {
      loading.value = true
      try {
        await Promise.all([fetchCurrentTasks(conversationId), fetchBackgroundTasks(conversationId)])
        hydrated.value = true
        await notifyTerminalBackgroundTransitions(conversationId)
      } finally {
        loading.value = false
        activeRefresh.value = null
        syncPolling()
      }
    })()
    return activeRefresh.value
  }

  async function setConversation(conversationId: string) {
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
    currentConversationId.value = ''
    currentTasks.value = []
    backgroundTasks.value = []
    loading.value = false
    hydrated.value = false
    previousBackgroundTasks = []
    previousConversationId = ''
  }

  return {
    currentConversationId,
    currentTasks,
    currentActiveTasks,
    currentTerminalTasks,
    backgroundTasks,
    loading,
    hydrated,
    hasActiveTasks,
    refreshNow,
    setConversation,
    performTaskAction,
    cancelTask,
    resumeTask,
    openTask,
    reset,
    stopPolling,
  }
})
