import type { UserTaskActionID } from '@/api/tasks'
import { useTaskActionDialog } from '@/composables/useTaskActionDialog'
import { useNotificationStore } from '@/stores/notification'
import { useTaskProjectionsStore } from '@/stores/taskProjections'

type Translate = (key: string, fallback: string) => string

export function useTaskProjectionActions(options: { translate: Translate }) {
  const notificationStore = useNotificationStore()
  const taskProjections = useTaskProjectionsStore()

  function reportTaskActionError(action: UserTaskActionID, error: unknown, message: string) {
    console.error(`Failed to perform projected task action ${String(action || '').trim()}:`, error)
    notificationStore.error(
      options.translate('chat.taskActionErrorTitle', 'Task action failed'),
      message
    )
  }

  return useTaskActionDialog({
    translate: options.translate,
    submitTaskAction: (task, action, payload) =>
      taskProjections.performTaskAction(task, action, payload),
    onActionError: reportTaskActionError,
  })
}
