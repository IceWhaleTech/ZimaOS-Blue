import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const listGroupsMock = vi.fn()
const listDatasetsMock = vi.fn()
const listDatasetVersionsMock = vi.fn()
const listEvalSpecsMock = vi.fn()
const listEvalRunsMock = vi.fn()
const listBaselinesMock = vi.fn()
const getEvalRunReportMock = vi.fn()
const createDatasetMock = vi.fn()
const createDatasetVersionMock = vi.fn()
const createEvalSpecMock = vi.fn()
const createEvalRunMock = vi.fn()
const createGroupMock = vi.fn()
const createBaselineMock = vi.fn()
const compareEvalRunMock = vi.fn()
const cancelEvalRunMock = vi.fn()
const getGroupReportMock = vi.fn()
const cancelGroupMock = vi.fn()
const retryFailedGroupMock = vi.fn()
const promoteGroupMock = vi.fn()
const listConversationsMock = vi.fn()
const getConversationMock = vi.fn()
const listMessagesMock = vi.fn()
const notificationSuccessMock = vi.fn()
const notificationInfoMock = vi.fn()
const notificationErrorMock = vi.fn()
const routerPushMock = vi.fn()

vi.mock('@/api/harness', () => ({
  harnessApi: {
    listGroups: listGroupsMock,
    listDatasets: listDatasetsMock,
    listDatasetVersions: listDatasetVersionsMock,
    listEvalSpecs: listEvalSpecsMock,
    listEvalRuns: listEvalRunsMock,
    listBaselines: listBaselinesMock,
    getEvalRunReport: getEvalRunReportMock,
    createDataset: createDatasetMock,
    createDatasetVersion: createDatasetVersionMock,
    createEvalSpec: createEvalSpecMock,
    createEvalRun: createEvalRunMock,
    createGroup: createGroupMock,
    createBaseline: createBaselineMock,
    compareEvalRun: compareEvalRunMock,
    cancelEvalRun: cancelEvalRunMock,
    getGroupReport: getGroupReportMock,
    cancelGroup: cancelGroupMock,
    retryFailedGroup: retryFailedGroupMock,
    promoteGroup: promoteGroupMock,
  },
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => ({
    success: notificationSuccessMock,
    info: notificationInfoMock,
    error: notificationErrorMock,
  }),
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    list: listConversationsMock,
    get: getConversationMock,
  },
  messageApi: {
    list: listMessagesMock,
  },
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRouter: () => ({
      push: routerPushMock,
    }),
    useRoute: () => ({
      params: { id: 'group-1' },
    }),
  }
})

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
          harness: 'Harness',
        },
        common: {
          loading: 'Loading',
          refresh: 'Refresh',
          error: 'Error',
          back: 'Back',
          all: 'All',
          search: 'Search',
          updatedAt: 'Updated',
          cancel: 'Cancel',
          notAvailable: 'Not available',
        },
      },
    },
  })
}

