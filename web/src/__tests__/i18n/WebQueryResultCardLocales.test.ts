import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

type LocaleMessages = Record<string, unknown>

const requiredLabelKeys = [
  'input',
  'provider',
  'mode',
  'has_results',
  'selected_result',
  'key_facts',
  'key_factsanalysis',
  'research_artifact_path',
  'llm_compacted',
  'materialized',
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

describe('web query result-card locale coverage', () => {
  it('backfills compact web-query labels in every enhanced locale', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]

    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const enhancedReference = mergeHarnessLocale('en-US', localeModules[enUSPath]!.default)

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      const enhancedMessages = mergeHarnessLocale(locale as never, mod.default)

      for (const key of requiredLabelKeys) {
        const value = getPathValue(enhancedMessages, `resultCard.labels.${key}`)
        expect(typeof value, `${locale} should expose resultCard.labels.${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave resultCard.labels.${key} empty`
        ).toBeGreaterThan(0)
      }

      expect(
        getPathValue(enhancedMessages, 'resultCard.labels.key_factsanalysis'),
        `${locale} should keep the malformed key alias aligned with key_facts`
      ).toBe(getPathValue(enhancedMessages, 'resultCard.labels.key_facts'))

      if (locale === 'en-US' || locale === 'en-GB') continue

      for (const key of [
        'input',
        'provider',
        'has_results',
        'key_facts',
        'llm_compacted',
        'materialized',
      ]) {
        expect(
          getPathValue(enhancedMessages, `resultCard.labels.${key}`),
          `${locale} should localize resultCard.labels.${key}`
        ).not.toBe(getPathValue(enhancedReference, `resultCard.labels.${key}`))
      }
    }
  })
})
