import { describe, expect, it } from 'vitest'

import {
  harnessProviderRemediationCopyByLocale,
  harnessProviderRemediationGroup,
} from './harness-provider-remediation-backfills'

const localeKeys = [
  'ca-ES',
  'cs-CZ',
  'da-DK',
  'de-DE',
  'el-GR',
  'en-GB',
  'en-US',
  'es-ES',
  'fr-FR',
  'ga-IE',
  'hr-HR',
  'hu-HU',
  'it-IT',
  'ja-JP',
  'ko-KR',
  'ml-IN',
  'nb-NO',
  'nl-NL',
  'pl-PL',
  'pt-BR',
  'pt-PT',
  'ro-RO',
  'ru-RU',
  'sk-SK',
  'sv-SE',
  'zh-CN',
  'zh-TW',
] as const

describe('harness provider remediation catalog', () => {
  it('covers the full 27-locale set', () => {
    expect(Object.keys(harnessProviderRemediationCopyByLocale).sort()).toEqual([...localeKeys].sort())
  })

  it.each(localeKeys)('builds a complete group payload for %s', (locale) => {
    const copy = harnessProviderRemediationCopyByLocale[locale]
    expect(copy).toHaveLength(3)

    for (const value of copy) {
      expect(typeof value).toBe('string')
      expect(value.trim().length).toBeGreaterThan(0)
    }

    expect(harnessProviderRemediationGroup(locale)).toEqual({
      remediationInfraProviderAuth: copy[0],
      remediationInfraProviderQuota: copy[1],
      remediationInfraProviderBlocked: copy[2],
    })
  })
})
