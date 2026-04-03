import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const listGroupsMock = vi.fn()
const listDatasetsMock = vi.fn()
const listDatasetVersionsMock = vi.fn()
const listEvalSpecsMock = vi.fn()
const listEvalRunsMock = vi.fn()
const listBaselinesMock = vi.fn()
const getEvalRunReportMock = vi.fn()
const compareEvalRunMock = vi.fn()
const getGroupReportMock = vi.fn()
const getRunDetailMock = vi.fn()
const cancelGroupMock = vi.fn()
const retryFailedGroupMock = vi.fn()
const listConversationsMock = vi.fn()
const listMessagesMock = vi.fn()
const routerPushMock = vi.fn()
const notificationSuccessMock = vi.fn()
const notificationInfoMock = vi.fn()
const notificationErrorMock = vi.fn()

const mountedWrappers: Array<{ unmount: () => void }> = []
const routeMock = {
  path: '/automation/harness/group-1',
  params: { id: 'group-1' },
}

vi.mock('@/api/harness', () => ({
  harnessApi: {
    listGroups: listGroupsMock,
    listDatasets: listDatasetsMock,
    listDatasetVersions: listDatasetVersionsMock,
    listEvalSpecs: listEvalSpecsMock,
    listEvalRuns: listEvalRunsMock,
    listBaselines: listBaselinesMock,
    getEvalRunReport: getEvalRunReportMock,
    compareEvalRun: compareEvalRunMock,
    getGroupReport: getGroupReportMock,
    getRunDetail: getRunDetailMock,
    cancelGroup: cancelGroupMock,
    retryFailedGroup: retryFailedGroupMock,
  },
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    list: listConversationsMock,
  },
  messageApi: {
    list: listMessagesMock,
  },
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => ({
    success: notificationSuccessMock,
    info: notificationInfoMock,
    error: notificationErrorMock,
  }),
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeMock,
    useRouter: () => ({
      push: routerPushMock,
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
          automation: 'Automation',
          harness: 'Harness',
        },
        common: {
          active: 'Active',
          all: 'All',
          back: 'Back',
          cancel: 'Cancel',
          error: 'Error',
          loading: 'Loading',
          notAvailable: 'Not available',
          refresh: 'Refresh',
          updatedAt: 'Updated',
          type: 'Type',
        },
        harness: {
          groups: {
            subtitle: 'Browse groups',
            totalGroups: 'Groups',
            avgPassRate: 'Average pass rate',
            loading: 'Fetching groups',
            terminalOnly: 'Terminal',
            searchPlaceholder: 'Search title, subject, owner, kind, or status',
            noSubject: 'No subject provided',
            kind: 'Kind',
            owner: 'Owner',
            itemCount: 'Items',
            passRate: 'Pass rate',
            score: 'Score',
            running: 'Running',
            failed: 'Failed',
            passed: 'Passed',
            emptyDescription: 'No groups',
            startedAt: 'Started',
            finishedAt: 'Finished',
            status: 'Status',
          },
          group: {
            noSubject: 'This group does not include a subject line.',
            cancelled: 'Group cancelled',
            retryResult: 'Failed items requeued.',
            retryFailed: 'Retry failed',
            loading: 'Loading the latest group report.',
            totalAttempts: 'Attempts',
            scoreDistribution: 'Score distribution',
            scoringMode: 'Scoring mode',
            passVerdict: 'Pass',
            failVerdict: 'Fail',
            partialVerdict: 'Partial',
            errorVerdict: 'Error',
            queuedCount: 'Queued',
            failedCount: 'Failed',
            failureLabel: 'Failure label',
            noFailureLabels: 'No failure labels recorded.',
            failedItems: 'Failed items',
            noFailedItems: 'No failed items in the latest report.',
            attempts: 'Attempts',
            noFailureReason: 'No failure reason recorded.',
            artifacts: 'Artifacts',
            noArtifacts: 'No artifacts attached to the linked runs yet.',
          },
        },
        automation: {
          tabs: {
            cron: 'Scheduled Tasks',
            cronDesc: 'Manage cron jobs and scheduled executions',
            harness: 'Harness',
            harnessDesc: 'Review run records, eval groups, and scoring results.',
          },
        },
      },
    },
  })
}

function setWindowWidth(width: number) {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: width,
  })
}

