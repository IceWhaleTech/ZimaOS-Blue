import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSettingsStore } from '@/stores/settings'
import { providerApi, toolApi } from '@/api/chat'

vi.mock('@/api/chat', () => ({
  providerApi: {
    list: vi.fn(),
  },
  toolApi: {
    list: vi.fn(),
  },
}))

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}
Object.defineProperty(window, 'localStorage', { value: localStorageMock })

describe('Settings Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    localStorageMock.getItem.mockReturnValue(null)
  })

  describe('fetchProviders', () => {
    it('should fetch and store providers', async () => {
      const mockProviders = [
        { name: 'openai', models: ['gpt-4o', 'gpt-4o-mini'] },
        { name: 'claude', models: ['claude-3-opus', 'claude-3-sonnet'] },
      ]
      vi.mocked(providerApi.list).mockResolvedValue({ data: mockProviders } as never)

      const store = useSettingsStore()
      await store.fetchProviders()

      expect(providerApi.list).toHaveBeenCalled()
      expect(store.providers).toEqual(mockProviders)
      expect(store.loading).toBe(false)
    })

    it('should set default provider if current is not available', async () => {
      const mockProviders = [{ name: 'ollama', models: ['llama2'] }]
      vi.mocked(providerApi.list).mockResolvedValue({ data: mockProviders } as never)

      const store = useSettingsStore()
      store.selectedProvider = 'nonexistent'

      await store.fetchProviders()

      expect(store.selectedProvider).toBe('ollama')
    })
  })

  describe('fetchTools', () => {
    it('should fetch and store tools', async () => {
      const mockTools = [
        { name: 'calculator', description: 'Perform calculations', parameters: {} },
        { name: 'system_info', description: 'Get system info', parameters: {} },
      ]
      vi.mocked(toolApi.list).mockResolvedValue({ data: mockTools } as never)

      const store = useSettingsStore()
      await store.fetchTools()

      expect(toolApi.list).toHaveBeenCalled()
      expect(store.tools).toEqual(mockTools)
    })
  })

  describe('setProvider', () => {
    it('should set provider and reset model', async () => {
      const store = useSettingsStore()
      store.providers = [
        { name: 'openai', models: ['gpt-4o', 'gpt-4o-mini'] },
        { name: 'claude', models: ['claude-3-opus'] },
      ]

      store.setProvider('claude')

      expect(store.selectedProvider).toBe('claude')
      expect(store.selectedModel).toBe('claude-3-opus')
    })
  })

  describe('setTemperature', () => {
    it('should clamp temperature between 0 and 2', () => {
      const store = useSettingsStore()

      store.setTemperature(-1)
      expect(store.temperature).toBe(0)

      store.setTemperature(3)
      expect(store.temperature).toBe(2)

      store.setTemperature(1.5)
      expect(store.temperature).toBe(1.5)
    })
  })

  describe('setMaxTokens', () => {
    it('should clamp max tokens between 1 and 128000', () => {
      const store = useSettingsStore()

      store.setMaxTokens(0)
      expect(store.maxTokens).toBe(1)

      store.setMaxTokens(200000)
      expect(store.maxTokens).toBe(128000)

      store.setMaxTokens(4096)
      expect(store.maxTokens).toBe(4096)
    })
  })

  describe('API key management', () => {
    it('should set and clear API keys', () => {
      const store = useSettingsStore()

      store.setApiKey('openai', 'sk-test-key')
      expect(store.apiKeys['openai']).toBe('sk-test-key')

      store.clearApiKey('openai')
      expect(store.apiKeys['openai']).toBeUndefined()
    })
  })

  describe('hasApiKey', () => {
    it('should return true for ollama without key', () => {
      const store = useSettingsStore()
      store.selectedProvider = 'ollama'

      expect(store.hasApiKey).toBe(true)
    })

    it('should return false for other providers without key', () => {
      const store = useSettingsStore()
      store.selectedProvider = 'openai'
      store.apiKeys = {}

      expect(store.hasApiKey).toBe(false)
    })

    it('should return true when API key is set', () => {
      const store = useSettingsStore()
      store.selectedProvider = 'openai'
      store.apiKeys = { openai: 'sk-test' }

      expect(store.hasApiKey).toBe(true)
    })
  })

  describe('availableModels', () => {
    it('should return models for current provider', () => {
      const store = useSettingsStore()
      store.providers = [
        { name: 'openai', models: ['gpt-4o', 'gpt-4o-mini'] },
        { name: 'claude', models: ['claude-3-opus'] },
      ]
      store.selectedProvider = 'openai'

      expect(store.availableModels).toEqual(['gpt-4o', 'gpt-4o-mini'])
    })

    it('should return empty array if provider not found', () => {
      const store = useSettingsStore()
      store.providers = []
      store.selectedProvider = 'nonexistent'

      expect(store.availableModels).toEqual([])
    })
  })
})
