import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const routeMock = {
  query: {} as Record<string, unknown>,
}
const routerReplaceMock = vi.fn()

const getWeChatILinkSetupSessionMock = vi.fn()
const completeWeChatILinkSetupSessionMock = vi.fn()

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeMock,
    useRouter: () => ({
      replace: routerReplaceMock,
    }),
  }
})

vi.mock('@/api/wechat-ilink-setup', () => ({
  getWeChatILinkSetupSession: getWeChatILinkSetupSessionMock,
  completeWeChatILinkSetupSession: completeWeChatILinkSetupSessionMock,
}))

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    missingWarn: false,
    fallbackWarn: false,
    messages: {
      'en-US': {
        channels: {
          wechatILinkSetupEyebrow: 'WeChat iLink',
          wechatILinkSetupTitle: 'Finish setup',
          wechatILinkSetupDescription: 'Authorize iLink on your phone.',
          wechatILinkSetupMissingSession: 'Missing setup session.',
          wechatILinkSetupLoadFailed: 'Failed to load session.',
          wechatILinkSetupSubmitFailed: 'Failed to submit setup.',
          wechatILinkSetupSuccess: 'Configuration complete, you can close this page.',
          wechatILinkPairingPayload: 'Pairing payload',
          wechatILinkPairingPayloadPlaceholder:
            'Paste pairing payload JSON, or leave it empty and fill in the bot token below.',
          botToken: 'Bot Token',
          placeholderBotTokenGeneric: 'token',
          wechatILinkSetupSubmitting: 'Submitting...',
          wechatILinkSetupSubmit: 'Complete setup',
        },
        common: {
          loading: 'Loading',
          optional: 'Optional',
        },
      },
    },
  })
}

describe('WeChatILinkSetupView (legacy fallback)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeMock.query = {}
    getWeChatILinkSetupSessionMock.mockResolvedValue({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'pending',
      },
    })
    completeWeChatILinkSetupSessionMock.mockResolvedValue({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'connected',
      },
    })
  })

  it('shows an error when the setup session is missing', async () => {
    const View = (await import('@/views/WeChatILinkSetupView.vue')).default
    const wrapper = mount(View, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(getWeChatILinkSetupSessionMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Missing setup session.')
  })

  it('loads the session and submits only the bot token for manual fallback', async () => {
    routeMock.query = { session_id: 'session-1' }

    const View = (await import('@/views/WeChatILinkSetupView.vue')).default
    const wrapper = mount(View, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(getWeChatILinkSetupSessionMock).toHaveBeenCalledWith('session-1')

    expect(wrapper.find('input[type="url"]').exists()).toBe(false)
    await wrapper.find('input[type="password"]').setValue('bot-token')
    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(completeWeChatILinkSetupSessionMock).toHaveBeenCalledWith('session-1', {
      bot_token: 'bot-token',
    })
    expect(wrapper.text()).toContain('Configuration complete, you can close this page.')
  })

  it('auto-submits only the bot token from URL params and clears legacy api_base_url query params', async () => {
    routeMock.query = {
      session_id: 'session-1',
      api_base_url: 'admin',
      bot_token: 'bot-token',
    }

    const View = (await import('@/views/WeChatILinkSetupView.vue')).default
    const wrapper = mount(View, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(getWeChatILinkSetupSessionMock).toHaveBeenCalledWith('session-1')
    expect(completeWeChatILinkSetupSessionMock).toHaveBeenCalledWith('session-1', {
      bot_token: 'bot-token',
    })
    expect(routerReplaceMock).toHaveBeenCalledWith({
      query: {
        session_id: 'session-1',
      },
    })
    expect(wrapper.text()).toContain('Configuration complete, you can close this page.')
  })

  it('still forwards raw pairing payload strings for legacy debugging links', async () => {
    routeMock.query = {
      session_id: 'session-1',
      pairing_payload: JSON.stringify({
        api_base_url: 'https://ilink.example.com',
        bot_token: 'bot-token',
      }),
      api_base_url: 'admin',
      bot_token: 'bot-token',
    }

    const View = (await import('@/views/WeChatILinkSetupView.vue')).default
    const wrapper = mount(View, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(getWeChatILinkSetupSessionMock).toHaveBeenCalledWith('session-1')
    expect(completeWeChatILinkSetupSessionMock).toHaveBeenCalledWith(
      'session-1',
      JSON.stringify({
        api_base_url: 'https://ilink.example.com',
        bot_token: 'bot-token',
      })
    )
    expect(routerReplaceMock).toHaveBeenCalledWith({
      query: {
        session_id: 'session-1',
      },
    })
    expect(wrapper.text()).toContain('Configuration complete, you can close this page.')
  })
})