function createLinkedRuns() {
  return [
    {
      id: 'run-root',
      root_run_id: 'run-root',
      kind: 'agent_task',
      status: 'executing',
      goal: 'Coordinate worker tasks',
      agent_id: 'coordinator.main',
      model: 'gpt-5.4',
      attempt_index: 1,
      depth: 0,
      progress: 0.55,
      created_at: '2026-03-20T10:00:00Z',
      updated_at: '2026-03-20T10:05:00Z',
      started_at: '2026-03-20T10:00:05Z',
    },
    {
      id: 'run-fetch',
      root_run_id: 'run-root',
      parent_run_id: 'run-root',
      kind: 'subagent',
      status: 'completed',
      goal: 'Fetch evaluation fixtures',
      agent_id: 'worker.fetch',
      model: 'gpt-5.4-mini',
      attempt_index: 1,
      depth: 1,
      progress: 1,
      result: 'Fixtures fetched',
      created_at: '2026-03-20T10:00:10Z',
      updated_at: '2026-03-20T10:01:00Z',
      started_at: '2026-03-20T10:00:12Z',
      finished_at: '2026-03-20T10:01:00Z',
    },
    {
      id: 'run-review',
      root_run_id: 'run-root',
      parent_run_id: 'run-root',
      kind: 'subagent',
      status: 'failed',
      goal: 'Review artifact integrity',
      agent_id: 'worker.review',
      model: 'gpt-5.4-mini',
      attempt_index: 1,
      depth: 1,
      progress: 0.9,
      error: 'artifact missing',
      created_at: '2026-03-20T10:00:12Z',
      updated_at: '2026-03-20T10:03:00Z',
      started_at: '2026-03-20T10:00:15Z',
      finished_at: '2026-03-20T10:03:00Z',
    },
    {
      id: 'run-input',
      root_run_id: 'run-root',
      parent_run_id: 'run-root',
      kind: 'subagent',
      status: 'waiting_input',
      goal: 'Ask for the fixture directory override',
      agent_id: 'worker.blocker',
      model: 'gpt-5.4-mini',
      attempt_index: 1,
      depth: 1,
      progress: 0.4,
      created_at: '2026-03-20T10:00:16Z',
      updated_at: '2026-03-20T10:04:00Z',
      started_at: '2026-03-20T10:00:18Z',
    },
    {
      id: 'run-verify',
      root_run_id: 'run-root',
      parent_run_id: 'run-root',
      kind: 'subagent',
      status: 'executing',
      goal: 'Verify bundle outputs',
      agent_id: 'worker.verify',
      model: 'gpt-5.4-mini',
      attempt_index: 1,
      depth: 1,
      progress: 0.62,
      created_at: '2026-03-20T10:00:18Z',
      updated_at: '2026-03-20T10:04:30Z',
      started_at: '2026-03-20T10:00:20Z',
    },
    {
      id: 'run-orphan',
      root_run_id: 'run-orphan-root',
      parent_run_id: 'missing-parent',
      kind: 'subagent',
      status: 'completed',
      goal: 'Detached worker for legacy replay',
      agent_id: 'worker.orphan',
      model: 'gpt-5.4-mini',
      attempt_index: 1,
      depth: 1,
      progress: 1,
      result: 'Detached cleanup complete',
      created_at: '2026-03-20T10:00:20Z',
      updated_at: '2026-03-20T10:04:50Z',
      started_at: '2026-03-20T10:00:25Z',
      finished_at: '2026-03-20T10:04:50Z',
    },
  ]
}

