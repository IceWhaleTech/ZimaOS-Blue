import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ChatMessage from '@/components/ChatMessage.vue'
import { i18n } from '@/i18n'

const mocks = vi.hoisted(() => ({
  chatStore: {
    messages: [] as Array<Record<string, unknown>>,
    selectedMessageIds: new Set<string>(),
    isMultiSelectMode: false,
    toolExecuting: false,
    toolExecutingCommands: [] as string[],
    toolExecutingNames: [] as string[],
    toolExecutingStartTime: 0,
    toolResults: [] as unknown[],
    toolSandboxAvailable: false,
    sendMessage: vi.fn(),
    getMessageMetadata: vi.fn(() => null),
    toggleMessageSelection: vi.fn(),
    enterMultiSelectMode: vi.fn(),
    deleteSelectedMessages: vi.fn(),
  },
  settingsStore: {
    showToolDetails: true,
  },
  providerPoolStore: {
    providers: [] as unknown[],
    getProviderDisplayName: vi.fn((providerId: string) => providerId),
  },
  cardActionSubmit: vi.fn(),
  speechGetStatus: vi.fn(),
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    remove: vi.fn(),
  },
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mocks.chatStore,
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => mocks.settingsStore,
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => mocks.providerPoolStore,
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/api/chat', () => ({
  cardActionApi: {
    submit: (...args: unknown[]) => mocks.cardActionSubmit(...args),
  },
}))

vi.mock('@/api/voice', () => ({
  ttsAudioManager: {
    stop: vi.fn(),
  },
  streamingTTSManager: {
    stop: vi.fn(),
    streamAndPlay: vi.fn(),
    reset: vi.fn(),
    streamText: vi.fn(),
    play: vi.fn(),
    onComplete: null as null | (() => void),
  },
}))

vi.mock('@/api/speech', () => ({
  speechApi: {
    getStatus: (...args: unknown[]) => mocks.speechGetStatus(...args),
  },
}))

const WEB_FETCH_URL = 'https://www.reddit.com/r/test'
const WEB_FETCH_CARD_ID = 'web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest'
const BROWSER_CARD_ID = 'browser-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest'

function makeMessage(content: string) {
  return {
    id: 'msg-1',
    conversation_id: 'conv-1',
    role: 'assistant' as const,
    content,
    created_at: '2026-03-08T00:00:00.000Z',
  }
}

function makeTypelessBlock(payload: Record<string, unknown>) {
  return ['```typeless', JSON.stringify(payload), '```'].join('\n')
}

async function mountMessage(content: string) {
  const wrapper = mount(ChatMessage, {
    props: {
      message: makeMessage(content),
      disableAutoTTS: true,
    },
    global: {
      plugins: [i18n],
      stubs: {
        AssistantTextState: true,
        MediaPlaceholder: true,
        Teleport: true,
        ToolDetailCard: true,
        Transition: true,
      },
    },
  })

  await flushPromises()
  await vi.dynamicImportSettled()
  await flushPromises()
  return wrapper
}

