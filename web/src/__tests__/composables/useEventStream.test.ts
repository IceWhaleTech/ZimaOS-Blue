import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  chatStore: {
    conversations: [] as Array<Record<string, unknown>>,
    fetchMessages: vi.fn(),
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
})
