import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { i18n } from '@/i18n'
import mediaFallbackOverrides from '@/i18n/media-fallback-overrides'
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
    ;(i18n.global as { locale: { value: string } }).locale.value = 'en-US'
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

  it('surfaces nanoslides fallback metadata on completed slide renders', async () => {
    getTaskMock.mockResolvedValueOnce({
      id: 'task-slide',
      status: 'succeeded',
      progress: 1,
      type: 'image',
      created_at: new Date().toISOString(),
      response: {
        data: [{ url: '/api/media/generated/images/slide.png' }],
      },
      fallback_info: {
        used: true,
        strategy: 'web_canvas',
        display_name: 'Web Search + Canvas',
        disclosure: 'Rendered with fallback slide generation.',
        style_preset: 'nano_slides',
        template_id: 'text_only',
      },
    })

    const wrapper = mount(MediaPlaceholder, {
      props: { taskId: 'task-slide' },
      global: {
        plugins: [i18n],
        stubs: { Teleport: true },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('nanoslides')
    expect(wrapper.findAll('.mp-fallback-chip').length).toBeGreaterThanOrEqual(2)
    wrapper.unmount()
  })

  it('renders disclosed fallback sources with license metadata', async () => {
    getTaskMock.mockResolvedValueOnce({
      id: 'task-source',
      status: 'succeeded',
      progress: 1,
      type: 'image',
      created_at: new Date().toISOString(),
      response: {
        data: [{ url: '/api/media/generated/images/source.png' }],
      },
      fallback_info: {
        used: true,
        strategy: 'web_canvas',
        display_name: 'Fallback Preview',
        disclosure: 'Source and license details are shown below.',
        sources: [
          {
            provider: 'Example Archive',
            title: 'Toy poodle in snow',
            page_url: 'https://example.com/teddy-in-snow',
            license: 'CC BY-SA 4.0',
            creator: 'Alice Example',
            verified_license: true,
          },
        ],
      },
    })

    const wrapper = mount(MediaPlaceholder, {
      props: { taskId: 'task-source' },
      global: {
        plugins: [i18n],
        stubs: { Teleport: true },
      },
    })

    await flushPromises()

    expect(wrapper.find('.mp-fallback-source').exists()).toBe(true)
    expect(wrapper.text()).toContain('Example Archive')
    expect(wrapper.text()).toContain('CC BY-SA 4.0')
    expect(wrapper.text()).toContain('Alice Example')
    expect(wrapper.text()).toContain(i18n.global.t('media.fallbackSourceLicenseVerified'))
    expect(wrapper.find('.mp-fallback-chip--verified').exists()).toBe(true)
    wrapper.unmount()
  })

  it('renders an explicit unverified license status for search-engine sources', async () => {
    getTaskMock.mockResolvedValueOnce({
      id: 'task-source-unverified',
      status: 'succeeded',
      progress: 1,
      type: 'image',
      created_at: new Date().toISOString(),
      response: {
        data: [{ url: '/api/media/generated/images/source-unverified.png' }],
      },
      fallback_info: {
        used: true,
        strategy: 'web_canvas',
        display_name: 'Fallback Preview',
        disclosure: 'Source and license details are shown below.',
        sources: [
          {
            provider: 'Bing Images',
            title: 'Toy poodle in snow',
            page_url: 'https://example.com/teddy-in-snow',
            note: 'Search engine image result; license not verified',
            verified_license: false,
          },
        ],
      },
    })

    const wrapper = mount(MediaPlaceholder, {
      props: { taskId: 'task-source-unverified' },
      global: {
        plugins: [i18n],
        stubs: { Teleport: true },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain(i18n.global.t('media.fallbackSourceLicenseUnverified'))
    expect(wrapper.find('.mp-fallback-chip--unverified').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Search engine image result; license not verified')
    wrapper.unmount()
  })

  it('localizes fallback template and link labels using the active UI locale', async () => {
    i18n.global.setLocaleMessage('zh-CN', {
      media: {
        pending: '等待中',
        processing: '处理中',
        succeeded: '已完成',
        processingHint: '仍在处理中，请稍候',
        ...(mediaFallbackOverrides as Record<string, any>)['zh-CN']?.media,
      },
    } as never)
    ;(i18n.global as { locale: { value: string } }).locale.value = 'zh-CN'

    getTaskMock.mockResolvedValueOnce({
      id: 'task-zh-template-links',
      status: 'succeeded',
      progress: 1,
      type: 'image',
      created_at: new Date().toISOString(),
      response: {
        data: [{ url: '/api/media/generated/images/source-zh-template.png' }],
      },
      fallback_info: {
        used: true,
        strategy: 'web_canvas',
        display_name: 'Fallback Preview',
        disclosure:
          'No configured media API key was available, so this result used a built-in reference or placeholder preview path instead of a newly generated AI image. Source and license details are shown below when available; search-engine fallbacks are marked as unverified.',
        template_id: 'text_only',
        space_url: 'https://example.com/space',
        source_urls: ['https://example.com/source-1'],
      },
    })

    const wrapper = mount(MediaPlaceholder, {
      props: { taskId: 'task-zh-template-links' },
      global: {
        plugins: [i18n],
        stubs: { Teleport: true },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('文字摘要')
    expect(wrapper.text()).toContain('空间')
    expect(wrapper.text()).toContain('来源 1')
    wrapper.unmount()
  })

  it('localizes fallback disclosure copy using the active UI locale', async () => {
    i18n.global.setLocaleMessage('zh-CN', {
      media: {
        pending: '等待中',
        processing: '处理中',
        processingHint: '仍在处理中，请稍候',
        ...(mediaFallbackOverrides as Record<string, any>)['zh-CN']?.media,
      },
    } as never)
    ;(i18n.global as { locale: { value: string } }).locale.value = 'zh-CN'

    getTaskMock.mockResolvedValueOnce({
      id: 'task-zh-fallback',
      status: 'processing',
      progress: 0.2,
      type: 'image',
      provider: 'fallback',
      model: 'fallback-web-canvas-t2i',
      created_at: new Date().toISOString(),
      fallback_info: {
        used: true,
        strategy: 'web_canvas',
        display_name: 'Fallback Preview',
        disclosure:
          'No configured media API key was available, so this result used a built-in reference or placeholder preview path instead of a newly generated AI image. Source and license details are shown below when available; search-engine fallbacks are marked as unverified.',
      },
    })

    const wrapper = mount(MediaPlaceholder, {
      props: { taskId: 'task-zh-fallback' },
      global: {
        plugins: [i18n],
        stubs: { Teleport: true },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('未检测到可用媒体生成 API Key')
    expect(wrapper.text()).not.toContain('No configured media API key was available')
    wrapper.unmount()
  })
})
