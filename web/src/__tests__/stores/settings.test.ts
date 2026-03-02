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
    expect(store.smallModelRuntime).toBe('llama_cpp_native')
    expect(store.smallModelID).toBe('lfm2.5-1.2b-instruct-q4km')
    expect(store.smallModelAutoDownload).toBe(true)
    expect(store.smallModelShadowRatio).toBe(0.1)
    expect(store.smallModelShadowGateMinSamples).toBe(40)
    expect(store.smallModelShadowGateThresholdDelta).toBe(0.35)
    expect(store.smallModelShadowGateScene).toBe('')
    expect(store.noLLMDegradeMode).toBe('deepresearch')
    expect(store.smallModelUnavailablePolicy).toBe('ir_first')
  })

  it('normalizes shadow ratio when updating backend settings', async () => {
    const store = useSettingsStore()
    vi.mocked(settingsApi.patch).mockResolvedValue({ data: { small_model_shadow_ratio: 1 } } as never)

    await store.setSmallModelShadowRatio(2)

    expect(settingsApi.patch).toHaveBeenCalledWith({ small_model_shadow_ratio: 1 })

    vi.mocked(settingsApi.patch).mockResolvedValue({ data: { small_model_shadow_ratio: 0.01 } } as never)
    await store.setSmallModelShadowRatio(0)
    expect(settingsApi.patch).toHaveBeenLastCalledWith({ small_model_shadow_ratio: 0.01 })
  })

  it('normalizes shadow gate settings when updating backend settings', async () => {
    const store = useSettingsStore()
    vi.mocked(settingsApi.patch).mockResolvedValue({ data: {} } as never)

    await store.setSmallModelShadowGateMinSamples(0)
    expect(settingsApi.patch).toHaveBeenCalledWith({ small_model_shadow_gate_min_samples: 1 })

    await store.setSmallModelShadowGateMinSamples(20001)
    expect(settingsApi.patch).toHaveBeenLastCalledWith({ small_model_shadow_gate_min_samples: 10000 })

    await store.setSmallModelShadowGateThresholdDelta(0)
    expect(settingsApi.patch).toHaveBeenLastCalledWith({ small_model_shadow_gate_threshold_delta: 0.01 })

    await store.setSmallModelShadowGateThresholdDelta(2)
    expect(settingsApi.patch).toHaveBeenLastCalledWith({ small_model_shadow_gate_threshold_delta: 1 })

    await store.setSmallModelShadowGateScene(' tool_dispatch_shadow ')
    expect(settingsApi.patch).toHaveBeenLastCalledWith({ small_model_shadow_gate_scene: 'tool_dispatch_shadow' })
  })

  it('fetches and stores small-model status', async () => {
    const store = useSettingsStore()
    vi.mocked(settingsApi.getSmallModelStatus).mockResolvedValue({
      data: {
        ready: true,
        downloading: false,
        model_id: 'lfm2.5-1.2b-instruct-q4km',
        runtime: 'llama_cpp_native',
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
          short_qa_shadow_total: 9,
          tool_dispatch_shadow_total: 4,
          shadow_failures: 1,
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
          short_qa_shadow_total: 0,
          tool_dispatch_shadow_total: 0,
          shadow_failures: 0,
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
