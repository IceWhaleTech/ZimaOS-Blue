import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'

import SecurityView from '@/views/SecurityView.vue'
import { approvalApi } from '@/api/approval'
import { securityApi } from '@/api/security'
import { settingsApi } from '@/api/settings'
import { systemApi } from '@/api/index'
import { sandboxApi } from '@/api/sandbox'
import { companionApi } from '@/api/companion'
import { getActiveConnections, getConnectionStats } from '@/api/connections'
import { useAuthStore } from '@/stores/auth'
import { PagePermissions } from '@/constants/pagePermissions'

vi.mock('@/api/approval', () => ({
  approvalApi: {
    listApprovedDirectories: vi.fn(),
    revokeApprovedDirectory: vi.fn(),
    listApprovedBrowserSites: vi.fn(),
    revokeApprovedBrowserSite: vi.fn(),
  },
}))

vi.mock('@/api/security', () => ({
  securityApi: {
    getPromptFirewall: vi.fn(),
    updatePromptFirewall: vi.fn(),
    addPromptFirewallRule: vi.fn(),
    updatePromptFirewallRule: vi.fn(),
    deletePromptFirewallRule: vi.fn(),
    runSecurityScan: vi.fn(),
    fixScanIssue: vi.fn(),
  },
}))

vi.mock('@/api/settings', () => ({
  settingsApi: {
    get: vi.fn(),
    patch: vi.fn(),
  },
}))

vi.mock('@/api/index', () => ({
  systemApi: {
    getLogs: vi.fn(),
  },
}))

vi.mock('@/api/sandbox', () => ({
  sandboxApi: {
    getInfo: vi.fn(),
    updateConfig: vi.fn(),
  },
}))

vi.mock('@/api/companion', () => ({
  companionApi: {
    listSessions: vi.fn(),
    getStats: vi.fn(),
  },
}))

vi.mock('@/api/connections', () => ({
  getActiveConnections: vi.fn(),
  getConnectionStats: vi.fn(),
}))

const replaceMock = vi.fn()
const pushMock = vi.fn()
const routeMock = {
  query: {} as Record<string, unknown>,
}

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeMock,
    useRouter: () => ({
      replace: replaceMock,
      push: pushMock,
    }),
  }
})

vi.mock('@/utils/securityScanSummary', () => ({
  formatSecurityScanSummary: vi.fn(() => 'All checks cached'),
  getVisibleSecurityScanSummaryMetrics: vi.fn(() => []),
}))

vi.mock('@/components/companion/SessionList.vue', () => ({
  default: { name: 'SessionList', template: '<div class="session-list-stub"></div>' },
}))

vi.mock('@/components/companion/SessionDetail.vue', () => ({
  default: { name: 'SessionDetail', template: '<div class="session-detail-stub"></div>' },
}))

vi.mock('@/components/security/FixPreviewDialog.vue', () => ({
  default: { name: 'FixPreviewDialog', template: '<div class="fix-preview-dialog-stub"></div>' },
}))

vi.mock('@/components/security/DataMaskingSettings.vue', () => ({
  default: {
    name: 'DataMaskingSettings',
    template: '<div class="data-masking-settings-stub"></div>',
  },
}))

vi.mock('@/components/security/MonitoringRetentionSettings.vue', () => ({
  default: {
    name: 'MonitoringRetentionSettings',
    template: '<div class="monitoring-retention-settings-stub"></div>',
  },
}))

