import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TunnelStatus from '@/components/remote-access/TunnelStatus.vue'

const { getRemoteAccessQRCodeMock, getRemoteAccessDiagnosticsMock, getRemoteAccessLogsMock } =
  vi.hoisted(() => ({
    getRemoteAccessQRCodeMock: vi.fn(),
    getRemoteAccessDiagnosticsMock: vi.fn(),
    getRemoteAccessLogsMock: vi.fn(),
  }))

vi.mock('@/api/remote-access', () => ({
  getRemoteAccessQRCode: getRemoteAccessQRCodeMock,
  getRemoteAccessDiagnostics: getRemoteAccessDiagnosticsMock,
  getRemoteAccessLogs: getRemoteAccessLogsMock,
}))

vi.mock('@/utils/channelIcons', () => ({
  getTunnelProviderIcon: () => '',
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    openInBrowser: vi.fn(),
  }),
}))

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    missingWarn: false,
    fallbackWarn: false,
    messages: {
      'en-US': {
        common: {
          copy: 'Copy',
          openInNewTab: 'Open in new tab',
          refresh: 'Refresh',
        },
        remoteAccess: {
          connected: 'Connected',
          connecting: 'Connecting',
          disconnected: 'Disconnected',
          showQRCode: 'Show QR Code',
          qrCodeError: 'Failed to load QR code',
          accessUrl: 'Access URL',
          diagnostics: 'Diagnostics',
          logs: 'Logs',
          disconnect: 'Disconnect',
          cancel: 'Cancel',
          remainingTime: 'Remaining: {time}',
          diagnosticsTitle: 'Diagnostics',
          tunnelRunning: 'Tunnel running',
          firewallException: 'Firewall exception',
          provider: 'Provider',
          platform: 'Platform',
          troubleshootingHints: 'Hints',
          recentErrors: 'Recent errors',
          activeSession: 'Active session',
          startedAt: 'Started at',
          status: 'Status',
          error: 'Error',
          logsTitle: 'Logs',
          noLogs: 'No logs',
        },
      },
    },
  })
}

describe('TunnelStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getRemoteAccessQRCodeMock.mockResolvedValue({
      data: {
        success: true,
        url: 'https://blue.example.com',
        qr_url: 'https://blue.example.com/chat',
      },
    })
    getRemoteAccessDiagnosticsMock.mockResolvedValue({
      data: {
        success: true,
        diagnostics: {
          tunnel_running: true,
          firewall_exception: true,
          os: { platform: 'linux' },
          hints: [],
        },
      },
    })
    getRemoteAccessLogsMock.mockResolvedValue({
      data: {
        success: true,
        logs: [],
        limit: 50,
        offset: 0,
      },
    })
  })

  it('renders a QR code from the backend-provided qr_url instead of image data', async () => {
    const wrapper = mount(TunnelStatus, {
      props: {
        status: {
          active: true,
          url: 'https://blue.example.com',
          provider: 'auto',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(getRemoteAccessQRCodeMock).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-qr-value="https://blue.example.com/chat"]').exists()).toBe(true)
  })
})
