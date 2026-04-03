import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useDeepResearchJobsStore } from '@/stores/deepResearchJobs'
import { deepResearchApi } from '@/api/deepResearch'

const mocks = vi.hoisted(() => ({
  routerPush: vi.fn(),
  onSSEEvent: vi.fn(),
  offSSEEvent: vi.fn(),
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

vi.mock('@/composables/useEventStream', () => ({
  onSSEEvent: (...args: unknown[]) => mocks.onSSEEvent(...args),
  offSSEEvent: (...args: unknown[]) => mocks.offSSEEvent(...args),
}))

vi.mock('@/api/deepResearch', () => ({
  deepResearchApi: {
    listJobs: vi.fn(),
    getJob: vi.fn(),
    cancelJob: vi.fn(),
  },
}))

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('deepResearchJobs store', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mocks.routerPush.mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockResolvedValue(undefined)
    vi.mocked(deepResearchApi.listJobs).mockResolvedValue({ data: [] } as never)
    vi.mocked(deepResearchApi.getJob).mockResolvedValue({ data: null } as never)
    vi.mocked(deepResearchApi.cancelJob).mockResolvedValue({
      data: { status: 'cancelled' },
    } as never)
  })

  it('registers deep research SSE listeners on first use', () => {
    useDeepResearchJobsStore()

    expect(mocks.onSSEEvent).toHaveBeenCalledWith('deep_research.job_created', expect.any(Function))
    expect(mocks.onSSEEvent).toHaveBeenCalledWith('deep_research.job_updated', expect.any(Function))
    expect(mocks.onSSEEvent).toHaveBeenCalledWith(
      'deep_research.job_completed',
      expect.any(Function)
    )
  })

  it('hydrates active jobs on first use without an explicit refresh', async () => {
    vi.mocked(deepResearchApi.listJobs).mockResolvedValue({
      data: [
        {
          id: 'job-boot',
          job_id: 'job-boot',
          query: 'hydrate existing job',
          status: 'running',
          stage: 'planning',
          progress: 18,
          iteration: 1,
          conversation_id: 'conv-boot',
          updated_at: '2026-03-11T00:00:00.000Z',
        },
      ],
    } as never)

    const store = useDeepResearchJobsStore()
    useDeepResearchJobsStore()

    await vi.waitFor(() => {
      expect(deepResearchApi.listJobs).toHaveBeenCalledTimes(1)
      expect(store.activeJobs[0]).toMatchObject({
        job_id: 'job-boot',
        conversation_id: 'conv-boot',
        progress: 18,
      })
    })

    expect(store.hydrated).toBe(true)
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

  it('applies deep_research.job_updated SSE updates locally for known jobs', () => {
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

    const jobUpdatedHandler = mocks.onSSEEvent.mock.calls.find(
      ([eventType]) => eventType === 'deep_research.job_updated'
    )?.[1] as ((payload: unknown) => void) | undefined

    expect(jobUpdatedHandler).toBeTypeOf('function')

    jobUpdatedHandler?.({
      job_id: 'job-1',
      stage: 'verify',
      progress: 72,
      latest_action: 'verification',
      latest_gap: 'Need primary source',
      conversation_id: 'conv-1',
      updated_at: '2026-03-11T00:01:00.000Z',
    })

    expect(store.jobMap['job-1']).toMatchObject({
      job_id: 'job-1',
      status: 'running',
      stage: 'verify',
      progress: 72,
      latest_action: 'verification',
      latest_gap: 'Need primary source',
    })
    expect(store.activeJobs).toHaveLength(1)
    expect(deepResearchApi.getJob).not.toHaveBeenCalled()
  })

  it('preserves newer SSE-known active jobs while initial hydration is in flight', async () => {
    const deferred = createDeferred<{ data: Array<Record<string, unknown>> }>()
    vi.mocked(deepResearchApi.listJobs).mockReturnValue(deferred.promise as never)

    const store = useDeepResearchJobsStore()
    const jobCreatedHandler = mocks.onSSEEvent.mock.calls.find(
      ([eventType]) => eventType === 'deep_research.job_created'
    )?.[1] as ((payload: unknown) => void) | undefined

    expect(jobCreatedHandler).toBeTypeOf('function')

    jobCreatedHandler?.({
      job_id: 'job-sse',
      query: 'arrived during hydration',
      status: 'running',
      stage: 'retrieve',
      progress: 24,
      conversation_id: 'conv-sse',
      updated_at: '2026-03-11T00:00:30.000Z',
    })

    deferred.resolve({ data: [] })
    await Promise.resolve()
    await Promise.resolve()

    expect(store.activeJobs).toHaveLength(1)
    expect(store.activeJobs[0]).toMatchObject({
      job_id: 'job-sse',
      conversation_id: 'conv-sse',
      progress: 24,
    })
  })

  it('hydrates unknown deep_research.job_created updates with a targeted job fetch', async () => {
    vi.mocked(deepResearchApi.getJob).mockResolvedValue({
      data: {
        id: 'job-new',
        job_id: 'job-new',
        query: 'new research topic',
        status: 'running',
        stage: 'retrieve',
        progress: 12,
        iteration: 1,
        latest_action: 'initial_retrieve',
        latest_gap: 'Need primary source',
        conversation_id: 'conv-2',
        updated_at: '2026-03-11T00:02:00.000Z',
      },
    } as never)

    const store = useDeepResearchJobsStore()
    const jobCreatedHandler = mocks.onSSEEvent.mock.calls.find(
      ([eventType]) => eventType === 'deep_research.job_created'
    )?.[1] as ((payload: unknown) => void) | undefined

    expect(jobCreatedHandler).toBeTypeOf('function')

    jobCreatedHandler?.({
      job_id: 'job-new',
      conversation_id: 'conv-2',
      query: 'new research topic',
    })

    expect(store.jobMap['job-new']).toMatchObject({
      job_id: 'job-new',
      status: 'running',
    })

    await vi.advanceTimersByTimeAsync(200)

    expect(deepResearchApi.getJob).toHaveBeenCalledWith('job-new')
    expect(store.jobMap['job-new']).toMatchObject({
      job_id: 'job-new',
      progress: 12,
      latest_action: 'initial_retrieve',
      conversation_id: 'conv-2',
    })
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
