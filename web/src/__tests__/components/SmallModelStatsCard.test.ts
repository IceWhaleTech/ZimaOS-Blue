import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SmallModelStatsCard from '@/components/dashboard/cards/SmallModelStatsCard.vue'
import { i18n } from '@/i18n'
import { settingsApi } from '@/api/settings'

vi.mock('@/api/settings', () => ({
  settingsApi: {
    getSmallModelStats: vi.fn(),
    getSmallModelShadowQuality: vi.fn(),
    getSmallModelShadowQualityGateEval: vi.fn(),
    executeSmallModelShadowAutoRollout: vi.fn(),
  },
}))

describe('SmallModelStatsCard', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders fetched small-model stats', async () => {
    vi.mocked(settingsApi.getSmallModelStats).mockResolvedValue({
      data: {
        short_qa_route_attempts: 20,
        short_qa_route_success: 16,
        tool_dispatch_route_attempts: 10,
        tool_dispatch_route_success: 7,
        summary_attempts: 5,
        summary_success: 4,
        doc_extract_attempts: 4,
        doc_extract_success: 3,
        short_qa_shadow_total: 9,
        tool_dispatch_shadow_total: 5,
        shadow_failures: 1,
        small_model_fallback_total: 6,
        small_model_timeout_total: 2,
        small_model_latency_ms: 18.5,
        small_model_latency_samples: 22,
        short_qa_latency_ms: 9.2,
        short_qa_latency_samples: 8,
        tool_dispatch_latency_ms: 21.4,
        tool_dispatch_latency_samples: 6,
        summary_latency_ms: 14.8,
        summary_latency_samples: 4,
        doc_extract_latency_ms: 32.1,
        doc_extract_latency_samples: 4,
        shadow_quality_delta: 0.125,
        shadow_quality_samples: 8,
        auto_rollback_total: 2,
        no_provider_deepresearch_total: 3,
        ir_takeover_total: 4,
        fallback_reasons: {
          deepresearch_unavailable: 2,
        },
      },
    } as never)
    vi.mocked(settingsApi.getSmallModelShadowQuality).mockResolvedValue({
      data: {
        total: 2,
        average_delta: 0.5,
        samples: [
          {
            scene: 'short_qa_shadow',
            delta: 0.1,
            main_digest: 'main one',
            shadow_digest: 'shadow one',
            created_at: '2026-03-02T00:00:00Z',
          },
          {
            scene: 'tool_dispatch_shadow',
            delta: 0.9,
            main_digest: 'alpha_tool',
            shadow_digest: 'beta_tool',
            created_at: '2026-03-02T00:00:01Z',
          },
        ],
      },
    } as never)
    vi.mocked(settingsApi.getSmallModelShadowQualityGateEval).mockResolvedValue({
      data: {
        overall_pass: false,
        threshold_delta: 0.35,
        min_samples: 40,
        scene_filter: '',
        evaluated_samples: 2,
        scenes: [
          {
            scene: 'short_qa_shadow',
            samples: 1,
            average_delta: 0.1,
            pass: false,
            reason: 'insufficient_samples',
            threshold: 0.35,
            min_samples: 40,
          },
        ],
      },
    } as never)
    vi.mocked(settingsApi.executeSmallModelShadowAutoRollout).mockResolvedValue({
      data: {
        advanced: false,
        reason: 'gate_not_passed',
        current_ratio: 0.1,
        next_ratio: 0.1,
        current_percent: 10,
        next_percent: 10,
        gate_eval: {
          overall_pass: false,
          threshold_delta: 0.35,
          min_samples: 40,
          scene_filter: '',
          evaluated_samples: 2,
          scenes: [],
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
    expect(settingsApi.getSmallModelShadowQuality).toHaveBeenCalledTimes(1)
    expect(settingsApi.getSmallModelShadowQualityGateEval).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('20')
    expect(wrapper.text()).toContain('80%')
    expect(wrapper.text()).toContain('70%')
    expect(wrapper.text()).toContain('75%')
    expect(wrapper.text()).toContain('4')
    expect(wrapper.text()).toContain('2')
    expect(wrapper.text()).toContain('18.5ms')
    expect(wrapper.text()).toContain('QA 9.2ms')
    expect(wrapper.text()).toContain('Tool 21.4ms')
    expect(wrapper.text()).toContain('Summary 14.8ms')
    expect(wrapper.text()).toContain('Extract 32.1ms')
    expect(wrapper.text()).toContain('Delta 12.5% / 8')
    expect(wrapper.text()).toContain('Shadow Gate Eval')
    expect(wrapper.text()).toContain('HOLD')
    expect(wrapper.text()).toContain('Recent Shadow Samples')
    expect(wrapper.text()).toContain('deepresearch unavailable')

    wrapper.unmount()
  })

  it('sorts fallback reasons by count and supports manual refresh', async () => {
    vi.mocked(settingsApi.getSmallModelStats)
      .mockResolvedValueOnce({
        data: {
          short_qa_route_attempts: 2,
          short_qa_route_success: 1,
          tool_dispatch_route_attempts: 4,
          tool_dispatch_route_success: 3,
          summary_attempts: 0,
          summary_success: 0,
          doc_extract_attempts: 0,
          doc_extract_success: 0,
          short_qa_shadow_total: 0,
          tool_dispatch_shadow_total: 0,
          shadow_failures: 0,
          small_model_fallback_total: 8,
          small_model_timeout_total: 5,
          small_model_latency_ms: 30,
          small_model_latency_samples: 10,
          short_qa_latency_ms: 11,
          short_qa_latency_samples: 4,
          tool_dispatch_latency_ms: 22,
          tool_dispatch_latency_samples: 3,
          summary_latency_ms: 0,
          summary_latency_samples: 0,
          doc_extract_latency_ms: 0,
          doc_extract_latency_samples: 0,
          shadow_quality_delta: 0.9,
          shadow_quality_samples: 10,
          auto_rollback_total: 0,
          no_provider_deepresearch_total: 0,
          ir_takeover_total: 0,
          fallback_reasons: {
            low_confidence: 1,
            timeout: 5,
            model_unready: 2,
          },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          short_qa_route_attempts: 2,
          short_qa_route_success: 2,
          tool_dispatch_route_attempts: 4,
          tool_dispatch_route_success: 4,
          summary_attempts: 3,
          summary_success: 3,
          doc_extract_attempts: 2,
          doc_extract_success: 2,
          short_qa_shadow_total: 0,
          tool_dispatch_shadow_total: 0,
          shadow_failures: 0,
          small_model_fallback_total: 1,
          small_model_timeout_total: 1,
          small_model_latency_ms: 12.3,
          small_model_latency_samples: 7,
          short_qa_latency_ms: 10,
          short_qa_latency_samples: 3,
          tool_dispatch_latency_ms: 14.6,
          tool_dispatch_latency_samples: 2,
          summary_latency_ms: 8.5,
          summary_latency_samples: 1,
          doc_extract_latency_ms: 18.2,
          doc_extract_latency_samples: 1,
          shadow_quality_delta: 0.2,
          shadow_quality_samples: 5,
          auto_rollback_total: 1,
          no_provider_deepresearch_total: 0,
          ir_takeover_total: 1,
          fallback_reasons: {
            timeout: 1,
          },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          short_qa_route_attempts: 2,
          short_qa_route_success: 2,
          tool_dispatch_route_attempts: 4,
          tool_dispatch_route_success: 4,
          summary_attempts: 3,
          summary_success: 3,
          doc_extract_attempts: 2,
          doc_extract_success: 2,
          short_qa_shadow_total: 0,
          tool_dispatch_shadow_total: 0,
          shadow_failures: 0,
          small_model_fallback_total: 1,
          small_model_timeout_total: 1,
          small_model_latency_ms: 12.3,
          small_model_latency_samples: 7,
          short_qa_latency_ms: 10,
          short_qa_latency_samples: 3,
          tool_dispatch_latency_ms: 14.6,
          tool_dispatch_latency_samples: 2,
          summary_latency_ms: 8.5,
          summary_latency_samples: 1,
          doc_extract_latency_ms: 18.2,
          doc_extract_latency_samples: 1,
          shadow_quality_delta: 0.2,
          shadow_quality_samples: 5,
          auto_rollback_total: 1,
          no_provider_deepresearch_total: 0,
          ir_takeover_total: 1,
          fallback_reasons: {
            timeout: 1,
          },
        },
      } as never)
    vi.mocked(settingsApi.getSmallModelShadowQuality)
      .mockResolvedValueOnce({
        data: {
          total: 1,
          average_delta: 0.9,
          samples: [
            {
              scene: 'short_qa_shadow',
              delta: 0.9,
              main_digest: 'a',
              shadow_digest: 'b',
              created_at: '2026-03-02T00:00:02Z',
            },
          ],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          total: 1,
          average_delta: 0.2,
          samples: [
            {
              scene: 'short_qa_shadow',
              delta: 0.2,
              main_digest: 'c',
              shadow_digest: 'c',
              created_at: '2026-03-02T00:00:03Z',
            },
          ],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          total: 1,
          average_delta: 0.2,
          samples: [
            {
              scene: 'short_qa_shadow',
              delta: 0.2,
              main_digest: 'c',
              shadow_digest: 'c',
              created_at: '2026-03-02T00:00:03Z',
            },
          ],
        },
      } as never)
    vi.mocked(settingsApi.getSmallModelShadowQualityGateEval)
      .mockResolvedValueOnce({
        data: {
          overall_pass: false,
          threshold_delta: 0.35,
          min_samples: 40,
          scene_filter: '',
          evaluated_samples: 1,
          scenes: [],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          overall_pass: true,
          threshold_delta: 0.35,
          min_samples: 40,
          scene_filter: '',
          evaluated_samples: 1,
          scenes: [],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          overall_pass: true,
          threshold_delta: 0.35,
          min_samples: 40,
          scene_filter: '',
          evaluated_samples: 1,
          scenes: [],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          overall_pass: true,
          threshold_delta: 0.35,
          min_samples: 40,
          scene_filter: '',
          evaluated_samples: 1,
          scenes: [],
        },
      } as never)
    vi.mocked(settingsApi.executeSmallModelShadowAutoRollout).mockResolvedValueOnce({
      data: {
        advanced: true,
        reason: 'advanced',
        current_ratio: 0.1,
        next_ratio: 0.3,
        current_percent: 10,
        next_percent: 30,
        gate_eval: {
          overall_pass: true,
          threshold_delta: 0.35,
          min_samples: 40,
          scene_filter: '',
          evaluated_samples: 1,
          scenes: [],
        },
      },
    } as never)

    const wrapper = mount(SmallModelStatsCard, {
      global: {
        plugins: [i18n],
      },
    })
    await flushPromises()

    const textBefore = wrapper.text()
    expect(textBefore.indexOf('timeout')).toBeLessThan(textBefore.indexOf('model unready'))
    expect(settingsApi.getSmallModelStats).toHaveBeenCalledTimes(1)
    expect(settingsApi.getSmallModelShadowQuality).toHaveBeenCalledTimes(1)
    expect(settingsApi.getSmallModelShadowQualityGateEval).toHaveBeenCalledTimes(1)

    const refresh = wrapper.find('button')
    await refresh.trigger('click')
    await flushPromises()
    expect(settingsApi.getSmallModelStats).toHaveBeenCalledTimes(2)
    expect(settingsApi.getSmallModelShadowQuality).toHaveBeenCalledTimes(2)
    expect(settingsApi.getSmallModelShadowQualityGateEval).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('100%')
    expect(wrapper.text()).toContain('PASS')

    await wrapper.get('[data-testid="small-model-auto-rollout-exec"]').trigger('click')
    await flushPromises()
    expect(settingsApi.executeSmallModelShadowAutoRollout).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Rollout 10% -> 30%')

    wrapper.unmount()
  })
})
