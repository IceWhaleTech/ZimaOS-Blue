import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useDeepResearchJobsStore } from '@/stores/deepResearchJobs'
import { deepResearchApi } from '@/api/deepResearch'

const mocks = vi.hoisted(() => ({
  routerPush: vi.fn(),
  chatStore: {
    selectConversation: vi.fn(),
  },
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  },
}))

vi.mock('@/router', () => ({
  default: {
    push: (...args: unknown[]) => mocks.routerPush(...args),
  },
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mocks.chatStore,
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/api/deepResearch', () => ({
  deepResearchApi: {
    listJobs: vi.fn(),
    getJob: vi.fn(),
    cancelJob: vi.fn(),
  },
}))

describe('deepResearchJobs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mocks.routerPush.mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockResolvedValue(undefined)
    vi.mocked(deepResearchApi.listJobs).mockResolvedValue({ data: [] } as never)
    vi.mocked(deepResearchApi.cancelJob).mockResolvedValue({
      data: { status: 'cancelled' },
    } as never)
  })

  it('hydrates active jobs and preserves conversation ids', async () => {
    vi.mocked(deepResearchApi.listJobs).mockResolvedValue({
      data: [
        {
          id: 'job-1',
          job_id: 'job-1',
          query: 'research topic',
          status: 'running',
          stage: 'verify',
          progress: 65,
          iteration: 2,
          latest_action: 'verification_completed',
          latest_gap: 'Need primary source',
          conversation_id: 'conv-1',
          updated_at: '2026-03-11T00:00:00.000Z',
        },
      ],
    } as never)

    const store = useDeepResearchJobsStore()
    await store.fetchActiveJobs()

    expect(store.activeJobs).toHaveLength(1)
    expect(store.activeJobs[0].job_id).toBe('job-1')
    expect(store.activeJobs[0].conversation_id).toBe('conv-1')
    expect(store.hydrated).toBe(true)
  })

  it('removes terminal jobs from dock and fires toast once', () => {
    const store = useDeepResearchJobsStore()
    store.applyJobSnapshot({
      id: 'job-1',
      job_id: 'job-1',
      query: 'research topic',
      status: 'running',
      stage: 'retrieve',
      progress: 40,
      iteration: 1,
      conversation_id: 'conv-1',
      updated_at: '2026-03-11T00:00:00.000Z',
    })

    store.handleGlobalEvent('deep_research.job_completed', {
      job_id: 'job-1',
      query: 'research topic',
      status: 'completed',
      stage: 'completed',
      progress: 100,
      conversation_id: 'conv-1',
      updated_at: '2026-03-11T00:01:00.000Z',
    })
    store.handleGlobalEvent('deep_research.job_completed', {
      job_id: 'job-1',
      query: 'research topic',
      status: 'completed',
      stage: 'completed',
      progress: 100,
      conversation_id: 'conv-1',
      updated_at: '2026-03-11T00:01:01.000Z',
    })

    expect(store.activeJobs).toHaveLength(0)
    expect(mocks.notificationStore.success).toHaveBeenCalledTimes(1)
  })

  it('opens the owning conversation and queues focus', async () => {
    const store = useDeepResearchJobsStore()

    await store.openJob('job-9', 'conv-9')

    expect(store.pendingFocusJobId).toBe('job-9')
    expect(mocks.routerPush).toHaveBeenCalledWith({
      name: 'Chat',
      query: { conversationId: 'conv-9' },
    })
    expect(mocks.chatStore.selectConversation).toHaveBeenCalledWith('conv-9')

    store.consumePendingFocusJobId()
    expect(store.pendingFocusJobId).toBeNull()
  })

  it('cancels a job and updates local snapshot', async () => {
    const store = useDeepResearchJobsStore()
    store.applyJobSnapshot({
      id: 'job-2',
      job_id: 'job-2',
      query: 'cancel me',
      status: 'running',
      stage: 'verify',
      progress: 60,
      updated_at: '2026-03-11T00:00:00.000Z',
    })

    await store.cancelJob('job-2')

    expect(deepResearchApi.cancelJob).toHaveBeenCalledWith('job-2')
    expect(store.jobMap['job-2']?.status).toBe('cancelled')
    expect(store.activeJobs).toHaveLength(0)
  })
})
