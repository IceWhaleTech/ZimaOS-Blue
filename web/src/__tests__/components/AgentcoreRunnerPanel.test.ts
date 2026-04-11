import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import AgentcoreRunnerPanel from '@/components/harness/AgentcoreRunnerPanel.vue'
import { settingsApi } from '@/api/settings'

vi.mock('@/api/settings', () => ({
  settingsApi: {
    getAgentcoreRunnerStatus: vi.fn(),
    getAgentcoreRunnerLastRun: vi.fn(),
    getAgentcoreRunnerTags: vi.fn(),
    patch: vi.fn(),
    prepareAgentcoreRunner: vi.fn(),
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
        common: {
          refresh: 'Refresh',
          yes: 'Yes',
          no: 'No',
        },
      },
    },
  })
}

describe('AgentcoreRunnerPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const pinia = createPinia()
    setActivePinia(pinia)

    vi.mocked(settingsApi.getAgentcoreRunnerStatus).mockResolvedValue({
      data: {
        enabled: true,
        repo_url: 'https://github.com/example/runner',
        resolved_ref: 'main',
        resolved_commit: 'abc123',
        required_go_version: '1.24.0',
        installed_go_version: '1.24.0',
        toolchain_ready: true,
        binary_ready: true,
        binary_path: '/tmp/agentcore-runner',
        binary_sha256: 'deadbeef',
        last_prepare_at: '2026-04-03T12:00:00Z',
        last_prepare_state: 'ready',
        last_error: '',
        last_optimization_run_id: 'opt-123',
        last_optimization_at: '2026-04-03T12:05:00Z',
        last_optimization_state: 'completed',
        last_optimization_summary: 'agentcore runner summary',
        supported_parts: ['coordinator_policy', 'orchestrator_policy', 'runner_code'],
        optimized_parts: ['orchestrator_policy'],
        primary_part: 'runner_code',
        source_optimization_run_id: 'opt-source-1',
      },
    } as never)
    vi.mocked(settingsApi.getAgentcoreRunnerLastRun).mockResolvedValue({
      data: {
        id: 'opt-123',
        reason: 'execution_gate_failed',
        candidate_id: 'candidate-123',
        runner_protocol: 'acp',
        runner_stop_reason: 'completed',
        runner_duration_ms: 128,
        runner_transcript: [],
        pareto_frontier: ['candidate-123', 'candidate-456'],
        selected_candidate: {
          candidate_id: 'candidate-123',
          hard_pass: true,
          followup_gate: 'selector',
          followup_state: 'accepted',
          followup_summary: 'Accepted by the selector follow-up gate with the smallest safe diff.',
          diff_size: 14,
          objectives: {
            execution_pass_rate_delta: 0.14,
            verification_pass_rate_delta: 0.09,
            evidence_backed_pass_rate_delta: 0.05,
            median_latency_increase_rate: 0.06,
            repeat_failure_recurrence: 0.08,
          },
        },
        evaluated_candidates: [
          {
            candidate_id: 'candidate-123',
            hard_pass: true,
            followup_gate: 'selector',
            followup_state: 'accepted',
            followup_summary: 'Accepted by the selector follow-up gate with the smallest safe diff.',
            diff_size: 14,
            objectives: {
              execution_pass_rate_delta: 0.14,
              verification_pass_rate_delta: 0.09,
              evidence_backed_pass_rate_delta: 0.05,
              median_latency_increase_rate: 0.06,
              repeat_failure_recurrence: 0.08,
            },
          },
          {
            candidate_id: 'candidate-456',
            hard_pass: true,
            followup_gate: 'selector',
            followup_state: 'accepted',
            followup_summary: 'Higher verification gain, but the latency tradeoff is larger.',
            diff_size: 19,
            objectives: {
              execution_pass_rate_delta: 0.12,
              verification_pass_rate_delta: 0.1,
              evidence_backed_pass_rate_delta: 0.07,
              median_latency_increase_rate: 0.09,
              repeat_failure_recurrence: 0.05,
            },
          },
          {
            candidate_id: 'candidate-789',
            hard_pass: false,
            followup_gate: 'critical',
            followup_state: 'rejected',
            followup_summary: 'Execution improved, but evidence-backed verification regressed.',
            diff_size: 27,
            objectives: {
              execution_pass_rate_delta: 0.11,
              verification_pass_rate_delta: -0.02,
              evidence_backed_pass_rate_delta: -0.04,
              median_latency_increase_rate: 0.04,
              repeat_failure_recurrence: 0.12,
            },
          },
        ],
        offline_value_report: {
          offline_recommendation: 'hold',
          value_summary: 'Execution improved, but cutover readiness still needs more evidence.',
          top_improvements: ['Verification pass rate improved by 0.10'],
          top_tradeoffs: ['Cutover readiness is not green yet'],
          confidence: 'medium',
        },
        runtime_value_report: {
          status: 'provisional',
          value_summary: 'Promoted candidate has fewer than 20 post-promotion runtime samples.',
          before_sample_count: 20,
          after_sample_count: 10,
          failure_recurrence_delta: -0.3,
          median_duration_delta_rate: 0.25,
          median_total_tokens_delta_rate: 0.18,
          capture_quality_delta: 0.12,
          validation_quality_delta: -0.08,
          top_improvements: ['Repeat failure recurrence improved by 0.30'],
          top_tradeoffs: ['Median token usage regressed by 18%'],
          confidence: 'low',
        },
        proposal_set: [
          { candidate_id: 'candidate-123', generation: 1 },
          { candidate_id: 'candidate-456', generation: 1 },
          { candidate_id: 'candidate-789', generation: 1 },
        ],
        sample_efficiency_report: {
          proposal_count: 3,
          evaluated_candidate_count: 3,
          hard_pass_candidate_count: 2,
          pareto_frontier_size: 2,
        },
      },
    } as never)
    vi.mocked(settingsApi.getAgentcoreRunnerTags).mockResolvedValue({
      data: {
        repo_url: 'https://github.com/IceWhaleTech/ZimaOS-Blue',
        default_ref: 'main',
        tags: ['v0.10.37'],
      },
    } as never)
    vi.mocked(settingsApi.patch).mockImplementation(async (payload: unknown) => ({ data: payload }) as never)
    vi.mocked(settingsApi.prepareAgentcoreRunner).mockResolvedValue({
      data: {
        enabled: true,
      },
    } as never)
  })

  it('renders the runner card and hides the manual refresh button when requested', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(AgentcoreRunnerPanel, {
      props: {
        showRefreshButton: false,
      },
      global: {
        plugins: [pinia, createTestI18n()],
      },
    })

    await flushPromises()
    expect(wrapper.find('[data-testid="agentcore-runner-source-content"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="agentcore-runner-source-toggle"]').text()).toContain(
      'GitHub Repo & Ref'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-source-toggle"]').text()).toContain(
      'IceWhaleTech/ZimaOS-Blue'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-source-toggle"]').text()).toContain('main')
    await wrapper.get('[data-testid="agentcore-runner-source-toggle"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="agentcore-runner-source-content"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="agentcore-runner-repo-input"]').element).toBeTruthy()
    expect(wrapper.get('[data-testid="agentcore-runner-ref-input"]').element).toBeTruthy()
    await wrapper.get('[data-testid="agentcore-runner-status-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="agentcore-runner-card"]').text()).toContain('Agentcore Runner')
    expect(wrapper.find('[data-testid="agentcore-runner-refresh"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="agentcore-runner-prepare"]').text()).toContain(
      'Prepare Runner'
    )
    expect(wrapper.text()).toContain('Coordinator')
    expect(wrapper.text()).toContain('Orchestrator')
    expect(wrapper.text()).toContain('Runner code')
    expect(wrapper.get('[data-testid="agentcore-runner-primary-part"]').text()).toContain(
      'Primary: Runner code'
    )
    expect(
      wrapper
        .get('[data-testid="agentcore-runner-evolvable-part"][data-part="orchestrator_policy"]')
        .attributes('title')
    ).toContain('Governs multi-step flow control, retries, and cross-stage coordination.')
    expect(
      wrapper
        .get('[data-testid="agentcore-runner-evolvable-part"][data-part="orchestrator_policy"]')
        .attributes('title')
    ).toContain('This part was optimized in the current candidate')
    expect(wrapper.text()).toContain('Selected candidate: candidate-123')
    expect(wrapper.text()).toContain('candidate-456')
    expect(wrapper.text()).toContain('Hard pass')
    expect(wrapper.text()).toContain('Gate: selector')
    expect(wrapper.text()).toContain('State: accepted')
    expect(wrapper.text()).toContain('Diff size: 14')
    expect(wrapper.text()).toContain('Selection basis')
    expect(wrapper.text()).toContain('Execution pass rate')
    expect(wrapper.text()).toContain('+0.14')
    expect(wrapper.text()).toContain('Verification pass rate')
    expect(wrapper.text()).toContain('+0.09')
    expect(wrapper.text()).toContain('Evidence-backed pass rate')
    expect(wrapper.text()).toContain('+0.05')
    expect(wrapper.text()).toContain('Latency increase')
    expect(wrapper.text()).toContain('+6%')
    expect(wrapper.text()).toContain('Repeat failure recurrence')
    expect(wrapper.text()).toContain('0.08')
    expect(wrapper.text()).toContain('Higher is better')
    expect(wrapper.text()).toContain('Lower is better')
    expect(wrapper.text()).toContain('Frontier candidates')
    expect(wrapper.text()).toContain('Evaluated candidates')
    expect(wrapper.text()).toContain('Exec +0.12')
    expect(wrapper.text()).toContain('Verify +0.10')
    expect(wrapper.text()).toContain('Latency +9%')
    expect(wrapper.text()).toContain('Accepted by the selector follow-up gate with the smallest safe diff.')
    expect(wrapper.text()).toContain('Higher verification gain, but the latency tradeoff is larger.')
    expect(wrapper.text()).toContain('Execution improved, but evidence-backed verification regressed.')
    expect(wrapper.text()).toContain('State: rejected')
    expect(wrapper.text()).toContain('Did not hard pass')
    expect(wrapper.text()).toContain('Diff size: 27')
    expect(wrapper.text()).toContain('Objective vector')
    expect(wrapper.text()).toContain('Follow-up outcome')
    expect(wrapper.text()).toContain('Execution improved, but cutover readiness still needs more evidence.')
    expect(wrapper.text()).toContain('Verification pass rate improved by 0.10')
    expect(wrapper.text()).toContain('Cutover readiness is not green yet')
    expect(wrapper.text()).toContain('Evaluated 3 candidates')
    expect(wrapper.text()).toContain('Generated 3 proposals')
    expect(wrapper.text()).toContain('2 hard-pass')
    expect(wrapper.text()).toContain('frontier 2')
    expect(wrapper.text()).toContain('Promoted candidate has fewer than 20 post-promotion runtime samples.')
    expect(wrapper.text()).toContain('Recommendation: hold')
    expect(wrapper.text()).toContain('Confidence: medium')
    expect(wrapper.text()).toContain('Status: provisional')
    expect(wrapper.text()).toContain('20 before')
    expect(wrapper.text()).toContain('10 after')
    expect(wrapper.text()).toContain('Failure recurrence')
    expect(wrapper.text()).toContain('-0.30')
    expect(wrapper.text()).toContain('Duration')
    expect(wrapper.text()).toContain('+25%')
    expect(wrapper.text()).toContain('Token usage')
    expect(wrapper.text()).toContain('+18%')
    expect(wrapper.text()).toContain('Capture quality')
    expect(wrapper.text()).toContain('+0.12')
    expect(wrapper.text()).toContain('Validation quality')
    expect(wrapper.text()).toContain('-0.08')
    expect(wrapper.text()).toContain('Repeat failure recurrence improved by 0.30')
    expect(wrapper.text()).toContain('Median token usage regressed by 18%')
  })

  it('renders a flatter embedded variant for evolution surfaces', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(AgentcoreRunnerPanel, {
      props: {
        embedded: true,
      },
      global: {
        plugins: [pinia, createTestI18n()],
      },
    })

    await flushPromises()

    const card = wrapper.get('[data-testid="agentcore-runner-card"]')
    expect(card.classes()).toContain('agentcore-runner-panel--embedded')
    expect(card.classes()).not.toContain('dashboard-card-surface')
    expect(wrapper.find('.agentcore-runner-panel__title').exists()).toBe(false)
    expect(wrapper.find('.dashboard-card-subsurface').exists()).toBe(false)
  })
})
