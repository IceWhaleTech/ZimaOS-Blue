import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useProviderPoolStore } from '@/stores/providerPool'

describe('providerPool store visibility gating', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('excludes unsupported official providers from enabled and location-based provider lists', () => {
    const store = useProviderPoolStore()

    store.providers = [
      {
        id: 'bedrock',
        name: 'Amazon Bedrock',
        type: 'platform',
        location: 'cloud',
        enabled: true,
        status: 'active',
        metadata_mode: 'catalog',
        priority: 10,
      },
      {
        id: 'ollama',
        name: 'Ollama',
        type: 'builtin',
        location: 'local',
        enabled: true,
        status: 'active',
        metadata_mode: 'catalog',
        base_url: 'http://localhost:11434',
        priority: 20,
      },
      {
        id: 'openai',
        name: 'OpenAI',
        type: 'builtin',
        location: 'cloud',
        enabled: true,
        status: 'active',
        metadata_mode: 'catalog',
        base_url: 'https://api.openai.com/v1',
        priority: 30,
      },
    ]

    expect(store.enabledProviders.map((provider) => provider.id)).toEqual(['ollama', 'openai'])
    expect(store.cloudProviders.map((provider) => provider.id)).toEqual(['openai'])
    expect(store.localProviders.map((provider) => provider.id)).toEqual(['ollama'])
    expect(store.hasCloudProviders).toBe(true)
    expect(store.hasLocalProviders).toBe(true)
    expect(store.hasUserConfiguredProviders).toBe(true)
  })

  it('does not treat hidden unsupported official providers as configured cloud providers', () => {
    const store = useProviderPoolStore()

    store.providers = [
      {
        id: 'bedrock',
        name: 'Amazon Bedrock',
        type: 'platform',
        location: 'cloud',
        enabled: true,
        status: 'active',
        metadata_mode: 'catalog',
        priority: 10,
      },
    ]

    expect(store.enabledProviders).toEqual([])
    expect(store.cloudProviders).toEqual([])
    expect(store.hasCloudProviders).toBe(false)
    expect(store.hasUserConfiguredProviders).toBe(false)
  })
})
