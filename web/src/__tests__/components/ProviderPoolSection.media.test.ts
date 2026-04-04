import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const mocks = vi.hoisted(() => ({
  route: {
    query: {} as Record<string, string>,
  },
  providerPoolStore: {
    providers: [] as Array<Record<string, unknown>>,
    models: [] as Array<Record<string, unknown>>,
    customPricing: [] as Array<Record<string, unknown>>,
    selectedProviderId: null as string | null,
    selectedProvider: null as Record<string, unknown> | null,
    loading: false,
    error: null as string | null,
    oauthQuota: {} as Record<string, unknown>,
    loadingQuota: null as string | null,
    accountStatus: {} as Record<string, unknown>,
    loadingAccountStatus: null as string | null,
    trialProviders: [] as Array<Record<string, unknown>>,
    trialQuota: null as Record<string, unknown> | null,
    fetchProviders: vi.fn(),
    fetchCustomPricing: vi.fn(),
    fetchAccountStatus: vi.fn(),
    clearAccountStatus: vi.fn(),
    refreshModels: vi.fn(),
    probeModels: vi.fn(),
    testProvider: vi.fn(),
    updateProvider: vi.fn(),
    updateProviderPriorityLocal: vi.fn(),
    syncPrioritiesToBackend: vi.fn(),
    addAPIKey: vi.fn(),
    clearProviderError: vi.fn(),
  },
  notificationStore: {
    success: vi.fn(),
    info: vi.fn(),
    error: vi.fn(),
  },
  getProviderUsage: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => mocks.providerPoolStore,
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    getProviderUsage: mocks.getProviderUsage,
  },
}))

import ProviderPoolSection from '@/components/ProviderPoolSection.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'zh-CN',
    fallbackLocale: 'zh-CN',
    missingWarn: false,
    fallbackWarn: false,
    messages: {
      'zh-CN': {
        common: {
          edit: '编辑',
          save: '保存',
          cancel: '取消',
          expand: '展开',
          collapse: '收起',
        },
        providerPool: {
          description: 'LLM 配置',
          advancedOptions: '高级选项',
          location: '位置',
          locationCloud: '云端',
          locationLocal: '本地',
          apiFormatLabel: '格式类型',
          apiFormatHint: '默认使用自动检测，也可以手动固定为某一种 API 格式。修改后会立即生效。',
          apiFormatAutoDetected: '当前自动检测结果：{format}',
          apiFormatOptions: {
            auto: '自动检测',
            openai: 'OpenAI 兼容',
            responses: 'Responses',
            anthropic: 'Anthropic',
            google: 'Google Gemini',
          },
          verifySection: '验证与推荐',
          verifyHint: '探测当前提供商并给出推荐的 API 格式与 Base URL。',
          verifyRun: '验证',
          verifyApply: '应用推荐',
          verifyRunning: '验证中...',
          verifyApplying: '应用中...',
          baseUrl: 'Base URL',
          configure: '配置',
          baseUrlNotConfigured: '未配置 Base URL',
          default: '默认',
          models: '模型',
          noModels: '暂无模型',
          apiKeys: 'API Keys',
          noKeys: '暂无密钥',
          usage: {
            title: '使用情况',
            loading: '加载中',
            requests: '请求数',
            inputTokens: '输入 Tokens',
            outputTokens: '输出 Tokens',
            estimatedCost: '预估费用',
          },
          accountStatus: {
            title: '账户状态',
            loading: '加载账户状态中...',
            unavailable: '账户状态不可用',
            usingKey: '使用密钥 {key}',
            items: {
              remaining: '剩余',
              used: '已用',
              limit: '额度',
              granted: '赠送',
              toppedUp: '充值',
            },
          },
          trial: {
            name: '试用',
          },
        },
      },
    },
  })
}

function createProvider(type: 'media' | 'custom') {
  return {
    id: `${type}-provider`,
    name: type === 'media' ? 'Media Provider' : 'Custom Provider',
    type,
    location: 'cloud',
    enabled: true,
    status: 'active',
    base_url: 'https://example.com/v1',
    api_format: 'openai',
    api_format_mode: 'auto',
    priority: 10,
    api_keys: [],
  }
}

