import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const listGroupsMock = vi.fn()
const getGroupReportMock = vi.fn()
const cancelGroupMock = vi.fn()
const retryFailedGroupMock = vi.fn()

vi.mock('@/api/harness', () => ({
  harnessApi: {
    listGroups: listGroupsMock,
    getGroupReport: getGroupReportMock,
    cancelGroup: cancelGroupMock,
    retryFailedGroup: retryFailedGroupMock,
  },
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => ({
    success: vi.fn(),
    info: vi.fn(),
    error: vi.fn(),
  }),
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
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
              judge_backend: 'llm_evaluator',
              judge_model: 'gpt-5.4-mini',
              calibration_ref: 'deep_research:job-1:calibration',
              takeaway_candidate_count: 2,
              proposal_count: 1,
              proposal_ids: ['proposal-1'],
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
              breakdown_json: JSON.stringify({ reason: 'evidence mismatch' }),
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
    expect(wrapper.text()).toContain('Calibration & proposal summary')
    expect(wrapper.text()).toContain('llm_evaluator')
    expect(wrapper.text()).toContain('gpt-5.4-mini')
    expect(wrapper.text()).toContain('deep_research:job-1:calibration')
    expect(wrapper.text()).toContain('proposal-1')
    expect(wrapper.text()).toContain('Review in Memory')
  })
})
