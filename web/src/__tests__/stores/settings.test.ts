import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSettingsStore } from '@/stores/settings'
import { settingsApi } from '@/api/settings'

vi.mock('@/api/chat', () => ({
  toolApi: {
    list: vi.fn(),
  },
}))

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    listProviders: vi.fn(),
    fetchProviderModels: vi.fn(),
  },
}))

vi.mock('@/api/claudecode', () => ({
  claudeCodeApi: {
    getConfig: vi.fn(),
  },
}))

vi.mock('@/api/settings', () => ({
  settingsApi: {
    get: vi.fn(),
    update: vi.fn(),
    patch: vi.fn(),
    getToolStats: vi.fn(),
    getSkillRerankerModelStatus: vi.fn(),
    downloadSkillRerankerModel: vi.fn(),
    cancelSkillRerankerModelDownload: vi.fn(),
    getSmallModelStatus: vi.fn(),
    downloadSmallModel: vi.fn(),
    cancelSmallModelDownload: vi.fn(),
    listSoulProposals: vi.fn(),
    approveSoulProposal: vi.fn(),
    rejectSoulProposal: vi.fn(),
    getSmallModelStats: vi.fn(),
    resetSmallModelStats: vi.fn(),
  },
}))

const SETTINGS_KEY = 'zimaos-blue-settings'
const MAX_TOKENS_MIGRATION_KEY = 'zimaos-blue-max-tokens-migrated-v1'

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

