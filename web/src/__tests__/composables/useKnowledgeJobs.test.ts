import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

const mocks = vi.hoisted(() => {
  const handlers = new Map<string, (payload: unknown) => void>()
  return {
    handlers,
    onSSEEvent: vi.fn((event: string, handler: (payload: unknown) => void) => {
      handlers.set(event, handler)
    }),
    offSSEEvent: vi.fn((event: string) => {
      handlers.delete(event)
    }),
  }
})

vi.mock('@/composables/useEventStream', () => ({
  onSSEEvent: (...args: unknown[]) => mocks.onSSEEvent(...args),
  offSSEEvent: (...args: unknown[]) => mocks.offSSEEvent(...args),
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

import { knowledgeApi } from '@/api/knowledge'
import { useKnowledgeJobs } from '@/composables/useKnowledgeJobs'

describe('useKnowledgeJobs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.handlers.clear()
    vi.mocked(knowledgeApi.listJobs).mockResolvedValue({ data: [] } as never)
    vi.mocked(knowledgeApi.getJob).mockResolvedValue({ data: null } as never)
  })

  it('hydrates the newest active job by kind and keeps SSE updates flowing after re-entry', async () => {
    vi.mocked(knowledgeApi.listJobs).mockResolvedValue({
      data: [
        {
          id: 'job-answer',
          job_id: 'job-answer',
          kind: 'answer',
          status: 'running',
          progress: 12,
          stage: 'running',
          detail: 'querying',
          updated_at: '2026-04-11T10:00:00Z',
        },
        {
          id: 'job-repair',
          job_id: 'job-repair',
          kind: 'repair_conflicts',
          status: 'running',
          progress: 61,
          stage: 'resolve_group',
          detail: 'architecture, readme',
          updated_at: '2026-04-11T10:01:00Z',
        },
      ],
    } as never)
    vi.mocked(knowledgeApi.getJob).mockResolvedValue({
      data: {
        id: 'job-repair',
        job_id: 'job-repair',
        kind: 'repair_conflicts',
        status: 'running',
        progress: 61,
        stage: 'resolve_group',
        detail: 'architecture, readme',
        updated_at: '2026-04-11T10:01:00Z',
        created_at: '2026-04-11T10:00:00Z',
      },
    } as never)

    let jobs!: ReturnType<typeof useKnowledgeJobs>
    const Harness = defineComponent({
      setup() {
        jobs = useKnowledgeJobs()
        return () => h('div')
      },
    })

    const wrapper = mount(Harness)

    await jobs.hydrateLatestActiveJob({ kinds: ['repair_conflicts'] })

    expect(knowledgeApi.listJobs).toHaveBeenCalledWith('active')
    expect(knowledgeApi.getJob).toHaveBeenCalledWith('job-repair')
    expect(jobs.currentJob.value).toMatchObject({
      id: 'job-repair',
      kind: 'repair_conflicts',
      progress: 61,
      detail: 'architecture, readme',
    })

    const progressHandler = mocks.handlers.get('knowledge.job_progress')
    expect(progressHandler).toBeTypeOf('function')

    progressHandler?.({
      id: 'job-repair',
      job_id: 'job-repair',
      kind: 'repair_conflicts',
      status: 'running',
      progress: 74,
      stage: 'write_pages',
      detail: 'readme',
      updated_at: '2026-04-11T10:01:30Z',
      created_at: '2026-04-11T10:00:00Z',
    })

    expect(jobs.currentJob.value).toMatchObject({
      id: 'job-repair',
      progress: 74,
      stage: 'write_pages',
      detail: 'readme',
    })

    wrapper.unmount()
  })
})
