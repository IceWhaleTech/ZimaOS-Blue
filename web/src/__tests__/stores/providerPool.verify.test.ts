import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useProviderPoolStore } from '@/stores/providerPool'
import { providerPoolApi } from '@/api/providerPool'

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    verifyProviderCandidate: vi.fn(),
    verifyProviderByID: vi.fn(),
  },
}))

vi.mock('@/api/mediaProviders', () => ({
  mediaProviderApi: {},
}))

describe('providerPool store verification actions', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('calls verify provider candidate endpoint and returns verification', async () => {
    const store = useProviderPoolStore()
    vi.mocked(providerPoolApi.verifyProviderCandidate).mockResolvedValue({
      data: {
        base_url: 'https://example.com/v1',
        model: 'gpt-5.3-codex',
        detected_format: 'openai',
        recommended_api_format: 'responses',
        recommended_base_url: 'https://example.com/v1/responses',
        responses_only: true,
        probes: {},
      },
    } as never)

    const result = await store.verifyProviderCandidate({
      base_url: 'https://example.com/v1',
      api_key: 'sk-test',
    })

    expect(providerPoolApi.verifyProviderCandidate).toHaveBeenCalledWith({
      base_url: 'https://example.com/v1',
      api_key: 'sk-test',
    })
    expect(result.responses_only).toBe(true)
    expect(result.recommended_api_format).toBe('responses')
  })

  it('applies verification recommendation and updates provider in local store', async () => {
    const store = useProviderPoolStore()
    store.providers.push({
      id: 'custom-provider',
      name: 'Custom',
      type: 'custom',
      location: 'cloud',
      enabled: true,
      status: 'active',
      base_url: 'https://example.com/v1',
      api_format: 'openai',
      priority: 10,
    })

    vi.mocked(providerPoolApi.verifyProviderByID).mockResolvedValue({
      data: {
        applied: true,
        verification: {
          base_url: 'https://example.com/v1',
          model: 'gpt-5.3-codex',
          detected_format: 'openai',
          recommended_api_format: 'responses',
          recommended_base_url: 'https://example.com/v1/responses',
          responses_only: true,
          probes: {
            models: { url: 'https://example.com/v1/models', status_code: 200, reachable: true },
          },
        },
        provider: {
          id: 'custom-provider',
          name: 'Custom',
          type: 'custom',
          location: 'cloud',
          enabled: true,
          status: 'active',
          base_url: 'https://example.com/v1/responses',
          api_format: 'responses',
          priority: 10,
        },
      },
    } as never)

    const result = await store.verifyProviderRecommendation('custom-provider', true, 'key-main')

    expect(providerPoolApi.verifyProviderByID).toHaveBeenCalledWith('custom-provider', {
      apply: true,
      key_id: 'key-main',
    })
    expect(result.applied).toBe(true)
    expect(store.providers[0]?.api_format).toBe('responses')
    expect(store.providers[0]?.base_url).toBe('https://example.com/v1/responses')
  })
})
