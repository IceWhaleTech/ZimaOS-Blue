import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const storageState = new Map<string, string>()
const localStorageMock = {
  getItem: (key: string) => storageState.get(key) ?? null,
  setItem: (key: string, value: string) => {
    storageState.set(key, value)
  },
  removeItem: (key: string) => {
    storageState.delete(key)
  },
  clear: () => {
    storageState.clear()
  },
}

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'tenant-1' } }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@/stores/tenant', () => ({
  useTenantStore: () => ({
    currentTenant: {
      id: 'tenant-1',
      name: 'Workspace Alpha',
      slug: 'workspace-alpha',
      owner_id: 'user-1',
    },
    loading: false,
    selectTenant: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('@/api/proxy', () => ({
  proxyApi: {
    getSecurityAlerts: vi.fn(),
    getGuardStats: vi.fn(),
    getAuthStats: vi.fn(),
  },
}))

vi.mock('@/api/tts', () => ({
  ttsApi: {
    setConsent: vi.fn(),
  },
}))

vi.mock('@/api/extauth', () => ({
  extauthAdminApi: {
    listAllProviders: vi.fn(),
    createProvider: vi.fn(),
    updateProvider: vi.fn(),
    toggleProvider: vi.fn(),
    deleteProvider: vi.fn(),
  },
}))

vi.mock('@/api/tenant', () => ({
  listMembers: vi.fn(),
  listInvitations: vi.fn(),
  getTenantSettings: vi.fn(),
  getTenantLimits: vi.fn(),
  inviteMember: vi.fn(),
  updateMember: vi.fn(),
  removeMember: vi.fn(),
  cancelInvitation: vi.fn(),
  updateTenantSettings: vi.fn(),
  getRoleColor: (role: string) => {
    if (role === 'owner') return '#8b5cf6'
    if (role === 'admin') return '#3b82f6'
    return '#6b7280'
  },
  formatStorageSize: (bytes: number) => `${bytes} B`,
}))

import { i18n, setLocale } from '@/i18n'
import SecurityAlerts from '@/components/SecurityAlerts.vue'
import PrivacyConsentDialog from '@/components/tts/PrivacyConsentDialog.vue'
import AuthProvidersView from '@/views/AuthProvidersView.vue'
import TenantDetailView from '@/views/TenantDetailView.vue'
import { proxyApi } from '@/api/proxy'
import { ttsApi } from '@/api/tts'
import { extauthAdminApi } from '@/api/extauth'
import * as tenantApi from '@/api/tenant'

describe('remaining UI i18n sweep', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    storageState.clear()
    vi.stubGlobal('localStorage', localStorageMock)
    if (typeof window !== 'undefined') {
      Object.defineProperty(window, 'localStorage', {
        value: localStorageMock,
        configurable: true,
      })
    }
    localStorage.setItem('user_id', 'user-1')
    await setLocale('zh-CN')

    vi.mocked(proxyApi.getSecurityAlerts).mockResolvedValue({
      data: [
        {
          id: 'alert-1',
          type: 'injection',
          severity: 'high',
          message: 'Blocked suspicious prompt',
          details: 'Matched prompt signature',
          source_ip: '127.0.0.1',
          resolved: false,
          timestamp: '2026-03-09T00:00:00Z',
        },
      ],
    })
    vi.mocked(proxyApi.getGuardStats).mockResolvedValue({
      data: {
        enabled: true,
        pattern_count: 12,
        detection_count: 5,
        blocked_count: 3,
      },
    })
    vi.mocked(proxyApi.getAuthStats).mockResolvedValue({
      data: {
        auth_enabled: true,
        api_key_count: 2,
        auth_failures: 1,
        rate_limit_hits: 4,
      },
    })

    vi.mocked(ttsApi.setConsent).mockResolvedValue(undefined)

    vi.mocked(extauthAdminApi.listAllProviders).mockResolvedValue({
      data: [
        {
          id: 'google',
          name: 'Google Login',
          type: 'google',
          client_id: 'google-client-id-1234567890',
          client_secret: '',
          redirect_url: 'https://example.com/auth/callback/google',
          issuer_url: 'https://accounts.google.com',
          auth_url: '',
          token_url: '',
          userinfo_url: '',
          enabled: true,
          auto_create_user: true,
          default_role: 'user',
          scopes: ['openid', 'profile', 'email'],
          allowed_domains: [],
          order: 0,
        },
      ],
    })

    vi.mocked(tenantApi.listMembers).mockResolvedValue({
      members: [
        {
          id: 'member-1',
          tenant_id: 'tenant-1',
          user_id: 'user-1',
          username: 'orca',
          email: 'orca@example.com',
          role: 'owner',
          joined_at: '2026-03-08T00:00:00Z',
        },
      ],
    })
    vi.mocked(tenantApi.listInvitations).mockResolvedValue({ invitations: [] })
    vi.mocked(tenantApi.getTenantSettings).mockResolvedValue({
      default_language: 'zh',
      timezone: 'America/New_York',
      features: {},
    })
    vi.mocked(tenantApi.getTenantLimits).mockResolvedValue({
      max_users: 10,
      max_storage: 1024,
      max_api_requests: 1000,
      max_workflows: 5,
      max_channels: 3,
    })
  })

  it('localizes security alerts with real locale overrides', async () => {
    const wrapper = mount(SecurityAlerts, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('安全告警')
    expect(wrapper.text()).toContain('严重/高危')
    expect(wrapper.text()).toContain('Prompt Guard 状态')
    expect(wrapper.text()).toContain('提示词注入')
    expect(wrapper.text()).not.toContain('Security Alerts')
  })

  it('localizes the Edge TTS privacy consent dialog', async () => {
    const wrapper = mount(PrivacyConsentDialog, {
      props: {
        visible: true,
      },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('隐私提示：Microsoft Edge TTS')
    expect(wrapper.text()).toContain('隐私信息：')
    expect(wrapper.text()).toContain('此账号不再显示')
    expect(wrapper.text()).toContain('接受并继续')
    expect(wrapper.text()).not.toContain('Privacy Notice')
  })

  it('localizes auth provider card metadata and modal labels', async () => {
    const wrapper = mount(AuthProvidersView, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('认证提供商')
    expect(wrapper.text()).toContain('提供商 ID: google')
    expect(wrapper.text()).toContain('客户端 ID:')

    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('添加认证提供商')
    expect(wrapper.text()).toContain('提供商类型')
    expect(wrapper.text()).toContain('+ 添加范围')
    expect(wrapper.text()).not.toContain('Add Authentication Provider')
  })

  it('localizes tenant navigation, roles, and timezone labels', async () => {
    const wrapper = mount(TenantDetailView, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('返回')
    expect(wrapper.text()).toContain('所有者')

    const settingsTab = wrapper.findAll('button').find((button) => button.text().includes('设置'))
    expect(settingsTab).toBeTruthy()
    await settingsTab!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('美国东部时间')
    expect(wrapper.text()).not.toContain('Eastern Time')
  })
})
