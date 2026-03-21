import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import MemoryManager from '@/components/MemoryManager.vue'
import { i18n } from '@/i18n'
import { memoryApi } from '@/api/memory'
import { selfReflectApi } from '@/api/selfReflect'

vi.mock('@/api/memory', () => ({
  memoryApi: {
    store: vi.fn(),
    search: vi.fn(),
    get: vi.fn(),
    delete: vi.fn(),
    prune: vi.fn(),
    clear: vi.fn(),
    stats: vi.fn(),
    exportMarkdown: vi.fn(),
    importMarkdown: vi.fn(),
  },
}))

vi.mock('@/api/selfReflect', () => ({
  selfReflectApi: {
    listProposals: vi.fn(),
    getProposal: vi.fn(),
    getPatchPreview: vi.fn(),
    approveProposal: vi.fn(),
    rejectProposal: vi.fn(),
  },
}))

function statsPayload(overrides: Record<string, unknown> = {}) {
  return {
    data: {
      total_chunks: 0,
      total_size_bytes: 0,
      total_display_count: 0,
      total_display_size_bytes: 0,
      daily_logs_count: 0,
      daily_entries_count: 0,
      daily_total_size_bytes: 0,
      backend: 'markdown',
      ...overrides,
    },
  } as never
}

function mountManager() {
  return mount(MemoryManager, {
    global: {
      plugins: [i18n],
      stubs: {
        teleport: true,
      },
    },
  })
}

function proposalPayload(overrides: Record<string, unknown> = {}) {
  return {
    id: 'proposal-1',
    source_kind: 'harness_group',
    source_id: 'group-1',
    proposal_mode: 'review_only',
    target_file: 'AGENTS.md',
    status: 'pending',
    lesson: 'Keep low-coverage findings explicitly cautious.',
    when_to_apply: 'When citation coverage stays below the publish threshold.',
    evidence: 'Coverage was 72% and one claim remained unresolved.',
    evidence_ids: ['ev-1', 'ev-2'],
    evaluation_summary: {
      verdict: 'partial',
      score: 0.78,
      judge_backend: 'llm_evaluator',
      judge_model: 'gpt-5.4-mini',
      calibration_ref: 'deep_research:job-1:calibration',
      takeaway_candidate_count: 1,
    },
    calibration_summary: {
      coverage: 0.72,
      groundedness: 0.84,
      freshness: 0.66,
      conflict_risk: 'medium',
      confidence: 0.79,
      recommended_action: 'caution',
    },
    patch_preview: '@@ section: Research Takeaways @@',
    created_at: '2026-03-01T00:00:00Z',
    updated_at: '2026-03-01T00:00:00Z',
    ...overrides,
  }
}

