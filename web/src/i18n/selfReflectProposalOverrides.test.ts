import { describe, expect, it } from 'vitest'

import { deepMergeMessages, type LocaleMessages } from './merge'
import selfReflectProposalOverrides from './self-reflect-proposal-overrides'

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

const selfReflectProposalKeys = [
  'memory.proposalsTitle',
  'memory.proposalsDescription',
  'memory.proposalsDisabled',
  'memory.proposalStatusPending',
  'memory.proposalFilterPending',
  'memory.proposalStatusApproved',
  'memory.proposalStatusRejected',
  'memory.proposalsEmpty',
  'memory.proposalTarget',
  'memory.proposalLesson',
  'memory.proposalWhen',
  'memory.proposalEvidence',
  'memory.proposalEvaluation',
  'memory.proposalVerdict',
  'memory.proposalScore',
  'memory.proposalJudgeBackend',
  'memory.proposalJudgeModel',
  'memory.proposalCalibrationRef',
  'memory.proposalCandidateCount',
  'memory.proposalCalibration',
  'memory.proposalCoverage',
  'memory.proposalGroundedness',
  'memory.proposalFreshness',
  'memory.proposalConflictRisk',
  'memory.proposalConfidence',
  'memory.proposalRecommendedAction',
  'memory.proposalPatchPreview',
  'memory.proposalNoPatch',
  'memory.proposalReviewNote',
  'memory.proposalReviewPlaceholder',
  'memory.proposalReviewedAt',
  'memory.proposalNoSelection',
  'memory.proposalApproved',
  'memory.proposalRejected',
  'memory.proposalSourceKindResearch',
  'memory.proposalSourceKindSearch',
  'memory.proposalSourceKindUrl',
] as const

function buildMergedLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }

  const localeBase = locale === 'en-US' ? baseLocale : deepMergeMessages(baseLocale, localeMessages)
  return deepMergeMessages(
    localeBase,
    (selfReflectProposalOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
}

describe('self-reflect proposal locale coverage', () => {
  it('covers every non-primary locale in the dedicated override file', () => {
    const expectedLocales = localeCodes
      .filter((locale) => !primaryLocales.includes(locale as (typeof primaryLocales)[number]))
      .sort()
    expect(Object.keys(selfReflectProposalOverrides).sort()).toEqual(expectedLocales)

    for (const locale of expectedLocales) {
      const overrides = (
        selfReflectProposalOverrides as Record<string, LocaleMessages | undefined>
      )[locale]
      expect(overrides, `${locale} should have self-reflect proposal overrides`).toBeTruthy()
      for (const key of selfReflectProposalKeys) {
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
      for (const key of selfReflectProposalKeys) {
        const value = getByPath(messages as LocaleMessages, key)
        expect(typeof value, `${locale} should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('exposes self-reflect proposal strings in all 27 merged locales', () => {
    expect(localeCodes.length).toBe(27)
    for (const locale of localeCodes) {
      const messages = buildMergedLocaleMessages(locale)
      for (const key of selfReflectProposalKeys) {
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
