import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useProviderPoolStore } from '@/stores/providerPool'
import { providerPoolApi } from '@/api/providerPool'

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    addAPIKey: vi.fn(),
    removeAPIKey: vi.fn(),
  },
}))

vi.mock('@/api/mediaProviders', () => ({
  mediaProviderApi: {},
}))

describe('providerPool store credential-driven enablement', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('auto-enables a non-media provider when adding an API key', async () => {
    const store = useProviderPoolStore()
    store.providers.push({
      id: 'custom-provider',
      name: 'Custom',
      type: 'custom',
      location: 'cloud',
      enabled: false,
      status: 'inactive',
      base_url: 'https://example.com/v1',
      priority: 10,
      api_keys: [],
    })

    vi.mocked(providerPoolApi.addAPIKey).mockResolvedValue({
      data: {
        id: '',
        key_hash: 'sk-****',
        usage_count: 0,
        created_at: '',
        enabled: true,
      },
    } as never)

    await store.addAPIKey('custom-provider', 'sk-test')

    expect(store.providers[0]?.enabled).toBe(true)
    expect(store.providers[0]?.api_keys).toHaveLength(1)
  })

  it('auto-disables a non-media provider when removing its last API key', async () => {
    const store = useProviderPoolStore()
    store.providers.push({
      id: 'custom-provider',
      name: 'Custom',
      type: 'custom',
      location: 'cloud',
      enabled: true,
      status: 'inactive',
      base_url: 'https://example.com/v1',
      priority: 10,
      api_keys: [
        {
          id: 'key-1',
          key_hash: 'sk-****',
          usage_count: 0,
          created_at: '',
          enabled: true,
        },
      ],
    })

    vi.mocked(providerPoolApi.removeAPIKey).mockResolvedValue({} as never)

    await store.removeAPIKey('custom-provider', 'key-1')

    expect(store.providers[0]?.api_keys).toHaveLength(0)
    expect(store.providers[0]?.enabled).toBe(false)
    expect(store.providers[0]?.status).toBe('inactive')
  })
})
