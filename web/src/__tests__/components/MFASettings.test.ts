import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import MFASettings from '@/components/MFASettings.vue'

const { getStatusMock, setupMock } = vi.hoisted(() => ({
  getStatusMock: vi.fn(),
  setupMock: vi.fn(),
}))

vi.mock('@/api/mfa', () => ({
  mfaApi: {
    getStatus: getStatusMock,
    setup: setupMock,
    verify: vi.fn(),
    disable: vi.fn(),
    getRecoveryCodes: vi.fn(),
    regenerateRecoveryCodes: vi.fn(),
  },
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
          loading: 'Loading',
          verifying: 'Verifying',
          processing: 'Processing',
          cancel: 'Cancel',
          done: 'Done',
          copy: 'Copy',
          copied: 'Copied',
        },
        mfa: {
          title: 'MFA',
          enabled: 'Enabled',
          disabled: 'Disabled',
          enabledDescription: 'Enabled description',
          disabledDescription: 'Disabled description',
          recoveryCodesRemaining: 'Recovery codes left: {count}',
          setup: 'Set up',
          regenerateCodes: 'Regenerate codes',
          disable: 'Disable',
          step1Title: 'Step 1',
          step1Description: 'Scan the QR code',
          step2Title: 'Step 2',
          step2Description: 'Enter the code',
          manualEntry: 'Manual entry',
          enterCode: 'Enter code',
          verify: 'Verify',
          recoveryCodes: 'Recovery codes',
          recoveryCodesWarning: 'Save these codes',
          disableTitle: 'Disable MFA',
          disableWarning: 'This will disable MFA',
          enterPassword: 'Password',
          passwordPlaceholder: 'Password',
          confirmDisable: 'Disable MFA',
          setupFailed: 'Setup failed',
          enabledSuccessfully: 'Enabled',
          invalidCode: 'Invalid code',
          disabledSuccessfully: 'Disabled',
          disableFailed: 'Disable failed',
          codesRegenerated: 'Regenerated',
          regenerateFailed: 'Regenerate failed',
        },
      },
    },
  })
}

describe('MFASettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getStatusMock.mockResolvedValue({
      data: {
        enabled: false,
        recovery_codes_remaining: 0,
      },
    })
    setupMock.mockResolvedValue({
      data: {
        secret: 'SECRET123',
        uri: 'otpauth://totp/ZimaOS:testuser?secret=SECRET123&issuer=ZimaOS',
      },
    })
  })

  it('requests setup and renders a QR code from the provisioning URI', async () => {
    const wrapper = mount(MFASettings, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(setupMock).toHaveBeenCalledWith()
    expect(
      wrapper.find(
        '[data-qr-value="otpauth://totp/ZimaOS:testuser?secret=SECRET123&issuer=ZimaOS"]'
      ).exists()
    ).toBe(true)
  })
})
