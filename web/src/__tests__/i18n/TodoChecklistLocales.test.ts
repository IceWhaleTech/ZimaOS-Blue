import { describe, expect, it } from 'vitest'

import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'
import type { LocaleKey } from '@/i18n/locale-catalog'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

const localeSourceModules = import.meta.glob('@/i18n/locales/*.ts', {
  eager: true,
  query: '?raw',
  import: 'default',
}) as Record<string, string>

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

describe('todo checklist locale coverage', () => {
  it('declares todo checklist progress copy in all 27 locale source files', () => {
    const sources = new Map(
      Object.entries(localeSourceModules).map(([modulePath, source]) => [
        fileNameFromModulePath(modulePath),
        source,
      ])
    )

    expect(sources.size).toBe(27)

    for (const [fileName, source] of sources) {
      expect(source, `${fileName} should explicitly declare chat.todoChecklist`).toMatch(
        /todoChecklist\s*:\s*{[\s\S]*?progress\s*:\s*['"]/
      )
    }
  })

  it('exposes localized todo checklist progress copy for all 27 locales at runtime', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))

    expect(entries).toHaveLength(27)

    const enUSEntry = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))
    expect(enUSEntry).toBeTruthy()
    if (!enUSEntry) {
      throw new Error('Missing en-US locale module')
    }

    const englishRuntimeMessages = mergeHarnessLocale('en-US', enUSEntry[1].default)
    const englishValue = getPathValue(englishRuntimeMessages, 'chat.todoChecklist.progress')

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)
      const runtimeMessages = mergeHarnessLocale(locale, mod.default)
      const value = getPathValue(runtimeMessages, 'chat.todoChecklist.progress')

      expect(typeof value, `${file} missing chat.todoChecklist.progress`).toBe('string')
      expect(
        String(value).trim().length,
        `${file} should not leave chat.todoChecklist.progress empty`
      ).toBeGreaterThan(0)
      expect(String(value), `${file} should preserve {completed}`).toContain('{completed}')
      expect(String(value), `${file} should preserve {total}`).toContain('{total}')

      if (locale !== 'en-US' && locale !== 'en-GB') {
        expect(value, `${file} should not fall back to the en-US checklist copy`).not.toBe(
          englishValue
        )
      }
    }
  })
})