function createRankedProvider(id: string, priority: number) {
  return {
    id,
    name: id.toUpperCase(),
    type: 'custom',
    location: 'cloud',
    enabled: true,
    status: 'active',
    base_url: 'https://example.com/v1',
    api_format: 'openai',
    api_format_mode: 'auto',
    priority,
    api_keys: [],
  }
}

function mountSection() {
  return shallowMount(ProviderPoolSection, {
    global: {
      plugins: [createTestI18n()],
      stubs: {
        teleport: true,
        RouterLink: true,
        'router-link': true,
      },
    },
  })
}

describe('ProviderPoolSection media verification gating', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.route.query = {}
    mocks.getProviderUsage.mockResolvedValue({
      data: {
        summary: null,
      },
    })
    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.models = []
    mocks.providerPoolStore.customPricing = []
    mocks.providerPoolStore.selectedProviderId = null
    mocks.providerPoolStore.selectedProvider = null
    mocks.providerPoolStore.accountStatus = {}
    mocks.providerPoolStore.loadingAccountStatus = null
    mocks.providerPoolStore.fetchProviders.mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchCustomPricing.mockResolvedValue([])
    mocks.providerPoolStore.fetchAccountStatus.mockResolvedValue(undefined)
    mocks.providerPoolStore.clearAccountStatus.mockResolvedValue(undefined)
    mocks.providerPoolStore.refreshModels.mockResolvedValue({ success: true, models: [] })
    mocks.providerPoolStore.probeModels.mockResolvedValue({
      success: true,
      total: 0,
      available: 0,
      unavailable: 0,
      results: [],
    })
    mocks.providerPoolStore.testProvider.mockResolvedValue({ healthy: true })
    mocks.providerPoolStore.updateProvider.mockImplementation(async (_id, updates) => ({
      ...(mocks.providerPoolStore.selectedProvider || {}),
      ...updates,
    }))
    mocks.providerPoolStore.updateProviderPriorityLocal.mockImplementation((id, priority) => {
      const provider = mocks.providerPoolStore.providers.find((item) => item.id === id)
      if (provider) {
        provider.priority = priority
      }
    })
    mocks.providerPoolStore.syncPrioritiesToBackend.mockResolvedValue(undefined)
    mocks.providerPoolStore.addAPIKey.mockResolvedValue({})
    mocks.providerPoolStore.clearProviderError.mockResolvedValue(undefined)
  })

  it('marks the provider settings root with the provider form filler scope', async () => {
    const wrapper = mountSection()

    await flushPromises()

    expect(wrapper.attributes('data-form-filler-scope')).toBe('provider')
  })

  it('hides verify and recommend for media providers', async () => {
    const provider = createProvider('media')
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.text()).not.toContain('验证与推荐')
  })

  it('keeps verify and recommend for non-media providers', async () => {
    const provider = createProvider('custom')
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.text()).toContain('验证与推荐')
  })

  it('hides verify and recommend for catalog providers', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openai',
      type: 'builtin',
      metadata_mode: 'catalog',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.text()).not.toContain('验证与推荐')
  })

  it('hides the base URL section for catalog providers', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openai',
      type: 'builtin',
      metadata_mode: 'catalog',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="provider-base-url-section"]').exists()).toBe(false)
  })

  it('shows custom format selector and saves pinned formats immediately', async () => {
    const provider = createProvider('custom')
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    const select = wrapper.find('[data-testid="custom-provider-format-select"]')
    expect(select.exists()).toBe(true)

    await select.setValue('anthropic')
    await flushPromises()

    expect(mocks.providerPoolStore.updateProvider).toHaveBeenCalledWith(provider.id, {
      api_format: 'anthropic',
      api_format_mode: 'pinned',
    })
  })

  it('switches custom format back to auto-detect immediately', async () => {
    const provider = {
      ...createProvider('custom'),
      api_format: 'anthropic',
      api_format_mode: 'pinned',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    const select = wrapper.find('[data-testid="custom-provider-format-select"]')
    await select.setValue('auto')
    await flushPromises()

    expect(mocks.providerPoolStore.updateProvider).toHaveBeenCalledWith(provider.id, {
      api_format_mode: 'auto',
    })
  })

  it('keeps non-custom providers fixed to cloud location', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openai',
      name: 'OpenAI',
      type: 'builtin',
      location: 'cloud',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="provider-location-fixed-cloud"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="provider-location-local-button"]').exists()).toBe(false)
  })

  it('allows custom providers to switch location', async () => {
    const provider = {
      ...createProvider('custom'),
      location: 'local',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="provider-location-cloud-button"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="provider-location-local-button"]').exists()).toBe(true)
  })

  it('shows catalog local providers as fixed local location', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'ollama',
      name: 'Ollama',
      type: 'builtin',
      location: 'local',
      metadata_mode: 'catalog',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="provider-location-cloud-button"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="provider-location-local-button"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="provider-location-fixed-local"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="provider-location-fixed-cloud"]').exists()).toBe(false)
  })

  it('refreshes catalog providers without probing', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openai',
      name: 'OpenAI',
      type: 'builtin',
      metadata_mode: 'catalog',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    mocks.providerPoolStore.refreshModels.mockClear()
    mocks.providerPoolStore.probeModels.mockClear()

    const setupState = (wrapper.vm.$ as any).setupState
    await setupState.refreshModels(provider.id)
    await flushPromises()

    expect(mocks.providerPoolStore.probeModels).not.toHaveBeenCalled()
    expect(mocks.providerPoolStore.refreshModels).toHaveBeenCalledWith(provider.id)
  })

  it('refreshes dynamic providers by probing first', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openrouter',
      name: 'OpenRouter',
      type: 'platform',
      metadata_mode: 'dynamic',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider
    mocks.providerPoolStore.probeModels.mockResolvedValue({
      success: true,
      total: 2,
      available: 2,
      unavailable: 0,
      results: [],
    })

    const wrapper = mountSection()
    await flushPromises()

    mocks.providerPoolStore.refreshModels.mockClear()
    mocks.providerPoolStore.probeModels.mockClear()

    const setupState = (wrapper.vm.$ as any).setupState
    await setupState.refreshModels(provider.id)
    await flushPromises()

    expect(mocks.providerPoolStore.probeModels).toHaveBeenCalledWith(provider.id)
    expect(mocks.providerPoolStore.refreshModels).not.toHaveBeenCalled()
  })

  it('does not trigger background probing after adding an API key to catalog providers', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openai',
      name: 'OpenAI',
      type: 'builtin',
      metadata_mode: 'catalog',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    mocks.providerPoolStore.probeModels.mockClear()
    mocks.providerPoolStore.addAPIKey.mockClear()
    mocks.providerPoolStore.clearProviderError.mockClear()

    const setupState = (wrapper.vm.$ as any).setupState
    const newKey = setupState.newKey?.value ?? setupState.newKey
    newKey.providerId = provider.id
    newKey.key = 'sk-test'
    newKey.label = 'Primary'

    await setupState.addAPIKey()
    await flushPromises()

    expect(mocks.providerPoolStore.addAPIKey).toHaveBeenCalledWith(provider.id, 'sk-test', 'Primary')
    expect(mocks.providerPoolStore.clearProviderError).toHaveBeenCalledWith(provider.id)
    expect(mocks.providerPoolStore.probeModels).not.toHaveBeenCalled()
  })

  it('does not show catalog provider base URLs in official provider chooser cards', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openai',
      name: 'OpenAI',
      type: 'builtin',
      metadata_mode: 'catalog',
      description: 'Official OpenAI API',
      base_url: 'https://api.openai.com/v1',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    const wrapper = mountSection()
    await flushPromises()

    const setupState = (wrapper.vm.$ as any).setupState
    setupState.openAddProviderModal()
    await flushPromises()

    expect(wrapper.text()).toContain('Official OpenAI API')
    expect(wrapper.text()).not.toContain('https://api.openai.com/v1')
  })

  it('keeps format and location collapsed by default in the add custom provider form', async () => {
    const wrapper = mountSection()
    await flushPromises()

    const setupState = (wrapper.vm.$ as any).setupState
    setupState.openAddProviderModal()
    setupState.openCustomProviderForm()
    await flushPromises()

    const advancedToggle = wrapper.find('[data-testid="new-provider-advanced-toggle"]')
    expect(advancedToggle.exists()).toBe(true)
    expect(advancedToggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('[data-testid="new-provider-advanced-content"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="new-provider-format-select"]').exists()).toBe(false)
  })

  it('reveals format and location when the add custom provider advanced section is expanded', async () => {
    const wrapper = mountSection()
    await flushPromises()

    const setupState = (wrapper.vm.$ as any).setupState
    setupState.openAddProviderModal()
    setupState.openCustomProviderForm()
    await flushPromises()

    await wrapper.find('[data-testid="new-provider-advanced-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="new-provider-advanced-toggle"]').attributes('aria-expanded')).toBe(
      'true'
    )
    expect(wrapper.find('[data-testid="new-provider-advanced-content"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="new-provider-format-select"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="new-provider-location-options"]').exists()).toBe(true)
  })

  it('fetches provider account status for supported API-key providers', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openrouter',
      name: 'OpenRouter',
      type: 'platform',
      api_keys: [
        {
          id: 'key-1',
          key_hash: 'sk-or-1234',
          usage_count: 0,
          created_at: '',
          enabled: true,
        },
      ],
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider

    mountSection()
    await flushPromises()

    expect(mocks.providerPoolStore.fetchAccountStatus).toHaveBeenCalledWith(provider.id, 'key-1')
  })

  it('renders provider account status metrics for supported providers', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'deepseek',
      name: 'DeepSeek',
      type: 'builtin',
      metadata_mode: 'catalog',
      api_keys: [
        {
          id: 'key-1',
          key_hash: 'sk-ds-1234',
          usage_count: 0,
          created_at: '',
          enabled: true,
        },
      ],
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider
    mocks.providerPoolStore.accountStatus = {
      deepseek: {
        provider_id: 'deepseek',
        key_id: 'key-1',
        key_hash: 'sk-ds-1234',
        kind: 'balance',
        primary_item_key: 'remaining',
        items: [
          { key: 'remaining', value: 12.34, currency: 'CNY' },
          { key: 'granted', value: 2, currency: 'CNY' },
          { key: 'topped_up', value: 10.34, currency: 'CNY' },
        ],
        fetched_at: Date.now(),
      },
    }

    const wrapper = mountSection()
    await flushPromises()

    expect(wrapper.find('[data-testid="provider-account-status-section"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('账户状态')
    expect(wrapper.text()).toContain('使用密钥 sk-ds-1234')
    expect(wrapper.text()).toContain('¥12.34')
    expect(wrapper.text()).toContain('¥10.34')
  })

  it('does not show temporarily unsupported official providers in the official chooser', async () => {
    const hiddenProvider = {
      ...createProvider('custom'),
      id: 'bedrock',
      name: 'Amazon Bedrock',
      type: 'platform',
      metadata_mode: 'catalog',
      base_url: '',
    }
    const visibleProvider = {
      ...createProvider('custom'),
      id: 'openai',
      name: 'OpenAI',
      type: 'builtin',
      metadata_mode: 'catalog',
      base_url: 'https://api.openai.com/v1',
    }
    mocks.providerPoolStore.providers = [hiddenProvider, visibleProvider]
    mocks.providerPoolStore.selectedProviderId = visibleProvider.id
    mocks.providerPoolStore.selectedProvider = visibleProvider

    const wrapper = mountSection()
    await flushPromises()

    const setupState = (wrapper.vm.$ as any).setupState
    setupState.openAddProviderModal()
    await flushPromises()

    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).not.toContain('Amazon Bedrock')
  })

  it('shows oauth-backed official providers in the provider management UI', async () => {
    const oauthProvider = {
      ...createProvider('custom'),
      id: 'google-antigravity',
      name: 'Google Cloud Code Assist (Antigravity)',
      type: 'platform',
      oauth: {
        connected: false,
      },
    }
    const visibleProvider = {
      ...createProvider('custom'),
      id: 'anthropic',
      name: 'Anthropic',
      type: 'builtin',
    }
    mocks.providerPoolStore.providers = [oauthProvider, visibleProvider]
    mocks.providerPoolStore.selectedProviderId = oauthProvider.id
    mocks.providerPoolStore.selectedProvider = oauthProvider

    const wrapper = mountSection()
    await flushPromises()

    const setupState = (wrapper.vm.$ as any).setupState
    setupState.openAddProviderModal()
    await flushPromises()

    expect(wrapper.text()).toContain('Google Cloud Code Assist (Antigravity)')
    expect(wrapper.text()).toContain('Anthropic')
  })

  it('shows ollama first in the add provider dialog', async () => {
    const openai = {
      ...createRankedProvider('openai', 100),
      type: 'builtin',
      metadata_mode: 'catalog',
    }
    const anthropic = {
      ...createRankedProvider('anthropic', 90),
      type: 'builtin',
      metadata_mode: 'catalog',
    }
    const ollama = {
      ...createRankedProvider('ollama', 10),
      name: 'Ollama',
      type: 'builtin',
      location: 'local',
      metadata_mode: 'catalog',
    }
    mocks.providerPoolStore.providers = [openai, anthropic, ollama]

    const wrapper = mountSection()
    await flushPromises()

    const setupState = (wrapper.vm.$ as any).setupState
    setupState.openAddProviderModal()
    await flushPromises()

    expect(setupState.officialProviderOptions.map((provider: { id: string }) => provider.id)).toEqual([
      'ollama',
      'openai',
      'anthropic',
    ])
  })

  it('renders localized location labels on provider cards', async () => {
    const cloudProvider = {
      ...createProvider('custom'),
      id: 'remote-provider',
      name: 'Remote Provider',
      location: 'cloud',
    }
    const localProvider = {
      ...createProvider('custom'),
      id: 'local-provider',
      name: 'Local Provider',
      location: 'local',
      priority: 5,
    }
    mocks.providerPoolStore.providers = [cloudProvider, localProvider]

    const wrapper = mountSection()
    await flushPromises()

    const providerCards = wrapper.findAll('[draggable="true"]')
    const remoteCard = providerCards.find((card) => card.text().includes('Remote Provider'))
    const localCard = providerCards.find((card) => card.text().includes('Local Provider'))

    expect(remoteCard?.text()).toContain('云端')
    expect(localCard?.text()).toContain('本地')
  })

  it('renders a clickable PinchBench badge for models with score metadata', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openai',
      name: 'OpenAI',
      type: 'builtin',
      metadata_mode: 'catalog',
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider
    mocks.providerPoolStore.models = [
      {
        id: 'gpt-4o-mini',
        provider_id: 'openai',
        name: 'gpt-4o-mini',
        display_name: 'GPT-4o Mini',
        enabled: true,
        capabilities: ['chat'],
        pinchbench_score: 75,
        pinchbench_url: 'https://pinchbench.com/model/openai/openai/gpt-4o-mini',
      },
    ]

    const wrapper = mountSection()
    await flushPromises()

    const badge = wrapper.find('[data-testid="pinchbench-badge"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('75.0')
    expect(badge.attributes('href')).toBe('https://pinchbench.com/model/openai/openai/gpt-4o-mini')
  })

  it('reuses provider-level PinchBench metadata for key-scoped models via heuristic matching', async () => {
    const provider = {
      ...createProvider('custom'),
      id: 'openai',
      name: 'OpenAI',
      type: 'builtin',
      metadata_mode: 'catalog',
      api_keys: [
        {
          id: 'key-1',
          key_hash: 'sk-test-1',
          usage_count: 0,
          created_at: '',
          enabled: true,
          models: [
            {
              id: 'openai/gpt-4o-mini',
              provider_id: 'openai',
              name: 'openai/gpt-4o-mini',
              display_name: 'openai/gpt-4o-mini',
              enabled: true,
              capabilities: ['chat'],
            },
          ],
        },
      ],
    }
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.selectedProviderId = provider.id
    mocks.providerPoolStore.selectedProvider = provider
    mocks.providerPoolStore.models = [
      {
        id: 'gpt-4o-mini',
        provider_id: 'openai',
        name: 'gpt-4o-mini',
        display_name: 'GPT-4o Mini',
        enabled: true,
        capabilities: ['chat'],
        pinchbench_score: 75,
        pinchbench_url: 'https://pinchbench.com/model/openai/openai/gpt-4o-mini',
      },
    ]

    const wrapper = mountSection()
    const setupState = (wrapper.vm.$ as any).setupState
    setupState.selectedKeyId = 'key-1'
    await flushPromises()

    const badge = wrapper.find('[data-testid="pinchbench-badge"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('75.0')
    expect(badge.attributes('href')).toBe('https://pinchbench.com/model/openai/openai/gpt-4o-mini')
  })

  it('reorders provider cards during dragover before drop', async () => {
    const providerA = createRankedProvider('alpha', 90)
    const providerB = createRankedProvider('beta', 60)
    const providerC = createRankedProvider('gamma', 30)
    mocks.providerPoolStore.providers = [providerA, providerB, providerC]

    const wrapper = mountSection()
    await flushPromises()

    const providerCards = () => wrapper.findAll('[draggable="true"]')
    const findProviderCard = (id: string) =>
      providerCards().find((card) => card.text().includes(id))
    const topCardIds = () =>
      providerCards()
        .slice(0, 3)
        .map((card) => {
          const text = card.text()
          if (text.includes('gamma')) return 'gamma'
          if (text.includes('beta')) return 'beta'
          return 'alpha'
        })
    const dataTransfer = {
      effectAllowed: '',
      dropEffect: '',
      setData: vi.fn(),
    }

    expect(topCardIds()).toEqual(['alpha', 'beta', 'gamma'])

    await findProviderCard('gamma')?.trigger('dragstart', { dataTransfer })
    await findProviderCard('alpha')?.trigger('dragover', { dataTransfer })
    await flushPromises()

    expect(topCardIds()).toEqual(['gamma', 'alpha', 'beta'])
    expect(mocks.providerPoolStore.syncPrioritiesToBackend).not.toHaveBeenCalled()
  })

  it('persists the previewed provider order on drop', async () => {
    const providerA = createRankedProvider('alpha', 90)
    const providerB = createRankedProvider('beta', 60)
    const providerC = createRankedProvider('gamma', 30)
    mocks.providerPoolStore.providers = [providerA, providerB, providerC]

    const wrapper = mountSection()
    await flushPromises()

    const providerCards = () => wrapper.findAll('[draggable="true"]')
    const findProviderCard = (id: string) =>
      providerCards().find((card) => card.text().includes(id))
    const topCardIds = () =>
      providerCards()
        .slice(0, 3)
        .map((card) => {
          const text = card.text()
          if (text.includes('gamma')) return 'gamma'
          if (text.includes('beta')) return 'beta'
          return 'alpha'
        })
    const dataTransfer = {
      effectAllowed: '',
      dropEffect: '',
      setData: vi.fn(),
    }

    await findProviderCard('gamma')?.trigger('dragstart', { dataTransfer })
    await findProviderCard('alpha')?.trigger('dragover', { dataTransfer })
    await findProviderCard('alpha')?.trigger('drop', { dataTransfer })
    await flushPromises()

    expect(mocks.providerPoolStore.updateProviderPriorityLocal).toHaveBeenNthCalledWith(
      1,
      'gamma',
      100
    )
    expect(mocks.providerPoolStore.updateProviderPriorityLocal).toHaveBeenNthCalledWith(
      2,
      'alpha',
      75
    )
    expect(mocks.providerPoolStore.updateProviderPriorityLocal).toHaveBeenNthCalledWith(
      3,
      'beta',
      50
    )
    expect(mocks.providerPoolStore.syncPrioritiesToBackend).toHaveBeenCalledWith([
      { id: 'gamma', priority: 100 },
      { id: 'alpha', priority: 75 },
      { id: 'beta', priority: 50 },
    ])
  })
})
