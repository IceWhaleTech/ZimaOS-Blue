import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import BrowserMonitorWidget from '@/components/BrowserMonitorWidget.vue'
import { __resetBrowserMonitorStateForTests, useBrowserMonitor } from '@/composables/useBrowserMonitor'

const { pushMock, listTasksMock, getSessionsMock, takeScreenshotMock } = vi.hoisted(() => ({
  pushMock: vi.fn(),
  listTasksMock: vi.fn(),
  getSessionsMock: vi.fn(),
  takeScreenshotMock: vi.fn(),
}))

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

function setViewport(width: number, height: number) {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: width,
  })
  Object.defineProperty(window, 'innerHeight', {
    configurable: true,
    writable: true,
    value: height,
  })
}

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: { conversationId: 'conv-1' },
    fullPath: '/chat?conversationId=conv-1',
  }),
  useRouter: () => ({
    push: pushMock,
  }),
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => ({
    currentConversationId: 'conv-1',
  }),
}))

vi.mock('@/api/tasks', () => ({
  taskProjectionApi: {
    listTasks: listTasksMock,
  },
}))

vi.mock('@/api/browser', () => ({
  getSessions: getSessionsMock,
  takeScreenshot: takeScreenshotMock,
}))

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'zh-CN',
    fallbackLocale: 'en-US',
    messages: {
      'zh-CN': {
        browserMonitor: {
          refresh: '刷新',
          collapse: '收起',
          expand: '展开',
          title: '实时监控',
          subtitle: '在这里跟踪最新的任务和标签页状态。',
          tasksHeading: '当前任务动态',
          taskViewSmart: '智能',
          taskViewCurrentShort: '当前',
          taskViewAllShort: '全部',
          taskViewCurrent: '当前对话',
          taskViewAll: '全部任务',
          tasksCount: '项',
          compactTask: '主要任务',
          compactTaskIdle: '暂无启用中的任务',
          compactTab: '启用中的分页',
          compactTabIdle: '等待浏览器活动',
          compactTabHint: '展开后会恢复预览',
          launcherActive: '执行监控',
          launcherIdle: '实时监控',
          launcherMeta: '点击打开',
          tasksShort: '任务',
          tabsShort: '标签页',
          previewHeading: '浏览器预览',
          previewIdleTitle: '等待浏览器活动',
          previewIdleUrl: '打开或复用一个浏览器标签页以开始实时画面。',
          previewPending: '等待中',
          previewEmptyTitle: '暂无预览画面',
          previewEmptyBody: '一旦浏览器标签页处于活跃状态，监控就会显示截图。',
          previewLoading: '正在捕获当前视口...',
          previewAlt: '浏览器预览',
          untitledTab: '未命名标签页',
          updated: '已更新',
          noCurrentConversation: '打开一个聊天会话后，可只跟踪当前这次执行。',
          noCurrentTasks: '当前会话里还没有最近任务。',
          noTasks: '还没有最近任务。',
          justNow: '刚刚',
          capabilityHeading: '能力优先级',
          capabilityTask: '任务焦点',
          capabilityTaskIdle: '等待下一次执行',
          capabilityBrowser: '浏览器连续性',
          capabilityBrowserIdle: '暂无启用中的浏览器分页',
          capabilityBlocker: '执行阻塞',
          capabilityPreview: '预览新鲜度',
          capabilityPreviewIdle: '等待最新画面',
          refreshing: '刷新中',
          overviewError: '刷新失败',
          screenshotError: '预览不可用',
          stageRunning: '运行中',
          eyebrow: '浏览器执行',
          showTooltip: '显示执行监控',
          hideTooltip: '隐藏执行监控',
          buttonLabel: '监控',
          blockerFallback: '等待输入',
        },
        chat: {
          taskStagePlanning: '规划中',
          taskStageWorking: '执行中',
          taskStageVerifying: '验证中',
          taskStageWaiting: '等待中',
          taskStageCompleted: '已完成',
          taskStageFailed: '已失败',
          taskStageCancelled: '已取消',
          taskDefaultResearchTitle: '研究任务',
          taskDefaultAgentTitle: '智能体任务',
          taskRuntimeExecute: '执行中',
          deepResearchStagePlanning: '规划中',
          deepResearchActionVerificationCompleted: '验证已完成',
        },
        common: {
          close: '关闭',
        },
      },
      'en-US': {
        browserMonitor: {
          refresh: 'Refresh',
        },
      },
    },
  })
}

