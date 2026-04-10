import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import KnowledgeView from '@/views/KnowledgeView.vue'
import { i18n } from '@/i18n'
import { knowledgeApi } from '@/api/knowledge'
import automationKnowledgeEvolutionBackfills from '@/i18n/automation-knowledge-evolution-backfills'

const routeState = {
  query: {} as Record<string, string>,
}
const routerReplace = vi.fn()

const runJobMock = vi.fn()
const maintenanceCurrentJob = ref<any>(null)
const maintenanceLatestReport = ref(null)
const maintenanceIsRunning = ref(false)
const queryCurrentJob = ref<any>(null)
const queryLatestReport = ref(null)
const queryIsRunning = ref(false)
let useKnowledgeJobsCallCount = 0

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => routeState,
    useRouter: () => ({
      replace: routerReplace,
      push: vi.fn(),
    }),
  }
})

vi.mock('@/components/KnowledgeManagerCard.vue', () => ({
  default: {
    name: 'KnowledgeManagerCard',
    props: ['latestLint'],
    template:
      '<div class="knowledge-manager-card-stub">{{ latestLint?.issues?.length ?? 0 }} issues</div>',
  },
}))

vi.mock('@/composables/useKnowledgeJobs', () => ({
  useKnowledgeJobs: () => {
    useKnowledgeJobsCallCount += 1
    const isMaintenance = useKnowledgeJobsCallCount % 2 === 1
    return {
      currentJob: isMaintenance ? maintenanceCurrentJob : queryCurrentJob,
      latestReport: isMaintenance ? maintenanceLatestReport : queryLatestReport,
      isRunning: isMaintenance ? maintenanceIsRunning : queryIsRunning,
      runJob: runJobMock,
      hydrateJob: vi.fn(),
      hydrateReport: vi.fn(),
    }
  },
}))

vi.mock('@/api/knowledge', () => ({
  knowledgeApi: {
    createJob: vi.fn(),
    listJobs: vi.fn(),
    getJob: vi.fn(),
    getReport: vi.fn(),
    cancelJob: vi.fn(),
    listPages: vi.fn(),
    getPage: vi.fn(),
    getIndex: vi.fn(),
    getSchema: vi.fn(),
    updateSchema: vi.fn(),
    getLog: vi.fn(),
    promoteQuery: vi.fn(),
    getLatestLint: vi.fn(),
  },
}))

