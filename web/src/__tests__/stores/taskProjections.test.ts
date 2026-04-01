import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useTaskProjectionsStore } from '@/stores/taskProjections'
import { taskProjectionApi } from '@/api/tasks'

const mocks = vi.hoisted(() => ({
  routerPush: vi.fn(),
  chatStore: {
    selectConversation: vi.fn(),
  },
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  },
}))

vi.mock('@/router', () => ({
  default: {
    push: (...args: unknown[]) => mocks.routerPush(...args),
  },
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mocks.chatStore,
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/api/tasks', () => ({
  taskProjectionApi: {
    listTasks: vi.fn(),
    getTask: vi.fn(),
    performTaskAction: vi.fn(),
    performTaskActionDescriptor: vi.fn(),
  },
}))

describe('taskProjections store', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mocks.routerPush.mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockResolvedValue(undefined)
    vi.mocked(taskProjectionApi.listTasks).mockResolvedValue({ data: [] } as never)
    vi.mocked(taskProjectionApi.getTask).mockResolvedValue({ data: null } as never)
    vi.mocked(taskProjectionApi.performTaskAction).mockResolvedValue({ data: {} } as never)
    vi.mocked(taskProjectionApi.performTaskActionDescriptor).mockResolvedValue({
      data: {},
    } as never)
  })

  it('hydrates current and background projections through the unified tasks API', async () => {
    vi.mocked(taskProjectionApi.listTasks).mockImplementation(async ({ scope }) => {
      if (scope === 'current') {
        return {
          data: [
            {
              id: 'task-current',
              kind: 'agent_task',
              scope: 'current',
              conversation_id: 'conv-1',
              title: 'Current task',
              status: 'running',
              stage: 'working',
              progress: 45,
              actions: {
                items: [
                  {
                    id: 'cancel',
                    label: 'Cancel',
                    method: 'POST',
                    path: '/tasks/task-current/actions/cancel',
                    variant: 'danger',
                  },
                  {
                    id: 'send_update',
                    label: 'Send update',
                    method: 'POST',
                    path: '/agent/tasks/task-current/message',
                    requires_input: true,
                  },
                ],
              },
              updated_at: '2026-03-20T12:00:00.000Z',
            },
            {
              id: 'task-terminal',
              kind: 'agent_task',
              scope: 'current',
              conversation_id: 'conv-1',
              title: 'Completed task',
              status: 'completed',
              stage: 'completed',
              progress: 100,
              actions: { items: [] },
              updated_at: '2026-03-20T11:59:00.000Z',
            },
          ],
        } as never
      }
      return {
        data: [
          {
            id: 'task-background',
            kind: 'research',
            scope: 'background',
            conversation_id: 'conv-2',
            title: 'Background research',
            status: 'running',
            stage: 'verifying',
            progress: 72,
            actions: {
              items: [
                {
                  id: 'cancel',
                  label: 'Cancel',
                  method: 'POST',
                  path: '/tasks/task-background/actions/cancel',
                  variant: 'danger',
                },
              ],
            },
            updated_at: '2026-03-20T12:01:00.000Z',
          },
        ],
      } as never
    })

    const store = useTaskProjectionsStore()
    await store.setConversation('conv-1')

    expect(taskProjectionApi.listTasks).toHaveBeenCalledTimes(2)
    expect(store.currentTasks).toHaveLength(2)
    expect(store.currentActiveTasks).toHaveLength(1)
    expect(store.currentTerminalTasks).toHaveLength(1)
    expect(store.backgroundTasks).toHaveLength(1)
    expect(store.hasActiveTasks).toBe(true)

    store.stopPolling()
  })

  it('notifies when a background task becomes terminal on refresh', async () => {
    let backgroundActive = true
    vi.mocked(taskProjectionApi.listTasks).mockImplementation(async ({ scope }) => {
      if (scope === 'current') {
        return { data: [] } as never
      }
      return {
        data: backgroundActive
          ? [
              {
                id: 'task-bg-1',
                kind: 'research',
                scope: 'background',
                conversation_id: 'conv-2',
                title: 'Background task',
                status: 'running',
                stage: 'working',
                progress: 60,
                actions: {
                  items: [
                    {
                      id: 'cancel',
                      label: 'Cancel',
                      method: 'POST',
                      path: '/tasks/task-bg-1/actions/cancel',
                      variant: 'danger',
                    },
                  ],
                },
                updated_at: '2026-03-20T12:10:00.000Z',
              },
            ]
          : [],
      } as never
    })
    vi.mocked(taskProjectionApi.getTask).mockResolvedValue({
      data: {
        id: 'task-bg-1',
        kind: 'research',
        scope: 'background',
        conversation_id: 'conv-2',
        title: 'Background task',
        status: 'completed',
        stage: 'completed',
        progress: 100,
        actions: { items: [] },
        updated_at: '2026-03-20T12:12:00.000Z',
      },
    } as never)

    const store = useTaskProjectionsStore()
    await store.setConversation('conv-1')

    backgroundActive = false
    await store.refreshNow()

    expect(taskProjectionApi.getTask).toHaveBeenCalledWith('task-bg-1')
    expect(mocks.notificationStore.success).toHaveBeenCalledTimes(1)

    store.stopPolling()
  })

  it('opens the owning conversation for a projected task', async () => {
    const store = useTaskProjectionsStore()

    await store.openTask({ conversation_id: 'conv-77' })

    expect(mocks.routerPush).toHaveBeenCalledWith({
      name: 'Chat',
      query: { conversationId: 'conv-77' },
    })
    expect(mocks.chatStore.selectConversation).toHaveBeenCalledWith('conv-77')

    store.stopPolling()
  })

  it('performs task actions through descriptors when available', async () => {
    vi.mocked(taskProjectionApi.listTasks).mockImplementation(async ({ scope }) => {
      if (scope === 'current') {
        return {
          data: [
            {
              id: 'task-workflow',
              kind: 'workflow',
              scope: 'current',
              conversation_id: 'conv-1',
              title: 'Workflow gate',
              status: 'waiting_user',
              stage: 'waiting_user',
              progress: 75,
              actions: {
                items: [
                  {
                    id: 'resume',
                    label: 'Resume workflow',
                    method: 'POST',
                    path: '/tasks/task-workflow/actions/resume',
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
              updated_at: '2026-03-20T12:00:00.000Z',
            },
          ],
        } as never
      }
      return { data: [] } as never
    })

    const store = useTaskProjectionsStore()
    await store.setConversation('conv-1')
    await store.resumeTask('task-workflow')

    expect(taskProjectionApi.performTaskActionDescriptor).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'resume',
        label: 'Resume workflow',
        method: 'POST',
        path: '/tasks/task-workflow/actions/resume',
        requires_input: true,
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
      }),
      undefined
    )

    store.stopPolling()
  })

  it('retains legacy input hints on normalized descriptors when schema fields are absent', async () => {
    vi.mocked(taskProjectionApi.listTasks).mockImplementation(async ({ scope }) => {
      if (scope === 'current') {
        return {
          data: [
            {
              id: 'task-legacy-workflow',
              kind: 'workflow',
              scope: 'current',
              conversation_id: 'conv-1',
              title: 'Workflow gate',
              status: 'waiting_user',
              stage: 'waiting_user',
              progress: 75,
              actions: {
                items: [
                  {
                    id: 'resume',
                    label: 'Resume workflow',
                    method: 'POST',
                    path: '/tasks/task-legacy-workflow/actions/resume',
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
              updated_at: '2026-03-20T12:00:00.000Z',
            },
          ],
        } as never
      }
      return { data: [] } as never
    })

    const store = useTaskProjectionsStore()
    await store.setConversation('conv-1')

    const input = store.currentTasks[0]?.actions.items?.[0]?.input as
      | Record<string, unknown>
      | undefined
    expect(input?.title).toBe('Provide clarification')
    expect(input?.submit_label).toBe('Send response')
    expect(input?.mode).toBe('payload')
    expect(input?.payload_label).toBe('Response')
    expect(input?.payload_required).toBe(true)
    expect(input?.payload_format).toBe('text')
    expect(input?.payload_text_key).toBe('response')
    expect(input?.payload_placeholder).toBe('Provide the missing detail')

    store.stopPolling()
  })
})
