import { describe, expect, it } from 'vitest'

import { buildBuiltinSkillBackfill } from '@/i18n/builtin-skill-backfills'
import builtinToolBackfills from '@/i18n/builtin-tool-backfills'
import type { LocaleKey } from '@/i18n/locale-catalog'

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

const visibleBuiltinSkillLocaleCoverage = {
  'skills.catalog.computer_use.name': 'Computer Use',
  'skills.catalog.computer_use.description':
    'Control supported host OS windows and accessibility-backed browser flows for snapshots, targeting, scrolling, input, screenshots, and chat-style actions.',
  'skills.catalog.ask.name': 'Ask',
  'skills.catalog.ask.description': 'Ask the user follow-up questions needed to continue the task',
  'skills.catalog.advisor.name': 'Advisor',
  'skills.catalog.advisor.description':
    'Decision advisor for selection, replacement, migration, and best-practice questions.',
  'skills.catalog.calendar.name': 'Calendar',
  'skills.catalog.calendar.description': 'Create and manage calendar events',
  'skills.catalog.contacts.name': 'Contacts',
  'skills.catalog.contacts.description': 'Access and manage native contacts',
  'skills.catalog.config.name': 'Configuration',
  'skills.catalog.config.description': 'Manage runtime settings, providers, users, and admin controls.',
  'skills.catalog.research.name': 'Research',
  'skills.catalog.research.description':
    'Unified research family for citation-first investigation, bounded analysis, decision support, and UI or accessibility review.',
  'skills.catalog.deep_research.name': 'Deep Research',
  'skills.catalog.deep_research.description':
    'Run a Deep Research workflow. You can wait for the final report or get a job ID to check later.',
  'skills.catalog.docx.name': 'DOCX',
  'skills.catalog.docx.description':
    'Use when the task centers on a workspace .docx file and needs a native Word-style document for writing, template filling, placeholder edits, or validation.',
  'skills.catalog.email.name': 'Email',
  'skills.catalog.email.description': 'Send and manage emails via SMTP/IMAP',
  'skills.catalog.humanizer.name': 'Humanizer',
  'skills.catalog.humanizer.description': 'Rewrite a local text file into more natural language.',
  'skills.catalog.pdf.name': 'PDF',
  'skills.catalog.pdf.description':
    'Use when the task centers on a workspace .pdf file and needs PDF-native reading, form filling, printable output, or layout-preserving reformatting.',
  'skills.catalog.plan_append.name': 'Plan Append',
  'skills.catalog.plan_append.description': 'Append a task to an existing plan.',
  'skills.catalog.plan_create.name': 'Plan Create',
  'skills.catalog.plan_create.description': 'Create a checklist plan for the current scope.',
  'skills.catalog.plan_update.name': 'Plan Update',
  'skills.catalog.plan_update.description': 'Update an existing plan task.',
  'skills.catalog.pptx.name': 'PPTX',
  'skills.catalog.pptx.description':
    'Use when the task centers on a workspace .pptx file and needs a native slide deck for editable slides, layout changes, or chart updates.',
  'skills.catalog.scheduler.name': 'Scheduler',
  'skills.catalog.scheduler.description':
    'Create, list, delete, and trigger scheduled tasks (cron jobs)',
  'skills.catalog.self_reflect.name': 'Self Reflect',
  'skills.catalog.self_reflect.description': 'Capture lessons learned from a completed task.',
  'skills.catalog.summarize.name': 'Summarize',
  'skills.catalog.summarize.description':
    'Use summarize.sh to summarize URLs, files, and transcripts.',
  'skills.catalog.tasks.name': 'Tasks',
  'skills.catalog.tasks.description': 'Task and todo management',
  'skills.catalog.web_query.name': 'Web Query',
  'skills.catalog.web_query.description': 'Search or read public web pages from a query or URL.',
  'skills.catalog.xlsx.name': 'XLSX',
  'skills.catalog.xlsx.description':
    'Use when the task centers on a workspace .xlsx file and needs a native spreadsheet for tables, formulas, sheet edits, analysis, or validation.',
} as const

describe('skill locale labels', () => {
  it('keeps visible built-in skill names and descriptions localized for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)
      const builtinToolOverrides =
        (builtinToolBackfills as Record<string, LocaleMessages>)[locale] || {}
      const baseMessages = deepMergeMessages(mod.default, builtinToolOverrides)
      const builtinSkillOverrides = buildBuiltinSkillBackfill(locale as LocaleKey, baseMessages)
      const messages = deepMergeMessages(baseMessages, builtinSkillOverrides)

      for (const [path, englishValue] of Object.entries(visibleBuiltinSkillLocaleCoverage)) {
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

        if (path.endsWith('.name')) {
          continue
        }

        expect(localizedValue, `${file} should not fall back to English for ${path}`).not.toBe(
          englishValue
        )
      }

      expect(getPathValue(messages, 'skills.catalog.a11y.name'), `${file} should drop legacy a11y skill label`).toBeUndefined()
      expect(
        getPathValue(messages, 'skills.catalog.a11y.description'),
        `${file} should drop legacy a11y skill description`
      ).toBeUndefined()
      expect(
        getPathValue(messages, 'skills.catalog.himalaya.name'),
        `${file} should drop legacy himalaya skill label`
      ).toBeUndefined()
      expect(
        getPathValue(messages, 'skills.catalog.himalaya.description'),
        `${file} should drop legacy himalaya skill description`
      ).toBeUndefined()
    }
  })

})
