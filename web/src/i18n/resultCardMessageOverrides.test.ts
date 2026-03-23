import { describe, expect, it } from 'vitest'

import type { LocaleMessages } from './merge'
import resultCardMessageOverrides from './result-card-message-overrides'

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

const localeCodes = Object.keys(localeModules)
  .map(fileNameFromModulePath)
  .map(localeCodeFromFile)
  .sort()

const requiredKeys = [
  'resultCard.messages.file_written_successfully',
  'resultCard.messages.search_completed',
  'resultCard.messages.auto_answered_silent_mode',
  'resultCard.messages.screenshot_captured',
  'resultCard.messages.screenshot_captured_interactive_elements_unavailable',
  'resultCard.messageTemplates.found_results',
  'resultCard.messageTemplates.reminder_count',
  'resultCard.messageTemplates.cleared_reminders',
  'resultCard.messageTemplates.entries_in_path',
  'resultCard.messageTemplates.single_entry_in_path',
  'resultCard.messageTemplates.no_entries_in_path',
  'resultCard.messageTemplates.showing_first_entries_in_path',
  'resultCard.messageTemplates.screenshot_captured_for',
  'resultCard.warnings.listing_truncated',
] as const

describe('result card message override coverage', () => {
  it('defines translated backend result-card text for all 27 locales', () => {
    const overrides = resultCardMessageOverrides as Record<string, LocaleMessages>
    const overrideLocales = Object.keys(overrides).sort()

    expect(localeCodes).toHaveLength(27)
    expect(overrideLocales).toEqual(localeCodes)

    for (const locale of overrideLocales) {
      const localeMessages = overrides[locale]
      expect(localeMessages, `${locale} should define result-card overrides`).toBeTruthy()
      const resolvedLocaleMessages = localeMessages as LocaleMessages

      for (const key of requiredKeys) {
        const value = getByPath(resolvedLocaleMessages, key)
        expect(typeof value, `${locale} should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })
})
