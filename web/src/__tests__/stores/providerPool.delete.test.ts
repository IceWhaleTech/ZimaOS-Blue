import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useProviderPoolStore } from '@/stores/providerPool'
import { providerPoolApi } from '@/api/providerPool'

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    deleteProvider: vi.fn(),
  },
}))

vi.mock('@/api/mediaProviders', () => ({
  mediaProviderApi: {},
}))

describe('providerPool store delete provider', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('deletes custom provider through providerPool API', async () => {
    const store = useProviderPoolStore()
    store.providers.push({
      id: 'custom-provider',
      name: 'Custom',
      type: 'custom',
      location: 'cloud',
      enabled: true,
      status: 'active',
      priority: 10,
    })
    vi.mocked(providerPoolApi.deleteProvider).mockResolvedValue({} as never)

    await store.deleteProvider('custom-provider')

    expect(providerPoolApi.deleteProvider).toHaveBeenCalledWith('custom-provider')
    expect(store.providers).toHaveLength(0)
  })

  it('rejects deleting media provider and does not call providerPool API', async () => {
    const store = useProviderPoolStore()
    store.providers.push({
      id: 'media-kling',
      name: 'Kling',
      type: 'media',
      location: 'cloud',
      enabled: true,
      status: 'active',
      priority: 10,
    })

    await expect(store.deleteProvider('media-kling')).rejects.toThrow('Media providers are not deletable')
    expect(providerPoolApi.deleteProvider).not.toHaveBeenCalled()
  })
})
