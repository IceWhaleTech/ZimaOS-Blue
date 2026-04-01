import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SmallModelStatsCard from '@/components/dashboard/cards/SmallModelStatsCard.vue'
import { i18n, setLocale } from '@/i18n'
import { settingsApi } from '@/api/settings'

vi.mock('@/api/settings', () => ({
  settingsApi: {
    getSmallModelStats: vi.fn(),
  },
}))

const localStorageMock = {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}

vi.stubGlobal('localStorage', localStorageMock)

describe('SmallModelStatsCard', () => {
  beforeEach(async () => {
    vi.resetAllMocks()
    await setLocale('en-US')
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders stats, localizes fallback reason labels, and supports manual refresh', async () => {
    vi.mocked(settingsApi.getSmallModelStats)
      .mockResolvedValueOnce({
        data: {
          short_qa_route_attempts: 20,
          short_qa_route_success: 16,
          summary_attempts: 5,
          summary_success: 4,
          doc_extract_attempts: 4,
          doc_extract_success: 3,
          small_model_fallback_total: 6,
          small_model_timeout_total: 2,
          small_model_latency_ms: 18.5,
          short_qa_latency_ms: 9.2,
          summary_latency_ms: 14.8,
          doc_extract_latency_ms: 32.1,
          auto_rollback_total: 2,
          no_provider_deepresearch_total: 3,
          ir_takeover_total: 4,
          fallback_reasons: {
            timeout: 5,
            model_unready: 2,
            deepresearch_unavailable: 3,
          },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          short_qa_route_attempts: 20,
          short_qa_route_success: 20,
          summary_attempts: 5,
          summary_success: 5,
          doc_extract_attempts: 4,
          doc_extract_success: 4,
          small_model_fallback_total: 1,
          small_model_timeout_total: 1,
          small_model_latency_ms: 12.3,
          short_qa_latency_ms: 8.1,
          summary_latency_ms: 7.0,
          doc_extract_latency_ms: 15.7,
          auto_rollback_total: 0,
          no_provider_deepresearch_total: 0,
          ir_takeover_total: 1,
          fallback_reasons: {
            timeout: 1,
          },
        },
      } as never)

    const wrapper = mount(SmallModelStatsCard, {
      global: {
        plugins: [i18n],
      },
    })
    await flushPromises()

    expect(settingsApi.getSmallModelStats).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('20')
    expect(wrapper.text()).toContain('80%')
    expect(wrapper.text()).toContain('80%')
    expect(wrapper.text()).toContain('75%')
    expect(wrapper.text()).toContain('18.5ms')
    expect(wrapper.text()).toContain(
      i18n.global.t('settings.smallModel.fallbackReasonLabels.deepresearch_unavailable')
    )
    expect(wrapper.text().toLowerCase()).not.toContain('shadow')
    expect(wrapper.text()).not.toContain('deepresearch_unavailable')
    expect(wrapper.text()).not.toContain('model_unready')

    const fallbackChips = wrapper.findAll('.dashboard-card-chip').slice(1)
    expect(fallbackChips).toHaveLength(3)
    expect(fallbackChips[0]?.text()).toContain(
      i18n.global.t('settings.smallModel.fallbackReasonLabels.timeout')
    )
    expect(fallbackChips[1]?.text()).toContain(
      i18n.global.t('settings.smallModel.fallbackReasonLabels.deepresearch_unavailable')
    )
    expect(fallbackChips[2]?.text()).toContain(
      i18n.global.t('settings.smallModel.fallbackReasonLabels.model_unready')
    )

    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(settingsApi.getSmallModelStats).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('100%')

    wrapper.unmount()
  })

  it('auto-refreshes every 30 seconds and stops after unmount', async () => {
    vi.mocked(settingsApi.getSmallModelStats).mockResolvedValue({
      data: {
        short_qa_route_attempts: 1,
        short_qa_route_success: 1,
        summary_attempts: 1,
        summary_success: 1,
        doc_extract_attempts: 1,
        doc_extract_success: 1,
        small_model_fallback_total: 0,
        small_model_timeout_total: 0,
        small_model_latency_ms: 1,
        short_qa_latency_ms: 1,
        summary_latency_ms: 1,
        doc_extract_latency_ms: 1,
        auto_rollback_total: 0,
        no_provider_deepresearch_total: 0,
        ir_takeover_total: 0,
        fallback_reasons: {},
      },
    } as never)

    const wrapper = mount(SmallModelStatsCard, {
      global: {
        plugins: [i18n],
      },
    })
    await flushPromises()
    expect(settingsApi.getSmallModelStats).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(30000)
    await flushPromises()
    expect(settingsApi.getSmallModelStats).toHaveBeenCalledTimes(2)

    wrapper.unmount()
    vi.advanceTimersByTime(60000)
    await flushPromises()
    expect(settingsApi.getSmallModelStats).toHaveBeenCalledTimes(2)
  })
})