function createGroupReport(overrides: Record<string, unknown> = {}) {
  const base = {
    group: {
      id: 'group-1',
      kind: 'eval',
      title: 'Regression batch',
      status: 'running',
      subject: 'Validate agent task flows',
      owner_user_id: 'owner-1',
      scoring_config: {
        mode: 'rule',
      },
      summary: {
        counts: {
          queued: 1,
          running: 2,
          passed: 8,
          failed: 2,
        },
        failure_label_counts: {
          missing_artifact: 2,
        },
      },
      created_at: '2026-03-20T10:00:00Z',
      updated_at: '2026-03-20T10:05:00Z',
      started_at: '2026-03-20T10:00:00Z',
      finished_at: null,
    },
    items: [
      {
        id: 'item-1',
        group_id: 'group-1',
        index: 0,
        run_kind: 'agent_task',
        input: { goal: 'finish and verify' },
        status: 'failed',
        latest_run_id: 'run-review',
        attempt_count: 1,
        max_attempts: 2,
        created_at: '2026-03-20T10:00:00Z',
        updated_at: '2026-03-20T10:05:00Z',
      },
      {
        id: 'item-2',
        group_id: 'group-1',
        index: 1,
        run_kind: 'agent_task',
        input: { goal: 'wait for fixture path' },
        status: 'running',
        latest_run_id: 'run-input',
        attempt_count: 1,
        max_attempts: 2,
        created_at: '2026-03-20T10:00:10Z',
        updated_at: '2026-03-20T10:05:00Z',
      },
    ],
    verdict_counts: {
      pass: 8,
      fail: 2,
      partial: 0,
      error: 0,
    },
    overall_score: 0.61,
    pass_rate: 0.8,
    linked_runs: createLinkedRuns(),
    artifacts: [
      {
        id: 'artifact-1',
        run_id: 'run-review',
        kind: 'log',
        label: 'stderr.log',
        path_or_url: '/tmp/stderr.log',
      },
    ],
    failed_items: [
      {
        item: {
          id: 'item-1',
          index: 0,
        },
        scorecard: {
          breakdown_json: JSON.stringify({
            failure_label: 'missing_artifact',
            reason: 'artifact missing',
          }),
        },
        run: {
          id: 'run-review',
          status: 'failed',
          goal: 'Review artifact integrity',
          error: 'artifact missing',
        },
      },
    ],
    runtime_traces: {
      'run-verify': {
        run_id: 'run-verify',
        events: [
          {
            type: 'tool_call',
            message: 'Running bundle verification',
            created_at: '2026-03-20T10:04:25Z',
          },
        ],
      },
    },
  }

  return {
    ...base,
    ...overrides,
    group: {
      ...base.group,
      ...((overrides.group as Record<string, unknown>) || {}),
    },
    items: (overrides.items as unknown[]) || base.items,
    verdict_counts: (overrides.verdict_counts as Record<string, number>) || base.verdict_counts,
    linked_runs: (overrides.linked_runs as unknown[]) || base.linked_runs,
    artifacts: (overrides.artifacts as unknown[]) || base.artifacts,
    failed_items: (overrides.failed_items as unknown[]) || base.failed_items,
    runtime_traces: (overrides.runtime_traces as Record<string, unknown>) || base.runtime_traces,
  }
}

function createRunDetail(runID: string) {
  const run = createLinkedRuns().find((entry) => entry.id === runID) || createLinkedRuns()[0]

  const reviewEvents = [
    {
      id: 'review-event-1',
      run_id: 'run-review',
      type: 'agent_note',
      message: 'Checking generated artifact manifest',
      created_at: '2026-03-20T10:00:30Z',
    },
    {
      id: 'review-event-2',
      run_id: 'run-review',
      type: 'tool_call',
      tool_name: 'shell.exec',
      message: 'Listing artifact directory',
      created_at: '2026-03-20T10:00:45Z',
    },
    {
      id: 'review-event-3',
      run_id: 'run-review',
      type: 'tool_call',
      tool_name: 'file.read',
      message: 'Inspecting report bundle',
      created_at: '2026-03-20T10:01:10Z',
    },
    {
      id: 'review-event-4',
      run_id: 'run-review',
      type: 'agent_note',
      message: 'Expected stderr.log was not found in outputs',
      created_at: '2026-03-20T10:01:25Z',
    },
    {
      id: 'review-event-5',
      run_id: 'run-review',
      type: 'tool_call',
      tool_name: 'file.search',
      message: 'Searching for missing artifact',
      created_at: '2026-03-20T10:01:40Z',
    },
    {
      id: 'review-event-6',
      run_id: 'run-review',
      type: 'agent_note',
      message: 'Validation failed after artifact check',
      created_at: '2026-03-20T10:02:00Z',
    },
  ]

  const eventsByRun: Record<string, unknown[]> = {
    'run-root': [
      {
        id: 'root-event-1',
        run_id: 'run-root',
        type: 'agent_note',
        message: 'Spawning 4 workers',
        created_at: '2026-03-20T10:00:20Z',
      },
      {
        id: 'root-event-2',
        run_id: 'run-root',
        type: 'agent_note',
        message: 'Waiting on worker results',
        created_at: '2026-03-20T10:04:55Z',
      },
    ],
    'run-review': reviewEvents,
    'run-input': [
      {
        id: 'input-event-1',
        run_id: 'run-input',
        type: 'agent_note',
        message: 'Need a fixture path before continuing',
        created_at: '2026-03-20T10:04:00Z',
      },
    ],
    'run-fetch': [
      {
        id: 'fetch-event-1',
        run_id: 'run-fetch',
        type: 'agent_note',
        message: 'Fixture fetch completed',
        created_at: '2026-03-20T10:01:00Z',
      },
    ],
    'run-verify': [
      {
        id: 'verify-event-1',
        run_id: 'run-verify',
        type: 'tool_call',
        tool_name: 'shell.exec',
        message: 'Running bundle verification',
        created_at: '2026-03-20T10:04:25Z',
      },
    ],
    'run-orphan': [
      {
        id: 'orphan-event-1',
        run_id: 'run-orphan',
        type: 'agent_note',
        message: 'Detached replay completed',
        created_at: '2026-03-20T10:04:40Z',
      },
    ],
  }

  return {
    run: {
      ...run,
      runtime_state: run.status === 'waiting_input' ? 'blocked' : 'executing',
      sandbox_mode: 'workspace-write',
      approval_mode: 'ask',
      workspace_root: '/workspace/zima-blue',
    },
    actions: null,
    events: eventsByRun[runID] || [],
    artifacts:
      runID === 'run-review'
        ? [
            {
              id: 'artifact-run-review',
              run_id: 'run-review',
              kind: 'log',
              label: 'stderr.log',
              path_or_url: '/tmp/stderr.log',
              mime_type: 'text/plain',
            },
          ]
        : [],
    pending_approvals: [],
    pending_questions:
      runID === 'run-input'
        ? [
            {
              question: 'Please provide the fixture directory.',
            },
          ]
        : [],
    run_trace: {
      run_id: runID,
      root_run_id: run.root_run_id,
      parent_run_id: run.parent_run_id,
      kind: run.kind,
      status: run.status,
      started_at: run.started_at,
      finished_at: run.finished_at,
      latency_ms: runID === 'run-review' ? 1540 : 920,
      stages: [
        {
          stage: 'planning',
          message: 'Trace planning stage entered',
          status: 'completed',
          created_at: '2026-03-20T10:00:20Z',
          details: { iteration: 1 },
        },
        {
          stage: 'finalize',
          message: 'Trace finalize stage recorded',
          status: run.status === 'failed' ? 'failed' : 'completed',
          created_at: '2026-03-20T10:02:10Z',
          details: { terminal_status: run.status },
        },
      ],
      events: [
        {
          type: 'trace_started',
          message: 'Trace started',
          created_at: '2026-03-20T10:00:20Z',
        },
      ],
      artifacts:
        runID === 'run-review'
          ? [
              {
                id: 'trace-artifact-review',
                run_id: 'run-review',
                kind: 'log',
                label: 'trace.log',
                path_or_url: '/tmp/trace.log',
              },
            ]
          : [],
    },
  }
}

