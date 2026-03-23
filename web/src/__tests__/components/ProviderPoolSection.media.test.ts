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
    trialProviders: [] as Array<Record<string, unknown>>,
    trialQuota: null as Record<string, unknown> | null,
    fetchProviders: vi.fn(),
    fetchCustomPricing: vi.fn(),
    refreshModels: vi.fn(),
    updateProvider: vi.fn(),
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
        },
        providerPool: {
          description: 'LLM 配置',
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
    mocks.providerPoolStore.fetchProviders.mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchCustomPricing.mockResolvedValue([])
    mocks.providerPoolStore.refreshModels.mockResolvedValue({ success: true, models: [] })
    mocks.providerPoolStore.updateProvider.mockImplementation(async (_id, updates) => ({
      ...(mocks.providerPoolStore.selectedProvider || {}),
      ...updates,
    }))
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
})
