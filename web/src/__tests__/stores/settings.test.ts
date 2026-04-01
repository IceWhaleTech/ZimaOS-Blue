import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSettingsStore } from '@/stores/settings'
import { providerPoolApi } from '@/api/providerPool'
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

vi.mock('@/api/settings', () => ({
  settingsApi: {
    get: vi.fn(),
    update: vi.fn(),
    patch: vi.fn(),
    getSkillRerankerModelStatus: vi.fn(),
    downloadSkillRerankerModel: vi.fn(),
    cancelSkillRerankerModelDownload: vi.fn(),
    getSmallModelStatus: vi.fn(),
    downloadSmallModel: vi.fn(),
    cancelSmallModelDownload: vi.fn(),
    getSmallModelStats: vi.fn(),
    resetSmallModelStats: vi.fn(),
  },
}))

const SETTINGS_KEY = 'zimaos-blue-settings'
const MAX_TOKENS_MIGRATION_KEY_V1 = 'zimaos-blue-max-tokens-migrated-v1'
const MAX_TOKENS_MIGRATION_KEY_V2 = 'zimaos-blue-max-tokens-migrated-v2'

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
    expect(store.smallModelContextCompressEnabled).toBe(false)
    expect(store.smallModelDocExtractEnabled).toBe(false)
    expect(store.smallModelRerankEnabled).toBe(false)
    expect(store.contextCompressionMode).toBe('auto')
    expect(store.smallModelRouteImageQAEnabled).toBe(false)
    expect(store.smallModelRouteShortQAEnabled).toBe(false)
    expect(store.noLLMDegradeMode).toBe('deepresearch')
    expect(store.smallModelUnavailablePolicy).toBe('ir_first')
    expect(store.offlineIRFallbackEnabled).toBe(false)
    expect(store.featureIntentIREnabled).toBe(false)
    expect(store.smartSkillSelection).toBe(false)
  })

  it('migrates legacy maxTokens 2048 to 16384 once', () => {
    localStorageMock.setItem(SETTINGS_KEY, JSON.stringify({ maxTokens: 2048, temperature: 0.7 }))

    const store = useSettingsStore()

    expect(store.maxTokens).toBe(16384)
    expect(localStorageMock.getItem(MAX_TOKENS_MIGRATION_KEY_V1)).toBe('1')
    expect(localStorageMock.getItem(MAX_TOKENS_MIGRATION_KEY_V2)).toBe('1')
    expect(JSON.parse(localStorageMock.getItem(SETTINGS_KEY) || '{}').maxTokens).toBe(16384)
  })

  it('does not re-migrate when migration marker already exists', () => {
    localStorageMock.setItem(SETTINGS_KEY, JSON.stringify({ maxTokens: 2048, temperature: 0.7 }))
    localStorageMock.setItem(MAX_TOKENS_MIGRATION_KEY_V1, '1')

    const store = useSettingsStore()

    expect(store.maxTokens).toBe(2048)
  })

  it('migrates the previous default maxTokens 8192 to 16384 once', () => {
    localStorageMock.setItem(SETTINGS_KEY, JSON.stringify({ maxTokens: 8192, temperature: 0.7 }))
    localStorageMock.setItem(MAX_TOKENS_MIGRATION_KEY_V1, '1')

    const store = useSettingsStore()

    expect(store.maxTokens).toBe(16384)
    expect(localStorageMock.getItem(MAX_TOKENS_MIGRATION_KEY_V2)).toBe('1')
    expect(JSON.parse(localStorageMock.getItem(SETTINGS_KEY) || '{}').maxTokens).toBe(16384)
  })

  it('does not re-migrate 8192 when v2 migration marker already exists', () => {
    localStorageMock.setItem(SETTINGS_KEY, JSON.stringify({ maxTokens: 8192, temperature: 0.7 }))
    localStorageMock.setItem(MAX_TOKENS_MIGRATION_KEY_V1, '1')
    localStorageMock.setItem(MAX_TOKENS_MIGRATION_KEY_V2, '1')

    const store = useSettingsStore()

    expect(store.maxTokens).toBe(8192)
  })

  it('marks migration as done for fresh profiles', () => {
    useSettingsStore()

    expect(localStorageMock.getItem(MAX_TOKENS_MIGRATION_KEY_V1)).toBe('1')
    expect(localStorageMock.getItem(MAX_TOKENS_MIGRATION_KEY_V2)).toBe('1')
  })

  it('defaults close behavior to minimize for fresh profiles', () => {
    const store = useSettingsStore()

    expect(store.closeBehavior).toBe('minimize')
  })

  it('sorts provider-model options by descending model name', async () => {
    const store = useSettingsStore()
    const models = [
      { id: 'o3-mini', enabled: true },
      { id: 'gemini-2.5-pro', enabled: true },
      { id: 'gpt-4o-mini', enabled: true },
      { id: 'claude-haiku-4-5', enabled: true },
    ]
    for (let index = 0; index < 100; index += 1) {
      models.push({ id: `misc-${index.toString().padStart(3, '0')}`, enabled: true })
    }
    vi.mocked(providerPoolApi.listProviders).mockResolvedValue({
      data: {
        providers: [
          {
            id: 'relay',
            name: 'Relay',
            enabled: true,
            models,
          },
        ],
      },
    } as never)

    await store.fetchProviders()

    expect(store.providerModelOptions.slice(0, 4).map((option) => option.modelId)).toEqual([
      'o3-mini',
      'misc-099',
      'misc-098',
      'misc-097',
    ])
  })

  it('keeps enabled oauth-backed providers in chat provider options', async () => {
    const store = useSettingsStore()
    vi.mocked(providerPoolApi.listProviders).mockResolvedValue({
      data: {
        providers: [
          {
            id: 'google-antigravity',
            name: 'Google Cloud Code Assist (Antigravity)',
            enabled: true,
            oauth: { connected: false },
            models: [{ id: 'claude-sonnet-4-5', enabled: true }],
          },
          {
            id: 'anthropic',
            name: 'Anthropic',
            enabled: true,
            models: [{ id: 'claude-sonnet-4-5', enabled: true }],
          },
        ],
      },
    } as never)

    await store.fetchProviders()

    expect(store.providers.map((provider) => provider.id)).toEqual([
      'google-antigravity',
      'anthropic',
    ])
    expect(store.providerModelOptions.map((option) => option.providerId)).toEqual([
      'anthropic',
      'google-antigravity',
    ])
  })

  it('hides unsupported official providers from chat provider options', async () => {
    const store = useSettingsStore()
    vi.mocked(providerPoolApi.listProviders).mockResolvedValue({
      data: {
        providers: [
          {
            id: 'bedrock',
            name: 'Amazon Bedrock',
            enabled: true,
            metadata_mode: 'catalog',
            models: [{ id: 'anthropic.claude-3-5-sonnet-20241022-v2:0', enabled: true }],
          },
          {
            id: 'openai',
            name: 'OpenAI',
            enabled: true,
            metadata_mode: 'catalog',
            base_url: 'https://api.openai.com/v1',
            models: [{ id: 'gpt-4o', enabled: true }],
          },
        ],
      },
    } as never)

    await store.fetchProviders()

    expect(store.providers.map((provider) => provider.id)).toEqual(['openai'])
    expect(store.providerModelOptions.map((option) => option.providerId)).toEqual(['openai'])
  })

  it('updates small-model route toggles in backend settings', async () => {
    const store = useSettingsStore()
    vi.mocked(settingsApi.patch).mockResolvedValue({ data: {} } as never)

    await store.setSmallModelContextCompressEnabled(true)
    expect(settingsApi.patch).toHaveBeenNthCalledWith(1, {
      small_model_context_compress_enabled: true,
    })

    await store.setContextCompressionMode('offline')
    expect(settingsApi.patch).toHaveBeenNthCalledWith(2, {
      context_compression_mode: 'offline',
    })

    await store.setSmallModelRouteImageQAEnabled(false)
    expect(settingsApi.patch).toHaveBeenNthCalledWith(3, {
      small_model_route_image_qa_enabled: false,
    })

    await store.setSmallModelRouteShortQAEnabled(false)
    expect(settingsApi.patch).toHaveBeenNthCalledWith(4, {
      small_model_route_short_qa_enabled: false,
    })
  })

  it('normalizes legacy off compression mode to auto when loading backend settings', async () => {
    const store = useSettingsStore()
    vi.mocked(settingsApi.get).mockResolvedValue({
      data: {
        context_compression_mode: 'off',
      },
    } as never)

    await store.fetchBackendSettings()

    expect(store.contextCompressionMode).toBe('auto')
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
          summary_attempts: 0,
          summary_success: 0,
          doc_extract_attempts: 0,
          doc_extract_success: 0,
          no_provider_deepresearch_total: 0,
          ir_takeover_total: 0,
          fallback_reasons: {},
        },
      } as never)
    vi.mocked(settingsApi.resetSmallModelStats).mockResolvedValue({
      data: { success: true },
    } as never)

    await store.fetchSmallModelStats()
    expect(store.smallModelStats?.ir_takeover_total).toBe(3)
    expect(store.smallModelStats?.fallback_reasons.timeout).toBe(2)

    await store.resetSmallModelStats()
    expect(settingsApi.resetSmallModelStats).toHaveBeenCalled()
    expect(store.smallModelStats?.short_qa_route_attempts).toBe(0)
  })
})
