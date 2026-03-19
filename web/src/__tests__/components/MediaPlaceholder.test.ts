import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { i18n } from '@/i18n'
import MediaPlaceholder from '@/components/MediaPlaceholder.vue'

const getTaskMock = vi.fn()
const retryTaskMock = vi.fn()
const onSSEEventMock = vi.fn()
const offSSEEventMock = vi.fn()
const pushMock = vi.fn()
const notificationStoreMock = {
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
}
let sseHandler: ((data: any) => void) | null = null

vi.mock('@/api/media', () => ({
  getTask: (...args: any[]) => getTaskMock(...args),
  retryTask: (...args: any[]) => retryTaskMock(...args),
}))

vi.mock('@/composables/useEventStream', () => ({
  onSSEEvent: (...args: any[]) => {
    sseHandler = args[1]
    return onSSEEventMock(...args)
  },
  offSSEEvent: (...args: any[]) => offSSEEventMock(...args),
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => notificationStoreMock,
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
}))

describe('MediaPlaceholder', () => {
  beforeEach(() => {
    getTaskMock.mockReset()
    retryTaskMock.mockReset()
    onSSEEventMock.mockReset()
    offSSEEventMock.mockReset()
    pushMock.mockReset()
    notificationStoreMock.success.mockReset()
    notificationStoreMock.error.mockReset()
    notificationStoreMock.info.mockReset()
    sseHandler = null
  })

  afterEach(() => {
    vi.clearAllTimers()
  })

  it('shows retry for cancelled tasks and retries the active task', async () => {
    getTaskMock.mockResolvedValueOnce({
      id: 'task-cancelled',
      status: 'cancelled',
      error: 'cancelled by user',
      progress: 0.3,
      type: 'image',
      created_at: new Date().toISOString(),
    })
    retryTaskMock.mockResolvedValueOnce({ task_id: 'task-retried' })
    getTaskMock.mockResolvedValueOnce({
      id: 'task-retried',
      status: 'processing',
      progress: 0.1,
      type: 'image',
      created_at: new Date().toISOString(),
    })

    const wrapper = mount(MediaPlaceholder, {
      props: { taskId: 'task-cancelled' },
      global: {
        plugins: [i18n],
        stubs: { Teleport: true },
      },
    })

    await flushPromises()

    const retryButton = wrapper.find('button.mp-retry--cancelled')
    expect(retryButton.exists()).toBe(true)
    expect(wrapper.text()).toContain('chat.taskCancelled')

    await retryButton.trigger('click')
    await flushPromises()

    expect(retryTaskMock).toHaveBeenCalledWith('task-cancelled')
    wrapper.unmount()
  })

  it('emits an info toast when a live task is cancelled', async () => {
    getTaskMock.mockResolvedValueOnce({
      id: 'task-live',
      status: 'processing',
      progress: 0.2,
      type: 'image',
      created_at: new Date().toISOString(),
    })

    const wrapper = mount(MediaPlaceholder, {
      props: { taskId: 'task-live' },
      global: {
        plugins: [i18n],
        stubs: { Teleport: true },
      },
    })

    await flushPromises()
    expect(typeof sseHandler).toBe('function')

    sseHandler?.({
      id: 'task-live',
      status: 'cancelled',
      error: 'cancelled by user',
      progress: 0.2,
    })
    await flushPromises()

    expect(notificationStoreMock.info).toHaveBeenCalledWith(
      'media.cancelled',
      'chat.taskCancelled',
      {
        titleKey: 'media.cancelled',
      }
    )
    expect(wrapper.find('button.mp-retry--cancelled').exists()).toBe(true)
    wrapper.unmount()
  })
})
