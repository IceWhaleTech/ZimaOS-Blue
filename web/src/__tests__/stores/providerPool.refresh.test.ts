import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useProviderPoolStore } from '@/stores/providerPool'
import { providerPoolApi } from '@/api/providerPool'

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    fetchProviderModels: vi.fn(),
  },
}))

vi.mock('@/api/mediaProviders', () => ({
  mediaProviderApi: {},
}))

describe('providerPool store model refresh', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('updates provider status from the backend after a successful refresh', async () => {
    const store = useProviderPoolStore()
    store.providers.push({
      id: 'custom-provider',
      name: 'Custom',
      type: 'custom',
      location: 'local',
      enabled: true,
      status: 'inactive',
      base_url: 'http://127.0.0.1:11434/v1',
      priority: 10,
    })

    vi.mocked(providerPoolApi.fetchProviderModels).mockResolvedValue({
      data: {
        models: [
          {
            id: 'llama3.1',
            provider_id: 'custom-provider',
            name: 'llama3.1',
            display_name: 'Llama 3.1',
            enabled: true,
            capabilities: ['chat'],
          },
        ],
        total: 1,
        provider: {
          id: 'custom-provider',
          name: 'Custom',
          type: 'custom',
          location: 'local',
          enabled: true,
          status: 'active',
          base_url: 'http://127.0.0.1:11434/v1',
          priority: 10,
        },
      },
    } as never)

    const result = await store.refreshModels('custom-provider')

    expect(result.success).toBe(true)
    expect(store.models).toHaveLength(1)
    expect(store.providers[0]?.status).toBe('active')
  })
})
