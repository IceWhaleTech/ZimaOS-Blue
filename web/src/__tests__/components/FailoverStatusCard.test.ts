import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import FailoverStatusCard from '@/components/dashboard/cards/FailoverStatusCard.vue'
import { i18n, setLocale } from '@/i18n'
import { providerPoolApi } from '@/api/providerPool'

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    getFailoverOverview: vi.fn(),
    getFailoverMetrics: vi.fn(),
    getFailoverConfig: vi.fn(),
    getCircuitBreakerStatus: vi.fn(),
  },
}))

const localStorageMock = {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}

vi.stubGlobal('localStorage', localStorageMock)

describe('FailoverStatusCard', () => {
  beforeEach(async () => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    await setLocale('en-US')
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('hydrates from failover overview without calling the legacy sparse endpoints', async () => {
    vi.mocked(providerPoolApi.getFailoverOverview).mockResolvedValue({
      data: {
        metrics: {
          errors_by_type: { timeout: 2, rate_limited: 1 },
          failover_total: 4,
          failover_success: 3,
          failover_failure: 1,
          provider_errors: {
            'provider-a': { timeout: 2 },
          },
          provider_failovers: {
            'provider-a': 3,
          },
          stream_anomalies: 2,
        },
        config: {
          enabled: true,
          max_retries: 2,
          retry_delay: '1s',
          circuit_breaker: true,
          failure_threshold: 3,
          recovery_timeout: '30s',
          error_classification: { enabled: true },
          streaming_anomaly: { enabled: true },
          provider_race: { enabled: false },
        },
        circuit_breakers: {
          'provider-a': {
            state: 'open',
            failures: 2,
            last_failure: '2026-04-05T00:00:00Z',
          },
        },
      },
    } as never)

    const wrapper = mount(FailoverStatusCard, {
      global: {
        plugins: [i18n],
      },
    })
    await flushPromises()

    expect(providerPoolApi.getFailoverOverview).toHaveBeenCalledTimes(1)
    expect(providerPoolApi.getFailoverMetrics).not.toHaveBeenCalled()
    expect(providerPoolApi.getFailoverConfig).not.toHaveBeenCalled()
    expect(providerPoolApi.getCircuitBreakerStatus).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('75%')
    expect(wrapper.text()).toContain('4')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('2')
    expect(wrapper.text()).toContain('1 Open')
  })
})
