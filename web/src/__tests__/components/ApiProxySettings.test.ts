import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { getFailoverConfig, updateFailoverConfig, getSettings, getToolStats } = vi.hoisted(() => ({
  getFailoverConfig: vi.fn(),
  updateFailoverConfig: vi.fn(),
  getSettings: vi.fn(),
  getToolStats: vi.fn(),
}))

vi.mock('@/api/proxy', () => ({
  proxyApi: {
    getFailoverConfig,
    updateFailoverConfig,
  },
}))

vi.mock('@/api/settings', () => ({
  settingsApi: {
    get: getSettings,
    getToolStats,
  },
}))

import { i18n } from '@/i18n'
import ApiProxySettings from '@/components/settings/ApiProxySettings.vue'

describe('ApiProxySettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(i18n.global as { locale: { value: string } }).locale.value = 'en-US'

    getFailoverConfig.mockResolvedValue({
      data: {
        enabled: true,
        provider_race: {
          enabled: false,
        },
      },
    })
    updateFailoverConfig.mockResolvedValue({
      data: {
        enabled: true,
        provider_race: {
          enabled: true,
        },
      },
    })
    getSettings.mockResolvedValue({
      data: {
        smart_tool_selection: false,
      },
    })
    getToolStats.mockResolvedValue({
      data: {
        requests: 0,
        tools_skipped: 0,
        tokens_saved: 0,
      },
    })
  })

  it('renders the failover capability grid from proxy config', async () => {
    const wrapper = mount(ApiProxySettings, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('circuit_breaker')
    expect(wrapper.text()).toContain('context_window_check')
    expect(wrapper.text()).toContain('error_classification')
    expect(wrapper.text()).toContain('streaming_anomaly')
  })

  it('no longer renders the global pruner switch in ApiProxySettings', async () => {
    const wrapper = mount(ApiProxySettings, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="proxy-pruner-switch"]').exists()).toBe(false)
  })
})
