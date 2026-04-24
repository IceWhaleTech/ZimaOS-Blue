import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

const settingsStoreMock = {
  backendSettings: {},
  fetchBackendSettings: vi.fn().mockResolvedValue(undefined),
}

const listChannelsMock = vi.fn()
const getChannelSettingsMock = vi.fn()
const updateChannelSettingsMock = vi.fn()
const updateChannelMock = vi.fn()
const toggleChannelMock = vi.fn()
const getChannelStatusMock = vi.fn()
const testChannelConnectionMock = vi.fn()
const getTunnelProvidersMock = vi.fn()
const getRemoteAccessConfigMock = vi.fn()
const getRemoteAccessStatusMock = vi.fn()
const createWeChatILinkSetupSessionMock = vi.fn()
const getWeChatILinkSetupSessionMock = vi.fn()

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsStoreMock,
}))

vi.mock('@/api/channels', () => ({
  channelsApi: {
    list: listChannelsMock,
    getSettings: getChannelSettingsMock,
    updateSettings: updateChannelSettingsMock,
    updateChannel: updateChannelMock,
    toggleChannel: toggleChannelMock,
    getChannelStatus: getChannelStatusMock,
    testConnection: testChannelConnectionMock,
  },
}))

vi.mock('@/api/remote-access', () => ({
  getTunnelProviders: getTunnelProvidersMock,
  getRemoteAccessConfig: getRemoteAccessConfigMock,
  getRemoteAccessStatus: getRemoteAccessStatusMock,
  updateRemoteAccessConfig: vi.fn(),
  startRemoteAccess: vi.fn(),
  stopRemoteAccess: vi.fn(),
}))

vi.mock('@/api/wechat-ilink-setup', () => ({
  createWeChatILinkSetupSession: createWeChatILinkSetupSessionMock,
  getWeChatILinkSetupSession: getWeChatILinkSetupSessionMock,
}))

vi.mock('@/components/channels/ChannelCard.vue', () => ({
  default: {
    name: 'ChannelCard',
    props: ['channel'],
    template:
      '<div class="channel-card-stub" :data-id="channel.id" :data-enabled="channel.enabled ? \'true\' : \'false\'" :data-status="channel.status">{{ channel.id }}<button class="channel-card-select-stub" @click="$emit(\'toggle\')">select</button></div>',
  },
}))

vi.mock('@/components/channels/ChannelDetailPanel.vue', () => ({
  default: {
    name: 'ChannelDetailPanel',
    props: ['channel'],
    template:
      '<div class="channel-detail-stub" :data-id="channel.id" :data-last-field="channel.fields?.[channel.fields.length - 1]?.key" :data-last-value="channel.fields?.[channel.fields.length - 1]?.value">{{ channel.id }}<button class="channel-detail-update-stub" @click="$emit(\'update-field\', channel.fields.length - 1, \'true\')">update</button><button class="channel-detail-save-stub" @click="$emit(\'save\')">save</button></div>',
  },
}))

vi.mock('@/components/remote-access/TunnelStatus.vue', () => ({
  default: {
    name: 'TunnelStatus',
    template: '<div class="tunnel-status-stub"></div>',
  },
}))