function createEvalRunReport(overrides: Record<string, unknown> = {}) {
  const base = {
    eval_run: {
      id: 'eval-run-1',
      eval_spec_id: 'eval-spec-1',
      group_id: 'group-1',
      dataset_version_id: 'dataset-version-1',
      title: 'Nightly Eval Run',
      owner_user_id: 'owner-1',
      status: 'running',
      trigger_kind: 'manual',
      summary: {
        pass_rate: 0.8,
        overall_score: 0.61,
      },
      created_at: '2026-03-20T10:00:00Z',
      updated_at: '2026-03-20T10:05:00Z',
      started_at: '2026-03-20T10:00:00Z',
      finished_at: null,
    },
    eval_spec: {
      id: 'eval-spec-1',
      name: 'Nightly Spec',
      run_kind: 'agent_task',
      profile: 'default',
      scoring_config: {
        mode: 'rule',
        pass_threshold: 0.7,
      },
    },
    dataset: {
      id: 'dataset-1',
      name: 'Nightly Dataset',
      subject: 'Regression harness cases',
      active_version_id: 'dataset-version-1',
    },
    dataset_version: {
      id: 'dataset-version-1',
      dataset_id: 'dataset-1',
      version: 'v3',
      item_count: 12,
      created_at: '2026-03-20T09:00:00Z',
      updated_at: '2026-03-20T09:05:00Z',
    },
    group_report: createGroupReport({
      group: {
        summary: {
          counts: {
            queued: 1,
            running: 2,
            passed: 8,
            failed: 2,
          },
          contextpack_breakdown: {
            items_with_snapshot: 2,
            selected_skill_counts: {
              web_query: 1,
              analyze: 1,
            },
            source_trust_counts: {
              official: 3,
              community: 1,
            },
            entry_id_counts: {
              'openai/docs/responses-api': 2,
              'community/forum/contextpack-recipes': 1,
            },
          },
        },
      },
    }),
  }

  return {
    ...base,
    ...overrides,
    eval_run: {
      ...base.eval_run,
      ...((overrides.eval_run as Record<string, unknown>) || {}),
    },
    eval_spec: {
      ...base.eval_spec,
      ...((overrides.eval_spec as Record<string, unknown>) || {}),
    },
    dataset: {
      ...base.dataset,
      ...((overrides.dataset as Record<string, unknown>) || {}),
    },
    dataset_version: {
      ...base.dataset_version,
      ...((overrides.dataset_version as Record<string, unknown>) || {}),
    },
    group_report: createGroupReport(
      (overrides.group_report as Record<string, unknown>) || (base.group_report as Record<string, unknown>)
    ),
  }
}

