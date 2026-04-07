import { describe, expect, it } from 'vitest'

import { getSecurityScanItemDetailI18n } from './securityScanItemLocalization'

describe('getSecurityScanItemDetailI18n', () => {
  it('extracts password length detail params from current scanner messages', () => {
    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_password_length',
        status: 'passed',
        details: 'Password minimum length is 12 characters (meets requirement)',
      })
    ).toEqual({
      key: 'security.scan.items.auth_password_length.details.passed',
      params: { length: 12 },
    })

    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_password_length',
        status: 'warning',
        details:
          'Password minimum length is 8 characters. Recommend increasing to 12+ for better security.',
      })
    ).toEqual({
      key: 'security.scan.items.auth_password_length.details.warning',
      params: { length: 8 },
    })

    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_password_length',
        status: 'failed',
        details:
          'Password minimum length (6) is too short. Minimum 8 characters required, 12+ recommended.',
      })
    ).toEqual({
      key: 'security.scan.items.auth_password_length.details.failed',
      params: { length: 6 },
    })
  })

  it('extracts token expiration detail params from current scanner messages', () => {
    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_token_expiration',
        status: 'passed',
        details: 'Token expires in 60 minutes',
      })
    ).toEqual({
      key: 'security.scan.items.auth_token_expiration.details.passed',
      params: { minutes: 60 },
    })

    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_token_expiration',
        status: 'warning',
        details:
          'Token expiration (4 hours) is long. Consider shorter duration for sensitive operations.',
      })
    ).toEqual({
      key: 'security.scan.items.auth_token_expiration.details.warningHours',
      params: { hours: 4 },
    })

    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_token_expiration',
        status: 'warning',
        details: 'Token expiration exceeds 8 hours. This increases risk of token theft.',
      })
    ).toEqual({
      key: 'security.scan.items.auth_token_expiration.details.warningTooLong',
    })

    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_token_expiration',
        status: 'failed',
        details:
          'Token expiration is not set. Check security.jwt.expiration in the loaded security configuration.',
      })
    ).toEqual({
      key: 'security.scan.items.auth_token_expiration.details.failedNotSet',
    })
  })

  it('localizes MFA details for both scanner and legacy handler ids', () => {
    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_mfa_available',
        status: 'passed',
        details: 'MFA is required for all users',
      })
    ).toEqual({
      key: 'security.scan.items.auth_mfa_available.details.passed',
    })

    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_mfa_available',
        status: 'warning',
        details: 'MFA is available but not required. Consider enforcing MFA for enhanced security.',
      })
    ).toEqual({
      key: 'security.scan.items.auth_mfa_available.details.warning',
    })

    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_mfa_enabled',
        status: 'passed',
        details: 'MFA is available for users',
      })
    ).toEqual({
      key: 'security.scan.items.auth_mfa_enabled.details.passed',
    })
  })

  it('returns null for unrelated detail text', () => {
    expect(
      getSecurityScanItemDetailI18n({
        id: 'auth_password_length',
        status: 'warning',
        details: 'Something else entirely',
      })
    ).toBeNull()
  })
})
