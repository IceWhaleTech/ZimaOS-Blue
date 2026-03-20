import { describe, expect, it } from 'vitest'

import harnessLocaleOverrides from '@/i18n/harness-locale-overrides'
import type { LocaleMessages } from '@/i18n/merge'

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

const requiredPaths = [
  'common.all',
  'common.notAvailable',
  'common.updatedAt',
  'nav.harness',
  'harness.groups.subtitle',
  'harness.groups.totalGroups',
  'harness.groups.emptyDescription',
  'harness.group.artifacts',
  'harness.group.retryFailed',
  'harness.group.unprofiled',
] as const

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

const localeMessagesByFile = new Map(
  Object.entries(localeModules).map(([modulePath, mod]) => [
    fileNameFromModulePath(modulePath),
    mod.default,
  ])
)

const localeSourceByFile = new Map(
  Object.entries(localeSourceModules).map(([modulePath, source]) => [
    fileNameFromModulePath(modulePath),
    source,
  ])
)

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

describe('Harness locale overrides', () => {
  it('covers every supported non-en-US locale', () => {
    expect(Object.keys(harnessLocaleOverrides).sort()).toEqual(
      localeKeys.filter((locale) => locale !== 'en-US').sort()
    )
  })

  it('is wired through all 27 locale source files', () => {
    expect(localeMessagesByFile.size).toBe(27)

    for (const locale of localeKeys) {
      const file = `${locale}.ts`
      const source = localeSourceByFile.get(file)
      expect(source, `${file} should be loadable as raw source`).toBeTruthy()
      expect(source, `${file} should expose common.all`).toMatch(/\ball:\s*/)
      expect(source, `${file} should expose common.notAvailable`).toMatch(/\bnotAvailable:\s*/)
      expect(source, `${file} should expose common.updatedAt`).toMatch(/\bupdatedAt:\s*/)
      expect(source, `${file} should expose nav.harness`).toMatch(/\bharness:\s*/)
    }
  })

  it.each(localeKeys)('provides harness copy for %s', (locale) => {
    const file = `${locale}.ts`
    const mergedMessages = localeMessagesByFile.get(file)
    expect(mergedMessages, `${file} should be loadable via import.meta.glob`).toBeTruthy()

    for (const path of requiredPaths) {
      const value = getPathValue(mergedMessages as LocaleMessages, path)
      expect(typeof value).toBe('string')
      expect(String(value).trim().length).toBeGreaterThan(0)
    }
  })
})