function createComparisonReport(overrides: Record<string, unknown> = {}) {
  const base = {
    id: 'comparison-1',
    eval_spec_id: 'eval-spec-1',
    base_eval_run_id: 'baseline-run-1',
    target_eval_run_id: 'eval-run-1',
    baseline_id: 'baseline-1',
    created_at: '2026-03-20T10:06:00Z',
    summary: {
      comparison_kind: 'baseline',
      baseline_name: 'Nightly Baseline',
      overall_score_delta: -0.11,
      pass_rate_delta: -0.2,
      verification_pass_rate_delta: -0.25,
      evidence_backed_pass_rate_delta: -0.3,
      retry_recovered_delta: 0,
      contextpack_items_with_snapshot_delta: 2,
      contextpack_selected_skill_delta: {
        web_query: 1,
        analyze: 1,
      },
      contextpack_source_trust_delta: {
        official: 2,
        community: 1,
      },
      contextpack_entry_delta: {
        'openai/docs/responses-api': 2,
        'community/forum/contextpack-recipes': 1,
      },
      regression_count: 1,
      improvement_count: 0,
    },
    regressions: [],
    improvements: [],
  }

  return {
    ...base,
    ...overrides,
    summary: {
      ...base.summary,
      ...((overrides.summary as Record<string, unknown>) || {}),
    },
    regressions: (overrides.regressions as unknown[]) || base.regressions,
    improvements: (overrides.improvements as unknown[]) || base.improvements,
  }
}

