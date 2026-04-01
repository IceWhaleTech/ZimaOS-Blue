import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useTaskProjectionActions } from '@/composables/useTaskProjectionActions'

const mocks = vi.hoisted(() => ({
  notificationStore: {
    error: vi.fn(),
  },
  taskProjectionsStore: {
    performTaskAction: vi.fn(),
  },
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/stores/taskProjections', () => ({
  useTaskProjectionsStore: () => mocks.taskProjectionsStore,
}))

function translate(_key: string, fallback: string): string {
  return fallback
}

describe('useTaskProjectionActions', () => {
  beforeEach(() => {
    mocks.notificationStore.error.mockReset()
    mocks.taskProjectionsStore.performTaskAction.mockReset()
    vi.restoreAllMocks()
  })

  it('opens a dialog for projected actions that require input and submits through the store', async () => {
    mocks.taskProjectionsStore.performTaskAction.mockResolvedValue(undefined)

    const taskActions = useTaskProjectionActions({ translate })
    const task = {
      id: 'task-workflow',
      actions: {
        items: [
          {
            id: 'resume',
            label: 'Resume workflow',
            method: 'POST',
            path: '/tasks/task-workflow/actions/resume',
            requires_input: true,
          },
        ],
      },
    }

    await taskActions.performProjectedTaskAction(task, 'resume')

    expect(taskActions.pendingTaskActionDialog.value?.action.id).toBe('resume')
    expect(mocks.taskProjectionsStore.performTaskAction).not.toHaveBeenCalled()

    await taskActions.confirmTaskActionDialog({ decision: 'approve' })

    expect(mocks.taskProjectionsStore.performTaskAction).toHaveBeenCalledWith(task, 'resume', {
      decision: 'approve',
    })
    expect(taskActions.pendingTaskActionDialog.value).toBeNull()
  })

  it('reports direct action failures through notifications', async () => {
    const failure = new Error('permission denied')
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    mocks.taskProjectionsStore.performTaskAction.mockRejectedValueOnce(failure)

    const taskActions = useTaskProjectionActions({ translate })
    const task = {
      id: 'task-agent',
      actions: {
        items: [
          {
            id: 'cancel',
            label: 'Cancel',
            method: 'POST',
            path: '/tasks/task-agent/actions/cancel',
            requires_input: false,
          },
        ],
      },
    }

    await taskActions.performProjectedTaskAction(task, 'cancel')

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      'Failed to perform projected task action cancel:',
      failure
    )
    expect(mocks.notificationStore.error).toHaveBeenCalledWith(
      'Task action failed',
      'permission denied'
    )
  })
})
