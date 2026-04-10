import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const requiredKeys = [
  'eyebrow',
  'title',
  'description',
  'dismissAction',
  'manualAction',
  'primaryAction',
  'currentRoute',
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

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

describe('chat provider failover locale coverage', () => {
  it('declares providerFailover in all 27 locale source files', () => {
    const sources = new Map(
      Object.entries(localeSourceModules).map(([modulePath, source]) => [
        fileNameFromModulePath(modulePath),
        source,
      ])
    )

    expect(sources.size).toBe(27)

    for (const [fileName, source] of sources) {
      expect(source, `${fileName} should declare chat.providerFailover`).toMatch(
        /["']?providerFailover["']?\s*:/
      )
    }
  })

  it('exposes non-empty provider failover keys in every final locale module', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    expect(messagesByFile.size).toBe(27)

    for (const [fileName, messages] of messagesByFile) {
      for (const key of requiredKeys) {
        const value = getPathValue(messages, `chat.providerFailover.${key}`)
        expect(typeof value, `${fileName} should expose chat.providerFailover.${key}`).toBe(
          'string'
        )
        expect(
          String(value).trim().length,
          `${fileName} should not leave chat.providerFailover.${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('compiles provider failover copy and localizes non-English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSPath = entries.find(
      ([modulePath]) => fileNameFromModulePath(modulePath) === 'en-US.ts'
    )?.[0]
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const referenceMessages = localeModules[enUSPath]?.default
    if (!referenceMessages) {
      throw new Error('Missing en-US locale messages')
    }

    const referenceDescription = getPathValue(
      referenceMessages,
      'chat.providerFailover.description'
    )
    expect(typeof referenceDescription).toBe('string')

    const exemptLocales = new Set(['en-US', 'en-GB'])

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      const i18n = createI18n({
        legacy: false,
        locale,
        fallbackLocale: locale,
        messages: {
          [locale]: mod.default,
        },
      })

      const rendered = i18n.global.t('chat.providerFailover.description', {
        provider: 'prov_primary',
      })

      expect(
        rendered,
        `${locale} should preserve the provider placeholder when rendering description`
      ).toContain('prov_primary')

      if (exemptLocales.has(locale)) continue

      expect(
        getPathValue(mod.default, 'chat.providerFailover.description'),
        `${locale} should localize chat.providerFailover.description`
      ).not.toBe(referenceDescription)
    }
  })
})