async function mountHarnessGroupDetail(width = 1280) {
  routeMock.path = '/automation/harness/group-1'
  routeMock.params = { id: 'group-1' }
  setWindowWidth(width)
  const HarnessGroupDetailView = (await import('@/views/HarnessGroupDetailView.vue')).default
  const wrapper = mount(HarnessGroupDetailView, {
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
  mountedWrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

async function mountHarnessGroupsView() {
  routeMock.path = '/automation/harness'
  routeMock.params = { id: '' }
  const HarnessGroupsView = (await import('@/views/HarnessGroupsView.vue')).default
  const wrapper = mount(HarnessGroupsView, {
    global: {
      plugins: [createTestI18n()],
      stubs: {
        RouterLink: {
          props: ['to'],
          template: '<a class="group-card-link" :data-to="JSON.stringify(to)"><slot /></a>',
        },
      },
    },
  })
  mountedWrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('Harness views', () => {
  beforeEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
    routeMock.path = '/automation/harness/group-1'
    routeMock.params = { id: 'group-1' }
    setWindowWidth(1280)
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: vi.fn(),
    })
    listGroupsMock.mockResolvedValue({ data: [] })
    listDatasetsMock.mockResolvedValue({ data: [] })
    listDatasetVersionsMock.mockResolvedValue({ data: [] })
    listEvalSpecsMock.mockResolvedValue({ data: [] })
    listEvalRunsMock.mockResolvedValue({ data: [] })
    listBaselinesMock.mockResolvedValue({ data: [] })
    getEvalRunReportMock.mockResolvedValue({ data: null })
    compareEvalRunMock.mockResolvedValue({ data: null })
    listConversationsMock.mockResolvedValue({ data: [] })
    listMessagesMock.mockResolvedValue({ data: [] })
    getGroupReportMock.mockResolvedValue({ data: createGroupReport() })
    getRunDetailMock.mockImplementation((id: string) =>
      Promise.resolve({ data: createRunDetail(id) })
    )
    cancelGroupMock.mockResolvedValue({ data: { status: 'cancelled' } })
    retryFailedGroupMock.mockResolvedValue({ data: { retried: 2 } })
  })

  afterEach(() => {
    while (mountedWrappers.length) {
      mountedWrappers.pop()?.unmount()
    }
    vi.useRealTimers()
  })

  it('renders the restored harness console and filters group records', async () => {
    routeMock.path = '/automation/harness'
    routeMock.params = { id: '' }
    listGroupsMock.mockResolvedValue({
      data: [
        {
          id: 'group-1',
          kind: 'eval',
          title: 'Regression batch',
          status: 'running',
          subject: 'Validate agent task flows',
          owner_user_id: 'owner-1',
          summary: {
            item_count: 12,
            pass_rate: 0.75,
            overall_score: 0.82,
            counts: {
              running: 2,
              passed: 9,
              failed: 1,
            },
            contextpack_breakdown: {
              items_with_snapshot: 3,
            },
          },
          created_at: '2026-03-20T10:00:00Z',
          updated_at: '2026-03-20T10:05:00Z',
        },
        {
          id: 'group-2',
          kind: 'experiment',
          title: 'Nightly snapshot',
          status: 'completed',
          subject: 'Nightly evaluation',
          owner_user_id: 'owner-2',
          summary: {
            item_count: 4,
            pass_rate: 1,
            overall_score: 0.95,
            counts: {
              passed: 4,
            },
          },
          created_at: '2026-03-20T11:00:00Z',
          updated_at: '2026-03-20T11:05:00Z',
        },
      ],
    })

    const HarnessGroupsView = (await import('@/views/HarnessGroupsView.vue')).default
    const wrapper = shallowMount(HarnessGroupsView, {
      global: {
        plugins: [createTestI18n()],
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a class="group-card-link" :data-to="JSON.stringify(to)"><slot /></a>',
          },
        },
      },
    })
    mountedWrappers.push(wrapper)

    await flushPromises()

    expect(listGroupsMock).toHaveBeenCalled()
    expect(listDatasetsMock).toHaveBeenCalled()
    expect(listEvalRunsMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Harness')
    expect(wrapper.text()).toContain('Quick Eval')
    expect(wrapper.text()).toContain('Datasets & versions')
    expect(wrapper.text()).toContain('Eval runs')
    expect(wrapper.text()).toContain('Regression batch')
    expect(wrapper.text()).toContain('Nightly snapshot')
    expect(wrapper.text()).toContain('Context Packs: 3')
    expect(wrapper.findAll('.group-card-link')).toHaveLength(2)

    await wrapper.get('input[type="search"]').setValue('nightly')
    expect(wrapper.findAll('.group-card-link')).toHaveLength(1)
    expect(wrapper.text()).toContain('Nightly snapshot')
    expect(wrapper.text()).not.toContain('Regression batch')

    const terminalButton = wrapper
      .findAll('button')
      .find((button) => button.text().trim() === 'Terminal')

    expect(terminalButton).toBeTruthy()

    await terminalButton!.trigger('click')
    expect(wrapper.findAll('.group-card-link')).toHaveLength(1)
    expect(wrapper.text()).toContain('Nightly snapshot')
  })

  it('renders context pack summary inside the selected eval run linked report', async () => {
    listGroupsMock.mockResolvedValue({ data: [] })
    listDatasetsMock.mockResolvedValue({
      data: [
        {
          id: 'dataset-1',
          name: 'Nightly Dataset',
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
          version: 'v3',
          item_count: 12,
          created_at: '2026-03-20T09:00:00Z',
          updated_at: '2026-03-20T09:05:00Z',
        },
      ],
    })
    listEvalSpecsMock.mockResolvedValue({
      data: [
        {
          id: 'eval-spec-1',
          dataset_id: 'dataset-1',
          dataset_version_id: 'dataset-version-1',
          name: 'Nightly Spec',
          run_kind: 'agent_task',
          profile: 'default',
          scoring_config: {
            mode: 'rule',
            pass_threshold: 0.7,
          },
          created_at: '2026-03-20T09:10:00Z',
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
          title: 'Nightly Eval Run',
          status: 'running',
          trigger_kind: 'manual',
          summary: {
            pass_rate: 0.8,
            overall_score: 0.61,
          },
          created_at: '2026-03-20T10:00:00Z',
          updated_at: '2026-03-20T10:05:00Z',
        },
      ],
    })
    getEvalRunReportMock.mockResolvedValue({
      data: createEvalRunReport(),
    })

    const wrapper = await mountHarnessGroupsView()

    expect(wrapper.text()).toContain('Nightly Eval Run')

    await wrapper.get('.eval-run-card').trigger('click')
    await flushPromises()

    expect(getEvalRunReportMock).toHaveBeenCalledWith('eval-run-1')
    expect(wrapper.text()).toContain('Linked report')
    expect(wrapper.text()).toContain('Context Packs')
    expect(wrapper.text()).toContain('Items with packs')
    expect(wrapper.text()).toContain('Selected skills')
    expect(wrapper.text()).toContain('Sources')
    expect(wrapper.text()).toContain('Entries')
    expect(wrapper.text()).toContain('openai/docs/responses-api')
    expect(wrapper.text()).toContain('community/forum/contextpack-recipes')
    expect(wrapper.text()).toContain('official')
    expect(wrapper.text()).toContain('web_query')
  })

  it('renders context pack deltas inside the comparison report', async () => {
    listGroupsMock.mockResolvedValue({ data: [] })
    listDatasetsMock.mockResolvedValue({
      data: [
        {
          id: 'dataset-1',
          name: 'Nightly Dataset',
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
          version: 'v3',
          item_count: 12,
          created_at: '2026-03-20T09:00:00Z',
          updated_at: '2026-03-20T09:05:00Z',
        },
      ],
    })
    listEvalSpecsMock.mockResolvedValue({
      data: [
        {
          id: 'eval-spec-1',
          dataset_id: 'dataset-1',
          dataset_version_id: 'dataset-version-1',
          name: 'Nightly Spec',
          run_kind: 'agent_task',
          profile: 'default',
          scoring_config: {
            mode: 'rule',
            pass_threshold: 0.7,
          },
          created_at: '2026-03-20T09:10:00Z',
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
          title: 'Nightly Eval Run',
          status: 'running',
          trigger_kind: 'manual',
          summary: {
            pass_rate: 0.8,
            overall_score: 0.61,
          },
          created_at: '2026-03-20T10:00:00Z',
          updated_at: '2026-03-20T10:05:00Z',
        },
      ],
    })
    getEvalRunReportMock.mockResolvedValue({
      data: createEvalRunReport(),
    })
    compareEvalRunMock.mockResolvedValue({
      data: createComparisonReport(),
    })

    const wrapper = await mountHarnessGroupsView()

    await wrapper.get('.eval-run-card').trigger('click')
    await flushPromises()

    const compareButton = wrapper
      .findAll('button')
      .find((button) => button.text().trim() === 'Generate comparison')

    expect(compareButton).toBeTruthy()

    await compareButton!.trigger('click')
    await flushPromises()

    expect(compareEvalRunMock).toHaveBeenCalledWith('eval-run-1', {})
    expect(wrapper.text()).toContain('Context pack delta')
    expect(wrapper.text()).toContain('Skill delta')
    expect(wrapper.text()).toContain('Source delta')
    expect(wrapper.text()).toContain('Entry delta')
    expect(wrapper.text()).toContain('openai/docs/responses-api +2')
    expect(wrapper.text()).toContain('community/forum/contextpack-recipes +1')
    expect(wrapper.text()).toContain('web_query +1')
    expect(wrapper.text()).toContain('official +2')
  })

  it('renders the hybrid harness detail report with run graph and summary', async () => {
    const wrapper = await mountHarnessGroupDetail()

    expect(getGroupReportMock).toHaveBeenCalledWith('group-1')
    expect(wrapper.find('.harness-detail-page').exists()).toBe(true)
    expect(wrapper.text()).toContain('Regression batch')
    expect(wrapper.text()).toContain('Validate agent task flows')
    expect(wrapper.text()).toContain('Score distribution')
    expect(wrapper.text()).toContain('Failed items')
    expect(wrapper.text()).toContain('Items')
    expect(wrapper.text()).toContain('Run graph')
    expect(wrapper.text()).toContain('Execution summary')
    expect(wrapper.text()).toContain('Detached workers')
    expect(wrapper.text()).toContain('Artifacts')
    expect(wrapper.text()).toContain('stderr.log')

    const backLink = wrapper.find('.back-link')
    expect(backLink.exists()).toBe(true)
    expect(backLink.attributes('data-to')).toBe(JSON.stringify({ name: 'HarnessGroups' }))
  })

  it('renders aggregated context pack breakdown in the group detail summary', async () => {
    getGroupReportMock.mockResolvedValueOnce({
      data: createGroupReport({
        group: {
          summary: {
            counts: {
              queued: 1,
              running: 2,
              passed: 8,
              failed: 2,
            },
            failure_label_counts: {
              missing_artifact: 2,
            },
            contextpack_breakdown: {
              items_with_snapshot: 2,
              selected_skill_counts: {
                web_query: 1,
                analyze: 1,
              },
              source_trust_counts: {
                official: 3,
                community: 1,
              },
              entry_id_counts: {
                'openai/docs/responses-api': 2,
                'community/forum/contextpack-recipes': 1,
              },
            },
          },
        },
      }),
    })

    const wrapper = await mountHarnessGroupDetail()

    expect(wrapper.text()).toContain('Context Packs')
    expect(wrapper.text()).toContain('Items with packs')
    expect(wrapper.text()).toContain('Selected skills')
    expect(wrapper.text()).toContain('Sources')
    expect(wrapper.text()).toContain('Entries')
    expect(wrapper.text()).toContain('openai/docs/responses-api')
    expect(wrapper.text()).toContain('community/forum/contextpack-recipes')
    expect(wrapper.text()).toContain('official')
    expect(wrapper.text()).toContain('web_query')
    expect(wrapper.text()).toContain('analyze')
  })

  it('shows a worker batch summary and expands grouped workers on demand', async () => {
    const wrapper = await mountHarnessGroupDetail()

    expect(wrapper.text()).toContain('Spawning 4 workers')
    expect(wrapper.find('[data-run-id="run-fetch"]').exists()).toBe(false)
    expect(wrapper.find('[data-run-id="run-verify"]').exists()).toBe(false)

    await wrapper.get('.batch-summary-card').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-run-id="run-fetch"]').exists()).toBe(true)
    expect(wrapper.find('[data-run-id="run-verify"]').exists()).toBe(true)
  })

  it('loads run detail when a tree node is selected', async () => {
    const wrapper = await mountHarnessGroupDetail()

    await wrapper.get('[data-run-id="run-review"]').trigger('click')
    await flushPromises()

    expect(getRunDetailMock).toHaveBeenCalledWith('run-review')
    expect(wrapper.find('.detail-drawer').text()).toContain('Run summary')
    expect(wrapper.find('.detail-drawer').text()).toContain('worker.review')
    expect(wrapper.find('.detail-drawer').text()).toContain('Runtime trace')
    expect(wrapper.find('.detail-drawer').text()).toContain('Finalize')
    expect(wrapper.find('.detail-drawer').text()).toContain('1.54 s')
  })

  it('opens the correct run from the failed items inspect action', async () => {
    const wrapper = await mountHarnessGroupDetail()

    await wrapper.get('.failed-card .inspect-run-button').trigger('click')
    await flushPromises()

    expect(getRunDetailMock).toHaveBeenCalledWith('run-review')
    expect(wrapper.find('.detail-drawer').text()).toContain('artifact missing')
  })

  it('renders detached workers in a separate section', async () => {
    const wrapper = await mountHarnessGroupDetail()

    expect(wrapper.find('#detached-workers').exists()).toBe(true)
    expect(wrapper.find('[data-run-id="run-orphan"]').exists()).toBe(true)
  })

  it('opens a bottom sheet on narrow screens', async () => {
    const wrapper = await mountHarnessGroupDetail(640)

    await wrapper.get('[data-run-id="run-review"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('.detail-sheet').exists()).toBe(true)
    expect(wrapper.find('.detail-drawer').exists()).toBe(false)
  })

  it('silently refreshes detail for an active selected run on the group timer', async () => {
    vi.useFakeTimers()
    const wrapper = await mountHarnessGroupDetail()

    await wrapper.get('[data-run-id="run-root"]').trigger('click')
    await flushPromises()

    expect(getRunDetailMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()

    expect(getGroupReportMock).toHaveBeenCalledTimes(2)
    expect(getRunDetailMock).toHaveBeenCalledTimes(2)
    expect(getRunDetailMock).toHaveBeenLastCalledWith('run-root')
    expect(wrapper.find('.detail-drawer').text()).toContain('coordinator.main')
  })

  it('does not refresh detail for a terminal selected run on the group timer', async () => {
    vi.useFakeTimers()
    const wrapper = await mountHarnessGroupDetail()

    await wrapper.get('[data-run-id="run-review"]').trigger('click')
    await flushPromises()

    expect(getRunDetailMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()

    expect(getGroupReportMock).toHaveBeenCalledTimes(2)
    expect(getRunDetailMock).toHaveBeenCalledTimes(1)
  })

  it('shows only the latest timeline events by default and expands on demand', async () => {
    const wrapper = await mountHarnessGroupDetail()

    await wrapper.get('[data-run-id="run-review"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.detail-drawer .event-card')).toHaveLength(5)
    expect(wrapper.find('.detail-drawer').text()).toContain('Show all events')

    const expandButton = wrapper
      .findAll('.detail-drawer button')
      .find((button) => button.text().trim() === 'Show all events')

    expect(expandButton).toBeTruthy()

    await expandButton!.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.detail-drawer .event-card')).toHaveLength(6)
    expect(wrapper.find('.detail-drawer').text()).toContain('Show fewer events')
  })

  it('retries failed items from the hybrid detail view', async () => {
    const wrapper = await mountHarnessGroupDetail()

    const retryButton = wrapper
      .findAll('button')
      .find((button) => button.text().trim() === 'Retry failed')

    expect(retryButton).toBeTruthy()

    await retryButton!.trigger('click')
    await flushPromises()

    expect(retryFailedGroupMock).toHaveBeenCalledWith('group-1')
    expect(notificationSuccessMock).toHaveBeenCalledWith('Harness', 'Failed items requeued.')
    expect(getGroupReportMock).toHaveBeenCalledTimes(2)
  })
})