function byId(wrapper: VueWrapper, id: string) {
  return wrapper.get(`[data-testid="${id}"]`)
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

function stubFileReaderWith(content: string) {
  class MockFileReader {
    result: string | ArrayBuffer | null = null
    onload: ((this: FileReader, ev: ProgressEvent<FileReader>) => unknown) | null = null
    onerror: ((this: FileReader, ev: ProgressEvent<FileReader>) => unknown) | null = null

    readAsText() {
      this.result = content
      this.onload?.call(
        this as unknown as FileReader,
        new Event('load') as ProgressEvent<FileReader>
      )
    }
  }
  vi.stubGlobal('FileReader', MockFileReader as unknown as typeof FileReader)
}

describe('MemoryManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
    vi.stubGlobal(
      'confirm',
      vi.fn(() => true)
    )

    if (!(URL as unknown as { createObjectURL?: unknown }).createObjectURL) {
      ;(URL as unknown as { createObjectURL: () => string }).createObjectURL = () => 'blob:memory'
    }
    if (!(URL as unknown as { revokeObjectURL?: unknown }).revokeObjectURL) {
      ;(URL as unknown as { revokeObjectURL: () => void }).revokeObjectURL = () => {}
    }

    vi.mocked(memoryApi.stats).mockResolvedValue(statsPayload())
    vi.mocked(memoryApi.store).mockResolvedValue({
      data: {
        id: 'mem-1',
        content: 'saved',
        created_at: '2026-03-01T00:00:00Z',
      },
    } as never)
    vi.mocked(memoryApi.search).mockResolvedValue({
      data: { results: [], total: 0 },
    } as never)
    vi.mocked(memoryApi.delete).mockResolvedValue({} as never)
    vi.mocked(memoryApi.prune).mockResolvedValue({ data: { deleted: 0 } } as never)
    vi.mocked(memoryApi.clear).mockResolvedValue({} as never)
    vi.mocked(memoryApi.exportMarkdown).mockResolvedValue({ data: '# export' } as never)
    vi.mocked(memoryApi.importMarkdown).mockResolvedValue({
      data: { imported: 0, skipped: 0, errors: [] },
    } as never)
    vi.mocked(selfReflectApi.listProposals).mockResolvedValue({ data: [] } as never)
    vi.mocked(selfReflectApi.getProposal).mockRejectedValue(new Error('proposal not found'))
    vi.mocked(selfReflectApi.getPatchPreview).mockRejectedValue(new Error('patch not found'))
    vi.mocked(selfReflectApi.approveProposal).mockResolvedValue({
      data: proposalPayload({ status: 'approved', review_note: 'Looks good', reviewed_at: '2026-03-02T00:00:00Z' }),
    } as never)
    vi.mocked(selfReflectApi.rejectProposal).mockResolvedValue({
      data: proposalPayload({ status: 'rejected', review_note: 'Needs stronger grounding', reviewed_at: '2026-03-02T00:00:00Z' }),
    } as never)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('emits memory recall mode changes from the embedded controls', async () => {
    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-open-recall-settings').trigger('click')
    await byId(wrapper, 'memory-recall-mode-aggressive').trigger('click')

    expect(wrapper.emitted('memory-recall-mode-change')).toEqual([['aggressive']])
  })

  it('adds memory with normalized tags and emits a status update', async () => {
    vi.mocked(memoryApi.stats)
      .mockResolvedValueOnce(statsPayload())
      .mockResolvedValueOnce(statsPayload({ total_chunks: 1, total_size_bytes: 256 }))

    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-toggle-composer').trigger('click')
    await byId(wrapper, 'memory-add-content').setValue('Remember: user prefers keyboard shortcuts')
    await byId(wrapper, 'memory-add-tags').setValue('ux, preferences,   ')
    await byId(wrapper, 'memory-add-submit').trigger('click')
    await flushPromises()

    expect(memoryApi.store).toHaveBeenCalledWith({
      content: 'Remember: user prefers keyboard shortcuts',
      tags: ['ux', 'preferences'],
    })
    expect(memoryApi.stats).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="memory-add-content"]').exists()).toBe(false)
    expect(wrapper.emitted('status-change')).toBeTruthy()

    wrapper.unmount()
  })

  it('keeps only the latest search result when older requests resolve later', async () => {
    vi.useFakeTimers()
    const older = createDeferred<any>()
    vi.mocked(memoryApi.search)
      .mockImplementationOnce(() => older.promise)
      .mockResolvedValueOnce({
        data: {
          results: [
            {
              id: 'new',
              content: 'NEWEST_MATCH',
              score: 0.93,
              match_types: ['exact'],
              created_at: '2026-03-01T00:00:00Z',
            },
          ],
          total: 1,
        },
      } as never)

    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-search-input').setValue('old-query')
    vi.advanceTimersByTime(300)
    await flushPromises()

    await byId(wrapper, 'memory-search-input').setValue('new-query')
    vi.advanceTimersByTime(300)
    await flushPromises()

    expect(memoryApi.search).toHaveBeenNthCalledWith(1, { query: 'old-query', limit: 50 })
    expect(memoryApi.search).toHaveBeenNthCalledWith(2, { query: 'new-query', limit: 50 })
    expect(wrapper.text()).toContain('NEWEST_MATCH')

    older.resolve({
      data: {
        results: [
          {
            id: 'old',
            content: 'STALE_MATCH',
            score: 0.97,
            match_types: ['exact'],
            created_at: '2026-03-01T00:00:00Z',
          },
        ],
        total: 1,
      },
    } as never)
    await flushPromises()

    expect(wrapper.text()).toContain('NEWEST_MATCH')
    expect(wrapper.text()).not.toContain('STALE_MATCH')

    wrapper.unmount()
  })

  it('hides the search result area until a search query is entered', async () => {
    const wrapper = mountManager()
    await flushPromises()

    expect(wrapper.find('[data-testid="memory-search-results-panel"]').exists()).toBe(false)

    await byId(wrapper, 'memory-search-input').setValue('query')
    await flushPromises()

    expect(wrapper.find('[data-testid="memory-search-results-panel"]').exists()).toBe(true)

    wrapper.unmount()
  })

  it('keeps search fully inline without extra action buttons', async () => {
    const wrapper = mountManager()
    await flushPromises()

    expect(wrapper.find('[data-testid="memory-search-submit"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="memory-search-clear"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('localizes match type labels in search results', async () => {
    vi.useFakeTimers()
    vi.mocked(memoryApi.search).mockResolvedValue({
      data: {
        results: [
          {
            id: 'memory-1',
            content: 'keyword hit',
            score: 0.89,
            match_types: ['keyword', 'vector'],
            created_at: '2026-03-01T00:00:00Z',
          },
        ],
        total: 1,
      },
    } as never)

    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-search-input').setValue('hit')
    vi.advanceTimersByTime(300)
    await flushPromises()

    expect(wrapper.text()).toContain('memory.matchTypes.keyword')
    expect(wrapper.text()).toContain('memory.matchTypes.vector')

    wrapper.unmount()
  })

  it('requires user confirmation before deleting a memory', async () => {
    vi.useFakeTimers()
    const confirmMock = vi.fn(() => false)
    vi.stubGlobal('confirm', confirmMock)

    vi.mocked(memoryApi.search).mockResolvedValue({
      data: {
        results: [
          {
            id: 'memory-1',
            content: 'delete target',
            score: 0.81,
            match_types: ['exact'],
            created_at: '2026-03-01T00:00:00Z',
          },
        ],
        total: 1,
      },
    } as never)

    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-search-input').setValue('delete target')
    vi.advanceTimersByTime(300)
    await flushPromises()

    const deleteButton = byId(wrapper, 'memory-delete-memory-1')
    await deleteButton.trigger('click')
    expect(confirmMock).toHaveBeenCalledTimes(1)
    expect(memoryApi.delete).not.toHaveBeenCalled()

    confirmMock.mockReturnValue(true)
    await deleteButton.trigger('click')
    await flushPromises()

    expect(memoryApi.delete).toHaveBeenCalledWith('memory-1')

    wrapper.unmount()
  })

  it('ignores in-flight search results after the query is cleared', async () => {
    vi.useFakeTimers()
    const pending = createDeferred<any>()
    vi.mocked(memoryApi.search).mockImplementationOnce(() => pending.promise)

    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-search-input').setValue('to-clear')
    vi.advanceTimersByTime(300)
    await flushPromises()

    await byId(wrapper, 'memory-search-input').setValue('')
    await flushPromises()

    pending.resolve({
      data: {
        results: [
          {
            id: 'late-result',
            content: 'SHOULD_NOT_RETURN',
            score: 0.88,
            match_types: ['exact'],
            created_at: '2026-03-01T00:00:00Z',
          },
        ],
        total: 1,
      },
    } as never)
    await flushPromises()

    expect(wrapper.find('[data-testid="memory-search-results-panel"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('SHOULD_NOT_RETURN')

    wrapper.unmount()
  })

  it('opens export modal and exports markdown with user feedback', async () => {
    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-export-button-inline').trigger('click')
    await flushPromises()

    expect(memoryApi.exportMarkdown).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('status-change')).toBeTruthy()

    wrapper.unmount()
  })

  it('imports markdown in replace mode and refreshes stats', async () => {
    stubFileReaderWith('# imported memory')

    vi.mocked(memoryApi.importMarkdown).mockResolvedValue({
      data: { imported: 2, skipped: 0, errors: [] },
    } as never)
    vi.mocked(memoryApi.stats)
      .mockResolvedValueOnce(statsPayload())
      .mockResolvedValueOnce(statsPayload({ total_chunks: 2, total_size_bytes: 512 }))

    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-open-import').trigger('click')
    const fileInput = byId(wrapper, 'memory-import-file-input')
    const file = new File(['dummy'], 'memory.md', { type: 'text/markdown' })
    Object.defineProperty(fileInput.element, 'files', {
      value: [file],
      configurable: true,
    })
    await fileInput.trigger('change')
    await byId(wrapper, 'memory-import-mode-replace').trigger('click')
    await byId(wrapper, 'memory-import-button').trigger('click')
    await flushPromises()

    expect(memoryApi.importMarkdown).toHaveBeenCalledWith('# imported memory', 'replace')
    expect(memoryApi.stats).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('status-change')).toBeTruthy()

    wrapper.unmount()
  })

  it('shows import parser errors returned by backend', async () => {
    stubFileReaderWith('# imported memory with warnings')

    vi.mocked(memoryApi.importMarkdown).mockResolvedValue({
      data: { imported: 1, skipped: 0, errors: ['line 3: malformed block'] },
    } as never)

    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-open-import').trigger('click')
    const fileInput = byId(wrapper, 'memory-import-file-input')
    const file = new File(['dummy'], 'memory.md', { type: 'text/markdown' })
    Object.defineProperty(fileInput.element, 'files', {
      value: [file],
      configurable: true,
    })
    await fileInput.trigger('change')
    await byId(wrapper, 'memory-import-button').trigger('click')
    await flushPromises()

    expect(memoryApi.importMarkdown).toHaveBeenCalled()
    expect(wrapper.text()).toContain('line 3: malformed block')

    wrapper.unmount()
  })

  it('shows clear-all confirmation and clears memories when confirmed', async () => {
    vi.mocked(memoryApi.stats)
      .mockResolvedValueOnce(statsPayload({ total_chunks: 3, total_size_bytes: 1024 }))
      .mockResolvedValueOnce(statsPayload({ total_chunks: 0, total_size_bytes: 0 }))

    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-open-cleanup').trigger('click')
    await byId(wrapper, 'memory-open-clear').trigger('click')
    expect(wrapper.find('[data-testid="memory-clear-confirm"]').exists()).toBe(true)
    await byId(wrapper, 'memory-clear-confirm').trigger('click')
    await flushPromises()

    expect(memoryApi.clear).toHaveBeenCalledTimes(1)
    expect(memoryApi.stats).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="memory-clear-confirm"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('disables destructive cleanup actions when there are no memories', async () => {
    vi.mocked(memoryApi.stats).mockResolvedValueOnce(
      statsPayload({ total_chunks: 0, total_size_bytes: 0 })
    )
    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-open-cleanup').trigger('click')
    expect(byId(wrapper, 'memory-prune').attributes('disabled')).toBeDefined()
    expect(byId(wrapper, 'memory-open-clear').attributes('disabled')).toBeDefined()

    wrapper.unmount()
  })

  it('shows export error feedback when export fails', async () => {
    vi.mocked(memoryApi.exportMarkdown).mockRejectedValue(new Error('export failed: network down'))
    const wrapper = mountManager()
    await flushPromises()

    await byId(wrapper, 'memory-export-button-inline').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('export failed: network down')
    expect(wrapper.emitted('status-change')).toBeFalsy()

    wrapper.unmount()
  })

  it('renders and reviews self-reflect proposals with patch previews', async () => {
    const currentProposal = proposalPayload()
    vi.mocked(selfReflectApi.listProposals).mockImplementation(async () => ({
      data: [currentProposal],
    }) as never)
    vi.mocked(selfReflectApi.getProposal).mockImplementation(async () => ({
      data: currentProposal,
    }) as never)
    vi.mocked(selfReflectApi.getPatchPreview).mockImplementation(async () => ({
      data: {
        id: currentProposal.id,
        target_file: currentProposal.target_file,
        patch_preview: '@@ section: Research Takeaways @@\n+- Keep low-coverage findings explicitly cautious.',
      },
    }) as never)
    vi.mocked(selfReflectApi.approveProposal).mockImplementation(async (_id: string, note: string) => {
      currentProposal.status = 'approved'
      currentProposal.review_note = note
      currentProposal.reviewed_at = '2026-03-02T00:00:00Z'
      return { data: currentProposal } as never
    })

    const wrapper = mountManager()
    await flushPromises()

    expect(selfReflectApi.listProposals).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Self-evolution review queue')
    expect(wrapper.text()).toContain('Keep low-coverage findings explicitly cautious.')
    expect(wrapper.text()).toContain('@@ section: Research Takeaways @@')

    await byId(wrapper, 'memory-proposal-note').setValue('Ship this as a guarded takeaway')
    await byId(wrapper, 'memory-proposal-approve').trigger('click')
    await flushPromises()

    expect(selfReflectApi.approveProposal).toHaveBeenCalledWith(
      'proposal-1',
      'Ship this as a guarded takeaway'
    )
    expect(wrapper.text()).toContain('Approved')

    wrapper.unmount()
  })

  it('shows the proposal-generation banner when agent auto-reflect is disabled', async () => {
    const wrapper = mount(MemoryManager, {
      props: {
        agentAutoReflect: false,
      },
      global: {
        plugins: [i18n],
        stubs: {
          teleport: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Agent auto-reflect is disabled')

    wrapper.unmount()
  })
})