vi.mock('@/components/settings/NetworkSettings.vue', () => ({
  default: { name: 'NetworkSettings', template: '<div class="network-settings-stub"></div>' },
}))

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en',
    fallbackLocale: 'en',
    missingWarn: false,
    fallbackWarn: false,
    messages: {
      en: {
        common: {
          loading: 'Loading',
          saving: 'Saving',
          refresh: 'Refresh',
          refreshing: 'Refreshing',
          enabled: 'Enabled',
          disabled: 'Disabled',
          saveFailed: 'Failed to save',
        },
        nav: {
          configuration: 'Configuration',
        },
        security: {
          title: 'Security',
          sandboxStatus: {
            title: 'Sandbox Status',
            description:
              'Review whether sandbox execution is enabled and inspect the runtime limits currently applied.',
            runtimeTitle: 'Runtime Configuration',
            enabled: 'Enabled',
            disabled: 'Disabled',
            checking: 'Checking...',
            unknown: 'Unknown',
            runtimeAvailable: 'Available',
            runtimeUnavailable: 'Unavailable',
          },
          scan: {
            checking: 'Checking...',
          },
        },
        sandbox: {
          notSupportedDesc:
            'Sandbox execution is not available on this platform or has not been configured.',
          config: {
            status: 'Status',
            defaultTimeout: 'Default Timeout',
            maxTimeout: 'Max Timeout',
            memoryLimit: 'Memory Limit',
            cpuLimit: 'CPU Limit',
            processLimit: 'Process Limit',
            network: 'Network Access',
          },
          errors: {
            fetchInfo: 'Failed to fetch sandbox information',
          },
        },
      },
    },
  })
}

function mountSecurityView(
  options: {
    role?: 'admin' | 'user' | 'guest'
    permissions?: string[]
    permissionsLoaded?: boolean
    token?: string | null
  } = {}
) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const authStore = useAuthStore()
  ;(authStore as any).user = { role: options.role ?? 'admin' }
  ;(authStore as any).permissions = options.permissions ?? []
  ;(authStore as any).permissionsLoaded = options.permissionsLoaded ?? false
  ;(authStore as any).token = options.token ?? 'test-token'

  return shallowMount(SecurityView, {
    global: {
      plugins: [pinia, createTestI18n()],
    },
  })
}

