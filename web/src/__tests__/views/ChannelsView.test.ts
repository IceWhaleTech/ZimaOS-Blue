import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const settingsStoreMock = {
  backendSettings: {},
  fetchBackendSettings: vi.fn().mockResolvedValue(undefined),
}

const authFetchMock = vi.fn()
const getTunnelProvidersMock = vi.fn()
const getRemoteAccessConfigMock = vi.fn()
const getRemoteAccessStatusMock = vi.fn()

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsStoreMock,
}))

vi.mock('@/api/client', () => ({
  authFetch: authFetchMock,
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
    template: '<div class="channel-card-stub">{{ channel.id }}</div>',
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
        },
        common: {
          loading: 'Loading',
          save: 'Save',
          loadMore: 'Load More',
          optional: 'Optional',
          retry: 'Retry',
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

    authFetchMock.mockResolvedValue({
      ok: false,
      status: 401,
      json: vi.fn().mockResolvedValue({}),
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
})
