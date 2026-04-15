import { describe, expect, it } from 'vitest'

import builtinToolBackfills from '@/i18n/builtin-tool-backfills'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function getLocaleCode(modulePath: string): string {
  return fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
}

function isPlainObject(value: unknown): value is LocaleMessages {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function deepMergeMessages(base: LocaleMessages, override: LocaleMessages): LocaleMessages {
  const merged: LocaleMessages = { ...base }

  for (const [key, overrideValue] of Object.entries(override)) {
    const baseValue = merged[key]
    merged[key] =
      isPlainObject(baseValue) && isPlainObject(overrideValue)
        ? deepMergeMessages(baseValue, overrideValue)
        : overrideValue
  }

  return merged
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

const visibleBuiltinToolLocaleCoverage = {
  'tools.names.advisor': 'Advisor',
  'tools.names.docx': 'DOCX',
  'tools.names.find': 'Find',
  'tools.names.ls': 'List',
  'tools.names.pdf': 'PDF',
  'tools.names.pptx': 'PPTX',
  'tools.names.tool_search': 'Tool Search',
  'tools.names.xlsx': 'XLSX',
  'tools.descriptions.advisor':
    'Decision advisor for selection, replacement, migration, and best-practice questions.',
  'tools.descriptions.docx':
    'Use when the task centers on a workspace .docx file and needs a native Word-style document for writing, template filling, placeholder edits, or validation.',
  'tools.descriptions.find': 'Find files and directories by glob pattern',
  'tools.descriptions.ls': 'List files and directories',
  'tools.descriptions.pdf':
    'Use when the task centers on a workspace .pdf file and needs PDF-native reading, form filling, printable output, or layout-preserving reformatting.',
  'tools.descriptions.pptx':
    'Use when the task centers on a workspace .pptx file and needs a native slide deck for editable slides, layout changes, or chart updates.',
  'tools.descriptions.tool_search': 'Search tools, skills, and agents by capability',
  'tools.descriptions.xlsx':
    'Use when the task centers on a workspace .xlsx file and needs a native spreadsheet for tables, formulas, sheet edits, analysis, or validation.',
} as const

describe('tool locale labels', () => {
  it('keeps tool-related details and tags localized for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)
      const builtinToolOverrides =
        (builtinToolBackfills as Record<string, LocaleMessages>)[locale] || {}
      const messages = deepMergeMessages(mod.default, builtinToolOverrides)

      const commonDetails = getPathValue(messages, 'common.details')
      const extensionsDetails = getPathValue(messages, 'extensions.actions.details')
      const skillStoreActionDetails = getPathValue(messages, 'skillStore.actions.details')
      const skillStoreDetailTitle = getPathValue(messages, 'skillStore.detail.title')
      const skillStoreSectionDetails = getPathValue(messages, 'skillStore.detail.sections.details')
      const memoryTags = getPathValue(messages, 'memory.tags')
      const memoryTagsPlaceholder = getPathValue(messages, 'memory.tagsPlaceholder')

      expect(typeof commonDetails, `${file} missing common.details`).toBe('string')
      expect(typeof extensionsDetails, `${file} missing extensions.actions.details`).toBe('string')
      expect(typeof skillStoreActionDetails, `${file} missing skillStore.actions.details`).toBe(
        'string'
      )
      expect(typeof skillStoreDetailTitle, `${file} missing skillStore.detail.title`).toBe('string')
      expect(
        typeof skillStoreSectionDetails,
        `${file} missing skillStore.detail.sections.details`
      ).toBe('string')
      expect(typeof memoryTags, `${file} missing memory.tags`).toBe('string')
      expect(typeof memoryTagsPlaceholder, `${file} missing memory.tagsPlaceholder`).toBe('string')

      expect(extensionsDetails, `${file} extensions details label`).toBe(commonDetails)
      expect(skillStoreActionDetails, `${file} skillStore action details label`).toBe(commonDetails)
      expect(skillStoreDetailTitle, `${file} skillStore detail title`).toBe(commonDetails)
      expect(skillStoreSectionDetails, `${file} skillStore detail section label`).toBe(
        commonDetails
      )

      if (locale === 'en-US' || locale === 'en-GB') {
        expect(commonDetails, `${file} English details label`).toBe('Details')
        expect(memoryTags, `${file} English memory tags label`).toBe('Tags (optional)')
        expect(memoryTagsPlaceholder, `${file} English memory tags placeholder`).toBe(
          'tag1, tag2, tag3'
        )
        continue
      }

      expect(commonDetails, `${file} should not fall back to English details`).not.toBe('Details')
      expect(memoryTags, `${file} should not fall back to English tags`).not.toBe('Tags (optional)')
      expect(
        memoryTagsPlaceholder,
        `${file} should not fall back to English tag placeholders`
      ).not.toBe('tag1, tag2, tag3')
    }
  })

  it('keeps visible built-in tool names and descriptions localized for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)
      const builtinToolOverrides =
        (builtinToolBackfills as Record<string, LocaleMessages>)[locale] || {}
      const messages = deepMergeMessages(mod.default, builtinToolOverrides)

      for (const [path, englishValue] of Object.entries(visibleBuiltinToolLocaleCoverage)) {
        const localizedValue = getPathValue(messages, path)
        expect(typeof localizedValue, `${file} missing ${path}`).toBe('string')
        expect(String(localizedValue).trim().length, `${file} empty ${path}`).toBeGreaterThan(0)

        if (locale === 'en-US') {
          expect(localizedValue, `${file} English copy for ${path}`).toBe(englishValue)
          continue
        }

        if (locale === 'en-GB') {
          continue
        }

        if (path.includes('.names.')) {
          continue
        }

        expect(localizedValue, `${file} should not fall back to English for ${path}`).not.toBe(
          englishValue
        )
      }
    }
  })
})