describe('SecurityView approved browser sites', () => {
  beforeEach(() => {
    routeMock.query = {}
    localStorageMock.clear()
    localStorageMock.setItem('security_last_scan_timestamp', new Date().toISOString())
    vi.clearAllMocks()
    replaceMock.mockReset()
    pushMock.mockReset()

    vi.mocked(securityApi.getPromptFirewall).mockResolvedValue({
      data: { enabled: true, rules: [], rule_count: 0 },
    } as never)
    vi.mocked(settingsApi.get).mockResolvedValue({
      data: {
        directory_whitelist_enabled: false,
        directory_whitelist: [],
      },
    } as never)
    vi.mocked(settingsApi.patch).mockImplementation(
      async (payload: unknown) => ({ data: payload }) as never
    )
    vi.mocked(approvalApi.listApprovedDirectories).mockResolvedValue({
      data: { entries: [] },
    } as never)
    vi.mocked(approvalApi.listApprovedBrowserSites).mockResolvedValue({
      data: {
        entries: [
          {
            id: 'site-1',
            origin: 'https://example.com',
            added_at: '2026-03-18T00:00:00Z',
            last_used: '2026-03-18T00:00:00Z',
          },
        ],
      },
    } as never)
    vi.mocked(approvalApi.revokeApprovedBrowserSite).mockResolvedValue({
      data: { deleted: true },
    } as never)
    vi.mocked(systemApi.getLogs).mockResolvedValue({ data: { logs: [] } } as never)
    vi.mocked(sandboxApi.getInfo).mockResolvedValue({
      data: {
        supported: true,
        default_timeout: '5m0s',
        max_timeout: '5m0s',
        memory_limit: 268435456,
        cpu_limit: 1,
        process_limit: 10,
        network_enabled: false,
      },
    } as never)
    vi.mocked(sandboxApi.updateConfig).mockImplementation(async (payload: unknown) => {
      const request = (payload || {}) as { network_enabled?: boolean }
      return {
        data: {
          supported: true,
          default_timeout: '5m0s',
          max_timeout: '5m0s',
          memory_limit: 268435456,
          cpu_limit: 1,
          process_limit: 10,
          network_enabled: Boolean(request.network_enabled),
        },
      } as never
    })
    vi.mocked(companionApi.listSessions).mockResolvedValue({
      data: { sessions: [] },
    } as never)
    vi.mocked(companionApi.getStats).mockResolvedValue({
      data: {
        active_sessions: 0,
        completed_sessions: 0,
        total_messages: 0,
        avg_messages_per_session: 0,
        avg_duration_ms: 0,
      },
    } as never)
    vi.mocked(getActiveConnections).mockResolvedValue({ connections: [] } as never)
    vi.mocked(getConnectionStats).mockResolvedValue({
      active_connections: 0,
      total_connections: 0,
      messages_sent: 0,
      messages_received: 0,
      bytes_sent: 0,
      bytes_received: 0,
    } as never)
  })

  afterEach(() => {
    localStorageMock.clear()
    vi.restoreAllMocks()
  })

  it('loads approved browser sites and revokes them from the controls tab', async () => {
    const wrapper = mountSecurityView()

    await flushPromises()

    expect(approvalApi.listApprovedBrowserSites).toHaveBeenCalledTimes(1)

    const controlsTab = wrapper.findAll('button[role="tab"]').find((node) => {
      return node.text().includes('Security Controls')
    })
    expect(controlsTab).toBeTruthy()

    await controlsTab!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Allowed Browser Sites')
    expect(wrapper.text()).toContain('https://example.com')

    const revokeButton = wrapper.findAll('button').find((node) => node.text().trim() === 'Revoke')
    expect(revokeButton).toBeTruthy()

    await revokeButton!.trigger('click')
    await flushPromises()

    expect(approvalApi.revokeApprovedBrowserSite).toHaveBeenCalledTimes(1)
    expect(approvalApi.revokeApprovedBrowserSite).toHaveBeenCalledWith('site-1')
    expect(wrapper.text()).not.toContain('https://example.com')

    wrapper.unmount()
  })

  it('renders the sandbox status section at the top of controls', async () => {
    localStorageMock.setItem(
      'security_last_scan_results',
      JSON.stringify([
        {
          id: 'sandbox_enabled',
          category: 'sandbox',
          name: 'Sandbox Execution',
          description: 'Check if code execution is sandboxed',
          status: 'passed',
          details: 'Sandbox execution is enabled',
        },
      ])
    )

    const wrapper = mountSecurityView()

    await flushPromises()

    const controlsTab = wrapper.findAll('button[role="tab"]').find((node) => {
      return node.text().includes('Security Controls')
    })
    expect(controlsTab).toBeTruthy()

    await controlsTab!.trigger('click')
    await flushPromises()

    expect(sandboxApi.getInfo).toHaveBeenCalledTimes(1)

    const sandboxSection = wrapper.get('[data-testid="sandbox-status-section"]')
    expect(sandboxSection.text()).toContain('Sandbox Status')
    expect(sandboxSection.text()).toContain('Enabled')
    expect(sandboxSection.text()).toContain('Sandbox execution is enabled')
    expect(sandboxSection.text()).toContain('Runtime Configuration')
    expect(sandboxSection.text()).toContain('Available')
    expect(sandboxSection.text()).toContain('Network Access')
    expect(sandboxSection.text()).toContain('Disabled')
    expect(sandboxSection.text()).toContain('5m0s')
    expect(sandboxSection.text()).toContain('256 MB')
    expect(sandboxSection.text()).toContain('Process Limit')

    wrapper.unmount()
  })

  it('toggles sandbox network access from the controls section', async () => {
    localStorageMock.setItem(
      'security_last_scan_results',
      JSON.stringify([
        {
          id: 'sandbox_enabled',
          category: 'sandbox',
          name: 'Sandbox Execution',
          description: 'Check if code execution is sandboxed',
          status: 'passed',
          details: 'Sandbox execution is enabled',
        },
      ])
    )

    const wrapper = mountSecurityView()

    await flushPromises()

    const controlsTab = wrapper.findAll('button[role="tab"]').find((node) => {
      return node.text().includes('Security Controls')
    })
    expect(controlsTab).toBeTruthy()

    await controlsTab!.trigger('click')
    await flushPromises()

    const networkSwitch = wrapper.get('[data-testid="sandbox-network-switch"]')
    expect(networkSwitch.attributes('aria-checked')).toBe('false')

    await networkSwitch.trigger('click')
    await flushPromises()

    expect(sandboxApi.updateConfig).toHaveBeenCalledTimes(1)
    expect(sandboxApi.updateConfig).toHaveBeenCalledWith({ network_enabled: true })
    expect(wrapper.get('[data-testid="sandbox-network-switch"]').attributes('aria-checked')).toBe(
      'true'
    )

    wrapper.unmount()
  })

  it('renders directory whitelist inside the authorized directories panel', async () => {
    vi.mocked(settingsApi.get).mockResolvedValueOnce({
      data: {
        directory_whitelist_enabled: true,
        directory_whitelist: [{ path: '/tmp/external-docs', alias: 'docs' }],
      },
    } as never)
    vi.mocked(approvalApi.listApprovedDirectories).mockResolvedValueOnce({
      data: {
        entries: [
          {
            id: 'dir-1',
            path: '/tmp/always-allowed',
            added_at: '2026-03-18T00:00:00Z',
            last_used: '2026-03-19T00:00:00Z',
          },
        ],
      },
    } as never)

    const wrapper = mountSecurityView()

    await flushPromises()

    const controlsTab = wrapper.findAll('button[role="tab"]').find((node) => {
      return node.text().includes('Security Controls')
    })
    expect(controlsTab).toBeTruthy()

    await controlsTab!.trigger('click')
    await flushPromises()

    const mergedPanel = wrapper.get('[data-testid="authorized-directories-panel"]')
    expect(mergedPanel.text()).toContain('Authorized Directories')
    expect(mergedPanel.text()).toContain('Whitelist')
    expect(mergedPanel.text()).toContain('/tmp/external-docs')
    expect(mergedPanel.text()).toContain('docs')
    expect(mergedPanel.text()).toContain('/tmp/always-allowed')

    const cardHeadings = wrapper.findAll('h3').map((node) => node.text())
    expect(cardHeadings).toContain('Authorized Directories')
    expect(cardHeadings).not.toContain('Whitelist')

    wrapper.unmount()
  })

  it('keeps network settings under controls without exposing a harness tab', async () => {
    const wrapper = mountSecurityView()

    await flushPromises()

    const tabLabels = wrapper.findAll('button[role="tab"]').map((node) => node.text())

    expect(tabLabels.some((label) => label.includes('Harness'))).toBe(false)
    expect(tabLabels.some((label) => label.trim() === 'Network')).toBe(false)

    const controlsTab = wrapper.findAll('button[role="tab"]').find((node) => {
      return node.text().includes('Security Controls')
    })
    expect(controlsTab).toBeTruthy()

    await controlsTab!.trigger('click')
    await flushPromises()

    expect(wrapper.findComponent({ name: 'NetworkSettings' }).exists()).toBe(true)

    wrapper.unmount()
  })

  it('normalizes a legacy harness tab query back to overview', async () => {
    routeMock.query = { tab: 'harness' }

    const wrapper = mountSecurityView()

    await flushPromises()

    const selectedTab = wrapper.findAll('button[role="tab"]').find((node) => {
      return node.attributes('aria-selected') === 'true'
    })

    expect(selectedTab?.text()).toContain('Overview')
    expect(wrapper.text()).not.toContain('Open in Automation')
    expect(replaceMock).toHaveBeenCalledWith({
      query: {
        tab: 'overview',
      },
    })

    wrapper.unmount()
  })

  it('does not render a harness tab for security-only users', async () => {
    const wrapper = mountSecurityView({
      role: 'user',
      permissions: [PagePermissions.SECURITY],
      permissionsLoaded: true,
    })

    await flushPromises()

    const tabLabels = wrapper.findAll('button[role="tab"]').map((node) => node.text())
    expect(tabLabels.some((label) => label.includes('Harness'))).toBe(false)
    expect(pushMock).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('hides the overview status banner when cached scan results include failures', async () => {
    localStorageMock.setItem(
      'security_last_scan_results',
      JSON.stringify([
        {
          id: 'prompt-guard',
          category: 'ai',
          name: 'Prompt Guard',
          description: 'Prompt injection protection',
          status: 'failed',
          details: 'Protection is disabled',
        },
      ])
    )

    const wrapper = mountSecurityView()

    await flushPromises()

    expect(wrapper.find('.security-status-banner').exists()).toBe(false)
    expect(wrapper.find('.security-scan-panel').exists()).toBe(true)

    wrapper.unmount()
  })
})
