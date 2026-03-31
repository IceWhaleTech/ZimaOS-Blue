import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

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

vi.mock('@/components/channels/ChannelCard.vue', () => ({
  default: {
    name: 'ChannelCard',
    props: ['channel'],
    template:
      '<div class="channel-card-stub" :data-id="channel.id" :data-last-field="channel.fields?.[channel.fields.length - 1]?.key" :data-last-value="channel.fields?.[channel.fields.length - 1]?.value">{{ channel.id }}<button class="channel-card-update-stub" @click="$emit(\'update-field\', channel.fields.length - 1, \'true\')">update</button><button class="channel-card-save-stub" @click="$emit(\'save\')">save</button></div>',
  },
}))

vi.mock('@/components/remote-access/TunnelStatus.vue', () => ({
  default: {
    name: 'TunnelStatus',
    template: '<div class="tunnel-status-stub"></div>',
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
        },
        common: {
          loading: 'Loading',
          save: 'Save',
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
    expect(wrapper.findAll('channel-card-stub').length).toBeGreaterThan(0)
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

  it('saves Feishu session mode through the channel config API', async () => {
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
    expect(feishuCard.attributes('data-last-field')).toBe('session_mode')
    expect(feishuCard.attributes('data-last-value')).toBe('false')

    await feishuCard.find('.channel-card-update-stub').trigger('click')
    await flushPromises()

    expect(wrapper.find('.channel-card-stub[data-id="feishu"]').attributes('data-last-value')).toBe(
      'true'
    )

    await feishuCard.find('.channel-card-save-stub').trigger('click')
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
})
