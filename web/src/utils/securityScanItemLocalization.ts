export type SecurityScanItemStatus = 'pending' | 'scanning' | 'passed' | 'warning' | 'failed'

export interface SecurityScanItemDetailInput {
  id: string
  status: SecurityScanItemStatus
  details?: string
}

export interface SecurityScanItemDetailI18n {
  key: string
  params?: Record<string, number>
}

function matchNumber(details: string, pattern: RegExp, key: string, param: string) {
  const match = details.match(pattern)
  if (!match) return null
  const value = Number(match[1])
  if (!Number.isFinite(value)) return null
  return {
    key,
    params: {
      [param]: value,
    },
  } satisfies SecurityScanItemDetailI18n
}

function getPasswordLengthDetailI18n(details: string): SecurityScanItemDetailI18n | null {
  return (
    matchNumber(
      details,
      /^Password minimum length is (\d+) characters(?: \(meets requirement\))?$/,
      'security.scan.items.auth_password_length.details.passed',
      'length'
    ) ||
    matchNumber(
      details,
      /^Password minimum length is (\d+) characters(?:\. Recommend increasing to 12\+ for better security\.|, recommend 12\+)$/,
      'security.scan.items.auth_password_length.details.warning',
      'length'
    ) ||
    matchNumber(
      details,
      /^Password minimum length \((\d+)\) is too short\. Minimum 8 characters required, 12\+ recommended\.$/,
      'security.scan.items.auth_password_length.details.failed',
      'length'
    ) ||
    matchNumber(
      details,
      /^Password minimum length is too short: (\d+)$/,
      'security.scan.items.auth_password_length.details.failed',
      'length'
    )
  )
}

function getTokenExpirationDetailI18n(details: string): SecurityScanItemDetailI18n | null {
  if (details === 'Token expiration exceeds 8 hours. This increases risk of token theft.') {
    return {
      key: 'security.scan.items.auth_token_expiration.details.warningTooLong',
    }
  }

  if (
    details ===
    'Token expiration is not set. Check security.jwt.expiration in the loaded security configuration.'
  ) {
    return {
      key: 'security.scan.items.auth_token_expiration.details.failedNotSet',
    }
  }

  return (
    matchNumber(
      details,
      /^Token expires in (\d+) minutes$/,
      'security.scan.items.auth_token_expiration.details.passed',
      'minutes'
    ) ||
    matchNumber(
      details,
      /^Token expiration \((\d+) hours\) is long\. Consider shorter duration for sensitive operations\.$/,
      'security.scan.items.auth_token_expiration.details.warningHours',
      'hours'
    )
  )
}

export function getSecurityScanItemDetailI18n(
  item: SecurityScanItemDetailInput
): SecurityScanItemDetailI18n | null {
  const details = item.details?.trim()
  if (!details) return null

  switch (item.id) {
    case 'auth_password_length':
      return getPasswordLengthDetailI18n(details)
    case 'auth_token_expiration':
      return getTokenExpirationDetailI18n(details)
    case 'auth_mfa_available':
      if (details === 'MFA is required for all users') {
        return { key: 'security.scan.items.auth_mfa_available.details.passed' }
      }
      if (
        details ===
        'MFA is available but not required. Consider enforcing MFA for enhanced security.'
      ) {
        return { key: 'security.scan.items.auth_mfa_available.details.warning' }
      }
      return null
    case 'auth_mfa_enabled':
      if (details === 'MFA is available for users') {
        return { key: 'security.scan.items.auth_mfa_enabled.details.passed' }
      }
      return null
    default:
      return null
  }
}
