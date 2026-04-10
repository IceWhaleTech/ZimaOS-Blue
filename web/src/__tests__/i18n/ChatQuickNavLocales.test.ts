import { describe, expect, it } from 'vitest'

import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'
import type { LocaleKey } from '@/i18n/locale-catalog'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function getLocaleCode(modulePath: string): LocaleKey {
  return fileNameFromModulePath(modulePath).replace(/\.ts$/, '') as LocaleKey
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

describe('chat quick nav locale coverage', () => {
  it('exposes quick navigation copy in the runtime locale tree for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)
      const runtimeMessages = mergeHarnessLocale(locale, mod.default)
      const title = getPathValue(runtimeMessages, 'chat.quickNav.title')
      const jumpToMessage = getPathValue(runtimeMessages, 'chat.quickNav.jumpToMessage')

      expect(typeof title, `${file} missing chat.quickNav.title`).toBe('string')
      expect(String(title).trim().length, `${file} empty chat.quickNav.title`).toBeGreaterThan(0)

      expect(typeof jumpToMessage, `${file} missing chat.quickNav.jumpToMessage`).toBe('string')
      expect(
        String(jumpToMessage).trim().length,
        `${file} empty chat.quickNav.jumpToMessage`
      ).toBeGreaterThan(0)
      expect(String(jumpToMessage), `${file} should preserve {preview} placeholder`).toContain(
        '{preview}'
      )
    }
  })

  it('keeps quick navigation labels localized outside English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSEntry = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))

    expect(enUSEntry).toBeTruthy()
    if (!enUSEntry) {
      throw new Error('Missing en-US locale module')
    }

    const englishMessages = mergeHarnessLocale('en-US', enUSEntry[1].default)
    const englishTitle = getPathValue(englishMessages, 'chat.quickNav.title')
    const englishJump = getPathValue(englishMessages, 'chat.quickNav.jumpToMessage')

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)
      if (locale === 'en-US' || locale === 'en-GB') continue

      const runtimeMessages = mergeHarnessLocale(locale, mod.default)
      expect(
        getPathValue(runtimeMessages, 'chat.quickNav.title'),
        `${file} should localize title`
      ).not.toBe(englishTitle)
      expect(
        getPathValue(runtimeMessages, 'chat.quickNav.jumpToMessage'),
        `${file} should localize jumpToMessage`
      ).not.toBe(englishJump)
    }
  })
})
