import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  chatStore: {
    conversations: [] as Array<Record<string, unknown>>,
    currentConversationId: null as string | null,
    fetchConversations: vi.fn(),
    fetchMessages: vi.fn(),
    selectConversation: vi.fn(),
    setPendingApproval: vi.fn(),
    setPendingExecApproval: vi.fn(),
  },
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    remove: vi.fn(),
  },
  providerPoolStore: {
    providers: [
      {
        id: 'openai',
        type: 'builtin',
        status: 'error',
        last_error: 'stale_error',
      },
    ] as Array<Record<string, unknown>>,
    updateProviderStatus: vi.fn(),
    fetchProviders: vi.fn(),
  },
  settingsStore: {
    updateFromPoolProviders: vi.fn(),
  },
  consumeSSEJsonStream: vi.fn(),
  fetch: vi.fn(),
  routerPush: vi.fn(),
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mocks.chatStore,
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => mocks.providerPoolStore,
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => mocks.settingsStore,
}))

vi.mock('@/utils/authStorage', () => ({
  getStoredAccessToken: () => 'test-token',
}))

vi.mock('@/utils/sseStream', () => ({
  consumeSSEJsonStream: (...args: unknown[]) => mocks.consumeSSEJsonStream(...args),
}))

vi.mock('@/api/client', () => ({
  ensureFreshToken: vi.fn(),
}))

vi.mock('@/router', () => ({
  default: {
    push: (...args: unknown[]) => mocks.routerPush(...args),
  },
}))

vi.mock('@/i18n', () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
}))

import { useEventStream } from '@/composables/useEventStream'

describe('useEventStream provider resync', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    mocks.chatStore.currentConversationId = null
    mocks.chatStore.fetchConversations.mockResolvedValue(undefined)
    mocks.chatStore.fetchMessages.mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockResolvedValue(undefined)
    mocks.routerPush.mockResolvedValue(undefined)
    mocks.providerPoolStore.providers = [
      {
        id: 'openai',
        type: 'builtin',
        status: 'error',
        last_error: 'stale_error',
      },
    ]
    mocks.providerPoolStore.fetchProviders.mockResolvedValue(undefined)
    mocks.consumeSSEJsonStream.mockImplementation(async (_body, handlers: any) => {
      handlers.onMessage('provider_status_changed', {
        provider_id: 'openai',
        status: 'active',
      })
    })
    mocks.fetch.mockResolvedValue({
      ok: true,
      status: 200,
      body: {},
    })
    vi.stubGlobal('fetch', mocks.fetch)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('re-fetches providers after provider status changes for an existing provider', async () => {
    const stream = useEventStream()

    await stream.connect()

    expect(mocks.providerPoolStore.updateProviderStatus).toHaveBeenCalledWith('openai', 'active')
    expect(mocks.providerPoolStore.fetchProviders).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(250)

    expect(mocks.providerPoolStore.fetchProviders).toHaveBeenCalledTimes(1)
    expect(mocks.settingsStore.updateFromPoolProviders).toHaveBeenCalledWith(
      mocks.providerPoolStore.providers
    )

    stream.disconnect()
  })

  it('navigates the notification action to the conversation route when a push event targets a conversation', async () => {
    mocks.consumeSSEJsonStream.mockImplementationOnce(async (_body, handlers: any) => {
      handlers.onMessage('push', {
        message: 'Review the latest output',
        conversation_id: 'conv-2',
      })
    })

    const stream = useEventStream()

    await stream.connect()

    expect(mocks.notificationStore.info).toHaveBeenCalledWith(
      'push.reminder',
      '⏰ Review the latest output',
      expect.objectContaining({
        duration: 10000,
        action: expect.objectContaining({
          label: 'push.viewConversation',
        }),
      })
    )

    const [, , options] = mocks.notificationStore.info.mock.calls.at(-1) as [
      string,
      string,
      { action?: { handler: () => void } }
    ]
    expect(options.action).toBeDefined()

    options.action!.handler()

    expect(mocks.routerPush).toHaveBeenCalledWith({
      name: 'Chat',
      query: { conversationId: 'conv-2' },
    })
    expect(mocks.chatStore.selectConversation).not.toHaveBeenCalled()

    stream.disconnect()
  })
})
