/**
 * Global media task tracker — singleton that survives page navigation.
 * Listens to the shared SSE connection (media_task_update events) with polling fallback.
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { MediaTask } from '@/api/media'
import { getTask } from '@/api/media'
import { onSSEEvent, offSSEEvent } from '@/composables/useEventStream'

interface TrackedTask {
  task: MediaTask
  chatMessageId: string // assistant message ID in chat store
  conversationId: string
  onUpdate: (task: MediaTask) => void
  pollTimer: ReturnType<typeof setTimeout> | null
  sseActive: boolean
}

export const useMediaTaskTracker = defineStore('mediaTaskTracker', () => {
  const tasks = ref<Map<string, TrackedTask>>(new Map())
  let listening = false

  /** All active (non-terminal) tasks */
  const activeTasks = computed(() => {
    const result: TrackedTask[] = []
    tasks.value.forEach((t) => {
      if (!isTerminal(t.task.status)) result.push(t)
    })
    return result
  })

  function isTerminal(status: string) {
    return status === 'succeeded' || status === 'failed' || status === 'cancelled'
  }

  // Global SSE listener — shared across all tracked tasks
  function handleSSEEvent(data: any) {
    const tracked = tasks.value.get(data.id)
    if (!tracked) return

    tracked.sseActive = true
    // Stop polling — SSE is delivering updates
    if (tracked.pollTimer) {
      clearTimeout(tracked.pollTimer)
      tracked.pollTimer = null
    }

    tracked.task = {
      ...tracked.task,
      status: data.status,
      progress: data.progress || tracked.task.progress,
    }
    if (data.error) tracked.task.error = data.error
    if (data.response) tracked.task.response = data.response
    tasks.value = new Map(tasks.value)
    tracked.onUpdate(tracked.task)

    if (isTerminal(data.status)) {
      tasks.value.delete(data.id)
      tasks.value = new Map(tasks.value)
      maybeStopListening()
    }
  }

  function ensureListening() {
    if (listening) return
    listening = true
    onSSEEvent('media_task_update', handleSSEEvent)
  }

  function maybeStopListening() {
    if (activeTasks.value.length > 0) return
    listening = false
    offSSEEvent('media_task_update', handleSSEEvent)
  }

  /**
   * Start tracking a media task via global SSE + polling fallback.
   */
  function track(
    taskId: string,
    initialTask: MediaTask,
    chatMessageId: string,
    conversationId: string,
    onUpdate: (task: MediaTask) => void,
  ) {
    stop(taskId)

    const tracked: TrackedTask = {
      task: initialTask,
      chatMessageId,
      conversationId,
      onUpdate,
      pollTimer: null,
      sseActive: false,
    }
    tasks.value.set(taskId, tracked)
    tasks.value = new Map(tasks.value)

    ensureListening()
    startPolling(taskId)
  }

  function startPolling(taskId: string) {
    const tracked = tasks.value.get(taskId)
    if (!tracked || isTerminal(tracked.task.status)) return

    const poll = async () => {
      const t = tasks.value.get(taskId)
      if (!t || isTerminal(t.task.status) || t.sseActive) return

      try {
        const fresh = await getTask(taskId)
        t.task = fresh
        tasks.value = new Map(tasks.value)
        t.onUpdate(fresh)

        if (isTerminal(fresh.status)) {
          tasks.value.delete(taskId)
          tasks.value = new Map(tasks.value)
          maybeStopListening()
        } else if (!t.sseActive) {
          t.pollTimer = setTimeout(poll, 3000)
        }
      } catch {
        if (!t.sseActive) t.pollTimer = setTimeout(poll, 5000)
      }
    }

    tracked.pollTimer = setTimeout(poll, 1000)
  }

  /** Stop tracking a task. */
  function stop(taskId: string) {
    const tracked = tasks.value.get(taskId)
    if (!tracked) return
    if (tracked.pollTimer) clearTimeout(tracked.pollTimer)
    tasks.value.delete(taskId)
    tasks.value = new Map(tasks.value)
    maybeStopListening()
  }

  /** Get the latest task state for a given chat message ID. */
  function getByMessageId(chatMessageId: string): MediaTask | null {
    for (const t of tasks.value.values()) {
      if (t.chatMessageId === chatMessageId) return t.task
    }
    return null
  }

  /** Get all tracked tasks for a conversation. */
  function getByConversation(conversationId: string): TrackedTask[] {
    const result: TrackedTask[] = []
    tasks.value.forEach((t) => {
      if (t.conversationId === conversationId) result.push(t)
    })
    return result
  }

  /** Resume tracking for tasks that were active before navigation. */
  function resumeForConversation(
    conversationId: string,
    onUpdate: (chatMessageId: string, task: MediaTask) => void,
  ) {
    tasks.value.forEach((tracked, taskId) => {
      if (tracked.conversationId !== conversationId) return
      if (isTerminal(tracked.task.status)) return
      if (tracked.sseActive || tracked.pollTimer) return

      tracked.onUpdate = (task) => onUpdate(tracked.chatMessageId, task)
      ensureListening()
      startPolling(taskId)
    })
  }

  return {
    tasks,
    activeTasks,
    track,
    stop,
    getByMessageId,
    getByConversation,
    resumeForConversation,
  }
})
