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
