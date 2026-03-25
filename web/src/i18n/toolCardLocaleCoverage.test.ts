import { describe, expect, it } from 'vitest'

import { deepMergeMessages, type LocaleMessages } from './merge'
import priorityLocaleOverrides from './priority-overrides'

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

const requiredPriorityOverrideKeys = [
  'terminalCard.title',
  'execCard.title',
  'execCard.local',
  'execCard.sandbox',
  'execCard.builtin',
  'execCard.command',
  'execCard.output',
  'execCard.stdout',
  'execCard.stderr',
  'execCard.session',
  'execCard.riskLabel',
  'resultCard.labels.pattern',
  'resultCard.labels.max_results',
  'resultCard.labels.case_sensitive',
  'resultCard.labels.backend',
  'resultCard.labels.backend_source',
  'resultCard.labels.fallback_reason',
] as const

const requiredSourceLocaleKeys = [
  'execCard.title',
  'execCard.local',
  'execCard.sandbox',
  'execCard.builtin',
  'execCard.command',
  'execCard.output',
  'execCard.stdout',
  'execCard.stderr',
  'execCard.session',
  'execCard.riskLabel',
] as const

const requiredMergedKeys = [
  ...requiredPriorityOverrideKeys,
  'execCard.local',
  'execCard.sandbox',
  'execCard.builtin',
  'execCard.exitCode',
  'execCard.duration',
  'execCard.outputTruncated',
  'execCard.outputUnavailable',
  'execCard.noOutput',
  'execCard.running',
  'execCard.copyCommand',
  'execCard.copied',
  'execCard.commandHidden',
  'execCard.hideCommand',
  'execCard.showCommand',
  'execCard.collapse',
  'execCard.expand',
  'execCard.lines',
  'execCard.risk.low',
  'execCard.risk.medium',
  'execCard.risk.high',
  'execCard.risk.critical',
] as const

function buildMergedLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }

  const localeBase = locale === 'en-US' ? baseLocale : deepMergeMessages(baseLocale, localeMessages)
  return deepMergeMessages(
    localeBase,
    (priorityLocaleOverrides as Record<string, LocaleMessages | undefined>)[locale] || {}
  )
}

describe('tool card locale coverage', () => {
  it('declares the core exec-card labels directly in every locale source file', () => {
    expect(localeCodes).toHaveLength(27)

    for (const locale of localeCodes) {
      const localeMessages = localeMessagesByCode.get(locale) as LocaleMessages
      expect(localeMessages, `${locale} should load source locale messages`).toBeTruthy()

      for (const key of requiredSourceLocaleKeys) {
        const value = getByPath(localeMessages, key)
        expect(typeof value, `${locale} source locale should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} source locale should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('defines the new exec/result tool-card keys in priority overrides for every non-primary locale', () => {
    expect(localeCodes).toHaveLength(27)

    for (const locale of localeCodes.filter((code) => code !== 'en-US')) {
      const overrides = (priorityLocaleOverrides as Record<string, LocaleMessages | undefined>)[
        locale
      ]
      expect(overrides, `${locale} should have priority locale overrides`).toBeTruthy()

      for (const key of requiredPriorityOverrideKeys) {
        const value = getByPath(overrides as LocaleMessages, key)
        expect(typeof value, `${locale} should override ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('exposes the new exec/result tool-card keys in every merged locale', () => {
    for (const locale of localeCodes) {
      const messages = buildMergedLocaleMessages(locale)

      for (const key of requiredMergedKeys) {
        const value = getByPath(messages, key)
        expect(typeof value, `${locale} should expose ${key} after merge`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty after merge`
        ).toBeGreaterThan(0)
      }
    }
  })
})
