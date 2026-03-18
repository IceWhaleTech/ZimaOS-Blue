import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import SecurityView from '@/views/SecurityView.vue'
import { approvalApi } from '@/api/approval'
import { securityApi } from '@/api/security'
import { systemApi } from '@/api/index'
import { companionApi } from '@/api/companion'
import { getActiveConnections, getConnectionStats } from '@/api/connections'

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

vi.mock('@/api/index', () => ({
  systemApi: {
    getLogs: vi.fn(),
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
        nav: {
          configuration: 'Configuration',
        },
        security: {
          title: 'Security',
        },
      },
    },
  })
}

function mountSecurityView() {
  return shallowMount(SecurityView, {
    global: {
      plugins: [createTestI18n()],
    },
  })
}

describe('SecurityView approved browser sites', () => {
  beforeEach(() => {
    localStorageMock.clear()
    localStorageMock.setItem('security_last_scan_timestamp', new Date().toISOString())
    vi.clearAllMocks()

    vi.mocked(securityApi.getPromptFirewall).mockResolvedValue({
      data: { enabled: true, rules: [], rule_count: 0 },
    } as never)
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
