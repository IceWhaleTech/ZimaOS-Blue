import { describe, expect, it } from 'vitest'

import { harnessFailureLabelHint } from '@/utils/harnessFailureHints'

const translate = (key: string, fallback?: string) => {
  const messages: Record<string, string> = {
    'harness.group.remediationInfraProviderAuth':
      'Reconnect provider credentials and retry once authentication is healthy.',
    'harness.group.remediationInfraProviderQuota':
      'Restore provider quota or credits before retrying this run.',
    'harness.group.remediationInfraProviderBlocked':
      'Inspect provider availability, overload, or rate-limit signals, then retry when the provider path is healthy.',
    'harness.group.remediationGeneric':
      'Inspect linked runs, checks, and artifacts to align the runtime output with the declared contract.',
  }
  return messages[key] || fallback || key
}

describe('harnessFailureLabelHint', () => {
  it('returns provider-specific remediation for infra provider failures', () => {
    expect(harnessFailureLabelHint('infra_provider_auth', translate)).toBe(
      'Reconnect provider credentials and retry once authentication is healthy.'
    )
    expect(harnessFailureLabelHint('infra_provider_quota', translate)).toBe(
      'Restore provider quota or credits before retrying this run.'
    )
    expect(harnessFailureLabelHint('infra_provider_blocked', translate)).toBe(
      'Inspect provider availability, overload, or rate-limit signals, then retry when the provider path is healthy.'
    )
  })

  it('keeps the generic remediation for unknown labels', () => {
    expect(harnessFailureLabelHint('unknown_label', translate)).toBe(
      'Inspect linked runs, checks, and artifacts to align the runtime output with the declared contract.'
    )
  })
})
