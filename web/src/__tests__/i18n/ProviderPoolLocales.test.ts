import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

type LocaleMessages = Record<string, unknown>

const requiredKeys = [
  'advancedOptions',
  'addProviderHint',
  'collapse',
  'expand',
  'location',
  'locationCloud',
  'locationLocal',
  'locationHint',
  'officialProvider',
] as const

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

describe('provider pool locale coverage', () => {
  it('exposes add-provider advanced and location copy in every final locale module', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    expect(messagesByFile.size).toBe(27)

    for (const [fileName, messages] of messagesByFile) {
      for (const key of requiredKeys) {
        const value = getPathValue(messages, `providerPool.${key}`)
        expect(typeof value, `${fileName} should expose providerPool.${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${fileName} should not leave providerPool.${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('does not fall back to raw Cloud or Local in non-English locales', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    for (const [fileName, messages] of messagesByFile) {
      if (fileName === 'en-US.ts' || fileName === 'en-GB.ts') continue

      expect(getPathValue(messages, 'providerPool.locationCloud'), `${fileName} should localize Cloud`).not.toBe(
        'Cloud'
      )
      expect(getPathValue(messages, 'providerPool.locationLocal'), `${fileName} should localize Local`).not.toBe(
        'Local'
      )
    }
  })

  it('renames the chooser heading away from the old official-provider wording', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    for (const [fileName, messages] of messagesByFile) {
      const heading = getPathValue(messages, 'providerPool.officialProvider')
      expect(typeof heading, `${fileName} should expose providerPool.officialProvider`).toBe('string')
      expect(String(heading).trim().length, `${fileName} should not leave providerPool.officialProvider empty`).toBeGreaterThan(0)
      expect(heading, `${fileName} should no longer use the old English chooser heading`).not.toBe(
        'Official Provider'
      )
    }
  })

  it('backfills provider tab labels in every enhanced locale', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]
    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const enhancedReference = mergeHarnessLocale('en-US', localeModules[enUSPath]!.default)
    const requiredTabKeys = ['all', 'trial', 'builtin', 'platform', 'other', 'custom', 'media']

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      const enhancedMessages = mergeHarnessLocale(locale as never, mod.default)

      for (const key of requiredTabKeys) {
        const value = getPathValue(enhancedMessages, `providerPool.tabs.${key}`)
        expect(typeof value, `${locale} should expose providerPool.tabs.${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave providerPool.tabs.${key} empty`
        ).toBeGreaterThan(0)
      }

      if (locale === 'en-US' || locale === 'en-GB') continue

      for (const key of ['all', 'builtin', 'custom']) {
        expect(
          getPathValue(enhancedMessages, `providerPool.tabs.${key}`),
          `${locale} should localize providerPool.tabs.${key}`
        ).not.toBe(getPathValue(enhancedReference, `providerPool.tabs.${key}`))
      }
    }
  })
})
