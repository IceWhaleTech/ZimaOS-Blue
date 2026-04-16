import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

type LocaleMessages = Record<string, unknown>

const requiredChannelsBannerKeys = ['partialLoadTitle', 'networkError'] as const

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

describe('channels banner locale coverage', () => {
  it('exposes localized channels partial-load banner copy in every enhanced locale', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]

    expect(entries).toHaveLength(27)
    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const enhancedReference = mergeHarnessLocale('en-US', localeModules[enUSPath]!.default)

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      const enhancedMessages = mergeHarnessLocale(locale as never, mod.default)

      for (const key of requiredChannelsBannerKeys) {
        const value = getPathValue(enhancedMessages, `channels.${key}`)
        expect(typeof value, `${locale} should expose channels.${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave channels.${key} empty`
        )
          .toBeGreaterThan(0)
      }

      if (locale === 'en-US' || locale === 'en-GB') continue

      for (const key of requiredChannelsBannerKeys) {
        expect(
          getPathValue(enhancedMessages, `channels.${key}`),
          `${locale} should localize channels.${key}`
        ).not.toBe(getPathValue(enhancedReference, `channels.${key}`))
      }
    }
  })
})
