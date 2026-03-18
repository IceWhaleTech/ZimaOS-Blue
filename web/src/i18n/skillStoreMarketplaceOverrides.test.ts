import { describe, expect, it } from 'vitest'

import { deepMergeMessages, type LocaleMessages } from './merge'
import skillStoreMarketplaceOverrides from './skill-store-marketplace-overrides'

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

const requiredMarketplaceKeys = [
  'skillStore.marketplace.badges.green',
  'skillStore.marketplace.badges.yellow',
  'skillStore.marketplace.badges.red',
  'skillStore.marketplace.categories.ai_intelligence',
  'skillStore.marketplace.categories.development_tools',
  'skillStore.marketplace.categories.productivity',
  'skillStore.marketplace.categories.data_analysis',
  'skillStore.marketplace.categories.content_creation',
  'skillStore.marketplace.categories.security_compliance',
  'skillStore.marketplace.categories.communication_collaboration',
  'skillStore.marketplace.categories.other',
] as const

function buildMergedLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }

  const localeBase = locale === 'en-US' ? baseLocale : deepMergeMessages(baseLocale, localeMessages)
  return deepMergeMessages(
    localeBase,
    (skillStoreMarketplaceOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
}

describe('skill store marketplace taxonomy translations', () => {
  it('declares marketplace overrides for all 27 locales', () => {
    const overrideLocales = Object.keys(skillStoreMarketplaceOverrides).sort()
    expect(overrideLocales).toEqual(localeCodes)
  })

  it('exposes localized badge and category labels in every merged locale', () => {
    for (const locale of localeCodes) {
      const messages = buildMergedLocaleMessages(locale)

      for (const key of requiredMarketplaceKeys) {
        const value = getByPath(messages, key)
        expect(typeof value, `${locale} should expose ${key}`).toBe('string')
        expect(String(value).trim().length, `${locale} should not leave ${key} empty`).toBeGreaterThan(
          0
        )
      }
    }
  })
})
