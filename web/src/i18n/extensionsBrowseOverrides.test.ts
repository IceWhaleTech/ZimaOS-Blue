import { describe, expect, it } from 'vitest'

import { deepMergeMessages, type LocaleMessages } from './merge'
import extensionsBrowseOverrides from './extensions-browse-overrides'

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

const requiredKeys = [
  'extensions.browse.skillGalleryHint',
  'extensions.browse.toolGalleryHint',
  'extensions.browse.closeSkillDetails',
  'extensions.browse.sourceLabel',
  'extensions.browse.skillCollectionFilters',
  'skills.catalog.analyze.name',
  'skills.catalog.ui_reviewer.name',
  'skills.catalog.mgmt.name',
  'skills.names.Analysis Report',
  'skills.names.UI Reviewer',
  'skills.names.Configuration',
  'tools.names.analyze',
  'tools.names.ui_reviewer',
  'tools.names.Analysis Report',
  'tools.names.UI Reviewer',
  'tools.names.Configuration',
] as const

function buildMergedLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }

  const localeBase = locale === 'en-US' ? baseLocale : deepMergeMessages(baseLocale, localeMessages)
  return deepMergeMessages(
    localeBase,
    (extensionsBrowseOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
}

describe('extensions browse localization coverage', () => {
  it('declares browse overrides for all 27 locales', () => {
    const overrideLocales = Object.keys(extensionsBrowseOverrides).sort()
    expect(overrideLocales).toEqual(localeCodes)
  })

  it('exposes translated browse copy and card labels in every merged locale', () => {
    for (const locale of localeCodes) {
      const messages = buildMergedLocaleMessages(locale)

      for (const key of requiredKeys) {
        const value = getByPath(messages, key)
        expect(typeof value, `${locale} should expose ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })
})
