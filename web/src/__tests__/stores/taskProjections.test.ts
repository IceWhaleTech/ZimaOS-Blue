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
    cancelTask: vi.fn(),
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
    vi.mocked(taskProjectionApi.cancelTask).mockResolvedValue({ data: {} } as never)
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
              actions: { can_cancel: true, can_open_chat: false, can_send_update: true },
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
              actions: { can_cancel: false, can_open_chat: false, can_send_update: false },
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
            actions: { can_cancel: true, can_open_chat: true, can_send_update: false },
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
                actions: { can_cancel: true, can_open_chat: true, can_send_update: false },
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
        actions: { can_cancel: false, can_open_chat: true, can_send_update: false },
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
})
