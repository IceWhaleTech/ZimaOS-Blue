import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import KnowledgeManagerCard from '@/components/KnowledgeManagerCard.vue'
import { i18n } from '@/i18n'
import { knowledgeApi } from '@/api/knowledge'

const routerPush = vi.fn()

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRouter: () => ({ push: routerPush }),
  }
})

vi.mock('@/composables/useEventStream', () => ({
  onSSEEvent: vi.fn(),
  offSSEEvent: vi.fn(),
}))

vi.mock('@/api/knowledge', () => ({
  knowledgeApi: {
    createJob: vi.fn(),
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

function pendingJob(kind: 'ingest' | 'lint') {
  return {
    data: {
      id: `job-${kind}`,
      job_id: `job-${kind}`,
      kind,
      status: 'pending',
      progress: 0,
      updated_at: '2026-04-05T12:00:00Z',
      created_at: '2026-04-05T12:00:00Z',
    },
  } as never
}

describe('KnowledgeManagerCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(knowledgeApi.createJob).mockResolvedValue(pendingJob('ingest'))
  })

  it('starts ingest and lint jobs with the expected payloads and shows lint state', async () => {
    const wrapper = mount(KnowledgeManagerCard, {
      props: {
        latestLint: {
          generated_at: '2026-04-05T12:00:00Z',
          issues: [
            { kind: 'stale_hash', message: 'README changed.' },
            { kind: 'missing_backlinks', message: 'Backlinks missing.' },
          ],
        },
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.text()).toContain('2 lint issues need review.')

    await wrapper.get('[data-testid="knowledge-ingest-button"]').trigger('click')
    expect(knowledgeApi.createJob).toHaveBeenCalledWith({ kind: 'ingest' })

    wrapper.unmount()
    vi.mocked(knowledgeApi.createJob).mockResolvedValueOnce(pendingJob('lint'))

    const lintWrapper = mount(KnowledgeManagerCard, {
      global: {
        plugins: [i18n],
      },
    })

    await lintWrapper.get('[data-testid="knowledge-lint-button"]').trigger('click')
    expect(knowledgeApi.createJob).toHaveBeenLastCalledWith({ kind: 'lint' })
  })

  it('opens the knowledge lane inside evolution from the browse action', async () => {
    const wrapper = mount(KnowledgeManagerCard, {
      global: {
        plugins: [i18n],
      },
    })

    await wrapper.get('[data-testid="knowledge-open-button"]').trigger('click')
    expect(routerPush).toHaveBeenCalledWith({
      name: 'Evolution',
      query: { pane: 'knowledge' },
    })
  })
})
