import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useProviderPoolStore } from '@/stores/providerPool'
import { providerPoolApi } from '@/api/providerPool'
import { mediaProviderApi } from '@/api/mediaProviders'

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    updateProvider: vi.fn(),
  },
}))

vi.mock('@/api/mediaProviders', () => ({
  mediaProviderApi: {
    update: vi.fn(),
  },
}))

describe('providerPool store reorder persistence', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('routes media priority updates through the media provider API', async () => {
    const store = useProviderPoolStore()
    store.providers.push(
      {
        id: 'openai',
        name: 'OpenAI',
        type: 'builtin',
        location: 'cloud',
        enabled: true,
        status: 'active',
        priority: 100,
      },
      {
        id: 'gemini-image',
        name: 'Gemini Image',
        type: 'media',
        location: 'cloud',
        enabled: true,
        status: 'active',
        priority: 20,
      }
    )

    vi.mocked(providerPoolApi.updateProvider).mockResolvedValue({} as never)
    vi.mocked(mediaProviderApi.update).mockResolvedValue({} as never)

    await store.syncPrioritiesToBackend([
      { id: 'openai', priority: 90 },
      { id: 'gemini-image', priority: 10 },
    ])

    expect(providerPoolApi.updateProvider).toHaveBeenCalledWith('openai', { priority: 90 })
    expect(mediaProviderApi.update).toHaveBeenCalledWith('gemini-image', { priority: 10 })
    expect(providerPoolApi.updateProvider).not.toHaveBeenCalledWith('gemini-image', { priority: 10 })
  })
})