vi.mock('@/components/remote-access/RemoteAccessDetailPanel.vue', () => ({
  default: {
    name: 'RemoteAccessDetailPanel',
    template: '<div class="remote-access-detail-stub">remote-access-detail</div>',
  },
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
        nav: {
          configuration: 'Configuration',
        },
        channels: {
          title: 'Channels',
          subtitle: 'Connect external channels',
          enabledChannels: 'Enabled channels',
          connectedChannels: 'Connected channels',
          partialLoadTitle: 'Channels did not fully load',
          networkError: 'Network Error',
          statusConnected: 'Connected',
          statusConnecting: 'Connecting',
          statusError: 'Error',
          statusDisconnected: 'Disconnected',
          groupAccessTitle: 'Group Access',
          groupAccessDesc: 'Set one unified rule for whether Blue accepts inbound group messages.',
          groupAccessPolicyOpen: 'Open',
          groupAccessPolicyAllowlist: 'Allowlist',
          groupAccessPolicyDisabled: 'Disabled',
          groupAccessMentionPolicyMentioned: 'Only reply when Blue is mentioned',
          groupAccessMentionPolicyAlways: 'Reply to all allowed group messages',
          groupAccessAllowedChats: 'Allowed Group Chats',
          groupAccessPolicy: 'Policy',
          groupAccessHint: 'Group access hint',
          groupAccessMentionPolicy: 'Reply Condition',
          groupAccessMentionHint: 'Mention hint',
          groupAccessAllowedChatsPlaceholder: 'feishu:oc_xxx_allowed',
          groupAccessAllowedChatsHint: 'Allowed chats hint',
          saving: 'Saving...',
          wechatILinkPrimaryAction: 'Primary Action',
          wechatILinkScanAction: 'Scan To Connect',
          wechatILinkManualAction: 'Manual Config',
          wechatILinkScanHint: 'Scan the iLink QR code with WeChat and confirm the login on your phone.',
          wechatILinkSetupCreating: 'Creating Session...',
          wechatILinkSetupDescription:
            'Scan the QR code with WeChat and confirm the iLink login. Blue will enable the channel automatically.',
          wechatILinkSetupStatus: 'Setup Status',
          wechatILinkSetupStatePending: 'Pending',
          wechatILinkSetupStateAuthorizing: 'Authorizing',
          wechatILinkSetupStateConfiguring: 'Configuring',
          wechatILinkSetupStateConnected: 'Connected',
          wechatILinkSetupStateError: 'Error',
          wechatILinkSetupStateExpired: 'Expired',
          wechatILinkOpenOnPhone: 'Open Authorization Link',
        },
        common: {
          loading: 'Loading',
          save: 'Save',
          select: 'Select',
          configure: 'Configure',
          loadMore: 'Load More',
          optional: 'Optional',
          retry: 'Retry',
          cancel: 'Cancel',
          close: 'Close',
        },
        remoteAccess: {
          title: 'Remote Access',
          recommended: 'Recommended',
          channelDescription: 'Expose Blue securely',
        },
      },
    },
  })
}

