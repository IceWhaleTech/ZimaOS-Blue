import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import router from '@/router'
import { i18n } from '@/i18n'
import type { UserTaskProjection } from '@/api/tasks'
import { useChatStore } from '@/stores/chat'
import { useNotificationStore } from '@/stores/notification'

type TasksApiModule = typeof import('@/api/tasks')
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
    actions: snapshot.actions || {
      can_cancel: false,
      can_open_chat: false,
      can_send_update: false,
    },
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
    const action =
      task.conversation_id && task.actions?.can_open_chat
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

  async function cancelTask(taskId: string) {
    const normalizedTaskID = normalizeString(taskId)
    if (!normalizedTaskID) return
    const { taskProjectionApi } = await loadTasksApiModule()
    await taskProjectionApi.cancelTask(normalizedTaskID)
    await refreshNow()
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
    cancelTask,
    openTask,
    reset,
    stopPolling,
  }
})
