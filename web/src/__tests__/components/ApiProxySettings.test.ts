import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { getFailoverConfig, getFailoverOverview, updateFailoverConfig } = vi.hoisted(() => ({
  getFailoverConfig: vi.fn(),
  getFailoverOverview: vi.fn(),
  updateFailoverConfig: vi.fn(),
}))

vi.mock('@/api/proxy', () => ({
  proxyApi: {
    getFailoverConfig,
    getFailoverOverview,
    updateFailoverConfig,
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
    getFailoverOverview.mockResolvedValue({
      data: {
        config: {
          enabled: true,
          provider_race: {
            enabled: false,
            max_parallel: 2,
            min_providers: 2,
            empty_rate_sink_threshold: 0.5,
            empty_rate_exclude_threshold: 0.8,
            empty_rate_cooldown_threshold: 0.3,
            empty_rate_min_samples: 10,
            empty_rate_cooldown: 120000000000,
          },
        },
        provider_race: {
          requests_total: 12,
          successful_races: 10,
          hits: 5,
          hit_rate: 0.416,
          avg_winner_latency_ms: 38.4,
          estimated_latency_saved_ms: 17.2,
          estimated_savings_samples: 4,
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

  it('renders provider race controls and toggles the provider race flag', async () => {
    const wrapper = mount(ApiProxySettings, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    const toggle = wrapper.find('[data-testid="provider-race-toggle"]')
    expect(toggle.exists()).toBe(true)

    await toggle.trigger('click')

    expect(updateFailoverConfig).toHaveBeenCalledWith({
      provider_race: {
        enabled: true,
      },
    })
  })

  it('renders provider race effectiveness stats from failover overview', async () => {
    const wrapper = mount(ApiProxySettings, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(getFailoverOverview).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="provider-race-stats"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('42%')
    expect(wrapper.text()).toContain('17 ms')
    expect(wrapper.text()).toContain('38 ms')
    expect(wrapper.text()).toContain('12')
  })
})
