import { describe, expect, it } from 'vitest'

import { findProjectedTaskAction, projectTaskControlActions } from '@/utils/taskProjectionActions'

function translate(_key: string, fallback: string): string {
  return fallback
}

describe('taskProjectionActions', () => {
  it('keeps descriptor-driven actions in the control button row', () => {
    const task = {
      id: 'task-1',
      actions: {
        items: [
          {
            id: 'cancel',
            label: 'Cancel',
            method: 'POST',
            path: '/tasks/task-1/actions/cancel',
            variant: 'danger',
          },
          {
            id: 'send_update',
            label: 'Send update',
            method: 'POST',
            path: '/agent/tasks/task-1/message',
            requires_input: true,
          },
        ],
      },
    }

    expect(projectTaskControlActions(task, translate)).toEqual([
      expect.objectContaining({
        id: 'send_update',
        path: '/agent/tasks/task-1/message',
        requires_input: true,
      }),
      expect.objectContaining({
        id: 'cancel',
        variant: 'danger',
      }),
    ])
  })

  it('preserves backend send-update descriptors in the action list', () => {
    const task = {
      id: 'task-2',
      actions: {
        items: [
          {
            id: 'send_update',
            label: 'Reply to task',
            method: 'POST',
            path: '/tasks/task-2/actions/send_update',
            requires_input: true,
          },
        ],
      },
    }

    expect(projectTaskControlActions(task, translate)).toEqual([
      expect.objectContaining({
        id: 'send_update',
        label: 'Reply to task',
        path: '/tasks/task-2/actions/send_update',
        requires_input: true,
      }),
    ])
  })

  it('orders workflow resume ahead of send_update and cancel based on action rank', () => {
    const task = {
      id: 'task-3',
      actions: {
        items: [
          {
            id: 'resume',
            label: 'Resume workflow',
            method: 'POST',
            path: '/tasks/task-3/actions/resume',
            variant: 'primary',
            requires_input: true,
          },
          {
            id: 'send_update',
            label: 'Reply to task',
            method: 'POST',
            path: '/agent/tasks/task-3/message',
            requires_input: true,
          },
        ],
      },
    }

    expect(projectTaskControlActions(task, translate)).toEqual([
      expect.objectContaining({
        id: 'resume',
        path: '/tasks/task-3/actions/resume',
        requires_input: true,
        variant: 'primary',
      }),
      expect.objectContaining({
        id: 'send_update',
        path: '/agent/tasks/task-3/message',
        requires_input: true,
      }),
    ])
    expect(findProjectedTaskAction(task, 'resume', translate)).toEqual(
      expect.objectContaining({
        id: 'resume',
        label: 'Resume workflow',
        requires_input: true,
      })
    )
  })

  it('preserves structured input contracts on projected actions', () => {
    const task = {
      id: 'task-4',
      actions: {
        items: [
          {
            id: 'resume',
            label: 'Resume workflow',
            method: 'POST',
            path: '/tasks/task-4/actions/resume',
            variant: 'primary',
            requires_input: true,
            input: {
              fields: [
                {
                  key: 'response',
                  label: 'Response',
                  kind: 'textarea',
                  target: 'payload',
                  payload_key: 'response',
                  required: true,
                  placeholder: 'Provide the missing detail',
                },
              ],
              title: 'Provide clarification',
              submit_label: 'Send response',
            },
          },
        ],
      },
    }

    expect(findProjectedTaskAction(task, 'resume', translate)).toEqual(
      expect.objectContaining({
        id: 'resume',
        input: expect.objectContaining({
          fields: [
            expect.objectContaining({
              key: 'response',
              label: 'Response',
              kind: 'textarea',
              target: 'payload',
              payload_key: 'response',
              required: true,
              placeholder: 'Provide the missing detail',
            }),
          ],
          title: 'Provide clarification',
          submit_label: 'Send response',
        }),
      })
    )
  })

  it('preserves legacy input hints for fallback dialogs when fields are absent', () => {
    const task = {
      id: 'task-5',
      actions: {
        items: [
          {
            id: 'resume',
            label: 'Resume workflow',
            method: 'POST',
            path: '/tasks/task-5/actions/resume',
            requires_input: true,
            input: {
              title: 'Provide clarification',
              submit_label: 'Send response',
              mode: 'payload',
              payload_label: 'Response',
              payload_required: true,
              payload_format: 'text',
              payload_text_key: 'response',
              payload_placeholder: 'Provide the missing detail',
            },
          },
        ],
      },
    }

    const projected = findProjectedTaskAction(task, 'resume', translate)
    expect(projected).toEqual(
      expect.objectContaining({
        id: 'resume',
        input: expect.objectContaining({
          title: 'Provide clarification',
          submit_label: 'Send response',
        }),
      })
    )

    const legacyInput = projected?.input as Record<string, unknown> | undefined
    expect(legacyInput?.mode).toBe('payload')
    expect(legacyInput?.payload_label).toBe('Response')
    expect(legacyInput?.payload_required).toBe(true)
    expect(legacyInput?.payload_format).toBe('text')
    expect(legacyInput?.payload_text_key).toBe('response')
    expect(legacyInput?.payload_placeholder).toBe('Provide the missing detail')
  })
})
