import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function getLocaleCode(modulePath: string): string {
  return fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

const requiredWeChatILinkKeys = [
  'channels.wechatILink',
  'channels.wechatProvider',
  'channels.wechatProviderWork',
  'channels.wechatProviderILink',
  'channels.placeholderILinkAPIBaseURL',
  'channels.apiBaseURL',
  'channels.wechatILinkPrimaryAction',
  'channels.wechatILinkScanAction',
  'channels.wechatILinkManualAction',
  'channels.wechatILinkScanHint',
  'channels.wechatILinkSetupCreating',
  'channels.wechatILinkSetupCreateFailed',
  'channels.wechatILinkSetupDescription',
  'channels.wechatILinkSetupStatus',
  'channels.wechatILinkOpenOnPhone',
  'channels.wechatILinkSetupEyebrow',
  'channels.wechatILinkSetupTitle',
  'channels.wechatILinkSetupMissingSession',
  'channels.wechatILinkSetupLoadFailed',
  'channels.wechatILinkSetupSubmit',
  'channels.wechatILinkSetupSubmitting',
  'channels.wechatILinkSetupSubmitFailed',
  'channels.wechatILinkSetupSuccess',
  'channels.wechatILinkPairingPayload',
  'channels.wechatILinkPairingPayloadPlaceholder',
  'channels.wechatILinkDesc',
  'channels.wechatILinkHint',
] as const

const nonEnglishGuardedKeys = {
  'channels.wechatProvider': 'WeChat Provider',
  'channels.apiBaseURL': 'API Base URL',
  'channels.wechatILinkPrimaryAction': 'Primary Action',
  'channels.wechatILinkScanAction': 'Scan To Configure',
  'channels.wechatILinkManualAction': 'Manual Config',
  'channels.wechatILinkSetupCreateFailed': 'Failed to create setup session',
  'channels.wechatILinkSetupTitle': 'Finish WeChat iLink Setup',
  'channels.wechatILinkDesc': 'Connect personal WeChat via iLink bot credentials',
  'channels.wechatILinkHint':
    'Use scan-to-config as the primary path, or fill API Base URL and Bot Token manually as fallback',
} as const

describe('WeChat iLink locale coverage', () => {
  it('exposes the full WeChat iLink copy block in all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default

      for (const key of requiredWeChatILinkKeys) {
        const value = getPathValue(messages, key)
        expect(typeof value, `${file} missing ${key}`).toBe('string')
        expect(String(value).trim().length, `${file} empty ${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps key WeChat iLink labels localized outside English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default

      if (locale === 'en-US' || locale === 'en-GB') {
        continue
      }

      for (const [key, englishValue] of Object.entries(nonEnglishGuardedKeys)) {
        const value = getPathValue(messages, key)
        expect(typeof value, `${file} missing ${key}`).toBe('string')
        expect(String(value).trim().length, `${file} empty ${key}`).toBeGreaterThan(0)
        expect(value, `${file} should not fall back to English for ${key}`).not.toBe(englishValue)
      }
    }
  })
})
