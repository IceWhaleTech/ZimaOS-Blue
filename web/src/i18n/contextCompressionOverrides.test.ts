import { describe, expect, it } from 'vitest'

import contextCompressionOverrides from './context-compression-overrides'
import { deepMergeMessages, type LocaleMessages } from './merge'

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function localeCodeFromFile(fileName: string): string {
  return fileName.replace(/\.ts$/, '')
}

function getByPath(source: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((value, part) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return undefined
    }
    return (value as Record<string, unknown>)[part]
  }, source)
}

const localeModules = import.meta.glob<{ default: LocaleMessages }>('./locales/*.ts', {
  eager: true,
})

const localeMessagesByCode = new Map(
  Object.entries(localeModules).map(([modulePath, mod]) => [
    localeCodeFromFile(fileNameFromModulePath(modulePath)),
    mod.default,
  ])
)

const localeCodes = [...localeMessagesByCode.keys()].sort()
const baseLocale = localeMessagesByCode.get('en-US') as LocaleMessages
const primaryLocales = ['en-US', 'zh-CN'] as const

const contextCompressionKeys = [
  'settings.smallModel.contextCompression',
  'settings.smallModel.contextCompressionHint',
  'settings.smallModel.contextCompressionMode',
  'settings.smallModel.contextCompressionModeHint',
  'settings.smallModel.contextCompressionModeAuto',
  'settings.smallModel.contextCompressionModeSmallModel',
  'settings.smallModel.contextCompressionModeOffline',
  'settings.smallModel.contextCompressionModeOff',
  'settings.smallModel.contextCompressionSuccessRate',
  'settings.smallModel.contextCompressionLatencyMs',
] as const

function buildMergedLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }
  const localeBase = locale === 'en-US' ? baseLocale : deepMergeMessages(baseLocale, localeMessages)
  return deepMergeMessages(
    localeBase,
    (contextCompressionOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
}

describe('context compression locale coverage', () => {
  it('covers every non-primary locale in the dedicated override file', () => {
    const expectedLocales = localeCodes
      .filter((locale) => !primaryLocales.includes(locale as (typeof primaryLocales)[number]))
      .sort()
    expect(Object.keys(contextCompressionOverrides).sort()).toEqual(expectedLocales)

    for (const locale of expectedLocales) {
      const overrides = (contextCompressionOverrides as Record<string, LocaleMessages | undefined>)[
        locale
      ]
      expect(overrides, `${locale} should have context-compression overrides`).toBeTruthy()
      for (const key of contextCompressionKeys) {
        const value = getByPath(overrides as LocaleMessages, key)
        expect(typeof value, `${locale} should override ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('keeps direct strings in the primary locale files', () => {
    for (const locale of primaryLocales) {
      const messages = localeMessagesByCode.get(locale)
      expect(messages, `${locale} should be loadable`).toBeTruthy()
      for (const key of contextCompressionKeys) {
        const value = getByPath(messages as LocaleMessages, key)
        expect(typeof value, `${locale} should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('keeps direct strings in all locale files', () => {
    for (const locale of localeCodes) {
      const messages = localeMessagesByCode.get(locale)
      expect(messages, `${locale} should be loadable`).toBeTruthy()
      for (const key of contextCompressionKeys) {
        const value = getByPath(messages as LocaleMessages, key)
        expect(typeof value, `${locale} locale file should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} locale file should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('exposes context compression strings in all 27 merged locales', () => {
    expect(localeCodes.length).toBe(27)
    for (const locale of localeCodes) {
      const messages = buildMergedLocaleMessages(locale)
      for (const key of contextCompressionKeys) {
        const value = getByPath(messages, key)
        expect(typeof value, `${locale} should expose ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty after merge`
        ).toBeGreaterThan(0)
      }
    }
  })
})