describe('BrowserMonitorWidget', () => {
  beforeEach(() => {
    pushMock.mockReset()
    listTasksMock.mockReset()
    getSessionsMock.mockReset()
    takeScreenshotMock.mockReset()
    localStorage.clear()
    setViewport(1440, 900)
    __resetBrowserMonitorStateForTests()
    const monitor = useBrowserMonitor()
    monitor.setOpen(true)
    monitor.setCollapsed(false)

    listTasksMock.mockResolvedValue({
      data: [
        {
          id: 'task-1',
          kind: 'research',
          scope: 'current',
          conversation_id: 'conv-1',
          title: 'Research task',
          subtitle: 'planning • verification_completed • Need official source',
          status: 'running',
          stage: 'planning',
          progress: 42,
          actions: { can_cancel: false, can_open_chat: true, can_send_update: false },
          updated_at: '2026-03-23T00:00:00.000Z',
        },
        {
          id: 'task-2',
          kind: 'agent_task',
          scope: 'current',
          conversation_id: 'conv-1',
          title: 'Agent task',
          subtitle: 'execute',
          status: 'running',
          stage: 'working',
          progress: 67,
          actions: { can_cancel: false, can_open_chat: true, can_send_update: false },
          updated_at: '2026-03-22T23:59:00.000Z',
        },
      ],
    })
    getSessionsMock.mockResolvedValue([])
    takeScreenshotMock.mockResolvedValue({ screenshot: '', history: [], error: '' })
  })

  it('renders localized task details and exposes the resizable task list structure', async () => {
    const wrapper = mount(BrowserMonitorWidget, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('当前任务动态')
    expect(wrapper.text()).toContain('研究任务')
    expect(wrapper.text()).toContain('规划中 · 验证已完成 · Need official source')
    expect(wrapper.text()).toContain('智能体任务')
    expect(wrapper.text()).toContain('执行中')
    expect(wrapper.find('.browser-monitor__task-list').exists()).toBe(true)
    expect(wrapper.find('.browser-monitor__resize-grip').exists()).toBe(true)
    expect(wrapper.find('.browser-monitor').attributes('style')).toContain('top: 88px;')
    expect(wrapper.find('.browser-monitor').attributes('style')).toContain('width:')
    expect(wrapper.find('.browser-monitor').attributes('style')).toContain('height:')
  })

  it('restores the closed launcher position from storage and still opens on click', async () => {
    const monitor = useBrowserMonitor()
    monitor.setOpen(false)
    listTasksMock.mockResolvedValue({ data: [] })
    localStorage.setItem('zima.browser.monitor.launcher.position.v1', '{"x":980,"y":668}')

    const wrapper = mount(BrowserMonitorWidget, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const launcher = wrapper.get('.browser-monitor-launcher')
    expect(launcher.attributes('style')).toContain('top: 668px;')
    expect(launcher.attributes('style')).toContain('left: 980px;')
    expect(monitor.isOpen.value).toBe(false)

    await launcher.trigger('click')
    expect(monitor.isOpen.value).toBe(true)
  })

  it('registers pointer listeners for launcher dragging on touch-style input', async () => {
    const monitor = useBrowserMonitor()
    monitor.setOpen(false)
    listTasksMock.mockResolvedValue({ data: [] })
    localStorage.setItem('zima.browser.monitor.launcher.position.v1', '{"x":980,"y":668}')
    const addEventListenerSpy = vi.spyOn(document, 'addEventListener')
    const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener')

    const wrapper = mount(BrowserMonitorWidget, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const setupState = (wrapper.vm.$ as any).setupState
    setupState.startLauncherDragging(
      {
        clientX: 990,
        clientY: 674,
        pointerId: 7,
        pointerType: 'touch',
        isPrimary: true,
        button: 0,
      } as PointerEvent
    )

    expect(addEventListenerSpy).toHaveBeenCalledWith('pointermove', setupState.onDrag)
    expect(addEventListenerSpy).toHaveBeenCalledWith('pointerup', setupState.stopDragging)
    expect(addEventListenerSpy).toHaveBeenCalledWith('pointercancel', setupState.stopDragging)

    setupState.stopDragging()

    expect(removeEventListenerSpy).toHaveBeenCalledWith('pointermove', setupState.onDrag)
    expect(removeEventListenerSpy).toHaveBeenCalledWith('pointerup', setupState.stopDragging)
    expect(removeEventListenerSpy).toHaveBeenCalledWith('pointercancel', setupState.stopDragging)
    expect(monitor.isOpen.value).toBe(false)
  })

  it('renders session screenshot history and updates screenshot info when switching frames', async () => {
    getSessionsMock.mockResolvedValue([
      {
        id: 'tab-1',
        status: 'active',
        current_url: 'https://live.example.com',
        page_title: 'Live Session',
        created_at: '2026-03-23T00:00:00.000Z',
        last_activity: '2026-03-23T00:00:00.000Z',
      },
    ])
    takeScreenshotMock.mockResolvedValue({
      screenshot: 'latest-base64',
      history: [
        {
          data: 'latest-base64',
          captured_at: '2026-03-23T00:00:00.000Z',
          title: 'Latest Snapshot',
          url: 'https://snapshots.example.com/latest',
          scope: 'viewport',
        },
        {
          data: 'older-base64',
          captured_at: '2026-03-22T23:58:00.000Z',
          title: 'Earlier Snapshot',
          url: 'https://snapshots.example.com/earlier',
          scope: 'full_page',
        },
      ],
      error: '',
    })

    const wrapper = mount(BrowserMonitorWidget, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const timelineShots = wrapper.findAll('.browser-monitor__timeline-shot')
    expect(timelineShots).toHaveLength(2)
    expect(wrapper.get('.browser-monitor__image').attributes('src')).toContain('latest-base64')
    expect(wrapper.get('.browser-monitor__preview-title').text()).toContain('Latest Snapshot')
    expect(wrapper.get('.browser-monitor__preview-url').text()).toContain(
      'https://snapshots.example.com/latest'
    )
    expect(wrapper.text()).toContain('视口截图')

    await timelineShots[1].trigger('click')
    expect(wrapper.get('.browser-monitor__image').attributes('src')).toContain('older-base64')
    expect(wrapper.get('.browser-monitor__preview-title').text()).toContain('Earlier Snapshot')
    expect(wrapper.get('.browser-monitor__preview-url').text()).toContain(
      'https://snapshots.example.com/earlier'
    )
    expect(wrapper.text()).toContain('整页截图')
    expect(wrapper.text()).toContain('第 2/2 帧')
  })

  it('keeps the last successful screenshot when later refreshes return no frame', async () => {
    getSessionsMock.mockResolvedValue([
      {
        id: 'tab-1',
        status: 'active',
        current_url: 'https://example.com',
        page_title: 'Example',
        created_at: '2026-03-23T00:00:00.000Z',
        last_activity: '2026-03-23T00:00:00.000Z',
      },
    ])
    takeScreenshotMock.mockResolvedValue({
      screenshot: 'stable-base64',
      history: [
        {
          data: 'stable-base64',
          captured_at: '2026-03-23T00:00:00.000Z',
          title: 'Example',
          url: 'https://example.com',
        },
      ],
      error: '',
    })

    const wrapper = mount(BrowserMonitorWidget, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('.browser-monitor__image').attributes('src')).toContain('stable-base64')

    takeScreenshotMock.mockResolvedValue({
      screenshot: '',
      history: [],
      error: 'capture failed',
    })
    const [refreshButton] = wrapper.findAll('.browser-monitor__icon-button')
    await refreshButton.trigger('click')
    await flushPromises()

    expect(wrapper.get('.browser-monitor__image').attributes('src')).toContain('stable-base64')
    expect(wrapper.text()).toContain('capture failed')

    getSessionsMock.mockResolvedValue([])
    await refreshButton.trigger('click')
    await flushPromises()

    expect(wrapper.get('.browser-monitor__image').attributes('src')).toContain('stable-base64')
    expect(wrapper.get('.browser-monitor__preview-title').text()).toContain('Example')
    expect(wrapper.get('.browser-monitor__preview-url').text()).toContain('https://example.com')
  })
})
