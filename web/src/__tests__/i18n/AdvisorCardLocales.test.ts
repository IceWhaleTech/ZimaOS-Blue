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

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

describe('Advisor card locale coverage', () => {
  it('keeps advisor card labels available across all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    const requiredKeys = [
      'advisorCard.pending',
      'advisorCard.recommendation',
      'advisorCard.rationale',
      'advisorCard.winner',
      'advisorCard.topCandidates',
      'advisorCard.weightedCriteria',
      'advisorCard.tradeoffs',
      'advisorCard.risks',
      'advisorCard.bestPractices',
      'advisorCard.alternatives',
      'advisorCard.confidence',
      'advisorCard.evidence',
      'advisorCard.progress',
      'advisorCard.jobId',
      'advisorCard.secondOpinion',
    ]

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const localeKey = file.replace(/\.ts$/, '') as LocaleKey
      const runtimeMessages = mergeHarnessLocale(localeKey, mod.default)

      for (const key of requiredKeys) {
        const value = getPathValue(runtimeMessages, key)
        expect(typeof value, `${file} missing ${key}`).toBe('string')
        expect(String(value).trim().length, `${file} empty ${key}`).toBeGreaterThan(0)
      }
    }
  })
})
