import { ref } from 'vue'
import type { UserTaskActionID, UserTaskProjection } from '@/api/tasks'
import {
  findProjectedTaskAction,
  type ProjectedUserTaskAction,
} from '@/utils/taskProjectionActions'

type Translate = (key: string, fallback: string) => string

export type TaskActionDialogPayload = unknown

export type TaskActionTarget = string | Pick<UserTaskProjection, 'id' | 'actions'>

export type PendingTaskActionDialog = {
  task: Pick<UserTaskProjection, 'id' | 'actions'>
  action: ProjectedUserTaskAction
}

export function useTaskActionDialog(options: {
  translate: Translate
  submitTaskAction: (
    task: TaskActionTarget,
    action: UserTaskActionID,
    payload?: unknown
  ) => Promise<void>
  onActionError?: (action: UserTaskActionID, error: unknown, message: string) => void
}) {
  const pendingTaskActionDialog = ref<PendingTaskActionDialog | null>(null)
  const taskActionDialogError = ref('')
  const taskActionDialogSubmitting = ref(false)

  function resetTaskActionDialogRequestState() {
    taskActionDialogError.value = ''
    taskActionDialogSubmitting.value = false
  }

  function openTaskActionDialog(
    task: Pick<UserTaskProjection, 'id' | 'actions'>,
    action: ProjectedUserTaskAction
  ) {
    pendingTaskActionDialog.value = { task, action }
    resetTaskActionDialogRequestState()
  }

  function closeTaskActionDialog(force = false) {
    if (taskActionDialogSubmitting.value && !force) return
    pendingTaskActionDialog.value = null
    resetTaskActionDialogRequestState()
  }

  function resolveTaskActionErrorMessage(error: unknown): string {
    const message = error instanceof Error ? error.message.trim() : ''
    return (
      message ||
      options.translate(
        'chat.taskActionDialog.submitError',
        'Could not perform this task action. Please try again.'
      )
    )
  }

  async function performProjectedTaskAction(
    task: TaskActionTarget,
    action: UserTaskActionID,
    payload?: unknown
  ) {
    if (payload === undefined && typeof task !== 'string') {
      const descriptor = findProjectedTaskAction(task, action, options.translate)
      if (descriptor?.requires_input) {
        openTaskActionDialog(task, descriptor)
        return
      }
    }

    try {
      await options.submitTaskAction(task, action, payload)
    } catch (error) {
      options.onActionError?.(action, error, resolveTaskActionErrorMessage(error))
    }
  }

  async function confirmTaskActionDialog(payload?: TaskActionDialogPayload) {
    const pending = pendingTaskActionDialog.value
    if (!pending || taskActionDialogSubmitting.value) return

    taskActionDialogError.value = ''
    taskActionDialogSubmitting.value = true

    try {
      await options.submitTaskAction(pending.task, pending.action.id, payload)
      closeTaskActionDialog(true)
    } catch (error) {
      taskActionDialogError.value = resolveTaskActionErrorMessage(error)
      taskActionDialogSubmitting.value = false
    }
  }

  return {
    pendingTaskActionDialog,
    taskActionDialogError,
    taskActionDialogSubmitting,
    openTaskActionDialog,
    closeTaskActionDialog,
    performProjectedTaskAction,
    confirmTaskActionDialog,
  }
}