function findButtonByText(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll('button').find((button) => button.text().includes(text))
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('ChatMessage card actions', () => {
  beforeEach(() => {
    mocks.chatStore.messages = []
    mocks.chatStore.selectedMessageIds = new Set<string>()
    mocks.chatStore.isMultiSelectMode = false
    mocks.chatStore.toolExecuting = false
    mocks.chatStore.toolExecutingCommands = []
    mocks.chatStore.toolExecutingNames = []
    mocks.chatStore.toolExecutingStartTime = 0
    mocks.chatStore.toolResults = []
    mocks.chatStore.toolSandboxAvailable = false
    mocks.chatStore.sendMessage.mockReset().mockResolvedValue(undefined)
    mocks.chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
    mocks.chatStore.toggleMessageSelection.mockReset()
    mocks.chatStore.enterMultiSelectMode.mockReset()
    mocks.chatStore.deleteSelectedMessages.mockReset()

    mocks.settingsStore.showToolDetails = true

    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.getProviderDisplayName
      .mockReset()
      .mockImplementation((providerId: string) => providerId)

    mocks.cardActionSubmit.mockReset()
    mocks.speechGetStatus.mockReset().mockResolvedValue({ data: {} })

    mocks.notificationStore.success.mockReset()
    mocks.notificationStore.error.mockReset()
    mocks.notificationStore.info.mockReset()
    mocks.notificationStore.remove.mockReset()
  })

  it('submits use_browser with the web-fetch card metadata and sends the follow-up message', async () => {
    mocks.cardActionSubmit.mockResolvedValue({
      data: {
        success: true,
        message: `Open ${WEB_FETCH_URL} with the browser tool.`,
      },
    })

    const content = makeTypelessBlock({
      type: 'web-fetch',
      id: WEB_FETCH_CARD_ID,
      title: 'Sign in',
      status: 'warning',
      url: WEB_FETCH_URL,
      content: 'Log in to continue',
      content_type: 'text/html',
      extract_mode: 'text',
      extractor: 'html',
      warning: 'page appears to be a login wall; use browser or pass browser_target_id',
      warning_code: 'login_wall',
      actions: [
        {
          id: 'use_browser',
          label: 'Use browser',
          variant: 'primary',
          form_data: {
            url: WEB_FETCH_URL,
          },
        },
      ],
    })

    const wrapper = await mountMessage(content)
    const button = findButtonByText(wrapper, 'Use browser')

    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Sign in',
      form_data: {
        url: WEB_FETCH_URL,
      },
    })
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(
      `Open ${WEB_FETCH_URL} with the browser tool.`
    )
  })

  it('keeps chinese fallback boilerplate and extracted summary text', async () => {
    const content =
      '工具执行已完成，但最终总结生成失败。以下是基于工具结果整理的简要摘要：\n\nWeb search fallback results for "OpenClaw 最近动向":\n\n- OpenClaw Release Notes\n\n原始 stdout/stderr/error 字段未包含在这条简要摘要中。如需我重试完整总结，请回复“重试总结”。'

    const wrapper = await mountMessage(content)
    const assistant = wrapper.find('.assistant-message')

    expect(assistant.exists()).toBe(true)
    expect(assistant.text()).toContain('Web search fallback results for "OpenClaw 最近动向"')
    expect(assistant.text()).toContain('OpenClaw Release Notes')
    expect(assistant.text()).toContain('工具执行已完成，但最终总结生成失败')
    expect(assistant.text()).toContain('简要摘要')
    expect(assistant.text()).toContain('原始 stdout/stderr/error 字段未包含在这条简要摘要中')
  })

  it('keeps fallback boilerplate when typeless cards are mixed into the message', async () => {
    const content = [
      '工具执行已完成，但最终总结生成失败。以下是基于工具结果整理的简要摘要：',
      '',
      'Web search fallback results for "OpenClaw 最近动向":',
      '',
      makeTypelessBlock({
        type: 'search',
        title: 'OpenClaw Release Notes',
        query: 'OpenClaw recent updates',
        results: [
          {
            title: 'OpenClaw Release Notes',
            url: 'https://github.com/opendungeons/openclaw/releases',
            description: 'recent release notes',
          },
        ],
      }),
      '',
      '原始 stdout/stderr/error 字段未包含在这条简要摘要中。如需我重试完整总结，请回复“重试总结”。',
    ].join('\n')

    const wrapper = await mountMessage(content)
    const assistant = wrapper.find('.assistant-message')

    expect(assistant.exists()).toBe(true)
    expect(assistant.text()).toContain('Web search fallback results for "OpenClaw 最近动向"')
    expect(assistant.text()).toContain('OpenClaw Release Notes')
    expect(assistant.text()).toContain('工具执行已完成，但最终总结生成失败')
    expect(assistant.text()).toContain('简要摘要')
    expect(assistant.text()).toContain('原始 stdout/stderr/error 字段未包含在这条简要摘要中')
  })

  it('submits browser extract action with browser_target_id and sends the mapped follow-up message', async () => {
    mocks.cardActionSubmit.mockResolvedValue({
      data: {
        success: true,
        message: `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
      },
    })

    const content = makeTypelessBlock({
      type: 'result',
      id: BROWSER_CARD_ID,
      title: 'Browser page',
      status: 'info',
      message: 'Interactive page opened in the browser session.',
      details: [
        { label: 'url', value: WEB_FETCH_URL },
        { label: 'browser_target_id', value: 'tab-42' },
      ],
      actions: [
        {
          id: 'extract_with_web_fetch',
          label: 'Extract readable content',
          variant: 'primary',
          form_data: {
            url: WEB_FETCH_URL,
            browser_target_id: 'tab-42',
          },
        },
      ],
    })

    const wrapper = await mountMessage(content)
    const button = findButtonByText(wrapper, 'Extract readable content')

    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-1', {
      card_id: BROWSER_CARD_ID,
      action_id: 'extract_with_web_fetch',
      action_label: 'Extract readable content',
      card_type: 'result',
      card_title: 'Browser page',
      form_data: {
        url: WEB_FETCH_URL,
        browser_target_id: 'tab-42',
      },
    })
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(
      `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`
    )
  })

  it('keeps web-fetch cards visible when tool details are hidden', async () => {
    mocks.settingsStore.showToolDetails = false

    const content = makeTypelessBlock({
      type: 'web-fetch',
      id: WEB_FETCH_CARD_ID,
      title: 'Sign in',
      status: 'warning',
      url: WEB_FETCH_URL,
      content: 'Log in to continue',
      content_type: 'text/html',
      extract_mode: 'text',
      extractor: 'html',
      warning: 'page appears to be a login wall; use browser or pass browser_target_id',
      warning_code: 'login_wall',
      actions: [
        {
          id: 'use_browser',
          label: 'Use browser',
          variant: 'primary',
          form_data: {
            url: WEB_FETCH_URL,
          },
        },
      ],
    })

    const wrapper = await mountMessage(content)
    const button = findButtonByText(wrapper, 'Use browser')

    expect(button?.exists()).toBe(true)
    expect(wrapper.text()).toContain('Sign in')
    expect(wrapper.text()).toContain('reddit.com')
  })

  it('shows a loading state and prevents duplicate web-fetch submissions while the action is pending', async () => {
    const deferred = createDeferred<{ data: { success: boolean; message: string } }>()
    mocks.cardActionSubmit.mockReturnValue(deferred.promise)

    const content = makeTypelessBlock({
      type: 'web-fetch',
      id: WEB_FETCH_CARD_ID,
      title: 'Sign in',
      status: 'warning',
      url: WEB_FETCH_URL,
      content: 'Log in to continue',
      content_type: 'text/html',
      extract_mode: 'text',
      extractor: 'html',
      warning: 'page appears to be a login wall; use browser or pass browser_target_id',
      warning_code: 'login_wall',
      actions: [
        {
          id: 'use_browser',
          label: 'Use browser',
          variant: 'primary',
          form_data: { url: WEB_FETCH_URL },
        },
      ],
    })

    const wrapper = await mountMessage(content)
    const button = findButtonByText(wrapper, 'Use browser')

    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await button!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(button!.attributes('disabled')).toBeDefined()
    expect(button!.attributes('aria-busy')).toBe('true')
    expect(button!.text()).toContain('Processing...')

    deferred.resolve({
      data: {
        success: true,
        message: `Open ${WEB_FETCH_URL} with the browser tool.`,
      },
    })
    await flushPromises()

    expect(button!.attributes('disabled')).toBeUndefined()
    expect(button!.text()).toContain('Use browser')
  })

  it('shows a loading state for browser extract actions while the request is pending', async () => {
    const deferred = createDeferred<{ data: { success: boolean; message: string } }>()
    mocks.cardActionSubmit.mockReturnValue(deferred.promise)

    const content = makeTypelessBlock({
      type: 'result',
      id: BROWSER_CARD_ID,
      title: 'Browser page',
      status: 'info',
      message: 'Interactive page opened in the browser session.',
      details: [
        { label: 'url', value: WEB_FETCH_URL },
        { label: 'browser_target_id', value: 'tab-42' },
      ],
      actions: [
        {
          id: 'extract_with_web_fetch',
          label: 'Extract readable content',
          variant: 'primary',
          form_data: {
            url: WEB_FETCH_URL,
            browser_target_id: 'tab-42',
          },
        },
      ],
    })

    const wrapper = await mountMessage(content)
    const button = findButtonByText(wrapper, 'Extract readable content')

    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(button!.attributes('disabled')).toBeDefined()
    expect(button!.attributes('aria-busy')).toBe('true')
    expect(button!.text()).toContain('Processing...')

    deferred.resolve({
      data: {
        success: true,
        message: `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
      },
    })
    await flushPromises()

    expect(button!.attributes('disabled')).toBeUndefined()
    expect(button!.text()).toContain('Extract readable content')
  })

  it('shows a loading state for action cards while the request is pending', async () => {
    const deferred = createDeferred<{ data: { success: boolean; message: string } }>()
    mocks.cardActionSubmit.mockReturnValue(deferred.promise)

    const wrapper = await mountMessage(
      makeTypelessBlock({
        type: 'action',
        id: 'action-card-1',
        title: 'Choose next step',
        description: 'Continue with the browser flow.',
        actions: [{ id: 'continue', label: 'Continue', variant: 'primary' }],
      })
    )

    const button = findButtonByText(wrapper, 'Continue')
    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(button!.attributes('disabled')).toBeDefined()
    expect(button!.attributes('aria-busy')).toBe('true')
    expect(button!.text()).toContain('Processing...')

    deferred.resolve({
      data: {
        success: true,
        message: 'Continue with the next step.',
      },
    })
    await flushPromises()

    expect(button!.attributes('disabled')).toBeUndefined()
    expect(button!.text()).toContain('Continue')
  })

  it('shows a loading state for ui-review retry actions while the request is pending', async () => {
    const deferred = createDeferred<{ data: { success: boolean; message: string } }>()
    mocks.cardActionSubmit.mockReturnValue(deferred.promise)

    const wrapper = await mountMessage(
      makeTypelessBlock({
        type: 'ui-review',
        id: 'ui-review-1',
        status: 'error',
        message: 'Browser start failed',
        actions: [{ id: 'retry', label: 'Retry', variant: 'primary' }],
      })
    )

    const button = findButtonByText(wrapper, 'Retry')
    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(button!.attributes('disabled')).toBeDefined()
    expect(button!.attributes('aria-busy')).toBe('true')
    expect(button!.text()).toContain('Processing...')

    deferred.resolve({
      data: {
        success: true,
        message: 'Retry the UI review.',
      },
    })
    await flushPromises()

    expect(button!.attributes('disabled')).toBeUndefined()
    expect(button!.text()).toContain('Retry')
  })

  it('shows an inline error for failed web-fetch actions and clears it after a successful retry', async () => {
    mocks.cardActionSubmit
      .mockRejectedValueOnce(new Error('Browser service unavailable'))
      .mockResolvedValueOnce({
        data: {
          success: true,
          message: `Open ${WEB_FETCH_URL} with the browser tool.`,
        },
      })

    const content = makeTypelessBlock({
      type: 'web-fetch',
      id: WEB_FETCH_CARD_ID,
      title: 'Sign in',
      status: 'warning',
      url: WEB_FETCH_URL,
      content: 'Log in to continue',
      content_type: 'text/html',
      extract_mode: 'text',
      extractor: 'html',
      warning: 'page appears to be a login wall; use browser or pass browser_target_id',
      warning_code: 'login_wall',
      actions: [
        {
          id: 'use_browser',
          label: 'Use browser',
          variant: 'primary',
          form_data: { url: WEB_FETCH_URL },
        },
      ],
    })

    const wrapper = await mountMessage(content)
    const button = findButtonByText(wrapper, 'Use browser')

    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.chatStore.sendMessage).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Browser service unavailable')
    expect(button!.attributes('disabled')).toBeUndefined()

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(2)
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(
      `Open ${WEB_FETCH_URL} with the browser tool.`
    )
    expect(wrapper.text()).not.toContain('Browser service unavailable')
  })

  it('shows an inline error for failed choice submissions', async () => {
    mocks.cardActionSubmit.mockRejectedValueOnce(new Error('Selection failed upstream'))

    const wrapper = await mountMessage(
      makeTypelessBlock({
        type: 'choice',
        id: 'choice-card-1',
        title: 'Choose a source',
        multiple: true,
        options: [
          { id: 'alpha', label: 'Alpha' },
          { id: 'beta', label: 'Beta' },
        ],
      })
    )

    const alphaButton = findButtonByText(wrapper, 'Alpha')
    expect(alphaButton?.exists()).toBe(true)

    await alphaButton!.trigger('click')
    await flushPromises()

    expect(mocks.chatStore.sendMessage).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Selection failed upstream')
    expect(alphaButton!.attributes('disabled')).toBeUndefined()
  })

  it('disables choice interactions while selection submission is pending', async () => {
    const deferred = createDeferred<{ data: { success: boolean; message: string } }>()
    mocks.cardActionSubmit.mockReturnValue(deferred.promise)

    const wrapper = await mountMessage(
      makeTypelessBlock({
        type: 'choice',
        id: 'choice-card-1',
        title: 'Choose a source',
        multiple: true,
        options: [
          { id: 'alpha', label: 'Alpha' },
          { id: 'beta', label: 'Beta' },
        ],
      })
    )

    const alphaButton = findButtonByText(wrapper, 'Alpha')
    const betaButton = findButtonByText(wrapper, 'Beta')

    expect(alphaButton?.exists()).toBe(true)
    expect(betaButton?.exists()).toBe(true)

    await alphaButton!.trigger('click')
    await betaButton!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-1', {
      card_id: 'choice-card-1',
      action_id: 'select',
      action_label: 'Alpha',
      form_data: {
        selected_ids: ['alpha'],
      },
    })
    expect(alphaButton!.attributes('disabled')).toBeDefined()
    expect(betaButton!.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('Processing...')

    deferred.resolve({
      data: {
        success: true,
        message: 'Selection received.',
      },
    })
    await flushPromises()

    expect(alphaButton!.attributes('disabled')).toBeUndefined()
    expect(betaButton!.attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toContain('Processing...')
  })

  it('renders persisted process summaries as tool detail cards for completed messages', async () => {
    const content = [
      '目录已经创建。',
      '',
      '<!-- process-start -->',
      '```process',
      '[{"cmd":"mkdir -p /Users/orca/.zimaos-blue/data/workspace/tank-battle","tool":"exec","icon":"✓","status":"25ms","output":""}]',
      '```',
      '<!-- process-end -->',
    ].join('\n')

    const wrapper = await mountMessage(content)

    expect(wrapper.find('.assistant-message').exists()).toBe(true)
    expect(wrapper.text()).toContain('目录已经创建')
    expect(wrapper.findAll('tool-detail-card-stub')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('```process')
    expect(wrapper.text()).not.toContain('<!-- process-start -->')
  })
})