describe('settings store - small model integration', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorageMock.clear()
    vi.clearAllMocks()
  })

  it('uses fixed defaults for small-model strategy fields', () => {
    const store = useSettingsStore()

    expect(store.smallModelEnabled).toBe(false)
    expect(store.smallModelRuntime).toBe('llama.cpp')
    expect(store.smallModelID).toBe('qwen3.5-0.8b-gguf-q4km')
    expect(store.smallModelAutoDownload).toBe(true)
    expect(store.smallModelSummaryEnabled).toBe(false)
    expect(store.smallModelDocExtractEnabled).toBe(false)
    expect(store.smallModelRerankEnabled).toBe(false)
    expect(store.smallModelRouteShortQAEnabled).toBe(false)
    expect(store.smallModelRouteToolDispatchEnabled).toBe(false)
    expect(store.noLLMDegradeMode).toBe('deepresearch')
    expect(store.smallModelUnavailablePolicy).toBe('ir_first')
    expect(store.offlineIRFallbackEnabled).toBe(false)
    expect(store.featureIntentIREnabled).toBe(false)
    expect(store.smartToolSelection).toBe(false)
    expect(store.smartSkillSelection).toBe(false)
  })

  it('migrates legacy maxTokens 2048 to 8192 once', () => {
    localStorageMock.setItem(SETTINGS_KEY, JSON.stringify({ maxTokens: 2048, temperature: 0.7 }))

    const store = useSettingsStore()

    expect(store.maxTokens).toBe(8192)
    expect(localStorageMock.getItem(MAX_TOKENS_MIGRATION_KEY)).toBe('1')
    expect(JSON.parse(localStorageMock.getItem(SETTINGS_KEY) || '{}').maxTokens).toBe(8192)
  })

  it('does not re-migrate when migration marker already exists', () => {
    localStorageMock.setItem(SETTINGS_KEY, JSON.stringify({ maxTokens: 2048, temperature: 0.7 }))
    localStorageMock.setItem(MAX_TOKENS_MIGRATION_KEY, '1')

    const store = useSettingsStore()

    expect(store.maxTokens).toBe(2048)
  })

  it('marks migration as done for fresh profiles', () => {
    useSettingsStore()

    expect(localStorageMock.getItem(MAX_TOKENS_MIGRATION_KEY)).toBe('1')
  })

  it('updates small-model route toggles in backend settings', async () => {
    const store = useSettingsStore()
    vi.mocked(settingsApi.patch).mockResolvedValue({ data: {} } as never)

    await store.setSmallModelRouteShortQAEnabled(false)
    expect(settingsApi.patch).toHaveBeenCalledWith({ small_model_route_short_qa_enabled: false })

    await store.setSmallModelRouteToolDispatchEnabled(false)
    expect(settingsApi.patch).toHaveBeenLastCalledWith({ small_model_route_tool_dispatch_enabled: false })
  })

  it('fetches and stores small-model status', async () => {
    const store = useSettingsStore()
    vi.mocked(settingsApi.getSmallModelStatus).mockResolvedValue({
      data: {
        ready: true,
        downloading: false,
        model_id: 'qwen3.5-0.8b-gguf-q4km',
        runtime: 'llama.cpp',
        model_path: '/tmp/model.gguf',
      },
    } as never)

    await store.fetchSmallModelStatus()

    expect(settingsApi.getSmallModelStatus).toHaveBeenCalled()
    expect(store.smallModelStatus?.ready).toBe(true)
    expect(store.smallModelStatusError).toBeNull()
  })

  it('fetches and resets small-model stats', async () => {
    const store = useSettingsStore()
    vi.mocked(settingsApi.getSmallModelStats)
      .mockResolvedValueOnce({
        data: {
          short_qa_route_attempts: 10,
          short_qa_route_success: 8,
          tool_dispatch_route_attempts: 6,
          tool_dispatch_route_success: 5,
          summary_attempts: 3,
          summary_success: 2,
          doc_extract_attempts: 4,
          doc_extract_success: 3,
          no_provider_deepresearch_total: 2,
          ir_takeover_total: 3,
          fallback_reasons: { timeout: 2 },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          short_qa_route_attempts: 0,
          short_qa_route_success: 0,
          tool_dispatch_route_attempts: 0,
          tool_dispatch_route_success: 0,
          summary_attempts: 0,
          summary_success: 0,
          doc_extract_attempts: 0,
          doc_extract_success: 0,
          no_provider_deepresearch_total: 0,
          ir_takeover_total: 0,
          fallback_reasons: {},
        },
      } as never)
    vi.mocked(settingsApi.resetSmallModelStats).mockResolvedValue({ data: { success: true } } as never)

    await store.fetchSmallModelStats()
    expect(store.smallModelStats?.ir_takeover_total).toBe(3)
    expect(store.smallModelStats?.fallback_reasons.timeout).toBe(2)

    await store.resetSmallModelStats()
    expect(settingsApi.resetSmallModelStats).toHaveBeenCalled()
    expect(store.smallModelStats?.short_qa_route_attempts).toBe(0)
  })

  it('handles SOUL proposal list and review actions', async () => {
    const store = useSettingsStore()
    const base = {
      id: 'p1',
      title: 'habit',
      content: 'do X first',
      status: 'pending',
      created_at: '2026-03-01T00:00:00Z',
    } as const

    vi.mocked(settingsApi.listSoulProposals).mockResolvedValue({ data: { proposals: [base] } } as never)
    await store.fetchSoulProposals()
    expect(store.soulProposals).toHaveLength(1)
    expect(store.soulProposals[0]?.status).toBe('pending')

    vi.mocked(settingsApi.approveSoulProposal).mockResolvedValue({
      data: {
        ...base,
        status: 'approved',
        reviewed_at: '2026-03-01T00:01:00Z',
      },
    } as never)
    await store.approveSoulProposal('p1')
    expect(store.soulProposals[0]?.status).toBe('approved')

    vi.mocked(settingsApi.rejectSoulProposal).mockResolvedValue({
      data: {
        ...base,
        id: 'p2',
        status: 'rejected',
        reviewed_at: '2026-03-01T00:02:00Z',
      },
    } as never)
    await store.rejectSoulProposal('p2')
    expect(store.soulProposals[0]?.id).toBe('p2')
    expect(store.soulProposals[0]?.status).toBe('rejected')
  })
})