describe('ChannelsView', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  beforeEach(() => {
    vi.clearAllMocks()
    settingsStoreMock.backendSettings = {}
    settingsStoreMock.fetchBackendSettings.mockResolvedValue(undefined)

    listChannelsMock.mockResolvedValue({
      status: 401,
      data: {},
    })
    getChannelSettingsMock.mockResolvedValue({
      status: 401,
      data: {},
    })
    updateChannelSettingsMock.mockResolvedValue({
      status: 404,
      data: {},
    })
    updateChannelMock.mockResolvedValue({
      status: 404,
      data: {},
    })
    toggleChannelMock.mockResolvedValue({
      status: 404,
      data: {},
    })
    getChannelStatusMock.mockResolvedValue({
      status: 404,
      data: {},
    })
    testChannelConnectionMock.mockResolvedValue({
      status: 404,
      data: {},
    })

    getTunnelProvidersMock.mockResolvedValue({ data: { providers: [] } })
    getRemoteAccessConfigMock.mockResolvedValue({ data: { config: {} } })
    getRemoteAccessStatusMock.mockResolvedValue({
      data: { tunnel: { active: false } },
    })
    createWeChatILinkSetupSessionMock.mockResolvedValue({
      status: 404,
      data: {},
    })
    getWeChatILinkSetupSessionMock.mockResolvedValue({
      status: 404,
      data: {},
    })
  })

  it('still renders the page shell when channel APIs are unauthorized', async () => {
    const ChannelsView = (await import('@/views/ChannelsView.vue')).default

    const wrapper = shallowMount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(wrapper.find('.channels-page').exists()).toBe(true)
    expect(wrapper.text()).toContain('Channels')
    expect(wrapper.text()).not.toContain('Loading')
    expect(wrapper.text()).toContain('Channels did not fully load')
    expect(wrapper.text()).toContain('401')
    expect(wrapper.find('.channels-board__detail-empty').exists()).toBe(true)
    expect(wrapper.find('.channel-detail-stub').exists()).toBe(false)
    expect(wrapper.findAll('channel-card-stub').length).toBeGreaterThan(0)
  })

  it('localizes the partial-load banner title and exact network error description', async () => {
    listChannelsMock.mockRejectedValueOnce(new Error('Network Error'))

    const zhCNMessages = mergeHarnessLocale(
      'zh-CN',
      (await import('@/i18n/locales/zh-CN')).default as Record<string, unknown>
    )
    const i18n = createI18n({
      legacy: false,
      locale: 'zh-CN',
      fallbackLocale: 'zh-CN',
      missingWarn: false,
      fallbackWarn: false,
      messages: {
        'zh-CN': zhCNMessages,
      },
    })
    const ChannelsView = (await import('@/views/ChannelsView.vue')).default

    const wrapper = shallowMount(ChannelsView, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('频道未完全加载')
    expect(wrapper.text()).toContain('网络错误')
    expect(wrapper.text()).not.toContain('Channels did not fully load')
    expect(wrapper.text()).not.toContain('Network Error')
  })

  it('renders group access as the fourth summary card and opens the modal', async () => {
    const ChannelsView = (await import('@/views/ChannelsView.vue')).default

    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.findAll('.channels-summary-grid .channels-summary-card')).toHaveLength(4)
    expect(wrapper.text()).toContain('Group Access')

    await wrapper.find('.channels-summary-button').trigger('click')

    expect(wrapper.find('.channels-group-modal').exists()).toBe(true)
    expect(wrapper.find('.channels-group-modal__title').text()).toBe('Group Access')
  })

  it('shows remote access details in the right detail panel when selected', async () => {
    const ChannelsView = (await import('@/views/ChannelsView.vue')).default

    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('.remote-access-detail-stub').exists()).toBe(false)
    expect(wrapper.find('.channels-board__detail-empty').exists()).toBe(true)

    await wrapper.find('.channels-remote-card__header').trigger('click')
    await flushPromises()

    expect(wrapper.find('.channels-board__detail-empty').exists()).toBe(false)
    expect(wrapper.find('.remote-access-detail-stub').exists()).toBe(true)
  })

  it('renders the remote access card with the shared status badge pattern', async () => {
    const ChannelsView = (await import('@/views/ChannelsView.vue')).default

    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const statusBadge = wrapper.find('.channels-remote-card__status-badge')
    expect(statusBadge.exists()).toBe(true)
    expect(statusBadge.classes()).toContain('channels-remote-card__status-badge--disconnected')
    expect(statusBadge.text()).toContain('Disconnected')
    expect(statusBadge.find('.channels-remote-card__status-dot').exists()).toBe(true)
  })

  it('updates Feishu session mode locally before saving through the channel config API', async () => {
    let savedBody: Record<string, unknown> | null = null
    listChannelsMock.mockResolvedValue({
      status: 200,
      data: {
        channels: [
          {
            id: 'feishu',
            enabled: false,
            status: 'disconnected',
            config: {
              app_id: 'cli_test',
              app_secret: 'secret',
              session_mode: 'false',
            },
          },
        ],
      },
    })
    getChannelSettingsMock.mockResolvedValue({
      status: 200,
      data: {},
    })
    updateChannelMock.mockImplementation((_channelId: string, payload: Record<string, unknown>) => {
      savedBody = payload
      return Promise.resolve({
        status: 200,
        data: {
          channel: {
            status: 'disconnected',
          },
        },
      })
    })

    const ChannelsView = (await import('@/views/ChannelsView.vue')).default
    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const loadMoreButton = wrapper.find('.channels-load-more')
    if (loadMoreButton.exists()) {
      await loadMoreButton.trigger('click')
      await flushPromises()
    }

    const feishuCard = wrapper.find('.channel-card-stub[data-id="feishu"]')
    expect(feishuCard.exists()).toBe(true)
    expect(wrapper.find('.channel-detail-stub').exists()).toBe(false)
    await feishuCard.find('.channel-card-select-stub').trigger('click')
    await flushPromises()

    const feishuDetail = wrapper.find('.channel-detail-stub[data-id="feishu"]')
    expect(feishuDetail.exists()).toBe(true)
    expect(feishuDetail.attributes('data-last-field')).toBe('session_mode')
    expect(feishuDetail.attributes('data-last-value')).toBe('false')

    await feishuDetail.find('.channel-detail-update-stub').trigger('click')
    await flushPromises()

    expect(
      wrapper.find('.channel-detail-stub[data-id="feishu"]').attributes('data-last-value')
    ).toBe('true')

    await feishuDetail.find('.channel-detail-save-stub').trigger('click')
    await flushPromises()

    expect(savedBody).not.toBeNull()
    expect(savedBody).toMatchObject({
      enabled: false,
      config: {
        app_id: 'cli_test',
        app_secret: 'secret',
        session_mode: 'true',
      },
    })
  })

  it('keeps WeChat Work detail panel while hiding WeChat iLink manual config', async () => {
    listChannelsMock.mockResolvedValue({
      status: 200,
      data: {
        channels: [
          {
            id: 'wechat',
            enabled: false,
            status: 'disconnected',
            config: {
              corp_id: 'ww1807890abcdef',
              agent_id: '1000001',
              secret: 'enterprise-secret',
            },
          },
          {
            id: 'wechat_ilink',
            enabled: false,
            status: 'disconnected',
            config: {
              api_base_url: 'https://ilink.example.com',
              bot_token: 'bot-token',
            },
          },
        ],
      },
    })
    getChannelSettingsMock.mockResolvedValue({
      status: 200,
      data: {},
    })

    const ChannelsView = (await import('@/views/ChannelsView.vue')).default
    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const loadMoreButton = wrapper.find('.channels-load-more')
    if (loadMoreButton.exists()) {
      await loadMoreButton.trigger('click')
      await flushPromises()
    }

    const wechatWorkCard = wrapper.find('.channel-card-stub[data-id="wechat"]')
    expect(wechatWorkCard.exists()).toBe(true)

    const wechatILinkCard = wrapper.find('.channel-card-stub[data-id="wechat_ilink"]')
    expect(wechatILinkCard.exists()).toBe(true)
    await wechatILinkCard.find('.channel-card-select-stub').trigger('click')
    await flushPromises()

    const wechatILinkDetail = wrapper.find('.channel-detail-stub[data-id="wechat_ilink"]')
    expect(wechatILinkDetail.exists()).toBe(false)

    await wechatWorkCard.find('.channel-card-select-stub').trigger('click')
    await flushPromises()

    const wechatWorkDetail = wrapper.find('.channel-detail-stub[data-id="wechat"]')
    expect(wechatWorkDetail.exists()).toBe(true)
    expect(wechatWorkDetail.attributes('data-last-field')).toBe('secret')
    expect(wechatWorkDetail.attributes('data-last-value')).toBe('enterprise-secret')
  })

  it('starts WeChat iLink setup and shows the QR modal', async () => {
    createWeChatILinkSetupSessionMock.mockResolvedValue({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'pending',
        scan_url: 'https://ilinkai.weixin.qq.com/connect/scan-session-1',
        mobile_url: 'https://blue.example.com/channels/setup/wechat_ilink?session_id=session-1',
        expires_at: '2026-04-14T10:10:00Z',
      },
    })
    listChannelsMock.mockResolvedValue({
      status: 200,
      data: {
        channels: [
          {
            id: 'wechat_ilink',
            enabled: false,
            status: 'disconnected',
            config: {},
          },
        ],
      },
    })
    getChannelSettingsMock.mockResolvedValue({
      status: 200,
      data: {},
    })

    const ChannelsView = (await import('@/views/ChannelsView.vue')).default
    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const loadMoreButton = wrapper.find('.channels-load-more')
    if (loadMoreButton.exists()) {
      await loadMoreButton.trigger('click')
      await flushPromises()
    }

    const wechatCard = wrapper.find('.channel-card-stub[data-id="wechat"]')
    if (wechatCard.exists()) {
      await wechatCard.find('.channel-card-select-stub').trigger('click')
      await flushPromises()
    }

    const wechatILinkCard = wrapper.find('.channel-card-stub[data-id="wechat_ilink"]')
    expect(wechatILinkCard.exists()).toBe(true)
    await wechatILinkCard.find('.channel-card-select-stub').trigger('click')
    await flushPromises()

    const startButton = wrapper.find('.channels-ilink-setup-card__primary')
    expect(startButton.exists()).toBe(true)
    await startButton.trigger('click')
    await flushPromises()

    expect(createWeChatILinkSetupSessionMock).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.channels-ilink-modal').exists()).toBe(true)
    expect(
      wrapper
        .find('.channels-ilink-modal')
        .find('[data-qr-value="https://ilinkai.weixin.qq.com/connect/scan-session-1"]')
        .exists()
    ).toBe(true)
    const mobileLink = wrapper.find('.channels-ilink-modal__link')
    expect(mobileLink.exists()).toBe(true)
    expect(mobileLink.attributes('href')).toBe('https://ilinkai.weixin.qq.com/connect/scan-session-1')
    expect(mobileLink.attributes('href')).not.toContain('/channels/setup/wechat_ilink')
  })

  it('keeps the QR code visible during polling, uses the upstream scan link, auto-refreshes on connected, closes the modal, enables the channel, hides retry before failure, and removes the manual action', async () => {
    vi.useFakeTimers()
    createWeChatILinkSetupSessionMock.mockResolvedValue({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'pending',
        scan_url: 'https://ilinkai.weixin.qq.com/connect/scan-session-1',
        mobile_url: 'https://blue.example.com/channels/setup/wechat_ilink?session_id=session-1',
        expires_at: '2026-04-14T10:10:00Z',
      },
    })
    getWeChatILinkSetupSessionMock.mockResolvedValueOnce({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'authorizing',
        expires_at: '2026-04-14T10:10:00Z',
      },
    })
    getWeChatILinkSetupSessionMock.mockResolvedValueOnce({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'connected',
        message: 'configured',
        expires_at: '2026-04-14T10:10:00Z',
      },
    })
    listChannelsMock.mockResolvedValue({
      status: 200,
      data: {
        channels: [
          {
            id: 'wechat_ilink',
            enabled: true,
            status: 'connected',
            config: {},
          },
        ],
      },
    })
    getChannelSettingsMock.mockResolvedValue({
      status: 200,
      data: {},
    })

    const ChannelsView = (await import('@/views/ChannelsView.vue')).default
    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const loadMoreButton = wrapper.find('.channels-load-more')
    if (loadMoreButton.exists()) {
      await loadMoreButton.trigger('click')
      await flushPromises()
    }

    const wechatILinkCard = wrapper.find('.channel-card-stub[data-id="wechat_ilink"]')
    expect(wechatILinkCard.exists()).toBe(true)
    await wechatILinkCard.find('.channel-card-select-stub').trigger('click')
    await flushPromises()

    expect(wrapper.find('.channels-ilink-setup-card__secondary').exists()).toBe(false)

    const startButton = wrapper.find('.channels-ilink-setup-card__primary')
    await startButton.trigger('click')
    await flushPromises()

    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()

    expect(getWeChatILinkSetupSessionMock).toHaveBeenCalledWith('session-1')
    expect(
      wrapper
        .find('.channels-ilink-modal')
        .find('[data-qr-value="https://ilinkai.weixin.qq.com/connect/scan-session-1"]')
        .exists()
    ).toBe(true)
    expect(wrapper.find('.channels-ilink-modal__status').text()).toContain('Authorizing')
    expect(wrapper.find('.channels-ilink-modal__primary').exists()).toBe(false)
    expect(wrapper.find('.channels-ilink-modal__link').attributes('href')).toBe(
      'https://ilinkai.weixin.qq.com/connect/scan-session-1'
    )
    expect(listChannelsMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()

    expect(wrapper.find('.channels-ilink-modal').exists()).toBe(false)
    expect(listChannelsMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('.channel-card-stub[data-id="wechat_ilink"]').attributes('data-enabled')).toBe(
      'true'
    )
    expect(wrapper.find('.channel-card-stub[data-id="wechat_ilink"]').attributes('data-status')).toBe(
      'connected'
    )

    wrapper.unmount()
  })

  it('keeps the latest WeChat iLink setup status when older poll responses resolve later', async () => {
    vi.useFakeTimers()
    createWeChatILinkSetupSessionMock.mockResolvedValue({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'pending',
        scan_url: 'https://ilinkai.weixin.qq.com/connect/scan-session-1',
        mobile_url: 'https://ilinkai.weixin.qq.com/connect/scan-session-1',
        expires_at: '2026-04-14T10:10:00Z',
      },
    })

    let resolveFirstPoll:
      | ((value: { status: number; data: Record<string, unknown> }) => void)
      | null = null

    getWeChatILinkSetupSessionMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFirstPoll = resolve
        })
    )
    getWeChatILinkSetupSessionMock.mockResolvedValueOnce({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'connected',
        message: 'configured',
        expires_at: '2026-04-14T10:10:00Z',
      },
    })
    listChannelsMock.mockResolvedValue({
      status: 200,
      data: {
        channels: [
          {
            id: 'wechat_ilink',
            enabled: true,
            status: 'connected',
            config: {},
          },
        ],
      },
    })
    getChannelSettingsMock.mockResolvedValue({
      status: 200,
      data: {},
    })

    const ChannelsView = (await import('@/views/ChannelsView.vue')).default
    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const loadMoreButton = wrapper.find('.channels-load-more')
    if (loadMoreButton.exists()) {
      await loadMoreButton.trigger('click')
      await flushPromises()
    }

    const wechatILinkCard = wrapper.find('.channel-card-stub[data-id="wechat_ilink"]')
    expect(wechatILinkCard.exists()).toBe(true)
    await wechatILinkCard.find('.channel-card-select-stub').trigger('click')
    await flushPromises()

    await wrapper.find('.channels-ilink-setup-card__primary').trigger('click')
    await flushPromises()

    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()
    expect(getWeChatILinkSetupSessionMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()
    expect(getWeChatILinkSetupSessionMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('.channels-ilink-modal').exists()).toBe(false)
    expect(wrapper.find('.channel-card-stub[data-id="wechat_ilink"]').attributes('data-enabled')).toBe(
      'true'
    )
    expect(wrapper.find('.channel-card-stub[data-id="wechat_ilink"]').attributes('data-status')).toBe(
      'connected'
    )

    expect(resolveFirstPoll).not.toBeNull()
    resolveFirstPoll?.({
      status: 200,
      data: {
        session_id: 'session-1',
        status: 'configuring',
        expires_at: '2026-04-14T10:10:00Z',
      },
    })
    await flushPromises()

    expect(wrapper.find('.channels-ilink-modal').exists()).toBe(false)
    expect(wrapper.find('.channel-card-stub[data-id="wechat_ilink"]').attributes('data-enabled')).toBe(
      'true'
    )
    expect(wrapper.find('.channel-card-stub[data-id="wechat_ilink"]').attributes('data-status')).toBe(
      'connected'
    )

    wrapper.unmount()
  })

  it('shows the backend error when WeChat iLink setup creation fails', async () => {
    createWeChatILinkSetupSessionMock.mockRejectedValue({
      isAxiosError: true,
      response: {
        status: 503,
        data: {
          error: 'tunnel runtime not available',
        },
      },
    })
    listChannelsMock.mockResolvedValue({
      status: 200,
      data: {
        channels: [
          {
            id: 'wechat_ilink',
            enabled: false,
            status: 'disconnected',
            config: {},
          },
        ],
      },
    })
    getChannelSettingsMock.mockResolvedValue({
      status: 200,
      data: {},
    })

    const ChannelsView = (await import('@/views/ChannelsView.vue')).default
    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const loadMoreButton = wrapper.find('.channels-load-more')
    if (loadMoreButton.exists()) {
      await loadMoreButton.trigger('click')
      await flushPromises()
    }

    const wechatCard = wrapper.find('.channel-card-stub[data-id="wechat"]')
    if (wechatCard.exists()) {
      await wechatCard.find('.channel-card-select-stub').trigger('click')
      await flushPromises()
    }

    const wechatILinkCard = wrapper.find('.channel-card-stub[data-id="wechat_ilink"]')
    expect(wechatILinkCard.exists()).toBe(true)
    await wechatILinkCard.find('.channel-card-select-stub').trigger('click')
    await flushPromises()

    const startButton = wrapper.find('.channels-ilink-setup-card__primary')
    expect(startButton.exists()).toBe(true)
    await startButton.trigger('click')
    await flushPromises()

    expect(createWeChatILinkSetupSessionMock).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.channels-ilink-modal__error').text()).toContain(
      'tunnel runtime not available'
    )
    expect(wrapper.find('.channels-ilink-modal__status').exists()).toBe(false)

    const retryButton = wrapper.find('.channels-ilink-modal__primary')
    expect(retryButton.exists()).toBe(true)
    expect(retryButton.attributes('disabled')).toBeUndefined()
  })

  it('shows the backend message when WeChat iLink setup creation returns a non-2xx response', async () => {
    createWeChatILinkSetupSessionMock.mockResolvedValue({
      status: 502,
      data: {
        message: 'get_bot_qrcode: http 404: not found',
      },
    })
    listChannelsMock.mockResolvedValue({
      status: 200,
      data: {
        channels: [
          {
            id: 'wechat_ilink',
            enabled: false,
            status: 'disconnected',
            config: {},
          },
        ],
      },
    })
    getChannelSettingsMock.mockResolvedValue({
      status: 200,
      data: {},
    })

    const ChannelsView = (await import('@/views/ChannelsView.vue')).default
    const wrapper = mount(ChannelsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    const loadMoreButton = wrapper.find('.channels-load-more')
    if (loadMoreButton.exists()) {
      await loadMoreButton.trigger('click')
      await flushPromises()
    }

    const wechatILinkCard = wrapper.find('.channel-card-stub[data-id="wechat_ilink"]')
    expect(wechatILinkCard.exists()).toBe(true)
    await wechatILinkCard.find('.channel-card-select-stub').trigger('click')
    await flushPromises()

    const startButton = wrapper.find('.channels-ilink-setup-card__primary')
    expect(startButton.exists()).toBe(true)
    await startButton.trigger('click')
    await flushPromises()

    expect(wrapper.find('.channels-ilink-modal__error').text()).toContain(
      'get_bot_qrcode: http 404: not found'
    )
  })
})