describe('Harness views', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listGroupsMock.mockResolvedValue({ data: [] })
    listDatasetsMock.mockResolvedValue({ data: [] })
    listDatasetVersionsMock.mockResolvedValue({ data: [] })
    listEvalSpecsMock.mockResolvedValue({ data: [] })
    listEvalRunsMock.mockResolvedValue({ data: [] })
    listBaselinesMock.mockResolvedValue({ data: [] })
    getEvalRunReportMock.mockResolvedValue({ data: null })
    createDatasetMock.mockResolvedValue({ data: null })
    createDatasetVersionMock.mockResolvedValue({ data: null })
    createEvalSpecMock.mockResolvedValue({ data: null })
    createEvalRunMock.mockResolvedValue({ data: null })
    createGroupMock.mockResolvedValue({ data: null })
    createBaselineMock.mockResolvedValue({ data: { id: 'baseline-created' } })
    compareEvalRunMock.mockResolvedValue({ data: null })
    promoteGroupMock.mockResolvedValue({ data: null })
    listConversationsMock.mockResolvedValue({ data: [] })
    getConversationMock.mockResolvedValue({ data: null })
    listMessagesMock.mockResolvedValue({ data: [] })
    routerPushMock.mockReset()
    routerPushMock.mockResolvedValue(undefined)
    notificationSuccessMock.mockReset()
    notificationInfoMock.mockReset()
    notificationErrorMock.mockReset()
  })

  it('renders the harness groups shell with fetched groups', async () => {
    listGroupsMock.mockResolvedValue({
      data: [
        {
          id: 'group-1',
          kind: 'eval',
          title: 'Regression batch',
          status: 'running',
          subject: 'Validate agent task flows',
          summary: {
            item_count: 12,
            pass_rate: 0.75,
            overall_score: 0.82,
            verification_pass_rate: 0.83,
            evidence_backed_pass_rate: 0.71,
            retry_recovered_count: 2,
            failure_label_counts: {
              missing_artifact: 1,
            },
            counts: {
              running: 2,
              passed: 7,
              failed: 1,
            },
          },
          created_at: '2026-03-20T10:00:00Z',
          updated_at: '2026-03-20T10:05:00Z',
        },
      ],
    })
    listDatasetsMock.mockResolvedValue({
      data: [
        {
          id: 'dataset-1',
          name: 'Smoke Dataset',
          subject: 'agent_task',
          default_run_kind: 'agent_task',
          default_profile: 'smoke',
          active_version_id: 'dataset-version-1',
          created_at: '2026-03-20T09:00:00Z',
          updated_at: '2026-03-20T09:05:00Z',
        },
      ],
    })
    listDatasetVersionsMock.mockResolvedValue({
      data: [
        {
          id: 'dataset-version-1',
          dataset_id: 'dataset-1',
          version: 'v1',
          item_count: 1,
          created_at: '2026-03-20T09:10:00Z',
        },
      ],
    })
    listEvalSpecsMock.mockResolvedValue({
      data: [
        {
          id: 'eval-spec-1',
          name: 'Smoke Eval',
          dataset_id: 'dataset-1',
          dataset_version_id: 'dataset-version-1',
          run_kind: 'agent_task',
          profile: 'smoke',
          scoring_config: {
            mode: 'rule',
            pass_threshold: 0.5,
          },
          created_at: '2026-03-20T09:15:00Z',
          updated_at: '2026-03-20T09:15:00Z',
        },
      ],
    })
    listEvalRunsMock.mockResolvedValue({
      data: [
        {
          id: 'eval-run-1',
          eval_spec_id: 'eval-spec-1',
          group_id: 'group-1',
          dataset_version_id: 'dataset-version-1',
          title: 'smoke-run-1',
          status: 'running',
          summary: {
            pass_rate: 0.5,
            overall_score: 0.7,
          },
          created_at: '2026-03-20T09:20:00Z',
          updated_at: '2026-03-20T09:21:00Z',
        },
      ],
    })
    listBaselinesMock.mockResolvedValue({
      data: [
        {
          id: 'baseline-1',
          name: 'Release Baseline',
          eval_spec_id: 'eval-spec-1',
          eval_run_id: 'baseline-run-1',
          is_default: true,
          created_at: '2026-03-20T09:18:00Z',
          updated_at: '2026-03-20T09:18:00Z',
        },
      ],
    })
    getEvalRunReportMock.mockResolvedValue({
      data: {
        eval_run: {
          id: 'eval-run-1',
          eval_spec_id: 'eval-spec-1',
          group_id: 'group-1',
          dataset_version_id: 'dataset-version-1',
          title: 'smoke-run-1',
          status: 'running',
          created_at: '2026-03-20T09:20:00Z',
          updated_at: '2026-03-20T09:21:00Z',
        },
        eval_spec: {
          id: 'eval-spec-1',
          name: 'Smoke Eval',
          dataset_id: 'dataset-1',
          dataset_version_id: 'dataset-version-1',
          run_kind: 'agent_task',
          scoring_config: {
            mode: 'rule',
          },
          created_at: '2026-03-20T09:15:00Z',
          updated_at: '2026-03-20T09:15:00Z',
        },
        dataset: {
          id: 'dataset-1',
          name: 'Smoke Dataset',
          created_at: '2026-03-20T09:00:00Z',
          updated_at: '2026-03-20T09:05:00Z',
        },
        dataset_version: {
          id: 'dataset-version-1',
          dataset_id: 'dataset-1',
          version: 'v1',
          item_count: 1,
          created_at: '2026-03-20T09:10:00Z',
        },
        group_report: {
          group: {
            id: 'group-1',
            kind: 'eval',
            status: 'running',
            summary: {
              item_count: 12,
              verification_pass_rate: 0.83,
              evidence_backed_pass_rate: 0.71,
              retry_recovered_count: 2,
              failure_label_counts: {
                missing_artifact: 1,
              },
            },
            created_at: '2026-03-20T10:00:00Z',
            updated_at: '2026-03-20T10:05:00Z',
          },
          items: [],
          pass_rate: 0.5,
          overall_score: 0.7,
          linked_runs: [],
        },
      },
    })
    compareEvalRunMock.mockResolvedValue({
      data: {
        id: 'comparison-1',
        eval_spec_id: 'eval-spec-1',
        base_eval_run_id: 'baseline-run-1',
        target_eval_run_id: 'eval-run-1',
        baseline_id: 'baseline-1',
        summary: {
          comparison_kind: 'baseline',
          baseline_name: 'Release Baseline',
          overall_score_delta: -0.3,
          pass_rate_delta: -0.5,
          verification_pass_rate_delta: -0.25,
          evidence_backed_pass_rate_delta: -0.5,
          retry_recovered_delta: 1,
          failure_label_delta: {
            missing_artifact: 1,
          },
          regression_count: 1,
          improvement_count: 0,
        },
        regressions: [
          {
            key: 'case-1',
            label: 'finish and verify',
            base_verdict: 'pass',
            target_verdict: 'fail',
            base_verification: 'passed',
            target_verification: 'failed',
            base_evidence_score: 1,
            target_evidence_score: 0.32,
            target_failure_label: 'missing_artifact',
            target_reason: 'evidence mismatch',
          },
        ],
        improvements: [],
        created_at: '2026-03-20T09:22:00Z',
      },
    })

    const HarnessGroupsView = (await import('@/views/HarnessGroupsView.vue')).default
    const wrapper = shallowMount(HarnessGroupsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            template: '<a><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(listGroupsMock).toHaveBeenCalled()
    expect(wrapper.find('.harness-groups-page').exists()).toBe(true)
    expect(wrapper.text()).toContain('Harness')
    expect(wrapper.text()).toContain('Regression batch')
    expect(wrapper.text()).toContain('Validate agent task flows')
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('Eval')
    expect(wrapper.text()).toContain('Running')
    expect(wrapper.text()).toContain('Verification pass rate')
    expect(wrapper.text()).toContain('Evidence-backed pass rate')
    expect(wrapper.text()).toContain('Retry recovered')
    expect(wrapper.text()).toContain('Smoke Dataset')
    expect(wrapper.text()).toContain('Smoke Eval')
    expect(wrapper.text()).toContain('smoke-run-1')

    const inspectButton = wrapper
      .findAll('button')
      .find((node) => node.text().trim() === 'Inspect report')
    expect(inspectButton).toBeTruthy()

    await inspectButton!.trigger('click')
    await flushPromises()

    expect(getEvalRunReportMock).toHaveBeenCalledWith('eval-run-1')
    expect(wrapper.text()).toContain('Dataset snapshot')
    expect(wrapper.text()).toContain('Baseline registry')
    expect(wrapper.text()).toContain('Release Baseline')
    expect(wrapper.text()).toContain('missing_artifact')

    const compareButton = wrapper
      .findAll('button')
      .find((node) => node.text().trim() === 'Generate comparison')
    expect(compareButton).toBeTruthy()

    await compareButton!.trigger('click')
    await flushPromises()

    expect(compareEvalRunMock).toHaveBeenCalledWith('eval-run-1', { baseline_id: 'baseline-1' })
    expect(wrapper.text()).toContain('Compare runs')
    expect(wrapper.text()).toContain('Verification pass rate delta')
    expect(wrapper.text()).toContain('Failure label delta')
    expect(wrapper.text()).toContain('Remediation')
    expect(wrapper.text()).toContain('Attach the expected artifact path or label')
    expect(wrapper.text()).toContain('evidence mismatch')
  }, 10000)

  it('launches a manifest quick eval as an ephemeral group', async () => {
    createGroupMock.mockResolvedValue({
      data: {
        id: 'group-quick-1',
      },
    })

    const HarnessGroupsView = (await import('@/views/HarnessGroupsView.vue')).default
    const wrapper = shallowMount(HarnessGroupsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            template: '<a><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    await wrapper.get('textarea[name="quick-eval-manifest"]').setValue(
      JSON.stringify(
        {
          dataset: {
            name: 'Smoke Dataset',
            subject: 'agent_task',
          },
          defaults: {
            run_kind: 'agent_task',
            profile: 'smoke',
            scoring: {
              mode: 'rule',
              pass_threshold: 0.5,
            },
          },
          items: [
            {
              id: 'case-1',
              input: {
                goal: 'finish and verify',
              },
              expected: {
                contains: 'verified',
              },
            },
          ],
        },
        null,
        2
      )
    )
    await wrapper.get('form.quick-eval-card').trigger('submit.prevent')
    await flushPromises()

    expect(createGroupMock).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: 'eval',
        subject: 'agent_task',
        metadata: expect.objectContaining({
          quick_eval: true,
          ephemeral: true,
          source_mode: 'manifest',
          source_ref: 'ui',
          quick_eval_preset: 'smoke',
          quick_eval_dataset_name: 'Smoke Dataset',
          quick_eval_eval_name: 'Smoke Dataset Smoke Eval',
        }),
        scoring: expect.objectContaining({
          mode: 'rule',
          pass_threshold: 0.5,
          rule_profile: 'smoke',
        }),
        items: [
          expect.objectContaining({
            run_kind: 'agent_task',
            profile: 'smoke',
            input: {
              goal: 'finish and verify',
            },
            expected: {
              contains: 'verified',
            },
          }),
        ],
      })
    )
    expect(createDatasetMock).not.toHaveBeenCalled()
    expect(createDatasetVersionMock).not.toHaveBeenCalled()
    expect(createEvalSpecMock).not.toHaveBeenCalled()
    expect(createEvalRunMock).not.toHaveBeenCalled()
    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroupDetail',
      params: { id: 'group-quick-1' },
    })
    expect(wrapper.text()).toContain('Quick Eval')
  })

  it('does not launch a quick eval from the empty template', async () => {
    const HarnessGroupsView = (await import('@/views/HarnessGroupsView.vue')).default
    const wrapper = shallowMount(HarnessGroupsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            template: '<a><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    await wrapper.get('form.quick-eval-card').trigger('submit.prevent')
    await flushPromises()

    expect(createGroupMock).not.toHaveBeenCalled()
    expect(createDatasetMock).not.toHaveBeenCalled()
    expect(createDatasetVersionMock).not.toHaveBeenCalled()
    expect(createEvalSpecMock).not.toHaveBeenCalled()
    expect(createEvalRunMock).not.toHaveBeenCalled()
    expect(notificationErrorMock).toHaveBeenCalledWith(
      'Harness',
      'Add at least one real case before launching a quick eval.'
    )
  })

  it('keeps dataset quick eval on the eval-run path', async () => {
    listDatasetsMock.mockResolvedValue({
      data: [
        {
          id: 'dataset-quick-1',
          name: 'Smoke Dataset',
          subject: 'agent_task',
          default_run_kind: 'agent_task',
          default_profile: 'smoke',
          active_version_id: 'dataset-version-quick-1',
          created_at: '2026-03-20T09:00:00Z',
          updated_at: '2026-03-20T09:00:00Z',
        },
      ],
    })
    listDatasetVersionsMock.mockResolvedValue({
      data: [
        {
          id: 'dataset-version-quick-1',
          dataset_id: 'dataset-quick-1',
          version: 'v1',
          item_count: 1,
          created_at: '2026-03-20T09:22:00Z',
        },
      ],
    })
    createEvalSpecMock.mockResolvedValue({
      data: {
        id: 'eval-spec-quick-1',
        name: 'Smoke Dataset Smoke Eval',
        dataset_id: 'dataset-quick-1',
        dataset_version_id: 'dataset-version-quick-1',
        run_kind: 'agent_task',
        profile: 'smoke',
        scoring_config: {
          mode: 'rule',
          pass_threshold: 0.5,
        },
        created_at: '2026-03-20T09:23:00Z',
        updated_at: '2026-03-20T09:23:00Z',
      },
    })
    createEvalRunMock.mockResolvedValue({
      data: {
        id: 'eval-run-quick-1',
        eval_spec_id: 'eval-spec-quick-1',
        group_id: 'group-quick-1',
        dataset_version_id: 'dataset-version-quick-1',
        title: 'Smoke Dataset Smoke 20260320-0924',
        status: 'queued',
        created_at: '2026-03-20T09:24:00Z',
        updated_at: '2026-03-20T09:24:00Z',
      },
    })
    getEvalRunReportMock.mockResolvedValue({
      data: {
        eval_run: {
          id: 'eval-run-quick-1',
          eval_spec_id: 'eval-spec-quick-1',
          group_id: 'group-quick-1',
          dataset_version_id: 'dataset-version-quick-1',
          title: 'Smoke Dataset Smoke 20260320-0924',
          status: 'queued',
          created_at: '2026-03-20T09:24:00Z',
          updated_at: '2026-03-20T09:24:00Z',
        },
        eval_spec: {
          id: 'eval-spec-quick-1',
          name: 'Smoke Dataset Smoke Eval',
          dataset_id: 'dataset-quick-1',
          dataset_version_id: 'dataset-version-quick-1',
          run_kind: 'agent_task',
          scoring_config: {
            mode: 'rule',
            pass_threshold: 0.5,
          },
          created_at: '2026-03-20T09:23:00Z',
          updated_at: '2026-03-20T09:23:00Z',
        },
        dataset: {
          id: 'dataset-quick-1',
          name: 'Smoke Dataset',
          created_at: '2026-03-20T09:00:00Z',
          updated_at: '2026-03-20T09:00:00Z',
        },
        dataset_version: {
          id: 'dataset-version-quick-1',
          dataset_id: 'dataset-quick-1',
          version: 'v1',
          item_count: 1,
          created_at: '2026-03-20T09:22:00Z',
        },
        group_report: {
          group: {
            id: 'group-quick-1',
            kind: 'eval',
            status: 'queued',
            created_at: '2026-03-20T09:24:00Z',
            updated_at: '2026-03-20T09:24:00Z',
          },
          items: [],
          linked_runs: [],
        },
      },
    })

    const HarnessGroupsView = (await import('@/views/HarnessGroupsView.vue')).default
    const wrapper = shallowMount(HarnessGroupsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            template: '<a><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    const datasetButton = wrapper
      .findAll('button')
      .find((node) => node.text().trim() === 'Reuse dataset')
    expect(datasetButton).toBeTruthy()

    await datasetButton!.trigger('click')
    await flushPromises()

    await wrapper.get('select[name="quick-eval-dataset"]').setValue('dataset-quick-1')
    await flushPromises()

    await wrapper.get('form.quick-eval-card').trigger('submit.prevent')
    await flushPromises()

    expect(createGroupMock).not.toHaveBeenCalled()
    expect(createEvalSpecMock).toHaveBeenCalledWith(
      expect.objectContaining({
        dataset_id: 'dataset-quick-1',
        dataset_version_id: 'dataset-version-quick-1',
        run_kind: 'agent_task',
        profile: 'smoke',
      })
    )
    expect(createEvalRunMock).toHaveBeenCalledWith(
      expect.objectContaining({
        eval_spec_id: 'eval-spec-quick-1',
        trigger_kind: 'quick_eval',
        trigger_ref: 'ui',
      })
    )
    expect(getEvalRunReportMock).toHaveBeenCalledWith('eval-run-quick-1')
    expect(routerPushMock).not.toHaveBeenCalled()
  })

  it('launches a conversation quick eval as an ephemeral group with preset-aware expectations', async () => {
    listConversationsMock.mockResolvedValue({
      data: [
        {
          id: 'conversation-1',
          title: 'Release candidate fix',
          created_at: '2026-03-20T09:00:00Z',
          updated_at: '2026-03-20T09:05:00Z',
        },
      ],
    })
    getConversationMock.mockResolvedValue({
      data: {
        id: 'conversation-1',
        title: 'Release candidate fix',
        created_at: '2026-03-20T09:00:00Z',
        updated_at: '2026-03-20T09:05:00Z',
      },
    })
    listMessagesMock.mockResolvedValue({
      data: [
        {
          id: 'msg-user-1',
          conversation_id: 'conversation-1',
          role: 'user',
          content: 'finish and verify',
          created_at: '2026-03-20T09:01:00Z',
        },
        {
          id: 'msg-assistant-1',
          conversation_id: 'conversation-1',
          role: 'assistant',
          content: 'verified',
          provider: 'openai',
          model: 'gpt-5.4-mini',
          created_at: '2026-03-20T09:02:00Z',
        },
      ],
    })
    createGroupMock.mockResolvedValue({
      data: {
        id: 'group-conversation-1',
      },
    })

    const HarnessGroupsView = (await import('@/views/HarnessGroupsView.vue')).default
    const wrapper = shallowMount(HarnessGroupsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            template: '<a><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    const conversationButton = wrapper
      .findAll('button')
      .find((node) => node.text().trim() === 'Use conversation')
    expect(conversationButton).toBeTruthy()

    await conversationButton!.trigger('click')
    await flushPromises()

    expect(listConversationsMock).toHaveBeenCalledWith(50, 0)
    expect(listMessagesMock).toHaveBeenCalledWith('conversation-1', 500, 0)

    const preview = wrapper.get('textarea[name="quick-eval-conversation-preview"]')
    expect((preview.element as HTMLTextAreaElement).value).toContain(
      'Release candidate fix Smoke Dataset'
    )
    expect((preview.element as HTMLTextAreaElement).value).toContain('finish and verify')
    expect((preview.element as HTMLTextAreaElement).value).toContain('"status": "completed"')
    expect((preview.element as HTMLTextAreaElement).value).not.toContain('"contains": "verified"')

    const editDraftButton = wrapper
      .findAll('button')
      .find((node) => node.text().trim() === 'Edit Cases JSON')
    expect(editDraftButton).toBeTruthy()

    await editDraftButton!.trigger('click')
    await flushPromises()

    const manifestEditor = wrapper.get('textarea[name="quick-eval-manifest"]')
    expect((manifestEditor.element as HTMLTextAreaElement).value).toContain(
      'Release candidate fix Smoke Dataset'
    )

    await conversationButton!.trigger('click')
    await flushPromises()

    await wrapper.get('select[name="quick-eval-preset"]').setValue('research')
    await flushPromises()

    const researchPreview = wrapper.get('textarea[name="quick-eval-conversation-preview"]')
    expect((researchPreview.element as HTMLTextAreaElement).value).toContain(
      '"required_observations"'
    )
    expect((researchPreview.element as HTMLTextAreaElement).value).toContain(
      '"evidence_tool_used"'
    )

    await wrapper.get('select[name="quick-eval-conversation"]').setValue('conversation-1')
    await flushPromises()

    await wrapper.get('form.quick-eval-card').trigger('submit.prevent')
    await flushPromises()

    expect(createGroupMock).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: 'eval',
        subject: 'research',
        metadata: expect.objectContaining({
          quick_eval: true,
          ephemeral: true,
          source_mode: 'conversation',
          source_ref: 'conversation-1',
          quick_eval_preset: 'research',
          quick_eval_dataset_name: 'Release candidate fix Research Dataset',
          quick_eval_eval_name: 'Release candidate fix Research Dataset Research Eval',
        }),
        items: [
          expect.objectContaining({
            run_kind: 'research',
            profile: 'research',
            input: {
              goal: 'finish and verify',
            },
            expected: {
              status: 'completed',
              required_observations: ['evidence_tool_used'],
            },
            metadata: expect.objectContaining({
              conversation_id: 'conversation-1',
              user_message_id: 'msg-user-1',
              assistant_message_id: 'msg-assistant-1',
              provider: 'openai',
              model: 'gpt-5.4-mini',
            }),
          }),
        ],
      })
    )
    expect(createDatasetMock).not.toHaveBeenCalled()
    expect(createDatasetVersionMock).not.toHaveBeenCalled()
    expect(createEvalSpecMock).not.toHaveBeenCalled()
    expect(createEvalRunMock).not.toHaveBeenCalled()
    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroupDetail',
      params: { id: 'group-conversation-1' },
    })
  })

  it('renders the harness group detail report with failed items and runs', async () => {
    getGroupReportMock.mockResolvedValue({
      data: {
        group: {
          id: 'group-1',
          kind: 'eval',
          title: 'Research calibration summary',
          status: 'completed',
          subject: 'Post-research calibration',
          owner_user_id: 'user-1',
          scoring_config: {
            mode: 'hybrid',
          },
          summary: {
            counts: {
              passed: 1,
              failed: 1,
            },
            verification_pass_rate: 0,
            evidence_backed_pass_rate: 0,
            failure_label_counts: {
              missing_artifact: 1,
            },
          },
          created_at: '2026-03-20T10:00:00Z',
          updated_at: '2026-03-20T10:05:00Z',
          started_at: '2026-03-20T10:00:00Z',
          finished_at: '2026-03-20T10:05:00Z',
        },
        items: [
          {
            id: 'item-1',
            group_id: 'group-1',
            index: 0,
            run_kind: 'research',
            profile: 'research',
            input: { goal: 'Projected experiment' },
            status: 'failed',
            latest_run_id: 'run-1',
            attempt_count: 1,
            max_attempts: 1,
            created_at: '2026-03-20T10:00:00Z',
            updated_at: '2026-03-20T10:05:00Z',
          },
        ],
        verdict_counts: {
          pass: 0,
          fail: 1,
          partial: 0,
          error: 0,
        },
        overall_score: 0.4,
        pass_rate: 0,
        linked_runs: [
          {
            id: 'run-1',
            root_run_id: 'run-1',
            kind: 'research',
            status: 'failed',
            goal: 'Post-research calibration',
            error: 'conflicting primary-source evidence',
            attempt_index: 1,
            created_at: '2026-03-20T10:00:00Z',
            updated_at: '2026-03-20T10:05:00Z',
          },
        ],
        artifacts: [
          {
            id: 'artifact-1',
            run_id: 'run-1',
            kind: 'report',
            label: 'projection-report',
            path_or_url: 'https://example.com/report',
          },
        ],
        scorecards: [
          {
            id: 'score-1',
            group_id: 'group-1',
            group_item_id: 'item-1',
            run_id: 'run-1',
            mode: 'hybrid',
            verdict: 'fail',
            score: 0.4,
            breakdown_json: JSON.stringify({
              reason: 'evidence mismatch',
              verification_passed: false,
              failure_label: 'missing_artifact',
              retryable: true,
              outcome_score: 1,
              evidence_score: 0.4,
              execution_score: 1,
              judge_backend: 'llm_evaluator',
              judge_model: 'gpt-5.4-mini',
              calibration_ref: 'deep_research:job-1:calibration',
              takeaway_candidate_count: 2,
              proposal_count: 1,
              proposal_ids: ['proposal-1'],
            }),
            evidence_json: JSON.stringify({
              verification: {
                passed: false,
                retryable: true,
                failure_label: 'missing_artifact',
                summary: 'expected artifact \"result.txt\" was not produced',
                outcome_score: 1,
                evidence_score: 0.4,
                execution_score: 1,
                observations: ['tool_error_seen', 'artifact_emitted'],
                checks: [
                  {
                    name: 'run_completed',
                    expected: 'completed',
                    actual: 'failed',
                    passed: false,
                  },
                  {
                    name: 'required_tool_call',
                    expected: 'web.search',
                    actual: ['web.search'],
                    passed: true,
                  },
                ],
                artifacts: [
                  {
                    path: 'result.txt',
                    must_exist: true,
                    actual: 'missing',
                    passed: false,
                  },
                  {
                    label: 'projection-report',
                    must_exist: true,
                    actual: 'https://example.com/report',
                    passed: true,
                  },
                ],
                trace_summary: {
                  event_count: 7,
                  artifact_count: 1,
                  tool_names: ['web.search', 'functions.exec_command'],
                },
              },
            }),
            judge_trace_json: JSON.stringify({
              proposal_skipped_reason: '',
            }),
            created_at: '2026-03-20T10:05:00Z',
          },
        ],
        failed_items: [
          {
            item: {
              id: 'item-1',
              index: 0,
              profile: 'research',
            },
            scorecard: {
              verdict: 'fail',
              breakdown_json: JSON.stringify({
                reason: 'evidence mismatch',
                verification_passed: false,
                failure_label: 'missing_artifact',
                evidence_score: 0.4,
              }),
            },
            run: {
              id: 'run-1',
              status: 'failed',
              error: 'conflicting primary-source evidence',
            },
          },
        ],
      },
    })

    const HarnessGroupDetailView = (await import('@/views/HarnessGroupDetailView.vue')).default
    const wrapper = shallowMount(HarnessGroupDetailView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            template: '<a><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(getGroupReportMock).toHaveBeenCalledWith('group-1')
    expect(wrapper.find('.harness-detail-page').exists()).toBe(true)
    expect(wrapper.text()).toContain('Research calibration summary')
    expect(wrapper.text()).toContain('Post-research calibration')
    expect(wrapper.text()).toContain('evidence mismatch')
    expect(wrapper.text()).toContain('conflicting primary-source evidence')
    expect(wrapper.text()).toContain('projection-report')
    expect(wrapper.text()).toContain('Eval')
    expect(wrapper.text()).toContain('Completed')
    expect(wrapper.text()).toContain('Research')
    expect(wrapper.text()).toContain('Hybrid')
    expect(wrapper.text()).toContain('Verification pass rate')
    expect(wrapper.text()).toContain('Evidence-backed pass rate')
    expect(wrapper.text()).toContain('Failure label')
    expect(wrapper.text()).toContain('missing_artifact')
    expect(wrapper.text()).toContain('Retryable')
    expect(wrapper.text()).toContain('Outcome score')
    expect(wrapper.text()).toContain('Execution score')
    expect(wrapper.text()).toContain('Remediation')
    expect(wrapper.text()).toContain('Attach the expected artifact path or label')
    expect(wrapper.text()).toContain('Observations')
    expect(wrapper.text()).toContain('tool_error_seen')
    expect(wrapper.text()).toContain('artifact_emitted')
    expect(wrapper.text()).toContain('Calibration & proposal summary')
    expect(wrapper.text()).toContain('llm_evaluator')
    expect(wrapper.text()).toContain('gpt-5.4-mini')
    expect(wrapper.text()).toContain('deep_research:job-1:calibration')
    expect(wrapper.text()).toContain('proposal-1')
    expect(wrapper.text()).toContain('Review in Memory')
    expect(wrapper.text()).toContain('Verification checks')
    expect(wrapper.text()).toContain('Expected artifacts')
    expect(wrapper.text()).toContain('Trace summary')
    expect(wrapper.text()).toContain('Expected')
    expect(wrapper.text()).toContain('Actual')
    expect(wrapper.text()).toContain('result.txt')
    expect(wrapper.text()).toContain('Observed tools')
    expect(wrapper.text()).toContain('web.search')
  })

  it('promotes an ephemeral group into reusable eval assets', async () => {
    getGroupReportMock.mockResolvedValue({
      data: {
        group: {
          id: 'group-1',
          kind: 'eval',
          title: 'Research quick eval',
          status: 'completed',
          subject: 'research',
          metadata: {
            quick_eval: true,
            ephemeral: true,
            quick_eval_dataset_name: 'Research quick eval',
            quick_eval_eval_name: 'Research quick eval Research Eval',
          },
          created_at: '2026-03-20T10:00:00Z',
          updated_at: '2026-03-20T10:05:00Z',
        },
        items: [],
        verdict_counts: {
          pass: 1,
          fail: 0,
          partial: 0,
          error: 0,
        },
        linked_runs: [],
        artifacts: [],
        scorecards: [],
        failed_items: [],
      },
    })
    promoteGroupMock.mockResolvedValue({
      data: {
        dataset: {
          id: 'dataset-1',
          name: 'Research Cases',
          subject: 'research',
          created_at: '2026-03-20T10:06:00Z',
          updated_at: '2026-03-20T10:06:00Z',
        },
        dataset_version: {
          id: 'dataset-version-1',
          dataset_id: 'dataset-1',
          version: 'promoted-20260320-100600',
          item_count: 1,
          created_at: '2026-03-20T10:06:00Z',
        },
        eval_spec: {
          id: 'eval-spec-1',
          name: 'Research Eval',
          dataset_id: 'dataset-1',
          dataset_version_id: 'dataset-version-1',
          run_kind: 'research',
          profile: 'research',
          created_at: '2026-03-20T10:06:00Z',
          updated_at: '2026-03-20T10:06:00Z',
        },
      },
    })

    const HarnessGroupDetailView = (await import('@/views/HarnessGroupDetailView.vue')).default
    const wrapper = shallowMount(HarnessGroupDetailView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            template: '<a><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Promote to regression assets')
    expect((wrapper.get('input[name="promotion-dataset-name"]').element as HTMLInputElement).value).toBe(
      'Research quick eval'
    )
    expect((wrapper.get('input[name="promotion-eval-name"]').element as HTMLInputElement).value).toBe(
      'Research quick eval Research Eval'
    )

    await wrapper.get('input[name="promotion-dataset-name"]').setValue('Research Cases')
    await wrapper.get('input[name="promotion-subject"]').setValue('research')
    await wrapper.get('textarea[name="promotion-description"]').setValue(
      'Promoted from a successful quick eval.'
    )
    await wrapper.get('input[name="promotion-eval-name"]').setValue('Research Eval')
    await wrapper.find('form.promotion-form').trigger('submit.prevent')
    await flushPromises()

    expect(promoteGroupMock).toHaveBeenCalledWith('group-1', {
      dataset_name: 'Research Cases',
      description: 'Promoted from a successful quick eval.',
      subject: 'research',
      eval_name: 'Research Eval',
    })
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      'Harness',
      'Group promoted into reusable eval assets'
    )
    expect(wrapper.text()).toContain('Research Cases')
    expect(wrapper.text()).toContain('promoted-20260320-100600')
    expect(wrapper.text()).toContain('Research Eval')
  })

  it('links the harness detail back action to the security harness tab', async () => {
    getGroupReportMock.mockResolvedValue({
      data: {
        group: {
          id: 'group-1',
          kind: 'eval',
          title: 'Regression batch',
          status: 'completed',
          subject: 'Validate agent task flows',
          created_at: '2026-03-20T10:00:00Z',
          updated_at: '2026-03-20T10:05:00Z',
        },
        items: [],
        verdict_counts: {
          pass: 0,
          fail: 0,
          partial: 0,
          error: 0,
        },
        linked_runs: [],
        artifacts: [],
        scorecards: [],
        failed_items: [],
      },
    })

    const HarnessGroupDetailView = (await import('@/views/HarnessGroupDetailView.vue')).default
    const wrapper = shallowMount(HarnessGroupDetailView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a class="router-link-stub" :data-to="JSON.stringify(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    const backLink = wrapper.find('.back-link')
    expect(backLink.exists()).toBe(true)
    expect(backLink.attributes('data-to')).toBe(
      JSON.stringify({ name: 'Security', query: { tab: 'harness' } })
    )
  })
})
