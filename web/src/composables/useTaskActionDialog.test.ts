import { describe, expect, it, vi } from 'vitest'
import { useTaskActionDialog } from '@/composables/useTaskActionDialog'

function translate(_key: string, fallback: string): string {
  return fallback
}

describe('useTaskActionDialog', () => {
  it('opens a dialog for projected actions that require input and submits on confirm', async () => {
    const submitTaskAction = vi.fn().mockResolvedValue(undefined)
    const dialog = useTaskActionDialog({
      translate,
      submitTaskAction,
    })

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

    await dialog.performProjectedTaskAction(task, 'resume')

    expect(dialog.pendingTaskActionDialog.value?.action.id).toBe('resume')
    expect(submitTaskAction).not.toHaveBeenCalled()

    await dialog.confirmTaskActionDialog({
      decision: 'approve',
      payload: { ticket: 'A-9' },
    })

    expect(submitTaskAction).toHaveBeenCalledWith(task, 'resume', {
      decision: 'approve',
      payload: { ticket: 'A-9' },
    })
    expect(dialog.pendingTaskActionDialog.value).toBeNull()
  })

  it('submits non-input actions immediately without opening a dialog', async () => {
    const submitTaskAction = vi.fn().mockResolvedValue(undefined)
    const dialog = useTaskActionDialog({
      translate,
      submitTaskAction,
    })

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

    await dialog.performProjectedTaskAction(task, 'cancel')

    expect(submitTaskAction).toHaveBeenCalledWith(task, 'cancel', undefined)
    expect(dialog.pendingTaskActionDialog.value).toBeNull()
  })

  it('keeps the dialog open and records an error when confirm submission fails', async () => {
    const submitTaskAction = vi.fn().mockRejectedValue(new Error('network unavailable'))
    const dialog = useTaskActionDialog({
      translate,
      submitTaskAction,
    })

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

    await dialog.performProjectedTaskAction(task, 'resume')
    await dialog.confirmTaskActionDialog({ decision: 'approve' })

    expect(dialog.pendingTaskActionDialog.value?.task.id).toBe('task-workflow')
    expect(dialog.taskActionDialogError.value).toBe('network unavailable')
    expect(dialog.taskActionDialogSubmitting.value).toBe(false)
  })

  it('forwards immediate-action failures to the error callback with a resolved message', async () => {
    const submitTaskAction = vi.fn().mockRejectedValue(new Error('permission denied'))
    const onActionError = vi.fn()
    const dialog = useTaskActionDialog({
      translate,
      submitTaskAction,
      onActionError,
    })

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

    await dialog.performProjectedTaskAction(task, 'cancel')

    expect(onActionError).toHaveBeenCalledWith('cancel', expect.any(Error), 'permission denied')
  })
})