describe('KnowledgeView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeState.query = {}
    useKnowledgeJobsCallCount = 0
    maintenanceCurrentJob.value = null
    maintenanceLatestReport.value = null
    maintenanceIsRunning.value = false
    queryCurrentJob.value = null
    queryLatestReport.value = null
    queryIsRunning.value = false

    vi.mocked(knowledgeApi.listPages).mockResolvedValue({
      data: [
        {
          title: 'Blue Knowledge',
          slug: 'readme',
          page_type: 'source_summary',
          summary: 'Compiled entry page.',
          source_refs: ['README.md'],
          keywords: ['blue', 'knowledge'],
          backlinks: ['architecture'],
          generated_at: '2026-04-05T12:00:00Z',
          updated_at: '2026-04-05T12:00:00Z',
          source_hash: 'hash-1',
          status: 'active',
          confidence: 'low',
          conflicts_with: [],
          superseded_by: [],
          derived_from_query: '',
        },
        {
          title: 'Blue Architecture',
          slug: 'architecture',
          page_type: 'source_summary',
          summary: 'Architecture detail.',
          source_refs: ['ARCHITECTURE.md'],
          keywords: ['architecture'],
          backlinks: ['readme'],
          generated_at: '2026-04-05T12:00:00Z',
          updated_at: '2026-04-05T12:00:00Z',
          source_hash: 'hash-2',
          status: 'conflicted',
          confidence: 'low',
          conflicts_with: ['readme'],
          superseded_by: [],
          derived_from_query: '',
        },
      ],
    } as never)

    vi.mocked(knowledgeApi.getLatestLint).mockResolvedValue({
      data: {
        generated_at: '2026-04-05T12:00:00Z',
        issues: [
          {
            kind: 'missing_concept_page',
            message: 'knowledge space has no concept pages yet',
            category: 'research_suggestions',
            severity: 'medium',
          },
          {
            kind: 'conflicting_claim',
            message: 'readme conflicts with architecture',
            category: 'review_required',
            severity: 'high',
            related_pages: ['readme', 'architecture'],
          },
        ],
      },
    } as never)

    vi.mocked(knowledgeApi.getSchema).mockResolvedValue({
      data: {
        content: '# Knowledge Space Schema\n\nKeep syntheses durable.\n',
      },
    } as never)

    vi.mocked(knowledgeApi.getLog).mockResolvedValue({
      data: [
        {
          timestamp: '2026-04-05T12:15:00Z',
          operation: 'schema',
          title: 'Knowledge schema updated',
          reason: 'schema content updated',
        },
        {
          timestamp: '2026-04-05T12:10:00Z',
          operation: 'ingest',
          title: 'Knowledge ingest',
          sources: ['README.md'],
          new_pages: ['readme'],
          updated_pages: [],
          conflicts: [],
          gaps: ['Missing concept pages for recurring topics'],
          reason: 'ingest updated the knowledge space',
        },
      ],
    } as never)

    vi.mocked(knowledgeApi.getPage).mockImplementation(async (slug: string) => ({
      data: {
        title: slug === 'architecture' ? 'Blue Architecture' : 'Blue Knowledge',
        slug,
        page_type: 'source_summary',
        summary: slug === 'architecture' ? 'Architecture detail.' : 'Compiled entry page.',
        source_refs: [slug === 'architecture' ? 'ARCHITECTURE.md' : 'README.md'],
        keywords: [slug],
        backlinks: slug === 'architecture' ? ['readme'] : ['architecture'],
        generated_at: '2026-04-05T12:00:00Z',
        updated_at: '2026-04-05T12:00:00Z',
        source_hash: `hash-${slug}`,
        status: slug === 'architecture' ? 'conflicted' : 'active',
        confidence: 'low',
        conflicts_with: slug === 'architecture' ? ['readme'] : [],
        superseded_by: [],
        derived_from_query: '',
        content: `# ${slug}\n\nCompiled markdown`,
        answers:
          slug === 'readme'
            ? [
                {
                  title: 'Knowledge Answer',
                  path: '/tmp/workspace/knowledge/answers/readme.md',
                  page_slug: 'readme',
                  query: 'How does this work?',
                  summary: 'A grounded answer.',
                  generated_at: '2026-04-05T12:10:00Z',
                },
              ]
            : [],
      },
    })) as never
  })

  it('renders the lighter knowledge workspace layout and keeps maintenance tools behind a secondary toggle', async () => {
    runJobMock.mockResolvedValue({
      id: 'job-answer',
      job_id: 'job-answer',
      kind: 'answer',
      status: 'pending',
      progress: 0,
      updated_at: '2026-04-05T12:00:00Z',
      created_at: '2026-04-05T12:00:00Z',
    })

    const wrapper = mount(KnowledgeView, {
      global: {
        plugins: [i18n],
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Knowledge map')
    expect(wrapper.text()).toContain('Query')
    expect(wrapper.text()).toContain('Blue Knowledge')
    expect(wrapper.findAll('[data-graph-node="true"]').length).toBe(2)
    expect(wrapper.findAll('[data-testid="knowledge-graph-edge"]').length).toBe(2)
    expect(
      wrapper
        .get('[data-testid="knowledge-graph-node-architecture"]')
        .attributes('data-label-visible')
    ).toBe('true')
    expect(
      wrapper.get('[data-testid="knowledge-graph-node-readme"]').attributes('data-label-visible')
    ).toBe('true')
    expect(wrapper.get('[data-testid="knowledge-graph-focus-card"]').text()).toContain(
      'Blue Architecture'
    )
    expect(vi.mocked(knowledgeApi.getPage)).toHaveBeenNthCalledWith(1, 'architecture')
    expect(wrapper.get('[data-testid="knowledge-ingest-button"]').text()).toContain('Ingest')
    expect(wrapper.get('[data-testid="knowledge-lint-button"]').text()).toContain('Lint')
    expect(wrapper.get('[data-testid="knowledge-repair-conflicts-button"]').text()).toContain(
      'Repair conflicts'
    )
    expect(wrapper.find('[data-testid="knowledge-summary-value-pages"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="knowledge-panel-log"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="knowledge-schema-editor"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Architecture detail.')

    await wrapper.get('[data-testid="knowledge-ingest-button"]').trigger('click')
    expect(runJobMock).toHaveBeenNthCalledWith(1, { kind: 'ingest' })

    await wrapper.get('[data-testid="knowledge-lint-button"]').trigger('click')
    expect(runJobMock).toHaveBeenNthCalledWith(2, { kind: 'lint' })

    await wrapper.get('[data-testid="knowledge-repair-conflicts-button"]').trigger('click')
    expect(runJobMock).toHaveBeenNthCalledWith(3, { kind: 'repair_conflicts' })

    await wrapper.get('[data-testid="knowledge-maintenance-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Recent activity')
    expect(wrapper.get('[data-testid="knowledge-maintenance-panel"]').text()).toContain(
      'Ingest and maintain Blue knowledge'
    )
    expect(wrapper.text()).toContain('Blue Architecture')
    expect(wrapper.text()).toContain('ARCHITECTURE.md')
    expect(wrapper.text()).toContain('Knowledge schema updated')
    expect(
      (wrapper.get('[data-testid="knowledge-schema-editor"]').element as HTMLTextAreaElement).value
    ).toBe('# Knowledge Space Schema\n\nKeep syntheses durable.\n')

    await wrapper
      .get('[data-testid="knowledge-query-input"]')
      .setValue('How does Blue knowledge compilation work?')
    await wrapper.get('[data-testid="knowledge-query-scope"]').setValue('selected_sources')
    await wrapper.get('[data-testid="knowledge-query-button"]').trigger('click')

    expect(runJobMock).toHaveBeenNthCalledWith(4, {
      kind: 'answer',
      query: 'How does Blue knowledge compilation work?',
      page_slug: 'architecture',
      archive_answer: true,
      query_scope: 'selected_sources',
      selected_refs: ['ARCHITECTURE.md'],
    })
  })

  it('stores translated schema labels for Chinese locales', () => {
    const zhCN = automationKnowledgeEvolutionBackfills['zh-CN'] as {
      knowledge?: Record<string, string>
    }
    const zhTW = automationKnowledgeEvolutionBackfills['zh-TW'] as {
      knowledge?: Record<string, string>
    }

    expect(zhCN.knowledge?.schemaTitle).toBe('结构定义')
    expect(zhCN.knowledge?.openSchema).toBe('打开结构定义')
    expect(zhCN.knowledge?.saveSchema).toBe('保存结构定义')
    expect(zhCN.knowledge?.active).toBe('当前')
    expect(zhCN.knowledge?.repairConflicts).toBe('修复冲突')
    expect(zhTW.knowledge?.schemaTitle).toBe('結構定義')
    expect(zhTW.knowledge?.active).toBe('當前')
    expect(zhTW.knowledge?.repairConflicts).toBe('修復衝突')
  })

  it('lets knowledge page groups collapse and expand independently', async () => {
    const wrapper = mount(KnowledgeView, {
      global: {
        plugins: [i18n],
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    const toggle = wrapper.get('[data-testid="knowledge-group-toggle-source_summary"]')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('[data-testid="knowledge-group-panel-source_summary"]').exists()).toBe(false)

    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('[data-testid="knowledge-group-panel-source_summary"]').text()).toContain(
      'Blue Knowledge'
    )
    expect(toggle.text()).toContain('2')

    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('[data-testid="knowledge-group-panel-source_summary"]').exists()).toBe(false)
  })

  it('shows live conflict repair progress details while the repair job is running', async () => {
    maintenanceCurrentJob.value = {
      id: 'job-repair',
      job_id: 'job-repair',
      kind: 'repair_conflicts',
      status: 'running',
      progress: 42,
      stage: 'resolve_group',
      detail: 'architecture, readme',
      updated_at: '2026-04-05T12:01:00Z',
      created_at: '2026-04-05T12:00:00Z',
    }
    maintenanceIsRunning.value = true

    const wrapper = mount(KnowledgeView, {
      global: {
        plugins: [i18n],
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    const progressCard = wrapper.get('[data-testid="knowledge-repair-progress"]')
    expect(progressCard.text()).toContain('42%')
    expect(progressCard.text()).toContain('architecture, readme')
  })
})
