import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import KnowledgeView from '@/views/KnowledgeView.vue'
import { i18n } from '@/i18n'
import { knowledgeApi } from '@/api/knowledge'

const routeState = {
  query: {} as Record<string, string>,
}
const routerReplace = vi.fn()

const runJobMock = vi.fn()

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
  useKnowledgeJobs: () => ({
    currentJob: ref(null),
    latestReport: ref(null),
    isRunning: ref(false),
    runJob: runJobMock,
    hydrateJob: vi.fn(),
    hydrateReport: vi.fn(),
  }),
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

  it('renders wiki-oriented overview state and forwards query jobs with scope controls', async () => {
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

    expect(wrapper.text()).toContain('Knowledge Space')
    expect(wrapper.text()).toContain('Recent activity')
    expect(wrapper.text()).toContain('Knowledge map')
    expect(wrapper.text()).toContain('2 pages')
    expect(wrapper.text()).toContain('1 unresolved conflicts')
    expect(wrapper.text()).toContain('1 open gaps')
    expect(wrapper.text()).toContain('Blue Knowledge')
    expect(wrapper.text()).toContain('README.md')
    expect(wrapper.text()).toContain('Knowledge schema updated')
    expect(
      (wrapper.get('[data-testid="knowledge-schema-editor"]').element as HTMLTextAreaElement).value
    ).toBe('# Knowledge Space Schema\n\nKeep syntheses durable.\n')

    await wrapper
      .get('[data-testid="knowledge-query-input"]')
      .setValue('How does Blue knowledge compilation work?')
    await wrapper.get('[data-testid="knowledge-query-scope"]').setValue('selected_sources')
    await wrapper.get('[data-testid="knowledge-query-button"]').trigger('click')

    expect(runJobMock).toHaveBeenCalledWith({
      kind: 'answer',
      query: 'How does Blue knowledge compilation work?',
      page_slug: 'readme',
      archive_answer: true,
      query_scope: 'selected_sources',
      selected_refs: ['README.md'],
    })
  })
})
